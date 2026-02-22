// Package app - delegates_list.go contains the CLI command for listing delegates.
package app

import (
	"strings"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

// DelegateListParams configures the delegate list command.
type DelegateListParams struct {
	IncludeBuiltin bool
	OutputFormat   string
	OutputPath     string
}

// DelegateFormatInfo represents a format with its preference status.
type DelegateFormatInfo struct {
	Format      string `json:"format"`
	Description string `json:"description,omitempty"`
	Preferred   bool   `json:"preferred"`
}

// DelegateListEntry represents a delegate with its formats.
type DelegateListEntry struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Source      string               `json:"source"`
	Location    string               `json:"location,omitempty"`
	Formats     []DelegateFormatInfo `json:"formats"`
}

// DelegateListOutput is the output of the delegate list operation.
type DelegateListOutput struct {
	Delegates []DelegateListEntry `json:"delegates"`
	Error     *Error              `json:"error,omitempty"`
}

// Render returns a human-friendly representation.
func (o DelegateListOutput) Render() string {
	s := Styles
	var sb strings.Builder

	if o.Error != nil {
		sb.WriteString(s.Error.Render("Error: "))
		sb.WriteString(o.Error.Message)
		return sb.String()
	}

	if len(o.Delegates) == 0 {
		return s.Dim.Render("No delegates in workspace")
	}

	sb.WriteString(s.Header.Render("Delegates:"))

	// Group delegates by source
	var builtin, workspace []DelegateListEntry
	for _, p := range o.Delegates {
		switch p.Source {
		case delegates.SourceBuiltin:
			builtin = append(builtin, p)
		case delegates.SourceWorkspace:
			workspace = append(workspace, p)
		}
	}

	// Render builtin first
	for _, p := range builtin {
		sb.WriteString("\n\n  ")
		sb.WriteString(s.Key.Render(p.Name))
		sb.WriteString(s.Dim.Render(" (builtin)"))
		if p.Description != "" {
			sb.WriteString("\n      ")
			sb.WriteString(s.Dim.Render(p.Description))
		}
		renderDelegateFormats(&sb, p, s)
	}

	// Render workspace delegates
	if len(workspace) > 0 {
		sb.WriteString("\n\n  ")
		sb.WriteString(s.Header.Render("Workspace"))
		for _, p := range workspace {
			sb.WriteString("\n    ")
			sb.WriteString(s.Key.Render(p.Name))
			if p.Location != "" {
				sb.WriteString(s.Dim.Render(" " + p.Location))
			}
			if p.Description != "" {
				sb.WriteString("\n      ")
				sb.WriteString(s.Dim.Render(p.Description))
			}
			renderDelegateFormats(&sb, p, s)
		}
	}

	return sb.String()
}

func renderDelegateFormats(sb *strings.Builder, p DelegateListEntry, s styles) {
	if len(p.Formats) == 0 {
		sb.WriteString("\n      ")
		sb.WriteString(s.Dim.Render("(no formats)"))
		return
	}
	for _, f := range p.Formats {
		sb.WriteString("\n      ")
		sb.WriteString(s.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(f.Format)
		if f.Description != "" {
			sb.WriteString(s.Dim.Render(" - " + f.Description))
		}
		if f.Preferred {
			sb.WriteString("  ")
			sb.WriteString(s.Success.Render("✓ preferred"))
		}
	}
}

// DelegateList is the CLI command handler for listing delegates.
func DelegateList(params DelegateListParams) error {
	output := BuildDelegateListOutput(params)
	if output.Error != nil {
		return exitText(1, output.Error.Message, true)
	}
	return OutputResult(output, params.OutputFormat, params.OutputPath)
}

// BuildDelegateListOutput builds the delegate list output.
func BuildDelegateListOutput(params DelegateListParams) DelegateListOutput {
	// Load the active workspace
	envPath, err := FindEnvPath()
	if err != nil {
		return DelegateListOutput{
			Error: &Error{Code: "no_env", Message: "no environment found; run 'ob init' first"},
		}
	}
	ws, _, err := LoadActiveWorkspace(envPath)
	if err != nil {
		return DelegateListOutput{
			Error: &Error{Code: "load_failed", Message: err.Error()},
		}
	}

	// Discover delegates from workspace
	disco := delegates.DiscoverParams{
		IncludeBuiltin:     params.IncludeBuiltin,
		WorkspaceDelegates: ws.Delegates,
	}
	discovered, err := delegates.Discover(disco)
	if err != nil {
		return DelegateListOutput{
			Error: &Error{Code: "discovery_failed", Message: err.Error()},
		}
	}

	var entries []DelegateListEntry
	for _, p := range discovered {
		entry := DelegateListEntry{
			Name:     p.Name,
			Source:   p.Source,
			Location: p.Location,
		}

		// Get formats for this delegate
		var formatInfos []delegates.FormatInfo
		if p.Source == delegates.SourceBuiltin {
			// For builtin, enumerate registered handlers and collect their formats
			registry := DefaultRegistry()
			for _, h := range registry.All() {
				formatInfos = append(formatInfos, h.ListFormats()...)
			}
			// Include the openbindings format itself
			formatInfos = append(formatInfos, delegates.FormatInfo{
				Token:       "openbindings@" + openbindings.MaxTestedVersion,
				Description: "OpenBindings interface format",
			})
		} else {
			// Probe external delegate for formats
			delegateFormats, err := delegates.ProbeFormats(p.Location, delegates.DefaultProbeTimeout)
			if err == nil {
				for _, f := range delegateFormats {
					formatInfos = append(formatInfos, delegates.FormatInfo{Token: f})
				}
			}
		}

		// Build format info with preference status
		for _, f := range formatInfos {
			// Mark preferred if this delegate would be selected by preference resolution for this format.
			preferred := false
			if ws.DelegatePreferences != nil {
				if pref, ok := delegates.PreferredDelegate(ws.DelegatePreferences, f.Token); ok {
					expectedLoc := strings.TrimSpace(p.Location)
					if p.Source == delegates.SourceBuiltin {
						expectedLoc = delegates.ExecScheme + delegates.BuiltinName
					}
					preferred = strings.TrimSpace(pref) == expectedLoc
				}
			}
			entry.Formats = append(entry.Formats, DelegateFormatInfo{
				Format:      f.Token,
				Description: f.Description,
				Preferred:   preferred,
			})
		}

		entries = append(entries, entry)
	}

	return DelegateListOutput{Delegates: entries}
}
