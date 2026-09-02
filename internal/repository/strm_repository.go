package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
)

var strmClaimMu sync.Mutex

// ─── StrmAccount ───────────────────────────────────────────────────────────────

// StrmAccountRepository persists model.StrmAccount.
type StrmAccountRepository struct{ db *gorm.DB }

func (r *StrmAccountRepository) Create(ctx context.Context, a *model.StrmAccount) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Create(a).Error
	})
}

func (r *StrmAccountRepository) FindByID(ctx context.Context, id string) (*model.StrmAccount, error) {
	var a model.StrmAccount
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *StrmAccountRepository) List(ctx context.Context) ([]model.StrmAccount, error) {
	var rows []model.StrmAccount
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (r *StrmAccountRepository) Update(ctx context.Context, a *model.StrmAccount) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Model(&model.StrmAccount{}).Where("id = ?", a.ID).Updates(map[string]any{
			"name":             a.Name,
			"provider":         a.Provider,
			"config":           a.Config,
			"enabled":          a.Enabled,
			"last_test_at":     a.LastTestAt,
			"last_test_result": a.LastTestResult,
			"last_test_ok":     a.LastTestOK,
			"updated_at":       time.Now(),
		}).Error
	})
}

func (r *StrmAccountRepository) Delete(ctx context.Context, id string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StrmAccount{}).Error
	})
}

// ─── StrmSyncPath ──────────────────────────────────────────────────────────────

// StrmSyncPathRepository persists model.StrmSyncPath.
type StrmSyncPathRepository struct{ db *gorm.DB }

func (r *StrmSyncPathRepository) Create(ctx context.Context, p *model.StrmSyncPath) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Create(p).Error
	})
}

func (r *StrmSyncPathRepository) FindByID(ctx context.Context, id string) (*model.StrmSyncPath, error) {
	var p model.StrmSyncPath
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *StrmSyncPathRepository) List(ctx context.Context) ([]model.StrmSyncPath, error) {
	var rows []model.StrmSyncPath
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (r *StrmSyncPathRepository) Update(ctx context.Context, p *model.StrmSyncPath) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Model(&model.StrmSyncPath{}).Where("id = ?", p.ID).Updates(map[string]any{
			"name":              p.Name,
			"account_id":        p.AccountID,
			"provider":          p.Provider,
			"remote_path":       p.RemotePath,
			"local_path":        p.LocalPath,
			"strm_base_url":     p.StrmBaseURL,
			"video_ext":         p.VideoExt,
			"meta_ext":          p.MetaExt,
			"exclude_name":      p.ExcludeName,
			"min_video_size_mb": p.MinVideoSizeMB,
			"add_path":          p.AddPath,
			"download_meta":     p.DownloadMeta,
			"upload_meta":       p.UploadMeta,
			"delete_dir":        p.DeleteDir,
			"cron":              p.Cron,
			"enable_cron":       p.EnableCron,
			"sync_mode":         p.SyncMode,
			"enabled":           p.Enabled,
			"last_sync_at":      p.LastSyncAt,
			"last_sync_status":  p.LastSyncStatus,
			"last_sync_message": p.LastSyncMessage,
			"updated_at":        time.Now(),
		}).Error
	})
}

func (r *StrmSyncPathRepository) Delete(ctx context.Context, id string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StrmSyncPath{}).Error
	})
}

// ─── StrmSyncRecord ────────────────────────────────────────────────────────────

// StrmSyncRecordRepository persists model.StrmSyncRecord.
type StrmSyncRecordRepository struct{ db *gorm.DB }

func (r *StrmSyncRecordRepository) Create(ctx context.Context, rec *model.StrmSyncRecord) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Create(rec).Error
	})
}

func (r *StrmSyncRecordRepository) Update(ctx context.Context, rec *model.StrmSyncRecord) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Model(&model.StrmSyncRecord{}).Where("id = ?", rec.ID).Updates(map[string]any{
			"sync_type":   rec.SyncType,
			"status":      rec.Status,
			"total":       rec.Total,
			"new_strm":    rec.NewStrm,
			"new_meta":    rec.NewMeta,
			"uploaded":    rec.Uploaded,
			"pruned":      rec.Pruned,
			"skipped":     rec.Skipped,
			"message":     rec.Message,
			"started_at":  rec.StartedAt,
			"finished_at": rec.FinishedAt,
			"updated_at":  time.Now(),
		}).Error
	})
}

