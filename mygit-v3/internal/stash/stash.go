package stash

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/user/mygit/internal/repo"
)

// Entry represents one stash entry.
type Entry struct {
	CommitHash string // hash of stash commit object
	Message    string
}

// stashPath returns path to .mygit/stash file.
func stashPath(r *repo.Repo) string {
	return r.Path("stash")
}

// List returns all stash entries (most recent first).
func List(r *repo.Repo) ([]Entry, error) {
	f, err := os.Open(stashPath(r))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			entries = append(entries, Entry{CommitHash: parts[0], Message: parts[1]})
		}
	}
	// Reverse so most recent is first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, scanner.Err()
}

// Push adds a new entry to the stash stack (prepends).
func Push(r *repo.Repo, commitHash, message string) error {
	existing, err := List(r)
	if err != nil {
		return err
	}

	f, err := os.Create(stashPath(r))
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	// New entry first (but file stores oldest first, we reverse on read)
	// Actually write newest at bottom, reverse on read gives newest first
	for i := len(existing) - 1; i >= 0; i-- {
		fmt.Fprintf(w, "%s %s\n", existing[i].CommitHash, existing[i].Message)
	}
	fmt.Fprintf(w, "%s %s\n", commitHash, message)
	return w.Flush()
}

// Pop removes and returns the most recent stash entry.
func Pop(r *repo.Repo) (*Entry, error) {
	entries, err := List(r)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no stash entries")
	}

	top := entries[0]
	rest := entries[1:]

	// Rewrite file
	f, err := os.Create(stashPath(r))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i := len(rest) - 1; i >= 0; i-- {
		fmt.Fprintf(w, "%s %s\n", rest[i].CommitHash, rest[i].Message)
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}
	return &top, nil
}

// Drop removes the most recent stash entry without returning it.
func Drop(r *repo.Repo, index int) error {
	entries, err := List(r)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(entries) {
		return fmt.Errorf("stash index %d out of range", index)
	}

	entries = append(entries[:index], entries[index+1:]...)

	f, err := os.Create(stashPath(r))
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i := len(entries) - 1; i >= 0; i-- {
		fmt.Fprintf(w, "%s %s\n", entries[i].CommitHash, entries[i].Message)
	}
	return w.Flush()
}
