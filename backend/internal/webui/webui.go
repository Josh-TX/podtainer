// Package webui embeds the built Vue frontend (the frontend build writes dist/
// here) into the Podtainer binary so it ships as one file. It is only embedded
// when built with -tags embed_frontend.
package webui

import (
	"io/fs"
	"net/http"
	"strings"
)

// Embedded reports whether a frontend build is embedded.
func Embedded() bool {
	_, err := fs.Stat(distFS, "dist/index.html")
	return err == nil
}

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
