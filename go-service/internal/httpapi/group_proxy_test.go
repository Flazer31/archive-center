package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func strPtr(v string) *string {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func testVertexServiceAccountJSON(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal RSA key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw})
	cred, err := json.Marshal(map[string]string{
		"client_email": "archive-center-test@example.iam.gserviceaccount.com",
		"private_key":  string(pemBytes),
		"project_id":   "proj",
	})
	if err != nil {
		t.Fatalf("marshal test vertex credential: %v", err)
	}
	return string(cred)
}

func TestProxyVertexEndpointErrorDetailExplainsGoogleHTML404(t *testing.T) {
	target := "https://us-central1-aiplatform.googleapis.com/v1/gemini-2.5-flash:generateContent"
	raw := `<!DOCTYPE html><html><title>Error 404 (Not Found)!!1</title></html>`
	detail := proxyVertexEndpointErrorDetail(http.StatusNotFound, target, nil, raw)
	if !strings.Contains(detail, "/publishers/google/models") || !strings.Contains(detail, "Current target") {
		t.Fatalf("Vertex endpoint hint missing expected guidance: %s", detail)
	}
}

func TestProxyNormalizeVertexEndpointRepairsCommonMultiRegionHosts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "us multi-region",
			in:   "https://us-aiplatform.googleapis.com/v1/projects/p/locations/us/publishers/google/models",
			want: "https://aiplatform.us.rep.googleapis.com/v1/projects/p/locations/us/publishers/google/models/gemini-3.5-flash:generateContent",
		},
		{
			name: "eu multi-region",
			in:   "https://eu-aiplatform.googleapis.com/v1/projects/p/locations/eu/publishers/google/models",
			want: "https://aiplatform.eu.rep.googleapis.com/v1/projects/p/locations/eu/publishers/google/models/gemini-3.5-flash:generateContent",
		},
		{
			name: "global",
			in:   "https://global-aiplatform.googleapis.com/v1/projects/p/locations/global/publishers/google/models",
			want: "https://aiplatform.googleapis.com/v1/projects/p/locations/global/publishers/google/models/gemini-3.5-flash:generateContent",
		},
		{
			name: "standard regional unchanged",
			in:   "https://us-central1-aiplatform.googleapis.com/v1/projects/p/locations/us-central1/publishers/google/models",
			want: "https://us-central1-aiplatform.googleapis.com/v1/projects/p/locations/us-central1/publishers/google/models/gemini-3.5-flash:generateContent",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := proxyNormalizeVertexEndpoint(tc.in, "gemini-3.5-flash"); got != tc.want {
				t.Fatalf("proxyNormalizeVertexEndpoint() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProxyNormalizeVertexEmbeddingEndpointRepairsCommonMultiRegionHosts(t *testing.T) {
	got := proxyNormalizeVertexEmbeddingEndpoint(
		"https://us-aiplatform.googleapis.com/v1/projects/p/locations/us/publishers/google/models",
		"gemini-embedding-001",
	)
	want := "https://aiplatform.us.rep.googleapis.com/v1/projects/p/locations/us/publishers/google/models/gemini-embedding-001:embedContent"
	if got != want {
		t.Fatalf("proxyNormalizeVertexEmbeddingEndpoint() = %q, want %q", got, want)
	}
}

func TestProxyResolveVertexProjectIDRejectsMissingProjectID(t *testing.T) {
	_, err := proxyResolveVertexProjectID(
		"https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models/gemini-3.5-flash:generateContent",
		`{"client_email":"x","private_key":"y"}`,
	)
	if err == nil || !strings.Contains(err.Error(), "missing project_id") {
		t.Fatalf("expected missing project_id error, got %v", err)
	}
}

func TestProxyEmptyContentPreservesProviderAndActual2xxStatus(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()

	tests := []struct {
		name         string
		provider     string
		endpoint     string
		model        string
		upstreamCode int
		upstreamBody string
	}{
		{
			name:         "claude empty 204",
			provider:     "claude",
			endpoint:     "https://api.anthropic.example",
			model:        "claude-test",
			upstreamCode: http.StatusNoContent,
			upstreamBody: "",
		},
		{
			name:         "gemini empty candidates 206",
			provider:     "gemini",
			endpoint:     "https://generativelanguage.googleapis.com/v1beta",
			model:        "gemini-test",
			upstreamCode: http.StatusPartialContent,
			upstreamBody: `{"candidates":[]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.upstreamCode,
					Status:     fmt.Sprintf("%d test", tc.upstreamCode),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.upstreamBody)),
				}, nil
			})}

			_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:   strPtr("provider-key"),
				Endpoint: strPtr(tc.endpoint),
				Model:    strPtr(tc.model),
				Provider: strPtr(tc.provider),
				Messages: []any{map[string]any{"role": "user", "content": "return text"}},
			})
			var emptyErr *proxyEmptyContentError
			if !errors.As(err, &emptyErr) {
				t.Fatalf("error = %T %v, want *proxyEmptyContentError", err, err)
			}
			if emptyErr.Provider != tc.provider {
				t.Fatalf("empty provider = %q, want %q", emptyErr.Provider, tc.provider)
			}
			if status != tc.upstreamCode {
				t.Fatalf("status = %d, want actual upstream %d", status, tc.upstreamCode)
			}
		})
	}
}

func TestProxyVertexEmptyContentPreservesActual2xxStatus(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
	generateStatus := http.StatusMultiStatus
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth2.googleapis.com/token":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"vertex-token","expires_in":3600}`)),
			}, nil
		case "https://aiplatform.googleapis.com/v1/projects/proj/locations/global/publishers/google/models/gemini-test:generateContent":
			return &http.Response{
				StatusCode: generateStatus,
				Status:     "207 Multi-Status",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[]}`)),
			}, nil
		default:
			t.Fatalf("unexpected request URL: %s", r.URL.String())
			return nil, nil
		}
	})}

	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   &credential,
		Endpoint: strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("vertex"),
		Messages: []any{map[string]any{"role": "user", "content": "return text"}},
	})
	var emptyErr *proxyEmptyContentError
	if !errors.As(err, &emptyErr) {
		t.Fatalf("error = %T %v, want *proxyEmptyContentError", err, err)
	}
	if emptyErr.Provider != "vertex" {
		t.Fatalf("empty provider = %q, want vertex", emptyErr.Provider)
	}
	if status != generateStatus {
		t.Fatalf("status = %d, want actual upstream %d", status, generateStatus)
	}
}

func TestProxyLocalRequestErrorsAreTypedSeparatelyFromUpstreamHTTP(t *testing.T) {
	assertLocal := func(t *testing.T, status int, err error, wantStage string) {
		t.Helper()
		var localErr *proxyLocalRequestError
		if !errors.As(err, &localErr) {
			t.Fatalf("error = %T %v, want *proxyLocalRequestError", err, err)
		}
		if localErr.Stage != wantStage {
			t.Fatalf("local stage = %q, want %q", localErr.Stage, wantStage)
		}
		if localErr.Cause == nil {
			t.Fatal("local error did not preserve cause")
		}
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
	}

	t.Run("missing configuration", func(t *testing.T) {
		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider: strPtr("openai"),
			Model:    strPtr("gpt-test"),
			APIKey:   strPtr("sk-test"),
		})
		assertLocal(t, status, err, "configuration")
	})

	t.Run("unsupported provider", func(t *testing.T) {
		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider: strPtr("not-a-provider"),
			Endpoint: strPtr("https://api.example.com"),
			Model:    strPtr("model"),
			APIKey:   strPtr("key"),
		})
		assertLocal(t, status, err, "configuration")
	})

	t.Run("invalid override json", func(t *testing.T) {
		invalid := `["not-an-object"]`
		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider:      strPtr("openai"),
			Endpoint:      strPtr("https://api.example.com"),
			Model:         strPtr("gpt-test"),
			APIKey:        strPtr("sk-test"),
			ExtraBodyJSON: &invalid,
		})
		assertLocal(t, status, err, "request_build")
	})

	t.Run("json response mime conflict", func(t *testing.T) {
		conflict := `{"generationConfig":{"responseMimeType":"text/plain"}}`
		_, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
			Provider:      strPtr("gemini"),
			Endpoint:      strPtr("https://generativelanguage.googleapis.com/v1beta"),
			Model:         strPtr("gemini-test"),
			APIKey:        strPtr("gem-key"),
			ExtraBodyJSON: &conflict,
		}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
		assertLocal(t, status, err, "request_build")
	})

	t.Run("vertex endpoint project resolution", func(t *testing.T) {
		credentialWithoutProject := `{"client_email":"test@example.com","private_key":"unused"}`
		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider: strPtr("vertex"),
			Endpoint: strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
			Model:    strPtr("gemini-test"),
			APIKey:   &credentialWithoutProject,
		})
		assertLocal(t, status, err, "request_build")
	})

	for name, credential := range map[string]string{
		"vertex malformed credential json": `{not-json`,
		"vertex missing credential fields": `{"client_email":"test@example.com"}`,
		"vertex invalid rsa key":           `{"client_email":"test@example.com","private_key":"not-a-private-key"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				Provider: strPtr("vertex"),
				Endpoint: strPtr("https://aiplatform.googleapis.com/v1/projects/proj/locations/global/publishers/google/models"),
				Model:    strPtr("gemini-test"),
				APIKey:   &credential,
				Messages: []any{map[string]any{"role": "user", "content": "return text"}},
			})
			assertLocal(t, status, err, "configuration")
		})
	}

	t.Run("actual upstream 400 remains upstream", func(t *testing.T) {
		oldClient := proxyHTTPClient
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream rejected request"}}`)),
			}, nil
		})}
		defer func() { proxyHTTPClient = oldClient }()

		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider: strPtr("openai"),
			Endpoint: strPtr("https://api.example.com/v1"),
			Model:    strPtr("gpt-test"),
			APIKey:   strPtr("sk-test"),
		})
		if err == nil {
			t.Fatal("expected upstream HTTP error")
		}
		var localErr *proxyLocalRequestError
		if errors.As(err, &localErr) {
			t.Fatalf("actual upstream error was misclassified as local: %+v", localErr)
		}
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want upstream 400", status)
		}
	})
}

