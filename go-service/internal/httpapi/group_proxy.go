package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

var proxyHTTPClient = http.DefaultClient

type llmRetryBudget struct {
	mu        sync.Mutex
	remaining int
}

func newLLMRetryBudget(retries int) *llmRetryBudget {
	if retries < 0 {
		retries = 0
	}
	return &llmRetryBudget{remaining: retries}
}

func (b *llmRetryBudget) take() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}

// registerProxyRoutes mounts supervisor, proxy plugin, and critic endpoints.
func (s *Server) registerProxyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /supervisor", s.handleSupervisor)
	mux.HandleFunc("POST /proxy/plugin-main", s.handleProxyPluginMain)
	mux.HandleFunc("POST /critic/test", s.handleCriticTest)
}

func (s *Server) handleSupervisor(w http.ResponseWriter, r *http.Request) {
	var req dto.SupervisorContractRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(*req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	guideMode := resolveNarrativeGuideMode(stringPtrValue(req.GuideMode, "off"), req.ContextMessages, stringPtrValue(req.WakeUpContext, ""), "")
	guideStrength := normalizeNarrativeGuideStrength(stringPtrValue(req.GuideStrength, "weak"))
	wakeUpContext := stringPtrValue(req.WakeUpContext, "")
	persistentGuidance := stringPtrValue(req.PersistentGuidance, "")
	promptTrace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	storylineSelection := storylineSupervisorSelection{}
	evidenceCounts := map[string]any{
		"context_messages":            len(req.ContextMessages),
		"wake_up_context_present":     wakeUpContext != "",
		"persistent_guidance_present": persistentGuidance != "",
	}
	sectionSummary := []map[string]any{
		{
			"name":      "supervisor_request_context",
			"chars":     len([]rune(wakeUpContext)) + len([]rune(persistentGuidance)),
			"available": wakeUpContext != "" || persistentGuidance != "" || len(req.ContextMessages) > 0,
			"truncated": false,
			"sources":   []string{"context_messages", "wake_up_context", "persistent_guidance"},
		},
	}
	currentInput := latestUserMessageText(req.ContextMessages)
	supervisorPack := buildSupervisorInputPack(sid, 0, currentInput, guideMode, guideStrength, "", "", "", promptTrace, evidenceCounts, sectionSummary, storylineSelection, false, "", nil)
	if len(req.ResponseExecutionContract) > 0 {
		supervisorPack["response_execution_contract"] = req.ResponseExecutionContract
	}
	supervisorPack["support_packet"] = buildSupervisorSupportPacket(sid, currentInput, mapFromAny(supervisorPack["response_execution_contract"]), nil)
	trace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	trace["guide_mode"] = guideMode
	trace["guide_strength"] = guideStrength
	trace["supervisor_proposal_coverage"] = supervisorProposalCoverage(guideStrength)
	trace["response_execution_contract_present"] = len(req.ResponseExecutionContract) > 0
	trace["guide_focus"] = supervisorPack["guide_focus"]
	trace["wake_up_context_present"] = wakeUpContext != ""
	trace["persistent_guidance_present"] = persistentGuidance != ""
	trace["context_messages_count"] = len(req.ContextMessages)
	trace["would_call_llm"] = false
	trace["would_write"] = false
	llmCfg := s.supervisorLLMConfig()
	if llmCfg.hasConfig() {
		if guideMode == "off" || guideStrength == "none" {
			result, proposalTrace := buildBoundedSupervisorResult(nil, supervisorPack)
			trace["llm_call"] = "skipped"
			trace["reason_code"] = "narrative_guide_disabled"
			trace["proposal_contract"] = proposalTrace
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                "ok",
				"source":                "guide_eligibility_gate",
				"note":                  "POST /supervisor skipped the LLM because narrative guidance is disabled",
				"chat_session_id":       sid,
				"supervisor_input_pack": supervisorPack,
				"would_call_llm":        false,
				"would_write":           false,
				"upstream_write":        "disabled",
				"supervisor_result":     result,
				"trace_summary":         trace,
			})
			return
		}
		if ready, reasonCode := supervisorExecutionContractReady(supervisorPack); !ready {
			result, proposalTrace := buildBoundedSupervisorResult(nil, supervisorPack)
			trace["llm_call"] = "skipped"
			trace["fail_open"] = true
			trace["reason_code"] = reasonCode
			trace["proposal_contract"] = proposalTrace
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                "partial",
				"source":                "execution_contract_gate",
				"note":                  "POST /supervisor skipped the LLM because no ready source-backed execution contract was available",
				"chat_session_id":       sid,
				"supervisor_input_pack": supervisorPack,
				"would_call_llm":        false,
				"would_write":           false,
				"upstream_write":        "disabled",
				"supervisor_result":     result,
				"fail_open":             true,
				"trace_summary":         trace,
			})
			return
		}
		result, llmTrace, err := s.runSupervisorLLM(r.Context(), sid, supervisorPack, llmCfg)
		trace["would_call_llm"] = true
		trace["llm_call"] = "executed"
		trace["llm_trace"] = llmTrace
		if err != nil {
			trace["llm_call"] = "failed"
			trace["fail_open"] = true
			trace["reason_code"] = "supervisor_provider_failed_open"
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                "partial",
				"source":                "runtime_llm_error",
				"note":                  "POST /supervisor attempted configured LLM call and failed open",
				"chat_session_id":       sid,
				"supervisor_input_pack": supervisorPack,
				"would_call_llm":        true,
				"would_write":           false,
				"upstream_write":        "disabled",
				"supervisor_result":     nil,
				"fail_open":             true,
				"reason_code":           "supervisor_provider_failed_open",
				"trace_summary":         trace,
			})
			return
		}
		responseStatus := "ok"
		responseSource := "runtime_llm"
		failOpen := false
		reasonCode := ""
		resultProposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
		if extractionStringFromAny(resultProposal["status"]) == "malformed_failed_open" {
			responseStatus = "partial"
			responseSource = "runtime_llm_malformed"
			failOpen = true
			reasonCode = extractionStringFromAny(resultProposal["reason_code"])
			trace["fail_open"] = true
			trace["reason_code"] = reasonCode
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                responseStatus,
			"source":                responseSource,
			"note":                  "POST /supervisor used configured runtime LLM settings",
			"chat_session_id":       sid,
			"supervisor_input_pack": supervisorPack,
			"would_call_llm":        true,
			"would_write":           false,
			"upstream_write":        "disabled",
			"supervisor_result":     result,
			"fail_open":             failOpen,
			"reason_code":           nilIfEmpty(reasonCode),
			"trace_summary":         trace,
		})
		return
	}
	trace["llm_call"] = "not_configured"

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"source":                "shadow",
		"note":                  "POST /supervisor is an R1 read-only evidence surface; no LLM call executed",
		"chat_session_id":       sid,
		"supervisor_input_pack": supervisorPack,
		"would_call_llm":        false,
		"would_write":           false,
		"upstream_write":        "disabled",
		"trace_summary":         trace,
	})
}

