package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

// This draft is a UI observation. Resolving it never updates RuntimeConfig,
// reads storage or contacts a provider. Credentials are not part of the contract.
type llmSettingsDraft struct {
	Purpose         string `json:"purpose,omitempty"`
	UsePublisher    bool   `json:"usePublisher,omitempty"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Endpoint        string `json:"endpoint"`
	Preset          string `json:"preset"`
	CurrentEffort   string `json:"currentEffort"`
	CurrentBudget   string `json:"currentBudget"`
	PreviousSyncKey string `json:"previousSyncKey"`
	IsFirstSync     bool   `json:"isFirstSync"`
}

type llmReasoningInput struct {
	Preset string  `json:"preset"`
	Effort string  `json:"effort"`
	Budget float64 `json:"budget"`
}

type llmReasoningControls struct {
	Family        string   `json:"family"`
	Mode          string   `json:"mode"`
	ShowEffort    bool     `json:"showEffort"`
	EffortOptions []string `json:"effortOptions"`
	EffortLabel   string   `json:"effortLabel"`
	EffortHint    string   `json:"effortHint"`
	ShowBudget    bool     `json:"showBudget"`
	BudgetLabel   string   `json:"budgetLabel"`
	BudgetHint    string   `json:"budgetHint"`
	GuideModeText string   `json:"guideModeText"`
}

type llmReasoningPreset struct {
	Label           string `json:"label"`
	Effort          string `json:"effort"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
	BudgetTokens    int    `json:"budgetTokens"`
	GlmThinkingType string `json:"glmThinkingType"`
	Hint            string `json:"hint"`
}

type llmSettingsView struct {
	TemperatureLocked bool                 `json:"temperatureLocked,omitempty"`
	ContractVersion   string               `json:"contract_version"`
	Family            string               `json:"family"`
	Controls          llmReasoningControls `json:"controls"`
	PresetInfo        llmReasoningPreset   `json:"presetInfo"`
	AllowedPresets    []string             `json:"allowedPresets"`
	SyncKey           string               `json:"syncKey"`
	GuideText         string               `json:"guideText"`
	NextEffort        string               `json:"nextEffort"`
	NextBudget        string               `json:"nextBudget"`
}

var llmSettingsProviders = []string{"openai", "claude", "gemini", "llmgateway", "vertex", "openrouter", "opencode", "opencode-go", "neuralwatt", "vercel", "copilot", "ollama", "custom"}
var llmSettingsPresets = []string{"auto", "gpt", "gemini", "claude", "glm", "custom"}

func llmSettingsEnum(value, fallback string, allowed []string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}

