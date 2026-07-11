package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestBuildRecallResultRoutingShadowEnforcedTakeoverOff(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"test-takeover-off","raw_user_input":"query"}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["version"] != "t1e.v1" {
		t.Fatalf("version = %v, want t1e.v1", rsto["version"])
	}
	if rsto["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("mode = %v, want enforced_default_takeover_only", rsto["mode"])
	}
	if rsto["status"] != "off" {
		t.Fatalf("status = %v, want off", rsto["status"])
	}
	if rsto["ready"] != false {
		t.Fatalf("ready = %v, want false", rsto["ready"])
	}
	if rsto["reason"] != "no_candidates" {
		t.Fatalf("reason = %v, want no_candidates", rsto["reason"])
	}
	if rsto["promote_candidate"] != nil {
		t.Fatalf("promote_candidate = %v, want nil", rsto["promote_candidate"])
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "policy_only" {
		t.Fatalf("budget_mode = %v, want policy_only", pbp["budget_mode"])
	}
}

func TestBuildRecallResultRoutingShadowEnforcedTakeoverPending(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-pend", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-pend", Subject: "A", Predicate: "B", Object: "C"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-pend","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rsto["status"])
	}
	if rsto["ready"] != false {
		t.Fatalf("ready = %v, want false", rsto["ready"])
	}
	if rsto["reason"] != "guard_not_ready" {
		t.Fatalf("reason = %v, want guard_not_ready", rsto["reason"])
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "policy_only" {
		t.Fatalf("budget_mode = %v, want policy_only", pbp["budget_mode"])
	}
}

func TestBuildRecallResultRoutingShadowEnforcedTakeoverReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9, PlaceWing: "East", PlaceRoom: "Garden"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ready","turn_index":3,"raw_user_input":"continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	recall := resp["recall_result"].(map[string]any)
	contract := recall["intent_contract"].(map[string]any)
	rsto, ok := contract["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_enforced_takeover missing")
	}
	if rsto["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rsto["status"])
	}
	if rsto["ready"] != true {
		t.Fatalf("ready = %v, want true", rsto["ready"])
	}
	if rsto["reason"] != "routing_shadow_takeover_ready" {
		t.Fatalf("reason = %v, want routing_shadow_takeover_ready", rsto["reason"])
	}
	if rsto["promote_candidate"] == nil {
		t.Fatal("promote_candidate should not be nil")
	}
	pbp, ok := recall["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatal("packet_budget_policy missing")
	}
	if pbp["budget_mode"] != "enforced" {
		t.Fatalf("budget_mode = %v, want enforced", pbp["budget_mode"])
	}
	if bt, ok := pbp["budget_transition"].(map[string]any); ok {
		if bt["to_mode"] != "enforced_shadow" {
			t.Fatalf("budget_transition.to_mode = %v, want enforced_shadow", bt["to_mode"])
		}
		if bt["transition_ready"] != true {
			t.Fatalf("budget_transition.transition_ready = %v, want true", bt["transition_ready"])
		}
	} else {
		t.Fatal("budget_transition missing")
	}
}

func TestS1dTraceContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace, ok := shadow["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace missing")
	}
	if trace["version"] != "s1d.v1" {
		t.Fatalf("version = %v, want s1d.v1", trace["version"])
	}
	if trace["mode"] != "shadow_trace_only" {
		t.Fatalf("mode = %v, want shadow_trace_only", trace["mode"])
	}
	summary := trace["summary"].(map[string]any)
	if summary == nil {
		t.Fatal("summary missing")
	}
	if summary["executed_intent_count"].(int) != 0 {
		t.Fatalf("executed_intent_count = %v, want 0", summary["executed_intent_count"])
	}
	if summary["input_candidate_count"].(int) != 0 {
		t.Fatalf("input_candidate_count = %v, want 0", summary["input_candidate_count"])
	}
	if summary["budget_drop_count"].(int) != 0 {
		t.Fatalf("budget_drop_count = %v, want 0", summary["budget_drop_count"])
	}
}

