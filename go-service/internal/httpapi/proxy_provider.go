package httpapi

import (
	"bufio"
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
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
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
	SessionID    string
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

type proxyFinalOutputExhaustedError struct {
	Provider string
}

func (e *proxyFinalOutputExhaustedError) Error() string {
	provider := strings.TrimSpace(e.Provider)
	if provider == "" {
		provider = "provider"
	}
	return provider + " exhausted its output token budget before returning final text"
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
	apiKey := strings.TrimSpace(stringPtrValue(req.APIKey, ""))
	model := strings.TrimSpace(stringPtrValue(req.Model, ""))
	provider := strings.ToLower(strings.TrimSpace(stringPtrValue(req.Provider, "")))
	endpoint := proxyProviderBaseURL(provider, stringPtrValue(req.Endpoint, ""))
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
	case "opencode", "opencode-go":
		// Zen/Go exposes native APIs per model. An explicit API endpoint takes
		// precedence over the model's documented default transport.
		path := ""
		if parsed, err := url.Parse(endpoint); err == nil {
			path = strings.TrimRight(parsed.Path, "/")
		}
		modelID := strings.ToLower(model)
		switch {
		case strings.HasSuffix(path, "/chat/completions"), strings.HasSuffix(path, "/responses"):
			return proxyCallOpenAILike(ctx, req, endpoint, apiKey, model, provider, policy, retryBudget)
		case strings.HasSuffix(path, "/messages"):
			return proxyCallClaude(ctx, req, endpoint, apiKey, model, policy)
		case strings.Contains(path, "/models/"):
			if !strings.Contains(path, ":") {
				endpoint += ":generateContent"
			}
			return proxyCallGemini(ctx, req, endpoint, apiKey, model, false, policy)
		case strings.HasPrefix(modelID, "claude-"), strings.HasPrefix(modelID, "qwen3."), provider == "opencode-go" && strings.HasPrefix(modelID, "minimax-"):
			return proxyCallClaude(ctx, req, endpoint+"/messages", apiKey, model, policy)
		case strings.HasPrefix(modelID, "gemini-"):
			return proxyCallGemini(ctx, req, endpoint, apiKey, model, false, policy)
		case strings.HasPrefix(modelID, "gpt-"), strings.HasPrefix(modelID, "grok-"), strings.HasPrefix(modelID, "muse-spark-"):
			return proxyCallOpenAIResponses(ctx, req, endpoint+"/responses", apiKey, model, provider, policy)
		default:
			return proxyCallOpenAILike(ctx, req, endpoint, apiKey, model, provider, policy, retryBudget)
		}
	case "claude":
		return proxyCallClaude(ctx, req, endpoint, apiKey, model, policy)
	case "gemini":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, false, policy)
	case "vertex":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, true, policy)
	case "openai", "openrouter", "llmgateway", "vercel", "neuralwatt", "copilot", "ollama", "custom":
		return proxyCallOpenAILike(ctx, req, endpoint, apiKey, model, provider, policy, retryBudget)
	default:
		return nil, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "configuration",
			Cause: fmt.Errorf("unsupported provider %q", provider),
		}
	}
}

