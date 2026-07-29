package httpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	memoryWorkerPollInterval = 2 * time.Second
	memoryWorkerLease        = 10 * time.Minute

	criticRetryLimitUnconfigured = "CRITIC_RETRY_LIMIT_UNCONFIGURED"
	criticRetryLimitReached      = "CRITIC_RETRY_LIMIT_REACHED"
)

type memoryReprocessingProcessResult struct {
	Processed      bool
	JobID          int64
	SourceRevision string
	State          string
	Failure        string
}

// StartMemoryWorkers starts the authority-owned reprocessing and vector-outbox
// loop. It never starts for shadow/read-only stores and it persists no provider
// credentials in either queue.
func (s *Server) StartMemoryWorkers(ctx context.Context) bool {
	if s == nil || s.Store == nil ||
		s.Cfg.StoreMode != config.StoreModeMariaDBAuthority ||
		ctx == nil {
		return false
	}
	if availability, ok := s.Store.(store.MemoryDerivationLifecycleAvailability); !ok ||
		!availability.MemoryDerivationLifecycleEnabled() {
		return false
	}
	if _, ok := s.Store.(store.MemoryReprocessingJobStore); !ok {
		return false
	}
	if _, ok := s.Store.(store.MemoryVectorOutboxStore); !ok {
		return false
	}
	if _, ok := s.Store.(store.MemoryAdmissionWriter); !ok {
		return false
	}
	owner := fmt.Sprintf("archive-memory-worker:%d", os.Getpid())
	go s.runMemoryWorkers(ctx, owner)
	return true
}

