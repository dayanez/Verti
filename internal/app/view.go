package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hinshun/vt10x"

	"github.com/dommcpro/verti/pkg/highlight"
	"github.com/dommcpro/verti/pkg/paint"
)

func (m *Model) View() string {
	if m.quitting || m.width <= 0 || m.height <= 0 {
		return ""
	}
	var sections []string
	if bar := m.renderTabBar(); bar != "" {
		sections = append(sections, bar)
	}
	sections = append(sections, m.renderTop())
	if m.termVisible {
		sections = append(sections, m.renderTerminalPane())
	}
	sections = append(sections, m.renderStatusBar())
	return strings.Join(sections, "\n")
}

// renderTabBar draws one row listing every open tab, with the active one
// bracketed, or "" if there's only one tab (the common case looks exactly
// as it did before tabs existed).
func (m *Model) renderTabBar() string {
	if len(m.tabs) <= 1 {
		return ""
	}
	parts := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		filename, dirty := t.filename, t.buf.Dirty()
		if i == m.activeTab {
			filename, dirty = m.filename, m.buf.Dirty()
		}
		name := "[untitled]"
		if filename != "" {
			name = filepath.Base(filename)
		}
		if dirty {
			name += " *"
		}
		if i == m.activeTab {
			parts[i] = "[" + name + "]"
		} else {
			parts[i] = " " + name + " "
		}
	}
	bar := strings.Join(parts, "|")
	return lipgloss.NewStyle().Reverse(true).Width(m.width).Render(padOrTruncate(bar, m.width))
}

// currentHighlighter returns the active tab's own highlighter instance
// (constructed once when that tab was opened, in tabs.go), not a fresh
// one: reusing the same instance across renders is what lets it do
// incremental reparsing instead of reparsing the whole file every
// keystroke.
func (m *Model) currentHighlighter() highlight.Highlighter {
	return m.tabs[m.activeTab].highlighter
}

func (m *Model) renderTop() string {
	if m.helpVisible {
		return m.renderHelp()
	}
	var editorContent string
	if m.paintOverlay.Active {
		editorContent = m.renderPaintCanvas()
	} else {
		editorContent = m.editorView.Render(m.buf, m.currentHighlighter(), m.focus == FocusEditor)
	}
	if !m.sidebarVisible {
		return editorContent
	}
	var sidebarContent string
	switch {
	case m.prompt == promptQuickOpen:
		sidebarContent = m.renderQuickOpenResults()
	case m.searchResultsActive:
		sidebarContent = m.renderSearchResults()
	default:
		sidebarContent = m.exp.Render(m.focus == FocusExplorer)
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Height(m.exp.Height).Render(strings.Repeat("│\n", m.exp.Height-1) + "│")
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, sep, editorContent)
}

// renderScrollableList draws a title row followed by one row per item (of
// count total, each rendered by rowText), the selected one reverse-styled,
// scrolled to keep it visible, and padded to fill h rows. Shared by
// renderSearchResults and renderQuickOpenResults, which differ only in
// what a "result" is and how one is described.
func renderScrollableList(w, h int, title string, count, selected int, rowText func(i int) string) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	rows := make([]string, 0, h)
	rows = append(rows, lipgloss.NewStyle().Bold(true).Render(padOrTruncate(title, w)))

	visibleRows := h - 1
	scrollTop := 0
	if selected >= visibleRows {
		scrollTop = selected - visibleRows + 1
	}
	end := scrollTop + visibleRows
	if end > count {
		end = count
	}
	for i := scrollTop; i < end; i++ {
		label := padOrTruncate(rowText(i), w)
		if i == selected {
			rows = append(rows, lipgloss.NewStyle().Reverse(true).Render(label))
		} else {
			rows = append(rows, label)
		}
	}
	for len(rows) < h {
		rows = append(rows, strings.Repeat(" ", w))
	}
	return strings.Join(rows, "\n")
}

