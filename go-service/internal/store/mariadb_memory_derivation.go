package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	memoryVectorDeleteClaimBatchSize    = 128
	memoryVectorUpsertClaimBatchSize    = 32
	memoryVectorDeleteCoalesceBatchSize = 512
)

var _ SourceRevisionStore = (*mariadbStore)(nil)
var _ CriticInputSnapshotStore = (*mariadbStore)(nil)
var _ MemoryDerivationLifecycleAvailability = (*mariadbStore)(nil)
var _ MemoryReprocessingJobStore = (*mariadbStore)(nil)
var _ MemoryReprocessingWakeScheduleStore = (*mariadbStore)(nil)
var _ MemoryReprocessingJobReopener = (*mariadbStore)(nil)
var _ MemoryVectorOutboxStore = (*mariadbStore)(nil)
var _ MemoryVectorMaterializedCompletionStore = (*mariadbStore)(nil)
var _ MemoryVectorOutboxMaintenanceStore = (*mariadbStore)(nil)

func (m *mariadbStore) MemoryDerivationLifecycleEnabled() bool {
	return m != nil && m.db != nil
}

func (m *mariadbStore) RegisterAcceptedSourceRevision(ctx context.Context, source *MemorySourceRevision) (SourceRevisionRegistration, error) {
	var result SourceRevisionRegistration
	if err := m.ensureDB(); err != nil {
		return result, err
	}
	if err := validateMemorySourceRevision(source); err != nil {
		return result, err
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
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

	var canonicalTailTurn int
	if err := tx.QueryRowContext(ctx, `
		SELECT turn_index
		FROM chat_logs
		WHERE chat_session_id = ?
		ORDER BY turn_index DESC, id DESC
		LIMIT 1 FOR UPDATE
	`, source.ChatSessionID).Scan(&canonicalTailTurn); err != nil {
		return result, err
	}

	activeRows, err := tx.QueryContext(ctx, `
		SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content
		FROM memory_source_revisions
		WHERE chat_session_id = ?
		  AND lifecycle_state = 'active'
		  AND (logical_turn_id = ? OR turn_index = ?)
		ORDER BY host_observed_at_ms DESC, id DESC
		LIMIT 2 FOR UPDATE
	`, source.ChatSessionID, source.LogicalTurnID, source.TurnIndex)
	if err != nil {
		return result, err
	}
	type activeSourceSnapshot struct {
		revision  string
		hash      string
		user      string
		assistant string
	}
	activeSources := []activeSourceSnapshot{}
	for activeRows.Next() {
		var item activeSourceSnapshot
		if err := activeRows.Scan(&item.revision, &item.hash, &item.user, &item.assistant); err != nil {
			_ = activeRows.Close()
			return result, err
		}
		activeSources = append(activeSources, item)
	}
	if err := activeRows.Close(); err != nil {
		return result, err
	}
	if err := activeRows.Err(); err != nil {
		return result, err
	}
	if len(activeSources) > 1 {
		return result, ErrSourceRevisionConflict
	}

	canonicalRows, err := tx.QueryContext(ctx, `
		SELECT role, content
		FROM chat_logs
		WHERE chat_session_id = ? AND turn_index = ?
		ORDER BY id
		FOR UPDATE
	`, source.ChatSessionID, source.TurnIndex)
	if err != nil {
		return result, err
	}
	var canonicalUser, canonicalAssistant string
	userRows := 0
	assistantRows := 0
	for canonicalRows.Next() {
		var role, content string
		if err := canonicalRows.Scan(&role, &content); err != nil {
			_ = canonicalRows.Close()
			return result, err
		}
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "user":
			userRows++
			canonicalUser = content
		case "assistant":
			assistantRows++
			canonicalAssistant = content
		}
	}
	if err := canonicalRows.Close(); err != nil {
		return result, err
	}
	if err := canonicalRows.Err(); err != nil {
		return result, err
	}
	userInputMissing := strings.TrimSpace(source.UserContent) == ""
	userRowsValid := userRows == 1 && canonicalUser == source.UserContent
	if userInputMissing {
		userRowsValid = userRows == 0
	}
	if !userRowsValid || assistantRows != 1 || canonicalAssistant != source.AssistantContent {
		return result, ErrSourceRevisionConflict
	}

	switch {
	case len(activeSources) == 1 && activeSources[0].revision == source.SourceRevision:
		active := activeSources[0]
		if active.hash != source.CombinedContentHash || active.user != source.UserContent || active.assistant != source.AssistantContent {
			return result, ErrSourceRevisionConflict
		}
		result.Idempotent = true
	case len(activeSources) == 1:
		return result, ErrSourceRevisionConflict
	default:
		if err := insertMemorySourceRevisionTx(ctx, tx, source); err != nil {
			if preciseMemoryDuplicateKeyError(err) {
				return result, ErrSourceRevisionConflict
			}
			return result, err
		}
		result.Inserted = true
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	return result, nil
}

func validateMemorySourceRevision(source *MemorySourceRevision) error {
	if source == nil ||
		strings.TrimSpace(source.SourceRevision) == "" ||
		strings.TrimSpace(source.ChatSessionID) == "" ||
		strings.TrimSpace(source.LogicalTurnID) == "" ||
		source.TurnIndex <= 0 ||
		strings.TrimSpace(source.AssistantContent) == "" ||
		strings.TrimSpace(source.CombinedContentHash) == "" ||
		source.HostObservedAtMS <= 0 {
		return fmt.Errorf("invalid accepted source revision")
	}
	if strings.TrimSpace(source.ContractVersion) == "" {
		source.ContractVersion = MemorySourceRevisionContract
	}
	if strings.TrimSpace(source.BranchState) == "" {
		source.BranchState = "not_exposed"
	}
	if strings.TrimSpace(source.LifecycleState) == "" {
		source.LifecycleState = "active"
	}
	return nil
}

type memoryDerivationSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func insertMemorySourceRevisionTx(ctx context.Context, exec memoryDerivationSQLExecutor, source *MemorySourceRevision) error {
	createdAt := nonZeroTime(source.CreatedAt)
	updatedAt := nonZeroTime(source.UpdatedAt)
	if source.UpdatedAt.IsZero() {
		updatedAt = createdAt
	}
	res, err := exec.ExecContext(ctx, `
		INSERT INTO memory_source_revisions (
			contract_version, source_revision, chat_session_id, logical_turn_id,
			turn_index, source_message_id, source_generation_id, branch_id,
			branch_state, raw_user_content, raw_assistant_content,
			combined_content_hash, user_observed_content_hash,
			assistant_observed_content_hash, hash_algorithm, host_observed_at_ms,
			lifecycle_state, superseded_by_revision, invalidation_reason,
			derived_admission_state, derived_admission_version,
			derived_extractor_version, derived_index_version,
			derived_result_hash, derived_result_json, derived_admitted_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, source.ContractVersion, source.SourceRevision, source.ChatSessionID,
		source.LogicalTurnID, source.TurnIndex, nullableString(source.SourceMessageID),
		nullableString(source.SourceGenerationID), nullableString(source.BranchID),
		source.BranchState, source.UserContent, source.AssistantContent,
		source.CombinedContentHash, nullableString(source.UserObservedContentHash),
		nullableString(source.AssistantObservedContentHash), source.HashAlgorithm,
		source.HostObservedAtMS, source.LifecycleState,
		nullableString(source.SupersededByRevision),
		nullableString(source.InvalidationReason),
		firstNonEmptyString(source.DerivedAdmissionState, "pending"),
		source.DerivedAdmissionVersion, source.DerivedExtractorVersion,
		source.DerivedIndexVersion, nullableString(source.DerivedResultHash),
		nullableString(source.DerivedResultJSON),
		nullableTime(source.DerivedAdmittedAt), createdAt, updatedAt)
	if err != nil {
		return err
	}
	if id, idErr := res.LastInsertId(); idErr == nil {
		source.ID = id
	}
	return nil
}

func (m *mariadbStore) GetSourceRevision(ctx context.Context, chatSessionID, sourceRevision string) (*MemorySourceRevision, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	source := &MemorySourceRevision{}
	var sourceMessageID, sourceGenerationID, branchID sql.NullString
	var supersededByRevision, invalidationReason sql.NullString
	var userObservedContentHash, assistantObservedContentHash sql.NullString
	var derivedResultHash sql.NullString
	var derivedResultJSON sql.NullString
	var criticInputSnapshotJSON sql.NullString
	var criticInputSnapshotHash sql.NullString
	var derivedAdmittedAt sql.NullTime
	err := m.db.QueryRowContext(ctx, `
		SELECT id, contract_version, source_revision, chat_session_id,
		       logical_turn_id, turn_index, source_message_id,
		       source_generation_id, branch_id, branch_state,
		       raw_user_content, raw_assistant_content, combined_content_hash,
		       user_observed_content_hash, assistant_observed_content_hash,
		       hash_algorithm, host_observed_at_ms, lifecycle_state,
		       superseded_by_revision, invalidation_reason,
		       derived_admission_state, derived_admission_version,
		       derived_extractor_version, derived_index_version,
		       derived_result_hash, derived_result_json, derived_admitted_at,
		       critic_input_snapshot_json, critic_input_snapshot_hash,
		       created_at, updated_at
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ?
	`, strings.TrimSpace(chatSessionID), strings.TrimSpace(sourceRevision)).Scan(
		&source.ID, &source.ContractVersion, &source.SourceRevision,
		&source.ChatSessionID, &source.LogicalTurnID, &source.TurnIndex,
		&sourceMessageID, &sourceGenerationID, &branchID, &source.BranchState,
		&source.UserContent, &source.AssistantContent,
		&source.CombinedContentHash, &userObservedContentHash,
		&assistantObservedContentHash, &source.HashAlgorithm,
		&source.HostObservedAtMS, &source.LifecycleState,
		&supersededByRevision, &invalidationReason,
		&source.DerivedAdmissionState, &source.DerivedAdmissionVersion,
		&source.DerivedExtractorVersion, &source.DerivedIndexVersion,
		&derivedResultHash, &derivedResultJSON, &derivedAdmittedAt,
		&criticInputSnapshotJSON, &criticInputSnapshotHash,
		&source.CreatedAt, &source.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	source.SourceMessageID = sourceMessageID.String
	source.SourceGenerationID = sourceGenerationID.String
	source.BranchID = branchID.String
	source.UserObservedContentHash = userObservedContentHash.String
	source.AssistantObservedContentHash = assistantObservedContentHash.String
	source.SupersededByRevision = supersededByRevision.String
	source.InvalidationReason = invalidationReason.String
	source.DerivedResultHash = derivedResultHash.String
	source.DerivedResultJSON = derivedResultJSON.String
	source.DerivedAdmittedAt = derivedAdmittedAt.Time
	source.CriticInputSnapshotJSON = criticInputSnapshotJSON.String
	source.CriticInputSnapshotHash = criticInputSnapshotHash.String
	return source, nil
}

func (m *mariadbStore) SaveCriticInputSnapshot(
	ctx context.Context,
	chatSessionID string,
	sourceRevision string,
	snapshotJSON string,
	snapshotHash string,
	updatedAt time.Time,
) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	sourceRevision = strings.TrimSpace(sourceRevision)
	snapshotJSON = strings.TrimSpace(snapshotJSON)
	snapshotHash = strings.ToLower(strings.TrimSpace(snapshotHash))
	if chatSessionID == "" || sourceRevision == "" || snapshotJSON == "" || len(snapshotHash) != sha256.Size*2 {
		return fmt.Errorf("invalid critic input snapshot")
	}
	var decoded any
	if json.Unmarshal([]byte(snapshotJSON), &decoded) != nil {
		return fmt.Errorf("invalid critic input snapshot json")
	}
	actualHash := sha256.Sum256([]byte(snapshotJSON))
	if hex.EncodeToString(actualHash[:]) != snapshotHash {
		return fmt.Errorf("critic input snapshot hash mismatch")
	}

	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	updatedAt = nonZeroTime(updatedAt)
	result, err := m.db.ExecContext(ctx, `
		UPDATE memory_source_revisions
		SET critic_input_snapshot_json = ?,
		    critic_input_snapshot_hash = ?,
		    updated_at = ?
		WHERE chat_session_id = ?
		  AND source_revision = ?
		  AND lifecycle_state = 'active'
		  AND critic_input_snapshot_hash IS NULL
	`, snapshotJSON, snapshotHash, updatedAt, chatSessionID, sourceRevision)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected == 1 {
		return nil
	}

	var lifecycleState string
	var existingHash sql.NullString
	err = m.db.QueryRowContext(ctx, `
		SELECT lifecycle_state, critic_input_snapshot_hash
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ?
	`, chatSessionID, sourceRevision).Scan(&lifecycleState, &existingHash)
	if err == sql.ErrNoRows || lifecycleState != "active" {
		return ErrSourceRevisionStale
	}
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(existingHash.String), snapshotHash) {
		return nil
	}
	return ErrSourceRevisionConflict
}

func (m *mariadbStore) IsSourceRevisionActive(ctx context.Context, chatSessionID, sourceRevision string) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	var active int
	err := m.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND source_revision = ? AND lifecycle_state = 'active'
	`, strings.TrimSpace(chatSessionID), strings.TrimSpace(sourceRevision)).Scan(&active)
	return active > 0, err
}

func (m *mariadbStore) ListActiveSourceRevisions(
	ctx context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
) ([]MemorySourceRevision, error) {
	return m.listSourceRevisions(ctx, chatSessionID, fromTurn, toTurn, true)
}

func (m *mariadbStore) ListSourceRevisions(
	ctx context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
) ([]MemorySourceRevision, error) {
	return m.listSourceRevisions(ctx, chatSessionID, fromTurn, toTurn, false)
}

func (m *mariadbStore) listSourceRevisions(
	ctx context.Context,
	chatSessionID string,
	fromTurn int,
	toTurn int,
	activeOnly bool,
) ([]MemorySourceRevision, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	where := "chat_session_id = ?"
	args := []any{strings.TrimSpace(chatSessionID)}
	if activeOnly {
		where += " AND lifecycle_state = 'active'"
	}
	if fromTurn > 0 {
		where += " AND turn_index >= ?"
		args = append(args, fromTurn)
	}
	if toTurn > 0 {
		where += " AND turn_index <= ?"
		args = append(args, toTurn)
	}
	rows, err := m.db.QueryContext(ctx, `
		SELECT source_revision, chat_session_id, logical_turn_id, turn_index,
		       source_message_id, source_generation_id, branch_id, branch_state,
		       raw_user_content, raw_assistant_content, combined_content_hash,
		       assistant_observed_content_hash, hash_algorithm,
		       host_observed_at_ms, lifecycle_state
		FROM memory_source_revisions
		WHERE `+where+`
		ORDER BY turn_index, CASE WHEN lifecycle_state = 'active' THEN 0 ELSE 1 END, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemorySourceRevision{}
	for rows.Next() {
		var item MemorySourceRevision
		var sourceMessageID, sourceGenerationID, branchID sql.NullString
		var assistantObservedContentHash, hashAlgorithm sql.NullString
		if err := rows.Scan(
			&item.SourceRevision, &item.ChatSessionID, &item.LogicalTurnID,
			&item.TurnIndex, &sourceMessageID, &sourceGenerationID,
			&branchID, &item.BranchState, &item.UserContent,
			&item.AssistantContent, &item.CombinedContentHash,
			&assistantObservedContentHash, &hashAlgorithm,
			&item.HostObservedAtMS, &item.LifecycleState,
		); err != nil {
			return nil, err
		}
		item.ContractVersion = MemorySourceRevisionContract
		item.SourceMessageID = sourceMessageID.String
		item.SourceGenerationID = sourceGenerationID.String
		item.BranchID = branchID.String
		item.AssistantObservedContentHash = assistantObservedContentHash.String
		item.HashAlgorithm = hashAlgorithm.String
		out = append(out, item)
	}
	return out, rows.Err()
}

func (m *mariadbStore) InvalidateSourceRevisions(ctx context.Context, chatSessionID string, fromTurn int, lifecycleState, reason string, invalidatedAt time.Time) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if strings.TrimSpace(chatSessionID) == "" || fromTurn <= 0 {
		return fmt.Errorf("invalid source invalidation")
	}
	switch lifecycleState {
	case "invalidated", "deleted", "superseded":
	default:
		return fmt.Errorf("invalid source lifecycle %q", lifecycleState)
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := invalidateMemorySourcesTx(ctx, tx, chatSessionID, fromTurn, false, "", lifecycleState, reason, nonZeroTime(invalidatedAt)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func invalidateMemorySourcesTx(
	ctx context.Context,
	tx *sql.Tx,
	chatSessionID string,
	fromTurn int,
	exactTurn bool,
	supersededByRevision string,
	lifecycleState string,
	reason string,
	now time.Time,
) error {
	comparison := "turn_index >= ?"
	if exactTurn {
		comparison = "turn_index = ?"
	}
	sourceStatePredicate := "lifecycle_state = 'active'"
	if lifecycleState == "deleted" {
		sourceStatePredicate = "lifecycle_state <> 'deleted'"
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT source_revision
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND `+comparison+` AND `+sourceStatePredicate+`
		ORDER BY turn_index, id
		FOR UPDATE
	`, chatSessionID, fromTurn)
	if err != nil {
		return err
	}
	var revisions []string
	for rows.Next() {
		var revision string
		if err := rows.Scan(&revision); err != nil {
			_ = rows.Close()
			return err
		}
		revisions = append(revisions, revision)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := enqueueKnownVectorDeletesTx(ctx, tx, chatSessionID, revisions, fromTurn, exactTurn, reason, now); err != nil {
		return err
	}
	for _, revision := range revisions {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_derivation_dependencies
			SET lifecycle_state = 'invalidated', invalidated_at = ?, updated_at = ?
			WHERE chat_session_id = ? AND source_revision = ? AND lifecycle_state = 'active'
		`, now, now, chatSessionID, revision); err != nil {
			return err
		}
		preciseStatePredicate := "AND lifecycle_state = 'active'"
		if lifecycleState == "deleted" {
			preciseStatePredicate = ""
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE precise_memory_units
			SET lifecycle_state = 'invalidated',
			    evidence_excerpt = CASE WHEN ? = 'deleted' THEN '' ELSE evidence_excerpt END,
			    direct_evidence_ids_json = CASE WHEN ? = 'deleted' THEN JSON_ARRAY() ELSE direct_evidence_ids_json END,
			    payload_json = CASE WHEN ? = 'deleted' THEN JSON_OBJECT() ELSE payload_json END,
			    relationship_key = CASE WHEN ? = 'deleted' THEN NULL ELSE relationship_key END,
			    reveal_condition = CASE WHEN ? = 'deleted' THEN NULL ELSE reveal_condition END,
			    updated_at = ?
			WHERE chat_session_id = ? AND source_revision = ? `+preciseStatePredicate+`
		`, lifecycleState, lifecycleState, lifecycleState, lifecycleState,
			lifecycleState, now, chatSessionID, revision); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_reprocessing_jobs
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    last_error = ?, updated_at = ?
			WHERE chat_session_id = ? AND source_revision = ?
			  AND status IN ('pending', 'leased', 'retryable')
		`, reason, now, chatSessionID, revision); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_vector_outbox
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    document_json = CASE WHEN ? = 'deleted' THEN NULL ELSE document_json END,
			    last_error = ?, updated_at = ?
			WHERE chat_session_id = ? AND source_revision = ? AND operation = 'upsert'
			  AND status IN ('pending', 'leased', 'retryable', 'needs_embedding')
		`, lifecycleState, reason, now, chatSessionID, revision); err != nil {
			return err
		}
		if lifecycleState == "deleted" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE memory_vector_outbox
				SET document_json = NULL, updated_at = ?
				WHERE chat_session_id = ? AND source_revision = ? AND operation = 'upsert'
			`, now, chatSessionID, revision); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_source_revisions
			SET lifecycle_state = ?, superseded_by_revision = ?,
			    invalidation_reason = ?, invalidated_at = ?, updated_at = ?
			    , raw_user_content = CASE WHEN ? = 'deleted' THEN '' ELSE raw_user_content END
			    , raw_assistant_content = CASE WHEN ? = 'deleted' THEN '' ELSE raw_assistant_content END
			    , source_message_id = CASE WHEN ? = 'deleted' THEN NULL ELSE source_message_id END
			    , source_generation_id = CASE WHEN ? = 'deleted' THEN NULL ELSE source_generation_id END
			    , derived_result_json = CASE WHEN ? = 'deleted' THEN NULL ELSE derived_result_json END
			    , critic_input_snapshot_json = CASE WHEN ? = 'deleted' THEN NULL ELSE critic_input_snapshot_json END
			    , critic_input_snapshot_hash = CASE WHEN ? = 'deleted' THEN NULL ELSE critic_input_snapshot_hash END
			WHERE chat_session_id = ? AND source_revision = ? AND `+sourceStatePredicate+`
		`, lifecycleState, nullableString(supersededByRevision), nullableString(reason),
			now, now, lifecycleState, lifecycleState, lifecycleState,
			lifecycleState, lifecycleState, lifecycleState, lifecycleState,
			chatSessionID, revision); err != nil {
			return err
		}
	}
	return nil
}

