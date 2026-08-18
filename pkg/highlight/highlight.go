// Package highlight turns source text into styled token spans. It defines
// a small, UI-agnostic interface (Highlighter) so the rendering package
// never needs to know whether a given language is highlighted via a real
// tree-sitter AST walk or a hand-written tokenizer — both implement the
// same contract and are selected by file extension through a Registry.
package highlight

import "strings"

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

// Highlighter classifies the contents of a single source file. A
// Highlighter is not required to be safe for concurrent use; callers
// (the Registry included) serialize calls per instance.
type Highlighter interface {
	Highlight(src []byte) ([]Token, error)
}

// Registry resolves a Highlighter by file extension (including the leading
// dot, e.g. ".go"), lazily constructing and caching one instance per
// extension.
type Registry struct {
	factories map[string]func() Highlighter
	instances map[string]Highlighter
}

// NewRegistry returns a Registry pre-populated with the editor's default
// language support: real tree-sitter AST highlighting for Go, Python,
// JavaScript/JSX, TypeScript/TSX, Rust, C/C++, Lua and Markdown, plus a
// small hand-written tokenizer for JSON (tree-sitter-json has no bundled
// Go grammar package, so it isn't part of the cgo path).
func NewRegistry() *Registry {
	r := &Registry{
		factories: make(map[string]func() Highlighter),
		instances: make(map[string]Highlighter),
	}
	registerTreeSitterLanguages(r)
	r.Register(".json", func() Highlighter { return NewJSONHighlighter() })
	return r
}

// Register associates a file extension with a Highlighter constructor.
// Registering the same extension twice replaces the previous entry.
func (r *Registry) Register(ext string, factory func() Highlighter) {
	r.factories[strings.ToLower(ext)] = factory
	delete(r.instances, strings.ToLower(ext))
}

// For returns the Highlighter registered for a filename's extension, and
// whether one was found.
func (r *Registry) For(filename string) (Highlighter, bool) {
	ext := extOf(filename)
	if h, ok := r.instances[ext]; ok {
		return h, true
	}
	factory, ok := r.factories[ext]
	if !ok {
		return nil, false
	}
	h := factory()
	r.instances[ext] = h
	return h, true
}

func extOf(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(filename[i:])
}
