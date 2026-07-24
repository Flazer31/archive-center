package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func completeTurnAcceptanceTestRequest(sid string, turn int, assistant string, observedAt int64, observedHash, generationID, streaming, position string, messageIndex, messageCount int) dto.M4CompleteTurnRequest {
	user := "user"
	return dto.M4CompleteTurnRequest{
		ChatSessionID:    sid,
		TurnIndex:        turn,
		UserInput:        &user,
		AssistantContent: &assistant,
		ClientMeta: map[string]any{
			"source_acceptance_required": true,
			"source_acceptance_observation": map[string]any{
				"contract_version":         completeTurnSourceAcceptanceContract,
				"observed_at_ms":           observedAt,
				"session_id":               sid,
				"host_chat_id":             "chat-1",
				"host_chat_id_state":       "observed",
				"chat_streaming_state":     streaming,
				"active_message_count":     messageCount,
				"message_index":            messageIndex,
				"message_role":             "char",
				"message_chat_id":          generationID,
				"message_chat_id_state":    "observed",
				"generation_id":            generationID,
				"generation_id_state":      "observed",
				"message_time_ms":          observedAt - 10,
				"message_time_state":       "observed",
				"observed_content_hash":    observedHash,
				"persistence_content_hash": prepareOR1CHash(assistant),
				"hash_algorithm":           "or1c_utf16_djb2.v1",
				"position_observation":     position,
				"revision_state":           "not_exposed_by_risuai",
			},
		},
	}
}

func completeTurnAnchoredAcceptanceTestRequest(sid string, turn int, user, assistant string, observedAt int64, generationID, streaming string, userIndex, assistantIndex, messageCount int) dto.M4CompleteTurnRequest {
	req := completeTurnAcceptanceTestRequest(sid, turn, assistant, observedAt, prepareOR1CHash(assistant), generationID, streaming, "current_active_chat_tail", assistantIndex, messageCount)
	req.UserInput = &user
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["user_message_index"] = userIndex
	observation["user_message_time_ms"] = int64(500)
	observation["user_message_time_state"] = "observed"
	observation["user_observed_content_hash"] = prepareOR1CHash(user)
	observation["user_persistence_content_hash"] = prepareOR1CHash(user)
	return req
}

func newCompleteTurnAcceptanceTestServer() *Server {
	return &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeDualShadow},
		Store:             store.NewNoopStore(),
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
	}
}

func TestCompleteTurnSourceAcceptanceUsesOfficialActiveChatObservation(t *testing.T) {
	req := completeTurnAcceptanceTestRequest("session-1", 2, "final answer", 1000, "or1c_host", "generation-1", "unobserved", "current_active_chat_tail", 3, 4)
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted || decision.Status != "accepted" || decision.Revision == "" {
		t.Fatalf("decision=%+v, want accepted Go-owned revision", decision)
	}
	if decision.Observation.RevisionState != "not_exposed_by_risuai" {
		t.Fatalf("host revision state=%q, want not_exposed_by_risuai", decision.Observation.RevisionState)
	}
}

