package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const dashboardViewModelContractVersion = "dashboard.viewmodel.v2"

type dashboardViewModelRequest struct {
	RuntimeState             map[string]any              `json:"runtime_state"`
	PluginEnabled            bool                        `json:"plugin_enabled"`
	CurrentSessionID         string                      `json:"current_session_id"`
	SessionCandidates        map[string]any              `json:"session_candidates"`
	PrepareTurnEverContacted bool                        `json:"prepare_turn_ever_contacted"`
	FailedQueueDepth         int                         `json:"failed_queue_depth"`
	CurrentWorkflowRequestID string                      `json:"current_workflow_request_id,omitempty"`
	QueueObservations        []dashboardQueueObservation `json:"queue_observations,omitempty"`
	GuideModeState           map[string]any              `json:"guide_mode_state"`
	FirstTurnLight           bool                        `json:"first_turn_light"`
	FirstTurnEndedAt         string                      `json:"first_turn_ended_at"`
	ReferenceCard            *dashboardCard              `json:"-"`
	WorkflowSnapshot         *turnWorkflowHUDViewModel   `json:"-"`
}

type dashboardQueueObservation struct {
	QueueKind   string `json:"queue_kind"`
	SessionID   string `json:"session_id,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	TurnIndex   int    `json:"turn_index,omitempty"`
	State       string `json:"state,omitempty"`
	ReasonCode  string `json:"reason_code,omitempty"`
	TerminalAt  string `json:"terminal_at,omitempty"`
	Attempts    int    `json:"attempts,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
	Count       int    `json:"count,omitempty"`
}

type dashboardViewModel struct {
	ContractVersion string          `json:"contract_version"`
	Status          string          `json:"status"`
	Summary         dashboardCounts `json:"summary"`
	Cards           []dashboardCard `json:"cards"`
}

type dashboardCounts struct {
	OK      int `json:"ok"`
	Neutral int `json:"neutral"`
	Notice  int `json:"notice"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Unknown int `json:"unknown"`
}

type dashboardCard struct {
	ID       string          `json:"id"`
	Icon     string          `json:"icon"`
	Title    string          `json:"title"`
	Severity string          `json:"severity"`
	Summary  dashboardCounts `json:"summary"`
	Rows     []dashboardRow  `json:"rows,omitempty"`
	Chips    []dashboardChip `json:"chips,omitempty"`
}

type dashboardRow struct {
	LabelKey   string `json:"label_key"`
	Status     string `json:"status"`
	DetailCode string `json:"detail_code,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Time       string `json:"time,omitempty"`
	TurnIndex  any    `json:"turn_index,omitempty"`
	ItemCount  any    `json:"item_count,omitempty"`
	Placement  any    `json:"placement,omitempty"`
	Scope      string `json:"scope,omitempty"`
	QueueKind  string `json:"queue_kind,omitempty"`
}

type dashboardChip struct {
	Tone  string `json:"tone"`
	Label string `json:"label"`
}

func (s *Server) registerDashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /dashboard/view-model", s.handleDashboardViewModel)
}

func (s *Server) handleDashboardViewModel(w http.ResponseWriter, r *http.Request) {
	var req dashboardViewModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_dashboard_snapshot"})
		return
	}
	req.ReferenceCard = s.buildReferenceDashboardCard(r.Context(), resolveDashboardSessionID(req))
	if s != nil && s.TurnWorkflows != nil {
		sessionID := resolveDashboardSessionID(req)
		requestID := strings.TrimSpace(req.CurrentWorkflowRequestID)
		if requestID != "" {
			if snapshot, ok := s.TurnWorkflows.snapshot(requestID); ok && snapshot.ChatSessionID == sessionID {
				req.WorkflowSnapshot = &snapshot
			}
		} else if snapshot, ok := s.TurnWorkflows.latestSnapshotForSession(sessionID); ok {
			req.WorkflowSnapshot = &snapshot
		}
	}
	writeJSON(w, http.StatusOK, buildDashboardViewModel(req))
}

func (s *Server) buildReferenceDashboardCard(ctx context.Context, sessionID string) *dashboardCard {
	if s == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok {
		return nil
	}
	bindings, err := ref.ListSessionReferenceBindings(ctx, sessionID, false)
	if len(bindings) == 0 {
		return nil
	}
	row := dashboardRow{LabelKey: "referenceVector", ItemCount: len(bindings)}
	fail := func(code string) *dashboardCard {
		row.Status = "fail"
		row.DetailCode = code
		card := newDashboardCard("reference", "REF", "Original Work Reference", []dashboardRow{row})
		return &card
	}
	if err != nil {
		return fail("referenceBindingReadFailed")
	}
	if s.ReferenceVectorOpenError != nil {
		return fail("referenceVectorOpenFailed")
	}
	if s.ReferenceVector == nil {
		return fail("referenceVectorUnavailable")
	}
	health, healthErr := s.ReferenceVector.Health(ctx)
	if healthErr != nil {
		return fail("referenceVectorHealthFailed")
	}
	if strings.TrimSpace(health.Status) != "ok" || !health.ModelReady {
		return fail("referenceVectorHealthNotReady")
	}
	if _, ok := s.ReferenceVector.(vector.ExactMetadataQuerier); !ok {
		return fail("referenceVectorExactQueryUnavailable")
	}
	if _, ok := s.ReferenceVector.(vector.DocumentLister); !ok {
		return fail("referenceVectorListingUnavailable")
	}
	if !s.completeTurnExtractionConfig(nil).Embedder.hasConfig() {
		return fail("referenceEmbeddingConfigMissing")
	}
	row.Status = "ok"
	row.DetailCode = "referenceReady"
	card := newDashboardCard("reference", "REF", "Original Work Reference", []dashboardRow{row})
	return &card
}

