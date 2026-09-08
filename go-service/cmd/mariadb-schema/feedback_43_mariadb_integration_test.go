package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// The test owns a newly created database on an explicitly disposable server.
// It never reads an installation's settings or connects to the default DB.
func feedback43Database(t *testing.T) (*sql.DB, archiveStore.Store) {
	t.Helper()
	if os.Getenv("AC_FEEDBACK_TEST_DISPOSABLE") != "YES" {
		t.Skip("requires an explicitly disposable MariaDB server")
	}
	cfg, err := mysql.ParseDSN(os.Getenv("AC_FEEDBACK_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBName != "" {
		t.Fatal("test DSN must name no existing database")
	}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("archive_center_feedback_43_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name + " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.DBName, cfg.ParseTime = name, true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	st, err := archiveStore.OpenMariaDB(cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		st.(interface{ Close() error }).Close()
		db.Close()
		if _, err := admin.Exec("DROP DATABASE " + name); err != nil {
			t.Errorf("drop owned test database: %v", err)
		}
		admin.Close()
	})
	schema := filepath.Join("..", "..", "..", "migrations", "001_schema.sql")
	statements, _, err := loadMigrationStatements(schema)
	if err != nil {
		t.Fatal(err)
	}
	report := newReport(schema, true)
	if err := applyStatements(context.Background(), db, statements, report); err != nil {
		t.Fatal(err)
	}
	if err := applyCompatibilityMigrations(context.Background(), db, report); err != nil {
		t.Fatal(err)
	}
	return db, st
}

func feedback43SeedSource(t *testing.T, exec sqlExecer, sid string, turn int) string {
	t.Helper()
	revision := fmt.Sprintf("%s-revision-%d", sid, turn)
	_, err := exec.ExecContext(context.Background(), `INSERT INTO memory_source_revisions
		(source_revision, chat_session_id, logical_turn_id, turn_index, raw_user_content,
		 raw_assistant_content, combined_content_hash, hash_algorithm, host_observed_at_ms)
		VALUES (?, ?, ?, ?, 'user original', 'assistant original', ?, 'sha256', 1)`,
		revision, sid, fmt.Sprintf("turn-%d", turn), turn, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	return revision
}

func TestFeedback43BulkSessionDeleteMariaDBIntegration(t *testing.T) {
	db, st := feedback43Database(t)
	const sessions, revisions, documents = 3, 112, 4000
	ctx := context.Background()
	var live vector.VectorStore
	var server *httpapi.Server
	var routes *http.ServeMux
	var allDocumentIDs []string
	if endpoint := os.Getenv("AC_FEEDBACK_TEST_CHROMA"); endpoint != "" {
		var err error
		live, err = vector.NewChromaStore(endpoint, fmt.Sprintf("feedback_43_%d", time.Now().UnixNano()), "/api/v2")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := live.(vector.CollectionResetter).ResetAll(context.Background()); err != nil {
				t.Error(err)
			}
		})
		server = httpapi.NewServer(config.Config{})
		server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
		server.Store, server.Vector = st, live
		server.RuntimeConfig = httpapi.RuntimeConfig{Synced: true, CriticTimeoutSec: 60, EmbeddingTimeoutSec: 30}
		routes = http.NewServeMux()
		server.RegisterRoutes(routes)
	}
	for session := 0; session < sessions; session++ {
		sid := fmt.Sprintf("bulk-%d", session)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for turn := 1; turn <= revisions; turn++ {
			feedback43SeedSource(t, tx, sid, turn)
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO memory_vector_outbox
			(operation_key, operation, chat_session_id, source_revision, document_id,
			 document_json, embedding_ready, required_source_state, status)
			VALUES (?, 'upsert', ?, ?, ?, '{}', TRUE, 'active', 'completed')`)
		if err != nil {
			t.Fatal(err)
		}
		var vectorDocuments []vector.VectorDocument
		for doc := 0; doc < documents; doc++ {
			documentID := fmt.Sprintf("precise_memory:%s:%d", sid, doc)
			allDocumentIDs = append(allDocumentIDs, documentID)
			if live != nil {
				vectorDocuments = append(vectorDocuments, vector.VectorDocument{ID: documentID, ChatSessionID: sid,
					Tier: "precise_memory", SourceTable: "precise_memory_units", SourceRowID: fmt.Sprint(doc),
					DocumentText: "Disposable deletion integration document", Embedding: []float32{0.1, 0.2, 0.3}})
			}
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(documentID)))
			if _, err := stmt.ExecContext(ctx, key, sid, fmt.Sprintf("%s-revision-%d", sid, doc%revisions+1), documentID); err != nil {
				t.Fatal(err)
			}
		}
		stmt.Close()
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		if live != nil {
			if err := live.Upsert(ctx, sid, vectorDocuments); err != nil {
				t.Fatal(err)
			}
		}
	}
	feedback43SeedSource(t, db, "untouched", 1)
	if live != nil {
		if err := live.Upsert(ctx, "untouched", []vector.VectorDocument{{ID: "untouched-vector", ChatSessionID: "untouched", Tier: "memory", DocumentText: "preserve", Embedding: []float32{0.1, 0.2, 0.3}}}); err != nil {
			t.Fatal(err)
		}
	}
	for session := 0; session < sessions; session++ {
		sid := fmt.Sprintf("bulk-%d", session)
		started := time.Now()
		if routes != nil {
			response := httptest.NewRecorder()
			routes.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/sessions/"+sid+"?req_source=timeline_manual_delete", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"vector_cleanup":"queued"`) {
				t.Fatalf("delete route did not commit and queue: %d %s", response.Code, response.Body.String())
			}
		} else if err := st.(archiveStore.RollbackStore).DeleteSession(ctx, sid); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d revisions / %d documents delete=%s", sid, revisions, documents, time.Since(started))
		var count, distinct int
		if err := db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT document_id) FROM memory_vector_outbox WHERE chat_session_id=? AND operation='delete'`, sid).Scan(&count, &distinct); err != nil {
			t.Fatal(err)
		}
		if count != documents || distinct != documents {
			t.Fatalf("delete fanout: rows=%d distinct=%d want=%d", count, distinct, documents)
		}
		if err := st.(archiveStore.RollbackStore).DeleteSession(ctx, sid); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM memory_vector_outbox WHERE chat_session_id=? AND operation='delete'`, sid).Scan(&count); err != nil || count != documents {
			t.Fatalf("repeat delete rows=%d err=%v", count, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM memory_source_revisions WHERE chat_session_id=? AND (lifecycle_state<>'deleted' OR raw_user_content<>'' OR raw_assistant_content<>'')`, sid).Scan(&count); err != nil || count != 0 {
			t.Fatalf("source deletion incomplete: %d %v", count, err)
		}
	}
	var untouched int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_source_revisions WHERE chat_session_id='untouched' AND lifecycle_state='active'`).Scan(&untouched); err != nil || untouched != 1 {
		t.Fatalf("unrelated session changed: %d %v", untouched, err)
	}
	if live != nil {
		// Model a process stopping after leasing; the real worker must reclaim
		// the expired lease and perform deletion/readback/completion itself.
		if _, err := st.(archiveStore.MemoryVectorOutboxStore).ClaimMemoryVectorOperations(ctx, "interrupted-worker", time.Now().UTC(), time.Millisecond); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
		workerCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		started := time.Now()
		if !server.StartMemoryWorkers(workerCtx) {
			t.Fatal("production worker did not start")
		}
		for {
			var completed int
			if err := db.QueryRowContext(workerCtx, `SELECT COUNT(*) FROM memory_vector_outbox WHERE operation='delete' AND status='completed'`).Scan(&completed); err != nil {
				t.Fatal(err)
			}
			if completed == sessions*documents {
				break
			}
			select {
			case <-workerCtx.Done():
				t.Fatalf("worker completed %d/%d: %v", completed, sessions*documents, workerCtx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}
		cancel()
		remaining, err := live.(vector.ExactDocumentReader).GetDocuments(ctx, allDocumentIDs)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("deleted Chroma documents remain: %d %v", len(remaining), err)
		}
		kept, err := live.(vector.ExactDocumentReader).GetDocuments(ctx, []string{"untouched-vector"})
		if err != nil || len(kept) != 1 {
			t.Fatalf("unrelated Chroma document changed: %d %v", len(kept), err)
		}
		t.Logf("production worker deleted and read back %d Chroma documents after expired lease in %s", sessions*documents, time.Since(started))
	}
}

func TestFeedback43HistoricalDeleteBacklogMariaDBIntegration(t *testing.T) {
	db, st := feedback43Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	revision := feedback43SeedSource(t, db, "historical-backlog", 1)
	if _, err := db.ExecContext(ctx, `UPDATE memory_source_revisions SET lifecycle_state='deleted' WHERE source_revision=?`, revision); err != nil {
		t.Fatal(err)
	}
	documents, copies := 1000, 112
	if os.Getenv("AC_FEEDBACK_TEST_LARGE") == "YES" {
		documents = 10000
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO memory_vector_outbox
		(operation_key, operation, chat_session_id, source_revision, document_id,
		 document_json, embedding_ready, required_source_state, status)
		SELECT SHA2(CONCAT('old-', d.seq, '-', c.seq), 256), 'delete', 'historical-backlog', ?,
		       CONCAT('old-document-', d.seq), '{}', TRUE, 'inactive', 'pending'
		FROM seq_1_to_%d d CROSS JOIN seq_1_to_%d c`, documents, copies), revision); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	coalesced, err := st.(archiveStore.MemoryVectorOutboxMaintenanceStore).CoalesceInactiveMemoryVectorDeleteOperations(ctx, "historical-backlog", time.Now().UTC())
	if err != nil {
		t.Fatalf("coalesced=%d elapsed=%s: %v", coalesced, time.Since(started), err)
	}
	if coalesced != int64(documents*(copies-1)) {
		t.Fatalf("coalesced=%d want=%d", coalesced, documents*(copies-1))
	}
	t.Logf("coalesced %d duplicate rows in %s", coalesced, time.Since(started))
	outbox := st.(archiveStore.MemoryVectorOutboxStore)
	started = time.Now()
	completed, groups := 0, 0
	for completed < documents {
		items, err := outbox.ClaimMemoryVectorOperations(ctx, "feedback-worker", time.Now().UTC(), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		groups++
		for _, item := range items {
			// This check covers the SQL queue only; the Chroma worker is tested separately.
			if err := outbox.CompleteMemoryVectorOperation(ctx, item.ID, "feedback-worker", time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			completed++
		}
	}
	if groups >= documents {
		t.Fatalf("delete batching regressed to %d single-item groups", groups)
	}
	t.Logf("SQL queue drained %d documents in %d groups / %s", completed, groups, time.Since(started))
}

func feedback43Unit(revision string, index int) *archiveStore.PreciseMemoryUnit {
	return &archiveStore.PreciseMemoryUnit{
		UnitID:          fmt.Sprintf("00000000-0000-4000-8000-%012d", index),
		ContractVersion: archiveStore.PreciseMemoryUnitContract, ChatSessionID: "long-values",
		SourceTurnStart: 1, SourceTurnEnd: 1, SourceContract: "source_acceptance_observation.v1",
		SourceRevision: revision, SourceContentHash: strings.Repeat("a", 64),
		SourceRole: "combined_turn_pair", SourceSpanStart: 0, SourceSpanEnd: 8,
		EvidenceExcerpt: "evidence", EvidenceHash: strings.Repeat("b", 64),
		DirectEvidenceIDsJSON: "[]", Kind: "event", PayloadJSON: `{}`,
		TruthScope: "objective", EpistemicMode: "direct", AuthorityClass: "objective_world_state",
		AdmissionState: "committed", ReviewState: "source_observed", Visibility: "public",
		IdempotencyKey: fmt.Sprintf("unit-%d", index), LifecycleState: "active",
	}
}

func TestFeedback43LongPreciseValuesMariaDBIntegration(t *testing.T) {
	db, st := feedback43Database(t)
	revision := feedback43SeedSource(t, db, "long-values", 1)
	for index, length := range []int{120, 121, 255, 256, 70000} {
		unit := feedback43Unit(revision, index+1)
		unit.Subtype = strings.Repeat("s", length)
		unit.RelationshipKey = strings.Repeat("관계🙂", length)
		unit.RevealCondition = strings.Repeat("조건🙂", length)
		if inserted, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(context.Background(), unit); err != nil || !inserted {
			t.Fatalf("length %d: inserted=%v err=%v", length, inserted, err)
		}
		var subtype, relationship, reveal string
		if err := db.QueryRow(`SELECT memory_subtype, relationship_key, reveal_condition FROM precise_memory_units WHERE unit_id=?`, unit.UnitID).Scan(&subtype, &relationship, &reveal); err != nil {
			t.Fatal(err)
		}
		if subtype != unit.Subtype || relationship != unit.RelationshipKey || reveal != unit.RevealCondition {
			t.Fatalf("length %d: stored text changed", length)
		}
	}
}

func TestFeedback43UpgradeReplaysPreservedAdmissionMariaDBIntegration(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	// Reproduce the released 4.2 column sizes on this newly owned database.
	if _, err := db.Exec(`ALTER TABLE precise_memory_units MODIFY memory_subtype VARCHAR(120) NULL, MODIFY relationship_key VARCHAR(255) NULL, MODIFY reveal_condition VARCHAR(255) NULL`); err != nil {
		t.Fatal(err)
	}
	existingRevision := feedback43SeedSource(t, db, "existing-short-values", 1)
	existing := feedback43Unit(existingRevision, 99)
	existing.ChatSessionID = "existing-short-values"
	existing.Subtype, existing.RelationshipKey, existing.RevealCondition = "상태", "친구🙂", "공개 조건"
	if inserted, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(ctx, existing); err != nil || !inserted {
		t.Fatalf("seed pre-upgrade precise row: inserted=%v err=%v", inserted, err)
	}
	revision := feedback43SeedSource(t, db, "long-values", 1)
	unit := feedback43Unit(revision, 1)
	unit.Subtype = strings.Repeat("s", 121)
	unit.RelationshipKey = strings.Repeat("관계", 256)
	unit.RevealCondition = strings.Repeat("조건🙂", 256)
	raw, err := json.Marshal(map[string]any{"subtype": unit.Subtype, "relationship_key": unit.RelationshipKey, "reveal_condition": unit.RevealCondition, "entities": []string{"가", "나"}})
	if err != nil {
		t.Fatal(err)
	}
	admission := &archiveStore.MemoryAdmission{
		ContractVersion: archiveStore.MemoryAdmissionContract, ChatSessionID: unit.ChatSessionID,
		SourceRevision: revision, TurnIndex: 1, DerivationVersion: archiveStore.MemoryAdmissionContract,
		ExtractorVersion: "critic.v1", IndexVersion: archiveStore.MemoryVectorOutboxContract,
		ResultJSON: string(raw), CreatedAt: time.Now().UTC(),
		Memory: &archiveStore.Memory{ChatSessionID: unit.ChatSessionID, TurnIndex: 1, SummaryJSON: string(raw)},
		Evidence: []*archiveStore.DirectEvidence{{ChatSessionID: unit.ChatSessionID, EvidenceText: unit.EvidenceExcerpt,
			SourceTurnStart: 1, SourceTurnEnd: 1, CaptureStage: "critic_extract", SourceMessageIDsJSON: "[]", LineageJSON: "{}"}},
		PreciseUnits: []*archiveStore.PreciseMemoryUnit{unit},
	}
	admission.ResultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{
		admission.SourceRevision, admission.DerivationVersion, admission.ExtractorVersion, admission.IndexVersion, admission.ResultJSON,
	}, "\x1f"))))
	writer := st.(archiveStore.MemoryAdmissionWriter)
	_, err = writer.CommitMemoryAdmission(ctx, admission)
	var overflow *mysql.MySQLError
	if !errors.As(err, &overflow) || overflow.Number != 1406 {
		t.Fatalf("old schema did not reproduce overflow: %v", err)
	}
	var state, storedJSON, storedHash string
	readResult := func() {
		t.Helper()
		if err := db.QueryRow(`SELECT derived_admission_state, derived_result_json, derived_result_hash FROM memory_source_revisions WHERE source_revision=?`, revision).Scan(&state, &storedJSON, &storedHash); err != nil {
			t.Fatal(err)
		}
		if storedJSON != admission.ResultJSON || storedHash != admission.ResultHash {
			t.Fatal("stored Critic result changed")
		}
	}
	readResult()
	if state != "pending" {
		t.Fatalf("failed admission state=%s", state)
	}
	for _, table := range []string{"memories", "direct_evidence_records", "precise_memory_units", "memory_vector_outbox"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE chat_session_id=?", unit.ChatSessionID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed projection left %s rows=%d err=%v", table, count, err)
		}
	}
	migration := filepath.Join("..", "..", "..", "migrations", "013_precise_memory_text_fields.sql")
	statements, err := loadStatements(migration)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := applyStatements(ctx, db, statements, newReport(migration, true)); err != nil {
			t.Fatal(err)
		}
		readResult()
	}
	if err := applyCompatibilityMigrations(ctx, db, newReport(migration, true)); err != nil {
		t.Fatal(err)
	}
	var existingSubtype, existingRelationship, existingReveal string
	if err := db.QueryRow(`SELECT memory_subtype, relationship_key, reveal_condition FROM precise_memory_units WHERE unit_id=?`, existing.UnitID).Scan(&existingSubtype, &existingRelationship, &existingReveal); err != nil {
		t.Fatal(err)
	}
	if existingSubtype != existing.Subtype || existingRelationship != existing.RelationshipKey || existingReveal != existing.RevealCondition {
		t.Fatal("upgrade changed the pre-existing precise row")
	}
	if result, err := writer.CommitMemoryAdmission(ctx, admission); err != nil || result.PreciseInserted != 1 {
		t.Fatalf("replay after upgrade: %+v %v", result, err)
	}
	readResult()
	if state != "committed" {
		t.Fatalf("replayed admission state=%s", state)
	}
	if result, err := writer.CommitMemoryAdmission(ctx, admission); err != nil || !result.Idempotent || result.ExistingResultJSON != admission.ResultJSON {
		t.Fatalf("committed replay: %+v %v", result, err)
	}
	var subtype, relationship, reveal string
	if err := db.QueryRow(`SELECT memory_subtype, relationship_key, reveal_condition FROM precise_memory_units WHERE unit_id=?`, unit.UnitID).Scan(&subtype, &relationship, &reveal); err != nil {
		t.Fatal(err)
	}
	if subtype != unit.Subtype || relationship != unit.RelationshipKey || reveal != unit.RevealCondition {
		t.Fatal("upgraded projection truncated text")
	}
}
