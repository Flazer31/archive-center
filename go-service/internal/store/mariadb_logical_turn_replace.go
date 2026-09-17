package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var _ LogicalTurnReplacementStore = (*mariadbStore)(nil)

func typedLogicalTurnReplacementError(code, stage string, retryable bool, commitState string, cause error) error {
	return &LogicalTurnReplacementError{
		Code:        code,
		Stage:       stage,
		Retryable:   retryable,
		CommitState: commitState,
		Cause:       cause,
	}
}

func classifyLogicalTurnReplacementStoreError(err error, stage string, commitAttempted bool) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotEnabled) {
		return typedLogicalTurnReplacementError(
			"logical_turn_store_unavailable", stage, false, "not_committed", err,
		)
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1044, 1045, 1142, 1143:
			return typedLogicalTurnReplacementError(
				"logical_turn_db_permission_denied", stage, false, "not_committed", err,
			)
		case 1062:
			return typedLogicalTurnReplacementError(
				"logical_turn_revision_conflict", stage, false, "not_committed", err,
			)
		case 1451, 1452:
			return typedLogicalTurnReplacementError(
				"logical_turn_constraint_conflict", stage, false, "not_committed", err,
			)
		case 1205, 1213:
			return typedLogicalTurnReplacementError(
				"logical_turn_transaction_temporarily_blocked", stage, true, "not_committed", err,
			)
		}
	}
	if commitAttempted {
		return typedLogicalTurnReplacementError(
			"logical_turn_commit_outcome_unknown", stage, false, "unknown", err,
		)
	}
	return typedLogicalTurnReplacementError(
		"logical_turn_transaction_failed", stage, true, "not_committed", err,
	)
}

// ReplaceLogicalTurn is intentionally limited to the canonical tail. It also
// accepts the immediately missing tail (latest == requested-1): RisuAI can
// delete the old assistant turn before committing its regenerated result. In
// that case the replacement must recreate the same logical turn, not fail or
// allocate a new turn. Historical turns and gaps remain rejected.
func (m *mariadbStore) ReplaceLogicalTurn(ctx context.Context, replacement LogicalTurnReplacement) error {
	if err := m.ensureDB(); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "preflight", false)
	}
	sid := strings.TrimSpace(replacement.ChatSessionID)
	if sid == "" || replacement.TurnIndex <= 0 || strings.TrimSpace(replacement.UserContent) == "" || strings.TrimSpace(replacement.AssistantContent) == "" {
		return typedLogicalTurnReplacementError(
			"logical_turn_request_invalid", "preflight", false, "not_committed",
			fmt.Errorf("invalid logical turn replacement"),
		)
	}
	if replacement.SourceRevision != nil {
		source := replacement.SourceRevision
		if source.ChatSessionID != sid || source.TurnIndex != replacement.TurnIndex ||
			source.UserContent != replacement.UserContent ||
			source.AssistantContent != replacement.AssistantContent {
			return typedLogicalTurnReplacementError(
				"logical_turn_revision_conflict", "source_revision", false, "not_committed",
				fmt.Errorf("logical turn replacement source revision mismatch"),
			)
		}
		if err := validateMemorySourceRevision(source); err != nil {
			return typedLogicalTurnReplacementError(
				"logical_turn_revision_invalid", "source_revision", false, "not_committed", err,
			)
		}
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "transaction_begin", false)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var latest sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE`, sid).Scan(&latest); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return classifyLogicalTurnReplacementStoreError(err, "canonical_tail_read", false)
	}
	latestTurn := 0
	if latest.Valid {
		latestTurn = int(latest.Int64)
	}
	if latestTurn != replacement.TurnIndex && latestTurn != replacement.TurnIndex-1 {
		return typedLogicalTurnReplacementError(
			"logical_turn_not_current_tail", "canonical_tail_check", false, "not_committed",
			fmt.Errorf("logical turn replacement requires current canonical tail: latest=%d requested=%d", latest.Int64, replacement.TurnIndex),
		)
	}
	t := replacement.TurnIndex
	if replacement.SourceRevision != nil {
		if err := invalidateMemorySourcesTx(
			ctx, tx, sid, t, true, replacement.SourceRevision.SourceRevision,
			"superseded", "logical_turn_replaced", nonZeroTime(replacement.CreatedAt),
		); err != nil {
			return classifyLogicalTurnReplacementStoreError(err, "source_revision_invalidate", false)
		}
		// A host-observed replacement may have invalidated the old source before
		// the new accepted final arrived. Bind every retained non-deleted prior
		// revision at this logical turn to the accepted successor now.
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_source_revisions
			SET lifecycle_state = 'superseded', superseded_by_revision = ?,
			    invalidation_reason = 'logical_turn_replaced',
			    invalidated_at = COALESCE(invalidated_at, ?), updated_at = ?
			WHERE chat_session_id = ? AND turn_index = ?
			  AND source_revision <> ? AND lifecycle_state <> 'deleted'
		`, replacement.SourceRevision.SourceRevision, nonZeroTime(replacement.CreatedAt),
			nonZeroTime(replacement.CreatedAt), sid, t, replacement.SourceRevision.SourceRevision); err != nil {
			return classifyLogicalTurnReplacementStoreError(err, "source_revision_successor_link", false)
		}
		if err := insertMemorySourceRevisionTx(ctx, tx, replacement.SourceRevision); err != nil {
			return classifyLogicalTurnReplacementStoreError(err, "source_revision_register", false)
		}
	}
	commands := canonicalTailDeleteCommands(sid, t, replacement.SourceRevision == nil, false)
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, command.query, command.args...); err != nil {
			return classifyLogicalTurnReplacementStoreError(err, "canonical_replace", false)
		}
	}
	if err := restoreActiveStatusCurrentValuesTx(ctx, tx, sid); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "status_current_restore", false)
	}
	createdAt := nonZeroTime(replacement.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at) VALUES (?, ?, 'user', ?, ?)`, sid, t, replacement.UserContent, createdAt); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "raw_user_insert", false)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at) VALUES (?, ?, 'assistant', ?, ?)`, sid, t, replacement.AssistantContent, createdAt); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "raw_assistant_insert", false)
	}
	if err := tx.Commit(); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "transaction_commit", true)
	}
	committed = true
	return nil
}