func (r *StrmSyncRecordRepository) List(ctx context.Context, syncPathID string, limit int) ([]model.StrmSyncRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []model.StrmSyncRecord
	q := r.db.WithContext(ctx)
	if syncPathID != "" {
		q = q.Where("sync_path_id = ?", syncPathID)
	}
	err := q.Order("created_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

// Delete 删除单条同步记录（物理删除）。
func (r *StrmSyncRecordRepository) Delete(ctx context.Context, id string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StrmSyncRecord{}).Error
	})
}

// DeleteBySyncPathID 删除某同步目录下的全部同步记录（删除同步目录时级联清理）。
func (r *StrmSyncRecordRepository) DeleteBySyncPathID(ctx context.Context, syncPathID string) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("sync_path_id = ?", syncPathID).Delete(&model.StrmSyncRecord{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ─── StrmDownloadTask ──────────────────────────────────────────────────────────

// StrmDownloadTaskRepository persists model.StrmDownloadTask.
type StrmDownloadTaskRepository struct{ db *gorm.DB }

func (r *StrmDownloadTaskRepository) Create(ctx context.Context, t *model.StrmDownloadTask) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Create(t).Error
	})
}

func (r *StrmDownloadTaskRepository) CreateInBatches(ctx context.Context, tasks []*model.StrmDownloadTask, batchSize int) error {
	if len(tasks) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).CreateInBatches(tasks, batchSize).Error
	})
}

func (r *StrmDownloadTaskRepository) FindByID(ctx context.Context, id string) (*model.StrmDownloadTask, error) {
	var t model.StrmDownloadTask
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *StrmDownloadTaskRepository) List(ctx context.Context, status string, page, pageSize int) ([]model.StrmDownloadTask, int64, error) {
	page, pageSize = normalizeTaskPage(page, pageSize)
	var total int64
	if err := taskStatusScope(r.db.WithContext(ctx), status).Model(&model.StrmDownloadTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.StrmDownloadTask
	err := taskStatusScope(r.db.WithContext(ctx), status).
		Order("created_at desc, id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&rows).Error
	return rows, total, err
}

func (r *StrmDownloadTaskRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
		Select("status, count(*) as count").
		Group("status").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Status] = row.Count
	}
	return out, nil
}

// ClaimPendingDownload picks the oldest pending task and marks it running.
// Returns (nil, nil) when the queue is empty.
func (r *StrmDownloadTaskRepository) ClaimPendingDownload(ctx context.Context, limit int) ([]model.StrmDownloadTask, error) {
	strmClaimMu.Lock()
	defer strmClaimMu.Unlock()

	var rows []model.StrmDownloadTask
	err := withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("status = ? AND (next_try_at IS NULL OR next_try_at <= ?)", model.StrmTaskPending, time.Now()).
				Order("created_at asc").Limit(limit).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			ids := make([]string, 0, len(rows))
			now := time.Now()
			for i := range rows {
				ids = append(ids, rows[i].ID)
				rows[i].Status = model.StrmTaskRunning
				rows[i].StartedAt = &now
			}
			return tx.Model(&model.StrmDownloadTask{}).Where("id IN ?", ids).
				Updates(map[string]any{"status": model.StrmTaskRunning, "started_at": now}).Error
		})
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *StrmDownloadTaskRepository) Update(ctx context.Context, t *model.StrmDownloadTask) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).Where("id = ?", t.ID).Updates(map[string]any{
			"status":      t.Status,
			"error":       t.Error,
			"retry_count": t.RetryCount,
			"next_try_at": t.NextTryAt,
			"started_at":  t.StartedAt,
			"finished_at": t.FinishedAt,
			"updated_at":  time.Now(),
		}).Error
	})
}

func (r *StrmDownloadTaskRepository) Delete(ctx context.Context, id string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StrmDownloadTask{}).Error
	})
}

