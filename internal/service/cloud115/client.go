// 115 开放平台 HTTP 客户端（集成全局三级令牌桶限流、熔断保护与防 405 重定向策略）。
package cloud115

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OpenClient 是 115 开放平台客户端。
type OpenClient struct {
	AppID           string
	HTTP            *http.Client
	AccessToken     string
	RefreshTokenStr string
	executor        *QueueExecutor

	// tokenMu 保护令牌刷新：业务请求中途 access_token 失效时自动刷新重试，
	// 多 goroutine（同步列表 + 下载队列）并发下只允许一次刷新进行。
	tokenMu sync.Mutex
}

// default115HTTPClient 创建带有防 405 重定向保护的 http.Client。
func default115HTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		// 防止 Go 标准库在遇到 301/302/307 重定向时将 POST 降级为 GET 导致 115 报 405 Method Not Allowed
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			// 如果原请求是 POST，重定向时不允许静默转为 GET（直接终止自动重定向，由业务层处理响应）
			if len(via) > 0 && via[len(via)-1].Method == http.MethodPost {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// NewOpenClient 构造客户端。
func NewOpenClient(appID, accessToken, refreshToken string) *OpenClient {
	return &OpenClient{
		AppID:           appID,
		HTTP:            default115HTTPClient(),
		AccessToken:     accessToken,
		RefreshTokenStr: refreshToken,
		executor:        GetGlobalExecutor(),
	}
}

// SetAuthToken 更新认证令牌。
func (c *OpenClient) SetAuthToken(accessToken, refreshToken string) {
	c.AccessToken = accessToken
	c.RefreshTokenStr = refreshToken
}

// RespState 兼容 115 不同端点返回的 state 类型（proapi 返回布尔、passport 返回数字）。
type RespState bool

func (s *RespState) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", "1":
		*s = true
		return nil
	case "false", "0", "null", "":
		*s = false
		return nil
	}
	var n float64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("115: 无法解析 state 字段 %s", string(data))
	}
	*s = n != 0
	return nil
}

