package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
var _ EffectiveInputListStore = (*mariadbStore)(nil)
var _ SessionMigrationStore = (*mariadbStore)(nil)
var _ SessionRoutingBaselineStore = (*mariadbStore)(nil)
var _ SessionMigrationVectorStore = (*mariadbStore)(nil)
var _ SessionMigrationVectorParityStore = (*mariadbStore)(nil)
var _ SessionMigrationSourceLockStore = (*mariadbStore)(nil)
var _ SessionMigrationSourceLockFenceStore = (*mariadbStore)(nil)
var _ SessionMigrationRecoveryStore = (*mariadbStore)(nil)

func (m *mariadbStore) GetSessionMigrationResumeContext(
	ctx context.Context,
	req SessionMigrationCompleteRequest,
) (*SessionMigrationResumeContext, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	sourceID := strings.TrimSpace(req.SourceSessionID)
	targetID := strings.TrimSpace(req.TargetSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = SessionMigrationModeCopyThenLockSource
	}
	if sourceID == "" || targetID == "" {
		return nil, ErrNotFound
	}
	result := &SessionMigrationResumeContext{}
	err := m.db.QueryRowContext(ctx, `
		SELECT id, status, source_session_id, target_session_id, mode
		FROM session_migrations
		WHERE source_session_id = ?
		  AND target_session_id = ?
		  AND mode = ?
		  AND status NOT IN ('rolled_back', 'rollback_partial')
		ORDER BY id DESC
		LIMIT 1
	`, sourceID, targetID, mode).Scan(
		&result.MigrationID,
		&result.Status,
		&result.SourceSessionID,
		&result.TargetSessionID,
		&result.Mode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *mariadbStore) InspectSessionMigrationOccupancy(ctx context.Context, sessionID string) (SessionMigrationOccupancy, error) {
	if err := m.ensureDB(); err != nil {
		return SessionMigrationOccupancy{}, err
	}
	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return classifySessionMigrationOccupancy(map[string]int{}, false), nil
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return SessionMigrationOccupancy{}, err
	}
	defer tx.Rollback()
	counts := make(map[string]int)
	for _, entry := range SessionMigrationManifest() {
		if !entry.Direct {
			continue
		}
		if _, ok := SessionMigrationExecutionPlanFor(entry.Table); !ok {
			return SessionMigrationOccupancy{}, fmt.Errorf("session migration manifest plan missing for %s", entry.Table)
		}
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE `%s` = ?", entry.Table, entry.SessionColumn)
		if err := tx.QueryRowContext(ctx, query, sid).Scan(&count); err != nil {
			return SessionMigrationOccupancy{}, fmt.Errorf("session migration occupancy %s: %w", entry.Table, err)
		}
		counts[entry.Table] = count
	}
	starter := false
	if counts["chat_logs"] == 1 {
		var turnIndex int
		var role string
		err := tx.QueryRowContext(ctx, `
			SELECT turn_index, role
			FROM chat_logs
			WHERE chat_session_id = ?
			LIMIT 1
		`, sid).Scan(&turnIndex, &role)
		if err != nil {
			return SessionMigrationOccupancy{}, fmt.Errorf("session migration occupancy chat_logs starter: %w", err)
		}
		starter = turnIndex == 0 && strings.EqualFold(strings.TrimSpace(role), "assistant")
	}
	occupancy := classifySessionMigrationOccupancy(counts, starter)
	if err := tx.Commit(); err != nil {
		return SessionMigrationOccupancy{}, err
	}
	return occupancy, nil
}

func (m *mariadbStore) GetSessionRoutingBaseline(ctx context.Context, targetSessionID string) (*SessionRoutingBaseline, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	targetID := strings.TrimSpace(targetSessionID)
	if targetID == "" {
		return nil, ErrNotFound
	}
	row := &SessionRoutingBaseline{}
	err := m.db.QueryRowContext(ctx, `
		SELECT sm.id, sm.source_session_id, sm.target_session_id, sm.mode,
		       COALESCE(MAX(cl.turn_index), 0) AS imported_through_turn
		FROM session_migrations sm
		JOIN session_migration_row_map rm
		  ON rm.migration_id = sm.id
		 AND rm.table_name = 'chat_logs'
		 AND rm.target_row_id IS NOT NULL
		 AND rm.row_status <> 'rolled_back'
		JOIN chat_logs cl
		  ON cl.id = rm.target_row_id
		 AND cl.chat_session_id = sm.target_session_id
		WHERE sm.target_session_id = ?
		  AND sm.status NOT IN ('rolled_back', 'rollback_partial')
		GROUP BY sm.id, sm.source_session_id, sm.target_session_id, sm.mode
		HAVING imported_through_turn > 0
		ORDER BY sm.id DESC
		LIMIT 1
	`, targetID).Scan(
		&row.MigrationID,
		&row.SourceSessionID,
		&row.TargetSessionID,
		&row.Mode,
		&row.ImportedThroughTurn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (m *mariadbStore) CompleteSessionMigration(ctx context.Context, req SessionMigrationCompleteRequest) (*SessionMigrationCompleteResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	sourceID := strings.TrimSpace(req.SourceSessionID)
	targetID := strings.TrimSpace(req.TargetSessionID)
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = SessionMigrationModeCopyThenLockSource
	}
	if sourceID == "" || targetID == "" {
		return nil, errors.New("source_session_id and target_session_id are required")
	}
	if sourceID == targetID {
		return nil, errors.New("source and target sessions must differ")
	}
	if mode != SessionMigrationModeCopyThenLockSource && mode != SessionMigrationModeCopyKeepSource {
		return nil, errors.New("unsupported session migration mode")
	}
	if blockers := SessionMigrationManifestReleaseBlockers(); len(blockers) > 0 {
		return nil, fmt.Errorf("session migration manifest unsupported: %s", strings.Join(blockers, ","))
	}
	if err := validateSessionMigrationManifestSchema(ctx, m.db); err != nil {
		return nil, err
	}
	if resumed, err := resumeCompletedSessionMigration(ctx, m.db, sourceID, targetID, mode); err != nil {
		return nil, err
	} else if resumed != nil {
		return resumed, nil
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	execution, err := completeSessionMigrationManifestTx(ctx, tx, SessionMigrationCompleteRequest{
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Mode:            mode,
		OperatorNote:    strings.TrimSpace(req.OperatorNote),
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &SessionMigrationCompleteResult{
		MigrationID:           execution.MigrationID,
		Status:                "copied",
		SourceSessionID:       sourceID,
		TargetSessionID:       targetID,
		Mode:                  mode,
		Counts:                execution.Counts,
		RowMapCount:           execution.RowMapCount,
		ChromaReindexedCount:  0,
		SourceLocked:          false,
		ChromaReindexRequired: execution.VectorExpectedCount > 0,
		ReadyForLive:          false,
		TargetStarterReplaced: execution.TargetStarterReplaced,
	}, nil
}

type sessionMigrationManifestExecution struct {
	MigrationID           int64
	Counts                SessionMigrationArtifactCounts
	RowMapCount           int
	VectorExpectedCount   int
	TargetStarterReplaced bool
}

type sessionMigrationCell struct {
	Valid bool
	Text  string
}

type sessionMigrationRow struct {
	Values map[string]sessionMigrationCell
}

type sessionMigrationKeyMaps struct {
	forward map[string]map[string]string
	inverse map[string]map[string]string
}

type sessionMigrationDeferredFK struct {
	Table       string
	PrimaryKey  string
	TargetKey   string
	Column      string
	SourceValue string
	Reference   SessionMigrationForeignKeyPlan
}

func newSessionMigrationKeyMaps() *sessionMigrationKeyMaps {
	return &sessionMigrationKeyMaps{
		forward: map[string]map[string]string{},
		inverse: map[string]map[string]string{},
	}
}

func (m *sessionMigrationKeyMaps) put(table, column, source, target string) error {
	key := table + "." + column
	if m.forward[key] == nil {
		m.forward[key] = map[string]string{}
		m.inverse[key] = map[string]string{}
	}
	if existing, ok := m.forward[key][source]; ok && existing != target {
		return fmt.Errorf("session migration row-map mismatch for %s source %q", key, source)
	}
	if existing, ok := m.inverse[key][target]; ok && existing != source {
		return fmt.Errorf("session migration row-map target collision for %s target %q", key, target)
	}
	m.forward[key][source] = target
	m.inverse[key][target] = source
	return nil
}

func (m *sessionMigrationKeyMaps) target(table, column, source string) (string, bool) {
	value, ok := m.forward[table+"."+column][source]
	return value, ok
}

func (m *sessionMigrationKeyMaps) source(table, column, target string) (string, bool) {
	value, ok := m.inverse[table+"."+column][target]
	return value, ok
}

func validateSessionMigrationManifestSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, entry := range SessionMigrationManifest() {
		plan, ok := SessionMigrationExecutionPlanFor(entry.Table)
		if !ok {
			return fmt.Errorf("session migration manifest plan missing for %s", entry.Table)
		}
		if err := sessionMigrationValidateSchemaTx(ctx, tx, plan); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func resumeCompletedSessionMigration(ctx context.Context, db *sql.DB, sourceID, targetID, mode string) (*SessionMigrationCompleteResult, error) {
	var migrationID int64
	var status string
	var countsJSON sql.NullString
	var chromaCount int
	var lockedAt sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT sm.id, sm.status, sm.counts_json, sm.chroma_reindexed_count, sm.locked_at
		FROM session_migrations sm
		WHERE sm.source_session_id = ? AND sm.target_session_id = ? AND sm.mode = ?
		  AND sm.status IN ('copied', 'vector_reindexed', 'source_locked', 'cleanup_prepared', 'source_cleaned')
		ORDER BY sm.id DESC
		LIMIT 1
	`, sourceID, targetID, mode).Scan(&migrationID, &status, &countsJSON, &chromaCount, &lockedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: map[bool]sql.IsolationLevel{
			true:  sql.LevelReadCommitted,
			false: sql.LevelRepeatableRead,
		}[status == "copied"],
		ReadOnly: status == "copied",
	})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	rowMapCount, err := sessionMigrationVerifyResumeParityTx(ctx, tx, migrationID, status)
	if err != nil {
		return nil, err
	}
	counts := SessionMigrationArtifactCounts{}
	if countsJSON.Valid && strings.TrimSpace(countsJSON.String) != "" {
		if err := json.Unmarshal([]byte(countsJSON.String), &counts); err != nil {
			return nil, fmt.Errorf("resume session migration counts: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationCompleteResult{
		MigrationID:           migrationID,
		Status:                status,
		SourceSessionID:       sourceID,
		TargetSessionID:       targetID,
		Mode:                  mode,
		Counts:                counts,
		RowMapCount:           rowMapCount,
		ChromaReindexedCount:  chromaCount,
		SourceLocked:          lockedAt.Valid,
		ChromaReindexRequired: status == "copied",
		ReadyForLive: status == "source_locked" || status == "cleanup_prepared" || status == "source_cleaned" ||
			(mode == SessionMigrationModeCopyKeepSource && status == "vector_reindexed"),
	}, nil
}

func sessionMigrationVerifyResumeParityTx(ctx context.Context, tx *sql.Tx, migrationID int64, status string) (int, error) {
	var total, relationalVerified, expectedRowMaps int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(
				CASE WHEN parity_state LIKE 'verified_%'
				 AND COALESCE(source_row_count, -1) >= 0
				 AND source_content_hash IS NOT NULL
				 AND COALESCE(target_row_count, -1) >= 0
				 AND target_content_hash IS NOT NULL
				 AND COALESCE(row_map_expected_count, 0) = COALESCE(row_map_verified_count, 0)
				 AND COALESCE(fk_expected_count, 0) = COALESCE(fk_verified_count, 0)
				THEN 1 ELSE 0 END
			), 0),
			COALESCE(SUM(row_map_expected_count), 0)
		FROM session_migration_artifact_parity
		WHERE migration_id = ? AND manifest_version = ?
	`, migrationID, SessionMigrationManifestVersion).Scan(&total, &relationalVerified, &expectedRowMaps); err != nil {
		return 0, err
	}
	expectedEntries := len(SessionMigrationManifest())
	if total != expectedEntries || relationalVerified != expectedEntries {
		return 0, fmt.Errorf(
			"session migration resume blocked: current manifest relational proof %d/%d (rows %d/%d)",
			relationalVerified, expectedEntries, total, expectedEntries,
		)
	}
	var actualRowMaps int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM session_migration_artifact_row_map
		WHERE migration_id = ? AND row_status = 'copied'
	`, migrationID).Scan(&actualRowMaps); err != nil {
		return 0, err
	}
	if actualRowMaps != expectedRowMaps {
		return 0, fmt.Errorf("session migration resume blocked: primary row-map proof %d/%d", actualRowMaps, expectedRowMaps)
	}
	if err := sessionMigrationVerifyExpectedVectorLedgerTx(ctx, tx, migrationID); err != nil {
		return 0, fmt.Errorf("session migration resume blocked: %w", err)
	}
	currentRelationalHash, err := sessionMigrationRevalidateCurrentRelationalStateTx(
		ctx, tx, migrationID, "resume", status != "copied",
	)
	if err != nil {
		return 0, err
	}
	if status != "copied" {
		if err := sessionMigrationVerifyDurableParityTx(ctx, tx, migrationID); err != nil {
			return 0, fmt.Errorf("session migration resume blocked: %w", err)
		}
		if err := sessionMigrationConsumeCurrentStateProofTx(
			ctx, tx, migrationID, SessionMigrationProofOperationResume, currentRelationalHash,
		); err != nil {
			return 0, err
		}
	}
	return actualRowMaps, nil
}

func sessionMigrationVerifyExpectedVectorLedgerTx(ctx context.Context, tx *sql.Tx, migrationID int64) error {
	expectedByTable := map[string][]string{}
	rows, err := tx.QueryContext(ctx, `
		SELECT source_table, document_id
		FROM session_migration_vector_expected_ids
		WHERE migration_id = ?
		ORDER BY source_table, document_id
	`, migrationID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var table, documentID string
		if err := rows.Scan(&table, &documentID); err != nil {
			_ = rows.Close()
			return err
		}
		expectedByTable[table] = append(expectedByTable[table], documentID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	parityRows, err := tx.QueryContext(ctx, `
		SELECT table_name, COALESCE(vector_expected_count, 0),
		       COALESCE(vector_expected_id_hash, '')
		FROM session_migration_artifact_parity
		WHERE migration_id = ? AND manifest_version = ?
		ORDER BY table_name
	`, migrationID, SessionMigrationManifestVersion)
	if err != nil {
		return err
	}
	defer parityRows.Close()
	seen := 0
	for parityRows.Next() {
		var table, expectedHash string
		var expectedCount int
		if err := parityRows.Scan(&table, &expectedCount, &expectedHash); err != nil {
			return err
		}
		seen++
		ids := expectedByTable[table]
		actualHash := ""
		if len(ids) > 0 {
			actualHash = sessionMigrationHashIDs(ids)
		}
		if len(ids) != expectedCount || actualHash != expectedHash {
			return fmt.Errorf("vector expected-ID ledger mismatch for %s: ids=%d/%s parity=%d/%s",
				table, len(ids), actualHash, expectedCount, expectedHash)
		}
		delete(expectedByTable, table)
	}
	if err := parityRows.Err(); err != nil {
		return err
	}
	if seen != len(SessionMigrationManifest()) || len(expectedByTable) != 0 {
		return fmt.Errorf("vector expected-ID ledger manifest coverage mismatch: parity=%d/%d orphan_tables=%d",
			seen, len(SessionMigrationManifest()), len(expectedByTable))
	}
	return nil
}

func completeSessionMigrationManifestTx(ctx context.Context, tx *sql.Tx, req SessionMigrationCompleteRequest) (*sessionMigrationManifestExecution, error) {
	manifest := SessionMigrationManifest()
	sourceRows := make(map[string][]sessionMigrationRow, len(manifest))
	targetRowsBefore := make(map[string][]sessionMigrationRow, len(manifest))
	for _, entry := range manifest {
		if !entry.Implemented {
			return nil, fmt.Errorf("session migration manifest entry %s is not implemented", entry.Table)
		}
		plan, ok := SessionMigrationExecutionPlanFor(entry.Table)
		if !ok {
			return nil, fmt.Errorf("session migration manifest plan missing for %s", entry.Table)
		}
		if err := sessionMigrationValidateSchemaTx(ctx, tx, plan); err != nil {
			return nil, err
		}
		rows, err := sessionMigrationReadManifestRows(ctx, tx, entry, plan, req.SourceSessionID)
		if err != nil {
			return nil, fmt.Errorf("session migration source snapshot %s: %w", entry.Table, err)
		}
		sourceRows[entry.Table] = rows
		rows, err = sessionMigrationReadManifestRows(ctx, tx, entry, plan, req.TargetSessionID)
		if err != nil {
			return nil, fmt.Errorf("session migration target snapshot %s: %w", entry.Table, err)
		}
		targetRowsBefore[entry.Table] = rows
	}
	targetStarter, err := sessionMigrationValidateManifestSnapshots(manifest, sourceRows, targetRowsBefore)
	if err != nil {
		return nil, err
	}
	counts := sessionMigrationLegacyCountsFromManifest(sourceRows)
	counts.ReplaceableStarterOnly = targetStarter
	initialCountsJSON, _ := json.Marshal(counts)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO session_migrations (
			source_session_id, target_session_id, mode, status, operator_note,
			counts_json, chroma_reindexed_count, errors_json
		) VALUES (?, ?, ?, 'copying', ?, ?, 0, JSON_ARRAY())
	`, req.SourceSessionID, req.TargetSessionID, req.Mode, nullableString(strings.TrimSpace(req.OperatorNote)), string(initialCountsJSON))
	if err != nil {
		return nil, err
	}
	migrationID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	requestHash := sessionMigrationStringHash(SessionMigrationManifestVersion, req.SourceSessionID, req.TargetSessionID, req.Mode)
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "manifest_validate", "completed", requestHash, `{"validated":true}`, ""); err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "relational_copy", "running", requestHash, "", ""); err != nil {
		return nil, err
	}
	if targetStarter {
		deleted, err := tx.ExecContext(ctx, `
			DELETE FROM chat_logs
			WHERE chat_session_id = ? AND turn_index = 0 AND LOWER(TRIM(role)) = 'assistant'
		`, req.TargetSessionID)
		if err != nil {
			return nil, fmt.Errorf("replace target starter turn: %w", err)
		}
		affected, err := deleted.RowsAffected()
		if err != nil || affected != 1 {
			return nil, fmt.Errorf("replace target starter turn: expected 1 row, got %d", affected)
		}
	}

	keyMaps := newSessionMigrationKeyMaps()
	if err := sessionMigrationPrecomputeDeterministicKeys(manifest, sourceRows, req.TargetSessionID, keyMaps); err != nil {
		return nil, err
	}
	deferred := []sessionMigrationDeferredFK{}
	vectorExpected := map[string]SessionMigrationVectorDocument{}
	memoryProjectionOps, err := sessionMigrationMemoryProjectionOperations(
		sourceRows["memory_vector_outbox"], sourceRows["memory_source_revisions"], req.SourceSessionID,
	)
	if err != nil {
		return nil, err
	}
	activeSourceRevisions := sessionMigrationActiveSourceRevisions(sourceRows["memory_source_revisions"])
	rowMapCount := 0
	for _, entry := range manifest {
		if entry.Policy != SessionMigrationPolicyCopy {
			continue
		}
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		for _, sourceRow := range sourceRows[entry.Table] {
			targetRow, _, rowDeferred, err := sessionMigrationInsertManifestRow(
				ctx, tx, migrationID, entry, plan, sourceRow, req.TargetSessionID, keyMaps,
			)
			if err != nil {
				return nil, fmt.Errorf("session migration copy %s: %w", entry.Table, err)
			}
			deferred = append(deferred, rowDeferred...)
			rowMapCount++
			expected, ok := sessionMigrationExpectedVectorDocument(migrationID, entry.Table, plan, sourceRow, targetRow, req.SourceSessionID, req.TargetSessionID)
			if ok && entry.Table == "precise_memory_units" {
				ok = sessionMigrationPreciseVectorSourceActive(sourceRow, activeSourceRevisions)
			}
			if ok && entry.Table == "memories" {
				sourceMemoryID := strings.TrimSpace(sourceRow.Values[plan.PrimaryKey[0]].Text)
				sourceDocumentID := "memory:" + req.SourceSessionID + ":" + sourceMemoryID
				projection, found := memoryProjectionOps[sourceDocumentID]
				if !found {
					return nil, fmt.Errorf("session migration memory public projection authority is missing for %s; run canonical force reindex before retrying migration", sourceDocumentID)
				}
				if projection.SourceTurn != sessionMigrationCellInt(sourceRow.Values["turn_index"]) {
					return nil, fmt.Errorf("session migration memory public projection turn mismatch for %s; run canonical force reindex before retrying migration", sourceDocumentID)
				}
				if projection.Operation == "delete" {
					ok = false
				} else {
					expected.DocumentText = projection.DocumentText
				}
			}
			if ok {
				if previous, duplicate := vectorExpected[expected.ID]; duplicate && previous.SourceTable != expected.SourceTable {
					return nil, fmt.Errorf("session migration vector expected ID collision %q", expected.ID)
				}
				vectorExpected[expected.ID] = expected
			}
		}
	}
	for _, item := range deferred {
		targetValue, ok := keyMaps.target(item.Reference.ReferenceTable, item.Reference.ReferenceColumn, item.SourceValue)
		if !ok {
			return nil, fmt.Errorf("session migration deferred FK %s.%s has no row map for %s.%s=%q",
				item.Table, item.Column, item.Reference.ReferenceTable, item.Reference.ReferenceColumn, item.SourceValue)
		}
		query := "UPDATE " + sessionMigrationQuoteIdentifier(item.Table) +
			" SET " + sessionMigrationQuoteIdentifier(item.Column) + " = ? WHERE " +
			sessionMigrationQuoteIdentifier(item.PrimaryKey) + " = ?"
		updateResult, err := tx.ExecContext(ctx, query, targetValue, item.TargetKey)
		if err != nil {
			return nil, err
		}
		affected, _ := updateResult.RowsAffected()
		if affected != 1 {
			return nil, fmt.Errorf("session migration deferred FK %s.%s updated %d rows", item.Table, item.Column, affected)
		}
	}
	if err := sessionMigrationVerifyAndPersistRelationalParity(
		ctx, tx, migrationID, manifest, sourceRows, req.SourceSessionID, req.TargetSessionID, keyMaps,
	); err != nil {
		return nil, err
	}
	expectedIDs := make([]string, 0, len(vectorExpected))
	for id := range vectorExpected {
		expectedIDs = append(expectedIDs, id)
	}
	sort.Strings(expectedIDs)
	for _, id := range expectedIDs {
		doc := vectorExpected[id]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO session_migration_vector_expected_ids (
				migration_id, document_id, source_table, source_row_id, observed
			) VALUES (?, ?, ?, ?, FALSE)
		`, migrationID, doc.ID, doc.SourceTable, doc.SourceRowID); err != nil {
			return nil, err
		}
	}
	if err := sessionMigrationPersistVectorExpectedParityTx(ctx, tx, migrationID, vectorExpected); err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "relational_copy", "completed", requestHash,
		fmt.Sprintf(`{"row_map_count":%d}`, rowMapCount), ""); err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "vector_expected_ids", "completed", requestHash,
		fmt.Sprintf(`{"expected_count":%d}`, len(expectedIDs)), ""); err != nil {
		return nil, err
	}
	countsJSON, _ := json.Marshal(counts)
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'copied',
		    counts_json = ?,
		    errors_json = JSON_ARRAY('chroma_reindex_pending'),
		    completed_at = CURRENT_TIMESTAMP(3),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, string(countsJSON), migrationID); err != nil {
		return nil, err
	}
	return &sessionMigrationManifestExecution{
		MigrationID:           migrationID,
		Counts:                counts,
		RowMapCount:           rowMapCount,
		VectorExpectedCount:   len(expectedIDs),
		TargetStarterReplaced: targetStarter,
	}, nil
}

type sessionMigrationMemoryProjectionOperation struct {
	OutboxID     int64
	Operation    string
	DocumentText string
	SourceTurn   int
}

func sessionMigrationActiveSourceRevisions(rows []sessionMigrationRow) map[string]struct{} {
	active := map[string]struct{}{}
	for _, row := range rows {
		if !strings.EqualFold(strings.TrimSpace(row.Values["lifecycle_state"].Text), "active") {
			continue
		}
		revision := strings.TrimSpace(row.Values["source_revision"].Text)
		if revision != "" {
			active[revision] = struct{}{}
		}
	}
	return active
}

func sessionMigrationPreciseVectorSourceActive(row sessionMigrationRow, activeRevisions map[string]struct{}) bool {
	_, ok := activeRevisions[strings.TrimSpace(row.Values["source_revision"].Text)]
	return ok
}

// sessionMigrationMemoryProjectionOperations consumes the durable output of
// the existing public-memory projection contract. It does not infer public
// visibility from canonical summaries: the latest causal outbox operation is
// either an exact public upsert or an explicit delete.
func sessionMigrationMemoryProjectionOperations(rows, sourceRevisions []sessionMigrationRow, sourceSessionID string) (map[string]sessionMigrationMemoryProjectionOperation, error) {
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	activeRevisions := map[string]int{}
	for _, row := range sourceRevisions {
		if strings.ToLower(strings.TrimSpace(row.Values["lifecycle_state"].Text)) != "active" ||
			strings.ToLower(strings.TrimSpace(row.Values["derived_admission_state"].Text)) != "committed" ||
			strings.TrimSpace(row.Values["derived_index_version"].Text) != MemoryPublicProjectionIndex {
			continue
		}
		revision := strings.TrimSpace(row.Values["source_revision"].Text)
		if revision != "" {
			activeRevisions[revision] = sessionMigrationCellInt(row.Values["turn_index"])
		}
	}
	latest := map[string]sessionMigrationRow{}
	latestID := map[string]int64{}
	prefix := "memory:" + sourceSessionID + ":"
	for _, row := range rows {
		documentID := strings.TrimSpace(row.Values["document_id"].Text)
		if !strings.HasPrefix(documentID, prefix) {
			continue
		}
		if _, ok := activeRevisions[strings.TrimSpace(row.Values["source_revision"].Text)]; !ok {
			continue
		}
		outboxID, err := strconv.ParseInt(strings.TrimSpace(row.Values["id"].Text), 10, 64)
		if err != nil || outboxID <= 0 {
			return nil, fmt.Errorf("session migration memory public projection has invalid outbox id for %s", documentID)
		}
		if current, ok := latestID[documentID]; ok && current >= outboxID {
			continue
		}
		latestID[documentID] = outboxID
		latest[documentID] = row
	}

	out := make(map[string]sessionMigrationMemoryProjectionOperation, len(latest))
	for documentID, row := range latest {
		if strings.TrimSpace(row.Values["contract_version"].Text) != MemoryVectorOutboxContract {
			return nil, fmt.Errorf("session migration memory public projection contract mismatch for %s", documentID)
		}
		if strings.EqualFold(strings.TrimSpace(row.Values["status"].Text), "stale_rejected") {
			return nil, fmt.Errorf("session migration memory public projection is stale for %s; run canonical force reindex before retrying migration", documentID)
		}
		operation := strings.ToLower(strings.TrimSpace(row.Values["operation"].Text))
		projection := sessionMigrationMemoryProjectionOperation{
			OutboxID:   latestID[documentID],
			Operation:  operation,
			SourceTurn: activeRevisions[strings.TrimSpace(row.Values["source_revision"].Text)],
		}
		switch operation {
		case "delete":
			out[documentID] = projection
			continue
		case "upsert":
		default:
			return nil, fmt.Errorf("session migration memory public projection operation %q is unsupported for %s", operation, documentID)
		}

		var document struct {
			ID            string         `json:"ID"`
			Tier          string         `json:"Tier"`
			ChatSessionID string         `json:"ChatSessionID"`
			SourceTable   string         `json:"SourceTable"`
			SourceRowID   string         `json:"SourceRowID"`
			SchemaVersion string         `json:"SchemaVersion"`
			DocumentText  string         `json:"DocumentText"`
			Metadata      map[string]any `json:"Metadata"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(row.Values["document_json"].Text)), &document); err != nil {
			return nil, fmt.Errorf("session migration memory public projection document is invalid for %s: %w", documentID, err)
		}
		sourceRowID := strings.TrimPrefix(documentID, prefix)
		indexIdentity, _ := document.Metadata["index_identity"].(string)
		if strings.TrimSpace(document.ID) != documentID ||
			strings.TrimSpace(document.Tier) != "memory" ||
			strings.TrimSpace(document.ChatSessionID) != sourceSessionID ||
			strings.TrimSpace(document.SourceTable) != "memories" ||
			strings.TrimSpace(document.SourceRowID) != sourceRowID ||
			strings.TrimSpace(document.SchemaVersion) != "memory.v2" ||
			strings.TrimSpace(indexIdentity) != MemoryPublicProjectionIndex ||
			strings.TrimSpace(document.DocumentText) == "" {
			return nil, fmt.Errorf("session migration memory public projection document contract mismatch for %s", documentID)
		}
		projection.DocumentText = strings.TrimSpace(document.DocumentText)
		out[documentID] = projection
	}
	return out, nil
}

