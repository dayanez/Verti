// Package display renders a buffer.GapBuffer, styled by a
// highlight.Highlighter, into terminal text: line numbers, scrolling
// (vertical and horizontal), and a cursor cell.
package display

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/dommcpro/verti/pkg/buffer"
	"github.com/dommcpro/verti/pkg/highlight"
)

// Theme holds the styles used to render each highlight.Kind, plus the
// gutter and cursor. Colors are chosen to be readable on both light and
// dark 256-color terminals.
type Theme struct {
	Default        lipgloss.Style
	Keyword        lipgloss.Style
	String         lipgloss.Style
	Comment        lipgloss.Style
	Number         lipgloss.Style
	Function       lipgloss.Style
	Type           lipgloss.Style
	Variable       lipgloss.Style
	Operator       lipgloss.Style
	Punctuation    lipgloss.Style
	Constant       lipgloss.Style
	Heading        lipgloss.Style
	LineNumber     lipgloss.Style
	Cursor         lipgloss.Style
	Selection      lipgloss.Style
	Scrollbar      lipgloss.Style
	ScrollbarThumb lipgloss.Style
}

// DefaultTheme returns the editor's built-in color scheme.
func DefaultTheme() Theme {
	return Theme{
		Default:        lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Keyword:        lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		String:         lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		Comment:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true),
		Number:         lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
		Function:       lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
		Type:           lipgloss.NewStyle().Foreground(lipgloss.Color("221")),
		Variable:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Operator:       lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		Punctuation:    lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		Constant:       lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true),
		Heading:        lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Underline(true),
		LineNumber:     lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Cursor:         lipgloss.NewStyle().Reverse(true),
		Selection:      lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")),
		Scrollbar:      lipgloss.NewStyle().Foreground(lipgloss.Color("237")),
		ScrollbarThumb: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

func (t Theme) styleFor(kind highlight.Kind) lipgloss.Style {
	switch kind {
	case highlight.KindKeyword:
		return t.Keyword
	case highlight.KindString:
		return t.String
	case highlight.KindComment:
		return t.Comment
	case highlight.KindNumber:
		return t.Number
	case highlight.KindFunction:
		return t.Function
	case highlight.KindType:
		return t.Type
	case highlight.KindVariable:
		return t.Variable
	case highlight.KindOperator:
		return t.Operator
	case highlight.KindPunctuation:
		return t.Punctuation
	case highlight.KindConstant:
		return t.Constant
	case highlight.KindHeading:
		return t.Heading
	default:
		return t.Default
	}
}

// Editor renders a viewport onto a buffer.GapBuffer.
type Editor struct {
	Width, Height int
	ScrollLine    int
	ScrollCol     int
	Theme         Theme
}

// New returns an Editor with the default theme.
func New() *Editor {
	return &Editor{Theme: DefaultTheme()}
}

// SetSize sets the viewport dimensions (including the line-number gutter).
func (e *Editor) SetSize(w, h int) {
	e.Width, e.Height = w, h
}

func gutterWidth(lineCount int) int {
	return len(strconv.Itoa(lineCount)) + 2
}

// scrollbarWidth is the one column always reserved on the right edge of
// the viewport for the scroll indicator, kept constant (rather than only
// appearing once a file is taller than the viewport) so the gutter and
// text don't jump sideways as you type past that threshold.
const scrollbarWidth = 1

// EnsureCursorVisible adjusts scroll offsets so the buffer's cursor stays
// within the viewport.
func (e *Editor) EnsureCursorVisible(buf *buffer.GapBuffer) {
	line, col := buf.CursorLineCol()

	if line < e.ScrollLine {
		e.ScrollLine = line
	}
	if e.Height > 0 && line >= e.ScrollLine+e.Height {
		e.ScrollLine = line - e.Height + 1
	}
	if e.ScrollLine < 0 {
		e.ScrollLine = 0
	}

	visibleWidth := e.Width - gutterWidth(buf.LineCount()) - scrollbarWidth
	if visibleWidth < 1 {
		visibleWidth = 1
	}
	if col < e.ScrollCol {
		e.ScrollCol = col
	}
	if col >= e.ScrollCol+visibleWidth {
		e.ScrollCol = col - visibleWidth + 1
	}
	if e.ScrollCol < 0 {
		e.ScrollCol = 0
	}
}

