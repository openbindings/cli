package app

import (
	"testing"
)

func TestWorkspaceSchema(t *testing.T) {
	schema, err := WorkspaceSchema()
	if err != nil {
		t.Fatalf("failed to load workspace schema: %v", err)
	}
	if schema == nil {
		t.Fatal("workspace schema is nil")
	}
}

func TestValidateWorkspace_Valid(t *testing.T) {
	workspace := map[string]any{
		"version": "0.1.0",
		"name":         "test-workspace",
		"targets": []any{
			map[string]any{"id": "abc123", "url": "exec:my-cli"},
		},
	}

	err := ValidateWorkspace(workspace)
	if err != nil {
		t.Errorf("expected valid workspace, got error: %v", err)
	}
}

func TestValidateWorkspace_MissingName(t *testing.T) {
	workspace := map[string]any{
		"version": "0.1.0",
		"targets": []any{
			map[string]any{"id": "abc123", "url": "exec:my-cli"},
		},
	}

	err := ValidateWorkspace(workspace)
	if err == nil {
		t.Error("expected validation error for missing name, got nil")
	}
}

func TestValidateWorkspace_MissingVersion(t *testing.T) {
	workspace := map[string]any{
		"name": "test-workspace",
	}

	err := ValidateWorkspace(workspace)
	if err == nil {
		t.Error("expected validation error for missing version, got nil")
	}
}

func TestValidateWorkspace_InvalidVersion(t *testing.T) {
	workspace := map[string]any{
		"version": "1.0", // missing patch
		"name":         "test",
	}

	err := ValidateWorkspace(workspace)
	if err == nil {
		t.Error("expected validation error for invalid version format, got nil")
	}
}

func TestValidateWorkspace_InvalidTarget(t *testing.T) {
	workspace := map[string]any{
		"version": "0.1.0",
		"name":         "test",
		"targets": []any{
			map[string]any{"id": "abc123", "label": "no url"}, // missing required 'url'
		},
	}

	err := ValidateWorkspace(workspace)
	if err == nil {
		t.Error("expected validation error for invalid target, got nil")
	}
}

func TestValidateWorkspace_WithBase(t *testing.T) {
	workspace := map[string]any{
		"version": "0.1.0",
		"name":         "child-workspace",
		"base":         "parent-workspace",
		"settings": map[string]any{
			"editor": "vim",
		},
	}

	err := ValidateWorkspace(workspace)
	if err != nil {
		t.Errorf("expected valid workspace with base, got error: %v", err)
	}
}

func TestValidateWorkspace_WithInputs(t *testing.T) {
	workspace := map[string]any{
		"version": "0.1.0",
		"name":         "test",
		"inputs": map[string]any{
			"target-abc123": map[string]any{
				"createUser": map[string]any{
					"happy-path": "./test-data/user.json",
				},
			},
		},
	}

	err := ValidateWorkspace(workspace)
	if err != nil {
		t.Errorf("expected valid workspace with inputs, got error: %v", err)
	}
}

func TestLoadWorkspace_Struct(t *testing.T) {
	// Test that the Workspace struct can be validated
	ws := Workspace{
		Version: "0.1.0",
		Name:         "test-ws",
		Targets: []WorkspaceTarget{
			{ID: "abc123", URL: "exec:my-cli", Label: "My CLI"},
		},
		Settings: WorkspaceSettings{
			Editor:       "code",
			OutputFormat: "json",
		},
		Delegates: []string{"exec:my-cli"},
		DelegatePreferences: map[string]string{
			"usage@2.0.0": "exec:ob",
		},
	}

	err := ValidateWorkspace(ws)
	if err != nil {
		t.Errorf("expected valid workspace struct, got error: %v", err)
	}
}
