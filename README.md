# verti

A terminal text editor written in Go, built on
[bubbletea](https://github.com/charmbracelet/bubbletea) and
[lipgloss](https://github.com/charmbracelet/lipgloss). It's a normal code
editor: gap-buffer text editing, a file explorer sidebar, real tree-sitter
syntax highlighting, an embedded shell, plus one thing other editors don't
have: a **paint overlay** that turns mouse drags into Unicode box-drawing
diagrams and drops them into your code as a properly-formatted comment for
whatever language you're editing.

## Features

- **Gap buffer core** (`pkg/buffer`): thread-safe, hand-written text
  storage with O(1)-amortized insertion/deletion at the cursor, line/column
  tracking, and sticky-column vertical movement.
- **Real tree-sitter highlighting** (`pkg/highlight`): 21+ languages
  (Go, Python, JavaScript/JSX, TypeScript/TSX, Rust, C/C++, Lua, Markdown,
  Bash, CSS, HTML, YAML, TOML, Ruby, Java, PHP, SQL, C#, Kotlin, Swift, and
  Dockerfile) are parsed into a real AST and classified using node type
  *and* parent context (so a function name colors differently from a plain
  variable), not regex. JSON is covered by a small hand-written tokenizer,
  since tree-sitter-json has no bundled Go grammar.
- **File explorer sidebar** (`pkg/explorer`): `Ctrl+B` toggles it, arrow
  keys navigate, directories expand on a single Enter, files need a second
  Enter within half a second to open (so a stray keypress while browsing
  doesn't yank you into a file).
- **Embedded subshell** (`pkg/terminal`): `Ctrl+\`` toggles a bottom pane
  running your real shell, attached to a proper PTY (Unix `pty` /
  Windows ConPTY via `aymanbagabas/go-pty`).
- **Paint overlay** (`pkg/paint`): `Ctrl+P` opens a drawing surface; drag
  with the mouse and each stroke is rasterized with Bresenham's line
  algorithm, then resolved into proper box-drawing corners and junctions
  (`┌┐└┘├┤┬┴┼`) by checking each cell's neighbors. Press Enter to insert the
  result as a comment block in your file's language (`//`, `#`, `--`,
  `/* */`, ...); press `c` to clear and keep drawing, Esc to cancel.
- **Lua configuration** (`pkg/luaengine`): an embedded
  [Gopher-Lua](https://github.com/yuin/gopher-lua) interpreter loads
  `init.lua` and lets you rebind keys and change settings. See
  `configs/init.lua` for a documented example.

## Building from source

You need Go 1.22+ and [Zig](https://ziglang.org/), not because verti is
written in Zig, but because real tree-sitter grammars are C code compiled
via cgo, and `zig cc` is a lightweight, cross-platform, drop-in cgo
compiler (a single portable download, versus a multi-GB Visual Studio
install on Windows or requiring system gcc elsewhere).

```sh
# install zig (pick one)
winget install zig.zig     # Windows
brew install zig           # macOS
# or download from https://ziglang.org/download/

git clone https://github.com/dommcpro/verti
cd verti

# Windows
.\scripts\dev.ps1 build

# Linux/macOS
make build
```

Both wrap `go build` with `CC=zig cc CXX=zig c++ CGO_ENABLED=1` so you don't
have to set those yourself. If you'd rather not use Zig, any cgo-capable C
compiler works the same way (`gcc`, `clang`, MSVC's `cl`); just export
those env vars pointing at it instead.

Run `make test` / `.\scripts\dev.ps1 test` to run the test suite (unit
tests cover the gap buffer, the tree-sitter classification, the explorer's
navigation and double-Enter logic, Bresenham line generation and
box-drawing junction resolution, comment formatting, the PTY manager, and
the Lua config loader).

## Usage

```sh
verti                 # open the current directory
verti path/to/file.go # open a file (workspace root = its directory)
verti path/to/dir     # open a specific directory as the workspace
```

### Default keybindings

| Chord | Action |
|---|---|
| `Ctrl+B` | Toggle the file explorer sidebar |
| `Ctrl+\`` | Toggle the embedded shell pane |
| `Ctrl+P` | Toggle the paint overlay |
| `Ctrl+S` | Save (opens Save As if the buffer has no filename yet) |
| `Ctrl+O` | Save As |
| `Ctrl+Z` / `Ctrl+Y` | Undo / redo |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Copy / cut / paste (selection, or the current line if none) |
| `Ctrl+F` | Find (Enter finds the next match, wrapping around) |
| `Ctrl+R` | Replace all |
| `Ctrl+G` | Go to line |
| `Ctrl+D` | Duplicate the current line |
| `Ctrl+K` | Delete the current line |
| `Ctrl+/` | Toggle line (or block) comment |
| `Ctrl+Q` | Quit |
| Shift+arrows/Home/End | Extend a selection |
| `Ctrl+Left` / `Ctrl+Right` | Jump by word |
| `Tab` / `Shift+Tab` | Indent / outdent a multi-line selection (else insert/remove one tab) |
| Arrows, Home/End, PgUp/PgDown | Move the cursor |
| Enter (in explorer) | Expand a folder; open a file on the *second* press |
| Esc | Return focus to the editor |

When the shell pane has focus, everything except Esc and `Ctrl+\`` is
forwarded to the shell as raw input, so `Ctrl+C` interrupts a running
command instead of quitting the editor. All of the above are rebindable
from `init.lua`; see `configs/init.lua`.

### Config file

Copy `configs/init.lua` to:

- Linux/macOS: `~/.config/verti/init.lua`
- Windows: `%AppData%\verti\init.lua`

A missing config file is not an error; verti just runs with defaults.

## Project layout

```
cmd/verti/       entrypoint
internal/app/    the bubbletea Model: event loop, keybindings, layout
pkg/buffer/      gap buffer
pkg/display/     buffer -> styled terminal text (line numbers, scrolling, cursor)
pkg/explorer/    file tree sidebar
pkg/highlight/   tree-sitter AST classification + JSON tokenizer
pkg/luaengine/   Gopher-Lua config loader
pkg/paint/       Bresenham, box-drawing junction resolution, comment formatting
pkg/terminal/    PTY-backed subshell manager
```

Everything under `pkg/` is UI-agnostic and has no dependency on bubbletea;
`internal/app` is the only place that wires them together into a running
program.

## Known limitations

This is a v1 scope, not the ceiling. A few things worth knowing about
rather than being surprised by:

- **OS clipboard integration is write-only.** `Ctrl+C`/`Ctrl+X` mirror to
  the system clipboard via an OSC52 escape sequence (works over SSH and
  tmux too), but there's no reliable way to read the system clipboard back
  from a terminal app, so `Ctrl+V` always pastes from verti's own internal
  clipboard register instead.
- **The shell pane renders plain text, not a full terminal.** Output is
  ANSI-stripped before display, so a normal shell session (`ls`, `git
  status`, build output, ...) looks right, but a full-screen program run
  inside it (`vim`, `top`, `htop`) won't redraw correctly. That needs a
  real VT100 cell-grid emulator, which is out of scope for v1.
- **Paint mode replaces the editor pane while drawing** rather than
  alpha-blending onto the live styled text underneath. Compositing plain
  glyphs onto ANSI-styled text safely needs a real cell-grid renderer, not
  string concatenation.
- **Enabling mouse mode for the paint overlay disables your terminal's
  native click-drag text selection** everywhere else in the app (a known
  tradeoff for any terminal app with mouse features; vim and tmux have the
  same one). Most terminals (Windows Terminal, iTerm2, GNOME Terminal, ...)
  let you hold Shift while dragging to bypass this and select text
  natively.
- **`go test -race` doesn't currently work when using `zig cc` as the
  compiler on Windows.** ThreadSanitizer needs `WaitOnAddress` /
  `WakeByAddress*` from `Synchronization.lib`, which zig's bundled mingw
  libs don't include. The buffer package's concurrency test still runs
  (just without the race detector attached); `-race` works fine on Linux
  CI or with a real gcc/clang install.
- No multi-cursor. Single-buffer, single-file editing for now (selection,
  clipboard, find/replace, go-to-line, and Save As are all supported).

## License

MIT. See [LICENSE](LICENSE).
