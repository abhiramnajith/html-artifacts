package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultArtifactsDirUnderHome(t *testing.T) {
	got := defaultArtifactsDir()
	if !strings.HasSuffix(filepath.ToSlash(got), ".html-artifacts/artifacts") {
		t.Fatalf("defaultArtifactsDir = %q, want it to end with .html-artifacts/artifacts", got)
	}
}

func TestRenderCmdWritesArtifact(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "note.md")
	if err := os.WriteFile(md, []byte("# Hello\n\nWorld **bold**.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderCmd([]string{"--dir", dir, md}); err != nil {
		t.Fatalf("renderCmd: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "note-*.html"))
	if len(matches) != 1 {
		t.Fatalf("expected one rendered artifact, got %v", matches)
	}
	body, _ := os.ReadFile(matches[0])
	if !strings.Contains(string(body), "<h1>Hello</h1>") || !strings.Contains(string(body), "<strong>bold</strong>") {
		t.Fatalf("rendered artifact missing converted content:\n%s", body)
	}
}

func TestRenderCmdRejectsInvalidID(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "n.md")
	os.WriteFile(md, []byte("# hi\n"), 0o644)
	if err := renderCmd([]string{"--dir", dir, "--id", "Bad_ID", md}); err == nil {
		t.Fatal("expected error for invalid --id")
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.html")); len(matches) != 0 {
		t.Fatalf("no artifact should be written for invalid id, got %v", matches)
	}
}
