package patchapply_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/floatpane/go-mailpatch"
	"github.com/floatpane/go-patchapply"
)

func mustParseDiff(t *testing.T, diff string) []mailpatch.FileChange {
	t.Helper()
	files, err := mailpatch.ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff: %v", err)
	}
	return files
}

const modifyDiff = `diff --git a/greet.txt b/greet.txt
index 111..222 100644
--- a/greet.txt
+++ b/greet.txt
@@ -1,3 +1,3 @@
 hello
-world
+there
 bye
`

func TestApplyModify(t *testing.T) {
	fsys := patchapply.NewMemFS(map[string][]byte{
		"greet.txt": []byte("hello\nworld\nbye\n"),
	})
	res, err := patchapply.Apply(fsys, mustParseDiff(t, modifyDiff), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].Status != patchapply.Updated {
		t.Fatalf("result = %+v", res.Files)
	}
	got, _ := fsys.ReadFile("greet.txt")
	if string(got) != "hello\nthere\nbye\n" {
		t.Errorf("content = %q", got)
	}
}

func TestApplyOffset(t *testing.T) {
	// The hunk says line 2, but the real file has two extra leading lines.
	// Application should still locate the context and patch it.
	fsys := patchapply.NewMemFS(map[string][]byte{
		"greet.txt": []byte("pre1\npre2\nhello\nworld\nbye\n"),
	})
	if _, err := patchapply.Apply(fsys, mustParseDiff(t, modifyDiff), nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, _ := fsys.ReadFile("greet.txt")
	if string(got) != "pre1\npre2\nhello\nthere\nbye\n" {
		t.Errorf("content = %q", got)
	}
}

const addDiff = `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..222
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+first
+second
`

func TestApplyAdd(t *testing.T) {
	fsys := patchapply.NewMemFS(nil)
	res, err := patchapply.Apply(fsys, mustParseDiff(t, addDiff), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Files[0].Status != patchapply.Created {
		t.Errorf("status = %v", res.Files[0].Status)
	}
	got, _ := fsys.ReadFile("new.txt")
	if string(got) != "first\nsecond\n" {
		t.Errorf("content = %q", got)
	}
}

func TestApplyAddExisting(t *testing.T) {
	fsys := patchapply.NewMemFS(map[string][]byte{"new.txt": []byte("x\n")})
	_, err := patchapply.Apply(fsys, mustParseDiff(t, addDiff), nil)
	if !errors.Is(err, patchapply.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	// Nothing should have been overwritten.
	if got, _ := fsys.ReadFile("new.txt"); string(got) != "x\n" {
		t.Errorf("file was clobbered: %q", got)
	}
}

const deleteDiff = `diff --git a/gone.txt b/gone.txt
deleted file mode 100644
index 111..0000000
--- a/gone.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-one
-two
`

func TestApplyDelete(t *testing.T) {
	fsys := patchapply.NewMemFS(map[string][]byte{"gone.txt": []byte("one\ntwo\n")})
	res, err := patchapply.Apply(fsys, mustParseDiff(t, deleteDiff), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Files[0].Status != patchapply.Removed {
		t.Errorf("status = %v", res.Files[0].Status)
	}
	if fsys.Exists("gone.txt") {
		t.Error("file still exists")
	}
}

const renameDiff = `diff --git a/old.txt b/new.txt
similarity index 80%
rename from old.txt
rename to new.txt
index 111..222 100644
--- a/old.txt
+++ b/new.txt
@@ -1,2 +1,2 @@
 keep
-was
+now
`

func TestApplyRename(t *testing.T) {
	fsys := patchapply.NewMemFS(map[string][]byte{"old.txt": []byte("keep\nwas\n")})
	res, err := patchapply.Apply(fsys, mustParseDiff(t, renameDiff), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Files[0].Status != patchapply.Renamed || res.Files[0].OldPath != "old.txt" {
		t.Errorf("result = %+v", res.Files[0])
	}
	if fsys.Exists("old.txt") {
		t.Error("old path still exists")
	}
	got, _ := fsys.ReadFile("new.txt")
	if string(got) != "keep\nnow\n" {
		t.Errorf("content = %q", got)
	}
}

func TestConflict(t *testing.T) {
	// File does not contain the expected "world" context line.
	fsys := patchapply.NewMemFS(map[string][]byte{
		"greet.txt": []byte("hello\nMARS\nbye\n"),
	})
	_, err := patchapply.Apply(fsys, mustParseDiff(t, modifyDiff), nil)
	if !errors.Is(err, patchapply.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	// Transactional: original untouched.
	if got, _ := fsys.ReadFile("greet.txt"); string(got) != "hello\nMARS\nbye\n" {
		t.Errorf("file changed despite conflict: %q", got)
	}
}

func TestDryRun(t *testing.T) {
	fsys := patchapply.NewMemFS(map[string][]byte{
		"greet.txt": []byte("hello\nworld\nbye\n"),
	})
	res, err := patchapply.Apply(fsys, mustParseDiff(t, modifyDiff), &patchapply.Options{DryRun: true})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("results = %d", len(res.Files))
	}
	if got, _ := fsys.ReadFile("greet.txt"); string(got) != "hello\nworld\nbye\n" {
		t.Errorf("dry run wrote to disk: %q", got)
	}
}

func TestReverseRoundTrip(t *testing.T) {
	orig := []byte("hello\nworld\nbye\n")
	fsys := patchapply.NewMemFS(map[string][]byte{"greet.txt": orig})
	files := mustParseDiff(t, modifyDiff)

	if _, err := patchapply.Apply(fsys, files, nil); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if got, _ := fsys.ReadFile("greet.txt"); string(got) != "hello\nthere\nbye\n" {
		t.Fatalf("forward content = %q", got)
	}
	if _, err := patchapply.Apply(fsys, files, &patchapply.Options{Reverse: true}); err != nil {
		t.Fatalf("reverse: %v", err)
	}
	if got, _ := fsys.ReadFile("greet.txt"); string(got) != string(orig) {
		t.Errorf("reverse did not restore: %q", got)
	}
}

func TestApplyToBytes(t *testing.T) {
	files := mustParseDiff(t, modifyDiff)
	out, err := patchapply.ApplyToBytes([]byte("hello\nworld\nbye\n"), files[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello\nthere\nbye\n" {
		t.Errorf("out = %q", out)
	}
}

func TestUnsafePath(t *testing.T) {
	dir := t.TempDir()
	fsys := patchapply.NewDirFS(dir)

	escape := `diff --git a/x b/x
new file mode 100644
--- /dev/null
+++ b/../../escape.txt
@@ -0,0 +1 @@
+pwned
`
	_, err := patchapply.ApplyDiff(fsys, escape, nil)
	if !errors.Is(err, patchapply.ErrUnsafePath) {
		t.Fatalf("err = %v, want ErrUnsafePath", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); statErr == nil {
		t.Fatal("escaped the root!")
	}
}

func TestDirFSRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "greet.txt"), []byte("hello\nworld\nbye\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := patchapply.NewDirFS(dir)
	if _, err := patchapply.ApplyDiff(fsys, modifyDiff, nil); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "greet.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello\nthere\nbye\n" {
		t.Errorf("content = %q", got)
	}
}

func TestMissingTarget(t *testing.T) {
	fsys := patchapply.NewMemFS(nil)
	_, err := patchapply.Apply(fsys, mustParseDiff(t, modifyDiff), nil)
	if !errors.Is(err, patchapply.ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
}
