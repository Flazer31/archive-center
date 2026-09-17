package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type lorebookReferenceScopeRow struct {
	ScopeID            int64
	EnabledModulesJSON string
	ScopeIdentityJSON  string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func canonicalLorebookModuleIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func lorebookScopeJSON(scope LorebookReferenceScope) (string, string, error) {
	scope.ChatSessionID = strings.TrimSpace(scope.ChatSessionID)
	scope.EnabledModuleIDs = canonicalLorebookModuleIDs(scope.EnabledModuleIDs)
	modules, err := json.Marshal(scope.EnabledModuleIDs)
	if err != nil {
		return "", "", err
	}
	identity, err := json.Marshal(scope)
	if err != nil {
		return "", "", err
	}
	return string(modules), string(identity), nil
}

func nullableInt64Pointer(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableBoolPointer(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableFloatPointer(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func normalizeLorebookSearchText(entry LorebookReferenceEntryObservation) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.Join([]string{
		entry.Key, entry.SecondKey, entry.Comment, entry.Content,
	}, "\n"))), " ")
}

func lorebookDiagnosticContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func ValidateLorebookReferenceSnapshot(item *LorebookReferenceSnapshot) error {
	if item == nil || strings.TrimSpace(item.Scope.ChatSessionID) == "" || strings.TrimSpace(item.SnapshotID) == "" {
		return ErrInvalidLorebookReference
	}
	if strings.TrimSpace(item.ContractVersion) != LorebookReferenceSnapshotContractV1 {
		return ErrInvalidLorebookReference
	}
	if item.ConsentState != LorebookConsentActive && item.ConsentState != LorebookConsentRevoked {
		return ErrInvalidLorebookReference
	}
	switch item.ObservationState {
	case LorebookObservationObserved:
		if item.ConsentState == LorebookConsentActive && !item.CompleteSnapshot {
			return ErrInvalidLorebookReference
		}
		if item.ConsentState == LorebookConsentActive && item.CompleteSnapshot &&
			(item.Scope.CharacterIndex == nil || item.Scope.ChatIndex == nil || !item.Scope.EnabledModulesObserved) {
			return ErrInvalidLorebookReference
		}
	case LorebookObservationPartial:
		if item.CompleteSnapshot {
			return ErrInvalidLorebookReference
		}
	case LorebookObservationUnavailable:
		if item.CompleteSnapshot || len(item.Entries) != 0 {
			return ErrInvalidLorebookReference
		}
	default:
		return ErrInvalidLorebookReference
	}
	return nil
}

func lockLorebookReferenceSession(ctx context.Context, tx *sql.Tx, chatSessionID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO lorebook_reference_session_locks (chat_session_id)
		VALUES (?)
		ON DUPLICATE KEY UPDATE chat_session_id = VALUES(chat_session_id)
	`, strings.TrimSpace(chatSessionID))
	return err
}

func latestLorebookReferenceAuthorityObservation(ctx context.Context, tx *sql.Tx, scopeID int64) (time.Time, bool, error) {
	var observedAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT observed_at
		FROM lorebook_reference_snapshots
		WHERE scope_id = ?
		  AND (consent_state = ? OR
		       (consent_state = ? AND observation_state = ? AND complete_snapshot = TRUE))
		ORDER BY observed_at DESC, created_at DESC, snapshot_id DESC
		LIMIT 1
	`, scopeID, LorebookConsentRevoked, LorebookConsentActive, LorebookObservationObserved).Scan(&observedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return observedAt.UTC(), true, nil
}

