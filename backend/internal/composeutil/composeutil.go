package composeutil

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateName enforces the strictest common subset of filename / systemd unit
// prefix / podman container name charsets, checked once so we never have to
// special-case a downstream rejection from podlet/systemctl/podman.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("stack name must be lowercase alphanumeric with hyphens, 1-63 chars, not starting/ending with a hyphen")
	}
	return nil
}

type service struct {
	Image    string      `yaml:"image"`
	Build    interface{} `yaml:"build"`
	Networks interface{} `yaml:"networks"`
}

type compose struct {
	Services map[string]service     `yaml:"services"`
	Networks map[string]interface{} `yaml:"networks"`
}

// Validate rejects compose features Podtainer deliberately doesn't support:
// image builds (rootless build scope excluded) and custom network topologies
// (Podtainer always synthesizes exactly one implicit network per stack).
func Validate(content []byte) error {
	var c compose
	if err := yaml.Unmarshal(content, &c); err != nil {
		return fmt.Errorf("invalid compose YAML: %w", err)
	}

	if len(c.Networks) > 0 {
		return fmt.Errorf("custom top-level 'networks:' is not supported; every stack gets one implicit shared network")
	}

	for name, svc := range c.Services {
		if svc.Build != nil {
			return fmt.Errorf("service %q uses 'build:', which is not supported; specify 'image:' instead", name)
		}
		if svc.Networks != nil {
			return fmt.Errorf("service %q sets 'networks:', which is not supported; all services share one implicit network", name)
		}
	}
	return nil
}

// ServiceImages returns service name -> image reference, used by pull-and-restart.
func ServiceImages(content []byte) (map[string]string, error) {
	var c compose
	if err := yaml.Unmarshal(content, &c); err != nil {
		return nil, fmt.Errorf("invalid compose YAML: %w", err)
	}
	out := make(map[string]string, len(c.Services))
	for name, svc := range c.Services {
		if svc.Image != "" {
			out[name] = svc.Image
		}
	}
	return out, nil
}

// ServiceNames returns the sorted-by-map-iteration list of service names declared
// in the compose file, used to know what podlet should have generated.
func ServiceNames(content []byte) ([]string, error) {
	var c compose
	if err := yaml.Unmarshal(content, &c); err != nil {
		return nil, fmt.Errorf("invalid compose YAML: %w", err)
	}
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	return names, nil
}
