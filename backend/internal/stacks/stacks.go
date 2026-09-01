// Package stacks implements Podtainer's compose-file-backed stack lifecycle:
// save+deploy, redeploy diffing, delete, pull, and live status/drift, all
// derived from disk and systemd/podman rather than any cached model.
package stacks

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"podtainer/internal/composeutil"
	"podtainer/internal/config"
	"podtainer/internal/execx"
	"podtainer/internal/quadletgen"
	"podtainer/internal/quadlets"
)

type UnitStatus struct {
	Filename string `json:"filename"`
	Active   string `json:"active"`
	Health   string `json:"health"`
}

type Status struct {
	Name     string       `json:"name"`
	Deployed bool         `json:"deployed"`
	Drift    bool         `json:"drift"`
	Units    []UnitStatus `json:"units"`
}

func composePath(cfg *config.Config, name string) string {
	return filepath.Join(cfg.StacksDir, name+".yml")
}

// List returns stack names derived from *.yml files in the stacks dir.
func List(cfg *config.Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.StacksDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".yml"))
	}
	sort.Strings(names)
	return names, nil
}

func Read(cfg *config.Config, name string) (string, error) {
	b, err := os.ReadFile(composePath(cfg, name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// dirFor returns the directory a generated file belongs in: .target files
// are plain systemd units and must live in systemd's own user unit
// directory, everything else is a real quadlet type and lives in the
// quadlet search path.
func dirFor(cfg *config.Config, filename string) string {
	if strings.HasSuffix(filename, ".target") {
		return cfg.TargetDir
	}
	return cfg.QuadletDir
}

type installedFile struct {
	dir     string
	content string
}

// installedFiles returns the on-disk unit files owned by this stack, keyed
// by filename, identified purely by the stack-prefix naming convention,
// across both the quadlet directory and systemd's user unit directory. If a
// stack-prefixed file turns up in the "wrong" one of those two directories
// (e.g. left over from a bug, or a manual mistake), it's still reported here
// so Deploy can clean it up rather than silently ignoring it.
func installedFiles(cfg *config.Config, name string) (map[string]installedFile, error) {
	out := map[string]installedFile{}
	dashPrefix := name + "-"
	dotPrefix := name + "."

	for _, dir := range []string{cfg.QuadletDir, cfg.TargetDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if !strings.HasPrefix(n, dashPrefix) && !strings.HasPrefix(n, dotPrefix) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, n))
			if err != nil {
				return nil, err
			}
			out[n] = installedFile{dir: dir, content: string(b)}
		}
	}
	return out, nil
}

