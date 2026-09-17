package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type canonPackSourceRef struct {
	ObservationID string
	DocumentID    string
	Hash          string
}

func (m *mariadbStore) InstallCanonPack(ctx context.Context, input CanonPackInstallInput) (*CanonPackInstall, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var manifest map[string]any
	if len(input.ManifestJSON) == 0 || json.Unmarshal(input.ManifestJSON, &manifest) != nil {
		return nil, ErrInvalidReference
	}
	pack, work := canonObject(manifest["pack"]), canonObject(manifest["work"])
	edition := canonObject(work["edition"])
	review, trust := canonObject(manifest["review"]), canonObject(manifest["trust"])
	packID, packVersion := canonString(pack["id"]), canonString(pack["version"])
	stableWorkID, editionID := canonString(work["stable_id"]), canonString(edition["edition_id"])
	title := canonString(canonObject(work["original_title"])["text"])
	if canonString(manifest["contract"]) != "canon-pack-manifest.v1" ||
		packID == "" || packVersion == "" || stableWorkID == "" || editionID == "" || title == "" ||
		canonString(review["status"]) != "approved" || !canonSHA256(input.ManifestSHA256) ||
		!canonSHA256(input.ArchiveSHA256) {
		return nil, ErrInvalidReference
	}
	installID, err := canonRandomID()
	if err != nil {
		return nil, err
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	workID, editionRowID, err := ensureCanonWorkEdition(ctx, tx, stableWorkID, editionID, title, work, edition)
	if err != nil {
		return nil, referenceStoreError(err)
	}
	continuities := canonStrings(work["continuity_ids"])
	if len(continuities) == 0 {
		return nil, ErrInvalidReference
	}
	continuityRows := make(map[string]string, len(continuities))
	for _, continuityKey := range continuities {
		continuityID := canonStableID("canon-continuity", stableWorkID, continuityKey)
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO reference_continuities
				(continuity_id, work_id, continuity_key, label, status, metadata_json)
			VALUES (?, ?, ?, ?, 'active', ?)
		`, continuityID, workID, continuityKey, continuityKey, canonJSON(map[string]any{
			"stable_continuity_id": continuityKey, "origin": "canon_pack",
		})); err != nil {
			return nil, referenceStoreError(err)
		}
		continuityRows[continuityKey] = continuityID
	}

	var generation uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(install_generation), 0) + 1
		FROM canon_pack_installs WHERE pack_id = ? AND pack_version = ? FOR UPDATE
	`, packID, packVersion).Scan(&generation); err != nil {
		return nil, err
	}
	validationJSON := canonValidationJSON(input.ValidationReportJSON, input.ArchiveSHA256)
	coverageJSON := canonJSON(canonObject(manifest["coverage_report"]))
	if _, err := tx.ExecContext(ctx, `
		UPDATE canon_pack_installs SET lifecycle_status = 'inactive'
		WHERE pack_id = ? AND edition_row_id = ? AND lifecycle_status = 'active'
	`, packID, editionRowID); err != nil {
		return nil, referenceStoreError(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO canon_pack_installs
			(install_id, pack_id, pack_version, install_generation, manifest_contract,
			 manifest_sha256, manifest_json, work_id, edition_row_id, pack_status,
			 review_status, trust_status, lifecycle_status, validation_report_json,
			 coverage_report_json, activated_at)
		VALUES (?, ?, ?, ?, 'canon-pack-manifest.v1', ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, CURRENT_TIMESTAMP(3))
	`, installID, packID, packVersion, generation, input.ManifestSHA256, string(input.ManifestJSON),
		workID, editionRowID, canonString(pack["status"]), canonString(review["status"]),
		canonString(trust["status"]), validationJSON, coverageJSON); err != nil {
		return nil, referenceStoreError(err)
	}

	if err := insertCanonTitles(ctx, tx, workID, editionRowID, work); err != nil {
		return nil, err
	}
	sources, err := insertCanonSources(ctx, tx, installID, workID, editionRowID, manifest, continuityRows)
	if err != nil {
		return nil, err
	}
	if err := insertCanonContent(ctx, tx, installID, packID, packVersion, workID, editionRowID, manifest, continuityRows, sources); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return m.GetCanonPackInstall(ctx, installID)
}

func ensureCanonWorkEdition(ctx context.Context, tx *sql.Tx, stableWorkID, editionID, title string, work, edition map[string]any) (string, string, error) {
	var editionRowID, workID string
	err := tx.QueryRowContext(ctx, `
		SELECT edition_row_id, work_id FROM reference_work_editions
		WHERE stable_work_id = ? AND edition_id = ? FOR UPDATE
	`, stableWorkID, editionID).Scan(&editionRowID, &workID)
	if err == nil {
		return workID, editionRowID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	workID = canonStableID("canon-work", stableWorkID)
	editionRowID = canonStableID("canon-edition", stableWorkID, editionID)
	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO reference_works
			(work_id, title, work_type, default_language, status, metadata_json)
		VALUES (?, ?, 'canon', ?, 'active', ?)
	`, workID, title, canonString(work["original_language"]), canonJSON(map[string]any{"stable_work_id": stableWorkID, "origin": "canon_pack"})); err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reference_work_editions
			(edition_row_id, work_id, stable_work_id, edition_id, original_language,
			 edition_language, edition_label, edition_status, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?)
	`, editionRowID, workID, stableWorkID, editionID, canonString(work["original_language"]),
		canonString(edition["language"]), canonString(edition["label"]), canonJSON(edition)); err != nil {
		return "", "", err
	}
	return workID, editionRowID, nil
}

func insertCanonTitles(ctx context.Context, tx *sql.Tx, workID, editionRowID string, work map[string]any) error {
	type titleEntry struct{ Kind, Text, Language string }
	titles := []titleEntry{}
	original := canonObject(work["original_title"])
	titles = append(titles, titleEntry{"original", canonString(original["text"]), canonString(original["language"])})
	for _, raw := range canonArray(work["translated_titles"]) {
		v := canonObject(raw)
		titles = append(titles, titleEntry{"translated", canonString(v["text"]), canonString(v["language"])})
	}
	for _, raw := range canonArray(work["aliases"]) {
		v := canonObject(raw)
		titles = append(titles, titleEntry{"alias", canonString(v["text"]), canonString(v["language"])})
	}
	for _, item := range titles {
		lookup := canonNormalize(item.Text)
		if lookup == "" {
			continue
		}
		digest := canonHash(lookup)
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO reference_work_titles
				(title_row_id, work_id, edition_row_id, title_kind, title_text, language_code,
				 normalization_contract, normalized_lookup_key, normalized_lookup_digest, edition_scope_key)
			VALUES (?, ?, ?, ?, ?, ?, 'canon_title_normalization.v1', ?, ?, ?)
		`, canonStableID("canon-title", workID, editionRowID, item.Kind, item.Language, digest), workID,
			editionRowID, item.Kind, item.Text, item.Language, lookup, digest, editionRowID); err != nil {
			return referenceStoreError(err)
		}
	}
	return nil
}

