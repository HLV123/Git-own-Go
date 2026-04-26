package repo

import (
	"errors"
	"os"
	"path/filepath"
)

const DotDir = ".mygit"

// Repo holds the root path of the working directory.
type Repo struct {
	Root string // absolute path to working directory
}

// GitDir returns the absolute path to .mygit/.
func (r *Repo) GitDir() string {
	return filepath.Join(r.Root, DotDir)
}

// Path returns a path inside .mygit/.
func (r *Repo) Path(parts ...string) string {
	return filepath.Join(append([]string{r.GitDir()}, parts...)...)
}

// ObjectsDir returns .mygit/objects/.
func (r *Repo) ObjectsDir() string {
	return r.Path("objects")
}

// Init creates the .mygit directory structure.
func Init(root string) (*Repo, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	r := &Repo{Root: absRoot}

	dirs := []string{
		r.GitDir(),
		r.Path("objects"),
		r.Path("refs", "heads"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	// Write HEAD pointing to main branch
	headPath := r.Path("HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		return nil, err
	}

	return r, nil
}

// Open finds the repo root by walking up from cwd, returns error if not found.
func Open(start string) (*Repo, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, DotDir)); err == nil {
			return &Repo{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, errors.New("not a mygit repository (no .mygit directory found)")
}

// OpenCwd opens the repo from the current working directory.
func OpenCwd() (*Repo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return Open(cwd)
}
