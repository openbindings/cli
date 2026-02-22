package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Discovery holds the results of reflecting a gRPC server.
type Discovery struct {
	Services []*desc.ServiceDescriptor
	Address  string
}

// ToCanonical returns a deterministic representation for content hashing.
func (d *Discovery) ToCanonical() map[string]any {
	data := map[string]any{
		"address": d.Address,
	}
	var services []any
	for _, svc := range d.Services {
		var methods []any
		for _, m := range svc.GetMethods() {
			methods = append(methods, map[string]any{
				"name":            m.GetName(),
				"serverStreaming": m.IsServerStreaming(),
				"clientStreaming": m.IsClientStreaming(),
			})
		}
		services = append(services, map[string]any{
			"name":    svc.GetFullyQualifiedName(),
			"methods": methods,
		})
	}
	data["services"] = services
	return data
}

// Discover connects to a gRPC server using reflection, enumerates all
// services and their method descriptors. Standard gRPC infrastructure
// services (grpc.reflection, grpc.health) are excluded.
func Discover(ctx context.Context, address string) (*Discovery, error) {
	conn, err := dial(ctx, address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	refClient := grpcreflect.NewClientAuto(ctx, conn)
	defer refClient.Reset()

	serviceNames, err := refClient.ListServices()
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	disc := &Discovery{Address: address}
	for _, name := range serviceNames {
		if isInfraService(name) {
			continue
		}
		svcDesc, err := refClient.ResolveService(name)
		if err != nil {
			return nil, fmt.Errorf("resolve service %q: %w", name, err)
		}
		disc.Services = append(disc.Services, svcDesc)
	}

	return disc, nil
}

// dial creates a gRPC client connection with appropriate transport credentials.
// Addresses ending in :443 or containing "://" with https use TLS; otherwise plaintext.
func dial(ctx context.Context, address string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if needsTLS(address) {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	target := address
	if !strings.Contains(address, "://") {
		target = "dns:///" + address
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %q: %w", address, err)
	}
	return conn, nil
}

func needsTLS(address string) bool {
	if strings.HasSuffix(address, ":443") {
		return true
	}
	if strings.HasPrefix(address, "https://") {
		return true
	}
	return false
}

func isInfraService(name string) bool {
	return strings.HasPrefix(name, "grpc.reflection.") ||
		strings.HasPrefix(name, "grpc.health.")
}
