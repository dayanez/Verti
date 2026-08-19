package app

// globalToggleTerminalChord is pulled out as a constant because
// handleKey needs to recognize it even while the terminal pane owns
// focus and everything else is being forwarded to the shell.
const globalToggleTerminalChord = "ctrl+@"

// globalKeymap holds chords that work the same regardless of which pane
// has focus — VS Code's model of "global" commands. Focus-specific keys
// (arrows, Enter, Tab, typed characters, ...) are handled directly by
// each pane's key handler instead of going through this table.
//
// Ctrl+` (backtick) is bound as "ctrl+@": bubbletea's key parser reports
// Ctrl+Backtick using the same code as Ctrl+@ (they both send the NUL
// control byte over the wire — see charmbracelet/bubbletea's key.go,
// which documents this explicitly), so binding "ctrl+@" is what actually
// catches Ctrl+`. If your terminal sends something else for that chord,
// override it from init.lua with verti.keymap("ctrl+@", "toggle_terminal").
var globalKeymap = map[string]string{
	"ctrl+b":                  "toggle_sidebar",
	globalToggleTerminalChord: "toggle_terminal",
	"ctrl+p":                  "toggle_paint",
	"ctrl+s":                  "save",
	"ctrl+z":                  "undo",
	"ctrl+y":                  "redo",
	"ctrl+q":                  "quit",
	"ctrl+c":                  "copy",
	"ctrl+x":                  "cut",
	"ctrl+v":                  "paste",
	"ctrl+f":                  "find",
	"ctrl+g":                  "goto_line",
	"ctrl+d":                  "duplicate_line",
	"ctrl+k":                  "delete_line",
	"ctrl+r":                  "replace",
	// Ctrl+O ("write out") for Save As follows nano's convention rather
	// than VS Code's Ctrl+Shift+S: most terminals can't distinguish
	// Ctrl+Shift+<letter> from plain Ctrl+<letter>, since Shift only
	// changes case and Ctrl+letter is already a fixed control code.
	"ctrl+o": "save_as",
	// Ctrl+/ is bound as "ctrl+_": most terminals send the same control
	// byte (0x1F, historically "unit separator") for both chords, a VT100
	// convention bubbletea's key parser reports as "ctrl+_". Override from
	// init.lua with verti.keymap("ctrl+_", "toggle_comment") if yours
	// differs.
	"ctrl+_": "toggle_comment",
}
