package service

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
)

func TestListLibrariesWithPreview(t *testing.T) {
	db := newServiceTestDB(t, &model.Library{}, &model.Media{})
	repos := repository.New(db)

	lib1 := model.Library{Name: "电影", Path: "/media/movies", Type: "movie", Enabled: true}
	if err := repos.Library.Create(t.Context(), &lib1); err != nil {
		t.Fatal(err)
	}
	lib2 := model.Library{Name: "动漫", Path: "/media/anime", Type: "anime", Enabled: true}
	if err := repos.Library.Create(t.Context(), &lib2); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	var rows []model.Media

	// Add 5 movies to lib1
	for i := 1; i <= 5; i++ {
		rows = append(rows, model.Media{
			Base:      model.Base{ID: fmt.Sprintf("movie-%02d", i), CreatedAt: now.Add(time.Duration(i) * time.Hour)},
			LibraryID: lib1.ID,
			Title:     fmt.Sprintf("电影%d", i),
			Path:      fmt.Sprintf("/media/movies/电影%d/movie%d.mp4", i, i),
			PosterURL: fmt.Sprintf("/api/media/movie-%02d/poster", i),
		})
	}

	// Add 12 episodes of 1 anime to lib2
	for i := 1; i <= 12; i++ {
		rows = append(rows, model.Media{
			Base:       model.Base{ID: fmt.Sprintf("anime-ep-%02d", i), CreatedAt: now.Add(time.Duration(i) * time.Minute)},
			LibraryID:  lib2.ID,
			Title:      fmt.Sprintf("某动漫 第%d集", i),
			Path:       fmt.Sprintf("/media/anime/某动漫/Season 01/某动漫.S01E%02d.mp4", i),
			SeasonNum:  1,
			EpisodeNum: i,
			PosterURL:  "/api/media/anime-01/poster",
		})
	}

	if err := repos.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewMediaService(&config.Config{}, zap.NewNop(), repos)

	previews, err := svc.ListLibrariesWithPreview(t.Context(), []model.Library{lib1, lib2}, MediaVisibility{IncludeNSFW: true}, 10)
	if err != nil {
		t.Fatalf("ListLibrariesWithPreview failed: %v", err)
	}

	if len(previews) != 2 {
		t.Fatalf("got %d previews, want 2", len(previews))
	}

	// Verify lib1 (movies)
	if previews[0].ID != lib1.ID {
		t.Errorf("preview[0].ID = %q, want %q", previews[0].ID, lib1.ID)
	}
	if previews[0].Total != 5 {
		t.Errorf("preview[0].Total = %d, want 5", previews[0].Total)
	}
	if len(previews[0].Cards) != 5 {
		t.Errorf("preview[0].Cards count = %d, want 5", len(previews[0].Cards))
	}

	// Verify lib2 (anime)
	if previews[1].ID != lib2.ID {
		t.Errorf("preview[1].ID = %q, want %q", previews[1].ID, lib2.ID)
	}
	if previews[1].Total != 12 {
		t.Errorf("preview[1].Total = %d, want 12", previews[1].Total)
	}
	// 12 episodes should be grouped into 1 SeriesCard with Count = 12
	if len(previews[1].Cards) != 1 {
		t.Errorf("preview[1].Cards count = %d, want 1", len(previews[1].Cards))
	} else if previews[1].Cards[0].Count != 12 {
		t.Errorf("preview[1].Cards[0].Count = %d, want 12", previews[1].Cards[0].Count)
	}
}
