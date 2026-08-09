package httpapi

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

const (
	copilotCodeVersion = "1.85.0"
	copilotChatVersion = "0.22.0"
	copilotTokenURL    = "https://api.github.com/copilot_internal/v2/token"
)

type proxyRequestPolicy struct {
	JSONResponse bool
	Purpose      string
}

type proxyEmptyContentError struct {
	Provider string
}

func (e *proxyEmptyContentError) Error() string {
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "provider"
	}
	return provider + " returned no text content"
}

type proxyLocalRequestError struct {
	Stage string
	Cause error
}

func (e *proxyLocalRequestError) Error() string {
	if e.Cause == nil {
		return strings.TrimSpace(e.Stage) + " failed"
	}
	return e.Cause.Error()
}

func (e *proxyLocalRequestError) Unwrap() error {
	return e.Cause
}

func callProxyProvider(ctx context.Context, req dto.ProxyPluginMainRequest) (map[string]any, int, error) {
	return callProxyProviderWithPolicy(ctx, req, proxyRequestPolicy{}, nil)
}

func callProxyProviderWithPolicy(ctx context.Context, req dto.ProxyPluginMainRequest, policy proxyRequestPolicy, retryBudget *llmRetryBudget) (map[string]any, int, error) {
	endpoint := strings.TrimSpace(stringPtrValue(req.Endpoint, ""))
	apiKey := strings.TrimSpace(stringPtrValue(req.APIKey, ""))
	model := strings.TrimSpace(stringPtrValue(req.Model, ""))
	provider := strings.ToLower(strings.TrimSpace(stringPtrValue(req.Provider, "")))
	if provider == "" || endpoint == "" || model == "" || (apiKey == "" && provider != "ollama") {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: fmt.Errorf("provider / endpoint / api_key / model is required"),
		}
	}

	if req.TimeoutMs != nil && *req.TimeoutMs < 0 {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: errors.New("timeout_ms must not be negative"),
		}
	}
	if req.TimeoutMs != nil && *req.TimeoutMs > 0 {
		timeout := time.Duration(*req.TimeoutMs) * time.Millisecond
		if timeout <= 0 || int64(timeout/time.Millisecond) != *req.TimeoutMs {
			return nil, http.StatusBadRequest, &proxyLocalRequestError{
				Stage: "configuration",
				Cause: errors.New("timeout_ms is outside the supported duration range"),
			}
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	switch provider {
	case "claude":
		return proxyCallClaude(ctx, req, endpoint, apiKey, model, policy)
	case "gemini":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, false, policy)
	case "vertex":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, true, policy)
	case "openai", "openrouter", "llmgateway", "vercel", "copilot", "ollama", "custom":
		return proxyCallOpenAILike(ctx, req, endpoint, apiKey, model, provider, policy, retryBudget)
	default:
		return nil, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: fmt.Errorf("unsupported provider %q", provider),
		}
	}
}

func proxyCallOpenAILike(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model, provider string, policy proxyRequestPolicy, retryBudget *llmRetryBudget) (map[string]any, int, error) {
	isGLM := proxyIsGLMLike(model, endpoint, provider)
	target := proxyOpenAIChatEndpoint(proxyOpenAIBaseURL(provider, endpoint), provider, isGLM)

	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	if apiKey != "" && provider != "copilot" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	if provider == "openrouter" {
		headers["HTTP-Referer"] = "https://risuai.xyz"
		headers["X-Title"] = "Archive Center"
	} else if provider == "copilot" {
		headers["Editor-Version"] = "vscode/" + copilotCodeVersion
		headers["Editor-version"] = "vscode/" + copilotCodeVersion
		headers["Editor-Plugin-Version"] = "copilot-chat/" + copilotChatVersion
		headers["Editor-plugin-version"] = "copilot-chat/" + copilotChatVersion
		headers["Copilot-Integration-Id"] = "vscode-chat"
		headers["User-Agent"] = "GitHubCopilotChat/" + copilotChatVersion
		headers["X-Github-Api-Version"] = "2025-10-01"
		headers["X-Initiator"] = "user"
	}

	requestedTokens := maxInt64(1, int64Value(req.MaxTokens, 1024))
	configuredMax := maxInt64(0, int64Value(req.MaxCompletionTokens, 0))
	body := map[string]any{
		"model":       model,
		"messages":    req.Messages,
		"temperature": floatPtrValue(req.Temperature, 0.7),
		"max_tokens":  requestedTokens,
		"stream":      false,
	}
	reasoningFamily := proxyReasoningFamily(provider, stringPtrValue(req.ReasoningPreset, "auto"), model, endpoint)
	if reasoningFamily == "glm" {
		body["max_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
		effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
		thinkingType := proxyGLMThinkingTypeFromRequest(stringPtrValue(req.GlmThinkingType, ""), effort)
		body["thinking"] = map[string]any{
			"type": thinkingType,
		}
		if thinkingType == "enabled" {
			if normalizedEffort := proxyGLM52ReasoningEffort(model, effort); normalizedEffort != "" {
				body["reasoning_effort"] = normalizedEffort
			}
		}
	} else if effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, ""))); effort != "" {
		if effort != "none" || provider == "ollama" {
			body["reasoning_effort"] = effort
		}
		if effort != "none" {
			body["max_completion_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
			delete(body, "max_tokens")
		}
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, provider, false)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	if policyErr := proxyApplyOpenAIJSONResponsePolicy(body, overrideTrace, provider, policy); policyErr != nil {
		return map[string]any{"_proxy_request_overrides": overrideTrace}, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "request_build",
			Cause: policyErr,
		}
	}
	if provider == "copilot" {
		token, status, err := proxyGetCopilotToken(ctx, apiKey)
		if err != nil {
			return nil, status, err
		}
		headers["Authorization"] = "Bearer " + token
	}

	status, data, raw, err := proxyDoJSON(ctx, target, headers, body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if status == http.StatusBadRequest &&
		proxyHasAdvancedParams(body) &&
		proxyUnsupportedParameter(raw, data) &&
		!proxyServiceTierError(raw, data) &&
		retryBudget.take() {
		fallback := cloneMap(body)
		delete(fallback, "reasoning_effort")
		delete(fallback, "max_completion_tokens")
		delete(fallback, "thinking")
		fallback["max_tokens"] = requestedTokens
		status, data, raw, err = proxyDoJSON(ctx, target, headers, fallback)
		if err != nil {
			return nil, http.StatusBadGateway, err
		}
	}
	if status < 200 || status >= 300 {
		return nil, status, fmt.Errorf("%s", scrubProxySecret(proxyErrorDetail(status, data, raw), apiKey))
	}
	if data == nil {
		return nil, http.StatusBadGateway, fmt.Errorf("OpenAI-like provider returned invalid JSON")
	}
	if strings.TrimSpace(chatCompletionText(data)) == "" {
		return nil, http.StatusBadGateway, &proxyEmptyContentError{Provider: provider}
	}
	proxyAttachLLMGatewayServiceTierTrace(data, overrideTrace)
	proxyAttachRequestOverrideTrace(data, overrideTrace)
	return data, http.StatusOK, nil
}

