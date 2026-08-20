// Package explorer implements the collapsible left-hand file tree sidebar:
// lazy directory loading, arrow-key navigation, expand/collapse, and
// double-Enter-to-open semantics (a single Enter on a file just marks it
// as "about to open" so a stray keypress while navigating doesn't yank
// focus away from the tree; a second Enter within the window opens it).
package explorer

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dayanez/Verti/pkg/gitstatus"
	"github.com/dayanez/Verti/pkg/ignore"
)

// ErrNothingSelected is returned by Rename and Delete when the tree is
// empty (nothing for them to act on).
var ErrNothingSelected = errors.New("explorer: nothing selected")

const doubleEnterWindow = 500 * time.Millisecond

type node struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	Children []*node
	loaded   bool
	parent   *node
}

type visibleEntry struct {
	node  *node
	depth int
}

// Explorer is the sidebar's state: the loaded directory tree, which rows
// are currently visible (accounting for collapsed folders), and cursor /
// scroll position.
type Explorer struct {
	Root   *node
	Width  int
	Height int

	flat      []visibleEntry
	cursor    int
	scrollTop int

	lastEnterIdx  int
	lastEnterTime time.Time

	ignore *ignore.Matcher
	git    *gitstatus.Status
}

// New loads rootPath as the explorer's workspace root, with its immediate
// children loaded and visible (but not expanded). Entries matched by the
// workspace root's .gitignore (node_modules, build output, ...) are left
// out of the tree entirely, not just visually hidden, so they're never
// read from disk in the first place.
func New(rootPath string) (*Explorer, error) {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	root := &node{Name: filepath.Base(abs), Path: abs, IsDir: true, Expanded: true}
	e := &Explorer{Root: root, lastEnterIdx: -1, ignore: ignore.Load(abs), git: gitstatus.Load(abs)}
	if err := e.loadChildren(root); err != nil {
		return nil, err
	}
	e.rebuildFlat()
	return e, nil
}

func (e *Explorer) loadChildren(n *node) error {
	entries, err := os.ReadDir(n.Path)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
	n.Children = n.Children[:0]
	for _, ent := range entries {
		if ent.Name() == ".git" {
			continue
		}
		entPath := filepath.Join(n.Path, ent.Name())
		if e.ignore.Match(ignore.RelSlash(e.Root.Path, entPath), ent.IsDir()) {
			continue
		}
		n.Children = append(n.Children, &node{
			Name: ent.Name(), Path: entPath, IsDir: ent.IsDir(), parent: n,
		})
	}
	n.loaded = true
	return nil
}

