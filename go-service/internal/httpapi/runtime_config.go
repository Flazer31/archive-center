package httpapi

import (
	"os"
	"strings"
)

// RuntimeConfig mirrors the 0.8 /config/update runtime settings that the JS
// bridge sends after the user saves the settings panel.
type RuntimeConfig struct {
	Synced                             bool
	MainProvider                       string
	MainAPIKey                         string
	MainEndpoint                       string
	MainModel                          string
	MainTimeoutSec                     int64
	MainTemperature                    *float64
	MainMaxTokens                      *int64
	MainReasoningPreset                string
	MainReasoningEffort                string
	MainReasoningBudget                *int64
	MainExtraHeadersJSON               string
	MainExtraBodyJSON                  string
	MainVertexFlexMode                 string
	MainLLMGatewayServiceTier          string
	MainClaudePromptCacheMode          string
	CriticProvider                     string
	CriticAPIKey                       string
	CriticEndpoint                     string
	CriticModel                        string
	CriticTimeoutSec                   int64
	CriticTemperature                  *float64
	CriticMaxTokens                    *int64
	CriticReasoningPreset              string
	CriticReasoningEffort              string
	CriticReasoningBudget              *int64
	CriticExtraHeadersJSON             string
	CriticExtraBodyJSON                string
	CriticVertexFlexMode               string
	CriticLLMGatewayServiceTier        string
	CriticClaudePromptCacheMode        string
	SupervisorProvider                 string
	SupervisorAPIKey                   string
	SupervisorEndpoint                 string
	SupervisorModel                    string
	SupervisorTimeoutSec               int64
	SupervisorTemperature              *float64
	SupervisorMaxTokens                *int64
	SupervisorReasoningPreset          string
	SupervisorReasoningEffort          string
	SupervisorReasoningBudget          *int64
	SupervisorExtraHeadersJSON         string
	SupervisorExtraBodyJSON            string
	SupervisorVertexFlexMode           string
	SupervisorLLMGatewayServiceTier    string
	SupervisorClaudePromptCacheMode    string
	EmbeddingProvider                  string
	EmbeddingAPIKey                    string
	EmbeddingEndpoint                  string
	EmbeddingModel                     string
	EmbeddingTimeoutSec                int64
	SourceSearchPlannerProvider        string
	SourceSearchPlannerAPIKey          string
	SourceSearchPlannerEndpoint        string
	SourceSearchPlannerModel           string
	SourceSearchPlannerTimeoutSec      int64
	SourceSearchPlannerTemperature     *float64
	SourceSearchPlannerMaxTokens       *int64
	SourceSearchPlannerReasoningPreset string
	SourceSearchPlannerReasoningEffort string
	SourceSearchPlannerReasoningBudget *int64
	LLMRetryCount                      int
	FailedQueueMaxAttempts             int
	TopK                               int64
}

type embeddingModelIdentity struct {
	Model  string
	Source string
}

type runtimeSourceValue struct {
	Value  string
	Source string
}

func firstRuntimeSourceValue(candidates ...runtimeSourceValue) runtimeSourceValue {
	for _, candidate := range candidates {
		if value := strings.TrimSpace(candidate.Value); value != "" {
			return runtimeSourceValue{Value: value, Source: candidate.Source}
		}
	}
	return runtimeSourceValue{Source: "unset"}
}

func addRuntimeSourceTrace(trace map[string]any, provider, apiKey, endpoint, model runtimeSourceValue) {
	trace["config_authority"] = "runtime_config"
	trace["provider_source"] = provider.Source
	trace["api_key_source"] = apiKey.Source
	trace["endpoint_source"] = endpoint.Source
	trace["model_source"] = model.Source
}

