package service

import (
	"strings"
	"unicode"
)

// 整理入库与外部搜索依赖的通用标题归一化辅助函数。
// 这些函数原本与订阅/下载逻辑共存，移除对应功能时保留为通用帮助函数。

func normalizeAvailabilityComparable(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func compactUniqueStrings(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := normalizeAvailabilityComparable(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func titleMatchesResolution(titleFold, resolution string) bool {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "2160p", "4k", "uhd":
		return strings.Contains(titleFold, "2160p") || strings.Contains(titleFold, "4k") || strings.Contains(titleFold, "uhd")
	case "1080p":
		return strings.Contains(titleFold, "1080p") || strings.Contains(titleFold, "fhd")
	case "720p":
		return strings.Contains(titleFold, "720p")
	default:
		return strings.Contains(titleFold, strings.ToLower(strings.TrimSpace(resolution)))
	}
}

func detectResolutionScore(titleFold string) int {
	switch {
	case titleMatchesResolution(titleFold, "2160p"):
		return 4
	case titleMatchesResolution(titleFold, "1080p"):
		return 3
	case titleMatchesResolution(titleFold, "720p"):
		return 2
	default:
		return 1
	}
}
