package bisect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// State holds bisect session data.
type State struct {
	BadHash  string
	GoodHash string
	Current  string   // currently checked out commit
	Tested   map[string]string // hash -> "good"|"bad"|"skip"
}

func statePath(r *repo.Repo) string {
	return r.Path("bisect")
}

// IsActive returns true if bisect is in progress.
func IsActive(r *repo.Repo) bool {
	_, err := os.Stat(statePath(r))
	return err == nil
}

// Start initializes a bisect session.
func Start(r *repo.Repo) error {
	path := statePath(r)
	os.MkdirAll(path, 0755)
	// Clear previous state
	os.Remove(filepath.Join(path, "bad"))
	os.Remove(filepath.Join(path, "good"))
	os.Remove(filepath.Join(path, "log"))
	fmt.Println("Bisect started. Mark commits with 'mygit bisect good <hash>' and 'mygit bisect bad <hash>'")
	return nil
}

// MarkBad marks a commit as bad (has the bug).
func MarkBad(r *repo.Repo, hash string) error {
	return os.WriteFile(filepath.Join(statePath(r), "bad"), []byte(hash+"\n"), 0644)
}

// MarkGood marks a commit as good (no bug).
func MarkGood(r *repo.Repo, hash string) error {
	f, err := os.OpenFile(filepath.Join(statePath(r), "good"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, hash)
	return nil
}

// Next computes the next commit to test using binary search on the commit graph.
// Returns hash to test, remaining steps estimate, and whether bisect is done.
func Next(r *repo.Repo) (hash string, steps int, done bool, err error) {
	badHash, goodHashes, err := readState(r)
	if err != nil {
		return "", 0, false, err
	}
	if badHash == "" {
		return "", 0, false, fmt.Errorf("no bad commit marked (use 'mygit bisect bad <hash>')")
	}
	if len(goodHashes) == 0 {
		return "", 0, false, fmt.Errorf("no good commit marked (use 'mygit bisect good <hash>')")
	}

	// Collect commits between good and bad
	commits, err := collectCommitRange(r, goodHashes, badHash)
	if err != nil {
		return "", 0, false, err
	}

	if len(commits) == 0 {
		return badHash, 0, true, nil
	}
	if len(commits) == 1 {
		return commits[0], 0, true, nil
	}

	// Binary search: pick midpoint
	mid := commits[len(commits)/2]
	steps = log2(len(commits))
	return mid, steps, false, nil
}

// Reset ends the bisect session.
func Reset(r *repo.Repo) error {
	return os.RemoveAll(statePath(r))
}

// Log returns the bisect log.
func Log(r *repo.Repo) ([]string, error) {
	path := filepath.Join(statePath(r), "log")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// AppendLog adds a line to bisect log.
func AppendLog(r *repo.Repo, line string) {
	f, err := os.OpenFile(filepath.Join(statePath(r), "log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}

func readState(r *repo.Repo) (bad string, good []string, err error) {
	data, err := os.ReadFile(filepath.Join(statePath(r), "bad"))
	if err == nil {
		bad = strings.TrimSpace(string(data))
	}

	f, err := os.Open(filepath.Join(statePath(r), "good"))
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			h := strings.TrimSpace(scanner.Text())
			if h != "" {
				good = append(good, h)
			}
		}
	}
	return bad, good, nil
}

// collectCommitRange returns commits reachable from bad but not from any good,
// sorted topologically (oldest first).
func collectCommitRange(r *repo.Repo, goodHashes []string, badHash string) ([]string, error) {
	// Collect all ancestors of good commits
	goodAncestors := map[string]bool{}
	for _, g := range goodHashes {
		if err := collectAllAncestors(r, g, goodAncestors); err != nil {
			return nil, err
		}
	}

	// BFS from bad, collect commits not in good ancestors
	var commits []string
	visited := map[string]bool{}
	queue := []string{badHash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if visited[h] || h == "" || goodAncestors[h] {
			continue
		}
		visited[h] = true
		commits = append(commits, h)

		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			continue
		}
		c, err := object.ParseCommit(content)
		if err != nil {
			continue
		}
		queue = append(queue, c.Parents...)
	}

	// Reverse for chronological order
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

func collectAllAncestors(r *repo.Repo, hash string, seen map[string]bool) error {
	queue := []string{hash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if seen[h] || h == "" {
			continue
		}
		seen[h] = true
		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			continue
		}
		c, err := object.ParseCommit(content)
		if err != nil {
			continue
		}
		queue = append(queue, c.Parents...)
	}
	return nil
}

func log2(n int) int {
	steps := 0
	for n > 1 {
		n /= 2
		steps++
	}
	return steps
}
