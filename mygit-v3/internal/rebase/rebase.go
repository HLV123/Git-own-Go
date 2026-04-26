package rebase

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/mygit/internal/diff"
	"github.com/user/mygit/internal/merge"
	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/patch"
	"github.com/user/mygit/internal/repo"
)

// RebaseState holds in-progress rebase information.
type RebaseState struct {
	OrigBranch string   // branch being rebased
	OntoHash   string   // target base commit
	Remaining  []string // commit hashes yet to be applied (oldest first)
	Done       []string // already applied
}

// statePath returns .mygit/rebase-merge/
func statePath(r *repo.Repo) string {
	return r.Path("rebase-merge")
}

// IsInProgress returns true if a rebase is currently in progress.
func IsInProgress(r *repo.Repo) bool {
	_, err := os.Stat(statePath(r))
	return err == nil
}

// SaveState persists rebase state to disk.
func SaveState(r *repo.Repo, state *RebaseState) error {
	dir := statePath(r)
	os.MkdirAll(dir, 0755)
	write := func(name, content string) error {
		return os.WriteFile(filepath.Join(dir, name), []byte(content+"\n"), 0644)
	}
	if err := write("orig-branch", state.OrigBranch); err != nil {
		return err
	}
	if err := write("onto", state.OntoHash); err != nil {
		return err
	}
	if err := write("remaining", strings.Join(state.Remaining, "\n")); err != nil {
		return err
	}
	if err := write("done", strings.Join(state.Done, "\n")); err != nil {
		return err
	}
	return nil
}

// LoadState reads rebase state from disk.
func LoadState(r *repo.Repo) (*RebaseState, error) {
	dir := statePath(r)
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	remaining := []string{}
	for _, h := range strings.Split(read("remaining"), "\n") {
		if h != "" {
			remaining = append(remaining, h)
		}
	}
	done := []string{}
	for _, h := range strings.Split(read("done"), "\n") {
		if h != "" {
			done = append(done, h)
		}
	}
	return &RebaseState{
		OrigBranch: read("orig-branch"),
		OntoHash:   read("onto"),
		Remaining:  remaining,
		Done:       done,
	}, nil
}

// ClearState removes the rebase state directory.
func ClearState(r *repo.Repo) error {
	return os.RemoveAll(statePath(r))
}

// CommitsBetween returns commits reachable from tip but NOT from base,
// in chronological order (oldest first) — the commits to replay.
func CommitsBetween(r *repo.Repo, base, tip string) ([]string, error) {
	// BFS from tip, stop at base ancestors
	baseAncestors := map[string]bool{}
	if err := collectAncestors(r, base, baseAncestors); err != nil {
		return nil, err
	}

	var commits []string
	visited := map[string]bool{}
	queue := []string{tip}

	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if visited[h] || h == "" || baseAncestors[h] {
			continue
		}
		visited[h] = true
		commits = append(commits, h)

		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			return nil, err
		}
		c, err := object.ParseCommit(content)
		if err != nil {
			return nil, err
		}
		queue = append(queue, c.Parents...)
	}

	// Reverse to get chronological order
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

func collectAncestors(r *repo.Repo, hash string, seen map[string]bool) error {
	queue := []string{hash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if seen[h] || h == "" {
			continue
		}
		seen[h] = true
		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			return err
		}
		c, err := object.ParseCommit(content)
		if err != nil {
			return err
		}
		queue = append(queue, c.Parents...)
	}
	return nil
}

// CherryPick applies a single commit on top of the current HEAD.
// Returns the new commit hash.
func CherryPick(r *repo.Repo, pickHash string, currentHead string, identity string) (string, error) {
	// Get the diff introduced by pickHash vs its parent
	_, pickContent, err := object.ReadRaw(r, pickHash)
	if err != nil {
		return "", fmt.Errorf("cannot read commit %s: %w", pickHash[:7], err)
	}
	pickCommit, err := object.ParseCommit(pickContent)
	if err != nil {
		return "", err
	}

	// Get parent tree of the picked commit
	var pickParentTree string
	if len(pickCommit.Parents) > 0 {
		pickParentTree, err = getCommitTree(r, pickCommit.Parents[0])
		if err != nil {
			return "", err
		}
	}

	// Get current HEAD tree
	currentTree, err := getCommitTree(r, currentHead)
	if err != nil {
		return "", err
	}

	// Three-way merge: base=pickParent, ours=currentHEAD, theirs=pick
	mergedFiles, conflicts, err := merge.ThreeWayMerge(r, pickParentTree, currentTree, pickCommit.Tree)
	if err != nil {
		return "", err
	}
	if len(conflicts) > 0 {
		return "", fmt.Errorf("cherry-pick conflict in: %s", strings.Join(conflicts, ", "))
	}

	// Build new tree
	newTree, err := merge.BuildTreeFromMap(r, mergedFiles)
	if err != nil {
		return "", err
	}

	// Create new commit preserving original message
	msg := strings.TrimSpace(pickCommit.Message)
	newCommit := &object.Commit{
		Tree:      newTree,
		Parents:   []string{currentHead},
		Author:    pickCommit.Author, // preserve original author
		Committer: identity,          // committer = person doing the cherry-pick
		Message:   msg,
	}
	content, err := newCommit.Serialize()
	if err != nil {
		return "", err
	}
	return object.WriteObject(r, object.TypeCommit, content)
}

