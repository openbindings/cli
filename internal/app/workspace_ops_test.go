package app

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create workspaces directory
	workspacesPath := filepath.Join(tmpDir, WorkspacesDir)
	os.MkdirAll(workspacesPath, DirPerm)

	// Create default workspace
	defaultWs := Workspace{
		Version: WorkspaceFormatVersion,
		Name:         DefaultWorkspaceName,
	}
	SaveWorkspace(tmpDir, &defaultWs)

	// Set default as active
	SetActiveWorkspace(tmpDir, DefaultWorkspaceName)

	return tmpDir
}

func TestCreateWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create standalone workspace
	err := CreateWorkspace(envPath, "standalone", "")
	if err != nil {
		t.Fatalf("CreateWorkspace() failed: %v", err)
	}

	if !WorkspaceExists(envPath, "standalone") {
		t.Error("workspace 'standalone' was not created")
	}

	// Verify it has no base
	ws, _ := LoadWorkspace(WorkspacePath(envPath, "standalone"))
	if ws.Base != nil {
		t.Errorf("expected no base, got %q", *ws.Base)
	}
}

func TestCreateWorkspace_WithBase(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create workspace with base
	err := CreateWorkspace(envPath, "child", DefaultWorkspaceName)
	if err != nil {
		t.Fatalf("CreateWorkspace() failed: %v", err)
	}

	ws, _ := LoadWorkspace(WorkspacePath(envPath, "child"))
	if ws.Base == nil || *ws.Base != DefaultWorkspaceName {
		t.Errorf("expected base %q, got %v", DefaultWorkspaceName, ws.Base)
	}
}

func TestCreateWorkspace_AlreadyExists(t *testing.T) {
	envPath := setupTestEnv(t)

	err := CreateWorkspace(envPath, DefaultWorkspaceName, "")
	if err == nil {
		t.Error("expected error for duplicate workspace")
	}
}

func TestCreateWorkspace_InvalidBase(t *testing.T) {
	envPath := setupTestEnv(t)

	err := CreateWorkspace(envPath, "child", "nonexistent")
	if err == nil {
		t.Error("expected error for invalid base")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create a workspace to delete
	CreateWorkspace(envPath, "to-delete", "")

	// Switch away from it first
	SetActiveWorkspace(envPath, DefaultWorkspaceName)

	err := DeleteWorkspace(envPath, "to-delete")
	if err != nil {
		t.Fatalf("DeleteWorkspace() failed: %v", err)
	}

	if WorkspaceExists(envPath, "to-delete") {
		t.Error("workspace was not deleted")
	}
}

func TestDeleteWorkspace_CannotDeleteActive(t *testing.T) {
	envPath := setupTestEnv(t)

	err := DeleteWorkspace(envPath, DefaultWorkspaceName)
	if err == nil {
		t.Error("expected error when deleting active workspace")
	}
}

func TestUseWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create another workspace
	CreateWorkspace(envPath, "other", "")

	err := UseWorkspace(envPath, "other")
	if err != nil {
		t.Fatalf("UseWorkspace() failed: %v", err)
	}

	active, _ := GetActiveWorkspace(envPath)
	if active != "other" {
		t.Errorf("expected active %q, got %q", "other", active)
	}
}

func TestUseWorkspace_NotExists(t *testing.T) {
	envPath := setupTestEnv(t)

	err := UseWorkspace(envPath, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}
}

func TestRebaseWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create a base and a child
	CreateWorkspace(envPath, "base1", "")
	CreateWorkspace(envPath, "base2", "")
	CreateWorkspace(envPath, "child", "base1")

	// Rebase to base2
	err := RebaseWorkspace(envPath, "child", "base2")
	if err != nil {
		t.Fatalf("RebaseWorkspace() failed: %v", err)
	}

	ws, _ := LoadWorkspace(WorkspacePath(envPath, "child"))
	if ws.Base == nil || *ws.Base != "base2" {
		t.Errorf("expected base 'base2', got %v", ws.Base)
	}
}

func TestRebaseWorkspace_ToStandalone(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "child", DefaultWorkspaceName)

	err := RebaseWorkspace(envPath, "child", "")
	if err != nil {
		t.Fatalf("RebaseWorkspace() failed: %v", err)
	}

	ws, _ := LoadWorkspace(WorkspacePath(envPath, "child"))
	if ws.Base != nil {
		t.Errorf("expected no base, got %q", *ws.Base)
	}
}

func TestRebaseWorkspace_CircularReference_Self(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "self", "")

	err := RebaseWorkspace(envPath, "self", "self")
	if err == nil {
		t.Error("expected error for circular reference")
	}
}

func TestRebaseWorkspace_CircularReference_Chain(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create A → B → C chain
	CreateWorkspace(envPath, "a", "")
	CreateWorkspace(envPath, "b", "a")
	CreateWorkspace(envPath, "c", "b")

	// Try to make A → C (would create C → B → A → C)
	err := RebaseWorkspace(envPath, "a", "c")
	if err == nil {
		t.Error("expected error for circular chain reference")
	}
}

