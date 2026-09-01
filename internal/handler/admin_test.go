package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"github.com/ShukeBta/MMTL/internal/service"
)

func TestDeleteUserRefusesRecentRealtimeSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	admin := model.User{Base: model.Base{ID: "admin"}, Username: "admin", PasswordHash: "x", Role: "admin", IsActive: true}
	viewer := model.User{Base: model.Base{ID: "viewer"}, Username: "viewer", PasswordHash: "x", Role: "user", IsActive: true}
	if err := repos.DB.Create(&[]model.User{admin, viewer}).Error; err != nil {
		t.Fatal(err)
	}
	tracker := service.NewSessionTrackerService(zap.NewNop())
	tracker.RecordLogin(t.Context(), viewer.ID, viewer.Username, "dev-1", "Apple TV", "Yamby", "10.0.0.8")
	svc := &service.Container{Repo: repos, Sessions: tracker}
	router := gin.New()
	router.DELETE("/admin/users/:id", deleteUserHandler(svc))

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/viewer", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if found, _ := repos.User.FindByID(t.Context(), viewer.ID); found == nil {
		t.Fatal("recent realtime user should not be deleted")
	}
}

func TestUpdateUserLibraries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	user := model.User{Base: model.Base{ID: "u1"}, Username: "alice", PasswordHash: "x", Role: "user", IsActive: true}
	lib1 := model.Library{Base: model.Base{ID: "lib-1"}, Name: "电影", Type: "movie", Path: "/movie"}
	lib2 := model.Library{Base: model.Base{ID: "lib-2"}, Name: "剧集", Type: "tv", Path: "/tv"}
	lib3 := model.Library{Base: model.Base{ID: "lib-3"}, Name: "动漫", Type: "anime", Path: "/anime"}
	if err := repos.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := repos.DB.Create(&[]model.Library{lib1, lib2, lib3}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &service.Container{Repo: repos}
	router := gin.New()
	router.PATCH("/admin/users/:id/libraries", updateUserLibrariesHandler(svc))

	// 1. 设置限制为 lib-1 和 lib-2
	body := `{"allowed_library_ids":["lib-1","lib-2"]}`
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/u1/libraries", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	found, err := repos.User.FindByID(t.Context(), "u1")
	if err != nil || found == nil {
		t.Fatal("user not found")
	}
	allowed := found.DecodeAllowedLibraryIDs()
	if len(allowed) != 2 || allowed[0] != "lib-1" || allowed[1] != "lib-2" {
		t.Fatalf("expected [lib-1, lib-2], got %v", allowed)
	}

	// 验证可见性
	vis := service.UserDefaultMediaVisibility(t.Context(), repos, "u1")
	if len(vis.AllowedLibraryIDs) != 2 {
		t.Fatalf("expected 2 allowed libraries, got %v", vis.AllowedLibraryIDs)
	}
	if !service.LibraryVisibleForUser(t.Context(), repos, lib1, vis) {
		t.Fatal("lib1 should be visible")
	}
	if !service.LibraryVisibleForUser(t.Context(), repos, lib2, vis) {
		t.Fatal("lib2 should be visible")
	}
	if service.LibraryVisibleForUser(t.Context(), repos, lib3, vis) {
		t.Fatal("lib3 should not be visible")
	}

	// 2. 清空限制，恢复全部可见
	bodyEmpty := `{"allowed_library_ids":[]}`
	reqEmpty := httptest.NewRequest(http.MethodPatch, "/admin/users/u1/libraries", strings.NewReader(bodyEmpty))
	reqEmpty.Header.Set("Content-Type", "application/json")
	wEmpty := httptest.NewRecorder()
	router.ServeHTTP(wEmpty, reqEmpty)

	if wEmpty.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", wEmpty.Code, wEmpty.Body.String())
	}

	foundReset, _ := repos.User.FindByID(t.Context(), "u1")
	if len(foundReset.DecodeAllowedLibraryIDs()) != 0 {
		t.Fatalf("expected nil or empty, got %v", foundReset.DecodeAllowedLibraryIDs())
	}

	visReset := service.UserDefaultMediaVisibility(t.Context(), repos, "u1")
	if len(visReset.AllowedLibraryIDs) != 0 {
		t.Fatalf("expected no library restrictions, got %v", visReset.AllowedLibraryIDs)
	}
	if !service.LibraryVisibleForUser(t.Context(), repos, lib3, visReset) {
		t.Fatal("lib3 should now be visible")
	}
}
