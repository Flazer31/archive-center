package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

var proxyHTTPClient = http.DefaultClient

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
	narrativeStance := stringPtrValue(req.NarrativeStance, "balanced")
	autoAdvanceTrigger := stringPtrValue(req.AutoAdvanceTrigger, "none")
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
	supervisorPack := buildSupervisorInputPack(sid, 0, "", guideMode, guideStrength, narrativeStance, autoAdvanceTrigger, wakeUpContext, promptTrace, evidenceCounts, sectionSummary, storylineSelection, false, "", nil)
	if len(req.ResponseExecutionContract) > 0 {
		supervisorPack["response_execution_contract"] = req.ResponseExecutionContract
	}
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
		result, llmTrace, err := s.runSupervisorLLM(r.Context(), sid, supervisorPack, req.SupervisorRequest, llmCfg)
		trace["would_call_llm"] = true
		trace["llm_call"] = "executed"
		trace["llm_trace"] = llmTrace
		if err != nil {
			trace["llm_call"] = "failed"
			trace["fail_open"] = true
			trace["error"] = scrubProxySecret(err.Error(), llmCfg.APIKey)
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
				"error":                 scrubProxySecret(err.Error(), llmCfg.APIKey),
				"trace_summary":         trace,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                "ok",
			"source":                "runtime_llm",
			"note":                  "POST /supervisor used configured runtime LLM settings",
			"chat_session_id":       sid,
			"supervisor_input_pack": supervisorPack,
			"would_call_llm":        true,
			"would_write":           false,
			"upstream_write":        "disabled",
			"supervisor_result":     result,
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

func (s *Server) runSupervisorLLM(ctx context.Context, sid string, supervisorPack map[string]any, req dto.SupervisorRequest, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	systemPrompt, promptSource := readSupervisorSystemPrompt(s.Cfg.PromptDir)
	guideMode := resolveNarrativeGuideMode(stringPtrValue(req.GuideMode, "off"), req.ContextMessages, stringPtrValue(req.WakeUpContext, ""), "")
	payload := map[string]any{
		"chat_session_id":              sid,
		"guide_mode":                   guideMode,
		"guide_strength":               extractionStringFromAny(supervisorPack["guide_strength"]),
		"guide_focus":                  supervisorPack["guide_focus"],
		"context_messages":             req.ContextMessages,
		"response_execution_contract":  supervisorPack["response_execution_contract"],
		"supervisor_proposal_coverage": supervisorProposalCoverage(extractionStringFromAny(supervisorPack["guide_strength"])),
		"required_output": "Return only JSON with supervisor_scene_proposal. Every proposal item must be an object with text and source_refs, including at least one exact ref copied from response_execution_contract.source_refs.memory. " +
			"Use only fidelity_warnings and portrayal_notes fields allowed by supervisor_proposal_coverage. Proposals may preserve supported memory, relationship, knowledge, and privacy continuity, but may not prescribe story structure, pacing, scene count, transitions, endings, or user actions.",
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
	upstream, _, err := performProxyPluginMain(ctx, reqBody)
	if err != nil {
		return nil, map[string]any{"prompt_source": promptSource, "model": cfg.Model}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		parsed = map[string]any{"directive": map[string]any{"raw_text": strings.TrimSpace(content)}}
	}
	trace := map[string]any{
		"prompt_source": promptSource,
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	bounded, proposalTrace := buildBoundedSupervisorResult(parsed, supervisorPack)
	trace["proposal_contract"] = proposalTrace
	return bounded, trace, nil
}

func supervisorProposalCoverage(strength string) map[string]any {
	switch normalizeNarrativeGuideStrength(strength) {
	case "none":
		return map[string]any{
			"profile":       "disabled",
			"allowed_roles": []string{},
		}
	case "strong":
		return map[string]any{
			"profile":       "guard_and_portrayal",
			"allowed_roles": []string{"fidelity_warning", "portrayal_note"},
		}
	case "medium":
		return map[string]any{
			"profile":       "guard_and_portrayal",
			"allowed_roles": []string{"fidelity_warning", "portrayal_note"},
		}
	default:
		return map[string]any{
			"profile":       "guard_only",
			"allowed_roles": []string{"fidelity_warning"},
		}
	}
}

func buildBoundedSupervisorResult(parsed, supervisorPack map[string]any) (map[string]any, map[string]any) {
	strength := normalizeNarrativeGuideStrength(extractionStringFromAny(supervisorPack["guide_strength"]))
	coverage := supervisorProposalCoverage(strength)
	executionContract := mapFromAny(supervisorPack["response_execution_contract"])

	sourceRefs := mapFromAny(executionContract["source_refs"])
	allowedRefList := append([]string{}, stringSliceFromAny(sourceRefs["all"])...)
	allowedRefList = appendUniqueStringValues(allowedRefList, stringSliceFromAny(sourceRefs["current_input"])...)
	allowedRefList = appendUniqueStringValues(allowedRefList, stringSliceFromAny(sourceRefs["native_system"])...)
	memoryRefList := appendUniqueStringValues([]string{}, stringSliceFromAny(sourceRefs["memory"])...)
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
	contractReady, contractReasonCode := supervisorExecutionContractReady(supervisorPack)

	proposal := map[string]any{
		"contract_version":   "supervisor_scene_proposal.v2",
		"status":             "ready",
		"authority":          "proposal_only",
		"truth_authority":    false,
		"would_write":        false,
		"guide_strength":     strength,
		"coverage":           coverage,
		"verification_state": "delivered_memory_ref_required_per_item",
		"application_rule":   "Treat every item as optional support. Apply it only when it cites at least one delivered memory source and remains compatible with the current user input; never use it as story truth or permission to decide user actions, relationship changes, event closure, scene structure, pacing, transitions, endings, or next actions.",
		"blocked_authority": []string{
			"new_fact",
			"unverified_relationship_change",
			"user_protagonist_action",
			"unresolved_event_closure",
			"canonical_write",
		},
		"source_refs": map[string]any{
			"allowed":                  allowedRefList,
			"required_memory_evidence": memoryRefList,
		},
		"fidelity_warnings": []map[string]any{},
		"portrayal_notes":   []map[string]any{},
	}
	trace := map[string]any{
		"contract_ready": contractReady,
		"allowed_refs":   len(allowedRefs),
		"memory_refs":    len(memoryRefs),
		"guide_strength": strength,
		"coverage":       coverage,
	}
	if !contractReady {
		proposal["status"] = "degraded_missing_execution_contract"
		proposal["reason_code"] = contractReasonCode
		trace["reason_code"] = contractReasonCode
		return boundedSupervisorEnvelope(proposal), trace
	}
	if strength == "none" {
		proposal["status"] = "disabled"
		proposal["reason_code"] = "narrative_guide_disabled"
		trace["reason_code"] = "narrative_guide_disabled"
		return boundedSupervisorEnvelope(proposal), trace
	}

	rawProposal := mapFromAny(parsed["supervisor_scene_proposal"])
	if len(rawProposal) == 0 {
		rawDirective := mapFromAny(parsed["directive"])
		rawProposal = mapFromAny(rawDirective["supervisor_scene_proposal"])
	}
	allowedRoles := make(map[string]struct{})
	for _, role := range stringSliceFromAny(coverage["allowed_roles"]) {
		allowedRoles[role] = struct{}{}
	}

	acceptedTotal := 0
	rejectedTotal := 0
	for _, lane := range []struct {
		field string
		role  string
	}{
		{field: "fidelity_warnings", role: "fidelity_warning"},
		{field: "portrayal_notes", role: "portrayal_note"},
	} {
		if _, allowed := allowedRoles[lane.role]; !allowed {
			rejectedTotal += anySliceLength(rawProposal[lane.field])
			continue
		}
		items, rejected := normalizeSupervisorProposalItems(rawProposal[lane.field], allowedRefs, memoryRefs)
		proposal[lane.field] = items
		acceptedTotal += len(items)
		rejectedTotal += rejected
	}
	if acceptedTotal == 0 {
		proposal["status"] = "ready_no_supported_proposal"
	}
	trace["accepted_items"] = acceptedTotal
	trace["rejected_items"] = rejectedTotal
	trace["raw_legacy_fields_discarded"] = len(rawProposal) == 0
	return boundedSupervisorEnvelope(proposal), trace
}

func supervisorExecutionContractReady(supervisorPack map[string]any) (bool, string) {
	executionContract := mapFromAny(supervisorPack["response_execution_contract"])
	if extractionStringFromAny(executionContract["contract_version"]) != "response_execution_contract.v1" ||
		extractionStringFromAny(executionContract["status"]) != "ready" ||
		!boolFromAny(executionContract["active"]) {
		return false, "supervisor_execution_contract_missing"
	}
	sourceRefs := mapFromAny(executionContract["source_refs"])
	for _, ref := range stringSliceFromAny(sourceRefs["memory"]) {
		if strings.TrimSpace(ref) != "" {
			return true, ""
		}
	}
	return false, "supervisor_execution_contract_has_no_memory_refs"
}

func boundedSupervisorEnvelope(proposal map[string]any) map[string]any {
	return map[string]any{
		"contract_version": "supervisor_scene_proposal.v2",
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
		validRefs := make([]string, 0, len(refs))
		valid := true
		hasMemoryEvidence := false
		for _, ref := range refs {
			ref = strings.TrimSpace(ref)
			if _, exists := allowedRefs[ref]; !exists {
				valid = false
				break
			}
			if _, exists := memoryRefs[ref]; exists {
				hasMemoryEvidence = true
			}
			validRefs = appendUniqueStringValues(validRefs, ref)
		}
		key := strings.ToLower(text)
		if !valid || len(validRefs) == 0 || !hasMemoryEvidence {
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

	resp, status, err := performProxyPluginMain(r.Context(), req)
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
	return callProxyProvider(ctx, req)
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
