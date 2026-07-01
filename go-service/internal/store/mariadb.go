package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
type mariadbStore struct {
	db *sql.DB
}

// OpenMariaDB returns a Store backed by MariaDB.
// Empty DSNs stay disabled so default local runs never touch a live database.
func OpenMariaDB(dsn string) (Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, ErrNotEnabled
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(2 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &mariadbStore{db: db}, nil
}

func (m *mariadbStore) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

func (m *mariadbStore) ensureDB() error {
	if m == nil || m.db == nil {
		return ErrNotEnabled
	}
	return nil
}

func (m *mariadbStore) Ping(ctx context.Context) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	return m.db.PingContext(ctx)
}

var mariaAdminResetTables = []string{
	"persona_capsule_attachments",
	"persona_memory_entries",
	"persona_memory_capsules",
	"protagonist_entity_memories",
	"effective_input_logs",
	"chat_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"consequence_records",
	"psychology_branches",
	"session_fork_lineage",
	"theme_offscreen_carries",
	"capture_verification_records",
	"status_effects",
	"status_change_events",
	"status_current_values",
	"status_schema_registry",
	"status_schema_proposals",
	"character_events",
	"trust_states",
	"storylines",
	"world_rules",
	"session_active_scopes",
	"character_states",
	"pending_threads",
	"active_states",
	"canonical_state_layers",
	"guidance_plan_states",
	"episode_summaries",
	"chapter_summaries",
	"arc_summaries",
	"saga_digests",
	"entities",
}

func (m *mariadbStore) ResetAll(ctx context.Context) (AdminResetResult, error) {
	var result AdminResetResult
	if err := m.ensureDB(); err != nil {
		return result, err
	}
	conn, err := m.db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return result, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS=1")
	}()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, table := range mariaAdminResetTables {
		res, err := tx.ExecContext(ctx, "DELETE FROM "+mariaQuoteIdentifier(table))
		if err != nil {
			return result, err
		}
		if rows, err := res.RowsAffected(); err == nil {
			result.RowsDeleted += rows
		}
		result.TablesCleared++
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	return result, nil
}

func mariaQuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

type mariaQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (m *mariadbStore) ReadSessionStateSnapshot(ctx context.Context, chatSessionID string) (*SessionStateSnapshot, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	activeStates, err := mariaListActiveStates(ctx, tx, chatSessionID, "")
	if err != nil {
		return nil, err
	}
	canonicalLayers, err := mariaListCanonicalStateLayers(ctx, tx, chatSessionID, "")
	if err != nil {
		return nil, err
	}
	storylines, err := mariaListStorylines(ctx, tx, chatSessionID)
	if err != nil {
		return nil, err
	}
	characters, err := mariaListCharacterStates(ctx, tx, chatSessionID)
	if err != nil {
		return nil, err
	}
	worldRules, err := mariaListWorldRules(ctx, tx, chatSessionID)
	if err != nil {
		return nil, err
	}
	pendingThreads, err := mariaListPendingThreads(ctx, tx, chatSessionID, "")
	if err != nil {
		return nil, err
	}
	characterEvents, err := mariaListCharacterEvents(ctx, tx, chatSessionID, "")
	if err != nil {
		return nil, err
	}
	recentChatLogs, err := mariaListChatLogs(ctx, tx, chatSessionID, 0, 0)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &SessionStateSnapshot{
		ActiveStates:         activeStates,
		CanonicalStateLayers: canonicalLayers,
		Storylines:           storylines,
		CharacterStates:      characters,
		WorldRules:           worldRules,
		PendingThreads:       pendingThreads,
		CharacterEvents:      characterEvents,
		RecentChatLogs:       recentChatLogs,
		SingleConnection:     true,
		TraceMethods: []string{
			"ListActiveStates",
			"ListCanonicalStateLayers",
			"ListStorylines",
			"ListCharacterStates",
			"ListWorldRules",
			"ListPendingThreads",
			"ListCharacterEvents",
			"ListChatLogs",
		},
	}, nil
}

func mariaListChatLogs(ctx context.Context, q mariaQueryer, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, turn_index, role, content, created_at
		FROM chat_logs
		WHERE chat_session_id = ? AND (? <= 0 OR turn_index >= ?) AND (? <= 0 OR turn_index <= ?)
		ORDER BY turn_index ASC, id ASC
	`, chatSessionID, fromTurn, fromTurn, toTurn, toTurn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChatLog
	for rows.Next() {
		var item ChatLog
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.TurnIndex, &item.Role, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func mariaListActiveStates(ctx context.Context, q mariaQueryer, chatSessionID, stateType string) ([]ActiveState, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, state_type, content, turn_index, created_at
		FROM active_states
		WHERE chat_session_id = ? AND (? = '' OR state_type = ?)
		ORDER BY turn_index DESC, id DESC
	`, chatSessionID, strings.TrimSpace(stateType), stateType)
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

func mariaListCanonicalStateLayers(ctx context.Context, q mariaQueryer, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, layer_type, content, source_state_type, turn_index, source_turn,
			   source_record, last_verified_turn, confidence, created_at
		FROM canonical_state_layers
		WHERE chat_session_id = ? AND (? = '' OR layer_type = ?)
		ORDER BY turn_index DESC, id DESC
	`, chatSessionID, strings.TrimSpace(layerType), layerType)
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

func mariaListStorylines(ctx context.Context, q mariaQueryer, chatSessionID string) ([]Storyline, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, name, status, entities_json, current_context, key_points_json,
			   ongoing_tensions_json, confidence, evidence_count, last_evidence_turn, first_turn, last_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM storylines
		WHERE chat_session_id = ?
		ORDER BY last_turn DESC, id DESC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Storyline
	for rows.Next() {
		var item Storyline
		var entitiesJSON, currentContext, keyPointsJSON, ongoingTensionsJSON sql.NullString
		var confidence sql.NullFloat64
		var evidenceCount, lastEvidenceTurn, firstTurn, lastTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Name, &item.Status,
			&entitiesJSON, &currentContext, &keyPointsJSON, &ongoingTensionsJSON,
			&confidence, &evidenceCount, &lastEvidenceTurn, &firstTurn, &lastTurn,
			&item.Pinned, &item.Suppressed, &item.UserCorrected, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.EntitiesJSON = stringFromNull(entitiesJSON)
		item.CurrentContext = stringFromNull(currentContext)
		item.KeyPointsJSON = stringFromNull(keyPointsJSON)
		item.OngoingTensionsJSON = stringFromNull(ongoingTensionsJSON)
		item.Confidence = float64FromNull(confidence)
		item.EvidenceCount = intFromNull(evidenceCount)
		item.LastEvidenceTurn = intFromNull(lastEvidenceTurn)
		item.FirstTurn = intFromNull(firstTurn)
		item.LastTurn = intFromNull(lastTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func mariaListCharacterStates(ctx context.Context, q mariaQueryer, chatSessionID string) ([]CharacterState, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, character_name, appearance_json, personality_json, status_json,
			   relationships_json, speech_style_json, turn_index, created_at, updated_at
		FROM character_states
		WHERE chat_session_id = ?
		ORDER BY turn_index DESC, id DESC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CharacterState
	seen := map[string]bool{}
	for rows.Next() {
		var item CharacterState
		var appearanceJSON, personalityJSON, statusJSON, relationshipsJSON, speechStyleJSON sql.NullString
		var turnIndex sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.CharacterName,
			&appearanceJSON, &personalityJSON, &statusJSON, &relationshipsJSON, &speechStyleJSON,
			&turnIndex, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.AppearanceJSON = stringFromNull(appearanceJSON)
		item.PersonalityJSON = stringFromNull(personalityJSON)
		item.StatusJSON = stringFromNull(statusJSON)
		item.RelationshipsJSON = stringFromNull(relationshipsJSON)
		item.SpeechStyleJSON = stringFromNull(speechStyleJSON)
		item.TurnIndex = intFromNull(turnIndex)
		key := strings.ToLower(strings.TrimSpace(item.CharacterName))
		if key == "" {
			key = strconv.FormatInt(item.ID, 10)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out, rows.Err()
}

func mariaListCharacterStateHistory(ctx context.Context, q mariaQueryer, chatSessionID, characterName string, limit, offset int) ([]CharacterState, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, character_name, appearance_json, personality_json, status_json,
			   relationships_json, speech_style_json, turn_index, created_at, updated_at
		FROM character_states
		WHERE chat_session_id = ? AND character_name = ?
		ORDER BY turn_index DESC, id DESC
		LIMIT ? OFFSET ?
	`, chatSessionID, characterName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CharacterState
	for rows.Next() {
		var item CharacterState
		var appearanceJSON, personalityJSON, statusJSON, relationshipsJSON, speechStyleJSON sql.NullString
		var turnIndex sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.CharacterName,
			&appearanceJSON, &personalityJSON, &statusJSON, &relationshipsJSON, &speechStyleJSON,
			&turnIndex, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.AppearanceJSON = stringFromNull(appearanceJSON)
		item.PersonalityJSON = stringFromNull(personalityJSON)
		item.StatusJSON = stringFromNull(statusJSON)
		item.RelationshipsJSON = stringFromNull(relationshipsJSON)
		item.SpeechStyleJSON = stringFromNull(speechStyleJSON)
		item.TurnIndex = intFromNull(turnIndex)
		out = append(out, item)
	}
	return out, rows.Err()
}

