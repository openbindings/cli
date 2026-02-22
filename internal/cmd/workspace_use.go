package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch to a workspace",
		Long: `Switch to a workspace.

Examples:
  ob workspace use dev
  ob ws use production`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			if err := app.UseWorkspace(envPath, name); err != nil {
				return err
			}

			// Check for broken base and warn
			info, err := app.GetWorkspaceInfo(envPath, name)
			baseBroken := err == nil && info.BaseBroken
			baseName := ""
			if info != nil {
				baseName = info.Base
			}

			result := struct {
				Switched   string `json:"switched"`
				Workspace  string `json:"workspace"`
				BaseBroken bool   `json:"baseBroken,omitempty"`
				BrokenBase string `json:"brokenBase,omitempty"`
			}{
				Switched:   "workspace",
				Workspace:  name,
				BaseBroken: baseBroken,
			}
			if baseBroken {
				result.BrokenBase = baseName
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(result, format, outputPath, func() string {
				var sb strings.Builder
				if baseBroken {
					sb.WriteString(fmt.Sprintf("Warning: base workspace %q does not exist; %q will be treated as standalone\n", baseName, name))
				}
				sb.WriteString(fmt.Sprintf("Switched to workspace %q", name))
				return sb.String()
			})
		},
	}

	return cmd
}
