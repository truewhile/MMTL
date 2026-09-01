package service

import (
	"testing"
)

// rewriteSubtitleDeliveryURLs 只应改动字幕轨道的 DeliveryUrl，其余媒体流不动。
func TestRewriteSubtitleDeliveryURLsProxyMode(t *testing.T) {
	src := map[string]any{
		"MediaStreams": []any{
			map[string]any{"Type": "Video", "DeliveryUrl": "/Videos/x/stream"},
			map[string]any{"Type": "Audio", "DeliveryUrl": "/Videos/x/stream"},
			map[string]any{"Type": "Subtitle", "DeliveryUrl": "/Videos/item-1/ms-9/Subtitles/2/Stream.srt"},
		},
	}
	rewriteSubtitleDeliveryURLs(src, "/Videos/embyremote~acct-1~item-1", &EmbyRemoteConfig{})
	streams := src["MediaStreams"].([]any)
	if got := streams[0].(map[string]any)["DeliveryUrl"]; got != "/Videos/x/stream" {
		t.Fatalf("video DeliveryUrl must stay, got %v", got)
	}
	want := "/Videos/embyremote~acct-1~item-1/Subtitles/2/Stream.srt"
	if got := streams[2].(map[string]any)["DeliveryUrl"]; got != want {
		t.Fatalf("subtitle DeliveryUrl = %v, want %v", got, want)
	}
}

func TestRewriteSubtitleDeliveryURLsDirectMode(t *testing.T) {
	src := map[string]any{
		"MediaStreams": []any{
			map[string]any{"Type": "Subtitle", "DeliveryUrl": "/Videos/item-1/ms-9/Subtitles/1/Stream.ass"},
		},
	}
	cfg := &EmbyRemoteConfig{Token: "tok123"}
	rewriteSubtitleDeliveryURLs(src, "http://remote:8096/emby/Videos/item-1", cfg)
	streams := src["MediaStreams"].([]any)
	want := "http://remote:8096/emby/Videos/item-1/Subtitles/1/Stream.ass?api_key=tok123"
	if got := streams[0].(map[string]any)["DeliveryUrl"]; got != want {
		t.Fatalf("subtitle DeliveryUrl = %v, want %v", got, want)
	}
}

func TestRewriteSubtitleDeliveryURLsFallsBackIndexOne(t *testing.T) {
	src := map[string]any{
		"MediaStreams": []any{
			map[string]any{"Type": "Subtitle", "DeliveryUrl": "custom/url"},
		},
	}
	rewriteSubtitleDeliveryURLs(src, "/Videos/embyremote~acct-1~item-1", &EmbyRemoteConfig{})
	streams := src["MediaStreams"].([]any)
	want := "/Videos/embyremote~acct-1~item-1/Subtitles/1/Stream"
	if got := streams[0].(map[string]any)["DeliveryUrl"]; got != want {
		t.Fatalf("subtitle DeliveryUrl = %v, want %v", got, want)
	}
}