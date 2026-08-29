package store

import (
	"context"
	"database/sql"
	"errors"
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

func (m *mariadbStore) SaveEntityIdentityLink(ctx context.Context, item *EntityIdentityLink) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if item == nil || strings.TrimSpace(item.ChatSessionID) == "" ||
		strings.TrimSpace(item.SourceEntityID) == "" || strings.TrimSpace(item.TargetEntityID) == "" ||
		item.SourceEntityID == item.TargetEntityID {
		return ErrNotFound
	}
	return m.withActiveEntitySourceWrite(ctx, item.SourceContract, item.ChatSessionID, item.SourceRevision, func(exec memoryDerivationSQLExecutor) error {
		_, err := exec.ExecContext(ctx, `
		INSERT INTO entity_identity_links (
			link_id, chat_session_id, source_entity_id, target_entity_id,
			link_kind, link_state, evidence_json, mapping_revision,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			link_state = VALUES(link_state),
			evidence_json = VALUES(evidence_json),
			mapping_revision = GREATEST(mapping_revision, VALUES(mapping_revision)),
			updated_at = VALUES(updated_at)
	`, item.LinkID, item.ChatSessionID, item.SourceEntityID, item.TargetEntityID,
			item.LinkKind, item.LinkState, item.EvidenceJSON, item.MappingRevision,
			nonZeroTime(item.CreatedAt), nonZeroTime(item.UpdatedAt))
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
	current := sourceEntityID
	visited := map[string]bool{}
	resolvedAny := false
	for {
		if visited[current] {
			return "", ErrReviewedEntityIdentityCycle
		}
		visited[current] = true
		next, err := m.resolveReviewedCanonicalEntityIDOneHop(ctx, chatSessionID, current)
		if errors.Is(err, ErrNotFound) {
			if resolvedAny {
				return current, nil
			}
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		resolvedAny = true
		current = next
	}
}

func (m *mariadbStore) resolveReviewedCanonicalEntityIDOneHop(ctx context.Context, chatSessionID, sourceEntityID string) (string, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT DISTINCT identity_link.target_entity_id
		FROM entity_identity_links identity_link
		JOIN entity_identities canonical_target
		  ON canonical_target.stable_entity_id = identity_link.target_entity_id
		 AND canonical_target.chat_session_id = identity_link.chat_session_id
		JOIN memory_source_revisions canonical_revision
		  ON canonical_revision.chat_session_id = canonical_target.chat_session_id
		 AND canonical_revision.source_revision = canonical_target.source_revision
		 AND canonical_revision.lifecycle_state = 'active'
		WHERE identity_link.chat_session_id = ?
		  AND identity_link.source_entity_id = ?
		  AND identity_link.target_entity_id <> identity_link.source_entity_id
		  AND identity_link.link_kind = ?
		  AND identity_link.link_state = ?
		  AND canonical_target.lifecycle_state = 'active'
		  AND canonical_target.review_state IN (?, ?)
		ORDER BY identity_link.target_entity_id ASC
	`, chatSessionID, sourceEntityID, EntityIdentityLinkKindCanonicalEquivalence,
		EntityIdentityLinkStateReviewed, EntityIdentityReviewStateSourceObserved, EntityIdentityReviewStateReviewed)
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

func (m *mariadbStore) ListActiveEntityIdentities(ctx context.Context, chatSessionID string) ([]EntityIdentity, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT identity.stable_entity_id, identity.chat_session_id,
		       identity.identity_namespace, identity.entity_kind, identity.canonical_label,
		       identity.lifecycle_state, identity.review_state, identity.presence_authority,
		       identity.occurrence_authority, identity.source_contract, identity.source_revision,
		       COALESCE(identity.source_logical_turn_id, ''), COALESCE(identity.source_message_id, ''),
		       COALESCE(identity.source_generation_id, ''), identity.source_content_hash,
		       identity.source_turn, identity.source_index, identity.idempotency_key,
		       identity.mapping_revision, identity.first_seen_turn, identity.last_seen_turn,
		       identity.created_at, identity.updated_at
		FROM entity_identities identity
		JOIN memory_source_revisions revision
		  ON revision.chat_session_id = identity.chat_session_id
		 AND revision.source_revision = identity.source_revision
		 AND revision.lifecycle_state = 'active'
		WHERE identity.chat_session_id = ?
		  AND identity.lifecycle_state = 'active'
		  AND identity.review_state IN (?, ?)
		ORDER BY identity.first_seen_turn ASC, identity.stable_entity_id ASC
	`, chatSessionID, EntityIdentityReviewStateSourceObserved, EntityIdentityReviewStateReviewed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EntityIdentity{}
	for rows.Next() {
		item, err := scanEntityIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListActiveEntityIdentitySurfaces(ctx context.Context, chatSessionID string) ([]EntityIdentitySurface, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT surface.surface_id, surface.stable_entity_id, surface.chat_session_id,
		       surface.identity_namespace, surface.surface_kind, surface.surface_text,
		       surface.normalized_surface, surface.surface_scope, surface.valid_from_turn,
		       COALESCE(surface.valid_to_turn, 0), surface.source_contract, surface.source_revision,
		       surface.source_turn, COALESCE(surface.source_span_start, -1),
		       COALESCE(surface.source_span_end, -1), COALESCE(surface.evidence_excerpt, ''),
		       surface.review_state, surface.idempotency_key, surface.created_at, surface.updated_at
		FROM entity_identity_surfaces surface
		JOIN entity_identities identity
		  ON identity.chat_session_id = surface.chat_session_id
		 AND identity.stable_entity_id = surface.stable_entity_id
		 AND identity.lifecycle_state = 'active'
		JOIN memory_source_revisions revision
		  ON revision.chat_session_id = surface.chat_session_id
		 AND revision.source_revision = surface.source_revision
		 AND revision.lifecycle_state = 'active'
		WHERE surface.chat_session_id = ?
		  AND surface.review_state = ?
		ORDER BY surface.source_turn ASC, surface.surface_id ASC
	`, chatSessionID, EntityIdentityReviewStateSourceObserved)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EntityIdentitySurface{}
	for rows.Next() {
		var item EntityIdentitySurface
		if err := rows.Scan(
			&item.SurfaceID, &item.StableEntityID, &item.ChatSessionID,
			&item.IdentityNamespace, &item.SurfaceKind, &item.SurfaceText,
			&item.NormalizedSurface, &item.Scope, &item.ValidFromTurn,
			&item.ValidToTurn, &item.SourceContract, &item.SourceRevision,
			&item.SourceTurn, &item.SourceSpanStart, &item.SourceSpanEnd,
			&item.EvidenceExcerpt, &item.ReviewState, &item.IdempotencyKey,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) ListReviewedEntityIdentityLinks(ctx context.Context, chatSessionID string) ([]EntityIdentityLink, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return nil, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT link_id, chat_session_id, source_entity_id, target_entity_id,
		       link_kind, link_state, evidence_json, mapping_revision, created_at, updated_at
		FROM entity_identity_links
		WHERE chat_session_id = ?
		  AND link_kind = ?
		  AND link_state = ?
		ORDER BY created_at ASC, link_id ASC
	`, chatSessionID, EntityIdentityLinkKindCanonicalEquivalence, EntityIdentityLinkStateReviewed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EntityIdentityLink{}
	for rows.Next() {
		var item EntityIdentityLink
		if err := rows.Scan(
			&item.LinkID, &item.ChatSessionID, &item.SourceEntityID, &item.TargetEntityID,
			&item.LinkKind, &item.LinkState, &item.EvidenceJSON, &item.MappingRevision,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type entityIdentityScanner interface {
	Scan(dest ...any) error
}

func scanEntityIdentity(scanner entityIdentityScanner) (EntityIdentity, error) {
	var item EntityIdentity
	err := scanner.Scan(
		&item.StableEntityID, &item.ChatSessionID, &item.IdentityNamespace,
		&item.EntityKind, &item.CanonicalLabel, &item.LifecycleState,
		&item.ReviewState, &item.PresenceAuthority, &item.OccurrenceAuthority,
		&item.SourceContract, &item.SourceRevision, &item.SourceLogicalTurnID,
		&item.SourceMessageID, &item.SourceGenerationID, &item.SourceContentHash,
		&item.SourceTurn, &item.SourceIndex, &item.IdempotencyKey,
		&item.MappingRevision, &item.FirstSeenTurn, &item.LastSeenTurn,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (m *mariadbStore) getActiveEntityIdentity(ctx context.Context, chatSessionID, stableEntityID string) (EntityIdentity, error) {
	row := m.db.QueryRowContext(ctx, `
		SELECT identity.stable_entity_id, identity.chat_session_id,
		       identity.identity_namespace, identity.entity_kind, identity.canonical_label,
		       identity.lifecycle_state, identity.review_state, identity.presence_authority,
		       identity.occurrence_authority, identity.source_contract, identity.source_revision,
		       COALESCE(identity.source_logical_turn_id, ''), COALESCE(identity.source_message_id, ''),
		       COALESCE(identity.source_generation_id, ''), identity.source_content_hash,
		       identity.source_turn, identity.source_index, identity.idempotency_key,
		       identity.mapping_revision, identity.first_seen_turn, identity.last_seen_turn,
		       identity.created_at, identity.updated_at
		FROM entity_identities identity
		JOIN memory_source_revisions revision
		  ON revision.chat_session_id = identity.chat_session_id
		 AND revision.source_revision = identity.source_revision
		 AND revision.lifecycle_state = 'active'
		WHERE identity.chat_session_id = ?
		  AND identity.stable_entity_id = ?
		  AND identity.lifecycle_state = 'active'
		  AND identity.review_state IN (?, ?)
	`, chatSessionID, stableEntityID, EntityIdentityReviewStateSourceObserved, EntityIdentityReviewStateReviewed)
	item, err := scanEntityIdentity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return EntityIdentity{}, ErrNotFound
	}
	return item, err
}

func (m *mariadbStore) ResolveUniqueActiveEntityIDBySurface(ctx context.Context, chatSessionID, normalizedSurface string) (string, error) {
	resolved, err := m.ResolveUniqueActiveEntityIdentityBySurface(ctx, chatSessionID, normalizedSurface)
	return resolved.StableEntityID, err
}

func (m *mariadbStore) ResolveUniqueActiveEntityIdentityBySurface(ctx context.Context, chatSessionID, normalizedSurface string) (ResolvedEntityIdentity, error) {
	if err := m.ensureDB(); err != nil {
		return ResolvedEntityIdentity{}, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	normalizedSurface = strings.TrimSpace(normalizedSurface)
	if chatSessionID == "" || normalizedSurface == "" {
		return ResolvedEntityIdentity{}, ErrNotFound
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			surface.stable_entity_id,
			surface.surface_kind,
			source_identity.identity_namespace,
			source_identity.entity_kind,
			source_identity.canonical_label,
			source_identity.source_turn,
			COALESCE(identity_link.target_entity_id, ''),
			COALESCE(canonical_target.identity_namespace, ''),
			COALESCE(canonical_target.entity_kind, ''),
			COALESCE(canonical_target.canonical_label, ''),
			COALESCE(canonical_target.source_turn, 0)
		FROM entity_identity_surfaces surface
		JOIN entity_identities source_identity
		  ON source_identity.chat_session_id = surface.chat_session_id
		 AND source_identity.stable_entity_id = surface.stable_entity_id
		 AND source_identity.lifecycle_state = 'active'
		 AND source_identity.review_state IN ('source_observed', 'reviewed')
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
		 AND canonical_target.review_state IN (?, ?)
		LEFT JOIN memory_source_revisions canonical_revision
		  ON canonical_revision.chat_session_id = canonical_target.chat_session_id
		 AND canonical_revision.source_revision = canonical_target.source_revision
		 AND canonical_revision.lifecycle_state = 'active'
		WHERE surface.chat_session_id = ?
		  AND surface.normalized_surface = ?
		  AND surface.surface_scope IN (?, ?)
		  AND surface.review_state = 'source_observed'
		  AND (identity_link.target_entity_id IS NULL OR canonical_revision.source_revision IS NOT NULL)
		ORDER BY surface.stable_entity_id ASC, identity_link.target_entity_id ASC
	`, EntityIdentityLinkKindCanonicalEquivalence, EntityIdentityLinkStateReviewed,
		EntityIdentityReviewStateSourceObserved, EntityIdentityReviewStateReviewed,
		chatSessionID, normalizedSurface, EntityIdentitySurfaceScope39, EntityIdentitySurfaceScopeCurrent)
	if err != nil {
		return ResolvedEntityIdentity{}, err
	}
	defer rows.Close()
	type candidate struct {
		identity   ResolvedEntityIdentity
		sourceTurn int
		needsChain bool
	}
	resolved := map[string]candidate{}
	exactDisplayTuple := ""
	exactDisplayOnly := true
	for rows.Next() {
		var sourceEntityID, surfaceKind, sourceNamespace, sourceKind, sourceLabel string
		var targetEntityID, targetNamespace, targetKind, targetLabel string
		var sourceTurn, targetTurn int
		if err := rows.Scan(
			&sourceEntityID, &surfaceKind, &sourceNamespace, &sourceKind, &sourceLabel, &sourceTurn,
			&targetEntityID, &targetNamespace, &targetKind, &targetLabel, &targetTurn,
		); err != nil {
			return ResolvedEntityIdentity{}, err
		}
		entityID := strings.TrimSpace(targetEntityID)
		namespace := strings.TrimSpace(targetNamespace)
		entityKind := strings.TrimSpace(targetKind)
		label := strings.TrimSpace(targetLabel)
		identityTurn := targetTurn
		if entityID == "" {
			entityID = strings.TrimSpace(sourceEntityID)
			namespace = strings.TrimSpace(sourceNamespace)
			entityKind = strings.TrimSpace(sourceKind)
			label = strings.TrimSpace(sourceLabel)
			identityTurn = sourceTurn
		}
		if entityID != "" && namespace != "" && entityKind != "" {
			identity := ResolvedEntityIdentity{
				StableEntityID: entityID, IdentityNamespace: namespace,
				EntityKind: entityKind, CanonicalLabel: label,
			}
			key := entityID + "\x1f" + namespace
			current, exists := resolved[key]
			if !exists || identityTurn < current.sourceTurn || (identityTurn == current.sourceTurn && entityID < current.identity.StableEntityID) {
				resolved[key] = candidate{
					identity: identity, sourceTurn: identityTurn,
					needsChain: strings.TrimSpace(targetEntityID) != "",
				}
			} else if strings.TrimSpace(targetEntityID) != "" {
				current.needsChain = true
				resolved[key] = current
			}
			tuple := namespace + "\x1f" + entityKind + "\x1f" + label
			if strings.TrimSpace(surfaceKind) != "display_name" {
				exactDisplayOnly = false
			} else if exactDisplayTuple == "" {
				exactDisplayTuple = tuple
			} else if exactDisplayTuple != tuple {
				exactDisplayOnly = false
			}
		}
	}
	if err := rows.Err(); err != nil {
		return ResolvedEntityIdentity{}, err
	}
	if err := rows.Close(); err != nil {
		return ResolvedEntityIdentity{}, err
	}
	collapsed := map[string]candidate{}
	for _, item := range resolved {
		if !item.needsChain {
			key := item.identity.StableEntityID + "\x1f" + item.identity.IdentityNamespace
			collpasedCurrent, exists := collapsed[key]
			if !exists || item.sourceTurn < collpasedCurrent.sourceTurn ||
				(item.sourceTurn == collpasedCurrent.sourceTurn && item.identity.StableEntityID < collpasedCurrent.identity.StableEntityID) {
				collapsed[key] = item
			}
			continue
		}
		rootID, err := m.ResolveReviewedCanonicalEntityID(ctx, chatSessionID, item.identity.StableEntityID)
		switch {
		case err == nil && strings.TrimSpace(rootID) != "":
			root, getErr := m.getActiveEntityIdentity(ctx, chatSessionID, rootID)
			if getErr != nil {
				return ResolvedEntityIdentity{}, getErr
			}
			item.identity = ResolvedEntityIdentity{
				StableEntityID: root.StableEntityID, IdentityNamespace: root.IdentityNamespace,
				EntityKind: root.EntityKind, CanonicalLabel: root.CanonicalLabel,
			}
			item.sourceTurn = root.SourceTurn
		case errors.Is(err, ErrNotFound):
			// This identity is already a canonical root.
		case err != nil:
			return ResolvedEntityIdentity{}, err
		}
		key := item.identity.StableEntityID + "\x1f" + item.identity.IdentityNamespace
		current, exists := collapsed[key]
		if !exists || item.sourceTurn < current.sourceTurn ||
			(item.sourceTurn == current.sourceTurn && item.identity.StableEntityID < current.identity.StableEntityID) {
			collapsed[key] = item
		}
	}
	resolved = collapsed
	if len(resolved) == 0 {
		return ResolvedEntityIdentity{}, ErrNotFound
	}
	if len(resolved) == 1 {
		for _, item := range resolved {
			return item.identity, nil
		}
	}
	if exactDisplayOnly && exactDisplayTuple != "" {
		var selected candidate
		for _, item := range resolved {
			if selected.identity.StableEntityID == "" || item.sourceTurn < selected.sourceTurn ||
				(item.sourceTurn == selected.sourceTurn && item.identity.StableEntityID < selected.identity.StableEntityID) {
				selected = item
			}
		}
		if selected.identity.StableEntityID != "" {
			return selected.identity, nil
		}
	}
	return ResolvedEntityIdentity{}, ErrReviewedEntityIdentityAmbiguous
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
	if err == sql.ErrNoRows {
		return ErrSourceRevisionStale
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(lifecycle) != "active" {
		return ErrSourceRevisionStale
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
