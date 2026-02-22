package tui

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbindings/cli/internal/app"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Invalidate per-frame render cache on every update.
	m.renderCache = nil

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncViewport()
		return m, nil

	case probeResultMsg:
		return m.handleProbeResult(msg)

	case opRunResultMsg:
		return m.handleRunResult(msg)

	case opCancelledMsg:
		return m.handleOpCancelled(msg)

	case streamReadyMsg:
		return m.handleStreamReady(msg)

	case streamEventMsg:
		return m.handleStreamEvent(msg)

	case streamEndedMsg:
		return m.handleStreamEnded(msg)

	case spinnerTickMsg:
		return m.handleSpinnerTick()

	case inputFilesLoadedMsg:
		return m.handleInputFilesLoaded(msg)

	case inputCreatedMsg:
		return m.handleInputCreated(msg)
	case inputDeletedMsg:
		return m.handleInputDeleted(msg)
	case inputErrorMsg:
		return m.handleInputError(msg)
	case workspaceInputCreatedMsg:
		return m.handleWorkspaceInputCreated(msg)

	case workspaceSavedMsg:
		return m.handleWorkspaceSaved(msg)

	case workspaceConflictMsg:
		return m.handleWorkspaceConflict()

	case workspaceLoadedMsg:
		return m.handleWorkspaceReloaded(msg)

	case clearStatusMsg:
		m.statusMsg = ""
		m.pendingConfirm = nil // Clear pending confirmation when status clears
		return m, nil

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m *model) handleWorkspaceSaved(msg workspaceSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "Save failed: " + msg.err.Error()
	} else {
		m.dirty = false
		m.workspaceMtime = msg.newMtime // Update mtime after successful save
		m.statusMsg = "Saved!"
		// Handle post-save action (quit if was save-quit)
		if m.saveConfirm != nil && m.saveConfirm.action == "save-quit" {
			m.saveConfirm = nil
			return m, tea.Quit
		}
	}
	m.saveConfirm = nil
	return m, clearStatusAfter(2 * time.Second)
}

func (m *model) handleWorkspaceConflict() (tea.Model, tea.Cmd) {
	// Determine if this was during a quit attempt
	postAction := "save"
	if m.saveConfirm != nil && m.saveConfirm.action == "save-quit" {
		postAction = "save-quit"
	}
	m.saveConfirm = nil

	m.conflictConfirm = &conflictConfirmState{postAction: postAction}
	m.statusMsg = "Modified externally. [o]verwrite [r]eload [c]ancel"
	return m, nil
}

func (m *model) handleWorkspaceReloaded(msg workspaceLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "Reload failed: " + msg.err.Error()
		return m, clearStatusAfter(3 * time.Second)
	}

	// Update workspace state
	m.workspace = msg.workspace
	m.workspacePath = msg.path
	m.workspaceMtime = msg.mtime
	m.dirty = false

	// Re-initialize tabs from reloaded workspace
	m.initTabsFromWorkspace()

	// Probe all tabs
	var cmds []tea.Cmd
	for i := range m.tabs {
		if m.tabs[i].url != "" {
			m.tabs[i].probe = probeResult{status: app.ProbeStatusProbing}
			cmds = append(cmds, probeCmd(m.tabs[i].id, m.tabs[i].url))
		}
	}

	m.statusMsg = "Reloaded!"
	m.syncViewport()
	cmds = append(cmds, clearStatusAfter(2*time.Second))
	return m, tea.Batch(cmds...)
}

func (m *model) handleProbeResult(msg probeResultMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t == nil {
		return m, nil
	}

	var autoRunCmds []tea.Cmd
	t.probe = msg.result
	if msg.result.finalURL != "" {
		if strings.HasPrefix(t.url, "http://") && strings.HasPrefix(msg.result.finalURL, "https://") {
			t.url = "https://" + strings.TrimPrefix(t.url, "http://")
		}
	}
	t.obi = msg.result.parsed
	t.opKeys = msg.result.opKeys
	t.obiDir = msg.result.obiDir

	if msg.result.parsed != nil {
		tree := BuildOBITree(msg.result.parsed, msg.result.opKeys)
		t.tree = NewTreeState(tree)
		m.restoreUIState(t)
		for _, opKey := range msg.result.opKeys {
			m.rebuildInputsNode(t, opKey)
		}
	}

	if t.runState == nil {
		t.runState = make(map[string]*opRunState)
	}

	if msg.result.parsed != nil {
		for _, opKey := range msg.result.opKeys {
			op := msg.result.parsed.Operations[opKey]
			if op.Idempotent != nil && *op.Idempotent {
				ctx, cancel := context.WithCancel(context.Background())
				t.runState[opKey] = &opRunState{status: app.RunStatusRunning, cancel: cancel}
				autoRunCmds = append(autoRunCmds, runOpCmd(ctx, t.id, opKey, t.url, t.obiDir, msg.result.parsed, nil, ""))
				if t.tree != nil {
					t.tree.ExpandToNode(opKey)
					if node := t.tree.findByID(t.tree.Root.Children, opKey); node != nil {
						node.Expanded = true
					}
				}
			}
		}
		if len(autoRunCmds) > 0 && !m.spinnerActive {
			m.spinnerActive = true
			autoRunCmds = append(autoRunCmds, spinnerTick())
		}
	}

	m.syncViewport()
	if len(autoRunCmds) > 0 {
		return m, tea.Batch(autoRunCmds...)
	}
	return m, nil
}

