package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestBuildShapeCasesCoversAtLeast35Routes(t *testing.T) {
	cases := buildShapeCases("sess-test")
	if len(cases) < 35 {
		t.Fatalf("expected at least 35 shape cases, got %d", len(cases))
	}
	seen := map[int]bool{}
	for _, tc := range cases {
		if tc.ID <= 0 {
			t.Fatalf("route %q has non-positive id %d", tc.Name, tc.ID)
		}
		if seen[tc.ID] {
			t.Fatalf("duplicate route id %d", tc.ID)
		}
		seen[tc.ID] = true
		if tc.Name == "" || tc.Method == "" || tc.Path == "" {
			t.Fatalf("shape case has empty required field: %+v", tc)
		}
		if len(tc.ExpectedKinds) == 0 {
			t.Fatalf("shape case %q has no expected kinds", tc.Name)
		}
	}
}

func TestJSONKindClassification(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{nil, "null"},
		{"hello", "string"},
		{float64(42), "number"},
		{true, "boolean"},
		{[]any{1, 2}, "array"},
		{map[string]any{"a": 1}, "object"},
	}
	for _, tt := range tests {
		got := jsonKind(tt.value)
		if got != tt.want {
			t.Fatalf("jsonKind(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestProbeShapeDetectsMissingField(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})
	result := probeShape(handler, shapeCase{
		ID:            1,
		Name:          "missing-field",
		Method:        http.MethodGet,
		Path:          "/x",
		ExpectedKinds: []expectedKind{{"status", "string"}, {"items", "array"}},
	})
	if result.Passed {
		t.Fatal("expected missing field to fail")
	}
	if result.Detail != "missing_field:items" {
		t.Fatalf("detail = %q, want missing_field:items", result.Detail)
	}
}

func TestProbeShapeDetectsTypeMismatch(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "total": "not-a-number"})
	})
	result := probeShape(handler, shapeCase{
		ID:            2,
		Name:          "type-mismatch",
		Method:        http.MethodGet,
		Path:          "/x",
		ExpectedKinds: []expectedKind{{"status", "string"}, {"total", "number"}},
	})
	if result.Passed {
		t.Fatal("expected type mismatch to fail")
	}
	wantDetail := "type_mismatch:total expected number got string"
	if result.Detail != wantDetail {
		t.Fatalf("detail = %q, want %q", result.Detail, wantDetail)
	}
}

func TestProbeShapeRejects404And405(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		})
		result := probeShape(handler, shapeCase{
			ID:            3,
			Name:          "no-route",
			Method:        http.MethodGet,
			Path:          "/missing",
			ExpectedKinds: []expectedKind{{"status", "string"}},
		})
		if result.Passed {
			t.Fatalf("expected status %d to fail", code)
		}
		if code == http.StatusNotFound && result.Detail != "no_route" {
			t.Fatalf("detail = %q, want no_route", result.Detail)
		}
		if code == http.StatusMethodNotAllowed && result.Detail != "method_not_allowed" {
			t.Fatalf("detail = %q, want method_not_allowed", result.Detail)
		}
	}
}

func TestProbeShapeRejectsServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	result := probeShape(handler, shapeCase{
		ID:            4,
		Name:          "server-error",
		Method:        http.MethodGet,
		Path:          "/err",
		ExpectedKinds: []expectedKind{{"status", "string"}},
	})
	if result.Passed {
		t.Fatal("expected 500 to fail")
	}
	if result.Detail != "server_error:500" {
		t.Fatalf("detail = %q, want server_error:500", result.Detail)
	}
}

func TestProbeShapeAcceptsJSONParseFailure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	})
	result := probeShape(handler, shapeCase{
		ID:            5,
		Name:          "bad-json",
		Method:        http.MethodGet,
		Path:          "/bad",
		ExpectedKinds: []expectedKind{{"status", "string"}},
	})
	if result.Passed {
		t.Fatal("expected invalid json to fail")
	}
	if result.Detail != "json_parse_failure" {
		t.Fatalf("detail = %q, want json_parse_failure", result.Detail)
	}
}

func TestRunSmokeWithRealServer(t *testing.T) {
	handler := newRealSmokeHandler()
	report := runSmoke(handler, "sess-real")
	if report.Status != "ok" {
		failed := []shapeResult{}
		for _, route := range report.Routes {
			if !route.Passed {
				failed = append(failed, route)
			}
		}
		t.Fatalf("real server smoke status = %q, failed routes = %+v", report.Status, failed)
	}
	if report.Summary.Total < 35 {
		t.Fatalf("summary total = %d, want >= 35", report.Summary.Total)
	}
	if report.Summary.Failed != 0 {
		t.Fatalf("summary failed = %d, want 0", report.Summary.Failed)
	}
	if report.Summary.NoRouteFailures != 0 {
		t.Fatalf("summary no_route_failures = %d, want 0", report.Summary.NoRouteFailures)
	}
	if report.Summary.JSONFailures != 0 {
		t.Fatalf("summary json_failures = %d, want 0", report.Summary.JSONFailures)
	}
	if report.Summary.MissingFields != 0 {
		t.Fatalf("summary missing_fields = %d, want 0", report.Summary.MissingFields)
	}
	if report.Summary.TypeMismatches != 0 {
		t.Fatalf("summary type_mismatches = %d, want 0", report.Summary.TypeMismatches)
	}
	if report.Summary.ServerErrors != 0 {
		t.Fatalf("summary server_errors = %d, want 0", report.Summary.ServerErrors)
	}
}
