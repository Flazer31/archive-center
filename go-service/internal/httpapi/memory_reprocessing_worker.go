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
			ctx, jobs, job, leaseOwner, now, "source_revision_read_failed",
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

	extractionCfg := s.completeTurnExtractionConfig(nil)
	if !extractionCfg.Critic.hasConfig() {
		result.State = "retryable"
		result.Failure = "critic_config_missing"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, now, result.Failure,
		)
	}
	extraction, _, err := s.runCompleteTurnCriticWithInputPolicy(
		ctx,
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
		result.State = "retryable"
		result.Failure = "critic_extract_failed"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), result.Failure,
		)
	}
	extraction, _ = applyRisuPersonaSubjectiveMemoryRoles(extraction, nil)
	content := strings.TrimSpace(strings.Join(
		[]string{source.UserContent, source.AssistantContent}, "\n",
	))
	artifactContext := contextWithStoredMemorySource(ctx, source)
	existingEvidence, _ := s.Store.ListEvidence(ctx, source.ChatSessionID)
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
		result.State = "retryable"
		result.Failure = "derived_persist_failed"
		return result, s.retryMemoryReprocessingJob(
			ctx, jobs, job, leaseOwner, time.Now().UTC(), result.Failure,
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

func (s *Server) retryMemoryReprocessingJob(
	ctx context.Context,
	jobs store.MemoryReprocessingJobStore,
	job *store.MemoryReprocessingJob,
	leaseOwner string,
	now time.Time,
	failure string,
) error {
	if job == nil {
		return fmt.Errorf("memory reprocessing job is missing")
	}
	retryAfter := now.Add(memoryReprocessingRetryDelay(job.Attempts))
	err := jobs.FailMemoryReprocessingJob(
		ctx, job.ID, leaseOwner, now, retryAfter, false, failure,
	)
	if errors.Is(err, store.ErrSourceRevisionStale) {
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
