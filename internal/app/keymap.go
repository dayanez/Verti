package app

// ---------------- Global Keymap ----------------

// globalToggleTerminalChord is pulled out as a constant because
// handleKey needs to recognize it even while the terminal pane owns
// focus and everything else is being forwarded to the shell.
const globalToggleTerminalChord = "ctrl+j"

// globalKeymap holds chords that work the same regardless of which pane
// has focus, VS Code's model of "global" commands. Focus-specific keys
// (arrows, Enter, Tab, typed characters, ...) are handled directly by
// each pane's key handler instead of going through this table.
//
// The terminal toggle deliberately isn't bound to Ctrl+` (a natural
// mnemonic, but unusable): bubbletea only recognizes Ctrl+<punctuation>
// for the handful of keys with a real ASCII control-code mapping
// (@ [ \ ] ^ _ ?), backtick isn't one of them, and outside that set
// bubbletea's Key.String() doesn't even report a "ctrl+" prefix, so the
// chord is indistinguishable from the bare key. Ctrl+<letter> chords are
// the one category bubbletea resolves the same way on every platform
// (including Windows' native console input, a separate code path from
// ANSI terminals with its own gaps), which is why every chord below is
// either that or a multi-key sequence with an established meaning.
var globalKeymap = map[string]string{
	"ctrl+b":                  "toggle_sidebar",
	globalToggleTerminalChord: "toggle_terminal",
	"ctrl+p":                  "toggle_paint",
	"ctrl+s":                  "save",
	"ctrl+z":                  "undo",
	"ctrl+a":                  "select_all",
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
	"ctrl+n": "new_tab",
	"ctrl+w": "close_tab",
	// Ctrl+PgUp/PgDown for tab switching follows the browser convention
	// (Firefox, Chrome) rather than Ctrl+Tab: many terminal emulators and
	// multiplexers already intercept Ctrl+Tab for their own tab switching,
	// so it would often never reach verti at all.
	"ctrl+pgup":   "prev_tab",
	"ctrl+pgdown": "next_tab",
	// Find-in-files uses Ctrl+T rather than VS Code's Ctrl+Shift+F, for
	// the same Ctrl+Shift+<letter> reliability reason as Ctrl+O above.
	"ctrl+t": "find_in_files",
	// Quick-open (jump to a file by fuzzy-matching its name) doesn't get
	// VS Code's Ctrl+P: that's already Paint here. Ctrl+E was free and
	// unclaimed by any other chord in this table.
	"ctrl+e": "quick_open",
	"f1":     "toggle_help",
	// Ctrl+[ is unusable for this: terminals send the same byte for it as
	// Esc (bubbletea's own KeyCtrlOpenBracket is literally aliased to
	// keyESC), which Verti relies on heavily elsewhere. Ctrl+] has no such
	// collision and was otherwise unclaimed.
	"ctrl+]": "jump_to_matching_bracket",
}