func (m *model) handleRunResult(msg opRunResultMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t == nil {
		m.syncViewport()
		return m, nil
	}
	if t.runState == nil {
		t.runState = make(map[string]*opRunState)
	}
	existing := t.runState[msg.opKey]
	if existing == nil {
		existing = &opRunState{}
		t.runState[msg.opKey] = existing
	}
	existing.status = msg.status
	existing.output = msg.output
	existing.cancel = nil
	if msg.err != nil {
		existing.error = msg.err.Error()
	} else {
		existing.error = ""
	}
	existing.inputName = msg.inputName
	existing.durationMs = msg.durationMs
	if !existing.expanded && (msg.status == app.RunStatusSuccess || msg.status == app.RunStatusError) {
		existing.expanded = true
	}
	m.syncViewport()
	return m, nil
}

func (m *model) handleOpCancelled(msg opCancelledMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t != nil {
		if t.runState == nil {
			t.runState = make(map[string]*opRunState)
		}
		existing := t.runState[msg.opKey]
		if existing != nil && (existing.status == app.RunStatusRunning || existing.status == app.RunStatusStreaming) {
			return m, nil
		}
		if existing == nil {
			existing = &opRunState{}
			t.runState[msg.opKey] = existing
		}
		existing.status = app.RunStatusError
		existing.error = "cancelled"
		existing.cancel = nil
	}
	m.syncViewport()
	m.statusMsg = "Operation cancelled"
	return m, clearStatusAfter(2 * time.Second)
}

func (m *model) handleStreamReady(msg streamReadyMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t == nil {
		return m, nil
	}
	rs := t.runState[msg.opKey]
	if rs == nil {
		return m, nil
	}
	rs.status = app.RunStatusStreaming
	rs.streamCh = msg.ch
	rs.expanded = true
	m.syncViewport()
	return m, listenCmd(msg.ch, msg.tabID, msg.opKey)
}

func (m *model) handleStreamEvent(msg streamEventMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t == nil {
		return m, nil
	}
	rs := t.runState[msg.opKey]
	if rs == nil || rs.status != app.RunStatusStreaming {
		return m, nil
	}
	rs.eventCount++
	if rs.output != "" {
		rs.output += "\n"
	}
	rs.output += msg.data
	if lines := strings.Split(rs.output, "\n"); len(lines) > maxStreamBufferLines {
		rs.output = strings.Join(lines[len(lines)-maxStreamBufferLines:], "\n")
	}
	m.syncViewport()
	if rs.streamCh != nil {
		return m, listenCmd(rs.streamCh, msg.tabID, msg.opKey)
	}
	return m, nil
}

func (m *model) handleStreamEnded(msg streamEndedMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t == nil {
		return m, nil
	}
	rs := t.runState[msg.opKey]
	if rs == nil {
		return m, nil
	}
	rs.status = app.RunStatusSuccess
	rs.streamCh = nil
	rs.cancel = nil
	m.syncViewport()
	return m, nil
}

func (m *model) handleSpinnerTick() (tea.Model, tea.Cmd) {
	hasRunning := false
	for i := range m.tabs {
		for _, rs := range m.tabs[i].runState {
			if rs.status == app.RunStatusRunning || rs.status == app.RunStatusStreaming {
				rs.frame = (rs.frame + 1) % len(spinnerFrames)
				hasRunning = true
			}
		}
	}
	m.syncViewport()
	if hasRunning {
		return m, spinnerTick()
	}
	m.spinnerActive = false
	return m, nil
}

