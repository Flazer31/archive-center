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

var _ SourceRevisionStore = (*mariadbStore)(nil)
var _ MemoryDerivationLifecycleAvailability = (*mariadbStore)(nil)
var _ MemoryReprocessingJobStore = (*mariadbStore)(nil)
var _ MemoryVectorOutboxStore = (*mariadbStore)(nil)

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

	var activeRevision, activeHash, activeUser, activeAssistant string
	err = tx.QueryRowContext(ctx, `
		SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content
		FROM memory_source_revisions
		WHERE chat_session_id = ? AND logical_turn_id = ? AND lifecycle_state = 'active'
		ORDER BY host_observed_at_ms DESC, id DESC
		LIMIT 1 FOR UPDATE
	`, source.ChatSessionID, source.LogicalTurnID).Scan(&activeRevision, &activeHash, &activeUser, &activeAssistant)
	switch {
	case err == nil && activeRevision == source.SourceRevision:
		if activeHash != source.CombinedContentHash || activeUser != source.UserContent || activeAssistant != source.AssistantContent {
			return result, ErrSourceRevisionConflict
		}
		result.Idempotent = true
	case err == nil:
		return result, ErrSourceRevisionConflict
	case err != sql.ErrNoRows:
		return result, err
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
		strings.TrimSpace(source.UserContent) == "" ||
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
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, source.ContractVersion, source.SourceRevision, source.ChatSessionID,
		source.LogicalTurnID, source.TurnIndex, nullableString(source.SourceMessageID),
		nullableString(source.SourceGenerationID), nullableString(source.BranchID),
		source.BranchState, source.UserContent, source.AssistantContent,
		source.CombinedContentHash, nullableString(source.UserObservedContentHash),
		nullableString(source.AssistantObservedContentHash), source.HashAlgorithm,
		source.HostObservedAtMS, source.LifecycleState,
		nullableString(source.SupersededByRevision),
		nullableString(source.InvalidationReason), createdAt, updatedAt)
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
	err := m.db.QueryRowContext(ctx, `
		SELECT id, contract_version, source_revision, chat_session_id,
		       logical_turn_id, turn_index, source_message_id,
		       source_generation_id, branch_id, branch_state,
		       raw_user_content, raw_assistant_content, combined_content_hash,
		       user_observed_content_hash, assistant_observed_content_hash,
		       hash_algorithm, host_observed_at_ms, lifecycle_state,
		       superseded_by_revision, invalidation_reason, created_at, updated_at
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
	return source, nil
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
	for _, revision := range revisions {
		if err := enqueueKnownVectorDeletesTx(ctx, tx, chatSessionID, revision, fromTurn, exactTurn, now); err != nil {
			return err
		}
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
			WHERE chat_session_id = ? AND source_revision = ? AND `+sourceStatePredicate+`
		`, lifecycleState, nullableString(supersededByRevision), nullableString(reason),
			now, now, lifecycleState, lifecycleState, lifecycleState,
			lifecycleState, chatSessionID, revision); err != nil {
			return err
		}
	}
	return nil
}

