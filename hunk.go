package patchapply

import (
	"fmt"
	"strings"

	"github.com/floatpane/go-mailpatch"
)

// ApplyToBytes applies a single file's hunks to orig and returns the new
// contents. It is the pure, filesystem-free core: read a file yourself, pass
// its bytes here, write the result yourself.
//
// Hunks are located at the line numbers the diff records, but a whole-hunk
// offset is tolerated (so a patch still applies when earlier, unrelated edits
// shifted the file). Context lines must match exactly — there is no fuzz. A
// hunk that cannot be placed returns an error wrapping ErrConflict.
func ApplyToBytes(orig []byte, f mailpatch.FileChange) ([]byte, error) {
	src, trailingNL := splitLines(orig)
	if len(orig) == 0 {
		// A file built from nothing (an addition) gets a trailing newline,
		// matching how git writes added text files.
		trailingNL = true
	}

	out := make([]string, 0, len(src))
	cursor := 0
	for i, h := range f.Hunks {
		oldBlock, newBlock := buildBlocks(h)
		pos, ok := locate(src, oldBlock, h.OldStart-1, cursor)
		if !ok {
			return nil, fmt.Errorf("%w: %s hunk %d (@@ -%d,%d)",
				ErrConflict, f.Path(), i+1, h.OldStart, h.OldLines)
		}
		out = append(out, src[cursor:pos]...)
		out = append(out, newBlock...)
		cursor = pos + len(oldBlock)
	}
	out = append(out, src[cursor:]...)

	return joinLines(out, trailingNL), nil
}

// buildBlocks splits a hunk into the lines it expects to find (context +
// deletions) and the lines it produces (context + additions).
func buildBlocks(h mailpatch.Hunk) (oldBlock, newBlock []string) {
	for _, ln := range h.Lines {
		switch ln.Kind {
		case mailpatch.Context:
			oldBlock = append(oldBlock, ln.Text)
			newBlock = append(newBlock, ln.Text)
		case mailpatch.Delete:
			oldBlock = append(oldBlock, ln.Text)
		case mailpatch.Add:
			newBlock = append(newBlock, ln.Text)
		}
	}
	return oldBlock, newBlock
}

// locate finds where oldBlock sits in src, preferring the expected index and
// searching outward, never before minPos. For an empty oldBlock (a pure
// insertion) it returns the clamped expected index.
func locate(src, oldBlock []string, expected, minPos int) (int, bool) {
	if expected < minPos {
		expected = minPos
	}
	if len(oldBlock) == 0 {
		if expected > len(src) {
			expected = len(src)
		}
		return expected, true
	}
	last := len(src) - len(oldBlock)
	if last < minPos {
		return 0, false
	}
	if expected > last {
		expected = last
	}
	// Expand outward from the expected position: 0, +1, -1, +2, -2, ...
	for delta := 0; ; delta++ {
		fwd := expected + delta
		bwd := expected - delta
		tried := false
		if fwd <= last {
			tried = true
			if matchAt(src, oldBlock, fwd) {
				return fwd, true
			}
		}
		if delta != 0 && bwd >= minPos {
			tried = true
			if matchAt(src, oldBlock, bwd) {
				return bwd, true
			}
		}
		if !tried {
			return 0, false
		}
	}
}

func matchAt(src, block []string, pos int) bool {
	for i, line := range block {
		if src[pos+i] != line {
			return false
		}
	}
	return true
}

// splitLines splits content into lines, reporting whether it ended with a
// newline so the result can be reconstructed faithfully.
func splitLines(b []byte) (lines []string, trailingNL bool) {
	if len(b) == 0 {
		return nil, false
	}
	s := string(b)
	if strings.HasSuffix(s, "\n") {
		trailingNL = true
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n"), trailingNL
}

func joinLines(lines []string, trailingNL bool) []byte {
	if len(lines) == 0 {
		if trailingNL {
			return []byte("\n")
		}
		return nil
	}
	s := strings.Join(lines, "\n")
	if trailingNL {
		s += "\n"
	}
	return []byte(s)
}
