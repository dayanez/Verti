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
		st.Dirty[strings.Trim(path, `"`)] = code
	}
	return st
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
