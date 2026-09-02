package service

import (
	"strings"
	"testing"

	"github.com/truewhile/MeBox/internal/config"
)

func TestBuildFFmpegArgs(t *testing.T) {
	base := &config.Config{}
	base.Transcoder.MaxHeight = 720
	base.Transcoder.SegmentSeconds = 4
	base.Transcoder.Realtime = true
	base.Transcoder.Threads = 2
	base.App.VAAPIDevice = "/dev/dri/renderD128"

	cases := []struct {
		name                   string
		encoder                string
		expectVCodec           string
		expectInArgs           []string
		expectNotPresetIfBlank bool
	}{
		{"software", "", "libx264", []string{"-re", "-preset", "veryfast", "-c:v", "libx264", "-threads", "2"}, false},
		{"nvenc", "nvenc", "h264_nvenc", []string{"-hwaccel", "cuda", "-c:v", "h264_nvenc", "-preset", "p4"}, false},
		{"qsv", "qsv", "h264_qsv", []string{"-hwaccel", "qsv", "-c:v", "h264_qsv"}, false},
		{"vaapi", "vaapi", "h264_vaapi", []string{"-hwaccel", "vaapi", "-vaapi_device", "/dev/dri/renderD128", "-c:v", "h264_vaapi"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *base
			cfg.Transcoder.Encoder = tc.encoder
			cfg.Transcoder.HardwareAccel = tc.encoder != ""
			args := buildFFmpegArgs(&cfg, "/x.mkv", "/o/x.m3u8", "/o/seg_%05d.ts")
			joined := strings.Join(args, " ")
			for _, frag := range tc.expectInArgs {
				if !strings.Contains(joined, frag) {
					t.Errorf("expected %q in args, got: %s", frag, joined)
				}
			}
			// vaapi has no -preset flag.
			if tc.expectNotPresetIfBlank && strings.Contains(joined, "-preset") {
				t.Errorf("vaapi should not include -preset, got: %s", joined)
			}
		})
	}
}

func TestBuildFFmpegArgsIgnoresEncoderWhenHardwareAccelDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Transcoder.Encoder = "nvenc"
	cfg.Transcoder.HardwareAccel = false
	cfg.Transcoder.MaxHeight = 720
	cfg.Transcoder.SegmentSeconds = 4
	cfg.Transcoder.Realtime = true
	cfg.Transcoder.Threads = 2

	args := buildFFmpegArgs(cfg, "/x.mkv", "/o/x.m3u8", "/o/seg_%05d.ts")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "h264_nvenc") {
		t.Fatalf("hardware disabled should not use nvenc, got: %s", joined)
	}
	if !strings.Contains(joined, "libx264") {
		t.Fatalf("hardware disabled should fall back to libx264, got: %s", joined)
	}
}

func TestBuildFFmpegArgsCanDisableRealtimeAndThreadCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.Transcoder.MaxHeight = 720
	cfg.Transcoder.SegmentSeconds = 4
	cfg.Transcoder.Realtime = false
	cfg.Transcoder.Threads = 0

	args := buildFFmpegArgs(cfg, "/x.mkv", "/o/x.m3u8", "/o/seg_%05d.ts")
	joined := " " + strings.Join(args, " ") + " "
	if strings.Contains(joined, " -re ") {
		t.Fatalf("realtime=false should not include -re, got: %s", joined)
	}
	if strings.Contains(joined, " -threads ") {
		t.Fatalf("threads=0 should not include -threads, got: %s", joined)
	}
}

func TestRequiredVideoEncoder(t *testing.T) {
	cases := map[string]string{
		"":      "libx264",
		"nvenc": "h264_nvenc",
		"qsv":   "h264_qsv",
		"vaapi": "h264_vaapi",
	}
	for encoder, want := range cases {
		if got := requiredVideoEncoder(encoder); got != want {
			t.Fatalf("requiredVideoEncoder(%q) = %q, want %q", encoder, got, want)
		}
	}
}

func TestHasFFmpegListEntry(t *testing.T) {
	out := " V..... libx264              libx264 H.264 / AVC\n A..... aac"
	if !hasFFmpegListEntry(out, "libx264") {
		t.Fatal("expected libx264 entry")
	}
	if hasFFmpegListEntry(out, "x264") {
		t.Fatal("must match whole ffmpeg list entries only")
	}
}
