package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb" //nolint:staticcheck // required by jhump/protoreflect/dynamic
	"github.com/golang/protobuf/proto" //nolint:staticcheck // matches jhump/protoreflect return types
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/openbindings/cli/internal/delegates"
)

// ExecuteInput is the input for gRPC operation execution.
type ExecuteInput struct {
	Address string // host:port of the gRPC server
	Ref     string // fully qualified method ref (e.g., "blend.CoffeeShop/PlaceOrder")
	Input   any    // Operation input data (JSON-compatible map)
}

// ExecuteOutput is the output from gRPC operation execution.
type ExecuteOutput struct {
	Output     any    // Execution result
	Status     int    // 0 for success, 1 for error
	DurationMs int64  // Execution duration
	Error      *delegates.Error
}

// Execute invokes a gRPC method dynamically. It resolves the method descriptor
// via reflection, marshals JSON input to a protobuf message, invokes the RPC,
// and marshals the response back to JSON.
func Execute(ctx context.Context, input ExecuteInput) ExecuteOutput {
	start := time.Now()

	svcName, methodName, err := parseRef(input.Ref)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "invalid_ref", Message: err.Error()},
		}
	}

	conn, err := dial(ctx, input.Address)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "connect_failed", Message: err.Error()},
		}
	}
	defer conn.Close()

	refClient := grpcreflect.NewClientAuto(ctx, conn)
	defer refClient.Reset()

	svcDesc, err := refClient.ResolveService(svcName)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "resolve_failed", Message: fmt.Sprintf("resolve service %q: %v", svcName, err)},
		}
	}

	methodDesc := svcDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "method_not_found", Message: fmt.Sprintf("method %q not found in service %q", methodName, svcName)},
		}
	}

	reqMsg, err := buildRequest(methodDesc, input.Input)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "invalid_input", Message: err.Error()},
		}
	}

	stub := grpcdynamic.NewStub(conn)

	if methodDesc.IsServerStreaming() {
		stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
		if err != nil {
			return ExecuteOutput{
				Status:     1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      &delegates.Error{Code: "rpc_failed", Message: err.Error()},
			}
		}
		resp, err := stream.RecvMsg()
		if err != nil {
			return ExecuteOutput{
				Status:     1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      &delegates.Error{Code: "stream_recv_failed", Message: err.Error()},
			}
		}
		output, err := responseToJSON(resp)
		if err != nil {
			return ExecuteOutput{
				Status:     1,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      &delegates.Error{Code: "marshal_failed", Message: err.Error()},
			}
		}
		return ExecuteOutput{Output: output, DurationMs: time.Since(start).Milliseconds()}
	}

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "rpc_failed", Message: err.Error()},
		}
	}

	output, err := responseToJSON(resp)
	if err != nil {
		return ExecuteOutput{
			Status:     1,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      &delegates.Error{Code: "marshal_failed", Message: err.Error()},
		}
	}

	return ExecuteOutput{Output: output, DurationMs: time.Since(start).Milliseconds()}
}

// Subscribe opens a server-streaming RPC and returns events on a channel.
func Subscribe(ctx context.Context, input ExecuteInput) (<-chan delegates.StreamEvent, error) {
	svcName, methodName, err := parseRef(input.Ref)
	if err != nil {
		return nil, err
	}

	conn, err := dial(ctx, input.Address)
	if err != nil {
		return nil, err
	}

	refClient := grpcreflect.NewClientAuto(ctx, conn)

	// cleanup closes resources on error paths; nilled once the goroutine takes ownership.
	cleanup := func() {
		refClient.Reset()
		conn.Close()
	}
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	svcDesc, err := refClient.ResolveService(svcName)
	if err != nil {
		return nil, fmt.Errorf("resolve service %q: %w", svcName, err)
	}

	methodDesc := svcDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found in service %q", methodName, svcName)
	}

	if !methodDesc.IsServerStreaming() {
		return nil, fmt.Errorf("method %q is not server-streaming", input.Ref)
	}

	reqMsg, err := buildRequest(methodDesc, input.Input)
	if err != nil {
		return nil, err
	}

	stub := grpcdynamic.NewStub(conn)
	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	if err != nil {
		return nil, fmt.Errorf("invoke stream: %w", err)
	}

	ch := make(chan delegates.StreamEvent, 16)
	go func() {
		defer close(ch)
		defer refClient.Reset()
		defer conn.Close()

		for {
			resp, err := stream.RecvMsg()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				ch <- delegates.StreamEvent{Error: &delegates.Error{
					Code:    "stream_error",
					Message: err.Error(),
				}}
				return
			}

			output, err := responseToJSON(resp)
			if err != nil {
				ch <- delegates.StreamEvent{Error: &delegates.Error{
					Code:    "marshal_failed",
					Message: err.Error(),
				}}
				return
			}
			ch <- delegates.StreamEvent{Data: output}
		}
	}()

	cleanup = nil
	return ch, nil
}

// parseRef splits "package.Service/Method" into service and method names.
func parseRef(ref string) (string, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("empty gRPC ref")
	}
	idx := strings.LastIndex(ref, "/")
	if idx < 0 || idx == 0 || idx == len(ref)-1 {
		return "", "", fmt.Errorf("gRPC ref %q must be in the form package.Service/Method", ref)
	}
	return ref[:idx], ref[idx+1:], nil
}

func buildRequest(method *desc.MethodDescriptor, input any) (*dynamic.Message, error) {
	msg := dynamic.NewMessage(method.GetInputType())

	if input == nil {
		return msg, nil
	}

	inputMap, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("gRPC input must be a JSON object, got %T", input)
	}

	jsonBytes, err := json.Marshal(inputMap)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	if err := msg.UnmarshalJSONPB(&jsonpb.Unmarshaler{AllowUnknownFields: true}, jsonBytes); err != nil {
		return nil, fmt.Errorf("unmarshal input to protobuf: %w", err)
	}

	return msg, nil
}

func responseToJSON(resp proto.Message) (any, error) {
	dm, ok := resp.(*dynamic.Message)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T (expected *dynamic.Message)", resp)
	}
	jsonBytes, err := dm.MarshalJSONPB(&jsonpb.Marshaler{OrigName: true})
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}
	var result any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("parse response JSON: %w", err)
	}
	return result, nil
}
