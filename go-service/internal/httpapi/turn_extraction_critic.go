package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

var (
	criticAuthorizationSecretPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^\s,;}\]]+`)
	criticBearerSecretPattern        = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	criticJSONSecretPattern          = regexp.MustCompile(`(?i)("(?:x-api-key|api[_-]?key|password|client_secret|access_token|refresh_token)"\s*:\s*)"[^"]*"`)
	criticKVSecretPattern            = regexp.MustCompile(`(?i)((?:x-api-key|api[_-]?key|password|client_secret|access_token|refresh_token)\s*[:=]\s*)[^\s,;}\]]+`)
	criticClaimStopTokens            = map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "but": {},
		"for": {}, "from": {}, "he": {}, "her": {}, "his": {}, "in": {}, "is": {},
		"it": {}, "of": {}, "on": {}, "or": {}, "she": {}, "that": {}, "the": {},
		"their": {}, "they": {}, "this": {}, "to": {}, "was": {}, "were": {}, "with": {},
		"그": {}, "그가": {}, "그녀": {}, "그는": {}, "그것": {}, "그의": {}, "이것": {}, "저것": {},
	}
)

type criticPipelineError struct {
	Code       string
	Stage      string
	Retryable  bool
	HTTPStatus int
	Cause      error
}

func (e *criticPipelineError) Error() string {
	if e == nil {
		return "critic pipeline failed"
	}
	if e.Cause == nil {
		return e.Code
	}
	return e.Code + ": " + e.Cause.Error()
}

func (e *criticPipelineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newCriticPipelineError(code, stage string, retryable bool, httpStatus int, cause error) *criticPipelineError {
	return &criticPipelineError{
		Code:       strings.TrimSpace(code),
		Stage:      strings.TrimSpace(stage),
		Retryable:  retryable,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}

func criticPipelineErrorDetails(err error) map[string]any {
	var pipelineErr *criticPipelineError
	if !errors.As(err, &pipelineErr) || pipelineErr == nil {
		return map[string]any{
			"code":      "CRITIC_UNKNOWN_FAILED",
			"stage":     "unknown",
			"retryable": true,
		}
	}
	out := map[string]any{
		"code":      pipelineErr.Code,
		"stage":     pipelineErr.Stage,
		"retryable": pipelineErr.Retryable,
	}
	if pipelineErr.HTTPStatus > 0 {
		out["http_status"] = pipelineErr.HTTPStatus
	}
	return out
}

func classifyCriticProviderError(err error, status int) *criticPipelineError {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newCriticPipelineError("CRITIC_PROVIDER_TIMEOUT", "provider_call", true, status, err)
	case errors.Is(err, context.Canceled):
		return newCriticPipelineError("CRITIC_PROVIDER_CANCELED", "provider_call", false, status, err)
	}
	var emptyContentErr *proxyEmptyContentError
	if errors.As(err, &emptyContentErr) {
		return newCriticPipelineError("CRITIC_EMPTY_RESPONSE", "provider_response", true, status, err)
	}
	var localRequestErr *proxyLocalRequestError
	if errors.As(err, &localRequestErr) {
		stage := strings.TrimSpace(localRequestErr.Stage)
		code := "CRITIC_REQUEST_BUILD_FAILED"
		if stage == "configuration" {
			code = "CRITIC_CONFIG_INVALID"
		}
		if stage == "" {
			stage = "request_build"
		}
		return newCriticPipelineError(code, stage, false, 0, err)
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		if networkErr.Timeout() {
			return newCriticPipelineError("CRITIC_PROVIDER_TIMEOUT", "provider_call", true, status, err)
		}
		return newCriticPipelineError("CRITIC_PROVIDER_CALL_FAILED", "provider_call", true, status, err)
	}
	if status >= http.StatusBadRequest {
		retryable := status == http.StatusRequestTimeout ||
			status == http.StatusTooEarly ||
			status == http.StatusTooManyRequests ||
			status >= http.StatusInternalServerError
		return newCriticPipelineError("CRITIC_PROVIDER_HTTP_ERROR", "provider_response", retryable, status, err)
	}
	return newCriticPipelineError("CRITIC_PROVIDER_CALL_FAILED", "provider_call", true, status, err)
}

func criticFailureTrace(promptSource string, cfg completeTurnLLMConfig, status int, err error, content string) map[string]any {
	trace := map[string]any{
		"prompt_source": promptSource,
		"provider":      strings.TrimSpace(cfg.Provider),
		"model":         strings.TrimSpace(cfg.Model),
	}
	for key, value := range criticPipelineErrorDetails(err) {
		trace[key] = value
	}
	if status > 0 {
		trace["http_status"] = status
	}
	preview := strings.TrimSpace(scrubCriticFailureText(content, cfg.APIKey))
	if preview == "" && err != nil {
		preview = scrubCriticFailureText(err.Error(), cfg.APIKey)
	}
	if preview != "" {
		trace["raw_preview"] = truncateRunes(preview, 1000)
	}
	return trace
}

func scrubCriticFailureText(text, apiKey string) string {
	out := text
	if key := strings.TrimSpace(apiKey); key != "" {
		out = strings.ReplaceAll(out, key, "[redacted]")
	}
	out = criticAuthorizationSecretPattern.ReplaceAllString(out, `${1}[redacted]`)
	out = criticBearerSecretPattern.ReplaceAllString(out, "Bearer [redacted]")
	out = criticJSONSecretPattern.ReplaceAllString(out, `${1}"[redacted]"`)
	out = criticKVSecretPattern.ReplaceAllString(out, `${1}[redacted]`)
	return out
}

func (s *Server) runCompleteTurnCritic(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, cfg completeTurnLLMConfig, languageContextArg ...map[string]any) (map[string]any, map[string]any, error) {
	return s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userInput, assistantContent, contextMessages, outputLanguageOverride, cfg, false, languageContextArg...)
}

func (s *Server) runCompleteTurnCriticFromCanonicalLogs(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	return s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userInput, assistantContent, nil, nil, cfg, true)
}