func TestS1dTraceContractReadyMode(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
		{"tier": "chapter", "document_id": "d3", "text": "foo"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace, ok := shadow["trace"].(map[string]any)
	if !ok {
		t.Fatal("trace missing")
	}
	if trace["version"] != "s1d.v1" {
		t.Fatalf("version = %v, want s1d.v1", trace["version"])
	}
	summary := trace["summary"].(map[string]any)
	if summary["executed_intent_count"].(int) != 4 {
		t.Fatalf("executed_intent_count = %v, want 4", summary["executed_intent_count"])
	}
	if summary["input_candidate_count"].(int) != 3 {
		t.Fatalf("input_candidate_count = %v, want 3", summary["input_candidate_count"])
	}
	selectionEvents, ok := trace["selection_events"].([]map[string]any)
	if !ok || len(selectionEvents) == 0 {
		t.Fatal("selection_events missing or empty")
	}
	budgetEvents, ok := trace["budget_events"].([]map[string]any)
	if !ok || len(budgetEvents) == 0 {
		t.Fatal("budget_events missing or empty")
	}
	qb := trace["query_builder"].(map[string]any)
	if qb["routing_mode"] != "single_query_shared" {
		t.Fatalf("routing_mode = %v, want single_query_shared", qb["routing_mode"])
	}
}

func TestS1dTraceSelectionCountConsistency(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "memory", "document_id": "d2", "text": "b"},
		{"tier": "memory", "document_id": "d3", "text": "c"},
		{"tier": "memory", "document_id": "d4", "text": "d"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	summary := trace["summary"].(map[string]any)
	selectedCount := summary["selected_count"].(int)
	selectionEvents := trace["selection_events"].([]map[string]any)
	selectedInEvents := 0
	for _, ev := range selectionEvents {
		if ev["selected"].(bool) {
			selectedInEvents++
		}
	}
	if selectedInEvents != selectedCount {
		t.Fatalf("selected_in_events = %d, want %d", selectedInEvents, selectedCount)
	}
}

func TestS1dTraceSuppressionConsistency(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "memory", "document_id": "d2", "text": "b"},
		{"tier": "memory", "document_id": "d3", "text": "c"},
		{"tier": "memory", "document_id": "d4", "text": "d"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	summary := trace["summary"].(map[string]any)
	suppressedCount := summary["suppressed_count"].(int)
	selectionEvents := trace["selection_events"].([]map[string]any)
	suppressedInEvents := 0
	for _, ev := range selectionEvents {
		if !ev["selected"].(bool) {
			suppressedInEvents++
		}
	}
	if suppressedInEvents != suppressedCount {
		t.Fatalf("suppressed_in_events = %d, want %d", suppressedInEvents, suppressedCount)
	}
}

func TestS1dTraceNoBehaviorRegression(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	if shadow["version"] != "p29a.v1" {
		t.Fatalf("version = %v, want p29a.v1", shadow["version"])
	}
	be := shadow["budget_enforcement"].(map[string]any)
	if be["version"] != "t1b.v1" {
		t.Fatalf("budget_enforcement version = %v, want t1b.v1", be["version"])
	}
	gt := shadow["guarded_takeover"].(map[string]any)
	if gt["decision"] != "shadow_compare" {
		t.Fatalf("guarded_takeover decision = %v, want shadow_compare", gt["decision"])
	}
	et := shadow["enforced_takeover"].(map[string]any)
	if et["decision"] != "enforced_shadow" {
		t.Fatalf("enforced_takeover decision = %v, want enforced_shadow", et["decision"])
	}
}

func TestT1aShadowBudgetContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	be := shadow["budget_enforcement"].(map[string]any)
	if be["event_count"].(int) != 0 {
		t.Fatalf("event_count = %v, want 0", be["event_count"])
	}
	reasons := be["budget_reasons"].(map[string]int)
	if reasons["no_cap"] != 1 {
		t.Fatalf("no_cap = %v, want 1", reasons["no_cap"])
	}
	if reasons["within_cap"] != 0 {
		t.Fatalf("within_cap = %v, want 0", reasons["within_cap"])
	}
}