func sessionMigrationValidateSchemaTx(ctx context.Context, tx *sql.Tx, plan SessionMigrationExecutionPlan) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT COLUMN_NAME, EXTRA, GENERATION_EXPRESSION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, plan.Table)
	if err != nil {
		return fmt.Errorf("session migration schema validation %s: %w", plan.Table, err)
	}
	defer rows.Close()
	actual := []string{}
	generated := []string{}
	for rows.Next() {
		var column, extra string
		var generationExpression sql.NullString
		if err := rows.Scan(&column, &extra, &generationExpression); err != nil {
			return err
		}
		actual = append(actual, column)
		if strings.Contains(strings.ToUpper(extra), "GENERATED") ||
			(generationExpression.Valid && strings.TrimSpace(generationExpression.String) != "") {
			generated = append(generated, column)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if missing, extra, duplicates := sessionMigrationColumnSetDiff(actual, plan.Columns); len(missing) > 0 || len(extra) > 0 || len(duplicates) > 0 {
		return fmt.Errorf("session migration schema mismatch for %s: actual=%q manifest=%q missing=%q extra=%q duplicates=%q",
			plan.Table, strings.Join(actual, ","), strings.Join(plan.Columns, ","),
			strings.Join(missing, ","), strings.Join(extra, ","), strings.Join(duplicates, ","))
	}
	if missing, extra, duplicates := sessionMigrationColumnSetDiff(generated, plan.DatabaseGenerated); len(missing) > 0 || len(extra) > 0 || len(duplicates) > 0 {
		return fmt.Errorf(
			"session migration generated-column mismatch for %s: actual=%q manifest=%q missing=%q extra=%q duplicates=%q",
			plan.Table, strings.Join(generated, ","), strings.Join(plan.DatabaseGenerated, ","),
			strings.Join(missing, ","), strings.Join(extra, ","), strings.Join(duplicates, ","),
		)
	}
	primaryRows, err := tx.QueryContext(ctx, `
		SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION
	`, plan.Table)
	if err != nil {
		return fmt.Errorf("session migration primary-key validation %s: %w", plan.Table, err)
	}
	defer primaryRows.Close()
	primary := []string{}
	for primaryRows.Next() {
		var column string
		if err := primaryRows.Scan(&column); err != nil {
			return err
		}
		primary = append(primary, column)
	}
	if err := primaryRows.Err(); err != nil {
		return err
	}
	if strings.Join(primary, ",") != strings.Join(plan.PrimaryKey, ",") {
		return fmt.Errorf(
			"session migration primary-key mismatch for %s: actual=%q manifest=%q",
			plan.Table, strings.Join(primary, ","), strings.Join(plan.PrimaryKey, ","),
		)
	}
	if entry, ok := sessionMigrationManifestEntryByTable(plan.Table); ok && entry.Direct {
		var indexedRanges int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = ?
			  AND COLUMN_NAME = ?
			  AND SEQ_IN_INDEX = 1
		`, plan.Table, entry.SessionColumn).Scan(&indexedRanges); err != nil {
			return fmt.Errorf("session migration range-index validation %s: %w", plan.Table, err)
		}
		if indexedRanges == 0 {
			return fmt.Errorf(
				"session migration range-lock index missing for %s.%s",
				plan.Table, entry.SessionColumn,
			)
		}
	}
	return nil
}

func sessionMigrationColumnSetDiff(actual, expected []string) (missing, extra, duplicates []string) {
	actualCounts := make(map[string]int, len(actual))
	expectedCounts := make(map[string]int, len(expected))
	for _, column := range actual {
		column = strings.TrimSpace(column)
		actualCounts[column]++
		if actualCounts[column] == 2 {
			duplicates = append(duplicates, column)
		}
	}
	for _, column := range expected {
		column = strings.TrimSpace(column)
		expectedCounts[column]++
		if expectedCounts[column] == 2 {
			duplicates = append(duplicates, column)
		}
	}
	for _, column := range expected {
		column = strings.TrimSpace(column)
		if actualCounts[column] == 0 {
			missing = append(missing, column)
		}
	}
	for _, column := range actual {
		column = strings.TrimSpace(column)
		if expectedCounts[column] == 0 {
			extra = append(extra, column)
		}
	}
	return missing, extra, duplicates
}

func sessionMigrationReadManifestRows(
	ctx context.Context,
	tx *sql.Tx,
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	sessionID string,
) ([]sessionMigrationRow, error) {
	return sessionMigrationReadManifestRowsMode(ctx, tx, entry, plan, sessionID, false)
}

func sessionMigrationReadManifestRowsMode(
	ctx context.Context,
	tx *sql.Tx,
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	sessionID string,
	forUpdate bool,
) ([]sessionMigrationRow, error) {
	selectColumns := make([]string, len(plan.Columns))
	for index, column := range plan.Columns {
		selectColumns[index] = "t0." + sessionMigrationQuoteIdentifier(column)
	}
	query := "SELECT " + strings.Join(selectColumns, ",") + " FROM " +
		sessionMigrationQuoteIdentifier(entry.Table) + " t0"
	args := []any{sessionID}
	if entry.Direct {
		query += " WHERE t0." + sessionMigrationQuoteIdentifier(entry.SessionColumn) + " = ?"
	} else {
		currentEntry := entry
		currentPlan := plan
		aliasIndex := 0
		for !currentEntry.Direct {
			parentEntry, ok := sessionMigrationManifestEntryByTable(currentEntry.ParentTable)
			if !ok {
				return nil, fmt.Errorf("parent manifest entry %q not found", currentEntry.ParentTable)
			}
			parentPlan, ok := SessionMigrationExecutionPlanFor(parentEntry.Table)
			if !ok || len(parentPlan.PrimaryKey) != 1 {
				return nil, fmt.Errorf("parent plan %q has unsupported key", parentEntry.Table)
			}
			parentAlias := fmt.Sprintf("t%d", aliasIndex+1)
			query += " JOIN " + sessionMigrationQuoteIdentifier(parentEntry.Table) + " " + parentAlias +
				" ON t" + strconv.Itoa(aliasIndex) + "." + sessionMigrationQuoteIdentifier(currentPlan.ParentColumn) +
				" = " + parentAlias + "." + sessionMigrationQuoteIdentifier(parentPlan.PrimaryKey[0])
			aliasIndex++
			currentEntry = parentEntry
			currentPlan = parentPlan
		}
		query += " WHERE t" + strconv.Itoa(aliasIndex) + "." + sessionMigrationQuoteIdentifier(currentEntry.SessionColumn) + " = ?"
	}
	orderColumns := make([]string, len(plan.PrimaryKey))
	for index, column := range plan.PrimaryKey {
		orderColumns[index] = "t0." + sessionMigrationQuoteIdentifier(column)
	}
	query += " ORDER BY " + strings.Join(orderColumns, ",")
	if forUpdate {
		query += " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []sessionMigrationRow{}
	for rows.Next() {
		scans := make([]any, len(plan.Columns))
		dest := make([]any, len(scans))
		for index := range scans {
			dest[index] = &scans[index]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		item := sessionMigrationRow{Values: make(map[string]sessionMigrationCell, len(plan.Columns))}
		for index, column := range plan.Columns {
			cell, err := sessionMigrationNormalizeDatabaseValue(scans[index])
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", entry.Table, column, err)
			}
			item.Values[column] = cell
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func sessionMigrationNormalizeDatabaseValue(value any) (sessionMigrationCell, error) {
	switch typed := value.(type) {
	case nil:
		return sessionMigrationCell{}, nil
	case string:
		return sessionMigrationCell{Valid: true, Text: typed}, nil
	case []byte:
		return sessionMigrationCell{Valid: true, Text: string(typed)}, nil
	case time.Time:
		return sessionMigrationCell{Valid: true, Text: typed.Format("2006-01-02 15:04:05.999999")}, nil
	case bool:
		if typed {
			return sessionMigrationCell{Valid: true, Text: "1"}, nil
		}
		return sessionMigrationCell{Valid: true, Text: "0"}, nil
	case int64:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatInt(typed, 10)}, nil
	case int32:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatInt(int64(typed), 10)}, nil
	case int:
		return sessionMigrationCell{Valid: true, Text: strconv.Itoa(typed)}, nil
	case uint64:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatUint(typed, 10)}, nil
	case uint32:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatUint(uint64(typed), 10)}, nil
	case uint:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatUint(uint64(typed), 10)}, nil
	case float64:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatFloat(typed, 'g', -1, 64)}, nil
	case float32:
		return sessionMigrationCell{Valid: true, Text: strconv.FormatFloat(float64(typed), 'g', -1, 32)}, nil
	default:
		return sessionMigrationCell{}, fmt.Errorf("unsupported database value type %T", value)
	}
}

func sessionMigrationManifestEntryByTable(table string) (SessionMigrationManifestEntry, bool) {
	for _, entry := range SessionMigrationManifest() {
		if entry.Table == table {
			return entry, true
		}
	}
	return SessionMigrationManifestEntry{}, false
}

func sessionMigrationValidateManifestSnapshots(
	manifest []SessionMigrationManifestEntry,
	sourceRows map[string][]sessionMigrationRow,
	targetRows map[string][]sessionMigrationRow,
) (bool, error) {
	sourceTotal := 0
	targetCounts := make(map[string]int)
	targetStarter := false
	for _, entry := range manifest {
		if entry.Direct {
			sourceTotal += len(sourceRows[entry.Table])
			targetCounts[entry.Table] = len(targetRows[entry.Table])
		}
	}
	if sourceTotal == 0 {
		return false, errors.New("source session has no archive data")
	}
	if rows := targetRows["chat_logs"]; len(rows) == 1 {
		turn := rows[0].Values["turn_index"]
		role := rows[0].Values["role"]
		targetStarter = turn.Valid && strings.TrimSpace(turn.Text) == "0" &&
			role.Valid && strings.EqualFold(strings.TrimSpace(role.Text), "assistant")
	}
	occupancy := classifySessionMigrationOccupancy(targetCounts, targetStarter)
	if len(occupancy.BlockingTables) > 0 {
		tables := make([]string, 0, len(occupancy.BlockingTables))
		for table := range occupancy.BlockingTables {
			tables = append(tables, table)
		}
		sort.Strings(tables)
		return false, &SessionMigrationBlockerError{
			Code: "target_session_not_empty", Phase: "copy_transaction",
			Table: tables[0], Count: occupancy.BlockingTables[tables[0]],
		}
	}
	return occupancy.ReplaceableStarterOnly, nil
}

func classifySessionMigrationOccupancy(counts map[string]int, starter bool) SessionMigrationOccupancy {
	direct := make(map[string]int, len(counts))
	blocking := make(map[string]int)
	total := 0
	for _, entry := range SessionMigrationManifest() {
		if !entry.Direct {
			continue
		}
		count := counts[entry.Table]
		direct[entry.Table] = count
		total += count
		if count > 0 {
			blocking[entry.Table] = count
		}
	}
	replaceableStarterOnly := starter && total == 1 && direct["chat_logs"] == 1
	if replaceableStarterOnly {
		delete(blocking, "chat_logs")
	}
	return SessionMigrationOccupancy{
		DirectTableCounts: direct, TotalDirectRows: total,
		ReplaceableStarterOnly: replaceableStarterOnly, BlockingTables: blocking,
	}
}

func sessionMigrationLegacyCountsFromManifest(sourceRows map[string][]sessionMigrationRow) SessionMigrationArtifactCounts {
	counts := SessionMigrationArtifactCounts{
		ChatLogs:                 len(sourceRows["chat_logs"]),
		EffectiveInputs:          len(sourceRows["effective_input_logs"]),
		Memories:                 len(sourceRows["memories"]),
		DirectEvidence:           len(sourceRows["direct_evidence_records"]),
		KGTriples:                len(sourceRows["kg_triples"]),
		Episodes:                 len(sourceRows["episode_summaries"]),
		SubjectiveEntityMemories: len(sourceRows["protagonist_entity_memories"]),
		ReferenceBindings:        len(sourceRows["session_reference_bindings"]),
		ReferenceRuntimes:        0,
	}
	sessionMigrationFinalizeCounts(&counts)
	return counts
}

func sessionMigrationPrecomputeDeterministicKeys(
	manifest []SessionMigrationManifestEntry,
	sourceRows map[string][]sessionMigrationRow,
	targetSessionID string,
	maps *sessionMigrationKeyMaps,
) error {
	for _, entry := range manifest {
		if entry.Policy != SessionMigrationPolicyCopy {
			continue
		}
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		if len(plan.PrimaryKey) == 1 && entry.Direct && plan.PrimaryKey[0] == entry.SessionColumn {
			column := plan.PrimaryKey[0]
			for _, row := range sourceRows[entry.Table] {
				source := row.Values[column]
				if !source.Valid || strings.TrimSpace(source.Text) == "" {
					return fmt.Errorf("%s primary key %s is empty", entry.Table, column)
				}
				if err := maps.put(entry.Table, column, source.Text, targetSessionID); err != nil {
					return err
				}
			}
		} else if len(plan.PrimaryKey) == 1 && sessionMigrationPrecomputedPrimaryKey(plan.PrimaryKeyMode) {
			column := plan.PrimaryKey[0]
			for _, row := range sourceRows[entry.Table] {
				source := row.Values[column]
				if !source.Valid || strings.TrimSpace(source.Text) == "" {
					return fmt.Errorf("%s primary key %s is empty", entry.Table, column)
				}
				target := sessionMigrationDeterministicKey(plan.PrimaryKeyMode, SessionMigrationManifestVersion, targetSessionID, entry.Table, column, source.Text)
				if err := maps.put(entry.Table, column, source.Text, target); err != nil {
					return err
				}
			}
		}
		for _, generated := range plan.GeneratedKeys {
			for _, row := range sourceRows[entry.Table] {
				source := row.Values[generated.Column]
				if !source.Valid || strings.TrimSpace(source.Text) == "" {
					continue
				}
				target := sessionMigrationDeterministicUUID(SessionMigrationManifestVersion, targetSessionID, entry.Table, generated.Column, source.Text)
				if err := maps.put(entry.Table, generated.Column, source.Text, target); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func sessionMigrationInsertManifestRow(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	source sessionMigrationRow,
	targetSessionID string,
	maps *sessionMigrationKeyMaps,
) (sessionMigrationRow, string, []sessionMigrationDeferredFK, error) {
	if len(plan.PrimaryKey) != 1 {
		return sessionMigrationRow{}, "", nil, fmt.Errorf("copy table %s has unsupported composite primary key", entry.Table)
	}
	primaryKey := plan.PrimaryKey[0]
	sourceKey := source.Values[primaryKey]
	if !sourceKey.Valid || strings.TrimSpace(sourceKey.Text) == "" {
		return sessionMigrationRow{}, "", nil, fmt.Errorf("source primary key %s is empty", primaryKey)
	}
	target := sessionMigrationRow{Values: make(map[string]sessionMigrationCell, len(source.Values))}
	for column, value := range source.Values {
		target.Values[column] = value
	}
	if entry.Direct {
		target.Values[entry.SessionColumn] = sessionMigrationCell{Valid: true, Text: targetSessionID}
	}
	if entry.Table == "lorebook_reference_scopes" {
		if err := sessionMigrationRemapLorebookScopeIdentity(&target, targetSessionID); err != nil {
			return sessionMigrationRow{}, "", nil, err
		}
	}
	if sessionMigrationPrecomputedPrimaryKey(plan.PrimaryKeyMode) {
		value, ok := maps.target(entry.Table, primaryKey, sourceKey.Text)
		if !ok {
			return sessionMigrationRow{}, "", nil, fmt.Errorf("precomputed primary key map missing")
		}
		target.Values[primaryKey] = sessionMigrationCell{Valid: true, Text: value}
	}
	for _, generated := range plan.GeneratedKeys {
		sourceValue := source.Values[generated.Column]
		if !sourceValue.Valid {
			continue
		}
		targetValue, ok := maps.target(entry.Table, generated.Column, sourceValue.Text)
		if !ok {
			return sessionMigrationRow{}, "", nil, fmt.Errorf("generated key map missing for %s", generated.Column)
		}
		target.Values[generated.Column] = sessionMigrationCell{Valid: true, Text: targetValue}
	}
	deferred := []sessionMigrationDeferredFK{}
	for _, fk := range plan.ForeignKeys {
		sourceValue := source.Values[fk.Column]
		if !sourceValue.Valid || strings.TrimSpace(sourceValue.Text) == "" {
			continue
		}
		if fk.Deferred {
			target.Values[fk.Column] = sessionMigrationCell{}
			deferred = append(deferred, sessionMigrationDeferredFK{
				Table: entry.Table, PrimaryKey: primaryKey, Column: fk.Column,
				SourceValue: sourceValue.Text, Reference: fk,
			})
			continue
		}
		targetValue, ok := maps.target(fk.ReferenceTable, fk.ReferenceColumn, sourceValue.Text)
		if !ok {
			return sessionMigrationRow{}, "", nil, fmt.Errorf("FK %s has no row map for %s.%s=%q",
				fk.Column, fk.ReferenceTable, fk.ReferenceColumn, sourceValue.Text)
		}
		target.Values[fk.Column] = sessionMigrationCell{Valid: true, Text: targetValue}
	}
	if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil {
		return sessionMigrationRow{}, "", nil, err
	}
	if entry.Table == "memory_source_revisions" {
		if err := sessionMigrationValidateAdmissionResult(source); err != nil {
			return sessionMigrationRow{}, "", nil, err
		}
		resultHash, _, err := sessionMigrationRemappedAdmissionResult(target)
		if err != nil {
			return sessionMigrationRow{}, "", nil, err
		}
		if resultHash != "" {
			target.Values["derived_result_hash"] = sessionMigrationCell{Valid: true, Text: resultHash}
		}
	}
	insertColumns := make([]string, 0, len(plan.Columns))
	args := make([]any, 0, len(plan.Columns))
	for _, column := range plan.Columns {
		if column == primaryKey && plan.PrimaryKeyMode == SessionMigrationKeyAutoIncrement {
			continue
		}
		if sessionMigrationPlanDatabaseGenerated(plan, column) {
			continue
		}
		insertColumns = append(insertColumns, sessionMigrationQuoteIdentifier(column))
		value := target.Values[column]
		if value.Valid {
			args = append(args, value.Text)
		} else {
			args = append(args, nil)
		}
	}
	placeholders := make([]string, len(insertColumns))
	for index := range placeholders {
		placeholders[index] = "?"
	}
	query := "INSERT INTO " + sessionMigrationQuoteIdentifier(entry.Table) + " (" +
		strings.Join(insertColumns, ",") + ") VALUES (" + strings.Join(placeholders, ",") + ")"
	insertResult, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return sessionMigrationRow{}, "", nil, err
	}
	targetKey := target.Values[primaryKey].Text
	if plan.PrimaryKeyMode == SessionMigrationKeyAutoIncrement {
		insertID, err := insertResult.LastInsertId()
		if err != nil || insertID <= 0 {
			return sessionMigrationRow{}, "", nil, fmt.Errorf("target primary key readback failed: %w", err)
		}
		targetKey = strconv.FormatInt(insertID, 10)
		target.Values[primaryKey] = sessionMigrationCell{Valid: true, Text: targetKey}
		if err := maps.put(entry.Table, primaryKey, sourceKey.Text, targetKey); err != nil {
			return sessionMigrationRow{}, "", nil, err
		}
	}
	for index := range deferred {
		deferred[index].TargetKey = targetKey
	}
	if err := sessionMigrationInsertArtifactKeyMapTx(ctx, tx, migrationID, entry.Table, primaryKey, sourceKey.Text, targetKey, "copied"); err != nil {
		return sessionMigrationRow{}, "", nil, err
	}
	for _, generated := range plan.GeneratedKeys {
		sourceValue := source.Values[generated.Column]
		targetValue := target.Values[generated.Column]
		if sourceValue.Valid && targetValue.Valid {
			if err := sessionMigrationInsertArtifactKeyMapTx(ctx, tx, migrationID, entry.Table, generated.Column, sourceValue.Text, targetValue.Text, "alternate_key"); err != nil {
				return sessionMigrationRow{}, "", nil, err
			}
		}
	}
	if plan.PrimaryKeyMode == SessionMigrationKeyAutoIncrement {
		sourceID, parseSourceErr := strconv.ParseInt(sourceKey.Text, 10, 64)
		targetID, parseTargetErr := strconv.ParseInt(targetKey, 10, 64)
		if parseSourceErr == nil && parseTargetErr == nil {
			if err := insertSessionMigrationRowMap(ctx, tx, migrationID, entry.Table, sourceID, targetID); err != nil {
				return sessionMigrationRow{}, "", nil, err
			}
		}
	}
	if entry.Table == "session_reference_bindings" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO session_migration_reference_binding_map (
				migration_id, source_binding_id, target_binding_id, row_status
			) VALUES (?, ?, ?, 'copied')
		`, migrationID, sourceKey.Text, targetKey); err != nil {
			return sessionMigrationRow{}, "", nil, err
		}
	}
	return target, targetKey, deferred, nil
}

