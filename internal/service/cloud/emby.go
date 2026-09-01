// Emby remote provider: exposes a remote Emby server through the same
// Provider interface used by cloud disks, so account CRUD / connectivity
// test / directory browser work unchanged. This is a thin adapter — the
// federated Emby API aggregation (Views / Items / PlaybackInfo / streaming
// proxy) lives in service.EmbyRemoteService and does not go through the
// cloud-disk sync machinery.
package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Emby 远程挂载类型（service 层聚合走 EmbyRemoteService，不走 STRM 同步）。

// embyProvider implements Provider against a remote Emby server using an
// api_key (token) for authentication. DirectLink.Resolve returns the remote
// stream URL; whether MMTL reverse-proxies the bytes is decided by the
// emby.proxy_play account config (defaults to off).
type embyProvider struct {
	base      string // e.g. http://host:8096（自动补 /emby 前缀）
	username  string
	password  string
	token     string // api_key
	userID    string // 远程用户 Id
	proxyPlay bool
	client    *http.Client
}

type embyUserPayload struct {
	Id string `json:"Id"`
}

type embyLoginResponse struct {
	AccessToken string          `json:"AccessToken"`
	User        embyUserPayload `json:"User"`
}

type embyPingResponse struct {
	ServerName string `json:"ServerName"`
}

// newEmby builds the provider from the account config map.
func newEmby(cfg map[string]any, client *http.Client) Provider {
	p := &embyProvider{
		base:      strings.TrimRight(str(cfg["url"]), "/"),
		username:  str(cfg["username"]),
		password:  str(cfg["password"]),
		token:     firstNonEmpty(str(cfg["api_key"]), str(cfg["token"])),
		userID:    str(cfg["remote_user_id"]),
		proxyPlay: boolish(cfg["proxy_play"]),
		client:    client,
	}
	if p.client == nil {
		p.client = http.DefaultClient
	}
	return p
}

// embyBase normalizes the address so requests go to /emby/... endpoints.
func (p *embyProvider) embyBase() string {
	base := strings.TrimRight(p.base, "/")
	if !strings.Contains(base, "/emby") {
		base += "/emby"
	}
	return base
}

// externalBase 不追加 /emby（内嵌媒体资源 URL 使用 /emby 会更贴近习惯，此处
// 与 embyBase 保持一致：所有端点统一以 /emby 开头）。
func (p *embyProvider) apiBase() string { return p.embyBase() }

func (p *embyProvider) Type() string { return TypeEmbyRemote }

// Ping 验证地址连通性与凭据（/System/Info）。
func (p *embyProvider) Ping(ctx context.Context) error {
	if p.base == "" {
		return errors.New("缺少 Emby 地址")
	}
	token, err := p.ensureToken(ctx)
	if err != nil {
		return err
	}
	return p.doJSON(ctx, http.MethodGet, "/System/Info", nil, token, &embyPingResponse{})
}

// doJSON 向远程 Emby 发起带 api_key 的请求并解析 JSON 响应。
func (p *embyProvider) doJSON(ctx context.Context, method, path string, body io.Reader, token string, out any) error {
	endpoint := p.apiBase() + path
	if token != "" {
		sep := "?"
		if strings.Contains(endpoint, "?") {
			sep = "&"
		}
		endpoint += sep + "api_key=" + url.QueryEscape(token)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Emby-Token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized {
			return ErrEmbyUnauthorized
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("emby 请求失败(%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ErrEmbyUnauthorized 表示远程凭据失效（触发重新认证/打回测试）。
var ErrEmbyUnauthorized = errors.New("emby 认证失败或凭据已失效")

// ensureToken 返回可用 api_key：已有则直接用，否则尝试账号密码认证。
func (p *embyProvider) ensureToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(p.token) != "" {
		return p.token, nil
	}
	if strings.TrimSpace(p.username) == "" {
		return "", errors.New("缺少 Emby 凭据（token 或 用户名/密码）")
	}
	payload := map[string]string{"Username": p.username, "Pw": p.password}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase()+"/Users/AuthenticateByName", strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization", `MediaBrowser Client="MMTL", Device="MMTL-Federated", DeviceId="mmtl-federated", Version="1.0"`)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("emby 登录失败(%d)", resp.StatusCode)
	}
	var login embyLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil {
		return "", err
	}
	if strings.TrimSpace(login.AccessToken) == "" {
		return "", errors.New("emby 登录成功但未返回 AccessToken")
	}
	p.token = login.AccessToken
	if login.User.Id != "" {
		p.userID = login.User.Id
	}
	return p.token, nil
}

// embyItemSummary 目录浏览所需的最小 Emby 条目字段。
type embyItemSummary struct {
	Id          string `json:"Id"`
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	IsFolder    bool   `json:"IsFolder"`
	ChildCount  int    `json:"ChildCount"`
	RunTimeTicks int64 `json:"RunTimeTicks"`
}

type embyItemListResponse struct {
	Items []embyItemSummary `json:"Items"`
}

// List 把远程媒体库(View)展开为目录树：dirID 为空=媒体库列表；否则返回该
// 目录(Movie/Series/Season/Folder)下的条目。用于账号「浏览目录」调试入口。
func (p *embyProvider) List(ctx context.Context, dirID string) ([]FileEntry, error) {
	token, err := p.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	userID := p.userID
	if userID == "" {
		userID = "0" // 某些 Emby 允许用 0 代表管理员
	}
	path := "/Users/" + url.PathEscape(userID) + "/Items"
	if dirID != "" {
		path += "?ParentId=" + url.QueryEscape(dirID)
	} else {
		path += "?IncludeItemTypes=CollectionFolder"
	}
	var out embyItemListResponse
	if err := p.doJSON(ctx, http.MethodGet, path, nil, token, &out); err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, len(out.Items))
	for _, it := range out.Items {
		size := int64(0)
		if it.RunTimeTicks > 0 {
			size = it.RunTimeTicks / 10_000_000 // 秒
		}
		entries = append(entries, FileEntry{
			ID:    it.Id,
			Name:  it.Name,
			IsDir: it.IsFolder || it.Type != "Movie",
			Size:  size,
		})
	}
	return entries, nil
}

// Resolve 返回远程 Emby 直链。Proxy=true 时由调用方(StrmService.ProxyDirect)
// 反向代理流量；false 时 302 到直链。默认不代理（播放字节不经过 MMTL）。
func (p *embyProvider) Resolve(ctx context.Context, fileRef string) (*DirectLink, error) {
	token, err := p.ensureToken(ctx)
	if err != nil {
		return nil, err
	}
	u := p.apiBase() + "/Videos/" + url.PathEscape(fileRef) + "/stream"
	u += "?api_key=" + url.QueryEscape(token) + "&Static=true&MediaSourceId=" + url.QueryEscape(fileRef)
	return &DirectLink{URL: u, Headers: map[string]string{"X-Emby-Token": token}, Proxy: p.proxyPlay}, nil
}