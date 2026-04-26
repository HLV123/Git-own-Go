package reflog

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/user/mygit/internal/repo"
)

// Entry is one reflog line.
type Entry struct {
	OldHash   string
	NewHash   string
	Timestamp time.Time
	Message   string
}

// reflogPath returns .mygit/logs/HEAD or .mygit/logs/refs/heads/<branch>
func reflogPath(r *repo.Repo, ref string) string {
	if ref == "HEAD" {
		return r.Path("logs", "HEAD")
	}
	return r.Path("logs", ref)
}

// Append adds a new entry to the reflog for the given ref.
func Append(r *repo.Repo, ref, oldHash, newHash, message string) error {
	path := reflogPath(r, ref)
	if err := os.MkdirAll(getDir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := time.Now().Unix()
	fmt.Fprintf(f, "%s %s %d\t%s\n", oldHash, newHash, ts, message)
	return nil
}

// Read returns reflog entries for a ref (most recent first).
func Read(r *repo.Repo, ref string) ([]Entry, error) {
	path := reflogPath(r, ref)
	f, err := os.Open(path)
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
		e, err := parseLine(line)
		if err == nil {
			entries = append(entries, e)
		}
	}

	// Reverse for most-recent-first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, scanner.Err()
}

func parseLine(line string) (Entry, error) {
	// Format: "<old> <new> <ts>\t<message>"
	tab := strings.Index(line, "\t")
	if tab < 0 {
		return Entry{}, fmt.Errorf("no tab in reflog line")
	}
	msg := line[tab+1:]
	parts := strings.Fields(line[:tab])
	if len(parts) < 3 {
		return Entry{}, fmt.Errorf("malformed reflog line")
	}

	var ts int64
	fmt.Sscanf(parts[2], "%d", &ts)

	return Entry{
		OldHash:   parts[0],
		NewHash:   parts[1],
		Timestamp: time.Unix(ts, 0),
		Message:   msg,
	}, nil
}

func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
