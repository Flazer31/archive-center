package compare

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCompareValues_EqualMaps(t *testing.T) {
	a := map[string]any{"x": 1, "y": "foo"}
	b := map[string]any{"x": 1, "y": "foo"}
	diffs, truncated := CompareValues(a, b, 10)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %v", diffs)
	}
	if truncated {
		t.Error("expected truncated=false")
	}
}

func TestCompareValues_ScalarDiff(t *testing.T) {
	a := map[string]any{"x": 1}
	b := map[string]any{"x": 2}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Type != "scalar" {
		t.Errorf("expected scalar diff, got %s", diffs[0].Type)
	}
	if diffs[0].Path != ".x" {
		t.Errorf("expected path .x, got %s", diffs[0].Path)
	}
}

func TestCompareValues_TypeDiff(t *testing.T) {
	a := map[string]any{"x": 1}
	b := map[string]any{"x": "1"}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Type != "type" {
		t.Errorf("expected type diff, got %s", diffs[0].Type)
	}
}

func TestCompareValues_NestedDiff(t *testing.T) {
	a := map[string]any{"a": map[string]any{"b": 1}}
	b := map[string]any{"a": map[string]any{"b": 2}}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Path != ".a.b" {
		t.Errorf("expected path .a.b, got %s", diffs[0].Path)
	}
}

func TestCompareValues_ListLengthDiff(t *testing.T) {
	a := map[string]any{"items": []any{1, 2, 3}}
	b := map[string]any{"items": []any{1, 2}}
	diffs, _ := CompareValues(a, b, 10)
	found := false
	for _, d := range diffs {
		if d.Type == "list_length" && d.Path == ".items" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected list_length diff at .items, got %v", diffs)
	}
}

func TestCompareValues_NullVsEmptyObject(t *testing.T) {
	a := map[string]any{"meta": nil}
	b := map[string]any{"meta": map[string]any{}}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Type != "null_vs_empty" {
		t.Errorf("expected null_vs_empty, got %s", diffs[0].Type)
	}
}

func TestCompareValues_NullVsEmptySlice(t *testing.T) {
	a := map[string]any{"items": nil}
	b := map[string]any{"items": []any{}}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Type != "null_vs_empty" {
		t.Errorf("expected null_vs_empty, got %s", diffs[0].Type)
	}
}

func TestCompareValues_MissingKey(t *testing.T) {
	a := map[string]any{"x": 1, "y": 2}
	b := map[string]any{"x": 1}
	diffs, _ := CompareValues(a, b, 10)
	found := false
	for _, d := range diffs {
		if d.Type == "missing_key" && d.Path == ".y" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_key diff at .y, got %v", diffs)
	}
}

func TestCompareValues_VolatileField(t *testing.T) {
	a := map[string]any{"updated_at": "2024-01-01T00:00:00Z"}
	b := map[string]any{"updated_at": "2024-02-01T00:00:00Z"}
	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Type != "volatile" {
		t.Errorf("expected volatile type, got %s", diffs[0].Type)
	}
	if diffs[0].Note != "volatile field" {
		t.Errorf("expected volatile field note, got %s", diffs[0].Note)
	}
}

func TestCompareValues_Truncation(t *testing.T) {
	a := map[string]any{}
	b := map[string]any{}
	for i := 0; i < 10; i++ {
		a[fmt.Sprintf("k%d", i)] = i
		b[fmt.Sprintf("k%d", i)] = i + 1
	}
	diffs, truncated := CompareValues(a, b, 5)
	if len(diffs) != 5 {
		t.Fatalf("expected 5 diffs, got %d", len(diffs))
	}
	if !truncated {
		t.Error("expected truncated=true")
	}
}

func TestCompareValues_JsonNumberNormalization(t *testing.T) {
	raw := `{"x": 42}`
	var a, b map[string]any
	decA := json.NewDecoder(strings.NewReader(raw))
	decA.UseNumber()
	_ = decA.Decode(&a)
	decB := json.NewDecoder(strings.NewReader(raw))
	decB.UseNumber()
	_ = decB.Decode(&b)

	diffs, _ := CompareValues(a, b, 10)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs for identical json.Number values, got %v", diffs)
	}
}

func TestCompareValues_ArrayElements(t *testing.T) {
	a := map[string]any{"items": []any{1, 2, 3}}
	b := map[string]any{"items": []any{1, 2, 4}}
	diffs, _ := CompareValues(a, b, 10)
	found := false
	for _, d := range diffs {
		if d.Type == "scalar" && d.Path == ".items[2]" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scalar diff at .items[2], got %v", diffs)
	}
}

func TestIsVolatilePath(t *testing.T) {
	cases := []struct {
		path     string
		expected bool
	}{
		{".updated_at", true},
		{".exported_at", true},
		{".meta.generated_at", true},
		{".items[0].timestamp", true},
		{".items[0].name", false},
		{".runtime_updated_at", true},
	}
	for _, tc := range cases {
		got := isVolatilePath(tc.path)
		if got != tc.expected {
			t.Errorf("isVolatilePath(%q) = %v, want %v", tc.path, got, tc.expected)
		}
	}
}