func enqueueKnownVectorDeletesTx(ctx context.Context, tx *sql.Tx, sid string, revisions []string, fromTurn int, exactTurn bool, reason string, now time.Time) error {
	if len(revisions) == 0 {
		return nil
	}
	comparison := ">= ?"
	if exactTurn {
		comparison = "= ?"
	}
	type vectorRow struct {
		tier string
		id   int64
	}
	queries := []struct {
		tier  string
		query string
	}{
		{"memory", `SELECT id FROM memories WHERE chat_session_id = ? AND turn_index ` + comparison},
		{"evidence", `SELECT id FROM direct_evidence_records WHERE chat_session_id = ? AND source_turn_end ` + comparison},
		{"world_rule", `SELECT id FROM world_rules WHERE chat_session_id = ? AND source_turn ` + comparison},
	}
	var vectors []vectorRow
	for _, candidate := range queries {
		rows, err := tx.QueryContext(ctx, candidate.query, sid, fromTurn)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			vectors = append(vectors, vectorRow{tier: candidate.tier, id: id})
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	type vectorDelete struct {
		documentID     string
		sourceRevision string
	}
	var deletes []vectorDelete
	deleteIndexes := map[string]int{}
	addDelete := func(documentID, sourceRevision string) {
		documentID = strings.TrimSpace(documentID)
		if documentID == "" {
			return
		}
		if index, exists := deleteIndexes[documentID]; exists {
			deletes[index].sourceRevision = sourceRevision
			return
		}
		deleteIndexes[documentID] = len(deletes)
		deletes = append(deletes, vectorDelete{documentID: documentID, sourceRevision: sourceRevision})
	}
	for _, revision := range revisions {
		rows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT document_id
			FROM memory_vector_outbox
			WHERE chat_session_id = ? AND source_revision = ? AND operation = 'upsert'
			  AND document_id <> ''
			ORDER BY document_id
		`, sid, revision)
		if err != nil {
			return err
		}
		for rows.Next() {
			var documentID string
			if err := rows.Scan(&documentID); err != nil {
				_ = rows.Close()
				return err
			}
			addDelete(documentID, revision)
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	for _, row := range vectors {
		addDelete(fmt.Sprintf("%s:%s:%d", row.tier, sid, row.id), revisions[len(revisions)-1])
		addDelete(fmt.Sprintf("%s:%d", row.tier, row.id), revisions[len(revisions)-1])
	}
	for _, delete := range deletes {
		item := &MemoryVectorOutboxItem{
			ContractVersion:     MemoryVectorOutboxContract,
			OperationKey:        memoryVectorOperationKey("delete:inactive", sid, delete.sourceRevision, delete.documentID),
			Operation:           "delete",
			ChatSessionID:       sid,
			SourceRevision:      delete.sourceRevision,
			DocumentID:          delete.documentID,
			DocumentJSON:        memoryVectorDeleteAuditJSON(reason),
			EmbeddingReady:      true,
			RequiredSourceState: "inactive",
			Status:              "pending",
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if _, err := enqueueMemoryVectorOperation(ctx, tx, item); err != nil {
			return err
		}
	}
	return nil
}

func memoryVectorOperationKey(operation, sid, revision, documentID string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{operation, sid, revision, documentID}, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func memoryVectorDeleteAuditJSON(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	encoded, err := json.Marshal(map[string]string{"delete_reason": reason})
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (m *mariadbStore) EnqueueMemoryReprocessingJob(ctx context.Context, job *MemoryReprocessingJob) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	if job == nil || strings.TrimSpace(job.IdempotencyKey) == "" || strings.TrimSpace(job.ChatSessionID) == "" || strings.TrimSpace(job.SourceRevision) == "" {
		return false, fmt.Errorf("invalid memory reprocessing job")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	if strings.TrimSpace(job.ContractVersion) == "" {
		job.ContractVersion = MemoryReprocessingJobContract
	}
	if strings.TrimSpace(job.Status) == "" {
		job.Status = "pending"
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO memory_reprocessing_jobs (
			contract_version, idempotency_key, chat_session_id, source_revision,
			source_contract, derivation_version, extractor_version, index_version,
			status, attempts, retry_after, lease_owner, lease_until, last_error,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ContractVersion, job.IdempotencyKey, job.ChatSessionID,
		job.SourceRevision, job.SourceContract, job.DerivationVersion,
		job.ExtractorVersion, job.IndexVersion, job.Status, job.Attempts,
		nullableTime(job.RetryAfter), nullableString(job.LeaseOwner),
		nullableTime(job.LeaseUntil), nullableString(job.LastError),
		nonZeroTime(job.CreatedAt), nonZeroTime(job.UpdatedAt))
	if preciseMemoryDuplicateKeyError(err) {
		var sid, revision, sourceContract, derivationVersion, extractorVersion, indexVersion string
		if queryErr := m.db.QueryRowContext(ctx, `
			SELECT chat_session_id, source_revision, source_contract,
			       derivation_version, extractor_version, index_version
			FROM memory_reprocessing_jobs
			WHERE idempotency_key = ?
		`, job.IdempotencyKey).Scan(&sid, &revision, &sourceContract,
			&derivationVersion, &extractorVersion, &indexVersion); queryErr != nil {
			return false, queryErr
		}
		if sid != job.ChatSessionID || revision != job.SourceRevision ||
			sourceContract != job.SourceContract ||
			derivationVersion != job.DerivationVersion ||
			extractorVersion != job.ExtractorVersion ||
			indexVersion != job.IndexVersion {
			return false, fmt.Errorf("memory reprocessing idempotency conflict")
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (m *mariadbStore) ReopenMemoryReprocessingJob(
	ctx context.Context,
	idempotencyKey string,
	chatSessionID string,
	sourceRevision string,
	now time.Time,
) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	chatSessionID = strings.TrimSpace(chatSessionID)
	sourceRevision = strings.TrimSpace(sourceRevision)
	if idempotencyKey == "" || chatSessionID == "" || sourceRevision == "" {
		return false, fmt.Errorf("invalid memory reprocessing reopen request")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var (
		jobID       int64
		sid         string
		revision    string
		status      string
		leaseUntil  sql.NullTime
		sourceState string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT j.id, j.chat_session_id, j.source_revision,
		       j.status, j.lease_until, s.lifecycle_state
		FROM memory_reprocessing_jobs j
		JOIN memory_source_revisions s
		  ON s.chat_session_id = j.chat_session_id
		 AND s.source_revision = j.source_revision
		WHERE j.idempotency_key = ?
		FOR UPDATE
	`, idempotencyKey).Scan(
		&jobID, &sid, &revision, &status, &leaseUntil, &sourceState,
	)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if sid != chatSessionID || revision != sourceRevision {
		return false, fmt.Errorf("memory reprocessing idempotency conflict")
	}
	if sourceState != "active" {
		return false, ErrSourceRevisionStale
	}
	now = nonZeroTime(now)
	if status == "leased" && leaseUntil.Valid && leaseUntil.Time.After(now) {
		return false, ErrMemoryReprocessingLeased
	}

	sourceResult, err := tx.ExecContext(ctx, `
		UPDATE memory_source_revisions
		SET derived_admission_state = 'pending',
		    derived_admission_version = '',
		    derived_extractor_version = '',
		    derived_index_version = '',
		    derived_result_hash = NULL,
		    derived_result_json = NULL,
		    derived_admitted_at = NULL,
		    updated_at = ?
		WHERE chat_session_id = ?
		  AND source_revision = ?
		  AND lifecycle_state = 'active'
	`, now, sid, revision)
	if err != nil {
		return false, err
	}
	if affected, err := sourceResult.RowsAffected(); err != nil {
		return false, err
	} else if affected != 1 {
		return false, ErrSourceRevisionStale
	}
	jobResult, err := tx.ExecContext(ctx, `
		UPDATE memory_reprocessing_jobs
		SET status = 'pending',
		    attempts = 0,
		    retry_after = NULL,
		    lease_owner = NULL,
		    lease_until = NULL,
		    last_error = NULL,
		    updated_at = ?
		WHERE id = ?
		  AND idempotency_key = ?
	`, now, jobID, idempotencyKey)
	if err != nil {
		return false, err
	}
	if affected, err := jobResult.RowsAffected(); err != nil {
		return false, err
	} else if affected != 1 {
		return false, fmt.Errorf("memory reprocessing job reopen lost")
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	committed = true
	return true, nil
}

func (m *mariadbStore) ClaimMemoryReprocessingJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*MemoryReprocessingJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(leaseOwner) == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("invalid memory reprocessing lease")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
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
	now = nonZeroTime(now)
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_reprocessing_jobs j
		JOIN memory_source_revisions s ON s.source_revision = j.source_revision
		SET j.status = 'stale_rejected', j.lease_owner = NULL, j.lease_until = NULL,
		    j.last_error = 'source_revision_not_active', j.updated_at = ?
		WHERE j.status IN ('pending', 'leased', 'retryable')
		  AND s.lifecycle_state <> 'active'
	`, now); err != nil {
		return nil, err
	}
	job, err := selectMemoryReprocessingJobForLease(ctx, tx, now)
	if err != nil {
		return nil, err
	}
	job.LeaseOwner = leaseOwner
	job.LeaseUntil = now.Add(leaseDuration)
	job.Status = "leased"
	job.Attempts++
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_reprocessing_jobs
		SET status = 'leased', attempts = attempts + 1, lease_owner = ?,
		    lease_until = ?, updated_at = ?
		WHERE id = ?
	`, leaseOwner, job.LeaseUntil, now, job.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return job, nil
}

func (m *mariadbStore) NextMemoryReprocessingWakeAt(ctx context.Context) (time.Time, error) {
	if err := m.ensureDB(); err != nil {
		return time.Time{}, err
	}
	var next sql.NullTime
	err := m.db.QueryRowContext(ctx, `
		SELECT MIN(CASE
		         WHEN j.status = 'leased' THEN j.lease_until
		         ELSE j.retry_after
		       END)
		FROM memory_reprocessing_jobs j
		JOIN memory_source_revisions s ON s.source_revision = j.source_revision
		WHERE s.lifecycle_state = 'active'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM session_migration_locks migration_lock
		    WHERE migration_lock.source_session_id = j.chat_session_id
		      AND migration_lock.locked = TRUE
		      AND migration_lock.unlocked_at IS NULL
		  )
		  AND (
		    (j.status IN ('pending', 'retryable') AND j.retry_after IS NOT NULL)
		    OR (j.status = 'leased' AND j.lease_until IS NOT NULL)
		  )
	`).Scan(&next)
	if err != nil {
		return time.Time{}, err
	}
	if !next.Valid || next.Time.IsZero() {
		return time.Time{}, ErrNotFound
	}
	return next.Time, nil
}

// selectMemoryReprocessingJobForLease treats retry_after as an exclusive wake
// cursor so a job failed in this wake cannot be reclaimed by the same drain.
func selectMemoryReprocessingJobForLease(ctx context.Context, tx *sql.Tx, now time.Time) (*MemoryReprocessingJob, error) {
	job := &MemoryReprocessingJob{}
	var retryAfter, leaseUntil sql.NullTime
	var leaseOwner, lastError sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT j.id, j.contract_version, j.idempotency_key, j.chat_session_id,
		       j.source_revision, j.source_contract, j.derivation_version,
		       j.extractor_version, j.index_version, j.status, j.attempts,
		       j.retry_after, j.lease_owner, j.lease_until, j.last_error,
		       j.created_at, j.updated_at
		FROM memory_reprocessing_jobs j
		JOIN memory_source_revisions s ON s.source_revision = j.source_revision
		WHERE s.lifecycle_state = 'active'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM session_migration_locks migration_lock
		    WHERE migration_lock.source_session_id = j.chat_session_id
		      AND migration_lock.locked = TRUE
		      AND migration_lock.unlocked_at IS NULL
		  )
		  AND (
		    (j.status IN ('pending', 'retryable') AND (j.retry_after IS NULL OR j.retry_after < ?))
		    OR (j.status = 'leased' AND j.lease_until < ?)
		  )
		ORDER BY j.created_at, j.id
		LIMIT 1 FOR UPDATE
	`, now, now).Scan(&job.ID, &job.ContractVersion, &job.IdempotencyKey,
		&job.ChatSessionID, &job.SourceRevision, &job.SourceContract,
		&job.DerivationVersion, &job.ExtractorVersion, &job.IndexVersion,
		&job.Status, &job.Attempts, &retryAfter, &leaseOwner, &leaseUntil,
		&lastError, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	job.RetryAfter = retryAfter.Time
	job.LeaseOwner = leaseOwner.String
	job.LeaseUntil = leaseUntil.Time
	job.LastError = lastError.String
	return job, nil
}

func (m *mariadbStore) CompleteMemoryReprocessingJob(ctx context.Context, jobID int64, leaseOwner string, now time.Time) error {
	return m.finishMemoryReprocessingJob(ctx, jobID, leaseOwner, now, time.Time{}, false, false, "")
}

func (m *mariadbStore) FailMemoryReprocessingJob(ctx context.Context, jobID int64, leaseOwner string, now, retryAfter time.Time, permanent bool, failure string) error {
	return m.finishMemoryReprocessingJob(ctx, jobID, leaseOwner, now, retryAfter, true, permanent, failure)
}

func (m *mariadbStore) finishMemoryReprocessingJob(ctx context.Context, jobID int64, leaseOwner string, now, retryAfter time.Time, failed, permanent bool, failure string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now = nonZeroTime(now)
	var currentOwner, sourceState string
	var leaseUntil time.Time
	if err := tx.QueryRowContext(ctx, `
		SELECT j.lease_owner, j.lease_until, s.lifecycle_state
		FROM memory_reprocessing_jobs j
		JOIN memory_source_revisions s ON s.source_revision = j.source_revision
		WHERE j.id = ? AND j.status = 'leased'
		FOR UPDATE
	`, jobID).Scan(&currentOwner, &leaseUntil, &sourceState); err != nil {
		if err == sql.ErrNoRows {
			return ErrLeaseExpired
		}
		return err
	}
	if currentOwner != leaseOwner || leaseUntil.Before(now) {
		return ErrLeaseExpired
	}
	if sourceState != "active" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_reprocessing_jobs
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    last_error = 'source_revision_not_active', updated_at = ?
			WHERE id = ?
		`, now, jobID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return ErrSourceRevisionStale
	}
	status := "completed"
	if failed {
		status = "retryable"
		if permanent {
			status = "permanent"
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_reprocessing_jobs
		SET status = ?, retry_after = ?, lease_owner = NULL, lease_until = NULL,
		    last_error = ?, updated_at = ?
		WHERE id = ?
	`, status, nullableTime(retryAfter), nullableString(failure), now, jobID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *mariadbStore) EnqueueMemoryVectorOperation(ctx context.Context, item *MemoryVectorOutboxItem) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	return enqueueMemoryVectorOperation(ctx, m.db, item)
}

func enqueueMemoryVectorOperation(ctx context.Context, exec memoryDerivationSQLExecutor, item *MemoryVectorOutboxItem) (bool, error) {
	if item == nil || strings.TrimSpace(item.OperationKey) == "" ||
		strings.TrimSpace(item.ChatSessionID) == "" ||
		strings.TrimSpace(item.SourceRevision) == "" ||
		strings.TrimSpace(item.DocumentID) == "" {
		return false, fmt.Errorf("invalid memory vector outbox item")
	}
	if strings.TrimSpace(item.ContractVersion) == "" {
		item.ContractVersion = MemoryVectorOutboxContract
	}
	if item.Operation != "upsert" && item.Operation != "delete" {
		return false, fmt.Errorf("invalid vector outbox operation %q", item.Operation)
	}
	if strings.TrimSpace(item.RequiredSourceState) == "" {
		if item.Operation == "upsert" {
			item.RequiredSourceState = "active"
		} else {
			item.RequiredSourceState = "inactive"
		}
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "pending"
	}
	documentJSON := strings.TrimSpace(item.DocumentJSON)
	if documentJSON != "" && !json.Valid([]byte(documentJSON)) {
		return false, fmt.Errorf("invalid memory vector document JSON")
	}
	if item.Operation == "upsert" && documentJSON == "" {
		return false, fmt.Errorf("memory vector upsert document is required")
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO memory_vector_outbox (
			contract_version, operation_key, operation, chat_session_id,
			source_revision, document_id, document_json, embedding_ready,
			required_source_state, status, attempts, retry_after, lease_owner,
			lease_until, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ContractVersion, item.OperationKey, item.Operation,
		item.ChatSessionID, item.SourceRevision, item.DocumentID,
		nullableString(documentJSON), item.EmbeddingReady,
		item.RequiredSourceState, item.Status, item.Attempts,
		nullableTime(item.RetryAfter), nullableString(item.LeaseOwner),
		nullableTime(item.LeaseUntil), nullableString(item.LastError),
		nonZeroTime(item.CreatedAt), nonZeroTime(item.UpdatedAt))
	if preciseMemoryDuplicateKeyError(err) {
		var operation, sid, revision, documentID, existingJSON, requiredSourceState string
		var embeddingReady bool
		if queryErr := exec.QueryRowContext(ctx, `
			SELECT operation, chat_session_id, source_revision, document_id,
			       COALESCE(document_json, ''), embedding_ready, required_source_state
			FROM memory_vector_outbox
			WHERE operation_key = ?
		`, item.OperationKey).Scan(&operation, &sid, &revision, &documentID,
			&existingJSON, &embeddingReady, &requiredSourceState); queryErr != nil {
			return false, queryErr
		}
		if operation != item.Operation || sid != item.ChatSessionID ||
			revision != item.SourceRevision || documentID != item.DocumentID ||
			(operation == "upsert" && strings.TrimSpace(existingJSON) != documentJSON) ||
			embeddingReady != item.EmbeddingReady ||
			requiredSourceState != item.RequiredSourceState {
			return false, fmt.Errorf("memory vector operation idempotency conflict")
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (m *mariadbStore) ClaimMemoryVectorOperations(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) ([]*MemoryVectorOutboxItem, error) {
	return m.claimMemoryVectorOperations(ctx, leaseOwner, now, leaseDuration, "")
}

func (m *mariadbStore) ClaimMemoryVectorOperationsByOperation(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration, operation string) ([]*MemoryVectorOutboxItem, error) {
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "upsert" && operation != "delete" {
		return nil, fmt.Errorf("invalid vector outbox operation lane")
	}
	return m.claimMemoryVectorOperations(ctx, leaseOwner, now, leaseDuration, operation)
}

func (m *mariadbStore) claimMemoryVectorOperations(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration, operation string) ([]*MemoryVectorOutboxItem, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(leaseOwner) == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("invalid vector outbox lease")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
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
	now = nonZeroTime(now)
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		SET o.status = 'stale_rejected', o.lease_owner = NULL, o.lease_until = NULL,
		    o.last_error = 'source_revision_fence_rejected', o.updated_at = ?
		WHERE o.status IN ('pending', 'leased', 'retryable', 'needs_embedding')
		  AND (
		    (o.required_source_state = 'active' AND s.lifecycle_state <> 'active')
		    OR (o.required_source_state = 'inactive' AND s.lifecycle_state = 'active')
		  )
	`, now); err != nil {
		return nil, err
	}
	item, err := selectMemoryVectorOperationForLease(ctx, tx, now, operation)
	if err != nil {
		return nil, err
	}
	items := []*MemoryVectorOutboxItem{item}
	if item.Operation == "delete" || (item.Operation == "upsert" && !item.EmbeddingReady) {
		siblings, err := selectMemoryVectorOperationSiblingsForLease(ctx, tx, item, now)
		if err != nil {
			return nil, err
		}
		items = append(items, siblings...)
	}
	leaseUntil := now.Add(leaseDuration)
	for _, claimed := range items {
		claimed.LeaseOwner = leaseOwner
		claimed.LeaseUntil = leaseUntil
		claimed.Status = "leased"
		claimed.Attempts++
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_vector_outbox
			SET status = 'leased', attempts = attempts + 1, lease_owner = ?,
			    lease_until = ?, updated_at = ?
			WHERE id = ?
		`, leaseOwner, claimed.LeaseUntil, now, claimed.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return items, nil
}

func selectMemoryVectorOperationForLease(ctx context.Context, tx *sql.Tx, now time.Time, operation string) (*MemoryVectorOutboxItem, error) {
	operationClause := ""
	args := []any{now, now, now}
	if operation != "" {
		operationClause = " AND o.operation = ?"
		args = append(args, operation)
	}
	row := tx.QueryRowContext(ctx, `
		SELECT o.id, o.contract_version, o.operation_key, o.operation,
		       o.chat_session_id, o.source_revision, o.document_id,
		       o.document_json, o.embedding_ready, o.required_source_state,
		       o.status, o.attempts, o.retry_after, o.lease_owner,
		       o.lease_until, o.last_error, o.created_at, o.updated_at
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		WHERE (
		    (
		      o.embedding_ready = TRUE
		      AND o.status IN ('pending', 'retryable')
		      AND (o.retry_after IS NULL OR o.retry_after < ?)
		    )
		    OR (
		      o.operation = 'upsert'
		      AND o.embedding_ready = FALSE
		      AND o.status IN ('needs_embedding', 'retryable')
		      AND (o.retry_after IS NULL OR o.retry_after < ?)
		    )
		    OR (o.status = 'leased' AND o.lease_until < ?)
		  )
		  AND (
		    (o.required_source_state = 'active' AND s.lifecycle_state = 'active')
		    OR (o.required_source_state = 'inactive' AND s.lifecycle_state <> 'active')
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM session_migration_locks migration_lock
		    WHERE migration_lock.source_session_id = o.chat_session_id
		      AND migration_lock.locked = TRUE
		      AND migration_lock.unlocked_at IS NULL
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM memory_vector_outbox prior
		    WHERE prior.chat_session_id = o.chat_session_id
		      AND prior.document_id = o.document_id
		      AND prior.id < o.id
		      AND prior.status IN ('pending', 'leased', 'retryable', 'needs_embedding')
		  )`+operationClause+`
		ORDER BY o.created_at, o.id
		LIMIT 1 FOR UPDATE
	`, args...)
	item, err := scanMemoryVectorOutboxItem(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func selectMemoryVectorOperationSiblingsForLease(ctx context.Context, tx *sql.Tx, seed *MemoryVectorOutboxItem, now time.Time) ([]*MemoryVectorOutboxItem, error) {
	where := `o.source_revision = ?
		  AND o.chat_session_id = ?
		  AND o.id <> ?
		  AND o.operation = 'upsert'
		  AND o.embedding_ready = FALSE
		  AND (
		    (
		      o.status IN ('needs_embedding', 'retryable')
		      AND (o.retry_after IS NULL OR o.retry_after < ?)
		    )
		    OR (o.status = 'leased' AND o.lease_until < ?)
		  )
		  AND o.required_source_state = 'active'
		  AND s.lifecycle_state = 'active'`
	args := []any{seed.SourceRevision, seed.ChatSessionID, seed.ID, now, now}
	limitClause := " LIMIT ?"
	args = append(args, memoryVectorUpsertClaimBatchSize-1)
	if seed.Operation == "delete" {
		where = `o.chat_session_id = ?
		  AND o.id <> ?
		  AND o.operation = 'delete'
		  AND o.embedding_ready = TRUE
		  AND (
		    (
		      o.status IN ('pending', 'retryable')
		      AND (o.retry_after IS NULL OR o.retry_after < ?)
		    )
		    OR (o.status = 'leased' AND o.lease_until < ?)
		  )
		  AND o.required_source_state = 'inactive'
		  AND s.lifecycle_state <> 'active'`
		limitClause = " LIMIT ?"
		args = []any{seed.ChatSessionID, seed.ID, now, now, memoryVectorDeleteClaimBatchSize - 1}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT o.id, o.contract_version, o.operation_key, o.operation,
		       o.chat_session_id, o.source_revision, o.document_id,
		       o.document_json, o.embedding_ready, o.required_source_state,
		       o.status, o.attempts, o.retry_after, o.lease_owner,
		       o.lease_until, o.last_error, o.created_at, o.updated_at
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		WHERE `+where+`
		  AND NOT EXISTS (
		    SELECT 1
		    FROM session_migration_locks migration_lock
		    WHERE migration_lock.source_session_id = o.chat_session_id
		      AND migration_lock.locked = TRUE
		      AND migration_lock.unlocked_at IS NULL
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM memory_vector_outbox prior
		    WHERE prior.chat_session_id = o.chat_session_id
		      AND prior.document_id = o.document_id
		      AND prior.id < o.id
		      AND prior.status IN ('pending', 'leased', 'retryable', 'needs_embedding')
		  )
		ORDER BY o.created_at, o.id`+limitClause+`
		FOR UPDATE
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*MemoryVectorOutboxItem{}
	for rows.Next() {
		item, err := scanMemoryVectorOutboxItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *mariadbStore) CoalesceInactiveMemoryVectorDeleteOperations(
	ctx context.Context,
	chatSessionID string,
	now time.Time,
) (int64, error) {
	if err := m.ensureDB(); err != nil {
		return 0, err
	}
	chatSessionID = strings.TrimSpace(chatSessionID)
	if chatSessionID == "" {
		return 0, fmt.Errorf("chat session id is required")
	}
	now = nonZeroTime(now)
	var total, afterID int64
	for {
		staleRejected, err := m.coalesceInactiveMemoryVectorDeleteBatch(ctx, chatSessionID, now, &afterID)
		if err != nil {
			return total, err
		}
		total += staleRejected
		if staleRejected < memoryVectorDeleteCoalesceBatchSize {
			return total, nil
		}
	}
}

func (m *mariadbStore) coalesceInactiveMemoryVectorDeleteBatch(
	ctx context.Context,
	chatSessionID string,
	now time.Time,
	afterID *int64,
) (int64, error) {
	// Each batch keeps the existing duplicate/lease rules, but releases the
	// writer between commits so a large historical backlog cannot monopolize it.
	// The indexed join and scalar MIN avoid MariaDB's EXISTS semijoin plan,
	// which otherwise scans/sorts the historical outbox again for every batch.
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	rows, err := tx.QueryContext(ctx, `
		SELECT o.id
		FROM memory_vector_outbox o FORCE INDEX (PRIMARY)
		STRAIGHT_JOIN memory_source_revisions s FORCE INDEX (uq_memory_source_revision)
		  ON s.chat_session_id = o.chat_session_id
		 AND s.source_revision = o.source_revision
		WHERE o.chat_session_id = ? AND o.id > ?
		  AND o.operation = 'delete'
		  AND o.required_source_state = 'inactive'
		  AND s.lifecycle_state <> 'active'
		  AND o.status IN ('pending', 'retryable', 'leased', 'needs_embedding')
		  AND (o.status <> 'leased' OR o.lease_until IS NULL OR o.lease_until <= ?)
		  AND NOT EXISTS (
		    SELECT 1
		    FROM memory_vector_outbox active_lease
		    WHERE active_lease.chat_session_id = o.chat_session_id
		      AND active_lease.source_revision = o.source_revision
		      AND active_lease.document_id = o.document_id
		      AND active_lease.operation = 'delete'
		      AND active_lease.required_source_state = 'inactive'
		      AND active_lease.status = 'leased'
		      AND active_lease.lease_until > ?
		  )
		  AND (
		    SELECT MIN(keep.id)
		    FROM memory_vector_outbox keep FORCE INDEX (idx_memory_vector_document)
		    WHERE keep.chat_session_id = o.chat_session_id
		      AND keep.source_revision = o.source_revision
		      AND keep.document_id = o.document_id
		      AND keep.operation = 'delete'
		      AND keep.required_source_state = 'inactive'
		      AND (
		        (keep.status IN ('pending', 'retryable') AND (keep.retry_after IS NULL OR keep.retry_after < ?))
		        OR (keep.status = 'leased' AND keep.lease_until < ?)
		      )
		      AND (
		        o.status = 'needs_embedding'
		        OR (o.status IN ('pending', 'retryable') AND o.retry_after IS NOT NULL AND o.retry_after >= ?)
		        OR (o.status = 'leased' AND (o.lease_until IS NULL OR o.lease_until >= ?))
		        OR keep.created_at < o.created_at
		        OR (keep.created_at = o.created_at AND keep.id < o.id)
		      )
		  ) IS NOT NULL
		ORDER BY o.id
		LIMIT ?
		FOR UPDATE
	`, chatSessionID, *afterID, now, now, now, now, now, now, memoryVectorDeleteCoalesceBatchSize)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0, memoryVectorDeleteCoalesceBatchSize)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		committed = true
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+3)
	args = append(args, "duplicate_delete_operation_key_4_0_4", now)
	for index, id := range ids {
		placeholders[index] = "?"
		args = append(args, id)
	}
	args = append(args, now)
	result, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox
		SET status = 'stale_rejected', retry_after = NULL,
		    lease_owner = NULL, lease_until = NULL, last_error = ?, updated_at = ?
		WHERE id IN (`+strings.Join(placeholders, ",")+`)
		  AND status IN ('pending', 'retryable', 'leased', 'needs_embedding')
		  AND (status <> 'leased' OR lease_until IS NULL OR lease_until <= ?)
	`, args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	// Re-scanning completed batches makes old duplicate cleanup quadratic.
	*afterID = ids[len(ids)-1]
	return affected, nil
}

type memoryVectorOutboxScanner interface {
	Scan(...any) error
}

func scanMemoryVectorOutboxItem(scanner memoryVectorOutboxScanner) (*MemoryVectorOutboxItem, error) {
	item := &MemoryVectorOutboxItem{}
	var documentJSON, leaseOwner, lastError sql.NullString
	var retryAfter, leaseUntil sql.NullTime
	if err := scanner.Scan(&item.ID, &item.ContractVersion, &item.OperationKey,
		&item.Operation, &item.ChatSessionID, &item.SourceRevision,
		&item.DocumentID, &documentJSON, &item.EmbeddingReady,
		&item.RequiredSourceState, &item.Status, &item.Attempts,
		&retryAfter, &leaseOwner, &leaseUntil, &lastError,
		&item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.DocumentJSON = documentJSON.String
	item.RetryAfter = retryAfter.Time
	item.LeaseOwner = leaseOwner.String
	item.LeaseUntil = leaseUntil.Time
	item.LastError = lastError.String
	return item, nil
}

func (m *mariadbStore) CompleteMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now time.Time) error {
	return m.finishMemoryVectorOperation(ctx, outboxID, leaseOwner, now, time.Time{}, false, false, "")
}

func (m *mariadbStore) CompleteMemoryVectorMaterializedOperation(
	ctx context.Context,
	outboxID int64,
	leaseOwner string,
	now time.Time,
	materialization MemoryVectorMaterialization,
) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	if strings.TrimSpace(materialization.ChatSessionID) == "" ||
		strings.TrimSpace(materialization.SourceRevision) == "" ||
		strings.TrimSpace(materialization.DocumentID) == "" ||
		materialization.SourceRowID <= 0 ||
		strings.TrimSpace(materialization.EmbeddingModel) == "" {
		return fmt.Errorf("invalid memory vector materialization")
	}
	var embedding []float64
	if err := json.Unmarshal([]byte(strings.TrimSpace(materialization.EmbeddingJSON)), &embedding); err != nil || len(embedding) == 0 {
		return fmt.Errorf("invalid memory vector materialization embedding")
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now = nonZeroTime(now)
	var currentStatus, currentOperation, currentSID, currentRevision, currentDocumentID string
	var requiredState, sourceState string
	var sourceTurn int
	var currentOwner sql.NullString
	var leaseUntil sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT o.status, o.operation, o.chat_session_id, o.source_revision,
		       o.document_id, o.lease_owner, o.lease_until,
		       o.required_source_state, s.lifecycle_state, s.turn_index
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s
		  ON s.chat_session_id = o.chat_session_id
		 AND s.source_revision = o.source_revision
		WHERE o.id = ?
		FOR UPDATE
	`, outboxID).Scan(
		&currentStatus, &currentOperation, &currentSID, &currentRevision,
		&currentDocumentID, &currentOwner, &leaseUntil,
		&requiredState, &sourceState, &sourceTurn,
	); err != nil {
		if err == sql.ErrNoRows {
			return ErrLeaseExpired
		}
		return err
	}
	if currentStatus == "stale_rejected" {
		return ErrSourceRevisionStale
	}
	if currentStatus != "leased" || currentOperation != "upsert" ||
		!leaseUntil.Valid || currentOwner.String != leaseOwner || leaseUntil.Time.Before(now) {
		return ErrLeaseExpired
	}
	if currentSID != strings.TrimSpace(materialization.ChatSessionID) ||
		currentRevision != strings.TrimSpace(materialization.SourceRevision) ||
		currentDocumentID != strings.TrimSpace(materialization.DocumentID) {
		return fmt.Errorf("memory vector materialization identity mismatch")
	}
	if requiredState != "active" || sourceState != "active" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_vector_outbox
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    last_error = 'source_revision_fence_rejected', updated_at = ?
			WHERE id = ?
		`, now, outboxID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return ErrSourceRevisionStale
	}
	var memoryTurn int
	if err := tx.QueryRowContext(ctx, `
		SELECT turn_index
		FROM memories
		WHERE id = ? AND chat_session_id = ?
		FOR UPDATE
	`, materialization.SourceRowID, currentSID).Scan(&memoryTurn); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("materialized memory row is missing")
		}
		return err
	}
	if memoryTurn != sourceTurn {
		return fmt.Errorf("materialized memory row source turn mismatch")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memories
		SET embedding = ?, embedding_model = ?
		WHERE id = ? AND chat_session_id = ?
	`, materialization.EmbeddingJSON, materialization.EmbeddingModel,
		materialization.SourceRowID, currentSID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox
		SET status = 'completed', retry_after = NULL,
		    lease_owner = NULL, lease_until = NULL, last_error = NULL, updated_at = ?
		WHERE id = ?
	`, now, outboxID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *mariadbStore) FailMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now, retryAfter time.Time, permanent bool, failure string) error {
	return m.finishMemoryVectorOperation(ctx, outboxID, leaseOwner, now, retryAfter, true, permanent, failure)
}

