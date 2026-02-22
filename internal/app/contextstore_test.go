package app

import (
	"os"
	"path/filepath"
	"testing"
)

func setupContextTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	contextsDirFunc = func() (string, error) { return dir, nil }
	t.Cleanup(func() { contextsDirFunc = defaultContextsDir })
	return dir
}

func TestSaveAndLoadContextConfig(t *testing.T) {
	setupContextTestDir(t)

	cfg := ContextConfig{
		Headers:     map[string]string{"Authorization": "Bearer tok"},
		Cookies:     map[string]string{"session": "abc123"},
		Environment: map[string]string{"API_URL": "https://example.com"},
		Metadata:    map[string]any{"region": "us-east-1"},
	}

	if err := SaveContextConfig("test-ctx", cfg); err != nil {
		t.Fatalf("SaveContextConfig: %v", err)
	}

	loaded, err := LoadContextConfig("test-ctx")
	if err != nil {
		t.Fatalf("LoadContextConfig: %v", err)
	}

	if loaded.Headers["Authorization"] != "Bearer tok" {
		t.Errorf("header mismatch: got %q", loaded.Headers["Authorization"])
	}
	if loaded.Cookies["session"] != "abc123" {
		t.Errorf("cookie mismatch: got %q", loaded.Cookies["session"])
	}
	if loaded.Environment["API_URL"] != "https://example.com" {
		t.Errorf("env mismatch: got %q", loaded.Environment["API_URL"])
	}
	if loaded.Metadata["region"] != "us-east-1" {
		t.Errorf("metadata mismatch: got %v", loaded.Metadata["region"])
	}
}

func TestLoadContextConfig_NotFound(t *testing.T) {
	setupContextTestDir(t)

	cfg, err := LoadContextConfig("nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for missing config, got: %v", err)
	}
	if cfg.Headers != nil || cfg.Cookies != nil || cfg.Environment != nil || cfg.Metadata != nil {
		t.Errorf("expected empty config, got: %+v", cfg)
	}
}

func TestDeleteContextConfig(t *testing.T) {
	dir := setupContextTestDir(t)

	cfg := ContextConfig{
		Headers: map[string]string{"X-Test": "value"},
	}
	if err := SaveContextConfig("doomed", cfg); err != nil {
		t.Fatalf("SaveContextConfig: %v", err)
	}

	path := filepath.Join(dir, "doomed.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	loaded, err := LoadContextConfig("doomed")
	if err != nil {
		t.Fatalf("LoadContextConfig after delete: %v", err)
	}
	if loaded.Headers != nil {
		t.Errorf("expected empty config after delete, got: %+v", loaded)
	}
}

func TestContextConfigPath(t *testing.T) {
	dir := setupContextTestDir(t)

	path, err := contextConfigPath("my-api")
	if err != nil {
		t.Fatalf("contextConfigPath: %v", err)
	}
	expected := filepath.Join(dir, "my-api.json")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestSaveContextConfig_OverwritesExisting(t *testing.T) {
	setupContextTestDir(t)

	cfg1 := ContextConfig{
		Headers: map[string]string{"X-Old": "old"},
	}
	if err := SaveContextConfig("overwrite-test", cfg1); err != nil {
		t.Fatalf("SaveContextConfig (first): %v", err)
	}

	cfg2 := ContextConfig{
		Headers: map[string]string{"X-New": "new"},
	}
	if err := SaveContextConfig("overwrite-test", cfg2); err != nil {
		t.Fatalf("SaveContextConfig (second): %v", err)
	}

	loaded, err := LoadContextConfig("overwrite-test")
	if err != nil {
		t.Fatalf("LoadContextConfig: %v", err)
	}
	if _, ok := loaded.Headers["X-Old"]; ok {
		t.Errorf("old header should not be present")
	}
	if loaded.Headers["X-New"] != "new" {
		t.Errorf("new header mismatch: got %q", loaded.Headers["X-New"])
	}
}

func TestSaveContextConfig_EmptyConfig(t *testing.T) {
	setupContextTestDir(t)

	if err := SaveContextConfig("empty", ContextConfig{}); err != nil {
		t.Fatalf("SaveContextConfig: %v", err)
	}

	loaded, err := LoadContextConfig("empty")
	if err != nil {
		t.Fatalf("LoadContextConfig: %v", err)
	}
	if loaded.Headers != nil || loaded.Cookies != nil {
		t.Errorf("expected empty maps to be nil, got: %+v", loaded)
	}
}
