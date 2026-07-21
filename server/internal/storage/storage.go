// Package storage handles artifact and annotation files on disk.
//
// It is the single home for the slug/path-traversal guards: every path that
// reaches the filesystem is validated and resolved strictly inside the
// artifacts directory before any file is opened. Phase 2 fills in the read,
// write, and listing operations; Phase 0 ships the package so the module
// compiles.
package storage

// Store owns an artifacts directory and mediates all access to it.
type Store struct {
	dir string
}

// New returns a Store rooted at dir.
func New(dir string) *Store {
	return &Store{dir: dir}
}