// DeleteBatch 批量删除指定 ID 的下载任务。
func (r *StrmDownloadTaskRepository) DeleteBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Delete(&model.StrmDownloadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// RetryBatch 批量重试指定 ID 的失败/已取消下载任务。
func (r *StrmDownloadTaskRepository) RetryBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
			Where("id IN ? AND status IN ?", ids, []string{model.StrmTaskFailed, model.StrmTaskCanceled}).
			Updates(map[string]any{
				"status":      model.StrmTaskPending,
				"error":       "",
				"retry_count": 0,
				"next_try_at": nil,
				"started_at":  nil,
				"finished_at": nil,
				"updated_at":  time.Now(),
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CancelBatch 批量取消指定 ID 的排队/进行中下载任务。
func (r *StrmDownloadTaskRepository) CancelBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := time.Now()
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
			Where("id IN ? AND status IN ?", ids, []string{model.StrmTaskPending, model.StrmTaskRunning}).
			Updates(map[string]any{
				"status":      model.StrmTaskCanceled,
				"error":       "已批量取消",
				"finished_at": now,
				"updated_at":  now,
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearDone 清空全部已完成下载任务。
func (r *StrmDownloadTaskRepository) ClearDone(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status = ?", model.StrmTaskDone).Delete(&model.StrmDownloadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearFinished 清空全部已完成与失败下载任务。
func (r *StrmDownloadTaskRepository) ClearFinished(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status IN ?", []string{model.StrmTaskDone, model.StrmTaskFailed, model.StrmTaskCanceled}).
			Delete(&model.StrmDownloadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearCanceled 清空全部已取消下载任务。
func (r *StrmDownloadTaskRepository) ClearCanceled(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status = ?", model.StrmTaskCanceled).Delete(&model.StrmDownloadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// RetryAllFailed 把所有失败任务重置回待处理，清空错误与重试计数。
func (r *StrmDownloadTaskRepository) RetryAllFailed(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
			Where("status = ?", model.StrmTaskFailed).
			Updates(map[string]any{
				"status":      model.StrmTaskPending,
				"error":       "",
				"retry_count": 0,
				"next_try_at": nil,
				"started_at":  nil,
				"finished_at": nil,
				"updated_at":  time.Now(),
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CancelPending 批量取消所有排队中和进行中的任务。
func (r *StrmDownloadTaskRepository) CancelPending(ctx context.Context) (int64, error) {
	now := time.Now()
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
			Where("status IN ?", []string{model.StrmTaskPending, model.StrmTaskRunning}).
			Updates(map[string]any{
				"status":      model.StrmTaskCanceled,
				"error":       "已批量取消",
				"finished_at": now,
				"updated_at":  now,
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CountActive 统计某同步目录下目标仍在排队/进行的任务数（用于去重）。
func (r *StrmDownloadTaskRepository) CountActive(ctx context.Context, syncPathID, localPath string) int64 {
	var count int64
	r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
		Where("sync_path_id = ? AND local_path = ? AND status IN ?",
			syncPathID, localPath, []string{model.StrmTaskPending, model.StrmTaskRunning}).
		Count(&count)
	return count
}

// GetActiveLocalPathMap 一次性获取某同步目录下正在排队或执行中的 local_path 集合，供同步时 O(1) 内存去重。
func (r *StrmDownloadTaskRepository) GetActiveLocalPathMap(ctx context.Context, syncPathID string) (map[string]bool, error) {
	var paths []string
	err := r.db.WithContext(ctx).Model(&model.StrmDownloadTask{}).
		Where("sync_path_id = ? AND status IN ?", syncPathID, []string{model.StrmTaskPending, model.StrmTaskRunning}).
		Pluck("local_path", &paths).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[p] = true
	}
	return out, nil
}

func (r *StrmDownloadTaskRepository) DeleteFinishedOlderThan(ctx context.Context, before time.Time) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("status IN ? AND finished_at < ?",
			[]string{model.StrmTaskDone, model.StrmTaskFailed, model.StrmTaskCanceled}, before).
			Delete(&model.StrmDownloadTask{}).Error
	})
}

// ─── StrmUploadTask ────────────────────────────────────────────────────────────

// StrmUploadTaskRepository persists model.StrmUploadTask.
type StrmUploadTaskRepository struct{ db *gorm.DB }

func (r *StrmUploadTaskRepository) Create(ctx context.Context, t *model.StrmUploadTask) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Create(t).Error
	})
}

func (r *StrmUploadTaskRepository) CreateInBatches(ctx context.Context, tasks []*model.StrmUploadTask, batchSize int) error {
	if len(tasks) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).CreateInBatches(tasks, batchSize).Error
	})
}

func (r *StrmUploadTaskRepository) FindByID(ctx context.Context, id string) (*model.StrmUploadTask, error) {
	var t model.StrmUploadTask
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *StrmUploadTaskRepository) List(ctx context.Context, status string, page, pageSize int) ([]model.StrmUploadTask, int64, error) {
	page, pageSize = normalizeTaskPage(page, pageSize)
	var total int64
	if err := taskStatusScope(r.db.WithContext(ctx), status).Model(&model.StrmUploadTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.StrmUploadTask
	err := taskStatusScope(r.db.WithContext(ctx), status).
		Order("created_at desc, id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&rows).Error
	return rows, total, err
}

// normalizeTaskPage 钳制分页参数：页码至少 1，单页大小 1..200。
func normalizeTaskPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	return page, pageSize
}

// taskStatusScope 按状态过滤（空状态表示不过滤）。
func taskStatusScope(db *gorm.DB, status string) *gorm.DB {
	if status != "" {
		return db.Where("status = ?", status)
	}
	return db
}

func (r *StrmUploadTaskRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	err := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
		Select("status, count(*) as count").
		Group("status").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Status] = row.Count
	}
	return out, nil
}

// ClaimPendingUpload picks the oldest pending task and marks it running.
// Returns (nil, nil) when the queue is empty.
func (r *StrmUploadTaskRepository) ClaimPendingUpload(ctx context.Context, limit int) ([]model.StrmUploadTask, error) {
	strmClaimMu.Lock()
	defer strmClaimMu.Unlock()

	var rows []model.StrmUploadTask
	err := withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("status = ? AND (next_try_at IS NULL OR next_try_at <= ?)", model.StrmTaskPending, time.Now()).
				Order("created_at asc").Limit(limit).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			ids := make([]string, 0, len(rows))
			now := time.Now()
			for i := range rows {
				ids = append(ids, rows[i].ID)
				rows[i].Status = model.StrmTaskRunning
				rows[i].StartedAt = &now
			}
			return tx.Model(&model.StrmUploadTask{}).Where("id IN ?", ids).
				Updates(map[string]any{"status": model.StrmTaskRunning, "started_at": now}).Error
		})
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *StrmUploadTaskRepository) Update(ctx context.Context, t *model.StrmUploadTask) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).Where("id = ?", t.ID).Updates(map[string]any{
			"status":      t.Status,
			"error":       t.Error,
			"retry_count": t.RetryCount,
			"next_try_at": t.NextTryAt,
			"started_at":  t.StartedAt,
			"finished_at": t.FinishedAt,
			"updated_at":  time.Now(),
		}).Error
	})
}

