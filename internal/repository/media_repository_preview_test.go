package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/database"
	"github.com/ShukeBta/MMTL/internal/model"
)

func TestListRecentByLibraries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repos := New(db)

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
	for i := 1; i <= 5; i++ {
		rows = append(rows, model.Media{
			Base:      model.Base{ID: fmt.Sprintf("movie-%02d", i), CreatedAt: now.Add(time.Duration(i) * time.Hour)},
			LibraryID: lib1.ID,
			Title:     fmt.Sprintf("电影%d", i),
			Path:      fmt.Sprintf("/media/movies/电影%d/movie%d.mp4", i, i),
		})
	}
	for i := 1; i <= 8; i++ {
		rows = append(rows, model.Media{
			Base:       model.Base{ID: fmt.Sprintf("anime-ep-%02d", i), CreatedAt: now.Add(time.Duration(i) * time.Minute)},
			LibraryID:  lib2.ID,
			Title:      fmt.Sprintf("某动漫 第%d集", i),
			Path:       fmt.Sprintf("/media/anime/某动漫/Season 01/某动漫.S01E%02d.mp4", i),
			SeasonNum:  1,
			EpisodeNum: i,
		})
	}
	if err := repos.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	filter := MediaQueryFilter{IncludeNSFW: true}
	got, err := repos.Media.ListRecentByLibraries(t.Context(), []string{lib1.ID, lib2.ID}, 3, filter)
	if err != nil {
		t.Fatalf("ListRecentByLibraries failed: %v", err)
	}
	if len(got[lib1.ID]) != 3 {
		t.Fatalf("lib1 recent count = %d, want 3", len(got[lib1.ID]))
	}
	if len(got[lib2.ID]) != 3 {
		t.Fatalf("lib2 recent count = %d, want 3", len(got[lib2.ID]))
	}
	if got[lib1.ID][0].ID != "movie-05" {
		t.Fatalf("lib1 newest = %q, want movie-05", got[lib1.ID][0].ID)
	}
}
