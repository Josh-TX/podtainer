// Package api wires the HTTP surface for Podtainer's four views: stacks,
// quadlets, systemd units, and podman containers.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"podtainer/internal/auth"
	"podtainer/internal/config"
	"podtainer/internal/favorites"
	"podtainer/internal/images"
	"podtainer/internal/podmanx"
	"podtainer/internal/quadlets"
	"podtainer/internal/shellsvc"
	"podtainer/internal/stacks"
	"podtainer/internal/sysdunits"
	"podtainer/internal/volumes"
)

// NewMux wires the API. Everything under /api/auth is reachable without a
// session (that's how you get one); every other /api route requires one.
func NewMux(cfg *config.Config, a *auth.Auth, shellMgr *shellsvc.Manager, execMgr *shellsvc.Manager) *http.ServeMux {
	mux := http.NewServeMux()
	api := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/status", a.StatusHandler())
	mux.HandleFunc("POST /api/auth/setup", a.SetupHandler())
	mux.HandleFunc("POST /api/auth/login", a.LoginHandler())
	mux.HandleFunc("POST /api/auth/logout", a.LogoutHandler())
	mux.Handle("/api/", a.Middleware(api))

	api.HandleFunc("GET /api/stacks", listStacks(cfg))
	api.HandleFunc("GET /api/stacks/{name}", getStack(cfg))
	api.HandleFunc("PUT /api/stacks/{name}", deployStack(cfg))
	api.HandleFunc("DELETE /api/stacks/{name}", deleteStack(cfg))
	api.HandleFunc("GET /api/stacks/{name}/services/{service}/logs", stackServiceLogs(cfg))

	api.HandleFunc("GET /api/quadlets", listQuadlets(cfg))
	api.HandleFunc("POST /api/quadlets", createQuadlet(cfg))
	api.HandleFunc("GET /api/quadlets/generator-logs", quadletGeneratorLogs())
	api.HandleFunc("GET /api/quadlets/{filename}", getQuadlet(cfg))
	api.HandleFunc("PUT /api/quadlets/{filename}", writeQuadlet(cfg))
	api.HandleFunc("DELETE /api/quadlets/{filename}", deleteQuadlet(cfg))
	api.HandleFunc("POST /api/quadlets/{filename}/start", startQuadlet())
	api.HandleFunc("POST /api/quadlets/{filename}/stop", stopQuadlet())
	api.HandleFunc("POST /api/quadlets/{filename}/restart", restartQuadlet())
	api.HandleFunc("GET /api/quadlets/{filename}/logs", quadletLogs())

	api.HandleFunc("GET /api/systemd", listSystemd(cfg))
	api.HandleFunc("GET /api/systemd/fs-suggestions", fsSuggestions())
	api.HandleFunc("POST /api/systemd", createSystemd(cfg))
	api.HandleFunc("GET /api/systemd/{name}/logs", systemdLogs())
	api.HandleFunc("GET /api/systemd/{name}/content", systemdContent())
	api.HandleFunc("PUT /api/systemd/{name}/content", writeSystemdContent())
	api.HandleFunc("PUT /api/systemd/{name}/favorite", setSystemdFavorite(cfg))
	api.HandleFunc("POST /api/systemd/{name}/start", startSystemd())
	api.HandleFunc("POST /api/systemd/{name}/stop", stopSystemd())
	api.HandleFunc("POST /api/systemd/{name}/restart", restartSystemd())
	api.HandleFunc("POST /api/systemd/{name}/enable", enableSystemd())
	api.HandleFunc("POST /api/systemd/{name}/disable", disableSystemd())
	api.HandleFunc("DELETE /api/systemd/{name}", deleteSystemd())

	api.HandleFunc("GET /api/containers", listContainers())
	api.HandleFunc("GET /api/containers/{id}/stats", containerStats())
	api.HandleFunc("GET /api/containers/{id}/logs", containerLogs())
	api.HandleFunc("POST /api/containers/{id}/start", startContainer())
	api.HandleFunc("POST /api/containers/{id}/stop", stopContainer())
	api.HandleFunc("POST /api/containers/{id}/restart", restartContainer())
	api.HandleFunc("DELETE /api/containers/{id}", removeContainer())
	api.HandleFunc("POST /api/containers/{id}/exec", createContainerExec(execMgr))
	api.HandleFunc("DELETE /api/containers/exec/{id}", closeShellSession(execMgr))
	api.HandleFunc("GET /api/containers/exec/{id}/ws", shellSessionWS(execMgr))

	api.HandleFunc("GET /api/images", listImages())
	api.HandleFunc("DELETE /api/images/{id}", deleteImage())
	api.HandleFunc("POST /api/images/prune", pruneImages())

	api.HandleFunc("GET /api/volumes", listVolumes())
	api.HandleFunc("GET /api/volumes/{name}", getVolume())
	api.HandleFunc("GET /api/volumes/{name}/fs/{path...}", getVolumeFs())
	api.HandleFunc("POST /api/volumes/{name}/fs/{path...}", createVolumeFsEntry())
	api.HandleFunc("PUT /api/volumes/{name}/fs/{path...}", writeVolumeFsEntry())
	api.HandleFunc("PATCH /api/volumes/{name}/fs/{path...}", moveVolumeFsEntry())
	api.HandleFunc("DELETE /api/volumes/{name}/fs/{path...}", deleteVolumeFsEntry())

	api.HandleFunc("GET /api/shell/sessions", listShellSessions(shellMgr))
	api.HandleFunc("POST /api/shell/sessions", createShellSession(shellMgr))
	api.HandleFunc("DELETE /api/shell/sessions/{id}", closeShellSession(shellMgr))
	api.HandleFunc("GET /api/shell/sessions/{id}/ws", shellSessionWS(shellMgr))

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func linesParam(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	return n
}

// --- stacks ---

func listStacks(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names, err := stacks.List(cfg)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		result := make([]*stacks.Status, 0, len(names))
		for _, name := range names {
			st, err := stacks.GetStatus(r.Context(), cfg, name)
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			result = append(result, st)
		}
		writeJSON(w, result)
	}
}

