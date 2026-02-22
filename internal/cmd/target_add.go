package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newTargetAddCmd() *cobra.Command {
	var label string

	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a target to a workspace",
		Long: `Add a target to a workspace.

Targets are OpenBindings endpoints to explore.

Examples:
  ob target add exec:my-cli
  ob target add https://api.example.com
  ob target add exec:my-cli --label "My CLI Tool"
  ob target add exec:ob -F json
  ob target add exec:foo -w staging

Duplicate URLs are allowed (like browser tabs).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]

			wsName, _ := cmd.Flags().GetString("workspace")
			ctx, err := getWorkspaceContextFor(wsName)
			if err != nil {
				return err
			}

			target := app.AddTarget(ctx.Workspace, url, label)

			if err := app.SaveWorkspace(ctx.EnvPath, ctx.Workspace); err != nil {
				return err
			}

			result := struct {
				Added     string `json:"added"`
				TargetID  string `json:"targetId"`
				URL       string `json:"url"`
				Label     string `json:"label,omitempty"`
				Workspace string `json:"workspace"`
			}{
				Added:     "target",
				TargetID:  target.ID,
				URL:       target.URL,
				Label:     target.Label,
				Workspace: ctx.Active,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				var sb strings.Builder
				if label != "" {
					sb.WriteString(fmt.Sprintf("Added target %q (%s) to workspace %q\n", label, url, ctx.Active))
				} else {
					sb.WriteString(fmt.Sprintf("Added target %s to workspace %q\n", url, ctx.Active))
				}
				sb.WriteString(fmt.Sprintf("ID: %s", target.ID))
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVarP(&label, "label", "l", "", "Display label for the target")

	return cmd
}
