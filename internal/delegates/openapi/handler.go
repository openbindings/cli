// Package openapi implements the OpenAPI binding format handler delegate.
//
// The openapi handler handles:
//   - Converting OpenAPI 3.x documents to OpenBindings interfaces
//   - Executing operations via HTTP requests
package openapi

import (
	"context"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

// Handler implements the OpenAPI binding format handler delegate.
type Handler struct{}

// New creates a new OpenAPI handler.
func New() *Handler {
	return &Handler{}
}

// GetInfo returns identity and metadata about this delegate.
func (h *Handler) GetInfo() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "OpenAPI",
		Description: "REST APIs described by OpenAPI 3.x specifications",
	}
}

// ListFormats returns the binding formats this delegate supports.
func (h *Handler) ListFormats() []delegates.FormatInfo {
	return []delegates.FormatInfo{
		{
			Token:       FormatToken,
			Description: "OpenAPI 3.x specifications (JSON or YAML)",
		},
	}
}

// CreateInterface converts an OpenAPI document to an OpenBindings interface.
func (h *Handler) CreateInterface(source delegates.Source) (openbindings.Interface, error) {
	return ConvertToInterface(source)
}

// ExecuteOperation executes an HTTP request based on an OpenAPI binding.
func (h *Handler) ExecuteOperation(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	return Execute(ctx, input)
}

// Register registers the OpenAPI handler with a registry.
func Register(r *delegates.Registry) {
	r.Register(New())
}

