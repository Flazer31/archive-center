package store

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBRollbackStoreDeleteFromTurn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	ctx := context.Background()
	sid := "sess-1"
	fromTurn := 5

	mock.ExpectExec("DELETE FROM chat_logs").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM effective_input_logs").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE precise_memory_units").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM memories").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM kg_triples").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectExec("DELETE FROM critic_feedback").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM character_events").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE precise_memory_units").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM speaker_attributions").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entity_identity_artifact_bindings").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entity_identity_surfaces").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE entity_identities").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entity_identity_links").WithArgs(sid, sid, fromTurn, sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM entity_identities").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE entities").WithArgs(fromTurn-1, sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entities").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec("DELETE FROM trust_states").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM storylines").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM world_rules").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM character_states").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM pending_threads").WithArgs(sid, fromTurn, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM active_states").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM canonical_state_layers").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM episode_summaries").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE guidance_plan_states").WithArgs(sqlmock.AnyArg(), sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM chapter_summaries").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM arc_summaries").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM saga_digests").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM session_active_scopes").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM protagonist_entity_memories").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM consequence_records").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM psychology_branches").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM theme_offscreen_carries").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM capture_verification_records").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM status_current_values").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM status_change_events").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE status_effects").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM status_effects").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := m.DeleteChatLogs(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteChatLogs: %v", err)
	}
	if err := m.DeleteEffectiveInputs(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteEffectiveInputs: %v", err)
	}
	if err := m.DeleteMemories(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteMemories: %v", err)
	}
	if err := m.DeleteEvidence(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteEvidence: %v", err)
	}
	if err := m.DeleteKGTriples(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteKGTriples: %v", err)
	}
	if err := m.DeleteCriticFeedback(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteCriticFeedback: %v", err)
	}
	if err := m.DeleteCharacterEvents(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteCharacterEvents: %v", err)
	}
	if err := m.DeleteEntities(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteEntities: %v", err)
	}
	if err := m.DeleteTrustStates(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteTrustStates: %v", err)
	}
	if err := m.DeleteStorylines(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteStorylines: %v", err)
	}
	if err := m.DeleteWorldRules(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteWorldRules: %v", err)
	}
	if err := m.DeleteCharacterStates(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteCharacterStates: %v", err)
	}
	if err := m.DeletePendingThreads(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeletePendingThreads: %v", err)
	}
	if err := m.DeleteActiveStates(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteActiveStates: %v", err)
	}
	if err := m.DeleteCanonicalStateLayers(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteCanonicalStateLayers: %v", err)
	}
	if err := m.DeleteEpisodeSummaries(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteEpisodeSummaries: %v", err)
	}
	if err := m.DeleteGuidancePlanState(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteGuidancePlanState: %v", err)
	}
	if err := m.DeleteChapterSummaries(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteChapterSummaries: %v", err)
	}
	if err := m.DeleteArcSummaries(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteArcSummaries: %v", err)
	}
	if err := m.DeleteSagaDigests(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteSagaDigests: %v", err)
	}
	if err := m.DeleteSessionActiveScopes(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteSessionActiveScopes: %v", err)
	}
	if err := m.DeleteProtagonistEntityMemories(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteProtagonistEntityMemories: %v", err)
	}
	if err := m.DeleteConsequenceRecords(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteConsequenceRecords: %v", err)
	}
	if err := m.DeletePsychologyBranches(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeletePsychologyBranches: %v", err)
	}
	if err := m.DeleteThemeOffscreenCarries(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteThemeOffscreenCarries: %v", err)
	}
	if err := m.DeleteCaptureVerificationRecords(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteCaptureVerificationRecords: %v", err)
	}
	if err := m.DeleteStatusCurrentValues(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteStatusCurrentValues: %v", err)
	}
	if err := m.DeleteStatusChangeEvents(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteStatusChangeEvents: %v", err)
	}
	if err := m.DeleteStatusEffects(ctx, sid, fromTurn); err != nil {
		t.Errorf("DeleteStatusEffects: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMariaDBDeleteSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	ctx := context.Background()
	sid := "sess-delete"

	mock.ExpectBegin()
	mock.ExpectExec("(?s)DELETE entry.*FROM lorebook_reference_entries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("(?s)DELETE snapshot.*FROM lorebook_reference_snapshots").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM lorebook_reference_scopes").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM lorebook_reference_session_locks").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM session_reference_bindings").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM persona_capsule_attachments").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM protagonist_entity_memories").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT source_revision").
		WithArgs(sid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision"}).AddRow("revision-delete"))
	mock.ExpectQuery("SELECT id FROM memories").
		WithArgs(sid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT id FROM direct_evidence_records").
		WithArgs(sid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT id FROM world_rules").
		WithArgs(sid, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT DISTINCT document_id").
		WithArgs(sid, "revision-delete").
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}).
			AddRow("precise_memory:sess-delete:unit"))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE memory_derivation_dependencies").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE precise_memory_units").
		WithArgs("deleted", "deleted", "deleted", "deleted", "deleted",
			sqlmock.AnyArg(), sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_reprocessing_jobs").
		WithArgs("session_deleted", sqlmock.AnyArg(), sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs("deleted", "session_deleted", sqlmock.AnyArg(), sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs(sqlmock.AnyArg(), sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs("deleted", nil, "session_deleted", sqlmock.AnyArg(), sqlmock.AnyArg(),
			"deleted", "deleted", "deleted", "deleted", "deleted",
			"deleted", "deleted",
			sid, "revision-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM chat_logs").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("DELETE FROM effective_input_logs").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec("DELETE FROM memories").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM kg_triples").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM character_events").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM storylines").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM world_rules").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM character_states").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM pending_threads").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM active_states").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM canonical_state_layers").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM episode_summaries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM chapter_summaries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM arc_summaries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM saga_digests").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM session_active_scopes").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM guidance_plan_states").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM speaker_attributions").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entity_identity_artifact_bindings").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM entity_identity_links").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM entity_identity_surfaces").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM entity_identities").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM entities").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM trust_states").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM consequence_records").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM psychology_branches").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM session_fork_lineage").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM theme_offscreen_carries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM capture_verification_records").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM status_effects").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM status_change_events").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM status_current_values").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM status_schema_registry").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM status_schema_proposals").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM critic_feedback").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := m.DeleteSession(ctx, sid); err != nil {
		t.Errorf("DeleteSession: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMariaDBDeleteSessionRollsBackAllRelationalChangesOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	sid := "sess-delete-rollback"
	mock.ExpectBegin()
	mock.ExpectExec("(?s)DELETE entry.*FROM lorebook_reference_entries").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)DELETE snapshot.*FROM lorebook_reference_snapshots").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM lorebook_reference_scopes").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM lorebook_reference_session_locks").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM session_reference_bindings").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM persona_capsule_attachments").WithArgs(sid).WillReturnError(errors.New("delete attachment failed"))
	mock.ExpectRollback()
	if err := m.DeleteSession(context.Background(), sid); err == nil || !strings.Contains(err.Error(), "delete attachment failed") {
		t.Fatalf("DeleteSession error = %v, want transactional delete failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("delete failure did not roll back the transaction: %v", err)
	}
}

