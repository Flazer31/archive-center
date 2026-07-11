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

// SEQ-16-P239: graph signal optional accelerator — validates that the
// /prepare-turn response exposes signal_mix_contract with graph signal
// as optional accelerator (not canonical truth).
func TestSeq16P239GraphSignalOptionalAccelerator(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p239", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "seq16-p239", Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p239", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p239","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	signals, _ := smc["signals"].([]any)
	var graphSignal map[string]any
	for _, raw := range signals {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if s["signal"] == "graph" {
			graphSignal = s
			break
		}
	}
	if graphSignal == nil {
		t.Fatalf("graph signal missing from signal_mix_contract")
	}
	if graphSignal["role"] != "support_accelerator" {
		t.Fatalf("graph signal role=%v, want support_accelerator", graphSignal["role"])
	}
	if authority, ok := graphSignal["truth_authority"].(bool); ok && authority {
		t.Fatalf("graph signal must not claim truth_authority")
	}

	if _, ok := graphSignal["count"]; !ok {
		t.Fatalf("graph signal missing count")
	}
}

// SEQ-16-P240: temporal invalidation storage/read surface — validates that
// the /prepare-turn response exposes temporal_disambiguation_contract with
// invalidation-aware markers.
func TestSeq16P240TemporalInvalidationStorageReadSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p240", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p240", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p240", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p240","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	td, ok := resp["temporal_disambiguation_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_disambiguation_contract")
	}
	if td["version"] != "p195a.v1" {
		t.Fatalf("version=%v, want p195a.v1", td["version"])
	}
	if td["disambig_policy"] != "turn_span_exactness" {
		t.Fatalf("disambig_policy=%v, want turn_span_exactness", td["disambig_policy"])
	}

	for _, k := range []string{"disambig_policy", "buckets", "bucket_count"} {
		if _, ok := td[k]; !ok {
			t.Fatalf("temporal_disambiguation_contract missing %q", k)
		}
	}

	buckets, _ := td["buckets"].([]any)
	if len(buckets) == 0 {
		t.Fatalf("temporal_disambiguation_contract.buckets must not be empty")
	}
	for _, raw := range buckets {
		b, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := b["from_turn"]; !ok {
			t.Fatalf("bucket missing from_turn: %v", b)
		}
		if _, ok := b["to_turn"]; !ok {
			t.Fatalf("bucket missing to_turn: %v", b)
		}
	}

	if td["authority"] == "canonical" {
		t.Fatalf("temporal_disambiguation_contract must not claim canonical authority")
	}
}

// SEQ-16-P241: query class routing table — validates that the /prepare-turn
// response exposes query_class_routing with a non-empty classes array and
// truth_authority=false on every class.
func TestSeq16P241QueryClassRoutingTable(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p241", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p241", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p241", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p241","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	qcr, ok := resp["query_class_routing"].(map[string]any)
	if !ok {
		t.Fatalf("missing query_class_routing")
	}
	if qcr["version"] != "p187a.v1" {
		t.Fatalf("version=%v, want p187a.v1", qcr["version"])
	}
	classes, _ := qcr["classes"].([]any)
	if len(classes) == 0 {
		t.Fatalf("query_class_routing.classes must not be empty")
	}
	for _, raw := range classes {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if authority, ok := c["truth_authority"].(bool); ok && authority {
			t.Fatalf("class %v must not claim truth_authority", c["query_class"])
		}
		if _, ok := c["query_class"]; !ok {
			t.Fatalf("class missing query_class: %v", c)
		}
		if _, ok := c["depth_policy"]; !ok {
			t.Fatalf("class missing depth_policy: %v", c)
		}
		if _, ok := c["primary_signal"]; !ok {
			t.Fatalf("class missing primary_signal: %v", c)
		}
	}
}

