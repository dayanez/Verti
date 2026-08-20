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
