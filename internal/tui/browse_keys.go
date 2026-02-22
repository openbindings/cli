package tui

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbindings/cli/internal/app"
)

// clearStatusAfter returns a command that clears the status after duration.
func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

// clearStatusMsg is sent after a delay to clear the status message.
type clearStatusMsg struct{}

// handleURLFocusKeys handles keyboard input when the URL bar is focused.
func (m *model) handleURLFocusKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.blurURLBar()
		u := strings.TrimSpace(m.urlInput.Value())
		m.tabs[m.active].setURL(u)
		m.syncViewport()
		return m, probeCmd(m.tabs[m.active].id, m.tabs[m.active].url)
	case "ctrl+u", "ctrl+k", "ctrl+backspace", "ctrl+delete", "cmd+backspace", "cmd+delete":
		m.urlInput.SetValue("")
		m.urlInput.CursorEnd()
		return m, nil
	case "esc", "ctrl+g":
		m.blurURLBar()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

// handleGlobalKeys handles global keyboard shortcuts.
func (m *model) handleGlobalKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	// Handle conflict confirmation modal
	if m.conflictConfirm != nil {
		return m.handleConflictConfirmKeys(msg)
	}

	// Handle save confirmation modal
	if m.saveConfirm != nil {
		return m.handleSaveConfirmKeys(msg)
	}

	// Handle remove input confirmation modal
	if m.removeInputConfirm != nil {
		return m.handleRemoveInputConfirmKeys(msg)
	}

	switch msg.String() {
	case "q", "esc":
		return m.requestQuit(), true
	case "ctrl+c":
		// Force quit without save prompt
		return tea.Quit, true
	case "ctrl+s":
		return m.saveWorkspace(), true
	case "ctrl+r":
		return m.refreshActiveTab(), true
	case "tab":
		m.active = (m.active + 1) % len(m.tabs)
		m.syncViewport()
		return nil, true
	case "shift+tab":
		m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
		m.syncViewport()
		return nil, true
	case "t":
		cmd := m.newTab(defaultBrowseURL)
		m.syncViewport()
		return cmd, true
	case "x":
		m.closeActive()
		m.syncViewport()
		return nil, true
	case "u", "ctrl+l":
		return m.focusURLBar(), true
	}
	return nil, false
}

// handleSaveConfirmKeys handles keys during save confirmation.
func (m *model) handleSaveConfirmKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "y", "Y":
		// Save and quit (or just save)
		m.saveConfirm.action = "save-quit"
		return m.saveWorkspace(), true
	case "n", "N":
		// Discard and quit
		m.saveConfirm = nil
		return tea.Quit, true
	case "esc", "c", "C":
		// Cancel - go back
		m.saveConfirm = nil
		m.statusMsg = ""
		return nil, true
	}
	return nil, true // Consume all keys while modal is open
}

// handleConflictConfirmKeys handles keys during conflict confirmation.
func (m *model) handleConflictConfirmKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "o", "O":
		// Overwrite - force save
		postAction := m.conflictConfirm.postAction
		m.conflictConfirm = nil
		m.statusMsg = ""

		// Collect UI state and sync
		m.collectUIState()
		m.syncWorkspaceTargets()

		if postAction == "save-quit" {
			// Set up to quit after save
			m.saveConfirm = &saveConfirmState{action: "save-quit"}
		}
		return forceSaveWorkspaceCmd(m.workspace, m.workspacePath), true

	case "r", "R":
		// Reload - discard local changes
		m.conflictConfirm = nil
		m.statusMsg = "Reloading..."
		return reloadWorkspaceCmd(m.workspacePath), true

	case "esc", "c", "C":
		// Cancel - go back
		m.conflictConfirm = nil
		m.statusMsg = ""
		return nil, true
	}
	return nil, true // Consume all keys while modal is open
}

// requestQuit handles quit request, prompting if dirty.
func (m *model) requestQuit() tea.Cmd {
	if !m.dirty || m.workspace == nil {
		return tea.Quit
	}

	// Show save confirmation
	m.saveConfirm = &saveConfirmState{action: "quit"}
	m.statusMsg = "Unsaved changes. Save? (y/n/c)"
	return nil
}

