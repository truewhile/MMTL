// STRM 同步引擎：扫描网盘/本地目录，生成 .strm 文件，按需入队元数据下载/上传，
// 并清理远端已不存在的本地多余文件。参考 QMediaSync 的 STRM 同步流程实现。
package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MediaStationGo/internal/model"
	"github.com/ShukeBta/MediaStationGo/internal/service/cloud"
)

// strmSyncState 是一次同步执行的上下文。
type strmSyncState struct {
	s        *StrmService
	ctx      context.Context
	p        *model.StrmSyncPath
	acct     *model.StrmAccount
	provider cloud.Provider // local 提供方为 nil
	cfg      *strmPathConfig
	rec      *model.StrmSyncRecord

	mu         sync.Mutex
	processed  int              // 已处理文件计数（用于定期落库进度）
	seenVideo  map[string]bool  // "v:"+去掉扩展名的相对路径 → 远端存在该视频
	seenMeta   map[string]bool  // "m:"+相对路径 → 远端存在该元数据
	remoteMeta map[string]int64 // 远端元数据大小（上传比对用）
}

// StartSync 启动一次同步（异步执行，同一目录同时只允许一个任务）。
func (s *StrmService) StartSync(ctx context.Context, pathID string) error {
	p, err := s.repo.StrmSyncPath.FindByID(ctx, pathID)
	if err != nil || p == nil {
		return errNotFoundOr(err, "同步目录不存在")
	}
	if !p.Enabled {
		return errors.New("同步目录已禁用")
	}
	if p.Provider == model.StrmProviderLocal && strings.TrimSpace(p.RemotePath) == "" {
		return errors.New("本地同步需要填写源目录")
	}
	if p.Provider != model.StrmProviderLocal && strings.TrimSpace(p.AccountID) == "" {
		return errors.New("该同步目录未关联网盘账号")
	}
	s.mu.Lock()
	if _, exists := s.running[pathID]; exists {
		s.mu.Unlock()
		return errors.New("该目录正在同步中")
	}
	// 同步在后台持续执行，不受 HTTP 请求生命周期影响
	runCtx, cancel := context.WithCancel(s.baseCtx)
	s.running[pathID] = cancel
	s.mu.Unlock()

	now := time.Now()
	rec := &model.StrmSyncRecord{
		SyncPathID: pathID,
		Status:     model.StrmSyncRecordRunning,
		StartedAt:  &now,
	}
	if err := s.repo.StrmSyncRecord.Create(ctx, rec); err != nil {
		s.clearRunning(pathID)
		return err
	}
	status := model.StrmSyncRecordRunning
	p.LastSyncAt = &now
	p.LastSyncStatus = status
	p.LastSyncMessage = "同步进行中"
	_ = s.repo.StrmSyncPath.Update(ctx, p)

	go s.runSync(runCtx, p, rec)
	return nil
}

// CancelSync 取消正在进行的同步。
func (s *StrmService) CancelSync(ctx context.Context, pathID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel, exists := s.running[pathID]
	if !exists {
		return errors.New("该目录当前没有进行中的同步")
	}
	cancel()
	return nil
}

// IsSyncRunning 报告某同步目录是否正在同步。
func (s *StrmService) IsSyncRunning(pathID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.running[pathID]
	return exists
}

func (s *StrmService) clearRunning(pathID string) {
	s.mu.Lock()
	delete(s.running, pathID)
	s.mu.Unlock()
}

// ListRemoteDir 列出网盘账号某目录下的条目（供前端目录选择器使用）。
func (s *StrmService) ListRemoteDir(ctx context.Context, accountID, dir string) ([]cloud.FileEntry, error) {
	acct, err := s.repo.StrmAccount.FindByID(ctx, accountID)
	if err != nil || acct == nil {
		return nil, errNotFoundOr(err, "网盘账号不存在")
	}
	provider, err := s.providerFor(ctx, acct)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		if acct.Provider == model.StrmProvider115 {
			dir = "0"
		} else {
			dir = "/"
		}
	}
	return provider.List(ctx, dir)
}