func (s *Server) runCompleteTurnCriticWithInputPolicy(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, cfg completeTurnLLMConfig, canonicalChatLogs bool, languageContextArg ...map[string]any) (map[string]any, map[string]any, error) {
	if !cfg.hasConfig() {
		err := newCriticPipelineError("CRITIC_CONFIG_MISSING", "configuration", false, 0, errors.New("critic_config_missing"))
		return nil, criticFailureTrace("", cfg, 0, err, ""), err
	}
	var languageContext map[string]any
	if len(languageContextArg) > 0 {
		languageContext = normalizeCompleteTurnLanguageContext(languageContextArg[0])
	}
	systemPrompt, promptSource := readCriticSystemPrompt(s.Cfg.PromptDir)
	sanitizedUserInput := ""
	sanitizedAssistantContent := ""
	if canonicalChatLogs {
		sanitizedUserInput = sanitizeCriticStorageText(userInput)
		sanitizedAssistantContent = sanitizeCriticStorageText(assistantContent)
	} else {
		sanitizedUserInput = sanitizeTextForCriticInput(userInput)
		sanitizedAssistantContent = sanitizeTextForCriticInput(assistantContent)
	}
	safeUserInput := boundCompleteTurnCriticInput(sanitizedUserInput, 0)
	safeAssistantContent := boundCompleteTurnCriticInput(sanitizedAssistantContent, 0)
	if strings.TrimSpace(safeUserInput+"\n"+safeAssistantContent) == "" {
		err := newCriticPipelineError("CRITIC_INPUT_EMPTY", "input", false, 0, errors.New("critic_input_empty_after_sanitize"))
		trace := criticFailureTrace(promptSource, cfg, 0, err, "")
		trace["source_aware_ingest_guard"] = !canonicalChatLogs
		trace["canonical_chat_logs"] = canonicalChatLogs
		return nil, trace, err
	}
	safeContextMessages := sanitizeContextMessagesForCriticInput(contextMessages)
	previewPass := s.buildCompleteTurnCriticPreviewPass(ctx, sid, turnIndex, safeContextMessages, safeUserInput, safeAssistantContent)
	criticArchiveLedgerPromptInput, criticArchiveLedgerTrace := s.buildCompleteTurnCriticArchiveLedgerInput(ctx, sid, turnIndex, safeAssistantContent, outputLanguageOverride)
	userPrompt := buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, safeUserInput, safeAssistantContent, safeContextMessages, outputLanguageOverride, previewPass, languageContext, criticArchiveLedgerPromptInput)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&req, cfg)
	jsonPolicy := proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"}

	upstream, upstreamStatus, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, cfg.RetryBudget, jsonPolicy)
	providerRetryTrace := map[string]any{}
	if err != nil {
		providerErr := classifyCriticProviderError(err, upstreamStatus)
		firstFailureTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, providerErr, "")
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			firstFailureTrace["request_overrides"] = requestOverrides
		}
		retryUserInput, userRedacted := redactSensitiveCriticRetryText(safeUserInput)
		retryAssistantContent, assistantRedacted := redactSensitiveCriticRetryText(safeAssistantContent)
		if !userRedacted && !assistantRedacted {
			return nil, firstFailureTrace, providerErr
		}
		if !cfg.RetryBudget.take() {
			return nil, firstFailureTrace, providerErr
		}
		retryPreviewPass := s.buildCompleteTurnCriticPreviewPass(ctx, sid, turnIndex, safeContextMessages, retryUserInput, retryAssistantContent)
		retryPrompt := buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, retryUserInput, retryAssistantContent, safeContextMessages, outputLanguageOverride, retryPreviewPass, languageContext, criticArchiveLedgerPromptInput)
		retryReq := req
		retryReq.Messages = []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": retryPrompt}}
		retryUpstream, retryStatus, retryErr := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, retryReq, cfg.RetryBudget, jsonPolicy)
		providerRetryTrace = map[string]any{
			"mode":                "sensitive_input_redacted_retry",
			"user_input_redacted": userRedacted,
			"assistant_redacted":  assistantRedacted,
			"first_failure":       firstFailureTrace,
			"retry_preview_pass":  retryPreviewPass,
		}
		if retryErr != nil {
			retryPipelineErr := classifyCriticProviderError(retryErr, retryStatus)
			retryFailureTrace := criticFailureTrace(promptSource, cfg, retryStatus, retryPipelineErr, "")
			if requestOverrides := mapFromAny(retryUpstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
				providerRetryTrace["request_overrides"] = requestOverrides
			}
			providerRetryTrace["retry_failure_recorded"] = "top_level"
			retryFailureTrace["provider_retry"] = providerRetryTrace
			return nil, retryFailureTrace, retryPipelineErr
		}
		upstream = retryUpstream
		upstreamStatus = retryStatus
		previewPass = retryPreviewPass
		safeUserInput = retryUserInput
		safeAssistantContent = retryAssistantContent
	}
	content := chatCompletionText(upstream)
	if strings.TrimSpace(content) == "" {
		emptyErr := newCriticPipelineError("CRITIC_EMPTY_RESPONSE", "provider_response", true, upstreamStatus, errors.New("critic provider returned no assistant content"))
		trace := criticFailureTrace(promptSource, cfg, upstreamStatus, emptyErr, "")
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			trace["request_overrides"] = requestOverrides
		}
		if len(providerRetryTrace) > 0 {
			trace["provider_retry"] = providerRetryTrace
		}
		return nil, trace, emptyErr
	}
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		code := "CRITIC_JSON_PARSE_FAILED"
		if strings.Contains(err.Error(), "critic_json_missing") {
			code = "CRITIC_JSON_MISSING"
		}
		parseErr := newCriticPipelineError(code, "json_parse", true, upstreamStatus, err)
		parseTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, parseErr, content)
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			parseTrace["request_overrides"] = requestOverrides
		}
		return nil, parseTrace, parseErr
	}
	if err := validateCriticExtractionSchema(parsed); err != nil {
		schemaErr := newCriticPipelineError("CRITIC_SCHEMA_INVALID", "schema_validation", true, upstreamStatus, err)
		schemaTrace := criticFailureTrace(promptSource, cfg, upstreamStatus, schemaErr, content)
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			schemaTrace["request_overrides"] = requestOverrides
		}
		return nil, schemaTrace, schemaErr
	}
	parsed, quarantineTrace := quarantineCriticProtectedCandidates(parsed, safeUserInput, safeAssistantContent)
	trace := map[string]any{
		"prompt_source": promptSource,
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"provider":      strings.TrimSpace(cfg.Provider),
		"http_status":   upstreamStatus,
		"usage":         upstream["usage"],
		"input_budget": map[string]any{
			"user_input_chars":        len([]rune(safeUserInput)),
			"assistant_content_chars": len([]rune(safeAssistantContent)),
			"user_input_bounded":      len([]rune(sanitizedUserInput)) > len([]rune(safeUserInput)),
			"assistant_bounded":       len([]rune(sanitizedAssistantContent)) > len([]rune(safeAssistantContent)),
		},
		"pipeline": map[string]any{
			"policy_version": completeTurnCriticPipelineVersion,
			"stages": map[string]any{
				"evidence_extractor": map[string]any{
					"status":                 "ok",
					"owner":                  "complete_turn.configured_critic_extract",
					"preview_policy_version": completeTurnCriticPreviewPassVersion,
					"preview_seed_applied":   true,
				},
				"deterministic_reducer": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.normalizeCriticExtraction",
				},
				"focused_recall_enricher": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.enrichNormalizedCriticExtractionForFocusedRecall",
				},
				"summary_compactor_background": map[string]any{
					"status": "handoff",
					"owner":  "complete_turn.maintenance_handoff",
				},
			},
		},
		"preview_pass": previewPass,
	}
	if len(quarantineTrace) > 0 {
		trace["protected_candidate_quarantine"] = quarantineTrace
	}
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	trace["critic_archive_ledger"] = criticArchiveLedgerTrace
	if len(languageContext) > 0 {
		trace["language_context"] = languageContext
		trace["memory_write_contract"] = completeTurnMemoryWriteContract(languageContext)
	}
	if len(providerRetryTrace) > 0 {
		trace["provider_retry"] = providerRetryTrace
	}
	normalized := normalizeCriticExtraction(parsed)
	if len(worldRuleItemsForSave(normalized)) == 0 && (cfg.ForceWorldRuleAudit || shouldRunFocusedWorldRuleAudit(normalized)) {
		auditedRules, auditTrace := s.runCompleteTurnWorldRuleAudit(ctx, sid, turnIndex, safeUserInput, safeAssistantContent, safeContextMessages, previewPass, normalized, cfg)
		trace["world_rule_audit"] = auditTrace
		if len(worldRuleItemsForSave(auditedRules)) > 0 {
			var mergedCount int
			normalized, mergedCount = mergeWorldRuleAuditIntoExtraction(normalized, auditedRules)
			auditTrace["merged_world_rule_count"] = mergedCount
		}
	} else if len(worldRuleItemsForSave(normalized)) > 0 {
		trace["world_rule_audit"] = map[string]any{
			"status": "skipped",
			"reason": "initial_extraction_has_world_rules",
		}
	} else {
		reason := "initial_audit_did_not_request_focused_world_rule_pass"
		if cfg.ForceWorldRuleAudit {
			reason = "force_world_rule_audit_configured_but_not_reached"
		}
		trace["world_rule_audit"] = map[string]any{
			"status": "skipped",
			"reason": reason,
		}
	}
	normalized = enrichNormalizedCriticExtractionForFocusedRecall(normalized, safeUserInput, safeAssistantContent, turnIndex)
	normalized = applyLanguageMemoryWriteContract(normalized, languageContext)
	return normalized, trace, nil
}

func shouldRunFocusedWorldRuleAudit(extraction map[string]any) bool {
	audit := mapFromAny(extraction["world_rule_audit"])
	if len(audit) == 0 {
		audit = mapFromAny(extraction["world_rules_audit"])
	}
	if len(audit) == 0 {
		return false
	}
	for _, key := range []string{"durable_rule_found", "rule_found", "needs_world_rule", "audit_positive"} {
		if boolFromAny(audit[key]) {
			return true
		}
	}
	status := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(audit, "status"),
		stringFromMap(audit, "verdict"),
		stringFromMap(audit, "decision"),
	)))
	return status == "positive" || status == "found" || status == "needs_world_rule"
}

func (s *Server) runCompleteTurnWorldRuleAudit(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, previewPass map[string]any, initialExtraction map[string]any, cfg completeTurnLLMConfig) (map[string]any, map[string]any) {
	trace := map[string]any{
		"status":           "skipped",
		"policy_version":   "world_rule_audit.v1",
		"llm_call_attempt": false,
	}
	if !cfg.hasConfig() {
		trace["reason"] = "critic_config_missing"
		return nil, trace
	}
	if strings.TrimSpace(userInput+"\n"+assistantContent) == "" {
		trace["reason"] = "empty_turn"
		return nil, trace
	}
	prompt := buildCompleteTurnWorldRuleAuditPrompt(sid, turnIndex, userInput, assistantContent, contextMessages, previewPass, initialExtraction)
	maxTokens := cfg.MaxTokens
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": "You are Archive Center's world-rule audit extractor. Return only valid JSON. Do not use markdown fences."}, map[string]any{"role": "user", "content": prompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&req, cfg)
	trace["llm_call_attempt"] = true
	upstream, _, err := performProxyPluginMainWithRetryBudgetAndPolicy(ctx, req, cfg.RetryBudget, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_world_rule_audit"})
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
			trace["request_overrides"] = requestOverrides
		}
		return nil, trace
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		trace["raw_preview"] = truncateRunes(content, 1000)
		return nil, trace
	}
	normalized := normalizeCriticExtraction(parsed)
	count := len(worldRuleItemsForSave(normalized))
	trace["status"] = "ok"
	trace["model"] = extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model)
	trace["usage"] = upstream["usage"]
	if requestOverrides := mapFromAny(upstream["_proxy_request_overrides"]); len(requestOverrides) > 0 {
		trace["request_overrides"] = requestOverrides
	}
	trace["world_rule_count"] = count
	if count == 0 {
		trace["reason"] = extractionFirstNonEmpty(stringFromMap(mapFromAny(parsed["audit"]), "reason"), "audit_returned_no_durable_rule")
	}
	return normalized, trace
}

func buildCompleteTurnWorldRuleAuditPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, previewPass map[string]any, initialExtraction map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	preview, _ := json.Marshal(previewPass)
	initial, _ := json.Marshal(initialExtraction)
	return strings.Join([]string{
		"Audit whether the completed turn establishes durable world rules that the main extraction missed.",
		"Return ONLY JSON. Do not use markdown fences.",
		"Use this JSON shape:",
		`{"audit":{"durable_rule_found":false,"reason":""},"world_rules":[],"world_state":{"version":"world_state.v1","confidence":0,"verification":"","rules":[]}}`,
		"Decision contract:",
		"- This is an AI judgement step. Do not rely on keyword lists, genre names, or instruction examples as facts.",
		"- Extract the abstract invariant established by the session's own evidence.",
		"- A world rule is a durable constraint that should remain true after this exchange: physical/natural law, supernatural or technology mechanic, progression or reward economy, acquisition method, access gate, location constraint, social law, institution/custom, faction norm, rank/authority rule, contract, resource/logistics limit, schedule/calendar rule, taboo, or equivalent stable setting law.",
		"- Creation myths, cosmology, divine non-intervention rules, origin rules for monsters/threats, granted powers, chosen-agent roles, sacred/institutional authority, and stable religious doctrine are world rules when the turn presents them as setting truth rather than rumor or metaphor.",
		"- It can appear in any genre: academy, workplace, household, romance, survival, fantasy, dungeon/progression, sci-fi, political, slice-of-life, or apocalypse.",
		"- If the latest turn only has a temporary action, mood, one-off dialogue, rejected plan, speculation, or private thought with no durable setting constraint, return empty arrays.",
		"- If the latest turn confirms a durable rule, world_rules must not be empty. Emit compact evidence-bound rules with scope, category, key, value, and optional scope_name/genre/confidence/verification.",
		"- Use the canonical scope vocabulary exactly: root, region, location, faction, system, session.",
		"- Scope guidance: root=universal cosmology or setting-wide law; region=named country/city/territory/large area; location=concrete place/base/building/dungeon/site; faction=organization/church/guild/government/gang/party/team; system=magic/technology/progression/economy/combat/reward mechanics; session=temporary session-only plan or rule without a more specific stable scope.",
		"- Do not put named regions, named locations, named factions, or progression mechanics under root just because they are important. Use their specific scope and scope_name.",
		"- Mirror the same durable rules in world_state.rules when they shape the current setting state.",
		"- Do not invent mechanics. If uncertain, use audit.reason and return empty arrays.",
		"",
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Deterministic_Preview_Pass_JSON>",
		string(preview),
		"</Deterministic_Preview_Pass_JSON>",
		"",
		"<Initial_Critic_Extraction_JSON>",
		string(initial),
		"</Initial_Critic_Extraction_JSON>",
	}, "\n")
}

func mergeWorldRuleAuditIntoExtraction(base map[string]any, audit map[string]any) (map[string]any, int) {
	items := worldRuleItemsForSave(audit)
	if len(items) == 0 {
		return base, 0
	}
	out := make(map[string]any, len(base)+2)
	for k, v := range base {
		out[k] = v
	}
	out["world_rules"] = append(sliceFromAny(out["world_rules"]), items...)
	ws := mapFromAny(out["world_state"])
	if len(ws) == 0 {
		ws = map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.85,
			"verification": "verified_by_world_rule_audit",
		}
	}
	ws["rules"] = append(sliceFromAny(ws["rules"]), items...)
	if strings.TrimSpace(stringFromMap(ws, "version")) == "" {
		ws["version"] = "world_state.v1"
	}
	if strings.TrimSpace(stringFromMap(ws, "verification")) == "" {
		ws["verification"] = "verified_by_world_rule_audit"
	}
	out["world_state"] = ws
	return out, len(worldRuleItemsForSave(out))
}

func (s *Server) buildCompleteTurnCriticArchiveLedgerInput(ctx context.Context, sid string, turnIndex int, assistantContent string, outputLanguageOverride *map[string]any) (map[string]any, map[string]any) {
	trace := map[string]any{
		"enabled":          s != nil && s.Cfg.CriticLedgerEnabled,
		"included":         false,
		"contract_version": criticArchiveLedgerContractVersion,
	}
	if s == nil || !s.Cfg.CriticLedgerEnabled {
		trace["status"] = "disabled"
		return nil, trace
	}
	req := criticArchiveLedgerPreviewRequest{
		ChatSessionID:          sid,
		TurnIndex:              turnIndex,
		AssistantFinalText:     assistantContent,
		AssistantFinalLanguage: completeTurnAssistantFinalLanguage(outputLanguageOverride),
		StreamingMismatch:      "unknown",
	}
	resp := s.buildCriticArchiveLedgerPreviewWithContext(ctx, req)
	promptInput := criticArchiveLedgerPromptInput(resp)
	trace["included"] = true
	trace["status"] = resp.Status
	trace["item_count"] = len(resp.Items)
	trace["vector_status"] = resp.VectorStatus
	trace["language"] = resp.Language
	trace["safety"] = resp.Safety
	trace["degraded"] = resp.Degraded
	trace["warnings"] = resp.Warnings
	trace["write_attempted"] = resp.WriteAttempted
	trace["vector_write_attempted"] = resp.VectorWriteAttempted
	trace["llm_call_attempted"] = resp.LLMCallAttempted
	return promptInput, trace
}

func completeTurnAssistantFinalLanguage(outputLanguageOverride *map[string]any) string {
	if outputLanguageOverride == nil || *outputLanguageOverride == nil {
		return ""
	}
	for _, key := range []string{"language", "lang", "target_language", "output_language"} {
		if value, ok := (*outputLanguageOverride)[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func criticArchiveLedgerPromptInput(resp criticArchiveLedgerPreviewResponse) map[string]any {
	items := make([]map[string]any, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, map[string]any{
			"lane":       item.Lane,
			"id":         item.ID,
			"authority":  item.Authority,
			"status":     item.Status,
			"summary":    item.Summary,
			"updated_at": item.UpdatedAt,
			"source_ref": item.SourceRef,
		})
	}
	return map[string]any{
		"contract_version":         resp.ContractVersion,
		"status":                   resp.Status,
		"session_id":               resp.SessionID,
		"runtime_profile":          resp.RuntimeProfile,
		"store_mode":               resp.StoreMode,
		"vector_status":            resp.VectorStatus,
		"language":                 resp.Language,
		"limits":                   resp.Limits,
		"counts":                   resp.Counts,
		"safety":                   resp.Safety,
		"degraded":                 resp.Degraded,
		"warnings":                 resp.Warnings,
		"items":                    items,
		"read_only":                true,
		"write_attempted":          false,
		"vector_write_attempted":   false,
		"llm_call_attempted":       false,
		"raw_archive_dump_blocked": true,
		"usage_policy":             "support_only_do_not_copy_as_new_evidence_without_latest_turn_support",
	}
}

func readCriticSystemPrompt(configuredDir string) (string, string) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "critic_system.txt"))
	}
	candidates = append(candidates,
		filepath.Join("..", "prompts", "critic_system.txt"),
		filepath.Join("prompts", "critic_system.txt"),
		filepath.Join("..", "..", "prompts", "critic_system.txt"),
	)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), path
		}
	}
	return "You are Archive Center's critic extractor. Return only valid JSON matching the configured critic schema.", "fallback_builtin"
}

func readSupervisorSystemPrompt(configuredDir string) (string, string) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "supervisor_system.txt"))
	}
	candidates = append(candidates,
		filepath.Join("..", "prompts", "supervisor_system.txt"),
		filepath.Join("prompts", "supervisor_system.txt"),
		filepath.Join("..", "..", "prompts", "supervisor_system.txt"),
		filepath.Join("..", "..", "..", "prompts", "supervisor_system.txt"),
	)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), path
		}
	}
	return "You are Archive Center's source-backed narrative support reviewer. Return only valid JSON matching supervisor_scene_proposal.v3 with fidelity_warnings and typed expression_hints. Fidelity warnings and callbacks require delivered-memory support; portrayal, pacing, scene emphasis, and reversible options require current-input or delivered-memory support. Every item is optional, proposal-only, non-canonical, and cannot decide user actions, new facts or knowledge, relationship changes, event closure, or scene jumps.", "fallback_builtin"
}

func buildCompleteTurnCriticPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, previewPass map[string]any, archiveLedger ...map[string]any) string {
	return buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, userInput, assistantContent, contextMessages, outputLanguageOverride, previewPass, nil, archiveLedger...)
}

func buildCompleteTurnCriticPromptWithLanguageContext(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, previewPass map[string]any, languageContext map[string]any, archiveLedger ...map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	lang, _ := json.Marshal(outputLanguageOverride)
	langCtx, _ := json.Marshal(normalizeCompleteTurnLanguageContext(languageContext))
	preview, _ := json.Marshal(previewPass)
	var ledgerInput any
	if len(archiveLedger) > 0 && archiveLedger[0] != nil {
		ledgerInput = archiveLedger[0]
	}
	ledger, _ := json.Marshal(ledgerInput)
	return strings.Join([]string{
		"Extract durable Archive Center memory data from the completed turn.",
		"Return ONLY JSON. Do not use markdown fences.",
		"Use this JSON shape. Omit unknown facts instead of inventing placeholders:",
		`{"turn_summary":"","importance_score":5,"evidence_excerpts":[],"story_clock":{"version":"story_clock.v1","observation_kind":"absolute","scene_scope":"current","precision":"exact","absolute":{"date":"1423-04-12","time":"13:00"},"evidence_excerpt":"exact latest-turn excerpt","transition":"set"},"kg_triples":[],"entities":{"characters":[],"locations":[],"items":[]},"speaker_attributions":[],"relationship_memory":{},"state_deltas":{},"character_deltas":[],"physical_conditions":[],"entity_conditions":[],"pending_threads":[],"world_rule_audit":{"durable_rule_found":false,"reason":""},"world_rules":[],"world_state":{"version":"world_state.v1","confidence":0,"verification":"","rules":[]},"subjective_entity_memories":[],"protected_secrets":[],"character_identity_accuracy":[],"persona_capsule_candidates":[],"narrative_events":[],"state_claims":[],"belief_updates":[],"archive_hint":{}}`,
		"Rules:",
		"- Sensitivity policy: if the latest turn contains concrete in-story action, decision, relationship shift, promise, threat, injury, plan/resource, location movement, authority change, world constraint, or unresolved tension, extract it. Empty arrays are valid only for pure OOC/meta, repetition, or no new in-story information.",
		"- Prefer several small focused records over one vague memory. Aim to cover the user's intent, the assistant's visible outcome, affected named actors, and durable consequences without inventing anything beyond the latest turn and safe context.",
		"- evidence_excerpts must be short exact excerpts from the latest user/assistant turn, not the whole turn.",
		"- Language contract: use Language_Context_JSON as the memory-write contract. If summary_language/session_output_language is ko, en, or ja, generated natural-language memory fields must use that language. Do not default to English just because these instructions are English. Raw evidence excerpts must stay exact source text and must not be translated or rewritten.",
		"- Apply the same language contract to all generated support fields, including turn_summary, pending_threads titles/details, world_rules key/value/display text, world_state rule values, subjective_entity_memories, protected_secrets summaries, physical/entity condition labels, and storyline/continuity-hook style text. Proper nouns and exact evidence quotes may remain in their original language.",
		"- If the latest user input language differs from session_output_language, do not follow the user input language for generated summaries or support records. Follow session_output_language and preserve user text only inside exact raw evidence excerpts.",
		"- Keep internal enum/category/predicate keys stable. Do not translate system keys per turn just because the output language changes.",
		"- For ordinary narrative turns with new information, include 1-3 evidence_excerpts that ground the most important user intent and assistant outcome.",
		"- kg_triples must use real in-story names only. Never use char_*, cid_*, turn_*, user, assistant, system, prompt, or has_turn edges.",
		"- For ordinary narrative turns with named actors, emit kg_triples for durable relations, assignments, locations, ownership, promises, threats, injuries, permissions, commands, faction links, or plan participation.",
		"- entities.characters/locations/items should contain only concrete in-story people, places, or objects observed in this turn.",
		"- speaker_attributions is optional and source-bound. Each item needs speaker_name when known, attribution_kind=dialogue|quoted_speech|thought|narration|unknown, attribution_state=linked|tentative|ambiguous|unknown, confidence, and a short exact evidence_excerpt from the latest turn. Never guess a speaker from style alone; use ambiguous or unknown when multiple speakers fit.",
		"- Separate location/time fact classes. A current scene location or current scene time belongs in state_deltas.scene_state; a durable residence, hometown, birthplace, workplace, or affiliation belongs in character_deltas.status and/or kg_triples with predicates such as residence, hometown, lives_in, or based_in.",
		"- Do not treat 'X lives in London' as 'the current scene is London'. Do not treat a temporary visit as a durable residence unless the latest turn says it directly.",
		"- Story calendar facts such as 'summer vacation has started' belong in world_state/time_state or state_deltas.scene_state.time_state when they anchor the current scene. Do not infer an immediate return to school, a season change, or a day jump without direct evidence.",
		"- story_clock is optional and proposal-only. Emit it only when the latest completed turn contains an exact supporting excerpt about story time, sequence, or duration; repeat that excerpt in top-level evidence_excerpts.",
		"- story_clock.observation_kind must be absolute|partial|relative|bounded_range|unknown and scene_scope must be current|flashback|planned|hypothetical. Keep absolute date/time, partial daypart/season, relative offset+unit+anchor, bounded range start/end, sequence, and duration structurally separate.",
		"- story_clock.precision must be exact|partial|bounded_range|unknown. Never turn server time, audit time, turn_index, or an unknown/relative phrase without a current story-clock anchor into an exact story date.",
		"- Use only the primary object matching observation_kind: absolute, partial, relative, or range. sequence may use relation/anchor/index/label; duration may use value or min/max with unit and approximate. Do not emit contradictory primary objects together.",
		"- flashback, planned, and hypothetical observations describe non-current time and must not be presented as the current scene clock. Use transition=set|advance|correction|reaffirm|supersede|retract; correction, supersession, and retraction require exact latest-turn evidence.",
		"- relationship_memory may include target_name or pair when trust changes. If no target exists, leave it empty.",
		"- character_deltas should capture named character status, location, emotional posture, relationship changes, injuries, intentions, or role/authority changes seen in the latest turn.",
		"- Separate narrative_events (what happened), state_claims (objective current facts), and belief_updates (one character's current perception). Do not promote beliefs to objective truth.",
		"- state_claims and belief_updates use stable state_slot keys and transition=set|reaffirm|change|reversal|recovery|correction|reveal|resolve|uncertain|clear|defer|abandon|complete|supersede|reopen|resume. Turn is audit order, not semantic authority.",
		"- For goal or thread lifecycle state_claims, use the exact goal or thread title as subject, subject_type=entity, and state_slot=goal_status. Do not use goal_status for another entity-state dimension.",
		"- When that goal or thread is also emitted in pending_threads or state_deltas unresolved_threads.opened, include the same exact title and subject plus state_slot=goal_status in that open record.",
		"- Use reopen or resume only when the latest completed turn explicitly reactivates a state previously deferred, abandoned, completed, superseded, resolved, or cleared. Use reversal only for a directly evidenced state inversion. A suggestion, condition, possibility, or proposal is uncertain and must not replace an existing current value.",
		"- Every narrative_events/state_claims/belief_updates item requires a short exact evidence_excerpt from the latest completed turn. Omit unsupported items.",
		"- Also repeat each accepted event/state/belief evidence_excerpt in top-level evidence_excerpts so current values and change events can link to direct evidence.",
		"- physical_conditions is for evidence-bound body/health continuity that can affect roleplay: illness, fever, cold, pregnancy, menstruation, poisoning, fracture, accident/fall injury, body damage, impairment, missing body part, recovery, worsening, or cleared condition.",
		"- Each physical_conditions item should include owner_entity_name or owner_entity_key, condition_label, effect_kind when obvious (temporary_effect or injury), evidence_excerpt, source_turn_index, and may include severity_text, body_area, onset_story_clock_json, duration_json, expires_at_clock_json, prognosis_text, age_or_vulnerability_note, uncertainty_note, and authority_hint.",
		"- Do not invent medical calendars, fixed cycles, healing times, or numeric severity values. If duration is not explicit in the latest turn or safe context, use duration_policy=unknown_until_updated and keep prognosis_text/age_or_vulnerability_note descriptive.",
		"- Do not hardcode rules such as menstruation lasting a fixed number of days or a cold always resolving quickly. Let later evidence update, clear, worsen, or extend the condition.",
		"- If LUA, a character sheet, or another chat runtime owns exact health/stat values, set authority_hint=external_runtime and record only the narrative evidence; do not override that runtime's numeric state.",
		"- entity_conditions is for evidence-bound continuity of important non-character entities, especially named items, equipment, locations, or artifacts whose changed state should persist: broken, repaired, sealed, unlocked, activated, depleted, contaminated, lost, inaccessible, transformed, or cleared.",
		"- Each entity_conditions item should include owner_entity_name or owner_entity_key, owner_entity_type when known, condition_label, evidence_excerpt, source_turn_index, and may include effect_kind, onset_story_clock_json, duration_json, expires_at_clock_json, uncertainty_note, and authority_hint.",
		"- Do not emit entity_conditions for ordinary props or unchanged descriptions. Use it only when the changed entity state would create a continuity error if forgotten later.",
		"- world_rules must describe durable world facts, not prompt instructions or style rules. You are responsible for judging them; backend code will not infer rules from keyword lists.",
		"- Emit world_rules and world_state.rules when the latest turn establishes a durable constraint that should affect future turns: natural/physical laws, magic/technology mechanics, apocalypse survival norms, unspoken social law, institutional policy, school/academy custom, workplace procedure, family/household rule, contract, rank/authority, faction/group norm, location access, schedule/calendar, economy/resource constraint, logistics doctrine, or other world-law equivalent.",
		"- The category list is non-exhaustive. If the story establishes a stable law of the setting, social order, organization, environment, or genre logic, capture it even when it does not literally use words like rule, law, policy, or protocol.",
		"- Use the canonical world-rule scope vocabulary exactly: root, region, location, faction, system, session.",
		"- Scope guidance: root=universal cosmology or setting-wide law; region=named country/city/territory/large area; location=concrete place/base/building/dungeon/site; faction=organization/church/guild/government/gang/party/team; system=magic/technology/progression/economy/combat/reward mechanics; session=temporary session-only plan or rule without a more specific stable scope.",
		"- Do not put named regions, named locations, named factions, or progression mechanics under root just because they are important. Use their specific scope and scope_name.",
		"- In system/progression stories, judge durable mechanics as world_rules when confirmed: randomized or conditional acquisition, base/home/environment constraints, challenge entry/clear/reward loops, exchange/cost economy, upgrade or unlock rules, item acquisition/crafting rules, stat growth, group/party limits, cooldowns, ranks, quests, or other recurring progression mechanics.",
		"- Mandatory world-rule audit: before returning JSON, check whether the latest turn established or confirmed any stable setting constraint, repeated system mechanic, progression mechanic, acquisition method, challenge/reward loop, exchange/cost rule, growth/unlock rule, access condition, social order, faction norm, institution rule, resource/logistics rule, environment constraint, magic/technology law, rank/authority rule, schedule/calendar rule, contract, taboo, or unspoken norm.",
		"- Always fill world_rule_audit. If that audit is positive, set world_rule_audit.durable_rule_found=true and world_rules must not be empty. Emit at least one compact evidence-bound rule with scope, category, key, and value; mirror it in world_state.rules when it shapes current setting state.",
		"- If you detect a durable rule but cannot fit the final rule list, still set world_rule_audit.durable_rule_found=true and explain the missing rule in world_rule_audit.reason. A focused follow-up audit may repair the omission.",
		"- Early-session setup counts. Do not wait for many turns: a 1-7 turn session can already establish foundational world rules such as randomized acquisition, progression currency exchange, challenge reward loops, environment/base constraints, access gates, or upgrade/item progression.",
		"- Extract the abstract invariant behind the session's surface nouns. Do not copy these instruction examples as setting facts; use the session's own evidence and names.",
		"- Do not leave world_rules empty for confirmed public facts, institutional rules, class/company policies, social obligations, access permissions, hierarchy/authority rules, special-world mechanics, supernatural/technology rules, recurring resource constraints, or implicit norms that remain true beyond this single exchange.",
		"- A proposal, temporary strategy, one-off plan, implementation method, named operation, or unresolved objective is not a world_rule merely because characters accept or intend it. Keep it in pending_threads or goal_status.",
		"- Emit a world_rule only when the latest completed turn establishes an enacted, continuing institutional or setting constraint beyond the current objective. A procedure or tactical doctrine qualifies only when evidence shows that it is a recurring durable rule rather than a one-off objective.",
		"- Each world rule must include key and value; prefer scope, scope_name, category, confidence, and verification/evidence when available. Use world_state.rules for the same durable rules when they shape the current world state.",
		"- subjective_entity_memories is for each named in-story entity's subjective recollection or interpretation of the latest turn. It is not canonical truth.",
		"- Each subjective_entity_memories item must include owner_entity_key or owner_entity_name, memory_text, and may include owner_entity_role, owner_visibility, source_turn_index, importance_10, emotional_weight, evidence_excerpt, secret_guard, target_reveal_policy, tags, and portability.",
		"- When a named character clearly feels, fears, trusts, suspects, misunderstands, decides, resents, or privately interprets the event, include a subjective_entity_memories item for that owner. Keep it evidence-bound and support-only.",
		"- Use owner_entity_role=protagonist for the player/persona and owner_entity_role=npc with owner_visibility=owner_private for private NPC recollections. Keep NPC-only memories out of persona_capsule_candidates.",
		"- subjective_entity_memories must remain support-only: never use it to overwrite current-world truth, canonical memory, direct evidence, KG triples, character state, or world rules.",
		"- NPC/private subjective_entity_memories are interpretations, suspicions, misunderstandings, or private bias unless current direct evidence states otherwise; never promote them to objective fact or narrator-revealed truth.",
		"- Conflict or misunderstanding memories should stay owner-private and may only influence that owner entity's behavior, subtext, hesitation, avoidance, or selective silence until explicit current-session reveal.",
		"- protected_secrets is for any information that should not become public narration or impossible character knowledge: private affection, guilt, shame, mistakes, lies, fears, debts, hidden plans, hidden identity, hidden role, hidden allegiance, lineage, succession, protected power inheritance, or similar private knowledge.",
		"- Each protected_secrets item may include secret_kind, owner, subject, summary, sensitivity, evidence_strength, disclosure_policy, knowledge_scope, and evidence_excerpt. Keep the text evidence-bound and do not invent secrets.",
		"- If a protected secret exists, set secret_guard=true on the matching subjective_entity_memories item and use target_reveal_policy such as owner_private_until_revealed, explicit_reveal_event_required, or user_directed_reveal_only.",
		"- Stored secret truth is not permission for spontaneous confession, public narration, or unrelated-character discovery. Preserve it as owner-scoped support until current evidence reveals it.",
		"- character_identity_accuracy is for evidence-bound identity/role/allegiance mappings such as cover identity, disguise, hidden role, hidden allegiance, secret successor, hidden lineage, or protected power inheritance. Include same_entity, surface_identity_name, true_identity_name, identity_kind, reveal_policy, and knowledge_scope when supported.",
		"- Do not use character-specific hardcoded aliases. Identity/protected-secret candidates must come from the latest turn or safe context evidence only.",
		"- persona_capsule_candidates is optional and proposal-only. Use it only for protagonist/player subjective recollections that may be carried to another session, loop, regression, reincarnation, isekai, or same-character continuation.",
		"- persona_capsule_candidates must never be used to write current-world truth, canonical memory, direct evidence, KG triples, character state, or world rules. It is support_only_persona_recollection and requires later user/operator approval.",
		"- Each persona_capsule_candidates item may include memory_text, source_turn_index, importance_10, emotional_weight, portability, mode, secret_guard, tags, evidence_excerpt, and injection_policy.",
		"- Mark secret_guard true when the recollection reveals regression, loop, reincarnation, possession/rebirth, isekai transfer, or identity-carryover that should remain protagonist-private until explicitly revealed by current user input.",
		"- Critic_Archive_Ledger_JSON is a bounded read-only support ledger. Use it to avoid duplicate memories, stale residue, and contradiction drift.",
		"- Never copy Critic_Archive_Ledger_JSON item summaries as new evidence unless the latest user/assistant turn also supports the fact.",
		"- If Critic_Archive_Ledger_JSON is null, empty, or degraded, continue extracting only from the latest turn and safe context.",
		"",
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Deterministic_Preview_Pass_JSON>",
		string(preview),
		"</Deterministic_Preview_Pass_JSON>",
		"",
		"<Critic_Archive_Ledger_JSON>",
		string(ledger),
		"</Critic_Archive_Ledger_JSON>",
		"",
		"<Output_Language_Override_JSON>",
		string(lang),
		"</Output_Language_Override_JSON>",
		"",
		"<Language_Context_JSON>",
		string(langCtx),
		"</Language_Context_JSON>",
	}, "\n")
}

