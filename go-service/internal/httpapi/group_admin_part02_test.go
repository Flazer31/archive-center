package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestSeq13P67P70P74P78P81SessionPortabilityDryRunGate(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"source_session_id":"source","target_session_id":"target","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["gate_status"] != "ready" || resp["apply_status"] != "dry_run_only" {
		t.Fatalf("unexpected migration dry-run state: %+v", resp)
	}
	if resp["package_policy_version"] != "sp1a.v1" || resp["ingest_gate_policy_version"] != "sp1b.v1" || resp["merge_policy_version"] != "sp1c.v1" {
		t.Fatalf("missing SP policy versions: %+v", resp)
	}
	if resp["manual_first"] != true || resp["operation_policy_version"] != "sp1e.v1" || resp["auto_copy_detection"] != "deferred" {
		t.Fatalf("manual-first portability contract missing: %+v", resp)
	}

	dedupe := resp["dedupe_report"].(map[string]any)
	if dedupe["direct_evidence_duplicate_source_hash"] != float64(1) || dedupe["canonical_layer_collisions"] != float64(1) {
		t.Fatalf("dedupe report mismatch: %+v", dedupe)
	}
	merge := resp["merge_report"].(map[string]any)
	if merge["tombstone_propagations"] != float64(1) || merge["supersede_propagations"] != float64(1) || merge["unresolved_superseded_blocks"] != float64(0) {
		t.Fatalf("merge report mismatch: %+v", merge)
	}
	handoff := resp["rebuild_handoff"].(map[string]any)
	if handoff["policy_version"] != "sp1d.v1" || handoff["dirty_event_type"] != "backfill_import" || handoff["rebuild_mode"] != "selective" || handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("selective rebuild handoff mismatch: %+v", handoff)
	}
	fields := resp["lineage_preserve_fields"].([]any)
	for _, needle := range []string{"source_hash", "source_turn", "session_origin", "tombstoned", "superseded_by_id"} {
		if !anySliceContains(fields, needle) {
			t.Fatalf("lineage preserve fields missing %q: %+v", needle, fields)
		}
	}
	if len(fake.savedEvidence) != 0 || len(fake.patchedEvidence) != 0 {
		t.Fatalf("dry-run should not write: saved=%d patched=%d", len(fake.savedEvidence), len(fake.patchedEvidence))
	}
}

