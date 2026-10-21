// Package files is a file manager over the server's own filesystem, run with
// the Podtainer process's own permissions (no podman involvement).
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"podtainer/internal/execx"
)

// MaxEditSize caps what the editor will load; larger files are download-only.
const MaxEditSize = 1 << 20

type Entry struct {
	Name       string    `json:"name"`
	IsDir      bool      `json:"isDir"`
	IsSymlink  bool      `json:"isSymlink"`
	LinkTarget string    `json:"linkTarget,omitempty"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"modTime"`
	Mode       string    `json:"mode"` // e.g. rwxr-xr-x
	Perm       int       `json:"perm"` // numeric permission bits (0-0777)
	Owner      string    `json:"owner"`
	Group      string    `json:"group"`
}

// IsText reports whether b looks like editable text: valid UTF-8, no NULs.
func IsText(b []byte) bool {
	return utf8.Valid(b) && !containsNUL(b)
}

func containsNUL(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// Clean validates p as an absolute path and normalizes it. Empty means home.
func Clean(p string) (string, error) {
	if p == "" {
		return Home()
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be absolute")
	}
	return filepath.Clean(p), nil
}

func Home() (string, error) {
	return os.UserHomeDir()
}

var (
	nameMu   sync.Mutex
	uidNames = map[uint32]string{}
	gidNames = map[uint32]string{}
)

func lookupUser(id uint32) string {
	nameMu.Lock()
	defer nameMu.Unlock()
	if n, ok := uidNames[id]; ok {
		return n
	}
	n := strconv.FormatUint(uint64(id), 10)
	if u, err := user.LookupId(n); err == nil {
		n = u.Username
	}
	uidNames[id] = n
	return n
}

func lookupGroup(id uint32) string {
	nameMu.Lock()
	defer nameMu.Unlock()
	if n, ok := gidNames[id]; ok {
		return n
	}
	n := strconv.FormatUint(uint64(id), 10)
	if g, err := user.LookupGroupId(n); err == nil {
		n = g.Name
	}
	gidNames[id] = n
	return n
}

func rwx(m fs.FileMode) string {
	const chars = "rwxrwxrwx"
	b := []byte("---------")
	for i := 0; i < 9; i++ {
		if m&(1<<uint(8-i)) != 0 {
			b[i] = chars[i]
		}
	}
	return string(b)
}

func makeEntry(dir, name string, info fs.FileInfo) Entry {
	e := Entry{
		Name:    name,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}
	target := info
	if info.Mode()&fs.ModeSymlink != 0 {
		e.IsSymlink = true
		full := filepath.Join(dir, name)
		e.LinkTarget, _ = os.Readlink(full)
		if t, err := os.Stat(full); err == nil {
			target = t
			e.IsDir = t.IsDir()
			e.Size = t.Size()
		}
	}
	e.Perm = int(target.Mode().Perm())
	e.Mode = rwx(target.Mode())
	if st, ok := target.Sys().(*syscall.Stat_t); ok {
		e.Owner = lookupUser(st.Uid)
		e.Group = lookupGroup(st.Gid)
	}
	return e
}

// ListDir lists dir's entries. Directory sizes are reported as 0; callers
// compute them on demand via DirSize.
func ListDir(dir string) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := makeEntry(dir, de.Name(), info)
		if e.IsDir {
			e.Size = 0
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// Stat describes a single path (following symlinks for isDir/size).
func Stat(p string) (isDir bool, size int64, err error) {
	info, err := os.Stat(p)
	if err != nil {
		return false, 0, err
	}
	return info.IsDir(), info.Size(), nil
}

// ReadText returns the file's content when it's small enough and text.
// editable=false means the UI should only offer a download.
func ReadText(p string, size int64) (content string, editable bool, err error) {
	if size > MaxEditSize {
		return "", false, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false, err
	}
	if !IsText(b) {
		return "", false, nil
	}
	return string(b), true, nil
}

func WriteFile(p, content string) error {
	return os.WriteFile(p, []byte(content), 0o644)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Upload streams r to p. Without overwrite it refuses an existing path.
func Upload(r io.Reader, p string, overwrite bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(p, flags, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%q already exists", filepath.Base(p))
		}
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func CreateFile(p string) error {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%q already exists", filepath.Base(p))
		}
		return err
	}
	return f.Close()
}

func Mkdir(p string) error {
	if err := os.Mkdir(p, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%q already exists", filepath.Base(p))
		}
		return err
	}
	return nil
}

func Delete(p string) error {
	if p == "/" {
		return fmt.Errorf("cannot delete the filesystem root")
	}
	return os.RemoveAll(p)
}

// Move moves or (with isCopy) copies src to dest, refusing to overwrite.
func Move(src, dest string, isCopy bool) error {
	if src == "/" {
		return fmt.Errorf("cannot move or copy the filesystem root")
	}
	if dest == src {
		return fmt.Errorf("source and destination are the same")
	}
	if strings.HasPrefix(dest, src+string(filepath.Separator)) {
		return fmt.Errorf("cannot move or copy a folder into itself")
	}
	if exists(dest) {
		return fmt.Errorf("%q already exists", dest)
	}
	if isCopy {
		return copyTree(src, dest)
	}
	err := os.Rename(src, dest)
	if errors.Is(err, syscall.EXDEV) {
		if err := copyTree(src, dest); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}
	return err
}

func copyTree(src, dest string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, target)
		case info.IsDir():
			return os.Mkdir(target, info.Mode().Perm()|0o700)
		case info.Mode().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		default:
			return nil // sockets, devices, etc.
		}
	})
}

