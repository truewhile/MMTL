package service

import (
	"testing"
	"time"

	"github.com/truewhile/MeBox/internal/model"
)

func TestEmbyRootItemsExposeLibraries(t *testing.T) {
	svc := newTestEmbyService(t)
	for _, lib := range []model.Library{
		{Name: "电影", Path: `F:\downloads\电影`, Type: "movie", Enabled: true},
		{Name: "综艺", Path: `F:\downloads\综艺`, Type: "variety", Enabled: true},
	} {
		if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
			t.Fatalf("create library: %v", err)
		}
	}

	root, err := svc.Items(t.Context(), ItemsParams{Limit: 50})
	if err != nil {
		t.Fatalf("root items: %v", err)
	}
	items := root["Items"].([]map[string]any)
	if len(items) != 2 {
		t.Fatalf("expected root items to expose libraries, got %#v", items)
	}
	if items[0]["Type"] != "CollectionFolder" || items[1]["Type"] != "CollectionFolder" {
		t.Fatalf("root should return collection folders: %#v", items)
	}
	if items[1]["CollectionType"] != "tvshows" {
		t.Fatalf("variety libraries should use tvshows collection type: %#v", items[1])
	}
}

func TestEmbyFolderItemQueryExposesLibrariesForHome(t *testing.T) {
	svc := newTestEmbyService(t)
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	if err := svc.repo.DB.Create(&model.Media{Base: model.Base{ID: "movie-1"}, LibraryID: lib.ID, Title: "不应出现在文件夹查询", Path: `/media/movies/a.mkv`}).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}

	out, err := svc.Items(t.Context(), ItemsParams{
		IncludeItemTypes: []string{"Folder", "CollectionFolder"},
		Limit:            50,
	})
	if err != nil {
		t.Fatalf("folder items: %v", err)
	}
	items := out["Items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one library folder, got %#v", items)
	}
	if items[0]["Type"] != "CollectionFolder" || items[0]["IsFolder"] != true {
		t.Fatalf("folder query should return collection folders, got %#v", items[0])
	}
}

func TestEmbyUnsupportedItemTypesDoNotLeakAllMedia(t *testing.T) {
	svc := newTestEmbyService(t)
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	if err := svc.repo.DB.Create(&model.Media{Base: model.Base{ID: "movie-1"}, LibraryID: lib.ID, Title: "普通电影", Path: `/media/movies/a.mkv`}).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}

	for _, includeType := range []string{"BoxSet", "Game", "Book", "Audio", "MusicAlbum", "Playlist", "TvChannel"} {
		out, err := svc.Items(t.Context(), ItemsParams{
			IncludeItemTypes: []string{includeType},
			Recursive:        true,
			Limit:            50,
		})
		if err != nil {
			t.Fatalf("%s items: %v", includeType, err)
		}
		if out["TotalRecordCount"] != int64(0) {
			t.Fatalf("%s should not return media rows, got %#v", includeType, out)
		}
		items := out["Items"].([]map[string]any)
		if len(items) != 0 {
			t.Fatalf("%s should return an empty list, got %#v", includeType, items)
		}
	}
}

