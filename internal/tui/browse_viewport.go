package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ensureSelectionVisible scrolls the viewport to keep the selected node visible.
func (m *model) ensureSelectionVisible() {
	m.syncViewport()
	line := m.selectedOperationLine()
	if line < 0 || m.viewport.Height <= 0 {
		return
	}
	top := m.viewport.YOffset
	bottom := top + m.viewport.Height - 1
	if line < top {
		m.viewport.YOffset = line
	} else if line > bottom {
		m.viewport.YOffset = line - m.viewport.Height + 1
	}
	m.syncViewport()
}

// syncViewport updates the viewport dimensions and content.
func (m *model) syncViewport() {
	innerW := clampMin(m.width-2, 0)
	innerH := clampMin(m.height-2, 0)
	contentWFull := clampMin(innerW-4, 0) // padding left+right (2 each)
	contentH := clampMin(innerH-2, 0)     // padding top+bottom (1 each)

	header := m.viewURLBar() + "\n" + m.viewTabs()
	footer := m.viewFooter(contentWFull)
	headerLines := wrappedLineCountWithWidth(header, contentWFull)
	footerLines := wrappedLineCountWithWidth(footer, contentWFull)

	// Account for blank lines between header/body and body/footer.
	availH := contentH - headerLines - footerLines - 2
	if availH < 1 {
		availH = 1
	}

	// Decide if we need a scrollbar based on wrapped content height.
	body := m.viewOperations()
	contentFits := wrappedLineCountWithWidth(body, contentWFull) <= availH
	contentW := contentWFull
	if !contentFits && contentWFull > 0 {
		contentW = contentWFull - 1 // reserve space for scrollbar
	}

	m.viewport.Width = contentW
	m.viewport.Height = availH

	if m.tabs[m.active].url == "" {
		m.viewport.SetContent("")
		return
	}
	wrapped := lipgloss.NewStyle().Width(m.viewport.Width).Render(m.viewOperations())
	m.viewport.SetContent(wrapped)
}

// wrappedLineCountWithWidth returns the number of lines after wrapping to width.
func wrappedLineCountWithWidth(s string, width int) int {
	if width <= 0 {
		return countLines(s)
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	return countLines(wrapped)
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 1
	}
	return strings.Count(s, "\n") + 1
}
