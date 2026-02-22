package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) viewURLBar() string {
	// cli-js style: ◀ ▶ then either "❯ <input>" (focused) or the current URL / hint (muted).
	avail := clampMin(m.width-2-4, 0) // outer border + our padding(1,2)

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// We don't have history yet; render disabled nav arrows like cli-js does when unavailable.
	nav := muted.Render("◀") + " " + muted.Render("▶") + "  "

	if m.focusURL {
		fixed := lipgloss.Width(nav) + lipgloss.Width("❯ ")
		w := avail - fixed
		if w < 1 {
			w = 1
		}
		m.urlInput.Width = w

		line := nav + accent.Render("❯ ") + m.urlInput.View()
		// Pad to width (textinput might be shorter depending on value).
		if pad := avail - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		if avail > 0 {
			return lipgloss.NewStyle().Width(avail).Render(line)
		}
		return line
	}

	current := strings.TrimSpace(m.tabs[m.active].url)
	if current == "" {
		current = "Press ^l to enter a URL"
		return nav + muted.Render(current)
	}
	line := nav + text.Render(current)
	if pad := avail - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	if avail > 0 {
		return lipgloss.NewStyle().Width(avail).Render(line)
	}
	return line
}
