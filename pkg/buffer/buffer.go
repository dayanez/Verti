// Package buffer implements a thread-safe gap buffer, the core text storage
// structure used by the editor view. A gap buffer keeps an unused region
// ("the gap") positioned at the cursor so that typing and deleting near the
// cursor are cheap (amortized O(1)); moving the cursor a long distance costs
// O(distance) to relocate the gap.
package buffer

import (
	"os"
	"sort"
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

	// lineStarts[i] is the logical rune offset of line i's first rune;
	// lineStarts[0] is always 0. It's maintained incrementally by every
	// edit (see insertIntoLineIndexLocked / deleteFromLineIndexLocked)
	// rather than recomputed from scratch, so that line/column lookups
	// (CursorLineCol, OffsetLineCol, MoveUp/Down/Home/End, ...) are
	// O(log n) or O(1) instead of re-scanning the whole buffer on every
	// keystroke -- the latter is invisible on a small file and a real
	// stutter on a large one, exactly the case a gap buffer exists to
	// handle well.
	lineStarts []int

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

	// version counts mutations, so stringLocked can cache its result and
	// only redo the O(n) rebuild when the content has actually changed
	// since the last call, instead of on every call -- String is on the
	// hot path once per render frame (the syntax highlighter needs the
	// full source text to parse accurately), so most calls happen between
	// edits, driven by cursor movement, scrolling, or a mouse event that
	// didn't touch the buffer at all.
	version    uint64
	strCache   string
	strCacheV  uint64
	strCacheOK bool
}

const defaultFilePerm = 0o644

// New returns an empty buffer.
func New() *GapBuffer {
	return &GapBuffer{
		data:       make([]rune, defaultGapSize),
		gapEnd:     defaultGapSize,
		lineStarts: []int{0},
		perm:       defaultFilePerm,
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
	savedVersion := gb.version
	gb.mu.Unlock()
	if crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if err := os.WriteFile(path, []byte(text), perm); err != nil {
		return err
	}
	gb.mu.Lock()
	if gb.version == savedVersion {
		gb.dirty = false
	}
	gb.mu.Unlock()
	return nil
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
	return gb.logicalLenLocked()
}

func (gb *GapBuffer) logicalLenLocked() int {
	return len(gb.data) - (gb.gapEnd - gb.gapStart)
}

// String returns the full text of the buffer. On a cache hit (nothing
// changed since the last call) this only takes a read lock; a miss
// upgrades to a write lock to safely populate the cache, since a bare
// RLock permits concurrent readers and must never see stringLocked mutate
// shared state.
func (gb *GapBuffer) String() string {
	gb.mu.RLock()
	if gb.strCacheOK && gb.strCacheV == gb.version {
		s := gb.strCache
		gb.mu.RUnlock()
		return s
	}
	gb.mu.RUnlock()

	gb.mu.Lock()
	defer gb.mu.Unlock()
	return gb.stringLocked()
}

// stringLocked requires the caller to hold gb.mu for writing: a cache
// miss populates gb.strCache, which a bare read lock isn't safe for.
func (gb *GapBuffer) stringLocked() string {
	if gb.strCacheOK && gb.strCacheV == gb.version {
		return gb.strCache
	}
	var sb strings.Builder
	sb.Grow(gb.logicalLenLocked())
	sb.WriteString(string(gb.data[:gb.gapStart]))
	sb.WriteString(string(gb.data[gb.gapEnd:]))
	s := sb.String()
	gb.strCache = s
	gb.strCacheV = gb.version
	gb.strCacheOK = true
	return s
}

// markDirtyLocked marks the buffer as having unsaved changes and bumps
// its version, invalidating stringLocked's cache. Every mutator calls
// this exactly once, instead of setting gb.dirty directly, so the two
// always stay in sync.
func (gb *GapBuffer) markDirtyLocked() {
	gb.dirty = true
	gb.version++
}

// runeAtLocked returns the rune at logical offset i (0 <= i < logical
// length), reading straight out of the underlying array around the gap
// rather than materializing any part of the buffer as a string or slice.
func (gb *GapBuffer) runeAtLocked(i int) rune {
	if i < gb.gapStart {
		return gb.data[i]
	}
	return gb.data[i+(gb.gapEnd-gb.gapStart)]
}

// substringLocked returns the text within the logical range [start, end)
// by reading directly out of the underlying array, so a caller that only
// needs a small slice of a huge buffer (a selection, a single line) never
// pays to materialize the rest of it.
func (gb *GapBuffer) substringLocked(start, end int) string {
	if start >= end {
		return ""
	}
	var sb strings.Builder
	sb.Grow(end - start)
	for i := start; i < end; i++ {
		sb.WriteRune(gb.runeAtLocked(i))
	}
	return sb.String()
}

// Lines splits the current text on '\n'. It always returns at least one
// element, even for an empty buffer.
func (gb *GapBuffer) Lines() []string {
	return strings.Split(gb.String(), "\n")
}

// LineCount returns the number of lines in the buffer, O(1) via the
// incrementally-maintained line index.
func (gb *GapBuffer) LineCount() int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return len(gb.lineStarts)
}