func buildDashboardViewModel(req dashboardViewModelRequest) dashboardViewModel {
	runtime := req.RuntimeState
	state := func(key string) map[string]any { return dashboardMap(runtime[key]) }
	firstTurnState := func(input map[string]any) map[string]any {
		if !req.FirstTurnLight {
			return input
		}
		status := dashboardString(input["status"])
		if status != "" && status != "unknown" && status != "off" {
			return input
		}
		out := dashboardCloneMap(input)
		out["status"] = "skipped"
		out["reason_code"] = "first_turn_light"
		out["detail"] = "first turn light mode"
		if dashboardString(out["time"]) == "" {
			out["time"] = req.FirstTurnEndedAt
		}
		return out
	}

	sessionID := resolveDashboardSessionID(req)
	sessionState := map[string]any{"status": "unknown", "detail": "resolving..."}
	if sessionID != "" {
		sessionState = map[string]any{"status": "ok", "detail": shortenDashboardSessionID(sessionID)}
	}

	supervisorHealth := state("lastSupervisorWakeup")
	if dashboardStatus(supervisorHealth) == "unknown" && dashboardStatus(state("lastSupervisorStatus")) == "ok" {
		supervisorHealth = map[string]any{
			"status":      "skipped",
			"reason_code": "health_test_not_run_turn_call_ok",
			"time":        state("lastSupervisorStatus")["time"],
			"detail":      "health test not run / turn call ok",
		}
	}

	cards := []dashboardCard{
		newDashboardCard("connection", "🔌", "Connection", []dashboardRow{
			dashboardRowFromState("plugin", map[string]any{"status": dashboardBoolStatus(req.PluginEnabled), "detail": dashboardEnabledDetail(req.PluginEnabled)}),
			dashboardRowFromState("sessionId", sessionState),
			dashboardRowFromState("bridgeHealth", state("lastBridgeHealth")),
			dashboardRowFromState("supervisorHealthTest", supervisorHealth),
			dashboardRowFromState("search", firstTurnState(state("lastSearchStatus"))),
			dashboardRowFromState("supervisorCall", state("lastSupervisorStatus")),
		}),
		newDashboardCard("engine", "⚙️", "Engine", []dashboardRow{
			dashboardRowFromState("turnEngine", firstTurnState(state("prepareTurnStatus"))),
			dashboardRowFromState("guideMode", req.GuideModeState),
			dashboardRowFromState("runtimeSync", map[string]any{
				"status":      dashboardBoolUnknownStatus(req.PrepareTurnEverContacted),
				"reason_code": dashboardSyncReasonCode(req.PrepareTurnEverContacted),
				"detail":      dashboardSyncDetail(req.PrepareTurnEverContacted),
			}),
		}),
	}
	if req.WorkflowSnapshot != nil {
		cards = append(cards, buildCurrentWorkflowDashboardCard(*req.WorkflowSnapshot))
	}
	if critic := buildCriticLedgerDashboardCard(state("lastCriticLedgerProbe")); critic != nil {
		cards = append(cards, *critic)
	}
	if req.ReferenceCard != nil {
		cards = append(cards, *req.ReferenceCard)
	}
	queue := dashboardMap(runtime["queuePersistence"])
	load := dashboardMap(queue["lastLoad"])
	save := dashboardMap(queue["lastSave"])
	queueDetail := []string{}
	if len(load) > 0 {
		queueDetail = append(queueDetail, "load:"+dashboardStatus(load))
	}
	if len(save) > 0 {
		queueDetail = append(queueDetail, "save:"+dashboardStatus(save))
	}
	queueStorageReasonCode := ""
	if dashboardStatus(load) == "ok" && dashboardStatus(save) == "ok" {
		queueStorageReasonCode = "queue_storage_ok"
	}
	cards = append(cards, newDashboardCard("save_queue", "💾", "Save / Queue", []dashboardRow{
		dashboardRowFromState("injection", firstTurnState(state("lastInjectionStatus"))),
		dashboardRowFromState("save", state("lastSaveStatus")),
		dashboardRowFromState("complete", state("lastCompleteStatus")),
		dashboardRowFromState("queueStorage", map[string]any{
			"status":      dashboardStatus(save),
			"reason_code": queueStorageReasonCode,
			"detail":      dashboardFirstNonEmpty(strings.Join(queueDetail, " / "), "not yet"),
		}),
	}))

	currentQueue, historicalQueue := buildDashboardQueueCards(req, sessionID)
	if currentQueue != nil {
		cards = append(cards, *currentQueue)
	}
	if historicalQueue != nil {
		cards = append(cards, *historicalQueue)
	}

	complete := state("lastCompleteTurnStatus")
	if dashboardStatus(complete) != "idle" {
		cards = append(cards, buildCompleteTurnDashboardCard(complete))
		if rows := buildPersistenceDashboardRows(complete); len(rows) > 0 {
			cards = append(cards, newDashboardCard("persistence_lanes", "📦", "Persistence Lanes", rows))
		}
	}
	if rows := buildDashboardTimingRows(state("prepareTurnStatus"), complete); len(rows) > 0 {
		cards = append(cards, newDashboardCard("backend_timing", "⏱", "Backend Timing", rows))
	}

	activityKeys := []struct{ key, label string }{
		{"lastAutoRollback", "autoRollback"}, {"lastStreamingAfterRequest", "streamingHook"},
		{"sessionWriteRouting", "sessionRouting"}, {"lastRisuForkCopyCapture", "forkCopyCapture"},
		{"lastSessionDeleteSync", "sessionDeleteSync"}, {"lastActiveChatBackfill", "activeChatBackfill"},
		{"lastRerollReplacement", "rerollReplacement"},
	}
	activityRows := []dashboardRow{}
	for _, item := range activityKeys {
		if current := state(item.key); dashboardStatus(current) != "idle" {
			activityRows = append(activityRows, dashboardRowFromState(item.label, current))
		}
	}
	if len(activityRows) > 0 {
		cards = append(cards, newDashboardCard("activity", "🔄", "Activity", activityRows))
	}
	if lastError := state("lastError"); len(lastError) > 0 {
		cards = append(cards, newDashboardCard("last_error", "⚠️", "Last Error", []dashboardRow{dashboardRowFromState("lastError", lastError)}))
	}

	summary := dashboardCounts{}
	for _, card := range cards {
		summary.OK += card.Summary.OK
		summary.Neutral += card.Summary.Neutral
		summary.Notice += card.Summary.Notice
		summary.Warn += card.Summary.Warn
		summary.Fail += card.Summary.Fail
		summary.Unknown += card.Summary.Unknown
	}
	return dashboardViewModel{ContractVersion: dashboardViewModelContractVersion, Status: "ok", Summary: summary, Cards: cards}
}

