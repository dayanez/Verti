package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEnterCarriesForwardCurrentLineIndent(t *testing.T) {
	m := newTestModel(t, "    x = 1")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyEnter), runes("y"))

	want := "    x = 1\n    y"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEnterAddsExtraIndentAfterColon(t *testing.T) {
	m := newTestModel(t, "    if true:")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyEnter), runes("pass"))

	want := "    if true:\n        pass" // 4 (carried) + 4 (extra level)
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEnterAddsExtraIndentAfterOpenBrace(t *testing.T) {
	m := newTestModel(t, "func f() {")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyEnter), runes("x"))

	want := "func f() {\n    x"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestEnterSplitsFreshBracePairOntoThreeLines(t *testing.T) {
	m := newTestModel(t, "func f() {}")
	sendKeys(m, key(tea.KeyEnd), key(tea.KeyLeft), key(tea.KeyEnter))

	want := "func f() {\n    \n}"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if _, col := m.buf.CursorLineCol(); col != 4 {
		t.Fatalf("cursor col after split = %d, want 4 (end of the indented middle line)", col)
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

func TestOpenFileWithUnsavedChangesOpensANewTabInstead(t *testing.T) {
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
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v, want promptNone: opening a file should never need confirmation, it opens a new tab", m.prompt)
	}
	if got := m.buf.String(); got != "other file content" {
		t.Fatalf("buffer = %q, want the new file's content %q", got, "other file content")
	}
	if len(m.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(m.tabs))
	}

	// The original dirty tab must still exist, untouched, in the background.
	m.prevTab()
	if got := m.buf.String(); got != "Xhello" {
		t.Fatalf("original tab's buffer = %q after switching back, want unchanged %q", got, "Xhello")
	}
}

func TestOpeningAnAlreadyOpenFileSwitchesToItsTabInsteadOfDuplicating(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := newTestModel(t, "hello")
	m.openFile(other)
	m.prevTab()
	m.openFile(other) // already open -> should switch, not duplicate

	if len(m.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2 (no duplicate tab)", len(m.tabs))
	}
	if got := m.buf.String(); got != "content" {
		t.Fatalf("buffer = %q, want %q", got, "content")
	}
}

func TestCloseTabWithUnsavedChangesAsksToConfirm(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, runes("X"))
	if !m.buf.Dirty() {
		t.Fatal("buffer should be dirty after an edit")
	}

	sendKeys(m, key(tea.KeyCtrlW))
	if m.prompt != promptConfirmDiscard {
		t.Fatalf("prompt = %v, want promptConfirmDiscard", m.prompt)
	}
	if len(m.tabs) != 1 {
		t.Fatalf("len(tabs) = %d, want 1 (tab not closed before confirmation)", len(m.tabs))
	}

	// 'n' cancels: the dirty tab survives.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after 'n', want promptNone", m.prompt)
	}
	if m.buf.String() != "Xhello" {
		t.Fatalf("buffer = %q after canceling close, want unchanged %q", m.buf.String(), "Xhello")
	}

	// Retry and confirm with 'y': closing the last tab leaves a fresh blank one.
	sendKeys(m, key(tea.KeyCtrlW))
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after 'y', want promptNone", m.prompt)
	}
	if got := m.buf.String(); got != "" {
		t.Fatalf("buffer = %q after confirming close of the last tab, want a fresh blank buffer", got)
	}
}

func TestCloseTabWithoutUnsavedChangesClosesImmediately(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := newTestModel(t, "hello")
	m.openFile(other)
	if len(m.tabs) != 2 {
		t.Fatalf("len(tabs) = %d, want 2", len(m.tabs))
	}

	sendKeys(m, key(tea.KeyCtrlW))
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v, want promptNone (no unsaved changes, no confirmation needed)", m.prompt)
	}
	if len(m.tabs) != 1 {
		t.Fatalf("len(tabs) = %d, want 1", len(m.tabs))
	}
	if got := m.buf.String(); got != "hello" {
		t.Fatalf("buffer = %q after closing other.txt's tab, want back on the original %q", got, "hello")
	}
}

func TestTabSwitchingIsolatesUndoHistory(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, runes("A")) // tab 0 buffer: "Ahello"

	m.newTab()
	sendKeys(m, runes("B")) // tab 1 (blank) buffer: "B"
	if got := m.buf.String(); got != "B" {
		t.Fatalf("tab 1 buffer = %q, want %q", got, "B")
	}

	m.undo() // undo within tab 1 only
	if got := m.buf.String(); got != "" {
		t.Fatalf("tab 1 buffer after undo = %q, want empty", got)
	}

	m.prevTab()
	if got := m.buf.String(); got != "Ahello" {
		t.Fatalf("tab 0 buffer = %q, want %q (its own edit untouched by tab 1's undo)", got, "Ahello")
	}
}

