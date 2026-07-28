package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

var _ MemoryAdmissionWriter = (*mariadbStore)(nil)
var _ MemoryAdmissionWriteAvailability = (*mariadbStore)(nil)

func (m *mariadbStore) MemoryAdmissionWritesEnabled() bool {
	return m != nil && m.db != nil
}

func (m *mariadbStore) CommitMemoryAdmission(ctx context.Context, admission *MemoryAdmission) (MemoryAdmissionResult, error) {
	var result MemoryAdmissionResult
	if err := m.ensureDB(); err != nil {
		return result, err
	}
	if err := validateMemoryAdmission(admission); err != nil {
		return result, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var sourceState, admissionState, derivationVersion, extractorVersion, indexVersion string
	var existingResultHash, existingResultJSON sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT lifecycle_state, derived_admission_state,
		       derived_admission_version, derived_extractor_version,
		       derived_index_version, derived_result_hash, derived_result_json
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ? AND turn_index = ?
		FOR UPDATE
	`, admission.ChatSessionID, admission.SourceRevision, admission.TurnIndex).Scan(
		&sourceState, &admissionState, &derivationVersion,
		&extractorVersion, &indexVersion, &existingResultHash,
		&existingResultJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return result, ErrSourceRevisionStale
		}
		return result, err
	}
	if sourceState != "active" {
		return result, ErrSourceRevisionStale
	}
	if admissionState == "committed" &&
		derivationVersion == admission.DerivationVersion &&
		extractorVersion == admission.ExtractorVersion &&
		indexVersion == admission.IndexVersion {
		result.Idempotent = true
		result.ExistingResultHash = existingResultHash.String
		result.ExistingResultJSON = existingResultJSON.String
		result.CommittedResultHash = existingResultHash.String
		if err := tx.Commit(); err != nil {
			return result, err
		}
		committed = true
		return result, nil
	}

	memoryID, inserted, updated, err := commitAdmissionMemoryTx(ctx, tx, admission.Memory)
	if err != nil {
		return result, err
	}
	result.MemoryInserted = inserted
	result.MemoryUpdated = updated

	evidenceByText, evidenceResult, err := reconcileAdmissionEvidenceTx(ctx, tx, admission)
	if err != nil {
		return result, err
	}
	result.EvidenceInserted = evidenceResult.inserted
	result.EvidenceReactivated = evidenceResult.reactivated
	result.EvidenceRetired = evidenceResult.retired
	result.VectorOperations += evidenceResult.vectorOperations

	for _, unit := range admission.PreciseUnits {
		if unit == nil {
			continue
		}
		evidenceID := evidenceByText[strings.TrimSpace(unit.EvidenceExcerpt)]
		if evidenceID <= 0 {
			return result, fmt.Errorf("precise memory evidence was not committed: %s", unit.EvidenceHash)
		}
		unit.RootEvidenceID = evidenceID
		unit.DirectEvidenceIDsJSON = mustJSON([]int64{evidenceID})
	}
	preciseResult, err := reconcileAdmissionPreciseMemoryTx(ctx, tx, admission)
	if err != nil {
		return result, err
	}
	result.PreciseInserted = preciseResult.inserted
	result.PreciseReactivated = preciseResult.reactivated
	result.PreciseRetired = preciseResult.retired
	result.VectorOperations += preciseResult.vectorOperations

	vectorCount, err := enqueueAdmissionVectorsTx(ctx, tx, admission, memoryID, evidenceByText)
	if err != nil {
		return result, err
	}
	result.VectorOperations += vectorCount

	admittedAt := nonZeroTime(admission.CreatedAt)
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_source_revisions
		SET derived_admission_state = 'committed',
		    derived_admission_version = ?,
		    derived_extractor_version = ?,
		    derived_index_version = ?,
		    derived_result_hash = ?,
		    derived_result_json = ?,
		    derived_admitted_at = ?,
		    updated_at = ?
		WHERE chat_session_id = ? AND source_revision = ?
		  AND lifecycle_state = 'active'
	`, admission.DerivationVersion, admission.ExtractorVersion,
		admission.IndexVersion, admission.ResultHash, admission.ResultJSON,
		admittedAt, admittedAt,
		admission.ChatSessionID, admission.SourceRevision); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	result.CommittedResultHash = admission.ResultHash
	result.CommittedAt = admittedAt
	return result, nil
}