// Deploy validates and writes the compose file, regenerates its quadlet
// units, and applies a pure filesystem diff against the stack's existing
// prefixed files: removed files are stopped+disabled+deleted, new/changed
// ones are written and (re)started. force rewrites+restarts every unit
// unconditionally even if content is unchanged.
func Deploy(ctx context.Context, cfg *config.Config, name, content string, force bool) error {
	if err := composeutil.ValidateName(name); err != nil {
		return err
	}
	if err := composeutil.Validate([]byte(content)); err != nil {
		return err
	}

	path := composePath(cfg, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}

	generated, err := quadletgen.Generate(ctx, name, path)
	if err != nil {
		return err
	}
	genMap := map[string]string{}
	for _, u := range generated {
		genMap[u.Filename] = u.Content
	}

	existing, err := installedFiles(cfg, name)
	if err != nil {
		return err
	}

	var toRemove, toWrite []string
	for filename, old := range existing {
		_, stillWanted := genMap[filename]
		if !stillWanted {
			toRemove = append(toRemove, filename)
			continue
		}
		// A stack-prefixed file sitting in the wrong directory (e.g. a
		// .target that ended up in the quadlet dir) needs to be removed
		// from there even though the filename is still wanted overall.
		if old.dir != dirFor(cfg, filename) {
			toRemove = append(toRemove, filename)
		}
	}
	for filename, newContent := range genMap {
		old, existed := existing[filename]
		wrongDir := existed && old.dir != dirFor(cfg, filename)
		if force || !existed || wrongDir || old.content != newContent {
			toWrite = append(toWrite, filename)
		}
	}

	for _, filename := range orderForRemoval(toRemove) {
		if err := quadlets.RemoveFile(ctx, existing[filename].dir, filename); err != nil {
			return err
		}
	}

	for _, filename := range toWrite {
		if err := os.WriteFile(filepath.Join(dirFor(cfg, filename), filename), []byte(genMap[filename]), 0o644); err != nil {
			return err
		}
	}

	if _, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}

	if _, err := execx.Run(ctx, "systemctl", "--user", "start", name+".target"); err != nil {
		return err
	}

	// Units that already existed and merely changed content need an
	// explicit restart: starting an already-active unit is a no-op and
	// won't pick up the new file.
	for _, filename := range toWrite {
		if _, existed := existing[filename]; existed && strings.HasSuffix(filename, ".container") {
			if _, err := execx.Run(ctx, "systemctl", "--user", "restart", quadlets.UnitName(filename)); err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete stops and removes every unit file owned by this stack (matched by
// filename prefix) and its compose source. Named volumes are only ever
// referenced by a .volume unit file here, which is included in this delete
// like any other unit — the underlying `podman volume` data is untouched.
func Delete(ctx context.Context, cfg *config.Config, name string) error {
	// Best-effort: the target may already be stopped, failed, or gone
	// entirely (e.g. cleaning up after a previous partial failure), none
	// of which should block removing what's actually left.
	_, _ = execx.Run(ctx, "systemctl", "--user", "stop", name+".target")

	existing, err := installedFiles(cfg, name)
	if err != nil {
		return err
	}
	filenames := make([]string, 0, len(existing))
	for filename := range existing {
		filenames = append(filenames, filename)
	}
	for _, filename := range orderForRemoval(filenames) {
		if err := quadlets.RemoveFile(ctx, existing[filename].dir, filename); err != nil {
			return err
		}
	}

	if err := os.Remove(composePath(cfg, name)); err != nil && !os.IsNotExist(err) {
		return err
	}

	_, err = execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}

// orderForRemoval sorts filenames so containers are torn down before the
// networks they're attached to, and everything else (volumes, targets)
// last — `podman network rm` fails while a container still references the
// network.
func orderForRemoval(filenames []string) []string {
	rank := func(f string) int {
		switch {
		case strings.HasSuffix(f, ".container"):
			return 0
		case strings.HasSuffix(f, ".network"):
			return 1
		default:
			return 2
		}
	}
	sorted := append([]string(nil), filenames...)
	sort.SliceStable(sorted, func(i, j int) bool { return rank(sorted[i]) < rank(sorted[j]) })
	return sorted
}

// PullAndRestart pulls every image referenced by the stack's compose file
// and restarts the whole stack via its target unit.
func PullAndRestart(ctx context.Context, cfg *config.Config, name string) error {
	content, err := Read(cfg, name)
	if err != nil {
		return err
	}
	images, err := composeutil.ServiceImages([]byte(content))
	if err != nil {
		return err
	}
	for _, image := range images {
		if _, err := execx.RunLong(ctx, 5*time.Minute, "podman", "pull", image); err != nil {
			return err
		}
	}
	_, err = execx.Run(ctx, "systemctl", "--user", "restart", name+".target")
	return err
}

// GetStatus reports deployment/drift state plus per-unit systemd/health
// status for one stack, all computed live.
func GetStatus(ctx context.Context, cfg *config.Config, name string) (*Status, error) {
	existing, err := installedFiles(cfg, name)
	if err != nil {
		return nil, err
	}

	st := &Status{Name: name, Deployed: len(existing) > 0, Units: []UnitStatus{}}

	if _, err := os.Stat(composePath(cfg, name)); err == nil {
		if generated, err := quadletgen.Generate(ctx, name, composePath(cfg, name)); err == nil {
			genMap := map[string]string{}
			for _, u := range generated {
				genMap[u.Filename] = u.Content
			}
			st.Drift = !filesEqual(cfg, existing, genMap)
		}
	}

	filenames := make([]string, 0, len(existing))
	for f := range existing {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		us := UnitStatus{Filename: filename, Active: "unknown", Health: "none"}
		unit := quadlets.UnitName(filename)
		if out, err := execx.Run(ctx, "systemctl", "--user", "show", unit, "--property=ActiveState", "--value"); err == nil {
			us.Active = strings.TrimSpace(out)
		}
		if strings.HasSuffix(filename, ".container") {
			container := strings.TrimSuffix(filename, ".container")
			if out, err := execx.Run(ctx, "podman", "inspect", container, "--format", "{{.State.Health.Status}}"); err == nil {
				h := strings.TrimSpace(out)
				if h != "" && h != "<no value>" {
					us.Health = h
				}
			}
		}
		st.Units = append(st.Units, us)
	}

	return st, nil
}

func filesEqual(cfg *config.Config, a map[string]installedFile, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v.content || v.dir != dirFor(cfg, k) {
			return false
		}
	}
	return true
}