func proxyCallClaude(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model string, policy proxyRequestPolicy) (map[string]any, int, error) {
	target := strings.TrimRight(endpoint, "/")
	if !strings.Contains(target, "/v1/") {
		target += "/v1/messages"
	}
	system, user := proxySplitSystemUser(req.Messages)
	requestedTokens := maxInt64(1, int64Value(req.MaxTokens, 1024))
	body := map[string]any{
		"model":       model,
		"messages":    []map[string]any{{"role": "user", "content": user}},
		"max_tokens":  requestedTokens,
		"temperature": floatPtrValue(req.Temperature, 0.7),
		"stream":      false,
	}
	if system != "" {
		body["system"] = system
	}
	budget := maxInt64(0, firstPositiveInt64(int64Value(req.ReasoningBudgetTokens, 0), int64Value(req.BudgetTokens, 0)))
	configuredMax := maxInt64(0, int64Value(req.MaxCompletionTokens, 0))
	if budget >= 1024 {
		body["max_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
		body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": maxInt64(1024, budget)}
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"Accept":            "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, "claude", false)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	if policyErr := proxyApplyClaudeJSONResponsePolicy(body, overrideTrace, policy); policyErr != nil {
		return map[string]any{"_proxy_request_overrides": overrideTrace}, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "request_build",
			Cause: policyErr,
		}
	}
	status, data, raw, err := proxyDoJSON(ctx, target, headers, body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if status < 200 || status >= 300 {
		return nil, status, fmt.Errorf("%s", scrubProxySecret(proxyErrorDetail(status, data, raw), apiKey))
	}
	content := proxyExtractClaudeText(data)
	if content == "" {
		return nil, status, &proxyEmptyContentError{Provider: "claude"}
	}
	resp := proxyNormalizeChatResponse(content, model, "stop")
	proxyAttachClaudeUsage(resp, data, overrideTrace)
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	return resp, http.StatusOK, nil
}

func proxyCallGemini(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model string, vertex bool, policy proxyRequestPolicy) (map[string]any, int, error) {
	system, user := proxySplitSystemUser(req.Messages)
	requestedTokens := maxInt64(1, int64Value(req.MaxTokens, 1024))
	configuredMax := maxInt64(0, int64Value(req.MaxCompletionTokens, 0))
	budget := maxInt64(0, firstPositiveInt64(int64Value(req.ReasoningBudgetTokens, 0), int64Value(req.BudgetTokens, 0)))
	effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
	isThinking := regexp.MustCompile(`(?i)gemini-(3|2\.5)`).MatchString(model)
	maxOutputTokens := requestedTokens
	if isThinking {
		maxOutputTokens = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
	}
	body := map[string]any{
		"contents": []map[string]any{{"role": "user", "parts": []map[string]any{{"text": user}}}},
		"generationConfig": map[string]any{
			"temperature":     floatPtrValue(req.Temperature, 0.7),
			"maxOutputTokens": maxOutputTokens,
		},
	}
	if system != "" {
		body["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": system}}}
	}
	genCfg := body["generationConfig"].(map[string]any)
	if isThinking {
		thinking := map[string]any{"includeThoughts": false}
		if proxyGeminiThinkingMode(model) == "level" && isGeminiThinkingLevel(effort) {
			thinking["thinkingLevel"] = effort
		} else if budget > 0 {
			thinking["thinkingBudget"] = budget
		}
		genCfg["thinkingConfig"] = thinking
	} else if budget > 0 {
		genCfg["thinkingConfig"] = map[string]any{"thinkingBudget": budget}
	}

	target := ""
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if !vertex {
		target = proxyNormalizeGeminiEndpoint(endpoint, model, "generateContent")
		headers["x-goog-api-key"] = apiKey
	}
	geminiProvider := "gemini"
	if vertex {
		geminiProvider = "vertex"
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, geminiProvider, vertex)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	if policyErr := proxyApplyJSONResponsePolicy(body, overrideTrace, policy); policyErr != nil {
		return map[string]any{"_proxy_request_overrides": overrideTrace}, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "request_build",
			Cause: policyErr,
		}
	}
	if vertex {
		target = proxyNormalizeVertexEndpoint(endpoint, model)
		resolvedTarget, resolveErr := proxyResolveVertexProjectID(target, apiKey)
		if resolveErr != nil {
			return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: resolveErr}
		}
		target = resolvedTarget
		token, status, tokenErr := proxyGetVertexAccessToken(ctx, apiKey)
		if tokenErr != nil {
			return nil, status, tokenErr
		}
		headers["Authorization"] = "Bearer " + token
	}

	status, data, raw, err := proxyDoJSON(ctx, target, headers, body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if status < 200 || status >= 300 {
		detail := proxyErrorDetail(status, data, raw)
		if vertex {
			detail = proxyVertexEndpointErrorDetail(status, target, data, raw)
		}
		return nil, status, fmt.Errorf("%s", scrubProxySecret(detail, apiKey))
	}
	content := proxyExtractGeminiText(data)
	if content == "" {
		return nil, status, &proxyEmptyContentError{Provider: geminiProvider}
	}
	resp := proxyNormalizeChatResponse(content, model, "stop")
	proxyAttachGeminiUsage(resp, data, overrideTrace)
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	return resp, http.StatusOK, nil
}

