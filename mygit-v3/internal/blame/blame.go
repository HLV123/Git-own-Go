package blame

import (
	"fmt"
	"strings"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// Line represents one blamed line.
type Line struct {
	LineNo     int
	Content    string
	CommitHash string
	Author     string
	Message    string
}

// File runs blame on a file path starting from a given commit hash.
func File(r *repo.Repo, startCommit, filePath string) ([]Line, error) {
	// Walk commit history and find when each line was last introduced.
	// Strategy: collect (commit, lines) pairs in chronological order,
	// then for each line in current file, find the most recent commit that changed it.

	type commitInfo struct {
		hash    string
		author  string
		message string
		lines   []string
	}

	// Traverse DAG from startCommit (BFS, oldest first via reversal)
	var history []commitInfo
	visited := map[string]bool{}
	queue := []string{startCommit}

	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if visited[h] || h == "" {
			continue
		}
		visited[h] = true

		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			return nil, err
		}
		commit, err := object.ParseCommit(content)
		if err != nil {
			return nil, err
		}

		// Get file content at this commit
		lines, err := fileAtCommit(r, commit.Tree, filePath)
		if err != nil {
			// File didn't exist at this commit — skip
			queue = append(queue, commit.Parents...)
			continue
		}

		author := commit.Author
		// Extract just the name part "Name <email> ts tz" -> "Name"
		if idx := strings.Index(author, " <"); idx >= 0 {
			author = author[:idx]
		}

		msg := commit.Message
		if idx := strings.Index(msg, "\n"); idx >= 0 {
			msg = msg[:idx]
		}

		history = append(history, commitInfo{
			hash:    h,
			author:  author,
			message: msg,
			lines:   lines,
		})
		queue = append(queue, commit.Parents...)
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("file %q not found in history", filePath)
	}

	// Current file lines (from most recent = history[0])
	current := history[0].lines
	result := make([]Line, len(current))

	for i, lineContent := range current {
		// Find earliest commit where this line appeared
		blameHash := history[0].hash
		blameAuthor := history[0].author
		blameMsg := history[0].message

		for _, ci := range history[1:] {
			for _, l := range ci.lines {
				if l == lineContent {
					blameHash = ci.hash
					blameAuthor = ci.author
					blameMsg = ci.message
					break
				}
			}
		}

		result[i] = Line{
			LineNo:     i + 1,
			Content:    lineContent,
			CommitHash: blameHash,
			Author:     blameAuthor,
			Message:    blameMsg,
		}
	}

	return result, nil
}

// fileAtCommit returns lines of a file at a given tree hash.
func fileAtCommit(r *repo.Repo, treeHash, filePath string) ([]string, error) {
	parts := strings.Split(filePath, "/")
	return resolveFile(r, treeHash, parts)
}

func resolveFile(r *repo.Repo, treeHash string, parts []string) ([]string, error) {
	_, content, err := object.ReadRaw(r, treeHash)
	if err != nil {
		return nil, err
	}
	tree, err := object.ParseTree(content)
	if err != nil {
		return nil, err
	}

	for _, e := range tree.Entries {
		if e.Name == parts[0] {
			if len(parts) == 1 {
				// Found the file
				_, blobContent, err := object.ReadRaw(r, e.Hash)
				if err != nil {
					return nil, err
				}
				lines := strings.Split(string(blobContent), "\n")
				if len(lines) > 0 && lines[len(lines)-1] == "" {
					lines = lines[:len(lines)-1]
				}
				return lines, nil
			}
			// Recurse into subdirectory
			if e.IsDir() {
				return resolveFile(r, e.Hash, parts[1:])
			}
		}
	}
	return nil, fmt.Errorf("path not found: %s", strings.Join(parts, "/"))
}

// Format formats blame output.
func Format(lines []Line) string {
	var sb strings.Builder
	for _, l := range lines {
		short := l.CommitHash
		if len(short) > 7 {
			short = short[:7]
		}
		sb.WriteString(fmt.Sprintf("%s %-20s %4d | %s\n",
			short, l.Author, l.LineNo, l.Content))
	}
	return sb.String()
}