func (s *Server) buildCompleteTurnCriticPreviewPass(ctx context.Context, sid string, turnIndex int, contextMessages []map[string]any, userInput, assistantContent string) map[string]any {
	rawPreview := []map[string]any{}
	start := len(contextMessages) - 3
	if start < 0 {
		start = 0
	}
	for _, item := range contextMessages[start:] {
		content := strings.TrimSpace(stringFromMap(item, "content"))
		if content == "" {
			continue
		}
		rawPreview = append(rawPreview, map[string]any{
			"role":    extractionFirstNonEmpty(stringFromMap(item, "role"), "unknown"),
			"text":    truncateRunes(content, 240),
			"source":  extractionFirstNonEmpty(stringFromMap(item, "source"), "context"),
			"bounded": true,
		})
	}
	directSeed := []map[string]any{}
	if s.Store != nil {
		if rows, err := s.Store.ListEvidence(ctx, sid); err == nil {
			for i := len(rows) - 1; i >= 0 && len(directSeed) < 3; i-- {
				row := rows[i]
				if row.Tombstoned || strings.TrimSpace(row.EvidenceText) == "" {
					continue
				}
				if row.SourceTurnEnd > 0 && row.SourceTurnEnd > turnIndex {
					continue
				}
				evidenceText := sanitizeTextForCriticInput(row.EvidenceText)
				if strings.TrimSpace(evidenceText) == "" {
					continue
				}
				directSeed = append(directSeed, map[string]any{
					"text":        truncateRunes(evidenceText, 240),
					"turn_anchor": row.TurnAnchor,
					"source_turn": map[string]any{"start": row.SourceTurnStart, "end": row.SourceTurnEnd},
					"kind":        row.EvidenceKind,
				})
			}
		}
	}
	latestChars := len([]rune(strings.TrimSpace(userInput + "\n" + assistantContent)))
	priority := "low"
	if len(directSeed) > 0 || len(rawPreview) >= 2 || latestChars >= 1200 {
		priority = "medium"
	}
	shouldCompact := latestChars >= 4000 || len(rawPreview) >= 3
	return map[string]any{
		"policy_version":                       completeTurnCriticPreviewPassVersion,
		"status":                               "ok",
		"recent_raw_preview":                   rawPreview,
		"recent_verified_direct_evidence_seed": directSeed,
		"triage": map[string]any{
			"priority":        priority,
			"latest_chars":    latestChars,
			"raw_item_count":  len(rawPreview),
			"direct_seed_hit": len(directSeed) > 0,
		},
		"compaction_hint": map[string]any{
			"should_trigger": shouldCompact,
			"mode":           "hint_only",
		},
	}
}

