package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	rollbackDecisionContractVersion  = "rollback.decision.v2"
	routingTurnContractVersion       = "session-routing.turn-resolution.v1"
	risuWorldlineObservationContract = "risu_worldline_observation.v2"
	risuBranchShapeContract          = "risu_branchedfrom.v1"
	automaticActiveChatFullSweep     = "automatic_active_chat_full_sweep"
	rollbackDecisionMax              = 1024
)

type routingTurnBaseline struct {
	BackendTurnAtRoute int    `json:"backend_turn_at_route"`
	LocalPairsAtRoute  int    `json:"local_pairs_at_route"`
	Reason             string `json:"reason"`
	durableSourceID    string
	durableVerified    bool
}

type rollbackDecisionRequest struct {
	ChatSessionID                   string                                   `json:"chat_session_id"`
	StableCharacterID               string                                   `json:"stable_character_id,omitempty"`
	StableCharacterIDState          string                                   `json:"stable_character_id_state,omitempty"`
	HostChatID                      string                                   `json:"host_chat_id,omitempty"`
	HostChatIDState                 string                                   `json:"host_chat_id_state,omitempty"`
	RequestSource                   string                                   `json:"request_source"`
	Reason                          string                                   `json:"reason"`
	CandidateFromTurn               int                                      `json:"candidate_from_turn"`
	PreviousTurnIndex               int                                      `json:"previous_turn_index"`
	FirstRemovedTurn                int                                      `json:"first_removed_turn"`
	LedgerAnchorTurn                int                                      `json:"ledger_anchor_turn"`
	RemovedAssistantCount           int                                      `json:"removed_assistant_count"`
	RemovedUserCount                int                                      `json:"removed_user_count"`
	RemovedMessageCount             int                                      `json:"removed_message_count"`
	VisibleCompletedTurns           int                                      `json:"visible_completed_turns"`
	BackendLatestTurn               int                                      `json:"backend_latest_turn"`
	DeletionObserved                bool                                     `json:"deletion_observed"`
	LedgerVerified                  bool                                     `json:"ledger_verified"`
	IncompleteTailCandidate         bool                                     `json:"incomplete_tail_candidate"`
	BackendIncompleteTailVerified   bool                                     `json:"-"`
	HistoryTrimGuard                bool                                     `json:"history_trim_guard"`
	DuplicateBlocked                bool                                     `json:"duplicate_blocked"`
	HostLifecycleObservation        string                                   `json:"host_lifecycle_observation"`
	LifecycleActionObservation      string                                   `json:"lifecycle_action_observation"`
	AllowManualCandidate            bool                                     `json:"allow_manual_candidate"`
	AssistantObservationScope       string                                   `json:"assistant_observation_scope,omitempty"`
	AssistantObservations           []rollbackAssistantObservation           `json:"assistant_observations,omitempty"`
	Baseline                        *routingTurnBaseline                     `json:"baseline,omitempty"`
	ManualTargetOwnershipObserved   bool                                     `json:"-"`
	ManualTargetOwned               bool                                     `json:"-"`
	AssistantEvidenceRequired       bool                                     `json:"-"`
	AssistantEvidenceVerified       bool                                     `json:"-"`
	AssistantOutputRemoved          bool                                     `json:"-"`
	AssistantEvidenceReason         string                                   `json:"-"`
	RouteBindingRevision            uint64                                   `json:"-"`
	AssistantObservationDigest      string                                   `json:"-"`
	IncompleteAssistantObservations []rollbackAssistantObservationDiagnostic `json:"-"`
}

type rollbackAssistantObservation struct {
	MessageID      string `json:"message_id,omitempty"`
	GenerationID   string `json:"generation_id,omitempty"`
	ContentHash    string `json:"content_hash,omitempty"`
	MessageIndex   int    `json:"message_index"`
	DisabledState  string `json:"disabled_state,omitempty"`
	StreamingState string `json:"streaming_state,omitempty"`
	FinalState     string `json:"final_state,omitempty"`
}

type rollbackAssistantObservationDiagnostic struct {
	ObservationIndex int    `json:"observation_index"`
	MessageIndex     int    `json:"message_index"`
	Reason           string `json:"reason"`
}

type normalizedRollbackAssistantObservation struct {
	MessageID      string `json:"message_id,omitempty"`
	GenerationID   string `json:"generation_id,omitempty"`
	ContentHash    string `json:"content_hash,omitempty"`
	MessageIndex   int    `json:"message_index"`
	DisabledState  string `json:"disabled_state,omitempty"`
	StreamingState string `json:"streaming_state,omitempty"`
	FinalState     string `json:"final_state,omitempty"`
}

func normalizeRollbackAssistantObservations(
	scope string,
	observations []rollbackAssistantObservation,
) ([]rollbackAssistantObservation, string, []rollbackAssistantObservationDiagnostic) {
	complete := make([]rollbackAssistantObservation, 0, len(observations))
	canonical := make([]normalizedRollbackAssistantObservation, 0, len(observations))
	incomplete := make([]rollbackAssistantObservationDiagnostic, 0)
	for index, item := range observations {
		item.MessageID = strings.TrimSpace(item.MessageID)
		item.GenerationID = strings.TrimSpace(item.GenerationID)
		item.ContentHash = strings.TrimSpace(item.ContentHash)
		item.DisabledState = strings.ToLower(strings.TrimSpace(item.DisabledState))
		item.StreamingState = strings.ToLower(strings.TrimSpace(item.StreamingState))
		item.FinalState = strings.ToLower(strings.TrimSpace(item.FinalState))
		reason := ""
		if item.MessageIndex < 0 {
			reason = "message_index_missing"
		}
		if item.MessageID == "" && item.GenerationID == "" && item.ContentHash == "" {
			if reason == "" {
				reason = "assistant_identity_missing"
			} else {
				reason += "+assistant_identity_missing"
			}
		}
		if reason != "" {
			incomplete = append(incomplete, rollbackAssistantObservationDiagnostic{
				ObservationIndex: index,
				MessageIndex:     item.MessageIndex,
				Reason:           reason,
			})
			continue
		}
		complete = append(complete, item)
		canonical = append(canonical, normalizedRollbackAssistantObservation{
			MessageID: item.MessageID, GenerationID: item.GenerationID,
			ContentHash: item.ContentHash, MessageIndex: item.MessageIndex,
			DisabledState: item.DisabledState, StreamingState: item.StreamingState,
			FinalState: item.FinalState,
		})
	}
	sort.Slice(canonical, func(i, j int) bool {
		left, right := canonical[i], canonical[j]
		if left.MessageIndex != right.MessageIndex {
			return left.MessageIndex < right.MessageIndex
		}
		leftKey := left.MessageID + "\x00" + left.GenerationID + "\x00" + left.ContentHash + "\x00" + left.DisabledState + "\x00" + left.StreamingState + "\x00" + left.FinalState
		rightKey := right.MessageID + "\x00" + right.GenerationID + "\x00" + right.ContentHash + "\x00" + right.DisabledState + "\x00" + right.StreamingState + "\x00" + right.FinalState
		return leftKey < rightKey
	})
	encoded, _ := json.Marshal(struct {
		Scope        string                                   `json:"scope"`
		Observations []normalizedRollbackAssistantObservation `json:"observations"`
	}{Scope: strings.TrimSpace(scope), Observations: canonical})
	digest := sha256.Sum256(encoded)
	return complete, hex.EncodeToString(digest[:]), incomplete
}

func rollbackAssistantObservationEligible(observation rollbackAssistantObservation) bool {
	if strings.TrimSpace(observation.DisabledState) == "disabled" ||
		strings.TrimSpace(observation.StreamingState) == "streaming" {
		return false
	}
	switch strings.TrimSpace(observation.FinalState) {
	case "inactive", "pending":
		return false
	default:
		return true
	}
}

type rollbackAssistantDeletionEvidence struct {
	Verified         bool
	RemovedCount     int
	FirstRemovedTurn int
	Reason           string
}

func rollbackAssistantObservationMatchesSource(source store.MemorySourceRevision, observation rollbackAssistantObservation, identityOnly bool) bool {
	messageMatch := strings.TrimSpace(source.SourceMessageID) != "" &&
		strings.TrimSpace(observation.MessageID) != "" &&
		strings.TrimSpace(source.SourceMessageID) == strings.TrimSpace(observation.MessageID)
	generationMatch := strings.TrimSpace(source.SourceGenerationID) != "" &&
		strings.TrimSpace(observation.GenerationID) != "" &&
		strings.TrimSpace(source.SourceGenerationID) == strings.TrimSpace(observation.GenerationID)
	if messageMatch || generationMatch {
		return true
	}
	if identityOnly {
		return false
	}
	observedHash := strings.TrimSpace(observation.ContentHash)
	if observedHash == "" {
		return false
	}
	storedObservedHash := strings.TrimSpace(source.AssistantObservedContentHash)
	canonicalObservedHash := prepareOR1CHash(source.AssistantContent)
	return observedHash == storedObservedHash || observedHash == canonicalObservedHash
}

type rollbackDecisionResponse struct {
	Status                          string                                   `json:"status"`
	ContractVersion                 string                                   `json:"contract_version"`
	Allowed                         bool                                     `json:"allowed"`
	Decision                        string                                   `json:"decision"`
	Reason                          string                                   `json:"reason"`
	ChatSessionID                   string                                   `json:"chat_session_id"`
	RequestedFromTurn               int                                      `json:"requested_from_turn"`
	FromTurn                        int                                      `json:"from_turn"`
	ProtectedBeforeTurn             int                                      `json:"protected_before_turn"`
	MinFromTurn                     int                                      `json:"min_from_turn"`
	EffectiveCompleted              int                                      `json:"effective_completed_turns"`
	BaselineApplied                 bool                                     `json:"baseline_applied"`
	DecisionToken                   string                                   `json:"decision_token,omitempty"`
	LifecycleAction                 string                                   `json:"lifecycle_action"`
	TurnWorkflowHUD                 any                                      `json:"turn_workflow_hud,omitempty"`
	RouteBindingRevision            uint64                                   `json:"route_binding_revision,omitempty"`
	AssistantObservationDigest      string                                   `json:"assistant_observation_digest,omitempty"`
	IncompleteAssistantObservations []rollbackAssistantObservationDiagnostic `json:"incomplete_assistant_observations,omitempty"`
}

type rollbackDecisionRecord struct {
	Token                      string
	SessionID                  string
	FromTurn                   int
	RequestSource              string
	LifecycleAction            string
	StableCharacterID          string
	HostChatID                 string
	CanonicalSessionID         string
	RouteBindingRevision       uint64
	AssistantObservationDigest string
	Sequence                   uint64
}

