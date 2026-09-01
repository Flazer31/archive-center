package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
				"branch_id":                "",
				"branch_id_state":          "not_exposed_by_risuai",
				"message_swipe_id":         -1,
				"message_swipe_id_state":   "not_present",
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
	observation["user_observed_pair_ordinal"] = (userIndex / 2) + 1
	observation["user_message_time_ms"] = int64(500)
	observation["user_message_time_state"] = "observed"
	observation["user_observed_content_hash"] = prepareOR1CHash(user)
	observation["user_persistence_content_hash"] = prepareOR1CHash(user)
	return req
}

func completeTurnNextHostSignalAcceptanceTestRequest(sid string, turn int, user, assistant string, observedAt int64, correlationID string) dto.M4CompleteTurnRequest {
	return dto.M4CompleteTurnRequest{
		ChatSessionID:    sid,
		TurnIndex:        turn,
		UserInput:        &user,
		AssistantContent: &assistant,
		ClientMeta: map[string]any{
			"source_acceptance_required":            true,
			"archive_center_request_correlation_id": correlationID,
			"source_acceptance_observation": map[string]any{
				"contract_version":                       completeTurnNextHostSignalAcceptanceContract,
				"host_lifecycle_contract_version":        completeTurnRisuHostLifecycleContract,
				"observed_at_ms":                         observedAt,
				"session_id":                             sid,
				"finality_source":                        "risu_next_host_signal_active_chat",
				"finality_state":                         "committed_assistant_observed",
				"host_signal_source":                     "beforeRequest",
				"archive_center_request_correlation_id":  correlationID,
				"request_id_provenance":                  "archive_center_correlation",
				"request_correlation_state":              "matched_before_request_context",
				"request_type":                           "model",
				"response_role":                          "assistant",
				"after_request_content_hash":             prepareOR1CHash(assistant),
				"host_chat_id":                           "chat-1",
				"host_chat_id_state":                     "observed",
				"chat_streaming_state":                   "not_streaming",
				"active_message_count":                   4,
				"message_index":                          2,
				"message_role":                           "char",
				"message_chat_id":                        "assistant-message-1",
				"message_chat_id_state":                  "observed",
				"generation_id":                          "generation-1",
				"generation_id_state":                    "observed",
				"branch_id":                              "",
				"branch_id_state":                        "not_exposed_by_risuai",
				"message_swipe_id":                       -1,
				"message_swipe_id_state":                 "not_present",
				"message_time_ms":                        int64(900),
				"message_time_state":                     "observed",
				"request_message_count":                  2,
				"user_message_index":                     1,
				"user_observed_pair_ordinal":             1,
				"user_message_chat_id":                   "",
				"user_message_chat_id_state":             "unobserved",
				"user_message_time_ms":                   int64(0),
				"user_message_time_state":                "unobserved",
				"user_observed_content_hash":             prepareOR1CHash(user),
				"user_persistence_content_hash":          prepareOR1CHash(user),
				"observed_content_hash":                  prepareOR1CHash(assistant),
				"persistence_content_hash":               prepareOR1CHash(assistant),
				"hash_algorithm":                         "or1c_utf16_djb2.v1",
				"position_observation":                   "committed_before_next_host_signal",
				"later_active_turn_message_count":        1,
				"next_signal_active_role":                "user",
				"next_signal_user_index":                 3,
				"next_signal_user_observed_content_hash": prepareOR1CHash("next user"),
				"message_disabled_state":                 "not_disabled",
				"revision_state":                         "not_exposed_by_risuai",
			},
		},
	}
}

func completeTurnAfterRequestAcceptanceTestRequest(sid string, turn int, user, assistant string, observedAt int64, correlationID string) dto.M4CompleteTurnRequest {
	assistantHash := prepareOR1CHash(sanitizeCriticStorageText(assistant))
	userHash := prepareOR1CHash(strings.TrimSpace(user))
	return dto.M4CompleteTurnRequest{
		ChatSessionID:    sid,
		TurnIndex:        turn,
		UserInput:        &user,
		AssistantContent: &assistant,
		ClientMeta: map[string]any{
			"source_acceptance_required":            true,
			"archive_center_request_correlation_id": correlationID,
			"source_acceptance_observation": map[string]any{
				"contract_version":                      completeTurnAfterRequestAcceptanceContract,
				"host_lifecycle_contract_version":       completeTurnRisuHostLifecycleContract,
				"observed_at_ms":                        observedAt,
				"session_id":                            sid,
				"finality_source":                       "risu_afterRequest",
				"finality_state":                        "received_final_response",
				"host_signal_source":                    "afterRequest",
				"archive_center_request_correlation_id": correlationID,
				"request_id_provenance":                 "archive_center_correlation",
				"request_correlation_state":             "matched_before_request_context",
				"request_type":                          "model",
				"response_role":                         "assistant",
				"after_request_content_hash":            assistantHash,
				"host_chat_id":                          "chat-1",
				"host_chat_id_state":                    "observed_before_request",
				"chat_streaming_state":                  "not_exposed_by_risu_afterRequest",
				"active_message_count":                  0,
				"message_index":                         -1,
				"message_role":                          "",
				"message_chat_id":                       "",
				"message_chat_id_state":                 "not_exposed_by_risu_afterRequest",
				"generation_id":                         "",
				"generation_id_state":                   "not_exposed_by_risu_afterRequest",
				"branch_id":                             "",
				"branch_id_state":                       "not_exposed_by_risuai",
				"message_swipe_id":                      -1,
				"message_swipe_id_state":                "unobserved",
				"message_time_ms":                       int64(0),
				"message_time_state":                    "not_exposed_by_risu_afterRequest",
				"request_message_count":                 2,
				"user_message_index":                    1,
				"user_observed_pair_ordinal":            1,
				"user_message_chat_id":                  "user-message-1",
				"user_message_chat_id_state":            "observed_before_request",
				"user_message_time_ms":                  int64(500),
				"user_message_time_state":               "observed_before_request",
				"user_observed_content_hash":            userHash,
				"user_persistence_content_hash":         userHash,
				"observed_content_hash":                 assistantHash,
				"persistence_content_hash":              assistantHash,
				"hash_algorithm":                        "or1c_utf16_djb2.v1",
				"position_observation":                  "not_exposed_by_risu_afterRequest",
				"later_active_turn_message_count":       0,
				"later_disabled_turn_message_count":     0,
				"later_non_turn_message_count":          0,
				"message_disabled_state":                "not_exposed_by_risu_afterRequest",
				"revision_state":                        "not_exposed_by_risuai",
			},
		},
	}
}