func parseJSONFromLLMContent(content string) (map[string]any, error) {
	candidate, err := extractJSONCandidateFromLLMContent(content)
	if err == nil {
		if out, parseErr := unmarshalJSONCandidate(candidate); parseErr == nil {
			return out, nil
		}
		if out, repairErr := unmarshalJSONCandidate(repairJSONCandidate(candidate)); repairErr == nil {
			return out, nil
		}
	}

	structuralRepair := repairStructuralJSONQuotes(normalizeLLMJSONText(content))
	repairedCandidate, repairExtractErr := extractJSONCandidateFromLLMContent(structuralRepair)
	if repairExtractErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, repairExtractErr
	}
	repairedCandidate = repairJSONCandidate(repairedCandidate)
	out, repairErr := unmarshalJSONCandidate(repairedCandidate)
	if repairErr != nil {
		return nil, repairErr
	}
	return out, nil
}

func unmarshalJSONCandidate(candidate string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(candidate), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractJSONCandidateFromLLMContent(content string) (string, error) {
	cleaned := normalizeLLMJSONText(content)
	start := strings.Index(cleaned, "{")
	if start < 0 {
		return "", errors.New("critic_json_missing")
	}
	stack := []byte{}
	inString := false
	escaped := false
	for i := start; i < len(cleaned); i++ {
		ch := cleaned[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return "", errors.New("critic_json_mismatched_braces")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(cleaned[start : i+1]), nil
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return "", errors.New("critic_json_mismatched_brackets")
			}
			stack = stack[:len(stack)-1]
		}
	}
	return closeTruncatedJSONCandidate(cleaned[start:], stack, inString, escaped)
}