type rollbackDecisionLedger struct {
	mu           sync.Mutex
	records      map[string]rollbackDecisionRecord
	nextSequence uint64
}

func newRollbackDecisionLedger() *rollbackDecisionLedger {
	return &rollbackDecisionLedger{records: map[string]rollbackDecisionRecord{}}
}

func (l *rollbackDecisionLedger) issue(record rollbackDecisionRecord) rollbackDecisionRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UTC()
	if len(l.records) >= rollbackDecisionMax {
		l.evictOldestLocked()
	}
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		bytes = []byte(now.Format("20060102150405.000000000"))
	}
	token := hex.EncodeToString(bytes)
	l.nextSequence++
	record.Token = token
	record.Sequence = l.nextSequence
	l.records[token] = record
	return record
}

func (l *rollbackDecisionLedger) evictOldestLocked() {
	oldestToken := ""
	var oldestSequence uint64
	for token, record := range l.records {
		if oldestToken == "" || record.Sequence < oldestSequence {
			oldestToken = token
			oldestSequence = record.Sequence
		}
	}
	if oldestToken != "" {
		delete(l.records, oldestToken)
	}
}

func (l *rollbackDecisionLedger) consume(token, sessionID string, fromTurn int, assistantObservationDigest string) (rollbackDecisionRecord, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	record, ok := l.records[token]
	if !ok {
		return rollbackDecisionRecord{}, false
	}
	delete(l.records, token)
	if record.SessionID != sessionID || record.FromTurn != fromTurn ||
		strings.TrimSpace(record.AssistantObservationDigest) != strings.TrimSpace(assistantObservationDigest) {
		return rollbackDecisionRecord{}, false
	}
	return record, true
}

func resolveExistingRollbackRoute(
	ctx context.Context,
	base store.Store,
	stableCharacterID string,
	hostChatID string,
) (store.SessionRouteBinding, error) {
	bindingStore, ok := base.(store.SessionRouteBindingStore)
	if !ok {
		return store.SessionRouteBinding{}, errors.New("session route binding store is unavailable")
	}
	result, err := bindingStore.BindSessionRoute(ctx, store.SessionRouteBindingRequest{
		StableCharacterID: strings.TrimSpace(stableCharacterID),
		HostChatID:        strings.TrimSpace(hostChatID),
		Mode:              store.SessionRouteBindingModeResolveExisting,
	})
	if err != nil {
		return store.SessionRouteBinding{}, err
	}
	if result == nil || !result.ReadbackVerified ||
		strings.TrimSpace(result.Binding.StableCharacterID) != strings.TrimSpace(stableCharacterID) ||
		strings.TrimSpace(result.Binding.HostChatID) != strings.TrimSpace(hostChatID) ||
		strings.TrimSpace(result.Binding.CanonicalSessionID) == "" ||
		strings.TrimSpace(result.Binding.BindingState) != "active" {
		return store.SessionRouteBinding{}, errors.New("session route binding readback mismatch")
	}
	return result.Binding, nil
}

func (s *Server) rollbackDecisionLedger() *rollbackDecisionLedger {
	if s.RollbackDecisions == nil {
		s.RollbackDecisions = newRollbackDecisionLedger()
	}
	return s.RollbackDecisions
}

func verifyRollbackAssistantDeletionEvidence(
	ctx context.Context,
	base store.Store,
	chatSessionID string,
	observations []rollbackAssistantObservation,
) (rollbackAssistantDeletionEvidence, error) {
	lister, ok := base.(store.ActiveSourceRevisionLister)
	if !ok {
		return rollbackAssistantDeletionEvidence{Reason: "active_source_revision_lister_unavailable"}, nil
	}
	sources, err := lister.ListActiveSourceRevisions(ctx, strings.TrimSpace(chatSessionID), 0, 0)
	if err != nil {
		return rollbackAssistantDeletionEvidence{Reason: "active_source_revision_list_failed"}, err
	}
	filtered := make([]store.MemorySourceRevision, 0, len(sources))
	for _, source := range sources {
		if source.LifecycleState != "active" || source.TurnIndex <= 0 || strings.TrimSpace(source.AssistantContent) == "" {
			continue
		}
		filtered = append(filtered, source)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex < filtered[j].TurnIndex
		}
		return filtered[i].ID < filtered[j].ID
	})
	if len(filtered) == 0 {
		return rollbackAssistantDeletionEvidence{Verified: true, Reason: "no_active_completed_source_revisions"}, nil
	}

	// Preserve the 4.0.2 rollback contract: the current Host assistant history
	// must be an exact prefix of the durable active-source history. Only the
	// contiguous missing suffix is deletion evidence. A middle replacement,
	// stale source revision, reordered observation, or later matched turn after
	// a mismatch is a historical conflict and must never widen the rollback.
	orderedObservations := append([]rollbackAssistantObservation(nil), observations...)
	sort.SliceStable(orderedObservations, func(i, j int) bool {
		return orderedObservations[i].MessageIndex < orderedObservations[j].MessageIndex
	})
	for index := 1; index < len(orderedObservations); index++ {
		if orderedObservations[index-1].MessageIndex == orderedObservations[index].MessageIndex {
			return rollbackAssistantDeletionEvidence{Reason: "historical_revision_conflict"}, nil
		}
	}

	prefixLength := minInt(len(filtered), len(orderedObservations))
	for index := 0; index < prefixLength; index++ {
		source := filtered[index]
		observation := orderedObservations[index]
		if rollbackAssistantObservationMatchesSource(source, observation, true) ||
			rollbackAssistantObservationMatchesSource(source, observation, false) {
			continue
		}
		return rollbackAssistantDeletionEvidence{Reason: "historical_revision_conflict"}, nil
	}
	if len(orderedObservations) >= len(filtered) {
		return rollbackAssistantDeletionEvidence{Verified: true, Reason: "verified_no_assistant_output_removed"}, nil
	}

	removedCount := len(filtered) - len(orderedObservations)
	firstRemovedTurn := filtered[len(orderedObservations)].TurnIndex
	reason := "verified_no_assistant_output_removed"
	if removedCount > 0 {
		reason = "verified_assistant_output_removed"
	}
	return rollbackAssistantDeletionEvidence{
		Verified:         true,
		RemovedCount:     removedCount,
		FirstRemovedTurn: firstRemovedTurn,
		Reason:           reason,
	}, nil
}

