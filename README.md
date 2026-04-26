# mygit

A Git implementation built from scratch in Go, focusing on the three core concepts that Git is built upon.

## Core Concepts

| Concept | Description |
|---|---|
| **Content-Addressable Storage** | Every object is identified by the SHA-1 hash of its content |
| **Merkle Tree** | Tree objects whose hash depends on all descendants — enabling structural sharing |
| **DAG of Commits** | Commits form a Directed Acyclic Graph via parent pointers |

## Features

```
init · add · commit · status · log
diff · show · blame
branch · checkout · merge · rebase · cherry-pick
commit --amend · reset · restore
stash · tag · reflog · bisect
fsck · gc · cat-file · hash-object
```

## Quick Start

Requires Go 1.21+

```bash
git clone <repo-url>
cd mygit-v3

# Build
go build -o mygit ./cmd/mygit/        # Linux/Mac
go build -o mygit.exe .\cmd\mygit\    # Windows

# Configure
./mygit config user.name "Your Name"
./mygit config user.email "you@example.com"

# Init a repo and start
mkdir myproject && cd myproject
../mygit init
../mygit help
```

## Run Tests

```bash
go test ./...
```

## Project Structure

```
mygit-v3/
├── cmd/mygit/         # CLI entry point
├── internal/
│   ├── object/        # Blob, tree, commit objects + CAS storage
│   ├── diff/          # LCS-based line diff
│   ├── merge/         # Three-way merge + LCA
│   ├── rebase/        # Rebase + cherry-pick
│   ├── bisect/        # Binary search on commit DAG
│   ├── resolve/       # Ref resolution: HEAD~N, short hash, branch, tag
│   ├── stash/         # Stash stack
│   ├── tag/           # Lightweight + annotated tags
│   ├── blame/         # Line-level blame
│   ├── fsck/          # Object graph verification + GC
│   ├── reflog/        # HEAD history
│   ├── config/        # User config
│   └── color/         # Terminal color output
└── test/              # Unit tests
```

## Docs

- [`design.md`](./design.md) — Why this project exists
- [`mygit-complete-guide.md`](./mygit-complete-guide.md) — Full walkthrough

## Out of Scope

Pack files, delta encoding, remote operations, GPG signing — not relevant to the three core concepts this project targets.