func newCompleteTurnAcceptanceTestServer() *Server {
	return &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeDualShadow},
		Store:             store.NewNoopStore(),
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
	}
}

type logicalTurnTailRejectingStore struct {
	*turnRecordingStore
	replacementCalls int
}

func (s *logicalTurnTailRejectingStore) ReplaceLogicalTurn(_ context.Context, replacement store.LogicalTurnReplacement) error {
	s.replacementCalls++
	s.logicalTurnReplacements = append(s.logicalTurnReplacements, replacement)
	return &store.LogicalTurnReplacementError{
		Code:        "logical_turn_not_current_tail",
		Stage:       "canonical_tail_check",
		Retryable:   false,
		CommitState: "not_committed",
		Cause:       fmt.Errorf("latest turn changed"),
	}
}

func (s *logicalTurnTailRejectingStore) SaveAuditLog(_ context.Context, audit *store.AuditLog) error {
	s.savedAuditLogs = append(s.savedAuditLogs, audit)
	s.auditLogs = append([]store.AuditLog{*audit}, s.auditLogs...)
	return nil
}

func TestCompleteTurnSourceAcceptanceAcceptsCorrelatedAfterRequestFinalResponse(t *testing.T) {
	req := completeTurnAfterRequestAcceptanceTestRequest(
		"session-1", 2, "user", " final answer ", 1000, "archive-request-1",
	)
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted ||
		decision.Reason != "after_request_final_response_observation_accepted" ||
		decision.Revision == "" ||
		decision.LogicalTurnID == "" {
		t.Fatalf("decision=%+v", decision)
	}
	if decision.Observation.MessageIndex != -1 ||
		decision.Observation.MessageChatID != "" ||
		decision.Observation.GenerationID != "" ||
		decision.Observation.BranchID != "" {
		t.Fatalf("afterRequest observation fabricated active-chat identity: %+v", decision.Observation)
	}
	source, err := completeTurnMemorySourceRevision(
		decision, "session-1", decision.BoundTurn, "user", " final answer ", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("completeTurnMemorySourceRevision: %v", err)
	}
	if source == nil || source.SourceMessageID != "" || source.SourceGenerationID != "" || source.BranchID != "" {
		t.Fatalf("afterRequest source fabricated active-chat identity: %+v", source)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsInvalidAfterRequestCorrelationAndContent(t *testing.T) {
	t.Run("correlation", func(t *testing.T) {
		req := completeTurnAfterRequestAcceptanceTestRequest(
			"session-1", 2, "user", "final answer", 1000, "archive-request-1",
		)
		observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
		observation["archive_center_request_correlation_id"] = "different-request"
		decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
		if decision.Accepted || decision.Reason != "source_acceptance_after_request_correlation_mismatch" {
			t.Fatalf("decision=%+v", decision)
		}
	})

	t.Run("after_request_content", func(t *testing.T) {
		req := completeTurnAfterRequestAcceptanceTestRequest(
			"session-1", 2, "user", "final answer", 1000, "archive-request-1",
		)
		observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
		observation["after_request_content_hash"] = prepareOR1CHash("different answer")
		decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
		if decision.Accepted || decision.Reason != "source_acceptance_after_request_content_mismatch" {
			t.Fatalf("decision=%+v", decision)
		}
	})

	t.Run("persistence_content", func(t *testing.T) {
		req := completeTurnAfterRequestAcceptanceTestRequest(
			"session-1", 2, "user", "final answer", 1000, "archive-request-1",
		)
		observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
		observation["persistence_content_hash"] = prepareOR1CHash("different answer")
		decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
		if decision.Accepted || decision.Reason != "source_acceptance_persistence_content_mismatch" {
			t.Fatalf("decision=%+v", decision)
		}
	})

	t.Run("fabricated_active_chat_fact", func(t *testing.T) {
		req := completeTurnAfterRequestAcceptanceTestRequest(
			"session-1", 2, "user", "final answer", 1000, "archive-request-1",
		)
		observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
		observation["message_index"] = 2
		decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
		if decision.Accepted || decision.Reason != "source_acceptance_after_request_active_chat_facts_invalid" {
			t.Fatalf("decision=%+v", decision)
		}
	})
}

func TestCompleteTurnSourceAcceptanceAfterRequestCorrelationOwnsRevisionAndReplacement(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAfterRequestAcceptanceTestRequest(
		"session-1", 2, "same user", "same answer", 1000, "archive-request-1",
	)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted {
		t.Fatalf("first=%+v", firstDecision)
	}
	second := completeTurnAfterRequestAcceptanceTestRequest(
		"session-1", 3, "same user", "same answer", 2000, "archive-request-2",
	)
	secondDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), second)
	if !secondDecision.Accepted ||
		!secondDecision.ReplaceExisting ||
		secondDecision.Revision == firstDecision.Revision ||
		secondDecision.LogicalTurnID != firstDecision.LogicalTurnID ||
		secondDecision.BoundTurn != firstDecision.BoundTurn {
		t.Fatalf("second=%+v first=%+v", secondDecision, firstDecision)
	}
}

