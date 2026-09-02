package service

import (
	"testing"
)

func TestParseEmbyRemoteLinesLegacyURL(t *testing.T) {
	lines, active, err := ParseEmbyRemoteLines(map[string]string{
		"url": "http://192.168.1.10:8096/",
	})
	if err != nil {
		t.Fatalf("ParseEmbyRemoteLines() error = %v", err)
	}
	if active != 0 || len(lines) != 1 || lines[0].URL != "http://192.168.1.10:8096" {
		t.Fatalf("unexpected lines: %#v active=%d", lines, active)
	}
}

func TestParseEmbyRemoteLinesMulti(t *testing.T) {
	lines, active, err := ParseEmbyRemoteLines(map[string]string{
		"urls":        `[{"name":"内网","url":"http://10.0.0.8:8096"},{"name":"外网","url":"https://emby.example.com"}]`,
		"active_line": "1",
	})
	if err != nil {
		t.Fatalf("ParseEmbyRemoteLines() error = %v", err)
	}
	if active != 1 || len(lines) != 2 {
		t.Fatalf("unexpected lines: %#v active=%d", lines, active)
	}
	if lines[1].Name != "外网" || lines[1].URL != "https://emby.example.com" {
		t.Fatalf("unexpected second line: %#v", lines[1])
	}
}

func TestEncodeEmbyRemoteLines(t *testing.T) {
	jsonText, primary, err := EncodeEmbyRemoteLines([]EmbyRemoteLine{
		{Name: "内网", URL: "http://10.0.0.8:8096/"},
		{Name: "外网", URL: "https://emby.example.com"},
	})
	if err != nil {
		t.Fatalf("EncodeEmbyRemoteLines() error = %v", err)
	}
	if primary != "http://10.0.0.8:8096" {
		t.Fatalf("primary = %q", primary)
	}
	lines, active, err := ParseEmbyRemoteLines(map[string]string{"urls": jsonText})
	if err != nil || active != 0 || len(lines) != 2 {
		t.Fatalf("roundtrip failed: %#v active=%d err=%v", lines, active, err)
	}
}

func TestEmbyRemoteServiceLineOrder(t *testing.T) {
	svc := &EmbyRemoteService{}
	cfg := &EmbyRemoteConfig{
		Lines: []EmbyRemoteLine{
			{URL: "http://a"},
			{URL: "http://b"},
			{URL: "http://c"},
		},
		ActiveLine: 1,
	}
	order := svc.lineOrder(cfg)
	if len(order) != 3 || order[0] != 1 || order[1] != 0 || order[2] != 2 {
		t.Fatalf("unexpected order: %#v", order)
	}
}
