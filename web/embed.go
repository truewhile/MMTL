// Package webui embeds the built React SPA so a single binary can serve the
// UI without requiring a separate web/dist directory on disk. The embed
// happens at compile time, so `web/dist` must exist before `go build` runs
// (the CI pipeline builds it via `npm run build` first).
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded SPA build artifacts rooted at the directory
// containing index.html (i.e. web/dist).
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}