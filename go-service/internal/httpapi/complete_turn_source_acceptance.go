package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	completeTurnSourceAcceptanceContract         = "source_acceptance_observation.v1"
	completeTurnNextHostSignalAcceptanceContract = "source_acceptance_observation.v2"
	completeTurnRisuHostLifecycleContract        = "risu_host_lifecycle_observation.v1"
	completeTurnSourceLifecycleContract          = "source_acceptance_lifecycle.v1"
	sourceAcceptanceTransitionEvent              = "source_acceptance_transition"
	sourceAcceptanceInvalidationEvent            = "source_acceptance_invalidation"
)

type completeTurnSourceObservation struct {
	ContractVersion               string `json:"contract_version"`
	HostLifecycleContractVersion  string `json:"host_lifecycle_contract_version"`
	ObservedAtMS                  int64  `json:"observed_at_ms"`
	SessionID                     string `json:"session_id"`
	FinalitySource                string `json:"finality_source"`
	FinalityState                 string `json:"finality_state"`
	HostSignalSource              string `json:"host_signal_source"`
	ArchiveCenterCorrelationID    string `json:"archive_center_request_correlation_id"`
	RequestIDProvenance           string `json:"request_id_provenance"`
	RequestCorrelationState       string `json:"request_correlation_state"`
	RequestType                   string `json:"request_type"`
	ResponseRole                  string `json:"response_role"`
	AfterRequestContentHash       string `json:"after_request_content_hash"`
	HostChatID                    string `json:"host_chat_id"`
	HostChatIDState               string `json:"host_chat_id_state"`
	ChatStreamingState            string `json:"chat_streaming_state"`
	ActiveMessageCount            int    `json:"active_message_count"`
	MessageIndex                  int    `json:"message_index"`
	MessageRole                   string `json:"message_role"`
	MessageChatID                 string `json:"message_chat_id"`
	MessageChatIDState            string `json:"message_chat_id_state"`
	GenerationID                  string `json:"generation_id"`
	GenerationIDState             string `json:"generation_id_state"`
	MessageTimeMS                 int64  `json:"message_time_ms"`
	MessageTimeState              string `json:"message_time_state"`
	UserMessageIndex              int    `json:"user_message_index"`
	UserMessageChatID             string `json:"user_message_chat_id"`
	UserMessageChatIDState        string `json:"user_message_chat_id_state"`
	UserMessageTimeMS             int64  `json:"user_message_time_ms"`
	UserMessageTimeState          string `json:"user_message_time_state"`
	UserObservedContentHash       string `json:"user_observed_content_hash"`
	UserPersistenceContentHash    string `json:"user_persistence_content_hash"`
	RequestMessageCount           int    `json:"request_message_count"`
	ObservedContentHash           string `json:"observed_content_hash"`
	PersistenceContentHash        string `json:"persistence_content_hash"`
	HashAlgorithm                 string `json:"hash_algorithm"`
	PositionObservation           string `json:"position_observation"`
	LaterActiveTurnMessageCount   int    `json:"later_active_turn_message_count"`
	LaterDisabledTurnMessageCount int    `json:"later_disabled_turn_message_count"`
	LaterNonTurnMessageCount      int    `json:"later_non_turn_message_count"`
	NextSignalActiveRole          string `json:"next_signal_active_role"`
	NextSignalUserIndex           int    `json:"next_signal_user_index"`
	NextSignalUserContentHash     string `json:"next_signal_user_observed_content_hash"`
	MessageDisabledState          string `json:"message_disabled_state"`
	RevisionState                 string `json:"revision_state"`
}

type completeTurnSourceAcceptanceState struct {
	SessionID         string `json:"session_id"`
	TurnIndex         int    `json:"turn_index"`
	Revision          string `json:"revision"`
	GenerationID      string `json:"generation_id,omitempty"`
	HostChatID        string `json:"host_chat_id,omitempty"`
	ContentHash       string `json:"content_hash"`
	ObservedAtMS      int64  `json:"observed_at_ms"`
	Lifecycle         string `json:"lifecycle"`
	LogicalTurnID     string `json:"logical_turn_id,omitempty"`
	ReplacementStatus string `json:"replacement_status,omitempty"`
}

type completeTurnSourceAcceptanceDecision struct {
	Enabled         bool
	Accepted        bool
	Status          string
	Reason          string
	Retryable       bool
	QueueAction     string
	Revision        string
	Previous        string
	BoundTurn       int
	LogicalTurnID   string
	ReplaceExisting bool
	Observation     completeTurnSourceObservation
}

type sourceAcceptanceInvalidation struct {
	FromTurn     int   `json:"from_turn"`
	ObservedAtMS int64 `json:"observed_at_ms"`
}

type completeTurnSourceAcceptanceWorker struct {
	revision string
	cancel   context.CancelFunc
	done     chan struct{}
}

type completeTurnSourceReprocessingWorker struct {
	sessionID string
	turnIndex int
	cancel    context.CancelFunc
	done      chan struct{}
}

type completeTurnSourceAcceptanceLedger struct {
	mu                  sync.Mutex
	current             map[string]completeTurnSourceAcceptanceState
	invalidations       map[string]sourceAcceptanceInvalidation
	workers             map[string]completeTurnSourceAcceptanceWorker
	reprocessingWorkers map[string]*completeTurnSourceReprocessingWorker
}

