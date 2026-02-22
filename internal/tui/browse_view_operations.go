package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/openbindings/cli/internal/app"
	openbindings "github.com/openbindings/openbindings-go"
)

// viewOperations renders the operations list for the current tab.
// The result is cached for the current frame to avoid recomputation.
func (m *model) viewOperations() string {
	if m.renderCache != nil && m.renderCache.opsViewSet {
		return m.renderCache.opsView
	}
	result := m.viewOperationsUncached()
	if m.renderCache == nil {
		m.renderCache = &frameCache{}
	}
	m.renderCache.opsView = result
	m.renderCache.opsViewSet = true
	return result
}

func (m *model) viewOperationsUncached() string {
	t := m.tabs[m.active]

	if t.obi == nil {
		return m.viewNoOBI()
	}

	if len(t.obi.Operations) == 0 {
		return m.viewZeroOperations()
	}

	var sb strings.Builder

	// Header: interface info + operations list title
	sb.WriteString(m.viewOperationsHeader(t))

	// Render tree if available
	if t.tree != nil {
		selectedNode := t.tree.SelectedNode()

		// Render tree nodes with output blocks after operation children
		visible := t.tree.VisibleNodes()
		var pendingOutput struct {
			opKey string
			depth int
			rs    *opRunState
		}

		for i, node := range visible {
			// Check if we need to render pending output before this node
			// Output should appear when we leave the operation's subtree
			if pendingOutput.rs != nil && node.Depth <= pendingOutput.depth {
				indent := strings.Repeat(" ", (pendingOutput.depth+1)*2)
				sb.WriteString(m.renderRunState(indent, pendingOutput.rs))
				pendingOutput.rs = nil
			}

			// Render the node
			line := m.renderTreeNode(node, node == selectedNode, t)
			sb.WriteString(line)
			sb.WriteString("\n")

			// If this is an expanded operation with run output, queue it
			if node.Type == NodeTypeOperation && node.Expanded {
				if rs, ok := t.runState[node.ID]; ok && (rs.status == app.RunStatusSuccess || rs.status == app.RunStatusError || rs.status == app.RunStatusRunning || rs.status == app.RunStatusStreaming) {
					pendingOutput.opKey = node.ID
					pendingOutput.depth = node.Depth
					pendingOutput.rs = rs
				}
			}

			// If this is the last node, render any pending output
			if i == len(visible)-1 && pendingOutput.rs != nil {
				indent := strings.Repeat(" ", (pendingOutput.depth+1)*2)
				sb.WriteString(m.renderRunState(indent, pendingOutput.rs))
			}
		}
	}

	return sb.String()
}

// renderTreeNode renders a single tree node with proper styling.
func (m *model) renderTreeNode(node *TreeNode, selected bool, t tab) string {
	var parts []string

	// Indentation
	indent := strings.Repeat(" ", node.Depth*2)
	parts = append(parts, indent)

	// Expand/collapse indicator (styled same as label when selected)
	indicator := m.getNodeIndicator(node, selected)
	labelStyle := m.getNodeLabelStyle(node, selected)
	parts = append(parts, labelStyle.Render(indicator), " ")

	// Label with styling
	parts = append(parts, labelStyle.Render(node.Label))

	// Badge
	if node.Badge != "" {
		badgeStyle := m.getNodeBadgeStyle(node)
		parts = append(parts, " ", badgeStyle.Render(node.Badge))
	}

	// Binding format badge for operations
	if node.Type == NodeTypeOperation && t.obi != nil {
		bindingBadge := m.defaultBindingBadge(node.ID, t.obi)
		if bindingBadge != "" {
			parts = append(parts, " ", bindingBadge)
		}
	}

	// Run status for operations
	if node.Type == NodeTypeOperation && t.runState != nil {
		if rs, ok := t.runState[node.ID]; ok {
			parts = append(parts, " ", m.renderRunStatusBadge(rs))
		}
	}

	// Description for schema properties (inline if short)
	if (node.Type == NodeTypeSchemaProp || node.Type == NodeTypeSchema) && node.Data != nil {
		if desc, ok := node.Data.(string); ok && desc != "" && len(desc) < 50 {
			parts = append(parts, " ", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render("— "+desc))
		}
	}

	return strings.Join(parts, "")
}

func (m *model) getNodeIndicator(node *TreeNode, selected bool) string {
	// Custom icon overrides default behavior
	if node.Icon != "" {
		return node.Icon
	}

	// Default: arrows for expandable nodes, bullets for leaves
	if node.IsLeaf() {
		if selected {
			return "•"
		}
		return "·"
	}

	if selected {
		if node.Expanded {
			return "▼"
		}
		return "▶"
	}

	if node.Expanded {
		return "▽"
	}
	return "▷"
}

