// Package webui embeds the built Vue frontend (frontend/dist, copied here by
// the frontend build) into the Podtainer binary so it ships as one file.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded frontend build rooted at dist/, ready to serve.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
