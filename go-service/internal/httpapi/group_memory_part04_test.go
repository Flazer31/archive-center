package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestExplorerPatchEvidenceRevalidateCommitsGateAndClearsRepair(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{
				ID:                  10,
				ChatSessionID:       "sess-edit",
				ArchiveState:        "repair_queue",
				CaptureVerification: "needs_review",
				CommittedGate:       "recovery",
				RepairNeeded:        true,
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := []byte(`{"chat_session_id":"sess-edit","review_note":"revalidated by operator"}`)
	req := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/10/revalidate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.updatedEvidence) != 1 {
		t.Fatalf("updatedEvidence len = %d, want 1", len(fake.updatedEvidence))
	}
	ev := fake.evidenceItems[0]
	if ev.CaptureVerification != "verified" || ev.ArchiveState != "committed" || ev.CommittedGate != "manual_revalidate" || ev.RepairNeeded {
		t.Fatalf("revalidated evidence = %#v, want verified/committed/manual_revalidate/not repair-needed", ev)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_edit audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_edit" || audit.TargetType != "direct_evidence" || audit.TargetID != 10 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	for _, needle := range []string{"revalidate", "manual_revalidate", "revalidated by operator", "changed_at"} {
		if !strings.Contains(audit.DetailsJSON, needle) {
			t.Fatalf("audit details missing %q: %s", needle, audit.DetailsJSON)
		}
	}
}