func TestCompleteTurnSourceAcceptanceAllowsAssistantTailBeforeNonTurnMetadata(t *testing.T) {
	req := completeTurnAcceptanceTestRequest("session-1", 2, "final answer", 1000, "or1c_host", "generation-1", "not_streaming", "current_active_assistant_tail", 3, 5)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["later_active_turn_message_count"] = 0
	observation["later_non_turn_message_count"] = 1
	observation["message_disabled_state"] = "not_disabled"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted {
		t.Fatalf("assistant tail followed only by host metadata must be accepted: %+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsAssistantWithLaterActiveTurn(t *testing.T) {
	req := completeTurnAcceptanceTestRequest("session-1", 2, "old answer", 1000, "or1c_host", "generation-1", "not_streaming", "current_active_assistant_tail", 3, 5)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["later_active_turn_message_count"] = 1
	observation["message_disabled_state"] = "not_disabled"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_stale_or_superseded" || decision.Retryable || decision.QueueAction != "discard" {
		t.Fatalf("assistant with a later active turn must be rejected: %+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsDisabledAssistant(t *testing.T) {
	req := completeTurnAcceptanceTestRequest("session-1", 2, "disabled answer", 1000, "or1c_host", "generation-1", "not_streaming", "current_active_chat_tail", 3, 4)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["message_disabled_state"] = "disabled"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_assistant_message_disabled" {
		t.Fatalf("disabled assistant must not be canonical: %+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsStreamingWithoutWrites(t *testing.T) {
	req := completeTurnAcceptanceTestRequest("session-1", 2, "partial", 1000, "or1c_partial", "generation-1", "streaming", "current_active_chat_tail", 3, 4)
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_streaming_candidate" || decision.QueueAction != "retry_after_new_observation" {
		t.Fatalf("decision=%+v, want retryable streaming rejection", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsSupersededQueuedRevision(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAcceptanceTestRequest("session-1", 2, "first", 1000, "or1c_first", "generation-1", "not_streaming", "current_active_chat_tail", 3, 4)
	second := completeTurnAcceptanceTestRequest("session-1", 2, "second", 2000, "or1c_second", "generation-2", "not_streaming", "current_active_chat_tail", 3, 4)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), first); !decision.Accepted {
		t.Fatalf("first decision=%+v", decision)
	}
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), second); !decision.Accepted {
		t.Fatalf("second decision=%+v", decision)
	}
	stale := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if stale.Accepted || stale.Reason != "source_acceptance_stale_or_superseded" || stale.QueueAction != "discard" {
		t.Fatalf("stale decision=%+v, want terminal superseded rejection", stale)
	}
}

func TestCompleteTurnSourceAcceptanceRollbackFenceRejectsOldQueueAndAllowsNewGeneration(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	old := completeTurnAcceptanceTestRequest("session-1", 4, "old", 1000, "or1c_old", "generation-old", "not_streaming", "current_active_chat_tail", 7, 8)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), old); !decision.Accepted {
		t.Fatalf("old decision=%+v", decision)
	}
	server.invalidateCompleteTurnSourceAcceptances(context.Background(), "session-1", 4, "test", 1500)
	stale := server.beginCompleteTurnSourceAcceptance(context.Background(), old)
	if stale.Accepted || stale.Reason != "source_acceptance_deleted_or_rolled_back" {
		t.Fatalf("stale decision=%+v, want rollback rejection", stale)
	}
	newer := completeTurnAcceptanceTestRequest("session-1", 4, "new", stale.Observation.ObservedAtMS+2000000000000, "or1c_new", "generation-new", "not_streaming", "current_active_chat_tail", 7, 8)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), newer); !decision.Accepted {
		t.Fatalf("new generation decision=%+v, want accepted", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRollbackFenceRejectsPreviouslyUnseenOfflineQueue(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	server.invalidateCompleteTurnSourceAcceptances(context.Background(), "session-1", 4, "test", 1500)
	old := completeTurnAcceptanceTestRequest("session-1", 4, "offline old", 1000, "or1c_offline_old", "generation-old", "not_streaming", "current_active_chat_tail", 7, 8)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), old); decision.Accepted || decision.Reason != "source_acceptance_deleted_or_rolled_back" {
		t.Fatalf("offline stale decision=%+v, want rollback rejection", decision)
	}
	newer := completeTurnAcceptanceTestRequest("session-1", 4, "new", 2000, "or1c_new", "generation-new", "not_streaming", "current_active_chat_tail", 7, 8)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), newer); !decision.Accepted {
		t.Fatalf("post-rollback generation decision=%+v, want accepted", decision)
	}
}

