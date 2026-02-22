// Package usage - convert.go contains interface creation logic for usage specs.
package usage

import (
	"fmt"
	"strings"

	"github.com/openbindings/openbindings-go"
	"github.com/openbindings/openbindings-go/canonicaljson"
	"github.com/openbindings/openbindings-go/formattoken"
	"github.com/openbindings/usage-go/usage"
)

// Schema type constants for JSON Schema generation.
const (
	schemaTypeString  = "string"
	schemaTypeBoolean = "boolean"
	schemaTypeInteger = "integer"
	schemaTypeArray   = "array"
	schemaTypeObject  = "object"
)

// DefaultSourceName is the default name for a binding source.
const DefaultSourceName = "usage"


// ConvertParams defines parameters for converting a usage spec.
type ConvertParams struct {
	ToFormat  string
	InputPath string
	Content   string // Inline content (alternative to InputPath)
}

// ConvertToInterface converts a usage spec to an openbindings.Interface directly.
func ConvertToInterface(params ConvertParams) (openbindings.Interface, error) {
	if strings.TrimSpace(params.InputPath) == "" && strings.TrimSpace(params.Content) == "" {
		return openbindings.Interface{}, fmt.Errorf("missing input path or content")
	}

	toFormat := strings.TrimSpace(params.ToFormat)
	if toFormat == "" {
		toFormat = "openbindings@" + openbindings.MaxTestedVersion
	}

	toTok, err := formattoken.Parse(toFormat)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("invalid --to %q", toFormat)
	}

	if toTok.Name != "openbindings" {
		return openbindings.Interface{}, fmt.Errorf("unsupported --to %q (expected openbindings@<ver>)", toFormat)
	}
	fromTok := formattoken.FormatToken{Name: "usage", Version: usage.MaxTestedVersion}
	ok, err := usage.IsSupportedVersion(fromTok.Version)
	if err != nil || !ok {
		return openbindings.Interface{}, fmt.Errorf("unsupported usage version %q (supported %s-%s)", fromTok.Version, usage.MinSupportedVersion, usage.MaxTestedVersion)
	}
	ok, err = openbindings.IsSupportedVersion(toTok.Version)
	if err != nil || !ok {
		return openbindings.Interface{}, fmt.Errorf("unsupported openbindings version %q (supported %s-%s)", toTok.Version, openbindings.MinSupportedVersion, openbindings.MaxTestedVersion)
	}

	var spec *usage.Spec
	if params.Content != "" {
		spec, err = usage.ParseKDL([]byte(params.Content))
	} else {
		spec, err = usage.ParseFile(params.InputPath)
	}
	if err != nil {
		return openbindings.Interface{}, err
	}

	meta := spec.Meta()

	sourceEntry := openbindings.Source{
		Format: fromTok.String(),
	}
	if params.InputPath != "" {
		sourceEntry.Location = params.InputPath
	}

	iface := openbindings.Interface{
		OpenBindings: toTok.Version,
		Name:         meta.Name,
		Version:      meta.Version,
		Description:  meta.About,
		Operations:   map[string]openbindings.Operation{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: sourceEntry,
		},
	}

	bindings := map[string]openbindings.BindingEntry{}
	var dupes []string
	var schemaErr error

	// Walk the command tree with inherited globals tracking
	walkWithGlobals(spec, func(path []string, cmd usage.Command, inheritedGlobals []usage.Flag) {
		if schemaErr != nil {
			return // Stop processing on first error
		}
		if len(path) == 0 {
			return
		}
		// Skip commands that require a subcommand - they can't be invoked directly
		if cmd.SubcommandRequired {
			return
		}
		opKey := strings.Join(path, ".")
		if _, exists := iface.Operations[opKey]; exists {
			dupes = append(dupes, opKey)
			return
		}

		op := openbindings.Operation{
			Kind:        openbindings.OperationKindMethod,
			Description: cmd.Help,
		}

		// Derive tags from parent command hierarchy.
		// e.g. path ["config", "set"] → tags: ["config"]
		if len(path) > 1 {
			op.Tags = make([]string, len(path)-1)
			copy(op.Tags, path[:len(path)-1])
		}

		// Map command aliases to operation aliases
		for _, alias := range cmd.Aliases {
			for _, name := range alias.Names {
				if !alias.Hide {
					op.Aliases = append(op.Aliases, name)
				}
			}
		}

		// Generate input schema from flags and args
		inputSchema, err := generateInputSchema(cmd, inheritedGlobals)
		if err != nil {
			schemaErr = err
			return
		}
		if inputSchema != nil {
			op.Input = inputSchema
		}

		iface.Operations[opKey] = op
		bindingKey := opKey + "." + DefaultSourceName
		bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       strings.Join(path, " "),
		}
	})
	if schemaErr != nil {
		return openbindings.Interface{}, schemaErr
	}
	if len(dupes) > 0 {
		return openbindings.Interface{}, fmt.Errorf("duplicate command paths: %s", strings.Join(dupes, ", "))
	}
	iface.Bindings = bindings

	return iface, nil
}

// Convert converts a usage spec to OpenBindings JSON bytes.
func Convert(params ConvertParams) ([]byte, error) {
	iface, err := ConvertToInterface(params)
	if err != nil {
		return nil, err
	}

	out, err := canonicaljson.Marshal(iface)
	if err != nil {
		return nil, fmt.Errorf("marshal openbindings: %w", err)
	}
	return out, nil
}

// walkWithGlobals walks the command tree while tracking inherited global flags.
// This is similar to Spec.Walk but passes the accumulated global flags from ancestors.
func walkWithGlobals(spec *usage.Spec, fn func(path []string, cmd usage.Command, inheritedGlobals []usage.Flag)) {
	for _, cmd := range spec.Commands() {
		walkCommandWithGlobals([]string{}, cmd, nil, fn)
	}
}

