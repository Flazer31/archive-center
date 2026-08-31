package httpapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func completeTurnMemorySourceRevision(
	decision completeTurnSourceAcceptanceDecision,
	sid string,
	turnIndex int,
	userText string,
	assistantText string,
	now time.Time,
) (*store.MemorySourceRevision, error) {
	if !decision.Enabled || !decision.Accepted {
		return nil, nil
	}
	if strings.TrimSpace(decision.Revision) == "" {
		return nil, fmt.Errorf("source_revision_not_exposed")
	}
	if strings.TrimSpace(decision.LogicalTurnID) == "" {
		return nil, fmt.Errorf("source_revision_logical_turn_not_exposed")
	}
	if strings.TrimSpace(userText) == "" || strings.TrimSpace(assistantText) == "" {
		return nil, fmt.Errorf("source_revision_raw_pair_missing")
	}
	messageID := strings.TrimSpace(decision.Observation.MessageChatID)
	if decision.Observation.MessageChatIDState != "observed" || messageID == "" {
		messageID = ""
	}
	if messageID == "" && decision.Observation.MessageIndex >= 0 {
		messageID = fmt.Sprintf("%s:index:%d", decision.Observation.HostChatID, decision.Observation.MessageIndex)
	}
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	branchState := strings.TrimSpace(decision.Observation.BranchIDState)
	if branchState != "observed" {
		branchState = "not_exposed"
	}
	return &store.MemorySourceRevision{
		ContractVersion:              store.MemorySourceRevisionContract,
		SourceRevision:               decision.Revision,
		ChatSessionID:                strings.TrimSpace(sid),
		LogicalTurnID:                decision.LogicalTurnID,
		TurnIndex:                    turnIndex,
		SourceMessageID:              messageID,
		SourceGenerationID:           decision.Observation.GenerationID,
		BranchID:                     observedCompleteTurnBranchIdentity(decision.Observation),
		BranchState:                  branchState,
		UserContent:                  userText,
		AssistantContent:             assistantText,
		CombinedContentHash:          contentHash,
		UserObservedContentHash:      decision.Observation.UserObservedContentHash,
		AssistantObservedContentHash: decision.Observation.ObservedContentHash,
		HashAlgorithm:                decision.Observation.HashAlgorithm,
		HostObservedAtMS:             decision.Observation.ObservedAtMS,
		LifecycleState:               "active",
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}, nil
}

func (s *Server) registerCompleteTurnSourceRevision(
	ctx context.Context,
	decision completeTurnSourceAcceptanceDecision,
	sid string,
	turnIndex int,
	userText string,
	assistantText string,
	now time.Time,
) error {
	if !decision.Enabled || !decision.Accepted || s == nil || s.Store == nil {
		return nil
	}
	writer, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		// The common aggregate admission cutover is a later bounded change.
		// Legacy/noop/read-only stores must not claim durable source fencing.
		return nil
	}
	if availability, ok := s.Store.(store.MemoryDerivationLifecycleAvailability); ok &&
		!availability.MemoryDerivationLifecycleEnabled() {
		return nil
	}
	source, err := completeTurnMemorySourceRevision(
		decision, sid, turnIndex, userText, assistantText, now,
	)
	if err != nil {
		return err
	}
	if source == nil {
		return nil
	}
	_, err = writer.RegisterAcceptedSourceRevision(ctx, source)
	return err
}

func completeTurnReprocessingIdempotencyKey(
	sid string,
	sourceRevision string,
	derivationVersion string,
	extractorVersion string,
	indexVersion string,
) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(sid),
		strings.TrimSpace(sourceRevision),
		strings.TrimSpace(derivationVersion),
		strings.TrimSpace(extractorVersion),
		strings.TrimSpace(indexVersion),
	}, "\x1f")))
	return fmt.Sprintf("%x", sum)
}

func (s *Server) enqueueCompleteTurnReprocessingJob(
	ctx context.Context,
	decision completeTurnSourceAcceptanceDecision,
	sid string,
	reason string,
	now time.Time,
	retryAfter time.Time,
) (bool, error) {
	if !decision.Enabled || !decision.Accepted || strings.TrimSpace(reason) == "" ||
		s == nil || s.Store == nil {
		return false, nil
	}
	writer, ok := s.Store.(store.MemoryReprocessingJobStore)
	if !ok {
		return false, nil
	}
	if availability, ok := s.Store.(store.MemoryDerivationLifecycleAvailability); ok &&
		!availability.MemoryDerivationLifecycleEnabled() {
		return false, nil
	}
	source := &store.MemorySourceRevision{
		ChatSessionID:  strings.TrimSpace(sid),
		SourceRevision: strings.TrimSpace(decision.Revision),
	}
	return s.enqueueSourceRevisionReprocessingJob(ctx, writer, source, reason, now, retryAfter, false)
}

func (s *Server) enqueueSourceRevisionReprocessingJob(
	ctx context.Context,
	writer store.MemoryReprocessingJobStore,
	source *store.MemorySourceRevision,
	reason string,
	now time.Time,
	retryAfter time.Time,
	wakeAfterEnqueue bool,
) (bool, error) {
	if source == nil ||
		strings.TrimSpace(source.ChatSessionID) == "" ||
		strings.TrimSpace(source.SourceRevision) == "" ||
		strings.TrimSpace(reason) == "" {
		return false, nil
	}
	initialStatus := "pending"
	if strings.HasPrefix(strings.TrimSpace(reason), "CRITIC_SCHEMA_INVALID") {
		initialStatus = "permanent"
	}
	job := &store.MemoryReprocessingJob{
		ContractVersion:   store.MemoryReprocessingJobContract,
		ChatSessionID:     strings.TrimSpace(source.ChatSessionID),
		SourceRevision:    strings.TrimSpace(source.SourceRevision),
		SourceContract:    completeTurnSourceAcceptanceContract,
		DerivationVersion: store.MemoryAdmissionContract,
		ExtractorVersion:  completeTurnCriticPipelineVersion,
		IndexVersion:      memoryAdmissionIndexVersion,
		Status:            initialStatus,
		RetryAfter:        retryAfter,
		LastError:         strings.TrimSpace(reason),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	job.IdempotencyKey = completeTurnReprocessingIdempotencyKey(
		job.ChatSessionID,
		job.SourceRevision,
		job.DerivationVersion,
		job.ExtractorVersion,
		job.IndexVersion,
	)
	inserted, err := writer.EnqueueMemoryReprocessingJob(ctx, job)
	if wakeAfterEnqueue && err == nil && (job.Status == "pending" || job.Status == "retryable") {
		s.wakeMemoryWorkers()
	}
	return inserted, err
}
