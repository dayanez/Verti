// Package terminal manages the embedded interactive subshell shown in the
// bottom pane (toggled with Ctrl+`). It wraps a cross-platform PTY
// (Unix pty on Linux/macOS, ConPTY on Windows via aymanbagabas/go-pty) so
// the rest of the editor only ever deals with plain Read/Write/Resize. It
// also feeds every byte read from the PTY into a vt10x virtual terminal,
// so callers can render a real full-screen program (vim, top, ...)
// correctly via Screen() instead of just scrolling raw bytes.
package terminal

import (
	"errors"
	"os"
	"runtime"
	"sync"

	"github.com/aymanbagabas/go-pty"
	"github.com/hinshun/vt10x"
)

var (
	ErrNotRunning     = errors.New("terminal: not running")
	ErrAlreadyRunning = errors.New("terminal: already running")
)

// Manager owns a single subshell process attached to a PTY.
type Manager struct {
	mu  sync.Mutex
	pty pty.Pty
	cmd *pty.Cmd
	vt  vt10x.Terminal
}

// New returns a Manager with no subshell started yet.
func New() *Manager { return &Manager{} }

// Start spawns the user's default shell attached to a new PTY sized
// cols x rows (a size of 0,0 skips the initial resize).
func (m *Manager) Start(cols, rows int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pty != nil {
		return ErrAlreadyRunning
	}
	p, err := pty.New()
	if err != nil {
		return err
	}
	cmd := p.Command(defaultShell())
	if err := cmd.Start(); err != nil {
		p.Close()
		return err
	}
	if cols > 0 && rows > 0 {
		_ = p.Resize(cols, rows)
	}
	vtCols, vtRows := cols, rows
	if vtCols < 1 {
		vtCols = 1
	}
	if vtRows < 1 {
		vtRows = 1
	}
	m.pty = p
	m.cmd = cmd
	m.vt = vt10x.New(vt10x.WithSize(vtCols, vtRows))
	return nil
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		if p := os.Getenv("COMSPEC"); p != "" {
			return p
		}
		return "powershell.exe"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}

// Read reads subshell output. It blocks like a normal PTY read; callers
// driving a UI loop should run it in its own goroutine.
func (m *Manager) Read(p []byte) (int, error) {
	pt := m.activePty()
	if pt == nil {
		return 0, ErrNotRunning
	}
	return pt.Read(p)
}

// Write sends input to the subshell (keystrokes typed into the pane).
func (m *Manager) Write(p []byte) (int, error) {
	pt := m.activePty()
	if pt == nil {
		return 0, ErrNotRunning
	}
	return pt.Write(p)
}

// Resize notifies the subshell that the pane's dimensions changed.
func (m *Manager) Resize(cols, rows int) error {
	pt := m.activePty()
	if pt == nil {
		return nil
	}
	if cols > 0 && rows > 0 {
		if vt := m.Screen(); vt != nil {
			vt.Resize(cols, rows)
		}
	}
	return pt.Resize(cols, rows)
}

// Feed writes PTY output bytes into the virtual terminal, updating its
// screen state (cursor, cell grid, alt-screen mode, ...). Call this with
// whatever Read returns; it's a no-op if no subshell is running.
func (m *Manager) Feed(data []byte) {
	if vt := m.Screen(); vt != nil {
		_, _ = vt.Write(data)
	}
}

// Screen returns the virtual terminal's current state for rendering
// (Cell, Cursor, CursorVisible, Mode, Size), or nil if no subshell is
// running.
func (m *Manager) Screen() vt10x.Terminal {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vt
}

// Running reports whether a subshell is currently active.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pty != nil
}

func (m *Manager) activePty() pty.Pty {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pty
}

// Close terminates the subshell and releases the PTY. It is safe to call
// even if no subshell is running.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pty == nil {
		return nil
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	err := m.pty.Close()
	m.pty = nil
	m.cmd = nil
	m.vt = nil
	return err
}
