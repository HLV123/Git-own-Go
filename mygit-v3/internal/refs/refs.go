package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/mygit/internal/repo"
)

// ReadHEAD returns either "ref: refs/heads/<branch>" or a 40-char hash.
func ReadHEAD(r *repo.Repo) (string, error) {
	data, err := os.ReadFile(r.Path("HEAD"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ResolveHEAD returns the commit hash HEAD points to, or "" for unborn branch.
func ResolveHEAD(r *repo.Repo) (string, error) {
	head, err := ReadHEAD(r)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(head, "ref: ") {
		refName := strings.TrimPrefix(head, "ref: ")
		return ReadRef(r, refName)
	}
	// Detached HEAD
	return head, nil
}

// ReadRef reads a ref file (e.g. "refs/heads/main") and returns the hash.
// Returns "" if ref doesn't exist yet (unborn branch).
func ReadRef(r *repo.Repo, refName string) (string, error) {
	path := r.Path(filepath.FromSlash(refName))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // unborn
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteRef writes a commit hash to a ref file.
func WriteRef(r *repo.Repo, refName string, hash string) error {
	path := r.Path(filepath.FromSlash(refName))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(hash+"\n"), 0644)
}

// CurrentBranch returns the branch name if HEAD is symbolic, or "" if detached.
func CurrentBranch(r *repo.Repo) (string, error) {
	head, err := ReadHEAD(r)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimPrefix(head, "ref: refs/heads/"), nil
	}
	return "", nil // detached
}

// UpdateHEAD updates HEAD to point at the given branch name.
func UpdateHEADBranch(r *repo.Repo, branch string) error {
	return os.WriteFile(r.Path("HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0644)
}

// UpdateHEADDetached sets HEAD to a direct commit hash (detached).
func UpdateHEADDetached(r *repo.Repo, hash string) error {
	return os.WriteFile(r.Path("HEAD"), []byte(hash+"\n"), 0644)
}

// AdvanceHEAD updates the branch ref that HEAD points to.
func AdvanceHEAD(r *repo.Repo, hash string) error {
	head, err := ReadHEAD(r)
	if err != nil {
		return err
	}
	if strings.HasPrefix(head, "ref: ") {
		refName := strings.TrimPrefix(head, "ref: ")
		return WriteRef(r, refName, hash)
	}
	// Detached HEAD: update HEAD directly
	return UpdateHEADDetached(r, hash)
}

// ListBranches returns all branch names under refs/heads/.
func ListBranches(r *repo.Repo) ([]string, error) {
	dir := r.Path("refs", "heads")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, e := range entries {
		if !e.IsDir() {
			branches = append(branches, e.Name())
		}
	}
	return branches, nil
}

// CreateBranch creates a new branch pointing to the given commit.
func CreateBranch(r *repo.Repo, name string, hash string) error {
	refName := "refs/heads/" + name
	existing, err := ReadRef(r, refName)
	if err != nil {
		return err
	}
	if existing != "" {
		return fmt.Errorf("branch %q already exists", name)
	}
	return WriteRef(r, refName, hash)
}
