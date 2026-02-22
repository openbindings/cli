// Package delegates - discovery.go contains binding format handler delegate discovery logic.
package delegates

import (
	"sort"
	"strings"
)

// DiscoverParams configures delegate discovery.
type DiscoverParams struct {
	IncludeBuiltin bool
	// WorkspaceDelegates is the list of delegate locations from the active workspace.
	WorkspaceDelegates []string
}

// Discover finds all delegates from the workspace and optionally the builtin.
// If IncludeBuiltin is true, the built-in binding format handler delegate is included first.
func Discover(params DiscoverParams) ([]Info, error) {
	var all []Info
	seen := map[string]struct{}{}

	// Include builtin delegate first (ob itself)
	if params.IncludeBuiltin {
		all = append(all, BuiltinInfo())
		seen[BuiltinName] = struct{}{}
	}

	// Add workspace delegates
	for _, loc := range params.WorkspaceDelegates {
		loc = strings.TrimSpace(loc)
		if loc == "" {
			continue
		}
		if _, ok := seen[loc]; ok {
			continue
		}
		seen[loc] = struct{}{}

		name := NameFromLocation(loc)
		all = append(all, Info{
			Name:     name,
			Location: loc,
			Source:   SourceWorkspace,
		})
	}

	sort.Slice(all, func(i, j int) bool {
		// Keep builtin first
		if all[i].Source == SourceBuiltin {
			return true
		}
		if all[j].Source == SourceBuiltin {
			return false
		}
		if all[i].Name == all[j].Name {
			return all[i].Location < all[j].Location
		}
		return all[i].Name < all[j].Name
	})
	return all, nil
}
