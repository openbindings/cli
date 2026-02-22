package app

import (
	"sort"
	"strings"
	"sync"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

var (
	builtinTokens     []string
	builtinTokensOnce sync.Once
)

// RenderFormatList returns a human-friendly styled representation of a format list.
func RenderFormatList(formats []delegates.FormatInfo) string {
	s := Styles
	var sb strings.Builder

	sb.WriteString(s.Header.Render("Supported formats:"))
	sb.WriteString("\n")
	for _, f := range formats {
		sb.WriteString("  ")
		sb.WriteString(s.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(s.Key.Render(f.Token))
		if f.Description != "" {
			sb.WriteString(s.Dim.Render(" - " + f.Description))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// ListFormats returns the formats that ob can handle via built-in support or delegates.
func ListFormats() []delegates.FormatInfo {
	formats := []delegates.FormatInfo{
		{Token: "openbindings@" + openbindings.MaxTestedVersion, Description: "OpenBindings interface format"},
	}
	formats = append(formats, DefaultRegistry().AllFormats()...)

	wsCtx := GetWorkspaceDelegateContext()

	discovered, _ := delegates.Discover(delegates.DiscoverParams{
		WorkspaceDelegates: wsCtx.Delegates,
	})
	for _, p := range discovered {
		delegateFormats, err := delegates.ProbeFormats(p.Location, delegates.DefaultProbeTimeout)
		if err == nil {
			for _, f := range delegateFormats {
				formats = append(formats, delegates.FormatInfo{Token: f})
			}
		}
	}

	return uniqueSortedFormats(formats)
}

func getBuiltinTokens() []string {
	builtinTokensOnce.Do(func() {
		builtinTokens = []string{"openbindings@" + openbindings.MaxTestedVersion}
		for _, fi := range DefaultRegistry().AllFormats() {
			builtinTokens = append(builtinTokens, fi.Token)
		}
	})
	return builtinTokens
}

// BuiltinSupportsFormat checks if the builtin delegate supports a given format.
// Only checks the statically-known builtin tokens (openbindings + registered delegates).
func BuiltinSupportsFormat(format string) bool {
	for _, tok := range getBuiltinTokens() {
		if delegates.SupportsFormat(tok, format) {
			return true
		}
	}
	return false
}

func uniqueSortedFormats(in []delegates.FormatInfo) []delegates.FormatInfo {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]delegates.FormatInfo, len(in))
	for _, f := range in {
		if f.Token == "" {
			continue
		}
		if _, exists := seen[f.Token]; !exists {
			seen[f.Token] = f
		}
	}
	out := make([]delegates.FormatInfo, 0, len(seen))
	for _, f := range seen {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Token < out[j].Token
	})
	return out
}
