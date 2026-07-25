package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnEvidenceCounts(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, resumePack *store.ResumePack, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary) map[string]any {
	return map[string]any{
		"memories":               len(memories),
		"kg_triples":             len(kgTriples),
		"direct_evidence":        len(evidence),
		"chat_logs":              len(chatLogs),
		"resume_pack_present":    resumePack != nil,
		"storylines":             len(storylines),
		"world_rules":            len(worldRules),
		"character_states":       len(charStates),
		"pending_threads":        len(pendingThreads),
		"active_states":          len(activeStates),
		"canonical_state_layers": len(canonicalLayers),
		"episode_summaries":      len(episodeSums),
	}
}

func prepareTurnSectionSummary(injectionText, inputContextText string, injectionTruncated, inputContextTruncated bool) []map[string]any {
	return []map[string]any{
		{
			"name":      "injection_text",
			"chars":     len([]rune(injectionText)),
			"available": strings.TrimSpace(injectionText) != "",
			"truncated": injectionTruncated,
			"sources":   []string{"memories", "kg_triples", "storylines", "world_rules", "character_states", "pending_threads"},
		},
		{
			"name":      "input_context_text",
			"chars":     len([]rune(inputContextText)),
			"available": strings.TrimSpace(inputContextText) != "",
			"truncated": inputContextTruncated,
			"sources":   []string{"direct_evidence", "chat_logs", "resume_pack", "active_states", "canonical_state_layers", "episode_summaries"},
		},
	}
}

func buildSupervisorInputPack(chatSessionID string, turnIndex int, rawUserInput, guideMode, guideStrength, narrativeStance, autoAdvanceTrigger, continuityQuery string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, storylineSelection storylineSupervisorSelection, degraded bool, fallbackReason string, languageContext map[string]any) map[string]any {
	guideMode = resolveNarrativeGuideMode(guideMode, nil, "", rawUserInput)
	guideStrength = normalizeNarrativeGuideStrength(guideStrength)
	guideFocus := buildNarrativeGuideFocus(guideMode)
	storylineSelectionTrace := storylineSelectionSummary(storylineSelection)
	plannerLanguageContract := buildPrepareTurnPlannerLanguageContract(languageContext)
	guidanceParts := []string{
		"[Go R1 Supervisor Read Shadow]",
		"mode=read_shadow; would_call_llm=false; would_write=false",
		fmt.Sprintf("guide_mode=%s; guide_strength=%s", guideMode, guideStrength),
		fmt.Sprintf("evidence_counts=%s", compactJSONForShadow(evidenceCounts, 500)),
		fmt.Sprintf("section_summary=%s", compactJSONForShadow(sectionSummary, 500)),
	}
	persistentGuidance := strings.Join(guidanceParts, "\n")
	finalGuidance := persistentGuidance
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":                    status,
		"source":                    "go_r1_read_shadow",
		"chat_session_id":           chatSessionID,
		"turn_index":                turnIndex,
		"raw_user_input_chars":      len([]rune(rawUserInput)),
		"prompt_assembly":           promptAssembly,
		"prompt_source":             promptAssembly["prompt_source"],
		"guide_mode":                guideMode,
		"guide_strength":            guideStrength,
		"guide_focus":               guideFocus,
		"language_context":          nilIfEmptyMap(languageContext),
		"planner_language_contract": plannerLanguageContract,
		"persistent_guidance":       persistentGuidance,
		"storyline_selection":       storylineSelectionTrace,
		"final_guidance_suffix":     finalGuidance,
		"momentum_packet": map[string]any{
			"packet_status":   status,
			"evidence_counts": evidenceCounts,
			"section_summary": sectionSummary,
		},
		"prompt_plan": []string{
			"supervisor_system.txt",
			"supervisor_prompt.txt",
			"persistent_guidance",
			"recent_context_summary",
			"wake_up_or_continuity_context",
		},
		"degraded":        degraded,
		"fallback_reason": fallbackReason,
		"would_call_llm":  false,
		"would_write":     false,
	}
}

func buildPrepareTurnPlannerLanguageContract(languageContext map[string]any) map[string]any {
	target := prepareTurnSessionOutputLanguage(languageContext)
	status := "unknown"
	if target != "" && target != "auto" && target != "unknown" {
		status = "ready"
	}
	return map[string]any{
		"contract_version":              languageMemoryContractVersion,
		"status":                        status,
		"planner_support_language":      nilIfEmpty(target),
		"planner_language_source":       nilIfEmpty(extractionStringFromAny(languageContext["output_language_source"])),
		"current_user_input_priority":   "highest",
		"raw_user_input_rewritten":      false,
		"raw_evidence_rewritten":        false,
		"generated_support_policy":      "use_session_output_language_when_language_is_known",
		"trace_labels_language_neutral": true,
	}
}

