package paint

import (
	"strings"
	"testing"
)

func TestLineHorizontal(t *testing.T) {
	pts := Line(0, 0, 4, 0)
	want := []Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}}
	if len(pts) != len(want) {
		t.Fatalf("len(pts) = %d, want %d", len(pts), len(want))
	}
	for i := range want {
		if pts[i] != want[i] {
			t.Errorf("pts[%d] = %v, want %v", i, pts[i], want[i])
		}
	}
}

func TestLineVertical(t *testing.T) {
	pts := Line(2, 5, 2, 1)
	if len(pts) != 5 {
		t.Fatalf("len(pts) = %d, want 5", len(pts))
	}
	for _, p := range pts {
		if p.X != 2 {
			t.Errorf("point %v not on vertical line x=2", p)
		}
	}
}

func TestLineDiagonal(t *testing.T) {
	pts := Line(0, 0, 3, 3)
	want := []Point{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	if len(pts) != len(want) {
		t.Fatalf("len(pts) = %d, want %d", len(pts), len(want))
	}
	for i := range want {
		if pts[i] != want[i] {
			t.Errorf("pts[%d] = %v, want %v", i, pts[i], want[i])
		}
	}
}

func TestCanvasStraightLinesRenderAsBars(t *testing.T) {
	c := NewCanvas()
	c.DrawLine(0, 0, 4, 0) // horizontal
	rows := c.Render()
	if len(rows) != 1 || rows[0] != "─────" {
		t.Fatalf("Render() = %#v, want [\"─────\"]", rows)
	}
}

func TestCanvasPlusJunction(t *testing.T) {
	c := NewCanvas()
	c.DrawLine(0, 2, 4, 2) // horizontal through row 2
	c.DrawLine(2, 0, 2, 4) // vertical through col 2
	rows := c.Render()
	if len(rows) != 5 {
		t.Fatalf("len(rows) = %d, want 5", len(rows))
	}
	middle := rows[2]
	if []rune(middle)[2] != '┼' {
		t.Fatalf("center of a cross = %q, want '┼'", middle)
	}
	if []rune(rows[0])[2] != '│' {
		t.Fatalf("top of vertical arm = %q, want '│'", rows[0])
	}
}

func TestCanvasCorner(t *testing.T) {
	c := NewCanvas()
	c.DrawLine(0, 0, 3, 0) // horizontal along row 0
	c.DrawLine(0, 0, 0, 3) // vertical along col 0, sharing (0,0)
	rows := c.Render()
	if []rune(rows[0])[0] != '┌' {
		t.Fatalf("shared corner = %q, want '┌'", string([]rune(rows[0])[0]))
	}
}

func TestCanvasDiagonal(t *testing.T) {
	c := NewCanvas()
	c.DrawLine(0, 0, 2, 2)
	rows := c.Render()
	for _, row := range rows {
		if !strings.ContainsRune(row, '╲') && strings.TrimSpace(row) != "" {
			t.Fatalf("expected diagonal glyph in row %q", row)
		}
	}
}

func TestFormatCommentLineStyle(t *testing.T) {
	art := []string{"┌──┐", "│  │", "└──┘"}
	got := FormatComment("box.go", art)
	want := "// ┌──┐\n// │  │\n// └──┘"
	if got != want {
		t.Fatalf("FormatComment() = %q, want %q", got, want)
	}
}

func TestFormatCommentBlockStyle(t *testing.T) {
	art := []string{"┌──┐", "└──┘"}
	got := FormatComment("style.css", art)
	want := "/*\n┌──┐\n└──┘\n*/"
	if got != want {
		t.Fatalf("FormatComment() = %q, want %q", got, want)
	}
}

func TestFormatCommentLuaUsesDashDash(t *testing.T) {
	art := []string{"─"}
	got := FormatComment("init.lua", art)
	if got != "-- ─" {
		t.Fatalf("FormatComment() = %q, want %q", got, "-- ─")
	}
}

func TestOverlayDragProducesComment(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	o.BeginDrag(0, 0)
	o.UpdateDrag(3, 0)
	o.EndDrag(3, 0)
	comment := o.Finish("main.go")
	if !strings.Contains(comment, "─") {
		t.Fatalf("Finish() = %q, expected box-drawing characters", comment)
	}
	if !o.Canvas.Empty() {
		t.Fatal("Finish() should clear the canvas")
	}
}
