package app

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dayanez/Verti/pkg/search"
)

// fileSearchResultMsg carries a background search's results back into
// Update once search.Files finishes walking the workspace.
type fileSearchResultMsg struct {
	query   string
	results []search.Match
	err     error
}

// runFileSearch kicks off a background search across the workspace for
// query and returns immediately. search.Files walks and reads every
// matching file under the root, which on a large tree can take real time,
// so it runs on its own goroutine via the returned tea.Cmd rather than
// blocking Update (and therefore every keystroke and redraw) until it's
// done.
func (m *Model) runFileSearch(query string) tea.Cmd {
	if query == "" {
		return nil
	}
	m.status = fmt.Sprintf("searching for %q...", query)
	root := m.exp.Root.Path
	return func() tea.Msg {
		results, err := search.Files(root, query)
		return fileSearchResultMsg{query: query, results: results, err: err}
	}
}

// applyFileSearchResult shows a finished background search's results in
// the sidebar area (replacing the file tree until Esc or a new search),
// moving focus there so up/down/Enter browse them immediately.
func (m *Model) applyFileSearchResult(msg fileSearchResultMsg) {
	if msg.err != nil {
		m.status = "search failed: " + msg.err.Error()
		return
	}

	m.searchResults = msg.results
	m.searchSelected = 0
	m.searchResultsActive = true
	m.sidebarVisible = true
	m.focus = FocusSearchResults
	m.layout()

	if len(msg.results) == 0 {
		m.status = fmt.Sprintf("no matches for %q", msg.query)
	} else {
		m.status = fmt.Sprintf("%d match(es) for %q", len(msg.results), msg.query)
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
