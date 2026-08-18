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
}

// New returns an empty buffer.
func New() *GapBuffer {
	return &GapBuffer{
		data:   make([]rune, defaultGapSize),
		gapEnd: defaultGapSize,
	}
}

// NewFromString returns a buffer pre-populated with s, cursor at offset 0.
func NewFromString(s string) *GapBuffer {
	gb := New()
	gb.InsertString(s)
	gb.MoveCursorTo(0)
	return gb
}

// LoadFile reads path and returns a buffer containing its contents.
func LoadFile(path string) (*GapBuffer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewFromString(string(raw)), nil
}

// SaveFile writes the buffer's current contents to path, truncating it.
func (gb *GapBuffer) SaveFile(path string) error {
	gb.mu.Lock()
	text := gb.stringLocked()
	gb.dirty = false
	gb.mu.Unlock()
	return os.WriteFile(path, []byte(text), 0o644)
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
func (gb *GapBuffer) growGap(minExtra int) {
	growth := (gb.gapEnd - gb.gapStart) + minExtra + defaultGapSize
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
