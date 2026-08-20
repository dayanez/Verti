// Package session persists and restores a workspace's editing session --
// which files were open, each one's cursor and scroll position, and
// which tab was active -- across restarts, so reopening a workspace with
// no specific file requested picks up where you left off instead of
// starting from a single blank buffer every time.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Tab is one open file's saved editing position.
type Tab struct {
	Filename     string
	CursorOffset int
	ScrollLine   int
	ScrollCol    int
}

// Session is a workspace's saved set of open tabs.
type Session struct {
	ActiveTab int
	Tabs      []Tab
}

// pathFor returns the session file for workspace root: one file per
// workspace, named by a hash of its absolute path, under the same config
// directory init.lua lives in (see pkg/luaengine.DefaultConfigPath).
func pathFor(root string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(abs)))
	return filepath.Join(dir, "verti", "sessions", hex.EncodeToString(sum[:8])+".json"), nil
}

// Load reads the saved session for root. A missing file, an unwritable
// config directory, or a corrupt session file all return a zero-value
// Session (nothing to restore) rather than an error: session restore is
// a convenience, never something that should block opening the editor.
func Load(root string) (*Session, error) {
	path, err := pathFor(root)
	if err != nil {
		return &Session{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &Session{}, nil
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return &Session{}, nil
	}
	return &s, nil
}

// Save writes s as the session for root, creating the sessions directory
// if it doesn't exist yet.
func Save(root string, s *Session) error {
	path, err := pathFor(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
