package app

import "testing"

func TestNormalizeJSON_Nil(t *testing.T) {
	result, err := NormalizeJSON(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestNormalizeJSON_BasicTypes(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"map", map[string]any{"key": "value"}},
		{"slice", []any{1, 2, 3}},
		{"string", "hello"},
		{"float64", float64(42.5)},
		{"bool", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeJSON(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Basic types should be returned as-is (same reference)
			// We can't easily test reference equality for all types,
			// so just verify no error and non-nil result
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestNormalizeJSON_Struct(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	input := testStruct{Name: "test", Value: 42}
	result, err := NormalizeJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["name"] != "test" {
		t.Errorf("expected 'test', got %v", m["name"])
	}
	if m["value"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected 42, got %v", m["value"])
	}
}

func TestNormalizeJSON_NestedStruct(t *testing.T) {
	type inner struct {
		X int `json:"x"`
	}
	type outer struct {
		Inner inner `json:"inner"`
	}

	input := outer{Inner: inner{X: 10}}
	result, err := NormalizeJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	innerMap, ok := m["inner"].(map[string]any)
	if !ok {
		t.Fatalf("expected inner to be map[string]any, got %T", m["inner"])
	}
	if innerMap["x"] != float64(10) {
		t.Errorf("expected 10, got %v", innerMap["x"])
	}
}

func TestToStringMap_AlreadyMap(t *testing.T) {
	input := map[string]any{"key": "value"}
	result, ok := ToStringMap(input)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result["key"] != "value" {
		t.Errorf("expected 'value', got %v", result["key"])
	}
}

func TestToStringMap_Struct(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
	}

	input := testStruct{Name: "test"}
	result, ok := ToStringMap(input)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result["name"] != "test" {
		t.Errorf("expected 'test', got %v", result["name"])
	}
}

func TestToStringMap_NonConvertible(t *testing.T) {
	// A function cannot be marshaled to JSON
	input := func() {}
	_, ok := ToStringMap(input)
	if ok {
		t.Error("expected ok=false for function")
	}
}

func TestToStringMap_Slice(t *testing.T) {
	// A slice cannot be converted to map[string]any
	input := []string{"a", "b"}
	_, ok := ToStringMap(input)
	if ok {
		t.Error("expected ok=false for slice")
	}
}
