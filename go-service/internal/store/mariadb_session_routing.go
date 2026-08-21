package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SessionRouteBindingContractVersion  = "session-route-binding.v1"
	worldlineTopologyCompletedTurnLimit = 4096

	SessionRouteBindingModeResolveOrCreate = "resolve_or_create"
	SessionRouteBindingModeResolveExisting = "resolve_existing"
	SessionRouteBindingModeManualAttach    = "manual_attach"
	SessionRouteBindingModeMigrationCommit = "migration_commit"
	SessionRouteBindingModeLegacyPromotion = "legacy_promotion"
)

// SessionRouteBinding is the durable identity join between the official RisuAI
// stable character/chat observations and one canonical Archive Center SID.
type SessionRouteBinding struct {
	ContractVersion         string
	StableCharacterID       string
	HostChatID              string
	CanonicalSessionID      string
	BindingState            string
	BindingReason           string
	RedirectedFromSessionID string
	RedirectMigrationID     int64
	Revision                uint64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type SessionRouteBindingRequest struct {
	StableCharacterID  string
	HostChatID         string
	RequestedSessionID string
	Mode               string
}

type SessionRouteBindingResult struct {
	Binding              SessionRouteBinding
	Created              bool
	Updated              bool
	ReadbackVerified     bool
	LockedSourceRedirect bool
}

type SessionRouteBindingStore interface {
	BindSessionRoute(ctx context.Context, req SessionRouteBindingRequest) (*SessionRouteBindingResult, error)
}

var _ SessionRouteBindingStore = (*mariadbStore)(nil)
var _ WorldlineTopologySnapshotStore = (*mariadbStore)(nil)

func (m *mariadbStore) GetWorldlineTopologySnapshot(ctx context.Context, anchorSessionID string, limit int) (WorldlineTopologySnapshot, error) {
	if err := m.ensureDB(); err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	anchorSessionID = strings.TrimSpace(anchorSessionID)
	if anchorSessionID == "" {
		return WorldlineTopologySnapshot{}, errors.New("anchor_session_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	characterRows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT stable_character_id
		FROM session_route_bindings
		WHERE canonical_session_id = ? AND binding_state = 'active'
		ORDER BY stable_character_id ASC
		LIMIT 2
	`, anchorSessionID)
	if err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	stableCharacterIDs := make([]string, 0, 2)
	for characterRows.Next() {
		var stableCharacterID string
		if err := characterRows.Scan(&stableCharacterID); err != nil {
			characterRows.Close()
			return WorldlineTopologySnapshot{}, err
		}
		stableCharacterID = strings.TrimSpace(stableCharacterID)
		if stableCharacterID != "" {
			stableCharacterIDs = append(stableCharacterIDs, stableCharacterID)
		}
	}
	if err := characterRows.Err(); err != nil {
		characterRows.Close()
		return WorldlineTopologySnapshot{}, err
	}
	characterRows.Close()
	if len(stableCharacterIDs) == 0 {
		return WorldlineTopologySnapshot{}, ErrNotFound
	}
	if len(stableCharacterIDs) > 1 {
		return WorldlineTopologySnapshot{}, errors.New("anchor session is bound to multiple stable characters")
	}

	snapshot := WorldlineTopologySnapshot{
		StableCharacterID: stableCharacterIDs[0],
		AnchorSessionID:   anchorSessionID,
		SessionIDs:        []string{},
		LineageRecords:    []ForkLineageRecord{},
		CompletedTurns:    []WorldlineCompletedTurn{},
	}
	familyRows, err := tx.QueryContext(ctx, `
		SELECT canonical_session_id
		FROM session_route_bindings
		WHERE stable_character_id = ? AND binding_state = 'active'
		GROUP BY canonical_session_id
		ORDER BY CASE WHEN canonical_session_id = ? THEN 0 ELSE 1 END,
		         canonical_session_id ASC
		LIMIT ?
	`, snapshot.StableCharacterID, anchorSessionID, limit+1)
	if err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	for familyRows.Next() {
		var sessionID string
		if err := familyRows.Scan(&sessionID); err != nil {
			familyRows.Close()
			return WorldlineTopologySnapshot{}, err
		}
		if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
			snapshot.SessionIDs = append(snapshot.SessionIDs, sessionID)
		}
	}
	if err := familyRows.Err(); err != nil {
		familyRows.Close()
		return WorldlineTopologySnapshot{}, err
	}
	familyRows.Close()
	if len(snapshot.SessionIDs) > limit {
		snapshot.SessionIDs = snapshot.SessionIDs[:limit]
		snapshot.Truncated = true
	}
	if len(snapshot.SessionIDs) == 0 {
		return WorldlineTopologySnapshot{}, ErrNotFound
	}

	placeholders := make([]string, len(snapshot.SessionIDs))
	args := make([]any, len(snapshot.SessionIDs))
	for index, sessionID := range snapshot.SessionIDs {
		placeholders[index] = "?"
		args[index] = sessionID
	}
	lineageRows, err := tx.QueryContext(ctx, `
		SELECT id, contract_version, lineage_state, chat_session_id,
		       scope_id, parent_scope_id, copied_from_scope_id, copied_from_session_id,
		       fork_turn, fork_source_message_id, fork_source_role, idempotency_key,
		       imported_at, divergence_marker, provenance_source,
		       inheritance_mode, inherited_items_json, created_at, updated_at
		FROM session_fork_lineage
		WHERE chat_session_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY chat_session_id ASC, imported_at DESC, id DESC
	`, args...)
	if err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	for lineageRows.Next() {
		var item ForkLineageRecord
		var scopeID, parentScopeID, copiedFromScopeID, copiedFromSessionID sql.NullString
		var forkTurn sql.NullInt64
		var forkSourceMessageID, forkSourceRole, idempotencyKey sql.NullString
		var divergenceMarker, inheritedItemsJSON sql.NullString
		if err := lineageRows.Scan(
			&item.ID, &item.ContractVersion, &item.LineageState, &item.ChatSessionID,
			&scopeID, &parentScopeID, &copiedFromScopeID, &copiedFromSessionID,
			&forkTurn, &forkSourceMessageID, &forkSourceRole, &idempotencyKey,
			&item.ImportedAt, &divergenceMarker, &item.ProvenanceSource,
			&item.InheritanceMode, &inheritedItemsJSON, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return WorldlineTopologySnapshot{}, err
		}
		item.ScopeID = stringFromNull(scopeID)
		item.ParentScopeID = stringFromNull(parentScopeID)
		item.CopiedFromScopeID = stringFromNull(copiedFromScopeID)
		item.CopiedFromSessionID = stringFromNull(copiedFromSessionID)
		item.ForkTurn = int(forkTurn.Int64)
		item.ForkSourceMessageID = stringFromNull(forkSourceMessageID)
		item.ForkSourceRole = stringFromNull(forkSourceRole)
		item.IdempotencyKey = stringFromNull(idempotencyKey)
		item.DivergenceMarker = stringFromNull(divergenceMarker)
		item.InheritedItemsJSON = stringFromNull(inheritedItemsJSON)
		snapshot.LineageRecords = append(snapshot.LineageRecords, item)
	}
	if err := lineageRows.Err(); err != nil {
		lineageRows.Close()
		return WorldlineTopologySnapshot{}, err
	}
	lineageRows.Close()

	turnArgs := append([]any(nil), args...)
	turnArgs = append(turnArgs, worldlineTopologyCompletedTurnLimit+1)
	turnRows, err := tx.QueryContext(ctx, `
		SELECT chat_session_id, turn_index
		FROM chat_logs
		WHERE chat_session_id IN (`+strings.Join(placeholders, ",")+`)
		  AND turn_index > 0
		GROUP BY chat_session_id, turn_index
		HAVING MAX(CASE
		           WHEN LOWER(TRIM(role)) = 'user'
		            AND CHAR_LENGTH(TRIM(content)) > 0 THEN 1 ELSE 0
		       END) = 1
		   AND MAX(CASE
		           WHEN LOWER(TRIM(role)) = 'assistant'
		            AND CHAR_LENGTH(TRIM(content)) > 0 THEN 1 ELSE 0
		       END) = 1
		ORDER BY turn_index ASC, chat_session_id ASC
		LIMIT ?
	`, turnArgs...)
	if err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	for turnRows.Next() {
		var item WorldlineCompletedTurn
		if err := turnRows.Scan(&item.ChatSessionID, &item.TurnIndex); err != nil {
			turnRows.Close()
			return WorldlineTopologySnapshot{}, err
		}
		item.ChatSessionID = strings.TrimSpace(item.ChatSessionID)
		if item.ChatSessionID != "" && item.TurnIndex > 0 {
			snapshot.CompletedTurns = append(snapshot.CompletedTurns, item)
		}
	}
	if err := turnRows.Err(); err != nil {
		turnRows.Close()
		return WorldlineTopologySnapshot{}, err
	}
	turnRows.Close()
	if len(snapshot.CompletedTurns) > worldlineTopologyCompletedTurnLimit {
		snapshot.CompletedTurns = snapshot.CompletedTurns[:worldlineTopologyCompletedTurnLimit]
		snapshot.TurnsTruncated = true
	}
	if err := tx.Commit(); err != nil {
		return WorldlineTopologySnapshot{}, err
	}
	committed = true
	return snapshot, nil
}

func (m *mariadbStore) BindSessionRoute(ctx context.Context, req SessionRouteBindingRequest) (*SessionRouteBindingResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	stableCharacterID := strings.TrimSpace(req.StableCharacterID)
	hostChatID := strings.TrimSpace(req.HostChatID)
	requestedSessionID := strings.TrimSpace(req.RequestedSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = SessionRouteBindingModeResolveOrCreate
	}
	if stableCharacterID == "" || hostChatID == "" {
		return nil, errors.New("stable_character_id and host_chat_id are required")
	}
	if !sessionRouteBindingModeSupported(mode) {
		return nil, fmt.Errorf("unsupported session route binding mode %q", mode)
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	existing, err := selectSessionRouteBindingTx(ctx, tx, stableCharacterID, hostChatID, true)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if mode == SessionRouteBindingModeResolveExisting {
		if errors.Is(err, ErrNotFound) || existing == nil {
			return nil, ErrNotFound
		}
		if existing.ContractVersion != SessionRouteBindingContractVersion ||
			existing.StableCharacterID != stableCharacterID ||
			existing.HostChatID != hostChatID ||
			strings.TrimSpace(existing.CanonicalSessionID) == "" ||
			existing.BindingState != "active" {
			return nil, errors.New("session route binding readback mismatch")
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		committed = true
		return &SessionRouteBindingResult{
			Binding:          *existing,
			ReadbackVerified: true,
		}, nil
	}
	forceRequested := mode == SessionRouteBindingModeManualAttach || mode == SessionRouteBindingModeMigrationCommit
	canonicalSessionID := requestedSessionID
	if existing != nil && !forceRequested {
		canonicalSessionID = existing.CanonicalSessionID
	}
	if canonicalSessionID == "" {
		return nil, errors.New("requested_session_id is required when no durable route binding exists")
	}

	redirectedSessionID, redirectMigrationID, err := resolveLockedSessionRouteTx(ctx, tx, canonicalSessionID)
	if err != nil {
		return nil, err
	}
	lockedRedirect := redirectedSessionID != canonicalSessionID
	redirectedFrom := ""
	if lockedRedirect {
		redirectedFrom = canonicalSessionID
		canonicalSessionID = redirectedSessionID
	}
	bindingReason := mode
	if existing != nil && !forceRequested && !lockedRedirect {
		bindingReason = "existing_readback"
	}
	if lockedRedirect {
		bindingReason = "locked_source_redirect"
	}

	result := &SessionRouteBindingResult{LockedSourceRedirect: lockedRedirect}
	if existing == nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_route_bindings (
				contract_version, stable_character_id, host_chat_id,
				canonical_session_id, binding_state, binding_reason,
				redirected_from_session_id, redirect_migration_id, revision
			) VALUES (?, ?, ?, ?, 'active', ?, ?, ?, 1)
		`, SessionRouteBindingContractVersion, stableCharacterID, hostChatID,
			canonicalSessionID, bindingReason, nullableString(redirectedFrom), nullablePositiveInt64(redirectMigrationID))
		if err != nil {
			return nil, err
		}
		result.Created = true
	} else if forceRequested || lockedRedirect {
		_, err = tx.ExecContext(ctx, `
			UPDATE session_route_bindings
			SET contract_version = ?,
			    canonical_session_id = ?,
			    binding_state = 'active',
			    binding_reason = ?,
			    redirected_from_session_id = ?,
			    redirect_migration_id = ?,
			    revision = revision + 1
			WHERE stable_character_id = ? AND host_chat_id = ?
		`, SessionRouteBindingContractVersion, canonicalSessionID, bindingReason,
			nullableString(redirectedFrom), nullablePositiveInt64(redirectMigrationID),
			stableCharacterID, hostChatID)
		if err != nil {
			return nil, err
		}
		result.Updated = true
	}

	readback, err := selectSessionRouteBindingTx(ctx, tx, stableCharacterID, hostChatID, false)
	if err != nil {
		return nil, fmt.Errorf("session route binding readback: %w", err)
	}
	if readback.ContractVersion != SessionRouteBindingContractVersion ||
		readback.StableCharacterID != stableCharacterID ||
		readback.HostChatID != hostChatID ||
		readback.CanonicalSessionID != canonicalSessionID ||
		readback.BindingState != "active" {
		return nil, errors.New("session route binding readback mismatch")
	}
	result.Binding = *readback
	result.ReadbackVerified = true
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return result, nil
}

func sessionRouteBindingModeSupported(mode string) bool {
	switch strings.TrimSpace(mode) {
	case SessionRouteBindingModeResolveOrCreate,
		SessionRouteBindingModeResolveExisting,
		SessionRouteBindingModeManualAttach,
		SessionRouteBindingModeMigrationCommit,
		SessionRouteBindingModeLegacyPromotion:
		return true
	default:
		return false
	}
}

func selectSessionRouteBindingTx(ctx context.Context, tx *sql.Tx, stableCharacterID, hostChatID string, forUpdate bool) (*SessionRouteBinding, error) {
	query := `
		SELECT contract_version, stable_character_id, host_chat_id,
		       canonical_session_id, binding_state, binding_reason,
		       COALESCE(redirected_from_session_id, ''),
		       COALESCE(redirect_migration_id, 0), revision, created_at, updated_at
		FROM session_route_bindings
		WHERE stable_character_id = ? AND host_chat_id = ?
	`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var binding SessionRouteBinding
	err := tx.QueryRowContext(ctx, query, stableCharacterID, hostChatID).Scan(
		&binding.ContractVersion,
		&binding.StableCharacterID,
		&binding.HostChatID,
		&binding.CanonicalSessionID,
		&binding.BindingState,
		&binding.BindingReason,
		&binding.RedirectedFromSessionID,
		&binding.RedirectMigrationID,
		&binding.Revision,
		&binding.CreatedAt,
		&binding.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func resolveLockedSessionRouteTx(ctx context.Context, tx *sql.Tx, sessionID string) (string, int64, error) {
	current := strings.TrimSpace(sessionID)
	seen := map[string]bool{}
	var lastMigrationID int64
	for hop := 0; hop < 8; hop++ {
		if current == "" {
			return "", 0, errors.New("empty canonical session route")
		}
		if seen[current] {
			return "", 0, errors.New("session route lock redirect cycle")
		}
		seen[current] = true
		var target string
		var migrationID int64
		var lockStatus string
		err := tx.QueryRowContext(ctx, `
			SELECT target_session_id, migration_id, lock_status
			FROM session_migration_locks
			WHERE source_session_id = ? AND locked = TRUE AND unlocked_at IS NULL
			ORDER BY locked_at DESC, id DESC
			LIMIT 1
		`, current).Scan(&target, &migrationID, &lockStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return current, lastMigrationID, nil
		}
		if err != nil {
			return "", 0, err
		}
		if lockStatus == "lock_pending_verification" {
			return "", 0, sessionMigrationBlocker(
				"source_lock_verification_in_progress", "session_route", "",
			)
		}
		if lockStatus != "migrated_away" {
			return "", 0, sessionMigrationBlocker(
				"source_lock_state_not_routable", "session_route", "",
			)
		}
		target = strings.TrimSpace(target)
		if target == "" || target == current {
			return "", 0, errors.New("invalid session route lock redirect")
		}
		current = target
		lastMigrationID = migrationID
	}
	return "", 0, errors.New("session route lock redirect depth exceeded")
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
