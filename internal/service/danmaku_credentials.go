// Package service — 内置 dandanplay 应用凭据（签名认证回退用）。
//
// 弹弹play 开放 API（https://doc.dandanplay.com/open/）要求所有请求
// 携带应用认证。官方推荐客户端应用使用「签名验证模式」：
//
//	请求头 X-AppId + X-Timestamp + X-Signature
//	X-Signature = base64(sha256(AppId + Timestamp + Path + AppSecret))
//
// Timestamp 为 UTC 秒级 Unix 时间戳，Path 为不含域名/查询参数的请求路径。
// 该模式里 AppSecret 只存在于服务端本地，网络上传输的只有绑定
// 时间戳与路径的签名，无法重放到其它请求上——这是「隐藏密钥」真正
// 有效的部分。
//
// 为了让用户开箱即用，这里内置了一组项目自用凭据作为回退（在
// DevCenter 申请）。管理员可在「弹幕」设置页填写自己的 AppId /
// AppSecret 覆盖内置凭据。
//
// 混淆说明（重要）：开源项目无法真正隐藏随二进制分发的密钥，XOR
// 混淆只能挡住直接扫源码/复制常量这种程度的提取，挡不住反编译内存
// 取证。因此内置凭据只作为回退，不建议过度依赖它承载大流量。
package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

// dandanplayObfuscationKey 是解开内置凭据的 XOR 混淆密钥。它与密文
// 同处一个二进制，仅作提取门槛，不作安全边界。
const dandanplayObfuscationKey = "MMTL-Danmaku#2026!v2"

// 内置回退凭据（XOR 混淆后的 hex 编码）。
const (
	danmakuEmbeddedAppIDHex  = "352c24755f7c115d0a52"
	danmakuEmbeddedAppKeyHex = "74390d751832375917150d4717586578586620652a05043d5e320f0d0b120d38"
)

// danmakuEmbeddedCredentials 解出内置回退凭据。
func danmakuEmbeddedCredentials() (appID, appKey string) {
	return xorDecode(danmakuEmbeddedAppIDHex), xorDecode(danmakuEmbeddedAppKeyHex)
}

// xorDecode 用 dandanplayObfuscationKey 逐字节解开 hex 密文。
func xorDecode(hexStr string) string {
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return ""
	}
	key := []byte(dandanplayObfuscationKey)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ key[i%len(key)]
	}
	return string(out)
}

// dandanplaySignature 计算开放 API 请求签名：
// base64(sha256(AppId + Timestamp + Path + AppSecret))。
// path 只含请求路径（不含域名与查询参数、小写、不 URL 编码）。
func dandanplaySignature(appID, appSecret string, ts int64, path string) string {
	sum := sha256.Sum256([]byte(appID + strconv.FormatInt(ts, 10) + path + appSecret))
	return base64.StdEncoding.EncodeToString(sum[:])
}
