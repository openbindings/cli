package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputRemoveCmd() *cobra.Command {
	var (
		del   bool
		force bool
	)
	c := &cobra.Command{
		Use:     "remove --target <target-id> --op <opKey> <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a named input reference",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if force && !del {
				return app.ExitResult{Code: 2, Message: "-f/--force requires -d/--delete-file", ToStderr: true}
			}
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsRemove(targetID, opKey, args[0], del, force)
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	c.Flags().BoolVarP(&del, "delete-file", "d", false, "also delete the referenced local file")
	c.Flags().BoolVarP(&force, "force", "f", false, "force deletion even if referenced elsewhere (requires -d)")
	return c
}
