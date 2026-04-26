package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/mygit/internal/bisect"
	"github.com/user/mygit/internal/blame"
	"github.com/user/mygit/internal/color"
	"github.com/user/mygit/internal/config"
	diffpkg "github.com/user/mygit/internal/diff"
	"github.com/user/mygit/internal/fsck"
	"github.com/user/mygit/internal/ignore"
	"github.com/user/mygit/internal/index"
	mergepkg "github.com/user/mygit/internal/merge"
	"github.com/user/mygit/internal/object"
	"github.com/user/mygit/internal/rebase"
	"github.com/user/mygit/internal/reflog"
	"github.com/user/mygit/internal/refs"
	"github.com/user/mygit/internal/repo"
	"github.com/user/mygit/internal/resolve"
	"github.com/user/mygit/internal/stash"
	tagpkg "github.com/user/mygit/internal/tag"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "init":          err = cmdInit(args)
	case "hash-object":   err = cmdHashObject(args)
	case "cat-file":      err = cmdCatFile(args)
	case "add":           err = cmdAdd(args)
	case "write-tree":    err = cmdWriteTree(args)
	case "ls-tree":       err = cmdLsTree(args)
	case "commit":        err = cmdCommit(args)
	case "log":           err = cmdLog(args)
	case "branch":        err = cmdBranch(args)
	case "checkout":      err = cmdCheckout(args)
	case "status":        err = cmdStatus(args)
	case "diff":          err = cmdDiff(args)
	case "merge":         err = cmdMerge(args)
	case "show":          err = cmdShow(args)
	case "reset":         err = cmdReset(args)
	case "stash":         err = cmdStash(args)
	case "tag":           err = cmdTag(args)
	case "blame":         err = cmdBlame(args)
	case "reflog":        err = cmdReflog(args)
	case "fsck":          err = cmdFsck(args)
	case "gc":            err = cmdGC(args)
	case "config":        err = cmdConfig(args)
	case "rebase":        err = cmdRebase(args)
	case "cherry-pick":   err = cmdCherryPick(args)
	case "restore":       err = cmdRestore(args)
	case "bisect":        err = cmdBisect(args)
	case "help", "--help", "-h": printHelp()
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n", color.Red("error"), cmd)
		fmt.Fprintf(os.Stderr, "Run 'mygit help' for available commands.\n")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", color.Red("error"), err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(color.Bold(color.Cyan("mygit — Build Your Own Git (Full)")))
	fmt.Println()
	fmt.Println(color.Bold("Core:"))
	fmt.Println("  init                          Initialize repository")
	fmt.Println("  add <paths...>                Stage files")
	fmt.Println("  add -p <file>                 Interactively stage hunks")
	fmt.Println("  commit -m <msg>               Create a commit")
	fmt.Println("  commit --amend [-m <msg>]     Amend last commit")
	fmt.Println("  status                        Show working tree status")
	fmt.Println("  restore <file>                Discard changes to file")
	fmt.Println("  restore --staged <file>       Unstage file")
	fmt.Println()
	fmt.Println(color.Bold("History:"))
	fmt.Println("  log [--oneline] [-n N] [--graph] [--author <name>]")
	fmt.Println("  diff [<ref1> <ref2>]          Show changes")
	fmt.Println("  show [<ref>]                  Show commit + diff")
	fmt.Println("  blame <file>                  Show who changed each line")
	fmt.Println()
	fmt.Println(color.Bold("Branch & merge:"))
	fmt.Println("  branch [name]                 List or create branches")
	fmt.Println("  branch -d <n>              Delete branch")
	fmt.Println("  checkout <branch|ref>         Switch branches")
	fmt.Println("  merge <branch>                Merge a branch")
	fmt.Println("  merge --abort                 Abort in-progress merge")
	fmt.Println("  rebase <branch>               Rebase current branch")
	fmt.Println("  rebase -i <ref>               Interactive rebase")
	fmt.Println("  rebase --abort                Abort in-progress rebase")
	fmt.Println("  cherry-pick <ref>             Apply a commit")
	fmt.Println("  reset [--soft|--hard] <ref>   Reset HEAD")
	fmt.Println()
	fmt.Println(color.Bold("Stash & tags:"))
	fmt.Println("  stash [push|list|pop|drop]")
	fmt.Println("  tag [-l|-d|-a] [name] [-m msg]")
	fmt.Println()
	fmt.Println(color.Bold("Debug & internals:"))
	fmt.Println("  bisect [start|good|bad|next|reset|log]")
	fmt.Println("  hash-object [-w] [--stdin] <file>")
	fmt.Println("  cat-file -t|-p|-s <hash>")
	fmt.Println("  write-tree / ls-tree <hash>")
	fmt.Println("  reflog [ref]")
	fmt.Println("  fsck / gc")
	fmt.Println("  config <key> [value]")
	fmt.Println()
	fmt.Println(color.Bold("Ref syntax:"))
	fmt.Println("  HEAD, HEAD~1, HEAD~3, HEAD^, HEAD^2")
	fmt.Println("  branch-name, tag-name, short-hash (min 4 chars)")
}

// ─── helpers shared across commands ──────────────────────────────────────────

func openRepo() (*repo.Repo, error) { return repo.OpenCwd() }

func resolveRef(r *repo.Repo, s string) (string, error) {
	return resolve.Commit(r, s)
}

func getIdentity() string {
	cfg := config.Load()
	ts := fmt.Sprintf("%d +0000", timeNow())
	return fmt.Sprintf("%s <%s> %s", cfg.AuthorName(), cfg.AuthorEmail(), ts)
}

func commitTreeHash(r *repo.Repo, commitHash string) (string, error) {
	_, content, err := object.ReadRaw(r, commitHash)
	if err != nil { return "", err }
	c, err := object.ParseCommit(content)
	if err != nil { return "", err }
	return c.Tree, nil
}

func writeTreeFromIndex(r *repo.Repo, idx *index.Index) (string, error) {
	return buildTree(r, idx.Entries, "")
}

