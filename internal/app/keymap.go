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
	// Ctrl+C is bound to quit as a safety net: there is no OS clipboard
	// integration yet (see README), so Ctrl+C isn't "copy" here the way
	// it is in VS Code — until clipboard support lands, quitting is the
	// more useful default so the app is never impossible to exit.
	"ctrl+c": "quit",
}
