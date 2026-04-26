package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Matcher holds compiled ignore patterns.
type Matcher struct {
	patterns []string
}

// Load reads .mygitignore from the repo root.
func Load(root string) *Matcher {
	m := &Matcher{}
	path := filepath.Join(root, ".mygitignore")
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.patterns = append(m.patterns, line)
	}
	return m
}

// Match returns true if the given relative path should be ignored.
func (m *Matcher) Match(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	base := filepath.Base(relPath)

	for _, pattern := range m.patterns {
		// Match against full path
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
		// Match against base name
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Prefix match for directories (e.g. "vendor/")
		if strings.HasSuffix(pattern, "/") {
			dir := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(relPath, dir+"/") {
				return true
			}
		}
	}
	return false
}
