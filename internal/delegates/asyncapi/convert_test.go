package asyncapi

import (
	"testing"

	"github.com/openbindings/cli/internal/delegates"
)

const sampleAsyncAPI = `{
  "asyncapi": "3.0.0",
  "info": {
    "title": "Demo Events",
    "version": "1.0.0",
    "description": "A demo event-driven API"
  },
  "servers": {
    "production": {
      "host": "localhost:8080",
      "protocol": "http",
      "pathname": "/events"
    }
  },
  "channels": {
    "tick": {
      "address": "/tick",
      "messages": {
        "TickMessage": {
          "payload": {
            "type": "object",
            "properties": {
              "utc": {"type": "string"},
              "seq": {"type": "integer"}
            }
          }
        }
      }
    },
    "echo": {
      "address": "/echo",
      "messages": {
        "EchoRequest": {
          "payload": {
            "type": "object",
            "properties": {
              "message": {"type": "string"}
            },
            "required": ["message"]
          }
        },
        "EchoResponse": {
          "payload": {
            "type": "object",
            "properties": {
              "message": {"type": "string"}
            }
          }
        }
      }
    }
  },
  "operations": {
    "onTick": {
      "action": "receive",
      "channel": {"$ref": "#/channels/tick"},
      "summary": "Receive periodic tick events",
      "messages": [{"$ref": "#/channels/tick/messages/TickMessage"}]
    },
    "sendEcho": {
      "action": "send",
      "channel": {"$ref": "#/channels/echo"},
      "summary": "Send an echo message",
      "messages": [{"$ref": "#/channels/echo/messages/EchoRequest"}],
      "reply": {
        "messages": [{"$ref": "#/channels/echo/messages/EchoResponse"}]
      }
    }
  }
}`

func TestConvertToInterface(t *testing.T) {
	source := delegates.Source{
		Format:  "asyncapi@3.0",
		Content: sampleAsyncAPI,
	}

	iface, err := ConvertToInterface(source)
	if err != nil {
		t.Fatalf("ConvertToInterface failed: %v", err)
	}

	if iface.Name != "Demo Events" {
		t.Errorf("Name = %q, want %q", iface.Name, "Demo Events")
	}
	if iface.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", iface.Version, "1.0.0")
	}

	if len(iface.Operations) != 2 {
		t.Fatalf("got %d operations, want 2", len(iface.Operations))
	}

	tick, ok := iface.Operations["onTick"]
	if !ok {
		t.Fatal("missing operation 'onTick'")
	}
	if tick.Kind != "event" {
		t.Errorf("onTick.Kind = %q, want %q", tick.Kind, "event")
	}
	if tick.Payload == nil {
		t.Error("onTick.Payload is nil, want schema")
	}

	echo, ok := iface.Operations["sendEcho"]
	if !ok {
		t.Fatal("missing operation 'sendEcho'")
	}
	if echo.Kind != "method" {
		t.Errorf("sendEcho.Kind = %q, want %q", echo.Kind, "method")
	}
	if echo.Input == nil {
		t.Error("sendEcho.Input is nil, want schema")
	}
	if echo.Output == nil {
		t.Error("sendEcho.Output is nil, want schema (from reply)")
	}

	if len(iface.Bindings) != 2 {
		t.Fatalf("got %d bindings, want 2", len(iface.Bindings))
	}

	if len(iface.Sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(iface.Sources))
	}
	src := iface.Sources[DefaultSourceName]
	if src.Format != "asyncapi@3.0" {
		t.Errorf("source format = %q, want %q", src.Format, "asyncapi@3.0")
	}
}

func TestConvertReceiveEvent(t *testing.T) {
	source := delegates.Source{
		Format: "asyncapi@3.0",
		Content: `{
			"asyncapi": "3.0.0",
			"info": {"title": "Test", "version": "1.0.0"},
			"channels": {
				"notifications": {
					"address": "/notifications",
					"messages": {
						"Notification": {
							"payload": {
								"type": "object",
								"properties": {
									"text": {"type": "string"},
									"level": {"type": "string", "enum": ["info", "warn", "error"]}
								}
							}
						}
					}
				}
			},
			"operations": {
				"onNotification": {
					"action": "receive",
					"channel": {"$ref": "#/channels/notifications"},
					"messages": [{"$ref": "#/channels/notifications/messages/Notification"}]
				}
			}
		}`,
	}

	iface, err := ConvertToInterface(source)
	if err != nil {
		t.Fatalf("ConvertToInterface failed: %v", err)
	}

	op, ok := iface.Operations["onNotification"]
	if !ok {
		t.Fatal("missing operation 'onNotification'")
	}
	if op.Kind != "event" {
		t.Errorf("Kind = %q, want %q", op.Kind, "event")
	}
	if op.Payload == nil {
		t.Fatal("Payload is nil")
	}

	props, ok := op.Payload["properties"].(map[string]any)
	if !ok {
		t.Fatal("Payload.properties is not a map")
	}
	if _, ok := props["text"]; !ok {
		t.Error("Payload.properties missing 'text'")
	}
	if _, ok := props["level"]; !ok {
		t.Error("Payload.properties missing 'level'")
	}
}

func TestConvertSendVoidMethod(t *testing.T) {
	source := delegates.Source{
		Format: "asyncapi@3.0",
		Content: `{
			"asyncapi": "3.0.0",
			"info": {"title": "Test", "version": "1.0.0"},
			"channels": {
				"logs": {
					"address": "/logs",
					"messages": {
						"LogEntry": {
							"payload": {
								"type": "object",
								"properties": {
									"message": {"type": "string"},
									"severity": {"type": "string"}
								}
							}
						}
					}
				}
			},
			"operations": {
				"publishLog": {
					"action": "send",
					"channel": {"$ref": "#/channels/logs"},
					"messages": [{"$ref": "#/channels/logs/messages/LogEntry"}]
				}
			}
		}`,
	}

	iface, err := ConvertToInterface(source)
	if err != nil {
		t.Fatalf("ConvertToInterface failed: %v", err)
	}

	op, ok := iface.Operations["publishLog"]
	if !ok {
		t.Fatal("missing operation 'publishLog'")
	}
	if op.Kind != "method" {
		t.Errorf("Kind = %q, want %q", op.Kind, "method")
	}
	if op.Input == nil {
		t.Error("Input is nil, want payload schema")
	}
	if op.Output != nil {
		t.Error("Output should be nil for fire-and-forget send")
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref     string
		want    string
		wantErr bool
	}{
		{"#/operations/onTick", "onTick", false},
		{"#/operations/sendEcho", "sendEcho", false},
		{"onTick", "onTick", false},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := parseRef(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseRef(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
