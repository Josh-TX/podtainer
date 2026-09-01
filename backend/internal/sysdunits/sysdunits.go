// Package sysdunits lists systemd --user units that were generated from a
// quadlet file, identified via each unit's SourcePath rather than any
// Podtainer-specific naming or label, so hand-authored quadlets outside
// Podtainer show up too.
package sysdunits

import (
	"context"
	"encoding/json"
	"strings"

	"podtainer/internal/execx"
)

type Unit struct {
	Name        string `json:"name"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	Description string `json:"description"`
	SourcePath  string `json:"sourcePath"`
}

type rawUnitFile struct {
	UnitFile string `json:"unit_file"`
}

// List returns every systemd --user unit whose SourcePath lives under
// quadletDir, i.e. every quadlet-origin unit on the system. It uses
// list-unit-files rather than list-units so units that were generated but
// never started or enabled (and so never loaded into the manager) still show
// up.
func List(ctx context.Context, quadletDir string) ([]Unit, error) {
	out, err := execx.Run(ctx, "systemctl", "--user", "list-unit-files", "--all", "--output=json", "--no-pager")
	if err != nil {
		return nil, err
	}

	var files []rawUnitFile
	if err := json.Unmarshal([]byte(out), &files); err != nil {
		return nil, err
	}

	units := []Unit{}
	for _, f := range files {
		props, err := execx.Run(ctx, "systemctl", "--user", "show", f.UnitFile,
			"--property=LoadState,ActiveState,SubState,Description,SourcePath")
		if err != nil {
			continue
		}
		vals := parseProperties(props)
		sourcePath := vals["SourcePath"]
		if sourcePath == "" || !strings.HasPrefix(sourcePath, quadletDir) {
			continue
		}
		units = append(units, Unit{
			Name:        f.UnitFile,
			Load:        vals["LoadState"],
			Active:      vals["ActiveState"],
			Sub:         vals["SubState"],
			Description: vals["Description"],
			SourcePath:  sourcePath,
		})
	}
	return units, nil
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
