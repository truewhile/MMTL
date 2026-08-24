package service

import (
	"testing"
	"time"

	"github.com/ShukeBta/MMTL/internal/model"
)

func TestEmbyLatestItemsCollapsesMovieVersions(t *testing.T) {
	svc := newTestEmbyService(t)
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	now := time.Now()
	for _, media := range []model.Media{
		{
			Base:      model.Base{ID: "dune-1080", CreatedAt: now.Add(time.Minute)},
			LibraryID: lib.ID,
			Title:     "Dune",
			Year:      2021,
			TMDbID:    438631,
			Path:      `/media/movies/Dune.2021.1080p.mkv`,
			Width:     1920,
			SizeBytes: 100,
		},
		{
			Base:      model.Base{ID: "dune-2160", CreatedAt: now.Add(2 * time.Minute)},
			LibraryID: lib.ID,
			Title:     "Dune",
			Year:      2021,
			TMDbID:    438631,
			Path:      `/media/movies/Dune.2021.2160p.mkv`,
			Width:     3840,
			SizeBytes: 200,
		},
		{
			Base:      model.Base{ID: "matrix", CreatedAt: now},
			LibraryID: lib.ID,
			Title:     "The Matrix",
			Year:      1999,
			TMDbID:    603,
			Path:      `/media/movies/The.Matrix.1999.mkv`,
		},
	} {
		if err := svc.repo.DB.Create(&media).Error; err != nil {
			t.Fatalf("create media: %v", err)
		}
	}

	latest, err := svc.LatestItems(t.Context(), "user-1", lib.ID, 10)
	if err != nil {
		t.Fatalf("latest items: %v", err)
	}
	if len(latest) != 2 {
		t.Fatalf("latest items = %#v, want Dune collapsed plus Matrix", latest)
	}
	if latest[0]["Id"] != "dune-2160" {
		t.Fatalf("latest first item = %#v, want best Dune version", latest[0])
	}
	sources := latest[0]["MediaSources"].([]map[string]any)
	if len(sources) != 2 {
		t.Fatalf("collapsed latest item should expose both versions, got %#v", sources)
	}
}