func buildCurrentWorkflowDashboardCard(view turnWorkflowHUDViewModel) dashboardCard {
	rows := make([]dashboardRow, 0, len(view.Facts)+1)
	alignmentStatus := "neutral"
	switch strings.TrimSpace(view.TurnAlignment.State) {
	case "aligned":
		alignmentStatus = "ok"
	case "host_ahead", "backend_ahead":
		alignmentStatus = "warn"
	}
	rows = append(rows, dashboardRow{
		LabelKey:   "turnAlignment",
		Status:     alignmentStatus,
		DetailCode: strings.TrimSpace(view.TurnAlignment.ReasonCode),
		Detail:     fmt.Sprintf("host:%d / backend:%d / %s", view.TurnAlignment.HostTurn, view.TurnAlignment.BackendTurn, dashboardFirstNonEmpty(view.TurnAlignment.State, "unobserved")),
		TurnIndex:  view.BackendTurn,
		Scope:      "current_request",
	})
	for _, fact := range view.Facts {
		detail := strings.TrimSpace(fact.Detail)
		if detail == "" {
			detail = strings.Join(nonEmptyDashboardParts(fact.Disposition, fact.Status, fact.ReasonCode), " / ")
		}
		rows = append(rows, dashboardRow{
			LabelKey:   "workflowFact." + strings.TrimSpace(fact.Key),
			Status:     dashboardStatusFromWorkflowFact(fact),
			DetailCode: strings.TrimSpace(fact.ReasonCode),
			Detail:     detail,
			TurnIndex:  view.BackendTurn,
			ItemCount:  fact.Count,
			Scope:      strings.TrimSpace(fact.Scope),
		})
	}
	return newDashboardCard("current_workflow", "HUD", "Current Turn Workflow", rows)
}

