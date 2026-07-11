package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq20P332GraphAcceleratorPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_graph_accelerator_preserve")
	if s["version"] != "s20-p332.v1" {
		t.Fatalf("version=%v, want s20-p332.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"structured_edge_signal_degraded_optional_off", "entity_side_fail_open_path"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_graph_accelerator_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_graph_accelerator_preserve_definition", s["mode"])
	}
}

func TestSeq20P333AmbiguitySupportNotePreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p333","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_ambiguity_support_note_preserve")
	if s["version"] != "s20-p333.v1" {
		t.Fatalf("version=%v, want s20-p333.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"primary_compare_source_tag_join_context", "bounded_semi_structured_note"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_ambiguity_support_note_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_ambiguity_support_note_preserve_definition", s["mode"])
	}
}
