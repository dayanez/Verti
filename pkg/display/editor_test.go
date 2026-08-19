package display

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dommcpro/verti/pkg/buffer"
)

func TestScrollbarThumbFillsTrackWhenEverythingFits(t *testing.T) {
	e := New()
	e.SetSize(40, 10)
	start, end := e.scrollbarThumb(5) // fewer lines than the viewport height
	if start != 0 || end != 10 {
		t.Fatalf("scrollbarThumb(5) = (%d,%d), want (0,10): the thumb should fill the whole track", start, end)
	}
}

func TestScrollbarThumbAtTopWhenScrolledToStart(t *testing.T) {
	e := New()
	e.SetSize(40, 10)
	e.ScrollLine = 0
	start, _ := e.scrollbarThumb(1000)
	if start != 0 {
		t.Fatalf("scrollbarThumb start = %d, want 0 when scrolled to the top", start)
	}
}

func TestScrollbarThumbAtBottomWhenScrolledToEnd(t *testing.T) {
	e := New()
	e.SetSize(40, 10)
	total := 1000
	e.ScrollLine = total - e.Height // fully scrolled down
	_, end := e.scrollbarThumb(total)
	if end != e.Height {
		t.Fatalf("scrollbarThumb end = %d, want %d: the thumb should reach the bottom of the track when fully scrolled", end, e.Height)
	}
}

func TestScrollbarThumbMovesMonotonicallyWithScroll(t *testing.T) {
	e := New()
	e.SetSize(40, 10)
	total := 1000

	e.ScrollLine = 0
	start0, _ := e.scrollbarThumb(total)
	e.ScrollLine = total / 2
	startMid, _ := e.scrollbarThumb(total)
	e.ScrollLine = total - e.Height
	startEnd, _ := e.scrollbarThumb(total)

	if !(start0 <= startMid && startMid <= startEnd) {
		t.Fatalf("thumb should move monotonically down as ScrollLine increases: got start=%d mid=%d end=%d", start0, startMid, startEnd)
	}
	if start0 == startEnd {
		t.Fatal("thumb never moved despite scrolling from top to bottom")
	}
}

func TestRenderRowsAreUniformWidthWithScrollbarAligned(t *testing.T) {
	gb := buffer.NewFromString("a\nbb\nccccccccccccccccccccccc\n\nlast")
	e := New()
	e.SetSize(30, 6)
	out := e.Render(gb, nil, true)
	rows := strings.Split(out, "\n")
	if len(rows) != e.Height {
		t.Fatalf("got %d rows, want %d", len(rows), e.Height)
	}

	var width int
	for i, row := range rows {
		plain := ansi.Strip(row)
		runes := []rune(plain)
		if i == 0 {
			width = len(runes)
		} else if len(runes) != width {
			t.Fatalf("row %d width = %d, want %d: rows of different line lengths must still be uniform width for the scrollbar column to line up", i, len(runes), width)
		}
		last := runes[len(runes)-1]
		if last != '│' && last != '█' {
			t.Fatalf("row %d's last rune = %q, want the scrollbar track (│) or thumb (█) glyph", i, last)
		}
	}
}

func TestRenderShowsThumbGlyphWhenScrollable(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("line\n")
	}
	gb := buffer.NewFromString(sb.String())
	e := New()
	e.SetSize(30, 10)
	out := ansi.Strip(e.Render(gb, nil, true))
	if !strings.ContainsRune(out, '█') {
		t.Fatal("expected a thumb glyph somewhere in the render when content exceeds the viewport")
	}
	if !strings.ContainsRune(out, '│') {
		t.Fatal("expected at least one track glyph too, since 100 lines into a 10-row viewport shouldn't fill the whole thumb")
	}
}

func TestRenderHasNoTrackGlyphWhenEverythingFits(t *testing.T) {
	gb := buffer.NewFromString("only\ntwo\nlines")
	e := New()
	e.SetSize(30, 10)
	out := ansi.Strip(e.Render(gb, nil, true))
	if strings.ContainsRune(out, '│') {
		t.Fatalf("expected no track glyph when the thumb fills the whole height (everything fits), got %q", out)
	}
}
