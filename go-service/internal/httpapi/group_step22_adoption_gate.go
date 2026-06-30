package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	step22AdoptionGateContractVersion = "step22_adoption_gate.v1"
	step22AdoptionGateRoute           = "/validation/step22/adoption-gate/preview"
)

type step22AdoptionGateCheck struct {
	ID               string         `json:"id"`
	Label            string         `json:"label"`
	Status           string         `json:"status"`
	RequiredForGreen bool           `json:"required_for_green"`
	Evidence         map[string]any `json:"evidence"`
	Blockers         []string       `json:"blockers"`
	Warnings         []string       `json:"warnings"`
}

func (s *Server) registerStep22ValidationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step22AdoptionGateRoute, s.handleStep22AdoptionGatePreview)
}

func (s *Server) handleStep22AdoptionGatePreview(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sid := strings.TrimSpace(query.Get("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	turnIndex := step22IntQuery(query.Get("turn_index"), 0)
	limit := step22ClampInt(step22IntQuery(query.Get("limit"), 50), 1, 200)
	req := criticArchiveLedgerPreviewRequest{
		ChatSessionID:          sid,
		TurnIndex:              turnIndex,
		AssistantFinalText:     strings.TrimSpace(query.Get("assistant_final_text")),
		AssistantFinalLanguage: strings.TrimSpace(query.Get("assistant_final_language")),
		StreamingMismatch:      strings.TrimSpace(query.Get("streaming_mismatch")),
	}
	rawUserInput := strings.TrimSpace(query.Get("raw_user_input"))
	progressionProfile := strings.TrimSpace(query.Get("progression_profile"))
	opsGate := map[string]string{
		"schema_migration_state": strings.TrimSpace(query.Get("schema_migration_state")),
		"backfill_state":         strings.TrimSpace(query.Get("backfill_state")),
		"rollback_state":         strings.TrimSpace(query.Get("rollback_state")),
	}

	resp := s.buildStep22AdoptionGatePreview(r, sid, turnIndex, rawUserInput, progressionProfile, req, limit, opsGate)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) buildStep22AdoptionGatePreview(r *http.Request, sid string, turnIndex int, rawUserInput, progressionProfile string, ledgerReq criticArchiveLedgerPreviewRequest, limit int, opsGate map[string]string) map[string]any {
	ctx := r.Context()
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	warnings := []string{}

	ledger := criticArchiveLedgerPreviewResponse{
		Status:          "disabled",
		ContractVersion: criticArchiveLedgerContractVersion,
		SessionID:       sid,
	}
	if s.Cfg.CriticLedgerPreviewEnabled {
		ledger = s.buildCriticArchiveLedgerPreview(r, ledgerReq)
	} else {
		warnings = append(warnings, "critic_ledger_preview_disabled")
	}

	cdmCandidates, cdmWarnings, cdmTrace := s.buildMaintenanceContradictionDuplicatePreview(r, sid, limit)
	warnings = append(warnings, cdmWarnings...)

	narrativePacket := s.buildNarrativeRecallPacketPreview(r, sid, turnIndex, rawUserInput, progressionProfile, 12)

	resolutionRecords, resolutionWarnings := s.step22SupersessionResolutionRecords(ctx, sid, limit)
	warnings = append(warnings, resolutionWarnings...)

	checks := []step22AdoptionGateCheck{
		step22RetrievalReassemblyCheck(narrativePacket),
		step22DuplicateInflationCheck(cdmCandidates),
		step22StaleResidueClosureCheck(cdmCandidates, resolutionRecords),
		step22RelationshipSceneCarryoverCheck(narrativePacket),
		step22OutputLanguageParityCheck(ledgerReq, ledger),
		step22ReasoningLeakageCheck(ledger),
		step22PromptAuthorityCheck(narrativePacket),
		step22ProgressionProfileCheck(narrativePacket),
		step22SchemaBackfillRollbackCheck(opsGate),
	}
	sort.SliceStable(checks, func(i, j int) bool {
		return step22CheckOrder(checks[i].ID) < step22CheckOrder(checks[j].ID)
	})

	blockers := []string{}
	warnIDs := []string{}
	passCount := 0
	for _, check := range checks {
		switch check.Status {
		case "pass":
			passCount++
		case "blocked":
			if check.RequiredForGreen {
				blockers = append(blockers, check.ID)
			}
		default:
			warnIDs = append(warnIDs, check.ID)
		}
	}
	gateState := "ready"
	defaultEnableAllowed := true
	status := "ok"
	if len(blockers) > 0 {
		gateState = "closed"
		defaultEnableAllowed = false
		status = "blocked"
	} else if len(warnIDs) > 0 || len(warnings) > 0 {
		gateState = "review"
		defaultEnableAllowed = false
		status = "review"
	}

	return map[string]any{
		"status":                    status,
		"contract_version":          step22AdoptionGateContractVersion,
		"route":                     step22AdoptionGateRoute,
		"session_id":                sid,
		"turn_index":                turnIndex,
		"generated_at":              generatedAt,
		"read_only":                 true,
		"adoption_gate_state":       gateState,
		"default_enable_allowed":    defaultEnableAllowed,
		"live_feature_write":        false,
		"write_attempted":           false,
		"vector_write_attempted":    false,
		"llm_call_attempted":        false,
		"pass_count":                passCount,
		"check_count":               len(checks),
		"blockers":                  blockers,
		"warnings":                  warnings,
		"checks":                    checks,
		"required_green_gates":      step22RequiredGreenGates(),
		"source_contract_versions":  step22SourceContractVersions(),
		"ops_gate_inputs":           opsGate,
		"cdm_candidate_count":       len(cdmCandidates),
		"resolution_record_count":   len(resolutionRecords),
		"narrative_packet_status":   step22String(narrativePacket["status"]),
		"critic_ledger_status":      ledger.Status,
		"maintenance_trace_summary": cdmTrace,
		"trace": map[string]any{
			"contract_owner":         "22-5",
			"truth_boundary":         "validation_only_no_runtime_adoption",
			"default_action":         "stay_off_until_gate_ready",
			"auto_apply":             false,
			"write_attempted":        false,
			"vector_write_attempted": false,
			"llm_call_attempted":     false,
		},
	}
}

func (s *Server) step22SupersessionResolutionRecords(ctx context.Context, sid string, limit int) ([]map[string]any, []string) {
	records := []map[string]any{}
	warnings := []string{}
	if resolver, ok := s.Store.(store.SupersessionResolutionStore); ok {
		items, err := resolver.ListSupersessionResolutions(ctx, sid, limit)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "supersession_resolution_store_unavailable: "+err.Error())
		}
		for _, item := range items {
			records = append(records, supersessionResolutionRecordMap(item))
		}
	}
	if len(records) == 0 {
		audits, err := s.Store.ListAuditLogs(ctx, sid, "supersession_resolution", limit)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "supersession_resolution_audits_unavailable: "+err.Error())
		}
		for _, audit := range audits {
			records = append(records, supersessionResolutionAuditMap(audit))
		}
	}
	return records, warnings
}

