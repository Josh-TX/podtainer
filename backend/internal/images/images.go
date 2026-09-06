// Package images is the general-purpose view over every podman image on the
// host, regardless of how it got there (pulled, built, or left behind by a
// stack/quadlet).
package images

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"podtainer/internal/execx"
)

type Image struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       int64  `json:"size"`
	CreatedAt  string `json:"createdAt"`
}

type rawImage struct {
	ID         string `json:"Id"`
	Repository string `json:"Repository"`
	Tag        string `json:"Tag"`
	Size       int64  `json:"Size"`
	CreatedAt  string `json:"CreatedAt"`
}

// List returns one row per repo:tag, matching `podman images`' own table
// output: an image with two tags shows up twice, sharing the same ID.
func List(ctx context.Context) ([]Image, error) {
	out, err := execx.Run(ctx, "podman", "image", "ls", "--format", "json")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []Image{}, nil
	}
	var raws []rawImage
	if err := json.Unmarshal([]byte(out), &raws); err != nil {
		return nil, err
	}
	result := make([]Image, 0, len(raws))
	for _, r := range raws {
		result = append(result, Image{ID: r.ID, Repository: r.Repository, Tag: r.Tag, Size: r.Size, CreatedAt: r.CreatedAt})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})
	return result, nil
}

// Delete removes a single image by ID.
func Delete(ctx context.Context, id string) error {
	_, err := execx.Run(ctx, "podman", "rmi", id)
	return err
}

// Prune removes dangling images, or every unused image when all is true.
func Prune(ctx context.Context, all bool) error {
	args := []string{"image", "prune", "-f"}
	if all {
		args = []string{"image", "prune", "-a", "-f"}
	}
	_, err := execx.Run(ctx, "podman", args...)
	return err
}