func insertCanonSources(ctx context.Context, tx *sql.Tx, installID, workID, editionRowID string, manifest map[string]any, continuityRows map[string]string) (map[string]canonPackSourceRef, error) {
	result := map[string]canonPackSourceRef{}
	for _, raw := range canonArray(manifest["sources"]) {
		source := canonObject(raw)
		sourceID, documentHash := canonString(source["id"]), canonString(source["document_sha256"])
		retrievedAt, err := time.Parse(time.RFC3339, canonString(source["retrieved_at"]))
		if err != nil {
			return nil, ErrInvalidReference
		}
		for continuityKey, continuityID := range continuityRows {
			documentID := canonStableID("canon-document", workID, continuityID, documentHash)
			provenance := canonJSON(map[string]any{"origin": "canon_pack", "source_id": sourceID, "title": canonString(source["title"])})
			if _, err := tx.ExecContext(ctx, `
				INSERT IGNORE INTO reference_documents
					(document_id, work_id, continuity_id, source_type, source_uri, content_hash,
					 raw_retention, raw_text, import_status, provenance_json)
				VALUES (?, ?, ?, ?, ?, ?, 'none', NULL, 'metadata_only', ?)
			`, documentID, workID, continuityID, canonString(source["source_type"]), canonNullable(canonString(source["uri"])), documentHash, provenance); err != nil {
				return nil, referenceStoreError(err)
			}
			if err := tx.QueryRowContext(ctx, `
				SELECT document_id FROM reference_documents
				WHERE work_id = ? AND continuity_id = ? AND content_hash = ?
			`, workID, continuityID, documentHash).Scan(&documentID); err != nil {
				return nil, err
			}
			observationID := canonStableID("canon-observation", installID, continuityKey, sourceID)
			sourceKey := sourceID + ":" + canonHash(continuityKey)[:16]
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO reference_source_observations
					(observation_id, work_id, edition_row_id, continuity_id, origin_kind,
					 install_id, source_key, source_type, source_uri, license_json, access_class,
					 retrieved_at, document_sha256, document_id, provenance_json)
				VALUES (?, ?, ?, ?, 'canon_pack', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, observationID, workID, editionRowID, continuityID, installID, sourceKey,
				canonString(source["source_type"]), canonNullable(canonString(source["uri"])),
				canonJSON(canonObject(source["license"])), canonString(source["access_class"]),
				retrievedAt, documentHash, documentID, provenance); err != nil {
				return nil, referenceStoreError(err)
			}
			result[continuityKey+"\x00"+sourceID] = canonPackSourceRef{observationID, documentID, documentHash}
		}
	}
	return result, nil
}