// RespBase 是 115 开放平台统一响应外壳。
type RespBase struct {
	State   RespState       `json:"state"`
	Code    int             `json:"code"`
	Errno   int             `json:"errno"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Count   int64           `json:"count"`
	Data    json.RawMessage `json:"data"`
	Raw     json.RawMessage `json:"-"` // 原始响应体（外层附加字段用）
}

// doJSON 执行 HTTP 请求并解析为统一响应；带 AccessToken（access=true 时）。
func (c *OpenClient) doJSON(ctx context.Context, method, rawURL string, form map[string]string, access bool, retries int, uas ...string) (*RespBase, error) {
	executor := c.executor
	if executor == nil {
		executor = GetGlobalExecutor()
	}
	ua := ""
	if len(uas) > 0 {
		ua = uas[0]
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		// 1. 获取全局三级令牌桶（QPS/QPM/QPH）令牌，若在熔断状态则阻塞等待冷却
		if err := executor.Acquire(ctx); err != nil {
			return nil, err
		}

		req, err := c.buildRequestWithUA(ctx, method, rawURL, form, access, ua)
		if err != nil {
			return nil, err
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt < retries {
				time.Sleep(time.Duration(attempt+1) * 1 * time.Second)
				continue
			}
			return nil, err
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		// 检查 HTTP 状态码：特别处理 405/406/429 等限流和异常阻断
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotAcceptable || resp.StatusCode == http.StatusTooManyRequests {
				// 触发全局熔断冷却，防止短时间连续重试导致 IP 被完全拉黑
				executor.MarkThrottled()
				lastErr = fmt.Errorf("115 接口触发频控/安全拦截（HTTP %d）：%s", resp.StatusCode, strings.TrimSpace(string(body)))
				if attempt < retries {
					// 阻塞等待 115 限流冷却解封后自动重试当前请求
					if waitErr := executor.throttleManager.WaitThrottleRecovery(ctx); waitErr != nil {
						return nil, waitErr
					}
					continue
				}
				return nil, lastErr
			}
			lastErr = fmt.Errorf("115 接口返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
			if attempt < retries {
				time.Sleep(time.Duration(attempt+1) * 1 * time.Second)
				continue
			}
			return nil, lastErr
		}

		var base RespBase
		if err := json.Unmarshal(body, &base); err != nil {
			return nil, fmt.Errorf("115 接口响应解析失败：%w", err)
		}
		base.Raw = body
		if base.State {
			return &base, nil
		}

		// 业务返回限流错误码（406 / 770004）：标记全局熔断，等待冷却后重试
		if IsThrottleCode(base.Code) {
			executor.MarkThrottled()
			lastErr = NewOpenAPIResponseError(base.Code, base.Errno, base.Message, base.Error, "115 访问频率达到上限，已进入冷却")
			if attempt < retries {
				// 阻塞等待限流冷却解封后自动重试当前请求
				if waitErr := executor.throttleManager.WaitThrottleRecovery(ctx); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			return &base, lastErr
		}

		// Token 失效（access_token 过期，如长时间同步中途过期）：自动用
		// refresh_token 刷新后重试一次。刷新失败或重试后仍失败才返回，
		// 避免长时间同步因 token 过期而整体失败。
		if isTokenCode(base.Code) {
			if access && c.tryRefreshTokenLocked() {
				continue
			}
			if access {
				// 刷新失败（或已刷新仍失败）时返回明确错误
				lastErr = NewOpenAPIResponseError(base.Code, base.Errno, base.Message, base.Error, "115: access_token 校验失败且刷新未成功")
			}
			return &base, lastErr
		}

		lastErr = NewOpenAPIResponseError(base.Code, base.Errno, base.Message, base.Error, "115 接口调用失败")
		if attempt < retries {
			time.Sleep(time.Duration(attempt+1) * 1 * time.Second)
			continue
		}
		return &base, lastErr
	}
	return nil, lastErr
}

func (c *OpenClient) buildRequest(ctx context.Context, method, rawURL string, form map[string]string, access bool) (*http.Request, error) {
	return c.buildRequestWithUA(ctx, method, rawURL, form, access, "")
}

func (c *OpenClient) buildRequestWithUA(ctx context.Context, method, rawURL string, form map[string]string, access bool, ua string) (*http.Request, error) {
	method = strings.ToUpper(method)
	var body io.Reader
	if method == http.MethodPost && len(form) > 0 {
		values := url.Values{}
		for k, v := range form {
			values.Set(k, v)
		}
		body = bytes.NewBufferString(values.Encode())
	} else if method == http.MethodGet && len(form) > 0 {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		query := u.Query()
		for k, v := range form {
			query.Set(k, v)
		}
		u.RawQuery = query.Encode()
		rawURL = u.String()
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	targetUA := DefaultUA
	if strings.TrimSpace(ua) != "" {
		targetUA = strings.TrimSpace(ua)
	}
	req.Header.Set("User-Agent", targetUA)
	if method == http.MethodPost && len(form) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if access && c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	return req, nil
}

// doAuthJSON 带 AccessToken 的业务请求。
func (c *OpenClient) doAuthJSON(ctx context.Context, method, rawURL string, form map[string]string, retries int) (*RespBase, error) {
	return c.doJSON(ctx, method, rawURL, form, true, retries)
}

// doAuthJSONWithUA 带自定义 User-Agent 的业务请求（换取直链等防盗链接口使用）。
func (c *OpenClient) doAuthJSONWithUA(ctx context.Context, method, rawURL string, form map[string]string, retries int, ua string) (*RespBase, error) {
	return c.doJSON(ctx, method, rawURL, form, true, retries, ua)
}

// tryRefreshTokenLocked 并发安全地刷新 access_token；成功返回 true（调用方
// 应使用内存中的新 token 重试原请求）。refresh_token 已失效时也会清空内存 token。
func (c *OpenClient) tryRefreshTokenLocked() bool {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	token, err := c.RefreshToken(c.RefreshTokenStr)
	if err != nil {
		if IsRefreshTokenDead(err) {
			c.SetAuthToken("", "")
		}
		return false
	}
	c.SetAuthToken(token.AccessToken, token.RefreshToken)
	return true
}

// IsThrottleCode 判断是否为限流错误码。
func IsThrottleCode(code int) bool {
	return code == RequestMaxLimitCode || code == RequestRateLimitCode
}

func isTokenCode(code int) bool {
	switch code {
	case AccessTokenAuthFail, AccessAuthInvalid, AccessTokenExpiryCode, RefreshTokenInvalid:
		return true
	}
	return false
}

// openList 解析 data 为对象或数组（StructOrArray 语义）。
func openList[T any](raw json.RawMessage) ([]T, error) {
	var single T
	if err := json.Unmarshal(raw, &single); err == nil {
		return []T{single}, nil
	}
	var arr []T
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("115: data 既不是对象也不是数组")
}

// openFirstList 取 data 的第一个元素。
func openFirstList[T any](raw json.RawMessage) (*T, error) {
	items, err := openList[T](raw)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func firstOrEmpty(m map[string]downloadURLData) downloadURLData {
	for _, v := range m {
		return v
	}
	return downloadURLData{}
}
