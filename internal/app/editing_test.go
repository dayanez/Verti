package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEnterCarriesForwardCurrentLineIndent(t *testing.T) {
	m := newTestModel(t, "    if true:")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyEnter), runes("pass"))

	want := "    if true:\n    pass"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEnterWithNoIndentInsertsPlainNewline(t *testing.T) {
	m := newTestModel(t, "flat line")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyEnter), runes("next"))

	want := "flat line\nnext"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEnterMidLineStillUsesLinesIndentNotCursorColumn(t *testing.T) {
	m := newTestModel(t, "  ab")
	sendKeys(m, key(tea.KeyRight), key(tea.KeyRight), key(tea.KeyRight)) // cursor between "a" and "b"
	sendKeys(m, key(tea.KeyEnter))

	want := "  a\n  b"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestFindSelectsMatchAndWrapsAround(t *testing.T) {
	m := newTestModel(t, "cat dog cat bird")
	sendKeys(m, key(tea.KeyCtrlF))
	if m.prompt != promptFind {
		t.Fatalf("prompt = %v, want promptFind", m.prompt)
	}
	sendKeys(m, runes("cat"), key(tea.KeyEnter))

	if got := m.buf.SelectedText(); got != "cat" {
		t.Fatalf("SelectedText() = %q, want %q", got, "cat")
	}
	if got := m.buf.CursorOffset(); got != 3 { // first "cat" is [0,3)
		t.Fatalf("CursorOffset() = %d, want 3 (end of first match)", got)
	}

	sendKeys(m, key(tea.KeyEnter)) // find next -> second "cat" at [8,11)
	if got := m.buf.CursorOffset(); got != 11 {
		t.Fatalf("CursorOffset() after 2nd find = %d, want 11", got)
	}

	sendKeys(m, key(tea.KeyEnter)) // no more "cat" ahead -> wraps to the first match
	if got := m.buf.CursorOffset(); got != 3 {
		t.Fatalf("CursorOffset() after wrap = %d, want 3", got)
	}

	sendKeys(m, key(tea.KeyEsc))
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after Esc, want promptNone", m.prompt)
	}
}

func TestGotoLineMovesCursorAndClosesPrompt(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	sendKeys(m, key(tea.KeyCtrlG))
	sendKeys(m, runes("3"), key(tea.KeyEnter))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after goto Enter, want promptNone (single-shot)", m.prompt)
	}
	line, col := m.buf.CursorLineCol()
	if line != 2 || col != 0 {
		t.Fatalf("CursorLineCol() = (%d,%d), want (2,0)", line, col)
	}
}

func TestGotoLineClampsOutOfRangeInput(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	sendKeys(m, key(tea.KeyCtrlG))
	sendKeys(m, runes("999"), key(tea.KeyEnter))

	line, _ := m.buf.CursorLineCol()
	if line != 2 {
		t.Fatalf("CursorLineCol() line = %d, want 2 (clamped to last line)", line)
	}
}

func TestDuplicateLine(t *testing.T) {
	m := newTestModel(t, "hello\nworld")
	sendKeys(m, key(tea.KeyCtrlD))
	if got := m.buf.String(); got != "hello\nhello\nworld" {
		t.Fatalf("buffer = %q, want %q", got, "hello\nhello\nworld")
	}
	line, _ := m.buf.CursorLineCol()
	if line != 1 {
		t.Fatalf("cursor line after duplicate = %d, want 1 (on the new copy)", line)
	}
}

func TestDuplicateLastLineWithNoTrailingNewline(t *testing.T) {
	m := newTestModel(t, "onlyline")
	sendKeys(m, key(tea.KeyCtrlD))
	if got := m.buf.String(); got != "onlyline\nonlyline" {
		t.Fatalf("buffer = %q, want %q", got, "onlyline\nonlyline")
	}
}

func TestDeleteLineDoesNotTouchClipboard(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	m.clipboard = "untouched"
	sendKeys(m, key(tea.KeyCtrlK))

	if got := m.buf.String(); got != "two\nthree" {
		t.Fatalf("buffer = %q, want %q", got, "two\nthree")
	}
	if m.clipboard != "untouched" {
		t.Fatalf("clipboard = %q, want it left untouched", m.clipboard)
	}
}

func TestCtrlLeftRightJumpWords(t *testing.T) {
	m := newTestModel(t, "the quick brown")
	sendKeys(m, key(tea.KeyCtrlRight))
	if got := m.buf.CursorOffset(); got != 4 {
		t.Fatalf("CursorOffset() = %d, want 4 (start of \"quick\")", got)
	}
	sendKeys(m, key(tea.KeyCtrlLeft))
	if got := m.buf.CursorOffset(); got != 0 {
		t.Fatalf("CursorOffset() = %d, want 0", got)
	}
}

func TestOpenFileWithUnsavedChangesAsksToConfirm(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("other file content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := newTestModel(t, "hello")
	sendKeys(m, runes("X")) // dirty the buffer
	if !m.buf.Dirty() {
		t.Fatal("buffer should be dirty after an edit")
	}

	m.openFile(other)
	if m.prompt != promptConfirmDiscard {
		t.Fatalf("prompt = %v, want promptConfirmDiscard", m.prompt)
	}
	if m.buf.String() != "Xhello" {
		t.Fatalf("buffer changed before confirmation: %q", m.buf.String())
	}

	// 'n' cancels: the dirty buffer must survive untouched.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after 'n', want promptNone", m.prompt)
	}
	if m.buf.String() != "Xhello" {
		t.Fatalf("buffer = %q after canceling discard, want unchanged %q", m.buf.String(), "Xhello")
	}

	// Retry and confirm with 'y': the new file should load.
	m.openFile(other)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after 'y', want promptNone", m.prompt)
	}
	if got := m.buf.String(); got != "other file content" {
		t.Fatalf("buffer = %q after confirming discard, want %q", got, "other file content")
	}
	if !strings.HasSuffix(m.filename, "other.txt") {
		t.Fatalf("filename = %q, want it to end with other.txt", m.filename)
	}
}