func TestMariaDBDeleteSessionRollsBackLorebookChildCleanupOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	sid := "sess-delete-lorebook-rollback"
	mock.ExpectBegin()
	mock.ExpectExec("(?s)DELETE entry.*FROM lorebook_reference_entries").
		WithArgs(sid).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("(?s)DELETE snapshot.*FROM lorebook_reference_snapshots").
		WithArgs(sid).
		WillReturnError(errors.New("delete lorebook snapshot failed"))
	mock.ExpectRollback()
	err = m.DeleteSession(context.Background(), sid)
	if err == nil || !strings.Contains(err.Error(), "delete lorebook reference snapshots") {
		t.Fatalf("DeleteSession error = %v, want lorebook snapshot delete failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("lorebook delete failure did not roll back the transaction: %v", err)
	}
}

func TestMariaDBAdminResetClearsPreciseMemoryUnits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}

	mock.ExpectExec(regexp.QuoteMeta("SET FOREIGN_KEY_CHECKS=0")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	foundPrecise := false
	for _, table := range mariaAdminResetTables {
		if table == "precise_memory_units" {
			foundPrecise = true
		}
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM " + mariaQuoteIdentifier(table))).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("SET FOREIGN_KEY_CHECKS=1")).WillReturnResult(sqlmock.NewResult(0, 0))

	if !foundPrecise {
		t.Fatal("admin reset table list omits precise_memory_units")
	}
	result, err := m.ResetAll(context.Background())
	if err != nil {
		t.Fatalf("ResetAll: %v", err)
	}
	if result.TablesCleared != len(mariaAdminResetTables) || result.RowsDeleted != int64(len(mariaAdminResetTables)) {
		t.Fatalf("reset result=%+v tables=%d", result, len(mariaAdminResetTables))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBAdminResetWaitsForDerivationWriteLane(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	originalTables := mariaAdminResetTables
	mariaAdminResetTables = []string{"memory_source_revisions"}
	defer func() { mariaAdminResetTables = originalTables }()

	mock.ExpectExec(regexp.QuoteMeta("SET FOREIGN_KEY_CHECKS=0")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `memory_source_revisions`")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta("SET FOREIGN_KEY_CHECKS=1")).WillReturnResult(sqlmock.NewResult(0, 0))

	m.memoryDerivationWriteMu.Lock()
	locked := true
	defer func() {
		if locked {
			m.memoryDerivationWriteMu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		_, resetErr := m.ResetAll(context.Background())
		done <- resetErr
	}()
	select {
	case resetErr := <-done:
		t.Fatalf("ResetAll bypassed derivation write lane: %v", resetErr)
	case <-time.After(25 * time.Millisecond):
	}
	m.memoryDerivationWriteMu.Unlock()
	locked = false
	select {
	case resetErr := <-done:
		if resetErr != nil {
			t.Fatal(resetErr)
		}
	case <-time.After(time.Second):
		t.Fatal("ResetAll did not resume after derivation write lane was released")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
