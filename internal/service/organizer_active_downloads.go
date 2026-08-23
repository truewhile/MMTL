package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type activeDownloadGuard struct {
	paths []string
}

func (o *OrganizerService) newActiveDownloadGuard(ctx context.Context) activeDownloadGuard {
	if o == nil || o.activeDownloadPaths == nil {
		return activeDownloadGuard{}
	}
	return activeDownloadGuard{paths: cleanUniqueExistingPaths(o.activeDownloadPaths(ctx))}
}

func (g activeDownloadGuard) contains(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return false
	}
	for _, root := range g.paths {
		if pathWithin(path, root) {
			return true
		}
	}
	return false
}

func cleanUniqueExistingPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if sameLibraryPath(existing, path) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, path)
		}
	}
	return out
}