func mariaListWorldRules(ctx context.Context, q mariaQueryer, chatSessionID string) ([]WorldRule, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, chat_session_id, scope, scope_name, category, `+"`key`"+`, value_json, genre, source_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM world_rules
		WHERE chat_session_id = ?
		ORDER BY scope, category, `+"`key`"+`
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorldRule
	for rows.Next() {
		var item WorldRule
		var scopeName, genre sql.NullString
		var valueJSON sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Scope, &scopeName, &item.Category, &item.Key,
			&valueJSON, &genre, &sourceTurn, &item.Pinned, &item.Suppressed, &item.UserCorrected,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScopeName = stringFromNull(scopeName)
		item.ValueJSON = stringFromNull(valueJSON)
		item.Genre = stringFromNull(genre)
		item.SourceTurn = intFromNull(sourceTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func mariaListPendingThreads(ctx context.Context, q mariaQueryer, chatSessionID, status string) ([]PendingThread, error) {
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
	rows, err := q.QueryContext(ctx, query, args...)
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

func mariaListCharacterEvents(ctx context.Context, q mariaQueryer, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	rows, err := q.QueryContext(ctx, `
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

func nonZeroTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullableJSONText(v string) any {
	text := strings.TrimSpace(v)
	if text == "" || text == "null" {
		return nil
	}
	return text
}

func stringFromNull(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intFromNull(v sql.NullInt64) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int64)
}

func int64FromNull(v sql.NullInt64) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func timeFromNull(v sql.NullTime) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

func float64FromNull(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}

func (m *mariadbStore) SaveChatLog(ctx context.Context, log *ChatLog) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	role := strings.ToLower(strings.TrimSpace(log.Role))
	if strings.TrimSpace(log.ChatSessionID) != "" && log.TurnIndex >= 0 && role != "" {
		var existingID int64
		var existingContent string
		err := m.db.QueryRowContext(ctx, `
			SELECT id, content
			FROM chat_logs
			WHERE chat_session_id = ? AND turn_index = ? AND LOWER(TRIM(role)) = ?
			ORDER BY id ASC
			LIMIT 1
		`, log.ChatSessionID, log.TurnIndex, role).Scan(&existingID, &existingContent)
		if err == nil {
			if strings.TrimSpace(existingContent) == strings.TrimSpace(log.Content) {
				log.ID = existingID
				return nil
			}
			return fmt.Errorf("chat log role conflict for session %s turn %d role %s", log.ChatSessionID, log.TurnIndex, role)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, log.ChatSessionID, log.TurnIndex, log.Role, log.Content, nonZeroTime(log.CreatedAt))
	return err
}

func (m *mariadbStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, chat_session_id, turn_index, role, content, created_at
		FROM chat_logs
		WHERE chat_session_id = ? AND (? <= 0 OR turn_index >= ?) AND (? <= 0 OR turn_index <= ?)
		ORDER BY turn_index ASC, id ASC
	`
	rows, err := m.db.QueryContext(ctx, query, chatSessionID, fromTurn, fromTurn, toTurn, toTurn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChatLog
	for rows.Next() {
		var item ChatLog
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.TurnIndex, &item.Role, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO effective_input_logs (chat_session_id, turn_index, effective_input, created_at)
		VALUES (?, ?, ?, ?)
	`, in.ChatSessionID, in.TurnIndex, in.EffectiveInput, nonZeroTime(in.CreatedAt))
	return err
}

func (m *mariadbStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item EffectiveInput
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, turn_index, effective_input, created_at
		FROM effective_input_logs
		WHERE chat_session_id = ? AND turn_index = ?
		ORDER BY id DESC
		LIMIT 1
	`, chatSessionID, turnIndex).Scan(&item.ID, &item.ChatSessionID, &item.TurnIndex, &item.EffectiveInput, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *mariadbStore) ListEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]EffectiveInput, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, turn_index, effective_input, created_at
		FROM effective_input_logs
		WHERE chat_session_id = ? AND (? <= 0 OR turn_index >= ?) AND (? <= 0 OR turn_index <= ?)
		ORDER BY turn_index ASC, id ASC
	`, chatSessionID, fromTurn, fromTurn, toTurn, toTurn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []EffectiveInput{}
	for rows.Next() {
		var item EffectiveInput
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.TurnIndex, &item.EffectiveInput, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveMemory(ctx context.Context, mem *Memory) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO memories (
			chat_session_id, turn_index, summary_json, embedding, embedding_model,
			importance, emotional_boost, evidence, emotional_intensity,
			narrative_significance, place_wing, place_room, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, mem.ChatSessionID, mem.TurnIndex, nullableString(mem.SummaryJSON), nullableString(mem.Embedding),
		nullableString(mem.EmbeddingModel), mem.Importance, mem.EmotionalBoost, nullableString(mem.Evidence),
		mem.EmotionalIntensity, mem.NarrativeSignificance, nullableString(mem.PlaceWing),
		nullableString(mem.PlaceRoom), nonZeroTime(mem.CreatedAt))
	if err == nil {
		if id, idErr := res.LastInsertId(); idErr == nil {
			mem.ID = id
		}
	}
	return err
}

func (m *mariadbStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, turn_index, summary_json, embedding, embedding_model,
			importance, emotional_boost, evidence, emotional_intensity,
			narrative_significance, place_wing, place_room, created_at
		FROM memories
		WHERE chat_session_id = ? AND (? <= 0 OR turn_index >= ?) AND (? <= 0 OR turn_index <= ?)
		ORDER BY turn_index ASC, id ASC
	`, chatSessionID, fromTurn, fromTurn, toTurn, toTurn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var item Memory
		var summaryJSON, embedding, embeddingModel, evidence, placeWing, placeRoom sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.TurnIndex, &summaryJSON, &embedding,
			&embeddingModel, &item.Importance, &item.EmotionalBoost, &evidence,
			&item.EmotionalIntensity, &item.NarrativeSignificance, &placeWing,
			&placeRoom, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.SummaryJSON = stringFromNull(summaryJSON)
		item.Embedding = stringFromNull(embedding)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		item.Evidence = stringFromNull(evidence)
		item.PlaceWing = stringFromNull(placeWing)
		item.PlaceRoom = stringFromNull(placeRoom)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) UpdateMemoryImportance(ctx context.Context, chatSessionID string, memoryID int64, importance float64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		UPDATE memories
		SET importance = ?
		WHERE id = ? AND chat_session_id = ?
	`, importance, memoryID, chatSessionID)
	return err
}

func (m *mariadbStore) UpdateMemoryExplorerFields(ctx context.Context, chatSessionID string, memoryID int64, patch MemoryExplorerPatch) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	set := []string{}
	args := []any{}
	if patch.SummaryJSON != nil {
		set = append(set, "summary_json = ?")
		args = append(args, *patch.SummaryJSON)
	}
	if patch.Importance != nil {
		set = append(set, "importance = ?")
		args = append(args, *patch.Importance)
	}
	if patch.PlaceWing != nil {
		set = append(set, "place_wing = ?")
		args = append(args, *patch.PlaceWing)
	}
	if patch.PlaceRoom != nil {
		set = append(set, "place_room = ?")
		args = append(args, *patch.PlaceRoom)
	}
	if len(set) == 0 {
		return ErrNotEnabled
	}
	args = append(args, memoryID, chatSessionID)
	_, err := m.db.ExecContext(ctx, `
		UPDATE memories
		SET `+strings.Join(set, ", ")+`
		WHERE id = ? AND chat_session_id = ?
	`, args...)
	return err
}

func (m *mariadbStore) UpdateKGTripleExplorerFields(ctx context.Context, chatSessionID string, tripleID int64, patch KGTripleExplorerPatch) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	set := []string{}
	args := []any{}
	if patch.Subject != nil {
		set = append(set, "subject = ?")
		args = append(args, *patch.Subject)
	}
	if patch.Predicate != nil {
		set = append(set, "predicate = ?")
		args = append(args, *patch.Predicate)
	}
	if patch.Object != nil {
		set = append(set, "object = ?")
		args = append(args, *patch.Object)
	}
	if patch.ValidFrom.Set {
		set = append(set, "valid_from = ?")
		args = append(args, optionalIntArg(patch.ValidFrom))
	}
	if patch.ValidTo.Set {
		set = append(set, "valid_to = ?")
		args = append(args, optionalIntArg(patch.ValidTo))
	}
	if len(set) == 0 {
		return ErrNotEnabled
	}
	args = append(args, tripleID, chatSessionID)
	_, err := m.db.ExecContext(ctx, `
		UPDATE kg_triples
		SET `+strings.Join(set, ", ")+`
		WHERE id = ? AND chat_session_id = ?
	`, args...)
	return err
}

func (m *mariadbStore) DeleteMemoryByID(ctx context.Context, chatSessionID string, memoryID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM memories
		WHERE id = ? AND chat_session_id = ?
	`, memoryID, chatSessionID)
	return err
}

func (m *mariadbStore) DeleteDirectEvidenceByID(ctx context.Context, chatSessionID string, recordID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM direct_evidence_records
		WHERE id = ? AND chat_session_id = ?
	`, recordID, chatSessionID)
	return err
}

func (m *mariadbStore) DeleteKGTripleByID(ctx context.Context, chatSessionID string, tripleID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		DELETE FROM kg_triples
		WHERE id = ? AND chat_session_id = ?
	`, tripleID, chatSessionID)
	return err
}

func (m *mariadbStore) DeleteCharacterByName(ctx context.Context, chatSessionID string, characterName string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	name := strings.TrimSpace(characterName)
	if name == "" {
		return ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM character_states WHERE chat_session_id = ? AND character_name = ?", chatSessionID, name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM character_events WHERE chat_session_id = ? AND character_name = ?", chatSessionID, name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entities
		WHERE chat_session_id = ?
		  AND name = ?
		  AND (entity_type IS NULL OR entity_type = '' OR LOWER(entity_type) IN ('character', 'person', 'npc', 'protagonist', 'persona', 'player'))
	`, chatSessionID, name); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *mariadbStore) UpdateDirectEvidenceExplorerFields(ctx context.Context, chatSessionID string, recordID int64, patch DirectEvidenceExplorerPatch) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	set := []string{}
	args := []any{}
	if patch.ArchiveState != nil {
		set = append(set, "archive_state = ?")
		args = append(args, *patch.ArchiveState)
	}
	if patch.CaptureVerification != nil {
		set = append(set, "capture_verification = ?")
		args = append(args, *patch.CaptureVerification)
	}
	if patch.CommittedGate != nil {
		set = append(set, "committed_gate = ?")
		args = append(args, *patch.CommittedGate)
	}
	if patch.RepairNeeded != nil {
		set = append(set, "repair_needed = ?")
		args = append(args, *patch.RepairNeeded)
	}
	if patch.Tombstoned != nil {
		set = append(set, "tombstoned = ?")
		args = append(args, *patch.Tombstoned)
	}
	if patch.SupersededByID.Set {
		set = append(set, "superseded_by_id = ?")
		args = append(args, optionalIntArg(patch.SupersededByID))
	}
	if len(set) == 0 {
		return ErrNotEnabled
	}
	args = append(args, recordID, chatSessionID)
	_, err := m.db.ExecContext(ctx, `
		UPDATE direct_evidence_records
		SET `+strings.Join(set, ", ")+`
		WHERE id = ? AND chat_session_id = ?
	`, args...)
	return err
}

func optionalIntArg(value OptionalIntPatch) any {
	if value.Value == nil {
		return nil
	}
	return *value.Value
}

func (m *mariadbStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO direct_evidence_records (
			chat_session_id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
			turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
			capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
			superseded_by_id, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ChatSessionID, e.EvidenceKind, e.EvidenceText, e.SourceTurnStart, e.SourceTurnEnd,
		e.TurnAnchor, nullableString(e.SourceMessageIDsJSON), nullableString(e.SourceHash),
		e.ArchiveState, e.CaptureStage, e.CaptureVerification, nullableString(e.CommittedGate),
		nullableString(e.LineageJSON), e.RepairNeeded, e.Tombstoned, e.SupersededByID, nonZeroTime(e.CreatedAt))
	return err
}

func (m *mariadbStore) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
			turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
			capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
			superseded_by_id, created_at
		FROM direct_evidence_records
		WHERE chat_session_id = ?
		ORDER BY source_turn_start ASC, id ASC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DirectEvidence
	for rows.Next() {
		var item DirectEvidence
		var turnAnchor, supersededByID sql.NullInt64
		var sourceMessageIDsJSON, sourceHash, committedGate, lineageJSON sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.EvidenceKind, &item.EvidenceText,
			&item.SourceTurnStart, &item.SourceTurnEnd, &turnAnchor,
			&sourceMessageIDsJSON, &sourceHash, &item.ArchiveState, &item.CaptureStage,
			&item.CaptureVerification, &committedGate, &lineageJSON, &item.RepairNeeded,
			&item.Tombstoned, &supersededByID, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.TurnAnchor = intFromNull(turnAnchor)
		item.SourceMessageIDsJSON = stringFromNull(sourceMessageIDsJSON)
		item.SourceHash = stringFromNull(sourceHash)
		item.CommittedGate = stringFromNull(committedGate)
		item.LineageJSON = stringFromNull(lineageJSON)
		item.SupersededByID = int64FromNull(supersededByID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveKGTriple(ctx context.Context, t *KGTriple) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO kg_triples (chat_session_id, subject, predicate, object, valid_from, valid_to, source_turn, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ChatSessionID, t.Subject, t.Predicate, t.Object, t.ValidFrom, t.ValidTo, t.SourceTurn, nonZeroTime(t.CreatedAt))
	return err
}

func (m *mariadbStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, subject, predicate, object, valid_from, valid_to, source_turn, created_at
		FROM kg_triples
		WHERE chat_session_id = ?
		ORDER BY id ASC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KGTriple
	for rows.Next() {
		var item KGTriple
		var validFrom, validTo, sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Subject, &item.Predicate,
			&item.Object, &validFrom, &validTo, &sourceTurn, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.ValidFrom = intFromNull(validFrom)
		item.ValidTo = intFromNull(validTo)
		item.SourceTurn = intFromNull(sourceTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveAuditLog(ctx context.Context, a *AuditLog) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO audit_logs (created_at, event_type, chat_session_id, target_type, target_id, summary, details_json, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, nonZeroTime(a.CreatedAt), a.EventType, nullableString(a.ChatSessionID), nullableString(a.TargetType),
		a.TargetID, nullableString(a.Summary), nullableString(a.DetailsJSON), nullableString(a.Source))
	return err
}

func (m *mariadbStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, created_at, event_type, chat_session_id, target_type, target_id, summary, details_json, source
		FROM audit_logs
		WHERE (? = '' OR chat_session_id = ?) AND (? = '' OR event_type = ?)
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, strings.TrimSpace(chatSessionID), chatSessionID, strings.TrimSpace(eventType), eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditLog
	for rows.Next() {
		var item AuditLog
		var sid, targetType, summary, detailsJSON, source sql.NullString
		var targetID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.EventType, &sid, &targetType,
			&targetID, &summary, &detailsJSON, &source); err != nil {
			return nil, err
		}
		item.ChatSessionID = stringFromNull(sid)
		item.TargetType = stringFromNull(targetType)
		item.TargetID = int64FromNull(targetID)
		item.Summary = stringFromNull(summary)
		item.DetailsJSON = stringFromNull(detailsJSON)
		item.Source = stringFromNull(source)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) CountAuditLogs(ctx context.Context, chatSessionID string, eventType string) (int, error) {
	if err := m.ensureDB(); err != nil {
		return 0, err
	}
	var total int
	if err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE (? = '' OR chat_session_id = ?) AND (? = '' OR event_type = ?)
	`, strings.TrimSpace(chatSessionID), chatSessionID, strings.TrimSpace(eventType), eventType).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (m *mariadbStore) SaveSupersessionResolution(ctx context.Context, d *SupersessionResolutionDecision) (*SupersessionResolutionRecord, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	decision, err := normalizeSupersessionResolutionDecision(d)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	detailsJSON := mariaSupersessionResolutionDetailsJSON(decision)
	summary := mariaSupersessionResolutionSummary(decision)
	source := firstNonEmptyString(decision.Operator, "critic")

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (created_at, event_type, chat_session_id, target_type, target_id, summary, details_json, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, now, "supersession_resolution", decision.ChatSessionID, decision.TargetType, decision.TargetID, summary, detailsJSON, source)
	if err != nil {
		return nil, err
	}
	if err := mariaApplySupersessionResolutionState(ctx, tx, decision, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	id, _ := res.LastInsertId()
	return &SupersessionResolutionRecord{
		ID:              id,
		CreatedAt:       now,
		ChatSessionID:   decision.ChatSessionID,
		TargetType:      decision.TargetType,
		TargetID:        decision.TargetID,
		SourceTurn:      decision.SourceTurn,
		ResolutionClass: decision.ResolutionClass,
		NewTargetType:   decision.NewTargetType,
		NewTargetID:     decision.NewTargetID,
		RelationshipKey: decision.RelationshipKey,
		Reason:          decision.Reason,
		DetailsJSON:     detailsJSON,
		Source:          source,
	}, nil
}

func (m *mariadbStore) ListSupersessionResolutions(ctx context.Context, chatSessionID string, limit int) ([]SupersessionResolutionRecord, error) {
	logs, err := m.ListAuditLogs(ctx, chatSessionID, "supersession_resolution", limit)
	if err != nil {
		return nil, err
	}
	out := make([]SupersessionResolutionRecord, 0, len(logs))
	for _, item := range logs {
		out = append(out, mariaSupersessionResolutionRecordFromAudit(item))
	}
	return out, nil
}

func normalizeSupersessionResolutionDecision(in *SupersessionResolutionDecision) (SupersessionResolutionDecision, error) {
	if in == nil {
		return SupersessionResolutionDecision{}, errors.New("supersession resolution decision is required")
	}
	out := *in
	out.ChatSessionID = strings.TrimSpace(out.ChatSessionID)
	out.TargetType = normalizeSupersessionResolutionTargetType(out.TargetType)
	out.NewTargetType = normalizeSupersessionResolutionTargetType(out.NewTargetType)
	out.ResolutionClass = normalizeSupersessionResolutionClass(out.ResolutionClass)
	out.RelationshipKey = strings.TrimSpace(out.RelationshipKey)
	out.Reason = strings.TrimSpace(out.Reason)
	out.EvidenceJSON = strings.TrimSpace(out.EvidenceJSON)
	out.Operator = strings.TrimSpace(out.Operator)
	if out.ChatSessionID == "" {
		return out, errors.New("chat_session_id is required")
	}
	if out.TargetType == "" {
		return out, errors.New("target_type is required")
	}
	if out.TargetID <= 0 {
		return out, errors.New("target_id must be positive")
	}
	if out.NewTargetID > 0 && out.NewTargetType == "" {
		out.NewTargetType = out.TargetType
	}
	return out, nil
}

func normalizeSupersessionResolutionTargetType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "direct_evidence_record", "evidence", "direct_evidence_records":
		return "direct_evidence"
	case "kg", "triple", "kg_triples":
		return "kg_triple"
	case "pending_threads":
		return "pending_thread"
	case "memories":
		return "memory"
	default:
		return value
	}
}

func normalizeSupersessionResolutionClass(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "", "soft", "demote":
		return "soft_demote"
	case "prune", "stale", "stale_close":
		return "stale_demote"
	case "closed", "resolved":
		return "close"
	case "superseded":
		return "supersede"
	case "refined":
		return "refine"
	case "reversed":
		return "reverse"
	default:
		for _, allowed := range []string{"soft_demote", "stale_demote", "close", "supersede", "refine", "reverse"} {
			if value == allowed {
				return value
			}
		}
		return "soft_demote"
	}
}

func mariaSupersessionResolutionDetailsJSON(d SupersessionResolutionDecision) string {
	details := map[string]any{
		"contract_version": SupersessionResolutionContractVersion,
		"resolution_class": d.ResolutionClass,
		"source_turn":      d.SourceTurn,
		"target": map[string]any{
			"type": d.TargetType,
			"id":   d.TargetID,
		},
		"afterglow_turns": SupersessionResolutionAfterglowTurns,
		"hard_delete":     false,
		"semantics": map[string]any{
			"soft_demote":  "lower foreground priority without deleting source history",
			"stale_demote": "stale emotional residue becomes background unless re-supported",
			"close":        "old state is closed and retained as audit history",
			"supersede":    "old state is replaced by a newer state",
			"refine":       "new state narrows or clarifies the same relationship",
			"reverse":      "new state contradicts the previous relationship direction",
		},
	}
	if d.NewTargetID > 0 {
		details["new_target"] = map[string]any{"type": firstNonEmptyString(d.NewTargetType, d.TargetType), "id": d.NewTargetID}
	}
	if d.RelationshipKey != "" {
		details["relationship_key"] = d.RelationshipKey
	}
	if d.Reason != "" {
		details["reason"] = d.Reason
	}
	if d.Operator != "" {
		details["operator"] = d.Operator
	}
	if d.EvidenceJSON != "" {
		var evidence any
		if err := json.Unmarshal([]byte(d.EvidenceJSON), &evidence); err == nil {
			details["evidence"] = evidence
		} else {
			details["evidence_text"] = d.EvidenceJSON
		}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func mariaSupersessionResolutionSummary(d SupersessionResolutionDecision) string {
	action := d.ResolutionClass
	target := fmt.Sprintf("%s #%d", d.TargetType, d.TargetID)
	if d.NewTargetID > 0 {
		return fmt.Sprintf("Resolution %s: %s -> %s #%d", action, target, firstNonEmptyString(d.NewTargetType, d.TargetType), d.NewTargetID)
	}
	if d.Reason != "" {
		return fmt.Sprintf("Resolution %s: %s (%s)", action, target, d.Reason)
	}
	return fmt.Sprintf("Resolution %s: %s", action, target)
}

func mariaSupersessionResolutionRecordFromAudit(a AuditLog) SupersessionResolutionRecord {
	out := SupersessionResolutionRecord{
		ID:            a.ID,
		CreatedAt:     a.CreatedAt,
		ChatSessionID: a.ChatSessionID,
		TargetType:    a.TargetType,
		TargetID:      a.TargetID,
		DetailsJSON:   a.DetailsJSON,
		Source:        a.Source,
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(a.DetailsJSON), &details); err == nil {
		out.ResolutionClass = stringFromAny(details["resolution_class"])
		out.SourceTurn = intFromAny(details["source_turn"])
		out.RelationshipKey = stringFromAny(details["relationship_key"])
		out.Reason = stringFromAny(details["reason"])
		if target := mapFromAny(details["target"]); target != nil {
			out.TargetType = firstNonEmptyString(stringFromAny(target["type"]), out.TargetType)
			if id := int64FromAny(target["id"]); id > 0 {
				out.TargetID = id
			}
		}
		if newTarget := mapFromAny(details["new_target"]); newTarget != nil {
			out.NewTargetType = stringFromAny(newTarget["type"])
			out.NewTargetID = int64FromAny(newTarget["id"])
		}
	}
	return out
}

func mariaApplySupersessionResolutionState(ctx context.Context, tx *sql.Tx, d SupersessionResolutionDecision, now time.Time) error {
	switch d.TargetType {
	case "direct_evidence":
		return mariaApplyDirectEvidenceSupersessionState(ctx, tx, d)
	case "kg_triple":
		return mariaApplyKGTripleSupersessionState(ctx, tx, d)
	case "pending_thread":
		return mariaApplyPendingThreadSupersessionState(ctx, tx, d, now)
	default:
		return nil
	}
}

func mariaApplyDirectEvidenceSupersessionState(ctx context.Context, tx *sql.Tx, d SupersessionResolutionDecision) error {
	switch d.ResolutionClass {
	case "close":
		_, err := tx.ExecContext(ctx, `
			UPDATE direct_evidence_records
			SET archive_state = ?, capture_verification = ?, committed_gate = ?, repair_needed = FALSE
			WHERE id = ? AND chat_session_id = ?
		`, "closed_archive", "closed", "closed_by_resolution", d.TargetID, d.ChatSessionID)
		return err
	case "supersede", "refine", "reverse":
		verification := map[string]string{
			"supersede": "superseded",
			"refine":    "refined",
			"reverse":   "reversed",
		}[d.ResolutionClass]
		var supersededBy any
		if d.NewTargetID > 0 {
			supersededBy = d.NewTargetID
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE direct_evidence_records
			SET archive_state = ?, capture_verification = ?, committed_gate = ?, repair_needed = FALSE, superseded_by_id = ?
			WHERE id = ? AND chat_session_id = ?
		`, "superseded_archive", verification, d.ResolutionClass+"_by_resolution", supersededBy, d.TargetID, d.ChatSessionID)
		return err
	default:
		return nil
	}
}

func mariaApplyKGTripleSupersessionState(ctx context.Context, tx *sql.Tx, d SupersessionResolutionDecision) error {
	if d.SourceTurn <= 0 {
		return nil
	}
	switch d.ResolutionClass {
	case "close", "supersede", "refine", "reverse", "stale_demote":
		_, err := tx.ExecContext(ctx, `
			UPDATE kg_triples
			SET valid_to = ?
			WHERE id = ? AND chat_session_id = ? AND (valid_to IS NULL OR valid_to = 0 OR valid_to > ?)
		`, d.SourceTurn, d.TargetID, d.ChatSessionID, d.SourceTurn)
		return err
	default:
		return nil
	}
}

func mariaApplyPendingThreadSupersessionState(ctx context.Context, tx *sql.Tx, d SupersessionResolutionDecision, now time.Time) error {
	switch d.ResolutionClass {
	case "close", "supersede", "refine", "reverse":
		_, err := tx.ExecContext(ctx, `
			UPDATE pending_threads
			SET status = ?, resolved_turn = ?, updated_at = ?
			WHERE id = ? AND chat_session_id = ?
		`, "resolved", d.SourceTurn, now, d.TargetID, d.ChatSessionID)
		return err
	default:
		return nil
	}
}

func mapFromAny(raw any) map[string]any {
	value, _ := raw.(map[string]any)
	return value
}

func stringFromAny(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		if raw == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}

func intFromAny(raw any) int {
	return int(int64FromAny(raw))
}

func int64FromAny(raw any) int64 {
	switch value := raw.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		out, _ := value.Int64()
		return out
	case string:
		out, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return out
	default:
		return 0
	}
}

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

func (m *mariadbStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT listed_sessions.chat_session_id,
		       listed_sessions.chat_logs_count,
		       listed_sessions.memories_count,
		       listed_sessions.kg_triples_count,
		       listed_sessions.last_activity
		FROM (
		SELECT sid.chat_session_id,
		       COALESCE(cl.chat_logs_count, 0) AS chat_logs_count,
		       COALESCE(mem.memories_count, 0) AS memories_count,
		       COALESCE(kg.kg_triples_count, 0) AS kg_triples_count,
		       CAST(NULLIF(GREATEST(
			       COALESCE(cl.last_activity, '1000-01-01 00:00:00'),
			       COALESCE(mem.last_activity, '1000-01-01 00:00:00'),
			       COALESCE(kg.last_activity, '1000-01-01 00:00:00')
		       ), '1000-01-01 00:00:00') AS DATETIME) AS last_activity
		FROM (
			SELECT chat_session_id FROM chat_logs
			UNION
			SELECT chat_session_id FROM memories
			UNION
			SELECT chat_session_id FROM kg_triples
		) sid
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS chat_logs_count, MAX(created_at) AS last_activity
			FROM chat_logs
			GROUP BY chat_session_id
		) cl ON cl.chat_session_id = sid.chat_session_id
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS memories_count, MAX(created_at) AS last_activity
			FROM memories
			GROUP BY chat_session_id
		) mem ON mem.chat_session_id = sid.chat_session_id
		LEFT JOIN (
			SELECT chat_session_id, COUNT(*) AS kg_triples_count, MAX(created_at) AS last_activity
			FROM kg_triples
			GROUP BY chat_session_id
		) kg ON kg.chat_session_id = sid.chat_session_id
		) listed_sessions
		ORDER BY listed_sessions.last_activity IS NULL ASC, listed_sessions.last_activity DESC, listed_sessions.chat_session_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionSummary
	for rows.Next() {
		var item SessionSummary
		var lastActivity sql.NullTime
		if err := rows.Scan(&item.ChatSessionID, &item.ChatLogsCount, &item.MemoriesCount, &item.KGTriplesCount, &lastActivity); err != nil {
			return nil, err
		}
		if lastActivity.Valid {
			item.LastActivity = lastActivity.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chapter, err := m.latestChapterSummary(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}
	arc, err := m.latestArcSummary(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}
	saga, err := m.latestSagaDigest(ctx, chatSessionID)
	if err != nil {
		return nil, err
	}

	sources := []string{}
	parts := []string{}
	if saga != nil {
		sources = append(sources, "saga_digests")
		if saga.ResumePackText != "" {
			parts = append(parts, "Saga: "+saga.ResumePackText)
		} else if saga.SagaSummary != "" {
			parts = append(parts, "Saga: "+saga.SagaSummary)
		}
	}
	if arc != nil {
		sources = append(sources, "arc_summaries")
		if arc.ArcResumeText != "" {
			parts = append(parts, "Arc: "+arc.ArcResumeText)
		} else if arc.CoreConflict != "" {
			parts = append(parts, "Arc: "+arc.CoreConflict)
		}
	}
	if chapter != nil {
		sources = append(sources, "chapter_summaries")
		if chapter.ChapterTitle != "" {
			parts = append(parts, "Chapter: "+chapter.ChapterTitle)
		}
		if chapter.ResumeText != "" {
			parts = append(parts, "Resume: "+chapter.ResumeText)
		}
		if chapter.SummaryText != "" {
			parts = append(parts, "Summary: "+chapter.SummaryText)
		}
	}
	if len(sources) == 0 {
		return &ResumePack{
			PackStatus:    "empty",
			Trigger:       trigger,
			SourcesUsed:   []string{},
			LayerCount:    0,
			AssembledText: "",
			AssemblyNote:  "no hierarchy rows found",
		}, nil
	}
	return &ResumePack{
		PackStatus:    "ready",
		Trigger:       trigger,
		SourcesUsed:   sources,
		LayerCount:    len(sources),
		AssembledText: strings.Join(parts, "\n"),
		Saga:          saga,
		Arc:           arc,
		Chapter:       chapter,
		AssemblyNote:  "assembled from latest saga/arc/chapter rows",
	}, nil
}

func (m *mariadbStore) latestChapterSummary(ctx context.Context, chatSessionID string) (*ChapterSummary, error) {
	var item ChapterSummary
	var chapterTitle, summaryText, resumeText sql.NullString
	var chapterIndex sql.NullInt64
	var openLoopsJSON, relationshipChangesJSON, worldChangesJSON, callbackCandidatesJSON sql.NullString
	var embeddingVector, embeddingModel sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
		       open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
		       resume_text, embedding_vector, embedding_model, created_at
		FROM chapter_summaries
		WHERE chat_session_id = ?
		ORDER BY chapter_index DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &chapterIndex, &chapterTitle, &summaryText,
		&openLoopsJSON, &relationshipChangesJSON, &worldChangesJSON, &callbackCandidatesJSON,
		&resumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.ChapterIndex = intFromNull(chapterIndex)
	item.ChapterTitle = stringFromNull(chapterTitle)
	item.SummaryText = stringFromNull(summaryText)
	item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
	item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
	item.WorldChangesJSON = stringFromNull(worldChangesJSON)
	item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
	item.ResumeText = stringFromNull(resumeText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) latestArcSummary(ctx context.Context, chatSessionID string) (*ArcSummary, error) {
	var item ArcSummary
	var arcName, arcStatus, coreConflict, arcResumeText sql.NullString
	var keyTurningPointsJSON, activePromisesJSON, unresolvedDebtsJSON, resolvedPayoffsJSON sql.NullString
	var callbackCandidatesJSON, futurePayoffCandidatesJSON, irreversibleTurnsJSON, callbackDebtsJSON sql.NullString
	var relationshipPivotsJSON, embeddingVector, embeddingModel sql.NullString
	var arcIndex sql.NullInt64
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
		       key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
		       callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
		       relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model, created_at
		FROM arc_summaries
		WHERE chat_session_id = ?
		ORDER BY arc_index DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &arcIndex, &arcName, &arcStatus, &coreConflict,
		&keyTurningPointsJSON, &activePromisesJSON, &unresolvedDebtsJSON, &resolvedPayoffsJSON,
		&callbackCandidatesJSON, &futurePayoffCandidatesJSON, &irreversibleTurnsJSON, &callbackDebtsJSON,
		&relationshipPivotsJSON, &arcResumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.ArcIndex = intFromNull(arcIndex)
	item.ArcName = stringFromNull(arcName)
	item.ArcStatus = stringFromNull(arcStatus)
	item.CoreConflict = stringFromNull(coreConflict)
	item.KeyTurningPointsJSON = stringFromNull(keyTurningPointsJSON)
	item.ActivePromisesJSON = stringFromNull(activePromisesJSON)
	item.UnresolvedDebtsJSON = stringFromNull(unresolvedDebtsJSON)
	item.ResolvedPayoffsJSON = stringFromNull(resolvedPayoffsJSON)
	item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
	item.FuturePayoffCandidatesJSON = stringFromNull(futurePayoffCandidatesJSON)
	item.IrreversibleTurnsJSON = stringFromNull(irreversibleTurnsJSON)
	item.CallbackDebtsJSON = stringFromNull(callbackDebtsJSON)
	item.RelationshipPivotsJSON = stringFromNull(relationshipPivotsJSON)
	item.ArcResumeText = stringFromNull(arcResumeText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) latestSagaDigest(ctx context.Context, chatSessionID string) (*SagaDigest, error) {
	var item SagaDigest
	var eraLabel, sagaSummary, persistentFactsJSON, neverDropCandidatesJSON, resumePackText sql.NullString
	var embeddingVector, embeddingModel sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,
		       persistent_facts_json, never_drop_candidates_json, resume_pack_text,
		       embedding_vector, embedding_model, created_at
		FROM saga_digests
		WHERE chat_session_id = ?
		ORDER BY to_turn DESC, id DESC
		LIMIT 1
	`, chatSessionID).Scan(
		&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &eraLabel, &sagaSummary,
		&persistentFactsJSON, &neverDropCandidatesJSON, &resumePackText,
		&embeddingVector, &embeddingModel, &item.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.EraLabel = stringFromNull(eraLabel)
	item.SagaSummary = stringFromNull(sagaSummary)
	item.PersistentFactsJSON = stringFromNull(persistentFactsJSON)
	item.NeverDropCandidatesJSON = stringFromNull(neverDropCandidatesJSON)
	item.ResumePackText = stringFromNull(resumePackText)
	item.EmbeddingVector = stringFromNull(embeddingVector)
	item.EmbeddingModel = stringFromNull(embeddingModel)
	return &item, nil
}

func (m *mariadbStore) SaveChapterSummary(ctx context.Context, item *ChapterSummary) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO chapter_summaries (
			chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
			open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
			resume_text, embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, item.ChapterIndex, nullableString(item.ChapterTitle), item.SummaryText,
		nullableString(item.OpenLoopsJSON), nullableString(item.RelationshipChangesJSON), nullableString(item.WorldChangesJSON),
		nullableString(item.CallbackCandidatesJSON), nullableString(item.ResumeText), nullableString(item.EmbeddingVector),
		nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) SearchChapterSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ChapterSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,
		       open_loops_json, relationship_changes_json, world_changes_json, callback_candidates_json,
		       resume_text, embedding_vector, embedding_model, created_at
		FROM chapter_summaries
		WHERE chat_session_id = ?
		  AND (? = '' OR chapter_title LIKE ? OR summary_text LIKE ? OR resume_text LIKE ?
		       OR open_loops_json LIKE ? OR relationship_changes_json LIKE ? OR world_changes_json LIKE ?
		       OR callback_candidates_json LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY chapter_index DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, like, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ChapterSummary{}
	for rows.Next() {
		var item ChapterSummary
		var chapterTitle, summaryText, resumeText sql.NullString
		var chapterIndex sql.NullInt64
		var openLoopsJSON, relationshipChangesJSON, worldChangesJSON, callbackCandidatesJSON sql.NullString
		var embeddingVector, embeddingModel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &chapterIndex, &chapterTitle, &summaryText,
			&openLoopsJSON, &relationshipChangesJSON, &worldChangesJSON, &callbackCandidatesJSON,
			&resumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ChapterIndex = intFromNull(chapterIndex)
		item.ChapterTitle = stringFromNull(chapterTitle)
		item.SummaryText = stringFromNull(summaryText)
		item.OpenLoopsJSON = stringFromNull(openLoopsJSON)
		item.RelationshipChangesJSON = stringFromNull(relationshipChangesJSON)
		item.WorldChangesJSON = stringFromNull(worldChangesJSON)
		item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
		item.ResumeText = stringFromNull(resumeText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *mariadbStore) SaveArcSummary(ctx context.Context, chatSessionID string, item *ArcSummary) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO arc_summaries (
			chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
			key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
			callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
			relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, item.ArcIndex, nullableString(item.ArcName),
		firstNonEmptyString(item.ArcStatus, "active"), nullableString(item.CoreConflict),
		nullableString(item.KeyTurningPointsJSON), nullableString(item.ActivePromisesJSON), nullableString(item.UnresolvedDebtsJSON),
		nullableString(item.ResolvedPayoffsJSON), nullableString(item.CallbackCandidatesJSON), nullableString(item.FuturePayoffCandidatesJSON),
		nullableString(item.IrreversibleTurnsJSON), nullableString(item.CallbackDebtsJSON), nullableString(item.RelationshipPivotsJSON),
		nullableString(item.ArcResumeText), nullableString(item.EmbeddingVector), nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) GetLatestArcSummary(ctx context.Context, chatSessionID string) (*ArcSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return m.latestArcSummary(ctx, chatSessionID)
}

func (m *mariadbStore) ListArcSummaries(ctx context.Context, chatSessionID string, status string, limit int) ([]ArcSummary, error) {
	items, err := m.SearchArcSummaries(ctx, chatSessionID, "", 0, 0, limit)
	if err != nil || strings.TrimSpace(status) == "" {
		return items, err
	}
	filtered := make([]ArcSummary, 0, len(items))
	for _, item := range items {
		if item.ArcStatus == status {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (m *mariadbStore) SearchArcSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ArcSummary, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,
		       key_turning_points_json, active_promises_json, unresolved_debts_json, resolved_payoffs_json,
		       callback_candidates_json, future_payoff_candidates_json, irreversible_turns_json, callback_debts_json,
		       relationship_pivots_json, arc_resume_text, embedding_vector, embedding_model, created_at
		FROM arc_summaries
		WHERE chat_session_id = ?
		  AND (? = '' OR arc_name LIKE ? OR core_conflict LIKE ? OR arc_resume_text LIKE ?
		       OR key_turning_points_json LIKE ? OR active_promises_json LIKE ? OR unresolved_debts_json LIKE ?
		       OR callback_candidates_json LIKE ? OR future_payoff_candidates_json LIKE ?
		       OR irreversible_turns_json LIKE ? OR callback_debts_json LIKE ? OR relationship_pivots_json LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY arc_index DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, like, like, like, like, like, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ArcSummary{}
	for rows.Next() {
		var item ArcSummary
		var arcName, arcStatus, coreConflict, arcResumeText sql.NullString
		var keyTurningPointsJSON, activePromisesJSON, unresolvedDebtsJSON, resolvedPayoffsJSON sql.NullString
		var callbackCandidatesJSON, futurePayoffCandidatesJSON, irreversibleTurnsJSON, callbackDebtsJSON sql.NullString
		var relationshipPivotsJSON, embeddingVector, embeddingModel sql.NullString
		var arcIndex sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &arcIndex, &arcName, &arcStatus, &coreConflict,
			&keyTurningPointsJSON, &activePromisesJSON, &unresolvedDebtsJSON, &resolvedPayoffsJSON,
			&callbackCandidatesJSON, &futurePayoffCandidatesJSON, &irreversibleTurnsJSON, &callbackDebtsJSON,
			&relationshipPivotsJSON, &arcResumeText, &embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.ArcIndex = intFromNull(arcIndex)
		item.ArcName = stringFromNull(arcName)
		item.ArcStatus = stringFromNull(arcStatus)
		item.CoreConflict = stringFromNull(coreConflict)
		item.KeyTurningPointsJSON = stringFromNull(keyTurningPointsJSON)
		item.ActivePromisesJSON = stringFromNull(activePromisesJSON)
		item.UnresolvedDebtsJSON = stringFromNull(unresolvedDebtsJSON)
		item.ResolvedPayoffsJSON = stringFromNull(resolvedPayoffsJSON)
		item.CallbackCandidatesJSON = stringFromNull(callbackCandidatesJSON)
		item.FuturePayoffCandidatesJSON = stringFromNull(futurePayoffCandidatesJSON)
		item.IrreversibleTurnsJSON = stringFromNull(irreversibleTurnsJSON)
		item.CallbackDebtsJSON = stringFromNull(callbackDebtsJSON)
		item.RelationshipPivotsJSON = stringFromNull(relationshipPivotsJSON)
		item.ArcResumeText = stringFromNull(arcResumeText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *mariadbStore) SaveSagaDigest(ctx context.Context, chatSessionID string, item *SagaDigest) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO saga_digests (
			chat_session_id, from_turn, to_turn, era_label, saga_summary,
			persistent_facts_json, never_drop_candidates_json, resume_pack_text,
			embedding_vector, embedding_model
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ChatSessionID, item.FromTurn, item.ToTurn, nullableString(item.EraLabel), item.SagaSummary,
		nullableString(item.PersistentFactsJSON), nullableString(item.NeverDropCandidatesJSON), nullableString(item.ResumePackText),
		nullableString(item.EmbeddingVector), nullableString(item.EmbeddingModel))
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return nil
}

func (m *mariadbStore) GetLatestSagaDigest(ctx context.Context, chatSessionID string) (*SagaDigest, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	return m.latestSagaDigest(ctx, chatSessionID)
}

func (m *mariadbStore) ListSagaDigests(ctx context.Context, chatSessionID string, limit int) ([]SagaDigest, error) {
	return m.SearchSagaDigests(ctx, chatSessionID, "", 0, 0, limit)
}

func (m *mariadbStore) SearchSagaDigests(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]SagaDigest, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	q := strings.TrimSpace(query)
	like := "%" + q + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,
		       persistent_facts_json, never_drop_candidates_json, resume_pack_text,
		       embedding_vector, embedding_model, created_at
		FROM saga_digests
		WHERE chat_session_id = ?
		  AND (? = '' OR era_label LIKE ? OR saga_summary LIKE ? OR resume_pack_text LIKE ?)
		  AND (? = 0 OR to_turn >= ?)
		  AND (? = 0 OR from_turn <= ?)
		ORDER BY to_turn DESC, id DESC
		LIMIT ?
	`, chatSessionID, q, like, like, like, fromTurn, fromTurn, toTurn, toTurn, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []SagaDigest{}
	for rows.Next() {
		var item SagaDigest
		var eraLabel, sagaSummary, persistentFactsJSON, neverDropCandidatesJSON, resumePackText sql.NullString
		var embeddingVector, embeddingModel sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.FromTurn, &item.ToTurn, &eraLabel, &sagaSummary,
			&persistentFactsJSON, &neverDropCandidatesJSON, &resumePackText,
			&embeddingVector, &embeddingModel, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.EraLabel = stringFromNull(eraLabel)
		item.SagaSummary = stringFromNull(sagaSummary)
		item.PersistentFactsJSON = stringFromNull(persistentFactsJSON)
		item.NeverDropCandidatesJSON = stringFromNull(neverDropCandidatesJSON)
		item.ResumePackText = stringFromNull(resumePackText)
		item.EmbeddingVector = stringFromNull(embeddingVector)
		item.EmbeddingModel = stringFromNull(embeddingModel)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ---------------------------------------------------------------------------
// Narrative / State read methods (R1)
// ---------------------------------------------------------------------------

func (m *mariadbStore) SaveStoryline(ctx context.Context, s *Storyline) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	updatedAt := nonZeroTime(s.UpdatedAt)
	res, err := m.db.ExecContext(ctx, `
		UPDATE storylines
		SET status = ?, entities_json = ?, current_context = ?,
			key_points_json = ?, ongoing_tensions_json = ?, confidence = ?,
			evidence_count = ?, last_evidence_turn = ?,
			first_turn = CASE WHEN first_turn IS NULL OR first_turn = 0 THEN ? ELSE first_turn END,
			last_turn = ?, pinned = ?, suppressed = ?, user_corrected = ?,
			updated_at = ?
		WHERE chat_session_id = ? AND name = ?
	`, firstNonEmptyString(s.Status, "active"), nullableString(s.EntitiesJSON),
		nullableString(s.CurrentContext), nullableString(s.KeyPointsJSON), nullableString(s.OngoingTensionsJSON),
		s.Confidence, s.EvidenceCount, s.LastEvidenceTurn, s.FirstTurn, s.LastTurn,
		s.Pinned, s.Suppressed, s.UserCorrected, updatedAt, s.ChatSessionID, s.Name)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return nil
	}
	exists, err := m.storylineExists(ctx, s.ChatSessionID, s.Name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO storylines (
			chat_session_id, name, status, entities_json, current_context,
			key_points_json, ongoing_tensions_json, confidence, evidence_count,
			last_evidence_turn, first_turn, last_turn, pinned, suppressed,
			user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ChatSessionID, s.Name, firstNonEmptyString(s.Status, "active"), nullableString(s.EntitiesJSON),
		nullableString(s.CurrentContext), nullableString(s.KeyPointsJSON), nullableString(s.OngoingTensionsJSON),
		s.Confidence, s.EvidenceCount, s.LastEvidenceTurn, s.FirstTurn, s.LastTurn,
		s.Pinned, s.Suppressed, s.UserCorrected, nonZeroTime(s.CreatedAt), nonZeroTime(s.UpdatedAt))
	return err
}

func (m *mariadbStore) PatchStoryline(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	return m.patchStorylineFields(ctx, storylineID, updates, []string{
		"name", "status", "entities_json", "current_context", "key_points_json",
		"ongoing_tensions_json", "confidence", "evidence_count", "last_evidence_turn",
		"first_turn", "last_turn",
	})
}

func (m *mariadbStore) PatchStorylineTrust(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	return m.patchStorylineFields(ctx, storylineID, updates, []string{"pinned", "suppressed", "user_corrected"})
}

func (m *mariadbStore) patchStorylineFields(ctx context.Context, storylineID int64, updates map[string]any, fieldOrder []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	updatedFields := make([]string, 0, len(updates))
	setParts := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for _, field := range fieldOrder {
		val, ok := updates[field]
		if !ok {
			continue
		}
		setParts = append(setParts, field+" = ?")
		args = append(args, val)
		updatedFields = append(updatedFields, field)
	}
	if len(setParts) == 0 {
		exists, err := m.storylineIDExists(ctx, storylineID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return updatedFields, nil
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, storylineID)
	res, err := m.db.ExecContext(ctx, "UPDATE storylines SET "+strings.Join(setParts, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return updatedFields, nil
	}
	exists, err := m.storylineIDExists(ctx, storylineID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return updatedFields, nil
}

func (m *mariadbStore) DeleteStoryline(ctx context.Context, storylineID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM storylines WHERE id = ?", storylineID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) storylineExists(ctx context.Context, chatSessionID, name string) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storylines WHERE chat_session_id = ? AND name = ?", chatSessionID, name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) storylineIDExists(ctx context.Context, storylineID int64) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM storylines WHERE id = ?", storylineID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, name, status, entities_json, current_context, key_points_json,
			   ongoing_tensions_json, confidence, evidence_count, last_evidence_turn, first_turn, last_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM storylines
		WHERE chat_session_id = ?
		ORDER BY last_turn DESC, id DESC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Storyline
	for rows.Next() {
		var item Storyline
		var entitiesJSON, currentContext, keyPointsJSON, ongoingTensionsJSON sql.NullString
		var confidence sql.NullFloat64
		var evidenceCount, lastEvidenceTurn, firstTurn, lastTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Name, &item.Status,
			&entitiesJSON, &currentContext, &keyPointsJSON, &ongoingTensionsJSON,
			&confidence, &evidenceCount, &lastEvidenceTurn, &firstTurn, &lastTurn,
			&item.Pinned, &item.Suppressed, &item.UserCorrected, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.EntitiesJSON = stringFromNull(entitiesJSON)
		item.CurrentContext = stringFromNull(currentContext)
		item.KeyPointsJSON = stringFromNull(keyPointsJSON)
		item.OngoingTensionsJSON = stringFromNull(ongoingTensionsJSON)
		item.Confidence = float64FromNull(confidence)
		item.EvidenceCount = intFromNull(evidenceCount)
		item.LastEvidenceTurn = intFromNull(lastEvidenceTurn)
		item.FirstTurn = intFromNull(firstTurn)
		item.LastTurn = intFromNull(lastTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveWorldRule(ctx context.Context, w *WorldRule) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	scope := firstNonEmptyString(w.Scope, "root")
	scopeName := nullableString(w.ScopeName)
	category := firstNonEmptyString(w.Category, "custom")
	updatedAt := nonZeroTime(w.UpdatedAt)
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE world_rules
		SET scope_name = ?,
			category = ?,
			value_json = COALESCE(?, value_json),
			genre = COALESCE(?, genre),
			source_turn = CASE WHEN ? > 0 THEN ? ELSE source_turn END,
			pinned = ?,
			suppressed = ?,
			user_corrected = ?,
			updated_at = ?
		WHERE chat_session_id = ? AND scope = ? AND `+"`key`"+` = ? AND scope_name <=> ?
		ORDER BY id DESC
		LIMIT 1
	`, scopeName, category, nullableString(w.ValueJSON), nullableString(w.Genre),
		w.SourceTurn, w.SourceTurn, w.Pinned, w.Suppressed, w.UserCorrected, updatedAt,
		w.ChatSessionID, scope, w.Key, scopeName)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return nil
	}
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO world_rules (
			chat_session_id, scope, scope_name, category, `+"`key`"+`, value_json,
			genre, source_turn, pinned, suppressed, user_corrected, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, w.ChatSessionID, scope, scopeName,
		category, w.Key, nullableString(w.ValueJSON),
		nullableString(w.Genre), w.SourceTurn, w.Pinned, w.Suppressed, w.UserCorrected,
		nonZeroTime(w.CreatedAt), updatedAt)
	return err
}

func (m *mariadbStore) PatchWorldRule(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	return m.patchWorldRuleFields(ctx, ruleID, updates, []string{"scope", "scope_name", "category", "key", "value_json", "genre", "source_turn"})
}

func (m *mariadbStore) PatchWorldRuleTrust(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	return m.patchWorldRuleFields(ctx, ruleID, updates, []string{"pinned", "suppressed", "user_corrected"})
}

func (m *mariadbStore) patchWorldRuleFields(ctx context.Context, ruleID int64, updates map[string]any, fieldOrder []string) ([]string, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	updatedFields := make([]string, 0, len(updates))
	setParts := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for _, field := range fieldOrder {
		val, ok := updates[field]
		if !ok {
			continue
		}
		column := field
		if field == "key" {
			column = "`key`"
		}
		setParts = append(setParts, column+" = ?")
		args = append(args, val)
		updatedFields = append(updatedFields, field)
	}
	if len(setParts) == 0 {
		exists, err := m.worldRuleIDExists(ctx, ruleID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNotFound
		}
		return updatedFields, nil
	}
	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, ruleID)
	res, err := m.db.ExecContext(ctx, "UPDATE world_rules SET "+strings.Join(setParts, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows > 0 {
		return updatedFields, nil
	}
	exists, err := m.worldRuleIDExists(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	return updatedFields, nil
}

func (m *mariadbStore) DeleteWorldRule(ctx context.Context, ruleID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM world_rules WHERE id = ?", ruleID)
	if err != nil {
		return err
	}
	if rows, rowErr := res.RowsAffected(); rowErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) worldRuleIDExists(ctx context.Context, ruleID int64) (bool, error) {
	var count int
	if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM world_rules WHERE id = ?", ruleID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *mariadbStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, scope, scope_name, category, `+"`key`"+`, value_json, genre, source_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM world_rules
		WHERE chat_session_id = ?
		ORDER BY scope, category, `+"`key`"+`
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorldRule
	for rows.Next() {
		var item WorldRule
		var scopeName, genre sql.NullString
		var valueJSON sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Scope, &scopeName, &item.Category, &item.Key,
			&valueJSON, &genre, &sourceTurn, &item.Pinned, &item.Suppressed, &item.UserCorrected,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScopeName = stringFromNull(scopeName)
		item.ValueJSON = stringFromNull(valueJSON)
		item.Genre = stringFromNull(genre)
		item.SourceTurn = intFromNull(sourceTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	activeScope = strings.TrimSpace(activeScope)
	scopeName = strings.TrimSpace(scopeName)
	if activeScope == "" {
		if saved, err := m.GetActiveScope(ctx, chatSessionID); err == nil && saved != nil {
			activeScope = strings.TrimSpace(saved.ActiveScope)
			if scopeName == "" {
				scopeName = strings.TrimSpace(saved.ScopeName)
			}
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	chain := WorldRuleScopeChain(activeScope)
	chainOrder := make(map[string]int, len(chain))
	for i, scope := range chain {
		chainOrder[scope] = i
	}

	query := `
		SELECT id, chat_session_id, scope, scope_name, category, ` + "`key`" + `, value_json, genre, source_turn,
			   pinned, suppressed, user_corrected, created_at, updated_at
		FROM world_rules
		WHERE chat_session_id = ? AND suppressed = FALSE
		ORDER BY scope, category, ` + "`key`" + `
	`
	rows, err := m.db.QueryContext(ctx, query, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorldRule
	for rows.Next() {
		var item WorldRule
		var scopeNameNull, genre sql.NullString
		var valueJSON sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ChatSessionID, &item.Scope, &scopeNameNull, &item.Category, &item.Key,
			&valueJSON, &genre, &sourceTurn, &item.Pinned, &item.Suppressed, &item.UserCorrected,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ScopeName = stringFromNull(scopeNameNull)
		item.ValueJSON = stringFromNull(valueJSON)
		item.Genre = stringFromNull(genre)
		item.SourceTurn = intFromNull(sourceTurn)
		itemScope := NormalizeWorldRuleScope(item.Scope)
		if _, ok := chainOrder[itemScope]; !ok {
			continue
		}
		if itemScope == NormalizeWorldRuleScope(activeScope) {
			if scopeName != "" && strings.TrimSpace(item.ScopeName) != scopeName {
				continue
			}
			if scopeName == "" && strings.TrimSpace(item.ScopeName) != "" {
				continue
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := chainOrder[NormalizeWorldRuleScope(out[i].Scope)], chainOrder[NormalizeWorldRuleScope(out[j].Scope)]
		if left != right {
			return left < right
		}
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

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

var _ EffectiveInputListStore = (*mariadbStore)(nil)
var _ SessionMigrationStore = (*mariadbStore)(nil)
var _ SessionMigrationVectorStore = (*mariadbStore)(nil)
var _ SessionMigrationSourceLockStore = (*mariadbStore)(nil)
var _ SessionMigrationRecoveryStore = (*mariadbStore)(nil)

func (m *mariadbStore) CompleteSessionMigration(ctx context.Context, req SessionMigrationCompleteRequest) (*SessionMigrationCompleteResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	sourceID := strings.TrimSpace(req.SourceSessionID)
	targetID := strings.TrimSpace(req.TargetSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "copy_then_lock_source"
	}
	if sourceID == "" || targetID == "" {
		return nil, errors.New("source_session_id and target_session_id are required")
	}
	if sourceID == targetID {
		return nil, errors.New("source and target sessions must differ")
	}
	if mode != "copy_then_lock_source" {
		return nil, errors.New("unsupported session migration mode")
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	sourceCounts, err := sessionMigrationCountArtifactsTx(ctx, tx, sourceID)
	if err != nil {
		return nil, err
	}
	if sourceCounts.CanonicalAndSubjectiveTotal == 0 {
		return nil, errors.New("source session has no archive data")
	}
	targetCounts, err := sessionMigrationCountArtifactsTx(ctx, tx, targetID)
	if err != nil {
		return nil, err
	}
	if targetCounts.CanonicalAndSubjectiveTotal > 0 {
		return nil, errors.New("target session is not empty")
	}

	initialCountsJSON, _ := json.Marshal(sourceCounts)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO session_migrations (
			source_session_id, target_session_id, mode, status, operator_note,
			counts_json, chroma_reindexed_count, errors_json
		) VALUES (?, ?, ?, 'copying', ?, ?, 0, JSON_ARRAY())
	`, sourceID, targetID, mode, nullableString(strings.TrimSpace(req.OperatorNote)), string(initialCountsJSON))
	if err != nil {
		return nil, err
	}
	migrationID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	counts := SessionMigrationArtifactCounts{}
	rowMapCount := 0
	count, maps, err := copySessionMigrationChatLogs(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.ChatLogs = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEffectiveInputs(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.EffectiveInputs = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationMemories(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.Memories = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEvidence(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.DirectEvidence = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationKGTriples(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.KGTriples = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEpisodes(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.Episodes = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationSubjectiveEntityMemories(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, err
	}
	counts.SubjectiveEntityMemories = count
	rowMapCount += maps
	sessionMigrationFinalizeCounts(&counts)

	countsJSON, _ := json.Marshal(counts)
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'copied',
		    counts_json = ?,
		    errors_json = JSON_ARRAY('chroma_reindex_pending'),
		    completed_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, string(countsJSON), migrationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &SessionMigrationCompleteResult{
		MigrationID:           migrationID,
		Status:                "copied",
		SourceSessionID:       sourceID,
		TargetSessionID:       targetID,
		Mode:                  mode,
		Counts:                counts,
		RowMapCount:           rowMapCount,
		ChromaReindexedCount:  0,
		SourceLocked:          false,
		ChromaReindexRequired: true,
		ReadyForLive:          false,
	}, nil
}

func (m *mariadbStore) ListSessionMigrationVectorDocuments(ctx context.Context, migrationID int64) ([]SessionMigrationVectorDocument, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	docs := []SessionMigrationVectorDocument{}
	memoryRows, err := m.db.QueryContext(ctx, `
		SELECT sm.id, sm.source_session_id, sm.target_session_id, m.id, m.embedding,
		       m.summary_json, m.evidence
		FROM session_migration_row_map rm
		JOIN session_migrations sm ON sm.id = rm.migration_id
		JOIN memories m ON m.id = rm.target_row_id
		WHERE rm.migration_id = ? AND rm.table_name = 'memories'
		ORDER BY m.turn_index ASC, m.id ASC
	`, migrationID)
	if err != nil {
		return nil, err
	}
	for memoryRows.Next() {
		var doc SessionMigrationVectorDocument
		var targetRowID int64
		var summaryJSON, evidence sql.NullString
		if err := memoryRows.Scan(&doc.MigrationID, &doc.MigratedFromSessionID, &doc.ChatSessionID, &targetRowID,
			&doc.EmbeddingJSON, &summaryJSON, &evidence); err != nil {
			memoryRows.Close()
			return nil, err
		}
		doc.Tier = "memory"
		doc.SourceTable = "memories"
		doc.SourceRowID = strconv.FormatInt(targetRowID, 10)
		doc.ID = "memory:" + doc.ChatSessionID + ":" + doc.SourceRowID
		doc.SchemaVersion = "memory.v1"
		doc.DocumentText = sessionMigrationMemoryDocumentText(stringFromNull(summaryJSON), stringFromNull(evidence))
		docs = append(docs, doc)
	}
	if err := memoryRows.Close(); err != nil {
		return nil, err
	}
	if err := memoryRows.Err(); err != nil {
		return nil, err
	}

	episodeRows, err := m.db.QueryContext(ctx, `
		SELECT sm.id, sm.source_session_id, sm.target_session_id, e.id, e.embedding_vector,
		       e.summary_text
		FROM session_migration_row_map rm
		JOIN session_migrations sm ON sm.id = rm.migration_id
		JOIN episode_summaries e ON e.id = rm.target_row_id
		WHERE rm.migration_id = ? AND rm.table_name = 'episode_summaries'
		ORDER BY e.from_turn ASC, e.to_turn ASC, e.id ASC
	`, migrationID)
	if err != nil {
		return nil, err
	}
	defer episodeRows.Close()
	for episodeRows.Next() {
		var doc SessionMigrationVectorDocument
		var targetRowID int64
		if err := episodeRows.Scan(&doc.MigrationID, &doc.MigratedFromSessionID, &doc.ChatSessionID, &targetRowID,
			&doc.EmbeddingJSON, &doc.DocumentText); err != nil {
			return nil, err
		}
		doc.Tier = "episode"
		doc.SourceTable = "episode_summaries"
		doc.SourceRowID = strconv.FormatInt(targetRowID, 10)
		doc.ID = "episode:" + doc.ChatSessionID + ":" + doc.SourceRowID
		doc.SchemaVersion = "episode.v1"
		docs = append(docs, doc)
	}
	return docs, episodeRows.Err()
}

func (m *mariadbStore) UpdateSessionMigrationVectorStatus(ctx context.Context, migrationID int64, status string, reindexedCount int, errorsJSON string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if migrationID <= 0 {
		return ErrNotFound
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "vector_reindexed"
	}
	if strings.TrimSpace(errorsJSON) == "" {
		errorsJSON = "[]"
	}
	_, err := m.db.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = ?,
		    chroma_reindexed_count = ?,
		    errors_json = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, status, reindexedCount, errorsJSON, migrationID)
	return err
}

func (m *mariadbStore) LockSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationSourceLockResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var sourceID, targetID, status string
	var reindexedCount int
	err = tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status, chroma_reindexed_count
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &status, &reindexedCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "vector_reindexed" && status != "source_locked" {
		return nil, fmt.Errorf("session migration source lock blocked: migration status %q is not vector_reindexed", status)
	}
	if reindexedCount <= 0 && status != "source_locked" {
		return nil, errors.New("session migration source lock blocked: chroma_reindexed_count is zero")
	}

	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if lock != nil && lock.MigrationID != migrationID {
		return nil, fmt.Errorf("session migration source lock blocked: source session is already locked by migration %d", lock.MigrationID)
	}
	if lock == nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_migration_locks (
				migration_id, source_session_id, target_session_id, locked, lock_status, reason
			) VALUES (?, ?, ?, TRUE, 'migrated_away', ?)
		`, migrationID, sourceID, targetID, strings.TrimSpace(reason))
		if err != nil {
			return nil, err
		}
		lock, err = sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'source_locked',
		    locked_at = COALESCE(locked_at, CURRENT_TIMESTAMP(3)),
		    errors_json = JSON_ARRAY(),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, migrationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationSourceLockResult{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          "source_locked",
		Lock:            *lock,
		ReadyForLive:    true,
	}, nil
}

func (m *mariadbStore) GetSessionMigrationSourceLock(ctx context.Context, sourceSessionID string) (*SessionMigrationLock, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	if sourceSessionID == "" {
		return nil, ErrNotFound
	}
	return sessionMigrationSelectActiveLock(ctx, m.db, sourceSessionID)
}

func (m *mariadbStore) RollbackSessionMigration(ctx context.Context, migrationID int64, reason string) (*SessionMigrationRollbackResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var sourceID, targetID, status string
	err = tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == "rolled_back" {
		return &SessionMigrationRollbackResult{
			MigrationID:     migrationID,
			SourceSessionID: sourceID,
			TargetSessionID: targetID,
			Status:          "rolled_back",
			ReadyForLive:    false,
		}, nil
	}
	if status == "source_cleaned" || status == "cleanup_completed" {
		return nil, fmt.Errorf("session migration rollback blocked: migration status %q has already cleaned the source", status)
	}

	counts := SessionMigrationArtifactCounts{}
	rowMapCount := 0
	deletionPlan := []struct {
		table string
		dst   *int
	}{
		{"protagonist_entity_memories", &counts.SubjectiveEntityMemories},
		{"episode_summaries", &counts.Episodes},
		{"kg_triples", &counts.KGTriples},
		{"memories", &counts.Memories},
		{"direct_evidence_records", &counts.DirectEvidence},
		{"effective_input_logs", &counts.EffectiveInputs},
		{"chat_logs", &counts.ChatLogs},
	}
	for _, item := range deletionPlan {
		deleted, err := deleteSessionMigrationMappedRows(ctx, tx, migrationID, item.table)
		if err != nil {
			return nil, err
		}
		*item.dst = deleted
		rowMapCount += deleted
	}
	sessionMigrationFinalizeCounts(&counts)

	sourceUnlocked := false
	if status == "source_locked" {
		res, err := tx.ExecContext(ctx, `
			UPDATE session_migration_locks
			SET locked = FALSE,
			    lock_status = 'rolled_back',
			    reason = CONCAT(COALESCE(reason, ''), CASE WHEN COALESCE(reason, '') = '' THEN '' ELSE '\n' END, ?),
			    unlocked_at = CURRENT_TIMESTAMP(3),
			    updated_at = CURRENT_TIMESTAMP(3)
			WHERE migration_id = ? AND locked = TRUE AND unlocked_at IS NULL
		`, strings.TrimSpace(reason), migrationID)
		if err != nil {
			return nil, err
		}
		affected, _ := res.RowsAffected()
		sourceUnlocked = affected > 0
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE session_migration_row_map
		SET row_status = 'rolled_back'
		WHERE migration_id = ? AND row_status <> 'rolled_back'
	`, migrationID)
	if err != nil {
		return nil, err
	}
	errorsJSON, _ := json.Marshal([]string{"rolled_back", strings.TrimSpace(reason)})
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'rolled_back',
		    errors_json = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, string(errorsJSON), migrationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationRollbackResult{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          "rolled_back",
		Counts:          counts,
		RowMapCount:     rowMapCount,
		SourceUnlocked:  sourceUnlocked,
		ReadyForLive:    false,
	}, nil
}

func (m *mariadbStore) PreviewSessionMigrationSourceCleanup(ctx context.Context, migrationID int64) (*SessionMigrationCleanupPreview, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	return preview, nil
}

func (m *mariadbStore) CleanupSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationCleanupResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	if !preview.ReadyForCleanup {
		return nil, fmt.Errorf("session migration source cleanup blocked: %s", strings.Join(preview.BlockedReasons, ","))
	}

	if err := deleteSessionRowsTx(ctx, tx, preview.SourceSessionID); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'source_cleaned',
		    cleanup_at = CURRENT_TIMESTAMP(3),
		    errors_json = JSON_ARRAY(),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, migrationID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migration_locks
		SET lock_status = 'source_cleaned',
		    reason = CONCAT(COALESCE(reason, ''), CASE WHEN COALESCE(reason, '') = '' THEN '' ELSE '\n' END, ?),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ? AND locked = TRUE AND unlocked_at IS NULL
	`, strings.TrimSpace(reason), migrationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationCleanupResult{
		MigrationID:     migrationID,
		SourceSessionID: preview.SourceSessionID,
		TargetSessionID: preview.TargetSessionID,
		Status:          "source_cleaned",
		Counts:          preview.Counts,
		SourceCleaned:   true,
		ReadyForLive:    true,
	}, nil
}

type sessionMigrationLockQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func sessionMigrationSelectActiveLock(ctx context.Context, q sessionMigrationLockQuerier, sourceSessionID string) (*SessionMigrationLock, error) {
	return sessionMigrationScanActiveLock(q.QueryRowContext(ctx, `
		SELECT migration_id, source_session_id, target_session_id, locked, lock_status,
		       COALESCE(reason, ''), locked_at
		FROM session_migration_locks
		WHERE source_session_id = ? AND locked = TRUE AND unlocked_at IS NULL
		ORDER BY locked_at DESC, id DESC
		LIMIT 1
	`, sourceSessionID))
}

func sessionMigrationSelectActiveLockTx(ctx context.Context, tx *sql.Tx, sourceSessionID string) (*SessionMigrationLock, error) {
	return sessionMigrationScanActiveLock(tx.QueryRowContext(ctx, `
		SELECT migration_id, source_session_id, target_session_id, locked, lock_status,
		       COALESCE(reason, ''), locked_at
		FROM session_migration_locks
		WHERE source_session_id = ? AND locked = TRUE AND unlocked_at IS NULL
		ORDER BY locked_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, sourceSessionID))
}

func sessionMigrationScanActiveLock(row *sql.Row) (*SessionMigrationLock, error) {
	var lock SessionMigrationLock
	if err := row.Scan(&lock.MigrationID, &lock.SourceSessionID, &lock.TargetSessionID, &lock.Locked, &lock.LockStatus, &lock.Reason, &lock.LockedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &lock, nil
}

func sessionMigrationMemoryDocumentText(summaryJSON, evidence string) string {
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(summaryJSON), &parsed); err == nil {
		for _, key := range []string{"summary", "turn_summary", "text", "content"} {
			if value := strings.TrimSpace(fmt.Sprint(parsed[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	if text := strings.TrimSpace(evidence); text != "" {
		return text
	}
	return strings.TrimSpace(summaryJSON)
}

func sessionMigrationFinalizeCounts(counts *SessionMigrationArtifactCounts) {
	counts.CanonicalTotal = counts.ChatLogs + counts.EffectiveInputs + counts.Memories + counts.DirectEvidence + counts.KGTriples + counts.Episodes
	counts.CanonicalAndSubjectiveTotal = counts.CanonicalTotal + counts.SubjectiveEntityMemories
}

func sessionMigrationCountArtifactsTx(ctx context.Context, tx *sql.Tx, sessionID string) (SessionMigrationArtifactCounts, error) {
	counts := SessionMigrationArtifactCounts{}
	tableCounts := []struct {
		name string
		dst  *int
	}{
		{"chat_logs", &counts.ChatLogs},
		{"effective_input_logs", &counts.EffectiveInputs},
		{"memories", &counts.Memories},
		{"direct_evidence_records", &counts.DirectEvidence},
		{"kg_triples", &counts.KGTriples},
		{"episode_summaries", &counts.Episodes},
		{"protagonist_entity_memories", &counts.SubjectiveEntityMemories},
	}
	for _, item := range tableCounts {
		query := "SELECT COUNT(*) FROM " + item.name + " WHERE chat_session_id = ?"
		if item.name == "protagonist_entity_memories" {
			query = "SELECT COUNT(*) FROM protagonist_entity_memories WHERE source_chat_session_id = ?"
		}
		if err := tx.QueryRowContext(ctx, query, sessionID).Scan(item.dst); err != nil {
			return counts, err
		}
	}
	sessionMigrationFinalizeCounts(&counts)
	return counts, nil
}

func previewSessionMigrationSourceCleanupTx(ctx context.Context, tx *sql.Tx, migrationID int64) (*SessionMigrationCleanupPreview, error) {
	var sourceID, targetID, status string
	err := tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	counts, err := sessionMigrationCountArtifactsTx(ctx, tx, sourceID)
	if err != nil {
		return nil, err
	}
	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	sourceLocked := lock != nil && lock.MigrationID == migrationID && lock.Locked
	blocked := []string{}
	if status != "source_locked" {
		blocked = append(blocked, "migration_status_not_source_locked")
	}
	if !sourceLocked {
		blocked = append(blocked, "active_source_lock_not_found_for_migration")
	}
	return &SessionMigrationCleanupPreview{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          status,
		SourceLocked:    sourceLocked,
		Counts:          counts,
		BlockedReasons:  blocked,
		ReadyForCleanup: len(blocked) == 0,
	}, nil
}

func deleteSessionMigrationMappedRows(ctx context.Context, tx *sql.Tx, migrationID int64, tableName string) (int, error) {
	if !sessionMigrationAllowedMappedTable(tableName) {
		return 0, fmt.Errorf("session migration rollback blocked: unsupported mapped table %q", tableName)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT target_row_id
		FROM session_migration_row_map
		WHERE migration_id = ? AND table_name = ? AND target_row_id IS NOT NULL
		ORDER BY target_row_id ASC
	`, migrationID, tableName)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if tableName == "direct_evidence_records" {
		if _, err := execIDBatch(ctx, tx, "UPDATE direct_evidence_records SET superseded_by_id = NULL WHERE superseded_by_id IN", ids); err != nil {
			return 0, err
		}
	}
	return execIDBatch(ctx, tx, "DELETE FROM "+tableName+" WHERE id IN", ids)
}

func sessionMigrationAllowedMappedTable(tableName string) bool {
	switch tableName {
	case "chat_logs", "effective_input_logs", "memories", "direct_evidence_records", "kg_triples", "episode_summaries", "protagonist_entity_memories":
		return true
	default:
		return false
	}
}

func execIDBatch(ctx context.Context, tx *sql.Tx, prefix string, ids []int64) (int, error) {
	total := 0
	const batchSize = 500
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = "?"
			args[i] = id
		}
		res, err := tx.ExecContext(ctx, prefix+" ("+strings.Join(placeholders, ",")+")", args...)
		if err != nil {
			return total, err
		}
		affected, _ := res.RowsAffected()
		total += int(affected)
	}
	return total, nil
}

func deleteSessionRowsTx(ctx context.Context, tx *sql.Tx, chatSessionID string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM persona_capsule_attachments WHERE target_chat_session_id = ?", chatSessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM protagonist_entity_memories WHERE source_chat_session_id = ?", chatSessionID); err != nil {
		return err
	}
	tables := []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"character_events",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
		"canonical_state_layers",
		"episode_summaries",
		"chapter_summaries",
		"arc_summaries",
		"saga_digests",
		"session_active_scopes",
		"guidance_plan_states",
		"entities",
		"trust_states",
		"consequence_records",
		"psychology_branches",
		"session_fork_lineage",
		"theme_offscreen_carries",
		"critic_feedback",
	}
	for _, tbl := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE chat_session_id = ?", chatSessionID); err != nil {
			return err
		}
	}
	return nil
}

func copySessionMigrationChatLogs(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, role, content, created_at
		FROM chat_logs
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var turnIndex int
		var role, content string
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &turnIndex, &role, &content, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, targetID, turnIndex, role, content, nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "chat_logs", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func copySessionMigrationEffectiveInputs(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, effective_input, created_at
		FROM effective_input_logs
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var turnIndex int
		var effectiveInput string
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &turnIndex, &effectiveInput, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO effective_input_logs (chat_session_id, turn_index, effective_input, created_at)
			VALUES (?, ?, ?, ?)
		`, targetID, turnIndex, effectiveInput, nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "effective_input_logs", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func copySessionMigrationMemories(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, summary_json, embedding, embedding_model, importance,
		       emotional_boost, evidence, emotional_intensity, narrative_significance,
		       place_wing, place_room, created_at
		FROM memories
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var turnIndex int
		var summaryJSON, embedding, embeddingModel, evidence, placeWing, placeRoom sql.NullString
		var importance, emotionalBoost, emotionalIntensity, narrativeSignificance sql.NullFloat64
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &turnIndex, &summaryJSON, &embedding, &embeddingModel,
			&importance, &emotionalBoost, &evidence, &emotionalIntensity, &narrativeSignificance,
			&placeWing, &placeRoom, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO memories (
				chat_session_id, turn_index, summary_json, embedding, embedding_model,
				importance, emotional_boost, evidence, emotional_intensity,
				narrative_significance, place_wing, place_room, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, turnIndex, nullStringArg(summaryJSON), nullStringArg(embedding), nullStringArg(embeddingModel),
			nullFloatArg(importance), nullFloatArg(emotionalBoost), nullStringArg(evidence),
			nullFloatArg(emotionalIntensity), nullFloatArg(narrativeSignificance),
			nullStringArg(placeWing), nullStringArg(placeRoom), nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "memories", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func copySessionMigrationEvidence(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
		       turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
		       capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
		       superseded_by_id, created_at
		FROM direct_evidence_records
		WHERE chat_session_id = ?
		ORDER BY source_turn_start ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	targetBySource := map[int64]int64{}
	supersededByOldSource := map[int64]int64{}
	for rows.Next() {
		var sourceRowID int64
		var evidenceKind, evidenceText, archiveState, captureStage, captureVerification string
		var sourceTurnStart, sourceTurnEnd int
		var turnAnchor, supersededByID sql.NullInt64
		var sourceMessageIDsJSON, sourceHash, committedGate, lineageJSON sql.NullString
		var repairNeeded, tombstoned bool
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &evidenceKind, &evidenceText, &sourceTurnStart, &sourceTurnEnd,
			&turnAnchor, &sourceMessageIDsJSON, &sourceHash, &archiveState, &captureStage,
			&captureVerification, &committedGate, &lineageJSON, &repairNeeded, &tombstoned,
			&supersededByID, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO direct_evidence_records (
				chat_session_id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
				turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
				capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
				superseded_by_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, evidenceKind, evidenceText, sourceTurnStart, sourceTurnEnd,
			nullIntArg(turnAnchor), nullStringArg(sourceMessageIDsJSON), nullStringArg(sourceHash),
			archiveState, captureStage, captureVerification, nullStringArg(committedGate),
			nullStringArg(lineageJSON), repairNeeded, tombstoned, nullIntArg(supersededByID),
			nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		targetBySource[sourceRowID] = targetRowID
		if supersededByID.Valid {
			supersededByOldSource[targetRowID] = supersededByID.Int64
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "direct_evidence_records", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	for targetRowID, oldSupersededID := range supersededByOldSource {
		if newSupersededID, ok := targetBySource[oldSupersededID]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE direct_evidence_records
				SET superseded_by_id = ?
				WHERE id = ?
			`, newSupersededID, targetRowID); err != nil {
				return 0, 0, err
			}
		}
	}
	return count, mapped, nil
}

func copySessionMigrationKGTriples(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, subject, predicate, object, valid_from, valid_to, source_turn, created_at
		FROM kg_triples
		WHERE chat_session_id = ?
		ORDER BY id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var subject, predicate, object string
		var validFrom, validTo, sourceTurn sql.NullInt64
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &subject, &predicate, &object, &validFrom, &validTo, &sourceTurn, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO kg_triples (chat_session_id, subject, predicate, object, valid_from, valid_to, source_turn, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, subject, predicate, object, nullIntArg(validFrom), nullIntArg(validTo), nullIntArg(sourceTurn), nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "kg_triples", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func copySessionMigrationEpisodes(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, from_turn, to_turn, summary_text, key_entities, key_events,
		       open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
		FROM episode_summaries
		WHERE chat_session_id = ?
		ORDER BY from_turn ASC, to_turn ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var fromTurn, toTurn int
		var summaryText string
		var keyEntities, keyEvents, openLoopsJSON, relationshipChangesJSON, embeddingVector, embeddingModel sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&sourceRowID, &fromTurn, &toTurn, &summaryText, &keyEntities, &keyEvents,
			&openLoopsJSON, &relationshipChangesJSON, &embeddingVector, &embeddingModel, &createdAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO episode_summaries (
				chat_session_id, from_turn, to_turn, summary_text, key_entities, key_events,
				open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, fromTurn, toTurn, summaryText, nullStringArg(keyEntities), nullStringArg(keyEvents),
			nullStringArg(openLoopsJSON), nullStringArg(relationshipChangesJSON), nullStringArg(embeddingVector),
			nullStringArg(embeddingModel), nonZeroTime(createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "episode_summaries", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func copySessionMigrationSubjectiveEntityMemories(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
		       owner_entity_role, owner_visibility, source_character_name, source_turn_index,
		       memory_text, evidence_excerpt, secret_guard, portability, tags_json,
		       target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
		FROM protagonist_entity_memories
		WHERE source_chat_session_id = ?
		ORDER BY source_turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	count, mapped := 0, 0
	for rows.Next() {
		var sourceRowID int64
		var personaEntityKey, personaEntityName, ownerEntityKey, ownerEntityName string
		var ownerEntityRole, ownerVisibility, memoryText, portability, targetRevealPolicy string
		var sourceCharacterName, evidenceExcerpt, tagsJSON sql.NullString
		var sourceTurn sql.NullInt64
		var secretGuard bool
		var importance10, emotionalWeight sql.NullFloat64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&sourceRowID, &personaEntityKey, &personaEntityName, &ownerEntityKey, &ownerEntityName,
			&ownerEntityRole, &ownerVisibility, &sourceCharacterName, &sourceTurn,
			&memoryText, &evidenceExcerpt, &secretGuard, &portability, &tagsJSON,
			&targetRevealPolicy, &importance10, &emotionalWeight, &createdAt, &updatedAt); err != nil {
			return 0, 0, err
		}
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO protagonist_entity_memories (
				persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
				owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
				source_turn_index, memory_text, evidence_excerpt, secret_guard, portability,
				tags_json, target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, personaEntityKey, personaEntityName, ownerEntityKey, ownerEntityName,
			ownerEntityRole, ownerVisibility, targetID, nullStringArg(sourceCharacterName),
			nullIntArg(sourceTurn), memoryText, nullStringArg(evidenceExcerpt), secretGuard, portability,
			nullStringArg(tagsJSON), targetRevealPolicy, nullFloatArg(importance10), nullFloatArg(emotionalWeight),
			nonZeroTime(createdAt), nonZeroTime(updatedAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "protagonist_entity_memories", sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, rows.Err()
}

func insertSessionMigrationCopiedRow(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertSessionMigrationRowMap(ctx context.Context, tx *sql.Tx, migrationID int64, tableName string, sourceRowID, targetRowID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO session_migration_row_map (
			migration_id, table_name, source_row_id, target_row_id, row_status
		) VALUES (?, ?, ?, ?, 'copied')
	`, migrationID, tableName, sourceRowID, targetRowID)
	return err
}

func nullStringArg(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

func nullIntArg(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func nullFloatArg(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

// ---------------------------------------------------------------------------
// RollbackStore implementation
// ---------------------------------------------------------------------------

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

func (m *mariadbStore) CreatePersonaMemoryCapsule(ctx context.Context, capsule *PersonaMemoryCapsule, entries []PersonaMemoryEntry) (*PersonaMemoryCapsule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	mode := strings.TrimSpace(capsule.Mode)
	if mode == "" {
		mode = "manual"
	}
	title := strings.TrimSpace(capsule.Title)
	if title == "" {
		title = "Persona Memory Capsule"
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO persona_memory_capsules
			(persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, capsule.PersonaKey, capsule.SourceChatSessionID, capsule.SourceCharacterName, title, mode, capsule.Summary, now, now)
	if err != nil {
		return nil, err
	}
	capsuleID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		portability := strings.TrimSpace(entry.Portability)
		if portability == "" {
			portability = "same_chat"
		}
		injectionPolicy := strings.TrimSpace(entry.InjectionPolicy)
		if injectionPolicy == "" {
			injectionPolicy = "support_only"
		}
		tagsJSON := strings.TrimSpace(entry.TagsJSON)
		if tagsJSON == "" {
			tagsJSON = "[]"
		}
		var sourceMemoryID any
		if entry.SourceMemoryID > 0 {
			sourceMemoryID = entry.SourceMemoryID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO persona_memory_entries
				(capsule_id, source_memory_type, source_memory_id, source_turn_index, memory_text, emotional_weight, importance_10, portability, tags_json, evidence_excerpt, injection_policy, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, capsuleID, nullableString(entry.SourceMemoryType), sourceMemoryID, entry.SourceTurn, entry.MemoryText, entry.EmotionalWeight, entry.Importance10, portability, tagsJSON, entry.EvidenceExcerpt, injectionPolicy, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	out := *capsule
	out.ID = capsuleID
	out.Title = title
	out.Mode = mode
	out.CreatedAt = now
	out.UpdatedAt = now
	return &out, nil
}

func (m *mariadbStore) ListPersonaMemoryCapsules(ctx context.Context, filter PersonaCapsuleFilter) ([]PersonaMemoryCapsule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at
		FROM persona_memory_capsules
		WHERE 1 = 1`
	args := []any{}
	if strings.TrimSpace(filter.PersonaKey) != "" {
		query += " AND persona_key = ?"
		args = append(args, strings.TrimSpace(filter.PersonaKey))
	}
	if strings.TrimSpace(filter.SourceChatSessionID) != "" {
		query += " AND source_chat_session_id = ?"
		args = append(args, strings.TrimSpace(filter.SourceChatSessionID))
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT 200"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryCapsule{}
	for rows.Next() {
		item, err := scanPersonaMemoryCapsule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetPersonaMemoryCapsule(ctx context.Context, capsuleID int64) (*PersonaMemoryCapsule, []PersonaMemoryEntry, error) {
	if err := m.ensureDB(); err != nil {
		return nil, nil, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT id, persona_key, source_chat_session_id, source_character_name, title, mode, summary, created_at, updated_at
		FROM persona_memory_capsules
		WHERE id = ?
	`, capsuleID)
	capsule, err := scanPersonaMemoryCapsule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	entries, err := m.listPersonaMemoryEntriesByCapsule(ctx, capsuleID)
	if err != nil {
		return nil, nil, err
	}
	return &capsule, entries, nil
}

func (m *mariadbStore) DeletePersonaMemoryCapsule(ctx context.Context, capsuleID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, "DELETE FROM persona_memory_capsules WHERE id = ?", capsuleID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) AttachPersonaMemoryCapsule(ctx context.Context, attachment *PersonaCapsuleAttachment) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	mode := strings.TrimSpace(attachment.InjectionMode)
	if mode == "" {
		mode = "subtle_deja_vu"
	}
	now := time.Now().UTC()
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO persona_capsule_attachments
			(capsule_id, target_chat_session_id, injection_mode, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			injection_mode = VALUES(injection_mode),
			enabled = VALUES(enabled),
			updated_at = VALUES(updated_at)
	`, attachment.CapsuleID, attachment.TargetChatSessionID, mode, attachment.Enabled, now, now)
	return err
}

func (m *mariadbStore) DetachPersonaMemoryCapsule(ctx context.Context, capsuleID int64, targetChatSessionID string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, "DELETE FROM persona_capsule_attachments WHERE capsule_id = ? AND target_chat_session_id = ?", capsuleID, targetChatSessionID)
	return err
}

func (m *mariadbStore) ListPersonaCapsuleAttachments(ctx context.Context, targetChatSessionID string) ([]PersonaCapsuleAttachment, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, capsule_id, target_chat_session_id, injection_mode, enabled, created_at, updated_at
		FROM persona_capsule_attachments
		WHERE target_chat_session_id = ?
		ORDER BY updated_at DESC, id DESC
	`, targetChatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaCapsuleAttachment{}
	for rows.Next() {
		item, err := scanPersonaCapsuleAttachment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListAttachedPersonaMemoryEntries(ctx context.Context, targetChatSessionID string, limit int) ([]PersonaMemoryEntry, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 80 {
		limit = 80
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT e.id, e.capsule_id, e.source_memory_type, e.source_memory_id,
		       COALESCE(p.source_turn_index, e.source_turn_index),
		       COALESCE(NULLIF(p.memory_text, ''), e.memory_text),
		       COALESCE(p.emotional_weight, e.emotional_weight),
		       COALESCE(p.importance_10, e.importance_10),
		       COALESCE(NULLIF(p.portability, ''), e.portability),
		       COALESCE(p.tags_json, e.tags_json),
		       COALESCE(NULLIF(p.evidence_excerpt, ''), e.evidence_excerpt),
		       e.injection_policy,
		       e.created_at
		FROM persona_memory_entries e
		INNER JOIN persona_capsule_attachments a ON a.capsule_id = e.capsule_id
		LEFT JOIN protagonist_entity_memories p
			ON e.source_memory_type = 'subjective_entity_memory'
			AND e.source_memory_id = p.id
		WHERE a.target_chat_session_id = ? AND a.enabled = TRUE
		ORDER BY COALESCE(p.importance_10, e.importance_10) DESC, e.id ASC
		LIMIT ?
	`, targetChatSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryEntry{}
	for rows.Next() {
		item, err := scanPersonaMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) listPersonaMemoryEntriesByCapsule(ctx context.Context, capsuleID int64) ([]PersonaMemoryEntry, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT e.id, e.capsule_id, e.source_memory_type, e.source_memory_id,
		       COALESCE(p.source_turn_index, e.source_turn_index),
		       COALESCE(NULLIF(p.memory_text, ''), e.memory_text),
		       COALESCE(p.emotional_weight, e.emotional_weight),
		       COALESCE(p.importance_10, e.importance_10),
		       COALESCE(NULLIF(p.portability, ''), e.portability),
		       COALESCE(p.tags_json, e.tags_json),
		       COALESCE(NULLIF(p.evidence_excerpt, ''), e.evidence_excerpt),
		       e.injection_policy,
		       e.created_at
		FROM persona_memory_entries e
		LEFT JOIN protagonist_entity_memories p
			ON e.source_memory_type = 'subjective_entity_memory'
			AND e.source_memory_id = p.id
		WHERE e.capsule_id = ?
		ORDER BY COALESCE(p.source_turn_index, e.source_turn_index) ASC, e.id ASC
	`, capsuleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonaMemoryEntry{}
	for rows.Next() {
		item, err := scanPersonaMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) CreateProtagonistEntityMemory(ctx context.Context, item *ProtagonistEntityMemory) (*ProtagonistEntityMemory, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotEnabled
	}
	now := time.Now().UTC()
	portability := strings.TrimSpace(item.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	ownerEntityKey := strings.TrimSpace(item.OwnerEntityKey)
	if ownerEntityKey == "" {
		ownerEntityKey = strings.TrimSpace(item.PersonaEntityKey)
	}
	personaEntityKey := strings.TrimSpace(item.PersonaEntityKey)
	if personaEntityKey == "" {
		personaEntityKey = ownerEntityKey
	}
	ownerEntityName := strings.TrimSpace(item.OwnerEntityName)
	if ownerEntityName == "" {
		ownerEntityName = strings.TrimSpace(item.PersonaEntityName)
	}
	personaEntityName := strings.TrimSpace(item.PersonaEntityName)
	if personaEntityName == "" {
		personaEntityName = ownerEntityName
	}
	if ownerEntityName == "" {
		ownerEntityName = ownerEntityKey
	}
	if personaEntityName == "" {
		personaEntityName = personaEntityKey
	}
	ownerEntityRole := strings.TrimSpace(item.OwnerEntityRole)
	if ownerEntityRole == "" {
		ownerEntityRole = "protagonist"
	}
	ownerVisibility := strings.TrimSpace(item.OwnerVisibility)
	if ownerVisibility == "" {
		ownerVisibility = "player_known"
	}
	targetRevealPolicy := strings.TrimSpace(item.TargetRevealPolicy)
	if targetRevealPolicy == "" {
		targetRevealPolicy = "requires_explicit_attachment"
	}
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO protagonist_entity_memories
			(persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
			 owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
			 source_turn_index, memory_text, evidence_excerpt, secret_guard, portability, tags_json,
			 target_reveal_policy, importance_10, emotional_weight, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, personaEntityKey, personaEntityName, ownerEntityKey, ownerEntityName, ownerEntityRole, ownerVisibility,
		strings.TrimSpace(item.SourceChatSessionID), strings.TrimSpace(item.SourceCharacterName),
		item.SourceTurn, strings.TrimSpace(item.MemoryText), strings.TrimSpace(item.EvidenceExcerpt),
		item.SecretGuard, portability, strings.TrimSpace(item.TagsJSON), targetRevealPolicy,
		item.Importance10, item.EmotionalWeight, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	out := *item
	out.ID = id
	out.PersonaEntityKey = personaEntityKey
	out.PersonaEntityName = personaEntityName
	out.OwnerEntityKey = ownerEntityKey
	out.OwnerEntityName = ownerEntityName
	out.OwnerEntityRole = ownerEntityRole
	out.OwnerVisibility = ownerVisibility
	out.Portability = portability
	out.TargetRevealPolicy = targetRevealPolicy
	out.CreatedAt = now
	out.UpdatedAt = now
	return &out, nil
}

func (m *mariadbStore) ListProtagonistEntityMemories(ctx context.Context, filter ProtagonistEntityMemoryFilter) ([]ProtagonistEntityMemory, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `
		SELECT id, persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
		       owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
		       source_turn_index, memory_text, evidence_excerpt, secret_guard, portability, tags_json,
		       target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
		FROM protagonist_entity_memories
		WHERE 1 = 1`
	args := []any{}
	if strings.TrimSpace(filter.OwnerEntityKey) != "" {
		query += " AND (owner_entity_key = ? OR (owner_entity_key = '' AND persona_entity_key = ?))"
		key := strings.TrimSpace(filter.OwnerEntityKey)
		args = append(args, key, key)
	} else if strings.TrimSpace(filter.PersonaEntityKey) != "" {
		query += " AND (persona_entity_key = ? OR owner_entity_key = ?)"
		key := strings.TrimSpace(filter.PersonaEntityKey)
		args = append(args, key, key)
	}
	if strings.TrimSpace(filter.OwnerEntityRole) != "" {
		query += " AND owner_entity_role = ?"
		args = append(args, strings.TrimSpace(filter.OwnerEntityRole))
	}
	if strings.TrimSpace(filter.OwnerVisibility) != "" {
		query += " AND owner_visibility = ?"
		args = append(args, strings.TrimSpace(filter.OwnerVisibility))
	}
	if strings.TrimSpace(filter.SourceChatSessionID) != "" {
		query += " AND source_chat_session_id = ?"
		args = append(args, strings.TrimSpace(filter.SourceChatSessionID))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 80
	}
	if limit > 1000 {
		limit = 1000
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProtagonistEntityMemory{}
	for rows.Next() {
		item, err := scanProtagonistEntityMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) UpdateProtagonistEntityMemoryOwner(ctx context.Context, update ProtagonistEntityMemoryOwnerUpdate) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if update.ID <= 0 {
		return ErrNotFound
	}
	ownerKey := strings.TrimSpace(update.OwnerEntityKey)
	if ownerKey == "" {
		ownerKey = strings.TrimSpace(update.PersonaEntityKey)
	}
	personaKey := strings.TrimSpace(update.PersonaEntityKey)
	if personaKey == "" {
		personaKey = ownerKey
	}
	ownerName := strings.TrimSpace(update.OwnerEntityName)
	if ownerName == "" {
		ownerName = strings.TrimSpace(update.PersonaEntityName)
	}
	personaName := strings.TrimSpace(update.PersonaEntityName)
	if personaName == "" {
		personaName = ownerName
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if personaName == "" {
		personaName = personaKey
	}
	ownerRole := strings.TrimSpace(update.OwnerEntityRole)
	ownerVisibility := strings.TrimSpace(update.OwnerVisibility)
	res, err := m.db.ExecContext(ctx, `
		UPDATE protagonist_entity_memories
		SET persona_entity_key = ?,
		    persona_entity_name = ?,
		    owner_entity_key = ?,
		    owner_entity_name = ?,
		    owner_entity_role = CASE WHEN ? = '' THEN owner_entity_role ELSE ? END,
		    owner_visibility = CASE WHEN ? = '' THEN owner_visibility ELSE ? END,
		    tags_json = ?,
		    updated_at = ?
		WHERE id = ?
	`, personaKey, personaName, ownerKey, ownerName, ownerRole, ownerRole, ownerVisibility, ownerVisibility, strings.TrimSpace(update.TagsJSON), time.Now().UTC(), update.ID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) UpdateProtagonistEntityMemory(ctx context.Context, update ProtagonistEntityMemoryUpdate) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if update.ID <= 0 {
		return ErrNotFound
	}
	ownerKey := strings.TrimSpace(update.OwnerEntityKey)
	if ownerKey == "" {
		ownerKey = strings.TrimSpace(update.PersonaEntityKey)
	}
	personaKey := strings.TrimSpace(update.PersonaEntityKey)
	if personaKey == "" {
		personaKey = ownerKey
	}
	ownerName := strings.TrimSpace(update.OwnerEntityName)
	if ownerName == "" {
		ownerName = strings.TrimSpace(update.PersonaEntityName)
	}
	personaName := strings.TrimSpace(update.PersonaEntityName)
	if personaName == "" {
		personaName = ownerName
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if personaName == "" {
		personaName = personaKey
	}
	ownerRole := strings.TrimSpace(update.OwnerEntityRole)
	if ownerRole == "" {
		ownerRole = "protagonist"
	}
	ownerVisibility := strings.TrimSpace(update.OwnerVisibility)
	if ownerVisibility == "" {
		ownerVisibility = "player_known"
	}
	portability := strings.TrimSpace(update.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	targetRevealPolicy := strings.TrimSpace(update.TargetRevealPolicy)
	if targetRevealPolicy == "" {
		targetRevealPolicy = "requires_explicit_attachment"
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE protagonist_entity_memories
		SET persona_entity_key = ?,
		    persona_entity_name = ?,
		    owner_entity_key = ?,
		    owner_entity_name = ?,
		    owner_entity_role = ?,
		    owner_visibility = ?,
		    source_character_name = ?,
		    memory_text = ?,
		    evidence_excerpt = ?,
		    secret_guard = ?,
		    portability = ?,
		    tags_json = ?,
		    target_reveal_policy = ?,
		    importance_10 = ?,
		    emotional_weight = ?,
		    updated_at = ?
		WHERE id = ?
	`, personaKey, personaName, ownerKey, ownerName, ownerRole, ownerVisibility,
		strings.TrimSpace(update.SourceCharacterName), strings.TrimSpace(update.MemoryText), strings.TrimSpace(update.EvidenceExcerpt),
		update.SecretGuard, portability, strings.TrimSpace(update.TagsJSON), targetRevealPolicy,
		update.Importance10, update.EmotionalWeight, time.Now().UTC(), update.ID)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) DeleteProtagonistEntityMemory(ctx context.Context, id int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if id <= 0 {
		return ErrNotFound
	}
	res, err := m.db.ExecContext(ctx, `DELETE FROM protagonist_entity_memories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

type personaMemoryCapsuleScanner interface {
	Scan(dest ...any) error
}

func scanPersonaMemoryCapsule(scanner personaMemoryCapsuleScanner) (PersonaMemoryCapsule, error) {
	var item PersonaMemoryCapsule
	var sourceCharacterName, summary sql.NullString
	if err := scanner.Scan(&item.ID, &item.PersonaKey, &item.SourceChatSessionID, &sourceCharacterName, &item.Title, &item.Mode, &summary, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	item.SourceCharacterName = stringFromNull(sourceCharacterName)
	item.Summary = stringFromNull(summary)
	return item, nil
}

func scanPersonaMemoryEntry(scanner personaMemoryCapsuleScanner) (PersonaMemoryEntry, error) {
	var item PersonaMemoryEntry
	var sourceMemoryType sql.NullString
	var sourceMemoryID, sourceTurn sql.NullInt64
	var emotionalWeight, importance10 sql.NullFloat64
	var tagsJSON, evidenceExcerpt sql.NullString
	if err := scanner.Scan(&item.ID, &item.CapsuleID, &sourceMemoryType, &sourceMemoryID, &sourceTurn, &item.MemoryText, &emotionalWeight, &importance10, &item.Portability, &tagsJSON, &evidenceExcerpt, &item.InjectionPolicy, &item.CreatedAt); err != nil {
		return item, err
	}
	item.SourceMemoryType = stringFromNull(sourceMemoryType)
	item.SourceMemoryID = int64FromNull(sourceMemoryID)
	item.SourceTurn = intFromNull(sourceTurn)
	item.EmotionalWeight = float64FromNull(emotionalWeight)
	item.Importance10 = float64FromNull(importance10)
	item.TagsJSON = stringFromNull(tagsJSON)
	item.EvidenceExcerpt = stringFromNull(evidenceExcerpt)
	return item, nil
}

func scanPersonaCapsuleAttachment(scanner personaMemoryCapsuleScanner) (PersonaCapsuleAttachment, error) {
	var item PersonaCapsuleAttachment
	if err := scanner.Scan(&item.ID, &item.CapsuleID, &item.TargetChatSessionID, &item.InjectionMode, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return item, err
	}
	return item, nil
}

func scanProtagonistEntityMemory(scanner personaMemoryCapsuleScanner) (ProtagonistEntityMemory, error) {
	var item ProtagonistEntityMemory
	var sourceTurn sql.NullInt64
	var ownerEntityKey, ownerEntityName, ownerEntityRole, ownerVisibility sql.NullString
	var sourceCharacterName, evidenceExcerpt, tagsJSON, targetRevealPolicy sql.NullString
	var importance10, emotionalWeight sql.NullFloat64
	if err := scanner.Scan(
		&item.ID, &item.PersonaEntityKey, &item.PersonaEntityName, &ownerEntityKey, &ownerEntityName,
		&ownerEntityRole, &ownerVisibility, &item.SourceChatSessionID, &sourceCharacterName,
		&sourceTurn, &item.MemoryText, &evidenceExcerpt, &item.SecretGuard, &item.Portability,
		&tagsJSON, &targetRevealPolicy, &importance10, &emotionalWeight, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return item, err
	}
	item.OwnerEntityKey = stringFromNull(ownerEntityKey)
	if item.OwnerEntityKey == "" {
		item.OwnerEntityKey = item.PersonaEntityKey
	}
	item.OwnerEntityName = stringFromNull(ownerEntityName)
	if item.OwnerEntityName == "" {
		item.OwnerEntityName = item.PersonaEntityName
	}
	item.OwnerEntityRole = stringFromNull(ownerEntityRole)
	if item.OwnerEntityRole == "" {
		item.OwnerEntityRole = "protagonist"
	}
	item.OwnerVisibility = stringFromNull(ownerVisibility)
	if item.OwnerVisibility == "" {
		item.OwnerVisibility = "player_known"
	}
	item.SourceCharacterName = stringFromNull(sourceCharacterName)
	item.SourceTurn = intFromNull(sourceTurn)
	item.EvidenceExcerpt = stringFromNull(evidenceExcerpt)
	item.TagsJSON = stringFromNull(tagsJSON)
	item.TargetRevealPolicy = stringFromNull(targetRevealPolicy)
	if item.TargetRevealPolicy == "" {
		item.TargetRevealPolicy = "requires_explicit_attachment"
	}
	item.Importance10 = float64FromNull(importance10)
	item.EmotionalWeight = float64FromNull(emotionalWeight)
	return item, nil
}

func (m *mariadbStore) ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]ConsequenceRecord, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, source_turn_start, source_turn_end, decision, immediate_result,
		       delayed_effect, affected_relations, affected_world, status, importance, confidence,
		       foreground_eligible, quiet_turns, last_seen_turn, paid_turn, expires_after_quiet_turns,
		       source_hash, evidence_json, created_at, updated_at
		FROM consequence_records
		WHERE chat_session_id = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?
	`, chatSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ConsequenceRecord
	for rows.Next() {
		var item ConsequenceRecord
		var affectedRelations, affectedWorld, sourceHash, evidenceJSON sql.NullString
		var lastSeenTurn, paidTurn sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.SourceTurnStart, &item.SourceTurnEnd,
			&item.Decision, &item.ImmediateResult, &item.DelayedEffect,
			&affectedRelations, &affectedWorld, &item.Status, &item.Importance, &item.Confidence,
			&item.ForegroundEligible, &item.QuietTurns, &lastSeenTurn, &paidTurn, &item.ExpiresAfterQuietTurns,
			&sourceHash, &evidenceJSON, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.AffectedRelationsJSON = stringFromNull(affectedRelations)
		item.AffectedWorldJSON = stringFromNull(affectedWorld)
		item.SourceHash = stringFromNull(sourceHash)
		item.EvidenceJSON = stringFromNull(evidenceJSON)
		item.LastSeenTurn = intFromNull(lastSeenTurn)
		item.PaidTurn = intFromNull(paidTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveConsequenceRecord(ctx context.Context, record ConsequenceRecord) (ConsequenceRecord, error) {
	if err := m.ensureDB(); err != nil {
		return record, err
	}
	now := nonZeroTime(record.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO consequence_records (
			chat_session_id, source_turn_start, source_turn_end, decision, immediate_result,
			delayed_effect, affected_relations, affected_world, status, importance, confidence,
			foreground_eligible, quiet_turns, last_seen_turn, paid_turn, expires_after_quiet_turns,
			source_hash, evidence_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), ?, ?, ?, ?)
	`, record.ChatSessionID, record.SourceTurnStart, record.SourceTurnEnd, record.Decision,
		record.ImmediateResult, record.DelayedEffect, nullableString(record.AffectedRelationsJSON),
		nullableString(record.AffectedWorldJSON), firstNonEmptyString(record.Status, "active"),
		record.Importance, record.Confidence, record.ForegroundEligible, record.QuietTurns,
		record.LastSeenTurn, record.PaidTurn, record.ExpiresAfterQuietTurns,
		nullableString(record.SourceHash), nullableString(record.EvidenceJSON), now)
	if err != nil {
		return record, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		record.ID = id
	}
	record.CreatedAt = now
	record.UpdatedAt = now
	return record, nil
}

func (m *mariadbStore) UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("status is required")
	}
	query := `UPDATE consequence_records SET status = ?`
	args := []any{status}
	if paidTurn > 0 {
		query += `, paid_turn = NULLIF(?, 0)`
		args = append(args, paidTurn)
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	res, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]PsychologyBranch, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, character_name, branch_type, axis_name, summary, status,
		       confidence, confidence_label, source_kind, source_turn_start, source_turn_end,
		       source_hash, evidence_json, quiet_turns, last_seen_turn, dormant_after_quiet_turns,
		       created_at, updated_at
		FROM psychology_branches
		WHERE chat_session_id = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?
	`, chatSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PsychologyBranch
	for rows.Next() {
		var item PsychologyBranch
		var confidenceLabel, sourceKind, sourceHash, evidenceJSON sql.NullString
		var lastSeenTurn sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.CharacterName, &item.BranchType, &item.AxisName,
			&item.Summary, &item.Status, &item.Confidence, &confidenceLabel, &sourceKind,
			&item.SourceTurnStart, &item.SourceTurnEnd, &sourceHash, &evidenceJSON,
			&item.QuietTurns, &lastSeenTurn, &item.DormantAfterQuietTurns, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.ConfidenceLabel = stringFromNull(confidenceLabel)
		item.SourceKind = stringFromNull(sourceKind)
		item.SourceHash = stringFromNull(sourceHash)
		item.EvidenceJSON = stringFromNull(evidenceJSON)
		item.LastSeenTurn = intFromNull(lastSeenTurn)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SavePsychologyBranch(ctx context.Context, branch PsychologyBranch) (PsychologyBranch, error) {
	if err := m.ensureDB(); err != nil {
		return branch, err
	}
	now := nonZeroTime(branch.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO psychology_branches (
			chat_session_id, character_name, branch_type, axis_name, summary, status,
			confidence, confidence_label, source_kind, source_turn_start, source_turn_end,
			source_hash, evidence_json, quiet_turns, last_seen_turn, dormant_after_quiet_turns,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?)
	`, branch.ChatSessionID, branch.CharacterName, branch.BranchType, branch.AxisName, branch.Summary,
		firstNonEmptyString(branch.Status, "active"), branch.Confidence, nullableString(branch.ConfidenceLabel),
		nullableString(branch.SourceKind), branch.SourceTurnStart, branch.SourceTurnEnd,
		nullableString(branch.SourceHash), nullableString(branch.EvidenceJSON), branch.QuietTurns,
		branch.LastSeenTurn, branch.DormantAfterQuietTurns, now)
	if err != nil {
		return branch, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		branch.ID = id
	}
	branch.CreatedAt = now
	branch.UpdatedAt = now
	return branch, nil
}

func (m *mariadbStore) UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("status is required")
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE psychology_branches
		SET status = ?, quiet_turns = ?
		WHERE id = ?
	`, status, quietTurns, id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]ForkLineageRecord, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, scope_id, parent_scope_id, copied_from_scope_id,
		       copied_from_session_id, imported_at, divergence_marker, provenance_source,
		       inheritance_mode, inherited_items_json, created_at, updated_at
		FROM session_fork_lineage
		WHERE chat_session_id = ? AND (? = '' OR scope_id = ?)
		ORDER BY imported_at DESC, id DESC
		LIMIT ?
	`, chatSessionID, scopeID, scopeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ForkLineageRecord
	for rows.Next() {
		var item ForkLineageRecord
		var scopeID, parentScopeID, copiedFromScopeID, copiedFromSessionID sql.NullString
		var divergenceMarker, inheritedItemsJSON sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &scopeID, &parentScopeID, &copiedFromScopeID,
			&copiedFromSessionID, &item.ImportedAt, &divergenceMarker, &item.ProvenanceSource,
			&item.InheritanceMode, &inheritedItemsJSON, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.ScopeID = stringFromNull(scopeID)
		item.ParentScopeID = stringFromNull(parentScopeID)
		item.CopiedFromScopeID = stringFromNull(copiedFromScopeID)
		item.CopiedFromSessionID = stringFromNull(copiedFromSessionID)
		item.DivergenceMarker = stringFromNull(divergenceMarker)
		item.InheritedItemsJSON = stringFromNull(inheritedItemsJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveForkLineageRecord(ctx context.Context, record ForkLineageRecord) (ForkLineageRecord, error) {
	if err := m.ensureDB(); err != nil {
		return record, err
	}
	importedAt := nonZeroTime(record.ImportedAt)
	now := nonZeroTime(record.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO session_fork_lineage (
			chat_session_id, scope_id, parent_scope_id, copied_from_scope_id, copied_from_session_id,
			imported_at, divergence_marker, provenance_source, inheritance_mode, inherited_items_json,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ChatSessionID, nullableString(record.ScopeID), nullableString(record.ParentScopeID),
		nullableString(record.CopiedFromScopeID), nullableString(record.CopiedFromSessionID),
		importedAt, nullableString(record.DivergenceMarker),
		firstNonEmptyString(record.ProvenanceSource, "manual"),
		firstNonEmptyString(record.InheritanceMode, "conservative_import"),
		nullableString(record.InheritedItemsJSON), now)
	if err != nil {
		return record, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		record.ID = id
	}
	record.ImportedAt = importedAt
	record.CreatedAt = now
	record.UpdatedAt = now
	return record, nil
}

func (m *mariadbStore) ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]ThemeOffscreenCarryRecord, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, surface_type, label, summary, status, confidence,
		       confidence_label, source_kind, source_turn_start, source_turn_end,
		       source_hash, evidence_json, quiet_turns, last_seen_turn,
		       dormant_after_quiet_turns, foreground_eligible, foreground_reason_json,
		       created_at, updated_at
		FROM theme_offscreen_carries
		WHERE chat_session_id = ? AND (? = '' OR surface_type = ?)
		ORDER BY foreground_eligible DESC, updated_at DESC, id DESC
		LIMIT ?
	`, chatSessionID, surfaceType, surfaceType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ThemeOffscreenCarryRecord
	for rows.Next() {
		var item ThemeOffscreenCarryRecord
		var confidenceLabel, sourceKind, sourceHash, evidenceJSON, foregroundReasonJSON sql.NullString
		var lastSeenTurn sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.SurfaceType, &item.Label, &item.Summary,
			&item.Status, &item.Confidence, &confidenceLabel, &sourceKind,
			&item.SourceTurnStart, &item.SourceTurnEnd, &sourceHash, &evidenceJSON,
			&item.QuietTurns, &lastSeenTurn, &item.DormantAfterQuietTurns, &item.ForegroundEligible,
			&foregroundReasonJSON, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.ConfidenceLabel = stringFromNull(confidenceLabel)
		item.SourceKind = stringFromNull(sourceKind)
		item.SourceHash = stringFromNull(sourceHash)
		item.EvidenceJSON = stringFromNull(evidenceJSON)
		item.LastSeenTurn = intFromNull(lastSeenTurn)
		item.ForegroundReasonJSON = stringFromNull(foregroundReasonJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveThemeOffscreenCarry(ctx context.Context, record ThemeOffscreenCarryRecord) (ThemeOffscreenCarryRecord, error) {
	if err := m.ensureDB(); err != nil {
		return record, err
	}
	now := nonZeroTime(record.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO theme_offscreen_carries (
			chat_session_id, surface_type, label, summary, status, confidence,
			confidence_label, source_kind, source_turn_start, source_turn_end,
			source_hash, evidence_json, quiet_turns, last_seen_turn,
			dormant_after_quiet_turns, foreground_eligible, foreground_reason_json,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?)
	`, record.ChatSessionID, record.SurfaceType, record.Label, record.Summary,
		firstNonEmptyString(record.Status, "active"), record.Confidence, nullableString(record.ConfidenceLabel),
		nullableString(record.SourceKind), record.SourceTurnStart, record.SourceTurnEnd,
		nullableString(record.SourceHash), nullableString(record.EvidenceJSON), record.QuietTurns,
		record.LastSeenTurn, record.DormantAfterQuietTurns, record.ForegroundEligible,
		nullableString(record.ForegroundReasonJSON), now)
	if err != nil {
		return record, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		record.ID = id
	}
	record.CreatedAt = now
	record.UpdatedAt = now
	return record, nil
}

func (m *mariadbStore) UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("status is required")
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE theme_offscreen_carries
		SET status = ?, quiet_turns = ?
		WHERE id = ?
	`, status, quietTurns, id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]CaptureVerificationRecord, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, chat_session_id, turn_index, stage_name, verification_state, degraded_reason,
		       compact_metadata_json, content_hash, evidence_json, previous_record_id, repaired_by_record_id,
		       repair_attempt_count, repair_evidence_json, repaired_at, user_input_preserved, payload_rewrite,
		       created_at, updated_at
		FROM capture_verification_records
		WHERE chat_session_id = ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?
	`, chatSessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CaptureVerificationRecord
	for rows.Next() {
		var item CaptureVerificationRecord
		var degradedReason, compactMetadata, contentHash, evidenceJSON, repairEvidence sql.NullString
		var previousID, repairedByID sql.NullInt64
		var repairedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.TurnIndex, &item.StageName, &item.VerificationState, &degradedReason,
			&compactMetadata, &contentHash, &evidenceJSON, &previousID, &repairedByID,
			&item.RepairAttemptCount, &repairEvidence, &repairedAt, &item.UserInputPreserved, &item.PayloadRewrite,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.DegradedReason = stringFromNull(degradedReason)
		item.CompactMetadataJSON = stringFromNull(compactMetadata)
		item.ContentHash = stringFromNull(contentHash)
		item.EvidenceJSON = stringFromNull(evidenceJSON)
		item.PreviousRecordID = int64FromNull(previousID)
		item.RepairedByRecordID = int64FromNull(repairedByID)
		item.RepairEvidenceJSON = stringFromNull(repairEvidence)
		item.RepairedAt = timeFromNull(repairedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveCaptureVerification(ctx context.Context, record CaptureVerificationRecord) (CaptureVerificationRecord, error) {
	if err := m.ensureDB(); err != nil {
		return record, err
	}
	now := nonZeroTime(record.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO capture_verification_records (
			chat_session_id, turn_index, stage_name, verification_state, degraded_reason,
			compact_metadata_json, content_hash, evidence_json, previous_record_id, repaired_by_record_id,
			repair_attempt_count, repair_evidence_json, repaired_at, user_input_preserved, payload_rewrite, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0), ?, ?, NULLIF(?, '0000-00-00 00:00:00.000'), ?, ?, ?)
	`, record.ChatSessionID, record.TurnIndex, firstNonEmptyString(record.StageName, "afterRequest"),
		firstNonEmptyString(record.VerificationState, "single-stage"), nullableString(record.DegradedReason),
		nullableString(record.CompactMetadataJSON), nullableString(record.ContentHash), nullableString(record.EvidenceJSON),
		record.PreviousRecordID, record.RepairedByRecordID, record.RepairAttemptCount,
		nullableString(record.RepairEvidenceJSON), nullableTime(record.RepairedAt), record.UserInputPreserved,
		record.PayloadRewrite, now)
	if err != nil {
		return record, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		record.ID = id
	}
	record.CreatedAt = now
	record.UpdatedAt = now
	return record, nil
}

func (m *mariadbStore) UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	state = strings.TrimSpace(state)
	if state == "" {
		return errors.New("state is required")
	}
	// Do not mark repair success without evidence.
	if state == "verified" || state == "verified-final" {
		if strings.TrimSpace(repairEvidenceJSON) == "" {
			return errors.New("repair success state requires repair_evidence_json")
		}
	}
	if state == "degraded" && strings.TrimSpace(degradedReason) == "" {
		return errors.New("degraded state requires degraded_reason")
	}
	if !userInputPreserved && state != "degraded" {
		return errors.New("user_input_preserved=false requires degraded verification_state")
	}
	query := `UPDATE capture_verification_records SET verification_state = ?, degraded_reason = NULLIF(?, ''), repair_evidence_json = NULLIF(?, ''), repaired_by_record_id = NULLIF(?, 0), user_input_preserved = ?, updated_at = CURRENT_TIMESTAMP(3)`
	args := []any{state, degradedReason, repairEvidenceJSON, repairedByID, userInputPreserved}
	if state == "verified" || state == "verified-final" || strings.TrimSpace(repairEvidenceJSON) != "" {
		query += `, repaired_at = CURRENT_TIMESTAMP(3)`
	}
	if strings.TrimSpace(repairEvidenceJSON) != "" {
		query += `, repair_attempt_count = repair_attempt_count + 1`
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	res, err := m.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT id, chat_session_id, input_channel, proposal_state, schema_name, ruleset_label,
		       schema_json, provenance_json, review_note, reviewer, reviewed_at, created_at, updated_at
		FROM status_schema_proposals
		WHERE chat_session_id = ?
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(proposalState) != "" {
		query += ` AND proposal_state = ?`
		args = append(args, strings.TrimSpace(proposalState))
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatusSchemaProposal
	for rows.Next() {
		var item StatusSchemaProposal
		var rulesetLabel, provenanceJSON, reviewNote, reviewer sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.InputChannel, &item.ProposalState, &item.SchemaName, &rulesetLabel,
			&item.SchemaJSON, &provenanceJSON, &reviewNote, &reviewer, &reviewedAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.RulesetLabel = stringFromNull(rulesetLabel)
		item.ProvenanceJSON = stringFromNull(provenanceJSON)
		item.ReviewNote = stringFromNull(reviewNote)
		item.Reviewer = stringFromNull(reviewer)
		item.ReviewedAt = timeFromNull(reviewedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetStatusSchemaProposal(ctx context.Context, id int64) (StatusSchemaProposal, error) {
	if err := m.ensureDB(); err != nil {
		return StatusSchemaProposal{}, err
	}
	var item StatusSchemaProposal
	var rulesetLabel, provenanceJSON, reviewNote, reviewer sql.NullString
	var reviewedAt sql.NullTime
	err := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, input_channel, proposal_state, schema_name, ruleset_label,
		       schema_json, provenance_json, review_note, reviewer, reviewed_at, created_at, updated_at
		FROM status_schema_proposals
		WHERE id = ?
	`, id).Scan(
		&item.ID, &item.ChatSessionID, &item.InputChannel, &item.ProposalState, &item.SchemaName, &rulesetLabel,
		&item.SchemaJSON, &provenanceJSON, &reviewNote, &reviewer, &reviewedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StatusSchemaProposal{}, ErrNotFound
		}
		return StatusSchemaProposal{}, err
	}
	item.RulesetLabel = stringFromNull(rulesetLabel)
	item.ProvenanceJSON = stringFromNull(provenanceJSON)
	item.ReviewNote = stringFromNull(reviewNote)
	item.Reviewer = stringFromNull(reviewer)
	item.ReviewedAt = timeFromNull(reviewedAt)
	return item, nil
}

func (m *mariadbStore) SaveStatusSchemaProposal(ctx context.Context, proposal StatusSchemaProposal) (StatusSchemaProposal, error) {
	if err := m.ensureDB(); err != nil {
		return proposal, err
	}
	now := nonZeroTime(proposal.CreatedAt)
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO status_schema_proposals (
			chat_session_id, input_channel, proposal_state, schema_name, ruleset_label,
			schema_json, provenance_json, review_note, reviewer, reviewed_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, proposal.ChatSessionID, firstNonEmptyString(proposal.InputChannel, "bootstrap"),
		firstNonEmptyString(proposal.ProposalState, "pending_review"),
		firstNonEmptyString(proposal.SchemaName, "status_schema"), nullableString(proposal.RulesetLabel),
		proposal.SchemaJSON, nullableString(proposal.ProvenanceJSON), nullableString(proposal.ReviewNote),
		nullableString(proposal.Reviewer), nullableTime(proposal.ReviewedAt), now)
	if err != nil {
		return proposal, err
	}
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		proposal.ID = id
	}
	proposal.CreatedAt = now
	proposal.UpdatedAt = now
	if strings.TrimSpace(proposal.InputChannel) == "" {
		proposal.InputChannel = "bootstrap"
	}
	if strings.TrimSpace(proposal.ProposalState) == "" {
		proposal.ProposalState = "pending_review"
	}
	if strings.TrimSpace(proposal.SchemaName) == "" {
		proposal.SchemaName = "status_schema"
	}
	return proposal, nil
}

func (m *mariadbStore) UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	proposalState = strings.TrimSpace(proposalState)
	if proposalState == "" {
		return errors.New("proposal_state is required")
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE status_schema_proposals
		SET proposal_state = ?, review_note = NULLIF(?, ''), reviewer = NULLIF(?, ''),
		    reviewed_at = CURRENT_TIMESTAMP(3), updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, proposalState, strings.TrimSpace(reviewNote), strings.TrimSpace(reviewer), id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]StatusSchemaDefinition, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT id, chat_session_id, source_proposal_id, schema_name, ruleset_label,
		       status_key, label, owner_scope, value_kind, bounds_json, options_json,
		       default_value_json, registry_state, created_at, updated_at
		FROM status_schema_registry
		WHERE chat_session_id = ?
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(registryState) != "" {
		query += ` AND registry_state = ?`
		args = append(args, strings.TrimSpace(registryState))
	}
	query += ` ORDER BY schema_name ASC, status_key ASC, id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatusSchemaDefinition
	for rows.Next() {
		var item StatusSchemaDefinition
		var proposalID sql.NullInt64
		var rulesetLabel, boundsJSON, optionsJSON, defaultValueJSON sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &proposalID, &item.SchemaName, &rulesetLabel,
			&item.StatusKey, &item.Label, &item.OwnerScope, &item.ValueKind, &boundsJSON, &optionsJSON,
			&defaultValueJSON, &item.RegistryState, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SourceProposalID = int64FromNull(proposalID)
		item.RulesetLabel = stringFromNull(rulesetLabel)
		item.BoundsJSON = stringFromNull(boundsJSON)
		item.OptionsJSON = stringFromNull(optionsJSON)
		item.DefaultValueJSON = stringFromNull(defaultValueJSON)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (StatusSchemaDefinition, error) {
	if err := m.ensureDB(); err != nil {
		return StatusSchemaDefinition{}, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, source_proposal_id, schema_name, ruleset_label,
		       status_key, label, owner_scope, value_kind, bounds_json, options_json,
		       default_value_json, registry_state, created_at, updated_at
		FROM status_schema_registry
		WHERE chat_session_id = ? AND status_key = ? AND owner_scope = ? AND registry_state = 'active'
		ORDER BY id DESC
		LIMIT 1
	`, chatSessionID, statusKey, ownerScope)
	var item StatusSchemaDefinition
	var proposalID sql.NullInt64
	var rulesetLabel, boundsJSON, optionsJSON, defaultValueJSON sql.NullString
	if err := row.Scan(
		&item.ID, &item.ChatSessionID, &proposalID, &item.SchemaName, &rulesetLabel,
		&item.StatusKey, &item.Label, &item.OwnerScope, &item.ValueKind, &boundsJSON, &optionsJSON,
		&defaultValueJSON, &item.RegistryState, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StatusSchemaDefinition{}, ErrNotFound
		}
		return StatusSchemaDefinition{}, err
	}
	item.SourceProposalID = int64FromNull(proposalID)
	item.RulesetLabel = stringFromNull(rulesetLabel)
	item.BoundsJSON = stringFromNull(boundsJSON)
	item.OptionsJSON = stringFromNull(optionsJSON)
	item.DefaultValueJSON = stringFromNull(defaultValueJSON)
	return item, nil
}

func (m *mariadbStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []StatusSchemaDefinition) ([]StatusSchemaDefinition, error) {
	if err := m.ensureDB(); err != nil {
		return definitions, err
	}
	out := make([]StatusSchemaDefinition, 0, len(definitions))
	for _, def := range definitions {
		now := nonZeroTime(def.CreatedAt)
		state := firstNonEmptyString(def.RegistryState, "active")
		res, err := m.db.ExecContext(ctx, `
			INSERT INTO status_schema_registry (
				chat_session_id, source_proposal_id, schema_name, ruleset_label,
				status_key, label, owner_scope, value_kind, bounds_json, options_json,
				default_value_json, registry_state, created_at
			) VALUES (?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, def.ChatSessionID, def.SourceProposalID, firstNonEmptyString(def.SchemaName, "status_schema"),
			nullableString(def.RulesetLabel), def.StatusKey, firstNonEmptyString(def.Label, def.StatusKey),
			def.OwnerScope, def.ValueKind, nullableString(def.BoundsJSON), nullableString(def.OptionsJSON),
			nullableString(def.DefaultValueJSON), state, now)
		if err != nil {
			return out, err
		}
		if id, err := res.LastInsertId(); err == nil && id > 0 {
			def.ID = id
		}
		def.CreatedAt = now
		def.UpdatedAt = now
		def.RegistryState = state
		if strings.TrimSpace(def.SchemaName) == "" {
			def.SchemaName = "status_schema"
		}
		if strings.TrimSpace(def.Label) == "" {
			def.Label = def.StatusKey
		}
		out = append(out, def)
	}
	return out, nil
}

func (m *mariadbStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusCurrentValue, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT id, chat_session_id, registry_id, status_key, owner_scope, owner_id,
		       owner_label, value_kind, value_json, evidence_json, source_turn,
		       write_state, created_at, updated_at
		FROM status_current_values
		WHERE chat_session_id = ? AND write_state = 'current'
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(ownerScope) != "" {
		query += ` AND owner_scope = ?`
		args = append(args, strings.TrimSpace(ownerScope))
	}
	if strings.TrimSpace(ownerID) != "" {
		query += ` AND owner_id = ?`
		args = append(args, strings.TrimSpace(ownerID))
	}
	if strings.TrimSpace(statusKey) != "" {
		query += ` AND status_key = ?`
		args = append(args, strings.TrimSpace(statusKey))
	}
	query += ` ORDER BY owner_scope ASC, owner_id ASC, status_key ASC, updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatusCurrentValue
	for rows.Next() {
		var item StatusCurrentValue
		var ownerLabel sql.NullString
		var sourceTurn sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.RegistryID, &item.StatusKey, &item.OwnerScope, &item.OwnerID,
			&ownerLabel, &item.ValueKind, &item.ValueJSON, &item.EvidenceJSON, &sourceTurn,
			&item.WriteState, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.OwnerLabel = stringFromNull(ownerLabel)
		if sourceTurn.Valid {
			item.SourceTurn = int(sourceTurn.Int64)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveStatusCurrentValue(ctx context.Context, value StatusCurrentValue) (StatusCurrentValue, error) {
	if err := m.ensureDB(); err != nil {
		return value, err
	}
	now := nonZeroTime(value.CreatedAt)
	state := firstNonEmptyString(value.WriteState, "current")
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO status_current_values (
			chat_session_id, registry_id, status_key, owner_scope, owner_id, owner_label,
			value_kind, value_json, evidence_json, source_turn, write_state, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?)
		ON DUPLICATE KEY UPDATE
			id = LAST_INSERT_ID(id),
			status_key = VALUES(status_key),
			owner_label = VALUES(owner_label),
			value_kind = VALUES(value_kind),
			value_json = VALUES(value_json),
			evidence_json = VALUES(evidence_json),
			source_turn = VALUES(source_turn),
			write_state = VALUES(write_state),
			updated_at = CURRENT_TIMESTAMP(3)
	`, value.ChatSessionID, value.RegistryID, value.StatusKey, value.OwnerScope, value.OwnerID, nullableString(value.OwnerLabel),
		value.ValueKind, value.ValueJSON, value.EvidenceJSON, value.SourceTurn, state, now)
	if err != nil {
		return value, err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		value.ID = id
	}
	value.CreatedAt = now
	value.UpdatedAt = now
	value.WriteState = state
	return value, nil
}

func (m *mariadbStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT id, chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
		       event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
		       story_clock_json, event_state, created_at
		FROM status_change_events
		WHERE chat_session_id = ?
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(ownerScope) != "" {
		query += ` AND owner_scope = ?`
		args = append(args, strings.TrimSpace(ownerScope))
	}
	if strings.TrimSpace(ownerID) != "" {
		query += ` AND owner_id = ?`
		args = append(args, strings.TrimSpace(ownerID))
	}
	if strings.TrimSpace(statusKey) != "" {
		query += ` AND status_key = ?`
		args = append(args, strings.TrimSpace(statusKey))
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatusChangeEvent
	for rows.Next() {
		var item StatusChangeEvent
		var statusValueID, sourceTurn sql.NullInt64
		var previousValueJSON, newValueJSON, storyClockJSON sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.RegistryID, &statusValueID, &item.StatusKey, &item.OwnerScope, &item.OwnerID,
			&item.EventKind, &previousValueJSON, &newValueJSON, &item.EvidenceJSON, &sourceTurn,
			&storyClockJSON, &item.EventState, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.StatusValueID = int64FromNull(statusValueID)
		item.PreviousValueJSON = stringFromNull(previousValueJSON)
		item.NewValueJSON = stringFromNull(newValueJSON)
		item.StoryClockJSON = stringFromNull(storyClockJSON)
		if sourceTurn.Valid {
			item.SourceTurn = int(sourceTurn.Int64)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveStatusChangeEvent(ctx context.Context, event StatusChangeEvent) (StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return event, err
	}
	now := nonZeroTime(event.CreatedAt)
	state := firstNonEmptyString(event.EventState, "recorded")
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO status_change_events (
			chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
			event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
			story_clock_json, event_state, created_at
		) VALUES (?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)
	`, event.ChatSessionID, event.RegistryID, event.StatusValueID, event.StatusKey, event.OwnerScope, event.OwnerID,
		event.EventKind, nullableString(event.PreviousValueJSON), nullableString(event.NewValueJSON), event.EvidenceJSON, event.SourceTurn,
		nullableString(event.StoryClockJSON), state, now)
	if err != nil {
		return event, err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		event.ID = id
	}
	event.CreatedAt = now
	event.EventState = state
	return event, nil
}

func (m *mariadbStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]StatusEffect, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT id, chat_session_id, registry_id, status_key, owner_scope, owner_id,
		       effect_kind, effect_label, effect_payload_json, evidence_json, source_turn,
		       start_clock_json, duration_json, expires_at_clock_json, effect_state,
		       cleared_evidence_json, cleared_turn, created_at, updated_at
		FROM status_effects
		WHERE chat_session_id = ?
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(ownerScope) != "" {
		query += ` AND owner_scope = ?`
		args = append(args, strings.TrimSpace(ownerScope))
	}
	if strings.TrimSpace(ownerID) != "" {
		query += ` AND owner_id = ?`
		args = append(args, strings.TrimSpace(ownerID))
	}
	if strings.TrimSpace(effectState) != "" {
		query += ` AND effect_state = ?`
		args = append(args, strings.TrimSpace(effectState))
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatusEffect
	for rows.Next() {
		var item StatusEffect
		var effectLabel, payloadJSON, durationJSON, expiresJSON, clearedEvidence sql.NullString
		var sourceTurn, clearedTurn sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.ChatSessionID, &item.RegistryID, &item.StatusKey, &item.OwnerScope, &item.OwnerID,
			&item.EffectKind, &effectLabel, &payloadJSON, &item.EvidenceJSON, &sourceTurn,
			&item.StartClockJSON, &durationJSON, &expiresJSON, &item.EffectState,
			&clearedEvidence, &clearedTurn, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.EffectLabel = stringFromNull(effectLabel)
		item.EffectPayloadJSON = stringFromNull(payloadJSON)
		item.DurationJSON = stringFromNull(durationJSON)
		item.ExpiresAtClockJSON = stringFromNull(expiresJSON)
		item.ClearedEvidenceJSON = stringFromNull(clearedEvidence)
		if sourceTurn.Valid {
			item.SourceTurn = int(sourceTurn.Int64)
		}
		if clearedTurn.Valid {
			item.ClearedTurn = int(clearedTurn.Int64)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) SaveStatusEffect(ctx context.Context, effect StatusEffect) (StatusEffect, error) {
	if err := m.ensureDB(); err != nil {
		return effect, err
	}
	now := nonZeroTime(effect.CreatedAt)
	state := firstNonEmptyString(effect.EffectState, "active")
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO status_effects (
			chat_session_id, registry_id, status_key, owner_scope, owner_id,
			effect_kind, effect_label, effect_payload_json, evidence_json, source_turn,
			start_clock_json, duration_json, expires_at_clock_json, effect_state, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?)
	`, effect.ChatSessionID, effect.RegistryID, effect.StatusKey, effect.OwnerScope, effect.OwnerID,
		effect.EffectKind, nullableString(effect.EffectLabel), nullableString(effect.EffectPayloadJSON), effect.EvidenceJSON, effect.SourceTurn,
		effect.StartClockJSON, nullableString(effect.DurationJSON), nullableString(effect.ExpiresAtClockJSON), state, now)
	if err != nil {
		return effect, err
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		effect.ID = id
	}
	effect.CreatedAt = now
	effect.UpdatedAt = now
	effect.EffectState = state
	return effect, nil
}

func (m *mariadbStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	res, err := m.db.ExecContext(ctx, `
		UPDATE status_effects
		SET effect_state = ?, cleared_evidence_json = NULLIF(?, ''), cleared_turn = NULLIF(?, 0),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, strings.TrimSpace(effectState), strings.TrimSpace(clearedEvidenceJSON), clearedTurn, id)
	if err != nil {
		return err
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *mariadbStore) DeleteSession(ctx context.Context, chatSessionID string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM persona_capsule_attachments WHERE target_chat_session_id = ?", chatSessionID); err != nil {
		return err
	}
	if _, err := m.db.ExecContext(ctx, "DELETE FROM protagonist_entity_memories WHERE source_chat_session_id = ?", chatSessionID); err != nil {
		return err
	}
	tables := []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"character_events",
		"storylines",
		"world_rules",
		"character_states",
		"pending_threads",
		"active_states",
		"canonical_state_layers",
		"episode_summaries",
		"chapter_summaries",
		"arc_summaries",
		"saga_digests",
		"session_active_scopes",
		"guidance_plan_states",
		"entities",
		"trust_states",
		"consequence_records",
		"psychology_branches",
		"session_fork_lineage",
		"theme_offscreen_carries",
		"capture_verification_records",
		"status_effects",
		"status_change_events",
		"status_current_values",
		"status_schema_registry",
		"status_schema_proposals",
		"critic_feedback",
	}
	for _, tbl := range tables {
		if _, err := m.db.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE chat_session_id = ?", chatSessionID); err != nil {
			return err
		}
	}
	return nil
}
