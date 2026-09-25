package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func TestSessionStitch47MariaDB(t *testing.T) {
	db, st := feedback43Database(t)
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	ctx := context.Background()
	ids := []string{"first-story", "second-story", "current-story"}
	var originalEvidence []int64
	for i, sid := range ids {
		for turn := 1; turn <= 2; turn++ {
			revision := feedback43SeedSource(t, db, sid, turn)
			result := fmt.Sprintf(`{"turn_summary":"%s scene %d","narrative_events":[{"summary":"The courier visited %s."}]}`, sid, turn, sid)
			private := i == 1 && turn == 2
			if private {
				result = `{"turn_summary":"HiddenPassword","protected_secrets":[{"secret":"HiddenPassword","evidence_excerpt":"Secret evidence"}]}`
			}
			a := &archiveStore.MemoryAdmission{ContractVersion: archiveStore.MemoryAdmissionContract, ChatSessionID: sid, SourceRevision: revision, TurnIndex: turn,
				DerivationVersion: archiveStore.MemoryAdmissionContract, ExtractorVersion: "critic.synthetic", IndexVersion: archiveStore.MemoryPublicProjectionIndex, ResultJSON: result, CreatedAt: time.Now().UTC(),
				Memory: &archiveStore.Memory{ChatSessionID: sid, TurnIndex: turn, SummaryJSON: result}}
			a.MemoryPublicProjectionExcluded = private
			a.ResultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{revision, a.DerivationVersion, a.ExtractorVersion, a.IndexVersion, result}, "\x1f"))))
			if _, err := st.(archiveStore.MemoryAdmissionWriter).CommitMemoryAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			if turn == 1 {
				quote := fmt.Sprintf("The courier carries the %s key.", sid)
				saved, err := db.Exec(`INSERT INTO direct_evidence_records(chat_session_id,evidence_text,source_turn_start,source_turn_end) VALUES(?,?,1,1)`, sid, quote)
				if err != nil {
					t.Fatal(err)
				}
				evidenceID, err := saved.LastInsertId()
				if err != nil {
					t.Fatal(err)
				}
				originalEvidence = append(originalEvidence, evidenceID)
				unit := &archiveStore.PreciseMemoryUnit{UnitID: fmt.Sprintf("11111111-2222-5333-8444-%012d", i), ContractVersion: archiveStore.PreciseMemoryUnitContract, ChatSessionID: sid, SourceTurnStart: 1, SourceTurnEnd: 1, SourceContract: "source_acceptance_observation.v1", SourceRevision: revision, SourceContentHash: strings.Repeat("a", 64), SourceRole: "assistant", SourceSpanEnd: len(quote), EvidenceExcerpt: quote, EvidenceHash: strings.Repeat("a", 64), RootEvidenceID: evidenceID, DirectEvidenceIDsJSON: fmt.Sprintf("[%d]", evidenceID), Kind: "event", PayloadJSON: fmt.Sprintf(`{"text":%q,"source_turn":1}`, quote), TruthScope: "objective", EpistemicMode: "observed", AuthorityClass: "source_observed", AdmissionState: "admitted", ReviewState: "accepted", Visibility: "public", IdempotencyKey: "same-local-key", LifecycleState: "active"}
				if _, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(ctx, unit); err != nil {
					t.Fatal(err)
				}
			}
			for _, role := range []string{"user", "assistant"} {
				if _, err := db.Exec(`INSERT INTO chat_logs(chat_session_id,turn_index,role,content) VALUES(?,?,?,?)`, sid, turn, role, fmt.Sprintf("%s %d %s", sid, turn, role)); err != nil {
					t.Fatal(err)
				}
			}
		}
		status := []string{`{"condition":"injured"}`, `{"condition":"recovering"}`, `{"condition":"recovered"}`}[i]
		if _, err := db.Exec(`INSERT INTO character_states(chat_session_id,character_name,turn_index,status_json) VALUES(?,?,?,?)`, sid, "Courier", 2, status); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO pending_threads(chat_session_id,thread_key,description,status,created_turn,source_turn) VALUES(?,?,?,?,?,?)`, sid, "bridge", "Restore bridge", []string{"open", "open", "resolved"}[i], 1, 2); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO world_rules(chat_session_id,scope,scope_name,category,`key`,value_json,source_turn) VALUES(?,?,?,?,?,?,?)", sid, "session", "", "travel", "gate", fmt.Sprintf(`"gate-%d"`, i), 2); err != nil {
			t.Fatal(err)
		}
		if sid == ids[len(ids)-1] {
			if err := st.SaveAuditLog(ctx, &archiveStore.AuditLog{ChatSessionID: sid, EventType: "source_acceptance_transition", DetailsJSON: `{"logical_turn_id":"turn-1","user_logical_turn_ids":["member-A","turn-1"]}`, CreatedAt: time.Now().UTC()}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Old-copy receipts missing on the middle segment must be repaired only in
	// the new composition, leaving the original DB exactly as it was.
	if _, err := db.Exec(`DELETE FROM memory_vector_outbox WHERE chat_session_id=?`, ids[1]); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	server := httpapi.NewServer(cfg)
	server.Store = st
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	body := `{"source_session_ids":["first-story","second-story"],"current_session_id":"current-story","operation_id":"synthetic-stitch"}`
	call := func() archiveStore.SessionStitchResult {
		t.Helper()
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest("POST", "/sessions/stitch", strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatalf("stitch failed: %d %s", rec.Code, rec.Body.String())
		}
		var result archiveStore.SessionStitchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	result := call()
	responses := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() {
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, httptest.NewRequest("POST", "/sessions/stitch", strings.NewReader(body)))
			responses <- rec
		}()
	}
	for i := 0; i < 2; i++ {
		rec := <-responses
		var repeated archiveStore.SessionStitchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &repeated); err != nil || rec.Code != 200 || repeated.MigrationID != result.MigrationID {
			t.Fatalf("concurrent retry: %s %v", rec.Body.String(), err)
		}
	}
	if len(result.Segments) != len(ids) {
		t.Fatalf("segments: %+v", result)
	}
	for i, segment := range result.Segments {
		if segment.SessionID != ids[i] || segment.Offset != i*2 || segment.ThroughTurn != (i+1)*2 {
			t.Fatalf("wrong order: %+v", segment)
		}
	}
	for i, sid := range ids {
		var excerpt, evidenceIDs, payload string
		var rootEvidence int64
		var sourceTurn int
		if err := db.QueryRow(`SELECT evidence_excerpt,direct_evidence_ids_json,root_evidence_id,source_turn_start,payload_json FROM precise_memory_units WHERE chat_session_id=? AND source_turn_start=?`, result.TargetSessionID, i*2+1).Scan(&excerpt, &evidenceIDs, &rootEvidence, &sourceTurn, &payload); err != nil {
			t.Fatal(err)
		}
		if rootEvidence == originalEvidence[i] || evidenceIDs != fmt.Sprintf("[%d]", rootEvidence) || !strings.Contains(excerpt, sid) || sourceTurn != i*2+1 {
			t.Fatalf("broken evidence mapping: %s %s %d", excerpt, evidenceIDs, sourceTurn)
		}
		var mappedQuote string
		if err := db.QueryRow(`SELECT evidence_text FROM direct_evidence_records WHERE id=? AND chat_session_id=?`, rootEvidence, result.TargetSessionID).Scan(&mappedQuote); err != nil || mappedQuote != excerpt {
			t.Fatalf("wrong citation %q %v", mappedQuote, err)
		}
		var content string
		if err := db.QueryRow(`SELECT content FROM chat_logs WHERE chat_session_id=? AND turn_index=? AND role='assistant'`, result.TargetSessionID, i*2+1).Scan(&content); err != nil || !strings.Contains(content, sid) {
			t.Fatalf("ordered source lost: %s %v", content, err)
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM chat_logs WHERE chat_session_id=?`, sid).Scan(&count); err != nil || count != 4 {
			t.Fatalf("source changed: %s %d %v", sid, count, err)
		}
	}
	var receipts int
	db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id=?`, ids[1]).Scan(&receipts)
	if receipts != 0 {
		t.Fatal("original repaired/mutated")
	}
	var state string
	if err := db.QueryRow(`SELECT status_json FROM character_states WHERE chat_session_id=? ORDER BY turn_index DESC,id DESC LIMIT 1`, result.TargetSessionID).Scan(&state); err != nil || !strings.Contains(state, "recovered") {
		t.Fatalf("old injury won: %s %v", state, err)
	}
	if err := db.QueryRow(`SELECT status FROM pending_threads WHERE chat_session_id=? AND thread_key='bridge'`, result.TargetSessionID).Scan(&state); err != nil || state != "resolved" {
		t.Fatalf("completed goal reopened: %s %v", state, err)
	}
	currentState, err := st.GetCharacterState(ctx, result.TargetSessionID, "Courier")
	if err != nil || !strings.Contains(currentState.StatusJSON, "recovered") {
		t.Fatalf("current state read: %+v %v", currentState, err)
	}
	baseline, err := st.(archiveStore.SessionRoutingBaselineStore).GetSessionRoutingBaseline(ctx, result.TargetSessionID)
	if err != nil || baseline.ImportedThroughTurn != 4 || baseline.Mode != archiveStore.SessionMigrationModeStitch {
		t.Fatalf("current chat mapping wrong: %+v %v", baseline, err)
	}
	if len(baseline.InputGroupAliases["turn-1"]) != 2 || len(baseline.SourceSessionIDs) != 1 || baseline.SourceSessionIDs[0] != ids[len(ids)-1] {
		t.Fatalf("current input identity lost %+v", baseline)
	}
	repeated := call()
	if repeated.MigrationID != result.MigrationID || repeated.TargetSessionID != result.TargetSessionID {
		t.Fatal("retry duplicated operation")
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM memories WHERE chat_session_id=?`, result.TargetSessionID).Scan(&count)
	if count != 6 {
		t.Fatalf("memory count %d", count)
	}
	vectorDocs, err := st.(archiveStore.SessionMigrationVectorStore).ListSessionMigrationVectorDocuments(ctx, result.MigrationID)
	if err != nil || len(vectorDocs) < 6 {
		t.Fatalf("vector handoff: %d %v", len(vectorDocs), err)
	}
	for _, doc := range vectorDocs {
		if doc.ChatSessionID != result.TargetSessionID {
			t.Fatal("vector escaped target")
		}
		if strings.Contains(doc.DocumentText, "HiddenPassword") {
			t.Fatal("private memory became public")
		}
	}
	// Exercise the actual reindex endpoint. Only the external embedder/vector
	// boundaries are synthetic, with payload and durable ID readback assertions.
	embeddingCalls := 0
	embedder := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected provider call %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 400)
			return
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "bad JSON", 400)
			return
		}
		raw, _ := json.Marshal(request)
		if strings.Contains(string(raw), "HiddenPassword") {
			t.Error("private memory sent to embedding")
		}
		if !strings.Contains(string(raw), "story") && !strings.Contains(string(raw), "courier") && !strings.Contains(string(raw), "gate") {
			t.Errorf("source text missing from embedding: %s", raw)
		}
		embeddingCalls++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"index":0,"embedding":[0.3,0.4]}]}`)
	}))
	defer embedder.Close()
	server.Cfg.ChromaEndpoint = "synthetic-recording-boundary"
	server.Vector = vector.NewMutationFencedStore(vector.NewFakeVectorStore())
	server.RuntimeConfig = httpapi.RuntimeConfig{EmbeddingProvider: "openai", EmbeddingEndpoint: embedder.URL + "/v1/embeddings", EmbeddingModel: "synthetic-embedding", EmbeddingAPIKey: "synthetic-only", EmbeddingTimeoutSec: 5}
	reindex := httptest.NewRecorder()
	routes.ServeHTTP(reindex, httptest.NewRequest("POST", "/sessions/migrate-reindex", strings.NewReader(fmt.Sprintf(`{"migration_id":%d}`, result.MigrationID))))
	var indexed map[string]any
	if err := json.Unmarshal(reindex.Body.Bytes(), &indexed); err != nil || reindex.Code != 200 || indexed["verification_status"] != "exact_id_parity_verified" {
		t.Fatalf("reindex: %s %v", reindex.Body.String(), err)
	}
	vectors, err := server.Vector.(vector.DocumentLister).ListDocuments(ctx, result.TargetSessionID)
	if err != nil || len(vectors) != len(vectorDocs) || embeddingCalls != len(vectorDocs) {
		t.Fatalf("index writes=%d embeds=%d wanted=%d %v", len(vectors), embeddingCalls, len(vectorDocs), err)
	}
	if _, err := db.Exec(`UPDATE character_states SET status_json='{"condition":"tampered"}' WHERE chat_session_id=? AND turn_index=?`, result.TargetSessionID, result.Segments[len(ids)-1].ThroughTurn); err != nil {
		t.Fatal(err)
	}
	actualIDs := []string{}
	for _, v := range vectors {
		actualIDs = append(actualIDs, v.ID)
	}
	if _, err := st.(archiveStore.SessionMigrationVectorParityStore).VerifySessionMigrationVectorParity(ctx, result.MigrationID, archiveStore.SessionMigrationProofOperationSourceLock, actualIDs); err == nil || !strings.Contains(err.Error(), "current_target_snapshot_drift") {
		t.Fatalf("modified target not detected: %v", err)
	}
	if _, err := db.Exec(`UPDATE character_states SET status_json='{"condition":"recovered"}' WHERE chat_session_id=? AND turn_index=?`, result.TargetSessionID, result.Segments[len(ids)-1].ThroughTurn); err != nil {
		t.Fatal(err)
	}
	bound, err := st.(archiveStore.SessionRouteBindingStore).BindSessionRoute(ctx, archiveStore.SessionRouteBindingRequest{StableCharacterID: "synthetic-character", HostChatID: "synthetic-current-chat", RequestedSessionID: result.TargetSessionID, Mode: archiveStore.SessionRouteBindingModeManualAttach})
	if err != nil || !bound.ReadbackVerified || bound.Binding.CanonicalSessionID != result.TargetSessionID {
		t.Fatalf("route not durable %+v %v", bound, err)
	}
	// Selecting the combined session again must retain the original Host tail
	// boundary, even though the current DB now contains older segments too.
	body = fmt.Sprintf(`{"source_session_ids":["first-story"],"current_session_id":%q,"operation_id":"nested-stitch"}`, result.TargetSessionID)
	nested := call()
	nestedBaseline, err := st.(archiveStore.SessionRoutingBaselineStore).GetSessionRoutingBaseline(ctx, nested.TargetSessionID)
	if err != nil || nestedBaseline.ImportedThroughTurn != result.CurrentOffset+result.Segments[0].ThroughTurn || nestedBaseline.SourceSessionID != ids[len(ids)-1] {
		t.Fatalf("nested host mapping %+v %v", nestedBaseline, err)
	}
	if len(nestedBaseline.InputGroupAliases["turn-1"]) != 2 || len(nestedBaseline.SourceSessionIDs) != 2 || nestedBaseline.SourceSessionIDs[0] != result.TargetSessionID {
		t.Fatalf("nested input identity lost %+v", nestedBaseline)
	}
	// A future ordinary session copy must also accept the composed source.
	if _, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: result.TargetSessionID, TargetSessionID: "copy-of-stitched", Mode: archiveStore.SessionMigrationModeCopyKeepSource}); err != nil {
		t.Fatalf("copy of stitch: %v", err)
	}
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: result.TargetSessionID, TurnIndex: result.CurrentOffset + 1, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatalf("current tail deletion: %v", err)
	}
	var retained int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE chat_session_id=?`, result.TargetSessionID).Scan(&retained); err != nil || retained != result.CurrentOffset {
		t.Fatalf("imported memories lost after current deletion %d %v", retained, err)
	}
	for _, sid := range ids {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM chat_logs WHERE chat_session_id=?`, sid).Scan(&count); err != nil || count != 4 {
			t.Fatalf("original changed during rollback %s %d %v", sid, count, err)
		}
	}
	// A source may retain derived memories after its raw transcript is removed.
	// The continuation boundary lives in the operation, not surviving raw rows.
	if _, err := db.Exec(`DELETE FROM chat_logs WHERE chat_session_id=?`, result.TargetSessionID); err != nil {
		t.Fatal(err)
	}
	withoutRaw, err := st.(archiveStore.SessionRoutingBaselineStore).GetSessionRoutingBaseline(ctx, result.TargetSessionID)
	if err != nil || withoutRaw.ImportedThroughTurn != result.CurrentOffset {
		t.Fatalf("lost durable offset without raw rows: %+v %v", withoutRaw, err)
	}
	var migrationsBefore, migrationsAfter int
	if err := db.QueryRow(`SELECT COUNT(*) FROM session_migrations`).Scan(&migrationsBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE memory_source_revisions SET derived_result_json=NULL WHERE chat_session_id=? AND turn_index=1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	failed := httptest.NewRecorder()
	routes.ServeHTTP(failed, httptest.NewRequest("POST", "/sessions/stitch", strings.NewReader(`{"source_session_ids":["first-story"],"current_session_id":"current-story","operation_id":"corrupt-copy"}`)))
	if failed.Code != 500 || !strings.Contains(failed.Body.String(), "committed derived result is invalid") {
		t.Fatalf("expected retained-result error %s", failed.Body.String())
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM session_migrations`).Scan(&migrationsAfter); err != nil || migrationsAfter != migrationsBefore {
		t.Fatalf("failed operation retained partial migration: %d -> %d %v", migrationsBefore, migrationsAfter, err)
	}
}
