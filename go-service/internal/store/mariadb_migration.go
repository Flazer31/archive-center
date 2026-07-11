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

	_ "github.com/go-sql-driver/mysql"
)

// mariadbStore is the R1 MariaDB shadow target implementation.
// It is opened only through AC_STORE_MODE=mariadb_shadow and remains behind
// the dual-write wrapper with noop primary, so it is not an authority switch.
var _ EffectiveInputListStore = (*mariadbStore)(nil)
var _ SessionMigrationStore = (*mariadbStore)(nil)
var _ SessionMigrationVectorStore = (*mariadbStore)(nil)
var _ SessionMigrationSourceLockStore = (*mariadbStore)(nil)
var _ SessionMigrationRecoveryStore = (*mariadbStore)(nil)

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

	sourceCounts, err := sessionMigrationCountArtifactsTx(ctx, tx, sourceID)
	if err != nil {
		return nil, err
	}
	if sourceCounts.CanonicalAndSubjectiveTotal == 0 {
		return nil, errors.New("source session has no archive data")
	}
	targetCounts, err := sessionMigrationCountArtifactsTx(ctx, tx, targetID)
	if err != nil {
		return nil, err
	}
	if targetCounts.CanonicalAndSubjectiveTotal > 0 {
		return nil, errors.New("target session is not empty")
	}

	initialCountsJSON, _ := json.Marshal(sourceCounts)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO session_migrations (
			source_session_id, target_session_id, mode, status, operator_note,
			counts_json, chroma_reindexed_count, errors_json
		) VALUES (?, ?, ?, 'copying', ?, ?, 0, JSON_ARRAY())
	`, sourceID, targetID, mode, nullableString(strings.TrimSpace(req.OperatorNote)), string(initialCountsJSON))
	if err != nil {
		return nil, err
	}
	migrationID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	counts := SessionMigrationArtifactCounts{}
	rowMapCount := 0
	count, maps, err := copySessionMigrationChatLogs(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy chat_logs: %w", err)
	}
	counts.ChatLogs = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEffectiveInputs(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy effective_input_logs: %w", err)
	}
	counts.EffectiveInputs = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationMemories(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy memories: %w", err)
	}
	counts.Memories = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEvidence(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy direct_evidence_records: %w", err)
	}
	counts.DirectEvidence = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationKGTriples(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy kg_triples: %w", err)
	}
	counts.KGTriples = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationEpisodes(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy episode_summaries: %w", err)
	}
	counts.Episodes = count
	rowMapCount += maps

	count, maps, err = copySessionMigrationSubjectiveEntityMemories(ctx, tx, migrationID, sourceID, targetID)
	if err != nil {
		return nil, fmt.Errorf("copy protagonist_entity_memories: %w", err)
	}
	counts.SubjectiveEntityMemories = count
	rowMapCount += maps
	sessionMigrationFinalizeCounts(&counts)

	countsJSON, _ := json.Marshal(counts)
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'copied',
		    counts_json = ?,
		    errors_json = JSON_ARRAY('chroma_reindex_pending'),
		    completed_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, string(countsJSON), migrationID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	return &SessionMigrationCompleteResult{
		MigrationID:           migrationID,
		Status:                "copied",
		SourceSessionID:       sourceID,
		TargetSessionID:       targetID,
		Mode:                  mode,
		Counts:                counts,
		RowMapCount:           rowMapCount,
		ChromaReindexedCount:  0,
		SourceLocked:          false,
		ChromaReindexRequired: true,
		ReadyForLive:          false,
	}, nil
}

func (m *mariadbStore) ListSessionMigrationVectorDocuments(ctx context.Context, migrationID int64) ([]SessionMigrationVectorDocument, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	docs := []SessionMigrationVectorDocument{}
	memoryRows, err := m.db.QueryContext(ctx, `
		SELECT sm.id, sm.source_session_id, sm.target_session_id, m.id, m.embedding,
		       m.summary_json, m.evidence
		FROM session_migration_row_map rm
		JOIN session_migrations sm ON sm.id = rm.migration_id
		JOIN memories m ON m.id = rm.target_row_id
		WHERE rm.migration_id = ? AND rm.table_name = 'memories'
		ORDER BY m.turn_index ASC, m.id ASC
	`, migrationID)
	if err != nil {
		return nil, err
	}
	for memoryRows.Next() {
		var doc SessionMigrationVectorDocument
		var targetRowID int64
		var summaryJSON, evidence sql.NullString
		if err := memoryRows.Scan(&doc.MigrationID, &doc.MigratedFromSessionID, &doc.ChatSessionID, &targetRowID,
			&doc.EmbeddingJSON, &summaryJSON, &evidence); err != nil {
			memoryRows.Close()
			return nil, err
		}
		doc.Tier = "memory"
		doc.SourceTable = "memories"
		doc.SourceRowID = strconv.FormatInt(targetRowID, 10)
		doc.ID = "memory:" + doc.ChatSessionID + ":" + doc.SourceRowID
		doc.SchemaVersion = "memory.v1"
		doc.DocumentText = sessionMigrationMemoryDocumentText(stringFromNull(summaryJSON), stringFromNull(evidence))
		docs = append(docs, doc)
	}
	if err := memoryRows.Close(); err != nil {
		return nil, err
	}
	if err := memoryRows.Err(); err != nil {
		return nil, err
	}

	episodeRows, err := m.db.QueryContext(ctx, `
		SELECT sm.id, sm.source_session_id, sm.target_session_id, e.id, e.embedding_vector,
		       e.summary_text
		FROM session_migration_row_map rm
		JOIN session_migrations sm ON sm.id = rm.migration_id
		JOIN episode_summaries e ON e.id = rm.target_row_id
		WHERE rm.migration_id = ? AND rm.table_name = 'episode_summaries'
		ORDER BY e.from_turn ASC, e.to_turn ASC, e.id ASC
	`, migrationID)
	if err != nil {
		return nil, err
	}
	defer episodeRows.Close()
	for episodeRows.Next() {
		var doc SessionMigrationVectorDocument
		var targetRowID int64
		if err := episodeRows.Scan(&doc.MigrationID, &doc.MigratedFromSessionID, &doc.ChatSessionID, &targetRowID,
			&doc.EmbeddingJSON, &doc.DocumentText); err != nil {
			return nil, err
		}
		doc.Tier = "episode"
		doc.SourceTable = "episode_summaries"
		doc.SourceRowID = strconv.FormatInt(targetRowID, 10)
		doc.ID = "episode:" + doc.ChatSessionID + ":" + doc.SourceRowID
		doc.SchemaVersion = "episode.v1"
		docs = append(docs, doc)
	}
	return docs, episodeRows.Err()
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

func (m *mariadbStore) LockSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationSourceLockResult, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if migrationID <= 0 {
		return nil, ErrNotFound
	}
	tx, err := m.db.BeginTx(ctx, nil)
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
	if reindexedCount <= 0 && status != "source_locked" {
		return nil, errors.New("session migration source lock blocked: chroma_reindexed_count is zero")
	}

	lock, err := sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if lock != nil && lock.MigrationID != migrationID {
		return nil, fmt.Errorf("session migration source lock blocked: source session is already locked by migration %d", lock.MigrationID)
	}
	if lock == nil {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO session_migration_locks (
				migration_id, source_session_id, target_session_id, locked, lock_status, reason
			) VALUES (?, ?, ?, TRUE, 'migrated_away', ?)
		`, migrationID, sourceID, targetID, strings.TrimSpace(reason))
		if err != nil {
			return nil, err
		}
		lock, err = sessionMigrationSelectActiveLockTx(ctx, tx, sourceID)
		if err != nil {
			return nil, err
		}
	}

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

	counts := SessionMigrationArtifactCounts{}
	rowMapCount := 0
	deletionPlan := []struct {
		table string
		dst   *int
	}{
		{"protagonist_entity_memories", &counts.SubjectiveEntityMemories},
		{"episode_summaries", &counts.Episodes},
		{"kg_triples", &counts.KGTriples},
		{"memories", &counts.Memories},
		{"direct_evidence_records", &counts.DirectEvidence},
		{"effective_input_logs", &counts.EffectiveInputs},
		{"chat_logs", &counts.ChatLogs},
	}
	for _, item := range deletionPlan {
		deleted, err := deleteSessionMigrationMappedRows(ctx, tx, migrationID, item.table)
		if err != nil {
			return nil, err
		}
		*item.dst = deleted
		rowMapCount += deleted
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
	return preview, nil
}

