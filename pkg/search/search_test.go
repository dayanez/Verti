package search

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestFilesFindsMatchesAcrossMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc needle() {}\n")
	writeFile(t, filepath.Join(dir, "sub", "b.go"), "package b\n\n// contains needle too\n")
	writeFile(t, filepath.Join(dir, "c.go"), "package c\n\nfunc unrelated() {}\n")

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2: %+v", len(matches), matches)
	}

	byPath := map[string]Match{}
	for _, m := range matches {
		byPath[filepath.ToSlash(m.Path)] = m
	}
	if m, ok := byPath["a.go"]; !ok || m.Line != 3 {
		t.Errorf("a.go match = %+v, ok=%v, want line 3", m, ok)
	}
	if m, ok := byPath["sub/b.go"]; !ok || m.Line != 3 {
		t.Errorf("sub/b.go match = %+v, ok=%v, want line 3", m, ok)
	}
}

func TestFilesSkipsDotDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".git", "config"), "needle inside git metadata\n")
	writeFile(t, filepath.Join(dir, "real.go"), "needle in real source\n")

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1 (the .git file should be skipped): %+v", len(matches), matches)
	}
	if !strings.HasSuffix(filepath.ToSlash(matches[0].Path), "real.go") {
		t.Errorf("match path = %q, want it to be real.go", matches[0].Path)
	}
}

func TestFilesSkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(binPath, []byte("needle\x00binary junk"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeFile(t, filepath.Join(dir, "text.go"), "needle in text\n")

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1 (the binary file should be skipped): %+v", len(matches), matches)
	}
}

func TestFilesSkipsOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("x"), maxFileSize+1)
	copy(big, []byte("needle"))
	if err := os.WriteFile(filepath.Join(dir, "huge.txt"), big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	writeFile(t, filepath.Join(dir, "small.txt"), "needle\n")

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1 (the oversized file should be skipped): %+v", len(matches), matches)
	}
}

func TestFilesFindsMatchOnAnOverlongSingleLine(t *testing.T) {
	dir := t.TempDir()
	// One line with no newline at all, under maxFileSize but over the
	// scanner's old 1MB token cap: the scanner's max token size must
	// track maxFileSize, since a line can be as long as the whole file.
	line := bytes.Repeat([]byte("x"), maxFileSize-100)
	line = append(line, []byte("needle")...)
	if err := os.WriteFile(filepath.Join(dir, "long.txt"), line, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1: a match on an overlong single line must not be silently dropped", len(matches))
	}
}

func TestFilesSkipsGitignoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n")
	writeFile(t, filepath.Join(dir, "node_modules", "dep", "index.js"), "needle in a dependency\n")
	writeFile(t, filepath.Join(dir, "real.go"), "needle in real source\n")

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1 (node_modules should be skipped): %+v", len(matches), matches)
	}
	if !strings.HasSuffix(filepath.ToSlash(matches[0].Path), "real.go") {
		t.Errorf("match path = %q, want it to be real.go", matches[0].Path)
	}
}

func TestListFilesReturnsNamesNotContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n")
	writeFile(t, filepath.Join(dir, "node_modules", "dep", "index.js"), "ignored\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	writeFile(t, filepath.Join(dir, "sub", "b.go"), "package b\n")
	writeFile(t, filepath.Join(dir, ".git", "config"), "should be skipped\n")

	names, err := ListFiles(dir)
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	want := map[string]bool{"a.go": true, "sub/b.go": true, ".gitignore": true}
	got := map[string]bool{}
	for _, n := range names {
		got[filepath.ToSlash(n)] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("ListFiles() missing %q, got %v", name, names)
		}
	}
	if got["node_modules/dep/index.js"] {
		t.Error("ListFiles() should not include gitignored node_modules")
	}
	for name := range got {
		if strings.Contains(name, ".git/") {
			t.Errorf("ListFiles() should not include .git contents, got %q", name)
		}
	}
}

func TestFilesReturnsNilForEmptyQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "anything\n")

	matches, err := Files(dir, "")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0 for an empty query", len(matches))
	}
}

func TestFilesCapsResultsAtMaxMatches(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 0; i < maxMatches+50; i++ {
		sb.WriteString("needle\n")
	}
	writeFile(t, filepath.Join(dir, "many.txt"), sb.String())

	matches, err := Files(dir, "needle")
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(matches) != maxMatches {
		t.Fatalf("len(matches) = %d, want the capped %d", len(matches), maxMatches)
	}
}
