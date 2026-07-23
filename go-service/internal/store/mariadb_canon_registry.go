package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

const CanonPackDiagnosticsContract = "canon_pack_diagnostics.v1"

func (m *mariadbStore) SearchCanonRegistry(ctx context.Context, query string, limit int) ([]CanonRegistryItem, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	lookup := canonNormalize(query)
	if lookup == "" {
		return nil, ErrInvalidReference
	}
	querySQL := `
		SELECT i.install_id, i.pack_id, i.pack_version, i.lifecycle_status,
		       i.work_id, i.edition_row_id, e.stable_work_id, e.edition_id, w.title,
		       t.title_text, i.review_status, i.trust_status, i.manifest_json,
		       i.coverage_report_json,
		       (SELECT COUNT(*) FROM reference_source_observations s WHERE s.install_id = i.install_id)
		FROM canon_pack_installs i
		JOIN reference_work_editions e ON e.edition_row_id = i.edition_row_id
		JOIN reference_works w ON w.work_id = i.work_id
		JOIN reference_work_titles t ON t.edition_row_id = i.edition_row_id
		WHERE i.lifecycle_status <> 'removed' AND t.normalized_lookup_key LIKE ?
		ORDER BY (t.normalized_lookup_key = ?) DESC, i.updated_at DESC, t.title_kind, t.title_text
	`
	args := []any{"%" + lookup + "%", lookup}
	if limit > 0 {
		querySQL += " LIMIT ?"
		args = append(args, limit*4)
	}
	rows, err := m.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CanonRegistryItem{}
	byInstall := map[string]int{}
	for rows.Next() {
		var item CanonRegistryItem
		var matchedTitle string
		var manifestRaw, coverageRaw []byte
		if err := rows.Scan(&item.InstallID, &item.PackID, &item.PackVersion, &item.LifecycleStatus,
			&item.WorkID, &item.EditionRowID, &item.StableWorkID, &item.EditionID, &item.Title,
			&matchedTitle, &item.ReviewStatus, &item.TrustStatus, &manifestRaw, &coverageRaw,
			&item.SourceCount); err != nil {
			return nil, err
		}
		if index, exists := byInstall[item.InstallID]; exists {
			items[index].MatchedTitles = appendUniqueString(items[index].MatchedTitles, matchedTitle)
			continue
		}
		_ = json.Unmarshal(coverageRaw, &item.CoverageReport)
		var manifest map[string]any
		_ = json.Unmarshal(manifestRaw, &manifest)
		item.ConflictCount = len(canonArray(manifest["conflicts"]))
		item.UncertainCount = len(canonArray(manifest["uncertainties"]))
		item.MatchedTitles = []string{matchedTitle}
		byInstall[item.InstallID] = len(items)
		items = append(items, item)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	continuitiesByWork := map[string][]string{}
	for i := range items {
		if cached, ok := continuitiesByWork[items[i].WorkID]; ok {
			items[i].Continuities = cached
			continue
		}
		continuityRows, err := m.db.QueryContext(ctx, `SELECT continuity_key FROM reference_continuities WHERE work_id=? AND status='active' ORDER BY continuity_key`, items[i].WorkID)
		if err != nil {
			return nil, err
		}
		values := []string{}
		for continuityRows.Next() {
			var value string
			if err := continuityRows.Scan(&value); err != nil {
				continuityRows.Close()
				return nil, err
			}
			values = append(values, value)
		}
		if err := continuityRows.Close(); err != nil {
			return nil, err
		}
		continuitiesByWork[items[i].WorkID] = values
		items[i].Continuities = values
	}
	return items, nil
}

func (m *mariadbStore) GetCanonPackDiagnostics(ctx context.Context, installID string) (*CanonPackDiagnostics, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var lifecycle string
	var manifestRaw, coverageRaw []byte
	if err := m.db.QueryRowContext(ctx, `
		SELECT lifecycle_status, manifest_json, coverage_report_json
		FROM canon_pack_installs WHERE install_id = ?
	`, strings.TrimSpace(installID)).Scan(&lifecycle, &manifestRaw, &coverageRaw); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	result := &CanonPackDiagnostics{
		Contract: CanonPackDiagnosticsContract, InstallID: installID,
		LifecycleStatus: lifecycle, Sources: []map[string]any{}, Quality: map[string]any{},
	}
	var manifest map[string]any
	_ = json.Unmarshal(manifestRaw, &manifest)
	_ = json.Unmarshal(coverageRaw, &result.CoverageReport)
	result.Conflicts = canonArray(manifest["conflicts"])
	result.Uncertainties = canonArray(manifest["uncertainties"])

	rows, err := m.db.QueryContext(ctx, `
		SELECT source_key, source_type, COALESCE(source_uri, ''), access_class,
		       retrieved_at, document_sha256, COALESCE(document_id, '')
		FROM reference_source_observations WHERE install_id = ?
		ORDER BY source_type, source_key
	`, strings.TrimSpace(installID))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var key, sourceType, uri, accessClass, hash, documentID string
		var retrievedAt any
		if err := rows.Scan(&key, &sourceType, &uri, &accessClass, &retrievedAt, &hash, &documentID); err != nil {
			rows.Close()
			return nil, err
		}
		result.Sources = append(result.Sources, map[string]any{
			"source_key": key, "source_type": sourceType, "uri": uri,
			"access_class": accessClass, "retrieved_at": retrievedAt,
			"document_sha256": hash, "document_id": documentID,
		})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var itemCount, evidenceBound, multiSource, claimCount, connectedClaims, logicalFacts int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_item_origins WHERE install_id = ?`, installID).Scan(&itemCount); err != nil {
		return nil, err
	}
	if err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT o.item_kind, o.source_item_id
			FROM reference_item_origins o
			JOIN reference_item_evidence ev ON
				(o.item_kind='entity' AND ev.entity_id=o.entity_id) OR
				(o.item_kind='timeline' AND ev.node_id=o.node_id) OR
				(o.item_kind='claim' AND ev.claim_id=o.claim_id)
			WHERE o.install_id = ? GROUP BY o.item_kind, o.source_item_id
		) grounded
	`, installID).Scan(&evidenceBound); err != nil {
		return nil, err
	}
	if err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT o.item_kind, o.source_item_id
			FROM reference_item_origins o
			JOIN reference_item_evidence ev ON
				(o.item_kind='entity' AND ev.entity_id=o.entity_id) OR
				(o.item_kind='timeline' AND ev.node_id=o.node_id) OR
				(o.item_kind='claim' AND ev.claim_id=o.claim_id)
			WHERE o.install_id = ? GROUP BY o.item_kind, o.source_item_id
			HAVING COUNT(DISTINCT ev.source_observation_id) > 1
		) corroborated
	`, installID).Scan(&multiSource); err != nil {
		return nil, err
	}
	if err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN c.subject_entity_id IS NOT NULL THEN 1 ELSE 0 END), 0)
		FROM reference_item_origins o JOIN reference_claims c ON c.claim_id=o.claim_id
		WHERE o.install_id=? AND o.item_kind='claim'
	`, installID).Scan(&claimCount, &connectedClaims); err != nil {
		return nil, err
	}
	if err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT fi.logical_fact_id)
		FROM reference_item_origins o JOIN reference_fact_identities fi ON fi.claim_id=o.claim_id
		WHERE o.install_id=? AND o.item_kind='claim'
	`, installID).Scan(&logicalFacts); err != nil {
		return nil, err
	}
	result.Quality = map[string]any{
		"traceability":                 map[string]any{"status": diagnosticStatus(itemCount == evidenceBound), "items": itemCount, "evidence_bound_items": evidenceBound},
		"evidence":                     map[string]any{"status": diagnosticStatus(evidenceBound > 0 || itemCount == 0), "source_observations": len(result.Sources)},
		"independent_source_agreement": map[string]any{"status": "unassessed", "multi_source_items": multiSource, "single_source_or_unassessed_items": evidenceBound - multiSource, "note": "Multiple observations are not treated as independent until mirror and repost relationships are classified."},
		"duplicate_and_conflict":       map[string]any{"logical_facts": logicalFacts, "conflicts": len(result.Conflicts), "uncertainties": len(result.Uncertainties)},
		"entity_connectivity":          map[string]any{"claims": claimCount, "claims_linked_to_entity": connectedClaims},
		"coverage":                     result.CoverageReport,
	}
	return result, nil
}

func (m *mariadbStore) CreateCanonOverlay(ctx context.Context, input CanonOverlayInput) (*CanonOverlayRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	input.WorkID, input.EditionRowID = strings.TrimSpace(input.WorkID), strings.TrimSpace(input.EditionRowID)
	input.TargetKind, input.TargetID, input.Action = strings.TrimSpace(input.TargetKind), strings.TrimSpace(input.TargetID), strings.TrimSpace(input.Action)
	if input.WorkID == "" || input.EditionRowID == "" || input.TargetID == "" ||
		(input.Action != "supplement" && input.Action != "override" && input.Action != "suppress_for_retrieval" && input.Action != "conflict") {
		return nil, ErrInvalidReference
	}
	var editionWork string
	if err := m.db.QueryRowContext(ctx, `SELECT work_id FROM reference_work_editions WHERE edition_row_id=?`, input.EditionRowID).Scan(&editionWork); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil || editionWork != input.WorkID {
		if err != nil {
			return nil, err
		}
		return nil, ErrReferenceConflict
	}

	rule := &CanonOverlayRule{OverlayRuleID: canonStableID("canon-overlay", input.WorkID, input.EditionRowID, input.TargetKind, input.TargetID, input.Action, input.ReplacementID), WorkID: input.WorkID, EditionRowID: input.EditionRowID, Action: input.Action, Status: "active", Reason: strings.TrimSpace(input.Reason)}
	switch input.TargetKind {
	case "claim":
		if err := m.db.QueryRowContext(ctx, `
			SELECT fi.logical_fact_id FROM reference_fact_identities fi
			JOIN reference_claims c ON c.claim_id=fi.claim_id
			WHERE fi.claim_id=? AND c.work_id=? AND fi.edition_row_id=?
		`, input.TargetID, input.WorkID, input.EditionRowID).Scan(&rule.TargetLogicalFactID); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		}
	case "logical_fact":
		var targetWork, targetEdition string
		if err := m.db.QueryRowContext(ctx, `SELECT work_id, edition_row_id FROM reference_logical_facts WHERE logical_fact_id=?`, input.TargetID).Scan(&targetWork, &targetEdition); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if targetWork != input.WorkID || targetEdition != input.EditionRowID {
			return nil, ErrReferenceConflict
		}
		rule.TargetLogicalFactID = input.TargetID
	case "entity":
		var targetWork string
		if err := m.db.QueryRowContext(ctx, `SELECT work_id FROM reference_entities WHERE entity_id=?`, input.TargetID).Scan(&targetWork); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if targetWork != input.WorkID {
			return nil, ErrReferenceConflict
		}
		rule.TargetEntityID = input.TargetID
	case "timeline":
		var targetWork string
		if err := m.db.QueryRowContext(ctx, `SELECT work_id FROM reference_timeline_nodes WHERE node_id=?`, input.TargetID).Scan(&targetWork); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if targetWork != input.WorkID {
			return nil, ErrReferenceConflict
		}
		rule.TargetNodeID = input.TargetID
	default:
		return nil, ErrInvalidReference
	}
	if input.Action == "override" && strings.TrimSpace(input.ReplacementID) == "" {
		return nil, ErrInvalidReference
	}
	switch strings.TrimSpace(input.ReplacementKind) {
	case "":
	case "claim":
		rule.ReplacementClaimID = strings.TrimSpace(input.ReplacementID)
	case "entity":
		rule.ReplacementEntityID = strings.TrimSpace(input.ReplacementID)
	case "timeline":
		rule.ReplacementNodeID = strings.TrimSpace(input.ReplacementID)
	default:
		return nil, ErrInvalidReference
	}
	if rule.ReplacementClaimID != "" {
		var replacementWork, reviewStatus string
		if err := m.db.QueryRowContext(ctx, `
			SELECT work_id, review_status FROM reference_claims WHERE claim_id=?
		`, rule.ReplacementClaimID).Scan(&replacementWork, &reviewStatus); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if replacementWork != input.WorkID || reviewStatus != "approved" {
			return nil, ErrReferenceConflict
		}
	}
	if rule.ReplacementEntityID != "" {
		var replacementWork, reviewStatus string
		if err := m.db.QueryRowContext(ctx, `SELECT work_id, review_status FROM reference_entities WHERE entity_id=?`, rule.ReplacementEntityID).Scan(&replacementWork, &reviewStatus); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if replacementWork != input.WorkID || reviewStatus != "approved" {
			return nil, ErrReferenceConflict
		}
	}
	if rule.ReplacementNodeID != "" {
		var replacementWork, reviewStatus string
		if err := m.db.QueryRowContext(ctx, `SELECT work_id, review_status FROM reference_timeline_nodes WHERE node_id=?`, rule.ReplacementNodeID).Scan(&replacementWork, &reviewStatus); errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		} else if err != nil {
			return nil, err
		} else if replacementWork != input.WorkID || reviewStatus != "approved" {
			return nil, ErrReferenceConflict
		}
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO reference_overlay_rules
			(overlay_rule_id, work_id, edition_row_id, target_logical_fact_id, target_entity_id,
			 target_node_id, overlay_action, replacement_claim_id, replacement_entity_id,
			 replacement_node_id, rule_status, reason_text)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)
		ON DUPLICATE KEY UPDATE rule_status='active', reason_text=VALUES(reason_text), revision=revision+1
	`, rule.OverlayRuleID, rule.WorkID, rule.EditionRowID, canonNullable(rule.TargetLogicalFactID),
		canonNullable(rule.TargetEntityID), canonNullable(rule.TargetNodeID), rule.Action,
		canonNullable(rule.ReplacementClaimID), canonNullable(rule.ReplacementEntityID),
		canonNullable(rule.ReplacementNodeID), canonNullable(rule.Reason))
	if err != nil {
		return nil, referenceStoreError(err)
	}
	items, err := m.ListCanonOverlays(ctx, input.WorkID, input.EditionRowID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].OverlayRuleID == rule.OverlayRuleID {
			return &items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (m *mariadbStore) ListCanonOverlays(ctx context.Context, workID, editionRowID string) ([]CanonOverlayRule, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := `SELECT overlay_rule_id, work_id, edition_row_id, target_logical_fact_id,
		target_entity_id, target_node_id, overlay_action, replacement_claim_id,
		replacement_entity_id, replacement_node_id, rule_status, reason_text, created_at, updated_at
		FROM reference_overlay_rules WHERE work_id=?`
	args := []any{strings.TrimSpace(workID)}
	if strings.TrimSpace(editionRowID) != "" {
		query += " AND edition_row_id=?"
		args = append(args, strings.TrimSpace(editionRowID))
	}
	query += " ORDER BY updated_at DESC, overlay_rule_id"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CanonOverlayRule{}
	for rows.Next() {
		var item CanonOverlayRule
		var targetFact, targetEntity, targetNode, replacementClaim, replacementEntity, replacementNode, reason sql.NullString
		if err := rows.Scan(&item.OverlayRuleID, &item.WorkID, &item.EditionRowID, &targetFact,
			&targetEntity, &targetNode, &item.Action, &replacementClaim, &replacementEntity,
			&replacementNode, &item.Status, &reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.TargetLogicalFactID, item.TargetEntityID, item.TargetNodeID = targetFact.String, targetEntity.String, targetNode.String
		item.ReplacementClaimID, item.ReplacementEntityID, item.ReplacementNodeID = replacementClaim.String, replacementEntity.String, replacementNode.String
		item.Reason = reason.String
		items = append(items, item)
	}
	return items, rows.Err()
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func diagnosticStatus(ok bool) string {
	if ok {
		return "satisfied"
	}
	return "needs_attention"
}