func buildTree(r *repo.Repo, entries []index.Entry, prefix string) (string, error) {
	seen := map[string]bool{}
	var treeEntries []object.TreeEntry
	for i := range entries {
		e := &entries[i]
		rel := e.Path
		if prefix != "" {
			if len(rel) <= len(prefix)+1 || rel[:len(prefix)+1] != prefix+"/" { continue }
			rel = rel[len(prefix)+1:]
		}
		slash := strings.IndexByte(rel, '/')
		if slash < 0 {
			treeEntries = append(treeEntries, object.TreeEntry{Mode: e.Mode, Name: rel, Hash: e.Hash})
		} else {
			dirName := rel[:slash]
			if seen[dirName] { continue }
			seen[dirName] = true
			sub := dirName
			if prefix != "" { sub = prefix + "/" + dirName }
			subHash, err := buildTree(r, entries, sub)
			if err != nil { return "", err }
			treeEntries = append(treeEntries, object.TreeEntry{Mode: "40000", Name: dirName, Hash: subHash})
		}
	}
	tree := &object.Tree{Entries: treeEntries}
	content, err := tree.Serialize()
	if err != nil { return "", err }
	return object.WriteObject(r, object.TypeTree, content)
}

func checkoutTree(r *repo.Repo, treeHash, dir string) error {
	_, content, err := object.ReadRaw(r, treeHash)
	if err != nil { return err }
	tree, err := object.ParseTree(content)
	if err != nil { return err }
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == ".mygit" { continue }
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
	for _, e := range tree.Entries {
		tp := filepath.Join(dir, e.Name)
		if e.IsDir() {
			os.MkdirAll(tp, 0755)
			if err := checkoutTree(r, e.Hash, tp); err != nil { return err }
		} else {
			_, blob, err := object.ReadRaw(r, e.Hash)
			if err != nil { return err }
			if err := os.WriteFile(tp, blob, 0644); err != nil { return err }
		}
	}
	return nil
}

func addPath(r *repo.Repo, idx *index.Index, path string, ignorer *ignore.Matcher) error {
	info, err := os.Stat(path)
	if err != nil { return fmt.Errorf("cannot stat %q: %w", path, err) }
	if info.IsDir() { return addDir(r, idx, path, ignorer) }
	absPath, _ := filepath.Abs(path)
	relPath, err := filepath.Rel(r.Root, absPath)
	if err != nil { relPath = path }
	relPath = filepath.ToSlash(relPath)
	if ignorer.Match(relPath) { return nil }
	data, err := os.ReadFile(path)
	if err != nil { return err }
	hash, err := object.WriteObject(r, object.TypeBlob, data)
	if err != nil { return err }
	mode := "100644"
	if info.Mode()&0111 != 0 { mode = "100755" }
	idx.Add(index.Entry{Mode: mode, Hash: hash, Path: relPath})
	return nil
}

func addDir(r *repo.Repo, idx *index.Index, dirPath string, ignorer *ignore.Matcher) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil { return err }
	for _, e := range entries {
		if e.Name() == ".mygit" || e.Name() == ".git" { continue }
		full := filepath.Join(dirPath, e.Name())
		if e.IsDir() {
			if err := addDir(r, idx, full, ignorer); err != nil { return err }
		} else {
			if err := addPath(r, idx, full, ignorer); err != nil { return err }
		}
	}
	return nil
}

func readAllStdin() ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(tmp)
		if n > 0 { buf = append(buf, tmp[:n]...) }
		if err != nil { break }
	}
	return buf, nil
}

// ─── init ────────────────────────────────────────────────────────────────────

func cmdInit(args []string) error {
	dir := "."
	if len(args) > 0 { dir = args[0] }
	r, err := repo.Init(dir)
	if err != nil { return err }
	fmt.Printf("Initialized empty mygit repository in %s\n", color.Cyan(r.GitDir()))
	return nil
}

// ─── hash-object ─────────────────────────────────────────────────────────────

func cmdHashObject(args []string) error {
	write, stdin := false, false
	var files []string
	for _, a := range args {
		switch a {
		case "-w": write = true
		case "--stdin": stdin = true
		default: files = append(files, a)
		}
	}
	var r *repo.Repo
	if write {
		var err error
		if r, err = openRepo(); err != nil { return err }
	}
	hashOne := func(data []byte) error {
		h := object.HashHex(object.TypeBlob, data)
		if write { object.WriteObject(r, object.TypeBlob, data) }
		fmt.Println(h)
		return nil
	}
	if stdin { data, _ := readAllStdin(); return hashOne(data) }
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil { return err }
		if err := hashOne(data); err != nil { return err }
	}
	return nil
}

// ─── cat-file ────────────────────────────────────────────────────────────────

func cmdCatFile(args []string) error {
	if len(args) < 2 { return fmt.Errorf("usage: mygit cat-file (-t|-p|-s) <hash>") }
	flag, rawRef := args[0], args[1]
	r, err := openRepo()
	if err != nil { return err }
	hash, err := resolveRef(r, rawRef)
	if err != nil { return err }
	switch flag {
	case "-t":
		t, err := object.ReadType(r, hash)
		if err != nil { return err }
		fmt.Println(t)
	case "-s":
		sz, err := object.ReadSize(r, hash)
		if err != nil { return err }
		fmt.Println(sz)
	case "-p":
		t, content, err := object.ReadRaw(r, hash)
		if err != nil { return err }
		if t == object.TypeTree {
			tree, err := object.ParseTree(content)
			if err != nil { return err }
			for _, e := range tree.Entries {
				fmt.Printf("%s %s %s\t%s\n", e.Mode, e.EntryType(), color.Yellow(e.Hash), e.Name)
			}
		} else {
			os.Stdout.Write(content)
		}
	default:
		return fmt.Errorf("unknown flag %q", flag)
	}
	return nil
}

// ─── add ─────────────────────────────────────────────────────────────────────

func cmdAdd(args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: mygit add [-p] <paths...>") }
	r, err := openRepo()
	if err != nil { return err }

	// Interactive patch mode: add -p <file>
	if args[0] == "-p" {
		if len(args) < 2 { return fmt.Errorf("usage: mygit add -p <file>") }
		return cmdAddPatch(r, args[1:])
	}

	idx, err := index.Read(r)
	if err != nil { return err }
	ignorer := ignore.Load(r.Root)
	for _, path := range args {
		if err := addPath(r, idx, path, ignorer); err != nil { return err }
	}
	return index.Write(r, idx)
}

