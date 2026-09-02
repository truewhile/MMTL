// Package handler — per-user feature toggle endpoints.
//
//	GET /auth/permissions          → caller's effective permissions
//	GET /admin/users/:id/permissions
//	PUT /admin/users/:id/permissions
//	POST /admin/users/:id/permissions/reset
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/middleware"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/service"
)

func requirePermission(svc *service.Container, key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(middleware.CtxUserRole)
		if role == "admin" {
			c.Next()
			return
		}
		uid, _ := c.Get(middleware.CtxUserID)
		userID, _ := uid.(string)
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		row, err := svc.Permissions.Effective(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if row == nil || !row.PermissionMap()[key] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			return
		}
		c.Next()
	}
}

func myPermissionsHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get(middleware.CtxUserID)
		row, err := svc.Permissions.Effective(c.Request.Context(), toString(uid))
		if err != nil {
			if service.IsTransientDatabaseLock(err) {
				role, _ := c.Get(middleware.CtxUserRole)
				c.JSON(http.StatusOK, service.FallbackPermissions(toString(uid), toString(role)))
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if row == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func getUserPermissionsHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		row, err := svc.Permissions.Effective(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if row == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}

func updateUserPermissionsHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p model.UserPermission
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.Permissions.Save(c.Request.Context(), c.Param("id"), &p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
	}
}

func resetUserPermissionsHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		row, err := svc.Permissions.Reset(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}
