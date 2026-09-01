package service

import (
	"net/url"
	"testing"
)

// TestNormalizeCloudPlayTarget 验证存库的云盘播放 URL（可能携带扫描时的
// 旧 host）被规范化为相对路径，使 302 始终基于当前请求地址构造。
func TestNormalizeCloudPlayTarget(t *testing.T) {
	ref := "/电影/某部影片 (2024)/movie.mkv"
	stale := "http://192.168.1.4:9011/api/cloud/play/openlist?ref=" + url.QueryEscape(ref)
	got := normalizeCloudPlayTarget(stale)
	want := BuildRelativeCloudPlayURL("openlist", ref)
	if got != want {
		t.Fatalf("normalizeCloudPlayTarget = %q, want %q", got, want)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.IsAbs() || parsed.Host != "" {
		t.Fatalf("normalized target should be relative, got %q", got)
	}
	if parsed.Query().Get("ref") != ref {
		t.Fatalf("ref round-trip failed: %q", parsed.Query().Get("ref"))
	}

	strmStale := "http://bwg.linkmy.fun:1314/api/strm/play/cloud115/video.mkv?acct=abc&pickcode=123"
	gotStrm := normalizeCloudPlayTarget(strmStale)
	wantStrm := "/api/strm/play/cloud115/video.mkv?acct=abc&pickcode=123"
	if gotStrm != wantStrm {
		t.Fatalf("normalizeCloudPlayTarget(strm) = %q, want %q", gotStrm, wantStrm)
	}

	// 非云盘播放 URL 保持原样（WebDAV/直链等）。
	passthrough := "https://dav.example.com/media/file.mkv"
	if got := normalizeCloudPlayTarget(passthrough); got != passthrough {
		t.Fatalf("non-cloud target should pass through, got %q", got)
	}
}