func enqueueKnownVectorDeletesTx(ctx context.Context, tx *sql.Tx, sid, revision string, fromTurn int, exactTurn bool, now time.Time) error {
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
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT document_id
		FROM memory_vector_outbox
		WHERE chat_session_id = ? AND source_revision = ? AND operation = 'upsert'
		  AND document_id <> ''
	`, sid, revision)
	if err != nil {
		return err
	}
	var documentIDs []string
	for rows.Next() {
		var documentID string
		if err := rows.Scan(&documentID); err != nil {
			_ = rows.Close()
			return err
		}
		documentIDs = append(documentIDs, documentID)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, row := range vectors {
		documentIDs = append(documentIDs,
			fmt.Sprintf("%s:%s:%d", row.tier, sid, row.id),
			fmt.Sprintf("%s:%d", row.tier, row.id))
	}
	seen := map[string]bool{}
	for _, documentID := range documentIDs {
		documentID = strings.TrimSpace(documentID)
		if documentID == "" || seen[documentID] {
			continue
		}
		seen[documentID] = true
		item := &MemoryVectorOutboxItem{
			ContractVersion:     MemoryVectorOutboxContract,
			OperationKey:        memoryVectorOperationKey("delete", sid, revision, documentID),
			Operation:           "delete",
			ChatSessionID:       sid,
			SourceRevision:      revision,
			DocumentID:          documentID,
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

func (m *mariadbStore) EnqueueMemoryReprocessingJob(ctx context.Context, job *MemoryReprocessingJob) (bool, error) {
	if err := m.ensureDB(); err != nil {
		return false, err
	}
	if job == nil || strings.TrimSpace(job.IdempotencyKey) == "" || strings.TrimSpace(job.ChatSessionID) == "" || strings.TrimSpace(job.SourceRevision) == "" {
		return false, fmt.Errorf("invalid memory reprocessing job")
	}
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

func (m *mariadbStore) ClaimMemoryReprocessingJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*MemoryReprocessingJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(leaseOwner) == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("invalid memory reprocessing lease")
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
		  AND (
		    (j.status IN ('pending', 'retryable') AND (j.retry_after IS NULL OR j.retry_after <= ?))
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
			strings.TrimSpace(existingJSON) != documentJSON ||
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

func (m *mariadbStore) ClaimMemoryVectorOperation(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*MemoryVectorOutboxItem, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(leaseOwner) == "" || leaseDuration <= 0 {
		return nil, fmt.Errorf("invalid vector outbox lease")
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
	now = nonZeroTime(now)
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		SET o.status = 'stale_rejected', o.lease_owner = NULL, o.lease_until = NULL,
		    o.last_error = 'source_revision_fence_rejected', o.updated_at = ?
		WHERE o.status IN ('pending', 'leased', 'retryable')
		  AND (
		    (o.required_source_state = 'active' AND s.lifecycle_state <> 'active')
		    OR (o.required_source_state = 'inactive' AND s.lifecycle_state = 'active')
		  )
	`, now); err != nil {
		return nil, err
	}
	item, err := selectMemoryVectorOperationForLease(ctx, tx, now)
	if err != nil {
		return nil, err
	}
	item.LeaseOwner = leaseOwner
	item.LeaseUntil = now.Add(leaseDuration)
	item.Status = "leased"
	item.Attempts++
	if _, err := tx.ExecContext(ctx, `
		UPDATE memory_vector_outbox
		SET status = 'leased', attempts = attempts + 1, lease_owner = ?,
		    lease_until = ?, updated_at = ?
		WHERE id = ?
	`, leaseOwner, item.LeaseUntil, now, item.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	return item, nil
}

func selectMemoryVectorOperationForLease(ctx context.Context, tx *sql.Tx, now time.Time) (*MemoryVectorOutboxItem, error) {
	item := &MemoryVectorOutboxItem{}
	var documentJSON, leaseOwner, lastError sql.NullString
	var retryAfter, leaseUntil sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT o.id, o.contract_version, o.operation_key, o.operation,
		       o.chat_session_id, o.source_revision, o.document_id,
		       o.document_json, o.embedding_ready, o.required_source_state,
		       o.status, o.attempts, o.retry_after, o.lease_owner,
		       o.lease_until, o.last_error, o.created_at, o.updated_at
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		WHERE o.embedding_ready = TRUE
		  AND (
		    (o.status IN ('pending', 'retryable') AND (o.retry_after IS NULL OR o.retry_after <= ?))
		    OR (o.status = 'leased' AND o.lease_until < ?)
		  )
		  AND (
		    (o.required_source_state = 'active' AND s.lifecycle_state = 'active')
		    OR (o.required_source_state = 'inactive' AND s.lifecycle_state <> 'active')
		  )
		ORDER BY o.created_at, o.id
		LIMIT 1 FOR UPDATE
	`, now, now).Scan(&item.ID, &item.ContractVersion, &item.OperationKey,
		&item.Operation, &item.ChatSessionID, &item.SourceRevision,
		&item.DocumentID, &documentJSON, &item.EmbeddingReady,
		&item.RequiredSourceState, &item.Status, &item.Attempts,
		&retryAfter, &leaseOwner, &leaseUntil, &lastError,
		&item.CreatedAt, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
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

func (m *mariadbStore) FailMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now, retryAfter time.Time, permanent bool, failure string) error {
	return m.finishMemoryVectorOperation(ctx, outboxID, leaseOwner, now, retryAfter, true, permanent, failure)
}

func (m *mariadbStore) finishMemoryVectorOperation(ctx context.Context, outboxID int64, leaseOwner string, now, retryAfter time.Time, failed, permanent bool, failure string) error {
	if err := m.ensureDB(); err != nil {
		return err
	}
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
	var currentOwner, requiredState, sourceState string
	var leaseUntil time.Time
	if err := tx.QueryRowContext(ctx, `
		SELECT o.lease_owner, o.lease_until, o.required_source_state, s.lifecycle_state
		FROM memory_vector_outbox o
		JOIN memory_source_revisions s ON s.source_revision = o.source_revision
		WHERE o.id = ? AND o.status = 'leased'
		FOR UPDATE
	`, outboxID).Scan(&currentOwner, &leaseUntil, &requiredState, &sourceState); err != nil {
		if err == sql.ErrNoRows {
			return ErrLeaseExpired
		}
		return err
	}
	if currentOwner != leaseOwner || leaseUntil.Before(now) {
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
