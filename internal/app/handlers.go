package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/dommcpro/verti/pkg/paint"
)

// osClipboard abstracts the system clipboard so tests can substitute a
// fake instead of touching the real one, which would be flaky in a
// headless CI environment and rude to clobber on a developer's own
// machine every time the test suite runs.
type osClipboard interface {
	ReadAll() (string, error)
	WriteAll(text string) error
}

type realOSClipboard struct{}

func (realOSClipboard) ReadAll() (string, error)   { return clipboard.ReadAll() }
func (realOSClipboard) WriteAll(text string) error { return clipboard.WriteAll(text) }

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	chord := msg.String()

	if m.paintOverlay.Active {
		return m.handlePaintKey(msg, chord)
	}

	if m.prompt != promptNone {
		return m.handlePromptKey(msg, chord)
	}

	// When the subshell pane has focus, almost every keystroke, including
	// chords like Ctrl+C or Ctrl+S that are global elsewhere, must reach
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
	case FocusSearchResults:
		return m.handleSearchResultsKey(msg)
	default:
		return m.handleEditorKey(msg)
	}
}

func (m *Model) dispatchGlobal(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "toggle_sidebar":
		m.sidebarVisible = !m.sidebarVisible
		if m.sidebarVisible {
			if m.searchResultsActive {
				m.focus = FocusSearchResults
			} else {
				m.focus = FocusExplorer
			}
		} else if m.focus == FocusExplorer || m.focus == FocusSearchResults {
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
			m.prompt = promptSaveAs
			m.promptText = ""
			m.focus = FocusEditor
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

	case "copy":
		m.copySelectionOrLine()
		return m, nil

	case "cut":
		m.cutSelectionOrLine()
		return m, nil

	case "paste":
		m.pasteClipboard()
		return m, nil

	case "find":
		m.prompt = promptFind
		m.promptText = ""
		m.focus = FocusEditor
		return m, nil

	case "goto_line":
		m.prompt = promptGoto
		m.promptText = ""
		m.focus = FocusEditor
		return m, nil

	case "duplicate_line":
		m.duplicateLine()
		return m, nil

	case "delete_line":
		m.deleteLine()
		return m, nil

	case "replace":
		m.prompt = promptReplaceFind
		m.promptText = ""
		m.focus = FocusEditor
		return m, nil

	case "save_as":
		m.prompt = promptSaveAs
		m.promptText = m.filename
		m.focus = FocusEditor
		return m, nil

	case "toggle_comment":
		m.toggleComment()
		return m, nil

	case "new_tab":
		m.newTab()
		return m, nil

	case "close_tab":
		m.closeActiveTab()
		return m, nil

	case "next_tab":
		m.nextTab()
		return m, nil

	case "prev_tab":
		m.prevTab()
		return m, nil

	case "find_in_files":
		m.prompt = promptFindInFiles
		m.promptText = ""
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
		m.buf.DeleteSelection() // no-op if nothing selected; makes typing replace a selection
		m.buf.InsertString(string(msg.Runes))
	case tea.KeySpace:
		m.recordUndoBoundary(editInsert)
		m.buf.DeleteSelection()
		m.buf.InsertRune(' ')
	case tea.KeyEnter:
		m.recordUndoBoundary(editInsert)
		m.buf.DeleteSelection()
		m.buf.InsertString("\n" + m.currentLineIndent())
	case tea.KeyTab:
		if m.selectionSpansMultipleLines() {
			m.indentSelection()
		} else {
			m.recordUndoBoundary(editInsert)
			m.buf.DeleteSelection()
			m.buf.InsertString(strings.Repeat(" ", tabWidthOrDefault(m.cfg.TabWidth)))
		}
	case tea.KeyShiftTab:
		if m.selectionSpansMultipleLines() {
			m.outdentSelection()
		} else {
			m.outdentCurrentLine()
		}
	case tea.KeyBackspace:
		m.recordUndoBoundary(editDelete)
		if !m.buf.HasSelection() {
			m.buf.DeleteBackward()
		} else {
			m.buf.DeleteSelection()
		}
	case tea.KeyDelete:
		m.recordUndoBoundary(editDelete)
		if !m.buf.HasSelection() {
			m.buf.DeleteForward()
		} else {
			m.buf.DeleteSelection()
		}
	case tea.KeyUp:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveUp()
	case tea.KeyDown:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveDown()
	case tea.KeyLeft:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveLeft()
	case tea.KeyRight:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveRight()
	case tea.KeyCtrlLeft:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveWordLeft()
	case tea.KeyCtrlRight:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveWordRight()
	case tea.KeyCtrlShiftLeft:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveWordLeft()
	case tea.KeyCtrlShiftRight:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveWordRight()
	case tea.KeyHome:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveHome()
	case tea.KeyEnd:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		m.buf.MoveEnd()
	case tea.KeyPgUp:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		for i := 0; i < m.editorView.Height; i++ {
			m.buf.MoveUp()
		}
	case tea.KeyPgDown:
		m.buf.ClearSelection()
		m.lastEditKind = editNone
		for i := 0; i < m.editorView.Height; i++ {
			m.buf.MoveDown()
		}
	case tea.KeyShiftUp:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveUp()
	case tea.KeyShiftDown:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveDown()
	case tea.KeyShiftLeft:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveLeft()
	case tea.KeyShiftRight:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveRight()
	case tea.KeyShiftHome:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveHome()
	case tea.KeyShiftEnd:
		m.buf.StartSelection()
		m.lastEditKind = editNone
		m.buf.MoveEnd()
	}
	return m, nil
}

