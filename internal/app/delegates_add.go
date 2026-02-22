// Package app - delegates_add.go contains the CLI command for adding delegates.
package app

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// DelegateAddParams configures the delegate add command.
type DelegateAddParams struct {
	URL          string
	OutputFormat string
	OutputPath   string
}

// DelegateAdd adds a binding format handler delegate to the active workspace.
func DelegateAdd(params DelegateAddParams) error {
	url := strings.TrimSpace(params.URL)
	if url == "" {
		return usageExit("delegate add <url>")
	}

	// Validate URL format
	if !delegates.IsHTTPURL(url) && !delegates.IsExecURL(url) && !delegates.IsLocalPath(url) {
		return exitText(1, "delegate must be an exec:, http://, https://, or local path", true)
	}

	// For local paths, convert to exec: URL
	if delegates.IsLocalPath(url) {
		url = delegates.ExecScheme + url
	}

	// Load the active workspace
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return err
	}

	// Check if already present
	for _, p := range ws.Delegates {
		if p == url {
			return exitText(1, fmt.Sprintf("delegate %q already in workspace", url), true)
		}
	}

	// Add the delegate
	ws.Delegates = append(ws.Delegates, url)

	// Save the workspace
	if err := SaveWorkspace(envPath, ws); err != nil {
		return exitText(1, err.Error(), true)
	}

	result := struct {
		Added     string `json:"added"`
		Delegate  string `json:"delegate"`
		Workspace string `json:"workspace"`
	}{
		Added:     "delegate",
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

	return okText(fmt.Sprintf("added delegate %s to workspace %q", url, ws.Name))
}
