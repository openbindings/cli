package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	openbindings "github.com/openbindings/openbindings-go"

	"github.com/openbindings/cli/internal/app"
)

// opRunResultMsg is sent when an operation execution completes.
type opRunResultMsg struct {
	tabID      int
	opKey      string
	status     string // app.RunStatusSuccess | app.RunStatusError
	output     string
	err        error
	inputName  string
	durationMs int64
}

// opCancelledMsg is sent when an operation is cancelled.
type opCancelledMsg struct {
	tabID int
	opKey string
}

// streamEventMsg is sent for each event received from a streaming subscription.
type streamEventMsg struct {
	tabID int
	opKey string
	data  string
}

// streamEndedMsg is sent when a streaming subscription closes.
type streamEndedMsg struct {
	tabID int
	opKey string
}

// spinnerTickMsg triggers spinner animation updates.
type spinnerTickMsg struct{}

// spinnerFrames for animated loading indicator
var spinnerFrames = []string{"⢎ ", "⠎⠁", "⠊⠑", "⠈⠱", " ⡱", "⢀⡰", "⢄⡠", "⢆⡀"}

// runOpCmd creates a command that executes an operation.
// Returns the command and a cancel function to abort the operation.
func runOpCmd(ctx context.Context, tabID int, opKey string, targetURL string, obiDir string, iface *openbindings.Interface, inputData map[string]any, inputName string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		output, err := executeOpWithContext(ctx, opKey, targetURL, obiDir, iface, inputData)
		durationMs := time.Since(start).Milliseconds()

		if err != nil && ctx.Err() != nil {
			return opCancelledMsg{tabID: tabID, opKey: opKey}
		}

		status := app.RunStatusSuccess
		if err != nil {
			status = app.RunStatusError
		}
		return opRunResultMsg{
			tabID:      tabID,
			opKey:      opKey,
			status:     status,
			output:     output,
			err:        err,
			inputName:  inputName,
			durationMs: durationMs,
		}
	}
}

// spinnerTick returns a command that ticks the spinner after a delay.
func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func executeOpWithContext(ctx context.Context, opKey string, targetURL string, obiDir string, iface *openbindings.Interface, inputData map[string]any) (string, error) {
	if iface == nil {
		return "", fmt.Errorf("no interface")
	}

	// Find the default binding for this operation
	_, binding := app.DefaultBindingForOp(opKey, iface)
	if binding == nil {
		return "", fmt.Errorf("no binding available for operation %q", opKey)
	}

	// Get the binding source
	source, ok := iface.Sources[binding.Source]
	if !ok {
		return "", fmt.Errorf("binding source %q not found", binding.Source)
	}

	// Prepare the input for execution
	var execInputData any
	if binding.InputTransform != nil {
		// Apply input transform: OBI schema → binding format
		transformed, err := app.ApplyTransform(iface.Transforms, binding.InputTransform, inputData)
		if err != nil {
			return "", fmt.Errorf("input transform failed: %w", err)
		}
		execInputData = transformed
	} else if len(inputData) > 0 {
		execInputData = inputData
	}

	// Build ExecuteOperationInput
	execInput := app.ExecuteOperationInput{
		Source: app.ExecuteSource{
			Format: source.Format,
		},
		Ref:   binding.Ref,
		Input: execInputData,
	}

	// Extract binary name from target URL, but only when it's a simple
	// exec:<command> reference (e.g., "exec:ob" → "ob"). Multi-token exec:
	// refs like "exec:curl file:///path" are a probe/delivery mechanism,
	// not the actual binary — let the spec's bin field resolve it instead.
	if strings.HasPrefix(targetURL, "exec:") {
		rest := strings.TrimPrefix(targetURL, "exec:")
		if !strings.ContainsAny(rest, " \t") {
			execInput.Source.Binary = rest
		}
	}

	// Determine the source location
	// Priority: Location (file path/URI) > Content (embedded)
	// Resolve relative locations against the OBI's base directory (per spec).
	if source.Location != "" {
		loc := source.Location
		if !filepath.IsAbs(loc) && obiDir != "" {
			loc = filepath.Join(obiDir, loc)
		}
		execInput.Source.Location = loc
	} else if source.Content != nil {
		execInput.Source.Content = source.Content
	} else {
		// No artifact or inline - that's okay if we have a binary hint
		if execInput.Source.Binary == "" {
			return "", fmt.Errorf("binding source %q has no artifact or inline content", binding.Source)
		}
	}

	// Execute via the unified operation executor with context for cancellation
	result := app.ExecuteOperationWithContext(ctx, execInput)

	// Apply output transform if present
	output := result.Output
	if binding.OutputTransform != nil && result.Error == nil {
		transformed, err := app.ApplyTransform(iface.Transforms, binding.OutputTransform, output)
		if err != nil {
			return formatOutput(output), fmt.Errorf("output transform failed: %w", err)
		}
		output = transformed
	}

	// Format output
	if result.Error != nil {
		return formatOutput(output), fmt.Errorf("%s", result.Error.Message)
	}
	if result.Status != 0 {
		return formatOutput(output), fmt.Errorf("exit status %d", result.Status)
	}

	return formatOutput(output), nil
}