func buildDashboardQueueCards(req dashboardViewModelRequest, currentSessionID string) (*dashboardCard, *dashboardCard) {
	currentRows := []dashboardRow{}
	type historicalQueueKey struct {
		Scope      string
		Kind       string
		State      string
		ReasonCode string
	}
	historicalCounts := map[historicalQueueKey]int{}
	historicalLatestTimes := map[historicalQueueKey]string{}
	observedTransportCount := 0
	for _, observation := range req.QueueObservations {
		kind := normalizeDashboardQueueKind(observation.QueueKind)
		if kind == "" {
			continue
		}
		if kind == "transport_retry" {
			observedTransportCount += maxInt(1, observation.Count)
		}
		scope := classifyDashboardQueueScope(observation, currentSessionID, req.CurrentWorkflowRequestID, req.WorkflowSnapshot)
		if scope == "current_request" {
			currentRows = append(currentRows, dashboardRow{
				LabelKey:   "queue." + kind,
				Status:     dashboardCurrentQueueStatus(kind, observation.State, observation.Attempts, observation.MaxAttempts),
				DetailCode: dashboardFirstNonEmpty(strings.TrimSpace(observation.ReasonCode), strings.TrimSpace(observation.State), "pending"),
				Detail:     dashboardQueueAttemptDetail(observation),
				Time:       strings.TrimSpace(observation.TerminalAt),
				TurnIndex:  observation.TurnIndex,
				ItemCount:  maxInt(1, observation.Count),
				Scope:      scope,
				QueueKind:  kind,
			})
			continue
		}
		state := strings.ToLower(strings.TrimSpace(observation.State))
		if state == "" {
			state = "unknown"
		}
		key := historicalQueueKey{
			Scope:      scope,
			Kind:       kind,
			State:      state,
			ReasonCode: strings.TrimSpace(observation.ReasonCode),
		}
		historicalCounts[key] += maxInt(1, observation.Count)
		terminalAt := strings.TrimSpace(observation.TerminalAt)
		if terminalAt != "" && terminalAt > historicalLatestTimes[key] {
			historicalLatestTimes[key] = terminalAt
		}
	}
	if req.FailedQueueDepth > observedTransportCount {
		historicalCounts[historicalQueueKey{Scope: "unknown", Kind: "transport_retry", State: "unknown"}] += req.FailedQueueDepth - observedTransportCount
	}
	var currentCard *dashboardCard
	if len(currentRows) > 0 {
		card := newDashboardCard("current_queue", "NOW", "Current Turn Queue", currentRows)
		currentCard = &card
	}
	var historicalCard *dashboardCard
	if len(historicalCounts) > 0 {
		keys := make([]historicalQueueKey, 0, len(historicalCounts))
		for key := range historicalCounts {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			left := strings.Join([]string{keys[i].Scope, keys[i].Kind, keys[i].State, keys[i].ReasonCode}, "\x00")
			right := strings.Join([]string{keys[j].Scope, keys[j].Kind, keys[j].State, keys[j].ReasonCode}, "\x00")
			return left < right
		})
		rows := make([]dashboardRow, 0, len(keys))
		for _, key := range keys {
			status := "notice"
			detailCode := "historical_queue_not_current_turn"
			if key.State == "terminal" || key.State == "failed" {
				status = "fail"
				detailCode = "historical_queue_terminal"
			} else if key.State == "retryable" {
				detailCode = "historical_queue_retryable"
			}
			detailCode = dashboardFirstNonEmpty(key.ReasonCode, detailCode)
			rows = append(rows, dashboardRow{
				LabelKey:   "queueHistory." + key.Kind,
				Status:     status,
				DetailCode: detailCode,
				Detail:     strconv.Itoa(historicalCounts[key]) + " " + key.State,
				Time:       historicalLatestTimes[key],
				ItemCount:  historicalCounts[key],
				Scope:      key.Scope,
				QueueKind:  key.Kind,
			})
		}
		card := newDashboardCard("historical_queue", "HIS", "Historical Queue", rows)
		historicalCard = &card
	}
	return currentCard, historicalCard
}

func classifyDashboardQueueScope(observation dashboardQueueObservation, currentSessionID, currentRequestID string, workflow *turnWorkflowHUDViewModel) string {
	observationRequestID := strings.TrimSpace(observation.RequestID)
	activeRequestID := strings.TrimSpace(currentRequestID)
	if activeRequestID == "" && workflow != nil {
		activeRequestID = strings.TrimSpace(workflow.RequestID)
	}
	if observationRequestID != "" && activeRequestID != "" {
		if observationRequestID == activeRequestID {
			return "current_request"
		}
		return classifyDashboardHistoricalQueueScope(observation.SessionID, currentSessionID)
	}
	if normalizeDashboardQueueKind(observation.QueueKind) == "maintenance" && observationRequestID == "" {
		return classifyDashboardHistoricalQueueScope(observation.SessionID, currentSessionID)
	}
	if workflow != nil {
		if strings.TrimSpace(observation.SessionID) == strings.TrimSpace(workflow.ChatSessionID) &&
			observation.TurnIndex > 0 && observation.TurnIndex == workflow.BackendTurn {
			return "current_request"
		}
	}
	return classifyDashboardHistoricalQueueScope(observation.SessionID, currentSessionID)
}

func classifyDashboardHistoricalQueueScope(observationSessionID, currentSessionID string) string {
	observationSessionID = strings.TrimSpace(observationSessionID)
	currentSessionID = strings.TrimSpace(currentSessionID)
	if observationSessionID == "" {
		return "unknown"
	}
	if observationSessionID == currentSessionID {
		return "current_session_history"
	}
	return "other_session"
}

func normalizeDashboardQueueKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "transport_retry", "pending_confirmation", "pending_confirmation_recovery", "maintenance":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func dashboardCurrentQueueStatus(kind, state string, attempts, maxAttempts int) string {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "terminal" || state == "failed" || (maxAttempts > 0 && attempts >= maxAttempts) {
		return "fail"
	}
	if kind == "transport_retry" {
		return "warn"
	}
	return "notice"
}

