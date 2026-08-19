// Package buffer implements a thread-safe gap buffer, the core text storage
// structure used by the editor view. A gap buffer keeps an unused region
// ("the gap") positioned at the cursor so that typing and deleting near the
// cursor are cheap (amortized O(1)); moving the cursor a long distance costs
// O(distance) to relocate the gap.
package buffer

import (
	"os"
	"strings"
	"sync"
	"unicode"
)

const defaultGapSize = 64

// GapBuffer is a thread-safe, mutable text buffer. The zero value is not
// usable; construct one with New or NewFromString.
type GapBuffer struct {
	mu       sync.RWMutex
	data     []rune
	gapStart int // logical cursor offset; data[gapStart:gapEnd] is unused
	gapEnd   int
	dirty    bool

	// desiredCol remembers the column the cursor was last moved to
	// horizontally, so that moving through short lines and back to a long
	// one restores the original column (VS Code / most editors do this).
	desiredCol int

	// selAnchor is the fixed end of an in-progress selection; the other end
	// is always the cursor (gapStart). hasSel distinguishes "anchored at
	// offset 0" from "no selection".
	selAnchor int
	hasSel    bool

	// crlf records whether the file this buffer was loaded from used CRLF
	// line endings, so SaveFile can restore them. Every other operation in
	// this package assumes '\n' is the sole line separator, so LoadFile
	// normalizes CRLF to LF up front rather than teaching the rest of the
	// buffer about '\r'.
	crlf bool

	// perm is the permission bits SaveFile writes with. LoadFile sets it
	// from the file actually on disk so an executable script, for
	// instance, keeps its +x bit; a buffer with no backing file yet (New,
	// NewFromString) defaults to a plain 0o644 for its eventual Save As.
	perm os.FileMode
}

const defaultFilePerm = 0o644

// New returns an empty buffer.
func New() *GapBuffer {
	return &GapBuffer{
		data:   make([]rune, defaultGapSize),
		gapEnd: defaultGapSize,
		perm:   defaultFilePerm,
	}
}

// NewFromString returns a buffer pre-populated with s, cursor at offset 0.
// The buffer starts out not dirty: populating it is construction, not a
// user edit, even though it's built via InsertString internally.
func NewFromString(s string) *GapBuffer {
	gb := New()
	gb.InsertString(s)
	gb.MoveCursorTo(0)
	gb.dirty = false
	return gb
}

// LoadFile reads path and returns a buffer containing its contents. CRLF
// line endings are normalized to LF and remembered, so SaveFile writes the
// file back with its original line-ending convention, and so does the
// file's permission mode (an executable script keeps its +x bit).
func LoadFile(path string) (*GapBuffer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	perm := os.FileMode(defaultFilePerm)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	text := string(raw)
	crlf := strings.Contains(text, "\r\n")
	if crlf {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	gb := NewFromString(text)
	gb.crlf = crlf
	gb.perm = perm
	return gb, nil
}

// SaveFile writes the buffer's current contents to path, truncating it,
// restoring CRLF line endings first if the file originally had them, and
// using the permission bits LoadFile captured (or 0o644 by default).
func (gb *GapBuffer) SaveFile(path string) error {
	gb.mu.Lock()
	text := gb.stringLocked()
	crlf := gb.crlf
	perm := gb.perm
	gb.dirty = false
	gb.mu.Unlock()
	if crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return os.WriteFile(path, []byte(text), perm)
}

// Dirty reports whether the buffer has unsaved changes.
func (gb *GapBuffer) Dirty() bool {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.dirty
}

// Len returns the number of runes of actual text (excluding the gap).
func (gb *GapBuffer) Len() int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return len(gb.data) - (gb.gapEnd - gb.gapStart)
}

// String returns the full text of the buffer.
func (gb *GapBuffer) String() string {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.stringLocked()
}

func (gb *GapBuffer) stringLocked() string {
	var sb strings.Builder
	sb.Grow(len(gb.data) - (gb.gapEnd - gb.gapStart))
	sb.WriteString(string(gb.data[:gb.gapStart]))
	sb.WriteString(string(gb.data[gb.gapEnd:]))
	return sb.String()
}

// Lines splits the current text on '\n'. It always returns at least one
// element, even for an empty buffer.
func (gb *GapBuffer) Lines() []string {
	return strings.Split(gb.String(), "\n")
}