func TestFormatMomentumSuffixOnlyForReadyOrPartialPackets(t *testing.T) {
	ready := map[string]any{
		"packet_status":    "ready",
		"next_pressure":    []any{map[string]any{"label": "answer the confession"}},
		"tension_to_reuse": []any{map[string]any{"label": "old promise"}},
	}
	suffix := formatMomentumSuffix(&ready)
	if !strings.Contains(suffix, "[Story Momentum Packet]") || !strings.Contains(suffix, "answer the confession") {
		t.Fatalf("ready suffix missing packet content: %q", suffix)
	}
	empty := map[string]any{"packet_status": "empty"}
	if got := formatMomentumSuffix(&empty); got != "" {
		t.Fatalf("empty packet suffix = %q, want empty", got)
	}
}

func TestHandleProxyPluginMainValidEndpointCallsUpstream(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.com/v1/chat/completions" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"cmpl-test","model":"gpt-4","choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"openai","endpoint":"https://api.example.com/v1","model":"gpt-4","api_key":"sk-test","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", resp["model"])
	}
	if resp["endpoint_validated"] != true {
		t.Errorf("endpoint_validated = %v, want true", resp["endpoint_validated"])
	}
	if resp["upstream_call_enabled"] != true {
		t.Errorf("upstream_call_enabled = %v, want true", resp["upstream_call_enabled"])
	}
}

func TestHandleProxyPluginMainOllamaLoopbackWithoutAPIKey(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "http://127.0.0.1:11434/v1/chat/completions" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want omitted", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"local-model","choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"ollama","endpoint":"http://127.0.0.1:11434/v1","model":"local-model","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestOllamaRuntimeLLMConfigDoesNotRequireAPIKey(t *testing.T) {
	cfg := completeTurnLLMConfig{
		Provider:  "ollama",
		Endpoint:  "http://127.0.0.1:11434/v1",
		Model:     "local-model",
		TimeoutMs: 45_000,
	}
	if !cfg.hasConfig() {
		t.Fatalf("local Ollama config should be complete without an API key; missing=%v", cfg.missingFields())
	}

	openAI := cfg
	openAI.Provider = "openai"
	if openAI.hasConfig() || !strings.Contains(strings.Join(openAI.missingFields(), ","), "api_key") {
		t.Fatalf("non-Ollama providers must still require an API key; missing=%v", openAI.missingFields())
	}
}

func TestHandleProxyPluginMainMissingProviderReturns400WithoutFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"endpoint":"https://api.example.com/v1","model":"gpt-4","api_key":"sk-test","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "provider / endpoint") {
		t.Fatalf("missing provider error not surfaced: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"config_error"`) || !strings.Contains(rec.Body.String(), `"upstream_call_enabled":false`) {
		t.Fatalf("missing provider should be a local config error without upstream call: %s", rec.Body.String())
	}
}