func getStack(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		content, err := stacks.Read(cfg, name)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		status, err := stacks.GetStatus(r.Context(), cfg, name)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		path := filepath.Join(cfg.StacksDir, name+".yml")
		writeJSON(w, map[string]any{"name": name, "content": content, "status": status, "path": path})
	}
}

func deployStack(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		var body struct {
			Content  string `json:"content"`
			Force    bool   `json:"force"`
			Pull     bool   `json:"pull"`
			IsCreate bool   `json:"isCreate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := stacks.Deploy(r.Context(), cfg, name, body.Content, body.Force, body.Pull, body.IsCreate); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deployed"})
	}
}

func deleteStack(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		q := r.URL.Query()
		opts := stacks.DeleteOptions{
			Stack:   q.Get("stack") == "true",
			Quadlet: q.Get("quadlet") == "true",
			Images:  q.Get("images") == "true",
			Volumes: q.Get("volumes") == "true",
		}
		if err := stacks.Delete(r.Context(), cfg, name, opts); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

func stackServiceLogs(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		service := r.PathValue("service")
		unit := quadlets.UnitName(name + "-" + service + ".container")
		out, err := podmanx.UnitLogs(r.Context(), unit, linesParam(r))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"logs": out})
	}
}

// --- quadlets ---

func listQuadlets(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := quadlets.List(r.Context(), cfg.QuadletDir)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, files)
	}
}

func getQuadlet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.PathValue("filename")
		content, err := quadlets.Read(cfg.QuadletDir, filename)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, map[string]string{"content": content, "path": filepath.Join(cfg.QuadletDir, filename)})
	}
}

func createQuadlet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := quadlets.Create(r.Context(), cfg.QuadletDir, body.Filename, body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "created"})
	}
}

func writeQuadlet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		filename := r.PathValue("filename")
		if err := quadlets.Validate(r.Context(), cfg.QuadletDir, filename, body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		if err := quadlets.Write(r.Context(), cfg.QuadletDir, filename, body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	}
}

func deleteQuadlet(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := quadlets.Delete(r.Context(), cfg.QuadletDir, r.PathValue("filename")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

func startQuadlet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := quadlets.Start(r.Context(), r.PathValue("filename")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "started"})
	}
}

func stopQuadlet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := quadlets.Stop(r.Context(), r.PathValue("filename")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "stopped"})
	}
}

func restartQuadlet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := quadlets.Restart(r.Context(), r.PathValue("filename")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "restarted"})
	}
}

func quadletLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unit := quadlets.UnitName(r.PathValue("filename"))
		out, err := podmanx.UnitLogs(r.Context(), unit, linesParam(r))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"logs": out})
	}
}

func quadletGeneratorLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := podmanx.GeneratorLogs(r.Context(), linesParam(r))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"logs": out})
	}
}

// --- systemd ---

func listSystemd(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		favs, err := favorites.Load(cfg.FavoritesFile)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		opts := sysdunits.ListOptions{
			All:      r.URL.Query().Get("all") == "true",
			Quadlet:  r.URL.Query().Get("quadlet") == "true",
			Favorite: r.URL.Query().Get("favorite") == "true",
		}
		units, err := sysdunits.List(r.Context(), cfg.QuadletDir, favs, opts)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, units)
	}
}

func fsSuggestions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		dirsOnly := r.URL.Query().Get("dirsOnly") == "true"
		writeJSON(w, sysdunits.ListFsSuggestions(path, dirsOnly))
	}
}

func createSystemd(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := sysdunits.Create(r.Context(), cfg.SystemdUnitDir, body.Filename, body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "created"})
	}
}

func setSystemdFavorite(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Favorite bool `json:"favorite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := favorites.Set(cfg.FavoritesFile, r.PathValue("name"), body.Favorite); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]bool{"favorite": body.Favorite})
	}
}

func systemdLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := podmanx.UnitLogs(r.Context(), r.PathValue("name"), linesParam(r))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"logs": out})
	}
}

func systemdContent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := sysdunits.Content(r.Context(), r.PathValue("name"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"content": out})
	}
}

func writeSystemdContent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := sysdunits.WriteContent(r.Context(), r.PathValue("name"), body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	}
}

func startSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Start(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "started"})
	}
}

func stopSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Stop(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "stopped"})
	}
}

func restartSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Restart(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "restarted"})
	}
}

func enableSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Enable(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "enabled"})
	}
}

func disableSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Disable(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "disabled"})
	}
}

func deleteSystemd() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := sysdunits.Delete(r.Context(), r.PathValue("name")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

// --- containers ---

func listContainers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		containers, err := podmanx.List(r.Context())
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, containers)
	}
}

func containerStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := podmanx.Stats(r.Context(), r.PathValue("id"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, stats)
	}
}

func containerLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := podmanx.ContainerLogs(r.Context(), r.PathValue("id"), linesParam(r))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"logs": out})
	}
}

func startContainer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := podmanx.Start(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "started"})
	}
}

func stopContainer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := podmanx.Stop(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "stopped"})
	}
}

func restartContainer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := podmanx.Restart(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "restarted"})
	}
}

func removeContainer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := podmanx.Remove(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "removed"})
	}
}

func createContainerExec(mgr *shellsvc.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := mgr.CreateExec(r.PathValue("id"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, shellSessionRow{ID: s.ID, CreatedAt: s.CreatedAt})
	}
}

// --- images ---

// imageRow is the images list's response shape: each image row carries the
// containers that reference its ID pre-joined, since the page has no detail
// view to fetch that separately.
type imageRow struct {
	ID         string              `json:"id"`
	Repository string              `json:"repository"`
	Tag        string              `json:"tag"`
	Size       int64               `json:"size"`
	CreatedAt  string              `json:"createdAt"`
	Containers []podmanx.Container `json:"containers"`
}

func listImages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		imgs, err := images.List(r.Context())
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		containers, err := podmanx.List(r.Context())
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		byImageID := map[string][]podmanx.Container{}
		for _, c := range containers {
			byImageID[c.ImageID] = append(byImageID[c.ImageID], c)
		}
		rows := make([]imageRow, 0, len(imgs))
		for _, img := range imgs {
			imgContainers := byImageID[img.ID]
			if imgContainers == nil {
				imgContainers = []podmanx.Container{}
			}
			rows = append(rows, imageRow{
				ID:         img.ID,
				Repository: img.Repository,
				Tag:        img.Tag,
				Size:       img.Size,
				CreatedAt:  img.CreatedAt,
				Containers: imgContainers,
			})
		}
		writeJSON(w, rows)
	}
}

func deleteImage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := images.Delete(r.Context(), r.PathValue("id")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

func pruneImages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := r.URL.Query().Get("all") == "1"
		if err := images.Prune(r.Context(), all); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "pruned"})
	}
}

// --- volumes ---

func listVolumes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vols, err := volumes.List(r.Context())
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, vols)
	}
}

func getVolume() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		vol, err := volumes.Inspect(r.Context(), name)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		containers, err := podmanx.ListByVolume(r.Context(), name)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]any{"volume": vol, "containers": containers})
	}
}

