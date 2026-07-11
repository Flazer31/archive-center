package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const dashboardViewModelContractVersion = "dashboard.viewmodel.v1"

type dashboardViewModelRequest struct {
	RuntimeState             map[string]any `json:"runtime_state"`
	PluginEnabled            bool           `json:"plugin_enabled"`
	CurrentSessionID         string         `json:"current_session_id"`
	SessionCandidates        map[string]any `json:"session_candidates"`
	PrepareTurnEverContacted bool           `json:"prepare_turn_ever_contacted"`
	FailedQueueDepth         int            `json:"failed_queue_depth"`
	GuideModeState           map[string]any `json:"guide_mode_state"`
	FirstTurnLight           bool           `json:"first_turn_light"`
	FirstTurnEndedAt         string         `json:"first_turn_ended_at"`
}

type dashboardViewModel struct {
	ContractVersion string          `json:"contract_version"`
	Status          string          `json:"status"`
	Summary         dashboardCounts `json:"summary"`
	Cards           []dashboardCard `json:"cards"`
}

type dashboardCounts struct {
	OK      int `json:"ok"`
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
	writeJSON(w, http.StatusOK, buildDashboardViewModel(req))
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
		out["detail"] = "first turn light mode"
		if dashboardString(out["time"]) == "" {
			out["time"] = req.FirstTurnEndedAt
		}
		return out
	}

	sessionID := strings.TrimSpace(req.CurrentSessionID)
	if sessionID == "" {
		sessionID = dashboardFirstNonEmpty(
			dashboardString(req.SessionCandidates["runtime_current"]),
			dashboardString(req.SessionCandidates["timeline_current"]),
		)
	}
	if sessionID == "" {
		routing := state("sessionWriteRouting")
		sessionID = dashboardFirstNonEmpty(dashboardString(routing["targetSessionId"]), dashboardString(routing["rawSessionId"]))
	}
	sessionState := map[string]any{"status": "unknown", "detail": "resolving..."}
	if sessionID != "" {
		sessionState = map[string]any{"status": "ok", "detail": shortenDashboardSessionID(sessionID)}
	}

	supervisorHealth := state("lastSupervisorWakeup")
	if dashboardStatus(supervisorHealth) == "unknown" && dashboardStatus(state("lastSupervisorStatus")) == "ok" {
		supervisorHealth = map[string]any{
			"status": "skipped",
			"time":   state("lastSupervisorStatus")["time"],
			"detail": "health test not run / turn call ok",
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
			dashboardRowFromState("runtimeSync", map[string]any{"status": dashboardBoolUnknownStatus(req.PrepareTurnEverContacted), "detail": dashboardSyncDetail(req.PrepareTurnEverContacted)}),
		}),
	}
	if critic := buildCriticLedgerDashboardCard(state("lastCriticLedgerProbe")); critic != nil {
		cards = append(cards, *critic)
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
	retryStatus, retryDetail := "ok", "empty"
	if req.FailedQueueDepth > 0 {
		retryStatus, retryDetail = "warn", strconv.Itoa(req.FailedQueueDepth)+" pending"
	}
	cards = append(cards, newDashboardCard("save_queue", "💾", "Save / Queue", []dashboardRow{
		dashboardRowFromState("injection", firstTurnState(state("lastInjectionStatus"))),
		dashboardRowFromState("save", state("lastSaveStatus")),
		dashboardRowFromState("complete", state("lastCompleteStatus")),
		dashboardRowFromState("retryQueue", map[string]any{"status": retryStatus, "detail": retryDetail}),
		dashboardRowFromState("queueStorage", map[string]any{"status": dashboardStatus(save), "detail": dashboardFirstNonEmpty(strings.Join(queueDetail, " / "), "not yet")}),
	}))

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
		summary.Warn += card.Summary.Warn
		summary.Fail += card.Summary.Fail
		summary.Unknown += card.Summary.Unknown
	}
	return dashboardViewModel{ContractVersion: dashboardViewModelContractVersion, Status: "ok", Summary: summary, Cards: cards}
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
	detailCode, detail := classifyDashboardDetail(complete["detail"])
	if detail != "" {
		chips = append(chips, dashboardChip{Tone: "num", Label: detail})
		if detailCode != "" {
			chips[len(chips)-1].Label = "@" + detailCode
		}
	}
	for _, reason := range dashboardStrings(complete["failReasons"]) {
		chips = append(chips, dashboardChip{Tone: "fail", Label: reason})
	}
	status := normalizeDashboardStatus(dashboardStatus(complete), complete["detail"])
	source := dashboardFirstNonEmpty(dashboardString(complete["source"]), "local")
	if isDuplicateDashboardDetail(complete["detail"]) {
		source = "@existingAccepted"
	}
	chips = append([]dashboardChip{{Tone: dashboardSeverity(status), Label: "[" + source + "]"}}, chips...)
	card := newDashboardCard("complete_turn", "✅", "Complete Turn", nil)
	card.Severity = dashboardSeverity(status)
	switch card.Severity {
	case "ok":
		card.Summary.OK = 1
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
	if isDuplicateDashboardDetail(complete["detail"]) {
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
		for _, count := range lane.counts {
			if value, ok := complete[count.key]; ok && value != nil {
				parts = append(parts, count.label+":"+dashboardNumberString(value))
			}
		}
		rows = append(rows, dashboardRow{LabelKey: lane.label, Status: dashboardLaneStatus(statusValue, complete[lane.countKey]), Detail: strings.Join(parts, " / "), TurnIndex: complete["turnIndex"]})
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
			status = "warn"
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
		case "ok":
			counts.OK++
		default:
			counts.Unknown++
		}
	}
	severity := "ok"
	if counts.Fail > 0 {
		severity = "fail"
	} else if counts.Warn > 0 {
		severity = "warn"
	} else if counts.OK == 0 {
		severity = "unknown"
	}
	return dashboardCard{ID: id, Icon: icon, Title: title, Severity: severity, Summary: counts, Rows: rows}
}

