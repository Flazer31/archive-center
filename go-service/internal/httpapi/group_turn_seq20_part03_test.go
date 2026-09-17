package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq20P248RollupQ20fLightweightEntityIndex confirms that the q20f
// lightweight entity index surface is present for SEQ-20-P248.
func TestSeq20P248RollupQ20fLightweightEntityIndex(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p248","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20f_v1_lightweight_entity_index")
	if s["version"] != "q20f.v1" {
		t.Fatalf("version=%v, want q20f.v1", s["version"])
	}
	if s["mode"] != "q20f_v1_lightweight_entity_index_definition" {
		t.Fatalf("mode=%v, want q20f_v1_lightweight_entity_index_definition", s["mode"])
	}
}

// TestSeq20P249RollupQ20gGraphLikeSupportSignal confirms that the q20g
// graph-like support signal surface is present for SEQ-20-P249.
func TestSeq20P249RollupQ20gGraphLikeSupportSignal(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p249","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_v1_graph_like_support_signal")
	if s["version"] != "q20g.v1" {
		t.Fatalf("version=%v, want q20g.v1", s["version"])
	}
	if s["mode"] != "q20g_v1_graph_like_support_signal_definition" {
		t.Fatalf("mode=%v, want q20g_v1_graph_like_support_signal_definition", s["mode"])
	}
}

// TestSeq20P250RollupQ20hEntityGraphBoostInspection confirms that the q20h
// entity/graph boost inspection surface is present for SEQ-20-P250.
func TestSeq20P250RollupQ20hEntityGraphBoostInspection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p250","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20h_v1_entity_graph_boost_inspection_surface")
	if s["version"] != "q20h.v1" {
		t.Fatalf("version=%v, want q20h.v1", s["version"])
	}
	if s["mode"] != "q20h_v1_entity_graph_boost_inspection_surface_definition" {
		t.Fatalf("mode=%v, want q20h_v1_entity_graph_boost_inspection_surface_definition", s["mode"])
	}
}

// TestSeq20P251RollupQ20iLaggingCurrentStateBoost confirms that the q20i
// lagging current state boost surface is present for SEQ-20-P251.
func TestSeq20P251RollupQ20iLaggingCurrentStateBoost(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p251","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20i_v1_lagging_current_state_boost")
	if s["version"] != "q20i.v1" {
		t.Fatalf("version=%v, want q20i.v1", s["version"])
	}
	if s["mode"] != "q20i_v1_lagging_current_state_boost_definition" {
		t.Fatalf("mode=%v, want q20i_v1_lagging_current_state_boost_definition", s["mode"])
	}
}

// TestSeq20P252RollupQ20jMotiveShadowHint confirms that the q20j motive-shadow
// hint surface is present for SEQ-20-P252.
func TestSeq20P252RollupQ20jMotiveShadowHint(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p252","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20j_v1_motive_shadow_hint")
	if s["version"] != "q20j.v1" {
		t.Fatalf("version=%v, want q20j.v1", s["version"])
	}
	if s["mode"] != "q20j_v1_motive_shadow_hint_definition" {
		t.Fatalf("mode=%v, want q20j_v1_motive_shadow_hint_definition", s["mode"])
	}
}

// TestSeq20P253RollupQ20kMotiveShadowNonEscalationGuard confirms that the q20k
// motive-shadow non-escalation guard surface is present for SEQ-20-P253.
func TestSeq20P253RollupQ20kMotiveShadowNonEscalationGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p253","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20k_v1_motive_shadow_non_escalation_guard")
	if s["version"] != "q20k.v1" {
		t.Fatalf("version=%v, want q20k.v1", s["version"])
	}
	if s["mode"] != "q20k_v1_motive_shadow_non_escalation_guard_definition" {
		t.Fatalf("mode=%v, want q20k_v1_motive_shadow_non_escalation_guard_definition", s["mode"])
	}
}