func (s *Server) currentEmbeddingModelIdentity() embeddingModelIdentity {
	if s != nil {
		rt := s.runtimeConfigSnapshot()
		if model := strings.TrimSpace(rt.EmbeddingModel); model != "" {
			return embeddingModelIdentity{Model: model, Source: "runtime.embeddingModel"}
		}
		if rt.Synced {
			return embeddingModelIdentity{Source: "unset.runtime.embeddingModel"}
		}
		if model := strings.TrimSpace(s.Cfg.EmbedderModel); model != "" {
			return embeddingModelIdentity{Model: model, Source: "config.AC_EMBEDDER_MODEL"}
		}
	}
	for _, item := range []struct {
		key    string
		source string
	}{
		{"AC_EMBEDDER_MODEL", "env.AC_EMBEDDER_MODEL"},
		{"AC_LT_EMBEDDING_MODEL", "env.AC_LT_EMBEDDING_MODEL"},
		{"PROJECT_EMBEDDING_MODEL", "env.PROJECT_EMBEDDING_MODEL"},
		{"AC_PROJECT_EMBEDDING_MODEL", "env.AC_PROJECT_EMBEDDING_MODEL"},
	} {
		if model := strings.TrimSpace(os.Getenv(item.key)); model != "" {
			return embeddingModelIdentity{Model: model, Source: item.source}
		}
	}
	return embeddingModelIdentity{Source: "unset"}
}

func (s *Server) currentProjectEmbeddingModel() string {
	return s.currentEmbeddingModelIdentity().Model
}

func embeddingEnvFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func embeddingEnvSource(keys ...string) runtimeSourceValue {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return runtimeSourceValue{Value: value, Source: "env." + key}
		}
	}
	return runtimeSourceValue{}
}

