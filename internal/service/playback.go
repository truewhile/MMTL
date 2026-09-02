// Package service — playback history / favourites / playlists.
//
// These three concerns are intentionally co-located: they all sit between
// "the user" and "a media item" and share the same join-table flavour. A
// dedicated PlaybackService keeps the wiring simple and lets handlers
// dispatch by feature instead of by repository.
package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
)

// PlaybackService bundles history / favourite / playlist business logic.
type PlaybackService struct {
	log    *zap.Logger
	repo   *repository.Container
	remote *EmbyRemoteService
}

// NewPlaybackService is the constructor.
func NewPlaybackService(log *zap.Logger, repo *repository.Container) *PlaybackService {
	return &PlaybackService{log: log, repo: repo}
}

// SetEmbyRemote wires the remote Emby service for hydrating mounted remote items.
func (p *PlaybackService) SetEmbyRemote(remote *EmbyRemoteService) *PlaybackService {
	if p != nil {
		p.remote = remote
	}
	return p
}

// ─── History ────────────────────────────────────────────────────────────────

// RecordProgress upserts the resume position for a (user, media) pair. A
// position within 30 seconds of the duration auto-flags the item as
// completed so the home page can hide it from "Continue Watching".
func (p *PlaybackService) RecordProgress(ctx context.Context, userID, mediaID string, position, duration int64) error {
	if userID == "" || mediaID == "" {
		return errors.New("missing user or media")
	}
	dur := p.resolvePlaybackDuration(ctx, userID, mediaID, duration)
	completed := dur > 0 && position >= dur-30_000
	h := &model.PlaybackHistory{
		UserID:     userID,
		MediaID:    mediaID,
		PositionMs: position,
		DurationMs: dur,
		WatchedAt:  time.Now(),
		Completed:  completed,
	}
	return p.repo.History.Upsert(ctx, h)
}

// GetProgress returns the saved resume row for one media item, or nil when absent.
func (p *PlaybackService) GetProgress(ctx context.Context, userID, mediaID string) (*model.PlaybackHistory, error) {
	if userID == "" || mediaID == "" {
		return nil, errors.New("missing user or media")
	}
	var row model.PlaybackHistory
	err := p.repo.DB.WithContext(ctx).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (p *PlaybackService) resolvePlaybackDuration(ctx context.Context, userID, mediaID string, duration int64) int64 {
	if duration > 0 {
		return duration
	}
	var existing model.PlaybackHistory
	if err := p.repo.DB.WithContext(ctx).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		First(&existing).Error; err == nil && existing.DurationMs > 0 {
		return existing.DurationMs
	}
	if m, _ := p.repo.Media.FindByID(ctx, mediaID); m != nil && m.DurationSec > 0 {
		return int64(m.DurationSec) * 1000
	}
	if p.remote != nil && IsEmbyRemoteID(mediaID) {
		mountID, remoteID, _ := DecodeEmbyRemoteID(mediaID)
		if mount, acct, _ := p.remote.ResolveMount(ctx, mountID); mount != nil && acct != nil {
			if rm, err := p.remote.RemoteMediaDetail(ctx, mount, acct, remoteID); err == nil && rm != nil && rm.DurationSec > 0 {
				return int64(rm.DurationSec) * 1000
			}
		}
	}
	return duration
}

// HistoryItem joins the playback row with its media so the API consumer
// gets a fully-populated card without a second round-trip.
type HistoryItem struct {
	model.PlaybackHistory
	Media *model.Media `json:"media,omitempty"`
}

// RecentHistory returns the most recently-watched items for a user. We
// fetch the history rows first then attach each Media row in a single
// follow-up query.
func (p *PlaybackService) RecentHistory(ctx context.Context, userID string, limit int) ([]HistoryItem, error) {
	rows, err := p.repo.History.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	mediaIDs := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].MediaID != "" {
			mediaIDs = append(mediaIDs, rows[i].MediaID)
		}
	}
	mediaByID := map[string]model.Media{}
	if len(mediaIDs) > 0 {
		var mediaRows []model.Media
		if err := p.repo.DB.WithContext(ctx).Where("id IN ?", mediaIDs).Find(&mediaRows).Error; err == nil {
			for _, media := range mediaRows {
				mediaByID[media.ID] = media
			}
		}
	}
	items := make([]HistoryItem, 0, len(rows))
	for i := range rows {
		if m, ok := mediaByID[rows[i].MediaID]; ok {
			media := m
			items = append(items, HistoryItem{PlaybackHistory: rows[i], Media: &media})
			continue
		}
		if p.remote != nil && IsEmbyRemoteID(rows[i].MediaID) {
			mountID, remoteID, _ := DecodeEmbyRemoteID(rows[i].MediaID)
			if mount, acct, _ := p.remote.ResolveMount(ctx, mountID); mount != nil && acct != nil {
				if rm, err := p.remote.RemoteMediaDetail(ctx, mount, acct, remoteID); err == nil && rm != nil {
					media := *rm
					items = append(items, HistoryItem{PlaybackHistory: rows[i], Media: &media})
					continue
				}
			}
		}
		items = append(items, HistoryItem{PlaybackHistory: rows[i]})
	}
	return items, nil
}

// ─── Favourites ─────────────────────────────────────────────────────────────

// ToggleFavourite flips the favourite flag and reports the new state.
func (p *PlaybackService) ToggleFavourite(ctx context.Context, userID, mediaID string) (bool, error) {
	return p.repo.Favorite.Toggle(ctx, userID, mediaID)
}

