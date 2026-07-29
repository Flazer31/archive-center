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

func (s *logicalReplacementFailureStore) ReplaceLogicalTurn(context.Context, store.LogicalTurnReplacement) error {
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