func (s *Server) updateRuntimeConfig(body map[string]any) []string {
	updated := []string{}
	s.RuntimeConfigMu.Lock()
	defer func() {
		s.RuntimeConfigMu.Unlock()
		s.wakeMemoryWorkers()
	}()
	s.RuntimeConfig.Synced = true

	setString := func(pluginKey string, target *string) {
		if value, ok := body[pluginKey]; ok {
			*target = strings.TrimSpace(extractionStringFromAny(value))
			updated = append(updated, pluginKey)
		}
	}
	setInt := func(pluginKey string, target *int64) {
		if value, ok := body[pluginKey]; ok {
			*target = int64(intFromAny(value, 0))
			updated = append(updated, pluginKey)
		}
	}
	setIntPtr := func(pluginKey string, target **int64) {
		if value, ok := body[pluginKey]; ok {
			parsed := int64(intFromAny(value, 0))
			*target = &parsed
			updated = append(updated, pluginKey)
		}
	}
	setFloatPtr := func(pluginKey string, target **float64) {
		if value, ok := body[pluginKey]; ok {
			parsed := extractionFloatFromAny(value, 0)
			if parsed < 0 {
				parsed = 0
			}
			if parsed > 2 {
				parsed = 2
			}
			*target = &parsed
			updated = append(updated, pluginKey)
		}
	}
	setClampedInt := func(pluginKey string, target *int, minimum, maximum int) {
		if value, ok := body[pluginKey]; ok {
			parsed := intFromAny(value, 0)
			if parsed < minimum {
				parsed = minimum
			}
			if parsed > maximum {
				parsed = maximum
			}
			*target = parsed
			updated = append(updated, pluginKey)
		}
	}

	setString("mainProvider", &s.RuntimeConfig.MainProvider)
	setString("mainApiKey", &s.RuntimeConfig.MainAPIKey)
	setString("mainEndpoint", &s.RuntimeConfig.MainEndpoint)
	setString("mainModel", &s.RuntimeConfig.MainModel)
	setInt("mainTimeout", &s.RuntimeConfig.MainTimeoutSec)
	setFloatPtr("mainTemperature", &s.RuntimeConfig.MainTemperature)
	setIntPtr("mainMaxCompletionTokens", &s.RuntimeConfig.MainMaxTokens)
	setString("mainReasoningPreset", &s.RuntimeConfig.MainReasoningPreset)
	setString("mainReasoningEffort", &s.RuntimeConfig.MainReasoningEffort)
	setIntPtr("mainReasoningBudgetTokens", &s.RuntimeConfig.MainReasoningBudget)
	setString("mainExtraHeadersJson", &s.RuntimeConfig.MainExtraHeadersJSON)
	setString("mainExtraBodyJson", &s.RuntimeConfig.MainExtraBodyJSON)
	setString("mainVertexFlexMode", &s.RuntimeConfig.MainVertexFlexMode)
	setString("mainLlmGatewayServiceTier", &s.RuntimeConfig.MainLLMGatewayServiceTier)
	setString("mainClaudePromptCacheMode", &s.RuntimeConfig.MainClaudePromptCacheMode)
	setString("criticProvider", &s.RuntimeConfig.CriticProvider)
	setString("criticApiKey", &s.RuntimeConfig.CriticAPIKey)
	setString("criticEndpoint", &s.RuntimeConfig.CriticEndpoint)
	setString("criticModel", &s.RuntimeConfig.CriticModel)
	setInt("criticTimeout", &s.RuntimeConfig.CriticTimeoutSec)
	setFloatPtr("criticTemperature", &s.RuntimeConfig.CriticTemperature)
	setIntPtr("criticMaxCompletionTokens", &s.RuntimeConfig.CriticMaxTokens)
	setString("criticReasoningPreset", &s.RuntimeConfig.CriticReasoningPreset)
	setString("criticReasoningEffort", &s.RuntimeConfig.CriticReasoningEffort)
	setIntPtr("criticReasoningBudgetTokens", &s.RuntimeConfig.CriticReasoningBudget)
	setString("criticExtraHeadersJson", &s.RuntimeConfig.CriticExtraHeadersJSON)
	setString("criticExtraBodyJson", &s.RuntimeConfig.CriticExtraBodyJSON)
	setString("criticVertexFlexMode", &s.RuntimeConfig.CriticVertexFlexMode)
	setString("criticLlmGatewayServiceTier", &s.RuntimeConfig.CriticLLMGatewayServiceTier)
	setString("criticClaudePromptCacheMode", &s.RuntimeConfig.CriticClaudePromptCacheMode)
	setString("supervisorProvider", &s.RuntimeConfig.SupervisorProvider)
	setString("supervisorApiKey", &s.RuntimeConfig.SupervisorAPIKey)
	setString("supervisorEndpoint", &s.RuntimeConfig.SupervisorEndpoint)
	setString("supervisorModel", &s.RuntimeConfig.SupervisorModel)
	setInt("supervisorTimeout", &s.RuntimeConfig.SupervisorTimeoutSec)
	setFloatPtr("supervisorTemperature", &s.RuntimeConfig.SupervisorTemperature)
	setIntPtr("supervisorMaxCompletionTokens", &s.RuntimeConfig.SupervisorMaxTokens)
	setString("supervisorReasoningPreset", &s.RuntimeConfig.SupervisorReasoningPreset)
	setString("supervisorReasoningEffort", &s.RuntimeConfig.SupervisorReasoningEffort)
	setIntPtr("supervisorReasoningBudgetTokens", &s.RuntimeConfig.SupervisorReasoningBudget)
	setString("supervisorExtraHeadersJson", &s.RuntimeConfig.SupervisorExtraHeadersJSON)
	setString("supervisorExtraBodyJson", &s.RuntimeConfig.SupervisorExtraBodyJSON)
	setString("supervisorVertexFlexMode", &s.RuntimeConfig.SupervisorVertexFlexMode)
	setString("supervisorLlmGatewayServiceTier", &s.RuntimeConfig.SupervisorLLMGatewayServiceTier)
	setString("supervisorClaudePromptCacheMode", &s.RuntimeConfig.SupervisorClaudePromptCacheMode)
	setString("embeddingProvider", &s.RuntimeConfig.EmbeddingProvider)
	setString("embeddingApiKey", &s.RuntimeConfig.EmbeddingAPIKey)
	setString("embeddingEndpoint", &s.RuntimeConfig.EmbeddingEndpoint)
	setString("embeddingModel", &s.RuntimeConfig.EmbeddingModel)
	setInt("embeddingTimeout", &s.RuntimeConfig.EmbeddingTimeoutSec)
	setString("sourceSearchPlannerProvider", &s.RuntimeConfig.SourceSearchPlannerProvider)
	setString("sourceSearchPlannerApiKey", &s.RuntimeConfig.SourceSearchPlannerAPIKey)
	setString("sourceSearchPlannerEndpoint", &s.RuntimeConfig.SourceSearchPlannerEndpoint)
	setString("sourceSearchPlannerModel", &s.RuntimeConfig.SourceSearchPlannerModel)
	setInt("sourceSearchPlannerTimeout", &s.RuntimeConfig.SourceSearchPlannerTimeoutSec)
	setFloatPtr("sourceSearchPlannerTemperature", &s.RuntimeConfig.SourceSearchPlannerTemperature)
	setIntPtr("sourceSearchPlannerMaxCompletionTokens", &s.RuntimeConfig.SourceSearchPlannerMaxTokens)
	setString("sourceSearchPlannerReasoningPreset", &s.RuntimeConfig.SourceSearchPlannerReasoningPreset)
	setString("sourceSearchPlannerReasoningEffort", &s.RuntimeConfig.SourceSearchPlannerReasoningEffort)
	setIntPtr("sourceSearchPlannerReasoningBudgetTokens", &s.RuntimeConfig.SourceSearchPlannerReasoningBudget)
	setClampedInt("llmRetryCount", &s.RuntimeConfig.LLMRetryCount, 0, 10)
	setClampedInt("failedQueueMaxAttempts", &s.RuntimeConfig.FailedQueueMaxAttempts, 1, 11)
	setInt("topK", &s.RuntimeConfig.TopK)

	return updated
}

