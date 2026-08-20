package app

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// handleHelpKey drives the keybinding overlay while it's showing: F1 or
// Esc closes it, arrows/PgUp/PgDown scroll. It owns every keystroke while
// active (see handleKey), the same as paint mode, so nothing underneath
// reacts to a key the user only meant for browsing the list.
func (m *Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := len(m.helpLines()) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch msg.Type {
	case tea.KeyF1, tea.KeyEsc:
		m.helpVisible = false
	case tea.KeyUp:
		if m.helpScroll > 0 {
			m.helpScroll--
		}
	case tea.KeyDown:
		if m.helpScroll < maxScroll {
			m.helpScroll++
		}
	case tea.KeyPgUp:
		m.helpScroll -= m.editorView.Height
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	case tea.KeyPgDown:
		m.helpScroll += m.editorView.Height
		if m.helpScroll > maxScroll {
			m.helpScroll = maxScroll
		}
	}
	return m, nil
}

// helpLines returns the overlay's content, building and caching it on
// first use. m.keymap is fixed after startup (globalKeymap plus whatever
// init.lua rebound or added, merged once in New), so there's nothing to
// invalidate; without the cache this rebuilds the same sorted,
// formatted list from scratch on every scroll keypress and every render
// frame while the overlay is open.
func (m *Model) helpLines() []string {
	if m.helpLinesCache == nil {
		m.helpLinesCache = buildHelpLines(m.keymap)
	}
	return m.helpLinesCache
}

func buildHelpLines(keymap map[string]string) []string {
	lines := []string{
		" Keybindings  (F1 or Esc to close, ↑/↓ or PgUp/PgDown to scroll)",
		"",
	}
	chords := make([]string, 0, len(keymap))
	for c := range keymap {
		chords = append(chords, c)
	}
	sort.Strings(chords)
	for _, c := range chords {
		lines = append(lines, fmt.Sprintf("  %-14s %s", c, describeAction(keymap[c])))
	}
	lines = append(lines,
		"",
		" Navigation",
		"  Arrows, Home/End, PgUp/PgDown     Move the cursor",
		"  Shift + any of the above          Extend a selection",
		"  Ctrl+Left / Ctrl+Right            Jump by word",
		"  Tab / Shift+Tab                   Indent/outdent a selection, complete a word, or insert a tab",
		"  Enter (in the explorer)           Expand a folder; open a file on the second press",
		"  Esc                               Return focus to the editor, or close a prompt",
	)
	return lines
}

// describeAction turns a keymap command name ("toggle_sidebar") into a
// readable label ("Toggle sidebar").
func describeAction(name string) string {
	r := []rune(strings.ReplaceAll(name, "_", " "))
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}
