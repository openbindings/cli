package mcp

import (
	"context"
	"fmt"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"github.com/openbindings/openbindings-go/canonicaljson"
)

// FormatToken is the format identifier for MCP sources.
// Targets the 2025-11-25 MCP spec revision. Supported features:
//   - tools/list, tools/call (incl. structuredContent and outputSchema)
//   - resources/list, resources/read
//   - resources/templates/list
//   - prompts/list, prompts/get
//
// Not yet supported: resource subscriptions, sampling, icons, elicitation.
const FormatToken = "mcp@2025-11-25"

// Handler implements the MCP binding format handler delegate.
type Handler struct{}

// New creates a new MCP handler.
func New() *Handler {
	return &Handler{}
}

// GetInfo returns identity and metadata about this delegate.
func (h *Handler) GetInfo() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "Model Context Protocol",
		Description: "MCP servers exposing tools, resources, and prompts via JSON-RPC",
	}
}

// ListFormats returns the binding formats this delegate supports.
func (h *Handler) ListFormats() []delegates.FormatInfo {
	return []delegates.FormatInfo{
		{
			Token:       FormatToken,
			Description: "MCP servers (tools, resources, prompts) via Streamable HTTP transport",
		},
	}
}

// CreateInterface discovers an MCP server's capabilities and converts them
// to an OpenBindings interface.
func (h *Handler) CreateInterface(source delegates.Source) (openbindings.Interface, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, iface, err := h.discoverAndConvert(ctx, source)
	return iface, err
}

// ExecuteOperation executes an MCP operation via the appropriate JSON-RPC method.
func (h *Handler) ExecuteOperation(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	result := Execute(ctx, ExecuteInput{
		URL:   input.Source.Location,
		Ref:   input.Ref,
		Input: input.Input,
	})

	return delegates.ExecuteOutput{
		Output:     result.Output,
		Status:     result.Status,
		DurationMs: result.DurationMs,
		Error:      result.Error,
	}
}

// DiscoverSource implements the delegates.SourceDiscoverer interface.
// It connects to the MCP server once and returns both the raw content
// (for hashing/drift detection) and the derived interface (for merge).
func (h *Handler) DiscoverSource(ctx context.Context, source delegates.Source) ([]byte, openbindings.Interface, error) {
	return h.discoverAndConvert(ctx, source)
}

// discoverAndConvert connects to an MCP server, discovers capabilities,
// converts them to an OpenBindings interface, and serializes the discovery
// data for content hashing. Shared by CreateInterface and DiscoverSource.
func (h *Handler) discoverAndConvert(ctx context.Context, source delegates.Source) ([]byte, openbindings.Interface, error) {
	url := source.Location
	if url == "" {
		return nil, openbindings.Interface{}, fmt.Errorf("MCP source requires an HTTP or HTTPS URL")
	}

	discovery, err := Discover(ctx, url)
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("MCP discovery: %w", err)
	}

	iface, err := ConvertToInterface(discovery, source.Location)
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("MCP convert: %w", err)
	}

	content, err := canonicalDiscoveryJSON(discovery)
	if err != nil {
		return nil, openbindings.Interface{}, fmt.Errorf("MCP serialize discovery: %w", err)
	}

	return content, iface, nil
}

// canonicalDiscoveryJSON produces a deterministic JSON representation of the
// discovery data for content hashing and drift detection.
//
// We explicitly select fields rather than marshaling full SDK structs so that:
//   - Hashes are deterministic across SDK versions (internal SDK fields don't leak in).
//   - Only semantically meaningful fields contribute to drift detection.
//
// When new MCP fields are added to the spec, they should be added here too.
func canonicalDiscoveryJSON(discovery *Discovery) ([]byte, error) {
	data := map[string]any{}

	if discovery.ServerInfo != nil {
		info := map[string]any{
			"name":    discovery.ServerInfo.Name,
			"version": discovery.ServerInfo.Version,
		}
		if discovery.ServerInfo.Title != "" {
			info["title"] = discovery.ServerInfo.Title
		}
		data["serverInfo"] = info
	}

	if len(discovery.Tools) > 0 {
		tools := make([]any, len(discovery.Tools))
		for i, t := range discovery.Tools {
			entry := map[string]any{
				"name": t.Name,
			}
			if t.Title != "" {
				entry["title"] = t.Title
			}
			if t.Description != "" {
				entry["description"] = t.Description
			}
			if t.InputSchema != nil {
				entry["inputSchema"] = t.InputSchema
			}
			if t.OutputSchema != nil {
				entry["outputSchema"] = t.OutputSchema
			}
			tools[i] = entry
		}
		data["tools"] = tools
	}

	if len(discovery.Resources) > 0 {
		resources := make([]any, len(discovery.Resources))
		for i, r := range discovery.Resources {
			entry := map[string]any{
				"name": r.Name,
				"uri":  r.URI,
			}
			if r.Title != "" {
				entry["title"] = r.Title
			}
			if r.Description != "" {
				entry["description"] = r.Description
			}
			if r.MIMEType != "" {
				entry["mimeType"] = r.MIMEType
			}
			resources[i] = entry
		}
		data["resources"] = resources
	}

	if len(discovery.ResourceTemplates) > 0 {
		templates := make([]any, len(discovery.ResourceTemplates))
		for i, tmpl := range discovery.ResourceTemplates {
			entry := map[string]any{
				"name":        tmpl.Name,
				"uriTemplate": tmpl.URITemplate,
			}
			if tmpl.Title != "" {
				entry["title"] = tmpl.Title
			}
			if tmpl.Description != "" {
				entry["description"] = tmpl.Description
			}
			if tmpl.MIMEType != "" {
				entry["mimeType"] = tmpl.MIMEType
			}
			templates[i] = entry
		}
		data["resourceTemplates"] = templates
	}

	if len(discovery.Prompts) > 0 {
		prompts := make([]any, len(discovery.Prompts))
		for i, p := range discovery.Prompts {
			entry := map[string]any{
				"name": p.Name,
			}
			if p.Title != "" {
				entry["title"] = p.Title
			}
			if p.Description != "" {
				entry["description"] = p.Description
			}
			if len(p.Arguments) > 0 {
				args := make([]any, len(p.Arguments))
				for j, a := range p.Arguments {
					arg := map[string]any{
						"name": a.Name,
					}
					if a.Description != "" {
						arg["description"] = a.Description
					}
					if a.Required {
						arg["required"] = true
					}
					args[j] = arg
				}
				entry["arguments"] = args
			}
			prompts[i] = entry
		}
		data["prompts"] = prompts
	}

	return canonicaljson.Marshal(data)
}

// Register registers the MCP handler with a registry.
func Register(r *delegates.Registry) {
	r.Register(New())
}
