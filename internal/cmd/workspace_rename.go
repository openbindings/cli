package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rename <old-name> <new-name>",
		Aliases: []string{"mv"},
		Short:   "Rename a workspace",
		Long: `Rename a workspace and update all references to it.

This updates:
- The workspace file name
- The 'name' field in the workspace
- Any workspaces that use this as their base
- The active workspace if it's being renamed

Examples:
  ob workspace rename staging production
  ob ws rename old-name new-name`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			if err := app.RenameWorkspace(envPath, oldName, newName); err != nil {
				return err
			}

			path := app.WorkspacePath(envPath, newName)

			result := struct {
				Renamed  string `json:"renamed"`
				OldName  string `json:"oldName"`
				NewName  string `json:"newName"`
				FilePath string `json:"filePath"`
			}{
				Renamed:  "workspace",
				OldName:  oldName,
				NewName:  newName,
				FilePath: path,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				return fmt.Sprintf("Renamed workspace %q to %q\nFile: %s", oldName, newName, path)
			})
		},
	}

	return cmd
}