func proxyApplyJSONResponsePolicy(body map[string]any, trace map[string]any, policy proxyRequestPolicy) error {
	if !policy.JSONResponse {
		return nil
	}
	if trace == nil {
		trace = map[string]any{}
	}
	trace["json_response_requested"] = true
	if purpose := strings.TrimSpace(policy.Purpose); purpose != "" {
		trace["json_response_purpose"] = purpose
	}
	rawGenerationConfig, exists := body["generationConfig"]
	if !exists {
		rawGenerationConfig = map[string]any{}
		body["generationConfig"] = rawGenerationConfig
	}
	generationConfig, ok := rawGenerationConfig.(map[string]any)
	if !ok {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "generationConfig must be a JSON object"
		trace["json_response_existing_type"] = fmt.Sprintf("%T", rawGenerationConfig)
		return fmt.Errorf("json_response_generation_config_conflict: generationConfig must be a JSON object")
	}
	if generationConfig == nil {
		generationConfig = map[string]any{}
		body["generationConfig"] = generationConfig
	}
	const requiredMIME = "application/json"
	existing, mimeExists := generationConfig["responseMimeType"]
	if !mimeExists {
		generationConfig["responseMimeType"] = requiredMIME
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_mime_type"] = requiredMIME
		return nil
	}
	existingText, stringValue := existing.(string)
	if stringValue && strings.EqualFold(strings.TrimSpace(existingText), requiredMIME) {
		trace["json_response_applied"] = true
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_mime_type"] = existingText
		return nil
	}
	trace["json_response_applied"] = false
	trace["json_response_source"] = "extra_body_json"
	trace["json_response_conflict"] = true
	trace["json_response_conflict_reason"] = "generationConfig.responseMimeType must be application/json"
	trace["json_response_existing_type"] = fmt.Sprintf("%T", existing)
	if stringValue {
		trace["json_response_existing_value"] = strings.TrimSpace(existingText)
	}
	return fmt.Errorf("json_response_mime_conflict: generationConfig.responseMimeType must be application/json")
}

func proxyApplyOpenAIJSONResponsePolicy(body map[string]any, trace map[string]any, provider string, policy proxyRequestPolicy) error {
	if !policy.JSONResponse {
		return nil
	}
	if trace == nil {
		trace = map[string]any{}
	}
	trace["json_response_requested"] = true
	if purpose := strings.TrimSpace(policy.Purpose); purpose != "" {
		trace["json_response_purpose"] = purpose
	}

	const requiredType = "json_object"
	existing, exists := body["response_format"]
	if !exists {
		if !proxyProviderSupportsAutomaticOpenAIJSONResponse(provider) {
			trace["json_response_applied"] = false
			trace["json_response_source"] = "backend_policy"
			trace["json_response_skip_reason"] = "provider_native_contract_not_verified"
			return nil
		}
		appliedType := requiredType
		if strings.EqualFold(strings.TrimSpace(provider), "vercel") {
			appliedType = "json_schema"
			body["response_format"] = map[string]any{
				"type": appliedType,
				"json_schema": map[string]any{
					"name":   "archive_center_critic",
					"schema": proxyCriticTopLevelJSONSchema(),
				},
			}
		} else {
			body["response_format"] = map[string]any{"type": appliedType}
		}
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_format"] = appliedType
		return nil
	}

	format, ok := existing.(map[string]any)
	if !ok {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "response_format must be a JSON object"
		trace["json_response_existing_type"] = fmt.Sprintf("%T", existing)
		return fmt.Errorf("json_response_format_conflict: response_format must be a JSON object")
	}
	formatType, isString := format["type"].(string)
	formatType = strings.ToLower(strings.TrimSpace(formatType))
	vercelLegacyJSON := strings.EqualFold(strings.TrimSpace(provider), "vercel") && formatType == "json"
	if isString && (formatType == requiredType || formatType == "json_schema" || vercelLegacyJSON) {
		trace["json_response_applied"] = true
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_format"] = formatType
		return nil
	}
	trace["json_response_applied"] = false
	trace["json_response_source"] = "extra_body_json"
	trace["json_response_conflict"] = true
	trace["json_response_conflict_reason"] = "response_format.type must be json_object or json_schema"
	if strings.EqualFold(strings.TrimSpace(provider), "vercel") {
		trace["json_response_conflict_reason"] = "response_format.type must be json_object, json_schema, or json"
	}
	trace["json_response_existing_type"] = fmt.Sprintf("%T", format["type"])
	if isString {
		trace["json_response_existing_value"] = formatType
	}
	if strings.EqualFold(strings.TrimSpace(provider), "vercel") {
		return fmt.Errorf("json_response_format_conflict: response_format.type must be json_object, json_schema, or json")
	}
	return fmt.Errorf("json_response_format_conflict: response_format.type must be json_object or json_schema")
}

