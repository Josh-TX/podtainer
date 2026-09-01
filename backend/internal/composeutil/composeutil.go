package composeutil

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// validTopLevelKeys and validServiceKeys are the compose-spec keys Podtainer
// recognizes. They're used to catch typos (e.g. "port" for "ports") with a
// clear message instead of letting them through to podlet, whose own error
// for an unrecognized key is a multi-line Rust error chain.
var validTopLevelKeys = map[string]bool{
	"version": true, "name": true, "services": true, "networks": true,
	"volumes": true, "configs": true, "secrets": true, "include": true,
}

var validServiceKeys = map[string]bool{
	"annotations": true, "attach": true, "blkio_config": true, "build": true,
	"cap_add": true, "cap_drop": true, "cgroup": true, "cgroup_parent": true,
	"command": true, "configs": true, "container_name": true, "credential_spec": true,
	"depends_on": true, "deploy": true, "develop": true, "device_cgroup_rules": true,
	"devices": true, "dns": true, "dns_opt": true, "dns_search": true, "domainname": true,
	"entrypoint": true, "env_file": true, "environment": true, "expose": true, "extends": true,
	"external_links": true, "extra_hosts": true, "gpus": true, "group_add": true,
	"healthcheck": true, "hostname": true, "image": true, "init": true, "ipc": true,
	"isolation": true, "labels": true, "links": true, "logging": true, "mac_address": true,
	"mem_limit": true, "mem_reservation": true, "mem_swappiness": true, "memswap_limit": true,
	"network_mode": true, "networks": true, "oom_kill_disable": true, "oom_score_adj": true,
	"pid": true, "pids_limit": true, "platform": true, "ports": true, "post_start": true,
	"pre_stop": true, "privileged": true, "profiles": true, "pull_policy": true, "read_only": true,
	"restart": true, "runtime": true, "scale": true, "security_opt": true, "shm_size": true,
	"sysctls": true, "stdin_open": true, "stop_grace_period": true, "stop_signal": true,
	"storage_opt": true, "tty": true, "ulimits": true, "user": true, "userns_mode": true,
	"uts": true, "volumes": true, "volumes_from": true, "working_dir": true,
}

// checkUnknownKeys walks the raw YAML (rather than the decoded compose
// struct) so it can flag keys the compose spec doesn't define at all, e.g. a
// typo like "port" instead of "ports". Podlet's own error for this is a
// multi-line Rust error chain, so catching it here gives a much clearer
// message.
func checkUnknownKeys(content []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return fmt.Errorf("invalid compose YAML: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	doc := root.Content[0]

	if err := checkMappingKeys(doc, validTopLevelKeys, "top-level"); err != nil {
		return err
	}

	services := mappingValue(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svcName := services.Content[i].Value
		svc := services.Content[i+1]
		if svc.Kind != yaml.MappingNode {
			continue
		}
		if err := checkMappingKeys(svc, validServiceKeys, fmt.Sprintf("service %q", svcName)); err != nil {
			return err
		}
	}
	return nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func checkMappingKeys(mapping *yaml.Node, valid map[string]bool, context string) error {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if strings.HasPrefix(key, "x-") || valid[key] {
			continue
		}
		msg := fmt.Sprintf("unknown field %q in %s", key, context)
		if suggestion := closestKey(key, valid); suggestion != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", suggestion)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// closestKey suggests a valid key only when it's a plausible typo of one
// (edit distance < 3); otherwise the guess is more confusing than helpful.
func closestKey(key string, valid map[string]bool) string {
	best, bestDist := "", 3
	for k := range valid {
		if d := levenshtein(key, k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	dp := make([]int, len(b)+1)
	for j := range dp {
		dp[j] = j
	}
	for i := 1; i <= len(a); i++ {
		prev := dp[0]
		dp[0] = i
		for j := 1; j <= len(b); j++ {
			tmp := dp[j]
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[j] = min3(dp[j]+1, dp[j-1]+1, prev+cost)
			prev = tmp
		}
	}
	return dp[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

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
// (Podtainer synthesizes at most one implicit shared network per stack, and
// none at all for single-container stacks).
func Validate(content []byte) error {
	var c compose
	if err := yaml.Unmarshal(content, &c); err != nil {
		return fmt.Errorf("invalid compose YAML: %w", err)
	}

	if err := checkUnknownKeys(content); err != nil {
		return err
	}

	if len(c.Networks) > 0 {
		return fmt.Errorf("custom top-level 'networks:' is not supported; stacks with multiple services share one implicit network")
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
