package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all workspaces",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			infos, err := app.ListWorkspaceInfos(envPath)
			if err != nil {
				return err
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(infos, format, outputPath, func() string {
				if len(infos) == 0 {
					return "No workspaces found. Run 'ob init' to create one."
				}

				var sb strings.Builder
				workspacesDir := envPath + "/workspaces/"
				sb.WriteString(fmt.Sprintf("Workspaces in %s\n", workspacesDir))

				for _, info := range infos {
					marker := "  "
					if info.IsActive {
						marker = "* "
					}

					line := marker + info.Name
					if info.Description != "" {
						line += "  " + info.Description
					}
					sb.WriteString("\n" + line)

					var details []string
					if info.Base != "" {
						if info.BaseBroken {
							details = append(details, fmt.Sprintf("base: %s [BROKEN]", info.Base))
						} else {
							details = append(details, fmt.Sprintf("base: %s", info.Base))
						}
					}
					details = append(details, fmt.Sprintf("%d targets", info.TargetCount))
					sb.WriteString(fmt.Sprintf("\n      %s", strings.Join(details, ", ")))
				}
				return sb.String()
			})
		},
	}


	return cmd
}
