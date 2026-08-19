// Package app wires every pkg/ subsystem — buffer, display, explorer,
// highlight, luaengine, paint, terminal — into a single bubbletea Model:
// the editor's central event loop and default keybindings.
package app

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dommcpro/verti/pkg/buffer"
	"github.com/dommcpro/verti/pkg/display"
	"github.com/dommcpro/verti/pkg/explorer"
	"github.com/dommcpro/verti/pkg/highlight"
	"github.com/dommcpro/verti/pkg/luaengine"
	"github.com/dommcpro/verti/pkg/paint"
	"github.com/dommcpro/verti/pkg/terminal"
)

// Focus identifies which pane currently receives non-global keystrokes.
type Focus int

const (
	FocusEditor Focus = iota
	FocusExplorer
	FocusTerminal
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
)

// promptLabels gives the status-bar prefix for prompts that show live
// typed input. promptConfirmDiscard is deliberately absent: its full
// message (already phrased as a question) is set directly on m.status
// instead.
var promptLabels = map[promptKind]string{
	promptFind:        "Find: ",
	promptGoto:        "Go to line: ",
	promptReplaceFind: "Replace -- find: ",
	promptReplaceWith: "Replace -- with: ",
	promptSaveAs:      "Save as: ",
}

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

	term        *terminal.Manager
	termVisible bool
	termOutput  []byte
	termWidth   int
	termHeight  int

	paintOverlay *paint.Overlay

	clipboard string

	prompt          promptKind
	promptText      string
	pendingOpenPath string
	replaceFindTerm string

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

	m := &Model{
		buf:            buf,
		filename:       filename,
		highlighter:    highlight.NewRegistry(),
		editorView:     display.New(),
		exp:            exp,
		sidebarVisible: true,
		term:           terminal.New(),
		paintOverlay:   paint.NewOverlay(),
		focus:          FocusEditor,
		cfg:            cfg,
		keymap:         keymap,
		status:         status,
	}
	return m, nil
}

func (m *Model) Init() tea.Cmd { return nil }

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
		return m, readTermCmd(m.term)
	case termClosedMsg:
		m.termVisible = false
		if m.focus == FocusTerminal {
			m.focus = FocusEditor
		}
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

// layout recomputes every pane's dimensions from the terminal window size,
// matching VS Code's arrangement: the subshell pane spans the full width
// at the bottom, with the sidebar and editor splitting the width above it.
func (m *Model) layout() {
	const statusH = 1

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

	topH := m.height - termH - statusH
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

// recordUndoBoundary snapshots the buffer before an edit if this edit
// doesn't coalesce with the previous one (different kind, or too much
// time has passed), and always clears the redo stack — the usual "any
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

// openFile opens path, but first asks for confirmation (via promptConfirmDiscard)
// if the current buffer has unsaved changes that would otherwise be silently
// discarded.
func (m *Model) openFile(path string) {
	if m.buf.Dirty() {
		m.prompt = promptConfirmDiscard
		m.pendingOpenPath = path
		m.status = "unsaved changes in " + m.displayFilename() + " -- discard and open? (y/n)"
		return
	}
	m.reallyOpenFile(path)
}

func (m *Model) reallyOpenFile(path string) {
	buf, err := buffer.LoadFile(path)
	if err != nil {
		m.status = "open failed: " + err.Error()
		return
	}
	m.buf = buf
	m.filename = path
	m.undoStack = nil
	m.redoStack = nil
	m.lastEditKind = editNone
	m.editorView.ScrollLine, m.editorView.ScrollCol = 0, 0
	m.focus = FocusEditor
	m.status = "opened " + path
}
