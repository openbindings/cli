package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "list --target <target-id> --op <opKey>",
		Aliases: []string{"ls"},
		Short:   "List named inputs for an operation",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsList(targetID, opKey)
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	return c
}
