package explorer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mkTestTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNewLoadsTopLevelSortedDirsFirst(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.flat) != 2 {
		t.Fatalf("len(flat) = %d, want 2", len(e.flat))
	}
	if !e.flat[0].node.IsDir || e.flat[0].node.Name != "src" {
		t.Fatalf("first entry = %+v, want dir 'src'", e.flat[0].node)
	}
	if e.flat[1].node.Name != "README.md" {
		t.Fatalf("second entry name = %q, want README.md", e.flat[1].node.Name)
	}
}

func TestToggleExpandsAndCollapses(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	// cursor starts at 0 = "src"
	if err := e.Toggle(); err != nil {
		t.Fatal(err)
	}
	if len(e.flat) != 3 {
		t.Fatalf("after expand, len(flat) = %d, want 3", len(e.flat))
	}
	if e.flat[1].node.Name != "main.go" || e.flat[1].depth != 1 {
		t.Fatalf("expanded child = %+v, want main.go at depth 1", e.flat[1])
	}
	if err := e.Toggle(); err != nil {
		t.Fatal(err)
	}
	if len(e.flat) != 2 {
		t.Fatalf("after collapse, len(flat) = %d, want 2", len(e.flat))
	}
}

func TestHandleEnterRequiresTwoPressesForFiles(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveDown() // onto README.md
	path, _, ok := e.Selected()
	if !ok || filepath.Base(path) != "README.md" {
		t.Fatalf("Selected() = %q, want README.md", path)
	}

	opened, err := e.HandleEnter()
	if err != nil {
		t.Fatal(err)
	}
	if opened != "" {
		t.Fatalf("first Enter on a file opened it immediately: %q", opened)
	}

	opened, err = e.HandleEnter()
	if err != nil {
		t.Fatal(err)
	}
	if opened == "" {
		t.Fatal("second Enter within the window did not open the file")
	}
}

func TestHandleEnterTogglesDirsImmediately(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := e.HandleEnter() // cursor on "src" dir
	if err != nil {
		t.Fatal(err)
	}
	if opened != "" {
		t.Fatal("Enter on a directory should never return an open path")
	}
	if !e.Root.Children[0].Expanded {
		t.Fatal("single Enter on a directory should expand it")
	}
}

func TestMoveUpDownClampsAtEdges(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveUp()
	if e.cursor != 0 {
		t.Fatalf("MoveUp at top: cursor = %d, want 0", e.cursor)
	}
	e.MoveDown()
	e.MoveDown()
	e.MoveDown()
	if e.cursor != len(e.flat)-1 {
		t.Fatalf("MoveDown past bottom: cursor = %d, want %d", e.cursor, len(e.flat)-1)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func flatNames(e *Explorer) []string {
	names := make([]string, len(e.flat))
	for i, entry := range e.flat {
		names[i] = entry.node.Name
	}
	return names
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func TestGitignoreHidesMatchedTopLevelEntries(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustMkdir(t, filepath.Join(root, "src"))
	mustWrite(t, filepath.Join(root, "src", "main.go"), "package main\n")
	mustWrite(t, filepath.Join(root, "debug.log"), "boom\n")
	mustWrite(t, filepath.Join(root, "README.md"), "# hi\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\n*.log\n")

	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	names := flatNames(e)
	if containsName(names, "node_modules") {
		t.Fatalf("flat entries = %v, node_modules should be hidden by .gitignore", names)
	}
	if containsName(names, "debug.log") {
		t.Fatalf("flat entries = %v, debug.log should be hidden by *.log", names)
	}
	if !containsName(names, "src") || !containsName(names, "README.md") {
		t.Fatalf("flat entries = %v, want src and README.md still visible", names)
	}
}

func TestGitignoreUnanchoredPatternMatchesAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "a", "__pycache__"))
	mustMkdir(t, filepath.Join(root, "a", "b"))
	mustWrite(t, filepath.Join(root, ".gitignore"), "__pycache__/\n")

	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Toggle(); err != nil { // expand "a"
		t.Fatal(err)
	}
	names := flatNames(e)
	if containsName(names, "__pycache__") {
		t.Fatalf("flat entries = %v, nested __pycache__ should be hidden by unanchored pattern", names)
	}
	if !containsName(names, "b") {
		t.Fatalf("flat entries = %v, want sibling dir b still visible", names)
	}
}

func TestGitignoreAnchoredPatternOnlyMatchesAtRoot(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "dist"))
	mustMkdir(t, filepath.Join(root, "src", "dist"))
	mustWrite(t, filepath.Join(root, ".gitignore"), "/dist\n")

	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if containsName(flatNames(e), "dist") {
		t.Fatal("root-level dist should be hidden by anchored /dist pattern")
	}
	if err := e.Toggle(); err != nil { // expand "src", which sorts before the (now hidden) root dist would have
		t.Fatal(err)
	}
	if !containsName(flatNames(e), "dist") {
		t.Fatal("nested src/dist should stay visible: /dist is anchored to the workspace root")
	}
}

