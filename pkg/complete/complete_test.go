package complete

import (
	"reflect"
	"testing"
)

func TestPrefixStartScansBackOverIdentifierRunes(t *testing.T) {
	text := []rune("foo.wo")
	if got := PrefixStart(text, len(text)); got != 4 {
		t.Fatalf("PrefixStart() = %d, want 4 (start of \"wo\")", got)
	}
}

func TestPrefixStartStopsAtBufferStart(t *testing.T) {
	text := []rune("word")
	if got := PrefixStart(text, len(text)); got != 0 {
		t.Fatalf("PrefixStart() = %d, want 0", got)
	}
}

func TestCandidatesFindsLongerMatchesOrderedByFirstAppearance(t *testing.T) {
	text := []rune("worker work workspace wor")
	// "wor" typed at the very end, offset 26 (start 23, end 26).
	got := Candidates(text, "wor", 23, 26)
	want := []string{"worker", "work", "workspace"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates() = %v, want %v", got, want)
	}
}

func TestCandidatesExcludesExactPrefixMatch(t *testing.T) {
	text := []rune("cat category cat")
	got := Candidates(text, "cat", 14, 17)
	want := []string{"category"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates() = %v, want %v (a bare \"cat\" elsewhere shouldn't complete into itself)", got, want)
	}
}

func TestCandidatesExcludesTheOccurrenceBeingTyped(t *testing.T) {
	text := []rune("workspace wo")
	// "wo" typed at the end, offset 12 (start 10, end 12); "workspace"
	// itself starts with "wo" too and must still be offered.
	got := Candidates(text, "wo", 10, 12)
	want := []string{"workspace"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates() = %v, want %v", got, want)
	}
}

func TestCandidatesExcludesTheOccurrenceBeingTypedMidWord(t *testing.T) {
	text := []rune("word")
	// Cursor sits between "wo" and "rd" (offset 2), so prefix "wo" is
	// typed mid-word: prefixStart=0, prefixEnd=2, but the identifier run
	// containing the cursor extends to 4, past prefixEnd. The run must
	// still be excluded -- offering "word" here as a completion of "wo"
	// would corrupt the buffer into "wordrd" if accepted.
	got := Candidates(text, "wo", 0, 2)
	if got != nil {
		t.Fatalf("Candidates() = %v, want nil (the word under the cursor must never complete into itself)", got)
	}
}

func TestCandidatesDedupesRepeatedWords(t *testing.T) {
	text := []rune("value value value val")
	got := Candidates(text, "val", 19, 22)
	want := []string{"value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates() = %v, want %v (deduped)", got, want)
	}
}

func TestCandidatesEmptyPrefixReturnsNil(t *testing.T) {
	if got := Candidates([]rune("anything"), "", 0, 0); got != nil {
		t.Fatalf("Candidates() with empty prefix = %v, want nil", got)
	}
}

func TestCandidatesNoMatchesReturnsNil(t *testing.T) {
	text := []rune("apple banana")
	if got := Candidates(text, "zzz", 0, 0); got != nil {
		t.Fatalf("Candidates() = %v, want nil", got)
	}
}

func TestIsWordRune(t *testing.T) {
	cases := map[rune]bool{'a': true, 'Z': true, '9': true, '_': true, '.': false, ' ': false, '-': false}
	for r, want := range cases {
		if got := IsWordRune(r); got != want {
			t.Errorf("IsWordRune(%q) = %v, want %v", r, got, want)
		}
	}
}