// walkCommandWithGlobals recursively walks commands, accumulating global flags.
func walkCommandWithGlobals(path []string, cmd usage.Command, inheritedGlobals []usage.Flag, fn func([]string, usage.Command, []usage.Flag)) {
	currentPath := make([]string, len(path)+1)
	copy(currentPath, path)
	currentPath[len(path)] = cmd.Name

	// Accumulate globals from this command
	var newGlobals []usage.Flag
	newGlobals = append(newGlobals, inheritedGlobals...)
	for _, f := range cmd.Flags {
		if f.Global {
			newGlobals = append(newGlobals, f)
		}
	}

	fn(currentPath, cmd, inheritedGlobals)

	for _, sub := range cmd.Commands {
		walkCommandWithGlobals(currentPath, sub, newGlobals, fn)
	}
}

// generateInputSchema generates a JSON Schema for a command's input (flags + args).
// The schema is a flat object where keys are flag/arg names.
// This matches the OpenBindings transform output format for CLI/Usage bindings.
// Returns an error if there are name collisions between flags and args.
func generateInputSchema(cmd usage.Command, inheritedGlobals []usage.Flag) (map[string]any, error) {
	properties := make(map[string]any)
	seen := make(map[string]string) // name -> source description for error messages
	var required []string

	// Process all flags (including inherited globals)
	allFlags := cmd.AllFlags(inheritedGlobals)
	for _, flag := range allFlags {
		name := flag.PrimaryName()
		if name == "" {
			continue
		}

		// Check for collision
		if existing, ok := seen[name]; ok {
			return nil, fmt.Errorf("name collision in command %q: %q is used by both %s and flag --%s",
				cmd.Name, name, existing, name)
		}
		seen[name] = fmt.Sprintf("flag --%s", name)

		prop := generateFlagSchema(flag)
		if prop != nil {
			properties[name] = prop
		}
	}

	// Process positional arguments
	for _, arg := range cmd.Args {
		name := arg.CleanName()
		if name == "" {
			continue
		}

		// Check for collision
		if existing, ok := seen[name]; ok {
			return nil, fmt.Errorf("name collision in command %q: %q is used by both %s and arg <%s>",
				cmd.Name, name, existing, name)
		}
		seen[name] = fmt.Sprintf("arg <%s>", name)

		prop := generateArgSchema(arg)
		if prop != nil {
			properties[name] = prop
		}

		// Required if using <name> syntax (not [name]) AND no default value
		// Args with defaults are effectively optional at runtime
		if arg.IsRequired() && arg.Default == nil {
			required = append(required, name)
		}
	}

	// Build the schema object
	if len(properties) == 0 {
		return nil, nil
	}

	schema := map[string]any{
		"type":       schemaTypeObject,
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// generateFlagSchema generates a JSON Schema property for a flag.
func generateFlagSchema(flag usage.Flag) map[string]any {
	prop := make(map[string]any)

	parsed := flag.ParseUsage()

	// Determine the type based on flag characteristics
	if flag.Count {
		// Count flags like -vvv produce an integer
		prop["type"] = schemaTypeInteger
		if flag.Help != "" {
			prop["description"] = flag.Help
		}
		if flag.Default != nil {
			prop["default"] = flag.Default
		}
		return prop
	}

	// Check if flag takes a value (has an arg in the usage like "--user <user>")
	takesValue := parsed.ArgName != "" || len(flag.Args) > 0

	if !takesValue {
		// Boolean flag (just a switch)
		prop["type"] = schemaTypeBoolean
		if flag.Help != "" {
			prop["description"] = flag.Help
		}
		if flag.Default != nil {
			prop["default"] = flag.Default
		}
		return prop
	}

	// Flag takes a value
	if flag.Var {
		// Variadic: can be repeated (--include a --include b)
		itemSchema := map[string]any{"type": schemaTypeString}
		if len(flag.Choices) > 0 {
			itemSchema["enum"] = flag.Choices
		}
		prop["type"] = schemaTypeArray
		prop["items"] = itemSchema
		// Add min/max constraints if specified
		if flag.VarMin != nil {
			prop["minItems"] = *flag.VarMin
		}
		if flag.VarMax != nil {
			prop["maxItems"] = *flag.VarMax
		}
	} else {
		// Single value
		prop["type"] = schemaTypeString
		if len(flag.Choices) > 0 {
			prop["enum"] = flag.Choices
		}
	}

	if flag.Help != "" {
		prop["description"] = flag.Help
	}
	if flag.Default != nil {
		prop["default"] = flag.Default
	}

	return prop
}

// generateArgSchema generates a JSON Schema property for a positional argument.
func generateArgSchema(arg usage.Arg) map[string]any {
	prop := make(map[string]any)

	if arg.IsVariadic() {
		// Variadic: can accept multiple values
		itemSchema := map[string]any{"type": schemaTypeString}
		if len(arg.Choices) > 0 {
			itemSchema["enum"] = arg.Choices
		}
		prop["type"] = schemaTypeArray
		prop["items"] = itemSchema
		// Add min/max constraints
		if arg.VarMin != nil {
			prop["minItems"] = *arg.VarMin
		} else if arg.IsRequired() && arg.Default == nil {
			// Required variadic without explicit min needs at least 1
			prop["minItems"] = 1
		}
		if arg.VarMax != nil {
			prop["maxItems"] = *arg.VarMax
		}
	} else {
		// Single value
		prop["type"] = schemaTypeString
		if len(arg.Choices) > 0 {
			prop["enum"] = arg.Choices
		}
	}

	if arg.Help != "" {
		prop["description"] = arg.Help
	}
	if arg.Default != nil {
		prop["default"] = arg.Default
	}

	return prop
}
