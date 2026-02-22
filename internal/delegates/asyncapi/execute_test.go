package asyncapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbindings/cli/internal/delegates"
)

func TestExecuteSSEReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tick" {
			t.Errorf("expected /tick, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprintf(w, "data: {\"utc\":\"2025-01-01T00:00:00Z\",\"seq\":1}\n\n")
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"asyncapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": {
			"local": {"host": %q, "protocol": "http"}
		},
		"channels": {
			"tick": {
				"address": "/tick",
				"messages": {
					"TickMessage": {
						"payload": {"type": "object"}
					}
				}
			}
		},
		"operations": {
			"onTick": {
				"action": "receive",
				"channel": {"$ref": "#/channels/tick"},
				"messages": [{"$ref": "#/channels/tick/messages/TickMessage"}]
			}
		}
	}`, hostFromURL(server.URL))

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "asyncapi@3.0",
			Content: doc,
		},
		Ref: "#/operations/onTick",
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}

	outputMap, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Output is %T, want map[string]any", result.Output)
	}
	if outputMap["seq"] != float64(1) {
		t.Errorf("seq = %v, want 1", outputMap["seq"])
	}
}

func TestExecuteHTTPSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/echo" {
			t.Errorf("expected /echo, got %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"asyncapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": {
			"local": {"host": %q, "protocol": "http"}
		},
		"channels": {
			"echo": {
				"address": "/echo",
				"messages": {
					"EchoRequest": {"payload": {"type": "object"}}
				}
			}
		},
		"operations": {
			"sendEcho": {
				"action": "send",
				"channel": {"$ref": "#/channels/echo"},
				"messages": [{"$ref": "#/channels/echo/messages/EchoRequest"}]
			}
		}
	}`, hostFromURL(server.URL))

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "asyncapi@3.0",
			Content: doc,
		},
		Ref:   "#/operations/sendEcho",
		Input: map[string]any{"message": "hello"},
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}

	outputMap, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Output is %T, want map[string]any", result.Output)
	}
	if outputMap["message"] != "hello" {
		t.Errorf("message = %v, want hello", outputMap["message"])
	}
}

// hostFromURL strips the scheme from a URL to get host:port.
func hostFromURL(rawURL string) string {
	for _, prefix := range []string{"http://", "https://"} {
		if len(rawURL) > len(prefix) && rawURL[:len(prefix)] == prefix {
			return rawURL[len(prefix):]
		}
	}
	return rawURL
}
