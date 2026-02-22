package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/openbindings/cli/internal/delegates"
)

const sampleOpenAPI = `{
  "openapi": "3.1.0",
  "info": {
    "title": "Demo API",
    "version": "1.0.0",
    "description": "A simple demo API"
  },
  "servers": [
    {"url": "http://localhost:8080"}
  ],
  "paths": {
    "/echo": {
      "post": {
        "operationId": "echo",
        "summary": "Echo a message",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "message": {"type": "string"}
                },
                "required": ["message"]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Echoed message",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "message": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/time": {
      "get": {
        "operationId": "getTime",
        "description": "Get the current time",
        "responses": {
          "200": {
            "description": "Current time",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "utc": {"type": "string"},
                    "unix": {"type": "integer"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/tasks/{id}": {
      "get": {
        "operationId": "getTask",
        "description": "Get a task by ID",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "A task",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "id": {"type": "string"},
                    "title": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

func TestConvertToInterface(t *testing.T) {
	source := delegates.Source{
		Format:  "openapi@3.1",
		Content: sampleOpenAPI,
	}

	iface, err := ConvertToInterface(source)
	if err != nil {
		t.Fatalf("ConvertToInterface failed: %v", err)
	}

	if iface.Name != "Demo API" {
		t.Errorf("Name = %q, want %q", iface.Name, "Demo API")
	}
	if iface.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", iface.Version, "1.0.0")
	}

	if len(iface.Operations) != 3 {
		t.Fatalf("got %d operations, want 3", len(iface.Operations))
	}

	echo, ok := iface.Operations["echo"]
	if !ok {
		t.Fatal("missing operation 'echo'")
	}
	if echo.Kind != "method" {
		t.Errorf("echo.Kind = %q, want %q", echo.Kind, "method")
	}
	if echo.Input == nil {
		t.Error("echo.Input is nil, want schema")
	}
	if echo.Output == nil {
		t.Error("echo.Output is nil, want schema")
	}

	getTime, ok := iface.Operations["getTime"]
	if !ok {
		t.Fatal("missing operation 'getTime'")
	}
	if getTime.Input != nil {
		t.Error("getTime.Input should be nil (no params or body)")
	}
	if getTime.Output == nil {
		t.Error("getTime.Output is nil, want schema")
	}

	getTask, ok := iface.Operations["getTask"]
	if !ok {
		t.Fatal("missing operation 'getTask'")
	}
	if getTask.Input == nil {
		t.Fatal("getTask.Input is nil, want schema with 'id' param")
	}
	props, ok := getTask.Input["properties"].(map[string]any)
	if !ok {
		t.Fatal("getTask.Input.properties is not a map")
	}
	if _, ok := props["id"]; !ok {
		t.Error("getTask.Input.properties missing 'id'")
	}

	if len(iface.Bindings) != 3 {
		t.Fatalf("got %d bindings, want 3", len(iface.Bindings))
	}

	if len(iface.Sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(iface.Sources))
	}
	src := iface.Sources[DefaultSourceName]
	if src.Format != "openapi@3.1" {
		t.Errorf("source format = %q, want %q", src.Format, "openapi@3.1")
	}
}

func TestBuildJSONPointerRef(t *testing.T) {
	tests := []struct {
		path, method, want string
	}{
		{"/tasks", "get", "#/paths/~1tasks/get"},
		{"/tasks/{id}", "get", "#/paths/~1tasks~1{id}/get"},
		{"/api/v1/users", "post", "#/paths/~1api~1v1~1users/post"},
	}
	for _, tt := range tests {
		got := buildJSONPointerRef(tt.path, tt.method)
		if got != tt.want {
			t.Errorf("buildJSONPointerRef(%q, %q) = %q, want %q", tt.path, tt.method, got, tt.want)
		}
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantPath string
		wantMeth string
		wantErr  bool
	}{
		{"#/paths/~1tasks/get", "/tasks", "get", false},
		{"#/paths/~1tasks~1{id}/get", "/tasks/{id}", "get", false},
		{"#/paths/~1api~1v1~1users/post", "/api/v1/users", "post", false},
		{"invalid", "", "", true},
		{"#/components/schemas/Foo", "", "", true},
	}
	for _, tt := range tests {
		path, method, err := parseRef(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if path != tt.wantPath {
			t.Errorf("parseRef(%q) path = %q, want %q", tt.ref, path, tt.wantPath)
		}
		if method != tt.wantMeth {
			t.Errorf("parseRef(%q) method = %q, want %q", tt.ref, method, tt.wantMeth)
		}
	}
}

func TestDeriveOperationKey(t *testing.T) {
	tests := []struct {
		name     string
		opID     string
		path     string
		method   string
		wantKey  string
	}{
		{"uses operationId", "listTasks", "/tasks", "get", "listTasks"},
		{"fallback to path.method", "", "/tasks", "get", "tasks.get"},
		{"strips path params", "", "/tasks/{id}", "get", "tasks.get"},
		{"nested path", "", "/api/v1/users", "post", "api.v1.users.post"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &openapi3.Operation{OperationID: tt.opID}
			got := deriveOperationKey(op, tt.path, tt.method, map[string]bool{})
			if got != tt.wantKey {
				t.Errorf("got %q, want %q", got, tt.wantKey)
			}
		})
	}
}