func (s *Server) runSupervisorLLM(ctx context.Context, sid string, supervisorPack map[string]any, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	systemPrompt, promptSource := readSupervisorSystemPrompt(s.Cfg.PromptDir)
	guideMode := normalizeNarrativeGuideMode(extractionStringFromAny(supervisorPack["guide_mode"]))
	payload := map[string]any{
		"chat_session_id":              sid,
		"guide_mode":                   guideMode,
		"guide_strength":               extractionStringFromAny(supervisorPack["guide_strength"]),
		"guide_focus":                  supervisorPack["guide_focus"],
		"supervisor_support_packet":    supervisorPack["support_packet"],
		"response_execution_contract":  supervisorPack["response_execution_contract"],
		"supervisor_proposal_coverage": supervisorProposalCoverage(extractionStringFromAny(supervisorPack["guide_strength"])),
		"required_output": "Return only JSON with supervisor_scene_proposal. Use fidelity_warnings for delivered-memory fidelity and expression_hints with an allowed kind for optional expression support. " +
			"Copy exact refs from supervisor_support_packet, follow supervisor_proposal_coverage, and keep every item proposal-only without deciding facts, user actions, relationships, scene jumps, or event closure.",
	}
	userPromptBytes, _ := json.MarshalIndent(payload, "", "  ")
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	temp := cfg.Temperature
	reqBody := dto.ProxyPluginMainRequest{
		APIKey:      &cfg.APIKey,
		Endpoint:    &cfg.Endpoint,
		Model:       &cfg.Model,
		Provider:    &cfg.Provider,
		Messages:    []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": string(userPromptBytes)}},
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		TimeoutMs:   &cfg.TimeoutMs,
	}
	applyProxyOverridesFromLLMConfig(&reqBody, cfg)
	upstream, _, err := performProxyPluginMainWithRetryBudget(ctx, reqBody, cfg.RetryBudget)
	if err != nil {
		return nil, map[string]any{"prompt_source": promptSource, "model": cfg.Model}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	trace := map[string]any{
		"prompt_source": promptSource,
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	if err != nil {
		trace["parse_status"] = "malformed_failed_open"
		parsed = nil
	} else {
		trace["parse_status"] = "parsed"
	}
	bounded, proposalTrace := buildBoundedSupervisorResult(parsed, supervisorPack)
	trace["proposal_contract"] = proposalTrace
	return bounded, trace, nil
}

func supervisorProposalCoverage(strength string) map[string]any {
	common := map[string]any{
		"truth_authority":                        false,
		"canonical_write":                        false,
		"force_progress":                         false,
		"proactive_complication_opt_in":          false,
		"proactive_complication_requires_opt_in": true,
		"proactive_complication_default":         "off",
		"strong_implies_proactive":               false,
		"strong_implies_forced_progress":         false,
		"blocked_user_action":                    true,
		"blocked_new_truth":                      true,
		"blocked_relationship_change":            true,
		"blocked_unresolved_event_closure":       true,
	}
	withCommon := func(coverage map[string]any) map[string]any {
		for key, value := range common {
			coverage[key] = value
		}
		return coverage
	}
	switch normalizeNarrativeGuideStrength(strength) {
	case "none":
		return withCommon(map[string]any{
			"profile":                  "disabled",
			"supervisor_call":          "none",
			"guidance_scope":           "none",
			"allowed_roles":            []string{},
			"allowed_expression_kinds": []string{},
		})
	case "strong":
		return withCommon(map[string]any{
			"profile":         "fidelity_expression_reversible",
			"supervisor_call": "source_backed_optional",
			"guidance_scope":  "arc_anchor_and_preferred_frontier",
			"guidance_options": []string{
				"arc_anchor", "preferred_frontier", "hold_allowed",
			},
			"allowed_roles": []string{"fidelity_warning", "portrayal", "pacing", "scene_emphasis", "callback", "reversible_option"},
			"allowed_expression_kinds": []string{
				"portrayal", "response_focus", "must_account", "pacing", "scene_emphasis", "callback",
				"may_advance", "hold_allowed", "arc_anchor", "preferred_frontier", "reversible_option",
			},
		})
	case "medium":
		return withCommon(map[string]any{
			"profile":         "fidelity_expression_contextual",
			"supervisor_call": "source_backed_optional",
			"guidance_scope":  "may_advance_or_hold_allowed",
			"guidance_options": []string{
				"may_advance", "hold_allowed",
			},
			"allowed_roles": []string{"fidelity_warning", "portrayal", "pacing", "scene_emphasis", "callback"},
			"allowed_expression_kinds": []string{
				"portrayal", "response_focus", "must_account", "pacing", "scene_emphasis", "callback",
				"may_advance", "hold_allowed",
			},
		})
	default:
		return withCommon(map[string]any{
			"profile":                  "fidelity_expression_low_impact",
			"supervisor_call":          "source_backed_optional",
			"guidance_scope":           "response_focus_and_must_account",
			"guidance_options":         []string{"response_focus", "must_account"},
			"allowed_roles":            []string{"fidelity_warning", "portrayal"},
			"allowed_expression_kinds": []string{"portrayal", "response_focus", "must_account"},
		})
	}
}

func buildBoundedSupervisorResult(parsed, supervisorPack map[string]any) (map[string]any, map[string]any) {
	strength := normalizeNarrativeGuideStrength(extractionStringFromAny(supervisorPack["guide_strength"]))
	coverage := supervisorProposalCoverage(strength)
	executionContract := mapFromAny(supervisorPack["response_execution_contract"])

	sourceRefs := mapFromAny(executionContract["source_refs"])
	currentInputRefList, memoryRefList := supervisorSupportReferenceLists(supervisorPack)
	allowedRefList := appendUniqueStringValues([]string{}, currentInputRefList...)
	allowedRefList = appendUniqueStringValues(allowedRefList, memoryRefList...)
	allowedRefList = appendUniqueStringValues(allowedRefList, stringSliceFromAny(sourceRefs["native_system"])...)
	allowedRefs := make(map[string]struct{}, len(allowedRefList))
	for _, ref := range allowedRefList {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedRefs[ref] = struct{}{}
		}
	}
	memoryRefs := make(map[string]struct{}, len(memoryRefList))
	for _, ref := range memoryRefList {
		if ref = strings.TrimSpace(ref); ref != "" {
			memoryRefs[ref] = struct{}{}
		}
	}
	currentInputRefs := make(map[string]struct{}, len(currentInputRefList))
	for _, ref := range currentInputRefList {
		if ref = strings.TrimSpace(ref); ref != "" {
			currentInputRefs[ref] = struct{}{}
		}
	}
	expressionSupportRefs := make(map[string]struct{}, len(currentInputRefs)+len(memoryRefs))
	for ref := range currentInputRefs {
		expressionSupportRefs[ref] = struct{}{}
	}
	for ref := range memoryRefs {
		expressionSupportRefs[ref] = struct{}{}
	}
	contractReady, contractReasonCode := supervisorExecutionContractReady(supervisorPack)

	proposal := map[string]any{
		"contract_version":   "supervisor_scene_proposal.v3",
		"status":             "ready",
		"authority":          "proposal_only",
		"truth_authority":    false,
		"would_write":        false,
		"guide_strength":     strength,
		"coverage":           coverage,
		"verification_state": "lane_specific_source_support_required",
		"application_rule":   "Every item is optional support. Keep the current user input authoritative and do not treat a proposal as story truth or permission to decide irreversible outcomes.",
		"blocked_authority": []string{
			"new_fact",
			"new_emotion_or_knowledge",
			"relationship_change",
			"user_protagonist_action",
			"unresolved_event_closure",
			"scene_jump",
			"canonical_write",
		},
		"source_refs": map[string]any{
			"allowed":                  allowedRefList,
			"current_input_support":    currentInputRefList,
			"required_memory_evidence": memoryRefList,
		},
		"fidelity_warnings": []map[string]any{},
		"expression_hints":  []map[string]any{},
	}
	trace := map[string]any{
		"contract_ready": contractReady,
		"allowed_refs":   len(allowedRefs),
		"memory_refs":    len(memoryRefs),
		"guide_strength": strength,
		"coverage":       coverage,
	}
	rawGuideMode := strings.TrimSpace(extractionStringFromAny(supervisorPack["guide_mode"]))
	if strength == "none" || (rawGuideMode != "" && normalizeNarrativeGuideMode(rawGuideMode) == "off") {
		proposal["status"] = "disabled"
		proposal["reason_code"] = "narrative_guide_disabled"
		trace["reason_code"] = "narrative_guide_disabled"
		return boundedSupervisorEnvelope(proposal), trace
	}
	if !contractReady {
		proposal["status"] = "degraded_missing_execution_contract"
		proposal["reason_code"] = contractReasonCode
		trace["reason_code"] = contractReasonCode
		return boundedSupervisorEnvelope(proposal), trace
	}
	if parsed == nil {
		proposal["status"] = "malformed_failed_open"
		proposal["reason_code"] = "supervisor_malformed_json"
		trace["reason_code"] = "supervisor_malformed_json"
		trace["fail_open"] = true
		trace["accepted_items"] = 0
		trace["rejected_items"] = 0
		return boundedSupervisorEnvelope(proposal), trace
	}

	rawProposal := map[string]any{}
	proposalEnvelopePresent := false
	schemaInvalid := false
	if rawEnvelope, exists := parsed["supervisor_scene_proposal"]; exists {
		proposalEnvelopePresent = true
		var ok bool
		rawProposal, ok = rawEnvelope.(map[string]any)
		schemaInvalid = !ok
	} else if rawDirectiveValue, directiveExists := parsed["directive"]; directiveExists {
		rawDirective, ok := rawDirectiveValue.(map[string]any)
		if !ok {
			schemaInvalid = true
		} else if rawEnvelope, exists := rawDirective["supervisor_scene_proposal"]; exists {
			proposalEnvelopePresent = true
			rawProposal, ok = rawEnvelope.(map[string]any)
			schemaInvalid = !ok
		}
	}
	for _, key := range []string{"fidelity_warnings", "expression_hints", "portrayal_notes", "may_advance"} {
		rawItems, exists := rawProposal[key]
		if !exists {
			continue
		}
		items, ok := rawItems.([]any)
		if !ok {
			schemaInvalid = true
			break
		}
		if key == "fidelity_warnings" || key == "expression_hints" {
			for _, rawItem := range items {
				item, ok := rawItem.(map[string]any)
				if !ok || !supervisorProposalItemSchemaValid(item, key == "expression_hints") {
					schemaInvalid = true
					break
				}
			}
		}
		if schemaInvalid {
			break
		}
	}
	if schemaInvalid {
		proposal["status"] = "malformed_failed_open"
		proposal["reason_code"] = "supervisor_schema_invalid"
		trace["reason_code"] = "supervisor_schema_invalid"
		trace["fail_open"] = true
		trace["accepted_items"] = 0
		trace["rejected_items"] = 0
		return boundedSupervisorEnvelope(proposal), trace
	}
	allowedKinds := make(map[string]struct{})
	for _, kind := range stringSliceFromAny(coverage["allowed_expression_kinds"]) {
		allowedKinds[kind] = struct{}{}
	}

	acceptedTotal := 0
	rejectedTotal := 0
	fidelityItems, fidelityRejected := normalizeSupervisorProposalItems(rawProposal["fidelity_warnings"], allowedRefs, memoryRefs)
	proposal["fidelity_warnings"] = fidelityItems
	acceptedTotal += len(fidelityItems)
	rejectedTotal += fidelityRejected
	expressionItems, expressionRejected := normalizeSupervisorExpressionItems(rawProposal["expression_hints"], allowedKinds, allowedRefs, expressionSupportRefs, memoryRefs)
	proposal["expression_hints"] = expressionItems
	acceptedTotal += len(expressionItems)
	rejectedTotal += expressionRejected
	rejectedTotal += anySliceLength(rawProposal["portrayal_notes"])
	rejectedTotal += anySliceLength(rawProposal["may_advance"])
	for key := range rawProposal {
		switch key {
		case "contract_version", "fidelity_warnings", "expression_hints", "portrayal_notes", "may_advance":
			continue
		default:
			rejectedTotal++
		}
	}
	if acceptedTotal == 0 {
		switch {
		case rejectedTotal > 0 || (!proposalEnvelopePresent && len(parsed) > 0):
			proposal["status"] = "unsupported_rejected"
			proposal["reason_code"] = "supervisor_unsupported_proposal_rejected"
			trace["reason_code"] = "supervisor_unsupported_proposal_rejected"
		default:
			proposal["status"] = "valid_empty"
			proposal["reason_code"] = "supervisor_valid_empty"
			trace["reason_code"] = "supervisor_valid_empty"
		}
	}
	trace["accepted_items"] = acceptedTotal
	trace["rejected_items"] = rejectedTotal
	trace["raw_legacy_fields_discarded"] = len(rawProposal) == 0
	return boundedSupervisorEnvelope(proposal), trace
}

func supervisorProposalItemSchemaValid(item map[string]any, requireKind bool) bool {
	if rawText, exists := item["text"]; exists {
		if _, ok := rawText.(string); !ok {
			return false
		}
	}
	if rawRefs, exists := item["source_refs"]; exists {
		switch refs := rawRefs.(type) {
		case []any:
			for _, rawRef := range refs {
				if _, ok := rawRef.(string); !ok {
					return false
				}
			}
		case []string:
		default:
			return false
		}
	}
	if requireKind {
		if rawKind, exists := item["kind"]; exists {
			if _, ok := rawKind.(string); !ok {
				return false
			}
		}
	}
	return true
}

func supervisorExecutionContractReady(supervisorPack map[string]any) (bool, string) {
	executionContract := mapFromAny(supervisorPack["response_execution_contract"])
	if extractionStringFromAny(executionContract["contract_version"]) != "response_execution_contract.v1" ||
		extractionStringFromAny(executionContract["status"]) != "ready" ||
		!boolFromAny(executionContract["active"]) {
		return false, "supervisor_execution_contract_missing"
	}
	currentInputRefs, memoryRefs := supervisorSupportReferenceLists(supervisorPack)
	if len(currentInputRefs) > 0 || len(memoryRefs) > 0 {
		return true, ""
	}
	return false, "supervisor_support_packet_has_no_supported_lane"
}

func boundedSupervisorEnvelope(proposal map[string]any) map[string]any {
	return map[string]any{
		"contract_version": "supervisor_scene_proposal.v3",
		"authority":        "proposal_only",
		"truth_authority":  false,
		"would_write":      false,
		"directive": map[string]any{
			"supervisor_scene_proposal": proposal,
		},
	}
}

func normalizeSupervisorProposalItems(raw any, allowedRefs, memoryRefs map[string]struct{}) ([]map[string]any, int) {
	values, ok := raw.([]any)
	if !ok {
		return []map[string]any{}, anySliceLength(raw)
	}
	accepted := make([]map[string]any, 0, len(values))
	rejected := 0
	seenText := map[string]struct{}{}
	for _, value := range values {
		item := mapFromAny(value)
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		refs := stringSliceFromAny(item["source_refs"])
		if text == "" || len(refs) == 0 {
			rejected++
			continue
		}
		validRefs, valid := normalizeSupervisorItemRefs(refs, allowedRefs, memoryRefs)
		key := strings.ToLower(text)
		if !valid {
			rejected++
			continue
		}
		if _, duplicate := seenText[key]; duplicate {
			rejected++
			continue
		}
		seenText[key] = struct{}{}
		accepted = append(accepted, map[string]any{
			"text":               text,
			"source_refs":        validRefs,
			"verification_state": "delivered_memory_linked_proposal",
		})
	}
	return accepted, rejected
}

func normalizeSupervisorExpressionItems(raw any, allowedKinds, allowedRefs, expressionSupportRefs, memoryRefs map[string]struct{}) ([]map[string]any, int) {
	values, ok := raw.([]any)
	if !ok {
		return []map[string]any{}, anySliceLength(raw)
	}
	accepted := make([]map[string]any, 0, len(values))
	rejected := 0
	seen := map[string]struct{}{}
	for _, value := range values {
		item := mapFromAny(value)
		kind := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item["kind"])))
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		refs := stringSliceFromAny(item["source_refs"])
		if _, allowed := allowedKinds[kind]; !allowed || text == "" || len(refs) == 0 {
			rejected++
			continue
		}
		requiredRefs := expressionSupportRefs
		verificationState := "current_input_or_delivered_memory_linked_proposal"
		if kind == "callback" {
			requiredRefs = memoryRefs
			verificationState = "delivered_memory_linked_callback"
		}
		validRefs, valid := normalizeSupervisorItemRefs(refs, allowedRefs, requiredRefs)
		if !valid {
			rejected++
			continue
		}
		key := kind + "\x1f" + strings.ToLower(text)
		if _, duplicate := seen[key]; duplicate {
			rejected++
			continue
		}
		seen[key] = struct{}{}
		accepted = append(accepted, map[string]any{
			"kind":               kind,
			"text":               text,
			"source_refs":        validRefs,
			"verification_state": verificationState,
		})
	}
	return accepted, rejected
}

