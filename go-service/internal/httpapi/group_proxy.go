package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

var proxyHTTPClient = http.DefaultClient

// registerProxyRoutes mounts supervisor, proxy plugin, and critic endpoints.
func (s *Server) registerProxyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /supervisor", s.handleSupervisor)
	mux.HandleFunc("POST /proxy/plugin-main", s.handleProxyPluginMain)
	mux.HandleFunc("POST /critic/test", s.handleCriticTest)
}

func (s *Server) handleSupervisor(w http.ResponseWriter, r *http.Request) {
	var req dto.SupervisorRequest
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
	narrativeStance := stringPtrValue(req.NarrativeStance, "balanced")
	autoAdvanceTrigger := stringPtrValue(req.AutoAdvanceTrigger, "none")
	wakeUpContext := stringPtrValue(req.WakeUpContext, "")
	persistentGuidance := stringPtrValue(req.PersistentGuidance, "")
	promptTrace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	var storylines []store.Storyline
	storylineReadStatus := "unavailable"
	if s.Store != nil {
		if rows, err := s.Store.ListStorylines(r.Context(), sid); err == nil {
			storylines = rows
			storylineReadStatus = "ok"
		} else if errors.Is(err, store.ErrNotEnabled) {
			storylineReadStatus = "disabled"
		} else {
			storylineReadStatus = "error"
		}
	}
	storylineSelection := selectStorylinesForSupervisor(storylines, nil, 5)
	evidenceCounts := map[string]any{
		"context_messages":            len(req.ContextMessages),
		"wake_up_context_present":     wakeUpContext != "",
		"persistent_guidance_present": persistentGuidance != "",
		"storyline_count":             len(storylines),
		"storyline_selected_count":    len(storylineSelection.Selected),
	}
	sectionSummary := []map[string]any{
		{
			"name":      "supervisor_request_context",
			"chars":     len([]rune(wakeUpContext)) + len([]rune(persistentGuidance)),
			"available": wakeUpContext != "" || persistentGuidance != "" || len(req.ContextMessages) > 0,
			"truncated": false,
			"sources":   []string{"context_messages", "wake_up_context", "persistent_guidance"},
		},
		{
			"name":      "storyline_selection",
			"chars":     len([]rune(formatStorylinesForSupervisor(storylineSelection))),
			"available": len(storylineSelection.Selected) > 0,
			"truncated": false,
			"sources":   []string{"store.storylines"},
		},
	}
	supervisorPack := buildSupervisorInputPack(sid, 0, "", guideMode, "weak", narrativeStance, autoAdvanceTrigger, wakeUpContext, promptTrace, evidenceCounts, sectionSummary, storylineSelection, false, "")
	trace := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	trace["guide_mode"] = guideMode
	trace["guide_suffix_present"] = supervisorPack["guide_suffix"] != ""
	trace["director_overrides"] = supervisorPack["director_overrides"]
	trace["narrative_stance"] = narrativeStance
	trace["narrative_stance_summary"] = supervisorPack["narrative_stance_summary"]
	trace["narrative_stance_suffix_present"] = supervisorPack["narrative_stance_suffix"] != ""
	trace["narrative_stance_bounds_present"] = supervisorPack["narrative_stance_bounds"] != nil
	trace["auto_advance_trigger"] = autoAdvanceTrigger
	trace["wake_up_context_present"] = wakeUpContext != ""
	trace["persistent_guidance_present"] = persistentGuidance != ""
	trace["context_messages_count"] = len(req.ContextMessages)
	trace["storyline_read_status"] = storylineReadStatus
	trace["storyline_selection"] = supervisorPack["storyline_selection"]
	trace["would_call_llm"] = false
	trace["would_write"] = false
	llmCfg := s.supervisorLLMConfig()
	if llmCfg.hasConfig() {
		result, llmTrace, err := s.runSupervisorLLM(r.Context(), sid, supervisorPack, req, llmCfg)
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
	guideSuffix := stringPtrValue(req.GuideSuffix, "")
	if guideSuffix == "" {
		if v, ok := supervisorPack["guide_suffix"].(string); ok {
			guideSuffix = v
		}
	}
	systemPromptForCall := systemPrompt
	if strings.TrimSpace(guideSuffix) != "" {
		systemPromptForCall = strings.TrimRight(systemPromptForCall, "\n") + "\n" + guideSuffix
	}
	narrativeStance := stringPtrValue(req.NarrativeStance, "balanced")
	narrativeStanceSuffix := stringPtrValue(req.NarrativeStanceSuffix, "")
	if narrativeStanceSuffix == "" {
		if v, ok := supervisorPack["narrative_stance_suffix"].(string); ok {
			narrativeStanceSuffix = v
		}
	}
	narrativeStanceBounds := req.NarrativeStanceBounds
	if len(narrativeStanceBounds) == 0 {
		if v, ok := supervisorPack["narrative_stance_bounds"].(map[string]any); ok {
			narrativeStanceBounds = v
		}
	}
	if strings.TrimSpace(narrativeStanceSuffix) != "" {
		systemPromptForCall = strings.TrimRight(systemPromptForCall, "\n") + "\n" + narrativeStanceSuffix
	}
	if len(narrativeStanceBounds) > 0 {
		systemPromptForCall = strings.TrimRight(systemPromptForCall, "\n") + "\n[Story Initiative Bounds]\n" + compactJSONForShadow(narrativeStanceBounds, 600)
	}
	momentumSuffix := formatMomentumSuffix(req.MomentumPacket)
	if strings.TrimSpace(momentumSuffix) != "" {
		systemPromptForCall = strings.TrimRight(systemPromptForCall, "\n") + "\n" + momentumSuffix
	}
	payload := map[string]any{
		"chat_session_id":         sid,
		"guide_mode":              guideMode,
		"narrative_stance":        narrativeStance,
		"auto_advance_trigger":    stringPtrValue(req.AutoAdvanceTrigger, "none"),
		"wake_up_context":         stringPtrValue(req.WakeUpContext, ""),
		"persistent_guidance":     stringPtrValue(req.PersistentGuidance, ""),
		"context_messages":        req.ContextMessages,
		"momentum_packet":         req.MomentumPacket,
		"narrative_stance_bounds": narrativeStanceBounds,
		"narrative_stance_suffix": narrativeStanceSuffix,
		"guide_suffix":            guideSuffix,
		"director_overrides":      supervisorPack["director_overrides"],
		"supervisor_input_pack":   supervisorPack,
		"required_output":         "Return only JSON. Include directive/director/book_author/section_world fields when applicable.",
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
		Messages:    []any{map[string]any{"role": "system", "content": systemPromptForCall}, map[string]any{"role": "user", "content": string(userPromptBytes)}},
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		TimeoutMs:   &cfg.TimeoutMs,
	}
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
	return parsed, trace, nil
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

	if err := ValidateProxyEndpoint(endpoint); err != nil {
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
