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

	mysql "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
func (m *mariadbStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
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
		SELECT current_value.id, current_value.chat_session_id, current_value.registry_id, current_value.status_key, current_value.owner_scope, current_value.owner_id,
		       current_value.owner_label, current_value.value_kind, current_value.value_json, current_value.evidence_json, current_value.source_turn,
		       current_value.write_state, current_value.created_at, current_value.updated_at
		FROM status_current_values current_value
		LEFT JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = current_value.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(current_value.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE current_value.chat_session_id = ? AND current_value.write_state = 'current'
		  AND (
			NULLIF(JSON_UNQUOTE(JSON_EXTRACT(current_value.evidence_json, '$.source_revision')), '') IS NULL
			OR source_revision.source_revision IS NOT NULL
		  )
	`
	args := []any{chatSessionID}
	if strings.TrimSpace(ownerScope) != "" {
		query += ` AND current_value.owner_scope = ?`
		args = append(args, strings.TrimSpace(ownerScope))
	}
	if strings.TrimSpace(ownerID) != "" {
		query += ` AND current_value.owner_id = ?`
		args = append(args, strings.TrimSpace(ownerID))
	}
	if strings.TrimSpace(statusKey) != "" {
		query += ` AND current_value.status_key = ?`
		args = append(args, strings.TrimSpace(statusKey))
	}
	query += ` ORDER BY current_value.owner_scope ASC, current_value.owner_id ASC, current_value.status_key ASC, current_value.updated_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
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

func (m *mariadbStore) ApplyReversibleStatusTransition(ctx context.Context, transition ReversibleStatusTransition) (result ReversibleStatusTransitionResult, returnErr error) {
	sourceRevision := strings.TrimSpace(transition.SourceRevision)
	sourceUnitID := strings.TrimSpace(transition.SourceUnitID)
	event := transition.Event
	defer func() {
		returnErr = reversibleStatusTransitionLockDiagnostic(returnErr, sourceRevision, sourceUnitID, event)
	}()
	if err := m.ensureDB(); err != nil {
		return result, err
	}
	if strings.TrimSpace(transition.SourceContract) != acceptedSourceObservationContract ||
		sourceRevision == "" || sourceUnitID == "" ||
		strings.TrimSpace(event.ChatSessionID) == "" ||
		strings.TrimSpace(event.StatusKey) == "" ||
		strings.TrimSpace(event.OwnerScope) == "" ||
		strings.TrimSpace(event.OwnerID) == "" ||
		event.SourceTurn <= 0 {
		return result, ErrSourceRevisionStale
	}
	eventEvidence := map[string]any{}
	if json.Unmarshal([]byte(event.EvidenceJSON), &eventEvidence) != nil ||
		strings.TrimSpace(fmt.Sprint(eventEvidence["source_revision"])) != sourceRevision ||
		strings.TrimSpace(fmt.Sprint(eventEvidence["source_unit_id"])) != sourceUnitID {
		return result, ErrSourceRevisionStale
	}
	if transition.CurrentValue != nil {
		current := transition.CurrentValue
		currentEvidence := map[string]any{}
		if current.ChatSessionID != event.ChatSessionID ||
			current.StatusKey != event.StatusKey ||
			current.OwnerScope != event.OwnerScope ||
			current.OwnerID != event.OwnerID ||
			current.SourceTurn != event.SourceTurn ||
			json.Unmarshal([]byte(current.EvidenceJSON), &currentEvidence) != nil ||
			strings.TrimSpace(fmt.Sprint(currentEvidence["source_revision"])) != sourceRevision ||
			strings.TrimSpace(fmt.Sprint(currentEvidence["source_unit_id"])) != sourceUnitID {
			return result, ErrSourceRevisionStale
		}
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
	var lifecycle string
	if err := tx.QueryRowContext(ctx, `
		SELECT lifecycle_state
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ?
		FOR UPDATE
	`, event.ChatSessionID, sourceRevision).Scan(&lifecycle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, ErrSourceRevisionStale
		}
		return result, err
	}
	if strings.TrimSpace(lifecycle) != "active" {
		return result, ErrSourceRevisionStale
	}

	existing, err := scanStatusChangeEvent(tx.QueryRowContext(ctx, `
		SELECT id, chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
		       event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
		       story_clock_json, event_state, created_at
		FROM status_change_events
		WHERE chat_session_id = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_revision')) = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_unit_id')) = ?
		ORDER BY id DESC
		LIMIT 1
	`, event.ChatSessionID, sourceRevision, sourceUnitID))
	if err == nil {
		result.Event = existing
		result.Replayed = true
		if transition.CurrentValue != nil {
			currentRows, readErr := m.listStatusCurrentValuesWithExecutor(ctx, tx, event.ChatSessionID, event.OwnerScope, event.OwnerID, event.StatusKey)
			if readErr != nil {
				return ReversibleStatusTransitionResult{}, readErr
			}
			if len(currentRows) > 0 {
				result.CurrentValue = currentRows[0]
			}
		}
		if err := tx.Commit(); err != nil {
			return ReversibleStatusTransitionResult{}, err
		}
		committed = true
		return result, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return result, err
	}
	if transition.CurrentValue != nil {
		current := *transition.CurrentValue
		var existingTurn sql.NullInt64
		err := tx.QueryRowContext(ctx, `
			SELECT source_turn
			FROM status_current_values
			WHERE chat_session_id = ? AND owner_scope = ? AND owner_id = ? AND status_key = ?
			FOR UPDATE
		`, current.ChatSessionID, current.OwnerScope, current.OwnerID, current.StatusKey).Scan(&existingTurn)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		if existingTurn.Valid && int(existingTurn.Int64) > current.SourceTurn {
			return result, ErrStatusProjectionStale
		}
		now := nonZeroTime(current.CreatedAt)
		state := firstNonEmptyString(current.WriteState, "current")
		res, err := tx.ExecContext(ctx, `
			INSERT INTO status_current_values (
				chat_session_id, registry_id, status_key, owner_scope, owner_id, owner_label,
				value_kind, value_json, evidence_json, source_turn, write_state, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?)
			ON DUPLICATE KEY UPDATE
				id = LAST_INSERT_ID(id),
				registry_id = VALUES(registry_id),
				owner_label = VALUES(owner_label),
				value_kind = VALUES(value_kind),
				value_json = VALUES(value_json),
				evidence_json = VALUES(evidence_json),
				source_turn = VALUES(source_turn),
				write_state = VALUES(write_state),
				updated_at = CURRENT_TIMESTAMP(3)
		`, current.ChatSessionID, current.RegistryID, current.StatusKey, current.OwnerScope, current.OwnerID,
			nullableString(current.OwnerLabel), current.ValueKind, current.ValueJSON, current.EvidenceJSON,
			current.SourceTurn, state, now)
		if err != nil {
			return result, err
		}
		if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
			current.ID = id
		}
		current.CreatedAt = now
		current.UpdatedAt = now
		current.WriteState = state
		result.CurrentValue = current
		event.StatusValueID = current.ID
	}
	priorEventID, err := m.latestActiveStatusEventIDTx(ctx, tx, event)
	if err != nil {
		return result, err
	}

	eventNow := nonZeroTime(event.CreatedAt)
	eventState := firstNonEmptyString(event.EventState, "recorded")
	res, err := tx.ExecContext(ctx, `
		INSERT INTO status_change_events (
			chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
			event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
			story_clock_json, event_state, created_at
		) VALUES (?, ?, NULLIF(?, 0), ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?)
	`, event.ChatSessionID, event.RegistryID, event.StatusValueID, event.StatusKey, event.OwnerScope, event.OwnerID,
		event.EventKind, nullableString(event.PreviousValueJSON), nullableString(event.NewValueJSON),
		event.EvidenceJSON, event.SourceTurn, nullableString(event.StoryClockJSON), eventState, eventNow)
	if err != nil {
		return result, err
	}
	if id, idErr := res.LastInsertId(); idErr == nil && id > 0 {
		event.ID = id
	}
	event.CreatedAt = eventNow
	event.EventState = eventState
	result.Event = event
	if err := insertStatusTransitionDependenciesTx(ctx, tx, event, sourceRevision, sourceUnitID, priorEventID); err != nil {
		return ReversibleStatusTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ReversibleStatusTransitionResult{}, err
	}
	committed = true
	return result, nil
}

func reversibleStatusTransitionLockDiagnostic(err error, sourceRevision, sourceUnitID string, event StatusChangeEvent) error {
	if err == nil {
		return nil
	}
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || (mysqlErr.Number != 1213 && mysqlErr.Number != 1205) {
		return err
	}
	sqlState := strings.Trim(string(mysqlErr.SQLState[:]), "\x00 ")
	if sqlState == "" {
		sqlState = "unknown"
	}
	return fmt.Errorf(
		"mariadb_lock_error mysql_error=%d sql_state=%s source_revision=%q source_unit_id=%q status_key=%q owner_scope=%q owner_id=%q: %w",
		mysqlErr.Number,
		sqlState,
		boundedStatusLockDiagnosticValue(sourceRevision),
		boundedStatusLockDiagnosticValue(sourceUnitID),
		boundedStatusLockDiagnosticValue(event.StatusKey),
		boundedStatusLockDiagnosticValue(event.OwnerScope),
		boundedStatusLockDiagnosticValue(event.OwnerID),
		err,
	)
}

func boundedStatusLockDiagnosticValue(value string) string {
	const maxRunes = 120
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func (m *mariadbStore) latestActiveStatusEventIDTx(ctx context.Context, tx *sql.Tx, event StatusChangeEvent) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT prior.id
		FROM status_change_events prior
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = prior.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(prior.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE prior.chat_session_id = ?
		  AND prior.status_key = ?
		  AND prior.owner_scope = ?
		  AND prior.owner_id = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(prior.evidence_json, '$.current_projection')) = 'true'
		ORDER BY prior.source_turn DESC, prior.id DESC
		LIMIT 1
		FOR UPDATE
	`, event.ChatSessionID, event.StatusKey, event.OwnerScope, event.OwnerID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func insertStatusTransitionDependenciesTx(ctx context.Context, tx *sql.Tx, event StatusChangeEvent, sourceRevision, sourceUnitID string, priorEventID int64) error {
	parents := []struct {
		kind string
		id   string
	}{{kind: "source_revision", id: sourceRevision}}
	if priorEventID > 0 {
		parents = append(parents, struct {
			kind string
			id   string
		}{kind: "status_change_event", id: strconv.FormatInt(priorEventID, 10)})
	}
	evidence := map[string]any{}
	_ = json.Unmarshal([]byte(event.EvidenceJSON), &evidence)
	seenEvidence := map[int64]bool{}
	for _, rawID := range anySlice(evidence["direct_evidence_ids"]) {
		id, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(rawID)), 10, 64)
		if err != nil || id <= 0 || seenEvidence[id] {
			continue
		}
		seenEvidence[id] = true
		parents = append(parents, struct {
			kind string
			id   string
		}{kind: "direct_evidence", id: strconv.FormatInt(id, 10)})
	}
	now := nonZeroTime(event.CreatedAt)
	for _, parent := range parents {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO memory_derivation_dependencies (
				contract_version, chat_session_id, source_revision,
				root_source_pointer, child_artifact_type, child_artifact_id,
				parent_artifact_type, parent_artifact_id, derivation_version,
				extractor_version, index_version, lifecycle_state, created_at, updated_at
			) VALUES (?, ?, ?, ?, 'status_change_event', ?, ?, ?, ?, ?, ?, 'active', ?, ?)
			ON DUPLICATE KEY UPDATE
				lifecycle_state = 'active', invalidated_at = NULL, updated_at = VALUES(updated_at)
		`, MemoryDerivationDependencyContract, event.ChatSessionID, sourceRevision,
			"source_revision:"+sourceRevision, strconv.FormatInt(event.ID, 10),
			parent.kind, parent.id, "status_transition.v1", "source_observation", "not_materialized", now, now); err != nil {
			return err
		}
	}
	_ = sourceUnitID // retained in event evidence and idempotency lookup; not a fabricated parent ID.
	return nil
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []int64:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

