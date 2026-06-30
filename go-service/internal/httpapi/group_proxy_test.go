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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
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

func TestSupervisorStorylineFeedbackReplayAssumedRuntimeGate(t *testing.T) {
	const sid = "sess-e1f-replay"
	const freshContext = "Fresh gate confrontation: Mira chooses whether to expose the forged seal."
	const staleContext = "Old corridor rumor repeats the same key point without new evidence."
	const suppressedContext = "Suppressed detour must not enter supervisor prompt."

	callCount := 0
	capturedPrompts := []string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		messages, _ := body["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("upstream messages missing: %+v", body)
		}
		userMsg, _ := messages[1].(map[string]any)
		prompt := extractionStringFromAny(userMsg["content"])
		capturedPrompts = append(capturedPrompts, prompt)
		callCount++

		currentArc := "baseline_continue"
		narrativeGoal := "Continue from recent chat without storyline feedback."
		requiredOutcome := "preserve scene continuity"
		if strings.Contains(prompt, freshContext) {
			currentArc = "gate_confrontation_push"
			narrativeGoal = "Advance the fresh gate confrontation without repeating the old corridor rumor."
			requiredOutcome = "advance fresh confrontation"
		}
		response := map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": `{
				"directive": {
					"story_author": {
						"current_arc": "` + currentArc + `",
						"narrative_goal": "` + narrativeGoal + `"
					},
					"director": {
						"pressure_level": "normal",
						"required_outcomes": ["` + requiredOutcome + `"],
						"forbidden_moves": ["repeat stale storyline"]
					}
				}
			}`}}},
			"model": "supervisor-replay",
			"usage": map[string]any{"total_tokens": 42},
		}
		data, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(data)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	run := func(withFeedback bool) map[string]any {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-supervisor-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "supervisor-replay",
			SupervisorTimeoutSec: 10,
		}
		if withFeedback {
			srv.Store = &turnRecordingStore{
				returnStorylines: []store.Storyline{
					{ID: 1, ChatSessionID: sid, Name: "Fresh gate confrontation", Status: "active", CurrentContext: freshContext, Confidence: 0.86, EvidenceCount: 4, LastEvidenceTurn: 14, LastTurn: 14},
					{ID: 2, ChatSessionID: sid, Name: "Old corridor rumor", Status: "active", CurrentContext: staleContext, Confidence: 0.91, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 2},
					{ID: 3, ChatSessionID: sid, Name: "Resolved apology", Status: "resolved", CurrentContext: "Resolved apology should remain summary-only.", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 6},
					{ID: 4, ChatSessionID: sid, Name: "Suppressed detour", Status: "active", CurrentContext: suppressedContext, Confidence: 1, EvidenceCount: 5, LastEvidenceTurn: 15, LastTurn: 15, Suppressed: true},
				},
			}
		}
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"` + sid + `",
			"guide_mode":"standard",
			"narrative_stance":"balanced",
			"wake_up_context":"The forged seal is in the guard captain's hand.",
			"persistent_guidance":"Avoid repeating stale hooks.",
			"context_messages":[
				{"role":"user","content":"I ask Mira whether we should expose the seal now."},
				{"role":"assistant","content":"Mira hesitates, watching the captain's expression."},
				{"role":"user","content":"Continue from here."}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode supervisor response: %v", err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("supervisor did not use runtime LLM path: %+v", resp)
		}
		return resp
	}

	offResp := run(false)
	onResp := run(true)
	onResp2 := run(true)
	onResp3 := run(true)
	if callCount != 4 || len(capturedPrompts) != 4 {
		t.Fatalf("runtime replay calls = %d prompts = %d, want 4/4", callCount, len(capturedPrompts))
	}
	if strings.Contains(capturedPrompts[0], freshContext) {
		t.Fatalf("feedback-off prompt should not include storyline context: %s", capturedPrompts[0])
	}
	for i, prompt := range capturedPrompts[1:] {
		if !strings.Contains(prompt, freshContext) {
			t.Fatalf("feedback-on replay %d prompt missing fresh storyline context: %s", i+1, prompt)
		}
		for _, forbidden := range []string{staleContext, suppressedContext, "Resolved apology should remain summary-only."} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("feedback-on replay %d prompt contains stale/resolved/suppressed storyline %q: %s", i+1, forbidden, prompt)
			}
		}
	}

	onPack := onResp["supervisor_input_pack"].(map[string]any)
	selection := onPack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) || selection["suppressed_count"] != float64(1) || selection["resolved_summary_count"] != float64(1) {
		t.Fatalf("unexpected storyline selection summary: %+v", selection)
	}

	offArc := supervisorCurrentArc(offResp)
	onArc := supervisorCurrentArc(onResp)
	if offArc != "baseline_continue" || onArc != "gate_confrontation_push" {
		t.Fatalf("current_arc off/on = %q/%q, want baseline_continue/gate_confrontation_push", offArc, onArc)
	}
	for i, resp := range []map[string]any{onResp2, onResp3} {
		if arc := supervisorCurrentArc(resp); arc != onArc {
			t.Fatalf("feedback-on replay %d current_arc = %q, want stable %q", i+2, arc, onArc)
		}
	}
	onDirector := supervisorDirector(onResp)
	required, _ := onDirector["required_outcomes"].([]any)
	if len(required) == 0 || extractionStringFromAny(required[0]) != "advance fresh confrontation" {
		t.Fatalf("director.required_outcomes = %+v, want advance fresh confrontation", onDirector["required_outcomes"])
	}
	forbidden, _ := onDirector["forbidden_moves"].([]any)
	if len(forbidden) == 0 || !strings.Contains(extractionStringFromAny(forbidden[0]), "stale") {
		t.Fatalf("director.forbidden_moves = %+v, want stale-repeat guard", onDirector["forbidden_moves"])
	}
}

func TestNarrativeGuideModesControlledReplayDiverges(t *testing.T) {
	type modeCase struct {
		mode             string
		suffixNeedle     string
		emphasisNeedle   string
		forbiddenNeedle  string
		expectedArc      string
		expectedResponse string
	}
	cases := []modeCase{
		{mode: "off", expectedArc: "baseline_arc", expectedResponse: "baseline continuation"},
		{mode: "romantic", suffixNeedle: "Romantic", emphasisNeedle: "emotional resonance", forbiddenNeedle: "trivializing emotional moments", expectedArc: "romantic_arc", expectedResponse: "romantic emotional beat"},
		{mode: "action", suffixNeedle: "Action", emphasisNeedle: "combat choreography", forbiddenNeedle: "excessive monologuing during action", expectedArc: "action_arc", expectedResponse: "action forward motion"},
		{mode: "mature_soft", suffixNeedle: "Mature (Sensual)", emphasisNeedle: "sensory atmosphere", forbiddenNeedle: "ignoring character consent", expectedArc: "mature_soft_arc", expectedResponse: "sensual consent-aware beat"},
	}

	callByMode := map[string]int{}
	capturedPromptByMode := map[string]string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &reqBody); err != nil {
			t.Fatalf("decode guide replay proxy body: %v; raw=%s", err, raw)
		}
		messages, _ := reqBody["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("guide replay proxy body missing messages: %+v", reqBody)
		}
		userMessage, _ := messages[1].(map[string]any)
		body := extractionStringFromAny(userMessage["content"])
		mode := "off"
		for _, candidate := range []string{"romantic", "action", "mature_soft"} {
			if strings.Contains(body, `"guide_mode": "`+candidate+`"`) {
				mode = candidate
				break
			}
		}
		callByMode[mode]++
		capturedPromptByMode[mode] = body
		arc := map[string]string{
			"off":         "baseline_arc",
			"romantic":    "romantic_arc",
			"action":      "action_arc",
			"mature_soft": "mature_soft_arc",
		}[mode]
		responseText := map[string]string{
			"off":         "baseline continuation",
			"romantic":    "romantic emotional beat",
			"action":      "action forward motion",
			"mature_soft": "sensual consent-aware beat",
		}[mode]
		content := `{"directive":{"story_author":{"current_arc":"` + arc + `","narrative_goal":"` + responseText + `"},"director":{"pressure_level":"normal","required_outcomes":["` + responseText + `"],"forbidden_moves":["mode-specific guard"]}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"guide-replay","choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	results := map[string]map[string]any{}
	for _, tc := range cases {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-guide-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "guide-replay",
			SupervisorTimeoutSec: 10,
		}
		srv.RegisterRoutes(mux)
		body := `{
			"chat_session_id":"sess-guide-effect",
			"guide_mode":"` + tc.mode + `",
			"narrative_stance":"balanced",
			"auto_advance_trigger":"none",
			"wake_up_context":"Same scene: Chloe faces the locked archive door.",
			"persistent_guidance":"Use the requested narrative mode without changing the factual scene.",
			"context_messages":[{"role":"user","content":"Continue the same scene from here."}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s supervisor status = %d, want 200: %s", tc.mode, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s decode response: %v", tc.mode, err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("%s did not use runtime supervisor path: %+v", tc.mode, resp)
		}
		results[tc.mode] = resp

		pack := resp["supervisor_input_pack"].(map[string]any)
		if pack["guide_mode"] != tc.mode {
			t.Fatalf("%s pack guide_mode = %v", tc.mode, pack["guide_mode"])
		}
		trace := resp["trace_summary"].(map[string]any)
		if trace["guide_mode"] != tc.mode {
			t.Fatalf("%s trace guide_mode = %v", tc.mode, trace["guide_mode"])
		}
		suffix, _ := pack["guide_suffix"].(string)
		directorOverrides := pack["director_overrides"].(map[string]any)
		emphasis, _ := directorOverrides["emphasis"].([]any)
		forbidden, _ := directorOverrides["forbidden_moves"].([]any)
		if tc.mode == "off" {
			if suffix != "" || len(emphasis) != 0 || len(forbidden) != 0 || trace["guide_suffix_present"] != false {
				t.Fatalf("off mode should not add suffix/overrides, suffix=%q emphasis=%+v forbidden=%+v trace=%+v", suffix, emphasis, forbidden, trace["guide_suffix_present"])
			}
		} else {
			if !strings.Contains(suffix, tc.suffixNeedle) || trace["guide_suffix_present"] != true {
				t.Fatalf("%s suffix/trace mismatch: suffix=%q trace=%+v", tc.mode, suffix, trace["guide_suffix_present"])
			}
			if !anySliceContains(emphasis, tc.emphasisNeedle) {
				t.Fatalf("%s emphasis missing %q: %+v", tc.mode, tc.emphasisNeedle, emphasis)
			}
			if !anySliceContains(forbidden, tc.forbiddenNeedle) {
				t.Fatalf("%s forbidden_moves missing %q: %+v", tc.mode, tc.forbiddenNeedle, forbidden)
			}
			if !strings.Contains(capturedPromptByMode[tc.mode], tc.suffixNeedle) || !strings.Contains(capturedPromptByMode[tc.mode], tc.emphasisNeedle) {
				t.Fatalf("%s upstream prompt missing suffix/emphasis: %s", tc.mode, capturedPromptByMode[tc.mode])
			}
		}
		if arc := supervisorCurrentArc(resp); arc != tc.expectedArc {
			t.Fatalf("%s current_arc = %q, want %q", tc.mode, arc, tc.expectedArc)
		}
		director := supervisorDirector(resp)
		outcomes, _ := director["required_outcomes"].([]any)
		if len(outcomes) == 0 || !strings.Contains(extractionStringFromAny(outcomes[0]), tc.expectedResponse) {
			t.Fatalf("%s required_outcomes = %+v, want %q", tc.mode, outcomes, tc.expectedResponse)
		}
	}
	if len(callByMode) != len(cases) {
		t.Fatalf("runtime calls by mode = %+v, want all modes", callByMode)
	}
	if supervisorCurrentArc(results["off"]) == supervisorCurrentArc(results["romantic"]) ||
		supervisorCurrentArc(results["romantic"]) == supervisorCurrentArc(results["action"]) ||
		supervisorCurrentArc(results["action"]) == supervisorCurrentArc(results["mature_soft"]) {
		t.Fatalf("guide mode arcs should diverge: off=%s romantic=%s action=%s mature=%s",
			supervisorCurrentArc(results["off"]),
			supervisorCurrentArc(results["romantic"]),
			supervisorCurrentArc(results["action"]),
			supervisorCurrentArc(results["mature_soft"]))
	}
}

func TestNarrativeStanceModesControlledReplayDiverges(t *testing.T) {
	type stanceCase struct {
		mode          string
		suffixNeedle  string
		expectedBeats any
		expectedArc   string
		expectedGoal  string
	}
	cases := []stanceCase{
		{mode: "reactive", suffixNeedle: "Story Initiative - Reactive", expectedBeats: float64(0), expectedArc: "reactive_hold", expectedGoal: "hold the current beat and avoid new hooks"},
		{mode: "balanced", suffixNeedle: "Story Initiative - Balanced", expectedBeats: float64(1), expectedArc: "balanced_follow", expectedGoal: "advance one grounded beat"},
		{mode: "proactive", suffixNeedle: "Story Initiative - Proactive", expectedBeats: float64(1), expectedArc: "proactive_push", expectedGoal: "introduce a grounded follow-up hook"},
	}

	callByMode := map[string]int{}
	capturedPromptByMode := map[string]string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &reqBody); err != nil {
			t.Fatalf("decode stance replay proxy body: %v; raw=%s", err, raw)
		}
		messages, _ := reqBody["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("stance replay proxy body missing messages: %+v", reqBody)
		}
		userMessage, _ := messages[1].(map[string]any)
		body := extractionStringFromAny(userMessage["content"])
		mode := "balanced"
		for _, candidate := range []string{"reactive", "balanced", "proactive"} {
			if strings.Contains(body, `"narrative_stance": "`+candidate+`"`) {
				mode = candidate
				break
			}
		}
		callByMode[mode]++
		capturedPromptByMode[mode] = body
		arc := map[string]string{
			"reactive":  "reactive_hold",
			"balanced":  "balanced_follow",
			"proactive": "proactive_push",
		}[mode]
		goal := map[string]string{
			"reactive":  "hold the current beat and avoid new hooks",
			"balanced":  "advance one grounded beat",
			"proactive": "introduce a grounded follow-up hook",
		}[mode]
		content := `{"directive":{"story_author":{"current_arc":"` + arc + `","narrative_goal":"` + goal + `"},"director":{"pressure_level":"normal","required_outcomes":["` + goal + `"],"forbidden_moves":["stance-specific guard"]}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"stance-replay","choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	results := map[string]map[string]any{}
	for _, tc := range cases {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-stance-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "stance-replay",
			SupervisorTimeoutSec: 10,
		}
		srv.RegisterRoutes(mux)
		body := `{
			"chat_session_id":"sess-stance-effect",
			"guide_mode":"off",
			"narrative_stance":"` + tc.mode + `",
			"auto_advance_trigger":"none",
			"wake_up_context":"Same scene: Chloe pauses at the archive door.",
			"persistent_guidance":"Use the requested initiative mode without changing the factual scene.",
			"context_messages":[{"role":"user","content":"Continue the same scene from here."}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s supervisor status = %d, want 200: %s", tc.mode, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s decode response: %v", tc.mode, err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("%s did not use runtime supervisor path: %+v", tc.mode, resp)
		}
		results[tc.mode] = resp
		pack := resp["supervisor_input_pack"].(map[string]any)
		if pack["narrative_stance"] != tc.mode {
			t.Fatalf("%s pack narrative_stance = %v", tc.mode, pack["narrative_stance"])
		}
		suffix := extractionStringFromAny(pack["narrative_stance_suffix"])
		if !strings.Contains(suffix, tc.suffixNeedle) || !strings.Contains(capturedPromptByMode[tc.mode], tc.suffixNeedle) {
			t.Fatalf("%s prompt/suffix missing %q: suffix=%q prompt=%s", tc.mode, tc.suffixNeedle, suffix, capturedPromptByMode[tc.mode])
		}
		bounds, _ := pack["narrative_stance_bounds"].(map[string]any)
		if bounds["max_new_beats"] != tc.expectedBeats {
			t.Fatalf("%s max_new_beats = %v, want %v in bounds %+v", tc.mode, bounds["max_new_beats"], tc.expectedBeats, bounds)
		}
		trace := resp["trace_summary"].(map[string]any)
		if trace["narrative_stance"] != tc.mode || trace["narrative_stance_suffix_present"] != true || trace["narrative_stance_bounds_present"] != true {
			t.Fatalf("%s trace missing stance evidence: %+v", tc.mode, trace)
		}
		if arc := supervisorCurrentArc(resp); arc != tc.expectedArc {
			t.Fatalf("%s current_arc = %q, want %q", tc.mode, arc, tc.expectedArc)
		}
		director := supervisorDirector(resp)
		outcomes, _ := director["required_outcomes"].([]any)
		if len(outcomes) == 0 || !strings.Contains(extractionStringFromAny(outcomes[0]), tc.expectedGoal) {
			t.Fatalf("%s required_outcomes = %+v, want %q", tc.mode, outcomes, tc.expectedGoal)
		}
	}
	if len(callByMode) != len(cases) {
		t.Fatalf("runtime calls by stance = %+v, want all stances", callByMode)
	}
	if supervisorCurrentArc(results["reactive"]) == supervisorCurrentArc(results["balanced"]) ||
		supervisorCurrentArc(results["balanced"]) == supervisorCurrentArc(results["proactive"]) {
		t.Fatalf("narrative stance arcs should diverge: reactive=%s balanced=%s proactive=%s",
			supervisorCurrentArc(results["reactive"]),
			supervisorCurrentArc(results["balanced"]),
			supervisorCurrentArc(results["proactive"]))
	}
}

func anySliceContains(values []any, needle string) bool {
	for _, value := range values {
		if strings.Contains(extractionStringFromAny(value), needle) {
			return true
		}
	}
	return false
}

func supervisorCurrentArc(resp map[string]any) string {
	result, _ := resp["supervisor_result"].(map[string]any)
	directive, _ := result["directive"].(map[string]any)
	author, _ := directive["story_author"].(map[string]any)
	return extractionStringFromAny(author["current_arc"])
}

func supervisorDirector(resp map[string]any) map[string]any {
	result, _ := resp["supervisor_result"].(map[string]any)
	directive, _ := result["directive"].(map[string]any)
	director, _ := directive["director"].(map[string]any)
	return director
}

func TestConfigUpdateProjectGUISettingsTraceMasksSecrets(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	const mainKey = "sk-main-secret-seq02"
	const criticKey = "sk-critic-secret-seq02"
	const embeddingKey = "sk-embedding-secret-seq02"
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"ollama",
		"mainApiKey":"`+mainKey+`",
		"mainEndpoint":"http://127.0.0.1:11434/v1",
		"mainModel":"glm-5.1:cloud",
		"mainTimeout":61,
		"mainTemperature":0.65,
		"mainMaxCompletionTokens":2048,
		"mainReasoningPreset":"glm",
		"mainReasoningEffort":"enable",
		"mainReasoningBudgetTokens":4096,
		"criticProvider":"ollama",
		"criticApiKey":"`+criticKey+`",
		"criticEndpoint":"http://127.0.0.1:11434/v1",
		"criticModel":"glm-5.1:cloud",
		"criticTimeout":62,
		"criticTemperature":0.21,
		"criticMaxCompletionTokens":1536,
		"criticReasoningPreset":"custom",
		"criticReasoningEffort":"high",
		"criticReasoningBudgetTokens":2048,
		"supervisorProvider":"ollama",
		"supervisorApiKey":"`+mainKey+`",
		"supervisorEndpoint":"http://127.0.0.1:11434/v1",
		"supervisorModel":"glm-5.1:cloud",
		"supervisorTimeout":63,
		"supervisorTemperature":0.65,
		"supervisorMaxCompletionTokens":2048,
		"supervisorReasoningPreset":"glm",
		"supervisorReasoningEffort":"enable",
		"supervisorReasoningBudgetTokens":4096,
		"embeddingProvider":"ollama",
		"embeddingApiKey":"`+embeddingKey+`",
		"embeddingEndpoint":"http://127.0.0.1:11434",
		"embeddingModel":"nomic-embed-text",
		"embeddingTimeout":64,
		"topK":7
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}
	body := updateRec.Body.String()
	for _, secret := range []string{mainKey, criticKey, embeddingKey} {
		if strings.Contains(body, secret) {
			t.Fatalf("config/update response leaked secret %q: %s", secret, body)
		}
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update response: %+v", updateResp)
	}
	if trace["top_k"] != float64(7) {
		t.Fatalf("runtime_config_trace.top_k = %v, want 7", trace["top_k"])
	}
	mainTrace, ok := trace["main"].(map[string]any)
	if !ok {
		t.Fatalf("main trace missing: %+v", trace)
	}
	if mainTrace["provider"] != "ollama" || mainTrace["endpoint_host"] != "127.0.0.1:11434" || mainTrace["model"] != "glm-5.1:cloud" {
		t.Fatalf("main trace did not reflect GUI settings: %+v", mainTrace)
	}
	if mainTrace["config_authority"] != "runtime_config" || mainTrace["model_source"] != "runtime.mainModel" || mainTrace["provider_source"] != "runtime.mainProvider" {
		t.Fatalf("main trace did not expose runtime UI authority/source: %+v", mainTrace)
	}
	if mainTrace["temperature"] != float64(0.65) || mainTrace["max_completion_tokens"] != float64(2048) {
		t.Fatalf("main trace did not reflect generation settings: %+v", mainTrace)
	}
	if mainTrace["reasoning_preset"] != "glm" || mainTrace["reasoning_effort"] != "enable" || mainTrace["reasoning_budget_tokens"] != float64(4096) || mainTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("main trace did not reflect reasoning settings: %+v", mainTrace)
	}
	criticTrace, ok := trace["critic"].(map[string]any)
	if !ok {
		t.Fatalf("critic trace missing: %+v", trace)
	}
	if criticTrace["provider"] != "ollama" || criticTrace["temperature"] != float64(0.21) || criticTrace["max_completion_tokens"] != float64(1536) {
		t.Fatalf("critic trace did not reflect GUI settings: %+v", criticTrace)
	}
	if criticTrace["config_authority"] != "runtime_config" || criticTrace["model_source"] != "runtime.criticModel" || criticTrace["provider_source"] != "runtime.criticProvider" {
		t.Fatalf("critic trace did not expose runtime UI authority/source: %+v", criticTrace)
	}
	if criticTrace["reasoning_preset"] != "custom" || criticTrace["reasoning_effort"] != "high" || criticTrace["reasoning_budget_tokens"] != float64(2048) {
		t.Fatalf("critic trace did not reflect reasoning settings: %+v", criticTrace)
	}
	supervisorTrace, ok := trace["supervisor"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor trace missing: %+v", trace)
	}
	if supervisorTrace["reasoning_preset"] != "glm" || supervisorTrace["reasoning_effort"] != "enable" || supervisorTrace["reasoning_budget_tokens"] != float64(4096) || supervisorTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("supervisor trace did not reflect reasoning settings: %+v", supervisorTrace)
	}
	if supervisorTrace["config_authority"] != "runtime_config" || supervisorTrace["model_source"] != "runtime.supervisorModel" || supervisorTrace["provider_source"] != "runtime.supervisorProvider" {
		t.Fatalf("supervisor trace did not expose runtime UI authority/source: %+v", supervisorTrace)
	}
	embeddingTrace, ok := trace["embedding"].(map[string]any)
	if !ok {
		t.Fatalf("embedding trace missing: %+v", trace)
	}
	if embeddingTrace["provider"] != "ollama" || embeddingTrace["endpoint_host"] != "127.0.0.1:11434" || embeddingTrace["model"] != "nomic-embed-text" {
		t.Fatalf("embedding trace did not reflect GUI settings: %+v", embeddingTrace)
	}
	if embeddingTrace["config_authority"] != "runtime_config" || embeddingTrace["model_source"] != "runtime.embeddingModel" || embeddingTrace["provider_source"] != "runtime.embeddingProvider" {
		t.Fatalf("embedding trace did not expose runtime UI authority/source: %+v", embeddingTrace)
	}

	cfg := srv.supervisorLLMConfig()
	if cfg.Provider != "ollama" || cfg.Temperature != 0.65 || cfg.MaxTokens != 2048 {
		t.Fatalf("supervisor runtime config = provider %q temp %v max %d, want ollama/0.65/2048", cfg.Provider, cfg.Temperature, cfg.MaxTokens)
	}
	if cfg.ReasoningPreset != "glm" || cfg.ReasoningEffort != "enable" || cfg.ReasoningBudgetTokens != 4096 {
		t.Fatalf("supervisor reasoning config = preset %q effort %q budget %d, want glm/enable/4096", cfg.ReasoningPreset, cfg.ReasoningEffort, cfg.ReasoningBudgetTokens)
	}
}

func TestConfigUpdateSupervisorTraceDoesNotInferMainConfig(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"openai",
		"mainApiKey":"sk-main",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"main-runtime-model",
		"supervisorTimeout":30
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace := updateResp["runtime_config_trace"].(map[string]any)
	supervisorTrace := trace["supervisor"].(map[string]any)
	if supervisorTrace["configured"] != false {
		t.Fatalf("supervisor configured = %v, want false when supervisor fields are empty: %+v", supervisorTrace["configured"], supervisorTrace)
	}
	if supervisorTrace["model"] != "" || supervisorTrace["endpoint_host"] != "" {
		t.Fatalf("supervisor trace inferred main values: %+v", supervisorTrace)
	}
	if supervisorTrace["model_source"] != "unset" || supervisorTrace["api_key_source"] != "unset" || supervisorTrace["endpoint_source"] != "unset" {
		t.Fatalf("supervisor trace should mark empty runtime fields as unset: %+v", supervisorTrace)
	}
	missing, ok := supervisorTrace["missing_fields"].([]any)
	if !ok || len(missing) != 4 {
		t.Fatalf("supervisor missing_fields = %#v, want provider/api_key/endpoint/model", supervisorTrace["missing_fields"])
	}
}

func TestChapterLLMConfigDoesNotDefaultProvider(t *testing.T) {
	srv := setupTestServer()
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.MainAPIKey = "sk-main"
	srv.RuntimeConfig.MainEndpoint = "https://api.example.com/v1"
	srv.RuntimeConfig.MainModel = "chapter-model"

	cfg := srv.chapterLLMConfig()
	if cfg.Provider != "" {
		t.Fatalf("chapter provider = %q, want empty when runtime main provider is empty", cfg.Provider)
	}
	if cfg.hasConfig() {
		t.Fatalf("chapter config hasConfig=true, want false when provider is empty")
	}
	missing := cfg.missingFields()
	foundProvider := false
	for _, field := range missing {
		if field == "provider" {
			foundProvider = true
		}
	}
	if !foundProvider {
		t.Fatalf("chapter missing fields = %v, want provider", missing)
	}
}

func TestHandleSupervisorFailOpenOnRuntimeLLMError(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	const apiKey = "sk-supervisor-fail"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key sk-supervisor-fail"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainApiKey":"`+apiKey+`",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"supervisor-model",
		"mainProvider":"openai",
		"supervisorProvider":"openai",
		"supervisorApiKey":"`+apiKey+`",
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

	body := `{"chat_session_id":"sess-sv-fail","guide_mode":"strict","narrative_stance":"immersive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","context_messages":[{"role":"user","content":"move forward"}]}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), apiKey) {
		t.Fatalf("supervisor fail-open response leaked API key: %s", rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["source"] != "runtime_llm_error" || resp["fail_open"] != true || resp["would_call_llm"] != true {
		t.Fatalf("unexpected fail-open response: %+v", resp)
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary missing: %+v", resp)
	}
	if trace["llm_call"] != "failed" || trace["fail_open"] != true {
		t.Fatalf("trace did not expose failed fail-open call: %+v", trace)
	}
}

func TestHandleSupervisorMissingChatSessionID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"","guide_mode":"strict"}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "missing_param" {
		t.Errorf("code = %v, want missing_param", resp["code"])
	}
}

func TestHandleCriticTestPromptAssemblyTrace(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "critic_system.txt"), []byte("critic system prompt"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "critic_prompt.txt"), []byte("critic prompt content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.PromptDir = tmpDir
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-critic2","turn_index":3,"turn_content":"test content","context":[{"role":"user"}],"output_language_override":{"language":"ko"}}`
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

	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["prompt_source"] != "configured" {
		t.Errorf("trace.prompt_source = %v, want configured", trace["prompt_source"])
	}
	if trace["files_found"] != float64(2) {
		t.Errorf("trace.files_found = %v, want 2", trace["files_found"])
	}
	if trace["llm_call"] != "disabled" {
		t.Errorf("trace.llm_call = %v, want disabled", trace["llm_call"])
	}
	if trace["verdict"] != "not_executed" {
		t.Errorf("trace.verdict = %v, want not_executed", trace["verdict"])
	}
	if trace["turn_content_chars"] != float64(12) { // len([]rune("test content"))
		t.Errorf("trace.turn_content_chars = %v, want 12", trace["turn_content_chars"])
	}
	pack, ok := resp["critic_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("critic_input_pack is not an object")
	}
	if pack["prompt_source"] != "configured" {
		t.Errorf("critic_input_pack.prompt_source = %v, want configured", pack["prompt_source"])
	}
	if pack["would_call_llm"] != false {
		t.Errorf("critic_input_pack.would_call_llm = %v, want false", pack["would_call_llm"])
	}
}
