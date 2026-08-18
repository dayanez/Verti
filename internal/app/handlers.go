package app

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	chord := msg.String()

	if m.paintOverlay.Active {
		return m.handlePaintKey(msg, chord)
	}

	// When the subshell pane has focus, almost every keystroke — including
	// chords like Ctrl+C or Ctrl+S that are global elsewhere — must reach
	// the shell as raw input (Ctrl+C needs to interrupt a running command,
	// not quit the editor). Only Esc and the terminal-toggle chord itself
	// escape back out; everything else is forwarded.
	if m.focus == FocusTerminal {
		if chord == "esc" {
			m.focus = FocusEditor
			return m, nil
		}
		if chord == globalToggleTerminalChord {
			return m.dispatchGlobal("toggle_terminal")
		}
		return m.handleTerminalKey(msg)
	}

	if cmdName, ok := m.keymap[chord]; ok {
		return m.dispatchGlobal(cmdName)
	}
	switch m.focus {
	case FocusExplorer:
		return m.handleExplorerKey(msg)
	default:
		return m.handleEditorKey(msg)
	}
}

func (m *Model) dispatchGlobal(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "toggle_sidebar":
		m.sidebarVisible = !m.sidebarVisible
		if m.sidebarVisible {
			m.focus = FocusExplorer
		} else if m.focus == FocusExplorer {
			m.focus = FocusEditor
		}
		m.layout()
		return m, nil

	case "toggle_terminal":
		m.termVisible = !m.termVisible
		if !m.termVisible {
			if m.focus == FocusTerminal {
				m.focus = FocusEditor
			}
			return m, nil
		}
		m.layout()
		var cmd tea.Cmd
		if !m.term.Running() {
			if err := m.term.Start(m.termWidth, m.termHeight); err != nil {
				m.status = "terminal error: " + err.Error()
				m.termVisible = false
				return m, nil
			}
			cmd = readTermCmd(m.term)
		}
		m.focus = FocusTerminal
		return m, cmd

	case "toggle_paint":
		m.paintOverlay.Toggle()
		return m, nil

	case "save":
		if m.filename == "" {
			m.status = "nothing to save: open a file first"
			return m, nil
		}
		if err := m.buf.SaveFile(m.filename); err != nil {
			m.status = "save failed: " + err.Error()
		} else {
			m.status = "saved " + m.filename
		}
		return m, nil

	case "undo":
		m.undo()
		return m, nil

	case "redo":
		m.redo()
		return m, nil

	case "quit":
		m.quitting = true
		_ = m.term.Close()
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) handleEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		m.recordUndoBoundary(editInsert)
		m.buf.InsertString(string(msg.Runes))
	case tea.KeySpace:
		m.recordUndoBoundary(editInsert)
		m.buf.InsertRune(' ')
	case tea.KeyEnter:
		m.recordUndoBoundary(editInsert)
		m.buf.InsertRune('\n')
	case tea.KeyTab:
		m.recordUndoBoundary(editInsert)
		m.buf.InsertString(strings.Repeat(" ", tabWidthOrDefault(m.cfg.TabWidth)))
	case tea.KeyBackspace:
		m.recordUndoBoundary(editDelete)
		m.buf.DeleteBackward()
	case tea.KeyDelete:
		m.recordUndoBoundary(editDelete)
		m.buf.DeleteForward()
	case tea.KeyUp:
		m.lastEditKind = editNone
		m.buf.MoveUp()
	case tea.KeyDown:
		m.lastEditKind = editNone
		m.buf.MoveDown()
	case tea.KeyLeft:
		m.lastEditKind = editNone
		m.buf.MoveLeft()
	case tea.KeyRight:
		m.lastEditKind = editNone
		m.buf.MoveRight()
	case tea.KeyHome:
		m.lastEditKind = editNone
		m.buf.MoveHome()
	case tea.KeyEnd:
		m.lastEditKind = editNone
		m.buf.MoveEnd()
	case tea.KeyPgUp:
		m.lastEditKind = editNone
		for i := 0; i < m.editorView.Height; i++ {
			m.buf.MoveUp()
		}
	case tea.KeyPgDown:
		m.lastEditKind = editNone
		for i := 0; i < m.editorView.Height; i++ {
			m.buf.MoveDown()
		}
	}
	return m, nil
}

