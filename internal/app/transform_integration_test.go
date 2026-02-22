package app

import (
	"encoding/json"
	"os/exec"
	"testing"

	openbindings "github.com/openbindings/openbindings-go"
)

func TestTransformIntegration_CreateInterface(t *testing.T) {
	out, err := exec.Command("ob", "--openbindings").Output()
	if err != nil {
		t.Fatalf("Error running ob: %v", err)
	}

	var iface openbindings.Interface
	if err := json.Unmarshal(out, &iface); err != nil {
		t.Fatalf("Error parsing: %v", err)
	}

	// Find binding
	opKey := "createInterface"
	var binding *openbindings.BindingEntry
	for _, b := range iface.Bindings {
		if b.Operation == opKey {
			binding = &b
			break
		}
	}
	if binding == nil {
		t.Fatalf("No binding found for %s", opKey)
	}

	t.Logf("Binding found: %s", binding.Operation)
	t.Logf("InputTransform: %+v", binding.InputTransform)

	if binding.InputTransform == nil {
		t.Fatal("InputTransform is nil - transform not being parsed!")
	}

	// Apply transform
	inputData := map[string]any{
		"openbindingsVersion": "0.1.0",
		"id":                  "test.interface",
	}

	t.Logf("Input: %v", inputData)

	transformed, err := ApplyTransform(iface.Transforms, binding.InputTransform, inputData)
	if err != nil {
		t.Fatalf("Transform error: %v", err)
	}

	t.Logf("Transformed output: %v", transformed)

	// Verify the transform result - should be flat (no flags wrapper)
	resultMap, ok := transformed.(map[string]any)
	if !ok {
		t.Fatalf("Expected map, got %T", transformed)
	}

	// The key assertion: openbindingsVersion should be mapped to "to" (flat)
	if resultMap["to"] != "0.1.0" {
		t.Errorf("Expected to=0.1.0, got %v", resultMap["to"])
	}

	// id should stay as id (flat)
	if resultMap["id"] != "test.interface" {
		t.Errorf("Expected id=test.interface, got %v", resultMap["id"])
	}

	// openbindingsVersion should NOT be in result (it was mapped to "to")
	if _, exists := resultMap["openbindingsVersion"]; exists {
		t.Errorf("openbindingsVersion should not exist after transform")
	}

	// Should NOT have a flags wrapper - flat output is the new design
	if _, exists := resultMap["flags"]; exists {
		t.Errorf("flags wrapper should not exist - transforms should produce flat output")
	}
}