// formatOutput converts the operation output to a display string.
func formatOutput(output any) string {
	if output == nil {
		return ""
	}

	switch o := output.(type) {
	case string:
		return o
	case map[string]any:
		// Check for stdout/stderr from CLI execution
		var result strings.Builder
		if stdout, ok := o["stdout"].(string); ok && stdout != "" {
			result.WriteString(stdout)
		}
		if stderr, ok := o["stderr"].(string); ok && stderr != "" {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(stderr)
		}
		if result.Len() > 0 {
			return result.String()
		}
		// Serialize map as pretty-printed JSON
		b, err := json.MarshalIndent(o, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", o)
		}
		return string(b)
	default:
		// Try JSON serialization for structured data
		b, err := json.MarshalIndent(o, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", o)
		}
		return string(b)
	}
}

// handleRunOp handles the run action for the currently selected node.
func (m *model) handleRunOp() tea.Cmd {
	t := &m.tabs[m.active]
	if t.obi == nil || t.tree == nil {
		return nil
	}

	node := t.tree.SelectedNode()
	if node == nil {
		return nil
	}

	// Handle different node types
	switch node.Type {
	case NodeTypeOperation:
		return m.runOperation(t, node.ID)

	case NodeTypeInputFile:
		// Select this input AND run the operation with it
		if src, ok := node.Data.(*InputSource); ok {
			opKey := m.selectedOperationKey()
			if opKey != "" {
				// Find the index of this input file and select it
				for i, f := range t.inputFiles[opKey] {
					if f.Path == src.Path {
						if t.selectedInputs == nil {
							t.selectedInputs = make(map[string]int)
						}
						t.selectedInputs[opKey] = i
						m.rebuildInputsNode(t, opKey)
						break
					}
				}
				// Now run with this input
				return m.runOperation(t, opKey)
			}
		}
		return nil

	case NodeTypeInputExample:
		// Run with example data
		if src, ok := node.Data.(*InputSource); ok {
			opKey := m.selectedOperationKey()
			if opKey != "" {
				return m.runWithExample(t, opKey, src.ExampleKey)
			}
		}
		return nil

	case NodeTypeInputNew:
		// Show new input modal
		opKey := m.selectedOperationKey()
		if opKey != "" {
			return m.showNewInputModal(opKey)
		}
		return nil

	default:
		// For other nodes, try to run the parent operation
		opKey := m.selectedOperationKey()
		if opKey != "" {
			return m.runOperation(t, opKey)
		}
		return nil
	}
}

// runOperation executes an operation with the currently selected input.
func (m *model) runOperation(t *tab, opKey string) tea.Cmd {
	var inputData map[string]any
	var inputName string
	var files []inputFile
	if m.workspace != nil && t.targetID != "" && t.obi != nil {
		if op, ok := t.obi.Operations[opKey]; ok {
			files = loadInputsFromWorkspace(m.workspace.Inputs, t.targetID, opKey, &op, t.obi)
		}
	}
	if files == nil {
		files = t.inputFiles[opKey]
	}
	if len(files) > 0 {
		selIdx := t.selectedInputs[opKey]
		if selIdx >= 0 && selIdx < len(files) {
			inputName = files[selIdx].Name
			data, err := readInputFile(files[selIdx].Path)
			if err == nil {
				inputData = data
			}
		}
	}
	return m.launchOp(t, opKey, inputData, inputName)
}

