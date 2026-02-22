// Package usage - execute.go contains operation execution logic for usage specs.
package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/usage-go/usage"
)

// Execution configuration constants.
const (
	// ResolveArtifactTimeout is the maximum time to wait when resolving an exec: artifact.
	// This timeout applies when running a command to retrieve a usage spec dynamically.
	// 5 seconds is generous for most CLI tools but may need adjustment for slow tools.
	ResolveArtifactTimeout = 5 * time.Second
)

// Execute executes an operation defined in a usage spec.
//
// There are two execution modes:
//  1. Direct mode: If Binary hint is provided, use the ref directly as the CLI command.
//  2. Spec mode: Load the usage spec to get the binary name and validate the command.
func Execute(input ExecuteInput) ExecuteOutput {
	return ExecuteWithContext(context.Background(), input)
}

// ExecuteWithContext executes an operation with cancellation support.
func ExecuteWithContext(ctx context.Context, input ExecuteInput) ExecuteOutput {
	start := time.Now()

	var binName string
	var args []string

	// If we have a binary hint, use direct execution (primary path for CLI targets)
	if input.Source.Binary != "" {
		binName = input.Source.Binary
		var err error
		args, err = buildDirectArgsFromRef(input.Ref, input.Input)
		if err != nil {
			return ExecuteOutput{
				Error: &Error{
					Code:    "args_build_failed",
					Message: err.Error(),
					Details: map[string]any{"ref": input.Ref},
				},
				DurationMs: time.Since(start).Milliseconds(),
			}
		}
	} else {
		// No binary hint - load the usage spec to get binary name and validate
		spec, err := loadSpec(input.Source)
		if err != nil {
			return ExecuteOutput{
				Error: &Error{
					Code:    "spec_load_failed",
					Message: err.Error(),
				},
				DurationMs: time.Since(start).Milliseconds(),
			}
		}

		// Find the command matching the ref
		found, err := findCommand(spec, input.Ref)
		if err != nil {
			return ExecuteOutput{
				Error: &Error{
					Code:    "command_not_found",
					Message: err.Error(),
					Details: map[string]any{"ref": input.Ref},
				},
				DurationMs: time.Since(start).Milliseconds(),
			}
		}

		// Get the binary name from spec
		meta := spec.Meta()
		binName = meta.Bin
		if binName == "" {
			binName = meta.Name
		}
		if binName == "" {
			return ExecuteOutput{
				Error: &Error{
					Code:    "no_binary",
					Message: "usage spec does not define a binary name (bin or name)",
				},
				DurationMs: time.Since(start).Milliseconds(),
			}
		}

		// Build CLI arguments from spec (pass inherited globals for global flag support)
		args, err = buildCLIArgs(found.path, found.cmd, found.inheritedFlags, input.Input)
		if err != nil {
			return ExecuteOutput{
				Error: &Error{
					Code:    "args_build_failed",
					Message: err.Error(),
				},
				DurationMs: time.Since(start).Milliseconds(),
			}
		}
	}

	// Execute the CLI command
	output, status, err := runCLI(ctx, binName, args)
	duration := time.Since(start).Milliseconds()

	// Check for context cancellation
	if ctx.Err() != nil {
		return ExecuteOutput{
			DurationMs: duration,
			Error: &Error{
				Code:    "cancelled",
				Message: "operation cancelled",
			},
		}
	}

	if err != nil {
		return ExecuteOutput{
			Output:     output,
			Status:     status,
			DurationMs: duration,
			Error: &Error{
				Code:    "execution_failed",
				Message: err.Error(),
			},
		}
	}

	return ExecuteOutput{
		Output:     output,
		Status:     status,
		DurationMs: duration,
	}
}

// buildDirectArgsFromRef builds CLI arguments directly from the ref.
// Input is a flat object where keys are flag names and values are flag values.
// This matches the output format of input transforms.
func buildDirectArgsFromRef(ref string, input any) ([]string, error) {
	args, err := shlex.Split(ref)
	if err != nil {
		return nil, err
	}

	if input == nil {
		return args, nil
	}

	inputMap, ok := toStringMap(input)
	if !ok {
		return args, nil
	}

	// All top-level keys are flags
	for name, value := range inputMap {
		flagArgs, _ := formatFlagWithDef(name, value, usage.Flag{})
		args = append(args, flagArgs...)
	}

	return args, nil
}

// loadSpec loads a usage spec from the source.
func loadSpec(source Source) (*usage.Spec, error) {
	if source.Location != "" {
		// Handle exec: locations by running the command and capturing output
		if strings.HasPrefix(source.Location, "exec:") {
			content, err := resolveCommandArtifact(source.Location)
			if err != nil {
				return nil, fmt.Errorf("resolve cmd artifact: %w", err)
			}
			spec, err := usage.ParseKDL([]byte(content))
			if err != nil {
				return nil, fmt.Errorf("parse usage content from exec: %w", err)
			}
			return spec, nil
		}

		// Regular file path
		spec, err := usage.ParseFile(source.Location)
		if err != nil {
			return nil, fmt.Errorf("parse usage spec: %w", err)
		}
		return spec, nil
	}

	if source.Content != nil {
		switch c := source.Content.(type) {
		case string:
			spec, err := usage.ParseKDL([]byte(c))
			if err != nil {
				return nil, fmt.Errorf("parse usage content: %w", err)
			}
			return spec, nil
		default:
			return nil, fmt.Errorf("unsupported content type %T (expected string)", source.Content)
		}
	}

	return nil, fmt.Errorf("source must have location or content")
}

