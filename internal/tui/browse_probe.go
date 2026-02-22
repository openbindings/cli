package tui

import (
	"encoding/json"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	openbindings "github.com/openbindings/openbindings-go"

	"github.com/openbindings/cli/internal/app"
)

type probeResult struct {
	status string // "idle" | "probing" | "ok" | "bad"
	detail string
	obi    string
	obiURL string

	// Parsed OBI interface (nil if parse failed or no OBI)
	parsed *openbindings.Interface
	// Sorted operation keys for stable ordering
	opKeys []string

	// Final URL after redirects
	finalURL string

	// OBI base directory for resolving relative artifact paths.
	// Set for file-path targets. Empty for exec: targets.
	obiDir string
}

type probeResultMsg struct {
	tabID  int
	result probeResult
}

func probeCmd(tabID int, rawURL string) tea.Cmd {
	return func() tea.Msg {
		result := app.ProbeOBI(rawURL, 2*time.Second)
		pr := probeResult{
			status:   result.Status,
			detail:   result.Detail,
			obi:      result.OBI,
			obiURL:   result.OBIURL,
			finalURL: result.FinalURL,
			obiDir:   result.OBIDir,
		}

		// Try to parse the OBI JSON
		if result.OBI != "" {
			var iface openbindings.Interface
			if err := json.Unmarshal([]byte(result.OBI), &iface); err == nil {
				pr.parsed = &iface
				pr.opKeys = sortedOpKeys(iface.Operations)
			}
		}

		return probeResultMsg{
			tabID:  tabID,
			result: pr,
		}
	}
}

func sortedOpKeys(ops map[string]openbindings.Operation) []string {
	keys := make([]string, 0, len(ops))
	for k := range ops {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