// runWithExample executes an operation using example data from the spec.
func (m *model) runWithExample(t *tab, opKey, exampleKey string) tea.Cmd {
	op, ok := t.obi.Operations[opKey]
	if !ok {
		return nil
	}

	example, ok := op.Examples[exampleKey]
	if !ok {
		return nil
	}

	// Convert example input to map
	var inputData map[string]any
	if example.Input != nil {
		if data, ok := example.Input.(map[string]any); ok {
			inputData = data
		}
	}

	return m.launchOp(t, opKey, inputData, "example:"+exampleKey)
}

// subscribeOpCmd starts a streaming subscription for an event operation.
// It opens the channel and returns a streamReadyMsg with the event channel.
func subscribeOpCmd(ctx context.Context, tabID int, opKey string, targetURL string, obiDir string, iface *openbindings.Interface) tea.Cmd {
	return func() tea.Msg {
		if iface == nil {
			return opRunResultMsg{tabID: tabID, opKey: opKey, status: app.RunStatusError, err: fmt.Errorf("no interface")}
		}

		_, binding := app.DefaultBindingForOp(opKey, iface)
		if binding == nil {
			return opRunResultMsg{tabID: tabID, opKey: opKey, status: app.RunStatusError, err: fmt.Errorf("no binding for operation %q", opKey)}
		}

		source, ok := iface.Sources[binding.Source]
		if !ok {
			return opRunResultMsg{tabID: tabID, opKey: opKey, status: app.RunStatusError, err: fmt.Errorf("source %q not found", binding.Source)}
		}

		ch, err := app.SubscribeOBIOperationDirect(ctx, iface, opKey, binding, source, obiDir, "")
		if err != nil {
			return opRunResultMsg{tabID: tabID, opKey: opKey, status: app.RunStatusError, err: err}
		}

		return streamReadyMsg{tabID: tabID, opKey: opKey, ch: ch}
	}
}

// streamReadyMsg is sent when a subscription channel is established.
type streamReadyMsg struct {
	tabID int
	opKey string
	ch    <-chan app.StreamEvent
}

// listenCmd waits for the next event from a stream channel.
// The channel is closed when the upstream context is cancelled or the stream ends.
func listenCmd(ch <-chan app.StreamEvent, tabID int, opKey string) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamEndedMsg{tabID: tabID, opKey: opKey}
		}
		if ev.Error != nil {
			return streamEventMsg{tabID: tabID, opKey: opKey, data: "Error: " + ev.Error.Message}
		}
		return streamEventMsg{tabID: tabID, opKey: opKey, data: formatOutput(ev.Data)}
	}
}

// launchOp is the shared logic for starting an operation run: cancels any
// existing run, sets up context/state, expands the tree node, and returns
// the batched command (run + spinner).
func (m *model) launchOp(t *tab, opKey string, inputData map[string]any, inputName string) tea.Cmd {
	if t.runState == nil {
		t.runState = make(map[string]*opRunState)
	}

	// Cancel any existing run/stream of this operation.
	if existing, ok := t.runState[opKey]; ok {
		if (existing.status == app.RunStatusRunning || existing.status == app.RunStatusStreaming) && existing.cancel != nil {
			existing.cancel()
		}
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Expand the operation in tree
	if t.tree != nil {
		if node := t.tree.findByID(t.tree.Root.Children, opKey); node != nil {
			node.Expanded = true
		}
	}

	// Check if this is an event operation — use streaming path.
	if t.obi != nil {
		if op, ok := t.obi.Operations[opKey]; ok && op.Kind == "event" {
			t.runState[opKey] = &opRunState{
				status:    app.RunStatusRunning,
				streaming: true,
				inputName: inputName,
				cancel:    cancel,
			}
			m.syncViewport()
			cmds := []tea.Cmd{subscribeOpCmd(ctx, t.id, opKey, t.url, t.obiDir, t.obi)}
			if !m.spinnerActive {
				m.spinnerActive = true
				cmds = append(cmds, spinnerTick())
			}
			return tea.Batch(cmds...)
		}
	}

	t.runState[opKey] = &opRunState{
		status:    app.RunStatusRunning,
		inputName: inputName,
		cancel:    cancel,
	}

	m.syncViewport()
	cmds := []tea.Cmd{runOpCmd(ctx, t.id, opKey, t.url, t.obiDir, t.obi, inputData, inputName)}
	if !m.spinnerActive {
		m.spinnerActive = true
		cmds = append(cmds, spinnerTick())
	}
	return tea.Batch(cmds...)
}