// copySelectionOrLine copies the active selection to the clipboard, or the
// current line (micro/VS Code's "nothing selected" fallback) if none.
func (m *Model) copySelectionOrLine() {
	text := m.buf.SelectedText()
	if text == "" {
		start, end := m.buf.CurrentLineRange()
		text = m.buf.TextRange(start, end)
	}
	if text == "" {
		return
	}
	m.setClipboard(text)
	m.status = "copied"
}

// cutSelectionOrLine cuts the active selection, or the current line if none.
func (m *Model) cutSelectionOrLine() {
	text := m.buf.SelectedText()
	if text != "" {
		m.recordUndoBoundary(editDelete)
		m.buf.DeleteSelection()
	} else {
		start, end := m.buf.CurrentLineRange()
		text = m.buf.TextRange(start, end)
		if text == "" {
			return
		}
		m.recordUndoBoundary(editDelete)
		m.buf.DeleteRange(start, end)
	}
	m.setClipboard(text)
	m.status = "cut"
}

// pasteClipboard inserts the OS clipboard's contents at the cursor if
// it's readable and non-empty, falling back to Verti's own clipboard
// register otherwise (e.g. no clipboard backend on a headless Linux box,
// or an SSH session where the native clipboard API reaches the wrong
// machine). Replaces the active selection if any.
func (m *Model) pasteClipboard() {
	text := m.clipboard
	if osText, err := m.osClipboard.ReadAll(); err == nil && osText != "" {
		text = osText
	}
	if text == "" {
		return
	}
	m.recordUndoBoundary(editInsert)
	m.buf.DeleteSelection()
	m.buf.InsertString(text)
	m.clipboard = text // keep the internal register in sync with what actually got pasted
	m.status = "pasted"
}

// setClipboard stores text in Verti's own clipboard register (paste's
// fallback) and best-effort mirrors it two ways: the real OS clipboard
// (via osClipboard, reliable on a local desktop session) and an OSC52
// escape sequence (which modern terminal emulators intercept, reaching
// the clipboard over SSH or tmux too, cases the native API can't reach
// since it would act on the remote machine's clipboard instead). Both
// are best-effort: a headless environment with neither available just
// leaves paste using the internal register.
func (m *Model) setClipboard(text string) {
	m.clipboard = text
	_ = m.osClipboard.WriteAll(text)
	termenv.Copy(text)
}

