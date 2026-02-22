package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	openbindings "github.com/openbindings/openbindings-go"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/cli/internal/execref"
)

// ExecuteSource represents the binding source for execution.
type ExecuteSource struct {
	Format   string `json:"format"`
	Location string `json:"location,omitempty"`
	Content  any    `json:"content,omitempty"`
	Binary   string `json:"binary,omitempty"` // Optional: binary name hint for CLI execution
}

// ExecuteOperationInput is the input for executeOperation.
type ExecuteOperationInput struct {
	Source  ExecuteSource          `json:"source"`
	Ref     string                 `json:"ref"`
	Input   any                    `json:"input,omitempty"`
	Context *delegates.BindingContext `json:"context,omitempty"`
}

// ExecuteOperationOutput is the output of executeOperation.
type ExecuteOperationOutput struct {
	Output     any    `json:"output,omitempty"`
	Status     int    `json:"status,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Error      *Error `json:"error,omitempty"`
}

// Render returns a human-friendly representation.
func (o ExecuteOperationOutput) Render() string {
	s := Styles
	var sb strings.Builder

	if o.Error != nil {
		sb.WriteString(s.Error.Render("Error: "))
		sb.WriteString(o.Error.Message)
		return sb.String()
	}

	sb.WriteString(s.Header.Render("Execution Result"))
	sb.WriteString("\n\n")

	sb.WriteString(s.Dim.Render("Status: "))
	if o.Status == 0 {
		sb.WriteString(s.Success.Render("0 (success)"))
	} else {
		sb.WriteString(s.Error.Render(fmt.Sprintf("%d", o.Status)))
	}
	sb.WriteString("\n")

	if o.DurationMs > 0 {
		sb.WriteString(s.Dim.Render("Duration: "))
		sb.WriteString(fmt.Sprintf("%dms", o.DurationMs))
		sb.WriteString("\n")
	}

	if o.Output != nil {
		sb.WriteString(s.Dim.Render("Output: "))
		switch v := o.Output.(type) {
		case string:
			sb.WriteString(v)
		default:
			// Structured output — render as indented JSON for readability.
			if j, err := json.MarshalIndent(v, "", "  "); err == nil {
				sb.WriteString(string(j))
			} else {
				sb.WriteString(fmt.Sprintf("%v", v))
			}
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// DefaultBindingForOp finds the highest-priority binding for a given operation.
// Returns the binding key and entry, or ("", nil) if no binding matches.
//
// Priority semantics: bindings with explicit priority beat those without.
// Among bindings with explicit priority, lower values are more preferred.
// Among bindings without explicit priority, selection order is unspecified.
func DefaultBindingForOp(opKey string, iface *openbindings.Interface) (string, *openbindings.BindingEntry) {
	if iface == nil || len(iface.Bindings) == 0 {
		return "", nil
	}

	var bestKey string
	var best *openbindings.BindingEntry
	bestPri := math.MaxFloat64
	for k, b := range iface.Bindings {
		if b.Operation != opKey {
			continue
		}
		bPri := math.MaxFloat64
		if b.Priority != nil {
			bPri = *b.Priority
		}
		if best == nil || bPri < bestPri || (bPri == bestPri && k < bestKey) {
			bestKey = k
			best = &b
			bestPri = bPri
		}
	}
	return bestKey, best
}

// BindingByKey looks up a binding by its key.
// Returns nil if the key does not exist.
func BindingByKey(bindingKey string, iface *openbindings.Interface) *openbindings.BindingEntry {
	if iface == nil {
		return nil
	}
	b, ok := iface.Bindings[bindingKey]
	if !ok {
		return nil
	}
	return &b
}

// IsEventOperation returns true if the named operation in the OBI has kind "event".
// Returns false on any error (missing file, missing operation, etc.).
func IsEventOperation(obiPath string, opKey string) bool {
	iface, err := resolveInterface(obiPath)
	if err != nil {
		return false
	}
	op, ok := iface.Operations[opKey]
	if !ok {
		return false
	}
	return op.Kind == "event"
}

// IsEventBinding returns true if the binding's operation has kind "event".
// Returns false on any error (missing file, missing binding, etc.).
func IsEventBinding(obiPath string, bindingKey string) bool {
	iface, err := resolveInterface(obiPath)
	if err != nil {
		return false
	}
	b := BindingByKey(bindingKey, iface)
	if b == nil {
		return false
	}
	op, ok := iface.Operations[b.Operation]
	if !ok {
		return false
	}
	return op.Kind == "event"
}

// resolvedBinding holds the resolved components for an OBI operation execution.
type resolvedBinding struct {
	binding *openbindings.BindingEntry
	source  openbindings.Source
	input   any
	bindCtx *delegates.BindingContext
}

// resolveBindingAndSource resolves a binding, source, input transform, and
// context from an OBI interface. This shared helper eliminates duplication
// across ExecuteOBIOperation, SubscribeOBIOperation, and SubscribeOBIOperationDirect.
func resolveBindingAndSource(iface *openbindings.Interface, opKey, bindingKey string, input any, contextName string, obiDir string) (*resolvedBinding, error) {
	if opKey != "" && bindingKey != "" {
		return nil, fmt.Errorf("operation key and binding key are mutually exclusive")
	}

	var binding *openbindings.BindingEntry
	if bindingKey != "" {
		binding = BindingByKey(bindingKey, iface)
		if binding == nil {
			return nil, fmt.Errorf("binding %q not found", bindingKey)
		}
		opKey = binding.Operation
		if _, ok := iface.Operations[opKey]; !ok {
			return nil, fmt.Errorf("operation %q (referenced by binding %q) not found", opKey, bindingKey)
		}
	} else {
		if _, ok := iface.Operations[opKey]; !ok {
			return nil, fmt.Errorf("operation %q not found", opKey)
		}
		_, b := DefaultBindingForOp(opKey, iface)
		binding = b
		if binding == nil {
			return nil, fmt.Errorf("no binding for operation %q", opKey)
		}
	}

	source, ok := iface.Sources[binding.Source]
	if !ok {
		return nil, fmt.Errorf("binding source %q not found", binding.Source)
	}

	execInput := input
	if binding.InputTransform != nil {
		transformed, tErr := ApplyTransform(iface.Transforms, binding.InputTransform, input)
		if tErr != nil {
			return nil, fmt.Errorf("input transform failed: %w", tErr)
		}
		execInput = transformed
	}

	var bindCtx *delegates.BindingContext
	if contextName != "" {
		bc, cErr := GetContext(contextName)
		if cErr != nil {
			return nil, cErr
		}
		bindCtx = &bc
	}

	return &resolvedBinding{
		binding: binding,
		source:  source,
		input:   execInput,
		bindCtx: bindCtx,
	}, nil
}

// resolveSourceLocation resolves a source location relative to the OBI directory.
// exec: refs, URIs, absolute paths, and host:port addresses pass through unchanged;
// relative file paths are joined with obiDir.
func resolveSourceLocation(source openbindings.Source, obiDir string) delegates.Source {
	delSource := delegates.Source{Format: source.Format}
	if source.Location != "" {
		loc := source.Location
		if !execref.IsExec(loc) && !strings.Contains(loc, "://") && !filepath.IsAbs(loc) && !isHostPort(loc) && obiDir != "" {
			loc = filepath.Join(obiDir, loc)
		}
		delSource.Location = loc
	} else if source.Content != nil {
		delSource.Content = source.Content
	}
	return delSource
}

// isHostPort returns true if s looks like a host:port network address.
func isHostPort(s string) bool {
	_, _, err := net.SplitHostPort(s)
	return err == nil
}

// ExecuteOBIOperation executes an operation from an OBI file, applying
// input/output transforms as declared in the binding entry.
//
// Exactly one of opKey or bindingKey must be non-empty:
//   - opKey: selects the highest-priority binding for that operation.
//   - bindingKey: looks up the binding directly (operation is read from the entry).
func ExecuteOBIOperation(ctx context.Context, obiPath string, opKey string, bindingKey string, input any, contextName string) ExecuteOperationOutput {
	iface, err := resolveInterface(obiPath)
	if err != nil {
		return ExecuteOperationOutput{
			Error: &Error{Code: "load_error", Message: fmt.Sprintf("failed to load OBI %q: %v", obiPath, err)},
		}
	}

	resolved, err := resolveBindingAndSource(iface, opKey, bindingKey, input, contextName, filepath.Dir(obiPath))
	if err != nil {
		return ExecuteOperationOutput{Error: &Error{Code: "resolution_error", Message: err.Error()}}
	}

	delSource := resolveSourceLocation(resolved.source, filepath.Dir(obiPath))

	lowLevel := ExecuteOperationInput{
		Source:  ExecuteSource{Format: delSource.Format, Location: delSource.Location, Content: delSource.Content},
		Ref:     resolved.binding.Ref,
		Input:   resolved.input,
		Context: resolved.bindCtx,
	}

	result := ExecuteOperationWithContext(ctx, lowLevel)

	if resolved.binding.OutputTransform != nil && result.Error == nil {
		transformed, tErr := ApplyTransform(iface.Transforms, resolved.binding.OutputTransform, result.Output)
		if tErr != nil {
			result.Error = &Error{Code: "output_transform_error", Message: fmt.Sprintf("output transform failed: %v", tErr)}
		} else {
			result.Output = transformed
		}
	}

	return result
}

// StreamEvent is an app-layer alias for delegates.StreamEvent.
type StreamEvent = delegates.StreamEvent

// SubscribeOBIOperation opens a streaming subscription for an event-kind
// operation from an OBI file. The channel is closed when the context is
// cancelled or the stream ends.
func SubscribeOBIOperation(ctx context.Context, obiPath string, opKey string, bindingKey string, input any, contextName string) (<-chan StreamEvent, error) {
	iface, err := resolveInterface(obiPath)
	if err != nil {
		return nil, fmt.Errorf("load OBI %q: %w", obiPath, err)
	}

	resolved, err := resolveBindingAndSource(iface, opKey, bindingKey, input, contextName, filepath.Dir(obiPath))
	if err != nil {
		return nil, err
	}

	delSource := resolveSourceLocation(resolved.source, filepath.Dir(obiPath))

	handler, err := DefaultRegistry().ForFormat(resolved.source.Format)
	if err != nil {
		return nil, fmt.Errorf("no handler for format %q: %w", resolved.source.Format, err)
	}
	sh, ok := handler.(delegates.StreamHandler)
	if !ok {
		return nil, fmt.Errorf("delegate for format %q does not support streaming", resolved.source.Format)
	}

	return sh.SubscribeOperation(ctx, delegates.ExecuteInput{
		Source:  delSource,
		Ref:     resolved.binding.Ref,
		Input:   resolved.input,
		Context: resolved.bindCtx,
	})
}

// SubscribeOBIOperationDirect opens a streaming subscription using
// pre-resolved binding components. Used by the TUI which already has the
// interface, binding, and source loaded.
func SubscribeOBIOperationDirect(ctx context.Context, iface *openbindings.Interface, opKey string, binding *openbindings.BindingEntry, source openbindings.Source, obiDir string, contextName string) (<-chan StreamEvent, error) {
	resolved, err := resolveBindingAndSource(iface, opKey, "", nil, contextName, obiDir)
	if err != nil {
		return nil, err
	}

	delSource := resolveSourceLocation(source, obiDir)

	handler, err := DefaultRegistry().ForFormat(source.Format)
	if err != nil {
		return nil, fmt.Errorf("no handler for format %q: %w", source.Format, err)
	}
	sh, ok := handler.(delegates.StreamHandler)
	if !ok {
		return nil, fmt.Errorf("delegate for format %q does not support streaming", source.Format)
	}

	return sh.SubscribeOperation(ctx, delegates.ExecuteInput{
		Source:  delSource,
		Ref:     binding.Ref,
		Context: resolved.bindCtx,
	})
}

// ExecuteOperation executes an operation via a binding.
// This implements executeOperation.
//
// The execution is delegated to the resolved delegate for the format:
//   - If builtin (ob), uses the internal handler registry
//   - If external, invokes the delegate executable or service
//
// This dogfooding ensures the delegate interface is exercised uniformly.
func ExecuteOperation(input ExecuteOperationInput) ExecuteOperationOutput {
	return ExecuteOperationWithContext(context.Background(), input)
}

// ExecuteOperationWithContext executes an operation with cancellation support.
// Pass a cancellable context to allow aborting long-running operations.
func ExecuteOperationWithContext(ctx context.Context, input ExecuteOperationInput) ExecuteOperationOutput {
	start := time.Now()

	// Validate input
	if input.Source.Format == "" {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "invalid_input",
				Message: "source.format is required",
			},
		}
	}
	if input.Ref == "" {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "invalid_input",
				Message: "ref is required",
			},
		}
	}

	// Load workspace for delegate preferences
	wsCtx := GetWorkspaceDelegateContext()

	// Resolve which delegate handles this format
	resolved, err := delegates.Resolve(delegates.ResolveParams{
		Format:              input.Source.Format,
		DelegatePreferences: wsCtx.DelegatePreferences,
		WorkspaceDelegates:  wsCtx.Delegates,
	}, BuiltinSupportsFormat)
	if err != nil {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "delegate_resolution_failed",
				Message: err.Error(),
			},
		}
	}

	// Dispatch to the appropriate delegate
	var output ExecuteOperationOutput
	switch resolved.Source {
	case delegates.SourceBuiltin:
		output = executeViaBuiltin(ctx, input)
	default:
		output = executeViaExternalDelegate(ctx, resolved, input)
	}

	output.DurationMs = time.Since(start).Milliseconds()
	return output
}

// executeViaBuiltin executes an operation using a registered builtin handler.
// The handler registry already knows which formats each handler handles, so we
// simply look up by format and delegate — no manual switch needed.
func executeViaBuiltin(ctx context.Context, input ExecuteOperationInput) ExecuteOperationOutput {
	handler, err := DefaultRegistry().ForFormat(input.Source.Format)
	if err != nil {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "unsupported_format",
				Message: fmt.Sprintf("no builtin handler for format %q", input.Source.Format),
			},
		}
	}

	result := handler.ExecuteOperation(ctx, delegates.ExecuteInput{
		Source: delegates.Source{
			Format:   input.Source.Format,
			Location: input.Source.Location,
			Content:  input.Source.Content,
			Binary:   input.Source.Binary,
		},
		Ref:     input.Ref,
		Input:   input.Input,
		Context: input.Context,
	})

	return ExecuteOperationOutput{
		Output:     result.Output,
		Status:     result.Status,
		DurationMs: result.DurationMs,
		Error:      result.Error,
	}
}

// executeViaExternalDelegate executes an operation via an external delegate.
func executeViaExternalDelegate(ctx context.Context, resolved delegates.Resolved, input ExecuteOperationInput) ExecuteOperationOutput {
	loc := resolved.Location
	if loc == "" {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "invalid_delegate",
				Message: fmt.Sprintf("delegate %q has no location", resolved.Delegate),
			},
		}
	}

	return executeViaCLIDelegate(ctx, loc, input)
}

// executeViaCLIDelegate executes an operation via a CLI-based delegate.
func executeViaCLIDelegate(ctx context.Context, delegatePath string, input ExecuteOperationInput) ExecuteOperationOutput {
	// Build command: <delegate> execute --as-delegate
	// The delegate reads ExecuteOperationInput from stdin and writes ExecuteOperationOutput to stdout
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "json_marshal_error",
				Message: fmt.Sprintf("failed to marshal input: %v", err),
			},
		}
	}

	cmd := exec.CommandContext(ctx, delegatePath, "execute", "--as-delegate")
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Check for context cancellation
	if ctx.Err() != nil {
		return ExecuteOperationOutput{
			Error: &Error{
				Code:    "cancelled",
				Message: "operation cancelled",
			},
		}
	}

	// Parse output as ExecuteOperationOutput
	var output ExecuteOperationOutput
	if stdout.Len() > 0 {
		if jsonErr := json.Unmarshal(stdout.Bytes(), &output); jsonErr != nil {
			// If we can't parse JSON, wrap raw output
			output.Output = stdout.String()
		}
	}

	// Handle execution errors
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output.Status = exitErr.ExitCode()
		} else {
			output.Status = 1
		}
		if output.Error == nil && stderr.Len() > 0 {
			output.Error = &Error{
				Code:    "execution_failed",
				Message: strings.TrimSpace(stderr.String()),
			}
		}
	}

	return output
}