// findCommand finds a command in the spec matching the given ref.
// It matches against primary command names and aliases.
// findCommandResult contains the result of finding a command in the spec.
type findCommandResult struct {
	path           []string
	cmd            *usage.Command
	inheritedFlags []usage.Flag // Global flags from ancestor commands
}

func findCommand(spec *usage.Spec, ref string) (*findCommandResult, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("ref is empty")
	}

	targetPath := strings.Fields(ref)
	var result *findCommandResult

	// Walk the spec collecting global flags along the path
	var walkWithGlobals func(cmds []usage.Command, path []string, inheritedGlobals []usage.Flag)
	walkWithGlobals = func(cmds []usage.Command, path []string, inheritedGlobals []usage.Flag) {
		for _, cmd := range cmds {
			cmdPath := make([]string, len(path)+1)
			copy(cmdPath, path)
			cmdPath[len(path)] = cmd.Name

			// Collect this command's global flags for children
			var globalsForChildren []usage.Flag
			globalsForChildren = append(globalsForChildren, inheritedGlobals...)
			for _, f := range cmd.Flags {
				if f.Global {
					globalsForChildren = append(globalsForChildren, f)
				}
			}

			// Check if this command matches
			if pathMatchesWithAliases(cmdPath, targetPath, cmd) {
				cmdCopy := cmd
				result = &findCommandResult{
					path:           cmdPath,
					cmd:            &cmdCopy,
					inheritedFlags: inheritedGlobals,
				}
			}

			// Recurse into subcommands
			walkWithGlobals(cmd.Commands, cmdPath, globalsForChildren)
		}
	}

	walkWithGlobals(spec.Commands(), nil, nil)

	if result == nil {
		return nil, fmt.Errorf("command %q not found in usage spec", ref)
	}

	return result, nil
}

// pathMatchesWithAliases checks if targetPath matches the command path,
// considering aliases for the final segment.
func pathMatchesWithAliases(cmdPath, targetPath []string, cmd usage.Command) bool {
	if len(cmdPath) != len(targetPath) {
		return false
	}
	if len(cmdPath) == 0 {
		return true
	}

	// Check all segments except the last must match exactly
	for i := 0; i < len(cmdPath)-1; i++ {
		if cmdPath[i] != targetPath[i] {
			return false
		}
	}

	// For the last segment, check primary name or aliases
	lastIdx := len(cmdPath) - 1
	targetName := targetPath[lastIdx]

	// Check primary name
	if cmdPath[lastIdx] == targetName {
		return true
	}

	// Check aliases
	for _, alias := range cmd.Aliases {
		for _, name := range alias.Names {
			if name == targetName {
				return true
			}
		}
	}

	return false
}

// buildCLIArgs builds CLI arguments from the command and input.
// It reads the usage spec to determine which input fields are flags vs positional args.
// inheritedGlobals contains global flags from ancestor commands.
func buildCLIArgs(cmdPath []string, cmd *usage.Command, inheritedGlobals []usage.Flag, input any) ([]string, error) {
	var args []string
	args = append(args, cmdPath...)

	if input == nil {
		return args, nil
	}

	inputMap, ok := toStringMap(input)
	if !ok {
		return nil, fmt.Errorf("input must be an object with field names matching the command's flags and args")
	}

	// Build a map of flag names -> flag definitions from the usage spec
	// Pass inheritedGlobals to include global flags from ancestor commands
	flagDefs := make(map[string]usage.Flag)
	for _, f := range cmd.AllFlags(inheritedGlobals) {
		name := f.PrimaryName()
		if name != "" {
			flagDefs[name] = f
		}
		// Also add short names as aliases
		parsed := f.ParseUsage()
		for _, short := range parsed.Short {
			flagDefs[short] = f
		}
		for _, long := range parsed.Long {
			flagDefs[long] = f
		}
	}

	// Build an ordered list of arg definitions from the usage spec
	// We need to preserve order for positional args
	type argDef struct {
		name     string
		cleanName string
		def      usage.Arg
	}
	var argDefs []argDef
	for _, a := range cmd.Args {
		argDefs = append(argDefs, argDef{
			name:      a.Name,
			cleanName: a.CleanName(),
			def:       a,
		})
	}

	// Track which input fields we've processed
	processed := make(map[string]bool)

	// First pass: process flags
	for key, value := range inputMap {
		if flagDef, isFlag := flagDefs[key]; isFlag {
			flagArgs, err := formatFlagWithDef(key, value, flagDef)
			if err != nil {
				return nil, fmt.Errorf("flag %q: %w", key, err)
			}
			args = append(args, flagArgs...)
			processed[key] = true
		}
	}

	// Second pass: process positional args in spec-defined order
	// Track if we've inserted the double-dash separator
	doubleDashInserted := false

	for _, ad := range argDefs {
		value, exists := inputMap[ad.cleanName]
		if !exists {
			continue
		}
		processed[ad.cleanName] = true

		// Check if this arg requires double-dash separator
		// double_dash can be "required", "optional", "automatic", or "preserve"
		if !doubleDashInserted && (ad.def.DoubleDash == "required" || ad.def.DoubleDash == "optional") {
			args = append(args, "--")
			doubleDashInserted = true
		}

		// Handle variadic args (arrays) vs single args
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				args = append(args, fmt.Sprintf("%v", item))
			}
		case []string:
			args = append(args, v...)
		case string:
			args = append(args, v)
		case nil:
			// Skip nil values
		default:
			args = append(args, fmt.Sprintf("%v", v))
		}
	}

	// Check for unrecognized fields (optional: could be made strict)
	for key := range inputMap {
		if !processed[key] {
			// Field doesn't match any flag or arg in the usage spec
			// For now, treat as an error to catch transform bugs
			return nil, fmt.Errorf("unknown field %q: not defined as a flag or arg in the usage spec for this command", key)
		}
	}

	return args, nil
}

