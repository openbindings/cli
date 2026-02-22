package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newDelegateResolveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "resolve <format>",
		Short: "Show which delegate handles a format",
		Long: `Show which delegate would handle a given binding format based on preferences and available delegates.

Resolves against binding format handler delegates.

Examples:
  ob delegate resolve usage@2.0.0
  ob delegate resolve openapi@3.1.0 -o result.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, outputPath := getOutputFlags(cmd)
			return app.DelegateResolve(app.DelegateResolveParams{
				Format:       args[0],
				OutputFormat: format,
				OutputPath:   outputPath,
			})
		},
	}
	return c
}