func TestProxyOpenAILikeReasoningFallbackRemovesUnsupportedParams(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	var fallbackBody map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if calls == 1 {
			if _, ok := body["reasoning_effort"]; !ok {
				t.Fatalf("first request missing reasoning_effort: %+v", body)
			}
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported parameter: reasoning_effort"}}`)),
			}, nil
		}
		fallbackBody = body
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"gpt-test","choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "low"
	req := dto.ProxyPluginMainRequest{
		APIKey:              strPtr("sk-test"),
		Endpoint:            strPtr("https://api.example.com/v1"),
		Model:               strPtr("gpt-test"),
		Provider:            strPtr("openai"),
		Messages:            []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:           int64Ptr(5),
		MaxCompletionTokens: int64Ptr(256),
		ReasoningEffort:     &effort,
	}
	resp, status, err := performProxyPluginMain(context.Background(), req)
	if err != nil {
		t.Fatalf("performProxyPluginMain error: %v", err)
	}
	if status != http.StatusOK || resp["model"] != "gpt-test" {
		t.Fatalf("unexpected response status=%d resp=%+v", status, resp)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if _, ok := fallbackBody["reasoning_effort"]; ok {
		t.Fatalf("fallback body kept reasoning_effort: %+v", fallbackBody)
	}
	if _, ok := fallbackBody["max_completion_tokens"]; ok {
		t.Fatalf("fallback body kept max_completion_tokens: %+v", fallbackBody)
	}
	if fallbackBody["max_tokens"] != float64(5) && fallbackBody["max_tokens"] != int64(5) {
		t.Fatalf("fallback max_tokens = %v, want 5", fallbackBody["max_tokens"])
	}
}

