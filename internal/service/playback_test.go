package service

import (
	"context"
	"testing"
	"time"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"go.uber.org/zap"
)

func TestListFavouritesIncludesLocalAndRemoteIDs(t *testing.T) {
	db := newServiceTestDB(t, &model.Favorite{}, &model.Media{})
	repos := repository.New(db)
	userID := "user-1"
	local := model.Media{
		Base:  model.Base{ID: "local-movie-1"},
		Title: "Local Movie",
	}
	if err := db.Create(&local).Error; err != nil {
		t.Fatalf("create local media: %v", err)
	}
	remoteID := EncodeEmbyRemoteID("mount-1", "remote-series-1")
	favs := []model.Favorite{
		{Base: model.Base{CreatedAt: time.Now().Add(-time.Minute)}, UserID: userID, MediaID: remoteID},
		{Base: model.Base{CreatedAt: time.Now()}, UserID: userID, MediaID: local.ID},
	}
	for i := range favs {
		if err := db.Create(&favs[i]).Error; err != nil {
			t.Fatalf("create favorite %d: %v", i, err)
		}
	}

	svc := NewPlaybackService(zap.NewNop(), repos)
	items, err := svc.ListFavourites(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListFavourites: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one hydrated local favourite without remote service, got %d: %#v", len(items), items)
	}
	if items[0].ID != local.ID {
		t.Fatalf("expected local favourite first by created_at desc, got %#v", items[0])
	}
}
