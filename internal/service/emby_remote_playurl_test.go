package service

import (
	"testing"

	"github.com/ShukeBta/MMTL/internal/model"
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

func TestMapRemoteItemToMediaExtractsCodecsAndContainer(t *testing.T) {
	r := &EmbyRemoteService{}
	item := map[string]any{
		"Id":        "item-100",
		"Name":      "Test Movie",
		"Container": "mkv",
		"MediaStreams": []any{
			map[string]any{
				"Type":   "Video",
				"Codec":  "h264",
				"Width":  1920,
				"Height": 1080,
			},
			map[string]any{
				"Type":  "Audio",
				"Codec": "aac",
			},
		},
		"MediaSources": []any{
			map[string]any{
				"Container": "mkv",
				"Size":      int64(104857600),
			},
		},
	}
	media := r.MapRemoteItemToMedia(t.Context(), nil, &model.StrmAccount{Base: model.Base{ID: "acct-1"}}, &EmbyRemoteConfig{}, item)
	if media.Container != "mkv" {
		t.Fatalf("media.Container = %v, want mkv", media.Container)
	}
	if media.VideoCodec != "h264" {
		t.Fatalf("media.VideoCodec = %v, want h264", media.VideoCodec)
	}
	if media.AudioCodec != "aac" {
		t.Fatalf("media.AudioCodec = %v, want aac", media.AudioCodec)
	}
	if media.Width != 1920 || media.Height != 1080 {
		t.Fatalf("resolution = %dx%d, want 1920x1080", media.Width, media.Height)
	}
	if media.SizeBytes != 104857600 {
		t.Fatalf("size = %d, want 104857600", media.SizeBytes)
	}
}