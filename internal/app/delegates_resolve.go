// Package app - delegates_resolve.go contains the CLI command for delegate resolution.
package app

import (
	"fmt"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// DelegateResolveParams configures delegate resolution.
type DelegateResolveParams struct {
	Format       string
	OutputFormat string
	OutputPath   string // when set, write result to this file (-o)
}

// DelegateResolve is the CLI command handler for delegate resolution.
func DelegateResolve(params DelegateResolveParams) error {
	if strings.TrimSpace(params.Format) == "" {
		return usageExit("delegate resolve <format>")
	}

	// Load the active workspace for delegate preferences
	wsCtx := GetWorkspaceDelegateContext()

	resolved, err := delegates.Resolve(delegates.ResolveParams{
		Format:              params.Format,
		DelegatePreferences: wsCtx.DelegatePreferences,
		WorkspaceDelegates:  wsCtx.Delegates,
	}, BuiltinSupportsFormat)
	if err != nil {
		return exitText(1, err.Error(), true)
	}

	result := struct {
		Format   string `json:"format"`
		Delegate string `json:"delegate"`
		Source   string `json:"source"`
		Location string `json:"location,omitempty"`
	}{
		Format:   resolved.Format,
		Delegate: resolved.Delegate,
		Source:   resolved.Source,
		Location: resolved.Location,
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

	// Text output
	fmt.Printf("Format: %s\n", result.Format)
	fmt.Printf("Delegate: %s\n", result.Delegate)
	fmt.Printf("Source: %s\n", result.Source)
	if result.Location != "" {
		fmt.Printf("Location: %s\n", result.Location)
	}
	return nil
}
