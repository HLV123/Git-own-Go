package fsck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// Result holds fsck findings.
type Result struct {
	OK       []string
	Errors   []string
	Warnings []string
}

// Run walks the entire object store and verifies every object.
func Run(r *repo.Repo) (*Result, error) {
	res := &Result{}

	hashes, err := allObjectHashes(r)
	if err != nil {
		return nil, err
	}

	res.OK = append(res.OK, fmt.Sprintf("found %d objects", len(hashes)))

	// Verify each object
	reachable := map[string]bool{}

	for _, hash := range hashes {
		objType, content, err := object.ReadRaw(r, hash)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("corrupt object %s: %v", hash[:7], err))
			continue
		}

		switch objType {
		case object.TypeBlob:
			reachable[hash] = true

		case object.TypeTree:
			tree, err := object.ParseTree(content)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("malformed tree %s: %v", hash[:7], err))
				continue
			}
			reachable[hash] = true
			for _, e := range tree.Entries {
				if !object.ValidateHash(e.Hash) {
					res.Errors = append(res.Errors, fmt.Sprintf("tree %s has invalid hash for entry %q", hash[:7], e.Name))
				}
			}

		case object.TypeCommit:
			commit, err := object.ParseCommit(content)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("malformed commit %s: %v", hash[:7], err))
				continue
			}
			reachable[hash] = true
			if !object.ValidateHash(commit.Tree) {
				res.Errors = append(res.Errors, fmt.Sprintf("commit %s has invalid tree hash", hash[:7]))
			}
			for _, p := range commit.Parents {
				if !object.ValidateHash(p) {
					res.Errors = append(res.Errors, fmt.Sprintf("commit %s has invalid parent hash", hash[:7]))
				}
			}

		case object.TypeTag:
			reachable[hash] = true
		}
	}

	// Find dangling objects (not reachable from any ref)
	reachableFromRefs := map[string]bool{}
	refsDir := r.Path("refs")
	walkRefs(r, refsDir, reachableFromRefs)

	// Also check HEAD
	headData, err := os.ReadFile(r.Path("HEAD"))
	if err == nil {
		headHash := strings.TrimSpace(string(headData))
		if strings.HasPrefix(headHash, "ref: ") {
			// Symbolic ref — resolve
		} else if object.ValidateHash(headHash) {
			markReachable(r, headHash, reachableFromRefs)
		}
	}

	for _, hash := range hashes {
		if !reachableFromRefs[hash] {
			res.Warnings = append(res.Warnings, fmt.Sprintf("dangling object %s", hash[:7]))
		}
	}

	if len(res.Errors) == 0 {
		res.OK = append(res.OK, "no errors found")
	}

	return res, nil
}

// allObjectHashes returns all loose object hashes in the object store.
func allObjectHashes(r *repo.Repo) ([]string, error) {
	var hashes []string
	objDir := r.ObjectsDir()

	dirs, err := os.ReadDir(objDir)
	if err != nil {
		return nil, err
	}

	for _, d := range dirs {
		if !d.IsDir() || len(d.Name()) != 2 {
			continue
		}
		prefix := d.Name()
		subDir := filepath.Join(objDir, prefix)
		files, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() {
				hashes = append(hashes, prefix+f.Name())
			}
		}
	}
	return hashes, nil
}

// walkRefs collects all hashes referenced from refs directory.
func walkRefs(r *repo.Repo, dir string, seen map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			walkRefs(r, path, seen)
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			hash := strings.TrimSpace(string(data))
			if object.ValidateHash(hash) {
				markReachable(r, hash, seen)
			}
		}
	}
}

// markReachable marks a commit and all objects reachable from it.
func markReachable(r *repo.Repo, hash string, seen map[string]bool) {
	if seen[hash] || hash == "" {
		return
	}
	seen[hash] = true

	objType, content, err := object.ReadRaw(r, hash)
	if err != nil {
		return
	}

	switch objType {
	case object.TypeCommit:
		commit, err := object.ParseCommit(content)
		if err != nil {
			return
		}
		markReachable(r, commit.Tree, seen)
		for _, p := range commit.Parents {
			markReachable(r, p, seen)
		}
	case object.TypeTree:
		tree, err := object.ParseTree(content)
		if err != nil {
			return
		}
		for _, e := range tree.Entries {
			markReachable(r, e.Hash, seen)
		}
	}
}

// GC removes dangling (unreachable) objects and returns count deleted.
func GC(r *repo.Repo) (int, error) {
	hashes, err := allObjectHashes(r)
	if err != nil {
		return 0, err
	}

	reachable := map[string]bool{}
	refsDir := r.Path("refs")
	walkRefs(r, refsDir, reachable)

	// Also HEAD
	headData, _ := os.ReadFile(r.Path("HEAD"))
	if headData != nil {
		h := strings.TrimSpace(string(headData))
		if !strings.HasPrefix(h, "ref: ") && object.ValidateHash(h) {
			markReachable(r, h, reachable)
		}
	}

	deleted := 0
	for _, hash := range hashes {
		if !reachable[hash] {
			path := filepath.Join(r.ObjectsDir(), hash[:2], hash[2:])
			if err := os.Remove(path); err == nil {
				deleted++
			}
		}
	}
	return deleted, nil
}
