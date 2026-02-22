package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceRebaseCmd() *cobra.Command {
	var standalone bool
	cmd := &cobra.Command{
		Use:   "rebase <name> [new-base]",
		Short: "Change the base of a workspace",
		Long: `Change which workspace a workspace extends.

Examples:
  ob workspace rebase staging prod-base   # staging now extends prod-base
  ob workspace rebase staging --standalone # staging becomes standalone`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			var newBase string
			if standalone {
				newBase = ""
			} else if len(args) > 1 {
				newBase = args[1]
			} else {
				return app.ExitResult{Code: 2, Message: "specify new base or use --standalone", ToStderr: true}
			}

			if err := app.RebaseWorkspace(envPath, name, newBase); err != nil {
				return err
			}

			result := struct {
				Rebased    string `json:"rebased"`
				Workspace  string `json:"workspace"`
				NewBase    string `json:"newBase,omitempty"`
				Standalone bool   `json:"standalone,omitempty"`
			}{
				Rebased:    "workspace",
				Workspace:  name,
				Standalone: newBase == "",
			}
			if newBase != "" {
				result.NewBase = newBase
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				if newBase != "" {
					return fmt.Sprintf("Workspace %q now extends %q", name, newBase)
				}
				return fmt.Sprintf("Workspace %q is now standalone", name)
			})
		},
	}

	cmd.Flags().BoolVar(&standalone, "standalone", false, "Remove base (make standalone)")

	return cmd
}
