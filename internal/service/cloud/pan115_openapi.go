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
	"strings"

	"github.com/ShukeBta/MediaStationGo/internal/service/cloud115"
)

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
	url, err := p.c.GetDownloadURL(ctx, fileRef)
	if err != nil {
		return nil, err
	}
	return &DirectLink{URL: url, Proxy: false}, nil
}

// OpenClient 暴露底层客户端（token 刷新用）。
func (p *openAPI115Provider) OpenClient() *cloud115.OpenClient { return p.c }

// RefreshToken 刷新访问令牌并返回新令牌；refresh_token 失效时返回
// cloud115.IsRefreshTokenDead(err) 为 true 的错误。
func (p *openAPI115Provider) RefreshToken(refreshToken string) (*cloud115.TokenData, error) {
	return p.c.RefreshToken(refreshToken)
}
