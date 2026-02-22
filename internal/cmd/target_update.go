package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newTargetUpdateCmd() *cobra.Command {
	var newLabel string
	var newURL string
	cmd := &cobra.Command{
		Use:   "update <target-id>",
		Short: "Update a target's label or URL",
		Long: `Update a target's label or URL.

Examples:
  ob target update abc123 --label "New Label"
  ob target update abc123 --url "exec:new-cli"
  ob target update abc123 --label "" # clear label
  ob target update abc123 --label "API" --url "https://api.example.com"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]

			// Check if any update flags were provided
			labelSet := cmd.Flags().Changed("label")
			urlSet := cmd.Flags().Changed("url")

			if !labelSet && !urlSet {
				return app.ExitResult{Code: 2, Message: "specify --label and/or --url to update", ToStderr: true}
			}

			wsName, _ := cmd.Flags().GetString("workspace")
			ctx, err := getWorkspaceContextFor(wsName)
			if err != nil {
				return err
			}

			target := app.FindTargetByID(ctx.Workspace, targetID)
			if target == nil {
				return app.ExitResult{Code: 1, Message: fmt.Sprintf("target %q not found in workspace %q", targetID, ctx.Active), ToStderr: true}
			}

			// Apply updates
			if labelSet {
				target.Label = newLabel
			}
			if urlSet {
				target.URL = newURL
			}

			if err := app.SaveWorkspace(ctx.EnvPath, ctx.Workspace); err != nil {
				return err
			}

			result := struct {
				Updated   string `json:"updated"`
				TargetID  string `json:"targetId"`
				URL       string `json:"url"`
				Label     string `json:"label,omitempty"`
				Workspace string `json:"workspace"`
			}{
				Updated:   "target",
				TargetID:  target.ID,
				URL:       target.URL,
				Label:     target.Label,
				Workspace: ctx.Active,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Updated target %s\n", target.ID))
				if target.Label != "" {
					sb.WriteString(fmt.Sprintf("  Label: %s\n", target.Label))
				}
				sb.WriteString(fmt.Sprintf("  URL: %s", target.URL))
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVarP(&newLabel, "label", "l", "", "New label for the target")
	cmd.Flags().StringVarP(&newURL, "url", "u", "", "New URL for the target")
	return cmd
}