func (m *model) handleInputFilesLoaded(msg inputFilesLoadedMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t != nil {
		if t.inputFiles == nil {
			t.inputFiles = make(map[string][]inputFile)
		}
		t.inputFiles[msg.opKey] = msg.files
		m.rebuildInputsNode(t, msg.opKey)
	}
	m.syncViewport()
	return m, nil
}

// rebuildInputsNode rebuilds the Inputs section of an operation node.
// Uses workspace-based input associations if workspace is available.
func (m *model) rebuildInputsNode(t *tab, opKey string) {
	if t.tree == nil || t.obi == nil {
		return
	}

	op, ok := t.obi.Operations[opKey]
	if !ok {
		return
	}

	// Find the operation node
	var opNode *TreeNode
	for _, n := range t.tree.Root.Children {
		if n.ID == opKey {
			opNode = n
			break
		}
	}
	if opNode == nil {
		return
	}

	// Gather sources for this operation
	var sources []InputSource

	// Load input files from workspace if available
	var files []inputFile
	if m.workspace != nil && t.targetID != "" {
		files = loadInputsFromWorkspace(m.workspace.Inputs, t.targetID, opKey, &op, t.obi)
	} else {
		// Fallback to old file-based system for compatibility
		files = t.inputFiles[opKey]
	}

	if len(files) > 0 {
		selectedIdx := 0
		if idx, ok := t.selectedInputs[opKey]; ok {
			selectedIdx = idx
		}
		for i, f := range files {
			sources = append(sources, InputSource{
				Name:       f.Name,
				SourceType: InputSourceFile,
				Path:       f.Path,
				Selected:   i == selectedIdx,
				Validation: f.Validation,
				Status:     f.Status,
			})
		}
	}

	// Add examples from operation
	for exKey := range op.Examples {
		sources = append(sources, InputSource{
			Name:       exKey,
			SourceType: InputSourceExample,
			ExampleKey: exKey,
		})
	}

	// Build new inputs node
	inputsNode := buildInputsNode(opKey, op, sources)

	// Find and replace the inputs node in the operation's children
	for j, child := range opNode.Children {
		if child.Type == NodeTypeInputs {
			// Preserve expanded state
			inputsNode.Expanded = child.Expanded
			opNode.Children[j] = inputsNode
			return
		}
	}

	// If no inputs node found (shouldn't happen), prepend it
	opNode.Children = append([]*TreeNode{inputsNode}, opNode.Children...)
}

func (m *model) handleInputCreated(msg inputCreatedMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t != nil {
		m.rebuildInputsNode(t, msg.opKey)
	}
	m.syncViewport()
	return m, nil
}

func (m *model) handleInputDeleted(msg inputDeletedMsg) (tea.Model, tea.Cmd) {
	t := m.findTab(msg.tabID)
	if t != nil {
		if t.inputFiles != nil {
			files := t.inputFiles[msg.opKey]
			for j, f := range files {
				if f.Path == msg.path {
					t.inputFiles[msg.opKey] = append(files[:j], files[j+1:]...)
					break
				}
			}
		}
		if t.selectedInputs != nil {
			current := t.selectedInputs[msg.opKey]
			remaining := len(t.inputFiles[msg.opKey])
			if current >= remaining && remaining > 0 {
				t.selectedInputs[msg.opKey] = remaining - 1
			}
		}
		m.rebuildInputsNode(t, msg.opKey)
	}
	m.syncViewport()
	return m, nil
}

func (m *model) handleInputError(msg inputErrorMsg) (tea.Model, tea.Cmd) {
	m.statusMsg = msg.err.Error()
	return m, clearStatusAfter(3 * time.Second)
}

