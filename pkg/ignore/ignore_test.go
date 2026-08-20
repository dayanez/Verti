package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMatchesTopLevelDirAndGlobPattern(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n*.log\n")
	m := Load(root)

	if !m.Match("node_modules", true) {
		t.Error("node_modules/ should match the dirOnly pattern")
	}
	if m.Match("node_modules", false) {
		t.Error("a file named node_modules should not match a dirOnly pattern")
	}
	if !m.Match("debug.log", false) {
		t.Error("debug.log should match *.log")
	}
	if m.Match("README.md", false) {
		t.Error("README.md should not be matched by any rule")
	}
}

func TestUnanchoredPatternMatchesAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "__pycache__/\n")
	m := Load(root)

	if !m.Match("a/b/__pycache__", true) {
		t.Error("unanchored pattern should match __pycache__ nested arbitrarily deep")
	}
}

func TestAnchoredPatternOnlyMatchesAtRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "/dist\n")
	m := Load(root)

	if !m.Match("dist", true) {
		t.Error("/dist should match the root-level dist")
	}
	if m.Match("src/dist", true) {
		t.Error("/dist is anchored to the root and should not match src/dist")
	}
}

func TestNegationReincludesLaterInFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n!keep.log\n")
	m := Load(root)

	if !m.Match("debug.log", false) {
		t.Error("debug.log should still be ignored")
	}
	if m.Match("keep.log", false) {
		t.Error("keep.log should be re-included by the negated rule")
	}
}

func TestCommentsAndBlankLinesIgnored(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "# a comment\n\n*.log\n")
	m := Load(root)

	if !m.Match("debug.log", false) {
		t.Error("*.log should still match past a comment and blank line")
	}
}

func TestMissingGitignoreMatchesNothing(t *testing.T) {
	root := t.TempDir()
	m := Load(root)
	if m.Match("node_modules", true) {
		t.Error("with no .gitignore present, Match should always return false")
	}
}

func TestNilMatcherMatchesNothing(t *testing.T) {
	var m *Matcher
	if m.Match("anything", true) {
		t.Error("a nil Matcher should match nothing")
	}
}

func TestRelSlashNormalizesSeparators(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "c", "d.go")
	if got := RelSlash(root, full); got != "c/d.go" {
		t.Errorf("RelSlash() = %q, want %q", got, "c/d.go")
	}
}
