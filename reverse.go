package patchapply

import "github.com/floatpane/go-mailpatch"

// reverseFile inverts a FileChange so applying it undoes the original change:
// additions become deletions, deleted/added files swap, paths and modes swap,
// and every hunk is flipped.
func reverseFile(f mailpatch.FileChange) mailpatch.FileChange {
	r := f
	r.OldPath, r.NewPath = f.NewPath, f.OldPath
	r.OldMode, r.NewMode = f.NewMode, f.OldMode
	r.Additions, r.Deletions = f.Deletions, f.Additions

	switch f.Type {
	case mailpatch.Added:
		r.Type = mailpatch.Deleted
	case mailpatch.Deleted:
		r.Type = mailpatch.Added
	case mailpatch.Copied:
		// Undoing a copy removes the copy that was made.
		r.Type = mailpatch.Deleted
	case mailpatch.Renamed, mailpatch.Modified:
		r.Type = f.Type
	}

	r.Hunks = make([]mailpatch.Hunk, len(f.Hunks))
	for i, h := range f.Hunks {
		r.Hunks[i] = reverseHunk(h)
	}
	return r
}

func reverseHunk(h mailpatch.Hunk) mailpatch.Hunk {
	r := mailpatch.Hunk{
		OldStart: h.NewStart,
		OldLines: h.NewLines,
		NewStart: h.OldStart,
		NewLines: h.OldLines,
		Section:  h.Section,
		Lines:    make([]mailpatch.Line, len(h.Lines)),
	}
	for i, ln := range h.Lines {
		switch ln.Kind {
		case mailpatch.Add:
			r.Lines[i] = mailpatch.Line{Kind: mailpatch.Delete, Text: ln.Text}
		case mailpatch.Delete:
			r.Lines[i] = mailpatch.Line{Kind: mailpatch.Add, Text: ln.Text}
		case mailpatch.Context:
			r.Lines[i] = ln
		}
	}
	return r
}
