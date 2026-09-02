package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
)

// PlayProfileRepository persists model.PlayProfile records.
type PlayProfileRepository struct{ db *gorm.DB }

// Create inserts a new play profile.
func (r *PlayProfileRepository) Create(ctx context.Context, p *model.PlayProfile) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// FindByID returns the profile or (nil, nil).
func (r *PlayProfileRepository) FindByID(ctx context.Context, id string) (*model.PlayProfile, error) {
	var p model.PlayProfile
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List returns every profile (admin view).
func (r *PlayProfileRepository) List(ctx context.Context) ([]model.PlayProfile, error) {
	var rows []model.PlayProfile
	err := r.db.WithContext(ctx).Order("created_at desc").Find(&rows).Error
	return rows, err
}

// ListByUser returns profiles owned by a user.
func (r *PlayProfileRepository) ListByUser(ctx context.Context, userID string) ([]model.PlayProfile, error) {
	var rows []model.PlayProfile
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at desc").Find(&rows).Error
	return rows, err
}

// CountByUser returns the number of active profiles owned by a user.
func (r *PlayProfileRepository) CountByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.PlayProfile{}).
		Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

// Update applies a partial update to a profile row.
func (r *PlayProfileRepository) Update(ctx context.Context, id string, patch map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.PlayProfile{}).
		Where("id = ?", id).Updates(patch).Error
}

// Delete 物理删除播放档案。
func (r *PlayProfileRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.PlayProfile{}, "id = ?", id).Error
}

// ClearDefaultsFor resets is_default for all of a user's profiles.
func (r *PlayProfileRepository) ClearDefaultsFor(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Model(&model.PlayProfile{}).
		Where("user_id = ?", userID).Update("is_default", false).Error
}
