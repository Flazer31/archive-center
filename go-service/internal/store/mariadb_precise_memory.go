package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var _ PreciseMemoryWriter = (*mariadbStore)(nil)
var _ PreciseMemoryWriteAvailability = (*mariadbStore)(nil)
var _ CharacterPerspectiveMemoryReader = (*mariadbStore)(nil)
var _ ActiveInteractionMemoryReader = (*mariadbStore)(nil)
var _ GeneralVectorPreciseMemoryReader = (*mariadbStore)(nil)

func (m *mariadbStore) PreciseMemoryWritesEnabled() bool {
	return m != nil && m.db != nil
}

func (m *mariadbStore) SavePreciseMemoryUnit(ctx context.Context, item *PreciseMemoryUnit) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	if item == nil || strings.TrimSpace(item.SourceRevision) == "" {
		return false, fmt.Errorf("precise memory source revision is required")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	inserted, err := savePreciseMemoryUnitTx(ctx, tx, item, false)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	committed = true
	return inserted, nil
}

func (m *mariadbStore) ListCharacterPerspectiveMemoryUnits(ctx context.Context, chatSessionID, knowledgeHolderEntityID string) ([]PreciseMemoryUnit, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	knowledgeHolderEntityID = strings.TrimSpace(knowledgeHolderEntityID)
	if chatSessionID == "" || knowledgeHolderEntityID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			unit.unit_id, unit.chat_session_id, unit.source_turn_start,
			unit.source_turn_end, unit.source_revision, unit.memory_kind,
			COALESCE(unit.memory_subtype, ''), unit.payload_json,
			COALESCE(unit.actor_entity_id, ''),
			COALESCE(unit.subject_entity_id, ''),
			unit.truth_scope, unit.epistemic_mode, unit.authority_class,
			unit.admission_state, unit.review_state, unit.visibility,
			COALESCE(unit.knowledge_holder_entity_id, ''),
			COALESCE(unit.reveal_condition, ''), unit.lifecycle_state,
			unit.created_at, unit.updated_at
		FROM precise_memory_units unit
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = unit.chat_session_id
		 AND source_revision.source_revision = unit.source_revision
		 AND source_revision.lifecycle_state = 'active'
		WHERE unit.chat_session_id = ?
		  AND unit.memory_kind = 'observation'
		  AND unit.knowledge_holder_entity_id = ?
		  AND unit.epistemic_mode IN ('known', 'suspected', 'unknown', 'misinformed', 'hidden', 'revealed')
		  AND unit.lifecycle_state = 'active'
		ORDER BY unit.source_turn_start ASC, unit.unit_id ASC
	`, chatSessionID, knowledgeHolderEntityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PreciseMemoryUnit{}
	for rows.Next() {
		var item PreciseMemoryUnit
		if err := rows.Scan(
			&item.UnitID, &item.ChatSessionID, &item.SourceTurnStart,
			&item.SourceTurnEnd, &item.SourceRevision, &item.Kind,
			&item.Subtype, &item.PayloadJSON, &item.ActorEntityID,
			&item.SubjectEntityID, &item.TruthScope, &item.EpistemicMode,
			&item.AuthorityClass, &item.AdmissionState, &item.ReviewState,
			&item.Visibility, &item.KnowledgeHolderEntityID,
			&item.RevealCondition, &item.LifecycleState, &item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *mariadbStore) ListActiveInteractionMemoryUnits(ctx context.Context, chatSessionID string) ([]PreciseMemoryUnit, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			unit.unit_id, unit.chat_session_id, unit.source_turn_start,
			unit.source_turn_end, unit.source_revision, unit.memory_kind,
			COALESCE(unit.memory_subtype, ''), unit.payload_json,
			COALESCE(unit.actor_entity_id, ''),
			COALESCE(unit.subject_entity_id, ''),
			COALESCE(unit.affected_entity_id, ''),
			COALESCE(unit.object_entity_id, ''),
			COALESCE(unit.relationship_key, ''),
			unit.truth_scope, unit.epistemic_mode, unit.authority_class,
			unit.admission_state, unit.review_state, unit.visibility,
			COALESCE(unit.knowledge_holder_entity_id, ''),
			COALESCE(unit.reveal_condition, ''), unit.lifecycle_state,
			unit.created_at, unit.updated_at
		FROM precise_memory_units unit
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = unit.chat_session_id
		 AND source_revision.source_revision = unit.source_revision
		 AND source_revision.lifecycle_state = 'active'
		WHERE unit.chat_session_id = ?
		  AND unit.memory_kind IN ('observation', 'boundary')
		  AND unit.lifecycle_state = 'active'
		ORDER BY unit.source_turn_start ASC, unit.unit_id ASC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PreciseMemoryUnit{}
	for rows.Next() {
		var item PreciseMemoryUnit
		if err := rows.Scan(
			&item.UnitID, &item.ChatSessionID, &item.SourceTurnStart,
			&item.SourceTurnEnd, &item.SourceRevision, &item.Kind,
			&item.Subtype, &item.PayloadJSON, &item.ActorEntityID,
			&item.SubjectEntityID, &item.AffectedEntityID, &item.ObjectEntityID,
			&item.RelationshipKey, &item.TruthScope, &item.EpistemicMode,
			&item.AuthorityClass, &item.AdmissionState, &item.ReviewState,
			&item.Visibility, &item.KnowledgeHolderEntityID,
			&item.RevealCondition, &item.LifecycleState, &item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *mariadbStore) ListGeneralVectorPreciseMemoryUnits(ctx context.Context, chatSessionID string) ([]PreciseMemoryUnit, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			unit.id, unit.unit_id, unit.chat_session_id,
			unit.admission_state, unit.review_state, unit.visibility,
			COALESCE(unit.knowledge_holder_entity_id, ''),
			unit.epistemic_mode, unit.lifecycle_state
		FROM precise_memory_units unit
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = unit.chat_session_id
		 AND source_revision.source_revision = unit.source_revision
		 AND source_revision.lifecycle_state = 'active'
		WHERE unit.chat_session_id = ?
		  AND unit.lifecycle_state = 'active'
		ORDER BY unit.id ASC
	`, chatSessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PreciseMemoryUnit{}
	for rows.Next() {
		var item PreciseMemoryUnit
		if err := rows.Scan(
			&item.ID, &item.UnitID, &item.ChatSessionID,
			&item.AdmissionState, &item.ReviewState, &item.Visibility,
			&item.KnowledgeHolderEntityID, &item.EpistemicMode,
			&item.LifecycleState,
		); err != nil {
			return nil, err
		}
		if PreciseMemoryGeneralVectorEligible(&item) {
			out = append(out, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func savePreciseMemoryUnitTx(ctx context.Context, tx *sql.Tx, item *PreciseMemoryUnit, sourceFenceLocked bool) (bool, error) {
	if item == nil || strings.TrimSpace(item.SourceRevision) == "" {
		return false, fmt.Errorf("precise memory source revision is required")
	}
	var lifecycle string
	if !sourceFenceLocked {
		if err := tx.QueryRowContext(ctx, `
			SELECT lifecycle_state
			FROM memory_source_revisions
			WHERE chat_session_id = ? AND source_revision = ?
			FOR UPDATE
		`, item.ChatSessionID, item.SourceRevision).Scan(&lifecycle); err != nil {
			if err == sql.ErrNoRows {
				return false, ErrSourceRevisionStale
			}
			return false, err
		}
		if lifecycle != "active" {
			return false, ErrSourceRevisionStale
		}
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO precise_memory_units (
			unit_id, contract_version, chat_session_id, source_turn_start,
			source_turn_end, source_contract, source_revision,
			source_logical_turn_id, source_message_id, source_generation_id,
			source_content_hash, source_role, source_span_start, source_span_end,
			evidence_excerpt, evidence_hash, root_evidence_id,
			direct_evidence_ids_json, memory_kind, memory_subtype, payload_json,
			actor_entity_id, subject_entity_id, affected_entity_id,
			location_entity_id, object_entity_id, relationship_key, truth_scope,
			epistemic_mode, authority_class, admission_state, review_state,
			visibility, knowledge_holder_entity_id, reveal_condition, confidence,
			idempotency_key, lifecycle_state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		          ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.UnitID, item.ContractVersion, item.ChatSessionID,
		item.SourceTurnStart, item.SourceTurnEnd, item.SourceContract,
		item.SourceRevision, nullableString(item.SourceLogicalTurnID),
		nullableString(item.SourceMessageID), nullableString(item.SourceGenerationID),
		item.SourceContentHash, item.SourceRole, item.SourceSpanStart,
		item.SourceSpanEnd, item.EvidenceExcerpt, item.EvidenceHash,
		nullablePositiveInt64(item.RootEvidenceID), item.DirectEvidenceIDsJSON, item.Kind,
		nullableString(item.Subtype), item.PayloadJSON,
		nullableString(item.ActorEntityID), nullableString(item.SubjectEntityID),
		nullableString(item.AffectedEntityID), nullableString(item.LocationEntityID),
		nullableString(item.ObjectEntityID), nullableString(item.RelationshipKey),
		item.TruthScope, item.EpistemicMode, item.AuthorityClass,
		item.AdmissionState, item.ReviewState, item.Visibility,
		nullableString(item.KnowledgeHolderEntityID),
		nullableString(item.RevealCondition), item.Confidence,
		item.IdempotencyKey, item.LifecycleState, nonZeroTime(item.CreatedAt),
		nonZeroTime(item.UpdatedAt))
	if preciseMemoryDuplicateKeyError(err) {
		var existingUnitID, existingRevision, existingKey string
		if queryErr := tx.QueryRowContext(ctx, `
			SELECT unit_id, source_revision, idempotency_key
			FROM precise_memory_units
			WHERE unit_id = ? OR idempotency_key = ?
			ORDER BY unit_id = ? DESC
			LIMIT 1
		`, item.UnitID, item.IdempotencyKey, item.UnitID).Scan(
			&existingUnitID, &existingRevision, &existingKey,
		); queryErr != nil {
			return false, queryErr
		}
		if existingUnitID != item.UnitID ||
			existingRevision != item.SourceRevision ||
			existingKey != item.IdempotencyKey {
			return false, fmt.Errorf("precise memory idempotency conflict")
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	if err := savePreciseMemoryDependenciesTx(ctx, tx, item); err != nil {
		return false, err
	}
	if _, err := enqueuePreciseMemoryVectorTx(ctx, tx, item); err != nil {
		return false, err
	}
	return true, nil
}

func savePreciseMemoryDependenciesTx(ctx context.Context, tx *sql.Tx, item *PreciseMemoryUnit) error {
	derivationVersion := strings.TrimSpace(item.DerivationVersion)
	if derivationVersion == "" {
		derivationVersion = PreciseMemoryUnitContract
	}
	extractorVersion := strings.TrimSpace(item.ExtractorVersion)
	if extractorVersion == "" {
		extractorVersion = "complete_turn.configured_critic_extract"
	}
	indexVersion := strings.TrimSpace(item.IndexVersion)
	if indexVersion == "" {
		indexVersion = "not_materialized"
	}
	parents := []struct {
		kind string
		id   string
	}{{kind: "source_revision", id: item.SourceRevision}}
	if item.RootEvidenceID > 0 {
		parents = append(parents, struct {
			kind string
			id   string
		}{kind: "direct_evidence", id: strconv.FormatInt(item.RootEvidenceID, 10)})
	}
	var evidenceIDs []int64
	_ = json.Unmarshal([]byte(item.DirectEvidenceIDsJSON), &evidenceIDs)
	for _, evidenceID := range evidenceIDs {
		if evidenceID <= 0 || evidenceID == item.RootEvidenceID {
			continue
		}
		parents = append(parents, struct {
			kind string
			id   string
		}{kind: "direct_evidence", id: strconv.FormatInt(evidenceID, 10)})
	}
	for _, parent := range parents {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO memory_derivation_dependencies (
				contract_version, chat_session_id, source_revision,
				root_source_pointer, child_artifact_type, child_artifact_id,
				parent_artifact_type, parent_artifact_id, derivation_version,
				extractor_version, index_version, lifecycle_state, created_at,
				updated_at
			) VALUES (?, ?, ?, ?, 'precise_memory_unit', ?, ?, ?, ?, ?, ?, 'active', ?, ?)
		`, MemoryDerivationDependencyContract, item.ChatSessionID,
			item.SourceRevision, "source_revision:"+item.SourceRevision, item.UnitID,
			parent.kind, parent.id, derivationVersion, extractorVersion,
			indexVersion, nonZeroTime(item.CreatedAt), nonZeroTime(item.UpdatedAt)); err != nil {
			if !preciseMemoryDuplicateKeyError(err) {
				return err
			}
			var count int
			if queryErr := tx.QueryRowContext(ctx, `
				SELECT COUNT(*)
				FROM memory_derivation_dependencies
				WHERE source_revision = ? AND child_artifact_type = 'precise_memory_unit'
				  AND child_artifact_id = ? AND parent_artifact_type = ?
				  AND parent_artifact_id = ? AND derivation_version = ?
				  AND extractor_version = ? AND index_version = ?
			`, item.SourceRevision, item.UnitID, parent.kind, parent.id,
				derivationVersion, extractorVersion, indexVersion).Scan(&count); queryErr != nil {
				return queryErr
			}
			if count != 1 {
				return fmt.Errorf("memory derivation dependency idempotency conflict")
			}
		}
	}
	return nil
}

func enqueuePreciseMemoryVectorTx(ctx context.Context, tx *sql.Tx, item *PreciseMemoryUnit) (bool, error) {
	if !preciseMemoryGeneralVectorEligible(item) {
		return false, nil
	}
	documentID := "precise_memory:" + item.ChatSessionID + ":" + item.UnitID
	documentText := PreciseMemorySemanticText(item)
	if documentText == "" {
		return false, nil
	}
	documentJSON, err := json.Marshal(map[string]any{
		"ID":            documentID,
		"ChatSessionID": item.ChatSessionID,
		"SourceTable":   "precise_memory_units",
		"SourceRowID":   item.UnitID,
		"SchemaVersion": PreciseMemoryUnitContract,
		"DocumentText":  documentText,
		"Embedding":     item.VectorEmbedding,
		"Metadata": memoryVectorDocumentMetadata(
			item.SourceRevision, item.SourceContract, item.IndexVersion, documentText,
			item.VectorEmbeddingModel, item.VectorContextChunks, item.VectorContextChunkIndex,
		),
	})
	if err != nil {
		return false, err
	}
	outbox := &MemoryVectorOutboxItem{
		ContractVersion: MemoryVectorOutboxContract,
		OperationKey: preciseMemoryVectorOperationKey(
			item.ChatSessionID, item.SourceRevision, item.DerivationVersion,
			item.ExtractorVersion, item.IndexVersion, documentID,
		),
		Operation:           "upsert",
		ChatSessionID:       item.ChatSessionID,
		SourceRevision:      item.SourceRevision,
		DocumentID:          documentID,
		DocumentJSON:        string(documentJSON),
		EmbeddingReady:      len(item.VectorEmbedding) > 0,
		RequiredSourceState: "active",
		Status:              preciseMemoryVectorOutboxStatus(item),
		CreatedAt:           nonZeroTime(item.CreatedAt),
		UpdatedAt:           nonZeroTime(item.UpdatedAt),
	}
	return enqueueAdmissionVectorOperation(ctx, tx, outbox)
}

func preciseMemoryVectorOutboxStatus(item *PreciseMemoryUnit) string {
	if item != nil && len(item.VectorEmbedding) > 0 {
		return "pending"
	}
	return "needs_embedding"
}

func PreciseMemoryGeneralVectorEligible(item *PreciseMemoryUnit) bool {
	if item == nil {
		return false
	}
	if strings.TrimSpace(item.KnowledgeHolderEntityID) != "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(item.Visibility)) {
	case "owner_private", "restricted", "reveal_required", "private", "hidden", "user_private":
		return false
	}
	switch strings.ToLower(strings.TrimSpace(item.EpistemicMode)) {
	case "known", "suspected", "unknown", "misinformed", "hidden", "revealed":
		return false
	}
	return true
}

func preciseMemoryGeneralVectorEligible(item *PreciseMemoryUnit) bool {
	return PreciseMemoryGeneralVectorEligible(item)
}

func preciseMemoryVectorOperationKey(
	sid string,
	sourceRevision string,
	derivationVersion string,
	extractorVersion string,
	indexVersion string,
	documentID string,
) string {
	versionedRevision := strings.Join([]string{
		sourceRevision,
		firstNonEmptyString(derivationVersion, PreciseMemoryUnitContract),
		firstNonEmptyString(extractorVersion, "complete_turn.configured_critic_extract"),
		firstNonEmptyString(indexVersion, "not_materialized"),
	}, ":")
	return memoryVectorOperationKey("upsert", sid, versionedRevision, documentID)
}

func preciseMemoryDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
