package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestQuickOpenPromptWiredToCtrlE(t *testing.T) {
	m := newTestModel(t, "hello")
	sendKeyAndRunCmd(m, key(tea.KeyCtrlE))
	if m.prompt != promptQuickOpen {
		t.Fatalf("prompt = %v, want promptQuickOpen", m.prompt)
	}
}

func TestQuickOpenFiltersAndOpensSelectedFile(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if err := os.WriteFile(filepath.Join(root, "widget.go"), []byte("package w\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "unrelated.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sendKeyAndRunCmd(m, key(tea.KeyCtrlE)) // opens the prompt and loads the file list
	sendKeys(m, runes("widget"))

	found := false
	for _, r := range m.quickOpenResults {
		if r == "widget.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("quickOpenResults = %v, want widget.go included for query %q", m.quickOpenResults, "widget")
	}

	sendKeys(m, key(tea.KeyEnter))
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after Enter, want promptNone", m.prompt)
	}
	if !strings.HasSuffix(m.filename, "widget.go") {
		t.Fatalf("filename = %q, want it to end with widget.go", m.filename)
	}
}

func TestQuickOpenUpDownMovesSelectionAndClamps(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	for _, name := range []string{"alpha.go", "alphabet.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package a\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	sendKeyAndRunCmd(m, key(tea.KeyCtrlE))
	sendKeys(m, runes("alpha"))
	if len(m.quickOpenResults) != 2 {
		t.Fatalf("quickOpenResults = %v, want 2 matches", m.quickOpenResults)
	}
	if m.quickOpenSelected != 0 {
		t.Fatalf("quickOpenSelected = %d, want 0", m.quickOpenSelected)
	}

	sendKeys(m, key(tea.KeyDown))
	if m.quickOpenSelected != 1 {
		t.Fatalf("quickOpenSelected after Down = %d, want 1", m.quickOpenSelected)
	}
	sendKeys(m, key(tea.KeyDown)) // no more results below; should clamp
	if m.quickOpenSelected != 1 {
		t.Fatalf("quickOpenSelected after 2nd Down = %d, want clamped at 1", m.quickOpenSelected)
	}
	sendKeys(m, key(tea.KeyUp))
	if m.quickOpenSelected != 0 {
		t.Fatalf("quickOpenSelected after Up = %d, want 0", m.quickOpenSelected)
	}
}

func TestQuickOpenEscCancelsWithoutOpeningAnything(t *testing.T) {
	m := newTestModel(t, "hello")
	root := m.exp.Root.Path
	if err := os.WriteFile(filepath.Join(root, "other.go"), []byte("package o\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	originalFilename := m.filename

	sendKeyAndRunCmd(m, key(tea.KeyCtrlE))
	sendKeys(m, runes("other"), key(tea.KeyEsc))

	if m.prompt != promptNone {
		t.Fatalf("prompt = %v after Esc, want promptNone", m.prompt)
	}
	if m.filename != originalFilename {
		t.Fatalf("filename = %q after Esc, want unchanged %q", m.filename, originalFilename)
	}
}

func TestFuzzyScorePrefersTighterEarlierMatches(t *testing.T) {
	_, ok := fuzzyScore("internal/app/main_test.go", "xyz")
	if ok {
		t.Fatal("fuzzyScore should not match a query whose characters aren't all present in order")
	}

	tight, ok := fuzzyScore("main.go", "main")
	if !ok {
		t.Fatal("fuzzyScore should match a contiguous prefix")
	}
	loose, ok := fuzzyScore("internal/app/main_test.go", "main")
	if !ok {
		t.Fatal("fuzzyScore should match main within a longer path")
	}
	if tight >= loose {
		t.Errorf("tight score = %d, loose score = %d, want the short direct match to score lower (better)", tight, loose)
	}
}