func (m *mariadbStore) CleanupSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationCleanupResult, error) {
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

	preview, err := previewSessionMigrationSourceCleanupTx(ctx, tx, migrationID)
	if err != nil {
		return nil, err
	}
	if !preview.ReadyForCleanup {
		return nil, fmt.Errorf("session migration source cleanup blocked: %s", strings.Join(preview.BlockedReasons, ","))
	}

	if err := deleteSessionRowsTx(ctx, tx, preview.SourceSessionID); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migrations
		SET status = 'source_cleaned',
		    cleanup_at = CURRENT_TIMESTAMP(3),
		    errors_json = JSON_ARRAY(),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE id = ?
	`, migrationID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE session_migration_locks
		SET lock_status = 'source_cleaned',
		    reason = CONCAT(COALESCE(reason, ''), CASE WHEN COALESCE(reason, '') = '' THEN '' ELSE '\n' END, ?),
		    updated_at = CURRENT_TIMESTAMP(3)
		WHERE migration_id = ? AND locked = TRUE AND unlocked_at IS NULL
	`, strings.TrimSpace(reason), migrationID)
	if err != nil {
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
		Counts:          preview.Counts,
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

func sessionMigrationMemoryDocumentText(summaryJSON, evidence string) string {
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(summaryJSON), &parsed); err == nil {
		for _, key := range []string{"summary", "turn_summary", "text", "content"} {
			if value := strings.TrimSpace(fmt.Sprint(parsed[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	if text := strings.TrimSpace(evidence); text != "" {
		return text
	}
	return strings.TrimSpace(summaryJSON)
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
	}
	for _, item := range tableCounts {
		query := "SELECT COUNT(*) FROM " + item.name + " WHERE chat_session_id = ?"
		if item.name == "protagonist_entity_memories" {
			query = "SELECT COUNT(*) FROM protagonist_entity_memories WHERE source_chat_session_id = ?"
		}
		if err := tx.QueryRowContext(ctx, query, sessionID).Scan(item.dst); err != nil {
			return counts, err
		}
	}
	sessionMigrationFinalizeCounts(&counts)
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
	if status != "source_locked" {
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

func deleteSessionMigrationMappedRows(ctx context.Context, tx *sql.Tx, migrationID int64, tableName string) (int, error) {
	if !sessionMigrationAllowedMappedTable(tableName) {
		return 0, fmt.Errorf("session migration rollback blocked: unsupported mapped table %q", tableName)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT target_row_id
		FROM session_migration_row_map
		WHERE migration_id = ? AND table_name = ? AND target_row_id IS NOT NULL
		ORDER BY target_row_id ASC
	`, migrationID, tableName)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if tableName == "direct_evidence_records" {
		if _, err := execIDBatch(ctx, tx, "UPDATE direct_evidence_records SET superseded_by_id = NULL WHERE superseded_by_id IN", ids); err != nil {
			return 0, err
		}
	}
	return execIDBatch(ctx, tx, "DELETE FROM "+tableName+" WHERE id IN", ids)
}

func sessionMigrationAllowedMappedTable(tableName string) bool {
	switch tableName {
	case "chat_logs", "effective_input_logs", "memories", "direct_evidence_records", "kg_triples", "episode_summaries", "protagonist_entity_memories":
		return true
	default:
		return false
	}
}

func execIDBatch(ctx context.Context, tx *sql.Tx, prefix string, ids []int64) (int, error) {
	total := 0
	const batchSize = 500
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = "?"
			args[i] = id
		}
		res, err := tx.ExecContext(ctx, prefix+" ("+strings.Join(placeholders, ",")+")", args...)
		if err != nil {
			return total, err
		}
		affected, _ := res.RowsAffected()
		total += int(affected)
	}
	return total, nil
}

func deleteSessionRowsTx(ctx context.Context, tx *sql.Tx, chatSessionID string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM persona_capsule_attachments WHERE target_chat_session_id = ?", chatSessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM protagonist_entity_memories WHERE source_chat_session_id = ?", chatSessionID); err != nil {
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
		"critic_feedback",
	}
	for _, tbl := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE chat_session_id = ?", chatSessionID); err != nil {
			return err
		}
	}
	return nil
}

func copySessionMigrationChatLogs(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, role, content, created_at
		FROM chat_logs
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type chatLogCopyRow struct {
		sourceRowID int64
		turnIndex   int
		role        string
		content     string
		createdAt   time.Time
	}
	items := []chatLogCopyRow{}
	for rows.Next() {
		var item chatLogCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.turnIndex, &item.role, &item.content, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO chat_logs (chat_session_id, turn_index, role, content, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, targetID, item.turnIndex, item.role, item.content, nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "chat_logs", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func copySessionMigrationEffectiveInputs(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, effective_input, created_at
		FROM effective_input_logs
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type effectiveInputCopyRow struct {
		sourceRowID    int64
		turnIndex      int
		effectiveInput string
		createdAt      time.Time
	}
	items := []effectiveInputCopyRow{}
	for rows.Next() {
		var item effectiveInputCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.turnIndex, &item.effectiveInput, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO effective_input_logs (chat_session_id, turn_index, effective_input, created_at)
			VALUES (?, ?, ?, ?)
		`, targetID, item.turnIndex, item.effectiveInput, nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "effective_input_logs", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func copySessionMigrationMemories(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, turn_index, summary_json, embedding, embedding_model, importance,
		       emotional_boost, evidence, emotional_intensity, narrative_significance,
		       place_wing, place_room, created_at
		FROM memories
		WHERE chat_session_id = ?
		ORDER BY turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type memoryCopyRow struct {
		sourceRowID           int64
		turnIndex             int
		summaryJSON           sql.NullString
		embedding             sql.NullString
		embeddingModel        sql.NullString
		evidence              sql.NullString
		placeWing             sql.NullString
		placeRoom             sql.NullString
		importance            sql.NullFloat64
		emotionalBoost        sql.NullFloat64
		emotionalIntensity    sql.NullFloat64
		narrativeSignificance sql.NullFloat64
		createdAt             time.Time
	}
	items := []memoryCopyRow{}
	for rows.Next() {
		var item memoryCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.turnIndex, &item.summaryJSON, &item.embedding, &item.embeddingModel,
			&item.importance, &item.emotionalBoost, &item.evidence, &item.emotionalIntensity, &item.narrativeSignificance,
			&item.placeWing, &item.placeRoom, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO memories (
				chat_session_id, turn_index, summary_json, embedding, embedding_model,
				importance, emotional_boost, evidence, emotional_intensity,
				narrative_significance, place_wing, place_room, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, item.turnIndex, nullStringArg(item.summaryJSON), nullStringArg(item.embedding), nullStringArg(item.embeddingModel),
			nullFloatArg(item.importance), nullFloatArg(item.emotionalBoost), nullStringArg(item.evidence),
			nullFloatArg(item.emotionalIntensity), nullFloatArg(item.narrativeSignificance),
			nullStringArg(item.placeWing), nullStringArg(item.placeRoom), nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "memories", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func copySessionMigrationEvidence(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
		       turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
		       capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
		       superseded_by_id, created_at
		FROM direct_evidence_records
		WHERE chat_session_id = ?
		ORDER BY source_turn_start ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type evidenceCopyRow struct {
		sourceRowID          int64
		evidenceKind         string
		evidenceText         string
		archiveState         string
		captureStage         string
		captureVerification  string
		sourceTurnStart      int
		sourceTurnEnd        int
		turnAnchor           sql.NullInt64
		supersededByID       sql.NullInt64
		sourceMessageIDsJSON sql.NullString
		sourceHash           sql.NullString
		committedGate        sql.NullString
		lineageJSON          sql.NullString
		repairNeeded         bool
		tombstoned           bool
		createdAt            time.Time
	}
	items := []evidenceCopyRow{}
	for rows.Next() {
		var item evidenceCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.evidenceKind, &item.evidenceText, &item.sourceTurnStart, &item.sourceTurnEnd,
			&item.turnAnchor, &item.sourceMessageIDsJSON, &item.sourceHash, &item.archiveState, &item.captureStage,
			&item.captureVerification, &item.committedGate, &item.lineageJSON, &item.repairNeeded, &item.tombstoned,
			&item.supersededByID, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	targetBySource := map[int64]int64{}
	supersededByOldSource := map[int64]int64{}
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO direct_evidence_records (
				chat_session_id, evidence_kind, evidence_text, source_turn_start, source_turn_end,
				turn_anchor, source_message_ids_json, source_hash, archive_state, capture_stage,
				capture_verification, committed_gate, lineage_json, repair_needed, tombstoned,
				superseded_by_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, item.evidenceKind, item.evidenceText, item.sourceTurnStart, item.sourceTurnEnd,
			nullIntArg(item.turnAnchor), nullStringArg(item.sourceMessageIDsJSON), nullStringArg(item.sourceHash),
			item.archiveState, item.captureStage, item.captureVerification, nullStringArg(item.committedGate),
			nullStringArg(item.lineageJSON), item.repairNeeded, item.tombstoned, nullIntArg(item.supersededByID),
			nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		targetBySource[item.sourceRowID] = targetRowID
		if item.supersededByID.Valid {
			supersededByOldSource[targetRowID] = item.supersededByID.Int64
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "direct_evidence_records", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	for targetRowID, oldSupersededID := range supersededByOldSource {
		if newSupersededID, ok := targetBySource[oldSupersededID]; ok {
			if _, err := tx.ExecContext(ctx, `
				UPDATE direct_evidence_records
				SET superseded_by_id = ?
				WHERE id = ?
			`, newSupersededID, targetRowID); err != nil {
				return 0, 0, err
			}
		}
	}
	return count, mapped, nil
}

func copySessionMigrationKGTriples(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, subject, predicate, object, valid_from, valid_to, source_turn, created_at
		FROM kg_triples
		WHERE chat_session_id = ?
		ORDER BY id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type kgTripleCopyRow struct {
		sourceRowID int64
		subject     string
		predicate   string
		object      string
		validFrom   sql.NullInt64
		validTo     sql.NullInt64
		sourceTurn  sql.NullInt64
		createdAt   time.Time
	}
	items := []kgTripleCopyRow{}
	for rows.Next() {
		var item kgTripleCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.subject, &item.predicate, &item.object, &item.validFrom, &item.validTo, &item.sourceTurn, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO kg_triples (chat_session_id, subject, predicate, object, valid_from, valid_to, source_turn, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, item.subject, item.predicate, item.object, nullIntArg(item.validFrom), nullIntArg(item.validTo), nullIntArg(item.sourceTurn), nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "kg_triples", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func copySessionMigrationEpisodes(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, from_turn, to_turn, summary_text, key_entities, key_events,
		       open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
		FROM episode_summaries
		WHERE chat_session_id = ?
		ORDER BY from_turn ASC, to_turn ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type episodeCopyRow struct {
		sourceRowID             int64
		fromTurn                int
		toTurn                  int
		summaryText             string
		keyEntities             sql.NullString
		keyEvents               sql.NullString
		openLoopsJSON           sql.NullString
		relationshipChangesJSON sql.NullString
		embeddingVector         sql.NullString
		embeddingModel          sql.NullString
		createdAt               time.Time
	}
	items := []episodeCopyRow{}
	for rows.Next() {
		var item episodeCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.fromTurn, &item.toTurn, &item.summaryText, &item.keyEntities, &item.keyEvents,
			&item.openLoopsJSON, &item.relationshipChangesJSON, &item.embeddingVector, &item.embeddingModel, &item.createdAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO episode_summaries (
				chat_session_id, from_turn, to_turn, summary_text, key_entities, key_events,
				open_loops_json, relationship_changes_json, embedding_vector, embedding_model, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, targetID, item.fromTurn, item.toTurn, item.summaryText, nullStringArg(item.keyEntities), nullStringArg(item.keyEvents),
			nullStringArg(item.openLoopsJSON), nullStringArg(item.relationshipChangesJSON), nullStringArg(item.embeddingVector),
			nullStringArg(item.embeddingModel), nonZeroTime(item.createdAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "episode_summaries", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func copySessionMigrationSubjectiveEntityMemories(ctx context.Context, tx *sql.Tx, migrationID int64, sourceID, targetID string) (int, int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
		       owner_entity_role, owner_visibility, source_character_name, source_turn_index,
		       memory_text, evidence_excerpt, secret_guard, portability, tags_json,
		       target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
		FROM protagonist_entity_memories
		WHERE source_chat_session_id = ?
		ORDER BY source_turn_index ASC, id ASC
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	type subjectiveEntityMemoryCopyRow struct {
		sourceRowID         int64
		personaEntityKey    string
		personaEntityName   string
		ownerEntityKey      string
		ownerEntityName     string
		ownerEntityRole     string
		ownerVisibility     string
		memoryText          string
		portability         string
		targetRevealPolicy  string
		sourceCharacterName sql.NullString
		evidenceExcerpt     sql.NullString
		tagsJSON            sql.NullString
		sourceTurn          sql.NullInt64
		secretGuard         bool
		importance10        sql.NullFloat64
		emotionalWeight     sql.NullFloat64
		createdAt           time.Time
		updatedAt           time.Time
	}
	items := []subjectiveEntityMemoryCopyRow{}
	for rows.Next() {
		var item subjectiveEntityMemoryCopyRow
		if err := rows.Scan(&item.sourceRowID, &item.personaEntityKey, &item.personaEntityName, &item.ownerEntityKey, &item.ownerEntityName,
			&item.ownerEntityRole, &item.ownerVisibility, &item.sourceCharacterName, &item.sourceTurn,
			&item.memoryText, &item.evidenceExcerpt, &item.secretGuard, &item.portability, &item.tagsJSON,
			&item.targetRevealPolicy, &item.importance10, &item.emotionalWeight, &item.createdAt, &item.updatedAt); err != nil {
			return 0, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	count, mapped := 0, 0
	for _, item := range items {
		targetRowID, err := insertSessionMigrationCopiedRow(ctx, tx, `
			INSERT INTO protagonist_entity_memories (
				persona_entity_key, persona_entity_name, owner_entity_key, owner_entity_name,
				owner_entity_role, owner_visibility, source_chat_session_id, source_character_name,
				source_turn_index, memory_text, evidence_excerpt, secret_guard, portability,
				tags_json, target_reveal_policy, importance_10, emotional_weight, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.personaEntityKey, item.personaEntityName, item.ownerEntityKey, item.ownerEntityName,
			item.ownerEntityRole, item.ownerVisibility, targetID, nullStringArg(item.sourceCharacterName),
			nullIntArg(item.sourceTurn), item.memoryText, nullStringArg(item.evidenceExcerpt), item.secretGuard, item.portability,
			nullStringArg(item.tagsJSON), item.targetRevealPolicy, nullFloatArg(item.importance10), nullFloatArg(item.emotionalWeight),
			nonZeroTime(item.createdAt), nonZeroTime(item.updatedAt))
		if err != nil {
			return 0, 0, err
		}
		if err := insertSessionMigrationRowMap(ctx, tx, migrationID, "protagonist_entity_memories", item.sourceRowID, targetRowID); err != nil {
			return 0, 0, err
		}
		count++
		mapped++
	}
	return count, mapped, nil
}

func insertSessionMigrationCopiedRow(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertSessionMigrationRowMap(ctx context.Context, tx *sql.Tx, migrationID int64, tableName string, sourceRowID, targetRowID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO session_migration_row_map (
			migration_id, table_name, source_row_id, target_row_id, row_status
		) VALUES (?, ?, ?, ?, 'copied')
	`, migrationID, tableName, sourceRowID, targetRowID)
	return err
}

func nullStringArg(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	return v.String
}

func nullIntArg(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func nullFloatArg(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

// ---------------------------------------------------------------------------
// RollbackStore implementation
// ---------------------------------------------------------------------------
