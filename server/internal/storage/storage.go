// Package storage handles artifact and annotation files on disk.
//
// It is the single home for the slug/path-traversal guards: every path that
// reaches the filesystem is validated and resolved strictly inside the
// artifacts directory before any file is opened.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Errors returned by the storage layer. Callers use errors.Is to branch.
var (
	// ErrInvalidID means the id failed validation (bad characters, traversal,
	// etc.) and no filesystem access was attempted.
	ErrInvalidID = errors.New("invalid artifact id")
	// ErrNotFound means the id is valid but no such artifact exists.
	ErrNotFound = errors.New("artifact not found")
)

// idPattern is the full, exact grammar for an artifact id. Because it forbids
// '.', '/', and '\', it structurally rules out path traversal; resolvePath adds
// a second, defence-in-depth check that the resolved path stays inside the dir.
var idPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// Store owns an artifacts directory and mediates all access to it.
type Store struct {
	dir string
}

// New returns a Store rooted at dir.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// Artifact is a listing entry for one artifact file.
type Artifact struct {
	ID      string
	Size    int64
	ModTime time.Time
}

// ValidID reports whether id is a well-formed artifact id.
func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

// resolvePath validates id and returns the absolute path to its .html file,
// guaranteed to live directly inside the store's directory. It performs no I/O.
func (s *Store) resolvePath(id, suffix string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	path := filepath.Join(s.dir, id+suffix)

	// Defence in depth: confirm the cleaned path is a direct child of dir and
	// never escapes it, regardless of the validation above.
	rel, err := filepath.Rel(s.dir, path)
	if err != nil || rel != id+suffix || strings.Contains(rel, "..") {
		return "", fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return path, nil
}

// ReadArtifact returns the raw HTML of the artifact with the given id.
func (s *Store) ReadArtifact(id string) ([]byte, error) {
	path, err := s.resolvePath(id, ".html")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("read artifact %q: %w", id, err)
	}
	return data, nil
}

// ArtifactExists reports whether an artifact with the given id exists.
func (s *Store) ArtifactExists(id string) (bool, error) {
	path, err := s.resolvePath(id, ".html")
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat artifact %q: %w", id, err)
	}
	return true, nil
}

// WriteAnnotations persists the annotation JSON for id to
// <id>.annotations.json, creating or overwriting it.
func (s *Store) WriteAnnotations(id string, data []byte) error {
	path, err := s.resolvePath(id, ".annotations.json")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write annotations %q: %w", id, err)
	}
	return nil
}

// ReadAnnotations returns the raw annotation JSON for id, or ErrNotFound if
// none has been written.
func (s *Store) ReadAnnotations(id string) ([]byte, error) {
	path, err := s.resolvePath(id, ".annotations.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("read annotations %q: %w", id, err)
	}
	return data, nil
}

// List returns every artifact in the directory, newest first. A missing
// directory is treated as empty, not an error.
func (s *Store) List() ([]Artifact, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}

	var arts []Artifact
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		id := strings.TrimSuffix(name, ".html")
		if !ValidID(id) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat artifact %q: %w", id, err)
		}
		arts = append(arts, Artifact{ID: id, Size: info.Size(), ModTime: info.ModTime()})
	}

	sort.Slice(arts, func(i, j int) bool {
		if arts[i].ModTime.Equal(arts[j].ModTime) {
			return arts[i].ID < arts[j].ID
		}
		return arts[i].ModTime.After(arts[j].ModTime)
	})
	return arts, nil
}