// runSync 执行同步主体；结束时更新记录与目录状态。
func (s *StrmService) runSync(ctx context.Context, p *model.StrmSyncPath, rec *model.StrmSyncRecord) {
	defer s.clearRunning(p.ID)

	cfg, err := s.strmEffectiveConfig(ctx, p)
	if err != nil {
		s.finishSync(p, rec, model.StrmSyncRecordFailed, err.Error())
		return
	}
	st := &strmSyncState{
		s:          s,
		ctx:        ctx,
		p:          p,
		cfg:        cfg,
		rec:        rec,
		seenVideo:  map[string]bool{},
		seenMeta:   map[string]bool{},
		remoteMeta: map[string]int64{},
	}
	if p.Provider != model.StrmProviderLocal {
		acct, err := s.repo.StrmAccount.FindByID(ctx, p.AccountID)
		if err != nil || acct == nil {
			s.finishSync(p, rec, model.StrmSyncRecordFailed, "网盘账号不存在或已删除")
			return
		}
		if !acct.Enabled {
			s.finishSync(p, rec, model.StrmSyncRecordFailed, "网盘账号已禁用")
			return
		}
		provider, err := s.providerFor(ctx, acct)
		if err != nil {
			s.finishSync(p, rec, model.StrmSyncRecordFailed, err.Error())
			return
		}
		st.acct = acct
		st.provider = provider
	}

	if err := st.run(); err != nil {
		if errors.Is(err, context.Canceled) {
			s.finishSync(p, rec, model.StrmSyncRecordCanceled, "已取消")
		} else {
			s.finishSync(p, rec, model.StrmSyncRecordFailed, err.Error())
		}
		return
	}
	s.finishSync(p, rec, model.StrmSyncRecordDone, "")
}

// finishSync 落库同步结果。
func (s *StrmService) finishSync(p *model.StrmSyncPath, rec *model.StrmSyncRecord, status, message string) {
	now := time.Now()
	rec.Status = status
	rec.Message = message
	rec.FinishedAt = &now
	if err := s.repo.StrmSyncRecord.Update(context.Background(), rec); err != nil {
		s.log.Warn("update strm sync record failed", zap.Error(err))
	}
	p.LastSyncStatus = status
	p.LastSyncMessage = message
	if status != model.StrmSyncRecordFailed && message == "" {
		p.LastSyncMessage = fmt.Sprintf("完成：新增/更新 %d 个 strm，下载 %d 个元数据，清理 %d 个文件",
			rec.NewStrm, rec.NewMeta, rec.Pruned)
	}
	if err := s.repo.StrmSyncPath.Update(context.Background(), p); err != nil {
		s.log.Warn("update strm sync path failed", zap.Error(err))
	}
	s.log.Info("strm sync finished",
		zap.String("path_id", p.ID), zap.String("status", status),
		zap.Int64("new_strm", rec.NewStrm), zap.Int64("new_meta", rec.NewMeta),
		zap.Int64("pruned", rec.Pruned), zap.String("message", message))
}

func (st *strmSyncState) run() error {
	if err := ensureLocalDir(st.p.LocalPath); err != nil {
		return fmt.Errorf("创建输出目录失败：%w", err)
	}
	if st.provider != nil {
		if err := st.walkRemote(); err != nil {
			return err
		}
	} else {
		if err := st.walkLocalSource(); err != nil {
			return err
		}
	}
	st.flushProgress()
	if st.cfg.UploadMeta && st.provider != nil && st.p.Provider != model.StrmProvider115 {
		if err := st.scanLocalMetaForUpload(); err != nil {
			return err
		}
	}
	if err := st.pruneLocal(); err != nil {
		return err
	}
	st.flushProgress()
	_ = st.ctx.Err()
	return nil
}

// walkRemote 广度优先遍历网盘目录树。
func (st *strmSyncState) walkRemote() error {
	root := strings.TrimSpace(st.p.RemotePath)
	if root == "" {
		root = "/"
	}
	type dirTask struct {
		id  string
		rel string
	}
	queue := []dirTask{{id: root, rel: ""}}
	for len(queue) > 0 {
		select {
		case <-st.ctx.Done():
			return st.ctx.Err()
		default:
		}
		task := queue[0]
		queue = queue[1:]
		entries, err := st.provider.List(st.ctx, task.id)
		if err != nil {
			return fmt.Errorf("列出远端目录 %s 失败：%w", task.id, err)
		}
		for _, entry := range entries {
			rel := entry.Name
			if task.rel != "" {
				rel = task.rel + "/" + entry.Name
			}
			if entry.IsDir {
				queue = append(queue, dirTask{id: entry.ID, rel: rel})
				continue
			}
			st.processRemoteFile(entry, rel)
		}
	}
	return nil
}

