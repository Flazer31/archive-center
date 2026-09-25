package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type deletionReconcileJournal47 struct {
	*rollbackDecisionExecutionStore
	events []store.AuditLog
}

func (s *deletionReconcileJournal47) ListAuditLogs(_ context.Context, _ string, event string, _ int) ([]store.AuditLog, error) {
	if event == sourceAcceptanceTransitionEvent {
		return s.events, nil
	}
	return nil, nil
}

func TestPendingInputGroupMemberIsReplacementDuringReconciliation47(t *testing.T) {
	const sid = "char_47_cid_input_group"
	first := completeTurnSourceObservation{HostChatID: "host", HostChatIDState: "observed", UserMessageChatID: "group-user-first", UserMessageChatIDState: "observed"}
	last := first
	last.UserMessageChatID = "group-user-last"
	current := store.MemorySourceRevision{ChatSessionID: sid, TurnIndex: 7, SourceRevision: "group-revision", LogicalTurnID: completeTurnLogicalTurnID(sid, last), LifecycleState: "active", AssistantContent: "group output"}
	state := completeTurnSourceAcceptanceState{SessionID: sid, TurnIndex: current.TurnIndex, Revision: current.SourceRevision, LogicalTurnID: current.LogicalTurnID, Lifecycle: "active_final", ObservedAtMS: 10, UserLogicalTurnIDs: []string{completeTurnLogicalTurnID(sid, first), completeTurnLogicalTurnID(sid, last)}}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	base := &deletionReconcileJournal47{rollbackDecisionExecutionStore: &rollbackDecisionExecutionStore{rollbackRecordingStore: &rollbackRecordingStore{Store: store.NewNoopStore()}, activeSources: []store.MemorySourceRevision{current}}, events: []store.AuditLog{{DetailsJSON: string(raw)}}}
	evidence, err := verifyRollbackAssistantDeletionEvidence(context.Background(), base, sid, nil, &first)
	if err != nil || !evidence.Verified || evidence.RemovedCount != 0 {
		t.Fatalf("surviving input-group member lost replacement: %+v %v", evidence, err)
	}
}

// Exercise the real decision endpoint without a browser snapshot or a claimed
// deletion count. Canonical turns deliberately differ from Host pair ordinals.
func TestFullHostDeletionReconciliation47(t *testing.T) {
	const sid = "char_47_cid_deletion"
	for _, tc := range []struct {
		name             string
		remaining        int
		pending          string
		middle, disabled bool
	}{
		{name: "ten_tail_turns_after_reload", remaining: 10},
		{name: "all_tail_turns_after_reload", remaining: 0},
		{name: "intact", remaining: 20},
		{name: "disabled_still_present", remaining: 20, disabled: true},
		{name: "middle_gap_keeps_surviving_suffix", remaining: 10, middle: true},
		{name: "same_user_reroll", remaining: 19, pending: "same"},
		{name: "same_user_edited_regeneration", remaining: 19, pending: "edited"},
		{name: "new_user_identical_text", remaining: 19, pending: "new"},
		{name: "normal_new_user_after_bulk_delete", remaining: 10, pending: "new"},
		{name: "regenerate_retained_user_before_deleted_suffix", remaining: 10, pending: "same"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &rollbackRecordingStore{Store: store.NewNoopStore()}
			base := &rollbackDecisionExecutionStore{rollbackRecordingStore: recorder, rollbackRouteBindingFixture: newRollbackRouteBindingFixture(sid)}
			var users []completeTurnSourceObservation
			var observed []rollbackAssistantObservation
			for i := 0; i < 20; i++ {
				user := completeTurnSourceObservation{HostChatID: rollbackTestHostChatID, HostChatIDState: "observed", UserMessageChatID: fmt.Sprintf("user-%d", i), UserMessageChatIDState: "observed", UserMessageIndex: i * 2, UserObservedContentHash: prepareOR1CHash("same input")}
				users = append(users, user)
				source := store.MemorySourceRevision{ChatSessionID: sid, ID: int64(i + 1), TurnIndex: i + 31, LogicalTurnID: completeTurnLogicalTurnID(sid, user), SourceMessageID: fmt.Sprintf("assistant-%d", i), AssistantContent: fmt.Sprintf("response-%d", i), LifecycleState: "active"}
				base.activeSources = append(base.activeSources, source)
				base.logs = append(base.logs, store.ChatLog{ChatSessionID: sid, TurnIndex: source.TurnIndex, Role: "assistant", Content: source.AssistantContent})
				if i < tc.remaining {
					observed = append(observed, rollbackAssistantObservation{MessageID: source.SourceMessageID, MessageIndex: i*2 + 1})
				}
			}
			if tc.middle {
				observed = append(observed, rollbackAssistantObservation{MessageID: base.activeSources[len(base.activeSources)-1].SourceMessageID, MessageIndex: 99})
			}
			if tc.disabled {
				observed[4].DisabledState = "disabled"
			}
			req := rollbackDecisionRequest{ChatSessionID: sid, StableCharacterID: rollbackTestStableCharacterID, StableCharacterIDState: "observed", HostChatID: rollbackTestHostChatID, HostChatIDState: "observed", RequestSource: "auto", AssistantObservationScope: "full_active_chat", AssistantObservations: observed}
			firstMissing := tc.remaining
			if tc.pending != "" {
				pending := users[tc.remaining]
				switch tc.pending {
				case "same", "edited":
					firstMissing++
					if tc.pending == "edited" {
						pending.UserObservedContentHash = prepareOR1CHash("edited input")
						pending.UserMessageIndex += 3
					}
				case "new":
					pending.UserMessageChatID = "new-user-with-identical-text"
				}
				req.PendingInputObservation = &pending
			}
			cfg := config.Default()
			cfg.StoreMode = config.StoreModeMariaDBAuthority
			server := &Server{Cfg: cfg, Store: base}
			mux := http.NewServeMux()
			server.RegisterRoutes(mux)
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/rollback/decision", bytes.NewReader(body)))
			var decision rollbackDecisionResponse
			if err := json.Unmarshal(response.Body.Bytes(), &decision); err != nil {
				t.Fatal(err, response.Body.String())
			}
			wantMutation := firstMissing < len(base.activeSources) && !tc.middle
			if decision.Allowed != wantMutation {
				t.Fatalf("decision=%+v; want mutation %v", decision, wantMutation)
			}
			if !wantMutation {
				if len(recorder.deletes) != 0 {
					t.Fatalf("intact/replacement/conflict mutated: %+v", recorder.deletes)
				}
				if tc.middle && decision.Reason != "historical_revision_conflict" {
					t.Fatalf("middle gap lost diagnostic: %+v", decision)
				}
				return
			}
			wantTurn := base.activeSources[firstMissing].TurnIndex
			if decision.FromTurn != wantTurn {
				t.Fatalf("range=%d want durable turn %d", decision.FromTurn, wantTurn)
			}
			deleteResponse := httptest.NewRecorder()
			path := fmt.Sprintf("/rollback/%d?chat_session_id=%s&req_source=auto&decision_token=%s&assistant_observation_digest=%s", decision.FromTurn, sid, decision.DecisionToken, decision.AssistantObservationDigest)
			mux.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, path, nil))
			if deleteResponse.Code != http.StatusOK || len(recorder.deletes) == 0 {
				t.Fatalf("canonical rollback not executed: %s", deleteResponse.Body.String())
			}
		})
	}
}
