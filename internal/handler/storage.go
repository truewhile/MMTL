package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/service"
)

func storageBreakdownHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		breakdown, err := svc.Storage.Compute(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, breakdown)
	}
}
