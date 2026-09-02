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

	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
	"github.com/truewhile/MeBox/internal/service"
)

func TestUserLimitHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	cfg := &service.Container{Repo: repos, Auth: service.NewAuthService(nil, zap.NewNop(), repos, service.NewTokenService(nil, zap.NewNop(), repos), service.NewPermissionService(zap.NewNop(), repos))}
	router := gin.New()
	router.GET("/admin/users/limit", getUserLimitHandler(cfg))
	router.PUT("/admin/users/limit", updateUserLimitHandler(cfg))

	req := httptest.NewRequest(http.MethodGet, "/admin/users/limit", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET limit status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"max_users":20`) {
		t.Fatalf("expected default max_users=20, got %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/users/limit", strings.NewReader(`{"max_users":42}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT limit status = %d body=%s", w.Code, w.Body.String())
	}
	got, err := service.LoadMaxUsers(t.Context(), repos)
	if err != nil || got != 42 {
		t.Fatalf("stored max users = %d err=%v, want 42", got, err)
	}
}
