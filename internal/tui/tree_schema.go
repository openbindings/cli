package tui

import (
	"fmt"
	"sort"
	"strings"

	openbindings "github.com/openbindings/openbindings-go"
)

// SchemaTreeBuilder builds tree nodes from JSON Schema definitions.
type SchemaTreeBuilder struct {
	Schemas map[string]openbindings.JSONSchema // Interface schemas for $ref resolution
	// visiting tracks $ref paths currently being expanded to prevent infinite recursion
	// on circular schema references (e.g., recursive tree types).
	visiting map[string]bool
}

// BuildSchemaTree creates a tree representation of a JSON Schema.
func (b *SchemaTreeBuilder) BuildSchemaTree(schema openbindings.JSONSchema, label string, idPrefix string) *TreeNode {
	if len(schema) == 0 {
		return nil
	}

	node := &TreeNode{
		ID:    idPrefix,
		Label: label,
		Type:  NodeTypeSchema,
	}

	// Handle $ref (with cycle detection to prevent infinite recursion)
	if ref, ok := schema["$ref"].(string); ok {
		node.Badge = b.refToType(ref)
		node.Type = NodeTypeSchemaRef
		// Try to resolve and expand the referenced schema, but only if
		// we haven't already started expanding this ref (cycle detection).
		if !b.visiting[ref] {
			if resolved := b.resolveRef(ref); resolved != nil {
				if b.visiting == nil {
					b.visiting = make(map[string]bool)
				}
				b.visiting[ref] = true
				node.Children = b.buildSchemaChildren(resolved, idPrefix)
				delete(b.visiting, ref)
			}
		}
		return node
	}

	// Handle type
	typeStr := b.schemaTypeString(schema)
	node.Badge = typeStr

	// Build children based on type
	node.Children = b.buildSchemaChildren(schema, idPrefix)

	return node
}

// buildSchemaChildren builds child nodes for a schema.
func (b *SchemaTreeBuilder) buildSchemaChildren(schema openbindings.JSONSchema, idPrefix string) []*TreeNode {
	var children []*TreeNode

	schemaType, _ := schema["type"].(string)

	switch schemaType {
	case "object":
		children = append(children, b.buildObjectChildren(schema, idPrefix)...)
	case "array":
		if items := b.buildArrayItems(schema, idPrefix); items != nil {
			children = append(children, items)
		}
	}

	// Handle anyOf/oneOf/allOf
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		if variants, ok := schema[keyword].([]any); ok {
			for i, v := range variants {
				if variantSchema, ok := v.(map[string]any); ok {
					child := b.BuildSchemaTree(variantSchema, fmt.Sprintf("variant %d", i+1), fmt.Sprintf("%s.%s.%d", idPrefix, keyword, i))
					if child != nil {
						children = append(children, child)
					}
				}
			}
		}
	}

	return children
}

// buildObjectChildren builds child nodes for object properties.
func (b *SchemaTreeBuilder) buildObjectChildren(schema openbindings.JSONSchema, idPrefix string) []*TreeNode {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	// Get required fields
	requiredSet := make(map[string]bool)
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	// Sort property names for stable ordering
	var propNames []string
	for name := range props {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	var children []*TreeNode
	for _, name := range propNames {
		propSchema, ok := props[name].(map[string]any)
		if !ok {
			continue
		}

		child := b.BuildSchemaTree(propSchema, name, fmt.Sprintf("%s.%s", idPrefix, name))
		if child == nil {
			child = &TreeNode{
				ID:    fmt.Sprintf("%s.%s", idPrefix, name),
				Label: name,
				Type:  NodeTypeSchemaProp,
			}
		}

		// Mark required fields
		if requiredSet[name] {
			child.Badge = child.Badge + " *"
		}

		// Add description if present
		if desc, ok := propSchema["description"].(string); ok {
			child.Data = desc
		}

		children = append(children, child)
	}

	return children
}

// buildArrayItems builds a child node for array items.
func (b *SchemaTreeBuilder) buildArrayItems(schema openbindings.JSONSchema, idPrefix string) *TreeNode {
	items, ok := schema["items"].(map[string]any)
	if !ok {
		return nil
	}

	child := b.BuildSchemaTree(items, "items", fmt.Sprintf("%s.items", idPrefix))
	if child != nil {
		child.Label = "items"
	}
	return child
}

// schemaTypeString returns a human-readable type string for a schema.
func (b *SchemaTreeBuilder) schemaTypeString(schema openbindings.JSONSchema) string {
	// Check for $ref first
	if ref, ok := schema["$ref"].(string); ok {
		return b.refToType(ref)
	}

	// Check type
	if t, ok := schema["type"].(string); ok {
		switch t {
		case "array":
			// Try to get item type
			if items, ok := schema["items"].(map[string]any); ok {
				itemType := b.schemaTypeString(items)
				return fmt.Sprintf("array<%s>", itemType)
			}
			return "array"
		case "object":
			return "object"
		default:
			return t
		}
	}

	// Check for anyOf/oneOf/allOf
	if _, ok := schema["anyOf"]; ok {
		return "anyOf"
	}
	if _, ok := schema["oneOf"]; ok {
		return "oneOf"
	}
	if _, ok := schema["allOf"]; ok {
		return "allOf"
	}

	// Check for const/enum
	if _, ok := schema["const"]; ok {
		return "const"
	}
	if _, ok := schema["enum"]; ok {
		return "enum"
	}

	return "any"
}

// refToType converts a $ref to a display type name.
func (b *SchemaTreeBuilder) refToType(ref string) string {
	// Handle local refs like "#/schemas/Foo"
	if strings.HasPrefix(ref, "#/schemas/") {
		return strings.TrimPrefix(ref, "#/schemas/")
	}
	if strings.HasPrefix(ref, "#/$defs/") {
		return strings.TrimPrefix(ref, "#/$defs/")
	}
	// Return the ref as-is for external refs
	return ref
}

// resolveRef resolves a $ref to its schema definition.
func (b *SchemaTreeBuilder) resolveRef(ref string) openbindings.JSONSchema {
	if b.Schemas == nil {
		return nil
	}

	// Handle local refs
	if strings.HasPrefix(ref, "#/schemas/") {
		name := strings.TrimPrefix(ref, "#/schemas/")
		if schema, ok := b.Schemas[name]; ok {
			return schema
		}
	}

	return nil
}
