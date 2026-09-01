package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Port       int
	StacksDir  string
	QuadletDir string
	// TargetDir is systemd's own user unit directory. Podtainer's
	// per-stack .target files are plain systemd units, not a quadlet type,
	// so they must live here rather than in QuadletDir to be found by
	// systemd at all.
	TargetDir string
}

func Load(port int) (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	stacksDir := filepath.Join(xdgConfig, "podtainer", "stacks")
	quadletDir := filepath.Join(xdgConfig, "containers", "systemd")
	targetDir := filepath.Join(xdgConfig, "systemd", "user")

	for _, dir := range []string{stacksDir, quadletDir, targetDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	return &Config{
		Port:       port,
		StacksDir:  stacksDir,
		QuadletDir: quadletDir,
		TargetDir:  targetDir,
	}, nil
}
