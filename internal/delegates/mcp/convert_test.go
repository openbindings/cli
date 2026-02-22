package mcp

import (
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openbindings/cli/internal/delegates"
)

func TestConvertToInterface_ToolsOnly(t *testing.T) {
	discovery := &Discovery{
		Tools: []*gomcp.Tool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
					"required": []any{"location"},
				},
			},
			{
				Name:         "add",
				Description:  "Add two numbers",
				InputSchema:  map[string]any{"type": "object"},
				OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"result": map[string]any{"type": "number"}}},
			},
		},
		ServerInfo: &gomcp.Implementation{Name: "test-server", Version: "1.0.0"},
	}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	if iface.Name != "test-server" {
		t.Errorf("name = %q, want %q", iface.Name, "test-server")
	}
	if iface.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", iface.Version, "1.0.0")
	}
	if len(iface.Operations) != 2 {
		t.Fatalf("operations = %d, want 2", len(iface.Operations))
	}

	// Check get_weather operation.
	op, ok := iface.Operations["get_weather"]
	if !ok {
		t.Fatal("missing operation get_weather")
	}
	if op.Kind != "method" {
		t.Errorf("get_weather kind = %q, want method", op.Kind)
	}
	if op.Description != "Get current weather" {
		t.Errorf("get_weather description = %q", op.Description)
	}
	if op.Input == nil {
		t.Error("get_weather input schema is nil")
	}

	// Check add operation has output schema.
	addOp := iface.Operations["add"]
	if addOp.Output == nil {
		t.Error("add output schema is nil")
	}

	// Check bindings.
	if len(iface.Bindings) != 2 {
		t.Fatalf("bindings = %d, want 2", len(iface.Bindings))
	}
	bind, ok := iface.Bindings["get_weather."+DefaultSourceName]
	if !ok {
		t.Fatal("missing binding for get_weather")
	}
	if bind.Ref != "tools/get_weather" {
		t.Errorf("binding ref = %q, want %q", bind.Ref, "tools/get_weather")
	}
	if bind.Source != DefaultSourceName {
		t.Errorf("binding source = %q, want %q", bind.Source, DefaultSourceName)
	}
}

func TestConvertToInterface_ResourcesOnly(t *testing.T) {
	discovery := &Discovery{
		Resources: []*gomcp.Resource{
			{
				Name:        "main.rs",
				URI:         "file:///project/src/main.rs",
				Description: "Main source file",
				MIMEType:    "text/x-rust",
			},
		},
	}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	if len(iface.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(iface.Operations))
	}

	op, ok := iface.Operations["main.rs"]
	if !ok {
		t.Fatal("missing operation main.rs")
	}
	if op.Kind != "method" {
		t.Errorf("kind = %q, want method", op.Kind)
	}
	if op.Description != "Main source file" {
		t.Errorf("description = %q", op.Description)
	}
	// Input should have a URI const.
	if op.Input == nil {
		t.Fatal("input schema is nil")
	}

	// Check binding ref uses resources/ prefix.
	bind := iface.Bindings["main.rs."+DefaultSourceName]
	if bind.Ref != "resources/file:///project/src/main.rs" {
		t.Errorf("binding ref = %q, want %q", bind.Ref, "resources/file:///project/src/main.rs")
	}
}

