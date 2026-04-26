package object_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/repo"
)

// ─── Phase 1: Blob hash tests ─────────────────────────────────────────────────

// Test cases from spec — must match real Git output.
func TestHashEmptyBlob(t *testing.T) {
	got := object.HashHex(object.TypeBlob, []byte{})
	want := "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	if got != want {
		t.Errorf("empty blob hash = %q, want %q", got, want)
	}
}

func TestHashHelloBlob(t *testing.T) {
	// "hello\n" — same as echo "hello" | git hash-object --stdin
	got := object.HashHex(object.TypeBlob, []byte("hello\n"))
	want := "ce013625030ba8dba906f756967f9e9ca394464a"
	if got != want {
		t.Errorf("hello blob hash = %q, want %q", got, want)
	}
}

func TestHashHelloWorldBlob(t *testing.T) {
	// "hello world\n"
	got := object.HashHex(object.TypeBlob, []byte("hello world\n"))
	want := "3b18e512dba79e4c8300dd08aeb37f8e728b8dad"
	if got != want {
		t.Errorf("hello world blob hash = %q, want %q", got, want)
	}
}

// ─── Phase 1: Round-trip write/read ──────────────────────────────────────────

func TestBlobRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}

	content := []byte("hello world")
	hash, err := object.WriteObject(r, object.TypeBlob, content)
	if err != nil {
		t.Fatalf("WriteObject: %v", err)
	}

	gotType, gotContent, err := object.ReadRaw(r, hash)
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if gotType != object.TypeBlob {
		t.Errorf("type = %q, want blob", gotType)
	}
	if string(gotContent) != string(content) {
		t.Errorf("content = %q, want %q", gotContent, content)
	}
}

func TestBlobWriteIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}

	content := []byte("idempotent test")
	h1, err := object.WriteObject(r, object.TypeBlob, content)
	if err != nil {
		t.Fatalf("first WriteObject: %v", err)
	}
	h2, err := object.WriteObject(r, object.TypeBlob, content)
	if err != nil {
		t.Fatalf("second WriteObject: %v", err)
	}
	if h1 != h2 {
		t.Errorf("idempotency failed: %q != %q", h1, h2)
	}
}

func TestReadObjectNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}
	_, _, err = object.ReadRaw(r, "0000000000000000000000000000000000000000")
	if err == nil {
		t.Error("expected error reading non-existent object, got nil")
	}
}

func TestCatFileType(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}
	hash, _ := object.WriteObject(r, object.TypeBlob, []byte("type test"))
	got, err := object.ReadType(r, hash)
	if err != nil {
		t.Fatalf("ReadType: %v", err)
	}
	if got != object.TypeBlob {
		t.Errorf("type = %q, want blob", got)
	}
}

func TestCatFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}
	content := []byte("size test content")
	hash, _ := object.WriteObject(r, object.TypeBlob, content)
	got, err := object.ReadSize(r, hash)
	if err != nil {
		t.Fatalf("ReadSize: %v", err)
	}
	if got != len(content) {
		t.Errorf("size = %d, want %d", got, len(content))
	}
}

// ─── Phase 2: Tree serialization/parsing ─────────────────────────────────────

func TestTreeSerializeParseRoundTrip(t *testing.T) {
	entries := []object.TreeEntry{
		{Mode: "100644", Name: "hello.txt", Hash: "ce013625030ba8dba906f756967f9e9ca394464a"},
		{Mode: "100644", Name: "README.md", Hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
	}
	tree := &object.Tree{Entries: entries}
	content, err := tree.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := object.ParseTree(content)
	if err != nil {
		t.Fatalf("ParseTree: %v", err)
	}

	if len(parsed.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed.Entries))
	}
}

