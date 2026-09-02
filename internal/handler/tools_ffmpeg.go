// Package handler — FFmpeg/FFprobe 工具安装端点。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/service"
)

// ffToolsStatusHandler 返回 ffmpeg/ffprobe 当前安装状态
// （GET /api/admin/tools/ffmpeg/status）。
func ffToolsStatusHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || svc.FFTools == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFmpeg 工具服务不可用"})
			return
		}
		c.JSON(http.StatusOK, svc.FFTools.Status(c.Request.Context()))
	}
}

// ffToolsInstallHandler 触发后台下载安装（POST /api/admin/tools/ffmpeg/install）。
// 自动匹配当前运行环境（OS+架构），安装到 data/tools/ffmpeg/ 并把路径写入设置。
func ffToolsInstallHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil || svc.FFTools == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFmpeg 工具服务不可用"})
			return
		}
		if err := svc.FFTools.StartInstall(c.Request.Context()); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, svc.FFTools.Status(c.Request.Context()))
	}
}