// restoreActiveStatusCurrentValuesTx rematerializes current values strictly
// from surviving accepted source history. It runs inside the same transaction
// that removes the canonical tail, so readers never observe a stale projection.
func restoreActiveStatusCurrentValuesTx(ctx context.Context, tx *sql.Tx, chatSessionID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO status_current_values (
			chat_session_id, registry_id, status_key, owner_scope, owner_id,
			owner_label, value_kind, value_json, evidence_json, source_turn,
			write_state, created_at, updated_at
		)
		SELECT event.chat_session_id, event.registry_id, event.status_key,
		       event.owner_scope, event.owner_id,
		       COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(event.new_value_json, '$.subject_label')), ''), registry.label),
		       registry.value_kind, event.new_value_json, event.evidence_json,
		       event.source_turn, 'current', event.created_at, CURRENT_TIMESTAMP(3)
		FROM status_change_events event
		JOIN status_schema_registry registry ON registry.id = event.registry_id
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = event.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(event.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE event.chat_session_id = ?
		  AND event.new_value_json IS NOT NULL
		  AND JSON_UNQUOTE(JSON_EXTRACT(event.evidence_json, '$.current_projection')) = 'true'
		  AND NOT EXISTS (
			SELECT 1
			FROM status_change_events newer
			JOIN memory_source_revisions newer_source
			  ON newer_source.chat_session_id = newer.chat_session_id
			 AND newer_source.source_revision = JSON_UNQUOTE(JSON_EXTRACT(newer.evidence_json, '$.source_revision'))
			 AND newer_source.lifecycle_state = 'active'
			WHERE newer.chat_session_id = event.chat_session_id
			  AND newer.status_key = event.status_key
			  AND newer.owner_scope = event.owner_scope
			  AND newer.owner_id = event.owner_id
			  AND JSON_UNQUOTE(JSON_EXTRACT(newer.evidence_json, '$.current_projection')) = 'true'
			  AND (newer.source_turn > event.source_turn OR (newer.source_turn = event.source_turn AND newer.id > event.id))
		  )
		ON DUPLICATE KEY UPDATE
			owner_label = VALUES(owner_label), value_kind = VALUES(value_kind),
			value_json = VALUES(value_json), evidence_json = VALUES(evidence_json),
			source_turn = VALUES(source_turn), write_state = 'current',
			updated_at = CURRENT_TIMESTAMP(3)
	`, chatSessionID)
	return err
}

type statusCurrentValueQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (m *mariadbStore) listStatusCurrentValuesWithExecutor(ctx context.Context, exec statusCurrentValueQueryer, chatSessionID, ownerScope, ownerID, statusKey string) ([]StatusCurrentValue, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT current_value.id, current_value.chat_session_id, current_value.registry_id, current_value.status_key, current_value.owner_scope, current_value.owner_id,
		       current_value.owner_label, current_value.value_kind, current_value.value_json, current_value.evidence_json, current_value.source_turn,
		       current_value.write_state, current_value.created_at, current_value.updated_at
		FROM status_current_values current_value
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = current_value.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(current_value.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE current_value.chat_session_id = ? AND current_value.owner_scope = ? AND current_value.owner_id = ? AND current_value.status_key = ?
		  AND current_value.write_state = 'current'
		ORDER BY current_value.updated_at DESC, current_value.id DESC
	`, chatSessionID, ownerScope, ownerID, statusKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatusCurrentValue{}
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

func (m *mariadbStore) GetReversibleStatusEventBySourceUnit(ctx context.Context, chatSessionID, sourceRevision, sourceUnitID string) (StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return StatusChangeEvent{}, err
	}
	return scanStatusChangeEvent(m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
		       event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
		       story_clock_json, event_state, created_at
		FROM status_change_events
		WHERE chat_session_id = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_revision')) = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_unit_id')) = ?
		ORDER BY id DESC
		LIMIT 1
	`, chatSessionID, sourceRevision, sourceUnitID))
}

func (m *mariadbStore) ListReversibleStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope string, statusKeys []string) ([]StatusCurrentValue, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	keys := normalizedNonEmptyStrings(statusKeys)
	if len(keys) == 0 {
		return []StatusCurrentValue{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	query := `
		SELECT current_value.id, current_value.chat_session_id, current_value.registry_id, current_value.status_key, current_value.owner_scope, current_value.owner_id,
		       current_value.owner_label, current_value.value_kind, current_value.value_json, current_value.evidence_json, current_value.source_turn,
		       current_value.write_state, current_value.created_at, current_value.updated_at
		FROM status_current_values current_value
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = current_value.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(current_value.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE current_value.chat_session_id = ?
		  AND current_value.write_state = 'current'
		  AND current_value.owner_scope = ?
		  AND current_value.status_key IN (` + placeholders + `)
		ORDER BY current_value.status_key ASC, current_value.owner_scope ASC, current_value.owner_id ASC
	`
	args := make([]any, 0, len(keys)+2)
	args = append(args, chatSessionID, strings.TrimSpace(ownerScope))
	for _, key := range keys {
		args = append(args, key)
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatusCurrentValue{}
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

func (m *mariadbStore) ListLatestReversibleCurrentProjectionEvents(ctx context.Context, chatSessionID string, statusKeys []string) ([]StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	keys := normalizedNonEmptyStrings(statusKeys)
	if len(keys) == 0 {
		return []StatusChangeEvent{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	query := `
		SELECT e.id, e.chat_session_id, e.registry_id, e.status_value_id, e.status_key, e.owner_scope, e.owner_id,
		       e.event_kind, e.previous_value_json, e.new_value_json, e.evidence_json, e.source_turn,
		       e.story_clock_json, e.event_state, e.created_at
		FROM status_change_events e
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = e.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(e.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE e.chat_session_id = ?
		  AND e.status_key IN (` + placeholders + `)
		  AND JSON_UNQUOTE(JSON_EXTRACT(e.evidence_json, '$.current_projection')) = 'true'
		  AND NOT EXISTS (
			SELECT 1
			FROM status_change_events newer
			JOIN memory_source_revisions newer_source
			  ON newer_source.chat_session_id = newer.chat_session_id
			 AND newer_source.source_revision = JSON_UNQUOTE(JSON_EXTRACT(newer.evidence_json, '$.source_revision'))
			 AND newer_source.lifecycle_state = 'active'
			WHERE newer.chat_session_id = e.chat_session_id
			  AND newer.status_key = e.status_key
			  AND newer.owner_scope = e.owner_scope
			  AND newer.owner_id = e.owner_id
			  AND JSON_UNQUOTE(JSON_EXTRACT(newer.evidence_json, '$.current_projection')) = 'true'
			  AND (newer.source_turn > e.source_turn OR (newer.source_turn = e.source_turn AND newer.id > e.id))
		  )
		ORDER BY e.status_key ASC, e.owner_scope ASC, e.owner_id ASC
	`
	args := make([]any, 0, len(keys)+1)
	args = append(args, chatSessionID)
	for _, key := range keys {
		args = append(args, key)
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatusChangeEvent{}
	for rows.Next() {
		event, scanErr := scanStatusChangeEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func normalizedNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func (m *mariadbStore) GetStatusChangeEventBySourceRevision(ctx context.Context, chatSessionID, statusKey, sourceRevision string, sourceTurn int) (StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return StatusChangeEvent{}, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT id, chat_session_id, registry_id, status_value_id, status_key, owner_scope, owner_id,
		       event_kind, previous_value_json, new_value_json, evidence_json, source_turn,
		       story_clock_json, event_state, created_at
		FROM status_change_events
		WHERE chat_session_id = ?
		  AND status_key = ?
		  AND source_turn = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_revision')) = ?
		ORDER BY id DESC
		LIMIT 1
	`, chatSessionID, statusKey, sourceTurn, sourceRevision)
	return scanStatusChangeEvent(row)
}

