package config

import (
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
)

type Config struct {
	Port    int              `short:"p" env:"PODTAINER_PORT" default:"8080" help:"Port to listen on."`
	Version kong.VersionFlag `short:"v" help:"Print version and exit."`

	// Derived from XDG paths; not CLI flags.
	StacksDir      string `kong:"-"`
	QuadletDir     string `kong:"-"`
	SystemdUnitDir string `kong:"-"`
	EnvironmentDir string `kong:"-"`
	AuthFile       string `kong:"-"`
	FavoritesFile  string `kong:"-"`
}

// Load parses args (typically os.Args[1:]); flags override env vars, which override defaults.
// It then resolves (and creates) the XDG directories podtainer uses.
func Load(args []string, version string, opts ...kong.Option) (*Config, error) {
	var cfg Config
	opts = append([]kong.Option{
		kong.Name("podtainer"),
		kong.Description("Browser-based container management UI built on Podman Quadlet and systemd --user."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	}, opts...)
	k, err := kong.New(&cfg, opts...)
	if err != nil {
		return nil, err
	}
	if _, err := k.Parse(args); err != nil {
		return nil, err
	}
	if err := cfg.resolveDirs(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) resolveDirs() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	stacksDir := filepath.Join(xdgConfig, "podtainer", "stacks")
	quadletDir := filepath.Join(xdgConfig, "containers", "systemd")
	systemdUnitDir := filepath.Join(xdgConfig, "systemd", "user")

	for _, dir := range []string{stacksDir, quadletDir, systemdUnitDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	c.StacksDir = stacksDir
	c.QuadletDir = quadletDir
	c.SystemdUnitDir = systemdUnitDir
	c.EnvironmentDir = filepath.Join(xdgConfig, "environment.d")
	c.AuthFile = filepath.Join(xdgConfig, "podtainer", "htpasswd")
	c.FavoritesFile = filepath.Join(xdgConfig, "podtainer", "systemd-favorites.txt")
	return nil
}
