package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit --target <target-id> --op <opKey> <name>",
		Short: "Open the input file in your editor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			return app.InputsEdit(targetID, opKey, args[0])
		},
	}
	return c
}
