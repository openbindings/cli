package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// GetContext loads a named context from the store.
// Returns an empty context if the name is empty.
func GetContext(name string) (delegates.BindingContext, error) {
	if name == "" {
		return delegates.BindingContext{}, nil
	}
	ctx, err := LoadContext(name)
	if err != nil {
		return delegates.BindingContext{}, fmt.Errorf("loading context %q: %w", name, err)
	}
	return ctx, nil
}

// RenderBindingContext returns a human-friendly representation of a BindingContext.
func RenderBindingContext(ctx delegates.BindingContext) string {
	s := Styles
	var sb strings.Builder

	empty := ctx.Credentials == nil &&
		len(ctx.Headers) == 0 &&
		len(ctx.Cookies) == 0 &&
		len(ctx.Environment) == 0 &&
		len(ctx.Metadata) == 0

	if empty {
		sb.WriteString(s.Dim.Render("No context configured"))
		return sb.String()
	}

	sb.WriteString(s.Header.Render("Binding Context"))

	if ctx.Credentials != nil {
		sb.WriteString("\n\n")
		sb.WriteString(s.Dim.Render("Credentials:"))
		if ctx.Credentials.BearerToken != "" {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Dim.Render("Bearer: "))
			sb.WriteString(maskSecret(ctx.Credentials.BearerToken))
		}
		if ctx.Credentials.APIKey != "" {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Dim.Render("API Key: "))
			sb.WriteString(maskSecret(ctx.Credentials.APIKey))
		}
		if ctx.Credentials.Basic != nil {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Dim.Render("Basic: "))
			sb.WriteString(ctx.Credentials.Basic.Username + ":****")
		}
	}

	if len(ctx.Headers) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(s.Dim.Render("Headers:"))
		for _, k := range sortedKeys(ctx.Headers) {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Key.Render(k))
			sb.WriteString(s.Dim.Render(": "))
			sb.WriteString(ctx.Headers[k])
		}
	}

	if len(ctx.Cookies) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(s.Dim.Render("Cookies:"))
		for _, k := range sortedKeys(ctx.Cookies) {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Key.Render(k))
			sb.WriteString(s.Dim.Render("="))
			sb.WriteString(maskSecret(ctx.Cookies[k]))
		}
	}

	if len(ctx.Environment) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(s.Dim.Render("Environment:"))
		for _, k := range sortedKeys(ctx.Environment) {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Key.Render(k))
			sb.WriteString(s.Dim.Render("="))
			sb.WriteString(maskSecret(ctx.Environment[k]))
		}
	}

	if len(ctx.Metadata) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(s.Dim.Render("Metadata:"))
		metaKeys := make([]string, 0, len(ctx.Metadata))
		for k := range ctx.Metadata {
			metaKeys = append(metaKeys, k)
		}
		sort.Strings(metaKeys)
		for _, k := range metaKeys {
			sb.WriteString("\n  ")
			sb.WriteString(s.Bullet.Render("•"))
			sb.WriteString(" ")
			sb.WriteString(s.Key.Render(k))
			sb.WriteString(s.Dim.Render(": "))
			sb.WriteString(fmt.Sprintf("%v", ctx.Metadata[k]))
		}
	}

	return sb.String()
}

// RenderContextList returns a human-friendly list of context summaries.
func RenderContextList(summaries []ContextSummary) string {
	s := Styles
	var sb strings.Builder

	if len(summaries) == 0 {
		sb.WriteString(s.Dim.Render("No contexts configured"))
		return sb.String()
	}

	sb.WriteString(s.Header.Render("Contexts"))

	for _, cs := range summaries {
		sb.WriteString("\n  ")
		sb.WriteString(s.Key.Render(cs.Name))

		if cs.LoadError != "" {
			sb.WriteString(s.Dim.Render(" (error: " + cs.LoadError + ")"))
			continue
		}

		var parts []string
		if cs.HasCredentials {
			parts = append(parts, "credentials")
		}
		if cs.HeaderCount > 0 {
			parts = append(parts, fmt.Sprintf("%d headers", cs.HeaderCount))
		}
		if cs.CookieCount > 0 {
			parts = append(parts, fmt.Sprintf("%d cookies", cs.CookieCount))
		}
		if cs.EnvCount > 0 {
			parts = append(parts, fmt.Sprintf("%d env", cs.EnvCount))
		}
		if cs.MetadataCount > 0 {
			parts = append(parts, fmt.Sprintf("%d metadata", cs.MetadataCount))
		}
		if len(parts) > 0 {
			sb.WriteString(s.Dim.Render(" (" + strings.Join(parts, ", ") + ")"))
		}
	}

	return sb.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
