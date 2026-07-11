package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// SEQ-16-P210: index lifecycle — the prepare-turn response must expose an
// index_lifecycle surface that remigrates the legacy
// backend/tests/test_q1c_index_lifecycle.py contract.
func TestSeq16P210IndexLifecycle(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p210", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p210","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	lc, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if lc["version"] != "p210a.v1" {
		t.Fatalf("expected version p210a.v1, got %v", lc["version"])
	}
	if lc["lifecycle_policy"] != "index_lifecycle_shadow" {
		t.Fatalf("unexpected lifecycle_policy: %v", lc["lifecycle_policy"])
	}
	if lc["shadow_status"] != "shadow" {
		t.Fatalf("shadow_status = %v, want shadow", lc["shadow_status"])
	}
	if lc["configured"] != false || lc["health_checked"] != true || lc["model_ready"] != false {
		t.Fatalf("configured/health_checked/model_ready = %v/%v/%v, want false/true/false", lc["configured"], lc["health_checked"], lc["model_ready"])
	}
	if lc["search_attempted"] != false {
		t.Fatalf("search_attempted = %v, want false", lc["search_attempted"])
	}
	if lc["rebuild_ready"] != false {
		t.Fatalf("rebuild_ready = %v, want false for unconfigured/model-not-ready vector shadow", lc["rebuild_ready"])
	}
	if lc["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", lc["truth_store"])
	}
	if lc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", lc["retrieval_role"])
	}
	if lc["reason"] != "seq16_p210_index_lifecycle" {
		t.Fatalf("unexpected reason: %v", lc["reason"])
	}
}

// SEQ-16-P211: source lookup audit — the prepare-turn response must expose
// a source_lookup_audit surface that remigrates the legacy
// backend/tests/test_q1d_source_lookup_audit.py contract.
func TestSeq16P211SourceLookupAudit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p211", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p211", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p211", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p211", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p211","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	audit, ok := resp["source_lookup_audit"].(map[string]any)
	if !ok {
		t.Fatalf("missing source_lookup_audit")
	}
	if audit["version"] != "p211a.v1" {
		t.Fatalf("expected version p211a.v1, got %v", audit["version"])
	}
	if audit["audit_policy"] != "source_lookup_inspectable" {
		t.Fatalf("unexpected audit_policy: %v", audit["audit_policy"])
	}
	if audit["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", audit["truth_store"])
	}
	if audit["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", audit["retrieval_role"])
	}
	sources, _ := audit["sources"].([]any)
	if len(sources) != 4 {
		t.Fatalf("sources length = %v, want 4", len(sources))
	}
	expected := map[int64]struct {
		authority   bool
		auditStatus string
	}{
		10: {true, "canonical_truth"},
		1:  {false, "support_only"},
		20: {false, "support_only"},
		30: {false, "fallback_support"},
	}
	for i, raw := range sources {
		s, _ := raw.(map[string]any)
		if s == nil {
			t.Fatalf("source[%d] not a map", i)
		}
		id := int64(s["id"].(float64))
		want, ok := expected[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if s["authority"] != want.authority {
			t.Fatalf("source %d authority=%v, want %v", id, s["authority"], want.authority)
		}
		if s["audit_status"] != want.auditStatus {
			t.Fatalf("source %d audit_status=%v, want %v", id, s["audit_status"], want.auditStatus)
		}
	}
	if audit["reason"] != "seq16_p211_source_lookup_audit" {
		t.Fatalf("unexpected reason: %v", audit["reason"])
	}
}

