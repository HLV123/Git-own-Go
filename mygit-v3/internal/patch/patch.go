package patch

import (
	"fmt"
	"strings"
)

// Hunk represents one diff hunk between old and new content.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []HunkLine
}

// HunkLine is one line in a hunk.
type HunkLine struct {
	Op      byte // ' ' context, '+' added, '-' removed
	Content string
}

// ParseHunks splits a unified diff into hunks.
func ParseHunks(diff string) []Hunk {
	var hunks []Hunk
	var cur *Hunk

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			h := parseHunkHeader(line)
			cur = &h
			continue
		}
		if cur == nil {
			continue
		}
		if len(line) == 0 {
			continue
		}
		op := line[0]
		if op == ' ' || op == '+' || op == '-' {
			cur.Lines = append(cur.Lines, HunkLine{Op: op, Content: line[1:]})
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	return hunks
}

func parseHunkHeader(line string) Hunk {
	// @@ -oldStart,oldCount +newStart,newCount @@
	var h Hunk
	fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &h.OldStart, &h.OldCount, &h.NewStart, &h.NewCount)
	return h
}

// ApplyHunk applies a single hunk to content lines, returns new content.
func ApplyHunk(lines []string, hunk Hunk) ([]string, error) {
	result := make([]string, 0, len(lines)+10)

	// Lines before hunk (0-indexed)
	start := hunk.OldStart - 1
	if start < 0 {
		start = 0
	}
	result = append(result, lines[:start]...)

	oldIdx := start
	for _, hl := range hunk.Lines {
		switch hl.Op {
		case ' ':
			// Context line — must match
			if oldIdx < len(lines) {
				result = append(result, lines[oldIdx])
				oldIdx++
			}
		case '+':
			result = append(result, hl.Content)
		case '-':
			oldIdx++ // skip old line
		}
	}

	// Lines after hunk
	result = append(result, lines[oldIdx:]...)
	return result, nil
}

// ApplyPatch applies a full set of hunks to file content.
func ApplyPatch(content string, hunks []Hunk) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var err error
	for _, h := range hunks {
		lines, err = ApplyHunk(lines, h)
		if err != nil {
			return "", err
		}
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// GenerateUnifiedDiff generates a simple unified diff between old and new content.
func GenerateUnifiedDiff(oldLines, newLines []string, context int) []Hunk {
	// Use LCS to find matching lines
	lcs := lcsIdx(oldLines, newLines)

	type match struct{ o, n int }
	var matches []match
	i, j := 0, 0
	for _, m := range lcs {
		matches = append(matches, match{m[0], m[1]})
		i = m[0] + 1
		j = m[1] + 1
	}
	_ = i
	_ = j

	if len(matches) == 0 && len(oldLines) == 0 && len(newLines) == 0 {
		return nil
	}

	// Build hunks from non-matching regions
	var hunks []Hunk
	oi, ni := 0, 0

	for oi < len(oldLines) || ni < len(newLines) {
		// Find next diff
		if oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == newLines[ni] {
			oi++
			ni++
			continue
		}
		// Start a hunk
		hunk := Hunk{OldStart: oi + 1, NewStart: ni + 1}

		// Add context before
		ctxStart := oi - context
		if ctxStart < 0 {
			ctxStart = 0
		}
		for k := ctxStart; k < oi; k++ {
			hunk.Lines = append(hunk.Lines, HunkLine{Op: ' ', Content: oldLines[k]})
		}

		// Collect changes
		for oi < len(oldLines) || ni < len(newLines) {
			if oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == newLines[ni] {
				// Check if we're past context distance from last change
				break
			}
			if oi < len(oldLines) {
				hunk.Lines = append(hunk.Lines, HunkLine{Op: '-', Content: oldLines[oi]})
				oi++
			}
			if ni < len(newLines) {
				hunk.Lines = append(hunk.Lines, HunkLine{Op: '+', Content: newLines[ni]})
				ni++
			}
		}

		// Add context after
		for k := 0; k < context && oi < len(oldLines); k++ {
			hunk.Lines = append(hunk.Lines, HunkLine{Op: ' ', Content: oldLines[oi]})
			oi++
			ni++
		}

		hunk.OldCount = countOp(hunk.Lines, '-', ' ')
		hunk.NewCount = countOp(hunk.Lines, '+', ' ')
		hunks = append(hunks, hunk)
	}
	return hunks
}

func countOp(lines []HunkLine, ops ...byte) int {
	n := 0
	for _, l := range lines {
		for _, op := range ops {
			if l.Op == op {
				n++
			}
		}
	}
	return n
}

// lcsIdx returns indices of LCS matches as [oldIdx, newIdx] pairs.
func lcsIdx(a, b []string) [][2]int {
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
	var result [][2]int
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([][2]int{{i - 1, j - 1}}, result...)
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

// FormatHunk formats a hunk as unified diff text.
func FormatHunk(h Hunk) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	for _, l := range h.Lines {
		sb.WriteByte(l.Op)
		sb.WriteString(l.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}