func normalizeSupervisorItemRefs(refs []string, allowedRefs, requiredSupportRefs map[string]struct{}) ([]string, bool) {
	validRefs := make([]string, 0, len(refs))
	hasRequiredSupport := false
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if _, exists := allowedRefs[ref]; !exists {
			return nil, false
		}
		if _, exists := requiredSupportRefs[ref]; exists {
			hasRequiredSupport = true
		}
		validRefs = appendUniqueStringValues(validRefs, ref)
	}
	return validRefs, len(validRefs) > 0 && hasRequiredSupport
}

func supervisorSupportReferenceLists(supervisorPack map[string]any) ([]string, []string) {
	executionRefs := mapFromAny(mapFromAny(supervisorPack["response_execution_contract"])["source_refs"])
	allowedCurrent := make(map[string]struct{})
	for _, ref := range stringSliceFromAny(executionRefs["current_input"]) {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedCurrent[ref] = struct{}{}
		}
	}
	allowedMemory := make(map[string]struct{})
	for _, ref := range stringSliceFromAny(executionRefs["memory"]) {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedMemory[ref] = struct{}{}
		}
	}

	supportPacket := mapFromAny(supervisorPack["support_packet"])
	currentRefs := []string{}
	currentInput := mapFromAny(supportPacket["current_input"])
	currentRef := strings.TrimSpace(extractionStringFromAny(currentInput["source_ref"]))
	if strings.TrimSpace(extractionStringFromAny(currentInput["raw_text"])) != "" {
		if _, allowed := allowedCurrent[currentRef]; allowed {
			currentRefs = append(currentRefs, currentRef)
		}
	}
	memoryRefs := []string{}
	for _, raw := range outputFidelityLineageSlice(supportPacket["delivered_memory"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		if _, allowed := allowedMemory[ref]; allowed {
			memoryRefs = appendUniqueStringValues(memoryRefs, ref)
		}
	}
	return currentRefs, memoryRefs
}

