// Package grpc implements the gRPC binding format handler delegate.
//
// The gRPC handler uses server reflection to discover services and methods,
// converts protobuf descriptors to OpenBindings operations, and dynamically
// invokes RPCs using JSON input/output.
package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"github.com/openbindings/openbindings-go/canonicaljson"
)

// FormatToken is the format identifier for gRPC sources.
const FormatToken = "grpc"

// DefaultTimeout is the maximum time to wait for gRPC operations.
const DefaultTimeout = 30 * time.Second

// Handler implements the gRPC binding format handler delegate.
type Handler struct{}

// New creates a new gRPC handler.
func New() *Handler {
	return &Handler{}
}

// GetInfo returns identity and metadata about this delegate.
func (h *Handler) GetInfo() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "gRPC",
		Description: "gRPC servers discovered via server reflection",
	}
}

// ListFormats returns the binding formats this delegate supports.
func (h *Handler) ListFormats() []delegates.FormatInfo {
	return []delegates.FormatInfo{
		{
			Token:       FormatToken,
			Description: "gRPC services via server reflection (unary and server-streaming)",
		},
	}
}

// CreateInterface discovers a gRPC server's services and converts them
// to an OpenBindings interface.
func (h *Handler) CreateInterface(source delegates.Source) (openbindings.Interface, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, iface, err := h.discoverAndConvert(ctx, source)
	return iface, err
}

// ExecuteOperation executes a gRPC operation via dynamic invocation.
func (h *Handler) ExecuteOperation(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	result := Execute(ctx, ExecuteInput{
		Address: input.Source.Location,
		Ref:     input.Ref,
		Input:   input.Input,
	})

	return delegates.ExecuteOutput{
		Output:     result.Output,
		Status:     result.Status,
		DurationMs: result.DurationMs,
		Error:      result.Error,
	}
}

// SubscribeOperation implements the delegates.StreamHandler interface for
// server-streaming RPCs.
func (h *Handler) SubscribeOperation(ctx context.Context, input delegates.ExecuteInput) (<-chan delegates.StreamEvent, error) {
	return Subscribe(ctx, ExecuteInput{
		Address: input.Source.Location,
		Ref:     input.Ref,
		Input:   input.Input,
	})
}

// DiscoverSource implements the delegates.SourceDiscoverer interface.
func (h *Handler) DiscoverSource(ctx context.Context, source delegates.Source) ([]byte, openbindings.Interface, error) {
	return h.discoverAndConvert(ctx, source)
}

func (h *Handler) discoverAndConvert(ctx context.Context, source delegates.Source) ([]byte, openbindings.Interface, error) {
	addr := source.Location
	if addr == "" {
		return nil, openbindings.Interface{}, fmt.Errorf("gRPC source requires a location (host:port address)")
	}

	disc, err := Discover(ctx, addr)
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("gRPC discovery: %w", err)
	}

	iface, err := ConvertToInterface(disc, source.Location)
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("gRPC convert: %w", err)
	}

	content, err := canonicaljson.Marshal(disc.ToCanonical())
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("gRPC serialize discovery: %w", err)
	}

	return content, iface, nil
}

// Register registers the gRPC handler with a registry.
func Register(r *delegates.Registry) {
	r.Register(New())
}