func normalizeLLMJSONText(content string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func repairJSONCandidate(candidate string) string {
	repaired := replaceJSONLiteralsOutsideStrings(candidate)
	repaired = repairMissingJSONValuesOutsideStrings(repaired)
	repaired = removeJSONTrailingCommasOutsideStrings(repaired)
	return strings.TrimSpace(repaired)
}

func repairStructuralJSONQuotes(input string) string {
	runes := []rune(input)
	var b strings.Builder
	b.Grow(len(input))
	inASCIIString := false
	inCurlyString := false
	escaped := false
	var previousSignificant rune
	for i, current := range runes {
		if inASCIIString {
			b.WriteRune(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inASCIIString = false
			}
			continue
		}
		if inCurlyString {
			if isCurlyJSONQuote(current) && curlyJSONQuoteClosesToken(runes, i) {
				b.WriteByte('"')
				inCurlyString = false
				previousSignificant = '"'
			} else {
				b.WriteRune(current)
			}
			continue
		}
		if current == '"' {
			b.WriteRune(current)
			inASCIIString = true
			escaped = false
			continue
		}
		if isCurlyJSONQuote(current) && curlyJSONQuoteCanOpenToken(previousSignificant) {
			b.WriteByte('"')
			inCurlyString = true
			escaped = false
			continue
		}
		b.WriteRune(current)
		if !isJSONWhitespaceRune(current) {
			previousSignificant = current
		}
	}
	return b.String()
}

func isCurlyJSONQuote(value rune) bool {
	switch value {
	case '\u2018', '\u2019', '\u201c', '\u201d', '\u201e', '\u201f':
		return true
	default:
		return false
	}
}

func curlyJSONQuoteCanOpenToken(previous rune) bool {
	switch previous {
	case 0, '{', '[', ',', ':':
		return true
	default:
		return false
	}
}

func curlyJSONQuoteClosesToken(input []rune, index int) bool {
	for i := index + 1; i < len(input); i++ {
		if isJSONWhitespaceRune(input[i]) {
			continue
		}
		switch input[i] {
		case ':', ',', '}', ']':
			return true
		default:
			return false
		}
	}
	return true
}

func isJSONWhitespaceRune(value rune) bool {
	switch value {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func removeJSONTrailingCommasOutsideStrings(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		current := input[i]
		if inString {
			b.WriteByte(current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			b.WriteByte(current)
			continue
		}
		if current == ',' {
			next := i + 1
			for next < len(input) && (input[next] == ' ' || input[next] == '\t' || input[next] == '\r' || input[next] == '\n') {
				next++
			}
			if next < len(input) && (input[next] == '}' || input[next] == ']') {
				continue
			}
		}
		b.WriteByte(current)
	}
	return b.String()
}

func closeTruncatedJSONCandidate(candidate string, stack []byte, inString bool, escaped bool) (string, error) {
	if len(stack) == 0 && !inString {
		return "", errors.New("critic_json_unclosed")
	}
	repaired := strings.TrimSpace(candidate)
	if inString {
		if escaped {
			repaired += "\\"
		}
		repaired += `"`
	}
	repaired = strings.TrimRight(repaired, " \t\r\n,")
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case '{':
			repaired += "}"
		case '[':
			repaired += "]"
		default:
			return "", errors.New("critic_json_unclosed")
		}
	}
	return repaired, nil
}

func replaceJSONLiteralsOutsideStrings(input string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); {
		ch := input[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			i++
			continue
		}
		if hasJSONLiteralAt(input, i, "None") {
			b.WriteString("null")
			i += len("None")
			continue
		}
		if hasJSONLiteralAt(input, i, "True") {
			b.WriteString("true")
			i += len("True")
			continue
		}
		if hasJSONLiteralAt(input, i, "False") {
			b.WriteString("false")
			i += len("False")
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func repairMissingJSONValuesOutsideStrings(input string) string {
	var b strings.Builder
	inString := false
	escaped := false
	expectValue := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			expectValue = false
			b.WriteByte(ch)
			continue
		}
		if ch == ':' {
			expectValue = true
			b.WriteByte(ch)
			continue
		}
		if expectValue {
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
				b.WriteByte(ch)
				continue
			}
			if ch == '}' || ch == ']' || ch == ',' {
				b.WriteString("null")
				expectValue = false
			} else {
				expectValue = false
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func hasJSONLiteralAt(input string, pos int, literal string) bool {
	if pos+len(literal) > len(input) || input[pos:pos+len(literal)] != literal {
		return false
	}
	beforeOK := pos == 0 || !isJSONLiteralChar(input[pos-1])
	after := pos + len(literal)
	afterOK := after >= len(input) || !isJSONLiteralChar(input[after])
	return beforeOK && afterOK
}

func isJSONLiteralChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func validateCriticExtractionSchema(raw map[string]any) error {
	if raw == nil || len(raw) == 0 {
		return errors.New("critic schema requires a non-empty JSON object")
	}
	recognizedPayload := false
	stringFields := []string{"turn_summary"}
	numberFields := []string{"importance_score", "emotional_intensity", "narrative_significance"}
	arrayFields := []string{
		"evidence_excerpts", "kg_triples", "character_deltas", "pending_threads",
		"speaker_attributions", "world_rules", "physical_conditions",
		"entity_conditions", "narrative_events", "state_claims", "belief_updates",
		"subjective_entity_memories", "protected_secrets",
		"character_identity_accuracy", "persona_capsule_candidates",
	}
	objectFields := []string{
		"entities", "relationship_memory", "state_deltas", "world_rule_audit",
		"world_state", "archive_hint", "story_clock",
	}
	for _, field := range stringFields {
		value, exists := raw[field]
		if !exists {
			continue
		}
		recognizedPayload = true
		if _, ok := value.(string); !ok {
			return fmt.Errorf("critic schema field %s must be a string", field)
		}
	}
	for _, field := range numberFields {
		value, exists := raw[field]
		if !exists {
			continue
		}
		switch value.(type) {
		case float64, float32, int, int32, int64, json.Number:
		default:
			return fmt.Errorf("critic schema field %s must be a number", field)
		}
	}
	for _, field := range arrayFields {
		value, exists := raw[field]
		if !exists {
			continue
		}
		recognizedPayload = true
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("critic schema field %s must be an array", field)
		}
	}
	for _, field := range objectFields {
		value, exists := raw[field]
		if !exists {
			continue
		}
		recognizedPayload = true
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("critic schema field %s must be an object", field)
		}
	}
	if excerpts, ok := raw["evidence_excerpts"].([]any); ok {
		for index, excerpt := range excerpts {
			if _, ok := excerpt.(string); !ok {
				return fmt.Errorf("critic schema field evidence_excerpts[%d] must be a string", index)
			}
		}
	}
	if value, exists := raw["story_clock"]; exists {
		if err := validateStoryClockProposal(value); err != nil {
			return err
		}
	}
	if !recognizedPayload {
		return errors.New("critic schema has no recognized extraction payload")
	}
	return nil
}

func quarantineCriticProtectedCandidates(raw map[string]any, userInput, assistantContent string) (map[string]any, map[string]any) {
	if raw == nil {
		return raw, nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = value
	}
	source := strings.TrimSpace(userInput + "\n" + assistantContent)
	reasons := map[string]int{}
	total := 0
	kept := 0
	quarantine := func(reason string) {
		reasons[reason]++
	}

	if values, ok := raw["protected_secrets"].([]any); ok {
		safe := make([]any, 0, len(values))
		for _, value := range values {
			total++
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				quarantine("protected_secret_not_object")
				continue
			}
			owner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "owner"),
				stringFromMap(item, "owner_entity_name"),
				stringFromMap(item, "character_name"),
			))
			summary := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "summary"),
				stringFromMap(item, "memory_text"),
				stringFromMap(item, "secret_summary"),
				stringFromMap(item, "text"),
			))
			policy := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "disclosure_policy"),
				stringFromMap(item, "target_reveal_policy"),
				stringFromMap(item, "reveal_policy"),
			))
			evidence := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "evidence_excerpt"),
				stringFromMap(item, "evidence"),
			))
			switch {
			case owner == "" || summary == "":
				quarantine("protected_secret_identity_or_summary_missing")
			case policy == "":
				quarantine("protected_secret_policy_missing")
			case !criticEvidenceOccursInSource(evidence, source):
				quarantine("protected_secret_evidence_unbound")
			case !criticOwnerOccursInSource(owner, source) ||
				!criticProtectedClaimSupported(summary, evidence, owner):
				quarantine("protected_secret_claim_unbound")
			default:
				safe = append(safe, item)
				kept++
			}
		}
		out["protected_secrets"] = safe
	}

	if values, ok := raw["subjective_entity_memories"].([]any); ok {
		safe := make([]any, 0, len(values))
		for _, value := range values {
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				total++
				quarantine("subjective_memory_not_object")
				continue
			}
			role := strings.ToLower(strings.TrimSpace(stringFromMap(item, "owner_entity_role")))
			visibility := strings.ToLower(strings.TrimSpace(stringFromMap(item, "owner_visibility")))
			portability := strings.ToLower(strings.TrimSpace(stringFromMap(item, "portability")))
			defaultsToProtected := (role == "" && visibility == "") ||
				(role == "npc" && visibility == "")
			protected := boolFromAny(item["secret_guard"]) ||
				visibility == "owner_private" ||
				portability == "npc_private_recollection" ||
				defaultsToProtected
			if !protected {
				safe = append(safe, item)
				continue
			}
			total++
			owner := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "owner_entity_name"),
				stringFromMap(item, "owner_entity_key"),
				stringFromMap(item, "entity_name"),
				stringFromMap(item, "name"),
			))
			text := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "memory_text"),
				stringFromMap(item, "subjective_memory"),
				stringFromMap(item, "recollection"),
				stringFromMap(item, "interpretation"),
				stringFromMap(item, "summary"),
			))
			policy := strings.TrimSpace(stringFromMap(item, "target_reveal_policy"))
			evidence := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "evidence_excerpt"),
				stringFromMap(item, "evidence"),
			))
			switch {
			case owner == "" || text == "":
				quarantine("protected_subjective_identity_or_text_missing")
			case policy == "":
				quarantine("protected_subjective_policy_missing")
			case !criticEvidenceOccursInSource(evidence, source):
				quarantine("protected_subjective_evidence_unbound")
			case !criticOwnerOccursInSource(owner, source) ||
				!criticProtectedClaimSupported(text, evidence, owner):
				quarantine("protected_subjective_claim_unbound")
			default:
				safe = append(safe, item)
				kept++
			}
		}
		out["subjective_entity_memories"] = safe
	}

	if values, ok := raw["character_identity_accuracy"].([]any); ok {
		safe := make([]any, 0, len(values))
		for _, value := range values {
			total++
			item, itemOK := value.(map[string]any)
			if !itemOK || item == nil {
				quarantine("protected_identity_not_object")
				continue
			}
			surface := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "surface_identity_name"),
				stringFromMap(item, "public_identity_name"),
				stringFromMap(item, "alias_name"),
			))
			trueName := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "true_identity_name"),
				stringFromMap(item, "canonical_entity_name"),
				stringFromMap(item, "real_identity_name"),
			))
			policy := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "reveal_policy"),
				stringFromMap(item, "target_reveal_policy"),
				stringFromMap(item, "disclosure_policy"),
			))
			evidence := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "evidence_excerpt"),
				stringFromMap(item, "evidence"),
			))
			switch {
			case surface == "" || trueName == "" || !boolFromAny(item["same_entity"]):
				quarantine("protected_identity_mapping_incomplete")
			case policy == "":
				quarantine("protected_identity_policy_missing")
			case !criticEvidenceOccursInSource(evidence, source) ||
				!criticProtectedIdentitySupported(surface, trueName, evidence, source):
				quarantine("protected_identity_evidence_unbound")
			default:
				safe = append(safe, item)
				kept++
			}
		}
		out["character_identity_accuracy"] = safe
	}

	if total == 0 {
		return out, nil
	}
	reasonPayload := map[string]any{}
	for reason, count := range reasons {
		reasonPayload[reason] = count
	}
	return out, map[string]any{
		"contract_version":  "critic_protected_candidate_quarantine.v1",
		"policy":            "exact_current_source_evidence_required",
		"candidate_count":   total,
		"kept_count":        kept,
		"quarantined_count": total - kept,
		"reasons":           reasonPayload,
	}
}

func criticEvidenceOccursInSource(evidence, source string) bool {
	evidence = strings.TrimSpace(evidence)
	source = strings.TrimSpace(source)
	return evidence != "" && source != "" && strings.Contains(source, evidence)
}

func criticOwnerOccursInSource(owner, source string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	source = strings.ToLower(strings.TrimSpace(source))
	return owner != "" && source != "" && strings.Contains(source, owner)
}

func criticProtectedIdentitySupported(surface, trueName, evidence, source string) bool {
	surface = strings.ToLower(strings.TrimSpace(surface))
	trueName = strings.ToLower(strings.TrimSpace(trueName))
	evidence = strings.ToLower(strings.TrimSpace(evidence))
	source = strings.ToLower(strings.TrimSpace(source))
	if surface == "" || trueName == "" || evidence == "" || source == "" {
		return false
	}
	return criticTextContainsDistinctIdentityPair(source, surface, trueName) &&
		criticTextContainsDistinctIdentityPair(evidence, surface, trueName)
}