func sessionMigrationRemapLorebookScopeIdentity(target *sessionMigrationRow, targetSessionID string) error {
	if target == nil {
		return errors.New("lorebook reference scope target is required")
	}
	value := target.Values["scope_identity_json"]
	if !value.Valid || strings.TrimSpace(value.Text) == "" {
		return errors.New("lorebook_reference_scopes scope_identity_json is empty")
	}
	var scope LorebookReferenceScope
	if err := json.Unmarshal([]byte(value.Text), &scope); err != nil {
		return fmt.Errorf("lorebook_reference_scopes scope_identity_json: %w", err)
	}
	scope.ChatSessionID = strings.TrimSpace(targetSessionID)
	_, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		return fmt.Errorf("lorebook_reference_scopes scope_identity_json: %w", err)
	}
	target.Values["scope_identity_json"] = sessionMigrationCell{Valid: true, Text: identityJSON}
	return nil
}

func sessionMigrationRemappedAdmissionResult(row sessionMigrationRow) (string, string, error) {
	committed := strings.EqualFold(strings.TrimSpace(row.Values["derived_admission_state"].Text), "committed")
	if strings.TrimSpace(row.Values["derived_result_hash"].Text) == "" {
		if committed {
			return "", "", errors.New("session migration committed derived result contract is incomplete")
		}
		return "", "", nil
	}
	resultJSON := strings.TrimSpace(row.Values["derived_result_json"].Text)
	var result any
	if resultJSON == "" || json.Unmarshal([]byte(resultJSON), &result) != nil || result == nil {
		return "", "", errors.New("session migration committed derived result is invalid")
	}
	admission := &MemoryAdmission{
		SourceRevision:    row.Values["source_revision"].Text,
		DerivationVersion: row.Values["derived_admission_version"].Text,
		ExtractorVersion:  row.Values["derived_extractor_version"].Text,
		IndexVersion:      row.Values["derived_index_version"].Text,
		ResultJSON:        resultJSON,
	}
	if strings.TrimSpace(admission.SourceRevision) == "" ||
		strings.TrimSpace(admission.DerivationVersion) == "" ||
		strings.TrimSpace(admission.ExtractorVersion) == "" ||
		strings.TrimSpace(admission.IndexVersion) == "" {
		return "", "", errors.New("session migration committed derived result contract is incomplete")
	}
	return memoryAdmissionExpectedResultHash(admission), admission.ResultJSON, nil
}

