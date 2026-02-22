package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbindings/cli/internal/delegates"
)

func TestExecuteSimpleGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/time" {
			t.Errorf("expected /time, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"utc":"2025-01-01T00:00:00Z","unix":1735689600}`)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": %q}],
		"paths": {
			"/time": {
				"get": {
					"operationId": "getTime",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`, server.URL)

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "openapi@3.1",
			Content: doc,
		},
		Ref: "#/paths/~1time/get",
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}
	if result.Status != 0 {
		t.Errorf("Status = %d, want 0", result.Status)
	}

	outputMap, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Output is %T, want map[string]any", result.Output)
	}
	if outputMap["utc"] != "2025-01-01T00:00:00Z" {
		t.Errorf("utc = %v, want 2025-01-01T00:00:00Z", outputMap["utc"])
	}
}

func TestExecutePostWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": %q}],
		"paths": {
			"/echo": {
				"post": {
					"operationId": "echo",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {"message": {"type": "string"}}
								}
							}
						}
					},
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`, server.URL)

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "openapi@3.1",
			Content: doc,
		},
		Ref:   "#/paths/~1echo/post",
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

func TestExecuteWithPathParam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/42" {
			t.Errorf("expected /tasks/42, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"42","title":"Test task"}`)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": %q}],
		"paths": {
			"/tasks/{id}": {
				"get": {
					"operationId": "getTask",
					"parameters": [
						{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
					],
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`, server.URL)

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "openapi@3.1",
			Content: doc,
		},
		Ref:   "#/paths/~1tasks~1{id}/get",
		Input: map[string]any{"id": "42"},
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}

	outputMap, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Output is %T, want map[string]any", result.Output)
	}
	if outputMap["id"] != "42" {
		t.Errorf("id = %v, want 42", outputMap["id"])
	}
}

func TestExecuteWithQueryParam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"limit":%q}`, limit)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": %q}],
		"paths": {
			"/items": {
				"get": {
					"operationId": "listItems",
					"parameters": [
						{"name": "limit", "in": "query", "schema": {"type": "integer"}}
					],
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`, server.URL)

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "openapi@3.1",
			Content: doc,
		},
		Ref:   "#/paths/~1items/get",
		Input: map[string]any{"limit": 10},
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}
	if result.Status != 0 {
		t.Errorf("Status = %d, want 0", result.Status)
	}
}

func TestExecuteWithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"auth":%q}`, auth)
	}))
	defer server.Close()

	doc := fmt.Sprintf(`{
		"openapi": "3.1.0",
		"info": {"title": "Test", "version": "1.0.0"},
		"servers": [{"url": %q}],
		"paths": {
			"/secure": {
				"get": {
					"operationId": "secure",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`, server.URL)

	input := delegates.ExecuteInput{
		Source: delegates.Source{
			Format:  "openapi@3.1",
			Content: doc,
		},
		Ref: "#/paths/~1secure/get",
		Context: &delegates.BindingContext{
			Credentials: &delegates.Credentials{
				BearerToken: "test-token-123",
			},
		},
	}

	result := Execute(context.Background(), input)
	if result.Error != nil {
		t.Fatalf("Execute failed: %s", result.Error.Message)
	}

	outputMap, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Output is %T, want map[string]any", result.Output)
	}
	if outputMap["auth"] != "Bearer test-token-123" {
		t.Errorf("auth = %v, want 'Bearer test-token-123'", outputMap["auth"])
	}
}
