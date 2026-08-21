# Contributing to Verti

Thanks for taking the time to contribute. This project stays small and
readable on purpose, so contributions that fit its existing style are
easiest to review and merge.

## Before you start

For anything beyond a small fix, open an issue first to discuss the
approach. It saves everyone time if a design question gets settled before
code is written.

## Project layout

- `cmd/verti` - the entry point.
- `internal/app` - the bubbletea application: input handling, keymap,
  view rendering, tabs, search, and so on.
- `pkg/*` - independently testable packages (`buffer`, `highlight`,
  `explorer`, `terminal`, `luaengine`, `search`, `session`, `gitstatus`,
  `paint`, `complete`, `display`, `ignore`). Each one should be usable
  and testable without importing `internal/app`.

## Setting up

Requires Go 1.25+ and a cgo-capable C compiler (Zig's `zig cc` works well
on any OS; gcc/clang/MSVC work too).

```sh
git clone https://github.com/dayanez/Verti
cd Verti
make build
make test
```

On Windows, use `.\scripts\dev.ps1 build` and `.\scripts\dev.ps1 test`.

## Code style

Follow standard Go conventions:

- Run `gofmt` (or `go vet ./...`, which the build already requires) before
  committing. Unformatted code will not be merged.
- Keep exported identifiers documented with a doc comment starting with
  the identifier's name, per [Effective Go](https://go.dev/doc/effective_go#commentary).
- Prefer small, focused functions and packages over large ones. If a
  package starts doing two unrelated things, it should probably be two
  packages.
- Handle errors explicitly; don't discard them silently unless there's a
  clear, commented reason.
- Avoid introducing new dependencies unless the alternative is
  substantial code to hand-roll. This project favors a small, auditable
  dependency tree.
- No em dashes in code comments, commit messages, documentation, or PR
  descriptions. Use a period, comma, or parenthetical instead.

## Tests

New behavior needs a test. `pkg/*` packages are unit-tested directly;
`internal/app` behavior is generally tested through the packages it
composes. Run the full suite with `make test` before opening a PR.

## Commit messages

Write a short, descriptive summary line, and use the body to explain why
a change was made when that isn't obvious from the diff. Reference the
issue number being fixed if there is one.

## Pull requests

- Keep PRs focused on one change. Unrelated cleanup should be its own PR.
- Make sure `go build ./...`, `go vet ./...`, and `go test ./...` all pass
  before requesting review; CI runs the same checks.
- Describe what changed and why in the PR description.

## Reporting bugs

Include your OS, the Verti version (or commit hash if built from source),
steps to reproduce, and what you expected to happen versus what actually
happened.