func validateMemoryAdmission(admission *MemoryAdmission) error {
	var extraction map[string]any
	if admission == nil ||
		strings.TrimSpace(admission.ContractVersion) != MemoryAdmissionContract ||
		strings.TrimSpace(admission.ChatSessionID) == "" ||
		strings.TrimSpace(admission.SourceRevision) == "" ||
		admission.TurnIndex <= 0 ||
		strings.TrimSpace(admission.DerivationVersion) == "" ||
		strings.TrimSpace(admission.ExtractorVersion) == "" ||
		strings.TrimSpace(admission.IndexVersion) == "" ||
		len(strings.TrimSpace(admission.ResultHash)) != 64 ||
		json.Unmarshal([]byte(admission.ResultJSON), &extraction) != nil ||
		extraction == nil {
		return fmt.Errorf("invalid memory admission")
	}
	if admission.ResultHash != memoryAdmissionExpectedResultHash(admission) {
		return fmt.Errorf("memory admission result hash mismatch")
	}
	if admission.Memory != nil &&
		(admission.Memory.ChatSessionID != admission.ChatSessionID ||
			admission.Memory.TurnIndex != admission.TurnIndex) {
		return fmt.Errorf("memory admission projection identity mismatch")
	}
	for _, evidence := range admission.Evidence {
		if evidence == nil {
			continue
		}
		if evidence.ChatSessionID != admission.ChatSessionID ||
			evidence.SourceTurnStart != admission.TurnIndex ||
			evidence.SourceTurnEnd != admission.TurnIndex ||
			evidence.CaptureStage != "critic_extract" {
			return fmt.Errorf("evidence admission projection identity mismatch")
		}
	}
	for _, unit := range admission.PreciseUnits {
		if unit == nil {
			continue
		}
		if unit.ChatSessionID != admission.ChatSessionID ||
			unit.SourceRevision != admission.SourceRevision ||
			unit.SourceTurnStart != admission.TurnIndex ||
			unit.SourceTurnEnd != admission.TurnIndex {
			return fmt.Errorf("precise memory admission projection identity mismatch")
		}
	}
	return nil
}

func memoryAdmissionExpectedResultHash(admission *MemoryAdmission) string {
	if admission == nil {
		return ""
	}
	material := strings.Join([]string{
		strings.TrimSpace(admission.SourceRevision),
		strings.TrimSpace(admission.DerivationVersion),
		strings.TrimSpace(admission.ExtractorVersion),
		strings.TrimSpace(admission.IndexVersion),
		strings.TrimSpace(admission.ResultJSON),
	}, "\x1f")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(material)))
}

func commitAdmissionMemoryTx(ctx context.Context, tx *sql.Tx, mem *Memory) (id int64, inserted, updated bool, err error) {
	if mem == nil {
		return 0, false, false, nil
	}
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM memories
		WHERE chat_session_id = ? AND turn_index = ?
		ORDER BY id
		LIMIT 1
		FOR UPDATE
	`, mem.ChatSessionID, mem.TurnIndex).Scan(&id)
	switch err {
	case nil:
		_, err = tx.ExecContext(ctx, `
			UPDATE memories
			SET summary_json = ?, embedding = ?, embedding_model = ?,
			    importance = ?, emotional_boost = ?, evidence = ?,
			    emotional_intensity = ?, narrative_significance = ?,
			    place_wing = ?, place_room = ?
			WHERE id = ? AND chat_session_id = ?
		`, nullableString(mem.SummaryJSON), nullableString(mem.Embedding),
			nullableString(mem.EmbeddingModel), mem.Importance, mem.EmotionalBoost,
			nullableString(mem.Evidence), mem.EmotionalIntensity,
			mem.NarrativeSignificance, nullableString(mem.PlaceWing),
			nullableString(mem.PlaceRoom), id, mem.ChatSessionID)
		if err != nil {
			return 0, false, false, err
		}
		mem.ID = id
		return id, false, true, nil
	case sql.ErrNoRows:
		res, insertErr := tx.ExecContext(ctx, `
			INSERT INTO memories (
				chat_session_id, turn_index, summary_json, embedding, embedding_model,
				importance, emotional_boost, evidence, emotional_intensity,
				narrative_significance, place_wing, place_room, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, mem.ChatSessionID, mem.TurnIndex, nullableString(mem.SummaryJSON),
			nullableString(mem.Embedding), nullableString(mem.EmbeddingModel),
			mem.Importance, mem.EmotionalBoost, nullableString(mem.Evidence),
			mem.EmotionalIntensity, mem.NarrativeSignificance,
			nullableString(mem.PlaceWing), nullableString(mem.PlaceRoom),
			nonZeroTime(mem.CreatedAt))
		if insertErr != nil {
			return 0, false, false, insertErr
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, false, false, err
		}
		mem.ID = id
		return id, true, false, nil
	default:
		return 0, false, false, err
	}
}

