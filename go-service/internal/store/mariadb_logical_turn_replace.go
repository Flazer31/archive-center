package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var _ LogicalTurnReplacementStore = (*mariadbStore)(nil)

// ReplaceLogicalTurn is intentionally limited to the canonical tail. It also
// accepts the immediately missing tail (latest == requested-1): RisuAI can
// delete the old assistant turn before committing its regenerated result. In
// that case the replacement must recreate the same logical turn, not fail or
// allocate a new turn. Historical turns and gaps remain rejected.
func (m *mariadbStore) ReplaceLogicalTurn(ctx context.Context, replacement LogicalTurnReplacement) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	sid := strings.TrimSpace(replacement.ChatSessionID)
	if sid == "" || replacement.TurnIndex <= 0 || strings.TrimSpace(replacement.UserContent) == "" || strings.TrimSpace(replacement.AssistantContent) == "" {
		return fmt.Errorf("invalid logical turn replacement")
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
	var latest sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE`, sid).Scan(&latest); err != nil {
		return err
	}
	latestTurn := 0
	if latest.Valid {
		latestTurn = int(latest.Int64)
	}
	if latestTurn != replacement.TurnIndex && latestTurn != replacement.TurnIndex-1 {
		return fmt.Errorf("logical turn replacement requires current canonical tail: latest=%d requested=%d", latest.Int64, replacement.TurnIndex)
	}
	t := replacement.TurnIndex
	commands := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM effective_input_logs WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM memories WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
		{`DELETE FROM direct_evidence_records WHERE chat_session_id = ? AND source_turn_end >= ?`, []any{sid, t}},
		{`DELETE FROM kg_triples WHERE chat_session_id = ? AND (source_turn >= ? OR valid_from >= ?)`, []any{sid, t, t}},
		{`DELETE FROM critic_feedback WHERE chat_session_id = ? AND target_type = 'turn' AND target_id >= ?`, []any{sid, t}},
		{`DELETE FROM character_events WHERE chat_session_id = ? AND turn_index >= ?`, []any{sid, t}},
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
		{`DELETE FROM status_change_events WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`UPDATE status_effects SET effect_state = 'active', cleared_evidence_json = NULL, cleared_turn = NULL, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND cleared_turn >= ?`, []any{sid, t}},
		{`DELETE FROM status_effects WHERE chat_session_id = ? AND source_turn >= ?`, []any{sid, t}},
		{`DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index = ?`, []any{sid, t}},
	}
	for _, command := range commands {
		if _, err := tx.ExecContext(ctx, command.query, command.args...); err != nil {
			return err
		}
	}
	createdAt := nonZeroTime(replacement.CreatedAt)
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at) VALUES (?, ?, 'user', ?, ?)`, sid, t, replacement.UserContent, createdAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at) VALUES (?, ?, 'assistant', ?, ?)`, sid, t, replacement.AssistantContent, createdAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
