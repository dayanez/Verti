package highlight

import "bytes"

// JSONHighlighter is a small hand-written tokenizer for JSON. It exists
// because tree-sitter-json has no bundled Go grammar package alongside the
// other languages here; JSON's structure is simple enough that a direct
// scan is both accurate and far less code than vendoring a grammar. It
// satisfies the same Highlighter interface as the tree-sitter-backed
// implementations, so callers never need to know the difference.
type JSONHighlighter struct {
	prevSrc    []byte
	prevResult []Token
}

func NewJSONHighlighter() *JSONHighlighter { return &JSONHighlighter{} }

// Highlight ignores viewStart/viewEnd and always tokenizes the whole
// file: JSON files are rarely huge, and this is a single cheap linear
// scan rather than a tree-sitter parse, so there's nothing worth limiting.
// If src is unchanged since the last call, it returns the cached result
// without rescanning at all -- the same cheap-insurance idiom the
// tree-sitter-backed highlighters use (see incrementalParser.parse) --
// since View() re-renders, and therefore re-highlights, on every frame,
// including ones triggered by something unrelated to this buffer (cursor
// movement, terminal output streaming in a different pane, ...).
func (h *JSONHighlighter) Highlight(src []byte, viewStart, viewEnd int) ([]Token, error) {
	if bytes.Equal(h.prevSrc, src) {
		return h.prevResult, nil
	}

	var out []Token
	i := 0
	n := len(src)

	// expectKey tracks whether the next string literal is an object key
	// (colors as KindType) versus a value (colors as KindString). It is
	// purely positional: true right after '{' or ',' at object-key position.
	expectKey := false
	keyStack := []bool{}

	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '{':
			keyStack = append(keyStack, expectKey)
			expectKey = true
			i++
		case c == '[':
			keyStack = append(keyStack, expectKey)
			expectKey = false
			i++
		case c == '}' || c == ']':
			if len(keyStack) > 0 {
				expectKey = keyStack[len(keyStack)-1]
				keyStack = keyStack[:len(keyStack)-1]
			}
			i++
		case c == ':':
			expectKey = false
			i++
		case c == ',':
			if len(keyStack) > 0 && !keyStack[len(keyStack)-1] {
				// inside an array, commas don't toggle key/value
			} else {
				expectKey = true
			}
			i++
		case c == '"':
			start := i
			i++
			for i < n && src[i] != '"' {
				if src[i] == '\\' && i+1 < n {
					i++
				}
				i++
			}
			if i < n {
				i++ // closing quote
			}
			kind := KindString
			if expectKey {
				kind = KindType
			}
			out = append(out, Token{StartByte: start, EndByte: i, Kind: kind})
		case c == '-' || (c >= '0' && c <= '9'):
			start := i
			i++
			for i < n && isJSONNumberByte(src[i]) {
				i++
			}
			out = append(out, Token{StartByte: start, EndByte: i, Kind: KindNumber})
		case matchLiteral(src[i:], "true") || matchLiteral(src[i:], "false") || matchLiteral(src[i:], "null"):
			start := i
			for _, w := range []string{"true", "false", "null"} {
				if matchLiteral(src[i:], w) {
					i += len(w)
					break
				}
			}
			out = append(out, Token{StartByte: start, EndByte: i, Kind: KindConstant})
		default:
			if isJSONPunct(c) {
				out = append(out, Token{StartByte: i, EndByte: i + 1, Kind: KindPunctuation})
			}
			i++
		}
	}
	h.prevSrc = append(h.prevSrc[:0], src...)
	h.prevResult = out
	return out, nil
}

// ---------------- Scan helpers ----------------

func isJSONNumberByte(b byte) bool {
	return (b >= '0' && b <= '9') || b == '.' || b == 'e' || b == 'E' || b == '+' || b == '-'
}

func isJSONPunct(b byte) bool {
	switch b {
	case '{', '}', '[', ']', ':', ',':
		return true
	}
	return false
}

func matchLiteral(s []byte, word string) bool {
	if len(s) < len(word) {
		return false
	}
	return string(s[:len(word)]) == word
}
