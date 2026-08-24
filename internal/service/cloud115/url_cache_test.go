package cloud115

import (
	"testing"
	"time"
)

func TestDownloadURLCache(t *testing.T) {
	urlCacheMu.Lock()
	urlCache = map[string]urlCacheEntry{}
	urlCacheMu.Unlock()

	if got := GetDownloadURLCache("pick1"); got != "" {
		t.Fatalf("empty cache should return empty, got %q", got)
	}
	SetDownloadURLCache("pick1", "http://cdn/1")
	if got := GetDownloadURLCache("pick1"); got != "http://cdn/1" {
		t.Fatalf("cache miss, got %q", got)
	}
	// 过期条目按未命中处理
	urlCacheMu.Lock()
	urlCache["pick1"] = urlCacheEntry{url: "http://cdn/old", expiresAt: time.Now().Add(-time.Second)}
	urlCacheMu.Unlock()
	if got := GetDownloadURLCache("pick1"); got != "" {
		t.Fatalf("expired entry should be a miss, got %q", got)
	}
	ClearDownloadURLCache("pick1")
	if got := GetDownloadURLCache("pick1"); got != "" {
		t.Fatalf("cleared entry should be a miss, got %q", got)
	}
}
