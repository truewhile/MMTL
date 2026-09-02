package service

import (
	"path/filepath"
	"testing"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
	"go.uber.org/zap"
)

func TestExistingLocalMediaSnapshotFiltersAndCleansLocalRows(t *testing.T) {
	db := newServiceTestDB(t, &model.Media{})
	repos := repository.New(db)
	scanner := NewScannerService(&config.Config{}, zap.NewNop(), repos, NewHub(zap.NewNop()), nil, nil)

	rawPath := filepath.Join("D:", "media", "Movies", "..", "Movie.mkv")
	cleanPath := filepath.Clean(rawPath)
	if err := db.Create(&[]model.Media{
		{
			LibraryID:    "lib-1",
			Path:         rawPath,
			SizeBytes:    4096,
			DurationSec:  240,
			Width:        3840,
			Height:       2160,
			VideoCodec:   "hevc",
			AudioCodec:   "truehd",
			Container:    "mkv",
			STRMURL:      "https://cdn.example.com/movie.mkv",
			FileID:       "dev:inode",
			ScrapeStatus: "matched",
		},
		{LibraryID: "lib-1", Path: "cloud://openlist/Movie.mkv", SizeBytes: 99},
		{LibraryID: "lib-2", Path: filepath.Join("D:", "media", "Other.mkv"), SizeBytes: 88},
	}).Error; err != nil {
		t.Fatal(err)
	}

	got, err := scanner.existingLocalMediaSnapshot(t.Context(), "lib-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("snapshot len = %d, want 1: %#v", len(got), got)
	}
	row, ok := got[cleanPath]
	if !ok {
		t.Fatalf("snapshot key %q not found in %#v", cleanPath, got)
	}
	if row.SizeBytes != 4096 || row.DurationSec != 240 || row.Width != 3840 || row.Height != 2160 {
		t.Fatalf("track fields not preserved: %#v", row)
	}
	if row.VideoCodec != "hevc" || row.AudioCodec != "truehd" || row.Container != "mkv" {
		t.Fatalf("codec fields not preserved: %#v", row)
	}
	if row.STRMURL == "" || row.FileID != "dev:inode" {
		t.Fatalf("identity fields not preserved: %#v", row)
	}
}