func (m *mariadbStore) GetLatestCurrentProjectionStatusChangeEvent(ctx context.Context, chatSessionID, statusKey string) (StatusChangeEvent, error) {
	if err := m.ensureDB(); err != nil {
		return StatusChangeEvent{}, err
	}
	row := m.db.QueryRowContext(ctx, `
		SELECT event.id, event.chat_session_id, event.registry_id, event.status_value_id, event.status_key, event.owner_scope, event.owner_id,
		       event.event_kind, event.previous_value_json, event.new_value_json, event.evidence_json, event.source_turn,
		       event.story_clock_json, event.event_state, event.created_at
		FROM status_change_events event
		JOIN memory_source_revisions source_revision
		  ON source_revision.chat_session_id = event.chat_session_id
		 AND source_revision.source_revision = JSON_UNQUOTE(JSON_EXTRACT(event.evidence_json, '$.source_revision'))
		 AND source_revision.lifecycle_state = 'active'
		WHERE event.chat_session_id = ?
		  AND event.status_key = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(event.evidence_json, '$.current_projection')) = 'true'
		ORDER BY event.source_turn DESC, event.id DESC
		LIMIT 1
	`, chatSessionID, statusKey)
	return scanStatusChangeEvent(row)
}

type statusChangeEventScanner interface {
	Scan(dest ...any) error
}