func (m *model) getNodeLabelStyle(node *TreeNode, selected bool) lipgloss.Style {
	style := lipgloss.NewStyle()

	if selected {
		return style.Bold(true).Foreground(lipgloss.Color("14"))
	}

	switch node.Type {
	case NodeTypeOperation:
		return style.Foreground(lipgloss.Color("7"))
	case NodeTypeSchemaSection:
		return style.Bold(true).Foreground(lipgloss.Color("7"))
	case NodeTypeSchema, NodeTypeSchemaRef:
		return style.Foreground(lipgloss.Color("6"))
	case NodeTypeSchemaProp:
		return style.Foreground(lipgloss.Color("7"))
	case NodeTypeBindings:
		return style.Bold(true).Foreground(lipgloss.Color("7"))
	case NodeTypeBinding:
		return style.Foreground(lipgloss.Color("12"))
	case NodeTypeAliases, NodeTypeSatisfies:
		return style.Bold(true).Foreground(lipgloss.Color("7"))
	case NodeTypeAlias:
		return style.Foreground(lipgloss.Color("6"))
	case NodeTypeSatisfiesRef:
		return style.Foreground(lipgloss.Color("13"))
	default:
		return style.Foreground(lipgloss.Color("7"))
	}
}

func (m *model) getNodeBadgeStyle(node *TreeNode) lipgloss.Style {
	style := lipgloss.NewStyle()

	switch node.Type {
	case NodeTypeOperation:
		if strings.Contains(node.Badge, "[deprecated]") {
			return style.Foreground(lipgloss.Color("9"))
		}
		if strings.Contains(node.Badge, "[idempotent]") {
			return style.Foreground(lipgloss.Color("6"))
		}
		return style.Foreground(lipgloss.Color("8"))
	case NodeTypeSchema, NodeTypeSchemaRef, NodeTypeSchemaSection, NodeTypeSchemaProp:
		if strings.Contains(node.Badge, "*") {
			return style.Foreground(lipgloss.Color("3")) // Required fields in yellow
		}
		return style.Foreground(lipgloss.Color("8"))
	case NodeTypeBinding:
		if node.Badge == "[deprecated]" {
			return style.Foreground(lipgloss.Color("9"))
		}
		return style.Foreground(lipgloss.Color("8"))
	default:
		return style.Foreground(lipgloss.Color("8"))
	}
}

func (m *model) renderRunStatusBadge(rs *opRunState) string {
	if rs == nil {
		return ""
	}
	switch rs.status {
	case app.RunStatusRunning:
		spinner := spinnerFrames[rs.frame%len(spinnerFrames)]
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(spinner)
	case app.RunStatusStreaming:
		spinner := spinnerFrames[rs.frame%len(spinnerFrames)]
		return lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render(spinner)
	case app.RunStatusSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓")
	case app.RunStatusError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗")
	default:
		return ""
	}
}

func (m *model) viewNoOBI() string {
	t := m.tabs[m.active]
	status := t.probe.status
	detail := t.probe.detail

	var msg string
	switch status {
	case app.ProbeStatusProbing:
		msg = "Loading..."
	case app.ProbeStatusBad:
		if detail != "" {
			msg = fmt.Sprintf("No OpenBindings interface found at %s\n\n%s", t.url, detail)
		} else {
			msg = fmt.Sprintf("No OpenBindings interface found at %s", t.url)
		}
	default:
		msg = fmt.Sprintf("No OpenBindings interface found at %s", t.url)
	}

	return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(msg)
}

func (m *model) viewZeroOperations() string {
	t := m.tabs[m.active]
	title := t.obi.Name
	if title == "" {
		title = "Unnamed Interface"
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).Render(title) + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("This interface has no operations.")
}

func (m *model) defaultBindingBadge(opKey string, iface *openbindings.Interface) string {
	_, best := app.DefaultBindingForOp(opKey, iface)
	if best == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("[no binding]")
	}
	sourceName := bindingDisplayName(best, iface)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(fmt.Sprintf("[%s]", sourceName))
}

