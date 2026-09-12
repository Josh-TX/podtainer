// Package favorites persists the set of systemd unit names a user has
// starred on the systemd page, as a plain newline-separated text file (one
// unit name per line), matching the rest of Podtainer's file-based storage.
package favorites

import (
	"bufio"
	"os"
	"sort"
	"strings"
)

func Load(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	set := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if name := strings.TrimSpace(scanner.Text()); name != "" {
			set[name] = true
		}
	}
	return set, scanner.Err()
}

// Set adds or removes name from the favorites file at path.
func Set(path, name string, favorite bool) error {
	set, err := Load(path)
	if err != nil {
		return err
	}
	if favorite {
		set[name] = true
	} else {
		delete(set, name)
	}

	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)

	content := ""
	if len(names) > 0 {
		content = strings.Join(names, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