func (m *mariadbStore) finishMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now, retryAfter time.Time, failed, permanent bool, failure string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
	m.memoryDerivationWriteMu.Lock()
	defer m.memoryDerivationWriteMu.Unlock()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now = nonZeroTime(now)
	var currentStatus, requiredState, sourceState string
	var currentOwner sql.NullString
	var leaseUntil sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT o.status, o.lease_owner, o.lease_until, o.required_source_state,
		       s.lifecycle_state
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		WHERE o.id = ?
		FOR UPDATE
	`, outboxID).Scan(&currentStatus, &currentOwner, &leaseUntil, &requiredState, &sourceState); err != nil {
		if err == sql.ErrNoRows {
			return ErrLeaseExpired
		}
		return err
	}
	if currentStatus == "stale_rejected" {
		return ErrSourceRevisionStale
	}
	if currentStatus != "leased" || !leaseUntil.Valid || currentOwner.String != leaseOwner || leaseUntil.Time.Before(now) {
		return ErrLeaseExpired
	}
	sourceFenceSatisfied := (requiredState == "active" && sourceState == "active") ||
		(requiredState == "inactive" && sourceState != "active")
	if !sourceFenceSatisfied {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memory_vector_outbox
			SET status = 'stale_rejected', lease_owner = NULL, lease_until = NULL,
			    last_error = 'source_revision_fence_rejected', updated_at = ?
			WHERE id = ?
		`, now, outboxID); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return ErrSourceRevisionStale
	}
	status := "completed"
	if failed {
		status = "retryable"
		if permanent {
			status = "permanent"
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox
		SET status = ?, retry_after = ?, lease_owner = NULL, lease_until = NULL,
		    last_error = ?, updated_at = ?
		WHERE id = ?
	`, status, nullableTime(retryAfter), nullableString(failure), now, outboxID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
