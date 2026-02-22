package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/openbindings/cli/internal/delegates"
)

const defaultTimeout = 30 * time.Second

// Execute executes an OpenAPI operation via HTTP.
func Execute(ctx context.Context, input delegates.ExecuteInput) delegates.ExecuteOutput {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	start := time.Now()

	doc, err := loadDocument(input.Source)
	if err != nil {
		return delegates.FailedOutput(start, "doc_load_failed", err.Error())
	}

	pathTemplate, method, err := parseRef(input.Ref)
	if err != nil {
		return delegates.FailedOutput(start, "invalid_ref", err.Error())
	}

	baseURL, err := resolveBaseURL(doc, input.Context)
	if err != nil {
		return delegates.FailedOutput(start, "no_base_url", err.Error())
	}

	pathItem := doc.Paths.Find(pathTemplate)
	if pathItem == nil {
		return delegates.FailedOutput(start, "path_not_found", fmt.Sprintf("path %q not in OpenAPI doc", pathTemplate))
	}
	op := pathItem.GetOperation(strings.ToUpper(method))
	if op == nil {
		return delegates.FailedOutput(start, "method_not_found", fmt.Sprintf("method %q not in path %q", method, pathTemplate))
	}

	allParams := mergeParameters(pathItem.Parameters, op.Parameters)
	inputMap, _ := delegates.ToStringAnyMap(input.Input)
	if inputMap == nil {
		inputMap = map[string]any{}
	}

	resolvedPath, queryParams, headerParams, bodyFields := classifyInput(allParams, inputMap, pathTemplate)

	reqURL := baseURL + resolvedPath
	if len(queryParams) > 0 {
		q := url.Values{}
		for k, v := range queryParams {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		reqURL += "?" + q.Encode()
	}

	var bodyReader io.Reader
	if hasRequestBody(op) {
		bodyBytes, err := json.Marshal(bodyFields)
		if err != nil {
			return delegates.FailedOutput(start, "body_marshal_failed", err.Error())
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), reqURL, bodyReader)
	if err != nil {
		return delegates.FailedOutput(start, "request_build_failed", err.Error())
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	for k, v := range headerParams {
		req.Header.Set(k, fmt.Sprintf("%v", v))
	}

	delegates.ApplyHTTPContext(req, input.Context)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return delegates.FailedOutput(start, "request_failed", err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return delegates.FailedOutput(start, "response_read_failed", err.Error())
	}

	duration := time.Since(start).Milliseconds()

	var output any
	if len(respBody) > 0 {
		trimmed := strings.TrimSpace(string(respBody))
		if delegates.MaybeJSON(trimmed) {
			var parsed any
			if json.Unmarshal(respBody, &parsed) == nil {
				output = parsed
			} else {
				output = string(respBody)
			}
		} else {
			output = string(respBody)
		}
	}

	if resp.StatusCode >= 400 {
		errOutput := delegates.HTTPErrorOutput(start, resp.StatusCode, resp.Status)
		errOutput.Output = output
		return errOutput
	}

	return delegates.ExecuteOutput{
		Output:     output,
		Status:     0,
		DurationMs: duration,
	}
}

// parseRef extracts the path template and HTTP method from an OpenAPI JSON Pointer ref.
// e.g., "#/paths/~1tasks~1{id}/get" → ("/tasks/{id}", "get")
func parseRef(ref string) (path string, method string, err error) {
	ref = strings.TrimPrefix(ref, "#/")

	parts := strings.Split(ref, "/")
	if len(parts) < 3 || parts[0] != "paths" {
		return "", "", fmt.Errorf("ref %q must be in format #/paths/<escaped-path>/<method>", ref)
	}

	method = parts[len(parts)-1]
	pathSegments := parts[1 : len(parts)-1]
	escapedPath := strings.Join(pathSegments, "/")

	path = strings.ReplaceAll(escapedPath, "~1", "/")
	path = strings.ReplaceAll(path, "~0", "~")

	validMethods := map[string]bool{
		"get": true, "post": true, "put": true, "patch": true,
		"delete": true, "head": true, "options": true, "trace": true,
	}
	if !validMethods[strings.ToLower(method)] {
		return "", "", fmt.Errorf("invalid HTTP method %q in ref", method)
	}

	return path, strings.ToLower(method), nil
}

// resolveBaseURL determines the base URL for requests.
// Checks BindingContext.Metadata["baseURL"] first, then falls back to the
// first server URL in the OpenAPI document.
func resolveBaseURL(doc *openapi3.T, bindCtx *delegates.BindingContext) (string, error) {
	if bindCtx != nil && bindCtx.Metadata != nil {
		if base, ok := bindCtx.Metadata["baseURL"].(string); ok && base != "" {
			return strings.TrimRight(base, "/"), nil
		}
	}

	if doc.Servers != nil && len(doc.Servers) > 0 {
		serverURL := doc.Servers[0].URL
		if serverURL != "" {
			return strings.TrimRight(serverURL, "/"), nil
		}
	}

	return "", fmt.Errorf("no server URL: set servers in the OpenAPI doc or provide baseURL in context metadata")
}

// classifyInput separates input fields into path params, query params, header params,
// and body fields based on the OpenAPI parameter definitions.
func classifyInput(params openapi3.Parameters, input map[string]any, pathTemplate string) (resolvedPath string, query, headers, body map[string]any) {
	query = map[string]any{}
	headers = map[string]any{}
	body = map[string]any{}

	paramClassification := map[string]string{}
	for _, paramRef := range params {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		paramClassification[paramRef.Value.Name] = paramRef.Value.In
	}

	resolvedPath = pathTemplate
	for name, value := range input {
		classification, isParam := paramClassification[name]
		if !isParam {
			body[name] = value
			continue
		}
		switch classification {
		case "path":
			resolvedPath = strings.ReplaceAll(resolvedPath, "{"+name+"}", fmt.Sprintf("%v", value))
		case "query":
			query[name] = value
		case "header":
			headers[name] = value
		default:
			body[name] = value
		}
	}

	return resolvedPath, query, headers, body
}

func hasRequestBody(op *openapi3.Operation) bool {
	return op.RequestBody != nil && op.RequestBody.Value != nil
}

