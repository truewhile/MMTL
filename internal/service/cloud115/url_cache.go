// 下载直链进程级缓存。
//
// 115 的 /open/ufile/downurl 换取结果在一段时间内有效，且换取请求是
// WAF 风控重点关注的密集接口。缓存按 pickcode 存直链、避免对同一文件
// 反复换取；下载失败（http 403/404 等）时由调用方主动清除对应条目，
// 下一轮重试会重新换取链接。
package cloud115

import (
	"strings"
	"sync"
	"time"
)

// downloadURLCacheTTL 直链缓存时长。115 直链一般有效 1 小时左右，
// 取 45 分钟留出余量。QMediaSync 同类缓存使用 50 分钟。
const downloadURLCacheTTL = 45 * time.Minute

// maxCachedURLs 超过该数量时顺带清理过期条目，防止 map 无限膨胀。
const maxCachedURLs = 10000

type urlCacheEntry struct {
	url       string
	expiresAt time.Time
}

var (
	urlCacheMu sync.Mutex
	urlCache   = map[string]urlCacheEntry{}
)

// urlCacheKey 构造缓存键名（pickCode + UA，实现直链防盗链按客户端 UA 独立缓存）。
func urlCacheKey(pickCode, ua string) string {
	if ua == "" {
		return pickCode
	}
	return pickCode + "@" + ua
}

// GetDownloadURLCache 返回未过期的缓存直链；不存在或已过期返回空串。
func GetDownloadURLCache(pickCode string, uas ...string) string {
	if pickCode == "" {
		return ""
	}
	ua := ""
	if len(uas) > 0 {
		ua = uas[0]
	}
	key := urlCacheKey(pickCode, ua)
	urlCacheMu.Lock()
	defer urlCacheMu.Unlock()
	entry, ok := urlCache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(urlCache, key)
		}
		return ""
	}
	return entry.url
}

// SetDownloadURLCache 写入直链缓存。
func SetDownloadURLCache(pickCode string, url string, uas ...string) {
	if pickCode == "" || url == "" {
		return
	}
	ua := ""
	if len(uas) > 0 {
		ua = uas[0]
	}
	urlCacheMu.Lock()
	defer urlCacheMu.Unlock()
	if len(urlCache) >= maxCachedURLs {
		now := time.Now()
		for k, e := range urlCache {
			if now.After(e.expiresAt) {
				delete(urlCache, k)
			}
		}
	}
	key := urlCacheKey(pickCode, ua)
	urlCache[key] = urlCacheEntry{url: url, expiresAt: time.Now().Add(downloadURLCacheTTL)}
}

// ClearDownloadURLCache 删除指定 pickcode 的所有缓存（下载得到非 2xx 时调用）。
func ClearDownloadURLCache(pickCode string) {
	if pickCode == "" {
		return
	}
	urlCacheMu.Lock()
	defer urlCacheMu.Unlock()
	for k := range urlCache {
		if k == pickCode || strings.HasPrefix(k, pickCode+"@") {
			delete(urlCache, k)
		}
	}
}
