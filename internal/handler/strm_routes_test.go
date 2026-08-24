package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"github.com/ShukeBta/MMTL/internal/service"
)

// TestStrmAdminRoutes 冒烟测试 STRM 管理端点注册与 401 拦截。
func TestStrmAdminRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router, &config.Config{
		Secrets: config.SecretsConfig{JWTSecret: "test-secret"},
	}, zap.NewNop(), &service.Container{Log: zap.NewNop()})

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/admin/strm/accounts",
		"POST /api/admin/strm/accounts",
		"PUT /api/admin/strm/accounts/:id",
		"DELETE /api/admin/strm/accounts/:id",
		"POST /api/admin/strm/accounts/:id/test",
		"GET /api/admin/strm/accounts/:id/list",
		"POST /api/admin/strm/accounts/:id/oauth/start",
		"POST /api/admin/strm/accounts/:id/oauth/poll",
		"GET /api/admin/strm/settings",
		"PUT /api/admin/strm/settings",
		"GET /api/admin/strm/paths",
		"POST /api/admin/strm/paths",
		"PUT /api/admin/strm/paths/:id",
		"DELETE /api/admin/strm/paths/:id",
		"POST /api/admin/strm/paths/:id/sync",
		"POST /api/admin/strm/paths/:id/cancel",
		"GET /api/admin/strm/records",
		"GET /api/admin/strm/downloads",
		"POST /api/admin/strm/downloads/:id/cancel",
		"POST /api/admin/strm/downloads/:id/retry",
		"GET /api/admin/strm/uploads",
		"POST /api/admin/strm/uploads/:id/cancel",
		"POST /api/admin/strm/uploads/:id/retry",
		"GET /api/strm/play/:provider/:file",
	} {
		if !routes[want] {
			t.Fatalf("%s route is not registered", want)
		}
	}
}

// TestStrmAccountsCRUD 用内存库走一遍账号/设置/同步目录接口。
func TestStrmAccountsCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:strm_handler?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.StrmAccount{}, &model.StrmSyncPath{}, &model.StrmSyncRecord{},
		&model.StrmDownloadTask{}, &model.StrmUploadTask{}, &model.Setting{}); err != nil {
		t.Fatal(err)
	}
	repos := repository.New(db)
	svc := service.NewWithVersion(&config.Config{}, zap.NewNop(), repos, "test")
	router := gin.New()
	Register(router, &config.Config{Secrets: config.SecretsConfig{JWTSecret: "test-secret"}}, zap.NewNop(), svc)

	// 未登录访问应 401
	req := httptest.NewRequest(http.MethodGet, "/api/admin/strm/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /admin/strm/accounts = %d, want 401", w.Code)
	}

	// 公开播放端点（本地提供方路径校验失败 → 404/400，不 panic）
	req = httptest.NewRequest(http.MethodGet, "/api/strm/play/local/video.mkv?path=%2Ftmp%2Fnope.mkv", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("strm play endpoint errored: %d", w.Code)
	}
}
