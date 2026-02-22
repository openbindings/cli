package tui

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// Output box style with subtle border
var outputBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("8")).
	Padding(0, 1).
	MarginTop(1)

// chromaStyle is the color scheme for syntax highlighting.
// Using "dracula" for good contrast on dark terminals.
var chromaStyle = styles.Get("dracula")

// chromaFormatter outputs 256-color ANSI codes for terminal display.
var chromaFormatter = formatters.Get("terminal256")

func init() {
	// Fallback if styles/formatters not found
	if chromaStyle == nil {
		chromaStyle = styles.Fallback
	}
	if chromaFormatter == nil {
		chromaFormatter = formatters.Fallback
	}
}

// highlightOutput detects the format and applies syntax highlighting.
func highlightOutput(input string) string {
	if input == "" {
		return input
	}

	trimmed := strings.TrimSpace(input)

	// Detect lexer based on content
	var lexer chroma.Lexer
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		lexer = lexers.Get("json")
	} else if looksLikeYAML(trimmed) {
		lexer = lexers.Get("yaml")
	} else if strings.HasPrefix(trimmed, "<") {
		lexer = lexers.Get("xml")
	}

	// Fallback to plain text if no lexer matched
	if lexer == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render(input)
	}

	// Ensure lexer uses coalescing for cleaner tokens
	lexer = chroma.Coalesce(lexer)

	// Tokenize
	iterator, err := lexer.Tokenise(nil, input)
	if err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render(input)
	}

	// Format to string
	var buf bytes.Buffer
	err = chromaFormatter.Format(&buf, chromaStyle, iterator)
	if err != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render(input)
	}

	return buf.String()
}

// looksLikeYAML does a quick check for YAML-like content.
func looksLikeYAML(s string) bool {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return false
	}

	// Check first few non-empty lines for "key: value" pattern
	checked := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Look for "key:" pattern (with or without value)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := line[:idx]
			// Key should be alphanumeric (with underscores/hyphens)
			if isValidYAMLKey(key) {
				checked++
				if checked >= 2 {
					return true
				}
			}
		} else {
			// Line doesn't have colon - probably not YAML
			return false
		}
	}
	return checked > 0
}

func isValidYAMLKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
				return false
			}
		}
	}
	return true
}