// ScreenToOffset converts a screen coordinate within the editor's
// rendered area (0,0 at its top-left, the same coordinate space Render
// draws into, including the gutter) to a logical rune offset in buf. It
// reports false, rather than guessing, for a coordinate that isn't over
// the text itself: outside the viewport, in the gutter, or on the
// scrollbar column. A y below the last real line clamps to the last
// line, matching where a click past the end of a short file should
// reasonably land.
func (e *Editor) ScreenToOffset(buf *buffer.GapBuffer, x, y int) (offset int, ok bool) {
	if e.Width <= 0 || e.Height <= 0 || x < 0 || y < 0 || y >= e.Height {
		return 0, false
	}
	totalLines := buf.LineCount()
	gutterW := gutterWidth(totalLines) // already includes the separating space before the text, see Render
	textX := x - gutterW
	if textX < 0 {
		return 0, false
	}
	visibleWidth := e.Width - gutterW - scrollbarWidth
	if textX >= visibleWidth {
		return 0, false
	}

	line := e.ScrollLine + y
	if line >= totalLines {
		line = totalLines - 1
	}
	if line < 0 {
		line = 0
	}
	return buf.OffsetForLineCol(line, e.ScrollCol+textX), true
}

// Render draws the buffer's currently-visible lines with syntax
// highlighting, line numbers, and (when focused) a visible cursor cell.
// hl may be nil, in which case everything renders with the default style.
func (e *Editor) Render(buf *buffer.GapBuffer, hl highlight.Highlighter, focused bool) string {
	if e.Width <= 0 || e.Height <= 0 {
		return ""
	}
	e.EnsureCursorVisible(buf)

	totalLines := buf.LineCount()
	cursorLine, cursorCol := buf.CursorLineCol()
	gutterW := gutterWidth(totalLines)

	selStart, selEnd, hasSel := buf.Selection()
	var selStartLine, selStartCol, selEndLine, selEndCol int
	if hasSel {
		selStartLine, selStartCol = buf.OffsetLineCol(selStart)
		selEndLine, selEndCol = buf.OffsetLineCol(selEnd)
	}
	visibleWidth := e.Width - gutterW - scrollbarWidth
	if visibleWidth < 1 {
		visibleWidth = 1
	}

	end := e.ScrollLine + e.Height
	if end > totalLines {
		end = totalLines
	}

	// Only materialize the visible line range, not the whole file: on a
	// huge file, splitting every line into its own string on every single
	// keystroke (most of them thrown away unrendered) is wasted work and
	// GC pressure that scales with file size instead of viewport size.
	visible := buf.LinesRange(e.ScrollLine, end)

	// The highlighter still needs the full source to parse accurately
	// (see pkg/highlight's incremental parser), but only the visible
	// range's tokens are worth classifying and walking below.
	text := buf.String()
	viewStart := lineByteOffset(text, e.ScrollLine)

	lineStartByte := make([]int, len(visible))
	offset := viewStart
	for i, l := range visible {
		lineStartByte[i] = offset
		offset += len(l) + 1
	}
	viewEnd := offset

	var tokens []highlight.Token
	if hl != nil {
		if toks, err := hl.Highlight([]byte(text), viewStart, viewEnd); err == nil {
			tokens = toks
			sort.Slice(tokens, func(i, j int) bool { return tokens[i].StartByte < tokens[j].StartByte })
		}
	}

	tIdx := 0
	for tIdx < len(tokens) && tokens[tIdx].EndByte <= viewStart {
		tIdx++
	}

	thumbStart, thumbEnd := e.scrollbarThumb(totalLines)

	rows := make([]string, 0, e.Height)
	for vi, ln := range visible {
		i := e.ScrollLine + vi
		selFrom, selTo := -1, -1
		if hasSel && i >= selStartLine && i <= selEndLine {
			selFrom, selTo = 0, len([]rune(ln))
			if i == selStartLine {
				selFrom = selStartCol
			}
			if i == selEndLine {
				selTo = selEndCol
			}
		}
		gutter := e.Theme.LineNumber.Width(gutterW - 1).Align(lipgloss.Right).Render(strconv.Itoa(i + 1))
		line := e.renderLine(ln, lineStartByte[vi], tokens, &tIdx, i == cursorLine && focused, cursorCol, visibleWidth, selFrom, selTo)
		rows = append(rows, gutter+" "+line+scrollbarCell(e.Theme, vi, thumbStart, thumbEnd))
	}
	for len(rows) < e.Height {
		rowIdx := len(rows)
		gutter := e.Theme.LineNumber.Width(gutterW - 1).Align(lipgloss.Right).Render("~")
		rows = append(rows, gutter+" "+strings.Repeat(" ", visibleWidth)+scrollbarCell(e.Theme, rowIdx, thumbStart, thumbEnd))
	}
	return strings.Join(rows, "\n")
}