// renderSearchResults draws the find-in-files results list in the sidebar
// area, mirroring the file explorer's own layout.
func (m *Model) renderSearchResults() string {
	title := fmt.Sprintf("%d result(s)", len(m.searchResults))
	return renderScrollableList(m.exp.Width, m.exp.Height, title, len(m.searchResults), m.searchSelected, func(i int) string {
		r := m.searchResults[i]
		return fmt.Sprintf("%s:%d: %s", r.Path, r.Line, r.Text)
	})
}

// renderQuickOpenResults draws the quick-open fuzzy-filtered file list in
// the sidebar area, the same layout renderSearchResults uses.
func (m *Model) renderQuickOpenResults() string {
	title := fmt.Sprintf("%d file(s)", len(m.quickOpenResults))
	if m.quickOpenFiles == nil {
		title = "loading..."
	}
	return renderScrollableList(m.exp.Width, m.exp.Height, title, len(m.quickOpenResults), m.quickOpenSelected, func(i int) string {
		return m.quickOpenResults[i]
	})
}

// renderHelp draws the keybinding overlay: helpLines' content, scrolled
// by helpScroll, filling the same width x height the editor+sidebar
// normally occupy.
func (m *Model) renderHelp() string {
	w, h := m.width, m.editorView.Height
	if w <= 0 || h <= 0 {
		return ""
	}
	lines := m.helpLines()
	end := m.helpScroll + h
	if end > len(lines) {
		end = len(lines)
	}
	rows := make([]string, 0, h)
	for i := m.helpScroll; i < end; i++ {
		style := lipgloss.NewStyle()
		// Section headers have a single leading space; every content line
		// (keybinding entries, nav descriptions) has two, so that's
		// enough to tell them apart without a separate marker.
		if strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(lines[i], "  ") {
			style = style.Bold(true)
		}
		rows = append(rows, style.Render(padOrTruncate(lines[i], w)))
	}
	for len(rows) < h {
		rows = append(rows, strings.Repeat(" ", w))
	}
	return strings.Join(rows, "\n")
}

// renderPaintCanvas draws the in-progress paint canvas as a standalone
// drawing surface the size of the editor pane. Paint mode replaces the
// editor's live view rather than alpha-blending onto styled text. See
// README for why (compositing plain glyphs onto ANSI-styled text safely
// would need a real cell-grid renderer, out of scope for v1).
func (m *Model) renderPaintCanvas() string {
	w, h := m.editorView.Width, m.editorView.Height
	if w < 1 || h < 1 {
		return ""
	}
	resolved := m.paintOverlay.Canvas.Resolve()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			if r, ok := resolved[paint.Point{X: x, Y: y}]; ok {
				sb.WriteRune(r)
			} else {
				sb.WriteRune(' ')
			}
		}
		rows[y] = sb.String()
	}
	rows[0] = padOrTruncate(" PAINT: drag to draw, Enter to insert as a comment, c to clear, Esc to cancel ", w)
	return strings.Join(rows, "\n")
}

func (m *Model) renderTerminalPane() string {
	if m.termWidth < 1 || m.termHeight < 1 {
		return ""
	}
	title := lipgloss.NewStyle().Reverse(true).Render(padOrTruncate(" shell (Ctrl+` to hide) ", m.termWidth))

	contentHeight := m.termHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	var lines []string
	if vt := m.term.Screen(); vt != nil && vtInAltScreen(vt) {
		// A full-screen program (vim, top, less, ...) has switched to the
		// alternate screen buffer: render its real character grid instead
		// of scrolling raw bytes, which is what actually lets it redraw
		// correctly instead of showing garbled escape-sequence soup.
		lines = m.renderTerminalCellGrid(vt, m.termWidth, contentHeight)
	} else {
		text := ansi.Strip(string(m.termOutput))
		lines = strings.Split(text, "\n")
		if len(lines) > contentHeight {
			lines = lines[len(lines)-contentHeight:]
		}
		for len(lines) < contentHeight {
			lines = append(lines, "")
		}
		for i, l := range lines {
			lines[i] = padOrTruncate(l, m.termWidth)
		}
	}

	rows := make([]string, 0, m.termHeight)
	rows = append(rows, title)
	rows = append(rows, lines...)
	return strings.Join(rows, "\n")
}

