// Package podmanx wraps `podman` CLI invocations for the Containers page:
// a plain `podman ps -a` view covering every container on the host, not
// just quadlet-managed ones.
package podmanx

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"podtainer/internal/execx"
)

type Container struct {
	ID          string            `json:"id"`
	Names       []string          `json:"names"`
	Image       string            `json:"image"`
	ImageID     string            `json:"imageId"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	SystemdUnit string            `json:"systemdUnit"`
}

type rawContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
}

func List(ctx context.Context) ([]Container, error) {
	out, err := execx.Run(ctx, "podman", "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseContainers(out)
}

// ListByVolume returns every container (running or not) that mounts the
// named volume, for the volume detail page's "used by" cross-reference.
func ListByVolume(ctx context.Context, volumeName string) ([]Container, error) {
	out, err := execx.Run(ctx, "podman", "ps", "-a", "--filter", "volume="+volumeName, "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseContainers(out)
}

func parseContainers(out string) ([]Container, error) {
	var raws []rawContainer
	if strings.TrimSpace(out) == "" {
		return []Container{}, nil
	}
	if err := json.Unmarshal([]byte(out), &raws); err != nil {
		return nil, err
	}
	containers := make([]Container, 0, len(raws))
	for _, r := range raws {
		containers = append(containers, Container{
			ID:          r.ID,
			Names:       r.Names,
			Image:       r.Image,
			ImageID:     r.ImageID,
			State:       r.State,
			Status:      r.Status,
			Labels:      r.Labels,
			SystemdUnit: r.Labels["PODMAN_SYSTEMD_UNIT"],
		})
	}
	return containers, nil
}

// Stats returns a single point-in-time CPU/mem snapshot, no streaming.
func Stats(ctx context.Context, nameOrID string) (map[string]any, error) {
	out, err := execx.Run(ctx, "podman", "stats", "--no-stream", "--format", "json", nameOrID)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{}, nil
	}
	return rows[0], nil
}

func Start(ctx context.Context, nameOrID string) error {
	_, err := execx.Run(ctx, "podman", "start", nameOrID)
	return err
}

func Stop(ctx context.Context, nameOrID string) error {
	_, err := execx.Run(ctx, "podman", "stop", nameOrID)
	return err
}

func Restart(ctx context.Context, nameOrID string) error {
	_, err := execx.Run(ctx, "podman", "restart", nameOrID)
	return err
}

func Remove(ctx context.Context, nameOrID string) error {
	_, err := execx.Run(ctx, "podman", "rm", "-f", nameOrID)
	return err
}

// Logs fetches the last n lines for a systemd unit via journald, on demand
// (no live tailing).
// UnitLogs fetches the last n lines for a systemd unit via journald, used
// for quadlet/stack service logs.
func UnitLogs(ctx context.Context, unit string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return execx.RunLong(ctx, 15*time.Second, "journalctl", "--user", "-u", unit, "--no-pager", "-n", strconv.Itoa(lines), "--output=short-iso")
}

// GeneratorLogs fetches the last n lines logged by podman's quadlet
// generator (which runs on every `systemctl --user daemon-reload`, e.g.
// reporting a quadlet file it failed to parse). These aren't tied to any
// single unit, so they're filtered by the generator's process name
// (_COMM=podman-user-gen) rather than -u.
func GeneratorLogs(ctx context.Context, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return execx.RunLong(ctx, 15*time.Second, "journalctl", "--user", "_COMM=podman-user-gen", "--no-pager", "-n", strconv.Itoa(lines), "--output=short-iso")
}

// ContainerLogs fetches the last n lines directly from podman, which works
// regardless of the container's configured log driver.
func ContainerLogs(ctx context.Context, nameOrID string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return execx.RunLong(ctx, 15*time.Second, "podman", "logs", "--tail", strconv.Itoa(lines), nameOrID)
}
