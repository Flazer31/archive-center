package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) GetActiveScope(ctx context.Context, chatSessionID string) (*SessionActiveScope, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item SessionActiveScope
	var scopeName sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, active_scope, scope_name, updated_at
		FROM session_active_scopes
		WHERE chat_session_id = ?
		LIMIT 1
	`, chatSessionID).Scan(&item.ID, &item.ChatSessionID, &item.ActiveScope, &scopeName, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.ScopeName = stringFromNull(scopeName)
	return &item, nil
}

func (m *mariadbStore) UpsertActiveScope(ctx context.Context, item *SessionActiveScope) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item == nil || strings.TrimSpace(item.ChatSessionID) == "" {
		return ErrNotFound
	}
	activeScope := firstNonEmptyString(strings.TrimSpace(item.ActiveScope), "root")
	updatedAt := nonZeroTime(item.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO session_active_scopes (chat_session_id, active_scope, scope_name, updated_at)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			active_scope = VALUES(active_scope),
			scope_name = VALUES(scope_name),
			updated_at = VALUES(updated_at)
	`, item.ChatSessionID, activeScope, nullableString(item.ScopeName), updatedAt)
	return err
}

func (m *mariadbStore) SaveCharacterState(ctx context.Context, c *CharacterState) error {
	if err := m.ensureDB(); err != nil {
		return err
	}

	next := *c
	if current, err := m.GetCharacterState(ctx, c.ChatSessionID, c.CharacterName); err == nil && current != nil {
		next.AppearanceJSON = firstNonEmptyString(next.AppearanceJSON, current.AppearanceJSON)
		next.PersonalityJSON = firstNonEmptyString(next.PersonalityJSON, current.PersonalityJSON)
		next.StatusJSON = firstNonEmptyString(next.StatusJSON, current.StatusJSON)
		next.RelationshipsJSON = firstNonEmptyString(next.RelationshipsJSON, current.RelationshipsJSON)
		next.SpeechStyleJSON = firstNonEmptyString(next.SpeechStyleJSON, current.SpeechStyleJSON)
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	updatedAt := c.UpdatedAt.UTC()
	if updatedAt.IsZero() {
		if c.CreatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		} else {
			updatedAt = c.CreatedAt.UTC()
		}
	}
	createdAt := c.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = updatedAt
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO character_states (
			chat_session_id, character_name, appearance_json, personality_json,
			status_json, relationships_json, speech_style_json, turn_index,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, next.ChatSessionID, next.CharacterName, nullableJSONText(next.AppearanceJSON), nullableJSONText(next.PersonalityJSON),
		nullableJSONText(next.StatusJSON), nullableJSONText(next.RelationshipsJSON), nullableJSONText(next.SpeechStyleJSON),
		next.TurnIndex, createdAt, updatedAt)
	return err
}

func (m *mariadbStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return mariaListCharacterStates(ctx, m.db, chatSessionID)
}

func (m *mariadbStore) ListCharacterStateHistory(ctx context.Context, chatSessionID, characterName string, limit, offset int) ([]CharacterState, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return mariaListCharacterStateHistory(ctx, m.db, chatSessionID, characterName, limit, offset)
}

