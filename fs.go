package patchapply

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FS is the filesystem an apply runs against: read the current files, write new
// content, and remove deletions. DirFS (rooted at a directory) and MemFS (in
// memory) implement it. Implement it yourself to apply patches against, say, a
// git object store or a virtual tree.
type FS interface {
	// ReadFile returns the current contents of name. It must report a
	// not-exist error (testable with os.IsNotExist / fs.ErrNotExist) when the
	// file is absent.
	ReadFile(name string) ([]byte, error)
	// WriteFile creates or overwrites name with data, creating parent
	// directories as needed.
	WriteFile(name string, data []byte, perm fs.FileMode) error
	// Remove deletes name.
	Remove(name string) error
	// Exists reports whether name currently exists.
	Exists(name string) bool
}

// DirFS is an FS rooted at a directory on the real filesystem. Every path is
// confined to the root: a patch whose path escapes it (via "../" or an
// absolute path) fails with ErrUnsafePath instead of touching anything
// outside.
type DirFS struct {
	root string
}

// NewDirFS returns a DirFS rooted at dir.
func NewDirFS(dir string) *DirFS {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return &DirFS{root: filepath.Clean(abs)}
}

// resolve maps a patch path to an absolute path inside the root, rejecting any
// path that would escape it (an absolute path, or one that climbs out via
// "../").
func (d *DirFS) resolve(name string) (string, error) {
	unsafe := func() (string, error) {
		return "", &PathError{Path: name, Err: ErrUnsafePath}
	}
	if filepath.IsAbs(name) {
		return unsafe()
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return unsafe()
	}
	full := filepath.Join(d.root, clean)
	rel, err := filepath.Rel(d.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return unsafe()
	}
	return full, nil
}

func (d *DirFS) ReadFile(name string) ([]byte, error) {
	full, err := d.resolve(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (d *DirFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	full, err := d.resolve(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	if perm == 0 {
		perm = 0o644
	}
	return os.WriteFile(full, data, perm)
}

func (d *DirFS) Remove(name string) error {
	full, err := d.resolve(name)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (d *DirFS) Exists(name string) bool {
	full, err := d.resolve(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(full)
	return err == nil
}

// MemFS is an in-memory FS, handy for tests and for applying a patch to a set
// of files you already hold in memory. The zero value is not usable; use
// NewMemFS.
type MemFS struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMemFS returns an empty MemFS. Seed it with WriteFile or by passing initial
// files.
func NewMemFS(initial map[string][]byte) *MemFS {
	m := &MemFS{files: make(map[string][]byte, len(initial))}
	for k, v := range initial {
		m.files[memKey(k)] = append([]byte(nil), v...)
	}
	return m
}

func memKey(name string) string {
	return filepath.ToSlash(filepath.Clean("/" + filepath.ToSlash(name)))[1:]
}

func (m *MemFS) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.files[memKey(name)]
	if !ok {
		return nil, &PathError{Path: name, Err: fs.ErrNotExist}
	}
	return append([]byte(nil), b...), nil
}

func (m *MemFS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[memKey(name)] = append([]byte(nil), data...)
	return nil
}

func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, memKey(name))
	return nil
}

func (m *MemFS) Exists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.files[memKey(name)]
	return ok
}

// Files returns a snapshot of the current contents, keyed by clean path.
func (m *MemFS) Files() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]byte, len(m.files))
	for k, v := range m.files {
		out[k] = append([]byte(nil), v...)
	}
	return out
}
