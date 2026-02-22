// Package delegates - probe.go contains delegate probing logic.
package delegates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/openbindings/cli/internal/execref"
	"github.com/openbindings/openbindings-go"
)

// ErrCommandTimeout indicates a command timed out.
var ErrCommandTimeout = errors.New("command timeout")

// ProbeFormats fetches the supported formats from a delegate
// by running its listFormats operation.
func ProbeFormats(path string, timeout time.Duration) ([]string, error) {
	if IsHTTPURL(path) {
		return nil, fmt.Errorf("formats require executor")
	}
	if IsExecURL(path) {
		cmd, err := execref.RootCommand(path)
		if err != nil {
			return nil, err
		}
		iface, err := RunCLIOpenBindings(cmd, timeout)
		if err != nil {
			return nil, err
		}
		return probeFormatsFromInterface(cmd, timeout, iface)
	}

	iface, err := RunCLIOpenBindings(path, timeout)
	if err != nil {
		return nil, err
	}
	return probeFormatsFromInterface(path, timeout, iface)
}

// RunCLIOpenBindings runs "<path> --openbindings" and parses the result.
func RunCLIOpenBindings(path string, timeout time.Duration) (openbindings.Interface, error) {
	stdout, stderr, err := RunCLI(path, []string{"--openbindings"}, timeout)
	if err != nil {
		if errors.Is(err, ErrCommandTimeout) {
			return openbindings.Interface{}, fmt.Errorf("openbindings timeout: %w", ErrCommandTimeout)
		}
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return openbindings.Interface{}, fmt.Errorf("openbindings command failed: %s", msg)
	}
	var iface openbindings.Interface
	if err := json.Unmarshal([]byte(stdout), &iface); err != nil {
		return openbindings.Interface{}, fmt.Errorf("invalid openbindings JSON: %w", err)
	}
	return iface, nil
}

// probeFormatsFromInterface executes the listFormats binding from an interface.
func probeFormatsFromInterface(path string, timeout time.Duration, iface openbindings.Interface) ([]string, error) {
	var (
		formatsRef string
		sourceKey  string
	)
	for _, b := range iface.Bindings {
		if b.Operation == OpListFormats {
			formatsRef = b.Ref
			sourceKey = b.Source
			break
		}
	}
	if formatsRef == "" || sourceKey == "" {
		return nil, fmt.Errorf("missing binding for %s", OpListFormats)
	}
	src, ok := iface.Sources[sourceKey]
	if !ok {
		return nil, fmt.Errorf("binding source not found for %s", OpListFormats)
	}
	if !strings.HasPrefix(src.Format, "usage@") {
		return nil, fmt.Errorf("unsupported binding format for %s", OpListFormats)
	}

	args := strings.Fields(formatsRef)
	if len(args) == 0 {
		return nil, fmt.Errorf("empty formats ref")
	}

	stdout, stderr, err := RunCLI(path, args, timeout)
	if err != nil {
		if errors.Is(err, ErrCommandTimeout) {
			return nil, fmt.Errorf("formats command timeout: %w", ErrCommandTimeout)
		}
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("formats command failed: %s", msg)
	}

	lines := strings.Split(stdout, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	sort.Strings(out)
	return out, nil
}

// FetchOpenBindings fetches an OpenBindings interface from an HTTP URL.
func FetchOpenBindings(raw string, timeout time.Duration) (openbindings.Interface, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("invalid delegate URL %q: %w", raw, err)
	}
	if !strings.Contains(u.Path, WellKnownPath) {
		u.Path = strings.TrimRight(u.Path, "/") + WellKnownPath
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("create request for %q: %w", u.String(), err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return openbindings.Interface{}, fmt.Errorf("fetch openbindings from %q: %w", u.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return openbindings.Interface{}, fmt.Errorf("openbindings request to %q failed: %s", u.String(), resp.Status)
	}
	var iface openbindings.Interface
	if err := json.NewDecoder(resp.Body).Decode(&iface); err != nil {
		return openbindings.Interface{}, fmt.Errorf("invalid openbindings JSON from %q: %w", u.String(), err)
	}
	return iface, nil
}

// RunCLI executes a CLI command with timeout and returns stdout, stderr, and error.
func RunCLI(command string, args []string, timeout time.Duration) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, stderr, ErrCommandTimeout
	}
	return stdout, stderr, err
}
