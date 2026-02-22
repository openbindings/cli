package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [obi-path]",
		Short: "Show environment status or OBI sync report",
		Long: `Show current environment and workspace status.

If an OBI file path is provided, shows a per-source sync report with
managed vs hand-authored breakdowns. Without arguments, shows workspace
environment info.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, outputPath := getOutputFlags(cmd)

			// If an OBI path is provided, show the OBI sync report.
			if len(args) == 1 {
				result, err := app.OBIStatus(app.OBIStatusInput{OBIPath: args[0]})
				if err != nil {
					return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
				}
				return app.OutputResult(result, format, outputPath)
			}

			// No args: environment status.
			envPath, isLocal, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			active, _ := app.GetActiveWorkspace(envPath)
			workspaces, _ := app.ListWorkspaces(envPath)
			broken := app.ValidateAllWorkspaceReferences(envPath)

			status := struct {
				EnvironmentType string   `json:"environmentType"`
				EnvironmentPath string   `json:"environmentPath"`
				ActiveWorkspace string   `json:"activeWorkspace"`
				WorkspaceCount  int      `json:"workspaceCount"`
				BrokenCount     int      `json:"brokenCount,omitempty"`
				BrokenBases     []string `json:"brokenBases,omitempty"`
			}{
				EnvironmentPath: envPath,
				ActiveWorkspace: active,
				WorkspaceCount:  len(workspaces),
				BrokenCount:     len(broken),
			}

			if isLocal {
				status.EnvironmentType = "local"
			} else {
				status.EnvironmentType = "global"
			}

			for name := range broken {
				status.BrokenBases = append(status.BrokenBases, name)
			}
			sort.Strings(status.BrokenBases)

			return app.OutputResultText(status, format, outputPath, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Environment: %s (%s)\n", status.EnvironmentType, status.EnvironmentPath))
				sb.WriteString(fmt.Sprintf("Active workspace: %s\n", status.ActiveWorkspace))
				sb.WriteString(fmt.Sprintf("Workspaces: %d", status.WorkspaceCount))

				if len(broken) > 0 {
					sb.WriteString(fmt.Sprintf("\nWarnings: %d workspace(s) with broken base references", len(broken)))
					for _, name := range status.BrokenBases {
						sb.WriteString(fmt.Sprintf("\n  - %s", name))
					}
				}
				return sb.String()
			})
		},
	}
	return cmd
}
