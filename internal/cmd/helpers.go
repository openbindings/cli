package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

// getOutputFlags returns the global --format and -o/--output (path) from the root command.
// -o/--output = output path (file to write). --format/-F = output format (json|yaml|text|quiet).
func getOutputFlags(c *cobra.Command) (format string, outputPath string) {
	format, _ = c.Root().PersistentFlags().GetString("format")
	outputPath, _ = c.Root().PersistentFlags().GetString("output")
	return format, outputPath
}

// workspaceContext holds the common context needed by workspace-related commands.
type workspaceContext struct {
	EnvPath   string
	Active    string
	Workspace *app.Workspace
}

// getInputFlags returns the --target and --op persistent flags from the input parent.
func getInputFlags(c *cobra.Command) (targetID, opKey string) {
	targetID, _ = c.Flags().GetString("target")
	opKey, _ = c.Flags().GetString("op")
	return targetID, opKey
}

// validateSourceModeArgs validates the mutual exclusion between positional OBI args
// and --from-sources / --only flags used by diff and merge commands.
func validateSourceModeArgs(args []string, fromSources bool, onlySource string) error {
	if len(args) == 2 && fromSources {
		return app.ExitResult{
			Code:     2,
			Message:  "cannot use both a positional OBI argument and --from-sources",
			ToStderr: true,
		}
	}
	if len(args) < 2 && !fromSources {
		return app.ExitResult{
			Code:     2,
			Message:  "either provide two OBI arguments or use --from-sources",
			ToStderr: true,
		}
	}
	if onlySource != "" && !fromSources {
		return app.ExitResult{
			Code:     2,
			Message:  "--only requires --from-sources",
			ToStderr: true,
		}
	}
	return nil
}

// getWorkspaceContextFor loads a specific workspace by name, or the active
// workspace if name is empty. Used by target commands that support --workspace.
func getWorkspaceContextFor(name string) (*workspaceContext, error) {
	envPath, _, err := app.FindEnvironment()
	if err != nil {
		return nil, err
	}

	if name == "" {
		name, err = app.GetActiveWorkspace(envPath)
		if err != nil {
			return nil, err
		}
	} else {
		if !app.WorkspaceExists(envPath, name) {
			return nil, fmt.Errorf("workspace %q not found", name)
		}
	}

	wsPath := app.WorkspacePath(envPath, name)
	ws, err := app.LoadWorkspace(wsPath)
	if err != nil {
		return nil, err
	}

	return &workspaceContext{
		EnvPath:   envPath,
		Active:    name,
		Workspace: ws,
	}, nil
}

