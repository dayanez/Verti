package highlight

import (
	"sort"
	"testing"
)

func findKind(t *testing.T, tokens []Token, src []byte, text string, kind Kind) {
	t.Helper()
	for _, tok := range tokens {
		if string(src[tok.StartByte:tok.EndByte]) == text {
			if tok.Kind != kind {
				t.Errorf("token %q classified as %q, want %q", text, tok.Kind, kind)
			}
			return
		}
	}
	t.Errorf("no token found for %q (want kind %q)", text, kind)
}

func TestRegistryResolvesByExtension(t *testing.T) {
	r := NewRegistry()
	exts := []string{
		".go", ".py", ".js", ".ts", ".tsx", ".rs", ".c", ".cpp", ".lua", ".md", ".json",
		".sh", ".css", ".html", ".yml", ".yaml", ".toml", ".rb", ".java", ".php", ".sql",
		".cs", ".kt", ".swift",
	}
	for _, ext := range exts {
		if _, ok := r.For("file" + ext); !ok {
			t.Errorf("Registry.For(%q) found no highlighter", ext)
		}
	}
	if _, ok := r.For("file.unknownext"); ok {
		t.Error("Registry.For(unknown extension) unexpectedly found a highlighter")
	}
}

func TestRegistryResolvesDockerfileByExactFilename(t *testing.T) {
	r := NewRegistry()
	for _, name := range []string{"Dockerfile", "dockerfile", "DOCKERFILE"} {
		if _, ok := r.For(name); !ok {
			t.Errorf("Registry.For(%q) found no highlighter", name)
		}
	}
}

func TestGoHighlighting(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\t// greet\n\tname := \"world\"\n\tprintln(name)\n}\n")
	r := NewRegistry()
	h, ok := r.For("main.go")
	if !ok {
		t.Fatal("no Go highlighter registered")
	}
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "func", KindKeyword)
	findKind(t, tokens, src, "\"world\"", KindString)
	findKind(t, tokens, src, "// greet", KindComment)
	findKind(t, tokens, src, "println", KindFunction)
}

func TestPythonHighlighting(t *testing.T) {
	src := []byte("def greet(name):\n    return \"hi \" + name\n")
	r := NewRegistry()
	h, _ := r.For("greet.py")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "def", KindKeyword)
	findKind(t, tokens, src, "return", KindKeyword)
	findKind(t, tokens, src, "\"hi \"", KindString)
}

func TestJSONHighlighting(t *testing.T) {
	src := []byte(`{"name": "verti", "version": 1, "stable": true}`)
	h := NewJSONHighlighter()
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, `"name"`, KindType)
	findKind(t, tokens, src, `"verti"`, KindString)
	findKind(t, tokens, src, "1", KindNumber)
	findKind(t, tokens, src, "true", KindConstant)
}

func TestBashHighlighting(t *testing.T) {
	src := []byte("#!/bin/bash\nif [ -f \"$1\" ]; then\n  echo \"found\"\nfi\n")
	r := NewRegistry()
	h, _ := r.For("build.sh")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "if", KindKeyword)
	findKind(t, tokens, src, "\"found\"", KindString)
}

func TestRubyHighlighting(t *testing.T) {
	src := []byte("def greet(name)\n  return \"hi \" + name\nend\n")
	r := NewRegistry()
	h, _ := r.For("greet.rb")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "def", KindKeyword)
	findKind(t, tokens, src, "end", KindKeyword)
	findKind(t, tokens, src, "\"hi \"", KindString)
}

func TestJavaHighlighting(t *testing.T) {
	src := []byte("public class Main {\n  // entry point\n  public static void main(String[] args) {}\n}\n")
	r := NewRegistry()
	h, _ := r.For("Main.java")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "class", KindKeyword)
	findKind(t, tokens, src, "public", KindKeyword)
	findKind(t, tokens, src, "// entry point", KindComment)
}

