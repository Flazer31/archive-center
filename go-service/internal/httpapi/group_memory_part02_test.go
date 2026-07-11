package httpapi

import (
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

// Test 3: GET /explorer/direct-evidence returns Store-backed data
func TestExplorerDirectEvidenceReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 30, ChatSessionID: "sess-3", EvidenceKind: "fact", EvidenceText: "evidence text that is longer than one hundred and twenty characters so that we can verify the preview truncation dots and more", ArchiveState: "pending_capture", CaptureVerification: "verified", CaptureStage: "critic_extract", CommittedGate: "", LineageJSON: `{"origin":"critic_extract","bucket":"direct_archive"}`, SourceMessageIDsJSON: `["msg-1","msg-5"]`, SourceHash: "sha256:direct-evidence-fixture", SourceTurnStart: 1, SourceTurnEnd: 5, TurnAnchor: 3, RepairNeeded: false, Tombstoned: false, SupersededByID: 0},
		},
		auditLogs: []store.AuditLog{
			{ID: 11, EventType: "critic_ingest_trace", ChatSessionID: "sess-3", DetailsJSON: `{"surface":"direct_evidence","trace":{"elapsed_ms":8.669,"inserted":2,"skipped":1,"write_chars":93}}`},
			{ID: 10, EventType: "critic_ingest_trace", ChatSessionID: "other", DetailsJSON: `{"surface":"direct_evidence","trace":{"elapsed_ms":999,"inserted":9,"skipped":9,"write_chars":999}}`},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-3", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
	it := items[0].(map[string]any)
	if it["evidence_preview"] != "evidence text that is longer than one hundred and twenty characters so that we can verify the preview truncation dots an..." {
		t.Errorf("evidence_preview mismatch: got %q", it["evidence_preview"])
	}
	if it["committed_gate"] != nil {
		t.Errorf("committed_gate = %v, want nil", it["committed_gate"])
	}
	if it["normalized_committed_gate"] != "finalize" {
		t.Errorf("normalized_committed_gate = %v, want finalize", it["normalized_committed_gate"])
	}
	if it["archive_bucket"] != "pending_capture" {
		t.Errorf("archive_bucket = %v, want pending_capture", it["archive_bucket"])
	}
	if it["retention_ttl_turns"] != float64(180) {
		t.Errorf("retention_ttl_turns = %v, want 180", it["retention_ttl_turns"])
	}
	if it["source_message_ids"] == nil {
		t.Error("source_message_ids is nil")
	}
	sourceIDs := it["source_message_ids"].([]any)
	if len(sourceIDs) != 2 || sourceIDs[0] != "msg-1" || sourceIDs[1] != "msg-5" {
		t.Fatalf("source_message_ids = %#v, want msg-1/msg-5 lineage", sourceIDs)
	}
	if it["source_turn_start"] != float64(1) || it["source_turn_end"] != float64(5) {
		t.Fatalf("source turn range = %v-%v, want 1-5", it["source_turn_start"], it["source_turn_end"])
	}
	if it["source_hash"] != "sha256:direct-evidence-fixture" {
		t.Fatalf("source_hash = %v, want fixture hash", it["source_hash"])
	}
	if it["lineage"] == nil {
		t.Error("lineage is nil")
	}
	lineage := it["lineage"].(map[string]any)
	if lineage["origin"] != "critic_extract" || lineage["bucket"] != "direct_archive" {
		t.Fatalf("lineage = %#v, want direct archive lineage", lineage)
	}
	if it["excluded_from_current_truth"] != false {
		t.Errorf("excluded_from_current_truth = %v, want false", it["excluded_from_current_truth"])
	}
	if it["turn_anchor"] != float64(3) {
		t.Errorf("turn_anchor = %v, want 3", it["turn_anchor"])
	}
	if resp["cost_measurement"] == nil {
		t.Error("cost_measurement is nil")
	}
	cm := resp["cost_measurement"].(map[string]any)
	write := cm["direct_evidence_write"].(map[string]any)
	if write["sample_count"] != float64(1) {
		t.Errorf("direct_evidence_write.sample_count = %v, want 1", write["sample_count"])
	}
	if write["avg_latency_ms"] != 8.669 {
		t.Errorf("direct_evidence_write.avg_latency_ms = %v, want 8.669", write["avg_latency_ms"])
	}
	if write["avg_write_chars"] != float64(93) {
		t.Errorf("direct_evidence_write.avg_write_chars = %v, want 93", write["avg_write_chars"])
	}
	rq := cm["repair_queue"].(map[string]any)
	if rq["queue_count"] != float64(0) {
		t.Errorf("repair_queue.queue_count = %v, want 0", rq["queue_count"])
	}
	contract := resp["state_contract"].(map[string]any)
	if contract["conflict_resolution_policy_version"] != "ea1h.v1" || contract["conflict_confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict policy contract mismatch: %#v", contract)
	}
	cr := it["conflict_resolution"].(map[string]any)
	if cr["policy_version"] != "ea1h.v1" || cr["confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("conflict resolution policy mismatch: %#v", cr)
	}
	if cr["classification"] != "state_transition" || cr["route"] != "superseded" {
		t.Fatalf("conflict resolution = %#v, want state_transition/superseded", cr)
	}
}