func cmdAddPatch(r *repo.Repo, files []string) error {
	idx, err := index.Read(r)
	if err != nil { return err }

	for _, filePath := range files {
		// Read current file from disk
		diskData, err := os.ReadFile(filePath)
		if err != nil { return err }
		diskLines := strings.Split(strings.TrimRight(string(diskData), "\n"), "\n")

		// Read staged version (or HEAD version if not staged)
		absPath, _ := filepath.Abs(filePath)
		relPath, _ := filepath.Rel(r.Root, absPath)
		relPath = filepath.ToSlash(relPath)

		var stagedLines []string
		for _, e := range idx.Entries {
			if e.Path == relPath {
				_, blob, err := object.ReadRaw(r, e.Hash)
				if err == nil {
					stagedLines = strings.Split(strings.TrimRight(string(blob), "\n"), "\n")
				}
				break
			}
		}
		if stagedLines == nil {
			// Try HEAD
			headHash, _ := refs.ResolveHEAD(r)
			if headHash != "" {
				headFiles := map[string]string{}
				if _, cc, err := object.ReadRaw(r, headHash); err == nil {
					if c, err := object.ParseCommit(cc); err == nil {
						mergepkg.WalkTree(r, c.Tree, "", headFiles)
					}
				}
				if blobHash, ok := headFiles[relPath]; ok {
					if _, blob, err := object.ReadRaw(r, blobHash); err == nil {
						stagedLines = strings.Split(strings.TrimRight(string(blob), "\n"), "\n")
					}
				}
			}
		}
		if stagedLines == nil {
			stagedLines = []string{}
		}

		// Generate hunks
		from := stagedLines
		to := diskLines
		// Simple diff between from and to
		var stagedContent strings.Builder
		for _, l := range to { stagedContent.WriteString(l + "\n") }

		// For each changed section, ask user
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("\n%s %s\n", color.Bold("Staging hunks for:"), color.Cyan(filePath))

		// Simple approach: show whole diff, ask stage all or nothing
		if strings.Join(from, "\n") == strings.Join(to, "\n") {
			fmt.Println("  (no changes)")
			continue
		}

		fmt.Println(color.Bold("--- staged"))
		fmt.Println(color.Bold("+++ disk"))
		for _, l := range from { fmt.Println(color.Red("-" + l)) }
		for _, l := range to   { fmt.Println(color.Green("+" + l)) }

		fmt.Print("Stage this file? [y/n/q] ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "q" { break }
		if line == "y" {
			hash, err := object.WriteObject(r, object.TypeBlob, diskData)
			if err != nil { return err }
			mode := "100644"
			info, _ := os.Stat(filePath)
			if info != nil && info.Mode()&0111 != 0 { mode = "100755" }
			idx.Add(index.Entry{Mode: mode, Hash: hash, Path: relPath})
			fmt.Printf("  %s staged\n", color.Green("✓"))
		} else {
			fmt.Println("  skipped")
		}
	}
	return index.Write(r, idx)
}

// ─── write-tree / ls-tree ────────────────────────────────────────────────────

func cmdWriteTree(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	idx, err := index.Read(r)
	if err != nil { return err }
	hash, err := writeTreeFromIndex(r, idx)
	if err != nil { return err }
	fmt.Println(hash)
	return nil
}

func cmdLsTree(args []string) error {
	if len(args) < 1 { return fmt.Errorf("usage: mygit ls-tree <hash>") }
	r, err := openRepo()
	if err != nil { return err }
	hash, err := resolveRef(r, args[0])
	if err != nil { return err }
	// If it's a commit, get its tree
	if t, _ := object.ReadType(r, hash); t == object.TypeCommit {
		hash, err = commitTreeHash(r, hash)
		if err != nil { return err }
	}
	_, content, err := object.ReadRaw(r, hash)
	if err != nil { return err }
	tree, err := object.ParseTree(content)
	if err != nil { return err }
	for _, e := range tree.Entries {
		fmt.Printf("%s %s %s\t%s\n", e.Mode, color.Cyan(e.EntryType()), color.Yellow(e.Hash), e.Name)
	}
	return nil
}

// ─── commit ──────────────────────────────────────────────────────────────────

func cmdCommit(args []string) error {
	amend := false
	var message string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--amend": amend = true
		case "-m":
			if i+1 < len(args) { message = args[i+1]; i++ }
		}
	}

	r, err := openRepo()
	if err != nil { return err }
	idx, err := index.Read(r)
	if err != nil { return err }

	if amend {
		return cmdCommitAmend(r, idx, message)
	}

	if len(idx.Entries) == 0 { return fmt.Errorf("nothing to commit (use 'mygit add' to stage files)") }
	if message == "" { return fmt.Errorf("usage: mygit commit -m <message>") }

	treeHash, err := writeTreeFromIndex(r, idx)
	if err != nil { return err }

	parentHash, _ := refs.ResolveHEAD(r)
	var parents []string
	if parentHash != "" { parents = []string{parentHash} }

	identity := getIdentity()
	commit := &object.Commit{Tree: treeHash, Parents: parents, Author: identity, Committer: identity, Message: message}
	content, err := commit.Serialize()
	if err != nil { return err }
	hash, err := object.WriteObject(r, object.TypeCommit, content)
	if err != nil { return err }

	oldHash := parentHash
	if oldHash == "" { oldHash = strings.Repeat("0", 40) }
	refs.AdvanceHEAD(r, hash)

	branch, _ := refs.CurrentBranch(r)
	refName := "HEAD"
	if branch != "" { refName = "refs/heads/" + branch }
	reflog.Append(r, refName, oldHash, hash, "commit: "+message)
	reflog.Append(r, "HEAD", oldHash, hash, "commit: "+message)

	if branch == "" { branch = "(detached)" }
	fmt.Printf("[%s %s] %s\n", color.Cyan(branch), color.Yellow(hash[:7]), message)
	return nil
}

func cmdCommitAmend(r *repo.Repo, idx *index.Index, newMsg string) error {
	headHash, err := refs.ResolveHEAD(r)
	if err != nil || headHash == "" { return fmt.Errorf("nothing to amend: no commits yet") }

	_, oldContent, err := object.ReadRaw(r, headHash)
	if err != nil { return err }
	oldCommit, err := object.ParseCommit(oldContent)
	if err != nil { return err }

	// Use new tree if index has changes, otherwise keep old tree
	treeHash := oldCommit.Tree
	if len(idx.Entries) > 0 {
		treeHash, err = writeTreeFromIndex(r, idx)
		if err != nil { return err }
	}

	msg := oldCommit.Message
	if newMsg != "" { msg = newMsg }

	amended := &object.Commit{
		Tree:      treeHash,
		Parents:   oldCommit.Parents,
		Author:    oldCommit.Author,
		Committer: getIdentity(),
		Message:   msg,
	}
	content, err := amended.Serialize()
	if err != nil { return err }
	hash, err := object.WriteObject(r, object.TypeCommit, content)
	if err != nil { return err }

	refs.AdvanceHEAD(r, hash)
	branch, _ := refs.CurrentBranch(r)
	if branch == "" { branch = "(detached)" }
	fmt.Printf("[%s %s] (amend) %s\n", color.Cyan(branch), color.Yellow(hash[:7]), strings.TrimSpace(msg))
	return nil
}

