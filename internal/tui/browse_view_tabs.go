package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/openbindings/cli/internal/app"
)

func (m *model) viewTabs() string {
	// cli-js style:
	// - active tab:    [label] (bold, tabActive color)
	// - inactive tabs: label (muted)
	// - compact mode:  active uses ›label
	avail := clampMin(m.width-2-4, 0) // outer border + our padding(1,2)
	compact := avail < 60

	border := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	tabActive := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errS := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// Reserve space for dirty indicator and workspace name
	dirtyIndicator := ""
	workspaceLabel := ""
	if m.workspace != nil {
		workspaceLabel = m.workspace.Name
		if m.dirty {
			dirtyIndicator = " [*]"
		}
	}
	reservedRight := lipgloss.Width(workspaceLabel+dirtyIndicator) + 2

	var out strings.Builder
	used := 0
	tabsAvail := avail - reservedRight

	for i, t := range m.tabs {
		label := "home ⌂"
		if t.obi != nil && t.obi.Name != "" {
			label = t.obi.Name
		} else if t.url != "" {
			// Fallback to URL while loading
			label = t.url
		}

		// Status icon (match cli-js: ○ loading, ● ok/error; color conveys status).
		icon := ""
		iconStyle := muted
		switch t.probe.status {
	case app.ProbeStatusProbing:
			icon = "○"
			iconStyle = warn
	case app.ProbeStatusOK:
			icon = "●"
			iconStyle = success
	case app.ProbeStatusBad:
			icon = "●"
			iconStyle = errS
		}
		showIcon := !compact && icon != ""

		// Rough truncation to keep the strip on one line.
		maxLabel := 28
		if compact {
			maxLabel = 20
		}
		if len(label) > maxLabel {
			label = label[:maxLabel-1] + "…"
		}

		var piece string
		if i == m.active {
			if compact {
				piece = tabActive.Render("›"+label) + " "
			} else {
				if showIcon {
					piece = tabActive.Render("["+label) + iconStyle.Render(" "+icon) + tabActive.Render("]") + " "
				} else {
					piece = tabActive.Render("["+label+"]") + " "
				}
			}
		} else {
			if compact {
				piece = muted.Render(" "+label) + " "
			} else {
				if showIcon {
					piece = muted.Render(label) + iconStyle.Render(" "+icon) + muted.Render(" ")
				} else {
					piece = muted.Render(label) + muted.Render(" ")
				}
			}
		}

		w := lipgloss.Width(piece)
		if tabsAvail > 0 && used+w > tabsAvail {
			break
		}
		out.WriteString(piece)
		used += w
	}

	// Add workspace name and dirty indicator on the right
	rightPart := ""
	if m.workspace != nil {
		rightPart = muted.Render(workspaceLabel)
		if m.dirty {
			rightPart += warn.Render(" [*]")
		}
	}

	// Pad to full width so the separator lines up cleanly.
	padding := avail - used - lipgloss.Width(rightPart)
	if padding < 0 {
		padding = 0
	}
	out.WriteString(strings.Repeat(" ", padding))
	out.WriteString(rightPart)

	sep := border.Render(strings.Repeat("─", avail))
	return out.String() + "\n" + sep
}
