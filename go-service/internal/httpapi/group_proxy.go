package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
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
	supervisorPack["support_packet"] = buildSupervisorSupportPacket(sid, currentInput, mapFromAny(supervisorPack["response_execution_contract"]), nil, "", nil, nil)
	trace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	trace["guide_mode"] = guideMode
	trace["guide_strength"] = guideStrength
	trace["supervisor_proposal_coverage"] = publisherStrengthProfile(guideStrength)
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
		trace["llm_trace"] = llmTrace
		if err != nil {
			failureCode := extractionFirstNonEmpty(extractionStringFromAny(llmTrace["failure_code"]), "publisher_llm_failed_open")
			providerCallAttempted := failureCode != "publisher_system_prompt_unavailable"
			trace["would_call_llm"] = providerCallAttempted
			if providerCallAttempted {
				trace["llm_call"] = "failed"
			} else {
				trace["llm_call"] = "skipped"
			}
			trace["fail_open"] = true
			trace["reason_code"] = failureCode
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                "partial",
				"source":                "runtime_llm_error",
				"note":                  "POST /supervisor could not complete the configured LLM call and failed open",
				"chat_session_id":       sid,
				"supervisor_input_pack": supervisorPack,
				"would_call_llm":        providerCallAttempted,
				"would_write":           false,
				"upstream_write":        "disabled",
				"supervisor_result":     nil,
				"fail_open":             true,
				"reason_code":           failureCode,
				"trace_summary":         trace,
			})
			return
		}
		trace["would_call_llm"] = true
		trace["llm_call"] = "executed"
		responseStatus := "ok"
		responseSource := "runtime_llm"
		failOpen := false
		reasonCode := ""
		resultProposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
		proposalStatus := extractionStringFromAny(resultProposal["status"])
		if proposalStatus == "publisher_response_container_invalid" || proposalStatus == "publisher_llm_empty_content" || proposalStatus == "publisher_json_malformed" || proposalStatus == "publisher_json_truncated" || proposalStatus == "publisher_schema_invalid" || proposalStatus == "publisher_plan_no_valid_items" {
			responseStatus = "partial"
			responseSource = "runtime_llm_rejected"
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
	systemPrompt, promptSource, promptErr := readSupervisorSystemPrompt(s.Cfg.PromptDir)
	if promptErr != nil {
		return nil, map[string]any{
			"prompt_source":  promptSource,
			"model":          cfg.Model,
			"failure_code":   "publisher_system_prompt_unavailable",
			"failure_detail": promptErr.Error(),
		}, promptErr
	}
	guideMode := normalizeNarrativeGuideMode(extractionStringFromAny(supervisorPack["guide_mode"]))
	payload := map[string]any{
		"chat_session_id":             sid,
		"guide_mode":                  guideMode,
		"guide_strength":              extractionStringFromAny(supervisorPack["guide_strength"]),
		"publisher_strength_profile":  publisherStrengthProfile(extractionStringFromAny(supervisorPack["guide_strength"])),
		"guide_focus":                 supervisorPack["guide_focus"],
		"supervisor_support_packet":   supervisorPack["support_packet"],
		"response_execution_contract": supervisorPack["response_execution_contract"],
		"required_output":             "Return one JSON object containing supervisor_scene_proposal.publisher_plan with contract_version publisher_plan.v2 and both book_author and director roles. Include all eight required role fields, using null or [] when no supported item exists. Every non-empty item must copy exact source_refs from text-bearing entries in supervisor_support_packet. Do not add prose, markdown, defaults, legacy fields, invented refs, facts, user actions, relationship changes, scene jumps, or event closure.",
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
	// Publisher planning is exactly one provider request. A rejected request is
	// reported explicitly; it is never retried with a different parameter set.
	upstream, upstreamStatus, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, reqBody, nil, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
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
	trace := map[string]any{
		"prompt_source": promptSource,
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
	}
	providerResponse := mapFromAny(upstream[proxyResponseMetadataKey])
	if len(providerResponse) > 0 {
		trace["provider_response"] = providerResponse
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	content, responseTrace, responseFailure := normalizePublisherResponseContent(upstream)
	trace["response_normalization"] = responseTrace
	if responseFailure != "" {
		trace["parse_status"] = responseFailure
		bounded, proposalTrace := buildPublisherFailureResult(supervisorPack, responseFailure)
		trace["proposal_contract"] = proposalTrace
		return bounded, trace, nil
	}
	parsed, parseErr := parsePublisherJSONObject(content)
	if parseErr != nil {
		parseStatus := "publisher_json_malformed"
		if stringFromMap(providerResponse, "termination_kind") == "length" {
			parseStatus = "publisher_json_truncated"
		}
		trace["parse_status"] = parseStatus
		trace["parse_failure"] = "strict_json_rejected"
		bounded, proposalTrace := buildPublisherFailureResult(supervisorPack, parseStatus)
		trace["proposal_contract"] = proposalTrace
		return bounded, trace, nil
	}
	trace["parse_status"] = "parsed"
	bounded, proposalTrace := buildBoundedSupervisorResult(parsed, supervisorPack)
	trace["proposal_contract"] = proposalTrace
	return bounded, trace, nil
}

func publisherStrengthProfile(strength string) map[string]any {
	strength = normalizeNarrativeGuideStrength(strength)
	profile := map[string]any{
		"contract_version":            "publisher_strength_profile.v1",
		"strength":                    strength,
		"response_scope":              "current_response_only",
		"truth_authority":             false,
		"canonical_write":             false,
		"force_progress":              false,
		"force_user_action":           false,
		"invent_new_facts":            false,
		"confirm_relationship_change": false,
		"close_unresolved_event":      false,
		"persistent_carry":            false,
		"pressure_independent":        true,
		"item_policy":                 "supported_items_only_no_filler",
	}
	if strength == "none" {
		profile["publisher_call"] = "none"
		profile["roles"] = []string{}
		profile["guidance_explicitness"] = "disabled"
		return profile
	}
	profile["publisher_call"] = "single_source_backed"
	profile["roles"] = []string{"book_author", "director"}
	switch strength {
	case "medium":
		profile["guidance_explicitness"] = "balanced"
	case "strong":
		profile["guidance_explicitness"] = "direct"
	case "extreme":
		profile["guidance_explicitness"] = "ordered"
	case "maximum":
		profile["guidance_explicitness"] = "execution_brief"
	default:
		profile["guidance_explicitness"] = "gentle"
	}
	return profile
}

func normalizePublisherResponseContent(resp map[string]any) (string, map[string]any, string) {
	trace := map[string]any{
		"choice_index":           0,
		"extra_choices_ignored":  0,
		"text_parts_accepted":    0,
		"non_text_parts_ignored": 0,
	}
	choices, ok := resp["choices"].([]any)
	if !ok || len(choices) == 0 {
		trace["container"] = "choices_missing"
		return "", trace, "publisher_response_container_invalid"
	}
	trace["extra_choices_ignored"] = maxInt(0, len(choices)-1)
	choice, ok := choices[0].(map[string]any)
	if !ok {
		trace["container"] = "choice_invalid"
		return "", trace, "publisher_response_container_invalid"
	}
	messageValue, messagePresent := choice["message"]
	message, messageOK := messageValue.(map[string]any)
	contentValue, contentPresent := any(nil), false
	if messageOK {
		contentValue, contentPresent = message["content"]
	} else if messagePresent && messageValue != nil {
		trace["container"] = "message_invalid"
		return "", trace, "publisher_response_container_invalid"
	}

	content := ""
	if contentPresent && contentValue != nil {
		switch value := contentValue.(type) {
		case string:
			trace["container"] = "message_content_string"
			content = value
		case []any:
			trace["container"] = "message_content_array"
			var builder strings.Builder
			for _, rawPart := range value {
				switch part := rawPart.(type) {
				case string:
					builder.WriteString(part)
					trace["text_parts_accepted"] = intFromAny(trace["text_parts_accepted"], 0) + 1
				case map[string]any:
					if textPart, ok := part["text"].(string); ok {
						builder.WriteString(textPart)
						trace["text_parts_accepted"] = intFromAny(trace["text_parts_accepted"], 0) + 1
					} else {
						trace["non_text_parts_ignored"] = intFromAny(trace["non_text_parts_ignored"], 0) + 1
					}
				default:
					trace["non_text_parts_ignored"] = intFromAny(trace["non_text_parts_ignored"], 0) + 1
				}
			}
			content = builder.String()
		default:
			trace["container"] = "message_content_unsupported"
			return "", trace, "publisher_response_container_invalid"
		}
	}
	if strings.TrimSpace(content) == "" {
		return "", trace, "publisher_llm_empty_content"
	}
	return content, trace, ""
}

func parsePublisherJSONObject(content string) (map[string]any, error) {
	content = strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if content == "" {
		return nil, fmt.Errorf("publisher JSON is empty")
	}
	objects, incomplete := publisherTopLevelJSONObjectRanges(content)
	if incomplete {
		return nil, fmt.Errorf("publisher JSON object is incomplete")
	}
	if len(objects) != 1 {
		return nil, fmt.Errorf("publisher response must contain exactly one top-level JSON object")
	}
	objectStart, objectEnd := objects[0][0], objects[0][1]
	if !publisherWrapperIsHarmless(content[:objectStart]) || !publisherWrapperIsHarmless(content[objectEnd:]) {
		return nil, fmt.Errorf("publisher response wrapper is not harmless")
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(content[objectStart:objectEnd])))
	decoder.UseNumber()
	value, err := decodePublisherJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("publisher JSON has trailing content")
		}
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("publisher JSON root is not an object")
	}
	return object, nil
}

