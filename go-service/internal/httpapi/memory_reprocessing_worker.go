package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	criticRetryLimitUnconfigured      = "CRITIC_RETRY_LIMIT_UNCONFIGURED"
	criticRetryLimitReached           = "CRITIC_RETRY_LIMIT_REACHED"
	memoryWorkerConfigDeferred        = "RUNTIME_CONFIG_NOT_SYNCED"
	memoryWorkerOperationalRetryDelay = time.Second
	criticReprocessingDefaultInterval = 30 * time.Second
	memoryReprocessingPerWake         = 4
	memoryVectorGroupsPerWake         = 8
	memoryVectorDeleteEvery           = 4
)

type memoryReprocessingProcessResult struct {
	Processed      bool
	JobID          int64
	SourceRevision string
	State          string
	Failure        string
	Attempt        int
	MaxAttempts    int
	RetryAfter     time.Time
	RetryDelay     time.Duration
}

type acceptedSourceDerivationResult struct {
	State              string
	Failure            string
	SaveResult         artifactSaveResult
	CriticFailure      map[string]any
	CriticTrace        map[string]any
	AuditCriticFailure bool
	RetryDelay         time.Duration
}

type acceptedSourceDerivationOptions struct {
	CreateCriticInputSnapshot bool
	CommittedReplayOnly       bool
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
	started := false
	s.memoryWorkerStartOnce.Do(func() {
		started = true
		owner := fmt.Sprintf("archive-memory-worker:%d", os.Getpid())
		go s.runMemoryWorkers(ctx, owner)
		s.wakeMemoryWorkers()
	})
	return started
}

func (s *Server) runMemoryWorkers(ctx context.Context, owner string) {
	var retryTimer *time.Timer
	var retryWake <-chan time.Time
	var retryScheduledAt time.Time
	scheduleRetryWake := func(at time.Time) {
		if at.IsZero() || (!retryScheduledAt.IsZero() && !at.Before(retryScheduledAt)) {
			return
		}
		delay := time.Until(at)
		if delay < 0 {
			delay = 0
		}
		if retryTimer == nil {
			retryTimer = time.NewTimer(delay)
		} else {
			if !retryTimer.Stop() {
				select {
				case <-retryTimer.C:
				default:
				}
			}
			retryTimer.Reset(delay)
		}
		retryWake = retryTimer.C
		retryScheduledAt = at
	}
	defer func() {
		if retryTimer != nil {
			retryTimer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.memoryWorkerWakeChannel():
		case <-retryWake:
			retryWake = nil
			retryScheduledAt = time.Time{}
		}
		scheduleRetryWake(s.processMemoryWorkerWake(ctx, owner, time.Now().UTC()))
	}
}

func (s *Server) memoryWorkerWakeChannel() chan struct{} {
	if s == nil {
		return nil
	}
	s.memoryWorkerWakeOnce.Do(func() {
		if s.memoryWorkerWake == nil {
			s.memoryWorkerWake = make(chan struct{}, 1)
		}
	})
	return s.memoryWorkerWake
}

func (s *Server) wakeMemoryWorkers() {
	if s == nil {
		return
	}
	select {
	case s.memoryWorkerWakeChannel() <- struct{}{}:
	default:
	}
}

