// Package service — generic on-disk cleanup helper used by the
// scheduler. Public so handlers can call it for "purge transcode cache
// now" buttons.
package service

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// walkAndPrune recursively deletes every file under root whose mtime is
// older than cutoff. Empty directories left behind are removed too.
// Best-effort: per-file errors are ignored so a single permission denial
// doesn't abort the cleanup.
func walkAndPrune(root string, cutoff time.Time) error {
	if root == "" {
		return nil
	}
	if _, err := os.Stat(root); err != nil {
		return nil // nothing to clean
	}
	dirs := []string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != root {
				dirs = append(dirs, path)
			}
			return nil
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path) // #nosec G122 -- cache pruning is best-effort under the configured cache root.
		}
		return nil
	})
	// Remove emptied directories from deepest to shallowest.
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i])
	}
	return nil
}

// PruneImageCacheResult holds stats from an image cache prune operation.
type PruneImageCacheResult struct {
	TotalFilesBefore int
	TotalBytesBefore int64
	DeletedFiles     int
	FreedBytes       int64
	RemainingBytes   int64
}

type imageCacheFileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// PruneImageCache scans imagesDir for cached image files. If the total disk usage
// exceeds maxSizeBytes, it removes files starting from the oldest (by ModTime)
// until disk usage falls to or below targetSizeBytes (80% of maxSizeBytes).
//
// In-flight temporary files (*.tmp) are skipped to avoid corrupting concurrent writes.
// Empty subdirectories left behind are best-effort removed.
func PruneImageCache(imagesDir string, maxSizeBytes int64) (PruneImageCacheResult, error) {
	var result PruneImageCacheResult
	if imagesDir == "" || maxSizeBytes <= 0 {
		return result, nil
	}
	if _, err := os.Stat(imagesDir); err != nil {
		return result, nil
	}

	var (
		dirs    []string
		entries []imageCacheFileEntry
	)

	_ = filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != imagesDir {
				dirs = append(dirs, path)
			}
			return nil
		}
		// Skip temporary files created during image download.
		name := info.Name()
		if strings.HasSuffix(name, ".tmp") || strings.HasPrefix(name, "img-") && strings.Contains(name, ".tmp") {
			return nil
		}
		size := info.Size()
		result.TotalFilesBefore++
		result.TotalBytesBefore += size
		entries = append(entries, imageCacheFileEntry{
			path:    path,
			size:    size,
			modTime: info.ModTime(),
		})
		return nil
	})

	result.RemainingBytes = result.TotalBytesBefore
	if result.TotalBytesBefore <= maxSizeBytes {
		return result, nil
	}

	// High/Low watermark: prune down to 80% of max size to leave headroom
	// and prevent disk thrashing on consecutive writes.
	targetSizeBytes := maxSizeBytes * 80 / 100

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime.Before(entries[j].modTime)
	})

	for _, entry := range entries {
		if result.RemainingBytes <= targetSizeBytes {
			break
		}
		if err := os.Remove(entry.path); err == nil || errors.Is(err, os.ErrNotExist) {
			result.DeletedFiles++
			result.FreedBytes += entry.size
			result.RemainingBytes -= entry.size
		}
	}

	// Clean up emptied subdirectories from deepest to shallowest.
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i])
	}

	return result, nil
}

