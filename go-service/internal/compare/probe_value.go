package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ProbeValue sends the same request to both backends and performs deep value
// comparison on the decoded JSON bodies. It returns a ValueResult even when
// one or both backends return transport errors.
func (h *Harness) ProbeValue(ctx context.Context, method, path string, body []byte, maxDiffs int) *ValueResult {
	endpoint := method + " " + path
	if !isAllowed(method, path) {
		return &ValueResult{
			Endpoint: endpoint,
			Allowed:  false,
			Error:    ErrUnsafeRoute.Error(),
		}
	}

	result := &ValueResult{Endpoint: endpoint, Allowed: true}
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

		// Deep value comparison
		result.Diffs, result.DiffTruncated = CompareValues(pyMap, goMap, maxDiffs)

		// Exact JSON match
		result.ExactJSONMatch = len(result.Diffs) == 0

		// Type match: no structural type diffs
		result.TypeMatch = true
		volatileOnly := len(result.Diffs) > 0
		for _, d := range result.Diffs {
			if d.Type == "type" || d.Type == "missing_key" || d.Type == "null_vs_empty" {
				result.TypeMatch = false
			}
			if d.Type != "volatile" {
				volatileOnly = false
			}
		}
		result.VolatileOnlyDiffs = volatileOnly
		result.BehaviorMatch = result.StatusMatch && result.KeysMatch && result.TypeMatch && (result.ExactJSONMatch || result.VolatileOnlyDiffs)
	}

	return result
}
