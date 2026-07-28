package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) replaceCompleteTurnLogicalTail(ctx context.Context, sid string, turnIndex int, userText, assistantText string, decision completeTurnSourceAcceptanceDecision, now time.Time) error {
	replacer, ok := s.Store.(store.LogicalTurnReplacementStore)
	if !ok {
		return fmt.Errorf("canonical store does not support logical turn replacement")
	}
	sourceRevision, err := completeTurnMemorySourceRevision(decision, sid, turnIndex, userText, assistantText, now)
	if err != nil {
		return err
	}
	if err := replacer.ReplaceLogicalTurn(ctx, store.LogicalTurnReplacement{
		ChatSessionID: sid, TurnIndex: turnIndex, UserContent: userText,
		AssistantContent: assistantText, CreatedAt: now, SourceRevision: sourceRevision,
	}); err != nil {
		return err
	}
	// Canonical replacement is already committed. Vector work is drained from
	// the durable outbox; provider failure records retry state and cannot undo
	// or falsely complete the MariaDB source transition.
	s.processMemoryVectorOutboxBatch(
		ctx, fmt.Sprintf("complete-turn:%s", sid), now, 30*time.Second, 64,
	)
	if _, err := clearReferenceRuntimeCandidatesAfterRollback(ctx, s.Store, sid, turnIndex); err != nil {
		return fmt.Errorf("clear superseded reference runtime: %w", err)
	}
	if _, err := restoreNarrativeCurrentStatesAfterRollback(ctx, s.Store, sid); err != nil {
		return fmt.Errorf("restore narrative current states: %w", err)
	}
	return nil
}
