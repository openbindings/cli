// Package asyncapi implements the AsyncAPI binding format handler delegate.
//
// This package provides lightweight AsyncAPI 3.0 document types sufficient for
// converting AsyncAPI specs to OpenBindings interfaces and executing operations
// via SSE and WebSocket transports.
package asyncapi

// Document represents an AsyncAPI 3.0 document.
// Only the fields needed for OpenBindings conversion are modeled.
type Document struct {
	AsyncAPI   string                `json:"asyncapi" yaml:"asyncapi"`
	Info       Info                  `json:"info" yaml:"info"`
	Servers    map[string]Server     `json:"servers,omitempty" yaml:"servers,omitempty"`
	Channels   map[string]Channel    `json:"channels,omitempty" yaml:"channels,omitempty"`
	Operations map[string]Operation  `json:"operations,omitempty" yaml:"operations,omitempty"`
	Components *Components           `json:"components,omitempty" yaml:"components,omitempty"`
}

// Info contains metadata about the API.
type Info struct {
	Title       string `json:"title" yaml:"title"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Server describes a message broker or transport endpoint.
type Server struct {
	Host        string            `json:"host" yaml:"host"`
	Protocol    string            `json:"protocol" yaml:"protocol"`
	PathName    string            `json:"pathname,omitempty" yaml:"pathname,omitempty"`
	Description string            `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []Tag             `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// Channel represents a communication channel.
type Channel struct {
	Address     string              `json:"address,omitempty" yaml:"address,omitempty"`
	Messages    map[string]Message  `json:"messages,omitempty" yaml:"messages,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Servers     []ServerRef         `json:"servers,omitempty" yaml:"servers,omitempty"`
	Parameters  map[string]Parameter `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Ref         string              `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// Operation describes an action performed on a channel.
// In AsyncAPI 3.0, action is "send" or "receive".
type Operation struct {
	Action      string              `json:"action" yaml:"action"`
	Channel     ChannelRef          `json:"channel" yaml:"channel"`
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string              `json:"description,omitempty" yaml:"description,omitempty"`
	Messages    []MessageRef        `json:"messages,omitempty" yaml:"messages,omitempty"`
	Tags        []Tag               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Reply       *OperationReply     `json:"reply,omitempty" yaml:"reply,omitempty"`
}

// OperationReply describes a reply to an operation (request/reply pattern).
type OperationReply struct {
	Channel  *ChannelRef  `json:"channel,omitempty" yaml:"channel,omitempty"`
	Messages []MessageRef `json:"messages,omitempty" yaml:"messages,omitempty"`
}

// Message describes a message payload.
type Message struct {
	Name        string         `json:"name,omitempty" yaml:"name,omitempty"`
	Title       string         `json:"title,omitempty" yaml:"title,omitempty"`
	Summary     string         `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	ContentType string         `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Payload     map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
	Ref         string         `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// ChannelRef is a reference to a channel.
type ChannelRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// MessageRef is a reference to a message.
type MessageRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// ServerRef is a reference to a server.
type ServerRef struct {
	Ref string `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// Parameter describes a channel parameter.
type Parameter struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// Tag is a metadata tag.
type Tag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Components holds reusable document objects.
type Components struct {
	Messages map[string]Message `json:"messages,omitempty" yaml:"messages,omitempty"`
	Schemas  map[string]any     `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	Channels map[string]Channel `json:"channels,omitempty" yaml:"channels,omitempty"`
}
