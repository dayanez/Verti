// Package app wires every pkg/ subsystem (buffer, display, explorer,
// highlight, luaengine, paint, terminal) into a single bubbletea Model:
// the editor's central event loop and default keybindings.
package app

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dayanez/Verti/pkg/buffer"
	"github.com/dayanez/Verti/pkg/display"
	"github.com/dayanez/Verti/pkg/explorer"
	"github.com/dayanez/Verti/pkg/highlight"
	"github.com/dayanez/Verti/pkg/luaengine"
	"github.com/dayanez/Verti/pkg/paint"
	"github.com/dayanez/Verti/pkg/search"
	"github.com/dayanez/Verti/pkg/terminal"
)

// ---------------- Focus & prompt types ----------------

// Focus identifies which pane currently receives non-global keystrokes.
type Focus int

const (
	FocusEditor Focus = iota
	FocusExplorer
	FocusTerminal
	FocusSearchResults
)

type editKind int

const (
	editNone editKind = iota
	editInsert
	editDelete
)

// promptKind identifies which single-line prompt (if any) currently owns
// keyboard input, overlaid on the status bar.
type promptKind int

const (
	promptNone promptKind = iota
	promptFind
	promptGoto
	promptConfirmDiscard
	promptReplaceFind
	promptReplaceWith
	promptSaveAs
	promptFindInFiles
	promptQuickOpen
	promptNewFile
	promptNewFolder
	promptRename
	promptConfirmDeleteFile
)

// promptLabels gives the status-bar prefix for prompts that show live
// typed input. promptConfirmDiscard and promptConfirmDeleteFile are
// deliberately absent: their full message (already phrased as a
// question) is set directly on m.status instead.
var promptLabels = map[promptKind]string{
	promptFind:        "Find: ",
	promptGoto:        "Go to line: ",
	promptReplaceFind: "Replace -- find: ",
	promptReplaceWith: "Replace -- with: ",
	promptSaveAs:      "Save as: ",
	promptFindInFiles: "Find in files: ",
	promptQuickOpen:   "Go to file: ",
	promptNewFile:     "New file: ",
	promptNewFolder:   "New folder: ",
	promptRename:      "Rename to: ",
}

// ---------------- Model ----------------

type snapshot struct {
	text   string
	cursor int
}

// undoCoalesceWindow: consecutive edits of the same kind within this
// window are treated as one undo step, so undo doesn't require pressing
// Ctrl+Z once per character.
const undoCoalesceWindow = 700 * time.Millisecond

const maxTermOutput = 64 * 1024 // cap the in-memory subshell scrollback

// Model is the editor's root bubbletea model.
type Model struct {
	buf         *buffer.GapBuffer
	filename    string
	highlighter *highlight.Registry
	editorView  *display.Editor

	exp            *explorer.Explorer
	sidebarVisible bool

	// searchResults holds project-wide find-in-files results, shown in
	// the sidebar area in place of the file tree while searchResultsActive.
	searchResults       []search.Match
	searchSelected      int
	searchResultsActive bool

	// quickOpenFiles is the workspace's full file list, loaded once when
	// the quick-open prompt opens; quickOpenResults is the fuzzy-filtered
	// subset matching promptText, re-filtered on every keystroke while
	// promptQuickOpen is active.
	quickOpenFiles    []string
	quickOpenResults  []string
	quickOpenSelected int

	// completionActive is true immediately after Tab inserted a
	// buffer-word completion, so a second consecutive Tab (nothing else
	// happening in between) cycles to the next candidate instead of
	// starting over; completionPrefixStart/completionCandidates/
	// completionIndex track the cycle in progress. Any other key ends it
	// (see handleEditorKey).
	completionActive      bool
	completionPrefixStart int
	completionCandidates  []string
	completionIndex       int

	term        *terminal.Manager
	termVisible bool
	termOutput  []byte
	termWidth   int
	termHeight  int

	paintOverlay *paint.Overlay

	// helpVisible shows the current keybinding overlay (F1 to toggle);
	// helpScroll is how far into it the user has scrolled.
	helpVisible bool
	helpScroll  int
	// helpLinesCache memoizes helpLines(): m.keymap never changes after
	// startup, so it's built once and reused rather than rebuilt on
	// every scroll keypress and render frame the overlay is open for.
	helpLinesCache []string

	// mouseSelecting is true between a left-button press in the editor
	// and its release, so drag motion events extend a selection instead
	// of being ignored or (worse) misread as a fresh click.
	mouseSelecting bool

	clipboard   string
	osClipboard osClipboard

	prompt          promptKind
	promptText      string
	pendingCloseTab bool
	pendingQuit     bool
	replaceFindTerm string

	// tabs holds every open file's editing state; activeTab indexes the
	// one currently mirrored into buf/filename/undoStack/redoStack/etc.
	// above. See tabs.go for how switching tabs snapshots and restores
	// that state.
	tabs      []*openBuffer
	activeTab int

	focus  Focus
	width  int
	height int
	status string

	cfg    *luaengine.Config
	keymap map[string]string

	undoStack    []snapshot
	redoStack    []snapshot
	lastEditKind editKind
	lastEditTime time.Time

	quitting bool
}

