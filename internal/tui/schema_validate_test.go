package tui

import (
	"os"
	"path/filepath"
	"testing"

	openbindings "github.com/openbindings/openbindings-go"
)

func TestValidateInputData_NoSchema(t *testing.T) {
	op := openbindings.Operation{
		Kind: "method",
		// No Input schema
	}

	result := ValidateInputData(map[string]any{"foo": "bar"}, op, nil)

	if result.Status != ValidationUnknown {
		t.Errorf("expected ValidationUnknown, got %s", result.Status)
	}
}

func TestValidateInputData_ValidInput(t *testing.T) {
	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "integer"},
			},
			"required": []any{"name"},
		},
	}

	result := ValidateInputData(map[string]any{"name": "Alice", "age": 30}, op, nil)

	if result.Status != ValidationValid {
		t.Errorf("expected ValidationValid, got %s: %s", result.Status, result.Message)
	}
}

func TestValidateInputData_MissingRequired(t *testing.T) {
	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}

	result := ValidateInputData(map[string]any{"other": "value"}, op, nil)

	if result.Status != ValidationInvalid {
		t.Errorf("expected ValidationInvalid, got %s", result.Status)
	}
	if result.Message == "" {
		t.Error("expected validation error message")
	}
}

func TestValidateInputData_WrongType(t *testing.T) {
	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
		},
	}

	result := ValidateInputData(map[string]any{"count": "not a number"}, op, nil)

	if result.Status != ValidationInvalid {
		t.Errorf("expected ValidationInvalid, got %s", result.Status)
	}
}

func TestValidateInputFile_FileNotFound(t *testing.T) {
	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
		},
	}

	result := ValidateInputFile("/nonexistent/path.json", op, nil)

	if result.Status != ValidationError {
		t.Errorf("expected ValidationError for missing file, got %s", result.Status)
	}
}

func TestValidateInputFile_InvalidJSON(t *testing.T) {
	// Create temp file with invalid JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
		},
	}

	result := ValidateInputFile(path, op, nil)

	if result.Status != ValidationError {
		t.Errorf("expected ValidationError for invalid JSON, got %s", result.Status)
	}
}

func TestValidateInputFile_ValidFile(t *testing.T) {
	// Create temp file with valid JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.json")
	if err := os.WriteFile(path, []byte(`{"name": "test"}`), 0644); err != nil {
		t.Fatal(err)
	}

	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}

	result := ValidateInputFile(path, op, nil)

	if result.Status != ValidationValid {
		t.Errorf("expected ValidationValid, got %s: %s", result.Status, result.Message)
	}
}

func TestValidateInputData_WithSchemaRef(t *testing.T) {
	iface := &openbindings.Interface{
		Schemas: map[string]openbindings.JSONSchema{
			"Person": {
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
		},
	}

	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"$ref": "#/schemas/Person",
		},
	}

	// Valid input
	result := ValidateInputData(map[string]any{"name": "Alice"}, op, iface)
	if result.Status != ValidationValid {
		t.Errorf("expected ValidationValid, got %s: %s", result.Status, result.Message)
	}

	// Invalid input (missing required field)
	result = ValidateInputData(map[string]any{}, op, iface)
	if result.Status != ValidationInvalid {
		t.Errorf("expected ValidationInvalid, got %s", result.Status)
	}
}

func TestValidateInputData_WithNestedSchemaRef(t *testing.T) {
	// Test nested $ref like ExecuteOperationInput -> ExecuteSource
	iface := &openbindings.Interface{
		Schemas: map[string]openbindings.JSONSchema{
			"Address": {
				"type": "object",
				"properties": map[string]any{
					"street": map[string]any{"type": "string"},
					"city":   map[string]any{"type": "string"},
				},
				"required": []any{"city"},
			},
			"Person": {
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"address": map[string]any{
						"$ref": "#/schemas/Address",
					},
				},
				"required": []any{"name", "address"},
			},
		},
	}

	op := openbindings.Operation{
		Kind: "method",
		Input: map[string]any{
			"$ref": "#/schemas/Person",
		},
	}

	// Valid input with nested object
	result := ValidateInputData(map[string]any{
		"name": "Alice",
		"address": map[string]any{
			"city": "NYC",
		},
	}, op, iface)
	if result.Status != ValidationValid {
		t.Errorf("expected ValidationValid, got %s: %s", result.Status, result.Message)
	}

	// Invalid: missing nested required field
	result = ValidateInputData(map[string]any{
		"name": "Alice",
		"address": map[string]any{
			"street": "123 Main St",
			// missing "city"
		},
	}, op, iface)
	if result.Status != ValidationInvalid {
		t.Errorf("expected ValidationInvalid for missing nested field, got %s", result.Status)
	}

	// Invalid: missing address entirely
	result = ValidateInputData(map[string]any{
		"name": "Alice",
	}, op, iface)
	if result.Status != ValidationInvalid {
		t.Errorf("expected ValidationInvalid for missing address, got %s", result.Status)
	}
}
