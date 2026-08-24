// 115 开放平台 HTTP 客户端（移植自 QMediaSync 的 v115open，去掉 resty 依赖，
// 使用 net/http + 简单限流重试；只保留只读能力：授权/列目录/详情/下载直链）。
package cloud115

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// OpenClient 是 115 开放平台客户端。
type OpenClient struct {
	AppID           string
	HTTP            *http.Client
	AccessToken     string
	RefreshTokenStr string

	// 全局 QPS 限流（115 开放平台免费额度较低）
	lastSecond  int64
	reqInSecond int64
}

var openClientMu sync.Mutex

// NewOpenClient 构造客户端。
func NewOpenClient(appID, accessToken, refreshToken string) *OpenClient {
	return &OpenClient{
		AppID:           appID,
		HTTP:            &http.Client{Timeout: 60 * time.Second},
		AccessToken:     accessToken,
		RefreshTokenStr: refreshToken,
	}
}

// SetAuthToken 更新认证令牌。
func (c *OpenClient) SetAuthToken(accessToken, refreshToken string) {
	c.AccessToken = accessToken
	c.RefreshTokenStr = refreshToken
}

// throttle 简单的每秒限流（默认 4 QPS，115 免费应用限额约 5 QPS）。
func (c *OpenClient) throttle(n int) {
	for i := 0; i < n; i++ {
		now := time.Now().Unix()
		last := atomic.LoadInt64(&c.lastSecond)
		if last != now {
			if atomic.CompareAndSwapInt64(&c.lastSecond, last, now) {
				atomic.StoreInt64(&c.reqInSecond, 0)
			}
		}
		count := atomic.LoadInt64(&c.reqInSecond)
		if count >= 4 {
			time.Sleep(300 * time.Millisecond)
			i--
			continue
		}
		if atomic.CompareAndSwapInt64(&c.reqInSecond, count, count+1) {
			return
		}
		time.Sleep(50 * time.Millisecond)
		i--
	}
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
	Data    json.RawMessage `json:"data"`
	Raw     json.RawMessage `json:"-"` // 原始响应体（外层附加字段用）
}

// doJSON 执行 GET 请求并解析为统一响应；带 AccessToken（access=true 时）。
func (c *OpenClient) doJSON(ctx context.Context, method, rawURL string, form map[string]string, access bool, retries int) (*RespBase, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		c.throttle(1)
		req, err := c.buildRequest(ctx, method, rawURL, form, access)
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if attempt < retries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
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
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("115 接口返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
			if attempt < retries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
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
		// 业务失败：限流/Token 错误不重试，其余按配置重试
		if IsThrottleCode(base.Code) || isTokenCode(base.Code) {
			return &base, nil
		}
		lastErr = NewOpenAPIResponseError(base.Code, base.Errno, base.Message, base.Error, "115 接口调用失败")
		if attempt < retries {
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		return &base, lastErr
	}
	return nil, lastErr
}

func (c *OpenClient) buildRequest(ctx context.Context, method, rawURL string, form map[string]string, access bool) (*http.Request, error) {
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
	req.Header.Set("User-Agent", DefaultUA)
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
