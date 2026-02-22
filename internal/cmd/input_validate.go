package cmd

import (
	"time"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputValidateCmd() *cobra.Command {
	var timeout time.Duration
	c := &cobra.Command{
		Use:   "validate --target <target-id> --op <opKey> <name>",
		Short: "Validate an input against the operation schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsValidate(targetID, opKey, args[0], timeout)
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	c.Flags().DurationVar(&timeout, "timeout", 2*time.Second, "timeout for fetching the target OBI")
	return c
}
