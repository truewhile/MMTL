package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/middleware"
	"github.com/truewhile/MeBox/internal/service"
)

type pinnedLibrariesReq struct {
	LibraryIDs []string `json:"library_ids"`
}

func getPinnedLibrariesHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get(middleware.CtxUserID)
		ids, err := svc.Profile.GetPinnedLibraryIDs(c.Request.Context(), uid.(string))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if ids == nil {
			ids = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"library_ids": ids})
	}
}

func setPinnedLibrariesHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pinnedLibrariesReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		uid, _ := c.Get(middleware.CtxUserID)
		ids, err := svc.Profile.SetPinnedLibraryIDs(c.Request.Context(), uid.(string), req.LibraryIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if ids == nil {
			ids = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"library_ids": ids})
	}
}
