package app

import (
	"testing"

	openbindings "github.com/openbindings/openbindings-go"
)

func TestSlugifyForPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"listPets", "listpets"},
		{"list pets", "list-pets"},
		{"list/pets", "list-pets"},
		{"list:pets", "list-pets"},
		{"list.pets", "list-pets"},
		{"  spacey  ", "spacey"},
		{"multi--dash", "multi-dash"},
		{"", "op"},
		{"a", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugifyForPath(tt.input)
			if got != tt.want {
				t.Errorf("slugifyForPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugifyForPath_LongInput(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	got := slugifyForPath(long)
	if len(got) != 64 {
		t.Errorf("expected truncation to 64, got %d", len(got))
	}
}

func TestDefaultInputPath(t *testing.T) {
	got := defaultInputPath("target1", "listPets", "default")
	want := "inputs/target1/listpets/default.json"
	if got != want {
		t.Errorf("defaultInputPath() = %q, want %q", got, want)
	}
}

func TestEnsureInputs(t *testing.T) {
	got := ensureInputs(nil)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d entries", len(got))
	}

	// Ensure existing map is preserved
	existing := map[string]map[string]map[string]string{
		"t1": {"op": {"name": "ref"}},
	}
	got = ensureInputs(existing)
	if len(got) != 1 {
		t.Error("should preserve existing map")
	}
}

func TestEnsureOpInputs(t *testing.T) {
	got := ensureOpInputs(nil)
	if got == nil {
		t.Fatal("expected non-nil")
	}
}

func TestEnsureNameInputs(t *testing.T) {
	got := ensureNameInputs(nil)
	if got == nil {
		t.Fatal("expected non-nil")
	}
}

func TestLookupInput(t *testing.T) {
	ws := &Workspace{
		Inputs: map[string]map[string]map[string]string{
			"target1": {
				"listPets": {
					"default": "/path/to/input.json",
				},
			},
		},
	}

	t.Run("found", func(t *testing.T) {
		ref, ok := lookupInput(ws, "target1", "listPets", "default")
		if !ok {
			t.Error("expected to find input")
		}
		if ref != "/path/to/input.json" {
			t.Errorf("ref = %q", ref)
		}
	})

	t.Run("missing target", func(t *testing.T) {
		_, ok := lookupInput(ws, "target2", "listPets", "default")
		if ok {
			t.Error("expected not found")
		}
	})

	t.Run("missing op", func(t *testing.T) {
		_, ok := lookupInput(ws, "target1", "createPet", "default")
		if ok {
			t.Error("expected not found")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		_, ok := lookupInput(ws, "target1", "listPets", "other")
		if ok {
			t.Error("expected not found")
		}
	})

	t.Run("nil workspace", func(t *testing.T) {
		_, ok := lookupInput(nil, "target1", "listPets", "default")
		if ok {
			t.Error("expected not found for nil workspace")
		}
	})
}

func TestCountInputRefOccurrences(t *testing.T) {
	ws := &Workspace{
		Inputs: map[string]map[string]map[string]string{
			"t1": {
				"op1": {"a": "/path/file.json", "b": "/other/file.json"},
				"op2": {"c": "/path/file.json"},
			},
			"t2": {
				"op3": {"d": "/path/file.json"},
			},
		},
	}

	count := countInputRefOccurrences(ws, "/path/file.json")
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	count = countInputRefOccurrences(ws, "/nonexistent")
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	count = countInputRefOccurrences(nil, "/path/file.json")
	if count != 0 {
		t.Errorf("count for nil ws = %d, want 0", count)
	}
}

func TestSortInputs(t *testing.T) {
	entries := []InputEntry{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}
	sortInputs(entries)
	if entries[0].Name != "alpha" || entries[1].Name != "bravo" || entries[2].Name != "charlie" {
		t.Errorf("sort order: %v", entries)
	}
}

func TestGenerateInputTemplate_Nil(t *testing.T) {
	result := generateInputTemplate(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestGenerateInputTemplate_Properties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":    map[string]any{"type": "string"},
			"age":     map[string]any{"type": "integer"},
			"active":  map[string]any{"type": "boolean"},
			"tags":    map[string]any{"type": "array"},
			"address": map[string]any{"type": "object"},
		},
	}
	result := generateInputTemplate(schema)
	if result["name"] != "" {
		t.Errorf("name = %v", result["name"])
	}
	if result["age"] != 0 {
		t.Errorf("age = %v", result["age"])
	}
	if result["active"] != false {
		t.Errorf("active = %v", result["active"])
	}
	if _, ok := result["tags"].([]any); !ok {
		t.Errorf("tags type = %T", result["tags"])
	}
	if _, ok := result["address"].(map[string]any); !ok {
		t.Errorf("address type = %T", result["address"])
	}
}

func TestGeneratePropertyTemplate_WithDefault(t *testing.T) {
	prop := map[string]any{
		"type":    "string",
		"default": "hello",
	}
	result := generatePropertyTemplate(prop)
	if result != "hello" {
		t.Errorf("result = %v, want %q", result, "hello")
	}
}

func TestGeneratePropertyTemplate_NestedObject(t *testing.T) {
	prop := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"street": map[string]any{"type": "string"},
			"zip":    map[string]any{"type": "integer"},
		},
	}
	result := generatePropertyTemplate(prop)
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if obj["street"] != "" {
		t.Errorf("street = %v", obj["street"])
	}
	if obj["zip"] != 0 {
		t.Errorf("zip = %v", obj["zip"])
	}
}

