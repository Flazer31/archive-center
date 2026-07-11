package store

import (
	"context"
	"database/sql"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) SaveCriticFeedback(ctx context.Context, f *CriticFeedback) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO critic_feedback (created_at, chat_session_id, target_type, target_id, feedback_value, feedback_note, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, nonZeroTime(f.CreatedAt), f.ChatSessionID, f.TargetType, f.TargetID, f.FeedbackValue,
		nullableString(f.FeedbackNote), nullableString(f.Source))
	return err
}

func (m *mariadbStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, created_at, chat_session_id, target_type, target_id, feedback_value, feedback_note, source
		FROM critic_feedback
		WHERE chat_session_id = ? AND (? = '' OR target_type = ?) AND (? <= 0 OR target_id = ?)
		ORDER BY created_at DESC, id DESC
	`, chatSessionID, strings.TrimSpace(targetType), targetType, targetID, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CriticFeedback
	for rows.Next() {
		var item CriticFeedback
		var note, source sql.NullString
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.ChatSessionID, &item.TargetType,
			&item.TargetID, &item.FeedbackValue, &note, &source); err != nil {
			return nil, err
		}
		item.FeedbackNote = stringFromNull(note)
		item.Source = stringFromNull(source)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO character_events (chat_session_id, character_name, turn_index, event_type, details_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.ChatSessionID, e.CharacterName, e.TurnIndex, e.EventType, nullableString(e.DetailsJSON), nonZeroTime(e.CreatedAt))
	return err
}

func (m *mariadbStore) SaveEntity(ctx context.Context, e *Entity) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO entities (
			chat_session_id, name, entity_type, description, aliases_json,
			first_seen_turn, last_seen_turn, confidence, pinned, suppressed,
			user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ChatSessionID, e.Name, nullableString(e.EntityType), nullableString(e.Description),
		nullableString(e.AliasesJSON), e.FirstSeenTurn, e.LastSeenTurn, e.Confidence,
		e.Pinned, e.Suppressed, e.UserCorrected, nonZeroTime(e.CreatedAt), nonZeroTime(e.UpdatedAt))
	return err
}

func (m *mariadbStore) SaveTrust(ctx context.Context, t *Trust) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO trust_states (
			chat_session_id, target_name, target_type, score, reason_json,
			source_turn, pinned, suppressed, user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ChatSessionID, t.TargetName, nullableString(t.TargetType), t.Score,
		nullableString(t.ReasonJSON), t.SourceTurn, t.Pinned, t.Suppressed,
		t.UserCorrected, nonZeroTime(t.CreatedAt), nonZeroTime(t.UpdatedAt))
	return err
}

func (m *mariadbStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, character_name, turn_index, event_type, details_json, created_at
		FROM character_events
		WHERE chat_session_id = ? AND (? = '' OR character_name = ?)
		ORDER BY turn_index ASC, id ASC
	`, chatSessionID, strings.TrimSpace(characterName), characterName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CharacterEvent
	for rows.Next() {
		var item CharacterEvent
		var turnIndex sql.NullInt64
		var detailsJSON sql.NullString
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.CharacterName, &turnIndex,
			&item.EventType, &detailsJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.TurnIndex = intFromNull(turnIndex)
		item.DetailsJSON = stringFromNull(detailsJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) Stats(ctx context.Context) (StatsResult, error) {
	if err := m.ensureDB(); err != nil {
		return StatsResult{}, err
	}
	var out StatsResult
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_logs`).Scan(&out.ChatLogs); err != nil {
		return StatsResult{}, err
	}
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories`).Scan(&out.Memories); err != nil {
		return StatsResult{}, err
	}
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_triples`).Scan(&out.KgTriples); err != nil {
		return StatsResult{}, err
	}
	return out, nil
}
