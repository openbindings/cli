package asyncapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/openbindings/cli/internal/delegates"
)

const defaultTimeout = 30 * time.Second

// parseSSEPayload joins accumulated data lines and attempts JSON parsing.
func parseSSEPayload(dataLines []string) any {
	raw := strings.Join(dataLines, "\n")
	var parsed any
	if json.Unmarshal([]byte(raw), &parsed) == nil {
		return parsed
	}
	return raw
}

// Execute dispatches an AsyncAPI operation based on the ref and protocol.
func Execute(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	start := time.Now()

	doc, err := loadDocument(input.Source)
	if err != nil {
		return delegates.FailedOutput(start, "doc_load_failed", err.Error())
	}

	opID, err := parseRef(input.Ref)
	if err != nil {
		return delegates.FailedOutput(start, "invalid_ref", err.Error())
	}

	asyncOp, ok := doc.Operations[opID]
	if !ok {
		return delegates.FailedOutput(start, "operation_not_found", fmt.Sprintf("operation %q not in AsyncAPI doc", opID))
	}

	serverURL, protocol, err := resolveServer(doc, input.Context)
	if err != nil {
		return delegates.FailedOutput(start, "no_server", err.Error())
	}

	channelName := extractRefName(asyncOp.Channel.Ref)
	channel, hasChannel := doc.Channels[channelName]

	address := channelName
	if hasChannel && channel.Address != "" {
		address = channel.Address
	}

	switch asyncOp.Action {
	case "receive":
		return executeReceive(ctx, serverURL, protocol, address, input, start)
	case "send":
		return executeSend(ctx, serverURL, protocol, address, input, start)
	default:
		return delegates.FailedOutput(start, "unsupported_action", fmt.Sprintf("unknown action %q", asyncOp.Action))
	}
}

// parseRef extracts the operation ID from an AsyncAPI ref.
// e.g., "#/operations/onTick" → "onTick"
func parseRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty ref")
	}

	const prefix = "#/operations/"
	if strings.HasPrefix(ref, prefix) {
		opID := strings.TrimPrefix(ref, prefix)
		if opID == "" {
			return "", fmt.Errorf("empty operation ID in ref %q", ref)
		}
		return opID, nil
	}

	return ref, nil
}

// resolveServer determines the server URL and protocol from the document or context.
func resolveServer(doc *Document, bindCtx *delegates.BindingContext) (url string, protocol string, err error) {
	if bindCtx != nil && bindCtx.Metadata != nil {
		if base, ok := bindCtx.Metadata["baseURL"].(string); ok && base != "" {
			proto := "http"
			if strings.HasPrefix(base, "wss://") || strings.HasPrefix(base, "ws://") {
				proto = "ws"
			}
			return strings.TrimRight(base, "/"), proto, nil
		}
	}

	// Sort server names for deterministic selection.
	serverNames := make([]string, 0, len(doc.Servers))
	for name := range doc.Servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	for _, name := range serverNames {
		server := doc.Servers[name]
		proto := strings.ToLower(server.Protocol)
		host := server.Host
		pathname := server.PathName

		switch proto {
		case "http", "https", "ws", "wss":
			url := proto + "://" + host
			if pathname != "" {
				url += pathname
			}
			return strings.TrimRight(url, "/"), proto, nil
		}
	}

	return "", "", fmt.Errorf("no supported server found (need http, https, ws, or wss protocol)")
}

// executeReceive handles "receive" operations (subscribing to events).
// Supports SSE (HTTP) protocol. Reads a configurable number of events.
func executeReceive(ctx context.Context, serverURL, protocol, address string, input delegates.ExecuteInput, start time.Time) delegates.ExecuteOutput {
	maxEvents := 1
	if input.Input != nil {
		if m, ok := input.Input.(map[string]any); ok {
			if n, ok := m["maxEvents"].(float64); ok && n > 0 {
				maxEvents = int(n)
			}
		}
	}

	switch protocol {
	case "http", "https":
		return executeSSESubscribe(ctx, serverURL, address, maxEvents, input, start)
	default:
		return delegates.FailedOutput(start, "unsupported_protocol",
			fmt.Sprintf("receive not supported for protocol %q (supported: http, https)", protocol))
	}
}

// executeSend handles "send" operations (publishing messages).
// Supports HTTP protocol via POST.
func executeSend(ctx context.Context, serverURL, protocol, address string, input delegates.ExecuteInput, start time.Time) delegates.ExecuteOutput {
	switch protocol {
	case "http", "https":
		return executeHTTPSend(ctx, serverURL, address, input, start)
	default:
		return delegates.FailedOutput(start, "unsupported_protocol",
			fmt.Sprintf("send not supported for protocol %q (supported: http, https)", protocol))
	}
}

