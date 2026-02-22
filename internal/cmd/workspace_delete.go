package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a workspace",
		Long: `Delete a workspace.

Examples:
  ob workspace delete old-workspace
  ob workspace delete staging --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			// Check if any other workspaces depend on this one
			if !force {
				infos, err := app.ListWorkspaceInfos(envPath)
				if err != nil {
					return err
				}

				var dependents []string
				for _, info := range infos {
					if info.Base == name {
						dependents = append(dependents, info.Name)
					}
				}

				if len(dependents) > 0 {
					return app.ExitResult{Code: 1, Message: fmt.Sprintf("workspace %q is used as base by: %v\nUse --force to delete anyway", name, dependents), ToStderr: true}
				}
			}

			if err := app.DeleteWorkspace(envPath, name); err != nil {
				return err
			}

			result := struct {
				Deleted   string `json:"deleted"`
				Workspace string `json:"workspace"`
			}{
				Deleted:   "workspace",
				Workspace: name,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				return fmt.Sprintf("Deleted workspace %q", name)
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Delete even if other workspaces depend on it")

	return cmd
}