func TestProxyLLMGatewayServiceTierRoutingAndTrace(t *testing.T) {
	for _, tier := range []string{"standard", "flex", "priority"} {
		t.Run(tier, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if got := r.URL.String(); got != "https://api.llmgateway.io/v1/chat/completions" {
					t.Fatalf("upstream URL = %q", got)
				}
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				expected := tier
				if expected == "standard" {
					expected = "default"
				}
				if body["service_tier"] != expected {
					t.Fatalf("service_tier = %v, want %q; body=%+v", body["service_tier"], expected, body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"gateway/test","service_tier":"` + expected + `","choices":[{"message":{"content":"ok"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:                strPtr("llmg-test"),
				Endpoint:              strPtr("https://api.llmgateway.io/v1"),
				Model:                 strPtr("gateway/test"),
				Provider:              strPtr("llmgateway"),
				LLMGatewayServiceTier: strPtr(tier),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			expected := tier
			if expected == "standard" {
				expected = "default"
			}
			if trace["provider"] != "llmgateway" ||
				trace["llm_gateway_service_tier_requested"] != expected ||
				trace["llm_gateway_service_tier_applied"] != true ||
				trace["llm_gateway_service_tier_served"] != expected {
				t.Fatalf("unexpected LLM Gateway tier trace: %+v", trace)
			}
		})
	}
	if got := proxyOpenAIBaseURL("llmgateway", ""); got != "https://api.llmgateway.io/v1" {
		t.Fatalf("LLM Gateway default base = %q", got)
	}
}

func TestProxyLLMGatewayInvalidAndConflictingTiersFailBeforeUpstream(t *testing.T) {
	tests := []struct {
		name      string
		tier      string
		extraBody string
		wantError string
	}{
		{name: "invalid", tier: "economy", wantError: "must be standard, flex, or priority"},
		{name: "conflict", tier: "flex", extraBody: `{"service_tier":"priority"}`, wantError: "conflicts with extra_body_json"},
		{name: "wrong provider", tier: "flex", wantError: "requires provider llmgateway"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			upstreamCalls := 0
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				upstreamCalls++
				t.Fatalf("invalid tier must fail before upstream call: %s", r.URL.String())
				return nil, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			provider := "llmgateway"
			if tc.name == "wrong provider" {
				provider = "openai"
			}
			req := dto.ProxyPluginMainRequest{
				APIKey:                strPtr("llmg-test"),
				Endpoint:              strPtr("https://api.llmgateway.io/v1"),
				Model:                 strPtr("gateway/test"),
				Provider:              strPtr(provider),
				LLMGatewayServiceTier: strPtr(tc.tier),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			}
			if tc.extraBody != "" {
				req.ExtraBodyJSON = strPtr(tc.extraBody)
			}
			_, status, err := performProxyPluginMain(context.Background(), req)
			if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("status=%d err=%v, want %q", status, err, tc.wantError)
			}
			if upstreamCalls != 0 {
				t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
			}
		})
	}
}

func TestProxyLLMGatewayUnsupportedTierDoesNotFallback(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"unsupported_service_tier","message":"unsupported parameter service_tier for this model"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "low"
	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("llmg-test"),
		Endpoint:              strPtr("https://api.llmgateway.io/v1"),
		Model:                 strPtr("gateway/test"),
		Provider:              strPtr("llmgateway"),
		LLMGatewayServiceTier: strPtr("flex"),
		ReasoningEffort:       &effort,
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "unsupported_service_tier") {
		t.Fatalf("status=%d err=%v, want unsupported_service_tier", status, err)
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstreamCalls = %d, want 1 without fallback", upstreamCalls)
	}
}

func TestProxyLLMGatewayServedTierNotReportedAndUntypedExtraBodyPreserved(t *testing.T) {
	oldClient := proxyHTTPClient
	call := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		call++
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if call == 2 && body["service_tier"] != "flex" {
			t.Fatalf("untyped extra_body_json service_tier lost: %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("llmg-test"),
		Endpoint:              strPtr("https://api.llmgateway.io/v1"),
		Model:                 strPtr("gateway/test"),
		Provider:              strPtr("llmgateway"),
		LLMGatewayServiceTier: strPtr("flex"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("typed tier status=%d err=%v", status, err)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["llm_gateway_service_tier_served"] != "not_reported" {
		t.Fatalf("served tier trace = %+v", trace)
	}

	extraBody := `{"service_tier":"flex"}`
	_, status, err = performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:        strPtr("llmg-test"),
		Endpoint:      strPtr("https://api.llmgateway.io/v1"),
		Model:         strPtr("gateway/test"),
		Provider:      strPtr("llmgateway"),
		ExtraBodyJSON: &extraBody,
		Messages:      []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("untyped extra body status=%d err=%v", status, err)
	}
}

func TestProxyClaudePromptCacheModesAndUsageTrace(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantTTL string
	}{
		{name: "automatic 5 minutes", mode: "ephemeral_5m"},
		{name: "automatic 1 hour", mode: "ephemeral_1h", wantTTL: "1h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				cacheControl := mapFromAny(body["cache_control"])
				if cacheControl["type"] != "ephemeral" {
					t.Fatalf("cache_control type = %v, body=%+v", cacheControl["type"], body)
				}
				if tc.wantTTL == "" {
					if _, exists := cacheControl["ttl"]; exists {
						t.Fatalf("5m mapping must omit ttl: %+v", cacheControl)
					}
				} else if cacheControl["ttl"] != tc.wantTTL {
					t.Fatalf("cache_control ttl = %v, want %q", cacheControl["ttl"], tc.wantTTL)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"model":"claude-test",
						"content":[{"type":"text","text":"ok"}],
						"usage":{
							"input_tokens":21,
							"output_tokens":3,
							"cache_creation_input_tokens":13,
							"cache_read_input_tokens":8,
							"service_tier":"standard"
						}
					}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:                strPtr("anthropic-test"),
				Endpoint:              strPtr("https://api.anthropic.com"),
				Model:                 strPtr("claude-test"),
				Provider:              strPtr("claude"),
				ClaudePromptCacheMode: strPtr(tc.mode),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			usage := mapFromAny(resp["usage"])
			if usage["cache_creation_input_tokens"] != float64(13) ||
				usage["cache_read_input_tokens"] != float64(8) ||
				usage["service_tier"] != "standard" {
				t.Fatalf("normalized Claude usage missing raw fields: %+v", usage)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["provider"] != "claude" ||
				trace["claude_prompt_cache_mode_requested"] != tc.mode ||
				trace["claude_prompt_cache_mode_applied"] != true {
				t.Fatalf("unexpected Claude prompt cache trace: %+v", trace)
			}
			usageTrace := mapFromAny(trace["anthropic_usage"])
			if usageTrace["cache_creation_input_tokens"] != float64(13) ||
				usageTrace["cache_read_input_tokens"] != float64(8) ||
				usageTrace["service_tier"] != "standard" {
				t.Fatalf("Anthropic usage trace missing raw fields: %+v", usageTrace)
			}
		})
	}
}

func TestProxyClaudePromptCacheOffAndAbsentPreserveManualCacheControl(t *testing.T) {
	tests := []struct {
		name string
		mode *string
	}{
		{name: "typed absent"},
		{name: "typed off", mode: strPtr("off")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				cacheControl := mapFromAny(body["cache_control"])
				if cacheControl["type"] != "ephemeral" || cacheControl["ttl"] != "1h" {
					t.Fatalf("manual cache_control changed: %+v", cacheControl)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			extraBody := `{"cache_control":{"type":"ephemeral","ttl":"1h"}}`
			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:                strPtr("anthropic-test"),
				Endpoint:              strPtr("https://api.anthropic.com"),
				Model:                 strPtr("claude-test"),
				Provider:              strPtr("claude"),
				ClaudePromptCacheMode: tc.mode,
				ExtraBodyJSON:         &extraBody,
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			if _, exists := resp["usage"]; exists {
				t.Fatalf("normalized response invented usage: %+v", resp)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if _, exists := trace["anthropic_usage"]; exists {
				t.Fatalf("trace invented Anthropic usage: %+v", trace)
			}
		})
	}
}

func TestProxyClaudePromptCacheMatchingExtraBodyAccepted(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		extraBody string
	}{
		{
			name:      "5m explicit ttl is equivalent",
			mode:      "ephemeral_5m",
			extraBody: `{"cache_control":{"type":"ephemeral","ttl":"5m"}}`,
		},
		{
			name:      "1h exact match",
			mode:      "ephemeral_1h",
			extraBody: `{"cache_control":{"type":"ephemeral","ttl":"1h"}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:                strPtr("anthropic-test"),
				Endpoint:              strPtr("https://api.anthropic.com"),
				Model:                 strPtr("claude-test"),
				Provider:              strPtr("claude"),
				ClaudePromptCacheMode: strPtr(tc.mode),
				ExtraBodyJSON:         strPtr(tc.extraBody),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["claude_prompt_cache_mode_source"] != "typed_and_extra_body_json" ||
				trace["claude_prompt_cache_mode_applied"] != true {
				t.Fatalf("matching cache_control trace = %+v", trace)
			}
		})
	}
}

func TestProxyClaudePromptCacheInvalidWrongProviderAndConflictsFailBeforeUpstream(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		mode      string
		extraBody string
		wantError string
	}{
		{name: "invalid", provider: "claude", mode: "automatic", wantError: "must be off, ephemeral_5m, or ephemeral_1h"},
		{name: "uppercase near miss", provider: "claude", mode: "EPHEMERAL_5M", wantError: "must be off, ephemeral_5m, or ephemeral_1h"},
		{name: "hyphen near miss", provider: "claude", mode: "ephemeral-5m", wantError: "must be off, ephemeral_5m, or ephemeral_1h"},
		{name: "wrong provider", provider: "openai", mode: "ephemeral_5m", wantError: "requires provider claude"},
		{name: "conflicting ttl", provider: "claude", mode: "ephemeral_1h", extraBody: `{"cache_control":{"type":"ephemeral"}}`, wantError: "conflicts with extra_body_json"},
		{name: "extra keys conflict", provider: "claude", mode: "ephemeral_5m", extraBody: `{"cache_control":{"type":"ephemeral","scope":"session"}}`, wantError: "conflicts with extra_body_json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			upstreamCalls := 0
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				upstreamCalls++
				return nil, fmt.Errorf("unexpected upstream call")
			})}
			defer func() { proxyHTTPClient = oldClient }()

			req := dto.ProxyPluginMainRequest{
				APIKey:                strPtr("anthropic-test"),
				Endpoint:              strPtr("https://api.anthropic.com"),
				Model:                 strPtr("claude-test"),
				Provider:              strPtr(tc.provider),
				ClaudePromptCacheMode: strPtr(tc.mode),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			}
			if tc.extraBody != "" {
				req.ExtraBodyJSON = strPtr(tc.extraBody)
			}
			_, status, err := performProxyPluginMain(context.Background(), req)
			if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("status=%d err=%v, want %q", status, err, tc.wantError)
			}
			if upstreamCalls != 0 {
				t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
			}
		})
	}
}

func TestProxyClaudePromptCacheWrongCopilotProviderFailsBeforeTokenExchange(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return nil, fmt.Errorf("unexpected upstream call to %s", r.URL.String())
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("github-token"),
		Endpoint:              strPtr("https://api.githubcopilot.com"),
		Model:                 strPtr("gpt-test"),
		Provider:              strPtr("copilot"),
		ClaudePromptCacheMode: strPtr("ephemeral_5m"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "requires provider claude") {
		t.Fatalf("status=%d err=%v, want local Claude provider error", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0 before Copilot token exchange", upstreamCalls)
	}
}

func TestProxyClaudePromptCacheWrongVertexProviderFailsBeforeOAuthExchange(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return nil, fmt.Errorf("unexpected upstream call to %s", r.URL.String())
	})}
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                &credential,
		Endpoint:              strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
		Model:                 strPtr("gemini-test"),
		Provider:              strPtr("vertex"),
		ClaudePromptCacheMode: strPtr("ephemeral_5m"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "requires provider claude") {
		t.Fatalf("status=%d err=%v, want local Claude provider error", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0 before Vertex OAuth exchange", upstreamCalls)
	}
}

func TestProxyGLM52SendsReasoningEffortWithThinkingEnabled(t *testing.T) {
	oldClient := proxyHTTPClient
	var upstreamBody map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"glm-5.2","choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "max"
	preset := "glm"
	req := dto.ProxyPluginMainRequest{
		APIKey:          strPtr("sk-test"),
		Endpoint:        strPtr("https://api.z.ai/api/paas/v4"),
		Model:           strPtr("glm-5.2"),
		Provider:        strPtr("custom"),
		Messages:        []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:       int64Ptr(5),
		ReasoningPreset: &preset,
		ReasoningEffort: &effort,
	}
	if _, status, err := performProxyPluginMain(context.Background(), req); err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
	}
	thinking, _ := upstreamBody["thinking"].(map[string]any)
	if thinking["type"] != "enabled" {
		t.Fatalf("GLM-5.2 thinking = %+v, want enabled", upstreamBody["thinking"])
	}
	if upstreamBody["reasoning_effort"] != "max" {
		t.Fatalf("GLM-5.2 reasoning_effort = %v, want max; body=%+v", upstreamBody["reasoning_effort"], upstreamBody)
	}
}

func TestProxyGLM52ReasoningNoneDisablesThinking(t *testing.T) {
	oldClient := proxyHTTPClient
	var upstreamBody map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"glm-5.2","choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "none"
	preset := "glm"
	req := dto.ProxyPluginMainRequest{
		APIKey:          strPtr("sk-test"),
		Endpoint:        strPtr("https://api.z.ai/api/paas/v4"),
		Model:           strPtr("glm-5.2"),
		Provider:        strPtr("custom"),
		Messages:        []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:       int64Ptr(5),
		ReasoningPreset: &preset,
		ReasoningEffort: &effort,
	}
	if _, status, err := performProxyPluginMain(context.Background(), req); err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
	}
	thinking, _ := upstreamBody["thinking"].(map[string]any)
	if thinking["type"] != "disabled" {
		t.Fatalf("GLM-5.2 thinking = %+v, want disabled", upstreamBody["thinking"])
	}
	if _, ok := upstreamBody["reasoning_effort"]; ok {
		t.Fatalf("GLM-5.2 none should not send reasoning_effort: %+v", upstreamBody)
	}
}

func TestProxyGeminiNormalizesNativeResponse(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://generativelanguage.googleapis.com/v1beta/models/gemini-test:generateContent" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gem-key" {
			t.Fatalf("x-goog-api-key = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if mime := mapFromAny(body["generationConfig"])["responseMimeType"]; mime != nil {
			t.Fatalf("ordinary Gemini call unexpectedly forced responseMimeType: %v", mime)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"gemini ok"}]}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("gem-key"),
		Endpoint: strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("gemini"),
		Messages: []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil {
		t.Fatalf("performProxyPluginMain error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	got := chatCompletionText(resp)
	if got != "gemini ok" {
		t.Fatalf("content = %q, want gemini ok", got)
	}
}

func TestProxyGeminiJSONPolicyAddsMimeAndTrace(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if mime := mapFromAny(body["generationConfig"])["responseMimeType"]; mime != "application/json" {
			t.Fatalf("responseMimeType = %v, want application/json; body=%+v", mime, body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{\"turn_summary\":\"ok\"}"}]}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("gem-key"),
		Endpoint: strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("gemini"),
		Messages: []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_applied"] != true ||
		trace["json_response_source"] != "backend_policy" ||
		trace["json_response_mime_type"] != "application/json" ||
		trace["json_response_purpose"] != "complete_turn_critic" {
		t.Fatalf("unexpected JSON response trace: %+v", trace)
	}
}

func TestProxyGeminiJSONPolicyPreservesMatchingExtraBody(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		generationConfig := mapFromAny(body["generationConfig"])
		if generationConfig["responseMimeType"] != "application/json" || generationConfig["topP"] != float64(0.8) {
			t.Fatalf("matching ExtraBodyJSON was not preserved: %+v", generationConfig)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	extraBody := `{"generationConfig":{"responseMimeType":"application/json","topP":0.8}}`
	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:        strPtr("gem-key"),
		Endpoint:      strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:         strPtr("gemini-test"),
		Provider:      strPtr("gemini"),
		ExtraBodyJSON: &extraBody,
		Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_source"] != "extra_body_json" || trace["json_response_applied"] != true {
		t.Fatalf("unexpected matching ExtraBody trace: %+v", trace)
	}
}

func TestProxyGeminiJSONPolicyRejectsConflictingExtraBodyWithoutCall(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		t.Fatalf("conflicting JSON response MIME must fail before upstream call: %s", r.URL.String())
		return nil, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	extraBody := `{"generationConfig":{"responseMimeType":"text/plain"}}`
	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:        strPtr("gem-key"),
		Endpoint:      strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:         strPtr("gemini-test"),
		Provider:      strPtr("gemini"),
		ExtraBodyJSON: &extraBody,
		Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "json_response_mime_conflict") {
		t.Fatalf("conflict status=%d err=%v, want stable config error", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_conflict"] != true ||
		trace["json_response_existing_value"] != "text/plain" ||
		trace["json_response_applied"] != false {
		t.Fatalf("unexpected conflict trace: %+v", trace)
	}
}

func TestProxyGeminiJSONPolicyRejectsNonObjectGenerationConfigWithoutCall(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		t.Fatalf("invalid generationConfig must fail before upstream call: %s", r.URL.String())
		return nil, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	extraBody := `{"generationConfig":"invalid"}`
	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:        strPtr("gem-key"),
		Endpoint:      strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:         strPtr("gemini-test"),
		Provider:      strPtr("gemini"),
		ExtraBodyJSON: &extraBody,
		Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "json_response_generation_config_conflict") {
		t.Fatalf("conflict status=%d err=%v, want stable generationConfig error", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_conflict"] != true ||
		trace["json_response_existing_type"] != "string" ||
		trace["json_response_applied"] != false {
		t.Fatalf("unexpected generationConfig conflict trace: %+v", trace)
	}
}

func TestProxyJSONPolicyDoesNotAlterOpenAILikeRequest(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["response_format"] != nil || body["generationConfig"] != nil {
			t.Fatalf("OpenAI-like request was altered by native JSON policy: %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"gpt-test","choices":[{"message":{"content":"{}"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("sk-test"),
		Endpoint: strPtr("https://api.example.com/v1"),
		Model:    strPtr("gpt-test"),
		Provider: strPtr("openai"),
		Messages: []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
	}
}

func TestProxyGeminiThinkingNoneAvoidsOpenAIReasoningFields(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		if strings.Contains(body, "reasoning_effort") || strings.Contains(body, "max_completion_tokens") {
			t.Fatalf("Gemini request leaked OpenAI reasoning fields: %s", body)
		}
		if !strings.Contains(body, `"generationConfig"`) || !strings.Contains(body, `"maxOutputTokens"`) {
			t.Fatalf("Gemini request missing native generationConfig: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"gemini none ok"}]}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "none"
	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:          strPtr("gem-key"),
		Endpoint:        strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:           strPtr("gemini-2.5-flash"),
		Provider:        strPtr("gemini"),
		ReasoningEffort: &effort,
		Messages:        []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil {
		t.Fatalf("performProxyPluginMain error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if got := chatCompletionText(resp); got != "gemini none ok" {
		t.Fatalf("content = %q, want gemini none ok", got)
	}
}

func TestProxyVertexNormalizesNativeResponse(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.String() {
		case "https://oauth2.googleapis.com/token":
			raw, _ := io.ReadAll(r.Body)
			body := string(raw)
			if r.Method != http.MethodPost || !strings.Contains(body, "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer") {
				t.Fatalf("unexpected Vertex token request method/body: %s %s", r.Method, body)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"vertex-token","expires_in":3600}`)),
			}, nil
		case "https://us-central1-aiplatform.googleapis.com/v1/projects/proj/locations/us-central1/publishers/google/models/gemini-2.5-flash:generateContent":
			if got := r.Header.Get("Authorization"); got != "Bearer vertex-token" {
				t.Fatalf("Authorization = %q", got)
			}
			if got := r.Header.Get("x-goog-api-key"); got != "" {
				t.Fatalf("Vertex request should not use x-goog-api-key, got %q", got)
			}
			raw, _ := io.ReadAll(r.Body)
			body := string(raw)
			for _, want := range []string{`"systemInstruction"`, `"contents"`, `"generationConfig"`, `"maxOutputTokens"`} {
				if !strings.Contains(body, want) {
					t.Fatalf("Vertex request missing %s: %s", want, body)
				}
			}
			if strings.Contains(body, "reasoning_effort") || strings.Contains(body, "max_completion_tokens") {
				t.Fatalf("Vertex request leaked OpenAI reasoning fields: %s", body)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("decode Vertex request: %v", err)
			}
			if mime := mapFromAny(decoded["generationConfig"])["responseMimeType"]; mime != nil {
				t.Fatalf("ordinary Vertex call unexpectedly forced responseMimeType: %v", mime)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"vertex ok"}]}}]}`)),
			}, nil
		default:
			t.Fatalf("unexpected request URL: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
	effort := "none"
	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:              &credential,
		Endpoint:            strPtr("https://us-central1-aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/us-central1/publishers/google/models"),
		Model:               strPtr("gemini-2.5-flash"),
		Provider:            strPtr("vertex"),
		ReasoningEffort:     &effort,
		MaxTokens:           int64Ptr(5),
		MaxCompletionTokens: int64Ptr(32),
		Messages: []any{
			map[string]any{"role": "system", "content": "system"},
			map[string]any{"role": "user", "content": "ping"},
		},
	})
	if err != nil {
		t.Fatalf("performProxyPluginMain error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want token + generateContent", calls)
	}
	if got := chatCompletionText(resp); got != "vertex ok" {
		t.Fatalf("content = %q, want vertex ok", got)
	}
}

func TestProxyVertexFlexAndExtraBodyOverrides(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.String() {
		case "https://oauth2.googleapis.com/token":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"vertex-token","expires_in":3600}`)),
			}, nil
		case "https://aiplatform.googleapis.com/v1/projects/proj/locations/global/publishers/google/models/gemini-3.5-flash:generateContent":
			if got := r.Header.Get("X-Vertex-AI-LLM-Shared-Request-Type"); got != "flex" {
				t.Fatalf("shared request type = %q, want flex", got)
			}
			if got := r.Header.Get("X-Vertex-AI-LLM-Request-Type"); got != "shared" {
				t.Fatalf("request type = %q, want shared", got)
			}
			if got := r.Header.Get("X-Test-Feature"); got != "enabled" {
				t.Fatalf("extra header = %q, want enabled", got)
			}
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("decode upstream body: %v", err)
			}
			genCfg := mapFromAny(body["generationConfig"])
			if genCfg["responseMimeType"] != "application/json" {
				t.Fatalf("extra body did not merge generationConfig: %+v", genCfg)
			}
			if body["model"] != nil || body["stream"] != nil {
				t.Fatalf("protected extra body keys should be blocked: %+v", body)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"vertex flex ok"}]}}]}`)),
			}, nil
		default:
			t.Fatalf("unexpected request URL: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
	flex := "flex_only"
	headersJSON := `{"X-Test-Feature":"enabled","Authorization":"bad"}`
	bodyJSON := `{"generationConfig":{"topP":0.9},"model":"bad","stream":true}`
	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:           &credential,
		Endpoint:         strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
		Model:            strPtr("gemini-3.5-flash"),
		Provider:         strPtr("vertex"),
		VertexFlexMode:   &flex,
		ExtraHeadersJSON: &headersJSON,
		ExtraBodyJSON:    &bodyJSON,
		MaxTokens:        int64Ptr(5),
		Messages:         []any{map[string]any{"role": "user", "content": "ping"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err != nil {
		t.Fatalf("performProxyPluginMain error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want token + generateContent", calls)
	}
	if got := chatCompletionText(resp); got != "vertex flex ok" {
		t.Fatalf("content = %q, want vertex flex ok", got)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["vertex_flex_applied"] != true ||
		trace["json_response_applied"] != true ||
		trace["json_response_source"] != "backend_policy" {
		t.Fatalf("missing override trace: %+v", trace)
	}
}

func TestProxyRejectsInvalidExtraHeadersJSON(t *testing.T) {
	bad := `["not-object"]`
	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:           strPtr("sk-test"),
		Endpoint:         strPtr("https://api.example.com/v1"),
		Model:            strPtr("gpt-test"),
		Provider:         strPtr("openai"),
		ExtraHeadersJSON: &bad,
		Messages:         []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil {
		t.Fatalf("expected invalid JSON object error")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestCallEmbeddingGeminiUsesEmbedContent(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-test:embedContent" {
			t.Fatalf("upstream URL = %q", got)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gem-key" {
			t.Fatalf("x-goog-api-key = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), `"content"`) || !strings.Contains(string(raw), "embed me") {
			t.Fatalf("unexpected body: %s", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"embedding":{"values":[0.1,0.2,0.3]}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embedding, model, err := callEmbedding(context.Background(), completeTurnEmbeddingConfig{
		APIKey:   "gem-key",
		Endpoint: "https://generativelanguage.googleapis.com/v1beta",
		Model:    "text-embedding-test",
		Provider: "gemini",
	}, "embed me")
	if err != nil {
		t.Fatalf("callEmbedding error: %v", err)
	}
	if model != "text-embedding-test" {
		t.Fatalf("model = %q", model)
	}
	if embedding != `[0.1,0.2,0.3]` {
		t.Fatalf("embedding = %q", embedding)
	}
}

func TestCallEmbeddingVertexUsesEmbedContent(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch r.URL.String() {
		case "https://oauth2.googleapis.com/token":
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"vertex-token","expires_in":3600}`)),
			}, nil
		case "https://us-central1-aiplatform.googleapis.com/v1/projects/proj/locations/us-central1/publishers/google/models/text-embedding-005:embedContent":
			if got := r.Header.Get("Authorization"); got != "Bearer vertex-token" {
				t.Fatalf("Authorization = %q", got)
			}
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"content"`) || !strings.Contains(string(raw), "embed me") {
				t.Fatalf("unexpected body: %s", raw)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"embedding":{"values":[0.4,0.5,0.6]}}`)),
			}, nil
		default:
			t.Fatalf("unexpected request URL: %s", r.URL.String())
			return nil, nil
		}
	})}
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
	embedding, model, err := callEmbedding(context.Background(), completeTurnEmbeddingConfig{
		APIKey:   credential,
		Endpoint: "https://us-central1-aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/us-central1/publishers/google/models",
		Model:    "text-embedding-005",
		Provider: "vertex",
	}, "embed me")
	if err != nil {
		t.Fatalf("callEmbedding error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want token + embedContent", calls)
	}
	if model != "text-embedding-005" {
		t.Fatalf("model = %q", model)
	}
	if embedding != `[0.4,0.5,0.6]` {
		t.Fatalf("embedding = %q", embedding)
	}
}

