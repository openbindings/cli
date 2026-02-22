package app

import (
	"sync"

	"github.com/openbindings/cli/internal/delegates"
	asyncapihandler "github.com/openbindings/cli/internal/delegates/asyncapi"
	grpchandler "github.com/openbindings/cli/internal/delegates/grpc"
	mcphandler "github.com/openbindings/cli/internal/delegates/mcp"
	openapihandler "github.com/openbindings/cli/internal/delegates/openapi"
	usagehandler "github.com/openbindings/cli/internal/delegates/usage"
)

var (
	defaultRegistry     *delegates.Registry
	defaultRegistryOnce sync.Once
)

func init() {
	mcphandler.ClientVersion = OBVersion
}

// DefaultRegistry returns the singleton builtin handler registry.
// Builtin handlers are registered on first access.
func DefaultRegistry() *delegates.Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = delegates.NewRegistry()
		usagehandler.Register(defaultRegistry)
		mcphandler.Register(defaultRegistry)
		openapihandler.Register(defaultRegistry)
		asyncapihandler.Register(defaultRegistry)
		grpchandler.Register(defaultRegistry)
	})
	return defaultRegistry
}