// ListFavourites returns every favourited media for a user.
func (p *PlaybackService) ListFavourites(ctx context.Context, userID string) ([]model.Media, error) {
	favs, err := p.repo.Favorite.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(favs) == 0 {
		return nil, nil
	}
	sort.Slice(favs, func(i, j int) bool {
		return favs[i].CreatedAt.After(favs[j].CreatedAt)
	})

	localIDs := make([]string, 0, len(favs))
	for _, fav := range favs {
		if !IsEmbyRemoteID(fav.MediaID) {
			localIDs = append(localIDs, fav.MediaID)
		}
	}
	mediaByID := map[string]model.Media{}
	if len(localIDs) > 0 {
		var mediaRows []model.Media
		if err := p.repo.DB.WithContext(ctx).Where("id IN ?", localIDs).Find(&mediaRows).Error; err != nil {
			return nil, err
		}
		for _, media := range mediaRows {
			mediaByID[media.ID] = media
		}
	}

	out := make([]model.Media, 0, len(favs))
	for _, fav := range favs {
		if media, ok := mediaByID[fav.MediaID]; ok {
			out = append(out, media)
			continue
		}
		if p.remote != nil && IsEmbyRemoteID(fav.MediaID) {
			mountID, remoteID, _ := DecodeEmbyRemoteID(fav.MediaID)
			mount, acct, _ := p.remote.ResolveMount(ctx, mountID)
			if mount == nil || acct == nil {
				continue
			}
			remoteMedia, err := p.remote.RemoteMediaDetail(ctx, mount, acct, remoteID)
			if err != nil || remoteMedia == nil {
				continue
			}
			out = append(out, *remoteMedia)
		}
	}
	return out, nil
}

// ─── Playlists ──────────────────────────────────────────────────────────────

// CreatePlaylist persists a new playlist owned by userID.
func (p *PlaybackService) CreatePlaylist(ctx context.Context, userID, name string, isPublic bool) (*model.Playlist, error) {
	if name == "" {
		return nil, errors.New("name required")
	}
	pl := &model.Playlist{UserID: userID, Name: name, IsPublic: isPublic}
	if err := p.repo.Playlist.Create(ctx, pl); err != nil {
		return nil, err
	}
	return pl, nil
}

// ListPlaylists returns every playlist owned by userID.
func (p *PlaybackService) ListPlaylists(ctx context.Context, userID string) ([]model.Playlist, error) {
	return p.repo.Playlist.ListByUser(ctx, userID)
}

// PlaylistDetail returns the playlist together with its ordered media items.
type PlaylistDetail struct {
	Playlist model.Playlist `json:"playlist"`
	Items    []model.Media  `json:"items"`
}

// GetPlaylist returns the playlist + its ordered media. Visibility is
// enforced at the handler level; the service trusts callers.
func (p *PlaybackService) GetPlaylist(ctx context.Context, playlistID string) (*PlaylistDetail, error) {
	var pl model.Playlist
	if err := p.repo.DB.Where("id = ?", playlistID).First(&pl).Error; err != nil {
		return nil, err
	}
	var rows []model.PlaylistItem
	if err := p.repo.DB.
		Where("playlist_id = ?", playlistID).
		Order("position asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &PlaylistDetail{Playlist: pl}, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.MediaID
	}
	var media []model.Media
	if err := p.repo.DB.Where("id IN ?", ids).Find(&media).Error; err != nil {
		return nil, err
	}
	// Preserve playlist order.
	byID := make(map[string]model.Media, len(media))
	for _, m := range media {
		byID[m.ID] = m
	}
	ordered := make([]model.Media, 0, len(rows))
	for _, r := range rows {
		if m, ok := byID[r.MediaID]; ok {
			ordered = append(ordered, m)
		}
	}
	return &PlaylistDetail{Playlist: pl, Items: ordered}, nil
}

// AddToPlaylist appends a media item to the end of a playlist.
func (p *PlaybackService) AddToPlaylist(ctx context.Context, playlistID, mediaID string) error {
	var count int64
	if err := p.repo.DB.Model(&model.PlaylistItem{}).
		Where("playlist_id = ?", playlistID).Count(&count).Error; err != nil {
		return err
	}
	item := &model.PlaylistItem{
		PlaylistID: playlistID,
		MediaID:    mediaID,
		Position:   int(count) + 1,
	}
	return p.repo.DB.Create(item).Error
}

// RemoveFromPlaylist 物理删除播放列表项（幂等）。
func (p *PlaybackService) RemoveFromPlaylist(ctx context.Context, playlistID, mediaID string) error {
	return p.repo.DB.WithContext(ctx).Unscoped().
		Where("playlist_id = ? AND media_id = ?", playlistID, mediaID).
		Delete(&model.PlaylistItem{}).Error
}

// DeletePlaylist 物理删除播放列表及其全部条目。
func (p *PlaybackService) DeletePlaylist(ctx context.Context, playlistID string) error {
	if err := p.repo.DB.WithContext(ctx).Unscoped().Where("playlist_id = ?", playlistID).
		Delete(&model.PlaylistItem{}).Error; err != nil {
		return err
	}
	return p.repo.DB.WithContext(ctx).Unscoped().Where("id = ?", playlistID).Delete(&model.Playlist{}).Error
}