func TestT1aShadowBudgetEnforcedReadyMode(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
		{"tier": "episode", "document_id": "d2", "text": "world"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be := shadow["budget_enforcement"].(map[string]any)
	if be["event_count"].(int) != 2 {
		t.Fatalf("event_count = %v, want 2", be["event_count"])
	}
	reasons := be["budget_reasons"].(map[string]int)
	if reasons["within_cap"] != 2 {
		t.Fatalf("within_cap = %v, want 2", reasons["within_cap"])
	}
}

func TestT1aShadowBudgetDropsOverCapCandidates(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": strings.Repeat("a", 2000)},
		{"tier": "memory", "document_id": "d2", "text": strings.Repeat("b", 2000)},
		{"tier": "memory", "document_id": "d3", "text": strings.Repeat("c", 2000)},
		{"tier": "memory", "document_id": "d4", "text": strings.Repeat("d", 2000)},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	trace := shadow["trace"].(map[string]any)
	budgetEvents := trace["budget_events"].([]map[string]any)
	if len(budgetEvents) != 3 {
		t.Fatalf("budget_events len = %v, want 3", len(budgetEvents))
	}
	for _, ev := range budgetEvents {
		if ev["decision"] != "keep" {
			t.Fatalf("decision = %v, want keep", ev["decision"])
		}
		if ev["reason"] != "within_cap" {
			t.Fatalf("reason = %v, want within_cap", ev["reason"])
		}
	}
}

func TestS1gTemporalScoringOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["version"] != "s1g.v1" {
		t.Fatalf("version = %v, want s1g.v1", ts["version"])
	}
	if ts["mode"] != "shadow_temporal_scoring_only" {
		t.Fatalf("mode = %v, want shadow_temporal_scoring_only", ts["mode"])
	}
	if ts["status"] != "off" {
		t.Fatalf("status = %v, want off", ts["status"])
	}
	if ts["reason"] != "profile_not_target" {
		t.Fatalf("reason = %v, want profile_not_target", ts["reason"])
	}
}

func TestS1gTemporalScoringUltraApplies(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "ultra", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "ready" {
		t.Fatalf("status = %v, want ready", ts["status"])
	}
	if ts["reason"] != nil {
		t.Fatalf("reason = %v, want nil", ts["reason"])
	}
	ann, ok := ts["ann_recency_score"].(map[string]any)
	if !ok {
		t.Fatal("ann_recency_score missing")
	}
	if ann["score_source"] != "temporal_proximity" {
		t.Fatalf("score_source = %v, want temporal_proximity", ann["score_source"])
	}
}

func TestS1gTemporalScoringMidPassThrough(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	ts, ok := shadow["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "off" {
		t.Fatalf("status = %v, want off", ts["status"])
	}
	if ts["reason"] != "profile_not_target" {
		t.Fatalf("reason = %v, want profile_not_target", ts["reason"])
	}
}

func TestS1gTemporalScoringRegressionEquivalents(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-temp", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-temp","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"extreme"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_temporal"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_temporal missing")
	}
	if rst["version"] != "s1g.v1" {
		t.Fatalf("version = %v, want s1g.v1", rst["version"])
	}
	if rst["applied_intent_count"].(float64) != 4 {
		t.Fatalf("applied_intent_count = %v, want 4", rst["applied_intent_count"])
	}
	ies := rr["intent_execution_shadow"].(map[string]any)
	ts2, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("intent_execution_shadow.temporal_scoring missing")
	}
	if ts2["status"] != "ready" {
		t.Fatalf("temporal_scoring status = %v, want ready", ts2["status"])
	}
}

