// Package handler — DLNA / UPnP discovery + cast endpoints.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/service"
)

func dlnaListHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		force := c.Query("force") == "true"
		devices, err := svc.DLNA.Discover(c.Request.Context(), force)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"devices": devices})
	}
}

type dlnaCastReq struct {
	ControlURL string `json:"control_url" binding:"required"`
	MediaURL   string `json:"media_url" binding:"required"`
}

func dlnaCastHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dlnaCastReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.DLNA.Cast(c.Request.Context(), req.ControlURL, req.MediaURL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