func proxyProviderSupportsAutomaticOpenAIJSONResponse(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "openrouter", "llmgateway", "vercel":
		return true
	default:
		return false
	}
}

func proxyApplyClaudeJSONResponsePolicy(body map[string]any, trace map[string]any, policy proxyRequestPolicy) error {
	if !policy.JSONResponse {
		return nil
	}
	if trace == nil {
		trace = map[string]any{}
	}
	trace["json_response_requested"] = true
	if purpose := strings.TrimSpace(policy.Purpose); purpose != "" {
		trace["json_response_purpose"] = purpose
	}
	existing, exists := body["output_config"]
	if !exists {
		body["output_config"] = map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"schema": proxyCriticTopLevelJSONSchema(),
			},
		}
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_format"] = "json_schema"
		return nil
	}
	outputConfig, ok := existing.(map[string]any)
	if !ok {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config must be a JSON object"
		return fmt.Errorf("json_response_format_conflict: output_config must be a JSON object")
	}
	format, ok := outputConfig["format"].(map[string]any)
	if !ok {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config.format must be a JSON object"
		return fmt.Errorf("json_response_format_conflict: output_config.format must be a JSON object")
	}
	formatType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(format["type"])))
	_, schemaOK := format["schema"].(map[string]any)
	if formatType != "json_schema" || !schemaOK {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config.format requires type json_schema and object schema"
		return fmt.Errorf("json_response_format_conflict: output_config.format requires type json_schema and object schema")
	}
	trace["json_response_applied"] = true
	trace["json_response_source"] = "extra_body_json"
	trace["json_response_format"] = "json_schema"
	return nil
}

func proxyCriticTopLevelJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"turn_summary":                   map[string]any{"type": "string"},
			"importance_score":               map[string]any{"type": "number"},
			"story_clock":                    map[string]any{},
			"relationship_memory":            map[string]any{},
			"entities":                       map[string]any{},
			"kg_triples":                     map[string]any{"type": "array", "items": map[string]any{}},
			"archive_hint":                   map[string]any{},
			"world_rule_audit":               map[string]any{},
			"world_rules":                    map[string]any{"type": "array", "items": map[string]any{}},
			"world_state":                    map[string]any{},
			"subjective_entity_memories":     map[string]any{"type": "array", "items": map[string]any{}},
			"protected_secrets":              map[string]any{"type": "array", "items": map[string]any{}},
			"character_identity_accuracy":    map[string]any{"type": "array", "items": map[string]any{}},
			"persona_capsule_candidates":     map[string]any{"type": "array", "items": map[string]any{}},
			"narrative_events":               map[string]any{"type": "array", "items": map[string]any{}},
			"state_claims":                   map[string]any{"type": "array", "items": map[string]any{}},
			"belief_updates":                 map[string]any{"type": "array", "items": map[string]any{}},
			"interaction_events":             map[string]any{"type": "array", "items": map[string]any{}},
			"relationship_observations":      map[string]any{"type": "array", "items": map[string]any{}},
			"interaction_boundaries":         map[string]any{"type": "array", "items": map[string]any{}},
			"habit_observations":             map[string]any{"type": "array", "items": map[string]any{}},
			"character_profile_observations": map[string]any{"type": "array", "items": map[string]any{}},
			"voice_observations":             map[string]any{"type": "array", "items": map[string]any{}},
			"user_interaction_profile":       map[string]any{"type": "array", "items": map[string]any{}},
			"rp_character_profile":           map[string]any{"type": "array", "items": map[string]any{}},
			"prune_targets":                  map[string]any{"type": "array", "items": map[string]any{}},
			"evidence_excerpts":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"emotional_intensity":            map[string]any{"type": "number"},
			"narrative_significance":         map[string]any{"type": "number"},
			"state_deltas":                   map[string]any{},
			"character_deltas":               map[string]any{"type": "array", "items": map[string]any{}},
			"physical_conditions":            map[string]any{"type": "array", "items": map[string]any{}},
			"entity_conditions":              map[string]any{"type": "array", "items": map[string]any{}},
			"reversible_states":              map[string]any{"type": "array", "items": map[string]any{}},
			"pending_threads":                map[string]any{"type": "array", "items": map[string]any{}},
		},
		"additionalProperties": true,
	}
}

func proxyGetCopilotToken(ctx context.Context, apiKey string) (string, int, error) {
	source := regexp.MustCompile(`[^\x20-\x7E]`).ReplaceAllString(strings.TrimSpace(apiKey), "")
	if source == "" {
		return "", http.StatusBadRequest, fmt.Errorf("Copilot token is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, copilotTokenURL, nil)
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+source)
	req.Header.Set("Origin", "vscode-file://vscode-app")
	req.Header.Set("Editor-Version", "vscode/"+copilotCodeVersion)
	req.Header.Set("Editor-Plugin-Version", "copilot-chat/"+copilotChatVersion)
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")
	req.Header.Set("User-Agent", "GitHubCopilotChat/"+copilotChatVersion)
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return source, http.StatusOK, nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return source, http.StatusOK, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return source, http.StatusOK, nil
	}
	token := strings.TrimSpace(extractionStringFromAny(data["token"]))
	if token == "" {
		return source, http.StatusOK, nil
	}
	return token, http.StatusOK, nil
}

