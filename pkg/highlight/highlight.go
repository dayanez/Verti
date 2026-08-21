// Package highlight turns source text into styled token spans. It defines
// a small, UI-agnostic interface (Highlighter) so the rendering package
// never needs to know whether a given language is highlighted via a real
// tree-sitter AST walk or a hand-written tokenizer: both implement the
// same contract and are selected by file extension through a Registry.
package highlight

import "strings"

// ---------------- Token model ----------------

// Kind is a semantic token category. The display package owns the actual
// colors; this package only classifies.
type Kind string

const (
	KindDefault     Kind = "default"
	KindKeyword     Kind = "keyword"
	KindString      Kind = "string"
	KindComment     Kind = "comment"
	KindNumber      Kind = "number"
	KindFunction    Kind = "function"
	KindType        Kind = "type"
	KindVariable    Kind = "variable"
	KindOperator    Kind = "operator"
	KindPunctuation Kind = "punctuation"
	KindConstant    Kind = "constant"
	KindHeading     Kind = "heading"
)

// Token is a byte-offset span of src classified as Kind. Spans not covered
// by any returned Token should be rendered as KindDefault.
type Token struct {
	StartByte int
	EndByte   int
	Kind      Kind
}

// Highlighter classifies the contents of a single source file, optionally
// restricted to [viewStart, viewEnd) so a caller that only needs to render
// part of a large file (the visible viewport) doesn't pay to classify the
// rest; pass 0, len(src) to request the whole file. Implementations that
// retain state between calls (the tree-sitter-backed ones) use each call's
// source text, diffed against the previous call's, to reparse only the
// edited region instead of the whole file -- which only works if the
// caller reuses the same Highlighter instance across calls for a given
// file. A fresh instance every call defeats it and just costs more. A
// Highlighter is not required to be safe for concurrent use; callers
// serialize calls per instance.
type Highlighter interface {
	Highlight(src []byte, viewStart, viewEnd int) ([]Token, error)
}

// ---------------- Registry ----------------

// Registry resolves a Highlighter constructor by file extension (including
// the leading dot, e.g. ".go"). Unlike a typical registry it does not
// cache instances: each Highlighter now retains its own parse state
// (see the Highlighter doc comment), so a single shared-by-extension
// instance would corrupt itself when two files of the same language are
// open at once. Callers construct one instance per open file (via For)
// and hold onto it themselves for as long as that file stays open.
type Registry struct {
	factories map[string]func() Highlighter
}

// NewRegistry returns a Registry pre-populated with the editor's default
// language support: real tree-sitter AST highlighting for Go, Python,
// JavaScript/JSX, TypeScript/TSX, Rust, C/C++, Lua, Markdown, Bash, CSS,
// HTML, YAML, TOML, Ruby, Java, PHP, SQL, C#, Kotlin, Swift and Dockerfile
// (all bundled by the already-vendored smacker/go-tree-sitter dependency),
// plus a small hand-written tokenizer for JSON (tree-sitter-json has no
// bundled Go grammar package, so it isn't part of the cgo path).
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]func() Highlighter),
	}
	registerTreeSitterLanguages(r)
	r.Register(".json", func() Highlighter { return NewJSONHighlighter() })
	return r
}

// Register associates a file extension with a Highlighter constructor.
// Registering the same extension twice replaces the previous entry.
func (r *Registry) Register(ext string, factory func() Highlighter) {
	r.factories[strings.ToLower(ext)] = factory
}

// For constructs a fresh Highlighter registered for filename, and whether
// one was found. filename is matched two ways: first as a whole lowercase
// name (so extension-less conventions like "Dockerfile" can be registered
// via Register("dockerfile", ...)), then by extension. Call this once per
// opened file and keep the result; see the Highlighter doc comment for why.
func (r *Registry) For(filename string) (Highlighter, bool) {
	if factory, ok := r.factories[strings.ToLower(filename)]; ok {
		return factory(), true
	}
	if factory, ok := r.factories[extOf(filename)]; ok {
		return factory(), true
	}
	return nil, false
}

func extOf(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(filename[i:])
}
