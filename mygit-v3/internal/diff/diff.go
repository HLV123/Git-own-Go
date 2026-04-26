package diff

import (
	"fmt"
	"strings"

	"github.com/user/mygit/internal/color"
	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// ChangeType represents what happened to a file.
type ChangeType string

const (
	Added    ChangeType = "A"
	Deleted  ChangeType = "D"
	Modified ChangeType = "M"
)

// FileDiff describes a single file change between two trees.
type FileDiff struct {
	Type     ChangeType
	Path     string
	OldHash  string
	NewHash  string
	OldLines []string
	NewLines []string
}

// TreeDiff computes differences between two tree hashes.
// Returns a slice of FileDiff sorted by path.
func TreeDiff(r *repo.Repo, oldTree, newTree string) ([]FileDiff, error) {
	oldFiles := map[string]string{} // path -> hash
	newFiles := map[string]string{}

	if oldTree != "" {
		if err := walkTree(r, oldTree, "", oldFiles); err != nil {
			return nil, fmt.Errorf("reading old tree: %w", err)
		}
	}
	if newTree != "" {
		if err := walkTree(r, newTree, "", newFiles); err != nil {
			return nil, fmt.Errorf("reading new tree: %w", err)
		}
	}

	var diffs []FileDiff

	// Files deleted or modified
	for path, oldHash := range oldFiles {
		newHash, exists := newFiles[path]
		if !exists {
			diffs = append(diffs, FileDiff{Type: Deleted, Path: path, OldHash: oldHash})
		} else if oldHash != newHash {
			diffs = append(diffs, FileDiff{Type: Modified, Path: path, OldHash: oldHash, NewHash: newHash})
		}
	}

	// Files added
	for path, newHash := range newFiles {
		if _, exists := oldFiles[path]; !exists {
			diffs = append(diffs, FileDiff{Type: Added, Path: path, NewHash: newHash})
		}
	}

	// Sort by path
	sortDiffs(diffs)

	// Populate line content for modified files
	for i := range diffs {
		if diffs[i].Type == Modified || diffs[i].Type == Deleted {
			lines, err := blobLines(r, diffs[i].OldHash)
			if err == nil {
				diffs[i].OldLines = lines
			}
		}
		if diffs[i].Type == Modified || diffs[i].Type == Added {
			lines, err := blobLines(r, diffs[i].NewHash)
			if err == nil {
				diffs[i].NewLines = lines
			}
		}
	}

	return diffs, nil
}

// walkTree recursively walks a tree object, populating files map with path->hash.
func walkTree(r *repo.Repo, treeHash string, prefix string, files map[string]string) error {
	_, content, err := object.ReadRaw(r, treeHash)
	if err != nil {
		return err
	}
	tree, err := object.ParseTree(content)
	if err != nil {
		return err
	}

	for _, e := range tree.Entries {
		path := e.Name
		if prefix != "" {
			path = prefix + "/" + e.Name
		}
		if e.IsDir() {
			if err := walkTree(r, e.Hash, path, files); err != nil {
				return err
			}
		} else {
			files[path] = e.Hash
		}
	}
	return nil
}

// blobLines reads a blob and returns its lines.
func blobLines(r *repo.Repo, hash string) ([]string, error) {
	if hash == "" {
		return nil, nil
	}
	_, content, err := object.ReadRaw(r, hash)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	// Remove trailing empty string from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// FormatDiff returns a colored unified-style diff string.
func FormatDiff(diffs []FileDiff) string {
	var sb strings.Builder
	for _, d := range diffs {
		switch d.Type {
		case Added:
			sb.WriteString(color.Header(fmt.Sprintf("A  %s\n", d.Path)))
			for _, line := range d.NewLines {
				sb.WriteString(color.Green("+"+line) + "\n")
			}
		case Deleted:
			sb.WriteString(color.Header(fmt.Sprintf("D  %s\n", d.Path)))
			for _, line := range d.OldLines {
				sb.WriteString(color.Red("-"+line) + "\n")
			}
		case Modified:
			sb.WriteString(color.Header(fmt.Sprintf("M  %s\n", d.Path)))
			hunks := computeHunks(d.OldLines, d.NewLines)
			sb.WriteString(hunks)
		}
	}
	return sb.String()
}

// computeHunks does a simple line-level diff (LCS-based).
func computeHunks(old, new []string) string {
	lcs := lcsLines(old, new)
	var sb strings.Builder

	i, j, k := 0, 0, 0
	for k < len(lcs) {
		for i < len(old) && old[i] != lcs[k] {
			sb.WriteString(color.Red("-"+old[i]) + "\n")
			i++
		}
		for j < len(new) && new[j] != lcs[k] {
			sb.WriteString(color.Green("+"+new[j]) + "\n")
			j++
		}
		sb.WriteString(" " + lcs[k] + "\n")
		i++
		j++
		k++
	}
	// Remaining lines
	for ; i < len(old); i++ {
		sb.WriteString(color.Red("-"+old[i]) + "\n")
	}
	for ; j < len(new); j++ {
		sb.WriteString(color.Green("+"+new[j]) + "\n")
	}
	return sb.String()
}

// lcsLines computes the Longest Common Subsequence of two string slices.
func lcsLines(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack
	result := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([]string{a[i-1]}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return result
}

// sortDiffs sorts FileDiff slice by path.
func sortDiffs(diffs []FileDiff) {
	for i := 1; i < len(diffs); i++ {
		for j := i; j > 0 && diffs[j].Path < diffs[j-1].Path; j-- {
			diffs[j], diffs[j-1] = diffs[j-1], diffs[j]
		}
	}
}
