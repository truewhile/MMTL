// Package handler — danmaku endpoints.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/service"
)

// getDanmakuHandler returns danmaku for a media. ?kw= optionally overrides the
// search keyword (used by the player's manual search box); ?episodeId= forces a
// specific danmaku library chosen by the user after a disambiguation.
func getDanmakuHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		res, err := svc.Danmaku.Fetch(c.Request.Context(), c.Param("id"), c.Query("kw"), c.Query("episodeId"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// getDanmakuConfigHandler exposes the danmaku renderer knobs (opacity, font
// size, area, enabled) so the player can initialize its control panel without
// admin privileges.
func getDanmakuConfigHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, svc.Danmaku.Config(c.Request.Context()))
	}
}
