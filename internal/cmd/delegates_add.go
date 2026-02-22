package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newDelegateAddCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a delegate to the workspace",
		Long: `Register a delegate URL in the active workspace.

ob probes delegates to discover which interface contracts they implement.

Examples:
  ob delegate add exec:my-cli
  ob delegate add https://api.example.com
  ob delegate add exec:./local-tool -o result.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, outputPath := getOutputFlags(cmd)
			return app.DelegateAdd(app.DelegateAddParams{
				URL:          args[0],
				OutputFormat: format,
				OutputPath:   outputPath,
			})
		},
	}
	return c
}
