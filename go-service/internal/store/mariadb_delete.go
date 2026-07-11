package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
var _ RollbackStore = (*mariadbStore)(nil)

func (m *mariadbStore) DeleteChatLogs(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM effective_input_logs WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM memories WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteEvidence(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	// Overlap by source range: evidence that spans into or starts at/after fromTurn.
	_, err := m.db.ExecContext(ctx, "DELETE FROM direct_evidence_records WHERE chat_session_id = ? AND source_turn_end >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteKGTriples(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM kg_triples
		WHERE chat_session_id = ?
		  AND (source_turn >= ? OR valid_from >= ?)
	`, chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteCriticFeedback(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM critic_feedback
		WHERE chat_session_id = ? AND target_type = 'turn' AND target_id >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteCharacterEvents(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM character_events WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteEntities(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
		UPDATE entities
		SET last_seen_turn = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE chat_session_id = ?
		  AND (first_seen_turn IS NULL OR first_seen_turn < ?)
		  AND last_seen_turn >= ?
	`, fromTurn-1, chatSessionID, fromTurn, fromTurn); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entities
		WHERE chat_session_id = ? AND first_seen_turn >= ?
	`, chatSessionID, fromTurn); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *mariadbStore) DeleteTrustStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM trust_states WHERE chat_session_id = ? AND source_turn >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteStorylines(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM storylines WHERE chat_session_id = ? AND (last_turn >= ? OR first_turn >= ?)", chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteWorldRules(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM world_rules WHERE chat_session_id = ? AND source_turn >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteCharacterStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM character_states WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeletePendingThreads(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM pending_threads WHERE chat_session_id = ? AND (source_turn >= ? OR created_turn >= ? OR resolved_turn >= ?)", chatSessionID, fromTurn, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteActiveStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM active_states WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteCanonicalStateLayers(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM canonical_state_layers WHERE chat_session_id = ? AND turn_index >= ?", chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteEpisodeSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM episode_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)", chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) GetGuidancePlanState(ctx context.Context, chatSessionID string) (*GuidancePlanState, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, story_plan_json, director_json, state_status, last_turn, warnings_json, created_at, updated_at
		FROM guidance_plan_states
		WHERE chat_session_id = ?
	`, chatSessionID)
	var item GuidancePlanState
	var storyPlanJSON, directorJSON, stateStatus, warningsJSON sql.NullString
	if err := row.Scan(&item.ID, &item.ChatSessionID, &storyPlanJSON, &directorJSON, &stateStatus, &item.LastTurn, &warningsJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.StoryPlanJSON = stringFromNull(storyPlanJSON)
	item.DirectorJSON = stringFromNull(directorJSON)
	item.StateStatus = stringFromNull(stateStatus)
	item.WarningsJSON = stringFromNull(warningsJSON)
	return &item, nil
}

func (m *mariadbStore) UpsertGuidancePlanState(ctx context.Context, item *GuidancePlanState) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	now := item.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO guidance_plan_states (chat_session_id, story_plan_json, director_json, state_status, last_turn, warnings_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			story_plan_json = VALUES(story_plan_json),
			director_json   = VALUES(director_json),
			state_status    = VALUES(state_status),
			last_turn       = VALUES(last_turn),
			warnings_json   = VALUES(warnings_json),
			updated_at      = VALUES(updated_at)
	`, item.ChatSessionID, item.StoryPlanJSON, item.DirectorJSON, item.StateStatus, item.LastTurn, item.WarningsJSON, now, now)
	return err
}

func (m *mariadbStore) DeleteGuidancePlanState(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err := m.db.ExecContext(ctx, `
		UPDATE guidance_plan_states
		SET story_plan_json = NULL,
		    director_json = NULL,
		    warnings_json = NULL,
		    state_status = 'empty',
		    last_turn = -1,
		    updated_at = ?
		WHERE chat_session_id = ?
		  AND last_turn >= ?
	`, now, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteChapterSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM chapter_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)", chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteArcSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM arc_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)", chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteSagaDigests(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM saga_digests WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)", chatSessionID, fromTurn, fromTurn)
	return err
}

func (m *mariadbStore) DeleteSessionActiveScopes(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM session_active_scopes WHERE chat_session_id = ?", chatSessionID)
	return err
}

func (m *mariadbStore) DeleteProtagonistEntityMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM protagonist_entity_memories
		WHERE source_chat_session_id = ?
		  AND source_turn_index >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteConsequenceRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM consequence_records
		WHERE chat_session_id = ?
		  AND source_turn_end >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeletePsychologyBranches(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM psychology_branches
		WHERE chat_session_id = ?
		  AND source_turn_end >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteThemeOffscreenCarries(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM theme_offscreen_carries
		WHERE chat_session_id = ?
		  AND source_turn_end >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteCaptureVerificationRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM capture_verification_records
		WHERE chat_session_id = ?
		  AND turn_index >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteStatusCurrentValues(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM status_current_values
		WHERE chat_session_id = ?
		  AND source_turn >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteStatusChangeEvents(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM status_change_events
		WHERE chat_session_id = ?
		  AND source_turn >= ?
	`, chatSessionID, fromTurn)
	return err
}

func (m *mariadbStore) DeleteStatusEffects(ctx context.Context, chatSessionID string, fromTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
		UPDATE status_effects
		SET effect_state = 'active',
		    cleared_evidence_json = NULL,
		    cleared_turn = NULL,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE chat_session_id = ?
		  AND cleared_turn >= ?
	`, chatSessionID, fromTurn); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM status_effects
		WHERE chat_session_id = ?
		  AND source_turn >= ?
	`, chatSessionID, fromTurn); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
