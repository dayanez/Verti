package app

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dommcpro/verti/pkg/search"
)

// runFileSearch searches every file under the workspace root for query
// and shows the results in the sidebar area (replacing the file tree
// until Esc or a new search), moving focus there so up/down/Enter browse
// them immediately.
func (m *Model) runFileSearch(query string) {
	if query == "" {
		return
	}
	results, err := search.Files(m.exp.Root.Path, query)
	if err != nil {
		m.status = "search failed: " + err.Error()
		return
	}

	m.searchResults = results
	m.searchSelected = 0
	m.searchResultsActive = true
	m.sidebarVisible = true
	m.focus = FocusSearchResults
	m.layout()

	if len(results) == 0 {
		m.status = fmt.Sprintf("no matches for %q", query)
	} else {
		m.status = fmt.Sprintf("%d match(es) for %q", len(results), query)
	}
}

// handleSearchResultsKey drives the find-in-files results list: up/down
// to move the selection, Enter to jump to that match, Esc to go back to
// the file tree.
func (m *Model) handleSearchResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.searchSelected > 0 {
			m.searchSelected--
		}
	case tea.KeyDown:
		if m.searchSelected < len(m.searchResults)-1 {
			m.searchSelected++
		}
	case tea.KeyEnter:
		m.jumpToSearchResult(m.searchSelected)
	case tea.KeyEsc:
		m.searchResultsActive = false
		m.focus = FocusExplorer
	}
	return m, nil
}

// jumpToSearchResult opens the file at result i (in its own tab, per the
// normal openFile rules) and moves the cursor to the matching line,
// leaving the results list.
func (m *Model) jumpToSearchResult(i int) {
	if i < 0 || i >= len(m.searchResults) {
		return
	}
	r := m.searchResults[i]
	abs := filepath.Join(m.exp.Root.Path, r.Path)
	m.openFile(abs)
	m.buf.ClearSelection()
	m.buf.MoveCursorTo(m.buf.LineOffset(r.Line - 1))
	m.searchResultsActive = false
}
