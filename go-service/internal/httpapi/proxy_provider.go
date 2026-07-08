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

func callProxyProvider(ctx context.Context, req dto.ProxyPluginMainRequest) (map[string]any, int, error) {
	endpoint := strings.TrimSpace(stringPtrValue(req.Endpoint, ""))
	apiKey := strings.TrimSpace(stringPtrValue(req.APIKey, ""))
	model := strings.TrimSpace(stringPtrValue(req.Model, ""))
	provider := strings.ToLower(strings.TrimSpace(stringPtrValue(req.Provider, "")))
	if provider == "" || endpoint == "" || apiKey == "" || model == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("provider / endpoint / api_key / model is required")
	}

	timeout := time.Duration(int64Value(req.TimeoutMs, 60000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch provider {
	case "claude":
		return proxyCallClaude(ctx, req, endpoint, apiKey, model)
	case "gemini":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, false)
	case "vertex":
		return proxyCallGemini(ctx, req, endpoint, apiKey, model, true)
	case "openai", "openrouter", "copilot", "ollama", "custom":
		return proxyCallOpenAILike(ctx, req, endpoint, apiKey, model, provider)
	default:
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported provider %q", provider)
	}
}

func proxyCallOpenAILike(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model, provider string) (map[string]any, int, error) {
	isGLM := proxyIsGLMLike(model, endpoint, provider)
	target := proxyOpenAIChatEndpoint(proxyOpenAIBaseURL(provider, endpoint), provider, isGLM)
	authToken := apiKey
	if provider == "copilot" {
		token, status, err := proxyGetCopilotToken(ctx, apiKey)
		if err != nil {
			return nil, status, err
		}
		authToken = token
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Accept":        "application/json",
		"Authorization": "Bearer " + authToken,
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
	} else if effort := strings.ToLower(strings.TrimSpace(stringPtrValue(req.ReasoningEffort, ""))); effort != "" && effort != "none" {
		body["reasoning_effort"] = effort
		body["max_completion_tokens"] = maxInt64(requestedTokens, firstPositiveInt64(configuredMax, requestedTokens))
		delete(body, "max_tokens")
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, provider, false)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, overrideErr
	}

	status, data, raw, err := proxyDoJSON(ctx, target, headers, body)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if status == http.StatusBadRequest && proxyHasAdvancedParams(body) && proxyUnsupportedParameter(raw, data) {
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
	proxyAttachRequestOverrideTrace(data, overrideTrace)
	return data, http.StatusOK, nil
}

func proxyCallClaude(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model string) (map[string]any, int, error) {
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
		return nil, http.StatusBadRequest, overrideErr
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
		return nil, http.StatusBadGateway, fmt.Errorf("Claude returned no text content")
	}
	resp := proxyNormalizeChatResponse(content, model, "stop")
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	return resp, http.StatusOK, nil
}

func proxyCallGemini(ctx context.Context, req dto.ProxyPluginMainRequest, endpoint, apiKey, model string, vertex bool) (map[string]any, int, error) {
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
	if vertex {
		token, status, err := proxyGetVertexAccessToken(ctx, apiKey)
		if err != nil {
			return nil, status, err
		}
		target = proxyNormalizeVertexEndpoint(endpoint, model)
		target, err = proxyResolveVertexProjectID(target, apiKey)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		headers["Authorization"] = "Bearer " + token
	} else {
		target = proxyNormalizeGeminiEndpoint(endpoint, model, "generateContent")
		headers["x-goog-api-key"] = apiKey
	}
	geminiProvider := "gemini"
	if vertex {
		geminiProvider = "vertex"
	}
	overrideTrace, overrideErr := proxyApplyRequestOverrides(headers, body, req, geminiProvider, vertex)
	if overrideErr != nil {
		return nil, http.StatusBadRequest, overrideErr
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
		return nil, http.StatusBadGateway, fmt.Errorf("Gemini/Vertex returned no text content")
	}
	resp := proxyNormalizeChatResponse(content, model, "stop")
	proxyAttachRequestOverrideTrace(resp, overrideTrace)
	return resp, http.StatusOK, nil
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
		return "", http.StatusBadRequest, fmt.Errorf("Vertex AI Key must be a JSON service account credential")
	}
	if strings.TrimSpace(cred.ClientEmail) == "" || strings.TrimSpace(cred.PrivateKey) == "" {
		return "", http.StatusBadRequest, fmt.Errorf("Vertex AI credentials missing client_email or private_key")
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
		return "", http.StatusBadRequest, err
	}
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", http.StatusBadGateway, err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signingInput+"."+proxyBase64URL(sig))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", http.StatusBadGateway, err
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
