package store

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
)

var _ PreciseMemoryWriter = (*mariadbStore)(nil)
var _ PreciseMemoryWriteAvailability = (*mariadbStore)(nil)

func (m *mariadbStore) PreciseMemoryWritesEnabled() bool {
	return m != nil && m.db != nil
}

func (m *mariadbStore) SavePreciseMemoryUnit(ctx context.Context, item *PreciseMemoryUnit) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	res, err := m.db.ExecContext(ctx, `
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
		item.RootEvidenceID, item.DirectEvidenceIDsJSON, item.Kind,
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
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		item.ID = id
	}
	return true, nil
}

func preciseMemoryDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
