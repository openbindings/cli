package app

import (
	"testing"
)

func TestDiff_IdenticalOBIs(t *testing.T) {
	dir := t.TempDir()

	iface := minimalInterface(map[string]any{
		"greet": map[string]any{
			"kind": "method",
			"input": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	})

	a := writeInterface(t, dir, "a.json", iface)
	b := writeInterface(t, dir, "b.json", iface)

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Identical {
		t.Error("expected identical, got differences")
	}
	if len(report.Operations) != 1 {
		t.Errorf("expected 1 operation, got %d", len(report.Operations))
	}
	if report.Operations[0].Status != DiffInSync {
		t.Errorf("expected in-sync, got %s", report.Operations[0].Status)
	}
}

func TestDiff_AddedOperation(t *testing.T) {
	dir := t.TempDir()

	a := writeInterface(t, dir, "a.json", minimalInterface(map[string]any{
		"greet": map[string]any{"kind": "method"},
	}))
	b := writeInterface(t, dir, "b.json", minimalInterface(map[string]any{
		"greet":   map[string]any{"kind": "method"},
		"goodbye": map[string]any{"kind": "method"},
	}))

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Identical {
		t.Error("expected differences, got identical")
	}

	// Find added operation.
	found := false
	for _, op := range report.Operations {
		if op.Operation == "goodbye" && op.Status == DiffAdded {
			found = true
		}
	}
	if !found {
		t.Error("expected 'goodbye' to be added")
	}
}

func TestDiff_RemovedOperation(t *testing.T) {
	dir := t.TempDir()

	a := writeInterface(t, dir, "a.json", minimalInterface(map[string]any{
		"greet":   map[string]any{"kind": "method"},
		"goodbye": map[string]any{"kind": "method"},
	}))
	b := writeInterface(t, dir, "b.json", minimalInterface(map[string]any{
		"greet": map[string]any{"kind": "method"},
	}))

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Identical {
		t.Error("expected differences, got identical")
	}

	found := false
	for _, op := range report.Operations {
		if op.Operation == "goodbye" && op.Status == DiffRemoved {
			found = true
		}
	}
	if !found {
		t.Error("expected 'goodbye' to be removed")
	}
}

func TestDiff_ChangedOperation(t *testing.T) {
	dir := t.TempDir()

	a := writeInterface(t, dir, "a.json", minimalInterface(map[string]any{
		"greet": map[string]any{
			"kind": "method",
			"input": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}))
	b := writeInterface(t, dir, "b.json", minimalInterface(map[string]any{
		"greet": map[string]any{
			"kind": "method",
			"input": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "integer"},
				},
			},
		},
	}))

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Identical {
		t.Error("expected differences")
	}

	found := false
	for _, op := range report.Operations {
		if op.Operation == "greet" && op.Status == DiffChanged {
			found = true
			if len(op.Details) == 0 {
				t.Error("expected details on changed operation")
			}
		}
	}
	if !found {
		t.Error("expected 'greet' to be changed")
	}
}

func TestDiff_MetadataDiff(t *testing.T) {
	dir := t.TempDir()

	a := writeInterface(t, dir, "a.json", map[string]any{
		"openbindings": "0.1.0",
		"name":        "My API",
		"version":     "1.0.0",
		"operations":  map[string]any{},
	})
	b := writeInterface(t, dir, "b.json", map[string]any{
		"openbindings": "0.1.0",
		"name":        "My API v2",
		"version":     "2.0.0",
		"operations":  map[string]any{},
	})

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Identical {
		t.Error("expected metadata differences")
	}

	if len(report.Metadata) < 2 {
		t.Errorf("expected at least 2 metadata diffs (name + version), got %d", len(report.Metadata))
	}
}

func TestDiff_KindChange(t *testing.T) {
	dir := t.TempDir()

	a := writeInterface(t, dir, "a.json", minimalInterface(map[string]any{
		"notify": map[string]any{"kind": "method"},
	}))
	b := writeInterface(t, dir, "b.json", minimalInterface(map[string]any{
		"notify": map[string]any{"kind": "event"},
	}))

	report, err := Diff(DiffInput{
		BaselineLocator:   a,
		ComparisonLocator: b,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, op := range report.Operations {
		if op.Operation == "notify" && op.Status == DiffChanged {
			found = true
			hasKindDetail := false
			for _, d := range op.Details {
				if d == `kind: "method" → "event"` {
					hasKindDetail = true
				}
			}
			if !hasKindDetail {
				t.Errorf("expected kind change detail, got %v", op.Details)
			}
		}
	}
	if !found {
		t.Error("expected 'notify' to be changed")
	}
}

func TestDiff_RenderOutput(t *testing.T) {
	report := DiffReport{
		Identical: false,
		Operations: []OperationDiff{
			{Operation: "greet", Status: DiffInSync},
			{Operation: "goodbye", Status: DiffAdded},
			{Operation: "hello", Status: DiffRemoved},
			{Operation: "update", Status: DiffChanged, Details: []string{"input schema differs"}},
		},
	}

	rendered := report.Render()
	if rendered == "" {
		t.Error("expected non-empty render output")
	}
}

func TestDiff_IdenticalRender(t *testing.T) {
	report := DiffReport{Identical: true}
	rendered := report.Render()
	if rendered == "" {
		t.Error("expected non-empty render output for identical")
	}
}