func TestSeq13P70P74SessionPortabilityApplyRequiresReadyGate(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	blockedBody := `{"source_session_id":"source","target_session_id":"target","dry_run":false,"gate_status":"review"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(blockedBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("blocked status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var blocked map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &blocked); err != nil {
		t.Fatalf("decode blocked: %v", err)
	}
	if blocked["status"] != "blocked" || blocked["apply_status"] != "gate_not_ready" {
		t.Fatalf("apply should be blocked without ready gate: %+v", blocked)
	}
	if len(fake.savedEvidence) != 0 || len(fake.patchedEvidence) != 0 {
		t.Fatalf("blocked apply should not write: saved=%d patched=%d", len(fake.savedEvidence), len(fake.patchedEvidence))
	}

	readyBody := `{"source_session_id":"source","target_session_id":"target","dry_run":false,"gate_status":"ready"}`
	req = httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(readyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var ready map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ready); err != nil {
		t.Fatalf("decode ready: %v", err)
	}
	if ready["status"] != "ok" || ready["apply_status"] != "applied" || ready["moved_rows"] != float64(1) || ready["merged_rows"] != float64(1) || ready["source_rows_remaining"] != float64(0) {
		t.Fatalf("ready apply summary mismatch: %+v", ready)
	}
	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].ChatSessionID != "target" || fake.savedEvidence[0].SourceHash != "sha256:new" {
		t.Fatalf("new source evidence was not copied into target: %+v", fake.savedEvidence)
	}
	var lineage map[string]any
	if err := json.Unmarshal([]byte(fake.savedEvidence[0].LineageJSON), &lineage); err != nil {
		t.Fatalf("decode saved lineage: %v", err)
	}
	if lineage["session_origin"] != "source" || lineage["import_policy_version"] != "sp1b.v1" {
		t.Fatalf("saved lineage did not preserve import origin: %+v", lineage)
	}
	if len(fake.patchedEvidence) != 1 || fake.patchedEvidence[0].Tombstoned == nil || *fake.patchedEvidence[0].Tombstoned != true {
		t.Fatalf("duplicate tombstone/supersede was not propagated to target: %+v", fake.patchedEvidence)
	}
}

// TestSeq13P223CopyAwareImportContractMarkers verifies P223:
// Session-migrate endpoint exposes copy-aware import contract with deferred
// auto-copy detection, manual-first policy, and lineage preserve fields.
func TestSeq13P223CopyAwareImportContractMarkers(t *testing.T) {
	fake := seedSessionMigrationFakeStore()
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"source_session_id":"source","target_session_id":"target","dry_run":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/session-migrate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["manual_first"] != true {
		t.Fatalf("manual_first = %v, want true", resp["manual_first"])
	}
	if resp["auto_copy_detection"] != "deferred" {
		t.Fatalf("auto_copy_detection = %v, want deferred", resp["auto_copy_detection"])
	}
	policies, ok := resp["policy_versions"].([]any)
	if !ok || len(policies) == 0 {
		t.Fatalf("policy_versions missing or empty")
	}
	for _, want := range []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"} {
		found := false
		for _, p := range policies {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("policy_versions missing %q: %v", want, policies)
		}
	}
	if resp["package_policy_version"] != "sp1a.v1" {
		t.Fatalf("package_policy_version = %v, want sp1a.v1", resp["package_policy_version"])
	}
	if resp["ingest_gate_policy_version"] != "sp1b.v1" {
		t.Fatalf("ingest_gate_policy_version = %v, want sp1b.v1", resp["ingest_gate_policy_version"])
	}
	if resp["merge_policy_version"] != "sp1c.v1" {
		t.Fatalf("merge_policy_version = %v, want sp1c.v1", resp["merge_policy_version"])
	}
	fields, ok := resp["lineage_preserve_fields"].([]any)
	if !ok || len(fields) == 0 {
		t.Fatalf("lineage_preserve_fields missing or empty")
	}
	for _, needle := range []string{"source_hash", "source_turn", "session_origin", "tombstoned", "superseded_by_id"} {
		if !anySliceContains(fields, needle) {
			t.Fatalf("lineage_preserve_fields missing %q: %+v", needle, fields)
		}
	}
	if _, ok := resp["dedupe_report"]; !ok {
		t.Fatalf("dedupe_report missing")
	}
	if _, ok := resp["merge_report"]; !ok {
		t.Fatalf("merge_report missing")
	}
	handoff, ok := resp["rebuild_handoff"].(map[string]any)
	if !ok {
		t.Fatalf("rebuild_handoff missing")
	}
	if handoff["policy_version"] != "sp1d.v1" {
		t.Fatalf("rebuild_handoff.policy_version = %v, want sp1d.v1", handoff["policy_version"])
	}
	if handoff["dirty_event_type"] != "backfill_import" {
		t.Fatalf("rebuild_handoff.dirty_event_type = %v, want backfill_import", handoff["dirty_event_type"])
	}
	if handoff["rebuild_mode"] != "selective" {
		t.Fatalf("rebuild_handoff.rebuild_mode = %v, want selective", handoff["rebuild_mode"])
	}
	if handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("rebuild_handoff.start_point = %v, want next_prepare_turn_fetch", handoff["start_point"])
	}
}

func TestMaintenanceQueueStatusErrNotEnabledIsSafeEmpty(t *testing.T) {
	fake := &adminQueueStore{
		narrativeFakeStore: &narrativeFakeStore{},
		err:                store.ErrNotEnabled,
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/maintenance/queue-status?limit=999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["queue_depth"] != float64(0) || resp["audit_count"] != float64(0) {
		t.Fatalf("expected safe empty counts, got queue_depth=%v audit_count=%v", resp["queue_depth"], resp["audit_count"])
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["store_backed"] != false {
		t.Fatalf("trace_summary.store_backed = %#v, want false", trace)
	}
}

func TestMaintenanceEnqueueShadowWritesAuditAndTrace(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-maint","turn_index":12,"shadow_only":true,"assistant_response":"The scene continues.","recent_responses":["a","b"],"supervisor_result":{"directive":{"director":{"scene_mandate":"hold"}}}}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "ok" || resp["queue_depth"] != float64(1) || resp["maintenance_pass_enabled"] != false {
		t.Fatalf("unexpected enqueue response: %+v", resp)
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["owner"] != "maintenance_shadow" || trace["non_blocking"] != true || trace["worker_enabled"] != false {
		t.Fatalf("trace_summary mismatch: %+v", trace)
	}
	refresh, ok := resp["refresh_output"].(map[string]any)
	if !ok || refresh["story_plan_refresh"] != "shadow_candidate" || refresh["director_refresh"] != "shadow_candidate" || refresh["writeback_enabled"] != false {
		t.Fatalf("refresh_output mismatch: %+v", refresh)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "maintenance_enqueued" || fake.auditLogs[0].ChatSessionID != "sess-maint" || fake.auditLogs[0].TargetID != 12 {
		t.Fatalf("expected maintenance audit, got %#v", fake.auditLogs)
	}
}

func TestMaintenanceEnqueueMalformedPayloadIsNonBlocking(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(`{"chat_session_id":`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "skipped_malformed_payload" || resp["maintenance_pass_enabled"] != false {
		t.Fatalf("malformed payload should fail open: %+v", resp)
	}
	trace, _ := resp["trace_summary"].(map[string]any)
	if trace["non_blocking"] != true || trace["fallback"] != "malformed_payload" {
		t.Fatalf("malformed trace mismatch: %+v", trace)
	}
}

func TestMaintenancePassPathUsesSessionAndShadowOutput(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/maintenance-pass/sess-path", strings.NewReader(`{"turn_index":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["chat_session_id"] != "sess-path" || resp["action"] != "maintenance_pass" || resp["queue_depth"] != float64(1) {
		t.Fatalf("maintenance-pass response mismatch: %+v", resp)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].ChatSessionID != "sess-path" {
		t.Fatalf("expected path session audit, got %#v", fake.auditLogs)
	}
}

func TestMaintenanceEnqueueDriftSignalsAndCorrectionHints(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-drift",
	  "turn_index":13,
	  "assistant_response":"They abandon the market mystery and repeat stale refrain repeat stale refrain and erase the rooftop confession.",
	  "recent_responses":["repeat stale refrain opened the previous answer too"],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"rooftop confession"},
	      "director":{"forbidden_moves":["Do not erase the rooftop confession"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	signals, ok := resp["drift_signals"].([]any)
	if !ok || len(signals) < 2 {
		t.Fatalf("drift_signals = %+v, want multiple signals", resp["drift_signals"])
	}
	joinedSignals := strings.ToLower(strings.Join(anySliceToStringSlice(signals), "\n"))
	if !strings.Contains(joinedSignals, "forbidden_move_conflict") || !strings.Contains(joinedSignals, "pattern_repeat") {
		t.Fatalf("missing forbidden/pattern signals: %s", joinedSignals)
	}
	hints, ok := resp["correction_hints"].([]any)
	if !ok || len(hints) == 0 {
		t.Fatalf("correction_hints missing: %+v", resp["correction_hints"])
	}
	joinedHints := strings.ToLower(strings.Join(anySliceToStringSlice(hints), "\n"))
	if !strings.Contains(joinedHints, "may_override_current_user_input:false") && !strings.Contains(joinedHints, "map[") {
		t.Fatalf("correction hints should be subordinate maps: %s", joinedHints)
	}
	trace, _ := resp["trace_summary"].(map[string]any)
	traceSignals, _ := trace["drift_signals"].([]any)
	if len(traceSignals) != len(signals) {
		t.Fatalf("trace drift signals mismatch: trace=%+v resp=%+v", traceSignals, signals)
	}
}

func TestMaintenanceTM1bProvenanceConfidenceDriftAuditSurface(t *testing.T) {
	fake := &narrativeFakeStore{}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-tm1b",
	  "turn_index":42,
	  "assistant_response":"They erase the archive promise and walk away from the archive promise.",
	  "recent_responses":["archive promise archive promise"],
	  "canonical_state_layers":[
	    {"layer_type":"relationship_state","last_verified_turn":39,"confidence":0.82},
	    {"layer_type":"scene_state","last_verified_turn":40,"confidence":0.33}
	  ],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"archive promise"},
	      "director":{"forbidden_moves":["erase the archive promise"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["maintenance_pass_enabled"] != false || resp["worker_enabled"] != false {
		t.Fatalf("maintenance should remain shadow-only: %+v", resp)
	}
	if resp["last_verified_turn"] != float64(42) {
		t.Fatalf("last_verified_turn = %v, want 42", resp["last_verified_turn"])
	}
	signals, ok := resp["drift_signals"].([]any)
	if !ok || len(signals) == 0 {
		t.Fatalf("drift_signals missing or empty: %T", resp["drift_signals"])
	}
	for _, s := range signals {
		sig, ok := s.(map[string]any)
		if !ok {
			t.Fatalf("drift signal wrong type: %T", s)
		}
		if sig["drift_type"] == "" {
			t.Fatalf("drift signal missing drift_type: %+v", sig)
		}
		if sig["canonical_name"] == "" {
			t.Fatalf("drift signal missing canonical_name: %+v", sig)
		}
		if sig["scene"] == "" {
			t.Fatalf("drift signal missing scene: %+v", sig)
		}
	}
	state, ok := resp["maintenance_pass_state"].(map[string]any)
	if !ok {
		t.Fatalf("maintenance_pass_state missing: %+v", resp)
	}
	if state["surface"] != "MaintenancePassState" || state["status"] != "shadow_only" || state["would_write"] != false {
		t.Fatalf("maintenance pass state authority mismatch: %+v", state)
	}
	if state["drift_detected"] != true || state["confidence_floor"] != float64(0.3) {
		t.Fatalf("drift/floor mismatch: %+v", state)
	}
	if !strings.Contains(state["drift_signals_json"].(string), "forbidden_move_conflict") {
		t.Fatalf("drift_signals_json missing forbidden conflict: %v", state["drift_signals_json"])
	}
	updates, ok := state["canonical_updates"].([]any)
	if !ok || len(updates) != 2 {
		t.Fatalf("canonical_updates missing: %+v", state["canonical_updates"])
	}
	first, ok := updates[0].(map[string]any)
	if !ok {
		t.Fatalf("first canonical update is not an object: %+v", updates[0])
	}
	if first["last_verified_turn"] != float64(39) || first["confidence"] != float64(0.82) || first["next_confidence"] != float64(0.67) {
		t.Fatalf("relationship provenance/degradation mismatch: %+v", first)
	}
	second, ok := updates[1].(map[string]any)
	if !ok {
		t.Fatalf("second canonical update is not an object: %+v", updates[1])
	}
	if second["confidence"] != float64(0.33) || second["next_confidence"] != float64(0.3) {
		t.Fatalf("confidence floor mismatch: %+v", second)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "drift_detected" {
		t.Fatalf("expected drift_detected audit, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["confidence_floor"] != float64(0.3) || details["maintenance_pass_state"] == nil {
		t.Fatalf("audit details missing TM-1b state: %+v", details)
	}
}

func TestMaintenanceTM1cMemoryImportanceFreshnessReweightingSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ChatSessionID: "sess-tm1c", TurnIndex: 10, Content: "The blue oath is mentioned again."},
			{ChatSessionID: "sess-tm1c", TurnIndex: 12, Content: "Blue oath promise returns in the current beat."},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1c", TurnIndex: 11, SummaryJSON: `{"turn_summary":"Blue oath promise remains active"}`, Importance: 0.5},
			{ID: 2, ChatSessionID: "sess-tm1c", TurnIndex: 2, SummaryJSON: `{"turn_summary":"Solved gate mystery clue"}`, Importance: 0.7, EmotionalIntensity: 0.8},
			{ID: 3, ChatSessionID: "sess-tm1c", TurnIndex: 1, SummaryJSON: `{"turn_summary":"Pinned vow callback"}`, Importance: 0.4, EmotionalIntensity: 0.9},
			{ID: 4, ChatSessionID: "sess-tm1c", TurnIndex: 1, SummaryJSON: `{"turn_summary":"User corrected anchor sacred key"}`, Importance: 0.3, EmotionalIntensity: 0.9},
		},
		storylines: []store.Storyline{
			{Name: "Gate mystery", Status: "resolved", CurrentContext: "Solved gate mystery clue"},
			{Name: "Pinned vow", Status: "active", CurrentContext: "Pinned vow callback", Pinned: true},
		},
		pendingThreads: []store.PendingThread{
			{ThreadKey: "gate-thread", Status: "resolved", Description: "Solved gate mystery clue"},
			{ThreadKey: "sacred-key", Status: "open", Description: "User corrected anchor sacred key", UserCorrected: true},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-tm1c",
	  "turn_index":12,
	  "assistant_response":"The blue oath is carried forward without contradiction."
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	state, ok := resp["memory_importance_reweighting"].(map[string]any)
	if !ok {
		t.Fatalf("memory_importance_reweighting missing: %+v", resp)
	}
	if state["policy_version"] != "tm1c.v1" || state["status"] != "shadow_only" || state["would_write"] != false {
		t.Fatalf("TM-1c authority mismatch: %+v", state)
	}
	if state["source"] != "store" || state["recent_source"] != "store" || state["importance_scale"] != "0..1" {
		t.Fatalf("TM-1c source/scale mismatch: %+v", state)
	}
	if state["updated_count"] != float64(2) || state["boosted_count"] != float64(1) || state["decayed_count"] != float64(1) || state["protected_count"] != float64(2) {
		t.Fatalf("TM-1c counters mismatch: %+v", state)
	}
	updates, ok := state["updates"].([]any)
	if !ok || len(updates) != 2 {
		t.Fatalf("TM-1c updates missing: %+v", state["updates"])
	}
	byID := map[float64]map[string]any{}
	for _, raw := range updates {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("TM-1c update wrong type: %+v", raw)
		}
		byID[item["memory_id"].(float64)] = item
	}
	boosted := byID[1]
	if boosted["next_importance"] != float64(0.56) || boosted["recent_rementioned"] != true {
		t.Fatalf("recent remention boost mismatch: %+v", boosted)
	}
	boostReasons := strings.Join(anySliceToStringSlice(boosted["reasons"].([]any)), ",")
	if !strings.Contains(boostReasons, "recent_remention_boost") {
		t.Fatalf("boost reasons missing recent_remention_boost: %s", boostReasons)
	}
	decayed := byID[2]
	if decayed["next_importance"] != float64(0.53) || decayed["age_gap"] != float64(10) {
		t.Fatalf("resolved/emotional decay mismatch: %+v", decayed)
	}
	decayReasons := strings.Join(anySliceToStringSlice(decayed["reasons"].([]any)), ",")
	for _, needle := range []string{"freshness_decay", "resolved_reference_decay", "emotional_decay"} {
		if !strings.Contains(decayReasons, needle) {
			t.Fatalf("decay reasons missing %s: %s", needle, decayReasons)
		}
	}
	if _, protectedPinnedUpdated := byID[3]; protectedPinnedUpdated {
		t.Fatalf("pinned protected memory should not decay: %+v", byID[3])
	}
	if _, protectedUserUpdated := byID[4]; protectedUserUpdated {
		t.Fatalf("user_corrected protected memory should not decay: %+v", byID[4])
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "importance_reevaluation" {
		t.Fatalf("expected importance_reevaluation audit, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["memory_importance_reweighting"] == nil {
		t.Fatalf("audit details missing TM-1c state: %+v", details)
	}
}

func TestMaintenanceEnqueueFalsePositiveSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{
	  "chat_session_id":"sess-clean",
	  "turn_index":14,
	  "assistant_response":"The rooftop confession continues with a quiet pause.",
	  "recent_responses":["A different sentence."],
	  "supervisor_result":{
	    "directive":{
	      "story_author":{"current_arc":"rooftop confession"},
	      "director":{"forbidden_moves":["no"]}
	    }
	  }
	}`
	req := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if signals, _ := resp["drift_signals"].([]any); len(signals) != 0 {
		t.Fatalf("false-positive drift signals = %+v, want none", signals)
	}
	if hints, _ := resp["correction_hints"].([]any); len(hints) != 0 {
		t.Fatalf("false-positive correction hints = %+v, want none", hints)
	}
}

func TestMaintenanceTM1dAuditReplayDirtyMatrixSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tm1d", TurnIndex: 5, SummaryJSON: `{"turn_summary":"recent mention of blue oath"}`, Importance: 0.5},
		},
		storylines: []store.Storyline{
			{Name: "arc-x", Status: "resolved", CurrentContext: "resolved arc"},
		},
		pendingThreads: []store.PendingThread{
			{ThreadKey: "thread-y", Status: "resolved", Description: "done"},
		},
		chatLogs: []store.ChatLog{
			{ChatSessionID: "sess-tm1d", TurnIndex: 11, Content: "blue oath"},
			{ChatSessionID: "sess-tm1d", TurnIndex: 12, Content: "continues"},
		},
	}
	srv := setupTestServer()
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body1 := `{"chat_session_id":"sess-tm1d-drift","turn_index":5,"assistant_response":"They erase the archive promise and walk away from the archive promise.","recent_responses":["archive promise archive promise"],"canonical_state_layers":[{"layer_type":"relationship_state","last_verified_turn":3,"confidence":0.82},{"layer_type":"scene_state","last_verified_turn":4,"confidence":0.33}],"supervisor_result":{"directive":{"story_author":{"current_arc":"archive promise"},"director":{"forbidden_moves":["erase the archive promise"]}}}}`
	req1 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("drift status = %d, want 200: %s", rec1.Code, rec1.Body.String())
	}
	var resp1 map[string]any
	if err := json.Unmarshal(rec1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("drift decode: %v", err)
	}

	refresh1, _ := resp1["refresh_output"].(map[string]any)
	if refresh1 == nil {
		t.Fatalf("drift refresh_output missing")
	}
	dirtyMatrix1, _ := refresh1["or_phase_dirty_matrix"].(map[string]any)
	if dirtyMatrix1 == nil {
		t.Fatalf("drift or_phase_dirty_matrix missing in response")
	}
	if dirtyMatrix1["policy_version"] != "tm1d.v1" {
		t.Fatalf("drift policy_version = %v, want tm1d.v1", dirtyMatrix1["policy_version"])
	}
	if dirtyMatrix1["matrix_version"] != "or1h.tm1d.v1" {
		t.Fatalf("drift matrix_version = %v, want or1h.tm1d.v1", dirtyMatrix1["matrix_version"])
	}
	rows1, _ := dirtyMatrix1["rows"].([]any)
	if len(rows1) == 0 {
		t.Fatalf("drift rows empty")
	}
	firstRow1, ok := rows1[0].(map[string]any)
	if !ok {
		t.Fatalf("drift row wrong type")
	}
	if firstRow1["event_type"] != "drift_detected" {
		t.Fatalf("drift row event_type = %v, want drift_detected", firstRow1["event_type"])
	}
	if firstRow1["dirty_scope"] != "relationship_state" {
		t.Fatalf("drift row dirty_scope = %v, want relationship_state", firstRow1["dirty_scope"])
	}
	if dirtyMatrix1["row_count"] != float64(len(rows1)) {
		t.Fatalf("drift row_count = %v, want %d", dirtyMatrix1["row_count"], len(rows1))
	}
	targets1, _ := firstRow1["dirty_targets"].([]any)
	if len(targets1) == 0 {
		t.Fatalf("drift row dirty_targets empty")
	}
	replay1, _ := refresh1["replay_measurements"].(map[string]any)
	if replay1 == nil {
		t.Fatalf("drift replay_measurements missing")
	}
	if replay1["tm_drift_pass_count"] != float64(1) {
		t.Fatalf("drift tm_drift_pass_count = %v, want 1", replay1["tm_drift_pass_count"])
	}

	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "drift_detected" {
		t.Fatalf("expected drift_detected audit, got %#v", fake.auditLogs)
	}
	var auditDetails1 map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &auditDetails1); err != nil {
		t.Fatalf("drift audit details decode: %v", err)
	}
	auditMatrix1, _ := auditDetails1["or_phase_dirty_matrix"].(map[string]any)
	if auditMatrix1 == nil {
		t.Fatalf("drift audit or_phase_dirty_matrix missing")
	}
	if auditMatrix1["event_type"] != "drift_detected" {
		t.Fatalf("drift audit event_type mismatch")
	}

	fake.auditLogs = nil
	body2 := `{"chat_session_id":"sess-tm1d-imp","turn_index":12,"assistant_response":"The blue oath is carried forward without contradiction."}`
	req2 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("imp status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("imp decode: %v", err)
	}

	refresh2, _ := resp2["refresh_output"].(map[string]any)
	if refresh2 == nil {
		t.Fatalf("imp refresh_output missing")
	}
	dirtyMatrix2, _ := refresh2["or_phase_dirty_matrix"].(map[string]any)
	if dirtyMatrix2 == nil {
		t.Fatalf("imp or_phase_dirty_matrix missing")
	}
	rows2, _ := dirtyMatrix2["rows"].([]any)
	if len(rows2) == 0 {
		t.Fatalf("imp rows empty")
	}
	if dirtyMatrix2["row_count"] != float64(len(rows2)) {
		t.Fatalf("imp row_count = %v, want %d", dirtyMatrix2["row_count"], len(rows2))
	}
	hasBoost := false
	for _, raw := range rows2 {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if row["event_type"] != "importance_reevaluation" {
			t.Fatalf("imp row event_type = %v", row["event_type"])
		}
		if row["dirty_scope"] != "memory_importance" {
			t.Fatalf("imp row dirty_scope = %v, want memory_importance", row["dirty_scope"])
		}
		if row["delta_direction"] == "boost" {
			hasBoost = true
		}
	}
	if !hasBoost {
		t.Fatalf("imp expected at least one boost row")
	}
	replay2, _ := refresh2["replay_measurements"].(map[string]any)
	if replay2 == nil {
		t.Fatalf("imp replay_measurements missing")
	}
	if replay2["tm_importance_pass_count"] != float64(1) {
		t.Fatalf("imp tm_importance_pass_count = %v, want 1", replay2["tm_importance_pass_count"])
	}

	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "importance_reevaluation" {
		t.Fatalf("expected importance_reevaluation audit, got %#v", fake.auditLogs)
	}
	var auditDetails2 map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &auditDetails2); err != nil {
		t.Fatalf("imp audit details decode: %v", err)
	}
	auditMatrix2, _ := auditDetails2["or_phase_dirty_matrix"].(map[string]any)
	if auditMatrix2 == nil {
		t.Fatalf("imp audit or_phase_dirty_matrix missing")
	}
	if auditMatrix2["event_type"] != "importance_reevaluation" {
		t.Fatalf("imp audit event_type mismatch")
	}
}

func TestSeq123P79PlanningOnlyBridgeMarkers(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200", rec.Code)
	}
	var r readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if r.Checks["live_cutover"] != "disabled" {
		t.Errorf("live_cutover = %q, want disabled", r.Checks["live_cutover"])
	}

	body := `{"chat_session_id":"sess-p79","turn_index":1,"shadow_only":true,"assistant_response":"ok","recent_responses":["a"]}`
	req2 := httptest.NewRequest(http.MethodPost, "/maintenance/enqueue", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("enqueue status = %d, want 200", rec2.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode enqueue: %v", err)
	}
	if resp["maintenance_pass_enabled"] != false {
		t.Errorf("maintenance_pass_enabled = %v, want false", resp["maintenance_pass_enabled"])
	}
	if resp["worker_enabled"] != false {
		t.Errorf("worker_enabled = %v, want false", resp["worker_enabled"])
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok || trace["non_blocking"] != true {
		t.Errorf("trace_summary non_blocking missing or false: %+v", trace)
	}

	req3 := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-p79","dry_run":true,"allow_shadow_boundary":true}`))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusServiceUnavailable {
		t.Errorf("reindex status = %d, want 503 (shadow guard blocks runtime activation)", rec3.Code)
	}
}

func TestAdminReindexUpsertsExistingMemoryEmbeddings(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:             42,
				ChatSessionID:  "sess-reindex-live",
				TurnIndex:      7,
				SummaryJSON:    `{"summary":"Blue lantern oath persists."}`,
				Embedding:      `[0.1,0.2,0.3]`,
				EmbeddingModel: "test-embedding",
			},
		},
	}
	vec := &turnRecordingVectorStore{}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-reindex-live","dry_run":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["reindex_executed"] != true {
		t.Fatalf("reindex_executed = %v, want true", resp["reindex_executed"])
	}
	if resp["upserted"] != float64(1) {
		t.Fatalf("upserted = %v, want 1", resp["upserted"])
	}
	qv, ok := resp["quality_verification"].(map[string]any)
	if !ok {
		t.Fatalf("quality_verification missing: %#v", resp["quality_verification"])
	}
	if qv["status"] != "requires_before_after_report" || qv["before_after_required"] != true {
		t.Fatalf("quality_verification = %#v, want before/after report required", qv)
	}
	if len(vec.docs) != 1 {
		t.Fatalf("vector docs = %d, want 1", len(vec.docs))
	}
	doc := vec.docs[0]
	if doc.ID != "memory:sess-reindex-live:42" {
		t.Fatalf("doc ID = %q", doc.ID)
	}
	if doc.ChatSessionID != "sess-reindex-live" || doc.SourceTable != "memories" || doc.SourceRowID != "42" {
		t.Fatalf("doc provenance mismatch: %#v", doc)
	}
	if !strings.Contains(doc.DocumentText, "Blue lantern oath persists.") || !strings.Contains(doc.DocumentText, "[Canonical Summary]") {
		t.Fatalf("doc text = %q", doc.DocumentText)
	}
	if len(fake.auditLogs) != 1 || fake.auditLogs[0].EventType != "admin_reindex" {
		t.Fatalf("expected admin_reindex audit log, got %#v", fake.auditLogs)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(fake.auditLogs[0].DetailsJSON), &details); err != nil {
		t.Fatalf("decode audit details: %v", err)
	}
	if details["upserted"] != float64(1) {
		t.Fatalf("audit upserted = %v, want 1", details["upserted"])
	}
	integrity, ok := resp["integrity_report"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_report missing: %#v", resp["integrity_report"])
	}
	if integrity["vector_count_matches_canonical"] != true {
		t.Fatalf("vector_count_matches_canonical = %v, want true; integrity=%#v", integrity["vector_count_matches_canonical"], integrity)
	}
	if integrity["missing_vector_count_estimate"] != float64(0) {
		t.Fatalf("missing_vector_count_estimate = %v, want 0", integrity["missing_vector_count_estimate"])
	}
}

