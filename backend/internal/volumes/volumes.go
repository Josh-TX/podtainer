// Package volumes is the general-purpose view over every podman volume on
// the host, plus a file browser/editor over each volume's contents.
//
// Rootless podman remaps container UIDs into a subuid range on the host, so
// a volume file written by a non-root container process is often owned by a
// host UID Podtainer's own process can't touch. Every filesystem operation
// here is therefore run inside `podman unshare`, which re-execs into the
// same user namespace podman set up for its containers, so reads/writes see
// the same ownership a container mounting the volume would see.
package volumes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"podtainer/internal/execx"
)

const maxEditSize = 1 << 20 // 1MB, matches the UI's edit-attempt cap

type Volume struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	CreatedAt  string `json:"createdAt"`
}

type rawVolume struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Mountpoint string `json:"Mountpoint"`
	CreatedAt  string `json:"CreatedAt"`
}

type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

func List(ctx context.Context) ([]Volume, error) {
	out, err := execx.Run(ctx, "podman", "volume", "ls", "--format", "json")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []Volume{}, nil
	}
	var raws []rawVolume
	if err := json.Unmarshal([]byte(out), &raws); err != nil {
		return nil, err
	}
	result := make([]Volume, 0, len(raws))
	for _, r := range raws {
		result = append(result, Volume{Name: r.Name, Driver: r.Driver, Mountpoint: r.Mountpoint, CreatedAt: r.CreatedAt})
	}
	return result, nil
}

func Inspect(ctx context.Context, name string) (*Volume, error) {
	out, err := execx.Run(ctx, "podman", "volume", "inspect", name)
	if err != nil {
		return nil, err
	}
	var raws []rawVolume
	if err := json.Unmarshal([]byte(out), &raws); err != nil {
		return nil, err
	}
	if len(raws) == 0 {
		return nil, fmt.Errorf("volume %q not found", name)
	}
	r := raws[0]
	return &Volume{Name: r.Name, Driver: r.Driver, Mountpoint: r.Mountpoint, CreatedAt: r.CreatedAt}, nil
}

// resolvePath joins a volume's mountpoint with a user-supplied relative path
// and confines the result to stay inside the mountpoint, since relPath comes
// straight off the URL.
func resolvePath(mountpoint, relPath string) (string, error) {
	clean := filepath.Clean("/" + relPath)
	full := filepath.Join(mountpoint, clean)
	if full != mountpoint && !strings.HasPrefix(full, mountpoint+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path")
	}
	return full, nil
}

func unshareArgs(args ...string) []string {
	return append([]string{"unshare", "--"}, args...)
}

func unshareRun(ctx context.Context, args ...string) (string, error) {
	return execx.Run(ctx, "podman", unshareArgs(args...)...)
}

func unshareRunLong(ctx context.Context, args ...string) (string, error) {
	return execx.RunLong(ctx, 2*time.Minute, "podman", unshareArgs(args...)...)
}

func ListDir(ctx context.Context, mountpoint, relPath string) ([]Entry, error) {
	dir, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return nil, err
	}
	out, err := unshareRun(ctx, "find", dir, "-mindepth", "1", "-maxdepth", "1", "-printf", "%f\t%y\t%s\n")
	if err != nil {
		return nil, err
	}
	entries := []Entry{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, Entry{Name: parts[0], IsDir: parts[1] == "d", Size: size})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func fileSize(ctx context.Context, full string) (int64, error) {
	out, err := unshareRun(ctx, "stat", "-c", "%s", full)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

// Stat reports whether relPath is a directory and, for files, its size in
// bytes. It's the single entry point the GET /fs handler uses to decide
// whether to list a directory or return file content.
func Stat(ctx context.Context, mountpoint, relPath string) (isDir bool, size int64, err error) {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return false, 0, err
	}
	out, err := unshareRun(ctx, "stat", "-c", "%F\t%s", full)
	if err != nil {
		return false, 0, err
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	if len(parts) != 2 {
		return false, 0, fmt.Errorf("unexpected stat output %q", out)
	}
	size, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false, 0, err
	}
	return parts[0] == "directory", size, nil
}