func (e *Explorer) rebuildFlat() {
	e.flat = e.flat[:0]
	var walk func(n *node, depth int)
	walk = func(n *node, depth int) {
		e.flat = append(e.flat, visibleEntry{n, depth})
		if n.IsDir && n.Expanded {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	for _, c := range e.Root.Children {
		walk(c, 0)
	}
	if e.cursor >= len(e.flat) {
		e.cursor = len(e.flat) - 1
	}
	if e.cursor < 0 {
		e.cursor = 0
	}
}

// SetSize sets the sidebar's rendered dimensions.
func (e *Explorer) SetSize(w, h int) { e.Width, e.Height = w, h }

// Branch returns the workspace's current git branch, or "" if it isn't a
// git repository (or git isn't installed).
func (e *Explorer) Branch() string { return e.git.Branch }

// ReloadGitStatus re-reads the workspace's git status (branch, and which
// files are dirty) without re-walking the directory tree. It shells out to
// git and blocks until that returns, so callers driving an interactive event
// loop should prefer running gitstatus.Load themselves on a goroutine and
// feeding the result to SetGitStatus instead of calling this directly.
func (e *Explorer) ReloadGitStatus() { e.git = gitstatus.Load(e.Root.Path) }

// SetGitStatus installs a git status computed elsewhere (typically on a
// background goroutine, since gitstatus.Load shells out to git and can
// take real time on a large or busy repo) without blocking to recompute
// it here.
func (e *Explorer) SetGitStatus(st *gitstatus.Status) { e.git = st }

// MoveDown moves the selection one row down.
func (e *Explorer) MoveDown() {
	if e.cursor < len(e.flat)-1 {
		e.cursor++
	}
}

// MoveUp moves the selection one row up.
func (e *Explorer) MoveUp() {
	if e.cursor > 0 {
		e.cursor--
	}
}

// Toggle expands or collapses the selected directory. It is a no-op for
// files.
func (e *Explorer) Toggle() error {
	if len(e.flat) == 0 {
		return nil
	}
	n := e.flat[e.cursor].node
	if !n.IsDir {
		return nil
	}
	if !n.Expanded {
		if !n.loaded {
			if err := e.loadChildren(n); err != nil {
				return err
			}
		}
		n.Expanded = true
	} else {
		n.Expanded = false
	}
	e.rebuildFlat()
	return nil
}

// ExpandOrDescend expands the selected collapsed directory, or if it's
// already expanded, moves the selection onto its first child (VS Code's
// Right-arrow behavior).
func (e *Explorer) ExpandOrDescend() error {
	if len(e.flat) == 0 {
		return nil
	}
	n := e.flat[e.cursor].node
	if !n.IsDir {
		return nil
	}
	if !n.Expanded {
		return e.Toggle()
	}
	if len(n.Children) > 0 {
		for i, en := range e.flat {
			if en.node == n.Children[0] {
				e.cursor = i
				break
			}
		}
	}
	return nil
}

// CollapseOrAscend collapses the selected expanded directory, or if it's
// already collapsed (or is a file), moves the selection onto its parent
// (VS Code's Left-arrow behavior).
func (e *Explorer) CollapseOrAscend() {
	if len(e.flat) == 0 {
		return
	}
	entry := e.flat[e.cursor]
	if entry.node.IsDir && entry.node.Expanded {
		entry.node.Expanded = false
		e.rebuildFlat()
		return
	}
	parent := entry.node.parent
	if parent == nil || parent == e.Root {
		return
	}
	for i, en := range e.flat {
		if en.node == parent {
			e.cursor = i
			return
		}
	}
}

// HandleEnter implements double-Enter-to-open: a directory toggles on the
// first press; a file only opens once the same row receives a second
// Enter within doubleEnterWindow. It returns the path to open, or "" if
// nothing should be opened yet.
func (e *Explorer) HandleEnter() (openPath string, err error) {
	if len(e.flat) == 0 {
		return "", nil
	}
	entry := e.flat[e.cursor]
	if entry.node.IsDir {
		e.lastEnterIdx = -1
		return "", e.Toggle()
	}
	now := time.Now()
	if e.lastEnterIdx == e.cursor && now.Sub(e.lastEnterTime) <= doubleEnterWindow {
		e.lastEnterIdx = -1
		return entry.node.Path, nil
	}
	e.lastEnterIdx = e.cursor
	e.lastEnterTime = now
	return "", nil
}

// Selected returns the currently highlighted entry.
func (e *Explorer) Selected() (path string, isDir bool, ok bool) {
	if len(e.flat) == 0 {
		return "", false, false
	}
	entry := e.flat[e.cursor]
	return entry.node.Path, entry.node.IsDir, true
}

// Refresh reloads the tree from disk, preserving which directories are
// expanded and the current selection where possible.
func (e *Explorer) Refresh() error {
	selectedPath := ""
	if p, _, ok := e.Selected(); ok {
		selectedPath = p
	}
	e.ignore = ignore.Load(e.Root.Path)
	e.git = gitstatus.Load(e.Root.Path)
	if err := e.reloadRecursive(e.Root); err != nil {
		return err
	}
	e.rebuildFlat()
	for i, en := range e.flat {
		if en.node.Path == selectedPath {
			e.cursor = i
			break
		}
	}
	return nil
}

// TargetDir returns the directory a new file or folder should be created
// in: the selected entry itself if it's a directory, its parent
// otherwise, or the workspace root if nothing is selected.
func (e *Explorer) TargetDir() string {
	if len(e.flat) == 0 {
		return e.Root.Path
	}
	n := e.flat[e.cursor].node
	if n.IsDir {
		return n.Path
	}
	if n.parent != nil {
		return n.parent.Path
	}
	return e.Root.Path
}

// CreateFile creates an empty file named name inside TargetDir(), fails
// if that path already exists (never silently truncates something the
// user didn't ask to replace), refreshes the tree, and selects it.
func (e *Explorer) CreateFile(name string) error {
	path := filepath.Join(e.TargetDir(), name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	f.Close()
	return e.refreshAndSelect(path)
}

// CreateDir creates a directory named name inside TargetDir(), refreshes
// the tree, and selects it.
func (e *Explorer) CreateDir(name string) error {
	path := filepath.Join(e.TargetDir(), name)
	if err := os.Mkdir(path, 0o755); err != nil {
		return err
	}
	return e.refreshAndSelect(path)
}

// Rename renames the selected entry to newName within its own parent
// directory, refreshes the tree, and selects it under its new path.
func (e *Explorer) Rename(newName string) error {
	path, _, ok := e.Selected()
	if !ok {
		return ErrNothingSelected
	}
	newPath := filepath.Join(filepath.Dir(path), newName)
	if err := os.Rename(path, newPath); err != nil {
		return err
	}
	return e.refreshAndSelect(newPath)
}

// Delete removes the selected entry -- recursively, if it's a directory
// -- and refreshes the tree. Callers are expected to confirm with the
// user before calling this: it's not undoable the way an editor buffer's
// changes are.
func (e *Explorer) Delete() error {
	path, isDir, ok := e.Selected()
	if !ok {
		return ErrNothingSelected
	}
	var err error
	if isDir {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		return err
	}
	return e.Refresh()
}

// refreshAndSelect reloads the tree from disk and, if path is now present
// in it, moves the selection there.
func (e *Explorer) refreshAndSelect(path string) error {
	if err := e.Refresh(); err != nil {
		return err
	}
	for i, en := range e.flat {
		if en.node.Path == path {
			e.cursor = i
			break
		}
	}
	return nil
}

func (e *Explorer) reloadRecursive(n *node) error {
	if !n.loaded {
		return nil
	}
	oldExpanded := make(map[string]bool, len(n.Children))
	for _, c := range n.Children {
		if c.IsDir && c.Expanded {
			oldExpanded[c.Name] = true
		}
	}
	if err := e.loadChildren(n); err != nil {
		return err
	}
	for _, c := range n.Children {
		if c.IsDir && oldExpanded[c.Name] {
			c.Expanded = true
			if err := e.reloadRecursive(c); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Explorer) ensureVisible() {
	visibleRows := e.Height - 1
	if visibleRows < 1 {
		visibleRows = 1
	}
	if e.cursor < e.scrollTop {
		e.scrollTop = e.cursor
	}
	if e.cursor >= e.scrollTop+visibleRows {
		e.scrollTop = e.cursor - visibleRows + 1
	}
	if e.scrollTop < 0 {
		e.scrollTop = 0
	}
}

// Render draws the sidebar: a bold header with the workspace root's name,
// then the visible tree rows, with the selected row shown in reverse
// video when focused is true.
func (e *Explorer) Render(focused bool) string {
	if e.Width <= 0 || e.Height <= 0 {
		return ""
	}
	e.ensureVisible()

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Bold(true).Width(e.Width).Render(truncate(e.Root.Name, e.Width)))

	visibleRows := e.Height - 1
	end := e.scrollTop + visibleRows
	if end > len(e.flat) {
		end = len(e.flat)
	}
	for i := e.scrollTop; i < end; i++ {
		entry := e.flat[i]
		indicator := " "
		if entry.node.IsDir {
			if entry.node.Expanded {
				indicator = "▾"
			} else {
				indicator = "▸"
			}
		}
		label := strings.Repeat("  ", entry.depth) + indicator + " " + entry.node.Name
		if code, ok := e.git.Dirty[ignore.RelSlash(e.Root.Path, entry.node.Path)]; ok {
			// A plain trailing status letter (git's own porcelain code:
			// M modified, A added, D deleted, ? untracked, ...) rather
			// than a separately-colored glyph: ambient, not another
			// thing to visually parse, and it needs no per-segment
			// styling machinery on top of this row's single style.
			label += " " + string(code)
		}

		style := lipgloss.NewStyle()
		switch {
		case i == e.cursor && focused:
			style = style.Reverse(true)
		case entry.node.IsDir:
			style = style.Bold(true)
		}
		sb.WriteString("\n")
		sb.WriteString(style.Width(e.Width).Render(truncate(label, e.Width)))
	}
	return sb.String()
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:w])
	}
	return string(r[:w-1]) + "…"
}
