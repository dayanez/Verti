package app

// bracketPairs and bracketOpeners are derived from autoPairs (see
// editpairs.go), minus the quote entries: quotes aren't nested the way
// brackets are, so "jump to match" doesn't apply to them. Deriving these
// rather than listing the brackets a second time keeps the two features'
// notion of "which characters pair up" from drifting apart.
var bracketPairs = func() map[rune]rune {
	pairs := make(map[rune]rune, len(autoPairs))
	for open, close := range autoPairs {
		if !isQuote(open) {
			pairs[open] = close
		}
	}
	return pairs
}()

var bracketOpeners = func() map[rune]rune {
	openers := make(map[rune]rune, len(bracketPairs))
	for open, close := range bracketPairs {
		openers[close] = open
	}
	return openers
}()

func isBracket(r rune) bool {
	_, isOpener := bracketPairs[r]
	_, isCloser := bracketOpeners[r]
	return isOpener || isCloser
}

// jumpToMatchingBracket moves the cursor to the bracket that matches the
// one at the cursor, or immediately before it (so placing the cursor
// right after a bracket works too), scanning forward for an opener or
// backward for a closer while tracking nesting depth so an enclosing
// pair -- not just the nearest bracket of the same kind -- is found
// correctly. It's a no-op, with a status message, if there's no bracket
// to jump from or no match for it.
func (m *Model) jumpToMatchingBracket() {
	r, pos, ok := m.bracketNear(m.buf.CursorOffset())
	if !ok {
		m.status = "no bracket at cursor"
		return
	}

	var match int
	var found bool
	if closer, isOpener := bracketPairs[r]; isOpener {
		match, found = m.scanForward(pos+1, r, closer)
	} else {
		match, found = m.scanBackward(pos, bracketOpeners[r], r)
	}
	if !found {
		m.status = "no matching bracket found"
		return
	}
	m.buf.ClearSelection()
	m.buf.MoveCursorTo(match)
}

// bracketNear returns the bracket rune at offset itself, or immediately
// before it, along with its own position.
func (m *Model) bracketNear(offset int) (r rune, pos int, ok bool) {
	if s := m.buf.TextRange(offset, offset+1); s != "" {
		if rr := []rune(s)[0]; isBracket(rr) {
			return rr, offset, true
		}
	}
	if offset > 0 {
		if s := m.buf.TextRange(offset-1, offset); s != "" {
			if rr := []rune(s)[0]; isBracket(rr) {
				return rr, offset - 1, true
			}
		}
	}
	return 0, 0, false
}

// scanForward and scanBackward each fetch the whole span they might need
// to search once, rather than calling buf.TextRange per character: a
// bracket match is usually close by, but an unmatched one means scanning
// to the end (or start) of the file, and a lock plus an allocation for
// every single character on the way there is exactly the kind of
// per-keystroke-shaped cost this codebase has spent effort eliminating
// elsewhere. One bulk fetch, then a plain local scan, costs one lock no
// matter how far the scan goes.
func (m *Model) scanForward(from int, open, close rune) (int, bool) {
	// []rune, not a range over the string directly: TextRange's offsets
	// (and the offset this returns) are rune offsets, but ranging over a
	// string yields byte offsets, which only agree with rune offsets for
	// pure ASCII text.
	text := []rune(m.buf.TextRange(from, m.buf.Len()))
	depth := 1
	for i, r := range text {
		switch r {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return from + i, true
			}
		}
	}
	return 0, false
}

func (m *Model) scanBackward(from int, open, close rune) (int, bool) {
	text := []rune(m.buf.TextRange(0, from))
	depth := 1
	for i := len(text) - 1; i >= 0; i-- {
		switch text[i] {
		case close:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}
