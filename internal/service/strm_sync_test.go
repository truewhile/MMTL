package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"github.com/ShukeBta/MMTL/internal/service/cloud"
	"github.com/ShukeBta/MMTL/internal/service/cloud115"
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
		&model.StrmDownloadTask{}, &model.StrmUploadTask{}, &model.StrmDirCache{}, &model.Setting{}); err != nil {
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

// TestStrmFullAndIncrementalSync 测试增量同步与全量同步模式切换及记录
func TestStrmFullAndIncrementalSync(t *testing.T) {
	svc := testStrmService(t)
	src := t.TempDir()
	out := t.TempDir()

	writeFile(t, filepath.Join(src, "电影", "星际穿越.mkv"), "fake-video-data")

	p := syncPathRecord(t, svc, model.StrmProviderLocal, src, out, true)

	// 1. 默认触发增量同步
	if err := svc.StartSync(context.Background(), p.ID, model.StrmSyncTypeIncremental); err != nil {
		t.Fatal(err)
	}
	record := waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.Status != model.StrmSyncRecordDone {
		t.Fatalf("sync status = %s, message = %s", record.Status, record.Message)
	}
	if record.SyncType != model.StrmSyncTypeIncremental {
		t.Fatalf("expected sync_type = incremental, got %s", record.SyncType)
	}
	if record.NewStrm != 1 {
		t.Fatalf("expected 1 new strm, got %d", record.NewStrm)
	}

	// 2. 再次执行增量同步，应当跳过
	if err := svc.StartSync(context.Background(), p.ID, model.StrmSyncTypeIncremental); err != nil {
		t.Fatal(err)
	}
	record = waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.SyncType != model.StrmSyncTypeIncremental {
		t.Fatalf("expected sync_type = incremental, got %s", record.SyncType)
	}
	if record.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", record.Skipped)
	}

	// 3. 执行全量同步
	if err := svc.StartSync(context.Background(), p.ID, model.StrmSyncTypeFull); err != nil {
		t.Fatal(err)
	}
	record = waitSyncDone(t, svc, p.ID, 10*time.Second)
	if record.SyncType != model.StrmSyncTypeFull {
		t.Fatalf("expected sync_type = full, got %s", record.SyncType)
	}
	if record.Status != model.StrmSyncRecordDone {
		t.Fatalf("full sync failed: status = %s, message = %s", record.Status, record.Message)
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
	// sanitizeLocalPath 的盘符处理是平台相关的：Windows 保留 `D:\` 前缀，
	// Linux 把反斜杠统一当作分隔符（结果等价于 D:/test/...）。
	localWin := `D:\test\动漫\数码宝贝:拯救者\poster.jpg`
	var wantLocalWin string
	if runtime.GOOS == "windows" {
		wantLocalWin = filepath.Join(`D:\`, "test", "动漫", "数码宝贝 拯救者", "poster.jpg")
	} else {
		wantLocalWin = filepath.Join("D", "test", "动漫", "数码宝贝 拯救者", "poster.jpg")
	}
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

// fakeRemoteProvider 是 walkRemote 并发遍历的假提供方：返回一棵固定目录树，
// 并记录每个目录被 List 的次数，用于验证并发遍历无漏目录、无重复目录。
type fakeRemoteProvider struct {
	listed map[string]int
}

func (f *fakeRemoteProvider) Type() string               { return "fake" }
func (f *fakeRemoteProvider) Ping(context.Context) error { return nil }
func (f *fakeRemoteProvider) Resolve(context.Context, string) (*cloud.DirectLink, error) {
	return &cloud.DirectLink{URL: "http://cdn/x.mkv"}, nil
}

func (f *fakeRemoteProvider) List(_ context.Context, dirID string) ([]cloud.FileEntry, error) {
	if f.listed == nil {
		f.listed = map[string]int{}
	}
	f.listed[dirID]++
	switch dirID {
	case "root":
		return []cloud.FileEntry{
			{ID: "a", Name: "动漫", IsDir: true},
			{ID: "b", Name: "电影", IsDir: true},
			{ID: "f1", Name: "孤儿视频.mkv", Size: 100},
		}, nil
	case "a":
		return []cloud.FileEntry{
			{ID: "a1", Name: "番剧", IsDir: true},
			{ID: "fa1", Name: "第01集.mkv", Size: 200},
		}, nil
	case "a1":
		return []cloud.FileEntry{
			{ID: "fa11", Name: "第01集.mkv", Size: 300},
			{ID: "fa12", Name: "第02集.mkv", Size: 300},
		}, nil
	case "b":
		return []cloud.FileEntry{
			{ID: "fb1", Name: "电影A.mkv", Size: 400},
		}, nil
	default:
		return nil, nil
	}
}

// TestWalkRemoteConcurrent 验证并发目录遍历：所有目录均被列出、所有文件
// 均被处理（strm 生成 / 元数据入队），且不重复。
func TestWalkRemoteConcurrent(t *testing.T) {
	svc := testStrmService(t)
	localDir := t.TempDir()

	acct := &model.StrmAccount{
		Name:     "fake",
		Provider: "cloud115",
		Config:   "{}",
		Enabled:  true,
	}
	if err := svc.repo.StrmAccount.Create(context.Background(), acct); err != nil {
		t.Fatal(err)
	}
	p := &model.StrmSyncPath{
		Base:       model.Base{ID: "walk-path"},
		AccountID:  acct.ID,
		Provider:   model.StrmProvider115,
		RemotePath: "root",
		LocalPath:  localDir,
	}

	provider := &fakeRemoteProvider{listed: map[string]int{}}
	st := &strmSyncState{
		s:          svc,
		ctx:        context.Background(),
		p:          p,
		provider:   provider,
		cfg:        &strmPathConfig{VideoExt: []string{"mkv"}, MetaExt: []string{"nfo"}, AddPath: 1, DownloadMeta: false},
		rec:        &model.StrmSyncRecord{},
		seenVideo:  map[string]bool{},
		seenMeta:   map[string]bool{},
		remoteMeta: map[string]int64{},
	}
	if err := st.walkRemote(); err != nil {
		t.Fatalf("walkRemote failed: %v", err)
	}

	for _, dir := range []string{"root", "a", "a1", "b"} {
		if provider.listed[dir] != 1 {
			t.Errorf("目录 %s 被列出 %d 次，期望 1 次", dir, provider.listed[dir])
		}
	}

	// 5 个视频文件应生成 5 个 .strm：孤儿视频.mkv / a目录第01集 /
	// 番剧第01集+第02集 / 电影A（递归统计，含子目录）
	strmCount := 0
	walkErr := filepath.WalkDir(localDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".strm") {
			strmCount++
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if strmCount != 5 {
		t.Errorf("生成的 .strm 数量 = %d，期望 5", strmCount)
	}
}

// TestStrmBatchEnqueueAndConcurrentClaim 测试大规模批量入库及多协程并发认领无死锁
func TestStrmBatchEnqueueAndConcurrentClaim(t *testing.T) {
	svc := testStrmService(t)
	ctx := context.Background()

	// 1. 批量插入 200 个下载任务
	tasks := make([]*model.StrmDownloadTask, 0, 200)
	for i := 0; i < 200; i++ {
		tasks = append(tasks, &model.StrmDownloadTask{
			SyncPathID: "test-sync-path",
			AccountID:  "test-acct",
			Provider:   model.StrmProvider115,
			FileName:   filepath.Base(string(rune('a'+i%26))) + ".nfo",
			LocalPath:  filepath.Join(t.TempDir(), string(rune('a'+i%26)), "test.nfo"),
			Status:     model.StrmTaskPending,
		})
	}
	if err := svc.repo.StrmDownload.CreateInBatches(ctx, tasks, 50); err != nil {
		t.Fatalf("CreateInBatches failed: %v", err)
	}

	// 2. 验证 ActiveLocalPathMap
	activeMap, err := svc.repo.StrmDownload.GetActiveLocalPathMap(ctx, "test-sync-path")
	if err != nil {
		t.Fatalf("GetActiveLocalPathMap failed: %v", err)
	}
	if len(activeMap) == 0 {
		t.Fatal("expected active local path map to have entries")
	}

	// 3. 模拟 6 个 worker 并发 ClaimPendingDownload
	claimedCount := 0
	var claimMu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < 6; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				batch, err := svc.repo.StrmDownload.ClaimPendingDownload(ctx, 10)
				if err != nil {
					t.Errorf("concurrent ClaimPendingDownload failed: %v", err)
					return
				}
				if len(batch) == 0 {
					return
				}
				claimMu.Lock()
				claimedCount += len(batch)
				claimMu.Unlock()
			}
		}()
	}
	wg.Wait()

	if claimedCount != 200 {
		t.Fatalf("expected all 200 tasks claimed, got %d", claimedCount)
	}
}

// TestStrmDuplicateFileConflictResolution 测试远端存在多个同名不同大小文件时，本地确定性仲裁，避免增量死循环
func TestStrmDuplicateFileConflictResolution(t *testing.T) {
	svc := testStrmService(t)
	localDir := t.TempDir()

	p := &model.StrmSyncPath{
		Base:         model.Base{ID: "dup-test-path"},
		Provider:     model.StrmProvider115,
		RemotePath:   "root",
		LocalPath:    localDir,
		DownloadMeta: true,
	}

	st := &strmSyncState{
		s:               svc,
		ctx:             context.Background(),
		p:               p,
		cfg:             &strmPathConfig{DownloadMeta: true, MetaExt: []string{"nfo"}},
		rec:             &model.StrmSyncRecord{},
		seenMeta:        map[string]bool{},
		remoteMeta:      map[string]int64{},
		seenMetaTarget:  map[string]cloud.FileEntry{},
		seenVideoTarget: map[string]cloud.FileEntry{},
	}

	// 模拟远端同目录下存在两个同名不同大小的 nfo 文件 (115 历史重复上传)
	// entry1: 较早文件 (MTime: 1000, Size: 100)
	entry1 := cloud.FileEntry{ID: "f1", Name: "test.nfo", Size: 100, MTime: 1000, PickCode: "p1"}
	// entry2: 较新文件 (MTime: 2000, Size: 200)
	entry2 := cloud.FileEntry{ID: "f2", Name: "test.nfo", Size: 200, MTime: 2000, PickCode: "p2"}

	// 第一次全量处理：两者都在列表中
	st.handleMeta(entry1, "test.nfo", ".nfo")
	st.handleMeta(entry2, "test.nfo", ".nfo")
	st.flushPendingDownloads()

	// 验证仲裁结果：最终只产生 1 个下载任务，且使用的是首个匹配项 (Size 100/p1)
	tasks, _, err := svc.repo.StrmDownload.List(context.Background(), "", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 download task after conflict resolution, got %d", len(tasks))
	}
	if tasks[0].Size != 100 || tasks[0].RemoteRef != "p1" {
		t.Fatalf("expected task with size 100/p1, got size=%d ref=%s", tasks[0].Size, tasks[0].RemoteRef)
	}

	// 模拟该任务下载落盘完成
	writeFile(t, filepath.Join(localDir, "test.nfo"), strings.Repeat("x", 100))

	// 第二次增量同步：两者再次依次扫描
	st2 := &strmSyncState{
		s:               svc,
		ctx:             context.Background(),
		p:               p,
		cfg:             &strmPathConfig{DownloadMeta: true, MetaExt: []string{"nfo"}},
		rec:             &model.StrmSyncRecord{},
		seenMeta:        map[string]bool{},
		remoteMeta:      map[string]int64{},
		seenMetaTarget:  map[string]cloud.FileEntry{},
		seenVideoTarget: map[string]cloud.FileEntry{},
	}
	st2.handleMeta(entry1, "test.nfo", ".nfo")
	st2.handleMeta(entry2, "test.nfo", ".nfo")
	st2.flushPendingDownloads()

	// 验证：不会新增任何下载任务，NewMeta 为 0，增量跳过
	if st2.rec.NewMeta != 0 {
		t.Fatalf("expected 0 new meta on incremental sync, got %d", st2.rec.NewMeta)
	}

}

// TestWalk115FlatAbortsOnDirResolveFailure 回归测试：115 开放平台 token 失效/目录详情
// 解析失败时，同步必须中止而不是带着塌缩的 rel 继续处理，否则会导致本地大量元数据
// 被误判为"云端不存在"而重复下载/上传，甚至误删本地文件（用户反馈"云盘没动却重下重传"）。
func TestWalk115FlatAbortsOnDirResolveFailure(t *testing.T) {
	svc := testStrmService(t)
	localDir := t.TempDir()

	acct := &model.StrmAccount{
		Name:     "fake115",
		Provider: "cloud115",
		Config:   "{}",
		Enabled:  true,
	}
	if err := svc.repo.StrmAccount.Create(context.Background(), acct); err != nil {
		t.Fatal(err)
	}
	p := &model.StrmSyncPath{
		Base:       model.Base{ID: "abort-path"},
		AccountID:  acct.ID,
		Provider:   model.StrmProvider115,
		RemotePath: "0",
		LocalPath:  localDir,
	}

	// 115 mock：文件列表返回一个视频（父目录 999 不在缓存，需要 get_info），
	// get_info 恒返回 access_token 格式错误（40140123）→ 目录树解析失败。
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/ufile/files":
			w.Write([]byte(`{"state":true,"count":1,"data":[{"fid":"100","pid":"999","fc":1,"fn":"movie.mkv","pc":"pc1","upt":1700000000,"fs":1024}]}`))
		case "/open/folder/get_info":
			w.Write([]byte(`{"state":false,"code":40140123,"message":"access_token 格式错误"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer api.Close()

	oldPro := cloud115.ProAPIBase
	cloud115.ProAPIBase = api.URL
	defer func() { cloud115.ProAPIBase = oldPro }()

	oc := cloud115.NewOpenClient("app", "at", "rt")
	st := &strmSyncState{
		s:          svc,
		ctx:        context.Background(),
		p:          p,
		provider:   cloud.NewOpenAPI115("app", "at", "rt"),
		cfg:        &strmPathConfig{VideoExt: []string{"mkv"}, MetaExt: []string{"nfo"}, AddPath: 1, DownloadMeta: false},
		rec:        &model.StrmSyncRecord{},
		syncType:   model.StrmSyncTypeFull,
		dirCache:   sync.Map{},
		seenVideo:  map[string]bool{},
		seenMeta:   map[string]bool{},
		remoteMeta: map[string]int64{},
	}
	err := st.walk115Flat(oc)
	if err == nil {
		t.Fatal("expected walk115Flat to abort on dir-resolve failure, got nil error")
	}

	// 中止后不允许产生任何部分写入（本地不允许生成 .strm 文件）。
	var strmCount int
	_ = filepath.WalkDir(localDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".strm") {
			strmCount++
		}
		return nil
	})
	if strmCount != 0 {
		t.Fatalf("expected no .strm written after abort, got %d", strmCount)
	}
}