func (s *Server) runtimeConfigSnapshot() RuntimeConfig {
	s.RuntimeConfigMu.RLock()
	defer s.RuntimeConfigMu.RUnlock()
	return s.RuntimeConfig
}

func runtimeTimeoutMs(seconds int64) int64 {
	if seconds <= 0 {
		return 0
	}
	return seconds * 1000
}

func (s *Server) supervisorLLMConfig() completeTurnLLMConfig {
	rt := s.runtimeConfigSnapshot()
	temperature := 0.3
	if rt.SupervisorTemperature != nil {
		temperature = *rt.SupervisorTemperature
	}
	maxTokens := int64(1200)
	if rt.SupervisorMaxTokens != nil && *rt.SupervisorMaxTokens > 0 {
		maxTokens = *rt.SupervisorMaxTokens
	}
	return completeTurnLLMConfig{
		APIKey:                rt.SupervisorAPIKey,
		Endpoint:              rt.SupervisorEndpoint,
		Model:                 rt.SupervisorModel,
		Provider:              rt.SupervisorProvider,
		TimeoutMs:             runtimeTimeoutMs(rt.SupervisorTimeoutSec),
		Temperature:           temperature,
		MaxTokens:             maxTokens,
		ReasoningPreset:       rt.SupervisorReasoningPreset,
		ReasoningEffort:       rt.SupervisorReasoningEffort,
		ReasoningBudgetTokens: int64PtrValue(rt.SupervisorReasoningBudget, 0),
		ExtraHeadersJSON:      rt.SupervisorExtraHeadersJSON,
		ExtraBodyJSON:         rt.SupervisorExtraBodyJSON,
		VertexFlexMode:        rt.SupervisorVertexFlexMode,
		LLMGatewayServiceTier: rt.SupervisorLLMGatewayServiceTier,
		ClaudePromptCacheMode: rt.SupervisorClaudePromptCacheMode,
		RetryBudget:           newLLMRetryBudget(rt.LLMRetryCount),
	}
}