// SEQ-16-P245: 16-C1 MariaDB/Store truth + Chroma shadow role split contract —
// validates that the /prepare-turn response exposes truth_store=maria_db and
// retrieval_role=support_accelerator_only on key surfaces.
func TestSeq16P245C1TruthStoreChromaShadowRoleSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p245", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p245", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p245", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p245","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	surfaces := []string{"retrieval_index_ir", "signal_mix_contract", "query_class_routing", "retrieval_result_inspection"}
	for _, name := range surfaces {
		surface, ok := resp[name].(map[string]any)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if surface["truth_store"] != "maria_db" {
			t.Fatalf("%s truth_store=%v, want maria_db", name, surface["truth_store"])
		}
		if surface["retrieval_role"] != "support_accelerator_only" {
			t.Fatalf("%s retrieval_role=%v, want support_accelerator_only", name, surface["retrieval_role"])
		}
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	if recall["source"] != "go_r1_read_shadow" {
		t.Fatalf("recall_result.source=%v, want go_r1_read_shadow", recall["source"])
	}
}

// SEQ-16-P246: 16-C2 Chroma adapter contract — validates that the
// /prepare-turn response exposes index_lifecycle with retrieval backend
// abstraction, collection naming, session partitioning, and embedder identity.
func TestSeq16P246C2ChromaAdapterContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p246", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p246", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p246","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	il, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if il["version"] != "p210a.v1" {
		t.Fatalf("version=%v, want p210a.v1", il["version"])
	}
	if il["truth_store"] != "maria_db" {
		t.Fatalf("truth_store=%v, want maria_db", il["truth_store"])
	}
	if il["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("retrieval_role=%v, want support_accelerator_only", il["retrieval_role"])
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["engine"] != "chromadb" {
		t.Fatalf("vector_shadow.engine=%v, want chromadb", vs["engine"])
	}
	for _, key := range []string{"optional_engine", "milvus_required", "milvus_live_enabled"} {
		if _, ok := vs[key]; ok {
			t.Fatalf("vector_shadow.%s should not be exposed in ChromaDB-only runtime: %+v", key, vs)
		}
	}

	if filter, ok := vs["filter"].(string); ok && !strings.Contains(filter, "seq16-p246") {
		t.Fatalf("vector_shadow.filter=%v, must contain session id", filter)
	}

	if _, ok := vs["model_ready"]; !ok {
		t.Fatalf("vector_shadow missing model_ready")
	}
	if _, ok := vs["project_model"]; !ok {
		t.Fatalf("vector_shadow missing project_model")
	}
}

// SEQ-16-P247: 16-C3 normalized retrieval unit -> Chroma document mapping —
// validates that recall_result.documents follow the q1a.v1 schema with
// document_id, tier, source_type, source_table, metadata, and query_matched.
func TestSeq16P247C3NormalizedUnitToChromaDocumentMapping(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p247", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p247", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p247", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p247","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	docs, ok := recall["documents"].([]any)
	if !ok {
		t.Fatalf("missing documents")
	}
	if len(docs) == 0 {
		t.Fatalf("documents must not be empty")
	}
	schema, _ := recall["document_schema"].(map[string]any)
	if schema == nil || schema["version"] != "q1a.v1" {
		t.Fatalf("document_schema.version missing or wrong")
	}
	required := []string{"document_id", "tier", "source_type", "source_table", "metadata", "query_matched"}
	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, k := range required {
			if _, ok := doc[k]; !ok {
				t.Fatalf("document missing %q: %v", k, doc)
			}
		}

		meta, _ := doc["metadata"].(map[string]any)
		if meta == nil {
			t.Fatalf("document metadata missing")
		}
	}
}

