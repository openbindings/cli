package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/openbindings/cli/internal/app"
	openbindings "github.com/openbindings/openbindings-go"
)

// ValidationStatus represents the validation state of an input.
type ValidationStatus string

const (
	// ValidationUnknown means no schema available or validation not attempted.
	ValidationUnknown ValidationStatus = app.ValidationStatusUnknown
	// ValidationValid means the input conforms to the schema.
	ValidationValid ValidationStatus = app.ValidationStatusValid
	// ValidationInvalid means the input does not conform to the schema.
	ValidationInvalid ValidationStatus = app.ValidationStatusInvalid
	// ValidationError means validation could not be performed (e.g., parse error).
	ValidationError ValidationStatus = app.ValidationStatusError
)

// ValidationResult holds the result of validating an input against a schema.
type ValidationResult struct {
	Status  ValidationStatus
	Message string // Human-readable validation error (empty if valid)
}

// ValidateInputFile validates an input file against an operation's input schema.
func ValidateInputFile(path string, op openbindings.Operation, iface *openbindings.Interface) ValidationResult {
	// No schema defined - can't validate
	if op.Input == nil {
		return ValidationResult{Status: ValidationUnknown, Message: "no schema defined"}
	}

	// Read and parse the input file
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{Status: ValidationError, Message: fmt.Sprintf("read error: %v", err)}
	}

	var inputData any
	if err := json.Unmarshal(data, &inputData); err != nil {
		return ValidationResult{Status: ValidationError, Message: fmt.Sprintf("JSON parse error: %v", err)}
	}

	// Resolve the schema (handle $ref)
	schema := resolveSchemaFully(op.Input, iface)
	if schema == nil {
		return ValidationResult{Status: ValidationUnknown, Message: "could not resolve schema"}
	}

	// Compile and validate
	return validateAgainstSchema(inputData, schema, iface)
}

// ValidateInputData validates input data (already parsed) against an operation's schema.
func ValidateInputData(inputData any, op openbindings.Operation, iface *openbindings.Interface) ValidationResult {
	if op.Input == nil {
		return ValidationResult{Status: ValidationUnknown, Message: "no schema defined"}
	}

	schema := resolveSchemaFully(op.Input, iface)
	if schema == nil {
		return ValidationResult{Status: ValidationUnknown, Message: "could not resolve schema"}
	}

	return validateAgainstSchema(inputData, schema, iface)
}

// validateAgainstSchema performs JSON Schema validation.
func validateAgainstSchema(data any, schema map[string]any, iface *openbindings.Interface) ValidationResult {
	// Build a complete schema document with definitions for $ref resolution
	schemaDoc := buildSchemaDocument(schema, iface)

	// Marshal to JSON for the validator
	schemaBytes, err := json.Marshal(schemaDoc)
	if err != nil {
		return ValidationResult{Status: ValidationError, Message: fmt.Sprintf("schema marshal error: %v", err)}
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(string(schemaBytes))); err != nil {
		return ValidationResult{Status: ValidationError, Message: fmt.Sprintf("schema compile error: %v", err)}
	}

	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return ValidationResult{Status: ValidationError, Message: fmt.Sprintf("schema compile error: %v", err)}
	}

	// Validate
	if err := compiled.Validate(data); err != nil {
		// Extract a human-readable error message
		msg := extractValidationError(err)
		return ValidationResult{Status: ValidationInvalid, Message: msg}
	}

	return ValidationResult{Status: ValidationValid}
}

// buildSchemaDocument creates a complete JSON Schema document with $defs for reference resolution.
func buildSchemaDocument(schema map[string]any, iface *openbindings.Interface) map[string]any {
	// Deep copy and transform the schema to use $defs instead of #/schemas/
	transformed := transformSchemaRefs(schema)
	doc, ok := transformed.(map[string]any)
	if !ok {
		doc = make(map[string]any)
		for k, v := range schema {
			doc[k] = v
		}
	}

	// Add $defs from interface schemas for $ref resolution
	if iface != nil && len(iface.Schemas) > 0 {
		defs := make(map[string]any)
		for name, s := range iface.Schemas {
			// Also transform nested schemas
			defs[name] = transformSchemaRefs(s)
		}
		doc["$defs"] = defs
	}

	return doc
}

// transformSchemaRefs recursively transforms #/schemas/ references to #/$defs/ for JSON Schema compatibility.
func transformSchemaRefs(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			if k == "$ref" {
				if refStr, ok := v.(string); ok && strings.HasPrefix(refStr, "#/schemas/") {
					// Transform #/schemas/Foo to #/$defs/Foo
					result[k] = "#/$defs/" + strings.TrimPrefix(refStr, "#/schemas/")
					continue
				}
			}
			result[k] = transformSchemaRefs(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = transformSchemaRefs(item)
		}
		return result
	default:
		return v
	}
}

// resolveSchemaFully resolves $ref references in a schema, returning the fully resolved schema.
func resolveSchemaFully(schema map[string]any, iface *openbindings.Interface) map[string]any {
	if schema == nil {
		return nil
	}

	// Check for $ref
	if ref, ok := schema["$ref"].(string); ok {
		// Parse the reference (e.g., "#/schemas/CreateInterfaceInput")
		if strings.HasPrefix(ref, "#/schemas/") {
			schemaName := strings.TrimPrefix(ref, "#/schemas/")
			if iface != nil && iface.Schemas != nil {
				if resolved, ok := iface.Schemas[schemaName]; ok {
					return resolved
				}
			}
		}
		// Can't resolve - return original with $ref (validator may handle it)
		return schema
	}

	return schema
}

// extractValidationError extracts a concise error message from validation errors.
func extractValidationError(err error) string {
	if err == nil {
		return ""
	}

	// The jsonschema library returns detailed errors
	// Extract the first/most relevant one
	errStr := err.Error()

	// Try to extract the key information
	// Typical format: "jsonschema: '/foo' does not validate with schema.json#/properties/foo: missing properties: 'bar'"
	if strings.Contains(errStr, "missing properties:") {
		// Extract missing properties
		idx := strings.Index(errStr, "missing properties:")
		if idx >= 0 {
			return "Missing required: " + strings.TrimSpace(errStr[idx+len("missing properties:"):])
		}
	}

	if strings.Contains(errStr, "expected") {
		// Type mismatch
		parts := strings.Split(errStr, ":")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// Fallback: return first line, truncated
	lines := strings.Split(errStr, "\n")
	msg := lines[0]
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}
	return msg
}
