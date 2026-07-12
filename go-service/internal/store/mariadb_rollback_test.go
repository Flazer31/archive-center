package store

import (
	"context"
	"testing"

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
	mock.ExpectExec("DELETE FROM memories").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM kg_triples").WithArgs(sid, fromTurn, fromTurn).WillReturnResult(sqlmock.NewResult(0, 4))
	mock.ExpectExec("DELETE FROM critic_feedback").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM character_events").WithArgs(sid, fromTurn).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
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

	mock.ExpectExec("DELETE FROM session_reference_bindings").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM persona_capsule_attachments").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM protagonist_entity_memories").WithArgs(sid).WillReturnResult(sqlmock.NewResult(0, 0))
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

	if err := m.DeleteSession(ctx, sid); err != nil {
		t.Errorf("DeleteSession: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
