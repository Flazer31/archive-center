package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// The frozen pre-RF03 capture remains intact. Only the model changes approved
// on 2026-09-11 are applied to its expected values below, from official docs.
// All other fields/rows still have to equal the original baseline exactly.
func applySeptemberReasoningExpectations(input llmSettingsDraft, view, payload map[string]any) {
	// The previous UI displayed high for saved Ollama max but sent none.
	// Keep the displayed high and use the existing Ollama wire normalization.
	if input.Provider == "ollama" && input.Model == "deepseek-v4-pro:0813-cloud" && input.CurrentEffort == "max" {
		payload["reasoning_effort"] = "high"
	}
	if input.Provider == "custom" && input.Model == "glm-5.3" {
		efforts := map[string]string{"none": "low", "low": "low", "medium": "high", "high": "high", "max": "max", "enable": "high", "disable": "low"}
		if view != nil {
			controls := view["controls"].(map[string]any)
			guide := "GLM 5.3 · thinking enabled + reasoning_effort"
			view["guideText"] = strings.ReplaceAll(view["guideText"].(string), controls["guideModeText"].(string), guide)
			view["nextEffort"] = efforts[input.CurrentEffort]
			controls["effortOptions"] = []any{"low", "high", "max"}
			controls["effortLabel"] = "추론 강도"
			controls["effortHint"] = "GLM 5.3 · 항상 추론하며 low / high / max를 지원합니다."
			controls["budgetLabel"], controls["guideModeText"] = "", guide
		}
		payload["glm_thinking_type"], payload["reasoning_effort"] = "enabled", efforts[input.CurrentEffort]
	}
	if input.Model == "z-ai/glm-5.2" && (input.Provider == "openrouter" || input.Provider == "llmgateway" || input.Provider == "vercel" || input.Provider == "neuralwatt") {
		if view != nil {
			view["controls"].(map[string]any)["effortOptions"] = []any{"none", "high", "max"}
		}
		if input.CurrentEffort == "max" {
			if view != nil {
				view["nextEffort"] = "max"
			}
			payload["reasoning_effort"] = "max"
		}
	}
}

func Test44SettingsViewAndRequestPreservePreMigrationMatrix(t *testing.T) {
	data, err := os.ReadFile("testdata/llm-settings-44-before.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input   llmSettingsDraft
		View    map[string]any
		Payload map[string]any
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%03d_%s_%s_%s", i, c.Input.Provider, c.Input.Model, c.Input.CurrentEffort), func(t *testing.T) {
			applySeptemberReasoningExpectations(c.Input, c.View, c.Payload)
			view := resolveLLMSettingsView(c.Input)
			b, _ := json.Marshal(view)
			var actual map[string]any
			if err := json.Unmarshal(b, &actual); err != nil {
				t.Fatal(err)
			}
			delete(actual, "contract_version")
			delete(actual, "allowedPresets")
			if !reflect.DeepEqual(actual, c.View) {
				for key, expected := range c.View {
					if !reflect.DeepEqual(actual[key], expected) {
						t.Errorf("view %s: got %#v; want %#v", key, actual[key], expected)
					}
				}
			}
			req := dto.ProxyPluginMainRequest{Provider: &c.Input.Provider, Model: &c.Input.Model, Endpoint: &c.Input.Endpoint}
			budget, _ := strconv.ParseFloat(c.Input.CurrentBudget, 64)
			applyHostReasoningInput(&req, &llmReasoningInput{Preset: c.Input.Preset, Effort: c.Input.CurrentEffort, Budget: budget})
			b, _ = json.Marshal(req)
			if err := json.Unmarshal(b, &actual); err != nil {
				t.Fatal(err)
			}
			fields := map[string]any{}
			for _, key := range []string{"reasoning_preset", "reasoning_effort", "glm_thinking_type", "reasoning_budget_tokens", "budget_tokens"} {
				if value, ok := actual[key]; ok {
					fields[key] = value
				}
			}
			if !reflect.DeepEqual(fields, c.Payload) {
				t.Errorf("request got %#v; want %#v", fields, c.Payload)
			}
		})
	}
}

