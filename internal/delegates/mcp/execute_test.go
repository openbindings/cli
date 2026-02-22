package mcp

import (
	"encoding/json"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openbindings/cli/internal/delegates"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref        string
		wantType   string
		wantName   string
		wantErr    bool
	}{
		// Valid refs.
		{"tools/get_weather", "tools", "get_weather", false},
		{"resources/file:///src/main.rs", "resources", "file:///src/main.rs", false},
		{"prompts/code_review", "prompts", "code_review", false},
		{"tools/my-tool", "tools", "my-tool", false},
		{"resources/https://example.com/data", "resources", "https://example.com/data", false},

		// Invalid refs.
		{"", "", "", true},
		{"   ", "", "", true},
		{"get_weather", "", "", true},
		{"unknown/thing", "", "", true},
		{"tools/", "", "", true},
		{"resources/", "", "", true},
		{"prompts/", "", "", true},
	}

	for _, tt := range tests {
		entityType, name, err := ParseRef(tt.ref)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRef(%q) = (%q, %q, nil), want error", tt.ref, entityType, name)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRef(%q) error: %v", tt.ref, err)
			continue
		}
		if entityType != tt.wantType {
			t.Errorf("ParseRef(%q) entityType = %q, want %q", tt.ref, entityType, tt.wantType)
		}
		if name != tt.wantName {
			t.Errorf("ParseRef(%q) name = %q, want %q", tt.ref, name, tt.wantName)
		}
	}
}

func TestCallToolResultToOutput_TextContent(t *testing.T) {
	result := &gomcp.CallToolResult{
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: "hello world"},
		},
	}

	output := callToolResultToOutput(result)
	if output.Status != 0 {
		t.Errorf("status = %d, want 0", output.Status)
	}
	text, ok := output.Output.(string)
	if !ok {
		t.Fatalf("output type = %T, want string", output.Output)
	}
	if text != "hello world" {
		t.Errorf("output = %q, want %q", text, "hello world")
	}
}

func TestCallToolResultToOutput_MultiContent(t *testing.T) {
	result := &gomcp.CallToolResult{
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: "part one"},
			&gomcp.TextContent{Text: "part two"},
		},
	}

	output := callToolResultToOutput(result)
	// Multi-content should produce a joined string.
	text, ok := output.Output.(string)
	if !ok {
		t.Fatalf("output type = %T, want string", output.Output)
	}
	if text != "part one\npart two" {
		t.Errorf("output = %q, want %q", text, "part one\npart two")
	}
}

func TestCallToolResultToOutput_IsError(t *testing.T) {
	result := &gomcp.CallToolResult{
		IsError: true,
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: "something went wrong"},
		},
	}

	output := callToolResultToOutput(result)
	if output.Status != 1 {
		t.Errorf("status = %d, want 1", output.Status)
	}
}

func TestCallToolResultToOutput_StructuredContent(t *testing.T) {
	result := &gomcp.CallToolResult{
		StructuredContent: map[string]any{
			"temperature": 72.5,
			"unit":        "fahrenheit",
		},
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: "fallback text"},
		},
	}

	output := callToolResultToOutput(result)
	if output.Status != 0 {
		t.Errorf("status = %d, want 0", output.Status)
	}
	// Should prefer structured content.
	m, ok := output.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type = %T, want map[string]any", output.Output)
	}
	if m["unit"] != "fahrenheit" {
		t.Errorf("unit = %v, want fahrenheit", m["unit"])
	}
}

func TestToStringAnyMap(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		wantOK bool
	}{
		{"nil", nil, false},
		{"map[string]any", map[string]any{"key": "val"}, true},
		{"json bytes", json.RawMessage(`{"key": "val"}`), false},
		{"string", "not a map", false},
		{"int", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := delegates.ToStringAnyMap(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ToStringAnyMap(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
		})
	}
}

func TestReadResourceResultToOutput(t *testing.T) {
	result := &gomcp.ReadResourceResult{
		Contents: []*gomcp.ResourceContents{
			{
				URI:      "file:///test.txt",
				MIMEType: "text/plain",
				Text:     "file content here",
			},
		},
	}

	output := readResourceResultToOutput(result)
	if output.Status != 0 {
		t.Errorf("status = %d, want 0", output.Status)
	}
	text, ok := output.Output.(string)
	if !ok {
		t.Fatalf("output type = %T, want string", output.Output)
	}
	if text != "file content here" {
		t.Errorf("output = %q, want %q", text, "file content here")
	}
}

func TestReadResourceResultToOutput_Empty(t *testing.T) {
	result := &gomcp.ReadResourceResult{
		Contents: []*gomcp.ResourceContents{},
	}
	output := readResourceResultToOutput(result)
	if output.Status != 0 {
		t.Errorf("status = %d, want 0", output.Status)
	}
	if output.Output != nil {
		t.Errorf("output = %v, want nil", output.Output)
	}
}

func TestGetPromptResultToOutput(t *testing.T) {
	result := &gomcp.GetPromptResult{
		Description: "A test prompt",
		Messages: []*gomcp.PromptMessage{
			{
				Role:    gomcp.Role("user"),
				Content: &gomcp.TextContent{Text: "Please review this code"},
			},
		},
	}

	output := getPromptResultToOutput(result)
	if output.Status != 0 {
		t.Errorf("status = %d, want 0", output.Status)
	}
	m, ok := output.Output.(map[string]any)
	if !ok {
		t.Fatalf("output type = %T, want map[string]any", output.Output)
	}
	if m["description"] != "A test prompt" {
		t.Errorf("description = %v, want %q", m["description"], "A test prompt")
	}
	messages, ok := m["messages"].([]any)
	if !ok {
		t.Fatalf("messages type = %T, want []any", m["messages"])
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	msg := messages[0].(map[string]any)
	contentMap, ok := msg["content"].(map[string]any)
	if !ok {
		t.Fatalf("content type = %T, want map[string]any", msg["content"])
	}
	if contentMap["type"] != "text" {
		t.Errorf("content type = %v, want text", contentMap["type"])
	}
	if contentMap["text"] != "Please review this code" {
		t.Errorf("content text = %v, want %q", contentMap["text"], "Please review this code")
	}
}

