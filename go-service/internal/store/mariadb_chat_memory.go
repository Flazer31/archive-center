package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
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
	res, err := m.db.ExecContext(ctx, `
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
	if err == nil {
		if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
			e.ID = id
		}
	}
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
