package cmd

import (
	"github.com/openbindings/cli/internal/tui"
	"github.com/spf13/cobra"
)

func newBrowseCmd() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Browse/discover services (TUI)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunBrowse(workspace)
		},
	}

	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "workspace to browse (default: active)")

	return cmd
}
