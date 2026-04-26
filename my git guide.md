# mygit — Hướng Dẫn Đầy Đủ Từ A đến Z

> Tài liệu ghi lại toàn bộ quá trình xây dựng và trải nghiệm `mygit` — một implementation Git từ đầu bằng Go — từ thiết kế kiến trúc, code từng phase, đến chạy thực tế mọi tính năng. Bao gồm cả những lỗi gặp phải và cách giải quyết.

---

## Mục lục

1. [Mục tiêu & Phạm vi dự án](#1-mục-tiêu--phạm-vi-dự-án)
2. [Kiến trúc & Công nghệ](#2-kiến-trúc--công-nghệ)
3. [Cài đặt môi trường trên Windows](#3-cài-đặt-môi-trường-trên-windows)
4. [Build & Test](#4-build--test)
5. [Trải nghiệm thực tế từ đầu đến cuối](#5-trải-nghiệm-thực-tế-từ-đầu-đến-cuối)
   - [Part 1 — Init & Commit đầu tiên](#part-1--init--commit-đầu-tiên)
   - [Part 2 — Khám phá Object Store (CAS)](#part-2--khám-phá-object-store-cas)
   - [Part 3 — Merkle Tree: Chứng minh Deduplication](#part-3--merkle-tree-chứng-minh-deduplication)
   - [Part 4 — Branch & DAG](#part-4--branch--dag)
   - [Part 5 — Merge: Tạo Merge Commit với 2 Parents](#part-5--merge-tạo-merge-commit-với-2-parents)
   - [Part 6 — diff, show, blame](#part-6--diff-show-blame)
   - [Part 7 — commit --amend, restore, reset](#part-7--commit---amend-restore-reset)
   - [Part 8 — cherry-pick](#part-8--cherry-pick)
   - [Part 9 — rebase (+ xử lý conflict)](#part-9--rebase--xử-lý-conflict)
   - [Part 10 — stash, tag, reflog](#part-10--stash-tag-reflog)
   - [Part 11 — bisect: Tìm Commit Gây Bug](#part-11--bisect-tìm-commit-gây-bug)
   - [Part 12 — fsck & gc](#part-12--fsck--gc)
   - [Part 13 — Kết quả cuối cùng](#part-13--kết-quả-cuối-cùng)
6. [Tổng kết tính năng](#6-tổng-kết-tính-năng)
7. [Key Insights](#7-key-insights)

---

## 1. Mục tiêu & Phạm vi dự án

### Mục tiêu

Implement lại Git từ đầu, tập trung vào việc hiểu sâu **3 concept cốt lõi**:

1. **Content-Addressable Storage (CAS)** — object được định danh bằng hash của content, không phải bằng path/ID tự đặt. Thay đổi 1 byte → hash khác → object khác.
2. **Merkle Tree** — tree object chứa hash của blob/subtree con. Hash của tree cha phụ thuộc vào hash của mọi thứ bên dưới.
3. **DAG của commits** — commit object trỏ tới parent commits + tree root. History là graph, không phải linked list đơn thuần.

### Phạm vi thực hiện

**Đã làm:**

| Phase | Tính năng |
|---|---|
| Phase 1 | `init`, `hash-object`, `cat-file`, object store (CAS + zlib) |
| Phase 2 | `add`, `write-tree`, `ls-tree`, tree object (Merkle tree) |
| Phase 3 | `commit`, `log`, refs, HEAD management |
| Phase 4 | `branch`, `checkout`, `status` |
| Phase 5 | `diff` (LCS line-level), `merge` (three-way + LCA) |
| Extra | `show`, `reset`, `restore`, `commit --amend` |
| Extra | `rebase`, `rebase -i`, `cherry-pick` |
| Extra | `stash`, `tag` (lightweight + annotated) |
| Extra | `blame`, `reflog`, `bisect` |
| Extra | `fsck`, `gc`, `config`, `.mygitignore` |
| Extra | Color output, short hash, `HEAD~N`/`HEAD^` syntax |

**Không làm và lý do:**

- **Pack files** — loose objects đủ để hiểu CAS, pack files là optimization không phải concept
- **Delta encoding** — cùng lý do trên
- **Remote operations** (push/pull/fetch/clone) — network layer không liên quan đến 3 concept mục tiêu
- **GPG signing** — security không phải mục tiêu
- **SHA-256** — chỉ cần SHA-1 như Git truyền thống
- **Submodules, worktree, sparse-checkout** — edge cases phức tạp, không thêm insight về core concepts

---

## 2. Kiến trúc & Công nghệ

### Ngôn ngữ: Go

Lý do chọn Go:
- `crypto/sha1`, `compress/zlib`, `encoding/hex`, `path/filepath` có sẵn trong stdlib
- Binary data handling tự nhiên với `[]byte`
- Cross-platform filesystem (Windows/Linux/Mac)
- Compile nhanh, iteration loop ngắn

### Cấu trúc thư mục

```
mygit/
├── cmd/mygit/
│   ├── main.go          # CLI dispatch cho tất cả commands
│   └── helpers.go       # time + filepath helpers
├── internal/
│   ├── object/
│   │   ├── object.go    # Interface + Type enum (blob/tree/commit/tag)
│   │   ├── blob.go      # Blob type
│   │   ├── tree.go      # Tree type + binary encoding/decoding + sort rules
│   │   ├── commit.go    # Commit type + text encoding/decoding
│   │   ├── hash.go      # SHA-1 hashing helpers
│   │   └── store.go     # Read/write loose objects (zlib compress/decompress)
│   ├── index/
│   │   └── index.go     # Staging area (text format: mode hash path)
│   ├── refs/
│   │   └── refs.go      # HEAD + branch refs management
│   ├── repo/
│   │   └── repo.go      # Repository open/init, path helpers
│   ├── resolve/
│   │   └── resolve.go   # Ref resolution: HEAD~N, HEAD^, short hash, branch, tag
│   ├── diff/
│   │   └── diff.go      # Tree diff (LCS-based line-level diff)
│   ├── merge/
│   │   └── merge.go     # Three-way merge + LCA algorithm
│   ├── patch/
│   │   └── patch.go     # Hunk parsing/applying cho cherry-pick và rebase
│   ├── rebase/
│   │   └── rebase.go    # Rebase, interactive rebase, cherry-pick logic
│   ├── stash/
│   │   └── stash.go     # Stash stack (entries lưu dưới dạng commit objects)
│   ├── reflog/
│   │   └── reflog.go    # Reflog append/read (.mygit/logs/)
│   ├── tag/
│   │   └── tag.go       # Lightweight + annotated tags
│   ├── blame/
│   │   └── blame.go     # Line-level blame qua history traversal
│   ├── fsck/
│   │   └── fsck.go      # Object graph verification + GC
│   ├── bisect/
│   │   └── bisect.go    # Binary search trên commit DAG
│   ├── ignore/
│   │   └── ignore.go    # .mygitignore pattern matching
│   ├── config/
│   │   └── config.go    # ~/.mygitconfig read/write, auto-detect từ real git
│   └── color/
│       └── color.go     # ANSI color output helpers
├── test/
│   └── object_test.go   # Unit tests (18 test cases, Phase 1-3)
└── go.mod
```

### Object format

Mọi object đều có format chung trước khi zlib compress:

```
<type> <size>\0<content>
```

Ví dụ với blob "hello\n":
```
blob 6\0hello\n
```

Hash = SHA-1 của toàn bộ chuỗi trên. Storage:
```
.mygit/objects/<aa>/<bbbb...>
```
Trong đó `aa` = 2 ký tự hex đầu, `bbbb...` = 38 ký tự còn lại.

---

## 3. Cài đặt môi trường trên Windows

### Cài Go

Tải installer `.msi` từ [go.dev/dl](https://go.dev/dl/) → cài mặc định. Go tự thêm vào PATH.

```powershell
go version
# go version go1.26.2 windows/386
```

### Cài Git for Windows

Tải từ [git-scm.com](https://git-scm.com/download/win) → cài mặc định. Cài kèm Git Bash để chạy shell scripts.

```powershell
git --version
# git version 2.54.0.windows.1
```

### Lưu ý Windows CRLF

Trên Windows, `echo "text"` trong PowerShell thêm `\r\n` thay vì `\n`, làm hash khác với Git thật. Luôn tạo file bằng:

```powershell
# Đúng — kiểm soát chính xác bytes
[System.IO.File]::WriteAllBytes("file.txt",
    [System.Text.Encoding]::UTF8.GetBytes("content`n"))

# Sai — PowerShell thêm \r\n
echo "content" > file.txt
```

---

## 4. Build & Test

### Build binary

```powershell
cd E:\mygit-v3
go build -o mygit.exe .\cmd\mygit\
```

### Chạy unit tests

```powershell
go test ./... -v
# === RUN   TestHashEmptyBlob
# --- PASS: TestHashEmptyBlob (0.00s)
# === RUN   TestHashHelloBlob
# --- PASS: TestHashHelloBlob (0.00s)
# ... 18 tests total
# ok  github.com/user/mygit/test  0.056s
```

### Cross-check với Git thật

```powershell
# Empty blob — phải match chính xác
[System.IO.File]::WriteAllBytes("E:\tmp_empty.txt", @())
.\mygit.exe hash-object E:\tmp_empty.txt
# e69de29bb2d1d6434b8b29ae775ad8c2e48c5391

git hash-object E:\tmp_empty.txt
# e69de29bb2d1d6434b8b29ae775ad8c2e48c5391  ← KHỚP

# "hello\n" blob
[System.IO.File]::WriteAllBytes("E:\tmp_hello.txt", [byte[]](0x68,0x65,0x6C,0x6C,0x6F,0x0A))
.\mygit.exe hash-object E:\tmp_hello.txt
# ce013625030ba8dba906f756967f9e9ca394464a

git hash-object E:\tmp_hello.txt
# ce013625030ba8dba906f756967f9e9ca394464a  ← KHỚP
```

### Cấu hình user

```powershell
.\mygit.exe config user.name "HLV"
.\mygit.exe config user.email "lehungkhonghoc@gmail.com"
# Lưu vào ~/.mygitconfig, tự động đọc từ ~/.gitconfig của Git thật nếu không set
```

---

## 5. Trải nghiệm thực tế từ đầu đến cuối

**Setup:**
```
OS: Windows 11, PowerShell
Binary: E:\mygit-v3\mygit.exe
Project: E:\myproject
```

---

### Part 1 — Init & Commit đầu tiên

```powershell
mkdir E:\myproject
cd E:\myproject
E:\mygit-v3\mygit.exe init
# Initialized empty mygit repository in E:\myproject\.mygit
```

Tạo files và commit:

```powershell
[System.IO.File]::WriteAllBytes("E:\myproject\main.py",
    [System.Text.Encoding]::UTF8.GetBytes("# My project`ndef hello():`n    print('Hello World')`n"))
[System.IO.File]::WriteAllBytes("E:\myproject\README.md",
    [System.Text.Encoding]::UTF8.GetBytes("# MyProject`nA simple Python project`n"))
[System.IO.File]::WriteAllBytes("E:\myproject\.mygitignore",
    [System.Text.Encoding]::UTF8.GetBytes("*.pyc`n__pycache__/`n.env`n"))

E:\mygit-v3\mygit.exe add main.py README.md .mygitignore
E:\mygit-v3\mygit.exe status
# On branch main
# Changes staged for commit:
#         new file: .mygitignore
#         new file: README.md
#         new file: main.py

E:\mygit-v3\mygit.exe commit -m "initial: add main.py and README"
# [main 385fd8e] initial: add main.py and README
```

---

### Part 2 — Khám phá Object Store (CAS)

Đây là phần quan trọng nhất — xem Git thật sự lưu gì bên trong.

```powershell
$C1 = (E:\mygit-v3\mygit.exe log --oneline | Select -First 1).Split(" ")[0]

# Xem loại object
E:\mygit-v3\mygit.exe cat-file -t $C1
# commit

# Xem raw content của commit object
E:\mygit-v3\mygit.exe cat-file -p $C1
# tree 1489298fdea432c1231d2a49f9ca9bfd8008ab60
# author HLV <lehungkhonghoc@gmail.com> 1777174588 +0000
# committer HLV <lehungkhonghoc@gmail.com> 1777174588 +0000
# initial: add main.py and README
```

Trace tiếp vào tree object:

```powershell
E:\mygit-v3\mygit.exe cat-file -p 1489298fdea432c1231d2a49f9ca9bfd8008ab60
# 100644 blob 566e1e615865af4a066f0836b945593e27a7837c    .mygitignore
# 100644 blob fbf1e1ed5af23b6ff899dd10848aabf12f100f22    README.md
# 100644 blob d348c7cdfd3a239978d46a29f0dd0abf8ab90cbd    main.py

# Xem content của blob main.py
E:\mygit-v3\mygit.exe cat-file -p d348c7cdfd3a239978d46a29f0dd0abf8ab90cbd
# # My project
# def hello():
#     print('Hello World')

# Verify CAS: hash file trực tiếp phải bằng hash trong store
E:\mygit-v3\mygit.exe hash-object main.py
# d348c7cdfd3a239978d46a29f0dd0abf8ab90cbd  ← KHỚP CHÍNH XÁC
```

Xem objects trên disk — 5 files cho 1 commit:

```powershell
Get-ChildItem -Recurse E:\myproject\.mygit\objects | Where {!$_.PSIsContainer}
# objects/14/89298fdea4...  ← tree
# objects/38/5fd8e190b4...  ← commit
# objects/56/6e1e615865...  ← .mygitignore blob
# objects/d3/48c7cdfd3a...  ← main.py blob
# objects/fb/f1e1ed5af2...  ← README.md blob
```

> **Lưu ý thực hành:** Khi dùng variable để capture hash từ output có màu ANSI, variable sẽ bị dính escape codes. Dùng hash trực tiếp thay vì qua variable cho `cat-file`.

### 💡 Concept 1: Content-Addressable Storage

```
Format object: "<type> <size>\0<content>"
Hash = SHA-1 của toàn bộ chuỗi trên

SHA-1("blob 6\0hello\n") = ce013625030ba8dba906f756967f9e9ca394464a
```

- Object được định danh bằng **hash của content**, không phải tên file hay ID
- Thay đổi 1 byte → hash khác hoàn toàn → object mới hoàn toàn
- File giống nhau dù ở đâu → cùng hash → **1 object duy nhất** trong store (deduplication tự động)
- Không thể sửa content mà không bị phát hiện (tamper-evident by design)
- Storage layout: `objects/<2-char-prefix>/<38-char-suffix>` — 2 ký tự đầu làm tên folder để tránh quá nhiều file trong 1 directory

---

### Part 3 — Merkle Tree: Chứng minh Deduplication

```powershell
# Thêm function mới vào main.py + thêm utils.py mới
# README.md và .mygitignore KHÔNG thay đổi
[System.IO.File]::WriteAllBytes("E:\myproject\main.py",
    [System.Text.Encoding]::UTF8.GetBytes("# My project`ndef hello():`n    print('Hello World')`n`ndef goodbye():`n    print('Goodbye!')`n"))
[System.IO.File]::WriteAllBytes("E:\myproject\utils.py",
    [System.Text.Encoding]::UTF8.GetBytes("def add(a, b):`n    return a + b`n`ndef sub(a, b):`n    return a - b`n"))

E:\mygit-v3\mygit.exe add main.py utils.py
E:\mygit-v3\mygit.exe commit -m "feat: add goodbye() and utils.py"
# [main 1ffa350] feat: add goodbye() and utils.py
```

Đếm objects:

```
Objects trước commit 2: 5
Objects sau commit 2:   9
Objects mới tạo ra:     4  ← không phải 6!
```

Tại sao 4 chứ không phải 6? Vì README.md và .mygitignore không đổi → tái sử dụng blob cũ.

Chứng minh bằng hash:

```powershell
# Tree của commit 2
E:\mygit-v3\mygit.exe cat-file -p 1739fb01d8562f7e7afdadc26521050569e4998c
# 100644 blob 566e1e615865af4a066f0836b945593e27a7837c    .mygitignore  ← GIỐNG commit 1
# 100644 blob fbf1e1ed5af23b6ff899dd10848aabf12f100f22    README.md     ← GIỐNG commit 1
# 100644 blob a1280b9c1eab4b520a7757c22382073e8d5dd21d    main.py       ← KHÁC (blob mới)
# 100644 blob 6cddd28bb99048c80d4abe8866af60262d657159    utils.py      ← MỚI

# README.md blob o commit 1: fbf1e1ed5af23b6ff899dd10848aabf12f100f22
# README.md blob o commit 2: fbf1e1ed5af23b6ff899dd10848aabf12f100f22  ← GIỐNG HỆT
```

### 💡 Concept 2: Merkle Tree

```
Commit 1 (385fd8e)          Commit 2 (1ffa350)
└── Tree 1489298            └── Tree 1739fb0
    ├── .mygitignore 566e1e ──── .mygitignore 566e1e  ← CHUNG (tái sử dụng)
    ├── README.md    fbf1e1 ──── README.md    fbf1e1  ← CHUNG (tái sử dụng)
    ├── main.py      d348c7      main.py      a1280b  ← KHÁC (blob mới)
    └── (chưa có)               utils.py     6cddd2  ← MỚI
```

Hash của tree cha **phụ thuộc vào hash của mọi thứ bên dưới**. Đổi 1 file sâu trong cây → chỉ tạo mới các nodes trên đường đi lên root, còn lại tái sử dụng. Đây là lý do Git hiệu quả với storage dù có nhiều commits.

---

### Part 4 — Branch & DAG

```powershell
# Tạo feature branch từ commit hiện tại
E:\mygit-v3\mygit.exe branch feature-logging
E:\mygit-v3\mygit.exe branch
#   feature-logging 1ffa350
# * main 1ffa350

E:\mygit-v3\mygit.exe checkout feature-logging
# Switched to feature-logging (1ffa350)

# Thêm logging trên feature branch
[System.IO.File]::WriteAllBytes("E:\myproject\utils.py",
    [System.Text.Encoding]::UTF8.GetBytes("def add(a, b):`n    print(f'add {a} + {b}')`n    return a + b`n`ndef sub(a, b):`n    print(f'sub {a} - {b}')`n    return a - b`n"))
[System.IO.File]::WriteAllBytes("E:\myproject\logger.py",
    [System.Text.Encoding]::UTF8.GetBytes("import datetime`n`ndef log(msg):`n    print(f'[{datetime.datetime.now()}] {msg}')`n"))
E:\mygit-v3\mygit.exe add utils.py logger.py
E:\mygit-v3\mygit.exe commit -m "feat: add logging to utils and logger.py"
# [feature-logging 3a95bab] feat: add logging to utils and logger.py

# Song song, main cũng có commit mới
E:\mygit-v3\mygit.exe checkout main
[System.IO.File]::WriteAllBytes("E:\myproject\README.md",
    [System.Text.Encoding]::UTF8.GetBytes("# MyProject`nA simple Python project`n`n## Functions`n- hello()`n- goodbye()`n"))
E:\mygit-v3\mygit.exe add README.md
E:\mygit-v3\mygit.exe commit -m "docs: update README with function list"
# [main 757d7bf] docs: update README with function list
```

DAG lúc này — 2 nhánh phân kỳ từ cùng 1 commit:

```
385fd8e ── 1ffa350 ── 757d7bf   (main)
                  \── 3a95bab   (feature-logging)
```

---

### Part 5 — Merge: Tạo Merge Commit với 2 Parents

```powershell
E:\mygit-v3\mygit.exe checkout main

# Xem diff trước khi merge
E:\mygit-v3\mygit.exe diff main feature-logging
# M  README.md
#  # MyProject
#  A simple Python project
# -
# -## Functions
# -- hello()
# -- goodbye()

E:\mygit-v3\mygit.exe merge feature-logging
# Merge base: 1ffa350
# Merged feature-logging into main → 457a736

E:\mygit-v3\mygit.exe log --graph
# * 457a736 Merge branch 'feature-logging' into main
# |\
# * 757d7bf docs: update README with function list
# * 3a95bab feat: add logging to utils and logger.py
# * 1ffa350 feat: add goodbye() and utils.py
# * 385fd8e initial: add main.py and README

# Xem merge commit object — có 2 dòng parent
E:\mygit-v3\mygit.exe cat-file -p 457a736
# tree 6121b02a4b71b0e59c562fc52e3e14c660888da7
# parent 757d7bf7efc65dd80444aeacd07d61419bb1b29c   ← parent 1: main
# parent 3a95bab5eeb67872d06b67f670e838dccf441463   ← parent 2: feature
# author HLV <lehungkhonghoc@gmail.com> 1777175166 +0000
# committer HLV <lehungkhonghoc@gmail.com> 1777175166 +0000
# Merge branch 'feature-logging' into main

# Verify: cả logger.py và README.md đều có mặt sau merge
dir E:\myproject
# .mygitignore, README.md, logger.py, main.py, utils.py  ← tất cả đủ mặt
```

### 💡 Concept 3: DAG of Commits

```
385fd8e ── 1ffa350 ── 757d7bf ──┐
                  \── 3a95bab ──┴── 457a736 (merge commit)
                                    parents: [757d7bf, 3a95bab]
```

- **DAG** (Directed Acyclic Graph) — không phải linked list đơn
- Merge commit có **2 parents** → tạo thành graph thật sự, không phải chuỗi
- `log --graph` visualize cấu trúc nhánh bằng ASCII
- Traverse DAG = theo parent pointers ngược về root — đây là cách `log`, `bisect`, `merge` hoạt động

---

### Part 6 — diff, show, blame

```powershell
# show: xem chi tiết 1 commit + diff nó thêm gì
E:\mygit-v3\mygit.exe show HEAD~2
# commit 1ffa350150592ab385a820872ba3eea084a6a53d
# Author: HLV <lehungkhonghoc@gmail.com> 1777174912 +0000
#     feat: add goodbye() and utils.py
#
# M  main.py
#  def hello():
# +def goodbye():
# A  utils.py
# +def add(a, b):
# +    return a + b

# diff giữa 2 commit bất kỳ — dùng HEAD~N syntax
E:\mygit-v3\mygit.exe diff HEAD~3 HEAD
# M  README.md  (thêm Functions section)
# A  logger.py  (file mới hoàn toàn)
# M  main.py    (thêm goodbye)
# A  utils.py   (file mới)

# blame: xem ai viết từng dòng, commit nào
E:\mygit-v3\mygit.exe blame utils.py
# 1ffa350 HLV                     1 | def add(a, b):
# 3a95bab HLV                     2 |     print(f'add {a} + {b}')  ← thêm ở feature commit
# 1ffa350 HLV                     3 |     return a + b
# 1ffa350 HLV                     4 |
# 1ffa350 HLV                     5 | def sub(a, b):
# 3a95bab HLV                     6 |     print(f'sub {a} - {b}')  ← thêm ở feature commit
# 1ffa350 HLV                     7 |     return a - b
```

Blame chính xác: `1ffa350` viết skeleton, `3a95bab` (feature-logging commit) thêm 2 dòng print.

---

### Part 7 — commit --amend, restore, reset

#### commit --amend: Sửa commit vừa tạo (typo thường gặp)

```powershell
[System.IO.File]::WriteAllBytes("E:\myproject\config.py",
    [System.Text.Encoding]::UTF8.GetBytes("DEBUG = True`nVERSION = '1.0.0'`n"))
E:\mygit-v3\mygit.exe add config.py
E:\mygit-v3\mygit.exe commit -m "add cofnig.py"   # typo!
# [main 97f24b4] add cofnig.py

E:\mygit-v3\mygit.exe commit --amend -m "add config.py"
# [main 76e496f] (amend) add config.py

E:\mygit-v3\mygit.exe log --oneline -n 2
# 76e496f add config.py    ← hash KHÁC vì content (message) khác → object mới
# 457a736 Merge branch 'feature-logging' into main
```

Hash thay đổi sau amend vì content của commit object thay đổi → SHA-1 khác → object mới. Commit cũ `97f24b4` trở thành dangling object.

#### restore: Bỏ thay đổi chưa commit

```powershell
# Vô tình ghi đè file
[System.IO.File]::WriteAllBytes("E:\myproject\main.py",
    [System.Text.Encoding]::UTF8.GetBytes("# BROKEN CODE`n"))
type E:\myproject\main.py
# # BROKEN CODE  ← file bị phá

E:\mygit-v3\mygit.exe restore main.py
# Restored main.py

type E:\myproject\main.py
# # My project
# def hello():
#     print('Hello World')
# def goodbye():
#     print('Goodbye!')
# ← Khôi phục từ HEAD blob, hoàn toàn chính xác
```

#### reset --soft: Quay lui commit, giữ staged changes

```powershell
E:\mygit-v3\mygit.exe reset --soft HEAD~1
# HEAD is now at 457a736 (soft reset — index unchanged)

E:\mygit-v3\mygit.exe status
# Changes staged for commit:
#         new file: config.py    ← vẫn còn trong stage, commit bị bỏ

# Commit lại với message tốt hơn
E:\mygit-v3\mygit.exe commit -m "config: add config.py with debug flag"
# [main 87ef695] config: add config.py with debug flag
```

---

### Part 8 — cherry-pick

```powershell
# Tạo hotfix branch từ commit cũ hơn (giả lập production branch)
E:\mygit-v3\mygit.exe branch hotfix HEAD~3
E:\mygit-v3\mygit.exe checkout hotfix
# Switched to hotfix (1ffa350)

# Fix bug trên hotfix branch
[System.IO.File]::WriteAllBytes("E:\myproject\main.py",
    [System.Text.Encoding]::UTF8.GetBytes("# My project`ndef hello():`n    print('Hello World')`n`ndef goodbye():`n    print('Goodbye!')`n`ndef version():`n    return '1.0.1'`n"))
E:\mygit-v3\mygit.exe add main.py
E:\mygit-v3\mygit.exe commit -m "hotfix: add version() function"
# [hotfix cd0e3ed] hotfix: add version() function

$HOTFIX = (E:\mygit-v3\mygit.exe log --oneline | Select -First 1).Split(" ")[0]

# Cherry-pick: copy đúng commit đó sang main, không merge cả branch
E:\mygit-v3\mygit.exe checkout main
E:\mygit-v3\mygit.exe cherry-pick $HOTFIX
# [9b4ed57] hotfix: add version() function

type E:\myproject\main.py
# ... def version(): return '1.0.1'   ← có mặt trên main
```

Cherry-pick = áp dụng diff của 1 commit lên commit hiện tại, tạo commit mới với hash mới nhưng cùng message và diff.

---

### Part 9 — rebase (+ xử lý conflict)

#### Lần 1: Conflict — expected behavior

```powershell
E:\mygit-v3\mygit.exe branch feature-config HEAD~2
E:\mygit-v3\mygit.exe checkout feature-config

[System.IO.File]::WriteAllBytes("E:\myproject\config.py",
    [System.Text.Encoding]::UTF8.GetBytes("DEBUG = False`nVERSION = '2.0.0'`nMAX_RETRY = 3`n"))
E:\mygit-v3\mygit.exe add config.py
E:\mygit-v3\mygit.exe commit -m "config: production settings"

E:\mygit-v3\mygit.exe rebase main
# Rebasing 1 commit(s) onto 9b4ed57
# error: conflict applying 613d6de: cherry-pick conflict in: config.py
# Use 'mygit rebase --abort' to cancel
```

**Tại sao conflict?** Cả `main` (`87ef695`) và `feature-config` (`613d6de`) đều sửa `config.py` từ cùng base `457a736`. Three-way merge nhìn thấy:
- Base: `config.py` không tồn tại
- Ours (main): `DEBUG=True, VERSION='1.0.0'`
- Theirs (feature): `DEBUG=False, VERSION='2.0.0', MAX_RETRY=3`

Cả 2 đều thay đổi → không thể tự resolve → **conflict đúng**.

```powershell
E:\mygit-v3\mygit.exe rebase --abort
# Rebase aborted
```

#### Lần 2: Resolve bằng merge

```powershell
E:\mygit-v3\mygit.exe checkout main
E:\mygit-v3\mygit.exe merge feature-config
# Merge base: 457a736
# CONFLICT — Conflicting files:
#   ✗ config.py
# error: resolve conflicts and use 'mygit commit' or 'mygit merge --abort'
```

Resolve tay — quyết định version nào thắng:

```powershell
# Xem 2 version
type E:\myproject\config.py  # main: DEBUG=True, VERSION=1.0.0

# Quyết định: production settings thắng
[System.IO.File]::WriteAllBytes("E:\myproject\config.py",
    [System.Text.Encoding]::UTF8.GetBytes("DEBUG = False`nVERSION = '2.0.0'`nMAX_RETRY = 3`n"))

E:\mygit-v3\mygit.exe add config.py
E:\mygit-v3\mygit.exe commit -m "merge: feature-config with resolved config.py conflict"
# [main da2ab48] merge: feature-config with resolved config.py conflict
```

### 💡 Rebase vs Merge

| | Rebase | Merge |
|---|---|---|
| History | Thẳng hàng (linear) | Giữ nguyên nhánh |
| Khi nào dùng | Branch sửa **file khác** với main | Cùng file hoặc muốn giữ history |
| Conflict risk | Cao hơn — replay từng commit | Thấp hơn — 1 lần merge |
| Workflow | Feature branch cá nhân trước khi push | Merge vào main/develop |

**Workflow chuẩn khi conflict:**
```
1. rebase/merge báo CONFLICT
2. Mở file → xem <<<<<<< HEAD ... ======= ... >>>>>>> theirs
3. Sửa tay → giữ lại những gì muốn
4. mygit add <file>
5. mygit commit (hoặc rebase --continue)
```

---

### Part 10 — stash, tag, reflog

#### stash: Lưu tạm thay đổi chưa commit

```powershell
# Đang làm dở feature, cần switch gấp để fix bug khác
[System.IO.File]::WriteAllBytes("E:\myproject\feature_wip.py",
    [System.Text.Encoding]::UTF8.GetBytes("# Work in progress`ndef new_feature():`n    pass  # TODO`n"))
E:\mygit-v3\mygit.exe add feature_wip.py
E:\mygit-v3\mygit.exe status
# new file: feature_wip.py

E:\mygit-v3\mygit.exe stash push -m "wip: new feature half done"
# Saved working directory state: f1d7675
# Working tree sạch, có thể switch branch

E:\mygit-v3\mygit.exe stash list
# stash@{0}: wip: new feature half done (f1d7675)

# Quay lại làm tiếp
E:\mygit-v3\mygit.exe stash pop
# Restored stash: wip: new feature half done
E:\mygit-v3\mygit.exe status
# new file: feature_wip.py    ← restored
```

Stash không phải magic — nó lưu dưới dạng **commit object thật sự** trong object store, tận dụng CAS.

#### tag: Đánh dấu version release

```powershell
# Lightweight tag — chỉ là pointer đến commit
E:\mygit-v3\mygit.exe tag v1.0.0
# Created tag v1.0.0 → da2ab48

# Annotated tag — có object riêng, có message, có tagger info
E:\mygit-v3\mygit.exe tag -a v2.0.0 -m "Release 2.0 - production config"
# Created annotated tag v2.0.0

E:\mygit-v3\mygit.exe tag -l
# v1.0.0 → da2ab48 (lightweight)
# v2.0.0 → da2ab48 (annotated)

# Checkout theo tag name
E:\mygit-v3\mygit.exe checkout v1.0.0
# Switched to v1.0.0 (da2ab48)
E:\mygit-v3\mygit.exe checkout main
```

#### reflog: Lịch sử đầy đủ của HEAD

```powershell
E:\mygit-v3\mygit.exe reflog
# HEAD@{0}:  da2ab48 checkout: moving to main
# HEAD@{1}:  da2ab48 checkout: moving to v1.0.0
# HEAD@{2}:  da2ab48 commit: merge: feature-config...
# HEAD@{3}:  9b4ed57 checkout: moving to main
# ...
# HEAD@{12}: 457a736 reset: --soft to HEAD~1
# HEAD@{13}: 97f24b4 commit: add cofnig.py    ← typo commit vẫn còn ở đây!
# ...
# HEAD@{22}: 385fd8e commit: initial
```

Reflog ghi lại **mọi di chuyển của HEAD** kể cả commits bị amend, reset, hay rebase abort. Không mất gì cho đến khi `gc` chạy — đây là lưới an toàn cuối cùng.

---

### Part 11 — bisect: Tìm Commit Gây Bug

Tạo scenario thực tế: bug được thêm vào ở đâu đó trong 12 commits.

```powershell
# Thêm vài commits, 1 commit có bug ẩn
E:\mygit-v3\mygit.exe commit -m "wip: add feature_wip.py"

# Commit này thêm buggy() — divide by zero
[System.IO.File]::WriteAllBytes("E:\myproject\main.py",
    [System.Text.Encoding]::UTF8.GetBytes("...`ndef buggy():`n    return 1/0  # BUG`n"))
E:\mygit-v3\mygit.exe add main.py
E:\mygit-v3\mygit.exe commit -m "add buggy function"   # ← BUG ở đây

E:\mygit-v3\mygit.exe commit -m "add test.py"
E:\mygit-v3\mygit.exe commit -m "update README v2"

E:\mygit-v3\mygit.exe log --oneline
# ab5250a update README v2       ← HEAD (có bug)
# af04c49 add test.py
# c519738 add buggy function     ← bug ở đây
# f208d46 wip: add feature_wip.py
# da2ab48 merge: ...
# ... 7 commits nữa ...
# 385fd8e initial                 ← không có bug
```

Chạy bisect — tìm commit gây bug bằng binary search:

```powershell
E:\mygit-v3\mygit.exe bisect start
E:\mygit-v3\mygit.exe bisect bad HEAD      # ab5250a = bad (có bug)
E:\mygit-v3\mygit.exe bisect good 385fd8e  # initial = good (không có bug)
# Bisecting: ~3 steps remaining
# Checking out 9b4ed57            ← checkout commit giữa để test

# Bước 1: test commit 9b4ed57
type E:\myproject\main.py | Select-String "buggy"   # không thấy → good
E:\mygit-v3\mygit.exe bisect good 9b4ed57
# Bisecting: ~2 steps remaining
# Checking out c519738

# Bước 2: test commit c519738
type E:\myproject\main.py | Select-String "buggy"
# def buggy():    ← thấy! → bad
E:\mygit-v3\mygit.exe bisect bad c519738
# Bisecting: ~1 steps remaining
# Checking out f208d46

# Bước 3: test commit f208d46
type E:\myproject\main.py | Select-String "buggy"   # không thấy → good
E:\mygit-v3\mygit.exe bisect good f208d46
# Found: c519738 is the first bad commit!
#   add buggy function
#   HLV <lehungkhonghoc@gmail.com> 1777176133 +0000

E:\mygit-v3\mygit.exe bisect reset
E:\mygit-v3\mygit.exe checkout main
```

### 💡 Bisect = Binary Search trên DAG

```
12 commits, tìm trong 3 bước = O(log n)

Bước 1: test giữa (9b4ed57) → good  → loại nửa dưới
Bước 2: test giữa nửa trên (c519738) → bad → loại nửa trên
Bước 3: test f208d46 → good
→ Kết luận: c519738 = first bad commit
```

Bisect traverse DAG giống như binary search trên mảng đã sort. Với 1000 commits chỉ cần ~10 bước.

---

### Part 12 — fsck & gc

```powershell
# Tổng số objects sau toàn bộ session
$total = (Get-ChildItem -Recurse E:\myproject\.mygit\objects |
    Where {!$_.PSIsContainer}).Count
echo "Tổng objects: $total"
# Tổng objects: 46

# Verify toàn bộ object graph
E:\mygit-v3\mygit.exe fsck
# ok   found 46 objects
# ok   no errors found
# warn dangling object 613d6de   ← feature-config commit trước amend
# warn dangling object 76e496f   ← commit "cofnig.py" trước amend sửa typo
# warn dangling object 97f24b4   ← commit bị xóa bởi reset --soft
# warn dangling object f1d7675   ← stash commit object
# warn dangling object ffcdc2e   ← tree tạo ra trong rebase --abort

# Dọn sạch dangling objects
E:\mygit-v3\mygit.exe gc
# Removed 5 unreachable object(s)

$after = (Get-ChildItem -Recurse E:\myproject\.mygit\objects |
    Where {!$_.PSIsContainer}).Count
echo "Sau gc: $after objects"   # 41
echo "Đã xóa: 5 unreachable objects"
```

### Nguồn gốc 5 dangling objects

| Hash | Tạo ra bởi | Lý do dangling |
|---|---|---|
| `613d6de` | `commit --amend` trên feature-config | Commit cũ bị thay thế bởi commit mới |
| `76e496f` | `commit --amend` sửa typo "cofnig" | Commit cũ bị thay thế |
| `97f24b4` | `reset --soft HEAD~1` | Branch pointer bỏ qua commit này |
| `f1d7675` | `stash push` | Stash đã được pop, không còn reference |
| `ffcdc2e` | `rebase --abort` | Partial tree tạo ra trước khi abort |

**Đây chính xác cách Git thật hoạt động.** Amend/reset/rebase không xóa objects ngay — chúng chỉ tạo objects mới và bỏ reference đến objects cũ. GC mới thật sự xóa.

---

### Part 13 — Kết quả cuối cùng

```powershell
E:\mygit-v3\mygit.exe log --graph
# * ab5250a update README v2
# * af04c49 add test.py
# * c519738 add buggy function
# * f208d46 wip: add feature_wip.py
# * da2ab48 merge: feature-config with resolved config.py conflict
# * 9b4ed57 hotfix: add version() function
# * 87ef695 config: add config.py with debug flag
# * 457a736 Merge branch 'feature-logging' into main
# |\
# * 757d7bf docs: update README with function list
# * 3a95bab feat: add logging to utils and logger.py
# * 1ffa350 feat: add goodbye() and utils.py
# * 385fd8e initial: add main.py and README
```

12 commits, 1 merge nhánh hiển thị rõ trong graph, 41 objects trong store sau gc.

---

## 6. Tổng kết tính năng

| Nhóm | Lệnh | Đã dùng |
|---|---|---|
| **Core** | `init`, `add`, `commit`, `status` | ✅ |
| **History** | `log --oneline --graph -n --author` | ✅ |
| **Inspect** | `cat-file -t/-p/-s`, `hash-object`, `ls-tree`, `write-tree` | ✅ |
| **Diff** | `diff`, `show`, `blame` | ✅ |
| **Branch** | `branch`, `checkout`, `branch -d` | ✅ |
| **Merge** | `merge`, `merge --abort` | ✅ |
| **Rebase** | `rebase`, `rebase -i`, `rebase --abort` | ✅ |
| **Fix** | `commit --amend`, `reset --soft/--hard`, `restore` | ✅ |
| **Copy** | `cherry-pick` | ✅ |
| **Stash** | `stash push/pop/list/drop` | ✅ |
| **Tag** | `tag` lightweight + annotated, `tag -l/-d` | ✅ |
| **Debug** | `reflog`, `bisect start/good/bad/reset` | ✅ |
| **Internals** | `fsck`, `gc` | ✅ |
| **Config** | `config user.name/user.email` | ✅ |
| **Ref syntax** | `HEAD~N`, `HEAD^`, short hash, tag name, branch name | ✅ |

---

## 7. Key Insights

**1. Mọi thứ trong Git đều là content-addressed object.**
Commits, trees, blobs, tags, thậm chí stash — tất cả đều là objects trong `.mygit/objects/`, được định danh bằng SHA-1 của content. Không có database, không có ID tự tăng — chỉ có hash.

**2. Amend/reset/rebase không xóa data.**
Chúng tạo objects mới và bỏ reference đến objects cũ. Objects cũ vẫn tồn tại cho đến khi GC chạy. Reflog ghi lại tất cả mọi di chuyển của HEAD — đây là lưới an toàn: nếu lỡ reset nhầm, vẫn có thể recover qua reflog trước khi GC dọn.

**3. Conflict là behavior đúng, không phải bug.**
Khi 2 branch sửa cùng 1 file từ cùng base, three-way merge không thể tự quyết định nên giữ version nào — đó là quyết định của developer. Tool đúng là tool báo conflict rõ ràng, không phải tool im lặng chọn một version rồi tạo ra kết quả sai.

**4. Rebase và merge giải quyết cùng vấn đề theo cách khác nhau.**
Rebase tạo history thẳng hàng dễ đọc nhưng risk conflict cao hơn vì phải replay từng commit. Merge giữ nguyên lịch sử phân nhánh trung thực hơn nhưng graph phức tạp hơn. Không có cái nào đúng tuyệt đối — tùy context và team convention.

**5. Bisect là binary search trên DAG.**
12 commits cần 3 bước, 1000 commits cần ~10 bước. Đây là ứng dụng trực tiếp của binary search trên cấu trúc đồ thị — hiểu DAG là hiểu tại sao bisect hiệu quả.