// ─── log ─────────────────────────────────────────────────────────────────────

func cmdLog(args []string) error {
	oneline, graph := false, false
	limit := -1
	author := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--oneline": oneline = true
		case args[i] == "--graph":   graph = true
		case args[i] == "-n" && i+1 < len(args):
			limit, _ = strconv.Atoi(args[i+1]); i++
		case strings.HasPrefix(args[i], "-n"):
			limit, _ = strconv.Atoi(args[i][2:])
		case args[i] == "--author" && i+1 < len(args):
			author = args[i+1]; i++
		}
	}

	r, err := openRepo()
	if err != nil { return err }
	startHash, err := refs.ResolveHEAD(r)
	if err != nil { return err }
	if startHash == "" { fmt.Println(color.Yellow("No commits yet")); return nil }

	visited := map[string]bool{}
	queue := []string{startHash}
	count := 0

	for len(queue) > 0 {
		if limit >= 0 && count >= limit { break }
		hash := queue[0]; queue = queue[1:]
		if visited[hash] || hash == "" { continue }
		visited[hash] = true

		_, content, err := object.ReadRaw(r, hash)
		if err != nil { return err }
		commit, err := object.ParseCommit(content)
		if err != nil { return err }

		msg := strings.TrimSpace(commit.Message)
		if i := strings.IndexByte(msg, '\n'); i >= 0 { msg = msg[:i] }

		// Filter by author
		if author != "" && !strings.Contains(commit.Author, author) {
			queue = append(queue, commit.Parents...)
			continue
		}

		count++
		if oneline {
			fmt.Printf("%s %s\n", hash[:7], msg)
		} else if graph {
			prefix := color.Magenta("*")
			if len(commit.Parents) > 1 { prefix = color.Magenta("*") }
			fmt.Printf("%s %s %s\n", prefix, color.Yellow(hash[:7]), msg)
			if len(commit.Parents) > 1 { fmt.Printf("%s\\\n", color.Magenta("|")) }
		} else {
			fmt.Printf("%s %s\n", color.Yellow(hash), msg)
		}
		queue = append(queue, commit.Parents...)
	}
	return nil
}

// ─── branch ──────────────────────────────────────────────────────────────────

func cmdBranch(args []string) error {
	r, err := openRepo()
	if err != nil { return err }

	if len(args) >= 2 && (args[0] == "-d" || args[0] == "-D") {
		name := args[1]
		current, _ := refs.CurrentBranch(r)
		if current == name { return fmt.Errorf("cannot delete current branch") }
		if err := os.Remove(r.Path("refs", "heads", name)); err != nil { return fmt.Errorf("branch %q not found", name) }
		fmt.Printf("Deleted branch %s\n", color.Red(name))
		return nil
	}

	if len(args) == 0 {
		branches, err := refs.ListBranches(r)
		if err != nil { return err }
		current, _ := refs.CurrentBranch(r)
		for _, b := range branches {
			h, _ := refs.ReadRef(r, "refs/heads/"+b)
			short := ""
			if len(h) >= 7 { short = " " + color.Yellow(h[:7]) }
			if b == current { fmt.Printf("%s%s\n", color.BranchCurrent(b), short) } else { fmt.Printf("%s%s\n", color.BranchOther(b), short) }
		}
		return nil
	}

	// Create branch at specific ref or HEAD
	name := args[0]
	startRef := "HEAD"
	if len(args) > 1 { startRef = args[1] }
	hash, err := resolveRef(r, startRef)
	if err != nil { return err }
	if err := refs.CreateBranch(r, name, hash); err != nil { return err }
	fmt.Printf("Branch %s created at %s\n", color.Cyan(name), color.Yellow(hash[:7]))
	return nil
}

// ─── checkout ────────────────────────────────────────────────────────────────

func cmdCheckout(args []string) error {
	if len(args) < 1 { return fmt.Errorf("usage: mygit checkout <branch-or-ref>") }
	target := args[0]
	r, err := openRepo()
	if err != nil { return err }

	var commitHash string
	isBranch := false
	if h, _ := refs.ReadRef(r, "refs/heads/"+target); h != "" {
		commitHash, isBranch = h, true
	} else {
		commitHash, err = resolveRef(r, target)
		if err != nil { return fmt.Errorf("checkout: cannot resolve %q", target) }
	}

	_, cc, err := object.ReadRaw(r, commitHash)
	if err != nil { return err }
	commit, err := object.ParseCommit(cc)
	if err != nil { return err }
	if err := checkoutTree(r, commit.Tree, r.Root); err != nil { return err }

	oldHash, _ := refs.ResolveHEAD(r)
	if isBranch { refs.UpdateHEADBranch(r, target) } else { refs.UpdateHEADDetached(r, commitHash) }
	reflog.Append(r, "HEAD", oldHash, commitHash, "checkout: moving to "+target)

	fmt.Printf("Switched to %s (%s)\n", color.Cyan(target), color.Yellow(commitHash[:7]))
	return nil
}

// ─── status ──────────────────────────────────────────────────────────────────

func cmdStatus(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	branch, _ := refs.CurrentBranch(r)
	if branch == "" { fmt.Println("HEAD detached") } else { fmt.Printf("On branch %s\n", color.Cyan(branch)) }

	if rebase.IsInProgress(r) { fmt.Println(color.Yellow("  (rebase in progress)")) }

	idx, err := index.Read(r)
	if err != nil { return err }
	headHash, _ := refs.ResolveHEAD(r)
	headFiles := map[string]string{}
	if headHash != "" {
		if _, cc, err := object.ReadRaw(r, headHash); err == nil {
			if c, err := object.ParseCommit(cc); err == nil {
				mergepkg.WalkTree(r, c.Tree, "", headFiles)
			}
		}
	}

	staged := false
	for _, e := range idx.Entries {
		if e.Hash != headFiles[e.Path] {
			if !staged { fmt.Println(color.Bold("\nChanges staged for commit:")); staged = true }
			if headFiles[e.Path] == "" { fmt.Printf("\t%s %s\n", color.Green("new file:"), e.Path) } else { fmt.Printf("\t%s %s\n", color.Green("modified:"), e.Path) }
		}
	}
	for path := range headFiles {
		found := false
		for _, e := range idx.Entries { if e.Path == path { found = true; break } }
		if !found {
			if !staged { fmt.Println(color.Bold("\nChanges staged for commit:")); staged = true }
			fmt.Printf("\t%s %s\n", color.Red("deleted:"), path)
		}
	}
	if !staged { fmt.Println("nothing to commit, working tree clean") }
	return nil
}

