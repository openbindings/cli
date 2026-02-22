package openapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
)

// FormatToken is the format identifier for OpenAPI sources.
const FormatToken = "openapi@^3.0.0"

// DefaultSourceName is the default source key for OpenAPI sources.
const DefaultSourceName = "openapi"

// ConvertToInterface converts an OpenAPI document to an OpenBindings interface.
func ConvertToInterface(source delegates.Source) (openbindings.Interface, error) {
	doc, err := loadDocument(source)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("load OpenAPI document: %w", err)
	}

	formatVersion := detectFormatVersion(doc.OpenAPI)

	iface := openbindings.Interface{
		OpenBindings: openbindings.MaxTestedVersion,
		Operations:   map[string]openbindings.Operation{},
		Bindings:     map[string]openbindings.BindingEntry{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: {
				Format:   "openapi@" + formatVersion,
				Location: source.Location,
			},
		},
	}

	if doc.Info != nil {
		iface.Name = doc.Info.Title
		iface.Version = doc.Info.Version
		iface.Description = doc.Info.Description
	}

	if doc.Paths == nil {
		return iface, nil
	}

	usedKeys := map[string]bool{}

	for _, path := range doc.Paths.InMatchingOrder() {
		pathItem := doc.Paths.Find(path)
		if pathItem == nil {
			continue
		}

		pathParams := pathItem.Parameters
		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}

			opKey := deriveOperationKey(op, path, method, usedKeys)
			usedKeys[opKey] = true

			obiOp := openbindings.Operation{
				Kind:        openbindings.OperationKindMethod,
				Description: operationDescription(op),
				Deprecated:  op.Deprecated,
			}

			if len(op.Tags) > 0 {
				obiOp.Tags = op.Tags
			}

			inputSchema := buildInputSchema(op, pathParams)
			if inputSchema != nil {
				obiOp.Input = inputSchema
			}

			outputSchema := buildOutputSchema(op)
			if outputSchema != nil {
				obiOp.Output = outputSchema
			}

			iface.Operations[opKey] = obiOp

			ref := buildJSONPointerRef(path, method)
			bindingKey := opKey + "." + DefaultSourceName
			iface.Bindings[bindingKey] = openbindings.BindingEntry{
				Operation: opKey,
				Source:    DefaultSourceName,
				Ref:       ref,
			}
		}
	}

	return iface, nil
}

// loadDocument loads and parses an OpenAPI document from a source.
func loadDocument(source delegates.Source) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	if source.Content != nil {
		data, err := delegates.ContentToBytes(source.Content)
		if err != nil {
			return nil, err
		}
		if source.Location != "" {
			loc, err := url.Parse(source.Location)
			if err == nil {
				return loader.LoadFromDataWithPath(data, loc)
			}
		}
		return loader.LoadFromData(data)
	}

	if source.Location == "" {
		return nil, fmt.Errorf("source must have location or content")
	}

	if strings.HasPrefix(source.Location, "http://") || strings.HasPrefix(source.Location, "https://") {
		loc, err := url.Parse(source.Location)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", source.Location, err)
		}
		return loader.LoadFromURI(loc)
	}

	return loader.LoadFromFile(source.Location)
}

