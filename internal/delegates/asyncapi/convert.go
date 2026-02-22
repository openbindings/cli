package asyncapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"gopkg.in/yaml.v3"
)

// FormatToken is the format identifier for AsyncAPI sources.
const FormatToken = "asyncapi@^3.0.0"

// DefaultSourceName is the default source key for AsyncAPI sources.
const DefaultSourceName = "asyncapi"

// ConvertToInterface converts an AsyncAPI 3.0 document to an OpenBindings interface.
func ConvertToInterface(source delegates.Source) (openbindings.Interface, error) {
	doc, err := loadDocument(source)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("load AsyncAPI document: %w", err)
	}

	formatVersion := detectFormatVersion(doc.AsyncAPI)

	iface := openbindings.Interface{
		OpenBindings: openbindings.MaxTestedVersion,
		Name:         doc.Info.Title,
		Version:      doc.Info.Version,
		Description:  doc.Info.Description,
		Operations:   map[string]openbindings.Operation{},
		Bindings:     map[string]openbindings.BindingEntry{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: {
				Format:   "asyncapi@" + formatVersion,
				Location: source.Location,
			},
		},
	}

	usedKeys := map[string]bool{}

	opIDs := make([]string, 0, len(doc.Operations))
	for opID := range doc.Operations {
		opIDs = append(opIDs, opID)
	}
	sort.Strings(opIDs)

	for _, opID := range opIDs {
		asyncOp := doc.Operations[opID]
		opKey := deriveOperationKey(opID, usedKeys)
		usedKeys[opKey] = true

		obiOp := openbindings.Operation{
			Description: operationDescription(asyncOp),
		}

		if len(asyncOp.Tags) > 0 {
			for _, tag := range asyncOp.Tags {
				obiOp.Tags = append(obiOp.Tags, tag.Name)
			}
		}

		switch asyncOp.Action {
		case "receive":
			obiOp.Kind = openbindings.OperationKindEvent
			payload := resolveOperationPayload(doc, asyncOp)
			if payload != nil {
				obiOp.Payload = payload
			}
		case "send":
			if asyncOp.Reply != nil {
				obiOp.Kind = openbindings.OperationKindMethod
				inputPayload := resolveOperationPayload(doc, asyncOp)
				if inputPayload != nil {
					obiOp.Input = inputPayload
				}
				outputPayload := resolveReplyPayload(doc, asyncOp.Reply)
				if outputPayload != nil {
					obiOp.Output = outputPayload
				}
			} else {
				obiOp.Kind = openbindings.OperationKindMethod
				inputPayload := resolveOperationPayload(doc, asyncOp)
				if inputPayload != nil {
					obiOp.Input = inputPayload
				}
			}
		default:
			obiOp.Kind = openbindings.OperationKindMethod
		}

		iface.Operations[opKey] = obiOp

		ref := "#/operations/" + opID
		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       ref,
		}
	}

	return iface, nil
}

// loadDocument loads and parses an AsyncAPI document from a source.
func loadDocument(source delegates.Source) (*Document, error) {
	data, err := sourceToBytesAsync(source)
	if err != nil {
		return nil, err
	}

	var doc Document

	if isJSON(data) {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse AsyncAPI JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parse AsyncAPI YAML: %w", err)
		}
	}

	if !strings.HasPrefix(doc.AsyncAPI, "3.") {
		return nil, fmt.Errorf("unsupported AsyncAPI version %q (expected 3.x)", doc.AsyncAPI)
	}

	return &doc, nil
}

func sourceToBytesAsync(source delegates.Source) ([]byte, error) {
	if source.Content != nil {
		return delegates.ContentToBytes(source.Content)
	}
	if source.Location == "" {
		return nil, fmt.Errorf("source must have location or content")
	}
	if delegates.IsHTTPURL(source.Location) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", source.Location, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch %q: %w", source.Location, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch %q: %w", source.Location, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("fetch %q: HTTP %d", source.Location, resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	return os.ReadFile(source.Location)
}

func isJSON(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return true
		default:
			return false
		}
	}
	return false
}

func detectFormatVersion(asyncapi string) string {
	parts := strings.Split(asyncapi, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return asyncapi
}

func deriveOperationKey(opID string, used map[string]bool) string {
	key := delegates.SanitizeKey(opID)
	if !used[key] {
		return key
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", key, i)
		if !used[candidate] {
			return candidate
		}
	}
}

func operationDescription(op Operation) string {
	if op.Description != "" {
		return op.Description
	}
	return op.Summary
}

// resolveOperationPayload extracts the payload schema from an operation's messages.
// If the operation specifies messages, uses the first one. Otherwise, falls back
// to the channel's messages.
func resolveOperationPayload(doc *Document, op Operation) map[string]any {
	if len(op.Messages) > 0 {
		msg := resolveMessageRef(doc, op.Messages[0])
		if msg != nil && msg.Payload != nil {
			return msg.Payload
		}
	}

	channelName := extractRefName(op.Channel.Ref)
	if channelName == "" {
		return nil
	}
	channel, ok := doc.Channels[channelName]
	if !ok {
		return nil
	}

	for _, msg := range channel.Messages {
		if msg.Payload != nil {
			return msg.Payload
		}
	}

	return nil
}

// resolveReplyPayload extracts the payload schema from a reply definition.
func resolveReplyPayload(doc *Document, reply *OperationReply) map[string]any {
	if reply == nil {
		return nil
	}

	if len(reply.Messages) > 0 {
		msg := resolveMessageRef(doc, reply.Messages[0])
		if msg != nil && msg.Payload != nil {
			return msg.Payload
		}
	}

	return nil
}

// resolveMessageRef resolves a $ref to a Message.
// Supports #/components/messages/<name> and #/channels/<name>/messages/<name>.
func resolveMessageRef(doc *Document, ref MessageRef) *Message {
	if ref.Ref == "" {
		return nil
	}

	path := strings.TrimPrefix(ref.Ref, "#/")
	parts := strings.Split(path, "/")

	if len(parts) == 3 && parts[0] == "components" && parts[1] == "messages" {
		if doc.Components != nil {
			if msg, ok := doc.Components.Messages[parts[2]]; ok {
				return &msg
			}
		}
	}

	if len(parts) == 4 && parts[0] == "channels" && parts[2] == "messages" {
		if ch, ok := doc.Channels[parts[1]]; ok {
			if msg, ok := ch.Messages[parts[3]]; ok {
				return &msg
			}
		}
	}

	return nil
}

// extractRefName extracts the last segment from a $ref string.
// e.g., "#/channels/tick" → "tick"
func extractRefName(ref string) string {
	if ref == "" {
		return ""
	}
	path := strings.TrimPrefix(ref, "#/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
