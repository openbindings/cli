package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) viewScrollbar() string {
	if m.viewport.Height <= 0 {
		return ""
	}
	totalLines := m.wrappedLineCount(m.viewOperations())
	maxOffset := totalLines - m.viewport.Height
	trackHeight := m.viewport.Height
	if trackHeight <= 0 {
		return ""
	}
	selectedLine := m.selectedOperationLine()
	if totalLines <= trackHeight {
		return ""
	}

	thumb := int(math.Round(float64(trackHeight) * float64(trackHeight) / float64(totalLines)))
	if thumb < 1 {
		thumb = 1
	}
	if thumb > trackHeight {
		thumb = trackHeight
	}

	maxThumbTop := trackHeight - thumb
	thumbTop := 0
	if maxOffset > 0 && maxThumbTop > 0 {
		thumbTop = int(math.Round(float64(m.viewport.YOffset) / float64(maxOffset) * float64(maxThumbTop)))
	}
	if thumbTop < 0 {
		thumbTop = 0
	}
	if thumbTop > maxThumbTop {
		thumbTop = maxThumbTop
	}

	lines := make([]string, trackHeight)
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i := 0; i < trackHeight; i++ {
		ch := trackStyle.Render("░")
		if i >= thumbTop && i < thumbTop+thumb {
			ch = "▒"
		}
		if selectedLine >= 0 && trackHeight > 0 {
			selPos := 0
			if totalLines <= trackHeight {
				selPos = selectedLine
			} else if totalLines > 1 && trackHeight > 1 {
				selPos = int(math.Round(float64(selectedLine) * float64(trackHeight-1) / float64(totalLines-1)))
			}
			if selPos < 0 {
				selPos = 0
			}
			if selPos > trackHeight-1 {
				selPos = trackHeight - 1
			}
			if i == selPos {
				ch = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Render("█")
			}
		}
		lines[i] = ch
	}
	return strings.Join(lines, "\n")
}
