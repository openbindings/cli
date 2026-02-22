package cmd

import "github.com/spf13/cobra"

func newInputCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "input",
		Aliases: []string{"inputs"},
		Short:   "Manage named input references",
	}

	c.PersistentFlags().String("target", "", "target ID (required)")
	c.PersistentFlags().String("op", "", "operation key (required)")
	_ = c.MarkPersistentFlagRequired("target")
	_ = c.MarkPersistentFlagRequired("op")

	c.AddCommand(
		newInputAddCmd(),
		newInputListCmd(),
		newInputShowCmd(),
		newInputRemoveCmd(),
		newInputCreateCmd(),
		newInputEditCmd(),
		newInputValidateCmd(),
	)

	return c
}
