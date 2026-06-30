package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seq215ArchiveCenterJSSource(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "Archive Center.js"),
		filepath.Join("..", "Archive Center.js"),
		"Archive Center.js",
		filepath.Join("Archive Center 2.0", "Archive Center.js"),
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return string(data)
		}
	}
	t.Fatalf("Archive Center.js not found from test working directory; tried %v", candidates)
	return ""
}

func seq215RequireJSSourceContains(t *testing.T, source string, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(source, marker) {
			t.Fatalf("Archive Center.js missing marker %q", marker)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Preparatory / peripheral extraction tests (P416 ~ P427)
// ---------------------------------------------------------------------------

func TestSeq215P416AuthorityFrozen(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p416","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_authority_frozen")
	if s["version"] != "s215-p416.v1" {
		t.Fatalf("version=%v, want s215-p416.v1", s["version"])
	}
	if s["sub_step"] != "21.5-preparatory" {
		t.Fatalf("sub_step=%v, want 21.5-preparatory", s["sub_step"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)", s["authority_path"])
	}
	if s["mode"] != "seq215_authority_frozen_definition" {
		t.Fatalf("mode=%v, want seq215_authority_frozen_definition", s["mode"])
	}
}

func TestSeq215P417StaleHistoryRejected(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p417","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_history_rejected")
	if s["version"] != "s215-p417.v1" {
		t.Fatalf("version=%v, want s215-p417.v1", s["version"])
	}
	if s["rejected_claim"] != "broad_46_slice_route_service_schema_plan_completed" {
		t.Fatalf("rejected_claim=%v, want broad_46_slice_route_service_schema_plan_completed", s["rejected_claim"])
	}
	if s["mode"] != "seq215_stale_history_rejected_definition" {
		t.Fatalf("mode=%v, want seq215_stale_history_rejected_definition", s["mode"])
	}
}

func TestSeq215P418TurnContractsMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p418","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_turn_contracts_moved")
	if s["version"] != "s215-p418.v1" {
		t.Fatalf("version=%v, want s215-p418.v1", s["version"])
	}
	items, ok := s["moved_surfaces"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("moved_surfaces=%v, want 2 items", s["moved_surfaces"])
	}
	if s["mode"] != "seq215_turn_contracts_moved_definition" {
		t.Fatalf("mode=%v, want seq215_turn_contracts_moved_definition", s["mode"])
	}
}

func TestSeq215P419M3aFormattingMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p419","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m3a_formatting_moved")
	if s["version"] != "s215-p419.v1" {
		t.Fatalf("version=%v, want s215-p419.v1", s["version"])
	}
	if s["moved_cluster"] != "pure_m3a_formatting_helpers" {
		t.Fatalf("moved_cluster=%v, want pure_m3a_formatting_helpers", s["moved_cluster"])
	}
	if s["mode"] != "seq215_m3a_formatting_moved_definition" {
		t.Fatalf("mode=%v, want seq215_m3a_formatting_moved_definition", s["mode"])
	}
}

func TestSeq215P420ProxyConfigMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p420","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_proxy_config_moved")
	if s["version"] != "s215-p420.v1" {
		t.Fatalf("version=%v, want s215-p420.v1", s["version"])
	}
	items, ok := s["moved_surfaces"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("moved_surfaces=%v, want 2 items", s["moved_surfaces"])
	}
	if s["mode"] != "seq215_proxy_config_moved_definition" {
		t.Fatalf("mode=%v, want seq215_proxy_config_moved_definition", s["mode"])
	}
}

func TestSeq215P421MaintenanceQueueMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p421","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_maintenance_queue_moved")
	if s["version"] != "s215-p421.v1" {
		t.Fatalf("version=%v, want s215-p421.v1", s["version"])
	}
	if s["moved_surface"] != "maintenance_queue_layer" {
		t.Fatalf("moved_surface=%v, want maintenance_queue_layer", s["moved_surface"])
	}
	if s["mode"] != "seq215_maintenance_queue_moved_definition" {
		t.Fatalf("mode=%v, want seq215_maintenance_queue_moved_definition", s["mode"])
	}
}

func TestSeq215P422ChromaC17Moved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p422","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_chroma_c17_moved")
	if s["version"] != "s215-p422.v1" {
		t.Fatalf("version=%v, want s215-p422.v1", s["version"])
	}
	if s["moved_surface"] != "chroma_shadow_c17_owner_functions" {
		t.Fatalf("moved_surface=%v, want chroma_shadow_c17_owner_functions", s["moved_surface"])
	}
	if s["mode"] != "seq215_chroma_c17_moved_definition" {
		t.Fatalf("mode=%v, want seq215_chroma_c17_moved_definition", s["mode"])
	}
}

func TestSeq215P423Step17HelpersExtracted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p423","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_step17_helpers_extracted")
	if s["version"] != "s215-p423.v1" {
		t.Fatalf("version=%v, want s215-p423.v1", s["version"])
	}
	if s["moved_surface"] != "step17_generic_helpers" {
		t.Fatalf("moved_surface=%v, want step17_generic_helpers", s["moved_surface"])
	}
	if s["mode"] != "seq215_step17_helpers_extracted_definition" {
		t.Fatalf("mode=%v, want seq215_step17_helpers_extracted_definition", s["mode"])
	}
}

func TestSeq215P424LC1PhaseAMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p424","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_lc1_phase_a_moved")
	if s["version"] != "s215-p424.v1" {
		t.Fatalf("version=%v, want s215-p424.v1", s["version"])
	}
	items, ok := s["moved_builders"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("moved_builders=%v, want 3 items", s["moved_builders"])
	}
	if s["mode"] != "seq215_lc1_phase_a_moved_definition" {
		t.Fatalf("mode=%v, want seq215_lc1_phase_a_moved_definition", s["mode"])
	}
}

func TestSeq215P425LC1PhaseBCDMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p425","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_lc1_phase_bcd_moved")
	if s["version"] != "s215-p425.v1" {
		t.Fatalf("version=%v, want s215-p425.v1", s["version"])
	}
	items, ok := s["moved_builders"].([]any)
	if !ok || len(items) != 6 {
		t.Fatalf("moved_builders=%v, want 6 items", s["moved_builders"])
	}
	if s["mode"] != "seq215_lc1_phase_bcd_moved_definition" {
		t.Fatalf("mode=%v, want seq215_lc1_phase_bcd_moved_definition", s["mode"])
	}
}

func TestSeq215P426UtilityServicesMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p426","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_utility_services_moved")
	if s["version"] != "s215-p426.v1" {
		t.Fatalf("version=%v, want s215-p426.v1", s["version"])
	}
	items, ok := s["moved_services"].([]any)
	if !ok || len(items) != 5 {
		t.Fatalf("moved_services=%v, want 5 items", s["moved_services"])
	}
	if s["mode"] != "seq215_utility_services_moved_definition" {
		t.Fatalf("mode=%v, want seq215_utility_services_moved_definition", s["mode"])
	}
}