// refreshActiveTab re-probes the active tab.
func (m *model) refreshActiveTab() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	t := &m.tabs[m.active]
	if t.url == "" {
		return nil
	}

	// Reset tab state
	t.probe = probeResult{status: app.ProbeStatusProbing}
	t.obi = nil
	t.tree = nil
	t.opKeys = nil
	t.runState = nil
	t.inputFiles = nil
	t.selectedInputs = nil

	m.syncViewport()
	return probeCmd(t.id, t.url)
}

// saveWorkspace saves the current workspace (with conflict check).
func (m *model) saveWorkspace() tea.Cmd {
	if m.workspace == nil || m.workspacePath == "" {
		m.statusMsg = "No workspace to save"
		return clearStatusAfter(2 * time.Second)
	}

	// Collect current UI state
	m.collectUIState()
	m.syncWorkspaceTargets()

	// Check for external modifications
	return checkAndSaveWorkspaceCmd(m.workspace, m.workspacePath, m.workspaceMtime)
}

// handleOperationKeys handles keyboard input for tree navigation and operations.
func (m *model) handleOperationKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	t := &m.tabs[m.active]

	// Handle pending confirmation
	if m.pendingConfirm != nil {
		if msg.String() == m.pendingConfirm.key {
			// Confirmed - execute the action
			cmd := m.executeConfirmedAction()
			m.pendingConfirm = nil
			m.statusMsg = ""
			return cmd, true
		}
		// Any other key cancels
		m.pendingConfirm = nil
		m.statusMsg = ""
		return nil, true
	}

	switch msg.String() {
	case "j", "down":
		if t.tree != nil {
			t.tree.MoveDown()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "k", "up":
		if t.tree != nil {
			t.tree.MoveUp()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "alt+down":
		// Jump to next sibling (or parent's next sibling)
		if t.tree != nil {
			t.tree.MoveToNextSibling()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "alt+up":
		// Jump to previous sibling (or parent)
		if t.tree != nil {
			t.tree.MoveToPrevSibling()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "shift+down", "G":
		// Go to last node
		if t.tree != nil {
			t.tree.MoveToLast()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "shift+up", "g":
		// Go to first node
		if t.tree != nil {
			t.tree.MoveToFirst()
			m.ensureSelectionVisible()
			m.syncViewport()
		}
		return nil, true
	case "right", "l":
		if t.tree != nil {
			node := t.tree.SelectedNode()
			wasExpanded := node != nil && node.Expanded
			t.tree.Expand()
			m.syncViewport()

			if node != nil && node.Type == NodeTypeOperation && !wasExpanded {
				m.rebuildInputsNode(t, node.ID)
			}
		}
		return nil, true
	case "shift+right":
		if t.tree != nil {
			t.tree.ExpandAll()
			m.syncViewport()
		}
		return nil, true
	case "left", "h":
		if t.tree != nil {
			t.tree.Collapse()
			m.syncViewport()
		}
		return nil, true
	case "shift+left":
		if t.tree != nil {
			t.tree.CollapseAll()
			m.syncViewport()
		}
		return nil, true
	case "enter", " ":
		return m.handleRunOp(), true
	case "r":
		// Context-sensitive: remove for input files, run otherwise
		if t.tree != nil {
			node := t.tree.SelectedNode()
			if node != nil && node.Type == NodeTypeInputFile {
				if src, ok := node.Data.(*InputSource); ok {
					opKey := m.selectedOperationKey()
					if opKey != "" && t.targetID != "" {
						return m.showRemoveInputConfirm(t.targetID, opKey, src), true
					}
				}
				return nil, true
			}
		}
		// Default: run operation
		return m.handleRunOp(), true
	case "o":
		// Copy output
		opKey := m.selectedOperationKey()
		if opKey != "" {
			if rs, ok := t.runState[opKey]; ok && rs.output != "" {
				if err := clipboard.WriteAll(rs.output); err == nil {
					m.statusMsg = "Copied!"
					return clearStatusAfter(2 * time.Second), true
				} else {
					m.statusMsg = "Copy failed"
					return clearStatusAfter(2 * time.Second), true
				}
			}
		}
		return nil, true
	case "c":
		// Cancel running operation
		opKey := m.selectedOperationKey()
		if opKey != "" {
			if rs, ok := t.runState[opKey]; ok && (rs.status == app.RunStatusRunning || rs.status == app.RunStatusStreaming) && rs.cancel != nil {
				rs.cancel()
				m.statusMsg = "Cancelling..."
				return clearStatusAfter(2 * time.Second), true
			}
		}
		return nil, true
	case "e":
		// Edit the currently selected input file
		if t.tree != nil {
			node := t.tree.SelectedNode()
			if node != nil && node.Type == NodeTypeInputFile {
				if src, ok := node.Data.(*InputSource); ok && src.Path != "" {
					// Can't edit missing files
					if src.Status == app.InputStatusMissing {
						m.statusMsg = "Cannot edit: file not found"
						return clearStatusAfter(2 * time.Second), true
					}
					return editInputCmd(src.Path), true
				}
			}
		}
		return nil, true
	case "v":
		// Validate the currently selected input file
		if t.tree != nil && t.obi != nil {
			node := t.tree.SelectedNode()
			if node != nil && node.Type == NodeTypeInputFile {
				if src, ok := node.Data.(*InputSource); ok && src.Path != "" {
					// Can't validate missing files
					if src.Status == app.InputStatusMissing {
						m.statusMsg = "Cannot validate: file not found"
						return clearStatusAfter(2 * time.Second), true
					}
					// Get the operation for validation
					opKey := m.selectedOperationKey()
					if opKey != "" {
						if op, ok := t.obi.Operations[opKey]; ok {
							result := ValidateInputFile(src.Path, op, t.obi)
							switch result.Status {
							case ValidationValid:
								m.statusMsg = "Valid: \"" + src.Name + "\" matches schema"
							case ValidationInvalid:
								m.statusMsg = "Invalid: " + result.Message
							case ValidationError:
								m.statusMsg = "Validation error: " + result.Message
							default:
								m.statusMsg = "Cannot validate: no schema defined"
							}
							return clearStatusAfter(3 * time.Second), true
						}
					}
				}
			}
		}
		return nil, true
	case "]":
		// Next input file for current operation
		opKey := m.selectedOperationKey()
		if opKey != "" {
			files := t.inputFiles[opKey]
			if len(files) > 0 {
				if t.selectedInputs == nil {
					t.selectedInputs = make(map[string]int)
				}
				current := t.selectedInputs[opKey]
				if current < len(files)-1 {
					t.selectedInputs[opKey] = current + 1
					m.syncViewport()
				}
			}
		}
		return nil, true
	case "[":
		// Previous input file for current operation
		opKey := m.selectedOperationKey()
		if opKey != "" {
			if t.selectedInputs == nil {
				t.selectedInputs = make(map[string]int)
			}
			current := t.selectedInputs[opKey]
			if current > 0 {
				t.selectedInputs[opKey] = current - 1
				m.syncViewport()
			}
		}
		return nil, true
	}
	return nil, false
}

// selectedOperationKey returns the operation key for the currently selected node.
// If the selected node is within an operation (e.g., an input node), returns the parent operation key.
func (m *model) selectedOperationKey() string {
	t := &m.tabs[m.active]
	if t.tree == nil || t.obi == nil {
		return ""
	}
	node := t.tree.SelectedNode()
	if node == nil {
		return ""
	}

	// If it's an operation node, return its ID
	if node.Type == NodeTypeOperation {
		return node.ID
	}

	// Otherwise, find which operation key the node ID starts with
	// Node IDs are formatted as "opKey.section.subsection..."
	// Since operation keys can contain dots, we need to check all keys
	for _, opKey := range t.opKeys {
		if strings.HasPrefix(node.ID, opKey+".") || node.ID == opKey {
			return opKey
		}
	}

	return ""
}
