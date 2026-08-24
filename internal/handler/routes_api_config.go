// Package handler — API configuration routes.
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/middleware"
	"github.com/ShukeBta/MMTL/internal/service"
)

func registerAPIConfigRoutes(api *gin.RouterGroup, cfg *config.Config, svc *service.Container) {
	// API Config management (admin only).
	apiConfig := api.Group("/api-config")
	apiConfig.Use(middleware.AuthRequired(cfg.Secrets.JWTSecret), middleware.AdminRequired())
	{
		apiConfig.GET("", listApiConfigsHandler(svc))
		apiConfig.GET("/providers/list", listProvidersHandler(svc))
		apiConfig.GET("/:provider", getApiConfigHandler(svc))
		apiConfig.GET("/:provider/effective", getEffectiveConfigHandler(svc))
		apiConfig.POST("/:provider", upsertApiConfigHandler(svc))
		apiConfig.DELETE("/:provider", deleteApiConfigHandler(svc))
		apiConfig.POST("/:provider/test", testApiConfigHandler(svc))
	}

}
