// Package stacks implements Podtainer's compose-file-backed stack lifecycle:
// save+deploy, redeploy diffing, delete, pull, and live status/drift, all
// derived from disk and systemd/podman rather than any cached model.
package stacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"podtainer/internal/composeutil"
	"podtainer/internal/config"
	"podtainer/internal/execx"
	"podtainer/internal/quadletgen"
	"podtainer/internal/quadlets"
)

// systemdTimestampLayout matches the human-readable timestamps emitted by
// `systemctl show` (e.g. "Wed 2026-09-02 19:17:48 CDT").
const systemdTimestampLayout = "Mon 2006-01-02 15:04:05 MST"

func parseProperties(s string) map[string]string {
	vals := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			vals[key] = value
		}
	}
	return vals
}

type QuadletUnitStatus struct {
	Filename       string `json:"filename"`
	Active         string `json:"active"`
	Sub            string `json:"sub"`
	NRestarts      int    `json:"nRestarts"`
	SinceTimestamp string `json:"sinceTimestamp,omitempty"`
}

type SystemdUnitStatus struct {
	Unit           string `json:"unit"`
	Active         string `json:"active"`
	Sub            string `json:"sub"`
	NRestarts      int    `json:"nRestarts"`
	SinceTimestamp string `json:"sinceTimestamp,omitempty"`
}

type ContainerStatus struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
}

type Status struct {
	Name             string              `json:"name"`
	Deployed         bool                `json:"deployed"`
	Drift            bool                `json:"drift"`
	QuadletUnits     []QuadletUnitStatus `json:"quadletUnits"`
	SystemdUnits     []SystemdUnitStatus `json:"systemdUnits"`
	PodmanContainers []ContainerStatus   `json:"podmanContainers"`
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

// installedFiles returns the on-disk quadlet files owned by this stack,
// keyed by filename to content, identified purely by the stack-prefix
// naming convention.
func installedFiles(cfg *config.Config, name string) (map[string]string, error) {
	out := map[string]string{}
	prefix := name + "-"

	entries, err := os.ReadDir(cfg.QuadletDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(cfg.QuadletDir, e.Name()))
		if err != nil {
			return nil, err
		}
		out[e.Name()] = string(b)
	}
	return out, nil
}

// Deploy validates and writes the compose file, regenerates its quadlet
// units, and applies a pure filesystem diff against the stack's existing
// prefixed files: removed files are stopped+disabled+deleted, new/changed
// ones are written and (re)started. force rewrites+restarts every unit
// unconditionally even if content is unchanged. pull re-pulls every image
// the new content references before any of that, and requires force (a
// pulled image only matters if every container unit actually restarts).
// isCreate additionally rejects a name already claimed by an existing
// compose file or by orphaned quadlet files sharing its "{name}-" prefix.
//
// Once the compose file and quadlet units are committed to disk, a unit
// that fails to start/restart (e.g. a port conflict) does not abort the
// deploy: the stack itself was created successfully, and its per-unit
// status is visible on the stack details page.
func Deploy(ctx context.Context, cfg *config.Config, name, content string, force, pull, isCreate bool) error {
	if err := composeutil.ValidateName(name); err != nil {
		return err
	}
	if pull && !force {
		return fmt.Errorf("re-pulling images requires redeploying unchanged systemd services too")
	}
	if err := composeutil.Validate([]byte(content)); err != nil {
		return err
	}

	path := composePath(cfg, name)

	if isCreate {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("a stack named %q already exists", name)
		}
		existing, err := installedFiles(cfg, name)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			return fmt.Errorf("quadlet files already exist with the %q- prefix; delete them or choose a different name", name)
		}
	}

	if pull {
		imgs, err := composeutil.ServiceImages([]byte(content))
		if err != nil {
			return err
		}
		for _, image := range imgs {
			if _, err := execx.RunLong(ctx, 5*time.Minute, "podman", "pull", image); err != nil {
				return err
			}
		}
	}

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
	for filename := range existing {
		if _, stillWanted := genMap[filename]; !stillWanted {
			toRemove = append(toRemove, filename)
		}
	}
	for filename, newContent := range genMap {
		oldContent, existed := existing[filename]
		if force || !existed || oldContent != newContent {
			toWrite = append(toWrite, filename)
		}
	}

	for _, filename := range orderForRemoval(toRemove) {
		if err := quadlets.RemoveFile(ctx, cfg.QuadletDir, filename); err != nil {
			return err
		}
	}

	for _, filename := range toWrite {
		if err := os.WriteFile(filepath.Join(cfg.QuadletDir, filename), []byte(genMap[filename]), 0o644); err != nil {
			return err
		}
	}

	if _, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}

	// Each container carries its own [Install] WantedBy=default.target, so
	// starting it here is what brings it up now that there's no stack
	// target to pull the whole set in at once.
	for _, u := range generated {
		if !strings.HasSuffix(u.Filename, ".container") {
			continue
		}
		execx.Run(ctx, "systemctl", "--user", "start", quadlets.UnitName(u.Filename))
	}

	// Units that already existed and merely changed content need an
	// explicit restart: starting an already-active unit is a no-op and
	// won't pick up the new file.
	for _, filename := range toWrite {
		if _, existed := existing[filename]; existed && strings.HasSuffix(filename, ".container") {
			execx.Run(ctx, "systemctl", "--user", "restart", quadlets.UnitName(filename))
		}
	}

	return nil
}