func TestCSSHighlighting(t *testing.T) {
	src := []byte("/* box */\nbody {\n  color: red;\n}\n")
	r := NewRegistry()
	h, _ := r.For("style.css")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "/* box */", KindComment)
}

func TestDockerfileHighlighting(t *testing.T) {
	src := []byte("FROM golang:1.25\nRUN go build ./...\n")
	r := NewRegistry()
	h, ok := r.For("Dockerfile")
	if !ok {
		t.Fatal("no Dockerfile highlighter registered")
	}
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "FROM", KindKeyword)
}

func TestMarkdownHighlighting(t *testing.T) {
	src := []byte("# Title\n\nSome `code` and a [link](https://example.com).\n")
	r := NewRegistry()
	h, _ := r.For("README.md")
	tokens, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	if len(tokens) == 0 {
		t.Fatal("expected at least one markdown token")
	}
	var sawHeading bool
	for _, tok := range tokens {
		if tok.Kind == KindHeading {
			sawHeading = true
		}
	}
	if !sawHeading {
		t.Error("expected a KindHeading token for the '# Title' line")
	}
}

// tokensMatch reports whether a and b classify the same byte spans the
// same way, ignoring order (collectNodes' recursion order can legitimately
// differ between an incremental and a from-scratch parse of otherwise
// identical trees).
func tokensMatch(a, b []Token) bool {
	if len(a) != len(b) {
		return false
	}
	sortByRange := func(toks []Token) []Token {
		out := append([]Token(nil), toks...)
		sort.Slice(out, func(i, j int) bool {
			if out[i].StartByte != out[j].StartByte {
				return out[i].StartByte < out[j].StartByte
			}
			return out[i].EndByte < out[j].EndByte
		})
		return out
	}
	as, bs := sortByRange(a), sortByRange(b)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// TestIncrementalHighlightMatchesFreshParse drives the same Highlighter
// instance through a sequence of edits (as real typing would) and checks
// its output after each one against a brand-new Highlighter given the
// same source from scratch. This is what actually verifies the
// incremental-reparse path (see incrementalParser.parse) produces correct
// tokens, not just that it runs without erroring.
func TestIncrementalHighlightMatchesFreshParse(t *testing.T) {
	reg := NewRegistry()
	stateful, ok := reg.For("main.go")
	if !ok {
		t.Fatal("no Go highlighter registered")
	}

	edits := []string{
		"package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n",
		"package main\n\nfunc main() {\n\tprintln(\"hi\")\n\tx := 1\n}\n",
		"package main\n\nfunc main() {\n\tprintln(\"hi\")\n\tx := 1\n\ty := 2\n}\n",
		"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tprintln(\"hi\")\n\tx := 1\n\ty := 2\n}\n",
		"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n\tx := 1\n\ty := 2\n}\n",
		"package main\n\nimport \"fmt\"\n\nfunc helper() int { return 42 }\n\nfunc main() {\n\tfmt.Println(\"hi\")\n\tx := 1\n\ty := 2\n}\n",
	}

	for step, src := range edits {
		got, err := stateful.Highlight([]byte(src), 0, len(src))
		if err != nil {
			t.Fatalf("step %d: incremental Highlight() error: %v", step, err)
		}

		fresh, ok := reg.For("main.go")
		if !ok {
			t.Fatal("no Go highlighter registered")
		}
		want, err := fresh.Highlight([]byte(src), 0, len(src))
		if err != nil {
			t.Fatalf("step %d: fresh Highlight() error: %v", step, err)
		}

		if !tokensMatch(got, want) {
			t.Fatalf("step %d: incremental tokens don't match a fresh parse\nincremental: %+v\nfresh:       %+v", step, got, want)
		}
	}
}

// TestViewportLimitingExcludesOutOfRangeTokensButKeepsOverlapping checks
// that restricting Highlight to a byte range prunes tokens entirely
// outside it while still including a token that merely overlaps the
// range's edge (e.g. a multi-line string starting before the viewport and
// extending into it) -- the case a naive "does the token start inside the
// range" check would get wrong.
func TestViewportLimitingExcludesOutOfRangeTokensButKeepsOverlapping(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tx := `line one\nline two\nline three`\n\tprintln(x)\n}\n")
	reg := NewRegistry()
	h, _ := reg.For("main.go")

	full, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	var wantMultiline Token
	found := false
	for _, tok := range full {
		if string(src[tok.StartByte:tok.EndByte]) == "`line one\nline two\nline three`" {
			wantMultiline = tok
			found = true
		}
	}
	if !found {
		t.Fatal("setup: couldn't find the multi-line backtick string in a full-file parse")
	}

	// A viewport landing in the middle of "line two", well after the
	// string's StartByte but before its EndByte.
	viewStart := wantMultiline.StartByte + len("`line one\nline")
	viewEnd := viewStart + 1

	h2, _ := reg.For("main.go")
	limited, err := h2.Highlight(src, viewStart, viewEnd)
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}

	var gotMultiline bool
	for _, tok := range limited {
		if tok == wantMultiline {
			gotMultiline = true
		}
		if tok.EndByte <= viewStart || tok.StartByte >= viewEnd {
			t.Errorf("token %+v (%q) doesn't overlap the requested range [%d,%d) at all, want it pruned",
				tok, src[tok.StartByte:tok.EndByte], viewStart, viewEnd)
		}
	}
	if !gotMultiline {
		t.Error("the multi-line string overlapping the viewport's edge was pruned entirely, want it kept")
	}

	// Sanity check: a comment far outside the viewport must not appear.
	src2 := []byte("// nowhere near the viewport\n" + string(src))
	h3, _ := reg.For("main.go")
	limited2, err := h3.Highlight(src2, len(src2)-5, len(src2))
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	for _, tok := range limited2 {
		if string(src2[tok.StartByte:tok.EndByte]) == "// nowhere near the viewport" {
			t.Error("a comment entirely outside the requested range was returned, want it pruned")
		}
	}
}