// ─── diff ────────────────────────────────────────────────────────────────────

func cmdDiff(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	var tree1, tree2 string

	if len(args) == 0 {
		// diff HEAD vs index (staged changes)
		headHash, err := refs.ResolveHEAD(r)
		if err != nil || headHash == "" { fmt.Println("No commits yet"); return nil }
		_, cc, _ := object.ReadRaw(r, headHash)
		c, _ := object.ParseCommit(cc)
		idx, _ := index.Read(r)
		indexTree, _ := writeTreeFromIndex(r, idx)
		tree1, tree2 = c.Tree, indexTree
	} else if len(args) == 1 {
		// diff <ref> vs working index
		hash, err := resolveRef(r, args[0])
		if err != nil { return err }
		tree1, err = commitTreeHash(r, hash)
		if err != nil { return err }
		idx, _ := index.Read(r)
		tree2, err = writeTreeFromIndex(r, idx)
		if err != nil { return err }
	} else if len(args) == 2 {
		h1, err := resolveRef(r, args[0])
		if err != nil { return err }
		h2, err := resolveRef(r, args[1])
		if err != nil { return err }
		tree1, err = commitTreeHash(r, h1)
		if err != nil { return err }
		tree2, err = commitTreeHash(r, h2)
		if err != nil { return err }
	} else {
		return fmt.Errorf("usage: mygit diff [<ref1>] [<ref2>]")
	}

	diffs, err := diffpkg.TreeDiff(r, tree1, tree2)
	if err != nil { return err }
	if len(diffs) == 0 { fmt.Println("No changes"); return nil }
	fmt.Print(diffpkg.FormatDiff(diffs))
	return nil
}

// ─── merge ───────────────────────────────────────────────────────────────────

func cmdMerge(args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: mygit merge <branch> | --abort") }
	r, err := openRepo()
	if err != nil { return err }

	if args[0] == "--abort" {
		// Restore HEAD
		headHash, err := refs.ResolveHEAD(r)
		if err != nil { return err }
		_, cc, err := object.ReadRaw(r, headHash)
		if err != nil { return err }
		c, err := object.ParseCommit(cc)
		if err != nil { return err }
		checkoutTree(r, c.Tree, r.Root)
		fmt.Println("Merge aborted")
		return nil
	}

	oursHash, err := refs.ResolveHEAD(r)
	if err != nil || oursHash == "" { return fmt.Errorf("no commits on current branch") }

	theirsHash, err := resolveRef(r, args[0])
	if err != nil { return err }

	isAnc, err := mergepkg.IsAncestor(r, oursHash, theirsHash)
	if err != nil { return err }
	if isAnc {
		refs.AdvanceHEAD(r, theirsHash)
		_, cc, _ := object.ReadRaw(r, theirsHash)
		c, _ := object.ParseCommit(cc)
		checkoutTree(r, c.Tree, r.Root)
		fmt.Printf("Fast-forward to %s\n", color.Yellow(theirsHash[:7]))
		return nil
	}

	// Already up to date?
	isAnc2, _ := mergepkg.IsAncestor(r, theirsHash, oursHash)
	if isAnc2 { fmt.Println("Already up to date."); return nil }

	lca, err := mergepkg.FindLCA(r, oursHash, theirsHash)
	if err != nil { return err }
	fmt.Printf("Merge base: %s\n", color.Yellow(lca[:7]))

	oursTree, _ := commitTreeHash(r, oursHash)
	theirsTree, _ := commitTreeHash(r, theirsHash)
	baseTree, _ := commitTreeHash(r, lca)

	mergedFiles, conflicts, err := mergepkg.ThreeWayMerge(r, baseTree, oursTree, theirsTree)
	if err != nil { return err }
	if len(conflicts) > 0 {
		fmt.Println(color.Conflict("CONFLICT — Conflicting files:"))
		for _, c := range conflicts { fmt.Printf("  %s %s\n", color.Red("✗"), c) }
		return fmt.Errorf("resolve conflicts and use 'mygit commit' or 'mygit merge --abort'")
	}

	mergedTree, err := mergepkg.BuildTreeFromMap(r, mergedFiles)
	if err != nil { return err }
	checkoutTree(r, mergedTree, r.Root)

	branch, _ := refs.CurrentBranch(r)
	msg := fmt.Sprintf("Merge branch '%s' into %s", args[0], branch)
	identity := getIdentity()
	commit := &object.Commit{Tree: mergedTree, Parents: []string{oursHash, theirsHash}, Author: identity, Committer: identity, Message: msg}
	content, _ := commit.Serialize()
	hash, err := object.WriteObject(r, object.TypeCommit, content)
	if err != nil { return err }
	refs.AdvanceHEAD(r, hash)
	fmt.Printf("Merged %s into %s → %s\n", color.Cyan(args[0]), color.Cyan(branch), color.Yellow(hash[:7]))
	return nil
}

// ─── show ────────────────────────────────────────────────────────────────────

func cmdShow(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	target := "HEAD"
	if len(args) > 0 { target = args[0] }
	hash, err := resolveRef(r, target)
	if err != nil { return err }

	_, content, err := object.ReadRaw(r, hash)
	if err != nil { return err }
	commit, err := object.ParseCommit(content)
	if err != nil { return err }

	fmt.Printf("%s %s\n", color.Yellow("commit"), color.Yellow(hash))
	fmt.Printf("%s %s\n", color.Bold("Author:   "), commit.Author)
	if len(commit.Parents) > 1 {
		fmt.Printf("%s %s\n", color.Bold("Parents:  "), strings.Join(commit.Parents, "\n           "))
	}
	fmt.Printf("\n    %s\n\n", strings.TrimSpace(commit.Message))

	if len(commit.Parents) > 0 {
		parentTree, _ := commitTreeHash(r, commit.Parents[0])
		diffs, err := diffpkg.TreeDiff(r, parentTree, commit.Tree)
		if err == nil && len(diffs) > 0 { fmt.Print(diffpkg.FormatDiff(diffs)) }
	}
	return nil
}

// ─── reset ───────────────────────────────────────────────────────────────────