func appendUniqueStringValues(base []string, values ...string) []string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		found := false
		for _, current := range base {
			if current == value {
				found = true
				break
			}
		}
		if !found {
			base = append(base, value)
		}
	}
	return base
}

func anySliceLength(value any) int {
	if values, ok := value.([]any); ok {
		return len(values)
	}
	return 0
}

func formatMomentumSuffix(packet *map[string]any) string {
	if packet == nil || len(*packet) == 0 {
		return ""
	}
	status := strings.TrimSpace(stringFromAny((*packet)["packet_status"]))
	if status != "ready" && status != "partial" {
		return ""
	}
	return "[Story Momentum Packet]\n" + compactJSONForShadow(*packet, 1000)
}

// handleProxyPluginMain validates the DTO and endpoint, then performs the
// bounded upstream call used by the RisuAI JS bridge.
func (s *Server) handleProxyPluginMain(w http.ResponseWriter, r *http.Request) {
	var req dto.ProxyPluginMainRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	endpoint := strings.TrimSpace(*req.Endpoint)
	if endpoint == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "endpoint is required")
		return
	}

	if err := ValidateProxyEndpointForProvider(endpoint, stringPtrValue(req.Provider, "")); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", err.Error())
		return
	}

	resp, status, err := performProxyPluginMainWithRetryBudget(
		r.Context(),
		req,
		newLLMRetryBudget(s.runtimeConfigSnapshot().LLMRetryCount),
	)
	if err != nil {
		code := "upstream_error"
		upstreamCallEnabled := true
		if status == http.StatusBadRequest {
			code = "config_error"
			upstreamCallEnabled = false
		}
		writeJSON(w, status, map[string]any{
			"status":                "error",
			"code":                  code,
			"source":                "proxy",
			"error":                 scrubProxySecret(err.Error(), stringPtrValue(req.APIKey, "")),
			"endpoint_validated":    true,
			"upstream_call_enabled": upstreamCallEnabled,
		})
		return
	}

	if resp == nil {
		resp = map[string]any{}
	}
	resp["endpoint_validated"] = true
	resp["upstream_call_enabled"] = true
	writeJSON(w, http.StatusOK, resp)
}

