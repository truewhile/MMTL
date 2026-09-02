package database

import (
	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/model"
)

// AutoMigrate creates tables for every model registered in the model package.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return err
	}
	if err := ensurePostgresColumnCompatibility(db); err != nil {
		return err
	}
	if err := ensurePerformanceIndexes(db); err != nil {
		return err
	}
	if err := ensureLibraryRootsCompatibility(db); err != nil {
		return err
	}
	if err := ensureEmbyMountsCompatibility(db); err != nil {
		return err
	}
	if isSQLite(db) {
		if err := ensureMediaSearchIndex(db); err != nil {
			return err
		}
		return ensureSQLiteQueryOptimizer(db)
	}
	return nil
}

func ensureSQLiteQueryOptimizer(db *gorm.DB) error {
	// Refresh planner statistics so indexes on large media tables are used.
	return db.Exec("ANALYZE").Error
}

func ensurePostgresColumnCompatibility(db *gorm.DB) error {
	if !isPostgres(db) {
		return nil
	}
	statements := []string{
		`ALTER TABLE media ALTER COLUMN container TYPE varchar(128)`,
		`ALTER TABLE media ALTER COLUMN genres TYPE text`,
		`ALTER TABLE media ALTER COLUMN series_id TYPE varchar(128)`,
		`ALTER TABLE media ALTER COLUMN duplicate_of TYPE varchar(128)`,
		`ALTER TABLE playback_histories ALTER COLUMN media_id TYPE varchar(128)`,
		`ALTER TABLE favorites ALTER COLUMN media_id TYPE varchar(128)`,
		`ALTER TABLE playlist_items ALTER COLUMN media_id TYPE varchar(128)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensurePerformanceIndexes(db *gorm.DB) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_media_library_created_active ON media(library_id, created_at DESC) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_media_library_release_active ON media(library_id, release_date DESC, year DESC) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_media_library_episode_active ON media(library_id, season_num, episode_num, created_at DESC) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_media_library_root_active ON media(library_id, library_root_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_media_series_active ON media(series_id, season_num, episode_num) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_favorites_user_media_active ON favorites(user_id, media_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_playback_histories_user_media_active ON playback_histories(user_id, media_id, watched_at DESC) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_playback_histories_resume_active ON playback_histories(user_id, completed, watched_at DESC) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_play_profiles_user_created_active ON play_profiles(user_id, created_at DESC) WHERE deleted_at IS NULL`,
	}
	if isSQLite(db) {
		statements = append(statements,
			`CREATE INDEX IF NOT EXISTS idx_media_title_active ON media(title COLLATE NOCASE) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_media_original_name_active ON media(original_name COLLATE NOCASE) WHERE deleted_at IS NULL`,
		)
	} else {
		statements = append(statements,
			`CREATE INDEX IF NOT EXISTS idx_media_title_active ON media(title) WHERE deleted_at IS NULL`,
			`CREATE INDEX IF NOT EXISTS idx_media_original_name_active ON media(original_name) WHERE deleted_at IS NULL`,
		)
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureEmbyMountsCompatibility(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.EmbyMount{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&model.EmbyMount{}, "sort_order") {
		if err := db.Migrator().AddColumn(&model.EmbyMount{}, "sort_order"); err != nil {
			return err
		}
	}
	// 针对已有数据：如果存在多个 sort_order=0/NULL 的记录，按创建时间顺序赋予稳定递增的序号
	var zeroCount int64
	if err := db.Model(&model.EmbyMount{}).Where("sort_order = 0 OR sort_order IS NULL").Count(&zeroCount).Error; err == nil && zeroCount > 1 {
		var mounts []model.EmbyMount
		if err := db.Order("created_at asc, id asc").Find(&mounts).Error; err == nil {
			for i, m := range mounts {
				_ = db.Exec("UPDATE emby_mounts SET sort_order = ? WHERE id = ?", i, m.ID).Error
			}
		}
	}
	return nil
}