func (s *Server) handleRollbackDecision(w http.ResponseWriter, r *http.Request) {
	var req rollbackDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_rollback_observation"})
		return
	}
	manualCandidate := strings.EqualFold(strings.TrimSpace(req.RequestSource), "manual") && req.AllowManualCandidate
	if !manualCandidate {
		completeObservations, digest, incomplete := normalizeRollbackAssistantObservations(
			req.AssistantObservationScope,
			req.AssistantObservations,
		)
		req.AssistantObservations = completeObservations
		req.AssistantObservationDigest = digest
		req.IncompleteAssistantObservations = incomplete
		if strings.TrimSpace(req.StableCharacterIDState) != "observed" ||
			strings.TrimSpace(req.HostChatIDState) != "observed" ||
			strings.TrimSpace(req.StableCharacterID) == "" ||
			strings.TrimSpace(req.HostChatID) == "" {
			writeJSON(w, http.StatusOK, rollbackDecisionResponse{
				Status: "ok", ContractVersion: rollbackDecisionContractVersion,
				Decision: "blocked", Reason: "session_route_identity_unobserved",
				ChatSessionID:                   strings.TrimSpace(req.ChatSessionID),
				RequestedFromTurn:               req.CandidateFromTurn,
				AssistantObservationDigest:      digest,
				IncompleteAssistantObservations: incomplete,
			})
			return
		}
		binding, err := resolveExistingRollbackRoute(
			r.Context(), s.Store, req.StableCharacterID, req.HostChatID,
		)
		if err != nil {
			reason := "session_route_binding_failed"
			if errors.Is(err, store.ErrNotFound) {
				reason = "session_route_binding_not_found"
			}
			writeJSON(w, http.StatusOK, rollbackDecisionResponse{
				Status: "ok", ContractVersion: rollbackDecisionContractVersion,
				Decision: "blocked", Reason: reason,
				ChatSessionID:                   strings.TrimSpace(req.ChatSessionID),
				RequestedFromTurn:               req.CandidateFromTurn,
				AssistantObservationDigest:      digest,
				IncompleteAssistantObservations: incomplete,
			})
			return
		}
		canonicalSessionID := strings.TrimSpace(binding.CanonicalSessionID)
		if canonicalSessionID != strings.TrimSpace(req.ChatSessionID) {
			writeJSON(w, http.StatusOK, rollbackDecisionResponse{
				Status: "ok", ContractVersion: rollbackDecisionContractVersion,
				Decision: "blocked", Reason: "session_route_canonical_mismatch",
				ChatSessionID:                   canonicalSessionID,
				RequestedFromTurn:               req.CandidateFromTurn,
				RouteBindingRevision:            binding.Revision,
				AssistantObservationDigest:      digest,
				IncompleteAssistantObservations: incomplete,
			})
			return
		}
		req.ChatSessionID = canonicalSessionID
		req.RouteBindingRevision = binding.Revision
	}
	req.Baseline = s.resolveDurableSessionRoutingBaseline(r.Context(), req.ChatSessionID, req.Baseline)
	backendLatestAuthoritative := false
	if rangeStore, ok := s.Store.(interface {
		LatestSessionTurnIndex(context.Context, string) (int, error)
	}); ok {
		if latestTurn, err := rangeStore.LatestSessionTurnIndex(r.Context(), strings.TrimSpace(req.ChatSessionID)); err == nil {
			req.BackendLatestTurn = latestTurn
			backendLatestAuthoritative = true
		}
	}
	if manualCandidate && req.CandidateFromTurn > 0 && s.Store != nil {
		logs, err := s.Store.ListChatLogs(
			r.Context(),
			strings.TrimSpace(req.ChatSessionID),
			req.CandidateFromTurn,
			req.CandidateFromTurn,
		)
		if err == nil {
			req.ManualTargetOwnershipObserved = true
			for _, item := range logs {
				if item.TurnIndex == req.CandidateFromTurn && strings.TrimSpace(item.ChatSessionID) == strings.TrimSpace(req.ChatSessionID) {
					req.ManualTargetOwned = true
					break
				}
			}
		}
	}
	if !manualCandidate &&
		!req.IncompleteTailCandidate &&
		strings.TrimSpace(req.AssistantObservationScope) != "" &&
		strings.TrimSpace(req.LifecycleActionObservation) != store.LogicalTurnLifecycleSuperseded {
		req.AssistantEvidenceRequired = true
		if strings.TrimSpace(req.AssistantObservationScope) != "full_active_chat" {
			req.AssistantEvidenceReason = "assistant_observation_scope_invalid"
		} else if len(req.IncompleteAssistantObservations) > 0 && len(req.AssistantObservations) == 0 {
			req.AssistantEvidenceReason = "assistant_observations_incomplete"
		} else if evidence, err := verifyRollbackAssistantDeletionEvidence(
			r.Context(),
			s.Store,
			req.ChatSessionID,
			req.AssistantObservations,
		); err != nil {
			req.AssistantEvidenceReason = evidence.Reason
		} else {
			req.AssistantEvidenceVerified = evidence.Verified
			req.AssistantOutputRemoved = evidence.RemovedCount > 0
			req.AssistantEvidenceReason = evidence.Reason
			if evidence.Verified {
				req.RemovedAssistantCount = evidence.RemovedCount
				req.FirstRemovedTurn = evidence.FirstRemovedTurn
				req.LedgerVerified = false
				req.DeletionObserved = evidence.RemovedCount > 0
			}
		}
	}
	if req.IncompleteTailCandidate &&
		req.DeletionObserved &&
		req.RemovedAssistantCount == 0 &&
		req.RemovedUserCount == 1 &&
		req.RemovedMessageCount == 1 &&
		req.BackendLatestTurn > 0 &&
		req.CandidateFromTurn == req.BackendLatestTurn &&
		s.Store != nil {
		fromTurn, toTurn := 0, 0
		if backendLatestAuthoritative {
			fromTurn, toTurn = req.BackendLatestTurn, req.BackendLatestTurn
		}
		logs, err := s.Store.ListChatLogs(r.Context(), strings.TrimSpace(req.ChatSessionID), fromTurn, toTurn)
		if err == nil && len(logs) > 0 {
			actualLatestTurn := req.BackendLatestTurn
			if !backendLatestAuthoritative {
				actualLatestTurn = 0
			}
			for _, item := range logs {
				if item.TurnIndex > actualLatestTurn {
					actualLatestTurn = item.TurnIndex
				}
			}
			userRowsOnly := actualLatestTurn == req.BackendLatestTurn
			latestRows := 0
			for _, item := range logs {
				if item.TurnIndex != actualLatestTurn {
					continue
				}
				latestRows++
				if !strings.EqualFold(strings.TrimSpace(item.Role), "user") {
					userRowsOnly = false
				}
			}
			req.BackendIncompleteTailVerified = userRowsOnly && latestRows > 0
		}
	}
	resp := calculateRollbackDecision(req)
	resp.RouteBindingRevision = req.RouteBindingRevision
	resp.AssistantObservationDigest = req.AssistantObservationDigest
	resp.IncompleteAssistantObservations = req.IncompleteAssistantObservations
	if resp.Allowed {
		requestSource := strings.TrimSpace(req.RequestSource)
		if requestSource == "" {
			requestSource = "auto"
		}
		record := s.rollbackDecisionLedger().issue(rollbackDecisionRecord{
			SessionID: resp.ChatSessionID, FromTurn: resp.FromTurn,
			RequestSource: requestSource, LifecycleAction: resp.LifecycleAction,
			StableCharacterID:          strings.TrimSpace(req.StableCharacterID),
			HostChatID:                 strings.TrimSpace(req.HostChatID),
			CanonicalSessionID:         resp.ChatSessionID,
			RouteBindingRevision:       req.RouteBindingRevision,
			AssistantObservationDigest: req.AssistantObservationDigest,
		})
		resp.DecisionToken = record.Token
		resp.TurnWorkflowHUD = s.turnWorkflowHUDOperationNotice(
			fmt.Sprintf("rollback:%s:%d:%s", resp.ChatSessionID, resp.FromTurn, requestSource),
			resp.ChatSessionID,
			resp.FromTurn,
			"running",
			"notice",
			"turn_hud.notice.delete_detected",
			"turn_hud.notice.delete_detected_detail",
			"ASSISTANT_OUTPUT_DELETE_DETECTED",
		)
	}
	writeJSON(w, http.StatusOK, resp)
}

func calculateRollbackDecision(req rollbackDecisionRequest) rollbackDecisionResponse {
	sid := strings.TrimSpace(req.ChatSessionID)
	resp := rollbackDecisionResponse{
		Status: "ok", ContractVersion: rollbackDecisionContractVersion,
		Decision: "blocked", Reason: "deletion_not_verified", ChatSessionID: sid,
		RequestedFromTurn: req.CandidateFromTurn,
	}
	if sid == "" || sid == "default" {
		resp.Reason = "invalid_session"
		return resp
	}
	if req.HistoryTrimGuard {
		resp.Reason = "history_trim_guard"
		return resp
	}
	if req.DuplicateBlocked {
		resp.Reason = "duplicate_rollback_blocked"
		return resp
	}
	manual := strings.EqualFold(strings.TrimSpace(req.RequestSource), "manual")
	lifecycleAction := strings.ToLower(strings.TrimSpace(req.LifecycleActionObservation))
	switch lifecycleAction {
	case "":
		// This decision contract only admits a verified host deletion. Callers
		// performing an output replacement must explicitly observe supersession.
		lifecycleAction = store.LogicalTurnLifecycleDeleted
	case store.LogicalTurnLifecycleDeleted, store.LogicalTurnLifecycleSuperseded:
	default:
		resp.Reason = "lifecycle_action_observation_invalid"
		return resp
	}
	if !manual && req.AssistantEvidenceRequired {
		if !req.AssistantEvidenceVerified {
			resp.Reason = strings.TrimSpace(req.AssistantEvidenceReason)
			if resp.Reason == "" {
				resp.Reason = "assistant_output_verification_unavailable"
			}
			return resp
		}
		if !req.AssistantOutputRemoved {
			resp.Reason = "assistant_output_not_removed"
			return resp
		}
	}
	if !req.DeletionObserved && !(manual && req.AllowManualCandidate) {
		return resp
	}
	if manual && req.AllowManualCandidate {
		if req.CandidateFromTurn <= 0 {
			resp.Reason = "missing_delete_anchor"
			return resp
		}
		if !req.ManualTargetOwnershipObserved {
			resp.Reason = "manual_target_ownership_unavailable"
			return resp
		}
		if !req.ManualTargetOwned {
			resp.Reason = "manual_target_not_owned"
			return resp
		}
	}
	if req.IncompleteTailCandidate && !req.BackendIncompleteTailVerified {
		resp.Reason = "incomplete_tail_not_verified"
		return resp
	}

	effectiveCompleted := maxInt(0, req.VisibleCompletedTurns)
	protectedBefore, minFrom := 0, 0
	baselineApplied := false
	if baseline := req.Baseline; !manual && baseline != nil && routingBaselineReasonSupported(baseline.Reason) && baseline.BackendTurnAtRoute > 0 {
		localBase := maxInt(0, baseline.LocalPairsAtRoute)
		backendBase := maxInt(0, baseline.BackendTurnAtRoute)
		effectiveCompleted = backendBase + maxInt(0, req.VisibleCompletedTurns-localBase)
		protectedBefore, minFrom, baselineApplied = backendBase, backendBase+1, true
	}

	fromTurn := 0
	if req.BackendIncompleteTailVerified && req.BackendLatestTurn > 0 {
		fromTurn = req.BackendLatestTurn
	} else if req.LedgerVerified && req.RemovedAssistantCount > 0 && req.BackendLatestTurn > 0 {
		if req.RemovedAssistantCount > req.BackendLatestTurn {
			resp.Reason = "ledger_removed_count_exceeds_backend_tail"
			return resp
		}
		// A persisted migration/attach baseline may be unavailable after a client
		// reload. A verified tail deletion still has an unambiguous server-side
		// range: remove exactly the observed assistant turns from the backend tail.
		fromTurn = req.BackendLatestTurn - req.RemovedAssistantCount + 1
	} else {
		fromTurn = firstPositive(req.FirstRemovedTurn, req.LedgerAnchorTurn)
	}
	if fromTurn == 0 && req.LedgerVerified && effectiveCompleted >= 0 {
		fromTurn = effectiveCompleted + 1
	}
	if fromTurn == 0 && req.RemovedAssistantCount > 0 && req.PreviousTurnIndex > 0 {
		fromTurn = maxInt(1, req.PreviousTurnIndex-req.RemovedAssistantCount+1)
	}
	if fromTurn == 0 {
		fromTurn = req.CandidateFromTurn
	}
	if fromTurn <= 0 {
		resp.Reason = "missing_delete_anchor"
		return resp
	}
	if minFrom > 0 && fromTurn < minFrom {
		fromTurn = minFrom
	}
	if req.BackendLatestTurn > 0 && fromTurn > req.BackendLatestTurn {
		resp.Reason = "delete_anchor_after_backend_tail"
		resp.FromTurn = fromTurn
		return resp
	}
	resp.Allowed = true
	resp.Decision = "execute"
	resp.Reason = "verified_delete_range"
	resp.FromTurn = fromTurn
	resp.ProtectedBeforeTurn = protectedBefore
	resp.MinFromTurn = minFrom
	resp.EffectiveCompleted = effectiveCompleted
	resp.BaselineApplied = baselineApplied
	resp.LifecycleAction = lifecycleAction
	return resp
}

type sessionRoutingTurnResolutionRequest struct {
	worldlineOriginDepth   int
	ChatSessionID          string                    `json:"chat_session_id"`
	Mode                   string                    `json:"mode"`
	StableCharacterID      string                    `json:"stable_character_id,omitempty"`
	StableCharacterIDState string                    `json:"stable_character_id_state,omitempty"`
	HostChatID             string                    `json:"host_chat_id,omitempty"`
	HostChatIDState        string                    `json:"host_chat_id_state,omitempty"`
	BindRequestedSession   bool                      `json:"bind_requested_session,omitempty"`
	BindingMode            string                    `json:"binding_mode,omitempty"`
	LatestUserHash         string                    `json:"latest_user_hash,omitempty"`
	LatestAssistantHash    string                    `json:"latest_assistant_hash,omitempty"`
	LocalTurnIndex         int                       `json:"local_turn_index"`
	VisibleCompletedTurns  int                       `json:"visible_completed_turns"`
	RisuUserMessageIndex   *int                      `json:"risu_user_message_index,omitempty"`
	ObservedPairOrdinal    int                       `json:"observed_pair_ordinal,omitempty"`
	Observations           []routingTurnObservation  `json:"observations,omitempty"`
	Baseline               *routingTurnBaseline      `json:"baseline,omitempty"`
	WorldlineObservation   *risuWorldlineObservation `json:"worldline_observation,omitempty"`
	RoutingContext         string                    `json:"routing_context,omitempty"`
	canonicalTailAligned   bool
}

