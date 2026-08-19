// Package display renders a buffer.GapBuffer, styled by a
// highlight.Highlighter, into terminal text — line numbers, scrolling
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
	Default     lipgloss.Style
	Keyword     lipgloss.Style
	String      lipgloss.Style
	Comment     lipgloss.Style
	Number      lipgloss.Style
	Function    lipgloss.Style
	Type        lipgloss.Style
	Variable    lipgloss.Style
	Operator    lipgloss.Style
	Punctuation lipgloss.Style
	Constant    lipgloss.Style
	Heading     lipgloss.Style
	LineNumber  lipgloss.Style
	Cursor      lipgloss.Style
	Selection   lipgloss.Style
}

// DefaultTheme returns the editor's built-in color scheme.
func DefaultTheme() Theme {
	return Theme{
		Default:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Keyword:     lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		String:      lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		Comment:     lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true),
		Number:      lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
		Function:    lipgloss.NewStyle().Foreground(lipgloss.Color("111")),
		Type:        lipgloss.NewStyle().Foreground(lipgloss.Color("221")),
		Variable:    lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Operator:    lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		Punctuation: lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		Constant:    lipgloss.NewStyle().Foreground(lipgloss.Color("215")).Bold(true),
		Heading:     lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Underline(true),
		LineNumber:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Cursor:      lipgloss.NewStyle().Reverse(true),
		Selection:   lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")),
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

	visibleWidth := e.Width - gutterWidth(buf.LineCount())
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

// Render draws the buffer's currently-visible lines with syntax
// highlighting, line numbers, and (when focused) a visible cursor cell.
// hl may be nil, in which case everything renders with the default style.
func (e *Editor) Render(buf *buffer.GapBuffer, hl highlight.Highlighter, focused bool) string {
	if e.Width <= 0 || e.Height <= 0 {
		return ""
	}
	e.EnsureCursorVisible(buf)

	lines := buf.Lines()
	cursorLine, cursorCol := buf.CursorLineCol()
	gutterW := gutterWidth(len(lines))

	selStart, selEnd, hasSel := buf.Selection()
	var selStartLine, selStartCol, selEndLine, selEndCol int
	if hasSel {
		selStartLine, selStartCol = buf.OffsetLineCol(selStart)
		selEndLine, selEndCol = buf.OffsetLineCol(selEnd)
	}
	visibleWidth := e.Width - gutterW
	if visibleWidth < 1 {
		visibleWidth = 1
	}

	var tokens []highlight.Token
	if hl != nil {
		if toks, err := hl.Highlight([]byte(buf.String())); err == nil {
			tokens = toks
			sort.Slice(tokens, func(i, j int) bool { return tokens[i].StartByte < tokens[j].StartByte })
		}
	}

	lineStartByte := make([]int, len(lines))
	offset := 0
	for i, l := range lines {
		lineStartByte[i] = offset
		offset += len(l) + 1
	}

	end := e.ScrollLine + e.Height
	if end > len(lines) {
		end = len(lines)
	}

	tIdx := 0
	if e.ScrollLine < len(lineStartByte) {
		start := lineStartByte[e.ScrollLine]
		for tIdx < len(tokens) && tokens[tIdx].EndByte <= start {
			tIdx++
		}
	}

	rows := make([]string, 0, e.Height)
	for i := e.ScrollLine; i < end; i++ {
		selFrom, selTo := -1, -1
		if hasSel && i >= selStartLine && i <= selEndLine {
			selFrom, selTo = 0, len([]rune(lines[i]))
			if i == selStartLine {
				selFrom = selStartCol
			}
			if i == selEndLine {
				selTo = selEndCol
			}
		}
		gutter := e.Theme.LineNumber.Width(gutterW - 1).Align(lipgloss.Right).Render(strconv.Itoa(i + 1))
		rows = append(rows, gutter+" "+e.renderLine(lines[i], lineStartByte[i], tokens, &tIdx, i == cursorLine && focused, cursorCol, visibleWidth, selFrom, selTo))
	}
	for len(rows) < e.Height {
		rows = append(rows, e.Theme.LineNumber.Width(gutterW-1).Align(lipgloss.Right).Render("~")+" ")
	}
	return strings.Join(rows, "\n")
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
	}
	return out.String()
}