func insertCanonContent(ctx context.Context, tx *sql.Tx, installID, packID, packVersion, workID, editionRowID string, manifest map[string]any, continuityRows map[string]string, sources map[string]canonPackSourceRef) error {
	content := canonObject(manifest["content"])
	entityIDs, entityNames := map[string]string{}, map[string]string{}
	for _, kind := range []string{"entities", "locations", "factions", "settings"} {
		for _, raw := range canonArray(content[kind]) {
			record := canonObject(raw)
			if canonString(record["review_state"]) == "rejected" {
				continue
			}
			for _, continuityKey := range canonStrings(record["continuity_ids"]) {
				continuityID := continuityRows[continuityKey]
				if continuityID == "" {
					return ErrInvalidReference
				}
				localID := canonString(record["id"])
				entityID := canonStableID("canon-item", installID, continuityKey, kind, localID)
				name := canonString(canonObject(record["name"])["text"])
				entityType := strings.TrimSuffix(kind, "s")
				if kind == "entities" {
					entityType = canonString(record["kind"])
				}
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO reference_entities
						(entity_id, work_id, continuity_id, entity_type, canonical_name,
						 description_text, metadata_json, review_status, review_source,
						 review_reason, reviewed_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'canon_pack', ?, CURRENT_TIMESTAMP(3))
				`, entityID, workID, continuityID, entityType, name, canonNullable(canonString(record["summary"])),
					canonJSON(record), canonReviewState(record), "installed from "+packID+"@"+packVersion); err != nil {
					return referenceStoreError(err)
				}
				if err := insertCanonOrigin(ctx, tx, installID, workID, editionRowID, "entity", entityID, localID, canonReviewState(record)); err != nil {
					return err
				}
				if err := insertCanonEvidence(ctx, tx, "entity", entityID, continuityKey, record, sources); err != nil {
					return err
				}
				for _, aliasRaw := range canonArray(record["aliases"]) {
					alias := canonObject(aliasRaw)
					text := canonString(alias["text"])
					if text == "" {
						continue
					}
					if _, err := tx.ExecContext(ctx, `
						INSERT IGNORE INTO reference_entity_aliases
							(work_id, continuity_id, entity_id, alias_text, normalized_alias, language_code)
						VALUES (?, ?, ?, ?, ?, ?)
					`, workID, continuityID, entityID, text, canonNormalize(text), canonString(alias["language"])); err != nil {
						return referenceStoreError(err)
					}
				}
				entityIDs[continuityKey+"\x00"+localID] = entityID
				entityNames[localID] = name
			}
		}
	}

	nodeIDs := map[string]string{}
	for index, raw := range canonArray(content["events"]) {
		record := canonObject(raw)
		if canonString(record["review_state"]) == "rejected" {
			continue
		}
		for _, continuityKey := range canonStrings(record["continuity_ids"]) {
			continuityID, localID := continuityRows[continuityKey], canonString(record["id"])
			nodeID := canonStableID("canon-item", installID, continuityKey, "events", localID)
			name := canonString(canonObject(record["name"])["text"])
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO reference_timeline_nodes
					(node_id, work_id, continuity_id, node_key, label, ordinal_value,
					 branch_key, node_kind, metadata_json, review_status, review_source,
					 review_reason, reviewed_at)
				VALUES (?, ?, ?, ?, ?, ?, 'main', 'event', ?, ?, 'canon_pack', ?, CURRENT_TIMESTAMP(3))
			`, nodeID, workID, continuityID, installID+":"+localID, name, int64((index+1)*10),
				canonJSON(record), canonReviewState(record), "installed from "+packID+"@"+packVersion); err != nil {
				return referenceStoreError(err)
			}
			if err := insertCanonOrigin(ctx, tx, installID, workID, editionRowID, "timeline", nodeID, localID, canonReviewState(record)); err != nil {
				return err
			}
			if err := insertCanonEvidence(ctx, tx, "timeline", nodeID, continuityKey, record, sources); err != nil {
				return err
			}
			nodeIDs[continuityKey+"\x00"+localID] = nodeID
			entityNames[localID] = name
		}
	}

	for _, kind := range []string{"relations", "claims"} {
		for _, raw := range canonArray(content[kind]) {
			record := canonObject(raw)
			if canonString(record["review_state"]) == "rejected" {
				continue
			}
			for _, continuityKey := range canonStrings(record["continuity_ids"]) {
				continuityID, localID := continuityRows[continuityKey], canonString(record["id"])
				claimType, statement := canonString(record["claim_type"]), canonString(record["statement"])
				subjectIDs := canonStrings(record["subject_ids"])
				if kind == "relations" {
					claimType = "relation:" + canonString(record["predicate"])
					subject, object := canonString(record["subject_id"]), canonString(record["object_id"])
					statement = fmt.Sprintf("%s %s %s", canonDisplayName(entityNames, subject), canonString(record["predicate"]), canonDisplayName(entityNames, object))
					subjectIDs = []string{subject, object}
				}
				firstSource, err := firstCanonEvidenceSource(continuityKey, record, sources)
				if err != nil {
					return err
				}
				subjectEntityID := ""
				for _, subject := range subjectIDs {
					if id := entityIDs[continuityKey+"\x00"+subject]; id != "" {
						subjectEntityID = id
						break
					}
				}
				claimID, err := ensureCanonLogicalClaim(ctx, tx, installID, kind, localID, packID, packVersion,
					workID, editionRowID, continuityID, firstSource.DocumentID, claimType, statement,
					subjectEntityID, record)
				if err != nil {
					return err
				}
				if err := insertCanonOrigin(ctx, tx, installID, workID, editionRowID, "claim", claimID, localID, canonReviewState(record)); err != nil {
					return err
				}
				if err := insertCanonEvidence(ctx, tx, "claim", claimID, continuityKey, record, sources); err != nil {
					return err
				}
			}
		}
	}
	_ = nodeIDs
	return nil
}