func proxyGetVertexAccessToken(ctx context.Context, serviceAccountJSON string) (string, int, error) {
	var cred struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(serviceAccountJSON), &cred); err != nil {
		return "", http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: fmt.Errorf("Vertex AI Key must be a JSON service account credential"),
		}
	}
	if strings.TrimSpace(cred.ClientEmail) == "" || strings.TrimSpace(cred.PrivateKey) == "" {
		return "", http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: fmt.Errorf("Vertex AI credentials missing client_email or private_key"),
		}
	}
	tokenURI := strings.TrimSpace(cred.TokenURI)
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	now := time.Now().Unix()
	header := proxyBase64URL(mustJSON(map[string]any{"alg": "RS256", "typ": "JWT"}))
	claim := proxyBase64URL(mustJSON(map[string]any{
		"iss":   cred.ClientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform",
		"aud":   "https://oauth2.googleapis.com/token",
		"exp":   now + 3600,
		"iat":   now,
	}))
	signingInput := header + "." + claim
	privateKey, err := parseRSAPrivateKey(cred.PrivateKey)
	if err != nil {
		return "", http.StatusBadRequest, &proxyLocalRequestError{Stage: "configuration", Cause: err}
	}
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: err}
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signingInput+"."+proxyBase64URL(sig))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.StatusCode, fmt.Errorf("%s", proxyErrorDetail(resp.StatusCode, data, string(raw)))
	}
	token := strings.TrimSpace(extractionStringFromAny(data["access_token"]))
	if token == "" {
		return "", http.StatusBadGateway, fmt.Errorf("Vertex token response missing access_token")
	}
	return token, http.StatusOK, nil
}

func proxyDoJSON(ctx context.Context, target string, headers map[string]string, body map[string]any) (int, map[string]any, string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return http.StatusBadRequest, nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return http.StatusBadRequest, nil, "", err
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return http.StatusBadGateway, nil, "", err
	}
	defer resp.Body.Close()
	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return http.StatusBadGateway, nil, "", err
	}
	raw := string(rawBytes)
	var data map[string]any
	if err := json.Unmarshal(rawBytes, &data); err != nil {
		data = nil
	}
	return resp.StatusCode, data, raw, nil
}

func proxyApplyRequestOverrides(headers map[string]string, body map[string]any, req dto.ProxyPluginMainRequest, provider string, vertex bool) (map[string]any, error) {
	trace := map[string]any{}
	headerJSON := strings.TrimSpace(stringPtrValue(req.ExtraHeadersJSON, ""))
	if headerJSON != "" {
		extraHeaders, err := proxyParseJSONObject(headerJSON, "extra_headers_json")
		if err != nil {
			return trace, err
		}
		applied, blocked := proxyApplyExtraHeaders(headers, extraHeaders)
		trace["extra_headers_applied"] = len(applied) > 0
		trace["extra_header_keys"] = applied
		if len(blocked) > 0 {
			trace["extra_header_blocked"] = blocked
		}
	}

	bodyJSON := strings.TrimSpace(stringPtrValue(req.ExtraBodyJSON, ""))
	if bodyJSON != "" {
		extraBody, err := proxyParseJSONObject(bodyJSON, "extra_body_json")
		if err != nil {
			return trace, err
		}
		applied, blocked := proxyMergeExtraBody(body, extraBody, "")
		trace["extra_body_applied"] = len(applied) > 0
		trace["extra_body_keys"] = applied
		if len(blocked) > 0 {
			trace["extra_body_blocked"] = blocked
		}
	}

	if err := proxyApplyLLMGatewayServiceTier(body, req, provider, trace); err != nil {
		return trace, err
	}
	if err := proxyApplyClaudePromptCacheMode(body, req, provider, trace); err != nil {
		return trace, err
	}

	mode := proxyNormalizeVertexFlexMode(stringPtrValue(req.VertexFlexMode, ""))
	if mode != "" && mode != "off" {
		trace["vertex_flex_mode"] = mode
		if !vertex {
			trace["vertex_flex_applied"] = false
			trace["vertex_flex_skip_reason"] = "provider_not_vertex"
		} else {
			headers["X-Vertex-AI-LLM-Shared-Request-Type"] = "flex"
			if mode == "flex_only" {
				headers["X-Vertex-AI-LLM-Request-Type"] = "shared"
			}
			trace["vertex_flex_applied"] = true
		}
	}
	if len(trace) > 0 && strings.TrimSpace(provider) != "" {
		trace["provider"] = strings.TrimSpace(provider)
	}
	return trace, nil
}

func proxyParseJSONObject(raw, label string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON object: %w", label, err)
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}
	return obj, nil
}

func proxyApplyExtraHeaders(headers map[string]string, extra map[string]any) ([]string, []string) {
	applied := []string{}
	blocked := []string{}
	for key, value := range extra {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		if proxyProtectedHeader(name) {
			blocked = append(blocked, name)
			continue
		}
		text, ok := proxyHeaderValue(value)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		headers[name] = text
		applied = append(applied, name)
	}
	return applied, blocked
}

func proxyHeaderValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case float64, bool:
		return fmt.Sprint(v), true
	default:
		return "", false
	}
}

func proxyProtectedHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "content-type", "accept", "x-goog-api-key", "x-api-key", "anthropic-version", "host", "content-length":
		return true
	default:
		return false
	}
}

func proxyMergeExtraBody(dst map[string]any, src map[string]any, path string) ([]string, []string) {
	applied := []string{}
	blocked := []string{}
	for key, value := range src {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		fullKey := name
		if path != "" {
			fullKey = path + "." + name
		}
		if path == "" && proxyProtectedBodyKey(name) {
			blocked = append(blocked, fullKey)
			continue
		}
		incomingMap, incomingIsMap := value.(map[string]any)
		if existingMap := mapFromAny(dst[name]); incomingIsMap && len(existingMap) > 0 {
			nestedApplied, nestedBlocked := proxyMergeExtraBody(existingMap, incomingMap, fullKey)
			dst[name] = existingMap
			applied = append(applied, nestedApplied...)
			blocked = append(blocked, nestedBlocked...)
			continue
		}
		dst[name] = value
		applied = append(applied, fullKey)
	}
	return applied, blocked
}

