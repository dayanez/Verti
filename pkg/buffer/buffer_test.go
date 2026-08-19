package buffer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsertAndString(t *testing.T) {
	gb := New()
	gb.InsertString("hello")
	if got := gb.String(); got != "hello" {
		t.Fatalf("String() = %q, want %q", got, "hello")
	}
	if got := gb.Len(); got != 5 {
		t.Fatalf("Len() = %d, want 5", got)
	}
}

func TestInsertAtCursorMiddle(t *testing.T) {
	gb := NewFromString("helloworld")
	gb.MoveCursorTo(5)
	gb.InsertString(", ")
	if got := gb.String(); got != "hello, world" {
		t.Fatalf("String() = %q, want %q", got, "hello, world")
	}
}

func TestDeleteBackwardAndForward(t *testing.T) {
	gb := NewFromString("hello")
	gb.MoveCursorTo(5)
	if !gb.DeleteBackward() {
		t.Fatal("DeleteBackward() = false, want true")
	}
	if got := gb.String(); got != "hell" {
		t.Fatalf("String() = %q, want %q", got, "hell")
	}
	gb.MoveCursorTo(0)
	if !gb.DeleteForward() {
		t.Fatal("DeleteForward() = false, want true")
	}
	if got := gb.String(); got != "ell" {
		t.Fatalf("String() = %q, want %q", got, "ell")
	}
	gb.MoveCursorTo(0)
	if gb.DeleteBackward() {
		t.Fatal("DeleteBackward() at offset 0 should return false")
	}
}

func TestGapGrowsAcrossManyInserts(t *testing.T) {
	gb := New()
	for i := 0; i < 1000; i++ {
		gb.InsertRune('x')
	}
	if got := gb.Len(); got != 1000 {
		t.Fatalf("Len() = %d, want 1000", got)
	}
}

func TestLinesAndCursorLineCol(t *testing.T) {
	gb := NewFromString("abc\ndefgh\nij")
	gb.MoveCursorTo(0)
	gb.MoveCursorTo(6) // "abc\nde|fgh\nij" -> line 1, col 2
	line, col := gb.CursorLineCol()
	if line != 1 || col != 2 {
		t.Fatalf("CursorLineCol() = (%d, %d), want (1, 2)", line, col)
	}
	if got := gb.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, want 3", got)
	}
}

func TestMoveUpDownPreservesDesiredColumn(t *testing.T) {
	gb := NewFromString("longline\nhi\nlongline")
	gb.MoveCursorTo(6) // col 6 on first (long) line
	gb.MoveDown()      // "hi" is short, should clamp to col 2
	_, col := gb.CursorLineCol()
	if col != 2 {
		t.Fatalf("after MoveDown onto short line, col = %d, want 2", col)
	}
	gb.MoveDown() // back onto a long line, should restore desired col 6
	line, col := gb.CursorLineCol()
	if line != 2 || col != 6 {
		t.Fatalf("after MoveDown restoring column, (line,col) = (%d,%d), want (2,6)", line, col)
	}
}

func TestHomeAndEnd(t *testing.T) {
	gb := NewFromString("hello\nworld")
	gb.MoveCursorTo(8) // within "world"
	gb.MoveHome()
	line, col := gb.CursorLineCol()
	if line != 1 || col != 0 {
		t.Fatalf("MoveHome -> (%d,%d), want (1,0)", line, col)
	}
	gb.MoveEnd()
	line, col = gb.CursorLineCol()
	if line != 1 || col != 5 {
		t.Fatalf("MoveEnd -> (%d,%d), want (1,5)", line, col)
	}
}

func TestRestore(t *testing.T) {
	gb := NewFromString("original")
	gb.Restore("replaced text", 4)
	if got := gb.String(); got != "replaced text" {
		t.Fatalf("String() = %q, want %q", got, "replaced text")
	}
	if got := gb.CursorOffset(); got != 4 {
		t.Fatalf("CursorOffset() = %d, want 4", got)
	}
}

