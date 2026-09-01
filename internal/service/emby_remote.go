// EmbyRemoteService 是「远程 Emby 联邦聚合」核心：把挂载的远程 Emby 服务器
// 作为外部媒体源，通过 MMTL 的 Emby 兼容 API 透出。
//
// 设计要点：
//   - 远程媒体的元数据完全不落库：每次请求实时向远程 Emby 拉取；
//   - 条目 ID 用 embyremote~{accountID}~{remoteID} 伪装（见 emby_remote_ids.go），
//     客户端拿伪装 ID 回来时按账号路由回远程；
//   - 播放分流由账号级 proxy_play 配置决定：
//     不代理（默认）= MediaSource 下发热门远程绝对 URL，播放字节完全不经过 MMTL；
//     代理 = 下发 MMTL 本地 /Videos/{encoded} 端点，由 ProxyVideoStream 反向拉流。
//
// 配置复用 STRM 账号体系（StrmAccount.Provider = emby_remote），CRUD/加密/连通
// 测试全部走既有 /admin/strm/accounts 接口，不需要新增数据表。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
)

// embyRemoteHTTPTimeout 远程 Emby 常规 API 请求超时（流式代理不在此列）。
const embyRemoteHTTPTimeout = 15 * time.Second

// EmbyRemoteConfig 是一个远程 Emby 账号的解密配置。
type EmbyRemoteConfig struct {
	BaseURL      string // http://host:8096（无需 /emby 后缀）
	Username     string
	Password     string
	Token        string // api_key（手动填写或自动认证获得）
	RemoteUserID string // 远程用户 Id（自动认证后回填）
	ProxyPlay    bool   // true=播放流量经 MMTL 反向代理；false=客户端直连远程
}

// EmbyRemoteService 提供对远程 Emby 服务器的读写封装。
type EmbyRemoteService struct {
	cfg    *config.Config
	log    *zap.Logger
	repo   *repository.Container
	crypto *CryptoService
	http   *http.Client
}

// NewEmbyRemoteService 构造远程 Emby 聚合服务。
func NewEmbyRemoteService(cfg *config.Config, log *zap.Logger, repo *repository.Container, crypto *CryptoService) *EmbyRemoteService {
	return &EmbyRemoteService{
		cfg:    cfg,
		log:    log,
		repo:   repo,
		crypto: crypto,
		http: &http.Client{
			Timeout: embyRemoteHTTPTimeout,
		},
	}
}

// ListAccounts 返回全部启用的远程 Emby 挂载账号。
func (r *EmbyRemoteService) ListAccounts(ctx context.Context) ([]model.StrmAccount, error) {
	accounts, err := r.repo.StrmAccount.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.StrmAccount, 0, len(accounts))
	for i := range accounts {
		if accounts[i].Provider == model.StrmProviderEmbyRemote {
			out = append(out, accounts[i])
		}
	}
	return out, nil
}

// AccountByID 按 ID 查找远程 Emby 挂载账号（不存在或类型不符返回 nil）。
func (r *EmbyRemoteService) AccountByID(ctx context.Context, id string) *model.StrmAccount {
	if strings.TrimSpace(id) == "" {
		return nil
	}
	acct, err := r.repo.StrmAccount.FindByID(ctx, id)
	if err != nil || acct == nil {
		return nil
	}
	if acct.Provider != model.StrmProviderEmbyRemote || !acct.Enabled {
		return nil
	}
	return acct
}

