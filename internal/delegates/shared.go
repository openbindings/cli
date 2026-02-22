// Package delegates - shared.go contains utilities shared across delegate handler implementations.
package delegates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// nonKeyChars matches characters that aren't valid in OBI operation keys.
var nonKeyChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// SanitizeKey converts a name to a valid OBI operation key.
// Replaces invalid characters with underscores and trims leading/trailing underscores.
func SanitizeKey(name string) string {
	key := nonKeyChars.ReplaceAllString(name, "_")
	key = strings.Trim(key, "_")
	if key == "" {
		key = "unnamed"
	}
	return key
}

// ContentToBytes converts a Source.Content value to raw bytes.
func ContentToBytes(content any) ([]byte, error) {
	switch c := content.(type) {
	case string:
		return []byte(c), nil
	case []byte:
		return c, nil
	default:
		return json.Marshal(c)
	}
}

// FailedOutput builds an ExecuteOutput for a pre-request failure.
// Status is set to 1 (general error, not an HTTP status code).
func FailedOutput(start time.Time, code, message string) ExecuteOutput {
	return ExecuteOutput{
		Status:     1,
		DurationMs: time.Since(start).Milliseconds(),
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}

// ApplyHTTPContext applies BindingContext credentials and headers to an HTTP request.
func ApplyHTTPContext(req *http.Request, bindCtx *BindingContext) {
	if bindCtx == nil {
		return
	}

	if bindCtx.Credentials != nil {
		creds := bindCtx.Credentials
		if creds.BearerToken != "" {
			req.Header.Set("Authorization", "Bearer "+creds.BearerToken)
		} else if creds.APIKey != "" {
			req.Header.Set("Authorization", "ApiKey "+creds.APIKey)
		} else if creds.Basic != nil {
			req.SetBasicAuth(creds.Basic.Username, creds.Basic.Password)
		}
	}

	for k, v := range bindCtx.Headers {
		req.Header.Set(k, v)
	}
}

// HTTPErrorOutput builds an ExecuteOutput from an HTTP error response.
func HTTPErrorOutput(start time.Time, statusCode int, status string) ExecuteOutput {
	return ExecuteOutput{
		Status:     statusCode,
		DurationMs: time.Since(start).Milliseconds(),
		Error: &Error{
			Code:    fmt.Sprintf("http_%d", statusCode),
			Message: fmt.Sprintf("HTTP %d %s", statusCode, status),
		},
	}
}

// ToStringAnyMap converts any to map[string]any if possible.
// Returns (nil, false) for nil input.
func ToStringAnyMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

// MaybeJSON returns true if the trimmed string looks like a JSON object or array.
func MaybeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}