func publisherTopLevelJSONObjectRanges(content string) ([][2]int, bool) {
	ranges := make([][2]int, 0, 1)
	depth := 0
	start := -1
	inString := false
	escaped := false
	for index := 0; index < len(content); index++ {
		ch := content[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		if ch == '"' && depth > 0 {
			inString = true
			continue
		}
		switch ch {
		case '{':
			if depth == 0 {
				start = index
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				ranges = append(ranges, [2]int{start, index + 1})
				start = -1
			}
		}
	}
	return ranges, depth != 0 || inString
}

func publisherWrapperIsHarmless(wrapper string) bool {
	wrapper = strings.TrimSpace(strings.TrimPrefix(wrapper, "\ufeff"))
	if wrapper == "" {
		return true
	}
	if strings.ContainsAny(wrapper, "{}[]") {
		return false
	}
	withoutFences := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(wrapper, "```json", ""), "```", ""))
	if withoutFences == "" {
		return true
	}
	return !json.Valid([]byte(withoutFences))
}

func decodePublisherJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("publisher JSON object key is invalid")
			}
			if _, duplicate := seen[key]; duplicate {
				return nil, fmt.Errorf("publisher JSON contains duplicate key %q", key)
			}
			seen[key] = struct{}{}
			value, err := decodePublisherJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("publisher JSON object is incomplete")
		}
		return object, nil
	case '[':
		values := []any{}
		for decoder.More() {
			value, err := decodePublisherJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("publisher JSON array is incomplete")
		}
		return values, nil
	default:
		return nil, fmt.Errorf("publisher JSON delimiter is invalid")
	}
}

