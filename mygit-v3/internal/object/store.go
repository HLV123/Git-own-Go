package object

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/mygit/internal/repo"
)

// objectPath returns the loose object path for a given hex hash.
func objectPath(r *repo.Repo, hash string) string {
	return filepath.Join(r.ObjectsDir(), hash[:2], hash[2:])
}

// WriteObject computes the hash and writes the object to the store.
// Returns the hex hash.
func WriteObject(r *repo.Repo, t Type, content []byte) (string, error) {
	_, hexHash := HashBytes(t, content)

	path := objectPath(r, hexHash)
	if _, err := os.Stat(path); err == nil {
		// Already exists — idempotent
		return hexHash, nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	// Build full data: header + content
	header := Header(t, len(content))
	full := append(header, content...)

	// zlib compress
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(full); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	if err := os.WriteFile(path, buf.Bytes(), 0444); err != nil {
		return "", err
	}
	return hexHash, nil
}

// ReadRaw decompresses a loose object and returns (type, content, error).
func ReadRaw(r *repo.Repo, hash string) (Type, []byte, error) {
	if !ValidateHash(hash) {
		return "", nil, fmt.Errorf("invalid hash: %q", hash)
	}

	path := objectPath(r, hash)
	compressed, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("object %s not found", hash)
		}
		return "", nil, err
	}

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", nil, fmt.Errorf("zlib open failed for %s: %w", hash, err)
	}
	defer zr.Close()

	raw, err := io.ReadAll(zr)
	if err != nil {
		return "", nil, fmt.Errorf("zlib read failed for %s: %w", hash, err)
	}

	// Parse header: "<type> <size>\0<content>"
	nul := bytes.IndexByte(raw, 0x00)
	if nul < 0 {
		return "", nil, fmt.Errorf("malformed object %s: no null byte in header", hash)
	}
	headerStr := string(raw[:nul])
	parts := strings.SplitN(headerStr, " ", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("malformed object header: %q", headerStr)
	}
	objType := Type(parts[0])
	expectedSize, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", nil, fmt.Errorf("malformed object size in header: %q", parts[1])
	}

	content := raw[nul+1:]
	if len(content) != expectedSize {
		return "", nil, fmt.Errorf("object %s size mismatch: header says %d, got %d", hash, expectedSize, len(content))
	}

	return objType, content, nil
}

// ReadType returns only the type of an object.
func ReadType(r *repo.Repo, hash string) (Type, error) {
	t, _, err := ReadRaw(r, hash)
	return t, err
}

// ReadSize returns only the content size of an object.
func ReadSize(r *repo.Repo, hash string) (int, error) {
	_, content, err := ReadRaw(r, hash)
	if err != nil {
		return 0, err
	}
	return len(content), nil
}
