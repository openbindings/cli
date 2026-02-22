package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	// Create a temp directory and change to it
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Run init
	err := Init(InitParams{})
	// Init returns ExitResult with code 0 on success
	if exitErr, ok := err.(ExitResult); ok {
		if exitErr.Code != 0 {
			t.Fatalf("Init() failed: %v", err)
		}
	} else if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Check .openbindings/ exists
	envPath := filepath.Join(tmpDir, EnvDir)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Error(".openbindings/ directory not created")
	}

	// Check workspaces/ exists
	workspacesPath := filepath.Join(envPath, WorkspacesDir)
	if _, err := os.Stat(workspacesPath); os.IsNotExist(err) {
		t.Error(".openbindings/workspaces/ directory not created")
	}

	// Check default.json exists
	defaultPath := filepath.Join(workspacesPath, "default.json")
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		t.Error(".openbindings/workspaces/default.json not created")
	}

	// Check config.json exists and has correct active workspace
	configPath := filepath.Join(envPath, EnvConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error(".openbindings/config.json not created")
	}

	config, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Errorf("failed to load env config: %v", err)
	}
	if config.ActiveWorkspace != DefaultWorkspaceName {
		t.Errorf("active workspace is %q, expected %q", config.ActiveWorkspace, DefaultWorkspaceName)
	}

	// Verify default workspace is valid
	ws, err := LoadWorkspace(defaultPath)
	if err != nil {
		t.Errorf("default workspace is invalid: %v", err)
	}
	if ws.Name != DefaultWorkspaceName {
		t.Errorf("workspace name is %q, expected %q", ws.Name, DefaultWorkspaceName)
	}
	if ws.Version != WorkspaceFormatVersion {
		t.Errorf("workspace version is %q, expected %q", ws.Version, WorkspaceFormatVersion)
	}
}

func TestInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create .openbindings/ first
	os.MkdirAll(filepath.Join(tmpDir, EnvDir), DirPerm)

	// Init should fail
	err := Init(InitParams{})
	if err == nil {
		t.Error("expected error when .openbindings/ already exists")
	}
}

func TestFindEnvironment_Local(t *testing.T) {
	tmpDir := t.TempDir()

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)

	// Create .openbindings/ in tmpDir
	envPath := filepath.Join(tmpDir, EnvDir)
	os.MkdirAll(envPath, DirPerm)

	// Create a subdirectory and work from there
	subDir := filepath.Join(tmpDir, "sub", "dir")
	os.MkdirAll(subDir, DirPerm)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(subDir)

	found, isLocal, err := FindEnvironment()
	if err != nil {
		t.Fatalf("FindEnvironment() failed: %v", err)
	}
	if !isLocal {
		t.Error("expected local environment")
	}
	if found != envPath {
		t.Errorf("found %q, expected %q", found, envPath)
	}
}

func TestListWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesPath := filepath.Join(tmpDir, WorkspacesDir)
	os.MkdirAll(workspacesPath, DirPerm)

	// Create some workspace files
	os.WriteFile(filepath.Join(workspacesPath, "default.json"), []byte("{}"), FilePerm)
	os.WriteFile(filepath.Join(workspacesPath, "staging.json"), []byte("{}"), FilePerm)
	os.WriteFile(filepath.Join(workspacesPath, "prod.json"), []byte("{}"), FilePerm)

	names, err := ListWorkspaces(tmpDir)
	if err != nil {
		t.Fatalf("ListWorkspaces() failed: %v", err)
	}

	if len(names) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(names))
	}

	// Check all names are present (order may vary)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"default", "staging", "prod"} {
		if !nameSet[expected] {
			t.Errorf("missing workspace %q", expected)
		}
	}
}

func TestGetSetActiveWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspaces directory and staging workspace
	workspacesPath := filepath.Join(tmpDir, WorkspacesDir)
	os.MkdirAll(workspacesPath, DirPerm)

	ws := Workspace{Version: WorkspaceFormatVersion, Name: "staging"}
	data, _ := json.Marshal(ws)
	os.WriteFile(filepath.Join(workspacesPath, "staging.json"), data, FilePerm)

	// No config file - should return default
	name, err := GetActiveWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("GetActiveWorkspace() failed: %v", err)
	}
	// Since default doesn't exist, it will fall back to staging (first available)
	if name != "staging" {
		t.Errorf("expected fallback to %q, got %q", "staging", name)
	}

	// Create default workspace
	wsDefault := Workspace{Version: WorkspaceFormatVersion, Name: DefaultWorkspaceName}
	dataDefault, _ := json.Marshal(wsDefault)
	os.WriteFile(filepath.Join(workspacesPath, DefaultWorkspaceName+".json"), dataDefault, FilePerm)

	// Now set active to staging explicitly
	err = SetActiveWorkspace(tmpDir, "staging")
	if err != nil {
		t.Fatalf("SetActiveWorkspace() failed: %v", err)
	}

	// Get active
	name, err = GetActiveWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("GetActiveWorkspace() failed: %v", err)
	}
	if name != "staging" {
		t.Errorf("expected %q, got %q", "staging", name)
	}
}

func TestLoadSaveEnvConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Load non-existent config - should return defaults
	config, err := LoadEnvConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadEnvConfig() failed: %v", err)
	}
	if config.ActiveWorkspace != DefaultWorkspaceName {
		t.Errorf("expected default workspace %q, got %q", DefaultWorkspaceName, config.ActiveWorkspace)
	}

	// Save config
	config.ActiveWorkspace = "production"
	err = SaveEnvConfig(tmpDir, config)
	if err != nil {
		t.Fatalf("SaveEnvConfig() failed: %v", err)
	}

	// Load again
	loaded, err := LoadEnvConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadEnvConfig() failed: %v", err)
	}
	if loaded.ActiveWorkspace != "production" {
		t.Errorf("expected %q, got %q", "production", loaded.ActiveWorkspace)
	}
}

func TestGetActiveWorkspace_DeletedWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up environment with workspaces
	workspacesPath := filepath.Join(tmpDir, WorkspacesDir)
	os.MkdirAll(workspacesPath, DirPerm)

	// Create two workspaces
	ws1 := Workspace{Version: WorkspaceFormatVersion, Name: "first"}
	ws2 := Workspace{Version: WorkspaceFormatVersion, Name: "second"}

	data1, _ := json.Marshal(ws1)
	data2, _ := json.Marshal(ws2)
	os.WriteFile(filepath.Join(workspacesPath, "first.json"), data1, FilePerm)
	os.WriteFile(filepath.Join(workspacesPath, "second.json"), data2, FilePerm)

	// Set "first" as active
	SetActiveWorkspace(tmpDir, "first")

	// Delete "first"
	os.Remove(filepath.Join(workspacesPath, "first.json"))

	// GetActiveWorkspace should recover to "second" (the remaining workspace)
	active, err := GetActiveWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("GetActiveWorkspace() failed: %v", err)
	}

	if active != "second" {
		t.Errorf("expected fallback to %q, got %q", "second", active)
	}

	// Config should be auto-fixed
	config, _ := LoadEnvConfig(tmpDir)
	if config.ActiveWorkspace != "second" {
		t.Errorf("config not auto-fixed: expected %q, got %q", "second", config.ActiveWorkspace)
	}
}

func TestWorkspaceExists(t *testing.T) {
	tmpDir := t.TempDir()
	workspacesPath := filepath.Join(tmpDir, WorkspacesDir)
	os.MkdirAll(workspacesPath, DirPerm)

	os.WriteFile(filepath.Join(workspacesPath, "exists.json"), []byte("{}"), FilePerm)

	if !WorkspaceExists(tmpDir, "exists") {
		t.Error("expected workspace to exist")
	}
	if WorkspaceExists(tmpDir, "notexists") {
		t.Error("expected workspace to not exist")
	}
}
