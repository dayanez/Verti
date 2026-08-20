package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabCompletesWordFromEarlierOccurrenceInBuffer(t *testing.T) {
	m := newTestModel(t, "workspace\nwor")
	m.buf.MoveCursorTo(m.buf.Len()) // end of "wor"

	sendKeys(m, key(tea.KeyTab))

	want := "workspace\nworkspace"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestTabCyclesThroughCandidatesOnRepeatedPress(t *testing.T) {
	m := newTestModel(t, "worker work wor")
	m.buf.MoveCursorTo(m.buf.Len()) // end of "wor"

	sendKeys(m, key(tea.KeyTab))
	first := m.buf.String()
	if first != "worker work worker" {
		t.Fatalf("buffer after 1st Tab = %q, want %q", first, "worker work worker")
	}

	sendKeys(m, key(tea.KeyTab))
	second := m.buf.String()
	if second != "worker work work" {
		t.Fatalf("buffer after 2nd Tab = %q, want %q", second, "worker work work")
	}

	// A third Tab should cycle back to the first candidate.
	sendKeys(m, key(tea.KeyTab))
	third := m.buf.String()
	if third != "worker work worker" {
		t.Fatalf("buffer after 3rd Tab = %q, want it to cycle back to %q", third, "worker work worker")
	}
}

func TestTabWithNoCompletionsInsertsPlainTab(t *testing.T) {
	m := newTestModel(t, "unique")
	m.buf.MoveCursorTo(m.buf.Len())

	sendKeys(m, key(tea.KeyTab))

	want := "unique    " // no other occurrence of "unique" anywhere: falls back to inserting spaces
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestTabNotAfterIdentifierInsertsPlainTab(t *testing.T) {
	m := newTestModel(t, "word\n")
	m.buf.MoveCursorTo(m.buf.Len()) // start of a blank line, nothing to complete

	sendKeys(m, key(tea.KeyTab))

	want := "word\n    "
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q", got, want)
	}
}

func TestTabAfterUnrelatedEditDoesNotCycleStaleCompletion(t *testing.T) {
	m := newTestModel(t, "worker work wor")
	m.buf.MoveCursorTo(m.buf.Len())

	sendKeys(m, key(tea.KeyTab)) // completes "wor" -> "worker"
	if got := m.buf.String(); got != "worker work worker" {
		t.Fatalf("buffer after 1st Tab = %q, want %q", got, "worker work worker")
	}

	sendKeys(m, key(tea.KeyLeft)) // any other key ends the completion cycle
	sendKeys(m, key(tea.KeyRight))

	sendKeys(m, key(tea.KeyTab)) // starts a *new* completion for "worker", not a cycle
	if got := m.buf.String(); got != "worker work worker    " {
		t.Fatalf("buffer after unrelated-key-then-Tab = %q, want a plain tab insert (no other completions for %q)", got, "worker")
	}
}

func TestTabStillIndentsMultiLineSelection(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree")
	sendKeys(m, key(tea.KeyShiftDown), key(tea.KeyShiftDown))

	sendKeys(m, key(tea.KeyTab))

	want := "    one\n    two\nthree"
	if got := m.buf.String(); got != want {
		t.Fatalf("buffer = %q, want %q (multi-line selection should still indent, not complete)", got, want)
	}
}