// SEQ-16-P212: runtime toggle — the prepare-turn response must expose a
// runtime_toggle surface that remigrates the legacy
// backend/tests/test_q1e_runtime_toggle.py contract.
func TestSeq16P212RuntimeToggle(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p212", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p212","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	tog, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if tog["version"] != "p212a.v1" {
		t.Fatalf("expected version p212a.v1, got %v", tog["version"])
	}
	if tog["toggle_policy"] != "guarded_shadow_support_only" {
		t.Fatalf("unexpected toggle_policy: %v", tog["toggle_policy"])
	}
	if tog["broad_takeover"] != false {
		t.Fatalf("broad_takeover = %v, want false", tog["broad_takeover"])
	}
	if tog["injection_enabled"] != true {
		t.Fatalf("injection_enabled = %v, want true", tog["injection_enabled"])
	}
	if tog["input_context_enabled"] != true {
		t.Fatalf("input_context_enabled = %v, want true", tog["input_context_enabled"])
	}
	if tog["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", tog["truth_store"])
	}
	if tog["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", tog["retrieval_role"])
	}
	if tog["reason"] != "seq16_p212_runtime_toggle" {
		t.Fatalf("unexpected reason: %v", tog["reason"])
	}
}

// SEQ-16-P213: routing shadow contract — the prepare-turn response must
// expose a recall_result.intent_contract with routing_shadow_replay_gate,
// routing_shadow_budget, routing_shadow_temporal, and routing_shadow_takeover.
func TestSeq16P213RoutingShadowContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p213", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p213", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p213", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p213","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	for _, k := range []string{"routing_shadow_replay_gate", "routing_shadow_budget", "routing_shadow_temporal", "routing_shadow_takeover"} {
		if _, ok := ic[k]; !ok {
			t.Fatalf("intent_contract missing %q", k)
		}
	}
	gate, _ := ic["routing_shadow_replay_gate"].(map[string]any)
	if gate == nil {
		t.Fatalf("routing_shadow_replay_gate not a map")
	}
	if gate["version"] != "u1e.v1" {
		t.Fatalf("gate version=%v, want u1e.v1", gate["version"])
	}
	budget, _ := ic["routing_shadow_budget"].(map[string]any)
	if budget == nil {
		t.Fatalf("routing_shadow_budget not a map")
	}
	if budget["version"] != "t1a.v1" {
		t.Fatalf("budget version=%v, want t1a.v1", budget["version"])
	}
	takeover, _ := ic["routing_shadow_takeover"].(map[string]any)
	if takeover == nil {
		t.Fatalf("routing_shadow_takeover not a map")
	}
	if takeover["version"] != "s1e.v1" {
		t.Fatalf("takeover version=%v, want s1e.v1", takeover["version"])
	}
	if takeover["mode"] != "guarded_default_takeover_only" {
		t.Fatalf("takeover mode=%v, want guarded_default_takeover_only", takeover["mode"])
	}
}

// SEQ-16-P214: intent execution contract — the prepare-turn response must
// expose a recall_result.intent_execution_shadow with status, routing_mode,
// budget_enforcement, and replay_gate.
func TestSeq16P214IntentExecutionContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p214", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p214", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p214", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p214","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	for _, k := range []string{"status", "routing_mode", "budget_enforcement", "replay_gate"} {
		if _, ok := ies[k]; !ok {
			t.Fatalf("intent_execution_shadow missing %q", k)
		}
	}
	if ies["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode=%v, want per_intent_shadow", ies["routing_mode"])
	}
	be, _ := ies["budget_enforcement"].(map[string]any)
	if be == nil {
		t.Fatalf("budget_enforcement not a map")
	}
	if _, ok := be["selected_count_before"]; !ok {
		t.Fatalf("budget_enforcement missing selected_count_before")
	}
}

