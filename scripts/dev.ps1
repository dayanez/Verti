# Dev helper for Windows: wires up zig as the cgo C compiler (needed for
# the tree-sitter grammars) and runs common Go tasks.
#
# Usage: .\scripts\dev.ps1 [build|test|vet|run|tidy]

param(
    [Parameter(Position = 0)]
    [ValidateSet("build", "test", "vet", "run", "tidy")]
    [string]$Task = "build"
)

if (-not (Get-Command zig -ErrorAction SilentlyContinue)) {
    Write-Error "zig not found on PATH. Install it with 'winget install zig.zig', then restart your terminal. verti uses cgo for tree-sitter, and zig's bundled 'zig cc' is the simplest cross-platform C compiler for that -- see README.md."
    exit 1
}

$env:CGO_ENABLED = "1"
$env:CC = "zig cc"
$env:CXX = "zig c++"

switch ($Task) {
    "build" { go build -o bin/verti.exe ./cmd/verti }
    "test" { go test ./... }
    "vet" { go vet ./... }
    "run" {
        go build -o bin/verti.exe ./cmd/verti
        if ($?) { & .\bin\verti.exe }
    }
    "tidy" { go mod tidy }
}
