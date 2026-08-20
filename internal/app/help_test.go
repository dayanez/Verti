package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestF1TogglesHelpVisible(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyF1))
	if !m.helpVisible {
		t.Fatal("helpVisible = false after F1, want true")
	}
	sendKeys(m, key(tea.KeyF1))
	if m.helpVisible {
		t.Fatal("helpVisible = true after 2nd F1, want false")
	}
}

func TestEscClosesHelp(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyF1))
	sendKeys(m, key(tea.KeyEsc))
	if m.helpVisible {
		t.Fatal("helpVisible = true after Esc, want false")
	}
}

func TestHelpOwnsInputWhileVisible(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyF1))
	sendKeys(m, key(tea.KeyCtrlB)) // would normally toggle the sidebar
	if !m.sidebarVisible {
		t.Fatal("Ctrl+B while help is open should not have reached the sidebar toggle")
	}
	if !m.helpVisible {
		t.Fatal("help should still be visible: Ctrl+B isn't F1 or Esc")
	}
}

func TestHelpListsLiveKeymapEntries(t *testing.T) {
	m := newTestModel(t, "hello")
	lines := m.helpLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ctrl+b") || !strings.Contains(joined, "Toggle sidebar") {
		t.Fatalf("helpLines() = %v, want an entry for ctrl+b -> Toggle sidebar", lines)
	}
	if !strings.Contains(joined, "ctrl+e") || !strings.Contains(joined, "Quick open") {
		t.Fatalf("helpLines() = %v, want an entry for ctrl+e -> Quick open", lines)
	}
}

func TestHelpScrollClampsAtBounds(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeys(m, key(tea.KeyF1))
	sendKeys(m, key(tea.KeyUp)) // already at 0; should stay clamped, not go negative
	if m.helpScroll != 0 {
		t.Fatalf("helpScroll = %d after Up at top, want 0", m.helpScroll)
	}

	maxScroll := len(m.helpLines()) - 1
	sendKeys(m, key(tea.KeyPgDown), key(tea.KeyPgDown), key(tea.KeyPgDown), key(tea.KeyPgDown))
	if m.helpScroll != maxScroll {
		t.Fatalf("helpScroll = %d after scrolling past the end, want clamped at %d", m.helpScroll, maxScroll)
	}
}

func TestDescribeAction(t *testing.T) {
	if got := describeAction("toggle_sidebar"); got != "Toggle sidebar" {
		t.Errorf("describeAction(%q) = %q, want %q", "toggle_sidebar", got, "Toggle sidebar")
	}
}
