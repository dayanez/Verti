package app

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dayanez/Verti/pkg/gitstatus"
)

// gitStatusMsg carries a background git-status reload's result back into
// Update once reloadGitStatusCmd's goroutine finishes.
type gitStatusMsg struct{ status *gitstatus.Status }

// reloadGitStatusCmd re-reads the workspace's git status on its own
// goroutine and returns immediately. gitstatus.Load shells out to git,
// which can take real time on a large or busy repo, so it must never run
// inline in an event handler: that would stall Update -- and therefore
// every keystroke and redraw -- until the subprocess returns.
func reloadGitStatusCmd(root string) tea.Cmd {
	return func() tea.Msg {
		return gitStatusMsg{status: gitstatus.Load(root)}
	}
}

// createExplorerFile creates name inside the explorer's current target
// directory (see explorer.TargetDir) and reports the result on the
// status bar.
func (m *Model) createExplorerFile(name string) {
	if name == "" {
		return
	}
	if err := m.exp.CreateFile(name); err != nil {
		m.status = "create file failed: " + err.Error()
		return
	}
	m.status = "created " + name
}

// createExplorerFolder creates a directory named name inside the
// explorer's current target directory.
func (m *Model) createExplorerFolder(name string) {
	if name == "" {
		return
	}
	if err := m.exp.CreateDir(name); err != nil {
		m.status = "create folder failed: " + err.Error()
		return
	}
	m.status = "created " + name + "/"
}

// renameExplorerEntry renames the selected explorer entry to newName, and
// retargets its open tab (if any) to the new path so a later Ctrl+S writes
// there instead of recreating the old file under its old name.
func (m *Model) renameExplorerEntry(newName string) {
	if newName == "" {
		return
	}
	oldPath, _, ok := m.exp.Selected()
	if !ok {
		return
	}
	if err := m.exp.Rename(newName); err != nil {
		m.status = "rename failed: " + err.Error()
		return
	}
	m.retargetTab(oldPath, filepath.Join(filepath.Dir(oldPath), newName))
	m.status = "renamed to " + newName
}

// retargetTab updates the tab open for oldPath, if any, to point at
// newPath instead, re-resolving its syntax highlighter for the new
// extension.
func (m *Model) retargetTab(oldPath, newPath string) {
	i := m.findTab(oldPath)
	if i < 0 {
		return
	}
	m.tabs[i].filename = newPath
	m.tabs[i].highlighter = highlighterFor(m.highlighter, newPath)
	if i == m.activeTab {
		m.filename = newPath
	}
}

// deleteSelectedExplorerEntry actually performs the delete after the
// user has confirmed via promptConfirmDeleteFile; see
// explorer.Delete's own doc comment for why that confirmation exists.
func (m *Model) deleteSelectedExplorerEntry() {
	path, _, ok := m.exp.Selected()
	if !ok {
		return
	}
	name := filepath.Base(path)
	if err := m.exp.Delete(); err != nil {
		m.status = "delete failed: " + err.Error()
		return
	}
	m.untitleTab(path)
	m.status = "deleted " + name
}

// untitleTab clears the filename of the tab open for path, if any, turning
// it into an untitled buffer. Used when the explorer deletes the file out
// from under an open tab, so Ctrl+S doesn't silently recreate the deleted
// file and the tab's unsaved content isn't lost.
func (m *Model) untitleTab(path string) {
	i := m.findTab(path)
	if i < 0 {
		return
	}
	m.tabs[i].filename = ""
	m.tabs[i].highlighter = highlighterFor(m.highlighter, "")
	if i == m.activeTab {
		m.filename = ""
	}
}