func (m *model) renderRunState(indent string, runState *opRunState) string {
	if runState == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(indent)
	switch runState.status {
	case app.RunStatusRunning:
		spinner := spinnerFrames[runState.frame%len(spinnerFrames)]
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(spinner + " Running..."))
		if runState.inputName != "" {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf(" (using %s)", runState.inputName)))
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" [c to cancel]"))
	case app.RunStatusStreaming:
		spinner := spinnerFrames[runState.frame%len(spinnerFrames)]
		evLabel := fmt.Sprintf("%d events", runState.eventCount)
		if runState.eventCount == 1 {
			evLabel = "1 event"
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render(spinner+" Listening..."))
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf(" (%s)", evLabel)))
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" [c to cancel]"))
		if runState.output != "" {
			sb.WriteString("\n")
			sb.WriteString(m.viewRunOutput(runState, indent))
		}
	case app.RunStatusSuccess:
		header := "Output"
		if runState.inputName != "" {
			header = fmt.Sprintf("Output (%s)", runState.inputName)
		}
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render(header))
		if runState.durationMs > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf(" (%dms)", runState.durationMs)))
		}
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render(":"))
		sb.WriteString("\n")
		sb.WriteString(m.viewRunOutput(runState, indent))
	case app.RunStatusError:
		sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("Error: "))
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(runState.error))
		if runState.durationMs > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf(" (%dms)", runState.durationMs)))
		}
		if runState.inputName != "" {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf(" (using %s)", runState.inputName)))
		}
		if runState.output != "" {
			sb.WriteString("\n")
			sb.WriteString(m.viewRunOutput(runState, indent))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// selectedOperationLine returns the zero-based line index of the selected node
// within the rendered operations view. Returns -1 if not applicable.
// The result is cached for the current frame.
func (m *model) selectedOperationLine() int {
	if m.renderCache != nil && m.renderCache.selLineSet {
		return m.renderCache.selLine
	}
	result := m.selectedOperationLineUncached()
	if m.renderCache == nil {
		m.renderCache = &frameCache{}
	}
	m.renderCache.selLine = result
	m.renderCache.selLineSet = true
	return result
}

func (m *model) selectedOperationLineUncached() int {
	t := m.tabs[m.active]
	if t.obi == nil || t.tree == nil {
		return -1
	}

	selectedIdx := t.tree.SelectedIndex()
	if selectedIdx < 0 {
		return -1
	}

	// Count header lines
	header := m.viewOperationsHeader(t)
	if m.viewport.Width > 0 {
		header = lipgloss.NewStyle().Width(m.viewport.Width).Render(header)
	}
	headerLines := strings.Count(header, "\n")

	// Each visible node takes one line, plus output blocks after operation children
	visible := t.tree.VisibleNodes()
	lineCount := headerLines

	var pendingOutput struct {
		depth int
		rs    *opRunState
	}

	for i, node := range visible {
		// Check if we need to count pending output lines before this node
		if pendingOutput.rs != nil && node.Depth <= pendingOutput.depth {
			indent := strings.Repeat(" ", (pendingOutput.depth+1)*2)
			outputView := m.renderRunState(indent, pendingOutput.rs)
			lineCount += strings.Count(outputView, "\n")
			pendingOutput.rs = nil
		}

		if i == selectedIdx {
			return lineCount
		}
		lineCount++ // The node itself

		// Queue output for expanded operations
		if node.Type == NodeTypeOperation && node.Expanded {
			if rs, ok := t.runState[node.ID]; ok && (rs.status == app.RunStatusSuccess || rs.status == app.RunStatusError || rs.status == app.RunStatusRunning || rs.status == app.RunStatusStreaming) {
				pendingOutput.depth = node.Depth
				pendingOutput.rs = rs
			}
		}
	}

	// Count any remaining pending output
	if pendingOutput.rs != nil {
		indent := strings.Repeat(" ", (pendingOutput.depth+1)*2)
		outputView := m.renderRunState(indent, pendingOutput.rs)
		lineCount += strings.Count(outputView, "\n")
	}

	return -1
}

func (m *model) viewOperationsHeader(t tab) string {
	var sb strings.Builder
	title := t.obi.Name
	if title == "" {
		title = "Unnamed Interface"
	}
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).Render(title))
	if t.obi.Version != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(" v" + t.obi.Version))
	}
	sb.WriteString("\n")
	if t.obi.Description != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(t.obi.Description))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Operations"))
	sb.WriteString(fmt.Sprintf(" (%d)\n\n", len(t.opKeys)))
	return sb.String()
}

func (m *model) wrappedLineCount(s string) int {
	if m.viewport.Width <= 0 {
		return countLines(s)
	}
	wrapped := lipgloss.NewStyle().Width(m.viewport.Width).Render(s)
	return countLines(wrapped)
}

const streamTailLines = 20
const maxStreamBufferLines = 200

func (m *model) viewRunOutput(runState *opRunState, indent string) string {
	if runState.output == "" {
		return indent + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("(no output)")
	}

	lines := strings.Split(runState.output, "\n")
	var content string

	if runState.streaming && len(lines) > streamTailLines {
		tail := lines[len(lines)-streamTailLines:]
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
			fmt.Sprintf("... %d earlier events hidden", len(lines)-streamTailLines)) +
			"\n" + highlightOutput(strings.Join(tail, "\n"))
	} else if runState.expanded || len(lines) <= 5 {
		content = highlightOutput(runState.output)
	} else {
		preview := strings.Join(lines[:3], "\n")
		content = highlightOutput(preview) + "\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("... %d more lines (press 'o' to expand)", len(lines)-3))
	}

	// Calculate available width for the box (account for indent and border)
	boxWidth := m.viewport.Width - len(indent) - 4
	if boxWidth < 20 {
		boxWidth = 60 // fallback
	}

	// Apply the output box style
	boxStyle := outputBoxStyle.Width(boxWidth)
	boxedOutput := boxStyle.Render(content)

	// Indent each line of the box
	boxLines := strings.Split(boxedOutput, "\n")
	var sb strings.Builder
	for _, line := range boxLines {
		sb.WriteString(indent)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func findBindingsForOp(opKey string, iface *openbindings.Interface) map[string]openbindings.BindingEntry {
	if iface == nil {
		return nil
	}
	result := map[string]openbindings.BindingEntry{}
	for k, b := range iface.Bindings {
		if b.Operation == opKey {
			result[k] = b
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
