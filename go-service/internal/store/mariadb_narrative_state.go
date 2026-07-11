package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
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