func newCompleteTurnSourceAcceptanceLedger() *completeTurnSourceAcceptanceLedger {
	return &completeTurnSourceAcceptanceLedger{
		current:             map[string]completeTurnSourceAcceptanceState{},
		invalidations:       map[string]sourceAcceptanceInvalidation{},
		workers:             map[string]completeTurnSourceAcceptanceWorker{},
		reprocessingWorkers: map[string]*completeTurnSourceReprocessingWorker{},
	}
}

func completeTurnSourceAcceptanceRequired(meta map[string]any) bool {
	return completeTurnBoolFromAny(meta["source_acceptance_required"])
}

func completeTurnSourceObservationFromMeta(meta map[string]any) (completeTurnSourceObservation, error) {
	var observation completeTurnSourceObservation
	raw, ok := meta["source_acceptance_observation"]
	if !ok || raw == nil {
		return observation, fmt.Errorf("source_acceptance_observation_missing")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return observation, fmt.Errorf("source_acceptance_observation_malformed")
	}
	if err := json.Unmarshal(encoded, &observation); err != nil {
		return observation, fmt.Errorf("source_acceptance_observation_malformed")
	}
	return observation, nil
}

func completeTurnSourceRevision(sid string, turnIndex int, observation completeTurnSourceObservation) string {
	if observation.ContractVersion == completeTurnNextHostSignalAcceptanceContract {
		seed := strings.Join([]string{
			sid,
			strconv.Itoa(turnIndex),
			observation.ContractVersion,
			observation.HostLifecycleContractVersion,
			observation.FinalitySource,
			observation.HostSignalSource,
			observation.ArchiveCenterCorrelationID,
			observation.RequestIDProvenance,
			observation.RequestCorrelationState,
			observation.HostChatID,
			strconv.Itoa(observation.UserMessageIndex),
			observation.UserMessageChatID,
			strconv.FormatInt(observation.UserMessageTimeMS, 10),
			observation.UserObservedContentHash,
			observation.MessageChatID,
			observation.GenerationID,
			strconv.FormatInt(observation.MessageTimeMS, 10),
			strconv.Itoa(observation.MessageIndex),
			observation.PersistenceContentHash,
		}, "\x1f")
		return "sar_" + strings.TrimPrefix(prepareOR1CHash(seed), "or1c_")
	}
	seed := strings.Join([]string{
		sid,
		strconv.Itoa(turnIndex),
		observation.HostChatID,
		observation.MessageChatID,
		observation.GenerationID,
		strconv.FormatInt(observation.MessageTimeMS, 10),
		strconv.Itoa(observation.MessageIndex),
		observation.ObservedContentHash,
		observation.PersistenceContentHash,
	}, "\x1f")
	return "sar_" + strings.TrimPrefix(prepareOR1CHash(seed), "or1c_")
}

func completeTurnLogicalTurnID(sid string, observation completeTurnSourceObservation) string {
	chatIdentity := strings.TrimSpace(observation.HostChatID)
	if (observation.HostChatIDState != "observed" && observation.HostChatIDState != "observed_before_request") || chatIdentity == "" {
		chatIdentity = sid
	}
	userIdentity := strings.TrimSpace(observation.UserMessageChatID)
	if (observation.UserMessageChatIDState != "observed" && observation.UserMessageChatIDState != "observed_before_request") || userIdentity == "" {
		userIdentity = strings.Join([]string{
			strconv.Itoa(observation.UserMessageIndex),
			strconv.FormatInt(observation.UserMessageTimeMS, 10),
			observation.UserObservedContentHash,
		}, "\x1f")
	}
	seed := strings.Join([]string{sid, chatIdentity, userIdentity}, "\x1f")
	return "lt_" + strings.TrimPrefix(prepareOR1CHash(seed), "or1c_")
}

func rejectedCompleteTurnSourceAcceptance(reason string, retryable bool, observation completeTurnSourceObservation) completeTurnSourceAcceptanceDecision {
	queueAction := "discard"
	if retryable {
		queueAction = "retry_after_new_observation"
	}
	return completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: false, Status: "rejected", Reason: reason,
		Retryable: retryable, QueueAction: queueAction, Observation: observation,
	}
}

