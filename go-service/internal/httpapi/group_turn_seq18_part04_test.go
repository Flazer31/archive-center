package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq18P337VRHybridRealism validates the VR hybrid realism summary.
func TestSeq18P337VRHybridRealism(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p337","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_hybrid_realism")
	if s["version"] != "seq18_p337.v1" {
		t.Fatalf("version=%v, want seq18_p337.v1", s["version"])
	}
	if s["role"] != "vr_hybrid_realism" {
		t.Fatalf("role=%v, want vr_hybrid_realism", s["role"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
}

// TestSeq18P338VRSoftRouting validates the VR soft routing summary.
func TestSeq18P338VRSoftRouting(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p338","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_soft_routing")
	if s["version"] != "seq18_p338.v1" {
		t.Fatalf("version=%v, want seq18_p338.v1", s["version"])
	}
	if s["role"] != "vr_soft_routing" {
		t.Fatalf("role=%v, want vr_soft_routing", s["role"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
}

// TestSeq18P339VRLatencyDiscipline validates the VR latency discipline summary.
func TestSeq18P339VRLatencyDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p339","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_latency_discipline")
	if s["version"] != "seq18_p339.v1" {
		t.Fatalf("version=%v, want seq18_p339.v1", s["version"])
	}
	if s["role"] != "vr_latency_discipline" {
		t.Fatalf("role=%v, want vr_latency_discipline", s["role"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
}

// TestSeq18P340VRTruthBoundaryPreserve validates the VR truth-boundary preserve summary.
func TestSeq18P340VRTruthBoundaryPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p340","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_truth_boundary_preserve")
	if s["version"] != "seq18_p340.v1" {
		t.Fatalf("version=%v, want seq18_p340.v1", s["version"])
	}
	if s["role"] != "vr_truth_boundary_preserve" {
		t.Fatalf("role=%v, want vr_truth_boundary_preserve", s["role"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
}

// TestSeq18P344VR18_1aRawTranscript validates the VR 18-1a raw transcript summary.
func TestSeq18P344VR18_1aRawTranscript(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p344","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_18_1a_raw_transcript")
	if s["version"] != "seq18_p344.v1" {
		t.Fatalf("version=%v, want seq18_p344.v1", s["version"])
	}
	if s["role"] != "vr_18_1a_raw_transcript" {
		t.Fatalf("role=%v, want vr_18_1a_raw_transcript", s["role"])
	}
	if s["sub_step"] != "18-1a" {
		t.Fatalf("sub_step=%v, want 18-1a", s["sub_step"])
	}
}

// TestSeq18P345VR18_1bSourceTag validates the VR 18-1b source-tag summary.
func TestSeq18P345VR18_1bSourceTag(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p345","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_18_1b_source_tag")
	if s["version"] != "seq18_p345.v1" {
		t.Fatalf("version=%v, want seq18_p345.v1", s["version"])
	}
	if s["role"] != "vr_18_1b_source_tag" {
		t.Fatalf("role=%v, want vr_18_1b_source_tag", s["role"])
	}
	if s["sub_step"] != "18-1b" {
		t.Fatalf("sub_step=%v, want 18-1b", s["sub_step"])
	}
}

// TestSeq18P346VR18_1cPromptInjection validates the VR 18-1c prompt injection summary.
func TestSeq18P346VR18_1cPromptInjection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p346","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_18_1c_prompt_injection")
	if s["version"] != "seq18_p346.v1" {
		t.Fatalf("version=%v, want seq18_p346.v1", s["version"])
	}
	if s["role"] != "vr_18_1c_prompt_injection" {
		t.Fatalf("role=%v, want vr_18_1c_prompt_injection", s["role"])
	}
	if s["sub_step"] != "18-1c" {
		t.Fatalf("sub_step=%v, want 18-1c", s["sub_step"])
	}
}

// TestSeq18P347VR18_1dHierarchyEscape validates the VR 18-1d hierarchy escape summary.
func TestSeq18P347VR18_1dHierarchyEscape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p347","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_18_1d_hierarchy_escape")
	if s["version"] != "seq18_p347.v1" {
		t.Fatalf("version=%v, want seq18_p347.v1", s["version"])
	}
	if s["role"] != "vr_18_1d_hierarchy_escape" {
		t.Fatalf("role=%v, want vr_18_1d_hierarchy_escape", s["role"])
	}
	if s["sub_step"] != "18-1d" {
		t.Fatalf("sub_step=%v, want 18-1d", s["sub_step"])
	}
}

// TestSeq18P351HY18_2aSemanticKeyword validates the HY 18-2a semantic+keyword summary.
func TestSeq18P351HY18_2aSemanticKeyword(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p351","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_18_2a_semantic_keyword")
	if s["version"] != "seq18_p351.v1" {
		t.Fatalf("version=%v, want seq18_p351.v1", s["version"])
	}
	if s["role"] != "hy_18_2a_semantic_keyword" {
		t.Fatalf("role=%v, want hy_18_2a_semantic_keyword", s["role"])
	}
	if s["sub_step"] != "18-2a" {
		t.Fatalf("sub_step=%v, want 18-2a", s["sub_step"])
	}
}

// TestSeq18P352HY18_2bSoftBias validates the HY 18-2b soft bias summary.
func TestSeq18P352HY18_2bSoftBias(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p352","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_18_2b_soft_bias")
	if s["version"] != "seq18_p352.v1" {
		t.Fatalf("version=%v, want seq18_p352.v1", s["version"])
	}
	if s["role"] != "hy_18_2b_soft_bias" {
		t.Fatalf("role=%v, want hy_18_2b_soft_bias", s["role"])
	}
	if s["sub_step"] != "18-2b" {
		t.Fatalf("sub_step=%v, want 18-2b", s["sub_step"])
	}
}

// TestSeq18P353HY18_2cScoreInspection validates the HY 18-2c score inspection summary.
func TestSeq18P353HY18_2cScoreInspection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p353","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_18_2c_score_inspection")
	if s["version"] != "seq18_p353.v1" {
		t.Fatalf("version=%v, want seq18_p353.v1", s["version"])
	}
	if s["role"] != "hy_18_2c_score_inspection" {
		t.Fatalf("role=%v, want hy_18_2c_score_inspection", s["role"])
	}
	if s["sub_step"] != "18-2c" {
		t.Fatalf("sub_step=%v, want 18-2c", s["sub_step"])
	}
}

// TestSeq18P354HY18_2dAdaptiveTopK validates the HY 18-2d adaptive top-k summary.
func TestSeq18P354HY18_2dAdaptiveTopK(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p354","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_18_2d_adaptive_top_k")
	if s["version"] != "seq18_p354.v1" {
		t.Fatalf("version=%v, want seq18_p354.v1", s["version"])
	}
	if s["role"] != "hy_18_2d_adaptive_topk" {
		t.Fatalf("role=%v, want hy_18_2d_adaptive_topk", s["role"])
	}
	if s["sub_step"] != "18-2d" {
		t.Fatalf("sub_step=%v, want 18-2d", s["sub_step"])
	}
}

// TestSeq18P358QR18_3aQueryClass validates the QR 18-3a query class summary.
func TestSeq18P358QR18_3aQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p358","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_18_3a_query_class")
	if s["version"] != "seq18_p358.v1" {
		t.Fatalf("version=%v, want seq18_p358.v1", s["version"])
	}
	if s["role"] != "qr_18_3a_query_class" {
		t.Fatalf("role=%v, want qr_18_3a_query_class", s["role"])
	}
	if s["sub_step"] != "18-3a" {
		t.Fatalf("sub_step=%v, want 18-3a", s["sub_step"])
	}
}

// TestSeq18P359QR18_3bRetrievalDepth validates the QR 18-3b retrieval depth summary.
func TestSeq18P359QR18_3bRetrievalDepth(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p359","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_18_3b_retrieval_depth")
	if s["version"] != "seq18_p359.v1" {
		t.Fatalf("version=%v, want seq18_p359.v1", s["version"])
	}
	if s["role"] != "qr_18_3b_retrieval_depth" {
		t.Fatalf("role=%v, want qr_18_3b_retrieval_depth", s["role"])
	}
	if s["sub_step"] != "18-3b" {
		t.Fatalf("sub_step=%v, want 18-3b", s["sub_step"])
	}
}

// TestSeq18P360QR18_3cExtractBeforeRead validates the QR 18-3c extract-before-read summary.
func TestSeq18P360QR18_3cExtractBeforeRead(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p360","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_18_3c_extract_before_read")
	if s["version"] != "seq18_p360.v1" {
		t.Fatalf("version=%v, want seq18_p360.v1", s["version"])
	}
	if s["role"] != "qr_18_3c_extract_before_read" {
		t.Fatalf("role=%v, want qr_18_3c_extract_before_read", s["role"])
	}
	if s["sub_step"] != "18-3c" {
		t.Fatalf("sub_step=%v, want 18-3c", s["sub_step"])
	}
}

// TestSeq18P361QR18_3dLongTailRoute validates the QR 18-3d long-tail route summary.
func TestSeq18P361QR18_3dLongTailRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p361","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_18_3d_long_tail_route")
	if s["version"] != "seq18_p361.v1" {
		t.Fatalf("version=%v, want seq18_p361.v1", s["version"])
	}
	if s["role"] != "qr_18_3d_long_tail_route" {
		t.Fatalf("role=%v, want qr_18_3d_long_tail_route", s["role"])
	}
	if s["sub_step"] != "18-3d" {
		t.Fatalf("sub_step=%v, want 18-3d", s["sub_step"])
	}
}

// TestSeq18P365VX18_4aSemanticHybridReplay validates the VX 18-4a semantic hybrid replay summary.
func TestSeq18P365VX18_4aSemanticHybridReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p365","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_18_4a_semantic_hybrid_replay")
	if s["version"] != "seq18_p365.v1" {
		t.Fatalf("version=%v, want seq18_p365.v1", s["version"])
	}
	if s["role"] != "vx_18_4a_semantic_hybrid_replay" {
		t.Fatalf("role=%v, want vx_18_4a_semantic_hybrid_replay", s["role"])
	}
	if s["sub_step"] != "18-4a" {
		t.Fatalf("sub_step=%v, want 18-4a", s["sub_step"])
	}
}

// TestSeq18P366VX18_4bHeldOutRecall validates the VX 18-4b held-out recall summary.
func TestSeq18P366VX18_4bHeldOutRecall(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p366","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_18_4b_held_out_recall")
	if s["version"] != "seq18_p366.v1" {
		t.Fatalf("version=%v, want seq18_p366.v1", s["version"])
	}
	if s["role"] != "vx_18_4b_held_out_recall" {
		t.Fatalf("role=%v, want vx_18_4b_held_out_recall", s["role"])
	}
	if s["sub_step"] != "18-4b" {
		t.Fatalf("sub_step=%v, want 18-4b", s["sub_step"])
	}
}

// TestSeq18P367VX18_4cLatencyToken validates the VX 18-4c latency token summary.
func TestSeq18P367VX18_4cLatencyToken(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p367","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_18_4c_latency_token")
	if s["version"] != "seq18_p367.v1" {
		t.Fatalf("version=%v, want seq18_p367.v1", s["version"])
	}
	if s["role"] != "vx_18_4c_latency_token" {
		t.Fatalf("role=%v, want vx_18_4c_latency_token", s["role"])
	}
	if s["sub_step"] != "18-4c" {
		t.Fatalf("sub_step=%v, want 18-4c", s["sub_step"])
	}
}

// TestSeq18P368VX18_4dTruthBoundaryReplay validates the VX 18-4d truth-boundary replay summary.
func TestSeq18P368VX18_4dTruthBoundaryReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p368","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_18_4d_truth_boundary_replay")
	if s["version"] != "seq18_p368.v1" {
		t.Fatalf("version=%v, want seq18_p368.v1", s["version"])
	}
	if s["role"] != "vx_18_4d_truth_boundary_replay" {
		t.Fatalf("role=%v, want vx_18_4d_truth_boundary_replay", s["role"])
	}
	if s["sub_step"] != "18-4d" {
		t.Fatalf("sub_step=%v, want 18-4d", s["sub_step"])
	}
}

// TestSeq18P369VX18_4eTopKTruncation validates the VX 18-4e top-k truncation summary.
func TestSeq18P369VX18_4eTopKTruncation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p369","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_18_4e_top_k_truncation")
	if s["version"] != "seq18_p369.v1" {
		t.Fatalf("version=%v, want seq18_p369.v1", s["version"])
	}
	if s["role"] != "vx_18_4e_topk_truncation" {
		t.Fatalf("role=%v, want vx_18_4e_topk_truncation", s["role"])
	}
	if s["sub_step"] != "18-4e" {
		t.Fatalf("sub_step=%v, want 18-4e", s["sub_step"])
	}
}

// TestSeq18P373PreReleaseVersionMarker validates the pre-release version marker summary.
func TestSeq18P373PreReleaseVersionMarker(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p373","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_version_marker")
	if s["version"] != "seq18_p373.v1" {
		t.Fatalf("version=%v, want seq18_p373.v1", s["version"])
	}
	if s["role"] != "pre_release_version_marker" {
		t.Fatalf("role=%v, want pre_release_version_marker", s["role"])
	}
	if s["marker_file"] != "1.0.0-pre" {
		t.Fatalf("marker_file=%v, want 1.0.0-pre", s["marker_file"])
	}
	if s["promotion"] != true {
		t.Fatalf("promotion=%v, want true", s["promotion"])
	}
}

// TestSeq18P374PreReleaseBundleAuthority validates the pre-release bundle authority summary.
func TestSeq18P374PreReleaseBundleAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p374","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_bundle_authority")
	if s["version"] != "seq18_p374.v1" {
		t.Fatalf("version=%v, want seq18_p374.v1", s["version"])
	}
	if s["role"] != "pre_release_bundle_authority" {
		t.Fatalf("role=%v, want pre_release_bundle_authority", s["role"])
	}
	if s["bundle_name"] != "Archive Center Pre-release 1.0.0" {
		t.Fatalf("bundle_name=%v, want Archive Center Pre-release 1.0.0", s["bundle_name"])
	}
}

// TestSeq18P375PreReleaseArtifact validates the pre-release artifact summary.
func TestSeq18P375PreReleaseArtifact(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p375","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_artifact")
	if s["version"] != "seq18_p375.v1" {
		t.Fatalf("version=%v, want seq18_p375.v1", s["version"])
	}
	if s["role"] != "pre_release_artifact" {
		t.Fatalf("role=%v, want pre_release_artifact", s["role"])
	}
	if s["validated"] != true {
		t.Fatalf("validated=%v, want true", s["validated"])
	}
}

// TestSeq18P376PreReleaseVRSmoke validates the pre-release VR smoke summary.
func TestSeq18P376PreReleaseVRSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p376","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_vr_smoke")
	if s["version"] != "seq18_p376.v1" {
		t.Fatalf("version=%v, want seq18_p376.v1", s["version"])
	}
	if s["role"] != "pre_release_vr_smoke" {
		t.Fatalf("role=%v, want pre_release_vr_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P377PreReleaseHYSmoke validates the pre-release HY smoke summary.
func TestSeq18P377PreReleaseHYSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p377","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_hy_smoke")
	if s["version"] != "seq18_p377.v1" {
		t.Fatalf("version=%v, want seq18_p377.v1", s["version"])
	}
	if s["role"] != "pre_release_hy_smoke" {
		t.Fatalf("role=%v, want pre_release_hy_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P378PreReleaseQRSmoke validates the pre-release QR smoke summary.
func TestSeq18P378PreReleaseQRSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p378","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_qr_smoke")
	if s["version"] != "seq18_p378.v1" {
		t.Fatalf("version=%v, want seq18_p378.v1", s["version"])
	}
	if s["role"] != "pre_release_qr_smoke" {
		t.Fatalf("role=%v, want pre_release_qr_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P379PreReleaseVXReview validates the pre-release VX review checklist summary.
func TestSeq18P379PreReleaseVXReview(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p379","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_vx_review")
	if s["version"] != "seq18_p379.v1" {
		t.Fatalf("version=%v, want seq18_p379.v1", s["version"])
	}
	if s["role"] != "pre_release_vx_review" {
		t.Fatalf("role=%v, want pre_release_vx_review", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
	components, ok := s["components"].([]any)
	if !ok || len(components) != 4 {
		t.Fatalf("components=%v, want 4 items", s["components"])
	}
}

// TestSeq18P391PreReleaseRawSnippet validates the pre-release raw snippet summary.
func TestSeq18P391PreReleaseRawSnippet(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p391","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_raw_snippet")
	if s["version"] != "seq18_p391.v1" {
		t.Fatalf("version=%v, want seq18_p391.v1", s["version"])
	}
	if s["role"] != "pre_release_raw_snippet" {
		t.Fatalf("role=%v, want pre_release_raw_snippet", s["role"])
	}
	if s["snippet_count"] != float64(3) {
		t.Fatalf("snippet_count=%v, want 3", s["snippet_count"])
	}
	if s["max_chars"] != float64(720) {
		t.Fatalf("max_chars=%v, want 720", s["max_chars"])
	}
	if s["excerpt_chars"] != float64(160) {
		t.Fatalf("excerpt_chars=%v, want 160", s["excerpt_chars"])
	}
}

// TestSeq18P392PreReleaseHybridBias validates the pre-release hybrid bias summary.
func TestSeq18P392PreReleaseHybridBias(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p392","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_hybrid_bias")
	if s["version"] != "seq18_p392.v1" {
		t.Fatalf("version=%v, want seq18_p392.v1", s["version"])
	}
	if s["role"] != "pre_release_hybrid_bias" {
		t.Fatalf("role=%v, want pre_release_hybrid_bias", s["role"])
	}
	if s["speaker"] != float64(0.04) {
		t.Fatalf("speaker=%v, want 0.04", s["speaker"])
	}
	if s["location"] != float64(0.05) {
		t.Fatalf("location=%v, want 0.05", s["location"])
	}
	if s["storyline"] != float64(0.06) {
		t.Fatalf("storyline=%v, want 0.06", s["storyline"])
	}
	if s["cap"] != float64(0.12) {
		t.Fatalf("cap=%v, want 0.12", s["cap"])
	}
}

// TestSeq18P393PreReleaseQueryClassRule validates the pre-release query class rule summary.
func TestSeq18P393PreReleaseQueryClassRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p393","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_query_class_rule")
	if s["version"] != "seq18_p393.v1" {
		t.Fatalf("version=%v, want seq18_p393.v1", s["version"])
	}
	if s["role"] != "pre_release_query_class_rule" {
		t.Fatalf("role=%v, want pre_release_query_class_rule", s["role"])
	}
	if s["heuristic"] != "rule_first_additive_contract" {
		t.Fatalf("heuristic=%v, want rule_first_additive_contract", s["heuristic"])
	}
	if s["execution"] != "fail_open_shared_execution" {
		t.Fatalf("execution=%v, want fail_open_shared_execution", s["execution"])
	}
}

// TestSeq18P394PreReleaseRetrievalNote validates the pre-release retrieval note summary.
func TestSeq18P394PreReleaseRetrievalNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p394","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "pre_release_retrieval_note")
	if s["version"] != "seq18_p394.v1" {
		t.Fatalf("version=%v, want seq18_p394.v1", s["version"])
	}
	if s["role"] != "pre_release_retrieval_note" {
		t.Fatalf("role=%v, want pre_release_retrieval_note", s["role"])
	}
	defaults, ok := s["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("defaults missing or not map")
	}
	if defaults["support_surface_first"] != true {
		t.Fatalf("support_surface_first=%v, want true", defaults["support_surface_first"])
	}
	if defaults["scene_canon_no_extract"] != true {
		t.Fatalf("scene_canon_no_extract=%v, want true", defaults["scene_canon_no_extract"])
	}
	if defaults["callback_resume_temporal_note_only"] != true {
		t.Fatalf("callback_resume_temporal_note_only=%v, want true", defaults["callback_resume_temporal_note_only"])
	}
}
