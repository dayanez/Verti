package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/dommcpro/verti/pkg/highlight"
	"github.com/dommcpro/verti/pkg/paint"
)

func (m *Model) View() string {
	if m.quitting || m.width <= 0 || m.height <= 0 {
		return ""
	}
	sections := []string{m.renderTop()}
	if m.termVisible {
		sections = append(sections, m.renderTerminalPane())
	}
	sections = append(sections, m.renderStatusBar())
	return strings.Join(sections, "\n")
}

func (m *Model) currentHighlighter() highlight.Highlighter {
	h, ok := m.highlighter.For(m.displayFilename())
	if !ok {
		return nil
	}
	return h
}

func (m *Model) renderTop() string {
	var editorContent string
	if m.paintOverlay.Active {
		editorContent = m.renderPaintCanvas()
	} else {
		editorContent = m.editorView.Render(m.buf, m.currentHighlighter(), m.focus == FocusEditor)
	}
	if !m.sidebarVisible {
		return editorContent
	}
	sidebarContent := m.exp.Render(m.focus == FocusExplorer)
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Height(m.exp.Height).Render(strings.Repeat("│\n", m.exp.Height-1) + "│")
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarContent, sep, editorContent)
}

// renderPaintCanvas draws the in-progress paint canvas as a standalone
// drawing surface the size of the editor pane. Paint mode replaces the
// editor's live view rather than alpha-blending onto styled text — see
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
	rows[0] = padOrTruncate(" PAINT — drag to draw, Enter to insert as a comment, c to clear, Esc to cancel ", w)
	return strings.Join(rows, "\n")
}

func (m *Model) renderTerminalPane() string {
	if m.termWidth < 1 || m.termHeight < 1 {
		return ""
	}
	title := lipgloss.NewStyle().Reverse(true).Render(padOrTruncate(" shell — Ctrl+` to hide ", m.termWidth))

	contentHeight := m.termHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}
	text := ansi.Strip(string(m.termOutput))
	lines := strings.Split(text, "\n")
	if len(lines) > contentHeight {
		lines = lines[len(lines)-contentHeight:]
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}

	rows := make([]string, 0, m.termHeight)
	rows = append(rows, title)
	for _, l := range lines {
		rows = append(rows, padOrTruncate(l, m.termWidth))
	}
	return strings.Join(rows, "\n")
}

func (m *Model) renderStatusBar() string {
	focusLabel := "EDITOR"
	switch {
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

	left := fmt.Sprintf(" %s%s — %s", name, dirty, m.status)
	right := fmt.Sprintf("%s  Ln %d, Col %d ", focusLabel, line+1, col+1)

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
