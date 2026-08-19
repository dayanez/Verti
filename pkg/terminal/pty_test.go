package terminal

import (
	"bytes"
	"testing"
	"time"

	"github.com/hinshun/vt10x"
)

func TestStartWriteReadClose(t *testing.T) {
	m := New()
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	defer m.Close()

	if !m.Running() {
		t.Fatal("Running() = false after Start")
	}

	marker := "verti-pty-check"
	cmd := "echo " + marker + "\r\n"
	if _, err := m.Write([]byte(cmd)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var out bytes.Buffer
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := readWithTimeout(m, buf, time.Second)
		if n > 0 {
			out.Write(buf[:n])
		}
		if bytes.Contains(out.Bytes(), []byte(marker)) {
			return
		}
		if err != nil {
			break
		}
	}
	t.Fatalf("did not see marker %q in PTY output; got: %q", marker, out.String())
}

func readWithTimeout(m *Manager, buf []byte, timeout time.Duration) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := m.Read(buf)
		ch <- result{n, err}
	}()
	select {
	case r := <-ch:
		return r.n, r.err
	case <-time.After(timeout):
		return 0, nil
	}
}

func TestStartTwiceReturnsErrAlreadyRunning(t *testing.T) {
	m := New()
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	defer m.Close()
	if err := m.Start(80, 24); err != ErrAlreadyRunning {
		t.Fatalf("second Start() error = %v, want ErrAlreadyRunning", err)
	}
}

func TestScreenNilBeforeStartAndAfterClose(t *testing.T) {
	m := New()
	if m.Screen() != nil {
		t.Fatal("Screen() before Start should be nil")
	}
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	if m.Screen() == nil {
		t.Fatal("Screen() after Start should be non-nil")
	}
	m.Close()
	if m.Screen() != nil {
		t.Fatal("Screen() after Close should be nil again")
	}
}

func TestFeedUpdatesScreenCells(t *testing.T) {
	m := New()
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	defer m.Close()

	m.Feed([]byte("hi"))
	vt := m.Screen()
	vt.Lock()
	defer vt.Unlock()
	if got := vt.Cell(0, 0).Char; got != 'h' {
		t.Fatalf("Cell(0,0).Char = %q, want 'h'", got)
	}
	if got := vt.Cell(1, 0).Char; got != 'i' {
		t.Fatalf("Cell(1,0).Char = %q, want 'i'", got)
	}
	cur := vt.Cursor()
	if cur.X != 2 || cur.Y != 0 {
		t.Fatalf("Cursor() = (%d,%d), want (2,0) after writing 2 characters", cur.X, cur.Y)
	}
}

func TestFeedDetectsAltScreenMode(t *testing.T) {
	m := New()
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	defer m.Close()

	if vtInAltScreenForTest(m.Screen()) {
		t.Fatal("alt-screen mode should be off before any escape sequence is fed")
	}

	// The standard "enter alternate screen buffer" sequence, sent by vim,
	// less, htop, and other full-screen programs.
	m.Feed([]byte("\x1b[?1049h"))
	if !vtInAltScreenForTest(m.Screen()) {
		t.Fatal("alt-screen mode should be on after \\x1b[?1049h")
	}

	// And the matching "exit" sequence, sent when such a program quits.
	m.Feed([]byte("\x1b[?1049l"))
	if vtInAltScreenForTest(m.Screen()) {
		t.Fatal("alt-screen mode should be off after \\x1b[?1049l")
	}
}

func vtInAltScreenForTest(vt vt10x.Terminal) bool {
	vt.Lock()
	defer vt.Unlock()
	return vt.Mode()&vt10x.ModeAltScreen != 0
}

func TestResizeAlsoResizesVirtualTerminal(t *testing.T) {
	m := New()
	if err := m.Start(80, 24); err != nil {
		t.Skipf("PTY unavailable in this environment: %v", err)
	}
	defer m.Close()

	if err := m.Resize(100, 40); err != nil {
		t.Fatalf("Resize() error: %v", err)
	}
	vt := m.Screen()
	vt.Lock()
	cols, rows := vt.Size()
	vt.Unlock()
	if cols != 100 || rows != 40 {
		t.Fatalf("Screen().Size() = (%d,%d), want (100,40)", cols, rows)
	}
}

func TestReadWriteBeforeStartReturnsErrNotRunning(t *testing.T) {
	m := New()
	if _, err := m.Write([]byte("x")); err != ErrNotRunning {
		t.Fatalf("Write() before Start error = %v, want ErrNotRunning", err)
	}
	if _, err := m.Read(make([]byte, 1)); err != ErrNotRunning {
		t.Fatalf("Read() before Start error = %v, want ErrNotRunning", err)
	}
}