func dashboardRowFromState(label string, input map[string]any) dashboardRow {
	status := normalizeDashboardStatus(dashboardStatus(input), input["detail"])
	code, detail := classifyDashboardDetail(input["detail"])
	return dashboardRow{LabelKey: label, Status: status, DetailCode: code, Detail: detail, Time: dashboardString(input["time"]), TurnIndex: input["turnIndex"], ItemCount: firstNonNil(input["itemCount"], input["count"]), Placement: input["placement"]}
}

var dashboardDetailPatterns = []struct {
	code string
	re   *regexp.Regexp
}{
	{"duplicateExisting", regexp.MustCompile(`(?i)idempotent pair replay|idempotent_pair_replay|duplicate save skipped`)},
	{"existingAccepted", regexp.MustCompile(`(?i)accepted \(existing pair\)`)},
	{"supervisorOkByTurn", regexp.MustCompile(`(?i)health test not run\s*/\s*turn call ok`)},
	{"firstTurnLight", regexp.MustCompile(`(?i)first turn light mode`)},
	{"streamingWaitFinal", regexp.MustCompile(`(?i)native non-persistable fragment ignored|waiting final output`)},
	{"streamingRecovered", regexp.MustCompile(`(?i)native afterRequest missing; recovered from active chat`)},
	{"streamingTimeout", regexp.MustCompile(`(?i)timeout waiting for native afterRequest/active assistant`)},
	{"deletedTurnSynced", regexp.MustCompile(`(?i)(active_chat_tail_missing_from_runtime|assistant_deleted_output_removed).*(rolled back|rollback)|(rolled back|rollback).*(active_chat_tail_missing_from_runtime|assistant_deleted_output_removed)`)},
	{"rollbackBlockedUnverified", regexp.MustCompile(`(?i)unverified rollback signal blocked`)},
	{"historyTrimProtected", regexp.MustCompile(`(?i)active chat tail is shorter than backend|history trim/cut protected|possible /cut`)},
	{"pendingSync", regexp.MustCompile(`(?i)recent_completed_turn_waiting_active_chat_sync`)},
	{"noTrackedTurn", regexp.MustCompile(`(?i)no_tracked_turn_index`)},
	{"noCompletedPairs", regexp.MustCompile(`(?i)no_completed_pairs`)},
	{"noMissingBackfill", regexp.MustCompile(`(?i)active chat backfill 0 saved / 1 existing`)},
	{"queueOk", regexp.MustCompile(`(?i)^load:ok\s*/\s*save:ok$`)},
	{"localOnly", regexp.MustCompile(`(?i)^local only$`)},
	{"synced", regexp.MustCompile(`(?i)^synced$`)},
}

func classifyDashboardDetail(value any) (string, string) {
	raw := dashboardStringValue(value)
	for _, pattern := range dashboardDetailPatterns {
		if pattern.re.MatchString(raw) {
			return pattern.code, raw
		}
	}
	return "", raw
}

func normalizeDashboardStatus(status string, detail any) string {
	status = dashboardFirstNonEmpty(strings.ToLower(status), "unknown")
	code, _ := classifyDashboardDetail(detail)
	switch code {
	case "duplicateExisting", "existingAccepted", "supervisorOkByTurn", "noMissingBackfill", "queueOk":
		return "ok"
	case "streamingWaitFinal":
		return "running"
	}
	return status
}

func isDuplicateDashboardDetail(value any) bool {
	code, _ := classifyDashboardDetail(value)
	return code == "duplicateExisting" || code == "existingAccepted"
}
func dashboardSeverity(status string) string {
	switch strings.ToLower(status) {
	case "ok":
		return "ok"
	case "warn":
		return "warn"
	case "fail", "error":
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
func dashboardWarnIf(value bool) string {
	if value {
		return "warn"
	}
	return "ok"
}
func dashboardLaneStatus(status string, fallback any) string {
	status = strings.ToLower(status)
	if status == "" {
		if dashboardFloat(fallback) > 0 {
			return "ok"
		}
		return "unknown"
	}
	if regexp.MustCompile(`^(ok|saved|present|completed|accepted|upserted|repaired)$`).MatchString(status) {
		return "ok"
	}
	if regexp.MustCompile(`^(skipped|not_called|not_configured|disabled|empty|none)$`).MatchString(status) {
		return "skipped"
	}
	if regexp.MustCompile(`^(queued|pending|delayed|partial|degraded|fallback|missing_suspected|not_checked_no_raw)$`).MatchString(status) {
		return "warn"
	}
	if regexp.MustCompile(`^(fail|failed|error|missing|lost|blocked)$`).MatchString(status) {
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
