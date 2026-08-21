<p align="center"><img src="docs/logo.svg" width="56" height="56" alt="Verti"></p>
<h1 align="center">Verti</h1>
<p align="center">A fast, minimal terminal code editor written in Go.</p>

Gap-buffer editing, real tree-sitter syntax highlighting, a file explorer,
and an embedded shell, all in one static binary. No LSP, no plugins, no
telemetry: just a keyboard-driven editor for getting work done.

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

## Install

Two ways to get Verti:

**Download a release.** Grab the binary for your OS from the
[releases page](https://github.com/dayanez/Verti/releases/latest), covering
Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64). Put it
somewhere on your `PATH` and run `verti`.

**Or build from source.** Requires Go 1.25+ and a cgo-capable C compiler.
[Zig](https://ziglang.org/) works well as a lightweight one on any OS
(`zig cc`); `gcc`/`clang`/MSVC work too.

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
All KeyBinds rebindable from `init.lua`. Full keymap: `F1` inside the editor.

## Config

Copy `configs/init.lua` to `~/.config/verti/init.lua` (Linux/macOS) or
`%AppData%\verti\init.lua` (Windows). No config file is required.

## License

[Verti License (MIT with Attribution)](LICENSE): free to use, modify, and
distribute, including commercially, as long as dayanez and Verti are
credited and the work isn't presented as your own original creation.
