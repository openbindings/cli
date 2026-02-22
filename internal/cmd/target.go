package cmd

import "github.com/spf13/cobra"

func newTargetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "target",
		Aliases: []string{"t", "targets"},
		Short:   "Manage targets in a workspace",
	}

	c.PersistentFlags().StringP("workspace", "w", "", "workspace to operate on (default: active)")

	c.AddCommand(
		newTargetAddCmd(),
		newTargetListCmd(),
		newTargetRemoveCmd(),
		newTargetUpdateCmd(),
		newTargetCopyCmd(),
	)

	return c
}