func cmdReset(args []string) error {
	if len(args) < 1 { return fmt.Errorf("usage: mygit reset [--soft|--hard|--mixed] <ref>") }
	r, err := openRepo()
	if err != nil { return err }
	mode, target := "--mixed", args[0]
	if len(args) == 2 { mode, target = args[0], args[1] }

	commitHash, err := resolveRef(r, target)
	if err != nil { return err }
	oldHash, _ := refs.ResolveHEAD(r)
	refs.AdvanceHEAD(r, commitHash)
	reflog.Append(r, "HEAD", oldHash, commitHash, "reset: "+mode+" to "+target)

	switch mode {
	case "--hard":
		_, cc, _ := object.ReadRaw(r, commitHash)
		c, _ := object.ParseCommit(cc)
		checkoutTree(r, c.Tree, r.Root)
		files := map[string]string{}
		mergepkg.WalkTree(r, c.Tree, "", files)
		idx := &index.Index{}
		for p, h := range files { idx.Add(index.Entry{Mode: "100644", Hash: h, Path: p}) }
		index.Write(r, idx)
		fmt.Printf("HEAD is now at %s (hard reset)\n", color.Yellow(commitHash[:7]))
	case "--soft":
		fmt.Printf("HEAD is now at %s (soft reset — index unchanged)\n", color.Yellow(commitHash[:7]))
	default:
		// --mixed: reset index but not working dir
		_, cc, _ := object.ReadRaw(r, commitHash)
		c, _ := object.ParseCommit(cc)
		files := map[string]string{}
		mergepkg.WalkTree(r, c.Tree, "", files)
		idx := &index.Index{}
		for p, h := range files { idx.Add(index.Entry{Mode: "100644", Hash: h, Path: p}) }
		index.Write(r, idx)
		fmt.Printf("HEAD is now at %s\n", color.Yellow(commitHash[:7]))
	}
	return nil
}

// ─── restore ─────────────────────────────────────────────────────────────────

func cmdRestore(args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: mygit restore [--staged] <file>") }
	r, err := openRepo()
	if err != nil { return err }

	staged := false
	var files []string
	for _, a := range args {
		if a == "--staged" { staged = true } else { files = append(files, a) }
	}

	headHash, err := refs.ResolveHEAD(r)
	if err != nil { return err }
	if headHash == "" { return fmt.Errorf("no commits yet") }

	headFiles := map[string]string{}
	if _, cc, err := object.ReadRaw(r, headHash); err == nil {
		if c, err := object.ParseCommit(cc); err == nil {
			mergepkg.WalkTree(r, c.Tree, "", headFiles)
		}
	}

	idx, err := index.Read(r)
	if err != nil { return err }

	for _, filePath := range files {
		absPath, _ := filepath.Abs(filePath)
		relPath, _ := filepath.Rel(r.Root, absPath)
		relPath = filepath.ToSlash(relPath)

		blobHash, ok := headFiles[relPath]
		if !ok { return fmt.Errorf("file %q not in HEAD", relPath) }

		if staged {
			// Unstage: restore index entry to HEAD version
			idx.Add(index.Entry{Mode: "100644", Hash: blobHash, Path: relPath})
			fmt.Printf("Unstaged %s\n", color.Cyan(relPath))
		} else {
			// Discard working dir changes: restore file from HEAD
			_, blob, err := object.ReadRaw(r, blobHash)
			if err != nil { return err }
			if err := os.WriteFile(filePath, blob, 0644); err != nil { return err }
			fmt.Printf("Restored %s\n", color.Cyan(relPath))
		}
	}
	if staged { return index.Write(r, idx) }
	return nil
}

// ─── rebase ──────────────────────────────────────────────────────────────────

func cmdRebase(args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: mygit rebase [-i] <branch|ref>") }
	r, err := openRepo()
	if err != nil { return err }

	if args[0] == "--abort" {
		if !rebase.IsInProgress(r) { return fmt.Errorf("no rebase in progress") }
		state, err := rebase.LoadState(r)
		if err != nil { return err }
		// Restore original branch
		origHash, err := refs.ReadRef(r, "refs/heads/"+state.OrigBranch)
		if err == nil && origHash != "" {
			refs.UpdateHEADBranch(r, state.OrigBranch)
			_, cc, _ := object.ReadRaw(r, origHash)
			if c, err := object.ParseCommit(cc); err == nil {
				checkoutTree(r, c.Tree, r.Root)
			}
		}
		rebase.ClearState(r)
		fmt.Println("Rebase aborted")
		return nil
	}

	interactive := false
	onto := args[0]
	if args[0] == "-i" {
		if len(args) < 2 { return fmt.Errorf("usage: mygit rebase -i <ref>") }
		interactive = true
		onto = args[1]
	}

	ontoHash, err := resolveRef(r, onto)
	if err != nil { return err }
	headHash, err := refs.ResolveHEAD(r)
	if err != nil || headHash == "" { return fmt.Errorf("no commits on current branch") }

	branch, _ := refs.CurrentBranch(r)

	// Already up to date?
	isAnc, _ := mergepkg.IsAncestor(r, headHash, ontoHash)
	if isAnc { fmt.Println("Already up to date."); return nil }

	// Get commits to replay
	commits, err := rebase.CommitsBetween(r, ontoHash, headHash)
	if err != nil { return err }
	if len(commits) == 0 { fmt.Println("Nothing to rebase"); return nil }

	fmt.Printf("Rebasing %d commit(s) onto %s\n", len(commits), color.Yellow(ontoHash[:7]))

	identity := getIdentity()

	if interactive {
		// Generate todo list and open editor (or show it)
		todo, err := rebase.GenerateTodoList(r, commits)
		if err != nil { return err }

		// Write todo to temp file
		todoPath := r.Path("rebase-merge", "git-rebase-todo")
		os.MkdirAll(r.Path("rebase-merge"), 0755)
		if err := os.WriteFile(todoPath, []byte(todo), 0644); err != nil { return err }

		fmt.Println(color.Bold("Interactive rebase todo:"))
		fmt.Println(todo)
		fmt.Println("(Edit " + todoPath + " then run 'mygit rebase --continue')")
		fmt.Println()
		fmt.Println("Supported ops: pick, squash, reword, drop")
		fmt.Println("Proceeding with default (all pick)...")

		// Auto-proceed with the generated list
		actions := rebase.ParseTodoList(todo)
		resolver := func(shortHash string) (string, error) {
			return resolveRef(r, shortHash)
		}
		newHead, err := rebase.ApplyInteractive(r, actions, ontoHash, identity, resolver)
		if err != nil { return err }
		refs.AdvanceHEAD(r, newHead)
		if branch != "" { refs.WriteRef(r, "refs/heads/"+branch, newHead) }
		checkoutTree(r, func() string { h, _ := commitTreeHash(r, newHead); return h }(), r.Root)
		rebase.ClearState(r)
		fmt.Printf("Successfully rebased %s onto %s\n", color.Cyan(branch), color.Yellow(ontoHash[:7]))
		return nil
	}

	// Normal rebase: replay each commit
	cur := ontoHash
	for i, commitHash := range commits {
		newHash, err := rebase.ApplyCommitAsPatch(r, commitHash, cur, identity)
		if err != nil {
			// Save state for --continue or --abort
			rebase.SaveState(r, &rebase.RebaseState{
				OrigBranch: branch,
				OntoHash:   ontoHash,
				Remaining:  commits[i:],
				Done:       commits[:i],
			})
			return fmt.Errorf("conflict applying %s: %v\nUse 'mygit rebase --abort' to cancel", commitHash[:7], err)
		}
		_, content, _ := object.ReadRaw(r, commitHash)
		origCommit, _ := object.ParseCommit(content)
		msg := strings.TrimSpace(origCommit.Message)
		if j := strings.IndexByte(msg, '\n'); j >= 0 { msg = msg[:j] }
		fmt.Printf("  %s %s %s\n", color.Green("✓"), color.Yellow(newHash[:7]), msg)
		cur = newHash
	}

	refs.AdvanceHEAD(r, cur)
	if branch != "" { refs.WriteRef(r, "refs/heads/"+branch, cur) }
	checkoutTree(r, func() string { h, _ := commitTreeHash(r, cur); return h }(), r.Root)
	fmt.Printf("Successfully rebased %s onto %s\n", color.Cyan(branch), color.Yellow(ontoHash[:7]))
	return nil
}