// TestSeq20P254RollupQ20lRelationEdgeSupportLedger confirms that the q20l
// relation edge support ledger surface is present for SEQ-20-P254.
func TestSeq20P254RollupQ20lRelationEdgeSupportLedger(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p254","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20l_v1_relation_edge_support_ledger")
	if s["version"] != "q20l.v1" {
		t.Fatalf("version=%v, want q20l.v1", s["version"])
	}
	if s["mode"] != "q20l_v1_relation_edge_support_ledger_definition" {
		t.Fatalf("mode=%v, want q20l_v1_relation_edge_support_ledger_definition", s["mode"])
	}
}

func TestSeq20P258Q20mTemporalAmbiguitySupportNotePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p258","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20m_temporal_ambiguity_support_note_preparatory")
	if s["version"] != "q20m-p258.v1" {
		t.Fatalf("version=%v, want q20m-p258.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20m_temporal_ambiguity_support_note_preparatory_definition" {
		t.Fatalf("mode=%v, want q20m_temporal_ambiguity_support_note_preparatory_definition", s["mode"])
	}
}

func TestSeq20P259Q20mV1TemporalAmbiguitySupportNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p259","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20m_v1_temporal_ambiguity_support_note")
	if s["version"] != "q20m.v1" {
		t.Fatalf("version=%v, want q20m.v1", s["version"])
	}
	composes := seq165Slice(t, s, "composes")
	for _, expected := range []string{"q20c_exact_event_compare_note", "q20d_bounded_window_compare_note", "q20e_chronology_support_note"} {
		if !sliceContains(composes, expected) {
			t.Fatalf("composes missing %q: %v", expected, composes)
		}
	}
	if s["disabled_for_current_clock_query"] != true {
		t.Fatalf("disabled_for_current_clock_query=%v, want true", s["disabled_for_current_clock_query"])
	}
	if s["deferred_chronology_gap_visible"] != true {
		t.Fatalf("deferred_chronology_gap_visible=%v, want true", s["deferred_chronology_gap_visible"])
	}
	if s["fake_fill_blocked"] != true {
		t.Fatalf("fake_fill_blocked=%v, want true", s["fake_fill_blocked"])
	}
	if s["mode"] != "q20m_v1_temporal_ambiguity_support_note_definition" {
		t.Fatalf("mode=%v, want q20m_v1_temporal_ambiguity_support_note_definition", s["mode"])
	}
}

func TestSeq20P260Q20nAliasEntityConflictDisambiguationPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p260","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20n_alias_entity_conflict_disambiguation_preparatory")
	if s["version"] != "q20n-p260.v1" {
		t.Fatalf("version=%v, want q20n-p260.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20n_alias_entity_conflict_disambiguation_preparatory_definition" {
		t.Fatalf("mode=%v, want q20n_alias_entity_conflict_disambiguation_preparatory_definition", s["mode"])
	}
}

func TestSeq20P261Q20nV1AliasEntityConflictDisambiguation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p261","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20n_v1_alias_entity_conflict_disambiguation")
	if s["version"] != "q20n.v1" {
		t.Fatalf("version=%v, want q20n.v1", s["version"])
	}
	if s["explicit_alias_table"] != false {
		t.Fatalf("explicit_alias_table=%v, want false", s["explicit_alias_table"])
	}
	if s["structured_label_collision_only"] != true {
		t.Fatalf("structured_label_collision_only=%v, want true", s["structured_label_collision_only"])
	}
	if s["auto_resolution"] != false {
		t.Fatalf("auto_resolution=%v, want false", s["auto_resolution"])
	}
	if s["output"] != "candidate_entries_only" {
		t.Fatalf("output=%v, want candidate_entries_only", s["output"])
	}
	if s["mode"] != "q20n_v1_alias_entity_conflict_disambiguation_definition" {
		t.Fatalf("mode=%v, want q20n_v1_alias_entity_conflict_disambiguation_definition", s["mode"])
	}
}

