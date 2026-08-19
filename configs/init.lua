-- Example verti config.
--
-- Copy this file to your config directory and edit it:
--   Linux/macOS: ~/.config/verti/init.lua
--   Windows:     %AppData%\verti\init.lua
--
-- It's plain Lua, run through an embedded Gopher-Lua interpreter at
-- startup. Two functions are available on the global `verti` table:
--
--   verti.set(option, value)     -- change an editor setting
--   verti.keymap(chord, command) -- bind or rebind a keyboard shortcut
--
-- A config file is entirely optional. If it's missing, verti just runs
-- with its built-in defaults.

verti.set("tab_width", 4)

-- Chords are written like bubbletea reports them: "ctrl+s", "alt+x",
-- plain key names like "enter" or "f5". Rebinding a chord here overrides
-- the built-in default for that chord.
--
-- Available command names: toggle_sidebar, toggle_terminal, toggle_paint,
-- save, undo, redo, quit.
verti.keymap("ctrl+q", "quit")
verti.keymap("ctrl+s", "save")

-- Example of remapping the paint overlay to a different chord instead of
-- Ctrl+P:
-- verti.keymap("ctrl+p", "toggle_paint")
