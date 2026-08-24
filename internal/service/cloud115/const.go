// 115 开放平台常量与错误类型。
package cloud115

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

func sha256Sum(data []byte) [32]byte     { return sha256.Sum256(data) }
func base64StdEncode(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// 115 开放平台 API 地址（变量便于测试注入 mock server）
var (
	ProAPIBase      = "https://proapi.115.com"
	PassportAPIBase = "https://passportapi.115.com"
	QRCodeAPIBase   = "https://qrcodeapi.115.com"
	FSPIsAPIBase    = "https://fsapi.115.com"
)

const (
	// 业务错误码
	AccessTokenAuthFail   = 40140126 // 访问过期，需刷新
	AccessTokenExpiryCode = 40140125 // 访问过期，需刷新
	AccessAuthInvalid     = 40140124 // 访问无效，需刷新
	RefreshTokenInvalid   = 40140116 // 需重新授权
	TokenRefreshFail      = 40140121 // 刷新失败，可重试
	RequestMaxLimitCode   = 770004   // 访问频率过高
	RequestRateLimitCode  = 406      // 达到访问上限

	// 刷新 token 的错误码
	RefreshTokenFormatInvalid = 40140114
	RefreshTokenSignInvalid   = 40140115
	RefreshTooFrequent        = 40140117
	RefreshTokenExpired       = 40140119
	RefreshTokenCheckFailed   = 40140120
)

// DefaultUA 请求 UA。
const DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 MMTL-115-OpenAPI/1.0"

// OpenAPIError 保留 115 开放平台返回的原始错误信息。
type OpenAPIError struct {
	Code    int
	Message string
}

func (e *OpenAPIError) Error() string {
	if e.Code == 0 {
		return fmt.Sprintf("115 接口错误：%s", e.Message)
	}
	return fmt.Sprintf("115 接口错误（%d）：%s", e.Code, e.Message)
}

// NewOpenAPIResponseError 组装接口错误。
func NewOpenAPIResponseError(code, errno int, message, errorText, fallback string) error {
	if code == 0 {
		code = errno
	}
	if message == "" {
		message = errorText
	}
	if code == 0 && message == "" {
		return fmt.Errorf("%s", fallback)
	}
	return NewOpenAPIError(code, message)
}

// NewOpenAPIError 构造接口错误。
func NewOpenAPIError(code int, message string) *OpenAPIError {
	if message == "" {
		message = "未知错误"
	}
	return &OpenAPIError{Code: code, Message: message}
}

// IsRefreshTokenDead 判断 refresh_token 是否已无法继续使用。
func IsRefreshTokenDead(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *OpenAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case RefreshTokenFormatInvalid, RefreshTokenSignInvalid, RefreshTokenInvalid, RefreshTokenExpired, RefreshTokenCheckFailed:
			return true
		}
	}
	return false
}

// genCodeChallenge 生成 PKCE code_challenge（sha256(codeVerifier) base64）。
func genCodeChallenge(codeVerifier string) string {
	sum := sha256Sum([]byte(codeVerifier))
	return base64StdEncode(sum[:])
}