func validateCompleteTurnSourceObservation(req dto.M4CompleteTurnRequest, observation completeTurnSourceObservation) completeTurnSourceAcceptanceDecision {
	sid := strings.TrimSpace(req.ChatSessionID)
	if observation.ContractVersion != completeTurnSourceAcceptanceContract &&
		observation.ContractVersion != completeTurnNextHostSignalAcceptanceContract {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_contract_incompatible", false, observation)
	}
	if observation.SessionID != sid || observation.ObservedAtMS <= 0 {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_identity_missing_or_mismatch", false, observation)
	}
	if observation.ContractVersion == completeTurnNextHostSignalAcceptanceContract {
		return validateCompleteTurnNextHostSignalObservation(req, observation)
	}
	if observation.ChatStreamingState == "streaming" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_streaming_candidate", true, observation)
	}
	if observation.ChatStreamingState != "not_streaming" && observation.ChatStreamingState != "unobserved" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_streaming_observation_invalid", false, observation)
	}
	backfillObservation := req.ClientMeta["active_chat_backfill"] != nil
	positionAccepted := observation.PositionObservation == "current_active_chat_tail" && observation.MessageIndex == observation.ActiveMessageCount-1
	positionAccepted = positionAccepted || (observation.PositionObservation == "current_active_assistant_tail" && observation.LaterActiveTurnMessageCount == 0)
	if backfillObservation {
		positionAccepted = positionAccepted || observation.PositionObservation == "current_active_chat_message"
	}
	if !positionAccepted || observation.ActiveMessageCount <= 0 || observation.MessageIndex < 0 || observation.MessageIndex >= observation.ActiveMessageCount {
		if !backfillObservation && observation.LaterActiveTurnMessageCount > 0 {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_stale_or_superseded", false, observation)
		}
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_not_current_active_tail", true, observation)
	}
	if observation.MessageRole != "char" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_not_assistant_message", false, observation)
	}
	if observation.MessageDisabledState == "disabled" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_assistant_message_disabled", false, observation)
	}
	if observation.MessageDisabledState != "" && observation.MessageDisabledState != "unobserved" && observation.MessageDisabledState != "not_disabled" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_message_visibility_observation_invalid", false, observation)
	}
	if observation.HashAlgorithm != "or1c_utf16_djb2.v1" || observation.ObservedContentHash == "" || observation.PersistenceContentHash == "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_content_observation_missing", false, observation)
	}
	if req.AssistantContent == nil {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_persistence_content_missing", false, observation)
	}
	userAnchorReported := observation.UserObservedContentHash != "" || observation.UserPersistenceContentHash != "" || observation.UserMessageChatID != ""
	if userAnchorReported {
		if observation.UserMessageIndex < 0 || observation.UserMessageIndex >= observation.MessageIndex || observation.UserObservedContentHash == "" || observation.UserPersistenceContentHash == "" {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_missing", false, observation)
		}
		if req.UserInput == nil || prepareOR1CHash(strings.TrimSpace(*req.UserInput)) != observation.UserPersistenceContentHash {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_mismatch", false, observation)
		}
	}
	assistantText := sanitizeCriticStorageText(*req.AssistantContent)
	if prepareOR1CHash(assistantText) != observation.PersistenceContentHash {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_persistence_content_mismatch", false, observation)
	}
	return completeTurnSourceAcceptanceDecision{Enabled: true, Accepted: true, Status: "accepted", Reason: "active_final_observation_accepted", QueueAction: "remove", Observation: observation}
}

