package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// validWorkspaceName matches alphanumeric, dash, underscore, 1-64 chars.
var validWorkspaceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidateWorkspaceName checks if a workspace name is valid.
func ValidateWorkspaceName(name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}
	if !validWorkspaceName.MatchString(name) {
		return fmt.Errorf("workspace name %q is invalid: must be 1-64 alphanumeric characters, dashes, or underscores, starting with alphanumeric", name)
	}
	return nil
}

// GenerateTargetID generates a short unique ID for a target.
func GenerateTargetID() string {
	b := make([]byte, 4) // 8 hex chars
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail; if it does, fall back to zero bytes
		// which will still work but may cause collisions
		b = []byte{0, 0, 0, 0}
	}
	return hex.EncodeToString(b)
}

// AddTarget adds a target to a workspace with an auto-generated ID.
// Duplicates (same URL) are allowed - like browser tabs.
// Returns nil if ws is nil.
func AddTarget(ws *Workspace, url, label string) *WorkspaceTarget {
	if ws == nil {
		return nil
	}
	target := WorkspaceTarget{
		ID:    GenerateTargetID(),
		URL:   url,
		Label: label,
	}
	ws.Targets = append(ws.Targets, target)
	return &target
}

// RemoveTargetByID removes a target from a workspace by ID.
// Also removes any associated inputs. Returns false if ws is nil or target not found.
func RemoveTargetByID(ws *Workspace, id string) bool {
	if ws == nil {
		return false
	}
	for i, t := range ws.Targets {
		if t.ID == id {
			ws.Targets = append(ws.Targets[:i], ws.Targets[i+1:]...)
			// Clean up orphaned inputs
			if ws.Inputs != nil {
				delete(ws.Inputs, id)
			}
			return true
		}
	}
	return false
}

// RemoveTargetByIndex removes a target from a workspace by index.
// Also removes any associated inputs. Returns false if ws is nil or index out of bounds.
func RemoveTargetByIndex(ws *Workspace, index int) bool {
	if ws == nil || index < 0 || index >= len(ws.Targets) {
		return false
	}
	// Clean up orphaned inputs
	id := ws.Targets[index].ID
	if ws.Inputs != nil {
		delete(ws.Inputs, id)
	}
	ws.Targets = append(ws.Targets[:index], ws.Targets[index+1:]...)
	return true
}

// FindTargetByID finds a target by ID. Returns nil if ws is nil or target not found.
func FindTargetByID(ws *Workspace, id string) *WorkspaceTarget {
	if ws == nil {
		return nil
	}
	for i := range ws.Targets {
		if ws.Targets[i].ID == id {
			return &ws.Targets[i]
		}
	}
	return nil
}

// RenameWorkspace renames a workspace and updates all references to it.
func RenameWorkspace(envPath, oldName, newName string) error {
	if err := ValidateWorkspaceName(newName); err != nil {
		return err
	}

	if !WorkspaceExists(envPath, oldName) {
		return fmt.Errorf("workspace %q does not exist", oldName)
	}

	if WorkspaceExists(envPath, newName) {
		return fmt.Errorf("workspace %q already exists", newName)
	}

	// Load the workspace
	oldPath := WorkspacePath(envPath, oldName)
	ws, err := LoadWorkspace(oldPath)
	if err != nil {
		return err
	}

	// Update the name
	ws.Name = newName

	// Save to new path
	if err := SaveWorkspace(envPath, ws); err != nil {
		return err
	}

	// Delete old file
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("failed to remove old workspace file: %w", err)
	}

	// Update references in other workspaces
	names, err := ListWorkspaces(envPath)
	if err != nil {
		return err
	}

	for _, name := range names {
		if name == newName {
			continue // skip the renamed workspace itself
		}

		path := WorkspacePath(envPath, name)
		other, err := LoadWorkspace(path)
		if err != nil {
			continue // skip workspaces that fail to load
		}

		if other.Base != nil && *other.Base == oldName {
			other.Base = &newName
			if err := SaveWorkspace(envPath, other); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update base reference in workspace %q: %v\n", name, err)
			}
		}
	}

	// Update active workspace if needed.
	config, _ := LoadEnvConfig(envPath)
	if config != nil && config.ActiveWorkspace == oldName {
		config.ActiveWorkspace = newName
		if err := SaveEnvConfig(envPath, config); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update active workspace config: %v\n", err)
		}
	}

	return nil
}