func proxyCallOpenAILike(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model, provider string, policy proxyRequestPolicy, retryBudget *llmRetryBudget) (map[string]any, int, error) {
	if proxyEndpointUsesResponses(endpoint) {
		return proxyCallOpenAIResponses(ctx, req, endpoint, apiKey, model, provider, policy)
	}
	isGLM := provider != "ollama" && proxyIsGLMLike(model, endpoint, provider)
	target := proxyOpenAIChatEndpoint(proxyProviderBaseURL(provider, endpoint), provider, isGLM)
	reasoningTransport, transportErr := proxyReasoningTransport(provider, endpoint)
	if transportErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "configuration", Cause: transportErr}
	}

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
	if reasoningTransport == "ollama" {
		if effort := proxyOllamaReasoningEffort(
			reasoningFamily,
			model,
			stringPtrValue(req.ReasoningEffort, ""),
			stringPtrValue(req.GlmThinkingType, ""),
		); effort != "" {
			outputTokens := maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
			reasoningBudget := maxInt64(0, firstPositiveInt64(
				int64Value(req.ReasoningBudgetTokens, 0),
				int64Value(req.BudgetTokens, 0),
			))
			body["reasoning_effort"] = effort
			body["max_tokens"] = outputTokens
			if effort != "none" {
				body["max_tokens"] = outputTokens + reasoningBudget
			}
		}
	} else if reasoningTransport == "llmgateway" || reasoningTransport == "neuralwatt" || ((reasoningTransport == "custom" || reasoningTransport == "opencode" || reasoningTransport == "opencode-go") && reasoningFamily == "deepseek_v4") {
		if effort := proxyGatewayReasoningEffort(reasoningTransport, reasoningFamily, model, stringPtrValue(req.ReasoningEffort, ""), stringPtrValue(req.GlmThinkingType, "")); effort != "" {
			body["reasoning_effort"] = effort
			body["max_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
			if reasoningFamily == "gpt" {
				delete(body, "temperature")
			}
		}
	} else if reasoningTransport == "openrouter" || reasoningTransport == "vercel" {
		if effort := proxyGatewayReasoningEffort(reasoningTransport, reasoningFamily, model, stringPtrValue(req.ReasoningEffort, ""), stringPtrValue(req.GlmThinkingType, "")); effort != "" {
			body["reasoning"] = map[string]any{"effort": effort}
			body["max_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
			if reasoningFamily == "gpt" {
				delete(body, "temperature")
			}
		}
	} else if reasoningFamily == "glm" {
		body["max_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
		effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
		thinkingType := proxyGLMThinkingTypeFromRequest(stringPtrValue(req.GlmThinkingType, ""), effort)
		body["thinking"] = map[string]any{
			"type": thinkingType,
		}
		if thinkingType == "enabled" && proxyGLMSupportsReasoningEffort(model) {
			switch effort {
			case "xhigh", "max":
				body["reasoning_effort"] = "max"
			case "enable", "enabled", "on", "true", "low", "medium", "high":
				body["reasoning_effort"] = "high"
			}
		}
	} else if reasoningFamily == "deepseek_v4" && reasoningTransport == "deepseek" {
		outputTokens := maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
		body["max_tokens"] = outputTokens
		effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
		normalizedEffort := "none"
		switch effort {
		case "low", "high", "max":
			normalizedEffort = effort
		case "medium":
			normalizedEffort = "high"
		case "xhigh":
			normalizedEffort = "max"
		}
		if normalizedEffort == "none" {
			body["thinking"] = map[string]any{"type": "disabled"}
		} else {
			body["thinking"] = map[string]any{"type": "enabled"}
			body["reasoning_effort"] = normalizedEffort
		}
	} else if reasoningFamily == "gpt" {
		if effort := proxyOpenAICompatibleReasoningEffort(model, stringPtrValue(req.ReasoningEffort, "")); effort != "" {
			body["reasoning_effort"] = effort
			body["max_completion_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
			delete(body, "max_tokens")
			if reasoningFamily == "gpt" {
				delete(body, "temperature")
			}
		}
	}
	managedReasoning := map[string][]byte{}
	for _, key := range []string{"reasoning_effort", "reasoning", "thinking"} {
		if value, ok := body[key]; ok {
			managedReasoning[key], _ = json.Marshal(value)
		}
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, provider, false, policy)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	for key, expected := range managedReasoning {
		actual, _ := json.Marshal(body[key])
		if !bytes.Equal(actual, expected) {
			return nil, http.StatusBadRequest, &proxyLocalRequestError{
				Stage: "request_build",
				Cause: fmt.Errorf("extra_body_json conflicts with backend-managed %s", key),
			}
		}
	}
	if policyErr := proxyApplyOpenAIJSONResponsePolicy(body, overrideTrace, provider, policy); policyErr != nil {
		return map[string]any{"_proxy_request_overrides": overrideTrace}, http.StatusBadRequest, &proxyLocalRequestError{
			Stage: "request_build",
			Cause: policyErr,
		}
	}
	neuralWattFlex := provider == "neuralwatt" && strings.EqualFold(strings.TrimSpace(extractionStringFromAny(body["service_tier"])), "flex")
	if neuralWattFlex {
		body["stream"] = true
		streamOptions := mapFromAny(body["stream_options"])
		streamOptions["include_usage"] = true
		body["stream_options"] = streamOptions
		headers["Accept"] = "text/event-stream"
		overrideTrace["neuralwatt_streaming_applied"] = true
		overrideTrace["neuralwatt_usage_stream_requested"] = true
	}
	if provider == "copilot" {
		token, status, err := proxyGetCopilotToken(ctx, apiKey)
		if err != nil {
			return nil, status, err
		}
		headers["Authorization"] = "Bearer " + token
	}

	var status int
	var data map[string]any
	var raw string
	var err error
	if neuralWattFlex {
		status, data, raw, err = proxyDoNeuralWattFlex(ctx, target, headers, body)
	} else {
		status, data, raw, err = proxyDoJSON(ctx, target, headers, body)
	}
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	_, hasReasoningEffort := body["reasoning_effort"]
	_, hasReasoningObject := body["reasoning"]
	_, hasThinking := body["thinking"]
	managedReasoningRequest := hasReasoningEffort || hasReasoningObject || hasThinking
	if status == http.StatusBadRequest &&
		!neuralWattFlex &&
		!managedReasoningRequest &&
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
		return data, status, fmt.Errorf("%s", scrubProxySecret(proxyErrorDetail(status, data, raw), apiKey))
	}
	if data == nil {
		return nil, http.StatusBadGateway, fmt.Errorf("OpenAI-like provider returned invalid JSON")
	}
	choice := map[string]any{}
	if choices := sliceFromAny(data["choices"]); len(choices) > 0 {
		choice = mapFromAny(choices[0])
	}
	responseMeta := buildProxyResponseMetadata(
		"openai_compatible",
		strings.TrimSpace(extractionStringFromAny(choice["finish_reason"])),
		mapFromAny(data["usage"]),
	)
	responseMeta["reasoning_observed"] = proxyChatReasoningObserved(data)
	data[proxyResponseMetadataKey] = responseMeta
	if policy.Purpose != "publisher" && strings.TrimSpace(chatCompletionText(data)) == "" {
		if stringFromMap(responseMeta, "termination_kind") == "length" {
			return data, http.StatusOK, &proxyFinalOutputExhaustedError{Provider: provider}
		}
		return data, http.StatusBadGateway, &proxyEmptyContentError{Provider: provider}
	}
	proxyAttachLLMGatewayServiceTierTrace(data, overrideTrace)
	proxyAttachRequestOverrideTrace(data, overrideTrace)
	return data, http.StatusOK, nil
}

func proxyEndpointUsesResponses(endpoint string) bool {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed == nil {
		return false
	}
	path := strings.ToLower(strings.TrimRight(strings.TrimSpace(parsed.Path), "/"))
	return strings.HasSuffix(path, "/responses")
}

func proxyCallOpenAIResponses(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model, provider string, policy proxyRequestPolicy) (map[string]any, int, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed == nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "configuration", Cause: fmt.Errorf("invalid Responses endpoint")}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	target := parsed.String()
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
		"model":             model,
		"input":             req.Messages,
		"max_output_tokens": maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens)),
		"stream":            false,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	reasoningEffort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
	switch reasoningEffort {
	case "none", "minimal", "low", "medium", "high", "xhigh":
		body["reasoning"] = map[string]any{"effort": reasoningEffort}
	}
	if (provider == "opencode" || provider == "opencode-go") && strings.HasPrefix(strings.ToLower(model), "gpt-") && reasoningEffort != "" && reasoningEffort != "none" {
		delete(body, "temperature")
	}
	managedReasoning, _ := json.Marshal(body["reasoning"])
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, provider, false, policy)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	if len(managedReasoning) > 0 {
		actual, _ := json.Marshal(body["reasoning"])
		if !bytes.Equal(actual, managedReasoning) {
			return nil, http.StatusBadRequest, &proxyLocalRequestError{
				Stage: "request_build",
				Cause: fmt.Errorf("extra_body_json conflicts with backend-managed reasoning"),
			}
		}
	}
	if policy.JSONResponse {
		overrideTrace["json_response_requested"] = true
		overrideTrace["json_response_purpose"] = strings.TrimSpace(policy.Purpose)
		overrideTrace["json_response_applied"] = false
		overrideTrace["json_response_skip_reason"] = "responses_endpoint_uses_prompt_contract"
	}
	if provider == "copilot" {
		token, status, tokenErr := proxyGetCopilotToken(ctx, apiKey)
		if tokenErr != nil {
			return nil, status, tokenErr
		}
		headers["Authorization"] = "Bearer " + token
	}

	status, data, raw, callErr := proxyDoJSON(ctx, target, headers, body)
	if callErr != nil {
		return nil, http.StatusBadGateway, callErr
	}
	if status < 200 || status >= 300 {
		return data, status, fmt.Errorf("%s", scrubProxySecret(proxyErrorDetail(status, data, raw), apiKey))
	}
	if data == nil {
		return nil, http.StatusBadGateway, fmt.Errorf("Responses provider returned invalid JSON")
	}

	content := proxyExtractResponsesText(data)
	finishReason := proxyResponsesFinishReason(data)
	responseModel := extractionFirstNonEmpty(extractionStringFromAny(data["model"]), model)
	resp := proxyNormalizeChatResponse(content, responseModel, finishReason)
	usage := mapFromAny(data["usage"])
	if len(usage) > 0 {
		resp["usage"] = usage
	}
	responseMeta := buildProxyResponseMetadata("openai_responses", finishReason, usage)
	responseMeta["response_status"] = strings.TrimSpace(extractionStringFromAny(data["status"]))
	responseMeta["reasoning_observed"] = proxyResponsesReasoningObserved(data)
	resp[proxyResponseMetadataKey] = responseMeta
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	if policy.Purpose != "publisher" && strings.TrimSpace(content) == "" {
		if stringFromMap(responseMeta, "termination_kind") == "length" {
			return resp, http.StatusOK, &proxyFinalOutputExhaustedError{Provider: provider}
		}
		return resp, http.StatusBadGateway, &proxyEmptyContentError{Provider: provider}
	}
	return resp, http.StatusOK, nil
}