// TestHighlightWithUnchangedSourceReturnsSameTokens exercises the
// incrementalParser's no-op shortcut (identical source since the last
// call, e.g. a render triggered by moving the cursor rather than editing)
// and checks it doesn't error and returns the same tokens as a fresh parse.
func TestHighlightWithUnchangedSourceReturnsSameTokens(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	reg := NewRegistry()
	h, _ := reg.For("main.go")

	first, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("first Highlight() error: %v", err)
	}
	second, err := h.Highlight(src, 0, len(src))
	if err != nil {
		t.Fatalf("second (unchanged) Highlight() error: %v", err)
	}
	if !tokensMatch(first, second) {
		t.Fatalf("tokens changed across an unchanged-source call\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

// TestRegistryForReturnsFreshInstancesNotShared checks that two calls to
// For with the same extension return independent Highlighter instances,
// not a shared one: sharing would corrupt incremental parse state when
// two different open files use the same language (a real scenario now
// that multiple tabs can be open at once).
func TestRegistryForReturnsFreshInstancesNotShared(t *testing.T) {
	reg := NewRegistry()
	a, _ := reg.For("a.go")
	b, _ := reg.For("b.go")

	srcA := []byte("package main\n\nfunc a() {}\n")
	srcB := []byte("package main\n\nfunc totallyDifferentAndLonger() { println(1, 2, 3) }\n")

	if _, err := a.Highlight(srcA, 0, len(srcA)); err != nil {
		t.Fatalf("a.Highlight() error: %v", err)
	}
	// If a and b were the same underlying instance, this call's diff
	// against srcA would compute a bogus edit region and could panic or
	// silently misclassify tokens.
	tokensB, err := b.Highlight(srcB, 0, len(srcB))
	if err != nil {
		t.Fatalf("b.Highlight() error: %v", err)
	}
	findKind(t, tokensB, srcB, "func", KindKeyword)
	findKind(t, tokensB, srcB, "totallyDifferentAndLonger", KindFunction)
}
