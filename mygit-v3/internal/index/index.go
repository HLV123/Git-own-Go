package index

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/user/mygit/internal/repo"
)

// Entry is a single staged file entry.
type Entry struct {
	Mode string // "100644" or "100755"
	Hash string // 40-char hex SHA-1
	Path string // relative path from repo root, using forward slashes
}

// Index is the staging area.
type Index struct {
	Entries []Entry
}

// indexPath returns the path to .mygit/index.
func indexPath(r *repo.Repo) string {
	return r.Path("index")
}

// Read loads the index from disk. Returns empty index if file doesn't exist.
func Read(r *repo.Repo) (*Index, error) {
	path := indexPath(r)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var idx Index
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed index line: %q", line)
		}
		idx.Entries = append(idx.Entries, Entry{
			Mode: parts[0],
			Hash: parts[1],
			Path: parts[2],
		})
	}
	return &idx, scanner.Err()
}

// Write persists the index to disk.
func Write(r *repo.Repo, idx *Index) error {
	path := indexPath(r)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range idx.Entries {
		fmt.Fprintf(w, "%s %s %s\n", e.Mode, e.Hash, e.Path)
	}
	return w.Flush()
}

// Add adds or updates an entry in the index (sorted by path).
func (idx *Index) Add(e Entry) {
	// Normalize to forward slashes
	e.Path = filepath.ToSlash(e.Path)

	for i, existing := range idx.Entries {
		if existing.Path == e.Path {
			idx.Entries[i] = e
			return
		}
	}
	idx.Entries = append(idx.Entries, e)
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Path < idx.Entries[j].Path
	})
}

// Remove removes an entry by path.
func (idx *Index) Remove(path string) {
	path = filepath.ToSlash(path)
	var kept []Entry
	for _, e := range idx.Entries {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	idx.Entries = kept
}