type risuWorldlineObservation struct {
	ContractVersion     string                            `json:"contract_version"`
	HostSignalSource    string                            `json:"host_signal_source"`
	BranchShapeContract string                            `json:"branch_shape_contract"`
	ObservedAtMS        int64                             `json:"observed_at_ms"`
	MarkerState         string                            `json:"marker_state"`
	BranchMarker        string                            `json:"branch_marker"`
	MarkerIndex         int                               `json:"marker_index"`
	Messages            []risuWorldlineMessageObservation `json:"messages"`
	MessageOrigins      *risuWorldlineOriginObservation   `json:"message_origins,omitempty"`
}

type risuWorldlineMessageObservation struct {
	MessageIndex  int    `json:"message_index"`
	Role          string `json:"role"`
	MessageChatID string `json:"message_chat_id"`
	Disabled      bool   `json:"disabled"`
}

type routingTurnObservation struct {
	ObservationIndex          int    `json:"observation_index"`
	RisuUserMessageIndex      *int   `json:"risu_user_message_index,omitempty"`
	RisuAssistantMessageIndex *int   `json:"risu_assistant_message_index,omitempty"`
	ObservedPairOrdinal       int    `json:"observed_pair_ordinal,omitempty"`
	AssistantMessageID        string `json:"assistant_message_id,omitempty"`
	AssistantGenerationID     string `json:"assistant_generation_id,omitempty"`
	AssistantContentHash      string `json:"assistant_content_hash,omitempty"`
	AssistantContent          string `json:"assistant_content,omitempty"`
	AdjacentUserPresent       bool   `json:"adjacent_user_present"`
	AdjacentUserContent       string `json:"adjacent_user_content,omitempty"`
	AssistantDisabledState    string `json:"assistant_disabled_state,omitempty"`
	AssistantStreamingState   string `json:"assistant_streaming_state,omitempty"`
	AssistantFinalState       string `json:"assistant_final_state,omitempty"`
}

type routingTurnResolvedObservation struct {
	ObservationIndex          int    `json:"observation_index"`
	RisuUserMessageIndex      *int   `json:"risu_user_message_index,omitempty"`
	RisuAssistantMessageIndex *int   `json:"risu_assistant_message_index,omitempty"`
	ObservedPairOrdinal       int    `json:"observed_pair_ordinal"`
	LocalTurnIndex            int    `json:"local_turn_index"`
	TurnIndex                 int    `json:"turn_index"`
	Resolution                string `json:"resolution"`
	Source                    string `json:"source"`
	SourceRevision            string `json:"source_revision,omitempty"`
	SourceLifecycleState      string `json:"source_lifecycle_state,omitempty"`
	StoredUserContent         string `json:"stored_user_content,omitempty"`
	StoredAssistantContent    string `json:"stored_assistant_content,omitempty"`
	InputMode                 string `json:"input_mode,omitempty"`
	UserInputState            string `json:"user_input_state,omitempty"`
	TurnIdentityState         string `json:"turn_identity_state,omitempty"`
}

type sessionRoutingTurnResolutionResponse struct {
	Status                 string                           `json:"status"`
	ContractVersion        string                           `json:"contract_version"`
	Code                   string                           `json:"code,omitempty"`
	ChatSessionID          string                           `json:"chat_session_id,omitempty"`
	IdentityResolution     string                           `json:"identity_resolution,omitempty"`
	BindingContractVersion string                           `json:"binding_contract_version,omitempty"`
	BindingRequired        bool                             `json:"binding_required"`
	BindingAcknowledged    bool                             `json:"binding_acknowledged"`
	BindingCreated         bool                             `json:"binding_created,omitempty"`
	BindingUpdated         bool                             `json:"binding_updated,omitempty"`
	LockedSourceRedirect   bool                             `json:"locked_source_redirect,omitempty"`
	Resolution             string                           `json:"resolution"`
	TurnIndex              int                              `json:"turn_index"`
	CompletedTurns         int                              `json:"completed_turns"`
	LocalTurnIndex         int                              `json:"local_turn_index"`
	LocalTurnSource        string                           `json:"local_turn_source"`
	ProtectedBeforeTurn    int                              `json:"protected_before_turn"`
	MinFromTurn            int                              `json:"min_from_turn"`
	BaselineApplied        bool                             `json:"baseline_applied"`
	ResolvedObservations   []routingTurnResolvedObservation `json:"resolved_observations,omitempty"`
	ObservationCounts      map[string]int                   `json:"observation_counts,omitempty"`
	Worldline              *worldlineViewModel              `json:"worldline,omitempty"`
}

