package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTypingOpenParenInsertsPairWithCursorBetween(t *testing.T) {
	m := newTestModel(t, "")
	sendKeys(m, runes("("))

	if got := m.buf.String(); got != "()" {
		t.Fatalf("buffer = %q, want %q", got, "()")
	}
	if got := m.buf.CursorOffset(); got != 1 {
		t.Fatalf("CursorOffset() = %d, want 1 (between the delimiters)", got)
	}
}

func TestTypingCloserRightBeforeItsMatchTypesThrough(t *testing.T) {
	m := newTestModel(t, "")
	sendKeys(m, runes("(")) // buffer: "(|)"
	sendKeys(m, runes(")")) // should move past the auto-inserted ')', not insert another

	if got := m.buf.String(); got != "()" {
		t.Fatalf("buffer = %q, want %q (no duplicate closer)", got, "()")
	}
	if got := m.buf.CursorOffset(); got != 2 {
		t.Fatalf("CursorOffset() = %d, want 2 (past the closer)", got)
	}
}

func TestTypingQuoteRightBeforeMatchingQuoteTypesThrough(t *testing.T) {
	m := newTestModel(t, "")
	sendKeys(m, runes("\"")) // buffer: "|""
	sendKeys(m, runes("hi"))
	sendKeys(m, runes("\"")) // should type through the auto-inserted closing quote

	if got := m.buf.String(); got != `"hi"` {
		t.Fatalf("buffer = %q, want %q", got, `"hi"`)
	}
	if got := m.buf.CursorOffset(); got != 4 {
		t.Fatalf("CursorOffset() = %d, want 4 (past the closing quote)", got)
	}
}

func TestTypingOpenBracketWithSelectionWrapsIt(t *testing.T) {
	m := newTestModel(t, "hello world")
	for i := 0; i < 5; i++ {
		sendKeys(m, key(tea.KeyShiftRight)) // select "hello"
	}
	sendKeys(m, runes("("))

	want := "(hello) world"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestBackspaceBetweenEmptyPairRemovesBoth(t *testing.T) {
	m := newTestModel(t, "")
	sendKeys(m, runes("[")) // buffer: "[|]"
	sendKeys(m, key(tea.KeyBackspace))

	if got := m.buf.String(); got != "" {
		t.Fatalf("buffer = %q, want empty (both delimiters removed)", got)
	}
}

func TestBackspaceWithContentBetweenPairOnlyDeletesOneChar(t *testing.T) {
	m := newTestModel(t, "(x)")
	sendKeys(m, key(tea.KeyRight), key(tea.KeyRight)) // cursor between 'x' and ')'
	sendKeys(m, key(tea.KeyBackspace))

	want := "()"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q (only 'x' removed, delimiters untouched)", got, want)
	}
}

func TestUnmatchedCloserStillInsertsNormally(t *testing.T) {
	m := newTestModel(t, "")
	sendKeys(m, runes(")"))

	if got := m.buf.String(); got != ")" {
		t.Fatalf("buffer = %q, want %q (nothing to type through)", got, ")")
	}
}
