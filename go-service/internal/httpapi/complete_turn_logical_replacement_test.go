package httpapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type logicalReplacementFailureStore struct {
	store.Store
	err error
}

type logicalReplacementStoryClockStore struct {
	turnRecordingStore
}

func (s *logicalReplacementStoryClockStore) ReplaceLogicalTurn(_ context.Context, replacement store.LogicalTurnReplacement) error {
	events := s.savedStatusEvents[:0]
	for _, event := range s.savedStatusEvents {
		if event.ChatSessionID == replacement.ChatSessionID && event.SourceTurn >= replacement.TurnIndex {
			continue
		}
		events = append(events, event)
	}
	s.savedStatusEvents = events
	current := s.returnStatusCurrent[:0]
	for _, value := range s.returnStatusCurrent {
		if value.ChatSessionID == replacement.ChatSessionID && value.SourceTurn >= replacement.TurnIndex {
			continue
		}
		current = append(current, value)
	}
	s.returnStatusCurrent = current
	return nil
}

func (s *logicalReplacementStoryClockStore) RollbackCanonicalTail(context.Context, store.LogicalTurnRollback) error {
	return nil
}

func (s *logicalReplacementFailureStore) ReplaceLogicalTurn(context.Context, store.LogicalTurnReplacement) error {
	return s.err
}

func (s *logicalReplacementFailureStore) RollbackCanonicalTail(context.Context, store.LogicalTurnRollback) error {
	return s.err
}

func TestLogicalTurnReplacementErrorPreservesTypedState(t *testing.T) {
	cause := errors.New("cleanup failed")
	err := newLogicalTurnReplacementError(
		"logical_turn_reference_cleanup_failed",
		"reference_cleanup",
		true,
		true,
		cause,
	)
	if err.Code != "logical_turn_reference_cleanup_failed" || err.Stage != "reference_cleanup" {
		t.Fatalf("unexpected typed replacement error: %+v", err)
	}
	if !err.Retryable || !err.RawCommitted {
		t.Fatalf("post-commit cleanup failure lost commit/retry state: %+v", err)
	}
	if err.Error() == "" {
		t.Fatal("typed replacement error must expose a diagnostic message")
	}
}

func TestReplaceCompleteTurnLogicalTailTypesPreCommitStoreFailure(t *testing.T) {
	storeErr := errors.New("replace transaction failed")
	server := &Server{Store: &logicalReplacementFailureStore{
		Store: store.NewNoopStore(),
		err:   storeErr,
	}}
	decision := completeTurnSourceAcceptanceDecision{
		Enabled:       true,
		Accepted:      true,
		Revision:      "sar_test",
		LogicalTurnID: "logical_turn_test",
		Observation: completeTurnSourceObservation{
			MessageIndex: -1,
		},
	}
	err := server.replaceCompleteTurnLogicalTail(
		context.Background(),
		"session-test",
		1,
		"user",
		"assistant",
		decision,
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected typed logical replacement failure")
	}
	if err.Code != "logical_turn_replace_transaction_failed" || err.Stage != "canonical_replace" {
		t.Fatalf("unexpected replacement classification: %+v", err)
	}
	if !err.Retryable || err.RawCommitted {
		t.Fatalf("pre-commit transaction failure has wrong retry/commit state: %+v", err)
	}
}

func TestReplaceCompleteTurnLogicalTailPreservesTerminalStoreClassification(t *testing.T) {
	storeErr := &store.LogicalTurnReplacementError{
		Code:        "logical_turn_not_current_tail",
		Stage:       "canonical_tail_check",
		Retryable:   false,
		CommitState: "not_committed",
		Cause:       errors.New("latest turn changed"),
	}
	server := &Server{Store: &logicalReplacementFailureStore{
		Store: store.NewNoopStore(),
		err:   storeErr,
	}}
	decision := completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: "sar_test", LogicalTurnID: "logical_turn_test",
		Observation: completeTurnSourceObservation{MessageIndex: -1},
	}
	err := server.replaceCompleteTurnLogicalTail(
		context.Background(), "session-test", 1, "user", "assistant", decision, time.Now().UTC(),
	)
	if err == nil {
		t.Fatal("expected terminal logical replacement failure")
	}
	if err.Code != storeErr.Code || err.Stage != storeErr.Stage || err.Retryable || err.RawCommitted || err.CommitState != "not_committed" {
		t.Fatalf("store classification was flattened: %+v", err)
	}
}

func TestReplaceCompleteTurnLogicalTailRestoresStoryClockProjection(t *testing.T) {
	prior := mustCompactJSON(map[string]any{
		"version": storyClockContractVersion, "observation_kind": "absolute",
		"scene_scope": "current", "precision": "exact",
		"absolute": map[string]any{"date": "1423-04-12"}, "source_turn": 8,
	})
	fake := &logicalReplacementStoryClockStore{}
	fake.savedStatusEvents = []store.StatusChangeEvent{
		{
			ID: 10, ChatSessionID: "session-test", RegistryID: 3, StatusKey: storyClockStatusKey,
			OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, EventKind: "set",
			NewValueJSON: prior,
			EvidenceJSON: `{"source_revision":"prior","current_projection":true}`,
			SourceTurn:   8, EventState: "recorded", CreatedAt: time.Unix(8, 0),
		},
		{
			ID: 11, ChatSessionID: "session-test", RegistryID: 3, StatusKey: storyClockStatusKey,
			OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, EventKind: "change",
			NewValueJSON: `{"version":"story_clock.v1","observation_kind":"absolute","scene_scope":"current","precision":"exact","absolute":{"date":"1423-04-13"},"source_turn":9}`,
			EvidenceJSON: `{"source_revision":"replaced","current_projection":true}`,
			SourceTurn:   9, EventState: "recorded", CreatedAt: time.Unix(9, 0),
		},
	}
	fake.returnStatusCurrent = []store.StatusCurrentValue{{
		ID: 11, ChatSessionID: "session-test", RegistryID: 3, StatusKey: storyClockStatusKey,
		OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID,
		ValueJSON: fake.savedStatusEvents[1].NewValueJSON, SourceTurn: 9,
	}}
	server := &Server{Store: fake}
	decision := completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Revision: "sar_test", LogicalTurnID: "logical_turn_test",
		Observation: completeTurnSourceObservation{MessageIndex: -1},
	}
	if err := server.replaceCompleteTurnLogicalTail(
		context.Background(), "session-test", 9, "user", "assistant", decision, time.Now().UTC(),
	); err != nil {
		t.Fatalf("logical replacement story-clock restore failed: %+v", err)
	}
	if len(fake.savedStatusCurrent) != 1 ||
		decodeStoryClockValue(t, fake.savedStatusCurrent[0])["source_turn"] != float64(8) {
		t.Fatalf("logical replacement did not restore prior story clock: %#v", fake.savedStatusCurrent)
	}
}
