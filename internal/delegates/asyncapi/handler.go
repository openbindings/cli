package asyncapi

import (
	"context"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

// Handler implements the AsyncAPI binding format handler delegate.
type Handler struct{}

// New creates a new AsyncAPI handler.
func New() *Handler {
	return &Handler{}
}

// GetInfo returns identity and metadata about this delegate.
func (h *Handler) GetInfo() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "AsyncAPI",
		Description: "Event-driven APIs described by AsyncAPI 3.0 specifications",
	}
}

// ListFormats returns the binding formats this delegate supports.
func (h *Handler) ListFormats() []delegates.FormatInfo {
	return []delegates.FormatInfo{
		{
			Token:       FormatToken,
			Description: "AsyncAPI 3.0 specifications (JSON or YAML) for event-driven APIs",
		},
	}
}

// CreateInterface converts an AsyncAPI document to an OpenBindings interface.
func (h *Handler) CreateInterface(source delegates.Source) (openbindings.Interface, error) {
	return ConvertToInterface(source)
}

// ExecuteOperation executes an AsyncAPI operation via SSE or WebSocket.
func (h *Handler) ExecuteOperation(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	return Execute(ctx, input)
}

// SubscribeOperation opens a streaming subscription for an AsyncAPI "receive"
// operation, returning a channel that emits events until the context is
// cancelled or the connection drops.
func (h *Handler) SubscribeOperation(ctx context.Context, input delegates.ExecuteInput) (<-chan delegates.StreamEvent, error) {
	return Subscribe(ctx, input)
}

// Register registers the AsyncAPI handler with a registry.
func Register(r *delegates.Registry) {
	r.Register(New())
}

