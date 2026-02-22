// Package usage - execute_test.go contains tests for operation execution.
package usage

import (
	"testing"

	"github.com/openbindings/usage-go/usage"
)

func TestBuildCLIArgs_BasicFlags(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-v --verbose", Help: "Verbose"},
			{Usage: "-o --output <file>", Help: "Output file"},
		},
	}

	input := map[string]any{
		"verbose": true,
		"output":  "out.txt",
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check args contain expected flags
	hasVerbose := false
	hasOutput := false
	for i, arg := range args {
		if arg == "--verbose" {
			hasVerbose = true
		}
		if arg == "--output" && i+1 < len(args) && args[i+1] == "out.txt" {
			hasOutput = true
		}
	}

	if !hasVerbose {
		t.Error("expected --verbose in args")
	}
	if !hasOutput {
		t.Error("expected --output out.txt in args")
	}
}

func TestBuildCLIArgs_BooleanFlagFalse(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-v --verbose", Help: "Verbose"},
		},
	}

	input := map[string]any{
		"verbose": false,
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Boolean false should not add the flag
	for _, arg := range args {
		if arg == "--verbose" {
			t.Error("boolean false should not include --verbose")
		}
	}
}

func TestBuildCLIArgs_NegateFlag(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--color --no-color", Help: "Colorize", Negate: "--no-color"},
		},
	}

	// When false, should use negate form
	input := map[string]any{
		"color": false,
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasNoColor := false
	for _, arg := range args {
		if arg == "--no-color" {
			hasNoColor = true
		}
		if arg == "--color" {
			t.Error("should use --no-color for false value")
		}
	}

	if !hasNoColor {
		t.Error("expected --no-color in args")
	}
}

func TestBuildCLIArgs_CountFlag(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-v --verbose", Help: "Verbosity", Count: true},
		},
	}

	input := map[string]any{
		"verbose": float64(3), // JSON numbers come as float64
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have --verbose repeated 3 times (uses long form from input key)
	count := 0
	for _, arg := range args {
		if arg == "--verbose" {
			count++
		}
	}

	if count != 3 {
		t.Errorf("expected 3 instances of --verbose, got %d: %v", count, args)
	}
}

func TestBuildCLIArgs_VariadicFlag(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "-e --exclude <pattern>", Help: "Exclude patterns", Var: true},
		},
	}

	input := map[string]any{
		"exclude": []any{"*.log", "*.tmp"},
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have --exclude for each value
	expected := []string{"--exclude", "*.log", "--exclude", "*.tmp"}
	if !sliceContainsSequence(args, expected) {
		t.Errorf("expected %v in args, got %v", expected, args)
	}
}

func TestBuildCLIArgs_PositionalArgs(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<source>", Help: "Source file"},
			{Name: "<dest>", Help: "Destination file"},
		},
	}

	input := map[string]any{
		"source": "input.txt",
		"dest":   "output.txt",
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Positional args should be at the end in order
	if len(args) < 2 {
		t.Fatalf("expected at least 2 args, got %d", len(args))
	}

	if args[len(args)-2] != "input.txt" || args[len(args)-1] != "output.txt" {
		t.Errorf("expected positional args at end, got %v", args)
	}
}

func TestBuildCLIArgs_DoubleDash(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<file>", Help: "File", DoubleDash: "required"},
		},
	}

	input := map[string]any{
		"file": "-special-file.txt",
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have -- before positional args
	hasDash := false
	for i, arg := range args {
		if arg == "--" {
			hasDash = true
			if i+1 < len(args) && args[i+1] != "-special-file.txt" {
				t.Error("expected positional arg after --")
			}
		}
	}

	if !hasDash {
		t.Error("expected -- in args for DoubleDash arg")
	}
}

func TestBuildCLIArgs_VariadicArg(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Args: []usage.Arg{
			{Name: "<files>...", Help: "Files"},
		},
	}

	input := map[string]any{
		"files": []any{"a.txt", "b.txt", "c.txt"},
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Args should include command path + 3 file args
	// The "test" command path is first, then the 3 files
	if len(args) != 4 {
		t.Errorf("expected 4 args (1 cmd + 3 files), got %d: %v", len(args), args)
	}
}

func TestBuildCLIArgs_InheritedGlobals(t *testing.T) {
	cmd := &usage.Command{
		Name: "test",
		Flags: []usage.Flag{
			{Usage: "--local", Help: "Local flag"},
		},
	}

	inheritedGlobals := []usage.Flag{
		{Usage: "-v --verbose", Help: "Global verbose", Global: true},
	}

	input := map[string]any{
		"local":   true,
		"verbose": true,
	}

	args, err := buildCLIArgs([]string{"test"}, cmd, inheritedGlobals, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasLocal := false
	hasVerbose := false
	for _, arg := range args {
		if arg == "--local" {
			hasLocal = true
		}
		if arg == "--verbose" {
			hasVerbose = true
		}
	}

	if !hasLocal {
		t.Error("expected --local in args")
	}
	if !hasVerbose {
		t.Error("expected --verbose (inherited global) in args")
	}
}

// Helper to check if slice contains a sequence
func sliceContainsSequence(slice []string, seq []string) bool {
	if len(seq) == 0 {
		return true
	}
	if len(slice) < len(seq) {
		return false
	}

outer:
	for i := 0; i <= len(slice)-len(seq); i++ {
		for j := 0; j < len(seq); j++ {
			if slice[i+j] != seq[j] {
				continue outer
			}
		}
		return true
	}
	return false
}
