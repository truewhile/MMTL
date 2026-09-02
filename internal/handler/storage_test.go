package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/config"
	"github.com/truewhile/MeBox/internal/service"
)

func TestStorageRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	Register(router, &config.Config{
		Secrets: config.SecretsConfig{JWTSecret: "test-secret"},
	}, zap.NewNop(), &service.Container{Log: zap.NewNop()})

	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/api/storage" {
			return
		}
	}
	t.Fatal("GET /api/storage route is not registered")
}
