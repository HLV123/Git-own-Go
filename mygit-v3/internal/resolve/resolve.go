package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// Commit resolves any ref-like string to a full 40-char commit hash.
// Supports: HEAD, HEAD~N, HEAD^, branch names, tag names, short hashes, full hashes.
func Commit(r *repo.Repo, s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("empty ref")
	}

	// Handle HEAD~N and HEAD^N and branch~N etc.
	if idx := strings.IndexAny(s, "~^"); idx >= 0 {
		base := s[:idx]
		rest := s[idx:]
		baseHash, err := Commit(r, base)
		if err != nil {
			return "", err
		}
		return walkAncestors(r, baseHash, rest)
	}

	// HEAD
	if s == "HEAD" {
		return resolveHEAD(r)
	}

	// Full 40-char hash
	if len(s) == 40 {
		if _, err := object.ReadType(r, s); err != nil {
			return "", fmt.Errorf("object %s not found", s)
		}
		return s, nil
	}

	// Short hash (4-39 chars, all hex)
	if len(s) >= 4 && len(s) < 40 && isHex(s) {
		full, err := expandShortHash(r, s)
		if err == nil {
			return full, nil
		}
	}

	// Branch ref
	if h := readRefFile(r, "refs/heads/"+s); h != "" {
		return h, nil
	}

	// Tag ref
	if h := readRefFile(r, "refs/tags/"+s); h != "" {
		// Dereference annotated tags
		objType, content, err := object.ReadRaw(r, h)
		if err == nil && objType == object.TypeTag {
			return parseTagObject(content), nil
		}
		return h, nil
	}

	return "", fmt.Errorf("cannot resolve %q to a commit", s)
}

// walkAncestors follows ~N and ^ operators from a starting hash.
// rest is the remaining string after the base, e.g. "~2" or "^1^2"
func walkAncestors(r *repo.Repo, hash, rest string) (string, error) {
	cur := hash
	for len(rest) > 0 {
		switch rest[0] {
		case '~':
			rest = rest[1:]
			n := 1
			// Parse optional number
			numStr := ""
			for len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
				numStr += string(rest[0])
				rest = rest[1:]
			}
			if numStr != "" {
				parsed, err := strconv.Atoi(numStr)
				if err != nil {
					return "", err
				}
				n = parsed
			}
			for i := 0; i < n; i++ {
				parent, err := firstParent(r, cur)
				if err != nil {
					return "", fmt.Errorf("HEAD~%d: %w", i+1, err)
				}
				cur = parent
			}
		case '^':
			rest = rest[1:]
			n := 1
			numStr := ""
			for len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
				numStr += string(rest[0])
				rest = rest[1:]
			}
			if numStr != "" {
				parsed, err := strconv.Atoi(numStr)
				if err != nil {
					return "", err
				}
				n = parsed
			}
			if n == 0 {
				// ^0 means the commit itself
				continue
			}
			parent, err := nthParent(r, cur, n)
			if err != nil {
				return "", err
			}
			cur = parent
		default:
			return "", fmt.Errorf("unexpected character %q in ref", rest[0])
		}
	}
	return cur, nil
}

func firstParent(r *repo.Repo, hash string) (string, error) {
	return nthParent(r, hash, 1)
}

func nthParent(r *repo.Repo, hash string, n int) (string, error) {
	_, content, err := object.ReadRaw(r, hash)
	if err != nil {
		return "", err
	}
	commit, err := object.ParseCommit(content)
	if err != nil {
		return "", err
	}
	if n < 1 || n > len(commit.Parents) {
		return "", fmt.Errorf("commit %s has no parent #%d (has %d parents)", hash[:7], n, len(commit.Parents))
	}
	return commit.Parents[n-1], nil
}

// expandShortHash finds the full hash matching a short prefix.
func expandShortHash(r *repo.Repo, prefix string) (string, error) {
	prefix = strings.ToLower(prefix)
	if len(prefix) < 2 {
		return "", fmt.Errorf("short hash too short")
	}
	dir := filepath.Join(r.ObjectsDir(), prefix[:2])
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no object with prefix %s", prefix)
	}

	suffix := prefix[2:]
	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), suffix) {
			matches = append(matches, prefix[:2]+e.Name())
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no object with prefix %s", prefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous short hash %s (%d matches)", prefix, len(matches))
	}
	return matches[0], nil
}

func resolveHEAD(r *repo.Repo) (string, error) {
	data, err := os.ReadFile(r.Path("HEAD"))
	if err != nil {
		return "", err
	}
	head := strings.TrimSpace(string(data))
	if strings.HasPrefix(head, "ref: ") {
		ref := strings.TrimPrefix(head, "ref: ")
		h := readRefFile(r, ref)
		if h == "" {
			return "", fmt.Errorf("HEAD points to unborn branch")
		}
		return h, nil
	}
	if object.ValidateHash(head) {
		return head, nil
	}
	return "", fmt.Errorf("cannot resolve HEAD")
}

func readRefFile(r *repo.Repo, ref string) string {
	path := r.Path(filepath.FromSlash(ref))
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func parseTagObject(content []byte) string {
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "object ") {
			return strings.TrimPrefix(line, "object ")
		}
	}
	return ""
}
