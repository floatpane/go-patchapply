<div align="center">

# go-patchapply

**Apply parsed patches to files, in Go. Add / modify / delete / rename, reverse, dry-run — confined to a directory, transactional, never runs git.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/floatpane/go-patchapply)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/floatpane/go-patchapply.svg)](https://pkg.go.dev/github.com/floatpane/go-patchapply)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/floatpane/go-patchapply)](https://github.com/floatpane/go-patchapply/releases)
[![CI](https://github.com/floatpane/go-patchapply/actions/workflows/ci.yml/badge.svg)](https://github.com/floatpane/go-patchapply/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

`go-patchapply` is the **apply** half of [go-mailpatch](https://github.com/floatpane/go-mailpatch). mailpatch turns a `git format-patch` email into structured `FileChange`s; patchapply takes those — or a bare unified diff — and writes the changes to a filesystem: creating, modifying, deleting, renaming, and copying files. It can reverse a patch and dry-run one. It **never executes git**.

It grew out of [matcha](https://github.com/floatpane/matcha)'s `git-mail` feature, where applying a mailed patch must be safe against hostile paths and must not half-apply.

## Features

- **Every change type.** Added, deleted, modified, renamed, copied — driven straight off `mailpatch.FileChange`.
- **Offset-tolerant, exact-context hunks.** A hunk still applies when earlier edits shifted the file; context lines must match (no silent fuzz), and a hunk that can't be placed fails with `ErrConflict`.
- **Transactional.** Every file is read and every hunk placed in memory first. If any hunk fails, **nothing is written** — no half-applied trees.
- **Confined by construction.** `DirFS` pins every path inside a root; a patch that tries to escape via `../` or an absolute path fails with `ErrUnsafePath` before touching disk.
- **Reverse & dry-run.** `Options{Reverse: true}` unapplies; `Options{DryRun: true}` validates the whole patch and writes nothing.
- **Apply anywhere.** Run against the OS (`DirFS`), entirely in memory (`MemFS`), or your own `FS` (a git object store, a virtual tree…).
- **Zero third-party deps beyond go-mailpatch.** Standard library otherwise.

## Install

```bash
go get github.com/floatpane/go-patchapply
```

Requires Go 1.26+.

## Usage

### Apply a patch email to a directory

```go
package main

import (
	"log"
	"os"

	"github.com/floatpane/go-mailpatch"
	"github.com/floatpane/go-patchapply"
)

func main() {
	raw, _ := os.ReadFile("fix.patch")

	p, err := mailpatch.ParseBytes(raw) // parse the format-patch email
	if err != nil {
		log.Fatal(err)
	}

	fsys := patchapply.NewDirFS("/path/to/repo") // confined to this root
	res, err := patchapply.ApplyPatch(fsys, p, nil)
	if err != nil {
		log.Fatal(err) // ErrConflict, ErrUnsafePath, ErrMissing, ErrExists
	}

	for _, f := range res.Files {
		log.Printf("%s %s", f.Status, f.Path) // updated/created/removed/renamed
	}
}
```

### Apply a bare diff in memory

```go
fsys := patchapply.NewMemFS(map[string][]byte{
	"greet.txt": []byte("hello\nworld\nbye\n"),
})

_, err := patchapply.ApplyDiff(fsys, diffText, nil)

out, _ := fsys.ReadFile("greet.txt")
```

### Dry-run, then reverse

```go
// Validate without writing — nil error means a real apply would succeed.
if _, err := patchapply.Apply(fsys, files, &patchapply.Options{DryRun: true}); err != nil {
	log.Fatal("patch will not apply cleanly:", err)
}

// Apply, then later undo.
patchapply.Apply(fsys, files, nil)
patchapply.Apply(fsys, files, &patchapply.Options{Reverse: true})
```

### Just transform bytes

```go
// No filesystem at all: apply one file's hunks to content you hold.
newContent, err := patchapply.ApplyToBytes(oldContent, fileChange)
```

## API at a glance

| Function | Does |
|----------|------|
| `Apply(fs, []FileChange, *Options)` | apply already-parsed changes |
| `ApplyDiff(fs, diff, *Options)` | parse a bare diff (via go-mailpatch) and apply |
| `ApplyPatch(fs, *mailpatch.Patch, *Options)` | apply a parsed patch email |
| `ApplyToBytes(orig, FileChange)` | pure, filesystem-free single-file apply |
| `NewDirFS(root)` / `NewMemFS(seed)` | the two built-in `FS` implementations |

`Options{Reverse, DryRun}`. Errors: `ErrConflict`, `ErrUnsafePath`, `ErrMissing`, `ErrExists` (compare with `errors.Is`).

## What this is not

- **Not a merge tool.** No 3-way merge, no conflict markers. A hunk that doesn't apply is an error, not a `<<<<<<<`.
- **Not `git am`.** It writes files; it does not create commits, move HEAD, or touch the index.
- **Not a fuzzy patcher.** It tolerates line *offset*, not changed context. That's deliberate — applying a security-relevant patch to the wrong place silently is worse than failing.

> [!NOTE]
> End-of-file newline state (`\ No newline at end of file`) is not tracked; a
> file produced from an addition ends with a newline.

## Documentation

Full API reference: [pkg.go.dev/github.com/floatpane/go-patchapply](https://pkg.go.dev/github.com/floatpane/go-patchapply)

Guides and diagrams: see [`docs/`](docs/).

## Sister projects

| Project | Role |
|---------|------|
| [floatpane/go-mailpatch](https://github.com/floatpane/go-mailpatch) | The parser half — turns format-patch email into the `FileChange`s this library applies. |
| [floatpane/matcha](https://github.com/floatpane/matcha) | Reference consumer — `git-mail` patch apply. |
| [floatpane/go-secretbox](https://github.com/floatpane/go-secretbox) | Sibling extraction — password-based encryption for data at rest. |

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

It writes files from untrusted patches. Path-confinement matters — report vulnerabilities privately via [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
