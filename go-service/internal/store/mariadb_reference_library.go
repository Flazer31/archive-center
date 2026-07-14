package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func referenceRequired(values ...string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return ErrInvalidReference
		}
	}
	return nil
}

func referenceJSON(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func referenceNullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func referenceStoreError(err error) error {
	if err == nil {
		return nil
	}
	var mariaErr *mysql.MySQLError
	if errors.As(err, &mariaErr) {
		switch mariaErr.Number {
		case 1062:
			return fmt.Errorf("%w: %v", ErrReferenceConflict, err)
		case 1451, 1452:
			return fmt.Errorf("%w: %v", ErrInvalidReference, err)
		}
	}
	return err
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func (m *mariadbStore) CreateReferenceWork(ctx context.Context, item *ReferenceWork) error {
	if item == nil || referenceRequired(item.WorkID, item.Title) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_works
			(work_id, title, work_type, default_language, status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, strings.TrimSpace(item.WorkID), strings.TrimSpace(item.Title), defaultString(item.WorkType, "custom"),
		strings.TrimSpace(item.DefaultLanguage), defaultString(item.Status, "draft"), referenceJSON(item.MetadataJSON))
	return referenceStoreError(err)
}

func (m *mariadbStore) GetReferenceWork(ctx context.Context, workID string) (*ReferenceWork, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item ReferenceWork
	var metadata sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT work_id, title, work_type, default_language, status, metadata_json,
		       revision, created_at, updated_at
		FROM reference_works WHERE work_id = ?
	`, strings.TrimSpace(workID)).Scan(&item.WorkID, &item.Title, &item.WorkType, &item.DefaultLanguage,
		&item.Status, &metadata, &item.Revision, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.MetadataJSON = nullStringValue(metadata)
	return &item, nil
}

func (m *mariadbStore) ListReferenceWorks(ctx context.Context, status string, limit int) ([]ReferenceWork, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT work_id, title, work_type, default_language, status, metadata_json,
	                 revision, created_at, updated_at FROM reference_works`
	args := []any{}
	if strings.TrimSpace(status) != "" {
		query += " WHERE status = ?"
		args = append(args, strings.TrimSpace(status))
	}
	query += " ORDER BY updated_at DESC, title ASC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceWork{}
	for rows.Next() {
		var item ReferenceWork
		var metadata sql.NullString
		if err := rows.Scan(&item.WorkID, &item.Title, &item.WorkType, &item.DefaultLanguage,
			&item.Status, &metadata, &item.Revision, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.MetadataJSON = nullStringValue(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) UpdateReferenceWork(ctx context.Context, item *ReferenceWork, expectedRevision int64) error {
	if item == nil || expectedRevision < 1 || referenceRequired(item.WorkID, item.Title) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE reference_works
		SET title = ?, work_type = ?, default_language = ?, status = ?, metadata_json = ?, revision = revision + 1
		WHERE work_id = ? AND revision = ?
	`, strings.TrimSpace(item.Title), defaultString(item.WorkType, "custom"), strings.TrimSpace(item.DefaultLanguage),
		defaultString(item.Status, "draft"), referenceJSON(item.MetadataJSON), strings.TrimSpace(item.WorkID), expectedRevision)
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRevisionChanged(result)
}

func (m *mariadbStore) DeleteReferenceWork(ctx context.Context, workID string) error {
	if referenceRequired(workID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bindingID string
	err = tx.QueryRowContext(ctx, `
		SELECT binding_id FROM session_reference_bindings
		WHERE work_id = ? LIMIT 1 FOR UPDATE
	`, strings.TrimSpace(workID)).Scan(&bindingID)
	if err == nil {
		return ErrReferenceWorkInUse
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	result, err := tx.ExecContext(ctx, "DELETE FROM reference_works WHERE work_id = ?", strings.TrimSpace(workID))
	if err != nil {
		return referenceStoreError(err)
	}
	if err := referenceRowsChanged(result); err != nil {
		return err
	}
	return tx.Commit()
}

func referenceRowsChanged(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func referenceRevisionChanged(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrReferenceConflict
	}
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func (m *mariadbStore) UpsertReferenceContinuity(ctx context.Context, item *ReferenceContinuity) error {
	if item == nil || referenceRequired(item.ContinuityID, item.WorkID, item.ContinuityKey, item.Label) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_continuities
			(continuity_id, work_id, continuity_key, label, parent_continuity_id, status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE label = VALUES(label), parent_continuity_id = VALUES(parent_continuity_id),
			status = VALUES(status), metadata_json = VALUES(metadata_json), revision = revision + 1
	`, strings.TrimSpace(item.ContinuityID), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityKey),
		strings.TrimSpace(item.Label), referenceNullable(item.ParentContinuityID), defaultString(item.Status, "active"),
		referenceJSON(item.MetadataJSON))
	return referenceStoreError(err)
}

func (m *mariadbStore) ListReferenceContinuities(ctx context.Context, workID string) ([]ReferenceContinuity, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT continuity_id, work_id, continuity_key, label, parent_continuity_id,
		       status, metadata_json, revision, created_at, updated_at
		FROM reference_continuities WHERE work_id = ? ORDER BY label, continuity_key
	`, strings.TrimSpace(workID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceContinuity{}
	for rows.Next() {
		var item ReferenceContinuity
		var parent, metadata sql.NullString
		if err := rows.Scan(&item.ContinuityID, &item.WorkID, &item.ContinuityKey, &item.Label,
			&parent, &item.Status, &metadata, &item.Revision, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ParentContinuityID = nullStringValue(parent)
		item.MetadataJSON = nullStringValue(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) DeleteReferenceContinuity(ctx context.Context, continuityID string) error {
	return m.deleteReferenceRow(ctx, "reference_continuities", "continuity_id", continuityID)
}

func (m *mariadbStore) SaveReferenceDocument(ctx context.Context, item *ReferenceDocument) error {
	if item == nil || referenceRequired(item.DocumentID, item.WorkID, item.ContinuityID, item.ContentHash) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_documents
			(document_id, work_id, continuity_id, source_type, source_uri, content_hash,
			 raw_retention, raw_text, import_status, provenance_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, strings.TrimSpace(item.DocumentID), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityID),
		defaultString(item.SourceType, "manual_text"), referenceNullable(item.SourceURI), strings.TrimSpace(item.ContentHash),
		defaultString(item.RawRetention, "full"), referenceNullable(item.RawText), defaultString(item.ImportStatus, "pending"),
		referenceJSON(item.ProvenanceJSON))
	return referenceStoreError(err)
}

func (m *mariadbStore) GetReferenceDocument(ctx context.Context, documentID string) (*ReferenceDocument, error) {
	if referenceRequired(documentID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item ReferenceDocument
	var uri, raw, provenance sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT document_id, work_id, continuity_id, source_type, source_uri, content_hash,
		       raw_retention, raw_text, import_status, provenance_json, created_at, updated_at
		FROM reference_documents WHERE document_id = ?
	`, strings.TrimSpace(documentID)).Scan(&item.DocumentID, &item.WorkID, &item.ContinuityID,
		&item.SourceType, &uri, &item.ContentHash, &item.RawRetention, &raw, &item.ImportStatus,
		&provenance, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.SourceURI = nullStringValue(uri)
	item.RawText = nullStringValue(raw)
	item.ProvenanceJSON = nullStringValue(provenance)
	return &item, nil
}

func (m *mariadbStore) UpdateReferenceDocumentStatus(ctx context.Context, documentID, status string) error {
	if referenceRequired(documentID, status) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE reference_documents SET import_status = ? WHERE document_id = ?
	`, strings.TrimSpace(status), strings.TrimSpace(documentID))
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRowsChanged(result)
}

func (m *mariadbStore) ListReferenceDocuments(ctx context.Context, workID, continuityID, status string) ([]ReferenceDocument, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT document_id, work_id, continuity_id, source_type, source_uri, content_hash,
	                 raw_retention, raw_text, import_status, provenance_json, created_at, updated_at
	          FROM reference_documents WHERE work_id = ?`
	args := []any{strings.TrimSpace(workID)}
	if strings.TrimSpace(continuityID) != "" {
		query += " AND continuity_id = ?"
		args = append(args, strings.TrimSpace(continuityID))
	}
	if strings.TrimSpace(status) != "" {
		query += " AND import_status = ?"
		args = append(args, strings.TrimSpace(status))
	}
	query += " ORDER BY updated_at DESC, document_id"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceDocument{}
	for rows.Next() {
		var item ReferenceDocument
		var uri, raw, provenance sql.NullString
		if err := rows.Scan(&item.DocumentID, &item.WorkID, &item.ContinuityID, &item.SourceType, &uri,
			&item.ContentHash, &item.RawRetention, &raw, &item.ImportStatus, &provenance,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.SourceURI = nullStringValue(uri)
		item.RawText = nullStringValue(raw)
		item.ProvenanceJSON = nullStringValue(provenance)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) DeleteReferenceDocument(ctx context.Context, documentID string) error {
	return m.deleteReferenceRow(ctx, "reference_documents", "document_id", documentID)
}

func (m *mariadbStore) deleteReferenceRow(ctx context.Context, table, key, value string) error {
	if referenceRequired(value) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	allowed := map[string]string{
		"reference_continuities":   "continuity_id",
		"reference_documents":      "document_id",
		"reference_timeline_nodes": "node_id",
		"reference_entities":       "entity_id",
		"reference_claims":         "claim_id",
	}
	if allowed[table] != key {
		return ErrInvalidReference
	}
	result, err := m.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE "+key+" = ?", strings.TrimSpace(value))
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRowsChanged(result)
}

func (m *mariadbStore) UpsertReferenceTimelineNode(ctx context.Context, item *ReferenceTimelineNode) error {
	if item == nil || referenceRequired(item.NodeID, item.WorkID, item.ContinuityID, item.NodeKey, item.Label) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_timeline_nodes
			(node_id, work_id, continuity_id, node_key, label, ordinal_value,
			 parent_node_id, branch_key, node_kind, metadata_json, review_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			label = IF(review_status = 'pending', VALUES(label), label),
			ordinal_value = IF(review_status = 'pending', VALUES(ordinal_value), ordinal_value),
			parent_node_id = IF(review_status = 'pending', VALUES(parent_node_id), parent_node_id),
			branch_key = IF(review_status = 'pending', VALUES(branch_key), branch_key),
			node_kind = IF(review_status = 'pending', VALUES(node_kind), node_kind),
			metadata_json = IF(review_status = 'pending', VALUES(metadata_json), metadata_json),
			review_status = IF(review_status = 'pending', VALUES(review_status), review_status)
	`, strings.TrimSpace(item.NodeID), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityID),
		strings.TrimSpace(item.NodeKey), strings.TrimSpace(item.Label), item.Ordinal,
		referenceNullable(item.ParentNodeID), defaultString(item.BranchKey, "main"),
		defaultString(item.NodeKind, "event"), referenceJSON(item.MetadataJSON), defaultString(item.ReviewStatus, "pending"))
	return referenceStoreError(err)
}

func (m *mariadbStore) ListReferenceTimelineNodes(ctx context.Context, workID, continuityID, branchKey string) ([]ReferenceTimelineNode, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT node_id, work_id, continuity_id, node_key, label, ordinal_value,
		                 parent_node_id, branch_key, node_kind, metadata_json, review_status,
		                 review_source, review_reason, reviewed_at, created_at, updated_at
	          FROM reference_timeline_nodes WHERE work_id = ?`
	args := []any{strings.TrimSpace(workID)}
	if strings.TrimSpace(continuityID) != "" {
		query += " AND continuity_id = ?"
		args = append(args, strings.TrimSpace(continuityID))
	}
	if strings.TrimSpace(branchKey) != "" {
		query += " AND branch_key = ?"
		args = append(args, strings.TrimSpace(branchKey))
	}
	query += " ORDER BY continuity_id, branch_key, ordinal_value, node_key"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceTimelineNode{}
	for rows.Next() {
		var item ReferenceTimelineNode
		var parent, metadata, reviewReason sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(&item.NodeID, &item.WorkID, &item.ContinuityID, &item.NodeKey, &item.Label,
			&item.Ordinal, &parent, &item.BranchKey, &item.NodeKind, &metadata, &item.ReviewStatus,
			&item.ReviewSource, &reviewReason, &reviewedAt,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ParentNodeID = nullStringValue(parent)
		item.MetadataJSON = nullStringValue(metadata)
		item.ReviewReason = nullStringValue(reviewReason)
		if reviewedAt.Valid {
			value := reviewedAt.Time
			item.ReviewedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) DeleteReferenceTimelineNode(ctx context.Context, nodeID string) error {
	return m.deleteReferenceRow(ctx, "reference_timeline_nodes", "node_id", nodeID)
}

func (m *mariadbStore) NormalizeReferenceTimelineOrder(ctx context.Context, workID, continuityID string) (int, error) {
	if referenceRequired(workID, continuityID) != nil {
		return 0, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return 0, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT node_id FROM reference_timeline_nodes
		WHERE work_id = ? AND continuity_id = ? AND review_status = 'approved'
		ORDER BY ordinal_value, branch_key, node_key, node_id
	`, strings.TrimSpace(workID), strings.TrimSpace(continuityID))
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			UPDATE reference_timeline_nodes SET ordinal_value = ?
			WHERE work_id = ? AND continuity_id = ? AND node_id = ?
		`, int64((i+1)*10), strings.TrimSpace(workID), strings.TrimSpace(continuityID), id); err != nil {
			return 0, referenceStoreError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (m *mariadbStore) ApplyReferenceTimelineOrder(ctx context.Context, workID, continuityID string, orderedIDs []string) (int, error) {
	if referenceRequired(workID, continuityID) != nil || len(orderedIDs) == 0 {
		return 0, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return 0, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT node_id FROM reference_timeline_nodes
		WHERE work_id = ? AND continuity_id = ? AND review_status = 'approved'
	`, strings.TrimSpace(workID), strings.TrimSpace(continuityID))
	if err != nil {
		return 0, err
	}
	allowed := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		allowed[id] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(allowed) != len(orderedIDs) {
		return 0, ErrReferenceConflict
	}
	seen := map[string]struct{}{}
	for _, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if _, ok := allowed[id]; !ok {
			return 0, ErrReferenceConflict
		}
		if _, duplicate := seen[id]; duplicate {
			return 0, ErrReferenceConflict
		}
		seen[id] = struct{}{}
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE reference_timeline_nodes SET ordinal_value = ?
			WHERE work_id = ? AND continuity_id = ? AND node_id = ? AND review_status = 'approved'
		`, int64((i+1)*10), strings.TrimSpace(workID), strings.TrimSpace(continuityID), strings.TrimSpace(id)); err != nil {
			return 0, referenceStoreError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(orderedIDs), nil
}

func (m *mariadbStore) UpsertReferenceEntity(ctx context.Context, item *ReferenceEntity) error {
	if item == nil || referenceRequired(item.EntityID, item.WorkID, item.ContinuityID, item.EntityType, item.CanonicalName) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_entities
			(entity_id, work_id, continuity_id, entity_type, canonical_name,
			 description_text, metadata_json, review_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			entity_type = IF(review_status = 'pending', VALUES(entity_type), entity_type),
			canonical_name = IF(review_status = 'pending', VALUES(canonical_name), canonical_name),
			description_text = IF(review_status = 'pending', VALUES(description_text), description_text),
			metadata_json = IF(review_status = 'pending', VALUES(metadata_json), metadata_json),
			review_status = IF(review_status = 'pending', VALUES(review_status), review_status)
	`, strings.TrimSpace(item.EntityID), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityID),
		strings.TrimSpace(item.EntityType), strings.TrimSpace(item.CanonicalName), referenceNullable(item.DescriptionText),
		referenceJSON(item.MetadataJSON), defaultString(item.ReviewStatus, "pending"))
	return referenceStoreError(err)
}

func (m *mariadbStore) ListReferenceEntities(ctx context.Context, workID, continuityID, entityType string) ([]ReferenceEntity, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT entity_id, work_id, continuity_id, entity_type, canonical_name,
	                 description_text, metadata_json, review_status, review_source,
	                 review_reason, reviewed_at, created_at, updated_at
	          FROM reference_entities WHERE work_id = ?`
	args := []any{strings.TrimSpace(workID)}
	if strings.TrimSpace(continuityID) != "" {
		query += " AND continuity_id = ?"
		args = append(args, strings.TrimSpace(continuityID))
	}
	if strings.TrimSpace(entityType) != "" {
		query += " AND entity_type = ?"
		args = append(args, strings.TrimSpace(entityType))
	}
	query += " ORDER BY canonical_name, entity_id"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ReferenceEntity{}
	for rows.Next() {
		var item ReferenceEntity
		var description, metadata, reviewReason sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(&item.EntityID, &item.WorkID, &item.ContinuityID, &item.EntityType,
			&item.CanonicalName, &description, &metadata, &item.ReviewStatus, &item.ReviewSource,
			&reviewReason, &reviewedAt,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.DescriptionText = nullStringValue(description)
		item.MetadataJSON = nullStringValue(metadata)
		item.ReviewReason = nullStringValue(reviewReason)
		if reviewedAt.Valid {
			value := reviewedAt.Time
			item.ReviewedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) UpsertReferenceEntityAlias(ctx context.Context, item *ReferenceEntityAlias) error {
	if item == nil || referenceRequired(item.WorkID, item.ContinuityID, item.EntityID, item.AliasText, item.NormalizedAlias) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_entity_aliases
			(work_id, continuity_id, entity_id, alias_text, normalized_alias, language_code)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE alias_text = VALUES(alias_text), language_code = VALUES(language_code)
	`, strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityID), strings.TrimSpace(item.EntityID),
		strings.TrimSpace(item.AliasText), strings.TrimSpace(item.NormalizedAlias), strings.TrimSpace(item.LanguageCode))
	if err != nil {
		return referenceStoreError(err)
	}
	if item.AliasID == 0 {
		item.AliasID, _ = result.LastInsertId()
	}
	return nil
}

func (m *mariadbStore) ListReferenceEntityAliases(ctx context.Context, entityID string) ([]ReferenceEntityAlias, error) {
	if referenceRequired(entityID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT alias_id, work_id, continuity_id, entity_id, alias_text,
		       normalized_alias, language_code, created_at
		FROM reference_entity_aliases WHERE entity_id = ? ORDER BY alias_text, alias_id
	`, strings.TrimSpace(entityID))
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

func (m *mariadbStore) DeleteReferenceEntity(ctx context.Context, entityID string) error {
	return m.deleteReferenceRow(ctx, "reference_entities", "entity_id", entityID)
}

func (m *mariadbStore) UpsertReferenceClaim(ctx context.Context, item *ReferenceClaim) error {
	if item == nil || referenceRequired(item.ClaimID, item.WorkID, item.ContinuityID, item.DocumentID, item.ClaimType, item.ClaimText) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_claims
			(claim_id, work_id, continuity_id, document_id, claim_type, subject_entity_id,
			 claim_text, evidence_excerpt, temporal_scope, valid_from_node_id, valid_to_node_id,
			 reveal_from_node_id, branch_key, knowledge_scope, confidence, review_status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			claim_type = IF(review_status = 'pending', VALUES(claim_type), claim_type),
			subject_entity_id = IF(review_status = 'pending', VALUES(subject_entity_id), subject_entity_id),
			claim_text = IF(review_status = 'pending', VALUES(claim_text), claim_text),
			evidence_excerpt = IF(review_status = 'pending', VALUES(evidence_excerpt), evidence_excerpt),
			temporal_scope = IF(review_status = 'pending', VALUES(temporal_scope), temporal_scope),
			valid_from_node_id = IF(review_status = 'pending', VALUES(valid_from_node_id), valid_from_node_id),
			valid_to_node_id = IF(review_status = 'pending', VALUES(valid_to_node_id), valid_to_node_id),
			reveal_from_node_id = IF(review_status = 'pending', VALUES(reveal_from_node_id), reveal_from_node_id),
			branch_key = IF(review_status = 'pending', VALUES(branch_key), branch_key),
			knowledge_scope = IF(review_status = 'pending', VALUES(knowledge_scope), knowledge_scope),
			confidence = IF(review_status = 'pending', VALUES(confidence), confidence),
			metadata_json = IF(review_status = 'pending', VALUES(metadata_json), metadata_json),
			review_status = IF(review_status = 'pending', VALUES(review_status), review_status)
	`, strings.TrimSpace(item.ClaimID), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ContinuityID),
		strings.TrimSpace(item.DocumentID), strings.TrimSpace(item.ClaimType), referenceNullable(item.SubjectEntityID),
		strings.TrimSpace(item.ClaimText), referenceNullable(item.EvidenceExcerpt), defaultString(item.TemporalScope, "bounded"),
		referenceNullable(item.ValidFromNodeID), referenceNullable(item.ValidToNodeID), referenceNullable(item.RevealFromNodeID),
		defaultString(item.BranchKey, "main"), defaultString(item.KnowledgeScope, "public_world"), item.Confidence,
		defaultString(item.ReviewStatus, "pending"), referenceJSON(item.MetadataJSON))
	return referenceStoreError(err)
}

func (m *mariadbStore) ListReferenceClaims(ctx context.Context, workID, continuityID, reviewStatus, branchKey string) ([]ReferenceClaim, error) {
	if referenceRequired(workID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT claim_id, work_id, continuity_id, document_id, claim_type, subject_entity_id,
	                 claim_text, evidence_excerpt, temporal_scope, valid_from_node_id, valid_to_node_id,
	                 reveal_from_node_id, branch_key, knowledge_scope, confidence, review_status,
	                 review_source, review_reason, reviewed_at, metadata_json, created_at, updated_at
	          FROM reference_claims WHERE work_id = ?`
	args := []any{strings.TrimSpace(workID)}
	if strings.TrimSpace(continuityID) != "" {
		query += " AND continuity_id = ?"
		args = append(args, strings.TrimSpace(continuityID))
	}
	if strings.TrimSpace(reviewStatus) != "" {
		query += " AND review_status = ?"
		args = append(args, strings.TrimSpace(reviewStatus))
	}
	if strings.TrimSpace(branchKey) != "" {
		query += " AND branch_key = ?"
		args = append(args, strings.TrimSpace(branchKey))
	}
	query += " ORDER BY updated_at DESC, claim_id"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	items := []ReferenceClaim{}
	for rows.Next() {
		var item ReferenceClaim
		var subject, evidence, validFrom, validTo, revealFrom, metadata, reviewReason sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(&item.ClaimID, &item.WorkID, &item.ContinuityID, &item.DocumentID,
			&item.ClaimType, &subject, &item.ClaimText, &evidence, &item.TemporalScope,
			&validFrom, &validTo, &revealFrom, &item.BranchKey, &item.KnowledgeScope,
			&item.Confidence, &item.ReviewStatus, &item.ReviewSource, &reviewReason, &reviewedAt,
			&metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.SubjectEntityID = nullStringValue(subject)
		item.EvidenceExcerpt = nullStringValue(evidence)
		item.ValidFromNodeID = nullStringValue(validFrom)
		item.ValidToNodeID = nullStringValue(validTo)
		item.RevealFromNodeID = nullStringValue(revealFrom)
		item.MetadataJSON = nullStringValue(metadata)
		item.ReviewReason = nullStringValue(reviewReason)
		if reviewedAt.Valid {
			value := reviewedAt.Time
			item.ReviewedAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		knowers, err := m.listReferenceClaimKnowers(ctx, items[i].ClaimID)
		if err != nil {
			return nil, err
		}
		items[i].KnowerEntityIDs = knowers
	}
	return items, nil
}

func (m *mariadbStore) ReplaceReferenceClaimKnowers(ctx context.Context, claimID string, entityIDs []string) error {
	if referenceRequired(claimID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM reference_claim_knowers WHERE claim_id = ?", strings.TrimSpace(claimID)); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, entityID := range entityIDs {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			return ErrInvalidReference
		}
		if _, ok := seen[entityID]; ok {
			continue
		}
		seen[entityID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO reference_claim_knowers (claim_id, entity_id) VALUES (?, ?)
		`, strings.TrimSpace(claimID), entityID); err != nil {
			return referenceStoreError(err)
		}
	}
	return tx.Commit()
}

func (m *mariadbStore) listReferenceClaimKnowers(ctx context.Context, claimID string) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT entity_id FROM reference_claim_knowers WHERE claim_id = ? ORDER BY entity_id
	`, strings.TrimSpace(claimID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var entityID string
		if err := rows.Scan(&entityID); err != nil {
			return nil, err
		}
		items = append(items, entityID)
	}
	return items, rows.Err()
}

func (m *mariadbStore) DeleteReferenceClaim(ctx context.Context, claimID string) error {
	return m.deleteReferenceRow(ctx, "reference_claims", "claim_id", claimID)
}

func (m *mariadbStore) UpdateReferenceCandidateReview(ctx context.Context, workID, kind, id, status, source, reason string) error {
	if referenceRequired(workID, kind, id, status, source) != nil {
		return ErrInvalidReference
	}
	if status != "approved" && status != "rejected" && status != "pending" {
		return ErrInvalidReference
	}
	tableAndKey := map[string][2]string{
		"timeline": {"reference_timeline_nodes", "node_id"},
		"entity":   {"reference_entities", "entity_id"},
		"claim":    {"reference_claims", "claim_id"},
	}
	target, ok := tableAndKey[strings.TrimSpace(kind)]
	if !ok {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx,
		"UPDATE "+target[0]+" SET review_status = ?, review_source = ?, review_reason = ?, reviewed_at = CURRENT_TIMESTAMP(3) WHERE work_id = ? AND "+target[1]+" = ?",
		status, strings.TrimSpace(source), referenceNullable(reason), strings.TrimSpace(workID), strings.TrimSpace(id))
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRowsChanged(result)
}

func (m *mariadbStore) UpdateReferenceLibraryItem(ctx context.Context, item *ReferenceLibraryItemUpdate) error {
	if item == nil || referenceRequired(item.WorkID, item.Kind, item.ID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	var result sql.Result
	var err error
	switch strings.TrimSpace(item.Kind) {
	case "timeline":
		if referenceRequired(item.NodeKey, item.Label, item.NodeKind, item.BranchKey) != nil {
			return ErrInvalidReference
		}
		result, err = m.db.ExecContext(ctx, `
			UPDATE reference_timeline_nodes
			SET node_key = ?, label = ?, ordinal_value = ?, node_kind = ?, branch_key = ?,
				review_status = 'approved', review_source = 'user_edit',
				review_reason = 'user corrected reference data', reviewed_at = CURRENT_TIMESTAMP(3)
			WHERE work_id = ? AND node_id = ?
		`, strings.TrimSpace(item.NodeKey), strings.TrimSpace(item.Label), item.Ordinal,
			strings.TrimSpace(item.NodeKind), strings.TrimSpace(item.BranchKey),
			strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ID))
	case "entity":
		if referenceRequired(item.EntityType, item.CanonicalName) != nil {
			return ErrInvalidReference
		}
		result, err = m.db.ExecContext(ctx, `
			UPDATE reference_entities
			SET entity_type = ?, canonical_name = ?, description_text = ?,
				review_status = 'approved', review_source = 'user_edit',
				review_reason = 'user corrected reference data', reviewed_at = CURRENT_TIMESTAMP(3)
			WHERE work_id = ? AND entity_id = ?
		`, strings.TrimSpace(item.EntityType), strings.TrimSpace(item.CanonicalName),
			referenceNullable(item.DescriptionText), strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ID))
	case "claim":
		if referenceRequired(item.ClaimType, item.ClaimText, item.TemporalScope, item.KnowledgeScope) != nil {
			return ErrInvalidReference
		}
		result, err = m.db.ExecContext(ctx, `
			UPDATE reference_claims
			SET claim_type = ?, claim_text = ?, evidence_excerpt = ?, temporal_scope = ?,
				knowledge_scope = ?, confidence = ?, review_status = 'approved',
				metadata_json = JSON_SET(COALESCE(metadata_json, JSON_OBJECT()), '$.evidence_grounded', false),
				review_source = 'user_edit', review_reason = 'user corrected reference data',
				reviewed_at = CURRENT_TIMESTAMP(3)
			WHERE work_id = ? AND claim_id = ?
		`, strings.TrimSpace(item.ClaimType), strings.TrimSpace(item.ClaimText),
			referenceNullable(item.EvidenceExcerpt), strings.TrimSpace(item.TemporalScope),
			strings.TrimSpace(item.KnowledgeScope), item.Confidence,
			strings.TrimSpace(item.WorkID), strings.TrimSpace(item.ID))
	default:
		return ErrInvalidReference
	}
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRowsChanged(result)
}

func (m *mariadbStore) UpsertSessionReferenceBinding(ctx context.Context, item *SessionReferenceBinding, expectedRevision int64) error {
	if item == nil || expectedRevision < 0 || referenceRequired(item.BindingID, item.ChatSessionID, item.WorkID, item.ContinuityID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	if expectedRevision == 0 {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO session_reference_bindings
				(binding_id, chat_session_id, work_id, continuity_id, binding_role, enabled, injection_enabled,
				 anchor_mode, current_node_id, reveal_ceiling_node_id, divergence_node_id,
				 future_policy, priority)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, strings.TrimSpace(item.BindingID), strings.TrimSpace(item.ChatSessionID), strings.TrimSpace(item.WorkID),
			strings.TrimSpace(item.ContinuityID), defaultString(item.BindingRole, "primary"), item.Enabled, item.InjectionEnabled,
			defaultString(item.AnchorMode, "manual"), referenceNullable(item.CurrentNodeID),
			referenceNullable(item.RevealCeilingNodeID), referenceNullable(item.DivergenceNodeID),
			defaultString(item.FuturePolicy, "block"), item.Priority)
		return referenceStoreError(err)
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE session_reference_bindings
		SET binding_role = ?, enabled = ?, injection_enabled = ?, anchor_mode = ?, current_node_id = ?,
			reveal_ceiling_node_id = ?, divergence_node_id = ?, future_policy = ?,
			priority = ?, revision = revision + 1
		WHERE binding_id = ? AND chat_session_id = ? AND revision = ?
	`, defaultString(item.BindingRole, "primary"), item.Enabled, item.InjectionEnabled, defaultString(item.AnchorMode, "manual"),
		referenceNullable(item.CurrentNodeID), referenceNullable(item.RevealCeilingNodeID),
		referenceNullable(item.DivergenceNodeID), defaultString(item.FuturePolicy, "block"), item.Priority,
		strings.TrimSpace(item.BindingID), strings.TrimSpace(item.ChatSessionID), expectedRevision)
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRevisionChanged(result)
}

func (m *mariadbStore) ListSessionReferenceBindings(ctx context.Context, chatSessionID string, enabledOnly bool) ([]SessionReferenceBinding, error) {
	if referenceRequired(chatSessionID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT binding_id, chat_session_id, work_id, continuity_id, binding_role,
	                 enabled, injection_enabled, anchor_mode, current_node_id, reveal_ceiling_node_id,
	                 divergence_node_id, future_policy, priority, revision, created_at, updated_at
	          FROM session_reference_bindings WHERE chat_session_id = ?`
	args := []any{strings.TrimSpace(chatSessionID)}
	if enabledOnly {
		query += " AND enabled = TRUE"
	}
	query += " ORDER BY priority DESC, created_at, binding_id"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SessionReferenceBinding{}
	for rows.Next() {
		var item SessionReferenceBinding
		var current, reveal, divergence sql.NullString
		if err := rows.Scan(&item.BindingID, &item.ChatSessionID, &item.WorkID, &item.ContinuityID,
			&item.BindingRole, &item.Enabled, &item.InjectionEnabled, &item.AnchorMode, &current, &reveal, &divergence,
			&item.FuturePolicy, &item.Priority, &item.Revision, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.CurrentNodeID = nullStringValue(current)
		item.RevealCeilingNodeID = nullStringValue(reveal)
		item.DivergenceNodeID = nullStringValue(divergence)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) DeleteSessionReferenceBinding(ctx context.Context, chatSessionID, bindingID string) error {
	if referenceRequired(chatSessionID, bindingID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	result, err := m.db.ExecContext(ctx, `
		DELETE FROM session_reference_bindings WHERE chat_session_id = ? AND binding_id = ?
	`, strings.TrimSpace(chatSessionID), strings.TrimSpace(bindingID))
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRowsChanged(result)
}

func (m *mariadbStore) UpsertSessionReferenceRuntime(ctx context.Context, item *SessionReferenceRuntime, expectedRevision int64) error {
	if item == nil || expectedRevision < 0 || referenceRequired(item.BindingID) != nil {
		return ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return err
	}
	if expectedRevision == 0 {
		_, err := m.db.ExecContext(ctx, `
			INSERT INTO session_reference_runtime
				(binding_id, candidate_node_id, candidate_source_turn, candidate_evidence_json,
				 candidate_confirmed, last_claim_ids_json, diagnostics_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, strings.TrimSpace(item.BindingID), referenceNullable(item.CandidateNodeID), nullablePositiveInt(item.CandidateSourceTurn),
			referenceJSON(item.CandidateEvidenceJSON), item.CandidateConfirmed,
			referenceJSON(item.LastClaimIDsJSON), referenceJSON(item.DiagnosticsJSON))
		return referenceStoreError(err)
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE session_reference_runtime
		SET candidate_node_id = ?, candidate_source_turn = ?, candidate_evidence_json = ?,
			candidate_confirmed = ?, last_claim_ids_json = ?, diagnostics_json = ?, revision = revision + 1
		WHERE binding_id = ? AND revision = ?
	`, referenceNullable(item.CandidateNodeID), nullablePositiveInt(item.CandidateSourceTurn),
		referenceJSON(item.CandidateEvidenceJSON), item.CandidateConfirmed,
		referenceJSON(item.LastClaimIDsJSON), referenceJSON(item.DiagnosticsJSON),
		strings.TrimSpace(item.BindingID), expectedRevision)
	if err != nil {
		return referenceStoreError(err)
	}
	return referenceRevisionChanged(result)
}

func nullablePositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (m *mariadbStore) GetSessionReferenceRuntime(ctx context.Context, bindingID string) (*SessionReferenceRuntime, error) {
	if referenceRequired(bindingID) != nil {
		return nil, ErrInvalidReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var item SessionReferenceRuntime
	var candidateNode, evidence, claims, diagnostics sql.NullString
	var sourceTurn sql.NullInt64
	err := m.db.QueryRowContext(ctx, `
		SELECT binding_id, candidate_node_id, candidate_source_turn, candidate_evidence_json,
		       candidate_confirmed, last_claim_ids_json, diagnostics_json, revision, created_at, updated_at
		FROM session_reference_runtime WHERE binding_id = ?
	`, strings.TrimSpace(bindingID)).Scan(&item.BindingID, &candidateNode, &sourceTurn, &evidence,
		&item.CandidateConfirmed, &claims, &diagnostics, &item.Revision, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.CandidateNodeID = nullStringValue(candidateNode)
	if sourceTurn.Valid {
		item.CandidateSourceTurn = int(sourceTurn.Int64)
	}
	item.CandidateEvidenceJSON = nullStringValue(evidence)
	item.LastClaimIDsJSON = nullStringValue(claims)
	item.DiagnosticsJSON = nullStringValue(diagnostics)
	return &item, nil
}

var _ ReferenceLibraryStore = (*mariadbStore)(nil)