func validateCompleteTurnNextHostSignalObservation(req dto.M4CompleteTurnRequest, observation completeTurnSourceObservation) completeTurnSourceAcceptanceDecision {
	if observation.HostLifecycleContractVersion != completeTurnRisuHostLifecycleContract ||
		observation.FinalitySource != "risu_next_host_signal_active_chat" ||
		observation.FinalityState != "committed_assistant_observed" ||
		(observation.HostSignalSource != "input" && observation.HostSignalSource != "beforeRequest") {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_lifecycle_invalid", false, observation)
	}
	if observation.RequestIDProvenance != "archive_center_correlation" ||
		observation.RequestCorrelationState != "matched_before_request_context" ||
		observation.RequestType != "model" ||
		observation.ResponseRole != "assistant" ||
		strings.TrimSpace(observation.ArchiveCenterCorrelationID) == "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_correlation_invalid", false, observation)
	}
	metaCorrelation, _ := req.ClientMeta["archive_center_request_correlation_id"].(string)
	if strings.TrimSpace(metaCorrelation) == "" ||
		strings.TrimSpace(metaCorrelation) != strings.TrimSpace(observation.ArchiveCenterCorrelationID) {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_correlation_mismatch", false, observation)
	}
	if observation.HostChatIDState != "observed" ||
		strings.TrimSpace(observation.HostChatID) == "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_chat_identity_missing", false, observation)
	}
	if observation.ChatStreamingState == "streaming" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_streaming_candidate", true, observation)
	}
	if observation.ChatStreamingState != "not_streaming" && observation.ChatStreamingState != "unobserved" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_streaming_observation_invalid", false, observation)
	}
	if observation.PositionObservation != "committed_before_next_host_signal" ||
		observation.ActiveMessageCount <= 0 ||
		observation.MessageIndex < 0 ||
		observation.MessageIndex >= observation.ActiveMessageCount ||
		observation.MessageRole != "char" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_message_invalid", false, observation)
	}
	if observation.MessageDisabledState != "not_disabled" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_assistant_message_disabled", false, observation)
	}
	if observation.MessageChatIDState != "observed" && observation.MessageChatIDState != "unobserved" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_message_identity_invalid", false, observation)
	}
	if observation.MessageChatIDState == "observed" && strings.TrimSpace(observation.MessageChatID) == "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_message_identity_invalid", false, observation)
	}
	if observation.GenerationIDState != "observed" && observation.GenerationIDState != "unobserved" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_generation_invalid", false, observation)
	}
	if (observation.GenerationIDState == "observed") != (strings.TrimSpace(observation.GenerationID) != "") {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_generation_invalid", false, observation)
	}
	if observation.MessageTimeState != "observed" && observation.MessageTimeState != "unobserved" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_message_time_invalid", false, observation)
	}
	if (observation.MessageTimeState == "observed") != (observation.MessageTimeMS > 0) {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_message_time_invalid", false, observation)
	}
	if observation.LaterActiveTurnMessageCount < 0 || observation.LaterActiveTurnMessageCount > 1 {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_stale_or_superseded", false, observation)
	}
	if observation.LaterActiveTurnMessageCount == 1 {
		if observation.NextSignalActiveRole != "user" ||
			observation.NextSignalUserIndex <= observation.MessageIndex ||
			observation.NextSignalUserIndex >= observation.ActiveMessageCount ||
			strings.TrimSpace(observation.NextSignalUserContentHash) == "" {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_stale_or_superseded", false, observation)
		}
	} else if observation.NextSignalActiveRole != "" ||
		observation.NextSignalUserIndex != -1 ||
		strings.TrimSpace(observation.NextSignalUserContentHash) != "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_signal_anchor_invalid", false, observation)
	}
	if observation.HashAlgorithm != "or1c_utf16_djb2.v1" ||
		strings.TrimSpace(observation.ObservedContentHash) == "" ||
		strings.TrimSpace(observation.PersistenceContentHash) == "" {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_next_host_signal_content_observation_missing", false, observation)
	}
	if req.AssistantContent == nil ||
		prepareOR1CHash(sanitizeCriticStorageText(*req.AssistantContent)) != observation.PersistenceContentHash {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_persistence_content_mismatch", false, observation)
	}
	userAnchorReported := observation.UserObservedContentHash != "" ||
		observation.UserPersistenceContentHash != "" ||
		observation.UserMessageChatID != ""
	if userAnchorReported {
		if observation.UserMessageIndex < 0 ||
			observation.UserMessageIndex >= observation.RequestMessageCount ||
			observation.UserObservedContentHash == "" ||
			observation.UserPersistenceContentHash == "" {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_missing", false, observation)
		}
		if observation.UserMessageChatIDState != "observed_before_request" &&
			observation.UserMessageChatIDState != "unobserved" {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_state_invalid", false, observation)
		}
		if observation.UserMessageTimeState != "observed_before_request" &&
			observation.UserMessageTimeState != "unobserved" {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_state_invalid", false, observation)
		}
		if req.UserInput == nil ||
			prepareOR1CHash(strings.TrimSpace(*req.UserInput)) != observation.UserPersistenceContentHash {
			return rejectedCompleteTurnSourceAcceptance("source_acceptance_user_anchor_mismatch", false, observation)
		}
	}
	return completeTurnSourceAcceptanceDecision{
		Enabled: true, Accepted: true, Status: "accepted",
		Reason:      "next_host_signal_finality_observation_accepted",
		QueueAction: "remove", Observation: observation,
	}
}

func sourceAcceptanceStateKey(sid string, turnIndex int) string {
	return sid + "\x1f" + strconv.Itoa(turnIndex)
}

