package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestStep22AdoptionGatePreviewReadyWhenAllReplayGatesGreen(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":                     1,
		"chat_session_id":        "sess-step22",
		"turn_index":             3,
		"summary_json":           `{"summary":"Mina trusted Rowan after the bridge promise and moved closer to him."}`,
		"importance":             8.0,
		"emotional_boost":        0.5,
		"narrative_significance": 0.7,
		"created_at":             "2026-06-22T00:00:00Z",
	})
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   2,
		"chat_session_id":      "sess-step22",
		"evidence_kind":        "relationship_shift",
		"evidence_text":        "<thinking>private chain</thinking>Mina trusted Rowan enough to hold his hand.",
		"source_turn_start":    3,
		"source_turn_end":      3,
		"turn_anchor":          3,
		"archive_state":        "committed",
		"capture_verification": "verified",
		"created_at":           "2026-06-22T00:00:01Z",
	})
	writeLedgerFixture(t, dir, "pending_threads", map[string]any{
		"id":              3,
		"chat_session_id": "sess-step22",
		"title":           "Ask Mina why she hesitated before taking Rowan's hand.",
		"thread_type":     "open_question",
		"status":          "open",
		"source_turn":     3,
		"priority":        50,
		"confidence":      0.8,
		"created_at":      "2026-06-22T00:00:02Z",
		"updated_at":      "2026-06-22T00:00:03Z",
	})
	writeLedgerFixture(t, dir, "active_states", map[string]any{
		"id":              4,
		"chat_session_id": "sess-step22",
		"state_type":      "scene_state",
		"content":         "Mina and Rowan are close together after a quiet date.",
		"turn_index":      4,
		"created_at":      "2026-06-22T00:00:04Z",
	})
	writeLedgerFixture(t, dir, "canonical_state_layers", map[string]any{
		"id":              5,
		"chat_session_id": "sess-step22",
		"layer_type":      "relationship_state",
		"content":         "Mina is more comfortable with Rowan.",
		"turn_index":      4,
		"confidence":      0.9,
		"created_at":      "2026-06-22T00:00:05Z",
	})
	writeLedgerFixture(t, dir, "audit_logs", map[string]any{
		"id":              6,
		"chat_session_id": "sess-step22",
		"event_type":      "supersession_resolution",
		"target_type":     "memory",
		"target_id":       99,
		"summary":         "Resolution stale_demote: old bridge rumor",
		"details_json":    `{"contract_version":"supersession_resolution.v1","resolution_class":"stale_demote","target":{"type":"memory","id":99},"hard_delete":false}`,
		"source":          "critic",
		"created_at":      "2026-06-22T00:00:06Z",
	})
	writeLedgerFixture(t, dir, "storylines", map[string]any{
		"id":                    7,
		"chat_session_id":       "sess-step22",
		"name":                  "Bridge trust",
		"status":                "active",
		"current_context":       "Mina and Rowan are rebuilding trust after the bridge promise.",
		"ongoing_tensions_json": `["Mina has not explained her hesitation."]`,
		"confidence":            0.8,
		"evidence_count":        2,
		"first_turn":            2,
		"last_turn":             4,
		"created_at":            "2026-06-22T00:00:07Z",
		"updated_at":            "2026-06-22T00:00:08Z",
	})

	fixture, err := store.NewFixtureStoreFromExportDir(dir)
	if err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	promptDir := t.TempDir()
	if err := os.WriteFile(promptDir+"/supervisor_system.txt", []byte("supervisor prompt"), 0644); err != nil {
		t.Fatalf("write supervisor prompt: %v", err)
	}
	if err := os.WriteFile(promptDir+"/critic_system.txt", []byte("critic prompt"), 0644); err != nil {
		t.Fatalf("write critic prompt: %v", err)
	}
	cfg := config.Default()
	cfg.PromptDir = promptDir
	srv := NewServer(cfg)
	srv.Store = fixture
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	reqURL := "/validation/step22/adoption-gate/preview?chat_session_id=sess-step22" +
		"&turn_index=5" +
		"&assistant_final_language=ko" +
		"&assistant_final_text=" + url.QueryEscape("미나는 로완의 손을 잡고 조용히 고개를 끄덕였다.") +
		"&raw_user_input=" + url.QueryEscape("Rowan gently takes Mina's hand on a quiet date.") +
		"&progression_profile=" + url.QueryEscape("AI추천") +
		"&streaming_mismatch=none" +
		"&schema_migration_state=green&backfill_state=green&rollback_state=green"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, reqURL, nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["contract_version"] != step22AdoptionGateContractVersion {
		t.Fatalf("contract_version=%v", resp["contract_version"])
	}
	if resp["adoption_gate_state"] != "ready" || resp["default_enable_allowed"] != true || resp["status"] != "ok" {
		t.Fatalf("gate should be ready: %+v", resp)
	}
	if resp["write_attempted"] != false || resp["vector_write_attempted"] != false || resp["llm_call_attempted"] != false {
		t.Fatalf("gate must remain read-only/no-call: %+v", resp)
	}
	checks := resp["checks"].([]any)
	if len(checks) != len(step22RequiredGreenGates()) {
		t.Fatalf("checks=%d want %d", len(checks), len(step22RequiredGreenGates()))
	}
	for _, raw := range checks {
		check := raw.(map[string]any)
		if check["status"] != "pass" {
			t.Fatalf("check did not pass: %+v\nbody=%s", check, rec.Body.String())
		}
	}
}

func TestStep22AdoptionGatePreviewBlocksWhenOpsGateMissing(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/validation/step22/adoption-gate/preview?chat_session_id=sess-empty", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["adoption_gate_state"] != "closed" || resp["default_enable_allowed"] != false {
		t.Fatalf("gate should stay closed without ops evidence: %+v", resp)
	}
	blockers := resp["blockers"].([]any)
	found := false
	for _, raw := range blockers {
		if raw == "schema_migration_backfill_rollback" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("schema/backfill/rollback blocker missing: %+v", resp)
	}
}

func TestStep22AdoptionGatePreviewRequiresSession(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/validation/step22/adoption-gate/preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
