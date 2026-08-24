// Package handler — admin-only routes.
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/middleware"
	"github.com/ShukeBta/MMTL/internal/service"
)

func registerAdminRoutes(api *gin.RouterGroup, cfg *config.Config, svc *service.Container) {
	admin := api.Group("/admin")
	admin.Use(middleware.AuthRequired(cfg.Secrets.JWTSecret), middleware.AdminRequired())
	registerAdminUserRoutes(admin, svc)
	registerAdminPermissionRoutes(admin, svc)
	registerAdminSystemRoutes(admin, svc)
	registerAdminBackupRoutes(admin, svc)
	registerAdminOrganizerRoutes(admin, svc)
	registerAdminAPIConfigRoutes(admin, svc)
	registerAdminRecognitionWordRoutes(admin, svc)
	registerAdminStrmRoutes(admin, svc)
}

func registerAdminStrmRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/strm/accounts", listStrmAccountsHandler(svc))
	admin.POST("/strm/accounts", createStrmAccountHandler(svc))
	admin.PUT("/strm/accounts/:id", updateStrmAccountHandler(svc))
	admin.DELETE("/strm/accounts/:id", deleteStrmAccountHandler(svc))
	admin.POST("/strm/accounts/:id/test", testStrmAccountHandler(svc))
	admin.GET("/strm/accounts/:id/list", listStrmRemoteDirHandler(svc))
	admin.GET("/strm/115/sources", listStrm115SourcesHandler(svc))
	admin.POST("/strm/accounts/:id/oauth/start", startStrm115OAuthHandler(svc))
	admin.POST("/strm/accounts/:id/oauth/poll", pollStrm115OAuthHandler(svc))

	admin.GET("/strm/settings", getStrmSettingsHandler(svc))
	admin.PUT("/strm/settings", updateStrmSettingsHandler(svc))

	admin.GET("/strm/paths", listStrmSyncPathsHandler(svc))
	admin.POST("/strm/paths", createStrmSyncPathHandler(svc))
	admin.PUT("/strm/paths/:id", updateStrmSyncPathHandler(svc))
	admin.DELETE("/strm/paths/:id", deleteStrmSyncPathHandler(svc))
	admin.POST("/strm/paths/:id/sync", startStrmSyncHandler(svc))
	admin.POST("/strm/paths/:id/cancel", cancelStrmSyncHandler(svc))
	admin.GET("/strm/records", listStrmSyncRecordsHandler(svc))
	admin.GET("/strm/local-dirs", listStrmLocalDirsHandler(svc))

	admin.GET("/strm/downloads", downloadQueueHandler(svc))
	admin.POST("/strm/downloads/:id/cancel", cancelStrmDownloadHandler(svc))
	admin.POST("/strm/downloads/:id/retry", retryStrmDownloadHandler(svc))
	admin.POST("/strm/downloads/clear-done", clearDoneDownloadsHandler(svc))
	admin.POST("/strm/downloads/clear-finished", clearFinishedDownloadsHandler(svc))
	admin.POST("/strm/downloads/retry-failed", retryAllFailedDownloadsHandler(svc))
	admin.POST("/strm/downloads/cancel-pending", cancelPendingDownloadsHandler(svc))
	admin.GET("/strm/uploads", uploadQueueHandler(svc))
	admin.POST("/strm/uploads/:id/cancel", cancelStrmUploadHandler(svc))
	admin.POST("/strm/uploads/:id/retry", retryStrmUploadHandler(svc))
}

func registerAdminUserRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/users", listUsersHandler(svc))
	admin.POST("/users", createUserHandler(svc))
	admin.PATCH("/users/:id", updateUserHandler(svc))
	admin.PATCH("/users/:id/password", resetUserPasswordHandler(svc))
	admin.PATCH("/users/:id/status", updateUserStatusHandler(svc))
	admin.PATCH("/users/:id/role", adminUpdateRoleHandler(svc))
	admin.DELETE("/users/:id", deleteUserHandler(svc))
	admin.GET("/settings", listSettingsHandler(svc))
	admin.PUT("/settings", updateSettingHandler(svc))
	admin.POST("/adult/test-scraper", testAdultScraperHandler(svc))
	admin.GET("/logs", recentLogsHandler(svc))
}

func registerAdminPermissionRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/users/:id/permissions", getUserPermissionsHandler(svc))
	admin.PUT("/users/:id/permissions", updateUserPermissionsHandler(svc))
	admin.POST("/users/:id/permissions/reset", resetUserPermissionsHandler(svc))
}

func registerAdminSystemRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/system/update", systemUpdateStatusHandler(svc))
	admin.POST("/system/update/check", systemUpdateCheckHandler(svc))
	admin.POST("/system/update/apply", systemUpdateApplyHandler(svc))
}

func registerAdminBackupRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/backups", listBackupsHandler(svc))
	admin.POST("/backups", createBackupHandler(svc))
	admin.DELETE("/backups", deleteBackupHandler(svc))
	admin.POST("/backups/restore", restoreBackupHandler(svc))
}

func registerAdminOrganizerRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.POST("/media/:id/organize", organizeMediaHandler(svc))
	admin.POST("/libraries/:id/organize", organizeLibraryHandler(svc))
	admin.GET("/organize/sources", organizeSourcesHandler(svc))
	admin.POST("/organize/source", organizeDirectoryHandler(svc))
}

func registerAdminAPIConfigRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/api-configs", listAPIConfigsHandler(svc))
	admin.GET("/api-configs/:provider", getAPIConfigHandler(svc))
	admin.PUT("/api-configs/:provider", updateAPIConfigHandler(svc))
	admin.DELETE("/api-configs/:provider", deleteAPIConfigHandler(svc))
	admin.POST("/api-configs/:provider/test", testAPIConfigHandler(svc))
}

func registerAdminRecognitionWordRoutes(admin *gin.RouterGroup, svc *service.Container) {
	admin.GET("/recognition-words", getRecognitionWordsHandler(svc))
	admin.PUT("/recognition-words", saveRecognitionWordsHandler(svc))
	admin.POST("/recognition-words/sync", syncRecognitionWordsHandler(svc))
	admin.POST("/recognition-words/test", testRecognitionWordsHandler(svc))
}