func Test44DraftSettingsViewDoesNotWriteOrCallProvider(t *testing.T) {
	s := &Server{Cfg: config.Default()} // No store: a view lookup has no persistence dependency.
	s.updateRuntimeConfig(map[string]any{"supervisorModel": "saved-publisher", "criticModel": "saved-critic"})
	before := s.runtimeConfigSnapshot()
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("view lookup called a provider")
		return nil, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()
	mux := http.NewServeMux()
	s.registerConfigRoutes(mux)
	for _, provider := range []string{"gemini", "vertex", "opencode", "llmgateway", "openrouter"} {
		body, _ := json.Marshal(llmSettingsDraft{Provider: provider, Model: "gemini-3.8-flash", CurrentEffort: "medium", CurrentBudget: "0", IsFirstSync: true})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/config/view-model", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("view status %d: %s", rec.Code, rec.Body)
		}
		var view llmSettingsView
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		if view.ContractVersion != "llm_settings_view.v1" || view.NextEffort != "medium" || !reflect.DeepEqual(view.Controls.EffortOptions, []string{"none", "low", "medium", "high"}) {
			t.Fatalf("Gemini 3.8 medium changed: %+v", view)
		}
		if !reflect.DeepEqual(before, s.runtimeConfigSnapshot()) {
			t.Fatal("draft changed saved role settings")
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/config/view-model", strings.NewReader("{")))
	if rec.Code != 400 || !reflect.DeepEqual(before, s.runtimeConfigSnapshot()) {
		t.Fatal("malformed view request changed settings or error contract")
	}
}

// Compare the registered HTTP owner all the way to the provider request. The
// legacy fields come from the frozen pre-migration JS, not the new resolver.
func Test44HostReasoningAndLegacyRequestsHaveIdenticalWire(t *testing.T) {
	vertexCredential := testVertexServiceAccountJSON(t)
	data, err := os.ReadFile("testdata/llm-settings-44-before.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input   llmSettingsDraft
		Payload map[string]any
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%03d", i), func(t *testing.T) {
			applySeptemberReasoningExpectations(c.Input, nil, c.Payload)
			s := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
			mux := http.NewServeMux()
			s.registerProxyRoutes(mux)
			var wire []map[string]any
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == "https://oauth2.googleapis.com/token" {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-token","expires_in":3600}`))}, nil
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				wire = append(wire, map[string]any{"url": r.URL.String(), "headers": r.Header, "body": body})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}],"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"OK"}]}}],"content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn"}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()
			endpoint := c.Input.Endpoint
			if c.Input.Provider == "custom" && endpoint == "" {
				endpoint = "https://fixture.invalid/v1"
			}
			if c.Input.Provider == "vertex" {
				endpoint = "https://aiplatform.googleapis.com/v1/projects/fixture-project/locations/global/publishers/google/models"
			}
			var outputs []string
			for _, legacy := range []bool{true, false} {
				body := map[string]any{"provider": c.Input.Provider, "model": c.Input.Model, "endpoint": endpoint, "api_key": "fixture-key", "temperature": 0.2, "max_tokens": 4096, "max_completion_tokens": 4096, "messages": []map[string]any{{"role": "user", "content": "Reply OK"}}, "extra_headers": map[string]any{"X-Fixture": "independent-role"}}
				if c.Input.Provider == "vertex" {
					body["api_key"] = vertexCredential
				}
				if legacy {
					for k, v := range c.Payload {
						body[k] = v
					}
				} else {
					budget, _ := strconv.ParseFloat(c.Input.CurrentBudget, 64)
					body["reasoning_input"] = llmReasoningInput{Preset: c.Input.Preset, Effort: c.Input.CurrentEffort, Budget: budget}
				}
				b, _ := json.Marshal(body)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest("POST", "/proxy/plugin-main", bytes.NewReader(b)))
				outputs = append(outputs, fmt.Sprintf("%d:%s", rec.Code, rec.Body))
			}
			if len(wire) == 0 { // Explicit pre-existing endpoint conflicts fail identically.
				if outputs[0] != outputs[1] {
					t.Fatalf("validation changed: %v", outputs)
				}
				if c.Input.Provider != "ollama" || !strings.Contains(endpoint, "deepseek.com") {
					t.Fatalf("expected provider calls: %v", outputs)
				}
				return
			}
			if len(wire) != 2 || !reflect.DeepEqual(wire[0], wire[1]) {
				t.Fatalf("upstream changed: %#v", wire)
			}
			if !strings.HasPrefix(outputs[0], "200:") || !strings.HasPrefix(outputs[1], "200:") {
				t.Fatalf("upstream success lost: %v", outputs)
			}
		})
	}
}

func Test44HostReasoningPreservesCriticConnectionTestCap(t *testing.T) {
	s := &Server{Cfg: config.Default(), Store: store.NewNoopStore()}
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["max_tokens"] != float64(1024) {
			t.Fatalf("critic test cap changed: %v", body)
		}
		if _, ok := body["thinking"]; ok {
			t.Fatalf("critic connection test received thinking budget: %v", body)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn"}`))}, nil
	})}
	rec := httptest.NewRecorder()
	s.handleProxyPluginMain(rec, httptest.NewRequest("POST", "/proxy/plugin-main?connection_test=critic", strings.NewReader(`{"provider":"claude","model":"claude-sonnet-4-5","api_key":"fixture","max_tokens":16000,"reasoning_input":{"preset":"auto","effort":"high","budget":8192},"messages":[{"role":"user","content":"OK"}]}`)))
	if rec.Code != 200 || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body)
	}
}
