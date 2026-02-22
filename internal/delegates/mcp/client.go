// Package mcp implements the MCP (Model Context Protocol) binding format handler delegate.
//
// The MCP handler handles:
//   - Discovering tools, resources, and prompts from MCP servers
//   - Converting MCP entities to OpenBindings interfaces
//   - Executing operations via the MCP JSON-RPC protocol
//
// Only the Streamable HTTP transport is supported. Source locations must be
// HTTP or HTTPS URLs pointing to an MCP-capable endpoint.
package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultTimeout is the maximum time to wait for MCP server operations.
const DefaultTimeout = 30 * time.Second

// Discovery holds the results of discovering an MCP server's capabilities.
type Discovery struct {
	Tools             []*mcp.Tool
	Resources         []*mcp.Resource
	ResourceTemplates []*mcp.ResourceTemplate
	Prompts           []*mcp.Prompt
	ServerInfo        *mcp.Implementation
}

// ClientVersion can be set by the app layer to inject the CLI version
// into MCP client handshakes. Defaults to "0.0.0" if unset.
var ClientVersion = "0.0.0"

// clientInfo is the MCP client implementation identity sent during the
// initialize handshake. Built lazily from ClientVersion.
func clientInfo() *mcp.Implementation {
	return &mcp.Implementation{
		Name:    "ob",
		Version: ClientVersion,
	}
}

// Discover connects to an MCP server over Streamable HTTP, performs the
// initialization handshake, and paginates through tools/list, resources/list,
// and prompts/list. The url must be an HTTP or HTTPS URL.
func Discover(ctx context.Context, url string) (*Discovery, error) {
	session, err := connect(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	result := &Discovery{}

	// Get server info from the init result.
	initResult := session.InitializeResult()
	if initResult != nil {
		result.ServerInfo = initResult.ServerInfo
	}

	// List tools (paginated).
	if initResult != nil && initResult.Capabilities.Tools != nil {
		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list tools: %w", err)
			}
			result.Tools = append(result.Tools, tool)
		}
	}

	// List resources (paginated).
	if initResult != nil && initResult.Capabilities.Resources != nil {
		for resource, err := range session.Resources(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list resources: %w", err)
			}
			result.Resources = append(result.Resources, resource)
		}

		// List resource templates (paginated).
		for tmpl, err := range session.ResourceTemplates(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list resource templates: %w", err)
			}
			result.ResourceTemplates = append(result.ResourceTemplates, tmpl)
		}
	}

	// List prompts (paginated).
	if initResult != nil && initResult.Capabilities.Prompts != nil {
		for prompt, err := range session.Prompts(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list prompts: %w", err)
			}
			result.Prompts = append(result.Prompts, prompt)
		}
	}

	return result, nil
}

// CallTool connects to an MCP server and calls a tool by name.
func CallTool(ctx context.Context, url string, toolName string, args map[string]any) (*mcp.CallToolResult, error) {
	session, err := connect(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("call tool %q: %w", toolName, err)
	}
	return result, nil
}

// ReadResource connects to an MCP server and reads a resource by URI.
func ReadResource(ctx context.Context, url string, uri string) (*mcp.ReadResourceResult, error) {
	session, err := connect(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: uri,
	})
	if err != nil {
		return nil, fmt.Errorf("read resource %q: %w", uri, err)
	}
	return result, nil
}

// GetPrompt connects to an MCP server and gets a prompt by name.
func GetPrompt(ctx context.Context, url string, promptName string, args map[string]string) (*mcp.GetPromptResult, error) {
	session, err := connect(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer session.Close()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      promptName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("get prompt %q: %w", promptName, err)
	}
	return result, nil
}

// connect establishes a Streamable HTTP connection to an MCP server and
// performs the initialization handshake. Only HTTP and HTTPS URLs are accepted.
func connect(ctx context.Context, url string) (*mcp.ClientSession, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("MCP source location must be an HTTP or HTTPS URL, got %q", url)
	}

	transport := &mcp.StreamableClientTransport{Endpoint: url}
	client := mcp.NewClient(clientInfo(), nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return session, nil
}