func TestCompleteTurnSourceAcceptanceAfterRequestSameCorrelationIsIdempotent(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	req := completeTurnAfterRequestAcceptanceTestRequest(
		"session-1", 2, "same user", "same answer", 1000, "archive-request-1",
	)
	first := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	duplicate := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !first.Accepted ||
		!duplicate.Accepted ||
		duplicate.Reason != "active_final_observation_idempotent" ||
		duplicate.Revision != first.Revision ||
		duplicate.ReplaceExisting {
		t.Fatalf("first=%+v duplicate=%+v", first, duplicate)
	}
}

func TestCompleteTurnSourceAcceptanceAcceptsCorrelatedNextHostSignalCommit(t *testing.T) {
	req := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "user", "final answer", 1000, "archive-request-1")
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted || decision.Reason != "next_host_signal_finality_observation_accepted" || decision.Revision == "" {
		t.Fatalf("decision=%+v", decision)
	}
	if decision.Observation.GenerationID != "generation-1" ||
		decision.Observation.MessageIndex != 2 ||
		decision.Observation.NextSignalUserIndex != 3 ||
		decision.Observation.NextSignalUserContentHash == "" {
		t.Fatalf("next-host-signal commit facts missing: %+v", decision.Observation)
	}
	source, err := completeTurnMemorySourceRevision(decision, "session-1", 2, "user", "final answer", time.Now().UTC())
	if err != nil {
		t.Fatalf("completeTurnMemorySourceRevision: %v", err)
	}
	if source == nil || source.SourceMessageID == "" || source.SourceGenerationID != "generation-1" {
		t.Fatalf("next-host-signal source identity missing: %+v", source)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsAfterRequestCandidateAndInvalidNextSignalProvenance(t *testing.T) {
	req := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "user", "final answer", 1000, "archive-request-1")
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["finality_source"] = "risu_afterRequest"
	observation["finality_state"] = "received_success_response"
	if decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req); decision.Accepted || decision.Reason != "source_acceptance_next_host_signal_lifecycle_invalid" {
		t.Fatalf("afterRequest candidate decision=%+v", decision)
	}

	req = completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "user", "final answer", 1000, "archive-request-1")
	observation = req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["request_id_provenance"] = "risu_request_id"
	if decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req); decision.Accepted || decision.Reason != "source_acceptance_next_host_signal_correlation_invalid" {
		t.Fatalf("invalid provenance decision=%+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceNextHostSignalIsIdempotentAndSupersedesByCorrelation(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "same user", "same answer", 1000, "archive-request-1")
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted {
		t.Fatalf("first=%+v", firstDecision)
	}
	duplicate := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !duplicate.Accepted || duplicate.Reason != "active_final_observation_idempotent" || duplicate.Revision != firstDecision.Revision {
		t.Fatalf("duplicate=%+v first=%+v", duplicate, firstDecision)
	}
	second := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "same user", "same answer", 2000, "archive-request-2")
	secondObservation := second.ClientMeta["source_acceptance_observation"].(map[string]any)
	secondObservation["generation_id"] = "generation-2"
	secondObservation["message_chat_id"] = "assistant-message-2"
	secondObservation["message_time_ms"] = int64(1900)
	secondDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), second)
	if !secondDecision.Accepted || !secondDecision.ReplaceExisting || secondDecision.Revision == firstDecision.Revision {
		t.Fatalf("second=%+v first=%+v", secondDecision, firstDecision)
	}
	stale := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if stale.Accepted || stale.Reason != "source_acceptance_stale_or_superseded" {
		t.Fatalf("stale=%+v", stale)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsNextHostSignalRevisionInvalidatedByDelete(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	req := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "user", "answer", 1000, "archive-request-1")
	decision := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted {
		t.Fatalf("decision=%+v", decision)
	}
	server.invalidateCompleteTurnSourceAcceptances(context.Background(), "session-1", 2, "test_delete", 1500)
	replayed := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	if replayed.Accepted || replayed.Reason != "source_acceptance_deleted_or_rolled_back" {
		t.Fatalf("replayed=%+v", replayed)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsLaterAssistantBeyondNextSignalUser(t *testing.T) {
	req := completeTurnNextHostSignalAcceptanceTestRequest("session-1", 2, "user", "answer", 1000, "archive-request-1")
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["active_message_count"] = 5
	observation["later_active_turn_message_count"] = 2
	if decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req); decision.Accepted || decision.Reason != "source_acceptance_stale_or_superseded" {
		t.Fatalf("later assistant decision=%+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceUsesOfficialActiveChatObservation(t *testing.T) {
	req := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 2, "user", "final answer", 1000,
		"generation-1", "unobserved", 2, 3, 4,
	)
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted || decision.Status != "accepted" || decision.Revision == "" {
		t.Fatalf("decision=%+v, want accepted Go-owned revision", decision)
	}
	if decision.Observation.RevisionState != "not_exposed_by_risuai" {
		t.Fatalf("host revision state=%q, want not_exposed_by_risuai", decision.Observation.RevisionState)
	}
	if decision.Observation.BranchID != "" || decision.Observation.BranchIDState != "not_exposed_by_risuai" {
		t.Fatalf("branch observation=%q/%q, want typed not-exposed", decision.Observation.BranchID, decision.Observation.BranchIDState)
	}
	source, err := completeTurnMemorySourceRevision(decision, "session-1", 2, "user", "final answer", time.Now().UTC())
	if err != nil {
		t.Fatalf("completeTurnMemorySourceRevision: %v", err)
	}
	if source.BranchID != "" || source.BranchState != "not_exposed" {
		t.Fatalf("persisted branch=%q/%q, want not_exposed without invented identity", source.BranchID, source.BranchState)
	}
}

func TestValidateCompleteTurnSourceObservationAcceptsOfficialOutputV1Snapshot(t *testing.T) {
	const (
		sid       = "char_0_cid_fixture-chat-a"
		user      = "A input"
		assistant = "A output"
	)
	req := dto.M4CompleteTurnRequest{
		ChatSessionID:    sid,
		UserInput:        stringPointer(user),
		AssistantContent: stringPointer(assistant),
	}
	observation := completeTurnSourceObservation{
		ContractVersion:              completeTurnSourceAcceptanceContract,
		HostLifecycleContractVersion: completeTurnRisuHostLifecycleContract,
		ObservedAtMS:                 1000,
		SessionID:                    sid,
		FinalitySource:               "risu_output",
		FinalityState:                "committed_assistant_observed",
		HostSignalSource:             "output",
		ArchiveCenterCorrelationID:   "fixture-request-a",
		RequestIDProvenance:          "archive_center_correlation",
		RequestCorrelationState:      "matched_before_request_context",
		RequestType:                  "model",
		ResponseRole:                 "assistant",
		HostChatID:                   "fixture-chat-a",
		HostChatIDState:              "observed",
		ChatStreamingState:           "not_streaming",
		ActiveMessageCount:           2,
		MessageIndex:                 1,
		MessageRole:                  "char",
		MessageChatID:                "assistant-a",
		MessageChatIDState:           "observed",
		GenerationID:                 "generation-a",
		GenerationIDState:            "observed",
		BranchIDState:                "not_exposed_by_risuai",
		MessageSwipeID:               -1,
		MessageSwipeIDState:          "not_present",
		MessageTimeMS:                900,
		MessageTimeState:             "observed",
		UserMessageIndex:             0,
		UserMessageChatID:            "user-a",
		UserMessageChatIDState:       "observed_before_request",
		UserMessageTimeMS:            800,
		UserMessageTimeState:         "observed_before_request",
		UserObservedContentHash:      prepareOR1CHash(user),
		UserPersistenceContentHash:   prepareOR1CHash(user),
		ObservedContentHash:          prepareOR1CHash(assistant),
		PersistenceContentHash:       prepareOR1CHash(sanitizeCriticStorageText(assistant)),
		HashAlgorithm:                "or1c_utf16_djb2.v1",
		PositionObservation:          "current_active_chat_tail",
		MessageDisabledState:         "not_disabled",
		RevisionState:                "not_exposed_by_risuai",
	}

	decision := validateCompleteTurnSourceObservation(req, observation)
	if !decision.Accepted || decision.Reason != "active_final_observation_accepted" {
		t.Fatalf("decision=%+v, want official output v1 observation accepted", decision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsBranchIdentityWithoutObservedState(t *testing.T) {
	req := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 1, "user", "answer", 1000,
		"generation-1", "not_streaming", 0, 1, 2,
	)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["branch_id"] = "invented-branch"
	observation["branch_id_state"] = "not_exposed_by_risuai"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_branch_identity_state_invalid" {
		t.Fatalf("invented branch identity must be rejected: %+v", decision)
	}
}

func TestCompleteTurnSourceAcceptanceCarriesExplicitObservedBranchIdentity(t *testing.T) {
	req := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 1, "user", "answer", 1000,
		"generation-1", "not_streaming", 0, 1, 2,
	)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["branch_id"] = "official-host-branch"
	observation["branch_id_state"] = "observed"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted {
		t.Fatalf("explicit observed branch decision=%+v", decision)
	}
	source, err := completeTurnMemorySourceRevision(decision, "session-1", 1, "user", "answer", time.Now().UTC())
	if err != nil {
		t.Fatalf("completeTurnMemorySourceRevision: %v", err)
	}
	if source.BranchID != "official-host-branch" || source.BranchState != "observed" {
		t.Fatalf("persisted branch=%q/%q", source.BranchID, source.BranchState)
	}
}

func TestCompleteTurnSourceAcceptanceClassifiesOfficialMessageEditAndReroll(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 1, "same user", "first", 1000,
		"generation-1", "not_streaming", 0, 1, 2,
	)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted {
		t.Fatalf("first=%+v", firstDecision)
	}

	edit := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 2, "same user", "edited", 2000,
		"generation-1", "not_streaming", 0, 1, 2,
	)
	editDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), edit)
	if !editDecision.Accepted || !editDecision.ReplaceExisting ||
		editDecision.ReplacementKind != "host_observed_edit" ||
		editDecision.BoundTurn != firstDecision.BoundTurn {
		t.Fatalf("edit=%+v first=%+v", editDecision, firstDecision)
	}

	reroll := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 3, "same user", "rerolled", 3000,
		"generation-2", "not_streaming", 0, 1, 2,
	)
	rerollDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !rerollDecision.Accepted || !rerollDecision.ReplaceExisting ||
		rerollDecision.ReplacementKind != "host_observed_reroll" ||
		rerollDecision.BoundTurn != firstDecision.BoundTurn {
		t.Fatalf("reroll=%+v first=%+v", rerollDecision, firstDecision)
	}
}

