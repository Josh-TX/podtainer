package api

import (
	"encoding/json"
	"fmt"
	"mime"
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
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(p)}))
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

type filesResult struct {
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}

// checkFilesConflicts reports which of the given destination paths exist.
func checkFilesConflicts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Dests []string `json:"dests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		for _, d := range body.Dests {
			if !filepath.IsAbs(d) {
				writeErr(w, 400, fmt.Errorf("paths must be absolute"))
				return
			}
		}
		writeJSON(w, map[string][]string{"existing": files.Existing(body.Dests)})
	}
}

// pasteFiles moves or copies many items; failures don't stop the batch.
func pasteFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Items []struct {
				Src        string `json:"src"`
				Dest       string `json:"dest"`
				Resolution string `json:"resolution"`
			} `json:"items"`
			IsCopy bool `json:"isCopy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		results := []filesResult{}
		for _, it := range body.Items {
			if !filepath.IsAbs(it.Src) || !filepath.IsAbs(it.Dest) {
				results = append(results, filesResult{it.Src, "paths must be absolute"})
				continue
			}
			if _, err := files.Paste(filepath.Clean(it.Src), filepath.Clean(it.Dest), body.IsCopy, it.Resolution); err != nil {
				results = append(results, filesResult{it.Src, err.Error()})
			}
		}
		writeJSON(w, map[string][]filesResult{"errors": results})
	}
}

// deleteFilesBatch deletes many paths; failures don't stop the batch.
func deleteFilesBatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Paths []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		results := []filesResult{}
		for _, p := range body.Paths {
			c, err := files.Clean(p)
			if err == nil {
				err = files.Delete(c)
			}
			if err != nil {
				results = append(results, filesResult{p, err.Error()})
			}
		}
		writeJSON(w, map[string][]filesResult{"errors": results})
	}
}

func searchFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := filesPath(w, r, "path")
		if !ok {
			return
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			writeErr(w, 400, fmt.Errorf("query required"))
			return
		}
		if isDir, _, err := files.Stat(p); err != nil {
			writeErr(w, 404, err)
			return
		} else if !isDir {
			writeErr(w, 400, fmt.Errorf("not a directory"))
			return
		}
		results, truncated, err := files.Search(r.Context(), p, q, r.URL.Query().Get("regex") == "1")
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, map[string]any{"results": results, "truncated": truncated})
	}
}