func TestU1eReplayGateContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	rg, ok := shadow["replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("replay_gate missing")
	}
	if rg["version"] != "u1e.v1" {
		t.Fatalf("version = %v, want u1e.v1", rg["version"])
	}
	if rg["status"] != "off" {
		t.Fatalf("status = %v, want off", rg["status"])
	}
	if rg["reason"] != "runtime_mode_not_per_intent_shadow" {
		t.Fatalf("reason = %v, want runtime_mode_not_per_intent_shadow", rg["reason"])
	}
}

func TestU1eReplayGatePendingWithoutEvidence(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-pend","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rg["status"])
	}
	if rg["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rg["decision"])
	}
	if rg["reason"] != "without_evidence" {
		t.Fatalf("reason = %v, want without_evidence", rg["reason"])
	}
}

func TestU1eReplayGateReadyWithPassedEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg-ready", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-ready","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rg["status"])
	}
	if rg["decision"] != "promote_candidate" {
		t.Fatalf("decision = %v, want promote_candidate", rg["decision"])
	}
	if rg["reason"] != "passed_evidence" {
		t.Fatalf("reason = %v, want passed_evidence", rg["reason"])
	}
}

func TestU1eReplayGateBlocksShortMidRegression(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg-block", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg-block", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg-block","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"compact"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rg, ok := contract["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_replay_gate missing")
	}
	if rg["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rg["status"])
	}
	ies := rr["intent_execution_shadow"].(map[string]any)
	ts, ok := ies["temporal_scoring"].(map[string]any)
	if !ok {
		t.Fatal("temporal_scoring missing")
	}
	if ts["status"] != "off" {
		t.Fatalf("temporal_scoring status = %v, want off", ts["status"])
	}
}

func TestS1eGuardedTakeoverContractOffMode(t *testing.T) {
	shadow := buildIntentExecutionShadow([]map[string]any{}, map[string]any{}, "long", q3PacketBudgetPolicy())
	rg, _ := shadow["replay_gate"].(map[string]any)
	if rg["status"] != "off" {
		t.Fatalf("replay_gate status = %v, want off", rg["status"])
	}
}

func TestS1eGuardedTakeoverPendingWhenReplayNotReady(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-to-pend", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-pend","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["version"] != "s1e.v1" {
		t.Fatalf("version = %v, want s1e.v1", rst["version"])
	}
	if rst["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rst["status"])
	}
	if rst["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rst["decision"])
	}
	if rst["reason"] != "replay_gate_not_ready" {
		t.Fatalf("reason = %v, want replay_gate_not_ready", rst["reason"])
	}
}

func TestS1eGuardedTakeoverReadyWithReplayGatePass(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-to-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-to-ready", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-ready","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["status"] != "ready" {
		t.Fatalf("status = %v, want ready", rst["status"])
	}
	if rst["decision"] != "promote_candidate" {
		t.Fatalf("decision = %v, want promote_candidate", rst["decision"])
	}
	if rst["reason"] != "guarded_takeover_gate_passed" {
		t.Fatalf("reason = %v, want guarded_takeover_gate_passed", rst["reason"])
	}
}

func TestS1eGuardedTakeoverBlocksWithoutShadowCandidates(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-to-block", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-to-block","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	rr := resp["recall_result"].(map[string]any)
	contract := rr["intent_contract"].(map[string]any)
	rst, ok := contract["routing_shadow_takeover"].(map[string]any)
	if !ok {
		t.Fatal("routing_shadow_takeover missing")
	}
	if rst["status"] != "pending" {
		t.Fatalf("status = %v, want pending", rst["status"])
	}
	if rst["decision"] != "hold" {
		t.Fatalf("decision = %v, want hold", rst["decision"])
	}
	if rst["reason"] != "no_shadow_candidates" {
		t.Fatalf("reason = %v, want no_shadow_candidates", rst["reason"])
	}
}

