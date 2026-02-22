package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newTargetCopyCmd() *cobra.Command {
	var fromWorkspace string
	cmd := &cobra.Command{
		Use:   "copy --from <workspace> <target-id>",
		Short: "Copy a target from another workspace into the active workspace",
		Long: `Copy a target from another workspace into the active workspace.

The target (and its inputs) are copied with a new ID. The source workspace
is not modified.

Examples:
  ob target copy --from dev abc12345
  ob target copy --from production 1a2b3c4d
  ob target copy --from staging abc123 -F json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID := args[0]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			activeName, err := app.GetActiveWorkspace(envPath)
			if err != nil {
				return err
			}

			newTarget, err := app.CopyTargetFromWorkspace(envPath, fromWorkspace, targetID)
			if err != nil {
				return err
			}

			result := struct {
				Copied        string `json:"copied"`
				NewTargetID   string `json:"newTargetId"`
				OldTargetID   string `json:"oldTargetId"`
				URL           string `json:"url"`
				Label         string `json:"label,omitempty"`
				FromWorkspace string `json:"fromWorkspace"`
				ToWorkspace   string `json:"toWorkspace"`
			}{
				Copied:        "target",
				NewTargetID:   newTarget.ID,
				OldTargetID:   targetID,
				URL:           newTarget.URL,
				Label:         newTarget.Label,
				FromWorkspace: fromWorkspace,
				ToWorkspace:   activeName,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				var sb strings.Builder
				if newTarget.Label != "" {
					sb.WriteString(fmt.Sprintf("Copied target %q (%s) to workspace %q\n", newTarget.Label, newTarget.URL, activeName))
				} else {
					sb.WriteString(fmt.Sprintf("Copied target %s to workspace %q\n", newTarget.URL, activeName))
				}
				sb.WriteString(fmt.Sprintf("New ID: %s", newTarget.ID))
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&fromWorkspace, "from", "", "source workspace to copy from (required)")
	_ = cmd.MarkFlagRequired("from")

	return cmd
}
