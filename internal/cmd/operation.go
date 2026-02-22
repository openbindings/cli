package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newOperationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "operation",
		Aliases: []string{"op", "operations"},
		Short:   "Manage and execute operations on an OBI",
		Long: `Manage and execute operations on an OpenBindings interface document.

Operations define the abstract methods and events that an interface
exposes. Use subcommands to list, rename, remove, or execute operations.`,
	}

	cmd.AddCommand(
		newOperationListCmd(),
		newOperationExecCmd(),
		newOperationRenameCmd(),
		newOperationRemoveCmd(),
	)

	return cmd
}

func newOperationExecCmd() *cobra.Command {
	var bindingKey string
	var inputJSON string
	var contextName string

	cmd := &cobra.Command{
		Use:     "exec <obi-path> [operation]",
		Aliases: []string{"execute"},
		Short:   "Execute an operation via a binding",
		Long: `Execute an operation from an OpenBindings interface.

The operation key is a positional argument. The highest-priority
binding for that operation is used automatically.

Alternatively, use --binding to select a specific binding directly;
the operation is derived from the binding entry.

Use --context to apply a named context (credentials, headers, etc.)
to the execution.

Examples:
  ob op exec interface.json listPets --input '{"limit":10}'
  ob op exec interface.json echo
  ob op exec interface.json --binding listPets.openapi --input '{"limit":10}'
  ob op exec interface.json listPets --context github
  ob op exec interface.json listPets -F json`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			obiFile := args[0]

			// Determine operation key from positional arg or --binding.
			var operationKey string
			if len(args) == 2 {
				operationKey = args[1]
			}

			if operationKey == "" && bindingKey == "" {
				return app.ExitResult{Code: 2, Message: "provide an operation key or use --binding", ToStderr: true}
			}
			if operationKey != "" && bindingKey != "" {
				return app.ExitResult{Code: 2, Message: "operation key and --binding are mutually exclusive", ToStderr: true}
			}

			// Parse input JSON if provided.
			var input any
			if inputJSON != "" {
				if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
					return app.ExitResult{Code: 2, Message: fmt.Sprintf("invalid --input JSON: %v", err), ToStderr: true}
				}
			}

			// Check if the operation is an event — if so, stream via subscribe.
			isEvent := (operationKey != "" && app.IsEventOperation(obiFile, operationKey)) ||
				(bindingKey != "" && app.IsEventBinding(obiFile, bindingKey))
			if isEvent {
				ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
				defer stop()

				ch, err := app.SubscribeOBIOperation(ctx, obiFile, operationKey, bindingKey, input, contextName)
				if err != nil {
					return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
				}

				enc := json.NewEncoder(os.Stdout)
				for ev := range ch {
					if ev.Error != nil {
						fmt.Fprintf(os.Stderr, "stream error: %s\n", ev.Error.Message)
						continue
					}
					if err := enc.Encode(ev.Data); err != nil {
						return app.ExitResult{Code: 1, Message: fmt.Sprintf("write error: %v", err), ToStderr: true}
					}
				}
				return nil
			}

			output := app.ExecuteOBIOperation(context.Background(), obiFile, operationKey, bindingKey, input, contextName)
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(output, format, outputPath)
		},
	}

	cmd.Flags().StringVar(&bindingKey, "binding", "", "binding key to execute (operation is derived from the entry)")
	cmd.Flags().StringVar(&inputJSON, "input", "", "operation input as JSON")
	cmd.Flags().StringVar(&contextName, "context", "", "named context to apply (credentials, headers, etc.)")

	return cmd
}

func newOperationListCmd() *cobra.Command {
	var tagFilter string

	cmd := &cobra.Command{
		Use:     "list <obi-path>",
		Aliases: []string{"ls"},
		Short:   "List operations on an OBI",
		Long: `List all operations defined in an OpenBindings interface document.

Shows each operation's key, kind, tags, managed status, and binding count.

Examples:
  ob operation list interface.json
  ob op list interface.json --tag admin
  ob op list interface.json -F json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.OperationList(args[0], tagFilter)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}

	cmd.Flags().StringVar(&tagFilter, "tag", "", "filter operations by tag")

	return cmd
}

func newOperationRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rename <obi-path> <old-key> <new-key>",
		Aliases: []string{"mv"},
		Short:   "Rename an operation and update all references",
		Long: `Rename an operation key throughout an OpenBindings interface document.

Updates the operation key, all binding 'operation' fields that reference
it, and binding keys that follow the <operation>.<source> convention.

Examples:
  ob operation rename interface.json hello greet
  ob op rename interface.json config.set settings.update`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.OperationRename(args[0], args[1], args[2])
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}

	return cmd
}

func newOperationRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove <obi-path> <key>...",
		Aliases: []string{"rm"},
		Short:   "Remove operations and their bindings from an OBI",
		Long: `Remove one or more operations from an OpenBindings interface document.

All bindings that reference the removed operations are also deleted.

For managed operations (those with x-ob metadata from sync), a warning
is shown because the next 'ob sync' will recreate them. Use --force to
suppress the warning, or 'ob source remove' to stop syncing from the
source entirely.

Examples:
  ob operation remove interface.json hello
  ob op remove interface.json config.set config.get
  ob op remove interface.json hello --force`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			obiPath := args[0]
			keys := args[1:]

			// Warn about managed operations unless --force.
			if !force {
				if warning := checkManagedOps(obiPath, keys); warning != "" {
					return app.ExitResult{Code: 1, Message: warning, ToStderr: true}
				}
			}

			result, err := app.OperationRemove(obiPath, keys)
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "remove managed operations without warning")

	return cmd
}

// checkManagedOps loads the OBI and returns a warning string if any of the
// given operation keys are managed (have x-ob metadata). Returns "" if none are managed.
func checkManagedOps(obiPath string, keys []string) string {
	result, err := app.OperationList(obiPath, "")
	if err != nil {
		return "" // let the actual remove call surface the error
	}

	managed := map[string]bool{}
	for _, op := range result.Operations {
		if op.Managed {
			managed[op.Key] = true
		}
	}

	var warn []string
	for _, key := range keys {
		if managed[key] {
			warn = append(warn, key)
		}
	}

	if len(warn) == 0 {
		return ""
	}

	return "managed operations (will be recreated by sync): " +
		joinKeys(warn) +
		"\nuse --force to remove anyway, or 'ob source remove' to stop syncing"
}

func joinKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = "\"" + k + "\""
	}
	return strings.Join(quoted, ", ")
}
