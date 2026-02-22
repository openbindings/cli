package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvConfig represents environment-level configuration stored in .openbindings/config.json.
type EnvConfig struct {
	ActiveWorkspace string `json:"activeWorkspace"`
}

// InitParams configures the init command.
type InitParams struct {
	OutputFormat string // json|yaml|text (from --format)
	OutputPath   string // when set, write result to this file (from -o)
	Global       bool
}

// Init creates an OpenBindings environment directory with a default workspace.
// If Global is true, initializes at ~/.config/openbindings/ instead of .openbindings/.
func Init(params InitParams) error {
	envDir := EnvDir
	description := "Default workspace"

	if params.Global {
		globalPath, err := GlobalConfigPath()
		if err != nil {
			return err
		}
		envDir = globalPath
		description = "Default global workspace"
	}

	if _, err := os.Stat(envDir); err == nil {
		return ExitResult{Code: 1, Message: envDir + " already exists", ToStderr: true}
	}

	// Create environment directory
	if err := os.MkdirAll(envDir, DirPerm); err != nil {
		return err
	}

	// Create workspaces/ directory
	workspacesPath := filepath.Join(envDir, WorkspacesDir)
	if err := os.MkdirAll(workspacesPath, DirPerm); err != nil {
		return err
	}

	// Create default workspace with exec:ob as the initial target
	defaultWorkspace := newDefaultWorkspace(description)

	workspaceData, err := json.MarshalIndent(defaultWorkspace, "", "  ")
	if err != nil {
		return err
	}

	defaultPath := filepath.Join(workspacesPath, DefaultWorkspaceName+".json")
	if err := AtomicWriteFile(defaultPath, append(workspaceData, '\n'), FilePerm); err != nil {
		return err
	}

	// Create environment config with default as active workspace
	envConfig := EnvConfig{
		ActiveWorkspace: DefaultWorkspaceName,
	}
	configData, err := json.MarshalIndent(envConfig, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(envDir, EnvConfigFile)
	if err := AtomicWriteFile(configPath, append(configData, '\n'), FilePerm); err != nil {
		return err
	}

	// Get absolute path for display
	absPath, err := filepath.Abs(envDir)
	if err != nil {
		absPath = envDir
	}

	result := struct {
		Initialized     string `json:"initialized"`
		EnvironmentPath string `json:"environmentPath"`
		WorkspacePath   string `json:"workspacePath"`
		Workspace       string `json:"workspace"`
		Global          bool   `json:"global,omitempty"`
	}{
		Initialized:     "environment",
		EnvironmentPath: absPath,
		WorkspacePath:   defaultPath,
		Workspace:       DefaultWorkspaceName,
		Global:          params.Global,
	}

	format, err := ParseOutputFormat(params.OutputFormat)
	if err != nil {
		return err
	}

	// Write to file when -o is set.
	if params.OutputPath != "" {
		outFormat := OutputFormatJSON
		if strings.HasSuffix(strings.ToLower(params.OutputPath), ".yaml") || strings.HasSuffix(strings.ToLower(params.OutputPath), ".yml") {
			outFormat = OutputFormatYAML
		}
		b, err := FormatOutput(result, outFormat)
		if err != nil {
			return err
		}
		if err := AtomicWriteFile(params.OutputPath, b, FilePerm); err != nil {
			return err
		}
		return ExitResult{Code: 0, Message: "Wrote " + params.OutputPath, ToStderr: false}
	}

	if format != OutputFormatText {
		out, err := FormatOutputString(format, result)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}

	return okText("Initialized " + absPath + "/\n\nWorkspace: " + defaultPath)
}

// newDefaultWorkspace creates a default workspace with exec:ob as the initial target.
func newDefaultWorkspace(description string) Workspace {
	return Workspace{
		Version:     WorkspaceFormatVersion,
		Name:        DefaultWorkspaceName,
		Description: description,
		Targets: []WorkspaceTarget{
			{
				ID:    GenerateTargetID(),
				URL:   DefaultTargetURL,
				Label: "ob",
			},
		},
		Settings: WorkspaceSettings{},
		Delegates:  []string{},
	}
}

// FindEnvironment walks up from the current directory looking for .openbindings/.
// Returns the path to the .openbindings/ directory if found, or the global ~/.config/openbindings/ path.
// Also returns a boolean indicating whether a local environment was found.
func FindEnvironment() (string, bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	// Resolve symlinks for consistent path comparison
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", false, err
	}

	// Walk up looking for .openbindings/
	dir := cwd
	for {
		envPath := filepath.Join(dir, EnvDir)
		if info, err := os.Stat(envPath); err == nil && info.IsDir() {
			return envPath, true, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, use global
			break
		}
		dir = parent
	}

	// Fall back to global config directory.
	globalPath, err := GlobalConfigPath()
	if err != nil {
		return "", false, err
	}

	return globalPath, false, nil
}

// LoadEnvConfig loads the environment configuration from config.json.
func LoadEnvConfig(envPath string) (*EnvConfig, error) {
	configPath := filepath.Join(envPath, EnvConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if no config exists
			return &EnvConfig{ActiveWorkspace: DefaultWorkspaceName}, nil
		}
		return nil, err
	}

	var config EnvConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveEnvConfig saves the environment configuration to config.json atomically.
