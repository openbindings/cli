package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newDelegateListCmd() *cobra.Command {
	var includeBuiltin bool
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List delegates",
		Long:    "List delegates registered in the active workspace with their supported formats.\nA ✓ indicates the delegate is preferred for that format.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := app.BuildDelegateListOutput(app.DelegateListParams{
				IncludeBuiltin: includeBuiltin,
			})
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(output, format, outputPath)
		},
	}
	c.Flags().BoolVar(&includeBuiltin, "builtin", true, "include the builtin delegate")
	return c
}
