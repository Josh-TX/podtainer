package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"podtainer/internal/files"
)

// filesPath reads and validates the absolute ?path= query param.
func filesPath(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	p, err := files.Clean(r.URL.Query().Get(key))
	if err != nil {
		writeErr(w, 400, err)
		return "", false
	}
	return p, true
}

// getFiles serves a directory listing, a file's editor content, or (with
// ?download=1) the raw file bytes.
func getFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		isDir, size, err := files.Stat(p)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		if r.URL.Query().Get("download") == "1" {
			if isDir {
				writeErr(w, 400, fmt.Errorf("cannot download a directory"))
				return
			}
			http.ServeFile(w, r, p)
			return
		}
		if isDir {
			entries, err := files.ListDir(p)
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			writeJSON(w, map[string]any{"isDir": true, "path": p, "entries": entries})
			return
		}
		content, editable, err := files.ReadText(p, size)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]any{"isDir": false, "path": p, "size": size, "editable": editable, "content": content})
	}
}

func getFilesDirSize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		size, err := files.DirSize(r.Context(), p)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]int64{"size": size})
	}
}

func createFilesEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		q := r.URL.Query()
		if q.Get("upload") == "1" {
			if err := files.Upload(r.Body, p, q.Get("overwrite") == "1"); err != nil {
				writeErr(w, 500, err)
				return
			}
			writeJSON(w, map[string]string{"status": "uploaded"})
			return
		}
		var body struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		var err error
		switch body.Type {
		case "file":
			err = files.CreateFile(p)
		case "dir":
			err = files.Mkdir(p)
		default:
			writeErr(w, 400, fmt.Errorf(`type must be "file" or "dir"`))
			return
		}
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "created"})
	}
}

func writeFilesEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := files.WriteFile(p, body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	}
}

func moveFilesEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		var body struct {
			Dest   string `json:"dest"`
			IsCopy bool   `json:"isCopy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if !filepath.IsAbs(body.Dest) {
			writeErr(w, 400, fmt.Errorf("destination must be absolute"))
			return
		}
		if err := files.Move(p, filepath.Clean(body.Dest), body.IsCopy); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "done"})
	}
}

func chmodFilesEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		var body struct {
			Perm      int  `json:"perm"`
			Recursive bool `json:"recursive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := files.Chmod(p, body.Perm, body.Recursive); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func deleteFilesEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		if err := files.Delete(p); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}
