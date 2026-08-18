package highlight

import (
	"context"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/lua"
	tsmarkdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	tstypescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// registerTreeSitterLanguages wires every bundled tree-sitter grammar into
// r under its conventional file extensions.
func registerTreeSitterLanguages(r *Registry) {
	r.Register(".go", func() Highlighter { return newASTHighlighter(golang.GetLanguage(), goKeywords) })
	r.Register(".py", func() Highlighter { return newASTHighlighter(python.GetLanguage(), pythonKeywords) })
	r.Register(".js", func() Highlighter { return newASTHighlighter(javascript.GetLanguage(), jsTSKeywords) })
	r.Register(".jsx", func() Highlighter { return newASTHighlighter(javascript.GetLanguage(), jsTSKeywords) })
	r.Register(".ts", func() Highlighter { return newASTHighlighter(tstypescript.GetLanguage(), jsTSKeywords) })
	r.Register(".tsx", func() Highlighter { return newASTHighlighter(tstypescript.GetLanguage(), jsTSKeywords) })
	r.Register(".rs", func() Highlighter { return newASTHighlighter(rust.GetLanguage(), rustKeywords) })
	r.Register(".c", func() Highlighter { return newASTHighlighter(c.GetLanguage(), cCppKeywords) })
	r.Register(".h", func() Highlighter { return newASTHighlighter(c.GetLanguage(), cCppKeywords) })
	r.Register(".cpp", func() Highlighter { return newASTHighlighter(cpp.GetLanguage(), cCppKeywords) })
	r.Register(".cc", func() Highlighter { return newASTHighlighter(cpp.GetLanguage(), cCppKeywords) })
	r.Register(".hpp", func() Highlighter { return newASTHighlighter(cpp.GetLanguage(), cCppKeywords) })
	r.Register(".lua", func() Highlighter { return newASTHighlighter(lua.GetLanguage(), luaKeywords) })
	r.Register(".md", func() Highlighter { return newMarkdownHighlighter() })
	r.Register(".markdown", func() Highlighter { return newMarkdownHighlighter() })
}

// astHighlighter classifies tokens by walking the leaves of a tree-sitter
// parse tree, using each leaf's grammar node type (and, for identifiers,
// its immediate parent's node type) rather than a regex over raw text.
// That AST context is what lets it tell a function name apart from a plain
// variable, or a keyword apart from an identically-spelled string — things
// a regex-based highlighter cannot do reliably.
type astHighlighter struct {
	parser   *sitter.Parser
	keywords map[string]Kind
}

func newASTHighlighter(lang *sitter.Language, keywords map[string]Kind) *astHighlighter {
	p := sitter.NewParser()
	p.SetLanguage(lang)
	return &astHighlighter{parser: p, keywords: keywords}
}

func (h *astHighlighter) Highlight(src []byte) ([]Token, error) {
	tree, err := h.parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var out []Token
	collectNodes(tree.RootNode(), h.keywords, &out)
	return out, nil
}

// collectNodes walks the tree top-down. Unnamed (literal/punctuation)
// nodes are always leaves and classified directly. Named nodes are
// classified as a whole span where possible — e.g. a Go
// interpreted_string_literal has two unnamed '"' children, and we want
// the whole quoted span colored as a string rather than recursing into
// just its quote marks — and only recursed into when no whole-node
// classification applies, so nested structure (call args, block bodies,
// etc.) still gets visited.
func collectNodes(node *sitter.Node, keywords map[string]Kind, out *[]Token) {
	if node == nil {
		return
	}
	if !node.IsNamed() {
		t := node.Type()
		if kind, ok := keywords[t]; ok {
			*out = append(*out, Token{StartByte: int(node.StartByte()), EndByte: int(node.EndByte()), Kind: kind})
		} else if isPunctuationText(t) {
			*out = append(*out, Token{StartByte: int(node.StartByte()), EndByte: int(node.EndByte()), Kind: KindPunctuation})
		}
		return
	}
	if kind := classifyNamed(node); kind != "" {
		*out = append(*out, Token{StartByte: int(node.StartByte()), EndByte: int(node.EndByte()), Kind: kind})
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectNodes(node.Child(i), keywords, out)
	}
}

func classifyNamed(node *sitter.Node) Kind {
	t := node.Type()
	switch {
	case strings.Contains(t, "comment"):
		return KindComment
	case strings.Contains(t, "string"), strings.Contains(t, "char_literal"), t == "char":
		return KindString
	case numericLeafTypes[t]:
		return KindNumber
	case strings.Contains(t, "type_identifier"), t == "primitive_type", t == "type":
		return KindType
	case t == "identifier" || strings.Contains(t, "identifier"):
		if parent := node.Parent(); parent != nil {
			pt := parent.Type()
			if strings.Contains(pt, "call") || strings.Contains(pt, "function") || strings.Contains(pt, "method") {
				return KindFunction
			}
		}
		return KindVariable
	}
	return ""
}

// isPunctuationText reports whether an unnamed leaf's literal text is made
// up entirely of operator/punctuation characters (e.g. "+", "==", "{").
func isPunctuationText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("+-*/%=<>!&|^~?:;,.()[]{}@#$", r) {
			return false
		}
	}
	return true
}

