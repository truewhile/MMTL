// STRM 播放端点解析：strm 文件内容指向 /api/strm/play/{provider}，服务端
// 依据账号凭据解析出直链：能 302 的走 302（115/OpenList API），需要携带请求
// 头（CloudDrive2 WebDAV）的走反向代理；本地源直接以静态文件方式提供。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/service/cloud"
)

// ErrStrmPlayNotFound 表示 strm 播放目标不存在（handler 返回 404）。
var ErrStrmPlayNotFound = errors.New("strm play target not found")

// StrmPlayResult 是播放解析结果。
type StrmPlayResult struct {
	// RedirectURL 非空时 handler 直接 302 到该地址。
	RedirectURL string
	// LocalPath 非空时 handler 以静态文件方式提供（本地源）。
	LocalPath string
	// Link 非空且 Proxy 为 true 时 handler 反向代理该直链。
	Link  *cloud.DirectLink
	Proxy bool
}

// ResolvePlay 解析 strm 播放请求。
func (s *StrmService) ResolvePlay(ctx context.Context, provider string, q url.Values) (*StrmPlayResult, error) {
	switch provider {
	case model.StrmProviderLocal:
		return s.resolveLocalPlay(ctx, q.Get("path"))
	case model.StrmProvider115:
		return s.resolveCloudPlay(ctx, provider, q, "pickcode")
	case model.StrmProviderCloudDrive, model.StrmProviderOpenList:
		return s.resolveCloudPlay(ctx, provider, q, "ref")
	default:
		return nil, errors.New("未知的 STRM 提供方")
	}
}

func (s *StrmService) resolveCloudPlay(ctx context.Context, provider string, q url.Values, refKey string) (*StrmPlayResult, error) {
	acctID := q.Get("acct")
	ref := q.Get(refKey)
	if acctID == "" || ref == "" {
		return nil, fmt.Errorf("缺少 %s 参数", refKey)
	}
	acct, err := s.repo.StrmAccount.FindByID(ctx, acctID)
	if err != nil || acct == nil {
		return nil, errors.New("网盘账号不存在")
	}
	if !acct.Enabled {
		return nil, errors.New("网盘账号已禁用")
	}
	if acct.Provider != provider {
		return nil, errors.New("网盘账号类型不匹配")
	}
	p, err := s.providerFor(ctx, acct)
	if err != nil {
		return nil, err
	}
	var link *cloud.DirectLink
	ua := q.Get("__ua")
	if uaProvider, ok := p.(interface {
		ResolveWithUA(ctx context.Context, fileRef, ua string) (*cloud.DirectLink, error)
	}); ok && ua != "" {
		link, err = uaProvider.ResolveWithUA(ctx, ref, ua)
	} else {
		link, err = p.Resolve(ctx, ref)
	}
	if err != nil {
		return nil, err
	}
	if link == nil || link.URL == "" {
		return nil, errors.New("解析播放地址失败")
	}
	if link.Proxy {
		return &StrmPlayResult{Link: link, Proxy: true}, nil
	}
	// Link 一并保留：302 处理器只认 RedirectURL，但服务端直连（弹幕 hash
	// 拉取）需要 link.Headers 才能通过直链防盗链校验。
	return &StrmPlayResult{RedirectURL: link.URL, Link: link}, nil
}

// resolveLocalPlay 本地源：校验路径位于某个本地同步目录的源目录内。
func (s *StrmService) resolveLocalPlay(ctx context.Context, rawPath string) (*StrmPlayResult, error) {
	if rawPath == "" {
		return nil, errors.New("缺少 path 参数")
	}
	target := filepath.Clean(rawPath)
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return nil, ErrStrmPlayNotFound
	}
	paths, err := s.repo.StrmSyncPath.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range paths {
		p := &paths[i]
		if p.Provider != model.StrmProviderLocal || !p.Enabled || strings.TrimSpace(p.RemotePath) == "" {
			continue
		}
		root := filepath.Clean(p.RemotePath)
		if target == root || strings.HasPrefix(target, root+string(filepath.Separator)) {
			return &StrmPlayResult{LocalPath: target}, nil
		}
	}
	return nil, errors.New("文件不在任何本地同步目录内")
}

// ResolvePlayTarget 解析媒体行固化的播放目标（STRMURL 或 .strm 文件内容）为
// 可播放结果，供弹幕 hash、内嵌字幕提取等「先解析直链再读取远端」的场景复用。
// 支持：
//   - /api/strm/play/{provider}/video{ext}?acct=..&pickcode=.. （常规格式，含账号）
//   - /api/cloud/play/{type}?ref=.. （旧格式，无账号 → 取该类型第一个启用账号）
//   - 绝对 http(s) 链接（直接透传）
//   - 其余协议（webdav:// 等）返回错误，由调用方决定是否静默跳过
func (s *StrmService) ResolvePlayTarget(ctx context.Context, raw string) (*StrmPlayResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("空播放目标")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("解析播放目标失败: %w", err)
	}
	lowerPath := strings.ToLower(u.Path)
	switch {
	case strings.HasPrefix(lowerPath, "/api/strm/play/"):
		segs := strings.Split(strings.TrimPrefix(u.Path, "/api/strm/play/"), "/")
		if len(segs) < 1 || strings.TrimSpace(segs[0]) == "" {
			return nil, errors.New("无效的 strm 播放地址")
		}
		return s.ResolvePlay(ctx, segs[0], u.Query())
	case strings.HasPrefix(lowerPath, "/api/cloud/play/"):
		typ := strings.TrimSpace(strings.TrimPrefix(u.Path, "/api/cloud/play/"))
		acct, err := s.firstEnabledAccountOf(ctx, typ)
		if err != nil || acct == nil {
			return nil, errors.New("没有可用的网盘账号，无法解析直链")
		}
		q := u.Query()
		q.Set("acct", acct.ID)
		return s.ResolvePlay(ctx, typ, q)
	case u.Scheme == "http" || u.Scheme == "https":
		return &StrmPlayResult{RedirectURL: raw}, nil
	default:
		return nil, fmt.Errorf("不支持的播放目标协议: %s", u.Scheme)
	}
}

// firstEnabledAccountOf 返回指定提供方第一个凭据可用的启用账号。
func (s *StrmService) firstEnabledAccountOf(ctx context.Context, provider string) (*model.StrmAccount, error) {
	accounts, err := s.repo.StrmAccount.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		a := &accounts[i]
		if !a.Enabled || a.Provider != provider {
			continue
		}
		if _, err := s.providerFor(ctx, a); err == nil {
			return a, nil
		}
	}
	return nil, nil
}

// ProxyDirect 反向代理渲染直链内容（保留 Range 请求头以支持拖动播放）。
func (s *StrmService) ProxyDirect(ctx context.Context, w http.ResponseWriter, r *http.Request, link *cloud.DirectLink) error {
	if link == nil || link.URL == "" {
		return errors.New("空直链")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range link.Headers {
		req.Header.Set(k, v)
	}
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag"} {
		if value := resp.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(resp.StatusCode)
	}
	if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(w, resp.Body)
	}
	return nil
}
