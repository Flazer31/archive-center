package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	rollbackDecisionContractVersion = "rollback.decision.v1"
	routingTurnContractVersion      = "session-routing.turn-resolution.v1"
	rollbackDecisionTTL             = 2 * time.Minute
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
	ExpiresAt           string `json:"expires_at,omitempty"`
}

type rollbackDecisionRecord struct {
	Token         string
	SessionID     string
	FromTurn      int
	RequestSource string
	ExpiresAt     time.Time
}

type rollbackDecisionLedger struct {
	mu      sync.Mutex
	records map[string]rollbackDecisionRecord
}

func newRollbackDecisionLedger() *rollbackDecisionLedger {
	return &rollbackDecisionLedger{records: map[string]rollbackDecisionRecord{}}
}

func (l *rollbackDecisionLedger) issue(sessionID string, fromTurn int, requestSource string) rollbackDecisionRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UTC()
	for token, record := range l.records {
		if now.After(record.ExpiresAt) {
			delete(l.records, token)
		}
	}
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		bytes = []byte(now.Format("20060102150405.000000000"))
	}
	token := hex.EncodeToString(bytes)
	record := rollbackDecisionRecord{Token: token, SessionID: sessionID, FromTurn: fromTurn, RequestSource: requestSource, ExpiresAt: now.Add(rollbackDecisionTTL)}
	l.records[token] = record
	return record
}

func (l *rollbackDecisionLedger) consume(token, sessionID string, fromTurn int) (rollbackDecisionRecord, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	record, ok := l.records[token]
	if !ok {
		return rollbackDecisionRecord{}, false
	}
	delete(l.records, token)
	if time.Now().UTC().After(record.ExpiresAt) || record.SessionID != sessionID || record.FromTurn != fromTurn {
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
	if req.IncompleteTailCandidate &&
		req.DeletionObserved &&
		req.RemovedAssistantCount == 0 &&
		req.RemovedUserCount == 1 &&
		req.RemovedMessageCount == 1 &&
		req.BackendLatestTurn > 0 &&
		req.CandidateFromTurn == req.BackendLatestTurn &&
		s.Store != nil {
		logs, err := s.Store.ListChatLogs(r.Context(), strings.TrimSpace(req.ChatSessionID), 0, 0)
		if err == nil && len(logs) > 0 {
			actualLatestTurn := 0
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
		record := s.rollbackDecisionLedger().issue(resp.ChatSessionID, resp.FromTurn, req.RequestSource)
		resp.DecisionToken = record.Token
		resp.ExpiresAt = record.ExpiresAt.Format(time.RFC3339Nano)
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
	ChatSessionID         string                   `json:"chat_session_id"`
	Mode                  string                   `json:"mode"`
	HostChatID            string                   `json:"host_chat_id,omitempty"`
	HostChatIDState       string                   `json:"host_chat_id_state,omitempty"`
	LatestUserHash        string                   `json:"latest_user_hash,omitempty"`
	LatestAssistantHash   string                   `json:"latest_assistant_hash,omitempty"`
	LocalTurnIndex        int                      `json:"local_turn_index"`
	VisibleCompletedTurns int                      `json:"visible_completed_turns"`
	RisuUserMessageIndex  *int                     `json:"risu_user_message_index,omitempty"`
	ObservedPairOrdinal   int                      `json:"observed_pair_ordinal,omitempty"`
	Observations          []routingTurnObservation `json:"observations,omitempty"`
	Baseline              *routingTurnBaseline     `json:"baseline,omitempty"`
	canonicalTailAligned  bool
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
	Status               string                           `json:"status"`
	ContractVersion      string                           `json:"contract_version"`
	ChatSessionID        string                           `json:"chat_session_id,omitempty"`
	IdentityResolution   string                           `json:"identity_resolution,omitempty"`
	Resolution           string                           `json:"resolution"`
	TurnIndex            int                              `json:"turn_index"`
	CompletedTurns       int                              `json:"completed_turns"`
	LocalTurnIndex       int                              `json:"local_turn_index"`
	LocalTurnSource      string                           `json:"local_turn_source"`
	ProtectedBeforeTurn  int                              `json:"protected_before_turn"`
	MinFromTurn          int                              `json:"min_from_turn"`
	BaselineApplied      bool                             `json:"baseline_applied"`
	ResolvedObservations []routingTurnResolvedObservation `json:"resolved_observations,omitempty"`
}

func (s *Server) handleSessionRoutingTurnResolution(w http.ResponseWriter, r *http.Request) {
	var req sessionRoutingTurnResolutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_session_routing_observation"})
		return
	}
	canonicalSessionID, identityResolution, canonicalTailAligned := s.resolveObservedRisuSessionIdentity(r.Context(), req)
	if canonicalSessionID != "" {
		req.ChatSessionID = canonicalSessionID
	}
	req.canonicalTailAligned = canonicalTailAligned
	req.Baseline = s.resolveDurableSessionRoutingBaseline(r.Context(), req.ChatSessionID, req.Baseline)
	resp := calculateSessionRoutingTurnResolution(req)
	resp.ChatSessionID = strings.TrimSpace(req.ChatSessionID)
	resp.IdentityResolution = identityResolution
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) resolveObservedRisuSessionIdentity(ctx context.Context, req sessionRoutingTurnResolutionRequest) (string, string, bool) {
	requested := strings.TrimSpace(req.ChatSessionID)
	hostChatID := strings.TrimSpace(req.HostChatID)
	if req.HostChatIDState != "observed" || hostChatID == "" || s.Store == nil {
		return requested, "host_chat_id_unobserved", false
	}
	sessions, err := s.Store.ListSessions(ctx)
	if err != nil {
		return requested, "session_list_unavailable", false
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
		return requested, "new_host_chat_id", false
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
			return strings.TrimSpace(matched[0].ChatSessionID), "existing_host_chat_tail_match", true
		}
		if len(matched) > 1 {
			candidates = matched
		} else if len(candidates) == 1 {
			// A matching CID alone is not enough to reconnect an index alias.
			// Copy/cold-start damage can leave that CID attached to a backend
			// tail that is not the chat RisuAI is currently showing.
			if turnMismatchObserved {
				return requested, "existing_host_chat_turn_mismatch", false
			}
			return requested, "existing_host_chat_tail_mismatch", false
		}
	}
	if len(candidates) == 1 {
		return strings.TrimSpace(candidates[0].ChatSessionID), "existing_host_chat_id", false
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
		return strings.TrimSpace(best.ChatSessionID), "existing_host_chat_most_complete", false
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ChatSessionID) == requested {
			return requested, "existing_host_chat_requested_alias", false
		}
	}
	return requested, "host_chat_id_ambiguous", false
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