func proxyExtractResponsesText(data map[string]any) string {
	var builder strings.Builder
	for _, rawItem := range sliceFromAny(data["output"]) {
		item := mapFromAny(rawItem)
		if !strings.EqualFold(strings.TrimSpace(extractionStringFromAny(item["type"])), "message") {
			continue
		}
		for _, rawPart := range sliceFromAny(item["content"]) {
			part := mapFromAny(rawPart)
			partType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(part["type"])))
			if partType != "output_text" && partType != "text" {
				continue
			}
			builder.WriteString(extractionStringFromAny(part["text"]))
		}
	}
	if builder.Len() > 0 {
		return builder.String()
	}
	return extractionStringFromAny(data["output_text"])
}

func proxyResponsesFinishReason(data map[string]any) string {
	if reason := strings.TrimSpace(extractionStringFromAny(mapFromAny(data["incomplete_details"])["reason"])); reason != "" {
		return reason
	}
	return strings.TrimSpace(extractionStringFromAny(data["status"]))
}

func proxyChatReasoningObserved(data map[string]any) bool {
	usage := mapFromAny(data["usage"])
	if intFromAny(mapFromAny(usage["completion_tokens_details"])["reasoning_tokens"], 0) > 0 ||
		intFromAny(mapFromAny(usage["output_tokens_details"])["reasoning_tokens"], 0) > 0 {
		return true
	}
	choices := sliceFromAny(data["choices"])
	if len(choices) == 0 {
		return false
	}
	choice := mapFromAny(choices[0])
	message := mapFromAny(choice["message"])
	for _, value := range []any{choice["reasoning"], choice["reasoning_content"], message["reasoning"], message["reasoning_content"]} {
		if strings.TrimSpace(extractionStringFromAny(value)) != "" || len(sliceFromAny(value)) > 0 || len(mapFromAny(value)) > 0 {
			return true
		}
	}
	for _, rawPart := range sliceFromAny(message["content"]) {
		partType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(mapFromAny(rawPart)["type"])))
		if strings.Contains(partType, "reasoning") || strings.Contains(partType, "thinking") {
			return true
		}
	}
	return false
}

func proxyResponsesReasoningObserved(data map[string]any) bool {
	if intFromAny(mapFromAny(mapFromAny(data["usage"])["output_tokens_details"])["reasoning_tokens"], 0) > 0 {
		return true
	}
	for _, rawItem := range sliceFromAny(data["output"]) {
		item := mapFromAny(rawItem)
		itemType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item["type"])))
		if itemType == "reasoning" {
			return true
		}
		for _, rawPart := range sliceFromAny(item["content"]) {
			partType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(mapFromAny(rawPart)["type"])))
			if strings.Contains(partType, "reasoning") || partType == "summary_text" {
				return true
			}
		}
	}
	return false
}

func proxyReasoningTransport(provider, endpoint string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	knownEndpoint := ""
	if parsed, err := url.Parse(strings.TrimSpace(endpoint)); err == nil {
		host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		switch host {
		case "api.openai.com":
			knownEndpoint = "openai"
		case "openrouter.ai":
			knownEndpoint = "openrouter"
		case "api.llmgateway.io":
			knownEndpoint = "llmgateway"
		case "api.neuralwatt.com":
			knownEndpoint = "neuralwatt"
		case "ai-gateway.vercel.sh":
			knownEndpoint = "vercel"
		case "api.deepseek.com":
			knownEndpoint = "deepseek"
		case "localhost", "127.0.0.1", "::1":
			if parsed.Port() == "11434" {
				knownEndpoint = "ollama"
			}
		}
	}

	if provider == "custom" {
		if knownEndpoint == "deepseek" {
			return "deepseek", nil
		}
		return "custom", nil
	}
	if provider == "openai" && knownEndpoint == "deepseek" {
		return "deepseek", nil
	}
	if knownEndpoint != "" && knownEndpoint != provider {
		return "", fmt.Errorf("provider %q conflicts with endpoint host for %q reasoning transport", provider, knownEndpoint)
	}
	return provider, nil
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
	maxTokens := maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
	switch proxyClaudeThinkingMode(model) {
	case "manual_budget":
		if budget >= 1024 {
			if budget >= maxTokens {
				return nil, http.StatusBadRequest, &proxyLocalRequestError{
					Stage: "configuration",
					Cause: fmt.Errorf("Claude thinking budget_tokens must be smaller than max_tokens"),
				}
			}
			body["max_tokens"] = maxTokens
			body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": budget}
			delete(body, "temperature")
		}
	case "adaptive":
		if effort := proxyClaudeAdaptiveEffort(stringPtrValue(req.ReasoningEffort, "")); effort != "" {
			body["max_tokens"] = maxTokens
			body["thinking"] = map[string]any{"type": "adaptive"}
			body["output_config"] = map[string]any{"effort": effort}
			delete(body, "temperature")
		}
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"Accept":            "application/json",
		"x-api-key":         apiKey,
		"anthropic-version": "2023-06-01",
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, "claude", false, policy)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, &proxyLocalRequestError{Stage: "request_build", Cause: overrideErr}
	}
	// OpenCode's non-Claude Messages routes share the wire format, not
	// Anthropic's structured-output capability. Keep their JSON prompt contract
	// and any explicitly configured output fields without adding output_config.
	jsonPolicy := policy
	requestProvider := strings.ToLower(stringPtrValue(req.Provider, ""))
	if (requestProvider == "opencode" || requestProvider == "opencode-go") && !strings.HasPrefix(strings.ToLower(model), "claude-") {
		jsonPolicy.JSONResponse = false
		overrideTrace["json_response_requested"] = policy.JSONResponse
		overrideTrace["json_response_source"] = "prompt_contract"
	}
	if policyErr := proxyApplyClaudeJSONResponsePolicy(body, overrideTrace, jsonPolicy); policyErr != nil {
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
		return data, status, fmt.Errorf("%s", scrubProxySecret(proxyErrorDetail(status, data, raw), apiKey))
	}
	content := proxyExtractClaudeText(data)
	finishReason := strings.TrimSpace(extractionStringFromAny(data["stop_reason"]))
	resp := proxyNormalizeChatResponse(content, model, finishReason)
	proxyAttachClaudeUsage(resp, data, overrideTrace)
	responseMeta := buildProxyResponseMetadata("anthropic_messages", finishReason, mapFromAny(data["usage"]))
	resp[proxyResponseMetadataKey] = responseMeta
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	if content == "" && policy.Purpose != "publisher" {
		if stringFromMap(responseMeta, "termination_kind") == "length" {
			return resp, status, &proxyFinalOutputExhaustedError{Provider: "claude"}
		}
		return resp, status, &proxyEmptyContentError{Provider: "claude"}
	}
	return resp, http.StatusOK, nil
}

