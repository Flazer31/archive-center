package store

import (
	"context"
	"database/sql"
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
	"session_reference_coverage_fields",
	"session_reference_coverage_snapshots",
	"session_reference_runtime",
	"session_reference_bindings",
	"reference_claim_knowers",
	"reference_claims",
	"reference_entity_aliases",
	"reference_entities",
	"reference_timeline_nodes",
	"reference_documents",
	"reference_continuities",
	"reference_works",
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
