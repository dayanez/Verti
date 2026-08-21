// Package search implements a plain-text search across every file in a
// workspace, for jumping to a match from anywhere rather than only within
// whatever buffer happens to be open.
package search

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dayanez/Verti/pkg/ignore"
)

// ---------------- Types & limits ----------------

// Match is one line, in one file, that contained the search text.
type Match struct {
	// Path is relative to the root Files was called with.
	Path string
	Line int // 1-based
	Text string
}

// maxFileSize skips anything bigger than this: a file that large is
// almost certainly not hand-written source worth searching, and reading
// it whole would be wasteful.
const maxFileSize = 2 << 20 // 2MB

// maxMatches caps how many results Files collects before stopping early,
// so a very common substring in a very large tree doesn't produce an
// unusable, unbounded results list.
const maxMatches = 500

var errStop = errors.New("search: match limit reached")

// ---------------- Traversal ----------------

// walkFiles calls visit for every regular file under root, skipping
// dot-directories (.git and the like) and anything the workspace root's
// .gitignore excludes (node_modules, build output, ...). It's the shared
// traversal behind both Files (which reads and greps each file) and
// ListFiles (which just wants the names, for quick-open); passing the
// already-fetched fs.DirEntry through avoids a second stat syscall per
// file for callers (like Files) that need its size.
func walkFiles(root string, visit func(path, rel string, d fs.DirEntry) error) error {
	ig := ignore.Load(root)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // an unreadable entry shouldn't abort the whole walk
		}
		if path != root && ig.Match(ignore.RelSlash(root, path), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		return visit(path, ignore.RelSlash(root, path), d)
	})
}

// ---------------- Full-text search ----------------

// Files searches every regular file under root for substr, a plain
// case-sensitive substring match (no regex or globbing, keeping results
// predictable), skipping anything walkFiles excludes and any file that's
// too large or looks binary (contains a NUL byte).
func Files(root, substr string) ([]Match, error) {
	if substr == "" {
		return nil, nil
	}

	var out []Match
	err := walkFiles(root, func(path, rel string, d fs.DirEntry) error {
		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		// The scanner's max token size must be at least maxFileSize: a
		// single line can be as long as the whole file (no newlines at
		// all), and data is already capped there by the size check above.
		// A smaller cap here would make the scanner hit bufio.ErrTooLong
		// on a file that already passed that check, silently truncating
		// results partway through with no indication anything was missed.
		scanner.Buffer(make([]byte, 0, 64*1024), maxFileSize)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(text, substr) {
				out = append(out, Match{Path: rel, Line: line, Text: strings.TrimSpace(text)})
				if len(out) >= maxMatches {
					return errStop
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return out, err
	}
	return out, nil
}

// ---------------- File listing ----------------

// ListFiles returns the workspace-relative, slash-separated path of every
// regular file under root that Files would be willing to search (same
// dot-directory and .gitignore filtering), without reading any of their
// contents. Used for quick-open, where all that's needed is the set of
// names to fuzzy-match against.
func ListFiles(root string) ([]string, error) {
	var out []string
	err := walkFiles(root, func(_, rel string, _ fs.DirEntry) error {
		out = append(out, rel)
		return nil
	})
	return out, err
}
