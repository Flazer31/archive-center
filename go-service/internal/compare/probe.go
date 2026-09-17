package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Probe sends the same request to both backends and returns a Result even
// when one or both backends return transport errors. Unsafe routes still
// return a blocked Result with Error set to ErrUnsafeRoute.
func (h *Harness) Probe(ctx context.Context, method, path string, body []byte) *Result {
	endpoint := method + " " + path
	if !isAllowed(method, path) {
		return &Result{
			Endpoint: endpoint,
			Allowed:  false,
			Error:    ErrUnsafeRoute.Error(),
		}
	}

	result := &Result{Endpoint: endpoint, Allowed: true}
	start := time.Now()

	pyBody, pyStatus, pyErr := h.do(ctx, h.PythonBaseURL+path, method, body)
	result.PythonStatus = pyStatus
	if pyErr != nil {
		result.Error = fmt.Sprintf("python backend: %v", pyErr)
	}

	goBody, goStatus, goErr := h.do(ctx, h.GoBaseURL+path, method, body)
	result.GoStatus = goStatus
	if goErr != nil {
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += fmt.Sprintf("go backend: %v", goErr)
	}

	result.DurationMS = time.Since(start).Milliseconds()
	result.StatusMatch = pyStatus == goStatus && pyErr == nil && goErr == nil

	if pyErr == nil && goErr == nil {
		var pyMap, goMap map[string]any
		_ = json.Unmarshal(pyBody, &pyMap)
		_ = json.Unmarshal(goBody, &goMap)

		result.PythonKeys = sortedKeys(pyMap)
		result.GoKeys = sortedKeys(goMap)
		result.MissingKeys, result.ExtraKeys = keyDiff(pyMap, goMap)
		result.KeysMatch = len(result.MissingKeys) == 0 && len(result.ExtraKeys) == 0
	}

	return result
}