func TestExplorerDirectEvidenceConflictStateMachineAndRetention(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 101, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "Alice now trusts Bob.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"state_transition","confidence":0.92,"field_class":"relationship","importance_tier":"high"}`, SourceTurnStart: 10, SourceTurnEnd: 10, TurnAnchor: 10, CreatedAt: time.Date(2026, 6, 1, 0, 0, 4, 0, time.UTC)},
			{ID: 102, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "Alice is secretly another person.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"hard_contradiction","confidence":0.72,"field_class":"identity","high_impact":true}`, SourceTurnStart: 11, SourceTurnEnd: 11, TurnAnchor: 11, CreatedAt: time.Date(2026, 6, 1, 0, 0, 3, 0, time.UTC)},
			{ID: 103, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "An older archive says Alice avoided Bob.", ArchiveState: "previous_archive", CaptureVerification: "verified", LineageJSON: `{"conflict_class":"parallel_context","confidence":0.75,"field_class":"relationship"}`, SourceTurnStart: 1, SourceTurnEnd: 1, TurnAnchor: 1, CreatedAt: time.Date(2026, 6, 1, 0, 0, 2, 0, time.UTC)},
			{ID: 104, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "A weak rumor says the trust scene was false.", ArchiveState: "pending_capture", CaptureVerification: "pending", LineageJSON: `{"conflict_class":"low_confidence_noise","confidence":0.22,"field_class":"rumor"}`, SourceTurnStart: 12, SourceTurnEnd: 12, TurnAnchor: 12, CreatedAt: time.Date(2026, 6, 1, 0, 0, 1, 0, time.UTC)},
			{ID: 105, ChatSessionID: "sess-conflict", EvidenceKind: "fact", EvidenceText: "A deleted turn once contradicted Alice's trust.", ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"importance_tier":"critical"}`, SourceTurnStart: 13, SourceTurnEnd: 13, TurnAnchor: 13, Tombstoned: true, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
		},
		chatLogs: []store.ChatLog{{ChatSessionID: "sess-conflict", TurnIndex: 20}},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-conflict&limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	contract := resp["state_contract"].(map[string]any)
	if contract["conflict_resolution_policy_version"] != "ea1h.v1" || contract["conflict_confidence_policy_version"] != "ea1i.v1" {
		t.Fatalf("contract conflict policy mismatch: %#v", contract)
	}
	if _, ok := contract["conflict_confidence_thresholds"].(map[string]any); !ok {
		t.Fatalf("contract missing confidence thresholds: %#v", contract)
	}
	items := resp["items"].([]any)
	byID := map[float64]map[string]any{}
	for _, raw := range items {
		item := raw.(map[string]any)
		byID[item["id"].(float64)] = item
	}
	assertConflict := func(id float64, class, route string, manual bool) {
		item, ok := byID[id]
		if !ok {
			t.Fatalf("missing evidence id %.0f in %#v", id, byID)
		}
		cr := item["conflict_resolution"].(map[string]any)
		if cr["classification"] != class || cr["route"] != route || cr["requires_manual_review"] != manual {
			t.Fatalf("id %.0f conflict = %#v, want %s/%s/manual=%v", id, cr, class, route, manual)
		}
	}
	assertConflict(101, "state_transition", "superseded", false)
	assertConflict(102, "hard_contradiction", "manual_review", true)
	assertConflict(103, "parallel_context", "hold", false)
	assertConflict(104, "low_confidence_noise", "hold", false)
	assertConflict(105, "hard_contradiction", "tombstone", false)

	tombstone := byID[105]["conflict_resolution"].(map[string]any)
	if tombstone["retention_importance_tier"] != "critical" {
		t.Fatalf("tombstone retention tier = %v, want critical", tombstone["retention_importance_tier"])
	}
	if byID[105]["retention_ttl_turns"] != float64(240) || byID[105]["excluded_from_current_truth"] != true {
		t.Fatalf("tombstone retention/current truth fields mismatch: %#v", byID[105])
	}
}

// Test 4: GET /explorer/kg_triples returns Store-backed data
func TestExplorerKGTriplesReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 40, ChatSessionID: "sess-4", Subject: "Alice", Predicate: "knows", Object: "Bob", ValidFrom: 1, ValidTo: 999, SourceTurn: 2},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["total"] != float64(1) {
		t.Errorf("total = %v, want 1", resp["total"])
	}
	it := items[0].(map[string]any)
	if it["valid_from"] != float64(1) {
		t.Errorf("valid_from = %v, want 1", it["valid_from"])
	}
	if it["valid_to"] != float64(999) {
		t.Errorf("valid_to = %v, want 999", it["valid_to"])
	}
	if it["source_turn"] != float64(2) {
		t.Errorf("source_turn = %v, want 2", it["source_turn"])
	}
	if it["created_at"] != nil {
		t.Errorf("created_at = %v, want nil", it["created_at"])
	}
}

func TestExplorerKGTriplesNullableAndSort(t *testing.T) {
	now := time.Now().UTC()
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-4", Subject: "A", Predicate: "p1", Object: "o1", ValidFrom: 0, ValidTo: 0, SourceTurn: 0, CreatedAt: now.Add(-2 * time.Hour)},
			{ID: 2, ChatSessionID: "sess-4", Subject: "B", Predicate: "p2", Object: "o2", ValidFrom: 3, ValidTo: 0, SourceTurn: 1, CreatedAt: now.Add(-1 * time.Hour)},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}

	first := items[0].(map[string]any)
	if first["id"] != float64(2) {
		t.Errorf("first id = %v, want 2", first["id"])
	}
	if first["valid_from"] != float64(3) {
		t.Errorf("first valid_from = %v, want 3", first["valid_from"])
	}
	if first["valid_to"] != nil {
		t.Errorf("first valid_to = %v, want nil", first["valid_to"])
	}
	if first["source_turn"] != float64(1) {
		t.Errorf("first source_turn = %v, want 1", first["source_turn"])
	}
	second := items[1].(map[string]any)
	if second["id"] != float64(1) {
		t.Errorf("second id = %v, want 1", second["id"])
	}
	if second["valid_from"] != nil {
		t.Errorf("second valid_from = %v, want nil", second["valid_from"])
	}
	if second["valid_to"] != nil {
		t.Errorf("second valid_to = %v, want nil", second["valid_to"])
	}
	if second["source_turn"] != nil {
		t.Errorf("second source_turn = %v, want nil", second["source_turn"])
	}
}

// Test 5: POST /kg/recall returns Store-backed data
func TestKGRecallReturnsTriplesFromStore(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "sits_on", Object: "Mat", ValidFrom: 0, ValidTo: 0, SourceTurn: 3},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-5","entities":["Cat","Mat"]}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	if resp["count"] != float64(1) {
		t.Errorf("count = %v, want 1", resp["count"])
	}
	if resp["entities_received"] != float64(2) {
		t.Errorf("entities_received = %v, want 2", resp["entities_received"])
	}
}

func TestKGRecallMatchesMultilingualNormalizedEntityKeys(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 60, ChatSessionID: "sess-f2", Subject: "민아", Predicate: "trusts", Object: "アキラ", SourceTurn: 4},
			{ID: 61, ChatSessionID: "sess-f2", Subject: "Unrelated", Predicate: "ignores", Object: "Nobody", SourceTurn: 4},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-f2","entities":["Mina","Akira"],"limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1: %#v", len(items), items)
	}
	item := items[0].(map[string]any)
	if item["subject"] != "민아" || item["object"] != "アキラ" {
		t.Fatalf("matched item = %#v, want multilingual KG triple", item)
	}
}

func TestKGRecallKeepsEnglishMatchesWithMultilingualEntityQuery(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 71, ChatSessionID: "sess-f2-runtime", Subject: "Mina", Predicate: "trusts", Object: "Rowan", SourceTurn: 5},
			{ID: 70, ChatSessionID: "sess-f2-runtime", Subject: "\uC544\uD0A4\uB77C", Predicate: "guards", Object: "\u30A2\u30AD\u30E9", SourceTurn: 4},
			{ID: 69, ChatSessionID: "sess-f2-runtime", Subject: "Unrelated", Predicate: "ignores", Object: "Nobody", SourceTurn: 3},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-f2-runtime","entities":["Mina","Rowan","Akira","\uC544\uD0A4\uB77C","\u30A2\u30AD\u30E9"],"limit":10}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2: %#v", len(items), items)
	}
	seen := map[string]bool{}
	for _, raw := range items {
		item := raw.(map[string]any)
		seen[item["subject"].(string)+"->"+item["object"].(string)] = true
	}
	if !seen["Mina->Rowan"] {
		t.Fatalf("English KG recall match disappeared: %#v", items)
	}
	if !seen["\uC544\uD0A4\uB77C->\u30A2\u30AD\u30E9"] {
		t.Fatalf("multilingual KG recall match disappeared: %#v", items)
	}
}

func TestKGRecallFiltersExpiredTriplesWhenCurrentTurnProvided(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 50, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "sits_on", Object: "Mat", ValidFrom: 1, ValidTo: 3, SourceTurn: 1},
			{ID: 51, ChatSessionID: "sess-5", Subject: "Cat", Predicate: "guards", Object: "Mat", ValidFrom: 4, ValidTo: 0, SourceTurn: 4},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-5","entities":["Cat","Mat"],"current_turn":5}`
	req := httptest.NewRequest(http.MethodPost, "/kg/recall", strings.NewReader(body))
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
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["predicate"] != "guards" {
		t.Errorf("predicate = %v, want guards", item["predicate"])
	}
	if resp["current_turn"] != float64(5) {
		t.Errorf("current_turn = %v, want 5", resp["current_turn"])
	}
	if resp["expired_filtered"] != float64(1) {
		t.Errorf("expired_filtered = %v, want 1", resp["expired_filtered"])
	}
}

// Test 6: GET /retrieval-index/{sid} returns Store-backed document_count and status
func TestRetrievalIndexSnapshotReturnsStoreData(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-6", TurnIndex: 1},
			{ID: 2, ChatSessionID: "sess-6", TurnIndex: 2},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 3, ChatSessionID: "sess-6", EvidenceKind: "fact"},
		},
		kgTriples: []store.KGTriple{
			{ID: 4, ChatSessionID: "sess-6", Subject: "A"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-6", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["document_count"] != float64(4) {
		t.Errorf("document_count = %v, want 4", resp["document_count"])
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	stc, ok := resp["source_type_counts"].(map[string]any)
	if !ok {
		t.Fatalf("source_type_counts is not an object: %T", resp["source_type_counts"])
	}
	if stc["memories"] != float64(2) {
		t.Errorf("source_type_counts.memories = %v, want 2", stc["memories"])
	}
	if stc["direct_evidence"] != float64(1) {
		t.Errorf("source_type_counts.direct_evidence = %v, want 1", stc["direct_evidence"])
	}
	if stc["kg_triples"] != float64(1) {
		t.Errorf("source_type_counts.kg_triples = %v, want 1", stc["kg_triples"])
	}
	tc, ok := resp["tier_counts"].(map[string]any)
	if !ok {
		t.Fatalf("tier_counts is not an object: %T", resp["tier_counts"])
	}
	if tc["memory"] != float64(2) {
		t.Errorf("tier_counts.memory = %v, want 2", tc["memory"])
	}
}

// Test 7: Disabled store returns safe fallback
func TestSearchWithNoopStoreReturnsEmptyNotError(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"user_input":"hello","chat_session_id":"sess-noop"}`
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
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
	if resp["memory_count"] != float64(0) {
		t.Errorf("memory_count = %v, want 0", resp["memory_count"])
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 8: GET /chroma-shadow/preflight matches Python 0.8 shape
func TestChromaPreflightShape(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/chroma-shadow/preflight", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["step"] != "17-C1" {
		t.Errorf("step = %v, want 17-C1", resp["step"])
	}
	if _, ok := resp["vector_health"]; ok {
		t.Errorf("vector_health should not be present in Python-aligned preflight")
	}
}

// Test 9: GET /retrieval-index/{sid}/source-row with type and id params
func TestRetrievalIndexSourceRowReturnsMemory(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 99, ChatSessionID: "sess-src", TurnIndex: 5, SummaryJSON: `{"s":"v"}`, Importance: 0.7, CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-src/source-row?document_id=memory:99", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sr, ok := resp["source_row"].(map[string]any)
	if !ok {
		t.Fatalf("source_row is not an object: %T", resp["source_row"])
	}
	if sr["type"] != "memory" {
		t.Errorf("source_row.type = %v, want memory", sr["type"])
	}
	if sr["id"] != float64(99) {
		t.Errorf("source_row.id = %v, want 99", sr["id"])
	}
}

func TestRetrievalIndexSourceRowReturnsHierarchyRows(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &memoryFakeStore{
		episodes: []store.EpisodeSummary{
			{ID: 11, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 20, SummaryText: "Episode gate", CreatedAt: now},
		},
		chapters: []store.ChapterSummary{
			{ID: 22, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate Chapter", SummaryText: "Chapter gate", CreatedAt: &now},
		},
		arcs: []store.ArcSummary{
			{ID: 33, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 240, ArcIndex: 1, ArcName: "Gate Arc", ArcStatus: "active", ArcResumeText: "Arc gate", CreatedAt: &now},
		},
		sagas: []store.SagaDigest{
			{ID: 44, ChatSessionID: "sess-hier", FromTurn: 1, ToTurn: 960, EraLabel: "Gate Era", ResumePackText: "Saga gate", CreatedAt: &now},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	cases := []struct {
		docID       string
		sourceTable string
		tier        string
	}{
		{"episode:11", "episode_summaries", "episode"},
		{"chapter:22", "chapter_summaries", "chapter"},
		{"arc:33", "arc_summaries", "arc"},
		{"saga:44", "saga_digests", "saga"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-hier/source-row?document_id="+tc.docID, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200: %s", tc.docID, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s: %v", tc.docID, err)
		}
		ref, ok := resp["source_ref"].(map[string]any)
		if !ok {
			t.Fatalf("%s source_ref is not an object: %T", tc.docID, resp["source_ref"])
		}
		if ref["source_table"] != tc.sourceTable {
			t.Errorf("%s source_table = %v, want %s", tc.docID, ref["source_table"], tc.sourceTable)
		}
		if ref["tier"] != tc.tier {
			t.Errorf("%s tier = %v, want %s", tc.docID, ref["tier"], tc.tier)
		}
	}
}

// Test 10: ErrNotEnabled store returns safe empty shapes, not 500
type notEnabledStore struct {
	memoryFakeStore
}

func (n *notEnabledStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return nil, store.ErrNotEnabled
}

func (n *notEnabledStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return nil, store.ErrNotEnabled
}

func (n *notEnabledStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return nil, store.ErrNotEnabled
}

func TestErrNotEnabledReturnsSafeFallbackNot500(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &notEnabledStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/memories?chat_session_id=sess-ne", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/memories status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/explorer/direct-evidence?chat_session_id=sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/direct-evidence status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/explorer/kg_triples?chat_session_id=sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/kg_triples status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/retrieval-index/sess-ne", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("retrieval-index status = %d, want 200", rec.Code)
	}
}

// Test 11: GET /kg/recall without session returns empty items
func TestKGRecallGetWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/kg/recall", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 13: GET /explorer/chat_logs returns Store-backed data
func TestExplorerChatLogsReturnsStoreData(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	fake := &memoryFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-cl", TurnIndex: 1, Role: "user", Content: "Hello there", CreatedAt: ts},
			{ID: 2, ChatSessionID: "sess-cl", TurnIndex: 2, Role: "assistant", Content: "Hi! How can I help you today?", CreatedAt: ts},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-cl", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	if resp["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", resp["total"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["role"] != "assistant" {
		t.Errorf("first item role = %v, want assistant", first["role"])
	}
	if first["preview"] != "Hi! How can I help you today?" {
		t.Errorf("first item preview = %v, want Hi! How can I help you today?", first["preview"])
	}
}

// Test 14: GET /explorer/chat_logs without session returns empty
func TestExplorerChatLogsWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 15: GET /explorer/chat_logs respects limit/offset pagination
func TestExplorerChatLogsLimitOffset(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	logs := make([]store.ChatLog, 5)
	for i := 0; i < 5; i++ {
		logs[i] = store.ChatLog{ID: int64(i + 1), ChatSessionID: "sess-page", TurnIndex: i + 1, Role: "user", Content: "msg", CreatedAt: ts}
	}
	fake := &memoryFakeStore{chatLogs: logs}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-page&limit=2&offset=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 2 {
		t.Errorf("items count = %d, want 2", len(items))
	}
	if resp["total"].(float64) != 5 {
		t.Errorf("total = %v, want 5", resp["total"])
	}
	if resp["has_more"] != true {
		t.Errorf("has_more = %v, want true", resp["has_more"])
	}
}

// Test 16: GET /explorer/chapter_summaries returns chapter data from Store
func TestExplorerChapterSummariesReturnsChapterData(t *testing.T) {
	fake := &memoryFakeStore{
		chapters: []store.ChapterSummary{
			{ID: 100, ChatSessionID: "sess-ch", FromTurn: 1, ToTurn: 10, ChapterIndex: 1, ChapterTitle: "Chapter One", SummaryText: "Chapter one summary", ResumeText: "Resume chapter one"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-ch", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["source"] != "chapter_summary" {
		t.Errorf("first item source = %v, want chapter_summary", first["source"])
	}
	if first["chapter_title"] != "Chapter One" {
		t.Errorf("first item chapter_title = %v, want Chapter One", first["chapter_title"])
	}
	if first["summary_text"] != "Chapter one summary" {
		t.Errorf("first item summary_text = %v, want Chapter one summary", first["summary_text"])
	}
}

func TestExplorerArcSummariesReturnsArcData(t *testing.T) {
	fake := &memoryFakeStore{
		arcs: []store.ArcSummary{
			{ID: 300, ChatSessionID: "sess-arc", FromTurn: 1, ToTurn: 40, ArcIndex: 2, ArcName: "Bridge Siege", ArcStatus: "active", CoreConflict: "Escape route is closing", ArcResumeText: "Resume arc"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/arc_summaries?chat_session_id=sess-arc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if first["source"] != "arc_summary" {
		t.Errorf("first item source = %v, want arc_summary", first["source"])
	}
	if first["arc_name"] != "Bridge Siege" {
		t.Errorf("first item arc_name = %v, want Bridge Siege", first["arc_name"])
	}
}

func TestExplorerSagaDigestsReturnsSagaData(t *testing.T) {
	fake := &memoryFakeStore{
		sagas: []store.SagaDigest{
			{ID: 400, ChatSessionID: "sess-saga", FromTurn: 1, ToTurn: 120, EraLabel: "Airport Era", SagaSummary: "The group establishes a fragile base.", ResumePackText: "Resume saga"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/saga_digests?chat_session_id=sess-saga", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	first := items[0].(map[string]any)
	if first["source"] != "saga_digest" {
		t.Errorf("first item source = %v, want saga_digest", first["source"])
	}
	if first["era_label"] != "Airport Era" {
		t.Errorf("first item era_label = %v, want Airport Era", first["era_label"])
	}
}

// Test 17: GET /explorer/chapter_summaries falls back to GetResumePack chapter
func TestExplorerChapterSummariesResumePackFallback(t *testing.T) {
	chapterTime, _ := time.Parse(time.RFC3339, "2026-02-01T00:00:00Z")
	fake := &memoryFakeStore{
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Trigger:    "resume",
			Chapter: &store.ChapterSummary{
				ID:           200,
				FromTurn:     1,
				ToTurn:       20,
				ChapterIndex: 3,
				ChapterTitle: "The Beginning",
				ResumeText:   "Characters meet",
				SummaryText:  "Full chapter summary",
				CreatedAt:    &chapterTime,
			},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-rp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 1 {
		t.Errorf("items count = %d, want 1", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first item not object: %T", items[0])
	}
	if first["source"] != "resume_pack_chapter" {
		t.Errorf("first item source = %v, want resume_pack_chapter", first["source"])
	}
	if first["chapter_title"] != "The Beginning" {
		t.Errorf("first item chapter_title = %v, want The Beginning", first["chapter_title"])
	}
}

// Test 18: GET /explorer/chapter_summaries without session returns empty
func TestExplorerChapterSummariesWithoutSessionReturnsEmpty(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["items"].([]any)
	if !ok {
		t.Fatalf("items is not an array: %T", resp["items"])
	}
	if len(items) != 0 {
		t.Errorf("items count = %d, want 0", len(items))
	}
}

// Test 19: ErrNotEnabled on chat_logs/chapter_summaries returns safe 200
type notEnabledChatStore struct {
	memoryFakeStore
}

func (n *notEnabledChatStore) ListChatLogs(ctx context.Context, sid string, from, to int) ([]store.ChatLog, error) {
	return nil, store.ErrNotEnabled
}

func (n *notEnabledChatStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	return nil, store.ErrNotEnabled
}

func (n *notEnabledChatStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return nil, store.ErrNotEnabled
}

func TestErrNotEnabledChatLogsAndChapterSummariesReturnsSafe200(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = &notEnabledChatStore{}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/explorer/chat_logs?chat_session_id=sess-ne2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/chat_logs status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/explorer/chapter_summaries?chat_session_id=sess-ne2", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explorer/chapter_summaries status = %d, want 200", rec.Code)
	}
}