func proxyCallGemini(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model string, vertex bool, policy proxyRequestPolicy) (map[string]any, int, error) {
	system, user := proxySplitSystemUser(req.Messages)
	requestedTokens := maxInt64(1, int64Value(req.MaxTokens, 1024))
	configuredMax := maxInt64(0, int64Value(req.MaxCompletionTokens, 0))
	budget := maxInt64(0, firstPositiveInt64(int64Value(req.ReasoningBudgetTokens, 0), int64Value(req.BudgetTokens, 0)))
	effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, "")))
	thinkingMode := proxyGeminiThinkingMode(model)
	isThinking := thinkingMode != "none"
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
	if thinkingMode == "level" {
		if level := proxyGeminiThinkingLevel(model, effort); level != "" {
			genCfg["thinkingConfig"] = map[string]any{"includeThoughts": false, "thinkingLevel": level}
		}
	} else if thinkingMode == "budget" && budget > 0 {
		genCfg["thinkingConfig"] = map[string]any{"includeThoughts": false, "thinkingBudget": budget}
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
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, geminiProvider, vertex, policy)
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
		return data, status, fmt.Errorf("%s", scrubProxySecret(detail, apiKey))
	}
	content := proxyExtractGeminiText(data)
	candidate := map[string]any{}
	if candidates := sliceFromAny(data["candidates"]); len(candidates) > 0 {
		candidate = mapFromAny(candidates[0])
	}
	finishReason := strings.TrimSpace(extractionStringFromAny(candidate["finishReason"]))
	resp := proxyNormalizeChatResponse(content, model, finishReason)
	proxyAttachGeminiUsage(resp, data, overrideTrace)
	responseMeta := buildProxyResponseMetadata("google_generate_content", finishReason, mapFromAny(data["usageMetadata"]))
	resp[proxyResponseMetadataKey] = responseMeta
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	if content == "" && policy.Purpose != "publisher" {
		if stringFromMap(responseMeta, "termination_kind") == "length" {
			return resp, status, &proxyFinalOutputExhaustedError{Provider: geminiProvider}
		}
		return resp, status, &proxyEmptyContentError{Provider: geminiProvider}
	}
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
	} else if existingText, stringValue := existing.(string); stringValue && strings.EqualFold(strings.TrimSpace(existingText), requiredMIME) {
		trace["json_response_applied"] = true
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_mime_type"] = existingText
	} else {
		existingText, stringValue := existing.(string)
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
	if !proxyJSONResponsePurposeIsPublisher(policy) {
		return nil
	}
	if _, exists := generationConfig["responseJsonSchema"]; !exists {
		generationConfig["responseJsonSchema"] = proxyPublisherTopLevelJSONSchema()
		trace["json_response_schema_source"] = "backend_policy"
	} else {
		if !proxyPublisherSchemaMatches(generationConfig["responseJsonSchema"]) {
			trace["json_response_applied"] = false
			trace["json_response_schema_source"] = "extra_body_json"
			trace["json_response_conflict"] = true
			trace["json_response_conflict_reason"] = "generationConfig.responseJsonSchema must match publisher_output.v3"
			return fmt.Errorf("json_response_schema_conflict: generationConfig.responseJsonSchema must match publisher_output.v3")
		}
		trace["json_response_schema_source"] = "extra_body_json"
	}
	trace["json_response_schema_contract"] = publisherWireContractVersion
	return nil
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

	providerSupportsNativeJSON := proxyProviderSupportsAutomaticOpenAIJSONResponse(provider)
	providerSupportsPublisherJSONObject := proxyJSONResponsePurposeIsPublisher(policy)
	providerUsesStrictPublisherSchema := providerSupportsPublisherJSONObject && proxyProviderUsesStrictPublisherSchemaByDefault(provider)

	const requiredType = "json_object"
	existing, exists := body["response_format"]
	if !exists {
		if providerSupportsPublisherJSONObject && !providerUsesStrictPublisherSchema {
			body["response_format"] = map[string]any{"type": requiredType}
			trace["json_response_applied"] = true
			trace["json_response_source"] = "backend_policy"
			trace["json_response_format"] = requiredType
			trace["json_response_schema_contract"] = publisherWireContractVersion + "_prompt_validated"
			trace["json_response_schema_source"] = "system_prompt"
			return nil
		}
		if !providerSupportsNativeJSON {
			trace["json_response_applied"] = false
			trace["json_response_source"] = "backend_policy"
			trace["json_response_skip_reason"] = "provider_native_contract_not_verified"
			return nil
		}
		appliedType := requiredType
		if providerUsesStrictPublisherSchema || strings.EqualFold(strings.TrimSpace(provider), "vercel") {
			appliedType = "json_schema"
			schemaName, schema := proxyJSONResponseSchema(policy)
			jsonSchema := map[string]any{
				"name":   schemaName,
				"schema": schema,
			}
			if proxyJSONResponsePurposeIsPublisher(policy) {
				jsonSchema["strict"] = true
			}
			body["response_format"] = map[string]any{
				"type":        appliedType,
				"json_schema": jsonSchema,
			}
		} else {
			body["response_format"] = map[string]any{"type": appliedType}
		}
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_format"] = appliedType
		if proxyJSONResponsePurposeIsPublisher(policy) {
			trace["json_response_schema_contract"] = publisherWireContractVersion
			trace["json_response_schema_source"] = "backend_policy"
		}
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
		if providerUsesStrictPublisherSchema && formatType == requiredType {
			trace["json_response_applied"] = false
			trace["json_response_source"] = "extra_body_json"
			trace["json_response_conflict"] = true
			trace["json_response_conflict_reason"] = "publisher response_format.type=json_object would replace the required publisher_output.v3 schema"
			return fmt.Errorf("json_response_schema_conflict: publisher response_format.type=json_object would replace the required publisher_output.v3 schema")
		}
		if proxyJSONResponsePurposeIsPublisher(policy) && formatType == "json_schema" {
			jsonSchema := mapFromAny(format["json_schema"])
			if len(jsonSchema) == 0 || !proxyPublisherSchemaMatches(jsonSchema["schema"]) {
				trace["json_response_applied"] = false
				trace["json_response_source"] = "extra_body_json"
				trace["json_response_conflict"] = true
				trace["json_response_conflict_reason"] = "response_format.json_schema.schema must match publisher_output.v3"
				return fmt.Errorf("json_response_schema_conflict: response_format.json_schema.schema must match publisher_output.v3")
			}
			trace["json_response_schema_contract"] = publisherWireContractVersion
			trace["json_response_schema_source"] = "extra_body_json"
		}
		if proxyJSONResponsePurposeIsPublisher(policy) && formatType == requiredType && !providerUsesStrictPublisherSchema {
			trace["json_response_schema_contract"] = publisherWireContractVersion + "_prompt_validated"
			trace["json_response_schema_source"] = "system_prompt"
		}
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

// Aggregating gateways expose JSON-schema capability per model/provider
// mapping, not for every model behind the gateway. Publisher defaults therefore
// use the portable json_object contract on those routes. A caller may still
// supply the exact publisher_output.v3 json_schema explicitly when that exact
// mapping is known to support it.
func proxyProviderUsesStrictPublisherSchemaByDefault(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "openai")
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
		_, schema := proxyJSONResponseSchema(policy)
		body["output_config"] = map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"schema": schema,
			},
		}
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_format"] = "json_schema"
		if proxyJSONResponsePurposeIsPublisher(policy) {
			trace["json_response_schema_contract"] = publisherWireContractVersion
			trace["json_response_schema_source"] = "backend_policy"
		}
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
	existingFormat, formatExists := outputConfig["format"]
	if !formatExists {
		_, schema := proxyJSONResponseSchema(policy)
		outputConfig["format"] = map[string]any{"type": "json_schema", "schema": schema}
		trace["json_response_applied"] = true
		trace["json_response_source"] = "backend_policy"
		trace["json_response_format"] = "json_schema"
		if proxyJSONResponsePurposeIsPublisher(policy) {
			trace["json_response_schema_contract"] = publisherWireContractVersion
			trace["json_response_schema_source"] = "backend_policy"
		}
		return nil
	}
	format, ok := existingFormat.(map[string]any)
	if !ok {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config.format must be a JSON object"
		return fmt.Errorf("json_response_format_conflict: output_config.format must be a JSON object")
	}
	formatType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(format["type"])))
	schema, schemaOK := format["schema"].(map[string]any)
	if formatType != "json_schema" || !schemaOK {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config.format requires type json_schema and object schema"
		return fmt.Errorf("json_response_format_conflict: output_config.format requires type json_schema and object schema")
	}
	if proxyJSONResponsePurposeIsPublisher(policy) && !proxyPublisherSchemaMatches(schema) {
		trace["json_response_applied"] = false
		trace["json_response_source"] = "extra_body_json"
		trace["json_response_schema_source"] = "extra_body_json"
		trace["json_response_conflict"] = true
		trace["json_response_conflict_reason"] = "output_config.format.schema must match publisher_output.v3"
		return fmt.Errorf("json_response_schema_conflict: output_config.format.schema must match publisher_output.v3")
	}
	trace["json_response_applied"] = true
	trace["json_response_source"] = "extra_body_json"
	trace["json_response_format"] = "json_schema"
	if proxyJSONResponsePurposeIsPublisher(policy) {
		trace["json_response_schema_contract"] = publisherWireContractVersion
		trace["json_response_schema_source"] = "extra_body_json"
	}
	return nil
}