func dashboardQueueAttemptDetail(observation dashboardQueueObservation) string {
	state := dashboardFirstNonEmpty(strings.TrimSpace(observation.State), "pending")
	if observation.Attempts <= 0 && observation.MaxAttempts <= 0 {
		return state
	}
	return fmt.Sprintf("%s / attempts:%d/%d", state, maxInt(0, observation.Attempts), maxInt(0, observation.MaxAttempts))
}

func dashboardStatusFromWorkflowFact(fact turnWorkflowHUDFact) string {
	switch normalizeTurnWorkflowHUDSeverity(fact.Severity) {
	case turnWorkflowHUDSeverityError:
		return "fail"
	case turnWorkflowHUDSeverityWarning:
		return "warn"
	case turnWorkflowHUDSeverityNotice:
		return "notice"
	}
	if strings.TrimSpace(fact.Disposition) == "deferred" {
		return "notice"
	}
	if strings.TrimSpace(fact.Disposition) == "dropped" ||
		strings.TrimSpace(fact.Status) == "pending" ||
		strings.TrimSpace(fact.Status) == "unobserved" {
		return "neutral"
	}
	return "ok"
}

func nonEmptyDashboardParts(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func resolveDashboardSessionID(req dashboardViewModelRequest) string {
	sessionID := strings.TrimSpace(req.CurrentSessionID)
	if sessionID == "" {
		sessionID = dashboardFirstNonEmpty(
			dashboardString(req.SessionCandidates["runtime_current"]),
			dashboardString(req.SessionCandidates["timeline_current"]),
		)
	}
	if sessionID == "" {
		routing := dashboardMap(req.RuntimeState["sessionWriteRouting"])
		sessionID = dashboardFirstNonEmpty(dashboardString(routing["targetSessionId"]), dashboardString(routing["rawSessionId"]))
	}
	return sessionID
}

func buildCriticLedgerDashboardCard(probe map[string]any) *dashboardCard {
	if len(probe) == 0 {
		probe = map[string]any{"status": "idle"}
	}
	dash := dashboardMap(probe["dashboard"])
	rows := []dashboardRow{dashboardRowFromState("criticProbe", probe)}
	if len(dash) > 0 {
		itemCount := dashboardInt(dash["item_count"])
		missing := dashboardStrings(dash["missing_lanes"])
		rows = append(rows, dashboardRowFromState("criticItems", map[string]any{
			"status":    dashboardWarnIf(len(missing) > 0),
			"detail":    fmt.Sprintf("items:%d / missing:%d", itemCount, len(missing)),
			"itemCount": itemCount,
		}))
		if lanes := dashboardMap(dash["lane_counts"]); len(lanes) > 0 {
			keys := make([]string, 0, len(lanes))
			for key := range lanes {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, key := range keys {
				parts = append(parts, key+":"+dashboardNumberString(lanes[key]))
			}
			rows = append(rows, dashboardRowFromState("criticLanes", map[string]any{"status": "ok", "detail": strings.Join(parts, " / ")}))
		}
		if safety := dashboardMap(dash["safety"]); len(safety) > 0 {
			rows = append(rows, dashboardRowFromState("criticSafety", map[string]any{
				"status": dashboardWarnIf(dashboardInt(safety["scrubbed_items"]) > 0),
				"detail": fmt.Sprintf("scrubbed:%d / streaming:%s", dashboardInt(safety["scrubbed_items"]), dashboardFirstNonEmpty(dashboardString(safety["streaming_mismatch"]), "none")),
			}))
		}
	}
	card := newDashboardCard("critic_ledger", "📋", "Critic Ledger", rows)
	return &card
}

func buildCompleteTurnDashboardCard(complete map[string]any) dashboardCard {
	chips := []dashboardChip{}
	for _, item := range []struct{ key, label string }{
		{"chatLogsSaved", "log"}, {"memoriesSaved", "mem"}, {"evidenceSaved", "evi"},
		{"kgTriplesSaved", "kg"}, {"subjectiveEntityMemoriesSaved", "sem"}, {"worldRulesSaved", "rule"},
		{"derivedArtifactsSaved", "der"}, {"vectorUpserted", "vec"},
	} {
		if value, ok := complete[item.key]; ok && value != nil {
			chips = append(chips, dashboardChip{Tone: "num", Label: item.label + ":" + dashboardNumberString(value)})
		}
	}
	for _, item := range []struct{ key, label string }{{"rawStatus", "raw"}, {"derivedStatus", "derived"}} {
		if value := dashboardString(complete[item.key]); value != "" {
			chips = append(chips, dashboardChip{Tone: "num", Label: item.label + ":" + value})
		}
	}
	if _, mok := complete["vectorMemoryUpserted"]; mok {
		chips = append(chips, dashboardChip{Tone: "num", Label: fmt.Sprintf("vecLane m:%s e:%s r:%s", dashboardNumberString(complete["vectorMemoryUpserted"]), dashboardNumberString(complete["vectorEvidenceUpserted"]), dashboardNumberString(complete["vectorWorldRuleUpserted"]))})
	}
	detailCode := dashboardRuntimeReasonCode(complete)
	detail := dashboardStringValue(complete["detail"])
	if detail != "" {
		chips = append(chips, dashboardChip{Tone: "num", Label: detail})
		if detailCode != "" {
			chips[len(chips)-1].Label = "@" + detailCode
		}
	}
	for _, reason := range dashboardStrings(complete["failReasons"]) {
		chips = append(chips, dashboardChip{Tone: "fail", Label: reason})
	}
	status := normalizeDashboardRuntimeStatus(complete)
	source := dashboardFirstNonEmpty(dashboardString(complete["source"]), "local")
	if isDuplicateDashboardState(complete) {
		source = "@existingAccepted"
	}
	chips = append([]dashboardChip{{Tone: dashboardSeverity(status), Label: "[" + source + "]"}}, chips...)
	card := newDashboardCard("complete_turn", "✅", "Complete Turn", nil)
	card.Severity = dashboardSeverity(status)
	switch card.Severity {
	case "ok":
		card.Summary.OK = 1
	case "neutral":
		card.Summary.Neutral = 1
	case "notice":
		card.Summary.Notice = 1
	case "warn":
		card.Summary.Warn = 1
	case "fail":
		card.Summary.Fail = 1
	default:
		card.Summary.Unknown = 1
	}
	card.Chips = chips
	return card
}

func buildPersistenceDashboardRows(complete map[string]any) []dashboardRow {
	if isDuplicateDashboardState(complete) {
		return []dashboardRow{
			{LabelKey: "rawSave", Status: "ok", DetailCode: "noNewLaneNeeded", TurnIndex: complete["turnIndex"]},
			{LabelKey: "derived", Status: "ok", DetailCode: "noNewLaneNeeded", TurnIndex: complete["turnIndex"]},
			{LabelKey: "vectorUpsert", Status: "ok", DetailCode: "noNewLaneNeeded", TurnIndex: complete["turnIndex"]},
		}
	}
	pipeline := dashboardMap(complete["persistencePipeline"])
	rows := []dashboardRow{}
	for _, lane := range []struct {
		key, label, statusKey, countKey string
		counts                          []struct{ key, label string }
	}{
		{"raw", "rawSave", "rawStatus", "chatLogsSaved", []struct{ key, label string }{{"chatLogsSaved", "log"}}},
		{"derived", "derived", "derivedStatus", "derivedArtifactsSaved", []struct{ key, label string }{{"memoriesSaved", "mem"}, {"evidenceSaved", "evi"}, {"kgTriplesSaved", "kg"}, {"worldRulesSaved", "rule"}, {"derivedArtifactsSaved", "der"}}},
		{"vector", "vectorUpsert", "vectorStatus", "vectorUpserted", []struct{ key, label string }{{"vectorUpserted", "total"}, {"vectorMemoryUpserted", "mem"}, {"vectorEvidenceUpserted", "evi"}, {"vectorWorldRuleUpserted", "rule"}}},
	} {
		laneState := dashboardMap(pipeline[lane.key])
		statusValue := dashboardFirstNonEmpty(dashboardString(laneState["status"]), dashboardString(complete[lane.statusKey]))
		parts := []string{dashboardFirstNonEmpty(statusValue, "unknown")}
		if lane.key == "derived" {
			if value, ok := laneState["attempted"]; ok {
				parts = append(parts, "attempted:"+dashboardNumberString(value))
			}
			if value, ok := laneState["committed"]; ok {
				parts = append(parts, "committed:"+dashboardNumberString(value))
			}
			if rollback := dashboardString(laneState["rollback_state"]); rollback != "" && rollback != "not_applicable" {
				parts = append(parts, "transaction:"+rollback)
			}
			for _, raw := range sliceFromAny(laneState["error_diagnostics"]) {
				diagnostic := mapFromAny(raw)
				operation := strings.TrimSpace(stringFromMap(diagnostic, "operation"))
				cause := strings.TrimSpace(stringFromMap(diagnostic, "cause"))
				if operation != "" || cause != "" {
					parts = append(parts, strings.TrimSpace(operation+": "+cause))
				}
			}
		}
		for _, count := range lane.counts {
			if value, ok := complete[count.key]; ok && value != nil {
				parts = append(parts, count.label+":"+dashboardNumberString(value))
			}
		}
		countValue := complete[lane.countKey]
		if lane.key == "derived" && laneState["committed"] != nil {
			countValue = laneState["committed"]
		}
		rows = append(rows, dashboardRow{LabelKey: lane.label, Status: dashboardLaneStatus(statusValue, countValue), Detail: strings.Join(parts, " / "), TurnIndex: complete["turnIndex"]})
	}
	return rows
}

func buildDashboardTimingRows(prepare, complete map[string]any) []dashboardRow {
	rows := []dashboardRow{}
	for _, item := range []struct {
		label string
		state map[string]any
	}{{"prepareTiming", prepare}, {"completeTiming", complete}} {
		timing := dashboardMap(item.state["backendTiming"])
		if len(timing) == 0 {
			continue
		}
		total := dashboardFloat(timing["total_ms"])
		slowest := dashboardFloat(timing["slowest_ms"])
		stage := dashboardString(timing["slowest_stage"])
		parts := []string{"total " + dashboardDuration(total)}
		if stage != "" {
			parts = append(parts, "slowest: "+stage+" "+dashboardDuration(slowest))
		}
		stages := dashboardMap(timing["stages_ms"])
		type stageTiming struct {
			name string
			ms   float64
		}
		ordered := []stageTiming{}
		for name, value := range stages {
			ms := dashboardFloat(value)
			if ms >= 1 && name != stage {
				ordered = append(ordered, stageTiming{name, ms})
			}
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].ms > ordered[j].ms })
		for i, value := range ordered {
			if i >= 5 {
				break
			}
			parts = append(parts, value.name+" "+dashboardDuration(value.ms))
		}
		status := "ok"
		if slowest >= 10000 {
			status = "notice"
		}
		rows = append(rows, dashboardRow{LabelKey: item.label, Status: status, Detail: strings.Join(parts, " / ")})
	}
	return rows
}