func TestCompleteTurnSourceAcceptanceClassifiesPocketRisuSwipeAsReroll(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 1, "same user", "second swipe", 1000,
		"generation-2", "not_streaming", 0, 1, 2,
	)
	firstObservation := first.ClientMeta["source_acceptance_observation"].(map[string]any)
	firstObservation["message_swipe_id"] = 1
	firstObservation["message_swipe_id_state"] = "observed"
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted {
		t.Fatalf("first=%+v", firstDecision)
	}

	previousSwipe := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 2, "same user", "first swipe", 2000,
		"generation-2", "not_streaming", 0, 1, 2,
	)
	previousSwipeObservation := previousSwipe.ClientMeta["source_acceptance_observation"].(map[string]any)
	previousSwipeObservation["message_swipe_id"] = 0
	previousSwipeObservation["message_swipe_id_state"] = "observed"
	previousSwipeDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), previousSwipe)
	if !previousSwipeDecision.Accepted || !previousSwipeDecision.ReplaceExisting ||
		previousSwipeDecision.ReplacementKind != "host_observed_reroll" ||
		previousSwipeDecision.BoundTurn != firstDecision.BoundTurn {
		t.Fatalf("PocketRisu swipe replacement=%+v first=%+v", previousSwipeDecision, firstDecision)
	}

	editedSwipe := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 3, "same user", "manually edited first swipe", 3000,
		"generation-2", "not_streaming", 0, 1, 2,
	)
	editedSwipeObservation := editedSwipe.ClientMeta["source_acceptance_observation"].(map[string]any)
	editedSwipeObservation["message_swipe_id"] = 0
	editedSwipeObservation["message_swipe_id_state"] = "observed"
	editedSwipeDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), editedSwipe)
	if !editedSwipeDecision.Accepted || !editedSwipeDecision.ReplaceExisting ||
		editedSwipeDecision.ReplacementKind != "host_observed_edit" {
		t.Fatalf("same PocketRisu swipe content edit=%+v", editedSwipeDecision)
	}
}