func sessionMigrationValidateAdmissionResult(row sessionMigrationRow) error {
	expectedHash, _, err := sessionMigrationRemappedAdmissionResult(row)
	if err != nil {
		return err
	}
	if expectedHash != "" && !strings.EqualFold(strings.TrimSpace(row.Values["derived_result_hash"].Text), expectedHash) {
		return errors.New("session migration committed derived result hash does not match its source revision")
	}
	return nil
}

func sessionMigrationInsertArtifactKeyMapTx(ctx context.Context, tx *sql.Tx, migrationID int64, table, column, source, target, rowStatus string) error {
	if rowStatus != "copied" && rowStatus != "alternate_key" {
		return fmt.Errorf("unsupported session migration row-map status %q", rowStatus)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO session_migration_artifact_row_map (
			migration_id, table_name, key_column_name, source_key, target_key, row_status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, migrationID, table, column, source, target, rowStatus)
	return err
}

func sessionMigrationExpectedVectorDocument(
	migrationID int64,
	table string,
	plan SessionMigrationExecutionPlan,
	sourceRow, targetRow sessionMigrationRow,
	sourceSessionID, targetSessionID string,
) (SessionMigrationVectorDocument, bool) {
	if plan.Vector == nil {
		return SessionMigrationVectorDocument{}, false
	}
	if !sessionMigrationVectorRowEligible(plan.Vector, targetRow) {
		return SessionMigrationVectorDocument{}, false
	}
	embeddingJSON := ""
	if plan.Vector.EmbeddingColumn != "" {
		embedding := targetRow.Values[plan.Vector.EmbeddingColumn]
		if embedding.Valid {
			embeddingJSON = strings.TrimSpace(embedding.Text)
		}
	}
	documentText := sessionMigrationVectorDocumentText(plan.Vector, targetRow)
	if documentText == "" {
		return SessionMigrationVectorDocument{}, false
	}
	vectorKey := targetRow.Values[plan.Vector.IDColumn]
	if !vectorKey.Valid || strings.TrimSpace(vectorKey.Text) == "" {
		return SessionMigrationVectorDocument{}, false
	}
	sourceVectorKey := sourceRow.Values[plan.Vector.IDColumn]
	if !sourceVectorKey.Valid || strings.TrimSpace(sourceVectorKey.Text) == "" {
		return SessionMigrationVectorDocument{}, false
	}
	contextTurnIndex, contextTurnKnown := sessionMigrationVectorContextTurn(plan.Vector, targetRow)
	return SessionMigrationVectorDocument{
		ID:                    plan.Vector.Tier + ":" + targetSessionID + ":" + vectorKey.Text,
		MigrationID:           migrationID,
		Tier:                  plan.Vector.Tier,
		ChatSessionID:         targetSessionID,
		ContextTurnIndex:      contextTurnIndex,
		ContextTurnKnown:      contextTurnKnown,
		SourceTable:           table,
		SourceRowID:           sourceVectorKey.Text,
		SchemaVersion:         plan.Vector.SchemaVersion,
		DocumentText:          documentText,
		EmbeddingJSON:         embeddingJSON,
		MigratedFromSessionID: sourceSessionID,
	}, true
}

func sessionMigrationVectorContextTurn(plan *SessionMigrationVectorPlan, row sessionMigrationRow) (int, bool) {
	if plan == nil || len(plan.ContextTurnColumns) == 0 {
		return 0, false
	}
	turn := 0
	known := false
	for _, column := range plan.ContextTurnColumns {
		cell := row.Values[column]
		if !cell.Valid {
			continue
		}
		candidate := sessionMigrationCellInt(cell)
		if candidate < 0 || (candidate == 0 && !plan.ContextTurnZeroValid) {
			continue
		}
		if !known || candidate > turn {
			turn = candidate
		}
		known = true
	}
	return turn, known
}

func sessionMigrationVectorRowEligible(plan *SessionMigrationVectorPlan, row sessionMigrationRow) bool {
	switch plan.Eligibility {
	case "":
		return true
	case "active_direct_evidence":
		return !sessionMigrationCellBool(row.Values["tombstoned"]) &&
			!sessionMigrationCellBool(row.Values["repair_needed"]) &&
			sessionMigrationCellNonPositive(row.Values["superseded_by_id"]) &&
			!strings.EqualFold(strings.TrimSpace(row.Values["evidence_kind"].Text), "perspective_scoped_turn_excerpt") &&
			strings.TrimSpace(row.Values["evidence_text"].Text) != ""
	case "active_world_rule":
		return !sessionMigrationCellBool(row.Values["suppressed"]) &&
			sessionMigrationVectorDocumentText(plan, row) != ""
	case "active_precise_memory":
		return strings.EqualFold(strings.TrimSpace(row.Values["lifecycle_state"].Text), "active") &&
			PreciseMemoryGeneralVectorEligible(&PreciseMemoryUnit{
				AdmissionState:          row.Values["admission_state"].Text,
				ReviewState:             row.Values["review_state"].Text,
				Visibility:              row.Values["visibility"].Text,
				EpistemicMode:           row.Values["epistemic_mode"].Text,
				KnowledgeHolderEntityID: row.Values["knowledge_holder_entity_id"].Text,
			})
	default:
		return false
	}
}

func sessionMigrationVectorDocumentText(plan *SessionMigrationVectorPlan, row sessionMigrationRow) string {
	value := func(column string) string {
		cell := row.Values[column]
		if !cell.Valid {
			return ""
		}
		return strings.TrimSpace(cell.Text)
	}
	switch plan.TextFormat {
	case "direct_evidence":
		parts := []string{}
		if kind := value("evidence_kind"); kind != "" {
			parts = append(parts, "kind: "+kind)
		}
		if text := value("evidence_text"); text != "" {
			parts = append(parts, text)
		}
		start := sessionMigrationCellInt(row.Values["source_turn_start"])
		end := sessionMigrationCellInt(row.Values["source_turn_end"])
		anchor := sessionMigrationCellInt(row.Values["turn_anchor"])
		if start > 0 || end > 0 || anchor > 0 {
			parts = append(parts, fmt.Sprintf("turns: %d-%d anchor:%d", start, end, anchor))
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case "world_rule", "kg_triple", "plain":
		parts := []string{}
		for _, column := range plan.TextColumns {
			if text := value(column); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case "precise_memory":
		return PreciseMemorySemanticText(&PreciseMemoryUnit{
			Kind:            value("memory_kind"),
			Subtype:         value("memory_subtype"),
			PayloadJSON:     value("payload_json"),
			EvidenceExcerpt: value("evidence_excerpt"),
		})
	case "memory":
		return sessionMigrationMemoryDocumentText(
			value("summary_json"),
			value("evidence"),
			value("place_wing"),
			value("place_room"),
		)
	default:
		return ""
	}
}

func sessionMigrationCellBool(cell sessionMigrationCell) bool {
	if !cell.Valid {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cell.Text)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func sessionMigrationCellInt(cell sessionMigrationCell) int {
	if !cell.Valid {
		return 0
	}
	value, _ := strconv.Atoi(strings.TrimSpace(cell.Text))
	return value
}

func sessionMigrationCellNonPositive(cell sessionMigrationCell) bool {
	return !cell.Valid || sessionMigrationCellInt(cell) <= 0
}

func sessionMigrationVerifyAndPersistRelationalParity(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	manifest []SessionMigrationManifestEntry,
	sourceRows map[string][]sessionMigrationRow,
	sourceSessionID, targetSessionID string,
	keyMaps *sessionMigrationKeyMaps,
) error {
	for _, entry := range manifest {
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		targetRows, err := sessionMigrationReadManifestRows(ctx, tx, entry, plan, targetSessionID)
		if err != nil {
			return fmt.Errorf("session migration target parity read %s: %w", entry.Table, err)
		}
		sourceHash := sessionMigrationCanonicalRowsHash(entry, plan, sourceRows[entry.Table], sourceSessionID, false, keyMaps)
		targetHash := sessionMigrationCanonicalRowsHash(entry, plan, targetRows, targetSessionID, true, keyMaps)
		evaluation, err := sessionMigrationEvaluateArtifactParity(entry, plan, sourceRows[entry.Table], targetRows, sourceHash, targetHash, keyMaps)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO session_migration_artifact_parity (
				migration_id, manifest_version, table_name, parent_table_name,
				session_column_name, migration_policy, source_row_count,
				source_content_hash, target_row_count, target_content_hash,
				row_map_expected_count, row_map_verified_count,
				fk_expected_count, fk_verified_count, parity_state,
				blocker_code, verified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP(3))
		`, migrationID, SessionMigrationManifestVersion, entry.Table,
			nullableString(entry.ParentTable), nullableString(entry.SessionColumn), entry.Policy,
			len(sourceRows[entry.Table]), sourceHash, len(targetRows), targetHash,
			evaluation.RowMapExpected, evaluation.RowMapVerified,
			evaluation.FKExpected, evaluation.FKVerified, evaluation.ParityState); err != nil {
			return err
		}
	}
	return nil
}

type sessionMigrationArtifactParityEvaluation struct {
	RowMapExpected int
	RowMapVerified int
	FKExpected     int
	FKVerified     int
	ParityState    string
}

func sessionMigrationEvaluateArtifactParity(
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	sourceRows, targetRows []sessionMigrationRow,
	sourceHash, targetHash string,
	keyMaps *sessionMigrationKeyMaps,
) (sessionMigrationArtifactParityEvaluation, error) {
	evaluation := sessionMigrationArtifactParityEvaluation{
		FKExpected: sessionMigrationFKValueCount(sourceRows, plan),
	}
	switch entry.Policy {
	case SessionMigrationPolicyCopy:
		evaluation.RowMapExpected = len(sourceRows)
		evaluation.RowMapVerified = sessionMigrationMappedRowCount(sourceRows, entry.Table, plan, keyMaps)
		evaluation.FKVerified = sessionMigrationVerifiedFKValueCount(sourceRows, targetRows, entry, plan, keyMaps)
		if len(targetRows) != len(sourceRows) || sourceHash != targetHash ||
			evaluation.RowMapExpected != evaluation.RowMapVerified ||
			evaluation.FKExpected != evaluation.FKVerified {
			return evaluation, fmt.Errorf("session migration parity mismatch for %s: source=%d/%s target=%d/%s row_map=%d/%d fk=%d/%d",
				entry.Table, len(sourceRows), sourceHash, len(targetRows), targetHash,
				evaluation.RowMapVerified, evaluation.RowMapExpected, evaluation.FKVerified, evaluation.FKExpected)
		}
		evaluation.ParityState = "verified_copy"
	case SessionMigrationPolicyRetainAudit:
		if len(targetRows) != 0 {
			return evaluation, fmt.Errorf("session migration retain-audit target %s is not empty", entry.Table)
		}
		evaluation.ParityState = "verified_retain_audit"
	case SessionMigrationPolicyRegenerate:
		if len(targetRows) != 0 {
			return evaluation, fmt.Errorf("session migration regenerate target %s was mutated", entry.Table)
		}
		evaluation.ParityState = "verified_regenerate"
	case SessionMigrationPolicyDeleteAfterVerified:
		if len(targetRows) != 0 {
			return evaluation, fmt.Errorf("session migration delete-after-verified target %s was mutated", entry.Table)
		}
		evaluation.ParityState = "verified_delete_pending"
	default:
		return evaluation, fmt.Errorf("session migration unsupported policy %q", entry.Policy)
	}
	return evaluation, nil
}

func sessionMigrationCanonicalRowsHash(
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	rows []sessionMigrationRow,
	sessionID string,
	target bool,
	keyMaps *sessionMigrationKeyMaps,
) string {
	encodedRows := make([]string, 0, len(rows))
	for _, row := range rows {
		var builder strings.Builder
		for _, column := range plan.Columns {
			if sessionMigrationPlanDatabaseGenerated(plan, column) {
				continue
			}
			value := row.Values[column]
			builder.WriteString(strconv.Itoa(len(column)))
			builder.WriteByte(':')
			builder.WriteString(column)
			builder.WriteByte('=')
			if !value.Valid {
				builder.WriteString("-1:")
				continue
			}
			textValue := value.Text
			if entry.Direct && column == entry.SessionColumn {
				textValue = "<session>"
			} else if entry.Table == "lorebook_reference_scopes" && column == "scope_identity_json" {
				if canonical, ok := sessionMigrationCanonicalLorebookScopeIdentity(textValue); ok {
					textValue = canonical
				}
			} else if entry.Table == "memory_source_revisions" && column == "derived_result_hash" && strings.TrimSpace(textValue) != "" {
				ownHash, _, ownErr := sessionMigrationRemappedAdmissionResult(row)
				if ownErr == nil && ownHash != "" && strings.EqualFold(strings.TrimSpace(textValue), ownHash) {
					canonicalRow := row
					if target {
						canonicalRow.Values = make(map[string]sessionMigrationCell, len(row.Values))
						for key, cell := range row.Values {
							canonicalRow.Values[key] = cell
						}
						if revision := row.Values["source_revision"]; revision.Valid {
							if sourceRevision, ok := keyMaps.source(entry.Table, "source_revision", revision.Text); ok {
								canonicalRow.Values["source_revision"] = sessionMigrationCell{Valid: true, Text: sourceRevision}
							}
						}
					}
					if canonicalHash, _, err := sessionMigrationRemappedAdmissionResult(canonicalRow); err == nil && canonicalHash != "" {
						textValue = canonicalHash
					}
				}
			} else if !target {
				textValue = sessionMigrationCanonicalSemanticSourceValue(plan, column, textValue)
			} else if target {
				if normalized, ok := keyMaps.source(entry.Table, column, textValue); ok {
					textValue = normalized
				} else {
					for _, fk := range plan.ForeignKeys {
						if fk.Column == column {
							if normalized, ok := keyMaps.source(fk.ReferenceTable, fk.ReferenceColumn, textValue); ok {
								textValue = normalized
							}
							break
						}
					}
					if normalized, ok := sessionMigrationNormalizeSemanticReference(plan, row, column, textValue, keyMaps); ok {
						textValue = normalized
					}
				}
			}
			builder.WriteString(strconv.Itoa(len(textValue)))
			builder.WriteByte(':')
			builder.WriteString(textValue)
			builder.WriteByte(';')
		}
		encodedRows = append(encodedRows, builder.String())
	}
	sort.Strings(encodedRows)
	hash := sha256.New()
	for _, encoded := range encodedRows {
		hash.Write([]byte(encoded))
		hash.Write([]byte{0})
	}
	_ = sessionID
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func sessionMigrationCanonicalLorebookScopeIdentity(raw string) (string, bool) {
	var scope LorebookReferenceScope
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &scope) != nil {
		return "", false
	}
	scope.ChatSessionID = "<session>"
	_, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		return "", false
	}
	return identityJSON, true
}

func sessionMigrationPlanDatabaseGenerated(plan SessionMigrationExecutionPlan, column string) bool {
	for _, generated := range plan.DatabaseGenerated {
		if generated == column {
			return true
		}
	}
	return false
}

func sessionMigrationFKValueCount(rows []sessionMigrationRow, plan SessionMigrationExecutionPlan) int {
	count := 0
	for _, row := range rows {
		for _, fk := range plan.ForeignKeys {
			if value := row.Values[fk.Column]; value.Valid && strings.TrimSpace(value.Text) != "" {
				count++
			}
		}
		for _, semantic := range plan.SemanticReferences {
			count += sessionMigrationSemanticReferenceValueCount(row, semantic)
		}
	}
	return count
}

func sessionMigrationMappedRowCount(rows []sessionMigrationRow, table string, plan SessionMigrationExecutionPlan, maps *sessionMigrationKeyMaps) int {
	if len(plan.PrimaryKey) != 1 {
		return 0
	}
	count := 0
	for _, row := range rows {
		value := row.Values[plan.PrimaryKey[0]]
		if value.Valid {
			if _, ok := maps.target(table, plan.PrimaryKey[0], value.Text); ok {
				count++
			}
		}
	}
	return count
}

func sessionMigrationVerifiedFKValueCount(
	sourceRows, targetRows []sessionMigrationRow,
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	maps *sessionMigrationKeyMaps,
) int {
	if len(plan.PrimaryKey) != 1 {
		return 0
	}
	sourceByKey := map[string]sessionMigrationRow{}
	for _, row := range sourceRows {
		sourceByKey[row.Values[plan.PrimaryKey[0]].Text] = row
	}
	count := 0
	for _, targetRow := range targetRows {
		targetKey := targetRow.Values[plan.PrimaryKey[0]]
		sourceKey, ok := maps.source(entry.Table, plan.PrimaryKey[0], targetKey.Text)
		if !ok {
			continue
		}
		sourceRow, ok := sourceByKey[sourceKey]
		if !ok {
			continue
		}
		for _, fk := range plan.ForeignKeys {
			sourceValue := sourceRow.Values[fk.Column]
			if !sourceValue.Valid || strings.TrimSpace(sourceValue.Text) == "" {
				continue
			}
			targetValue := targetRow.Values[fk.Column]
			if !targetValue.Valid {
				continue
			}
			normalized, ok := maps.source(fk.ReferenceTable, fk.ReferenceColumn, targetValue.Text)
			if ok && normalized == sourceValue.Text {
				count++
			}
		}
		for _, semantic := range plan.SemanticReferences {
			count += sessionMigrationVerifiedSemanticReferenceCount(sourceRow, targetRow, semantic, maps)
		}
	}
	return count
}

func sessionMigrationRemapSemanticReferences(
	plan SessionMigrationExecutionPlan,
	source sessionMigrationRow,
	target *sessionMigrationRow,
	maps *sessionMigrationKeyMaps,
) error {
	for _, semantic := range plan.SemanticReferences {
		sourceValue := source.Values[semantic.Column]
		if !sourceValue.Valid || strings.TrimSpace(sourceValue.Text) == "" {
			continue
		}
		switch semantic.Kind {
		case SessionMigrationSemanticJSONIDArray:
			mapped, err := sessionMigrationRemapJSONIDArray(sourceValue.Text, semantic.References["default"], maps, false)
			if err != nil {
				return fmt.Errorf("semantic reference %s: %w", semantic.Column, err)
			}
			target.Values[semantic.Column] = sessionMigrationCell{Valid: true, Text: mapped}
		case SessionMigrationSemanticTypedArtifactID:
			referenceType := strings.TrimSpace(source.Values[semantic.TypeColumn].Text)
			reference, ok := semantic.References[referenceType]
			if !ok {
				return fmt.Errorf("semantic reference %s has unsupported %s %q", semantic.Column, semantic.TypeColumn, referenceType)
			}
			mapped, ok := maps.target(reference.Table, reference.Column, sourceValue.Text)
			if !ok {
				return fmt.Errorf("semantic reference %s has no row map for %s.%s=%q",
					semantic.Column, reference.Table, reference.Column, sourceValue.Text)
			}
			target.Values[semantic.Column] = sessionMigrationCell{Valid: true, Text: mapped}
		case SessionMigrationSemanticPrefixedID:
			prefix := semantic.ReferenceType + ":"
			if !strings.HasPrefix(sourceValue.Text, prefix) {
				return fmt.Errorf("semantic reference %s must use %s prefix", semantic.Column, prefix)
			}
			reference := semantic.References[semantic.ReferenceType]
			sourceID := strings.TrimPrefix(sourceValue.Text, prefix)
			mapped, ok := maps.target(reference.Table, reference.Column, sourceID)
			if !ok {
				return fmt.Errorf("semantic reference %s has no row map for %s.%s=%q",
					semantic.Column, reference.Table, reference.Column, sourceID)
			}
			target.Values[semantic.Column] = sessionMigrationCell{Valid: true, Text: prefix + mapped}
		default:
			return fmt.Errorf("unsupported semantic reference kind %q", semantic.Kind)
		}
	}
	return nil
}

func sessionMigrationNormalizeSemanticReference(
	plan SessionMigrationExecutionPlan,
	row sessionMigrationRow,
	column, targetValue string,
	maps *sessionMigrationKeyMaps,
) (string, bool) {
	for _, semantic := range plan.SemanticReferences {
		if semantic.Column != column {
			continue
		}
		switch semantic.Kind {
		case SessionMigrationSemanticJSONIDArray:
			normalized, err := sessionMigrationRemapJSONIDArray(targetValue, semantic.References["default"], maps, true)
			return normalized, err == nil
		case SessionMigrationSemanticTypedArtifactID:
			reference, ok := semantic.References[strings.TrimSpace(row.Values[semantic.TypeColumn].Text)]
			if !ok {
				return "", false
			}
			normalized, ok := maps.source(reference.Table, reference.Column, targetValue)
			return normalized, ok
		case SessionMigrationSemanticPrefixedID:
			prefix := semantic.ReferenceType + ":"
			if !strings.HasPrefix(targetValue, prefix) {
				return "", false
			}
			reference := semantic.References[semantic.ReferenceType]
			normalized, ok := maps.source(reference.Table, reference.Column, strings.TrimPrefix(targetValue, prefix))
			return prefix + normalized, ok
		}
	}
	return "", false
}

func sessionMigrationCanonicalSemanticSourceValue(plan SessionMigrationExecutionPlan, column, value string) string {
	for _, semantic := range plan.SemanticReferences {
		if semantic.Column != column || semantic.Kind != SessionMigrationSemanticJSONIDArray {
			continue
		}
		ids := []int64{}
		if json.Unmarshal([]byte(strings.TrimSpace(value)), &ids) != nil {
			return value
		}
		encoded, err := json.Marshal(ids)
		if err == nil {
			return string(encoded)
		}
	}
	return value
}

func sessionMigrationRemapJSONIDArray(
	raw string,
	reference SessionMigrationArtifactReference,
	maps *sessionMigrationKeyMaps,
	inverse bool,
) (string, error) {
	ids := []int64{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &ids); err != nil {
		return "", fmt.Errorf("invalid numeric ID array: %w", err)
	}
	mapped := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return "", fmt.Errorf("invalid referenced ID %d", id)
		}
		value := strconv.FormatInt(id, 10)
		var next string
		var ok bool
		if inverse {
			next, ok = maps.source(reference.Table, reference.Column, value)
		} else {
			next, ok = maps.target(reference.Table, reference.Column, value)
		}
		if !ok {
			return "", fmt.Errorf("no row map for %s.%s=%q", reference.Table, reference.Column, value)
		}
		parsed, err := strconv.ParseInt(next, 10, 64)
		if err != nil || parsed <= 0 {
			return "", fmt.Errorf("mapped ID %q is not positive numeric", next)
		}
		mapped = append(mapped, parsed)
	}
	encoded, err := json.Marshal(mapped)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func sessionMigrationSemanticReferenceValueCount(row sessionMigrationRow, semantic SessionMigrationSemanticReferencePlan) int {
	value := row.Values[semantic.Column]
	if !value.Valid || strings.TrimSpace(value.Text) == "" {
		return 0
	}
	if semantic.Kind != SessionMigrationSemanticJSONIDArray {
		return 1
	}
	ids := []int64{}
	if json.Unmarshal([]byte(value.Text), &ids) != nil {
		return 1
	}
	return len(ids)
}

func sessionMigrationVerifiedSemanticReferenceCount(
	sourceRow, targetRow sessionMigrationRow,
	semantic SessionMigrationSemanticReferencePlan,
	maps *sessionMigrationKeyMaps,
) int {
	sourceValue := sourceRow.Values[semantic.Column]
	if !sourceValue.Valid || strings.TrimSpace(sourceValue.Text) == "" {
		return 0
	}
	targetValue := targetRow.Values[semantic.Column]
	if !targetValue.Valid {
		return 0
	}
	plan := SessionMigrationExecutionPlan{SemanticReferences: []SessionMigrationSemanticReferencePlan{semantic}}
	normalized, ok := sessionMigrationNormalizeSemanticReference(plan, targetRow, semantic.Column, targetValue.Text, maps)
	sourceCanonical := sessionMigrationCanonicalSemanticSourceValue(plan, semantic.Column, sourceValue.Text)
	if !ok || normalized != sourceCanonical {
		return 0
	}
	return sessionMigrationSemanticReferenceValueCount(sourceRow, semantic)
}

func sessionMigrationPersistVectorExpectedParityTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	expected map[string]SessionMigrationVectorDocument,
) error {
	idsByTable := map[string][]string{}
	for id, doc := range expected {
		idsByTable[doc.SourceTable] = append(idsByTable[doc.SourceTable], id)
	}
	for table, ids := range idsByTable {
		sort.Strings(ids)
		if _, err := tx.ExecContext(ctx, `
			UPDATE session_migration_artifact_parity
			SET vector_expected_count = ?,
			    vector_expected_id_hash = ?,
			    updated_at = CURRENT_TIMESTAMP(3)
			WHERE migration_id = ? AND manifest_version = ? AND table_name = ?
		`, len(ids), sessionMigrationHashIDs(ids), migrationID, SessionMigrationManifestVersion, table); err != nil {
			return err
		}
	}
	return nil
}

func sessionMigrationUpsertSagaStepTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	phase, state, requestHash, resultJSON, lastError string,
) error {
	if strings.TrimSpace(resultJSON) == "" {
		resultJSON = "{}"
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO session_migration_saga_steps (
			migration_id, phase, phase_state, attempt_count, request_hash,
			result_json, last_error, started_at, completed_at
		) VALUES (?, ?, ?, 1, ?, ?, ?, CURRENT_TIMESTAMP(3),
		          CASE WHEN ? = 'completed' THEN CURRENT_TIMESTAMP(3) ELSE NULL END)
		ON DUPLICATE KEY UPDATE
			phase_state = VALUES(phase_state),
			attempt_count = attempt_count + 1,
			request_hash = VALUES(request_hash),
			result_json = VALUES(result_json),
			last_error = VALUES(last_error),
			completed_at = VALUES(completed_at),
			updated_at = CURRENT_TIMESTAMP(3)
	`, migrationID, phase, state, nullableString(requestHash), resultJSON, nullableString(lastError), state)
	return err
}

func sessionMigrationQuoteIdentifier(identifier string) string {
	for _, r := range identifier {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			panic("unsafe session migration identifier: " + identifier)
		}
	}
	return "`" + identifier + "`"
}

func sessionMigrationDeterministicUUID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	hex := fmt.Sprintf("%x", bytes)
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}

func sessionMigrationDeterministicDigest32(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:16])
}

