package highlight

import "testing"

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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
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
	tokens, err := h.Highlight(src)
	if err != nil {
		t.Fatalf("Highlight() error: %v", err)
	}
	findKind(t, tokens, src, "FROM", KindKeyword)
}

func TestMarkdownHighlighting(t *testing.T) {
	src := []byte("# Title\n\nSome `code` and a [link](https://example.com).\n")
	r := NewRegistry()
	h, _ := r.For("README.md")
	tokens, err := h.Highlight(src)
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
