// Package quadlets is the general-purpose view over every quadlet unit file
// in Podman's user quadlet search path, whether or not Podtainer manages it.
package quadlets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"podtainer/internal/execx"
)

// systemdTimestampLayout matches the human-readable timestamps emitted by
// `systemctl show` (e.g. "Wed 2026-09-02 19:17:48 CDT").
const systemdTimestampLayout = "Mon 2006-01-02 15:04:05 MST"

type File struct {
	Filename       string `json:"filename"`
	Type           string `json:"type"`  // container, network, volume, target, or other
	Stack          string `json:"stack"` // owning stack name, "" if external/unmanaged
	Load           string `json:"load"`
	Active         string `json:"active"`
	Sub            string `json:"sub"`
	NRestarts      int    `json:"nRestarts"`
	SinceTimestamp string `json:"sinceTimestamp,omitempty"`
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

// UnitType recognizes actual quadlet unit types. A .target file here would
// be misplaced debris, not a legitimate quadlet resident, so it's
// deliberately not included and falls through to "other".
func UnitType(ext string) string {
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
		f := File{Filename: e.Name(), Type: UnitType(ext), Active: "unknown"}

		if raw, err := os.ReadFile(filepath.Join(quadletDir, e.Name())); err == nil {
			f.Stack = stackFromLabel(string(raw))
		}

		if out, err := execx.Run(ctx, "systemctl", "--user", "show", UnitName(e.Name()),
			"--property=LoadState,ActiveState,SubState,NRestarts,ConditionTimestamp"); err == nil {
			vals := parseProperties(out)
			f.Load = vals["LoadState"]
			f.Active = vals["ActiveState"]
			f.Sub = vals["SubState"]
			f.NRestarts, _ = strconv.Atoi(vals["NRestarts"])
			if t, err := time.Parse(systemdTimestampLayout, vals["ConditionTimestamp"]); err == nil {
				f.SinceTimestamp = t.Format(time.RFC3339)
			}
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

// quadletBinaryPaths are the known locations of podman's quadlet generator
// binary, which isn't normally on $PATH.
var quadletBinaryPaths = []string{
	"/usr/lib/podman/quadlet",
	"/usr/libexec/podman/quadlet",
}

func findQuadletBinary() string {
	if p, err := exec.LookPath("quadlet"); err == nil {
		return p
	}
	for _, p := range quadletBinaryPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// The quadlet generator reports a bad file in one of two forms: a syntax
// error caught while parsing it (keyed by full path), or a semantic error
// caught while converting it, e.g. an unresolvable Network= reference
// (keyed by bare filename).
var errorLoadingRe = regexp.MustCompile(`error loading "([^"]+)", (.+)`)
var errorConvertingRe = regexp.MustCompile(`converting "([^"]+)": (.+)`)

// Validate runs the real quadlet generator against content as if it were
// filename's saved content, without touching /run or systemd state, so a
// syntax error can be caught before Write() corrupts the live quadlet
// directory (daemon-reload alone exits 0 even when the generator fails to
// parse a file). The rest of quadletDir is copied in as-is so cross-file
// references like Network=foo.network still resolve during validation.
//
// If the generator binary can't be found, validation is skipped rather than
// blocking the save, since its install path varies by distro.
func Validate(ctx context.Context, quadletDir, filename, content string) error {
	bin := findQuadletBinary()
	if bin == "" {
		return nil
	}

	tmp, err := os.MkdirTemp("", "podtainer-quadlet-validate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	systemdDir := filepath.Join(tmp, "containers", "systemd")
	if err := os.MkdirAll(systemdDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(quadletDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == filename {
			continue
		}
		b, err := os.ReadFile(filepath.Join(quadletDir, e.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(systemdDir, e.Name()), b, 0o644); err != nil {
			return err
		}
	}
	target := filepath.Join(systemdDir, filename)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, bin, "-dryrun", "-user")
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+tmp)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return nil
	}

	for _, line := range strings.Split(stderr.String(), "\n") {
		if m := errorLoadingRe.FindStringSubmatch(line); m != nil && m[1] == target {
			return fmt.Errorf("%s", m[2])
		}
		if m := errorConvertingRe.FindStringSubmatch(line); m != nil && m[1] == filename {
			return fmt.Errorf("%s", m[2])
		}
	}
	return fmt.Errorf("quadlet validation failed: %s", strings.TrimSpace(stderr.String()))
}

// Create writes a brand-new quadlet file to dir, refusing to clobber an
// existing one, validating it via the quadlet generator, then reloading
// systemd and starting the resulting unit.
func Create(ctx context.Context, dir, filename, content string) error {
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("a quadlet file named %q already exists", filename)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := Validate(ctx, dir, filename, content); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if _, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	_, err := execx.Run(ctx, "systemctl", "--user", "start", UnitName(filename))
	return err
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