func (s *Server) handleSessionRoutingTurnResolution(w http.ResponseWriter, r *http.Request) {
	var req sessionRoutingTurnResolutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_session_routing_observation"})
		return
	}
	identity := s.resolveObservedRisuSessionIdentity(r.Context(), req)
	if identity.bindingError != nil {
		writeJSON(w, http.StatusOK, sessionRoutingTurnResolutionResponse{
			Status:                 "error",
			ContractVersion:        routingTurnContractVersion,
			Code:                   "session_route_binding_failed",
			ChatSessionID:          strings.TrimSpace(req.ChatSessionID),
			IdentityResolution:     identity.resolution,
			BindingContractVersion: store.SessionRouteBindingContractVersion,
			BindingRequired:        true,
			BindingAcknowledged:    false,
			Resolution:             "binding_failed",
		})
		return
	}
	if identity.sessionID != "" {
		req.ChatSessionID = identity.sessionID
	}
	req.canonicalTailAligned = identity.canonicalTailAligned
	req.Baseline = s.resolveDurableSessionRoutingBaseline(r.Context(), req.ChatSessionID, req.Baseline)
	resp := calculateSessionRoutingTurnResolution(req)
	resp = s.applyAssistantSourceRoutingResolution(r.Context(), req, resp)
	var worldline *worldlineViewModel
	if req.WorldlineObservation != nil {
		resolved := s.resolveRisuWorldlineObservation(r.Context(), req, identity.sessionID)
		worldline = &resolved
	} else if req.Mode == "pair" || req.Mode == "batch" {
		current := currentWorldlineViewModel(r.Context(), s.Store, req.ChatSessionID)
		worldline = &current
	}
	resp = applyAutomaticWorldlineBackfillBoundary(req, resp, worldline)
	if worldline != nil && req.Mode != "pair" && req.Mode != "batch" {
		resp.Worldline = worldline
	}
	resp.ChatSessionID = strings.TrimSpace(req.ChatSessionID)
	resp.IdentityResolution = identity.resolution
	resp.BindingContractVersion = identity.bindingContractVersion
	resp.BindingRequired = identity.bindingRequired
	resp.BindingAcknowledged = identity.bindingAcknowledged
	resp.BindingCreated = identity.bindingCreated
	resp.BindingUpdated = identity.bindingUpdated
	resp.LockedSourceRedirect = identity.lockedSourceRedirect
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) applyAssistantSourceRoutingResolution(
	ctx context.Context,
	req sessionRoutingTurnResolutionRequest,
	resp sessionRoutingTurnResolutionResponse,
) sessionRoutingTurnResolutionResponse {
	if req.Mode != "batch" || len(req.Observations) == 0 {
		return resp
	}
	hasAssistantEvidence := false
	for _, observation := range req.Observations {
		if strings.TrimSpace(observation.AssistantMessageID) != "" ||
			strings.TrimSpace(observation.AssistantGenerationID) != "" ||
			strings.TrimSpace(observation.AssistantContentHash) != "" {
			hasAssistantEvidence = true
			break
		}
	}
	if !hasAssistantEvidence {
		return resp
	}
	var sources []store.MemorySourceRevision
	var err error
	if strings.TrimSpace(req.RoutingContext) == automaticActiveChatFullSweep {
		if history, ok := s.Store.(store.SourceRevisionHistoryLister); ok {
			sources, err = history.ListSourceRevisions(ctx, strings.TrimSpace(req.ChatSessionID), 0, 0)
		} else if active, ok := s.Store.(store.ActiveSourceRevisionLister); ok {
			sources, err = active.ListActiveSourceRevisions(ctx, strings.TrimSpace(req.ChatSessionID), 0, 0)
		} else {
			resp.Status = "error"
			resp.Code = "assistant_source_resolution_unavailable"
			resp.Resolution = "assistant_source_resolution_unavailable"
			resp.ResolvedObservations = nil
			return resp
		}
	} else if active, ok := s.Store.(store.ActiveSourceRevisionLister); ok {
		sources, err = active.ListActiveSourceRevisions(ctx, strings.TrimSpace(req.ChatSessionID), 0, 0)
	} else {
		resp.Status = "error"
		resp.Code = "assistant_source_resolution_unavailable"
		resp.Resolution = "assistant_source_resolution_unavailable"
		resp.ResolvedObservations = nil
		return resp
	}
	if err != nil {
		resp.Status = "error"
		resp.Code = "assistant_source_resolution_failed"
		resp.Resolution = "assistant_source_resolution_failed"
		resp.ResolvedObservations = nil
		return resp
	}
	if strings.TrimSpace(req.RoutingContext) == automaticActiveChatFullSweep && s.Store != nil {
		sourceTurns := map[int]bool{}
		for _, source := range sources {
			if source.TurnIndex > 0 {
				sourceTurns[source.TurnIndex] = true
			}
		}
		if logs, listErr := s.Store.ListChatLogs(ctx, strings.TrimSpace(req.ChatSessionID), 0, 0); listErr == nil {
			rolesByTurn := map[int]map[string]string{}
			for _, log := range logs {
				if log.TurnIndex <= 0 || sourceTurns[log.TurnIndex] {
					continue
				}
				role := strings.ToLower(strings.TrimSpace(log.Role))
				if role != "user" && role != "assistant" {
					continue
				}
				if rolesByTurn[log.TurnIndex] == nil {
					rolesByTurn[log.TurnIndex] = map[string]string{}
				}
				rolesByTurn[log.TurnIndex][role] = log.Content
			}
			for turn, roles := range rolesByTurn {
				assistant := strings.TrimSpace(roles["assistant"])
				if assistant == "" {
					continue
				}
				sources = append(sources, store.MemorySourceRevision{
					ChatSessionID:                strings.TrimSpace(req.ChatSessionID),
					TurnIndex:                    turn,
					UserContent:                  strings.TrimSpace(roles["user"]),
					AssistantContent:             assistant,
					AssistantObservedContentHash: prepareOR1CHash(assistant),
					LifecycleState:               "canonical_raw",
				})
			}
		}
	}
	candidateSources := make([]store.MemorySourceRevision, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.ChatSessionID) != strings.TrimSpace(req.ChatSessionID) ||
			source.TurnIndex <= 0 || strings.TrimSpace(source.AssistantContent) == "" {
			continue
		}
		candidateSources = append(candidateSources, source)
	}
	sort.SliceStable(candidateSources, func(i, j int) bool {
		if candidateSources[i].TurnIndex != candidateSources[j].TurnIndex {
			return candidateSources[i].TurnIndex < candidateSources[j].TurnIndex
		}
		leftActive := candidateSources[i].LifecycleState == "active"
		rightActive := candidateSources[j].LifecycleState == "active"
		if leftActive != rightActive {
			return leftActive
		}
		if candidateSources[i].HostObservedAtMS != candidateSources[j].HostObservedAtMS {
			return candidateSources[i].HostObservedAtMS > candidateSources[j].HostObservedAtMS
		}
		return candidateSources[i].ID > candidateSources[j].ID
	})

	usedObservations := make([]bool, len(req.Observations))
	matchedSources := make([]bool, len(candidateSources))
	matchedTurns := make(map[int]bool, len(candidateSources))
	matches := make(map[int]int, len(candidateSources))
	matchPass := func(identityOnly bool) {
		for sourceIndex, source := range candidateSources {
			if matchedSources[sourceIndex] || matchedTurns[source.TurnIndex] {
				continue
			}
			for observationIndex, observation := range req.Observations {
				if usedObservations[observationIndex] {
					continue
				}
				candidate := rollbackAssistantObservation{
					MessageID:      observation.AssistantMessageID,
					GenerationID:   observation.AssistantGenerationID,
					ContentHash:    observation.AssistantContentHash,
					DisabledState:  observation.AssistantDisabledState,
					StreamingState: observation.AssistantStreamingState,
					FinalState:     observation.AssistantFinalState,
				}
				if !rollbackAssistantObservationEligible(candidate) {
					continue
				}
				if !rollbackAssistantObservationMatchesSource(source, candidate, identityOnly) {
					continue
				}
				matches[observationIndex] = sourceIndex
				usedObservations[observationIndex] = true
				matchedTurns[source.TurnIndex] = true
				for relatedIndex := range candidateSources {
					if candidateSources[relatedIndex].TurnIndex == source.TurnIndex {
						matchedSources[relatedIndex] = true
					}
				}
				break
			}
		}
	}
	// Stable RisuAI message/generation identity wins. Content hash is only the
	// fallback for hosts or historical rows that did not expose an identity.
	matchPass(true)
	matchPass(false)

	for index := range resp.ResolvedObservations {
		if index >= len(req.Observations) {
			break
		}
		observation := req.Observations[index]
		candidate := rollbackAssistantObservation{
			DisabledState:  observation.AssistantDisabledState,
			StreamingState: observation.AssistantStreamingState,
			FinalState:     observation.AssistantFinalState,
		}
		if !rollbackAssistantObservationEligible(candidate) {
			resp.ResolvedObservations[index].Resolution = "inactive_assistant_observation"
			resp.ResolvedObservations[index].Source = "host_observation"
			resp.ResolvedObservations[index].TurnIndex = 0
			resp.ResolvedObservations[index].TurnIdentityState = "inactive"
			continue
		}
		if strings.TrimSpace(observation.AssistantMessageID) == "" &&
			strings.TrimSpace(observation.AssistantGenerationID) == "" &&
			strings.TrimSpace(observation.AssistantContentHash) == "" {
			continue
		}
		sourceIndex, matched := matches[index]
		if !matched {
			resp.ResolvedObservations[index].Resolution = "new_assistant_observation"
			resp.ResolvedObservations[index].Source = "assistant_observation_order"
			resp.ResolvedObservations[index].StoredAssistantContent = observation.AssistantContent
			if observation.AdjacentUserPresent {
				resp.ResolvedObservations[index].InputMode = "paired"
				resp.ResolvedObservations[index].UserInputState = "observed"
				resp.ResolvedObservations[index].StoredUserContent = observation.AdjacentUserContent
			} else {
				resp.ResolvedObservations[index].InputMode = "assistant_only"
				resp.ResolvedObservations[index].UserInputState = "missing"
			}
			continue
		}
		source := candidateSources[sourceIndex]
		resp.ResolvedObservations[index].LocalTurnIndex = source.TurnIndex
		resp.ResolvedObservations[index].TurnIndex = source.TurnIndex
		resp.ResolvedObservations[index].Resolution = "existing_turn_by_assistant_source"
		resp.ResolvedObservations[index].Source = "source_revision"
		resp.ResolvedObservations[index].SourceRevision = strings.TrimSpace(source.SourceRevision)
		resp.ResolvedObservations[index].SourceLifecycleState = strings.TrimSpace(source.LifecycleState)
		resp.ResolvedObservations[index].TurnIdentityState = "source_matched"
		resp.ResolvedObservations[index].StoredAssistantContent = extractionFirstNonEmpty(observation.AssistantContent, source.AssistantContent)
		switch {
		case observation.AdjacentUserPresent:
			resp.ResolvedObservations[index].InputMode = "paired"
			resp.ResolvedObservations[index].UserInputState = "observed"
			resp.ResolvedObservations[index].StoredUserContent = observation.AdjacentUserContent
		case strings.TrimSpace(source.UserContent) != "":
			resp.ResolvedObservations[index].InputMode = "stored_pair_recovered"
			resp.ResolvedObservations[index].UserInputState = "restored_from_source_revision"
			resp.ResolvedObservations[index].StoredUserContent = source.UserContent
		default:
			resp.ResolvedObservations[index].InputMode = "assistant_only"
			resp.ResolvedObservations[index].UserInputState = "missing"
		}
	}
	resp = resolveAssistantObservationTurnIdentities(req, candidateSources, resp)
	resp.ObservationCounts = map[string]int{}
	for _, item := range resp.ResolvedObservations {
		if item.InputMode != "" {
			resp.ObservationCounts[item.InputMode]++
		}
		if item.TurnIdentityState == "inactive" {
			resp.ObservationCounts["inactive"]++
		} else if item.TurnIdentityState == "unresolved" {
			resp.ObservationCounts["unresolved"]++
		} else if item.TurnIndex > 0 {
			resp.ObservationCounts["resolved"]++
		}
	}
	return resp
}

func resolveAssistantObservationTurnIdentities(
	req sessionRoutingTurnResolutionRequest,
	sources []store.MemorySourceRevision,
	resp sessionRoutingTurnResolutionResponse,
) sessionRoutingTurnResolutionResponse {
	if len(resp.ResolvedObservations) == 0 {
		return resp
	}
	eligibleIndices := []int{}
	for index := range resp.ResolvedObservations {
		if resp.ResolvedObservations[index].TurnIdentityState != "inactive" {
			eligibleIndices = append(eligibleIndices, index)
		}
	}
	anchorPositions := []int{}
	for position, index := range eligibleIndices {
		if resp.ResolvedObservations[index].TurnIdentityState == "source_matched" {
			anchorPositions = append(anchorPositions, position)
		}
	}
	markResolved := func(index, turn int, state string) {
		item := &resp.ResolvedObservations[index]
		item.LocalTurnIndex = turn
		item.TurnIndex = turn
		item.TurnIdentityState = state
	}
	markUnresolved := func(fromPosition, toPosition int) {
		for position := fromPosition; position < toPosition; position++ {
			index := eligibleIndices[position]
			if resp.ResolvedObservations[index].TurnIdentityState == "source_matched" {
				continue
			}
			resp.ResolvedObservations[index].TurnIndex = 0
			resp.ResolvedObservations[index].Resolution = "assistant_turn_identity_unresolved"
			resp.ResolvedObservations[index].TurnIdentityState = "unresolved"
		}
	}
	if len(anchorPositions) == 0 {
		if len(sources) > 0 {
			markUnresolved(0, len(eligibleIndices))
			return resp
		}
		for position, index := range eligibleIndices {
			turn := resp.ResolvedObservations[index].TurnIndex
			if turn <= 0 {
				turn = position + 1
			}
			markResolved(index, turn, "ordered_new_session")
		}
		return resp
	}

	firstAnchorPosition := anchorPositions[0]
	firstAnchorIndex := eligibleIndices[firstAnchorPosition]
	firstTurn := resp.ResolvedObservations[firstAnchorIndex].TurnIndex
	if firstTurn == firstAnchorPosition+1 {
		for position := 0; position < firstAnchorPosition; position++ {
			markResolved(eligibleIndices[position], position+1, "source_bounded_order")
		}
	} else {
		markUnresolved(0, firstAnchorPosition)
	}
	for anchorIndex := 0; anchorIndex+1 < len(anchorPositions); anchorIndex++ {
		leftPosition, rightPosition := anchorPositions[anchorIndex], anchorPositions[anchorIndex+1]
		leftIndex, rightIndex := eligibleIndices[leftPosition], eligibleIndices[rightPosition]
		leftTurn := resp.ResolvedObservations[leftIndex].TurnIndex
		rightTurn := resp.ResolvedObservations[rightIndex].TurnIndex
		gapCount := rightPosition - leftPosition - 1
		if rightTurn-leftTurn == gapCount+1 {
			for offset := 1; offset <= gapCount; offset++ {
				markResolved(eligibleIndices[leftPosition+offset], leftTurn+offset, "source_bounded_order")
			}
		} else {
			markUnresolved(leftPosition+1, rightPosition)
		}
	}
	lastAnchorPosition := anchorPositions[len(anchorPositions)-1]
	lastAnchorIndex := eligibleIndices[lastAnchorPosition]
	lastTurn := resp.ResolvedObservations[lastAnchorIndex].TurnIndex
	hasLaterStoredTurn := false
	for _, source := range sources {
		if source.TurnIndex > lastTurn {
			hasLaterStoredTurn = true
			break
		}
	}
	if hasLaterStoredTurn {
		markUnresolved(lastAnchorPosition+1, len(eligibleIndices))
	} else {
		for position := lastAnchorPosition + 1; position < len(eligibleIndices); position++ {
			markResolved(eligibleIndices[position], lastTurn+(position-lastAnchorPosition), "source_tail_order")
		}
	}
	return resp
}