func memoryWorkerLeaseDuration(runtimeConfig RuntimeConfig) time.Duration {
	var timeoutSeconds int64
	if runtimeConfig.CriticTimeoutSec > 0 {
		timeoutSeconds += runtimeConfig.CriticTimeoutSec
	}
	if runtimeConfig.EmbeddingTimeoutSec > 0 {
		timeoutSeconds += runtimeConfig.EmbeddingTimeoutSec
	}
	if timeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func (s *Server) processMemoryWorkerWake(
	ctx context.Context,
	owner string,
	_ time.Time,
) time.Time {
	runtimeConfig := s.runtimeConfigSnapshot()
	if !runtimeConfig.Synced {
		return time.Time{}
	}
	leaseDuration := memoryWorkerLeaseDuration(runtimeConfig)
	if leaseDuration <= 0 {
		return time.Time{}
	}
	var retryWakeAt time.Time
	reprocessingProcessed := 0
	for ctx.Err() == nil && reprocessingProcessed < memoryReprocessingPerWake {
		result, err := s.processMemoryReprocessingOnce(
			ctx, owner, time.Now().UTC(), leaseDuration,
		)
		if err != nil || !result.Processed {
			break
		}
		// Wake at the earliest retry produced by this drain. After that wake, the
		// durable schedule query below rediscovers the next queued retry.
		if result.State == "retryable" && !result.RetryAfter.IsZero() &&
			(retryWakeAt.IsZero() || result.RetryAfter.Before(retryWakeAt)) {
			retryWakeAt = result.RetryAfter
		}
		reprocessingProcessed++
	}
	vectorGroupsProcessed := 0
	for ctx.Err() == nil && vectorGroupsProcessed < memoryVectorGroupsPerWake {
		preferredOperation := "upsert"
		if vectorGroupsProcessed%memoryVectorDeleteEvery == 0 {
			preferredOperation = "delete"
		}
		results, err := s.processMemoryVectorOutboxGroupPreferred(
			ctx, owner+":vector", time.Now().UTC(), leaseDuration, preferredOperation,
		)
		if (err == nil || errors.Is(err, store.ErrNotFound)) && len(results) == 0 {
			results, err = s.processMemoryVectorOutboxGroupPreferred(
				ctx, owner+":vector", time.Now().UTC(), leaseDuration, "",
			)
		}
		if err != nil || len(results) == 0 {
			break
		}
		vectorGroupsProcessed++
	}
	if reprocessingProcessed == memoryReprocessingPerWake || vectorGroupsProcessed == memoryVectorGroupsPerWake {
		s.wakeMemoryWorkers()
	}
	if schedule, ok := s.Store.(store.MemoryReprocessingWakeScheduleStore); ok {
		if next, err := schedule.NextMemoryReprocessingWakeAt(ctx); err == nil &&
			!next.IsZero() && (retryWakeAt.IsZero() || next.Before(retryWakeAt)) {
			retryWakeAt = next
		}
	}
	return retryWakeAt
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
	if !s.runtimeConfigSnapshot().Synced {
		result.State = "deferred_config_sync"
		result.Failure = memoryWorkerConfigDeferred
		return result, nil
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
	result.Attempt = job.Attempts
	result.MaxAttempts = s.runtimeConfigSnapshot().FailedQueueMaxAttempts

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
		if finishErr == nil && s.TurnWorkflows != nil {
			s.TurnWorkflows.updateRecoveryResult(
				source.ChatSessionID, source.TurnIndex, source.SourceRevision,
				"stale_rejected", "source_revision_not_active", artifactSaveResult{},
			)
		}
		return result, finishErr
	}
	derivation := s.processAcceptedSourceRevision(
		ctx,
		source,
		s.completeTurnExtractionConfig(nil),
		false,
	)
	result.State = derivation.State
	result.Failure = derivation.Failure
	result.RetryDelay = derivation.RetryDelay
	if derivation.AuditCriticFailure {
		s.recordMemoryReprocessingCriticFailure(
			context.WithoutCancel(ctx),
			job,
			source.TurnIndex,
			derivation.CriticFailure,
			derivation.CriticTrace,
		)
	}
	switch derivation.State {
	case "completed", "skipped_ooc":
		if err := jobs.CompleteMemoryReprocessingJob(
			ctx, job.ID, leaseOwner, time.Now().UTC(),
		); err != nil {
			if errors.Is(err, store.ErrSourceRevisionStale) {
				result.State = "stale_rejected"
				return result, nil
			}
			return result, err
		}
		if s.TurnWorkflows != nil {
			s.TurnWorkflows.updateRecoveryResult(
				source.ChatSessionID, source.TurnIndex, source.SourceRevision,
				derivation.State, "", derivation.SaveResult,
			)
		}
		return result, nil
	case "stale_rejected":
		finishErr := finishSupersededMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), &result,
		)
		if finishErr == nil && s.TurnWorkflows != nil {
			s.TurnWorkflows.updateRecoveryResult(
				source.ChatSessionID, source.TurnIndex, source.SourceRevision,
				"stale_rejected", result.Failure, artifactSaveResult{},
			)
		}
		return result, finishErr
	case "terminal":
		finishErr := jobs.FailMemoryReprocessingJob(
			ctx, job.ID, leaseOwner, time.Now().UTC(), time.Time{}, true, result.Failure,
		)
		if finishErr == nil && s.TurnWorkflows != nil {
			s.TurnWorkflows.updateRecoveryResult(
				source.ChatSessionID, source.TurnIndex, source.SourceRevision,
				"terminal", result.Failure, artifactSaveResult{},
			)
		}
		return result, finishErr
	case "retryable":
		retryErr := s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, now, &result, result.Failure,
		)
		if retryErr == nil && s.TurnWorkflows != nil {
			s.TurnWorkflows.updateRecoveryResultWithAttempt(
				source.ChatSessionID, source.TurnIndex, source.SourceRevision,
				result.State, result.Failure, artifactSaveResult{},
				result.Attempt, result.MaxAttempts, result.RetryAfter,
				criticProviderHUDDetails(derivation.CriticTrace),
			)
		}
		return result, retryErr
	default:
		return result, fmt.Errorf(
			"accepted source derivation returned unknown state %q",
			derivation.State,
		)
	}
}