// LinesRange returns the text of lines [startLine, endLine), clamped to
// the buffer's actual range, without materializing any line outside it.
// Used by the renderer so a viewport onto a huge file only ever pays for
// the lines actually on screen.
func (gb *GapBuffer) LinesRange(startLine, endLine int) []string {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(gb.lineStarts) {
		endLine = len(gb.lineStarts)
	}
	if startLine >= endLine {
		return nil
	}
	out := make([]string, 0, endLine-startLine)
	for line := startLine; line < endLine; line++ {
		out = append(out, gb.substringLocked(gb.lineStarts[line], gb.lineEndLocked(line)))
	}
	return out
}

// lineEndLocked returns the offset just past the given line's content,
// excluding its trailing newline (or the buffer's logical end, for the
// last line).
func (gb *GapBuffer) lineEndLocked(line int) int {
	if line+1 < len(gb.lineStarts) {
		return gb.lineStarts[line+1] - 1
	}
	return gb.logicalLenLocked()
}

// lineIndexForOffsetLocked returns the line containing the given logical
// offset: the largest i such that lineStarts[i] <= offset.
func (gb *GapBuffer) lineIndexForOffsetLocked(offset int) int {
	i := sort.Search(len(gb.lineStarts), func(i int) bool { return gb.lineStarts[i] > offset })
	return i - 1
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
	return gb.lineColLocked(gb.gapStart)
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
	pos := gb.gapStart
	gb.insertRuneLocked(r)
	gb.markDirtyLocked()
	if r == '\n' {
		gb.insertIntoLineIndexLocked(pos, 1, []int{1})
	} else {
		gb.insertIntoLineIndexLocked(pos, 1, nil)
	}
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// InsertString inserts s at the cursor, advancing the cursor past it.
func (gb *GapBuffer) InsertString(s string) {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	pos := gb.gapStart
	var newlineOffsets []int
	n := 0
	for _, r := range s {
		gb.insertRuneLocked(r)
		if r == '\n' {
			newlineOffsets = append(newlineOffsets, n+1)
		}
		n++
	}
	gb.markDirtyLocked()
	gb.insertIntoLineIndexLocked(pos, n, newlineOffsets)
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

func (gb *GapBuffer) insertRuneLocked(r rune) {
	if gb.gapStart == gb.gapEnd {
		gb.growGap(1)
	}
	gb.data[gb.gapStart] = r
	gb.gapStart++
}

// insertIntoLineIndexLocked updates the line index after insertedLen runes
// were inserted at pos, newlineOffsets holding the offsets (relative to
// pos) of any newlines among them. Every line-start entry at or after pos
// shifts right by insertedLen; any newlines inserted add new entries.
func (gb *GapBuffer) insertIntoLineIndexLocked(pos, insertedLen int, newlineOffsets []int) {
	li := gb.lineIndexForOffsetLocked(pos)
	result := make([]int, 0, len(gb.lineStarts)+len(newlineOffsets))
	result = append(result, gb.lineStarts[:li+1]...)
	for _, rel := range newlineOffsets {
		result = append(result, pos+rel)
	}
	for _, v := range gb.lineStarts[li+1:] {
		result = append(result, v+insertedLen)
	}
	gb.lineStarts = result
}

// deleteFromLineIndexLocked updates the line index after the text in
// [start, end) (removedLen runes) was deleted: any line-start entries that
// fell inside the removed range are dropped, and every entry after it
// shifts left by removedLen.
func (gb *GapBuffer) deleteFromLineIndexLocked(start, end int) {
	removedLen := end - start
	li := gb.lineIndexForOffsetLocked(start)
	hi := li + 1
	for hi < len(gb.lineStarts) && gb.lineStarts[hi] <= end {
		hi++
	}
	result := make([]int, 0, li+1+len(gb.lineStarts)-hi)
	result = append(result, gb.lineStarts[:li+1]...)
	for _, v := range gb.lineStarts[hi:] {
		result = append(result, v-removedLen)
	}
	gb.lineStarts = result
}

// DeleteBackward removes the rune immediately before the cursor (Backspace
// semantics). It reports whether a rune was removed.
func (gb *GapBuffer) DeleteBackward() bool {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	if gb.gapStart == 0 {
		return false
	}
	start := gb.gapStart - 1
	gb.gapStart--
	gb.markDirtyLocked()
	gb.deleteFromLineIndexLocked(start, start+1)
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
	gb.markDirtyLocked()
	gb.deleteFromLineIndexLocked(gb.gapStart, gb.gapStart+1)
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
	if gb.gapStart < gb.logicalLenLocked() {
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
	if line >= len(gb.lineStarts)-1 {
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
	gb.moveGapTo(gb.offsetForLineColLocked(line, gb.lineEndLocked(line)-gb.lineStarts[line]))
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
}

// MoveWordRight moves the cursor past the rest of the current word (if the
// cursor is inside one) and any following whitespace, landing at the start
// of the next word (or end of buffer).
func (gb *GapBuffer) MoveWordRight() {
	gb.mu.Lock()
	defer gb.mu.Unlock()
	n := gb.logicalLenLocked()
	pos := gb.gapStart
	for pos < n && !unicode.IsSpace(gb.runeAtLocked(pos)) {
		pos++
	}
	for pos < n && unicode.IsSpace(gb.runeAtLocked(pos)) {
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
	pos := gb.gapStart
	for pos > 0 && unicode.IsSpace(gb.runeAtLocked(pos-1)) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(gb.runeAtLocked(pos-1)) {
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

// OffsetForLineCol converts a zero-based (line, col) pair to a logical
// rune offset, clamping both the line and the column within it to the
// buffer's actual bounds. Used by display.Editor.ScreenToOffset to turn
// a mouse click's screen row/column into a buffer position.
func (gb *GapBuffer) OffsetForLineCol(line, col int) int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return gb.offsetForLineColLocked(line, col)
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
	gb.rebuildLineIndexLocked(runes)
	gb.moveGapTo(gb.clamp(offset))
	_, gb.desiredCol = gb.lineColLocked(gb.gapStart)
	gb.hasSel = false
	gb.markDirtyLocked()
}

func (gb *GapBuffer) rebuildLineIndexLocked(runes []rune) {
	starts := []int{0}
	for i, r := range runes {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	gb.lineStarts = starts
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
	return gb.substringLocked(start, end)
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
	deleted := gb.substringLocked(start, end)
	gb.moveGapTo(end)
	gb.gapStart = start
	gb.markDirtyLocked()
	gb.hasSel = false
	gb.deleteFromLineIndexLocked(start, end)
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
	start = gb.lineStarts[line]
	if line+1 < len(gb.lineStarts) {
		end = gb.lineStarts[line+1]
	} else {
		end = gb.logicalLenLocked()
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
	logicalLen := gb.logicalLenLocked()
	if offset < 0 {
		return 0
	}
	if offset > logicalLen {
		return logicalLen
	}
	return offset
}

// lineColLocked computes (line, col) for a logical offset in O(log n) via
// a binary search over the maintained line-start index, rather than
// re-scanning the buffer from the start counting newlines.
func (gb *GapBuffer) lineColLocked(offset int) (line, col int) {
	line = gb.lineIndexForOffsetLocked(offset)
	col = offset - gb.lineStarts[line]
	return line, col
}

// offsetForLineColLocked converts a (line, col) pair to a logical offset
// in O(1) via the line-start index, clamping col to the target line's
// length.
func (gb *GapBuffer) offsetForLineColLocked(line, col int) int {
	if line < 0 {
		line = 0
	}
	if line >= len(gb.lineStarts) {
		line = len(gb.lineStarts) - 1
	}
	lineStart := gb.lineStarts[line]
	lineLen := gb.lineEndLocked(line) - lineStart
	if col > lineLen {
		col = lineLen
	}
	if col < 0 {
		col = 0
	}
	return lineStart + col
}

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