type admissionReconcileResult struct {
	inserted         int
	reactivated      int
	retired          int
	vectorOperations int
}

type admissionEvidenceRow struct {
	id         int64
	text       string
	tombstoned bool
}

func reconcileAdmissionEvidenceTx(
	ctx context.Context,
	tx *sql.Tx,
	admission *MemoryAdmission,
) (map[string]int64, admissionReconcileResult, error) {
	result := admissionReconcileResult{}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, evidence_text, tombstoned
		FROM direct_evidence_records
		WHERE chat_session_id = ?
		  AND source_turn_start = ? AND source_turn_end = ?
		  AND capture_stage = 'critic_extract'
		ORDER BY id
		FOR UPDATE
	`, admission.ChatSessionID, admission.TurnIndex, admission.TurnIndex)
	if err != nil {
		return nil, result, err
	}
	existing := map[string]admissionEvidenceRow{}
	duplicates := []admissionEvidenceRow{}
	for rows.Next() {
		var item admissionEvidenceRow
		if err := rows.Scan(&item.id, &item.text, &item.tombstoned); err != nil {
			_ = rows.Close()
			return nil, result, err
		}
		key := strings.TrimSpace(item.text)
		if _, seen := existing[key]; seen {
			duplicates = append(duplicates, item)
		} else {
			existing[key] = item
		}
	}
	if err := rows.Close(); err != nil {
		return nil, result, err
	}

	desired := map[string]bool{}
	evidenceByText := map[string]int64{}
	for _, evidence := range admission.Evidence {
		if evidence == nil {
			continue
		}
		key := strings.TrimSpace(evidence.EvidenceText)
		if key == "" || desired[key] {
			continue
		}
		desired[key] = true
		if prior, ok := existing[key]; ok {
			evidence.ID = prior.id
			if _, err := tx.ExecContext(ctx, `
				UPDATE direct_evidence_records
				SET evidence_kind = ?, source_message_ids_json = ?, source_hash = ?,
				    archive_state = ?, capture_verification = ?, committed_gate = ?,
				    lineage_json = ?, repair_needed = FALSE, tombstoned = FALSE,
				    superseded_by_id = NULL
				WHERE id = ? AND chat_session_id = ?
			`, evidence.EvidenceKind, nullableString(evidence.SourceMessageIDsJSON),
				nullableString(evidence.SourceHash), evidence.ArchiveState,
				evidence.CaptureVerification, nullableString(evidence.CommittedGate),
				nullableString(evidence.LineageJSON), evidence.ID,
				admission.ChatSessionID); err != nil {
				return nil, result, err
			}
			if prior.tombstoned {
				result.reactivated++
			}
			evidenceByText[key] = evidence.ID
			continue
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO direct_evidence_records (
				chat_session_id, evidence_kind, evidence_text, source_turn_start,
				source_turn_end, turn_anchor, source_message_ids_json, source_hash,
				archive_state, capture_stage, capture_verification, committed_gate,
				lineage_json, repair_needed, tombstoned, superseded_by_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, evidence.ChatSessionID, evidence.EvidenceKind, evidence.EvidenceText,
			evidence.SourceTurnStart, evidence.SourceTurnEnd, evidence.TurnAnchor,
			nullableString(evidence.SourceMessageIDsJSON), nullableString(evidence.SourceHash),
			evidence.ArchiveState, evidence.CaptureStage,
			evidence.CaptureVerification, nullableString(evidence.CommittedGate),
			nullableString(evidence.LineageJSON), evidence.RepairNeeded,
			evidence.Tombstoned, evidence.SupersededByID,
			nonZeroTime(evidence.CreatedAt))
		if err != nil {
			return nil, result, err
		}
		evidence.ID, err = res.LastInsertId()
		if err != nil {
			return nil, result, err
		}
		evidenceByText[key] = evidence.ID
		result.inserted++
	}
	for key, prior := range existing {
		if desired[key] || prior.tombstoned {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE direct_evidence_records
			SET tombstoned = TRUE, repair_needed = FALSE
			WHERE id = ? AND chat_session_id = ?
		`, prior.id, admission.ChatSessionID); err != nil {
			return nil, result, err
		}
		queued, err := enqueueAdmissionVectorDeleteTx(
			ctx, tx, admission, "evidence:"+admission.ChatSessionID+":"+strconv.FormatInt(prior.id, 10),
			"active", "retired_evidence",
		)
		if err != nil {
			return nil, result, err
		}
		if queued {
			result.vectorOperations++
		}
		result.retired++
	}
	for _, duplicate := range duplicates {
		if duplicate.tombstoned {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE direct_evidence_records
			SET tombstoned = TRUE, repair_needed = FALSE
			WHERE id = ? AND chat_session_id = ?
		`, duplicate.id, admission.ChatSessionID); err != nil {
			return nil, result, err
		}
		queued, err := enqueueAdmissionVectorDeleteTx(
			ctx, tx, admission,
			"evidence:"+admission.ChatSessionID+":"+strconv.FormatInt(duplicate.id, 10),
			"active", "duplicate_evidence",
		)
		if err != nil {
			return nil, result, err
		}
		if queued {
			result.vectorOperations++
		}
		result.retired++
	}
	return evidenceByText, result, nil
}

type admissionPreciseRow struct {
	id             int64
	unitID         string
	idempotencyKey string
	lifecycle      string
}

func reconcileAdmissionPreciseMemoryTx(
	ctx context.Context,
	tx *sql.Tx,
	admission *MemoryAdmission,
) (admissionReconcileResult, error) {
	result := admissionReconcileResult{}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, unit_id, idempotency_key, lifecycle_state
		FROM precise_memory_units
		WHERE chat_session_id = ? AND source_revision = ?
		ORDER BY id
		FOR UPDATE
	`, admission.ChatSessionID, admission.SourceRevision)
	if err != nil {
		return result, err
	}
	existing := map[string]admissionPreciseRow{}
	for rows.Next() {
		var item admissionPreciseRow
		if err := rows.Scan(
			&item.id, &item.unitID, &item.idempotencyKey, &item.lifecycle,
		); err != nil {
			_ = rows.Close()
			return result, err
		}
		existing[item.unitID] = item
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	desired := map[string]bool{}
	for _, unit := range admission.PreciseUnits {
		if unit == nil || desired[unit.UnitID] {
			continue
		}
		desired[unit.UnitID] = true
		if prior, ok := existing[unit.UnitID]; ok {
			if prior.idempotencyKey != unit.IdempotencyKey {
				return result, fmt.Errorf("precise memory idempotency conflict")
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE precise_memory_units
				SET root_evidence_id = ?, direct_evidence_ids_json = ?,
				    lifecycle_state = 'active', updated_at = ?
				WHERE id = ? AND source_revision = ? AND idempotency_key = ?
			`, unit.RootEvidenceID, unit.DirectEvidenceIDsJSON,
				nonZeroTime(unit.UpdatedAt), prior.id, unit.SourceRevision,
				unit.IdempotencyKey); err != nil {
				return result, err
			}
			unit.ID = prior.id
			if _, err := tx.ExecContext(ctx, `
				UPDATE memory_derivation_dependencies
				SET lifecycle_state = 'invalidated', invalidated_at = ?, updated_at = ?
				WHERE source_revision = ?
				  AND child_artifact_type = 'precise_memory_unit'
				  AND child_artifact_id = ?
				  AND lifecycle_state = 'active'
				  AND (
				    derivation_version <> ? OR extractor_version <> ?
				    OR index_version <> ?
				  )
			`, nonZeroTime(admission.CreatedAt), nonZeroTime(admission.CreatedAt),
				unit.SourceRevision, unit.UnitID, unit.DerivationVersion,
				unit.ExtractorVersion, unit.IndexVersion); err != nil {
				return result, err
			}
			if err := savePreciseMemoryDependenciesTx(ctx, tx, unit); err != nil {
				return result, err
			}
			vectorQueued, err := enqueuePreciseMemoryVectorTx(ctx, tx, unit)
			if err != nil {
				return result, err
			}
			if vectorQueued {
				result.vectorOperations++
			}
			if prior.lifecycle != "active" {
				result.reactivated++
			}
			continue
		}
		inserted, err := savePreciseMemoryUnitTx(ctx, tx, unit, true)
		if err != nil {
			return result, err
		}
		if inserted {
			result.inserted++
			result.vectorOperations++
		}
	}
	for unitID, prior := range existing {
		if desired[unitID] || prior.lifecycle != "active" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE precise_memory_units
			SET lifecycle_state = 'invalidated', updated_at = ?
			WHERE id = ? AND source_revision = ?
		`, nonZeroTime(admission.CreatedAt), prior.id, admission.SourceRevision); err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_derivation_dependencies
			SET lifecycle_state = 'invalidated', invalidated_at = ?, updated_at = ?
			WHERE source_revision = ? AND child_artifact_type = 'precise_memory_unit'
			  AND child_artifact_id = ? AND lifecycle_state = 'active'
		`, nonZeroTime(admission.CreatedAt), nonZeroTime(admission.CreatedAt),
			admission.SourceRevision, unitID); err != nil {
			return result, err
		}
		documentID := "precise_memory:" + admission.ChatSessionID + ":" + unitID
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_vector_outbox
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    last_error = 'precise_unit_replaced', updated_at = ?
			WHERE document_id = ? AND operation = 'upsert'
			  AND status IN ('pending', 'leased', 'retryable', 'needs_embedding')
		`, nonZeroTime(admission.CreatedAt), documentID); err != nil {
			return result, err
		}
		queued, err := enqueueAdmissionVectorDeleteTx(
			ctx, tx, admission, documentID, "active", "retired_precise_memory",
		)
		if err != nil {
			return result, err
		}
		if queued {
			result.vectorOperations++
		}
		result.retired++
	}
	return result, nil
}

func enqueueAdmissionVectorsTx(
	ctx context.Context,
	tx *sql.Tx,
	admission *MemoryAdmission,
	memoryID int64,
	evidenceByText map[string]int64,
) (int, error) {
	queued := 0
	for _, item := range admission.Vectors {
		sourceRowID := int64(0)
		switch item.ArtifactType {
		case "memory":
			sourceRowID = memoryID
		case "evidence":
			sourceRowID = evidenceByText[strings.TrimSpace(item.EvidenceText)]
		default:
			return queued, fmt.Errorf("unsupported admission vector artifact %q", item.ArtifactType)
		}
		if sourceRowID <= 0 {
			return queued, fmt.Errorf("admission vector source row is missing")
		}
		rowID := strconv.FormatInt(sourceRowID, 10)
		documentID := item.ArtifactType + ":" + admission.ChatSessionID + ":" + rowID
		documentJSON, err := json.Marshal(map[string]any{
			"ID":                    documentID,
			"Embedding":             item.Embedding,
			"Tier":                  item.Tier,
			"ChatSessionID":         admission.ChatSessionID,
			"SourceTable":           item.SourceTable,
			"SourceRowID":           rowID,
			"SchemaVersion":         item.SchemaVersion,
			"DocumentText":          strings.TrimSpace(item.DocumentText),
			"SearchTextPolicy":      item.SearchTextPolicy,
			"RawLanguage":           item.RawLanguage,
			"SummaryLanguage":       item.SummaryLanguage,
			"SessionOutputLanguage": item.SessionOutputLanguage,
			"AliasCount":            item.AliasCount,
		})
		if err != nil {
			return queued, err
		}
		embeddingReady := len(item.Embedding) > 0
		status := "needs_embedding"
		if embeddingReady {
			status = "pending"
		}
		outbox := &MemoryVectorOutboxItem{
			ContractVersion:     MemoryVectorOutboxContract,
			OperationKey:        memoryAdmissionVectorOperationKey("upsert", admission, documentID),
			Operation:           "upsert",
			ChatSessionID:       admission.ChatSessionID,
			SourceRevision:      admission.SourceRevision,
			DocumentID:          documentID,
			DocumentJSON:        string(documentJSON),
			EmbeddingReady:      embeddingReady,
			RequiredSourceState: "active",
			Status:              status,
			CreatedAt:           nonZeroTime(admission.CreatedAt),
			UpdatedAt:           nonZeroTime(admission.CreatedAt),
		}
		inserted, err := enqueueMemoryVectorOperation(ctx, tx, outbox)
		if err != nil {
			return queued, err
		}
		if inserted {
			queued++
		}
	}
	return queued, nil
}

func enqueueAdmissionVectorDeleteTx(
	ctx context.Context,
	tx *sql.Tx,
	admission *MemoryAdmission,
	documentID string,
	requiredSourceState string,
	reason string,
) (bool, error) {
	item := &MemoryVectorOutboxItem{
		ContractVersion:     MemoryVectorOutboxContract,
		OperationKey:        memoryAdmissionVectorOperationKey("delete:"+reason, admission, documentID),
		Operation:           "delete",
		ChatSessionID:       admission.ChatSessionID,
		SourceRevision:      admission.SourceRevision,
		DocumentID:          documentID,
		EmbeddingReady:      true,
		RequiredSourceState: requiredSourceState,
		Status:              "pending",
		CreatedAt:           nonZeroTime(admission.CreatedAt),
		UpdatedAt:           nonZeroTime(admission.CreatedAt),
	}
	return enqueueMemoryVectorOperation(ctx, tx, item)
}

func memoryAdmissionVectorOperationKey(operation string, admission *MemoryAdmission, documentID string) string {
	return memoryVectorOperationKey(
		operation,
		admission.ChatSessionID,
		admission.SourceRevision+":"+admission.ResultHash,
		documentID,
	)
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}
