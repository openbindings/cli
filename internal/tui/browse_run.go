package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbindings/cli/internal/app"
)

// RunBrowse launches the TUI browser. If workspaceName is non-empty,
// it loads that workspace instead of the active one.
func RunBrowse(workspaceName string) error {
	ti := textinput.New()
	ti.Placeholder = "exec:ob or https://example.com"
	ti.Prompt = ""
	ti.CharLimit = 2048
	ti.Width = 60

	// Try to load workspace synchronously for initial state
	var ws *app.Workspace
	var wsPath string
	var wsMtime int64
	envPath, err := app.FindEnvPath()
	if err != nil {
		if workspaceName != "" {
			return fmt.Errorf("failed to load workspace %q: no environment found; run 'ob init' first", workspaceName)
		}
	} else if workspaceName != "" {
		wsPath = app.WorkspacePath(envPath, workspaceName)
		ws, err = app.LoadWorkspace(wsPath)
		if err != nil {
			return fmt.Errorf("failed to load workspace %q: %w", workspaceName, err)
		}
	} else {
		ws, wsPath, err = app.LoadActiveWorkspace(envPath)
		if err != nil {
			return fmt.Errorf("failed to load active workspace: %w", err)
		}
	}

	// Record mtime for conflict detection
	if wsPath != "" {
		if info, err := os.Stat(wsPath); err == nil {
			wsMtime = info.ModTime().UnixNano()
		}
	}

	m := &model{
		urlInput:       ti,
		focusURL:       false,
		viewport:       viewport.New(0, 0),
		workspace:      ws,
		workspacePath:  wsPath,
		workspaceMtime: wsMtime,
	}

	// Initialize tabs from workspace (or default)
	m.initTabsFromWorkspace()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