// LineCount returns the number of lines in the buffer.
func (gb *GapBuffer) LineCount() int {
	return len(gb.Lines())
}

// CursorOffset returns the current cursor position as a rune offset into
// the logical text.
func (gb *GapBuffer) CursorOffset() int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.gapStart
}

// CursorLineCol returns the cursor's (line, column), both zero-based.
func (gb *GapBuffer) CursorLineCol() (line, col int) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	for _, r := range gb.data[:gb.gapStart] {
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

// MoveCursorTo relocates the gap so the cursor sits at the given logical
// rune offset, clamped to [0, Len()].
func (gb *GapBuffer) MoveCursorTo(offset int) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	gb.moveGapTo(gb.clamp(offset))
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// InsertRune inserts a single rune at the cursor and advances the cursor
// past it.
func (gb *GapBuffer) InsertRune(r rune) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	gb.insertRuneLocked(r)
	gb.dirty = true
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// InsertString inserts s at the cursor, advancing the cursor past it.
func (gb *GapBuffer) InsertString(s string) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	for _, r := range s {
		gb.insertRuneLocked(r)
	}
	gb.dirty = true
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

func (gb *GapBuffer) insertRuneLocked(r rune) {
	if gb.gapStart == gb.gapEnd {
		gb.growGap(1)
	}
	gb.data[gb.gapStart] = r
	gb.gapStart++
}

// DeleteBackward removes the rune immediately before the cursor (Backspace
// semantics). It reports whether a rune was removed.
func (gb *GapBuffer) DeleteBackward() bool {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if gb.gapStart == 0 {
		return false
	}
	gb.gapStart--
	gb.dirty = true
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
	return true
}

// DeleteForward removes the rune immediately after the cursor (Delete key
// semantics). It reports whether a rune was removed.
func (gb *GapBuffer) DeleteForward() bool {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if gb.gapEnd == len(gb.data) {
		return false
	}
	gb.gapEnd++
	gb.dirty = true
	return true
}

// MoveLeft moves the cursor one rune left, if possible.
func (gb *GapBuffer) MoveLeft() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if gb.gapStart > 0 {
		gb.moveGapTo(gb.gapStart - 1)
	}
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// MoveRight moves the cursor one rune right, if possible.
func (gb *GapBuffer) MoveRight() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	logicalLen := len(gb.data) - (gb.gapEnd - gb.gapStart)
	if gb.gapStart < logicalLen {
		gb.moveGapTo(gb.gapStart + 1)
	}
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// MoveUp moves the cursor to the equivalent (desired) column on the
// previous line, if any.
func (gb *GapBuffer) MoveUp() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	line, _ := gb.lineColLocked(gb.gapStart)
	if line == 0 {
		return
	}
	gb.moveGapTo(gb.offsetForLineColLocked(line-1, gb.desiredCol))
}

// MoveDown moves the cursor to the equivalent (desired) column on the next
// line, if any.
func (gb *GapBuffer) MoveDown() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	line, _ := gb.lineColLocked(gb.gapStart)
	lines := strings.Split(gb.stringLocked(), "\n")
	if line >= len(lines)-1 {
		return
	}
	gb.moveGapTo(gb.offsetForLineColLocked(line+1, gb.desiredCol))
}

// MoveHome moves the cursor to the start of the current line.
func (gb *GapBuffer) MoveHome() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	line, _ := gb.lineColLocked(gb.gapStart)
	gb.moveGapTo(gb.offsetForLineColLocked(line, 0))
	gb.desiredCol = 0
}

// MoveEnd moves the cursor to the end of the current line.
func (gb *GapBuffer) MoveEnd() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	line, _ := gb.lineColLocked(gb.gapStart)
	lines := strings.Split(gb.stringLocked(), "\n")
	gb.moveGapTo(gb.offsetForLineColLocked(line, len([]rune(lines[line]))))
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// MoveWordRight moves the cursor past the rest of the current word (if the
// cursor is inside one) and any following whitespace, landing at the start
// of the next word (or end of buffer).
func (gb *GapBuffer) MoveWordRight() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	runes := []rune(gb.stringLocked())
	pos := gb.gapStart
	n := len(runes)
	for pos < n && !unicode.IsSpace(runes[pos]) {
		pos++
	}
	for pos < n && unicode.IsSpace(runes[pos]) {
		pos++
	}
	gb.moveGapTo(pos)
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// MoveWordLeft moves the cursor back over any whitespace immediately
// before it and then over the word before that, landing at that word's
// start (or offset 0).
func (gb *GapBuffer) MoveWordLeft() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	runes := []rune(gb.stringLocked())
	pos := gb.gapStart
	for pos > 0 && unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	gb.moveGapTo(pos)
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// LineOffset returns the starting logical offset of the given zero-based
// line, clamped to the buffer's actual line range.
func (gb *GapBuffer) LineOffset(line int) int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.offsetForLineColLocked(line, 0)
}

