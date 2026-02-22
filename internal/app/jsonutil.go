// Package app - jsonutil.go provides JSON normalization and conversion utilities.
package app

import "encoding/json"

// NormalizeJSON converts a Go value to a JSON-normalized form (map[string]any, []any, etc).
// This ensures consistent handling of structs, typed maps, and other Go values
// when working with JSON-based transformations.
//
// Returns the input unchanged if it's already a basic JSON type.
// Otherwise, round-trips through JSON marshaling to normalize.
func NormalizeJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}

	// If already a basic JSON type, return as-is
	switch v.(type) {
	case map[string]any, []any, string, float64, bool:
		return v, nil
	}

	// Round-trip through JSON to normalize
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ToStringMap attempts to convert a value to map[string]any.
// Returns the map and true if successful, nil and false otherwise.
//
// If the input is already map[string]any, returns it directly.
// Otherwise, attempts JSON round-trip conversion for struct types.
func ToStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	default:
		// Try JSON round-trip for struct types
		b, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		var result map[string]any
		if err := json.Unmarshal(b, &result); err != nil {
			return nil, false
		}
		return result, true
	}
}
