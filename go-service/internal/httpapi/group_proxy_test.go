package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	bodyJSON := `{"generationConfig":{"responseMimeType":"application/json"},"model":"bad","stream":true}`
	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:           &credential,
		Endpoint:         strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
		Model:            strPtr("gemini-3.5-flash"),
		Provider:         strPtr("vertex"),
		VertexFlexMode:   &flex,
		ExtraHeadersJSON: &headersJSON,
		ExtraBodyJSON:    &bodyJSON,
		MaxTokens:        int64Ptr(5),
		Messages:         []any{map[string]any{"role": "user", "content": "ping"}},
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
	if got := chatCompletionText(resp); got != "vertex flex ok" {
		t.Fatalf("content = %q, want vertex flex ok", got)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["vertex_flex_applied"] != true {
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
	if suffix, _ := pack["final_guidance_suffix"].(string); !strings.Contains(suffix, "Go R1 Supervisor Read Shadow") {
		t.Errorf("final_guidance_suffix missing read-shadow marker: %q", suffix)
	}

	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}
	if trace["guide_mode"] != "action" {
		t.Errorf("guide_mode = %v, want action", trace["guide_mode"])
	}
	if trace["guide_suffix_present"] != true {
		t.Errorf("guide_suffix_present = %v, want true", trace["guide_suffix_present"])
	}
	if suffix, _ := pack["guide_suffix"].(string); !strings.Contains(suffix, "Narrative Guide") || !strings.Contains(suffix, "Action") {
		t.Errorf("guide_suffix missing action narrative guide: %q", suffix)
	}
	overrides, ok := pack["director_overrides"].(map[string]any)
	if !ok {
		t.Fatalf("director_overrides is not an object")
	}
	emphasis, _ := overrides["emphasis"].([]any)
	if len(emphasis) == 0 {
		t.Errorf("director_overrides.emphasis is empty: %+v", overrides)
	}
	if trace["narrative_stance"] != "immersive" {
		t.Errorf("narrative_stance = %v, want immersive", trace["narrative_stance"])
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
		if !strings.Contains(userPrompt, "supervisor_input_pack") {
			t.Fatalf("supervisor request body missing input pack: %s", userPrompt)
		}
		if !strings.Contains(systemPrompt, "Narrative Guide") || !strings.Contains(systemPrompt, "Romantic") {
			t.Fatalf("supervisor system prompt missing guide mode suffix: %s", systemPrompt)
		}
		if !strings.Contains(systemPrompt, "Story Initiative - Proactive") || !strings.Contains(systemPrompt, "Story Initiative Bounds") {
			t.Fatalf("supervisor system prompt missing narrative stance suffix/bounds: %s", systemPrompt)
		}
		if !strings.Contains(userPrompt, `"narrative_stance": "proactive"`) ||
			!strings.Contains(userPrompt, `"narrative_stance_suffix"`) ||
			!strings.Contains(userPrompt, `"narrative_stance_bounds"`) {
			t.Fatalf("supervisor user prompt missing narrative stance payload: %s", userPrompt)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"supervisor-model",
				"choices":[{"message":{"content":"{\"directive\":{\"director\":{\"pressure_level\":\"normal\"}}}"}}]
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

	body := `{"chat_session_id":"sess-sv-live","guide_mode":"romantic","narrative_stance":"proactive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","context_messages":[{"role":"user","content":"move forward"}]}`
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
	director, _ := directive["director"].(map[string]any)
	if director["pressure_level"] != "normal" {
		t.Fatalf("pressure_level = %v, want normal", director["pressure_level"])
	}
	traceSummary, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object: %+v", resp)
	}
	if traceSummary["narrative_stance"] != "proactive" ||
		traceSummary["narrative_stance_suffix_present"] != true ||
		traceSummary["narrative_stance_bounds_present"] != true {
		t.Fatalf("trace_summary missing narrative stance evidence: %+v", traceSummary)
	}
	summary, ok := traceSummary["narrative_stance_summary"].(map[string]any)
	if !ok || summary["mode"] != "proactive" {
		t.Fatalf("narrative_stance_summary = %+v, want proactive object", traceSummary["narrative_stance_summary"])
	}
}