func proxyJSONResponsePurposeIsPublisher(policy proxyRequestPolicy) bool {
	return strings.EqualFold(strings.TrimSpace(policy.Purpose), "publisher")
}

func proxyJSONResponseSchema(policy proxyRequestPolicy) (string, map[string]any) {
	if proxyJSONResponsePurposeIsPublisher(policy) {
		return "archive_center_publisher_output_v3", proxyPublisherTopLevelJSONSchema()
	}
	return "archive_center_critic_output_v1", proxyCriticTopLevelJSONSchema()
}

func proxyPublisherTopLevelJSONSchema() map[string]any {
	commonProperties := func(fieldValues []string) map[string]any {
		return map[string]any{
			"role":  map[string]any{"type": "string", "enum": []string{"book_author", "director"}},
			"field": map[string]any{"type": "string", "enum": fieldValues},
			"text":  map[string]any{"type": "string", "minLength": 1},
			"source_refs": map[string]any{
				"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1,
			},
		}
	}
	standardItem := map[string]any{
		"type": "object",
		"properties": commonProperties([]string{
			"current_arc", "narrative_goal", "next_beats", "guardrails",
			"scene_mandate", "required_outcomes", "forbidden_moves",
		}),
		"required":             []string{"role", "field", "text", "source_refs"},
		"additionalProperties": false,
	}
	pressureProperties := commonProperties([]string{"pressure_level"})
	pressureProperties["role"] = map[string]any{"type": "string", "enum": []string{"director"}}
	pressureProperties["level"] = map[string]any{"type": "string", "enum": []string{"quiet", "low", "medium", "high"}}
	pressureItem := map[string]any{
		"type":                 "object",
		"properties":           pressureProperties,
		"required":             []string{"role", "field", "text", "source_refs", "level"},
		"additionalProperties": false,
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"contract_version": map[string]any{"type": "string", "enum": []string{publisherWireContractVersion}},
			"items": map[string]any{
				"type":  "array",
				"items": map[string]any{"anyOf": []any{standardItem, pressureItem}},
			},
		},
		"required":             []string{"contract_version", "items"},
		"additionalProperties": false,
	}
}

func proxyPublisherSchemaMatches(candidate any) bool {
	actual, actualErr := json.Marshal(candidate)
	expected, expectedErr := json.Marshal(proxyPublisherTopLevelJSONSchema())
	return actualErr == nil && expectedErr == nil && bytes.Equal(actual, expected)
}

func proxyCriticTopLevelJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"turn_summary":           map[string]any{"type": "string"},
			"importance_score":       map[string]any{"type": "number"},
			"emotional_intensity":    map[string]any{"type": "number"},
			"narrative_significance": map[string]any{"type": "number"},
		},
		"required":             []string{"turn_summary", "importance_score"},
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data = proxyAttachHTTPFailureMetadata(data, resp.StatusCode, resp.Header, time.Now().UTC())
	}
	return resp.StatusCode, data, raw, nil
}