func vtInAltScreen(vt vt10x.Terminal) bool {
	vt.Lock()
	defer vt.Unlock()
	return vt.Mode()&vt10x.ModeAltScreen != 0
}

// renderTerminalCellGrid draws the virtual terminal's current screen as a
// real width x height character grid with colors and cursor position.
// vt10x's public API doesn't expose bold/italic/underline/reverse-video
// (those bits exist internally but aren't exported), so those specific
// accents are lost; the layout, text, and 256-color/truecolor palette are
// not.
func (m *Model) renderTerminalCellGrid(vt vt10x.Terminal, width, height int) []string {
	vt.Lock()
	defer vt.Unlock()

	cols, rows := vt.Size()
	cx, cy := -1, -1
	if vt.CursorVisible() {
		cur := vt.Cursor()
		cx, cy = cur.X, cur.Y
	}

	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var sb strings.Builder
		for x := 0; x < width; x++ {
			ch := " "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			if x < cols && y < rows {
				g := vt.Cell(x, y)
				if g.Char != 0 {
					ch = string(g.Char)
				}
				style = lipgloss.NewStyle().Foreground(vtColor(g.FG, lipgloss.Color("252")))
				if g.BG != vt10x.DefaultBG {
					style = style.Background(vtColor(g.BG, ""))
				}
			}
			if x == cx && y == cy {
				style = style.Reverse(true)
			}
			sb.WriteString(style.Render(ch))
		}
		lines[y] = sb.String()
	}
	return lines
}

// vtColor maps a vt10x.Color (an ANSI/256-color palette index, a packed
// 24-bit truecolor value, or one of the Default* sentinels) to the
// lipgloss.Color that renders the same thing.
func vtColor(c vt10x.Color, fallback lipgloss.Color) lipgloss.Color {
	switch {
	case c == vt10x.DefaultFG || c == vt10x.DefaultBG || c == vt10x.DefaultCursor:
		return fallback
	case c < 256:
		return lipgloss.Color(strconv.Itoa(int(c)))
	default:
		return lipgloss.Color(fmt.Sprintf("#%06x", uint32(c)))
	}
}

func (m *Model) renderStatusBar() string {
	focusLabel := "EDITOR"
	switch {
	case m.helpVisible:
		focusLabel = "HELP"
	case m.paintOverlay.Active:
		focusLabel = "PAINT"
	case m.focus == FocusExplorer:
		focusLabel = "EXPLORER"
	case m.focus == FocusTerminal:
		focusLabel = "TERMINAL"
	}

	name := m.filename
	if name == "" {
		name = "[untitled]"
	}
	dirty := ""
	if m.buf.Dirty() {
		dirty = " ●"
	}
	line, col := m.buf.CursorLineCol()

	left := fmt.Sprintf(" %s%s | %s", name, dirty, m.status)
	if label, ok := promptLabels[m.prompt]; ok {
		left = fmt.Sprintf(" %s%s_", label, m.promptText)
	}
	branch := ""
	if b := m.exp.Branch(); b != "" {
		branch = b + "  "
	}
	right := fmt.Sprintf("%s%s  Ln %d, Col %d ", branch, focusLabel, line+1, col+1)

	pad := m.width - runeLen(left) - runeLen(right)
	if pad < 1 {
		left = padOrTruncate(left, m.width-runeLen(right)-1)
		pad = 1
	}
	bar := left + strings.Repeat(" ", pad) + right
	return lipgloss.NewStyle().Reverse(true).Width(m.width).Render(padOrTruncate(bar, m.width))
}

func runeLen(s string) int { return len([]rune(s)) }

func padOrTruncate(s string, w int) string {
	r := []rune(s)
	if w < 0 {
		w = 0
	}
	if len(r) > w {
		return string(r[:w])
	}
	if len(r) < w {
		return s + strings.Repeat(" ", w-len(r))
	}
	return s
}