func TestSelectionRangeAndText(t *testing.T) {
	gb := NewFromString("hello world")
	gb.MoveCursorTo(2)
	gb.StartSelection()
	gb.MoveCursorTo(7) // "he|llo w|orld" -> selection [2,7)

	start, end, ok := gb.Selection()
	if !ok || start != 2 || end != 7 {
		t.Fatalf("Selection() = (%d,%d,%v), want (2,7,true)", start, end, ok)
	}
	if got := gb.SelectedText(); got != "llo w" {
		t.Fatalf("SelectedText() = %q, want %q", got, "llo w")
	}

	// An anchor with no subsequent movement is an empty selection.
	gb.ClearSelection()
	gb.StartSelection()
	if gb.HasSelection() {
		t.Fatal("HasSelection() = true immediately after StartSelection with no movement, want false")
	}
}

func TestDeleteSelectionIsNoOpWithoutOne(t *testing.T) {
	gb := NewFromString("unchanged")
	if got := gb.DeleteSelection(); got != "" {
		t.Fatalf("DeleteSelection() with no selection = %q, want \"\"", got)
	}
	if got := gb.String(); got != "unchanged" {
		t.Fatalf("String() = %q, want %q", got, "unchanged")
	}
}

func TestDeleteSelectionRemovesRangeAndMovesCursor(t *testing.T) {
	gb := NewFromString("hello world")
	gb.MoveCursorTo(7)
	gb.StartSelection()
	gb.MoveCursorTo(2) // reversed selection: anchor(7) > cursor(2) -> range [2,7)

	deleted := gb.DeleteSelection()
	if deleted != "llo w" {
		t.Fatalf("DeleteSelection() = %q, want %q", deleted, "llo w")
	}
	if got := gb.String(); got != "heorld" {
		t.Fatalf("String() = %q, want %q", got, "heorld")
	}
	if got := gb.CursorOffset(); got != 2 {
		t.Fatalf("CursorOffset() = %d, want 2", got)
	}
	if gb.HasSelection() {
		t.Fatal("HasSelection() = true after DeleteSelection, want false")
	}
}

func TestCurrentLineRangeIncludesTrailingNewlineExceptLastLine(t *testing.T) {
	gb := NewFromString("abc\ndefgh\nij")

	gb.MoveCursorTo(1) // within "abc"
	start, end := gb.CurrentLineRange()
	if start != 0 || end != 4 { // "abc\n"
		t.Fatalf("CurrentLineRange() on line 0 = (%d,%d), want (0,4)", start, end)
	}
	if got := gb.TextRange(start, end); got != "abc\n" {
		t.Fatalf("TextRange(0,4) = %q, want %q", got, "abc\n")
	}

	gb.MoveCursorTo(11) // within "ij", the last line, no trailing newline
	start, end = gb.CurrentLineRange()
	if got := gb.TextRange(start, end); got != "ij" {
		t.Fatalf("TextRange on last line = %q, want %q", got, "ij")
	}
	if end != gb.Len() {
		t.Fatalf("CurrentLineRange() end on last line = %d, want Len() = %d", end, gb.Len())
	}
}

func TestOffsetLineCol(t *testing.T) {
	gb := NewFromString("abc\ndefgh\nij")
	line, col := gb.OffsetLineCol(6) // "abc\nde|fgh\nij"
	if line != 1 || col != 2 {
		t.Fatalf("OffsetLineCol(6) = (%d,%d), want (1,2)", line, col)
	}
}

