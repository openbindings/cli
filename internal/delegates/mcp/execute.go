package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/openbindings/cli/internal/delegates"
)

// ExecuteInput is the input for MCP operation execution.
type ExecuteInput struct {
	URL   string // HTTP/HTTPS URL of the MCP server
	Ref   string // MCP ref (e.g., "tools/get_weather", "resources/file:///...", "prompts/code_review")
	Input any    // Operation input data
}

// ExecuteOutput is the output from MCP operation execution.
type ExecuteOutput struct {
	Output     any    // Execution result
	Status     int    // 0 for success, 1 for error
	DurationMs int64  // Execution duration
	Error      *delegates.Error
}

// Execute dispatches an operation to the appropriate MCP method based on the ref prefix.
func Execute(ctx context.Context, input ExecuteInput) ExecuteOutput {
	start := time.Now()

	entityType, name, err := ParseRef(input.Ref)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error: &delegates.Error{
				Code:    "invalid_ref",
				Message: err.Error(),
			},
		}
	}

	// ParseRef guarantees entityType is one of "tools", "resources", or "prompts".
	var output ExecuteOutput
	switch entityType {
	case "tools":
		output = executeTool(ctx, input.URL, name, input.Input)
	case "resources":
		output = executeResource(ctx, input.URL, name)
	case "prompts":
		output = executePrompt(ctx, input.URL, name, input.Input)
	}

	output.DurationMs = time.Since(start).Milliseconds()
	return output
}

// ParseRef extracts the entity type and name from an MCP ref.
// Returns (entityType, name, error).
// Examples:
//
//	"tools/get_weather"              → ("tools", "get_weather", nil)
//	"resources/file:///src/main.rs"  → ("resources", "file:///src/main.rs", nil)
//	"prompts/code_review"            → ("prompts", "code_review", nil)
func ParseRef(ref string) (entityType string, name string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("empty MCP ref")
	}

	for _, prefix := range []string{RefPrefixTools, RefPrefixResources, RefPrefixPrompts} {
		if strings.HasPrefix(ref, prefix) {
			name := strings.TrimPrefix(ref, prefix)
			if name == "" {
				return "", "", fmt.Errorf("empty name in MCP ref %q", ref)
			}
			// Strip trailing slash from prefix to get entity type.
			entityType := strings.TrimSuffix(prefix, "/")
			return entityType, name, nil
		}
	}

	return "", "", fmt.Errorf("MCP ref %q must start with %q, %q, or %q",
		ref, RefPrefixTools, RefPrefixResources, RefPrefixPrompts)
}

// executeTool calls a tool on the MCP server.
func executeTool(ctx context.Context, url string, toolName string, input any) ExecuteOutput {
	args, ok := delegates.ToStringAnyMap(input)
	if input != nil && !ok {
		return ExecuteOutput{
			Status: 1,
			Error: &delegates.Error{
				Code:    "invalid_input",
				Message: fmt.Sprintf("tool input must be an object, got %T", input),
			},
		}
	}
	// MCP servers expect an object for arguments, never null.
	// Default to empty map so no-arg tools work without requiring --input '{}'.
	if args == nil {
		args = map[string]any{}
	}

	result, err := CallTool(ctx, url, toolName, args)
	if err != nil {
		return ExecuteOutput{
			Status: 1,
			Error: &delegates.Error{
				Code:    "tool_call_failed",
				Message: err.Error(),
			},
		}
	}

	return callToolResultToOutput(result)
}

// executeResource reads a resource from the MCP server.
func executeResource(ctx context.Context, url string, uri string) ExecuteOutput {
	result, err := ReadResource(ctx, url, uri)
	if err != nil {
		return ExecuteOutput{
			Status: 1,
			Error: &delegates.Error{
				Code:    "resource_read_failed",
				Message: err.Error(),
			},
		}
	}

	return readResourceResultToOutput(result)
}

// executePrompt gets a prompt from the MCP server.
func executePrompt(ctx context.Context, url string, promptName string, input any) ExecuteOutput {
	args, err := toStringStringMap(input)
	if err != nil {
		return ExecuteOutput{
			Status: 1,
			Error: &delegates.Error{
				Code:    "invalid_input",
				Message: fmt.Sprintf("prompt arguments must be an object with string values: %v", err),
			},
		}
	}

	result, err := GetPrompt(ctx, url, promptName, args)
	if err != nil {
		return ExecuteOutput{
			Status: 1,
			Error: &delegates.Error{
				Code:    "prompt_get_failed",
				Message: err.Error(),
			},
		}
	}

	return getPromptResultToOutput(result)
}

