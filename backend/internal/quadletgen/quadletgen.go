// Package quadletgen turns a compose file into the set of quadlet unit files
// Podtainer manages for a stack, by shelling out to podlet and then
// normalizing its output to Podtainer's naming/labeling conventions.
package quadletgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"podtainer/internal/execx"
	"podtainer/internal/quadlets"
)

// Unit is one generated file, keyed by its final on-disk basename
// (e.g. "mystack-web.container").
type Unit struct {
	Filename string
	Content  string
}

// Generate runs podlet against composePath and returns the full set of
// quadlet units for the stack, fully post-processed and ready to write.
// It has no side effects outside of a temp directory, so it's safe to call
// repeatedly for drift checks without touching the live quadlet directory.
func Generate(ctx context.Context, stackName, composePath string) ([]Unit, error) {
	tmpDir, err := os.MkdirTemp("", "podtainer-podlet-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// podlet's compose subcommand only groups services into a pod when
	// --pod is passed; omitting it already gives one .container per
	// service with no pod, which is what Podtainer wants.
	//
	// --skip-services-check disables podlet's own check for an existing
	// systemd service of the same name as the file it's about to generate
	// (e.g. "web.service"). That check is meaningless here: Podtainer
	// always renames the output with a stack prefix (e.g.
	// "mystack-web.container") before it's ever installed, so the
	// pre-rename name podlet checks against isn't the name that will
	// actually be used.
	if _, err := execx.RunLong(ctx, 30*time.Second, "podlet", "--file", tmpDir, "--skip-services-check", "compose", composePath); err != nil {
		return nil, fmt.Errorf("podlet conversion failed: %w", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}

	var units []Unit
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		base := strings.TrimSuffix(e.Name(), ext)

		// Podtainer authors its own network unit deterministically (see
		// below) rather than trusting podlet's project-network naming, so
		// any network file podlet produced is dropped here.
		if ext == ".network" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		if err != nil {
			return nil, err
		}

		switch ext {
		case ".container":
			units = append(units, Unit{
				Filename: fmt.Sprintf("%s-%s.container", stackName, base),
				Content:  postProcessContainer(string(raw), stackName, base),
			})
		case ".volume":
			units = append(units, Unit{
				Filename: fmt.Sprintf("%s-%s.volume", stackName, base),
				Content:  postProcessVolume(string(raw), stackName, base),
			})
		default:
			// Unsupported unit type from podlet (e.g. .pod, .kube) — skip;
			// compose validation should have already rejected inputs that
			// would produce these.
		}
	}

	units = append(units, Unit{
		Filename: fmt.Sprintf("%s-default.network", stackName),
		Content:  networkUnit(stackName),
	})
	units = append(units, Unit{
		Filename: fmt.Sprintf("%s.target", stackName),
		Content:  targetUnit(stackName, units),
	})

	sort.Slice(units, func(i, j int) bool { return units[i].Filename < units[j].Filename })
	return units, nil
}

func postProcessContainer(raw, stackName, serviceName string) string {
	uf := ParseUnitFile(raw)

	uf.Set("Unit", "PartOf", stackName+".target")

	uf.Set("Container", "ContainerName", stackName+"-"+serviceName)
	uf.RemoveKey("Container", "Network")
	uf.Append("Container", "Network", stackName+"-default.network")
	uf.Append("Container", "Label", "podtainer.stack="+stackName)
	uf.Append("Container", "Label", "podtainer.service="+serviceName)

	if !uf.HasKey("Service", "Restart") {
		uf.Set("Service", "Restart", "on-failure")
	}

	return uf.String()
}

func postProcessVolume(raw, stackName, volumeName string) string {
	uf := ParseUnitFile(raw)
	uf.Append("Volume", "Label", "podtainer.stack="+stackName)
	uf.Append("Volume", "Label", "podtainer.service="+volumeName)
	return uf.String()
}

func networkUnit(stackName string) string {
	uf := &UnitFile{}
	uf.Set("Network", "Label", "podtainer.stack="+stackName)
	return uf.String()
}

// targetUnit lists every container unit under Wants=/After= so that
// `systemctl start <stack>.target` brings the whole stack up: PartOf= on the
// containers only propagates stop/restart, not start, so the target has to
// pull its members in explicitly.
func targetUnit(stackName string, units []Unit) string {
	var containers []string
	for _, u := range units {
		if strings.HasSuffix(u.Filename, ".container") {
			containers = append(containers, quadlets.UnitName(u.Filename))
		}
	}
	sort.Strings(containers)

	uf := &UnitFile{}
	uf.Set("Unit", "Description", stackName+" stack")
	if len(containers) > 0 {
		list := strings.Join(containers, " ")
		uf.Set("Unit", "Wants", list)
		uf.Set("Unit", "After", list)
	}
	uf.Set("Install", "WantedBy", "default.target")
	return uf.String()
}
