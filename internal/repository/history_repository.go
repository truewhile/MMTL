package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/truewhile/MeBox/internal/model"
)

// HistoryRepository persists model.PlaybackHistory entries. The application
// upserts on (UserID, MediaID) so resume always reads the latest position.
type HistoryRepository struct{ db *gorm.DB }

// Upsert atomically inserts/updates the resume position in a single statement,
// relying on the uniq_user_history composite unique index. Concurrent progress
// reports for the same (user, media) can no longer double-insert.
func (r *HistoryRepository) Upsert(ctx context.Context, h *model.PlaybackHistory) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "media_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"position_ms": h.PositionMs,
			// 沿用旧语义：未知时长（0）不覆盖已记录的时长。
			"duration_ms": gorm.Expr(
				"CASE WHEN ? > 0 THEN ? ELSE playback_histories.duration_ms END",
				h.DurationMs, h.DurationMs,
			),
			"watched_at": h.WatchedAt,
			"completed":  h.Completed,
			"deleted_at": nil,
		}),
	}).Create(h).Error
}

// ListByUser returns the most recent history rows for the user.
func (r *HistoryRepository) ListByUser(ctx context.Context, userID string, limit int) ([]model.PlaybackHistory, error) {
	var rows []model.PlaybackHistory
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("watched_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}