func (s *Server) runMemoryWorkers(ctx context.Context, owner string) {
	ticker := time.NewTicker(memoryWorkerPollInterval)
	defer ticker.Stop()
	for {
		_, _ = s.processMemoryReprocessingOnce(
			ctx, owner, time.Now().UTC(), memoryWorkerLease,
		)
		s.processMemoryVectorOutboxBatch(
			ctx, owner+":vector", time.Now().UTC(), memoryWorkerLease, 16,
		)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) processMemoryReprocessingOnce(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseDuration time.Duration,
) (memoryReprocessingProcessResult, error) {
	var result memoryReprocessingProcessResult
	if s == nil || s.Store == nil {
		return result, store.ErrNotEnabled
	}
	jobs, ok := s.Store.(store.MemoryReprocessingJobStore)
	if !ok {
		return result, store.ErrNotEnabled
	}
	sources, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		return result, store.ErrNotEnabled
	}
	job, err := jobs.ClaimMemoryReprocessingJob(ctx, leaseOwner, now, leaseDuration)
	if errors.Is(err, store.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Processed = true
	result.JobID = job.ID
	result.SourceRevision = job.SourceRevision

	source, err := sources.GetSourceRevision(ctx, job.ChatSessionID, job.SourceRevision)
	if err != nil {
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, now, &result, "source_revision_read_failed",
		)
	}
	if source.LifecycleState != "active" {
		finishErr := jobs.FailMemoryReprocessingJob(
			ctx, job.ID, leaseOwner, now, time.Time{}, true, "source_revision_not_active",
		)
		if errors.Is(finishErr, store.ErrSourceRevisionStale) {
			result.State = "stale_rejected"
			return result, nil
		}
		return result, finishErr
	}
	processingCtx, releaseSourceWorker := s.completeTurnStoredSourceProcessingContext(ctx, source)
	defer releaseSourceWorker()

	extractionCfg := s.completeTurnExtractionConfig(nil)
	if !extractionCfg.Critic.hasConfig() {
		result.State = "retryable"
		result.Failure = "critic_config_missing"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, now, &result, result.Failure,
		)
	}
	extraction, criticTrace, err := s.runCompleteTurnCriticWithInputPolicy(
		processingCtx,
		source.ChatSessionID,
		source.TurnIndex,
		source.UserContent,
		source.AssistantContent,
		nil,
		nil,
		extractionCfg.Critic,
		true,
	)
	if err != nil {
		if processingCtx.Err() != nil {
			return result, finishSupersededMemoryReprocessingJob(
				ctx, jobs, job, leaseOwner, time.Now().UTC(), &result,
			)
		}
		sourceStillActive, sourceStateErr := sources.IsSourceRevisionActive(
			ctx, job.ChatSessionID, job.SourceRevision,
		)
		if sourceStateErr == nil && !sourceStillActive {
			return result, finishSupersededMemoryReprocessingJob(
				ctx, jobs, job, leaseOwner, time.Now().UTC(), &result,
			)
		}
		failure := criticPipelineErrorDetails(err)
		result.Failure = strings.TrimSpace(stringFromMap(failure, "code"))
		if result.Failure == "" {
			result.Failure = "CRITIC_UNKNOWN_FAILED"
		}
		if preview := strings.TrimSpace(stringFromMap(criticTrace, "raw_preview")); preview != "" {
			result.Failure += ": " + truncateRunes(preview, 240)
		}
		if sourceStateErr == nil && sourceStillActive {
			s.recordMemoryReprocessingCriticFailure(
				context.WithoutCancel(processingCtx), job, source.TurnIndex, failure, criticTrace,
			)
		}
		if !boolFromAny(failure["retryable"]) {
			result.State = "terminal"
			return result, jobs.FailMemoryReprocessingJob(
				ctx, job.ID, leaseOwner, time.Now().UTC(), time.Time{}, true, result.Failure,
			)
		}
		result.State = "retryable"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), &result, result.Failure,
		)
	}
	active, activeErr := sources.IsSourceRevisionActive(ctx, job.ChatSessionID, job.SourceRevision)
	if activeErr != nil {
		result.State = "retryable"
		result.Failure = "source_revision_recheck_failed"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), &result, result.Failure,
		)
	}
	if !active {
		return result, finishSupersededMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), &result,
		)
	}
	extraction, _ = applyRisuPersonaSubjectiveMemoryRoles(extraction, nil)
	content := strings.TrimSpace(strings.Join(
		[]string{source.UserContent, source.AssistantContent}, "\n",
	))
	artifactContext := contextWithStoredMemorySource(processingCtx, source)
	existingEvidence, _ := s.Store.ListEvidence(processingCtx, source.ChatSessionID)
	saveResult := s.saveCriticExtractionArtifacts(
		artifactContext,
		source.ChatSessionID,
		source.TurnIndex,
		extraction,
		content,
		extractionCfg.Embedder,
		time.Now().UTC(),
		existingEvidence,
	)
	if saveResult.Errors > 0 {
		if processingCtx.Err() != nil {
			return result, finishSupersededMemoryReprocessingJob(
				ctx, jobs, job, leaseOwner, time.Now().UTC(), &result,
			)
		}
		result.State = "retryable"
		result.Failure = "derived_persist_failed"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), &result, result.Failure,
		)
	}
	if err := jobs.CompleteMemoryReprocessingJob(
		ctx, job.ID, leaseOwner, time.Now().UTC(),
	); err != nil {
		if errors.Is(err, store.ErrSourceRevisionStale) {
			result.State = "stale_rejected"
			return result, nil
		}
		return result, err
	}
	result.State = "completed"
	return result, nil
}

