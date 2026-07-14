package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type durableRoutingBaselineStore struct {
	store.Store
	baseline *store.SessionRoutingBaseline
}

func (s *durableRoutingBaselineStore) GetSessionRoutingBaseline(context.Context, string) (*store.SessionRoutingBaseline, error) {
	return s.baseline, nil
}

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

func TestSessionRoutingHandlerUsesDurableCopiedBaselineWhenClientBaselineIsMissing(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	server := &Server{Store: &durableRoutingBaselineStore{
		Store: store.NewNoopStore(),
		baseline: &store.SessionRoutingBaseline{
			MigrationID: 42, SourceSessionID: "source", TargetSessionID: sid,
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 8,
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/session-routing/turn-resolution", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"mode":"pair",
		"risu_user_message_index":0,
		"observed_pair_ordinal":1,
		"baseline":{"backend_turn_at_route":1,"local_pairs_at_route":0,"reason":"timeline_copy"}
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response sessionRoutingTurnResolutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.BaselineApplied || response.TurnIndex != 9 || response.ProtectedBeforeTurn != 8 || response.MinFromTurn != 9 {
		t.Fatalf("durable copied baseline was not applied: %+v", response)
	}
}

func TestRollbackDecisionHandlerUsesDurableCopiedBaselineWhenClientBaselineIsMissing(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	server := &Server{Store: &durableRoutingBaselineStore{
		Store: store.NewNoopStore(),
		baseline: &store.SessionRoutingBaseline{
			MigrationID: 42, SourceSessionID: "source", TargetSessionID: sid,
			Mode: store.SessionMigrationModeCopyKeepSource, ImportedThroughTurn: 8,
		},
	}}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"request_source":"auto",
		"candidate_from_turn":1,
		"first_removed_turn":1,
		"visible_completed_turns":0,
		"backend_latest_turn":9,
		"deletion_observed":true
	}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response rollbackDecisionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Allowed || !response.BaselineApplied || response.FromTurn != 9 || response.ProtectedBeforeTurn != 8 || response.MinFromTurn != 9 {
		t.Fatalf("durable copied rollback baseline was not applied: %+v", response)
	}
}

func TestVerifiedTailDeleteUsesBackendTailWhenCopiedBaselineIsMissing(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto", Reason: "active_chat_tail_missing_from_runtime",
		CandidateFromTurn: 1, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9,
		DeletionObserved: true, LedgerVerified: true,
	})
	if !resp.Allowed || resp.FromTurn != 9 {
		t.Fatalf("missing-baseline verified tail decision=%+v", resp)
	}
}

func TestVerifiedTailDeleteRejectsImpossibleRemovedCount(t *testing.T) {
	resp := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: "char_1_cid_target", RequestSource: "auto",
		CandidateFromTurn: 1, RemovedAssistantCount: 10,
		BackendLatestTurn: 9, DeletionObserved: true, LedgerVerified: true,
	})
	if resp.Allowed || resp.Reason != "ledger_removed_count_exceeds_backend_tail" {
		t.Fatalf("impossible removed count decision=%+v", resp)
	}
}

func TestVerifiedTailDeleteWithoutClientBaselineExecutesOnlyBackendTail(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recordingStore := &rollbackRecordingStore{Store: store.NewNoopStore()}
	server := &Server{Cfg: cfg, Store: recordingStore}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	decisionReq := httptest.NewRequest(http.MethodPost, "/rollback/decision", strings.NewReader(`{
		"chat_session_id":"`+sid+`",
		"request_source":"auto",
		"reason":"active_chat_tail_missing_from_runtime",
		"candidate_from_turn":1,
		"removed_assistant_count":1,
		"visible_completed_turns":0,
		"backend_latest_turn":9,
		"deletion_observed":true,
		"ledger_verified":true
	}`))
	decisionRec := httptest.NewRecorder()
	mux.ServeHTTP(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status=%d body=%s", decisionRec.Code, decisionRec.Body.String())
	}
	var decision rollbackDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decision); err != nil {
		t.Fatalf("decode decision: %v", err)
	}
	if !decision.Allowed || decision.FromTurn != 9 || decision.DecisionToken == "" {
		t.Fatalf("decision=%+v", decision)
	}

	rollbackReq := httptest.NewRequest(http.MethodDelete, "/rollback/9?chat_session_id="+sid+"&req_source=auto&decision_token="+decision.DecisionToken, nil)
	rollbackRec := httptest.NewRecorder()
	mux.ServeHTTP(rollbackRec, rollbackReq)
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}
	if len(recordingStore.deletes) == 0 {
		t.Fatal("rollback did not execute store deletions")
	}
	for _, deletion := range recordingStore.deletes {
		if !strings.HasSuffix(deletion, ":"+sid+":9") {
			t.Fatalf("rollback escaped backend tail: %s", deletion)
		}
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

func TestSessionRoutingTurnResolutionDerivesTurnsFromRisuUserIndexes(t *testing.T) {
	for _, tc := range []struct {
		messageIndex int
		wantTurn     int
	}{
		{messageIndex: 0, wantTurn: 1},
		{messageIndex: 2, wantTurn: 2},
		{messageIndex: 4, wantTurn: 3},
		{messageIndex: 12, wantTurn: 7},
	} {
		messageIndex := tc.messageIndex
		got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
			Mode:                 "pair",
			RisuUserMessageIndex: &messageIndex,
			ObservedPairOrdinal:  99,
		})
		if got.TurnIndex != tc.wantTurn || got.LocalTurnIndex != tc.wantTurn || got.LocalTurnSource != "risu_user_message_index" {
			t.Fatalf("index=%d resolution=%+v", tc.messageIndex, got)
		}
	}
}