func TestCallEmbeddingOllamaNative(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "http://127.0.0.1:11434/api/embed" {
			t.Fatalf("upstream URL = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), `"model":"nomic-embed-text"`) || !strings.Contains(string(raw), "embed me") {
			t.Fatalf("unexpected body: %s", raw)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"nomic-embed-text","embeddings":[[0.1,0.2,0.3]]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embedding, model, err := callEmbedding(context.Background(), completeTurnEmbeddingConfig{
		APIKey:   "unused",
		Endpoint: "http://127.0.0.1:11434",
		Model:    "nomic-embed-text",
		Provider: "ollama",
	}, "embed me")
	if err != nil {
		t.Fatalf("callEmbedding error: %v", err)
	}
	if model != "nomic-embed-text" {
		t.Fatalf("model = %q", model)
	}
	if embedding != `[0.1,0.2,0.3]` {
		t.Fatalf("embedding = %q", embedding)
	}
}

func TestProviderAndEmbeddingCallsWithoutRuntimeTimeoutInheritCallerCancellation(t *testing.T) {
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, status, err := callProxyProvider(cancelledCtx, dto.ProxyPluginMainRequest{
		Provider: strPtr("openai"),
		Endpoint: strPtr("https://api.example.com/v1"),
		Model:    strPtr("test-model"),
		APIKey:   strPtr("test-key"),
	})
	if status != http.StatusBadGateway || !errors.Is(err, context.Canceled) {
		t.Fatalf("proxy cancellation: status=%d err=%T %v", status, err, err)
	}

	_, _, err = callEmbedding(cancelledCtx, completeTurnEmbeddingConfig{
		Provider: "openai",
		Endpoint: "https://api.example.com/v1",
		Model:    "test-embedding",
		APIKey:   "test-key",
	}, "embed me")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("embedding cancellation error=%T %v", err, err)
	}
}

func TestHandleCriticTestReturnsReadOnlyEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-critic","turn_index":7,"turn_content":"critic target","context":[{"role":"user"}],"output_language_override":{"language":"ko"}}`
	req := httptest.NewRequest(http.MethodPost, "/critic/test", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %v, want shadow", resp["source"])
	}
	if resp["chat_session_id"] != "sess-critic" {
		t.Errorf("chat_session_id = %v, want sess-critic", resp["chat_session_id"])
	}
	if int(resp["turn_index"].(float64)) != 7 {
		t.Errorf("turn_index = %v, want 7", resp["turn_index"])
	}
	if int(resp["context_count"].(float64)) != 1 {
		t.Errorf("context_count = %v, want 1", resp["context_count"])
	}
	if resp["output_language_override_present"] != true {
		t.Errorf("output_language_override_present = %v, want true", resp["output_language_override_present"])
	}
	if resp["llm_call_enabled"] != false {
		t.Errorf("llm_call_enabled = %v, want false", resp["llm_call_enabled"])
	}
	if resp["verdict"] != "not_executed" {
		t.Errorf("verdict = %v, want not_executed", resp["verdict"])
	}

	trace := resp["trace_summary"].(map[string]any)
	if trace["prompt_source"] != "not_configured" {
		t.Errorf("trace.prompt_source = %v, want not_configured", trace["prompt_source"])
	}
	if trace["llm_call"] != "disabled" {
		t.Errorf("trace.llm_call = %v, want disabled", trace["llm_call"])
	}
}