func TestCompleteTurnSourceAcceptanceRejectsInvalidSwipeObservation(t *testing.T) {
	req := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 1, "user", "answer", 1000,
		"generation-1", "not_streaming", 0, 1, 2,
	)
	observation := req.ClientMeta["source_acceptance_observation"].(map[string]any)
	observation["message_swipe_id"] = -1
	observation["message_swipe_id_state"] = "observed"
	decision := newCompleteTurnAcceptanceTestServer().beginCompleteTurnSourceAcceptance(context.Background(), req)
	if decision.Accepted || decision.Reason != "source_acceptance_swipe_identity_missing" {
		t.Fatalf("invalid swipe observation must be rejected: %+v", decision)
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

func TestCompleteTurnSourceAcceptanceSupersedesByteIdenticalNewGeneration(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 2, "user", "same answer", 1000,
		"generation-1", "not_streaming", 2, 3, 4,
	)
	second := completeTurnAnchoredAcceptanceTestRequest(
		"session-1", 2, "user", "same answer", 2000,
		"generation-2", "not_streaming", 2, 3, 4,
	)
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted || firstDecision.ReplaceExisting {
		t.Fatalf("first decision=%+v", firstDecision)
	}
	secondDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), second)
	if !secondDecision.Accepted || !secondDecision.ReplaceExisting ||
		secondDecision.Previous != firstDecision.Revision ||
		secondDecision.Revision == firstDecision.Revision {
		t.Fatalf("second decision=%+v first=%+v", secondDecision, firstDecision)
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
	if _, _, found := server.CompleteTurns.status("candidate-key"); found {
		t.Fatal("candidate rejection polluted complete-turn idempotency cache")
	}
}