func sessionMigrationPrecomputedPrimaryKey(mode string) bool {
	return mode == SessionMigrationKeyUUID || mode == SessionMigrationKeyDigest32
}

func sessionMigrationDeterministicKey(mode string, parts ...string) string {
	if mode == SessionMigrationKeyDigest32 {
		return sessionMigrationDeterministicDigest32(parts...)
	}
	return sessionMigrationDeterministicUUID(parts...)
}

func sessionMigrationStringHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func sessionMigrationHashIDs(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return sessionMigrationStringHash(sorted...)
}

func (m *mariadbStore) ListSessionMigrationVectorDocuments(ctx context.Context, migrationID int64) ([]SessionMigrationVectorDocument, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	docs := []SessionMigrationVectorDocument{}
	for _, entry := range SessionMigrationManifest() {
		plan, ok := SessionMigrationExecutionPlanFor(entry.Table)
		if !ok || plan.Vector == nil || len(plan.PrimaryKey) != 1 {
			continue
		}
		tableDocs, err := sessionMigrationListVectorDocumentsForPlan(ctx, m.db, migrationID, entry.Table, plan)
		if err != nil {
			return nil, err
		}
		docs = append(docs, tableDocs...)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	return docs, nil
}

func sessionMigrationListVectorDocumentsForPlan(
	ctx context.Context,
	db *sql.DB,
	migrationID int64,
	table string,
	plan SessionMigrationExecutionPlan,
) ([]SessionMigrationVectorDocument, error) {
	vectorPlan := plan.Vector
	selectText := make([]string, len(vectorPlan.TextColumns))
	for index, column := range vectorPlan.TextColumns {
		selectText[index] = "t." + sessionMigrationQuoteIdentifier(column)
	}
	selectContextTurns := make([]string, len(vectorPlan.ContextTurnColumns))
	for index, column := range vectorPlan.ContextTurnColumns {
		selectContextTurns[index] = "t." + sessionMigrationQuoteIdentifier(column)
	}
	embeddingSelect := "''"
	if vectorPlan.EmbeddingColumn != "" {
		embeddingSelect = "t." + sessionMigrationQuoteIdentifier(vectorPlan.EmbeddingColumn)
	}
	query := `
		SELECT ve.document_id, sm.source_session_id, sm.target_session_id,
		       arm.target_key, ` + embeddingSelect
	if len(selectText) > 0 {
		query += "," + strings.Join(selectText, ",")
	}
	if len(selectContextTurns) > 0 {
		query += "," + strings.Join(selectContextTurns, ",")
	}
	query += `
		FROM session_migration_vector_expected_ids ve
		JOIN session_migrations sm ON sm.id = ve.migration_id
		JOIN session_migration_artifact_row_map arm
		  ON arm.migration_id = ve.migration_id
		 AND arm.table_name = ve.source_table
		 AND arm.key_column_name = ?
		 AND arm.source_key = ve.source_row_id
		 AND arm.row_status <> 'rolled_back'
		JOIN ` + sessionMigrationQuoteIdentifier(table) + ` t
		  ON CAST(t.` + sessionMigrationQuoteIdentifier(vectorPlan.IDColumn) + ` AS CHAR) = arm.target_key
		WHERE ve.migration_id = ? AND ve.source_table = ?
		ORDER BY ve.document_id`
	rows, err := db.QueryContext(ctx, query, vectorPlan.IDColumn, migrationID, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	docs := []SessionMigrationVectorDocument{}
	for rows.Next() {
		var id, sourceSessionID, targetSessionID, targetKey, embedding string
		textValues := make([]sql.NullString, len(vectorPlan.TextColumns))
		contextTurnValues := make([]sql.NullString, len(vectorPlan.ContextTurnColumns))
		dest := []any{&id, &sourceSessionID, &targetSessionID, &targetKey, &embedding}
		for index := range textValues {
			dest = append(dest, &textValues[index])
		}
		for index := range contextTurnValues {
			dest = append(dest, &contextTurnValues[index])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		vectorRow := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
		for index, value := range textValues {
			vectorRow.Values[vectorPlan.TextColumns[index]] = sessionMigrationCell{Valid: value.Valid, Text: value.String}
		}
		for index, value := range contextTurnValues {
			vectorRow.Values[vectorPlan.ContextTurnColumns[index]] = sessionMigrationCell{Valid: value.Valid, Text: value.String}
		}
		contextTurnIndex, contextTurnKnown := sessionMigrationVectorContextTurn(vectorPlan, vectorRow)
		docs = append(docs, SessionMigrationVectorDocument{
			ID:                    id,
			MigrationID:           migrationID,
			Tier:                  vectorPlan.Tier,
			ChatSessionID:         targetSessionID,
			ContextTurnIndex:      contextTurnIndex,
			ContextTurnKnown:      contextTurnKnown,
			SourceTable:           table,
			SourceRowID:           targetKey,
			SchemaVersion:         vectorPlan.SchemaVersion,
			DocumentText:          sessionMigrationVectorDocumentText(vectorPlan, vectorRow),
			EmbeddingJSON:         embedding,
			MigratedFromSessionID: sourceSessionID,
		})
	}
	return docs, rows.Err()
}

func (m *mariadbStore) UpdateSessionMigrationVectorStatus(ctx context.Context, migrationID int64, status string, reindexedCount int, errorsJSON string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if migrationID <= 0 {
		return ErrNotFound
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "vector_reindexed"
	}
	if strings.TrimSpace(errorsJSON) == "" {
		errorsJSON = "[]"
	}
	_, err := m.db.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = ?,
		    chroma_reindexed_count = ?,
		    errors_json = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, status, reindexedCount, errorsJSON, migrationID)
	return err
}

func (m *mariadbStore) GetSessionMigrationVectorParityContext(ctx context.Context, migrationID int64) (*SessionMigrationVectorParityContext, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	var targetSessionID string
	err := m.db.QueryRowContext(ctx, `
		SELECT target_session_id
		FROM session_migrations
		WHERE id = ? AND status NOT IN ('rolled_back', 'rollback_partial')
	`, migrationID).Scan(&targetSessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT document_id
		FROM session_migration_vector_expected_ids
		WHERE migration_id = ?
		ORDER BY document_id
	`, migrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expected := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		expected = append(expected, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &SessionMigrationVectorParityContext{
		MigrationID:     migrationID,
		TargetSessionID: targetSessionID,
		ExpectedIDs:     expected,
	}, nil
}

func (m *mariadbStore) VerifySessionMigrationVectorParity(
	ctx context.Context,
	migrationID int64,
	operation string,
	actualIDs []string,
) (*SessionMigrationVectorParityResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	parityContext, err := sessionMigrationVectorParityContextTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	migrationStatus, err := sessionMigrationValidateProofOperationTx(ctx, tx, migrationID, operation)
	if err != nil {
		return nil, err
	}
	allowUnexpected := operation == SessionMigrationProofOperationCleanupPrepare ||
		operation == SessionMigrationProofOperationCleanupFinalize ||
		(operation == SessionMigrationProofOperationResume &&
			(migrationStatus == "source_locked" ||
				migrationStatus == "cleanup_prepared" ||
				migrationStatus == "source_cleaned"))
	result := sessionMigrationCompareVectorIDsWithPolicy(
		migrationID,
		parityContext.TargetSessionID,
		parityContext.ExpectedIDs,
		actualIDs,
		allowUnexpected,
	)
	expected := result.ExpectedIDs
	actual := result.ActualIDs
	missing := result.MissingIDs
	unexpected := result.UnexpectedIDs
	expectedSet := make(map[string]bool, len(expected))
	actualSet := make(map[string]bool, len(actual))
	for _, id := range expected {
		expectedSet[id] = true
	}
	for _, id := range actual {
		actualSet[id] = true
	}
	relationalHash, err := sessionMigrationRevalidateCurrentRelationalStateTx(ctx, tx, migrationID, "vector_verify", false)
	if err != nil {
		return result, err
	}
	if err := sessionMigrationVerifyExpectedVectorLedgerTx(ctx, tx, migrationID); err != nil {
		return result, sessionMigrationBlocker("current_vector_ledger_drift", "vector_verify", "")
	}
	if !result.Verified {
		return result, sessionMigrationBlocker("current_vector_id_drift", "vector_verify", "")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migration_vector_expected_ids
		SET observed = FALSE, observed_at = NULL
		WHERE migration_id = ?
	`, migrationID); err != nil {
		return nil, err
	}
	if len(actual) > 0 {
		for _, id := range actual {
			if !expectedSet[id] {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE session_migration_vector_expected_ids
				SET observed = TRUE, observed_at = CURRENT_TIMESTAMP(3)
				WHERE migration_id = ? AND document_id = ?
			`, migrationID, id); err != nil {
				return nil, err
			}
		}
	}
	tableActualIDs := map[string][]string{}
	rows, err := tx.QueryContext(ctx, `
		SELECT source_table, document_id
		FROM session_migration_vector_expected_ids
		WHERE migration_id = ?
		ORDER BY source_table, document_id
	`, migrationID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var table, id string
		if err := rows.Scan(&table, &id); err != nil {
			rows.Close()
			return nil, err
		}
		if actualSet[id] {
			tableActualIDs[table] = append(tableActualIDs[table], id)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for table, ids := range tableActualIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE session_migration_artifact_parity
			SET vector_actual_count = ?,
			    vector_actual_id_hash = ?,
			    updated_at = CURRENT_TIMESTAMP(3)
			WHERE migration_id = ? AND manifest_version = ? AND table_name = ?
		`, len(ids), sessionMigrationHashIDs(ids), migrationID, SessionMigrationManifestVersion, table); err != nil {
			return nil, err
		}
	}
	status := "vector_reindex_unverified"
	state := "failed"
	lastError := strings.Join(append(append([]string{}, missing...), unexpected...), ",")
	if result.Verified {
		status = "vector_reindexed"
		state = "completed"
		lastError = ""
	}
	errorsJSON, _ := json.Marshal(map[string]any{
		"missing_ids":    missing,
		"unexpected_ids": unexpected,
	})
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = CASE
		        WHEN status IN ('copied', 'vector_reindex_failed', 'vector_reindex_unverified')
		        THEN ?
		        ELSE status
		    END,
		    chroma_reindexed_count = ?,
		    errors_json = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, status, len(actual), string(errorsJSON), migrationID); err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "vector_exact_id_parity", state,
		result.ExpectedIDHash, string(errorsJSON), lastError); err != nil {
		return nil, err
	}
	if err := sessionMigrationPersistCurrentStateProofTx(
		ctx, tx, migrationID, operation, relationalHash, result.ActualIDHash,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return result, nil
}

func sessionMigrationVectorParityContextTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
) (*SessionMigrationVectorParityContext, error) {
	var targetSessionID string
	if err := tx.QueryRowContext(ctx, `
		SELECT target_session_id
		FROM session_migrations
		WHERE id = ? AND status NOT IN ('rolled_back', 'rollback_partial')
	`, migrationID).Scan(&targetSessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT document_id
		FROM session_migration_vector_expected_ids
		WHERE migration_id = ?
		ORDER BY document_id
	`, migrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expected := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		expected = append(expected, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &SessionMigrationVectorParityContext{
		MigrationID:     migrationID,
		TargetSessionID: targetSessionID,
		ExpectedIDs:     expected,
	}, nil
}

func sessionMigrationUniqueSortedIDs(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sessionMigrationCompareVectorIDs(
	migrationID int64,
	targetSessionID string,
	expectedIDs, actualIDs []string,
) *SessionMigrationVectorParityResult {
	return sessionMigrationCompareVectorIDsWithPolicy(
		migrationID, targetSessionID, expectedIDs, actualIDs, false,
	)
}

func sessionMigrationCompareVectorIDsWithPolicy(
	migrationID int64,
	targetSessionID string,
	expectedIDs, actualIDs []string,
	allowUnexpected bool,
) *SessionMigrationVectorParityResult {
	expected := sessionMigrationUniqueSortedIDs(expectedIDs)
	actual := sessionMigrationUniqueSortedIDs(actualIDs)
	expectedSet := make(map[string]bool, len(expected))
	actualSet := make(map[string]bool, len(actual))
	for _, id := range expected {
		expectedSet[id] = true
	}
	for _, id := range actual {
		actualSet[id] = true
	}
	missing := []string{}
	unexpected := []string{}
	for _, id := range expected {
		if !actualSet[id] {
			missing = append(missing, id)
		}
	}
	for _, id := range actual {
		if !expectedSet[id] {
			unexpected = append(unexpected, id)
		}
	}
	return &SessionMigrationVectorParityResult{
		MigrationID:     migrationID,
		TargetSessionID: targetSessionID,
		ExpectedIDs:     expected,
		ActualIDs:       actual,
		MissingIDs:      missing,
		UnexpectedIDs:   unexpected,
		ExpectedIDHash:  sessionMigrationHashIDs(expected),
		ActualIDHash:    sessionMigrationHashIDs(actual),
		Verified:        len(missing) == 0 && (allowUnexpected || len(unexpected) == 0),
	}
}

func (m *mariadbStore) PrepareSessionMigrationSourceLock(
	ctx context.Context,
	migrationID int64,
	reason string,
) (*SessionMigrationLock, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var sourceID, targetID, mode, status string
	if err := tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, mode, status
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &mode, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if mode != SessionMigrationModeCopyThenLockSource {
		return nil, sessionMigrationBlocker("source_lock_mode_does_not_lock", "source_lock_prepare", "")
	}
	if status != "vector_reindexed" && status != "source_locked" {
		return nil, sessionMigrationBlocker("source_lock_phase_mismatch", "source_lock_prepare", "")
	}
	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if lock != nil && lock.MigrationID != migrationID {
		return nil, sessionMigrationBlocker("source_session_locked_by_other_migration", "source_lock_prepare", "")
	}
	if lock == nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO session_migration_locks (
				migration_id, source_session_id, target_session_id, locked, lock_status, reason
			) VALUES (?, ?, ?, TRUE, 'lock_pending_verification', ?)
		`, migrationID, sourceID, targetID, strings.TrimSpace(reason)); err != nil {
			return nil, err
		}
		lock, err = sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
		if err != nil {
			return nil, err
		}
	}
	if lock.LockStatus != "migrated_away" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE session_migration_locks
			SET lock_status = 'lock_pending_verification',
			    reason = ?,
			    updated_at = CURRENT_TIMESTAMP(3)
			WHERE migration_id = ? AND source_session_id = ?
			  AND locked = TRUE AND unlocked_at IS NULL
		`, strings.TrimSpace(reason), migrationID, sourceID); err != nil {
			return nil, err
		}
		lock.LockStatus = "lock_pending_verification"
		lock.Reason = strings.TrimSpace(reason)
	}
	if err := sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, "source_lock_fence", "prepared",
		sessionMigrationStringHash(sourceID, targetID, reason),
		fmt.Sprintf(`{"source_session_id":%q,"lock_status":%q}`, sourceID, lock.LockStatus),
		"",
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return lock, nil
}

func sessionMigrationActiveDerivationLeasePhaseTx(ctx context.Context, tx *sql.Tx, sourceSessionID string) (string, error) {
	checks := []struct {
		phase string
		query string
	}{
		{
			phase: "memory_reprocessing_drain",
			query: `
				SELECT id
				FROM memory_reprocessing_jobs
				WHERE chat_session_id = ?
				  AND status = 'leased'
				  AND lease_until >= CURRENT_TIMESTAMP(3)
				ORDER BY id
				LIMIT 1
				FOR UPDATE
			`,
		},
		{
			phase: "memory_vector_drain",
			query: `
				SELECT id
				FROM memory_vector_outbox
				WHERE chat_session_id = ?
				  AND status = 'leased'
				  AND lease_until >= CURRENT_TIMESTAMP(3)
				ORDER BY id
				LIMIT 1
				FOR UPDATE
			`,
		},
	}
	for _, check := range checks {
		var id int64
		err := tx.QueryRowContext(ctx, check.query, sourceSessionID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return "", err
		}
		return check.phase, nil
	}
	return "", nil
}

func (m *mariadbStore) ReleaseSessionMigrationSourceLockFence(
	ctx context.Context,
	migrationID int64,
	reason string,
) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if migrationID <= 0 {
		return ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	result, err := tx.ExecContext(ctx, `
		UPDATE session_migration_locks
		SET locked = FALSE,
		    lock_status = 'lock_verification_failed',
		    reason = ?,
		    unlocked_at = CURRENT_TIMESTAMP(3),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ?
		  AND locked = TRUE
		  AND unlocked_at IS NULL
		  AND lock_status = 'lock_pending_verification'
	`, strings.TrimSpace(reason), migrationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 1 {
		return sessionMigrationBlocker("source_lock_fence_not_unique", "source_lock_release", "")
	}
	if err := sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, "source_lock_fence", "failed",
		sessionMigrationStringHash(strconv.FormatInt(migrationID, 10), reason),
		fmt.Sprintf(`{"released":%t}`, affected == 1),
		strings.TrimSpace(reason),
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *mariadbStore) LockSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationSourceLockResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	if blockers := SessionMigrationManifestReleaseBlockers(); len(blockers) > 0 {
		return nil, fmt.Errorf("session migration source lock blocked: %s", SessionMigrationManifestParityUnverifiedReason)
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var sourceID, targetID, mode, status string
	var reindexedCount int
	err = tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, mode, status, chroma_reindexed_count
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &mode, &status, &reindexedCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mode) != SessionMigrationModeCopyThenLockSource {
		return nil, fmt.Errorf("session migration source lock blocked: migration mode %q does not lock source", mode)
	}
	if status != "vector_reindexed" && status != "source_locked" {
		return nil, fmt.Errorf("session migration source lock blocked: migration status %q is not vector_reindexed", status)
	}
	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if lock != nil && lock.MigrationID != migrationID {
		return nil, fmt.Errorf("session migration source lock blocked: source session is already locked by migration %d", lock.MigrationID)
	}
	if lock == nil {
		return nil, sessionMigrationBlocker("source_lock_fence_required", "source_lock", "")
	}
	if lock.LockStatus != "lock_pending_verification" && lock.LockStatus != "migrated_away" {
		return nil, sessionMigrationBlocker("source_lock_fence_phase_mismatch", "source_lock", "")
	}
	leasePhase, err := sessionMigrationActiveDerivationLeasePhaseTx(ctx, tx, sourceID)
	if err != nil {
		return nil, err
	}
	if leasePhase != "" {
		return nil, sessionMigrationBlocker("source_derivation_lease_active", leasePhase, "")
	}
	if err := sessionMigrationVerifyDurableParityTx(ctx, tx, migrationID); err != nil {
		return nil, err
	}
	currentRelationalHash, err := sessionMigrationRevalidateCurrentRelationalStateTx(ctx, tx, migrationID, "source_lock", true)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationConsumeCurrentStateProofTx(
		ctx, tx, migrationID, SessionMigrationProofOperationSourceLock, currentRelationalHash,
	); err != nil {
		return nil, err
	}
	_ = reindexedCount

	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migration_locks
		SET lock_status = 'migrated_away',
		    reason = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ? AND source_session_id = ?
		  AND locked = TRUE AND unlocked_at IS NULL
	`, strings.TrimSpace(reason), migrationID, sourceID); err != nil {
		return nil, err
	}
	lock.LockStatus = "migrated_away"
	lock.Reason = strings.TrimSpace(reason)

	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'source_locked',
		    locked_at = COALESCE(locked_at, CURRENT_TIMESTAMP(3)),
		    errors_json = JSON_ARRAY(),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, migrationID)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, "source_lock_fence", "completed",
		sessionMigrationStringHash(sourceID, targetID, reason),
		`{"lock_status":"migrated_away"}`, "",
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationSourceLockResult{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          "source_locked",
		Lock:            *lock,
		ReadyForLive:    true,
	}, nil
}