func ensureCanonLogicalClaim(ctx context.Context, tx *sql.Tx, installID, kind, localID, packID, packVersion,
	workID, editionRowID, continuityID, documentID, claimType, statement, subjectEntityID string,
	record map[string]any) (string, error) {
	applicabilityDigest, exactFingerprint := canonFactIdentity(editionRowID, continuityID, claimType, statement)
	var claimID, logicalFactID string
	err := tx.QueryRowContext(ctx, `
		SELECT fi.claim_id, fi.logical_fact_id
		FROM reference_fact_identities fi
		WHERE fi.fingerprint_contract='canon_fact_exact.v1' AND fi.exact_fingerprint=?
		  AND fi.edition_row_id=? AND fi.continuity_id=?
		FOR UPDATE
	`, exactFingerprint, editionRowID, continuityID).Scan(&claimID, &logicalFactID)
	if err == nil {
		if canonReviewState(record) == "approved" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE reference_claims SET review_status='approved', review_source='canon_pack',
					review_reason=?, reviewed_at=CURRENT_TIMESTAMP(3)
				WHERE claim_id=? AND review_status <> 'approved'
			`, "corroborated by "+packID+"@"+packVersion, claimID); err != nil {
				return "", referenceStoreError(err)
			}
		}
		return claimID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	claimID = canonStableID("canon-item", installID, continuityID, kind, localID)
	logicalFactID = canonStableID("canon-logical-fact", editionRowID, continuityID, exactFingerprint)
	metadata := map[string]any{
		"manifest_record": record, "source_item_id": localID, "install_id": installID,
		"exact_fingerprint": exactFingerprint, "logical_fact_id": logicalFactID,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reference_claims
			(claim_id, work_id, continuity_id, document_id, claim_type,
			 subject_entity_id, claim_text, temporal_scope, branch_key,
			 knowledge_scope, confidence, review_status, review_source,
			 review_reason, reviewed_at, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'bounded', 'main', 'public_world', 1,
			 ?, 'canon_pack', ?, CURRENT_TIMESTAMP(3), ?)
	`, claimID, workID, continuityID, documentID, claimType, canonNullable(subjectEntityID),
		statement, canonReviewState(record), "installed from "+packID+"@"+packVersion,
		canonJSON(metadata)); err != nil {
		return "", referenceStoreError(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reference_logical_facts
			(logical_fact_id, work_id, edition_row_id, continuity_id, applicability_scope_digest, fact_status)
		VALUES (?, ?, ?, ?, ?, 'active')
	`, logicalFactID, workID, editionRowID, continuityID, applicabilityDigest); err != nil {
		return "", referenceStoreError(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reference_fact_identities
			(claim_id, fingerprint_contract, exact_fingerprint, logical_fact_id,
			 equivalence_status, equivalence_basis, edition_row_id, continuity_id,
			 applicability_scope_digest)
		VALUES (?, 'canon_fact_exact.v1', ?, ?, 'exact', 'normalized_exact', ?, ?, ?)
	`, claimID, exactFingerprint, logicalFactID, editionRowID, continuityID, applicabilityDigest); err != nil {
		return "", referenceStoreError(err)
	}
	return claimID, nil
}

func canonFactIdentity(editionRowID, continuityID, claimType, statement string) (string, string) {
	applicabilityDigest := canonHash(strings.Join([]string{editionRowID, continuityID, "bounded", "main", "public_world"}, "\x00"))
	exactFingerprint := canonHash(strings.Join([]string{editionRowID, continuityID, canonNormalize(claimType), canonNormalize(statement), applicabilityDigest}, "\x00"))
	return applicabilityDigest, exactFingerprint
}

func insertCanonOrigin(ctx context.Context, tx *sql.Tx, installID, workID, editionRowID, kind, targetID, sourceItemID, reviewState string) error {
	node, entity, claim := any(nil), any(nil), any(nil)
	switch kind {
	case "timeline":
		node = targetID
	case "entity":
		entity = targetID
	case "claim":
		claim = targetID
	default:
		return ErrInvalidReference
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO reference_item_origins
			(origin_membership_id, work_id, edition_row_id, item_kind, node_id,
			 entity_id, claim_id, origin_kind, origin_owner_id, install_id,
			 source_item_id, review_state)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'canon_pack', ?, ?, ?, ?)
	`, canonStableID("canon-origin", installID, kind, sourceItemID, targetID), workID, editionRowID,
		kind, node, entity, claim, installID, installID, sourceItemID, reviewState)
	return referenceStoreError(err)
}

func insertCanonEvidence(ctx context.Context, tx *sql.Tx, kind, targetID, continuityKey string, record map[string]any, sources map[string]canonPackSourceRef) error {
	for _, raw := range canonArray(record["evidence"]) {
		evidence := canonObject(raw)
		source := sources[continuityKey+"\x00"+canonString(evidence["source_id"])]
		if source.ObservationID == "" {
			return ErrInvalidReference
		}
		locatorJSON := canonJSON(canonObject(evidence["locator"]))
		node, entity, claim := any(nil), any(nil), any(nil)
		switch kind {
		case "timeline":
			node = targetID
		case "entity":
			entity = targetID
		case "claim":
			claim = targetID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO reference_item_evidence
				(evidence_edge_id, item_kind, node_id, entity_id, claim_id,
				 source_observation_id, document_hash_contract, document_sha256,
				 locator_json, locator_digest, evidence_state)
			VALUES (?, ?, ?, ?, ?, ?, 'source_bytes_sha256.v1', ?, ?, ?, 'active')
		`, canonStableID("canon-evidence", targetID, source.ObservationID, canonHash(locatorJSON)),
			kind, node, entity, claim, source.ObservationID, source.Hash, locatorJSON, canonHash(locatorJSON)); err != nil {
			return referenceStoreError(err)
		}
	}
	return nil
}

