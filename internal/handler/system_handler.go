// Package handler — system tools detection.
package handler

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/config"
	"github.com/ShukeBta/MMTL/internal/service"
)

// SystemHandler handles system-related endpoints.
type SystemHandler struct {
	cfg *config.Config
	log *zap.Logger
	svc *service.Container
}

// NewSystemHandler is the constructor.
func NewSystemHandler(cfg *config.Config, log *zap.Logger, svc *service.Container) *SystemHandler {
	return &SystemHandler{cfg: cfg, log: log, svc: svc}
}

// ToolStatus represents the detection status of a system tool.
type ToolStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	ConfigKey   string `json:"config_key"`
	Path        string `json:"path,omitempty"`
	Detected    bool   `json:"detected"`
	Version     string `json:"version,omitempty"`
}

// GetToolsStatus returns the status of system tools.
func (h *SystemHandler) GetToolsStatus(c *gin.Context) {
	tools := []ToolStatus{
		{Name: "ffprobe", DisplayName: "FFprobe", ConfigKey: "app.ffprobe_path"},
		{Name: "ffmpeg", DisplayName: "FFmpeg", ConfigKey: "app.ffmpeg_path"},
	}

	for i := range tools {
		// Check configured path first
		var configuredPath string
		switch tools[i].ConfigKey {
		case "app.ffprobe_path":
			configuredPath = h.cfg.App.FFprobePath
			if configuredPath == "" {
				configuredPath = "ffprobe"
			}
		case "app.ffmpeg_path":
			configuredPath = h.cfg.App.FFmpegPath
			if configuredPath == "" {
				configuredPath = "ffmpeg"
			}
		}

		// Try to find the tool
		path, err := exec.LookPath(configuredPath)
		if err == nil {
			tools[i].Detected = true
			tools[i].Path = path
			// Try to get version
			tools[i].Version = getToolVersion(path)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
	})
}

// getToolVersion attempts to get the version of a tool.
func getToolVersion(path string) string {
	out, err := exec.Command(path, "-version").Output()
	if err != nil {
		return ""
	}

	// Extract first line as version info
	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// InstallTools attempts to auto-install system tools (ffmpeg/ffprobe)
func (h *SystemHandler) InstallTools(c *gin.Context) {
	h.log.Info("Received tools auto-install request")

	// Call service layer to auto-install
	ffprobePath, ffmpegPath := service.AutoInstallFFmpeg(h.log, h.cfg)

	result := gin.H{
		"installed": ffprobePath != "" || ffmpegPath != "",
	}

	if ffprobePath != "" {
		result["ffprobe_path"] = ffprobePath
		result["ffprobe_installed"] = true
	}
	if ffmpegPath != "" {
		result["ffmpeg_path"] = ffmpegPath
		result["ffmpeg_installed"] = true
	}

	// Re-detect tool status
	tools := []ToolStatus{
		{Name: "ffprobe", DisplayName: "FFprobe", ConfigKey: "app.ffprobe_path"},
		{Name: "ffmpeg", DisplayName: "FFmpeg", ConfigKey: "app.ffmpeg_path"},
	}

	for i := range tools {
		var configuredPath string
		switch tools[i].ConfigKey {
		case "app.ffprobe_path":
			configuredPath = h.cfg.App.FFprobePath
			if configuredPath == "" {
				configuredPath = "ffprobe"
			}
		case "app.ffmpeg_path":
			configuredPath = h.cfg.App.FFmpegPath
			if configuredPath == "" {
				configuredPath = "ffmpeg"
			}
		}

		path, err := exec.LookPath(configuredPath)
		if err == nil {
			tools[i].Detected = true
			tools[i].Path = path
			tools[i].Version = getToolVersion(path)
		}
	}

	result["tools"] = tools

	h.log.Info("Tool installation completed", zap.Any("result", result))
	c.JSON(http.StatusOK, result)
}
