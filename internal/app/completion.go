package app

import (
	"fmt"

	"github.com/dayanez/Verti/pkg/complete"
)

// ---------------- Word Completion ----------------

// tryWordCompletion handles Tab when there's no selection to indent or
// replace: if the cursor sits right after a partial identifier, it
// completes it from another occurrence already in the buffer (or cycles
// to the next one, if wasActive means this Tab immediately follows a
// completion this function just inserted), and reports whether it
// handled the key at all -- false means the cursor wasn't in a
// completable position, so the caller should fall through to Tab's
// normal insert-spaces behavior.
func (m *Model) tryWordCompletion(wasActive bool) bool {
	if m.buf.HasSelection() {
		return false
	}
	if wasActive && len(m.completionCandidates) > 0 {
		m.cycleCompletion()
		return true
	}
	return m.startCompletion()
}

func (m *Model) startCompletion() bool {
	offset := m.buf.CursorOffset()
	text := []rune(m.buf.String())
	if offset <= 0 || offset > len(text) || !complete.IsWordRune(text[offset-1]) {
		return false
	}
	start := complete.PrefixStart(text, offset)
	prefix := string(text[start:offset])
	candidates := complete.Candidates(text, prefix, start, offset)
	if len(candidates) == 0 {
		m.status = fmt.Sprintf("no completions for %q", prefix)
		return false
	}

	m.recordUndoBoundary(editInsert)
	m.buf.SelectRange(start, offset)
	m.buf.DeleteSelection()
	m.buf.InsertString(candidates[0])

	m.completionActive = true
	m.completionPrefixStart = start
	m.completionCandidates = candidates
	m.completionIndex = 0
	m.status = fmt.Sprintf("completion 1/%d (Tab to cycle)", len(candidates))
	return true
}

func (m *Model) cycleCompletion() {
	m.completionIndex = (m.completionIndex + 1) % len(m.completionCandidates)
	m.buf.SelectRange(m.completionPrefixStart, m.buf.CursorOffset())
	m.buf.DeleteSelection()
	m.buf.InsertString(m.completionCandidates[m.completionIndex])
	m.completionActive = true
	m.status = fmt.Sprintf("completion %d/%d (Tab to cycle)", m.completionIndex+1, len(m.completionCandidates))
}
