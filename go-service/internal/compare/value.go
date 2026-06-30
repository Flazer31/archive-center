package compare

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ValueDiff captures a single nested difference between two JSON values.
type ValueDiff struct {
	Path      string `json:"path"`
	Type      string `json:"type"` // "type", "scalar", "list_length", "null_vs_empty", "volatile", "missing_key"
	PythonVal string `json:"python_val"`
	GoVal     string `json:"go_val"`
	Note      string `json:"note,omitempty"`
}

// volatileSuffixes are field names that are expected to differ between backends.
var volatileSuffixes = []string{
	"timestamp",
	"exported_at",
	"generated_at",
	"runtime_updated_at",
	"updated_at",
}

// isVolatilePath reports whether a JSON path ends with a volatile field name.
func isVolatilePath(path string) bool {
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	// Strip array index suffix like [0]
	if idx := strings.Index(last, "["); idx != -1 {
		last = last[:idx]
	}
	for _, s := range volatileSuffixes {
		if strings.EqualFold(last, s) {
			return true
		}
	}
	return false
}

// CompareValues performs a deep comparison of two JSON-decoded values and
// returns up to maxDiffs differences. The returned bool is true if differences
// were truncated.
func CompareValues(pyVal, goVal any, maxDiffs int) ([]ValueDiff, bool) {
	dc := &diffCollector{maxDiffs: maxDiffs}
	compareValue(pyVal, goVal, "", dc)
	return dc.diffs, dc.truncated
}

type diffCollector struct {
	diffs     []ValueDiff
	maxDiffs  int
	truncated bool
}

func (dc *diffCollector) append(d ValueDiff) {
	if len(dc.diffs) >= dc.maxDiffs {
		dc.truncated = true
		return
	}
	if isVolatilePath(d.Path) {
		d.Type = "volatile"
		if d.Note == "" {
			d.Note = "volatile field"
		}
	}
	dc.diffs = append(dc.diffs, d)
}

func compareValue(pyVal, goVal any, path string, dc *diffCollector) {
	if dc.truncated {
		return
	}

	// Handle nil/null cases
	pyIsNil := pyVal == nil
	goIsNil := goVal == nil

	if pyIsNil && goIsNil {
		return
	}
	if pyIsNil && !goIsNil {
		if isEmptyMap(goVal) {
			dc.append(ValueDiff{
				Path:      path,
				Type:      "null_vs_empty",
				PythonVal: "null",
				GoVal:     "empty object",
				Note:      "null vs empty object",
			})
			return
		}
		if isEmptySlice(goVal) {
			dc.append(ValueDiff{
				Path:      path,
				Type:      "null_vs_empty",
				PythonVal: "null",
				GoVal:     "empty array",
				Note:      "null vs empty array",
			})
			return
		}
		dc.append(ValueDiff{
			Path:      path,
			Type:      "type",
			PythonVal: "null",
			GoVal:     fmt.Sprintf("%T", goVal),
		})
		return
	}
	if !pyIsNil && goIsNil {
		if isEmptyMap(pyVal) {
			dc.append(ValueDiff{
				Path:      path,
				Type:      "null_vs_empty",
				PythonVal: "empty object",
				GoVal:     "null",
				Note:      "empty object vs null",
			})
			return
		}
		if isEmptySlice(pyVal) {
			dc.append(ValueDiff{
				Path:      path,
				Type:      "null_vs_empty",
				PythonVal: "empty array",
				GoVal:     "null",
				Note:      "empty array vs null",
			})
			return
		}
		dc.append(ValueDiff{
			Path:      path,
			Type:      "type",
			PythonVal: fmt.Sprintf("%T", pyVal),
			GoVal:     "null",
		})
		return
	}

	// Normalize json.Number before type comparison
	pyVal = normalizeNumber(pyVal)
	goVal = normalizeNumber(goVal)

	pyKind := reflect.TypeOf(pyVal).Kind()
	goKind := reflect.TypeOf(goVal).Kind()

	if pyKind != goKind {
		dc.append(ValueDiff{
			Path:      path,
			Type:      "type",
			PythonVal: fmt.Sprintf("%T", pyVal),
			GoVal:     fmt.Sprintf("%T", goVal),
		})
		return
	}

	switch pyKind {
	case reflect.Map:
		compareMap(pyVal, goVal, path, dc)
	case reflect.Slice, reflect.Array:
		compareSlice(pyVal, goVal, path, dc)
	default:
		// Scalar
		if !reflect.DeepEqual(pyVal, goVal) {
			dc.append(ValueDiff{
				Path:      path,
				Type:      "scalar",
				PythonVal: fmt.Sprintf("%v", pyVal),
				GoVal:     fmt.Sprintf("%v", goVal),
			})
		}
	}
}