// handlePromptKey handles input while a status-bar prompt (find, go-to-line,
// or the unsaved-changes confirm) owns the keyboard, intercepted before any
// other key routing.
func (m *Model) handlePromptKey(msg tea.KeyMsg, chord string) (tea.Model, tea.Cmd) {
	if m.prompt == promptConfirmDiscard {
		switch chord {
		case "y", "Y":
			m.reallyCloseActiveTab()
		case "n", "N", "esc":
			m.status = "close canceled"
		}
		m.pendingCloseTab = false
		m.prompt = promptNone
		return m, nil
	}

	switch {
	case chord == "esc":
		m.prompt = promptNone
		m.promptText = ""
		m.replaceFindTerm = ""
	case msg.Type == tea.KeyEnter:
		switch m.prompt {
		case promptFind:
			m.findNext(m.promptText)
		case promptGoto:
			m.gotoLine(m.promptText)
			m.prompt = promptNone
			m.promptText = ""
		case promptReplaceFind:
			if m.promptText != "" {
				m.replaceFindTerm = m.promptText
				m.promptText = ""
				m.prompt = promptReplaceWith
			}
		case promptReplaceWith:
			m.replaceAll(m.replaceFindTerm, m.promptText)
			m.prompt = promptNone
			m.promptText = ""
			m.replaceFindTerm = ""
		case promptSaveAs:
			m.saveAs(m.promptText)
			m.prompt = promptNone
			m.promptText = ""
		case promptFindInFiles:
			m.runFileSearch(m.promptText)
			m.prompt = promptNone
			m.promptText = ""
		}
	case msg.Type == tea.KeyBackspace:
		if r := []rune(m.promptText); len(r) > 0 {
			m.promptText = string(r[:len(r)-1])
		}
	case msg.Type == tea.KeySpace:
		m.promptText += " "
	case msg.Type == tea.KeyRunes:
		m.promptText += string(msg.Runes)
	}
	return m, nil
}

// findNext selects the next occurrence of query after the cursor, wrapping
// around to the start of the buffer if nothing is found before the end.
// Called on every Enter while the find prompt is open, so repeating Enter
// steps through matches.
func (m *Model) findNext(query string) {
	if query == "" {
		return
	}
	text := []rune(m.buf.String())
	needle := []rune(query)
	if idx := indexRunes(text, needle, m.buf.CursorOffset()); idx >= 0 {
		m.buf.SelectRange(idx, idx+len(needle))
		m.status = fmt.Sprintf("found %q", query)
		return
	}
	if idx := indexRunes(text, needle, 0); idx >= 0 {
		m.buf.SelectRange(idx, idx+len(needle))
		m.status = fmt.Sprintf("found %q (wrapped)", query)
		return
	}
	m.status = fmt.Sprintf("not found: %q", query)
}

// indexRunes returns the offset of needle's first occurrence in haystack at
// or after from, or -1 if there isn't one.
func indexRunes(haystack, needle []rune, from int) int {
	if from < 0 {
		from = 0
	}
	if len(needle) == 0 || from > len(haystack)-len(needle) {
		return -1
	}
outer:
	for i := from; i <= len(haystack)-len(needle); i++ {
		for j, r := range needle {
			if haystack[i+j] != r {
				continue outer
			}
		}
		return i
	}
	return -1
}

// gotoLine moves the cursor to the start of the 1-based line number in
// input, clamped to the buffer's line range.
func (m *Model) gotoLine(input string) {
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		m.status = "invalid line: " + input
		return
	}
	m.buf.ClearSelection()
	m.buf.MoveCursorTo(m.buf.LineOffset(n - 1))
	line, _ := m.buf.CursorLineCol()
	m.status = fmt.Sprintf("line %d", line+1)
}

// duplicateLine inserts a copy of the line under the cursor directly below
// it, leaving the cursor on the new copy at the same column.
func (m *Model) duplicateLine() {
	start, end := m.buf.CurrentLineRange()
	lineText := m.buf.TextRange(start, end)
	if lineText == "" {
		return
	}
	_, col := m.buf.CursorLineCol()

	insert := lineText
	target := end + col
	if !strings.HasSuffix(lineText, "\n") {
		insert = "\n" + lineText
		target = end + 1 + col
	}

	m.recordUndoBoundary(editInsert)
	m.buf.MoveCursorTo(end)
	m.buf.InsertString(insert)
	m.buf.MoveCursorTo(target)
	m.lastEditKind = editNone
}

// deleteLine removes the line under the cursor entirely, without touching
// the clipboard: a quick way to drop a line without clobbering whatever
// was last copied or cut.
func (m *Model) deleteLine() {
	start, end := m.buf.CurrentLineRange()
	if m.buf.TextRange(start, end) == "" {
		return
	}
	m.recordUndoBoundary(editDelete)
	m.buf.DeleteRange(start, end)
	m.lastEditKind = editNone
}

// currentLineIndent returns the leading spaces/tabs of the line under the
// cursor, used to carry indentation forward onto a new line on Enter.
func (m *Model) currentLineIndent() string {
	start, end := m.buf.CurrentLineRange()
	line := strings.TrimSuffix(m.buf.TextRange(start, end), "\n")
	return line[:indentLen(line)]
}