func (s *Server) processAcceptedSourceRevision(
	ctx context.Context,
	source *store.MemorySourceRevision,
	extractionCfg completeTurnExtractionConfig,
	createCriticInputSnapshot bool,
) acceptedSourceDerivationResult {
	return s.processAcceptedSourceRevisionWithOptions(
		ctx,
		source,
		extractionCfg,
		acceptedSourceDerivationOptions{
			CreateCriticInputSnapshot: createCriticInputSnapshot,
		},
	)
}

func (s *Server) processAcceptedSourceRevisionWithOptions(
	ctx context.Context,
	source *store.MemorySourceRevision,
	extractionCfg completeTurnExtractionConfig,
	options acceptedSourceDerivationOptions,
) acceptedSourceDerivationResult {
	var result acceptedSourceDerivationResult
	if s == nil || s.Store == nil || source == nil {
		result.State = "terminal"
		result.Failure = "accepted_source_revision_missing"
		return result
	}
	sources, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		result.State = "terminal"
		result.Failure = "source_revision_store_unavailable"
		return result
	}
	processingCtx, releaseSourceWorker := s.completeTurnStoredSourceProcessingContext(ctx, source)
	defer releaseSourceWorker()
	lock, lockErr := s.sessionMigrationSourceLock(ctx, source.ChatSessionID)
	if lockErr != nil {
		result.State = "retryable"
		result.Failure = "source_session_migration_lock_read_failed"
		return result
	}
	if lock != nil {
		result.State = "retryable"
		result.Failure = "source_session_migration_locked"
		result.RetryDelay = memoryWorkerOperationalRetryDelay
		return result
	}
	active, err := sources.IsSourceRevisionActive(
		processingCtx,
		source.ChatSessionID,
		source.SourceRevision,
	)
	if err != nil {
		result.State = "retryable"
		result.Failure = "source_revision_recheck_failed"
		return result
	}
	if !active || source.LifecycleState != "active" {
		result.State = "stale_rejected"
		result.Failure = "CRITIC_RESULT_SUPERSEDED"
		return result
	}
	extraction, committedResult, committedFailure := storedMemoryAdmissionExtraction(source)
	if committedResult {
		if committedFailure != "" {
			result.State = "retryable"
			result.Failure = committedFailure
			return result
		}
		result.CriticTrace = map[string]any{
			"stage":           source.DerivedAdmissionState + "_result_replay",
			"source_revision": source.SourceRevision,
			"stored_index":    source.DerivedIndexVersion,
			"target_index":    memoryAdmissionIndexVersion,
		}
	} else {
		if options.CommittedReplayOnly {
			result.State = "terminal"
			result.Failure = "committed_derived_result_missing"
			return result
		}
		if !extractionCfg.Critic.hasConfig() {
			result.State = "retryable"
			result.Failure = "critic_config_missing"
			return result
		}
		criticExtraction, criticTrace, err := s.runCompleteTurnCriticWithInputPolicy(
			processingCtx,
			source.ChatSessionID,
			source.TurnIndex,
			source.UserContent,
			source.AssistantContent,
			nil,
			nil,
			extractionCfg.Critic,
			true,
			s.completeTurnCriticInputPolicy(nil),
			completeTurnCriticInputReplay{
				SourceRevision: source.SourceRevision,
				SnapshotJSON:   source.CriticInputSnapshotJSON,
				SnapshotHash:   source.CriticInputSnapshotHash,
				Required:       !options.CreateCriticInputSnapshot,
			},
		)
		result.CriticTrace = criticTrace
		if err != nil {
			if processingCtx.Err() != nil {
				result.State = "stale_rejected"
				result.Failure = "CRITIC_RESULT_SUPERSEDED"
				return result
			}
			sourceStillActive, sourceStateErr := sources.IsSourceRevisionActive(
				ctx, source.ChatSessionID, source.SourceRevision,
			)
			if sourceStateErr == nil && !sourceStillActive {
				result.State = "stale_rejected"
				result.Failure = "CRITIC_RESULT_SUPERSEDED"
				return result
			}
			failure := criticPipelineErrorDetails(err)
			result.CriticFailure = failure
			failureCode := strings.TrimSpace(stringFromMap(failure, "code"))
			result.Failure = failureCode
			if result.Failure == "" {
				result.Failure = "CRITIC_UNKNOWN_FAILED"
			}
			if preview := strings.TrimSpace(stringFromMap(criticTrace, "raw_preview")); preview != "" {
				result.Failure += ": " + truncateRunes(preview, 240)
			}
			if sourceStateErr == nil && sourceStillActive {
				result.AuditCriticFailure = true
			}
			if failureCode == "CRITIC_SCHEMA_INVALID" || !boolFromAny(failure["retryable"]) {
				result.State = "terminal"
				return result
			}
			result.State = "retryable"
			result.RetryDelay = criticReprocessingDelay(s.runtimeConfigSnapshot(), result.CriticTrace)
			return result
		}
		extraction = criticExtraction
	}
	active, activeErr := sources.IsSourceRevisionActive(
		ctx,
		source.ChatSessionID,
		source.SourceRevision,
	)
	if activeErr != nil {
		result.State = "retryable"
		result.Failure = "source_revision_recheck_failed"
		return result
	}
	if !active {
		result.State = "stale_rejected"
		result.Failure = "CRITIC_RESULT_SUPERSEDED"
		return result
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
	result.SaveResult = saveResult
	if saveResult.Errors > 0 {
		if processingCtx.Err() != nil {
			result.State = "stale_rejected"
			result.Failure = "CRITIC_RESULT_SUPERSEDED"
			return result
		}
		result.State = "retryable"
		result.Failure = completeTurnPersistenceFailureSummary(
			s.completeTurnPersistenceDiagnostics(saveResult.ErrorDetails),
		)
		return result
	}
	result.State = "completed"
	return result
}

