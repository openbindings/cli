package mcp

import (
	"fmt"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

// Ref prefixes mirror the MCP JSON-RPC method namespaces.
const (
	RefPrefixTools     = "tools/"
	RefPrefixResources = "resources/"
	RefPrefixPrompts   = "prompts/"
)

// DefaultSourceName is the default source key for MCP sources.
const DefaultSourceName = "mcpServer"

// ConvertToInterface converts MCP discovery results to an OpenBindings interface.
func ConvertToInterface(discovery *Discovery, sourceLocation string) (openbindings.Interface, error) {
	if discovery == nil {
		return openbindings.Interface{}, fmt.Errorf("nil discovery result")
	}

	iface := openbindings.Interface{
		OpenBindings: openbindings.MaxTestedVersion,
		Operations:   map[string]openbindings.Operation{},
		Bindings:     map[string]openbindings.BindingEntry{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: {
				Format:   FormatToken,
				Location: sourceLocation,
			},
		},
	}

	// Set interface metadata from server info.
	if discovery.ServerInfo != nil {
		iface.Name = discovery.ServerInfo.Name
		iface.Version = discovery.ServerInfo.Version
		if discovery.ServerInfo.Title != "" {
			iface.Description = discovery.ServerInfo.Title
		}
	}

	// Track operation keys to detect collisions between tools and prompts.
	usedKeys := map[string]string{} // key -> entity type ("tool", "resource", "prompt")

	// Convert tools.
	for _, tool := range discovery.Tools {
		opKey := delegates.SanitizeKey(tool.Name)
		opKey = resolveKeyCollision(opKey, "tool", usedKeys)
		usedKeys[opKey] = "tool"

		// MCP display name precedence: title > annotations.title > name.
		// Use description if available; fall back to title for a human-readable label.
		desc := tool.Description
		if desc == "" {
			desc = tool.Title
		}

		op := openbindings.Operation{
			Kind:        openbindings.OperationKindMethod,
			Description: desc,
		}

		// Tool InputSchema is already JSON Schema (as map[string]any from client).
		if tool.InputSchema != nil {
			if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
				op.Input = schemaMap
			}
		}

		// Tool OutputSchema (optional, added in MCP 2025-06-18).
		if tool.OutputSchema != nil {
			if schemaMap, ok := tool.OutputSchema.(map[string]any); ok {
				op.Output = schemaMap
			}
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixTools + tool.Name,
		}
	}

	// Convert resources.
	for _, resource := range discovery.Resources {
		opKey := delegates.SanitizeKey(resource.Name)
		opKey = resolveKeyCollision(opKey, "resource", usedKeys)
		usedKeys[opKey] = "resource"

		// MCP display name precedence: title > annotations.title > name.
		desc := resource.Description
		if desc == "" {
			desc = resource.Title
		}

		op := openbindings.Operation{
			Kind:        openbindings.OperationKindMethod,
			Description: desc,
		}

		// Resources have a URI as input and return content.
		// Build a simple input schema with the resource URI as a constant.
		op.Input = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uri": map[string]any{
					"type":        "string",
					"const":       resource.URI,
					"description": "Resource URI",
				},
			},
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixResources + resource.URI,
		}
	}

	// Convert resource templates (parameterized resources with URI templates).
	for _, tmpl := range discovery.ResourceTemplates {
		opKey := delegates.SanitizeKey(tmpl.Name)
		opKey = resolveKeyCollision(opKey, "resource_template", usedKeys)
		usedKeys[opKey] = "resource_template"

		desc := tmpl.Description
		if desc == "" {
			desc = tmpl.Title
		}

		op := openbindings.Operation{
			Kind:        openbindings.OperationKindMethod,
			Description: desc,
		}

		// Build input schema with the URI template.
		op.Input = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uriTemplate": map[string]any{
					"type":        "string",
					"const":       tmpl.URITemplate,
					"description": "URI template (RFC 6570)",
				},
			},
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixResources + tmpl.URITemplate,
		}
	}

	// Convert prompts.
	for _, prompt := range discovery.Prompts {
		opKey := delegates.SanitizeKey(prompt.Name)
		opKey = resolveKeyCollision(opKey, "prompt", usedKeys)
		usedKeys[opKey] = "prompt"

		// MCP display name precedence: title > annotations.title > name.
		desc := prompt.Description
		if desc == "" {
			desc = prompt.Title
		}

		op := openbindings.Operation{
			Kind:        openbindings.OperationKindMethod,
			Description: desc,
		}

		// Build input schema from prompt arguments.
		if len(prompt.Arguments) > 0 {
			op.Input = promptArgsToSchema(prompt.Arguments)
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixPrompts + prompt.Name,
		}
	}

	return iface, nil
}

// promptArgsToSchema converts MCP prompt arguments to a JSON Schema.
func promptArgsToSchema(args []*gomcp.PromptArgument) map[string]any {
	properties := map[string]any{}
	var required []string

	for _, arg := range args {
		prop := map[string]any{
			"type": "string",
		}
		if arg.Description != "" {
			prop["description"] = arg.Description
		}
		properties[arg.Name] = prop

		if arg.Required {
			required = append(required, arg.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// resolveKeyCollision handles the case where a tool and prompt (or other entities)
// share the same name. The first entity to claim a key wins the unprefixed name;
// subsequent colliders are prefixed with their entity type (e.g., "prompt_echo").
//
// Processing order is tools → resources → resource templates → prompts, so tools
// win unprefixed names by convention.
func resolveKeyCollision(key string, entityType string, used map[string]string) string {
	if _, taken := used[key]; !taken {
		return key
	}
	candidate := entityType + "_" + key
	if _, taken := used[candidate]; !taken {
		return candidate
	}
	for i := 2; ; i++ {
		numbered := fmt.Sprintf("%s_%d", candidate, i)
		if _, taken := used[numbered]; !taken {
			return numbered
		}
	}
}
