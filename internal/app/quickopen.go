package app

import (
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dayanez/Verti/pkg/search"
)

// maxQuickOpenResults caps how many fuzzy matches are kept and shown, so
// a short, common query against a huge project doesn't produce an
// unusable, unbounded list.
const maxQuickOpenResults = 200

// quickOpenFilesMsg carries the workspace's file list back into Update
// once the background walk started by openQuickOpen finishes.
type quickOpenFilesMsg struct {
	files []string
	err   error
}

// openQuickOpen opens the quick-open prompt and kicks off a background
// walk of the workspace for its file list (see search.ListFiles): even
// without reading any file contents, walking a large tree takes real
// time, so it must not block the UI goroutine while it runs.
func (m *Model) openQuickOpen() tea.Cmd {
	m.prompt = promptQuickOpen
	m.promptText = ""
	m.quickOpenFiles = nil
	m.quickOpenResults = nil
	m.quickOpenSelected = 0
	m.sidebarVisible = true
	m.status = "loading files..."
	root := m.exp.Root.Path
	return func() tea.Msg {
		files, err := search.ListFiles(root)
		return quickOpenFilesMsg{files: files, err: err}
	}
}

// applyQuickOpenFiles stores a finished background walk's file list and
// runs the current query (if any was already typed while it was loading)
// against it.
func (m *Model) applyQuickOpenFiles(msg quickOpenFilesMsg) {
	if msg.err != nil {
		m.status = "quick open failed: " + msg.err.Error()
		return
	}
	m.quickOpenFiles = msg.files
	m.status = "ready"
	m.refilterQuickOpen()
}

// refilterQuickOpen re-runs the current prompt text against the loaded
// file list. Called after every keystroke in the quick-open prompt, not
// just once on Enter: matching is a plain in-memory scan over filenames
// (no I/O), so it's cheap enough to redo live on every character.
func (m *Model) refilterQuickOpen() {
	m.quickOpenResults = filterQuickOpen(m.quickOpenFiles, m.promptText)
	m.quickOpenSelected = 0
}

// filterQuickOpen returns the entries of files that fuzzy-match query,
// best match first, capped at maxQuickOpenResults. An empty query matches
// everything, in the order ListFiles returned it.
func filterQuickOpen(files []string, query string) []string {
	if query == "" {
		if len(files) > maxQuickOpenResults {
			return files[:maxQuickOpenResults]
		}
		return files
	}
	type scored struct {
		path  string
		score int
	}
	matches := make([]scored, 0, len(files))
	for _, f := range files {
		if score, ok := fuzzyScore(f, query); ok {
			matches = append(matches, scored{f, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	if len(matches) > maxQuickOpenResults {
		matches = matches[:maxQuickOpenResults]
	}
	out := make([]string, len(matches))
	for i, s := range matches {
		out[i] = s.path
	}
	return out
}

// fuzzyScore reports whether every rune of query appears in candidate in
// order, case-insensitively (the same basic rule most editors' quick-open
// uses), and a score where lower is a better match: an earlier, tighter
// (more contiguous) run of matched characters beats one scattered across
// a long path, and among equally tight matches a shorter overall path
// wins (so "main.go" outranks "internal/app/main_test.go" for "main").
func fuzzyScore(candidate, query string) (score int, ok bool) {
	c := []rune(strings.ToLower(candidate))
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return len(c), true
	}
	ci, qi := 0, 0
	first, last := -1, 0
	for ci < len(c) && qi < len(q) {
		if c[ci] == q[qi] {
			if first < 0 {
				first = ci
			}
			last = ci
			qi++
		}
		ci++
	}
	if qi < len(q) {
		return 0, false
	}
	span := last - first + 1
	return first*2 + span + len(c), true
}

// moveQuickOpenSelection moves the highlighted result by delta, clamped
// to the current result list.
func (m *Model) moveQuickOpenSelection(delta int) {
	if len(m.quickOpenResults) == 0 {
		return
	}
	m.quickOpenSelected += delta
	if m.quickOpenSelected < 0 {
		m.quickOpenSelected = 0
	}
	if m.quickOpenSelected >= len(m.quickOpenResults) {
		m.quickOpenSelected = len(m.quickOpenResults) - 1
	}
}

// openSelectedQuickOpenResult opens the highlighted result (in its own
// tab, per the normal openFile rules) and closes the prompt.
func (m *Model) openSelectedQuickOpenResult() {
	if m.quickOpenSelected < 0 || m.quickOpenSelected >= len(m.quickOpenResults) {
		return
	}
	rel := m.quickOpenResults[m.quickOpenSelected]
	m.openFile(filepath.Join(m.exp.Root.Path, rel))
	m.closeQuickOpen()
}

// closeQuickOpen leaves the quick-open prompt without opening anything.
func (m *Model) closeQuickOpen() {
	m.closePrompt()
	m.quickOpenFiles = nil
	m.quickOpenResults = nil
	m.quickOpenSelected = 0
}
