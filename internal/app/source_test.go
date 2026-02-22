package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceAdd_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal OBI.
	obiPath := writeInterface(t, dir, "my.obi.json", minimalInterface(map[string]any{}))

	// Create a dummy artifact file.
	artifactPath := filepath.Join(dir, "cli.kdl")
	os.WriteFile(artifactPath, []byte("# dummy"), 0644)

	result, err := SourceAdd(SourceAddInput{
		OBIPath:  obiPath,
		Format:   "usage@2.13.1",
		Location: artifactPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Format != "usage@2.13.1" {
		t.Errorf("expected format usage@2.13.1, got %q", result.Format)
	}
	if result.Key == "" {
		t.Error("expected derived key, got empty")
	}

	// Verify the OBI file was updated.
	data, _ := os.ReadFile(obiPath)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	sources, ok := parsed["sources"].(map[string]any)
	if !ok {
		t.Fatal("expected sources in OBI")
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestSourceAdd_KeyCollision(t *testing.T) {
	dir := t.TempDir()

	// Create OBI with existing source.
	obiData := map[string]any{
		"openbindings": "0.1.0",
		"id":           "test",
		"operations":   map[string]any{},
		"sources": map[string]any{
			"myKey": map[string]any{
				"format":   "usage@2.0.0",
				"location": "existing.kdl",
			},
		},
	}
	obiPath := writeInterface(t, dir, "my.obi.json", obiData)

	_, err := SourceAdd(SourceAddInput{
		OBIPath:  obiPath,
		Format:   "usage@2.13.1",
		Location: "other.kdl",
		Key:      "myKey", // collides with existing
	})

	if err == nil {
		t.Fatal("expected key collision error")
	}
}

func TestSourceAdd_ExplicitKey(t *testing.T) {
	dir := t.TempDir()

	obiPath := writeInterface(t, dir, "my.obi.json", minimalInterface(map[string]any{}))

	// Create the source file so SourceAdd can read it for hashing.
	srcPath := filepath.Join(dir, "cli.kdl")
	os.WriteFile(srcPath, []byte("# dummy"), 0644)

	result, err := SourceAdd(SourceAddInput{
		OBIPath:  obiPath,
		Format:   "usage@2.13.1",
		Location: srcPath,
		Key:      "myCli",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Key != "myCli" {
		t.Errorf("expected key 'myCli', got %q", result.Key)
	}
}

func TestSourceList_Empty(t *testing.T) {
	dir := t.TempDir()

	obiPath := writeInterface(t, dir, "my.obi.json", minimalInterface(map[string]any{}))

	result, err := SourceList(obiPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(result.Sources))
	}
}

func TestSourceList_WithSources(t *testing.T) {
	dir := t.TempDir()

	obiData := map[string]any{
		"openbindings": "0.1.0",
		"id":           "test",
		"operations":   map[string]any{},
		"sources": map[string]any{
			"restApi": map[string]any{
				"format":   "openapi@3.1",
				"location": "api.yaml",
			},
			"cliSpec": map[string]any{
				"format":   "usage@2.0.0",
				"location": "cli.kdl",
			},
		},
	}
	obiPath := writeInterface(t, dir, "my.obi.json", obiData)

	result, err := SourceList(obiPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(result.Sources))
	}

	// Verify sorted by key.
	if result.Sources[0].Key != "cliSpec" {
		t.Errorf("expected first source 'cliSpec', got %q", result.Sources[0].Key)
	}
	if result.Sources[1].Key != "restApi" {
		t.Errorf("expected second source 'restApi', got %q", result.Sources[1].Key)
	}
}

func TestSourceRemove_Basic(t *testing.T) {
	dir := t.TempDir()

	obiData := map[string]any{
		"openbindings": "0.1.0",
		"id":           "test",
		"operations":   map[string]any{},
		"sources": map[string]any{
			"myApi": map[string]any{
				"format":   "openapi@3.1",
				"location": "api.yaml",
			},
		},
	}
	obiPath := writeInterface(t, dir, "my.obi.json", obiData)

	result, err := SourceRemove(obiPath, "myApi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Key != "myApi" {
		t.Errorf("expected key 'myApi', got %q", result.Key)
	}
	if result.OrphanedBindings != 0 {
		t.Errorf("expected 0 orphaned bindings, got %d", result.OrphanedBindings)
	}

	// Verify removed from file.
	iface, _ := loadInterfaceFile(obiPath)
	if len(iface.Sources) != 0 {
		t.Errorf("expected 0 sources after removal, got %d", len(iface.Sources))
	}
}

func TestSourceRemove_NotFound(t *testing.T) {
	dir := t.TempDir()

	obiPath := writeInterface(t, dir, "my.obi.json", minimalInterface(map[string]any{}))

	_, err := SourceRemove(obiPath, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestSourceRemove_OrphanedBindings(t *testing.T) {
	dir := t.TempDir()

	obiData := map[string]any{
		"openbindings": "0.1.0",
		"id":           "test",
		"operations": map[string]any{
			"greet": map[string]any{"kind": "method"},
		},
		"sources": map[string]any{
			"myApi": map[string]any{
				"format":   "openapi@3.1",
				"location": "api.yaml",
			},
		},
		"bindings": map[string]any{
			"greet.myApi": map[string]any{"operation": "greet", "source": "myApi", "ref": "GET /greet"},
		},
	}
	obiPath := writeInterface(t, dir, "my.obi.json", obiData)

	result, err := SourceRemove(obiPath, "myApi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OrphanedBindings != 1 {
		t.Errorf("expected 1 orphaned binding, got %d", result.OrphanedBindings)
	}
}

func TestSourceAdd_RelativePath(t *testing.T) {
	dir := t.TempDir()

	// Create subdirectory structure.
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0755)

	obiPath := writeInterface(t, dir, "my.obi.json", minimalInterface(map[string]any{}))
	artifactPath := filepath.Join(subDir, "cli.kdl")
	os.WriteFile(artifactPath, []byte("# dummy"), 0644)

	result, err := SourceAdd(SourceAddInput{
		OBIPath:  obiPath,
		Format:   "usage@2.13.1",
		Location: artifactPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ref should be stored relative to OBI directory.
	if filepath.IsAbs(result.Ref) {
		t.Errorf("expected relative ref path, got %q", result.Ref)
	}
}

func TestSourceList_RenderOutput(t *testing.T) {
	output := SourceListOutput{
		Sources: []SourceEntry{
			{Key: "cliSpec", Format: "usage@2.0.0", Location: "cli.kdl"},
			{Key: "restApi", Format: "openapi@3.1", Location: "api.yaml"},
		},
	}

	rendered := output.Render()
	if rendered == "" {
		t.Error("expected non-empty render output")
	}
}

func TestSourceList_RenderEmpty(t *testing.T) {
	output := SourceListOutput{}
	rendered := output.Render()
	if rendered == "" {
		t.Error("expected non-empty render output for empty list")
	}
}