func (s *Server) recordMemoryReprocessingCriticFailure(
	ctx context.Context,
	job *store.MemoryReprocessingJob,
	turnIndex int,
	failure map[string]any,
	criticTrace map[string]any,
) {
	if s == nil || s.Store == nil || job == nil || ctx == nil {
		return
	}
	safeTrace := map[string]any{}
	for _, key := range []string{
		"prompt_source", "provider", "model", "code", "stage",
		"retryable", "http_status",
	} {
		if value, ok := criticTrace[key]; ok {
			safeTrace[key] = value
		}
	}
	if preview := strings.TrimSpace(stringFromMap(criticTrace, "raw_preview")); preview != "" {
		apiKey := s.runtimeConfigSnapshot().CriticAPIKey
		safeTrace["raw_preview"] = truncateRunes(
			strings.TrimSpace(scrubCriticFailureText(preview, apiKey)), 1000,
		)
	}
	_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
		ChatSessionID: job.ChatSessionID,
		EventType:     "critic_reprocessing_failed",
		TargetType:    "memory_reprocessing_job",
		TargetID:      job.ID,
		Summary:       fmt.Sprintf("critic reprocessing failed turn %d", turnIndex),
		DetailsJSON: mustCompactJSON(map[string]any{
			"job_id":          job.ID,
			"source_revision": job.SourceRevision,
			"turn_index":      turnIndex,
			"attempt":         job.Attempts,
			"failure":         failure,
			"trace":           safeTrace,
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: time.Now().UTC(),
	})
}

func finishSupersededMemoryReprocessingJob(
	ctx context.Context,
	jobs store.MemoryReprocessingJobStore,
	job *store.MemoryReprocessingJob,
	leaseOwner string,
	now time.Time,
	result *memoryReprocessingProcessResult,
) error {
	if result != nil {
		result.State = "stale_rejected"
		result.Failure = "CRITIC_RESULT_SUPERSEDED"
	}
	if job == nil {
		return fmt.Errorf("memory reprocessing job is missing")
	}
	err := jobs.FailMemoryReprocessingJob(
		ctx, job.ID, leaseOwner, now, time.Time{}, true, "CRITIC_RESULT_SUPERSEDED",
	)
	if errors.Is(err, store.ErrSourceRevisionStale) {
		return nil
	}
	return err
}

func (s *Server) retryMemoryReprocessingJob(
	ctx context.Context,
	jobs store.MemoryReprocessingJobStore,
	job *store.MemoryReprocessingJob,
	leaseOwner string,
	now time.Time,
	result *memoryReprocessingProcessResult,
	failure string,
) error {
	if job == nil {
		return fmt.Errorf("memory reprocessing job is missing")
	}
	maxAttempts := s.runtimeConfigSnapshot().FailedQueueMaxAttempts
	terminalCode := ""
	switch {
	case maxAttempts < 1 || maxAttempts > 11:
		terminalCode = criticRetryLimitUnconfigured
	case job.Attempts >= maxAttempts:
		terminalCode = criticRetryLimitReached
	}
	if terminalCode != "" {
		if result != nil {
			result.State = "terminal"
			result.Failure = terminalCode
		}
		persistedFailure := terminalCode
		if cause := strings.TrimSpace(failure); cause != "" &&
			!strings.EqualFold(cause, terminalCode) {
			persistedFailure += ": " + cause
		}
		err := jobs.FailMemoryReprocessingJob(
			ctx, job.ID, leaseOwner, now, time.Time{}, true, persistedFailure,
		)
		if errors.Is(err, store.ErrSourceRevisionStale) {
			if result != nil {
				result.State = "stale_rejected"
				result.Failure = ""
			}
			return nil
		}
		return err
	}
	if result != nil {
		result.State = "retryable"
		result.Failure = strings.TrimSpace(failure)
	}
	retryAfter := now.Add(memoryReprocessingRetryDelay(job.Attempts))
	err := jobs.FailMemoryReprocessingJob(
		ctx, job.ID, leaseOwner, now, retryAfter, false, failure,
	)
	if errors.Is(err, store.ErrSourceRevisionStale) {
		if result != nil {
			result.State = "stale_rejected"
			result.Failure = ""
		}
		return nil
	}
	return err
}

func memoryReprocessingRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 15 * time.Second
	for i := 1; i < attempt && delay < 15*time.Minute; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		return 15 * time.Minute
	}
	return delay
}
