package httpapi

import (
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

func buildSupervisorInputPack(chatSessionID string, turnIndex int, rawUserInput, guideMode, guideStrength, _, _, _ string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, storylineSelection storylineSupervisorSelection, degraded bool, fallbackReason string, languageContext map[string]any) map[string]any {
	guideMode = resolveNarrativeGuideMode(guideMode, nil, "", rawUserInput)
	guideStrength = normalizeNarrativeGuideStrength(guideStrength)
	guideFocus := buildNarrativeGuideFocus(guideMode)
	storylineSelectionTrace := storylineSelectionSummary(storylineSelection)
	plannerLanguageContract := buildPrepareTurnPlannerLanguageContract(languageContext)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":                    status,
		"source":                    "go_supervisor_support_planner",
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
		"storyline_selection":       storylineSelectionTrace,
		"momentum_packet": map[string]any{
			"packet_status":   status,
			"evidence_counts": evidenceCounts,
			"section_summary": sectionSummary,
		},
		"prompt_plan": []string{
			"supervisor_system.txt",
			"supervisor_support_packet",
			"response_execution_contract",
		},
		"degraded":        degraded,
		"fallback_reason": fallbackReason,
		"would_call_llm":  false,
		"would_write":     false,
	}
}

const supervisorAcceptedRecentContextSourceRef = "input-context:previous-completed-turn"

func buildSupervisorSupportPacket(chatSessionID, rawUserInput string, responseExecutionContract, memoryDeliveryLineage map[string]any, inputContextText string, characterMemorySupport map[string]any) map[string]any {
	sourceRefs := mapFromAny(responseExecutionContract["source_refs"])
	currentInputRefs := stringSliceFromAny(sourceRefs["current_input"])
	memoryRefList := stringSliceFromAny(sourceRefs["memory"])
	allowedMemoryRefs := make(map[string]struct{}, len(memoryRefList))
	for _, ref := range memoryRefList {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedMemoryRefs[ref] = struct{}{}
		}
	}
	allowedCharacterRefs := map[string]bool{}
	for _, ref := range stringSliceFromAny(sourceRefs["character_memory"]) {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedCharacterRefs[ref] = true
		}
	}

	var currentInput any
	if text := strings.TrimSpace(rawUserInput); text != "" {
		for _, ref := range currentInputRefs {
			if ref = strings.TrimSpace(ref); ref != "" {
				currentInput = map[string]any{
					"source_ref":          ref,
					"raw_text":            rawUserInput,
					"visibility_boundary": "current_request",
				}
				break
			}
		}
	}

	deliveredMemory := []map[string]any{}
	seenMemoryRefs := map[string]struct{}{}
	for _, raw := range outputFidelityLineageSlice(memoryDeliveryLineage["items"]) {
		item := mapFromAny(raw)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		finalText := extractionStringFromAny(item["final_text"])
		if strings.TrimSpace(finalText) == "" {
			continue
		}
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" {
			ref = prepareTurnMemoryLineageSourceRef(chatSessionID, item["source_row_id"])
		}
		if _, allowed := allowedMemoryRefs[ref]; !allowed {
			continue
		}
		if _, duplicate := seenMemoryRefs[ref]; duplicate {
			continue
		}
		seenMemoryRefs[ref] = struct{}{}
		protected := boolFromAny(item["protected_guard"])
		visibilityBoundary := "delivered_to_main_model"
		if protected {
			visibilityBoundary = "rendered_protection_guard_only"
		}
		deliveredMemory = append(deliveredMemory, map[string]any{
			"source_ref":          ref,
			"final_text":          finalText,
			"protected_guard":     protected,
			"visibility_boundary": visibilityBoundary,
		})
	}
	deliveredCharacterMemory := []map[string]any{}
	seenCharacterRefs := map[string]bool{}
	for _, raw := range outputFidelityLineageSlice(characterMemorySupport["delivered_items"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		finalText := strings.TrimSpace(extractionStringFromAny(item["text"]))
		if !boolFromAny(item["delivered"]) || ref == "" || finalText == "" || !allowedCharacterRefs[ref] || seenCharacterRefs[ref] {
			continue
		}
		seenCharacterRefs[ref] = true
		deliveredCharacterMemory = append(deliveredCharacterMemory, map[string]any{
			"source_ref":          ref,
			"final_text":          finalText,
			"class":               extractionStringFromAny(item["class"]),
			"kind":                extractionStringFromAny(item["kind"]),
			"privacy_guard":       item["privacy_guard"],
			"visibility_boundary": "delivered_projection_text_only",
		})
	}
	acceptedRecentContext := []map[string]any{}
	allowedContinuityRefs := map[string]bool{}
	for _, ref := range stringSliceFromAny(sourceRefs["continuity"]) {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedContinuityRefs[ref] = true
		}
	}
	if text := strings.TrimSpace(inputContextText); text != "" && allowedContinuityRefs[supervisorAcceptedRecentContextSourceRef] {
		acceptedRecentContext = append(acceptedRecentContext, map[string]any{
			"source_ref":          supervisorAcceptedRecentContextSourceRef,
			"final_text":          text,
			"role":                "continuity",
			"authority":           "continuity_only",
			"visibility_boundary": "delivered_input_context",
		})
	}

	status := "empty"
	if currentInput != nil || len(acceptedRecentContext) > 0 || len(deliveredMemory) > 0 || len(deliveredCharacterMemory) > 0 {
		status = "ready"
	}
	return map[string]any{
		"contract_version":                 "supervisor_support_packet.v1",
		"status":                           status,
		"current_input":                    currentInput,
		"accepted_recent_context":          acceptedRecentContext,
		"accepted_recent_context_count":    len(acceptedRecentContext),
		"delivered_memory":                 deliveredMemory,
		"delivered_memory_count":           len(deliveredMemory),
		"delivered_character_memory":       deliveredCharacterMemory,
		"delivered_character_memory_count": len(deliveredCharacterMemory),
		"undelivered_candidates_included":  false,
		"raw_private_memory_included":      false,
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
	return buildResponseExecutionSourceRulesWithMemory(currentInput, hostEvidence, "", nil, "", nil)
}