func TestHandleCriticTestBadJSONReturns400(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/critic/test", bytes.NewReader([]byte(`{"turn_content":`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleSupervisorReadOnlyShadowEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-sv","guide_mode":"action","narrative_stance":"immersive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","context_messages":[{"role":"user","content":"A battle starts at the gate."}]}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["source"] != "shadow" {
		t.Errorf("source = %v, want shadow", resp["source"])
	}
	if resp["chat_session_id"] != "sess-sv" {
		t.Errorf("chat_session_id = %v, want sess-sv", resp["chat_session_id"])
	}
	if resp["would_call_llm"] != false {
		t.Errorf("would_call_llm = %v, want false", resp["would_call_llm"])
	}
	pack, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack is not an object")
	}
	if pack["status"] != "ready" {
		t.Errorf("supervisor_input_pack.status = %v, want ready", pack["status"])
	}
	if pack["would_call_llm"] != false {
		t.Errorf("supervisor_input_pack.would_call_llm = %v, want false", pack["would_call_llm"])
	}
	if pack["source"] != "go_supervisor_support_planner" {
		t.Errorf("supervisor_input_pack.source = %v", pack["source"])
	}
	promptPlan := stringSliceFromAny(pack["prompt_plan"])
	promptPlanText := strings.Join(promptPlan, " ")
	if strings.Contains(promptPlanText, "persistent_guidance") ||
		strings.Contains(promptPlanText, "supervisor_prompt.txt") ||
		!strings.Contains(promptPlanText, "supervisor_support_packet") {
		t.Errorf("supervisor prompt plan retained read-shadow inputs: %#v", promptPlan)
	}

	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}
	if trace["guide_mode"] != "action" {
		t.Errorf("guide_mode = %v, want action", trace["guide_mode"])
	}
	focus := stringSliceFromAny(pack["guide_focus"])
	if len(focus) == 0 || focus[0] != "clear cause and effect" {
		t.Errorf("guide_focus = %#v, want optional action focus", focus)
	}
	for _, key := range []string{"guide_suffix", "director_overrides", "narrative_stance", "narrative_stance_bounds"} {
		if _, exists := pack[key]; exists {
			t.Fatalf("supervisor pack exposes story-control field %q: %#v", key, pack[key])
		}
	}
	if trace["wake_up_context_present"] != true {
		t.Errorf("wake_up_context_present = %v, want true", trace["wake_up_context_present"])
	}
	if trace["persistent_guidance_present"] != true {
		t.Errorf("persistent_guidance_present = %v, want true", trace["persistent_guidance_present"])
	}
	if trace["context_messages_count"] != float64(1) {
		t.Errorf("context_messages_count = %v, want 1", trace["context_messages_count"])
	}
}

