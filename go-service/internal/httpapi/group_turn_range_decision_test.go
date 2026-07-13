package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRollbackDecisionProtectsCopiedSessionBaseline(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "assistant_deleted_output_removed",
		CandidateFromTurn: 1, PreviousTurnIndex: 8, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 1, BackendLatestTurn: 8, DeletionObserved: true,
		Baseline: &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"},
	})
	if !resp.Allowed || resp.FromTurn != 8 || resp.ProtectedBeforeTurn != 7 || resp.MinFromTurn != 8 {
		t.Fatalf("decision=%+v", resp)
	}
}

func TestCopiedSessionSevenPlusTwoDeletesOnlyTurnNine(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		PreviousTurnIndex: 9, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 1, BackendLatestTurn: 9,
		DeletionObserved: true, Baseline: baseline,
	})
	if !resp.Allowed || resp.FromTurn != 9 || resp.ProtectedBeforeTurn != 7 {
		t.Fatalf("7+2 single-tail-delete decision=%+v", resp)
	}
}

func TestCopiedSessionSevenPlusTwoDeletingTwoTurnsStartsAtEight(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		PreviousTurnIndex: 9, RemovedAssistantCount: 2,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9,
		DeletionObserved: true, Baseline: baseline,
	})
	if !resp.Allowed || resp.FromTurn != 8 || resp.MinFromTurn != 8 {
		t.Fatalf("7+2 two-tail-delete decision=%+v", resp)
	}
}

func TestRollbackDecisionBlocksHistoryTrimAndOutOfRange(t *testing.T) {
	trim := calculateRollbackDecision(rollbackDecisionRequest{ChatSessionID: "s", DeletionObserved: true, CandidateFromTurn: 3, HistoryTrimGuard: true})
	if trim.Allowed || trim.Reason != "history_trim_guard" {
		t.Fatalf("trim=%+v", trim)
	}
	out := calculateRollbackDecision(rollbackDecisionRequest{ChatSessionID: "s", DeletionObserved: true, CandidateFromTurn: 9, BackendLatestTurn: 8})
	if out.Allowed || out.Reason != "delete_anchor_after_backend_tail" {
		t.Fatalf("out=%+v", out)
	}
}

func TestSessionRoutingTurnResolutionPreservesOldPlusNewTurns(t *testing.T) {
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: "timeline_migrate"}
	pair := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", LocalTurnIndex: 2, Baseline: baseline})
	if pair.Resolution != "rebased" || pair.TurnIndex != 9 {
		t.Fatalf("pair=%+v", pair)
	}
	visible := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "visible_completed", VisibleCompletedTurns: 2, Baseline: baseline})
	if visible.CompletedTurns != 9 || visible.MinFromTurn != 8 {
		t.Fatalf("visible=%+v", visible)
	}
}

func TestAllSessionRoutingModesProtectImportedTurnsOnFirstLocalDelete(t *testing.T) {
	for _, reason := range []string{"timeline_copy", "timeline_migrate", "timeline_attach"} {
		t.Run(reason, func(t *testing.T) {
			baseline := &routingTurnBaseline{BackendTurnAtRoute: 7, LocalPairsAtRoute: 0, Reason: reason}
			firstLocalTurn := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
				Mode: "pair", LocalTurnIndex: 1, Baseline: baseline,
			})
			if firstLocalTurn.TurnIndex != 8 || firstLocalTurn.ProtectedBeforeTurn != 7 || firstLocalTurn.MinFromTurn != 8 {
				t.Fatalf("first local turn must be 7+1, resolution=%+v", firstLocalTurn)
			}

			afterDelete := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
				Mode: "visible_completed", VisibleCompletedTurns: 0, Baseline: baseline,
			})
			if afterDelete.CompletedTurns != 7 || afterDelete.ProtectedBeforeTurn != 7 || afterDelete.MinFromTurn != 8 {
				t.Fatalf("empty local chat must retain imported seven turns, resolution=%+v", afterDelete)
			}

			decision := calculateRollbackDecision(rollbackDecisionRequest{
				ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "assistant_deleted_output_removed",
				CandidateFromTurn: 1, PreviousTurnIndex: 8, RemovedAssistantCount: 1,
				VisibleCompletedTurns: 0, BackendLatestTurn: 8, DeletionObserved: true, Baseline: baseline,
			})
			if !decision.Allowed || decision.FromTurn != 8 || decision.ProtectedBeforeTurn != 7 || decision.MinFromTurn != 8 {
				t.Fatalf("first local delete must remove only turn 8, decision=%+v", decision)
			}
		})
	}
}

func TestRollbackDecisionTokenIsOneUseAndBoundToRange(t *testing.T) {
	ledger := newRollbackDecisionLedger()
	record := ledger.issue("s", 4, "auto")
	if _, ok := ledger.consume(record.Token, "s", 5); ok {
		t.Fatal("token accepted wrong turn")
	}
	record = ledger.issue("s", 4, "auto")
	if _, ok := ledger.consume(record.Token, "s", 4); !ok {
		t.Fatal("token rejected matching decision")
	}
	if _, ok := ledger.consume(record.Token, "s", 4); ok {
		t.Fatal("token reused")
	}
}

func TestRollbackHandlerRejectsInvalidDecisionTokenBeforeMutation(t *testing.T) {
	server := &Server{RollbackDecisions: newRollbackDecisionLedger()}
	req := httptest.NewRequest(http.MethodDelete, "/rollback/4?chat_session_id=s&req_source=auto&decision_token=invalid", nil)
	req.SetPathValue("turn_index", "4")
	rec := httptest.NewRecorder()
	server.handleRollback(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
