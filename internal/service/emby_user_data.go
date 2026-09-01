package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/model"
)

// SetFavorite 把 mediaID 标为 userID 的收藏。远程 Emby 条目直接透传到对应
// 服务器（本地不落库）。
func (e *EmbyService) SetFavorite(ctx context.Context, userID, mediaID string, favorite bool) error {
	if e.remote != nil && IsEmbyRemoteID(mediaID) {
		acctID, remoteID, _ := DecodeEmbyRemoteID(mediaID)
		if err := e.ProxyRemoteSetFavorite(ctx, acctID, remoteID, favorite); err != nil {
			return err
		}
		return nil
	}
	if favorite {
		var f model.Favorite
		err := e.repo.DB.WithContext(ctx).
			Where("user_id = ? AND media_id = ?", userID, mediaID).First(&f).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.repo.DB.WithContext(ctx).Create(&model.Favorite{
				UserID: userID, MediaID: mediaID,
			}).Error
		}
		return err
	}
	return e.repo.DB.WithContext(ctx).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		Delete(&model.Favorite{}).Error
}

// MarkPlayed 把 mediaID 标为已看（写一个 100% 进度的 history 行）。
// 远程 Emby 条目直接透传到对应服务器（本地不落库）。
func (e *EmbyService) MarkPlayed(ctx context.Context, userID, mediaID string, played bool) error {
	if e.remote != nil && IsEmbyRemoteID(mediaID) {
		acctID, remoteID, _ := DecodeEmbyRemoteID(mediaID)
		if err := e.ProxyRemoteSetPlayed(ctx, acctID, remoteID, played); err != nil {
			return err
		}
		return nil
	}
	if !played {
		err := e.repo.DB.WithContext(ctx).
			Where("user_id = ? AND media_id = ?", userID, mediaID).
			Delete(&model.PlaybackHistory{}).Error
		if err == nil {
			e.invalidateEmbyItemsCache(ctx)
		}
		return err
	}
	m, err := e.repo.Media.FindByID(ctx, mediaID)
	if err != nil || m == nil {
		return errors.New("media not found")
	}
	dur := int64(m.DurationSec) * 1000
	if dur <= 0 {
		dur = 1
	}
	err = e.repo.History.Upsert(ctx, &model.PlaybackHistory{
		UserID:     userID,
		MediaID:    mediaID,
		PositionMs: dur,
		DurationMs: dur,
		WatchedAt:  time.Now(),
		Completed:  true,
	})
	if err == nil {
		e.invalidateEmbyItemsCache(ctx)
	}
	return err
}

// RecordProgress 记录播放进度（来自 Emby 客户端的 /Sessions/Playing/Progress）。
func (e *EmbyService) RecordProgress(ctx context.Context, userID, mediaID string, positionTicks, runtimeTicks int64) error {
	pos := positionTicks / 10_000
	dur := runtimeTicks / 10_000
	if dur <= 0 {
		// runtimeTicks 缺失时回退到 media.DurationSec
		if m, _ := e.repo.Media.FindByID(ctx, mediaID); m != nil {
			dur = int64(m.DurationSec) * 1000
		} else if IsEmbyRemoteID(mediaID) {
			// 远程挂载条目：尝试从既有历史记录或远程详情补齐时长
			var oldHist model.PlaybackHistory
			if err := e.repo.DB.WithContext(ctx).Where("user_id = ? AND media_id = ?", userID, mediaID).First(&oldHist).Error; err == nil && oldHist.DurationMs > 0 {
				dur = oldHist.DurationMs
			} else if e.remote != nil {
				mountID, remoteID, _ := DecodeEmbyRemoteID(mediaID)
				if mount, acct, _ := e.remote.ResolveMount(ctx, mountID); mount != nil && acct != nil {
					if item, _ := e.remote.RemoteItem(ctx, mount, acct, remoteID); item != nil {
						if ticks, ok := item["RunTimeTicks"].(float64); ok && ticks > 0 {
							dur = int64(ticks) / 10_000
						} else if ticks, ok := item["RunTimeTicks"].(int64); ok && ticks > 0 {
							dur = ticks / 10_000
						}
					}
				}
			}
		}
	}
	completed := dur > 0 && pos >= dur*9/10
	err := e.repo.History.Upsert(ctx, &model.PlaybackHistory{
		UserID:     userID,
		MediaID:    mediaID,
		PositionMs: pos,
		DurationMs: dur,
		WatchedAt:  time.Now(),
		Completed:  completed,
	})
	if err == nil {
		e.invalidateEmbyItemsCache(ctx)
	}
	return err
}

// mergeRemoteUserData applies the current MMTL user's locally recorded playback
// state to remote Emby payloads. Remote metadata remains authoritative unless the
// user has played the item through MMTL.
func (e *EmbyService) mergeRemoteUserData(ctx context.Context, userID string, payload any) error {
	if strings.TrimSpace(userID) == "" || payload == nil {
		return nil
	}
	items := remoteItemMaps(payload)
	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		id, _ := item["Id"].(string)
		if !IsEmbyRemoteID(id) {
			continue
		}
		if _, ok := seen[id]; !ok {
			ids = append(ids, id)
			seen[id] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var histories []model.PlaybackHistory
	if err := e.repo.DB.WithContext(ctx).Where("user_id = ? AND media_id IN ?", userID, ids).Find(&histories).Error; err != nil {
		return err
	}
	byMediaID := make(map[string]*model.PlaybackHistory, len(histories))
	for i := range histories {
		byMediaID[histories[i].MediaID] = &histories[i]
	}
	for _, item := range items {
		id, _ := item["Id"].(string)
		if h := byMediaID[id]; h != nil {
			item["UserData"] = mergedRemoteUserData(item["UserData"], h)
		}
	}
	return nil
}

func remoteItemMaps(payload any) []map[string]any {
	items := make([]map[string]any, 0)
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if _, ok := typed["Id"].(string); ok {
				items = append(items, typed)
			}
			if nested, ok := typed["Items"]; ok {
				visit(nested)
			}
		case []any:
			for _, value := range typed {
				visit(value)
			}
		case []map[string]any:
			for _, value := range typed {
				visit(value)
			}
		}
	}
	visit(payload)
	return items
}

func mergedRemoteUserData(raw any, history *model.PlaybackHistory) map[string]any {
	userData := map[string]any{}
	if existing, ok := raw.(map[string]any); ok {
		for key, value := range existing {
			userData[key] = value
		}
	}
	duration := history.DurationMs
	position := history.PositionMs
	percentage := float64(0)
	if duration > 0 {
		percentage = float64(position) / float64(duration) * 100
	}
	userData["PlaybackPositionTicks"] = position * 10_000
	userData["Played"] = history.Completed
	userData["PlayedPercentage"] = percentage
	if history.Completed {
		playCount := 0
		switch value := userData["PlayCount"].(type) {
		case int:
			playCount = value
		case int64:
			playCount = int(value)
		case float64:
			playCount = int(value)
		}
		if playCount < 1 {
			userData["PlayCount"] = 1
		}
	}
	return userData
}

func (e *EmbyService) invalidateEmbyItemsCache(ctx context.Context) {
	if e.cache != nil {
		e.cache.DeletePrefix(ctx, "media:emby:")
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func intToStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}
