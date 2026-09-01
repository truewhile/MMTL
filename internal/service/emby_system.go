package service

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ShukeBta/MMTL/internal/model"
)

// SystemInfo returns the full Emby identity payload.
func (e *EmbyService) SystemInfo() map[string]any {
	return map[string]any{
		"Id":                     embyServerID,
		"ServerId":               embyServerID,
		"ServerName":             "MMTL",
		"Version":                embyCompatVersion,
		"ServerVersion":          embyCompatVersion,
		"ProductName":            "Emby Server",
		"OperatingSystem":        "Windows",
		"Architecture":           "X64",
		"LocalAddress":           "",
		"WanAddress":             "",
		"HasPendingRestart":      false,
		"IsShuttingDown":         false,
		"SupportsLibraryMonitor": true,
		"SupportsHttps":          false,
		"SupportsAutoDiscovery":  true,
		"HttpServerPortNumber":   e.cfg.App.Port,
		"HttpsPortNumber":        0,
		"PublishedServerUrl":     "",
		"WebSocketPortNumber":    e.cfg.App.Port,
		"CompletedInstallations": []any{},
		"CanSelfRestart":         false,
		"CanLaunchWebBrowser":    false,
		"CanRestart":             false,
	}
}

// SystemInfoPublic 是不需要认证的精简版（Emby Web 客户端登陆前会拉）。
func (e *EmbyService) SystemInfoPublic() map[string]any {
	return map[string]any{
		"Id":                     embyServerID,
		"ServerId":               embyServerID,
		"ServerName":             "MMTL",
		"Version":                embyCompatVersion,
		"ServerVersion":          embyCompatVersion,
		"ProductName":            "Emby Server",
		"OperatingSystem":        "Windows",
		"LocalAddress":           "",
		"WanAddress":             "",
		"HttpServerPortNumber":   e.cfg.App.Port,
		"HttpsPortNumber":        0,
		"SupportsHttps":          false,
		"SupportsAutoDiscovery":  true,
		"StartupWizardCompleted": true,
	}
}

// ListUsers returns Emby-shaped users.
func (e *EmbyService) ListUsers(ctx context.Context) ([]map[string]any, error) {
	users, err := e.repo.User.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, e.userPayload(&u))
	}
	return out, nil
}

// FindUser 用 ID 查用户，用于 /Users/Me 与 /Users/{id}。
func (e *EmbyService) FindUser(ctx context.Context, id string) (map[string]any, error) {
	u, err := e.repo.User.FindByID(ctx, id)
	if err != nil || u == nil {
		return nil, err
	}
	return e.userPayload(u), nil
}

func (e *EmbyService) userPayload(u *model.User) map[string]any {
	canDownload := u.Role == "admin"
	return map[string]any{
		"Id":                        u.ID,
		"Name":                      u.Username,
		"ServerId":                  embyServerID,
		"ServerName":                "MMTL",
		"HasPassword":               true,
		"HasConfiguredPassword":     true,
		"HasConfiguredEasyPassword": false,
		"EnableAutoLogin":           false,
		"LastLoginDate":             u.LastLoginAt,
		"LastActivityDate":          u.UpdatedAt,
		"Configuration": map[string]any{
			"PlayDefaultAudioTrack":      true,
			"DisplayCollectionsView":     true,
			"DisplayMissingEpisodes":     false,
			"SubtitleMode":               "Default",
			"EnableNextEpisodeAutoPlay":  true,
			"AudioLanguagePreference":    "",
			"SubtitleLanguagePreference": "",
		},
		"Policy": map[string]any{
			"IsAdministrator":                u.Role == "admin",
			"IsHidden":                       false,
			"IsDisabled":                     !u.IsActive,
			"EnableUserPreferenceAccess":     true,
			"EnableRemoteAccess":             true,
			"EnableMediaPlayback":            true,
			"EnableAudioPlaybackTranscoding": true,
			"EnableVideoPlaybackTranscoding": true,
			"EnablePlaybackRemuxing":         true,
			"EnableLiveTvAccess":             false,
			"EnableContentDownloading":       canDownload,
			"EnableSyncTranscoding":          canDownload,
			"EnableMediaConversion":          canDownload,
			"EnableAllChannels":              true,
			"EnableAllFolders":               true,
			"EnableAllDevices":               true,
			"AuthenticationProviderId":       embyLocalAuthenticationProviderID,
			"PasswordResetProviderId":        embyLocalPasswordResetProviderID,
		},
	}
}

// Views 返回 Emby 中"虚拟根目录"——每个 library 一个条目，外加所有启用的
// 远程 Emby 挂载的媒体库（联邦聚合）。
func (e *EmbyService) Views(ctx context.Context, userID string) (map[string]any, error) {
	libs, err := e.repo.Library.List(ctx)
	if err != nil {
		return nil, err
	}
	libs = FilterDisplayCloudLibraries(ctx, e.repo, libs)
	visibility := e.mediaVisibility(ctx, userID)
	items := make([]map[string]any, 0, len(libs)+4)
	for _, l := range libs {
		if !e.libraryVisibleFromCachedVisibility(l, visibility) {
			continue
		}
		items = append(items, e.libraryAsView(ctx, &l))
	}
	for _, remote := range e.remoteViews(ctx) {
		items = append(items, remote)
	}
	return map[string]any{"Items": items, "TotalRecordCount": len(items), "StartIndex": 0}, nil
}

