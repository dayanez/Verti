// Package explorer implements the collapsible left-hand file tree sidebar:
// lazy directory loading, arrow-key navigation, expand/collapse, and
// double-Enter-to-open semantics (a single Enter on a file just marks it
// as "about to open" so a stray keypress while navigating doesn't yank
// focus away from the tree; a second Enter within the window opens it).
package explorer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

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
}

// New loads rootPath as the explorer's workspace root, with its immediate
// children loaded and visible (but not expanded).
func New(rootPath string) (*Explorer, error) {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	root := &node{Name: filepath.Base(abs), Path: abs, IsDir: true, Expanded: true}
	e := &Explorer{Root: root, lastEnterIdx: -1}
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
		n.Children = append(n.Children, &node{
			Name: ent.Name(), Path: filepath.Join(n.Path, ent.Name()), IsDir: ent.IsDir(), parent: n,
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
