package service

import (
	"context"
	"net/url"
	"path"
	"strings"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
)

// CloudMountInfo 是云盘挂载库的规范标识（因网盘后端已移除，仅保留类型以兼容
// 既有调用点；实际不会有 cloud:// 路径）。
type CloudMountInfo struct {
	Provider   string
	DisplayDir string
	ScanDir    string
	Path       string
}

// ParseCloudLibraryMount 原用于解析 cloud:// 挂载库路径。网盘后端已移除，恒
// 返回 (空, false)。
func ParseCloudLibraryMount(_ string) (CloudMountInfo, bool) {
	return CloudMountInfo{}, false
}

func cloudMountAncestor(_, _ string) bool {
	return false
}

func cloudRootMountNeedsAutoCategory(_ CloudMountInfo) bool {
	return false
}

func appendUniqueLibraryIDs(ids []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		exists := false
		for _, id := range ids {
			if id == value {
				exists = true
				break
			}
		}
		if !exists {
			ids = append(ids, value)
		}
	}
	return ids
}

func compactLibraryIDs(ids ...string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = appendUniqueLibraryIDs(out, id)
	}
	return out
}

// 云盘库展示/合并辅助函数。
//
// 网盘后端（存储配置/云盘扫描/云播放）已随「存储配置」功能整体移除，因此
// 库表里不再存在 cloud:// 挂载库。以下函数保留为空实现以保持媒体浏览、
// 检索与 Emby 兼容层对原有调用点的兼容；在没有云盘库的前提下它们的
// 语义等价于「不合并、不过滤、不开自动分类」。

// FilterDisplayCloudLibraries 原用于筛掉云盘库（展示时不单独列出）。现已无
// 云盘库，原样返回。
func FilterDisplayCloudLibraries(_ context.Context, _ *repository.Container, libs []model.Library) []model.Library {
	return libs
}

// MergedLibraryIDsForLibrary 原用于把合并展示的云盘库 ID 集合展开。现已无云盘
// 库，仅返回目标库自身 ID。
func MergedLibraryIDsForLibrary(_ context.Context, _ *repository.Container, libraryID string) ([]string, error) {
	return []string{libraryID}, nil
}

// ExpandMediaVisibilityForMergedCloudLibraries 原用于把用户可见范围展开到合并
// 的云盘库。现已无云盘库，原样返回。
func ExpandMediaVisibilityForMergedCloudLibraries(_ context.Context, _ *repository.Container, visibility MediaVisibility) MediaVisibility {
	return visibility
}

// CloudLibraryAutoCategory 原用于识别「自动分类」云盘库。现已无云盘库，恒为
// false。
func CloudLibraryAutoCategory(_ model.Library) bool {
	return false
}

// CloudLibraryMergeKey 原用于计算两个云盘库的合并键。现已无云盘库，返回
// (空, false)。
func CloudLibraryMergeKey(_ model.Library) (string, bool) {
	return "", false
}

// ShadowedCloudLibraryIDSet 原用于返回被合并/遮蔽的云盘库 ID 集合。现已无云盘
// 库，返回空集合。
func ShadowedCloudLibraryIDSet(_ []model.Library) map[string]bool {
	return map[string]bool{}
}

// NormalizeCloudLibraryDisplay 原用于归一化云盘库的展示名/类型。现已无云盘库，
// 原样返回。
func NormalizeCloudLibraryDisplay(libs []model.Library) []model.Library {
	return libs
}

// normalizeRemotePath 归一化远程（云盘/STRM 目标）路径。属通用路径处理辅助，
// 保留供 STRM 目标路径使用。
func normalizeRemotePath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	if p == "" || p == "." {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

// scanHasImportChanges 报告一次扫描是否产生了入库/变更。属通用扫描辅助。
func scanHasImportChanges(res *ScanResult) bool {
	return res != nil && (res.Added > 0 || res.Updated > 0 || res.Removed > 0)
}

// cloneLocalMetadata 深拷贝 LocalMetadata 值（值类型浅拷贝即可）。属通用辅助。
func cloneLocalMetadata(src *LocalMetadata) *LocalMetadata {
	if src == nil {
		return nil
	}
	cp := *src
	return &cp
}

// joinRemotePath 拼接远程路径片段（供 STRM 目标路径使用）。属通用路径辅助。
func joinRemotePath(base, rel string) string {
	parts := []string{normalizeRemotePath(base)}
	for _, part := range strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/") {
		part = strings.TrimSpace(part)
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return path.Clean(path.Join(parts...))
}

// 通用库路径辅助（原云盘库/STRM 生成逻辑使用；网盘后端移除后保留为纯工具函数，
// 供库路径构建与既有测试作为稳定夹具使用）。

const LegacyQuarkProvider = "quark"

func BuildCloudLibraryPath(provider, scanDir, displayDir string) string {
	provider = strings.TrimSpace(provider)
	scanDir = normalizeCloudMountDir(provider, scanDir)
	displayDir = normalizeCloudMountDir(provider, firstNonEmpty(displayDir, scanDir))
	if provider == "" {
		return ""
	}
	base := "cloud://" + provider
	if displayDir == "" {
		if scanDir != "" {
			return base + "?dir=" + url.QueryEscape(scanDir)
		}
		return base
	}
	pathStr := base + "/" + url.PathEscape(displayDir)
	if scanDir != "" && scanDir != displayDir {
		pathStr += "?dir=" + url.QueryEscape(scanDir)
	}
	return pathStr
}

func BuildCloudAutoCategoryLibraryPath(provider, displayDir string) string {
	return BuildCloudAutoCategoryLibraryPathWithScanDir(provider, "", displayDir)
}

func BuildCloudAutoCategoryLibraryPathWithScanDir(provider, scanDir, displayDir string) string {
	base := BuildCloudLibraryPath(provider, scanDir, displayDir)
	if base == "" || strings.TrimSpace(displayDir) == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "auto_category=1"
}

func normalizeCloudMountDir(provider, value string) string {
	value = strings.TrimSpace(value)
	if decoded, err := url.PathUnescape(value); err == nil {
		value = decoded
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "." || ((provider == "115" || provider == LegacyQuarkProvider) && value == "0") {
		return ""
	}
	return value
}

func cloudMountDirBase(dir string) string {
	dir = strings.Trim(strings.TrimSpace(strings.ReplaceAll(dir, "\\", "/")), "/")
	if dir == "" {
		return ""
	}
	parts := strings.Split(dir, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if part := strings.TrimSpace(parts[i]); part != "" {
			return part
		}
	}
	return ""
}

func CloudMountProviderLabel(provider string) string {
	switch strings.TrimSpace(provider) {
	case LegacyQuarkProvider:
		return "已停用网盘"
	case "115":
		return "115 网盘"
	case "clouddrive2":
		return "CloudDrive2"
	case "openlist":
		return "OpenList"
	default:
		if strings.TrimSpace(provider) == "" {
			return "网盘"
		}
		return strings.TrimSpace(provider)
	}
}

func CloudArtworkURL(typ, ref string) string {
	typ = strings.Trim(strings.ReplaceAll(strings.TrimSpace(typ), "\\", "/"), "/")
	ref = strings.TrimSpace(ref)
	if typ == "" || ref == "" {
		return ""
	}
	return "/api/img/cloud/" + url.PathEscape(typ) + "?ref=" + url.QueryEscape(ref)
}
