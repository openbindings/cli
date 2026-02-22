package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/openbindings/cli/internal/app"
)

func clampMin(n, min int) int {
	if n < min {
		return min
	}
	return n
}

func (m *model) View() string {
	// Wait until we get an initial window size.
	if m.width <= 0 || m.height <= 0 {
		return "Loading..."
	}

	// If modal is active, show modal view instead
	if m.newInputModal != nil {
		return m.viewNewInputModal()
	}

	// Match the older cli-js "browser-ish" layout:
	// URL bar first, then tabs, then a separator before the main content.
	contentW := clampMin(m.width-2-4, 0) // border + padding(1,2)
	header := lipgloss.NewStyle().Width(contentW).Render(m.viewURLBar() + "\n" + m.viewTabs())
	main := lipgloss.NewStyle().Width(contentW).Render(m.viewBody())
	footer := lipgloss.NewStyle().Width(contentW).Render(m.viewFooter(contentW))
	content := fmt.Sprintf("%s\n\n%s\n\n%s", header, main, footer)

	// Full-screen border (edge-to-edge).
	innerW := clampMin(m.width-2, 0)
	innerH := clampMin(m.height-2, 0)

	padded := lipgloss.NewStyle().Padding(1, 2).Render(content)
	inner := lipgloss.Place(innerW, innerH, lipgloss.Left, lipgloss.Top, padded)

	frame := lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8"))

	return frame.Render(inner)
}

func (m *model) viewFooter(width int) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var helpText string

	// Contextual key help based on current state
	if m.focusURL {
		helpText = style.Render(keyHelp(keyHelpURLFocus...))
	} else {
		t := m.tabs[m.active]
		if t.obi != nil && len(t.opKeys) > 0 {
			// Get contextual help based on selected node
			helpText = style.Render(keyHelp(m.contextualKeyHelp(t)...))
		} else {
			helpText = style.Render(keyHelp(keyHelpIdle...))
		}
	}

	// Build full-width divider
	divider := dividerStyle.Render(strings.Repeat("─", width))
	footer := divider + "\n" + helpText

	if m.statusMsg != "" {
		return footer + "   " + statusStyle.Render(m.statusMsg)
	}
	return footer
}

// contextualKeyHelp returns key help entries based on the selected node type
func (m *model) contextualKeyHelp(t tab) []keyHelpEntry {
	if t.tree == nil {
		return keyHelpBrowse
	}

	node := t.tree.SelectedNode()
	if node == nil {
		return keyHelpBrowse
	}

	// Base navigation keys
	base := []keyHelpEntry{
		{key: "j/k", label: "navigate"},
		{key: "h/l", label: "collapse/expand"},
	}

	// Add node-type-specific keys
	switch node.Type {
	case NodeTypeOperation:
		keys := append(base, keyHelpEntry{key: "enter", label: "run"})
		// Show c: cancel if running, y: yank if has output
		if rs, ok := t.runState[node.ID]; ok {
			if rs.status == app.RunStatusRunning || rs.status == app.RunStatusStreaming {
				keys = append(keys, keyHelpEntry{key: "c", label: "cancel"})
			}
			if rs.output != "" {
				keys = append(keys, keyHelpEntry{key: "o", label: "copy output"})
			}
		}
		return append(keys, keyHelpEntry{key: "q", label: "quit"})
	case NodeTypeInputFile:
		return append(base,
			keyHelpEntry{key: "enter", label: "select+run"},
			keyHelpEntry{key: "e", label: "edit"},
			keyHelpEntry{key: "r", label: "remove"},
			keyHelpEntry{key: "q", label: "quit"},
		)
	case NodeTypeInputNew:
		return append(base,
			keyHelpEntry{key: "enter", label: "create"},
			keyHelpEntry{key: "q", label: "quit"},
		)
	case NodeTypeInputExample:
		return append(base,
			keyHelpEntry{key: "enter", label: "run with example"},
			keyHelpEntry{key: "q", label: "quit"},
		)
	default:
		return keyHelpBrowse
	}
}