func TestValidateWorkspaceName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"default", false},
		{"my-workspace", false},
		{"my_workspace", false},
		{"Workspace123", false},
		{"a", false},
		{"", true},                        // empty
		{".hidden", true},                 // starts with dot
		{"../escape", true},               // path traversal
		{"foo/bar", true},                 // contains slash
		{"-starts-with-dash", true},       // starts with dash
		{"_starts-with-underscore", true}, // starts with underscore
		{"this-name-is-way-too-long-and-exceeds-the-64-character-limit-for-sure-now", true}, // too long (>64 chars)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspaceName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspaceName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestCreateWorkspace_InvalidName(t *testing.T) {
	envPath := setupTestEnv(t)

	err := CreateWorkspace(envPath, "../escape", "")
	if err == nil {
		t.Error("expected error for invalid workspace name")
	}
}

func TestGetWorkspaceInfo(t *testing.T) {
	envPath := setupTestEnv(t)

	// Add some targets to default
	path := WorkspacePath(envPath, DefaultWorkspaceName)
	ws, _ := LoadWorkspace(path)
	ws.Targets = []WorkspaceTarget{
		{URL: "exec:test1"},
		{URL: "exec:test2"},
	}
	ws.Description = "Test workspace"
	SaveWorkspace(envPath, ws)

	info, err := GetWorkspaceInfo(envPath, DefaultWorkspaceName)
	if err != nil {
		t.Fatalf("GetWorkspaceInfo() failed: %v", err)
	}

	if info.Name != DefaultWorkspaceName {
		t.Errorf("expected name %q, got %q", DefaultWorkspaceName, info.Name)
	}
	if info.Description != "Test workspace" {
		t.Errorf("expected description 'Test workspace', got %q", info.Description)
	}
	if !info.IsActive {
		t.Error("expected IsActive to be true")
	}
	if info.TargetCount != 2 {
		t.Errorf("expected 2 targets, got %d", info.TargetCount)
	}
}

func TestListWorkspaceInfos(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "ws1", "")
	CreateWorkspace(envPath, "ws2", "")

	infos, err := ListWorkspaceInfos(envPath)
	if err != nil {
		t.Fatalf("ListWorkspaceInfos() failed: %v", err)
	}

	if len(infos) != 3 { // default + ws1 + ws2
		t.Errorf("expected 3 workspaces, got %d", len(infos))
	}
}

func TestValidateWorkspaceReferences_Valid(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "child", DefaultWorkspaceName)

	err := ValidateWorkspaceReferences(envPath, "child")
	if err != nil {
		t.Errorf("expected valid workspace, got error: %v", err)
	}
}

func TestValidateWorkspaceReferences_BrokenBase(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create child with base
	CreateWorkspace(envPath, "child", DefaultWorkspaceName)

	// Delete the base (bypass the normal check)
	os.Remove(WorkspacePath(envPath, DefaultWorkspaceName))

	// Now child has broken reference
	err := ValidateWorkspaceReferences(envPath, "child")
	if err == nil {
		t.Error("expected error for broken base reference")
	}
}

func TestGetWorkspaceInfo_BrokenBase(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "child", DefaultWorkspaceName)
	os.Remove(WorkspacePath(envPath, DefaultWorkspaceName))

	info, err := GetWorkspaceInfo(envPath, "child")
	if err != nil {
		t.Fatalf("GetWorkspaceInfo() failed: %v", err)
	}

	if !info.BaseBroken {
		t.Error("expected BaseBroken to be true")
	}
}

func TestValidateAllWorkspaceReferences(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "good", "")
	CreateWorkspace(envPath, "bad", DefaultWorkspaceName)

	// Break the bad workspace's base
	os.Remove(WorkspacePath(envPath, DefaultWorkspaceName))

	broken := ValidateAllWorkspaceReferences(envPath)

	if len(broken) != 1 {
		t.Errorf("expected 1 broken workspace, got %d", len(broken))
	}

	if _, ok := broken["bad"]; !ok {
		t.Error("expected 'bad' workspace to be in broken list")
	}
}

func TestAddTarget(t *testing.T) {
	ws := &Workspace{}

	// Add first target
	target1 := AddTarget(ws, "exec:cli1", "CLI 1")
	if target1 == nil {
		t.Fatal("AddTarget() returned nil")
	}
	if target1.ID == "" {
		t.Error("target should have an ID")
	}
	if target1.URL != "exec:cli1" {
		t.Errorf("expected URL 'exec:cli1', got %q", target1.URL)
	}
	if target1.Label != "CLI 1" {
		t.Errorf("expected label 'CLI 1', got %q", target1.Label)
	}

	if len(ws.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(ws.Targets))
	}

	// Add second target
	target2 := AddTarget(ws, "exec:cli2", "")
	if target2.ID == "" || target2.ID == target1.ID {
		t.Error("each target should have a unique ID")
	}

	if len(ws.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(ws.Targets))
	}

	// Duplicates are allowed (like browser tabs)
	target3 := AddTarget(ws, "exec:cli1", "CLI 1 copy")
	if target3 == nil {
		t.Error("duplicate URLs should be allowed")
	}
	if target3.ID == target1.ID {
		t.Error("duplicate should have different ID")
	}

	if len(ws.Targets) != 3 {
		t.Errorf("expected 3 targets after adding duplicate, got %d", len(ws.Targets))
	}
}

