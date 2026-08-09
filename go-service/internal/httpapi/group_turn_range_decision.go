package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	rollbackDecisionContractVersion = "rollback.decision.v1"
	routingTurnContractVersion      = "session-routing.turn-resolution.v1"
	rollbackDecisionMax             = 1024
)

type routingTurnBaseline struct {
	BackendTurnAtRoute int    `json:"backend_turn_at_route"`
	LocalPairsAtRoute  int    `json:"local_pairs_at_route"`
	Reason             string `json:"reason"`
}

type rollbackDecisionRequest struct {
	ChatSessionID                 string               `json:"chat_session_id"`
	RequestSource                 string               `json:"request_source"`
	Reason                        string               `json:"reason"`
	CandidateFromTurn             int                  `json:"candidate_from_turn"`
	PreviousTurnIndex             int                  `json:"previous_turn_index"`
	FirstRemovedTurn              int                  `json:"first_removed_turn"`
	LedgerAnchorTurn              int                  `json:"ledger_anchor_turn"`
	RemovedAssistantCount         int                  `json:"removed_assistant_count"`
	RemovedUserCount              int                  `json:"removed_user_count"`
	RemovedMessageCount           int                  `json:"removed_message_count"`
	VisibleCompletedTurns         int                  `json:"visible_completed_turns"`
	BackendLatestTurn             int                  `json:"backend_latest_turn"`
	DeletionObserved              bool                 `json:"deletion_observed"`
	LedgerVerified                bool                 `json:"ledger_verified"`
	IncompleteTailCandidate       bool                 `json:"incomplete_tail_candidate"`
	BackendIncompleteTailVerified bool                 `json:"-"`
	HistoryTrimGuard              bool                 `json:"history_trim_guard"`
	DuplicateBlocked              bool                 `json:"duplicate_blocked"`
	PendingOutputGuard            bool                 `json:"pending_output_guard"`
	HostLifecycleObservation      string               `json:"host_lifecycle_observation"`
	LifecycleActionObservation    string               `json:"lifecycle_action_observation"`
	AllowManualCandidate          bool                 `json:"allow_manual_candidate"`
	Baseline                      *routingTurnBaseline `json:"baseline,omitempty"`
}

type rollbackDecisionResponse struct {
	Status              string `json:"status"`
	ContractVersion     string `json:"contract_version"`
	Allowed             bool   `json:"allowed"`
	Decision            string `json:"decision"`
	Reason              string `json:"reason"`
	ChatSessionID       string `json:"chat_session_id"`
	RequestedFromTurn   int    `json:"requested_from_turn"`
	FromTurn            int    `json:"from_turn"`
	ProtectedBeforeTurn int    `json:"protected_before_turn"`
	MinFromTurn         int    `json:"min_from_turn"`
	EffectiveCompleted  int    `json:"effective_completed_turns"`
	BaselineApplied     bool   `json:"baseline_applied"`
	DecisionToken       string `json:"decision_token,omitempty"`
	LifecycleAction     string `json:"lifecycle_action"`
	TurnWorkflowHUD     any    `json:"turn_workflow_hud,omitempty"`
}

type rollbackDecisionRecord struct {
	Token           string
	SessionID       string
	FromTurn        int
	RequestSource   string
	LifecycleAction string
	Sequence        uint64
}

type rollbackDecisionLedger struct {
	mu           sync.Mutex
	records      map[string]rollbackDecisionRecord
	nextSequence uint64
}

func newRollbackDecisionLedger() *rollbackDecisionLedger {
	return &rollbackDecisionLedger{records: map[string]rollbackDecisionRecord{}}
}

