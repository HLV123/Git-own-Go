package merge

import (
	"fmt"
	"strings"

	"github.com/user/mygit/internal/diff"
	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// Result holds the outcome of a merge operation.
type Result struct {
	FastForward bool
	Conflicts   []string // paths with conflicts
	MergeCommit string   // hash of new merge commit (if created)
}

// FindLCA finds the Lowest Common Ancestor of two commits using BFS.
func FindLCA(r *repo.Repo, hashA, hashB string) (string, error) {
	// Collect all ancestors of A (including A)
	ancestorsA := map[string]int{} // hash -> depth
	queue := []string{hashA}
	depth := 0
	for len(queue) > 0 {
		next := []string{}
		for _, h := range queue {
			if _, seen := ancestorsA[h]; seen {
				continue
			}
			ancestorsA[h] = depth
			_, content, err := object.ReadRaw(r, h)
			if err != nil {
				return "", err
			}
			commit, err := object.ParseCommit(content)
			if err != nil {
				return "", err
			}
			next = append(next, commit.Parents...)
		}
		queue = next
		depth++
	}

	// BFS from B, find first node in ancestorsA
	queue = []string{hashB}
	visited := map[string]bool{}
	for len(queue) > 0 {
		next := []string{}
		for _, h := range queue {
			if visited[h] {
				continue
			}
			visited[h] = true
			if _, inA := ancestorsA[h]; inA {
				return h, nil
			}
			_, content, err := object.ReadRaw(r, h)
			if err != nil {
				return "", err
			}
			commit, err := object.ParseCommit(content)
			if err != nil {
				return "", err
			}
			next = append(next, commit.Parents...)
		}
		queue = next
	}
	return "", fmt.Errorf("no common ancestor found between %s and %s", hashA[:7], hashB[:7])
}

// IsAncestor returns true if maybeAncestor is an ancestor of commitHash.
func IsAncestor(r *repo.Repo, maybeAncestor, commitHash string) (bool, error) {
	if maybeAncestor == commitHash {
		return true, nil
	}
	visited := map[string]bool{}
	queue := []string{commitHash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if visited[h] {
			continue
		}
		visited[h] = true
		if h == maybeAncestor {
			return true, nil
		}
		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			return false, err
		}
		commit, err := object.ParseCommit(content)
		if err != nil {
			return false, err
		}
		queue = append(queue, commit.Parents...)
	}
	return false, nil
}

// ThreeWayMerge merges theirs into ours using base as common ancestor.
// Returns merged file map and list of conflict paths.
func ThreeWayMerge(r *repo.Repo, baseTree, oursTree, theirsTree string) (map[string]string, []string, error) {
	baseFiles := map[string]string{}
	oursFiles := map[string]string{}
	theirsFiles := map[string]string{}

	if baseTree != "" {
		if err := walkTree(r, baseTree, "", baseFiles); err != nil {
			return nil, nil, fmt.Errorf("reading base tree: %w", err)
		}
	}
	if err := walkTree(r, oursTree, "", oursFiles); err != nil {
		return nil, nil, fmt.Errorf("reading ours tree: %w", err)
	}
	if err := walkTree(r, theirsTree, "", theirsFiles); err != nil {
		return nil, nil, fmt.Errorf("reading theirs tree: %w", err)
	}

	// Collect all paths
	allPaths := map[string]bool{}
	for p := range oursFiles {
		allPaths[p] = true
	}
	for p := range theirsFiles {
		allPaths[p] = true
	}
	for p := range baseFiles {
		allPaths[p] = true
	}

	result := map[string]string{}
	var conflicts []string

	for path := range allPaths {
		base := baseFiles[path]
		ours := oursFiles[path]
		theirs := theirsFiles[path]

		switch {
		case ours == theirs:
			// Both same (or both deleted) — no change needed
			if ours != "" {
				result[path] = ours
			}
		case base == ours && ours != theirs:
			// Only theirs changed — take theirs
			if theirs != "" {
				result[path] = theirs
			}
		case base == theirs && ours != theirs:
			// Only ours changed — keep ours
			if ours != "" {
				result[path] = ours
			}
		default:
			// Both changed differently — CONFLICT
			conflicts = append(conflicts, path)
			// Write conflict markers into a new blob
			conflictContent, err := makeConflictBlob(r, path, ours, theirs)
			if err == nil {
				hash, werr := object.WriteObject(r, object.TypeBlob, []byte(conflictContent))
				if werr == nil {
					result[path] = hash
				}
			}
		}
	}

	return result, conflicts, nil
}

// makeConflictBlob creates a blob with conflict markers.
func makeConflictBlob(r *repo.Repo, path, oursHash, theirsHash string) (string, error) {
	oursContent := ""
	theirsContent := ""

	if oursHash != "" {
		_, content, err := object.ReadRaw(r, oursHash)
		if err == nil {
			oursContent = string(content)
		}
	}
	if theirsHash != "" {
		_, content, err := object.ReadRaw(r, theirsHash)
		if err == nil {
			theirsContent = string(content)
		}
	}

	return fmt.Sprintf(
		"<<<<<<< HEAD\n%s=======\n%s>>>>>>> theirs\n",
		oursContent, theirsContent,
	), nil
}

// walkTree recursively flattens a tree into path->hash map.
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
		p := e.Name
		if prefix != "" {
			p = prefix + "/" + e.Name
		}
		if e.IsDir() {
			if err := walkTree(r, e.Hash, p, files); err != nil {
				return err
			}
		} else {
			files[p] = e.Hash
		}
	}
	return nil
}

// BuildTreeFromMap builds tree objects from a flat path->blobHash map.
func BuildTreeFromMap(r *repo.Repo, files map[string]string) (string, error) {
	return buildDir(r, files, "")
}

func buildDir(r *repo.Repo, files map[string]string, prefix string) (string, error) {
	dirs := map[string]bool{}
	var entries []object.TreeEntry

	for path, hash := range files {
		rel := path
		if prefix != "" {
			if !strings.HasPrefix(path, prefix+"/") {
				continue
			}
			rel = path[len(prefix)+1:]
		}

		slash := strings.Index(rel, "/")
		if slash < 0 {
			// Direct file
			entries = append(entries, object.TreeEntry{
				Mode: "100644",
				Name: rel,
				Hash: hash,
			})
		} else {
			dirName := rel[:slash]
			if dirs[dirName] {
				continue
			}
			dirs[dirName] = true

			subPrefix := dirName
			if prefix != "" {
				subPrefix = prefix + "/" + dirName
			}
			subHash, err := buildDir(r, files, subPrefix)
			if err != nil {
				return "", err
			}
			entries = append(entries, object.TreeEntry{
				Mode: "40000",
				Name: dirName,
				Hash: subHash,
			})
		}
	}

	tree := &object.Tree{Entries: entries}
	content, err := tree.Serialize()
	if err != nil {
		return "", err
	}
	return object.WriteObject(r, object.TypeTree, content)
}

// WalkTree is exported for use by other packages.
func WalkTree(r *repo.Repo, treeHash string, prefix string, files map[string]string) error {
	return walkTree(r, treeHash, prefix, files)
}

// DiffSummary returns a one-line summary of changes.
func DiffSummary(diffs []diff.FileDiff) string {
	added, modified, deleted := 0, 0, 0
	for _, d := range diffs {
		switch d.Type {
		case diff.Added:
			added++
		case diff.Modified:
			modified++
		case diff.Deleted:
			deleted++
		}
	}
	parts := []string{}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", deleted))
	}
	if len(parts) == 0 {
		return "nothing changed"
	}
	return strings.Join(parts, ", ")
}