func TestCompleteTurnSourceAcceptanceSupersedeCancelsOlderWorker(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	firstReq := completeTurnAcceptanceTestRequest("session-1", 2, "first", 1000, "or1c_first", "generation-1", "not_streaming", "current_active_chat_tail", 3, 4)
	first := server.beginCompleteTurnSourceAcceptance(context.Background(), firstReq)
	workerCtx, release := server.completeTurnSourceAcceptanceProcessingContext(context.Background(), first, "session-1", 2)
	secondReq := completeTurnAcceptanceTestRequest("session-1", 2, "second", 2000, "or1c_second", "generation-2", "not_streaming", "current_active_chat_tail", 3, 4)
	secondDone := make(chan completeTurnSourceAcceptanceDecision, 1)
	go func() {
		secondDone <- server.beginCompleteTurnSourceAcceptance(context.Background(), secondReq)
	}()
	select {
	case <-workerCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("older complete-turn worker context was not canceled by a newer revision")
	}
	select {
	case second := <-secondDone:
		t.Fatalf("newer revision returned before the canceled worker drained: %+v", second)
	default:
	}
	release()
	select {
	case second := <-secondDone:
		if !second.Accepted {
			t.Fatalf("second=%+v", second)
		}
	case <-time.After(time.Second):
		t.Fatal("newer revision did not resume after the canceled worker drained")
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

func TestCompleteTurnSourceAcceptanceEditedSameUserMessageReplacesLogicalTurn(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAfterRequestAcceptanceTestRequest("session-1", 1, "original user", "first", 1000, "request-1")
	firstObservation := first.ClientMeta["source_acceptance_observation"].(map[string]any)
	firstObservation["user_message_chat_id"] = "stable-user-message-id"
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted || firstDecision.ReplaceExisting {
		t.Fatalf("first decision=%+v", firstDecision)
	}

	edited := completeTurnAfterRequestAcceptanceTestRequest("session-1", 2, "original user with added instruction", "regenerated", 2000, "request-2")
	editedObservation := edited.ClientMeta["source_acceptance_observation"].(map[string]any)
	editedObservation["user_message_chat_id"] = "stable-user-message-id"
	editedDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), edited)
	if !editedDecision.Accepted || !editedDecision.ReplaceExisting || editedDecision.BoundTurn != 1 || editedDecision.LogicalTurnID != firstDecision.LogicalTurnID {
		t.Fatalf("edited same Host user row appended instead of replacing: first=%+v edited=%+v", firstDecision, editedDecision)
	}
}

func TestCompleteTurnSourceAcceptanceNewUserMessageWithIdenticalTextAppends(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAfterRequestAcceptanceTestRequest("session-1", 1, "identical user text", "first", 1000, "request-1")
	firstObservation := first.ClientMeta["source_acceptance_observation"].(map[string]any)
	firstObservation["user_message_chat_id"] = "user-message-1"
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)
	if !firstDecision.Accepted || firstDecision.ReplaceExisting {
		t.Fatalf("first decision=%+v", firstDecision)
	}

	next := completeTurnAfterRequestAcceptanceTestRequest("session-1", 2, "identical user text", "next", 2000, "request-2")
	nextObservation := next.ClientMeta["source_acceptance_observation"].(map[string]any)
	nextObservation["request_message_count"] = 4
	nextObservation["user_message_index"] = 3
	nextObservation["user_observed_pair_ordinal"] = 2
	nextObservation["user_message_chat_id"] = "user-message-2"
	nextObservation["user_message_time_ms"] = int64(1500)
	nextDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), next)
	if !nextDecision.Accepted || nextDecision.ReplaceExisting || nextDecision.BoundTurn != 2 || nextDecision.LogicalTurnID == firstDecision.LogicalTurnID {
		t.Fatalf("new Host user row with identical text did not append: first=%+v next=%+v", firstDecision, nextDecision)
	}
}

func TestCompleteTurnSourceAcceptanceAfterRequestReceivesObservedPairOrdinal(t *testing.T) {
	req := completeTurnAfterRequestAcceptanceTestRequest("session-1", 1, "user", "answer", 1000, "request-1")
	observation, err := completeTurnSourceObservationFromMeta(req.ClientMeta)
	if err != nil {
		t.Fatal(err)
	}
	if observation.UserObservedPairOrdinal != 1 {
		t.Fatalf("user_observed_pair_ordinal was not decoded: %+v", observation)
	}
}

func TestCompleteTurnSourceAcceptanceWithoutChatIDUsesObservedAnchorCoordinates(t *testing.T) {
	server := newCompleteTurnAcceptanceTestServer()
	first := completeTurnAfterRequestAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "request-1")
	firstObservation := first.ClientMeta["source_acceptance_observation"].(map[string]any)
	firstObservation["user_message_chat_id"] = ""
	firstObservation["user_message_chat_id_state"] = "unobserved"
	firstDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), first)

	reroll := completeTurnAfterRequestAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "request-2")
	rerollObservation := reroll.ClientMeta["source_acceptance_observation"].(map[string]any)
	rerollObservation["user_message_chat_id"] = ""
	rerollObservation["user_message_chat_id_state"] = "unobserved"
	rerollDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !firstDecision.Accepted || !rerollDecision.Accepted || !rerollDecision.ReplaceExisting || rerollDecision.BoundTurn != 1 || rerollDecision.LogicalTurnID != firstDecision.LogicalTurnID {
		t.Fatalf("unobserved chatId anchor mismatch: first=%+v reroll=%+v", firstDecision, rerollDecision)
	}
}

