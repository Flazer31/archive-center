package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSessionMigrationAdmittedHistoryLifecycle(t *testing.T) {
	for _, action := range []string{"unchanged", "reroll", "delete_tail", "delete_all", "reroll_then_delete", "private_projection", "active_missing_result"} {
		t.Run(action, func(t *testing.T) {
			db, st := feedback43Database(t)
			ctx := context.Background()
			const sid = "admitted-history-source"
			admit := func(revision string, turn int) {
				t.Helper()
				result := fmt.Sprintf(`{"turn_summary":"Synthetic gate event %d."}`, turn)
				a := &archiveStore.MemoryAdmission{
					ContractVersion: archiveStore.MemoryAdmissionContract, ChatSessionID: sid, SourceRevision: revision,
					TurnIndex: turn, DerivationVersion: archiveStore.MemoryAdmissionContract,
					ExtractorVersion: "critic.synthetic", IndexVersion: archiveStore.MemoryPublicProjectionIndex,
					ResultJSON: result, CreatedAt: time.Now().UTC(),
					Memory: &archiveStore.Memory{ChatSessionID: sid, TurnIndex: turn, SummaryJSON: result},
					Vectors: []archiveStore.MemoryAdmissionVector{{
						ArtifactType: "memory", Tier: "memory", SourceTable: "memories",
						SchemaVersion: "memory.v2", DocumentText: fmt.Sprintf("Synthetic gate event %d.", turn),
						Embedding: []float32{0.3, 0.4}, EmbeddingModel: "synthetic-vector",
					}},
				}
				if action == "private_projection" {
					a.MemoryPublicProjectionExcluded = true
					a.Vectors = nil
				}
				a.ResultHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join([]string{
					a.SourceRevision, a.DerivationVersion, a.ExtractorVersion, a.IndexVersion, a.ResultJSON,
				}, "\x1f"))))
				if _, err := st.(archiveStore.MemoryAdmissionWriter).CommitMemoryAdmission(ctx, a); err != nil {
					t.Fatalf("production admission: %v", err)
				}
			}
			for turn := 1; turn <= 2; turn++ {
				admit(state46Source(t, db, sid, turn), turn)
			}
			if action == "reroll" || action == "reroll_then_delete" {
				r := &archiveStore.MemorySourceRevision{
					ContractVersion: "source_acceptance_observation.v1", ChatSessionID: sid,
					SourceRevision: "rerolled-admitted-revision", LogicalTurnID: "turn-2", TurnIndex: 2,
					BranchState: "observed", UserContent: "Replacement user", AssistantContent: "The gate is now open.",
					CombinedContentHash: strings.Repeat("b", 64), HashAlgorithm: "sha256", HostObservedAtMS: 1, LifecycleState: "active",
				}
				if err := st.(archiveStore.LogicalTurnReplacementStore).ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{
					ChatSessionID: sid, TurnIndex: 2, UserContent: r.UserContent, AssistantContent: r.AssistantContent, SourceRevision: r,
				}); err != nil {
					t.Fatalf("production reroll: %v", err)
				}
				admit(r.SourceRevision, 2)
			}
			if action == "delete_tail" || action == "delete_all" || action == "reroll_then_delete" {
				from := 2
				if action == "delete_all" {
					from = 1
				}
				if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{
					ChatSessionID: sid, TurnIndex: from, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted,
				}); err != nil {
					t.Fatalf("production deletion: %v", err)
				}
				var redacted int
				if err := db.QueryRow(`SELECT COUNT(*) FROM memory_source_revisions WHERE chat_session_id=?
					AND lifecycle_state='deleted' AND derived_admission_state='committed'
					AND derived_result_json IS NULL AND derived_result_hash IS NOT NULL`, sid).Scan(&redacted); err != nil || redacted == 0 {
					t.Fatalf("deletion did not produce the reported history shape: count=%d err=%v", redacted, err)
				}
				t.Logf("production deletion retained %d committed fingerprints and cleared their JSON", redacted)
			}
			type sourceRecord struct {
				Revision, State, Admission, User, Assistant string
				Hash, Result, Superseded                    sql.NullString
				Updated                                     time.Time
			}
			read := func(session string) []sourceRecord {
				t.Helper()
				rows, err := db.Query(`SELECT source_revision,lifecycle_state,derived_admission_state,raw_user_content,
					raw_assistant_content,derived_result_hash,derived_result_json,superseded_by_revision,updated_at
					FROM memory_source_revisions WHERE chat_session_id=? ORDER BY id`, session)
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				var result []sourceRecord
				for rows.Next() {
					var r sourceRecord
					if err := rows.Scan(&r.Revision, &r.State, &r.Admission, &r.User, &r.Assistant, &r.Hash, &r.Result, &r.Superseded, &r.Updated); err != nil {
						t.Fatal(err)
					}
					result = append(result, r)
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				return result
			}
			if action == "active_missing_result" {
				if _, err := db.Exec(`UPDATE memory_source_revisions SET derived_result_json=NULL WHERE chat_session_id=? AND turn_index=2`, sid); err != nil {
					t.Fatal(err)
				}
				before := read(sid)
				_, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{
					SourceSessionID: sid, TargetSessionID: "damaged-copy", Mode: archiveStore.SessionMigrationModeCopyKeepSource,
				})
				if err == nil || !strings.Contains(err.Error(), "committed derived result is invalid") {
					t.Fatalf("active invalid result changed behavior: %v", err)
				}
				for _, entry := range archiveStore.SessionMigrationManifest() {
					if entry.Policy != archiveStore.SessionMigrationPolicyCopy || !entry.Direct {
						continue
					}
					var count int
					if err := db.QueryRow("SELECT COUNT(*) FROM `"+entry.Table+"` WHERE `"+entry.SessionColumn+"`=?", "damaged-copy").Scan(&count); err != nil || count != 0 {
						t.Fatalf("failed copy left %s rows=%d err=%v", entry.Table, count, err)
					}
				}
				if !reflect.DeepEqual(before, read(sid)) {
					t.Fatal("failed copy mutated source")
				}
				t.Log("active invalid result retains existing error; target rolled back and source preserved")
				return
			}
			before := read(sid)
			copySession := func(from, to string) int64 {
				t.Helper()
				result, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{
					SourceSessionID: from, TargetSessionID: to, Mode: archiveStore.SessionMigrationModeCopyKeepSource,
				})
				if err != nil {
					t.Fatalf("copy %s -> %s: %v", from, to, err)
				}
				if result.Status != "copied" {
					t.Fatalf("copy status=%s", result.Status)
				}
				for _, entry := range archiveStore.SessionMigrationManifest() {
					if entry.Policy != archiveStore.SessionMigrationPolicyCopy {
						continue
					}
					var a, b int
					var ah, bh string
					if err := db.QueryRow(`SELECT source_row_count,target_row_count,source_content_hash,target_content_hash
						FROM session_migration_artifact_parity WHERE migration_id=? AND table_name=?`, result.MigrationID, entry.Table).Scan(&a, &b, &ah, &bh); err != nil {
						t.Fatal(err)
					}
					if a != b || ah != bh {
						t.Fatalf("copy parity differs for %s", entry.Table)
					}
				}
				original, copied := read(from), read(to)
				if len(original) != len(copied) {
					t.Fatal("source history rows lost")
				}
				for i, r := range original {
					c := copied[i]
					if r.Revision == c.Revision {
						t.Fatal("source revision was not remapped")
					}
					if r.State != c.State || r.User != c.User || r.Assistant != c.Assistant || r.Result != c.Result || r.Admission != c.Admission || !r.Updated.Equal(c.Updated) {
						t.Fatal("copied history or timestamps changed")
					}
					if r.State == "deleted" {
						if r.Hash != c.Hash || c.Result.Valid || c.User != "" || c.Assistant != "" {
							t.Fatal("deleted history was restored or its fingerprint rewritten")
						}
					} else if r.Hash == c.Hash {
						t.Fatal("retained result was not rehashed for the new revision")
					}
				}
				if !reflect.DeepEqual(before, read(sid)) {
					t.Fatal("copy modified original source history")
				}
				var memories, receipts, pending, public int
				if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE chat_session_id=?`, to).Scan(&memories); err != nil {
					t.Fatal(err)
				}
				if err := db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(status<>'completed'),0),COALESCE(SUM(operation='upsert'),0) FROM memory_vector_outbox WHERE chat_session_id=?`, to).Scan(&receipts, &pending, &public); err != nil {
					t.Fatal(err)
				}
				if receipts != memories || pending != 0 {
					t.Fatalf("projection receipts=%d memories=%d queued=%d", receipts, memories, pending)
				}
				if action == "private_projection" && public != 0 {
					t.Fatal("excluded private memory became public")
				}
				return result.MigrationID
			}
			copySession(sid, "admitted-history-copy")
			copySession(sid, "admitted-history-copy")
			lastMigration := copySession("admitted-history-copy", "admitted-history-copy-again")
			// Exercise the relational proof used by reindex with synthetic vector
			// readback. This is a DB contract test, not a live Chroma assertion.
			vectors, err := st.(archiveStore.SessionMigrationVectorStore).ListSessionMigrationVectorDocuments(ctx, lastMigration)
			if err != nil {
				t.Fatal(err)
			}
			var vectorIDs []string
			for _, doc := range vectors {
				vectorIDs = append(vectorIDs, doc.ID)
			}
			parity, err := st.(archiveStore.SessionMigrationVectorParityStore).VerifySessionMigrationVectorParity(ctx, lastMigration, archiveStore.SessionMigrationProofOperationSourceLock, vectorIDs)
			if err != nil || !parity.Verified {
				t.Fatalf("reindex relational proof failed: %v", err)
			}
			if _, err := st.(archiveStore.SessionMigrationRecoveryStore).RollbackSessionMigration(ctx, lastMigration, "synthetic rollback"); err != nil {
				t.Fatalf("rollback copied receipts: %v", err)
			}
			for _, table := range []string{"memory_source_revisions", "memories", "memory_vector_outbox"} {
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE chat_session_id='admitted-history-copy-again'").Scan(&count); err != nil || count != 0 {
					t.Fatalf("rollback left %s rows=%d err=%v", table, count, err)
				}
			}
			if !reflect.DeepEqual(before, read(sid)) {
				t.Fatal("rollback modified original source")
			}
			t.Log("copy, retry and copy-of-copy passed; all manifest parity, source history and deletion state preserved")
		})
	}
}

