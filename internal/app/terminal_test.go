package app

import (
	"strings"
	"testing"
)

func TestTerminalPaneRendersCellGridWhenAltScreenActive(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal")
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}
	defer m.term.Close()

	// The standard "enter alternate screen buffer" sequence a full-screen
	// program (vim, less, htop, ...) sends, followed by some content.
	// Going through Update (not term.Feed directly) exercises the actual
	// production path, which also updates the plain-text scrollback in
	// parallel.
	m.Update(termOutputMsg{data: []byte("\x1b[?1049h")})
	m.Update(termOutputMsg{data: []byte("XYZ")})

	out := m.renderTerminalPane()
	if !strings.Contains(out, "XYZ") {
		t.Fatalf("renderTerminalPane() output missing cell-grid content %q: %q", "XYZ", out)
	}
}

func TestTerminalPaneStaysPlainTextWithoutAltScreen(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal")
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}
	defer m.term.Close()

	m.Update(termOutputMsg{data: []byte("plain output\r\n")})
	out := m.renderTerminalPane()
	if !strings.Contains(out, "plain output") {
		t.Fatalf("renderTerminalPane() output missing plain-text content: %q", out)
	}
}