func proxyDoNeuralWattFlex(ctx context.Context, target string, headers map[string]string, body map[string]any) (int, map[string]any, string, error) {
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		rawBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if readErr != nil {
			return http.StatusBadGateway, nil, "", readErr
		}
		raw := string(rawBytes)
		var data map[string]any
		if json.Unmarshal(rawBytes, &data) != nil {
			data = nil
		}
		data = proxyAttachHTTPFailureMetadata(data, resp.StatusCode, resp.Header, time.Now().UTC())
		return resp.StatusCode, data, raw, nil
	}

	var id, object, model, serviceTier, finishReason string
	var created any
	var content, reasoning strings.Builder
	var usage map[string]any
	var energy, cost any
	done := false
	seenChunk := false
	eventData := make([]string, 0, 1)
	applyEvent := func() error {
		if len(eventData) == 0 {
			return nil
		}
		rawEvent := strings.Join(eventData, "\n")
		eventData = eventData[:0]
		if strings.TrimSpace(rawEvent) == "[DONE]" {
			done = true
			return nil
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(rawEvent), &chunk); err != nil {
			return fmt.Errorf("NeuralWatt Flex stream returned invalid JSON chunk: %w", err)
		}
		seenChunk = true
		if value := strings.TrimSpace(extractionStringFromAny(chunk["id"])); value != "" {
			id = value
		}
		if value := strings.TrimSpace(extractionStringFromAny(chunk["object"])); value != "" {
			object = value
		}
		if value := strings.TrimSpace(extractionStringFromAny(chunk["model"])); value != "" {
			model = value
		}
		if value := strings.TrimSpace(extractionStringFromAny(chunk["service_tier"])); value != "" {
			serviceTier = value
		}
		if value, ok := chunk["created"]; ok {
			created = value
		}
		if value := mapFromAny(chunk["usage"]); len(value) > 0 {
			usage = value
		}
		for _, rawChoice := range sliceFromAny(chunk["choices"]) {
			choice := mapFromAny(rawChoice)
			delta := mapFromAny(choice["delta"])
			content.WriteString(extractionStringFromAny(delta["content"]))
			reasoningText := extractionStringFromAny(delta["reasoning_content"])
			if reasoningText == "" {
				reasoningText = extractionStringFromAny(delta["reasoning"])
			}
			reasoning.WriteString(reasoningText)
			if value := strings.TrimSpace(extractionStringFromAny(choice["finish_reason"])); value != "" {
				finishReason = value
			}
			break
		}
		return nil
	}
	applyComment := func(raw string) {
		trimmed := strings.TrimSpace(strings.TrimPrefix(raw, ":"))
		for _, target := range []struct {
			prefix string
			dst    *any
		}{{"energy", &energy}, {"cost", &cost}} {
			if !strings.HasPrefix(strings.ToLower(trimmed), target.prefix+" ") {
				continue
			}
			payload := strings.TrimSpace(trimmed[len(target.prefix):])
			var value any
			if json.Unmarshal([]byte(payload), &value) == nil {
				*target.dst = value
			}
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := applyEvent(); err != nil {
				return http.StatusBadGateway, nil, "", err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			applyComment(line)
			continue
		}
		if strings.HasPrefix(line, "data:") {
			eventData = append(eventData, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return http.StatusBadGateway, nil, "", err
	}
	if err := applyEvent(); err != nil {
		return http.StatusBadGateway, nil, "", err
	}
	if !seenChunk {
		return http.StatusBadGateway, nil, "", errors.New("NeuralWatt Flex stream returned no completion chunks")
	}
	if !done && finishReason == "" {
		return http.StatusBadGateway, nil, "", errors.New("NeuralWatt Flex stream ended before final completion marker")
	}
	if serviceTier == "" {
		serviceTier = strings.TrimSpace(resp.Header.Get("X-NW-Service-Tier"))
	}
	if object == "" || strings.HasSuffix(object, ".chunk") {
		object = "chat.completion"
	}
	message := map[string]any{"role": "assistant", "content": content.String()}
	if reasoning.Len() > 0 {
		message["reasoning_content"] = reasoning.String()
		message["reasoning"] = reasoning.String()
	}
	data := map[string]any{
		"id":           id,
		"object":       object,
		"created":      created,
		"model":        model,
		"service_tier": serviceTier,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
	}
	if len(usage) > 0 {
		data["usage"] = usage
	}
	if energy != nil {
		data["energy"] = energy
	}
	if cost != nil {
		data["cost"] = cost
	}
	return http.StatusOK, data, "", nil
}

func proxyApplyRequestOverrides(headers map[string]string, body map[string]any, req dto.ProxyPluginMainRequest, provider string, vertex bool, policies ...proxyRequestPolicy) (map[string]any, error) {
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

	if strings.EqualFold(stringPtrValue(req.Provider, ""), "opencode-go") {
		sid := "archive-center-auxiliary"
		if len(policies) > 0 && strings.TrimSpace(policies[0].SessionID) != "" {
			sid = strings.TrimSpace(policies[0].SessionID)
		}
		digest := sha256.Sum256([]byte(sid))
		hasSession, hasAgent := false, false
		for key := range headers {
			hasSession = hasSession || strings.EqualFold(key, "x-opencode-session")
			hasAgent = hasAgent || strings.EqualFold(key, "User-Agent")
		}
		if !hasSession {
			headers["x-opencode-session"] = fmt.Sprintf("archive-center-%x", digest[:16])
		}
		if !hasAgent {
			headers["User-Agent"] = "ArchiveCenter/4.3.0"
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
		if provider == "vertex" {
			// Provider switches retain the other transport's setting. Vertex
			// processing is selected separately by VertexFlexMode headers.
			trace["llm_gateway_service_tier_skip_reason"] = "vertex_uses_vertex_flex_mode"
			return nil
		}
		trace["llm_gateway_service_tier_skip_reason"] = "provider_not_openai_compatible_service_tier"
		return fmt.Errorf("llm_gateway_service_tier requires provider openai, llmgateway, vercel, neuralwatt, custom, or gemini")
	}
	bodyKey := "service_tier"
	if provider == "gemini" {
		bodyKey = "serviceTier"
	}
	if existing, exists := body[bodyKey]; exists {
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
	if provider == "gemini" && tier == "default" {
		body[bodyKey] = "standard"
	} else {
		body[bodyKey] = tier
	}
	trace["llm_gateway_service_tier_applied"] = true
	return nil
}

func proxyProviderSupportsServiceTier(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "llmgateway", "vercel", "neuralwatt", "custom", "gemini":
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

const proxyResponseMetadataKey = "_archive_center_response_meta"

func proxyAttachHTTPFailureMetadata(data map[string]any, status int, headers http.Header, now time.Time) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	meta := mapFromAny(data[proxyResponseMetadataKey])
	if len(meta) == 0 {
		meta = map[string]any{
			"contract_version": "archive_center.provider_response.v1",
			"adapter":          "http_error",
			"usage_reported":   false,
		}
	}
	meta["http_status"] = status
	if retryAfterSeconds := proxyRetryAfterSeconds(headers, data, now); retryAfterSeconds > 0 {
		meta["retry_after_seconds"] = retryAfterSeconds
	}
	data[proxyResponseMetadataKey] = meta
	return data
}

func proxyRetryAfterSeconds(headers http.Header, data map[string]any, now time.Time) int {
	seconds := proxyRetryAfterSecondsFromValue(data["retry_after"])
	if nested := mapFromAny(data["error"]); len(nested) > 0 {
		seconds = maxInt(seconds, proxyRetryAfterSecondsFromValue(nested["retry_after"]))
	}
	if raw := strings.TrimSpace(headers.Get("Retry-After")); raw != "" {
		if parsed := proxyRetryAfterSecondsFromValue(raw); parsed > 0 {
			seconds = maxInt(seconds, parsed)
		} else if retryAt, err := http.ParseTime(raw); err == nil {
			delay := retryAt.Sub(now)
			if delay > 0 {
				seconds = maxInt(seconds, int((delay+time.Second-1)/time.Second))
			}
		}
	}
	return seconds
}

func proxyRetryAfterSecondsFromValue(value any) int {
	var seconds float64
	switch typed := value.(type) {
	case float64:
		seconds = typed
	case float32:
		seconds = float64(typed)
	case int:
		seconds = float64(typed)
	case int64:
		seconds = float64(typed)
	case json.Number:
		seconds, _ = typed.Float64()
	case string:
		seconds, _ = strconv.ParseFloat(strings.TrimSpace(typed), 64)
	}
	if seconds <= 0 {
		return 0
	}
	return int(math.Ceil(seconds))
}

func buildProxyResponseMetadata(adapter, finishReason string, usage map[string]any) map[string]any {
	meta := map[string]any{
		"contract_version":     "archive_center.provider_response.v1",
		"adapter":              strings.TrimSpace(adapter),
		"native_finish_reason": strings.TrimSpace(finishReason),
		"termination_kind":     proxyTerminationKind(finishReason),
		"usage_reported":       len(usage) > 0,
	}
	inputTokens := firstPositiveInt(
		intFromAny(usage["prompt_tokens"], 0),
		intFromAny(usage["input_tokens"], 0),
		intFromAny(usage["promptTokenCount"], 0),
	)
	outputTokens := firstPositiveInt(
		intFromAny(usage["completion_tokens"], 0),
		intFromAny(usage["output_tokens"], 0),
		intFromAny(usage["candidatesTokenCount"], 0),
	)
	reasoningTokens := firstPositiveInt(
		intFromAny(mapFromAny(usage["completion_tokens_details"])["reasoning_tokens"], 0),
		intFromAny(mapFromAny(usage["output_tokens_details"])["reasoning_tokens"], 0),
		intFromAny(usage["thoughtsTokenCount"], 0),
	)
	totalTokens := firstPositiveInt(
		intFromAny(usage["total_tokens"], 0),
		intFromAny(usage["totalTokenCount"], 0),
	)
	if totalTokens <= 0 && inputTokens+outputTokens > 0 {
		totalTokens = inputTokens + outputTokens
	}
	cachedInputTokens := firstPositiveInt(
		intFromAny(mapFromAny(usage["prompt_tokens_details"])["cached_tokens"], 0),
		intFromAny(mapFromAny(usage["input_tokens_details"])["cached_tokens"], 0),
		intFromAny(usage["cache_read_input_tokens"], 0),
		intFromAny(usage["cachedContentTokenCount"], 0),
	)
	meta["input_tokens"] = inputTokens
	meta["output_tokens"] = outputTokens
	meta["reasoning_tokens"] = reasoningTokens
	meta["total_tokens"] = totalTokens
	meta["cached_input_tokens"] = cachedInputTokens
	return meta
}

func proxyTerminationKind(finishReason string) string {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "stop", "end_turn", "stop_sequence", "completed":
		return "complete"
	case "length", "max_tokens", "max_output_tokens", "model_context_window_exceeded":
		return "length"
	case "content_filter", "safety", "recitation", "prohibited_content", "blocked", "blocklist", "spii", "image_safety", "language":
		return "safety"
	case "tool_calls", "function_call", "tool_use":
		return "tool"
	default:
		return "unknown"
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

func proxyProviderBaseURL(provider, endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint != "" {
		return endpoint
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "https://api.openai.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "opencode":
		return "https://opencode.ai/zen/v1"
	case "opencode-go":
		return "https://opencode.ai/zen/go/v1"
	case "llmgateway":
		return "https://api.llmgateway.io/v1"
	case "vercel":
		return "https://ai-gateway.vercel.sh/v1"
	case "neuralwatt":
		return "https://api.neuralwatt.com/v1"
	case "copilot":
		return "https://api.githubcopilot.com"
	case "ollama":
		return "http://127.0.0.1:11434"
	case "claude":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "vertex":
		return "https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"
	default:
		return ""
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
	modelName := strings.ToLower(strings.TrimSpace(model))
	if regexp.MustCompile(`(^|/)deepseek[-_]?v4($|[-_:])`).MatchString(modelName) {
		return "deepseek_v4"
	}
	if regexp.MustCompile(`(^|/)gemini[-_]`).MatchString(modelName) {
		return "gemini"
	}
	if regexp.MustCompile(`(^|/)glm[-_]`).MatchString(modelName) {
		return "glm"
	}
	if regexp.MustCompile(`(^|/)claude[-_]`).MatchString(modelName) {
		return "claude"
	}
	if proxyOpenAIReasoningModel(modelName) {
		return "gpt"
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
	if provider == "ollama" && proxyOllamaThinkingModel(modelName) {
		return "ollama_thinking"
	}
	preset = strings.ToLower(strings.TrimSpace(preset))
	switch preset {
	case "gpt", "gemini", "claude", "glm":
		return preset
	default:
		return "none"
	}
}

func proxyGLMSupportsReasoningEffort(model string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(model)), "_", "-")
	match := regexp.MustCompile(`(?:^|/)glm-?(\d+)(?:[.-](\d+))?(?:$|[-_:])`).FindStringSubmatch(normalized)
	if len(match) == 0 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor := 0
	if len(match) > 2 && match[2] != "" {
		minor, _ = strconv.Atoi(match[2])
	}
	return major > 5 || (major == 5 && minor >= 2)
}

func proxyGeminiThinkingMode(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "gemini-2.5") {
		return "budget"
	}
	if regexp.MustCompile(`gemini-3(?:\D|$)`).MatchString(model) {
		return "level"
	}
	return "none"
}

func proxyGeminiThinkingLevel(model, level string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	level = strings.ToLower(strings.TrimSpace(level))
	allowed := map[string]bool{"low": true, "high": true}
	switch {
	case strings.Contains(model, "gemini-3.1-flash-lite-image"):
		allowed = map[string]bool{"minimal": true, "high": true}
	case strings.Contains(model, "gemini-3-pro-preview"):
		// low/high only
	case strings.Contains(model, "gemini-3.1-pro"), strings.Contains(model, "gemini-3.7-flash"), strings.Contains(model, "gemini-3.8-flash"):
		allowed["medium"] = true
	case regexp.MustCompile(`gemini-3(?:\.5|\.6)?-(?:flash|flash-lite)`).MatchString(model):
		allowed["minimal"] = true
		allowed["medium"] = true
	}
	if allowed[level] {
		return level
	}
	return ""
}

func proxyClaudeThinkingMode(model string) string {
	model = strings.NewReplacer(".", "-", "_", "-").Replace(strings.ToLower(strings.TrimSpace(model)))
	match := regexp.MustCompile(`claude(?:-[a-z]+)*-(\d+)(?:-(\d{1,2})(?:-|$))?`).FindStringSubmatch(model)
	if len(match) == 0 {
		return "none"
	}
	major, _ := strconv.Atoi(match[1])
	minor := -1
	if len(match) > 2 && match[2] != "" {
		minor, _ = strconv.Atoi(match[2])
	}
	if major >= 5 || (major == 4 && minor >= 6) {
		return "adaptive"
	}
	if (major == 3 && minor == 7) || (major == 4 && (minor < 0 || minor <= 5)) {
		return "manual_budget"
	}
	return "none"
}

func proxyClaudeAdaptiveEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal", "low":
		return "low"
	case "medium", "high":
		return strings.ToLower(strings.TrimSpace(effort))
	case "xhigh", "max":
		return "max"
	default:
		return ""
	}
}

func proxyOpenAIReasoningModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return regexp.MustCompile(`(^|/)(?:gpt[-_]?5(?:$|[-_.:])|o[134](?:$|[-_:]))`).MatchString(model)
}

func proxyOllamaThinkingModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return regexp.MustCompile(`(^|/)(?:gpt[-_]?oss|qwen3|deepseek[-_]?r1|deepseek[-_]?v3\.1)(?:$|[-_:])`).MatchString(model)
}