func (s *Server) beginCompleteTurnSourceAcceptance(ctx context.Context, req dto.M4CompleteTurnRequest) completeTurnSourceAcceptanceDecision {
	if !completeTurnSourceAcceptanceRequired(req.ClientMeta) {
		return completeTurnSourceAcceptanceDecision{Status: "legacy_unobserved", Reason: "source_acceptance_not_required"}
	}
	observation, err := completeTurnSourceObservationFromMeta(req.ClientMeta)
	if err != nil {
		return rejectedCompleteTurnSourceAcceptance(err.Error(), false, observation)
	}
	decision := validateCompleteTurnSourceObservation(req, observation)
	if !decision.Accepted {
		return decision
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	turnIndex := req.TurnIndex
	if turnIndex <= 0 {
		turnIndex = 1
	}
	if observation.UserObservedContentHash != "" {
		decision.LogicalTurnID = completeTurnLogicalTurnID(sid, observation)
	}
	ledger := s.SourceAcceptances
	if ledger == nil {
		ledger = newCompleteTurnSourceAcceptanceLedger()
		s.SourceAcceptances = ledger
	}
	ledger.mu.Lock()
	var supersededWorkerDone []<-chan struct{}
	defer func() {
		ledger.mu.Unlock()
		waitForCompleteTurnSourceWorkers(supersededWorkerDone)
	}()
	ledger.loadDurableStateLocked(ctx, s.Store, sid)
	latestCanonicalTurn := 0
	if s.Store != nil {
		if logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			for _, log := range logs {
				if log.ChatSessionID == sid && log.TurnIndex > latestCanonicalTurn {
					latestCanonicalTurn = log.TurnIndex
				}
			}
		}
	}
	// A lower observed turn cannot belong to this canonical tail. Do not turn
	// an active-chat turn 35 into backend turn 52: that would preserve the
	// routing mistake and contaminate the wrong session. The host must resolve
	// the active RisuAI chat identity again before retrying.
	if latestCanonicalTurn > 0 && turnIndex < latestCanonicalTurn {
		conflict := rejectedCompleteTurnSourceAcceptance("source_acceptance_session_tail_conflict", true, observation)
		conflict.LogicalTurnID = decision.LogicalTurnID
		conflict.BoundTurn = turnIndex
		return conflict
	}
	for _, candidate := range ledger.current {
		if decision.LogicalTurnID == "" {
			break
		}
		if candidate.SessionID != sid || candidate.LogicalTurnID != decision.LogicalTurnID || candidate.Lifecycle != "active_final" {
			continue
		}
		if candidate.ObservedAtMS > 0 && (turnIndex <= 0 || candidate.ObservedAtMS >= ledger.current[sourceAcceptanceStateKey(sid, turnIndex)].ObservedAtMS) {
			turnIndex = candidate.TurnIndex
		}
	}
	decision.Revision = completeTurnSourceRevision(sid, turnIndex, observation)
	decision.BoundTurn = turnIndex
	key := sourceAcceptanceStateKey(sid, turnIndex)
	previous := ledger.current[key]
	decision.Previous = previous.Revision
	legacyLogicalTurnMatch := false
	if decision.LogicalTurnID != "" && previous.LogicalTurnID == "" && s.Store != nil {
		if logs, err := s.Store.ListChatLogs(ctx, sid, turnIndex, turnIndex); err == nil {
			userMatches, assistantMatches := completeTurnRawRoleContentMatches(logs, sid, turnIndex, *req.UserInput, *req.AssistantContent)
			_, hasAssistant := completeTurnRawRolePresence(logs, sid, turnIndex)
			legacyLogicalTurnMatch = userMatches && hasAssistant && !assistantMatches
		}
	}
	if invalidation := ledger.invalidations[sid]; invalidation.FromTurn > 0 && turnIndex >= invalidation.FromTurn && observation.ObservedAtMS <= invalidation.ObservedAtMS {
		return rejectedCompleteTurnSourceAcceptance("source_acceptance_deleted_or_rolled_back", false, observation)
	}
	if previous.Revision != "" && previous.Lifecycle == "active_final" {
		if previous.Revision == decision.Revision {
			decision.Reason = "active_final_observation_idempotent"
			decision.ReplaceExisting = previous.ReplacementStatus == "pending"
			return decision
		}
		if observation.ObservedAtMS <= previous.ObservedAtMS {
			stale := rejectedCompleteTurnSourceAcceptance("source_acceptance_stale_or_superseded", false, observation)
			stale.Revision = decision.Revision
			stale.Previous = previous.Revision
			return stale
		}
	}
	// Revision identity, not only text inequality, owns supersession. A reroll
	// may legitimately produce byte-identical text under a new Host-observed
	// generation/message identity; that new accepted revision still supersedes
	// the prior active worker fence.
	decision.ReplaceExisting = (previous.Revision != "" &&
		previous.LogicalTurnID == decision.LogicalTurnID &&
		previous.Revision != decision.Revision) || legacyLogicalTurnMatch
	var superseded *completeTurnSourceAcceptanceState
	if decision.ReplaceExisting {
		prior := previous
		prior.Lifecycle = "superseded"
		if observation.ObservedAtMS > prior.ObservedAtMS {
			prior.ObservedAtMS = observation.ObservedAtMS - 1
		}
		superseded = &prior
	}
	state := completeTurnSourceAcceptanceState{
		SessionID: sid, TurnIndex: turnIndex, Revision: decision.Revision,
		GenerationID: observation.GenerationID, HostChatID: observation.HostChatID,
		ContentHash: observation.ObservedContentHash, ObservedAtMS: observation.ObservedAtMS,
		Lifecycle: "active_final", LogicalTurnID: decision.LogicalTurnID,
	}
	if decision.ReplaceExisting {
		state.ReplacementStatus = "pending"
	}
	if worker := ledger.workers[key]; worker.cancel != nil && worker.revision != decision.Revision {
		worker.cancel()
		if worker.done != nil {
			supersededWorkerDone = append(supersededWorkerDone, worker.done)
		}
		delete(ledger.workers, key)
	}
	if previous.Revision != "" && previous.Revision != decision.Revision {
		if worker := ledger.reprocessingWorkers[previous.Revision]; worker != nil {
			worker.cancel()
			if worker.done != nil {
				supersededWorkerDone = append(supersededWorkerDone, worker.done)
			}
			delete(ledger.reprocessingWorkers, previous.Revision)
		}
	}
	ledger.current[key] = state
	if s.Store != nil && s.usesShadowWriteStore() {
		if superseded != nil {
			_ = s.Store.SaveAuditLog(context.WithoutCancel(ctx), &store.AuditLog{
				ChatSessionID: sid, EventType: sourceAcceptanceTransitionEvent, TargetType: "turn", TargetID: int64(turnIndex),
				Summary: fmt.Sprintf("source acceptance superseded turn %d", turnIndex), DetailsJSON: mustCompactJSON(*superseded),
				Source: s.storeWriteSource(), CreatedAt: time.Now().UTC(),
			})
		}
		_ = s.Store.SaveAuditLog(context.WithoutCancel(ctx), &store.AuditLog{
			ChatSessionID: sid, EventType: sourceAcceptanceTransitionEvent, TargetType: "turn", TargetID: int64(turnIndex),
			Summary: fmt.Sprintf("source acceptance active final turn %d", turnIndex), DetailsJSON: mustCompactJSON(state),
			Source: s.storeWriteSource(), CreatedAt: time.Now().UTC(),
		})
	}
	return decision
}