// callToolResultToOutput converts an MCP CallToolResult to an ExecuteOutput.
func callToolResultToOutput(result *gomcp.CallToolResult) ExecuteOutput {
	status := 0
	if result.IsError {
		status = 1
	}

	// Prefer structured content if available.
	if result.StructuredContent != nil {
		// StructuredContent is any — it may already be a usable Go value (map, slice, etc.)
		// or it could be json.RawMessage. Try to use it directly first.
		switch sc := result.StructuredContent.(type) {
		case json.RawMessage:
			var structured any
			if json.Unmarshal(sc, &structured) == nil {
				return ExecuteOutput{Output: structured, Status: status}
			}
		default:
			return ExecuteOutput{Output: sc, Status: status}
		}
	}

	// Fall back to content array.
	output := extractContent(result.Content)
	return ExecuteOutput{
		Output: output,
		Status: status,
	}
}

// readResourceResultToOutput converts an MCP ReadResourceResult to an ExecuteOutput.
func readResourceResultToOutput(result *gomcp.ReadResourceResult) ExecuteOutput {
	if len(result.Contents) == 0 {
		return ExecuteOutput{Status: 0}
	}

	// Return the first content's text. For multi-content, build an array.
	if len(result.Contents) == 1 {
		c := result.Contents[0]
		if c.Text != "" {
			// Try to parse as JSON for structured output.
			var parsed any
			if json.Unmarshal([]byte(c.Text), &parsed) == nil {
				return ExecuteOutput{Output: parsed, Status: 0}
			}
			return ExecuteOutput{Output: c.Text, Status: 0}
		}
		return ExecuteOutput{Output: map[string]any{"uri": c.URI, "mimeType": c.MIMEType}, Status: 0}
	}

	var items []any
	for _, c := range result.Contents {
		items = append(items, map[string]any{
			"uri":      c.URI,
			"mimeType": c.MIMEType,
			"text":     c.Text,
		})
	}
	return ExecuteOutput{Output: items, Status: 0}
}

// getPromptResultToOutput converts an MCP GetPromptResult to an ExecuteOutput.
func getPromptResultToOutput(result *gomcp.GetPromptResult) ExecuteOutput {
	var messages []any
	for _, msg := range result.Messages {
		if msg == nil {
			continue
		}
		entry := map[string]any{
			"role": string(msg.Role),
		}
		if msg.Content != nil {
			// Use contentToMap for full type support (text, image, audio, resource, etc.)
			entry["content"] = contentToMap(msg.Content)
		}
		messages = append(messages, entry)
	}

	output := map[string]any{
		"messages": messages,
	}
	if result.Description != "" {
		output["description"] = result.Description
	}

	return ExecuteOutput{Output: output, Status: 0}
}

// extractContent converts MCP Content items to a usable output value.
// Text content is concatenated; non-text items are represented as structured maps.
func extractContent(content []gomcp.Content) any {
	if len(content) == 0 {
		return nil
	}

	// Fast path: single text content — return as string (with JSON parse attempt).
	if len(content) == 1 {
		if tc, ok := content[0].(*gomcp.TextContent); ok {
			var parsed any
			if json.Unmarshal([]byte(tc.Text), &parsed) == nil {
				return parsed
			}
			return tc.Text
		}
	}

	// Check if all items are text — if so, join them.
	allText := true
	for _, c := range content {
		if _, ok := c.(*gomcp.TextContent); !ok {
			allText = false
			break
		}
	}
	if allText {
		var texts []string
		for _, c := range content {
			texts = append(texts, c.(*gomcp.TextContent).Text)
		}
		return strings.Join(texts, "\n")
	}

	// Mixed content — return as structured array.
	var items []any
	for _, c := range content {
		items = append(items, contentToMap(c))
	}
	return items
}

// contentToMap converts a single MCP Content item to a structured map representation.
func contentToMap(c gomcp.Content) map[string]any {
	switch v := c.(type) {
	case *gomcp.TextContent:
		return map[string]any{"type": "text", "text": v.Text}
	case *gomcp.ImageContent:
		return map[string]any{"type": "image", "mimeType": v.MIMEType, "data": string(v.Data)}
	case *gomcp.AudioContent:
		return map[string]any{"type": "audio", "mimeType": v.MIMEType, "data": string(v.Data)}
	case *gomcp.ResourceLink:
		m := map[string]any{"type": "resource_link", "uri": v.URI}
		if v.Name != "" {
			m["name"] = v.Name
		}
		if v.MIMEType != "" {
			m["mimeType"] = v.MIMEType
		}
		return m
	case *gomcp.EmbeddedResource:
		m := map[string]any{"type": "resource"}
		if v.Resource != nil {
			m["uri"] = v.Resource.URI
			if v.Resource.MIMEType != "" {
				m["mimeType"] = v.Resource.MIMEType
			}
			if v.Resource.Text != "" {
				m["text"] = v.Resource.Text
			}
		}
		return m
	default:
		return map[string]any{"type": "unknown"}
	}
}

// toStringStringMap converts any to map[string]string for prompt arguments.
// Returns nil map for nil input.
func toStringStringMap(v any) (map[string]string, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map[string]any, got %T", v)
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		result[k] = fmt.Sprintf("%v", val)
	}
	return result, nil
}