type publisherV2FieldSpec struct {
	role     string
	field    string
	isArray  bool
	pressure bool
}

var publisherV2FieldSpecs = []publisherV2FieldSpec{
	{role: "book_author", field: "current_arc"},
	{role: "book_author", field: "narrative_goal"},
	{role: "book_author", field: "next_beats", isArray: true},
	{role: "book_author", field: "guardrails", isArray: true},
	{role: "director", field: "scene_mandate"},
	{role: "director", field: "required_outcomes", isArray: true},
	{role: "director", field: "forbidden_moves", isArray: true},
	{role: "director", field: "pressure_level", pressure: true},
}

func buildPublisherFailureResult(supervisorPack map[string]any, reason string) (map[string]any, map[string]any) {
	strength := normalizeNarrativeGuideStrength(extractionStringFromAny(supervisorPack["guide_strength"]))
	contractReady, _ := supervisorExecutionContractReady(supervisorPack)
	proposal := publisherProposalBase(strength)
	proposal["status"] = reason
	proposal["reason_code"] = reason
	trace := map[string]any{
		"contract_ready": contractReady,
		"guide_strength": strength,
		"reason_code":    reason,
		"accepted_items": 0,
		"rejected_items": 0,
		"fail_open":      true,
	}
	return boundedSupervisorEnvelope(proposal), trace
}

func publisherProposalBase(strength string) map[string]any {
	return map[string]any{
		"contract_version": "supervisor_scene_proposal.v3",
		"status":           "ready",
		"authority":        "proposal_only",
		"truth_authority":  false,
		"would_write":      false,
		"guide_strength":   strength,
		"coverage":         publisherStrengthProfile(strength),
	}
}

