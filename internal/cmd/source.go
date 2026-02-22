package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Aliases: []string{"src", "sources"},
		Short:   "Manage source references on an OBI",
		Long: `Manage binding source references on an OpenBindings interface document.

Sources are registered references to binding specification artifacts
(e.g., OpenAPI specs, usage specs). Adding a source does not derive
operations — use 'ob sync' for that.`,
	}

	cmd.AddCommand(
		newSourceAddCmd(),
		newSourceListCmd(),
		newSourceRemoveCmd(),
	)

	return cmd
}

func newSourceAddCmd() *cobra.Command {
	var (
		key        string
		resolveArg string
		uriArg     string
	)

	cmd := &cobra.Command{
		Use:   "add <obi-path> <format:path>",
		Short: "Register a source reference on an OBI",
		Long: `Register a binding source reference on an OpenBindings interface document.

The source path is stored relative to the OBI file's directory, so the
reference works regardless of where you run commands from.

This does NOT derive operations or create bindings — it only registers
the source reference. Use 'ob sync' afterward to derive operations
and bindings from the source.

The --resolve flag controls how the source is stored in the OBI:
  location  (default) Store a path/URI in the spec 'location' field.
  content   Read the source and embed its content in the spec 'content' field.

When --resolve=location and --uri is provided, the spec location field
uses the given URI instead of the local path.

Examples:
  ob source add my.obi.json usage@2.13.1:./cli.kdl
  ob source add my.obi.json openapi@3.1:./api.yaml --key restApi
  ob source add my.obi.json usage@2.0.0:./cli.kdl --resolve content
  ob source add my.obi.json openapi@3.1:./api.yaml --uri https://cdn.example.com/api.yaml`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			obiPath := args[0]

			// Parse format:path from the second arg.
			src, err := app.ParseSource(args[1])
			if err != nil {
				return app.ExitResult{Code: 2, Message: err.Error(), ToStderr: true}
			}

			result, err := app.SourceAdd(app.SourceAddInput{
				OBIPath:  obiPath,
				Format:   src.Format,
				Location: src.Location,
				Key:      key,
				Resolve:  resolveArg,
				URI:      uriArg,
			})
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}

	cmd.Flags().StringVar(&key, "key", "", "explicit source key (default: derived from format and path)")
	cmd.Flags().StringVar(&resolveArg, "resolve", "", "resolution mode: location (default) or content")
	cmd.Flags().StringVar(&uriArg, "uri", "", "explicit published URI for location mode")

	return cmd
}

func newSourceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list <obi-path>",
		Aliases: []string{"ls"},
		Short:   "List source references on an OBI",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.SourceList(args[0])
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}
	return cmd
}

func newSourceRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <obi-path> <key>",
		Aliases: []string{"rm"},
		Short:   "Remove a source reference from an OBI",
		Long: `Remove a binding source reference from an OpenBindings interface document.

This removes only the source entry. Operations and bindings that reference
this source are preserved — the user decides what to do with them.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.SourceRemove(args[0], args[1])
			if err != nil {
				return app.ExitResult{Code: 1, Message: err.Error(), ToStderr: true}
			}

			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(result, format, outputPath)
		},
	}

	return cmd
}
