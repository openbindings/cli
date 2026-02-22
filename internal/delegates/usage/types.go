// Package usage - types.go defines self-contained types for the usage handler.
//
// These types mirror the openbindings.binding-format-handler interface schemas but are owned
// by this package for independence. When extracted to a separate repo, these
// types come with it.
package usage

import "github.com/openbindings/cli/internal/delegates"

// Source represents a binding source for the usage handler.
type Source struct {
	Format   string // Format token (e.g., "usage@2.0.0")
	Location string // File path or URL to the usage spec
	Content  any    // Inline content (alternative to Location)
	Binary   string // Binary name hint for CLI execution
}

// ExecuteInput is the input for operation execution.
type ExecuteInput struct {
	Source Source // Binding source
	Ref    string // Operation reference (command path, e.g., "config set")
	Input  any    // Operation input data
}

// ExecuteOutput is the output from operation execution.
type ExecuteOutput struct {
	Output     any    // Execution result
	Status     int    // Exit code
	DurationMs int64  // Execution duration in milliseconds
	Error      *Error // Error if failed
}

// Error is the shared structured error type.
type Error = delegates.Error
