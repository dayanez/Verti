package buffer

import "testing"

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
