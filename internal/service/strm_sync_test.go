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

func TestSanitizePathWithSpecialChars(t *testing.T) {
	cases := []struct {
		input    string
		isDir    bool
		wantName string
	}{
		{"数码宝贝:拯救者", true, "数码宝贝 拯救者"},
		{"数码宝贝:拯救者.mp4", false, "数码宝贝 拯救者.mp4"},
		{"Season 01.", true, "Season 01"},
		{"file:name?test*<foo>|bar\".mkv", false, "file nametestfoobar.mkv"},
		{"poster.jpg", false, "poster.jpg"},
		{"  trailing space  ", true, "trailing space"},
		{"CON", true, "_CON"},
		{"aux.mp4", false, "_aux.mp4"},
		{"NUL.nfo", false, "_NUL.nfo"},
		{"COM1", false, "_COM1"},
		{"test\x00\x1f\x7fcontrol.mkv", false, "testcontrol.mkv"},
	}
	for _, tc := range cases {
		got := cleanEntryName(tc.input, tc.isDir)
		if got != tc.wantName {
			t.Errorf("cleanEntryName(%q, %v) = %q, want %q", tc.input, tc.isDir, got, tc.wantName)
		}
	}

	// Test sanitizeRelativePath
	rel := "动漫/数码宝贝:拯救者/数码宝贝:拯救者.mp4"
	wantRel := filepath.Join("动漫", "数码宝贝 拯救者", "数码宝贝 拯救者.mp4")
	if got := sanitizeRelativePath(rel); got != wantRel {
		t.Errorf("sanitizeRelativePath(%q) = %q, want %q", rel, got, wantRel)
	}

	// Test sanitizeLocalPath - Windows Drive
	localWin := `D:\test\动漫\数码宝贝:拯救者\poster.jpg`
	wantLocalWin := filepath.Join(`D:\`, "test", "动漫", "数码宝贝 拯救者", "poster.jpg")
	if got := sanitizeLocalPath(localWin); got != wantLocalWin {
		t.Errorf("sanitizeLocalPath(%q) = %q, want %q", localWin, got, wantLocalWin)
	}

	// Test sanitizeLocalPath - Linux root
	localLinux := `/media/data/动漫/数码宝贝:拯救者/ep01:拯救者.mkv`
	cleanedLinux := sanitizeLocalPath(localLinux)
	if strings.Contains(cleanedLinux, ":") && !strings.HasPrefix(cleanedLinux, "D:") && !strings.HasPrefix(cleanedLinux, "C:") {
		t.Errorf("sanitizeLocalPath should not contain colons in path components: %q", cleanedLinux)
	}

	// Test joinLocalRel
	root := t.TempDir()
	joined, err := joinLocalRel(root, "动漫/数码宝贝:拯救者/poster.jpg")
	if err != nil {
		t.Fatalf("joinLocalRel failed: %v", err)
	}
	wantJoined := filepath.Join(root, "动漫", "数码宝贝 拯救者", "poster.jpg")
	if joined != wantJoined {
		t.Errorf("joinLocalRel = %q, want %q", joined, wantJoined)
	}

	// Test joinLocalRel prevents path traversal
	if _, err := joinLocalRel(root, "../../"); err == nil {
		t.Error("joinLocalRel should error on empty/traversal-only path")
	}
	traversalCleaned, err := joinLocalRel(root, "../../etc/passwd")
	if err != nil {
		t.Fatalf("joinLocalRel failed on traversal path: %v", err)
	}
	if !strings.HasPrefix(traversalCleaned, root) {
		t.Errorf("joinLocalRel allowed path escape: %s", traversalCleaned)
	}

	// Verify mkdir succeeds on this joined path
	if err := os.MkdirAll(filepath.Dir(joined), 0o755); err != nil {
		t.Fatalf("MkdirAll on joined path failed: %v", err)
	}
	if info, err := os.Stat(filepath.Dir(joined)); err != nil || !info.IsDir() {
		t.Fatalf("Dir not created: %v", err)
	}
}

// TestScanLocalMetaForUpload 验证元数据上传比对逻辑：网盘已存在跳过，网盘不存在才入队上传。
func TestScanLocalMetaForUpload(t *testing.T) {
	svc := testStrmService(t)
	localDir := t.TempDir()

	// 本地有 2 个元数据：poster.jpg 和 fanart.jpg
	writeFile(t, filepath.Join(localDir, "动漫", "poster.jpg"), "poster-data")
	writeFile(t, filepath.Join(localDir, "动漫", "fanart.jpg"), "fanart-data")

	p := &model.StrmSyncPath{
		Base:       model.Base{ID: "test-path-upload"},
		AccountID:  "acct-1",
		Provider:   model.StrmProviderCloudDrive,
		RemotePath: "/Media",
		LocalPath:  localDir,
		UploadMeta: true,
	}

	st := &strmSyncState{
		s:          svc,
		ctx:        context.Background(),
		p:          p,
		cfg:        &strmPathConfig{UploadMeta: true, MetaExt: []string{"jpg", "nfo"}},
		rec:        &model.StrmSyncRecord{},
		seenMeta:   map[string]bool{},
		remoteMeta: map[string]int64{},
	}

	// 模拟远端已存在 poster.jpg
	st.remoteMeta["m:动漫/poster.jpg"] = 1000

	if err := st.scanLocalMetaForUpload(); err != nil {
		t.Fatalf("scanLocalMetaForUpload failed: %v", err)
	}

	// 此时应该只有 fanart.jpg 入队上传，poster.jpg 被跳过
	tasks, _, err := svc.repo.StrmUpload.List(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 upload task (fanart.jpg), got %d", len(tasks))
	}
	if tasks[0].FileName != "fanart.jpg" {
		t.Errorf("expected upload task for fanart.jpg, got %s", tasks[0].FileName)
	}

	// 再次扫描：fanart.jpg 已经在队列中，应自动去重，不重复入队
	if err := st.scanLocalMetaForUpload(); err != nil {
		t.Fatalf("second scanLocalMetaForUpload failed: %v", err)
	}
	tasks, _, err = svc.repo.StrmUpload.List(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected still 1 upload task after dedup, got %d", len(tasks))
	}
}
