// Package quadlets is the general-purpose view over every quadlet unit file
// in Podman's user quadlet search path, whether or not Podtainer manages it.
package quadlets

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"podtainer/internal/execx"
)

type File struct {
	Filename string `json:"filename"`
	Type     string `json:"type"`  // container, network, volume, target, or other
	Stack    string `json:"stack"` // owning stack name, "" if external/unmanaged
	Active   string `json:"active"`
}

// UnitName maps a quadlet-generated file to the systemd unit name that
// controls it. Quadlet's generator uses a 1:1 basename mapping for
// .container files but appends a suffix for .network/.volume to avoid
// colliding with same-named units of a different type.
func UnitName(filename string) string {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	switch ext {
	case ".container":
		return base + ".service"
	case ".volume":
		return base + "-volume.service"
	case ".network":
		return base + "-network.service"
	case ".target":
		return filename
	}
	return filename
}

// unitType recognizes actual quadlet unit types. A .target file here would
// be misplaced debris, not a legitimate quadlet resident, so it's
// deliberately not included and falls through to "other".
func unitType(ext string) string {
	switch ext {
	case ".container", ".network", ".volume", ".pod", ".kube":
		return strings.TrimPrefix(ext, ".")
	}
	return "other"
}

// stackFromLabel extracts podtainer.stack=<name> from a unit file's Label=
// lines, our second independently-derived ownership signal alongside the
// filename-prefix convention.
func stackFromLabel(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Label=podtainer.stack=") {
			return strings.TrimPrefix(line, "Label=podtainer.stack=")
		}
	}
	return ""
}

// List scans every quadlet file in the search directory, regardless of
// whether Podtainer generated it.
func List(ctx context.Context, quadletDir string) ([]File, error) {
	entries, err := os.ReadDir(quadletDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []File{}, nil
		}
		return nil, err
	}

	files := []File{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		f := File{Filename: e.Name(), Type: unitType(ext), Active: "unknown"}

		if raw, err := os.ReadFile(filepath.Join(quadletDir, e.Name())); err == nil {
			f.Stack = stackFromLabel(string(raw))
		}

		if out, err := execx.Run(ctx, "systemctl", "--user", "show", UnitName(e.Name()), "--property=ActiveState", "--value"); err == nil {
			f.Active = strings.TrimSpace(out)
		}

		files = append(files, f)
	}
	return files, nil
}

func Read(quadletDir, filename string) (string, error) {
	b, err := os.ReadFile(filepath.Join(quadletDir, filename))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Write overwrites a quadlet file's raw content and reloads systemd so the
// change takes effect. It does not restart the unit.
func Write(ctx context.Context, quadletDir, filename, content string) error {
	if err := os.WriteFile(filepath.Join(quadletDir, filename), []byte(content), 0o644); err != nil {
		return err
	}
	_, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}

// RemoveFile stops and disables filename's unit, deletes the podman
// resource it owns, and removes the file, without reloading systemd —
// callers removing several files at once should batch one daemon-reload
// after the whole set.
//
// A stop on a unit that's already `failed` (rather than `active`) is a
// systemd no-op, so it never runs that unit's ExecStopPost cleanup. That
// leaves the container/network behind even though the unit and file are
// gone, so container/network resources are removed directly here instead
// of relying on that cleanup happening as a side effect of stopping.
func RemoveFile(ctx context.Context, dir, filename string) error {
	unit := UnitName(filename)
	_, _ = execx.Run(ctx, "systemctl", "--user", "stop", unit)
	_, _ = execx.Run(ctx, "systemctl", "--user", "disable", unit)

	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	switch {
	case strings.HasSuffix(filename, ".container"):
		_, _ = execx.Run(ctx, "podman", "rm", "-f", base)
	case strings.HasSuffix(filename, ".network"):
		_, _ = execx.Run(ctx, "podman", "network", "rm", "systemd-"+base)
		// Named volumes are deliberately left alone here: only the
		// .volume unit file goes away, the underlying `podman volume`
		// data persists so deleting a stack can't silently drop it.
	}

	if err := os.Remove(filepath.Join(dir, filename)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func Delete(ctx context.Context, quadletDir, filename string) error {
	if err := RemoveFile(ctx, quadletDir, filename); err != nil {
		return err
	}
	_, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}

func Start(ctx context.Context, filename string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "start", UnitName(filename))
	return err
}

func Stop(ctx context.Context, filename string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "stop", UnitName(filename))
	return err
}

func Restart(ctx context.Context, filename string) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "restart", UnitName(filename))
	return err
}