func TestConvertToInterface_PromptsOnly(t *testing.T) {
	discovery := &Discovery{
		Prompts: []*gomcp.Prompt{
			{
				Name:        "code_review",
				Description: "Review code quality",
				Arguments: []*gomcp.PromptArgument{
					{Name: "code", Description: "The code to review", Required: true},
					{Name: "language", Description: "Programming language"},
				},
			},
		},
	}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	if len(iface.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(iface.Operations))
	}

	op := iface.Operations["code_review"]
	if op.Kind != "method" {
		t.Errorf("kind = %q, want method", op.Kind)
	}

	// Check input schema has prompt arguments.
	if op.Input == nil {
		t.Fatal("input schema is nil")
	}
	inputMap := map[string]any(op.Input)
	props, ok := inputMap["properties"].(map[string]any)
	if !ok {
		t.Fatal("input schema missing properties")
	}
	if _, ok := props["code"]; !ok {
		t.Error("input schema missing 'code' property")
	}
	if _, ok := props["language"]; !ok {
		t.Error("input schema missing 'language' property")
	}
	// Check required.
	req, ok := inputMap["required"].([]string)
	if !ok {
		t.Fatal("input schema required is not []string")
	}
	if len(req) != 1 || req[0] != "code" {
		t.Errorf("required = %v, want [code]", req)
	}

	// Check binding ref uses prompts/ prefix.
	bind := iface.Bindings["code_review."+DefaultSourceName]
	if bind.Ref != "prompts/code_review" {
		t.Errorf("binding ref = %q, want %q", bind.Ref, "prompts/code_review")
	}
}

func TestConvertToInterface_AllEntityTypes(t *testing.T) {
	discovery := &Discovery{
		Tools: []*gomcp.Tool{
			{Name: "echo", InputSchema: map[string]any{"type": "object"}},
		},
		Resources: []*gomcp.Resource{
			{Name: "readme", URI: "file:///README.md"},
		},
		Prompts: []*gomcp.Prompt{
			{Name: "greet"},
		},
	}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	if len(iface.Operations) != 3 {
		t.Errorf("operations = %d, want 3", len(iface.Operations))
	}
	if len(iface.Bindings) != 3 {
		t.Errorf("bindings = %d, want 3", len(iface.Bindings))
	}

	// Verify all ref prefixes are correct.
	for _, b := range iface.Bindings {
		hasPrefix := strings.HasPrefix(b.Ref, RefPrefixTools) ||
			strings.HasPrefix(b.Ref, RefPrefixResources) ||
			strings.HasPrefix(b.Ref, RefPrefixPrompts)
		if !hasPrefix {
			t.Errorf("binding %q has ref %q without a valid prefix", b.Operation, b.Ref)
		}
	}
}

func TestConvertToInterface_KeyCollision(t *testing.T) {
	discovery := &Discovery{
		Tools: []*gomcp.Tool{
			{Name: "echo", InputSchema: map[string]any{"type": "object"}},
		},
		Prompts: []*gomcp.Prompt{
			{Name: "echo"},
		},
	}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	// Should have 2 operations with different keys.
	if len(iface.Operations) != 2 {
		t.Fatalf("operations = %d, want 2", len(iface.Operations))
	}

	// One should be "echo" and the other should be prefixed (e.g., "prompt_echo").
	_, hasEcho := iface.Operations["echo"]
	_, hasPromptEcho := iface.Operations["prompt_echo"]
	if !hasEcho || !hasPromptEcho {
		keys := make([]string, 0, len(iface.Operations))
		for k := range iface.Operations {
			keys = append(keys, k)
		}
		t.Errorf("expected keys [echo, prompt_echo], got %v", keys)
	}
}

func TestConvertToInterface_Empty(t *testing.T) {
	discovery := &Discovery{}

	iface, err := ConvertToInterface(discovery, "https://test-server.example.com/mcp")
	if err != nil {
		t.Fatalf("ConvertToInterface: %v", err)
	}

	if len(iface.Operations) != 0 {
		t.Errorf("operations = %d, want 0", len(iface.Operations))
	}
	if len(iface.Bindings) != 0 {
		t.Errorf("bindings = %d, want 0", len(iface.Bindings))
	}
}

func TestConvertToInterface_Nil(t *testing.T) {
	_, err := ConvertToInterface(nil, "https://test-server.example.com/mcp")
	if err == nil {
		t.Fatal("expected error for nil discovery")
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"get_weather", "get_weather"},
		{"hello-world", "hello-world"},
		{"foo.bar", "foo.bar"},
		{"foo bar", "foo_bar"},
		{"foo/bar", "foo_bar"},
		{"", "unnamed"},
		{"___", "unnamed"},
	}

	for _, tt := range tests {
		got := delegates.SanitizeKey(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
