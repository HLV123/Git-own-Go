package object

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
)

// TreeEntry represents one entry in a tree object.
type TreeEntry struct {
	Mode string // "100644", "100755", "40000"
	Name string
	Hash string // 40-char hex
}

// IsDir returns true if the entry is a directory (tree).
func (e TreeEntry) IsDir() bool {
	return e.Mode == "40000"
}

// EntryType returns "blob" or "tree" based on mode.
func (e TreeEntry) EntryType() string {
	if e.IsDir() {
		return "tree"
	}
	return "blob"
}

// Tree represents a Git tree object.
type Tree struct {
	Entries []TreeEntry
}

func (t *Tree) Type() Type { return TypeTree }

// Serialize encodes the tree in Git's binary format.
// Format per entry: "<mode> <name>\0<20-raw-bytes-hash>"
func (t *Tree) Serialize() ([]byte, error) {
	sorted := make([]TreeEntry, len(t.Entries))
	copy(sorted, t.Entries)
	sortEntries(sorted)

	var buf bytes.Buffer
	for _, e := range sorted {
		buf.WriteString(e.Mode)
		buf.WriteByte(' ')
		buf.WriteString(e.Name)
		buf.WriteByte(0x00)

		raw, err := hex.DecodeString(e.Hash)
		if err != nil {
			return nil, fmt.Errorf("invalid hash %q in tree entry %q: %w", e.Hash, e.Name, err)
		}
		buf.Write(raw)
	}
	return buf.Bytes(), nil
}

// ParseTree decodes binary tree content into a Tree struct.
func ParseTree(content []byte) (*Tree, error) {
	var entries []TreeEntry
	i := 0
	for i < len(content) {
		// Find space between mode and name
		sp := bytes.IndexByte(content[i:], ' ')
		if sp < 0 {
			return nil, fmt.Errorf("malformed tree: no space at offset %d", i)
		}
		mode := string(content[i : i+sp])
		i += sp + 1

		// Find null terminator between name and hash
		nul := bytes.IndexByte(content[i:], 0x00)
		if nul < 0 {
			return nil, fmt.Errorf("malformed tree: no null after name at offset %d", i)
		}
		name := string(content[i : i+nul])
		i += nul + 1

		// 20 raw bytes for hash
		if i+20 > len(content) {
			return nil, fmt.Errorf("malformed tree: not enough bytes for hash at offset %d", i)
		}
		hashHex := hex.EncodeToString(content[i : i+20])
		i += 20

		entries = append(entries, TreeEntry{Mode: mode, Name: name, Hash: hashHex})
	}
	return &Tree{Entries: entries}, nil
}

// sortEntries sorts tree entries using Git's ordering rule:
// directories are compared as if their name ends with '/'.
func sortEntries(entries []TreeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		a := sortKey(entries[i])
		b := sortKey(entries[j])
		return a < b
	})
}

func sortKey(e TreeEntry) string {
	if e.IsDir() {
		return e.Name + "/"
	}
	return e.Name
}