func resolveLLMSettingsControls(provider, preset, model, endpoint string) llmReasoningControls {
	provider = llmSettingsEnum(provider, "openai", llmSettingsProviders)
	preset = llmSettingsEnum(preset, "auto", llmSettingsPresets)
	model = strings.ToLower(strings.TrimSpace(model))
	// UI family recognition historically used model/provider/preset, not an
	// endpoint-inferred GLM family. Keep that display distinction explicit.
	family := proxyReasoningFamily(provider, preset, model, "")
	// The legacy transport also recognizes loose custom GLM aliases such as
	// "glmfoo". The UI did not infer a toggle from those aliases. Preserve that
	// distinction instead of silently adding a thinking field to host requests.
	if family == "glm" && !regexp.MustCompile(`(^|/)glm[-_]`).MatchString(model) {
		family = proxyReasoningFamily(provider, preset, "", "")
	}
	transport, err := proxyReasoningTransport(provider, endpoint)
	makeControls := func(key string, options []string) llmReasoningControls {
		c := llmSettingsControlTemplates[key]
		c.Family = family
		c.EffortOptions = append([]string{}, options...)
		return c
	}
	if err != nil {
		return makeControls("conflict", nil)
	}
	glmEffort := proxyGLMSupportsReasoningEffort(model)
	glmRequired := proxyGLMRequiresThinking(model)
	geminiMode, claudeMode := proxyGeminiThinkingMode(model), proxyClaudeThinkingMode(model)
	geminiOptions := []string{"none"}
	for _, value := range []string{"minimal", "low", "medium", "high"} {
		if proxyGeminiThinkingLevel(model, value) != "" {
			geminiOptions = append(geminiOptions, value)
		}
	}
	gptOptions := []string{}
	if regexp.MustCompile(`(^|/)o[134](?:$|[-_:])`).MatchString(model) {
		gptOptions = []string{"none", "low", "medium", "high"}
	} else {
		for _, value := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
			if proxyOpenAICompatibleReasoningEffort(model, value) == value {
				gptOptions = append(gptOptions, value)
			}
		}
	}
	deepseekOptions := []string{"none", "low", "high", "max"}
	if transport == "neuralwatt" && regexp.MustCompile(`deepseek[-_]?v4(?:$|[-_:]).*flash`).MatchString(model) {
		deepseekOptions = []string{"none", "high", "max"}
	}
	if transport == "ollama" && family != "none" {
		if family == "kimi_toggle" {
			return makeControls("ollama", []string{"enable", "disable"})
		}
		if family == "kimi_effort" {
			return makeControls("ollama", []string{"low", "high"})
		}
		if family == "kimi_always" {
			return makeControls("kimi_always", nil)
		}
		if family == "glm" {
			if glmRequired {
				return makeControls("ollama", []string{"low", "high"})
			}
			if glmEffort {
				return makeControls("ollama_glm_effort", []string{"none", "high"})
			}
			return makeControls("ollama_glm_toggle", []string{"enable", "disable"})
		}
		return makeControls("ollama", []string{"none", "low", "medium", "high"})
	}
	if (slices.Contains([]string{"llmgateway", "openrouter", "vercel", "neuralwatt"}, transport) && family != "none") || (slices.Contains([]string{"custom", "opencode", "opencode-go"}, transport) && family == "deepseek_v4") {
		var options []string
		switch family {
		case "deepseek_v4":
			options = deepseekOptions
		case "gpt":
			options = gptOptions
		case "glm":
			options = []string{"enable", "disable"}
			if glmEffort {
				options = []string{"none", "high", "max"}
			}
			if glmRequired {
				options = []string{"low", "high", "max"}
			}
		case "kimi_toggle":
			options = []string{"enable", "disable"}
		case "kimi_always":
			return makeControls("kimi_always", nil)
		case "kimi_effort":
			options = []string{"low", "high", "max"}
		case "gemini":
			if geminiMode != "none" {
				options = geminiOptions
			}
		case "claude":
			if claudeMode != "none" {
				options = []string{"none", "low", "medium", "high", "max"}
			}
		}
		if len(options) > 0 {
			c := makeControls("gateway", options)
			c.GuideModeText = "현재 전송 규약: " + transport + " reasoning"
			return c
		}
	}
	if family == "glm" {
		if glmRequired {
			return makeControls("glm_required", []string{"low", "high", "max"})
		}
		if glmEffort {
			return makeControls("glm_effort", []string{"none", "high", "max"})
		}
		return makeControls("glm_toggle", []string{"enable", "disable"})
	}
	if family == "kimi_toggle" {
		return makeControls("kimi_toggle", []string{"enable", "disable"})
	}
	if family == "kimi_always" {
		return makeControls("kimi_always", nil)
	}
	if family == "kimi_effort" {
		return makeControls("kimi_effort", []string{"low", "high", "max"})
	}
	if family == "deepseek_v4" && transport == "deepseek" {
		return makeControls("deepseek", deepseekOptions)
	}
	if family == "gemini" {
		if !slices.Contains([]string{"gemini", "vertex", "opencode"}, provider) || geminiMode == "none" {
			return makeControls("unsupported_gemini", nil)
		}
		if geminiMode == "level" {
			return makeControls("gemini_level", geminiOptions)
		}
		return makeControls("gemini_budget", nil)
	}
	if family == "claude" && slices.Contains([]string{"claude", "opencode", "opencode-go"}, provider) {
		if claudeMode == "adaptive" {
			return makeControls("claude_adaptive", []string{"none", "low", "medium", "high", "max"})
		}
		if claudeMode == "manual_budget" {
			return makeControls("claude_budget", nil)
		}
	}
	if family == "gpt" && len(gptOptions) > 0 {
		return makeControls("gpt", gptOptions)
	}
	return makeControls("unsupported", nil)
}

