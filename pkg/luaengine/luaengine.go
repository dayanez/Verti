// Package luaengine loads the user's init.lua, exposing a small `verti`
// API table (verti.keymap, verti.set) via an embedded Gopher-Lua
// interpreter. A missing config file is not an error: the editor just
// runs with defaults, but a config file that fails to parse or run is
// reported to the caller so it can be surfaced instead of silently
// ignored.
package luaengine

import (
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// Config is the result of evaluating an init.lua script.
type Config struct {
	TabWidth int
	Keymap   map[string]string // key chord (e.g. "ctrl+s") -> command name
}

// DefaultConfig returns the editor's built-in settings, used when no
// config file is present.
func DefaultConfig() *Config {
	return &Config{TabWidth: 4, Keymap: map[string]string{}}
}

// DefaultConfigPath returns the conventional location for the user's
// init.lua (e.g. ~/.config/verti/init.lua on Linux/macOS, %AppData%\verti\init.lua on Windows).
func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "verti", "init.lua"), nil
}

// LoadDefault loads the config at DefaultConfigPath if it exists. A
// missing file returns DefaultConfig with no error.
func LoadDefault() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return DefaultConfig(), nil
	}
	return LoadConfig(path)
}

// LoadConfig evaluates the Lua script at path and returns the resulting
// Config. On error, it still returns whatever was successfully configured
// before the failure, layered onto the defaults, so a script that fails
// partway through doesn't discard everything it did set up.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	L := lua.NewState()
	defer L.Close()

	vertiTable := L.NewTable()
	L.SetGlobal("verti", vertiTable)

	L.SetField(vertiTable, "keymap", L.NewFunction(func(L *lua.LState) int {
		chord := L.CheckString(1)
		command := L.CheckString(2)
		cfg.Keymap[chord] = command
		return 0
	}))

	L.SetField(vertiTable, "set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		switch key {
		case "tab_width":
			cfg.TabWidth = L.CheckInt(2)
		}
		return 0
	}))

	if err := L.DoFile(path); err != nil {
		return cfg, err
	}
	return cfg, nil
}