func TestTreeSortOrder(t *testing.T) {
	// Entries added in wrong order — Serialize must sort them
	entries := []object.TreeEntry{
		{Mode: "100644", Name: "z.txt", Hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{Mode: "100644", Name: "a.txt", Hash: "ce013625030ba8dba906f756967f9e9ca394464a"},
	}
	tree := &object.Tree{Entries: entries}
	content, err := tree.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := object.ParseTree(content)
	if err != nil {
		t.Fatalf("ParseTree: %v", err)
	}

	if parsed.Entries[0].Name != "a.txt" {
		t.Errorf("expected entries[0].Name = a.txt, got %q", parsed.Entries[0].Name)
	}
	if parsed.Entries[1].Name != "z.txt" {
		t.Errorf("expected entries[1].Name = z.txt, got %q", parsed.Entries[1].Name)
	}
}

func TestTreeSortDirVsFile(t *testing.T) {
	// Git rule: "hello/" (dir) should sort after "hello.c" (file)
	// because '/' (0x2F) > '.' (0x2E)
	entries := []object.TreeEntry{
		{Mode: "40000", Name: "hello", Hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{Mode: "100644", Name: "hello.c", Hash: "ce013625030ba8dba906f756967f9e9ca394464a"},
	}
	tree := &object.Tree{Entries: entries}
	content, err := tree.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	parsed, err := object.ParseTree(content)
	if err != nil {
		t.Fatalf("ParseTree: %v", err)
	}

	// "hello.c" should come before "hello/" (dir)
	if parsed.Entries[0].Name != "hello.c" {
		t.Errorf("expected hello.c first (before hello/), got %q first", parsed.Entries[0].Name)
	}
}

func TestTreeDeterministic(t *testing.T) {
	// Same content built twice must yield same hash
	tmpDir := t.TempDir()
	r, _ := repo.Init(tmpDir)

	makeTree := func() string {
		entries := []object.TreeEntry{
			{Mode: "100644", Name: "b.txt", Hash: "ce013625030ba8dba906f756967f9e9ca394464a"},
			{Mode: "100644", Name: "a.txt", Hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		}
		tree := &object.Tree{Entries: entries}
		content, _ := tree.Serialize()
		hash, _ := object.WriteObject(r, object.TypeTree, content)
		return hash
	}

	h1 := makeTree()
	h2 := makeTree()
	if h1 != h2 {
		t.Errorf("tree hash not deterministic: %q != %q", h1, h2)
	}
}

func TestTreeRawHashBytes(t *testing.T) {
	// Verify tree entries use 20 raw bytes (not 40 hex chars)
	hash40 := "ce013625030ba8dba906f756967f9e9ca394464a"
	entries := []object.TreeEntry{
		{Mode: "100644", Name: "test.txt", Hash: hash40},
	}
	tree := &object.Tree{Entries: entries}
	content, err := tree.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// "100644 test.txt\0" = 16 bytes, then 20 raw bytes = 36 total
	expectedLen := len("100644") + 1 + len("test.txt") + 1 + 20
	if len(content) != expectedLen {
		t.Errorf("serialized length = %d, want %d (20 raw bytes, not 40 hex)", len(content), expectedLen)
	}

	// Verify the 20 bytes decode back to the original hash
	rawHash := content[expectedLen-20:]
	if hex.EncodeToString(rawHash) != hash40 {
		t.Errorf("raw hash bytes don't match original hash")
	}
}

// ─── Phase 2: write-tree integration test ───────────────────────────────────

func TestWriteTreeFromFiles(t *testing.T) {
	tmpDir := t.TempDir()
	r, err := repo.Init(tmpDir)
	if err != nil {
		t.Fatalf("repo.Init: %v", err)
	}

	// Create a file and hash it
	filePath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fileHash, err := object.WriteObject(r, object.TypeBlob, []byte("hello\n"))
	if err != nil {
		t.Fatalf("WriteObject blob: %v", err)
	}

	// Build a tree manually
	tree := &object.Tree{
		Entries: []object.TreeEntry{
			{Mode: "100644", Name: "a.txt", Hash: fileHash},
		},
	}
	content, err := tree.Serialize()
	if err != nil {
		t.Fatalf("tree.Serialize: %v", err)
	}
	treeHash, err := object.WriteObject(r, object.TypeTree, content)
	if err != nil {
		t.Fatalf("WriteObject tree: %v", err)
	}

	// Read back and verify
	_, rawContent, err := object.ReadRaw(r, treeHash)
	if err != nil {
		t.Fatalf("ReadRaw tree: %v", err)
	}
	parsed, err := object.ParseTree(rawContent)
	if err != nil {
		t.Fatalf("ParseTree: %v", err)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Name != "a.txt" {
		t.Errorf("unexpected tree entries: %+v", parsed.Entries)
	}
}

// ─── Phase 3: Commit object ──────────────────────────────────────────────────

func TestCommitSerializeParseRoundTrip(t *testing.T) {
	c := &object.Commit{
		Tree:      "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		Parents:   []string{},
		Author:    "Test User <test@example.com> 1609459200 +0000",
		Committer: "Test User <test@example.com> 1609459200 +0000",
		Message:   "initial commit",
	}

	content, err := c.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := object.ParseCommit(content)
	if err != nil {
		t.Fatalf("ParseCommit: %v", err)
	}

	if parsed.Tree != c.Tree {
		t.Errorf("tree = %q, want %q", parsed.Tree, c.Tree)
	}
	if parsed.Message != c.Message+"\n" && parsed.Message != c.Message {
		t.Errorf("message = %q, want %q", parsed.Message, c.Message)
	}
}

func TestCommitInitialHasNoParent(t *testing.T) {
	c := &object.Commit{
		Tree:      "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		Parents:   nil,
		Author:    "Dev <dev@example.com> 1609459200 +0000",
		Committer: "Dev <dev@example.com> 1609459200 +0000",
		Message:   "initial",
	}

	content, err := c.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	// Must NOT contain "parent " line
	s := string(content)
	if contains(s, "parent ") {
		t.Errorf("initial commit must not contain a parent line, got:\n%s", s)
	}
}

func TestCommitWithParent(t *testing.T) {
	c := &object.Commit{
		Tree:      "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		Parents:   []string{"ce013625030ba8dba906f756967f9e9ca394464a"},
		Author:    "Dev <dev@example.com> 1609459200 +0000",
		Committer: "Dev <dev@example.com> 1609459200 +0000",
		Message:   "second",
	}

	content, err := c.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := object.ParseCommit(content)
	if err != nil {
		t.Fatalf("ParseCommit: %v", err)
	}

	if len(parsed.Parents) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(parsed.Parents))
	}
	if parsed.Parents[0] != c.Parents[0] {
		t.Errorf("parent = %q, want %q", parsed.Parents[0], c.Parents[0])
	}
}

func TestCommitMergeHasTwoParents(t *testing.T) {
	c := &object.Commit{
		Tree: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		Parents: []string{
			"ce013625030ba8dba906f756967f9e9ca394464a",
			"3b18e512dba79e4c8300dd08aeb37f8e728b8dad",
		},
		Author:    "Dev <dev@example.com> 1609459200 +0000",
		Committer: "Dev <dev@example.com> 1609459200 +0000",
		Message:   "merge commit",
	}

	content, err := c.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	parsed, err := object.ParseCommit(content)
	if err != nil {
		t.Fatalf("ParseCommit: %v", err)
	}

	if len(parsed.Parents) != 2 {
		t.Fatalf("expected 2 parents, got %d", len(parsed.Parents))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
