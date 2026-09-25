package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test44OptionalReasoningMappingPreservesPresenceAndIsolation(t *testing.T) {
	applyProxyReasoningFromLLMConfig(nil, completeTurnLLMConfig{})
	for _, cfg := range []completeTurnLLMConfig{{}, {ReasoningEffort: " \t", ReasoningPreset: " ", ReasoningBudgetTokens: -1, GlmThinkingType: "\n"}} {
		req := dto.ProxyPluginMainRequest{ReasoningEffort: strPtr("existing"), ReasoningPreset: strPtr("existing"), ReasoningBudgetTokens: int64Ptr(91), BudgetTokens: int64Ptr(92), GlmThinkingType: strPtr("existing")}
		before := req
		applyProxyReasoningFromLLMConfig(&req, cfg)
		if !reflect.DeepEqual(before, req) {
			t.Fatal("unset or whitespace fields overwrote a request value")
		}
	}
	cfg := completeTurnLLMConfig{ReasoningEffort: " none ", ReasoningPreset: " glm ", ReasoningBudgetTokens: 777, GlmThinkingType: " disabled "}
	first, second := dto.ProxyPluginMainRequest{}, dto.ProxyPluginMainRequest{}
	applyProxyReasoningFromLLMConfig(&first, cfg)
	cfg.ReasoningEffort, cfg.ReasoningBudgetTokens = "high", 888
	applyProxyReasoningFromLLMConfig(&second, cfg)
	if *first.ReasoningEffort != " none " || *first.ReasoningPreset != " glm " || *first.GlmThinkingType != " disabled " || *first.ReasoningBudgetTokens != 777 || *first.BudgetTokens != 777 || *second.ReasoningEffort != "high" || *second.BudgetTokens != 888 {
		t.Fatal("optional mapping changed whitespace, aliases, or another request's values")
	}
	otherCaller := dto.ProxyPluginMainRequest{}
	applyProxyOverridesFromLLMConfig(&otherCaller, cfg)
	if !reflect.DeepEqual(otherCaller, dto.ProxyPluginMainRequest{}) {
		t.Fatal("the shared override helper acquired role-specific reasoning fields")
	}
}

// Exercise the role owners, not a hand-built proxy DTO. The external provider
// rejects after recording the wire request so output parsing is outside this test.
func Test44RoleOwnersPreserveProxyMapping(t *testing.T) {
	for _, role := range []string{"publisher", "critic", "world_audit", "editor_shared_1", "editor_shared_2"} {
		for _, configured := range []bool{false, true} {
			name := role + "/blank"
			if configured {
				name = role + "/configured"
			}
			t.Run(name, func(t *testing.T) {
				const retryCount = 2
				cfg := completeTurnLLMConfig{
					Provider: "ollama", Endpoint: "https://role-fixture.invalid/v1", APIKey: "fixture-key",
					Model: "deepseek-v4-flash:0731-cloud", MaxTokens: 13, MaxCompletionTokens: 4096,
					Temperature: 0, TimeoutMs: 12345, RetryBudget: newLLMRetryBudget(retryCount),
					ExtraHeadersJSON: `{"X-Role-Fixture":"` + role + `"}`,
					ExtraBodyJSON:    `{"user":"` + role + `"}`,
				}
				if configured {
					cfg.ReasoningEffort, cfg.ReasoningPreset, cfg.ReasoningBudgetTokens = "high", "gpt", 7777
				}
				var calls int
				var wire map[string]any
				oldClient := proxyHTTPClient
				proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if r.Header.Get("X-Role-Fixture") != role || r.Header.Get("Authorization") != "Bearer fixture-key" {
						t.Fatalf("role connection/overrides changed: %v", r.Header)
					}
					if body["user"] != role || body["model"] != cfg.Model || body["temperature"] != float64(0) {
						t.Fatalf("role generation settings changed: %v", body)
					}
					wantTokens := cfg.MaxCompletionTokens
					if configured && !strings.HasPrefix(role, "editor_") {
						wantTokens += cfg.ReasoningBudgetTokens
					}
					if body["max_tokens"] != float64(wantTokens) {
						t.Errorf("max_tokens=%v want=%d; editor sharing must not add Publisher's budget", body["max_tokens"], wantTokens)
					}
					if configured && body["reasoning_effort"] != "high" {
						t.Errorf("reasoning effort lost: %v", body)
					}
					// Unset effort preserves Ollama's default; output ceilings and
					// the separate role overrides above remain unchanged.
					if _, present := body["reasoning_effort"]; !configured && present {
						t.Errorf("unset reasoning was transmitted: %v", body["reasoning_effort"])
					}
					wire = map[string]any{"url": r.URL.String(), "method": r.Method, "headers": r.Header, "body": body}
					return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"fixture rate limit"}}`))}, nil
				})}
				t.Cleanup(func() { proxyHTTPClient = oldClient })
				s := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
				switch role {
				case "publisher":
					_, _, err := s.runSupervisorLLM(context.Background(), "fixture-session", map[string]any{}, cfg)
					if err == nil {
						t.Error("provider error lost")
					}
				case "critic":
					_, _, err := s.runCompleteTurnCritic(context.Background(), "fixture-session", 1, "Mina found the key.", "Mina kept the key.", nil, nil, cfg)
					if err == nil {
						t.Error("provider error lost")
					}
				case "world_audit":
					_, trace := s.runCompleteTurnWorldRuleAudit(context.Background(), "fixture-session", 1, "Mina found the key.", "Mina kept the key.", nil, nil, cfg, []map[string]any{})
					if trace["status"] == "ok" {
						t.Error("provider error reported as success")
					}
				default:
					s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint = cfg.Provider, cfg.Endpoint
					s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = cfg.Model, cfg.APIKey
					s.RuntimeConfig.SupervisorReasoningPreset, s.RuntimeConfig.SupervisorReasoningEffort = cfg.ReasoningPreset, cfg.ReasoningEffort
					s.RuntimeConfig.SupervisorReasoningBudget = &cfg.ReasoningBudgetTokens
					s.RuntimeConfig.SupervisorExtraHeadersJSON, s.RuntimeConfig.SupervisorExtraBodyJSON = cfg.ExtraHeadersJSON, cfg.ExtraBodyJSON
					settings := defaultMultiAgentSettings()
					settings.Enabled = true
					value := settings.Roles["event_recent"]
					value.UsePublisher, value.Temperature, value.TimeoutMs, value.MaxTokens = true, 0, cfg.TimeoutMs, cfg.MaxCompletionTokens
					value.ReasoningEffort = cfg.ReasoningEffort
					settings.Roles["event_recent"] = value
					round := 1
					if role == "editor_shared_2" {
						round = 2
					}
					call := s.callMultiAgent(context.Background(), "event_recent", settings, round, map[string]any{"role": "event_recent"})
					if call.Error == "" {
						t.Error("provider error lost")
					}
				}
				wantCalls := 1
				if role == "publisher" {
					wantCalls += retryCount
				}
				if calls != wantCalls {
					t.Fatalf("provider calls=%d want=%d", calls, wantCalls)
				}
				if dir := os.Getenv("ARCHIVE_CENTER_TEST_PROXY_CAPTURE_DIR"); dir != "" {
					data, err := json.MarshalIndent(wire, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, strings.ReplaceAll(name, "/", "-")+".json"), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