func TestSessionRoutingTurnResolutionUsesObservedOrdinalOnlyWhenIndexIsUnavailable(t *testing.T) {
	oddAssistantIndex := 5
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:                 "pair",
		RisuUserMessageIndex: &oddAssistantIndex,
		ObservedPairOrdinal:  4,
		LocalTurnIndex:       77,
	})
	if got.TurnIndex != 4 || got.LocalTurnIndex != 4 || got.LocalTurnSource != "observed_pair_ordinal" {
		t.Fatalf("ordinal fallback resolution=%+v", got)
	}

	legacy := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{Mode: "pair", LocalTurnIndex: 6})
	if legacy.ContractVersion != "session-routing.turn-resolution.v1" || legacy.TurnIndex != 6 || legacy.LocalTurnSource != "legacy_local_turn_index" {
		t.Fatalf("legacy compatibility resolution=%+v", legacy)
	}
}

func TestSessionRoutingVisibleCompletedUsesRawRisuIndex(t *testing.T) {
	messageIndex := 12
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:                  "visible_completed",
		RisuUserMessageIndex:  &messageIndex,
		ObservedPairOrdinal:   6,
		VisibleCompletedTurns: 99,
	})
	if got.CompletedTurns != 7 || got.LocalTurnIndex != 7 || got.LocalTurnSource != "risu_user_message_index" {
		t.Fatalf("visible completed resolution=%+v", got)
	}
}

func TestSessionRoutingTurnResolutionBatchPreservesIndexGapsAndBaseline(t *testing.T) {
	indexes := []int{0, 4, 8}
	observations := make([]routingTurnObservation, 0, len(indexes))
	for position := range indexes {
		index := indexes[position]
		observations = append(observations, routingTurnObservation{
			ObservationIndex:     position,
			RisuUserMessageIndex: &index,
			ObservedPairOrdinal:  position + 1,
		})
	}
	got := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
		Mode:         "batch",
		Observations: observations,
		Baseline:     &routingTurnBaseline{BackendTurnAtRoute: 8, LocalPairsAtRoute: 0, Reason: "timeline_copy"},
	})
	if len(got.ResolvedObservations) != len(observations) {
		t.Fatalf("batch size=%d want=%d", len(got.ResolvedObservations), len(observations))
	}
	wantTurns := []int{9, 11, 13}
	for i, item := range got.ResolvedObservations {
		if item.ObservationIndex != i || item.TurnIndex != wantTurns[i] || item.Source != "risu_user_message_index" {
			t.Fatalf("batch[%d]=%+v wantTurn=%d", i, item, wantTurns[i])
		}
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

func TestCopiedEightPlusOneRollbackDecisionExecutesOnlyTurnNine(t *testing.T) {
	const sid = "char_1_cid_copy_target"
	baseline := &routingTurnBaseline{BackendTurnAtRoute: 8, LocalPairsAtRoute: 0, Reason: "timeline_copy"}
	decision := calculateRollbackDecision(rollbackDecisionRequest{
		ChatSessionID: sid, RequestSource: "auto", Reason: "assistant_deleted_output_removed",
		CandidateFromTurn: 1, PreviousTurnIndex: 9, RemovedAssistantCount: 1,
		VisibleCompletedTurns: 0, BackendLatestTurn: 9, DeletionObserved: true,
		Baseline: baseline,
	})
	if !decision.Allowed || decision.FromTurn != 9 || decision.ProtectedBeforeTurn != 8 || decision.MinFromTurn != 9 {
		t.Fatalf("8+1 rollback decision=%+v", decision)
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	recordingStore := &rollbackRecordingStore{Store: store.NewNoopStore()}
	server := &Server{Cfg: cfg, Store: recordingStore}
	record := server.rollbackDecisionLedger().issue(sid, decision.FromTurn, "auto")
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodDelete, "/rollback/9?chat_session_id="+sid+"&req_source=auto&decision_token="+record.Token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(recordingStore.deletes) == 0 {
		t.Fatal("rollback handler did not execute store deletions")
	}
	for _, deletion := range recordingStore.deletes {
		if !strings.HasSuffix(deletion, ":"+sid+":9") {
			t.Fatalf("copied turn baseline was not preserved: %s", deletion)
		}
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
