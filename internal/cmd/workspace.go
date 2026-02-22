package cmd

import "github.com/spf13/cobra"

func newWorkspaceCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws", "workspaces"},
		Short:   "Manage workspaces",
	}

	c.AddCommand(
		newWorkspaceListCmd(),
		newWorkspaceCreateCmd(),
		newWorkspaceUseCmd(),
		newWorkspaceShowCmd(),
		newWorkspaceDeleteCmd(),
		newWorkspaceRebaseCmd(),
		newWorkspaceRenameCmd(),
		newWorkspaceCopyCmd(),
	)

	return c
}
