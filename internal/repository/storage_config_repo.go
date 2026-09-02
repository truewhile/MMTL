package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
)

// StorageConfigRepository persists model.StorageConfig records.
type StorageConfigRepository struct{ db *gorm.DB }

// Get returns the config row by type, or (nil, nil).
func (r *StorageConfigRepository) Get(ctx context.Context, kind string) (*model.StorageConfig, error) {
	var c model.StorageConfig
	err := r.db.WithContext(ctx).Where("type = ?", kind).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// List returns all storage configs.
func (r *StorageConfigRepository) List(ctx context.Context) ([]model.StorageConfig, error) {
	var rows []model.StorageConfig
	err := r.db.WithContext(ctx).Order("type asc").Find(&rows).Error
	return rows, err
}

// Upsert creates or replaces a storage config keyed by Type.
func (r *StorageConfigRepository) Upsert(ctx context.Context, c *model.StorageConfig) error {
	db := r.db.WithContext(ctx)
	var existing model.StorageConfig
	err := db.Where("type = ?", c.Type).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		c.ID = uuid.NewString()
		c.CreatedAt = now
		c.UpdatedAt = now
		return db.Model(&model.StorageConfig{}).Create(map[string]any{
			"id":         c.ID,
			"type":       c.Type,
			"config":     c.Config,
			"enabled":    c.Enabled,
			"last_error": c.LastError,
			"created_at": c.CreatedAt,
			"updated_at": c.UpdatedAt,
		}).Error
	}
	if err != nil {
		return err
	}
	c.ID = existing.ID
	return db.Model(&model.StorageConfig{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"config":     c.Config,
		"enabled":    c.Enabled,
		"last_error": c.LastError,
		"updated_at": time.Now(),
	}).Error
}

// Delete 物理删除存储配置。
func (r *StorageConfigRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.StorageConfig{}).Error
}

// FindByID returns a storage config by ID.
func (r *StorageConfigRepository) FindByID(ctx context.Context, id string) (*model.StorageConfig, error) {
	var c model.StorageConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}
