// Package app - delegates_remove.go contains the CLI command for removing delegates.
package app

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// DelegateRemoveParams configures the delegate remove command.
type DelegateRemoveParams struct {
	URL          string
	OutputFormat string
	OutputPath   string
}

// DelegateRemove removes a delegate from the active workspace.
func DelegateRemove(params DelegateRemoveParams) error {
	url := strings.TrimSpace(params.URL)
	if url == "" {
		return usageExit("delegate remove <url>")
	}

	// Normalize local paths to the stored exec: form.
	if delegates.IsLocalPath(url) {
		url = delegates.ExecScheme + url
	}

	// Load the active workspace
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return err
	}

	// Find and remove the delegate
	var newDelegates []string
	found := false
	for _, p := range ws.Delegates {
		if p == url {
			found = true
			continue
		}
		newDelegates = append(newDelegates, p)
	}

	if !found {
		return exitText(1, fmt.Sprintf("delegate %q not found in workspace", url), true)
	}

	ws.Delegates = newDelegates

	// Also remove from delegatePreferences if present
	for format, prov := range ws.DelegatePreferences {
		if prov == url {
			delete(ws.DelegatePreferences, format)
		}
	}

	// Save the workspace
	if err := SaveWorkspace(envPath, ws); err != nil {
		return exitText(1, err.Error(), true)
	}

	result := struct {
		Removed   string `json:"removed"`
		Delegate  string `json:"delegate"`
		Workspace string `json:"workspace"`
	}{
		Removed:   "delegate",
		Delegate:  url,
		Workspace: ws.Name,
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

	format, err := ParseOutputFormat(params.OutputFormat)
	if err != nil {
		return err
	}
	if format != OutputFormatText {
		out, err := FormatOutputString(format, result)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}

	return okText(fmt.Sprintf("removed delegate %s from workspace %q", url, ws.Name))
}