// ---------------- Construction ----------------

// New builds the editor's model. rootDir is the explorer's workspace
// root; filePath (optional) is a file to open on startup.
func New(rootDir, filePath string) (*Model, error) {
	cfg, cfgErr := luaengine.LoadDefault()

	exp, err := explorer.New(rootDir)
	if err != nil {
		return nil, err
	}

	buf := buffer.New()
	filename := ""
	if filePath != "" {
		if b, err := buffer.LoadFile(filePath); err == nil {
			buf = b
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		filename = filePath
	}

	keymap := make(map[string]string, len(globalKeymap)+len(cfg.Keymap))
	for k, v := range globalKeymap {
		keymap[k] = v
	}
	for k, v := range cfg.Keymap {
		keymap[k] = v
	}

	status := "ready"
	if cfgErr != nil {
		status = "config error: " + cfgErr.Error()
	}

	registry := highlight.NewRegistry()
	editorView := display.New()
	editorView.TabWidth = tabWidthOrDefault(cfg.TabWidth)
	m := &Model{
		buf:            buf,
		filename:       filename,
		highlighter:    registry,
		editorView:     editorView,
		exp:            exp,
		sidebarVisible: true,
		term:           terminal.New(),
		paintOverlay:   paint.NewOverlay(),
		focus:          FocusEditor,
		cfg:            cfg,
		keymap:         keymap,
		status:         status,
		tabs:           []*openBuffer{{buf: buf, filename: filename, highlighter: highlighterFor(registry, filename)}},
		activeTab:      0,
		osClipboard:    realOSClipboard{},
	}
	// Session restore only kicks in when no specific file was requested:
	// `verti path/to/file.go` is a deliberate, focused request that
	// shouldn't get buried under whatever tabs happened to be open last
	// time, but plain `verti` (or `verti .`) is exactly "reopen this
	// workspace," which is what session restore is for.
	if filePath == "" {
		m.restoreSession(rootDir)
	}
	return m, nil
}

// ---------------- Bubbletea lifecycle ----------------

func (m *Model) Init() tea.Cmd { return nil }

// Close releases resources New acquired that outlive a single Update call,
// namely the embedded subshell's PTY. Callers should defer this right
// after New succeeds, so the subshell is torn down on every exit path
// (an error from tea.Program.Run, a caught signal, ...), not only the
// explicit "quit" keybinding, which also calls this as part of its own
// cleanup. Safe to call more than once or when no subshell was started.
func (m *Model) Close() error {
	return m.term.Close()
}

type termOutputMsg struct{ data []byte }
type termClosedMsg struct{}

func readTermCmd(term *terminal.Manager) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := term.Read(buf)
		if err != nil {
			return termClosedMsg{}
		}
		out := make([]byte, n)
		copy(out, buf[:n])
		return termOutputMsg{data: out}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case termOutputMsg:
		m.appendTermOutput(msg.data)
		m.term.Feed(msg.data)
		return m, readTermCmd(m.term)
	case termClosedMsg:
		// The subshell exited on its own (e.g. the user typed "exit"), so
		// Read already failed and there's nothing left to read from. Close
		// releases the pty and clears Running(), so the next Ctrl+J starts
		// a fresh subshell instead of finding one it thinks is still alive.
		_ = m.term.Close()
		m.termVisible = false
		if m.focus == FocusTerminal {
			m.focus = FocusEditor
		}
		m.layout()
		return m, nil
	case fileSearchResultMsg:
		m.applyFileSearchResult(msg)
		return m, nil
	case gitStatusMsg:
		m.exp.SetGitStatus(msg.status)
		return m, nil
	case quickOpenFilesMsg:
		m.applyQuickOpenFiles(msg)
		return m, nil
	}
	return m, nil
}

