package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
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

func TestReport47MissingProjectionRecoveryHTTPMariaDB(t *testing.T) {
	for _, kind := range []string{"public", "private", "mixed", "corrupt_result"} {
		t.Run(kind, func(t *testing.T) {
			db, st := feedback43Database(t)
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			ctx := context.Background()
			const sid = "old-copy-missing-receipts"
			revision := state46Source(t, db, sid, 1)
			result := `{"turn_summary":"The public gate opened."}`
			if kind == "private" || kind == "mixed" {
				result = `{"turn_summary":"HiddenPassword","protected_secrets":[{"secret":"HiddenPassword","evidence_excerpt":"Secret evidence"}]`
				if kind == "mixed" {
					result += `,"narrative_events":[{"summary":"The public gate opened."}]`
				}
				result += `}`
			}
			a := &archiveStore.MemoryAdmission{
				ContractVersion: archiveStore.MemoryAdmissionContract, ChatSessionID: sid, SourceRevision: revision, TurnIndex: 1,
				DerivationVersion: archiveStore.MemoryAdmissionContract, ExtractorVersion: "critic.synthetic", IndexVersion: archiveStore.MemoryPublicProjectionIndex,
				ResultJSON: result, CreatedAt: time.Now().UTC(), Memory: &archiveStore.Memory{ChatSessionID: sid, TurnIndex: 1, SummaryJSON: result},
				MemoryPublicProjectionExcluded: kind == "private",
			}
			a.ResultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{revision, a.DerivationVersion, a.ExtractorVersion, a.IndexVersion, result}, "\x1f"))))
			if _, err := st.(archiveStore.MemoryAdmissionWriter).CommitMemoryAdmission(ctx, a); err != nil {
				t.Fatal(err)
			}
			// This is the persisted defect in older copied sessions: the canonical
			// source survives, but its public upsert/delete receipt is absent.
			if _, err := db.Exec(`DELETE FROM memory_vector_outbox WHERE chat_session_id=?`, sid); err != nil {
				t.Fatal(err)
			}
			_, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{
				SourceSessionID: sid, TargetSessionID: "negative-control", Mode: archiveStore.SessionMigrationModeCopyKeepSource,
			})
			if err == nil || !strings.Contains(err.Error(), "public projection authority is missing") {
				t.Fatalf("did not reproduce old failure: %v", err)
			}
			if kind == "corrupt_result" {
				second := *a
				second.SourceRevision = state46Source(t, db, sid, 2)
				second.TurnIndex = 2
				second.Memory = &archiveStore.Memory{ChatSessionID: sid, TurnIndex: 2, SummaryJSON: result}
				second.ResultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{second.SourceRevision, second.DerivationVersion, second.ExtractorVersion, second.IndexVersion, result}, "\x1f"))))
				if _, err := st.(archiveStore.MemoryAdmissionWriter).CommitMemoryAdmission(ctx, &second); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE memory_source_revisions SET derived_result_json=NULL WHERE source_revision=?`, second.SourceRevision); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeMariaDBAuthority
			server := httpapi.NewServer(cfg)
			server.Store = st
			routes := http.NewServeMux()
			server.RegisterRoutes(routes)
			copy := func(from, to string) *httptest.ResponseRecorder {
				t.Helper()
				rec := httptest.NewRecorder()
				body := fmt.Sprintf(`{"source_session_id":%q,"target_session_id":%q,"mode":%q}`, from, to, archiveStore.SessionMigrationModeCopyKeepSource)
				routes.ServeHTTP(rec, httptest.NewRequest("POST", "/sessions/migrate-complete", strings.NewReader(body)))
				return rec
			}
			rec := copy(sid, "repaired-copy")
			if kind == "corrupt_result" {
				if rec.Code != 500 || !strings.Contains(rec.Body.String(), "committed derived result is invalid") {
					t.Fatalf("corruption concealed: %d %s", rec.Code, rec.Body.String())
				}
				var count int
				if err := db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id IN (?,?)`, sid, "repaired-copy").Scan(&count); err != nil || count != 0 {
					t.Fatalf("failed transaction retained repair: %d %v", count, err)
				}
				return
			}
			if rec.Code != 200 {
				t.Fatalf("repair HTTP: %d %s", rec.Code, rec.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response["write_attempted"] != true || response["blocked"] == true {
				t.Fatalf("no copy: %s %v", rec.Body.String(), err)
			}
			for _, session := range []string{sid, "repaired-copy"} {
				var summary, retained, operation, document, status string
				if err := db.QueryRow(`SELECT summary_json FROM memories WHERE chat_session_id=?`, session).Scan(&summary); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRow(`SELECT derived_result_json FROM memory_source_revisions WHERE chat_session_id=?`, session).Scan(&retained); err != nil {
					t.Fatal(err)
				}
				if summary != result || retained != result {
					t.Fatal("canonical extraction was rewritten")
				}
				if err := db.QueryRow(`SELECT operation,COALESCE(document_json,''),status FROM memory_vector_outbox WHERE chat_session_id=?`, session).Scan(&operation, &document, &status); err != nil {
					t.Fatal(err)
				}
				if status != "completed" || strings.Contains(document, "HiddenPassword") {
					t.Fatalf("unsafe or queued receipt: %s %s", status, document)
				}
				if kind == "private" {
					if operation != "delete" {
						t.Fatal("private memory became public")
					}
				} else if operation != "upsert" || !strings.Contains(document, "public gate") {
					t.Fatalf("public memory lost: %s %s", operation, document)
				}
			}
			for _, pair := range [][2]string{{sid, "repaired-copy"}, {"repaired-copy", "repaired-again"}} {
				if next := copy(pair[0], pair[1]); next.Code != 200 || strings.Contains(next.Body.String(), `"blocked":true`) {
					t.Fatalf("retry/recopy failed: %s", next.Body.String())
				}
			}
			var migrationID int64
			if err := db.QueryRow(`SELECT id FROM session_migrations WHERE target_session_id='repaired-again'`).Scan(&migrationID); err != nil {
				t.Fatal(err)
			}
			// The store candidate is canonical. The production reindex HTTP owner
			// projects it before the external embedding/vector boundaries.
			embeddingCalls := 0
			embedder := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/v1/embeddings" {
					t.Errorf("unexpected provider call: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 400)
					return
				}
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					http.Error(w, "bad request", 400)
					return
				}
				payload, _ := json.Marshal(request)
				if strings.Contains(string(payload), "HiddenPassword") || !strings.Contains(string(payload), "public gate") {
					t.Errorf("incorrect embedding payload: %s", payload)
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
			routes.ServeHTTP(reindex, httptest.NewRequest("POST", "/sessions/migrate-reindex", strings.NewReader(fmt.Sprintf(`{"migration_id":%d}`, migrationID))))
			var reindexed map[string]any
			if err := json.Unmarshal(reindex.Body.Bytes(), &reindexed); err != nil || reindex.Code != 200 || reindexed["verification_status"] != "exact_id_parity_verified" {
				t.Fatalf("reindex failed: %s %v", reindex.Body.String(), err)
			}
			vectors, err := server.Vector.(vector.DocumentLister).ListDocuments(ctx, "repaired-again")
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if kind == "private" {
				want = 0
			}
			if len(vectors) != want || embeddingCalls != want {
				t.Fatalf("boundary writes=%d embeddings=%d expected=%d", len(vectors), embeddingCalls, want)
			}
			var ids []string
			for _, v := range vectors {
				ids = append(ids, v.ID)
				if strings.Contains(v.DocumentText, "HiddenPassword") || !strings.Contains(v.DocumentText, "public gate") {
					t.Fatalf("incorrect final vector: %#v", v)
				}
			}
			parity, err := st.(archiveStore.SessionMigrationVectorParityStore).VerifySessionMigrationVectorParity(ctx, migrationID, archiveStore.SessionMigrationProofOperationSourceLock, ids)
			if err != nil || !parity.Verified {
				t.Fatalf("repaired-copy parity: %#v %v", parity, err)
			}
			if _, err := st.(archiveStore.SessionMigrationRecoveryStore).RollbackSessionMigration(ctx, migrationID, "synthetic repair rollback"); err != nil {
				t.Fatal(err)
			}
			var left int
			if err := db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id='repaired-again'`).Scan(&left); err != nil || left != 0 {
				t.Fatalf("rollback left receipts: %d %v", left, err)
			}
		})
	}
}

func TestReport47AuditUnknownTargetMariaDB(t *testing.T) {
	db, st := feedback43Database(t)
	// MariaDB rejects the former literal sentinel, reproducing the report code.
	_, err := db.Exec(`INSERT INTO audit_logs(event_type,target_id) VALUES('negative-control',-1)`)
	if err == nil || !strings.Contains(err.Error(), "1264") {
		t.Fatalf("expected unsigned target overflow: %v", err)
	}
	for _, id := range []int64{-1, 0, 42, 1 << 40} {
		event := fmt.Sprintf("audit-case-%d", id)
		if err := st.SaveAuditLog(context.Background(), &archiveStore.AuditLog{EventType: event, TargetType: "turn", TargetID: id, DetailsJSON: fmt.Sprintf(`{"turn_index":%d}`, id)}); err != nil {
			t.Fatal(err)
		}
		var got sql.NullInt64
		var details string
		if err := db.QueryRow(`SELECT target_id,details_json FROM audit_logs WHERE event_type=?`, event).Scan(&got, &details); err != nil {
			t.Fatal(err)
		}
		if (id < 0 && got.Valid) || (id >= 0 && (!got.Valid || got.Int64 != id)) || !strings.Contains(details, fmt.Sprint(id)) {
			t.Fatalf("audit data lost: %v %s", got, details)
		}
	}
}

func TestReport47LorebookAggregateWithoutModuleConsentHTTPMariaDB(t *testing.T) {
	_, st := feedback43Database(t)
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	server := httpapi.NewServer(cfg)
	server.Store = st
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	post := func(path, body string, code int) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if rec.Code != code {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, moduleObservation := range []bool{false, true} {
		scope := fmt.Sprintf(`"character_index":1,"chat_index":2,"enabled_module_ids":[],"enabled_modules_observed":%t`, moduleObservation)
		post("/sessions/aggregate-lore/lorebook-reference/snapshots", `{"contract_version":"lorebook_reference_snapshot.v1","consent_state":"active","observation_state":"observed","complete_snapshot":true,`+scope+`,"entries":[{"id":"gate","key":"gate","content":"The gate opens at dawn.","alwaysActive":true}]}`, 201)
		result := post("/prepare-turn", `{"chat_session_id":"aggregate-lore","raw_user_input":"Visit the gate.","response_projection":"prepare_turn.production_compact.v1","lorebook_reference_scope":{"contract_version":"lorebook_reference_scope.v1","observation_state":"observed",`+scope+`},"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist","lorebook_reference_char_budget":3000}}`, 200)
		encoded, _ := json.Marshal(result["injection_pack"])
		lorebook, _ := result["lorebook_reference"].(map[string]any)
		if lorebook["delivery_count"] != float64(1) || !strings.Contains(string(encoded), "The gate opens at dawn.") {
			t.Fatalf("stored catalog was not delivered: %s", encoded)
		}
		post("/sessions/aggregate-lore/lorebook-reference/snapshots", `{"contract_version":"lorebook_reference_snapshot.v1","consent_state":"active","observation_state":"observed","complete_snapshot":true,`+scope+`,"entries":[]}`, 201)
		character, chat := int64(1), int64(2)
		current, err := st.(archiveStore.LorebookReferenceStore).GetLorebookReferenceCurrent(context.Background(), archiveStore.LorebookReferenceScope{ChatSessionID: "aggregate-lore", CharacterIndex: &character, ChatIndex: &chat, EnabledModulesObserved: moduleObservation})
		if err != nil || len(current.Entries) != 0 {
			t.Fatalf("complete empty catalog did not clear prior entries: %#v %v", current, err)
		}
	}
}
