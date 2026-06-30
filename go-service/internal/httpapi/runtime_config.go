package httpapi

import (
	"os"
	"strings"
)

// RuntimeConfig mirrors the 0.8 /config/update runtime settings that the JS
// bridge sends after the user saves the settings panel.
type RuntimeConfig struct {
	Synced                    bool
	MainProvider              string
	MainAPIKey                string
	MainEndpoint              string
	MainModel                 string
	MainTimeoutSec            int64
	MainTemperature           *float64
	MainMaxTokens             *int64
	MainReasoningPreset       string
	MainReasoningEffort       string
	MainReasoningBudget       *int64
	CriticProvider            string
	CriticAPIKey              string
	CriticEndpoint            string
	CriticModel               string
	CriticTimeoutSec          int64
	CriticTemperature         *float64
	CriticMaxTokens           *int64
	CriticReasoningPreset     string
	CriticReasoningEffort     string
	CriticReasoningBudget     *int64
	SupervisorProvider        string
	SupervisorAPIKey          string
	SupervisorEndpoint        string
	SupervisorModel           string
	SupervisorTimeoutSec      int64
	SupervisorTemperature     *float64
	SupervisorMaxTokens       *int64
	SupervisorReasoningPreset string
	SupervisorReasoningEffort string
	SupervisorReasoningBudget *int64
	EmbeddingProvider         string
	EmbeddingAPIKey           string
	EmbeddingEndpoint         string
	EmbeddingModel            string
	EmbeddingTimeoutSec       int64
	TopK                      int64
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
	defer s.RuntimeConfigMu.Unlock()
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
	setString("embeddingProvider", &s.RuntimeConfig.EmbeddingProvider)
	setString("embeddingApiKey", &s.RuntimeConfig.EmbeddingAPIKey)
	setString("embeddingEndpoint", &s.RuntimeConfig.EmbeddingEndpoint)
	setString("embeddingModel", &s.RuntimeConfig.EmbeddingModel)
	setInt("embeddingTimeout", &s.RuntimeConfig.EmbeddingTimeoutSec)
	setInt("topK", &s.RuntimeConfig.TopK)

	return updated
}

func (s *Server) runtimeConfigSnapshot() RuntimeConfig {
	s.RuntimeConfigMu.RLock()
	defer s.RuntimeConfigMu.RUnlock()
	return s.RuntimeConfig
}

func runtimeTimeoutMs(seconds int64, fallbackMs int64) int64 {
	if seconds <= 0 {
		return fallbackMs
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
		TimeoutMs:             runtimeTimeoutMs(rt.SupervisorTimeoutSec, 60000),
		Temperature:           temperature,
		MaxTokens:             maxTokens,
		ReasoningPreset:       rt.SupervisorReasoningPreset,
		ReasoningEffort:       rt.SupervisorReasoningEffort,
		ReasoningBudgetTokens: int64PtrValue(rt.SupervisorReasoningBudget, 0),
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
		TimeoutMs:             runtimeTimeoutMs(rt.MainTimeoutSec, 60000),
		Temperature:           temperature,
		MaxTokens:             maxTokens,
		ReasoningPreset:       rt.MainReasoningPreset,
		ReasoningEffort:       rt.MainReasoningEffort,
		ReasoningBudgetTokens: int64PtrValue(rt.MainReasoningBudget, 0),
	}
}

func configMissingFields(apiKey, endpoint, model string) []string {
	missing := []string{}
	if strings.TrimSpace(apiKey) == "" {
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

func configMissingFieldsWithProvider(provider, apiKey, endpoint, model string) []string {
	missing := []string{}
	if strings.TrimSpace(provider) == "" {
		missing = append(missing, "provider")
	}
	missing = append(missing, configMissingFields(apiKey, endpoint, model)...)
	return missing
}

func configuredTrace(provider, apiKey, endpoint, model string, timeoutSec int64) map[string]any {
	return map[string]any{
		"configured":     len(configMissingFieldsWithProvider(provider, apiKey, endpoint, model)) == 0,
		"provider":       strings.TrimSpace(provider),
		"endpoint_host":  endpointHost(endpoint),
		"model":          strings.TrimSpace(model),
		"timeout_sec":    timeoutSec,
		"missing_fields": configMissingFieldsWithProvider(provider, apiKey, endpoint, model),
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
	embeddingTrace := configuredTrace(
		embeddingProviderID.Value,
		embeddingAPIKeyID.Value,
		embeddingEndpointID.Value,
		embeddingModelID.Value,
		rt.EmbeddingTimeoutSec,
	)
	addRuntimeSourceTrace(embeddingTrace, embeddingProviderID, embeddingAPIKeyID, embeddingEndpointID, embeddingModelID)
	return map[string]any{
		"main":       mainTrace,
		"supervisor": supervisorTrace,
		"critic":     criticTrace,
		"embedding":  embeddingTrace,
		"top_k":      rt.TopK,
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