func TestExplorerPatchEvidenceTombstoneAndSupersedeWriteAudit(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{ID: 11, ChatSessionID: "sess-edit", ArchiveState: "verified_direct", CaptureVerification: "verified"},
			{ID: 12, ChatSessionID: "sess-edit", ArchiveState: "verified_direct", CaptureVerification: "verified"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	tombstoneReq := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/11/tombstone", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","review_note":"deleted turn rollback"}`)))
	tombstoneReq.Header.Set("Content-Type", "application/json")
	tombstoneRec := httptest.NewRecorder()
	mux.ServeHTTP(tombstoneRec, tombstoneReq)
	if tombstoneRec.Code != http.StatusOK {
		t.Fatalf("tombstone status = %d, want 200: %s", tombstoneRec.Code, tombstoneRec.Body.String())
	}
	if !fake.evidenceItems[0].Tombstoned || fake.evidenceItems[0].ArchiveState != "tombstoned" {
		t.Fatalf("tombstoned evidence = %#v, want tombstoned archive state", fake.evidenceItems[0])
	}

	supersedeReq := httptest.NewRequest(http.MethodPatch, "/explorer/direct-evidence/12/supersede", bytes.NewReader([]byte(`{"chat_session_id":"sess-edit","superseded_by_id":11,"review_note":"newer fact wins"}`)))
	supersedeReq.Header.Set("Content-Type", "application/json")
	supersedeRec := httptest.NewRecorder()
	mux.ServeHTTP(supersedeRec, supersedeReq)
	if supersedeRec.Code != http.StatusOK {
		t.Fatalf("supersede status = %d, want 200: %s", supersedeRec.Code, supersedeRec.Body.String())
	}
	if fake.evidenceItems[1].SupersededByID != 11 {
		t.Fatalf("superseded_by_id = %d, want 11", fake.evidenceItems[1].SupersededByID)
	}
	if len(fake.updatedEvidence) != 2 {
		t.Fatalf("updatedEvidence len = %d, want 2", len(fake.updatedEvidence))
	}
	if len(fake.auditLogs) < 2 {
		t.Fatalf("auditLogs len = %d, want >= 2", len(fake.auditLogs))
	}
	combined := fake.auditLogs[0].DetailsJSON + "\n" + fake.auditLogs[1].DetailsJSON
	for _, needle := range []string{"tombstone", "supersede", "superseded_by_id", "changed_at"} {
		if !strings.Contains(combined, needle) {
			t.Fatalf("combined audit details missing %q: %s", needle, combined)
		}
	}
}

func TestExplorerDeleteMemoryWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{
				ID:            42,
				ChatSessionID: "sess-delete",
				TurnIndex:     3,
				SummaryJSON:   `{"summary":"delete me"}`,
				Importance:    0.72,
				PlaceWing:     "wing",
				PlaceRoom:     "room",
				CreatedAt:     time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			},
			{ID: 42, ChatSessionID: "other-session", SummaryJSON: `{"summary":"keep me"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vec := &fakeVectorStore{}
	srv.Vector = vec

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/explorer/memories/42/delete?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedMemoryID != 42 {
		t.Fatalf("deletedMemoryID = %d, want 42", fake.deletedMemoryID)
	}
	if len(fake.memories) != 1 || fake.memories[0].ChatSessionID != "other-session" {
		t.Fatalf("memory delete was not session-scoped: %#v", fake.memories)
	}
	if len(vec.deleteDocIDs) != 1 || vec.deleteDocIDs[0] != "memory:sess-delete:42" {
		t.Fatalf("vector delete IDs = %#v, want memory:sess-delete:42", vec.deleteDocIDs)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "memory" || audit.TargetID != 42 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "delete me") || !strings.Contains(audit.DetailsJSON, "memory:sess-delete:42") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
	cleanup, ok := resp["vector_cleanup"].(map[string]any)
	if !ok || cleanup["attempted"] != true || cleanup["ok"] != true || cleanup["deleted_ids"] != float64(1) {
		t.Fatalf("response missing vector cleanup proof: %#v", resp)
	}
}

func TestExplorerDeleteDirectEvidenceWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		evidenceItems: []store.DirectEvidence{
			{
				ID:                  51,
				ChatSessionID:       "sess-delete",
				EvidenceKind:        "turn_excerpt",
				EvidenceText:        "delete this evidence",
				ArchiveState:        "verified_direct",
				CaptureVerification: "verified",
				TurnAnchor:          4,
				SourceTurnStart:     4,
				SourceTurnEnd:       4,
				CreatedAt:           time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			},
			{ID: 51, ChatSessionID: "other-session", EvidenceText: "keep this evidence"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/explorer/direct-evidence/51/delete?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedEvidenceID != 51 {
		t.Fatalf("deletedEvidenceID = %d, want 51", fake.deletedEvidenceID)
	}
	if len(fake.evidenceItems) != 1 || fake.evidenceItems[0].ChatSessionID != "other-session" {
		t.Fatalf("direct evidence delete was not session-scoped: %#v", fake.evidenceItems)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "direct_evidence" || audit.TargetID != 51 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "delete this evidence") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
}

func TestExplorerDeleteKGTripleWritesAuditAndScopesSession(t *testing.T) {
	fake := &memoryFakeStore{
		kgTriples: []store.KGTriple{
			{ID: 7, ChatSessionID: "sess-delete", Subject: "A", Predicate: "knows", Object: "B"},
			{ID: 7, ChatSessionID: "other-session", Subject: "Other", Predicate: "keeps", Object: "B"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/explorer/kg_triples/7?chat_session_id=sess-delete", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedKGID != 7 {
		t.Fatalf("deletedKGID = %d, want 7", fake.deletedKGID)
	}
	if len(fake.kgTriples) != 1 || fake.kgTriples[0].ChatSessionID != "other-session" {
		t.Fatalf("KG delete was not session-scoped: %#v", fake.kgTriples)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "kg_triple" || audit.TargetID != 7 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "changed_at") || !strings.Contains(audit.DetailsJSON, "knows") {
		t.Fatalf("audit details missing deletion history: %s", audit.DetailsJSON)
	}
}

func TestSeq123P97CanonicalToVectorDriftMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p97", TurnIndex: 1, SummaryJSON: "{\"summary\":\"m1\"}"},
			{ID: 2, ChatSessionID: "sess-p97", TurnIndex: 2, SummaryJSON: "{\"summary\":\"m2\"}"},
		},
		evidenceItems: []store.DirectEvidence{
			{ID: 10, ChatSessionID: "sess-p97"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = &fakeVectorStore{countResult: 1}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/visibility-guard", strings.NewReader("{\"chat_session_id\":\"sess-p97\"}"))
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
	ev := resp["evidence"].(map[string]any)
	if ev["drift_policy"] != "shadow_degraded" {
		t.Fatalf("drift_policy = %v, want shadow_degraded", ev["drift_policy"])
	}
	if ev["drift_status"] != "drift_detected" {
		t.Fatalf("drift_status = %v, want drift_detected", ev["drift_status"])
	}
	if ev["canonical_count"] != float64(3) {
		t.Fatalf("canonical_count = %v, want 3", ev["canonical_count"])
	}
	if ev["canonical_to_vector_gap"] != float64(2) {
		t.Fatalf("canonical_to_vector_gap = %v, want 2", ev["canonical_to_vector_gap"])
	}
	if ev["drift_action"] != "keep_canonical_baseline" {
		t.Fatalf("drift_action = %v, want keep_canonical_baseline", ev["drift_action"])
	}
}

func TestSeq123P98FallbackFailOpenDegradedVocabularyMarkers(t *testing.T) {
	fake := &memoryFakeStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.Vector = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/fallback-runbook", strings.NewReader("{\"chat_session_id\":\"sess-p98\"}"))
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
	ev := resp["evidence"].(map[string]any)
	if ev["fallback_policy"] != "store_first_then_vector" {
		t.Fatalf("fallback_policy = %v, want store_first_then_vector", ev["fallback_policy"])
	}
	if ev["degraded_mode"] != "canonical_baseline" {
		t.Fatalf("degraded_mode = %v, want canonical_baseline", ev["degraded_mode"])
	}
	if ev["fail_open_baseline"] != true {
		t.Fatalf("fail_open_baseline = %v, want true", ev["fail_open_baseline"])
	}
	if ev["retrieval_baseline"] != "sqlite_canonical" {
		t.Fatalf("retrieval_baseline = %v, want sqlite_canonical", ev["retrieval_baseline"])
	}
}

func TestSeq123P99RetrievalObservabilityMarkers(t *testing.T) {
	fake := &memoryFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-p99", TurnIndex: 1, SummaryJSON: "{\"summary\":\"memory only\"}"},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-p99", TurnIndex: 1, Role: "user", Content: "fallback log"},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader("{\"chat_session_id\":\"sess-p99\",\"user_input\":\"memory\",\"top_k\":5}"))
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
	if resp["observability_status"] != "shadow_r1" {
		t.Fatalf("observability_status = %v, want shadow_r1", resp["observability_status"])
	}
	if _, ok := resp["fallback_rate_metric"]; !ok {
		t.Fatal("missing fallback_rate_metric")
	}
	if resp["stale_hit_metric"] != float64(0) {
		t.Fatalf("stale_hit_metric = %v, want 0", resp["stale_hit_metric"])
	}
	if resp["no_candidate_metric"] != float64(0) {
		t.Fatalf("no_candidate_metric = %v, want 0", resp["no_candidate_metric"])
	}
	if resp["hydration_miss_metric"] != float64(0) {
		t.Fatalf("hydration_miss_metric = %v, want 0", resp["hydration_miss_metric"])
	}
}

func TestSeq123P100MultiTierLiveCutoverPrerequisiteMarkers(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chroma-shadow/adoption-gate", strings.NewReader("{\"chat_session_id\":\"sess-p100\"}"))
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
	if resp["live_cutover_allowed"] != false {
		t.Fatalf("live_cutover_allowed = %v, want false", resp["live_cutover_allowed"])
	}
	prereq, ok := resp["cutover_prerequisites"].([]any)
	if !ok || len(prereq) == 0 {
		t.Fatalf("missing cutover_prerequisites: %#v", resp["cutover_prerequisites"])
	}
	gates, ok := resp["required_green_gates"].([]any)
	if !ok || len(gates) == 0 {
		t.Fatalf("missing required_green_gates: %#v", resp["required_green_gates"])
	}
	if resp["multi_tier_cutover_scope"] != "memory_only" {
		t.Fatalf("multi_tier_cutover_scope = %v, want memory_only", resp["multi_tier_cutover_scope"])
	}
	if resp["adoption_gate_state"] != "closed" {
		t.Fatalf("adoption_gate_state = %v, want closed", resp["adoption_gate_state"])
	}
}
