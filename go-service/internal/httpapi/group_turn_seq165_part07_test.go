package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq17P422ChromaBackfillDryRun validates the 17-C3 backfill dry-run
// surface for SEQ-17-P422.
func TestSeq17P422ChromaBackfillDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p422","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_backfill_dry_run")
	if s["version"] != "seq17_p422.v1" {
		t.Fatalf("version=%v, want seq17_p422.v1", s["version"])
	}
	if s["role"] != "chroma_backfill_dry_run" {
		t.Fatalf("role=%v, want chroma_backfill_dry_run", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["sample_export_executed"] != false {
		t.Fatalf("sample_export_executed=%v, want false", s["sample_export_executed"])
	}
	if s["chroma_ingest_executed"] != false {
		t.Fatalf("chroma_ingest_executed=%v, want false", s["chroma_ingest_executed"])
	}
	if s["mode"] != "chroma_backfill_dry_run" {
		t.Fatalf("mode=%v, want chroma_backfill_dry_run", s["mode"])
	}
}

// TestSeq17P423ChromaBulkBackfill validates the 17-C4 bulk backfill dry-run
// surface for SEQ-17-P423.
func TestSeq17P423ChromaBulkBackfill(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p423","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_bulk_backfill")
	if s["version"] != "seq17_p423.v1" {
		t.Fatalf("version=%v, want seq17_p423.v1", s["version"])
	}
	if s["role"] != "chroma_bulk_backfill" {
		t.Fatalf("role=%v, want chroma_bulk_backfill", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["bulk_ingest_executed"] != false {
		t.Fatalf("bulk_ingest_executed=%v, want false", s["bulk_ingest_executed"])
	}
	if s["checkpoint_written"] != false {
		t.Fatalf("checkpoint_written=%v, want false", s["checkpoint_written"])
	}
	if s["mode"] != "chroma_bulk_backfill_dry_run" {
		t.Fatalf("mode=%v, want chroma_bulk_backfill_dry_run", s["mode"])
	}
}

// TestSeq17P424ChromaReembedDiscipline validates the 17-C5 reembed discipline
// dry-run surface for SEQ-17-P424.
func TestSeq17P424ChromaReembedDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p424","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_reembed_discipline")
	if s["version"] != "seq17_p424.v1" {
		t.Fatalf("version=%v, want seq17_p424.v1", s["version"])
	}
	if s["role"] != "chroma_reembed_discipline" {
		t.Fatalf("role=%v, want chroma_reembed_discipline", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["reembed_queue_mutated"] != false {
		t.Fatalf("reembed_queue_mutated=%v, want false", s["reembed_queue_mutated"])
	}
	if s["vectors_invalidated"] != false {
		t.Fatalf("vectors_invalidated=%v, want false", s["vectors_invalidated"])
	}
	if s["mode"] != "chroma_reembed_discipline_dry_run" {
		t.Fatalf("mode=%v, want chroma_reembed_discipline_dry_run", s["mode"])
	}
}

// TestSeq17P425ChromaDivergenceHealthProbe validates the 17-C6 divergence / health
// probe dry-run surface for SEQ-17-P425.
func TestSeq17P425ChromaDivergenceHealthProbe(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p425","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_divergence_health_probe")
	if s["version"] != "seq17_p425.v1" {
		t.Fatalf("version=%v, want seq17_p425.v1", s["version"])
	}
	if s["role"] != "chroma_divergence_health_probe" {
		t.Fatalf("role=%v, want chroma_divergence_health_probe", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["health_probe_run"] != false {
		t.Fatalf("health_probe_run=%v, want false", s["health_probe_run"])
	}
	if s["fallback_triggered"] != false {
		t.Fatalf("fallback_triggered=%v, want false", s["fallback_triggered"])
	}
	if s["cache_invalidation_rule"] != "stateless_per_request" {
		t.Fatalf("cache_invalidation_rule=%v, want stateless_per_request", s["cache_invalidation_rule"])
	}
	if s["mode"] != "chroma_divergence_health_probe_dry_run" {
		t.Fatalf("mode=%v, want chroma_divergence_health_probe_dry_run", s["mode"])
	}
}

// TestSeq17P426ChromaDegradedFallbackRunbook validates the 17-C7 degraded
// fallback runbook dry-run surface for SEQ-17-P426.
func TestSeq17P426ChromaDegradedFallbackRunbook(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p426","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_degraded_fallback_runbook")
	if s["version"] != "seq17_p426.v1" {
		t.Fatalf("version=%v, want seq17_p426.v1", s["version"])
	}
	if s["role"] != "chroma_degraded_fallback_runbook" {
		t.Fatalf("role=%v, want chroma_degraded_fallback_runbook", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["write_freeze_applied"] != false {
		t.Fatalf("write_freeze_applied=%v, want false", s["write_freeze_applied"])
	}
	if s["cleanup_executed"] != false {
		t.Fatalf("cleanup_executed=%v, want false", s["cleanup_executed"])
	}
	if s["mode"] != "chroma_degraded_fallback_runbook_dry_run" {
		t.Fatalf("mode=%v, want chroma_degraded_fallback_runbook_dry_run", s["mode"])
	}
}

// TestSeq17P427ChromaRebuildRollbackDrill validates the 17-C8 rebuild / rollback
// drill dry-run surface for SEQ-17-P427.
func TestSeq17P427ChromaRebuildRollbackDrill(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p427","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_rebuild_rollback_drill")
	if s["version"] != "seq17_p427.v1" {
		t.Fatalf("version=%v, want seq17_p427.v1", s["version"])
	}
	if s["role"] != "chroma_rebuild_rollback_drill" {
		t.Fatalf("role=%v, want chroma_rebuild_rollback_drill", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["collection_wiped"] != false {
		t.Fatalf("collection_wiped=%v, want false", s["collection_wiped"])
	}
	if s["rebuild_executed"] != false {
		t.Fatalf("rebuild_executed=%v, want false", s["rebuild_executed"])
	}
	if s["rollback_executed"] != false {
		t.Fatalf("rollback_executed=%v, want false", s["rollback_executed"])
	}
	if s["mode"] != "chroma_rebuild_rollback_drill_dry_run" {
		t.Fatalf("mode=%v, want chroma_rebuild_rollback_drill_dry_run", s["mode"])
	}
}

// TestSeq17P428ChromaAdoptionGate validates the 17-C9 adoption gate dry-run
// surface for SEQ-17-P428.
func TestSeq17P428ChromaAdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p428","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_adoption_gate")
	if s["version"] != "seq17_p428.v1" {
		t.Fatalf("version=%v, want seq17_p428.v1", s["version"])
	}
	if s["role"] != "chroma_adoption_gate" {
		t.Fatalf("role=%v, want chroma_adoption_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["limited_cutover_enabled"] != false {
		t.Fatalf("limited_cutover_enabled=%v, want false", s["limited_cutover_enabled"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_adoption_gate_dry_run" {
		t.Fatalf("mode=%v, want chroma_adoption_gate_dry_run", s["mode"])
	}
}

// TestSeq17P429ChromaReleaseHygiene validates the 17-C10 release hygiene dry-run
// surface for SEQ-17-P429.
func TestSeq17P429ChromaReleaseHygiene(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p429","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_release_hygiene")
	if s["version"] != "seq17_p429.v1" {
		t.Fatalf("version=%v, want seq17_p429.v1", s["version"])
	}
	if s["role"] != "chroma_release_hygiene" {
		t.Fatalf("role=%v, want chroma_release_hygiene", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["bundle_regenerated"] != false {
		t.Fatalf("bundle_regenerated=%v, want false", s["bundle_regenerated"])
	}
	if s["release_ready"] != false {
		t.Fatalf("release_ready=%v, want false", s["release_ready"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_release_hygiene_dry_run" {
		t.Fatalf("mode=%v, want chroma_release_hygiene_dry_run", s["mode"])
	}
}

// TestSeq17P430ChromaMigrationVisibilityGuard validates the 17-C11 migration
// visibility guard dry-run surface for SEQ-17-P430.
func TestSeq17P430ChromaMigrationVisibilityGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p430","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_migration_visibility_guard")
	if s["version"] != "seq17_p430.v1" {
		t.Fatalf("version=%v, want seq17_p430.v1", s["version"])
	}
	if s["role"] != "chroma_migration_visibility_guard" {
		t.Fatalf("role=%v, want chroma_migration_visibility_guard", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["dashboard_mutation"] != false {
		t.Fatalf("dashboard_mutation=%v, want false", s["dashboard_mutation"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_migration_visibility_guard_dry_run" {
		t.Fatalf("mode=%v, want chroma_migration_visibility_guard_dry_run", s["mode"])
	}
}