// DeleteOptions selects which parts of a stack Delete tears down. Stack and
// Quadlet are independent — deleting the compose file while leaving the
// quadlet units in place is the intended way to hand a stack off to
// unmanaged, hand-edited quadlet files. Images/Volumes do require Quadlet
// (both only make sense alongside removing the units that reference them) —
// callers must enforce that dependency themselves, since Delete rejects a
// mismatch rather than silently coercing it.
type DeleteOptions struct {
	Stack   bool
	Quadlet bool
	Images  bool
	Volumes bool
}

// Delete tears down the parts of a stack selected by opts. Quadlet removal
// stops+disables+deletes every unit file owned by this stack (matched by
// filename prefix); Volumes additionally removes the actual `podman volume`
// data those units referenced (normally preserved). Images removes every
// image the stack's compose file references, except ones still referenced
// by another stack's compose file.
func Delete(ctx context.Context, cfg *config.Config, name string, opts DeleteOptions) error {
	if (opts.Images || opts.Volumes) && !opts.Quadlet {
		return fmt.Errorf("deleting images or volumes requires also deleting quadlet files")
	}

	var content string
	if opts.Images {
		c, err := Read(cfg, name)
		if err != nil {
			return err
		}
		content = c
	}

	existing, err := installedFiles(cfg, name)
	if err != nil {
		return err
	}

	if opts.Quadlet {
		filenames := make([]string, 0, len(existing))
		for filename := range existing {
			filenames = append(filenames, filename)
		}
		for _, filename := range orderForRemoval(filenames) {
			if err := quadlets.RemoveFile(ctx, cfg.QuadletDir, filename); err != nil {
				return err
			}
		}

		if opts.Volumes {
			for filename := range existing {
				if !strings.HasSuffix(filename, ".volume") {
					continue
				}
				base := strings.TrimSuffix(filename, ".volume")
				execx.Run(ctx, "podman", "volume", "rm", "systemd-"+base)
			}
		}
	}

	if opts.Stack {
		if err := os.Remove(composePath(cfg, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if opts.Quadlet {
		if _, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
	}

	if opts.Images {
		imgs, err := composeutil.ServiceImages([]byte(content))
		if err != nil {
			return err
		}
		inUseElsewhere, err := imagesUsedByOtherStacks(cfg, name)
		if err != nil {
			return err
		}
		for _, image := range imgs {
			if inUseElsewhere[image] {
				continue
			}
			execx.Run(ctx, "podman", "rmi", image)
		}
	}

	return nil
}

// imagesUsedByOtherStacks returns the set of image refs referenced by any
// stack other than except, so Delete's image cleanup never removes an image
// a sibling stack still depends on.
func imagesUsedByOtherStacks(cfg *config.Config, except string) (map[string]bool, error) {
	names, err := List(cfg)
	if err != nil {
		return nil, err
	}
	inUse := map[string]bool{}
	for _, name := range names {
		if name == except {
			continue
		}
		content, err := Read(cfg, name)
		if err != nil {
			continue
		}
		imgs, err := composeutil.ServiceImages([]byte(content))
		if err != nil {
			continue
		}
		for _, image := range imgs {
			inUse[image] = true
		}
	}
	return inUse, nil
}

// orderForRemoval sorts filenames so containers are torn down before the
// networks they're attached to, and everything else (volumes) last —
// `podman network rm` fails while a container still references the network.
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

// GetStatus reports deployment/drift state plus per-unit systemd/health
// status for one stack, all computed live.
func GetStatus(ctx context.Context, cfg *config.Config, name string) (*Status, error) {
	existing, err := installedFiles(cfg, name)
	if err != nil {
		return nil, err
	}

	st := &Status{Name: name, Deployed: len(existing) > 0, PodmanContainers: []ContainerStatus{}}

	if _, err := os.Stat(composePath(cfg, name)); err == nil {
		if generated, err := quadletgen.Generate(ctx, name, composePath(cfg, name)); err == nil {
			genMap := map[string]string{}
			for _, u := range generated {
				genMap[u.Filename] = u.Content
			}
			st.Drift = !filesEqual(existing, genMap)
		}
	}

	filenames := make([]string, 0, len(existing))
	for f := range existing {
		filenames = append(filenames, f)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		unit := quadlets.UnitName(filename)
		active := "unknown"
		var sub, since string
		var nRestarts int
		if out, err := execx.Run(ctx, "systemctl", "--user", "show", unit,
			"--property=ActiveState,SubState,NRestarts,ConditionTimestamp"); err == nil {
			vals := parseProperties(out)
			active = vals["ActiveState"]
			sub = vals["SubState"]
			nRestarts, _ = strconv.Atoi(vals["NRestarts"])
			if t, err := time.Parse(systemdTimestampLayout, vals["ConditionTimestamp"]); err == nil {
				since = t.Format(time.RFC3339)
			}
		}
		st.QuadletUnits = append(st.QuadletUnits, QuadletUnitStatus{Filename: filename, Active: active, Sub: sub, NRestarts: nRestarts, SinceTimestamp: since})
		st.SystemdUnits = append(st.SystemdUnits, SystemdUnitStatus{Unit: unit, Active: active, Sub: sub, NRestarts: nRestarts, SinceTimestamp: since})

		if strings.HasSuffix(filename, ".container") {
			container := strings.TrimSuffix(filename, ".container")
			cs := ContainerStatus{Name: container, State: "unknown", Health: "none"}
			out, err := execx.Run(ctx, "podman", "inspect", container, "--format", "{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}")
			if err == nil {
				parts := strings.SplitN(strings.TrimSpace(out), "|", 2)
				if parts[0] != "" {
					cs.State = parts[0]
				}
				if len(parts) == 2 && parts[1] != "" && parts[1] != "<no value>" {
					cs.Health = parts[1]
				}
			} else if sub == "auto-restart" {
				// The container is torn down between crash-loop restarts, so
				// `podman inspect` finds nothing even though the unit is very
				// much not idle — report the systemd-derived state instead of
				// "unknown".
				cs.State = "missing"
			}
			st.PodmanContainers = append(st.PodmanContainers, cs)
		}
	}

	return st, nil
}

func filesEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