func TestGitignoreNegationReincludesPath(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "debug.log"), "boom\n")
	mustWrite(t, filepath.Join(root, "keep.log"), "keep me\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n!keep.log\n")

	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	names := flatNames(e)
	if containsName(names, "debug.log") {
		t.Fatalf("flat entries = %v, debug.log should still be hidden", names)
	}
	if !containsName(names, "keep.log") {
		t.Fatalf("flat entries = %v, keep.log should be re-included by the negated rule", names)
	}
}

func TestNoGitignoreShowsEverything(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "node_modules"))

	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsName(flatNames(e), "node_modules") {
		t.Fatal("with no .gitignore present, nothing should be filtered")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func TestBranchReflectsWorkspaceGitBranch(t *testing.T) {
	root := initGitRepo(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Branch(); got != "main" {
		t.Fatalf("Branch() = %q, want %q", got, "main")
	}
}

func TestBranchEmptyForNonGitWorkspace(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Branch(); got != "" {
		t.Fatalf("Branch() = %q, want empty for a non-git workspace", got)
	}
}

func TestRenderShowsGitStatusLetterOnDirtyFile(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.SetSize(40, 10)
	rendered := e.Render(false)
	if !strings.Contains(rendered, "new.txt ?") {
		t.Fatalf("Render() = %q, want a line for new.txt suffixed with the untracked status '?'", rendered)
	}
}

func TestReloadGitStatusPicksUpChangesMadeAfterNew(t *testing.T) {
	root := initGitRepo(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.SetSize(40, 10)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(e.Render(false), "new.txt") {
		t.Fatal("new.txt shouldn't be visible before Refresh loads it into the tree")
	}
	if err := e.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.Render(false), "new.txt ?") {
		t.Fatalf("Render() after Refresh() = %q, want new.txt shown with its untracked status", e.Render(false))
	}
}

func TestCreateFileAtWorkspaceRootWhenNothingSelected(t *testing.T) {
	root := t.TempDir()
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CreateFile("new.go"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if !containsName(flatNames(e), "new.go") {
		t.Fatalf("flat entries = %v, want new.go", flatNames(e))
	}
	if path, isDir, ok := e.Selected(); !ok || isDir || filepath.Base(path) != "new.go" {
		t.Fatalf("Selected() = (%q, %v, %v), want the newly created file selected", path, isDir, ok)
	}
}

func TestCreateFileFailsIfAlreadyExists(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveDown() // onto README.md, a file at the root: TargetDir() is root, its parent
	if err := e.CreateFile("README.md"); err == nil {
		t.Fatal("CreateFile over an existing file should fail, not silently truncate it")
	}
}

func TestCreateFileInsideSelectedDirectory(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	// cursor starts on "src" (dirs sort first)
	if err := e.CreateFile("helper.go"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src", "helper.go")); err != nil {
		t.Fatalf("expected helper.go inside src/: %v", err)
	}
}

func TestCreateFileNextToSelectedFile(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveDown() // onto README.md, a file, at the root
	if err := e.CreateFile("SIBLING.md"); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "SIBLING.md")); err != nil {
		t.Fatalf("expected SIBLING.md next to README.md at the root: %v", err)
	}
}

func TestCreateDir(t *testing.T) {
	root := t.TempDir()
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CreateDir("newdir"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "newdir"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected newdir/ to exist as a directory: %v", err)
	}
}

func TestRenameSelectedEntry(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveDown() // onto README.md
	if err := e.Rename("RENAMED.md"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "RENAMED.md")); err != nil {
		t.Fatalf("expected RENAMED.md to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("README.md should no longer exist after rename")
	}
	if path, _, ok := e.Selected(); !ok || filepath.Base(path) != "RENAMED.md" {
		t.Fatalf("Selected() = (%q, ok=%v), want RENAMED.md selected", path, ok)
	}
}

func TestRenameWithNothingSelectedReturnsError(t *testing.T) {
	root := t.TempDir()
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Rename("x"); err != ErrNothingSelected {
		t.Fatalf("Rename() with an empty tree = %v, want ErrNothingSelected", err)
	}
}

func TestDeleteSelectedFile(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.MoveDown() // onto README.md
	if err := e.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("README.md should no longer exist after Delete")
	}
	if containsName(flatNames(e), "README.md") {
		t.Fatal("README.md should no longer be in the tree after Delete")
	}
}

func TestDeleteSelectedDirectoryRemovesItRecursively(t *testing.T) {
	root := mkTestTree(t)
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	// cursor starts on "src", which contains main.go
	if err := e.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "src")); !os.IsNotExist(err) {
		t.Fatal("src/ should no longer exist after Delete")
	}
}

func TestDeleteWithNothingSelectedReturnsError(t *testing.T) {
	root := t.TempDir()
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Delete(); err != ErrNothingSelected {
		t.Fatalf("Delete() with an empty tree = %v, want ErrNothingSelected", err)
	}
}