func newDashboardCard(id, icon, title string, rows []dashboardRow) dashboardCard {
	counts := dashboardCounts{}
	for _, row := range rows {
		switch dashboardSeverity(row.Status) {
		case "fail":
			counts.Fail++
		case "warn":
			counts.Warn++
		case "notice":
			counts.Notice++
		case "ok":
			counts.OK++
		case "neutral":
			counts.Neutral++
		default:
			counts.Unknown++
		}
	}
	severity := "ok"
	if counts.Fail > 0 {
		severity = "fail"
	} else if counts.Warn > 0 {
		severity = "warn"
	} else if counts.Notice > 0 {
		severity = "notice"
	} else if counts.OK == 0 && counts.Neutral > 0 {
		severity = "neutral"
	} else if counts.OK == 0 {
		severity = "unknown"
	}
	return dashboardCard{ID: id, Icon: icon, Title: title, Severity: severity, Summary: counts, Rows: rows}
}

func dashboardRowFromState(label string, input map[string]any) dashboardRow {
	code := dashboardRuntimeReasonCodeForRow(label, input)
	status := normalizeDashboardRuntimeStatus(input, code)
	detail := dashboardStringValue(input["detail"])
	return dashboardRow{LabelKey: label, Status: status, DetailCode: code, Detail: detail, Time: dashboardString(input["time"]), TurnIndex: input["turnIndex"], ItemCount: firstNonNil(input["itemCount"], input["count"]), Placement: input["placement"]}
}