func TestCompleteTurnNonCommittedReplacementFailureIsTerminalAndDoesNotReenter(t *testing.T) {
	base := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 1, Role: "user", Content: "same user"},
		{ChatSessionID: "session-1", TurnIndex: 1, Role: "assistant", Content: "first"},
	}}
	storage := &logicalTurnTailRejectingStore{turnRecordingStore: base}
	server := &Server{
		Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(), CompleteTurns: newCompleteTurnRequestLedger(),
		TurnWorkflows: newTurnWorkflowHUDLedger(),
	}
	original := completeTurnAfterRequestAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "request-1")
	originalDecision := server.beginCompleteTurnSourceAcceptance(context.Background(), original)
	if !originalDecision.Accepted {
		t.Fatalf("original decision=%+v", originalDecision)
	}

	reroll := completeTurnAfterRequestAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "request-2")
	reroll.ClientMeta["idempotency_key"] = "reroll-attempt-1"
	body, err := json.Marshal(reroll)
	if err != nil {
		t.Fatal(err)
	}
	firstResponse := httptest.NewRecorder()
	server.handleCompleteTurn(firstResponse, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(body)))
	if firstResponse.Code != http.StatusOK || storage.replacementCalls != 1 {
		t.Fatalf("first failure status=%d calls=%d body=%s", firstResponse.Code, storage.replacementCalls, firstResponse.Body.String())
	}
	var firstPayload map[string]any
	if err := json.Unmarshal(firstResponse.Body.Bytes(), &firstPayload); err != nil {
		t.Fatal(err)
	}
	if firstPayload["code"] != "logical_turn_not_current_tail" || firstPayload["retryable"] != false || firstPayload["commit_state"] != "not_committed" || firstPayload["queue_action"] != "discard" {
		t.Fatalf("terminal failure payload=%#v", firstPayload)
	}
	failureAcceptance := mapFromAny(firstPayload["source_acceptance"])
	if failureAcceptance["lifecycle"] != "replacement_failed" || failureAcceptance["replacement_status"] != "terminal_failure" || failureAcceptance["accepted"] != false {
		t.Fatalf("terminal source acceptance payload=%#v", failureAcceptance)
	}
	if len(base.savedChatLogs) != 0 || len(base.savedMemories) != 0 || len(base.savedEvidence) != 0 || len(base.savedKGTriples) != 0 {
		t.Fatalf("rejected replacement mutated canonical/derived state: chats=%d memories=%d evidence=%d kg=%d", len(base.savedChatLogs), len(base.savedMemories), len(base.savedEvidence), len(base.savedKGTriples))
	}

	reroll.ClientMeta["idempotency_key"] = "reroll-attempt-2"
	body, err = json.Marshal(reroll)
	if err != nil {
		t.Fatal(err)
	}
	secondResponse := httptest.NewRecorder()
	server.handleCompleteTurn(secondResponse, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(body)))
	if secondResponse.Code != http.StatusOK || storage.replacementCalls != 1 {
		t.Fatalf("terminal revision re-entered replacement: status=%d calls=%d body=%s", secondResponse.Code, storage.replacementCalls, secondResponse.Body.String())
	}
	if len(base.savedChatLogs) != 0 || len(base.savedMemories) != 0 {
		t.Fatalf("terminal replay mutated state: chats=%d memories=%d", len(base.savedChatLogs), len(base.savedMemories))
	}

	restarted := &Server{
		Cfg: config.Config{StoreMode: config.StoreModeMariaDBAuthority}, Store: storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(), CompleteTurns: newCompleteTurnRequestLedger(),
		TurnWorkflows: newTurnWorkflowHUDLedger(),
	}
	reroll.ClientMeta["idempotency_key"] = "reroll-attempt-after-restart"
	body, err = json.Marshal(reroll)
	if err != nil {
		t.Fatal(err)
	}
	restartResponse := httptest.NewRecorder()
	restarted.handleCompleteTurn(restartResponse, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(body)))
	if restartResponse.Code != http.StatusOK || storage.replacementCalls != 1 {
		t.Fatalf("durable terminal revision re-entered replacement: status=%d calls=%d body=%s", restartResponse.Code, storage.replacementCalls, restartResponse.Body.String())
	}
	state := restarted.SourceAcceptances.current[sourceAcceptanceStateKey("session-1", 1)]
	if state.Revision != originalDecision.Revision || state.ReplacementStatus == "pending" {
		t.Fatalf("previous active state was not restored after restart: original=%+v restored=%+v", originalDecision, state)
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

func TestCompleteTurnSourceAcceptanceAppendsLowerObservedIndexAfterCanonicalTail(t *testing.T) {
	storage := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "user", Content: "persisted user"},
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "assistant", Content: "persisted assistant"},
	}}
	server := &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store:             storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
	}
	req := completeTurnAnchoredAcceptanceTestRequest("session-1", 35, "actual user", "actual assistant", 2000, "generation-35", "not_streaming", 68, 69, 70)
	decision := server.beginCompleteTurnSourceAcceptance(context.Background(), req)
	if !decision.Accepted || decision.BoundTurn != 52 || decision.ReplaceExisting {
		t.Fatalf("lower Host index must remain an observation while the new DB turn appends after the canonical tail: %+v", decision)
	}
}