// executeSSESubscribe connects to an SSE endpoint and collects events.
func executeSSESubscribe(ctx context.Context, serverURL, address string, maxEvents int, input delegates.ExecuteInput, start time.Time) delegates.ExecuteOutput {
	url := serverURL + "/" + strings.TrimLeft(address, "/")

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return delegates.FailedOutput(start, "request_build_failed", err.Error())
	}
	req.Header.Set("Accept", "text/event-stream")
	delegates.ApplyHTTPContext(req, input.Context)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return delegates.FailedOutput(start, "sse_connect_failed", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return delegates.HTTPErrorOutput(start, resp.StatusCode, resp.Status)
	}

	var events []any
	scanner := bufio.NewScanner(resp.Body)
	var dataLines []string

	for scanner.Scan() && len(events) < maxEvents {
		line := scanner.Text()

		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}

		if line == "" && len(dataLines) > 0 {
			events = append(events, parseSSEPayload(dataLines))
			dataLines = dataLines[:0]
		}
	}

	if len(dataLines) > 0 {
		events = append(events, parseSSEPayload(dataLines))
	}

	var output any
	if len(events) == 1 {
		output = events[0]
	} else {
		output = events
	}

	return delegates.ExecuteOutput{
		Output:     output,
		Status:     0,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// Subscribe opens a streaming subscription for an AsyncAPI operation.
// Only "receive" operations are supported for streaming.
func Subscribe(ctx context.Context, input delegates.ExecuteInput) (<-chan delegates.StreamEvent, error) {
	doc, err := loadDocument(input.Source)
	if err != nil {
		return nil, fmt.Errorf("load document: %w", err)
	}

	opID, err := parseRef(input.Ref)
	if err != nil {
		return nil, fmt.Errorf("parse ref: %w", err)
	}

	asyncOp, ok := doc.Operations[opID]
	if !ok {
		return nil, fmt.Errorf("operation %q not in AsyncAPI doc", opID)
	}

	if asyncOp.Action != "receive" {
		return nil, fmt.Errorf("streaming not supported for action %q (only receive)", asyncOp.Action)
	}

	serverURL, protocol, err := resolveServer(doc, input.Context)
	if err != nil {
		return nil, fmt.Errorf("resolve server: %w", err)
	}

	channelName := extractRefName(asyncOp.Channel.Ref)
	channel, hasChannel := doc.Channels[channelName]
	address := channelName
	if hasChannel && channel.Address != "" {
		address = channel.Address
	}

	switch protocol {
	case "http", "https":
		return subscribeSSE(ctx, serverURL, address, input)
	default:
		return nil, fmt.Errorf("streaming not supported for protocol %q (supported: http, https)", protocol)
	}
}

// subscribeSSE connects to an SSE endpoint and streams events on the returned channel.
func subscribeSSE(ctx context.Context, serverURL, address string, input delegates.ExecuteInput) (<-chan delegates.StreamEvent, error) {
	sseURL := serverURL + "/" + strings.TrimLeft(address, "/")

	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	delegates.ApplyHTTPContext(req, input.Context)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE connect: %w", err)
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("SSE endpoint returned HTTP %d", resp.StatusCode)
	}

	ch := make(chan delegates.StreamEvent)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		var dataLines []string

		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()

			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				continue
			}

			if line == "" && len(dataLines) > 0 {
				ev := parseSSEPayload(dataLines)
				dataLines = dataLines[:0]
				select {
				case ch <- delegates.StreamEvent{Data: ev}:
				case <-ctx.Done():
					return
				}
			}
		}

		if len(dataLines) > 0 {
			select {
			case ch <- delegates.StreamEvent{Data: parseSSEPayload(dataLines)}:
			case <-ctx.Done():
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- delegates.StreamEvent{Error: &delegates.Error{Code: "stream_error", Message: err.Error()}}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// executeHTTPSend publishes a message via HTTP POST to the channel address.
func executeHTTPSend(ctx context.Context, serverURL, address string, input delegates.ExecuteInput, start time.Time) delegates.ExecuteOutput {
	url := serverURL + "/" + strings.TrimLeft(address, "/")

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var bodyData []byte
	if input.Input != nil {
		var marshalErr error
		bodyData, marshalErr = json.Marshal(input.Input)
		if marshalErr != nil {
			return delegates.FailedOutput(start, "body_marshal_failed", marshalErr.Error())
		}
	} else {
		bodyData = []byte("{}")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyData))
	if err != nil {
		return delegates.FailedOutput(start, "request_build_failed", err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	delegates.ApplyHTTPContext(req, input.Context)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return delegates.FailedOutput(start, "request_failed", err.Error())
	}
	defer resp.Body.Close()

	duration := time.Since(start).Milliseconds()

	if resp.StatusCode >= 400 {
		return delegates.HTTPErrorOutput(start, resp.StatusCode, resp.Status)
	}

	if resp.StatusCode == 202 || resp.StatusCode == 204 {
		return delegates.ExecuteOutput{
			Status:     0,
			DurationMs: duration,
		}
	}

	var output any
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return delegates.ExecuteOutput{
			Status:     0,
			DurationMs: duration,
		}
	}

	return delegates.ExecuteOutput{
		Output:     output,
		Status:     0,
		DurationMs: duration,
	}
}