func proxyOpenAICompatibleReasoningEffort(model, effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	normalizedModel := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(model)), "_", "-")
	if regexp.MustCompile(`(^|/)gpt-?5\.6(?:$|[-_:])`).MatchString(normalizedModel) {
		switch effort {
		case "none", "low", "medium", "high", "xhigh", "max":
			return effort
		default:
			return ""
		}
	}
	if regexp.MustCompile(`(^|/)gpt-?5\.(?:2|5)(?:$|[-_:])`).MatchString(normalizedModel) {
		switch effort {
		case "none", "low", "medium", "high", "xhigh":
			return effort
		default:
			return ""
		}
	}
	if regexp.MustCompile(`(^|/)gpt-?5(?:$|[-_:])`).MatchString(normalizedModel) {
		switch effort {
		case "minimal", "low", "medium", "high":
			return effort
		default:
			return ""
		}
	}
	if !regexp.MustCompile(`(^|/)o[134](?:$|[-_:])`).MatchString(normalizedModel) {
		return ""
	}
	switch effort {
	case "minimal":
		return "low"
	case "low", "medium", "high":
		return effort
	default:
		return ""
	}
}

func proxyOllamaReasoningEffort(family, model, effort, glmThinkingType string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch family {
	case "glm":
		if proxyGLMThinkingTypeFromRequest(glmThinkingType, effort) == "disabled" {
			return "none"
		}
		return "high"
	case "deepseek_v4":
		switch effort {
		case "none":
			return effort
		case "minimal":
			return "low"
		case "low", "medium", "high":
			return effort
		case "xhigh", "max":
			return "high"
		default:
			return "none"
		}
	case "gpt":
		normalized := proxyOpenAICompatibleReasoningEffort(model, effort)
		switch normalized {
		case "none", "low", "medium", "high":
			return normalized
		case "minimal":
			return "low"
		case "xhigh", "max":
			return "high"
		default:
			return ""
		}
	case "gemini":
		if proxyGeminiThinkingMode(model) == "none" {
			return ""
		}
		fallthrough
	case "claude":
		if family == "claude" && proxyClaudeThinkingMode(model) == "none" {
			return ""
		}
		fallthrough
	case "ollama_thinking":
		switch effort {
		case "none":
			return "none"
		case "minimal":
			return "low"
		case "low", "medium", "high":
			return effort
		case "xhigh", "max":
			return "high"
		default:
			return ""
		}
	default:
		return ""
	}
}

