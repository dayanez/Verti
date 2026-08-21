package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestEscInTerminalPaneLeavesFocusWhenNotInAltScreen(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal")
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}
	defer m.term.Close()
	m.focus = FocusTerminal

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus != FocusEditor {
		t.Fatalf("focus = %v after Esc with no full-screen program running, want FocusEditor", m.focus)
	}
}

func TestEscInTerminalPaneIsForwardedDuringAltScreen(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal")
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}
	defer m.term.Close()
	m.focus = FocusTerminal

	// Simulate a full-screen program (vim, top, ...) having entered the
	// alternate screen buffer.
	m.Update(termOutputMsg{data: []byte("\x1b[?1049h")})

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.focus != FocusTerminal {
		t.Fatalf("focus = %v after Esc during alt-screen, want FocusTerminal: Esc must reach the full-screen program instead of yanking focus away from it", m.focus)
	}
}

func TestKeyToPTYBytesForwardsCtrlChordsAndEsc(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
		want string
	}{
		{"ctrl+a", tea.KeyMsg{Type: tea.KeyCtrlA}, "\x01"},
		{"ctrl+e", tea.KeyMsg{Type: tea.KeyCtrlE}, "\x05"},
		{"ctrl+u", tea.KeyMsg{Type: tea.KeyCtrlU}, "\x15"},
		{"ctrl+k", tea.KeyMsg{Type: tea.KeyCtrlK}, "\x0b"},
		{"ctrl+w", tea.KeyMsg{Type: tea.KeyCtrlW}, "\x17"},
		{"ctrl+l", tea.KeyMsg{Type: tea.KeyCtrlL}, "\x0c"},
		{"ctrl+z", tea.KeyMsg{Type: tea.KeyCtrlZ}, "\x1a"},
		{"ctrl+c (already handled)", tea.KeyMsg{Type: tea.KeyCtrlC}, "\x03"},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}, "\x1b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := keyToPTYBytes(c.msg); got != c.want {
				t.Errorf("keyToPTYBytes(%s) = %q, want %q: this chord must reach the shell as raw input instead of being silently dropped", c.name, got, c.want)
			}
		})
	}
}

func TestTermClosedMsgClosesPtyAndAllowsReopen(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal")
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}

	m.Update(termClosedMsg{})
	if m.term.Running() {
		t.Fatal("term.Running() = true after termClosedMsg, want false: the pty must be released so the next Ctrl+J can start a fresh subshell instead of finding a stuck one")
	}
	if m.termVisible {
		t.Fatal("termVisible = true after termClosedMsg, want false")
	}

	m.dispatchGlobal("toggle_terminal") // reopen
	if !m.term.Running() {
		t.Fatal("term.Running() = false after reopening the terminal pane, want true: a fresh subshell should have started")
	}
	m.term.Close()
}

func TestHidingTerminalPaneRelayouts(t *testing.T) {
	m := newTestModel(t, "hello")
	m.dispatchGlobal("toggle_terminal") // show
	if !m.term.Running() {
		t.Skip("PTY unavailable in this environment")
	}
	if m.termHeight == 0 {
		t.Fatal("termHeight = 0 after showing the terminal pane, want > 0")
	}

	m.dispatchGlobal("toggle_terminal") // hide
	if m.termHeight != 0 {
		t.Fatalf("termHeight = %d after hiding the terminal pane, want 0: layout() must run again so the editor reclaims that space", m.termHeight)
	}
	m.term.Close()
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