// SEQ-16-P215: execution trace surface — the prepare-turn response must
// expose a recall_result.trace with q3_multi_intent_router details.
func TestSeq16P215ExecutionTraceSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p215", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p215", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p215", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p215","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	execTrace, ok := ies["trace"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.trace")
	}
	if execTrace["version"] != "s1d.v1" {
		t.Fatalf("execution trace version=%v, want s1d.v1", execTrace["version"])
	}
	if execTrace["mode"] != "shadow_trace_only" {
		t.Fatalf("execution trace mode=%v, want shadow_trace_only", execTrace["mode"])
	}
	summary, ok := execTrace["summary"].(map[string]any)
	if !ok {
		t.Fatalf("execution trace summary missing")
	}
	for _, k := range []string{"executed_intent_count", "input_candidate_count", "selected_count", "suppressed_count", "budget_keep_count", "budget_drop_count"} {
		if _, ok := summary[k]; !ok {
			t.Fatalf("execution trace summary missing %q", k)
		}
	}
	if summary["executed_intent_count"] != float64(4) {
		t.Fatalf("executed_intent_count=%v, want 4", summary["executed_intent_count"])
	}
	if _, ok := execTrace["selection_events"].([]any); !ok {
		t.Fatalf("execution trace selection_events missing or wrong type")
	}
	if _, ok := execTrace["budget_events"].([]any); !ok {
		t.Fatalf("execution trace budget_events missing or wrong type")
	}
	qb, ok := execTrace["query_builder"].(map[string]any)
	if !ok {
		t.Fatalf("execution trace query_builder missing")
	}
	if qb["routing_mode"] != "single_query_shared" {
		t.Fatalf("execution query_builder routing_mode=%v, want single_query_shared", qb["routing_mode"])
	}
	trace, ok := recall["trace"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace")
	}
	q3, ok := trace["q3_multi_intent_router"].(map[string]any)
	if !ok {
		t.Fatalf("missing q3_multi_intent_router")
	}
	for _, k := range []string{"routing_mode", "intent_count", "preview_status", "budget_policy", "matched_intents"} {
		if _, ok := q3[k]; !ok {
			t.Fatalf("q3_multi_intent_router missing %q", k)
		}
	}
	if q3["routing_mode"] != "single_query_shared" {
		t.Fatalf("routing_mode=%v, want single_query_shared", q3["routing_mode"])
	}
}

// SEQ-16-P216: guarded takeover contract — the prepare-turn response must
// expose a recall_result.intent_contract.routing_shadow_takeover that is
// guarded (not broad/default takeover) and only promotes when gate passes.
func TestSeq16P216GuardedTakeoverContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p216", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p216", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p216", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p216","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	takeover, ok := ic["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_takeover")
	}
	if takeover["version"] != "s1e.v1" {
		t.Fatalf("version=%v, want s1e.v1", takeover["version"])
	}
	if takeover["mode"] != "guarded_default_takeover_only" {
		t.Fatalf("mode=%v, want guarded_default_takeover_only", takeover["mode"])
	}
	decision, _ := takeover["decision"].(string)
	if decision != "promote_candidate" && decision != "hold" && decision != "fail_open" {
		t.Fatalf("unexpected decision: %v", decision)
	}
	status, _ := takeover["status"].(string)
	if status != "ready" && status != "pending" && status != "off" {
		t.Fatalf("unexpected status: %v", status)
	}

	if bt, ok := takeover["broad_takeover"]; ok && bt == true {
		t.Fatalf("broad_takeover must not be true in guarded mode")
	}
}

// SEQ-16-P217: temporal scoring contract — the prepare-turn response must
// expose a recall_result.intent_contract.routing_shadow_temporal with
// profile-based temporal scoring flags that do not conflict with P193-P196
// temporal surfaces.
func TestSeq16P217TemporalScoringContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p217", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p217", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p217", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p217","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true},"client_meta":{"context_window_profile":"extreme"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	temporal, ok := ic["routing_shadow_temporal"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_temporal")
	}
	if temporal["version"] != "s1g.v1" {
		t.Fatalf("version=%v, want s1g.v1", temporal["version"])
	}
	if temporal["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("mode=%v, want shadow_temporal_scoring_only", temporal["mode"])
	}
	if temporal["profile"] != "extreme" {
		t.Fatalf("routing_shadow_temporal profile=%v, want extreme", temporal["profile"])
	}
	if temporal["reason"] != "long_profile_temporal_scoring_applied" {
		t.Fatalf("routing_shadow_temporal reason=%v, want long_profile_temporal_scoring_applied", temporal["reason"])
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	ts, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.temporal_scoring")
	}
	if ts["version"] != "s1g.v1" || ts["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("temporal_scoring version/mode = %v/%v, want s1g.v1/shadow_temporal_scoring_only", ts["version"], ts["mode"])
	}
	if ts["profile"] != "extreme" || ts["status"] != "ready" {
		t.Fatalf("temporal_scoring profile/status = %v/%v, want extreme/ready", ts["profile"], ts["status"])
	}
	if _, ok := ts["ann_recency_score"].(map[string]any); !ok {
		t.Fatalf("temporal_scoring missing ann_recency_score")
	}

	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — conflict detected")
	}
	if _, ok := resp["temporal_disambiguation_contract"]; !ok {
		t.Fatalf("P195 temporal_disambiguation_contract missing — conflict detected")
	}
}