func buildBoundedSupervisorResult(parsed, supervisorPack map[string]any) (map[string]any, map[string]any) {
	strength := normalizeNarrativeGuideStrength(extractionStringFromAny(supervisorPack["guide_strength"]))
	currentInputRefList, memoryRefList := supervisorSupportReferenceLists(supervisorPack)
	allowedRefList := appendUniqueStringValues([]string{}, currentInputRefList...)
	allowedRefList = appendUniqueStringValues(allowedRefList, memoryRefList...)
	allowedRefs := make(map[string]struct{}, len(allowedRefList))
	for _, ref := range allowedRefList {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedRefs[ref] = struct{}{}
		}
	}
	contractReady, contractReasonCode := supervisorExecutionContractReady(supervisorPack)
	proposal := publisherProposalBase(strength)
	trace := map[string]any{
		"contract_ready": contractReady,
		"allowed_refs":   len(allowedRefs),
		"memory_refs":    len(memoryRefList),
		"guide_strength": strength,
		"coverage":       proposal["coverage"],
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
		proposal["status"] = "publisher_json_malformed"
		proposal["reason_code"] = "publisher_json_malformed"
		trace["reason_code"] = "publisher_json_malformed"
		trace["fail_open"] = true
		trace["accepted_items"] = 0
		trace["rejected_items"] = 0
		return boundedSupervisorEnvelope(proposal), trace
	}

	rawProposal, ok := parsed["supervisor_scene_proposal"].(map[string]any)
	if !ok {
		return buildPublisherSchemaFailure(proposal, trace)
	}
	rawPlan, ok := rawProposal["publisher_plan"].(map[string]any)
	if !ok || extractionStringFromAny(rawPlan["contract_version"]) != "publisher_plan.v2" {
		return buildPublisherSchemaFailure(proposal, trace)
	}
	bookAuthor, bookAuthorOK := rawPlan["book_author"].(map[string]any)
	director, directorOK := rawPlan["director"].(map[string]any)
	if !bookAuthorOK && !directorOK {
		return buildPublisherSchemaFailure(proposal, trace)
	}

	accepted := []map[string]any{}
	rejected := []map[string]any{}
	addRejected := func(path, code string) {
		rejected = append(rejected, map[string]any{"path": path, "code": code})
	}
	publisherRecordUnknownFields(parsed, map[string]struct{}{"supervisor_scene_proposal": {}}, "", addRejected)
	publisherRecordUnknownFields(rawProposal, map[string]struct{}{"publisher_plan": {}}, "supervisor_scene_proposal", addRejected)
	publisherRecordUnknownFields(rawPlan, map[string]struct{}{"contract_version": {}, "book_author": {}, "director": {}}, "supervisor_scene_proposal.publisher_plan", addRejected)
	roles := map[string]map[string]any{}
	if bookAuthorOK {
		roles["book_author"] = bookAuthor
		publisherRecordUnknownFields(bookAuthor, map[string]struct{}{"current_arc": {}, "narrative_goal": {}, "next_beats": {}, "guardrails": {}}, "supervisor_scene_proposal.publisher_plan.book_author", addRejected)
	} else if _, exists := rawPlan["book_author"]; exists {
		addRejected("supervisor_scene_proposal.publisher_plan.book_author", "role_type_invalid")
	} else {
		addRejected("supervisor_scene_proposal.publisher_plan.book_author", "role_missing")
	}
	if directorOK {
		roles["director"] = director
		publisherRecordUnknownFields(director, map[string]struct{}{"scene_mandate": {}, "required_outcomes": {}, "forbidden_moves": {}, "pressure_level": {}}, "supervisor_scene_proposal.publisher_plan.director", addRejected)
	} else if _, exists := rawPlan["director"]; exists {
		addRejected("supervisor_scene_proposal.publisher_plan.director", "role_type_invalid")
	} else {
		addRejected("supervisor_scene_proposal.publisher_plan.director", "role_missing")
	}

	for _, spec := range publisherV2FieldSpecs {
		role, readable := roles[spec.role]
		if !readable {
			continue
		}
		path := "supervisor_scene_proposal.publisher_plan." + spec.role + "." + spec.field
		rawValue, exists := role[spec.field]
		if !exists {
			addRejected(path, "field_missing")
			continue
		}
		if rawValue == nil {
			if spec.isArray {
				addRejected(path, "field_type_invalid")
			}
			continue
		}
		if spec.isArray {
			values, ok := rawValue.([]any)
			if !ok {
				addRejected(path, "field_type_invalid")
				continue
			}
			for index, rawItem := range values {
				publisherAcceptV2Item(rawItem, spec, index, path+fmt.Sprintf("[%d]", index), allowedRefs, &accepted, addRejected)
			}
			continue
		}
		publisherAcceptV2Item(rawValue, spec, 0, path, allowedRefs, &accepted, addRejected)
	}

	status := "ready"
	reasonCode := ""
	switch {
	case len(accepted) > 0 && len(rejected) > 0:
		status = "partial"
		reasonCode = "publisher_plan_partial"
	case len(accepted) == 0 && len(rejected) == 0:
		status = "valid_empty"
		reasonCode = "publisher_valid_empty"
	case len(accepted) == 0:
		status = "publisher_plan_no_valid_items"
		reasonCode = "publisher_plan_no_valid_items"
	}
	plan := map[string]any{
		"contract_version": "publisher_plan.v2",
		"status":           status,
		"reason_code":      nilIfEmpty(reasonCode),
		"authority":        "response_scoped_proposal_only",
		"truth_authority":  false,
		"would_write":      false,
		"accepted_items":   accepted,
		"accepted_count":   len(accepted),
		"rejected_items":   rejected,
		"rejected_count":   len(rejected),
	}
	proposal["status"] = status
	proposal["reason_code"] = nilIfEmpty(reasonCode)
	proposal["publisher_plan"] = plan
	trace["accepted_items"] = len(accepted)
	trace["rejected_items"] = len(rejected)
	trace["reason_code"] = nilIfEmpty(reasonCode)
	return boundedSupervisorEnvelope(proposal), trace
}