func TestCompleteTurnPersistsLowerHostIndexAtCanonicalDBNextTurn(t *testing.T) {
	storage := &turnRecordingStore{returnChatLogs: []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "user", Content: "persisted user"},
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "assistant", Content: "persisted assistant"},
	}}
	server := &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store:             storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
		CompleteTurns:     newCompleteTurnRequestLedger(),
	}
	req := completeTurnAnchoredAcceptanceTestRequest("session-1", 35, "actual user", "actual assistant", 2000, "generation-35", "not_streaming", 68, 69, 70)
	req.ClientMeta["idempotency_key"] = "session-1-lower-host-index"
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleCompleteTurn(recorder, httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(storage.savedChatLogs) != 2 || storage.savedChatLogs[0].TurnIndex != 52 || storage.savedChatLogs[1].TurnIndex != 52 {
		t.Fatalf("canonical raw turn was not persisted at DB tail + 1: %+v", storage.savedChatLogs)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["turn_index"] != float64(52) || response["save_ok"] != true {
		t.Fatalf("response=%#v", response)
	}
}

func TestCompleteTurnSourceAcceptanceKeepsExistingLogicalTurnWhenDBTailIsAhead(t *testing.T) {
	storage := &turnRecordingStore{}
	server := &Server{
		Cfg:               config.Config{StoreMode: config.StoreModeMariaDBAuthority},
		Store:             storage,
		SourceAcceptances: newCompleteTurnSourceAcceptanceLedger(),
	}
	original := completeTurnAnchoredAcceptanceTestRequest("session-1", 35, "same user", "first answer", 1000, "generation-1", "not_streaming", 68, 69, 70)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), original); !decision.Accepted || decision.BoundTurn != 35 {
		t.Fatalf("original decision=%+v", decision)
	}
	storage.returnChatLogs = []store.ChatLog{
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "user", Content: "persisted user"},
		{ChatSessionID: "session-1", TurnIndex: 51, Role: "assistant", Content: "persisted assistant"},
	}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 35, "same user", "rerolled answer", 2000, "generation-2", "not_streaming", 68, 69, 70)
	decision := server.beginCompleteTurnSourceAcceptance(context.Background(), reroll)
	if !decision.Accepted || decision.BoundTurn != 35 || !decision.ReplaceExisting {
		t.Fatalf("existing logical turn must keep its canonical DB turn while rerolling: %+v", decision)
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
		TurnWorkflows: newTurnWorkflowHUDLedger(),
	}
	first := completeTurnAnchoredAcceptanceTestRequest("session-1", 1, "same user", "first", 1000, "generation-1", "not_streaming", 2, 3, 4)
	if decision := server.beginCompleteTurnSourceAcceptance(context.Background(), first); !decision.Accepted {
		t.Fatalf("first decision=%+v", decision)
	}
	reroll := completeTurnAnchoredAcceptanceTestRequest("session-1", 2, "same user", "rerolled", 2000, "generation-2", "not_streaming", 2, 3, 4)
	reroll.ClientMeta["idempotency_key"] = "session-1-turn-1-reroll-generation-2"
	reroll.ClientMeta["turn_workflow_request_id"] = "reroll-replacement-hud"
	server.TurnWorkflows.begin("reroll-replacement-hud", "session-1", 1)
	reroll.ClientMeta["critic"] = map[string]any{
		"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "timeout_ms": 90000,
	}
	extractionBytes := []byte(criticWireJSONForTest(map[string]any{
		"turn_summary":      "The corrected assistant output became the canonical memory.",
		"importance_score":  7,
		"evidence_excerpts": []any{"rerolled"},
		"kg_triples":        []any{map[string]any{"subject": "Assistant", "predicate": "replaces", "object": "Old output"}},
	}))
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
	if vectors.deleteDocumentCalls != 0 || len(vectors.deletedDocumentIDs) != 0 {
		t.Fatalf("replacement must queue canonical outbox deletion instead of pre-deleting vectors: calls=%d ids=%v", vectors.deleteDocumentCalls, vectors.deletedDocumentIDs)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["critic_triggered"] != true || len(storage.savedMemories) != 1 || criticCalls != 1 {
		t.Fatalf("replacement did not regenerate derived artifacts: critic=%v memories=%d calls=%d response=%v", response["critic_triggered"], len(storage.savedMemories), criticCalls, response)
	}
	sourceAcceptance, _ := response["source_acceptance"].(map[string]any)
	if sourceAcceptance["accepted"] != true || sourceAcceptance["replace_existing"] != true || sourceAcceptance["lifecycle"] != "active_final" {
		t.Fatalf("replacement source acceptance=%#v", sourceAcceptance)
	}
	hud, _ := response["turn_workflow_hud"].(map[string]any)
	if hud["display_mode"] != "notice" || hud["status"] != "completed_with_warning" || hud["severity"] != "warning" ||
		hud["title_key"] != "turn_hud.notice.reroll_confirmed" ||
		hud["message_key"] != "turn_hud.notice.reroll_confirmed_detail" ||
		hud["notice_code"] != "LOGICAL_TURN_REPLACED" {
		t.Fatalf("replacement HUD notice=%#v", hud)
	}
	repeated := httptest.NewRecorder()
	server.handleCompleteTurn(repeated, httptest.NewRequest("POST", "/complete-turn", bytes.NewReader(body)))
	if repeated.Code != 200 || len(storage.logicalTurnReplacements) != 1 || vectors.deleteDocumentCalls != 0 || criticCalls != 1 {
		t.Fatalf("repeat status=%d replacements=%d vector deletes=%d critic calls=%d body=%s", repeated.Code, len(storage.logicalTurnReplacements), vectors.deleteDocumentCalls, criticCalls, repeated.Body.String())
	}
}

func TestCompleteTurnSourceWorkerWaitHasExplicitDeadline(t *testing.T) {
	original := completeTurnSourceWorkerStopTimeout
	completeTurnSourceWorkerStopTimeout = 10 * time.Millisecond
	defer func() { completeTurnSourceWorkerStopTimeout = original }()
	blocked := make(chan struct{})
	started := time.Now()
	err := waitForCompleteTurnSourceWorkers([]<-chan struct{}{blocked})
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("worker wait err=%v elapsed=%s", err, time.Since(started))
	}
}