func sessionMigrationVerifyDurableParityTx(ctx context.Context, tx *sql.Tx, migrationID int64) error {
	var total, relationalVerified, vectorVerified int
	err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(
				CASE
					WHEN parity_state IN (
						'verified_copy', 'verified_retain_audit',
						'verified_regenerate', 'verified_delete_pending'
					)
					 AND source_row_count IS NOT NULL
					 AND source_content_hash IS NOT NULL
					 AND target_row_count IS NOT NULL
					 AND target_content_hash IS NOT NULL
					 AND COALESCE(row_map_expected_count, 0) = COALESCE(row_map_verified_count, 0)
					 AND COALESCE(fk_expected_count, 0) = COALESCE(fk_verified_count, 0)
					THEN 1 ELSE 0
				END
			), 0),
			COALESCE(SUM(
				CASE
					WHEN COALESCE(vector_expected_count, 0) = COALESCE(vector_actual_count, 0)
					 AND (
						COALESCE(vector_expected_count, 0) = 0
						OR vector_expected_id_hash = vector_actual_id_hash
					 )
					THEN 1 ELSE 0
				END
			), 0)
		FROM session_migration_artifact_parity
		WHERE migration_id = ? AND manifest_version = ?
	`, migrationID, SessionMigrationManifestVersion).Scan(&total, &relationalVerified, &vectorVerified)
	if err != nil {
		return err
	}
	direct, indirect, implemented := SessionMigrationManifestSummary()
	expectedEntries := direct + indirect
	if implemented != expectedEntries || total != expectedEntries {
		return fmt.Errorf("session migration source lock blocked: manifest parity rows %d/%d", total, expectedEntries)
	}
	if relationalVerified != expectedEntries {
		return fmt.Errorf("session migration source lock blocked: relational count/hash/row-map/FK parity %d/%d", relationalVerified, expectedEntries)
	}
	if vectorVerified != expectedEntries {
		return fmt.Errorf("session migration source lock blocked: vector artifact parity %d/%d", vectorVerified, expectedEntries)
	}
	var expectedVectors, observedVectors int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN observed = TRUE THEN 1 ELSE 0 END), 0)
		FROM session_migration_vector_expected_ids
		WHERE migration_id = ?
	`, migrationID).Scan(&expectedVectors, &observedVectors); err != nil {
		return err
	}
	if expectedVectors != observedVectors {
		return fmt.Errorf("session migration source lock blocked: exact vector expected-ID parity %d/%d", observedVectors, expectedVectors)
	}
	var completedSaga int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM session_migration_saga_steps
		WHERE migration_id = ? AND phase = 'vector_exact_id_parity' AND phase_state = 'completed'
	`, migrationID).Scan(&completedSaga); err != nil {
		return err
	}
	if completedSaga != 1 {
		return errors.New("session migration source lock blocked: vector exact-ID saga is not completed")
	}
	return nil
}

type sessionMigrationStoredArtifactParity struct {
	SourceCount int
	SourceHash  string
	TargetCount int
	TargetHash  string
}

func sessionMigrationOwnedTargetRows(
	entry SessionMigrationManifestEntry,
	plan SessionMigrationExecutionPlan,
	rows []sessionMigrationRow,
	keyMaps *sessionMigrationKeyMaps,
) []sessionMigrationRow {
	if entry.Policy != SessionMigrationPolicyCopy &&
		entry.Policy != SessionMigrationPolicyRetainAudit {
		return nil
	}
	if len(plan.PrimaryKey) != 1 {
		return nil
	}
	primaryKey := plan.PrimaryKey[0]
	owned := make([]sessionMigrationRow, 0, len(rows))
	for _, row := range rows {
		key := row.Values[primaryKey]
		if !key.Valid {
			continue
		}
		if _, ok := keyMaps.source(entry.Table, primaryKey, key.Text); ok {
			owned = append(owned, row)
		}
	}
	return owned
}

func sessionMigrationLoadCurrentKeyMapsTx(ctx context.Context, tx *sql.Tx, migrationID int64) (*sessionMigrationKeyMaps, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT table_name, key_column_name, source_key, target_key
		FROM session_migration_artifact_row_map
		WHERE migration_id = ? AND row_status <> 'rolled_back'
		ORDER BY table_name, key_column_name, source_key
	`, migrationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	maps := newSessionMigrationKeyMaps()
	for rows.Next() {
		var table, column, source, target string
		if err := rows.Scan(&table, &column, &source, &target); err != nil {
			return nil, err
		}
		if err := maps.put(table, column, source, target); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return maps, nil
}

