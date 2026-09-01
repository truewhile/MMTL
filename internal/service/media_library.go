package service

import (
	"context"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"gorm.io/gorm"
)

type LibraryPreviewItem struct {
	model.Library
	Total int64        `json:"total"`
	Cards []SeriesCard `json:"cards"`
}

// ListLibraries returns every library configured on the server.
func (s *MediaService) ListLibraries(ctx context.Context) ([]model.Library, error) {
	return s.repo.Library.List(ctx)
}

// ListLibrariesWithPreview returns libraries populated with item counts and latest preview cards.
func (s *MediaService) ListLibrariesWithPreview(ctx context.Context, libraries []model.Library, visibility MediaVisibility, cardLimit int) ([]LibraryPreviewItem, error) {
	if cardLimit <= 0 {
		cardLimit = 10
	}
	out := make([]LibraryPreviewItem, len(libraries))
	if len(libraries) == 0 {
		return out, nil
	}

	libIDs := make([]string, 0, len(libraries))
	for i, lib := range libraries {
		out[i] = LibraryPreviewItem{
			Library: lib,
			Total:   0,
			Cards:   []SeriesCard{},
		}
		libIDs = append(libIDs, lib.ID)
	}

	visibility = ExpandMediaVisibilityForMergedCloudLibraries(ctx, s.repo, visibility)
	filter := repository.MediaQueryFilter{
		IncludeNSFW:       visibility.IncludeNSFW,
		AllowedLibraryIDs: visibility.AllowedLibraryIDs,
		HiddenLibraryIDs:  visibility.HiddenLibraryIDs,
	}

	counts, err := s.repo.Media.CountByLibraries(ctx, libIDs, filter)
	if err != nil {
		return nil, err
	}

	for i := range out {
		if total, ok := counts[out[i].ID]; ok {
			out[i].Total = total
		}
	}

	fetchCount := cardLimit * 4
	if fetchCount < 60 {
		fetchCount = 60
	} else if fetchCount > 200 {
		fetchCount = 200
	}

	for i := range out {
		if out[i].Total == 0 {
			continue
		}
		items, _, err := s.repo.Media.ListByLibrariesFiltered(ctx, []string{out[i].ID}, 0, fetchCount, filter)
		if err != nil {
			continue
		}
		s.attachLibraryMetadata(ctx, items)
		cards := groupMediaSeriesCards(items)
		if len(cards) > cardLimit {
			cards = cards[:cardLimit]
		}
		if cards == nil {
			cards = []SeriesCard{}
		}
		out[i].Cards = cards
	}

	return out, nil
}

// DeleteLibrary removes a library and its media rows. The on-disk files are
// left untouched.
func (s *MediaService) DeleteLibrary(ctx context.Context, id string) error {
	lib, err := s.repo.Library.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if lib != nil {
		if _, ok := ParseCloudLibraryMount(lib.Path); ok {
			err := s.repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Unscoped().Where("library_id = ?", id).Delete(&model.Media{}).Error; err != nil {
					return err
				}
				if err := hardDeleteLibraryRoots(ctx, tx, id); err != nil {
					return err
				}
				return tx.Unscoped().Where("id = ?", id).Delete(&model.Library{}).Error
			})
			if err == nil {
				s.invalidateMediaCache(ctx)
			}
			return err
		}
	}
	err = s.repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 本地库媒体行统一硬删除（与云库分支一致）：删库即彻底清空该库的
		// media 行，避免重扫时旧行被 upsert 复活却仍挂在已删库下，导致重建
		// 同名库后条目始终为 0。只作用于数据库表行，磁盘文件不受影响。
		if err := tx.Unscoped().Where("library_id = ?", id).Delete(&model.Media{}).Error; err != nil {
			return err
		}
		if err := hardDeleteLibraryRoots(ctx, tx, id); err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.Library{}, "id = ?", id).Error
	})
	if err == nil {
		s.invalidateMediaCache(ctx)
	}
	return err
}

func hardDeleteLibraryRoots(ctx context.Context, tx *gorm.DB, libraryID string) error {
	if tx == nil || !tx.Migrator().HasTable(&model.LibraryRoot{}) {
		return nil
	}
	return tx.WithContext(ctx).Unscoped().Where("library_id = ?", libraryID).Delete(&model.LibraryRoot{}).Error
}