// indentLen returns the byte length of s's leading run of spaces and tabs.
// Since that run is always pure ASCII, this doubles as a safe rune offset
// into s.
func indentLen(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// lineEndOffset returns the offset just past the given zero-based line's
// content, including its trailing newline for every line but the last
// (mirroring CurrentLineRange's convention for the cursor's own line).
func (m *Model) lineEndOffset(line int) int {
	if line+1 < m.buf.LineCount() {
		return m.buf.LineOffset(line + 1)
	}
	return m.buf.Len()
}

// selectionSpansMultipleLines reports whether the active selection covers
// more than one line, the condition under which Tab/Shift+Tab indent or
// outdent the whole block instead of acting on a single line.
func (m *Model) selectionSpansMultipleLines() bool {
	start, end, ok := m.buf.Selection()
	if !ok {
		return false
	}
	startLine, _ := m.buf.OffsetLineCol(start)
	endLine, _ := m.buf.OffsetLineCol(end)
	return endLine > startLine
}

// selectedLineRange returns the (inclusive) line indices a multi-line
// selection touches. A selection that ends exactly at column 0 of a line
// doesn't count that line as touched -- matching how most editors treat a
// selection that merely reaches the start of the next line.
func (m *Model) selectedLineRange() (startLine, endLine int) {
	start, end, _ := m.buf.Selection()
	startLine, _ = m.buf.OffsetLineCol(start)
	var endCol int
	endLine, endCol = m.buf.OffsetLineCol(end)
	if endCol == 0 && endLine > startLine {
		endLine--
	}
	return startLine, endLine
}

// indentSelection adds one tab-width of leading spaces to every line the
// active multi-line selection touches, then re-selects the (now wider)
// block so repeated Tab presses keep indenting further.
func (m *Model) indentSelection() {
	startLine, endLine := m.selectedLineRange()
	tab := strings.Repeat(" ", tabWidthOrDefault(m.cfg.TabWidth))

	m.recordUndoBoundary(editInsert)
	for line := startLine; line <= endLine; line++ {
		m.buf.MoveCursorTo(m.buf.LineOffset(line))
		m.buf.InsertString(tab)
	}
	m.buf.SelectRange(m.buf.LineOffset(startLine), m.lineEndOffset(endLine))
	m.lastEditKind = editNone
}

// outdentSelection removes up to one tab-width of leading spaces from
// every line the active multi-line selection touches.
func (m *Model) outdentSelection() {
	startLine, endLine := m.selectedLineRange()
	tabWidth := tabWidthOrDefault(m.cfg.TabWidth)

	m.recordUndoBoundary(editDelete)
	for line := startLine; line <= endLine; line++ {
		lineStart := m.buf.LineOffset(line)
		n := leadingSpaces(m.buf.TextRange(lineStart, m.lineEndOffset(line)), tabWidth)
		if n > 0 {
			m.buf.DeleteRange(lineStart, lineStart+n)
		}
	}
	m.buf.SelectRange(m.buf.LineOffset(startLine), m.lineEndOffset(endLine))
	m.lastEditKind = editNone
}

// outdentCurrentLine removes up to one tab-width of leading spaces from the
// line under the cursor -- Shift+Tab's behavior when nothing is selected.
func (m *Model) outdentCurrentLine() {
	start, end := m.buf.CurrentLineRange()
	n := leadingSpaces(m.buf.TextRange(start, end), tabWidthOrDefault(m.cfg.TabWidth))
	if n == 0 {
		return
	}
	m.recordUndoBoundary(editDelete)
	m.buf.DeleteRange(start, start+n)
	m.lastEditKind = editNone
}

// leadingSpaces counts the leading ' ' runes in s, up to max.
func leadingSpaces(s string, max int) int {
	r := []rune(s)
	n := 0
	for n < len(r) && n < max && r[n] == ' ' {
		n++
	}
	return n
}

// replaceAll replaces every occurrence of find with replacement across the
// whole buffer as a single undo step.
func (m *Model) replaceAll(find, replacement string) {
	if find == "" {
		return
	}
	text := m.buf.String()
	count := strings.Count(text, find)
	if count == 0 {
		m.status = fmt.Sprintf("not found: %q", find)
		return
	}
	m.recordUndoBoundary(editInsert)
	m.buf.Restore(strings.ReplaceAll(text, find, replacement), 0)
	m.lastEditKind = editNone
	m.status = fmt.Sprintf("replaced %d occurrence(s) of %q", count, find)
}

// saveAs saves the buffer to path (resolved to an absolute path) and makes
// it the buffer's filename, so a subsequent plain Ctrl+S writes there too.
func (m *Model) saveAs(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.status = "save canceled: no filename"
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	if err := m.buf.SaveFile(abs); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	m.filename = abs
	m.status = "saved " + abs
}

// commentLineRange returns the (inclusive) line range Ctrl+/ should act
// on: the active selection's lines if there is one, otherwise just the
// cursor's line.
func (m *Model) commentLineRange() (startLine, endLine int) {
	if m.buf.HasSelection() {
		return m.selectedLineRange()
	}
	line, _ := m.buf.CursorLineCol()
	return line, line
}

// lineContent returns the given zero-based line's text, excluding its
// trailing newline.
func (m *Model) lineContent(line int) string {
	return strings.TrimSuffix(m.buf.TextRange(m.buf.LineOffset(line), m.lineEndOffset(line)), "\n")
}

// toggleComment comments or uncomments every line Ctrl+/ targets, using
// the current file's line-comment syntax (VS Code's behavior) if it has
// one, or wrapping the range in a single block comment otherwise (CSS,
// HTML, Markdown). Blank lines in the range are left alone either way.
func (m *Model) toggleComment() {
	style := paint.StyleForFile(m.displayFilename())
	if style.Line == "" {
		m.toggleBlockComment(style)
		return
	}

	hadSelection := m.buf.HasSelection()
	startLine, endLine := m.commentLineRange()
	prefix := style.Line

	allCommented, any := true, false
	for line := startLine; line <= endLine; line++ {
		trimmed := strings.TrimLeft(m.lineContent(line), " \t")
		if trimmed == "" {
			continue
		}
		any = true
		if !strings.HasPrefix(trimmed, prefix) {
			allCommented = false
		}
	}
	if !any {
		return
	}

	m.recordUndoBoundary(editInsert)
	for line := startLine; line <= endLine; line++ {
		content := m.lineContent(line)
		if strings.TrimSpace(content) == "" {
			continue
		}
		lineStart := m.buf.LineOffset(line)
		indent := indentLen(content)
		if allCommented {
			stripLen := len(prefix)
			if len(content) > indent+len(prefix) && content[indent+len(prefix)] == ' ' {
				stripLen++
			}
			m.buf.DeleteRange(lineStart+indent, lineStart+indent+stripLen)
		} else {
			m.buf.MoveCursorTo(lineStart + indent)
			m.buf.InsertString(prefix + " ")
		}
	}
	m.lastEditKind = editNone
	if hadSelection {
		m.buf.SelectRange(m.buf.LineOffset(startLine), m.lineEndOffset(endLine))
	}
}

// toggleBlockComment handles languages with no line-comment form: wraps
// the active selection (or the whole current line, if none) in a single
// block comment, or unwraps it if it's already wrapped.
func (m *Model) toggleBlockComment(style paint.CommentStyle) {
	if style.BlockStart == "" {
		return
	}
	hadSelection := m.buf.HasSelection()
	var start, end int
	if hadSelection {
		start, end, _ = m.buf.Selection()
	} else {
		lineStart, lineEnd := m.buf.CurrentLineRange()
		content := strings.TrimSuffix(m.buf.TextRange(lineStart, lineEnd), "\n")
		start, end = lineStart, lineStart+len([]rune(content))
	}

	text := m.buf.TextRange(start, end)
	trimmed := strings.TrimSpace(text)

	m.recordUndoBoundary(editInsert)
	m.buf.DeleteRange(start, end)
	var replacement string
	if strings.HasPrefix(trimmed, style.BlockStart) && strings.HasSuffix(trimmed, style.BlockEnd) {
		inner := strings.TrimPrefix(trimmed, style.BlockStart)
		inner = strings.TrimSuffix(inner, style.BlockEnd)
		replacement = strings.TrimSpace(inner)
	} else {
		replacement = style.BlockStart + " " + text + " " + style.BlockEnd
	}
	m.buf.MoveCursorTo(start)
	m.buf.InsertString(replacement)

	m.lastEditKind = editNone
	if hadSelection {
		m.buf.SelectRange(start, start+len([]rune(replacement)))
	}
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