// TestCompleteTurnConflictResolutionAndRetentionInTrace verifies EA-1h/EA-1i/EA-1l
// policy helpers without relying on a full live provider replay.
func TestCompleteTurnConflictResolutionAndRetentionInTrace(t *testing.T) {
	lineage := parseJSONMap(`{"conflict_class":"hard_contradiction","confidence":0.72,"field_class":"identity","high_impact":true,"importance_tier":"critical"}`)
	decision := directEvidenceConflictResolution(
		store.DirectEvidence{ID: 1, EvidenceKind: "turn_excerpt", EvidenceText: "Alice no longer trusts Bob.", CaptureVerification: "verified"},
		lineage,
		"verified_direct",
		"verified",
		"finalize",
		"critical",
	)
	if decision["policy_version"] != "ea1h.v1" || decision["confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict policy versions mismatch: %#v", decision)
	}
	if decision["classification"] != "hard_contradiction" || decision["route"] != "manual_review" || decision["requires_manual_review"] != true {
		t.Fatalf("conflict decision = %#v, want hard_contradiction/manual_review", decision)
	}
	contract := directEvidenceStateContract()
	if contract["retention_policy_version"] != "ea1l.v1" {
		t.Fatalf("retention policy version = %v, want ea1l.v1", contract["retention_policy_version"])
	}
	windows, ok := contract["retention_windows_turns"].(map[string]any)
	if !ok || windows["direct_evidence"] == nil || windows["previous_archive"] == nil || windows["tombstone"] == nil {
		t.Fatalf("retention windows missing: %#v", contract)
	}
}

// TestCompleteTurnRelationshipV2SurvivesCanonicalPromotion (P452 HS-1g, P502 HS-1j).
// V2 additive fields must survive save and canonical promotion with provenance.
func TestCompleteTurnRelationshipV2SurvivesCanonicalPromotion(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Elena reveals her hidden fear to Kael under moonlight.",
		"importance_score": 8,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Elena trusts Kael deeply after the ritual.",
			"trust":             0.92,
			"confidence":        0.88,
			"identity":          map[string]any{"self_concept": "protector"},
			"core_state":        map[string]any{"affection": 0.9, "tension": 0.2},
			"dynamics":          map[string]any{"power_balance": "equal"},
			"context":           map[string]any{"setting": "moonlit garden"},
			"history":           map[string]any{"first_meeting": "archive hall"},
			"verification":      map[string]any{"source": "critic_v2"},
			"desire":            map[string]any{"stated": "protect Kael"},
			"fear":              map[string]any{"revealed": "losing Kael"},
			"wound":             map[string]any{"old": "betrayed by mentor"},
			"mask":              map[string]any{"public": "aloof scholar"},
			"bond":              map[string]any{"type": "trust"},
			"fixation":          map[string]any{"topic": "ancient seals"},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-v2", 15, extraction, "Elena trusts Kael.", completeTurnEmbeddingConfig{}, time.Unix(1500, 0))
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	if cl.LayerType != "relationship_state" {
		t.Fatalf("layer type = %q, want relationship_state", cl.LayerType)
	}
	if cl.SourceTurn != 15 || cl.LastVerifiedTurn != 15 || cl.Confidence < 0.7 {
		t.Fatalf("provenance not preserved: %#v", cl)
	}

	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "verification", "desire", "fear", "wound", "mask", "bond", "fixation"} {
		if _, ok := content[key]; !ok {
			t.Fatalf("missing v2 field %q in content", key)
		}
	}
	if content["bond_and_distance"] != "Elena trusts Kael deeply after the ritual." {
		t.Fatalf("v1 bond_and_distance lost: %v", content["bond_and_distance"])
	}
}

