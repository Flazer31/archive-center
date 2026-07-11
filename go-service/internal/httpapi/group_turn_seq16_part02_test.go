package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// SEQ-16-P188: retrieval result inspection surface — the prepare-turn
// response must expose a retrieval_result_inspection surface with lane
// counts, bounds, and authority status per lane.
func TestSeq16P188RetrievalResultInspection(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p188", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq16-p188", TurnIndex: 3, SummaryJSON: `{"text":"used the key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p188", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p188", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p188", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p188", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p188","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	insp, ok := resp["retrieval_result_inspection"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_result_inspection")
	}
	if insp["version"] != "p188a.v1" {
		t.Fatalf("expected version p188a.v1, got %v", insp["version"])
	}
	if insp["inspection_policy"] != "lane_count_bound_authority" {
		t.Fatalf("unexpected inspection_policy: %v", insp["inspection_policy"])
	}
	if insp["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", insp["truth_store"])
	}
	if insp["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", insp["retrieval_role"])
	}
	lanes, _ := insp["lanes"].([]any)
	if len(lanes) != 5 {
		t.Fatalf("lanes length = %v, want 5", len(lanes))
	}
	expectedLanes := map[string]struct {
		authority bool
		role      string
		depth     string
	}{
		"memory":          {false, "support_accelerator", "derived_summary"},
		"direct_evidence": {true, "canonical_truth", "canonical_evidence"},
		"kg_triple":       {false, "support_accelerator", "derived_graph"},
		"chat_log":        {false, "fallback_support", "raw_turn"},
		"episode_summary": {false, "support_accelerator", "derived_summary"},
	}
	for i, raw := range lanes {
		l, _ := raw.(map[string]any)
		if l == nil {
			t.Fatalf("lane[%d] not a map", i)
		}
		name, _ := l["lane"].(string)
		want, ok := expectedLanes[name]
		if !ok {
			t.Fatalf("unexpected lane %q", name)
		}
		if l["authority"] != want.authority {
			t.Fatalf("lane %q authority=%v, want %v", name, l["authority"], want.authority)
		}
		if l["role"] != want.role {
			t.Fatalf("lane %q role=%v, want %v", name, l["role"], want.role)
		}
		if l["source_depth"] != want.depth {
			t.Fatalf("lane %q source_depth=%v, want %v", name, l["source_depth"], want.depth)
		}
		if l["total"] == nil {
			t.Fatalf("lane %q missing total", name)
		}
		if l["bound"] == nil {
			t.Fatalf("lane %q missing bound", name)
		}
	}
	if insp["reason"] != "seq16_p188_retrieval_result_inspection_surface" {
		t.Fatalf("unexpected reason: %v", insp["reason"])
	}
}

// SEQ-16-P189: sparse-tail recall route — the prepare-turn response must
// expose a sparse_tail_recall surface with dense_summary and
// raw_evidence_support routes, each showing summary_only and
// has_direct_pointer.
func TestSeq16P189SparseTailRecall(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p189", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p189", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p189", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p189", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p189", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p189","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	recall, ok := resp["sparse_tail_recall"].(map[string]any)
	if !ok {
		t.Fatalf("missing sparse_tail_recall")
	}
	if recall["version"] != "p189a.v1" {
		t.Fatalf("expected version p189a.v1, got %v", recall["version"])
	}
	if recall["recall_policy"] != "dense_summary_plus_raw_evidence_support" {
		t.Fatalf("unexpected recall_policy: %v", recall["recall_policy"])
	}
	if recall["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", recall["truth_store"])
	}
	if recall["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", recall["retrieval_role"])
	}
	routes, _ := recall["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("routes length = %v, want 3", len(routes))
	}
	expectedRoutes := map[string]struct {
		summaryOnly      bool
		hasDirectPointer bool
		role             string
	}{
		"dense_summary":        {true, false, "primary_support"},
		"raw_evidence_support": {false, true, "fallback_support"},
		"graph_link_support":   {false, true, "support_accelerator"},
	}
	for i, raw := range routes {
		r, _ := raw.(map[string]any)
		if r == nil {
			t.Fatalf("route[%d] not a map", i)
		}
		name, _ := r["route_name"].(string)
		want, ok := expectedRoutes[name]
		if !ok {
			t.Fatalf("unexpected route_name %q", name)
		}
		if r["summary_only"] != want.summaryOnly {
			t.Fatalf("route %q summary_only=%v, want %v", name, r["summary_only"], want.summaryOnly)
		}
		if r["has_direct_pointer"] != want.hasDirectPointer {
			t.Fatalf("route %q has_direct_pointer=%v, want %v", name, r["has_direct_pointer"], want.hasDirectPointer)
		}
		if r["role"] != want.role {
			t.Fatalf("route %q role=%v, want %v", name, r["role"], want.role)
		}
		srcs, _ := r["sources"].([]any)
		if len(srcs) == 0 {
			t.Fatalf("route %q sources must be non-empty", name)
		}
	}
	if recall["reason"] != "seq16_p189_sparse_tail_recall_dense_summary_raw_evidence_support_route" {
		t.Fatalf("unexpected reason: %v", recall["reason"])
	}
}

// SEQ-16-P193: validity window / invalidation reading - the prepare-turn
// response must expose a validity_window_reading surface with window_start,
// window_end, latest_chat_turn, latest_episode_to, and an invalidated flag.
func TestSeq16P193ValidityWindowReading(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p193", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p193", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p193", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p193", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p193","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vw, ok := resp["validity_window_reading"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_reading")
	}
	if vw["version"] != "p193a.v1" {
		t.Fatalf("expected version p193a.v1, got %v", vw["version"])
	}
	if vw["window_policy"] != "validity_first_invalidation_reading" {
		t.Fatalf("unexpected window_policy: %v", vw["window_policy"])
	}
	if vw["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", vw["truth_store"])
	}
	if vw["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", vw["retrieval_role"])
	}
	if vw["window_start"] != float64(1) {
		t.Fatalf("window_start = %v, want 1", vw["window_start"])
	}
	if vw["window_end"] != float64(4) {
		t.Fatalf("window_end = %v, want 4", vw["window_end"])
	}
	if vw["latest_chat_turn"] != float64(4) {
		t.Fatalf("latest_chat_turn = %v, want 4", vw["latest_chat_turn"])
	}
	if vw["latest_chat_role"] != "user" {
		t.Fatalf("latest_chat_role = %v, want user", vw["latest_chat_role"])
	}
	if vw["latest_episode_to"] != float64(4) {
		t.Fatalf("latest_episode_to = %v, want 4", vw["latest_episode_to"])
	}
	if vw["invalidated"] != true {
		t.Fatalf("invalidated = %v, want true", vw["invalidated"])
	}
	if vw["invalidation_reason"] != "newer_chat_turn_exists" {
		t.Fatalf("invalidation_reason = %v, want newer_chat_turn_exists", vw["invalidation_reason"])
	}
	if vw["reason"] != "seq16_p193_validity_window_invalidation_reading" {
		t.Fatalf("unexpected reason: %v", vw["reason"])
	}
}

// SEQ-16-P194: current truth vs old truth coexistence rules - the
// prepare-turn response must expose a truth_coexistence_rules surface with
// items marked current_truth or old_truth, authority=true only for
// direct_evidence, and coexist_rule per item.
func TestSeq16P194TruthCoexistenceRules(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p194", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p194", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p194", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p194","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	coex, ok := resp["truth_coexistence_rules"].(map[string]any)
	if !ok {
		t.Fatalf("missing truth_coexistence_rules")
	}
	if coex["version"] != "p194a.v1" {
		t.Fatalf("expected version p194a.v1, got %v", coex["version"])
	}
	if coex["coexist_policy"] != "current_truth_vs_old_truth_kept" {
		t.Fatalf("unexpected coexist_policy: %v", coex["coexist_policy"])
	}
	if coex["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", coex["truth_store"])
	}
	if coex["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", coex["retrieval_role"])
	}
	items, _ := coex["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items length = %v, want 2", len(items))
	}
	expected := map[int64]struct {
		status     string
		authority  bool
		superseded bool
	}{
		10: {"old_truth", true, true},
		1:  {"old_support", false, false},
	}
	for i, raw := range items {
		it, _ := raw.(map[string]any)
		if it == nil {
			t.Fatalf("item[%d] not a map", i)
		}
		id := int64(it["id"].(float64))
		want, ok := expected[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if it["status"] != want.status {
			t.Fatalf("item %d status=%v, want %v", id, it["status"], want.status)
		}
		if it["authority"] != want.authority {
			t.Fatalf("item %d authority=%v, want %v", id, it["authority"], want.authority)
		}
		if it["superseded"] != want.superseded {
			t.Fatalf("item %d superseded=%v, want %v", id, it["superseded"], want.superseded)
		}
	}
	if coex["reason"] != "seq16_p194_current_truth_vs_old_truth_coexistence_rules" {
		t.Fatalf("unexpected reason: %v", coex["reason"])
	}
}

// SEQ-16-P195: event retrieval / temporal disambiguation contract - the
// prepare-turn response must expose a temporal_disambiguation_contract
// surface with buckets, each showing from_turn, to_turn, ambiguous, and
// disambiguated.
func TestSeq16P195TemporalDisambiguationContract(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p195", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p195", SourceTurnStart: 2, SourceTurnEnd: 3, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p195", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p195", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p195","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dc, ok := resp["temporal_disambiguation_contract"].(map[string]any)
	if !ok {
		t.Fatalf("missing temporal_disambiguation_contract")
	}
	if dc["version"] != "p195a.v1" {
		t.Fatalf("expected version p195a.v1, got %v", dc["version"])
	}
	if dc["disambig_policy"] != "turn_span_exactness" {
		t.Fatalf("unexpected disambig_policy: %v", dc["disambig_policy"])
	}
	if dc["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", dc["truth_store"])
	}
	if dc["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", dc["retrieval_role"])
	}
	buckets, _ := dc["buckets"].([]any)
	if len(buckets) != 4 {
		t.Fatalf("buckets length = %v, want 4", len(buckets))
	}
	expectedBuckets := map[string]struct {
		ambiguous     bool
		disambiguated bool
		depth         string
	}{
		"ep_1_4": {false, true, "derived_summary"},
		"ev_10":  {true, false, "canonical_evidence"},
		"mem_1":  {false, true, "derived_summary"},
		"cl_30":  {false, true, "raw_turn"},
	}
	for i, raw := range buckets {
		b, _ := raw.(map[string]any)
		if b == nil {
			t.Fatalf("bucket[%d] not a map", i)
		}
		bid, _ := b["bucket_id"].(string)
		want, ok := expectedBuckets[bid]
		if !ok {
			t.Fatalf("unexpected bucket_id %q", bid)
		}
		if b["ambiguous"] != want.ambiguous {
			t.Fatalf("bucket %q ambiguous=%v, want %v", bid, b["ambiguous"], want.ambiguous)
		}
		if b["disambiguated"] != want.disambiguated {
			t.Fatalf("bucket %q disambiguated=%v, want %v", bid, b["disambiguated"], want.disambiguated)
		}
		if b["source_depth"] != want.depth {
			t.Fatalf("bucket %q source_depth=%v, want %v", bid, b["source_depth"], want.depth)
		}
	}
	if dc["reason"] != "seq16_p195_event_retrieval_temporal_disambiguation_contract" {
		t.Fatalf("unexpected reason: %v", dc["reason"])
	}
}

// SEQ-16-P196: current vs pending-current vs old-truth read contract with
// promotion lag invisibility split — the prepare-turn response must expose
// a promotion_lag_invisibility_split surface with reads, status,
// visibility, and promotion_lag per item.
func TestSeq16P196PromotionLagInvisibilitySplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p196", ThreadKey: "locked-door", CreatedTurn: 2},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 60, ChatSessionID: "seq16-p196", TurnIndex: 3},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p196", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p196", SourceTurnStart: 4, SourceTurnEnd: 4, TurnAnchor: 4, EvidenceText: "door unlocked"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p196","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	pl, ok := resp["promotion_lag_invisibility_split"].(map[string]any)
	if !ok {
		t.Fatalf("missing promotion_lag_invisibility_split")
	}
	if pl["version"] != "p196a.v1" {
		t.Fatalf("expected version p196a.v1, got %v", pl["version"])
	}
	if pl["split_policy"] != "current_vs_pending_current_vs_old_truth" {
		t.Fatalf("unexpected split_policy: %v", pl["split_policy"])
	}
	if pl["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", pl["truth_store"])
	}
	if pl["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", pl["retrieval_role"])
	}
	reads, _ := pl["reads"].([]any)
	if len(reads) != 3 {
		t.Fatalf("reads length = %v, want 3", len(reads))
	}
	expectedReads := map[int64]struct {
		status       string
		visibility   string
		promotionLag int
		authority    bool
	}{
		50: {"old_pending", "invisible_until_promoted", 2, false},
		60: {"old_truth", "visible_historical_truth", 1, true},
		10: {"current_truth", "visible_canonical_truth", 0, true},
	}
	for i, raw := range reads {
		r, _ := raw.(map[string]any)
		if r == nil {
			t.Fatalf("read[%d] not a map", i)
		}
		id := int64(r["id"].(float64))
		want, ok := expectedReads[id]
		if !ok {
			t.Fatalf("unexpected id %d", id)
		}
		if r["status"] != want.status {
			t.Fatalf("read %d status=%v, want %v", id, r["status"], want.status)
		}
		if r["visibility"] != want.visibility {
			t.Fatalf("read %d visibility=%v, want %v", id, r["visibility"], want.visibility)
		}
		if int(r["promotion_lag"].(float64)) != want.promotionLag {
			t.Fatalf("read %d promotion_lag=%v, want %v", id, r["promotion_lag"], want.promotionLag)
		}
		if r["authority"] != want.authority {
			t.Fatalf("read %d authority=%v, want %v", id, r["authority"], want.authority)
		}
	}
	if pl["current_truth_count"] != float64(1) {
		t.Fatalf("current_truth_count=%v, want 1", pl["current_truth_count"])
	}
	if pl["old_truth_count"] != float64(1) {
		t.Fatalf("old_truth_count=%v, want 1", pl["old_truth_count"])
	}
	if pl["pending_current_count"] != float64(0) {
		t.Fatalf("pending_current_count=%v, want 0", pl["pending_current_count"])
	}
	if pl["old_pending_count"] != float64(1) {
		t.Fatalf("old_pending_count=%v, want 1", pl["old_pending_count"])
	}
	if pl["reason"] != "seq16_p196_promotion_lag_invisibility_split" {
		t.Fatalf("unexpected reason: %v", pl["reason"])
	}
}

// SEQ-16-P200: session/permanent authority replay — the prepare-turn
// response must expose a session_permanent_authority_replay surface that
// replays the retrieval_role_boundary counts and confirms authority awareness.
func TestSeq16P200SessionPermanentAuthorityReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p200", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p200", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p200", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p200", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p200", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq16-p200", TurnIndex: 4, Role: "user"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p200","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["session_permanent_authority_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_permanent_authority_replay")
	}
	if replay["version"] != "p200a.v1" {
		t.Fatalf("expected version p200a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "session_permanent_authority_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["permanent_count"] != float64(3) {
		t.Fatalf("permanent_count=%v, want 3", replay["permanent_count"])
	}
	if replay["session_count"] != float64(3) {
		t.Fatalf("session_count=%v, want 3", replay["session_count"])
	}
	if replay["boundary_stable"] != true {
		t.Fatalf("boundary_stable = %v, want true", replay["boundary_stable"])
	}
	if replay["authority_aware"] != true {
		t.Fatalf("authority_aware = %v, want true", replay["authority_aware"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p200_session_permanent_authority_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P201: normalized-unit support-only replay — the prepare-turn
// response must expose a normalized_unit_support_only_replay surface that
// replays the retrieval_units_ir and confirms all units are support-only.
func TestSeq16P201NormalizedUnitSupportOnlyReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p201", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p201", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p201", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p201", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p201","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["normalized_unit_support_only_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing normalized_unit_support_only_replay")
	}
	if replay["version"] != "p201a.v1" {
		t.Fatalf("expected version p201a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "normalized_unit_support_only_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["unit_count"] != float64(4) {
		t.Fatalf("unit_count=%v, want 4", replay["unit_count"])
	}
	if replay["inspected_unit_count"] != float64(4) {
		t.Fatalf("inspected_unit_count=%v, want 4", replay["inspected_unit_count"])
	}
	if replay["top_level_support_only"] != true {
		t.Fatalf("top_level_support_only=%v, want true", replay["top_level_support_only"])
	}
	if replay["all_support_only"] != true {
		t.Fatalf("all_support_only = %v, want true", replay["all_support_only"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p201_normalized_unit_support_only_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P202: multi-signal retrieval inspection replay — the prepare-turn
// response must expose a multi_signal_retrieval_inspection_replay surface
// that replays signal_mix_contract and retrieval_result_inspection counts.
func TestSeq16P202MultiSignalRetrievalInspectionReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p202", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p202", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p202", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p202", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p202", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p202","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["multi_signal_retrieval_inspection_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing multi_signal_retrieval_inspection_replay")
	}
	if replay["version"] != "p202a.v1" {
		t.Fatalf("expected version p202a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "multi_signal_retrieval_inspection_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["signal_count"] != float64(5) || replay["inspected_signal_count"] != float64(5) {
		t.Fatalf("signal counts=%v/%v, want 5/5", replay["signal_count"], replay["inspected_signal_count"])
	}
	if replay["lane_count"] != float64(5) || replay["inspected_lane_count"] != float64(5) {
		t.Fatalf("lane counts=%v/%v, want 5/5", replay["lane_count"], replay["inspected_lane_count"])
	}
	if replay["signals_support_only"] != true {
		t.Fatalf("signals_support_only=%v, want true", replay["signals_support_only"])
	}
	if replay["inspection_stable"] != true {
		t.Fatalf("inspection_stable = %v, want true", replay["inspection_stable"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p202_multi_signal_retrieval_inspection_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P203: validity-window temporal replay — the prepare-turn response
// must expose a validity_window_temporal_replay surface that replays
// temporal_read_validity_first and validity_window_reading consistency.
func TestSeq16P203ValidityWindowTemporalReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p203", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p203", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p203", SourceTurnStart: 2, SourceTurnEnd: 2, TurnAnchor: 2, EvidenceText: "door unlocked"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p203", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p203","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["validity_window_temporal_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing validity_window_temporal_replay")
	}
	if replay["version"] != "p203a.v1" {
		t.Fatalf("expected version p203a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "validity_window_temporal_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["latest_chat_turn"] != float64(4) {
		t.Fatalf("latest_chat_turn=%v, want 4", replay["latest_chat_turn"])
	}
	if replay["window_end"] != float64(4) {
		t.Fatalf("window_end=%v, want 4", replay["window_end"])
	}
	if replay["temporal_consistent"] != true {
		t.Fatalf("temporal_consistent = %v, want true", replay["temporal_consistent"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p203_validity_window_temporal_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P204: source-tagged authority-aware assembly replay — the
// prepare-turn response must expose a
// source_tagged_authority_aware_assembly_replay surface that replays
// source_tagged_retrieval_unit_surface and retrieval_role_boundary.
func TestSeq16P204SourceTaggedAuthorityAwareAssemblyReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p204", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p204", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p204", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p204", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq16-p204", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq16-p204", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq16-p204", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq16-p204", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq16-p204", ThreadKey: "locked-door", CreatedTurn: 4},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p204","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["source_tagged_authority_aware_assembly_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing source_tagged_authority_aware_assembly_replay")
	}
	if replay["version"] != "p204a.v1" {
		t.Fatalf("expected version p204a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "source_tagged_authority_aware_assembly_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["tagged_count"] != float64(4) || replay["inspected_tag_count"] != float64(4) {
		t.Fatalf("tag counts=%v/%v, want 4/4", replay["tagged_count"], replay["inspected_tag_count"])
	}
	if replay["all_units_tagged"] != true {
		t.Fatalf("all_units_tagged=%v, want true", replay["all_units_tagged"])
	}
	if replay["boundary_stable"] != true {
		t.Fatalf("boundary_stable = %v, want true", replay["boundary_stable"])
	}
	if replay["authority_aware"] != true {
		t.Fatalf("authority_aware = %v, want true", replay["authority_aware"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p204_source_tagged_authority_aware_assembly_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P205: critic-truncation spillover replay — the prepare-turn
// response must expose a critic_truncation_spillover_replay surface that
// replays raw_turn_span_metadata, sparse_tail_recall, and retrieval_units_ir
// to confirm recall is verifiable across summary/raw/evidence routes.
func TestSeq16P205CriticTruncationSpilloverReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p205", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p205", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p205", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p205", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p205", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p205","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	replay, ok := resp["critic_truncation_spillover_replay"].(map[string]any)
	if !ok {
		t.Fatalf("missing critic_truncation_spillover_replay")
	}
	if replay["version"] != "p205a.v1" {
		t.Fatalf("expected version p205a.v1, got %v", replay["version"])
	}
	if replay["replay_policy"] != "critic_truncation_spillover_replay" {
		t.Fatalf("unexpected replay_policy: %v", replay["replay_policy"])
	}
	if replay["span_count"] != float64(3) {
		t.Fatalf("span_count=%v, want 3", replay["span_count"])
	}
	if replay["route_count"] != float64(3) || replay["inspected_route_count"] != float64(3) {
		t.Fatalf("route counts=%v/%v, want 3/3", replay["route_count"], replay["inspected_route_count"])
	}
	if replay["unit_count"] != float64(4) {
		t.Fatalf("unit_count=%v, want 4", replay["unit_count"])
	}
	if replay["has_summary_route"] != true || replay["has_raw_evidence_route"] != true || replay["has_direct_pointer_route"] != true {
		t.Fatalf("route booleans summary/raw/direct=%v/%v/%v, want true/true/true", replay["has_summary_route"], replay["has_raw_evidence_route"], replay["has_direct_pointer_route"])
	}
	if replay["recall_verifiable"] != true {
		t.Fatalf("recall_verifiable = %v, want true", replay["recall_verifiable"])
	}
	if replay["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", replay["truth_store"])
	}
	if replay["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", replay["retrieval_role"])
	}
	if replay["reason"] != "seq16_p205_critic_truncation_spillover_replay" {
		t.Fatalf("unexpected reason: %v", replay["reason"])
	}
}

// SEQ-16-P209: session-partitioned index — the prepare-turn response must
// expose a session_partitioned_index surface that remigrates the legacy
// backend/tests/test_q1b_session_partitioned_index.py contract.
func TestSeq16P209SessionPartitionedIndex(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq16-p209", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "seq16-p209", SourceTurnStart: 3, SourceTurnEnd: 3, TurnAnchor: 3, EvidenceText: "door unlocked"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq16-p209", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 20, ChatSessionID: "seq16-p209", SourceTurn: 2, Subject: "Iris", Predicate: "has", Object: "key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ChatSessionID: "seq16-p209", FromTurn: 1, ToTurn: 4, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq16-p209","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":false,"input_context_enabled":true}}`
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
	idx, ok := resp["session_partitioned_index"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_partitioned_index")
	}
	if idx["version"] != "p209a.v1" {
		t.Fatalf("expected version p209a.v1, got %v", idx["version"])
	}
	if idx["index_policy"] != "session_partitioned" {
		t.Fatalf("unexpected index_policy: %v", idx["index_policy"])
	}
	if idx["chat_session_id"] != "seq16-p209" {
		t.Fatalf("unexpected chat_session_id: %v", idx["chat_session_id"])
	}
	if idx["document_count"] != float64(5) {
		t.Fatalf("document_count=%v, want 5", idx["document_count"])
	}
	tierCounts, _ := idx["tier_counts"].(map[string]any)
	for _, tier := range []string{"memory", "evidence", "chat_log", "kg_triple", "episode"} {
		if tierCounts[tier] != float64(1) {
			t.Fatalf("tier_counts[%s]=%v, want 1; all=%v", tier, tierCounts[tier], tierCounts)
		}
	}
	authorityTierCounts, _ := idx["authority_tier_counts"].(map[string]any)
	if authorityTierCounts["canonical"] != float64(1) || authorityTierCounts["support"] != float64(3) || authorityTierCounts["fallback"] != float64(1) {
		t.Fatalf("authority_tier_counts=%v, want canonical=1 support=3 fallback=1", authorityTierCounts)
	}
	snapshot, _ := idx["index_snapshot"].(map[string]any)
	if snapshot["document_count"] != float64(5) || snapshot["chat_session_id"] != "seq16-p209" || snapshot["schema_version"] != "q1e.v1" {
		t.Fatalf("unexpected index_snapshot: %v", snapshot)
	}
	if idx["truth_store"] != "maria_db" {
		t.Fatalf("expected truth_store=maria_db, got %v", idx["truth_store"])
	}
	if idx["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("unexpected retrieval_role: %v", idx["retrieval_role"])
	}
	if idx["reason"] != "seq16_p209_session_partitioned_index" {
		t.Fatalf("unexpected reason: %v", idx["reason"])
	}
}