func buildResponseExecutionSourceRules(currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1) map[string]any {
	return buildResponseExecutionSourceRulesWithMemory(currentInput, hostEvidence, "", nil)
}

func buildResponseExecutionSourceRulesWithMemory(currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1, sessionID string, memoryDeliveryLineage map[string]any) map[string]any {
	currentInputRefs := []string{}
	if currentInput.SelectedObservationRef != nil && strings.TrimSpace(*currentInput.SelectedObservationRef) != "" {
		currentInputRefs = append(currentInputRefs, strings.TrimSpace(*currentInput.SelectedObservationRef))
	} else if currentInput.Envelope != nil && strings.TrimSpace(currentInput.Envelope.ObservationRef) != "" {
		currentInputRefs = append(currentInputRefs, strings.TrimSpace(currentInput.Envelope.ObservationRef))
	}

	hostRefs := make([]string, 0, len(hostEvidence.Items))
	for _, item := range hostEvidence.Items {
		if ref := strings.TrimSpace(item.EvidenceRef); ref != "" {
			hostRefs = appendUniqueMemorySearchText(hostRefs, ref)
		}
	}
	allRefs := append([]string{}, currentInputRefs...)
	for _, ref := range hostRefs {
		allRefs = appendUniqueMemorySearchText(allRefs, ref)
	}
	memoryRefs := deliveredPrepareTurnMemorySourceRefs(sessionID, memoryDeliveryLineage)
	for _, ref := range memoryRefs {
		allRefs = appendUniqueMemorySearchText(allRefs, ref)
	}

	mustPreserve := []map[string]any{}
	if len(hostRefs) > 0 {
		mustPreserve = append(mustPreserve, map[string]any{
			"instruction":    "Preserve every native system constraint already present in this request; use the linked spans in place and do not restate their source text.",
			"source_refs":    hostRefs,
			"native_present": true,
		})
	}
	if len(memoryRefs) > 0 {
		mustPreserve = append(mustPreserve, map[string]any{
			"instruction": "Preserve the continuity facts carried by the delivered long-term-memory items, subject to current evidence and protected-knowledge boundaries.",
			"source_refs": memoryRefs,
		})
	}
	mustRespond := []map[string]any{}
	mustAccount := []map[string]any{}
	if len(currentInputRefs) > 0 {
		mustRespond = append(mustRespond, map[string]any{
			"instruction": "Respond directly to the latest observed user input without rewriting or replacing it.",
			"source_refs": currentInputRefs,
		})
		mustAccount = append(mustAccount, map[string]any{
			"instruction": "Account for effects explicitly established by the latest observed user input; do not invent an unstated effect.",
			"source_refs": currentInputRefs,
		})
	}
	mustNotAssert := []map[string]any{}
	if len(allRefs) > 0 {
		mustNotAssert = append(mustNotAssert, map[string]any{
			"instruction": "Do not assert unsupported facts, hidden knowledge, user decisions, relationship changes, or final closure beyond the linked request evidence.",
			"source_refs": allRefs,
		})
	}

	return map[string]any{
		"must_preserve": map[string]any{"items": mustPreserve, "count": len(mustPreserve)},
		"must_respond":  map[string]any{"items": mustRespond, "count": len(mustRespond)},
		"must_account":  map[string]any{"items": mustAccount, "count": len(mustAccount)},
		"must_not_assert": map[string]any{
			"items": mustNotAssert,
			"count": len(mustNotAssert),
		},
		"source_refs": map[string]any{
			"current_input": currentInputRefs,
			"native_system": hostRefs,
			"memory":        memoryRefs,
			"all":           allRefs,
		},
	}
}

func buildResponseExecutionContract(inputAnchorGovernor map[string]any, selectedStorylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, assembly prepareTurnInjectionAssembly, languageContext map[string]any, currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1) map[string]any {
	return buildResponseExecutionContractWithMemoryLineage("", inputAnchorGovernor, selectedStorylines, pendingThreads, activeStates, canonicalLayers, worldRules, assembly, languageContext, currentInput, hostEvidence)
}