var numericLeafTypes = map[string]bool{
	"int_literal": true, "integer_literal": true, "float_literal": true,
	"number": true, "number_literal": true, "decimal_literal": true,
	"hex_literal": true, "octal_literal": true, "imaginary_literal": true,
}

var goKeywords = map[string]Kind{
	"func": KindKeyword, "package": KindKeyword, "import": KindKeyword, "return": KindKeyword,
	"if": KindKeyword, "else": KindKeyword, "for": KindKeyword, "range": KindKeyword,
	"switch": KindKeyword, "case": KindKeyword, "default": KindKeyword, "break": KindKeyword,
	"continue": KindKeyword, "go": KindKeyword, "defer": KindKeyword, "chan": KindKeyword,
	"select": KindKeyword, "var": KindKeyword, "const": KindKeyword, "type": KindKeyword,
	"struct": KindKeyword, "interface": KindKeyword, "map": KindKeyword, "fallthrough": KindKeyword,
	"goto": KindKeyword, "nil": KindConstant, "true": KindConstant, "false": KindConstant, "iota": KindConstant,
}

var pythonKeywords = map[string]Kind{
	"def": KindKeyword, "return": KindKeyword, "if": KindKeyword, "elif": KindKeyword, "else": KindKeyword,
	"for": KindKeyword, "while": KindKeyword, "in": KindKeyword, "import": KindKeyword, "from": KindKeyword,
	"as": KindKeyword, "class": KindKeyword, "try": KindKeyword, "except": KindKeyword, "finally": KindKeyword,
	"raise": KindKeyword, "with": KindKeyword, "lambda": KindKeyword, "pass": KindKeyword, "break": KindKeyword,
	"continue": KindKeyword, "global": KindKeyword, "nonlocal": KindKeyword, "yield": KindKeyword,
	"async": KindKeyword, "await": KindKeyword, "not": KindKeyword, "and": KindKeyword, "or": KindKeyword,
	"is": KindKeyword, "del": KindKeyword, "assert": KindKeyword,
	"None": KindConstant, "True": KindConstant, "False": KindConstant,
}

var jsTSKeywords = map[string]Kind{
	"function": KindKeyword, "return": KindKeyword, "if": KindKeyword, "else": KindKeyword, "for": KindKeyword,
	"while": KindKeyword, "do": KindKeyword, "switch": KindKeyword, "case": KindKeyword, "default": KindKeyword,
	"break": KindKeyword, "continue": KindKeyword, "var": KindKeyword, "let": KindKeyword, "const": KindKeyword,
	"class": KindKeyword, "extends": KindKeyword, "new": KindKeyword, "try": KindKeyword, "catch": KindKeyword,
	"finally": KindKeyword, "throw": KindKeyword, "typeof": KindKeyword, "instanceof": KindKeyword, "in": KindKeyword,
	"of": KindKeyword, "import": KindKeyword, "export": KindKeyword, "from": KindKeyword, "as": KindKeyword,
	"async": KindKeyword, "await": KindKeyword, "yield": KindKeyword, "interface": KindKeyword, "type": KindKeyword,
	"enum": KindKeyword, "implements": KindKeyword, "public": KindKeyword, "private": KindKeyword,
	"protected": KindKeyword, "readonly": KindKeyword, "static": KindKeyword,
	"null": KindConstant, "undefined": KindConstant, "true": KindConstant, "false": KindConstant, "this": KindConstant,
}