func TestAdminReindexIntegrityReportDetectsMissingVectorsAndModelMismatch(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.EmbedderModel = "embed-model"
	srv := NewServer(cfg)
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-integrity", TurnIndex: 1, SummaryJSON: `{"summary":"Already indexed memory"}`, Embedding: `[0.1]`, EmbeddingModel: "embed-model"},
			{ID: 2, ChatSessionID: "sess-integrity", TurnIndex: 2, SummaryJSON: `{"summary":"Missing embedding memory"}`, Embedding: ``, EmbeddingModel: ""},
			{ID: 3, ChatSessionID: "sess-integrity", TurnIndex: 3, SummaryJSON: `{"summary":"Old model memory"}`, Embedding: `[0.3]`, EmbeddingModel: "old-model"},
		},
	}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = &fakeVectorStore{
		healthSnapshot: vector.HealthSnapshot{Status: "ok", TotalCount: 1, ProjectModel: "embed-model", ModelReady: true},
		countResult:    1,
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-integrity","dry_run":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	integrity, ok := resp["integrity_report"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_report missing: %#v", resp["integrity_report"])
	}
	if integrity["status"] != "reindex_recommended" {
		t.Fatalf("integrity status = %v, want reindex_recommended; integrity=%#v", integrity["status"], integrity)
	}
	if integrity["canonical_memory_count"] != float64(3) || integrity["vector_count"] != float64(1) {
		t.Fatalf("count mismatch in report: %#v", integrity)
	}
	if integrity["missing_vector_count_estimate"] != float64(2) {
		t.Fatalf("missing_vector_count_estimate = %v, want 2", integrity["missing_vector_count_estimate"])
	}
	if integrity["missing_embedding_count"] != float64(1) {
		t.Fatalf("missing_embedding_count = %v, want 1", integrity["missing_embedding_count"])
	}
	if integrity["embedding_model_mismatch_count"] != float64(1) {
		t.Fatalf("embedding_model_mismatch_count = %v, want 1", integrity["embedding_model_mismatch_count"])
	}
	if integrity["reindex_recommended"] != true || integrity["reembed_recommended"] != true {
		t.Fatalf("recommendations missing: %#v", integrity)
	}
	reasons := fmt.Sprint(integrity["reindex_reasons"])
	for _, want := range []string{"vector_count_below_canonical_memory_count", "memory_rows_missing_embedding", "embedding_model_mismatch"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("reindex_reasons = %v, missing %q", reasons, want)
		}
	}
}