func TestSeq20P262Q20oTemporalEntitySourceTagRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p262","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20o_temporal_entity_source_tag_rule_preparatory")
	if s["version"] != "q20o-p262.v1" {
		t.Fatalf("version=%v, want q20o-p262.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20o_temporal_entity_source_tag_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20o_temporal_entity_source_tag_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P263Q20oV1TemporalEntitySourceTagRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p263","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20o_v1_temporal_entity_source_tag_rule")
	if s["version"] != "q20o.v1" {
		t.Fatalf("version=%v, want q20o.v1", s["version"])
	}
	catalogs := seq165Slice(t, s, "source_catalogs")
	for _, expected := range []string{"source_surfaces", "source_catalogs", "source_lanes"} {
		if !sliceContains(catalogs, expected) {
			t.Fatalf("source_catalogs missing %q: %v", expected, catalogs)
		}
	}
	if s["relabeling_blocked"] != true {
		t.Fatalf("relabeling_blocked=%v, want true", s["relabeling_blocked"])
	}
	if s["fake_alias_blocked"] != true {
		t.Fatalf("fake_alias_blocked=%v, want true", s["fake_alias_blocked"])
	}
	if s["mode"] != "q20o_v1_temporal_entity_source_tag_rule_definition" {
		t.Fatalf("mode=%v, want q20o_v1_temporal_entity_source_tag_rule_definition", s["mode"])
	}
}

func TestSeq20P264Q20pCanonicalPendingStaleCurrentConflictNotePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p264","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20p_canonical_pending_stale_current_conflict_note_preparatory")
	if s["version"] != "q20p-p264.v1" {
		t.Fatalf("version=%v, want q20p-p264.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20p_canonical_pending_stale_current_conflict_note_preparatory_definition" {
		t.Fatalf("mode=%v, want q20p_canonical_pending_stale_current_conflict_note_preparatory_definition", s["mode"])
	}
}

func TestSeq20P265Q20pV1CanonicalPendingStaleCurrentConflictNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p265","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20p_v1_canonical_pending_stale_current_conflict_note")
	if s["version"] != "q20p.v1" {
		t.Fatalf("version=%v, want q20p.v1", s["version"])
	}
	assembles := seq165Slice(t, s, "assembles")
	for _, expected := range []string{"pending_current_gap", "hot_buffer", "optional_lagging_boost"} {
		if !sliceContains(assembles, expected) {
			t.Fatalf("assembles missing %q: %v", expected, assembles)
		}
	}
	if s["read_only_conflict_note"] != true {
		t.Fatalf("read_only_conflict_note=%v, want true", s["read_only_conflict_note"])
	}
	if s["mode"] != "q20p_v1_canonical_pending_stale_current_conflict_note_definition" {
		t.Fatalf("mode=%v, want q20p_v1_canonical_pending_stale_current_conflict_note_definition", s["mode"])
	}
}