func TestCompleteTurnSourceAcceptanceBackfillRequiresExplicitActiveChatBackfill(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	req := completeTurnAcceptanceTestRequest("session-1", 1, "historical", 1000, "or1c_historical", "generation-1", "not_streaming", "current_active_chat_message", 1, 4)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), req); decision.Accepted {
		t.Fatalf("ordinary non-tail observation accepted: %+v", decision)
	}
	req.ClientMeta["active_chat_backfill"] = map[string]any{"version": "active_chat_complete_turn_backfill.v1"}
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), req); !decision.Accepted {
		t.Fatalf("explicit active-chat backfill rejected: %+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectionDoesNotCompleteIdempotencyCache(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	server.CompleteTurns = newCompleteTurnRequestLedger()
	reqBody := completeTurnAcceptanceTestRequest("session-1", 2, "partial", 1000, "or1c_partial", "generation-1", "streaming", "current_active_chat_tail", 3, 4)
	reqBody.ClientMeta["idempotency_key"] = "candidate-key"
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/complete-turn", bytes.NewReader(body))
	server.handleCompleteTurn(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != "rejected" || response["queue_action"] != "retry_after_new_observation" {
		t.Fatalf("response=%v", response)
	}
	if _, _, found := server.CompleteTurns.status("candidate-key", time.Now().UTC()); found {
		t.Fatal("candidate rejection polluted complete-turn idempotency cache")
	}
}

func TestCompleteTurnSourceAcceptanceSupersedeCancelsOlderWorker(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	firstReq := completeTurnAcceptanceTestRequest("session-1", 2, "first", 1000, "or1c_first", "generation-1", "not_streaming", "current_active_chat_tail", 3, 4)
	first := server.beginCompleteTurnSourceAcceptance(context.Background(), firstReq)
	workerCtx, release := server.completeTurnSourceAcceptanceProcessingContext(context.Background(), first, "session-1", 2)
	defer release()
	secondReq := completeTurnAcceptanceTestRequest("session-1", 2, "second", 2000, "or1c_second", "generation-2", "not_streaming", "current_active_chat_tail", 3, 4)
	if second := server.beginCompleteTurnSourceAcceptance(context.Background(), secondReq); !second.Accepted {
		t.Fatalf("second=%+v", second)
	}
	select {
	case <-workerCtx.Done():
	default:
		t.Fatal("older complete-turn worker context was not canceled by a newer revision")
	}
}

func TestCompleteTurnSourceAcceptanceBindsRerollToSameLogicalTurn(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted || firstDecision.BoundTurn != 1 || firstDecision.ReplaceExisting || firstDecision.LogicalTurnID == "" {
		t.Fatalf("first decision=%+v", firstDecision)
	}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "generation-2", "not_streaming", 2, 3, 4)
	rerollDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !rerollDecision.Accepted || rerollDecision.BoundTurn != 1 || !rerollDecision.ReplaceExisting || rerollDecision.LogicalTurnID != firstDecision.LogicalTurnID {
		t.Fatalf("reroll decision=%+v", rerollDecision)
	}
	third := completeTurnAnchoredAcceptanceTestRequest("session-1", 3, "same user", "third", 3000, "generation-3", "not_streaming", 2, 3, 4)
	thirdDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), third)
	if !thirdDecision.Accepted || thirdDecision.BoundTurn != 1 || !thirdDecision.ReplaceExisting || thirdDecision.LogicalTurnID != firstDecision.LogicalTurnID {
		t.Fatalf("third reroll decision=%+v", thirdDecision)
	}
	stale := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if stale.Accepted || stale.Reason != "source_acceptance_stale_or_superseded" {
		t.Fatalf("stale decision=%+v", stale)
	}
}

func TestCompleteTurnSourceAcceptanceCandidateKeepsExistingFinal(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	candidate := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "partial", 2000, "generation-2", "streaming", 2, 3, 4)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), candidate); decision.Accepted || decision.Reason != "source_acceptance_streaming_candidate" {
		t.Fatalf("candidate decision=%+v", decision)
	}
	if !server.completeTurnSourceAcceptanceStillCurrent(firstDecision, "session-1", 1) {
		t.Fatal("streaming candidate superseded the existing active final")
	}
}

func TestCompleteTurnSourceAcceptanceFailedRerollObservationKeepsExistingFinal(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	failed := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "failed candidate", 2000, "generation-2", "not_streaming", 2, -1, 4)
	failedObservation := failed.ClientMeta["source_acceptance_observation"].(map[string]any)
	failedObservation["position_observation"] = "unobserved"
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), failed); decision.Accepted || decision.Reason != "source_acceptance_not_current_active_tail" {
		t.Fatalf("failed reroll decision=%+v", decision)
	}
	if !server.completeTurnSourceAcceptanceStillCurrent(firstDecision, "session-1", 1) {
		t.Fatal("failed reroll observation superseded the existing active final")
	}
}

func TestCompleteTurnSourceAcceptanceNewUserCreatesNextLogicalTurn(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "first user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	next := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "next user", "next", 2000, "generation-2", "not_streaming", 4, 5, 6)
	nextDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), next)
	if !nextDecision.Accepted || nextDecision.BoundTurn != 2 || nextDecision.ReplaceExisting || nextDecision.LogicalTurnID == firstDecision.LogicalTurnID {
		t.Fatalf("next decision=%+v first=%+v", nextDecision, firstDecision)
	}
}

