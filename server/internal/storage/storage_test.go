package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadArtifactRejectsInvalidID(t *testing.T) {
	// A secret file living OUTSIDE the artifacts dir. No invalid id must ever
	// resolve to it (or anywhere outside the dir).
	base := t.TempDir()
	artDir := filepath.Join(base, "artifacts")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "secret.html"), []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(artDir)

	tests := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"dotdot", ".."},
		{"parent traversal", "../secret"},
		{"nested traversal", "../../etc/passwd"},
		{"forward slash", "a/b"},
		{"back slash", `a\b`},
		{"absolute path", "/etc/passwd"},
		{"single dot", "."},
		{"leading dot", ".hidden"},
		{"embedded dots", "a..b"},
		{"uppercase", "React"},
		{"underscore", "a_b"},
		{"space", "a b"},
		{"unicode", "café"},
		{"null byte", "a\x00b"},
		{"dotdot with html", "..%2fsecret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.ReadArtifact(tt.id)
			if !errors.Is(err, ErrInvalidID) {
				t.Fatalf("ReadArtifact(%q): want ErrInvalidID, got %v", tt.id, err)
			}
		})
	}
}

func TestReadArtifactReadsValidArtifact(t *testing.T) {
	dir := t.TempDir()
	id := "react-vs-vue-20260721-103000"
	want := []byte("<!doctype html><title>hi</title>")
	if err := os.WriteFile(filepath.Join(dir, id+".html"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := New(dir).ReadArtifact(id)
	if err != nil {
		t.Fatalf("ReadArtifact: unexpected error %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("ReadArtifact: got %q, want %q", got, want)
	}
}

func TestReadArtifactMissingReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := New(dir).ReadArtifact("does-not-exist-20260721-000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListReturnsArtifactsSortedNewestFirst(t *testing.T) {
	dir := t.TempDir()
	// Only .html files are artifacts; annotation/other files are ignored.
	for _, name := range []string{"a-1.html", "b-2.html", "c-3.html", "notes.txt", "a-1.annotations.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := New(dir).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List: want 3 artifacts, got %d (%v)", len(got), got)
	}
	ids := map[string]bool{}
	for _, a := range got {
		ids[a.ID] = true
		if a.ID == "" {
			t.Fatal("artifact has empty id")
		}
	}
	for _, want := range []string{"a-1", "b-2", "c-3"} {
		if !ids[want] {
			t.Fatalf("List: missing artifact %q in %v", want, got)
		}
	}
}

func TestAnnotationsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id := "react-vs-vue-20260721-103000"
	s := New(dir)

	want := []byte(`{"artifactId":"react-vs-vue-20260721-103000","annotations":[]}`)
	if err := s.WriteAnnotations(id, want); err != nil {
		t.Fatalf("WriteAnnotations: %v", err)
	}
	got, err := s.ReadAnnotations(id)
	if err != nil {
		t.Fatalf("ReadAnnotations: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, want)
	}
	// It must land at <id>.annotations.json, next to the artifact.
	if _, err := os.Stat(filepath.Join(dir, id+".annotations.json")); err != nil {
		t.Fatalf("annotations file not written where expected: %v", err)
	}
}

func TestWriteAnnotationsRejectsInvalidID(t *testing.T) {
	s := New(t.TempDir())
	if err := s.WriteAnnotations("../evil", []byte("{}")); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("WriteAnnotations(bad id): want ErrInvalidID, got %v", err)
	}
}

func TestReadAnnotationsMissingReturnsNotFound(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.ReadAnnotations("valid-20260101-000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReadAnnotations(absent): want ErrNotFound, got %v", err)
	}
}

func TestArtifactExists(t *testing.T) {
	dir := t.TempDir()
	id := "present-20260101-000000"
	if err := os.WriteFile(filepath.Join(dir, id+".html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(dir)

	if ok, err := s.ArtifactExists(id); err != nil || !ok {
		t.Fatalf("ArtifactExists(present): want true,nil got %v,%v", ok, err)
	}
	if ok, err := s.ArtifactExists("absent-20260101-000000"); err != nil || ok {
		t.Fatalf("ArtifactExists(absent): want false,nil got %v,%v", ok, err)
	}
	if _, err := s.ArtifactExists("../evil"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("ArtifactExists(bad id): want ErrInvalidID, got %v", err)
	}
}

func TestListOnMissingDirReturnsEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created-yet")
	got, err := New(dir).List()
	if err != nil {
		t.Fatalf("List on missing dir: want nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List on missing dir: want empty, got %v", got)
	}
}