// CopyWorkspace creates a copy of a workspace with a new name.
// The copy is independent - not linked to the original.
func CopyWorkspace(envPath, srcName, dstName string) error {
	if err := ValidateWorkspaceName(dstName); err != nil {
		return err
	}

	if !WorkspaceExists(envPath, srcName) {
		return fmt.Errorf("workspace %q does not exist", srcName)
	}

	if WorkspaceExists(envPath, dstName) {
		return fmt.Errorf("workspace %q already exists", dstName)
	}

	// Load source workspace
	srcPath := WorkspacePath(envPath, srcName)
	src, err := LoadWorkspace(srcPath)
	if err != nil {
		return err
	}

	// Create copy with new name
	dst := *src // shallow copy is fine for our structs
	dst.Name = dstName

	// Deep copy slices and maps to ensure independence
	if src.Targets != nil {
		dst.Targets = make([]WorkspaceTarget, len(src.Targets))
		copy(dst.Targets, src.Targets)
	}
	if src.Delegates != nil {
		dst.Delegates = make([]string, len(src.Delegates))
		copy(dst.Delegates, src.Delegates)
	}
	if src.DelegatePreferences != nil {
		dst.DelegatePreferences = make(map[string]string)
		for k, v := range src.DelegatePreferences {
			dst.DelegatePreferences[k] = v
		}
	}
	if src.Inputs != nil {
		dst.Inputs = make(map[string]map[string]map[string]string)
		for targetID, ops := range src.Inputs {
			dst.Inputs[targetID] = make(map[string]map[string]string)
			for opKey, names := range ops {
				dst.Inputs[targetID][opKey] = make(map[string]string)
				for name, ref := range names {
					dst.Inputs[targetID][opKey][name] = ref
				}
			}
		}
	}

	return SaveWorkspace(envPath, &dst)
}

// CreateWorkspace creates a new workspace in the environment.
// If baseName is non-empty, the new workspace extends that workspace.
func CreateWorkspace(envPath, name, baseName string) error {
	if err := ValidateWorkspaceName(name); err != nil {
		return err
	}

	// Check if workspace already exists
	if WorkspaceExists(envPath, name) {
		return fmt.Errorf("workspace %q already exists", name)
	}

	// If base is specified, verify it exists
	if baseName != "" && !WorkspaceExists(envPath, baseName) {
		return fmt.Errorf("base workspace %q does not exist", baseName)
	}

	// Create workspace
	ws := Workspace{
		Version: WorkspaceFormatVersion,
		Name:         name,
	}
	if baseName != "" {
		ws.Base = &baseName
	}

	return SaveWorkspace(envPath, &ws)
}

// SaveWorkspace saves a workspace to disk atomically.
func SaveWorkspace(envPath string, ws *Workspace) error {
	// Ensure workspaces directory exists
	workspacesPath := filepath.Join(envPath, WorkspacesDir)
	if err := os.MkdirAll(workspacesPath, DirPerm); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := WorkspacePath(envPath, ws.Name)
	return AtomicWriteFile(path, data, FilePerm)
}

// DeleteWorkspace removes a workspace from the environment.
func DeleteWorkspace(envPath, name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	if !WorkspaceExists(envPath, name) {
		return fmt.Errorf("workspace %q does not exist", name)
	}

	// Don't allow deleting the active workspace
	active, err := GetActiveWorkspace(envPath)
	if err != nil {
		return err
	}
	if active == name {
		return fmt.Errorf("cannot delete active workspace %q; switch to another workspace first", name)
	}

	path := WorkspacePath(envPath, name)
	return os.Remove(path)
}

// UseWorkspace sets the active workspace.
func UseWorkspace(envPath, name string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	if !WorkspaceExists(envPath, name) {
		return fmt.Errorf("workspace %q does not exist", name)
	}

	return SetActiveWorkspace(envPath, name)
}

// RebaseWorkspace changes the base of a workspace.
// If newBase is empty, the workspace becomes standalone.
func RebaseWorkspace(envPath, name, newBase string) error {
	if name == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	path := WorkspacePath(envPath, name)
	ws, err := LoadWorkspace(path)
	if err != nil {
		return fmt.Errorf("failed to load workspace %q: %w", name, err)
	}

	// Verify new base exists (if not going standalone)
	if newBase != "" && !WorkspaceExists(envPath, newBase) {
		return fmt.Errorf("base workspace %q does not exist", newBase)
	}

	// Prevent circular references
	if newBase != "" {
		if err := detectBaseCycle(envPath, name, newBase); err != nil {
			return err
		}
	}

	if newBase == "" {
		ws.Base = nil
	} else {
		ws.Base = &newBase
	}

	return SaveWorkspace(envPath, ws)
}

// detectBaseCycle checks if setting newBase as the base of name would create a cycle.
// It walks the base chain from newBase and checks if name appears in it.
func detectBaseCycle(envPath, name, newBase string) error {
	if newBase == name {
		return fmt.Errorf("workspace cannot be its own base")
	}

	visited := map[string]bool{name: true}
	current := newBase

	for current != "" {
		if visited[current] {
			return fmt.Errorf("circular base reference: %q appears in the base chain of %q", name, newBase)
		}
		visited[current] = true

		path := WorkspacePath(envPath, current)
		ws, err := LoadWorkspace(path)
		if err != nil {
			// Base doesn't exist or is invalid - not a cycle, but will error elsewhere
			return nil
		}

		if ws.Base == nil {
			break
		}
		current = *ws.Base
	}

	return nil
}