// SEQ-16-P218: t1a enforced shadow budget contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_budget (t1a.v1) and
// recall_result.intent_execution_shadow.budget_enforcement with shadow-only
// budget enforcement counters.
func TestSeq16P218T1aEnforcedShadowBudgetContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p218", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p218", TurnIndex: 3, SummaryJSON: `{"text":"opened chest"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p218", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p218", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p218","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	budget, ok := ic["routing_shadow_budget"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_budget")
	}
	if budget["version"] != "t1a.v1" {
		t.Fatalf("routing_shadow_budget version=%v, want t1a.v1", budget["version"])
	}
	if budget["mode"] != "enforced_shadow" {
		t.Fatalf("routing_shadow_budget mode=%v, want enforced_shadow", budget["mode"])
	}
	for _, k := range []string{"selected_count_before", "selected_count_after", "dropped_count", "event_count", "reasons"} {
		if _, ok := budget[k]; !ok {
			t.Fatalf("routing_shadow_budget missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_enforcement")
	}
	if be["version"] != "t1b.v1" {
		t.Fatalf("budget_enforcement version=%v, want t1b.v1", be["version"])
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("budget_enforcement mode=%v, want enforced_shadow", be["mode"])
	}
	for _, k := range []string{"selected_count_before", "selected_count_after", "dropped_count", "event_count", "budget_reasons", "global_cap_chars", "canon_hard_floor"} {
		if _, ok := be[k]; !ok {
			t.Fatalf("budget_enforcement missing %q", k)
		}
	}

	if be["budget_mode"] == "canonical" {
		t.Fatalf("budget_enforcement must not be canonical")
	}
}

// SEQ-16-P219: t1e enforced takeover contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_enforced_takeover (t1e.v1)
// and recall_result.intent_execution_shadow.enforced_takeover with guarded
// default takeover only (no broad takeover).
func TestSeq16P219T1eEnforcedTakeoverContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p219", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p219", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p219", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p219","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	et, ok := ic["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_enforced_takeover")
	}
	if et["version"] != "t1e.v1" {
		t.Fatalf("routing_shadow_enforced_takeover version=%v, want t1e.v1", et["version"])
	}
	if et["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("routing_shadow_enforced_takeover mode=%v, want enforced_default_takeover_only", et["mode"])
	}
	for _, k := range []string{"status", "ready", "promote_candidate", "selected_candidates", "budget_enforcement_mode", "selected_count_after", "reason"} {
		if _, ok := et[k]; !ok {
			t.Fatalf("routing_shadow_enforced_takeover missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	iesET, ok := ies["enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.enforced_takeover")
	}
	if iesET["version"] != "t1e.v1" {
		t.Fatalf("enforced_takeover version=%v, want t1e.v1", iesET["version"])
	}
	if iesET["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("enforced_takeover mode=%v, want enforced_default_takeover_only", iesET["mode"])
	}

	if bt, ok := et["broad_takeover"]; ok && bt == true {
		t.Fatalf("broad_takeover must not be true in enforced takeover")
	}
}

// SEQ-16-P220: u1e replay gate contract — the prepare-turn response must
// expose recall_result.intent_contract.routing_shadow_replay_gate (u1e.v1)
// and recall_result.intent_execution_shadow.replay_gate with session replay
// gate status/decision/reason.
func TestSeq16P220U1eReplayGateContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p220", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p220", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p220", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p220","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_contract")
	}
	rg, ok := ic["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing routing_shadow_replay_gate")
	}
	if rg["version"] != "u1e.v1" {
		t.Fatalf("routing_shadow_replay_gate version=%v, want u1e.v1", rg["version"])
	}
	if rg["mode"] != "captured_session_replay_gate_only" {
		t.Fatalf("routing_shadow_replay_gate mode=%v, want captured_session_replay_gate_only", rg["mode"])
	}
	for _, k := range []string{"status", "decision", "reason"} {
		if _, ok := rg[k]; !ok {
			t.Fatalf("routing_shadow_replay_gate missing %q", k)
		}
	}
	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow")
	}
	iesRG, ok := ies["replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("missing intent_execution_shadow.replay_gate")
	}
	if iesRG["version"] != "u1e.v1" {
		t.Fatalf("replay_gate version=%v, want u1e.v1", iesRG["version"])
	}
	if iesRG["mode"] != "captured_session_replay_gate_only" {
		t.Fatalf("replay_gate mode=%v, want captured_session_replay_gate_only", iesRG["mode"])
	}
	for _, k := range []string{"status", "decision", "reason"} {
		if _, ok := iesRG[k]; !ok {
			t.Fatalf("replay_gate missing %q", k)
		}
	}
}

// SEQ-16-P221: r1d canon-first injection contract — the prepare-turn response must
// seq16DocumentTierIndexes returns the first index where each document tier appears.
func seq16DocumentTierIndexes(docs []any) map[string]int {
	indexes := map[string]int{}
	for i, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tier, _ := doc["tier"].(string)
		if tier == "" {
			continue
		}
		if _, exists := indexes[tier]; !exists {
			indexes[tier] = i
		}
	}
	return indexes
}

// SEQ-16-P221: r1d canon-first injection contract. The prepare-turn response
// must expose injection_pack with canonical_state_hard_floor_enabled and
// canon_text. Supporting memory/evidence retrieval documents must precede
// chat_log fallback, and final truth overwrite must not occur.
func TestSeq16P221R1dCanonFirstInjectionContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p221", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p221", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p221", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 100, ChatSessionID: "seq16-p221", LayerType: "world_state", Content: "The world is stable.", Confidence: 0.9, SourceStateType: "verified"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p221","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack.budget_decisions")
	}
	if bd["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical_state_hard_floor_enabled=%v, want true", bd["canonical_state_hard_floor_enabled"])
	}
	if ip["canon_text"] == nil {
		t.Fatalf("injection_pack.canon_text must not be nil when canonical layers exist")
	}
	canonText, _ := ip["canon_text"].(string)
	if canonText == "" {
		t.Fatalf("injection_pack.canon_text must not be empty")
	}
	if !strings.Contains(canonText, "stable") {
		t.Fatalf("canon_text should contain canonical layer content")
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	docs, ok := recall["documents"].([]any)
	if !ok {
		t.Fatalf("missing documents")
	}
	tierIndexes := seq16DocumentTierIndexes(docs)
	memoryIndex, hasMemory := tierIndexes["memory"]
	evidenceIndex, hasEvidence := tierIndexes["evidence"]
	chatLogIndex, hasChatLog := tierIndexes["chat_log"]
	if !hasMemory {
		t.Fatalf("documents missing memory support tier")
	}
	if !hasEvidence {
		t.Fatalf("documents missing direct evidence tier")
	}
	if !hasChatLog {
		t.Fatalf("documents missing chat_log fallback tier")
	}
	if memoryIndex > chatLogIndex {
		t.Fatalf("memory tier index=%d should precede chat_log fallback index=%d", memoryIndex, chatLogIndex)
	}
	if evidenceIndex > chatLogIndex {
		t.Fatalf("evidence tier index=%d should precede chat_log fallback index=%d", evidenceIndex, chatLogIndex)
	}

	if !strings.Contains(canonText, "The world is stable.") {
		t.Fatalf("canonical truth overwritten or missing in canon_text")
	}
}