func proxyGatewayReasoningEffort(transport, family, model, effort, glmThinkingType string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch family {
	case "glm":
		if proxyGLMThinkingTypeFromRequest(glmThinkingType, effort) == "disabled" {
			return "none"
		}
		return "high"
	case "deepseek_v4":
		switch effort {
		case "none", "high", "max":
			return effort
		case "low":
			if transport == "neuralwatt" && strings.Contains(strings.ToLower(strings.TrimSpace(model)), "flash") {
				return "high"
			}
			return "low"
		case "minimal", "medium":
			return "high"
		case "xhigh":
			return "max"
		default:
			return ""
		}
	case "gpt":
		return proxyOpenAICompatibleReasoningEffort(model, effort)
	case "gemini":
		if proxyGeminiThinkingMode(model) == "none" {
			return ""
		}
		if effort == "none" {
			return "none"
		}
		return proxyGeminiThinkingLevel(model, effort)
	case "claude":
		if proxyClaudeThinkingMode(model) == "none" {
			return ""
		}
		if effort == "none" {
			return "none"
		}
		return proxyClaudeAdaptiveEffort(effort)
	case "ollama_thinking":
		switch effort {
		case "none", "minimal", "low", "medium", "high", "xhigh", "max":
			return effort
		default:
			return ""
		}
	default:
		return ""
	}
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
		blockType := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item["type"])))
		if strings.Contains(blockType, "thinking") || strings.Contains(blockType, "reasoning") {
			continue
		}
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
