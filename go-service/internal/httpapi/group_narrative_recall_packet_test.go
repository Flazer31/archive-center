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

func TestNarrativeRecallPacketPreviewBuildsSceneAwareSupportPacket(t *testing.T) {
	dir := t.TempDir()
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":                     1,
		"chat_session_id":        "sess-nrp",
		"turn_index":             2,
		"summary_json":           `{"summary":"Mina Rowan brass key bridge promise changed trust after she held his hand."}`,
		"importance":             8.0,
		"emotional_boost":        0.8,
		"narrative_significance": 0.9,
		"emotional_intensity":    0.7,
		"created_at":             "2026-06-21T00:00:00Z",
	})
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":                     2,
		"chat_session_id":        "sess-nrp",
		"turn_index":             3,
		"summary_json":           `{"summary":"Mina Rowan brass key bridge promise was repeated when she hesitated."}`,
		"importance":             7.5,
		"emotional_boost":        0.5,
		"narrative_significance": 0.8,
		"emotional_intensity":    0.5,
		"created_at":             "2026-06-21T00:00:01Z",
	})
	writeLedgerFixture(t, dir, "memories", map[string]any{
		"id":                     3,
		"chat_session_id":        "sess-nrp",
		"turn_index":             4,
		"summary_json":           `{"summary":"Rowan noticed Mina no longer avoided his touch and became closer to him."}`,
		"importance":             7.0,
		"emotional_boost":        0.7,
		"narrative_significance": 0.7,
		"emotional_intensity":    0.4,
		"created_at":             "2026-06-21T00:00:02Z",
	})
	writeLedgerFixture(t, dir, "direct_evidence_records", map[string]any{
		"id":                   4,
		"chat_session_id":      "sess-nrp",
		"evidence_kind":        "relationship_shift",
		"evidence_text":        "Mina trusted Rowan enough to hold his hand.",
		"source_turn_start":    4,
		"source_turn_end":      4,
		"turn_anchor":          4,
		"archive_state":        "committed",
		"capture_verification": "verified",
		"created_at":           "2026-06-21T00:00:03Z",
	})
	writeLedgerFixture(t, dir, "pending_threads", map[string]any{
		"id":              5,
		"chat_session_id": "sess-nrp",
		"title":           "Ask Mina why she hesitated before taking Rowan's hand.",
		"thread_type":     "open_question",
		"status":          "open",
		"source_turn":     4,
		"priority":        70,
		"confidence":      0.8,
		"created_at":      "2026-06-21T00:00:04Z",
		"updated_at":      "2026-06-21T00:00:05Z",
	})
	writeLedgerFixture(t, dir, "active_states", map[string]any{
		"id":              6,
		"chat_session_id": "sess-nrp",
		"state_type":      "scene_state",
		"content":         "They are in the living room, standing close after a quiet date.",
		"turn_index":      5,
		"created_at":      "2026-06-21T00:00:06Z",
	})
	writeLedgerFixture(t, dir, "canonical_state_layers", map[string]any{
		"id":              7,
		"chat_session_id": "sess-nrp",
		"layer_type":      "relationship_state",
		"content":         "Mina is becoming more comfortable with Rowan.",
		"turn_index":      5,
		"confidence":      0.9,
		"created_at":      "2026-06-21T00:00:07Z",
	})
	writeLedgerFixture(t, dir, "storylines", map[string]any{
		"id":                    8,
		"chat_session_id":       "sess-nrp",
		"name":                  "Bridge promise",
		"status":                "active",
		"current_context":       "The bridge promise still shapes Mina and Rowan's trust.",
		"ongoing_tensions_json": `["Mina has not explained why she hesitated."]`,
		"confidence":            0.8,
		"evidence_count":        3,
		"first_turn":            2,
		"last_turn":             5,
		"created_at":            "2026-06-21T00:00:08Z",
		"updated_at":            "2026-06-21T00:00:09Z",
	})
	writeLedgerFixture(t, dir, "episode_summaries", map[string]any{
		"id":              9,
		"chat_session_id": "sess-nrp",
		"from_turn":       1,
		"to_turn":         5,
		"summary_text":    "Mina and Rowan moved from hesitation toward trust.",
		"key_events":      "Bridge promise; hand holding; quiet date",
		"open_loops_json": `["Mina's hesitation"]`,
		"created_at":      "2026-06-21T00:00:10Z",
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

	reqURL := "/narrative-recall/packet/preview?chat_session_id=sess-nrp&turn_index=6&raw_user_input=" +
		url.QueryEscape("Rowan gently takes Mina's hand on a quiet date.") +
		"&progression_profile=" + url.QueryEscape("AI추천")
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
	if resp["contract_version"] != narrativeRecallPacketContractVersion {
		t.Fatalf("contract_version=%v", resp["contract_version"])
	}
	if resp["read_only"] != true || resp["write_attempted"] != false || resp["llm_call_attempted"] != false || resp["vector_write_attempted"] != false {
		t.Fatalf("preview must stay read-only and no-call: %+v", resp)
	}
	carryover := resp["carryover"].(map[string]any)
	heavy := carryover["heavy_carryover"].([]any)
	light := carryover["light_resurfacing_tag"].([]any)
	if len(heavy) == 0 || len(light) == 0 {
		t.Fatalf("expected heavy carryover and light resurfacing: %+v", carryover)
	}
	foundSameIncidentDemotion := false
	for _, raw := range light {
		item := raw.(map[string]any)
		if item["demotion_reason"] == "same_incident_demoted" {
			foundSameIncidentDemotion = true
			break
		}
	}
	if !foundSameIncidentDemotion {
		t.Fatalf("same incident duplicate was not demoted to light tag: %+v", light)
	}
	policy := carryover["foreground_policy"].(map[string]any)
	if policy["same_incident_foreground_cap"] != float64(1) {
		t.Fatalf("foreground policy missing same incident cap: %+v", policy)
	}

	scene := resp["scene_microstate"].(map[string]any)
	if scene["scene_type"] != "romance" || scene["immediate_pressure"] != "medium" {
		t.Fatalf("scene microstate mismatch: %+v", scene)
	}
	profile := resp["progression_profile"].(map[string]any)
	if profile["resolved"] != "romance" || profile["label"] != "낭만" || profile["source"] != "ai_recommend" {
		t.Fatalf("progression profile mismatch: %+v", profile)
	}
	relationship := resp["relationship_packet"].(map[string]any)
	shift := relationship["relationship_shift"].(map[string]any)
	if shift["summary"] == "" {
		t.Fatalf("relationship shift missing: %+v", relationship)
	}
	if len(relationship["unresolved_tension"].([]any)) == 0 {
		t.Fatalf("unresolved tension missing: %+v", relationship)
	}
	opportunity := resp["new_scene_opportunity"].(map[string]any)
	if opportunity["slot"] == "none" || opportunity["authority"] != "opportunity_not_mandate" {
		t.Fatalf("new scene opportunity mismatch: %+v", opportunity)
	}
	promptTrace := resp["prompt_authority_trace"].(map[string]any)
	supervisor := promptTrace["supervisor_system_authority"].(map[string]any)
	if supervisor["source"] != "file" || supervisor["code_fallback_can_override"] != false {
		t.Fatalf("supervisor prompt authority trace mismatch: %+v", supervisor)
	}
}

func TestNarrativeRecallPacketPreviewMissingSessionID(t *testing.T) {
	srv := NewServer(config.Default())
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/narrative-recall/packet/preview", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
