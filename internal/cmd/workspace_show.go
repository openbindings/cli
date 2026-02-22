package cmd

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newWorkspaceShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show workspace details",
		Long:  "Show details of a workspace. Defaults to the active workspace.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envPath, _, err := app.FindEnvironment()
			if err != nil {
				return err
			}

			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				name, err = app.GetActiveWorkspace(envPath)
				if err != nil {
					return err
				}
			}

			path := app.WorkspacePath(envPath, name)
			ws, err := app.LoadWorkspace(path)
			if err != nil {
				return err
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResultText(ws, format, outputPath, func() string {
				var sb strings.Builder
				active, _ := app.GetActiveWorkspace(envPath)
				isActive := ws.Name == active

				sb.WriteString(fmt.Sprintf("Name: %s", ws.Name))
				if isActive {
					sb.WriteString(" (active)")
				}

				if ws.Description != "" {
					sb.WriteString(fmt.Sprintf("\nDescription: %s", ws.Description))
				}

				if ws.Base != nil {
					if app.WorkspaceExists(envPath, *ws.Base) {
						sb.WriteString(fmt.Sprintf("\nBase: %s", *ws.Base))
					} else {
						sb.WriteString(fmt.Sprintf("\nBase: %s (WARNING: does not exist, treated as standalone)", *ws.Base))
					}
				} else {
					sb.WriteString("\nBase: (standalone)")
				}

				sb.WriteString(fmt.Sprintf("\nTargets: %d", len(ws.Targets)))
				for _, t := range ws.Targets {
					if t.Label != "" {
						sb.WriteString(fmt.Sprintf("\n  - %s (%s)", t.Label, t.URL))
					} else {
						sb.WriteString(fmt.Sprintf("\n  - %s", t.URL))
					}
				}

			if len(ws.Delegates) > 0 {
			sb.WriteString(fmt.Sprintf("\nDelegates: %d", len(ws.Delegates)))
			for _, d := range ws.Delegates {
				sb.WriteString(fmt.Sprintf("\n  - %s", d))
			}
		}

		if len(ws.DelegatePreferences) > 0 {
			sb.WriteString(fmt.Sprintf("\nDelegate Preferences: %d", len(ws.DelegatePreferences)))
			for format, delegate := range ws.DelegatePreferences {
				sb.WriteString(fmt.Sprintf("\n  - %s → %s", format, delegate))
			}
			}

				sb.WriteString(fmt.Sprintf("\n\nFile: %s", path))
				return sb.String()
			})
		},
	}


	return cmd
}