// Chmod sets p's permission bits, optionally across a whole tree. Symlinks
// are skipped when recursing.
func Chmod(p string, perm int, recursive bool) error {
	if perm < 0 || perm > 0o777 {
		return fmt.Errorf("mode out of range")
	}
	mode := fs.FileMode(perm)
	if !recursive {
		return os.Chmod(p, mode)
	}
	return filepath.WalkDir(p, func(q string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		return os.Chmod(q, mode)
	})
}

// DirSize returns the recursive disk usage of a directory via du. Best
// effort: unreadable subtrees still yield a partial total.
func DirSize(ctx context.Context, p string) (int64, error) {
	out, _ := execx.RunLong(ctx, 2*time.Minute, "du", "-sb", "--", p)
	size, _, ok := strings.Cut(strings.TrimSpace(out), "\t")
	if !ok {
		return 0, fmt.Errorf("could not compute size")
	}
	return strconv.ParseInt(size, 10, 64)
}

// UniquePath returns p, or "name (n).ext" for the first n whose path is free.
func UniquePath(p string) string {
	if !exists(p) {
		return p
	}
	dir, base := filepath.Split(p)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" { // dotfile like ".bashrc"
		stem, ext = base, ""
	}
	for n := 1; ; n++ {
		c := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
		if !exists(c) {
			return c
		}
	}
}

// Existing returns the subset of paths that already exist.
func Existing(paths []string) []string {
	out := []string{}
	for _, p := range paths {
		if exists(p) {
			out = append(out, p)
		}
	}
	return out
}

// Paste moves or copies src to dest, resolving an existing dest per
// resolution: "overwrite", "skip", "rename" (append " (n)"), or "" (error).
// Copying into the same folder always renames; moving there is a no-op.
// It reports whether anything was written.
func Paste(src, dest string, isCopy bool, resolution string) (bool, error) {
	if filepath.Dir(src) == filepath.Dir(dest) {
		if !isCopy {
			return false, nil
		}
		resolution = "rename"
	}
	if exists(dest) {
		switch resolution {
		case "skip":
			return false, nil
		case "rename":
			dest = UniquePath(dest)
		case "overwrite":
			if src == dest || strings.HasPrefix(src, dest+string(filepath.Separator)) {
				return false, fmt.Errorf("cannot overwrite %q with its own contents", dest)
			}
			if err := Delete(dest); err != nil {
				return false, err
			}
		}
	}
	return true, Move(src, dest, isCopy)
}
