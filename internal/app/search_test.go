package app

import (
	"testing"

	"github.com/dayanez/Verti/pkg/search"
)

func TestJumpToSearchResultOpensFileAndMovesCursor(t *testing.T) {
	m := newTestModel(t, "one\ntwo\nthree\n")
	m.searchResults = []search.Match{{Path: "scratch.txt", Line: 2, Text: "two"}}
	m.searchResultsActive = true

	m.jumpToSearchResult(0)

	if got := m.buf.LineOffset(1); got != m.buf.CursorOffset() {
		t.Fatalf("cursor offset = %d, want start of line 2 (%d)", m.buf.CursorOffset(), got)
	}
	if m.searchResultsActive {
		t.Error("searchResultsActive = true after a successful jump, want false")
	}
}

func TestJumpToSearchResultLeavesCursorAloneWhenFileFailsToOpen(t *testing.T) {
	m := newTestModel(t, "hello")
	m.buf.MoveCursorTo(3) // somewhere mid-buffer, so a wrongful reset is observable
	before := m.buf.CursorOffset()

	m.searchResults = []search.Match{{Path: "does-not-exist.txt", Line: 1, Text: "x"}}
	m.searchResultsActive = true

	m.jumpToSearchResult(0)

	if got := m.buf.CursorOffset(); got != before {
		t.Fatalf("cursor offset = %d after a failed jump, want unchanged %d: a failed open must not relocate the cursor in whatever tab was already active", got, before)
	}
	if !m.searchResultsActive {
		t.Error("searchResultsActive = false after a failed jump, want true: the results list should stay open so the user can pick a different match")
	}
}