func buildPublisherSchemaFailure(proposal, trace map[string]any) (map[string]any, map[string]any) {
	proposal["status"] = "publisher_schema_invalid"
	proposal["reason_code"] = "publisher_schema_invalid"
	trace["reason_code"] = "publisher_schema_invalid"
	trace["fail_open"] = true
	trace["accepted_items"] = 0
	trace["rejected_items"] = 0
	return boundedSupervisorEnvelope(proposal), trace
}

func publisherRecordUnknownFields(object map[string]any, allowed map[string]struct{}, prefix string, reject func(string, string)) {
	keys := make([]string, 0, len(object))
	for key := range object {
		if _, ok := allowed[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		reject(path, "unknown_field")
	}
}

func publisherAcceptV2Item(raw any, spec publisherV2FieldSpec, order int, path string, allowedRefs map[string]struct{}, accepted *[]map[string]any, reject func(string, string)) {
	item, ok := raw.(map[string]any)
	if !ok {
		reject(path, "item_type_invalid")
		return
	}
	allowedFields := map[string]struct{}{"text": {}, "source_refs": {}}
	if spec.pressure {
		allowedFields["level"] = struct{}{}
	}
	publisherRecordUnknownFields(item, allowedFields, path, reject)
	text, ok := item["text"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		reject(path+".text", "text_empty")
		return
	}
	rawRefs, exists := item["source_refs"]
	refs, ok := rawRefs.([]any)
	if !exists || !ok || len(refs) == 0 {
		reject(path+".source_refs", "source_refs_missing")
		return
	}
	validatedRefs := make([]string, 0, len(refs))
	for _, rawRef := range refs {
		ref, ok := rawRef.(string)
		if !ok {
			reject(path+".source_refs", "source_ref_invalid")
			return
		}
		if _, allowed := allowedRefs[ref]; !allowed {
			reject(path+".source_refs", "source_ref_invalid")
			return
		}
		validatedRefs = append(validatedRefs, ref)
	}
	acceptedItem := map[string]any{
		"role":        spec.role,
		"field":       spec.field,
		"order":       order,
		"text":        text,
		"source_refs": validatedRefs,
	}
	if spec.pressure {
		level, ok := item["level"].(string)
		if !ok || (level != "quiet" && level != "low" && level != "medium" && level != "high") {
			reject(path+".level", "pressure_level_invalid")
			return
		}
		acceptedItem["level"] = level
	}
	*accepted = append(*accepted, acceptedItem)
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
	envelope := map[string]any{
		"contract_version": "supervisor_scene_proposal.v3",
		"authority":        "proposal_only",
		"truth_authority":  false,
		"would_write":      false,
		"directive": map[string]any{
			"supervisor_scene_proposal": proposal,
		},
	}
	if plan, ok := proposal["publisher_plan"].(map[string]any); ok {
		envelope["publisher_plan"] = plan
	}
	return envelope
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
	for _, ref := range stringSliceFromAny(executionRefs["delivered_context"]) {
		if ref = strings.TrimSpace(ref); ref != "" {
			allowedMemory[ref] = struct{}{}
		}
	}
	for _, ref := range stringSliceFromAny(executionRefs["lorebook_reference"]) {
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
	for _, raw := range outputFidelityLineageSlice(supportPacket["delivered_context"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if !boolFromAny(item["delivered"]) || strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		if _, allowed := allowedMemory[ref]; allowed {
			memoryRefs = appendUniqueStringValues(memoryRefs, ref)
		}
	}
	for _, raw := range outputFidelityLineageSlice(supportPacket["delivered_lorebook_reference"]) {
		item := mapFromAny(raw)
		refList := stringSliceFromAny(item["source_refs"])
		if strings.TrimSpace(extractionStringFromAny(item["final_text"])) == "" {
			continue
		}
		for _, ref := range refList {
			if _, allowed := allowedMemory[ref]; allowed {
				memoryRefs = appendUniqueStringValues(memoryRefs, ref)
			}
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