func TestSessionMigrationMissingEvidenceAfterReroll(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid = "missing-evidence-experiment"
	revision := state46Source(t, db, sid, 1)
	const quote = "The brass key was inside the blue box."
	if _, err := db.Exec(`INSERT INTO direct_evidence_records (id, chat_session_id, evidence_text, source_turn_start, source_turn_end) VALUES (6748, ?, ?, 1, 1)`, sid, quote); err != nil {
		t.Fatal(err)
	}
	unit := &archiveStore.PreciseMemoryUnit{
		UnitID: "11111111-2222-5333-8444-555555555555", ContractVersion: archiveStore.PreciseMemoryUnitContract,
		ChatSessionID: sid, SourceTurnStart: 1, SourceTurnEnd: 1,
		SourceContract: "source_acceptance_observation.v1", SourceRevision: revision,
		SourceContentHash: strings.Repeat("a", 64), SourceRole: "assistant", SourceSpanEnd: len(quote),
		EvidenceExcerpt: quote, EvidenceHash: strings.Repeat("a", 64), RootEvidenceID: 6748,
		DirectEvidenceIDsJSON: `[6748]`, Kind: "event", PayloadJSON: `{"text":"The brass key was inside the blue box."}`,
		TruthScope: "objective", EpistemicMode: "observed", AuthorityClass: "source_observed",
		AdmissionState: "admitted", ReviewState: "accepted", Visibility: "public", IdempotencyKey: "synthetic-old-key", LifecycleState: "active",
	}
	if _, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(ctx, unit); err != nil {
		t.Fatal(err)
	}
	replacement := &archiveStore.MemorySourceRevision{
		ContractVersion: "source_acceptance_observation.v1", SourceRevision: "missing-evidence-new-revision", ChatSessionID: sid,
		LogicalTurnID: "turn-1", TurnIndex: 1, BranchState: "observed", UserContent: "Replacement user",
		AssistantContent: "The brass key is instead inside the red box.", CombinedContentHash: strings.Repeat("b", 64), HashAlgorithm: "sha256", HostObservedAtMS: 1, LifecycleState: "active",
	}
	if err := st.(archiveStore.LogicalTurnReplacementStore).ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{ChatSessionID: sid, TurnIndex: 1, UserContent: replacement.UserContent, AssistantContent: replacement.AssistantContent, SourceRevision: replacement}); err != nil {
		t.Fatal(err)
	}
	var evidenceCount int
	var sourceState, unitState, ids, text string
	if err := db.QueryRow(`SELECT COUNT(*) FROM direct_evidence_records WHERE id=6748`).Scan(&evidenceCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT lifecycle_state FROM memory_source_revisions WHERE source_revision=?`, revision).Scan(&sourceState); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT lifecycle_state, direct_evidence_ids_json, evidence_excerpt FROM precise_memory_units WHERE unit_id=?`, unit.UnitID).Scan(&unitState, &ids, &text); err != nil {
		t.Fatal(err)
	}
	if evidenceCount != 0 || sourceState != "superseded" || unitState != "invalidated" || ids != `[6748]` || text != quote {
		t.Fatalf("unexpected replacement: evidence=%d source=%s unit=%s ids=%s text=%s", evidenceCount, sourceState, unitState, ids, text)
	}
	t.Logf("production reroll reproduced: evidence absent, source=%s, memory=%s, ids=%s; excerpt retained", sourceState, unitState, ids)
	result, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: "missing-evidence-copy", Mode: archiveStore.SessionMigrationModeCopyKeepSource})
	if err != nil {
		t.Fatalf("copy after production reroll: %v", err)
	}
	if result.Status != "copied" {
		t.Fatalf("result=%+v", result)
	}
	if err := db.QueryRow(`SELECT lifecycle_state, direct_evidence_ids_json, evidence_excerpt FROM precise_memory_units WHERE chat_session_id='missing-evidence-copy'`).Scan(&unitState, &ids, &text); err != nil {
		t.Fatal(err)
	}
	if unitState != "invalidated" || ids != `[]` || text != quote {
		t.Fatalf("historical memory changed: state=%s ids=%s text=%s", unitState, ids, text)
	}
	var preserved string
	if err := db.QueryRow(`SELECT direct_evidence_ids_json FROM precise_memory_units WHERE unit_id=?`, unit.UnitID).Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != `[6748]` {
		t.Fatal("copy mutated source history")
	}
	var activeText string
	if err := db.QueryRow(`SELECT raw_assistant_content FROM memory_source_revisions WHERE chat_session_id='missing-evidence-copy' AND lifecycle_state='active'`).Scan(&activeText); err != nil {
		t.Fatal(err)
	}
	if activeText != replacement.AssistantContent {
		t.Fatal("active replacement text lost")
	}
	for _, entry := range archiveStore.SessionMigrationManifest() {
		if entry.Policy != archiveStore.SessionMigrationPolicyCopy {
			continue
		}
		var sourceCount, targetCount int
		var sourceHash, targetHash string
		if err := db.QueryRow(`SELECT source_row_count,target_row_count,source_content_hash,target_content_hash FROM session_migration_artifact_parity WHERE migration_id=? AND table_name=?`, result.MigrationID, entry.Table).Scan(&sourceCount, &targetCount, &sourceHash, &targetHash); err != nil {
			t.Fatal(err)
		}
		if sourceCount != targetCount || sourceHash != targetHash {
			t.Fatalf("copy parity differs for %s", entry.Table)
		}
	}
	if _, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: "missing-evidence-copy", Mode: archiveStore.SessionMigrationModeCopyKeepSource}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: "missing-evidence-copy", TargetSessionID: "missing-evidence-copy-again", Mode: archiveStore.SessionMigrationModeCopyKeepSource}); err != nil {
		t.Fatalf("copy of copy: %v", err)
	}
	var activeUnits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM precise_memory_units WHERE chat_session_id IN ('missing-evidence-copy','missing-evidence-copy-again') AND lifecycle_state='active'`).Scan(&activeUnits); err != nil || activeUnits != 0 {
		t.Fatalf("historical units reactivated: count=%d err=%v", activeUnits, err)
	}
	t.Log("copy and retry passed; source untouched; old excerpt/history, active replacement and table parity preserved")
}

func TestSessionMigrationMissingEvidenceReferenceCases(t *testing.T) {
	for _, tc := range []struct {
		name, unitState, sourceState string
		missing, wantError           bool
		overrideIDs                  string
		foreignEvidence              bool
	}{
		{"active intact", "active", "active", false, false, "", false},
		{"active missing unchanged", "active", "active", true, true, "", false},
		{"historical mixed refs", "invalidated", "superseded", true, false, "", false},
		{"historical valid ordering and duplicates", "invalidated", "superseded", false, false, `[52,51,52]`, false},
		{"inactive unit active source unchanged", "invalidated", "active", true, true, "", false},
		{"active unit inactive source unchanged", "active", "superseded", true, true, "", false},
		{"historical wrong session unchanged", "invalidated", "superseded", false, true, "", true},
		{"historical string ID unchanged", "invalidated", "superseded", true, true, `["52",51]`, false},
		{"historical zero ID unchanged", "invalidated", "superseded", true, true, `[0,51]`, false},
		{"historical negative ID unchanged", "invalidated", "superseded", true, true, `[-1,51]`, false},
		{"historical fractional ID unchanged", "invalidated", "superseded", true, true, `[1.5,51]`, false},
		{"historical overflow ID unchanged", "invalidated", "superseded", true, true, `[9223372036854775808,51]`, false},
		{"historical null ID unchanged", "invalidated", "superseded", true, true, `[null,51]`, false},
		{"historical nested ID unchanged", "invalidated", "superseded", true, true, `[[51],51]`, false},
		{"historical object unchanged", "invalidated", "superseded", true, true, `{"id":51}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, st := feedback43Database(t)
			ctx := context.Background()
			const sid = "reference-case"
			revision := state46Source(t, db, sid, 1)
			const quote = "The watchman carries a copper lantern."
			if _, err := db.Exec(`INSERT INTO direct_evidence_records (id,chat_session_id,evidence_text,source_turn_start,source_turn_end) VALUES (51,?,?,1,1),(52,?,?,1,1)`, sid, quote, sid, quote); err != nil {
				t.Fatal(err)
			}
			unit := &archiveStore.PreciseMemoryUnit{
				UnitID: "aaaaaaaa-bbbb-5ccc-8ddd-eeeeeeeeeeee", ContractVersion: archiveStore.PreciseMemoryUnitContract,
				ChatSessionID: sid, SourceTurnStart: 1, SourceTurnEnd: 1, SourceContract: "source_acceptance_observation.v1", SourceRevision: revision,
				SourceContentHash: strings.Repeat("a", 64), SourceRole: "assistant", SourceSpanEnd: len(quote), EvidenceExcerpt: quote, EvidenceHash: strings.Repeat("b", 64),
				RootEvidenceID: 51, DirectEvidenceIDsJSON: `[52,51]`, Kind: "event", PayloadJSON: `{"text":"A copper lantern."}`,
				TruthScope: "objective", EpistemicMode: "observed", AuthorityClass: "source_observed", AdmissionState: "admitted", ReviewState: "accepted", Visibility: "public", IdempotencyKey: "reference-case-unit", LifecycleState: "active",
			}
			if _, err := st.(archiveStore.PreciseMemoryWriter).SavePreciseMemoryUnit(ctx, unit); err != nil {
				t.Fatal(err)
			}
			if tc.missing {
				if _, err := db.Exec(`DELETE FROM direct_evidence_records WHERE id=52`); err != nil {
					t.Fatal(err)
				}
			}
			if tc.foreignEvidence {
				if _, err := db.Exec(`UPDATE direct_evidence_records SET chat_session_id='unrelated-session' WHERE id=52`); err != nil {
					t.Fatal(err)
				}
			}
			originalIDs := unit.DirectEvidenceIDsJSON
			if tc.overrideIDs != "" {
				originalIDs = tc.overrideIDs
				if _, err := db.Exec(`UPDATE precise_memory_units SET direct_evidence_ids_json=? WHERE unit_id=?`, originalIDs, unit.UnitID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`UPDATE precise_memory_units SET lifecycle_state=? WHERE unit_id=?`, tc.unitState, unit.UnitID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE memory_source_revisions SET lifecycle_state=? WHERE source_revision=?`, tc.sourceState, revision); err != nil {
				t.Fatal(err)
			}
			if tc.unitState == "invalidated" {
				if _, err := db.Exec(`UPDATE memory_derivation_dependencies SET lifecycle_state='invalidated' WHERE source_revision=?`, revision); err != nil {
					t.Fatal(err)
				}
			}
			result, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: "reference-copy", Mode: archiveStore.SessionMigrationModeCopyKeepSource})
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "semantic reference direct_evidence_ids_json") {
					t.Fatalf("existing unresolved active reference behavior changed: %v", err)
				}
				var n int
				if err := db.QueryRow(`SELECT COUNT(*) FROM precise_memory_units WHERE chat_session_id='reference-copy'`).Scan(&n); err != nil || n != 0 {
					t.Fatalf("failed copy left partial data: %d %v", n, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var copiedIDs, copiedState, copiedQuote string
			if err := db.QueryRow(`SELECT direct_evidence_ids_json,lifecycle_state,evidence_excerpt FROM precise_memory_units WHERE chat_session_id='reference-copy'`).Scan(&copiedIDs, &copiedState, &copiedQuote); err != nil {
				t.Fatal(err)
			}
			var ids []int64
			if err := json.Unmarshal([]byte(copiedIDs), &ids); err != nil {
				t.Fatal(err)
			}
			var oldIDs []int64
			if err := json.Unmarshal([]byte(originalIDs), &oldIDs); err != nil {
				t.Fatal(err)
			}
			retained := make([]int64, 0, len(oldIDs))
			for _, id := range oldIDs {
				if !tc.missing || id != 52 {
					retained = append(retained, id)
				}
			}
			if len(ids) != len(retained) || copiedState != tc.unitState || copiedQuote != quote {
				t.Fatalf("copy changed preserved data: ids=%s state=%s quote=%s", copiedIDs, copiedState, copiedQuote)
			}
			for index, id := range ids {
				var n int
				if err := db.QueryRow(`SELECT COUNT(*) FROM direct_evidence_records WHERE id=? AND chat_session_id='reference-copy'`, id).Scan(&n); err != nil || n != 1 {
					t.Fatalf("wrong mapped reference %d: %d %v", id, n, err)
				}
				var originalID string
				if err := db.QueryRow(`SELECT source_key FROM session_migration_artifact_row_map WHERE migration_id=? AND table_name='direct_evidence_records' AND key_column_name='id' AND target_key=?`, result.MigrationID, id).Scan(&originalID); err != nil || originalID != strconv.FormatInt(retained[index], 10) {
					t.Fatalf("retained reference order changed at %d: source=%s err=%v", index, originalID, err)
				}
			}
			var sourceIDs string
			if err := db.QueryRow(`SELECT direct_evidence_ids_json FROM precise_memory_units WHERE unit_id=?`, unit.UnitID).Scan(&sourceIDs); err != nil || sourceIDs != originalIDs {
				t.Fatalf("source changed: %s %v", sourceIDs, err)
			}
			if result.Status != "copied" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}