func (m *model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Scroll only
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle modal if active
	if m.newInputModal != nil {
		return m.handleNewInputModalKeys(msg)
	}
	if m.focusURL {
		return m.handleURLFocusKeys(msg)
	}
	if cmd, handled := m.handleGlobalKeys(msg); handled {
		return m, cmd
	}
	if cmd, handled := m.handleOperationKeys(msg); handled {
		return m, cmd
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// requestConfirm sets up a pending confirmation for a destructive action.
func (m *model) requestConfirm(key, action string, data any, message string) tea.Cmd {
	m.pendingConfirm = &pendingConfirmState{
		key:    key,
		action: action,
		data:   data,
	}
	m.statusMsg = message
	return clearStatusAfter(3 * time.Second)
}

// executeConfirmedAction executes the pending confirmed action.
func (m *model) executeConfirmedAction() tea.Cmd {
	if m.pendingConfirm == nil {
		return nil
	}

	switch m.pendingConfirm.action {
	case "delete-input":
		// Legacy: kept for compatibility but new flow uses removeInputConfirm
		data, ok := m.pendingConfirm.data.(map[string]string)
		if !ok {
			return nil
		}
		tabID, err := strconv.Atoi(data["tabID"])
		if err != nil {
			return nil
		}
		return deleteInputCmd(tabID, data["opKey"], data["path"])

	case "force-delete-input":
		// Force delete despite multiple references
		data, ok := m.pendingConfirm.data.(map[string]string)
		if !ok {
			return nil
		}
		targetID := data["targetID"]
		opKey := data["opKey"]
		inputName := data["inputName"]
		inputPath := data["inputPath"]

		// Remove association first
		m.removeInputAssociation(targetID, opKey, inputName)

		// Then delete the file
		m.statusMsg = "Deleted \"" + inputName + "\""
		return tea.Batch(
			deleteInputCmd(m.tabs[m.active].id, opKey, inputPath),
			clearStatusAfter(2*time.Second),
		)

	default:
		return nil
	}
}

// showRemoveInputConfirm shows the remove input confirmation modal.
func (m *model) showRemoveInputConfirm(targetID, opKey string, src *InputSource) tea.Cmd {
	// Count references to this file in the environment
	refCount := 0
	fileMissing := src.Status == app.InputStatusMissing

	if !fileMissing && m.workspace != nil {
		refCount = countInputRefOccurrences(m.workspace.Inputs, src.Path)
	}

	m.removeInputConfirm = &removeInputConfirmState{
		targetID:    targetID,
		opKey:       opKey,
		inputName:   src.Name,
		inputPath:   src.Path,
		fileMissing: fileMissing,
		refCount:    refCount,
	}

	// Build status message
	if fileMissing {
		m.statusMsg = "Remove \"" + src.Name + "\"? [y]es [c]ancel"
	} else if refCount > 1 {
		m.statusMsg = "Remove \"" + src.Name + "\"? [y]es [d]elete file (used " + strconv.Itoa(refCount) + "x) [c]ancel"
	} else {
		m.statusMsg = "Remove \"" + src.Name + "\"? [y]es [d]elete file [c]ancel"
	}

	return nil
}

// countInputRefOccurrences counts how many times a file path appears in workspace inputs.
func countInputRefOccurrences(inputs map[string]map[string]map[string]string, path string) int {
	count := 0
	for _, targetInputs := range inputs {
		for _, opInputs := range targetInputs {
			for _, ref := range opInputs {
				if ref == path {
					count++
				}
			}
		}
	}
	return count
}

// handleRemoveInputConfirmKeys handles keyboard input for the remove input confirmation modal.
func (m *model) handleRemoveInputConfirmKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	state := m.removeInputConfirm
	if state == nil {
		return nil, false
	}

	switch msg.String() {
	case "y", "Y":
		// Remove association only (don't delete file)
		cmd := m.removeInputAssociation(state.targetID, state.opKey, state.inputName)
		m.removeInputConfirm = nil
		m.statusMsg = "Removed \"" + state.inputName + "\""
		return tea.Batch(cmd, clearStatusAfter(2*time.Second)), true

	case "d", "D":
		// Remove association AND delete file
		if state.fileMissing {
			// Can't delete missing file, just remove association
			cmd := m.removeInputAssociation(state.targetID, state.opKey, state.inputName)
			m.removeInputConfirm = nil
			m.statusMsg = "Removed \"" + state.inputName + "\""
			return tea.Batch(cmd, clearStatusAfter(2*time.Second)), true
		}

		// Warn if file is used elsewhere
		if state.refCount > 1 {
			m.statusMsg = "File used by " + strconv.Itoa(state.refCount) + " inputs. Press D again to confirm delete."
			// Change to a second confirmation
			m.pendingConfirm = &pendingConfirmState{
				key:    "D",
				action: "force-delete-input",
				data: map[string]string{
					"targetID":  state.targetID,
					"opKey":     state.opKey,
					"inputName": state.inputName,
					"inputPath": state.inputPath,
				},
			}
			m.removeInputConfirm = nil
			return clearStatusAfter(3 * time.Second), true
		}

		// Safe to delete (only one reference)
		cmd := m.removeInputWithFile(state.targetID, state.opKey, state.inputName, state.inputPath)
		m.removeInputConfirm = nil
		m.statusMsg = "Deleted \"" + state.inputName + "\""
		return tea.Batch(cmd, clearStatusAfter(2*time.Second)), true

	case "c", "C", "esc":
		// Cancel
		m.removeInputConfirm = nil
		m.statusMsg = ""
		return nil, true
	}

	return nil, true
}

// removeInputAssociation removes an input association from the workspace (in-memory).
func (m *model) removeInputAssociation(targetID, opKey, inputName string) tea.Cmd {
	if m.workspace == nil || m.workspace.Inputs == nil {
		return nil
	}

	if targetInputs, ok := m.workspace.Inputs[targetID]; ok {
		if opInputs, ok := targetInputs[opKey]; ok {
			delete(opInputs, inputName)
			// Clean up empty maps
			if len(opInputs) == 0 {
				delete(targetInputs, opKey)
			}
			if len(targetInputs) == 0 {
				delete(m.workspace.Inputs, targetID)
			}
		}
	}

	m.markDirty()

	// Rebuild the inputs node for this operation
	t := &m.tabs[m.active]
	m.rebuildInputsNode(t, opKey)
	m.syncViewport()

	return nil
}

// removeInputWithFile removes an input association AND deletes the file.
func (m *model) removeInputWithFile(targetID, opKey, inputName, path string) tea.Cmd {
	// First remove the association
	m.removeInputAssociation(targetID, opKey, inputName)

	// Then delete the file (immediate, not undoable)
	return deleteInputCmd(m.tabs[m.active].id, opKey, path)
}

// workspaceInputCreatedMsg is sent after creating a new input file for the workspace.
type workspaceInputCreatedMsg struct {
	targetID string
	opKey    string
	name     string
	path     string
	err      error
}

// createWorkspaceInputCmd creates a new input file and adds an association to the workspace.
func (m *model) createWorkspaceInputCmd(targetID, opKey, name, templateContent string) tea.Cmd {
	// Determine file path - use ./inputs/<target-slug>/<op-slug>/<name>.json
	t := &m.tabs[m.active]
	dir := inputsDir(t.url, opKey)
	path := dir + "/" + name + ".json"

	return func() tea.Msg {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return workspaceInputCreatedMsg{targetID: targetID, opKey: opKey, name: name, err: err}
		}

		if err := os.WriteFile(path, []byte(templateContent), 0644); err != nil {
			return workspaceInputCreatedMsg{targetID: targetID, opKey: opKey, name: name, err: err}
		}

		return workspaceInputCreatedMsg{
			targetID: targetID,
			opKey:    opKey,
			name:     name,
			path:     path,
		}
	}
}