// SEQ-16-P248: 16-C4 shadow write / shadow read / off mode runtime matrix —
// validates that the /prepare-turn response exposes runtime_toggle with
// mode matrix and vector_shadow with live_retrieval_enabled=false (off by default).
func TestSeq16P248C4ShadowWriteReadOffModeMatrix(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p248", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p248", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p248","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["version"] != "p212a.v1" {
		t.Fatalf("version=%v, want p212a.v1", rt["version"])
	}

	for _, k := range []string{"injection_enabled", "input_context_enabled", "mode", "broad_takeover"} {
		if _, ok := rt[k]; !ok {
			t.Fatalf("runtime_toggle missing %q", k)
		}
	}

	if rt["mode"] != "shadow_guarded" {
		t.Fatalf("mode=%v, want shadow_guarded", rt["mode"])
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("missing recall_result")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["live_retrieval_enabled"] != false {
		t.Fatalf("vector_shadow.live_retrieval_enabled=%v, want false", vs["live_retrieval_enabled"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if traceSummary["would_write"] != false {
		t.Fatalf("trace_summary.would_write=%v, want false", traceSummary["would_write"])
	}
}

// SEQ-16-P249: 16-C5 degraded fallback — validates that the /prepare-turn
// response exposes degraded fallback surfaces when vector store is unavailable.
func TestSeq16P249C5DegradedFallbackContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p249", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p249", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p249","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	status, _ := vs["status"].(string)
	if status != "unconfigured" && status != "disabled" && status != "degraded" && status != "shadow" {
		t.Fatalf("vector_shadow.status=%v, want unconfigured/disabled/degraded/shadow", status)
	}

	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents must not be empty even when vector is degraded")
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["degraded"] != false {
		t.Fatalf("generation_packet.degraded=%v, want false (store is available)", gp["degraded"])
	}

	if resp["fallback_reason"] != "" {
		t.Fatalf("fallback_reason=%v, want empty", resp["fallback_reason"])
	}
}

// SEQ-16-P250: 16-C6 temporal read contract connect — validates that the
// /prepare-turn response exposes temporal_proximity_boost with recency-aware
// markers and that validity_window_reading coexists.
func TestSeq16P250C6TemporalReadContractConnect(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p250", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p250", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p250", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p250","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	boost, ok := recall["temporal_proximity_boost"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_proximity_boost")
	}
	if boost["version"] != "p71a.v1" {
		t.Fatalf("version=%v, want p71a.v1", boost["version"])
	}
	if boost["status"] != "shadow_only" {
		t.Fatalf("status=%v, want shadow_only", boost["status"])
	}

	if _, ok := boost["recent_turns"]; !ok {
		t.Fatalf("temporal_proximity_boost missing recent_turns")
	}

	if _, ok := resp["validity_window_reading"]; !ok {
		t.Fatalf("P193 validity_window_reading missing — temporal read contract requires coexistence")
	}

	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents missing or empty")
	}
	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := doc["created_at"]; !ok {
			t.Fatalf("document missing created_at: %v", doc)
		}
		if _, ok := doc["from_turn"]; !ok {
			t.Fatalf("document missing from_turn: %v", doc)
		}
		if _, ok := doc["to_turn"]; !ok {
			t.Fatalf("document missing to_turn: %v", doc)
		}
	}
}

// SEQ-16-P251: 16-C7 Step 16 exit gate — validates shadow-mode smoke pass
// and confirms bulk backfill / migration cutover is deferred to Step 17.
func TestSeq16P251C7Step16ExitGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p251", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p251", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p251", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p251","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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

	il, ok := resp["index_lifecycle"].(map[string]any)
	if !ok {
		t.Fatalf("missing index_lifecycle")
	}
	if _, ok := il["rebuild_ready"]; !ok {
		t.Fatalf("index_lifecycle missing rebuild_ready")
	}
	vs, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("missing vector_shadow")
	}
	if vs["backfill_attempted"] != false {
		t.Fatalf("backfill_attempted=%v, want false (cutover deferred)", vs["backfill_attempted"])
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
}

// SEQ-16-P252: 16-C8 no-summary-only dependency — validates that Chroma
// documents preserve dense summary plus raw/evidence pointers.
func TestSeq16P252C8NoSummaryOnlyDependency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p252", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`, PlaceWing: "wing_a", PlaceRoom: "room_1"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p252", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p252", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p252","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	docs, ok := recall["documents"].([]any)
	if !ok || len(docs) == 0 {
		t.Fatalf("documents missing or empty")
	}

	for _, raw := range docs {
		doc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text, _ := doc["text"].(string)
		if text == "" {
			t.Fatalf("document text must not be empty (no-summary-only dependency)")
		}
		meta, _ := doc["metadata"].(map[string]any)
		if meta == nil {
			t.Fatalf("document metadata missing")
		}

		if len(meta) == 0 {
			t.Fatalf("document metadata must contain raw/evidence pointers")
		}
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["latest_direct_evidence_text"] == nil {
		t.Fatalf("injection_pack.latest_direct_evidence_text must not be nil")
	}
}
