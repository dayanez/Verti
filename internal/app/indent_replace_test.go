package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabIndentsMultiLineSelectionInsteadOfReplacing(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	// (0,0) -> (2,0): fully highlights lines 0-1 ("one","two"); the
	// selection merely touching column 0 of line 2 doesn't count it as
	// selected, matching how most editors treat a drag that stops at the
	// very start of a line.
	sendKeys(m, key(tea.KeyShiftDown), key(tea.KeyShiftDown))

	sendKeys(m, key(tea.KeyTab))

	want := "    one\n    two\nthree"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if !m.buf.HasSelection() {
		t.Fatal("selection was dropped; Tab on a multi-line selection should re-select the indented block")
	}
}

func TestTabOnSingleLineSelectionStillReplacesIt(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight)) // selects "hello", within one line
	}
	sendKeys(m, key(tea.KeyTab))

	// "hello" is deleted and replaced by 4 spaces (the default tab width),
	// leaving the original space before "world" too: 5 spaces total.
	if got := m.buf.String(); got != "     world" {
		t.Fatalf("buffer = %q, want the single-line selection replaced by spaces", got)
	}
}

func TestShiftTabOutdentsMultiLineSelection(t *testing.T) {
	m := newTestModel(t, "    one\n    two\nthree")
	sendKeys(m, key(tea.KeyShiftDown), key(tea.KeyShiftDown))
	sendKeys(m, key(tea.KeyShiftTab))

	want := "one\ntwo\nthree"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestShiftTabOutdentsCurrentLineWithoutSelection(t *testing.T) {
	m := newTestModel(t, "    indented")
	sendKeys(m, key(tea.KeyShiftTab))

	if got := m.buf.String(); got != "indented" {
		t.Fatalf("buffer = %q, want %q", got, "indented")
	}
}

func TestReplaceAllReplacesEveryOccurrenceAsOneUndoStep(t *testing.T) {
	m := newTestModel(t, "cat dog cat bird cat")
	sendKeys(m, key(tea.KeyCtrlR))
	if m.prompt != promptReplaceFind {
		t.Fatalf("prompt = %v, want promptReplaceFind", m.prompt)
	}
	sendKeys(m, runes("cat"), key(tea.KeyEnter))
	if m.prompt != promptReplaceWith {
		t.Fatalf("prompt = %v after find term, want promptReplaceWith", m.prompt)
	}
	sendKeys(m, runes("fish"), key(tea.KeyEnter))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after replace Enter, want promptNone", m.prompt)
	}
	want := "fish dog fish bird fish"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if !m.buf.Dirty() {
		t.Fatal("buffer should be dirty after Replace-All, so closing the tab prompts to save instead of silently discarding it")
	}

	m.undo()
	if got := m.buf.String(); got != "cat dog cat bird cat" {
		t.Fatalf("after one undo, buffer = %q, want the original text restored in a single step", got)
	}
}

func TestReplaceAllReportsNotFound(t *testing.T) {
	m := newTestModel(t, "no match here")
	sendKeys(m, key(tea.KeyCtrlR))
	sendKeys(m, runes("xyz"), key(tea.KeyEnter))
	sendKeys(m, runes("abc"), key(tea.KeyEnter))

	if got := m.buf.String(); got != "no match here" {
		t.Fatalf("buffer changed despite no match: %q", got)
	}
}

func TestSaveAsWithinWorkspaceRefreshesExplorerTree(t *testing.T) {
	m := newTestModel(t, "content to save")
	newPath := filepath.Join(m.exp.Root.Path, "brandnew.txt")

	sendKeys(m, key(tea.KeyCtrlO))
	for range m.promptText {
		sendKeys(m, key(tea.KeyBackspace))
	}
	sendKeys(m, runes(newPath), key(tea.KeyEnter))

	if !strings.Contains(m.exp.Render(false), "brandnew.txt") {
		t.Fatal("explorer tree does not show brandnew.txt after Save As; the sidebar should refresh to reflect the newly created file")
	}
}

