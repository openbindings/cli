// Package usage implements the usage-spec binding format handler delegate.
//
// The usage handler handles:
//   - Converting usage specs (KDL format) to OpenBindings interfaces
//   - Executing operations via CLI shell commands
package usage

import (
	"context"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"github.com/openbindings/usage-go/usage"
)

// Handler implements the usage-spec binding format handler delegate.
// It adapts the self-contained usage functions to the delegates.Handler interface.
type Handler struct{}

// New creates a new usage handler.
func New() *Handler {
	return &Handler{}
}

// GetInfo returns identity and metadata about this delegate.
func (h *Handler) GetInfo() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "Usage Spec",
		Description: "CLI command definitions in KDL format",
	}
}

// ListFormats returns the binding formats this delegate supports.
func (h *Handler) ListFormats() []delegates.FormatInfo {
	return []delegates.FormatInfo{
		{
			Token:       "usage@" + caretRange(usage.MinSupportedVersion),
			Description: "usage-spec KDL files describing CLI commands, flags, and arguments",
		},
	}
}

// CreateInterface converts a usage spec to an OpenBindings interface.
func (h *Handler) CreateInterface(source delegates.Source) (openbindings.Interface, error) {
	params := ConvertParams{
		ToFormat:  "openbindings@" + openbindings.MaxTestedVersion,
		InputPath: source.Location,
	}
	if source.Content != nil {
		if s, ok := source.Content.(string); ok {
			params.Content = s
		}
	}
	return ConvertToInterface(params)
}

// ExecuteOperation executes a CLI operation from a usage spec.
func (h *Handler) ExecuteOperation(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	localInput := ExecuteInput{
		Source: Source{
			Format:   input.Source.Format,
			Location: input.Source.Location,
			Content:  input.Source.Content,
			Binary:   input.Source.Binary,
		},
		Ref:   input.Ref,
		Input: input.Input,
	}

	result := ExecuteWithContext(ctx, localInput)

	return delegates.ExecuteOutput{
		Output:     result.Output,
		Status:     result.Status,
		DurationMs: result.DurationMs,
		Error:      result.Error,
	}
}

// Register registers the usage handler with a registry.
func Register(r *delegates.Registry) {
	r.Register(New())
}
