// Package sysdunits lists systemd --user units that were generated from a
// quadlet file, identified via each unit's SourcePath rather than any
// Podtainer-specific naming or label, so hand-authored quadlets outside
// Podtainer show up too.
package sysdunits

import (
	"context"
	"encoding/json"
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
	// SinceTimestamp is when the unit's current run began (ConditionTimestamp),
	// which stays fixed across auto-restart cycles, so it doubles as "failing since"
	// for a unit stuck in the activating/auto-restart loop.
	SinceTimestamp string `json:"sinceTimestamp,omitempty"`
}

type rawUnitFile struct {
	UnitFile string `json:"unit_file"`
}

type rawUnit struct {
	Unit string `json:"unit"`
}

// List returns every systemd --user unit whose SourcePath lives under
// quadletDir, i.e. every quadlet-origin unit on the system, unioned with
// currently-loaded units that match a quadlet file's expected unit name.
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
func List(ctx context.Context, quadletDir string) ([]Unit, error) {
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

	candidates := map[string]bool{}
	for _, f := range files {
		candidates[f.UnitFile] = true
	}
	for _, u := range loaded {
		if _, ok := expected[u.Unit]; ok {
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
		if sourcePath == "" || !strings.HasPrefix(sourcePath, quadletDir) {
			filename, ok := expected[name]
			if !ok {
				continue
			}
			sourcePath = filepath.Join(quadletDir, filename)
		}
		nRestarts, _ := strconv.Atoi(vals["NRestarts"])
		var since string
		if t, err := time.Parse(systemdTimestampLayout, vals["ConditionTimestamp"]); err == nil {
			since = t.Format(time.RFC3339)
		}
		units = append(units, Unit{
			Name:           name,
			Load:           vals["LoadState"],
			Active:         vals["ActiveState"],
			Sub:            vals["SubState"],
			Description:    vals["Description"],
			SourcePath:     sourcePath,
			FragmentPath:   vals["FragmentPath"],
			NRestarts:      nRestarts,
			SinceTimestamp: since,
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
