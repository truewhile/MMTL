// 115 开放平台（openapi）驱动：替代原 cookie 逆向方案。
//
// 账号配置（StrmAccount.Config JSON）：
//
//	{
//	  "app_id":         "100195125",      // 开放平台应用 ID
//	  "access_token":   "...",            // 加密存储
//	  "refresh_token":  "...",            // 加密存储
//	  "user_id":        "12345",          // 可选
//	  "user_name":      "user"            // 可选
//	}
//
// 授权流程：设备码扫码（官方 PKCE 应用目录）、QMediaSync/MQFamily 中继、
// MoviePilot 轮询、CloudDrive 回跳，见 internal/service/cloud115。
package cloud

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ShukeBta/MMTL/internal/service/cloud115"
)

// OpenAPI115Provider 暴露 115 开放平台驱动接口。
type OpenAPI115Provider interface {
	Provider
	OpenClient() *cloud115.OpenClient
}

// openAPI115Provider 实现 Provider 接口：List 列目录、Resolve 用 pickcode
// 换下载直链（302 offload，无需代理）、Ping 探测根目录。
type openAPI115Provider struct {
	c *cloud115.OpenClient
}

// NewOpenAPI115 构造 115 开放平台驱动。
func NewOpenAPI115(appID, accessToken, refreshToken string) *openAPI115Provider {
	return &openAPI115Provider{c: cloud115.NewOpenClient(strings.TrimSpace(appID), accessToken, refreshToken)}
}

func (p *openAPI115Provider) Type() string { return Type115 }

func (p *openAPI115Provider) Ping(ctx context.Context) error {
	if strings.TrimSpace(p.c.AppID) == "" {
		return fmt.Errorf("115: 缺少开放平台应用 ID，请重新授权")
	}
	if strings.TrimSpace(p.c.AccessToken) == "" {
		return fmt.Errorf("115: 缺少访问令牌，请重新授权")
	}
	_, _, err := p.c.GetFsList(ctx, "0", 0, 1)
	return err
}

func (p *openAPI115Provider) List(ctx context.Context, dirID string) ([]FileEntry, error) {
	// 115 开放平台列表接口按 offset/limit 分页，这里循环取完整个目录
	const pageSize = 100
	var out []FileEntry
	for offset := 0; ; offset += pageSize {
		files, _, err := p.c.GetFsList(ctx, dirID, offset, pageSize)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			out = append(out, FileEntry{
				ID:       f.FileId,
				Name:     f.FileName,
				IsDir:    f.Category == cloud115.TypeDir,
				Size:     f.FileSize,
				MTime:    f.Utime,
				PickCode: f.PickCode,
			})
		}
		if len(files) < pageSize {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	return out, nil
}

func (p *openAPI115Provider) Resolve(ctx context.Context, fileRef string) (*DirectLink, error) {
	return p.ResolveWithUA(ctx, fileRef, "")
}

func (p *openAPI115Provider) ResolveWithUA(ctx context.Context, fileRef, ua string) (*DirectLink, error) {
	url, err := p.c.GetDownloadURLWithUA(ctx, fileRef, ua)
	if err != nil {
		return nil, err
	}
	// 115 CDN 防盗链白名单：直链绑定换取时的 User-Agent（调用方 UA 或
	// DefaultUA），后续请求必须携带同一 UA，否则 403。302 播放由客户端
	// 自带 UA 天然满足；服务端直连（弹幕 hash 等）依赖这里的 Headers。
	bound := strings.TrimSpace(ua)
	if bound == "" {
		bound = cloud115.DefaultUA
	}
	return &DirectLink{URL: url, Proxy: false, Headers: map[string]string{"User-Agent": bound}}, nil
}

// OpenClient 暴露底层客户端（token 刷新用）。
func (p *openAPI115Provider) OpenClient() *cloud115.OpenClient { return p.c }

// PutFileNamed 把本地元数据上传到 115 指定父目录（parentCID 为父目录 cid）。
// io.Reader 无法携带文件名，因此走独立的 named 上传接口。将内容落为临时文件后
// 重命名为目标文件名，再交给 115 上传（/open/upload/init 的 file_name 取真实文件名）。
func (p *openAPI115Provider) PutFileNamed(ctx context.Context, parentCID, fileName string, r io.Reader) error {
	tmp, err := os.CreateTemp("", "mmtl-upload-*")
	if err != nil {
		return fmt.Errorf("115: 创建临时文件失败：%w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("115: 写入临时文件失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("115: 关闭临时文件失败：%w", err)
	}
	// 重命名为目标文件名，保证上传到 115 后保留原始文件名
	if fileName != "" && fileName != filepath.Base(tmpPath) {
		namedPath := filepath.Join(filepath.Dir(tmpPath), fileName)
		if err := os.Rename(tmpPath, namedPath); err == nil {
			tmpPath = namedPath
		}
	}
	_, err = p.c.Upload(ctx, tmpPath, parentCID, "", "")
	if err != nil {
		return err
	}
	return nil
}

// RefreshToken 刷新访问令牌并返回新令牌；refresh_token 失效时返回
// cloud115.IsRefreshTokenDead(err) 为 true 的错误。
func (p *openAPI115Provider) RefreshToken(refreshToken string) (*cloud115.TokenData, error) {
	return p.c.RefreshToken(refreshToken)
}