func SaveEnvConfig(envPath string, config *EnvConfig) error {
	configPath := filepath.Join(envPath, EnvConfigFile)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return AtomicWriteFile(configPath, data, FilePerm)
}

// GetActiveWorkspace returns the name of the currently active workspace.
// If the active workspace no longer exists, falls back to default or first available.
func GetActiveWorkspace(envPath string) (string, error) {
	config, err := LoadEnvConfig(envPath)
	if err != nil {
		return "", err
	}

	active := config.ActiveWorkspace
	if active == "" {
		active = DefaultWorkspaceName
	}

	// Verify active workspace still exists
	if WorkspaceExists(envPath, active) {
		return active, nil
	}

	// Active workspace is missing - try to recover
	// First try default
	if active != DefaultWorkspaceName && WorkspaceExists(envPath, DefaultWorkspaceName) {
		// Auto-fix the config
		config.ActiveWorkspace = DefaultWorkspaceName
		_ = SaveEnvConfig(envPath, config) // best effort
		return DefaultWorkspaceName, nil
	}

	// Fall back to first available workspace
	names, err := ListWorkspaces(envPath)
	if err != nil || len(names) == 0 {
		return DefaultWorkspaceName, nil // will fail later when trying to use it
	}

	// Auto-fix the config
	config.ActiveWorkspace = names[0]
	_ = SaveEnvConfig(envPath, config) // best effort
	return names[0], nil
}

// SetActiveWorkspace sets the active workspace name.
func SetActiveWorkspace(envPath, name string) error {
	config, err := LoadEnvConfig(envPath)
	if err != nil {
		return err
	}
	config.ActiveWorkspace = name
	return SaveEnvConfig(envPath, config)
}

// WorkspacePath returns the path to a workspace file.
func WorkspacePath(envPath, name string) string {
	return filepath.Join(envPath, WorkspacesDir, name+".json")
}

// ListWorkspaces returns the names of all workspaces in an environment.
func ListWorkspaces(envPath string) ([]string, error) {
	workspacesPath := filepath.Join(envPath, WorkspacesDir)
	entries, err := os.ReadDir(workspacesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			names = append(names, name[:len(name)-5]) // strip .json
		}
	}
	return names, nil
}

// WorkspaceExists checks if a workspace exists.
func WorkspaceExists(envPath, name string) bool {
	path := WorkspacePath(envPath, name)
	_, err := os.Stat(path)
	return err == nil
}

// FindEnvPath finds the environment path, returning an error if none exists.
// Unlike FindEnvironment, this returns an error if no environment is found.
func FindEnvPath() (string, error) {
	envPath, _, err := FindEnvironment()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(envPath); err != nil {
		return "", os.ErrNotExist
	}

	return envPath, nil
}

// LoadActiveWorkspace loads the currently active workspace.
// Returns the workspace, its file path, and any error.
func LoadActiveWorkspace(envPath string) (*Workspace, string, error) {
	activeName, err := GetActiveWorkspace(envPath)
	if err != nil {
		return nil, "", err
	}

	wsPath := WorkspacePath(envPath, activeName)
	ws, err := LoadWorkspace(wsPath)
	if err != nil {
		return nil, "", err
	}

	return ws, wsPath, nil
}

// EnsureGlobalEnvironment creates the global openbindings environment if it doesn't exist.
// Returns the workspace, its file path, and any error.
// This is used by the TUI to auto-create a workspace when needed.
func EnsureGlobalEnvironment() (*Workspace, string, error) {
	globalEnvPath, err := GlobalConfigPath()
	if err != nil {
		return nil, "", err
	}

	// Check if global environment already exists
	if _, err := os.Stat(globalEnvPath); err == nil {
		// Already exists, just load it
		return LoadActiveWorkspace(globalEnvPath)
	}

	// Create global environment
	if err := os.MkdirAll(globalEnvPath, DirPerm); err != nil {
		return nil, "", err
	}

	// Create workspaces directory
	workspacesPath := filepath.Join(globalEnvPath, WorkspacesDir)
	if err := os.MkdirAll(workspacesPath, DirPerm); err != nil {
		return nil, "", err
	}

	// Create default workspace with exec:ob as the initial target
	defaultWorkspace := newDefaultWorkspace("Default global workspace")

	workspaceData, err := json.MarshalIndent(defaultWorkspace, "", "  ")
	if err != nil {
		return nil, "", err
	}

	wsPath := filepath.Join(workspacesPath, DefaultWorkspaceName+".json")
	if err := AtomicWriteFile(wsPath, append(workspaceData, '\n'), FilePerm); err != nil {
		return nil, "", err
	}

	// Create environment config with default as active workspace
	envConfig := EnvConfig{
		ActiveWorkspace: DefaultWorkspaceName,
	}
	configData, err := json.MarshalIndent(envConfig, "", "  ")
	if err != nil {
		return nil, "", err
	}

	configPath := filepath.Join(globalEnvPath, EnvConfigFile)
	if err := AtomicWriteFile(configPath, append(configData, '\n'), FilePerm); err != nil {
		return nil, "", err
	}

	return &defaultWorkspace, wsPath, nil
}
