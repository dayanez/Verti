package app

import "path/filepath"

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

// renameExplorerEntry renames the selected explorer entry to newName.
func (m *Model) renameExplorerEntry(newName string) {
	if newName == "" {
		return
	}
	if err := m.exp.Rename(newName); err != nil {
		m.status = "rename failed: " + err.Error()
		return
	}
	m.status = "renamed to " + newName
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
	m.status = "deleted " + name
}
