// Package api wires the HTTP surface for Podtainer's four views: stacks,
// quadlets, systemd units, and podman containers.
package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"podtainer/internal/config"
	"podtainer/internal/podmanx"
	"podtainer/internal/quadlets"
	"podtainer/internal/stacks"
	"podtainer/internal/sysdunits"
)

func NewMux(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/stacks", listStacks(cfg))
	mux.HandleFunc("GET /api/stacks/{name}", getStack(cfg))
	mux.HandleFunc("PUT /api/stacks/{name}", deployStack(cfg))
	mux.HandleFunc("DELETE /api/stacks/{name}", deleteStack(cfg))
	mux.HandleFunc("POST /api/stacks/{name}/pull", pullStack(cfg))
	mux.HandleFunc("GET /api/stacks/{name}/services/{service}/logs", stackServiceLogs(cfg))

	mux.HandleFunc("GET /api/quadlets", listQuadlets(cfg))
	mux.HandleFunc("GET /api/quadlets/generator-logs", quadletGeneratorLogs())
	mux.HandleFunc("GET /api/quadlets/{filename}", getQuadlet(cfg))
	mux.HandleFunc("PUT /api/quadlets/{filename}", writeQuadlet(cfg))
	mux.HandleFunc("DELETE /api/quadlets/{filename}", deleteQuadlet(cfg))
	mux.HandleFunc("POST /api/quadlets/{filename}/start", startQuadlet())
	mux.HandleFunc("POST /api/quadlets/{filename}/stop", stopQuadlet())
	mux.HandleFunc("POST /api/quadlets/{filename}/restart", restartQuadlet())
	mux.HandleFunc("GET /api/quadlets/{filename}/logs", quadletLogs())

	mux.HandleFunc("GET /api/systemd", listSystemd(cfg))
	mux.HandleFunc("GET /api/systemd/{name}/logs", systemdLogs())
	mux.HandleFunc("GET /api/systemd/{name}/content", systemdContent())

	mux.HandleFunc("GET /api/containers", listContainers())
	mux.HandleFunc("GET /api/containers/{id}/stats", containerStats())
	mux.HandleFunc("GET /api/containers/{id}/logs", containerLogs())
	mux.HandleFunc("POST /api/containers/{id}/start", startContainer())
	mux.HandleFunc("POST /api/containers/{id}/stop", stopContainer())
	mux.HandleFunc("POST /api/containers/{id}/restart", restartContainer())
	mux.HandleFunc("DELETE /api/containers/{id}", removeContainer())

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
			Content string `json:"content"`
			Force   bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := stacks.Deploy(r.Context(), cfg, name, body.Content, body.Force); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deployed"})
	}
}

func deleteStack(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := stacks.Delete(r.Context(), cfg, name); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	}
}

func pullStack(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := stacks.PullAndRestart(r.Context(), cfg, name); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, map[string]string{"status": "pulled"})
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
		units, err := sysdunits.List(r.Context(), cfg.QuadletDir)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, units)
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
