package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newTargetRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <target-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a target from a workspace",
		Long: `Remove a target from a workspace.

Examples:
  ob target remove abc12345
  ob target remove abc12345 -F json
  ob target remove abc12345 -w staging

Use 'ob target list' to see target IDs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]

			wsName, _ := cmd.Flags().GetString("workspace")
			ctx, err := getWorkspaceContextFor(wsName)
			if err != nil {
				return err
			}

			target := app.FindTargetByID(ctx.Workspace, targetID)
			if target == nil {
				return app.ExitResult{Code: 1, Message: fmt.Sprintf("target %q not found in workspace %q", targetID, ctx.Active), ToStderr: true}
			}
			removedTarget := *target

			if !app.RemoveTargetByID(ctx.Workspace, targetID) {
				return app.ExitResult{Code: 1, Message: "failed to remove target", ToStderr: true}
			}

			if err := app.SaveWorkspace(ctx.EnvPath, ctx.Workspace); err != nil {
				return err
			}

			result := struct {
				Removed   string `json:"removed"`
				TargetID  string `json:"targetId"`
				URL       string `json:"url"`
				Label     string `json:"label,omitempty"`
				Workspace string `json:"workspace"`
			}{
				Removed:   "target",
				TargetID:  removedTarget.ID,
				URL:       removedTarget.URL,
				Label:     removedTarget.Label,
				Workspace: ctx.Active,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				if removedTarget.Label != "" {
					return fmt.Sprintf("Removed target %q (%s)", removedTarget.Label, removedTarget.URL)
				}
				return fmt.Sprintf("Removed target %s", removedTarget.URL)
			})
		},
	}

	return cmd
}