func firstCanonEvidenceSource(continuityKey string, record map[string]any, sources map[string]canonPackSourceRef) (canonPackSourceRef, error) {
	evidence := canonArray(record["evidence"])
	if len(evidence) == 0 {
		return canonPackSourceRef{}, ErrInvalidReference
	}
	source := sources[continuityKey+"\x00"+canonString(canonObject(evidence[0])["source_id"])]
	if source.DocumentID == "" {
		return canonPackSourceRef{}, ErrInvalidReference
	}
	return source, nil
}

func (m *mariadbStore) ListCanonPackInstalls(ctx context.Context, lifecycle string) ([]CanonPackInstall, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	query := canonPackSelectSQL()
	args := []any{}
	if strings.TrimSpace(lifecycle) != "" {
		query += " WHERE i.lifecycle_status = ?"
		args = append(args, strings.TrimSpace(lifecycle))
	}
	query += " ORDER BY i.updated_at DESC, i.pack_id, i.install_generation DESC"
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []CanonPackInstall{}
	for rows.Next() {
		item, err := scanCanonPackInstall(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (m *mariadbStore) GetCanonPackInstall(ctx context.Context, installID string) (*CanonPackInstall, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	row := m.db.QueryRowContext(ctx, canonPackSelectSQL()+" WHERE i.install_id = ?", strings.TrimSpace(installID))
	item, err := scanCanonPackInstall(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (m *mariadbStore) SetCanonPackLifecycle(ctx context.Context, installID, action string) (*CanonPackLifecycleResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	action = strings.TrimSpace(action)
	if action != "activate" && action != "deactivate" && action != "remove" && action != "rollback" {
		return nil, ErrInvalidReference
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var packID, editionRowID, status string
	if err := tx.QueryRowContext(ctx, `
		SELECT pack_id, edition_row_id, lifecycle_status FROM canon_pack_installs
		WHERE install_id = ? FOR UPDATE
	`, strings.TrimSpace(installID)).Scan(&packID, &editionRowID, &status); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	result := &CanonPackLifecycleResult{Contract: CanonPackLifecycleContract, Action: action, InstallID: installID}
	switch action {
	case "activate", "rollback":
		if status == "removed" || status == "failed" {
			return nil, ErrReferenceConflict
		}
		err := tx.QueryRowContext(ctx, `
			SELECT install_id FROM canon_pack_installs
			WHERE pack_id = ? AND edition_row_id = ? AND lifecycle_status = 'active' AND install_id <> ?
			LIMIT 1 FOR UPDATE
		`, packID, editionRowID, installID).Scan(&result.PreviousInstallID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE canon_pack_installs SET lifecycle_status = 'inactive'
			WHERE pack_id = ? AND edition_row_id = ? AND lifecycle_status = 'active' AND install_id <> ?
		`, packID, editionRowID, installID); err != nil {
			return nil, referenceStoreError(err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE canon_pack_installs SET lifecycle_status = 'active', activated_at = CURRENT_TIMESTAMP(3), removed_at = NULL
			WHERE install_id = ?
		`, installID); err != nil {
			return nil, referenceStoreError(err)
		}
		result.LifecycleStatus = "active"
	case "deactivate":
		if status == "removed" {
			return nil, ErrReferenceConflict
		}
		if _, err := tx.ExecContext(ctx, `UPDATE canon_pack_installs SET lifecycle_status = 'inactive' WHERE install_id = ?`, installID); err != nil {
			return nil, referenceStoreError(err)
		}
		result.LifecycleStatus = "inactive"
	case "remove":
		if _, err := tx.ExecContext(ctx, `
			UPDATE canon_pack_installs SET lifecycle_status = 'removed', removed_at = CURRENT_TIMESTAMP(3)
			WHERE install_id = ?
		`, installID); err != nil {
			return nil, referenceStoreError(err)
		}
		result.LifecycleStatus = "removed"
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func canonPackSelectSQL() string {
	return `SELECT i.install_id, i.pack_id, i.pack_version, i.install_generation,
		i.work_id, i.edition_row_id, e.stable_work_id, e.edition_id, w.title,
		i.pack_status, i.review_status, i.trust_status, i.lifecycle_status,
		i.manifest_sha256, i.validation_report_json, i.coverage_report_json,
		i.installed_at, i.activated_at, i.removed_at, i.updated_at,
		(SELECT COUNT(*) FROM reference_item_origins o WHERE o.install_id = i.install_id AND o.item_kind = 'entity'),
		(SELECT COUNT(*) FROM reference_item_origins o WHERE o.install_id = i.install_id AND o.item_kind = 'timeline'),
		(SELECT COUNT(*) FROM reference_item_origins o WHERE o.install_id = i.install_id AND o.item_kind = 'claim')
	FROM canon_pack_installs i
	JOIN reference_work_editions e ON e.edition_row_id = i.edition_row_id
	JOIN reference_works w ON w.work_id = i.work_id`
}

type canonScanner interface{ Scan(...any) error }

func scanCanonPackInstall(row canonScanner) (*CanonPackInstall, error) {
	item := &CanonPackInstall{Contract: CanonPackLifecycleContract, RecordCounts: map[string]int{}}
	var validationRaw, coverageRaw []byte
	var activated, removed sql.NullTime
	var entityCount, timelineCount, claimCount int
	if err := row.Scan(&item.InstallID, &item.PackID, &item.PackVersion, &item.InstallGeneration,
		&item.WorkID, &item.EditionRowID, &item.StableWorkID, &item.EditionID, &item.Title,
		&item.PackStatus, &item.ReviewStatus, &item.TrustStatus, &item.LifecycleStatus,
		&item.ManifestSHA256, &validationRaw, &coverageRaw, &item.InstalledAt, &activated,
		&removed, &item.UpdatedAt, &entityCount, &timelineCount, &claimCount); err != nil {
		return nil, err
	}
	if activated.Valid {
		item.ActivatedAt = &activated.Time
	}
	if removed.Valid {
		item.RemovedAt = &removed.Time
	}
	var validation map[string]any
	_ = json.Unmarshal(validationRaw, &validation)
	item.ArchiveSHA256 = canonString(validation["archive_sha256"])
	_ = json.Unmarshal(coverageRaw, &item.CoverageReport)
	item.RecordCounts["entities"] = entityCount
	item.RecordCounts["timeline"] = timelineCount
	item.RecordCounts["claims"] = claimCount
	return item, nil
}

func canonObject(v any) map[string]any { m, _ := v.(map[string]any); return m }
func canonArray(v any) []any           { a, _ := v.([]any); return a }
func canonString(v any) string         { s, _ := v.(string); return strings.TrimSpace(s) }
func canonStrings(v any) []string {
	out := []string{}
	for _, raw := range canonArray(v) {
		if value := canonString(raw); value != "" {
			out = append(out, value)
		}
	}
	return out
}
func canonJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
func canonNullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}
func canonHash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}
func canonSHA256(v string) bool {
	if len(v) != 64 || strings.ToLower(v) != v {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
func canonStableID(namespace string, values ...string) string {
	sum := sha256.Sum256([]byte(namespace + "\x00" + strings.Join(values, "\x00")))
	b := append([]byte(nil), sum[:16]...)
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}
func canonRandomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32], nil
}
func canonNormalize(v string) string { return strings.ToLower(strings.Join(strings.Fields(v), " ")) }
func canonReviewState(record map[string]any) string {
	if canonString(record["review_state"]) == "approved" {
		return "approved"
	}
	return "pending"
}
func canonDisplayName(names map[string]string, id string) string {
	if names[id] != "" {
		return names[id]
	}
	return id
}
func canonValidationJSON(raw []byte, archiveSHA string) string {
	value := map[string]any{}
	_ = json.Unmarshal(raw, &value)
	value["archive_sha256"] = archiveSHA
	return canonJSON(value)
}

func canonPackActiveOriginClause(kind, idColumn, outerID string) string {
	return ` AND (
		NOT EXISTS (
			SELECT 1 FROM reference_item_origins origin_any
			WHERE origin_any.item_kind = '` + kind + `' AND origin_any.` + idColumn + ` = ` + outerID + `
		) OR EXISTS (
			SELECT 1 FROM reference_item_origins origin_local
			WHERE origin_local.item_kind = '` + kind + `' AND origin_local.` + idColumn + ` = ` + outerID + `
			  AND origin_local.origin_kind <> 'canon_pack' AND origin_local.review_state = 'approved'
		) OR EXISTS (
			SELECT 1 FROM reference_item_origins origin_pack
			JOIN canon_pack_installs active_pack ON active_pack.install_id = origin_pack.install_id
			WHERE origin_pack.item_kind = '` + kind + `' AND origin_pack.` + idColumn + ` = ` + outerID + `
			  AND origin_pack.origin_kind = 'canon_pack' AND origin_pack.review_state = 'approved'
			  AND active_pack.lifecycle_status = 'active'
		)
	)`
}