func TestAdminReindexDryRunReportsDerivedArtifactVectorCandidates(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-reindex-derived", TurnIndex: 1, SummaryJSON: `{"summary":"Known memory"}`, Embedding: `[0.1]`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-reindex-derived", EvidenceText: "The cellar key is brass.", SourceTurnEnd: 1},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "sess-reindex-derived", Scope: "location", ScopeName: "cellar", Category: "access", Key: "brass_key", ValueJSON: `{"value":"The cellar opens with the brass key."}`, SourceTurn: 1},
		},
	}
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{docs: []vector.VectorDocument{
		{ID: "memory:sess-reindex-derived:1", Tier: "memory", ChatSessionID: "sess-reindex-derived", SourceTable: "memories", SourceRowID: "1"},
	}}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/reindex", strings.NewReader(`{"chat_session_id":"sess-reindex-derived","dry_run":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	derived, ok := resp["derived_artifact_reindex"].(map[string]any)
	if !ok {
		t.Fatalf("derived_artifact_reindex missing: %#v", resp)
	}
	candidates, ok := derived["candidates_by_tier"].(map[string]any)
	if !ok {
		t.Fatalf("candidates_by_tier missing: %#v", derived)
	}
	if candidates["evidence"] != float64(1) || candidates["world_rule"] != float64(1) {
		t.Fatalf("derived candidates = %#v, want evidence/world_rule 1", candidates)
	}
	integrity, ok := resp["integrity_report"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_report missing: %#v", resp)
	}
	if integrity["canonical_vector_candidate_count"] != float64(3) {
		t.Fatalf("canonical_vector_candidate_count = %v, want 3; integrity=%#v", integrity["canonical_vector_candidate_count"], integrity)
	}
	if integrity["canonical_evidence_vector_count"] != float64(1) || integrity["canonical_world_rule_vector_count"] != float64(1) {
		t.Fatalf("derived canonical counts missing: %#v", integrity)
	}
}