func buildPrepareTurnGuideEligibility(guideMode, guideStrength string, injectionEnabled bool, narrativeSupportMaxChars int, responseExecutionContract map[string]any) map[string]any {
	mode := normalizeNarrativeGuideMode(guideMode)
	strength := normalizeNarrativeGuideStrength(guideStrength)
	sourceRefs := mapFromAny(responseExecutionContract["source_refs"])
	eligibleRefs := []string{}
	for _, ref := range stringSliceFromAny(sourceRefs["memory"]) {
		eligibleRefs = appendUniqueMemorySearchText(eligibleRefs, ref)
	}

	status := "eligible"
	reason := "source_backed_guide_support_available"
	switch {
	case mode == "off" || strength == "none":
		status = "off"
		reason = "narrative_guide_disabled"
		eligibleRefs = []string{}
	case !injectionEnabled:
		status = "injection_disabled"
		reason = "payload_injection_disabled"
		eligibleRefs = []string{}
	case narrativeSupportMaxChars <= 0:
		status = "budget_disabled"
		reason = "narrative_support_budget_zero"
		eligibleRefs = []string{}
	case len(eligibleRefs) == 0:
		status = "no_support"
		reason = "no_source_backed_guide_support"
	}

	return map[string]any{
		"contract_version":                  "guide_eligibility.v1",
		"status":                            status,
		"reason_code":                       reason,
		"guide_mode":                        mode,
		"guide_strength":                    strength,
		"source_refs":                       eligibleRefs,
		"source_ref_count":                  len(eligibleRefs),
		"current_input_only_is_not_support": true,
		"native_system_only_is_not_support": true,
		"truth_authority":                   false,
		"would_write":                       false,
		"coverage":                          supervisorProposalCoverage(strength),
	}
}

func buildResponseExecutionContractWithMemoryLineage(sessionID string, inputAnchorGovernor map[string]any, selectedStorylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, assembly prepareTurnInjectionAssembly, languageContext map[string]any, currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1) map[string]any {
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])

	protectedCount := intFromAny(assembly.Counts["protected_secret_count"], 0) +
		intFromAny(assembly.Counts["identity_accuracy_count"], 0) +
		intFromAny(assembly.Counts["protected_memory_guarded_count"], 0)
	privateLaneActive := intFromAny(assembly.Counts["character_private_recollection_bound"], intFromAny(assembly.Counts["character_private_recollection_count"], 0)) > 0 ||
		strings.TrimSpace(assembly.CharacterPrivateText) != ""

	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)
	sourceRules := buildResponseExecutionSourceRulesWithMemory(currentInput, hostEvidence, sessionID, assembly.MemoryDeliveryLineage)

	return map[string]any{
		"contract_version":            "response_execution_contract.v1",
		"status":                      "ready",
		"active":                      true,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"planner_support_language":    nilIfEmpty(targetLanguage),
		"consume_rule": map[string]any{
			"allowed_usage":  []string{"continuity_guard", "secret_leak_guard"},
			"blocked_usage":  []string{"truth_write", "canonical_override", "user_intent_override", "raw_memory_dump", "hidden_knowledge_reveal"},
			"priority_order": []string{"current_user_input", "explicit_user_correction", "direct_evidence", "canonical_state", "retrieved_support", "response_execution_contract"},
		},
		"read_surface_alignment": map[string]any{
			"selected_anchor_count":       len(selectedAnchors),
			"dropped_anchor_count":        len(droppedAnchors),
			"selected_storyline_count":    len(selectedStorylines),
			"pending_thread_count":        len(pendingThreads),
			"active_state_count":          len(activeStates),
			"canonical_layer_count":       len(canonicalLayers),
			"world_rule_count":            len(worldRules),
			"protected_signal_count":      protectedCount,
			"private_recollection_active": privateLaneActive,
		},
		"facet_audit_repair_ingestion": map[string]any{
			"status": "no_prior_facet_audit_surface",
			"rule":   "when prior drift or secret-leak audit exists, consume only as bounded repair hint for the next turn",
		},
		"concealment_guard": map[string]any{
			"active": protectedCount > 0 || privateLaneActive,
			"rule":   "preserve protected/private knowledge boundaries; do not reveal or externalize without current-scene evidence",
		},
		"must_preserve":   sourceRules["must_preserve"],
		"must_respond":    sourceRules["must_respond"],
		"must_account":    sourceRules["must_account"],
		"must_not_assert": sourceRules["must_not_assert"],
		"source_refs":     sourceRules["source_refs"],
		"host_context_observation": map[string]any{
			"contract_version": hostEvidence.ContractVersion,
			"status":           hostEvidence.Status,
			"reason_code":      hostEvidence.ReasonCode,
			"selected_count":   hostEvidence.SelectedCount,
			"duplicate_count":  hostEvidence.DuplicateCount,
			"deferred_count":   hostEvidence.DeferredCount,
		},
	}
}

func responseExecutionRuleSourceRefs(contract map[string]any, keys ...string) []string {
	refs := []string{}
	for _, key := range keys {
		for _, raw := range outputFidelityLineageSlice(mapFromAny(contract[key])["items"]) {
			for _, ref := range stringSliceFromAny(mapFromAny(raw)["source_refs"]) {
				refs = appendUniqueMemorySearchText(refs, ref)
			}
		}
	}
	return refs
}

