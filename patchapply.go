// Package patchapply applies parsed patches to files.
//
// It is the apply half of [go-mailpatch]: mailpatch turns a format-patch email
// into structured FileChanges; patchapply takes those (or a bare unified diff)
// and writes the changes to a filesystem — creating, modifying, deleting,
// renaming, and copying files. It can also reverse a patch and dry-run one
// without writing.
//
// Apply runs against an FS, not directly against the OS. Use DirFS to confine
// every path to a directory (patches that try to escape via "../" fail with
// ErrUnsafePath), or MemFS to apply entirely in memory.
//
// Application is transactional: every file is read and every hunk is placed in
// memory first, so if any hunk fails to apply (ErrConflict) nothing is written.
//
// It never executes git.
//
// [go-mailpatch]: https://github.com/floatpane/go-mailpatch
package patchapply

import (
	"errors"
	"io/fs"
	"strconv"

	"github.com/floatpane/go-mailpatch"
)

// Sentinel errors. Compare with errors.Is.
var (
	// ErrConflict is returned (wrapped, with the file and hunk) when a hunk's
	// context cannot be matched in the target file.
	ErrConflict = errors.New("patchapply: hunk does not apply")
	// ErrUnsafePath is returned by DirFS when a patch path escapes the root.
	ErrUnsafePath = errors.New("patchapply: unsafe path")
	// ErrExists is returned when a patch adds a file that is already present.
	ErrExists = errors.New("patchapply: file already exists")
	// ErrMissing is returned when a patch modifies, deletes, or renames a file
	// that is not present.
	ErrMissing = errors.New("patchapply: target file not found")
)

// PathError annotates an error with the offending path.
type PathError struct {
	Path string
	Err  error
}

func (e *PathError) Error() string { return e.Err.Error() + ": " + e.Path }
func (e *PathError) Unwrap() error { return e.Err }

// Options tunes an apply.
type Options struct {
	// Reverse undoes the patch instead of applying it.
	Reverse bool
	// DryRun validates the whole patch (reads and places every hunk) but
	// writes nothing. A nil error means a real apply would succeed.
	DryRun bool
}

// Status is what happened to a file.
type Status int

const (
	// Created means the file was written and did not exist before.
	Created Status = iota
	// Updated means an existing file's contents changed.
	Updated
	// Removed means the file was deleted.
	Removed
	// Renamed means the file moved (and possibly changed).
	Renamed
)

func (s Status) String() string {
	switch s {
	case Created:
		return "created"
	case Updated:
		return "updated"
	case Removed:
		return "removed"
	case Renamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// FileResult records what an apply did (or, for a dry run, would do) to one
// file.
type FileResult struct {
	Path    string // resulting path; for a removal, the path removed
	OldPath string // previous path for a rename, else empty
	Status  Status
	Hunks   int // hunks applied
}

// Result is the outcome of an apply.
type Result struct {
	Files []FileResult
}

// Apply applies files to fsys. opts may be nil.
func Apply(fsys FS, files []mailpatch.FileChange, opts *Options) (*Result, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Phase 1: read + compute every change in memory. Nothing is written, so a
	// failure here leaves fsys untouched.
	plan := make([]planContent, 0, len(files))
	for _, f := range files {
		if opts.Reverse {
			f = reverseFile(f)
		}
		p, err := planFile(fsys, f)
		if err != nil {
			return nil, err
		}
		plan = append(plan, p)
	}

	res := &Result{Files: make([]FileResult, 0, len(plan))}
	for _, p := range plan {
		res.Files = append(res.Files, p.result)
	}
	if opts.DryRun {
		return res, nil
	}

	// Phase 2: commit. Writes first, then removes (so a rename's new file
	// exists before the old one goes away).
	for _, p := range plan {
		if p.doWrite {
			if err := fsys.WriteFile(p.writePath, p.content, p.perm); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range plan {
		if p.doRemove {
			if err := fsys.Remove(p.removePath); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

// planContent is the per-file plan computed in phase 1.
type planContent struct {
	writePath  string
	content    []byte
	perm       fs.FileMode
	doWrite    bool
	removePath string
	doRemove   bool
	result     FileResult
}

func planFile(fsys FS, f mailpatch.FileChange) (planContent, error) {
	var p planContent
	switch f.Type {
	case mailpatch.Added:
		target := f.Path()
		if fsys.Exists(target) {
			return p, &PathError{Path: target, Err: ErrExists}
		}
		content, err := ApplyToBytes(nil, f)
		if err != nil {
			return p, err
		}
		p = planContent{
			writePath: target, content: content, perm: parseMode(f.NewMode), doWrite: true,
			result: FileResult{Path: target, Status: Created, Hunks: len(f.Hunks)},
		}

	case mailpatch.Deleted:
		target := f.OldPath
		if target == "" {
			target = f.Path()
		}
		if !fsys.Exists(target) {
			return p, &PathError{Path: target, Err: ErrMissing}
		}
		p = planContent{
			removePath: target, doRemove: true,
			result: FileResult{Path: target, Status: Removed, Hunks: len(f.Hunks)},
		}

	case mailpatch.Renamed:
		orig, err := readExisting(fsys, f.OldPath)
		if err != nil {
			return p, err
		}
		content, err := ApplyToBytes(orig, f)
		if err != nil {
			return p, err
		}
		p = planContent{
			writePath: f.NewPath, content: content, perm: parseMode(f.NewMode), doWrite: true,
			removePath: f.OldPath, doRemove: true,
			result: FileResult{Path: f.NewPath, OldPath: f.OldPath, Status: Renamed, Hunks: len(f.Hunks)},
		}

	case mailpatch.Copied:
		orig, err := readExisting(fsys, f.OldPath)
		if err != nil {
			return p, err
		}
		content, err := ApplyToBytes(orig, f)
		if err != nil {
			return p, err
		}
		p = planContent{
			writePath: f.NewPath, content: content, perm: parseMode(f.NewMode), doWrite: true,
			result: FileResult{Path: f.NewPath, Status: Created, Hunks: len(f.Hunks)},
		}

	default: // Modified
		target := f.Path()
		orig, err := readExisting(fsys, target)
		if err != nil {
			return p, err
		}
		content, err := ApplyToBytes(orig, f)
		if err != nil {
			return p, err
		}
		p = planContent{
			writePath: target, content: content, perm: parseMode(f.NewMode), doWrite: true,
			result: FileResult{Path: target, Status: Updated, Hunks: len(f.Hunks)},
		}
	}
	return p, nil
}

func readExisting(fsys FS, name string) ([]byte, error) {
	b, err := fsys.ReadFile(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &PathError{Path: name, Err: ErrMissing}
		}
		return nil, err
	}
	return b, nil
}

func parseMode(mode string) fs.FileMode {
	if mode == "" {
		return 0
	}
	// git modes look like "100644"; the low 9 bits are the unix permission.
	n, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return 0
	}
	return fs.FileMode(n).Perm()
}

// ApplyDiff parses a bare unified diff and applies it to fsys.
func ApplyDiff(fsys FS, diff string, opts *Options) (*Result, error) {
	files, err := mailpatch.ParseDiff(diff)
	if err != nil {
		return nil, err
	}
	return Apply(fsys, files, opts)
}

// ApplyPatch applies a parsed format-patch email (its FileChanges) to fsys.
func ApplyPatch(fsys FS, p *mailpatch.Patch, opts *Options) (*Result, error) {
	if p == nil {
		return &Result{}, nil
	}
	return Apply(fsys, p.Files, opts)
}