// ReadFile returns a file's content for the editor. It refuses anything over
// maxEditSize so a stray click on a multi-gigabyte file can't be read
// straight into memory.
func ReadFile(ctx context.Context, mountpoint, relPath string) (string, error) {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return "", err
	}
	size, err := fileSize(ctx, full)
	if err != nil {
		return "", err
	}
	if size > maxEditSize {
		return "", fmt.Errorf("file is too large to edit (%d bytes)", size)
	}
	return unshareRun(ctx, "cat", full)
}

// WriteFile overwrites relPath's content, creating it if it doesn't exist.
// Used both for saving edits to an existing file and for the "new file"
// action, so it does not check for existence first.
func WriteFile(ctx context.Context, mountpoint, relPath, content string) error {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	return runWithStdin(ctx, strings.NewReader(content), "cp", "/dev/stdin", full)
}

// StreamDownload writes a file's raw bytes directly to w without buffering
// the whole thing in memory, since downloads have no size cap.
func StreamDownload(ctx context.Context, w io.Writer, mountpoint, relPath string) error {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "podman", unshareArgs("cat", full)...)
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Exists reports whether relPath exists in the volume. Any error from the
// underlying check (including "doesn't exist") is treated as false.
func Exists(ctx context.Context, mountpoint, relPath string) bool {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return false
	}
	_, err = unshareRun(ctx, "test", "-e", full)
	return err == nil
}

// UploadFile streams r's body straight to disk without buffering, since
// uploads have no size cap. It refuses to overwrite an existing path.
func UploadFile(ctx context.Context, r io.Reader, mountpoint, relPath string) error {
	if Exists(ctx, mountpoint, relPath) {
		return fmt.Errorf("%q already exists", filepath.Base(relPath))
	}
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	return runWithStdin(ctx, r, "cp", "/dev/stdin", full)
}

// CreateFile makes an empty file, refusing to overwrite an existing one.
func CreateFile(ctx context.Context, mountpoint, relPath string) error {
	if Exists(ctx, mountpoint, relPath) {
		return fmt.Errorf("%q already exists", filepath.Base(relPath))
	}
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	_, err = unshareRun(ctx, "touch", full)
	return err
}

// Mkdir creates a folder, refusing to overwrite an existing path.
func Mkdir(ctx context.Context, mountpoint, relPath string) error {
	if Exists(ctx, mountpoint, relPath) {
		return fmt.Errorf("%q already exists", filepath.Base(relPath))
	}
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	_, err = unshareRun(ctx, "mkdir", full)
	return err
}

// Delete removes a file or, recursively, a folder. It refuses to delete the
// volume's root.
func Delete(ctx context.Context, mountpoint, relPath string) error {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	if full == mountpoint {
		return fmt.Errorf("cannot delete the volume root")
	}
	_, err = unshareRunLong(ctx, "rm", "-rf", full)
	return err
}

// Move moves or (with isCopy) copies relPath to destRelPath, both given
// relative to the volume root. A rename is just a move whose destination
// shares the source's parent directory.
func Move(ctx context.Context, mountpoint, relPath, destRelPath string, isCopy bool) error {
	full, err := resolvePath(mountpoint, relPath)
	if err != nil {
		return err
	}
	if full == mountpoint {
		return fmt.Errorf("cannot move or copy the volume root")
	}
	destFull, err := resolvePath(mountpoint, destRelPath)
	if err != nil {
		return err
	}
	if destFull == full {
		return fmt.Errorf("source and destination are the same")
	}
	if strings.HasPrefix(destFull, full+string(filepath.Separator)) {
		return fmt.Errorf("cannot move or copy a folder into itself")
	}
	if Exists(ctx, mountpoint, destRelPath) {
		return fmt.Errorf("%q already exists", destRelPath)
	}
	if isCopy {
		_, err = unshareRunLong(ctx, "cp", "-r", full, destFull)
	} else {
		_, err = unshareRunLong(ctx, "mv", full, destFull)
	}
	return err
}

func runWithStdin(ctx context.Context, stdin io.Reader, args ...string) error {
	cmd := exec.CommandContext(ctx, "podman", unshareArgs(args...)...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
