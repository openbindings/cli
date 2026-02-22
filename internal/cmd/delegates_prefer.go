package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newDelegatePreferCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "prefer <format> <delegate-url>",
		Short: "Set preferred delegate for a format",
		Long: `Set which delegate should be used to handle a specific binding format.

This applies to binding format handler delegates.

Examples:
  ob delegate prefer usage@2.0.0 exec:my-cli
  ob delegate prefer openapi@3.1.0 https://api.example.com`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, outputPath := getOutputFlags(cmd)
			return app.DelegatePrefer(app.DelegatePreferParams{
				Format:       args[0],
				Delegate:     args[1],
				OutputFormat: format,
				OutputPath:   outputPath,
			})
		},
	}
	return c
}
