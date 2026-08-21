package gitstatus

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initRepo creates a fresh git repo with a pinned branch name and local
// identity, so tests don't depend on the running machine's global git
// config (init.defaultBranch, user.name/email).
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.email", "test@example.com")
	run(t, dir, "config", "user.name", "Test")
	return dir
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadOnNonGitDirectoryReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	st := Load(dir)
	if st.Branch != "" {
		t.Errorf("Branch = %q, want empty for a non-git directory", st.Branch)
	}
	if len(st.Dirty) != 0 {
		t.Errorf("Dirty = %v, want empty for a non-git directory", st.Dirty)
	}
}

func TestLoadParsesBranchNameWithNoCommitsYet(t *testing.T) {
	dir := initRepo(t)
	st := Load(dir)
	if st.Branch != "main" {
		t.Errorf("Branch = %q, want %q", st.Branch, "main")
	}
}

func TestLoadDetectsUntrackedFile(t *testing.T) {
	dir := initRepo(t)
	mustWrite(t, filepath.Join(dir, "new.txt"), "hello\n")

	st := Load(dir)
	if code, ok := st.Dirty["new.txt"]; !ok || code != '?' {
		t.Errorf("Dirty[new.txt] = %q, ok=%v, want '?'", code, ok)
	}
}

func TestLoadDetectsModifiedTrackedFile(t *testing.T) {
	dir := initRepo(t)
	mustWrite(t, filepath.Join(dir, "tracked.txt"), "original\n")
	run(t, dir, "add", "tracked.txt")
	run(t, dir, "commit", "-q", "-m", "initial")

	mustWrite(t, filepath.Join(dir, "tracked.txt"), "changed\n")
	st := Load(dir)
	if st.Branch != "main" {
		t.Errorf("Branch = %q, want %q", st.Branch, "main")
	}
	if code, ok := st.Dirty["tracked.txt"]; !ok || code != 'M' {
		t.Errorf("Dirty[tracked.txt] = %q, ok=%v, want 'M'", code, ok)
	}
}

func TestLoadDetectsStagedAddition(t *testing.T) {
	dir := initRepo(t)
	mustWrite(t, filepath.Join(dir, "committed.txt"), "x\n")
	run(t, dir, "add", "committed.txt")
	run(t, dir, "commit", "-q", "-m", "initial")

	mustWrite(t, filepath.Join(dir, "staged.txt"), "y\n")
	run(t, dir, "add", "staged.txt")
	st := Load(dir)
	if code, ok := st.Dirty["staged.txt"]; !ok || code != 'A' {
		t.Errorf("Dirty[staged.txt] = %q, ok=%v, want 'A'", code, ok)
	}
}

func TestLoadDetectsDeletedFile(t *testing.T) {
	dir := initRepo(t)
	path := filepath.Join(dir, "gone.txt")
	mustWrite(t, path, "bye\n")
	run(t, dir, "add", "gone.txt")
	run(t, dir, "commit", "-q", "-m", "initial")

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	st := Load(dir)
	if code, ok := st.Dirty["gone.txt"]; !ok || code != 'D' {
		t.Errorf("Dirty[gone.txt] = %q, ok=%v, want 'D'", code, ok)
	}
}

func TestLoadOnSubdirectoryOfALargerRepoUsesSubdirRelativePaths(t *testing.T) {
	repo := initRepo(t)
	sub := filepath.Join(repo, "sub")
	mustWrite(t, filepath.Join(sub, "tracked.txt"), "original\n")
	run(t, repo, "add", "sub/tracked.txt")
	run(t, repo, "commit", "-q", "-m", "initial")

	mustWrite(t, filepath.Join(sub, "tracked.txt"), "changed\n")
	mustWrite(t, filepath.Join(sub, "new.txt"), "hello\n")

	// Load is pointed at the subdirectory, not the repo root: `git status
	// --porcelain` always reports paths relative to the repo's top level
	// regardless of `-C`, so without stripping that prefix these would
	// show up as "sub/tracked.txt" and "sub/new.txt" instead of matching
	// the subdir-relative paths callers (the explorer) actually look up.
	st := Load(sub)
	if code, ok := st.Dirty["tracked.txt"]; !ok || code != 'M' {
		t.Errorf("Dirty[tracked.txt] = %q, ok=%v, want 'M'; Dirty = %v", code, ok, st.Dirty)
	}
	if code, ok := st.Dirty["new.txt"]; !ok || code != '?' {
		t.Errorf("Dirty[new.txt] = %q, ok=%v, want '?'; Dirty = %v", code, ok, st.Dirty)
	}
}

func TestUnquoteGitPathReversesOctalAndBackslashEscapes(t *testing.T) {
	cases := map[string]string{
		`plain.txt`:         "plain.txt",
		`"caf\303\251.txt"`: "café.txt",
		`"a\\b.txt"`:        `a\b.txt`,
		`"quote\".txt"`:     `quote".txt`,
		`"tab\there.txt"`:   "tab\there.txt",
	}
	for in, want := range cases {
		if got := unquoteGitPath(in); got != want {
			t.Errorf("unquoteGitPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadUnescapesNonASCIIFilenames(t *testing.T) {
	dir := initRepo(t)
	mustWrite(t, filepath.Join(dir, "café.txt"), "hello\n")

	st := Load(dir)
	if code, ok := st.Dirty["café.txt"]; !ok || code != '?' {
		t.Errorf("Dirty[café.txt] = %q, ok=%v, want '?'; Dirty = %v", code, ok, st.Dirty)
	}
}

func TestLoadCleanRepoHasNothingDirty(t *testing.T) {
	dir := initRepo(t)
	mustWrite(t, filepath.Join(dir, "clean.txt"), "x\n")
	run(t, dir, "add", "clean.txt")
	run(t, dir, "commit", "-q", "-m", "initial")

	st := Load(dir)
	if len(st.Dirty) != 0 {
		t.Errorf("Dirty = %v, want empty for a clean repo", st.Dirty)
	}
	if st.Branch != "main" {
		t.Errorf("Branch = %q, want %q", st.Branch, "main")
	}
}