func scanStatusChangeEvent(row statusChangeEventScanner) (StatusChangeEvent, error) {
	var item StatusChangeEvent
	var statusValueID, sourceTurn sql.NullInt64
	var previousValueJSON, newValueJSON, storyClockJSON sql.NullString
	if err := row.Scan(
		&item.ID, &item.ChatSessionID, &item.RegistryID, &statusValueID, &item.StatusKey, &item.OwnerScope, &item.OwnerID,
		&item.EventKind, &previousValueJSON, &newValueJSON, &item.EvidenceJSON, &sourceTurn,
		&storyClockJSON, &item.EventState, &item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StatusChangeEvent{}, ErrNotFound
		}
		return StatusChangeEvent{}, err
	}
	item.StatusValueID = int64FromNull(statusValueID)
	item.PreviousValueJSON = stringFromNull(previousValueJSON)
	item.NewValueJSON = stringFromNull(newValueJSON)
	item.StoryClockJSON = stringFromNull(storyClockJSON)
	if sourceTurn.Valid {
		item.SourceTurn = int(sourceTurn.Int64)
	}
	return item, nil
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
	if err := m.InvalidateSourceRevisions(ctx, chatSessionID, 1, "deleted", "session_deleted", time.Now().UTC()); err != nil {
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
		"speaker_attributions",
		"entity_identity_artifact_bindings",
		"entity_identity_links",
		"entity_identity_surfaces",
		"entity_identities",
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