func TestSeq20P266Q20qRecallCueRescueRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p266","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20q_recall_cue_rescue_rule_preparatory")
	if s["version"] != "q20q-p266.v1" {
		t.Fatalf("version=%v, want q20q-p266.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20q_recall_cue_rescue_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20q_recall_cue_rescue_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P267Q20qV1RecallCueRescueRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p267","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20q_v1_recall_cue_rescue_rule")
	if s["version"] != "q20q.v1" {
		t.Fatalf("version=%v, want q20q.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20a_temporal_expansion", "qr1a_callback_lexical_signal", "qr1d_old_detail_signal"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	if s["new_cue_table"] != false {
		t.Fatalf("new_cue_table=%v, want false", s["new_cue_table"])
	}
	if s["recall_widening_only"] != true {
		t.Fatalf("recall_widening_only=%v, want true", s["recall_widening_only"])
	}
	if s["plain_detail_request_excluded"] != true {
		t.Fatalf("plain_detail_request_excluded=%v, want true", s["plain_detail_request_excluded"])
	}
	if s["mode"] != "q20q_v1_recall_cue_rescue_rule_definition" {
		t.Fatalf("mode=%v, want q20q_v1_recall_cue_rescue_rule_definition", s["mode"])
	}
}

func TestSeq20P268Q20rWideGatherValidityJoinRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p268","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20r_wide_gather_validity_join_rule_preparatory")
	if s["version"] != "q20r-p268.v1" {
		t.Fatalf("version=%v, want q20r-p268.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20r_wide_gather_validity_join_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20r_wide_gather_validity_join_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P269Q20rV1WideGatherValidityJoinRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p269","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20r_v1_wide_gather_validity_join_rule")
	if s["version"] != "q20r.v1" {
		t.Fatalf("version=%v, want q20r.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20b_read_priority", "q20m_compare_note_surface", "q20q_rescue_surface"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	authorities := seq165Slice(t, s, "validity_join_authorities")
	for _, expected := range []string{"mariadb_canonical_truth", "storyline_status", "temporal_validity"} {
		if !sliceContains(authorities, expected) {
			t.Fatalf("validity_join_authorities missing %q: %v", expected, authorities)
		}
	}
	if s["bounded_wide_gather"] != true {
		t.Fatalf("bounded_wide_gather=%v, want true", s["bounded_wide_gather"])
	}
	if s["validity_join_for_temporal_queries_only"] != true {
		t.Fatalf("validity_join_for_temporal_queries_only=%v, want true", s["validity_join_for_temporal_queries_only"])
	}
	if s["callback_only_recall_fail_open"] != true {
		t.Fatalf("callback_only_recall_fail_open=%v, want true", s["callback_only_recall_fail_open"])
	}
	if s["mode"] != "q20r_v1_wide_gather_validity_join_rule_definition" {
		t.Fatalf("mode=%v, want q20r_v1_wide_gather_validity_join_rule_definition", s["mode"])
	}
}

func TestSeq20P270Q20sThinSupportTagFallbackPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p270","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20s_thin_support_tag_fallback_preparatory")
	if s["version"] != "q20s-p270.v1" {
		t.Fatalf("version=%v, want q20s-p270.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20s_thin_support_tag_fallback_preparatory_definition" {
		t.Fatalf("mode=%v, want q20s_thin_support_tag_fallback_preparatory_definition", s["mode"])
	}
}

func TestSeq20P271Q20sV1ThinSupportTagFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p271","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20s_v1_thin_support_tag_fallback")
	if s["version"] != "q20s.v1" {
		t.Fatalf("version=%v, want q20s.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20e_thin_tag_mode", "q20o_source_tags"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	if s["low_density_support_visibility"] != true {
		t.Fatalf("low_density_support_visibility=%v, want true", s["low_density_support_visibility"])
	}
	if s["requires_prior_validity_join"] != true {
		t.Fatalf("requires_prior_validity_join=%v, want true", s["requires_prior_validity_join"])
	}
	if s["drop_replacement"] != "thin_support_tag" {
		t.Fatalf("drop_replacement=%v, want thin_support_tag", s["drop_replacement"])
	}
	if s["mode"] != "q20s_v1_thin_support_tag_fallback_definition" {
		t.Fatalf("mode=%v, want q20s_v1_thin_support_tag_fallback_definition", s["mode"])
	}
}

func TestSeq20P286Vx20aTemporalValidityReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p286","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20a_temporal_validity_replay_gate")
	if s["version"] != "vx20a.v1" {
		t.Fatalf("version=%v, want vx20a.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20b_read_priority", "q20m_compare_note_surface", "q20r_join_mode"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "temporal_validity_contract_drift_only" {
		t.Fatalf("check_target=%v, want temporal_validity_contract_drift_only", s["check_target"])
	}
	if s["mode"] != "vx20a_v1_temporal_validity_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20a_v1_temporal_validity_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P288Vx20bEntityBoostFalsePositiveGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p288","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20b_entity_boost_false_positive_gate")
	if s["version"] != "vx20b.v1" {
		t.Fatalf("version=%v, want vx20b.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20h_inspection_surface", "q20i_lagging_boost_surface"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["false_positive_ceiling"] != "lc1j_reuse" {
		t.Fatalf("false_positive_ceiling=%v, want lc1j_reuse", s["false_positive_ceiling"])
	}
	if s["mode"] != "vx20b_v1_entity_boost_false_positive_gate_definition" {
		t.Fatalf("mode=%v, want vx20b_v1_entity_boost_false_positive_gate_definition", s["mode"])
	}
}

func TestSeq20P290Vx20cGraphAcceleratorDegradeGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p290","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20c_graph_accelerator_degrade_gate")
	if s["version"] != "vx20c.v1" {
		t.Fatalf("version=%v, want vx20c.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20g_graph_like_support", "q20h_inspection_surface", "q20i_lagging_boost_surface"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["degrade_scenario"] != "graph_accelerator_off" {
		t.Fatalf("degrade_scenario=%v, want graph_accelerator_off", s["degrade_scenario"])
	}
	if s["fail_open_required"] != true {
		t.Fatalf("fail_open_required=%v, want true", s["fail_open_required"])
	}
	if s["entity_boost_survives"] != true {
		t.Fatalf("entity_boost_survives=%v, want true", s["entity_boost_survives"])
	}
	if s["mode"] != "vx20c_v1_graph_accelerator_degrade_gate_definition" {
		t.Fatalf("mode=%v, want vx20c_v1_graph_accelerator_degrade_gate_definition", s["mode"])
	}
}

func TestSeq20P292Vx20dCanonicalPrecedenceReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p292","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20d_canonical_precedence_replay_gate")
	if s["version"] != "vx20d.v1" {
		t.Fatalf("version=%v, want vx20d.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20p_source_precedence", "q20r_join_authority_ordering"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "canon_support_precedence_drift_only" {
		t.Fatalf("check_target=%v, want canon_support_precedence_drift_only", s["check_target"])
	}
	if s["mode"] != "vx20d_v1_canonical_precedence_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20d_v1_canonical_precedence_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P294Vx20ePromotionBlockedFreshnessReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p294","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20e_promotion_blocked_freshness_replay_gate")
	if s["version"] != "vx20e.v1" {
		t.Fatalf("version=%v, want vx20e.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20d_promotion_lag_support", "q20e_hot_recall_buffer", "q20p_canonical_pending_conflict", "q20i_lagging_boost"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "pending_current_visibility_preservation" {
		t.Fatalf("check_target=%v, want pending_current_visibility_preservation", s["check_target"])
	}
	if s["mode"] != "vx20e_v1_promotion_blocked_freshness_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20e_v1_promotion_blocked_freshness_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P296Vx20fRecallCueRescueReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p296","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20f_recall_cue_rescue_replay_gate")
	if s["version"] != "vx20f.v1" {
		t.Fatalf("version=%v, want vx20f.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	if !sliceContains(refs, "q20q_rescue_surface") {
		t.Fatalf("references missing q20q_rescue_surface: %v", refs)
	}
	if s["check_target"] != "over_filter_miss_reduction" {
		t.Fatalf("check_target=%v, want over_filter_miss_reduction", s["check_target"])
	}
	if s["stale_arc_auto_foreground"] != false {
		t.Fatalf("stale_arc_auto_foreground=%v, want false", s["stale_arc_auto_foreground"])
	}
	if s["mode"] != "vx20f_v1_recall_cue_rescue_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20f_v1_recall_cue_rescue_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P298Vx20gHotBufferWideGatherNonRegressionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p298","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx20g_hot_buffer_wide_gather_non_regression_gate")
	if s["version"] != "vx20g.v1" {
		t.Fatalf("version=%v, want vx20g.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20e_hot_recall_buffer", "q20r_wide_gather_validity_join", "q20s_thin_support_tag_fallback", "vx18c_upstream_gate", "vx18d_upstream_gate"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "truth_boundary_latency_ceiling" {
		t.Fatalf("check_target=%v, want truth_boundary_latency_ceiling", s["check_target"])
	}
	if s["step20_closeout"] != true {
		t.Fatalf("step20_closeout=%v, want true", s["step20_closeout"])
	}
	if s["mode"] != "vx20g_v1_hot_buffer_wide_gather_non_regression_gate_definition" {
		t.Fatalf("mode=%v, want vx20g_v1_hot_buffer_wide_gather_non_regression_gate_definition", s["mode"])
	}
}

func TestSeq20P312Beta11BundleDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p312","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_beta11_bundle_dry_run")
	if s["version"] != "s20-p312.v1" {
		t.Fatalf("version=%v, want s20-p312.v1", s["version"])
	}
	if s["dry_run"] != true {
		t.Fatalf("dry_run=%v, want true", s["dry_run"])
	}
	if s["mode"] != "seq20_beta11_bundle_dry_run_definition" {
		t.Fatalf("mode=%v, want seq20_beta11_bundle_dry_run_definition", s["mode"])
	}
}

func TestSeq20P313TemporalValidityRecallSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p313","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_temporal_validity_recall_smoke")
	if s["version"] != "s20-p313.v1" {
		t.Fatalf("version=%v, want s20-p313.v1", s["version"])
	}
	if s["smoke_target"] != "temporal_validity_recall" {
		t.Fatalf("smoke_target=%v, want temporal_validity_recall", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20a_temporal_query_expansion", "q20b_temporal_validity_read_policy", "q20m_temporal_ambiguity_support_note"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_temporal_validity_recall_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_validity_recall_smoke_definition", s["mode"])
	}
}

func TestSeq20P314EntityGraphAcceleratorSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p314","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_entity_graph_accelerator_smoke")
	if s["version"] != "s20-p314.v1" {
		t.Fatalf("version=%v, want s20-p314.v1", s["version"])
	}
	if s["smoke_target"] != "entity_graph_accelerator" {
		t.Fatalf("smoke_target=%v, want entity_graph_accelerator", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20f_lightweight_entity_index", "q20g_graph_like_support_signal", "q20h_entity_graph_boost_inspection", "q20i_lagging_current_state_boost"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_entity_graph_accelerator_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_entity_graph_accelerator_smoke_definition", s["mode"])
	}
}

func TestSeq20P315TemporalEntityDisambiguationSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p315","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_temporal_entity_disambiguation_smoke")
	if s["version"] != "s20-p315.v1" {
		t.Fatalf("version=%v, want s20-p315.v1", s["version"])
	}
	if s["smoke_target"] != "temporal_entity_disambiguation" {
		t.Fatalf("smoke_target=%v, want temporal_entity_disambiguation", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20n_alias_entity_conflict_disambiguation", "q20o_temporal_entity_source_tag_rule", "q20q_recall_cue_rescue_rule", "q20r_wide_gather_validity_join_rule"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_temporal_entity_disambiguation_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_entity_disambiguation_smoke_definition", s["mode"])
	}
}

func TestSeq20P316PrecedenceAmbiguityReviewChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p316","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_precedence_ambiguity_review_checklist")
	if s["version"] != "s20-p316.v1" {
		t.Fatalf("version=%v, want s20-p316.v1", s["version"])
	}
	items := seq165Slice(t, s, "checklist_items")
	for _, expected := range []string{"canonical_precedence_preserved", "support_lane_read_only", "ambiguity_reduction_active", "truth_boundary_intact"} {
		if !sliceContains(items, expected) {
			t.Fatalf("checklist_items missing %q: %v", expected, items)
		}
	}
	if s["mode"] != "seq20_precedence_ambiguity_review_checklist_definition" {
		t.Fatalf("mode=%v, want seq20_precedence_ambiguity_review_checklist_definition", s["mode"])
	}
}

func TestSeq20P330TemporalQueryExpansionPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_temporal_query_expansion_preserve")
	if s["version"] != "s20-p330.v1" {
		t.Fatalf("version=%v, want s20-p330.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"rule_first_temporal_expansion", "metadata_support_confirmation"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_temporal_query_expansion_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_query_expansion_preserve_definition", s["mode"])
	}
}

func TestSeq20P331EntityIndexPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_entity_index_preserve")
	if s["version"] != "s20-p331.v1" {
		t.Fatalf("version=%v, want s20-p331.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"lightweight_entity_event_axis", "bounded_relation_edge_summary_granularity"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_entity_index_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_entity_index_preserve_definition", s["mode"])
	}
}
