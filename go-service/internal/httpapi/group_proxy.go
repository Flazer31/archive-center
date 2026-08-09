package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	supervisorPack["support_packet"] = buildSupervisorSupportPacket(sid, currentInput, mapFromAny(supervisorPack["response_execution_contract"]), nil, "", nil)
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
			failureCode := extractionFirstNonEmpty(extractionStringFromAny(llmTrace["failure_code"]), "publisher_llm_failed_open")
			trace["llm_call"] = "failed"
			trace["fail_open"] = true
			trace["reason_code"] = failureCode
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
				"reason_code":           failureCode,
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
		"required_output": "Return only JSON with supervisor_scene_proposal. Translate the current input, accepted recent context, and delivered support into the bounded response focus, continuity, expression, pacing, and scene-direction kinds allowed by supervisor_proposal_coverage. " +
			"Copy exact refs from supervisor_support_packet and keep every item response-scoped and proposal-only without inventing facts, user actions, relationship changes, scene jumps, or event closure.",
	}
	userPromptBytes, _ := json.MarshalIndent(payload, "", "  ")
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	reqBody := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": string(userPromptBytes)}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		reqBody.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		reqBody.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		reqBody.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		reqBody.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		reqBody.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&reqBody, cfg)
	upstream, upstreamStatus, err := performProxyPluginMainWithRetryBudget(ctx, reqBody, cfg.RetryBudget)
	if err != nil {
		failureCode := "publisher_llm_provider_error"
		var emptyContentErr *proxyEmptyContentError
		var localRequestErr *proxyLocalRequestError
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			failureCode = "publisher_llm_timeout"
		case errors.Is(err, context.Canceled):
			failureCode = "publisher_llm_request_canceled"
		case errors.As(err, &emptyContentErr):
			failureCode = "publisher_llm_empty_content"
		case errors.As(err, &localRequestErr):
			failureCode = "publisher_llm_request_invalid"
		case upstreamStatus >= http.StatusBadRequest && upstreamStatus < http.StatusInternalServerError:
			failureCode = "publisher_llm_upstream_rejected"
		case upstreamStatus >= http.StatusInternalServerError:
			failureCode = "publisher_llm_upstream_unavailable"
		}
		return nil, map[string]any{
			"prompt_source":   promptSource,
			"model":           cfg.Model,
			"failure_code":    failureCode,
			"failure_detail":  scrubProxySecret(err.Error(), cfg.APIKey),
			"upstream_status": upstreamStatus,
		}, err
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
			"allowed_roles": []string{"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not", "pacing", "scene_emphasis", "may_advance", "hold_allowed", "arc_anchor", "preferred_frontier", "reversible_option", "ending_edge"},
			"allowed_expression_kinds": []string{
				"portrayal", "response_focus", "must_account", "pacing", "scene_emphasis", "callback",
				"may_advance", "hold_allowed", "arc_anchor", "preferred_frontier", "reversible_option",
				"character_expression", "relationship_expression", "world_guard", "must_not", "ending_edge",
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
			"allowed_roles": []string{"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not", "pacing", "scene_emphasis", "may_advance", "hold_allowed"},
			"allowed_expression_kinds": []string{
				"portrayal", "response_focus", "must_account", "callback", "character_expression", "relationship_expression", "world_guard", "must_not",
				"pacing", "scene_emphasis", "may_advance", "hold_allowed",
			},
		})
	default:
		return withCommon(map[string]any{
			"profile":                  "fidelity_expression_low_impact",
			"supervisor_call":          "source_backed_optional",
			"guidance_scope":           "response_focus_and_must_account",
			"guidance_options":         []string{"response_focus", "must_account"},
			"allowed_roles":            []string{"fidelity_warning", "response_focus", "must_account", "portrayal", "callback", "character_expression", "relationship_expression", "world_guard", "must_not"},
			"allowed_expression_kinds": []string{"portrayal", "response_focus", "must_account", "callback", "character_expression", "relationship_expression", "world_guard", "must_not"},
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
		"publisher_plan":    zeroPublisherPlan(strength, "zero", "no_accepted_proposal"),
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
		proposal["publisher_plan"] = zeroPublisherPlan(strength, "zero", "narrative_guide_disabled")
		trace["reason_code"] = "narrative_guide_disabled"
		return boundedSupervisorEnvelope(proposal), trace
	}
	if !contractReady {
		proposal["status"] = "degraded_missing_execution_contract"
		proposal["reason_code"] = contractReasonCode
		proposal["publisher_plan"] = zeroPublisherPlan(strength, "zero", contractReasonCode)
		trace["reason_code"] = contractReasonCode
		return boundedSupervisorEnvelope(proposal), trace
	}
	if parsed == nil {
		proposal["status"] = "malformed_failed_open"
		proposal["reason_code"] = "supervisor_malformed_json"
		proposal["publisher_plan"] = zeroPublisherPlan(strength, "zero", "supervisor_malformed_json")
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
		proposal["publisher_plan"] = zeroPublisherPlan(strength, "zero", "supervisor_schema_invalid")
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
	typedRequiredRefs := map[string]map[string]struct{}{}
	executionRefs := mapFromAny(mapFromAny(supervisorPack["response_execution_contract"])["source_refs"])
	for _, kind := range []string{"may_advance", "arc_anchor", "preferred_frontier"} {
		refs := map[string]struct{}{}
		for _, ref := range stringSliceFromAny(executionRefs[kind]) {
			if ref = strings.TrimSpace(ref); ref != "" {
				refs[ref] = struct{}{}
			}
		}
		typedRequiredRefs[kind] = refs
	}
	expressionItems, expressionRejected := normalizeSupervisorExpressionItems(rawProposal["expression_hints"], allowedKinds, allowedRefs, expressionSupportRefs, memoryRefs, typedRequiredRefs)
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
	} else {
		proposal["publisher_plan"] = buildPublisherPlan(proposal, supervisorPack)
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
		"publisher_plan":   proposal["publisher_plan"],
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

func normalizeSupervisorExpressionItems(raw any, allowedKinds, allowedRefs, expressionSupportRefs, memoryRefs map[string]struct{}, typedRequiredRefs map[string]map[string]struct{}) ([]map[string]any, int) {
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
		if refs, typed := typedRequiredRefs[kind]; typed {
			requiredRefs = refs
			verificationState = "go_preapproved_typed_ref_proposal"
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
	for _, ref := range stringSliceFromAny(executionRefs["continuity"]) {
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
	for _, raw := range outputFidelityLineageSlice(supportPacket["accepted_recent_context"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		if _, allowed := allowedMemory[ref]; allowed {
			memoryRefs = appendUniqueStringValues(memoryRefs, ref)
		}
	}
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
	for _, raw := range outputFidelityLineageSlice(supportPacket["delivered_character_memory"]) {
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

func zeroPublisherPlan(strength, status, reason string) map[string]any {
	return map[string]any{
		"contract_version":             "publisher_plan.v1",
		"status":                       status,
		"reason_code":                  nilIfEmpty(reason),
		"guide_strength":               normalizeNarrativeGuideStrength(strength),
		"authority":                    "expression_assistance_only",
		"truth_authority":              false,
		"would_write":                  false,
		"response_focus_refs":          []string{},
		"must_account_refs":            []string{},
		"continuity_anchor_ref":        nil,
		"character_expression_refs":    []string{},
		"relationship_expression_refs": []string{},
		"world_guard_refs":             []string{},
		"must_not_refs":                []string{},
		"advance_mode":                 nil,
		"preferred_frontier_ref":       nil,
		"ending_edge":                  nil,
		"guidance_items":               []map[string]any{},
		"candidate_count_cap":          nil,
		"selection_policy":             "accepted_source_bound_items_only_then_existing_output_guidance_char_budget",
		"blocked_generation": []string{
			"new_fact", "dialogue", "relationship_reciprocity", "user_action", "event_closure",
		},
	}
}

func buildPublisherPlan(proposal, supervisorPack map[string]any) map[string]any {
	strength := normalizeNarrativeGuideStrength(extractionStringFromAny(proposal["guide_strength"]))
	plan := zeroPublisherPlan(strength, "zero", "no_publisher_eligible_items")
	if extractionStringFromAny(proposal["status"]) != "ready" || strength == "none" {
		return plan
	}
	support := publisherDeliveredSupportByRef(supervisorPack)
	responseFocusRefs := []string{}
	mustAccountRefs := []string{}
	characterRefs := []string{}
	relationshipRefs := []string{}
	worldRefs := []string{}
	mustNotRefs := []string{}
	guidance := []map[string]any{}
	continuityRefs := []string{}
	preferredFrontierRefs := []string{}
	endingEdges := []map[string]any{}

	appendGuidance := func(slot string, item map[string]any) {
		compiled, ok := compilePublisherGuidanceItem(slot, item, support)
		if !ok {
			return
		}
		guidance = append(guidance, compiled)
		refs := stringSliceFromAny(compiled["source_refs"])
		switch slot {
		case "response_focus":
			responseFocusRefs = appendUniqueStringValues(responseFocusRefs, refs...)
		case "must_account":
			mustAccountRefs = appendUniqueStringValues(mustAccountRefs, refs...)
		case "continuity_anchor":
			continuityRefs = appendUniqueStringValues(continuityRefs, refs...)
		case "world_guard":
			worldRefs = appendUniqueStringValues(worldRefs, refs...)
		case "must_not":
			mustNotRefs = appendUniqueStringValues(mustNotRefs, refs...)
		case "preferred_frontier":
			preferredFrontierRefs = appendUniqueStringValues(preferredFrontierRefs, refs...)
		case "ending_edge":
			endingEdges = append(endingEdges, compiled)
		}
		for _, ref := range refs {
			meta := support[ref]
			class := extractionStringFromAny(meta["class"])
			kind := extractionStringFromAny(meta["kind"])
			switch {
			case class == "subjective_relationship" || kind == "relationship_state":
				relationshipRefs = appendUniqueStringValues(relationshipRefs, ref)
			case class == "character_objective" && (kind == "character_profile" || kind == "character_profile_counterevidence" || kind == "voice_behavior"):
				characterRefs = appendUniqueStringValues(characterRefs, ref)
			}
		}
	}

	for _, raw := range outputFidelityLineageSlice(proposal["fidelity_warnings"]) {
		appendGuidance("must_account", mapFromAny(raw))
	}
	expressions := []map[string]any{}
	for _, raw := range outputFidelityLineageSlice(proposal["expression_hints"]) {
		expressions = append(expressions, mapFromAny(raw))
	}
	for _, item := range expressions {
		switch extractionStringFromAny(item["kind"]) {
		case "response_focus":
			appendGuidance("response_focus", item)
		case "must_account":
			appendGuidance("must_account", item)
		case "portrayal":
			if publisherItemHasDeliveredCharacterRef(item, support) {
				appendGuidance("character_or_relationship_expression", item)
			} else {
				appendGuidance("response_focus", item)
			}
		case "callback", "arc_anchor":
			if publisherItemHasDeliveredMemoryRef(item, support) {
				appendGuidance("continuity_anchor", item)
			}
		case "character_expression", "relationship_expression":
			if publisherItemHasDeliveredCharacterRef(item, support) {
				appendGuidance("character_or_relationship_expression", item)
			}
		case "world_guard", "must_not", "pacing", "scene_emphasis", "reversible_option":
			appendGuidance(extractionStringFromAny(item["kind"]), item)
		case "preferred_frontier":
			if publisherItemHasDeliveredMemoryRef(item, support) {
				appendGuidance("preferred_frontier", item)
			}
		case "ending_edge":
			if publisherItemHasDeliveredMemoryRef(item, support) {
				appendGuidance("ending_edge", item)
			}
		}
	}

	if strength == "medium" || strength == "strong" {
		advanceKind := ""
		for _, item := range expressions {
			if extractionStringFromAny(item["kind"]) == "hold_allowed" {
				advanceKind = "hold_allowed"
				break
			}
		}
		if advanceKind == "" {
			for _, item := range expressions {
				if extractionStringFromAny(item["kind"]) == "may_advance" && publisherItemHasDeliveredMemoryRef(item, support) {
					advanceKind = "may_advance"
					break
				}
			}
		}
		if advanceKind != "" {
			if merged, ok := mergePublisherAdvanceItems(advanceKind, expressions, support); ok {
				plan["advance_mode"] = advanceKind
				appendGuidance(advanceKind, merged)
			}
		}
	}

	plan["response_focus_refs"] = responseFocusRefs
	plan["must_account_refs"] = mustAccountRefs
	plan["character_expression_refs"] = characterRefs
	plan["relationship_expression_refs"] = relationshipRefs
	plan["world_guard_refs"] = worldRefs
	plan["must_not_refs"] = mustNotRefs
	if len(continuityRefs) == 1 {
		plan["continuity_anchor_ref"] = continuityRefs[0]
	}
	if len(preferredFrontierRefs) == 1 {
		plan["preferred_frontier_ref"] = preferredFrontierRefs[0]
	}
	if len(endingEdges) == 1 {
		plan["ending_edge"] = map[string]any{
			"text":        endingEdges[0]["text"],
			"source_refs": endingEdges[0]["source_refs"],
		}
	}
	plan["guidance_items"] = guidance
	if len(guidance) > 0 {
		plan["status"] = "ready"
		plan["reason_code"] = nil
	}
	return plan
}

func mergePublisherAdvanceItems(kind string, expressions []map[string]any, support map[string]map[string]any) (map[string]any, bool) {
	texts := []string{}
	refs := []string{}
	for _, item := range expressions {
		if extractionStringFromAny(item["kind"]) != kind {
			continue
		}
		if kind == "may_advance" && !publisherItemHasDeliveredMemoryRef(item, support) {
			continue
		}
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		itemRefs := stringSliceFromAny(item["source_refs"])
		if text == "" || len(itemRefs) == 0 {
			continue
		}
		texts = append(texts, text)
		refs = appendUniqueStringValues(refs, itemRefs...)
	}
	if len(texts) == 0 || len(refs) == 0 || (kind == "may_advance" && len(texts) != 1) {
		return nil, false
	}
	return map[string]any{
		"kind": kind, "text": strings.Join(texts, "\n"), "source_refs": refs,
		"verification_state": "all_accepted_items_of_selected_advance_mode_merged_without_count_cut",
	}, true
}

func publisherDeliveredSupportByRef(supervisorPack map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	packet := mapFromAny(supervisorPack["support_packet"])
	if current := mapFromAny(packet["current_input"]); strings.TrimSpace(extractionStringFromAny(current["raw_text"])) != "" {
		if ref := strings.TrimSpace(extractionStringFromAny(current["source_ref"])); ref != "" {
			out[ref] = map[string]any{"kind": "current_input", "class": "current_input", "delivered": true}
		}
	}
	for _, raw := range outputFidelityLineageSlice(packet["accepted_recent_context"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" || strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		out[ref] = map[string]any{
			"kind": "accepted_recent_context", "class": "continuity", "delivered": true,
			"visibility_boundary": item["visibility_boundary"],
		}
	}
	for _, raw := range outputFidelityLineageSlice(packet["delivered_memory"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" || strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		out[ref] = map[string]any{
			"kind": "long_term_memory", "class": "memory", "delivered": true,
			"protected_guard": item["protected_guard"], "visibility_boundary": item["visibility_boundary"],
		}
	}
	for _, raw := range outputFidelityLineageSlice(packet["delivered_character_memory"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" || strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		out[ref] = map[string]any{
			"kind": extractionStringFromAny(item["kind"]), "class": extractionStringFromAny(item["class"]), "delivered": true,
			"privacy_guard": item["privacy_guard"], "visibility_boundary": item["visibility_boundary"],
		}
	}
	return out
}

func compilePublisherGuidanceItem(slot string, item map[string]any, support map[string]map[string]any) (map[string]any, bool) {
	text := strings.TrimSpace(extractionStringFromAny(item["text"]))
	refs := []string{}
	privacyGuard := ""
	directionalRelationship := false
	for _, ref := range stringSliceFromAny(item["source_refs"]) {
		meta, ok := support[ref]
		if !ok || !boolFromAny(meta["delivered"]) {
			return nil, false
		}
		refs = appendUniqueStringValues(refs, ref)
		if guard := extractionStringFromAny(meta["privacy_guard"]); guard != "" {
			privacyGuard = guard
		}
		if extractionStringFromAny(meta["kind"]) == "relationship_state" {
			directionalRelationship = true
		}
	}
	if text == "" || len(refs) == 0 {
		return nil, false
	}
	renderText := text
	switch slot {
	case "ending_edge":
		renderText += " Treat this only as the boundary of the current response; never close a thread, arc, session, or work."
	case "may_advance":
		renderText += " Keep any advance reversible; do not create a new fact, relationship change, user action, or closure."
	case "preferred_frontier":
		renderText += " Treat this only as a current-response preference and never as a persistent plot lock."
	}
	if privacyGuard != "" {
		renderText += " Apply only as guarded subtext; do not reveal the private fact."
	}
	if directionalRelationship {
		renderText += " Preserve the stated direction and do not infer reciprocity."
	}
	return map[string]any{
		"slot": slot, "kind": extractionStringFromAny(item["kind"]), "text": text, "render_text": renderText,
		"source_refs": refs, "privacy_guard": nilIfEmpty(privacyGuard),
		"relationship_directional_only": directionalRelationship,
		"no_reciprocity":                directionalRelationship,
		"authority":                     "expression_assistance_only",
	}, true
}

func publisherItemHasDeliveredMemoryRef(item map[string]any, support map[string]map[string]any) bool {
	for _, ref := range stringSliceFromAny(item["source_refs"]) {
		meta := support[ref]
		if boolFromAny(meta["delivered"]) && extractionStringFromAny(meta["kind"]) != "current_input" {
			return true
		}
	}
	return false
}

func publisherItemHasDeliveredCharacterRef(item map[string]any, support map[string]map[string]any) bool {
	for _, ref := range stringSliceFromAny(item["source_refs"]) {
		meta := support[ref]
		if !boolFromAny(meta["delivered"]) {
			continue
		}
		class := extractionStringFromAny(meta["class"])
		kind := extractionStringFromAny(meta["kind"])
		if class == "character_objective" || class == "subjective_relationship" ||
			kind == "character_profile" || kind == "character_profile_counterevidence" || kind == "voice_behavior" || kind == "relationship_state" {
			return true
		}
	}
	return false
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
