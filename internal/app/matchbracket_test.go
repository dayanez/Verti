package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func jumpBracket(m *Model) { sendKeys(m, key(tea.KeyCtrlCloseBracket)) }

func TestJumpFromOpenBracketToClose(t *testing.T) {
	m := newTestModel(t, "func f() { return }")
	m.buf.MoveCursorTo(9) // right before '{'

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 18 { // the matching '}'
		t.Fatalf("CursorOffset() = %d, want 18 (the matching '}')", got)
	}
}

func TestJumpFromCloseBracketToOpen(t *testing.T) {
	m := newTestModel(t, "func f() { return }")
	m.buf.MoveCursorTo(19) // right after '}', the end of the buffer

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 9 { // the matching '{'
		t.Fatalf("CursorOffset() = %d, want 9 (the matching '{')", got)
	}
}

func TestJumpSkipsNestedBracketsOfSameKind(t *testing.T) {
	m := newTestModel(t, "f(a, g(b, c), d)")
	m.buf.MoveCursorTo(1) // right before the outer '('

	jumpBracket(m)

	want := len("f(a, g(b, c), d)") - 1 // the outer, matching ')'
	if got := m.buf.CursorOffset(); got != want {
		t.Fatalf("CursorOffset() = %d, want %d (the outer closing paren, not the inner one)", got, want)
	}
}

func TestJumpFromInnerBracketFindsItsOwnMatch(t *testing.T) {
	m := newTestModel(t, "f(a, g(b, c), d)")
	m.buf.MoveCursorTo(6) // right before the inner '('

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 11 { // the inner matching ')'
		t.Fatalf("CursorOffset() = %d, want 11 (the inner closing paren)", got)
	}
}

func TestJumpWithNoBracketAtCursorIsNoOp(t *testing.T) {
	m := newTestModel(t, "hello world")
	m.buf.MoveCursorTo(3)

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 3 {
		t.Fatalf("CursorOffset() = %d, want unchanged 3", got)
	}
}

func TestJumpWithUnmatchedBracketIsNoOp(t *testing.T) {
	m := newTestModel(t, "func f( {")
	m.buf.MoveCursorTo(7) // right before the unmatched '('

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 7 {
		t.Fatalf("CursorOffset() = %d, want unchanged 7 (no match exists)", got)
	}
}

func TestJumpHandlesMultiByteUTF8BeforeTheBracket(t *testing.T) {
	// "日本語" is 3 runes but 9 bytes: a rune/byte offset mix-up in the
	// bulk-fetch scan would land on the wrong character here.
	m := newTestModel(t, "日本語(x)")
	m.buf.MoveCursorTo(3) // right before '(', which is rune offset 3

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 5 { // the matching ')'
		t.Fatalf("CursorOffset() = %d, want 5 (the matching ')')", got)
	}
}

func TestJumpWorksForSquareAndCurlyBrackets(t *testing.T) {
	m := newTestModel(t, "arr[0]")
	m.buf.MoveCursorTo(3) // right before '['

	jumpBracket(m)

	if got := m.buf.CursorOffset(); got != 5 {
		t.Fatalf("CursorOffset() = %d, want 5 (the matching ']')", got)
	}
}