func TestSeq215P427PhysicalBaselineRecorded(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p427","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_physical_baseline_recorded")
	if s["version"] != "s215-p427.v1" {
		t.Fatalf("version=%v, want s215-p427.v1", s["version"])
	}
	if s["physical_lines"] != float64(27196) {
		t.Fatalf("physical_lines=%v, want 27196", s["physical_lines"])
	}
	if s["route_decorators"] != float64(129) {
		t.Fatalf("route_decorators=%v, want 129", s["route_decorators"])
	}
	if s["basemodel_classes"] != float64(49) {
		t.Fatalf("basemodel_classes=%v, want 49", s["basemodel_classes"])
	}
	if s["toplevel_functions"] != float64(414) {
		t.Fatalf("toplevel_functions=%v, want 414", s["toplevel_functions"])
	}
	if s["sha256"] != "CB39A9A7606DCB8161B22FC2F53530461DFAF08BE2DB95C9619A88BA802AAA7E" {
		t.Fatalf("sha256=%v, want CB39A9A7606DCB8161B22FC2F53530461DFAF08BE2DB95C9619A88BA802AAA7E", s["sha256"])
	}
	if s["mode"] != "seq215_physical_baseline_recorded_definition" {
		t.Fatalf("mode=%v, want seq215_physical_baseline_recorded_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Pre-core cleanup tests (P431 ~ P432)
// ---------------------------------------------------------------------------

func TestSeq215P431WI14Removed(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p431","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_wi14_removed")
	if s["version"] != "s215-p431.v1" {
		t.Fatalf("version=%v, want s215-p431.v1", s["version"])
	}
	if s["removed_surface"] != "wi14_hardcoded_weak_input_steering" {
		t.Fatalf("removed_surface=%v, want wi14_hardcoded_weak_input_steering", s["removed_surface"])
	}
	if s["constraint"] != "short_control_phrases_not_promoted_to_long_memory" {
		t.Fatalf("constraint=%v, want short_control_phrases_not_promoted_to_long_memory", s["constraint"])
	}
	if s["mode"] != "seq215_wi14_removed_definition" {
		t.Fatalf("mode=%v, want seq215_wi14_removed_definition", s["mode"])
	}
}

func TestSeq215P432WI14DeletionRecords(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p432","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_wi14_deletion_records")
	if s["version"] != "s215-p432.v1" {
		t.Fatalf("version=%v, want s215-p432.v1", s["version"])
	}
	if s["record_type"] != "before_after_line_counts" {
		t.Fatalf("record_type=%v, want before_after_line_counts", s["record_type"])
	}
	if s["validation_type"] != "focused_validation" {
		t.Fatalf("validation_type=%v, want focused_validation", s["validation_type"])
	}
	if s["mode"] != "seq215_wi14_deletion_records_definition" {
		t.Fatalf("mode=%v, want seq215_wi14_deletion_records_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Core extraction / deferral tests (P436 ~ P448)
// ---------------------------------------------------------------------------

func TestSeq215P436RunMaintenancePassBlocked(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p436","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_run_maintenance_pass_blocked")
	if s["version"] != "s215-p436.v1" {
		t.Fatalf("version=%v, want s215-p436.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-deferred" {
		t.Fatalf("sub_step=%v, want 21.5-core-deferred", s["sub_step"])
	}
	if s["blocked_surface"] != "run_maintenance_pass_tm1_helper" {
		t.Fatalf("blocked_surface=%v, want run_maintenance_pass_tm1_helper", s["blocked_surface"])
	}
	if s["block_reason"] != "helper_dependencies_too_cross_cutting_for_safe_extraction" {
		t.Fatalf("block_reason=%v, want helper_dependencies_too_cross_cutting_for_safe_extraction", s["block_reason"])
	}
	if s["mode"] != "seq215_run_maintenance_pass_blocked_definition" {
		t.Fatalf("mode=%v, want seq215_run_maintenance_pass_blocked_definition", s["mode"])
	}
}

func TestSeq215P437CompleteTurnM4Extracted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p437","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_complete_turn_m4_extracted")
	if s["version"] != "s215-p437.v1" {
		t.Fatalf("version=%v, want s215-p437.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-extracted" {
		t.Fatalf("sub_step=%v, want 21.5-core-extracted", s["sub_step"])
	}
	if s["destination"] != "backend/services/complete_turn.py" {
		t.Fatalf("destination=%v, want backend/services/complete_turn.py", s["destination"])
	}
	if s["thin_wrapper"] != true {
		t.Fatalf("thin_wrapper=%v, want true", s["thin_wrapper"])
	}
	if s["no_import_backend_main"] != true {
		t.Fatalf("no_import_backend_main=%v, want true", s["no_import_backend_main"])
	}
	if s["mode"] != "seq215_complete_turn_m4_extracted_definition" {
		t.Fatalf("mode=%v, want seq215_complete_turn_m4_extracted_definition", s["mode"])
	}
}

func TestSeq215P438PrepareTurnExtracted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p438","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_extracted")
	if s["version"] != "s215-p438.v1" {
		t.Fatalf("version=%v, want s215-p438.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-extracted" {
		t.Fatalf("sub_step=%v, want 21.5-core-extracted", s["sub_step"])
	}
	if s["destination"] != "backend/services/prepare_turn.py" {
		t.Fatalf("destination=%v, want backend/services/prepare_turn.py", s["destination"])
	}
	if s["thin_wrapper"] != true {
		t.Fatalf("thin_wrapper=%v, want true", s["thin_wrapper"])
	}
	if s["no_import_backend_main"] != true {
		t.Fatalf("no_import_backend_main=%v, want true", s["no_import_backend_main"])
	}
	if s["mode"] != "seq215_prepare_turn_extracted_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_extracted_definition", s["mode"])
	}
}

func TestSeq215P439BundleSupervisorReduced(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p439","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_bundle_supervisor_reduced")
	if s["version"] != "s215-p439.v1" {
		t.Fatalf("version=%v, want s215-p439.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-reduced" {
		t.Fatalf("sub_step=%v, want 21.5-core-reduced", s["sub_step"])
	}
	items, ok := s["moved_logic"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("moved_logic=%v, want 3 items", s["moved_logic"])
	}
	if s["thin_wrapper"] != true {
		t.Fatalf("thin_wrapper=%v, want true", s["thin_wrapper"])
	}
	if s["callbacks_remain"] != true {
		t.Fatalf("callbacks_remain=%v, want true", s["callbacks_remain"])
	}
	if s["mode"] != "seq215_bundle_supervisor_reduced_definition" {
		t.Fatalf("mode=%v, want seq215_bundle_supervisor_reduced_definition", s["mode"])
	}
}

func TestSeq215P440BundleRecallReduced(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p440","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_bundle_recall_reduced")
	if s["version"] != "s215-p440.v1" {
		t.Fatalf("version=%v, want s215-p440.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-reduced" {
		t.Fatalf("sub_step=%v, want 21.5-core-reduced", s["sub_step"])
	}
	if s["destination"] != "backend/services/prepare_turn_recall.py" {
		t.Fatalf("destination=%v, want backend/services/prepare_turn_recall.py", s["destination"])
	}
	if s["thin_wrapper"] != true {
		t.Fatalf("thin_wrapper=%v, want true", s["thin_wrapper"])
	}
	if s["mode"] != "seq215_bundle_recall_reduced_definition" {
		t.Fatalf("mode=%v, want seq215_bundle_recall_reduced_definition", s["mode"])
	}
}

func TestSeq215P441BundleInjectionReduced(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p441","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_bundle_injection_reduced")
	if s["version"] != "s215-p441.v1" {
		t.Fatalf("version=%v, want s215-p441.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-reduced" {
		t.Fatalf("sub_step=%v, want 21.5-core-reduced", s["sub_step"])
	}
	if s["destination"] != "backend/services/prepare_turn_injection.py" {
		t.Fatalf("destination=%v, want backend/services/prepare_turn_injection.py", s["destination"])
	}
	if s["thin_wrapper"] != true {
		t.Fatalf("thin_wrapper=%v, want true", s["thin_wrapper"])
	}
	if s["mode"] != "seq215_bundle_injection_reduced_definition" {
		t.Fatalf("mode=%v, want seq215_bundle_injection_reduced_definition", s["mode"])
	}
}

func TestSeq215P442LC1RemainingMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p442","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_lc1_remaining_moved")
	if s["version"] != "s215-p442.v1" {
		t.Fatalf("version=%v, want s215-p442.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-extracted" {
		t.Fatalf("sub_step=%v, want 21.5-core-extracted", s["sub_step"])
	}
	items, ok := s["moved_builders"].([]any)
	if !ok || len(items) != 6 {
		t.Fatalf("moved_builders=%v, want 6 items", s["moved_builders"])
	}
	if s["imported_by_main_py"] != true {
		t.Fatalf("imported_by_main_py=%v, want true", s["imported_by_main_py"])
	}
	if s["preserved_constant"] != "_LC1J_MAX_FALSE_POSITIVE_INCREASE" {
		t.Fatalf("preserved_constant=%v, want _LC1J_MAX_FALSE_POSITIVE_INCREASE", s["preserved_constant"])
	}
	if s["mode"] != "seq215_lc1_remaining_moved_definition" {
		t.Fatalf("mode=%v, want seq215_lc1_remaining_moved_definition", s["mode"])
	}
}

func TestSeq215P443NarrativeReadLock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p443","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_narrative_read_lock")
	if s["version"] != "s215-p443.v1" {
		t.Fatalf("version=%v, want s215-p443.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-locked" {
		t.Fatalf("sub_step=%v, want 21.5-core-locked", s["sub_step"])
	}
	if s["lock_type"] != "dependency_map_plus_test_lock" {
		t.Fatalf("lock_type=%v, want dependency_map_plus_test_lock", s["lock_type"])
	}
	if s["test_count"] != float64(12) {
		t.Fatalf("test_count=%v, want 12", s["test_count"])
	}
	if s["mode"] != "seq215_narrative_read_lock_definition" {
		t.Fatalf("mode=%v, want seq215_narrative_read_lock_definition", s["mode"])
	}
}

func TestSeq215P444HypamemoryExtracted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p444","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_hypamemory_extracted")
	if s["version"] != "s215-p444.v1" {
		t.Fatalf("version=%v, want s215-p444.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-extracted" {
		t.Fatalf("sub_step=%v, want 21.5-core-extracted", s["sub_step"])
	}
	if s["background_task_preserved"] != true {
		t.Fatalf("background_task_preserved=%v, want true", s["background_task_preserved"])
	}
	if s["per_item_fail_open_locked"] != true {
		t.Fatalf("per_item_fail_open_locked=%v, want true", s["per_item_fail_open_locked"])
	}
	if s["mode"] != "seq215_hypamemory_extracted_definition" {
		t.Fatalf("mode=%v, want seq215_hypamemory_extracted_definition", s["mode"])
	}
}

func TestSeq215P445ArchiveCenterJSDeferral(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p445","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_archive_center_js_deferral")
	if s["version"] != "s215-p445.v1" {
		t.Fatalf("version=%v, want s215-p445.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-deferred" {
		t.Fatalf("sub_step=%v, want 21.5-core-deferred", s["sub_step"])
	}
	if s["true_offload"] != false {
		t.Fatalf("true_offload=%v, want false", s["true_offload"])
	}
	items, ok := s["plugin_retains"].([]any)
	if !ok || len(items) != 6 {
		t.Fatalf("plugin_retains=%v, want 6 items", s["plugin_retains"])
	}
	if s["mode"] != "seq215_archive_center_js_deferral_definition" {
		t.Fatalf("mode=%v, want seq215_archive_center_js_deferral_definition", s["mode"])
	}
}

func TestSeq215P446OR1eRechecked(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p446","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_or1e_rechecked")
	if s["version"] != "s215-p446.v1" {
		t.Fatalf("version=%v, want s215-p446.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-verified" {
		t.Fatalf("sub_step=%v, want 21.5-core-verified", s["sub_step"])
	}
	if s["split_status"] != "trace_contract_only" {
		t.Fatalf("split_status=%v, want trace_contract_only", s["split_status"])
	}
	if s["js_changes_made"] != false {
		t.Fatalf("js_changes_made=%v, want false", s["js_changes_made"])
	}
	if s["mode"] != "seq215_or1e_rechecked_definition" {
		t.Fatalf("mode=%v, want seq215_or1e_rechecked_definition", s["mode"])
	}
}

func TestSeq215P447FinalValidation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p447","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_final_validation")
	if s["version"] != "s215-p447.v1" {
		t.Fatalf("version=%v, want s215-p447.v1", s["version"])
	}
	if s["sub_step"] != "21.5-core-validated" {
		t.Fatalf("sub_step=%v, want 21.5-core-validated", s["sub_step"])
	}
	if s["backend_tests_total"] != float64(714) {
		t.Fatalf("backend_tests_total=%v, want 714", s["backend_tests_total"])
	}
	if s["backend_tests_passed"] != float64(714) {
		t.Fatalf("backend_tests_passed=%v, want 714", s["backend_tests_passed"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_final_validation_definition" {
		t.Fatalf("mode=%v, want seq215_final_validation_definition", s["mode"])
	}
}

func TestSeq215P448StepComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p448","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_step_complete")
	if s["version"] != "s215-p448.v1" {
		t.Fatalf("version=%v, want s215-p448.v1", s["version"])
	}
	if s["sub_step"] != "21.5-complete" {
		t.Fatalf("sub_step=%v, want 21.5-complete", s["sub_step"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["open_core_items"] != "all_completed_or_blocked_with_evidence" {
		t.Fatalf("open_core_items=%v, want all_completed_or_blocked_with_evidence", s["open_core_items"])
	}
	items, ok := s["remaining_work"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("remaining_work=%v, want 2 items", s["remaining_work"])
	}
	if s["remaining_target"] != "later_roadmap" {
		t.Fatalf("remaining_target=%v, want later_roadmap", s["remaining_target"])
	}
	if s["mode"] != "seq215_step_complete_definition" {
		t.Fatalf("mode=%v, want seq215_step_complete_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 WI14 deletion slice tests (P476 ~ P488)
// ---------------------------------------------------------------------------

func TestSeq215P476AuthorityRestate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p476","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_authority_restate")
	if s["version"] != "s215-p476.v1" {
		t.Fatalf("version=%v, want s215-p476.v1", s["version"])
	}
	if s["sub_step"] != "21.5-wi14-closeout" {
		t.Fatalf("sub_step=%v, want 21.5-wi14-closeout", s["sub_step"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)/backend/main.py" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)/backend/main.py", s["authority_path"])
	}
	if s["mode"] != "seq215_authority_restate_definition" {
		t.Fatalf("mode=%v, want seq215_authority_restate_definition", s["mode"])
	}
}

func TestSeq215P477BeforeCount(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p477","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_before_count")
	if s["version"] != "s215-p477.v1" {
		t.Fatalf("version=%v, want s215-p477.v1", s["version"])
	}
	if s["physical_lines"] != float64(28216) {
		t.Fatalf("physical_lines=%v, want 28216", s["physical_lines"])
	}
	if s["mode"] != "seq215_before_count_definition" {
		t.Fatalf("mode=%v, want seq215_before_count_definition", s["mode"])
	}
}

func TestSeq215P478ExactUsageSearch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p478","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_exact_usage_search")
	if s["version"] != "s215-p478.v1" {
		t.Fatalf("version=%v, want s215-p478.v1", s["version"])
	}
	items, ok := s["searched_symbols"].([]any)
	if !ok || len(items) != 5 {
		t.Fatalf("searched_symbols=%v, want 5 items", s["searched_symbols"])
	}
	if s["mode"] != "seq215_exact_usage_search_definition" {
		t.Fatalf("mode=%v, want seq215_exact_usage_search_definition", s["mode"])
	}
}

func TestSeq215P479DeleteMinimalContinuationCues(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p479","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_delete_minimal_continuation_cues")
	if s["version"] != "s215-p479.v1" {
		t.Fatalf("version=%v, want s215-p479.v1", s["version"])
	}
	if s["deleted_symbol"] != "_WI14_MINIMAL_CONTINUATION_CUES" {
		t.Fatalf("deleted_symbol=%v, want _WI14_MINIMAL_CONTINUATION_CUES", s["deleted_symbol"])
	}
	if s["mode"] != "seq215_delete_minimal_continuation_cues_definition" {
		t.Fatalf("mode=%v, want seq215_delete_minimal_continuation_cues_definition", s["mode"])
	}
}

func TestSeq215P480DeleteExplicitCorrectionMarkers(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p480","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_delete_explicit_correction_markers")
	if s["version"] != "s215-p480.v1" {
		t.Fatalf("version=%v, want s215-p480.v1", s["version"])
	}
	if s["deleted_symbol"] != "_WI14_EXPLICIT_CORRECTION_MARKERS" {
		t.Fatalf("deleted_symbol=%v, want _WI14_EXPLICIT_CORRECTION_MARKERS", s["deleted_symbol"])
	}
	if s["mode"] != "seq215_delete_explicit_correction_markers_definition" {
		t.Fatalf("mode=%v, want seq215_delete_explicit_correction_markers_definition", s["mode"])
	}
}

func TestSeq215P481DeleteDetectInputMode(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p481","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_delete_detect_input_mode")
	if s["version"] != "s215-p481.v1" {
		t.Fatalf("version=%v, want s215-p481.v1", s["version"])
	}
	if s["deleted_symbol"] != "_wi14_detect_input_mode" {
		t.Fatalf("deleted_symbol=%v, want _wi14_detect_input_mode", s["deleted_symbol"])
	}
	if s["mode"] != "seq215_delete_detect_input_mode_definition" {
		t.Fatalf("mode=%v, want seq215_delete_detect_input_mode_definition", s["mode"])
	}
}

func TestSeq215P482RemoveBuildWeakInputSteering(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p482","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_remove_build_weak_input_steering")
	if s["version"] != "s215-p482.v1" {
		t.Fatalf("version=%v, want s215-p482.v1", s["version"])
	}
	if s["deleted_symbol"] != "_build_weak_input_steering_wi14" {
		t.Fatalf("deleted_symbol=%v, want _build_weak_input_steering_wi14", s["deleted_symbol"])
	}
	if s["call_sites_removed"] != true {
		t.Fatalf("call_sites_removed=%v, want true", s["call_sites_removed"])
	}
	if s["mode"] != "seq215_remove_build_weak_input_steering_definition" {
		t.Fatalf("mode=%v, want seq215_remove_build_weak_input_steering_definition", s["mode"])
	}
}

func TestSeq215P483SimplifySupervisorPlanner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p483","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_simplify_supervisor_planner")
	if s["version"] != "s215-p483.v1" {
		t.Fatalf("version=%v, want s215-p483.v1", s["version"])
	}
	items, ok := s["simplified_paths"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("simplified_paths=%v, want 3 items", s["simplified_paths"])
	}
	if s["hardcoded_inference_removed"] != true {
		t.Fatalf("hardcoded_inference_removed=%v, want true", s["hardcoded_inference_removed"])
	}
	if s["mode"] != "seq215_simplify_supervisor_planner_definition" {
		t.Fatalf("mode=%v, want seq215_simplify_supervisor_planner_definition", s["mode"])
	}
}

func TestSeq215P484KeepAutoAdvanceExplicit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p484","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_keep_auto_advance_explicit")
	if s["version"] != "s215-p484.v1" {
		t.Fatalf("version=%v, want s215-p484.v1", s["version"])
	}
	if s["auto_advance_source"] != "explicit_trigger_fields" {
		t.Fatalf("auto_advance_source=%v, want explicit_trigger_fields", s["auto_advance_source"])
	}
	if s["backend_phrase_detection"] != false {
		t.Fatalf("backend_phrase_detection=%v, want false", s["backend_phrase_detection"])
	}
	if s["mode"] != "seq215_keep_auto_advance_explicit_definition" {
		t.Fatalf("mode=%v, want seq215_keep_auto_advance_explicit_definition", s["mode"])
	}
}

func TestSeq215P485PyCompilePass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p485","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_py_compile_pass")
	if s["version"] != "s215-p485.v1" {
		t.Fatalf("version=%v, want s215-p485.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_pass_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_pass_definition", s["mode"])
	}
}

func TestSeq215P486FocusedBackendTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p486","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_focused_backend_tests")
	if s["version"] != "s215-p486.v1" {
		t.Fatalf("version=%v, want s215-p486.v1", s["version"])
	}
	if s["tests_run"] != float64(32) {
		t.Fatalf("tests_run=%v, want 32", s["tests_run"])
	}
	if s["tests_passed"] != float64(32) {
		t.Fatalf("tests_passed=%v, want 32", s["tests_passed"])
	}
	if s["mode"] != "seq215_focused_backend_tests_definition" {
		t.Fatalf("mode=%v, want seq215_focused_backend_tests_definition", s["mode"])
	}
}

func TestSeq215P487JSUntouched(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p487","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_untouched")
	if s["version"] != "s215-p487.v1" {
		t.Fatalf("version=%v, want s215-p487.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_js_untouched_definition" {
		t.Fatalf("mode=%v, want seq215_js_untouched_definition", s["mode"])
	}
}

func TestSeq215P488AfterCount(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p488","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_after_count")
	if s["version"] != "s215-p488.v1" {
		t.Fatalf("version=%v, want s215-p488.v1", s["version"])
	}
	if s["after_lines"] != float64(27942) {
		t.Fatalf("after_lines=%v, want 27942", s["after_lines"])
	}
	if s["delta_lines"] != float64(-274) {
		t.Fatalf("delta_lines=%v, want -274", s["delta_lines"])
	}
	if s["mode"] != "seq215_after_count_definition" {
		t.Fatalf("mode=%v, want seq215_after_count_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Authority verification and guard tests (P556 ~ P566)
// ---------------------------------------------------------------------------

func TestSeq215P556JSAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p556","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_authority")
	if s["version"] != "s215-p556.v1" {
		t.Fatalf("version=%v, want s215-p556.v1", s["version"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)/Archive Center.js" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)/Archive Center.js", s["authority_path"])
	}
	if s["mode"] != "seq215_js_authority_definition" {
		t.Fatalf("mode=%v, want seq215_js_authority_definition", s["mode"])
	}
}

func TestSeq215P557BackendAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p557","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backend_authority")
	if s["version"] != "s215-p557.v1" {
		t.Fatalf("version=%v, want s215-p557.v1", s["version"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)/backend/main.py" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)/backend/main.py", s["authority_path"])
	}
	if s["mode"] != "seq215_backend_authority_definition" {
		t.Fatalf("mode=%v, want seq215_backend_authority_definition", s["mode"])
	}
}

func TestSeq215P558NoRootStandalonePair(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p558","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_root_standalone_pair")
	if s["version"] != "s215-p558.v1" {
		t.Fatalf("version=%v, want s215-p558.v1", s["version"])
	}
	if s["root_has_standalone"] != false {
		t.Fatalf("root_has_standalone=%v, want false", s["root_has_standalone"])
	}
	if s["root_backend_pair"] != false {
		t.Fatalf("root_backend_pair=%v, want false", s["root_backend_pair"])
	}
	if s["mode"] != "seq215_no_root_standalone_pair_definition" {
		t.Fatalf("mode=%v, want seq215_no_root_standalone_pair_definition", s["mode"])
	}
}

func TestSeq215P559BackupNotAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p559","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backup_not_authority")
	if s["version"] != "s215-p559.v1" {
		t.Fatalf("version=%v, want s215-p559.v1", s["version"])
	}
	if s["is_current_authority"] != false {
		t.Fatalf("is_current_authority=%v, want false", s["is_current_authority"])
	}
	if s["mode"] != "seq215_backup_not_authority_definition" {
		t.Fatalf("mode=%v, want seq215_backup_not_authority_definition", s["mode"])
	}
}

func TestSeq215P560DeployNotAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p560","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_deploy_not_authority")
	if s["version"] != "s215-p560.v1" {
		t.Fatalf("version=%v, want s215-p560.v1", s["version"])
	}
	if s["is_current_authority"] != false {
		t.Fatalf("is_current_authority=%v, want false", s["is_current_authority"])
	}
	if s["mode"] != "seq215_deploy_not_authority_definition" {
		t.Fatalf("mode=%v, want seq215_deploy_not_authority_definition", s["mode"])
	}
}

func TestSeq215P561NoBroadSplitBeforeNarrow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p561","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_broad_split_before_narrow")
	if s["version"] != "s215-p561.v1" {
		t.Fatalf("version=%v, want s215-p561.v1", s["version"])
	}
	if s["broad_routes_landed"] != false {
		t.Fatalf("broad_routes_landed=%v, want false", s["broad_routes_landed"])
	}
	if s["broad_services_landed"] != false {
		t.Fatalf("broad_services_landed=%v, want false", s["broad_services_landed"])
	}
	if s["broad_schemas_landed"] != false {
		t.Fatalf("broad_schemas_landed=%v, want false", s["broad_schemas_landed"])
	}
	if s["narrow_proxy_config"] != true {
		t.Fatalf("narrow_proxy_config=%v, want true", s["narrow_proxy_config"])
	}
	if s["mode"] != "seq215_no_broad_split_before_narrow_definition" {
		t.Fatalf("mode=%v, want seq215_no_broad_split_before_narrow_definition", s["mode"])
	}
}

func TestSeq215P562StaleSplitRejectedContext(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p562","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_split_rejected_context")
	if s["version"] != "s215-p562.v1" {
		t.Fatalf("version=%v, want s215-p562.v1", s["version"])
	}
	if s["rejected_claim"] != "broad_route_service_schema_split_landed" {
		t.Fatalf("rejected_claim=%v, want broad_route_service_schema_split_landed", s["rejected_claim"])
	}
	if s["mode"] != "seq215_stale_split_rejected_context_definition" {
		t.Fatalf("mode=%v, want seq215_stale_split_rejected_context_definition", s["mode"])
	}
}

func TestSeq215P563StaleSplitRejectedProgress(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p563","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_split_rejected_progress")
	if s["version"] != "s215-p563.v1" {
		t.Fatalf("version=%v, want s215-p563.v1", s["version"])
	}
	if s["rejected_claim"] != "broad_route_service_schema_split_landed" {
		t.Fatalf("rejected_claim=%v, want broad_route_service_schema_split_landed", s["rejected_claim"])
	}
	if s["mode"] != "seq215_stale_split_rejected_progress_definition" {
		t.Fatalf("mode=%v, want seq215_stale_split_rejected_progress_definition", s["mode"])
	}
}

func TestSeq215P564Beta08Metrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p564","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_beta08_metrics")
	if s["version"] != "s215-p564.v1" {
		t.Fatalf("version=%v, want s215-p564.v1", s["version"])
	}
	if s["js_lines"] != float64(31946) {
		t.Fatalf("js_lines=%v, want 31946", s["js_lines"])
	}
	if s["backend_lines"] != float64(23720) {
		t.Fatalf("backend_lines=%v, want 23720", s["backend_lines"])
	}
	if s["route_decorators"] != float64(129) {
		t.Fatalf("route_decorators=%v, want 129", s["route_decorators"])
	}
	if s["app_decorators_total"] != float64(130) {
		t.Fatalf("app_decorators_total=%v, want 130", s["app_decorators_total"])
	}
	if s["base_model_classes"] != float64(49) {
		t.Fatalf("base_model_classes=%v, want 49", s["base_model_classes"])
	}
	if s["top_level_functions"] != float64(408) {
		t.Fatalf("top_level_functions=%v, want 408", s["top_level_functions"])
	}
	if s["metrics_source"] != "PROGRESS/CONTEXT 21.5th step final metrics" {
		t.Fatalf("metrics_source=%v", s["metrics_source"])
	}
	if s["mode"] != "seq215_beta08_metrics_definition" {
		t.Fatalf("mode=%v, want seq215_beta08_metrics_definition", s["mode"])
	}
}

func TestSeq215P565RestateGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p565","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_restate_guard")
	if s["version"] != "s215-p565.v1" {
		t.Fatalf("version=%v, want s215-p565.v1", s["version"])
	}
	if s["guard_rule"] != "restate_active_edit_folder_before_each_slice" {
		t.Fatalf("guard_rule=%v, want restate_active_edit_folder_before_each_slice", s["guard_rule"])
	}
	if s["mode"] != "seq215_restate_guard_definition" {
		t.Fatalf("mode=%v, want seq215_restate_guard_definition", s["mode"])
	}
}

func TestSeq215P566PromoteGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p566","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_promote_guard")
	if s["version"] != "s215-p566.v1" {
		t.Fatalf("version=%v, want s215-p566.v1", s["version"])
	}
	if s["guard_rule"] != "update_context_before_promoting_backup_deploy" {
		t.Fatalf("guard_rule=%v, want update_context_before_promoting_backup_deploy", s["guard_rule"])
	}
	if s["mode"] != "seq215_promote_guard_definition" {
		t.Fatalf("mode=%v, want seq215_promote_guard_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Turn-contracts slice tests (P589 ~ P604)
// ---------------------------------------------------------------------------

func TestSeq215P589TurnContractsCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p589","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_turn_contracts_created")
	if s["version"] != "s215-p589.v1" {
		t.Fatalf("version=%v, want s215-p589.v1", s["version"])
	}
	if s["file_created"] != "backend/turn_contracts.py" {
		t.Fatalf("file_created=%v, want backend/turn_contracts.py", s["file_created"])
	}
	if s["module_created"] != "backend.turn_contracts" {
		t.Fatalf("module_created=%v, want backend.turn_contracts", s["module_created"])
	}
	if s["mode"] != "seq215_turn_contracts_created_definition" {
		t.Fatalf("mode=%v, want seq215_turn_contracts_created_definition", s["mode"])
	}
}

func TestSeq215P590CompleteTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p590","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_complete_turn_request_moved")
	if s["version"] != "s215-p590.v1" {
		t.Fatalf("version=%v, want s215-p590.v1", s["version"])
	}
	if s["class_name"] != "CompleteTurnRequest" {
		t.Fatalf("class_name=%v, want CompleteTurnRequest", s["class_name"])
	}
	if s["target_file"] != "backend/turn_contracts.py" {
		t.Fatalf("target_file=%v, want backend/turn_contracts.py", s["target_file"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_complete_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_complete_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P591M4CompleteTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p591","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m4_complete_turn_request_moved")
	if s["version"] != "s215-p591.v1" {
		t.Fatalf("version=%v, want s215-p591.v1", s["version"])
	}
	if s["class_name"] != "M4CompleteTurnRequest" {
		t.Fatalf("class_name=%v, want M4CompleteTurnRequest", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_m4_complete_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_m4_complete_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P592M4CompleteTurnResponseMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p592","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m4_complete_turn_response_moved")
	if s["version"] != "s215-p592.v1" {
		t.Fatalf("version=%v, want s215-p592.v1", s["version"])
	}
	if s["class_name"] != "M4CompleteTurnResponse" {
		t.Fatalf("class_name=%v, want M4CompleteTurnResponse", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_m4_complete_turn_response_moved_definition" {
		t.Fatalf("mode=%v, want seq215_m4_complete_turn_response_moved_definition", s["mode"])
	}
}

func TestSeq215P593PrepareTurnSettingsMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p593","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_settings_moved")
	if s["version"] != "s215-p593.v1" {
		t.Fatalf("version=%v, want s215-p593.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnSettings" {
		t.Fatalf("class_name=%v, want PrepareTurnSettings", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_settings_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_settings_moved_definition", s["mode"])
	}
}

func TestSeq215P594PrepareTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p594","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_request_moved")
	if s["version"] != "s215-p594.v1" {
		t.Fatalf("version=%v, want s215-p594.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnRequest" {
		t.Fatalf("class_name=%v, want PrepareTurnRequest", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P595RetrievalDocumentQ1AMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p595","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_retrieval_document_q1a_moved")
	if s["version"] != "s215-p595.v1" {
		t.Fatalf("version=%v, want s215-p595.v1", s["version"])
	}
	if s["class_name"] != "RetrievalDocumentQ1A" {
		t.Fatalf("class_name=%v, want RetrievalDocumentQ1A", s["class_name"])
	}
	if s["schema_symbol"] != "_RETRIEVAL_DOCUMENT_SCHEMA_Q1A" {
		t.Fatalf("schema_symbol=%v, want _RETRIEVAL_DOCUMENT_SCHEMA_Q1A", s["schema_symbol"])
	}
	if s["target_file"] != "backend/retrieval_contracts.py" {
		t.Fatalf("target_file=%v, want backend/retrieval_contracts.py", s["target_file"])
	}
	if s["target_module"] != "backend.retrieval_contracts" {
		t.Fatalf("target_module=%v, want backend.retrieval_contracts", s["target_module"])
	}
	if s["direct_test"] != "backend/test_step21_5_retrieval_contracts.py" {
		t.Fatalf("direct_test=%v, want backend/test_step21_5_retrieval_contracts.py", s["direct_test"])
	}
	if s["mode"] != "seq215_retrieval_document_q1a_moved_definition" {
		t.Fatalf("mode=%v, want seq215_retrieval_document_q1a_moved_definition", s["mode"])
	}
}

func TestSeq215P596GenerationPacketMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p596","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_generation_packet_moved")
	if s["version"] != "s215-p596.v1" {
		t.Fatalf("version=%v, want s215-p596.v1", s["version"])
	}
	if s["class_name"] != "GenerationPacket" {
		t.Fatalf("class_name=%v, want GenerationPacket", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_generation_packet_moved_definition" {
		t.Fatalf("mode=%v, want seq215_generation_packet_moved_definition", s["mode"])
	}
}

func TestSeq215P597PrepareTurnResponseMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p597","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_response_moved")
	if s["version"] != "s215-p597.v1" {
		t.Fatalf("version=%v, want s215-p597.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnResponse" {
		t.Fatalf("class_name=%v, want PrepareTurnResponse", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_response_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_response_moved_definition", s["mode"])
	}
}

func TestSeq215P598MovedClassesImportedBack(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p598","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_moved_classes_imported_back")
	if s["version"] != "s215-p598.v1" {
		t.Fatalf("version=%v, want s215-p598.v1", s["version"])
	}
	if s["import_target"] != "main.py" {
		t.Fatalf("import_target=%v, want main.py", s["import_target"])
	}
	importSources := seq165Slice(t, s, "import_sources")
	for _, expected := range []string{"backend.turn_contracts", "backend.retrieval_contracts"} {
		if !sliceContains(importSources, expected) {
			t.Fatalf("import_sources missing %s: %v", expected, importSources)
		}
	}
	imported := seq165Slice(t, s, "imported_symbols")
	for _, expected := range []string{
		"CompleteTurnRequest",
		"M4CompleteTurnRequest",
		"M4CompleteTurnResponse",
		"PrepareTurnSettings",
		"PrepareTurnRequest",
		"GenerationPacket",
		"PrepareTurnResponse",
		"_RETRIEVAL_DOCUMENT_SCHEMA_Q1A",
		"RetrievalDocumentQ1A",
	} {
		if !sliceContains(imported, expected) {
			t.Fatalf("imported_symbols missing %s: %v", expected, imported)
		}
	}
	if s["mode"] != "seq215_moved_classes_imported_back_definition" {
		t.Fatalf("mode=%v, want seq215_moved_classes_imported_back_definition", s["mode"])
	}
}

func TestSeq215P599RouteDecoratorsStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p599","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_route_decorators_stay")
	if s["version"] != "s215-p599.v1" {
		t.Fatalf("version=%v, want s215-p599.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_route_decorators_stay_definition" {
		t.Fatalf("mode=%v, want seq215_route_decorators_stay_definition", s["mode"])
	}
}

func TestSeq215P600PrepareTurnStays(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p600","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_stays")
	if s["version"] != "s215-p600.v1" {
		t.Fatalf("version=%v, want s215-p600.v1", s["version"])
	}
	if s["function_name"] != "prepare_turn" {
		t.Fatalf("function_name=%v, want prepare_turn", s["function_name"])
	}
	if s["mode"] != "seq215_prepare_turn_stays_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_stays_definition", s["mode"])
	}
}

func TestSeq215P601CompleteTurnM4Stays(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p601","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_complete_turn_m4_stays")
	if s["version"] != "s215-p601.v1" {
		t.Fatalf("version=%v, want s215-p601.v1", s["version"])
	}
	if s["function_name"] != "complete_turn_m4" {
		t.Fatalf("function_name=%v, want complete_turn_m4", s["function_name"])
	}
	if s["mode"] != "seq215_complete_turn_m4_stays_definition" {
		t.Fatalf("mode=%v, want seq215_complete_turn_m4_stays_definition", s["mode"])
	}
}

func TestSeq215P602PublicRoutePathsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p602","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_public_route_paths_unchanged")
	if s["version"] != "s215-p602.v1" {
		t.Fatalf("version=%v, want s215-p602.v1", s["version"])
	}
	if s["paths_unchanged"] != true {
		t.Fatalf("paths_unchanged=%v, want true", s["paths_unchanged"])
	}
	paths := seq165Slice(t, s, "route_paths")
	for _, expected := range []string{"/turns/complete", "/complete-turn", "/prepare-turn"} {
		if !sliceContains(paths, expected) {
			t.Fatalf("route_paths missing %s: %v", expected, paths)
		}
	}
	if s["mode"] != "seq215_public_route_paths_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_public_route_paths_unchanged_definition", s["mode"])
	}
}

func TestSeq215P603ResponseFieldsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p603","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_response_fields_unchanged")
	if s["version"] != "s215-p603.v1" {
		t.Fatalf("version=%v, want s215-p603.v1", s["version"])
	}
	if s["fields_unchanged"] != true {
		t.Fatalf("fields_unchanged=%v, want true", s["fields_unchanged"])
	}
	if s["defaults_unchanged"] != true {
		t.Fatalf("defaults_unchanged=%v, want true", s["defaults_unchanged"])
	}
	models := seq165Slice(t, s, "response_models")
	for _, expected := range []string{"M4CompleteTurnResponse", "PrepareTurnResponse", "GenerationPacket"} {
		if !sliceContains(models, expected) {
			t.Fatalf("response_models missing %s: %v", expected, models)
		}
	}
	defaults, ok := s["default_checks"].(map[string]any)
	if !ok {
		t.Fatalf("default_checks missing or wrong type: %T", s["default_checks"])
	}
	for key, want := range map[string]string{
		"GenerationPacket.packet_mode":        "off",
		"PrepareTurnResponse.source":          "skeleton",
		"PrepareTurnResponse.fallback_reason": "skeleton_only",
	} {
		if defaults[key] != want {
			t.Fatalf("default_checks[%s]=%v, want %s", key, defaults[key], want)
		}
	}
	if s["mode"] != "seq215_response_fields_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_response_fields_unchanged_definition", s["mode"])
	}
}

func TestSeq215P604NoBroadTreeCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p604","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_broad_tree_created")
	if s["version"] != "s215-p604.v1" {
		t.Fatalf("version=%v, want s215-p604.v1", s["version"])
	}
	if s["broad_routes_created"] != false {
		t.Fatalf("broad_routes_created=%v, want false", s["broad_routes_created"])
	}
	if s["broad_services_created"] != false {
		t.Fatalf("broad_services_created=%v, want false", s["broad_services_created"])
	}
	if s["broad_schemas_created"] != false {
		t.Fatalf("broad_schemas_created=%v, want false", s["broad_schemas_created"])
	}
	if s["mode"] != "seq215_no_broad_tree_created_definition" {
		t.Fatalf("mode=%v, want seq215_no_broad_tree_created_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Turn-contracts validation tests (P605 ~ P607)
// ---------------------------------------------------------------------------

func TestSeq215P605PyCompileTurnContracts(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p605","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_py_compile_turn_contracts")
	if s["version"] != "s215-p605.v1" {
		t.Fatalf("version=%v, want s215-p605.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_turn_contracts_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_turn_contracts_definition", s["mode"])
	}
}

func TestSeq215P606FocusedImportCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p606","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_focused_import_check")
	if s["version"] != "s215-p606.v1" {
		t.Fatalf("version=%v, want s215-p606.v1", s["version"])
	}
	if s["check_status"] != "pass" {
		t.Fatalf("check_status=%v, want pass", s["check_status"])
	}
	if s["mode"] != "seq215_focused_import_check_definition" {
		t.Fatalf("mode=%v, want seq215_focused_import_check_definition", s["mode"])
	}
}

func TestSeq215P607ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p607","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_validation_record")
	if s["version"] != "s215-p607.v1" {
		t.Fatalf("version=%v, want s215-p607.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_validation_record_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 M-3a helper slice tests (P663 ~ P675)
// ---------------------------------------------------------------------------

func TestSeq215P663Phase1ValidationPassed(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p663","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase1_validation_passed")
	if s["version"] != "s215-p663.v1" {
		t.Fatalf("version=%v, want s215-p663.v1", s["version"])
	}
	if s["validation_status"] != "passed" {
		t.Fatalf("validation_status=%v, want passed", s["validation_status"])
	}
	if s["mode"] != "seq215_phase1_validation_passed_definition" {
		t.Fatalf("mode=%v, want seq215_phase1_validation_passed_definition", s["mode"])
	}
}

func TestSeq215P664PrepareTurnAssemblyCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p664","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_assembly_created")
	if s["version"] != "s215-p664.v1" {
		t.Fatalf("version=%v, want s215-p664.v1", s["version"])
	}
	if s["file_created"] != "backend/prepare_turn_assembly.py" {
		t.Fatalf("file_created=%v, want backend/prepare_turn_assembly.py", s["file_created"])
	}
	if s["mode"] != "seq215_prepare_turn_assembly_created_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_assembly_created_definition", s["mode"])
	}
}

func TestSeq215P665FormatMemoryTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p665","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_memory_text_moved")
	if s["version"] != "s215-p665.v1" {
		t.Fatalf("version=%v, want s215-p665.v1", s["version"])
	}
	if s["function_name"] != "_format_memory_text_m3a" {
		t.Fatalf("function_name=%v, want _format_memory_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_memory_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_memory_text_moved_definition", s["mode"])
	}
}

func TestSeq215P666FormatKGTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p666","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_kg_text_moved")
	if s["version"] != "s215-p666.v1" {
		t.Fatalf("version=%v, want s215-p666.v1", s["version"])
	}
	if s["function_name"] != "_format_kg_text_m3a" {
		t.Fatalf("function_name=%v, want _format_kg_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_kg_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_kg_text_moved_definition", s["mode"])
	}
}

func TestSeq215P667FormatEpisodeTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p667","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_episode_text_moved")
	if s["version"] != "s215-p667.v1" {
		t.Fatalf("version=%v, want s215-p667.v1", s["version"])
	}
	if s["function_name"] != "_format_episode_text_m3a" {
		t.Fatalf("function_name=%v, want _format_episode_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_episode_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_episode_text_moved_definition", s["mode"])
	}
}

func TestSeq215P668FormatChapterTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p668","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_chapter_text_moved")
	if s["version"] != "s215-p668.v1" {
		t.Fatalf("version=%v, want s215-p668.v1", s["version"])
	}
	if s["function_name"] != "_format_chapter_text_m3a" {
		t.Fatalf("function_name=%v, want _format_chapter_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_chapter_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_chapter_text_moved_definition", s["mode"])
	}
}

func TestSeq215P669FormatFallbackTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p669","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_fallback_text_moved")
	if s["version"] != "s215-p669.v1" {
		t.Fatalf("version=%v, want s215-p669.v1", s["version"])
	}
	if s["function_name"] != "_format_fallback_text_m3a" {
		t.Fatalf("function_name=%v, want _format_fallback_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_fallback_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_fallback_text_moved_definition", s["mode"])
	}
}

func TestSeq215P670CleanShortMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p670","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_clean_short_moved")
	if s["version"] != "s215-p670.v1" {
		t.Fatalf("version=%v, want s215-p670.v1", s["version"])
	}
	if s["function_name"] != "_clean_short_m3a" {
		t.Fatalf("function_name=%v, want _clean_short_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_clean_short_moved_definition" {
		t.Fatalf("mode=%v, want seq215_clean_short_moved_definition", s["mode"])
	}
}

func TestSeq215P671JsonLoadMaybeMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p671","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_json_load_maybe_moved")
	if s["version"] != "s215-p671.v1" {
		t.Fatalf("version=%v, want s215-p671.v1", s["version"])
	}
	if s["function_name"] != "_json_load_maybe_m3a" {
		t.Fatalf("function_name=%v, want _json_load_maybe_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_json_load_maybe_moved_definition" {
		t.Fatalf("mode=%v, want seq215_json_load_maybe_moved_definition", s["mode"])
	}
}

func TestSeq215P672PredicateMatchesMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p672","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_predicate_matches_moved")
	if s["version"] != "s215-p672.v1" {
		t.Fatalf("version=%v, want s215-p672.v1", s["version"])
	}
	if s["function_name"] != "_predicate_matches_m3a" {
		t.Fatalf("function_name=%v, want _predicate_matches_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_predicate_matches_moved_definition" {
		t.Fatalf("mode=%v, want seq215_predicate_matches_moved_definition", s["mode"])
	}
}

func TestSeq215P673WorldRuleNoteMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p673","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_world_rule_note_moved")
	if s["version"] != "s215-p673.v1" {
		t.Fatalf("version=%v, want s215-p673.v1", s["version"])
	}
	if s["function_name"] != "_world_rule_note_m3a" {
		t.Fatalf("function_name=%v, want _world_rule_note_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_world_rule_note_moved_definition" {
		t.Fatalf("mode=%v, want seq215_world_rule_note_moved_definition", s["mode"])
	}
}

func TestSeq215P674FormatEntityDigestTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p674","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_entity_digest_text_moved")
	if s["version"] != "s215-p674.v1" {
		t.Fatalf("version=%v, want s215-p674.v1", s["version"])
	}
	if s["function_name"] != "_format_entity_digest_text_m3a" {
		t.Fatalf("function_name=%v, want _format_entity_digest_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_entity_digest_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_entity_digest_text_moved_definition", s["mode"])
	}
}

func TestSeq215P675FormatEntityAnchorTextMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p675","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_format_entity_anchor_text_moved")
	if s["version"] != "s215-p675.v1" {
		t.Fatalf("version=%v, want s215-p675.v1", s["version"])
	}
	if s["function_name"] != "_format_entity_anchor_text_m3a" {
		t.Fatalf("function_name=%v, want _format_entity_anchor_text_m3a", s["function_name"])
	}
	if s["acyclic_check"] != true {
		t.Fatalf("acyclic_check=%v, want true", s["acyclic_check"])
	}
	if s["mode"] != "seq215_format_entity_anchor_text_moved_definition" {
		t.Fatalf("mode=%v, want seq215_format_entity_anchor_text_moved_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 M-3a validation and guard tests (P677 ~ P682)
// ---------------------------------------------------------------------------

func TestSeq215P677DBSessionHelpersStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p677","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_db_session_helpers_stay")
	if s["version"] != "s215-p677.v1" {
		t.Fatalf("version=%v, want s215-p677.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_db_session_helpers_stay_definition" {
		t.Fatalf("mode=%v, want seq215_db_session_helpers_stay_definition", s["mode"])
	}
}

func TestSeq215P678CoreLogicStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p678","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_core_logic_stay")
	if s["version"] != "s215-p678.v1" {
		t.Fatalf("version=%v, want s215-p678.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_core_logic_stay_definition" {
		t.Fatalf("mode=%v, want seq215_core_logic_stay_definition", s["mode"])
	}
}

func TestSeq215P679InjectionPackFieldsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p679","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_injection_pack_fields_unchanged")
	if s["version"] != "s215-p679.v1" {
		t.Fatalf("version=%v, want s215-p679.v1", s["version"])
	}
	if s["fields_unchanged"] != true {
		t.Fatalf("fields_unchanged=%v, want true", s["fields_unchanged"])
	}
	fields := seq165Slice(t, s, "field_names")
	for _, field := range []string{"memory_text", "kg_text", "fallback_text", "episode_text"} {
		if !sliceContains(fields, field) {
			t.Fatalf("field_names missing %q: %v", field, fields)
		}
	}
	if s["backend_pack_status"] != "advisory_not_authoritative" {
		t.Fatalf("backend_pack_status=%v, want advisory_not_authoritative", s["backend_pack_status"])
	}
	if s["mode"] != "seq215_injection_pack_fields_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_injection_pack_fields_unchanged_definition", s["mode"])
	}
}

func TestSeq215P680PyCompilePrepareTurnAssembly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p680","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_py_compile_prepare_turn_assembly")
	if s["version"] != "s215-p680.v1" {
		t.Fatalf("version=%v, want s215-p680.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_prepare_turn_assembly_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_prepare_turn_assembly_definition", s["mode"])
	}
}

func TestSeq215P681FocusedBackendTestsM3a(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p681","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_focused_backend_tests_m3a")
	if s["version"] != "s215-p681.v1" {
		t.Fatalf("version=%v, want s215-p681.v1", s["version"])
	}
	if s["focused_test_file"] != "backend/test_step21_5_prepare_turn_assembly.py" {
		t.Fatalf("focused_test_file=%v, want backend/test_step21_5_prepare_turn_assembly.py", s["focused_test_file"])
	}
	if s["tests_passed"] != float64(5) {
		t.Fatalf("tests_passed=%v, want 5", s["tests_passed"])
	}
	if s["scoped_regression_tests_passed"] != float64(25) {
		t.Fatalf("scoped_regression_tests_passed=%v, want 25", s["scoped_regression_tests_passed"])
	}
	if s["backend_tests_passed"] != float64(212) {
		t.Fatalf("backend_tests_passed=%v, want 212", s["backend_tests_passed"])
	}
	coverage := seq165Slice(t, s, "test_coverage")
	for _, marker := range []string{"exact_kg_delimiter_preservation", "main_py_import_bridge", "bundle_injection_assembly_consumes_extracted_formatters"} {
		if !sliceContains(coverage, marker) {
			t.Fatalf("test_coverage missing %q: %v", marker, coverage)
		}
	}
	if s["mode"] != "seq215_focused_backend_tests_m3a_definition" {
		t.Fatalf("mode=%v, want seq215_focused_backend_tests_m3a_definition", s["mode"])
	}
}

func TestSeq215P682M3aValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p682","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m3a_validation_record")
	if s["version"] != "s215-p682.v1" {
		t.Fatalf("version=%v, want s215-p682.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_m3a_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_m3a_validation_record_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Proxy/Config owner split tests (P730 ~ P740)
// ---------------------------------------------------------------------------

func TestSeq215P730ProxyPluginMainModelSeparated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p730","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_proxy_plugin_main_model_separated")
	if s["version"] != "s215-p730.v1" {
		t.Fatalf("version=%v, want s215-p730.v1", s["version"])
	}
	if s["route"] != "/proxy/plugin-main" {
		t.Fatalf("route=%v, want /proxy/plugin-main", s["route"])
	}
	if s["monolith_separated"] != true {
		t.Fatalf("monolith_separated=%v, want true", s["monolith_separated"])
	}
	if s["mode"] != "seq215_proxy_plugin_main_model_separated_definition" {
		t.Fatalf("mode=%v, want seq215_proxy_plugin_main_model_separated_definition", s["mode"])
	}
}

func TestSeq215P731ProviderOwnershipSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p731","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_provider_ownership_split")
	if s["version"] != "s215-p731.v1" {
		t.Fatalf("version=%v, want s215-p731.v1", s["version"])
	}
	providers := seq165Slice(t, s, "providers")
	for _, p := range []string{"openai", "claude", "gemini", "vertex", "copilot"} {
		if !sliceContains(providers, p) {
			t.Fatalf("providers missing %q: %v", p, providers)
		}
	}
	if s["mode"] != "seq215_provider_ownership_split_definition" {
		t.Fatalf("mode=%v, want seq215_provider_ownership_split_definition", s["mode"])
	}
}

func TestSeq215P732ThinProxyRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p732","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_thin_proxy_route")
	if s["version"] != "s215-p732.v1" {
		t.Fatalf("version=%v, want s215-p732.v1", s["version"])
	}
	if s["route"] != "/proxy/plugin-main" {
		t.Fatalf("route=%v, want /proxy/plugin-main", s["route"])
	}
	if s["delegate"] != "performProxyPluginMain" {
		t.Fatalf("delegate=%v, want performProxyPluginMain", s["delegate"])
	}
	if s["route_type"] != "thin_delegating" {
		t.Fatalf("route_type=%v, want thin_delegating", s["route_type"])
	}
	if s["mode"] != "seq215_thin_proxy_route_definition" {
		t.Fatalf("mode=%v, want seq215_thin_proxy_route_definition", s["mode"])
	}
}

func TestSeq215P733ConfigServiceSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p733","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_config_service_split")
	if s["version"] != "s215-p733.v1" {
		t.Fatalf("version=%v, want s215-p733.v1", s["version"])
	}
	features := seq165Slice(t, s, "implemented_features")
	for _, f := range []string{"key_mapping", "type_normalization", "runtime_config_trace", "secret_response_masking"} {
		if !sliceContains(features, f) {
			t.Fatalf("implemented_features missing %q: %v", f, features)
		}
	}
	deferred := seq165Slice(t, s, "deferred_features")
	for _, f := range []string{"env_file_persistence", "encrypted_api_key_persistence"} {
		if !sliceContains(deferred, f) {
			t.Fatalf("deferred_features missing %q: %v", f, deferred)
		}
	}
	if s["persistence_mode"] != "runtime_only" {
		t.Fatalf("persistence_mode=%v, want runtime_only", s["persistence_mode"])
	}
	if s["persisted"] != false {
		t.Fatalf("persisted=%v, want false", s["persisted"])
	}
	if s["env_file_persistence"] != "not_enabled_in_2_0" {
		t.Fatalf("env_file_persistence=%v, want not_enabled_in_2_0", s["env_file_persistence"])
	}
	if s["encrypted_api_key_persistence"] != "not_enabled_in_2_0" {
		t.Fatalf("encrypted_api_key_persistence=%v, want not_enabled_in_2_0", s["encrypted_api_key_persistence"])
	}
	if s["mode"] != "seq215_config_service_split_definition" {
		t.Fatalf("mode=%v, want seq215_config_service_split_definition", s["mode"])
	}
}

func TestSeq215P734ThinConfigRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p734","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_thin_config_route")
	if s["version"] != "s215-p734.v1" {
		t.Fatalf("version=%v, want s215-p734.v1", s["version"])
	}
	if s["route"] != "/config/update" {
		t.Fatalf("route=%v, want /config/update", s["route"])
	}
	if s["route_type"] != "thin_delegating" {
		t.Fatalf("route_type=%v, want thin_delegating", s["route_type"])
	}
	if s["mode"] != "seq215_thin_config_route_definition" {
		t.Fatalf("mode=%v, want seq215_thin_config_route_definition", s["mode"])
	}
}

func TestSeq215P735RoutesExplicitlyWired(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p735","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_routes_explicitly_wired")
	if s["version"] != "s215-p735.v1" {
		t.Fatalf("version=%v, want s215-p735.v1", s["version"])
	}
	routes := seq165Slice(t, s, "routes")
	for _, r := range []string{"/proxy/plugin-main", "/config/update"} {
		if !sliceContains(routes, r) {
			t.Fatalf("routes missing %q: %v", r, routes)
		}
	}
	if s["explicit_binding"] != true {
		t.Fatalf("explicit_binding=%v, want true", s["explicit_binding"])
	}
	if s["mode"] != "seq215_routes_explicitly_wired_definition" {
		t.Fatalf("mode=%v, want seq215_routes_explicitly_wired_definition", s["mode"])
	}
}

func TestSeq215P736PublicPathsPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p736","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_public_paths_preserved")
	if s["version"] != "s215-p736.v1" {
		t.Fatalf("version=%v, want s215-p736.v1", s["version"])
	}
	paths := seq165Slice(t, s, "paths")
	for _, p := range []string{"/proxy/plugin-main", "/config/update"} {
		if !sliceContains(paths, p) {
			t.Fatalf("paths missing %q: %v", p, paths)
		}
	}
	if s["unchanged"] != true {
		t.Fatalf("unchanged=%v, want true", s["unchanged"])
	}
	if s["mode"] != "seq215_public_paths_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_public_paths_preserved_definition", s["mode"])
	}
}

func TestSeq215P737CompatibilityWrapper(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p737","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_compatibility_wrapper")
	if s["version"] != "s215-p737.v1" {
		t.Fatalf("version=%v, want s215-p737.v1", s["version"])
	}
	if s["wrapper_present"] != true {
		t.Fatalf("wrapper_present=%v, want true", s["wrapper_present"])
	}
	if s["backward_compatible"] != true {
		t.Fatalf("backward_compatible=%v, want true", s["backward_compatible"])
	}
	if s["compatibility_mode"] != "stable_route_and_handler_compatibility" {
		t.Fatalf("compatibility_mode=%v, want stable_route_and_handler_compatibility", s["compatibility_mode"])
	}
	if s["python_wrapper_not_applicable"] != true {
		t.Fatalf("python_wrapper_not_applicable=%v, want true", s["python_wrapper_not_applicable"])
	}
	if s["mode"] != "seq215_compatibility_wrapper_definition" {
		t.Fatalf("mode=%v, want seq215_compatibility_wrapper_definition", s["mode"])
	}
}

func TestSeq215P738RouteLevelTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p738","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_route_level_tests")
	if s["version"] != "s215-p738.v1" {
		t.Fatalf("version=%v, want s215-p738.v1", s["version"])
	}
	routes := seq165Slice(t, s, "test_routes")
	for _, r := range []string{"/config/update", "/proxy/plugin-main"} {
		if !sliceContains(routes, r) {
			t.Fatalf("test_routes missing %q: %v", r, routes)
		}
	}
	assertions := seq165Slice(t, s, "direct_route_assertions")
	for _, a := range []string{"proxy_missing_endpoint_returns_400_without_upstream", "config_update_masks_secret_and_reports_runtime_only"} {
		if !sliceContains(assertions, a) {
			t.Fatalf("direct_route_assertions missing %q: %v", a, assertions)
		}
	}
	proxyReq := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", strings.NewReader(`{"model":"seq215-p738-no-endpoint"}`))
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyRec := httptest.NewRecorder()
	mux.ServeHTTP(proxyRec, proxyReq)
	if proxyRec.Code != http.StatusBadRequest {
		t.Fatalf("proxy missing endpoint status=%d, want 400: %s", proxyRec.Code, proxyRec.Body.String())
	}
	secret := "sk-seq215-p738-secret"
	configReq := httptest.NewRequest(http.MethodPost, "/config/update", strings.NewReader(`{
		"mainProvider":"openai",
		"mainApiKey":"`+secret+`",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"seq215-p738-model"
	}`))
	configReq.Header.Set("Content-Type", "application/json")
	configRec := httptest.NewRecorder()
	mux.ServeHTTP(configRec, configReq)
	if configRec.Code != http.StatusOK {
		t.Fatalf("config/update status=%d, want 200: %s", configRec.Code, configRec.Body.String())
	}
	if strings.Contains(configRec.Body.String(), secret) {
		t.Fatalf("config/update response leaked secret %q: %s", secret, configRec.Body.String())
	}
	var configResp map[string]any
	if err := json.Unmarshal(configRec.Body.Bytes(), &configResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	if configResp["persistence"] != "runtime_only" {
		t.Fatalf("config persistence=%v, want runtime_only", configResp["persistence"])
	}
	if configResp["persisted"] != false {
		t.Fatalf("config persisted=%v, want false", configResp["persisted"])
	}
	if s["mode"] != "seq215_route_level_tests_definition" {
		t.Fatalf("mode=%v, want seq215_route_level_tests_definition", s["mode"])
	}
}

func TestSeq215P739JSRouteUsage(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p739","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_route_usage")
	if s["version"] != "s215-p739.v1" {
		t.Fatalf("version=%v, want s215-p739.v1", s["version"])
	}
	if s["config_route"] != "/config/update" {
		t.Fatalf("config_route=%v, want /config/update", s["config_route"])
	}
	if s["proxy_route"] != "/proxy/plugin-main" {
		t.Fatalf("proxy_route=%v, want /proxy/plugin-main", s["proxy_route"])
	}
	if s["mode"] != "seq215_js_route_usage_definition" {
		t.Fatalf("mode=%v, want seq215_js_route_usage_definition", s["mode"])
	}
}

func TestSeq215P740MonolithNotApplicable(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p740","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_monolith_not_applicable")
	if s["version"] != "s215-p740.v1" {
		t.Fatalf("version=%v, want s215-p740.v1", s["version"])
	}
	if s["go_interpretation"] != "monolith_not_applicable" {
		t.Fatalf("go_interpretation=%v, want monolith_not_applicable", s["go_interpretation"])
	}
	if s["go_route_owner_split"] != true {
		t.Fatalf("go_route_owner_split=%v, want true", s["go_route_owner_split"])
	}
	if s["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", s["beta_reference_mutated"])
	}
	if s["mode"] != "seq215_monolith_not_applicable_definition" {
		t.Fatalf("mode=%v, want seq215_monolith_not_applicable_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 JS ownership boundary and function preservation tests (P776 ~ P789)
// ---------------------------------------------------------------------------

func TestSeq215P776PrepareTurnBundleNormalUse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p776","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_bundle_normal_use")
	if s["version"] != "s215-p776.v1" {
		t.Fatalf("version=%v, want s215-p776.v1", s["version"])
	}
	if s["bundle_fields"] != "normal_use_data_sources" {
		t.Fatalf("bundle_fields=%v, want normal_use_data_sources", s["bundle_fields"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"preparedBundle && preparedBundle.continuityPack",
		"preparedBundle && preparedBundle.recallResult",
		"preparedBundle && preparedBundle.injectionPack",
	)
	if s["mode"] != "seq215_prepare_turn_bundle_normal_use_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_bundle_normal_use_definition", s["mode"])
	}
}

func TestSeq215P777JSPayloadMutationOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p777","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_payload_mutation_owner")
	if s["version"] != "s215-p777.v1" {
		t.Fatalf("version=%v, want s215-p777.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "final_payload_mutation" {
		t.Fatalf("responsibility=%v, want final_payload_mutation", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"const { payload: injectedPayload, injectionResult } = applyContextInjection(outgoingPayload, lastOrchResult);",
		"payload: injectedPayload",
	)
	if s["mode"] != "seq215_js_payload_mutation_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_payload_mutation_owner_definition", s["mode"])
	}
}

func TestSeq215P778JSInjectionBudgetOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p778","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_injection_budget_owner")
	if s["version"] != "s215-p778.v1" {
		t.Fatalf("version=%v, want s215-p778.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "injection_budget_application" {
		t.Fatalf("responsibility=%v, want injection_budget_application", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"function assembleInjectionWithBudget(",
		"const budgetResult = assembleInjectionWithBudget(",
	)
	if s["mode"] != "seq215_js_injection_budget_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_injection_budget_owner_definition", s["mode"])
	}
}

func TestSeq215P779JSInputContextSlottingOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p779","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_input_context_slotting_owner")
	if s["version"] != "s215-p779.v1" {
		t.Fatalf("version=%v, want s215-p779.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "input_context_slotting" {
		t.Fatalf("responsibility=%v, want input_context_slotting", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"function buildInputContext(",
		"_ip.input_context_text",
		"inputCtx = buildInputContext(",
	)
	if s["mode"] != "seq215_js_input_context_slotting_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_input_context_slotting_owner_definition", s["mode"])
	}
}

func TestSeq215P780JSProtectionBlocksOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p780","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_protection_blocks_owner")
	if s["version"] != "s215-p780.v1" {
		t.Fatalf("version=%v, want s215-p780.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "protection_blocks" {
		t.Fatalf("responsibility=%v, want protection_blocks", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"protection: protection",
		"baseRulesIncluded",
		"reliabilityGuardIncluded",
	)
	if s["mode"] != "seq215_js_protection_blocks_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_protection_blocks_owner_definition", s["mode"])
	}
}

func TestSeq215P781JSHookUIIntegrationOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p781","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_hook_ui_integration_owner")
	if s["version"] != "s215-p781.v1" {
		t.Fatalf("version=%v, want s215-p781.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "hook_ui_integration" {
		t.Fatalf("responsibility=%v, want hook_ui_integration", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"addRisuReplacer(\"beforeRequest\", onBeforeRequest)",
		"addRisuReplacer(\"afterRequest\", onAfterRequest)",
		"renderSettingsPanel",
	)
	if s["mode"] != "seq215_js_hook_ui_integration_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_hook_ui_integration_owner_definition", s["mode"])
	}
}

func TestSeq215P782JSOfflineFailOpenOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p782","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_offline_fail_open_owner")
	if s["version"] != "s215-p782.v1" {
		t.Fatalf("version=%v, want s215-p782.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "offline_fail_open_fallback" {
		t.Fatalf("responsibility=%v, want offline_fail_open_fallback", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"return { source: \"backend-off\", fallback_reason: \"backend_off\", status: \"error\" }",
		"return { source: \"backend-error\", fallback_reason: \"backend_error\", status: \"error\" }",
		"backend off / timeout 시 fail-open으로 진행",
		"prepare-turn unavailable - local fallback continues:",
	)
	if strings.Contains(source, "return buildBlockedPayload(payload, backendBlock.userMessage || backendBlock.reason)") {
		t.Fatal("backend-off prepare-turn path must stay fail-open and must not return a blocked payload")
	}
	if s["mode"] != "seq215_js_offline_fail_open_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_offline_fail_open_owner_definition", s["mode"])
	}
}

func TestSeq215P783BuildInputContextPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p783","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_build_input_context_preserved")
	if s["version"] != "s215-p783.v1" {
		t.Fatalf("version=%v, want s215-p783.v1", s["version"])
	}
	if s["function_name"] != "buildInputContext" {
		t.Fatalf("function_name=%v, want buildInputContext", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function buildInputContext(")
	if s["mode"] != "seq215_build_input_context_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_build_input_context_preserved_definition", s["mode"])
	}
}

func TestSeq215P784AssembleInjectionWithBudgetPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p784","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_assemble_injection_with_budget_preserved")
	if s["version"] != "s215-p784.v1" {
		t.Fatalf("version=%v, want s215-p784.v1", s["version"])
	}
	if s["function_name"] != "assembleInjectionWithBudget" {
		t.Fatalf("function_name=%v, want assembleInjectionWithBudget", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function assembleInjectionWithBudget(")
	if s["mode"] != "seq215_assemble_injection_with_budget_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_assemble_injection_with_budget_preserved_definition", s["mode"])
	}
}

func TestSeq215P785ApplyContextInjectionPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p785","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_apply_context_injection_preserved")
	if s["version"] != "s215-p785.v1" {
		t.Fatalf("version=%v, want s215-p785.v1", s["version"])
	}
	if s["function_name"] != "applyContextInjection" {
		t.Fatalf("function_name=%v, want applyContextInjection", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function applyContextInjection(")
	if s["mode"] != "seq215_apply_context_injection_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_apply_context_injection_preserved_definition", s["mode"])
	}
}

func TestSeq215P786TryPrepareTurnTakeoverOff(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p786","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_try_prepare_turn_takeover_off")
	if s["version"] != "s215-p786.v1" {
		t.Fatalf("version=%v, want s215-p786.v1", s["version"])
	}
	if s["function_name"] != "tryPrepareTurn" {
		t.Fatalf("function_name=%v, want tryPrepareTurn", s["function_name"])
	}
	if s["takeover_mode"] != "off" {
		t.Fatalf("takeover_mode=%v, want off", s["takeover_mode"])
	}
	if s["fixed"] != true {
		t.Fatalf("fixed=%v, want true", s["fixed"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"async function tryPrepareTurn(",
		"takeover_mode: \"off\"",
	)
	if s["mode"] != "seq215_try_prepare_turn_takeover_off_definition" {
		t.Fatalf("mode=%v, want seq215_try_prepare_turn_takeover_off_definition", s["mode"])
	}
}

func TestSeq215P787JSNodeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p787","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_node_check")
	if s["version"] != "s215-p787.v1" {
		t.Fatalf("version=%v, want s215-p787.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_js_node_check_definition" {
		t.Fatalf("mode=%v, want seq215_js_node_check_definition", s["mode"])
	}
}

func TestSeq215P788JSFocusedContractTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p788","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_focused_contract_tests")
	if s["version"] != "s215-p788.v1" {
		t.Fatalf("version=%v, want s215-p788.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["test_suite"] != "js-route-variant-smoke" {
		t.Fatalf("test_suite=%v, want js-route-variant-smoke", s["test_suite"])
	}
	if s["mode"] != "seq215_js_focused_contract_tests_definition" {
		t.Fatalf("mode=%v, want seq215_js_focused_contract_tests_definition", s["mode"])
	}
}

func TestSeq215P789ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p789","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_p789_validation_record")
	if s["version"] != "s215-p789.v1" {
		t.Fatalf("version=%v, want s215-p789.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_p789_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_p789_validation_record_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21.5 Final closeout and validation tests (P830 ~ P883)
// ---------------------------------------------------------------------------

func TestSeq215P830RuntimeSplitStatus(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p830","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_runtime_split_status")
	if s["version"] != "s215-p830.v1" {
		t.Fatalf("version=%v, want s215-p830.v1", s["version"])
	}
	if s["runtime_split_status"] != "trace_contract_only" {
		t.Fatalf("runtime_split_status=%v, want trace_contract_only", s["runtime_split_status"])
	}
	if s["matches_reality"] != true {
		t.Fatalf("matches_reality=%v, want true", s["matches_reality"])
	}
	if s["mode"] != "seq215_runtime_split_status_definition" {
		t.Fatalf("mode=%v, want seq215_runtime_split_status_definition", s["mode"])
	}
}

func TestSeq215P831BackendBundleAssisted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p831","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backend_bundle_assisted")
	if s["version"] != "s215-p831.v1" {
		t.Fatalf("version=%v, want s215-p831.v1", s["version"])
	}
	if s["backend_exclusive"] != false {
		t.Fatalf("backend_exclusive=%v, want false", s["backend_exclusive"])
	}
	if s["mode"] != "seq215_backend_bundle_assisted_definition" {
		t.Fatalf("mode=%v, want seq215_backend_bundle_assisted_definition", s["mode"])
	}
}

func TestSeq215P832PluginOnlyModules(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p832","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_plugin_only_modules")
	if s["version"] != "s215-p832.v1" {
		t.Fatalf("version=%v, want s215-p832.v1", s["version"])
	}
	if s["ownership"] != "plugin_local_runtime" {
		t.Fatalf("ownership=%v, want plugin_local_runtime", s["ownership"])
	}
	if s["mode"] != "seq215_plugin_only_modules_definition" {
		t.Fatalf("mode=%v, want seq215_plugin_only_modules_definition", s["mode"])
	}
}

func TestSeq215P833OR1eWording(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p833","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_or1e_wording")
	if s["version"] != "s215-p833.v1" {
		t.Fatalf("version=%v, want s215-p833.v1", s["version"])
	}
	if s["claims_step_22"] != false {
		t.Fatalf("claims_step_22=%v, want false", s["claims_step_22"])
	}
	if s["claims_2_0_migration"] != false {
		t.Fatalf("claims_2_0_migration=%v, want false", s["claims_2_0_migration"])
	}
	if s["mode"] != "seq215_or1e_wording_definition" {
		t.Fatalf("mode=%v, want seq215_or1e_wording_definition", s["mode"])
	}
}

func TestSeq215P834OR1eNodeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p834","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_or1e_node_check")
	if s["version"] != "s215-p834.v1" {
		t.Fatalf("version=%v, want s215-p834.v1", s["version"])
	}
	if s["or1e_edited"] != false {
		t.Fatalf("or1e_edited=%v, want false", s["or1e_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_or1e_node_check_definition" {
		t.Fatalf("mode=%v, want seq215_or1e_node_check_definition", s["mode"])
	}
}

func TestSeq215P835ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p835","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_validation_record_p835")
	if s["version"] != "s215-p835.v1" {
		t.Fatalf("version=%v, want s215-p835.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_p835_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_p835_validation_record_definition", s["mode"])
	}
}

func TestSeq215P869Phase1Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p869","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase1_complete")
	if s["version"] != "s215-p869.v1" {
		t.Fatalf("version=%v, want s215-p869.v1", s["version"])
	}
	if s["phase"] != "1" {
		t.Fatalf("phase=%v, want 1", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase1_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase1_complete_definition", s["mode"])
	}
}

func TestSeq215P870Phase2Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p870","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase2_complete")
	if s["version"] != "s215-p870.v1" {
		t.Fatalf("version=%v, want s215-p870.v1", s["version"])
	}
	if s["phase"] != "2" {
		t.Fatalf("phase=%v, want 2", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase2_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase2_complete_definition", s["mode"])
	}
}

func TestSeq215P871Phase3Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p871","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase3_complete")
	if s["version"] != "s215-p871.v1" {
		t.Fatalf("version=%v, want s215-p871.v1", s["version"])
	}
	if s["phase"] != "3" {
		t.Fatalf("phase=%v, want 3", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase3_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase3_complete_definition", s["mode"])
	}
}

func TestSeq215P872Phase4Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p872","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase4_complete")
	if s["version"] != "s215-p872.v1" {
		t.Fatalf("version=%v, want s215-p872.v1", s["version"])
	}
	if s["phase"] != "4" {
		t.Fatalf("phase=%v, want 4", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase4_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase4_complete_definition", s["mode"])
	}
}

func TestSeq215P873ContextReadback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p873","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_context_readback")
	if s["version"] != "s215-p873.v1" {
		t.Fatalf("version=%v, want s215-p873.v1", s["version"])
	}
	if s["document"] != "CONTEXT 21.5th step.md" {
		t.Fatalf("document=%v, want CONTEXT 21.5th step.md", s["document"])
	}
	if s["readback_status"] != "ok" {
		t.Fatalf("readback_status=%v, want ok", s["readback_status"])
	}
	if s["mode"] != "seq215_context_readback_definition" {
		t.Fatalf("mode=%v, want seq215_context_readback_definition", s["mode"])
	}
}

func TestSeq215P874ProgressReadback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p874","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_progress_readback")
	if s["version"] != "s215-p874.v1" {
		t.Fatalf("version=%v, want s215-p874.v1", s["version"])
	}
	if s["document"] != "PROGRESS 21.5th step.md" {
		t.Fatalf("document=%v, want PROGRESS 21.5th step.md", s["document"])
	}
	if s["readback_status"] != "ok" {
		t.Fatalf("readback_status=%v, want ok", s["readback_status"])
	}
	if s["mode"] != "seq215_progress_readback_definition" {
		t.Fatalf("mode=%v, want seq215_progress_readback_definition", s["mode"])
	}
}

func TestSeq215P875StaleAuthoritySearch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p875","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_authority_search")
	if s["version"] != "s215-p875.v1" {
		t.Fatalf("version=%v, want s215-p875.v1", s["version"])
	}
	if s["search_result"] != "no_unresolved_stale_claims" {
		t.Fatalf("search_result=%v, want no_unresolved_stale_claims", s["search_result"])
	}
	if s["historical_mentions_reviewed"] != true {
		t.Fatalf("historical_mentions_reviewed=%v, want true", s["historical_mentions_reviewed"])
	}
	if s["current_authority_claim_clean"] != true {
		t.Fatalf("current_authority_claim_clean=%v, want true", s["current_authority_claim_clean"])
	}
	if s["mode"] != "seq215_stale_authority_search_definition" {
		t.Fatalf("mode=%v, want seq215_stale_authority_search_definition", s["mode"])
	}
}

func TestSeq215P876FalseBackendTreeSearch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p876","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_false_backend_tree_search")
	if s["version"] != "s215-p876.v1" {
		t.Fatalf("version=%v, want s215-p876.v1", s["version"])
	}
	if s["search_result"] != "no_unresolved_false_landed_claims" {
		t.Fatalf("search_result=%v, want no_unresolved_false_landed_claims", s["search_result"])
	}
	if s["historical_backend_tree_mentions_present"] != true {
		t.Fatalf("historical_backend_tree_mentions_present=%v, want true", s["historical_backend_tree_mentions_present"])
	}
	if s["historical_backend_tree_mentions_classified"] != true {
		t.Fatalf("historical_backend_tree_mentions_classified=%v, want true", s["historical_backend_tree_mentions_classified"])
	}
	if s["active_remigration_track_marks_historical_drift"] != true {
		t.Fatalf("active_remigration_track_marks_historical_drift=%v, want true", s["active_remigration_track_marks_historical_drift"])
	}
	if s["mode"] != "seq215_false_backend_tree_search_definition" {
		t.Fatalf("mode=%v, want seq215_false_backend_tree_search_definition", s["mode"])
	}
}

func TestSeq215P877NoBackupDeployEdited(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p877","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_backup_deploy_edited")
	if s["version"] != "s215-p877.v1" {
		t.Fatalf("version=%v, want s215-p877.v1", s["version"])
	}
	if s["backup_deploy_edited"] != false {
		t.Fatalf("backup_deploy_edited=%v, want false", s["backup_deploy_edited"])
	}
	if s["package_folder_edited"] != false {
		t.Fatalf("package_folder_edited=%v, want false", s["package_folder_edited"])
	}
	if s["mode"] != "seq215_no_backup_deploy_edited_definition" {
		t.Fatalf("mode=%v, want seq215_no_backup_deploy_edited_definition", s["mode"])
	}
}

func TestSeq215P878ChangedFilesList(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p878","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_changed_files_list")
	if s["version"] != "s215-p878.v1" {
		t.Fatalf("version=%v, want s215-p878.v1", s["version"])
	}
	files := seq165Slice(t, s, "changed_files")
	for _, f := range []string{"group_turn_seq215_surfaces.go", "group_turn_seq215_test.go", "group_turn.go"} {
		if !sliceContains(files, f) {
			t.Fatalf("changed_files missing %q: %v", f, files)
		}
	}
	if s["mode"] != "seq215_changed_files_list_definition" {
		t.Fatalf("mode=%v, want seq215_changed_files_list_definition", s["mode"])
	}
}

func TestSeq215P879ValidationCommands(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p879","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_validation_commands")
	if s["version"] != "s215-p879.v1" {
		t.Fatalf("version=%v, want s215-p879.v1", s["version"])
	}
	if s["mode"] != "seq215_validation_commands_definition" {
		t.Fatalf("mode=%v, want seq215_validation_commands_definition", s["mode"])
	}
}

func TestSeq215P880AdditionalOwnerSplitBounded(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p880","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_additional_owner_split_bounded")
	if s["version"] != "s215-p880.v1" {
		t.Fatalf("version=%v, want s215-p880.v1", s["version"])
	}
	if s["beta_0_8_modified"] != false {
		t.Fatalf("beta_0_8_modified=%v, want false", s["beta_0_8_modified"])
	}
	if s["mode"] != "seq215_additional_owner_split_bounded_definition" {
		t.Fatalf("mode=%v, want seq215_additional_owner_split_bounded_definition", s["mode"])
	}
}

func TestSeq215P881JSBackendOffloadPluginOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p881","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_backend_offload_plugin_only")
	if s["version"] != "s215-p881.v1" {
		t.Fatalf("version=%v, want s215-p881.v1", s["version"])
	}
	if s["claimed_offload"] != false {
		t.Fatalf("claimed_offload=%v, want false", s["claimed_offload"])
	}
	if s["actual_state"] != "plugin_only_deferral" {
		t.Fatalf("actual_state=%v, want plugin_only_deferral", s["actual_state"])
	}
	if s["mode"] != "seq215_js_backend_offload_plugin_only_definition" {
		t.Fatalf("mode=%v, want seq215_js_backend_offload_plugin_only_definition", s["mode"])
	}
}

func TestSeq215P882MasterChecklistOpenZero(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p882","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_master_checklist_open_zero")
	if s["version"] != "s215-p882.v1" {
		t.Fatalf("version=%v, want s215-p882.v1", s["version"])
	}
	if s["open_rows"] != float64(0) {
		t.Fatalf("open_rows=%v, want 0", s["open_rows"])
	}
	if s["mode"] != "seq215_master_checklist_open_zero_definition" {
		t.Fatalf("mode=%v, want seq215_master_checklist_open_zero_definition", s["mode"])
	}
}

func TestSeq215P883StepComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p883","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_step_complete_p883")
	if s["version"] != "s215-p883.v1" {
		t.Fatalf("version=%v, want s215-p883.v1", s["version"])
	}
	if s["step"] != "21.5" {
		t.Fatalf("step=%v, want 21.5", s["step"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["all_slices_have_evidence"] != true {
		t.Fatalf("all_slices_have_evidence=%v, want true", s["all_slices_have_evidence"])
	}
	if s["size_reduction_goal_met"] != true {
		t.Fatalf("size_reduction_goal_met=%v, want true", s["size_reduction_goal_met"])
	}
	if s["mode"] != "seq215_step_complete_definition" {
		t.Fatalf("mode=%v, want seq215_step_complete_definition", s["mode"])
	}
}