func (s *Server) sourceSearchPlannerLLMConfig() completeTurnLLMConfig {
	rt := s.runtimeConfigSnapshot()
	temperature := 0.1
	if rt.SourceSearchPlannerTemperature != nil {
		temperature = *rt.SourceSearchPlannerTemperature
	}
	maxTokens := int64(512)
	if rt.SourceSearchPlannerMaxTokens != nil && *rt.SourceSearchPlannerMaxTokens > 0 {
		maxTokens = *rt.SourceSearchPlannerMaxTokens
	}
	reasoningPreset := strings.TrimSpace(rt.SourceSearchPlannerReasoningPreset)
	if reasoningPreset == "" {
		reasoningPreset = "auto"
	}
	reasoningEffort := strings.TrimSpace(rt.SourceSearchPlannerReasoningEffort)
	if reasoningEffort == "" {
		reasoningEffort = "none"
	}
	return completeTurnLLMConfig{
		APIKey: rt.SourceSearchPlannerAPIKey, Endpoint: rt.SourceSearchPlannerEndpoint,
		Model: rt.SourceSearchPlannerModel, Provider: rt.SourceSearchPlannerProvider,
		TimeoutMs:   runtimeTimeoutMs(rt.SourceSearchPlannerTimeoutSec),
		Temperature: temperature, MaxTokens: maxTokens,
		ReasoningPreset:       reasoningPreset,
		ReasoningEffort:       reasoningEffort,
		ReasoningBudgetTokens: int64PtrValue(rt.SourceSearchPlannerReasoningBudget, 0),
		RetryBudget:           newLLMRetryBudget(rt.LLMRetryCount),
	}
}

func (s *Server) chapterLLMConfig() completeTurnLLMConfig {
	rt := s.runtimeConfigSnapshot()
	temperature := 0.3
	if rt.MainTemperature != nil {
		temperature = *rt.MainTemperature
	}
	maxTokens := int64(1400)
	if rt.MainMaxTokens != nil && *rt.MainMaxTokens > 0 {
		maxTokens = *rt.MainMaxTokens
	}
	return completeTurnLLMConfig{
		APIKey:                rt.MainAPIKey,
		Endpoint:              rt.MainEndpoint,
		Model:                 rt.MainModel,
		Provider:              rt.MainProvider,
		TimeoutMs:             runtimeTimeoutMs(rt.MainTimeoutSec),
		Temperature:           temperature,
		MaxTokens:             maxTokens,
		ReasoningPreset:       rt.MainReasoningPreset,
		ReasoningEffort:       rt.MainReasoningEffort,
		ReasoningBudgetTokens: int64PtrValue(rt.MainReasoningBudget, 0),
		ExtraHeadersJSON:      rt.MainExtraHeadersJSON,
		ExtraBodyJSON:         rt.MainExtraBodyJSON,
		VertexFlexMode:        rt.MainVertexFlexMode,
		LLMGatewayServiceTier: rt.MainLLMGatewayServiceTier,
		ClaudePromptCacheMode: rt.MainClaudePromptCacheMode,
		RetryBudget:           newLLMRetryBudget(rt.LLMRetryCount),
	}
}

func configMissingFieldsWithProvider(provider, apiKey, endpoint, model string) []string {
	missing := []string{}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		missing = append(missing, "provider")
	}
	if strings.TrimSpace(apiKey) == "" && !strings.EqualFold(provider, "ollama") {
		missing = append(missing, "api_key")
	}
	if strings.TrimSpace(endpoint) == "" {
		missing = append(missing, "endpoint")
	}
	if strings.TrimSpace(model) == "" {
		missing = append(missing, "model")
	}
	return missing
}

func configuredTrace(provider, apiKey, endpoint, model string, timeoutSec int64) map[string]any {
	missing := configMissingFieldsWithProvider(provider, apiKey, endpoint, model)
	if timeoutSec <= 0 {
		missing = append(missing, "timeout_ms")
	}
	return map[string]any{
		"configured":     len(missing) == 0,
		"provider":       strings.TrimSpace(provider),
		"endpoint_host":  endpointHost(endpoint),
		"model":          strings.TrimSpace(model),
		"timeout_sec":    timeoutSec,
		"missing_fields": missing,
	}
}

