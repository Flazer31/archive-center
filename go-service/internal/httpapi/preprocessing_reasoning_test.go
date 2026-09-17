package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

// Configuration persistence -> both real preprocessing calls -> actual HTTP
// request body. The sole substitute is the upstream provider's response.
func TestPreprocessingReasoningSavedBothRoundsWire(t *testing.T) {
	cases := []struct {
		name, provider, endpoint, model, effort string
		budget                                  *int64
		want                                    map[string]any
		noTemperature                           bool
	}{
		{"glm47-on", "custom", "https://api.z.ai/api/paas/v4", "glm-4.7", "enable", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}}, false},
		{"glm51-off", "custom", "https://api.z.ai/api/paas/v4", "glm-5.1", "disable", nil, map[string]any{"thinking": map[string]any{"type": "disabled"}}, false},
		{"glm52-max", "custom", "https://api.z.ai/api/paas/v4", "glm-5.2", "max", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "max"}, false},
		{"glm53-low", "custom", "https://api.z.ai/api/paas/v4", "glm-5.3", "low", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "low"}, false},
		{"glm53-old-none", "custom", "https://api.z.ai/api/paas/v4", "glm-5.3-flash", "none", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "low"}, false},
		{"glm53-gateway-max", "llmgateway", "", "glm-5.3", "max", nil, map[string]any{"reasoning_effort": "max"}, false},
		{"glm52-router-max", "openrouter", "", "z-ai/glm-5.2", "max", nil, map[string]any{"reasoning": map[string]any{"effort": "max"}}, false},
		{"glm53-router-low", "openrouter", "", "z-ai/glm-5.3-flash", "low", nil, map[string]any{"reasoning": map[string]any{"effort": "low"}}, false},
		{"kimi25-off", "custom", "https://api.moonshot.ai/v1", "kimi-k2.5", "disable", nil, map[string]any{"thinking": map[string]any{"type": "disabled"}}, true},
		{"kimi26-on", "custom", "https://api.moonshot.ai/v1", "kimi-k2.6", "enable", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}}, true},
		{"kimi26-router-off", "openrouter", "", "moonshotai/kimi-k2.6", "disable", nil, map[string]any{"reasoning": map[string]any{"effort": "none"}}, true},
		{"kimi26-gateway-on", "llmgateway", "", "kimi-k2.6", "enable", nil, map[string]any{"reasoning_effort": "high"}, true},
		{"kimi27-always", "opencode-go", "", "kimi-k2.7-code", "none", nil, map[string]any{}, true},
		{"kimi27-highspeed", "custom", "https://api.moonshot.ai/v1", "kimi-k2.7-code-highspeed", "high", nil, map[string]any{}, true},
		{"kimi3-low", "custom", "https://api.moonshot.ai/v1", "kimi-k3", "low", nil, map[string]any{"reasoning_effort": "low"}, true},
		{"kimi3-router-max", "openrouter", "", "moonshotai/kimi-k3", "max", nil, map[string]any{"reasoning": map[string]any{"effort": "max"}}, true},
		{"deepseek41-gateway", "llmgateway", "", "deepseek-v4.1-flash", "low", nil, map[string]any{"reasoning_effort": "low"}, false},
		{"deepseek-flash-direct", "custom", "https://api.deepseek.com", "deepseek-flash", "max", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "max"}, false},
		{"deepseek-v4-off", "custom", "https://api.deepseek.com", "deepseek-v4-pro", "none", nil, map[string]any{"thinking": map[string]any{"type": "disabled"}}, false},
		{"deepseek-default", "custom", "https://api.deepseek.com", "deepseek-flash", "", nil, map[string]any{}, false},
		{"deepseek-ollama-default", "ollama", "", "deepseek-v4.1-flash:cloud", "", nil, map[string]any{}, false},
		{"deepseek-ollama-off", "ollama", "", "deepseek-v4.1-flash:cloud", "none", nil, map[string]any{"reasoning_effort": "none"}, false},
		{"deepseek-minimal", "custom", "https://api.deepseek.com", "deepseek-flash", "minimal", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "low"}, false},
		{"deepseek-xhigh", "custom", "https://api.deepseek.com", "deepseek-flash", "xhigh", nil, map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "high"}, false},
		{"deepseek-ultra", "llmgateway", "", "deepseek-v4.1-flash", "ultra", nil, map[string]any{"reasoning_effort": "max"}, false},
		{"gemini38-medium", "gemini", "", "gemini-3.8-flash", "medium", nil, map[string]any{"thinkingConfig": map[string]any{"includeThoughts": false, "thinkingLevel": "medium"}}, false},
		{"gemini25-budget", "gemini", "", "gemini-2.5-flash", "", int64Ptr(1024), map[string]any{"thinkingConfig": map[string]any{"includeThoughts": false, "thinkingBudget": float64(1024)}}, false},
		{"claude45-budget", "claude", "", "claude-sonnet-4-5", "", int64Ptr(1024), map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": float64(1024)}}, true},
		{"claude46-high", "claude", "", "claude-sonnet-4-6", "high", nil, map[string]any{"thinking": map[string]any{"type": "adaptive"}, "output_config": map[string]any{"effort": "high"}}, true},
		{"gpt52-high", "openai", "", "gpt-5.2", "high", nil, map[string]any{"reasoning_effort": "high"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			s := &Server{Cfg: config.Default()}
			cfg := defaultMultiAgentSettings()
			role := cfg.Roles["event_recent"]
			role.Provider, role.Endpoint, role.Model, role.APIKey = tc.provider, tc.endpoint, tc.model, "synthetic-key"
			role.ReasoningEffort, role.ReasoningBudgetTokens, role.MaxTokens = tc.effort, tc.budget, 4096
			cfg.Roles["event_recent"] = role
			b, _ := json.Marshal(cfg)
			rec := httptest.NewRecorder()
			s.handleMultiAgentSettings(rec, httptest.NewRequest("PUT", "/config/memory-preprocessing", bytes.NewReader(b)))
			if rec.Code != 200 {
				t.Fatalf("save: %s", rec.Body)
			}
			loaded, err := s.loadMultiAgentSettings()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(loaded.Roles["event_recent"], role) {
				t.Fatal("role settings did not round trip")
			}
			old := proxyHTTPClient
			defer func() { proxyHTTPClient = old }()
			calls := 0
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				got := map[string]any{}
				for _, key := range []string{"thinking", "reasoning_effort", "reasoning", "output_config"} {
					if v, ok := body[key]; ok {
						got[key] = v
					}
				}
				if v, ok := mapFromAny(body["generationConfig"])["thinkingConfig"]; ok {
					got["thinkingConfig"] = v
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("round %d wire=%#v want=%#v", calls, got, tc.want)
				}
				if tc.noTemperature {
					if _, ok := body["temperature"]; ok {
						t.Error("fixed/incompatible temperature transmitted")
					}
				}
				// JSON final content only; reasoning text is not used as the selection.
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"selected_ids\":[]}","reasoning_content":"not JSON"}}],"content":[{"type":"text","text":"{\"selected_ids\":[]}"}],"candidates":[{"content":{"parts":[{"text":"{\"selected_ids\":[]}"}]}}]}`))}, nil
			})}
			for round := 1; round <= 2; round++ {
				call := s.callMultiAgent(context.Background(), "event_recent", loaded, round, map[string]any{}, "fixture-session")
				if call.Error != "" {
					t.Fatalf("round %d: %s", round, call.Error)
				}
			}
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestPreprocessingReasoningPublisherInheritanceAndOverrides(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "inherit", true: "override"}[override], func(t *testing.T) {
			s := &Server{Cfg: config.Default(), RuntimeConfig: RuntimeConfig{SupervisorProvider: "claude", SupervisorModel: "claude-sonnet-4-5", SupervisorAPIKey: "synthetic", SupervisorReasoningBudget: int64Ptr(1024)}}
			cfg := defaultMultiAgentSettings()
			role := cfg.Roles["world_state"]
			role.UsePublisher = true
			role.MaxTokens = 4096
			want := float64(1024)
			if override {
				role.ReasoningBudgetTokens = int64Ptr(2048)
				want = 2048
			}
			cfg.Roles["world_state"] = role
			old := proxyHTTPClient
			defer func() { proxyHTTPClient = old }()
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if mapFromAny(body["thinking"])["budget_tokens"] != want {
					t.Errorf("inherited/override budget missing: %v", body["thinking"])
				}
				if body["max_tokens"] != float64(4096) {
					t.Error("role output budget replaced by Publisher")
				}
				if !strings.Contains(extractionStringFromAny(body["system"]), multiAgentRolePrompts["world_state"]) {
					t.Error("role prompt replaced")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"{\"selected_ids\":[]}"}]}`))}, nil
			})}
			call := s.callMultiAgent(context.Background(), "world_state", cfg, 1, map[string]any{})
			if call.Error != "" {
				t.Fatal(call.Error)
			}
		})
	}
}