func normalizeLLMSettingsEffort(value string, c llmReasoningControls) string {
	if !c.ShowEffort || len(c.EffortOptions) == 0 {
		return "none"
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if c.Family == "kimi_effort" {
		value = proxyKimiEffort(value)
	}
	if c.Family == "kimi_toggle" {
		value = strings.TrimSuffix(proxyGLMThinkingTypeFromRequest("", value), "d")
	}
	if c.Family == "glm" {
		if slices.Contains(c.EffortOptions, "low") && !slices.Contains(c.EffortOptions, "none") {
			value = proxyGLMRequiredEffort(value)
		} else if c.Mode == "glm_toggle" || (slices.Contains(c.EffortOptions, "enable") && slices.Contains(c.EffortOptions, "disable")) {
			if slices.Contains([]string{"none", "minimal", "disable", "disabled", "off", "false"}, value) {
				value = "disable"
			} else {
				value = "enable"
			}
		} else {
			if slices.Contains([]string{"minimal", "disable", "disabled", "off", "false"}, value) {
				value = "none"
			}
			if slices.Contains([]string{"enable", "enabled", "on", "true", "low", "medium"}, value) {
				value = "high"
			}
			if value == "xhigh" {
				if slices.Contains(c.EffortOptions, "max") {
					value = "max"
				} else {
					value = "high"
				}
			}
			if value == "max" && !slices.Contains(c.EffortOptions, "max") && slices.Contains(c.EffortOptions, "high") {
				value = "high"
			}
		}
	}
	if c.Family == "deepseek_v4" {
		if value == "minimal" && c.Mode != "ollama_reasoning_effort" {
			value = "low"
		}
		if value == "low" && !slices.Contains(c.EffortOptions, "low") {
			value = "high"
		}
		if value == "medium" && c.Mode != "ollama_reasoning_effort" {
			value = "high"
		}
		if value == "xhigh" {
			value = "high"
		}
		if value == "ultra" {
			value = "max"
		}
	}
	if (c.Family == "kimi_effort" || c.Family == "glm" || c.Family == "deepseek_v4") && value == "max" && !slices.Contains(c.EffortOptions, "max") && slices.Contains(c.EffortOptions, "high") {
		value = "high"
	}
	return llmSettingsEnum(value, c.EffortOptions[0], c.EffortOptions)
}

func resolveLLMSettingsView(input llmSettingsDraft) llmSettingsView {
	provider := llmSettingsEnum(input.Provider, "openai", llmSettingsProviders)
	preset := llmSettingsEnum(input.Preset, "auto", llmSettingsPresets)
	c := resolveLLMSettingsControls(provider, preset, input.Model, input.Endpoint)
	info, ok := llmSettingsPresetGuides[c.Family]
	if !ok {
		info = llmSettingsPresetGuides["none"]
	}
	key := strings.Join([]string{provider, preset, strings.ToLower(strings.TrimSpace(input.Model)), c.Mode}, "|")
	applyDefaults := !input.IsFirstSync && input.PreviousSyncKey != "" && input.PreviousSyncKey != key && preset != "custom"
	effort, budget := strings.TrimSpace(input.CurrentEffort), strings.TrimSpace(input.CurrentBudget)
	supported := slices.Contains(c.EffortOptions, effort) || (c.Mode == "deepseek_v4_reasoning_effort" && slices.Contains([]string{"minimal", "medium", "xhigh", "ultra"}, strings.ToLower(effort)))
	if (c.Family == "glm" && proxyGLMRequiresThinking(input.Model) && proxyGLMRequiredEffort(effort) != "") || (c.Family == "kimi_effort" && proxyKimiEffort(effort) != "") {
		supported = true
	}
	value, err := strconv.ParseFloat(budget, 64)
	numeric := budget != "" && err == nil && !math.IsInf(value, 0) && !math.IsNaN(value)
	defaultEffort := info.Effort
	if c.Mode == "thinking_level" {
		defaultEffort = info.ThinkingLevel
		if defaultEffort == "" {
			defaultEffort = "high"
		}
	}
	nextEffort, nextBudget := "none", "0"
	if c.ShowEffort {
		nextEffort = normalizeLLMSettingsEffort(effort, c)
		if applyDefaults || (input.IsFirstSync && preset != "custom" && !supported) {
			nextEffort = normalizeLLMSettingsEffort(defaultEffort, c)
		}
	}
	if c.ShowBudget {
		nextBudget = budget
		if applyDefaults || (input.IsFirstSync && preset != "custom" && !numeric) {
			nextBudget = strconv.Itoa(info.BudgetTokens)
		}
	}
	if input.Purpose == "memory_preprocessing" {
		// Blank is an intentional per-role inheritance choice, not reasoning off.
		// Opening/changing the form never changes it to a model preset default.
		if effort == "" {
			nextEffort = ""
		} else if c.ShowEffort {
			nextEffort = normalizeLLMSettingsEffort(effort, c)
		}
		if c.ShowEffort {
			c.EffortOptions = append([]string{""}, c.EffortOptions...)
		}
		nextBudget = budget
	}
	prefix := "현재 프리셋"
	if preset == "auto" {
		prefix = "자동 감지 결과"
	}
	return llmSettingsView{ContractVersion: "llm_settings_view.v1", Family: c.Family, Controls: c, PresetInfo: info, AllowedPresets: append([]string{}, llmSettingsPresets...), SyncKey: key, GuideText: prefix + ": " + info.Label + " · " + info.Hint + " · " + c.GuideModeText, NextEffort: nextEffort, NextBudget: nextBudget, TemperatureLocked: strings.HasPrefix(c.Family, "kimi_") && provider != "ollama"}
}

func (s *Server) handleConfigViewModel(w http.ResponseWriter, r *http.Request) {
	var draft llmSettingsDraft
	if err := json.NewDecoder(r.Body).Decode(&draft); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if draft.Purpose == "memory_preprocessing" && draft.UsePublisher {
		llm := s.supervisorLLMConfig()
		draft.Provider, draft.Endpoint, draft.Model, draft.Preset = llm.Provider, llm.Endpoint, llm.Model, llm.ReasoningPreset
	}
	writeJSON(w, http.StatusOK, resolveLLMSettingsView(draft))
}

// Host reasoning_input and explicit preprocessing role overrides use this mapping.
// Publisher/Critic and reference callers retain their own configuration contracts.
func applyHostReasoningInput(req *dto.ProxyPluginMainRequest, input *llmReasoningInput) {
	if input == nil {
		return
	}
	c := resolveLLMSettingsControls(stringPtrValue(req.Provider, "openai"), input.Preset, stringPtrValue(req.Model, ""), stringPtrValue(req.Endpoint, ""))
	effort := normalizeLLMSettingsEffort(input.Effort, c)
	if p := strings.TrimSpace(input.Preset); p != "" && strings.ToLower(p) != "auto" {
		req.ReasoningPreset = &p
	}
	switch c.Mode {
	case "glm_toggle":
		if c.ShowEffort && effort != "" {
			value := "enabled"
			if effort == "disable" || effort == "disabled" {
				value = "disabled"
			}
			req.GlmThinkingType = &value
		}
	case "glm_reasoning_effort":
		value := "enabled"
		if effort == "none" {
			value = "disabled"
		} else {
			req.ReasoningEffort = &effort
		}
		req.GlmThinkingType = &value
	case "deepseek_v4_reasoning_effort", "kimi_reasoning_effort", "kimi_toggle":
		req.ReasoningEffort = &effort
	default:
		if c.ShowEffort && effort != "" && (effort != "none" || slices.Contains([]string{"reasoning_effort", "ollama_reasoning_effort", "gateway_reasoning_effort"}, c.Mode)) {
			req.ReasoningEffort = &effort
		}
	}
	if c.ShowBudget && input.Budget > 0 {
		budget := int64(math.Min(input.Budget, 131072))
		req.ReasoningBudgetTokens, req.BudgetTokens = &budget, &budget
	}
}
