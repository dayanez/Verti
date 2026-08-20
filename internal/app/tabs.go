package app

import (
	"path/filepath"
	"time"

	"github.com/dayanez/Verti/pkg/buffer"
	"github.com/dayanez/Verti/pkg/highlight"
)

// openBuffer holds one open file's editing state. Switching tabs snapshots
// the Model's live buf/filename/undoStack/redoStack/lastEditKind/
// lastEditTime/scroll fields into the outgoing tab's openBuffer, then
// copies the incoming tab's openBuffer back into those same fields --
// they remain the single source of truth for whatever tab is on screen,
// so every existing m.buf/m.filename call site needs no changes at all.
//
// highlighter is not part of that mirroring: it's a stable reference
// constructed once when the tab is opened, so currentHighlighter (in
// view.go) reads it straight from the active tab rather than through the
// snapshot/restore dance. It must stay one instance per tab for its whole
// lifetime, since it retains incremental parse state keyed to that tab's
// own content; see the Highlighter doc comment in pkg/highlight.
type openBuffer struct {
	buf          *buffer.GapBuffer
	filename     string
	highlighter  highlight.Highlighter
	undoStack    []snapshot
	redoStack    []snapshot
	lastEditKind editKind
	lastEditTime time.Time
	scrollLine   int
	scrollCol    int
}

// highlighterFor resolves the highlighter for filename, falling back to
// "untitled.txt" for an empty filename (matching displayFilename, since an
// untitled buffer has no real extension to key off of), or nil if none is
// registered for it.
func highlighterFor(reg *highlight.Registry, filename string) highlight.Highlighter {
	name := "untitled.txt"
	if filename != "" {
		name = filepath.Base(filename)
	}
	h, ok := reg.For(name)
	if !ok {
		return nil
	}
	return h
}

// snapshotActiveTab copies the Model's live editing state into the active
// tab's saved slot.
func (m *Model) snapshotActiveTab() {
	t := m.tabs[m.activeTab]
	t.buf = m.buf
	t.filename = m.filename
	t.undoStack = m.undoStack
	t.redoStack = m.redoStack
	t.lastEditKind = m.lastEditKind
	t.lastEditTime = m.lastEditTime
	t.scrollLine = m.editorView.ScrollLine
	t.scrollCol = m.editorView.ScrollCol
}

// restoreTab makes tab i the active tab, copying its saved state back into
// the Model's live editing fields.
func (m *Model) restoreTab(i int) {
	t := m.tabs[i]
	m.buf = t.buf
	m.filename = t.filename
	m.undoStack = t.undoStack
	m.redoStack = t.redoStack
	m.lastEditKind = t.lastEditKind
	m.lastEditTime = t.lastEditTime
	m.editorView.ScrollLine = t.scrollLine
	m.editorView.ScrollCol = t.scrollCol
	m.activeTab = i
	m.focus = FocusEditor
}

// findTab returns the index of the open tab for path, or -1.
func (m *Model) findTab(path string) int {
	for i, t := range m.tabs {
		if i == m.activeTab {
			if m.filename == path {
				return i
			}
			continue
		}
		if t.filename == path {
			return i
		}
	}
	return -1
}

// switchToTab snapshots the currently active tab, then activates tab i.
func (m *Model) switchToTab(i int) {
	if i < 0 || i >= len(m.tabs) || i == m.activeTab {
		return
	}
	m.snapshotActiveTab()
	m.restoreTab(i)
}

func (m *Model) nextTab() {
	if len(m.tabs) < 2 {
		return
	}
	m.switchToTab((m.activeTab + 1) % len(m.tabs))
}

func (m *Model) prevTab() {
	if len(m.tabs) < 2 {
		return
	}
	m.switchToTab((m.activeTab - 1 + len(m.tabs)) % len(m.tabs))
}

// newTab opens a fresh blank, untitled tab.
func (m *Model) newTab() {
	m.snapshotActiveTab()
	m.tabs = append(m.tabs, &openBuffer{buf: buffer.New(), highlighter: highlighterFor(m.highlighter, "")})
	m.restoreTab(len(m.tabs) - 1)
	m.layout()
	m.status = "new file"
}

// openFile opens path in its own tab, switching to its existing tab
// instead of opening a duplicate if it's already open. Since opening a
// file always adds or switches to a tab rather than replacing the current
// one, there's no risk of silently discarding unsaved changes here (that
// risk now lives in closeActiveTab instead).
func (m *Model) openFile(path string) {
	if i := m.findTab(path); i >= 0 {
		m.switchToTab(i)
		m.status = "switched to " + path
		return
	}
	buf, err := buffer.LoadFile(path)
	if err != nil {
		m.status = "open failed: " + err.Error()
		return
	}
	m.snapshotActiveTab()
	m.tabs = append(m.tabs, &openBuffer{buf: buf, filename: path, highlighter: highlighterFor(m.highlighter, path)})
	m.restoreTab(len(m.tabs) - 1)
	m.layout()
	m.status = "opened " + path
}

// closeActiveTab closes the current tab, confirming first (via
// promptConfirmDiscard) if it has unsaved changes. Closing the last tab
// leaves a fresh blank one rather than exiting the app; Ctrl+Q quits.
func (m *Model) closeActiveTab() {
	if m.buf.Dirty() {
		m.prompt = promptConfirmDiscard
		m.pendingCloseTab = true
		m.status = "unsaved changes in " + m.displayFilename() + " -- close without saving? (y/n)"
		return
	}
	m.reallyCloseActiveTab()
}

func (m *Model) reallyCloseActiveTab() {
	i := m.activeTab
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if len(m.tabs) == 0 {
		m.tabs = append(m.tabs, &openBuffer{buf: buffer.New(), highlighter: highlighterFor(m.highlighter, "")})
	}
	if i >= len(m.tabs) {
		i = len(m.tabs) - 1
	}
	m.restoreTab(i)
	m.layout()
}
