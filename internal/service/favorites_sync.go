package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
)

// SyncUserFavorite keeps favourite state aligned across the local favourites table
// and the upstream remote Emby server for mounted items.
func SyncUserFavorite(ctx context.Context, repo *repository.Container, remote *EmbyRemoteService, userID, mediaID string, favorite bool) error {
	if repo == nil || userID == "" || mediaID == "" {
		return errors.New("missing favourite sync inputs")
	}
	if err := setLocalFavorite(ctx, repo, userID, mediaID, favorite); err != nil {
		return err
	}
	if favorite || IsEmbyRemoteID(mediaID) {
		if err := proxyRemoteFavorite(ctx, remote, mediaID, favorite); err != nil {
			return err
		}
	}
	return nil
}

// IsUserFavorite reports whether the user has favourited mediaID locally.
func IsUserFavorite(ctx context.Context, repo *repository.Container, userID, mediaID string) (bool, error) {
	if repo == nil || userID == "" || mediaID == "" {
		return false, nil
	}
	var count int64
	err := repo.DB.WithContext(ctx).Model(&model.Favorite{}).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		Count(&count).Error
	return count > 0, err
}

func setLocalFavorite(ctx context.Context, repo *repository.Container, userID, mediaID string, favorite bool) error {
	if favorite {
		var existing model.Favorite
		err := repo.DB.WithContext(ctx).
			Where("user_id = ? AND media_id = ?", userID, mediaID).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repo.DB.WithContext(ctx).Create(&model.Favorite{
				UserID:  userID,
				MediaID: mediaID,
			}).Error
		}
		return err
	}
	return repo.DB.WithContext(ctx).
		Where("user_id = ? AND media_id = ?", userID, mediaID).
		Delete(&model.Favorite{}).Error
}

func proxyRemoteFavorite(ctx context.Context, remote *EmbyRemoteService, mediaID string, favorite bool) error {
	if remote == nil || !IsEmbyRemoteID(mediaID) {
		return nil
	}
	mountID, remoteItemID, ok := DecodeEmbyRemoteID(mediaID)
	if !ok {
		return nil
	}
	_, acct, err := remote.ResolveMount(ctx, mountID)
	if err != nil {
		return err
	}
	return remote.ProxySetFavorite(ctx, acct, remoteItemID, favorite)
}