func buildResponseExecutionSourceRulesWithMemory(currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1, sessionID string, memoryDeliveryLineage map[string]any, inputContextText string, characterMemorySupport map[string]any) map[string]any {
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
	characterMemoryRefs := deliveredPrepareTurnCharacterMemorySourceRefs(characterMemorySupport)
	for _, ref := range characterMemoryRefs {
		memoryRefs = appendUniqueMemorySearchText(memoryRefs, ref)
	}
	for _, ref := range memoryRefs {
		allRefs = appendUniqueMemorySearchText(allRefs, ref)
	}
	continuityRefs := []string{}
	if strings.TrimSpace(inputContextText) != "" {
		continuityRefs = append(continuityRefs, supervisorAcceptedRecentContextSourceRef)
		allRefs = appendUniqueMemorySearchText(allRefs, supervisorAcceptedRecentContextSourceRef)
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
	if len(characterMemoryRefs) > 0 {
		mustPreserve = append(mustPreserve, map[string]any{
			"instruction": "Preserve delivered character profile, voice-behavior, and directional relationship projections as scoped support; do not convert them into objective truth or reveal guarded private facts.",
			"source_refs": characterMemoryRefs,
		})
	}
	if len(continuityRefs) > 0 {
		mustPreserve = append(mustPreserve, map[string]any{
			"instruction": "Use the accepted previous completed turn only as continuity context for the current response; it is not a new command source.",
			"source_refs": continuityRefs,
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
			"current_input":    currentInputRefs,
			"continuity":       continuityRefs,
			"native_system":    hostRefs,
			"memory":           memoryRefs,
			"character_memory": characterMemoryRefs,
			"all":              allRefs,
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
	currentInputRefs := []string{}
	memoryRefs := []string{}
	expressionRefs := []string{}
	for _, ref := range stringSliceFromAny(sourceRefs["current_input"]) {
		currentInputRefs = appendUniqueMemorySearchText(currentInputRefs, ref)
		expressionRefs = appendUniqueMemorySearchText(expressionRefs, ref)
	}
	for _, ref := range stringSliceFromAny(sourceRefs["memory"]) {
		memoryRefs = appendUniqueMemorySearchText(memoryRefs, ref)
		expressionRefs = appendUniqueMemorySearchText(expressionRefs, ref)
	}
	eligibleRefs := append([]string{}, expressionRefs...)

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
	case len(expressionRefs) == 0:
		status = "no_support"
		reason = "no_source_backed_guide_support"
	}
	lanesEnabled := status == "eligible"

	return map[string]any{
		"contract_version":                      "guide_eligibility.v2",
		"status":                                status,
		"reason_code":                           reason,
		"guide_mode":                            mode,
		"guide_strength":                        strength,
		"source_refs":                           eligibleRefs,
		"source_ref_count":                      len(eligibleRefs),
		"current_input_only_is_not_support":     false,
		"current_input_only_expression_support": len(currentInputRefs) > 0,
		"native_system_only_is_not_support":     true,
		"truth_authority":                       false,
		"would_write":                           false,
		"coverage":                              supervisorProposalCoverage(strength),
		"lanes": map[string]any{
			"fidelity": map[string]any{
				"eligible":    lanesEnabled && len(memoryRefs) > 0,
				"source_refs": memoryRefs,
			},
			"expression": map[string]any{
				"eligible":    lanesEnabled && len(expressionRefs) > 0,
				"source_refs": expressionRefs,
			},
		},
	}
}

func buildResponseExecutionContractWithMemoryLineage(sessionID string, inputAnchorGovernor map[string]any, selectedStorylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, assembly prepareTurnInjectionAssembly, languageContext map[string]any, currentInput dto.PrepareTurnCurrentInputDecisionV1, hostEvidence dto.PrepareTurnHostContextReferenceEvidenceV1, inputContextTextArg ...string) map[string]any {
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])

	protectedCount := intFromAny(assembly.Counts["protected_secret_count"], 0) +
		intFromAny(assembly.Counts["identity_accuracy_count"], 0) +
		intFromAny(assembly.Counts["protected_memory_guarded_count"], 0)
	privateLaneActive := intFromAny(assembly.Counts["character_private_recollection_bound"], intFromAny(assembly.Counts["character_private_recollection_count"], 0)) > 0 ||
		strings.TrimSpace(assembly.CharacterPrivateText) != ""

	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)
	inputContextText := ""
	if len(inputContextTextArg) > 0 {
		inputContextText = inputContextTextArg[0]
	}
	sourceRules := buildResponseExecutionSourceRulesWithMemory(currentInput, hostEvidence, sessionID, assembly.MemoryDeliveryLineage, inputContextText, assembly.CharacterMemorySupport)

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

func resolveNarrativeGuideMode(mode string, _ []map[string]any, _, _ string) string {
	normalized := normalizeNarrativeGuideMode(mode)
	if normalized != "auto" {
		return normalized
	}
	// Auto is a stable default, not a prose classifier. Inferring genre from
	// language-specific keywords made identical requests resolve differently
	// across languages and gave ordinary words hidden policy authority.
	return "standard"
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