func (s *Server) rebindCompleteTurnSourceAcceptance(ctx context.Context, decision completeTurnSourceAcceptanceDecision, sid string, requestedTurn, actualTurn int) completeTurnSourceAcceptanceDecision {
	if !decision.Enabled || !decision.Accepted || requestedTurn == actualTurn || s.SourceAcceptances == nil {
		return decision
	}
	if requestedTurn <= 0 {
		requestedTurn = 1
	}
	fromKey := sourceAcceptanceStateKey(sid, requestedTurn)
	toKey := sourceAcceptanceStateKey(sid, actualTurn)
	s.SourceAcceptances.mu.Lock()
	state := s.SourceAcceptances.current[fromKey]
	if state.Revision != decision.Revision || state.Lifecycle != "active_final" {
		s.SourceAcceptances.mu.Unlock()
		decision.Accepted = false
		decision.Status = "rejected"
		decision.Reason = "source_acceptance_revision_superseded_before_turn_resolution"
		decision.QueueAction = "discard"
		return decision
	}
	if existing := s.SourceAcceptances.current[toKey]; existing.Lifecycle == "active_final" && existing.Revision != decision.Revision && existing.ObservedAtMS >= state.ObservedAtMS {
		s.SourceAcceptances.mu.Unlock()
		decision.Accepted = false
		decision.Status = "rejected"
		decision.Reason = "source_acceptance_turn_resolution_conflict"
		decision.QueueAction = "discard"
		return decision
	}
	tombstone := state
	tombstone.Lifecycle = "rebound"
	tombstone.ObservedAtMS++
	s.SourceAcceptances.current[fromKey] = tombstone
	state.TurnIndex = actualTurn
	state.ObservedAtMS = tombstone.ObservedAtMS
	s.SourceAcceptances.current[toKey] = state
	decision.BoundTurn = actualTurn
	s.SourceAcceptances.mu.Unlock()
	if s.Store != nil && s.usesShadowWriteStore() {
		writeCtx := context.WithoutCancel(ctx)
		_ = s.Store.SaveAuditLog(writeCtx, &store.AuditLog{ChatSessionID: sid, EventType: sourceAcceptanceTransitionEvent, TargetType: "turn", TargetID: int64(requestedTurn), Summary: fmt.Sprintf("source acceptance rebound from turn %d", requestedTurn), DetailsJSON: mustCompactJSON(tombstone), Source: s.storeWriteSource(), CreatedAt: time.Now().UTC()})
		_ = s.Store.SaveAuditLog(writeCtx, &store.AuditLog{ChatSessionID: sid, EventType: sourceAcceptanceTransitionEvent, TargetType: "turn", TargetID: int64(actualTurn), Summary: fmt.Sprintf("source acceptance active final turn %d", actualTurn), DetailsJSON: mustCompactJSON(state), Source: s.storeWriteSource(), CreatedAt: time.Now().UTC()})
	}
	return decision
}

