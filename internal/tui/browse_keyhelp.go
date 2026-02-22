package tui

import (
	"strings"
)

type keyHelpEntry struct {
	key   string
	label string
}

func keyHelp(keys ...keyHelpEntry) string {
	var parts []string
	for _, k := range keys {
		parts = append(parts, k.key+": "+k.label)
	}
	return strings.Join(parts, "   ")
}

var keyHelpURLFocus = []keyHelpEntry{
	{key: "enter", label: "apply"},
	{key: "esc/ctrl+g", label: "cancel"},
}

var keyHelpBrowse = []keyHelpEntry{
	{key: "j/k", label: "navigate"},
	{key: "alt+↑/↓", label: "sibling"},
	{key: "h/l", label: "collapse/expand"},
	{key: "enter", label: "run/select"},
	{key: "e", label: "edit"},
	{key: "ctrl+r", label: "refresh"},
	{key: "ctrl+s", label: "save"},
	{key: "q", label: "quit"},
}

var keyHelpIdle = []keyHelpEntry{
	{key: "t", label: "new tab"},
	{key: "tab", label: "switch"},
	{key: "u/ctrl+l", label: "edit url"},
	{key: "x", label: "close"},
	{key: "ctrl+s", label: "save"},
	{key: "q", label: "quit"},
}