func TestMoveWordRightAndLeft(t *testing.T) {
	gb := NewFromString("the  quick brown")
	gb.MoveCursorTo(0)

	gb.MoveWordRight() // "the  " -> land at "quick"
	if got := gb.CursorOffset(); got != 5 {
		t.Fatalf("after MoveWordRight, CursorOffset() = %d, want 5", got)
	}
	gb.MoveWordRight() // "quick " -> land at "brown"
	if got := gb.CursorOffset(); got != 11 {
		t.Fatalf("after 2nd MoveWordRight, CursorOffset() = %d, want 11", got)
	}
	gb.MoveWordRight() // no more words -> end of buffer
	if got := gb.CursorOffset(); got != gb.Len() {
		t.Fatalf("after 3rd MoveWordRight, CursorOffset() = %d, want Len() = %d", got, gb.Len())
	}

	gb.MoveWordLeft() // back to start of "brown"
	if got := gb.CursorOffset(); got != 11 {
		t.Fatalf("after MoveWordLeft, CursorOffset() = %d, want 11", got)
	}
	gb.MoveWordLeft() // back to start of "quick"
	if got := gb.CursorOffset(); got != 5 {
		t.Fatalf("after 2nd MoveWordLeft, CursorOffset() = %d, want 5", got)
	}
	gb.MoveWordLeft() // no more words -> offset 0
	if got := gb.CursorOffset(); got != 0 {
		t.Fatalf("after 3rd MoveWordLeft, CursorOffset() = %d, want 0", got)
	}
}

func TestLineOffset(t *testing.T) {
	gb := NewFromString("abc\ndefgh\nij")
	if got := gb.LineOffset(0); got != 0 {
		t.Fatalf("LineOffset(0) = %d, want 0", got)
	}
	if got := gb.LineOffset(1); got != 4 {
		t.Fatalf("LineOffset(1) = %d, want 4", got)
	}
	if got := gb.LineOffset(2); got != 10 {
		t.Fatalf("LineOffset(2) = %d, want 10", got)
	}
	// out-of-range clamps to the last line, matching gotoLine's usage.
	if got := gb.LineOffset(99); got != 10 {
		t.Fatalf("LineOffset(99) = %d, want 10 (clamped)", got)
	}
}

func TestSelectRange(t *testing.T) {
	gb := NewFromString("hello world")
	gb.SelectRange(6, 11)
	if got := gb.SelectedText(); got != "world" {
		t.Fatalf("SelectedText() = %q, want %q", got, "world")
	}
	if got := gb.CursorOffset(); got != 11 {
		t.Fatalf("CursorOffset() after SelectRange = %d, want 11 (the higher end)", got)
	}

	// Order-independence: SelectRange(11, 6) should select the same text.
	gb.ClearSelection()
	gb.SelectRange(11, 6)
	if got := gb.SelectedText(); got != "world" {
		t.Fatalf("SelectedText() after reversed SelectRange = %q, want %q", got, "world")
	}
}

func TestLoadFileNormalizesCRLFAndSaveFileRestoresIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	if err := os.WriteFile(path, []byte("one\r\ntwo\r\nthree"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gb, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got := gb.String(); got != "one\ntwo\nthree" {
		t.Fatalf("String() = %q, want CRLF normalized to LF", got)
	}
	if got := gb.LineCount(); got != 3 {
		t.Fatalf("LineCount() = %d, want 3 (a stray \\r would throw this off)", got)
	}

	gb.InsertString("!")
	if err := gb.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(raw); got != "!one\r\ntwo\r\nthree" {
		t.Fatalf("saved file = %q, want CRLF line endings restored", got)
	}
}

func TestLoadFileWithLFOnlySavesAsLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lf.txt")
	if err := os.WriteFile(path, []byte("one\ntwo"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gb, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if err := gb.SaveFile(path); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(raw); got != "one\ntwo" {
		t.Fatalf("saved file = %q, want LF preserved (no CRLF introduced)", got)
	}
}

func TestConcurrentReadsDoNotRace(t *testing.T) {
	gb := NewFromString("concurrent access test")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 500; i++ {
			_ = gb.String()
			_, _ = gb.CursorLineCol()
		}
		close(done)
	}()
	for i := 0; i < 500; i++ {
		gb.InsertRune('z')
		gb.DeleteBackward()
	}
	<-done
}