// configOf 解密账号配置。
func (r *EmbyRemoteService) configOf(acct *model.StrmAccount) (*EmbyRemoteConfig, error) {
	raw := map[string]string{}
	if acct != nil && strings.TrimSpace(acct.Config) != "" {
		if err := json.Unmarshal([]byte(acct.Config), &raw); err != nil {
			return nil, fmt.Errorf("decode emby account config: %w", err)
		}
	}
	cfg := &EmbyRemoteConfig{
		BaseURL:      strings.TrimRight(strings.TrimSpace(raw["url"]), "/"),
		Username:     strings.TrimSpace(raw["username"]),
		Password:     r.crypto.Decrypt(raw["password"]),
		Token:        firstNonEmptyStr(r.crypto.Decrypt(raw["api_key"]), r.crypto.Decrypt(raw["token"])),
		RemoteUserID: strings.TrimSpace(raw["remote_user_id"]),
		ProxyPlay:    parseBoolSetting(raw["proxy_play"], false),
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("缺少 Emby 地址")
	}
	if !strings.HasPrefix(cfg.BaseURL, "http://") && !strings.HasPrefix(cfg.BaseURL, "https://") {
		return nil, errors.New("Emby 地址必须以 http:// 或 https:// 开头")
	}
	return cfg, nil
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// embyBase 把地址规范为不带尾部斜杠的 /emby 根。
func (r *EmbyRemoteService) embyBase(cfg *EmbyRemoteConfig) string {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(base, "/emby") {
		base += "/emby"
	}
	return base
}

// ensureToken 返回可用的 api_key：已有则直接用；否则用用户名/密码认证并回写
// 数据库（自动获得的 token 与 remote_user_id 会加密保存在账号配置里）。
func (r *EmbyRemoteService) ensureToken(ctx context.Context, acct *model.StrmAccount, cfg *EmbyRemoteConfig) error {
	if strings.TrimSpace(cfg.Token) != "" {
		return nil
	}
	if strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Password) == "" {
		return errors.New("缺少 Emby 凭据：请填写 api_key 或 用户名/密码")
	}
	body, _ := json.Marshal(map[string]string{"Username": cfg.Username, "Pw": cfg.Password})
	endpoint := r.embyBase(cfg) + "/Users/AuthenticateByName"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Emby-Authorization", `MediaBrowser Client="MMTL", Device="MMTL-Federated", DeviceId="mmtl-federated", Version="1.0"`)
	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("连接远程 Emby 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("远程 Emby 登录失败(%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var login struct {
		AccessToken string `json:"AccessToken"`
		User        struct {
			Id string `json:"Id"`
		} `json:"User"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil {
		return err
	}
	if strings.TrimSpace(login.AccessToken) == "" {
		return errors.New("远程 Emby 未返回 AccessToken")
	}
	cfg.Token = login.AccessToken
	if login.User.Id != "" {
		cfg.RemoteUserID = login.User.Id
	}
	return r.persistToken(ctx, acct, cfg)
}

// persistToken 把认证得到的 token / user id 加密写回账号配置（下次请求免登录）。
func (r *EmbyRemoteService) persistToken(ctx context.Context, acct *model.StrmAccount, cfg *EmbyRemoteConfig) error {
	if acct == nil {
		return nil
	}
	raw := map[string]string{}
	if strings.TrimSpace(acct.Config) != "" {
		_ = json.Unmarshal([]byte(acct.Config), &raw)
	}
	raw["api_key"] = r.crypto.Encrypt(cfg.Token)
	raw["remote_user_id"] = cfg.RemoteUserID
	if strings.TrimSpace(raw["username"]) == "" {
		raw["username"] = cfg.Username
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	acct.Config = string(data)
	return r.repo.StrmAccount.Update(ctx, acct)
}

// doGet 向远程 Emby 发起带 api_key 的 GET，把响应 JSON 解码到 out。
// 401 时自动重认证一次再重试（凭据过期场景）。
func (r *EmbyRemoteService) doGet(ctx context.Context, acct *model.StrmAccount, cfg *EmbyRemoteConfig, path string, q url.Values, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		if err := r.ensureToken(ctx, acct, cfg); err != nil {
			return err
		}
		endpoint := r.embyBase(cfg) + path
		if q != nil {
			endpoint += "?" + q.Encode()
		} else {
			endpoint += "?api_key=" + url.QueryEscape(cfg.Token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Emby-Token", cfg.Token)
		resp, err := r.http.Do(req)
		if err != nil {
			return fmt.Errorf("请求远程 Emby 失败: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			// token 失效：清空后重认证重试一次。
			cfg.Token = ""
			if acct != nil {
raw := map[string]string{}
			_ = json.Unmarshal([]byte(acct.Config), &raw)
			delete(raw, "api_key")
			enc, _ := json.Marshal(raw)
			acct.Config = string(enc)
			_ = r.repo.StrmAccount.Update(ctx, acct)
			}
			continue
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("远程 Emby 请求失败(%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(data, out)
	}
	return errors.New("远程 Emby 认证重试失败")
}

// TestConnection 连通性测试：确保地址可达且凭据有效；成功时回写自动认证信息。
func (r *EmbyRemoteService) TestConnection(ctx context.Context, acct *model.StrmAccount) error {
	cfg, err := r.configOf(acct)
	if err != nil {
		return err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return err
	}
	var out json.RawMessage
	return r.doGet(ctx, acct, cfg, "/System/Info", nil, &out)
}

// ─── 元数据 / 目录聚合 ─────────────────────────────────────────────────────────

// RemoteViews 拉取远程媒体库（View）列表，返回远程原始 view map（未重写）。
func (r *EmbyRemoteService) RemoteViews(ctx context.Context, acct *model.StrmAccount) ([]map[string]any, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	q := url.Values{"api_key": {cfg.Token}}
	var body struct {
		Items []map[string]any `json:"Items"`
	}
	if err := r.doGet(ctx, acct, cfg, "/Users/"+url.PathEscape(r.remoteUserID(cfg))+"/Views", q, &body); err != nil {
		return nil, err
	}
	return body.Items, nil
}

func (r *EmbyRemoteService) remoteUserID(cfg *EmbyRemoteConfig) string {
	if strings.TrimSpace(cfg.RemoteUserID) != "" {
		return cfg.RemoteUserID
	}
	return "0" // 未认证出的兜底：部分 Emby 接受 0 代表管理员
}

// RemoteItems 向远程 Emby 转发 /Items 浏览/搜索请求，返回重写后的响应载荷。
// p 的分页/排序/过滤参数原样转发，分页语义完全由远程承接。
func (r *EmbyRemoteService) RemoteItems(ctx context.Context, acct *model.StrmAccount, p ItemsParams) (map[string]any, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	_, remoteParent, _ := DecodeEmbyRemoteID(p.ParentID)
	q := url.Values{}
	if remoteParent != "" {
		q.Set("ParentId", remoteParent)
	}
	q.Set("UserId", r.remoteUserID(cfg))
	q.Set("Limit", strconv.Itoa(p.Limit))
	q.Set("StartIndex", strconv.Itoa(p.StartIndex))
	if p.SearchTerm != "" {
		q.Set("SearchTerm", p.SearchTerm)
	}
	if p.Recursive {
		q.Set("Recursive", "true")
	}
	if p.SortBy != "" {
		q.Set("SortBy", p.SortBy)
	}
	if p.SortOrder != "" {
		q.Set("SortOrder", p.SortOrder)
	}
	if len(p.IncludeItemTypes) > 0 {
		q.Set("IncludeItemTypes", strings.Join(p.IncludeItemTypes, ","))
	}
	if len(p.Filters) > 0 {
		q.Set("Filters", strings.Join(p.Filters, ","))
	}
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/Items"
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, path, q, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{"Items": []any{}, "TotalRecordCount": 0, "StartIndex": p.StartIndex}
	}
	RewriteEmbyRemoteIDs(out, acct.ID)
	return out, nil
}

// RemoteItem 拉取远程单条目详情（含响应的重写）。
func (r *EmbyRemoteService) RemoteItem(ctx context.Context, acct *model.StrmAccount, remoteID string) (map[string]any, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/Items/" + url.PathEscape(remoteID)
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, path, nil, &out); err != nil {
		return nil, err
	}
	RewriteEmbyRemoteIDs(out, acct.ID)
	return out, nil
}

// RemoteLatest 拉取远程「最近添加」（用于 /Items/Latest 聚合）。
func (r *EmbyRemoteService) RemoteLatest(ctx context.Context, acct *model.StrmAccount, parentID string, limit int) ([]map[string]any, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	q := url.Values{"Limit": {strconv.Itoa(limit)}}
	if parentID != "" {
		q.Set("ParentId", parentID)
	}
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/Items/Latest"
	var out []map[string]any
	if err := r.doGet(ctx, acct, cfg, path, q, &out); err != nil {
		return nil, err
	}
	RewriteEmbyRemoteIDs(out, acct.ID)
	return out, nil
}

// RemotePlaybackInfo 拉取远程 PlaybackInfo，并按 proxy_play 配置重写播放 URL：
//   - 不代理：MediaSource 的 DirectStreamUrl / TranscodingUrl / 字幕 DeliveryUrl
//     指向远程绝对地址（播放字节不过 MMTL）；
//   - 代理：指向 MMTL 本地 /Videos/{encodedID} 端点（由 ProxyVideoStream 反代）。
func (r *EmbyRemoteService) RemotePlaybackInfo(ctx context.Context, acct *model.StrmAccount, remoteID, userID string) (map[string]any, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return nil, err
	}
	q := url.Values{"UserId": {r.remoteUserID(cfg)}}
	path := "/Items/" + url.PathEscape(remoteID) + "/PlaybackInfo"
	var out map[string]any
	if err := r.doGet(ctx, acct, cfg, path, q, &out); err != nil {
		return nil, err
	}
	RewriteEmbyRemoteIDs(out, acct.ID)
	r.rewritePlayURLs(out, acct, cfg, remoteID)
	return out, nil
}

// rewritePlayURLs 按代理模式重写载荷内 MediaSources 的播放地址。
func (r *EmbyRemoteService) rewritePlayURLs(value any, acct *model.StrmAccount, cfg *EmbyRemoteConfig, remoteID string) {
	encoded := EncodeEmbyRemoteID(acct.ID, remoteID)
	sources := collectMediaSources(value)
	if sources == nil {
		return
	}
	for _, src := range sources {
		mediaSourceID, _ := src["Id"].(string)
		var streamPath, subtitlePlayURL string
		if cfg.ProxyPlay {
			streamPath = "/Videos/" + url.PathEscape(encoded) + "/stream"
			subtitlePlayURL = "/Videos/" + url.PathEscape(encoded)
		} else {
			base := r.embyBase(cfg)
			streamPath = base + "/Videos/" + url.PathEscape(remoteID) + "/stream?api_key=" + url.QueryEscape(cfg.Token) + "&Static=true"
			if mediaSourceID != "" {
				streamPath += "&MediaSourceId=" + url.QueryEscape(mediaSourceID)
			}
			subtitlePlayURL = base + "/Videos/" + url.PathEscape(remoteID)
		}
		if _, exists := src["DirectStreamUrl"]; exists {
			src["DirectStreamUrl"] = streamPath
		}
		if _, exists := src["TranscodingUrl"]; exists {
			src["TranscodingUrl"] = streamPath
		}
		rewriteSubtitleDeliveryURLs(src, subtitlePlayURL, cfg)
	}
}

// collectMediaSources 从载荷中取出所有 MediaSources（顶层或嵌套 Items 内）。
func collectMediaSources(value any) []map[string]any {
	var out []map[string]any
	switch typed := value.(type) {
	case map[string]any:
		if sources, ok := typed["MediaSources"].([]map[string]any); ok {
			out = append(out, sources...)
		} else if sources, ok := typed["MediaSources"].([]any); ok {
			for _, s := range sources {
				if m, isMap := s.(map[string]any); isMap {
					out = append(out, m)
				}
			}
		}
		if items, ok := typed["Items"]; ok {
			out = append(out, collectMediaSources(items)...)
		}
	case []any:
		for _, item := range typed {
			out = append(out, collectMediaSources(item)...)
		}
	case []map[string]any:
		for _, item := range typed {
			out = append(out, collectMediaSources(item)...)
		}
	}
	return out
}

var embySubtitleDeliveryRE = regexp.MustCompile(`/Subtitles/(\d+)/Stream(\.[A-Za-z0-9]+)?`)

// rewriteSubtitleDeliveryURLs 把 MediaSource 内字幕轨道的 DeliveryUrl 改写到
// subtitlePlayURL 前缀（客户端请求本地代理端点 / 远程绝对地址）。
func rewriteSubtitleDeliveryURLs(src map[string]any, playURL string, cfg *EmbyRemoteConfig) {
	if src == nil {
		return
	}
	streams, ok := src["MediaStreams"].([]any)
	if !ok {
		return
	}
	for _, s := range streams {
		stream, isMap := s.(map[string]any)
		if !isMap || stream["Type"] != "Subtitle" {
			continue
		}
		raw, _ := stream["DeliveryUrl"].(string)
		if raw == "" {
			continue
		}
		idx := "1"
		if m := embySubtitleDeliveryRE.FindStringSubmatch(raw); len(m) >= 2 {
			idx = m[1]
		}
		ext := ""
		if m := embySubtitleDeliveryRE.FindStringSubmatch(raw); len(m) >= 3 {
			ext = m[2]
		}
		base := strings.TrimRight(playURL, "/")
		delivery := base + "/Subtitles/" + idx + "/Stream" + ext
		if !cfg.ProxyPlay && strings.TrimSpace(cfg.Token) != "" {
			delivery += "?api_key=" + url.QueryEscape(cfg.Token)
		}
		stream["DeliveryUrl"] = delivery
	}
}

// RemoteImageURL 构造远程图片绝对地址（由既有 ImageProxy 拉取透传）。
func (r *EmbyRemoteService) RemoteImageURL(ctx context.Context, acct *model.StrmAccount, remoteID, imageType string) (string, error) {
	cfg, err := r.configOf(acct)
	if err != nil {
		return "", err
	}
	return r.embyBase(cfg) + "/Items/" + url.PathEscape(remoteID) + "/Images/" + url.PathEscape(strings.ToLower(imageType)) +
		"?api_key=" + url.QueryEscape(cfg.Token), nil
}

// ─── 播放代理 ─────────────────────────────────────────────────────────────────

// ProxyVideoStream 反向代理远程 Emby 视频流（保留 Range 以支持拖动）。
func (r *EmbyRemoteService) ProxyVideoStream(ctx context.Context, w http.ResponseWriter, req *http.Request, acct *model.StrmAccount, remoteID string) error {
	cfg, err := r.configOf(acct)
	if err != nil {
		return err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return err
	}
	endpoint := r.embyBase(cfg) + "/Videos/" + url.PathEscape(remoteID) + "/stream"
	q := url.Values{}
	if mediaSourceID := strings.TrimSpace(req.URL.Query().Get("MediaSourceId")); mediaSourceID != "" {
		q.Set("MediaSourceId", mediaSourceID)
	}
	if static := strings.TrimSpace(req.URL.Query().Get("Static")); static != "" {
		q.Set("Static", static)
	}
	q.Set("api_key", cfg.Token)
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	upstream.Header.Set("X-Emby-Token", cfg.Token)
	if rangeHeader := req.Header.Get("Range"); rangeHeader != "" {
		upstream.Header.Set("Range", rangeHeader)
	}
	resp, err := r.http.Do(upstream)
	if err != nil {
		return fmt.Errorf("连接远程 Emby 视频流失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("远程 Emby 视频流失败(%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Cache-Control"} {
		if value := resp.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK {
		w.WriteHeader(resp.StatusCode)
	} else {
		w.WriteHeader(resp.StatusCode)
	}
	if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(w, resp.Body)
	}
	return nil
}

// ProxySubtitle 反向代理远程 Emby 字幕流。
func (r *EmbyRemoteService) ProxySubtitle(ctx context.Context, w http.ResponseWriter, req *http.Request, acct *model.StrmAccount, remoteID, index string) error {
	cfg, err := r.configOf(acct)
	if err != nil {
		return err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return err
	}
	endpoint := r.embyBase(cfg) + "/Videos/" + url.PathEscape(remoteID) + "/Subtitles/" + url.PathEscape(index) + "/Stream"
	endpoint += "?api_key=" + url.QueryEscape(cfg.Token)
	upstream, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	upstream.Header.Set("X-Emby-Token", cfg.Token)
	resp, err := r.http.Do(upstream)
	if err != nil {
		return fmt.Errorf("连接远程 Emby 字幕流失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("远程 Emby 字幕流失败(%d)", resp.StatusCode)
	}
	if value := resp.Header.Get("Content-Type"); value != "" {
		w.Header().Set("Content-Type", value)
	}
	w.WriteHeader(resp.StatusCode)
	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(w, resp.Body)
	}
	return nil
}

// ─── 播放状态透传 ──────────────────────────────────────────────────────────────

// ProxySetPlayed 把「已看/未看」状态透传到远程 Emby（MMTL 本地不落库）。
func (r *EmbyRemoteService) ProxySetPlayed(ctx context.Context, acct *model.StrmAccount, remoteID string, played bool) error {
	cfg, err := r.configOf(acct)
	if err != nil {
		return err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return err
	}
	method := http.MethodPost
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/PlayedItems/" + url.PathEscape(remoteID)
	if !played {
		method = http.MethodDelete
	}
	return r.doMutate(ctx, acct, cfg, method, path)
}

// ProxySetFavorite 把「收藏/取消收藏」状态透传到远程 Emby。
func (r *EmbyRemoteService) ProxySetFavorite(ctx context.Context, acct *model.StrmAccount, remoteID string, favorite bool) error {
	cfg, err := r.configOf(acct)
	if err != nil {
		return err
	}
	if err := r.ensureToken(ctx, acct, cfg); err != nil {
		return err
	}
	method := http.MethodPost
	path := "/Users/" + url.PathEscape(r.remoteUserID(cfg)) + "/FavoriteItems/" + url.PathEscape(remoteID)
	if !favorite {
		method = http.MethodDelete
	}
	return r.doMutate(ctx, acct, cfg, method, path)
}

func (r *EmbyRemoteService) doMutate(ctx context.Context, acct *model.StrmAccount, cfg *EmbyRemoteConfig, method, path string) error {
	endpoint := r.embyBase(cfg) + path + "?api_key=" + url.QueryEscape(cfg.Token)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Emby-Token", cfg.Token)
	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求远程 Emby 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("远程 Emby 状态同步失败(%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}