// lineByteOffset returns the byte offset of the start of the given
// zero-based line within text, using IndexByte to jump newline to newline
// instead of splitting the whole string into a slice of every line just to
// find one boundary.
func lineByteOffset(text string, line int) int {
	off := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(text[off:], '\n')
		if idx < 0 {
			return len(text)
		}
		off += idx + 1
	}
	return off
}

// scrollbarThumb returns the [start, end) row range (within the
// viewport's e.Height rows) the scrollbar thumb should cover for a file
// of totalLines, proportional to how much of it is currently visible and
// how far scrolled it is. When everything fits in the viewport, the
// thumb fills every row, matching the usual convention of an unscrollable
// track.
func (e *Editor) scrollbarThumb(totalLines int) (start, end int) {
	if totalLines < 1 {
		totalLines = 1
	}
	size := e.Height * e.Height / totalLines
	if size < 1 {
		size = 1
	}
	if size > e.Height {
		size = e.Height
	}
	maxScroll := totalLines - e.Height
	if maxScroll > 0 {
		track := e.Height - size
		start = e.ScrollLine * track / maxScroll
		if start > track {
			start = track
		}
	}
	return start, start + size
}

// scrollbarCell renders the one-column scrollbar cell for viewport row
// rowIdx: the thumb glyph if rowIdx falls within [thumbStart, thumbEnd),
// the track glyph otherwise.
func scrollbarCell(theme Theme, rowIdx, thumbStart, thumbEnd int) string {
	if rowIdx >= thumbStart && rowIdx < thumbEnd {
		return theme.ScrollbarThumb.Render("█")
	}
	return theme.Scrollbar.Render("│")
}

// renderLine draws one line. selFrom/selTo are the rune-column bounds of
// the active selection on this line ([-1, -1] if none on this line);
// selection styling takes precedence over syntax highlighting, but the
// cursor cell always wins over both.
func (e *Editor) renderLine(line string, lineStart int, tokens []highlight.Token, tIdx *int, isCursorLine bool, cursorCol, visibleWidth, selFrom, selTo int) string {
	runes := []rune(line)
	var out strings.Builder
	var run strings.Builder
	runKind := highlight.KindDefault
	runIsCursor := false
	runIsSelected := false

	flush := func() {
		if run.Len() == 0 {
			return
		}
		switch {
		case runIsCursor:
			out.WriteString(e.Theme.Cursor.Render(run.String()))
		case runIsSelected:
			out.WriteString(e.Theme.Selection.Render(run.String()))
		default:
			out.WriteString(e.Theme.styleFor(runKind).Render(run.String()))
		}
		run.Reset()
	}

	byteOff := lineStart
	rendered := 0
	for j, r := range runes {
		rl := utf8.RuneLen(r)
		for *tIdx < len(tokens) && tokens[*tIdx].EndByte <= byteOff {
			*tIdx++
		}
		kind := highlight.KindDefault
		if *tIdx < len(tokens) && tokens[*tIdx].StartByte <= byteOff && byteOff < tokens[*tIdx].EndByte {
			kind = tokens[*tIdx].Kind
		}
		isCursor := isCursorLine && j == cursorCol
		isSelected := selFrom >= 0 && j >= selFrom && j < selTo

		if j < e.ScrollCol {
			byteOff += rl
			continue
		}
		if rendered >= visibleWidth {
			break
		}

		if run.Len() > 0 && (isCursor != runIsCursor || isSelected != runIsSelected || (!isCursor && !isSelected && kind != runKind)) {
			flush()
		}
		runKind, runIsCursor, runIsSelected = kind, isCursor, isSelected
		run.WriteRune(r)
		rendered++
		byteOff += rl
	}
	flush()

	if isCursorLine && cursorCol >= len(runes) && cursorCol >= e.ScrollCol && rendered < visibleWidth {
		out.WriteString(e.Theme.Cursor.Render(" "))
		rendered++
	}
	// Pad out to visibleWidth so every row is exactly the same visual
	// width regardless of line length: the scrollbar column appended
	// after this needs a consistent landing spot, not one that drifts
	// left on short lines.
	if rendered < visibleWidth {
		out.WriteString(strings.Repeat(" ", visibleWidth-rendered))
	}
	return out.String()
}