// ApplyCommitAsPatch applies a commit's changes as a patch onto newBase.
// This is what rebase does: replay each commit's diff onto a new base.
func ApplyCommitAsPatch(r *repo.Repo, commitHash, newParentHash string, identity string) (string, error) {
	return CherryPick(r, commitHash, newParentHash, identity)
}

func getCommitTree(r *repo.Repo, commitHash string) (string, error) {
	_, content, err := object.ReadRaw(r, commitHash)
	if err != nil {
		return "", err
	}
	commit, err := object.ParseCommit(content)
	if err != nil {
		return "", err
	}
	return commit.Tree, nil
}

// InteractiveAction represents one action in an interactive rebase.
type InteractiveAction struct {
	Op   string // "pick", "squash", "reword", "drop", "edit"
	Hash string
	Msg  string
}

// ParseTodoList parses an interactive rebase todo file.
func ParseTodoList(content string) []InteractiveAction {
	var actions []InteractiveAction
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		a := InteractiveAction{Op: parts[0], Hash: parts[1]}
		if len(parts) >= 3 {
			a.Msg = parts[2]
		}
		actions = append(actions, a)
	}
	return actions
}

// GenerateTodoList creates the default todo list for interactive rebase.
func GenerateTodoList(r *repo.Repo, commits []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("# Interactive rebase\n")
	sb.WriteString("# Commands: pick, squash, reword, drop, edit\n")
	sb.WriteString("#\n")

	for _, h := range commits {
		_, content, err := object.ReadRaw(r, h)
		if err != nil {
			continue
		}
		commit, err := object.ParseCommit(content)
		if err != nil {
			continue
		}
		msg := strings.TrimSpace(commit.Message)
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		short := h
		if len(h) > 7 {
			short = h[:7]
		}
		fmt.Fprintf(&sb, "pick %s %s\n", short, msg)
	}
	return sb.String(), nil
}

// ApplyInteractive applies an interactive rebase todo list.
// Returns new HEAD hash and list of squash messages.
func ApplyInteractive(r *repo.Repo, actions []InteractiveAction, ontoHash string, identity string, resolveFullHash func(string) (string, error)) (string, error) {
	cur := ontoHash
	var squashMsgs []string

	for i, action := range actions {
		fullHash, err := resolveFullHash(action.Hash)
		if err != nil {
			return "", fmt.Errorf("cannot resolve %s: %w", action.Hash, err)
		}

		_, content, err := object.ReadRaw(r, fullHash)
		if err != nil {
			return "", err
		}
		origCommit, err := object.ParseCommit(content)
		if err != nil {
			return "", err
		}

		switch action.Op {
		case "pick", "p":
			cur, err = CherryPick(r, fullHash, cur, identity)
			if err != nil {
				return "", fmt.Errorf("conflict at commit %s: %w", action.Hash, err)
			}
			squashMsgs = nil

		case "squash", "s":
			// Apply but squash into previous commit
			newHash, err := CherryPick(r, fullHash, cur, identity)
			if err != nil {
				return "", fmt.Errorf("conflict at squash %s: %w", action.Hash, err)
			}

			// Amend the previous commit to include this tree but combine messages
			squashMsgs = append(squashMsgs, strings.TrimSpace(origCommit.Message))

			// Get current commit info
			_, curContent, _ := object.ReadRaw(r, cur)
			curCommit, _ := object.ParseCommit(curContent)

			// Get new tree from the applied commit
			_, newContent, _ := object.ReadRaw(r, newHash)
			newCommit, _ := object.ParseCommit(newContent)

			combinedMsg := strings.TrimSpace(curCommit.Message) + "\n\n" + strings.Join(squashMsgs, "\n\n")
			amended := &object.Commit{
				Tree:      newCommit.Tree,
				Parents:   curCommit.Parents,
				Author:    curCommit.Author,
				Committer: identity,
				Message:   combinedMsg,
			}
			ac, _ := amended.Serialize()
			cur, err = object.WriteObject(r, object.TypeCommit, ac)
			if err != nil {
				return "", err
			}

		case "reword", "r":
			// Apply as normal pick, but use new message from action
			cur, err = CherryPick(r, fullHash, cur, identity)
			if err != nil {
				return "", fmt.Errorf("conflict at reword %s: %w", action.Hash, err)
			}
			if action.Msg != "" {
				// Amend message
				_, curContent, _ := object.ReadRaw(r, cur)
				curCommit, _ := object.ParseCommit(curContent)
				curCommit.Message = action.Msg
				ac, _ := curCommit.Serialize()
				cur, err = object.WriteObject(r, object.TypeCommit, ac)
				if err != nil {
					return "", err
				}
			}

		case "drop", "d":
			// Skip this commit entirely
			_ = i
			continue

		case "edit", "e":
			// Apply and pause — for simplicity we just apply and continue
			cur, err = CherryPick(r, fullHash, cur, identity)
			if err != nil {
				return "", fmt.Errorf("conflict at edit %s: %w", action.Hash, err)
			}
		}
	}

	return cur, nil
}

// Ensure diff and patch are used (avoid import errors)
var _ = diff.TreeDiff
var _ = patch.ParseHunks
