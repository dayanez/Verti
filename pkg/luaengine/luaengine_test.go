package luaengine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeScript(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "init.lua")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigAppliesKeymapAndSet(t *testing.T) {
	path := writeScript(t, `
verti.set("tab_width", 2)
verti.keymap("ctrl+shift+p", "command_palette")
verti.keymap("ctrl+s", "save")
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.TabWidth != 2 {
		t.Errorf("TabWidth = %d, want 2", cfg.TabWidth)
	}
	if cfg.Keymap["ctrl+shift+p"] != "command_palette" {
		t.Errorf("Keymap[ctrl+shift+p] = %q, want command_palette", cfg.Keymap["ctrl+shift+p"])
	}
	if cfg.Keymap["ctrl+s"] != "save" {
		t.Errorf("Keymap[ctrl+s] = %q, want save", cfg.Keymap["ctrl+s"])
	}
}

func TestLoadConfigReturnsPartialConfigOnError(t *testing.T) {
	path := writeScript(t, `
verti.set("tab_width", 8)
this_is_not_valid_lua((()
`)
	cfg, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected an error from invalid Lua syntax")
	}
	if cfg == nil {
		t.Fatal("expected a non-nil Config even on error")
	}
}

func TestLoadDefaultWithNoFileReturnsDefaults(t *testing.T) {
	empty := t.TempDir() // ensures no init.lua exists there, on any OS
	t.Setenv("XDG_CONFIG_HOME", empty)
	t.Setenv("AppData", empty)
	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cfg.TabWidth != 4 {
		t.Errorf("TabWidth = %d, want default 4", cfg.TabWidth)
	}
}