func sessionMigrationRevalidateCurrentRelationalStateTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	phase string,
	lockRanges bool,
) (string, error) {
	var sourceID, targetID, status string
	if err := tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status
		FROM session_migrations
		WHERE id = ?
	`, migrationID).Scan(&sourceID, &targetID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT table_name, source_row_count, source_content_hash,
		       target_row_count, target_content_hash
		FROM session_migration_artifact_parity
		WHERE migration_id = ? AND manifest_version = ?
		ORDER BY table_name
	`, migrationID, SessionMigrationManifestVersion)
	if err != nil {
		return "", err
	}
	stored := map[string]sessionMigrationStoredArtifactParity{}
	for rows.Next() {
		var table string
		var parity sessionMigrationStoredArtifactParity
		if err := rows.Scan(&table, &parity.SourceCount, &parity.SourceHash, &parity.TargetCount, &parity.TargetHash); err != nil {
			_ = rows.Close()
			return "", err
		}
		stored[table] = parity
	}
	if err := rows.Close(); err != nil {
		return "", err
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	manifest := SessionMigrationManifest()
	if len(stored) != len(manifest) {
		return "", sessionMigrationBlocker("current_manifest_parity_incomplete", phase, "")
	}
	keyMaps, err := sessionMigrationLoadCurrentKeyMapsTx(ctx, tx, migrationID)
	if err != nil {
		return "", err
	}
	fingerprintParts := []string{SessionMigrationManifestVersion, sourceID, targetID}
	sourceWasCleaned := status == "source_cleaned"
	allowLiveTargetAdditions := status == "source_locked" ||
		status == "cleanup_prepared" ||
		status == "source_cleaned"
	for _, entry := range manifest {
		plan, ok := SessionMigrationExecutionPlanFor(entry.Table)
		if !ok {
			return "", sessionMigrationBlocker("current_manifest_plan_missing", phase, entry.Table)
		}
		sourceRows, err := sessionMigrationReadManifestRowsMode(ctx, tx, entry, plan, sourceID, lockRanges)
		if err != nil {
			return "", err
		}
		targetRows, err := sessionMigrationReadManifestRowsMode(ctx, tx, entry, plan, targetID, lockRanges)
		if err != nil {
			return "", err
		}
		if allowLiveTargetAdditions {
			targetRows = sessionMigrationOwnedTargetRows(entry, plan, targetRows, keyMaps)
		}
		sourceHash := sessionMigrationCanonicalRowsHash(entry, plan, sourceRows, sourceID, false, keyMaps)
		targetHash := sessionMigrationCanonicalRowsHash(entry, plan, targetRows, targetID, true, keyMaps)
		want := stored[entry.Table]
		if sourceWasCleaned {
			shouldRemain := entry.Policy == SessionMigrationPolicyRetainAudit
			if shouldRemain {
				if len(sourceRows) != want.SourceCount || sourceHash != want.SourceHash {
					return "", sessionMigrationBlocker("current_source_snapshot_drift", phase, entry.Table)
				}
			} else if len(sourceRows) != 0 {
				return "", sessionMigrationBlocker("current_source_cleanup_incomplete", phase, entry.Table)
			}
		} else if len(sourceRows) != want.SourceCount || sourceHash != want.SourceHash {
			return "", sessionMigrationBlocker("current_source_snapshot_drift", phase, entry.Table)
		}
		if len(targetRows) != want.TargetCount || targetHash != want.TargetHash {
			return "", sessionMigrationBlocker("current_target_snapshot_drift", phase, entry.Table)
		}
		if !sourceWasCleaned {
			if _, err := sessionMigrationEvaluateArtifactParity(
				entry, plan, sourceRows, targetRows, sourceHash, targetHash, keyMaps,
			); err != nil {
				return "", sessionMigrationBlocker("current_relational_reference_drift", phase, entry.Table)
			}
		}
		fingerprintParts = append(
			fingerprintParts,
			entry.Table,
			strconv.Itoa(len(sourceRows)), sourceHash,
			strconv.Itoa(len(targetRows)), targetHash,
		)
	}
	return sessionMigrationStringHash(fingerprintParts...), nil
}

func sessionMigrationPersistCurrentStateProofTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	operation string,
	relationalHash, vectorHash string,
) error {
	phase, ok := sessionMigrationCurrentProofPhase(operation)
	if !ok {
		return sessionMigrationBlocker("current_vector_proof_operation_invalid", operation, "")
	}
	return sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, phase, "completed",
		sessionMigrationStringHash(relationalHash, vectorHash),
		fmt.Sprintf(`{"relational_hash":%q,"vector_id_hash":%q}`, relationalHash, vectorHash),
		"",
	)
}

func sessionMigrationCurrentProofPhase(operation string) (string, bool) {
	switch operation {
	case SessionMigrationProofOperationSourceLock,
		SessionMigrationProofOperationCleanupPrepare,
		SessionMigrationProofOperationCleanupFinalize,
		SessionMigrationProofOperationResume:
		return "current_state_revalidation_" + operation, true
	default:
		return "", false
	}
}

func sessionMigrationValidateProofOperationTx(ctx context.Context, tx *sql.Tx, migrationID int64, operation string) (string, error) {
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM session_migrations WHERE id = ?`, migrationID).Scan(&status); err != nil {
		return "", err
	}
	allowed := false
	switch operation {
	case SessionMigrationProofOperationSourceLock:
		allowed = status == "copied" || status == "vector_reindexed" || status == "source_locked"
	case SessionMigrationProofOperationCleanupPrepare:
		allowed = status == "source_locked"
	case SessionMigrationProofOperationCleanupFinalize:
		allowed = status == "cleanup_prepared"
	case SessionMigrationProofOperationResume:
		allowed = status == "vector_reindexed" || status == "source_locked" ||
			status == "cleanup_prepared" || status == "source_cleaned"
	}
	if !allowed {
		return status, sessionMigrationBlocker("current_vector_proof_phase_mismatch", operation, "")
	}
	return status, nil
}

func sessionMigrationConsumeCurrentStateProofTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	operation, relationalHash string,
) error {
	phase, ok := sessionMigrationCurrentProofPhase(operation)
	if !ok {
		return sessionMigrationBlocker("current_vector_proof_operation_invalid", operation, "")
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE session_migration_saga_steps
		SET phase_state = 'consumed',
		    result_json = JSON_SET(
		        COALESCE(result_json, JSON_OBJECT()),
		        '$.consumed', TRUE
		    ),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ?
		  AND phase = ?
		  AND phase_state = 'completed'
		  AND JSON_UNQUOTE(JSON_EXTRACT(result_json, '$.relational_hash')) = ?
	`, migrationID, phase, relationalHash)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return sessionMigrationBlocker("current_vector_snapshot_required", operation, "")
	}
	return nil
}

func (m *mariadbStore) GetSessionMigrationSourceLock(ctx context.Context, sourceSessionID string) (*SessionMigrationLock, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	if sourceSessionID == "" {
		return nil, ErrNotFound
	}
	return sessionMigrationSelectActiveLock(ctx, m.db, sourceSessionID)
}

func (m *mariadbStore) RollbackSessionMigration(ctx context.Context, migrationID int64, reason string) (*SessionMigrationRollbackResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var sourceID, targetID, status string
	err = tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == "rolled_back" {
		return &SessionMigrationRollbackResult{
			MigrationID:     migrationID,
			SourceSessionID: sourceID,
			TargetSessionID: targetID,
			Status:          "rolled_back",
			ReadyForLive:    false,
		}, nil
	}
	if status == "source_cleaned" || status == "cleanup_completed" {
		return nil, fmt.Errorf("session migration rollback blocked: migration status %q has already cleaned the source", status)
	}

	deletedByTable, rowMapCount, err := rollbackSessionMigrationManifestRowsTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	counts := SessionMigrationArtifactCounts{
		ChatLogs:                 deletedByTable["chat_logs"],
		EffectiveInputs:          deletedByTable["effective_input_logs"],
		Memories:                 deletedByTable["memories"],
		DirectEvidence:           deletedByTable["direct_evidence_records"],
		KGTriples:                deletedByTable["kg_triples"],
		Episodes:                 deletedByTable["episode_summaries"],
		SubjectiveEntityMemories: deletedByTable["protagonist_entity_memories"],
		ReferenceBindings:        deletedByTable["session_reference_bindings"],
	}
	sessionMigrationFinalizeCounts(&counts)

	sourceUnlocked := false
	if status == "source_locked" {
		res, err := tx.ExecContext(ctx, `
			UPDATE session_migration_locks
			SET locked = FALSE,
			    lock_status = 'rolled_back',
			    reason = CONCAT(COALESCE(reason, ''), CASE WHEN COALESCE(reason, '') = '' THEN '' ELSE '\n' END, ?),
			    unlocked_at = CURRENT_TIMESTAMP(3),
			    updated_at = CURRENT_TIMESTAMP(3)
			WHERE migration_id = ? AND locked = TRUE AND unlocked_at IS NULL
		`, strings.TrimSpace(reason), migrationID)
		if err != nil {
			return nil, err
		}
		affected, _ := res.RowsAffected()
		sourceUnlocked = affected > 0
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE session_migration_row_map
		SET row_status = 'rolled_back'
		WHERE migration_id = ? AND row_status <> 'rolled_back'
	`, migrationID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migration_artifact_row_map
		SET row_status = 'rolled_back',
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ? AND row_status <> 'rolled_back'
	`, migrationID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migration_reference_binding_map
		SET row_status = 'rolled_back'
		WHERE migration_id = ? AND row_status <> 'rolled_back'
	`, migrationID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migration_vector_expected_ids
		SET observed = FALSE, observed_at = NULL
		WHERE migration_id = ?
	`, migrationID); err != nil {
		return nil, err
	}
	errorsJSON, _ := json.Marshal([]string{"rolled_back", strings.TrimSpace(reason)})
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'rolled_back',
		    errors_json = ?,
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, string(errorsJSON), migrationID)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "rollback", "completed",
		sessionMigrationStringHash(strconv.FormatInt(migrationID, 10), reason),
		fmt.Sprintf(`{"deleted_rows":%d}`, rowMapCount), ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationRollbackResult{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          "rolled_back",
		Counts:          counts,
		RowMapCount:     rowMapCount,
		SourceUnlocked:  sourceUnlocked,
		ReadyForLive:    false,
	}, nil
}

func (m *mariadbStore) PreviewSessionMigrationSourceCleanup(ctx context.Context, migrationID int64) (*SessionMigrationCleanupPreview, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationVerifyDurableParityTx(ctx, tx, migrationID); err != nil {
		preview.BlockedReasons = append(preview.BlockedReasons, err.Error())
		preview.ReadyForCleanup = false
	}
	if _, err := sessionMigrationRevalidateCurrentRelationalStateTx(ctx, tx, migrationID, "cleanup_preview", false); err != nil {
		preview.BlockedReasons = append(preview.BlockedReasons, err.Error())
		preview.ReadyForCleanup = false
	}
	return preview, nil
}

// PrepareSessionMigrationSourceCleanup durably fences the relational source
// snapshot before the HTTP owner mutates Chroma. A failed vector deletion or
// failed relational finalize can be retried from cleanup_prepared without
// weakening the source count/hash gate.
func (m *mariadbStore) PrepareSessionMigrationSourceCleanup(ctx context.Context, migrationID int64, reason string) (*SessionMigrationCleanupPreview, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationVerifyDurableParityTx(ctx, tx, migrationID); err != nil {
		preview.BlockedReasons = append(preview.BlockedReasons, err.Error())
		preview.ReadyForCleanup = false
	}
	currentRelationalHash, err := sessionMigrationRevalidateCurrentRelationalStateTx(ctx, tx, migrationID, "cleanup_prepare", true)
	if err != nil {
		preview.BlockedReasons = append(preview.BlockedReasons, err.Error())
		preview.ReadyForCleanup = false
	} else if err := sessionMigrationConsumeCurrentStateProofTx(
		ctx, tx, migrationID, SessionMigrationProofOperationCleanupPrepare, currentRelationalHash,
	); err != nil {
		preview.BlockedReasons = append(preview.BlockedReasons, err.Error())
		preview.ReadyForCleanup = false
	}
	if !preview.ReadyForCleanup || len(preview.BlockedReasons) > 0 {
		return preview, nil
	}
	requestHash := sessionMigrationStringHash(
		SessionMigrationManifestVersion,
		strconv.FormatInt(migrationID, 10),
		preview.SourceSessionID,
		preview.TargetSessionID,
		strings.TrimSpace(reason),
	)
	if err := sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, "source_cleanup", "prepared", requestHash,
		`{"relational_source_snapshot_verified":true,"vector_delete_pending":true}`, "",
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'cleanup_prepared',
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND status IN ('source_locked', 'cleanup_prepared')
	`, migrationID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	preview.Status = "cleanup_prepared"
	return preview, nil
}

func (m *mariadbStore) MarkSessionMigrationSourceVectorCleanup(ctx context.Context, migrationID int64) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if migrationID <= 0 {
		return ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status FROM session_migrations WHERE id = ? FOR UPDATE
	`, migrationID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "cleanup_prepared" {
		return fmt.Errorf("session migration vector cleanup mark blocked: status %s", status)
	}
	if err := sessionMigrationUpsertSagaStepTx(
		ctx, tx, migrationID, "source_vector_cleanup", "completed",
		sessionMigrationStringHash(SessionMigrationManifestVersion, strconv.FormatInt(migrationID, 10)),
		`{"source_vector_delete_completed":true}`, "",
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *mariadbStore) CleanupSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationCleanupResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	if len(preview.BlockedReasons) > 0 || !preview.ReadyForCleanup {
		return nil, fmt.Errorf("session migration source cleanup blocked: %s", strings.Join(preview.BlockedReasons, ","))
	}
	if err := sessionMigrationVerifyDurableParityTx(ctx, tx, migrationID); err != nil {
		return nil, err
	}
	currentRelationalHash, err := sessionMigrationRevalidateCurrentRelationalStateTx(ctx, tx, migrationID, "cleanup_finalize", true)
	if err != nil {
		return nil, err
	}
	if err := sessionMigrationConsumeCurrentStateProofTx(
		ctx, tx, migrationID, SessionMigrationProofOperationCleanupFinalize, currentRelationalHash,
	); err != nil {
		return nil, err
	}
	var vectorCleanupCompleted int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM session_migration_saga_steps
		WHERE migration_id = ? AND phase = 'source_vector_cleanup' AND phase_state = 'completed'
	`, migrationID).Scan(&vectorCleanupCompleted); err != nil {
		return nil, err
	}
	if vectorCleanupCompleted != 1 {
		return nil, errors.New("session migration source cleanup blocked: source vector cleanup is not durably completed")
	}
	deletedByTable, err := sessionMigrationDeleteSourceManifestRowsTx(ctx, tx, preview.SourceSessionID)
	if err != nil {
		return nil, err
	}
	counts := SessionMigrationArtifactCounts{
		ChatLogs:                 deletedByTable["chat_logs"],
		EffectiveInputs:          deletedByTable["effective_input_logs"],
		Memories:                 deletedByTable["memories"],
		DirectEvidence:           deletedByTable["direct_evidence_records"],
		KGTriples:                deletedByTable["kg_triples"],
		Episodes:                 deletedByTable["episode_summaries"],
		SubjectiveEntityMemories: deletedByTable["protagonist_entity_memories"],
		ReferenceBindings:        deletedByTable["session_reference_bindings"],
	}
	sessionMigrationFinalizeCounts(&counts)
	if _, err := tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'source_cleaned',
		    cleanup_at = CURRENT_TIMESTAMP(3),
		    errors_json = JSON_ARRAY(),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, migrationID); err != nil {
		return nil, err
	}
	if err := sessionMigrationUpsertSagaStepTx(ctx, tx, migrationID, "source_cleanup", "completed",
		sessionMigrationStringHash(strconv.FormatInt(migrationID, 10), reason),
		fmt.Sprintf(`{"deleted_rows":%d}`, sessionMigrationDeletedTableTotal(deletedByTable)), ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return &SessionMigrationCleanupResult{
		MigrationID:     migrationID,
		SourceSessionID: preview.SourceSessionID,
		TargetSessionID: preview.TargetSessionID,
		Status:          "source_cleaned",
		Counts:          counts,
		SourceCleaned:   true,
		ReadyForLive:    true,
	}, nil
}

type sessionMigrationLockQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func sessionMigrationSelectActiveLock(ctx context.Context, q sessionMigrationLockQuerier, sourceSessionID string) (*SessionMigrationLock, error) {
	return sessionMigrationScanActiveLock(q.QueryRowContext(ctx, `
		SELECT migration_id, source_session_id, target_session_id, locked, lock_status,
		       COALESCE(reason, ''), locked_at
		FROM session_migration_locks
		WHERE source_session_id = ? AND locked = TRUE AND unlocked_at IS NULL
		ORDER BY locked_at DESC, id DESC
		LIMIT 1
	`, sourceSessionID))
}

func sessionMigrationSelectActiveLockTx(ctx context.Context, tx *sql.Tx, sourceSessionID string) (*SessionMigrationLock, error) {
	return sessionMigrationScanActiveLock(tx.QueryRowContext(ctx, `
		SELECT migration_id, source_session_id, target_session_id, locked, lock_status,
		       COALESCE(reason, ''), locked_at
		FROM session_migration_locks
		WHERE source_session_id = ? AND locked = TRUE AND unlocked_at IS NULL
		ORDER BY locked_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, sourceSessionID))
}