func performProxyPluginMain(ctx context.Context, req dto.ProxyPluginMainRequest) (map[string]any, int, error) {
	return performProxyPluginMainWithRetryBudget(ctx, req, nil)
}

func performProxyPluginMainWithPolicy(ctx context.Context, req dto.ProxyPluginMainRequest, policy proxyRequestPolicy) (map[string]any, int, error) {
	return performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, nil, policy)
}

func performProxyPluginMainWithRetryBudget(ctx context.Context, req dto.ProxyPluginMainRequest, retryBudget *llmRetryBudget) (map[string]any, int, error) {
	return callProxyProviderWithPolicy(ctx, req, proxyRequestPolicy{}, retryBudget)
}

func performProxyPluginMainWithRetryBudgetAndPolicy(ctx context.Context, req dto.ProxyPluginMainRequest, retryBudget *llmRetryBudget, policy proxyRequestPolicy) (map[string]any, int, error) {
	return callProxyProviderWithPolicy(ctx, req, policy, retryBudget)
}

func scrubProxySecret(text, apiKey string) string {
	out := text
	if strings.TrimSpace(apiKey) != "" {
		out = strings.ReplaceAll(out, strings.TrimSpace(apiKey), "[redacted]")
	}
	replacers := []string{"Authorization", "Bearer", "api_key", "api-key", "password", "secret"}
	for _, token := range replacers {
		out = strings.ReplaceAll(out, token, "[redacted]")
		out = strings.ReplaceAll(out, strings.ToLower(token), "[redacted]")
	}
	return out
}

