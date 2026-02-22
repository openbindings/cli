package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed workspace.schema.json
var workspaceSchemaJSON []byte

var (
	workspaceSchema     *jsonschema.Schema
	workspaceSchemaOnce sync.Once
	workspaceSchemaErr  error
)

// WorkspaceFormatVersion is the current version of the workspace format.
const WorkspaceFormatVersion = "0.1.0"

// Workspace represents an OpenBindings CLI workspace configuration.
type Workspace struct {
	Version            string                                  `json:"version"`
	Name               string                                  `json:"name"`
	Description        string                                  `json:"description,omitempty"`
	Base               *string                                 `json:"base,omitempty"`
	Targets            []WorkspaceTarget                       `json:"targets,omitempty"`
	Settings           WorkspaceSettings                       `json:"settings,omitempty"`
	Delegates            []string                                `json:"delegates,omitempty"`
	DelegatePreferences  map[string]string                       `json:"delegatePreferences,omitempty"`
	Inputs             map[string]map[string]map[string]string `json:"inputs,omitempty"` // targetID -> opKey -> inputName -> inputRef
	UI                 *WorkspaceUI                            `json:"ui,omitempty"`     // TUI state for session restoration
}

// WorkspaceTarget represents a target in a workspace.
type WorkspaceTarget struct {
	ID    string `json:"id"`              // Unique identifier (auto-generated)
	URL   string `json:"url"`             // Target URL (exec:, https://, etc.)
	Label string `json:"label,omitempty"` // Optional display label
}

// WorkspaceSettings represents workspace settings.
type WorkspaceSettings struct {
	Editor       string `json:"editor,omitempty"`
	OutputFormat string `json:"outputFormat,omitempty"`
}

// WorkspaceUI holds TUI state for session restoration.
type WorkspaceUI struct {
	ActiveTargetID string                        `json:"activeTargetId,omitempty"`
	Targets        map[string]*WorkspaceUITarget `json:"targets,omitempty"`
}

// WorkspaceUITarget holds per-target TUI state.
type WorkspaceUITarget struct {
	Expanded []string `json:"expanded,omitempty"` // Tree paths that are expanded
	Selected *string  `json:"selected,omitempty"` // Currently selected tree path
}

// WorkspaceSchema returns the compiled JSON Schema for workspace validation.
func WorkspaceSchema() (*jsonschema.Schema, error) {
	workspaceSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource("workspace.schema.json", strings.NewReader(string(workspaceSchemaJSON))); err != nil {
			workspaceSchemaErr = fmt.Errorf("failed to add workspace schema resource: %w", err)
			return
		}

		workspaceSchema, workspaceSchemaErr = compiler.Compile("workspace.schema.json")
	})
	return workspaceSchema, workspaceSchemaErr
}

// ValidateWorkspace validates a workspace against the schema.
// Returns nil if valid, or an error describing the validation failure.
func ValidateWorkspace(workspace any) error {
	schema, err := WorkspaceSchema()
	if err != nil {
		return fmt.Errorf("failed to load workspace schema: %w", err)
	}

	// Normalize to map[string]any for validation
	normalized, err := NormalizeJSON(workspace)
	if err != nil {
		return fmt.Errorf("failed to normalize workspace: %w", err)
	}

	if err := schema.Validate(normalized); err != nil {
		return fmt.Errorf("workspace validation failed: %w", err)
	}

	return nil
}

// LoadWorkspace loads and validates a workspace from a file.
func LoadWorkspace(path string) (*Workspace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace file: %w", err)
	}

	// First validate against schema
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse workspace JSON: %w", err)
	}
	if err := ValidateWorkspace(raw); err != nil {
		return nil, err
	}

	// Then unmarshal into struct
	var ws Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace: %w", err)
	}

	return &ws, nil
}

// RequireActiveWorkspace loads the active workspace, returning user-facing errors
// via ExitResult if no environment or workspace is found. This eliminates the
// repeated FindEnvPath + LoadActiveWorkspace + error-wrapping boilerplate.
func RequireActiveWorkspace() (ws *Workspace, wsPath string, envPath string, err error) {
	envPath, err = FindEnvPath()
	if err != nil {
		return nil, "", "", exitText(1, "no environment found; run 'ob init' first", true)
	}
	ws, wsPath, err = LoadActiveWorkspace(envPath)
	if err != nil {
		return nil, "", "", exitText(1, err.Error(), true)
	}
	return ws, wsPath, envPath, nil
}

// WorkspaceDelegateContext contains delegate-related data from the active workspace.
// Used to pass workspace context to delegate resolution without loading the workspace
// multiple times or repeating the loading pattern.
type WorkspaceDelegateContext struct {
	DelegatePreferences map[string]string
	Delegates           []string
}

// GetWorkspaceDelegateContext loads the active workspace and extracts delegate context.
// Returns zero values if no workspace is active or if loading fails.
// Errors are intentionally ignored - caller gets empty context if workspace unavailable.
func GetWorkspaceDelegateContext() WorkspaceDelegateContext {
	ws, _, _, err := RequireActiveWorkspace()
	if err != nil {
		return WorkspaceDelegateContext{}
	}
	return WorkspaceDelegateContext{
		DelegatePreferences: ws.DelegatePreferences,
		Delegates:           ws.Delegates,
	}
}