func normalizeNumber(v any) any {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
		if f, err := n.Float64(); err == nil {
			return f
		}
	}
	return v
}

func isEmptyMap(v any) bool {
	m, ok := v.(map[string]any)
	if ok {
		return len(m) == 0
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Map && rv.Len() == 0
}

func isEmptySlice(v any) bool {
	s, ok := v.([]any)
	if ok {
		return len(s) == 0
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Slice && rv.Len() == 0
}

func compareMap(pyVal, goVal any, path string, dc *diffCollector) {
	pyMap, ok1 := pyVal.(map[string]any)
	goMap, ok2 := goVal.(map[string]any)
	if !ok1 || !ok2 {
		dc.append(ValueDiff{
			Path:      path,
			Type:      "type",
			PythonVal: fmt.Sprintf("%T", pyVal),
			GoVal:     fmt.Sprintf("%T", goVal),
		})
		return
	}

	// Check for missing keys in go
	for k := range pyMap {
		if _, ok := goMap[k]; !ok {
			dc.append(ValueDiff{
				Path:      path + "." + k,
				Type:      "missing_key",
				PythonVal: "present",
				GoVal:     "missing",
			})
			if dc.truncated {
				return
			}
		}
	}

	// Check for extra keys in go
	for k := range goMap {
		if _, ok := pyMap[k]; !ok {
			dc.append(ValueDiff{
				Path:      path + "." + k,
				Type:      "missing_key",
				PythonVal: "missing",
				GoVal:     "present",
			})
			if dc.truncated {
				return
			}
		}
	}

	// Compare common keys
	for k, pyV := range pyMap {
		if goV, ok := goMap[k]; ok {
			childPath := path + "." + k
			if path == "" {
				childPath = "." + k
			}
			compareValue(pyV, goV, childPath, dc)
			if dc.truncated {
				return
			}
		}
	}
}

func compareSlice(pyVal, goVal any, path string, dc *diffCollector) {
	pySlice, ok1 := pyVal.([]any)
	goSlice, ok2 := goVal.([]any)
	if !ok1 || !ok2 {
		dc.append(ValueDiff{
			Path:      path,
			Type:      "type",
			PythonVal: fmt.Sprintf("%T", pyVal),
			GoVal:     fmt.Sprintf("%T", goVal),
		})
		return
	}

	if len(pySlice) != len(goSlice) {
		dc.append(ValueDiff{
			Path:      path,
			Type:      "list_length",
			PythonVal: fmt.Sprintf("%d", len(pySlice)),
			GoVal:     fmt.Sprintf("%d", len(goSlice)),
		})
		if dc.truncated {
			return
		}
	}

	minLen := len(pySlice)
	if len(goSlice) < minLen {
		minLen = len(goSlice)
	}

	for i := 0; i < minLen; i++ {
		childPath := fmt.Sprintf("%s[%d]", path, i)
		if path == "" {
			childPath = fmt.Sprintf("[%d]", i)
		}
		compareValue(pySlice[i], goSlice[i], childPath, dc)
		if dc.truncated {
			return
		}
	}
}
