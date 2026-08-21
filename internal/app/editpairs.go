package app

import "strings"

// ---------------- Auto-Pairing Brackets & Quotes ----------------

// autoPairs maps an opening delimiter to its closer. Quotes map to
// themselves since the same rune opens and closes them; handleAutoPairRune
// treats that case specially (typing through an existing one instead of
// nesting a fresh pair).
var autoPairs = map[rune]rune{
	'(':  ')',
	'[':  ']',
	'{':  '}',
	'"':  '"',
	'\'': '\'',
}

func isQuote(r rune) bool { return r == '"' || r == '\'' }

func isCloser(r rune) bool { return r == ')' || r == ']' || r == '}' }

// runeAfterCursor returns the rune immediately after the cursor, or
// (0, false) at the end of the buffer.
func (m *Model) runeAfterCursor() (rune, bool) {
	s := m.buf.TextRange(m.buf.CursorOffset(), m.buf.CursorOffset()+1)
	if s == "" {
		return 0, false
	}
	return []rune(s)[0], true
}

// handleAutoPairRune handles typing a single bracket or quote character:
// wrapping the selection if there is one, typing through an
// already-present matching closer instead of duplicating it, or inserting
// a fresh open/close pair with the cursor left between them. It reports
// whether it handled the rune at all; false means the caller should fall
// through to a plain insert.
func (m *Model) handleAutoPairRune(r rune) bool {
	if closer, isOpener := autoPairs[r]; isOpener {
		if m.buf.HasSelection() {
			selected := m.buf.SelectedText()
			m.recordUndoBoundary(editInsert)
			m.buf.DeleteSelection()
			m.buf.InsertString(string(r) + selected + string(closer))
			return true
		}
		if isQuote(r) {
			if next, ok := m.runeAfterCursor(); ok && next == r {
				m.buf.MoveRight() // already sitting before a matching quote: type through it
				return true
			}
		}
		m.recordUndoBoundary(editInsert)
		m.buf.InsertString(string(r) + string(closer))
		m.buf.MoveLeft()
		return true
	}
	if isCloser(r) {
		if next, ok := m.runeAfterCursor(); ok && next == r {
			m.buf.MoveRight() // already sitting before a matching closer: type through it
			return true
		}
	}
	return false
}

// cursorBetweenEmptyPair reports whether the cursor sits directly between
// a matching, empty open/close pair (e.g. "(|)"), the case where
// Backspace should remove both delimiters at once rather than leaving a
// dangling closer behind.
func (m *Model) cursorBetweenEmptyPair() bool {
	offset := m.buf.CursorOffset()
	if offset == 0 {
		return false
	}
	before := m.buf.TextRange(offset-1, offset)
	after := m.buf.TextRange(offset, offset+1)
	if before == "" || after == "" {
		return false
	}
	closer, ok := autoPairs[[]rune(before)[0]]
	return ok && string(closer) == after
}

// ---------------- Smart-Indent on Enter ----------------

// blockIndentContext reports, for the text immediately around the cursor,
// whether Enter should add an extra indent level (the line up to the
// cursor ends with an opening delimiter or a trailing ':', Python-style)
// and whether the cursor sits directly before that opener's matching
// closer (so Enter should also split the closer onto its own line at the
// original indent, e.g. "{|}" becomes "{\n\tindent\n}").
func (m *Model) blockIndentContext() (opensBlock, closesBlock bool) {
	offset := m.buf.CursorOffset()
	lineStart, _ := m.buf.CurrentLineRange()
	before := []rune(strings.TrimRight(m.buf.TextRange(lineStart, offset), " \t"))
	if len(before) == 0 {
		return false, false
	}
	last := before[len(before)-1]
	opensBlock = last == '{' || last == '[' || last == '(' || last == ':'
	if opensBlock {
		if closer, ok := autoPairs[last]; ok {
			if next, ok := m.runeAfterCursor(); ok && next == closer {
				closesBlock = true
			}
		}
	}
	return opensBlock, closesBlock
}

// insertNewlineWithIndent implements Enter's smart-indent behavior:
// carry the current line's indent forward as usual, add one extra level
// after a line ending in an opening delimiter or ':', and split a fresh,
// empty open/close pair onto three lines with the cursor left indented
// in the middle one.
func (m *Model) insertNewlineWithIndent() {
	m.recordUndoBoundary(editInsert)
	m.buf.DeleteSelection()

	indent := m.currentLineIndent()
	extra := strings.Repeat(" ", tabWidthOrDefault(m.cfg.TabWidth))
	opensBlock, closesBlock := m.blockIndentContext()

	if !opensBlock {
		m.buf.InsertString("\n" + indent)
		return
	}
	if closesBlock {
		inner := "\n" + indent + extra
		m.buf.InsertString(inner + "\n" + indent)
		m.buf.MoveCursorTo(m.buf.CursorOffset() - len(indent) - 1)
		return
	}
	m.buf.InsertString("\n" + indent + extra)
}
