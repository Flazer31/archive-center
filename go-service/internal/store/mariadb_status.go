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
	// Session deletion removes only the reusable-work link. The referenced
	// work, documents, claims, and vectors are library-owned and must survive.
	if _, err := m.db.ExecContext(ctx, "DELETE FROM session_reference_bindings WHERE chat_session_id = ?", chatSessionID); err != nil {
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