func int64Value(v *int64, fallback int64) int64 {
	if v == nil {
		return fallback
	}
	return *v
}

func floatPtrValue(v *float64, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *Server) handleCriticTest(w http.ResponseWriter, r *http.Request) {
	var req dto.CriticTestRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	chatSessionID := ""
	if req.ChatSessionID != nil {
		chatSessionID = *req.ChatSessionID
	}

	contextCount := len(req.Context)
	outputLanguageOverridePresent := req.OutputLanguageOverride != nil

	promptTrace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	evidenceCounts := map[string]any{
		"context_messages":                 contextCount,
		"output_language_override_present": outputLanguageOverridePresent,
	}
	sectionSummary := []map[string]any{
		{
			"name":      "critic_turn_content",
			"chars":     len([]rune(req.TurnContent)),
			"available": strings.TrimSpace(req.TurnContent) != "",
			"truncated": false,
			"sources":   []string{"turn_content", "context"},
		},
	}
	criticPack := buildCriticInputPack(chatSessionID, req.TurnIndex, req.TurnContent, promptTrace, evidenceCounts, sectionSummary, false)
	traceSummary := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	traceSummary["turn_content_chars"] = len([]rune(req.TurnContent))
	traceSummary["context_count"] = contextCount
	traceSummary["output_language_override_present"] = outputLanguageOverridePresent
	traceSummary["llm_call"] = "disabled"
	traceSummary["verdict"] = "not_executed"

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                           "ok",
		"source":                           "shadow",
		"note":                             "critic/test is an R1 read-only evidence surface; no LLM call executed",
		"chat_session_id":                  chatSessionID,
		"turn_index":                       req.TurnIndex,
		"turn_content_chars":               len([]rune(req.TurnContent)),
		"context_count":                    contextCount,
		"output_language_override_present": outputLanguageOverridePresent,
		"critic_input_pack":                criticPack,
		"llm_call_enabled":                 false,
		"would_write":                      false,
		"verdict":                          "not_executed",
		"trace_summary":                    traceSummary,
	})
}
