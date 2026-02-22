package cmd

import (
	"fmt"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy <source> <destination>",
		Aliases: []string{"cp"},
		Short:   "Copy a workspace",
		Long: `Create a copy of a workspace with a new name.

The copy is independent - it's a snapshot, not linked to the original.
Use 'create --base' if you want inheritance instead.

Examples:
  ob workspace copy dev dev-backup
  ob ws cp production production-snapshot`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcName := args[0]
			dstName := args[1]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			if err := app.CopyWorkspace(envPath, srcName, dstName); err != nil {
				return err
			}

			path := app.WorkspacePath(envPath, dstName)

			result := struct {
				Copied   string `json:"copied"`
				Source   string `json:"source"`
				Dest     string `json:"destination"`
				FilePath string `json:"filePath"`
			}{
				Copied:   "workspace",
				Source:   srcName,
				Dest:     dstName,
				FilePath: path,
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				return fmt.Sprintf("Copied workspace %q to %q\nFile: %s", srcName, dstName, path)
			})
		},
	}

	return cmd
}
