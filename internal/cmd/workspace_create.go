package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceCreateCmd() *cobra.Command {
	var baseName string
	var standalone bool

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new workspace",
		Long: `Create a new workspace.

By default, the new workspace extends the currently active workspace.
Use --base to specify a different base, or --standalone for no base.

Examples:
  ob workspace create dev
  ob workspace create staging --base production
  ob workspace create scratch --standalone`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			// Determine base
			if standalone {
				baseName = ""
			} else if baseName == "" {
				// Default to active workspace
				baseName, err = app.GetActiveWorkspace(envPath)
				if err != nil {
					return err
				}
			}

			if err := app.CreateWorkspace(envPath, name, baseName); err != nil {
				return err
			}

			path := app.WorkspacePath(envPath, name)

			result := struct {
				Created    string `json:"created"`
				Workspace  string `json:"workspace"`
				Base       string `json:"base,omitempty"`
				Standalone bool   `json:"standalone,omitempty"`
				FilePath   string `json:"filePath"`
			}{
				Created:    "workspace",
				Workspace:  name,
				Standalone: baseName == "",
				FilePath:   path,
			}
			if baseName != "" {
				result.Base = baseName
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				var sb strings.Builder
				if baseName != "" {
					sb.WriteString(fmt.Sprintf("Created workspace %q (base: %s)\n", name, baseName))
				} else {
					sb.WriteString(fmt.Sprintf("Created workspace %q (standalone)\n", name))
				}
				sb.WriteString(fmt.Sprintf("File: %s", path))
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&baseName, "base", "", "Base workspace to extend")
	cmd.Flags().BoolVar(&standalone, "standalone", false, "Create without a base workspace")

	return cmd
}
