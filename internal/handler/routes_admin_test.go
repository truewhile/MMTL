package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/service"
)

func TestAdminRouteSurfacesAreRegistered(t *testing.T) {
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
		"GET /api/admin/users",
		"GET /api/admin/users/:id/permissions",
		"POST /api/admin/backups",
		"GET /api/admin/organize/sources",
		"GET /api/admin/api-configs",
	} {
		if !routes[want] {
			t.Fatalf("%s route is not registered", want)
		}
	}
}
