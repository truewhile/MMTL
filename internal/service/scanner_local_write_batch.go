package service

import (
	"context"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/model"
)

type localMediaWriteBatch struct {
	scanner *ScannerService
	ctx     context.Context
	res     *ScanResult
	limit   int
	items   []localMediaWriteItem
}

type localMediaWriteItem struct {
	path  string
	media *model.Media
	after func()
}

func newLocalMediaWriteBatch(scanner *ScannerService, ctx context.Context, res *ScanResult, limit int) *localMediaWriteBatch {
	if limit <= 0 {
		limit = 100
	}
	return &localMediaWriteBatch{scanner: scanner, ctx: ctx, res: res, limit: limit}
}

func (b *localMediaWriteBatch) Add(path string, media *model.Media) {
	b.AddWithAfter(path, media, nil)
}

func (b *localMediaWriteBatch) AddWithAfter(path string, media *model.Media, after func()) {
	if b == nil || b.scanner == nil || media == nil {
		return
	}
	if media.ScrapeStatus == "" {
		media.ScrapeStatus = "pending"
	}
	b.items = append(b.items, localMediaWriteItem{path: path, media: media, after: after})
	if len(b.items) >= b.limit {
		b.Flush()
	}
}

func (b *localMediaWriteBatch) Flush() {
	if b == nil || len(b.items) == 0 || b.scanner == nil || b.scanner.repo == nil || b.scanner.repo.DB == nil {
		return
	}
	items := b.items
	b.items = nil
	media := make([]model.Media, 0, len(items))
	for _, item := range items {
		if item.media != nil {
			media = append(media, *item.media)
		}
	}
	if len(media) == 0 {
		return
	}
	existingPaths := b.existingPaths(items)
	upsertItems := make([]*model.Media, 0, len(items))
	upsertAfter := make([]func(), 0, len(items))
	createItems := make([]localMediaWriteItem, 0, len(items))
	createMedia := make([]model.Media, 0, len(items))
	for _, item := range items {
		if item.media == nil {
			continue
		}
		if existingPaths[filepath.Clean(item.media.Path)] {
			// 已存在行：攒起来在一个事务里逐条 upsert（一批一次提交）。
			after := item.after
			upsertItems = append(upsertItems, item.media)
			upsertAfter = append(upsertAfter, after)
			continue
		}
		createItems = append(createItems, item)
		createMedia = append(createMedia, *item.media)
	}
	b.flushUpserts(items, upsertItems, upsertAfter)
	if len(createMedia) == 0 {
		b.publish()
		return
	}
	if err := b.scanner.repo.DB.WithContext(b.ctx).CreateInBatches(&createMedia, b.limit).Error; err == nil {
		b.res.Added += len(createMedia)
		for _, item := range createItems {
			if item.after != nil {
				item.after()
			}
		}
		b.publish()
		return
	}
	for _, item := range createItems {
		if item.media == nil {
			continue
		}
		wasExisting := b.mediaPathExists(item.media.Path)
		if err := b.scanner.repo.Media.Upsert(b.ctx, item.media); err != nil {
			addScanError(b.res, item.path, err)
			b.scanner.log.Warn("upsert media failed", zap.String("path", item.path), zap.Error(err))
			continue
		}
		if wasExisting {
			b.res.Updated++
		} else {
			b.res.Added++
		}
		if item.after != nil {
			item.after()
		}
	}
	b.publish()
}

func (b *localMediaWriteBatch) existingPaths(items []localMediaWriteItem) map[string]bool {
	out := map[string]bool{}
	if b == nil || b.scanner == nil || b.scanner.repo == nil || b.scanner.repo.DB == nil || len(items) == 0 {
		return out
	}
	paths := make([]string, 0, len(items))
	for _, item := range items {
		if item.media == nil || item.media.Path == "" {
			continue
		}
		paths = append(paths, item.media.Path)
	}
	if len(paths) == 0 {
		return out
	}
	var rows []string
	if err := b.scanner.repo.DB.WithContext(b.ctx).
		Unscoped().
		Model(&model.Media{}).
		Where("path IN ?", paths).
		Pluck("path", &rows).Error; err != nil {
		b.scanner.log.Debug("load existing media paths for scan batch failed", zap.Error(err))
		return out
	}
	for _, path := range rows {
		out[filepath.Clean(path)] = true
	}
	return out
}

// flushUpserts 把已存在行的 upsert 攒成一个事务（一次提交/一组 fsync）。
// 整批失败（如单条数据触发约束）时退回逐条 Upsert，只丢真正坏的那几条。
func (b *localMediaWriteBatch) flushUpserts(allItems []localMediaWriteItem, upsertItems []*model.Media, upsertAfter []func()) {
	if len(upsertItems) == 0 {
		return
	}
	if err := b.scanner.repo.Media.UpsertBatch(b.ctx, upsertItems); err == nil {
		b.res.Updated += len(upsertItems)
		for _, after := range upsertAfter {
			if after != nil {
				after()
			}
		}
		return
	} else if b.scanner.log != nil {
		b.scanner.log.Warn("batch upsert failed; falling back to per-item upsert",
			zap.Int("items", len(upsertItems)))
	}
	// 兜底：按原始顺序找回每个条目的 path/after（两个切片同序但可能含 nil）。
	idx := 0
	for _, item := range allItems {
		if item.media == nil || idx >= len(upsertItems) || upsertItems[idx] != item.media {
			continue
		}
		idx++
		b.upsertExistingItem(item)
	}
}

func (b *localMediaWriteBatch) upsertExistingItem(item localMediaWriteItem) {
	if item.media == nil {
		return
	}
	if err := b.scanner.repo.Media.Upsert(b.ctx, item.media); err != nil {
		addScanError(b.res, item.path, err)
		b.scanner.log.Warn("upsert media failed", zap.String("path", item.path), zap.Error(err))
		return
	}
	b.res.Updated++
	if item.after != nil {
		item.after()
	}
}

func (b *localMediaWriteBatch) mediaPathExists(path string) bool {
	if b == nil || b.scanner == nil || b.scanner.repo == nil || b.scanner.repo.DB == nil || path == "" {
		return false
	}
	var count int64
	err := b.scanner.repo.DB.WithContext(b.ctx).
		Unscoped().
		Model(&model.Media{}).
		Where("path = ?", path).
		Count(&count).Error
	return err == nil && count > 0
}

func (b *localMediaWriteBatch) publish() {
	if b == nil || b.scanner == nil || b.scanner.hub == nil || b.res == nil {
		return
	}
	b.scanner.hub.Publish("scan", map[string]any{
		"library_id": b.res.LibraryID,
		"visited":    b.res.Visited,
		"added":      b.res.Added,
		"updated":    b.res.Updated,
		"probed":     b.res.Probed,
		"local_meta": b.res.LocalMetadata,
		"batched":    true,
	})
}