func sourceSearchConfigMissingFields(provider, apiKey, model string) []string {
	missing := []string{}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "gemini", "claude", "ollama":
	default:
		missing = append(missing, "provider")
	}
	if strings.TrimSpace(apiKey) == "" {
		missing = append(missing, "api_key")
	}
	if strings.TrimSpace(model) == "" {
		missing = append(missing, "model")
	}
	return missing
}

func sourceSearchConfiguredTrace(provider, apiKey, endpoint, model string, timeoutSec int64) map[string]any {
	missing := sourceSearchConfigMissingFields(provider, apiKey, model)
	return map[string]any{
		"configured":     len(missing) == 0,
		"provider":       strings.TrimSpace(provider),
		"endpoint_host":  endpointHost(endpoint),
		"model":          strings.TrimSpace(model),
		"timeout_sec":    timeoutSec,
		"missing_fields": missing,
	}
}

func addOptionalRuntimeTraceFields(trace map[string]any, temperature *float64, maxTokens *int64) {
	if temperature != nil {
		trace["temperature"] = *temperature
	}
	if maxTokens != nil {
		trace["max_completion_tokens"] = *maxTokens
	}
}

func addOptionalReasoningTraceFields(trace map[string]any, preset, effort string, budget *int64) {
	if strings.TrimSpace(preset) != "" {
		trace["reasoning_preset"] = strings.TrimSpace(preset)
	}
	if strings.TrimSpace(effort) != "" {
		trace["reasoning_effort"] = strings.TrimSpace(effort)
	}
	if budget != nil {
		trace["reasoning_budget_tokens"] = *budget
	}
	if strings.EqualFold(strings.TrimSpace(preset), "glm") {
		switch strings.ToLower(strings.TrimSpace(effort)) {
		case "enable", "enabled", "on", "true", "minimal", "low", "medium", "high", "xhigh", "max":
			trace["glm_thinking_type"] = "enabled"
		case "none", "disable", "disabled", "off", "false":
			trace["glm_thinking_type"] = "disabled"
		}
	}
}

func endpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	withoutScheme := endpoint
	if i := strings.Index(withoutScheme, "://"); i >= 0 {
		withoutScheme = withoutScheme[i+3:]
	}
	if i := strings.IndexAny(withoutScheme, "/?#"); i >= 0 {
		withoutScheme = withoutScheme[:i]
	}
	if i := strings.LastIndex(withoutScheme, "@"); i >= 0 {
		withoutScheme = withoutScheme[i+1:]
	}
	return withoutScheme
}

