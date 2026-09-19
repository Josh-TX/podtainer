// Package webui embeds the built Vue frontend (frontend/dist, copied here by
// the frontend build) into the Podtainer binary so it ships as one file.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded frontend build rooted at dist/, ready to serve.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Handler serves the embedded frontend, falling back to index.html for any
// path that isn't a real file so Vue Router's client-side routes survive a
// hard refresh (e.g. /stacks).
func Handler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "."
		}
		if _, err := fs.Stat(fsys, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
