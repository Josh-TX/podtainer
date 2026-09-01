package quadletgen

import (
	"strings"
)

// section holds the raw lines of one [Section] block, preserving order and
// any lines podlet emitted that we don't otherwise touch.
type section struct {
	name  string
	lines []string
}

// UnitFile is a minimal, order-preserving editor for systemd/quadlet unit
// files (INI-like, but systemd allows repeated keys, so we operate on whole
// lines rather than a key/value map).
type UnitFile struct {
	sections []*section
}

func ParseUnitFile(content string) *UnitFile {
	uf := &UnitFile{}
	var cur *section
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			cur = &section{name: strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")}
			uf.sections = append(uf.sections, cur)
			continue
		}
		if cur == nil {
			if trimmed == "" {
				continue
			}
			cur = &section{name: ""}
			uf.sections = append(uf.sections, cur)
		}
		cur.lines = append(cur.lines, line)
	}
	return uf
}

func (u *UnitFile) section(name string) *section {
	for _, s := range u.sections {
		if s.name == name {
			return s
		}
	}
	return nil
}

// EnsureSection returns the named section, creating it at the end if absent.
func (u *UnitFile) EnsureSection(name string) *section {
	if s := u.section(name); s != nil {
		return s
	}
	s := &section{name: name}
	u.sections = append(u.sections, s)
	return s
}

// RemoveKey deletes every line in the section that sets the given key.
func (u *UnitFile) RemoveKey(sectionName, key string) {
	s := u.section(sectionName)
	if s == nil {
		return
	}
	prefix := key + "="
	kept := s.lines[:0]
	for _, l := range s.lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			continue
		}
		kept = append(kept, l)
	}
	s.lines = kept
}

// HasKey reports whether the section already sets the given key.
func (u *UnitFile) HasKey(sectionName, key string) bool {
	s := u.section(sectionName)
	if s == nil {
		return false
	}
	prefix := key + "="
	for _, l := range s.lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			return true
		}
	}
	return false
}

// Set removes any existing lines for key in the section and appends a single new one.
func (u *UnitFile) Set(sectionName, key, value string) {
	u.RemoveKey(sectionName, key)
	s := u.EnsureSection(sectionName)
	s.lines = append(s.lines, key+"="+value)
}

// Append adds key=value to the section without touching existing entries,
// used for repeatable keys like Label= or Wants=.
func (u *UnitFile) Append(sectionName, key, value string) {
	s := u.EnsureSection(sectionName)
	s.lines = append(s.lines, key+"="+value)
}

func (u *UnitFile) String() string {
	var b strings.Builder
	for i, s := range u.sections {
		if s.name != "" {
			b.WriteString("[" + s.name + "]\n")
		}
		for _, l := range s.lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
		if i != len(u.sections)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
