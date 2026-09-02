package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/truewhile/MeBox/internal/database"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
	"github.com/truewhile/MeBox/internal/service"
)

func TestReorderEmbyMountsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	ctx := t.Context()

	m1 := &model.EmbyMount{AccountID: "acct-1", RemoteViewID: "view-1", Name: "Mount 1"}
	m2 := &model.EmbyMount{AccountID: "acct-1", RemoteViewID: "view-2", Name: "Mount 2"}
	_ = repos.EmbyMount.Create(ctx, m1)
	_ = repos.EmbyMount.Create(ctx, m2)

	svc := &service.Container{
		Repo:       repos,
		EmbyRemote: service.NewEmbyRemoteService(nil, zap.NewNop(), repos, nil),
	}

	router := gin.New()
	router.PUT("/admin/emby/mounts/reorder", reorderEmbyMountsHandler(svc))

	body, _ := json.Marshal(map[string]any{
		"ids": []string{m2.ID, m1.ID},
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/emby/mounts/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	list, err := repos.EmbyMount.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != m2.ID || list[1].ID != m1.ID {
		t.Fatalf("expected order [m2, m1], got [m%s, m%s]", list[0].ID, list[1].ID)
	}
}
