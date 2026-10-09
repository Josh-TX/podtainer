// Package envfile manages a Podtainer-owned environment.d file that sets
// environment variables (typically PATH) for the systemd --user manager, so
// user units can find binaries that only the user's shell knows about.
package envfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"podtainer/internal/execx"
)

const (
	fileName    = "50-podtainer.conf"
	legacyName  = "50-podtainer-path.conf"
	shellMarker = "__PODTAINER_PATH__"
)

var assignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

type Info struct {
	// Path is the full path of the managed file.
	Path string `json:"path"`
	// Home is the user's home directory.
	Home string `json:"home"`
	// Content is the managed file's raw contents, empty if it doesn't exist.
	Content string `json:"content"`
	// System is the PATH from system-wide login config, empty if it couldn't be determined.
	System string `json:"system"`
	// SystemError is set when the system PATH couldn't be detected.
	SystemError string `json:"systemError,omitempty"`
	// Shell is the PATH of the user's login shell, empty if it couldn't be determined.
	Shell string `json:"shell"`
	// ShellError is set when the shell PATH couldn't be detected.
	ShellError string `json:"shellError,omitempty"`
}

func shellPath(ctx context.Context) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	out, err := execx.RunLong(ctx, 10*time.Second, shell, "-l", "-i", "-c", "echo "+shellMarker+"; printenv PATH")
	if err != nil {
		return "", err
	}
	_, after, found := strings.Cut(out, shellMarker)
	if !found {
		return "", fmt.Errorf("unexpected output from %s", shell)
	}
	return strings.TrimSpace(after), nil
}

// systemPath is the PATH a login shell gets from system config alone
// (/etc/profile and friends), with the user's own dotfiles bypassed.
func systemPath(ctx context.Context) (string, error) {
	out, err := execx.RunLong(ctx, 10*time.Second, "env", "-i", "HOME=/nonexistent", "/bin/sh", "-l", "-c", "printenv PATH")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// read returns the managed file, falling back to the legacy PATH-only file.
func read(dir string) string {
	for _, name := range []string{fileName, legacyName} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return string(data)
		}
	}
	return ""
}

func Get(ctx context.Context, envDir string) (*Info, error) {
	home, _ := os.UserHomeDir()
	info := &Info{Home: home, Path: filepath.Join(envDir, fileName), Content: read(envDir)}
	if system, err := systemPath(ctx); err != nil {
		info.SystemError = err.Error()
	} else {
		info.System = system
	}
	if shell, err := shellPath(ctx); err != nil {
		info.ShellError = err.Error()
	} else {
		info.Shell = shell
	}
	return info, nil
}

// Set writes content verbatim, or removes the file if blank. Every non-comment
// line must be a KEY=value assignment. The legacy file is removed.
func Set(envDir, content string) error {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	for i, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if !assignRe.MatchString(t) {
			return fmt.Errorf("line %d: expected KEY=value", i+1)
		}
	}

	path := filepath.Join(envDir, fileName)
	if err := os.Remove(filepath.Join(envDir, legacyName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.TrimSpace(content) == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Reload makes the user manager re-read environment.d for units started afterwards.
func Reload(ctx context.Context) error {
	_, err := execx.Run(ctx, "systemctl", "--user", "daemon-reload")
	return err
}
