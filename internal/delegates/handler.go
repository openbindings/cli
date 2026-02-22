// Package delegates defines the Handler interface and registration for binding format handler delegates.
//
// A binding format handler delegate handles a specific binding format (e.g., usage-spec, OpenAPI, AsyncAPI, MCP).
// Each handler is responsible for:
//   - Reporting its identity and metadata (getInfo)
//   - Listing the formats it supports (listFormats)
//   - Converting those formats to OpenBindings interfaces (createInterface)
//   - Executing operations via the appropriate protocol (executeOperation)
//
// The Handler interface maps 1:1 to the operations defined in the
// openbindings.binding-format-handler OBI (spec/interfaces/openbindings.binding-format-handler.json).
package delegates

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/openbindings/openbindings-go"
)

// SoftwareInfo contains identity and metadata for a piece of software.
// Maps to the SoftwareInfo schema in the software OBI.
type SoftwareInfo struct {
	Name        string `json:"name"`                  // Human-readable display name
	Version     string `json:"version,omitempty"`      // Software implementation version
	Description string `json:"description,omitempty"`  // Brief description of what this software does
	Homepage    string `json:"homepage,omitempty"`      // URL to documentation or project page
	Repository  string `json:"repository,omitempty"`    // URL to source code repository
	Maintainer  string `json:"maintainer,omitempty"`    // Author or maintaining organization
}

// FormatInfo describes a supported binding format.
// Maps to the FormatInfo schema in the binding-format-handler OBI.
type FormatInfo struct {
	Token       string `json:"token"`                  // Format token (e.g., "usage@^2.0.0")
	Description string `json:"description,omitempty"`  // Human-readable description
}

// BasicCredentials holds username/password credentials.
type BasicCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Credentials holds well-known credential fields.
type Credentials struct {
	BearerToken string            `json:"bearerToken,omitempty"`
	APIKey      string            `json:"apiKey,omitempty"`
	Basic       *BasicCredentials `json:"basic,omitempty"`
	Custom      map[string]any    `json:"custom,omitempty"`
}

// BindingContext holds runtime context for a binding execution.
type BindingContext struct {
	Credentials *Credentials      `json:"credentials,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     map[string]string `json:"cookies,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// Handler defines the interface for a binding format handler delegate.
// Each method corresponds to an operation in the openbindings.binding-format-handler OBI.
type Handler interface {
	// GetInfo returns identity and metadata about this delegate.
	// Maps to the getInfo operation.
	GetInfo() SoftwareInfo

	// ListFormats returns the binding formats this delegate supports.
	// Maps to the listFormats operation.
	ListFormats() []FormatInfo

	// CreateInterface converts a source to an OpenBindings interface.
	// Maps to the createInterface operation.
	CreateInterface(source Source) (openbindings.Interface, error)

	// ExecuteOperation executes an operation and returns the result.
	// Maps to the executeOperation operation.
	ExecuteOperation(ctx context.Context, input ExecuteInput) ExecuteOutput
}

// SourceDiscoverer is an optional interface that delegates may implement
// when their source content cannot be read as a static file (e.g., MCP
// servers that require a JSON-RPC handshake for discovery).
//
// When a delegate implements SourceDiscoverer, sync uses it instead of
// the default ReadAndHashSource + DeriveFromSource path. This avoids
// hanging on exec: sources that expect protocol-level interaction.
type SourceDiscoverer interface {
	// DiscoverSource connects to the source, discovers its content, and
	// returns both the raw content bytes (for hashing/drift detection) and
	// the derived interface (for merge). A single call avoids connecting twice.
	DiscoverSource(ctx context.Context, source Source) (content []byte, iface openbindings.Interface, err error)
}

// StreamEvent represents a single event received from a streaming subscription.
type StreamEvent struct {
	Data  any    // Parsed event payload
	Error *Error // Non-nil if the stream encountered an error
}

// StreamHandler is an optional interface that delegates may implement to
// support streaming event subscriptions. When an operation's kind is "event",
// callers should type-assert the handler to StreamHandler and use
// SubscribeOperation instead of ExecuteOperation.
//
// The returned channel receives events until the context is cancelled or
// the underlying connection closes, at which point the channel is closed.
type StreamHandler interface {
	SubscribeOperation(ctx context.Context, input ExecuteInput) (<-chan StreamEvent, error)
}

// Source represents a binding source for conversion.
type Source struct {
	Format   string // Format token (e.g., "usage@2.0.0")
	Location string // File path or URL
	Content  any    // Inline content (alternative to Location)
	Binary   string // Binary name hint for CLI execution
}

// ExecuteInput is the input for operation execution.
type ExecuteInput struct {
	Source  Source          // Binding source
	Ref     string         // Operation reference within the source
	Input   any            // Operation input data
	Context *BindingContext // Runtime context (credentials, headers, etc.)
}

// ExecuteOutput is the output from operation execution.
type ExecuteOutput struct {
	Output     any    // Execution result
	Status     int    // 0 for success, 1 for pre-request error, HTTP status code for HTTP errors
	DurationMs int64  // Execution duration in milliseconds
	Error      *Error // Non-nil when Status != 0
}

// Error represents an execution error.
type Error struct {
	Code    string `json:"code"`            // Machine-readable error code
	Message string `json:"message"`         // Human-readable message
	Details any    `json:"details,omitempty"` // Additional context
}

// Registry holds registered binding format handler delegates.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler to the registry, keyed by the name portion
// of its first format token (e.g., "mcp" from "mcp@2025-11-25").
func (r *Registry) Register(h Handler) {
	formats := h.ListFormats()
	if len(formats) == 0 {
		return
	}
	name, _ := parseFormatToken(formats[0].Token)
	r.handlers[name] = h
}

// Get returns a handler by name.
func (r *Registry) Get(name string) (Handler, bool) {
	h, ok := r.handlers[name]
	return h, ok
}

// ForFormat finds a handler that supports the given format.
// Uses semver-aware matching: a handler registered with "openapi@^3.0.0"
// will match "openapi@3.1.0" but not "openapi@2.0.0".
func (r *Registry) ForFormat(format string) (Handler, error) {
	formatName, _ := parseFormatToken(format)
	if strings.TrimSpace(formatName) == "" {
		return nil, fmt.Errorf("invalid format %q", format)
	}

	h, ok := r.handlers[formatName]
	if !ok {
		return nil, fmt.Errorf("no delegate for format %q", format)
	}

	// Verify the handler's declared format token actually supports the requested version.
	for _, fi := range h.ListFormats() {
		if supportsFormatToken(fi.Token, format) {
			return h, nil
		}
	}

	return nil, fmt.Errorf("delegate %q does not support format %q", formatName, format)
}

// All returns all registered binding format handler delegates in deterministic order.
func (r *Registry) All() []Handler {
	keys := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]Handler, 0, len(keys))
	for _, k := range keys {
		result = append(result, r.handlers[k])
	}
	return result
}

// AllFormats returns all format info from all registered delegates in deterministic order.
func (r *Registry) AllFormats() []FormatInfo {
	var result []FormatInfo
	for _, h := range r.All() {
		result = append(result, h.ListFormats()...)
	}
	return result
}
