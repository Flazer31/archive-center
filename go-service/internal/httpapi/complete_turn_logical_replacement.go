package httpapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type logicalTurnReplacementError struct {
	Code         string
	Stage        string
	Retryable    bool
	RawCommitted bool
	CommitState  string
	Cause        error
}

func (e *logicalTurnReplacementError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func newLogicalTurnReplacementError(code, stage string, retryable, rawCommitted bool, cause error) *logicalTurnReplacementError {
	commitState := "not_committed"
	if rawCommitted {
		commitState = "committed"
	}
	return &logicalTurnReplacementError{
		Code:         code,
		Stage:        stage,
		Retryable:    retryable,
		RawCommitted: rawCommitted,
		CommitState:  commitState,
		Cause:        cause,
	}
}

func (s *Server) replaceCompleteTurnLogicalTail(ctx context.Context, sid string, turnIndex int, userText, assistantText string, decision completeTurnSourceAcceptanceDecision, now time.Time) *logicalTurnReplacementError {
	replacer, ok := s.Store.(store.LogicalTurnReplacementStore)
	if !ok {
		return newLogicalTurnReplacementError(
			"logical_turn_replacement_unsupported", "preflight", false, false,
			fmt.Errorf("canonical store does not support logical turn replacement"),
		)
	}
	sourceRevision, err := completeTurnMemorySourceRevision(decision, sid, turnIndex, userText, assistantText, now)
	if err != nil {
		return newLogicalTurnReplacementError(
			"logical_turn_source_revision_invalid", "source_revision", false, false, err,
		)
	}
	if err := replacer.ReplaceLogicalTurn(ctx, store.LogicalTurnReplacement{
		ChatSessionID: sid, TurnIndex: turnIndex, UserContent: userText,
		AssistantContent: assistantText, CreatedAt: now, SourceRevision: sourceRevision,
	}); err != nil {
		var typedStoreErr *store.LogicalTurnReplacementError
		if errors.As(err, &typedStoreErr) {
			commitState := typedStoreErr.CommitState
			if commitState == "" {
				commitState = "not_committed"
			}
			return &logicalTurnReplacementError{
				Code:         typedStoreErr.Code,
				Stage:        typedStoreErr.Stage,
				Retryable:    typedStoreErr.Retryable,
				RawCommitted: commitState == "committed",
				CommitState:  commitState,
				Cause:        err,
			}
		}
		return newLogicalTurnReplacementError(
			"logical_turn_replace_transaction_failed", "canonical_replace", true, false, err,
		)
	}
	// Canonical replacement is already committed. Wake the durable outbox
	// worker; provider failure records retry state and cannot undo or falsely
	// complete the MariaDB source transition.
	s.wakeMemoryWorkers()
	if _, err := clearReferenceRuntimeCandidatesAfterRollback(ctx, s.Store, sid, turnIndex); err != nil {
		return newLogicalTurnReplacementError(
			"logical_turn_reference_cleanup_failed", "reference_cleanup", true, true, err,
		)
	}
	if _, err := restoreNarrativeCurrentStatesAfterRollback(ctx, s.Store, sid, turnIndex-1); err != nil {
		return newLogicalTurnReplacementError(
			"logical_turn_narrative_restore_failed", "narrative_restore", true, true, err,
		)
	}
	if _, err := restoreStoryClockCurrentAfterRollback(ctx, s.Store, sid); err != nil {
		return newLogicalTurnReplacementError(
			"logical_turn_story_clock_restore_failed", "story_clock_restore", true, true, err,
		)
	}
	if _, err := restoreReversibleStateCurrentAfterRollback(ctx, s.Store, sid); err != nil {
		return newLogicalTurnReplacementError(
			"logical_turn_reversible_state_restore_failed", "reversible_state_restore", true, true, err,
		)
	}
	return nil
}
