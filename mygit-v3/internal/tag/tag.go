package tag

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// Tag represents a tag (lightweight or annotated).
type Tag struct {
	Name       string
	CommitHash string // for lightweight tags
	ObjectHash string // hash of tag object (for annotated)
	Message    string // empty for lightweight
	Annotated  bool
}

// tagsDir returns .mygit/refs/tags/
func tagsDir(r *repo.Repo) string {
	return r.Path("refs", "tags")
}

// CreateLightweight creates a tag pointing directly to a commit.
func CreateLightweight(r *repo.Repo, name, commitHash string) error {
	if err := os.MkdirAll(tagsDir(r), 0755); err != nil {
		return err
	}
	path := filepath.Join(tagsDir(r), name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("tag %q already exists", name)
	}
	return os.WriteFile(path, []byte(commitHash+"\n"), 0644)
}

// CreateAnnotated creates an annotated tag object.
func CreateAnnotated(r *repo.Repo, name, commitHash, tagger, message string) error {
	if err := os.MkdirAll(tagsDir(r), 0755); err != nil {
		return err
	}

	// Build tag object content
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "object %s\n", commitHash)
	fmt.Fprintf(&buf, "type commit\n")
	fmt.Fprintf(&buf, "tag %s\n", name)
	fmt.Fprintf(&buf, "tagger %s\n", tagger)
	fmt.Fprintf(&buf, "\n%s\n", message)

	hash, err := object.WriteObject(r, object.TypeTag, buf.Bytes())
	if err != nil {
		return err
	}

	path := filepath.Join(tagsDir(r), name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("tag %q already exists", name)
	}
	return os.WriteFile(path, []byte(hash+"\n"), 0644)
}

// List returns all tags.
func List(r *repo.Repo) ([]Tag, error) {
	dir := tagsDir(r)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hash := strings.TrimSpace(string(data))

		t := Tag{Name: e.Name()}

		// Check if it's an annotated tag (points to tag object)
		objType, content, err := object.ReadRaw(r, hash)
		if err == nil && objType == object.TypeTag {
			t.Annotated = true
			t.ObjectHash = hash
			t.CommitHash, t.Message = parseTagObject(content)
		} else {
			t.CommitHash = hash
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// Resolve returns the commit hash a tag points to.
func Resolve(r *repo.Repo, name string) (string, error) {
	path := filepath.Join(tagsDir(r), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("tag %q not found", name)
	}
	hash := strings.TrimSpace(string(data))

	// If annotated, dereference to commit
	objType, content, err := object.ReadRaw(r, hash)
	if err == nil && objType == object.TypeTag {
		commitHash, _ := parseTagObject(content)
		return commitHash, nil
	}
	return hash, nil
}

// Delete removes a tag.
func Delete(r *repo.Repo, name string) error {
	path := filepath.Join(tagsDir(r), name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("tag %q not found", name)
	}
	return os.Remove(path)
}

func parseTagObject(content []byte) (commitHash, message string) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	pastHeader := false
	var msgLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if !pastHeader {
			if line == "" {
				pastHeader = true
				continue
			}
			if strings.HasPrefix(line, "object ") {
				commitHash = strings.TrimPrefix(line, "object ")
			}
		} else {
			msgLines = append(msgLines, line)
		}
	}
	message = strings.Join(msgLines, "\n")
	return
}
