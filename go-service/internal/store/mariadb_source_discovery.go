package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

func (m *mariadbStore) SaveSourceDiscoveryJob(ctx context.Context, input SourceDiscoveryInput, state string, result, coverage map[string]any) (*SourceDiscoveryJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.WorkQuery) == "" || !sourceDiscoveryState(state) {
		return nil, ErrInvalidReference
	}
	jobID, err := canonRandomID()
	if err != nil {
		return nil, err
	}
	inputJSON, _ := json.Marshal(input)
	resultJSON, _ := json.Marshal(result)
	coverageJSON, _ := json.Marshal(coverage)
	if _, err := m.db.ExecContext(ctx, `
		INSERT INTO source_discovery_jobs
			(job_id, contract_version, work_query, original_title, language_code,
			 edition_hint, job_state, request_json, result_json, coverage_report_json)
		VALUES (?, 'source-discovery-pipeline.v1', ?, ?, ?, ?, ?, ?, ?, ?)
	`, jobID, strings.TrimSpace(input.WorkQuery), strings.TrimSpace(input.OriginalTitle),
		strings.TrimSpace(input.Language), strings.TrimSpace(input.EditionHint), state,
		string(inputJSON), string(resultJSON), string(coverageJSON)); err != nil {
		return nil, referenceStoreError(err)
	}
	return m.GetSourceDiscoveryJob(ctx, jobID)
}

func (m *mariadbStore) GetSourceDiscoveryJob(ctx context.Context, jobID string) (*SourceDiscoveryJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	item := &SourceDiscoveryJob{Contract: SourceDiscoveryContract}
	var contract string
	var inputRaw, resultRaw, coverageRaw []byte
	err := m.db.QueryRowContext(ctx, `
		SELECT job_id, contract_version, job_state, request_json, result_json,
		       coverage_report_json, revision, created_at, updated_at
		FROM source_discovery_jobs WHERE job_id=?
	`, strings.TrimSpace(jobID)).Scan(&item.JobID, &contract, &item.State, &inputRaw,
		&resultRaw, &coverageRaw, &item.Revision, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.Contract = contract
	_ = json.Unmarshal(inputRaw, &item.Input)
	_ = json.Unmarshal(resultRaw, &item.Result)
	_ = json.Unmarshal(coverageRaw, &item.CoverageReport)
	return item, nil
}

func (m *mariadbStore) UpdateSourceDiscoveryJob(ctx context.Context, jobID, state string, result, coverage map[string]any) (*SourceDiscoveryJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || !sourceDiscoveryState(state) {
		return nil, ErrInvalidReference
	}
	resultJSON, _ := json.Marshal(result)
	coverageJSON, _ := json.Marshal(coverage)
	updated, err := m.db.ExecContext(ctx, `
		UPDATE source_discovery_jobs
		SET job_state=?, result_json=?, coverage_report_json=?, revision=revision+1
		WHERE job_id=?
	`, state, string(resultJSON), string(coverageJSON), jobID)
	if err != nil {
		return nil, referenceStoreError(err)
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changed == 0 {
		return nil, ErrNotFound
	}
	return m.GetSourceDiscoveryJob(ctx, jobID)
}

func (m *mariadbStore) FindLatestSourceDiscoveryJob(ctx context.Context, workID, continuityID string) (*SourceDiscoveryJob, error) {
	if err := m.ensureDB(); err != nil {
		return nil, err
	}
	workID = strings.TrimSpace(workID)
	continuityID = strings.TrimSpace(continuityID)
	if workID == "" || continuityID == "" {
		return nil, ErrInvalidReference
	}
	var jobID string
	err := m.db.QueryRowContext(ctx, `
		SELECT job_id
		FROM source_discovery_jobs
		WHERE JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.work_id')) = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(request_json, '$.continuity_id')) = ?
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`, workID, continuityID).Scan(&jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.GetSourceDiscoveryJob(ctx, jobID)
}

func sourceDiscoveryState(value string) bool {
	switch strings.TrimSpace(value) {
	case "created", "scope_ready", "discovering", "fetching", "extracting", "reconciling",
		"coverage_review", "ready_for_admission", "awaiting_exception_review",
		"insufficient_source_coverage", "blocked_by_access_policy", "failed", "cancelled":
		return true
	default:
		return false
	}
}
