package cmd

import (
	"github.com/openbindings/cli/internal/app"
	"github.com/spf13/cobra"
)

func newInputCreateCmd() *cobra.Command {
	var (
		path       string
		template   string
		force      bool
		openEditor bool
	)
	c := &cobra.Command{
		Use:   "create --target <target-id> --op <opKey> <name>",
		Short: "Create an input file and add reference",
		Long:  "Creates a new JSON file (default under ./inputs/) and stores a reference in the active workspace.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetID, opKey := getInputFlags(cmd)
			out, err := app.InputsCreate(targetID, opKey, args[0], path, template, force, openEditor)
			if err != nil {
				return err
			}
			format, outputPath := getOutputFlags(cmd)
			return app.OutputResult(out, format, outputPath)
		},
	}
	c.Flags().StringVar(&path, "path", "", "path to write the new JSON file (default: ./inputs/...)")
	c.Flags().StringVar(&template, "template", "blank", "template to use: blank|schema")
	c.Flags().BoolVar(&force, "force", false, "overwrite file and/or association if it exists")
	c.Flags().BoolVar(&openEditor, "edit", false, "open the file in editor after creation")
	return c
}
