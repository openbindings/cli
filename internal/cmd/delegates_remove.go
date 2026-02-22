package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newDelegateRemoveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "remove <url>",
		Aliases: []string{"rm"},
		Short:   "Remove a delegate from the workspace",
		Long: `Remove a delegate URL from the active workspace.

Examples:
  ob delegate remove exec:my-cli
  ob delegate rm https://api.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, outputPath := getOutputFlags(cmd)
			return app.DelegateRemove(app.DelegateRemoveParams{
				URL:          args[0],
				OutputFormat: format,
				OutputPath:   outputPath,
			})
		},
	}
	return c
}