// TestCompleteTurnRelationshipV1BackfillDefaults (P518 HS-1k).
// Missing v2 sections should receive safe minimal defaults without destructive rewrite.
func TestCompleteTurnRelationshipV1BackfillDefaults(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Old v1 payload without v2 sections.",
		"importance_score": 5,
		"relationship_memory": map[string]any{
			"bond_and_distance": "Mina tolerates Rowan.",
			"trust":             0.6,
			"confidence":        0.85,
			"verification":      "verified",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-v1", 10, extraction, "Mina tolerates Rowan.", completeTurnEmbeddingConfig{}, time.Unix(1000, 0))
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	if content["bond_and_distance"] != "Mina tolerates Rowan." {
		t.Fatalf("v1 bond_and_distance lost: %v", content["bond_and_distance"])
	}
	if content["trust"] != 0.6 {
		t.Fatalf("v1 trust lost: %v", content["trust"])
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "desire", "fear", "wound", "mask", "bond", "fixation"} {
		v, ok := content[key]
		if !ok {
			t.Fatalf("missing v2 default field %q", key)
		}
		m, isMap := v.(map[string]any)
		if !isMap || len(m) != 0 {
			t.Fatalf("v2 default field %q should be empty map, got %v", key, v)
		}
	}
}

// TestCompleteTurnWorldStatePromotesWhenVerified (P469 HS-1h).
// World rules / faction status / region pressure should promote to canonical world_state.
func TestCompleteTurnWorldStatePromotesWhenVerified(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "The Northern faction gains influence after the treaty.",
		"importance_score": 7,
		"world_rules": []any{
			map[string]any{"key": "faction_north", "value": "ascendant", "scope": "global"},
			map[string]any{"key": "region_pressure", "value": "high", "scope": "borderlands"},
		},
		"faction_status":    map[string]any{"north": "rising", "south": "stable"},
		"region_pressure":   map[string]any{"borderlands": 0.8},
		"offscreen_threads": []any{map[string]any{"title": "spy network", "status": "active"}},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world", 20, extraction, "Northern faction ascendant.", completeTurnEmbeddingConfig{}, time.Unix(2000, 0))
	if result.ActiveStates != 1 {
		t.Fatalf("active states saved = %d, want 1", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("expected 1 canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
	cl := fake.savedCanonicalLayers[0]
	if cl.LayerType != "world_state" {
		t.Fatalf("layer type = %q, want world_state", cl.LayerType)
	}
	if cl.SourceTurn != 20 || cl.LastVerifiedTurn != 20 || cl.Confidence < 0.7 {
		t.Fatalf("provenance not preserved: %#v", cl)
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(cl.Content), &content); err != nil {
		t.Fatalf("content decode: %v", err)
	}
	rules, ok := content["rules"].([]any)
	if !ok || len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %v", content["rules"])
	}
	if content["version"] != "world_state.v1" {
		t.Fatalf("version mismatch: %v", content["version"])
	}
	if _, ok := content["faction_status"]; !ok {
		t.Fatalf("faction_status missing")
	}
	if _, ok := content["region_pressure"]; !ok {
		t.Fatalf("region_pressure missing")
	}
	if _, ok := content["offscreen_threads"]; !ok {
		t.Fatalf("offscreen_threads missing")
	}
}

func TestCompleteTurnWorldStateRulesAlsoSaveWorldRules(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "The heated beam can crack under river ice shock.",
		"importance_score": 7,
		"world_state": map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.9,
			"verification": "verified",
			"rules": []any{
				map[string]any{"category": "setting", "key": "ice_wedge_effect", "scope": "session", "scope_name": "Demolition Logic", "value": "Superheated steel can fracture when shocked with freezing river water."},
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world-rules", 22, extraction, "The team confirms the ice wedge demolition logic.", completeTurnEmbeddingConfig{}, time.Unix(2200, 0))
	if result.WorldRules != 1 || len(fake.savedWorldRules) != 1 {
		t.Fatalf("world_state.rules should save 1 world rule, got result=%d store=%d", result.WorldRules, len(fake.savedWorldRules))
	}
	saved := fake.savedWorldRules[0]
	if saved.Key != "ice_wedge_effect" || saved.Category != "setting" || saved.ScopeName != "Demolition Logic" {
		t.Fatalf("world rule fields not preserved: %+v", saved)
	}
	if result.CanonicalStateLayers != 1 || len(fake.savedCanonicalLayers) != 1 {
		t.Fatalf("world_state should still promote to canonical layer, got result=%d store=%d", result.CanonicalStateLayers, len(fake.savedCanonicalLayers))
	}
}

// TestCompleteTurnWorldStateLowConfidenceBlocked (P469 HS-1h filtering).
// Low-confidence or unverified world state must not promote to canonical.
func TestCompleteTurnWorldStateLowConfidenceBlocked(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Rumors about world changes remain unverified.",
		"importance_score": 4,
		"world_rules": []any{
			map[string]any{"key": "rumor", "value": "maybe true", "scope": "global"},
		},
		"world_state": map[string]any{
			"confidence":   0.4,
			"verification": "pending",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-world-low", 21, extraction, "Rumors.", completeTurnEmbeddingConfig{}, time.Unix(2100, 0))
	if result.ActiveStates != 1 {
		t.Fatalf("active states saved = %d, want 1", result.ActiveStates)
	}
	if result.CanonicalStateLayers != 0 || len(fake.savedCanonicalLayers) != 0 {
		t.Fatalf("low-confidence/unverified world state should not promote, got result=%d layers=%#v", result.CanonicalStateLayers, fake.savedCanonicalLayers)
	}
}

func TestCompleteTurnPhysicalConditionSavedWithoutHardcodedDuration(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina fell from the stairs and fractured her left arm.",
		"importance_score":  8,
		"evidence_excerpts": []any{"Mina fell from the stairs and fractured her left arm."},
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Mina", "entity_type": "character"}},
		},
		"physical_conditions": []any{
			map[string]any{
				"owner_entity_key":          "Mina",
				"condition_label":           "fractured left arm",
				"effect_kind":               "injury",
				"severity_text":             "painful and movement-limiting",
				"body_area":                 "left arm",
				"duration_policy":           "unknown_until_updated",
				"age_or_vulnerability_note": "age not specified; do not infer healing speed",
				"evidence_excerpt":          "Mina fell from the stairs and fractured her left arm.",
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-physical-condition", 23, extraction, "Mina fell from the stairs and fractured her left arm.", completeTurnEmbeddingConfig{}, time.Unix(2300, 0))
	if result.PhysicalConditions != 1 || result.StatusEffects != 1 {
		t.Fatalf("physical condition counters mismatch: PhysicalConditions=%d StatusEffects=%d errors=%v", result.PhysicalConditions, result.StatusEffects, result.ErrorDetails)
	}
	if result.StatusSchemaDefinitions != 1 || len(fake.savedStatusDefinitions) != 1 {
		t.Fatalf("expected one generic physical_condition schema, result=%d saved=%d", result.StatusSchemaDefinitions, len(fake.savedStatusDefinitions))
	}
	definition := fake.savedStatusDefinitions[0]
	if definition.StatusKey != physicalConditionStatusKey || definition.OwnerScope != "character" || definition.ValueKind != "note" {
		t.Fatalf("unexpected physical condition definition: %+v", definition)
	}
	if !strings.Contains(definition.OptionsJSON, "evidence_bound_no_default_duration") {
		t.Fatalf("schema options must require evidence-bound duration, got %s", definition.OptionsJSON)
	}
	if len(fake.savedStatusEffects) != 1 {
		t.Fatalf("expected one saved status effect, got %d", len(fake.savedStatusEffects))
	}
	effect := fake.savedStatusEffects[0]
	if effect.OwnerID != "Mina" || effect.StatusKey != physicalConditionStatusKey || effect.EffectKind != "injury" || effect.EffectLabel != "fractured left arm" {
		t.Fatalf("unexpected saved physical condition effect: %+v", effect)
	}
	if !strings.Contains(effect.EvidenceJSON, "Mina fell from the stairs and fractured her left arm.") {
		t.Fatalf("evidence excerpt missing from status effect: %s", effect.EvidenceJSON)
	}
	if !strings.Contains(effect.DurationJSON, "unknown_until_updated") || !strings.Contains(effect.DurationJSON, `"hardcoded_duration":false`) {
		t.Fatalf("duration should stay unknown until later evidence updates it, got %s", effect.DurationJSON)
	}
	lowerDuration := strings.ToLower(effect.DurationJSON)
	for _, forbidden := range []string{"day", "days", "month", "months", "3", "30"} {
		if strings.Contains(lowerDuration, forbidden) {
			t.Fatalf("duration contains hardcoded calendar value %q: %s", forbidden, effect.DurationJSON)
		}
	}
	if !strings.Contains(effect.EffectPayloadJSON, `"numeric_severity_required":false`) {
		t.Fatalf("payload should not require numeric severity, got %s", effect.EffectPayloadJSON)
	}
}

func TestCompleteTurnEntityConditionSavedAndAttachedToEntity(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "The sacred sword broke during the duel.",
		"importance_score":  8,
		"evidence_excerpts": []any{"The sacred sword broke during the duel."},
		"entities": map[string]any{
			"items": []any{map[string]any{"name": "Sacred Sword", "entity_type": "item", "description": "a legendary blade"}},
		},
		"entity_conditions": []any{
			map[string]any{
				"owner_entity_key":  "Sacred Sword",
				"owner_entity_type": "item",
				"condition_label":   "blade broken",
				"effect_kind":       "debuff",
				"evidence_excerpt":  "The sacred sword broke during the duel.",
				"duration_policy":   "unknown_until_updated",
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-entity-condition", 24, extraction, "The sacred sword broke during the duel.", completeTurnEmbeddingConfig{}, time.Unix(2400, 0))
	if result.EntityConditions != 1 || result.StatusEffects != 1 {
		t.Fatalf("entity condition counters mismatch: EntityConditions=%d StatusEffects=%d errors=%v", result.EntityConditions, result.StatusEffects, result.ErrorDetails)
	}
	if result.StatusSchemaDefinitions != 1 || len(fake.savedStatusDefinitions) != 1 {
		t.Fatalf("expected one generic entity_condition schema, result=%d saved=%d", result.StatusSchemaDefinitions, len(fake.savedStatusDefinitions))
	}
	definition := fake.savedStatusDefinitions[0]
	if definition.StatusKey != entityConditionStatusKey || definition.OwnerScope != "entity" || definition.ValueKind != "note" {
		t.Fatalf("unexpected entity condition definition: %+v", definition)
	}
	if len(fake.savedEntities) != 1 {
		t.Fatalf("expected one saved item entity, got %d", len(fake.savedEntities))
	}
	if !strings.Contains(fake.savedEntities[0].Description, "blade broken") {
		t.Fatalf("entity description should surface item condition, got %q", fake.savedEntities[0].Description)
	}
	if len(fake.savedStatusEffects) != 1 {
		t.Fatalf("expected one saved status effect, got %d", len(fake.savedStatusEffects))
	}
	effect := fake.savedStatusEffects[0]
	if effect.OwnerScope != "entity" || effect.OwnerID != "Sacred Sword" || effect.StatusKey != entityConditionStatusKey || effect.EffectKind != "debuff" || effect.EffectLabel != "blade broken" {
		t.Fatalf("unexpected saved entity condition effect: %+v", effect)
	}
	if !strings.Contains(effect.EffectPayloadJSON, `"entity_type":"item"`) {
		t.Fatalf("payload should preserve entity type, got %s", effect.EffectPayloadJSON)
	}
	if !strings.Contains(effect.DurationJSON, "unknown_until_updated") || !strings.Contains(effect.DurationJSON, `"hardcoded_duration":false`) {
		t.Fatalf("duration should stay unknown until later evidence updates it, got %s", effect.DurationJSON)
	}
}