// ─── cherry-pick ─────────────────────────────────────────────────────────────

func cmdCherryPick(args []string) error {
	if len(args) == 0 { return fmt.Errorf("usage: mygit cherry-pick <ref>") }
	r, err := openRepo()
	if err != nil { return err }

	headHash, err := refs.ResolveHEAD(r)
	if err != nil || headHash == "" { return fmt.Errorf("no commits on current branch") }

	for _, arg := range args {
		pickHash, err := resolveRef(r, arg)
		if err != nil { return err }

		newHash, err := rebase.CherryPick(r, pickHash, headHash, getIdentity())
		if err != nil { return fmt.Errorf("cherry-pick %s: %w", arg, err) }

		refs.AdvanceHEAD(r, newHash)
		headHash = newHash

		_, content, _ := object.ReadRaw(r, pickHash)
		origCommit, _ := object.ParseCommit(content)
		msg := strings.TrimSpace(origCommit.Message)
		if i := strings.IndexByte(msg, '\n'); i >= 0 { msg = msg[:i] }
		fmt.Printf("[%s] %s\n", color.Yellow(newHash[:7]), msg)
	}

	// Update working dir
	tree, _ := commitTreeHash(r, headHash)
	checkoutTree(r, tree, r.Root)
	return nil
}

// ─── stash ───────────────────────────────────────────────────────────────────

func cmdStash(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	sub := "push"
	if len(args) > 0 { sub = args[0] }

	switch sub {
	case "push", "save", "":
		idx, err := index.Read(r)
		if err != nil { return err }
		if len(idx.Entries) == 0 { fmt.Println("Nothing to stash"); return nil }
		treeHash, _ := writeTreeFromIndex(r, idx)
		headHash, _ := refs.ResolveHEAD(r)
		msg := "WIP stash"
		if len(args) >= 3 && args[1] == "-m" { msg = args[2] }
		identity := getIdentity()
		commit := &object.Commit{Tree: treeHash, Parents: []string{headHash}, Author: identity, Committer: identity, Message: "stash: " + msg}
		content, _ := commit.Serialize()
		hash, err := object.WriteObject(r, object.TypeCommit, content)
		if err != nil { return err }
		stash.Push(r, hash, msg)
		index.Write(r, &index.Index{})
		fmt.Printf("Saved working directory state: %s\n", color.Yellow(hash[:7]))

		// Also restore working dir to HEAD
		headHash2, _ := refs.ResolveHEAD(r)
		if headHash2 != "" {
			_, cc, _ := object.ReadRaw(r, headHash2)
			c, _ := object.ParseCommit(cc)
			checkoutTree(r, c.Tree, r.Root)
		}

	case "list":
		entries, err := stash.List(r)
		if err != nil { return err }
		if len(entries) == 0 { fmt.Println("No stash entries"); return nil }
		for i, e := range entries { fmt.Printf("stash@{%d}: %s (%s)\n", i, e.Message, color.Yellow(e.CommitHash[:7])) }

	case "pop":
		entry, err := stash.Pop(r)
		if err != nil { return err }
		_, cc, _ := object.ReadRaw(r, entry.CommitHash)
		c, _ := object.ParseCommit(cc)
		// Restore working dir
		checkoutTree(r, c.Tree, r.Root)
		// Restore index
		files := map[string]string{}
		mergepkg.WalkTree(r, c.Tree, "", files)
		idx := &index.Index{}
		for p, h := range files { idx.Add(index.Entry{Mode: "100644", Hash: h, Path: p}) }
		index.Write(r, idx)
		fmt.Printf("Restored stash: %s\n", entry.Message)

	case "drop":
		n := 0
		if len(args) > 1 { n, _ = strconv.Atoi(args[1]) }
		if err := stash.Drop(r, n); err != nil { return err }
		fmt.Printf("Dropped stash@{%d}\n", n)
	}
	return nil
}

// ─── tag ─────────────────────────────────────────────────────────────────────