// remoteViews 拉取全部启用远程 Emby 账号的媒体库视图。单个账号失败只跳过
// 该账号（日志记录），不影响其他来源。
func (e *EmbyService) remoteViews(ctx context.Context) []map[string]any {
	if e == nil || e.remote == nil {
		return nil
	}
	accounts, err := e.remote.ListAccounts(ctx)
	if err != nil || len(accounts) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(accounts)*2)
	for i := range accounts {
		acct := &accounts[i]
		views, err := e.remote.RemoteViews(ctx, acct)
		if err != nil {
			if e.log != nil {
				e.log.Warn("fetch remote emby views failed",
					zap.String("account", acct.Name), zap.Error(err))
			}
			continue
		}
		for _, v := range views {
			if view := e.remoteLibraryAsView(v, acct); view != nil {
				out = append(out, view)
			}
		}
	}
	return out
}

// remoteLibraryAsView 把远程媒体的 CollectionFolder 视图标准化为本地视图
// 同构的 payload（ID 伪装、名称前缀账号名以便区分多个远程来源）。
func (e *EmbyService) remoteLibraryAsView(raw map[string]any, acct *model.StrmAccount) map[string]any {
	if raw == nil {
		return nil
	}
	remoteID, _ := raw["Id"].(string)
	if remoteID == "" {
		return nil
	}
	name, _ := raw["Name"].(string)
	if name == "" {
		name = acct.Name
	}
	collectionType, _ := raw["CollectionType"].(string)
	if !isSupportedEmbyCollectionType(collectionType) {
		collectionType = "mixed"
	}
	encoded := EncodeEmbyRemoteID(acct.ID, remoteID)
	imageTags := map[string]string{}
	if primary := embyRemoteImageTagsPrimary(raw); primary != "" {
		imageTags["Primary"] = encoded
	}
	return map[string]any{
		"Id":                       encoded,
		"Name":                     acct.Name + " · " + name,
		"CollectionType":           collectionType,
		"ServerId":                 embyServerID,
		"Type":                     "CollectionFolder",
		"IsFolder":                 true,
		"Path":                     "",
		"SortName":                 strings.ToLower(name),
		"DateCreated":              time.Now().UTC().Format(time.RFC3339),
		"CanDelete":                false,
		"CanDownload":              false,
		"DisplayPreferencesId":     encoded,
		"PrimaryImageItemId":       encoded,
		"PrimaryImageAspectRatio":  1.7777777777777777,
		"RecursiveItemCount":       0,
		"ChildCount":               0,
		"SpecialFeatureCount":      0,
		"EnableMediaSourceDisplay": true,
		"PlayAccess":               "Full",
		"ExternalUrls":             []any{},
		"ProviderIds":              map[string]string{},
		"Genres":                   []string{},
		"Tags":                     []string{},
		"ImageTags":                imageTags,
		"BackdropImageTags":        []string{},
		"UserData": map[string]any{
			"PlaybackPositionTicks": 0,
			"PlayCount":             0,
			"IsFavorite":            false,
			"Played":                false,
			"UnplayedItemCount":     0,
		},
	}
}

// embyRemoteImageTagsPrimary 提取远程 View 的 ImageTags.Primary 值。
func embyRemoteImageTagsPrimary(raw map[string]any) string {
	switch tags := raw["ImageTags"].(type) {
	case map[string]any:
		if s, ok := tags["Primary"].(string); ok {
			return s
		}
	case map[string]string:
		return tags["Primary"]
	}
	return ""
}

func isSupportedEmbyCollectionType(t string) bool {
	switch t {
	case "movies", "tvshows", "music", "mixed", "homevideos", "boxsets":
		return true
	}
	return false
}

func (e *EmbyService) libraryAsView(ctx context.Context, l *model.Library) map[string]any {
	collectionType := "movies"
	switch l.Type {
	case "tv":
		collectionType = "tvshows"
	case "anime":
		collectionType = "tvshows" // Emby 没有专门的 anime CollectionType
	case "variety":
		collectionType = "tvshows"
	case "music":
		collectionType = "music"
	}
	// 库封面:与剧集/电影一样,只有在能解析出封面时才广告 ImageTags.Primary,
	// 否则客户端会认为该库无主图而不去请求 /Images/Primary。
	imageTags := map[string]string{}
	if e.LibraryHasCover(ctx, l.ID) {
		imageTags["Primary"] = l.ID
	}
	return map[string]any{
		"Id":                       l.ID,
		"Name":                     l.Name,
		"CollectionType":           collectionType,
		"ServerId":                 embyServerID,
		"Type":                     "CollectionFolder",
		"IsFolder":                 true,
		"Path":                     l.Path,
		"SortName":                 strings.ToLower(l.Name),
		"DateCreated":              l.CreatedAt.UTC().Format(time.RFC3339),
		"CanDelete":                false,
		"CanDownload":              false,
		"DisplayPreferencesId":     l.ID,
		"PrimaryImageItemId":       l.ID,
		"PrimaryImageAspectRatio":  1.7777777777777777,
		"RecursiveItemCount":       0,
		"ChildCount":               0,
		"SpecialFeatureCount":      0,
		"EnableMediaSourceDisplay": true,
		"PlayAccess":               "Full",
		"ExternalUrls":             []any{},
		"ProviderIds":              map[string]string{},
		"Genres":                   []string{},
		"Tags":                     []string{},
		"ImageTags":                imageTags,
		"BackdropImageTags":        []string{},
		"UserData": map[string]any{
			"PlaybackPositionTicks": 0,
			"PlayCount":             0,
			"IsFavorite":            false,
			"Played":                false,
			"UnplayedItemCount":     0,
		},
	}
}