// formatFlagWithDef formats a flag value using the flag definition.
// Handles:
//   - Negate flags: if value is false and Negate is set, emits the negated form.
//   - Count flags: if Count is true and value is a number, emits the flag that many times.
func formatFlagWithDef(name string, value any, flagDef usage.Flag) ([]string, error) {
	prefix := "--"
	if len(name) == 1 {
		prefix = "-"
	}
	flagName := prefix + name

	// Handle count flags: emit the flag N times for numeric values
	if flagDef.Count {
		count := 0
		switch v := value.(type) {
		case int:
			count = v
		case int64:
			count = int(v)
		case float64:
			count = int(v)
		case bool:
			if v {
				count = 1
			}
		}
		if count <= 0 {
			return nil, nil
		}
		var args []string
		for i := 0; i < count; i++ {
			args = append(args, flagName)
		}
		return args, nil
	}

	switch v := value.(type) {
	case bool:
		if v {
			return []string{flagName}, nil
		}
		// Value is false - check if we should emit the negated form
		if flagDef.Negate != "" {
			// Negate contains the full flag name like "--no-color"
			return []string{flagDef.Negate}, nil
		}
		// No negate form, just don't emit anything
		return nil, nil
	case string:
		return []string{flagName, v}, nil
	case float64:
		return []string{flagName, fmt.Sprintf("%v", v)}, nil
	case int, int64:
		return []string{flagName, fmt.Sprintf("%d", v)}, nil
	case []any:
		var args []string
		for _, item := range v {
			args = append(args, flagName, fmt.Sprintf("%v", item))
		}
		return args, nil
	case nil:
		return nil, nil
	default:
		return []string{flagName, fmt.Sprintf("%v", v)}, nil
	}
}

// runCLI executes a CLI command and captures its output.
// If stdout contains valid JSON, it is parsed and returned directly.
// Otherwise, returns {stdout: string, stderr?: string}.
// The context allows cancellation of long-running commands.
func runCLI(ctx context.Context, binName string, args []string) (any, int, error) {
	cmd := exec.CommandContext(ctx, binName, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, 1, err
		}
	}

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Try to parse stdout as JSON for cleaner output
	// This allows CLI commands that output JSON to return structured data
	if exitCode == 0 && len(stdoutStr) > 0 {
		trimmed := strings.TrimSpace(stdoutStr)
		if delegates.MaybeJSON(trimmed) {
			var parsed any
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				// Successfully parsed JSON - return it directly
				// Include stderr as metadata if present
				if stderrStr != "" {
					return map[string]any{
						"data":   parsed,
						"stderr": stderrStr,
					}, 0, nil
				}
				return parsed, 0, nil
			}
		}
	}

	// Fallback: return raw stdout/stderr
	output := map[string]any{
		"stdout": stdoutStr,
	}
	if stderrStr != "" {
		output["stderr"] = stderrStr
	}

	return output, exitCode, nil
}

// resolveCommandArtifact resolves an exec: artifact by running the command.
func resolveCommandArtifact(location string) (string, error) {
	cmdStr := strings.TrimPrefix(location, "exec:")
	if cmdStr == "" {
		return "", fmt.Errorf("empty command in exec: artifact")
	}

	parts, err := shlex.Split(cmdStr)
	if err != nil {
		return "", fmt.Errorf("invalid command syntax: %w", err)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command in exec: artifact")
	}

	binName := parts[0]
	args := parts[1:]

	ctx, cancel := context.WithTimeout(context.Background(), ResolveArtifactTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binName, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("command failed: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("command failed: %w", err)
	}

	return stdout.String(), nil
}

// toStringMap converts any to map[string]any if possible.
func toStringMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}