// Restore replaces the entire contents of the buffer with text and places
// the cursor at offset. Used by undo/redo.
func (gb *GapBuffer) Restore(text string, offset int) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	runes := []rune(text)
	gb.data = make([]rune, len(runes)+defaultGapSize)
	copy(gb.data, runes)
	gb.gapStart = len(runes)
	gb.gapEnd = len(gb.data)
	gb.moveGapTo(gb.clamp(offset))
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
	gb.hasSel = false
}

// OffsetLineCol converts a logical rune offset (clamped to the buffer's
// bounds) to its zero-based (line, col).
func (gb *GapBuffer) OffsetLineCol(offset int) (line, col int) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.lineColLocked(gb.clamp(offset))
}

// TextRange returns the logical text within [start, end), clamped to the
// buffer's bounds. It returns "" if the range is empty after clamping.
func (gb *GapBuffer) TextRange(start, end int) string {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	start, end = gb.clamp(start), gb.clamp(end)
	if start >= end {
		return ""
	}
	runes := []rune(gb.stringLocked())
	return string(runes[start:end])
}

// DeleteRange removes the text within [start, end) (clamped to the
// buffer's bounds), leaves the cursor at start, and returns the removed
// text. It's a no-op returning "" if the range is empty after clamping.
//
// Deletion is O(1) relative to the buffer's total size once the gap has
// been relocated to end: everything in [start, end) simply becomes part of
// the gap, the same trick DeleteBackward/DeleteForward use at a single
// boundary.
func (gb *GapBuffer) DeleteRange(start, end int) string {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	start, end = gb.clamp(start), gb.clamp(end)
	if start >= end {
		return ""
	}
	runes := []rune(gb.stringLocked())
	deleted := string(runes[start:end])
	gb.moveGapTo(end)
	gb.gapStart = start
	gb.dirty = true
	gb.hasSel = false
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
	return deleted
}

// CurrentLineRange returns the offset bounds of the line containing the
// cursor. end includes the line's trailing newline when one exists, so
// DeleteRange(start, end) removes the whole line cleanly, including the
// line break; the buffer's last line (which has no trailing newline) gets
// end == Len().
func (gb *GapBuffer) CurrentLineRange() (start, end int) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	line, _ := gb.lineColLocked(gb.gapStart)
	lines := strings.Split(gb.stringLocked(), "\n")
	for i := 0; i < line; i++ {
		start += len([]rune(lines[i])) + 1
	}
	end = start + len([]rune(lines[line]))
	if line < len(lines)-1 {
		end++ // include the trailing newline
	}
	return start, end
}

// StartSelection anchors a selection at the current cursor position, if one
// isn't already active. Safe to call on every extend-selection keystroke.
func (gb *GapBuffer) StartSelection() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if !gb.hasSel {
		gb.selAnchor = gb.gapStart
		gb.hasSel = true
	}
}

// ClearSelection drops any active selection without moving the cursor.
func (gb *GapBuffer) ClearSelection() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	gb.hasSel = false
}

// HasSelection reports whether a non-empty selection is active.
func (gb *GapBuffer) HasSelection() bool {
	_, _, ok := gb.Selection()
	return ok
}

// Selection returns the active selection's [start, end) range, ordered
// regardless of which end the cursor sits at, and whether a non-empty
// selection exists (an anchor at the cursor's own position counts as no
// selection).
func (gb *GapBuffer) Selection() (start, end int, ok bool) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	if !gb.hasSel || gb.selAnchor == gb.gapStart {
		return 0, 0, false
	}
	if gb.selAnchor < gb.gapStart {
		return gb.selAnchor, gb.gapStart, true
	}
	return gb.gapStart, gb.selAnchor, true
}