func findLorebookReferenceScope(ctx context.Context, queryer mariaQueryer, scope LorebookReferenceScope, forUpdate bool) (*lorebookReferenceScopeRow, error) {
	modulesJSON, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT scope_id, enabled_modules_json, scope_identity_json, created_at, updated_at
		FROM lorebook_reference_scopes
		WHERE chat_session_id = ? AND character_index <=> ? AND chat_index <=> ?
		ORDER BY scope_id ASC`
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := queryer.QueryContext(ctx, query, strings.TrimSpace(scope.ChatSessionID), nullableInt64Pointer(scope.CharacterIndex), nullableInt64Pointer(scope.ChatIndex))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item lorebookReferenceScopeRow
		if err := rows.Scan(&item.ScopeID, &item.EnabledModulesJSON, &item.ScopeIdentityJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		var storedModules []string
		if err := json.Unmarshal([]byte(item.EnabledModulesJSON), &storedModules); err != nil {
			return nil, err
		}
		storedCanonical, _ := json.Marshal(canonicalLorebookModuleIDs(storedModules))
		var storedScope LorebookReferenceScope
		if err := json.Unmarshal([]byte(item.ScopeIdentityJSON), &storedScope); err != nil {
			return nil, err
		}
		_, storedIdentityJSON, identityErr := lorebookScopeJSON(storedScope)
		if string(storedCanonical) == modulesJSON && identityErr == nil && storedIdentityJSON == identityJSON {
			return &item, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNotFound
}

func (m *mariadbStore) ApplyLorebookReferenceSnapshot(ctx context.Context, item *LorebookReferenceSnapshot) (*LorebookReferenceSnapshotResult, error) {
	if err := ValidateLorebookReferenceSnapshot(item); err != nil {
		return nil, err
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	modulesJSON, identityJSON, err := lorebookScopeJSON(item.Scope)
	if err != nil {
		return nil, err
	}
	provenanceJSON := strings.TrimSpace(item.ProvenanceJSON)
	if provenanceJSON == "" {
		provenanceJSON = "{}"
	}
	if !json.Valid([]byte(provenanceJSON)) {
		return nil, ErrInvalidLorebookReference
	}
	observedAt := item.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockLorebookReferenceSession(ctx, tx, item.Scope.ChatSessionID); err != nil {
		return nil, err
	}

	scopeRow, err := findLorebookReferenceScope(ctx, tx, item.Scope, true)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	scopeCreated := err == ErrNotFound
	if scopeCreated {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO lorebook_reference_scopes
				(chat_session_id, character_index, chat_index, enabled_modules_json, scope_identity_json)
			VALUES (?, ?, ?, ?, ?)
		`, strings.TrimSpace(item.Scope.ChatSessionID), nullableInt64Pointer(item.Scope.CharacterIndex), nullableInt64Pointer(item.Scope.ChatIndex), modulesJSON, identityJSON)
		if insertErr != nil {
			return nil, insertErr
		}
		scopeID, insertErr := result.LastInsertId()
		if insertErr != nil {
			return nil, insertErr
		}
		scopeRow = &lorebookReferenceScopeRow{ScopeID: scopeID, EnabledModulesJSON: modulesJSON, ScopeIdentityJSON: identityJSON}
	}
	authoritativeObservation := item.ConsentState == LorebookConsentRevoked ||
		(item.ConsentState == LorebookConsentActive && item.ObservationState == LorebookObservationObserved && item.CompleteSnapshot)
	outOfOrder := false
	if authoritativeObservation && !scopeCreated {
		latestObservedAt, found, latestErr := latestLorebookReferenceAuthorityObservation(ctx, tx, scopeRow.ScopeID)
		if latestErr != nil {
			return nil, latestErr
		}
		outOfOrder = found && observedAt.Before(latestObservedAt)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lorebook_reference_snapshots
			(snapshot_id, scope_id, contract_version, consent_state, observation_state,
			 complete_snapshot, entry_count, provenance_json, observed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, strings.TrimSpace(item.SnapshotID), scopeRow.ScopeID, item.ContractVersion, item.ConsentState,
		item.ObservationState, item.CompleteSnapshot, len(item.Entries), provenanceJSON, observedAt); err != nil {
		return nil, err
	}

	result := &LorebookReferenceSnapshotResult{
		ScopeID: scopeRow.ScopeID, SnapshotID: strings.TrimSpace(item.SnapshotID),
		ObservationState: item.ObservationState, ObservedEntryCount: len(item.Entries),
		LifecycleAction: "observation_recorded",
	}
	entryLifecycle := LorebookLifecyclePartial
	entryCurrent := false
	if outOfOrder {
		entryLifecycle = LorebookLifecycleStale
		result.LifecycleAction = "out_of_order_observation_recorded"
	} else if item.ConsentState == LorebookConsentRevoked {
		changed, err := tx.ExecContext(ctx, `
			UPDATE lorebook_reference_entries
			SET is_current = FALSE, lifecycle_state = ?, last_seen_at = ?
			WHERE scope_id = ? AND is_current = TRUE
		`, LorebookLifecycleConsentRevoked, observedAt, scopeRow.ScopeID)
		if err != nil {
			return nil, err
		}
		result.PreviousCurrentCount, _ = changed.RowsAffected()
		result.LifecycleAction = "consent_revoked"
	} else if item.ObservationState == LorebookObservationObserved && item.CompleteSnapshot {
		changed, err := tx.ExecContext(ctx, `
			UPDATE lorebook_reference_entries
			SET is_current = FALSE, lifecycle_state = ?, last_seen_at = ?
			WHERE scope_id = ? AND is_current = TRUE
		`, LorebookLifecycleStale, observedAt, scopeRow.ScopeID)
		if err != nil {
			return nil, err
		}
		result.PreviousCurrentCount, _ = changed.RowsAffected()
		result.LifecycleAction = "current_projection_replaced"
		entryLifecycle = LorebookLifecycleCurrent
		entryCurrent = true
	}

	for ordinal, observed := range item.Entries {
		observed.EntryOrdinal = ordinal
		observed.NormalizedSearch = normalizeLorebookSearchText(observed)
		observed.ContentHash = lorebookDiagnosticContentHash(observed.Content)
		if strings.TrimSpace(observed.SourceKind) == "" {
			observed.SourceKind = "current_host_aggregate"
		}
		extensionsJSON := strings.TrimSpace(observed.ExtensionsJSON)
		if extensionsJSON == "" {
			extensionsJSON = "{}"
		}
		if !json.Valid([]byte(extensionsJSON)) {
			return nil, ErrInvalidLorebookReference
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO lorebook_reference_entries
				(scope_id, snapshot_id, host_entry_id, entry_ordinal, source_kind, source_identity,
				 entry_key, second_key, entry_comment, content, normalized_search_text, entry_mode,
				 always_active, selective, use_regex, insert_order, activation_percent, book_version,
				 folder, extensions_json, content_hash, lifecycle_state, is_current, first_seen_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, scopeRow.ScopeID, item.SnapshotID, referenceNullable(observed.HostEntryID), ordinal,
			observed.SourceKind, referenceNullable(observed.SourceIdentity), observed.Key, observed.SecondKey,
			observed.Comment, observed.Content, observed.NormalizedSearch, referenceNullable(observed.Mode),
			nullableBoolPointer(observed.AlwaysActive), nullableBoolPointer(observed.Selective), nullableBoolPointer(observed.UseRegex),
			nullableIntPointer(observed.InsertOrder), nullableFloatPointer(observed.ActivationPct), nullableInt64Pointer(observed.BookVersion),
			referenceNullable(observed.Folder), extensionsJSON, observed.ContentHash, entryLifecycle, entryCurrent, observedAt, observedAt); err != nil {
			return nil, err
		}
	}
	if entryCurrent {
		result.CurrentEntryCount = len(item.Entries)
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM lorebook_reference_entries WHERE scope_id = ? AND is_current = TRUE`, scopeRow.ScopeID).Scan(&result.CurrentEntryCount); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *mariadbStore) getLorebookReferenceCurrent(ctx context.Context, scope LorebookReferenceScope, limit, offset int) (*LorebookReferenceCurrent, int, error) {
	if strings.TrimSpace(scope.ChatSessionID) == "" {
		return nil, 0, ErrInvalidLorebookReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, 0, err
	}
	scopeRow, err := findLorebookReferenceScope(ctx, m.db, scope, false)
	if err != nil {
		return nil, 0, err
	}
	result := &LorebookReferenceCurrent{ScopeID: scopeRow.ScopeID, Scope: scope, Entries: []LorebookReferenceEntryObservation{}}
	var snapshot LorebookReferenceSnapshot
	var provenance string
	err = m.db.QueryRowContext(ctx, `
		SELECT snapshot_id, contract_version, consent_state, observation_state, complete_snapshot,
		       provenance_json, observed_at
		FROM lorebook_reference_snapshots
		WHERE scope_id = ? ORDER BY observed_at DESC, created_at DESC, snapshot_id DESC LIMIT 1
	`, scopeRow.ScopeID).Scan(&snapshot.SnapshotID, &snapshot.ContractVersion, &snapshot.ConsentState,
		&snapshot.ObservationState, &snapshot.CompleteSnapshot, &provenance, &snapshot.ObservedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, 0, err
	}
	if err == nil {
		snapshot.Scope = scope
		snapshot.ProvenanceJSON = provenance
		result.LatestSnapshot = &snapshot
	}
	total := 0
	if limit > 0 {
		if err := m.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM lorebook_reference_entries
			WHERE scope_id = ? AND is_current = TRUE AND lifecycle_state = ?
		`, scopeRow.ScopeID, LorebookLifecycleCurrent).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	entryQuery := `
		SELECT host_entry_id, entry_ordinal, source_kind, source_identity, entry_key, second_key,
		       entry_comment, content, normalized_search_text, entry_mode, always_active, selective,
		       use_regex, insert_order, activation_percent, book_version, folder, extensions_json, content_hash
		FROM lorebook_reference_entries
		WHERE scope_id = ? AND is_current = TRUE AND lifecycle_state = ?
		ORDER BY entry_ordinal ASC, entry_record_id ASC`
	entryArgs := []any{scopeRow.ScopeID, LorebookLifecycleCurrent}
	if limit > 0 {
		entryQuery += " LIMIT ? OFFSET ?"
		entryArgs = append(entryArgs, limit, offset)
	}
	rows, err := m.db.QueryContext(ctx, entryQuery, entryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry LorebookReferenceEntryObservation
		var hostID, sourceIdentity, mode, folder sql.NullString
		var alwaysActive, selective, useRegex sql.NullBool
		var insertOrder sql.NullInt64
		var activationPct sql.NullFloat64
		var bookVersion sql.NullInt64
		if err := rows.Scan(&hostID, &entry.EntryOrdinal, &entry.SourceKind, &sourceIdentity, &entry.Key, &entry.SecondKey,
			&entry.Comment, &entry.Content, &entry.NormalizedSearch, &mode, &alwaysActive, &selective,
			&useRegex, &insertOrder, &activationPct, &bookVersion, &folder, &entry.ExtensionsJSON, &entry.ContentHash); err != nil {
			return nil, 0, err
		}
		entry.HostEntryID = nullStringValue(hostID)
		entry.SourceIdentity = nullStringValue(sourceIdentity)
		entry.Mode = nullStringValue(mode)
		entry.Folder = nullStringValue(folder)
		if alwaysActive.Valid {
			value := alwaysActive.Bool
			entry.AlwaysActive = &value
		}
		if selective.Valid {
			value := selective.Bool
			entry.Selective = &value
		}
		if useRegex.Valid {
			value := useRegex.Bool
			entry.UseRegex = &value
		}
		if insertOrder.Valid {
			value := int(insertOrder.Int64)
			entry.InsertOrder = &value
		}
		if activationPct.Valid {
			value := activationPct.Float64
			entry.ActivationPct = &value
		}
		if bookVersion.Valid {
			value := bookVersion.Int64
			entry.BookVersion = &value
		}
		result.Entries = append(result.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		total = len(result.Entries)
	}
	return result, total, nil
}

func (m *mariadbStore) GetLorebookReferenceCurrent(ctx context.Context, scope LorebookReferenceScope) (*LorebookReferenceCurrent, error) {
	result, _, err := m.getLorebookReferenceCurrent(ctx, scope, 0, 0)
	return result, err
}

func (m *mariadbStore) GetLorebookReferenceCurrentPage(ctx context.Context, scope LorebookReferenceScope, limit, offset int) (*LorebookReferenceCurrentPage, error) {
	if limit <= 0 || limit > 100 || offset < 0 {
		return nil, ErrInvalidLorebookReference
	}
	current, total, err := m.getLorebookReferenceCurrent(ctx, scope, limit, offset)
	if err != nil {
		return nil, err
	}
	return &LorebookReferenceCurrentPage{
		ScopeID: current.ScopeID, Scope: current.Scope, LatestSnapshot: current.LatestSnapshot,
		Entries: current.Entries, Total: total, Limit: limit, Offset: offset,
	}, nil
}

func (m *mariadbStore) GetLorebookReferenceLatestSessionPage(ctx context.Context, chatSessionID string, limit, offset int) (*LorebookReferenceCurrentPage, error) {
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" || limit <= 0 || limit > 100 || offset < 0 {
		return nil, ErrInvalidLorebookReference
	}
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	var scopeIdentityJSON string
	err := m.db.QueryRowContext(ctx, `
		SELECT scope.scope_identity_json
		FROM lorebook_reference_scopes AS scope
		JOIN lorebook_reference_snapshots AS snapshot ON snapshot.scope_id = scope.scope_id
		WHERE scope.chat_session_id = ?
		ORDER BY snapshot.observed_at DESC, snapshot.created_at DESC, snapshot.snapshot_id DESC, scope.scope_id DESC
		LIMIT 1
	`, chatSessionID).Scan(&scopeIdentityJSON)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var scope LorebookReferenceScope
	if err := json.Unmarshal([]byte(scopeIdentityJSON), &scope); err != nil {
		return nil, err
	}
	if strings.TrimSpace(scope.ChatSessionID) != chatSessionID {
		return nil, ErrInvalidLorebookReference
	}
	return m.GetLorebookReferenceCurrentPage(ctx, scope, limit, offset)
}