// handleWorkspaceInputCreated handles the result of creating a new workspace input.
func (m *model) handleWorkspaceInputCreated(msg workspaceInputCreatedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = "Error creating input: " + msg.err.Error()
		return m, clearStatusAfter(3 * time.Second)
	}

	// Add association to workspace
	m.addInputAssociation(msg.targetID, msg.opKey, msg.name, msg.path)

	m.statusMsg = "Created \"" + msg.name + "\""
	return m, clearStatusAfter(2 * time.Second)
}

// addInputAssociation adds an input association to the workspace (in-memory).
// If no workspace exists, it auto-creates the global environment.
func (m *model) addInputAssociation(targetID, opKey, name, path string) {
	// Auto-create global environment if no workspace exists
	if m.workspace == nil {
		ws, wsPath, err := app.EnsureGlobalEnvironment()
		if err != nil {
			m.statusMsg = "Failed to create environment: " + err.Error()
			return
		}
		m.workspace = ws
		m.workspacePath = wsPath

		// Update the active tab's targetID to match the workspace's target
		if len(ws.Targets) > 0 && m.active >= 0 && m.active < len(m.tabs) {
			m.tabs[m.active].targetID = ws.Targets[0].ID
			// Also update the targetID parameter if it was the same tab
			if targetID == "" || targetID != ws.Targets[0].ID {
				targetID = ws.Targets[0].ID
			}
		}
	}

	if m.workspace.Inputs == nil {
		m.workspace.Inputs = make(map[string]map[string]map[string]string)
	}

	if m.workspace.Inputs[targetID] == nil {
		m.workspace.Inputs[targetID] = make(map[string]map[string]string)
	}

	if m.workspace.Inputs[targetID][opKey] == nil {
		m.workspace.Inputs[targetID][opKey] = make(map[string]string)
	}

	m.workspace.Inputs[targetID][opKey][name] = path

	m.markDirty()

	// Rebuild the inputs node for the correct tab (matching targetID)
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.targetID == targetID {
			m.rebuildInputsNode(t, opKey)
			break
		}
	}
	m.syncViewport()
}