func criticTextContainsDistinctIdentityPair(text, surface, trueName string) bool {
	if !strings.Contains(text, surface) || !strings.Contains(text, trueName) {
		return false
	}
	if surface == trueName {
		return true
	}
	if strings.Contains(surface, trueName) {
		return strings.Contains(strings.ReplaceAll(text, surface, " "), trueName)
	}
	if strings.Contains(trueName, surface) {
		return strings.Contains(strings.ReplaceAll(text, trueName, " "), surface)
	}
	return true
}

func criticProtectedClaimSupported(claim, evidence, owner string) bool {
	claim = strings.ToLower(strings.TrimSpace(claim))
	evidence = strings.ToLower(strings.TrimSpace(evidence))
	if claim == "" || evidence == "" {
		return false
	}
	if strings.Contains(claim, evidence) || strings.Contains(evidence, claim) {
		return true
	}
	ownerTokens := criticSubstantiveTokens(owner)
	claimTokens := criticSubstantiveTokens(claim)
	evidenceTokens := criticSubstantiveTokens(evidence)
	overlap := 0
	for token := range claimTokens {
		if _, ownerToken := ownerTokens[token]; ownerToken {
			continue
		}
		if _, supported := evidenceTokens[token]; supported {
			overlap++
			if overlap >= 2 {
				return true
			}
		}
	}
	return false
}

func criticSubstantiveTokens(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	var current []rune
	flush := func() {
		if len(current) >= 2 {
			token := string(current)
			if _, ignored := criticClaimStopTokens[token]; !ignored {
				tokens[token] = struct{}{}
			}
		}
		current = current[:0]
	}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current = append(current, r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func normalizeCriticExtraction(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	out["turn_summary"] = normalizeCriticTurnSummary(raw["turn_summary"])
	out["importance_score"] = clampFloat(extractionFloatFromAny(raw["importance_score"], 3), 1, 10)
	out["emotional_intensity"] = clampFloat(extractionFloatFromAny(raw["emotional_intensity"], 0), 0, 1)
	out["narrative_significance"] = clampFloat(extractionFloatFromAny(raw["narrative_significance"], 0), 0, 1)
	out["evidence_excerpts"] = stringsFromAny(raw["evidence_excerpts"])
	if storyClock := normalizeStoryClockProposal(raw["story_clock"]); len(storyClock) > 0 {
		out["story_clock"] = storyClock
	} else {
		delete(out, "story_clock")
	}
	out["kg_triples"] = sliceFromAny(raw["kg_triples"])
	out["character_deltas"] = sliceFromAny(raw["character_deltas"])
	out["pending_threads"] = sliceFromAny(raw["pending_threads"])
	out["entities"] = mapFromAny(raw["entities"])
	out["speaker_attributions"] = normalizeSpeakerAttributionCandidates(raw["speaker_attributions"])
	out["relationship_memory"] = mapFromAny(raw["relationship_memory"])
	out["state_deltas"] = mapFromAny(raw["state_deltas"])
	out["world_rules"] = sliceFromAny(raw["world_rules"])
	out["physical_conditions"] = sliceFromAny(raw["physical_conditions"])
	out["entity_conditions"] = sliceFromAny(raw["entity_conditions"])
	out["narrative_events"] = sliceFromAny(raw["narrative_events"])
	out["state_claims"] = sliceFromAny(raw["state_claims"])
	out["belief_updates"] = sliceFromAny(raw["belief_updates"])
	protectedSecrets := normalizeProtectedSecrets(raw["protected_secrets"])
	characterIdentityAccuracy := normalizeCharacterIdentityAccuracy(raw["character_identity_accuracy"])
	subjectiveMemories := normalizeSubjectiveEntityMemories(raw["subjective_entity_memories"])
	subjectiveMemories = appendProtectedSecretSubjectiveMemories(subjectiveMemories, protectedSecrets)
	subjectiveMemories = appendIdentityAccuracySubjectiveMemories(subjectiveMemories, characterIdentityAccuracy)
	out["protected_secrets"] = protectedSecrets
	out["character_identity_accuracy"] = characterIdentityAccuracy
	out["subjective_entity_memories"] = subjectiveMemories
	out["persona_capsule_candidates"] = normalizePersonaCapsuleCandidates(raw["persona_capsule_candidates"])
	return out
}

func enrichNormalizedCriticExtractionForFocusedRecall(extraction map[string]any, userInput, assistantContent string, turnIndex int) map[string]any {
	if extraction == nil {
		extraction = map[string]any{}
	}
	extraction["turn_summary"] = normalizeCriticTurnSummary(extraction["turn_summary"])
	if strings.TrimSpace(extractionStringFromAny(extraction["turn_summary"])) == "" {
		if summary := focusedRecallFallbackSummary(userInput, assistantContent); summary != "" {
			extraction["turn_summary"] = summary
		}
	}
	if len(stringsFromAny(extraction["evidence_excerpts"])) == 0 {
		if excerpts := focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent); len(excerpts) > 0 {
			extraction["evidence_excerpts"] = excerpts
			extraction["focused_recall_fallback"] = map[string]any{
				"policy_version": "focused_recall_fallback.v1",
				"source":         "latest_turn_exact_excerpts",
				"turn_index":     turnIndex,
				"reason":         "critic_returned_no_evidence_excerpts",
			}
		}
	}
	return extraction
}

func normalizeCriticTurnSummary(value any) string {
	if value == nil || isStructuredCriticTurnSummaryValue(value) {
		return ""
	}
	text := strings.TrimSpace(extractionStringFromAny(value))
	if looksLikeStructuredCriticPayloadText(text) {
		return ""
	}
	return text
}

func isStructuredCriticTurnSummaryValue(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func looksLikeStructuredCriticPayloadText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "map[") && !strings.HasPrefix(trimmed, "[") {
		return false
	}
	lower := strings.ToLower(trimmed)
	hits := 0
	for _, marker := range []string{
		"archive_hint",
		"character_deltas",
		"entity_conditions",
		"evidence_excerpts",
		"kg_triples",
		"pending_threads",
		"physical_conditions",
		"relationship_memory",
		"narrative_events",
		"state_claims",
		"belief_updates",
		"state_deltas",
		"subjective_entity_memories",
		"turn_summary",
		"world_rules",
	} {
		if strings.Contains(lower, marker) {
			hits++
		}
	}
	return hits >= 2
}

func focusedRecallFallbackSummary(userInput, assistantContent string) string {
	user := focusedRecallFirstExcerpt(userInput, 220)
	assistant := focusedRecallFirstExcerpt(assistantContent, 360)
	parts := []string{}
	if user != "" {
		parts = append(parts, "user: "+user)
	}
	if assistant != "" {
		parts = append(parts, "assistant: "+assistant)
	}
	return truncateRunes(strings.Join(parts, " / "), 700)
}

func focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent string) []string {
	out := []string{}
	add := func(text string) {
		for _, excerpt := range focusedRecallExcerptCandidates(text) {
			if excerpt == "" || containsStringFold(out, excerpt) {
				continue
			}
			out = append(out, excerpt)
			if len(out) >= 3 {
				return
			}
		}
	}
	add(userInput)
	if len(out) < 3 {
		add(assistantContent)
	}
	return out
}

func focusedRecallFirstExcerpt(text string, limit int) string {
	for _, item := range focusedRecallExcerptCandidates(text) {
		return truncateRunes(item, limit)
	}
	return ""
}

func focusedRecallExcerptCandidates(text string) []string {
	text = strings.TrimSpace(sanitizeCriticStorageText(text))
	if text == "" {
		return nil
	}
	candidates := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, piece := range splitFocusedRecallLine(line) {
			piece = strings.TrimSpace(piece)
			if !looksLikeFocusedRecallExcerpt(piece) {
				continue
			}
			candidates = append(candidates, truncateRunes(piece, 240))
			if len(candidates) >= 4 {
				return candidates
			}
		}
	}
	if len(candidates) == 0 && looksLikeFocusedRecallExcerpt(text) {
		candidates = append(candidates, truncateRunes(text, 240))
	}
	return candidates
}

func splitFocusedRecallLine(line string) []string {
	out := []string{}
	start := 0
	runes := []rune(line)
	for i, r := range runes {
		switch r {
		case '.', '!', '?', '。', '！', '？', '…':
			if i+1-start >= 12 {
				out = append(out, string(runes[start:i+1]))
				start = i + 1
			}
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

func looksLikeFocusedRecallExcerpt(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	runeLen := len([]rune(text))
	if runeLen < 8 {
		return false
	}
	lower := strings.ToLower(text)
	blocked := []string{"```", "archive center", "auxiliary context", "direct evidence", "latest direct evidence", "recent raw turn"}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return false
		}
	}
	return true
}

func containsStringFold(items []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, item := range items {
		if strings.TrimSpace(strings.ToLower(item)) == target {
			return true
		}
	}
	return false
}

func appendUniqueTurnRoleText(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	for _, part := range strings.Split(existing, "\n") {
		if strings.EqualFold(strings.TrimSpace(part), next) {
			return existing
		}
	}
	return existing + "\n" + next
}
