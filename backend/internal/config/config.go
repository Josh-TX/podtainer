package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Port          int
	StacksDir     string
	QuadletDir    string
	AuthFile      string
	FavoritesFile string
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

	for _, dir := range []string{stacksDir, quadletDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	return &Config{
		Port:          port,
		StacksDir:     stacksDir,
		QuadletDir:    quadletDir,
		AuthFile:      filepath.Join(xdgConfig, "podtainer", "htpasswd"),
		FavoritesFile: filepath.Join(xdgConfig, "podtainer", "systemd-favorites.txt"),
	}, nil
}