func TestAdminReindexBlocksPartialClientMetaEmbeddingWithoutEnvFallback(t *testing.T) {
	t.Setenv("AC_LT_EMBEDDING_API_KEY", "env-key")
	t.Setenv("AC_LT_EMBEDDING_ENDPOINT", "https://env.example.test/v1/embeddings")
	t.Setenv("AC_LT_EMBEDDING_PROVIDER", "voyageai")
	t.Setenv("AC_LT_EMBEDDING_MODEL", "voyage-3-large")

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-partial-embed", TurnIndex: 1, SummaryJSON: `{"summary":"Needs a new vector."}`},
		},
	}
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{}

	meta := map[string]any{
		"embedding": map[string]any{
			"provider": "voyageai",
			"model":    "voyage-3-large",
		},
	}
	cfgResolved := srv.completeTurnExtractionConfig(meta)
	if cfgResolved.Embedder.Source != "client_meta_partial" {
		t.Fatalf("embedding source = %q, want client_meta_partial", cfgResolved.Embedder.Source)
	}
	if cfgResolved.Embedder.APIKey != "" || cfgResolved.Embedder.Endpoint != "" {
		t.Fatalf("partial client_meta must not be filled from env: %#v", cfgResolved.Embedder)
	}

	resp, err := srv.runAdminReindexJob(context.Background(), "sess-partial-embed", map[string]any{
		"client_meta": meta,
		"force":       true,
	}, nil)
	if err != nil {
		t.Fatalf("runAdminReindexJob: %v", err)
	}
	if resp["status"] != "blocked" || resp["reason"] != "embedding_config_incomplete" {
		t.Fatalf("response = %#v, want blocked embedding_config_incomplete", resp)
	}
	trace, ok := resp["embedding_config_trace"].(map[string]any)
	if !ok || trace["source"] != "client_meta_partial" {
		t.Fatalf("embedding trace = %#v", resp["embedding_config_trace"])
	}
}

func TestCompleteTurnConfigBlocksPartialClientMetaCriticWithoutRuntimeFallback(t *testing.T) {
	srv := NewServer(config.Default())
	srv.RuntimeConfig.CriticAPIKey = "runtime-key"
	srv.RuntimeConfig.CriticEndpoint = "https://runtime.example.test/v1/chat/completions"
	srv.RuntimeConfig.CriticModel = "runtime-critic"
	srv.RuntimeConfig.CriticProvider = "openai"

	cfg := srv.completeTurnExtractionConfig(map[string]any{
		"critic": map[string]any{
			"provider": "openai",
			"model":    "ui-critic",
		},
	})

	if cfg.Critic.Source != "client_meta_partial.critic" {
		t.Fatalf("critic source = %q, want client_meta_partial.critic", cfg.Critic.Source)
	}
	if cfg.Critic.APIKey != "" || cfg.Critic.Endpoint != "" {
		t.Fatalf("partial client_meta critic must not be filled from runtime: %#v", cfg.Critic)
	}
	if cfg.Critic.hasConfig() {
		t.Fatalf("partial client_meta critic unexpectedly configured: %#v", cfg.Critic)
	}
}
