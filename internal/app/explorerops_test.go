package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// focusExplorer drives the model to FocusExplorer the same way a user
// would: toggling the sidebar off then back on (the second toggle is what
// actually moves focus there).
func focusExplorer(m *Model) {
	sendKeys(m, key(tea.KeyCtrlB), key(tea.KeyCtrlB))
}

func TestExplorerKeyANewFilePromptsAndCreates(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	focusExplorer(m)

	sendKeys(m, runes("a"))
	if m.prompt != promptNewFile {
		t.Fatalf("prompt = %v after 'a', want promptNewFile", m.prompt)
	}
	sendKeys(m, runes("fresh.go"), key(tea.KeyEnter))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after Enter, want promptNone", m.prompt)
	}
	if _, err := os.Stat(filepath.Join(root, "fresh.go")); err != nil {
		t.Fatalf("expected fresh.go to exist: %v", err)
	}
}

func TestExplorerKeyShiftANewFolderPromptsAndCreates(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	focusExplorer(m)

	sendKeys(m, runes("A"))
	if m.prompt != promptNewFolder {
		t.Fatalf("prompt = %v after 'A', want promptNewFolder", m.prompt)
	}
	sendKeys(m, runes("newdir"), key(tea.KeyEnter))

	info, err := os.Stat(filepath.Join(root, "newdir"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected newdir/ to exist as a directory: %v", err)
	}
}

func TestExplorerKeyRRenamesSelectedEntry(t *testing.T) {
	m := newTestModel(t, "hello") // creates scratch.txt as the workspace's only file
	root := m.exp.Root.Path
	focusExplorer(m)

	sendKeys(m, runes("r"))
	if m.prompt != promptRename {
		t.Fatalf("prompt = %v after 'r', want promptRename", m.prompt)
	}
	if m.promptText != "scratch.txt" {
		t.Fatalf("promptText = %q, want it pre-filled with the current name %q", m.promptText, "scratch.txt")
	}

	// Clear the pre-filled name and type a new one.
	for range m.promptText {
		sendKeys(m, key(tea.KeyBackspace))
	}
	sendKeys(m, runes("renamed.txt"), key(tea.KeyEnter))

	if _, err := os.Stat(filepath.Join(root, "renamed.txt")); err != nil {
		t.Fatalf("expected renamed.txt to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatal("scratch.txt should no longer exist after rename")
	}
}

func TestExplorerKeyDDeletesOnConfirm(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	focusExplorer(m)

	sendKeys(m, runes("d"))
	if m.prompt != promptConfirmDeleteFile {
		t.Fatalf("prompt = %v after 'd', want promptConfirmDeleteFile", m.prompt)
	}
	sendKeys(m, runes("y"))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after confirming, want promptNone", m.prompt)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch.txt")); !os.IsNotExist(err) {
		t.Fatal("scratch.txt should no longer exist after confirming delete")
	}
}

func TestExplorerKeyDCancelOnNLeavesFileAlone(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	focusExplorer(m)

	sendKeys(m, runes("d"), runes("n"))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after canceling, want promptNone", m.prompt)
	}
	if _, err := os.Stat(filepath.Join(root, "scratch.txt")); err != nil {
		t.Fatalf("scratch.txt should still exist after canceling delete: %v", err)
	}
}