func TestTransformSchemaRefs(t *testing.T) {
	input := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pet": map[string]any{
				"$ref": "#/schemas/Pet",
			},
			"name": map[string]any{
				"type": "string",
			},
		},
	}
	result := transformSchemaRefs(input).(map[string]any)
	props := result["properties"].(map[string]any)
	pet := props["pet"].(map[string]any)
	if pet["$ref"] != "#/$defs/Pet" {
		t.Errorf("$ref = %v, want %q", pet["$ref"], "#/$defs/Pet")
	}
	// non-OBI refs should be unchanged
	name := props["name"].(map[string]any)
	if name["type"] != "string" {
		t.Errorf("name type = %v", name["type"])
	}
}

func TestTransformSchemaRefs_Array(t *testing.T) {
	input := []any{
		map[string]any{"$ref": "#/schemas/Foo"},
		"plain",
	}
	result := transformSchemaRefs(input).([]any)
	first := result[0].(map[string]any)
	if first["$ref"] != "#/$defs/Foo" {
		t.Errorf("$ref = %v", first["$ref"])
	}
	if result[1] != "plain" {
		t.Errorf("second = %v", result[1])
	}
}

func TestResolveSchemaFully(t *testing.T) {
	iface := &openbindings.Interface{
		Schemas: map[string]openbindings.JSONSchema{
			"Pet": {"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}},
		},
	}

	t.Run("ref resolved", func(t *testing.T) {
		schema := map[string]any{"$ref": "#/schemas/Pet"}
		result := resolveSchemaFully(schema, iface)
		if result["type"] != "object" {
			t.Errorf("type = %v", result["type"])
		}
	})

	t.Run("no ref passthrough", func(t *testing.T) {
		schema := map[string]any{"type": "string"}
		result := resolveSchemaFully(schema, iface)
		if result["type"] != "string" {
			t.Errorf("type = %v", result["type"])
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		result := resolveSchemaFully(nil, iface)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("unresolvable ref", func(t *testing.T) {
		schema := map[string]any{"$ref": "#/schemas/Unknown"}
		result := resolveSchemaFully(schema, iface)
		// Should return the original schema unchanged
		if result["$ref"] != "#/schemas/Unknown" {
			t.Errorf("expected original ref, got %v", result)
		}
	})
}

func TestBuildSchemaDocument(t *testing.T) {
	iface := &openbindings.Interface{
		Schemas: map[string]openbindings.JSONSchema{
			"Pet": {"type": "object"},
		},
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pet": map[string]any{"$ref": "#/schemas/Pet"},
		},
	}
	doc := buildSchemaDocument(schema, iface)

	// Should have $defs from schemas
	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		t.Fatal("expected $defs in document")
	}
	if _, ok := defs["Pet"]; !ok {
		t.Error("expected Pet in $defs")
	}
}

func TestValidateInputAgainstOperationSchema(t *testing.T) {
	t.Run("no schema", func(t *testing.T) {
		op := openbindings.Operation{}
		status, _ := validateInputAgainstOperationSchema(nil, op, nil)
		if status != "unknown" {
			t.Errorf("status = %q", status)
		}
	})

	t.Run("valid input", func(t *testing.T) {
		op := openbindings.Operation{
			Input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		}
		input := map[string]any{"name": "test"}
		status, _ := validateInputAgainstOperationSchema(input, op, nil)
		if status != "valid" {
			t.Errorf("status = %q", status)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		op := openbindings.Operation{
			Input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"age": map[string]any{"type": "integer"},
				},
				"required": []any{"age"},
			},
		}
		input := map[string]any{} // missing required "age"
		status, _ := validateInputAgainstOperationSchema(input, op, nil)
		if status != "invalid" {
			t.Errorf("status = %q, want %q", status, "invalid")
		}
	})
}

func TestInputsListOutput_Render(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		o := InputsListOutput{}
		r := o.Render()
		if r == "" {
			t.Error("expected non-empty render")
		}
	})

	t.Run("with entries", func(t *testing.T) {
		o := InputsListOutput{
			Inputs: []InputEntry{
				{Name: "default", Ref: "/path/to/file.json"},
			},
		}
		r := o.Render()
		if r == "" {
			t.Error("expected non-empty render")
		}
	})
}

func TestInputsMutateOutput_Render(t *testing.T) {
	actions := []string{"added", "removed", "deleted", "created", "other"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			o := InputsMutateOutput{
				Name:   "test",
				Ref:    "/path/to/file.json",
				Action: action,
			}
			r := o.Render()
			if r == "" {
				t.Error("expected non-empty render")
			}
		})
	}
}

func TestInputsValidateOutput_Render(t *testing.T) {
	statuses := []string{"valid", "invalid", "error", "unknown"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			o := InputsValidateOutput{Status: status, Message: "test"}
			r := o.Render()
			if r == "" {
				t.Error("expected non-empty render")
			}
		})
	}
}

func TestExtractValidationError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := extractValidationError(nil); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}
