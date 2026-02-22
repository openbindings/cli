// Package usage - convert_test.go contains tests for interface conversion.
package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openbindings/usage-go/usage"
)

func TestGenerateInputSchema_BasicFlags(t *testing.T) {
	// Create a command with basic flags
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-v --verbose", Help: "Verbose output"},
			{Usage: "-o --output <file>", Help: "Output file"},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema == nil {
		t.Fatal("expected schema, got nil")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}

	// Check verbose flag (boolean)
	verbose, ok := props["verbose"].(map[string]any)
	if !ok {
		t.Fatal("expected verbose property")
	}
	if verbose["type"] != schemaTypeBoolean {
		t.Errorf("expected verbose type boolean, got %v", verbose["type"])
	}

	// Check output flag (string with value)
	output, ok := props["output"].(map[string]any)
	if !ok {
		t.Fatal("expected output property")
	}
	if output["type"] != schemaTypeString {
		t.Errorf("expected output type string, got %v", output["type"])
	}
}

func TestGenerateInputSchema_CountFlag(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-v --verbose", Help: "Verbosity level", Count: true},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	verbose := props["verbose"].(map[string]any)

	if verbose["type"] != schemaTypeInteger {
		t.Errorf("expected count flag type integer, got %v", verbose["type"])
	}
}

func TestGenerateInputSchema_VariadicFlag(t *testing.T) {
	min := 1
	max := 5
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--include <pattern>", Help: "Include patterns", Var: true, VarMin: &min, VarMax: &max},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	include := props["include"].(map[string]any)

	if include["type"] != schemaTypeArray {
		t.Errorf("expected variadic flag type array, got %v", include["type"])
	}
	if include["minItems"] != 1 {
		t.Errorf("expected minItems 1, got %v", include["minItems"])
	}
	if include["maxItems"] != 5 {
		t.Errorf("expected maxItems 5, got %v", include["maxItems"])
	}
}

func TestGenerateInputSchema_FlagWithChoices(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--shell <shell>", Help: "Shell type", Choices: []string{"bash", "zsh", "fish"}},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	shell := props["shell"].(map[string]any)

	enum, ok := shell["enum"].([]string)
	if !ok {
		t.Fatal("expected enum array")
	}
	if len(enum) != 3 || enum[0] != "bash" {
		t.Errorf("expected enum [bash, zsh, fish], got %v", enum)
	}
}

func TestGenerateInputSchema_RequiredArg(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<file>", Help: "Input file"},
			{Name: "[output]", Help: "Output file"},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("expected required array")
	}
	if len(required) != 1 || required[0] != "file" {
		t.Errorf("expected required [file], got %v", required)
	}
}

func TestGenerateInputSchema_ArgWithDefault(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<file>", Help: "Input file", Default: "input.txt"},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Args with defaults should NOT be in required array
	required, ok := schema["required"].([]string)
	if ok && len(required) > 0 {
		t.Errorf("expected no required fields for arg with default, got %v", required)
	}

	props := schema["properties"].(map[string]any)
	file := props["file"].(map[string]any)
	if file["default"] != "input.txt" {
		t.Errorf("expected default 'input.txt', got %v", file["default"])
	}
}

func TestGenerateInputSchema_VariadicArgRequiresMinItems(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<files>...", Help: "Input files"},
		},
	}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)
	files := props["files"].(map[string]any)

	if files["type"] != schemaTypeArray {
		t.Errorf("expected array type, got %v", files["type"])
	}
	if files["minItems"] != 1 {
		t.Errorf("expected minItems 1 for required variadic, got %v", files["minItems"])
	}
}

func TestGenerateInputSchema_NameCollision(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--name <value>", Help: "Name flag"},
		},
		Args: []usage.Arg{
			{Name: "<name>", Help: "Name arg"},
		},
	}

	_, err := generateInputSchema(cmd, nil)
	if err == nil {
		t.Fatal("expected error for name collision")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("expected collision error, got: %v", err)
	}
}

func TestGenerateInputSchema_InheritedGlobals(t *testing.T) {
	cmd := usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--local <value>", Help: "Local flag"},
		},
	}

	inheritedGlobals := []usage.Flag{
		{Usage: "-v --verbose", Help: "Global verbose", Global: true},
	}

	schema, err := generateInputSchema(cmd, inheritedGlobals)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := schema["properties"].(map[string]any)

	// Should have both local and inherited global flags
	if _, ok := props["local"]; !ok {
		t.Error("expected local flag in schema")
	}
	if _, ok := props["verbose"]; !ok {
		t.Error("expected inherited global flag in schema")
	}
}

