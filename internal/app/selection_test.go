package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel opens a scratch file containing content inside a temp
// workspace, sized as a real terminal would be, and returns the model
// driven exactly the way the bubbletea runtime drives it: through
// Update(tea.Msg), never by poking package-private fields directly.
func newTestModel(t *testing.T, content string) *Model {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "scratch.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := New(dir, path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m
}

func sendKeys(m *Model, msgs ...tea.KeyMsg) {
	for _, msg := range msgs {
		m.Update(msg)
	}
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }
func runes(s string) tea.KeyMsg    { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestShiftArrowsSelectText(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	if got := m.buf.SelectedText(); got != "hello" {
		t.Fatalf("SelectedText() = %q, want %q", got, "hello")
	}

	// A plain (non-shift) arrow key collapses the selection.
	sendKeys(m, key(tea.KeyRight))
	if m.buf.HasSelection() {
		t.Fatal("HasSelection() = true after a non-shift arrow, want false")
	}
}

func TestCtrlCCopiesSelection(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	sendKeys(m, key(tea.KeyCtrlC))

	if m.clipboard != "hello" {
		t.Fatalf("clipboard = %q, want %q", m.clipboard, "hello")
	}
	if m.buf.String() != "hello world" {
		t.Fatalf("copy mutated the buffer: %q", m.buf.String())
	}
	if !strings.Contains(m.status, "copied") {
		t.Fatalf("status = %q, want it to mention the copy", m.status)
	}
}

func TestCtrlCWithNoSelectionCopiesCurrentLine(t *testing.T) {
	m := newTestModel(t, "line one\nline two")
	sendKeys(m, key(tea.KeyDown)) // move onto "line two"
	sendKeys(m, key(tea.KeyCtrlC))

	if m.clipboard != "line two" {
		t.Fatalf("clipboard = %q, want %q", m.clipboard, "line two")
	}
}

func TestCtrlXCutsSelectionAndCtrlVPastesIt(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	sendKeys(m, key(tea.KeyCtrlX))

	if m.buf.String() != " world" {
		t.Fatalf("after cut, buffer = %q, want %q", m.buf.String(), " world")
	}
	if m.buf.HasSelection() {
		t.Fatal("HasSelection() = true after cut, want false")
	}

	sendKeys(m, key(tea.KeyEnd), key(tea.KeyCtrlV))
	if m.buf.String() != " worldhello" {
		t.Fatalf("after paste, buffer = %q, want %q", m.buf.String(), " worldhello")
	}
}

func TestTypingOverSelectionReplacesIt(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	sendKeys(m, runes("X"))

	if m.buf.String() != "X world" {
		t.Fatalf("buffer = %q, want %q", m.buf.String(), "X world")
	}
}

func TestBackspaceOverSelectionDeletesOnlyTheSelection(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	sendKeys(m, key(tea.KeyBackspace))

	if m.buf.String() != " world" {
		t.Fatalf("buffer = %q, want %q", m.buf.String(), " world")
	}
}

func TestUndoAfterReplacingSelectionRestoresOriginalTextInOneStep(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	sendKeys(m, runes("X"))
	if m.buf.String() != "X world" {
		t.Fatalf("setup: buffer = %q, want %q", m.buf.String(), "X world")
	}

	m.undo()
	if got := m.buf.String(); got != "hello world" {
		t.Fatalf("after one undo, buffer = %q, want %q", got, "hello world")
	}
}

func TestViewRendersWithoutPanicWhileSelecting(t *testing.T) {
	m := newTestModel(t, "hello world\nsecond line")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight))
	}
	if out := m.View(); !strings.Contains(out, "world") {
		t.Fatalf("View() output missing visible text; got %q", out)
	}
}