// detectFormatVersion extracts a normalized version from the OpenAPI version string.
// "3.1.0" -> "3.1", "3.0.3" -> "3.0"
func detectFormatVersion(openapi string) string {
	parts := strings.Split(openapi, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return openapi
}

// deriveOperationKey generates an OBI operation key from an OpenAPI operation.
// Prefers operationId; falls back to path+method like "tasks.list".
func deriveOperationKey(op *openapi3.Operation, path, method string, used map[string]bool) string {
	if op.OperationID != "" {
		key := delegates.SanitizeKey(op.OperationID)
		if !used[key] {
			return key
		}
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	var parts []string
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			continue
		}
		if seg != "" {
			parts = append(parts, seg)
		}
	}

	key := strings.Join(parts, ".") + "." + strings.ToLower(method)
	key = delegates.SanitizeKey(key)
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

func operationDescription(op *openapi3.Operation) string {
	if op.Description != "" {
		return op.Description
	}
	return op.Summary
}

// buildJSONPointerRef builds a JSON Pointer ref for a path+method combination.
// Per the OpenBindings spec, refs for OpenAPI use JSON Pointer (RFC 6901).
// e.g., "#/paths/~1tasks~1{id}/get"
func buildJSONPointerRef(path, method string) string {
	escaped := strings.ReplaceAll(path, "~", "~0")
	escaped = strings.ReplaceAll(escaped, "/", "~1")
	return "#/paths/" + escaped + "/" + strings.ToLower(method)
}

// buildInputSchema constructs a JSON Schema for the operation's input from
// path/query/header parameters and the request body.
func buildInputSchema(op *openapi3.Operation, pathParams openapi3.Parameters) map[string]any {
	properties := map[string]any{}
	var required []string

	allParams := mergeParameters(pathParams, op.Parameters)

	for _, paramRef := range allParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value

		if param.In == "cookie" {
			continue
		}

		prop := paramToSchema(param)
		if prop != nil {
			properties[param.Name] = prop
		}

		if param.Required {
			required = append(required, param.Name)
		}
	}

	if op.RequestBody != nil && op.RequestBody.Value != nil {
		rb := op.RequestBody.Value
		bodySchema := requestBodyToSchema(rb)
		if bodySchema != nil {
			if bodyProps, ok := bodySchema["properties"].(map[string]any); ok {
				for k, v := range bodyProps {
					properties[k] = v
				}
				if bodyReq, ok := bodySchema["required"].([]string); ok {
					required = append(required, bodyReq...)
				}
			} else {
				properties["body"] = bodySchema
				if rb.Required {
					required = append(required, "body")
				}
			}
		}
	}

	if len(properties) == 0 {
		return nil
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

// mergeParameters merges path-level and operation-level parameters.
// Operation-level parameters override path-level parameters with the same name+in.
func mergeParameters(pathParams, opParams openapi3.Parameters) openapi3.Parameters {
	if len(pathParams) == 0 {
		return opParams
	}
	if len(opParams) == 0 {
		return pathParams
	}

	overridden := map[string]bool{}
	for _, p := range opParams {
		if p != nil && p.Value != nil {
			overridden[p.Value.In+":"+p.Value.Name] = true
		}
	}

	var merged openapi3.Parameters
	for _, p := range pathParams {
		if p != nil && p.Value != nil {
			if !overridden[p.Value.In+":"+p.Value.Name] {
				merged = append(merged, p)
			}
		}
	}
	merged = append(merged, opParams...)
	return merged
}

// paramToSchema converts an OpenAPI parameter to a JSON Schema property.
func paramToSchema(param *openapi3.Parameter) map[string]any {
	if param.Schema != nil && param.Schema.Value != nil {
		schema := schemaRefToMap(param.Schema)
		if param.Description != "" {
			if schema == nil {
				schema = map[string]any{}
			}
			schema["description"] = param.Description
		}
		return schema
	}

	prop := map[string]any{"type": "string"}
	if param.Description != "" {
		prop["description"] = param.Description
	}
	return prop
}

// requestBodyToSchema extracts a JSON Schema from a request body.
// Prefers application/json, then other JSON-like types, then first alphabetically.
func requestBodyToSchema(rb *openapi3.RequestBody) map[string]any {
	if rb.Content == nil {
		return nil
	}

	mt := preferJSONMediaType(rb.Content)
	if mt == nil || mt.Schema == nil {
		return nil
	}

	return schemaRefToMap(mt.Schema)
}

// buildOutputSchema extracts the output schema from the operation's success response.
func buildOutputSchema(op *openapi3.Operation) map[string]any {
	if op.Responses == nil {
		return nil
	}

	for _, code := range []string{"200", "201", "202"} {
		resp := op.Responses.Value(code)
		if resp == nil || resp.Value == nil {
			continue
		}
		return responseToSchema(resp.Value)
	}

	return nil
}

// responseToSchema extracts a JSON Schema from a response object.
func responseToSchema(resp *openapi3.Response) map[string]any {
	if resp.Content == nil {
		return nil
	}

	mt := preferJSONMediaType(resp.Content)
	if mt == nil || mt.Schema == nil {
		return nil
	}

	return schemaRefToMap(mt.Schema)
}

// preferJSONMediaType selects the best media type from a content map.
// Prefers application/json, then other JSON-compatible types, then the
// first type alphabetically for deterministic behavior.
func preferJSONMediaType(content openapi3.Content) *openapi3.MediaType {
	if mt := content.Get("application/json"); mt != nil {
		return mt
	}

	// Check for JSON-compatible types (e.g., application/vnd.api+json).
	keys := make([]string, 0, len(content))
	for k := range content {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if strings.Contains(k, "json") {
			return content[k]
		}
	}

	if len(keys) > 0 {
		return content[keys[0]]
	}
	return nil
}

// schemaRefToMap converts a kin-openapi SchemaRef to a plain map[string]any.
// This flattens $ref resolution into an inline schema for the OBI.
func schemaRefToMap(ref *openapi3.SchemaRef) map[string]any {
	if ref == nil || ref.Value == nil {
		return nil
	}

	data, err := ref.MarshalJSON()
	if err != nil {
		return map[string]any{"type": "object", "x-conversion-error": err.Error()}
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]any{"type": "object", "x-conversion-error": err.Error()}
	}

	delete(result, "__origin__")

	return result
}