func TestEmbyItemsFiltersFavorites(t *testing.T) {
	svc := newTestEmbyService(t)
	viewer := &model.User{Base: model.Base{ID: "user-1"}, Username: "viewer", Role: "user", Tier: "free", IsActive: true}
	if err := svc.repo.User.Create(t.Context(), viewer); err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	favorite := model.Media{Base: model.Base{ID: "fav-1"}, LibraryID: lib.ID, Title: "收藏电影", Path: `/media/movies/fav.mkv`}
	normal := model.Media{Base: model.Base{ID: "normal-1"}, LibraryID: lib.ID, Title: "普通电影", Path: `/media/movies/normal.mkv`}
	if err := svc.repo.DB.Create(&favorite).Error; err != nil {
		t.Fatalf("create favorite media: %v", err)
	}
	if err := svc.repo.DB.Create(&normal).Error; err != nil {
		t.Fatalf("create normal media: %v", err)
	}
	if err := svc.repo.DB.Create(&model.Favorite{UserID: viewer.ID, MediaID: favorite.ID}).Error; err != nil {
		t.Fatalf("create favorite: %v", err)
	}

	out, err := svc.Items(t.Context(), ItemsParams{
		UserID:    viewer.ID,
		Filters:   []string{"IsFavorite"},
		Recursive: true,
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("favorite items: %v", err)
	}
	if out["TotalRecordCount"] != int64(1) {
		t.Fatalf("expected one favorite, got %#v", out)
	}
	items := out["Items"].([]map[string]any)
	if len(items) != 1 || items[0]["Id"] != favorite.ID {
		t.Fatalf("favorite filter returned wrong items: %#v", items)
	}
	userData := items[0]["UserData"].(map[string]any)
	if userData["IsFavorite"] != true {
		t.Fatalf("favorite payload should carry IsFavorite=true: %#v", userData)
	}
}

func TestEmbyItemsFiltersResumableForHome(t *testing.T) {
	svc := newTestEmbyService(t)
	viewer := &model.User{Base: model.Base{ID: "user-1"}, Username: "viewer", Role: "user", Tier: "free", IsActive: true}
	if err := svc.repo.User.Create(t.Context(), viewer); err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	resumable := model.Media{Base: model.Base{ID: "resume-1"}, LibraryID: lib.ID, Title: "继续观看", Path: `/media/movies/resume.mkv`, DurationSec: 120}
	normal := model.Media{Base: model.Base{ID: "normal-1"}, LibraryID: lib.ID, Title: "普通电影", Path: `/media/movies/normal.mkv`, DurationSec: 120}
	if err := svc.repo.DB.Create(&resumable).Error; err != nil {
		t.Fatalf("create resumable media: %v", err)
	}
	if err := svc.repo.DB.Create(&normal).Error; err != nil {
		t.Fatalf("create normal media: %v", err)
	}
	if err := svc.repo.DB.Create(&model.PlaybackHistory{
		UserID:     viewer.ID,
		MediaID:    resumable.ID,
		PositionMs: 30_000,
		DurationMs: 120_000,
		WatchedAt:  time.Now(),
		Completed:  false,
	}).Error; err != nil {
		t.Fatalf("create playback history: %v", err)
	}

	out, err := svc.Items(t.Context(), ItemsParams{
		UserID:     viewer.ID,
		Filters:    []string{"IsResumable"},
		Recursive:  true,
		SortBy:     "DatePlayed",
		SortOrder:  "Descending",
		Limit:      50,
		StartIndex: 0,
	})
	if err != nil {
		t.Fatalf("resumable items: %v", err)
	}
	if out["TotalRecordCount"] != int64(1) {
		t.Fatalf("expected one resumable item, got %#v", out)
	}
	items := out["Items"].([]map[string]any)
	if len(items) != 1 || items[0]["Id"] != resumable.ID {
		t.Fatalf("resumable filter returned wrong items: %#v", items)
	}
}

func TestEmbyUserPolicyDisablesDownloadsForViewers(t *testing.T) {
	svc := newTestEmbyService(t)
	viewer := &model.User{Username: "viewer", Role: "user", Tier: "free", IsActive: true}
	admin := &model.User{Username: "admin", Role: "admin", Tier: "plus", IsActive: true}
	if err := svc.repo.User.Create(t.Context(), viewer); err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	if err := svc.repo.User.Create(t.Context(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	viewerPayload, err := svc.FindUser(t.Context(), viewer.ID)
	if err != nil {
		t.Fatalf("viewer payload: %v", err)
	}
	adminPayload, err := svc.FindUser(t.Context(), admin.ID)
	if err != nil {
		t.Fatalf("admin payload: %v", err)
	}
	viewerPolicy := viewerPayload["Policy"].(map[string]any)
	adminPolicy := adminPayload["Policy"].(map[string]any)
	if viewerPolicy["EnableMediaPlayback"] != true {
		t.Fatalf("viewer must keep playback enabled: %#v", viewerPolicy)
	}
	if viewerPolicy["EnableContentDownloading"] != false ||
		viewerPolicy["EnableSyncTranscoding"] != false ||
		viewerPolicy["EnableMediaConversion"] != false {
		t.Fatalf("viewer must not be allowed to download/sync media: %#v", viewerPolicy)
	}
	if adminPolicy["EnableContentDownloading"] != true {
		t.Fatalf("admin should keep downloading capability: %#v", adminPolicy)
	}
}

func TestEmbyHidesAdultLibrariesForUserLock(t *testing.T) {
	svc := newTestEmbyService(t)
	viewer := &model.User{Username: "viewer", Role: "user", Tier: "free", IsActive: true, HideAdult: true}
	if err := svc.repo.User.Create(t.Context(), viewer); err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	safe := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	adult := model.Library{Name: "9KG 成人", Path: `/media/9KG`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &safe); err != nil {
		t.Fatalf("create safe library: %v", err)
	}
	if err := svc.repo.Library.Create(t.Context(), &adult); err != nil {
		t.Fatalf("create adult library: %v", err)
	}
	if err := svc.repo.Setting.Set(t.Context(), AdultLibraryIDsSettingKey, `["`+adult.ID+`"]`); err != nil {
		t.Fatalf("set adult libraries: %v", err)
	}
	if err := svc.repo.DB.Create(&model.Media{LibraryID: safe.ID, Title: "安全电影", Path: `/media/movies/a.mkv`}).Error; err != nil {
		t.Fatalf("create safe media: %v", err)
	}
	if err := svc.repo.DB.Create(&model.Media{LibraryID: adult.ID, Title: "成人电影", Path: `/media/9KG/a.mkv`}).Error; err != nil {
		t.Fatalf("create adult media: %v", err)
	}

	root, err := svc.Items(t.Context(), ItemsParams{UserID: viewer.ID, Limit: 50})
	if err != nil {
		t.Fatalf("root items: %v", err)
	}
	items := root["Items"].([]map[string]any)
	if len(items) != 1 || items[0]["Name"] != "电影" {
		t.Fatalf("adult library should be hidden: %#v", items)
	}
	adultItems, err := svc.Items(t.Context(), ItemsParams{UserID: viewer.ID, ParentID: adult.ID, Limit: 50})
	if err != nil {
		t.Fatalf("adult items: %v", err)
	}
	if got := adultItems["TotalRecordCount"]; got != int64(0) {
		t.Fatalf("adult media should be hidden, total=%#v payload=%#v", got, adultItems)
	}
}

func TestEmbyPlaybackInfoRespectsDirectPlayOnly(t *testing.T) {
	svc := newTestEmbyService(t)
	lib := model.Library{Name: "电影", Path: `/media/movies`, Type: "movie", Enabled: true}
	if err := svc.repo.Library.Create(t.Context(), &lib); err != nil {
		t.Fatalf("create library: %v", err)
	}
	media := model.Media{Base: model.Base{ID: "m-1"}, LibraryID: lib.ID, Title: "Inception", Path: `/media/movies/inception.mkv`}
	if err := svc.repo.DB.Create(&media).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}

	pb, err := svc.PlaybackInfo(t.Context(), "m-1", "user-1")
	if err != nil {
		t.Fatalf("playback info: %v", err)
	}
	src := pb["MediaSources"].([]map[string]any)[0]
	if src["SupportsTranscoding"] != true {
		t.Fatalf("expected SupportsTranscoding=true by default, got %#v", src["SupportsTranscoding"])
	}
	if _, ok := src["TranscodingUrl"]; !ok {
		t.Fatalf("expected TranscodingUrl present by default: %#v", src)
	}
	if src["TranscodingUrl"] != "/Videos/m-1/master.m3u8" {
		t.Fatalf("expected HLS TranscodingUrl by default, got %#v", src["TranscodingUrl"])
	}

	if err := svc.repo.Setting.Set(t.Context(), PlaybackDirectOnlySettingKey, "true"); err != nil {
		t.Fatalf("enable direct-only: %v", err)
	}
	pb, err = svc.PlaybackInfo(t.Context(), "m-1", "user-1")
	if err != nil {
		t.Fatalf("playback info (direct-only): %v", err)
	}
	src = pb["MediaSources"].([]map[string]any)[0]
	if src["SupportsTranscoding"] != false {
		t.Fatalf("expected SupportsTranscoding=false in direct-only mode, got %#v", src["SupportsTranscoding"])
	}
	if _, ok := src["TranscodingUrl"]; ok {
		t.Fatalf("expected no TranscodingUrl in direct-only mode: %#v", src)
	}
	if src["SupportsDirectPlay"] != true || src["DirectStreamUrl"] != "/Videos/m-1/stream.mkv" {
		t.Fatalf("direct-only must still allow direct play: %#v", src)
	}
}