func TestCtrlPgUpPgDownCycleTabs(t *testing.T) {
	m := newTestModel(t, "hello")
	m.newTab()
	sendKeys(m, runes("second"))
	m.newTab()
	sendKeys(m, runes("third"))

	if got := m.buf.String(); got != "third" {
		t.Fatalf("buffer = %q, want %q", got, "third")
	}
	sendKeys(m, key(tea.KeyCtrlPgUp))
	if got := m.buf.String(); got != "second" {
		t.Fatalf("buffer after Ctrl+PgUp = %q, want %q", got, "second")
	}
	sendKeys(m, key(tea.KeyCtrlPgUp))
	if got := m.buf.String(); got != "hello" {
		t.Fatalf("buffer after 2nd Ctrl+PgUp = %q, want %q", got, "hello")
	}
	sendKeys(m, key(tea.KeyCtrlPgDown))
	if got := m.buf.String(); got != "second" {
		t.Fatalf("buffer after Ctrl+PgDown = %q, want %q", got, "second")
	}
}

func TestTabBarHiddenWithOneTabShownWithMultiple(t *testing.T) {
	m := newTestModel(t, "hello")
	if got := m.renderTabBar(); got != "" {
		t.Fatalf("renderTabBar() with 1 tab = %q, want empty (unchanged single-tab layout)", got)
	}

	m.newTab()
	bar := m.renderTabBar()
	if bar == "" {
		t.Fatal("renderTabBar() with 2 tabs is empty, want a rendered bar")
	}
	if !strings.Contains(bar, "scratch.txt") || !strings.Contains(bar, "[untitled]") {
		t.Fatalf("renderTabBar() = %q, want it to mention both scratch.txt and [untitled]", bar)
	}
}

func TestFindInFilesShowsResultsAndJumpsToMatch(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if err := os.WriteFile(filepath.Join(root, "other.go"), []byte("package other\n\nfunc needle() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sendKeys(m, key(tea.KeyCtrlT), runes("needle"))
	sendKeyAndRunCmd(m, key(tea.KeyEnter))
	if m.focus != FocusSearchResults {
		t.Fatalf("focus = %v, want FocusSearchResults", m.focus)
	}
	if len(m.searchResults) != 1 {
		t.Fatalf("len(searchResults) = %d, want 1: %+v", len(m.searchResults), m.searchResults)
	}
	if m.searchResults[0].Line != 3 {
		t.Fatalf("match line = %d, want 3", m.searchResults[0].Line)
	}

	sendKeys(m, key(tea.KeyEnter))
	if m.focus != FocusEditor {
		t.Fatalf("focus = %v after jumping to a result, want FocusEditor", m.focus)
	}
	if !strings.HasSuffix(m.filename, "other.go") {
		t.Fatalf("filename = %q, want it to end with other.go", m.filename)
	}
	line, _ := m.buf.CursorLineCol()
	if line != 2 { // 0-based; the match was on 1-based line 3
		t.Fatalf("cursor line = %d, want 2", line)
	}
	if m.searchResultsActive {
		t.Fatal("searchResultsActive should be false after jumping to a result")
	}
}

func TestFindInFilesEscReturnsToExplorer(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if err := os.WriteFile(filepath.Join(root, "other.go"), []byte("needle\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sendKeys(m, key(tea.KeyCtrlT), runes("needle"))
	sendKeyAndRunCmd(m, key(tea.KeyEnter))
	sendKeys(m, key(tea.KeyEsc))
	if m.focus != FocusExplorer {
		t.Fatalf("focus = %v after Esc, want FocusExplorer", m.focus)
	}
	if m.searchResultsActive {
		t.Fatal("searchResultsActive should be false after Esc")
	}
}

func TestFindInFilesUpDownMovesSelectionAndClamps(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("needle a\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("needle b\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sendKeys(m, key(tea.KeyCtrlT), runes("needle"))
	sendKeyAndRunCmd(m, key(tea.KeyEnter))
	if len(m.searchResults) != 2 {
		t.Fatalf("len(searchResults) = %d, want 2", len(m.searchResults))
	}
	if m.searchSelected != 0 {
		t.Fatalf("searchSelected = %d, want 0", m.searchSelected)
	}

	sendKeys(m, key(tea.KeyDown))
	if m.searchSelected != 1 {
		t.Fatalf("searchSelected after Down = %d, want 1", m.searchSelected)
	}
	sendKeys(m, key(tea.KeyDown)) // no more results below; should clamp
	if m.searchSelected != 1 {
		t.Fatalf("searchSelected after 2nd Down = %d, want clamped at 1", m.searchSelected)
	}
	sendKeys(m, key(tea.KeyUp))
	if m.searchSelected != 0 {
		t.Fatalf("searchSelected after Up = %d, want 0", m.searchSelected)
	}
}

func TestFindInFilesPromptWiredToCtrlT(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyCtrlT))
	if m.prompt != promptFindInFiles {
		t.Fatalf("prompt = %v, want promptFindInFiles", m.prompt)
	}
}

func TestNewTabAddsBlankTabAndLayoutReservesTabBarRow(t *testing.T) {
	m := newTestModel(t, "hello")
	heightBefore := m.editorView.Height

	m.newTab()
	if got := m.buf.String(); got != "" {
		t.Fatalf("new tab's buffer = %q, want empty", got)
	}
	if m.editorView.Height != heightBefore-1 {
		t.Fatalf("editorView.Height = %d, want %d (one row reserved for the tab bar)", m.editorView.Height, heightBefore-1)
	}
}
