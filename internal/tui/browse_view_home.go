package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) viewHome() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")).Render("openbindings browser")
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Enter a target URL or exec: reference to get started.")

	help := lipgloss.NewStyle().Render(strings.Join([]string{
		"t: new tab",
		"tab/l: next tab",
		"shift+tab/h: previous tab",
		"u: set tab URL",
		"x: close tab",
		"q: quit",
	}, "\n"))

	return fmt.Sprintf("%s\n%s\n\n%s", title, desc, help)
}