func proxyProtectedBodyKey(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "model", "messages", "contents", "system", "systeminstruction", "api_key", "apikey", "provider", "endpoint", "stream":
		return true
	default:
		return false
	}
}

func proxyNormalizeVertexFlexMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "", "off", "disabled", "disable", "none":
		return "off"
	case "provisioned_then_flex", "provisioned_flex":
		return "provisioned_then_flex"
	case "flex_only", "shared":
		return "flex_only"
	default:
		return "off"
	}
}

func proxyNormalizeLLMGatewayServiceTier(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "":
		return "", true
	case "standard", "default", "auto":
		return "default", true
	case "flex":
		return "flex", true
	case "priority":
		return "priority", true
	default:
		return "", false
	}
}

func proxyApplyLLMGatewayServiceTier(body map[string]any, req dto.ProxyPluginMainRequest, provider string, trace map[string]any) error {
	rawTier := strings.TrimSpace(stringPtrValue(req.LLMGatewayServiceTier, ""))
	if rawTier == "" {
		return nil
	}
	tier, ok := proxyNormalizeLLMGatewayServiceTier(rawTier)
	if !ok {
		return fmt.Errorf("llm_gateway_service_tier must be standard, flex, or priority")
	}
	trace["llm_gateway_service_tier_requested"] = tier
	if !proxyProviderSupportsServiceTier(provider) {
		trace["llm_gateway_service_tier_applied"] = false
		trace["llm_gateway_service_tier_skip_reason"] = "provider_not_openai_compatible_service_tier"
		return fmt.Errorf("llm_gateway_service_tier requires provider openai, llmgateway, vercel, or custom")
	}
	if existing, exists := body["service_tier"]; exists {
		existingText, isString := existing.(string)
		existingTier, valid := proxyNormalizeLLMGatewayServiceTier(existingText)
		if !isString || !valid || existingTier == "" || existingTier != tier {
			trace["llm_gateway_service_tier_applied"] = false
			trace["llm_gateway_service_tier_conflict"] = true
			return fmt.Errorf("llm_gateway_service_tier conflicts with extra_body_json service_tier")
		}
		trace["llm_gateway_service_tier_source"] = "typed_and_extra_body_json"
	} else {
		trace["llm_gateway_service_tier_source"] = "typed_setting"
	}
	body["service_tier"] = tier
	trace["llm_gateway_service_tier_applied"] = true
	return nil
}

func proxyProviderSupportsServiceTier(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "llmgateway", "vercel", "custom":
		return true
	default:
		return false
	}
}

func proxyNormalizeClaudePromptCacheMode(value string) (string, bool) {
	normalized := strings.TrimSpace(value)
	switch normalized {
	case "":
		return "", true
	case "off":
		return "off", true
	case "ephemeral_5m":
		return "ephemeral_5m", true
	case "ephemeral_1h":
		return "ephemeral_1h", true
	default:
		return "", false
	}
}

func proxyClaudePromptCacheControl(mode string) map[string]any {
	cacheControl := map[string]any{"type": "ephemeral"}
	if mode == "ephemeral_1h" {
		cacheControl["ttl"] = "1h"
	}
	return cacheControl
}

func proxyClaudePromptCacheControlMatches(value any, mode string) bool {
	cacheControl, ok := value.(map[string]any)
	if !ok || extractionStringFromAny(cacheControl["type"]) != "ephemeral" {
		return false
	}
	switch mode {
	case "ephemeral_5m":
		ttl, hasTTL := cacheControl["ttl"]
		return (len(cacheControl) == 1 && !hasTTL) ||
			(len(cacheControl) == 2 && extractionStringFromAny(ttl) == "5m")
	case "ephemeral_1h":
		return len(cacheControl) == 2 && extractionStringFromAny(cacheControl["ttl"]) == "1h"
	default:
		return false
	}
}

func proxyApplyClaudePromptCacheMode(body map[string]any, req dto.ProxyPluginMainRequest, provider string, trace map[string]any) error {
	rawMode := strings.TrimSpace(stringPtrValue(req.ClaudePromptCacheMode, ""))
	if rawMode == "" {
		return nil
	}
	mode, ok := proxyNormalizeClaudePromptCacheMode(rawMode)
	if !ok {
		return fmt.Errorf("claude_prompt_cache_mode must be off, ephemeral_5m, or ephemeral_1h")
	}
	trace["claude_prompt_cache_mode_requested"] = mode
	if mode == "off" {
		trace["claude_prompt_cache_mode_applied"] = false
		trace["claude_prompt_cache_mode_source"] = "typed_off"
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(provider), "claude") {
		trace["claude_prompt_cache_mode_applied"] = false
		trace["claude_prompt_cache_mode_skip_reason"] = "provider_not_claude"
		return fmt.Errorf("claude_prompt_cache_mode requires provider claude")
	}
	if existing, exists := body["cache_control"]; exists {
		if !proxyClaudePromptCacheControlMatches(existing, mode) {
			trace["claude_prompt_cache_mode_applied"] = false
			trace["claude_prompt_cache_mode_conflict"] = true
			return fmt.Errorf("claude_prompt_cache_mode conflicts with extra_body_json cache_control")
		}
		trace["claude_prompt_cache_mode_source"] = "typed_and_extra_body_json"
	} else {
		trace["claude_prompt_cache_mode_source"] = "typed_setting"
	}
	body["cache_control"] = proxyClaudePromptCacheControl(mode)
	trace["claude_prompt_cache_mode_applied"] = true
	return nil
}

