// Package compare provides a baseline comparison harness between the Python 0.8
// backend and the Go shadow backend. Only read-only routes are permitted; any
// unsafe route is rejected before a request is sent.
package compare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrUnsafeRoute is returned when the requested endpoint is not in the
	// read-only allowlist.
	ErrUnsafeRoute = errors.New("route is not in the read-only allowlist")

	// ErrStatusMismatch is returned when the status codes differ.
	ErrStatusMismatch = errors.New("status code mismatch")
)

// Harness compares responses between the Python 0.8 backend and the Go
// shadow backend.
type Harness struct {
	PythonBaseURL string
	GoBaseURL     string
	HTTPClient    *http.Client
}

// Result captures the outcome of a single comparison.
type Result struct {
	Endpoint     string
	Allowed      bool
	StatusMatch  bool
	KeysMatch    bool
	PythonStatus int
	GoStatus     int
	PythonKeys   []string
	GoKeys       []string
	MissingKeys  []string
	ExtraKeys    []string
	DurationMS   int64
	Error        string
}

// NewHarness creates a comparison harness.
func NewHarness(pythonURL, goURL string) *Harness {
	return &Harness{
		PythonBaseURL: strings.TrimRight(pythonURL, "/"),
		GoBaseURL:     strings.TrimRight(goURL, "/"),
		HTTPClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Compare sends the same request to both backends and compares the responses.
// If the endpoint is not in the read-only allowlist, it returns ErrUnsafeRoute
// immediately without making any network calls.
func (h *Harness) Compare(ctx context.Context, method, path string, body []byte) (*Result, error) {
	endpoint := method + " " + path
	if !isAllowed(method, path) {
		return &Result{Endpoint: endpoint, Allowed: false}, ErrUnsafeRoute
	}

	result := &Result{Endpoint: endpoint, Allowed: true}
	start := time.Now()

	// Call Python backend.
	pyBody, pyStatus, pyErr := h.do(ctx, h.PythonBaseURL+path, method, body)
	if pyErr != nil {
		return nil, fmt.Errorf("python backend: %w", pyErr)
	}
	result.PythonStatus = pyStatus

	// Call Go shadow backend.
	goBody, goStatus, goErr := h.do(ctx, h.GoBaseURL+path, method, body)
	if goErr != nil {
		return nil, fmt.Errorf("go backend: %w", goErr)
	}
	result.GoStatus = goStatus

	result.DurationMS = time.Since(start).Milliseconds()
	result.StatusMatch = pyStatus == goStatus

	var pyMap, goMap map[string]any
	_ = json.Unmarshal(pyBody, &pyMap)
	_ = json.Unmarshal(goBody, &goMap)

	result.PythonKeys = sortedKeys(pyMap)
	result.GoKeys = sortedKeys(goMap)
	result.MissingKeys, result.ExtraKeys = keyDiff(pyMap, goMap)
	result.KeysMatch = len(result.MissingKeys) == 0 && len(result.ExtraKeys) == 0

	return result, nil
}

// do executes a single HTTP request and returns the body, status, and any error.
func (h *Harness) do(ctx context.Context, url, method string, body []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}