func (s *Server) runtimeConfigTrace() map[string]any {
	rt := s.runtimeConfigSnapshot()
	mainProviderID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.MainProvider, Source: "runtime.mainProvider"})
	mainAPIKeyID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.MainAPIKey, Source: "runtime.mainApiKey"})
	mainEndpointID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.MainEndpoint, Source: "runtime.mainEndpoint"})
	mainModelID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.MainModel, Source: "runtime.mainModel"})
	supervisorAPIKeyID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SupervisorAPIKey, Source: "runtime.supervisorApiKey"})
	supervisorEndpointID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SupervisorEndpoint, Source: "runtime.supervisorEndpoint"})
	supervisorModelID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SupervisorModel, Source: "runtime.supervisorModel"})
	supervisorProviderID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SupervisorProvider, Source: "runtime.supervisorProvider"})
	criticAPIKeyID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.CriticAPIKey, Source: "runtime.criticApiKey"})
	criticEndpointID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.CriticEndpoint, Source: "runtime.criticEndpoint"})
	criticModelID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.CriticModel, Source: "runtime.criticModel"})
	criticProviderID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.CriticProvider, Source: "runtime.criticProvider"})
	embeddingProviderID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.EmbeddingProvider, Source: "runtime.embeddingProvider"})
	embeddingAPIKeyID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.EmbeddingAPIKey, Source: "runtime.embeddingApiKey"})
	embeddingEndpointID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.EmbeddingEndpoint, Source: "runtime.embeddingEndpoint"})
	sourceSearchPlannerProviderID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SourceSearchPlannerProvider, Source: "runtime.sourceSearchPlannerProvider"})
	sourceSearchPlannerAPIKeyID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SourceSearchPlannerAPIKey, Source: "runtime.sourceSearchPlannerApiKey"})
	sourceSearchPlannerEndpointID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SourceSearchPlannerEndpoint, Source: "runtime.sourceSearchPlannerEndpoint"})
	sourceSearchPlannerModelID := firstRuntimeSourceValue(runtimeSourceValue{Value: rt.SourceSearchPlannerModel, Source: "runtime.sourceSearchPlannerModel"})
	if !rt.Synced {
		embeddingProviderID = firstRuntimeSourceValue(
			embeddingProviderID,
			runtimeSourceValue{Value: s.Cfg.EmbedderProvider, Source: "config.AC_EMBEDDER_PROVIDER"},
			embeddingEnvSource("AC_EMBEDDER_PROVIDER", "AC_LT_EMBEDDING_PROVIDER", "PROJECT_EMBEDDING_PROVIDER", "AC_PROJECT_EMBEDDING_PROVIDER"),
		)
		embeddingAPIKeyID = firstRuntimeSourceValue(
			embeddingAPIKeyID,
			embeddingEnvSource("AC_EMBEDDER_API_KEY", "AC_LT_EMBEDDING_API_KEY", "PROJECT_EMBEDDING_API_KEY", "AC_PROJECT_EMBEDDING_API_KEY"),
		)
		embeddingEndpointID = firstRuntimeSourceValue(
			embeddingEndpointID,
			runtimeSourceValue{Value: s.Cfg.EmbedderEndpoint, Source: "config.AC_EMBEDDER_ENDPOINT"},
			embeddingEnvSource("AC_EMBEDDER_ENDPOINT", "AC_LT_EMBEDDING_ENDPOINT", "PROJECT_EMBEDDING_ENDPOINT", "AC_PROJECT_EMBEDDING_ENDPOINT"),
		)
	}
	embeddingIdentity := s.currentEmbeddingModelIdentity()
	embeddingModel := embeddingIdentity.Model
	embeddingModelID := runtimeSourceValue{Value: embeddingModel, Source: embeddingIdentity.Source}
	mainTrace := configuredTrace(mainProviderID.Value, mainAPIKeyID.Value, mainEndpointID.Value, mainModelID.Value, rt.MainTimeoutSec)
	addRuntimeSourceTrace(mainTrace, mainProviderID, mainAPIKeyID, mainEndpointID, mainModelID)
	addOptionalRuntimeTraceFields(mainTrace, rt.MainTemperature, rt.MainMaxTokens)
	addOptionalReasoningTraceFields(mainTrace, rt.MainReasoningPreset, rt.MainReasoningEffort, rt.MainReasoningBudget)
	if strings.TrimSpace(rt.MainLLMGatewayServiceTier) != "" {
		mainTrace["llm_gateway_service_tier"] = strings.TrimSpace(rt.MainLLMGatewayServiceTier)
	}
	if strings.TrimSpace(rt.MainClaudePromptCacheMode) != "" {
		mainTrace["claude_prompt_cache_mode"] = strings.TrimSpace(rt.MainClaudePromptCacheMode)
	}
	mainTrace["runtime_role"] = "publisher_editor_default"
	mainTrace["direct_generation"] = map[string]any{
		"status":  "risuai_host_retained",
		"enabled": false,
		"reason":  "SEQ-01 records Project Main direct generation as an original gap, and SEQ-02 keeps RisuAI main generation outside the immediate replacement scope.",
	}
	supervisorTrace := configuredTrace(
		supervisorProviderID.Value,
		supervisorAPIKeyID.Value,
		supervisorEndpointID.Value,
		supervisorModelID.Value,
		rt.SupervisorTimeoutSec,
	)
	addRuntimeSourceTrace(supervisorTrace, supervisorProviderID, supervisorAPIKeyID, supervisorEndpointID, supervisorModelID)
	addOptionalRuntimeTraceFields(supervisorTrace, rt.SupervisorTemperature, rt.SupervisorMaxTokens)
	addOptionalReasoningTraceFields(supervisorTrace, rt.SupervisorReasoningPreset, rt.SupervisorReasoningEffort, rt.SupervisorReasoningBudget)
	if strings.TrimSpace(rt.SupervisorLLMGatewayServiceTier) != "" {
		supervisorTrace["llm_gateway_service_tier"] = strings.TrimSpace(rt.SupervisorLLMGatewayServiceTier)
	}
	if strings.TrimSpace(rt.SupervisorClaudePromptCacheMode) != "" {
		supervisorTrace["claude_prompt_cache_mode"] = strings.TrimSpace(rt.SupervisorClaudePromptCacheMode)
	}
	criticTrace := configuredTrace(
		criticProviderID.Value,
		criticAPIKeyID.Value,
		criticEndpointID.Value,
		criticModelID.Value,
		rt.CriticTimeoutSec,
	)
	addRuntimeSourceTrace(criticTrace, criticProviderID, criticAPIKeyID, criticEndpointID, criticModelID)
	addOptionalRuntimeTraceFields(criticTrace, rt.CriticTemperature, rt.CriticMaxTokens)
	addOptionalReasoningTraceFields(criticTrace, rt.CriticReasoningPreset, rt.CriticReasoningEffort, rt.CriticReasoningBudget)
	if strings.TrimSpace(rt.CriticLLMGatewayServiceTier) != "" {
		criticTrace["llm_gateway_service_tier"] = strings.TrimSpace(rt.CriticLLMGatewayServiceTier)
	}
	if strings.TrimSpace(rt.CriticClaudePromptCacheMode) != "" {
		criticTrace["claude_prompt_cache_mode"] = strings.TrimSpace(rt.CriticClaudePromptCacheMode)
	}
	embeddingTrace := configuredTrace(
		embeddingProviderID.Value,
		embeddingAPIKeyID.Value,
		embeddingEndpointID.Value,
		embeddingModelID.Value,
		rt.EmbeddingTimeoutSec,
	)
	addRuntimeSourceTrace(embeddingTrace, embeddingProviderID, embeddingAPIKeyID, embeddingEndpointID, embeddingModelID)
	sourceSearchPlannerTrace := sourceSearchConfiguredTrace(
		sourceSearchPlannerProviderID.Value,
		sourceSearchPlannerAPIKeyID.Value,
		sourceSearchPlannerEndpointID.Value,
		sourceSearchPlannerModelID.Value,
		rt.SourceSearchPlannerTimeoutSec,
	)
	addRuntimeSourceTrace(sourceSearchPlannerTrace, sourceSearchPlannerProviderID, sourceSearchPlannerAPIKeyID, sourceSearchPlannerEndpointID, sourceSearchPlannerModelID)
	addOptionalRuntimeTraceFields(sourceSearchPlannerTrace, rt.SourceSearchPlannerTemperature, rt.SourceSearchPlannerMaxTokens)
	addOptionalReasoningTraceFields(sourceSearchPlannerTrace, rt.SourceSearchPlannerReasoningPreset, rt.SourceSearchPlannerReasoningEffort, rt.SourceSearchPlannerReasoningBudget)
	return map[string]any{
		"synced":            rt.Synced,
		"main":              mainTrace,
		"supervisor":        supervisorTrace,
		"critic":            criticTrace,
		"embedding":         embeddingTrace,
		"source_search_llm": sourceSearchPlannerTrace,
		"llm_retry_count":   rt.LLMRetryCount,
		"top_k":             rt.TopK,
	}
}

func firstFloatPtr(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstInt64Ptr(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func int64PtrValue(v *int64, fallback int64) int64 {
	if v == nil {
		return fallback
	}
	return *v
}
