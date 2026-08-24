package service

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// 内置回退凭据必须能解出有效的 AppId（AppId 非机密），且与密文
// XOR 回环一致（AppKey 以回环校验完整性，避免把明文密钥再抄一遍）。
func TestDanmakuEmbeddedCredentials(t *testing.T) {
	appID, appKey := danmakuEmbeddedCredentials()
	require.Equal(t, "xap9r8p3g3", appID)
	require.NotEmpty(t, appKey)
	require.GreaterOrEqual(t, len(appKey), 24)

	// 回环：解出的明文再用同一混淆密钥 XOR 后必须还原出原密文，
	// 否则说明密文与密钥失配（改了一边忘了另一边）。
	require.Equal(t, danmakuEmbeddedAppIDHex, xorEncode(appID, dandanplayObfuscationKey))
	require.Equal(t, danmakuEmbeddedAppKeyHex, xorEncode(appKey, dandanplayObfuscationKey))
}

// xorEncode 是 xorDecode 的逆操作（测试辅助，与生产实现同规则）。
func xorEncode(plain, key string) string {
	out := make([]byte, len(plain))
	for i := range plain {
		out[i] = plain[i] ^ key[i%len(key)]
	}
	return hex.EncodeToString(out)
}

// 签名向量：算法 base64(sha256(AppId+Timestamp+Path+AppSecret))，
// 用固定时间戳与路径交叉验证实现与文档一致（向量由独立脚本生成）。
func TestDandanplaySignatureVectors(t *testing.T) {
	appID, appKey := danmakuEmbeddedCredentials()
	const ts = int64(1700000000)
	vectors := map[string]string{
		"/api/v2/comment/25484":   "p3OJPfcsm0aFUUXzUTIoKA3vo9fUUtpZRV7/fqX0t0Y=",
		"/api/v2/search/episodes": "x9Wr1tPWmeXAT8UeRK2eut9NRofOPsbp5qEl/uqXHC0=",
	}
	for path, want := range vectors {
		require.Equal(t, want, dandanplaySignature(appID, appKey, ts, path), "path=%s", path)
	}
}

// 凭据解析：官方域名 + 未配置 → 内置回退；配置了 → 用户凭据优先；
// 只配一半 → 回退内置；第三方源一律不携带凭据。
func TestDanmakuCredentialsSelection(t *testing.T) {
	svc := newDanmakuTestService(t)
	ctx := context.Background()
	official := "https://api.dandanplay.net/api/v2/comment/25484?withRelated=true"
	thirdParty := "https://dm.example.com/api/v2/comment/25484"

	embedID, embedKey := danmakuEmbeddedCredentials()

	// 1) 官方域名、未配置：内置回退。
	id, key, ok := svc.danmakuCredentials(ctx, official)
	require.True(t, ok)
	require.Equal(t, embedID, id)
	require.Equal(t, embedKey, key)

	// 2) 官方域名、配置完整：用户凭据覆盖内置。
	require.NoError(t, svc.repo.Setting.Set(ctx, DanmakuAppIDKey, "my-app-id"))
	require.NoError(t, svc.repo.Setting.Set(ctx, DanmakuAppKeyKey, "my-app-key"))
	id, key, ok = svc.danmakuCredentials(ctx, official)
	require.True(t, ok)
	require.Equal(t, "my-app-id", id)
	require.Equal(t, "my-app-key", key)

	// 3) 只配一个：视为不完整，回退内置。
	require.NoError(t, svc.repo.Setting.Delete(ctx, DanmakuAppKeyKey))
	id, key, ok = svc.danmakuCredentials(ctx, official)
	require.True(t, ok)
	require.Equal(t, embedID, id)
	require.Equal(t, embedKey, key)

	// 4) 第三方源：即使配了凭据也不发送（内置凭据更不能外泄）。
	id, key, ok = svc.danmakuCredentials(ctx, thirdParty)
	require.False(t, ok)
	require.Empty(t, id)
	require.Empty(t, key)
}