// SelectRange sets the active selection directly to [start, end) (clamped,
// order-independent) and moves the cursor to the higher end. Used by
// search to highlight a match rather than requiring the caller to walk
// there one Shift+arrow at a time.
func (gb *GapBuffer) SelectRange(start, end int) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	start, end = gb.clamp(start), gb.clamp(end)
	if start > end {
		start, end = end, start
	}
	gb.moveGapTo(end)
	gb.selAnchor = start
	gb.hasSel = true
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// SelectedText returns the text within the active selection, or "" if none.
func (gb *GapBuffer) SelectedText() string {
	start, end, ok := gb.Selection()
	if !ok {
		return ""
	}
	return gb.TextRange(start, end)
}

// DeleteSelection removes the active selection's text and returns it. It's
// a no-op returning "" if no selection is active, so callers can invoke it
// unconditionally before an edit to implement "typing replaces the
// selection".
func (gb *GapBuffer) DeleteSelection() string {
	start, end, ok := gb.Selection()
	if !ok {
		return ""
	}
	return gb.DeleteRange(start, end)
}

func (gb *GapBuffer) clamp(offset int) int {
	logicalLen := len(gb.data) - (gb.gapEnd - gb.gapStart)
	if offset < 0 {
		return 0
	}
	if offset > logicalLen {
		return logicalLen
	}
	return offset
}

// lineColLocked computes (line, col) for a logical offset without needing
// the gap moved there first; it counts newlines directly against the raw
// buffer, treating indices within the gap as belonging to gapStart.
func (gb *GapBuffer) lineColLocked(offset int) (line, col int) {
	seen := 0
	scan := func(r rune) bool {
		if seen == offset {
			return false
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		seen++
		return true
	}
	for _, r := range gb.data[:gb.gapStart] {
		if !scan(r) {
			return
		}
	}
	for _, r := range gb.data[gb.gapEnd:] {
		if !scan(r) {
			return
		}
	}
	return
}

// offsetForLineColLocked converts a (line, col) pair to a logical offset,
// clamping col to the target line's length.
func (gb *GapBuffer) offsetForLineColLocked(line, col int) int {
	lines := strings.Split(gb.stringLocked(), "\n")
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	target := []rune(lines[line])
	if col > len(target) {
		col = len(target)
	}
	if col < 0 {
		col = 0
	}
	offset := 0
	for i := 0; i < line; i++ {
		offset += len([]rune(lines[i])) + 1 // +1 for the newline
	}
	return offset + col
}

// growGap enlarges the gap so it can hold at least minExtra more runes.
// growGap enlarges the gap so it can hold at least minExtra more runes.
// Growth is geometric (at least doubling the buffer's total size), the
// same amortized-O(1) trick append() uses. A fixed-size increment here
// would make bulk insertion (loading a large file, a large paste, which
// both go through InsertString one rune at a time) degrade into O(n^2):
// every defaultGapSize runes would re-copy the entire buffer built so far.
func (gb *GapBuffer) growGap(minExtra int) {
	existingGap := gb.gapEnd - gb.gapStart
	needed := minExtra - existingGap + defaultGapSize
	if needed < defaultGapSize {
		needed = defaultGapSize
	}
	growth := len(gb.data)
	if growth < needed {
		growth = needed
	}
	newLen := len(gb.data) + growth
	newData := make([]rune, newLen)
	copy(newData, gb.data[:gb.gapStart])
	tail := gb.data[gb.gapEnd:]
	copy(newData[newLen-len(tail):], tail)
	gb.gapEnd = newLen - len(tail)
	gb.data = newData
}

// moveGapTo relocates the gap so that gapStart == pos (a logical offset).
func (gb *GapBuffer) moveGapTo(pos int) {
	switch {
	case pos < gb.gapStart:
		shift := gb.gapStart - pos
		copy(gb.data[gb.gapEnd-shift:gb.gapEnd], gb.data[pos:gb.gapStart])
		gb.gapStart = pos
		gb.gapEnd -= shift
	case pos > gb.gapStart:
		shift := pos - gb.gapStart
		copy(gb.data[gb.gapStart:gb.gapStart+shift], gb.data[gb.gapEnd:gb.gapEnd+shift])
		gb.gapStart += shift
		gb.gapEnd += shift
	}
}
