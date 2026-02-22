// Package app - delegates_prefer.go contains the CLI command for setting delegate preferences.
package app

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// DelegatePreferParams configures the delegate prefer command.
type DelegatePreferParams struct {
	Format       string
	Delegate     string
	OutputFormat string
	OutputPath   string
}

// DelegatePrefer sets a delegate preference for a format in the active workspace.
func DelegatePrefer(params DelegatePreferParams) error {
	format := strings.TrimSpace(params.Format)
	delegate := strings.TrimSpace(params.Delegate)

	if format == "" || delegate == "" {
		return usageExit("delegate prefer <format> <delegate-url>")
	}

	// Normalize and validate delegate reference.
	// Store URLs/refs (exec:/http(s):) in the workspace, not names.
	switch {
	case delegates.IsHTTPURL(delegate), delegates.IsExecURL(delegate):
		// ok
	case delegates.IsLocalPath(delegate):
		delegate = delegates.ExecScheme + delegate
	default:
		return exitText(1, "delegate must be exec:, http://, https://, or a local path", true)
	}

	// Load the active workspace
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return err
	}

	// Initialize delegatePreferences if nil
	if ws.DelegatePreferences == nil {
		ws.DelegatePreferences = make(map[string]string)
	}

	// Ensure delegate is present in workspace delegates (auto-add).
	found := false
	for _, a := range ws.Delegates {
		if a == delegate {
			found = true
			break
		}
	}
	if !found {
		ws.Delegates = append(ws.Delegates, delegate)
	}

	// Set the preference
	ws.DelegatePreferences[format] = delegate

	// Save the workspace
	if err := SaveWorkspace(envPath, ws); err != nil {
		return exitText(1, err.Error(), true)
	}

	result := struct {
		Set            string `json:"set"`
		Format         string `json:"format"`
		Delegate       string `json:"delegate"`
		Workspace      string `json:"workspace"`
		DelegateAdded  bool   `json:"delegateAdded,omitempty"`
	}{
		Set:           "delegatePreference",
		Format:        format,
		Delegate:      delegate,
		Workspace:     ws.Name,
		DelegateAdded: !found,
	}

	if params.OutputPath != "" {
		outFormat := OutputFormatJSON
		if strings.HasSuffix(strings.ToLower(params.OutputPath), ".yaml") || strings.HasSuffix(strings.ToLower(params.OutputPath), ".yml") {
			outFormat = OutputFormatYAML
		}
		b, err := FormatOutput(result, outFormat)
		if err != nil {
			return err
		}
		if err := AtomicWriteFile(params.OutputPath, b, FilePerm); err != nil {
			return err
		}
		return ExitResult{Code: 0, Message: "Wrote " + params.OutputPath, ToStderr: false}
	}

	outputFmt, err := ParseOutputFormat(params.OutputFormat)
	if err != nil {
		return err
	}
	if outputFmt != OutputFormatText {
		out, err := FormatOutputString(outputFmt, result)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}

	if !found {
		return okText(fmt.Sprintf("added delegate %s and set preference: %s → %s", delegate, format, delegate))
	}
	return okText(fmt.Sprintf("set preference: %s → %s", format, delegate))
}
