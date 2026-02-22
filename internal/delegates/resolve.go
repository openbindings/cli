// Package delegates - resolve.go contains binding format handler delegate resolution logic.
package delegates

import (
	"fmt"
	"sort"
	"strings"
)

// Resolved contains the result of delegate resolution.
type Resolved struct {
	Format   string `json:"format"`
	Delegate string `json:"delegate"`
	Source   string `json:"source"`             // builtin|workspace
	Location string `json:"location,omitempty"` // how to reach the delegate
}

// ResolveParams configures delegate resolution.
type ResolveParams struct {
	Format string
	// DelegatePreferences is the format->delegate map from the active workspace.
	DelegatePreferences map[string]string
	// WorkspaceDelegates is the list of delegate locations from the active workspace.
	WorkspaceDelegates []string
}

// BuiltinFormatChecker is a function that checks if the builtin delegate supports a format.
type BuiltinFormatChecker func(format string) bool

// Resolve finds the delegate for a given format.
// Resolution order:
//  1. Workspace delegatePreferences (explicit mapping)
//  2. Builtin delegate if it supports the format
//  3. Workspace delegates that support the format (probed dynamically)
//
// The builtinChecker function is used to check if the builtin delegate supports a format.
// This allows the caller to inject the format checking logic without creating a circular dependency.
func Resolve(params ResolveParams, builtinChecker BuiltinFormatChecker) (Resolved, error) {
	format := strings.TrimSpace(params.Format)
	if format == "" {
		return Resolved{}, fmt.Errorf("format is required")
	}

	// 1. Check workspace delegatePreferences for explicit mapping
	if params.DelegatePreferences != nil {
		if delegate, ok := resolvePreferredDelegate(params.DelegatePreferences, format); ok {
			// Special-case: prefer exec:ob to use builtin implementation (fast path).
			if strings.EqualFold(strings.TrimSpace(delegate), ExecScheme+BuiltinName) {
				return Resolved{
					Format:   format,
					Delegate: BuiltinName,
					Source:   SourceBuiltin,
					Location: delegate,
				}, nil
			}
			return Resolved{
				Format:   format,
				Delegate: NameFromLocation(delegate),
				Source:   SourceWorkspace,
				Location: delegate,
			}, nil
		}
	}

	// 2. Check builtin delegate (ob itself)
	if builtinChecker != nil && builtinChecker(format) {
		return Resolved{
			Format:   format,
			Delegate: BuiltinName,
			Source:   SourceBuiltin,
		}, nil
	}

	// 3. Probe workspace delegates
	var matches []Info
	for _, loc := range params.WorkspaceDelegates {
		loc = strings.TrimSpace(loc)
		if loc == "" {
			continue
		}
		formats, err := ProbeFormats(loc, DefaultProbeTimeout)
		if err != nil {
			continue
		}
		for _, f := range formats {
			if SupportsFormat(f, format) {
				matches = append(matches, Info{
					Name:     NameFromLocation(loc),
					Location: loc,
					Source:   SourceWorkspace,
				})
				break
			}
		}
	}

	switch len(matches) {
	case 0:
		return Resolved{}, fmt.Errorf("no delegate supports %s", format)
	case 1:
		return Resolved{
			Format:   format,
			Delegate: matches[0].Name,
			Source:   matches[0].Source,
			Location: matches[0].Location,
		}, nil
	default:
		return Resolved{}, fmt.Errorf("multiple delegates support %s; use 'ob delegate prefer' to set a preference", format)
	}
}

// SupportsFormat checks if a delegate's format token supports a requested format.
// This handles version matching (e.g., "usage@^2.0.0" supports "usage@2.1.0").
func SupportsFormat(delegateFormat, requestedFormat string) bool {
	return supportsFormatToken(delegateFormat, requestedFormat)
}

func resolvePreferredDelegate(preferences map[string]string, requestedFormat string) (string, bool) {
	if len(preferences) == 0 {
		return "", false
	}

	// Deterministic selection:
	// - score candidates by specificity
	// - break ties lexicographically by preference key
	keys := make([]string, 0, len(preferences))
	for k := range preferences {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	bestTier := matchTierNone
	bestDelegate := ""
	for _, k := range keys {
		if !supportsFormatToken(k, requestedFormat) {
			continue
		}
		tier := matchTierFor(k, requestedFormat)
		if tier > bestTier {
			bestTier = tier
			bestDelegate = strings.TrimSpace(preferences[k])
		}
	}
	if bestTier == matchTierNone || bestDelegate == "" {
		return "", false
	}
	return bestDelegate, true
}

// PreferredDelegate returns the preferred delegate reference (URL/ref) for a requested format,
// based on workspace delegatePreferences. The selection is deterministic.
func PreferredDelegate(preferences map[string]string, requestedFormat string) (string, bool) {
	return resolvePreferredDelegate(preferences, requestedFormat)
}
