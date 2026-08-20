package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// withSessionConfigDir redirects the session package's config directory
// resolution (os.UserConfigDir) to a temp directory, portably across
// platforms.
func withSessionConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestSaveSessionThenReopenRestoresTabsAndCursor(t *testing.T) {
	withSessionConfigDir(t)
	root := t.TempDir()
	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("second file"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(root, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.Update(windowSizeMsg())
	m.openFile(fileA)
	m.buf.MoveCursorTo(3)
	m.openFile(fileB)
	m.buf.MoveCursorTo(5) // fileB is now active

	m.saveSession()

	reopened, err := New(root, "")
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	if len(reopened.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(reopened.tabs))
	}
	if reopened.filename != fileB {
		t.Fatalf("active tab filename = %q, want %q (the tab active when saved)", reopened.filename, fileB)
	}
	if got := reopened.buf.CursorOffset(); got != 5 {
		t.Fatalf("active tab cursor offset = %d, want 5", got)
	}

	// Switch to the other restored tab and check its cursor came back too.
	reopened.switchToTab(reopened.findTab(fileA))
	if got := reopened.buf.CursorOffset(); got != 3 {
		t.Fatalf("fileA cursor offset = %d, want 3", got)
	}
}

func TestNewWithExplicitFilePathSkipsSessionRestore(t *testing.T) {
	withSessionConfigDir(t)
	root := t.TempDir()
	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(root, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.Update(windowSizeMsg())
	m.openFile(fileA)
	m.openFile(fileB)
	m.saveSession()

	// Explicitly requesting fileA should open just fileA, not also pull
	// in the two-tab session saved above.
	reopened, err := New(root, fileA)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(reopened.tabs) != 1 {
		t.Fatalf("len(tabs) = %d, want 1 (explicit file request should skip session restore)", len(reopened.tabs))
	}
	if reopened.filename != fileA {
		t.Fatalf("filename = %q, want %q", reopened.filename, fileA)
	}
}

func TestRestoreSessionSkipsFilesThatNoLongerExist(t *testing.T) {
	withSessionConfigDir(t)
	root := t.TempDir()
	fileA := filepath.Join(root, "a.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(root, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.Update(windowSizeMsg())
	m.openFile(fileA)
	m.saveSession()

	if err := os.Remove(fileA); err != nil {
		t.Fatal(err)
	}

	reopened, err := New(root, "")
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	if len(reopened.tabs) != 1 || reopened.filename != "" {
		t.Fatalf("tabs = %+v, want a single blank placeholder tab since the saved file is gone", reopened.tabs)
	}
}

func TestNewWithNoSavedSessionStartsBlank(t *testing.T) {
	withSessionConfigDir(t)
	root := t.TempDir()

	m, err := New(root, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(m.tabs) != 1 || m.filename != "" {
		t.Fatalf("tabs = %+v, want a single blank tab with no prior session", m.tabs)
	}
}

func windowSizeMsg() tea.WindowSizeMsg { return tea.WindowSizeMsg{Width: 80, Height: 24} }
