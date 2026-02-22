// Package delegates - constants.go centralizes delegate-related constants.
package delegates

import "time"

// Delegate source types indicate where a delegate was discovered.
const (
	// SourceBuiltin is the built-in binding format handler delegate.
	SourceBuiltin = "builtin"

	// SourceWorkspace is a delegate from the active workspace.
	SourceWorkspace = "workspace"
)

// BuiltinName is the name of the built-in binding format handler delegate.
const BuiltinName = "ob"

// URL schemes and prefixes.
const (
	// ExecScheme is the prefix for executable command references.
	ExecScheme = "exec:"

	// HTTPScheme is the HTTP URL prefix.
	HTTPScheme = "http://"

	// HTTPSScheme is the HTTPS URL prefix.
	HTTPSScheme = "https://"
)

// Well-known paths for OpenBindings discovery.
const (
	// WellKnownPath is the standard path for OpenBindings discovery.
	WellKnownPath = "/.well-known/openbindings"
)

// Standard operation names from the OpenBindings binding format handler interface.
const (
	// OpListFormats is the listFormats operation.
	OpListFormats = "listFormats"
)

// Timeouts for network and probe operations.
const (
	// DefaultProbeTimeout is the default timeout for probing delegates.
	DefaultProbeTimeout = 2 * time.Second
)
