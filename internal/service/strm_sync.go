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
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/service/cloud"
	"github.com/ShukeBta/MMTL/internal/service/cloud115"
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
	syncType string

	mu                  sync.Mutex
	processed           int              // 已处理文件计数（用于定期落库进度）
	lastProgressFlush   time.Time        // 上次进度落库时间
	seenVideo           map[string]bool  // "v:"+去掉扩展名的相对路径 → 远端存在该视频
	seenMeta            map[string]bool  // "m:"+相对路径 → 远端存在该元数据
	remoteMeta          map[string]int64 // 远端元数据大小（上传比对用）
	seenMetaTarget      map[string]cloud.FileEntry
	seenVideoTarget     map[string]cloud.FileEntry
	activeDownloadPaths map[string]bool // 本地已在排队/进行的下载任务路径（内存去重）
	activeUploadPaths   map[string]bool // 本地已在排队/进行的上传任务路径（内存去重）
	pendingDownloads    []*model.StrmDownloadTask
	pendingUploads      []*model.StrmUploadTask
	dirCache            sync.Map // dirID (string) -> relativePath (string)
}

// StartSync 启动一次同步（异步执行，同一目录同时只允许一个任务）。
// syncType 支持 "incremental"（默认增量）和 "full"（全量同步）。
func (s *StrmService) StartSync(ctx context.Context, pathID string, syncType ...string) error {
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

	mode := model.StrmSyncTypeIncremental
	if len(syncType) > 0 && syncType[0] != "" {
		mode = syncType[0]
	} else if p.SyncMode != "" {
		mode = p.SyncMode
	}
	if mode != model.StrmSyncTypeFull {
		mode = model.StrmSyncTypeIncremental
	}

	now := time.Now()
	rec := &model.StrmSyncRecord{
		SyncPathID: pathID,
		SyncType:   mode,
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

// CancelSync 取消正在进行的同步（若为僵尸运行状态则直接自愈重置）。
func (s *StrmService) CancelSync(ctx context.Context, pathID string) error {
	s.mu.Lock()
	cancel, exists := s.running[pathID]
	if exists {
		delete(s.running, pathID)
	}
	s.mu.Unlock()

	if exists && cancel != nil {
		cancel()
	}

	// 无论内存中是否活跃，确保同步目录状态正确重置为已取消
	if p, err := s.repo.StrmSyncPath.FindByID(ctx, pathID); err == nil && p != nil {
		if p.LastSyncStatus == model.StrmSyncRecordRunning {
			p.LastSyncStatus = model.StrmSyncRecordCanceled
			p.LastSyncMessage = "已取消"
			_ = s.repo.StrmSyncPath.Update(ctx, p)
		}
	}
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
		s:               s,
		ctx:             ctx,
		p:               p,
		cfg:             cfg,
		rec:             rec,
		syncType:        rec.SyncType,
		seenVideo:       map[string]bool{},
		seenMeta:        map[string]bool{},
		remoteMeta:      map[string]int64{},
		seenMetaTarget:  map[string]cloud.FileEntry{},
		seenVideoTarget: map[string]cloud.FileEntry{},
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
		syncTypeLabel := "增量"
		if rec.SyncType == model.StrmSyncTypeFull {
			syncTypeLabel = "全量"
		}
		p.LastSyncMessage = fmt.Sprintf("[%s] 完成：新增/更新 %d 个 strm，跳过 %d 个，下载 %d 个元数据，清理 %d 个文件",
			syncTypeLabel, rec.NewStrm, rec.Skipped, rec.NewMeta, rec.Pruned)
	}
	if err := s.repo.StrmSyncPath.Update(context.Background(), p); err != nil {
		s.log.Warn("update strm sync path failed", zap.Error(err))
	}
	s.log.Info("strm sync finished",
		zap.String("path_id", p.ID), zap.String("sync_type", rec.SyncType), zap.String("status", status),
		zap.Int64("new_strm", rec.NewStrm), zap.Int64("skipped", rec.Skipped), zap.Int64("new_meta", rec.NewMeta),
		zap.Int64("pruned", rec.Pruned), zap.String("message", message))
}

func (st *strmSyncState) run() error {
	if err := ensureLocalDir(st.p.LocalPath); err != nil {
		return fmt.Errorf("创建输出目录失败：%w", err)
	}
	if st.cfg.DownloadMeta {
		if active, err := st.s.repo.StrmDownload.GetActiveLocalPathMap(st.ctx, st.p.ID); err == nil {
			st.activeDownloadPaths = active
		} else {
			st.activeDownloadPaths = map[string]bool{}
		}
	}
	if st.cfg.UploadMeta {
		if active, err := st.s.repo.StrmUpload.GetActiveLocalPathMap(st.ctx, st.p.ID); err == nil {
			st.activeUploadPaths = active
		} else {
			st.activeUploadPaths = map[string]bool{}
		}
	}

	if st.provider != nil {
		if open115, ok := st.provider.(cloud.OpenAPI115Provider); ok && st.p.Provider == model.StrmProvider115 {
			if err := st.walk115Flat(open115.OpenClient()); err != nil {
				return err
			}
		} else {
			if err := st.walkRemote(); err != nil {
				return err
			}
		}
	} else {
		if err := st.walkLocalSource(); err != nil {
			return err
		}
	}
	st.flushPendingDownloads()
	st.flushProgress()
	if st.cfg.UploadMeta && st.provider != nil && st.p.Provider != model.StrmProvider115 {
		if err := st.scanLocalMetaForUpload(); err != nil {
			return err
		}
		st.flushPendingUploads()
	}
	if err := st.pruneLocal(); err != nil {
		return err
	}
	st.flushProgress()
	_ = st.ctx.Err()
	return nil
}

// strmScanWorkers 远端目录树并发遍历的 worker 数。115 开放平台有全局
// 令牌桶限流（QPS/QPM/QPH），并发请求自动排队，不会触发风控；并发让
// 多个目录列表请求的网络往返彼此重叠，大幅缩短大目录树同步耗时。
const strmScanWorkers = 8

// walkRemote 并发广度优先遍历网盘目录树。
// 多个 worker 并行执行 List（受全局 115 令牌桶限流约束），子目录动态
// 入队；任一目录失败则取消其余 worker 并返回错误（与旧串行版语义一致）。
func (st *strmSyncState) walkRemote() error {
	defer st.flushPendingDownloads()
	root := strings.TrimSpace(st.p.RemotePath)
	if root == "" {
		root = "/"
	}
	type dirTask struct {
		id  string
		rel string
	}

	ctx, cancel := context.WithCancel(st.ctx)
	defer cancel()

	queue := make(chan dirTask, 512)
	var pending atomic.Int64

	// 根目录入队
	pending.Add(1)
	queue <- dirTask{id: root, rel: ""}

	// 当队列中所有目录都被消费（pending 归零）或出错时关闭 channel，
	// 让 worker 全部退出。
	go func() {
		for {
			if ctx.Err() != nil || pending.Load() == 0 {
				close(queue)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	for i := 0; i < strmScanWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range queue {
				if ctx.Err() != nil {
					return
				}
				entries, err := st.provider.List(ctx, task.id)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("列出远端目录 %s 失败：%w", task.id, err)
					}
					errMu.Unlock()
					cancel()
					return
				}
				for _, entry := range entries {
					cleanName := cleanEntryName(entry.Name, entry.IsDir)
					rel := cleanName
					if task.rel != "" {
						rel = task.rel + "/" + cleanName
					}
					if entry.IsDir {
						pending.Add(1)
						select {
						case queue <- dirTask{id: entry.ID, rel: rel}:
						case <-ctx.Done():
							pending.Add(-1)
						}
					} else {
						st.processRemoteFile(entry, rel)
					}
				}
				pending.Add(-1)
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
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
	case st.isMetaExt(ext):
		st.recordRemoteMeta(entry, rel)
		if st.cfg.DownloadMeta {
			st.handleMeta(entry, rel, ext)
		} else {
			st.touchProgress()
		}
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

// walk115Flat 使用 115 开放平台扁平化分页批量拉取机制与目录拓扑缓存（参考 QMediaSync）。
// 极大地降低 API 请求次数并支持毫秒级/秒级增量同步。
func (st *strmSyncState) walk115Flat(open115 *cloud115.OpenClient) error {
	defer st.flushPendingDownloads()
	ctx := st.ctx
	rootCID := strings.TrimSpace(st.p.RemotePath)
	if rootCID == "" {
		rootCID = "0"
	}

	// 1. 目录拓扑缓存处理
	st.dirCache.Store(rootCID, "")
	if st.syncType == model.StrmSyncTypeFull {
		// 全量同步：清空本路径的历史目录缓存
		if err := st.s.repo.StrmDirCache.DeleteBySyncPathID(ctx, st.p.ID); err != nil {
			st.s.log.Warn("delete strm dir cache failed", zap.Error(err))
		}
		} else {
			// 增量同步：预加载历史目录缓存（过滤历史一对多塌陷冲突的脏数据以自愈刷新）
			cached, err := st.s.repo.StrmDirCache.ListBySyncPathID(ctx, st.p.ID)
			if err == nil {
				pathCounts := make(map[string]int, len(cached))
				for _, item := range cached {
					pathCounts[item.Path]++
				}
				for _, item := range cached {
					// 若同一个 path 对应了多个不同 dir_id，说明包含历史层级塌陷的脏数据，不预加载，让后续步骤重新向 115 获取精确路径
					if pathCounts[item.Path] > 1 {
						continue
					}
					st.dirCache.Store(item.DirID, item.Path)
				}
			}
		}

	// 2. 探测文件总数
	const pageSize = 1150
	firstBatch, totalCount, err := open115.GetFsListFlat(ctx, rootCID, 0, pageSize)
	if err != nil {
		return fmt.Errorf("115: 获取文件列表失败：%w", err)
	}

	st.updateSyncMessage(fmt.Sprintf("正在拉取远端文件列表 (共 %d 个文件)...", totalCount))

	allFiles := make([]cloud115.RemoteFile, 0, totalCount)
	allFiles = append(allFiles, firstBatch...)

	// 3. 并发分页拉取剩余文件
	if totalCount > int64(len(firstBatch)) {
		totalPages := int((totalCount + pageSize - 1) / pageSize)
		type pageTask struct {
			offset int
		}
		pageTasks := make([]pageTask, 0, totalPages-1)
		for page := 1; page < totalPages; page++ {
			pageTasks = append(pageTasks, pageTask{offset: page * pageSize})
		}

		var (
			filesMu  sync.Mutex
			wg       sync.WaitGroup
			taskCh   = make(chan pageTask, len(pageTasks))
			errMu    sync.Mutex
			fetchErr error
		)

		for _, t := range pageTasks {
			taskCh <- t
		}
		close(taskCh)

		workers := 8
		if len(pageTasks) < workers {
			workers = len(pageTasks)
		}

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range taskCh {
					if ctx.Err() != nil {
						return
					}
					files, _, err := open115.GetFsListFlat(ctx, rootCID, t.offset, pageSize)
					if err != nil {
						errMu.Lock()
						if fetchErr == nil {
							fetchErr = err
						}
						errMu.Unlock()
						return
					}
					filesMu.Lock()
					allFiles = append(allFiles, files...)
					filesMu.Unlock()
				}
			}()
		}
		wg.Wait()
		if fetchErr != nil {
			return fmt.Errorf("115: 分页拉取失败：%w", fetchErr)
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 4. 收集所有未在缓存中的父目录 ID (file.Pid)
	missingPids := make(map[string]struct{})
	for _, f := range allFiles {
		pid := f.Pid
		if pid == "" || pid == rootCID {
			continue
		}
		if _, ok := st.dirCache.Load(pid); !ok {
			missingPids[pid] = struct{}{}
		}
	}

	// 并发补全未知目录详情与祖先链
	if len(missingPids) > 0 {
		pidList := make([]string, 0, len(missingPids))
		for pid := range missingPids {
			pidList = append(pidList, pid)
		}

		pidCh := make(chan string, len(pidList))
		for _, pid := range pidList {
			pidCh <- pid
		}
		close(pidCh)

		var (
			pwg        sync.WaitGroup
			dirWorkers = 8
			doneDirs   atomic.Int64
			totalDirs  = len(pidList)
		)
		if len(pidList) < dirWorkers {
			dirWorkers = len(pidList)
		}

		st.updateSyncMessage(fmt.Sprintf("正在解析目录树 (0/%d)...", totalDirs))

		for i := 0; i < dirWorkers; i++ {
			pwg.Add(1)
			go func() {
				defer pwg.Done()
				for pid := range pidCh {
					if ctx.Err() != nil {
						return
					}
					if _, loaded := st.dirCache.Load(pid); loaded {
						if n := doneDirs.Add(1); n%20 == 0 || n == int64(totalDirs) {
							st.updateSyncMessage(fmt.Sprintf("正在解析目录树 (%d/%d)...", n, totalDirs))
						}
						continue
					}
					detail, err := open115.GetFsDetailByCid(ctx, pid)
					if err != nil {
						st.s.log.Warn("115: 获取目录详情失败", zap.String("pid", pid), zap.Error(err))
					} else if detail != nil {
						// 解析相对路径
						relPath := detail.RelativePath(rootCID)
						st.dirCache.Store(pid, relPath)
						_ = st.s.repo.StrmDirCache.Set(ctx, st.p.ID, pid, relPath)

						// 顺便解析并缓存 detail.Paths 中包含的中间各层级目录
						for _, ancestor := range detail.Paths {
							if ancestor.FileId == "0" || ancestor.FileId == rootCID {
								continue
							}
							if _, loaded := st.dirCache.Load(ancestor.FileId); !loaded {
								subDetail := &cloud115.RemoteFileDetail{
									FileId:   ancestor.FileId,
									FileName: ancestor.Name,
									Paths:    nil,
								}
								for _, p := range detail.Paths {
									subDetail.Paths = append(subDetail.Paths, p)
									if p.FileId == ancestor.FileId {
										break
									}
								}
								ancestorRel := subDetail.RelativePath(rootCID)
								st.dirCache.Store(ancestor.FileId, ancestorRel)
								_ = st.s.repo.StrmDirCache.Set(ctx, st.p.ID, ancestor.FileId, ancestorRel)
							}
						}
					}
					if n := doneDirs.Add(1); n%10 == 0 || n == int64(totalDirs) {
						st.updateSyncMessage(fmt.Sprintf("正在解析目录树 (%d/%d)...", n, totalDirs))
					}
				}
			}()
		}
		pwg.Wait()
	}

	st.updateSyncMessage(fmt.Sprintf("正在生成 STRM 与同步文件 (共 %d 个)...", len(allFiles)))

	// 5. 分类处理所有文件
	for _, f := range allFiles {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		cleanName := cleanEntryName(f.FileName, false)
		var rel string
		if f.Pid == "" || f.Pid == rootCID {
			rel = cleanName
		} else {
			if parentVal, ok := st.dirCache.Load(f.Pid); ok && parentVal.(string) != "" {
				rel = parentVal.(string) + "/" + cleanName
			} else {
				rel = cleanName
			}
		}
		entry := cloud.FileEntry{
			ID:       f.FileId,
			Name:     f.FileName,
			IsDir:    false,
			Size:     f.FileSize,
			MTime:    f.Utime,
			PickCode: f.PickCode,
		}
		st.processRemoteFile(entry, rel)
	}

	return nil
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

	st.mu.Lock()
	if st.seenVideoTarget == nil {
		st.seenVideoTarget = map[string]cloud.FileEntry{}
	}
	if _, exists := st.seenVideoTarget[target]; exists {
		st.mu.Unlock()
		st.touchProgress()
		return
	}
	st.seenVideoTarget[target] = entry
	st.mu.Unlock()

	// 增量同步模式快速检查：本地 strm 文件存在、非空且修改时间与远端 mtime 一致，直接跳过无需读磁盘
	if st.syncType == model.StrmSyncTypeIncremental && entry.MTime > 0 {
		if info, err := os.Stat(target); err == nil && info.Size() > 0 && info.ModTime().Unix() == entry.MTime {
			st.mu.Lock()
			st.rec.Skipped++
			st.mu.Unlock()
			st.touchProgress()
			return
		}
	}

	content, err := st.strmContent(entry, rel, ext)
	if err != nil {
		// 并发 worker 下 rec.Message 无锁写会有数据竞争，这里仅记录日志；
		// 最终同步结果的 message 由 finishSync 统一填充。
		st.s.log.Warn("build strm content failed", zap.String("file", rel), zap.Error(err))
		return
	}
	existing := ""
	if data, err := os.ReadFile(target); err == nil {
		existing = string(data)
	}
	if existing == content {
		// 对齐本地 strm 修改时间为远端 mtime，便于后续秒级比对
		if entry.MTime > 0 {
			mTime := time.Unix(entry.MTime, 0)
			_ = os.Chtimes(target, mTime, mTime)
		}
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
	if entry.MTime > 0 {
		mTime := time.Unix(entry.MTime, 0)
		_ = os.Chtimes(target, mTime, mTime)
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

// recordRemoteMeta 记录远端存在的元数据索引及文件大小。
func (st *strmSyncState) recordRemoteMeta(entry cloud.FileEntry, rel string) {
	st.mu.Lock()
	st.seenMeta["m:"+rel] = true
	st.remoteMeta["m:"+rel] = entry.Size
	st.mu.Unlock()
}

func (st *strmSyncState) flushPendingDownloads() {
	st.mu.Lock()
	if len(st.pendingDownloads) == 0 {
		st.mu.Unlock()
		return
	}
	batch := st.pendingDownloads
	st.pendingDownloads = nil
	st.mu.Unlock()

	if err := st.s.repo.StrmDownload.CreateInBatches(st.ctx, batch, 100); err != nil {
		st.s.log.Warn("batch enqueue strm download tasks failed", zap.Error(err))
	}
}

func (st *strmSyncState) flushPendingUploads() {
	st.mu.Lock()
	if len(st.pendingUploads) == 0 {
		st.mu.Unlock()
		return
	}
	batch := st.pendingUploads
	st.pendingUploads = nil
	st.mu.Unlock()

	if err := st.s.repo.StrmUpload.CreateInBatches(st.ctx, batch, 100); err != nil {
		st.s.log.Warn("batch enqueue strm upload tasks failed", zap.Error(err))
	}
}

// handleMeta 元数据入下载队列（本地已存在且大小一致则跳过）。
func (st *strmSyncState) handleMeta(entry cloud.FileEntry, rel, ext string) {
	st.recordRemoteMeta(entry, rel)

	target, err := joinLocalRel(st.p.LocalPath, rel)
	if err != nil {
		return
	}

	st.mu.Lock()
	if st.seenMetaTarget == nil {
		st.seenMetaTarget = map[string]cloud.FileEntry{}
	}
	if _, exists := st.seenMetaTarget[target]; exists {
		// 该本地目标路径在当前批次中已被处理（存在同名/重名冲突），直接忽略重复项，避免多份不同大小的文件在本地交替覆盖导致增量死循环
		st.mu.Unlock()
		st.touchProgress()
		return
	}
	st.seenMetaTarget[target] = entry
	st.mu.Unlock()

	if info, err := os.Stat(target); err == nil && info.Size() == entry.Size {
		st.touchProgress()
		return
	}
	st.mu.Lock()
	if st.activeDownloadPaths == nil {
		if active, err := st.s.repo.StrmDownload.GetActiveLocalPathMap(st.ctx, st.p.ID); err == nil {
			st.activeDownloadPaths = active
		} else {
			st.activeDownloadPaths = map[string]bool{}
		}
	}
	if st.activeDownloadPaths[target] {
		st.mu.Unlock()
		st.touchProgress()
		return
	}
	st.activeDownloadPaths[target] = true
	st.mu.Unlock()

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

	st.mu.Lock()
	st.pendingDownloads = append(st.pendingDownloads, task)
	shouldFlush := len(st.pendingDownloads) >= 100
	st.rec.NewMeta++
	st.mu.Unlock()

	if shouldFlush {
		st.flushPendingDownloads()
	}
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
			mTime := info.ModTime()
			if st.syncType == model.StrmSyncTypeIncremental {
				if tInfo, err := os.Stat(target); err == nil && tInfo.Size() > 0 && tInfo.ModTime().Unix() == mTime.Unix() {
					st.mu.Lock()
					st.rec.Skipped++
					st.mu.Unlock()
					st.touchProgress()
					return nil
				}
			}
			if data, err := os.ReadFile(target); err == nil && string(data) == content {
				_ = os.Chtimes(target, mTime, mTime)
				st.mu.Lock()
				st.rec.Skipped++
				st.mu.Unlock()
				st.touchProgress()
				return nil
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil
			}
			tmp := target + ".tmp"
			if err := os.WriteFile(tmp, []byte(content), 0o644); err == nil {
				_ = os.Rename(tmp, target)
				_ = os.Chtimes(target, mTime, mTime)
			} else {
				_ = os.Remove(tmp)
			}
			st.mu.Lock()
			st.rec.NewStrm++
			st.mu.Unlock()
			st.touchProgress()
			return nil
	})
}

	// scanLocalMetaForUpload 扫描本地元数据，与远端比对后入上传队列。
	func (st *strmSyncState) scanLocalMetaForUpload() error {
		defer st.flushPendingUploads()
		if st.activeUploadPaths == nil {
			if active, err := st.s.repo.StrmUpload.GetActiveLocalPathMap(st.ctx, st.p.ID); err == nil {
				st.activeUploadPaths = active
			} else {
				st.activeUploadPaths = map[string]bool{}
			}
		}
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
			_, exists := st.remoteMeta["m:"+rel]
			st.mu.Unlock()
			if exists {
				// 网盘端已存在该元数据文件，跳过上传
				return nil
			}
			st.mu.Lock()
			if st.activeUploadPaths != nil && st.activeUploadPaths[path] {
				st.mu.Unlock()
				return nil
			}
			if st.activeUploadPaths != nil {
				st.activeUploadPaths[path] = true
			}
			st.mu.Unlock()

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
			st.mu.Lock()
			st.pendingUploads = append(st.pendingUploads, task)
			shouldFlush := len(st.pendingUploads) >= 100
			st.rec.Uploaded++
			st.mu.Unlock()

			if shouldFlush {
				st.flushPendingUploads()
			}
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

// touchProgress 进度计数并限流防抖落库（避免高频写 SQLite 导致锁竞争）。
func (st *strmSyncState) touchProgress() {
	st.mu.Lock()
	st.rec.Total++
	st.processed++
	now := time.Now()
	flush := st.processed%100 == 0 || (st.processed%20 == 0 && now.Sub(st.lastProgressFlush) >= 2*time.Second)
	if flush {
		st.lastProgressFlush = now
	}
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

// updateSyncMessage 实时更新同步阶段提示信息，让前端界面清晰了解当前进度。
func (st *strmSyncState) updateSyncMessage(msg string) {
	st.mu.Lock()
	st.rec.Message = msg
	st.p.LastSyncMessage = msg
	rec := *st.rec
	p := *st.p
	st.mu.Unlock()
	_ = st.s.repo.StrmSyncRecord.Update(st.ctx, &rec)
	_ = st.s.repo.StrmSyncPath.Update(st.ctx, &p)
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