func (m *Model) appendTermOutput(data []byte) {
	m.termOutput = append(m.termOutput, data...)
	if len(m.termOutput) > maxTermOutput {
		m.termOutput = m.termOutput[len(m.termOutput)-maxTermOutput:]
	}
}

// ---------------- Layout ----------------

// layout recomputes every pane's dimensions from the terminal window size,
// matching VS Code's arrangement: the subshell pane spans the full width
// at the bottom, with the sidebar and editor splitting the width above it.
// tabBarHeight is 1 when the tab bar is showing (more than one tab open)
// and 0 otherwise. layout() uses it to size the panes below the tab bar;
// editorOrigin() (handlers.go) uses the same value to know where the
// editor's content actually starts on screen, so mouse coordinates land
// on the right row -- both need to agree on this, not each recompute it.
func (m *Model) tabBarHeight() int {
	if len(m.tabs) > 1 {
		return 1
	}
	return 0
}

func (m *Model) layout() {
	const statusH = 1

	tabBarH := m.tabBarHeight()

	sidebarW := 0
	if m.sidebarVisible {
		sidebarW = 28
		if max := m.width / 3; sidebarW > max {
			sidebarW = max
		}
		if sidebarW < 10 {
			sidebarW = 10
		}
	}

	termH := 0
	if m.termVisible {
		termH = 10
		if max := m.height / 3; termH > max {
			termH = max
		}
		if termH < 3 {
			termH = 3
		}
	}

	topH := m.height - termH - statusH - tabBarH
	if topH < 1 {
		topH = 1
	}
	sep := 0
	if m.sidebarVisible {
		sep = 1 // one column separating the sidebar from the editor
	}
	editorW := m.width - sidebarW - sep
	if editorW < 1 {
		editorW = 1
	}

	m.editorView.SetSize(editorW, topH)
	m.exp.SetSize(sidebarW, topH)
	m.termWidth, m.termHeight = m.width, termH
	if m.term.Running() {
		_ = m.term.Resize(m.termWidth, m.termHeight)
	}
}

// ---------------- Undo / redo ----------------

// recordUndoBoundary snapshots the buffer before an edit if this edit
// doesn't coalesce with the previous one (different kind, or too much
// time has passed), and always clears the redo stack: the usual "any
// new edit invalidates redo history" rule.
func (m *Model) recordUndoBoundary(kind editKind) {
	now := time.Now()
	if kind != m.lastEditKind || now.Sub(m.lastEditTime) > undoCoalesceWindow {
		m.undoStack = append(m.undoStack, snapshot{text: m.buf.String(), cursor: m.buf.CursorOffset()})
		if len(m.undoStack) > 200 {
			m.undoStack = m.undoStack[1:]
		}
	}
	m.redoStack = nil
	m.lastEditKind = kind
	m.lastEditTime = now
}

func (m *Model) undo() {
	if len(m.undoStack) == 0 {
		return
	}
	cur := snapshot{text: m.buf.String(), cursor: m.buf.CursorOffset()}
	last := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	m.redoStack = append(m.redoStack, cur)
	m.buf.Restore(last.text, last.cursor)
	m.lastEditKind = editNone
}

func (m *Model) redo() {
	if len(m.redoStack) == 0 {
		return
	}
	cur := snapshot{text: m.buf.String(), cursor: m.buf.CursorOffset()}
	last := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	m.undoStack = append(m.undoStack, cur)
	m.buf.Restore(last.text, last.cursor)
	m.lastEditKind = editNone
}