func (s *Server) resolveRisuWorldlineObservation(ctx context.Context, req sessionRoutingTurnResolutionRequest, childSessionID string) (vm worldlineViewModel) {
	observation := req.WorldlineObservation
	ancestorRead := s.resolveRisuWorldlineParentObservation(ctx, req)
	durable := currentWorldlineViewModel(ctx, s.Store, childSessionID)
	defer func() {
		if ancestorRead != nil {
			vm.OriginReadRequest = ancestorRead
			return
		}
		if observation == nil || vm.MessageOriginsRecorded || observation.MessageOrigins != nil {
			return
		}
		parent, _, source, ok := parseExactRisuBranchMarker(observation.BranchMarker)
		if ok {
			vm.OriginReadRequest = &risuWorldlineOriginReadRequest{ParentHostChatID: parent, SourceMessageID: source, ChildHostChatID: req.HostChatID, ChildMarkerIndex: observation.MarkerIndex}
		}
	}()
	if durable.State == "confirmed" && durable.ForkSourceRole != "" {
		if observation != nil && observation.MessageOrigins != nil {
			s.enrichRisuWorldlineOrigins(ctx, childSessionID, req.StableCharacterID, observation)
			return currentWorldlineViewModel(ctx, s.Store, childSessionID)
		}
		return durable
	}
	vm = worldlineViewModel{
		ContractVersion:  worldlineViewModelContract,
		State:            "unresolved",
		CurrentSessionID: strings.TrimSpace(childSessionID),
		Reason:           "worldline_observation_unresolved",
	}
	if observation == nil {
		return durable
	}
	markerState := strings.TrimSpace(observation.MarkerState)
	marker := strings.TrimSpace(observation.BranchMarker)
	if markerState == "absent" && marker == "" {
		if durable.State != "not_applicable" {
			return durable
		}
		durable.Reason = "official_branch_marker_absent"
		return durable
	}
	assessmentParentSessionID := ""
	assessmentSourceMessageID := ""
	assessmentSourceRole := ""
	assessmentForkTurn := 0
	assessmentCoordinates := marker
	defer func() {
		persisted, err := persistRisuWorldlineAssessment(
			ctx, s.Store, vm, observation, assessmentParentSessionID,
			assessmentSourceMessageID, assessmentSourceRole, assessmentForkTurn, assessmentCoordinates,
		)
		if err != nil {
			vm.State = "unresolved"
			vm.ParentSessionID = ""
			vm.ForkTurn = 0
			vm.ForkSourceMessageID = ""
			vm.Reason = "worldline_assessment_persistence_failed"
			return
		}
		vm = persisted
	}()
	if observation.ContractVersion != risuWorldlineObservationContract ||
		!risuWorldlineHostSignalSupported(observation.HostSignalSource) ||
		observation.BranchShapeContract != risuBranchShapeContract ||
		observation.ObservedAtMS <= 0 ||
		markerState != "observed" {
		vm.State = "conflict"
		vm.Reason = "worldline_observation_contract_conflict"
		return vm
	}
	if strings.TrimSpace(req.HostChatIDState) != "observed" || strings.TrimSpace(req.HostChatID) == "" {
		vm.Reason = "child_host_chat_unresolved"
		return vm
	}
	parentHostChatID, _, sourceMessageID, ok := parseExactRisuBranchMarker(marker)
	if !ok {
		vm.State = "conflict"
		vm.Reason = "official_branch_marker_malformed"
		return vm
	}
	assessmentSourceMessageID = sourceMessageID
	assessmentCoordinates = strings.Join([]string{parentHostChatID, sourceMessageID}, "\x1f")
	if parentHostChatID == strings.TrimSpace(req.HostChatID) {
		vm.State = "conflict"
		vm.Reason = "branch_parent_matches_child_chat"
		return vm
	}

	messageByIndex := make(map[int]risuWorldlineMessageObservation, len(observation.Messages))
	if len(observation.Messages) < 2 || len(observation.Messages) > 3 {
		vm.State = "conflict"
		vm.Reason = "worldline_message_observation_not_bounded"
		return vm
	}
	for _, message := range observation.Messages {
		if message.MessageIndex < 0 {
			continue
		}
		if _, exists := messageByIndex[message.MessageIndex]; exists {
			vm.State = "conflict"
			vm.Reason = "worldline_message_index_conflict"
			return vm
		}
		messageByIndex[message.MessageIndex] = message
	}
	markerMessage, markerPresent := messageByIndex[observation.MarkerIndex]
	if !markerPresent || !markerMessage.Disabled || observation.MarkerIndex <= 0 {
		vm.State = "unresolved"
		vm.Reason = "branch_marker_message_unresolved"
		return vm
	}
	sourceMessage, sourcePresent := messageByIndex[observation.MarkerIndex-1]
	if !sourcePresent || strings.TrimSpace(sourceMessage.MessageChatID) == "" {
		vm.State = "unresolved"
		vm.Reason = "fork_source_message_unresolved"
		return vm
	}
	if sourceMessage.Disabled || (sourceMessage.Role != "user" && sourceMessage.Role != "char") {
		vm.Reason = "fork_source_message_unresolved"
		return vm
	}
	sourceRole := strings.TrimSpace(sourceMessage.Role)
	assessmentSourceRole = sourceRole
	userAnchorMessageID := ""
	switch sourceRole {
	case "user":
		if len(observation.Messages) != 2 {
			vm.State = "conflict"
			vm.Reason = "worldline_message_observation_not_bounded"
			return vm
		}
		userAnchorMessageID = sourceMessageID
	case "char":
		if len(observation.Messages) != 3 {
			vm.Reason = "fork_user_anchor_unresolved"
			return vm
		}
		var anchor *risuWorldlineMessageObservation
		for index := range observation.Messages {
			candidate := observation.Messages[index]
			if candidate.MessageIndex == observation.MarkerIndex || candidate.MessageIndex == observation.MarkerIndex-1 {
				continue
			}
			if anchor != nil {
				vm.State = "conflict"
				vm.Reason = "fork_user_anchor_conflict"
				return vm
			}
			anchor = &observation.Messages[index]
		}
		if anchor == nil || anchor.MessageIndex < 0 || anchor.MessageIndex >= observation.MarkerIndex-1 ||
			anchor.Role != "user" || anchor.Disabled || strings.TrimSpace(anchor.MessageChatID) == "" {
			vm.Reason = "fork_user_anchor_unresolved"
			return vm
		}
		userAnchorMessageID = strings.TrimSpace(anchor.MessageChatID)
	}
	// The marker names a message in the parent, not an ID in the child.
	// Official hosts may either preserve or reissue the child's IDs.
	if parentAnchor := risuWorldlineParentUserAnchor(observation, parentHostChatID, sourceMessageID); parentAnchor != "" {
		userAnchorMessageID = parentAnchor
	}
	bindingStore, ok := s.Store.(store.SessionRouteBindingStore)
	if !ok {
		vm.Reason = "parent_route_store_unavailable"
		return vm
	}
	parentBinding, err := bindingStore.BindSessionRoute(ctx, store.SessionRouteBindingRequest{
		StableCharacterID: strings.TrimSpace(req.StableCharacterID),
		HostChatID:        parentHostChatID,
		Mode:              store.SessionRouteBindingModeResolveExisting,
	})
	if err != nil || parentBinding == nil || !parentBinding.ReadbackVerified {
		vm.Reason = "parent_route_unresolved"
		return vm
	}
	parentSessionID := strings.TrimSpace(parentBinding.Binding.CanonicalSessionID)
	if parentSessionID == "" {
		vm.Reason = "parent_session_unresolved"
		return vm
	}
	if parentSessionID == strings.TrimSpace(childSessionID) {
		vm.State = "conflict"
		vm.Reason = "parent_child_session_conflict"
		return vm
	}
	activeSourceLister, ok := s.Store.(store.ActiveSourceRevisionLister)
	if !ok {
		vm.Reason = "parent_active_source_store_unavailable"
		return vm
	}
	parentSources, err := activeSourceLister.ListActiveSourceRevisions(ctx, parentSessionID, 0, 0)
	if err != nil {
		vm.Reason = "parent_active_source_read_unavailable"
		return vm
	}
	expectedUserLogicalTurnID := completeTurnLogicalTurnID(parentSessionID, completeTurnSourceObservation{
		HostChatID:             parentHostChatID,
		HostChatIDState:        "observed",
		UserMessageChatID:      userAnchorMessageID,
		UserMessageChatIDState: "observed",
	})
	matchingTurns := prioritizedWorldlineSourceTurns(
		parentSources, expectedUserLogicalTurnID, sourceRole, sourceMessageID,
	)
	if len(matchingTurns) > 1 {
		vm.State = "conflict"
		vm.Reason = "parent_active_fork_source_conflict"
		vm.CandidateParentID = parentSessionID
		vm.CandidateForkTurns = matchingTurns
		assessmentParentSessionID = parentSessionID
		return vm
	}
	confirmedReason := "official_branch_marker_validated"
	if len(matchingTurns) == 0 {
		historyStore, historyOK := s.Store.(store.SourceRevisionHistoryLister)
		if !historyOK {
			vm.Reason = "parent_source_history_store_unavailable"
			vm.CandidateParentID = parentSessionID
			assessmentParentSessionID = parentSessionID
			return vm
		}
		history, historyErr := historyStore.ListSourceRevisions(ctx, parentSessionID, 0, 0)
		if historyErr != nil {
			vm.Reason = "parent_source_history_read_unavailable"
			vm.CandidateParentID = parentSessionID
			assessmentParentSessionID = parentSessionID
			return vm
		}
		matchingTurns = prioritizedWorldlineSourceTurns(
			history, expectedUserLogicalTurnID, sourceRole, sourceMessageID,
		)
		if len(matchingTurns) == 0 {
			matchingTurns = s.risuWorldlineInheritedSourceTurns(ctx, parentSessionID, parentHostChatID, sourceMessageID, userAnchorMessageID, sourceRole)
		}
		if len(matchingTurns) == 0 {
			if turn := s.risuWorldlineObservedSourceTurn(ctx, observation, parentSessionID, parentHostChatID, sourceMessageID, append(parentSources, history...)); turn > 0 {
				matchingTurns = []int{turn}
				confirmedReason = "official_branch_marker_observed_prefix_validated"
			}
		}
		vm.CandidateParentID = parentSessionID
		vm.CandidateForkTurns = matchingTurns
		assessmentParentSessionID = parentSessionID
		switch len(matchingTurns) {
		case 0:
			vm.Reason = "parent_fork_source_history_unresolved"
			return vm
		case 1:
			if confirmedReason != "official_branch_marker_observed_prefix_validated" {
				confirmedReason = "official_branch_marker_historical_source_validated"
			}
		default:
			vm.State = "conflict"
			vm.Reason = "parent_fork_source_history_ambiguous"
			return vm
		}
	}
	forkTurn := matchingTurns[0]
	inheritedThroughTurn, _ := worldlineInheritedThroughTurn(forkTurn, sourceRole)
	assessmentParentSessionID = parentSessionID
	assessmentForkTurn = forkTurn
	vm = worldlineViewModel{
		ContractVersion:      worldlineViewModelContract,
		State:                "confirmed",
		CurrentSessionID:     strings.TrimSpace(childSessionID),
		ParentSessionID:      parentSessionID,
		ForkTurn:             forkTurn,
		ForkSourceMessageID:  sourceMessageID,
		ForkSourceRole:       sourceRole,
		InheritedThroughTurn: inheritedThroughTurn,
		Reason:               confirmedReason,
	}
	return vm
}

