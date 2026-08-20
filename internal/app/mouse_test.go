package app

import (
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// screenColForCol returns the absolute screen column a click on the
// given buffer column (0-based) would need to land on, deriving the
// gutter width the same way pkg/display does rather than hardcoding it,
// so this test doesn't silently drift if that formula ever changes.
func screenColForCol(m *Model, col int) int {
	ox, _ := m.editorOrigin()
	gutterW := len(strconv.Itoa(m.buf.LineCount())) + 2
	return ox + gutterW + col
}

func press(m *Model, col, line int) tea.MouseMsg {
	_, oy := m.editorOrigin()
	return tea.MouseMsg{X: screenColForCol(m, col), Y: oy + line, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func motion(m *Model, col, line int) tea.MouseMsg {
	_, oy := m.editorOrigin()
	return tea.MouseMsg{X: screenColForCol(m, col), Y: oy + line, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft}
}

func release(m *Model) tea.MouseMsg {
	return tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft}
}

func TestClickMovesCursorToClickedPosition(t *testing.T) {
	m := newTestModel(t, "aaa\nbbb\nccc")
	m.Update(press(m, 1, 2)) // line 2, col 1

	line, col := m.buf.CursorLineCol()
	if line != 2 || col != 1 {
		t.Fatalf("cursor at (%d,%d), want (2,1)", line, col)
	}
}

func TestClickAndDragExtendsSelection(t *testing.T) {
	m := newTestModel(t, "hello world")
	m.Update(press(m, 0, 0))
	m.Update(motion(m, 5, 0))

	if got := m.buf.SelectedText(); got != "hello" {
		t.Fatalf("SelectedText() = %q, want %q", got, "hello")
	}

	m.Update(release(m))
	m.Update(motion(m, 8, 0)) // motion after release shouldn't extend anything further
	if got := m.buf.SelectedText(); got != "hello" {
		t.Fatalf("SelectedText() after release = %q, want unchanged %q", got, "hello")
	}
}

func TestClickAccountsForTabBarRowWhenMultipleTabsOpen(t *testing.T) {
	m := newTestModel(t, "aaa\nbbb\nccc")
	m.newTab()       // a second tab means renderTop() now sits one row below a tab bar
	m.switchToTab(0) // back to the tab with real content to click into

	sidebarW := m.exp.Width
	gutterW := len(strconv.Itoa(m.buf.LineCount())) + 2
	// Absolute screen row 1 is the tab bar's row plus the editor's own
	// row 0 (file line 0): with the tab bar accounted for, this click
	// should land on line 0, not line 1.
	msg := tea.MouseMsg{X: sidebarW + 1 + gutterW, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m.Update(msg)

	line, _ := m.buf.CursorLineCol()
	if line != 0 {
		t.Fatalf("cursor line = %d, want 0 (editorOrigin should offset past the tab bar row)", line)
	}
}

func TestWheelDownMovesCursorDownSeveralLines(t *testing.T) {
	m := newTestModel(t, "0\n1\n2\n3\n4\n5\n6\n7\n8\n9")
	m.buf.MoveCursorTo(0)

	m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})

	line, _ := m.buf.CursorLineCol()
	if line != mouseWheelLines {
		t.Fatalf("cursor line = %d, want %d", line, mouseWheelLines)
	}
}

func TestWheelUpMovesCursorUpAndClampsAtTop(t *testing.T) {
	m := newTestModel(t, "0\n1\n2\n3\n4\n5\n6\n7\n8\n9")
	m.buf.MoveCursorTo(m.buf.LineOffset(1)) // line 1

	m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})

	line, _ := m.buf.CursorLineCol()
	if line != 0 {
		t.Fatalf("cursor line = %d, want 0 (clamped at the top, even though wheel moves %d lines)", line, mouseWheelLines)
	}
}

func TestWheelScrollIgnoredOutsideEditorFocus(t *testing.T) {
	m := newTestModel(t, "0\n1\n2\n3\n4\n5")
	sendKeys(m, key(tea.KeyCtrlB), key(tea.KeyCtrlB)) // focus the explorer
	before := m.buf.CursorOffset()

	m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})

	if got := m.buf.CursorOffset(); got != before {
		t.Fatalf("cursor moved to %d from a wheel event while the explorer had focus, want unchanged %d", got, before)
	}
}

func TestClickOutsideEditorFocusIsIgnored(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyCtrlB), key(tea.KeyCtrlB)) // focus the explorer
	before := m.buf.CursorOffset()

	m.Update(press(m, 2, 0))

	if got := m.buf.CursorOffset(); got != before {
		t.Fatalf("cursor moved to %d from a click while the explorer had focus, want unchanged %d", got, before)
	}
}