func TestCompleteTurnSourceAcceptanceRestoresLogicalTurnAfterRestart(t *testing.T) {
	storage := &memoryFakeStore{}
	firstServer := &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, SourceAcceptances: newCompleteTurnSourceAcceptanceLedger()}
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	if decision := firstServer.beginCompleteTurnSourceAcceptance(context.Background(), first); !decision.Accepted {
		t.Fatalf("first decision=%+v", decision)
	}
	restarted := &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, SourceAcceptances: newCompleteTurnSourceAcceptanceLedger()}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "generation-2", "not_streaming", 2, 3, 4)
	reroll.ClientMeta["idempotency_key"] = "reroll-final"
	decision := restarted.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !decision.Accepted || decision.BoundTurn != 1 || !decision.ReplaceExisting {
		t.Fatalf("restart reroll decision=%+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceAdoptsLegacyTailWithMatchingUserAnchor(t *testing.T) {
	storage := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 1, Role: "user", Content: "same user"},
		{ChatSessionID: "session-1", TurnIndex: 1, Role: "assistant", Content: "old"},
	}}
	server := &Server{Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, SourceAcceptances: newCompleteTurnSourceAcceptanceLedger()}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "rerolled", 2000, "generation-2", "not_streaming", 2, 3, 4)
	decision := server.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !decision.Accepted || decision.BoundTurn != 1 || !decision.ReplaceExisting {
		t.Fatalf("legacy tail decision=%+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsLowerObservedTurnInsteadOfAppendingAfterWrongTail(t *testing.T) {
	storage := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "user", Content: "wrong old user"},
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "assistant", Content: "wrong old assistant"},
	}}
	server := &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store:             storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
	}
	req := completeTurnAnchoredAcceptanceTestRequest("session-1", 35, "actual user", "actual assistant", 2000, "generation-35", "not_streaming", 68, 69, 70)
	decision := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_session_tail_conflict" || !decision.Retryable {
		t.Fatalf("lower active-chat turn must force rerouting, not become turn 52: %+v", decision)
	}
	if decision.BoundTurn != 35 {
		t.Fatalf("observed turn was rewritten: %+v", decision)
	}
}

func TestCompleteTurnRerollReplacesCanonicalTailAndDeletesSupersededVector(t *testing.T) {
	storage := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "session-1", TurnIndex: 1, Role: "user", Content: "same user"},
			{ChatSessionID: "session-1", TurnIndex: 1, Role: "assistant", Content: "first"},
		},
		returnMemories: []store.Memory{{ID: 7, ChatSessionID: "session-1", TurnIndex: 1}},
	}
	vectors := &turnRecordingVectorStore{docs: []vector.VectorDocument{{ID: memoryVectorDocumentID("session-1", storage.returnMemories[0]), ChatSessionID: "session-1"}}}
	server := &Server{
		Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage, Vector: vectors,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(), CompleteTurns: newCompleteTurnRequestLedger(),
	}
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), first); !decision.Accepted {
		t.Fatalf("first decision=%+v", decision)
	}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "generation-2", "not_streaming", 2, 3, 4)
	reroll.ClientMeta["idempotency_key"] = "session-1-turn-1-reroll-generation-2"
	reroll.ClientMeta["critic"] = map[string]any{
		"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "timeout_ms": 90000,
	}
	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "The corrected assistant output became the canonical memory.",
		"importance_score":  7,
		"evidence_excerpts": []any{"rerolled"},
		"kg_triples":        []any{map[string]any{"subject": "Assistant", "predicate": "replaces", "object": "Old output"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model": "critic-model", "choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	criticCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		criticCalls++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(chatResp))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	body, err := json.Marshal(reroll)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/complete-turn", bytes.NewReader(body))
	server.handleCompleteTurn(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(storage.logicalTurnReplacements) != 1 || storage.logicalTurnReplacements[0].TurnIndex != 1 {
		t.Fatalf("replacements=%+v", storage.logicalTurnReplacements)
	}
	if len(storage.returnChatLogs) != 2 || storage.returnChatLogs[1].Content != "rerolled" {
		t.Fatalf("chat logs=%+v", storage.returnChatLogs)
	}
	if len(storage.savedChatLogs) != 0 {
		t.Fatalf("atomic replacement must not be followed by duplicate raw saves: %+v", storage.savedChatLogs)
	}
	if vectors.deleteDocumentCalls != 1 || len(vectors.deletedDocumentIDs) == 0 {
		t.Fatalf("vector delete calls=%d ids=%v", vectors.deleteDocumentCalls, vectors.deletedDocumentIDs)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["critic_triggered"] != true || len(storage.savedMemories) != 1 || criticCalls != 1 {
		t.Fatalf("replacement did not regenerate derived artifacts: critic=%v memories=%d calls=%d response=%v", response["critic_triggered"], len(storage.savedMemories), criticCalls, response)
	}
	repeated := httptest.NewRecorder()
	server.handleCompleteTurn(repeated, httptest.NewRequest("POST", "/complete-turn", bytes.NewReader(body)))
	if repeated.Code != 200 || len(storage.logicalTurnReplacements) != 1 || vectors.deleteDocumentCalls != 1 || criticCalls != 1 {
		t.Fatalf("repeat status=%d replacements=%d vector deletes=%d critic calls=%d body=%s", repeated.Code, len(storage.logicalTurnReplacements), vectors.deleteDocumentCalls, criticCalls, repeated.Body.String())
	}
}