func tabWidthOrDefault(w int) int {
	if w <= 0 {
		return 4
	}
	return w
}

func (m *Model) handleExplorerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.exp.MoveUp()
	case tea.KeyDown:
		m.exp.MoveDown()
	case tea.KeyRight:
		_ = m.exp.ExpandOrDescend()
	case tea.KeyLeft:
		m.exp.CollapseOrAscend()
	case tea.KeyEnter:
		path, err := m.exp.HandleEnter()
		if err != nil {
			m.status = "explorer error: " + err.Error()
			return m, nil
		}
		if path != "" {
			m.openFile(path)
		}
	case tea.KeyEsc:
		m.focus = FocusEditor
	}
	return m, nil
}

func (m *Model) handleTerminalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.term.Running() {
		return m, nil
	}
	if b := keyToPTYBytes(msg); b != "" {
		_, _ = m.term.Write([]byte(b))
	}
	return m, nil
}

// keyToPTYBytes converts a bubbletea key event into the byte sequence a
// real terminal would have sent, so the subshell sees ordinary input.
func keyToPTYBytes(msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyRunes:
		return string(msg.Runes)
	case tea.KeySpace:
		return " "
	case tea.KeyEnter:
		return "\r"
	case tea.KeyTab:
		return "\t"
	case tea.KeyBackspace:
		return "\x7f"
	case tea.KeyUp:
		return "\x1b[A"
	case tea.KeyDown:
		return "\x1b[B"
	case tea.KeyRight:
		return "\x1b[C"
	case tea.KeyLeft:
		return "\x1b[D"
	case tea.KeyCtrlC:
		return "\x03"
	case tea.KeyCtrlD:
		return "\x04"
	}
	return ""
}

func (m *Model) handlePaintKey(msg tea.KeyMsg, chord string) (tea.Model, tea.Cmd) {
	switch {
	case chord == "esc":
		m.paintOverlay.Reset()
		m.paintOverlay.Active = false
	case chord == "ctrl+p":
		m.paintOverlay.Toggle()
	case msg.Type == tea.KeyEnter:
		if !m.paintOverlay.Canvas.Empty() {
			comment := m.paintOverlay.Finish(m.displayFilename())
			m.recordUndoBoundary(editInsert)
			m.buf.InsertString(comment + "\n")
		}
		m.paintOverlay.Active = false
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'c':
		m.paintOverlay.Reset()
	}
	return m, nil
}

// editorOrigin returns the screen column/row where the editor pane's
// content begins, so absolute mouse coordinates can be translated into
// paint-canvas-local coordinates.
func (m *Model) editorOrigin() (x, y int) {
	if m.sidebarVisible {
		return m.exp.Width + 1, 0
	}
	return 0, 0
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !m.paintOverlay.Active {
		return m, nil
	}
	ox, oy := m.editorOrigin()
	x, y := msg.X-ox, msg.Y-oy
	inBounds := x >= 0 && y >= 0 && x < m.editorView.Width && y < m.editorView.Height

	switch msg.Action {
	case tea.MouseActionPress:
		if inBounds {
			m.paintOverlay.BeginDrag(x, y)
		}
	case tea.MouseActionMotion:
		if inBounds {
			m.paintOverlay.UpdateDrag(x, y)
		}
	case tea.MouseActionRelease:
		cx := clampInt(x, 0, m.editorView.Width-1)
		cy := clampInt(y, 0, m.editorView.Height-1)
		m.paintOverlay.EndDrag(cx, cy)
	}
	return m, nil
}

func (m *Model) displayFilename() string {
	if m.filename == "" {
		return "untitled.txt"
	}
	return filepath.Base(m.filename)
}