func formatResponseExecutionRuleGuidance(contract map[string]any, heading string, keys []string) string {
	if contract == nil || !boolFromAny(contract["active"]) {
		return ""
	}
	lines := []string{
		heading,
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
	}
	for _, key := range keys {
		for _, raw := range outputFidelityLineageSlice(mapFromAny(contract[key])["items"]) {
			item := mapFromAny(raw)
			instruction := strings.TrimSpace(extractionStringFromAny(item["instruction"]))
			refs := stringSliceFromAny(item["source_refs"])
			if instruction == "" || len(refs) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s=%s; source_refs:%s", key, instruction, strings.Join(refs, ",")))
		}
	}
	if len(lines) == 2 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func formatResponseExecutionFidelityGuidance(contract map[string]any) string {
	return formatResponseExecutionRuleGuidance(
		contract,
		"[Source-backed Fidelity Preservation]",
		[]string{"must_preserve", "must_not_assert"},
	)
}

func normalizeNarrativeGuideMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return "auto"
	case "standard":
		return "standard"
	case "romantic":
		return "romantic"
	case "action":
		return "action"
	case "mature_soft", "mature-soft":
		return "mature_soft"
	case "mature_direct", "mature-direct":
		return "mature_direct"
	default:
		return "off"
	}
}

func resolveNarrativeGuideMode(mode string, contextMessages []map[string]any, wakeUpContext, fallbackUserInput string) string {
	normalized := normalizeNarrativeGuideMode(mode)
	if normalized != "auto" {
		return normalized
	}
	probe := strings.Join(nonEmptyStrings([]string{fallbackUserInput, latestUserMessageText(contextMessages), wakeUpContext}), "\n")
	return inferNarrativeGuideModeFromText(probe)
}

func latestUserMessageText(contextMessages []map[string]any) string {
	for i := len(contextMessages) - 1; i >= 0; i-- {
		msg := contextMessages[i]
		if strings.ToLower(strings.TrimSpace(extractionStringFromAny(msg["role"]))) != "user" {
			continue
		}
		content := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if content != "" {
			return content
		}
	}
	return ""
}

func inferNarrativeGuideModeFromText(text string) string {
	source := strings.ToLower(strings.TrimSpace(text))
	if source == "" {
		return "standard"
	}
	if containsAnyText(source, "r18", "r 18", "explicit", "direct sensual", "mature direct", "adult direct") {
		return "mature_direct"
	}
	if containsAnyText(source, "sensual", "mature", "adult romance", "soft mature", "intimate") {
		return "mature_soft"
	}
	if containsAnyText(source, "romance", "romantic", "love", "date", "crush", "kiss") {
		return "romantic"
	}
	if containsAnyText(source, "action", "battle", "fight", "combat", "mission", "chase", "duel") {
		return "action"
	}
	return "standard"
}

func containsAnyText(source string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(source, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeNarrativeGuideStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return "none"
	case "medium", "strong":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "weak"
	}
}

func buildNarrativeGuideFocus(mode string) []string {
	switch normalizeNarrativeGuideMode(mode) {
	case "standard":
		return []string{"coherent continuity", "consistent character voice"}
	case "romantic":
		return []string{"emotional nuance", "relationship-aware subtext"}
	case "action":
		return []string{"clear cause and effect", "grounded physical consequences"}
	case "mature_soft":
		return []string{"sensory atmosphere", "emotional and interpersonal nuance"}
	case "mature_direct":
		return []string{"direct description when already supported", "character agency and emotional context"}
	default:
		return []string{}
	}
}

func stringSliceFromAny(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func limitStringSlice(items []string, limit int) []string {
	if limit < 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func buildCriticInputPack(chatSessionID string, turnIndex int, rawUserInput string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, degraded bool) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":              status,
		"source":              "go_r1_read_shadow",
		"chat_session_id":     chatSessionID,
		"turn_index":          turnIndex,
		"turn_content_chars":  len([]rune(rawUserInput)),
		"prompt_assembly":     promptAssembly,
		"prompt_source":       promptAssembly["prompt_source"],
		"evidence_counts":     evidenceCounts,
		"section_summary":     sectionSummary,
		"output_contract":     []string{"memories", "direct_evidence", "kg_triples", "critic_feedback"},
		"critic_context_plan": []string{"turn_content", "recent_chat", "direct_evidence", "kg_triples", "supervisor_input_pack"},
		"verdict":             "not_executed",
		"would_call_llm":      false,
		"would_write":         false,
		"degraded":            degraded,
	}
}
