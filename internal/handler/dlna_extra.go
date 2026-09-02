// Package handler — per-renderer DLNA control endpoints used by the
// Vue UI. These are best-effort SOAP calls; failures surface as 4xx.
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/service"
)

// dlnaControlPath maps the action name to the AVTransport SOAP body.
// (kept for parity with the upstream Vue admin UI)
type dlnaAction string

const (
	_dlnaPlay  dlnaAction = "Play"
	_dlnaPause dlnaAction = "Pause"
	_dlnaStop  dlnaAction = "Stop"
)

var _ = []dlnaAction{_dlnaPlay, _dlnaPause, _dlnaStop}

// findRendererControlURL returns the cached control URL for the given
// uuid (matched against the device UDN). We rely on DLNAService's
// existing Discover() cache.
func findRendererControlURL(ctx context.Context, svc *service.Container, uuid string) (string, error) {
	devs, err := svc.DLNA.Discover(ctx, false)
	if err != nil {
		return "", err
	}
	for _, d := range devs {
		if d.UDN == uuid || strings.HasSuffix(d.UDN, uuid) {
			return d.ControlURL, nil
		}
	}
	return "", errors.New("renderer not found")
}

// dlnaPlayHandler resumes playback on the chosen renderer.
func dlnaPlayHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		controlURL, err := findRendererControlURL(c.Request.Context(), svc, c.Param("uuid"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		envelope := buildSimpleAVTransport("Play", `<Speed>1</Speed>`)
		if err := svc.DLNA.SOAP(c.Request.Context(), controlURL, "Play", envelope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// dlnaPauseHandler pauses playback on the chosen renderer.
func dlnaPauseHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		controlURL, err := findRendererControlURL(c.Request.Context(), svc, c.Param("uuid"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		envelope := buildSimpleAVTransport("Pause", "")
		if err := svc.DLNA.SOAP(c.Request.Context(), controlURL, "Pause", envelope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// dlnaStopHandler stops playback.
func dlnaStopHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		controlURL, err := findRendererControlURL(c.Request.Context(), svc, c.Param("uuid"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		envelope := buildSimpleAVTransport("Stop", "")
		if err := svc.DLNA.SOAP(c.Request.Context(), controlURL, "Stop", envelope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// dlnaStatusHandler returns "playing" / "paused" / "stopped" via
// GetTransportInfo. We don't parse the response — the UI can read the
// raw body via the upstream proxy if it needs more detail.
func dlnaStatusHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		controlURL, err := findRendererControlURL(c.Request.Context(), svc, c.Param("uuid"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		envelope := buildSimpleAVTransport("GetTransportInfo", "")
		if err := svc.DLNA.SOAP(c.Request.Context(), controlURL, "GetTransportInfo", envelope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// buildSimpleAVTransport assembles a SOAP body for the given action +
// extra body fragment.  InstanceID is hard-coded to 0 (single zone).
func buildSimpleAVTransport(action string, extra string) string {
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>%s
    </u:%s>
  </s:Body>
</s:Envelope>`, action, extra, action,
	)
}