func TestGenerateInputSchema_EmptyCommand(t *testing.T) {
	cmd := usage.Command{Name: "test"}

	schema, err := generateInputSchema(cmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema != nil {
		t.Errorf("expected nil schema for command with no flags/args, got %v", schema)
	}
}

func TestConvertToInterface_BasicSpec(t *testing.T) {
	// Create a temp usage spec file
	content := `
name "test-cli"
bin "test"
about "A test CLI"

cmd "hello" help="Say hello" {
  arg "[name]" help="Name to greet"
  flag "-F --format <format>" help="Output format: json|yaml|text"
}
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.usage.kdl")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	iface, err := ConvertToInterface(ConvertParams{
		InputPath: specPath,
	})
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	if iface.Name != "test-cli" {
		t.Errorf("expected Name 'test-cli', got %q", iface.Name)
	}

	// Check operation was created
	op, ok := iface.Operations["hello"]
	if !ok {
		t.Fatal("expected 'hello' operation")
	}
	if op.Kind != "method" {
		t.Errorf("expected kind 'method', got %q", op.Kind)
	}

	// Check input schema exists
	if op.Input == nil {
		t.Error("expected input schema for hello operation")
	}
}

func TestConvertToInterface_SubcommandRequired(t *testing.T) {
	content := `
name "test-cli"
bin "test"

cmd "config" help="Config commands" subcommand_required=#true {
  cmd "set" help="Set a value"
  cmd "get" help="Get a value"
}
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.usage.kdl")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	iface, err := ConvertToInterface(ConvertParams{
		InputPath: specPath,
	})
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// config should NOT have an operation (subcommand_required)
	if _, ok := iface.Operations["config"]; ok {
		t.Error("expected no 'config' operation (subcommand_required=true)")
	}

	// config.set and config.get should exist
	if _, ok := iface.Operations["config.set"]; !ok {
		t.Error("expected 'config.set' operation")
	}
	if _, ok := iface.Operations["config.get"]; !ok {
		t.Error("expected 'config.get' operation")
	}

	// Subcommands should have tags derived from parent hierarchy.
	setOp := iface.Operations["config.set"]
	if len(setOp.Tags) != 1 || setOp.Tags[0] != "config" {
		t.Errorf("expected tags [config] for config.set, got %v", setOp.Tags)
	}
	getOp := iface.Operations["config.get"]
	if len(getOp.Tags) != 1 || getOp.Tags[0] != "config" {
		t.Errorf("expected tags [config] for config.get, got %v", getOp.Tags)
	}
}

func TestConvertToInterface_TagsFromHierarchy(t *testing.T) {
	content := `
name "test-cli"
bin "test"

cmd "hello" help="Say hello"

cmd "admin" help="Admin commands" subcommand_required=#true {
  cmd "users" help="User commands" subcommand_required=#true {
    cmd "create" help="Create user"
    cmd "delete" help="Delete user"
  }
  cmd "reset" help="Reset system"
}
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.usage.kdl")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	iface, err := ConvertToInterface(ConvertParams{
		InputPath: specPath,
	})
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	// Root-level command: no tags.
	helloOp := iface.Operations["hello"]
	if len(helloOp.Tags) != 0 {
		t.Errorf("expected no tags for root-level 'hello', got %v", helloOp.Tags)
	}

	// Depth-2: single parent tag.
	resetOp := iface.Operations["admin.reset"]
	if len(resetOp.Tags) != 1 || resetOp.Tags[0] != "admin" {
		t.Errorf("expected tags [admin] for admin.reset, got %v", resetOp.Tags)
	}

	// Depth-3: two parent tags.
	createOp := iface.Operations["admin.users.create"]
	if len(createOp.Tags) != 2 || createOp.Tags[0] != "admin" || createOp.Tags[1] != "users" {
		t.Errorf("expected tags [admin, users] for admin.users.create, got %v", createOp.Tags)
	}

	deleteOp := iface.Operations["admin.users.delete"]
	if len(deleteOp.Tags) != 2 || deleteOp.Tags[0] != "admin" || deleteOp.Tags[1] != "users" {
		t.Errorf("expected tags [admin, users] for admin.users.delete, got %v", deleteOp.Tags)
	}
}

func TestConvertToInterface_CommandAliases(t *testing.T) {
	content := `
name "test-cli"
bin "test"

cmd "config" help="Config" {
  alias "cfg" "c"
}
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.usage.kdl")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	iface, err := ConvertToInterface(ConvertParams{
		InputPath: specPath,
	})
	if err != nil {
		t.Fatalf("conversion failed: %v", err)
	}

	op := iface.Operations["config"]
	if len(op.Aliases) != 2 {
		t.Errorf("expected 2 aliases, got %d: %v", len(op.Aliases), op.Aliases)
	}
}
