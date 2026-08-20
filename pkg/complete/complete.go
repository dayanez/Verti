// Package complete implements simple buffer-local word completion:
// finding other identifiers already present in a buffer that share the
// prefix currently being typed, the same trick Vim's Ctrl+N and Emacs's
// dabbrev-expand use. No language server, no semantic understanding --
// just "this word appears elsewhere in what you're already looking at,"
// which is most of what completion is reached for in practice and costs
// nothing to keep running.
package complete

import "unicode"

// IsWordRune reports whether r can be part of an identifier: a letter, a
// digit, or underscore, which covers identifiers in virtually every
// language Verti highlights.
func IsWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// PrefixStart scans backward from offset (a rune offset into text) over
// identifier runes and returns the start of that run, the bounds of the
// word currently being typed at the cursor.
func PrefixStart(text []rune, offset int) int {
	i := offset
	for i > 0 && IsWordRune(text[i-1]) {
		i--
	}
	return i
}

// Candidates returns every distinct identifier in text that starts with
// prefix and is strictly longer than it (so a word equal to prefix itself
// is never offered as its own completion), excluding the occurrence at
// [prefixStart, prefixEnd) -- the word currently being typed -- ordered
// by first appearance in text.
func Candidates(text []rune, prefix string, prefixStart, prefixEnd int) []string {
	if prefix == "" {
		return nil
	}
	pr := []rune(prefix)
	seen := make(map[string]bool)
	var out []string
	i := 0
	for i < len(text) {
		if !IsWordRune(text[i]) {
			i++
			continue
		}
		start := i
		for i < len(text) && IsWordRune(text[i]) {
			i++
		}
		if start == prefixStart && i == prefixEnd {
			continue // the occurrence currently being typed
		}
		if i-start <= len(pr) || !hasPrefix(text[start:i], pr) {
			continue
		}
		word := string(text[start:i])
		if !seen[word] {
			seen[word] = true
			out = append(out, word)
		}
	}
	return out
}

func hasPrefix(word, prefix []rune) bool {
	if len(word) < len(prefix) {
		return false
	}
	for i, r := range prefix {
		if word[i] != r {
			return false
		}
	}
	return true
}