// viewNewInputModal renders the full-screen modal for creating a new input
func (m *model) viewNewInputModal() string {
	modal := m.newInputModal
	t := m.tabs[m.active]

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	subtitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	previewBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	contentW := clampMin(m.width-2-4, 0)
	innerW := clampMin(m.width-2, 0)
	innerH := clampMin(m.height-2, 0)

	var sb strings.Builder

	// Context header: interface name › operation
	interfaceName := modal.interfaceName
	if interfaceName == "" {
		interfaceName = t.url
	}
	sb.WriteString(dimStyle.Render(interfaceName))
	sb.WriteString(dimStyle.Render(" › "))
	sb.WriteString(subtitleStyle.Render(modal.opKey))
	sb.WriteString("\n")

	// Title
	sb.WriteString(titleStyle.Render("Create New Input"))
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n\n")

	// Name field
	nameLabel := "Name: "
	if modal.focusField == 0 {
		nameLabel = labelStyle.Render(nameLabel)
	} else {
		nameLabel = dimStyle.Render(nameLabel)
	}
	sb.WriteString(nameLabel)
	sb.WriteString(modal.nameInput.View())
	if modal.nameTaken {
		sb.WriteString(" ")
		sb.WriteString(errorStyle.Render("(name taken)"))
	}
	sb.WriteString("\n\n")

	// Template selection
	templateLabel := "Template:"
	if modal.focusField == 1 {
		templateLabel = labelStyle.Render(templateLabel)
	} else {
		templateLabel = dimStyle.Render(templateLabel)
	}
	sb.WriteString(templateLabel)
	sb.WriteString("\n")
	for i, tmpl := range modal.templates {
		prefix := "  ○ "
		style := dimStyle
		if i == modal.selectedTemplate {
			prefix = "  ● "
			if modal.focusField == 1 {
				style = selectedStyle
			}
		}
		sb.WriteString(style.Render(prefix + tmpl.Label))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Check if "existing file" is selected
	isExisting := modal.selectedTemplate >= 0 &&
		modal.selectedTemplate < len(modal.templates) &&
		modal.templates[modal.selectedTemplate].ID == "existing"

	if isExisting {
		// Show path input instead of preview
		pathLabel := "File path: "
		if modal.focusField == 2 {
			pathLabel = labelStyle.Render(pathLabel)
		} else {
			pathLabel = dimStyle.Render(pathLabel)
		}
		sb.WriteString(pathLabel)
		sb.WriteString(modal.pathInput.View())
		if modal.pathError != "" {
			sb.WriteString(" ")
			sb.WriteString(errorStyle.Render("(" + modal.pathError + ")"))
		}
		sb.WriteString("\n")
	} else {
		// Preview
		sb.WriteString(labelStyle.Render("Preview:"))
		sb.WriteString("\n")
		preview := ""
		if modal.selectedTemplate >= 0 && modal.selectedTemplate < len(modal.templates) {
			preview = modal.templates[modal.selectedTemplate].Preview
		}
		if preview == "" {
			preview = "{}"
		}
		// Limit preview height
		previewLines := strings.Split(preview, "\n")
		maxPreviewLines := 10
		if len(previewLines) > maxPreviewLines {
			previewLines = append(previewLines[:maxPreviewLines], "  ...")
		}
		previewContent := strings.Join(previewLines, "\n")
		previewW := clampMin(contentW-4, 20)
		sb.WriteString(previewBoxStyle.Width(previewW).Render(previewContent))
		sb.WriteString("\n")
	}

	// Footer with key help
	sb.WriteString("\n")
	sb.WriteString(dividerStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n")
	var footerHelp string
	if isExisting {
		footerHelp = dimStyle.Render("esc: cancel    tab: next field    ↑/↓: select template    enter: add reference")
	} else {
		footerHelp = dimStyle.Render("esc: cancel    tab: next field    ↑/↓: select template    enter: create")
	}
	sb.WriteString(footerHelp)

	// Frame it
	content := sb.String()
	padded := lipgloss.NewStyle().Padding(1, 2).Render(content)
	inner := lipgloss.Place(innerW, innerH, lipgloss.Left, lipgloss.Top, padded)

	frame := lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")) // Highlight border for modal

	return frame.Render(inner)
}
