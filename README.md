```
█   █  █████  ████   █████  █████
█   █  █      █   █    █      █
█   █  █      █   █    █      █
█   █  ████   ████     █      █
 █ █   █      █  █     █      █
 █ █   █      █   █    █      █
  █    █████  █   █    █    █████
```

[![build](https://github.com/dayanez/Verti/actions/workflows/build.yml/badge.svg)](https://github.com/dayanez/Verti/actions/workflows/build.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/dayanez/Verti)](go.mod)

A fast, minimal terminal code editor written in Go. Gap-buffer editing,
real tree-sitter syntax highlighting, a file explorer, and an embedded
shell, all in one static binary. No LSP, no plugins, no telemetry: just a
keyboard-driven editor for getting work done.

## Features

- Hand-written gap buffer: O(1)-amortized editing at the cursor, with
  line/column tracking and sticky-column vertical movement.
- Real tree-sitter highlighting for 21+ languages, classified by AST node
  and parent context rather than regex.
- File explorer sidebar (`Ctrl+B`), double-Enter to open a file.
- Embedded subshell (`Ctrl+\``), a real PTY on every OS.
- Buffer-local word completion, no language server required.
- Lua-configurable keybindings and settings (`init.lua`).
- ASCII box-drawing paint overlay (`Ctrl+P`) for dropping diagrams into
  comments.

## Building from source

Requires Go 1.25+ and a cgo-capable C compiler. [Zig](https://ziglang.org/)
works well as a lightweight one on any OS (`zig cc`); `gcc`/`clang`/MSVC
work too.

```sh
git clone https://github.com/dayanez/Verti
cd Verti

# Windows
.\scripts\dev.ps1 build

# Linux/macOS
make build
```

`make test` / `.\scripts\dev.ps1 test` runs the test suite.

## Usage

```sh
verti                 # open the current directory
verti path/to/file.go # open a file
verti path/to/dir     # open a specific directory
```

| Chord | Action |
|---|---|
| `Ctrl+B` | Toggle the file explorer |
| `Ctrl+\`` | Toggle the embedded shell |
| `Ctrl+P` | Toggle the paint overlay |
| `Ctrl+S` / `Ctrl+O` | Save / Save As |
| `Ctrl+Z` / `Ctrl+Y` | Undo / redo |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Copy / cut / paste |
| `Ctrl+F` / `Ctrl+R` | Find / replace all |
| `Ctrl+G` | Go to line |
| `Ctrl+D` / `Ctrl+K` | Duplicate / delete line |
| `Ctrl+/` | Toggle comment |
| `Ctrl+Q` | Quit |
| `Tab` / `Shift+Tab` | Indent / outdent |

All rebindable from `init.lua`. Full keymap: `F1` inside the editor.

## Config

Copy `configs/init.lua` to `~/.config/verti/init.lua` (Linux/macOS) or
`%AppData%\verti\init.lua` (Windows). No config file is required.

## License

[Verti License (MIT with Attribution)](LICENSE): free to use, modify, and
distribute, including commercially, as long as dayanez and Verti are
credited and the work isn't presented as your own original creation.