var rustKeywords = map[string]Kind{
	"fn": KindKeyword, "let": KindKeyword, "mut": KindKeyword, "return": KindKeyword, "if": KindKeyword,
	"else": KindKeyword, "match": KindKeyword, "for": KindKeyword, "while": KindKeyword, "loop": KindKeyword,
	"break": KindKeyword, "continue": KindKeyword, "struct": KindKeyword, "enum": KindKeyword, "impl": KindKeyword,
	"trait": KindKeyword, "pub": KindKeyword, "use": KindKeyword, "mod": KindKeyword, "crate": KindKeyword,
	"self": KindKeyword, "Self": KindType, "async": KindKeyword, "await": KindKeyword, "move": KindKeyword,
	"where": KindKeyword, "as": KindKeyword, "in": KindKeyword, "dyn": KindKeyword, "unsafe": KindKeyword,
	"const": KindKeyword, "static": KindKeyword,
	"true": KindConstant, "false": KindConstant, "None": KindConstant, "Some": KindConstant,
}

var cCppKeywords = map[string]Kind{
	"if": KindKeyword, "else": KindKeyword, "for": KindKeyword, "while": KindKeyword, "do": KindKeyword,
	"switch": KindKeyword, "case": KindKeyword, "default": KindKeyword, "break": KindKeyword, "continue": KindKeyword,
	"return": KindKeyword, "struct": KindKeyword, "union": KindKeyword, "enum": KindKeyword, "typedef": KindKeyword,
	"static": KindKeyword, "const": KindKeyword, "volatile": KindKeyword, "extern": KindKeyword, "sizeof": KindKeyword,
	"void": KindType, "int": KindType, "char": KindType, "float": KindType, "double": KindType,
	"long": KindType, "short": KindType, "unsigned": KindType, "signed": KindType, "bool": KindType,
	"class": KindKeyword, "public": KindKeyword, "private": KindKeyword, "protected": KindKeyword,
	"namespace": KindKeyword, "template": KindKeyword, "virtual": KindKeyword, "new": KindKeyword, "delete": KindKeyword,
	"this": KindConstant, "nullptr": KindConstant, "true": KindConstant, "false": KindConstant, "NULL": KindConstant,
}

var luaKeywords = map[string]Kind{
	"function": KindKeyword, "local": KindKeyword, "return": KindKeyword, "if": KindKeyword, "then": KindKeyword,
	"else": KindKeyword, "elseif": KindKeyword, "end": KindKeyword, "for": KindKeyword, "while": KindKeyword,
	"do": KindKeyword, "repeat": KindKeyword, "until": KindKeyword, "break": KindKeyword, "in": KindKeyword,
	"and": KindKeyword, "or": KindKeyword, "not": KindKeyword,
	"nil": KindConstant, "true": KindConstant, "false": KindConstant,
}

// markdownHighlighter classifies whole composite nodes (headings, code
// spans, emphasis, links) rather than leaves, since markdown's grammar
// wraps meaningful spans in container nodes instead of single tokens.
type markdownHighlighter struct {
	parser *sitter.Parser
}

func newMarkdownHighlighter() *markdownHighlighter {
	p := sitter.NewParser()
	p.SetLanguage(tsmarkdown.GetLanguage())
	return &markdownHighlighter{parser: p}
}

var markdownNodeKinds = map[string]Kind{
	"atx_heading": KindHeading, "setext_heading": KindHeading,
	"fenced_code_block": KindString, "indented_code_block": KindString, "code_span": KindString,
	"emphasis": KindKeyword, "strong_emphasis": KindKeyword,
	"link": KindFunction, "inline_link": KindFunction, "image": KindFunction,
	"block_quote": KindComment, "html_block": KindComment,
}

func (h *markdownHighlighter) Highlight(src []byte) ([]Token, error) {
	tree, err := h.parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var out []Token
	collectMarkdown(tree.RootNode(), &out)
	return out, nil
}

func collectMarkdown(node *sitter.Node, out *[]Token) {
	if node == nil {
		return
	}
	if kind, ok := markdownNodeKinds[node.Type()]; ok {
		*out = append(*out, Token{StartByte: int(node.StartByte()), EndByte: int(node.EndByte()), Kind: kind})
		return // don't recurse into a node we've already colored as a whole
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectMarkdown(node.Child(i), out)
	}
}