// processRemoteFile 分类处理远端文件：视频生成 STRM，元数据入下载队列。
func (st *strmSyncState) processRemoteFile(entry cloud.FileEntry, rel string) {
	fileName := entry.Name
	if st.isExcluded(fileName) {
		return
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	switch {
	case st.isVideoExt(ext, entry.Size):
		st.handleVideo(entry, rel, ext)
	case st.cfg.DownloadMeta && st.isMetaExt(ext):
		st.handleMeta(entry, rel, ext)
	default:
		st.touchProgress()
	}
}

func (st *strmSyncState) isExcluded(fileName string) bool {
	lower := strings.ToLower(fileName)
	for _, keyword := range st.cfg.ExcludeName {
		if keyword != "" && strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (st *strmSyncState) isVideoExt(ext string, size int64) bool {
	if st.cfg.MinSize > 0 && size < st.cfg.MinSize {
		return false
	}
	for _, e := range st.cfg.VideoExt {
		if "."+e == ext {
			return true
		}
	}
	return false
}

func (st *strmSyncState) isMetaExt(ext string) bool {
	for _, e := range st.cfg.MetaExt {
		if "."+e == ext {
			return true
		}
	}
	return false
}

// handleVideo 生成/更新 .strm 文件。
func (st *strmSyncState) handleVideo(entry cloud.FileEntry, rel, ext string) {
	relSansExt := rel[:len(rel)-len(ext)]
	st.mu.Lock()
	if st.seenVideo["v:"+relSansExt] {
		st.mu.Unlock()
		return
	}
	st.seenVideo["v:"+relSansExt] = true
	st.mu.Unlock()

	targetRel := relSansExt + ".strm"
	target, err := joinLocalRel(st.p.LocalPath, targetRel)
	if err != nil {
		st.s.log.Warn("strm target path out of root", zap.String("rel", targetRel), zap.Error(err))
		return
	}
	content, err := st.strmContent(entry, rel, ext)
	if err != nil {
		st.rec.Message = err.Error()
		st.s.log.Warn("build strm content failed", zap.String("file", rel), zap.Error(err))
		return
	}
	existing := ""
	if data, err := os.ReadFile(target); err == nil {
		existing = string(data)
	}
	if existing == content {
		st.mu.Lock()
		st.rec.Skipped++
		st.mu.Unlock()
		st.touchProgress()
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		st.s.log.Warn("mkdir strm dir failed", zap.String("dir", filepath.Dir(target)), zap.Error(err))
		return
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		st.s.log.Warn("write strm tmp failed", zap.String("file", target), zap.Error(err))
		return
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		st.s.log.Warn("rename strm failed", zap.String("file", target), zap.Error(err))
		return
	}
	st.mu.Lock()
	st.rec.NewStrm++
	st.mu.Unlock()
	st.touchProgress()
}

// strmContent 构建 strm 文件内容（一行指向本服务播放端点的 URL）。
func (st *strmSyncState) strmContent(entry cloud.FileEntry, rel, ext string) (string, error) {
	q := url.Values{}
	switch st.p.Provider {
	case model.StrmProvider115:
		q.Set("acct", st.p.AccountID)
		q.Set("pickcode", entry.PickCode)
	case model.StrmProviderCloudDrive, model.StrmProviderOpenList:
		q.Set("acct", st.p.AccountID)
		q.Set("ref", entry.ID)
	case model.StrmProviderLocal:
		src, err := joinLocalRel(st.p.RemotePath, rel)
		if err != nil {
			return "", err
		}
		q.Set("path", src)
	default:
		return "", fmt.Errorf("不支持的提供方：%s", st.p.Provider)
	}
	// 本地源用 path 参数携带真实文件路径；网盘源用 path 参数展示目录结构
	if st.p.Provider != model.StrmProviderLocal {
		if pathParam := st.strmPathParam(rel); pathParam != "" {
			q.Set("path", pathParam)
		}
	}
	suffix := ""
	if encoded := q.Encode(); encoded != "" {
		suffix = "?" + encoded
	}
	return st.cfg.BaseURL + "/api/strm/play/" + st.p.Provider + "/video" + ext + suffix, nil
}

// strmPathParam 按 add_path 模式生成 path 查询参数（1=完整相对路径 2=仅文件名 3=不带）。
func (st *strmSyncState) strmPathParam(rel string) string {
	switch st.cfg.AddPath {
	case 1:
		return rel
	case 2:
		_, name := filepath.Split(rel)
		return name
	default:
		return ""
	}
}

// handleMeta 元数据入下载队列（本地已存在且大小一致则跳过）。
func (st *strmSyncState) handleMeta(entry cloud.FileEntry, rel, ext string) {
	st.mu.Lock()
	st.seenMeta["m:"+rel] = true
	st.remoteMeta["m:"+rel] = entry.Size
	st.mu.Unlock()

	target, err := joinLocalRel(st.p.LocalPath, rel)
	if err != nil {
		return
	}
	if info, err := os.Stat(target); err == nil && info.Size() == entry.Size {
		st.touchProgress()
		return
	}
	if st.taskExists("download", st.p.ID, target) {
		st.touchProgress()
		return
	}
	task := &model.StrmDownloadTask{
		SyncPathID: st.p.ID,
		AccountID:  st.p.AccountID,
		Provider:   st.p.Provider,
		FileName:   entry.Name,
		RemoteRef:  entry.PickCode,
		RemoteDir:  st.p.RemotePath,
		LocalPath:  target,
		Size:       entry.Size,
		Status:     model.StrmTaskPending,
	}
	if task.RemoteRef == "" {
		task.RemoteRef = entry.ID
	}
	// 115 用 pickcode 定位；DAV/OpenList 用路径定位
	if st.p.Provider != model.StrmProvider115 {
		task.RemoteRef = entry.ID
	}
	if err := st.s.repo.StrmDownload.Create(st.ctx, task); err != nil {
		st.s.log.Warn("enqueue strm download task failed", zap.Error(err))
		return
	}
	st.mu.Lock()
	st.rec.NewMeta++
	st.mu.Unlock()
	st.touchProgress()
}

// walkLocalSource 本地源：视频生成 STRM，元数据就地存在。
func (st *strmSyncState) walkLocalSource() error {
	srcRoot := filepath.Clean(st.p.RemotePath)
	info, err := os.Stat(srcRoot)
	if err != nil {
		return fmt.Errorf("本地源目录不可访问：%w", err)
	}
	if !info.IsDir() {
		return errors.New("本地源目录不是目录")
	}
	return filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == srcRoot {
			return nil
		}
		select {
		case <-st.ctx.Done():
			return st.ctx.Err()
		default:
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if st.isExcluded(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if !st.isVideoExt(ext, info.Size()) {
			st.touchProgress()
			return nil
		}
		relSansExt := rel[:len(rel)-len(ext)]
		st.mu.Lock()
		st.seenVideo["v:"+relSansExt] = true
		st.mu.Unlock()
		content, err := st.strmContent(cloud.FileEntry{Name: filepath.Base(rel), Size: info.Size()}, rel, ext)
		if err != nil {
			return nil
		}
		target, err := joinLocalRel(st.p.LocalPath, relSansExt+".strm")
		if err != nil {
			return nil
		}
		if data, err := os.ReadFile(target); err == nil && string(data) == content {
			st.mu.Lock()
			st.rec.Skipped++
			st.mu.Unlock()
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil
		}
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, []byte(content), 0o644); err == nil {
			_ = os.Rename(tmp, target)
		} else {
			_ = os.Remove(tmp)
		}
		st.mu.Lock()
		st.rec.NewStrm++
		st.mu.Unlock()
		return nil
	})
}

// scanLocalMetaForUpload 扫描本地元数据，与远端比对后入上传队列。
func (st *strmSyncState) scanLocalMetaForUpload() error {
	localRoot := filepath.Clean(st.p.LocalPath)
	return filepath.WalkDir(localRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == localRoot {
			return nil
		}
		select {
		case <-st.ctx.Done():
			return st.ctx.Err()
		default:
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		if !st.isMetaExt(ext) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		st.mu.Lock()
		remoteSize, exists := st.remoteMeta["m:"+rel]
		st.mu.Unlock()
		if exists && remoteSize == info.Size() {
			return nil
		}
		if st.taskExists("upload", st.p.ID, rel) {
			return nil
		}
		task := &model.StrmUploadTask{
			SyncPathID: st.p.ID,
			AccountID:  st.p.AccountID,
			Provider:   st.p.Provider,
			FileName:   filepath.Base(rel),
			LocalPath:  path,
			RemotePath: st.remoteUploadPath(rel),
			Size:       info.Size(),
			Status:     model.StrmTaskPending,
		}
		if err := st.s.repo.StrmUpload.Create(st.ctx, task); err != nil {
			st.s.log.Warn("enqueue strm upload task failed", zap.Error(err))
			return nil
		}
		st.mu.Lock()
		st.rec.Uploaded++
		st.mu.Unlock()
		return nil
	})
}

// remoteUploadPath 远端元数据目标路径 = 同步目录远端根 + 相对路径。
func (st *strmSyncState) remoteUploadPath(rel string) string {
	root := strings.TrimRight(normalizeRemotePath(st.p.RemotePath), "/")
	if root == "/" || root == "" {
		return "/" + rel
	}
	return root + "/" + rel
}

// taskExists 检查是否已有同目录、同目标的进行中/已完成任务（避免重复入队）。
func (st *strmSyncState) taskExists(kind, syncPathID, localPath string) bool {
	ctx := st.ctx
	var count int64
	switch kind {
	case "download":
		count = st.s.repo.StrmDownload.CountActive(ctx, syncPathID, localPath)
	default:
		count = st.s.repo.StrmUpload.CountActive(ctx, syncPathID, localPath)
	}
	return count > 0
}

// pruneLocal 清理本地多余 .strm 与元数据（远端已不存在），可选删除空目录。
func (st *strmSyncState) pruneLocal() error {
	localRoot := filepath.Clean(st.p.LocalPath)
	var dirs []string
	err := filepath.WalkDir(localRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == localRoot {
			return nil
		}
		select {
		case <-st.ctx.Done():
			return st.ctx.Err()
		default:
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		rel, err := filepath.Rel(localRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		remove := false
		switch {
		case ext == ".strm":
			relSansExt := rel[:len(rel)-len(ext)]
			st.mu.Lock()
			remove = !st.seenVideo["v:"+relSansExt]
			st.mu.Unlock()
		case st.isMetaExt(ext) && st.cfg.DownloadMeta && !st.cfg.UploadMeta && st.p.Provider != model.StrmProviderLocal:
			st.mu.Lock()
			remove = !st.seenMeta["m:"+rel]
			st.mu.Unlock()
		}
		if remove {
			if err := os.Remove(path); err == nil {
				st.mu.Lock()
				st.rec.Pruned++
				st.mu.Unlock()
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if st.cfg.DeleteDir {
		sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
		for _, dir := range dirs {
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) == 0 {
				_ = os.Remove(dir)
			}
		}
	}
	return nil
}

// touchProgress 每处理若干个文件落库一次进度。
func (st *strmSyncState) touchProgress() {
	st.mu.Lock()
	st.rec.Total++
	st.processed++
	flush := st.processed%100 == 0
	st.mu.Unlock()
	if flush {
		st.flushProgress()
	}
}

func (st *strmSyncState) flushProgress() {
	st.mu.Lock()
	rec := *st.rec
	st.mu.Unlock()
	if err := st.s.repo.StrmSyncRecord.Update(st.ctx, &rec); err != nil {
		st.s.log.Warn("update strm sync progress failed", zap.Error(err))
	}
}

// ─── 定时同步巡检 ──────────────────────────────────────────────────────────────

func (s *StrmService) cronLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			paths, err := s.repo.StrmSyncPath.List(ctx)
			if err != nil {
				continue
			}
			for i := range paths {
				p := &paths[i]
				if !p.Enabled || !p.EnableCron || strings.TrimSpace(p.Cron) == "" {
					continue
				}
				if !cronMatches(p.Cron, now) {
					continue
				}
				s.mu.Lock()
				_, running := s.running[p.ID]
				s.mu.Unlock()
				if running {
					continue
				}
				s.log.Info("strm cron triggered sync", zap.String("path_id", p.ID), zap.String("cron", p.Cron))
				if err := s.StartSync(ctx, p.ID); err != nil {
					s.log.Warn("strm cron sync start failed", zap.String("path_id", p.ID), zap.Error(err))
				}
			}
		}
	}
}

// cronMatches 匹配 5 段 cron 表达式（分 时 日 月 周）。支持 * ? /n a-b a,b 组合。
func cronMatches(expr string, t time.Time) bool {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return false
	}
	now := []int{t.Minute(), t.Hour(), t.Day(), int(t.Month()), int(t.Weekday())}
	if now[4] == 0 {
		now[4] = 7 // 周日常量统一为 7
	}
	for i, field := range fields {
		if !cronFieldMatches(field, now[i]) {
			return false
		}
	}
	return true
}

// cronFieldMatches 匹配单个 cron 字段。
func cronFieldMatches(field string, value int) bool {
	if field == "*" || field == "?" {
		return true
	}
	for _, item := range strings.Split(field, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(item, "*/"))
			if err != nil || step <= 0 {
				continue
			}
			if value%step == 0 {
				return true
			}
			continue
		}
		if strings.Contains(item, "-") {
			bounds := strings.SplitN(item, "-", 2)
			lo, errLo := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, errHi := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if errLo == nil && errHi == nil && lo <= hi && value >= lo && value <= hi {
				return true
			}
			continue
		}
		n, err := strconv.Atoi(item)
		if err == nil && n == value {
			return true
		}
	}
	return false
}
