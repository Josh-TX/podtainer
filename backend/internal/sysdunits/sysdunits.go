// Package sysdunits lists systemd --user units, categorized as quadlet
// (identified via each unit's SourcePath rather than any Podtainer-specific
// naming or label, so hand-authored quadlets outside Podtainer show up too),
// favorite (user-starred, see the favorites package), or neither.
package sysdunits

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"podtainer/internal/execx"
	"podtainer/internal/quadlets"
)

// systemdTimestampLayout matches the human-readable timestamps emitted by
// `systemctl show` (e.g. "Wed 2026-09-02 19:17:48 CDT").
const systemdTimestampLayout = "Mon 2006-01-02 15:04:05 MST"

type Unit struct {
	Name        string `json:"name"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
	SourcePath  string `json:"sourcePath"`
	// FragmentPath is the actual unit file systemd loaded, which for
	// quadlet-generated units lives under the generator's runtime directory
	// (e.g. /run/user/<uid>/systemd/generator/), distinct from SourcePath
	// (the quadlet file that produced it).
	FragmentPath string `json:"fragmentPath"`
	NRestarts    int    `json:"nRestarts"`
	IsQuadlet    bool   `json:"isQuadlet"`
	IsFavorite   bool   `json:"isFavorite"`
	// SinceTimestamp is when the unit's current run began (ConditionTimestamp),
	// which stays fixed across auto-restart cycles, so it doubles as "failing since"
	// for a unit stuck in the activating/auto-restart loop.
	SinceTimestamp string `json:"sinceTimestamp,omitempty"`
	// UnitFileState is systemctl's enabled/disabled/static/masked/etc.
	// classification, empty for units with no backing unit file entry
	// (e.g. orphans).
	UnitFileState string `json:"unitFileState"`
	// IsEditable is true for units backed by a real file outside /run -
	// units generated at runtime (quadlet-generated .service files, in
	// particular) live under /run and get overwritten on the next
	// daemon-reload, so editing them directly is pointless.
	IsEditable bool `json:"isEditable"`
}

const maxFsSuggestions = 30

// ListFsSuggestions lists directory entries under the directory portion of
// path whose name has the (still-being-typed) basename as a prefix, for use
// as ExecStart/WorkingDirectory typeahead suggestions. Directories are
// returned with a trailing slash so a further keystroke can drill into them.
// If dirsOnly is set, files are excluded (used for WorkingDirectory).
func ListFsSuggestions(path string, dirsOnly bool) []string {
	dir, prefix := path, ""
	if !strings.HasSuffix(path, "/") {
		dir, prefix = filepath.Dir(path), filepath.Base(path)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}

	dirs := []string{}
	files := []string{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			dirs = append(dirs, full+"/")
			continue
		}
		if dirsOnly {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Mode()&0111 == 0 {
			continue
		}
		files = append(files, full)
	}
	sort.Strings(dirs)
	sort.Strings(files)

	result := append(dirs, files...)
	if len(result) > maxFsSuggestions {
		result = result[:maxFsSuggestions]
	}
	return result
}

// ListOptions selects which categories of unit are returned by List. All, if
// set, overrides Quadlet/Favorite and returns every systemd --user unit.
type ListOptions struct {
	All      bool
	Quadlet  bool
	Favorite bool
}

type rawUnitFile struct {
	UnitFile string `json:"unit_file"`
	State    string `json:"state"`
}

type rawUnit struct {
	Unit string `json:"unit"`
}

// List returns systemd --user units matching opts. A unit is considered
// quadlet-origin if its SourcePath lives under quadletDir (or, for
// "orphaned" units - see below - if its name matches a quadlet file's
// expected unit name), and favorite if its name is in favorites. With
// opts.All every unit is returned regardless of category; otherwise only
// units matching an enabled category (Quadlet/Favorite) are returned. Every
// returned unit still carries accurate IsQuadlet/IsFavorite flags.
//
// list-unit-files alone would miss "orphaned" units: ones systemd still has
// loaded and running (or failed) in memory even though their backing file is
// gone, e.g. because quadlet-generator failed to regenerate it on the last
// daemon-reload (a syntax error in the source file, most commonly) without
// stopping the previously-running instance. list-units catches those, but
// also includes every other unit on the system, so orphan candidates are
// only admitted if their name matches a quadlet file currently present in
// quadletDir - systemd clears SourcePath/FragmentPath for them once orphaned,
// so that's the only way to attribute them back to a quadlet file.
func List(ctx context.Context, quadletDir string, favorites map[string]bool, opts ListOptions) ([]Unit, error) {
	fileOut, err := execx.Run(ctx, "systemctl", "--user", "list-unit-files", "--all", "--output=json", "--no-pager")
	if err != nil {
		return nil, err
	}
	var files []rawUnitFile
	if err := json.Unmarshal([]byte(fileOut), &files); err != nil {
		return nil, err
	}

	expected := map[string]string{} // unit name -> quadlet filename
	if entries, err := os.ReadDir(quadletDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				expected[quadlets.UnitName(e.Name())] = e.Name()
			}
		}
	}

	loadedOut, err := execx.Run(ctx, "systemctl", "--user", "list-units", "--all", "--output=json", "--no-pager")
	if err != nil {
		return nil, err
	}
	var loaded []rawUnit
	if err := json.Unmarshal([]byte(loadedOut), &loaded); err != nil {
		return nil, err
	}

	fileStates := map[string]string{}
	candidates := map[string]bool{}
	for _, f := range files {
		candidates[f.UnitFile] = true
		fileStates[f.UnitFile] = f.State
	}
	for _, u := range loaded {
		// A full loaded-unit scan is only needed for the "all"/favorite
		// categories; otherwise only quadlet orphans (loaded but with no
		// backing file) are worth the extra `show` call.
		if opts.All || opts.Favorite {
			candidates[u.Unit] = true
		} else if _, ok := expected[u.Unit]; ok {
			candidates[u.Unit] = true
		}
	}

	units := []Unit{}
	for name := range candidates {
		props, err := execx.Run(ctx, "systemctl", "--user", "show", name,
			"--property=LoadState,ActiveState,SubState,Description,SourcePath,FragmentPath,NRestarts,ConditionTimestamp")
		if err != nil {
			continue
		}
		vals := parseProperties(props)
		sourcePath := vals["SourcePath"]
		isQuadlet := sourcePath != "" && strings.HasPrefix(sourcePath, quadletDir)
		if !isQuadlet {
			if filename, ok := expected[name]; ok {
				isQuadlet = true
				sourcePath = filepath.Join(quadletDir, filename)
			}
		}
		isFavorite := favorites[name]
		if !opts.All && !(opts.Quadlet && isQuadlet) && !(opts.Favorite && isFavorite) {
			continue
		}
		nRestarts, _ := strconv.Atoi(vals["NRestarts"])
		var since string
		if t, err := time.Parse(systemdTimestampLayout, vals["ConditionTimestamp"]); err == nil {
			since = t.Format(time.RFC3339)
		}
		fragmentPath := vals["FragmentPath"]
		units = append(units, Unit{
			Name:           name,
			Load:           vals["LoadState"],
			Active:         vals["ActiveState"],
			Sub:            vals["SubState"],
			Description:    vals["Description"],
			SourcePath:     sourcePath,
			FragmentPath:   fragmentPath,
			NRestarts:      nRestarts,
			IsQuadlet:      isQuadlet,
			IsFavorite:     isFavorite,
			SinceTimestamp: since,
			UnitFileState:  fileStates[name],
			IsEditable:     fragmentPath != "" && !strings.HasPrefix(fragmentPath, "/run"),
		})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].Name < units[j].Name })
	return units, nil
}

// Content returns the fully-resolved unit file as systemd sees it (via
// `systemctl cat`), which for quadlet-origin units includes the generated
// .service file systemd actually loads, not the source .container/.network file.
func Content(ctx context.Context, unit string) (string, error) {
	return execx.Run(ctx, "systemctl", "--user", "cat", unit)
}

// Validate checks a not-yet-written unit file for syntax/semantic errors via
// `systemd-analyze verify`, which - unlike quadlet's generator - can check an
// arbitrary file path directly without it living in a systemd search path.
func Validate(ctx context.Context, content, filename string) error {
	tmp, err := os.CreateTemp("", "podtainer-unit-validate-*"+filepath.Ext(filename))
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.WriteString(content)
	tmp.Close()
	if werr != nil {
		return werr
	}
	_, err = execx.Run(ctx, "systemd-analyze", "--user", "verify", tmp.Name())
	return err
}

// Create writes a brand-new unit file to dir, refusing to clobber an
// existing one, then reloads systemd and starts the unit.
func Create(ctx context.Context, dir, filename, content string) error {
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("a unit file named %q already exists", filename)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := Validate(ctx, content, filename); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if _, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	_, err := execx.Run(ctx, "systemctl", "--user", "start", filename)
	return err
}

// WriteContent overwrites a static unit file's raw content and reloads
// systemd so the change takes effect. Units generated at runtime (their
// FragmentPath lives under /run, e.g. quadlet-generated .service files) are
// rejected rather than trusting the caller's IsEditable check, since editing
// them would just be silently discarded on the next daemon-reload anyway.
func WriteContent(ctx context.Context, unit, content string) error {
	props, err := execx.Run(ctx, "systemctl", "--user", "show", unit, "--property=FragmentPath")
	if err != nil {
		return err
	}
	path := parseProperties(props)["FragmentPath"]
	if path == "" {
		return fmt.Errorf("unit %q has no unit file to edit", unit)
	}
	if strings.HasPrefix(path, "/run") {
		return fmt.Errorf("unit %q is generated at runtime and can't be edited directly", unit)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	_, err = execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}

// Delete stops and disables the unit, then removes its unit file and
// reloads systemd. Units generated at runtime (FragmentPath under /run,
// e.g. quadlet-generated .service files) are rejected since there's no
// static file to remove.
func Delete(ctx context.Context, unit string) error {
	props, err := execx.Run(ctx, "systemctl", "--user", "show", unit, "--property=FragmentPath")
	if err != nil {
		return err
	}
	path := parseProperties(props)["FragmentPath"]
	if path == "" {
		return fmt.Errorf("unit %q has no unit file to delete", unit)
	}
	if strings.HasPrefix(path, "/run") {
		return fmt.Errorf("unit %q is generated at runtime and can't be deleted directly", unit)
	}
	_, _ = execx.Run(ctx, "systemctl", "--user", "stop", unit)
	_, _ = execx.Run(ctx, "systemctl", "--user", "disable", unit)
	if err := os.Remove(path); err != nil {
		return err
	}
	_, err = execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}

func Start(ctx context.Context, unit string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "start", unit)
	return err
}

func Stop(ctx context.Context, unit string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "stop", unit)
	return err
}

func Restart(ctx context.Context, unit string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "restart", unit)
	return err
}

func Enable(ctx context.Context, unit string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "enable", unit)
	return err
}

func Disable(ctx context.Context, unit string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "disable", unit)
	return err
}

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
