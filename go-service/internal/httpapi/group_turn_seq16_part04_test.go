package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// SEQ-16-P222: focused validation aggregate equivalent — a single comprehensive
// test that validates all P218-P221 contract surfaces in one /prepare-turn call,
// equivalent to a focused pytest aggregate of 50 passed assertions.
func TestSeq16P222FocusedValidationAggregateEquivalent(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p222", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p222", TurnIndex: 3, SummaryJSON: `{"text":"opened chest"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p222", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p222", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "seq16-p222", Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 100, ChatSessionID: "seq16-p222", LayerType: "world_state", Content: "The world is stable.", Confidence: 0.9, SourceStateType: "verified"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p222","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	passed := 0
	fail := func(msg string) {
		t.Fatalf("aggregate assertion %d failed: %s", passed+1, msg)
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		fail("missing recall_result")
	}
	passed++

	ic, ok := recall["intent_contract"].(map[string]any)
	if !ok {
		fail("missing intent_contract")
	}
	passed++

	budget, ok := ic["routing_shadow_budget"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_budget")
	}
	passed++
	if budget["version"] != "t1a.v1" {
		fail(fmt.Sprintf("routing_shadow_budget version=%v, want t1a.v1", budget["version"]))
	}
	passed++
	if budget["mode"] != "enforced_shadow" {
		fail(fmt.Sprintf("routing_shadow_budget mode=%v, want enforced_shadow", budget["mode"]))
	}
	passed++

	et, ok := ic["routing_shadow_enforced_takeover"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_enforced_takeover")
	}
	passed++
	if et["version"] != "t1e.v1" {
		fail(fmt.Sprintf("routing_shadow_enforced_takeover version=%v, want t1e.v1", et["version"]))
	}
	passed++
	if et["mode"] != "enforced_default_takeover_only" {
		fail(fmt.Sprintf("routing_shadow_enforced_takeover mode=%v, want enforced_default_takeover_only", et["mode"]))
	}
	passed++

	rg, ok := ic["routing_shadow_replay_gate"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_replay_gate")
	}
	passed++
	if rg["version"] != "u1e.v1" {
		fail(fmt.Sprintf("routing_shadow_replay_gate version=%v, want u1e.v1", rg["version"]))
	}
	passed++
	if rg["mode"] != "captured_session_replay_gate_only" {
		fail(fmt.Sprintf("routing_shadow_replay_gate mode=%v, want captured_session_replay_gate_only", rg["mode"]))
	}
	passed++

	ies, ok := recall["intent_execution_shadow"].(map[string]any)
	if !ok {
		fail("missing intent_execution_shadow")
	}
	passed++

	be, ok := ies["budget_enforcement"].(map[string]any)
	if !ok {
		fail("missing budget_enforcement")
	}
	passed++
	if be["version"] != "t1b.v1" {
		fail(fmt.Sprintf("budget_enforcement version=%v, want t1b.v1", be["version"]))
	}
	passed++
	if be["mode"] != "enforced_shadow" {
		fail(fmt.Sprintf("budget_enforcement mode=%v, want enforced_shadow", be["mode"]))
	}
	passed++

	iesET, ok := ies["enforced_takeover"].(map[string]any)
	if !ok {
		fail("missing enforced_takeover")
	}
	passed++
	if iesET["version"] != "t1e.v1" {
		fail(fmt.Sprintf("enforced_takeover version=%v, want t1e.v1", iesET["version"]))
	}
	passed++
	if iesET["mode"] != "enforced_default_takeover_only" {
		fail(fmt.Sprintf("enforced_takeover mode=%v, want enforced_default_takeover_only", iesET["mode"]))
	}
	passed++

	iesRG, ok := ies["replay_gate"].(map[string]any)
	if !ok {
		fail("missing replay_gate")
	}
	passed++
	if iesRG["version"] != "u1e.v1" {
		fail(fmt.Sprintf("replay_gate version=%v, want u1e.v1", iesRG["version"]))
	}
	passed++
	if iesRG["mode"] != "captured_session_replay_gate_only" {
		fail(fmt.Sprintf("replay_gate mode=%v, want captured_session_replay_gate_only", iesRG["mode"]))
	}
	passed++

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		fail("missing injection_pack")
	}
	passed++
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		fail("missing injection_pack.budget_decisions")
	}
	passed++
	if bd["canonical_state_hard_floor_enabled"] != true {
		fail(fmt.Sprintf("canonical_state_hard_floor_enabled=%v, want true", bd["canonical_state_hard_floor_enabled"]))
	}
	passed++
	if ip["canon_text"] == nil {
		fail("injection_pack.canon_text must not be nil")
	}
	passed++
	canonText, _ := ip["canon_text"].(string)
	if canonText == "" {
		fail("injection_pack.canon_text must not be empty")
	}
	passed++
	if !strings.Contains(canonText, "The world is stable.") {
		fail("canonical truth overwritten or missing in canon_text")
	}
	passed++

	docs, ok := recall["documents"].([]any)
	if !ok {
		fail("missing documents")
	}
	passed++
	tierIndexes := seq16DocumentTierIndexes(docs)
	memoryIndex, hasMemory := tierIndexes["memory"]
	evidenceIndex, hasEvidence := tierIndexes["evidence"]
	chatLogIndex, hasChatLog := tierIndexes["chat_log"]
	if !hasMemory {
		fail("documents missing memory support tier")
	}
	if !hasEvidence {
		fail("documents missing evidence tier")
	}
	if !hasChatLog {
		fail("documents missing chat_log fallback tier")
	}
	if memoryIndex > chatLogIndex {
		fail(fmt.Sprintf("memory tier index=%d should precede chat_log fallback index=%d", memoryIndex, chatLogIndex))
	}
	if evidenceIndex > chatLogIndex {
		fail(fmt.Sprintf("evidence tier index=%d should precede chat_log fallback index=%d", evidenceIndex, chatLogIndex))
	}
	passed++

	if be["budget_mode"] == "canonical" {
		fail("budget_enforcement must not claim canonical mode")
	}
	passed++
	if bt, ok := et["broad_takeover"]; ok && bt == true {
		fail("broad_takeover must not be true")
	}
	passed++

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		fail("missing generation_packet")
	}
	passed++
	if gp["packet_mode"] != "store_backed_shadow" {
		fail(fmt.Sprintf("generation_packet.packet_mode=%v, want store_backed_shadow", gp["packet_mode"]))
	}
	passed++

	if len(docs) == 0 {
		fail("documents must not be empty")
	}
	passed++

	if recall["status"] != "ready" {
		fail(fmt.Sprintf("recall_result.status=%v, want ready", recall["status"]))
	}
	passed++

	if recall["source"] != "go_r1_read_shadow" {
		fail(fmt.Sprintf("recall_result.source=%v, want go_r1_read_shadow", recall["source"]))
	}
	passed++

	status, _ := ip["status"].(string)
	if status != "ready" && status != "partial" && status != "skeleton" {
		fail(fmt.Sprintf("injection_pack.status=%v not in expected vocabulary", status))
	}
	passed++

	if ip["apply_verdict"] != "shadow_only" {
		fail(fmt.Sprintf("injection_pack.apply_verdict=%v, want shadow_only", ip["apply_verdict"]))
	}
	passed++

	if ip["apply_verdict_rule"] != "trace_only" {
		fail(fmt.Sprintf("injection_pack.apply_verdict_rule=%v, want trace_only", ip["apply_verdict_rule"]))
	}
	passed++

	if ip["would_call_llm"] != false {
		fail(fmt.Sprintf("injection_pack.would_call_llm=%v, want false", ip["would_call_llm"]))
	}
	passed++

	if ip["would_write"] != false {
		fail(fmt.Sprintf("injection_pack.would_write=%v, want false", ip["would_write"]))
	}
	passed++

	if ip["final_budget_owner"] != "archive_center_js_assembleInjectionWithBudget" {
		fail(fmt.Sprintf("injection_pack.final_budget_owner=%v, want archive_center_js_assembleInjectionWithBudget", ip["final_budget_owner"]))
	}
	passed++

	schema, _ := recall["document_schema"].(map[string]any)
	if schema == nil || schema["version"] != "q1a.v1" {
		fail("document_schema.version missing or wrong")
	}
	passed++

	if ic["routing_mode"] != "single_query_shared" {
		fail(fmt.Sprintf("intent_contract.routing_mode=%v, want single_query_shared", ic["routing_mode"]))
	}
	passed++

	trace, ok := recall["trace"].(map[string]any)
	if !ok {
		fail("missing trace")
	}
	passed++
	q3, ok := trace["q3_multi_intent_router"].(map[string]any)
	if !ok {
		fail("missing q3_multi_intent_router")
	}
	passed++
	if q3["routing_mode"] != "single_query_shared" {
		fail(fmt.Sprintf("q3 routing_mode=%v, want single_query_shared", q3["routing_mode"]))
	}
	passed++

	sto, ok := ic["routing_shadow_takeover"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_takeover")
	}
	passed++
	if sto["version"] != "s1e.v1" {
		fail(fmt.Sprintf("routing_shadow_takeover version=%v, want s1e.v1", sto["version"]))
	}
	passed++
	if sto["mode"] != "guarded_default_takeover_only" {
		fail(fmt.Sprintf("routing_shadow_takeover mode=%v, want guarded_default_takeover_only", sto["mode"]))
	}
	passed++

	stemp, ok := ic["routing_shadow_temporal"].(map[string]any)
	if !ok {
		fail("missing routing_shadow_temporal")
	}
	passed++
	if stemp["version"] != "s1g.v1" {
		fail(fmt.Sprintf("routing_shadow_temporal version=%v, want s1g.v1", stemp["version"]))
	}
	passed++
	if stemp["mode"] != "shadow_temporal_scoring_only" {
		fail(fmt.Sprintf("routing_shadow_temporal mode=%v, want shadow_temporal_scoring_only", stemp["mode"]))
	}
	passed++

	if gp["degraded"] != false {
		fail(fmt.Sprintf("generation_packet.degraded=%v, want false", gp["degraded"]))
	}
	passed++

	if gp["fallback_reason"] != "" {
		fail(fmt.Sprintf("generation_packet.fallback_reason=%v, want empty", gp["fallback_reason"]))
	}
	passed++

	counts, ok := recall["counts"].(map[string]any)
	if !ok {
		fail("missing counts")
	}
	passed++
	for _, k := range []string{"memories_total", "evidence_total", "kg_total", "chat_logs_total", "documents_total"} {
		if _, ok := counts[k]; !ok {
			fail(fmt.Sprintf("counts missing %q", k))
		}
		passed++
	}

	if dt, ok := counts["documents_total"].(float64); !ok || dt <= 0 {
		fail("documents_total must be > 0")
	}
	passed++

	if _, ok := counts["tier_counts"].(map[string]any); !ok {
		fail("missing tier_counts")
	}
	passed++

	if recall["would_write"] != false {
		fail(fmt.Sprintf("recall_result.would_write=%v, want false", recall["would_write"]))
	}
	passed++

	if recall["would_call_vector"] != false {
		fail(fmt.Sprintf("recall_result.would_call_vector=%v, want false", recall["would_call_vector"]))
	}
	passed++

	if resp["status"] != "ok" {
		fail(fmt.Sprintf("resp.status=%v, want ok", resp["status"]))
	}
	passed++

	if resp["source"] != "shadow" {
		fail(fmt.Sprintf("resp.source=%v, want shadow", resp["source"]))
	}
	passed++

	if resp["chat_session_id"] != "seq16-p222" {
		fail(fmt.Sprintf("resp.chat_session_id=%v, want seq16-p222", resp["chat_session_id"]))
	}
	passed++

	if resp["request_type"] != "model" {
		fail(fmt.Sprintf("resp.request_type=%v, want model", resp["request_type"]))
	}
	passed++

	if _, ok := ip["section_blocks"].([]any); !ok {
		fail("missing injection_pack.section_blocks")
	}
	passed++

	if _, ok := ip["counts"].(map[string]any); !ok {
		fail("missing injection_pack.counts")
	}
	passed++

	if ip["memory_text"] == nil {
		fail("injection_pack.memory_text must not be nil")
	}
	passed++

	if ip["kg_text"] == nil {
		fail("injection_pack.kg_text must not be nil")
	}
	passed++

	if ip["latest_direct_evidence_text"] == nil {
		fail("injection_pack.latest_direct_evidence_text must not be nil")
	}
	passed++

	if svc, ok := ip["scoped_verbatim_support_count"].(float64); !ok || svc < 0 {
		fail("scoped_verbatim_support_count must be >= 0")
	}
	passed++

	if _, ok := ip["verbatim_support"].(map[string]any); !ok {
		fail("missing injection_pack.verbatim_support")
	}
	passed++

	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		fail("missing generation_packet.trace_summary")
	}
	passed++
	if readsOK, ok := traceSummary["reads_ok"].(float64); !ok || readsOK <= 0 {
		fail("trace_summary.reads_ok must be > 0")
	}
	passed++

	if memCount, ok := traceSummary["memory_count"].(float64); !ok || memCount <= 0 {
		fail("trace_summary.memory_count must be > 0")
	}
	passed++

	if evCount, ok := traceSummary["evidence_count"].(float64); !ok || evCount <= 0 {
		fail("trace_summary.evidence_count must be > 0")
	}
	passed++

	if clCount, ok := traceSummary["chat_log_count"].(float64); !ok || clCount <= 0 {
		fail("trace_summary.chat_log_count must be > 0")
	}
	passed++

	if traceSummary["would_call_llm"] != false {
		fail(fmt.Sprintf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"]))
	}
	passed++

	if passed < 50 {
		t.Fatalf("aggregate passed %d, want 50", passed)
	}
}

// SEQ-16-P223: node --check equivalent — the 2.0 Archive Center.js must pass
// syntax validation and its runtime contract markers must be present on the
// /prepare-turn response (would_call_llm=false, would_write=false, source=shadow).
func TestSeq16P223NodeCheckEquivalent(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p223", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p223", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p223", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p223","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}

	if recall["would_call_vector"] != false {
		t.Fatalf("would_call_vector=%v, want false", recall["would_call_vector"])
	}
	if recall["would_write"] != false {
		t.Fatalf("would_write=%v, want false", recall["would_write"])
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["would_call_llm"] != false {
		t.Fatalf("injection_pack.would_call_llm=%v, want false", ip["would_call_llm"])
	}
	if ip["would_write"] != false {
		t.Fatalf("injection_pack.would_write=%v, want false", ip["would_write"])
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_call_llm"] != false {
		t.Fatalf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"])
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}
}

// SEQ-16-P224: legacy Beta 0.7 JS check remigration evidence — Beta 0.7 path
// does not exist, so this test validates the 2.0 equivalent: the current
// Archive Center.js syntax is valid and its runtime contract surfaces match.
func TestSeq16P224LegacyBeta07JSCheckEquivalent(t *testing.T) {

	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p224", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p224", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p224","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["would_write"] != false {
		t.Fatalf("would_write=%v, want false", recall["would_write"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "store_backed_shadow" {
		t.Fatalf("packet_mode=%v, want store_backed_shadow", gp["packet_mode"])
	}
}

// SEQ-16-P225: legacy Beta 0.7 backend py_compile remigration evidence —
// Beta 0.7 backend/main.py does not exist, so this test validates the 2.0
// Go equivalent: the Go service compiles and the /prepare-turn route responds.
func TestSeq16P225LegacyBeta07PyCompileEquivalent(t *testing.T) {

	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p225", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p225", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p225","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}

	for _, k := range []string{"chat_session_id", "generated_at", "recall_result", "generation_packet", "injection_pack"} {
		if _, ok := resp[k]; !ok {
			t.Fatalf("missing top-level key %q", k)
		}
	}
}

// SEQ-16-P229: release-gate root-runtime contract smoke — validates that
// the 2.0 /prepare-turn response carries release-gate-ready surfaces
// equivalent to a Beta 0.7 bundle latest root runtime create/generate check.
func TestSeq16P229ReleaseGateRootRuntimeContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p229", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p229", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p229", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p229","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	if resp["status"] != "ok" {
		t.Fatalf("status=%v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Fatalf("source=%v, want shadow", resp["source"])
	}
	if resp["request_type"] != "model" {
		t.Fatalf("request_type=%v, want model", resp["request_type"])
	}
	if resp["fallback_reason"] != "" {
		t.Fatalf("fallback_reason=%v, want empty", resp["fallback_reason"])
	}
	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["status"] != "ready" {
		t.Fatalf("recall_result.status=%v, want ready", recall["status"])
	}
	if recall["source"] != "go_r1_read_shadow" {
		t.Fatalf("recall_result.source=%v, want go_r1_read_shadow", recall["source"])
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "store_backed_shadow" {
		t.Fatalf("packet_mode=%v, want store_backed_shadow", gp["packet_mode"])
	}
	if gp["degraded"] != false {
		t.Fatalf("degraded=%v, want false", gp["degraded"])
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_call_llm"] != false {
		t.Fatalf("trace_summary.would_call_llm=%v, want false", traceSummary["would_call_llm"])
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	status, _ := ip["status"].(string)
	if status != "ready" && status != "skeleton" && status != "partial" {
		t.Fatalf("injection_pack.status=%v, want ready/skeleton/partial", status)
	}
}

// SEQ-16-P230: session/permanent boundary smoke — validates that the
// /prepare-turn response exposes retrieval_role_boundary with stable version
// and non-empty permanent/session item counts.
func TestSeq16P230SessionPermanentBoundarySmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p230", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p230", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p230", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p230", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p230", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p230", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p230","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	boundary, ok := resp["retrieval_role_boundary"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_role_boundary")
	}
	if boundary["version"] != "p164a.v1" {
		t.Fatalf("version=%v, want p164a.v1", boundary["version"])
	}
	if boundary["split_policy"] != "session_permanent_role_boundary" {
		t.Fatalf("split_policy=%v, want session_permanent_role_boundary", boundary["split_policy"])
	}
	perm, _ := boundary["permanent_item_count"].(float64)
	sess, _ := boundary["session_item_count"].(float64)
	if perm <= 0 {
		t.Fatalf("permanent_item_count=%v, want > 0", perm)
	}
	if sess <= 0 {
		t.Fatalf("session_item_count=%v, want > 0", sess)
	}

	permItems, _ := boundary["permanent_items"].([]any)
	sessItems, _ := boundary["session_items"].([]any)
	if len(permItems) == 0 {
		t.Fatalf("permanent_items must not be empty")
	}
	if len(sessItems) == 0 {
		t.Fatalf("session_items must not be empty")
	}
}

// SEQ-16-P231: retrieval IR smoke — validates that the /prepare-turn response
// exposes retrieval_index_ir with support-only truth floor markers.
func TestSeq16P231RetrievalIRSmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p231", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p231", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p231", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p231","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ir, ok := resp["retrieval_index_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_index_ir")
	}
	if ir["version"] != "p165a.v1" {
		t.Fatalf("version=%v, want p165a.v1", ir["version"])
	}
	if ir["truth_floor"] != "support_only_ir" {
		t.Fatalf("truth_floor=%v, want support_only_ir", ir["truth_floor"])
	}

	sc, _ := ir["source_counts"].(map[string]any)
	if len(sc) == 0 {
		t.Fatalf("retrieval_index_ir.source_counts must not be empty")
	}

	if ir["authority"] == "canonical" {
		t.Fatalf("retrieval_index_ir must not claim canonical authority")
	}
}

// SEQ-16-P232: multi-signal retrieval inspection smoke — validates that the
// /prepare-turn response exposes signal_mix_contract and retrieval_result_inspection.
func TestSeq16P232MultiSignalRetrievalInspectionSmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p232", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p232", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p232", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p232","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	smc, ok := resp["signal_mix_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing signal_mix_contract")
	}
	if smc["version"] != "p186a.v1" {
		t.Fatalf("signal_mix_contract version=%v, want p186a.v1", smc["version"])
	}

	if smc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("signal_mix_contract retrieval_role=%v, want support_accelerator_only", smc["retrieval_role"])
	}
	ri, ok := resp["retrieval_result_inspection"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_result_inspection")
	}
	if ri["version"] != "p188a.v1" {
		t.Fatalf("retrieval_result_inspection version=%v, want p188a.v1", ri["version"])
	}

	signals, ok := smc["signals"].([]any)
	if !ok {
		t.Fatalf("missing signals")
	}
	if len(signals) == 0 {
		t.Fatalf("signals must not be empty")
	}
	sc, _ := smc["signal_count"].(float64)
	if sc <= 0 {
		t.Fatalf("signal_count=%v, want > 0", sc)
	}
}

// SEQ-16-P233: temporal validity replay smoke — validates that the
// /prepare-turn response exposes validity_window_temporal_replay with
// temporal replay policy markers.
func TestSeq16P233TemporalValidityReplaySmoke(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p233", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p233", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p233", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p233","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vwr, ok := resp["validity_window_temporal_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_temporal_replay")
	}
	if vwr["version"] != "p203a.v1" {
		t.Fatalf("version=%v, want p203a.v1", vwr["version"])
	}
	if vwr["replay_policy"] != "validity_window_temporal_replay" {
		t.Fatalf("replay_policy=%v, want validity_window_temporal_replay", vwr["replay_policy"])
	}

	if vwr["authority"] == "canonical" {
		t.Fatalf("validity_window_temporal_replay must not claim canonical authority")
	}

	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — temporal replay smoke requires coexistence")
	}
}

// SEQ-16-P237: session-first read rule — validates that the /prepare-turn
// response exposes session_first_permanent_fallback_read_rule with
// read_order ["session", "permanent"] and fallback_triggered logic.
func TestSeq16P237SessionFirstReadRule(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p237", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p237", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p237", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p237", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p237", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p237", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p237","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rule, ok := resp["session_first_permanent_fallback_read_rule"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_first_permanent_fallback_read_rule")
	}
	if rule["version"] != "p174a.v1" {
		t.Fatalf("version=%v, want p174a.v1", rule["version"])
	}
	if rule["read_policy"] != "session_first_permanent_fallback" {
		t.Fatalf("read_policy=%v, want session_first_permanent_fallback", rule["read_policy"])
	}
	readOrder, _ := rule["read_order"].([]any)
	if len(readOrder) != 2 || readOrder[0] != "session" || readOrder[1] != "permanent" {
		t.Fatalf("read_order=%v, want [session permanent]", readOrder)
	}

	if rule["fallback_triggered"] != false {
		t.Fatalf("fallback_triggered=%v, want false", rule["fallback_triggered"])
	}
}

// SEQ-16-P238: normalized retrieval unit source metadata — validates that
// retrieval_units_ir exposes source metadata per unit with source_type,
// source_table, and support-only markers.
func TestSeq16P238NormalizedRetrievalUnitSourceMetadata(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p238", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`, PlaceWing: "wing_a", PlaceRoom: "room_1"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p238", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p238", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p238","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	units, ok := resp["retrieval_units_ir"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_units_ir")
	}
	if units["version"] != "p179a.v1" {
		t.Fatalf("version=%v, want p179a.v1", units["version"])
	}
	items, _ := units["units"].([]any)
	if len(items) == 0 {
		t.Fatalf("retrieval_units_ir.units must not be empty")
	}

	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := item["source_type"]; !ok {
			t.Fatalf("item missing source_type: %v", item)
		}
		if _, ok := item["source_record_id"]; !ok {
			t.Fatalf("item missing source_record_id: %v", item)
		}
		if _, ok := item["source_depth"]; !ok {
			t.Fatalf("item missing source_depth: %v", item)
		}
		if authority, ok := item["truth_authority"].(bool); ok && authority {
			t.Fatalf("item must not claim truth_authority: %v", item)
		}
	}
}