// volumeMountpoint is the shared first step for every file-browsing
// endpoint below: resolve {name} to the host path its files actually live
// under before touching {path}.
func volumeMountpoint(r *http.Request) (string, error) {
	vol, err := volumes.Inspect(r.Context(), r.PathValue("name"))
	if err != nil {
		return "", err
	}
	return vol.Mountpoint, nil
}

// getVolumeFs serves both directory listings and file reads/downloads off a
// single path, since the caller already knows which one it clicked on and
// the response shape says so anyway via "isDir".
func getVolumeFs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mp, err := volumeMountpoint(r)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		path := r.PathValue("path")
		isDir, size, err := volumes.Stat(r.Context(), mp, path)
		if err != nil {
			writeErr(w, 404, err)
			return
		}

		if r.URL.Query().Get("download") == "1" {
			if isDir {
				writeErr(w, 400, fmt.Errorf("cannot download a directory"))
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeHeaderFilename(filepath.Base(path))+`"`)
			_ = volumes.StreamDownload(r.Context(), w, mp, path)
			return
		}

		if isDir {
			entries, err := volumes.ListDir(r.Context(), mp, path)
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			writeJSON(w, map[string]any{"isDir": true, "entries": entries})
			return
		}

		content, err := volumes.ReadFile(r.Context(), mp, path)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, map[string]any{"isDir": false, "size": size, "content": content})
	}
}

// sanitizeHeaderFilename strips characters that would break out of the
// quoted filename in a Content-Disposition header.
func sanitizeHeaderFilename(name string) string {
	name = strings.ReplaceAll(name, `"`, "'")
	return strings.Map(func(r rune) rune {
		if r < 0x20 {
			return -1
		}
		return r
	}, name)
}

// createVolumeFsEntry either uploads a raw file body (?upload=1) or creates
// an empty file/dir per the JSON "type" field.
func createVolumeFsEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mp, err := volumeMountpoint(r)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		path := r.PathValue("path")

		if r.URL.Query().Get("upload") == "1" {
			if err := volumes.UploadFile(r.Context(), r.Body, mp, path); err != nil {
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
		switch body.Type {
		case "file":
			err = volumes.CreateFile(r.Context(), mp, path)
		case "dir":
			err = volumes.Mkdir(r.Context(), mp, path)
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

func writeVolumeFsEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		mp, err := volumeMountpoint(r)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		if err := volumes.WriteFile(r.Context(), mp, r.PathValue("path"), body.Content); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "saved"})
	}
}

// moveVolumeFsEntry handles both move and copy; a rename is just a move to
// a destination path that shares the same parent directory.
func moveVolumeFsEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Dest   string `json:"dest"`
			IsCopy bool   `json:"isCopy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		mp, err := volumeMountpoint(r)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		if err := volumes.Move(r.Context(), mp, r.PathValue("path"), body.Dest, body.IsCopy); err != nil {
			writeErr(w, 500, err)
			return
		}
		status := "moved"
		if body.IsCopy {
			status = "copied"
		}
		writeJSON(w, map[string]string{"status": status})
	}
}

func deleteVolumeFsEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mp, err := volumeMountpoint(r)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		if err := volumes.Delete(r.Context(), mp, r.PathValue("path")); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

// --- shell ---

type shellSessionRow struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

func listShellSessions(mgr *shellsvc.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessions := mgr.List()
		rows := make([]shellSessionRow, 0, len(sessions))
		for _, s := range sessions {
			rows = append(rows, shellSessionRow{ID: s.ID, CreatedAt: s.CreatedAt})
		}
		writeJSON(w, rows)
	}
}

func createShellSession(mgr *shellsvc.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := mgr.Create()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, shellSessionRow{ID: s.ID, CreatedAt: s.CreatedAt})
	}
}

func closeShellSession(mgr *shellsvc.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mgr.Close(r.PathValue("id")) {
			writeErr(w, 404, fmt.Errorf("session not found"))
			return
		}
		writeJSON(w, map[string]string{"status": "closed"})
	}
}

// shellSessionWS upgrades to a websocket carrying raw PTY bytes to the
// client (binary frames) and JSON control messages from the client (input
// keystrokes, terminal resizes).
func shellSessionWS(mgr *shellsvc.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, ok := mgr.Get(r.PathValue("id"))
		if !ok {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()
		s.Attach(ctx, conn)
		defer s.Detach(conn)

		for {
			typ, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if typ != websocket.MessageText {
				continue
			}
			var msg struct {
				Type string `json:"type"`
				Data string `json:"data"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				s.Write([]byte(msg.Data))
			case "resize":
				s.Resize(msg.Rows, msg.Cols)
			}
		}
	}
}