func normalizeDashboardRuntimeStatus(input map[string]any, observedReasonCode ...string) string {
	status := dashboardStatus(input)
	code := dashboardRuntimeReasonCode(input)
	if len(observedReasonCode) > 0 {
		code = strings.TrimSpace(observedReasonCode[0])
	}
	switch code {
	case "duplicateExisting", "existingAccepted", "supervisorOkByTurn", "streamingRecovered", "noMissingBackfill", "queueOk":
		return "ok"
	case "streamingWaitFinal":
		return "running"
	case "deletedTurnSynced", "rerollReplaced", "historyTrimProtected", "pendingSync", "postOutputPending", "beforeRequestRecovered", "forkCopyObserved", "legacyQueueItemRemoved", "activeChatRebuildQueued":
		return "notice"
	}
	switch strings.ToLower(dashboardString(input["severity"])) {
	case "error", "fail":
		return "fail"
	case "warning", "warn":
		return "warn"
	case "notice", "info", "informational":
		return "notice"
	case "neutral":
		return "neutral"
	case "unknown", "unobserved":
		return "unknown"
	}
	return status
}

func dashboardRuntimeReasonCodeForRow(label string, input map[string]any) string {
	if code := dashboardRuntimeReasonCode(input); code != "" {
		return code
	}
	if label != "activeChatBackfill" {
		return ""
	}
	saved := dashboardInt(firstNonNil(input["savedCount"], input["saved_count"]))
	existing := dashboardInt(firstNonNil(input["existingCount"], input["existing_count"]))
	queued := dashboardInt(firstNonNil(input["queuedCount"], input["queued_count"]))
	skipped := dashboardInt(firstNonNil(input["skippedCount"], input["skipped_count"]))
	if saved == 0 && existing > 0 && queued == 0 && skipped == 0 {
		return "noMissingBackfill"
	}
	return ""
}

