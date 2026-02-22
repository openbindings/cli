// Package app - probe.go contains URL probing logic for discovering OpenBindings interfaces.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/cli/internal/execref"
)

// ProbeResult is the shared, presentation-agnostic result of probing a URL for an OpenBindings interface.
type ProbeResult struct {
	Status string // "idle" | "probing" | "ok" | "bad"
	Detail string
	OBI    string
	OBIURL string
	// FinalURL is the resolved URL after redirects (if any).
	FinalURL string
	// OBIDir is the base directory for resolving relative artifact paths.
	// Set for file-path targets (dirname of the file). Empty for exec: targets.
	OBIDir string
}

// NormalizeURL trims input and adds a default http:// scheme when missing.
// It preserves exec: references and file paths as-is.
func NormalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if execref.IsExec(s) {
		return s
	}
	// Preserve file paths: absolute paths, explicit relative paths, or
	// home-relative paths are not URLs.
	if isFilePath(s) {
		return s
	}
	if !strings.Contains(s, "://") {
		s = delegates.HTTPScheme + s
	}
	return s
}

// isFilePath returns true if s looks like a local file path rather than a hostname.
// Matches: /absolute, ./relative, ../parent, ~/home, or any path containing
// a slash that also has a file extension typical of OBI documents.
func isFilePath(s string) bool {
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") || strings.HasPrefix(s, "~") {
		return true
	}
	// Bare name with JSON/YAML extension (e.g., "interface.json")
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		// Only if it doesn't look like a URL (no port, no path segments before extension)
		if !strings.Contains(s, ":") {
			return true
		}
	}
	return false
}

// IsHTTPURL returns true for http(s) URLs with explicit scheme.
func IsHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// IsExecURL returns true when the raw string is an exec: reference.
func IsExecURL(raw string) bool {
	return execref.IsExec(raw)
}

// ProbeOBI attempts to fetch an OpenBindings interface from the given URL (direct or discoverable).
// For exec: references, it runs the command as-is; if stdout is a valid OBI, that is used.
// If not and the command was a single token (e.g. exec:ob), it retries with --openbindings.
func ProbeOBI(rawURL string, timeout time.Duration) ProbeResult {
	u := NormalizeURL(rawURL)
	if u == "" {
		return ProbeResult{Status: ProbeStatusIdle}
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	if execref.IsExec(u) {
		args, err := execref.Parse(u)
		if err != nil {
			return ProbeResult{Status: ProbeStatusBad, Detail: "invalid cli command"}
		}
		// Run as-is first. If stdout is already a valid OBI, use it
		// (e.g. exec:curl file:///path, exec:ob --openbindings, exec:cat iface.json).
		doc, firstErr := fetchOBICLIArgs(args, timeout)
		if firstErr == nil && doc != "" {
			return ProbeResult{Status: ProbeStatusOK, Detail: "cli", OBI: doc, OBIURL: u}
		}
		// If not, and user gave a single bare command name (no path, no args),
		// retry with --openbindings appended (e.g. exec:ob → ob --openbindings).
		if len(args) == 1 && !strings.Contains(args[0], "/") {
			doc, retryErr := fetchOBICLIArgs(append(args, "--openbindings"), timeout)
			if retryErr == nil && doc != "" {
				return ProbeResult{Status: ProbeStatusOK, Detail: "cli", OBI: doc, OBIURL: u}
			}
			// Report the retry error if available, otherwise the first.
			if retryErr != nil {
				return ProbeResult{Status: ProbeStatusBad, Detail: retryErr.Error()}
			}
		}
		// No retry path — report the original error.
		if firstErr != nil {
			return ProbeResult{Status: ProbeStatusBad, Detail: firstErr.Error()}
		}
		return ProbeResult{Status: ProbeStatusBad, Detail: "no openbindings interface in output"}
	}

	// Try as a local file path (not exec:, not http(s)://).
	if !strings.Contains(u, "://") {
		absPath := u
		if !filepath.IsAbs(absPath) {
			if cwd, err := os.Getwd(); err == nil {
				absPath = filepath.Join(cwd, absPath)
			}
		}
		data, err := os.ReadFile(absPath)
		if err == nil {
			doc, ok := normalizeOBIJSON(data)
			if ok {
				return ProbeResult{
					Status: ProbeStatusOK,
					Detail: "file",
					OBI:    doc,
					OBIURL: absPath,
					OBIDir: filepath.Dir(absPath),
				}
			}
			return ProbeResult{Status: ProbeStatusBad, Detail: "not a valid OpenBindings interface"}
		}
		// File doesn't exist — fall through to HTTP in case it's a hostname.
	}

	client := &http.Client{Timeout: timeout}
	doc, docURL, status, finalURL, err := fetchOBIHTTP(client, u)
	if err != nil {
		return ProbeResult{Status: ProbeStatusBad, Detail: err.Error()}
	}
	if doc != "" {
		return ProbeResult{Status: ProbeStatusOK, Detail: fmt.Sprintf("%d", status), OBI: doc, OBIURL: docURL, FinalURL: finalURL}
	}
	if status >= 200 && status < 400 {
		return ProbeResult{Status: ProbeStatusBad, Detail: "openbindings not found", FinalURL: finalURL}
	}
	return ProbeResult{Status: ProbeStatusBad, Detail: fmt.Sprintf("%d", status), FinalURL: finalURL}
}

func fetchOBIHTTP(client *http.Client, rawURL string) (string, string, int, string, error) {
	status, doc, finalURL, err := fetchURL(client, rawURL)
	if err != nil {
		return "", "", status, finalURL, err
	}
	if doc != "" {
		if finalURL != "" {
			return doc, finalURL, status, finalURL, nil
		}
		return doc, rawURL, status, finalURL, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", status, finalURL, nil
	}
	if strings.HasSuffix(parsed.Path, delegates.WellKnownPath) {
		return "", "", status, finalURL, nil
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	wkPath := basePath + delegates.WellKnownPath
	if !strings.HasPrefix(wkPath, "/") {
		wkPath = "/" + wkPath
	}
	wellKnown := url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: wkPath}
	wkStatus, wkDoc, wkFinalURL, wkErr := fetchURL(client, wellKnown.String())
	if wkErr != nil {
		return "", "", status, finalURL, nil
	}
	if wkDoc != "" {
		if wkFinalURL != "" {
			return wkDoc, wkFinalURL, wkStatus, wkFinalURL, nil
		}
		return wkDoc, wellKnown.String(), wkStatus, wkFinalURL, nil
	}
	return "", "", status, finalURL, nil
}

func fetchURL(client *http.Client, rawURL string) (int, string, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 400 {
		return status, "", resp.Request.URL.String(), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return status, "", resp.Request.URL.String(), err
	}
	doc, ok := normalizeOBIJSON(body)
	if !ok {
		return status, "", resp.Request.URL.String(), nil
	}
	return status, doc, resp.Request.URL.String(), nil
}

func normalizeOBIJSON(body []byte) (string, bool) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", false
	}
	if _, ok := raw["openbindings"].(string); !ok {
		return "", false
	}
	if _, ok := raw["operations"].(map[string]any); !ok {
		return "", false
	}
	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", false
	}
	return string(bytes.TrimSpace(pretty)), true
}

func fetchOBICLIArgs(args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("openbindings timeout")
	}
	if err != nil {
		return "", fmt.Errorf("openbindings failed")
	}
	doc, ok := normalizeOBIJSON(out)
	if !ok {
		return "", nil
	}
	return doc, nil
}

