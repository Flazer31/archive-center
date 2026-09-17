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
