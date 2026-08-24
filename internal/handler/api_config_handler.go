// Package handler — API 配置 HTTP Handler。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/service"
)

// ApiConfigHandler API 配置 HTTP 处理。
type ApiConfigHandler struct {
	svc *service.Container
	log *zap.Logger
}

// NewApiConfigHandler 创建 API 配置处理器。
func NewApiConfigHandler(svc *service.Container, log *zap.Logger) *ApiConfigHandler {
	return &ApiConfigHandler{svc: svc, log: log}
}

// ListApiConfigs 获取所有 API 配置。
// GET /api/api-config
func (h *ApiConfigHandler) ListApiConfigs(c *gin.Context) {
	configs, err := h.svc.ApiConfig.List(c.Request.Context())
	if err != nil {
		h.log.Error("list api configs failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "internal error", "data": nil})
		return
	}

	// 遮蔽 API Key
	for i := range configs {
		if configs[i].APIKey != "" {
			configs[i].APIKey = h.svc.ApiConfig.MaskAPIKey(configs[i].APIKey)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": configs})
}

// ListProviders 获取预定义的提供者列表。
// GET /api/api-config/providers/list
func (h *ApiConfigHandler) ListProviders(c *gin.Context) {
	providers := h.svc.ApiConfig.GetProviders()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": providers})
}

// GetApiConfig 获取指定提供者的配置。
// GET /api/api-config/:provider
func (h *ApiConfigHandler) GetApiConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "provider required", "data": nil})
		return
	}

	cfg, err := h.svc.ApiConfig.GetByProvider(c.Request.Context(), provider)
	if err != nil {
		if err == service.ErrApiConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "api config not found", "data": nil})
			return
		}
		h.log.Error("get api config failed", zap.Error(err), zap.String("provider", provider))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "internal error", "data": nil})
		return
	}

	// 遮蔽 API Key
	if cfg.APIKey != "" {
		cfg.APIKey = h.svc.ApiConfig.MaskAPIKey(cfg.APIKey)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": cfg})
}

// GetEffectiveConfig 获取生效的配置（数据库配置优先于配置文件）。
// GET /api/api-config/:provider/effective
func (h *ApiConfigHandler) GetEffectiveConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "provider required", "data": nil})
		return
	}

	cfg, err := h.svc.ApiConfig.GetEffectiveConfig(c.Request.Context(), provider)
	if err != nil {
		if err == service.ErrApiConfigNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 40401, "message": "api config not found", "data": nil})
			return
		}
		h.log.Error("get effective config failed", zap.Error(err), zap.String("provider", provider))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "internal error", "data": nil})
		return
	}

	// 遮蔽 API Key
	if cfg.APIKey != "" {
		cfg.APIKey = h.svc.ApiConfig.MaskAPIKey(cfg.APIKey)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": cfg})
}

// UpsertApiConfig 创建或更新 API 配置。
// POST /api/api-config/:provider
func (h *ApiConfigHandler) UpsertApiConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "provider required", "data": nil})
		return
	}

	var req struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Extra   string `json:"extra"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "invalid request", "data": nil})
		return
	}

	cfg, err := h.svc.ApiConfig.Upsert(c.Request.Context(), provider, req.APIKey, req.BaseURL, req.Extra, req.Enabled)
	if err != nil {
		if err == service.ErrInvalidProvider {
			c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "invalid provider", "data": nil})
			return
		}
		h.log.Error("upsert api config failed", zap.Error(err), zap.String("provider", provider))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "internal error", "data": nil})
		return
	}

	// 返回遮蔽后的配置
	cfg.APIKey = h.svc.ApiConfig.MaskAPIKey(cfg.APIKey)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": cfg})
}

// DeleteApiConfig 删除 API 配置。
// DELETE /api/api-config/:provider
func (h *ApiConfigHandler) DeleteApiConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "provider required", "data": nil})
		return
	}

	if err := h.svc.ApiConfig.Delete(c.Request.Context(), provider); err != nil {
		h.log.Error("delete api config failed", zap.Error(err), zap.String("provider", provider))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "message": "internal error", "data": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": nil})
}

// TestApiConfig 测试 API 连接。
// POST /api/api-config/:provider/test
func (h *ApiConfigHandler) TestApiConfig(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "message": "provider required", "data": nil})
		return
	}

	result, err := h.svc.ApiConfig.TestConnection(c.Request.Context(), provider)
	if err != nil {
		h.log.Debug("test api config failed", zap.Error(err), zap.String("provider", provider))
		// 不返回错误，只返回测试结果
	}

	// 更新测试结果
	_ = h.svc.ApiConfig.UpdateTestResult(c.Request.Context(), provider, result)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"message": "ok",
		"data": gin.H{
			"result": result,
		},
	})
}
