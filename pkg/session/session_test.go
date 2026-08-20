package session

import (
	"os"
	"path/filepath"
	"testing"
)

// withConfigDir redirects os.UserConfigDir() to a temp directory for the
// duration of the test, portably across platforms (Go's implementation
// only consults the one relevant to the current OS; setting both is
// harmless).
func withConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)         // Windows
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux
	t.Setenv("HOME", dir)            // macOS fallback (~/Library/Application Support)
}

func TestLoadWithNoSavedSessionReturnsEmpty(t *testing.T) {
	withConfigDir(t)
	s, err := Load("/some/workspace")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Tabs) != 0 {
		t.Fatalf("Tabs = %v, want empty", s.Tabs)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	withConfigDir(t)
	root := t.TempDir()

	want := &Session{
		ActiveTab: 1,
		Tabs: []Tab{
			{Filename: "a.go", CursorOffset: 5, ScrollLine: 2, ScrollCol: 0},
			{Filename: "b.go", CursorOffset: 12, ScrollLine: 0, ScrollCol: 3},
		},
	}
	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ActiveTab != want.ActiveTab || len(got.Tabs) != len(want.Tabs) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
	for i := range want.Tabs {
		if got.Tabs[i] != want.Tabs[i] {
			t.Errorf("Tabs[%d] = %+v, want %+v", i, got.Tabs[i], want.Tabs[i])
		}
	}
}

func TestDifferentWorkspacesGetSeparateSessions(t *testing.T) {
	withConfigDir(t)
	rootA := t.TempDir()
	rootB := t.TempDir()

	if err := Save(rootA, &Session{Tabs: []Tab{{Filename: "a.go"}}}); err != nil {
		t.Fatalf("Save rootA: %v", err)
	}
	if err := Save(rootB, &Session{Tabs: []Tab{{Filename: "b.go"}}}); err != nil {
		t.Fatalf("Save rootB: %v", err)
	}

	gotA, err := Load(rootA)
	if err != nil {
		t.Fatalf("Load rootA: %v", err)
	}
	gotB, err := Load(rootB)
	if err != nil {
		t.Fatalf("Load rootB: %v", err)
	}
	if len(gotA.Tabs) != 1 || gotA.Tabs[0].Filename != "a.go" {
		t.Fatalf("rootA session = %+v, want a.go", gotA)
	}
	if len(gotB.Tabs) != 1 || gotB.Tabs[0].Filename != "b.go" {
		t.Fatalf("rootB session = %+v, want b.go", gotB)
	}
}

func TestLoadWithCorruptFileReturnsEmptyNotError(t *testing.T) {
	withConfigDir(t)
	root := t.TempDir()
	path, err := pathFor(root)
	if err != nil {
		t.Fatalf("pathFor: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Tabs) != 0 {
		t.Fatalf("Tabs = %v, want empty for a corrupt session file", s.Tabs)
	}
}
