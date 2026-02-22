package tui

import "github.com/charmbracelet/lipgloss"

func (m *model) viewBody() string {
	t := m.tabs[m.active]
	if t.url == "" {
		return m.viewHome()
	}
	content := m.viewport.View()
	scrollbar := m.viewScrollbar()
	if scrollbar == "" {
		return lipgloss.NewStyle().Width(m.viewport.Width).Render(content)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, content, scrollbar)
	return lipgloss.NewStyle().Width(m.viewport.Width + 1).Render(body)
}
