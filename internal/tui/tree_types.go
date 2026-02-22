package tui

// Tree node type constants for consistent type identification across the TUI.
// Use these constants instead of string literals when creating or checking node types.
const (
	// NodeTypeOperation is an OpenBindings operation.
	NodeTypeOperation = "operation"

	// NodeTypeBindings is a container for operation bindings.
	NodeTypeBindings = "bindings"

	// NodeTypeBinding is a single binding entry.
	NodeTypeBinding = "binding"

	// NodeTypeInputs is a container for operation inputs.
	NodeTypeInputs = "inputs"

	// NodeTypeInputFile is a saved input file.
	NodeTypeInputFile = "input-file"

	// NodeTypeInputExample is an example from the operation definition.
	NodeTypeInputExample = "input-example"

	// NodeTypeInputNew is the "New..." action node.
	NodeTypeInputNew = "input-new"

	// NodeTypeAliases is a container for operation aliases.
	NodeTypeAliases = "aliases"

	// NodeTypeAlias is a single alias.
	NodeTypeAlias = "alias"

	// NodeTypeSatisfies is a container for satisfies references.
	NodeTypeSatisfies = "satisfies"

	// NodeTypeSatisfiesRef is a single satisfies reference.
	NodeTypeSatisfiesRef = "satisfies-ref"

	// NodeTypeSchema is a JSON schema container.
	NodeTypeSchema = "schema"

	// NodeTypeSchemaProp is a JSON schema property.
	NodeTypeSchemaProp = "schema-prop"

	// NodeTypeSchemaRef is a JSON schema $ref reference.
	NodeTypeSchemaRef = "schema-ref"

	// NodeTypeSchemaSection is a schema section header (e.g., "Input", "Output").
	NodeTypeSchemaSection = "schema-section"
)

// Input source type constants for InputSource.SourceType.
const (
	// InputSourceFile is a saved input file.
	InputSourceFile = "file"

	// InputSourceExample is an example from the operation.
	InputSourceExample = "example"

	// InputSourceNew is a new input being created.
	InputSourceNew = "new"
)

// Input status values - use app.InputStatusOK and app.InputStatusMissing