func persistRisuWorldlineAssessment(
	ctx context.Context,
	st store.Store,
	vm worldlineViewModel,
	observation *risuWorldlineObservation,
	parentSessionID, sourceMessageID, sourceRole string,
	forkTurn int,
	coordinates string,
) (worldlineViewModel, error) {
	lineageStore, ok := st.(store.ForkLineageStore)
	if !ok || observation == nil {
		return vm, errors.New("fork lineage store unavailable")
	}
	candidateForkTurns := append([]int(nil), vm.CandidateForkTurns...)
	if candidateForkTurns == nil {
		candidateForkTurns = []int{}
	}
	reasonJSON, err := json.Marshal(map[string]any{
		"reason":                      strings.TrimSpace(vm.Reason),
		"candidate_parent_session_id": strings.TrimSpace(vm.CandidateParentID),
		"candidate_fork_turns":        candidateForkTurns,
	})
	if err != nil {
		return vm, err
	}
	idempotencyHash := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(coordinates),
	}, "\x1f")))
	record := store.ForkLineageRecord{
		ContractVersion:     store.RisuWorldlineForkLineageContractVersion,
		LineageState:        strings.TrimSpace(vm.State),
		ChatSessionID:       strings.TrimSpace(vm.CurrentSessionID),
		CopiedFromSessionID: "",
		ForkSourceMessageID: strings.TrimSpace(sourceMessageID),
		ForkSourceRole:      strings.TrimSpace(sourceRole),
		IdempotencyKey:      "risu-worldline:" + hex.EncodeToString(idempotencyHash[:]),
		ImportedAt:          time.UnixMilli(observation.ObservedAtMS).UTC(),
		DivergenceMarker:    string(reasonJSON),
		ProvenanceSource:    "automatic_hook",
		InheritanceMode:     "none",
	}
	if record.LineageState == "confirmed" {
		record.ForkTurn = forkTurn
		record.CopiedFromSessionID = strings.TrimSpace(parentSessionID)
		record.InheritedItemsJSON = risuWorldlineOriginItems(observation)
	}
	saved, err := lineageStore.SaveForkLineageRecord(ctx, record)
	if err != nil {
		return vm, err
	}
	return worldlineViewModelFromRecord(saved), nil
}

func uniqueWorldlineSourceTurns(sources []store.MemorySourceRevision) []int {
	seen := make(map[int]struct{}, len(sources))
	turns := make([]int, 0, len(sources))
	for _, source := range sources {
		if source.TurnIndex <= 0 {
			continue
		}
		if _, exists := seen[source.TurnIndex]; exists {
			continue
		}
		seen[source.TurnIndex] = struct{}{}
		turns = append(turns, source.TurnIndex)
	}
	sort.Ints(turns)
	return turns
}

func prioritizedWorldlineSourceTurns(
	sources []store.MemorySourceRevision,
	expectedUserLogicalTurnID string,
	sourceRole string,
	sourceMessageID string,
) []int {
	if strings.TrimSpace(sourceRole) == "char" && strings.TrimSpace(sourceMessageID) != "" {
		exactSourceMatches := make([]store.MemorySourceRevision, 0, 2)
		for _, source := range sources {
			if source.TurnIndex > 0 && strings.TrimSpace(source.SourceMessageID) == strings.TrimSpace(sourceMessageID) {
				exactSourceMatches = append(exactSourceMatches, source)
			}
		}
		if turns := uniqueWorldlineSourceTurns(exactSourceMatches); len(turns) > 0 {
			return turns
		}
	}
	logicalMatches := make([]store.MemorySourceRevision, 0, 2)
	for _, source := range sources {
		if source.TurnIndex > 0 && strings.TrimSpace(source.LogicalTurnID) == strings.TrimSpace(expectedUserLogicalTurnID) {
			logicalMatches = append(logicalMatches, source)
		}
	}
	return uniqueWorldlineSourceTurns(logicalMatches)
}

func risuWorldlineHostSignalSupported(source string) bool {
	switch strings.TrimSpace(source) {
	case "output", "active_chat_pre_backfill":
		return true
	default:
		return false
	}
}

func applyAutomaticWorldlineBackfillBoundary(
	req sessionRoutingTurnResolutionRequest,
	resp sessionRoutingTurnResolutionResponse,
	worldline *worldlineViewModel,
) sessionRoutingTurnResolutionResponse {
	if req.Mode != "pair" && req.Mode != "batch" {
		return resp
	}
	if worldline == nil || worldline.State == "not_applicable" {
		return resp
	}
	resp.Worldline = worldline
	boundary, ok := worldlineInheritedThroughTurn(worldline.ForkTurn, worldline.ForkSourceRole)
	if worldline.State != "confirmed" || !ok {
		resp.Code = "worldline_ownership_unresolved"
		resp.Resolution = "worldline_ownership_unresolved"
		resp.TurnIndex = 0
		for index := range resp.ResolvedObservations {
			resp.ResolvedObservations[index].Resolution = "worldline_ownership_unresolved"
			resp.ResolvedObservations[index].TurnIndex = 0
		}
		return resp
	}
	if req.Baseline != nil && req.Baseline.durableVerified &&
		(strings.TrimSpace(req.Baseline.durableSourceID) != strings.TrimSpace(worldline.ParentSessionID) ||
			req.Baseline.BackendTurnAtRoute != boundary) {
		resp.Code = "worldline_ownership_unresolved"
		resp.Resolution = "worldline_ownership_unresolved"
		resp.TurnIndex = 0
		for index := range resp.ResolvedObservations {
			resp.ResolvedObservations[index].Resolution = "worldline_ownership_unresolved"
			resp.ResolvedObservations[index].TurnIndex = 0
		}
		return resp
	}
	if req.Mode == "batch" {
		for index := range resp.ResolvedObservations {
			item := &resp.ResolvedObservations[index]
			if item.TurnIdentityState == "source_matched" {
				continue
			}
			if item.TurnIdentityState == "unresolved" {
				item.Resolution = "assistant_turn_identity_unresolved"
				item.TurnIndex = 0
				continue
			}
			if item.ObservedPairOrdinal > 0 {
				item.LocalTurnIndex = item.ObservedPairOrdinal
				item.Source = "observed_pair_ordinal"
			}
			if item.LocalTurnIndex > 0 && item.LocalTurnIndex <= boundary {
				item.Resolution = "skip_pre_route_visible_pair"
				item.TurnIndex = item.LocalTurnIndex
				continue
			}
			if item.LocalTurnIndex > boundary {
				item.Resolution = "normal"
				item.TurnIndex = item.LocalTurnIndex
			}
		}
		return resp
	}
	if req.ObservedPairOrdinal > 0 {
		resp.LocalTurnIndex = req.ObservedPairOrdinal
		resp.LocalTurnSource = "observed_pair_ordinal"
	}
	resp.ProtectedBeforeTurn = boundary
	resp.MinFromTurn = boundary + 1
	if resp.LocalTurnIndex > 0 && resp.LocalTurnIndex <= boundary {
		resp.Resolution = "skip_pre_route_visible_pair"
		resp.TurnIndex = resp.LocalTurnIndex
		resp.BaselineApplied = req.Baseline != nil && req.Baseline.durableVerified
		return resp
	}
	if resp.LocalTurnIndex > boundary {
		resp.Resolution = "normal"
		resp.TurnIndex = resp.LocalTurnIndex
		resp.BaselineApplied = req.Baseline != nil && req.Baseline.durableVerified
	}
	return resp
}

