// Package usage - formats.go contains format token helpers.
package usage

import "strings"

// caretRange converts a min version like "2.13.1" to a caret range like "^2.0.0".
func caretRange(minVersion string) string {
	parts := strings.Split(strings.TrimSpace(minVersion), ".")
	if len(parts) < 1 || parts[0] == "" {
		return minVersion
	}
	return "^" + parts[0] + ".0.0"
}
