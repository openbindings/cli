package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputAddCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "add --target <target-id> --op <opKey> <name> <ref>",
		Short: "Add a named input reference",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsAdd(targetID, opKey, args[0], args[1], force)
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite if name already exists")
	return c
}
