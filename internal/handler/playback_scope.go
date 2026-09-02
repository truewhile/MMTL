package handler

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/middleware"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/service"
)

func enforceScopedPlaybackToken(c *gin.Context, mediaID string) bool {
	mediaID = strings.TrimSpace(mediaID)
	purpose, _ := c.Get(middleware.CtxTokenPurpose)
	if strings.TrimSpace(toString(purpose)) == "" {
		return true
	}
	if strings.TrimSpace(toString(purpose)) != service.ExternalPlaybackTokenPurpose {
		c.JSON(http.StatusForbidden, gin.H{"error": "playback token scope denied"})
		return false
	}
	tokenMediaID, _ := c.Get(middleware.CtxTokenMediaID)
	if mediaID == "" || strings.TrimSpace(toString(tokenMediaID)) != mediaID {
		c.JSON(http.StatusForbidden, gin.H{"error": "playback token media mismatch"})
		return false
	}
	return true
}

func enforceScopedCloudPlaybackToken(c *gin.Context, svc *service.Container, typ, ref string) bool {
	purpose, _ := c.Get(middleware.CtxTokenPurpose)
	if strings.TrimSpace(toString(purpose)) == "" {
		return true
	}
	if strings.TrimSpace(toString(purpose)) != service.ExternalPlaybackTokenPurpose {
		c.JSON(http.StatusForbidden, gin.H{"error": "playback token scope denied"})
		return false
	}
	tokenMediaID, _ := c.Get(middleware.CtxTokenMediaID)
	mediaID := strings.TrimSpace(toString(tokenMediaID))
	if mediaID == "" || strings.TrimSpace(c.Query("media_id")) != mediaID {
		c.JSON(http.StatusForbidden, gin.H{"error": "playback token media mismatch"})
		return false
	}
	m, err := svc.Repo.Media.FindByID(c.Request.Context(), mediaID)
	if err != nil || m == nil || !mediaVisibleForRequest(c, svc, m) {
		c.JSON(http.StatusNotFound, gin.H{"error": "media not found"})
		return false
	}
	if !cloudPlaybackTargetMatchesMedia(m, typ, ref) {
		c.JSON(http.StatusForbidden, gin.H{"error": "playback token target mismatch"})
		return false
	}
	return true
}

func cloudPlaybackTargetMatchesMedia(m *model.Media, typ, ref string) bool {
	if m == nil {
		return false
	}
	if strmTyp, strmRef, ok := parseCloudPlaybackTarget(m.STRMURL); ok &&
		strings.EqualFold(strmTyp, typ) && sameCloudPlaybackRef(strmRef, ref) {
		return true
	}
	pathTyp, pathRef, ok := parseCloudMediaPath(m.Path)
	return ok && strings.EqualFold(pathTyp, typ) && sameCloudPlaybackRef(pathRef, ref)
}

func parseCloudPlaybackTarget(raw string) (typ, ref string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	pathValue := strings.ToLower(strings.TrimRight(u.Path, "/"))
	const prefix = "/api/cloud/play/"
	idx := strings.LastIndex(pathValue, prefix)
	if idx < 0 {
		return "", "", false
	}
	typ = strings.TrimSpace(u.Path[idx+len(prefix):])
	ref = strings.TrimSpace(u.Query().Get("ref"))
	return typ, ref, typ != "" && ref != ""
}

func parseCloudMediaPath(raw string) (typ, ref string, ok bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "cloud://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(raw, "cloud://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	typ = strings.TrimSpace(parts[0])
	ref = strings.TrimSpace(parts[1])
	return typ, ref, typ != "" && ref != ""
}

func sameCloudPlaybackRef(a, b string) bool {
	return normalizeCloudPlaybackRef(a) == normalizeCloudPlaybackRef(b)
}

func normalizeCloudPlaybackRef(value string) string {
	value = strings.TrimSpace(value)
	if decoded, err := url.PathUnescape(value); err == nil {
		value = decoded
	}
	value = strings.TrimSpace(value)
	return strings.TrimLeft(value, "/")
}
