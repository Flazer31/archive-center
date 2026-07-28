package store

import "context"

func (m *mariadbStore) SaveEntityIdentity(ctx context.Context, item *EntityIdentity) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO entity_identities (
			stable_entity_id, chat_session_id, identity_namespace, entity_kind,
			canonical_label, lifecycle_state, review_state, presence_authority,
			occurrence_authority, source_contract, source_revision,
			source_logical_turn_id, source_message_id, source_generation_id,
			source_content_hash, source_turn, source_index, idempotency_key,
			mapping_revision, first_seen_turn, last_seen_turn, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			last_seen_turn = GREATEST(last_seen_turn, VALUES(last_seen_turn)),
			updated_at = VALUES(updated_at)
	`, item.StableEntityID, item.ChatSessionID, item.IdentityNamespace, item.EntityKind,
		item.CanonicalLabel, item.LifecycleState, item.ReviewState, item.PresenceAuthority,
		item.OccurrenceAuthority, item.SourceContract, item.SourceRevision,
		nullableString(item.SourceLogicalTurnID), nullableString(item.SourceMessageID),
		nullableString(item.SourceGenerationID), item.SourceContentHash, item.SourceTurn,
		item.SourceIndex, item.IdempotencyKey, item.MappingRevision, item.FirstSeenTurn,
		item.LastSeenTurn, nonZeroTime(item.CreatedAt), nonZeroTime(item.UpdatedAt))
	return err
}

func (m *mariadbStore) SaveEntityIdentitySurface(ctx context.Context, item *EntityIdentitySurface) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO entity_identity_surfaces (
			surface_id, stable_entity_id, chat_session_id, identity_namespace,
			surface_kind, surface_text, normalized_surface, surface_scope,
			valid_from_turn, valid_to_turn, source_contract, source_revision,
			source_turn, source_span_start, source_span_end, evidence_excerpt,
			review_state, idempotency_key, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)
	`, item.SurfaceID, item.StableEntityID, item.ChatSessionID, item.IdentityNamespace,
		item.SurfaceKind, item.SurfaceText, item.NormalizedSurface, item.Scope,
		item.ValidFromTurn, nullableEntityIdentityPositiveInt(item.ValidToTurn), item.SourceContract,
		item.SourceRevision, item.SourceTurn, nullableNonNegativeInt(item.SourceSpanStart),
		nullableNonNegativeInt(item.SourceSpanEnd), nullableString(item.EvidenceExcerpt),
		item.ReviewState, item.IdempotencyKey, nonZeroTime(item.CreatedAt),
		nonZeroTime(item.UpdatedAt))
	return err
}

func (m *mariadbStore) SaveEntityIdentityArtifactBinding(ctx context.Context, item *EntityIdentityArtifactBinding) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO entity_identity_artifact_bindings (
			binding_id, stable_entity_id, chat_session_id, artifact_kind,
			artifact_role, artifact_ordinal, surface_text, review_state,
			source_contract, source_revision, source_turn, idempotency_key, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE review_state = VALUES(review_state)
	`, item.BindingID, item.StableEntityID, item.ChatSessionID, item.ArtifactKind,
		item.ArtifactRole, item.ArtifactOrdinal, item.SurfaceText, item.ReviewState,
		item.SourceContract, item.SourceRevision, item.SourceTurn, item.IdempotencyKey,
		nonZeroTime(item.CreatedAt))
	return err
}

func (m *mariadbStore) SaveSpeakerAttribution(ctx context.Context, item *SpeakerAttribution) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO speaker_attributions (
			attribution_id, chat_session_id, speaker_entity_id, identity_namespace,
			source_role, attribution_kind, attribution_state, review_state,
			confidence, source_contract, source_revision, source_logical_turn_id,
			source_message_id, source_generation_id, source_content_hash, source_turn,
			source_span_start, source_span_end, evidence_excerpt, idempotency_key,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)
	`, item.AttributionID, item.ChatSessionID, item.SpeakerEntityID,
		item.IdentityNamespace, item.SourceRole, item.AttributionKind,
		item.AttributionState, item.ReviewState, item.Confidence, item.SourceContract,
		item.SourceRevision, nullableString(item.SourceLogicalTurn),
		nullableString(item.SourceMessageID), nullableString(item.SourceGeneration),
		item.SourceContentHash, item.SourceTurn, item.SourceSpanStart, item.SourceSpanEnd,
		item.EvidenceExcerpt, item.IdempotencyKey, nonZeroTime(item.CreatedAt),
		nonZeroTime(item.UpdatedAt))
	return err
}

func nullableEntityIdentityPositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableNonNegativeInt(value int) any {
	if value < 0 {
		return nil
	}
	return value
}