type canonicalTailDeleteCommand struct {
	query string
	args  []any
}

func canonicalTailDeleteCommands(sid string, t int, legacyPhysicalCleanup, deleteTailChat bool) []canonicalTailDeleteCommand {
	commands := []canonicalTailDeleteCommand{
		{`DELETE FROM effective_input_logs WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
	}
	if legacyPhysicalCleanup {
		commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM precise_memory_units WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}})
	}
	commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM memories WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}})
	commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM direct_evidence_records WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}})
	commands = append(commands, []canonicalTailDeleteCommand{
		{`DELETE FROM kg_triples WHERE chat_session_id = ? AND (source_turn >= ? OR valid_from >= ?)`, []any{sid, t, t}},
		{`DELETE FROM critic_feedback WHERE chat_session_id = ? AND target_type = 'turn' AND target_id >= ?`, []any{sid, t}},
		{`DELETE FROM character_events WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM speaker_attributions WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM entity_identity_artifact_bindings WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM entity_identity_surfaces WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`UPDATE entity_identities identity_row SET last_seen_turn = GREATEST(identity_row.first_seen_turn, COALESCE((SELECT MAX(surface.source_turn) FROM entity_identity_surfaces surface WHERE surface.chat_session_id = identity_row.chat_session_id AND surface.stable_entity_id = identity_row.stable_entity_id), identity_row.first_seen_turn)), updated_at = CURRENT_TIMESTAMP(3) WHERE identity_row.chat_session_id = ? AND identity_row.source_turn < ? AND identity_row.last_seen_turn >= ?`, []any{sid, t, t}},
		{`DELETE FROM entity_identity_links WHERE chat_session_id = ? AND (source_entity_id IN (SELECT stable_entity_id FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?) OR target_entity_id IN (SELECT stable_entity_id FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?))`, []any{sid, sid, t, sid, t}},
		{`DELETE FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`UPDATE entities SET last_seen_turn = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND (first_seen_turn IS NULL OR first_seen_turn < ?) AND last_seen_turn >= ?`, []any{t - 1, sid, t, t}},
		{`DELETE FROM entities WHERE chat_session_id = ? AND first_seen_turn >= ?`, []any{sid, t}},
		{`DELETE FROM trust_states WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM storylines WHERE chat_session_id = ? AND (last_turn >= ? OR first_turn >= ?)`, []any{sid, t, t}},
		{`DELETE FROM world_rules WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM character_states WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM pending_threads WHERE chat_session_id = ? AND (source_turn >= ? OR created_turn >= ? OR resolved_turn >= ?)`, []any{sid, t, t, t}},
		{`DELETE FROM active_states WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM canonical_state_layers WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM episode_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []any{sid, t, t}},
		{`UPDATE guidance_plan_states SET story_plan_json = NULL, director_json = NULL, warnings_json = NULL, state_status = 'empty', last_turn = -1, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND last_turn >= ?`, []any{sid, t}},
		{`DELETE FROM chapter_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []any{sid, t, t}},
		{`DELETE FROM arc_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []any{sid, t, t}},
		{`DELETE FROM saga_digests WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []any{sid, t, t}},
		{`DELETE FROM session_active_scopes WHERE chat_session_id = ?`, []any{sid}},
		{`DELETE FROM protagonist_entity_memories WHERE source_chat_session_id = ? AND source_turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM consequence_records WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}},
		{`DELETE FROM psychology_branches WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}},
		{`DELETE FROM theme_offscreen_carries WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}},
		{`DELETE FROM capture_verification_records WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM status_current_values WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM status_change_events WHERE chat_session_id = ? AND source_turn >= ? AND NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_revision'))), '') IS NULL`, []any{sid, t}},
		{`UPDATE status_effects SET effect_state = 'active', cleared_evidence_json = NULL, cleared_turn = NULL, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND cleared_turn >= ?`, []any{sid, t}},
		{`DELETE FROM status_effects WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
	}...)
	if legacyPhysicalCleanup {
		commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM status_change_events WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}})
	}
	if deleteTailChat {
		commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}})
	} else {
		commands = append(commands, canonicalTailDeleteCommand{`DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index = ?`, []any{sid, t}})
	}
	return commands
}

// RollbackCanonicalTail performs source invalidation, durable vector-delete
// enqueueing, raw deletion, and derived cleanup in one MariaDB transaction.
// Vector provider I/O intentionally happens after this method commits.
func (m *mariadbStore) RollbackCanonicalTail(ctx context.Context, rollback LogicalTurnRollback) error {
	if err := m.ensureDB(); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "preflight", false)
	}
	sid := strings.TrimSpace(rollback.ChatSessionID)
	if sid == "" || rollback.TurnIndex <= 0 {
		return typedLogicalTurnReplacementError("logical_turn_request_invalid", "preflight", false, "not_committed", fmt.Errorf("invalid logical turn rollback"))
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "transaction_begin", false)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var latest sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE`, sid).Scan(&latest); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return classifyLogicalTurnReplacementStoreError(err, "canonical_tail_read", false)
	}
	if latest.Valid && rollback.TurnIndex > int(latest.Int64)+1 {
		return typedLogicalTurnReplacementError("logical_turn_not_current_tail", "canonical_tail_check", false, "not_committed", fmt.Errorf("logical turn rollback begins after canonical tail: latest=%d requested=%d", latest.Int64, rollback.TurnIndex))
	}
	now := nonZeroTime(rollback.CreatedAt)
	lifecycleAction := strings.ToLower(strings.TrimSpace(rollback.LifecycleAction))
	switch lifecycleAction {
	case "":
		lifecycleAction = LogicalTurnLifecycleInvalidated
	case LogicalTurnLifecycleInvalidated, LogicalTurnLifecycleSuperseded, LogicalTurnLifecycleDeleted:
	default:
		return typedLogicalTurnReplacementError("logical_turn_lifecycle_action_invalid", "preflight", false, "not_committed", fmt.Errorf("unsupported logical turn lifecycle action %q", rollback.LifecycleAction))
	}
	if err := invalidateMemorySourcesTx(ctx, tx, sid, rollback.TurnIndex, false, "", lifecycleAction, firstNonEmptyString(rollback.Reason, "turn_rollback"), now); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "source_revision_invalidate", false)
	}
	for _, command := range canonicalTailDeleteCommands(sid, rollback.TurnIndex, false, true) {
		if _, err := tx.ExecContext(ctx, command.query, command.args...); err != nil {
			return classifyLogicalTurnReplacementStoreError(err, "canonical_rollback", false)
		}
	}
	if err := restoreActiveStatusCurrentValuesTx(ctx, tx, sid); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "status_current_restore", false)
	}
	if err := tx.Commit(); err != nil {
		return classifyLogicalTurnReplacementStoreError(err, "transaction_commit", true)
	}
	committed = true
	return nil
}