func TestSaveAsWritesNewFileAndUpdatesFilename(t *testing.T) {
	m := newTestModel(t, "content to save")
	newPath := filepath.Join(t.TempDir(), "renamed.txt")

	sendKeys(m, key(tea.KeyCtrlO))
	if m.prompt != promptSaveAs {
		t.Fatalf("prompt = %v, want promptSaveAs", m.prompt)
	}
	// Clear the pre-filled current filename before typing the new path.
	for range m.promptText {
		sendKeys(m, key(tea.KeyBackspace))
	}
	sendKeys(m, runes(newPath), key(tea.KeyEnter))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after save-as Enter, want promptNone", m.prompt)
	}
	if m.filename != newPath {
		t.Fatalf("filename = %q, want %q", m.filename, newPath)
	}
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", newPath, err)
	}
	if string(data) != "content to save" {
		t.Fatalf("saved file content = %q, want %q", string(data), "content to save")
	}
}

func TestToggleCommentOnCurrentLineNoSelection(t *testing.T) {
	m := newTestModel(t, "print('hi')") // scratch.txt has no extension mapping -> falls back to "//"
	sendKeys(m, key(tea.KeyCtrlUnderscore))
	if got := m.buf.String(); got != "// print('hi')" {
		t.Fatalf("buffer = %q, want %q", got, "// print('hi')")
	}

	sendKeys(m, key(tea.KeyCtrlUnderscore)) // toggling again uncomments
	if got := m.buf.String(); got != "print('hi')" {
		t.Fatalf("buffer = %q, want %q", got, "print('hi')")
	}
}

func TestToggleCommentPreservesIndentation(t *testing.T) {
	m := newTestModel(t, "    indented")
	sendKeys(m, key(tea.KeyCtrlUnderscore))
	if got := m.buf.String(); got != "    // indented" {
		t.Fatalf("buffer = %q, want the comment marker after the indent, got %q", got, "    // indented")
	}
}

func TestToggleCommentOnMultiLineSelectionCommentsEveryLine(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	sendKeys(m, key(tea.KeyShiftDown), key(tea.KeyShiftDown)) // touches lines 0-1, see indent tests

	sendKeys(m, key(tea.KeyCtrlUnderscore))
	want := "// one\n// two\nthree"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
	if !m.buf.HasSelection() {
		t.Fatal("selection was dropped after commenting a multi-line selection")
	}

	sendKeys(m, key(tea.KeyCtrlUnderscore)) // toggle back off
	if got := m.buf.String(); got != "one\ntwo\nthree" {
		t.Fatalf("buffer = %q, want the original text restored", got)
	}
}

func TestToggleCommentSkipsBlankLinesInRange(t *testing.T) {
	m := newTestModel(t, "one\n\ntwo")
	// Reach into "two" (not just its column 0) so line 2 is actually
	// counted as touched by the selection -- see the indent tests for why
	// merely touching column 0 of the next line doesn't count it.
	sendKeys(m, key(tea.KeyShiftDown), key(tea.KeyShiftDown), key(tea.KeyShiftRight))

	sendKeys(m, key(tea.KeyCtrlUnderscore))
	want := "// one\n\n// two"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q (blank line untouched)", got, want)
	}
}

func TestToggleCommentUsesBlockCommentForCSS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "style.css")
	if err := os.WriteFile(path, []byte("color: red;"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := New(dir, path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.osClipboard = &fakeClipboard{}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	sendKeys(m, key(tea.KeyCtrlUnderscore))
	want := "/* color: red; */"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}

	sendKeys(m, key(tea.KeyCtrlUnderscore))
	if got := m.buf.String(); got != "color: red;" {
		t.Fatalf("buffer = %q, want the block comment removed", got)
	}
}

func TestSaveWithNoFilenameOpensSaveAsPrompt(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.osClipboard = &fakeClipboard{}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	sendKeys(m, runes("unsaved text"))

	sendKeys(m, key(tea.KeyCtrlS))
	if m.prompt != promptSaveAs {
		t.Fatalf("prompt = %v after Ctrl+S with no filename, want promptSaveAs", m.prompt)
	}
}
