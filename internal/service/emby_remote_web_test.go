package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
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

func TestRemoteSearchMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("SearchTerm") == "碧蓝之海" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"TotalRecordCount": 1,
				"Items": []map[string]any{
					{
						"Id":             "156030",
						"Name":           "碧蓝之海",
						"Type":           "Series",
						"ProductionYear": 2018,
					},
				},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"TotalRecordCount": 0,
			"Items":            []map[string]any{},
		})
	}))
	defer server.Close()

	db := newServiceTestDB(t, &model.StrmAccount{}, &model.EmbyMount{})
	repos := repository.New(db)
	svc := NewEmbyRemoteService(&config.Config{}, zap.NewNop(), repos, NewCryptoService("", zap.NewNop()))

	rawConfig, _ := json.Marshal(map[string]string{
		"url":   server.URL,
		"token": "fake-token",
	})
	acct := &model.StrmAccount{
		Base:     model.Base{ID: "acct-1"},
		Name:     "test-emby",
		Provider: model.StrmProviderEmbyRemote,
		Config:   string(rawConfig),
		Enabled:  true,
	}
	if err := repos.StrmAccount.Create(t.Context(), acct); err != nil {
		t.Fatalf("create account: %v", err)
	}

	mount := &model.EmbyMount{
		Base:           model.Base{ID: "mount-1"},
		AccountID:      acct.ID,
		RemoteViewID:   "view-1",
		RemoteViewName: "动漫",
		CollectionType: "tvshows",
		Enabled:        true,
	}
	if err := repos.EmbyMount.Create(t.Context(), mount); err != nil {
		t.Fatalf("create mount: %v", err)
	}

	// 1. 正常搜索
	items, err := svc.RemoteSearchMedia(t.Context(), "碧蓝之海", 10, MediaVisibility{IncludeNSFW: true})
	if err != nil {
		t.Fatalf("RemoteSearchMedia failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "碧蓝之海" {
		t.Fatalf("expected Title '碧蓝之海', got %q", items[0].Title)
	}
	expectedID := EncodeEmbyRemoteID("mount-1", "156030")
	if items[0].ID != expectedID {
		t.Fatalf("expected ID %q, got %q", expectedID, items[0].ID)
	}

	// 2. 搜索不到的内容
	notFound, err := svc.RemoteSearchMedia(t.Context(), "其它不存在的剧", 10, MediaVisibility{IncludeNSFW: true})
	if err != nil {
		t.Fatalf("RemoteSearchMedia failed: %v", err)
	}
	if len(notFound) != 0 {
		t.Fatalf("expected 0 items, got %d", len(notFound))
	}

	// 3. 白名单过滤：当白名单不包含该挂载虚拟库 ID 时应过滤掉
	allowedLibID := "local-lib-1"
	filtered, err := svc.RemoteSearchMedia(t.Context(), "碧蓝之海", 10, MediaVisibility{
		IncludeNSFW:       true,
		AllowedLibraryIDs: []string{allowedLibID},
	})
	if err != nil {
		t.Fatalf("RemoteSearchMedia with allowed filter failed: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("expected 0 items due to AllowedLibraryIDs, got %d", len(filtered))
	}

	// 4. 黑名单过滤：当黑名单包含该挂载虚拟库 ID 时应过滤掉
	mountLibID := EncodeEmbyRemoteID("mount-1", "view-1")
	hiddenFiltered, err := svc.RemoteSearchMedia(t.Context(), "碧蓝之海", 10, MediaVisibility{
		IncludeNSFW:      true,
		HiddenLibraryIDs: []string{mountLibID},
	})
	if err != nil {
		t.Fatalf("RemoteSearchMedia with hidden filter failed: %v", err)
	}
	if len(hiddenFiltered) != 0 {
		t.Fatalf("expected 0 items due to HiddenLibraryIDs, got %d", len(hiddenFiltered))
	}
}
