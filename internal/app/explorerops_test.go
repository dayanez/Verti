package app

import (
	"os"
	"os/exec"
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

// TestSaveDispatchesGitStatusReloadAsynchronously guards against
// regressing back to a synchronous `git status` call in the save
// keybinding: that shells out to git and blocks, which on a large or
// busy repo would stall Update -- and therefore every keystroke and
// redraw -- until the subprocess returns. Save should instead return a
// command bubbletea runs on its own goroutine.
func TestSaveDispatchesGitStatusReloadAsynchronously(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git unavailable in this environment: %v: %s", err, out)
	}

	_, cmd := m.Update(key(tea.KeyCtrlS))
	if cmd == nil {
		t.Fatal("save returned a nil command; want one that reloads git status in the background")
	}
	msg, ok := cmd().(gitStatusMsg)
	if !ok {
		t.Fatalf("cmd() produced %T, want gitStatusMsg", msg)
	}
	if msg.status == nil {
		t.Fatal("gitStatusMsg.status is nil")
	}

	m.Update(msg)
	if got := m.exp.Branch(); got == "" {
		t.Fatal("exp.Branch() is empty after applying the async git status reload, want the repo's branch name")
	}
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

func TestExplorerKeyRRenamesSelectedEntryRetargetsOpenTab(t *testing.T) {
	m := newTestModel(t, "hello") // opens scratch.txt as the active tab
	root := m.exp.Root.Path
	sendKeys(m, key(tea.KeyEnd), runes(" world")) // dirty the buffer so a later Ctrl+S would write it back out
	focusExplorer(m)

	sendKeys(m, runes("r"))
	for range m.promptText {
		sendKeys(m, key(tea.KeyBackspace))
	}
	sendKeys(m, runes("renamed.txt"), key(tea.KeyEnter))

	want := filepath.Join(root, "renamed.txt")
	if m.filename != want {
		t.Fatalf("active tab's filename = %q after rename, want %q: Ctrl+S must write to the new path, not recreate the old file", m.filename, want)
	}
	if len(m.tabs) != 1 || m.tabs[0].filename != want {
		t.Fatalf("tab filename = %q, want %q", m.tabs[0].filename, want)
	}
}

func TestExplorerKeyDDeletesOnConfirmUntitlesOpenTab(t *testing.T) {
	m := newTestModel(t, "hello") // opens scratch.txt as the active tab
	sendKeys(m, key(tea.KeyEnd), runes(" world")) // dirty the buffer, so its unsaved content matters
	focusExplorer(m)

	sendKeys(m, runes("d"), runes("y"))

	if m.filename != "" {
		t.Fatalf("active tab's filename = %q after deleting the file, want \"\": Ctrl+S must not silently recreate the deleted file", m.filename)
	}
	if got := m.buf.String(); got != "hello world" {
		t.Fatalf("buffer content = %q, want the unsaved edit preserved: %q", got, "hello world")
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
