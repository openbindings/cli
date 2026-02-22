package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newTargetListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List targets in a workspace",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wsName, _ := cmd.Flags().GetString("workspace")
			ctx, err := getWorkspaceContextFor(wsName)
			if err != nil {
				return err
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(ctx.Workspace.Targets, format, outputPath, func() string {
				if len(ctx.Workspace.Targets) == 0 {
					return fmt.Sprintf("No targets in workspace %q\n\nAdd a target with: ob target add <url>", ctx.Active)
				}

				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Targets in workspace %q:\n", ctx.Active))
				for i, t := range ctx.Workspace.Targets {
					if t.Label != "" {
						sb.WriteString(fmt.Sprintf("\n  %d. %s (%s)", i+1, t.Label, t.URL))
					} else {
						sb.WriteString(fmt.Sprintf("\n  %d. %s", i+1, t.URL))
					}
					sb.WriteString(fmt.Sprintf("\n     ID: %s", t.ID))
				}
				return sb.String()
			})
		},
	}

	return cmd
}