func proxyAttachClaudeUsage(resp, upstream map[string]any, trace map[string]any) {
	if resp == nil || upstream == nil {
		return
	}
	usage, ok := upstream["usage"].(map[string]any)
	if !ok || len(usage) == 0 {
		return
	}
	resp["usage"] = usage
	usageTrace := map[string]any{}
	for _, key := range []string{"cache_creation_input_tokens", "cache_read_input_tokens", "service_tier"} {
		if value, exists := usage[key]; exists {
			usageTrace[key] = value
		}
	}
	if len(usageTrace) > 0 {
		trace["anthropic_usage"] = usageTrace
	}
}

func proxyAttachGeminiUsage(resp, upstream map[string]any, trace map[string]any) {
	if resp == nil || upstream == nil {
		return
	}
	usage, ok := upstream["usageMetadata"].(map[string]any)
	if !ok || len(usage) == 0 {
		return
	}
	resp["usageMetadata"] = usage
	usageTrace := map[string]any{}
	for _, key := range []string{
		"promptTokenCount",
		"candidatesTokenCount",
		"totalTokenCount",
		"cachedContentTokenCount",
		"trafficType",
	} {
		if value, exists := usage[key]; exists {
			usageTrace[key] = value
		}
	}
	if len(usageTrace) > 0 {
		trace["gemini_usage"] = usageTrace
	}
}

func proxyAttachLLMGatewayServiceTierTrace(resp map[string]any, trace map[string]any) {
	if resp == nil || trace["llm_gateway_service_tier_applied"] != true {
		return
	}
	served := strings.TrimSpace(extractionStringFromAny(resp["service_tier"]))
	if served == "" {
		served = "not_reported"
	}
	trace["llm_gateway_service_tier_served"] = served
}

func proxyAttachRequestOverrideTrace(resp map[string]any, trace map[string]any) {
	if len(trace) == 0 || resp == nil {
		return
	}
	resp["_proxy_request_overrides"] = trace
}

func proxyOpenAIBaseURL(provider, endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint != "" {
		return endpoint
	}
	switch provider {
	case "openrouter":
		return "https://openrouter.ai/api"
	case "llmgateway":
		return "https://api.llmgateway.io/v1"
	case "vercel":
		return "https://ai-gateway.vercel.sh/v1"
	case "copilot":
		return "https://api.githubcopilot.com"
	default:
		return "https://api.openai.com"
	}
}

func proxyOpenAIChatEndpoint(base, provider string, isGLM bool) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	if provider == "copilot" || isGLM {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func proxyNormalizeGeminiEndpoint(endpoint, model, action string) string {
	if action != "embedContent" {
		action = "generateContent"
	}
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	if !regexp.MustCompile(`(?i)generativelanguage\.googleapis\.com`).MatchString(base) {
		if regexp.MustCompile(`:[a-zA-Z]+$`).MatchString(base) {
			return base
		}
		return base + "/models/" + strings.TrimSpace(model) + ":" + action
	}
	if !regexp.MustCompile(`(?i)/v[0-9][^/]*$`).MatchString(base) && !regexp.MustCompile(`(?i)/v[0-9][^/]*/models/`).MatchString(base) {
		base += "/v1beta"
	}
	if regexp.MustCompile(`(?i):` + regexp.QuoteMeta(action) + `$`).MatchString(base) {
		return base
	}
	if regexp.MustCompile(`(?i)/models/[^/:]+$`).MatchString(base) {
		return base + ":" + action
	}
	if strings.Contains(base, "/models/") {
		return base
	}
	return base + "/models/" + strings.TrimSpace(model) + ":" + action
}

func proxyNormalizeVertexEndpoint(endpoint, model string) string {
	base := proxyNormalizeVertexBaseEndpoint(endpoint)
	if strings.Contains(base, ":streamGenerateContent") {
		return strings.Replace(base, ":streamGenerateContent", ":generateContent", 1)
	}
	if strings.Contains(base, ":generateContent") {
		return base
	}
	return base + "/" + strings.TrimSpace(model) + ":generateContent"
}

func proxyNormalizeVertexBaseEndpoint(endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	lower := strings.ToLower(base)
	replacements := []struct {
		from string
		to   string
	}{
		{"https://global-aiplatform.googleapis.com", "https://aiplatform.googleapis.com"},
		{"http://global-aiplatform.googleapis.com", "http://aiplatform.googleapis.com"},
		{"https://us-aiplatform.googleapis.com", "https://aiplatform.us.rep.googleapis.com"},
		{"http://us-aiplatform.googleapis.com", "http://aiplatform.us.rep.googleapis.com"},
		{"https://eu-aiplatform.googleapis.com", "https://aiplatform.eu.rep.googleapis.com"},
		{"http://eu-aiplatform.googleapis.com", "http://aiplatform.eu.rep.googleapis.com"},
	}
	for _, item := range replacements {
		if strings.HasPrefix(lower, item.from) {
			return item.to + base[len(item.from):]
		}
	}
	return base
}

func proxyResolveVertexProjectID(endpoint, serviceAccountJSON string) (string, error) {
	if !strings.Contains(endpoint, "PROJECT_ID") {
		return endpoint, nil
	}
	var cred struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(serviceAccountJSON), &cred); err != nil {
		return "", fmt.Errorf("Vertex endpoint contains PROJECT_ID but the service account JSON could not be parsed")
	}
	projectID := strings.TrimSpace(cred.ProjectID)
	if projectID == "" {
		return "", fmt.Errorf("Vertex endpoint contains PROJECT_ID but the service account JSON is missing project_id")
	}
	return strings.ReplaceAll(endpoint, "PROJECT_ID", url.PathEscape(projectID)), nil
}

func proxyIsGLMLike(model, endpoint, provider string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	provider = strings.ToLower(strings.TrimSpace(provider))
	return regexp.MustCompile(`^glm[-\d.]`).MatchString(model) ||
		regexp.MustCompile(`(?:open\.)?bigmodel\.cn|zhipu`).MatchString(endpoint) ||
		(provider == "custom" && strings.HasPrefix(model, "glm"))
}

