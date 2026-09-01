package service

import (
	"context"
	"testing"
	"time"

	"github.com/ShukeBta/MMTL/internal/model"
)

func TestMapRemoteItemToMediaSortingFields(t *testing.T) {
	svc := &EmbyRemoteService{}
	mount := &model.EmbyMount{Base: model.Base{ID: "mount-1"}}
	acct := &model.StrmAccount{Base: model.Base{ID: "acct-1"}}
	cfg := &EmbyRemoteConfig{BaseURL: "http://localhost:8096"}

	item := map[string]any{
		"Id":              "item-1",
		"Name":            "测试电影",
		"OriginalTitle":   "Test Movie",
		"ProductionYear":  2023,
		"CommunityRating": 8.5,
		"PremiereDate":    "2023-05-12T00:00:00.0000000Z",
		"DateCreated":     "2024-01-15T08:30:00.0000000Z",
	}

	media := svc.MapRemoteItemToMedia(context.Background(), mount, acct, cfg, item)

	if media.ReleaseDate != "2023-05-12" {
		t.Fatalf("ReleaseDate = %q, want %q", media.ReleaseDate, "2023-05-12")
	}
	if media.Year != 2023 {
		t.Fatalf("Year = %d, want 2023", media.Year)
	}
	if media.Rating != 8.5 {
		t.Fatalf("Rating = %f, want 8.5", media.Rating)
	}
	expectedCreated, _ := time.Parse(time.RFC3339, "2024-01-15T08:30:00Z")
	if !media.CreatedAt.Equal(expectedCreated) {
		t.Fatalf("CreatedAt = %v, want %v", media.CreatedAt, expectedCreated)
	}
	if !media.UpdatedAt.Equal(expectedCreated) {
		t.Fatalf("UpdatedAt = %v, want %v", media.UpdatedAt, expectedCreated)
	}
}

func TestMapRemoteItemToMediaCriticRatingFallback(t *testing.T) {
	svc := &EmbyRemoteService{}
	mount := &model.EmbyMount{Base: model.Base{ID: "mount-1"}}
	acct := &model.StrmAccount{Base: model.Base{ID: "acct-1"}}
	cfg := &EmbyRemoteConfig{BaseURL: "http://localhost:8096"}

	item := map[string]any{
		"Id":           "item-2",
		"Name":         "评分测试",
		"CriticRating": 9.2,
		"PremiereDate": "2022-10-01",
	}

	media := svc.MapRemoteItemToMedia(context.Background(), mount, acct, cfg, item)
	if media.Rating != 9.2 {
		t.Fatalf("Rating = %f, want 9.2 from CriticRating", media.Rating)
	}
	if media.Year != 2022 {
		t.Fatalf("Year = %d, want 2022 from PremiereDate", media.Year)
	}
}
