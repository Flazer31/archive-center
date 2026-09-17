package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (m *mariadbStore) ListReferenceEntityAliasesByScope(ctx context.Context, workID, continuityID string) ([]ReferenceEntityAlias, error) {
	if referenceRequired(workID, continuityID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT alias_id, work_id, continuity_id, entity_id, alias_text,
		       normalized_alias, language_code, created_at
		FROM reference_entity_aliases
		WHERE work_id = ? AND continuity_id = ?
		ORDER BY entity_id, alias_id
	`, strings.TrimSpace(workID), strings.TrimSpace(continuityID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceEntityAlias{}
	for rows.Next() {
		var item ReferenceEntityAlias
		if err := rows.Scan(&item.AliasID, &item.WorkID, &item.ContinuityID, &item.EntityID,
			&item.AliasText, &item.NormalizedAlias, &item.LanguageCode, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) ReplaceSessionReferenceCoverageSnapshot(ctx context.Context, snapshot *SessionReferenceCoverageSnapshot, fields []SessionReferenceCoverageField) (bool, error) {
	if snapshot == nil || referenceRequired(snapshot.BindingID, snapshot.ContractVersion, snapshot.ContextHash, snapshot.InventoryHash, snapshot.SnapshotHash) != nil {
		return false, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingHash string
	var existingRevision int64
	err = tx.QueryRowContext(ctx, `
		SELECT snapshot_hash, revision
		FROM session_reference_coverage_snapshots
		WHERE binding_id = ? FOR UPDATE
	`, strings.TrimSpace(snapshot.BindingID)).Scan(&existingHash, &existingRevision)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_reference_coverage_snapshots
				(binding_id, contract_version, context_hash, inventory_hash, snapshot_hash,
				 source_message_count, field_count, covered_field_count, stats_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, strings.TrimSpace(snapshot.BindingID), strings.TrimSpace(snapshot.ContractVersion),
			strings.TrimSpace(snapshot.ContextHash), strings.TrimSpace(snapshot.InventoryHash),
			strings.TrimSpace(snapshot.SnapshotHash), snapshot.SourceMessageCount, snapshot.FieldCount,
			snapshot.CoveredFieldCount, referenceJSON(snapshot.StatsJSON))
	case err != nil:
		return false, err
	case strings.TrimSpace(existingHash) == strings.TrimSpace(snapshot.SnapshotHash):
		return false, nil
	default:
		_, err = tx.ExecContext(ctx, `
			UPDATE session_reference_coverage_snapshots
			SET contract_version = ?, context_hash = ?, inventory_hash = ?, snapshot_hash = ?,
			    source_message_count = ?, field_count = ?, covered_field_count = ?,
			    stats_json = ?, revision = revision + 1
			WHERE binding_id = ? AND revision = ?
		`, strings.TrimSpace(snapshot.ContractVersion), strings.TrimSpace(snapshot.ContextHash),
			strings.TrimSpace(snapshot.InventoryHash), strings.TrimSpace(snapshot.SnapshotHash),
			snapshot.SourceMessageCount, snapshot.FieldCount, snapshot.CoveredFieldCount,
			referenceJSON(snapshot.StatsJSON), strings.TrimSpace(snapshot.BindingID), existingRevision)
	}
	if err != nil {
		return false, referenceStoreError(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM session_reference_coverage_fields WHERE binding_id = ?`, strings.TrimSpace(snapshot.BindingID)); err != nil {
		return false, referenceStoreError(err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO session_reference_coverage_fields
			(binding_id, field_key, work_id, continuity_id, reference_kind, source_id,
			 field_name, field_value, normalized_value, match_values_json,
			 present_in_context, matched_locations_json, eligible, eligibility_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	for _, field := range fields {
		if referenceRequired(field.FieldKey, field.WorkID, field.ContinuityID, field.ReferenceKind, field.SourceID, field.FieldName) != nil {
			return false, ErrInvalidReference
		}
		if _, err := stmt.ExecContext(ctx, strings.TrimSpace(snapshot.BindingID), strings.TrimSpace(field.FieldKey),
			strings.TrimSpace(field.WorkID), strings.TrimSpace(field.ContinuityID), strings.TrimSpace(field.ReferenceKind),
			strings.TrimSpace(field.SourceID), strings.TrimSpace(field.FieldName), field.FieldValue, field.NormalizedValue,
			referenceJSON(field.MatchValuesJSON), field.PresentInContext, referenceJSON(field.MatchedLocationsJSON),
			field.Eligible, defaultString(field.EligibilityReason, "eligible")); err != nil {
			return false, referenceStoreError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

var _ ReferenceCoverageStore = (*mariadbStore)(nil)
