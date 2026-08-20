// Command verti is a terminal text editor: gap-buffer editing with
// tree-sitter syntax highlighting, a collapsible file explorer, an
// embedded PTY subshell, and a mouse-driven ASCII-art "paint" overlay for
// dropping box-drawing diagrams into comments. See README.md.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dommcpro/verti/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verti: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}
	filePath := ""

	if len(os.Args) > 1 {
		arg := os.Args[1]
		abs, err := filepath.Abs(arg)
		if err != nil {
			return err
		}
		if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
			rootDir = abs
		} else {
			filePath = abs
			rootDir = filepath.Dir(abs)
		}
	}

	m, err := app.New(rootDir, filePath)
	if err != nil {
		return err
	}
	defer m.Close()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