func (s *Server) completeTurnSourceAcceptanceProcessingContext(parent context.Context, decision completeTurnSourceAcceptanceDecision, sid string, turnIndex int) (context.Context, func()) {
	if !decision.Enabled {
		return parent, func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	if s.SourceAcceptances == nil {
		cancel()
		return ctx, func() {}
	}
	key := sourceAcceptanceStateKey(sid, turnIndex)
	s.SourceAcceptances.mu.Lock()
	current := s.SourceAcceptances.current[key]
	if !decision.Accepted || current.Revision != decision.Revision || current.Lifecycle != "active_final" {
		s.SourceAcceptances.mu.Unlock()
		cancel()
		return ctx, func() {}
	}
	if previous := s.SourceAcceptances.workers[key]; previous.cancel != nil && previous.revision != decision.Revision {
		previous.cancel()
	}
	worker := completeTurnSourceAcceptanceWorker{revision: decision.Revision, cancel: cancel, done: make(chan struct{})}
	s.SourceAcceptances.workers[key] = worker
	s.SourceAcceptances.mu.Unlock()
	return ctx, func() {
		s.SourceAcceptances.mu.Lock()
		if worker := s.SourceAcceptances.workers[key]; worker.revision == decision.Revision {
			delete(s.SourceAcceptances.workers, key)
		}
		s.SourceAcceptances.mu.Unlock()
		cancel()
		close(worker.done)
	}
}

func (s *Server) completeTurnStoredSourceProcessingContext(parent context.Context, source *store.MemorySourceRevision) (context.Context, func()) {
	if source == nil ||
		strings.TrimSpace(source.ChatSessionID) == "" ||
		strings.TrimSpace(source.SourceRevision) == "" ||
		source.TurnIndex <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	ledger := s.SourceAcceptances
	if ledger == nil {
		ledger = newCompleteTurnSourceAcceptanceLedger()
		s.SourceAcceptances = ledger
	}
	worker := &completeTurnSourceReprocessingWorker{
		sessionID: source.ChatSessionID,
		turnIndex: source.TurnIndex,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	var previousDone <-chan struct{}
	ledger.mu.Lock()
	if previous := ledger.reprocessingWorkers[source.SourceRevision]; previous != nil {
		previous.cancel()
		previousDone = previous.done
	}
	ledger.reprocessingWorkers[source.SourceRevision] = worker
	ledger.mu.Unlock()
	waitForCompleteTurnSourceWorkers([]<-chan struct{}{previousDone})
	return ctx, func() {
		ledger.mu.Lock()
		if ledger.reprocessingWorkers[source.SourceRevision] == worker {
			delete(ledger.reprocessingWorkers, source.SourceRevision)
		}
		ledger.mu.Unlock()
		cancel()
		close(worker.done)
	}
}

func waitForCompleteTurnSourceWorkers(workers []<-chan struct{}) {
	for _, done := range workers {
		if done != nil {
			<-done
		}
	}
}

func (s *Server) cancelCompleteTurnSourceWorkers(sid string, fromTurn int) {
	if strings.TrimSpace(sid) == "" || fromTurn <= 0 || s.SourceAcceptances == nil {
		return
	}
	ledger := s.SourceAcceptances
	var workers []<-chan struct{}
	ledger.mu.Lock()
	for key, state := range ledger.current {
		if state.SessionID != sid || state.TurnIndex < fromTurn {
			continue
		}
		if worker := ledger.workers[key]; worker.cancel != nil {
			worker.cancel()
			if worker.done != nil {
				workers = append(workers, worker.done)
			}
			delete(ledger.workers, key)
		}
	}
	for revision, worker := range ledger.reprocessingWorkers {
		if worker == nil || worker.sessionID != sid || worker.turnIndex < fromTurn {
			continue
		}
		worker.cancel()
		if worker.done != nil {
			workers = append(workers, worker.done)
		}
		delete(ledger.reprocessingWorkers, revision)
	}
	ledger.mu.Unlock()
	waitForCompleteTurnSourceWorkers(workers)
}

func (l *completeTurnSourceAcceptanceLedger) loadDurableStateLocked(ctx context.Context, st store.Store, sid string) {
	if st == nil {
		return
	}
	if events, err := st.ListAuditLogs(ctx, sid, sourceAcceptanceTransitionEvent, 500); err == nil {
		for _, event := range events {
			var state completeTurnSourceAcceptanceState
			if json.Unmarshal([]byte(event.DetailsJSON), &state) != nil || state.TurnIndex <= 0 || state.Revision == "" {
				continue
			}
			key := sourceAcceptanceStateKey(sid, state.TurnIndex)
			current := l.current[key]
			newer := state.ObservedAtMS > current.ObservedAtMS
			completedTie := state.ObservedAtMS == current.ObservedAtMS && state.ReplacementStatus == "complete" && current.ReplacementStatus != "complete"
			activeTie := state.ObservedAtMS == current.ObservedAtMS && state.Lifecycle == "active_final" && current.Lifecycle != "active_final"
			if newer || completedTie || activeTie {
				l.current[key] = state
			}
		}
	}
	if events, err := st.ListAuditLogs(ctx, sid, sourceAcceptanceInvalidationEvent, 100); err == nil {
		for _, event := range events {
			var invalidation sourceAcceptanceInvalidation
			if json.Unmarshal([]byte(event.DetailsJSON), &invalidation) != nil || invalidation.FromTurn <= 0 {
				continue
			}
			if invalidation.ObservedAtMS > l.invalidations[sid].ObservedAtMS {
				l.invalidations[sid] = invalidation
			}
		}
	}
}

func (s *Server) completeTurnSourceAcceptanceStillCurrent(decision completeTurnSourceAcceptanceDecision, sid string, turnIndex int) bool {
	if !decision.Enabled {
		return true
	}
	if !decision.Accepted || s.SourceAcceptances == nil {
		return false
	}
	s.SourceAcceptances.mu.Lock()
	defer s.SourceAcceptances.mu.Unlock()
	current := s.SourceAcceptances.current[sourceAcceptanceStateKey(sid, turnIndex)]
	return current.Revision == decision.Revision && current.Lifecycle == "active_final"
}

func (s *Server) completeTurnSourceReplacementCompleted(ctx context.Context, decision completeTurnSourceAcceptanceDecision, sid string, turnIndex int) {
	if !decision.Enabled || !decision.ReplaceExisting || s.SourceAcceptances == nil {
		return
	}
	key := sourceAcceptanceStateKey(sid, turnIndex)
	s.SourceAcceptances.mu.Lock()
	state := s.SourceAcceptances.current[key]
	if state.Revision != decision.Revision || state.Lifecycle != "active_final" {
		s.SourceAcceptances.mu.Unlock()
		return
	}
	state.ReplacementStatus = "complete"
	s.SourceAcceptances.current[key] = state
	s.SourceAcceptances.mu.Unlock()
	if s.Store != nil && s.usesShadowWriteStore() {
		_ = s.Store.SaveAuditLog(context.WithoutCancel(ctx), &store.AuditLog{
			ChatSessionID: sid, EventType: sourceAcceptanceTransitionEvent, TargetType: "turn", TargetID: int64(turnIndex),
			Summary: fmt.Sprintf("source acceptance replacement complete turn %d", turnIndex), DetailsJSON: mustCompactJSON(state),
			Source: s.storeWriteSource(), CreatedAt: time.Now().UTC(),
		})
	}
}

func (s *Server) invalidateCompleteTurnSourceAcceptances(ctx context.Context, sid string, fromTurn int, source string, hostObservedAtMS int64) {
	if fromTurn <= 0 {
		return
	}
	ledger := s.SourceAcceptances
	if ledger == nil {
		ledger = newCompleteTurnSourceAcceptanceLedger()
		s.SourceAcceptances = ledger
	}
	ledger.mu.Lock()
	ledger.loadDurableStateLocked(ctx, s.Store, sid)
	hasAcceptedSource := false
	var workers []<-chan struct{}
	for _, state := range ledger.current {
		if state.SessionID == sid && state.TurnIndex >= fromTurn && state.Lifecycle == "active_final" {
			hasAcceptedSource = true
			break
		}
	}
	if !hasAcceptedSource && hostObservedAtMS <= 0 {
		ledger.mu.Unlock()
		return
	}
	observedAt := hostObservedAtMS
	if observedAt <= 0 {
		for _, state := range ledger.current {
			if state.SessionID == sid && state.TurnIndex >= fromTurn && state.ObservedAtMS >= observedAt {
				observedAt = state.ObservedAtMS + 1
			}
		}
	}
	ledger.invalidations[sid] = sourceAcceptanceInvalidation{FromTurn: fromTurn, ObservedAtMS: observedAt}
	for key, state := range ledger.current {
		if state.SessionID == sid && state.TurnIndex >= fromTurn {
			state.Lifecycle = "deleted"
			ledger.current[key] = state
			if worker := ledger.workers[key]; worker.cancel != nil {
				worker.cancel()
				if worker.done != nil {
					workers = append(workers, worker.done)
				}
				delete(ledger.workers, key)
			}
		}
	}
	for revision, worker := range ledger.reprocessingWorkers {
		if worker != nil && worker.sessionID == sid && worker.turnIndex >= fromTurn {
			worker.cancel()
			if worker.done != nil {
				workers = append(workers, worker.done)
			}
			delete(ledger.reprocessingWorkers, revision)
		}
	}
	ledger.mu.Unlock()
	waitForCompleteTurnSourceWorkers(workers)
	if s.Store != nil && s.usesShadowWriteStore() {
		invalidation := sourceAcceptanceInvalidation{FromTurn: fromTurn, ObservedAtMS: observedAt}
		_ = s.Store.SaveAuditLog(context.WithoutCancel(ctx), &store.AuditLog{
			ChatSessionID: sid, EventType: sourceAcceptanceInvalidationEvent, TargetType: "turn", TargetID: int64(fromTurn),
			Summary: fmt.Sprintf("source acceptance invalidated from turn %d", fromTurn), DetailsJSON: mustCompactJSON(invalidation),
			Source: source, CreatedAt: time.Now().UTC(),
		})
	}
}

func writeCompleteTurnSourceAcceptanceRejection(w http.ResponseWriter, req dto.M4CompleteTurnRequest, decision completeTurnSourceAcceptanceDecision) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "rejected", "code": decision.Reason, "chat_session_id": strings.TrimSpace(req.ChatSessionID),
		"turn_index": req.TurnIndex, "save_ok": false, "chat_logs_saved": 0, "derived_artifacts_saved": 0,
		"vectors_upserted": 0, "critic_triggered": false, "derived_retry_required": false,
		"queue_action": decision.QueueAction, "fail_reasons": []string{decision.Reason},
		"source_acceptance":       completeTurnSourceAcceptancePayload(decision),
		"source_to_final_lineage": buildSourceToFinalLineage(req, decision),
	})
}

