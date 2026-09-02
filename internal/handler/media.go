// Package handler — library / media HTTP endpoints.
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/truewhile/MeBox/internal/middleware"
	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/service"
)

type createLibraryReq struct {
	Name               string                     `json:"name"`
	Path               string                     `json:"path"`
	Paths              []string                   `json:"paths"`
	Roots              []service.LibraryRootInput `json:"roots"`
	Type               string                     `json:"type"`
	CoverURL           string                     `json:"cover_url"`
	CreatePerSubfolder bool                       `json:"create_per_subfolder"`
}

// webLibraryPayload 是 /api/libraries 返回的库条目：本地库与远程 Emby 挂载库
// 统一结构（远程库附加 is_remote_emby / remote_source 只读标记）。
type webLibraryPayload struct {
	model.Library
	IsRemoteEmby bool                 `json:"is_remote_emby,omitempty"`
	RemoteSource string               `json:"remote_source,omitempty"`
	Total        int64                `json:"total,omitempty"`
	Cards        []service.SeriesCard `json:"cards,omitempty"`
}

// remoteLibraryItemTypes 远程库内容拉取时按 CollectionType 过滤直属条目，
// 避免电影库里的合集文件夹(Folder) 漏出为电影卡片。
func remoteLibraryItemTypes(collectionType string) string {
	switch collectionType {
	case "movies":
		return "Movie"
	case "tvshows":
		return "Series"
	}
	return ""
}

func listLibrariesHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		libs, err := svc.Media.ListLibraries(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		role, _ := c.Get(middleware.CtxUserRole)
		includeHidden := role == "admin" && (c.Query("include_hidden") == "1" || c.Query("include_hidden") == "true" || c.Query("all") == "1")
		if !includeHidden {
			libs = service.FilterDisplayCloudLibraries(ctx, svc.Repo, libs)
			visibility := mediaVisibilityForRequest(c, svc)
			filtered := libs[:0]
			for _, lib := range libs {
				if service.LibraryVisibleForUser(ctx, svc.Repo, lib, visibility) {
					filtered = append(filtered, lib)
				}
			}
			libs = filtered
		}
		withPreview := c.Query("with_preview") == "1" || c.Query("with_preview") == "true"
		limit := 10
		if withPreview {
			limit, _ = strconv.Atoi(c.DefaultQuery("preview_limit", c.DefaultQuery("limit", "10")))
			if limit <= 0 {
				limit = 10
			} else if limit > 100 {
				limit = 100
			}
		}
		out := make([]webLibraryPayload, 0, len(libs)+8)
		if withPreview {
			previews, err := svc.Media.ListLibrariesWithPreview(ctx, libs, mediaVisibilityForRequest(c, svc), limit)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			for _, p := range previews {
				out = append(out, webLibraryPayload{Library: p.Library, Total: p.Total, Cards: p.Cards})
			}
		} else {
			for _, l := range libs {
				out = append(out, webLibraryPayload{Library: l})
			}
		}
		// 远程 Emby 挂载库追加在本地库之后。
		if svc.EmbyRemote != nil {
			if views, err := svc.EmbyRemote.RemoteLibraries(ctx); err == nil {
				remotePayloads := make([]webLibraryPayload, len(views))
				for i, v := range views {
					remotePayloads[i] = webLibraryPayload{Library: v.Library, IsRemoteEmby: true, RemoteSource: v.AccountName}
				}
				if withPreview && len(views) > 0 {
					const maxRemotePreviewWorkers = 6
					sem := make(chan struct{}, maxRemotePreviewWorkers)
					var wg sync.WaitGroup
					for i, v := range views {
						i, v := i, v
						wg.Add(1)
						go func() {
							defer wg.Done()
							select {
							case sem <- struct{}{}:
								defer func() { <-sem }()
							case <-ctx.Done():
								return
							}
							acct := svc.EmbyRemote.AccountByID(ctx, v.AccountID)
							if acct == nil {
								return
							}
							tmpMount := &model.EmbyMount{Base: model.Base{ID: v.MountID}}
							itemTypes := remoteLibraryItemTypes(v.CollectionType)
							if _, total, err := svc.EmbyRemote.RemoteLibraryMedia(ctx, tmpMount, acct, v.RemoteID, itemTypes, 0, 1); err == nil {
								remotePayloads[i].Total = total
							}
							if cards, err := svc.EmbyRemote.RemoteLatestCards(ctx, tmpMount, acct, v.RemoteID, limit); err == nil {
								remotePayloads[i].Cards = cards
							}
						}()
					}
					wg.Wait()
				}
				out = append(out, remotePayloads...)
			}
		}
		c.JSON(http.StatusOK, out)
	}
}

func getLibraryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		// 远程 Emby 挂载库详情。
		if svc.EmbyRemote != nil && service.IsEmbyRemoteID(id) {
			mountID, remoteID, _ := service.DecodeEmbyRemoteID(id)
			view, err := svc.EmbyRemote.RemoteLibraryByID(ctx, mountID, remoteID)
			if err != nil || view == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusOK, webLibraryPayload{Library: view.Library, IsRemoteEmby: true, RemoteSource: view.AccountName})
			return
		}
		lib, err := svc.Repo.Library.FindByID(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if lib == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		role, _ := c.Get(middleware.CtxUserRole)
		includeHidden := role == "admin" && (c.Query("include_hidden") == "1" || c.Query("include_hidden") == "true" || c.Query("all") == "1")
		if !includeHidden {
			libs := service.FilterDisplayCloudLibraries(ctx, svc.Repo, []model.Library{*lib})
			if len(libs) == 0 || !service.LibraryVisibleForUser(ctx, svc.Repo, libs[0], mediaVisibilityForRequest(c, svc)) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusOK, webLibraryPayload{Library: libs[0]})
		} else {
			c.JSON(http.StatusOK, webLibraryPayload{Library: *lib})
		}
	}
}

func createLibraryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createLibraryReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		roots := req.Roots
		if len(roots) == 0 {
			for _, path := range req.Paths {
				roots = append(roots, service.LibraryRootInput{Path: path})
			}
		}
		if len(roots) == 0 && strings.TrimSpace(req.Path) != "" {
			roots = append(roots, service.LibraryRootInput{Path: req.Path})
		}
		var l *model.Library
		if req.CreatePerSubfolder {
			parent := ""
			if len(roots) > 0 {
				parent = roots[0].Path
			} else if strings.TrimSpace(req.Path) != "" {
				parent = req.Path
			}
			created, err := svc.Media.CreateLibrariesPerSubfolder(c.Request.Context(), parent, req.Type, req.CoverURL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			uid, _ := c.Get("ctx_user_id")
			for i := range created {
				lib := &created[i]
				svc.Audit.Record(c.Request.Context(), toString(uid), "library.create", lib.ID, c.ClientIP(), lib.Path)
				if svc.Watcher != nil {
					go func() { _ = svc.Watcher.Refresh(context.Background()) }()
				}
				for _, root := range lib.Roots {
					if root.Enabled {
						queueLibraryRootScan(svc, lib.ID, root.ID)
					}
				}
			}
			c.JSON(http.StatusCreated, gin.H{"libraries": created})
			return
		}
		l, err := svc.Media.CreateLibraryWithRootsAndCover(c.Request.Context(), req.Name, req.Type, req.CoverURL, roots)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		uid, _ := c.Get("ctx_user_id")
		svc.Audit.Record(c.Request.Context(), toString(uid), "library.create", l.ID, c.ClientIP(), l.Path)
		// Refresh fsnotify watcher to pick up the new library root, then perform
		// an initial scan. Without the scan a newly-created library remained
		// empty until the operator pressed the separate "扫描" action.
		if svc.Watcher != nil {
			go func() { _ = svc.Watcher.Refresh(context.Background()) }()
		}
		if len(l.Roots) == 0 {
			queueLibraryRootScan(svc, l.ID, "")
		} else {
			for _, root := range l.Roots {
				if root.Enabled {
					queueLibraryRootScan(svc, l.ID, root.ID)
				}
			}
		}
		c.JSON(http.StatusCreated, l)
	}
}

type updateLibraryReq struct {
	CoverURL        *string `json:"cover_url"`
	SortOrder       *int    `json:"sort_order"`
	CarouselEnabled *bool   `json:"carousel_enabled"`
}

func updateLibraryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req updateLibraryReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.CoverURL != nil {
			if err := svc.Media.UpdateLibraryCover(c.Request.Context(), c.Param("id"), *req.CoverURL); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		if req.SortOrder != nil || req.CarouselEnabled != nil {
			if err := svc.Media.UpdateLibraryFields(c.Request.Context(), c.Param("id"), req.SortOrder, req.CarouselEnabled); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		lib, err := svc.Repo.Library.FindByID(c.Request.Context(), c.Param("id"))
		if err != nil || lib == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "library not found"})
			return
		}
		c.JSON(http.StatusOK, lib)
	}
}

type reorderLibrariesReq struct {
	IDs []string `json:"ids" binding:"required"`
}

func reorderLibrariesHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reorderLibrariesReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := svc.Media.ReorderLibraries(c.Request.Context(), req.IDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"updated": len(req.IDs)})
	}
}

func deleteLibraryHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := svc.Media.DeleteLibrary(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		uid, _ := c.Get("ctx_user_id")
		svc.Audit.Record(c.Request.Context(), toString(uid), "library.delete", id, c.ClientIP(), "")
		go func() { _ = svc.Watcher.Refresh(context.Background()) }()
		c.Status(http.StatusNoContent)
	}
}

func listMediaHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx := c.Request.Context()
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
		// 远程 Emby 库：转发远程直属条目并映射为本地 Media 结构（分页由远程承接）。
		if svc.EmbyRemote != nil && service.IsEmbyRemoteID(id) {
			mountID, remoteID, _ := service.DecodeEmbyRemoteID(id)
			mount, acct, _ := svc.EmbyRemote.ResolveMount(ctx, mountID)
			if mount == nil || acct == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			itemTypes := ""
			if view, err := svc.EmbyRemote.RemoteLibraryByID(ctx, mountID, remoteID); err == nil && view != nil {
				itemTypes = remoteLibraryItemTypes(view.CollectionType)
			}
			items, total, err := svc.EmbyRemote.RemoteLibraryMedia(ctx, mount, acct, remoteID, itemTypes, (page-1)*size, size)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if items == nil {
				items = []model.Media{}
			}
			c.JSON(http.StatusOK, gin.H{
				"items":     items,
				"total":     total,
				"page":      page,
				"page_size": size,
			})
			return
		}
		groupVersions := c.DefaultQuery("group_versions", "1") != "0"
		if !groupVersions {
			items, total, err := svc.Media.ListMediaVisible(c.Request.Context(), id, page, size, mediaVisibilityForRequest(c, svc))
			if err != nil {
				writeInternalOrCanceled(c, err)
				return
			}
			if items == nil {
				items = []model.Media{}
			}
			c.JSON(http.StatusOK, gin.H{
				"items":     items,
				"total":     total,
				"page":      page,
				"page_size": size,
			})
			return
		}
		items, total, err := svc.Media.ListMediaVisibleGrouped(c.Request.Context(), id, page, size, mediaVisibilityForRequest(c, svc))
		if err != nil {
			writeInternalOrCanceled(c, err)
			return
		}
		if items == nil {
			items = []service.MediaItem{}
		}
		c.JSON(http.StatusOK, gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": size,
		})
	}
}

func getMediaHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		// 远程 Emby 条目：拉远程详情并映射为本地 Media 结构。
		if svc.EmbyRemote != nil && service.IsEmbyRemoteID(id) {
			mountID, remoteID, _ := service.DecodeEmbyRemoteID(id)
			mount, acct, _ := svc.EmbyRemote.ResolveMount(ctx, mountID)
			if mount == nil || acct == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			m, err := svc.EmbyRemote.RemoteMediaDetail(ctx, mount, acct, remoteID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if m == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			if !mediaVisibleForRequest(c, svc, m) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusOK, m)
			return
		}
		m, err := svc.Media.GetMedia(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if m == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if !mediaVisibleForRequest(c, svc, m) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusOK, m)
	}
}

func updateMediaMetadataHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req service.MediaMetadataUpdate
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		m, err := svc.Media.UpdateMetadata(c.Request.Context(), c.Param("id"), req)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				status = http.StatusNotFound
			} else if strings.Contains(strings.ToLower(err.Error()), "required") {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, m)
	}
}

func searchMediaHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		groupVersions := c.DefaultQuery("group_versions", "1") != "0"
		if c.Query("page") != "" || c.Query("page_size") != "" {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
			if !groupVersions {
				items, total, err := svc.Media.SearchMediaVisiblePage(c.Request.Context(), q, page, size, mediaVisibilityForRequest(c, svc))
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{
					"items":     items,
					"total":     total,
					"page":      page,
					"page_size": size,
				})
				return
			}
			items, total, err := svc.Media.SearchMediaVisiblePageGrouped(c.Request.Context(), q, page, size, mediaVisibilityForRequest(c, svc))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"items":     items,
				"total":     total,
				"page":      page,
				"page_size": size,
			})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if !groupVersions {
			items, err := svc.Media.SearchMediaVisible(c.Request.Context(), q, limit, mediaVisibilityForRequest(c, svc))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"items": items})
			return
		}
		items, err := svc.Media.SearchMediaVisibleGrouped(c.Request.Context(), q, limit, mediaVisibilityForRequest(c, svc))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

func streamHandler(svc *service.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("id")
		// 远程 Emby 条目：按挂载代理配置分流——代理走 MeBox 反代，否则 302 直连。
		if svc.EmbyRemote != nil && service.IsEmbyRemoteID(id) {
			if !enforceScopedPlaybackToken(c, id) {
				return
			}
			mountID, remoteID, _ := service.DecodeEmbyRemoteID(id)
			mount, acct, _ := svc.EmbyRemote.ResolveMount(ctx, mountID)
			if mount == nil || acct == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			if mount.ProxyPlay {
				if err := svc.Emby.ProxyRemoteVideoStream(ctx, c.Writer, c.Request, mountID, remoteID); err != nil {
					if !c.Writer.Written() {
						c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
					}
				}
				return
			}
			target, err := svc.EmbyRemote.WebStreamURL(ctx, acct, remoteID)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			setRedirectNoStoreHeaders(c)
			c.Redirect(http.StatusFound, target)
			return
		}
		m, err := svc.Media.GetMedia(ctx, id)
		if err != nil || m == nil || !mediaVisibleForRequest(c, svc, m) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if !enforceScopedPlaybackToken(c, m.ID) {
			return
		}
		err = svc.Stream.ServeFile(c.Writer, c.Request, c.Param("id"))
		if errors.Is(err, service.ErrMediaNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if errors.Is(err, service.ErrCloudPlaybackDisabled) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, service.ErrCloudPlaybackUnavailable) {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
}