func dashboardRuntimeReasonCode(input map[string]any) string {
	code := dashboardFirstNonEmpty(
		dashboardString(input["reason_code"]),
		dashboardString(input["detail_code"]),
	)
	if code == "" {
		detail := dashboardMap(input["detail"])
		code = dashboardFirstNonEmpty(
			dashboardString(detail["reason_code"]),
			dashboardString(detail["detail_code"]),
		)
	}
	switch strings.ToLower(code) {
	case "idempotent_pair_replay", "duplicate_existing":
		return "duplicateExisting"
	case "accepted_existing_pair", "existing_accepted":
		return "existingAccepted"
	case "health_test_not_run_turn_call_ok", "supervisor_ok_by_turn":
		return "supervisorOkByTurn"
	case "first_turn_light":
		return "firstTurnLight"
	case "fragment_skipped_waiting_final", "streaming_wait_final":
		return "streamingWaitFinal"
	case "native_after_request_active_chat_recovered", "streaming_recovered":
		return "streamingRecovered"
	case "streaming_timeout":
		return "streamingTimeout"
	case "assistant_deleted_output_removed", "active_chat_tail_missing_from_runtime", "deleted_turn_synced":
		return "deletedTurnSynced"
	case "logical_turn_replaced", "reroll_replaced":
		return "rerollReplaced"
	case "unverified_rollback_signal_blocked", "rollback_blocked_unverified":
		return "rollbackBlockedUnverified"
	case "history_trim_protected", "blind_tail_reconcile_blocked":
		return "historyTrimProtected"
	case "recent_completed_turn_waiting_active_chat_sync", "waiting_for_risuai_active_chat", "source_acceptance_waiting_active_chat", "pending_sync":
		return "pendingSync"
	case "post_output_final_pending", "post_output_final_replacement_pending":
		return "postOutputPending"
	case "before_request_payload_recovered":
		return "beforeRequestRecovered"
	case "risu_fork_copy_observed":
		return "forkCopyObserved"
	case "legacy_startup_message_write_removed":
		return "legacyQueueItemRemoved"
	case "recent_active_chat_rebuild_queued":
		return "activeChatRebuildQueued"
	case "no_tracked_turn_index":
		return "noTrackedTurn"
	case "no_completed_pairs":
		return "noCompletedPairs"
	case "no_missing_backfill":
		return "noMissingBackfill"
	case "queue_storage_ok":
		return "queueOk"
	case "local_only":
		return "localOnly"
	case "synced":
		return "synced"
	}
	return strings.TrimSpace(code)
}

func isDuplicateDashboardState(input map[string]any) bool {
	code := dashboardRuntimeReasonCode(input)
	return code == "duplicateExisting" || code == "existingAccepted"
}
func dashboardSeverity(status string) string {
	switch strings.ToLower(status) {
	case "ok", "eligible":
		return "ok"
	case "empty", "not_applicable", "skipped", "off", "idle":
		return "neutral"
	case "notice", "info", "informational", "deferred", "queued", "pending", "delayed", "waiting", "running", "watching":
		return "notice"
	case "warn", "warning", "degraded", "partial", "fallback", "ambiguous":
		return "warn"
	case "fail", "error", "failed", "incompatible":
		return "fail"
	default:
		return "unknown"
	}
}
func dashboardStatus(input map[string]any) string {
	return dashboardFirstNonEmpty(strings.ToLower(dashboardString(input["status"])), "unknown")
}
func dashboardBoolStatus(value bool) string {
	if value {
		return "ok"
	}
	return "fail"
}
func dashboardBoolUnknownStatus(value bool) string {
	if value {
		return "ok"
	}
	return "unknown"
}
func dashboardEnabledDetail(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}
func dashboardSyncDetail(value bool) string {
	if value {
		return "synced"
	}
	return "local only"
}
func dashboardSyncReasonCode(value bool) string {
	if value {
		return "synced"
	}
	return "local_only"
}
func dashboardWarnIf(value bool) string {
	if value {
		return "warn"
	}
	return "ok"
}
func dashboardLaneStatus(status string, fallback any) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		if dashboardFloat(fallback) > 0 {
			return "ok"
		}
		return "unknown"
	}
	switch status {
	case "ok", "saved", "present", "completed", "accepted", "upserted", "repaired":
		return "ok"
	case "skipped", "not_called", "not_configured", "disabled", "empty", "none":
		return "skipped"
	case "queued", "pending", "delayed":
		return "notice"
	case "partial", "degraded", "fallback", "missing_suspected", "not_checked_no_raw":
		return "warn"
	case "fail", "failed", "error", "missing", "lost", "blocked":
		return "fail"
	}
	return "warn"
}
func dashboardMap(value any) map[string]any {
	if out, ok := value.(map[string]any); ok && out != nil {
		return out
	}
	return map[string]any{}
}
func dashboardCloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
func dashboardString(value any) string {
	if value == nil {
		return ""
	}
	if out, ok := value.(string); ok {
		return strings.TrimSpace(out)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
func dashboardStringValue(value any) string {
	if value == nil {
		return ""
	}
	if out, ok := value.(string); ok {
		return strings.TrimSpace(out)
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}
func dashboardFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	}
	return 0
}
func dashboardInt(value any) int { return int(dashboardFloat(value)) }
func dashboardNumberString(value any) string {
	if value == nil {
		return "?"
	}
	n := dashboardFloat(value)
	if n == float64(int64(n)) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
func dashboardStrings(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if out, ok := value.([]string); ok {
			return out
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s := dashboardString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func dashboardDuration(ms float64) string {
	if ms >= 1000 {
		precision := 2
		if ms >= 10000 {
			precision = 1
		}
		return strconv.FormatFloat(ms/1000, 'f', precision, 64) + "s"
	}
	return strconv.FormatInt(int64(ms+0.5), 10) + "ms"
}
func shortenDashboardSessionID(value string) string {
	if len(value) <= 26 {
		return value
	}
	return value[:14] + "…" + value[len(value)-8:]
}
func dashboardFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