func completeTurnSourceAcceptancePayload(decision completeTurnSourceAcceptanceDecision) map[string]any {
	lifecycle := "unobserved"
	if decision.Enabled {
		lifecycle = map[bool]string{true: "active_final", false: "candidate_or_inactive"}[decision.Accepted]
	}
	return map[string]any{
		"contract_version": completeTurnSourceLifecycleContract, "status": decision.Status, "reason": decision.Reason,
		"accepted": decision.Accepted, "retryable": decision.Retryable, "queue_action": decision.QueueAction,
		"revision": decision.Revision, "previous_revision": decision.Previous,
		"logical_turn_id": decision.LogicalTurnID, "replace_existing": decision.ReplaceExisting,
		"observation_contract_version":          decision.Observation.ContractVersion,
		"host_lifecycle_contract_version":       nilIfEmpty(decision.Observation.HostLifecycleContractVersion),
		"finality_source":                       nilIfEmpty(decision.Observation.FinalitySource),
		"request_id_provenance":                 nilIfEmpty(decision.Observation.RequestIDProvenance),
		"archive_center_request_correlation_id": nilIfEmpty(decision.Observation.ArchiveCenterCorrelationID),
		"host_revision_capability":              decision.Observation.RevisionState,
		"lifecycle":                             lifecycle,
		"generation_id":                         nilIfEmpty(decision.Observation.GenerationID),
		"generation_id_state":                   decision.Observation.GenerationIDState,
		"message_index":                         decision.Observation.MessageIndex,
		"observed_content_hash":                 nilIfEmpty(decision.Observation.ObservedContentHash),
		"persistence_content_hash":              nilIfEmpty(decision.Observation.PersistenceContentHash),
		"hash_algorithm":                        nilIfEmpty(decision.Observation.HashAlgorithm),
	}
}