func TestHandleSupervisorUsesRuntimeLLMConfig(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.com/v1/chat/completions" {
			t.Fatalf("upstream URL = %q", got)
		}
		var upstreamReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		messages, _ := upstreamReq["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("upstream request missing messages: %+v", upstreamReq)
		}
		systemMsg, _ := messages[0].(map[string]any)
		userMsg, _ := messages[1].(map[string]any)
		systemPrompt := extractionStringFromAny(systemMsg["content"])
		userPrompt := extractionStringFromAny(userMsg["content"])
		if !strings.Contains(userPrompt, "response_execution_contract") ||
			!strings.Contains(userPrompt, "supervisor_support_packet") ||
			!strings.Contains(userPrompt, "guide_focus") {
			t.Fatalf("supervisor request body missing bounded memory guidance inputs: %s", userPrompt)
		}
		if !strings.Contains(systemPrompt, "source-backed narrative support reviewer") ||
			!strings.Contains(systemPrompt, "current user input is the only command source") {
			t.Fatalf("supervisor system prompt missing memory-guide boundary: %s", systemPrompt)
		}
		for _, forbidden := range []string{"Story Initiative", "max_new_beats", "narrative_stance", "auto_advance_trigger"} {
			if strings.Contains(systemPrompt, forbidden) || strings.Contains(userPrompt, `"`+forbidden+`"`) {
				t.Fatalf("supervisor prompt contains story-control field %q: system=%s user=%s", forbidden, systemPrompt, userPrompt)
			}
		}
		if !strings.Contains(systemPrompt, "may_advance") ||
			!strings.Contains(systemPrompt, "never authorize") {
			t.Fatalf("supervisor prompt missing bounded optional advance semantics: %s", systemPrompt)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"supervisor-model",
				"choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[],\"expression_hints\":[{\"kind\":\"portrayal\",\"text\":\"preserve the current request boundary\",\"source_refs\":[\"input:test\"]}]}}"}}]
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainApiKey":"sk-supervisor",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"supervisor-model",
		"mainProvider":"openai",
		"supervisorProvider":"openai",
		"supervisorApiKey":"sk-supervisor",
		"supervisorEndpoint":"https://api.example.com/v1",
		"supervisorModel":"supervisor-model",
		"supervisorTimeout":30
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}
	cfg := srv.supervisorLLMConfig()
	if cfg.Provider != "openai" {
		t.Fatalf("supervisor provider = %q, want openai", cfg.Provider)
	}
	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update response: %+v", updateResp)
	}
	supervisorTrace, ok := trace["supervisor"].(map[string]any)
	if !ok || supervisorTrace["configured"] != true {
		t.Fatalf("supervisor trace not configured: %+v", trace["supervisor"])
	}
	mainTrace, ok := trace["main"].(map[string]any)
	if !ok {
		t.Fatalf("main trace missing: %+v", trace)
	}
	directGeneration, ok := mainTrace["direct_generation"].(map[string]any)
	if !ok {
		t.Fatalf("main direct_generation trace missing: %+v", mainTrace)
	}
	if directGeneration["status"] != "risuai_host_retained" || directGeneration["enabled"] != false {
		t.Fatalf("unexpected direct generation trace: %+v", directGeneration)
	}

	body := `{"chat_session_id":"sess-sv-live","guide_mode":"romantic","guide_strength":"weak","narrative_stance":"proactive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test","memory:test:1"],"current_input":["input:test"],"native_system":[],"memory":["memory:test:1"]}},"context_messages":[{"role":"user","content":"move forward"}]}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["source"] != "runtime_llm" {
		t.Fatalf("source = %v, want runtime_llm", resp["source"])
	}
	if resp["would_call_llm"] != true {
		t.Fatalf("would_call_llm = %v, want true", resp["would_call_llm"])
	}
	result, ok := resp["supervisor_result"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_result is not an object: %+v", resp)
	}
	directive, _ := result["directive"].(map[string]any)
	proposal, _ := directive["supervisor_scene_proposal"].(map[string]any)
	if proposal["contract_version"] != "supervisor_scene_proposal.v3" ||
		proposal["authority"] != "proposal_only" || proposal["truth_authority"] != false ||
		proposal["would_write"] != false {
		t.Fatalf("supervisor proposal authority = %+v, want proposal_only/non-truth", proposal)
	}
	warnings, _ := proposal["fidelity_warnings"].([]any)
	expressions, _ := proposal["expression_hints"].([]any)
	if len(warnings) != 0 || len(expressions) != 1 ||
		mapFromAny(expressions[0])["kind"] != "portrayal" {
		t.Fatalf("supervisor proposal lanes = fidelity:%+v expression:%+v", warnings, expressions)
	}
	traceSummary, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object: %+v", resp)
	}
	for _, key := range []string{"narrative_stance", "narrative_stance_suffix_present", "narrative_stance_bounds_present", "narrative_stance_summary"} {
		if _, exists := traceSummary[key]; exists {
			t.Fatalf("trace_summary exposes story-control field %q: %+v", key, traceSummary)
		}
	}
}
