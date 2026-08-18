package explorer

import (
	"os"
	"path/filepath"
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
