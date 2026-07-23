package httpapi

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) replaceCompleteTurnLogicalTail(ctx context.Context, sid string, turnIndex int, userText, assistantText string, now time.Time) error {
	replacer, ok := s.Store.(store.LogicalTurnReplacementStore)
	if !ok {
		return fmt.Errorf("canonical store does not support logical turn replacement")
	}
	vectorIDs, err := rollbackVectorDocumentIDs(ctx, s.Store, sid, turnIndex)
	if err != nil {
		return fmt.Errorf("collect superseded vector ids: %w", err)
	}
	if len(vectorIDs) > 0 && s.Vector != nil {
		deleter, ok := s.Vector.(vector.DocumentDeleter)
		if !ok {
			return fmt.Errorf("vector store does not support superseded document deletion")
		}
		if err := deleter.DeleteDocuments(ctx, vectorIDs); err != nil && !errors.Is(err, vector.ErrNotEnabled) {
			return fmt.Errorf("delete superseded vectors: %w", err)
		}
	}
	if err := replacer.ReplaceLogicalTurn(ctx, store.LogicalTurnReplacement{
		ChatSessionID: sid, TurnIndex: turnIndex, UserContent: userText,
		AssistantContent: assistantText, CreatedAt: now,
	}); err != nil {
		return err
	}
	if _, err := clearReferenceRuntimeCandidatesAfterRollback(ctx, s.Store, sid, turnIndex); err != nil {
		return fmt.Errorf("clear superseded reference runtime: %w", err)
	}
	if _, err := restoreNarrativeCurrentStatesAfterRollback(ctx, s.Store, sid); err != nil {
		return fmt.Errorf("restore narrative current states: %w", err)
	}
	return nil
}