func parseExactRisuBranchMarker(marker string) (parentHostChatID, parentChatName, sourceMessageID string, ok bool) {
	const prefix = "{{specialcomment::branchedfrom::"
	const suffix = "::}}"
	if !strings.HasPrefix(marker, prefix) || !strings.HasSuffix(marker, suffix) {
		return "", "", "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(marker, prefix), suffix)
	firstSeparator := strings.Index(body, "::")
	lastSeparator := strings.LastIndex(body, "::")
	if firstSeparator < 0 || lastSeparator <= firstSeparator {
		return "", "", "", false
	}
	parentHostChatID = strings.TrimSpace(body[:firstSeparator])
	parentChatName = strings.TrimSpace(body[firstSeparator+2 : lastSeparator])
	sourceMessageID = strings.TrimSpace(body[lastSeparator+2:])
	if parentHostChatID == "" || sourceMessageID == "" {
		return "", "", "", false
	}
	return parentHostChatID, parentChatName, sourceMessageID, true
}

type observedRisuSessionIdentityResolution struct {
	sessionID              string
	resolution             string
	canonicalTailAligned   bool
	bindingContractVersion string
	bindingRequired        bool
	bindingAcknowledged    bool
	bindingCreated         bool
	bindingUpdated         bool
	lockedSourceRedirect   bool
	bindingError           error
}

func (s *Server) resolveObservedRisuSessionIdentity(ctx context.Context, req sessionRoutingTurnResolutionRequest) observedRisuSessionIdentityResolution {
	requested := strings.TrimSpace(req.ChatSessionID)
	hostChatID := strings.TrimSpace(req.HostChatID)
	stableCharacterID := strings.TrimSpace(req.StableCharacterID)
	if req.StableCharacterIDState == "observed" && stableCharacterID != "" &&
		req.HostChatIDState == "observed" && hostChatID != "" {
		resolution := observedRisuSessionIdentityResolution{
			sessionID:              requested,
			resolution:             "durable_binding_required",
			bindingContractVersion: store.SessionRouteBindingContractVersion,
			bindingRequired:        true,
		}
		bindingStore, ok := s.Store.(store.SessionRouteBindingStore)
		if !ok {
			resolution.bindingError = errors.New("session route binding store unavailable")
			return resolution
		}
		bindingMode := strings.TrimSpace(req.BindingMode)
		if bindingMode == "" {
			bindingMode = store.SessionRouteBindingModeResolveOrCreate
		}
		if req.BindRequestedSession && bindingMode == store.SessionRouteBindingModeResolveOrCreate {
			bindingMode = store.SessionRouteBindingModeManualAttach
		}
		result, err := bindingStore.BindSessionRoute(ctx, store.SessionRouteBindingRequest{
			StableCharacterID:  stableCharacterID,
			HostChatID:         hostChatID,
			RequestedSessionID: requested,
			Mode:               bindingMode,
		})
		if err != nil || result == nil || !result.ReadbackVerified ||
			strings.TrimSpace(result.Binding.CanonicalSessionID) == "" {
			if err == nil {
				err = errors.New("session route binding readback unverified")
			}
			resolution.bindingError = err
			return resolution
		}
		resolution.sessionID = strings.TrimSpace(result.Binding.CanonicalSessionID)
		resolution.bindingAcknowledged = true
		resolution.bindingCreated = result.Created
		resolution.bindingUpdated = result.Updated
		resolution.lockedSourceRedirect = result.LockedSourceRedirect
		switch {
		case result.LockedSourceRedirect:
			resolution.resolution = "durable_binding_locked_source_redirect"
		case result.Created:
			resolution.resolution = "durable_binding_created"
		case result.Updated:
			resolution.resolution = "durable_binding_updated"
		default:
			resolution.resolution = "durable_binding_existing"
		}
		return resolution
	}
	if req.HostChatIDState != "observed" || hostChatID == "" || s.Store == nil {
		return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "host_chat_id_unobserved"}
	}
	sessions, err := s.Store.ListSessions(ctx)
	if err != nil {
		return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "session_list_unavailable"}
	}
	suffix := "_cid_" + hostChatID
	candidates := make([]store.SessionSummary, 0, 2)
	for _, session := range sessions {
		sid := strings.TrimSpace(session.ChatSessionID)
		if sid == "cid_"+hostChatID || strings.HasSuffix(sid, suffix) {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "new_host_chat_id"}
	}

	userHash := strings.TrimSpace(req.LatestUserHash)
	assistantHash := strings.TrimSpace(req.LatestAssistantHash)
	turnMismatchObserved := false
	if userHash != "" || assistantHash != "" {
		matched := make([]store.SessionSummary, 0, len(candidates))
		for _, candidate := range candidates {
			logs, listErr := s.Store.ListChatLogs(ctx, candidate.ChatSessionID, 0, 0)
			if listErr != nil {
				continue
			}
			latestTurn := 0
			latestUser, latestAssistant := "", ""
			for _, log := range logs {
				if log.TurnIndex > latestTurn {
					latestTurn = log.TurnIndex
					latestUser, latestAssistant = "", ""
				}
				if log.TurnIndex != latestTurn {
					continue
				}
				switch strings.ToLower(strings.TrimSpace(log.Role)) {
				case "user":
					latestUser = log.Content
				case "assistant":
					latestAssistant = log.Content
				}
			}
			userMatches := userHash == "" || prepareOR1CHash(strings.TrimSpace(latestUser)) == userHash
			assistantMatches := assistantHash == "" || prepareOR1CHash(strings.TrimSpace(latestAssistant)) == assistantHash
			if userMatches && assistantMatches {
				if req.VisibleCompletedTurns > 0 && latestTurn != req.VisibleCompletedTurns {
					turnMismatchObserved = true
					continue
				}
				matched = append(matched, candidate)
			}
		}
		if len(matched) == 1 {
			return observedRisuSessionIdentityResolution{sessionID: strings.TrimSpace(matched[0].ChatSessionID), resolution: "existing_host_chat_tail_match", canonicalTailAligned: true}
		}
		if len(matched) > 1 {
			candidates = matched
		} else if len(candidates) == 1 {
			// A matching CID alone is not enough to reconnect an index alias.
			// Copy/cold-start damage can leave that CID attached to a backend
			// tail that is not the chat RisuAI is currently showing.
			if turnMismatchObserved {
				return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "existing_host_chat_turn_mismatch"}
			}
			return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "existing_host_chat_tail_mismatch"}
		}
	}
	if len(candidates) == 1 {
		return observedRisuSessionIdentityResolution{sessionID: strings.TrimSpace(candidates[0].ChatSessionID), resolution: "existing_host_chat_id"}
	}

	best := candidates[0]
	bestTied := false
	for _, candidate := range candidates[1:] {
		if candidate.ChatLogsCount > best.ChatLogsCount {
			best = candidate
			bestTied = false
		} else if candidate.ChatLogsCount == best.ChatLogsCount {
			bestTied = true
		}
	}
	if !bestTied {
		return observedRisuSessionIdentityResolution{sessionID: strings.TrimSpace(best.ChatSessionID), resolution: "existing_host_chat_most_complete"}
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ChatSessionID) == requested {
			return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "existing_host_chat_requested_alias"}
		}
	}
	return observedRisuSessionIdentityResolution{sessionID: requested, resolution: "host_chat_id_ambiguous"}
}

func (s *Server) resolveDurableSessionRoutingBaseline(ctx context.Context, sessionID string, clientBaseline *routingTurnBaseline) *routingTurnBaseline {
	resolver, ok := s.Store.(store.SessionRoutingBaselineStore)
	if !ok || strings.TrimSpace(sessionID) == "" {
		return clientBaseline
	}
	durable, err := resolver.GetSessionRoutingBaseline(ctx, strings.TrimSpace(sessionID))
	if err != nil || durable == nil || durable.ImportedThroughTurn <= 0 {
		return clientBaseline
	}
	reason := "timeline_migrate"
	if durable.Mode == store.SessionMigrationModeCopyKeepSource {
		reason = "timeline_copy"
	}
	localPairsAtRoute := 0
	if clientBaseline != nil && routingBaselineReasonSupported(clientBaseline.Reason) {
		localPairsAtRoute = maxInt(0, clientBaseline.LocalPairsAtRoute)
	}
	return &routingTurnBaseline{
		BackendTurnAtRoute: durable.ImportedThroughTurn,
		LocalPairsAtRoute:  localPairsAtRoute,
		Reason:             reason,
		durableSourceID:    strings.TrimSpace(durable.SourceSessionID),
		durableVerified:    true,
	}
}

func calculateSessionRoutingTurnResolution(req sessionRoutingTurnResolutionRequest) sessionRoutingTurnResolutionResponse {
	if req.Mode == "batch" {
		resp := sessionRoutingTurnResolutionResponse{
			Status:          "ok",
			ContractVersion: routingTurnContractVersion,
			Resolution:      "batch",
			LocalTurnSource: "batch",
		}
		resp.ResolvedObservations = make([]routingTurnResolvedObservation, 0, len(req.Observations))
		for _, observation := range req.Observations {
			resolved := calculateSessionRoutingTurnResolution(sessionRoutingTurnResolutionRequest{
				Mode:                 "pair",
				RisuUserMessageIndex: observation.RisuUserMessageIndex,
				ObservedPairOrdinal:  observation.ObservedPairOrdinal,
				Baseline:             req.Baseline,
			})
			resp.ResolvedObservations = append(resp.ResolvedObservations, routingTurnResolvedObservation{
				ObservationIndex:          observation.ObservationIndex,
				RisuUserMessageIndex:      observation.RisuUserMessageIndex,
				RisuAssistantMessageIndex: observation.RisuAssistantMessageIndex,
				ObservedPairOrdinal:       observation.ObservedPairOrdinal,
				LocalTurnIndex:            resolved.LocalTurnIndex,
				TurnIndex:                 resolved.TurnIndex,
				Resolution:                resolved.Resolution,
				Source:                    resolved.LocalTurnSource,
			})
		}
		return resp
	}

	legacyTurn := req.LocalTurnIndex
	if req.Mode == "visible_completed" {
		legacyTurn = req.VisibleCompletedTurns
	}
	localTurn, localTurnSource := resolveObservedRisuLocalTurn(req.RisuUserMessageIndex, req.ObservedPairOrdinal, legacyTurn)
	resp := sessionRoutingTurnResolutionResponse{
		Status:          "ok",
		ContractVersion: routingTurnContractVersion,
		Resolution:      "normal",
		LocalTurnIndex:  localTurn,
		LocalTurnSource: localTurnSource,
	}
	if req.Mode == "visible_completed" {
		resp.CompletedTurns = localTurn
	} else {
		resp.TurnIndex = localTurn
	}
	baseline := req.Baseline
	if req.canonicalTailAligned {
		resp.Resolution = "canonical_tail_aligned"
		return resp
	}
	if baseline == nil || !routingBaselineReasonSupported(baseline.Reason) || baseline.BackendTurnAtRoute <= 0 {
		return resp
	}
	localBase, backendBase := maxInt(0, baseline.LocalPairsAtRoute), maxInt(0, baseline.BackendTurnAtRoute)
	resp.BaselineApplied, resp.ProtectedBeforeTurn, resp.MinFromTurn = true, backendBase, backendBase+1
	if req.Mode == "visible_completed" {
		resp.CompletedTurns = backendBase + maxInt(0, localTurn-localBase)
		if resp.CompletedTurns != localTurn {
			resp.Resolution = "rebased"
		}
		return resp
	}
	if localTurn <= 0 {
		resp.Resolution, resp.TurnIndex = "invalid", 0
		return resp
	}
	if localTurn <= localBase {
		resp.Resolution, resp.TurnIndex = "skip_pre_route_visible_pair", localTurn
		return resp
	}
	resp.TurnIndex = backendBase + (localTurn - localBase)
	if resp.TurnIndex != localTurn {
		resp.Resolution = "rebased"
	} else {
		resp.Resolution = "aligned"
	}
	return resp
}

func resolveObservedRisuLocalTurn(risuUserMessageIndex *int, observedPairOrdinal, legacyTurn int) (int, string) {
	if risuUserMessageIndex != nil && *risuUserMessageIndex >= 0 && *risuUserMessageIndex%2 == 0 {
		return (*risuUserMessageIndex / 2) + 1, "risu_user_message_index"
	}
	if observedPairOrdinal > 0 {
		return observedPairOrdinal, "observed_pair_ordinal"
	}
	return maxInt(0, legacyTurn), "legacy_local_turn_index"
}

func routingBaselineReasonSupported(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "timeline_copy", "timeline_migrate", "timeline_attach":
		return true
	}
	return false
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
