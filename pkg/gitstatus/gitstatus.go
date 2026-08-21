// Package gitstatus reads a workspace's git status (current branch, and
// which paths have uncommitted changes) by shelling out to the git CLI --
// the same one the embedded terminal already assumes is on the user's
// PATH -- rather than vendoring a git implementation. It exists so the
// explorer and status bar can show ambient awareness (a branch name, a
// colored dot on a changed file) without the user ever opening a diff
// view or a dedicated git panel.
package gitstatus

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// Status is a workspace's git status as of the last Load call.
type Status struct {
	// Branch is the current branch name, or "" if root isn't a git
	// repository, has no commits yet, or git isn't installed.
	Branch string
	// Dirty maps a workspace-relative, slash-separated path to its
	// porcelain status code (one of 'M' modified, 'A' added, 'D'
	// deleted, 'R' renamed, '?' untracked) for every path git reports as
	// changed, preferring the worktree (unstaged) status over the index
	// (staged) one when a path has both, since that's the one that
	// reflects what's still unsaved-to-git about a file someone is
	// actively editing.
	Dirty map[string]byte
}

// Load runs `git status --porcelain=v1 --branch` in root and parses its
// output. A root that isn't a git repository, or a machine with no git
// on PATH, isn't an error: Load just returns a zero-value Status (no
// branch, nothing dirty), so callers can treat "not a git repo" the same
// as "nothing to show" rather than a failure worth surfacing.
func Load(root string) *Status {
	st := &Status{Dirty: map[string]byte{}}
	out, err := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--branch").Output()
	if err != nil {
		return st
	}
	prefix := repoRelativePrefix(root)
	for i, line := range strings.Split(string(bytes.TrimRight(out, "\n")), "\n") {
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "## ") {
			st.Branch = parseBranch(line[len("## "):])
			continue
		}
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		code := y
		if code == ' ' {
			code = x
		}
		path := line[3:]
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+len(" -> "):]
		}
		path = unquoteGitPath(path)
		path = strings.TrimPrefix(path, prefix)
		st.Dirty[path] = code
	}
	return st
}

// repoRelativePrefix returns root's path relative to its git repository's
// top level, slash-separated with a trailing slash (e.g. "sub/dir/"), or
// "" if root is itself the top level, isn't a git repository, or git isn't
// on PATH. `git status --porcelain` always reports paths relative to the
// repo's top level, ignoring both `-C` and the status.relativePaths config
// (unlike plain `git status`, which honors both) -- callers key Dirty by
// root-relative paths, so this prefix has to be stripped from every path
// Load parses before the two line up.
func repoRelativePrefix(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

// unquoteGitPath reverses git's C-style quoting of a path in porcelain
// output. core.quotePath (on by default) makes git wrap any path
// containing non-ASCII bytes or other special characters in double
// quotes, escaping each such byte as a backslash-octal sequence (plus the
// usual \\, \", \n, \t, ...), so a file named "café.txt" comes back as
// `"caf\303\251.txt"`. Left as the raw quoted string, it never matches
// the real filesystem path callers key Dirty by. A path with nothing to
// quote is returned unchanged.
func unquoteGitPath(path string) string {
	if len(path) < 2 || path[0] != '"' || path[len(path)-1] != '"' {
		return path
	}
	inner := path[1 : len(path)-1]
	var out []byte
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if c != '\\' || i+1 >= len(inner) {
			out = append(out, c)
			continue
		}
		i++
		switch e := inner[i]; e {
		case '\\', '"':
			out = append(out, e)
		case 'a':
			out = append(out, '\a')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'v':
			out = append(out, '\v')
		default:
			if e >= '0' && e <= '7' && i+2 < len(inner) {
				if n, err := strconv.ParseUint(inner[i:i+3], 8, 8); err == nil {
					out = append(out, byte(n))
					i += 2
					continue
				}
			}
			out = append(out, '\\', e)
		}
	}
	return string(out)
}

// parseBranch extracts the branch name from porcelain --branch's header
// line (with its leading "## " already stripped), which otherwise looks
// like "main...origin/main [ahead 1]", "main" (no upstream), "HEAD (no
// branch)" (detached HEAD), or "No commits yet on main" (empty repo).
func parseBranch(header string) string {
	header = strings.TrimPrefix(header, "No commits yet on ")
	if header == "HEAD (no branch)" {
		return "HEAD"
	}
	if idx := strings.Index(header, "..."); idx >= 0 {
		header = header[:idx]
	}
	if idx := strings.IndexByte(header, ' '); idx >= 0 {
		header = header[:idx]
	}
	return header
}