func (m *mariadbStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item CharacterState
	var appearanceJSON, personalityJSON, statusJSON, relationshipsJSON, speechStyleJSON sql.NullString
	var turnIndex sql.NullInt64
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, character_name, appearance_json, personality_json, status_json,
			   relationships_json, speech_style_json, turn_index, created_at, updated_at
		FROM character_states
		WHERE chat_session_id = ? AND character_name = ?
		ORDER BY turn_index DESC, id DESC
		LIMIT 1
	`, chatSessionID, characterName).Scan(&item.ID, &item.ChatSessionID, &item.CharacterName,
		&appearanceJSON, &personalityJSON, &statusJSON, &relationshipsJSON, &speechStyleJSON,
		&turnIndex, &item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.AppearanceJSON = stringFromNull(appearanceJSON)
	item.PersonalityJSON = stringFromNull(personalityJSON)
	item.StatusJSON = stringFromNull(statusJSON)
	item.RelationshipsJSON = stringFromNull(relationshipsJSON)
	item.SpeechStyleJSON = stringFromNull(speechStyleJSON)
	item.TurnIndex = intFromNull(turnIndex)
	return &item, nil
}

func (m *mariadbStore) SavePendingThread(ctx context.Context, p *PendingThread) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	now := nonZeroTime(p.UpdatedAt)
	result, err := m.db.ExecContext(ctx, `
		UPDATE pending_threads
		SET description = COALESCE(?, description),
			status = COALESCE(?, status),
			resolved_turn = NULLIF(?, 0),
			source_turn = NULLIF(?, 0),
			priority = NULLIF(?, 0),
			hook_type = COALESCE(?, hook_type),
			hook_metadata_json = COALESCE(?, hook_metadata_json),
			pinned = ?,
			suppressed = ?,
			user_corrected = ?,
			updated_at = ?
		WHERE chat_session_id = ? AND thread_key = ? AND status <> 'resolved'
		ORDER BY id DESC
		LIMIT 1
	`, nullableString(p.Description), nullableString(firstNonEmptyString(p.Status, "open")),
		p.ResolvedTurn, p.SourceTurn, p.Priority, nullableString(p.HookType),
		nullableString(firstNonEmptyString(p.HookMetadataJSON, p.DetailsJSON)),
		p.Pinned, p.Suppressed, p.UserCorrected, now, p.ChatSessionID, p.ThreadKey)
	if err != nil {
		return err
	}
	if affected, rowErr := result.RowsAffected(); rowErr == nil && affected > 0 {
		return nil
	}
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO pending_threads (
			chat_session_id, thread_key, description, status, created_turn,
			resolved_turn, source_turn, priority, hook_type, hook_metadata_json,
			pinned, suppressed, user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ChatSessionID, p.ThreadKey, nullableString(p.Description), firstNonEmptyString(p.Status, "open"),
		p.CreatedTurn, p.ResolvedTurn, p.SourceTurn, p.Priority, nullableString(p.HookType),
		nullableString(firstNonEmptyString(p.HookMetadataJSON, p.DetailsJSON)), p.Pinned, p.Suppressed,
		p.UserCorrected, nonZeroTime(p.CreatedAt), now)
	return err
}

func (m *mariadbStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, chat_session_id, thread_key, description, status, created_turn, resolved_turn,
			   source_turn, priority, hook_type, hook_metadata_json, pinned, suppressed, user_corrected,
			   created_at, updated_at
		FROM pending_threads
		WHERE chat_session_id = ?
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(status) != "" && status != "all" {
		query += ` AND status = ?`
		args = append(args, status)
	} else if status == "" {
		query += ` AND status IN ('open', 'paused')`
	}
	query += ` ORDER BY pinned DESC, source_turn DESC, id DESC`
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PendingThread
	for rows.Next() {
		var item PendingThread
		var description, hookType, hookMetadataJSON sql.NullString
		var createdTurn, resolvedTurn, sourceTurn, priority sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.ThreadKey, &description, &item.Status,
			&createdTurn, &resolvedTurn, &sourceTurn, &priority, &hookType, &hookMetadataJSON,
			&item.Pinned, &item.Suppressed, &item.UserCorrected, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Description = stringFromNull(description)
		item.CreatedTurn = intFromNull(createdTurn)
		item.ResolvedTurn = intFromNull(resolvedTurn)
		item.SourceTurn = intFromNull(sourceTurn)
		item.Priority = intFromNull(priority)
		item.HookType = stringFromNull(hookType)
		item.HookMetadataJSON = stringFromNull(hookMetadataJSON)
		hydratePendingThreadDerivedFields(&item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) PatchPendingThread(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	return m.patchPendingThreadFields(ctx, hookID, updates, []string{
		"status", "thread_type", "title", "owner", "target", "confidence", "details_json", "resolution_note",
	})
}

func (m *mariadbStore) PatchPendingThreadTrust(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	return m.patchPendingThreadFields(ctx, hookID, updates, []string{"pinned", "suppressed", "user_corrected"})
}

func (m *mariadbStore) patchPendingThreadFields(ctx context.Context, hookID int64, updates map[string]any, fieldOrder []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	currentMetadata, exists, err := m.pendingThreadMetadata(ctx, hookID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	metadata := pendingThreadMetadataMap(currentMetadata)
	updatedFields := make([]string, 0, len(updates))
	setParts := make([]string, 0, len(updates)+2)
	args := make([]any, 0, len(updates)+3)
	metadataChanged := false
	for _, field := range fieldOrder {
		val, ok := updates[field]
		if !ok {
			continue
		}
		switch field {
		case "status":
			setParts = append(setParts, "status = ?")
			args = append(args, val)
			metadata["status"] = val
			metadataChanged = true
		case "thread_type":
			setParts = append(setParts, "hook_type = ?")
			args = append(args, val)
			metadata["thread_type"] = val
			metadataChanged = true
		case "title":
			setParts = append(setParts, "description = ?")
			args = append(args, val)
			metadata["title"] = val
			metadataChanged = true
		case "owner", "target", "confidence", "details_json", "resolution_note":
			if val == nil {
				delete(metadata, field)
			} else {
				metadata[field] = val
			}
			metadataChanged = true
		case "pinned", "suppressed", "user_corrected":
			setParts = append(setParts, field+" = ?")
			args = append(args, val)
		default:
			continue
		}
		updatedFields = append(updatedFields, field)
	}
	if metadataChanged {
		setParts = append(setParts, "hook_metadata_json = ?")
		args = append(args, nullableJSONText(compactPendingThreadMetadata(metadata)))
	}
	if len(setParts) == 0 {
		return updatedFields, nil
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, hookID)
	res, err := m.db.ExecContext(ctx, "UPDATE pending_threads SET "+strings.Join(setParts, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}
	_, _ = res.RowsAffected()
	return updatedFields, nil
}

func (m *mariadbStore) DeletePendingThread(ctx context.Context, hookID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM pending_threads WHERE id = ?", hookID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) pendingThreadMetadata(ctx context.Context, hookID int64) (string, bool, error) {
	var raw sql.NullString
	err := m.db.QueryRowContext(ctx, "SELECT hook_metadata_json FROM pending_threads WHERE id = ?", hookID).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return stringFromNull(raw), true, nil
}

func pendingThreadMetadataMap(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func compactPendingThreadMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(b)
}

func hydratePendingThreadDerivedFields(item *PendingThread) {
	metadata := pendingThreadMetadataMap(item.HookMetadataJSON)
	if item.ThreadType == "" {
		item.ThreadType = firstNonEmptyString(pendingThreadStringMeta(metadata, "thread_type"), item.HookType)
	}
	if item.Title == "" {
		item.Title = firstNonEmptyString(pendingThreadStringMeta(metadata, "title"), item.Description, item.ThreadKey)
	}
	if item.Owner == "" {
		item.Owner = pendingThreadStringMeta(metadata, "owner")
	}
	if item.Target == "" {
		item.Target = pendingThreadStringMeta(metadata, "target")
	}
	if item.ResolutionNote == "" {
		item.ResolutionNote = pendingThreadStringMeta(metadata, "resolution_note")
	}
	if item.DetailsJSON == "" {
		item.DetailsJSON = pendingThreadStringMeta(metadata, "details_json")
	}
	if item.LastSeenTurn == 0 {
		item.LastSeenTurn = pendingThreadIntMeta(metadata, "last_seen_turn")
	}
	if item.Confidence == 0 {
		item.Confidence = pendingThreadFloatMeta(metadata, "confidence")
	}
}

func pendingThreadStringMeta(metadata map[string]any, key string) string {
	if text, ok := metadata[key].(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func pendingThreadIntMeta(metadata map[string]any, key string) int {
	switch typed := metadata[key].(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func pendingThreadFloatMeta(metadata map[string]any, key string) float64 {
	switch typed := metadata[key].(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	default:
		return 0
	}
}

func (m *mariadbStore) SaveActiveState(ctx context.Context, a *ActiveState) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO active_states (chat_session_id, state_type, content, turn_index, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, a.ChatSessionID, a.StateType, a.Content, a.TurnIndex, nonZeroTime(a.CreatedAt))
	return err
}

func (m *mariadbStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, chat_session_id, state_type, content, turn_index, created_at
		FROM active_states
		WHERE chat_session_id = ? AND (? = '' OR state_type = ?)
		ORDER BY turn_index DESC, id DESC
	`
	rows, err := m.db.QueryContext(ctx, query, chatSessionID, strings.TrimSpace(stateType), stateType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ActiveState
	for rows.Next() {
		var item ActiveState
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.StateType, &item.Content, &item.TurnIndex, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, chat_session_id, layer_type, content, source_state_type, turn_index, source_turn,
			   source_record, last_verified_turn, confidence, created_at
		FROM canonical_state_layers
		WHERE chat_session_id = ? AND (? = '' OR layer_type = ?)
		ORDER BY turn_index DESC, id DESC
	`
	rows, err := m.db.QueryContext(ctx, query, chatSessionID, strings.TrimSpace(layerType), layerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CanonicalStateLayer
	for rows.Next() {
		var item CanonicalStateLayer
		var sourceStateType sql.NullString
		var sourceTurn, sourceRecord, lastVerifiedTurn sql.NullInt64
		var confidence sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.LayerType, &item.Content,
			&sourceStateType, &item.TurnIndex, &sourceTurn, &sourceRecord, &lastVerifiedTurn,
			&confidence, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.SourceStateType = stringFromNull(sourceStateType)
		item.SourceTurn = intFromNull(sourceTurn)
		item.SourceRecord = int64FromNull(sourceRecord)
		item.LastVerifiedTurn = intFromNull(lastVerifiedTurn)
		item.Confidence = float64FromNull(confidence)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveCanonicalStateLayer(ctx context.Context, item *CanonicalStateLayer) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO canonical_state_layers (
			chat_session_id, layer_type, content, source_state_type, turn_index,
			source_turn, source_record, last_verified_turn, confidence, created_at
		)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), NULLIF(?, 0), ?, ?)
	`, item.ChatSessionID, item.LayerType, item.Content, nullableString(item.SourceStateType), item.TurnIndex,
		item.SourceTurn, item.SourceRecord, item.LastVerifiedTurn, item.Confidence, nonZeroTime(item.CreatedAt))
	return err
}

func (m *mariadbStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, chat_session_id, from_turn, to_turn, summary_text, key_entities, key_events,
			   open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
		FROM episode_summaries
		WHERE chat_session_id = ? AND (? <= 0 OR from_turn >= ?) AND (? <= 0 OR to_turn <= ?)
		ORDER BY to_turn DESC, id DESC
	`
	args := []any{chatSessionID, fromTurn, fromTurn, toTurn, toTurn}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EpisodeSummary
	for rows.Next() {
		var item EpisodeSummary
		var keyEntities, keyEvents, openLoopsJSON, relationshipChangesJSON, embeddingVector, embeddingModel sql.NullString
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &item.SummaryText,
			&keyEntities, &keyEvents, &openLoopsJSON, &relationshipChangesJSON, &embeddingVector, &embeddingModel,
			&item.CreatedAt); err != nil {
			return nil, err
		}
		item.KeyEntities = stringFromNull(keyEntities)
		item.KeyEvents = stringFromNull(keyEvents)
		item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
		item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item EpisodeSummary
	var keyEntities, keyEvents, openLoopsJSON, relationshipChangesJSON, embeddingVector, embeddingModel sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, summary_text, key_entities, key_events,
			   open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
		FROM episode_summaries
		WHERE id = ?
	`, episodeID).Scan(&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &item.SummaryText,
		&keyEntities, &keyEvents, &openLoopsJSON, &relationshipChangesJSON, &embeddingVector, &embeddingModel,
		&item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.KeyEntities = stringFromNull(keyEntities)
	item.KeyEvents = stringFromNull(keyEvents)
	item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
	item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) SaveEpisodeSummary(ctx context.Context, item *EpisodeSummary) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO episode_summaries (
			chat_session_id, from_turn, to_turn, summary_text, key_entities, key_events,
			open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, item.SummaryText, nullableString(item.KeyEntities),
		nullableString(item.KeyEvents), nullableString(item.OpenLoopsJSON), nullableString(item.RelationshipChangesJSON),
		nullableString(item.EmbeddingVector), nullableString(item.EmbeddingModel), nonZeroTime(item.CreatedAt))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) DeleteEpisodeSummary(ctx context.Context, episodeID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM episode_summaries WHERE id = ?", episodeID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) DeleteEpisodeSummariesInRange(ctx context.Context, chatSessionID string, fromTurn, toTurn int) (int64, error) {
	if err := m.ensureDB(); err != nil {
		return 0, err
	}
	if fromTurn > 0 && toTurn > 0 && fromTurn > toTurn {
		fromTurn, toTurn = toTurn, fromTurn
	}
	res, err := m.db.ExecContext(ctx, `
		DELETE FROM episode_summaries
		WHERE chat_session_id = ?
		  AND (? <= 0 OR to_turn >= ?)
		  AND (? <= 0 OR from_turn <= ?)
	`, chatSessionID, fromTurn, fromTurn, toTurn, toTurn)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ---------------------------------------------------------------------------
// SessionMigrationStore implementation
// ---------------------------------------------------------------------------
