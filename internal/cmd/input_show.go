package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputShowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "show --target <target-id> --op <opKey> <name>",
		Short: "Show a named input reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsShow(targetID, opKey, args[0])
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	return c
}
