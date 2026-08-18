package terminal

import (
	"bytes"
	"testing"
	"time"
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

func TestReadWriteBeforeStartReturnsErrNotRunning(t *testing.T) {
	m := New()
	if _, err := m.Write([]byte("x")); err != ErrNotRunning {
		t.Fatalf("Write() before Start error = %v, want ErrNotRunning", err)
	}
	if _, err := m.Read(make([]byte, 1)); err != ErrNotRunning {
		t.Fatalf("Read() before Start error = %v, want ErrNotRunning", err)
	}
}