func proxyReasoningFamily(provider, preset, model, endpoint string) string {
	preset = strings.ToLower(strings.TrimSpace(preset))
	switch preset {
	case "gpt", "gemini", "claude", "glm":
		return preset
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if proxyIsGLMLike(model, endpoint, provider) {
		return "glm"
	}
	if provider == "claude" {
		return "claude"
	}
	if provider == "gemini" || provider == "vertex" {
		return "gemini"
	}
	return "gpt"
}

func proxyGeminiThinkingMode(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "gemini-2.5") {
		return "budget"
	}
	if regexp.MustCompile(`gemini-(?:3(?:\D|$)|[4-9](?:\D|$)|\d{2,}(?:\D|$))`).MatchString(model) {
		return "level"
	}
	return "budget"
}

func proxyGLMThinkingType(value string) string {
	if strings.ToLower(strings.TrimSpace(value)) == "disabled" {
		return "disabled"
	}
	return "enabled"
}

func proxyGLMThinkingTypeFromRequest(value, effort string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "enabled", "enable", "on", "true":
		return "enabled"
	case "disabled", "disable", "off", "false":
		return "disabled"
	}
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "none", "disable", "disabled", "off", "false":
		return "disabled"
	default:
		return "enabled"
	}
}

func proxyGLM52ReasoningEffort(model, effort string) string {
	if !regexp.MustCompile(`(?i)\bglm[-_]?5\.2(?:\b|[-_])`).MatchString(strings.TrimSpace(model)) {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal", "low", "medium", "high", "xhigh", "max":
		return strings.ToLower(strings.TrimSpace(effort))
	default:
		return ""
	}
}

func proxySplitSystemUser(messages []any) (string, string) {
	var system []string
	var user []string
	for _, item := range messages {
		msg := mapFromAny(item)
		content := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if content == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(extractionStringFromAny(msg["role"])), "system") {
			system = append(system, content)
		} else {
			user = append(user, content)
		}
	}
	return strings.Join(system, "\n\n"), strings.Join(user, "\n\n")
}

func proxyExtractClaudeText(data map[string]any) string {
	blocks := sliceFromAny(data["content"])
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		item := mapFromAny(block)
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func proxyExtractGeminiText(data map[string]any) string {
	candidates := sliceFromAny(data["candidates"])
	if len(candidates) == 0 {
		return ""
	}
	content := mapFromAny(mapFromAny(candidates[0])["content"])
	parts := sliceFromAny(content["parts"])
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		item := mapFromAny(part)
		if thought, _ := item["thought"].(bool); thought {
			continue
		}
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		if text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n\n")
}

func proxyNormalizeChatResponse(content, model, finishReason string) map[string]any {
	return map[string]any{
		"model": model,
		"choices": []any{map[string]any{
			"finish_reason": finishReason,
			"message":       map[string]any{"role": "assistant", "content": content},
		}},
	}
}

func proxyErrorDetail(status int, data map[string]any, raw string) string {
	if data != nil {
		if text := strings.TrimSpace(extractionStringFromAny(data["error"])); text != "" {
			return text
		}
		errObj := mapFromAny(data["error"])
		if text := strings.TrimSpace(extractionStringFromAny(errObj["message"])); text != "" {
			return text
		}
		if text := strings.TrimSpace(extractionStringFromAny(data["message"])); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(raw); text != "" {
		if len(text) > 1000 {
			return text[:1000]
		}
		return text
	}
	return http.StatusText(status)
}

func proxyVertexEndpointErrorDetail(status int, target string, data map[string]any, raw string) string {
	detail := proxyErrorDetail(status, data, raw)
	lowerRaw := strings.ToLower(raw)
	if status == http.StatusNotFound &&
		strings.Contains(strings.ToLower(target), "aiplatform.googleapis.com") &&
		(strings.Contains(lowerRaw, "<!doctype html") || strings.Contains(lowerRaw, "error 404")) {
		return "Vertex endpoint returned Google HTML 404. Endpoint must include the model prefix up to /publishers/google/models. Regional example: https://us-central1-aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/us-central1/publishers/google/models. Global example: https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models. US multi-region example: https://aiplatform.us.rep.googleapis.com/v1/projects/PROJECT_ID/locations/us/publishers/google/models. Model should be only the model id, for example gemini-3.5-flash. Current target: " + target
	}
	return detail
}

func proxyUnsupportedParameter(raw string, data map[string]any) bool {
	text := strings.ToLower(proxyErrorDetail(http.StatusBadRequest, data, raw))
	for _, token := range []string{
		"unsupported parameter",
		"unknown parameter",
		"unrecognized parameter",
		"extra fields not permitted",
		"additional properties are not allowed",
	} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return strings.Contains(text, "invalid_request_error") && strings.Contains(text, "parameter")
}

func proxyServiceTierError(raw string, data map[string]any) bool {
	text := strings.ToLower(proxyErrorDetail(http.StatusBadRequest, data, raw))
	return strings.Contains(text, "unsupported_service_tier") ||
		strings.Contains(text, "service_tier") ||
		strings.Contains(text, "service tier")
}

func proxyHasAdvancedParams(body map[string]any) bool {
	for _, key := range []string{"reasoning_effort", "max_completion_tokens", "thinking"} {
		if _, ok := body[key]; ok {
			return true
		}
	}
	return false
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("Vertex private_key is not PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("Vertex private_key is not RSA")
}

func proxyBase64URL(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func mustJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func isGeminiThinkingLevel(value string) bool {
	switch value {
	case "minimal", "low", "medium", "high":
		return true
	default:
		return false
	}
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
