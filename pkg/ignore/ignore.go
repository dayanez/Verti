// Package ignore implements a small .gitignore matcher shared by the file
// explorer (pkg/explorer) and project-wide search (pkg/search), so both
// features treat "what counts as noise in this workspace" the same way:
// read once from the workspace root's .gitignore, applied by both features
// to skip node_modules/build output/etc. entirely rather than showing or
// searching it and filtering the result afterward.
package ignore

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// rule is one parsed line of a .gitignore file.
type rule struct {
	pattern  string // glob pattern, ready for path.Match
	anchored bool   // had a "/" other than a single trailing one: only matches from the workspace root, not at any depth
	dirOnly  bool   // had a trailing "/": only matches directories
	negate   bool   // had a leading "!": a later match re-includes a path an earlier rule excluded
}

// Matcher filters a workspace tree using its root .gitignore. Only the
// root .gitignore is read, not one merged from every directory down the
// tree: that covers the overwhelming majority of real projects without
// the complexity of a full per-directory merge, and a missing .gitignore
// just means nothing is filtered.
//
// It intentionally doesn't support "**" globs; gitignore's fuller pattern
// language is a lot of machinery for a filter that mainly needs to
// recognize plain names and simple wildcards, and path.Match's
// single-level "*" already covers what real-world .gitignore files
// overwhelmingly use.
type Matcher struct {
	rules []rule
}

// Load reads root's .gitignore, if any, and returns a Matcher for it. A
// missing .gitignore is not an error: the returned Matcher just matches
// nothing.
func Load(root string) *Matcher {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return &Matcher{}
	}
	m := &Matcher{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var r rule
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		if strings.HasPrefix(line, "/") {
			r.anchored = true
			line = strings.TrimPrefix(line, "/")
		} else if strings.Contains(line, "/") {
			// A slash anywhere but the end also anchors the pattern to the
			// .gitignore's directory, per gitignore's own rules.
			r.anchored = true
		}
		if line == "" {
			continue
		}
		r.pattern = line
		m.rules = append(m.rules, r)
	}
	return m
}

// Match reports whether relPath (slash-separated, relative to the
// workspace root) should be treated as ignored. Rules are applied in file
// order so a later "!keep-this" can override an earlier broad ignore,
// matching git's own last-match-wins semantics.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	if m == nil {
		return false
	}
	ignored := false
	base := path.Base(relPath)
	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		var matched bool
		if r.anchored {
			matched, _ = path.Match(r.pattern, relPath)
		} else {
			matched, _ = path.Match(r.pattern, base)
		}
		if matched {
			ignored = !r.negate
		}
	}
	return ignored
}

// RelSlash returns fullPath relative to root, with forward slashes
// regardless of OS, matching the separator .gitignore patterns are always
// written with.
func RelSlash(root, fullPath string) string {
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return filepath.ToSlash(fullPath)
	}
	return filepath.ToSlash(rel)
}