func step22RetrievalReassemblyCheck(packet map[string]any) step22AdoptionGateCheck {
	sourceCounts := step22Map(packet["source_counts"])
	sourceTotal := 0
	for _, raw := range sourceCounts {
		sourceTotal += step22IntAny(raw)
	}
	relationship := step22Map(packet["relationship_packet"])
	carryover := step22Map(packet["carryover"])
	heavy := step22Slice(carryover["heavy_carryover"])
	status := step22String(packet["status"])
	check := step22AdoptionGateCheck{
		ID:               "retrieval_success_vs_narrative_reassembly_failure",
		Label:            "retrieval success vs narrative reassembly failure replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"source_total":            sourceTotal,
			"narrative_packet_status": status,
			"relationship_shift":      step22Map(relationship["relationship_shift"])["summary"],
			"heavy_carryover_count":   len(heavy),
		},
	}
	switch {
	case sourceTotal == 0:
		check.Status = "warning"
		check.Warnings = []string{"no_replay_corpus_sources"}
	case status == "empty" || len(heavy) == 0:
		check.Status = "blocked"
		check.Blockers = []string{"retrieval_sources_exist_but_reassembly_packet_empty"}
	default:
		check.Status = "pass"
	}
	return check
}

func step22DuplicateInflationCheck(candidates []maintenanceCDMCandidate) step22AdoptionGateCheck {
	duplicateCount := 0
	types := map[string]int{}
	for _, item := range candidates {
		types[item.CandidateType]++
		if strings.Contains(item.CandidateType, "duplicate") {
			duplicateCount++
		}
	}
	check := step22AdoptionGateCheck{
		ID:               "duplicate_inflation_reduction",
		Label:            "duplicate inflation reduction replay",
		RequiredForGreen: true,
		Evidence:         map[string]any{"duplicate_candidate_count": duplicateCount, "candidate_types": types},
	}
	if duplicateCount > 0 {
		check.Status = "blocked"
		check.Blockers = []string{"duplicate_candidates_remain_review_required"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22StaleResidueClosureCheck(candidates []maintenanceCDMCandidate, resolutionRecords []map[string]any) step22AdoptionGateCheck {
	residueCount := 0
	for _, item := range candidates {
		if item.CandidateType == "thread_open_closed_contradiction" {
			residueCount++
		}
	}
	check := step22AdoptionGateCheck{
		ID:               "stale_residue_closure",
		Label:            "stale residue closure replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"open_closed_contradiction_count": residueCount,
			"supersession_resolution_records": len(resolutionRecords),
		},
	}
	switch {
	case residueCount > 0:
		check.Status = "blocked"
		check.Blockers = []string{"open_thread_conflicts_with_closed_resolution"}
	case len(resolutionRecords) == 0:
		check.Status = "warning"
		check.Warnings = []string{"no_supersession_resolution_replay_evidence"}
	default:
		check.Status = "pass"
	}
	return check
}

func step22RelationshipSceneCarryoverCheck(packet map[string]any) step22AdoptionGateCheck {
	relationship := step22Map(packet["relationship_packet"])
	shift := step22Map(relationship["relationship_shift"])
	tensions := step22Slice(relationship["unresolved_tension"])
	carryover := step22Map(packet["carryover"])
	heavy := step22Slice(carryover["heavy_carryover"])
	light := step22Slice(carryover["light_resurfacing_tag"])
	scene := step22Map(packet["scene_microstate"])
	check := step22AdoptionGateCheck{
		ID:               "relationship_shift_scene_carryover",
		Label:            "relationship-shift / scene carryover replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"relationship_shift_present": step22String(shift["summary"]) != "",
			"unresolved_tension_count":   len(tensions),
			"heavy_carryover_count":      len(heavy),
			"light_resurfacing_count":    len(light),
			"scene_type":                 scene["scene_type"],
			"immediate_pressure":         scene["immediate_pressure"],
		},
	}
	if step22String(shift["summary"]) == "" || len(heavy) == 0 || step22String(scene["scene_type"]) == "" {
		check.Status = "blocked"
		check.Blockers = []string{"relationship_or_scene_carryover_packet_incomplete"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22OutputLanguageParityCheck(req criticArchiveLedgerPreviewRequest, ledger criticArchiveLedgerPreviewResponse) step22AdoptionGateCheck {
	check := step22AdoptionGateCheck{
		ID:               "output_language_parity",
		Label:            "output-language parity replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"requested_language": req.AssistantFinalLanguage,
			"ledger_language":    ledger.Language.AssistantFinalLanguage,
			"language_source":    ledger.Language.Source,
		},
	}
	if strings.TrimSpace(req.AssistantFinalLanguage) == "" {
		check.Status = "warning"
		check.Warnings = []string{"assistant_final_language_not_provided"}
		return check
	}
	if !strings.EqualFold(req.AssistantFinalLanguage, ledger.Language.AssistantFinalLanguage) {
		check.Status = "blocked"
		check.Blockers = []string{"ledger_language_does_not_match_final_output_language"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22ReasoningLeakageCheck(ledger criticArchiveLedgerPreviewResponse) step22AdoptionGateCheck {
	check := step22AdoptionGateCheck{
		ID:               "reasoning_leakage",
		Label:            "reasoning leakage replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"reasoning_scrub_applied":  ledger.Safety.ReasoningScrubApplied,
			"raw_archive_dump_blocked": ledger.Safety.RawArchiveDumpBlocked,
			"scrubbed_items":           ledger.Safety.ScrubbedItems,
			"streaming_mismatch":       ledger.Safety.StreamingMismatch,
		},
	}
	if !ledger.Safety.ReasoningScrubApplied || !ledger.Safety.RawArchiveDumpBlocked {
		check.Status = "blocked"
		check.Blockers = []string{"reasoning_scrub_or_raw_dump_guard_missing"}
		return check
	}
	if ledger.Safety.ScrubbedItems == 0 {
		check.Status = "warning"
		check.Warnings = []string{"no_reasoning_marker_fixture_was_scrubbed"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22PromptAuthorityCheck(packet map[string]any) step22AdoptionGateCheck {
	trace := step22Map(packet["prompt_authority_trace"])
	supervisor := step22Map(trace["supervisor_system_authority"])
	critic := step22Map(trace["critic_system_authority"])
	check := step22AdoptionGateCheck{
		ID:               "prompt_authority",
		Label:            "prompt authority replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"supervisor_source":                 supervisor["source"],
			"critic_source":                     critic["source"],
			"supervisor_code_fallback_override": supervisor["code_fallback_can_override"],
			"critic_code_fallback_override":     critic["code_fallback_can_override"],
			"fallback_only_when_unreadable":     supervisor["fallback_only_when_unreadable"],
			"authority_policy":                  trace["authority_policy"],
		},
	}
	if step22Bool(supervisor["code_fallback_can_override"]) || step22Bool(critic["code_fallback_can_override"]) {
		check.Status = "blocked"
		check.Blockers = []string{"code_fallback_can_override_configured_prompt"}
		return check
	}
	if step22String(supervisor["source"]) != "file" || step22String(critic["source"]) != "file" {
		check.Status = "warning"
		check.Warnings = []string{"prompt_file_authority_not_fully_backed_by_configured_files"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22ProgressionProfileCheck(packet map[string]any) step22AdoptionGateCheck {
	profile := step22Map(packet["progression_profile"])
	mustNotOverride := step22Slice(profile["must_not_override"])
	check := step22AdoptionGateCheck{
		ID:               "progression_profile",
		Label:            "progression profile replay",
		RequiredForGreen: true,
		Evidence: map[string]any{
			"requested":         profile["requested"],
			"resolved":          profile["resolved"],
			"label":             profile["label"],
			"source":            profile["source"],
			"authority":         profile["authority"],
			"must_not_override": mustNotOverride,
		},
	}
	if step22String(profile["resolved"]) == "" || step22String(profile["authority"]) != "pacing_preference_only" {
		check.Status = "blocked"
		check.Blockers = []string{"progression_profile_authority_missing_or_too_strong"}
		return check
	}
	if len(mustNotOverride) < 3 {
		check.Status = "blocked"
		check.Blockers = []string{"progression_profile_missing_override_guards"}
		return check
	}
	check.Status = "pass"
	return check
}

func step22SchemaBackfillRollbackCheck(opsGate map[string]string) step22AdoptionGateCheck {
	states := map[string]string{}
	blockers := []string{}
	for _, key := range []string{"schema_migration_state", "backfill_state", "rollback_state"} {
		state := strings.ToLower(strings.TrimSpace(opsGate[key]))
		states[key] = state
		if !step22GreenState(state) {
			blockers = append(blockers, key+"_not_green")
		}
	}
	check := step22AdoptionGateCheck{
		ID:               "schema_migration_backfill_rollback",
		Label:            "schema migration / backfill / rollback gate",
		RequiredForGreen: true,
		Evidence:         map[string]any{"states": states},
	}
	if len(blockers) > 0 {
		check.Status = "blocked"
		check.Blockers = blockers
		return check
	}
	check.Status = "pass"
	return check
}

func step22RequiredGreenGates() []string {
	return []string{
		"retrieval_success_vs_narrative_reassembly_failure",
		"duplicate_inflation_reduction",
		"stale_residue_closure",
		"relationship_shift_scene_carryover",
		"output_language_parity",
		"reasoning_leakage",
		"prompt_authority",
		"progression_profile",
		"schema_migration_backfill_rollback",
	}
}

func step22SourceContractVersions() map[string]string {
	return map[string]string{
		"critic_archive_ledger":               criticArchiveLedgerContractVersion,
		"supersession_resolution":             store.SupersessionResolutionContractVersion,
		"contradiction_duplicate_maintenance": maintenanceCDMContractVersion,
		"narrative_recall_packet":             narrativeRecallPacketContractVersion,
	}
}

func step22CheckOrder(id string) int {
	order := step22RequiredGreenGates()
	for idx, candidate := range order {
		if candidate == id {
			return idx
		}
	}
	return len(order) + 1
}

func step22GreenState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "green", "ready", "pass", "passed", "ok":
		return true
	default:
		return false
	}
}

func step22IntQuery(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func step22ClampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func step22Map(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok && typed != nil {
		return typed
	}
	if typed, ok := value.(map[string]int); ok && typed != nil {
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out
	}
	if typed, ok := value.(map[string]string); ok && typed != nil {
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out
	}
	return map[string]any{}
}

func step22Slice(value any) []any {
	if typed, ok := value.([]any); ok && typed != nil {
		return typed
	}
	if typed, ok := value.([]map[string]any); ok && typed != nil {
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	}
	if typed, ok := value.([]string); ok && typed != nil {
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

func step22String(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func step22Bool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
}

func step22IntAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		out, _ := typed.Int64()
		return int(out)
	default:
		out, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return out
	}
}