func TestPreprocessingReasoningViewModelVersions(t *testing.T) {
	for _, model := range []string{"glm-5.3", "kimi-k3"} {
		v := resolveLLMSettingsView(llmSettingsDraft{Purpose: "memory_preprocessing", Provider: "ollama", Model: model, CurrentEffort: "max", IsFirstSync: true})
		if v.NextEffort != "high" {
			t.Fatalf("Ollama %s legacy max should match wire high: %+v", model, v)
		}
	}
	for _, tc := range []struct {
		provider, model string
		want            []string
	}{
		{"custom", "glm-4.7", []string{"", "enable", "disable"}},
		{"llmgateway", "glm-5.2", []string{"", "none", "high", "max"}},
		{"llmgateway", "glm-5.3", []string{"", "low", "high", "max"}},
		{"openrouter", "z-ai/glm-5.3-flash", []string{"", "low", "high", "max"}},
		{"custom", "kimi-k2.6", []string{"", "enable", "disable"}},
		{"opencode-go", "kimi-k2.7-code", []string{}},
		{"custom", "kimi-k3", []string{"", "low", "high", "max"}},
		{"llmgateway", "deepseek-v4.1-flash", []string{"", "none", "low", "high", "max"}},
	} {
		t.Run(tc.model, func(t *testing.T) {
			v := resolveLLMSettingsView(llmSettingsDraft{Purpose: "memory_preprocessing", Provider: tc.provider, Model: tc.model, IsFirstSync: true})
			if !reflect.DeepEqual(v.Controls.EffortOptions, tc.want) {
				t.Fatalf("options=%v want=%v", v.Controls.EffortOptions, tc.want)
			}
			if v.NextEffort != "" || v.NextBudget != "" {
				t.Fatal("opening form overwrote intentional inheritance")
			}
		})
	}
	// Shared connection is resolved from the actual backend config, not stale
	// independent fields or credentials copied into a JS preview request.
	s := &Server{Cfg: config.Default(), RuntimeConfig: RuntimeConfig{SupervisorProvider: "llmgateway", SupervisorModel: "glm-5.3"}}
	r := httptest.NewRecorder()
	s.handleConfigViewModel(r, httptest.NewRequest("POST", "/config/view-model", strings.NewReader(`{"purpose":"memory_preprocessing","usePublisher":true,"model":"gpt-4o","currentEffort":"none"}`)))
	var view llmSettingsView
	_ = json.Unmarshal(r.Body.Bytes(), &view)
	if view.NextEffort != "low" || view.Family != "glm" {
		t.Fatalf("shared provider preview=%s", r.Body)
	}
}