func (r *StrmUploadTaskRepository) Delete(ctx context.Context, id string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StrmUploadTask{}).Error
	})
}

// DeleteBatch 批量删除指定 ID 的上传任务。
func (r *StrmUploadTaskRepository) DeleteBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("id IN ?", ids).Delete(&model.StrmUploadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// RetryBatch 批量重试指定 ID 的失败/已取消上传任务。
func (r *StrmUploadTaskRepository) RetryBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
			Where("id IN ? AND status IN ?", ids, []string{model.StrmTaskFailed, model.StrmTaskCanceled}).
			Updates(map[string]any{
				"status":      model.StrmTaskPending,
				"error":       "",
				"retry_count": 0,
				"next_try_at": nil,
				"started_at":  nil,
				"finished_at": nil,
				"updated_at":  time.Now(),
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CancelBatch 批量取消指定 ID 的排队/进行中上传任务。
func (r *StrmUploadTaskRepository) CancelBatch(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := time.Now()
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
			Where("id IN ? AND status IN ?", ids, []string{model.StrmTaskPending, model.StrmTaskRunning}).
			Updates(map[string]any{
				"status":      model.StrmTaskCanceled,
				"error":       "已批量取消",
				"finished_at": now,
				"updated_at":  now,
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearDone 清空全部已完成上传任务。
func (r *StrmUploadTaskRepository) ClearDone(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status = ?", model.StrmTaskDone).Delete(&model.StrmUploadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearFinished 清空全部已完成与失败上传任务（包括已完成、失败及取消）。
func (r *StrmUploadTaskRepository) ClearFinished(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status IN ?", []string{model.StrmTaskDone, model.StrmTaskFailed, model.StrmTaskCanceled}).
			Delete(&model.StrmUploadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// ClearCanceled 清空全部已取消上传任务。
func (r *StrmUploadTaskRepository) ClearCanceled(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Unscoped().Where("status = ?", model.StrmTaskCanceled).Delete(&model.StrmUploadTask{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// RetryAllFailed 把所有失败任务重置回待处理，清空错误与重试计数。
func (r *StrmUploadTaskRepository) RetryAllFailed(ctx context.Context) (int64, error) {
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
			Where("status = ?", model.StrmTaskFailed).
			Updates(map[string]any{
				"status":      model.StrmTaskPending,
				"error":       "",
				"retry_count": 0,
				"next_try_at": nil,
				"started_at":  nil,
				"finished_at": nil,
				"updated_at":  time.Now(),
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CancelPending 批量取消所有排队中和进行中的任务。
func (r *StrmUploadTaskRepository) CancelPending(ctx context.Context) (int64, error) {
	now := time.Now()
	var count int64
	err := withSQLiteBusyRetry(ctx, func() error {
		res := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
			Where("status IN ?", []string{model.StrmTaskPending, model.StrmTaskRunning}).
			Updates(map[string]any{
				"status":      model.StrmTaskCanceled,
				"error":       "已批量取消",
				"finished_at": now,
				"updated_at":  now,
			})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// CountActive 统计某同步目录下目标仍在排队/进行的任务数（用于去重）。
func (r *StrmUploadTaskRepository) CountActive(ctx context.Context, syncPathID, localPath string) int64 {
	var count int64
	r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
		Where("sync_path_id = ? AND local_path = ? AND status IN ?",
			syncPathID, localPath, []string{model.StrmTaskPending, model.StrmTaskRunning}).
		Count(&count)
	return count
}

// GetActiveLocalPathMap 一次性获取某同步目录下正在排队或执行中的 local_path 集合，供同步时 O(1) 内存去重。
func (r *StrmUploadTaskRepository) GetActiveLocalPathMap(ctx context.Context, syncPathID string) (map[string]bool, error) {
	var paths []string
	err := r.db.WithContext(ctx).Model(&model.StrmUploadTask{}).
		Where("sync_path_id = ? AND status IN ?", syncPathID, []string{model.StrmTaskPending, model.StrmTaskRunning}).
		Pluck("local_path", &paths).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[p] = true
	}
	return out, nil
}

func (r *StrmUploadTaskRepository) DeleteFinishedOlderThan(ctx context.Context, before time.Time) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("status IN ? AND finished_at < ?",
			[]string{model.StrmTaskDone, model.StrmTaskFailed, model.StrmTaskCanceled}, before).
			Delete(&model.StrmUploadTask{}).Error
	})
}

// ─── StrmDirCache ─────────────────────────────────────────────────────────────

// StrmDirCacheRepository persists model.StrmDirCache.
type StrmDirCacheRepository struct{ db *gorm.DB }

func (r *StrmDirCacheRepository) ListBySyncPathID(ctx context.Context, syncPathID string) ([]model.StrmDirCache, error) {
	var rows []model.StrmDirCache
	err := r.db.WithContext(ctx).Where("sync_path_id = ?", syncPathID).Find(&rows).Error
	return rows, err
}

func (r *StrmDirCacheRepository) Set(ctx context.Context, syncPathID, dirID, path string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		var row model.StrmDirCache
		err := r.db.WithContext(ctx).Where("sync_path_id = ? AND dir_id = ?", syncPathID, dirID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = model.StrmDirCache{
				SyncPathID: syncPathID,
				DirID:      dirID,
				Path:       path,
			}
			return r.db.WithContext(ctx).Create(&row).Error
		}
		if err != nil {
			return err
		}
		return r.db.WithContext(ctx).Model(&model.StrmDirCache{}).Where("id = ?", row.ID).Updates(map[string]any{
			"path":       path,
			"updated_at": time.Now(),
		}).Error
	})
}

func (r *StrmDirCacheRepository) DeleteBySyncPathID(ctx context.Context, syncPathID string) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return r.db.WithContext(ctx).Unscoped().Where("sync_path_id = ?", syncPathID).Delete(&model.StrmDirCache{}).Error
	})
}