// storedMemoryAdmissionExtraction validates the durable Critic result with
// the exact versions that originally produced its hash. An index-version
// change is therefore a projection replay, not permission to sample Critic
// again. A committed or pending snapshot with corrupt material is a
// present-but-invalid result and must never fall through to Critic.
func storedMemoryAdmissionExtraction(source *store.MemorySourceRevision) (map[string]any, bool, string) {
	if source == nil {
		return nil, false, ""
	}
	state := strings.TrimSpace(source.DerivedAdmissionState)
	if state != "committed" &&
		!(state == "pending" && strings.TrimSpace(source.DerivedResultJSON) != "") {
		return nil, false, ""
	}
	if source.DerivedAdmissionVersion != store.MemoryAdmissionContract ||
		source.DerivedExtractorVersion != completeTurnCriticPipelineVersion {
		return nil, false, ""
	}
	if strings.TrimSpace(source.DerivedIndexVersion) == "" ||
		strings.TrimSpace(source.DerivedResultHash) == "" ||
		strings.TrimSpace(source.DerivedResultJSON) == "" {
		return nil, true, "committed_derived_result_incomplete"
	}
	var extraction map[string]any
	if err := json.Unmarshal([]byte(source.DerivedResultJSON), &extraction); err != nil || extraction == nil {
		return nil, true, "committed_derived_result_invalid"
	}
	if memoryAdmissionResultHashFromCanonicalJSON(
		source.SourceRevision,
		strings.TrimSpace(source.DerivedResultJSON),
		source.DerivedAdmissionVersion,
		source.DerivedExtractorVersion,
		source.DerivedIndexVersion,
	) != source.DerivedResultHash {
		return nil, true, "committed_derived_result_hash_mismatch"
	}
	return extraction, true, ""
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
	if callLedger := safeProviderCallBudgetLedger(criticTrace["provider_call_budget_ledger"]); len(callLedger) > 0 {
		safeTrace["provider_call_budget_ledger"] = callLedger
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
	now = time.Now().UTC()
	if job == nil {
		return fmt.Errorf("memory reprocessing job is missing")
	}
	maxAttempts := s.runtimeConfigSnapshot().FailedQueueMaxAttempts
	if result != nil {
		result.Attempt = job.Attempts
		result.MaxAttempts = maxAttempts
		result.RetryAfter = time.Time{}
	}
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
	retryDelay := memoryWorkerOperationalRetryDelay
	if result != nil && result.RetryDelay > 0 {
		retryDelay = result.RetryDelay
	}
	if result != nil {
		result.State = "retryable"
		result.Failure = strings.TrimSpace(failure)
		result.RetryAfter = now.Add(retryDelay)
	}
	err := jobs.FailMemoryReprocessingJob(
		ctx, job.ID, leaseOwner, now, now.Add(retryDelay), false, failure,
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

func criticReprocessingDelay(runtimeConfig RuntimeConfig, criticTrace map[string]any) time.Duration {
	seconds := runtimeConfig.CriticReprocessingIntervalSec
	if seconds <= 0 {
		seconds = int(criticReprocessingDefaultInterval / time.Second)
	}
	if seconds > 3600 {
		seconds = 3600
	}
	providerResponse := mapFromAny(criticTrace["provider_response"])
	providerSeconds := intFromAny(providerResponse["retry_after_seconds"], 0)
	if providerSeconds <= 0 {
		ledger := mapFromAny(criticTrace["provider_call_budget_ledger"])
		providerSeconds = intFromAny(ledger["retry_after_seconds"], 0)
	}
	if providerSeconds > seconds {
		seconds = providerSeconds
	}
	return time.Duration(seconds) * time.Second
}