// ValidateWorkspaceReferences checks if a workspace's base reference is valid.
// Returns nil if valid, or an error describing the broken reference.
func ValidateWorkspaceReferences(envPath, name string) error {
	path := WorkspacePath(envPath, name)
	ws, err := LoadWorkspace(path)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	if ws.Base != nil && *ws.Base != "" {
		if !WorkspaceExists(envPath, *ws.Base) {
			return fmt.Errorf("base workspace %q does not exist", *ws.Base)
		}
	}

	return nil
}

// ValidateAllWorkspaceReferences checks all workspaces for broken references.
// Returns a map of workspace name to error for any broken workspaces.
func ValidateAllWorkspaceReferences(envPath string) map[string]error {
	names, err := ListWorkspaces(envPath)
	if err != nil {
		return map[string]error{"_list": err}
	}

	broken := make(map[string]error)
	for _, name := range names {
		if err := ValidateWorkspaceReferences(envPath, name); err != nil {
			broken[name] = err
		}
	}

	return broken
}

// WorkspaceInfo returns information about a workspace for display.
type WorkspaceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Base        string `json:"base,omitempty"`
	BaseBroken  bool   `json:"baseBroken,omitempty"`
	IsActive    bool   `json:"isActive"`
	TargetCount int    `json:"targetCount"`
	Path        string `json:"path"`
}

// GetWorkspaceInfo loads workspace info for display.
func GetWorkspaceInfo(envPath, name string) (*WorkspaceInfo, error) {
	path := WorkspacePath(envPath, name)
	ws, err := LoadWorkspace(path)
	if err != nil {
		return nil, err
	}

	active, _ := GetActiveWorkspace(envPath)

	info := &WorkspaceInfo{
		Name:        ws.Name,
		Description: ws.Description,
		IsActive:    ws.Name == active,
		TargetCount: len(ws.Targets),
		Path:        path,
	}
	if ws.Base != nil {
		info.Base = *ws.Base
		// Check if base exists
		if !WorkspaceExists(envPath, *ws.Base) {
			info.BaseBroken = true
		}
	}

	return info, nil
}

// CopyTargetFromWorkspace copies a target from another workspace into the active workspace.
// It also copies any inputs associated with that target.
// The target gets a new ID in the destination workspace.
func CopyTargetFromWorkspace(envPath, srcWorkspaceName, targetID string) (*WorkspaceTarget, error) {
	// Load source workspace
	srcPath := WorkspacePath(envPath, srcWorkspaceName)
	srcWs, err := LoadWorkspace(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load source workspace %q: %w", srcWorkspaceName, err)
	}

	// Find target in source
	srcTarget := FindTargetByID(srcWs, targetID)
	if srcTarget == nil {
		return nil, fmt.Errorf("target %q not found in workspace %q", targetID, srcWorkspaceName)
	}

	// Load active workspace
	activeName, err := GetActiveWorkspace(envPath)
	if err != nil {
		return nil, err
	}

	dstPath := WorkspacePath(envPath, activeName)
	dstWs, err := LoadWorkspace(dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load active workspace %q: %w", activeName, err)
	}

	// Create new target with new ID
	newTarget := WorkspaceTarget{
		ID:    GenerateTargetID(),
		URL:   srcTarget.URL,
		Label: srcTarget.Label,
	}
	dstWs.Targets = append(dstWs.Targets, newTarget)

	// Copy inputs for this target (mapping old ID -> new ID)
	if srcInputs, ok := srcWs.Inputs[targetID]; ok {
		if dstWs.Inputs == nil {
			dstWs.Inputs = make(map[string]map[string]map[string]string)
		}
		dstWs.Inputs[newTarget.ID] = make(map[string]map[string]string)
		for opKey, names := range srcInputs {
			dstWs.Inputs[newTarget.ID][opKey] = make(map[string]string)
			for name, ref := range names {
				dstWs.Inputs[newTarget.ID][opKey][name] = ref
			}
		}
	}

	// Save destination workspace
	if err := SaveWorkspace(envPath, dstWs); err != nil {
		return nil, err
	}

	return &newTarget, nil
}

// ListWorkspaceInfos returns info for all workspaces.
func ListWorkspaceInfos(envPath string) ([]WorkspaceInfo, error) {
	names, err := ListWorkspaces(envPath)
	if err != nil {
		return nil, err
	}

	var infos []WorkspaceInfo
	for _, name := range names {
		info, err := GetWorkspaceInfo(envPath, name)
		if err != nil {
			// Skip workspaces that fail to load
			continue
		}
		infos = append(infos, *info)
	}

	return infos, nil
}