func TestRemoveTargetByID(t *testing.T) {
	ws := &Workspace{}

	target1 := AddTarget(ws, "exec:cli1", "")
	target2 := AddTarget(ws, "exec:cli2", "")
	AddTarget(ws, "exec:cli3", "")

	// Remove middle target by ID
	removed := RemoveTargetByID(ws, target2.ID)
	if !removed {
		t.Error("expected target to be removed")
	}

	if len(ws.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(ws.Targets))
	}

	// Verify correct target was removed
	if FindTargetByID(ws, target2.ID) != nil {
		t.Error("target2 should have been removed")
	}
	if FindTargetByID(ws, target1.ID) == nil {
		t.Error("target1 should still exist")
	}

	// Try to remove non-existent
	removed = RemoveTargetByID(ws, "nonexistent-id")
	if removed {
		t.Error("expected false for non-existent target")
	}
}

func TestRemoveTargetByIndex(t *testing.T) {
	ws := &Workspace{}

	AddTarget(ws, "exec:cli1", "")
	AddTarget(ws, "exec:cli2", "")
	AddTarget(ws, "exec:cli3", "")

	// Remove by index
	removed := RemoveTargetByIndex(ws, 1)
	if !removed {
		t.Error("expected target to be removed")
	}

	if len(ws.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(ws.Targets))
	}

	// Verify order is preserved
	if ws.Targets[0].URL != "exec:cli1" {
		t.Error("first target should still be cli1")
	}
	if ws.Targets[1].URL != "exec:cli3" {
		t.Error("second target should now be cli3")
	}

	// Invalid index
	if RemoveTargetByIndex(ws, -1) {
		t.Error("negative index should return false")
	}
	if RemoveTargetByIndex(ws, 99) {
		t.Error("out of bounds index should return false")
	}
}

func TestRenameWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create a workspace with a child
	CreateWorkspace(envPath, "parent", "")
	CreateWorkspace(envPath, "child", "parent")

	// Rename parent
	err := RenameWorkspace(envPath, "parent", "newparent")
	if err != nil {
		t.Fatalf("RenameWorkspace() failed: %v", err)
	}

	// Old name should not exist
	if WorkspaceExists(envPath, "parent") {
		t.Error("old workspace name should not exist")
	}

	// New name should exist
	if !WorkspaceExists(envPath, "newparent") {
		t.Error("new workspace name should exist")
	}

	// Child's base should be updated
	childPath := WorkspacePath(envPath, "child")
	child, _ := LoadWorkspace(childPath)
	if child.Base == nil || *child.Base != "newparent" {
		t.Errorf("child's base should be updated to 'newparent', got %v", child.Base)
	}
}

func TestRenameWorkspace_UpdatesActive(t *testing.T) {
	envPath := setupTestEnv(t)

	CreateWorkspace(envPath, "myws", "")
	SetActiveWorkspace(envPath, "myws")

	err := RenameWorkspace(envPath, "myws", "renamed")
	if err != nil {
		t.Fatalf("RenameWorkspace() failed: %v", err)
	}

	active, _ := GetActiveWorkspace(envPath)
	if active != "renamed" {
		t.Errorf("active workspace should be 'renamed', got %q", active)
	}
}

func TestCopyWorkspace(t *testing.T) {
	envPath := setupTestEnv(t)

	// Create source with some data
	CreateWorkspace(envPath, "source", "")
	srcPath := WorkspacePath(envPath, "source")
	src, _ := LoadWorkspace(srcPath)
	src.Description = "Original"
	src.Targets = []WorkspaceTarget{{URL: "exec:test"}}
	src.Delegates = []string{"exec:my-delegate"}
	SaveWorkspace(envPath, src)

	// Copy it
	err := CopyWorkspace(envPath, "source", "dest")
	if err != nil {
		t.Fatalf("CopyWorkspace() failed: %v", err)
	}

	// Verify copy exists
	if !WorkspaceExists(envPath, "dest") {
		t.Error("copy should exist")
	}

	// Verify copy has the data
	dstPath := WorkspacePath(envPath, "dest")
	dst, _ := LoadWorkspace(dstPath)

	if dst.Name != "dest" {
		t.Errorf("copy name should be 'dest', got %q", dst.Name)
	}
	if dst.Description != "Original" {
		t.Errorf("copy should have description 'Original', got %q", dst.Description)
	}
	if len(dst.Targets) != 1 {
		t.Errorf("copy should have 1 target, got %d", len(dst.Targets))
	}

	// Modify copy, verify source is unchanged
	dst.Description = "Modified"
	SaveWorkspace(envPath, dst)

	src2, _ := LoadWorkspace(srcPath)
	if src2.Description != "Original" {
		t.Error("modifying copy should not affect source")
	}
}