func cmdTag(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	if len(args) == 0 || args[0] == "-l" {
		tags, err := tagpkg.List(r)
		if err != nil { return err }
		if len(tags) == 0 { fmt.Println("No tags"); return nil }
		for _, t := range tags {
			kind := "lightweight"
			if t.Annotated { kind = "annotated" }
			fmt.Printf("%s → %s (%s)\n", color.TagName(t.Name), color.Yellow(t.CommitHash[:7]), kind)
		}
		return nil
	}
	if args[0] == "-d" && len(args) > 1 {
		if err := tagpkg.Delete(r, args[1]); err != nil { return err }
		fmt.Printf("Deleted tag %s\n", color.TagName(args[1]))
		return nil
	}
	if args[0] == "-a" {
		if len(args) < 4 { return fmt.Errorf("usage: mygit tag -a <n> -m <msg>") }
		name := args[1]; msg := ""
		for i := 2; i < len(args)-1; i++ { if args[i] == "-m" { msg = args[i+1] } }
		hash, _ := refs.ResolveHEAD(r)
		tagger := getIdentity()
		if err := tagpkg.CreateAnnotated(r, name, hash, tagger, msg); err != nil { return err }
		fmt.Printf("Created annotated tag %s\n", color.TagName(name))
		return nil
	}
	name := args[0]; hash := ""
	if len(args) > 1 { hash, err = resolveRef(r, args[1]) } else { hash, err = refs.ResolveHEAD(r) }
	if err != nil || hash == "" { return fmt.Errorf("no commit to tag") }
	if err := tagpkg.CreateLightweight(r, name, hash); err != nil { return err }
	fmt.Printf("Created tag %s → %s\n", color.TagName(name), color.Yellow(hash[:7]))
	return nil
}

// ─── blame ───────────────────────────────────────────────────────────────────

func cmdBlame(args []string) error {
	if len(args) < 1 { return fmt.Errorf("usage: mygit blame <file>") }
	r, err := openRepo()
	if err != nil { return err }
	startHash, err := refs.ResolveHEAD(r)
	if err != nil || startHash == "" { return fmt.Errorf("no commits yet") }
	lines, err := blame.File(r, startHash, filepath.ToSlash(args[0]))
	if err != nil { return err }
	fmt.Print(blame.Format(lines))
	return nil
}

// ─── reflog ──────────────────────────────────────────────────────────────────

func cmdReflog(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	ref := "HEAD"
	if len(args) > 0 { ref = args[0] }
	entries, err := reflog.Read(r, ref)
	if err != nil { return err }
	if len(entries) == 0 { fmt.Println("No reflog entries"); return nil }
	for i, e := range entries {
		short := e.NewHash
		if len(short) > 7 { short = short[:7] }
		fmt.Printf("%s@{%d}: %s %s\n", color.Cyan(ref), i, color.Yellow(short), e.Message)
	}
	return nil
}

// ─── fsck / gc ───────────────────────────────────────────────────────────────

func cmdFsck(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	result, err := fsck.Run(r)
	if err != nil { return err }
	for _, msg := range result.OK       { fmt.Printf("%s %s\n", color.Green("ok  "), msg) }
	for _, msg := range result.Warnings { fmt.Printf("%s %s\n", color.Yellow("warn"), msg) }
	for _, msg := range result.Errors   { fmt.Printf("%s %s\n", color.Red("err "), msg) }
	return nil
}

func cmdGC(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	deleted, err := fsck.GC(r)
	if err != nil { return err }
	if deleted == 0 { fmt.Println("Nothing to collect") } else { fmt.Printf("Removed %s unreachable object(s)\n", color.Yellow(strconv.Itoa(deleted))) }
	return nil
}

// ─── config ──────────────────────────────────────────────────────────────────

func cmdConfig(args []string) error {
	cfg := config.Load()
	if len(args) == 0 { fmt.Println("Usage: mygit config <key> [value]"); return nil }
	if len(args) == 1 {
		v := cfg.Get(args[0])
		if v == "" { return fmt.Errorf("key %q not set", args[0]) }
		fmt.Println(v); return nil
	}
	cfg.Set(args[0], args[1])
	if err := cfg.Save(); err != nil { return err }
	fmt.Printf("Set %s = %s\n", color.Cyan(args[0]), args[1])
	return nil
}

// ─── bisect ──────────────────────────────────────────────────────────────────

func cmdBisect(args []string) error {
	r, err := openRepo()
	if err != nil { return err }
	sub := ""
	if len(args) > 0 { sub = args[0] }

	switch sub {
	case "start":
		return bisect.Start(r)

	case "bad":
		if !bisect.IsActive(r) { return fmt.Errorf("bisect not started (run 'mygit bisect start')") }
		hash := "HEAD"
		if len(args) > 1 { hash = args[1] }
		h, err := resolveRef(r, hash)
		if err != nil { return err }
		bisect.MarkBad(r, h)
		bisect.AppendLog(r, "bad: "+h)
		fmt.Printf("Marked %s as %s\n", color.Yellow(h[:7]), color.Red("bad"))
		return cmdBisectNext(r)

	case "good":
		if !bisect.IsActive(r) { return fmt.Errorf("bisect not started") }
		hash := "HEAD"
		if len(args) > 1 { hash = args[1] }
		h, err := resolveRef(r, hash)
		if err != nil { return err }
		bisect.MarkGood(r, h)
		bisect.AppendLog(r, "good: "+h)
		fmt.Printf("Marked %s as %s\n", color.Yellow(h[:7]), color.Green("good"))
		return cmdBisectNext(r)

	case "next":
		return cmdBisectNext(r)

	case "reset":
		bisect.Reset(r)
		fmt.Println("Bisect reset")
		return nil

	case "log":
		lines, err := bisect.Log(r)
		if err != nil { return err }
		for _, l := range lines { fmt.Println(l) }
		return nil

	default:
		fmt.Println("Usage: mygit bisect start|good|bad|next|reset|log")
	}
	return nil
}

func cmdBisectNext(r *repo.Repo) error {
	hash, steps, done, err := bisect.Next(r)
	if err != nil { return err }
	if done {
		fmt.Printf("\n%s %s is the first bad commit!\n", color.Bold(color.Red("Found:")), color.Yellow(hash[:7]))
		_, content, _ := object.ReadRaw(r, hash)
		if commit, err := object.ParseCommit(content); err == nil {
			msg := strings.TrimSpace(commit.Message)
			if i := strings.IndexByte(msg, '\n'); i >= 0 { msg = msg[:i] }
			fmt.Printf("  %s\n  %s\n", color.Bold(msg), commit.Author)
		}
		return nil
	}
	// Checkout the midpoint commit
	_, cc, err := object.ReadRaw(r, hash)
	if err != nil { return err }
	commit, err := object.ParseCommit(cc)
	if err != nil { return err }
	checkoutTree(r, commit.Tree, r.Root)
	refs.UpdateHEADDetached(r, hash)

	fmt.Printf("Bisecting: ~%d steps remaining\n", steps)
	fmt.Printf("Checking out %s — test and run 'mygit bisect good' or 'mygit bisect bad'\n", color.Yellow(hash[:7]))
	return nil
}
