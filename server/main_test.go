package main

import (
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
