package store

import (
	"context"
	"database/sql"
	"strings"
)

const acceptedSourceObservationContract = "source_acceptance_observation.v1"

func (m *mariadbStore) SaveEntityIdentity(ctx context.Context, item *EntityIdentity) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	return m.withActiveEntitySourceWrite(ctx, item.SourceContract, item.ChatSessionID, item.SourceRevision, func(exec memoryDerivationSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `
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
	})
}

func (m *mariadbStore) SaveEntityIdentitySurface(ctx context.Context, item *EntityIdentitySurface) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	return m.withActiveEntitySourceWrite(ctx, item.SourceContract, item.ChatSessionID, item.SourceRevision, func(exec memoryDerivationSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `
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
	})
}

func (m *mariadbStore) SaveEntityIdentityArtifactBinding(ctx context.Context, item *EntityIdentityArtifactBinding) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	return m.withActiveEntitySourceWrite(ctx, item.SourceContract, item.ChatSessionID, item.SourceRevision, func(exec memoryDerivationSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `
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
	})
}

func (m *mariadbStore) SaveSpeakerAttribution(ctx context.Context, item *SpeakerAttribution) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	return m.withActiveEntitySourceWrite(ctx, item.SourceContract, item.ChatSessionID, item.SourceRevision, func(exec memoryDerivationSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `
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
	})
}

func (m *mariadbStore) ResolveReviewedCanonicalEntityID(ctx context.Context, chatSessionID, sourceEntityID string) (string, error) {
	if err := m.ensureDB(); err != nil {
		return "", err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	sourceEntityID = strings.TrimSpace(sourceEntityID)
	if chatSessionID == "" || sourceEntityID == "" {
		return "", ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT DISTINCT identity_link.target_entity_id
		FROM entity_identity_links identity_link
		JOIN entity_identities canonical_target
		  ON canonical_target.stable_entity_id = identity_link.target_entity_id
		 AND canonical_target.chat_session_id = identity_link.chat_session_id
		WHERE identity_link.chat_session_id = ?
		  AND identity_link.source_entity_id = ?
		  AND identity_link.target_entity_id <> identity_link.source_entity_id
		  AND identity_link.link_kind = ?
		  AND identity_link.link_state = ?
		  AND canonical_target.lifecycle_state = 'active'
		  AND canonical_target.review_state = ?
		ORDER BY identity_link.target_entity_id ASC
	`, chatSessionID, sourceEntityID, EntityIdentityLinkKindCanonicalEquivalence,
		EntityIdentityLinkStateReviewed, EntityIdentityReviewStateReviewed)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	targets := map[string]struct{}{}
	for rows.Next() {
		var targetEntityID string
		if err := rows.Scan(&targetEntityID); err != nil {
			return "", err
		}
		targetEntityID = strings.TrimSpace(targetEntityID)
		if targetEntityID != "" {
			targets[targetEntityID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return "", ErrNotFound
	}
	if len(targets) != 1 {
		return "", ErrReviewedEntityIdentityAmbiguous
	}
	for targetEntityID := range targets {
		return targetEntityID, nil
	}
	return "", ErrNotFound
}

func (m *mariadbStore) ResolveUniqueActiveEntityIDBySurface(ctx context.Context, chatSessionID, normalizedSurface string) (string, error) {
	if err := m.ensureDB(); err != nil {
		return "", err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	normalizedSurface = strings.TrimSpace(normalizedSurface)
	if chatSessionID == "" || normalizedSurface == "" {
		return "", ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			surface.stable_entity_id,
			COALESCE(identity_link.target_entity_id, '')
		FROM entity_identity_surfaces surface
		JOIN entity_identities source_identity
		  ON source_identity.chat_session_id = surface.chat_session_id
		 AND source_identity.stable_entity_id = surface.stable_entity_id
		 AND source_identity.lifecycle_state = 'active'
		 AND source_identity.review_state = 'source_observed'
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = surface.chat_session_id
		 AND source_revision.source_revision = surface.source_revision
		 AND source_revision.lifecycle_state = 'active'
		LEFT JOIN entity_identity_links identity_link
		  ON identity_link.chat_session_id = surface.chat_session_id
		 AND identity_link.source_entity_id = surface.stable_entity_id
		 AND identity_link.target_entity_id <> identity_link.source_entity_id
		 AND identity_link.link_kind = ?
		 AND identity_link.link_state = ?
		LEFT JOIN entity_identities canonical_target
		  ON canonical_target.chat_session_id = identity_link.chat_session_id
		 AND canonical_target.stable_entity_id = identity_link.target_entity_id
		 AND canonical_target.lifecycle_state = 'active'
		 AND canonical_target.review_state = ?
		WHERE surface.chat_session_id = ?
		  AND surface.normalized_surface = ?
		  AND surface.review_state = 'source_observed'
		  AND (identity_link.target_entity_id IS NULL OR canonical_target.stable_entity_id IS NOT NULL)
		ORDER BY surface.stable_entity_id ASC, identity_link.target_entity_id ASC
	`, EntityIdentityLinkKindCanonicalEquivalence, EntityIdentityLinkStateReviewed,
		EntityIdentityReviewStateReviewed, chatSessionID, normalizedSurface)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	resolved := map[string]struct{}{}
	for rows.Next() {
		var sourceEntityID, targetEntityID string
		if err := rows.Scan(&sourceEntityID, &targetEntityID); err != nil {
			return "", err
		}
		entityID := strings.TrimSpace(targetEntityID)
		if entityID == "" {
			entityID = strings.TrimSpace(sourceEntityID)
		}
		if entityID != "" {
			resolved[entityID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(resolved) == 0 {
		return "", ErrNotFound
	}
	if len(resolved) != 1 {
		return "", ErrReviewedEntityIdentityAmbiguous
	}
	for entityID := range resolved {
		return entityID, nil
	}
	return "", ErrNotFound
}

func (m *mariadbStore) withActiveEntitySourceWrite(
	ctx context.Context,
	sourceContract string,
	chatSessionID string,
	sourceRevision string,
	write func(memoryDerivationSQLExecutor) error,
) error {
	if strings.TrimSpace(sourceContract) != acceptedSourceObservationContract {
		return write(m.db)
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	sourceRevision = strings.TrimSpace(sourceRevision)
	if chatSessionID == "" || sourceRevision == "" {
		return ErrSourceRevisionStale
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
	var lifecycle string
	err = tx.QueryRowContext(ctx, `
		SELECT lifecycle_state
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ?
		FOR UPDATE
	`, chatSessionID, sourceRevision).Scan(&lifecycle)
	if err == sql.ErrNoRows || strings.TrimSpace(lifecycle) != "active" {
		return ErrSourceRevisionStale
	}
	if err != nil {
		return err
	}
	if err := write(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
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
