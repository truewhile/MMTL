package service

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ShukeBta/MediaStationGo/internal/config"
	"github.com/ShukeBta/MediaStationGo/internal/model"
	"github.com/ShukeBta/MediaStationGo/internal/repository"
)

// testStrmService 构建带内存库的 StrmService。
// 同步引擎在独立 goroutine 里访问 DB，因此必须用共享缓存的内存库，
// 否则每个连接都会得到一张空表。
func testStrmService(t *testing.T) *StrmService {
	t.Helper()
	dsn := "file:strmtest_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(4)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.StrmAccount{}, &model.StrmSyncPath{}, &model.StrmSyncRecord{},
		&model.StrmDownloadTask{}, &model.StrmUploadTask{}, &model.Setting{}); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	ctx := context.Background()
	if err := repos.Setting.Set(ctx, StrmSettingBaseURL, "http://test.local:8096"); err != nil {
		t.Fatal(err)
	}
	return NewStrmService(nil, zap.NewNop(), repos, NewCryptoService("test-secret", zap.NewNop()))
}

func syncPathRecord(t *testing.T, svc *StrmService, provider, remote, local string, enabled bool) *model.StrmSyncPath {
	t.Helper()
	p, err := svc.CreateSyncPath(context.Background(), &model.StrmSyncPath{
		Name:         "test",
		Provider:     provider,
		RemotePath:   remote,
		LocalPath:    local,
		AddPath:      1,
		Enabled:      enabled,
		DownloadMeta: true,
	})
	if err != nil {
		t.Fatalf("create sync path: %v", err)
	}
	return p
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitSyncDone(t *testing.T, svc *StrmService, pathID string, timeout time.Duration) *model.StrmSyncRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		records, err := svc.repo.StrmSyncRecord.List(context.Background(), pathID, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) > 0 && records[0].Status != model.StrmSyncRecordRunning &&
			records[0].Status != model.StrmSyncRecordPending {
			return &records[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("sync did not finish in time")
	return nil
}

// TestLocalStrmSync 本地目录：视频 → .strm 生成、内容指向播放端点、
// 二次同步跳过、远端删除后清理。
func TestLocalStrmSync(t *testing.T) {
	svc := testStrmService(t)
	src := t.TempDir()
	out := t.TempDir()

	writeFile(t, filepath.Join(src, "电影", "阿凡达.mkv"), "fake-video-data")
	writeFile(t, filepath.Join(src, "电影", "阿凡达.nfo"), "<xml/>")

	p := syncPathRecord(t, svc, model.StrmProviderLocal, src, out, true)
	if err := svc.StartSync(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	record := waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.Status != model.StrmSyncRecordDone {
		t.Fatalf("sync status = %s, message = %s", record.Status, record.Message)
	}
	if record.NewStrm != 1 {
		t.Fatalf("expected 1 new strm, got %d", record.NewStrm)
	}
	strmPath := filepath.Join(out, "电影", "阿凡达.strm")
	strmContent, err := os.ReadFile(strmPath)
	if err != nil {
		t.Fatalf("strm file not created: %v", err)
	}
	content := string(strmContent)
	if !strings.Contains(content, "/api/strm/play/local/video.mkv?") || !strings.Contains(content, "path=") {
		t.Fatalf("unexpected strm content: %s", content)
	}
	playURL, err := url.Parse(content)
	if err != nil {
		t.Fatalf("parse strm url: %v", err)
	}
	gotPath, err := url.QueryUnescape(playURL.Query().Get("path"))
	if err != nil {
		t.Fatalf("unescape path: %v", err)
	}
	wantPath := filepath.Join(src, "电影", "阿凡达.mkv")
	if gotPath != wantPath {
		t.Fatalf("strm path param = %q, want %q (content: %s)", gotPath, wantPath, content)
	}

	// 二次同步：内容未变应跳过
	if err := svc.StartSync(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	record = waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.Skipped != 1 {
		t.Fatalf("expected 1 skipped on second sync, got %d", record.Skipped)
	}

	// 删除远端视频后同步应清理本地 strm
	if err := os.Remove(filepath.Join(src, "电影", "阿凡达.mkv")); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartSync(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	record = waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.Pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", record.Pruned)
	}
	if _, err := os.Stat(strmPath); !os.IsNotExist(err) {
		t.Fatalf("strm file should have been pruned, got err=%v", err)
	}
}

// TestStrmCronMatches cron 表达式匹配。
func TestStrmCronMatches(t *testing.T) {
	cases := []struct {
		expr string
		now  time.Time
		want bool
	}{
		{"* * * * *", time.Date(2026, 8, 24, 10, 30, 0, 0, time.Local), true},
		{"30 10 * * *", time.Date(2026, 8, 24, 10, 30, 0, 0, time.Local), true},
		{"30 10 * * *", time.Date(2026, 8, 24, 10, 31, 0, 0, time.Local), false},
		{"*/15 * * * *", time.Date(2026, 8, 24, 10, 30, 0, 0, time.Local), true},
		{"*/15 * * * *", time.Date(2026, 8, 24, 10, 34, 0, 0, time.Local), false},
		{"0 */6 * * *", time.Date(2026, 8, 24, 18, 0, 0, 0, time.Local), true},
		{"0 */6 * * *", time.Date(2026, 8, 24, 19, 0, 0, 0, time.Local), false},
		{"0 2 * * 1-5", time.Date(2026, 8, 24, 2, 0, 0, 0, time.Local), true},  // 周一
		{"0 2 * * 1-5", time.Date(2026, 8, 23, 2, 0, 0, 0, time.Local), false}, // 周日
		{"bad", time.Now(), false},
	}
	for _, tc := range cases {
		if got := cronMatches(tc.expr, tc.now); got != tc.want {
			t.Errorf("cronMatches(%q) = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

// TestStrmDefaultBaseURL 无任何地址配置时自动使用本机监听地址。
func TestStrmDefaultBaseURL(t *testing.T) {
	if got := strmDefaultBaseURL(nil); got != "http://127.0.0.1:8080" {
		t.Fatalf("default base url = %q", got)
	}
	if got := strmDefaultBaseURL(&config.Config{App: config.AppConfig{Port: 9000}}); got != "http://127.0.0.1:9000" {
		t.Fatalf("default base url with port = %q", got)
	}
	// strmEffectiveConfig 无配置不报错且默认兜底
	svc := testStrmService(t)
	p := &model.StrmSyncPath{Provider: model.StrmProviderLocal, AddPath: 1, DownloadMeta: true}
	cfg, err := svc.strmEffectiveConfig(context.Background(), p)
	if err != nil {
		t.Fatalf("effective config should not fail without base url: %v", err)
	}
	if cfg.BaseURL == "" {
		t.Fatal("effective config base url should fall back to default")
	}
}
