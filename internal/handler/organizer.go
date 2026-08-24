// Package handler — media file organizer endpoints.
package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ShukeBta/MMTL/internal/service"
)

// organizeReq carries optional per-request overrides. 留空则沿用系统设置。
//
// source_path = 源目录（待整理），dest_path = 目的地目录（整理输出）。
// target_path 为 dest_path 的向后兼容别名。
type organizeReq struct {
	SourcePath    string `json:"source_path"`
	DestPath      string `json:"dest_path"`
	TargetPath    string `json:"target_path"` // deprecated alias for dest_path
	TransferMode  string `json:"transfer_mode"`
	MediaType     string `json:"media_type"`
	MediaCategory string `json:"media_category"`
	ScanAfter     *bool  `json:"scan_after"`
	ScrapeAfter   *bool  `json:"scrape_after"`
	LibraryID     string `json:"library_id"`
	DryRun        bool   `json:"dry_run"`
}

// bindOrganizeOptions parses the optional JSON body into OrganizeOptions.
// A missing/empty body is fine — it means "use the configured defaults".
func bindOrganizeOptions(c *gin.Context) service.OrganizeOptions {
	var req organizeReq
	_ = c.ShouldBindJSON(&req)
	return organizeOptionsFromReq(req)
}

func organizeOptionsFromReq(req organizeReq) service.OrganizeOptions {
	dest := strings.TrimSpace(req.DestPath)
	if dest == "" {
		dest = strings.TrimSpace(req.TargetPath)
	}
	opts := service.OrganizeOptions{
		SourcePath:    strings.TrimSpace(req.SourcePath),
		DestPath:      dest,
		MediaType:     strings.TrimSpace(req.MediaType),
		MediaCategory: strings.TrimSpace(req.MediaCategory),
		DryRun:        req.DryRun,
	}
	if m := strings.TrimSpace(req.TransferMode); m != "" {
		opts.TransferMode = service.TransferMode(m)
	}
	return opts
}

func organizeMediaHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req organizeReq
		_ = c.ShouldBindJSON(&req)
		runReq := organizePipelineRequestFromReq(req, service.OrganizeScopeMedia, "手动整理媒体")
		runReq.MediaID = c.Param("id")
		resp, err := organizePipeline(svc).Run(c.Request.Context(), runReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		payload := gin.H{"path": resp.Path}
		if resp.Result != nil {
			payload["scans"] = resp.Result.Scans
			payload["scrapes"] = resp.Result.Scrapes
		}
		c.JSON(http.StatusOK, payload)
	}
}

func organizeLibraryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req organizeReq
		_ = c.ShouldBindJSON(&req)
		runReq := organizePipelineRequestFromReq(req, service.OrganizeScopeLibrary, "手动整理媒体库")
		runReq.LibraryID = c.Param("id")
		resp, err := organizePipeline(svc).Run(c.Request.Context(), runReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp.Result)
	}
}

// organizeSourcesHandler lists selectable organize source directories (download
// dir + media dir) so the UI can offer them alongside registered libraries.
func organizeSourcesHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"sources": svc.Organizer.OrganizeSourceCandidates(c.Request.Context())})
	}
}

// organizeDirectoryHandler organizes an arbitrary source directory (e.g. the
// download directory) into the destination with dedup + 洗版.
func organizeDirectoryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req organizeReq
		_ = c.ShouldBindJSON(&req)
		runReq := organizePipelineRequestFromReq(req, service.OrganizeScopeDirectory, "手动整理入库")
		runReq.PreferredLibraryID = strings.TrimSpace(req.LibraryID)
		resp, err := organizePipeline(svc).Run(c.Request.Context(), runReq)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp.Result)
	}
}

func organizePipelineRequestFromReq(req organizeReq, scope service.OrganizeScope, taskName string) service.OrganizePipelineRequest {
	dest := strings.TrimSpace(req.DestPath)
	if dest == "" {
		dest = strings.TrimSpace(req.TargetPath)
	}
	return service.OrganizePipelineRequest{
		Scope:         scope,
		Trigger:       service.OrganizeTriggerManual,
		TaskName:      taskName,
		SourcePath:    strings.TrimSpace(req.SourcePath),
		DestPath:      dest,
		TransferMode:  strings.TrimSpace(req.TransferMode),
		MediaType:     strings.TrimSpace(req.MediaType),
		MediaCategory: strings.TrimSpace(req.MediaCategory),
		ScanAfter:     req.ScanAfter,
		ScrapeAfter:   req.ScrapeAfter,
		DryRun:        req.DryRun,
	}
}

func organizePipeline(svc *service.Container) *service.OrganizePipelineService {
	if svc.OrganizePipeline != nil {
		return svc.OrganizePipeline
	}
	return service.NewOrganizePipelineService(svc.Log, svc.Repo, svc.Organizer, svc.Scan, svc.Tasks)
}
