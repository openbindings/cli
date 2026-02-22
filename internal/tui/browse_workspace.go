package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/openbindings/cli/internal/app"
)

// workspaceLoadedMsg is sent when workspace loading completes.
type workspaceLoadedMsg struct {
	workspace *app.Workspace
	path      string
	mtime     int64
	err       error
}

// workspaceSavedMsg is sent when workspace save completes.
type workspaceSavedMsg struct {
	newMtime int64
	err      error
}

// workspaceConflictMsg is sent when external modification is detected.
type workspaceConflictMsg struct{}

// checkAndSaveWorkspaceCmd checks for external modifications before saving.
func checkAndSaveWorkspaceCmd(ws *app.Workspace, path string, expectedMtime int64) tea.Cmd {
	return func() tea.Msg {
		// Check current mtime (skip if we couldn't get mtime on load)
		if expectedMtime != 0 {
			if info, err := os.Stat(path); err == nil {
				currentMtime := info.ModTime().UnixNano()
				if currentMtime != expectedMtime {
					// File was modified externally
					return workspaceConflictMsg{}
				}
			}
		}

		// No conflict - proceed with save
		if err := app.SaveWorkspace(filepath.Dir(filepath.Dir(path)), ws); err != nil {
			return workspaceSavedMsg{err: err}
		}

		// Get new mtime after save
		var newMtime int64
		if info, err := os.Stat(path); err == nil {
			newMtime = info.ModTime().UnixNano()
		}

		return workspaceSavedMsg{newMtime: newMtime}
	}
}

// forceSaveWorkspaceCmd saves without checking mtime (for overwrite).
func forceSaveWorkspaceCmd(ws *app.Workspace, path string) tea.Cmd {
	return func() tea.Msg {
		err := app.SaveWorkspace(filepath.Dir(filepath.Dir(path)), ws)
		if err != nil {
			return workspaceSavedMsg{err: err}
		}

		// Get new mtime after save
		var newMtime int64
		if info, err := os.Stat(path); err == nil {
			newMtime = info.ModTime().UnixNano()
		}

		return workspaceSavedMsg{newMtime: newMtime}
	}
}

// reloadWorkspaceCmd reloads the workspace from disk, discarding in-memory changes.
func reloadWorkspaceCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ws, err := app.LoadWorkspace(path)
		if err != nil {
			return workspaceLoadedMsg{err: err, path: path}
		}

		var mtime int64
		if info, err := os.Stat(path); err == nil {
			mtime = info.ModTime().UnixNano()
		}

		return workspaceLoadedMsg{
			workspace: ws,
			path:      path,
			mtime:     mtime,
		}
	}
}

// initTabsFromWorkspace creates tabs from workspace targets.
func (m *model) initTabsFromWorkspace() {
	if m.workspace == nil || len(m.workspace.Targets) == 0 {
		// No workspace or no targets - create default tab with generated targetID
		m.tabs = []tab{{
			id:       1,
			url:      defaultBrowseURL,
			targetID: app.GenerateTargetID(),
			probe:    probeResult{status: app.ProbeStatusProbing},
		}}
		m.active = 0
		m.nextTabID = 1
		return
	}

	m.tabs = make([]tab, 0, len(m.workspace.Targets))
	m.nextTabID = 0

	for _, target := range m.workspace.Targets {
		m.nextTabID++
		t := tab{
			id:       m.nextTabID,
			url:      app.NormalizeURL(target.URL),
			targetID: target.ID,
			probe:    probeResult{status: app.ProbeStatusProbing},
		}
		m.tabs = append(m.tabs, t)
	}

	// Restore active target from workspace UI state
	m.active = 0
	if m.workspace.UI != nil && m.workspace.UI.ActiveTargetID != "" {
		for i, t := range m.tabs {
			if t.targetID == m.workspace.UI.ActiveTargetID {
				m.active = i
				break
			}
		}
	}
}

// restoreUIState restores expanded/selected state for a tab from workspace.
func (m *model) restoreUIState(t *tab) {
	if m.workspace == nil || m.workspace.UI == nil || t.tree == nil {
		return
	}

	uiTarget := m.workspace.UI.Targets[t.targetID]
	if uiTarget == nil {
		return
	}

	// Restore expanded nodes
	for _, path := range uiTarget.Expanded {
		t.tree.ExpandToNode(path)
		if node := t.tree.findByID(t.tree.Root.Children, path); node != nil {
			node.Expanded = true
		}
	}

	// Restore selected node
	if uiTarget.Selected != nil && *uiTarget.Selected != "" {
		t.tree.SelectByID(*uiTarget.Selected)
	}
}

// collectUIState collects current UI state for saving to workspace.
func (m *model) collectUIState() {
	if m.workspace == nil {
		return
	}

	// Initialize UI state if needed
	if m.workspace.UI == nil {
		m.workspace.UI = &app.WorkspaceUI{
			Targets: make(map[string]*app.WorkspaceUITarget),
		}
	}

	// Set active target
	if m.active >= 0 && m.active < len(m.tabs) {
		m.workspace.UI.ActiveTargetID = m.tabs[m.active].targetID
	}

	// Collect per-target UI state
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.targetID == "" || t.tree == nil {
			continue
		}

		uiTarget := &app.WorkspaceUITarget{
			Expanded: collectExpandedPaths(t.tree.Root.Children),
		}

		// Get selected node ID
		if node := t.tree.SelectedNode(); node != nil {
			uiTarget.Selected = &node.ID
		}

		m.workspace.UI.Targets[t.targetID] = uiTarget
	}
}

// collectExpandedPaths recursively collects IDs of expanded nodes.
func collectExpandedPaths(nodes []*TreeNode) []string {
	var result []string
	for _, n := range nodes {
		if n.Expanded {
			result = append(result, n.ID)
		}
		if len(n.Children) > 0 {
			result = append(result, collectExpandedPaths(n.Children)...)
		}
	}
	return result
}

// syncWorkspaceTargets syncs the in-memory workspace targets with current tabs.
func (m *model) syncWorkspaceTargets() {
	if m.workspace == nil {
		return
	}

	// Build a map of existing targets to preserve labels
	existingLabels := make(map[string]string)
	for _, t := range m.workspace.Targets {
		if t.Label != "" {
			existingLabels[t.ID] = t.Label
		}
	}

	m.workspace.Targets = make([]app.WorkspaceTarget, 0, len(m.tabs))
	for _, t := range m.tabs {
		if t.url == "" {
			continue
		}
		target := app.WorkspaceTarget{
			ID:  t.targetID,
			URL: t.url,
		}
		// Generate ID if missing (new tab)
		if target.ID == "" {
			target.ID = app.GenerateTargetID()
		}
		// Preserve existing label
		if label, ok := existingLabels[target.ID]; ok {
			target.Label = label
		}
		m.workspace.Targets = append(m.workspace.Targets, target)
	}
}

// markDirty marks the workspace as having unsaved changes.
func (m *model) markDirty() {
	if m.workspace != nil {
		m.dirty = true
	}
}