func (l *rollbackDecisionLedger) issue(sessionID string, fromTurn int, requestSource, lifecycleAction string) rollbackDecisionRecord {
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
	record := rollbackDecisionRecord{Token: token, SessionID: sessionID, FromTurn: fromTurn, RequestSource: requestSource, LifecycleAction: lifecycleAction, Sequence: l.nextSequence}
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

func (l *rollbackDecisionLedger) consume(token, sessionID string, fromTurn int) (rollbackDecisionRecord, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	record, ok := l.records[token]
	if !ok {
		return rollbackDecisionRecord{}, false
	}
	delete(l.records, token)
	if record.SessionID != sessionID || record.FromTurn != fromTurn {
		return rollbackDecisionRecord{}, false
	}
	return record, true
}

func (s *Server) rollbackDecisionLedger() *rollbackDecisionLedger {
	if s.RollbackDecisions == nil {
		s.RollbackDecisions = newRollbackDecisionLedger()
	}
	return s.RollbackDecisions
}

func (s *Server) handleRollbackDecision(w http.ResponseWriter, r *http.Request) {
	var req rollbackDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_rollback_observation"})
		return
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
	if resp.Allowed {
		record := s.rollbackDecisionLedger().issue(resp.ChatSessionID, resp.FromTurn, req.RequestSource, resp.LifecycleAction)
		resp.DecisionToken = record.Token
		requestSource := strings.TrimSpace(req.RequestSource)
		if requestSource == "" {
			requestSource = "auto"
		}
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
	if req.PendingOutputGuard || rollbackObservationHasPendingGeneration(req.HostLifecycleObservation) {
		resp.Reason = "pending_output_guard"
		return resp
	}
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
	manual := strings.EqualFold(strings.TrimSpace(req.RequestSource), "manual")
	if !req.DeletionObserved && !(manual && req.AllowManualCandidate) {
		return resp
	}
	if req.IncompleteTailCandidate && !req.BackendIncompleteTailVerified {
		resp.Reason = "incomplete_tail_not_verified"
		return resp
	}

	effectiveCompleted := maxInt(0, req.VisibleCompletedTurns)
	protectedBefore, minFrom := 0, 0
	baselineApplied := false
	if baseline := req.Baseline; baseline != nil && routingBaselineReasonSupported(baseline.Reason) && baseline.BackendTurnAtRoute > 0 {
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

func rollbackObservationHasPendingGeneration(observation string) bool {
	switch strings.ToLower(strings.TrimSpace(observation)) {
	case "before_request_observed", "generation_watch_active":
		return true
	default:
		return false
	}
}

type sessionRoutingTurnResolutionRequest struct {
	ChatSessionID          string                   `json:"chat_session_id"`
	Mode                   string                   `json:"mode"`
	StableCharacterID      string                   `json:"stable_character_id,omitempty"`
	StableCharacterIDState string                   `json:"stable_character_id_state,omitempty"`
	HostChatID             string                   `json:"host_chat_id,omitempty"`
	HostChatIDState        string                   `json:"host_chat_id_state,omitempty"`
	BindRequestedSession   bool                     `json:"bind_requested_session,omitempty"`
	BindingMode            string                   `json:"binding_mode,omitempty"`
	LatestUserHash         string                   `json:"latest_user_hash,omitempty"`
	LatestAssistantHash    string                   `json:"latest_assistant_hash,omitempty"`
	LocalTurnIndex         int                      `json:"local_turn_index"`
	VisibleCompletedTurns  int                      `json:"visible_completed_turns"`
	RisuUserMessageIndex   *int                     `json:"risu_user_message_index,omitempty"`
	ObservedPairOrdinal    int                      `json:"observed_pair_ordinal,omitempty"`
	Observations           []routingTurnObservation `json:"observations,omitempty"`
	Baseline               *routingTurnBaseline     `json:"baseline,omitempty"`
	canonicalTailAligned   bool
}

type routingTurnObservation struct {
	ObservationIndex     int  `json:"observation_index"`
	RisuUserMessageIndex *int `json:"risu_user_message_index,omitempty"`
	ObservedPairOrdinal  int  `json:"observed_pair_ordinal,omitempty"`
}

type routingTurnResolvedObservation struct {
	ObservationIndex     int    `json:"observation_index"`
	RisuUserMessageIndex *int   `json:"risu_user_message_index,omitempty"`
	ObservedPairOrdinal  int    `json:"observed_pair_ordinal"`
	LocalTurnIndex       int    `json:"local_turn_index"`
	TurnIndex            int    `json:"turn_index"`
	Resolution           string `json:"resolution"`
	Source               string `json:"source"`
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
				ObservationIndex:     observation.ObservationIndex,
				RisuUserMessageIndex: observation.RisuUserMessageIndex,
				ObservedPairOrdinal:  observation.ObservedPairOrdinal,
				LocalTurnIndex:       resolved.LocalTurnIndex,
				TurnIndex:            resolved.TurnIndex,
				Resolution:           resolved.Resolution,
				Source:               resolved.LocalTurnSource,
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