func sessionMigrationScanActiveLock(row *sql.Row) (*SessionMigrationLock, error) {
	var lock SessionMigrationLock
	if err := row.Scan(&lock.MigrationID, &lock.SourceSessionID, &lock.TargetSessionID, &lock.Locked, &lock.LockStatus, &lock.Reason, &lock.LockedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &lock, nil
}

func sessionMigrationMemoryDocumentText(summaryJSON, evidence, placeWing, placeRoom string) string {
	extraction := sessionMigrationJSONMap(summaryJSON)
	summary := sessionMigrationMemorySummary(extraction)
	evidenceJSON := sessionMigrationJSONMap(evidence)
	evidenceExcerpts := sessionMigrationStringValues(evidenceJSON["evidence_excerpts"])
	aliases := sessionMigrationMemoryAliases(extraction)
	aliases = sessionMigrationAppendUniqueText(aliases, placeWing)
	aliases = sessionMigrationAppendUniqueText(aliases, placeRoom)

	parts := []string{}
	seen := map[string]bool{}
	sessionMigrationAppendSearchTextPart(&parts, seen, "Canonical Summary", summary)
	if len(evidenceExcerpts) > 0 {
		sessionMigrationAppendSearchTextPart(&parts, seen, "Raw Evidence", strings.Join(evidenceExcerpts, "\n"))
	}
	if len(aliases) > 0 {
		sessionMigrationAppendSearchTextPart(&parts, seen, "Aliases", strings.Join(aliases, "\n"))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func sessionMigrationJSONMap(raw string) map[string]any {
	out := map[string]any{}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &out) != nil {
		return map[string]any{}
	}
	return out
}

func sessionMigrationMemorySummary(extraction map[string]any) string {
	for _, key := range []string{"turn_summary", "summary", "scene_summary", "core_meaning", "emotional_shift", "content", "text"} {
		value, ok := extraction[key].(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" && !sessionMigrationLooksStructuredCriticText(value) {
			return value
		}
	}
	return ""
}

func sessionMigrationLooksStructuredCriticText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "map[") && !strings.HasPrefix(trimmed, "[")) {
		return false
	}
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{
		"archive_hint", "character_deltas", "entity_conditions", "evidence_excerpts",
		"kg_triples", "pending_threads", "physical_conditions", "relationship_memory",
		"narrative_events", "state_claims", "belief_updates", "state_deltas",
		"subjective_entity_memories", "turn_summary",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sessionMigrationMemoryAliases(extraction map[string]any) []string {
	aliases := []string{}
	for _, key := range []string{
		"characters", "character_names", "people", "places", "locations", "items", "factions", "keywords", "tags",
	} {
		for _, value := range sessionMigrationStringValues(extraction[key]) {
			aliases = sessionMigrationAppendUniqueText(aliases, value)
		}
	}
	for _, key := range []string{"wing", "room", "section", "shelf"} {
		aliases = sessionMigrationAppendUniqueText(aliases, sessionMigrationMapString(extraction["archive_hint"], key))
	}
	for _, spec := range []struct {
		field string
		keys  []string
	}{
		{"entities", []string{"name", "canonical_name", "display_name", "role", "entity_type", "type", "location"}},
		{"character_states", []string{"name", "role", "location", "status_emotion"}},
		{"kg_triples", []string{"subject", "predicate", "object"}},
		{"world_rules", []string{"category", "key", "scope", "scope_name"}},
		{"storylines", []string{"name", "title"}},
		{"pending_threads", []string{"name", "title", "thread", "goal"}},
		{"protected_secrets", []string{"secret_kind", "owner", "sensitivity", "evidence_strength", "disclosure_policy"}},
		{"character_identity_accuracy", []string{
			"canonical_entity_name", "surface_identity_name", "true_identity_name", "public_identity_name",
			"alias_name", "real_identity_name", "identity_kind", "public_role", "true_role",
			"public_allegiance", "true_allegiance", "reveal_policy",
		}},
	} {
		for _, item := range sessionMigrationMapItems(extraction[spec.field]) {
			for _, key := range spec.keys {
				aliases = sessionMigrationAppendUniqueText(aliases, item[key])
			}
			for _, value := range sessionMigrationStringValues(item["aliases"]) {
				aliases = sessionMigrationAppendUniqueText(aliases, value)
			}
			if spec.field == "protected_secrets" {
				for _, value := range sessionMigrationStringValues(item["subject"]) {
					aliases = sessionMigrationAppendUniqueText(aliases, value)
				}
			}
			if spec.field == "protected_secrets" || spec.field == "character_identity_accuracy" {
				for _, key := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
					for _, value := range sessionMigrationStringValues(sessionMigrationMapValue(item["knowledge_scope"], key)) {
						aliases = sessionMigrationAppendUniqueText(aliases, value)
					}
				}
			}
		}
	}
	return aliases
}

func sessionMigrationMapItems(value any) []map[string]any {
	switch items := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if mapped, ok := item.(map[string]any); ok && len(mapped) > 0 {
				out = append(out, mapped)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{items}
	default:
		return nil
	}
}

func sessionMigrationStringValues(value any) []string {
	switch items := value.(type) {
	case []any:
		out := []string{}
		for _, item := range items {
			if text, ok := item.(string); ok {
				if text = strings.TrimSpace(text); text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	case []string:
		out := []string{}
		for _, item := range items {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	case string:
		if items = strings.TrimSpace(items); items != "" {
			return []string{items}
		}
	}
	return nil
}

func sessionMigrationMapValue(value any, key string) any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped[key]
	}
	return nil
}

func sessionMigrationMapString(value any, key string) string {
	text, _ := sessionMigrationMapValue(value, key).(string)
	return strings.TrimSpace(text)
}

func sessionMigrationAppendUniqueText(items []string, value any) []string {
	text, ok := value.(string)
	if !ok {
		return items
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return items
	}
	key := strings.ToLower(strings.Join(strings.Fields(text), " "))
	for _, existing := range items {
		if strings.ToLower(strings.Join(strings.Fields(existing), " ")) == key {
			return items
		}
	}
	return append(items, text)
}

func sessionMigrationAppendSearchTextPart(parts *[]string, seen map[string]bool, label, text string) {
	text = strings.TrimSpace(text)
	key := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*parts = append(*parts, fmt.Sprintf("[%s]\n%s", label, text))
}

func sessionMigrationFinalizeCounts(counts *SessionMigrationArtifactCounts) {
	counts.CanonicalTotal = counts.ChatLogs + counts.EffectiveInputs + counts.Memories + counts.DirectEvidence + counts.KGTriples + counts.Episodes
	counts.CanonicalAndSubjectiveTotal = counts.CanonicalTotal + counts.SubjectiveEntityMemories
}

func sessionMigrationCountArtifactsTx(ctx context.Context, tx *sql.Tx, sessionID string) (SessionMigrationArtifactCounts, error) {
	counts := SessionMigrationArtifactCounts{}
	tableCounts := []struct {
		name string
		dst  *int
	}{
		{"chat_logs", &counts.ChatLogs},
		{"effective_input_logs", &counts.EffectiveInputs},
		{"memories", &counts.Memories},
		{"direct_evidence_records", &counts.DirectEvidence},
		{"kg_triples", &counts.KGTriples},
		{"episode_summaries", &counts.Episodes},
		{"protagonist_entity_memories", &counts.SubjectiveEntityMemories},
		{"session_reference_bindings", &counts.ReferenceBindings},
	}
	for _, item := range tableCounts {
		query := "SELECT COUNT(*) FROM " + item.name + " WHERE chat_session_id = ?"
		if item.name == "protagonist_entity_memories" {
			query = "SELECT COUNT(*) FROM protagonist_entity_memories WHERE source_chat_session_id = ?"
		} else if item.name == "session_reference_bindings" {
			query = "SELECT COUNT(*) FROM session_reference_bindings WHERE chat_session_id = ?"
		}
		if err := tx.QueryRowContext(ctx, query, sessionID).Scan(item.dst); err != nil {
			return counts, err
		}
	}
	sessionMigrationFinalizeCounts(&counts)
	if counts.CanonicalAndSubjectiveTotal == 1 && counts.ChatLogs == 1 {
		var matching int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM chat_logs
			WHERE chat_session_id = ? AND turn_index = 0 AND LOWER(TRIM(role)) = 'assistant'
		`, sessionID).Scan(&matching); err != nil {
			return counts, err
		}
		counts.ReplaceableStarterOnly = matching == 1
	}
	return counts, nil
}

func previewSessionMigrationSourceCleanupTx(ctx context.Context, tx *sql.Tx, migrationID int64) (*SessionMigrationCleanupPreview, error) {
	var sourceID, targetID, status string
	err := tx.QueryRowContext(ctx, `
		SELECT source_session_id, target_session_id, status
		FROM session_migrations
		WHERE id = ?
		FOR UPDATE
	`, migrationID).Scan(&sourceID, &targetID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	counts, err := sessionMigrationCountArtifactsTx(ctx, tx, sourceID)
	if err != nil {
		return nil, err
	}
	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	sourceLocked := lock != nil && lock.MigrationID == migrationID && lock.Locked
	blocked := []string{}
	if status != "source_locked" && status != "cleanup_prepared" {
		blocked = append(blocked, "migration_status_not_source_locked")
	}
	if !sourceLocked {
		blocked = append(blocked, "active_source_lock_not_found_for_migration")
	}
	return &SessionMigrationCleanupPreview{
		MigrationID:     migrationID,
		SourceSessionID: sourceID,
		TargetSessionID: targetID,
		Status:          status,
		SourceLocked:    sourceLocked,
		Counts:          counts,
		BlockedReasons:  blocked,
		ReadyForCleanup: len(blocked) == 0,
	}, nil
}

func sessionMigrationDeleteSourceManifestRowsTx(
	ctx context.Context,
	tx *sql.Tx,
	sourceSessionID string,
) (map[string]int, error) {
	manifest := SessionMigrationManifest()
	deletedByTable := map[string]int{}
	for index := len(manifest) - 1; index >= 0; index-- {
		entry := manifest[index]
		if !entry.Direct {
			continue
		}
		if entry.Policy != SessionMigrationPolicyCopy && entry.Policy != SessionMigrationPolicyDeleteAfterVerified {
			continue
		}
		result, err := tx.ExecContext(ctx,
			"DELETE FROM "+sessionMigrationQuoteIdentifier(entry.Table)+" WHERE "+
				sessionMigrationQuoteIdentifier(entry.SessionColumn)+" = ?",
			sourceSessionID,
		)
		if err != nil {
			return nil, fmt.Errorf("session migration source cleanup %s: %w", entry.Table, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		deletedByTable[entry.Table] = int(affected)
	}
	return deletedByTable, nil
}

func sessionMigrationDeletedTableTotal(deleted map[string]int) int {
	total := 0
	for _, count := range deleted {
		total += count
	}
	return total
}

func rollbackSessionMigrationManifestRowsTx(ctx context.Context, tx *sql.Tx, migrationID int64) (map[string]int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT table_name
		FROM session_migration_artifact_row_map
		WHERE migration_id = ? AND row_status <> 'rolled_back'
		ORDER BY table_name
	`, migrationID)
	if err != nil {
		return nil, 0, err
	}
	mappedTables := map[string]bool{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return nil, 0, err
		}
		entry, ok := sessionMigrationManifestEntryByTable(table)
		if !ok || entry.Policy != SessionMigrationPolicyCopy {
			rows.Close()
			return nil, 0, fmt.Errorf("session migration rollback blocked: unsupported mapped table %q", table)
		}
		mappedTables[table] = true
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	manifest := SessionMigrationManifest()
	deletedByTable := map[string]int{}
	total := 0
	for index := len(manifest) - 1; index >= 0; index-- {
		entry := manifest[index]
		if !mappedTables[entry.Table] {
			continue
		}
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		if len(plan.PrimaryKey) != 1 {
			return nil, total, fmt.Errorf("session migration rollback blocked: composite key table %q", entry.Table)
		}
		keys, err := sessionMigrationMappedTargetKeysTx(ctx, tx, migrationID, entry.Table, plan.PrimaryKey[0])
		if err != nil {
			return nil, total, err
		}
		for _, fk := range plan.ForeignKeys {
			if !fk.Deferred || fk.ReferenceTable != entry.Table {
				continue
			}
			if _, err := execSessionMigrationStringBatch(ctx, tx,
				"UPDATE "+sessionMigrationQuoteIdentifier(entry.Table)+" SET "+
					sessionMigrationQuoteIdentifier(fk.Column)+" = NULL WHERE "+
					sessionMigrationQuoteIdentifier(fk.Column)+" IN", keys); err != nil {
				return nil, total, err
			}
		}
		deleted, err := execSessionMigrationStringBatch(ctx, tx,
			"DELETE FROM "+sessionMigrationQuoteIdentifier(entry.Table)+" WHERE "+
				sessionMigrationQuoteIdentifier(plan.PrimaryKey[0])+" IN", keys)
		if err != nil {
			return nil, total, err
		}
		if deleted != len(keys) {
			return nil, total, fmt.Errorf("session migration rollback %s deleted %d/%d mapped rows", entry.Table, deleted, len(keys))
		}
		deletedByTable[entry.Table] = deleted
		total += deleted
	}
	return deletedByTable, total, nil
}

func sessionMigrationMappedTargetKeysTx(
	ctx context.Context,
	tx *sql.Tx,
	migrationID int64,
	table, primaryKey string,
) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT target_key
		FROM session_migration_artifact_row_map
		WHERE migration_id = ? AND table_name = ? AND key_column_name = ?
		  AND row_status <> 'rolled_back'
		ORDER BY target_key
	`, migrationID, table, primaryKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func execSessionMigrationStringBatch(ctx context.Context, tx *sql.Tx, prefix string, values []string) (int, error) {
	total := 0
	const batchSize = 500
	for start := 0; start < len(values); start += batchSize {
		end := start + batchSize
		if end > len(values) {
			end = len(values)
		}
		batch := values[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for index, value := range batch {
			placeholders[index] = "?"
			args[index] = value
		}
		result, err := tx.ExecContext(ctx, prefix+" ("+strings.Join(placeholders, ",")+")", args...)
		if err != nil {
			return total, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return total, err
		}
		total += int(affected)
	}
	return total, nil
}

func insertSessionMigrationRowMap(ctx context.Context, tx *sql.Tx, migrationID int64, tableName string, sourceRowID, targetRowID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO session_migration_row_map (
			migration_id, table_name, source_row_id, target_row_id, row_status
		) VALUES (?, ?, ?, ?, 'copied')
	`, migrationID, tableName, sourceRowID, targetRowID)
	return err
}

// ---------------------------------------------------------------------------
// RollbackStore implementation
// ---------------------------------------------------------------------------
