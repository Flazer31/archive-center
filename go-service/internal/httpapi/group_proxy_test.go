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
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func strPtr(v string) *string {
	return &v
}

func TestProxyHTTPFailureMetadataKeepsLongestRetryAfterHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(524)
		_, _ = w.Write([]byte(`{"retry_after":120,"detail":"origin timeout"}`))
	}))
	defer server.Close()

	status, data, raw, err := proxyDoJSON(context.Background(), server.URL, nil, map[string]any{"test": true})
	if err != nil || status != 524 || !strings.Contains(raw, "origin timeout") {
		t.Fatalf("status=%d raw=%q err=%v", status, raw, err)
	}
	meta := mapFromAny(data[proxyResponseMetadataKey])
	if got := intFromAny(meta["retry_after_seconds"], 0); got != 120 {
		t.Fatalf("retry_after_seconds=%d meta=%+v", got, meta)
	}
	provider := "custom"
	endpoint := server.URL
	apiKey := "test-key"
	model := "test-model"
	upstream, upstreamStatus, upstreamErr := performProxyPluginMainWithRetryBudget(
		context.Background(),
		dto.ProxyPluginMainRequest{
			Provider: &provider, Endpoint: &endpoint, APIKey: &apiKey, Model: &model,
			Messages: []any{map[string]any{"role": "user", "content": "test"}},
		},
		nil,
	)
	if upstreamErr == nil || upstreamStatus != 524 {
		t.Fatalf("upstream status=%d err=%v response=%+v", upstreamStatus, upstreamErr, upstream)
	}
	if got := intFromAny(mapFromAny(upstream[proxyResponseMetadataKey])["retry_after_seconds"], 0); got != 120 {
		t.Fatalf("provider retry hint was discarded: got=%d response=%+v", got, upstream)
	}
}

func TestProxyHTTPFailureMetadataIgnoresOnlyMalformedRetryHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"retry_after":"later","detail":"rate limited"}`))
	}))
	defer server.Close()

	status, data, raw, err := proxyDoJSON(context.Background(), server.URL, nil, map[string]any{"test": true})
	if err != nil || status != http.StatusTooManyRequests || !strings.Contains(raw, "rate limited") {
		t.Fatalf("status=%d raw=%q err=%v", status, raw, err)
	}
	if got := intFromAny(mapFromAny(data[proxyResponseMetadataKey])["retry_after_seconds"], 0); got != 0 {
		t.Fatalf("malformed hint should be ignored, got=%d data=%+v", got, data)
	}
	if extractionStringFromAny(data["detail"]) != "rate limited" {
		t.Fatalf("valid error body was discarded: %+v", data)
	}
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

func TestProxyProviderBaseURLDefaults(t *testing.T) {
	wants := map[string]string{
		"openai":     "https://api.openai.com/v1",
		"openrouter": "https://openrouter.ai/api/v1",
		"llmgateway": "https://api.llmgateway.io/v1",
		"vercel":     "https://ai-gateway.vercel.sh/v1",
		"neuralwatt": "https://api.neuralwatt.com/v1",
		"copilot":    "https://api.githubcopilot.com",
		"ollama":     "http://127.0.0.1:11434",
		"claude":     "https://api.anthropic.com",
		"gemini":     "https://generativelanguage.googleapis.com/v1beta",
		"vertex":     "https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models",
	}
	for provider, want := range wants {
		if got := proxyProviderBaseURL(provider, ""); got != want {
			t.Errorf("provider %s default = %q, want %q", provider, got, want)
		}
	}
	if got := proxyProviderBaseURL("custom", ""); got != "" {
		t.Fatalf("custom blank endpoint = %q, want empty", got)
	}
	if got := proxyProviderBaseURL("neuralwatt", "https://override.example/v1/"); got != "https://override.example/v1" {
		t.Fatalf("explicit endpoint override = %q", got)
	}
}

func TestProxyBlankEndpointUsesNamedProviderDefaultAndHonorsOverride(t *testing.T) {
	t.Run("neuralwatt default", func(t *testing.T) {
		oldClient := proxyHTTPClient
		defer func() { proxyHTTPClient = oldClient }()
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.String(); got != "https://api.neuralwatt.com/v1/chat/completions" {
				t.Fatalf("default NeuralWatt URL = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)),
			}, nil
		})}

		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:   strPtr("nw-test"),
			Endpoint: strPtr(""),
			Model:    strPtr("test/model"),
			Provider: strPtr("neuralwatt"),
			Messages: []any{map[string]any{"role": "user", "content": "ping"}},
		})
		if err != nil || status != http.StatusOK {
			t.Fatalf("blank NeuralWatt endpoint status=%d err=%v", status, err)
		}
	})

	t.Run("explicit override", func(t *testing.T) {
		oldClient := proxyHTTPClient
		defer func() { proxyHTTPClient = oldClient }()
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.String(); got != "https://gateway.example.test/v1/chat/completions" {
				t.Fatalf("override URL = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)),
			}, nil
		})}

		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:   strPtr("openai-test"),
			Endpoint: strPtr("https://gateway.example.test/v1"),
			Model:    strPtr("gpt-test"),
			Provider: strPtr("openai"),
			Messages: []any{map[string]any{"role": "user", "content": "ping"}},
		})
		if err != nil || status != http.StatusOK {
			t.Fatalf("explicit endpoint status=%d err=%v", status, err)
		}
	})
}

func TestProxyBlankVertexEndpointUsesServiceAccountProjectAndGlobalLocation(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()
	credential := testVertexServiceAccountJSON(t)
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
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`)),
			}, nil
		default:
			t.Fatalf("unexpected Vertex URL: %s", r.URL.String())
			return nil, nil
		}
	})}

	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   &credential,
		Endpoint: strPtr(""),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("vertex"),
		Messages: []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("blank Vertex endpoint status=%d err=%v", status, err)
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

func TestProxyClaudeReasoningOnlyMaxTokensIsOutputTokenExhausted(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()

	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.anthropic.example/v1/messages" {
			t.Fatalf("upstream URL = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"stop_reason":"max_tokens",
				"content":[{"type":"thinking","thinking":"reasoning only","text":"{\"turn_summary\":\"must not become final\"}"}],
				"usage":{"input_tokens":40,"output_tokens":128}
			}`)),
		}, nil
	})}

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("anthropic-key"),
		Endpoint: strPtr("https://api.anthropic.example"),
		Model:    strPtr("claude-test"),
		Provider: strPtr("claude"),
		Messages: []any{map[string]any{"role": "user", "content": "return JSON"}},
	})
	var exhaustedErr *proxyFinalOutputExhaustedError
	if !errors.As(err, &exhaustedErr) || status != http.StatusOK {
		t.Fatalf("Claude reasoning-only response status=%d err=%T %v", status, err, err)
	}
	if got := chatCompletionText(resp); got != "" {
		t.Fatalf("Claude thinking block became final text: %q", got)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["native_finish_reason"] != "max_tokens" || metadata["termination_kind"] != "length" || intFromAny(metadata["output_tokens"], 0) != 128 {
		t.Fatalf("Claude termination metadata=%#v", metadata)
	}
}

func TestProxyGeminiReasoningOnlyMaxTokensIsOutputTokenExhausted(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()

	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"thought":true,"text":"{\"turn_summary\":\"must not become final\"}"}]}}],
				"usageMetadata":{"promptTokenCount":80,"candidatesTokenCount":16,"thoughtsTokenCount":16,"totalTokenCount":96}
			}`)),
		}, nil
	})}

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("gemini-key"),
		Endpoint: strPtr("https://generativelanguage.googleapis.com/v1beta"),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("gemini"),
		Messages: []any{map[string]any{"role": "user", "content": "return JSON"}},
	})
	var exhaustedErr *proxyFinalOutputExhaustedError
	if !errors.As(err, &exhaustedErr) || status != http.StatusOK {
		t.Fatalf("Gemini reasoning-only response status=%d err=%T %v", status, err, err)
	}
	if got := chatCompletionText(resp); got != "" {
		t.Fatalf("Gemini thought part became final text: %q", got)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["native_finish_reason"] != "MAX_TOKENS" || metadata["termination_kind"] != "length" || intFromAny(metadata["reasoning_tokens"], 0) != 16 {
		t.Fatalf("Gemini termination metadata=%#v", metadata)
	}
}

func TestProxyVertexReasoningOnlyMaxTokensIsOutputTokenExhausted(t *testing.T) {
	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()

	credential := testVertexServiceAccountJSON(t)
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
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"thought":true,"text":"reasoning only"}]}}],
					"usageMetadata":{"promptTokenCount":60,"candidatesTokenCount":12,"thoughtsTokenCount":12,"totalTokenCount":72}
				}`)),
			}, nil
		default:
			t.Fatalf("unexpected request URL: %s", r.URL.String())
			return nil, nil
		}
	})}

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   &credential,
		Endpoint: strPtr("https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"),
		Model:    strPtr("gemini-test"),
		Provider: strPtr("vertex"),
		Messages: []any{map[string]any{"role": "user", "content": "return JSON"}},
	})
	var exhaustedErr *proxyFinalOutputExhaustedError
	if !errors.As(err, &exhaustedErr) || exhaustedErr.Provider != "vertex" || status != http.StatusOK {
		t.Fatalf("Vertex reasoning-only response status=%d err=%T %v", status, err, err)
	}
	if got := chatCompletionText(resp); got != "" {
		t.Fatalf("Vertex thought part became final text: %q", got)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["native_finish_reason"] != "MAX_TOKENS" || metadata["termination_kind"] != "length" || intFromAny(metadata["reasoning_tokens"], 0) != 12 {
		t.Fatalf("Vertex termination metadata=%#v", metadata)
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

	t.Run("custom missing endpoint", func(t *testing.T) {
		_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
			Provider: strPtr("custom"),
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

func TestHandleProxyPluginMainRejectsEmptyOpenAIText(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"deepseek-test","choices":[{"message":{"content":"","reasoning_content":"tokens were consumed before a final answer"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"openai","endpoint":"https://api.example.com/v1","model":"deepseek-test","api_key":"sk-test","messages":[{"role":"user","content":"reply with a test token"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "error" || !strings.Contains(stringFromAny(resp["error"]), "returned no text content") {
		t.Fatalf("empty response was not rejected: %#v", resp)
	}
}

func TestHandleProxyPluginMainCriticConnectionTestReportsFinalOutputExhaustion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if intFromAny(body["max_tokens"], 0) != 1024 {
			t.Fatalf("connection test max_tokens=%v, want 1024", body["max_tokens"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"deepseek-test",
				"choices":[{"finish_reason":"length","message":{"content":null,"reasoning_content":"reasoning consumed the budget"}}],
				"usage":{"prompt_tokens":12,"completion_tokens":1024,"total_tokens":1036,"completion_tokens_details":{"reasoning_tokens":1024}}
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"custom","endpoint":"https://api.example.com/v1","model":"deepseek-test","api_key":"sk-test","max_tokens":5,"max_completion_tokens":5,"reasoning_budget_tokens":4096,"messages":[{"role":"user","content":"Reply with exactly: OK"}]}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main?connection_test=critic", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want diagnostic 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "incomplete" || resp["code"] != "final_output_token_exhausted" || resp["connection_ok"] != true || resp["final_output_ok"] != false {
		t.Fatalf("connection-test classification=%#v", resp)
	}
	metadata := mapFromAny(resp["provider_response"])
	if metadata["native_finish_reason"] != "length" || metadata["termination_kind"] != "length" || metadata["reasoning_observed"] != true || intFromAny(metadata["reasoning_tokens"], 0) != 1024 {
		t.Fatalf("connection-test metadata=%#v", metadata)
	}
	if strings.TrimSpace(stringFromAny(resp["final_text"])) != "" {
		t.Fatalf("reasoning-only response became final text: %#v", resp["final_text"])
	}
}

func TestHandleProxyPluginMainCriticConnectionTestDoesNotUseCompatibilityRetry(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RuntimeConfigMu.Lock()
	srv.RuntimeConfig.LLMRetryCount = 3
	srv.RuntimeConfigMu.Unlock()
	srv.RegisterRoutes(mux)

	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported parameter: max_completion_tokens"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	body := `{"provider":"custom","endpoint":"https://api.example.com/v1","model":"custom-model","api_key":"sk-test","messages":[{"role":"user","content":"ping"}],"extra_body_json":"{\"max_completion_tokens\":256}"}`
	req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main?connection_test=critic", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want diagnostic 200: %s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("connection test calls=%d, want exactly one", calls)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "error" || resp["connection_ok"] != false {
		t.Fatalf("unexpected retry-free diagnostic=%#v", resp)
	}
}

func TestChatCompletionTextSupportsTextBlocksAndLegacyTextWithoutUsingReasoning(t *testing.T) {
	arrayResponse := map[string]any{"choices": []any{map[string]any{
		"message": map[string]any{"content": []any{
			map[string]any{"type": "reasoning", "text": "do not expose"},
			map[string]any{"type": "text", "text": "part one"},
			map[string]any{"type": "output_text", "text": " and two"},
		}},
	}}}
	if got := chatCompletionText(arrayResponse); got != "part one and two" {
		t.Fatalf("array content=%q", got)
	}
	legacyResponse := map[string]any{"choices": []any{map[string]any{"text": "legacy final text"}}}
	if got := chatCompletionText(legacyResponse); got != "legacy final text" {
		t.Fatalf("legacy content=%q", got)
	}
	reasoningOnly := map[string]any{"choices": []any{map[string]any{
		"message": map[string]any{"content": []any{map[string]any{"type": "reasoning", "text": "internal"}}},
	}}}
	if got := chatCompletionText(reasoningOnly); got != "" {
		t.Fatalf("reasoning content was treated as final text: %q", got)
	}
}

func TestProxyExplicitResponsesEndpointUsesResponsesContractAndNormalizesFinalText(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.example.com/v1/responses" {
			t.Fatalf("Responses URL=%q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode Responses request: %v", err)
		}
		if _, exists := body["messages"]; exists {
			t.Fatalf("Responses request retained chat messages: %#v", body)
		}
		if len(sliceFromAny(body["input"])) != 1 || intFromAny(body["max_output_tokens"], 0) != 512 {
			t.Fatalf("Responses request contract=%#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"custom-responses-model","status":"completed",
				"output":[
					{"type":"reasoning","summary":[{"type":"summary_text","text":"not final"}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"responses final"}]}
				],
				"usage":{"input_tokens":20,"output_tokens":9,"total_tokens":29,"output_tokens_details":{"reasoning_tokens":4}}
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:              strPtr("sk-test"),
		Endpoint:            strPtr("https://api.example.com/v1/responses"),
		Model:               strPtr("custom-responses-model"),
		Provider:            strPtr("custom"),
		Messages:            []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:           int64Ptr(256),
		MaxCompletionTokens: int64Ptr(512),
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("Responses call status=%d err=%v", status, err)
	}
	if got := chatCompletionText(resp); got != "responses final" {
		t.Fatalf("Responses final text=%q", got)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["adapter"] != "openai_responses" || metadata["termination_kind"] != "complete" || metadata["reasoning_observed"] != true || intFromAny(metadata["reasoning_tokens"], 0) != 4 {
		t.Fatalf("Responses metadata=%#v", metadata)
	}
}

func TestProxyResponsesReasoningOnlyIncompleteIsNotFinalText(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"custom-responses-model","status":"incomplete",
				"incomplete_details":{"reason":"max_output_tokens"},
				"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"not final"}]}],
				"usage":{"input_tokens":20,"output_tokens":256,"total_tokens":276,"output_tokens_details":{"reasoning_tokens":256}}
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("sk-test"),
		Endpoint: strPtr("https://api.example.com/v1/responses"),
		Model:    strPtr("custom-responses-model"),
		Provider: strPtr("custom"),
		Messages: []any{map[string]any{"role": "user", "content": "ping"}},
	})
	var exhaustedErr *proxyFinalOutputExhaustedError
	if !errors.As(err, &exhaustedErr) || status != http.StatusOK {
		t.Fatalf("Responses incomplete status=%d err=%T %v", status, err, err)
	}
	if got := chatCompletionText(resp); got != "" {
		t.Fatalf("Responses reasoning became final text: %q", got)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["native_finish_reason"] != "max_output_tokens" || metadata["termination_kind"] != "length" || metadata["reasoning_observed"] != true {
		t.Fatalf("Responses incomplete metadata=%#v", metadata)
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

func TestProxyOpenAILikeReasoningFailureDoesNotStripConfiguredControl(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if _, ok := body["reasoning_effort"]; !ok {
			t.Fatalf("request missing reasoning_effort: %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported parameter: reasoning_effort"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "low"
	req := dto.ProxyPluginMainRequest{
		APIKey:              strPtr("sk-test"),
		Endpoint:            strPtr("https://api.example.com/v1"),
		Model:               strPtr("gpt-5-test"),
		Provider:            strPtr("openai"),
		Messages:            []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:           int64Ptr(5),
		MaxCompletionTokens: int64Ptr(256),
		ReasoningEffort:     &effort,
	}
	if _, status, err := performProxyPluginMainWithRetryBudget(context.Background(), req, newLLMRetryBudget(1)); err == nil || status != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v, want original reasoning error", status, err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want exactly one configured request", calls)
	}
}

func TestProxyOpenAILikeReasoningFallbackRespectsZeroRetryBudget(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported parameter: reasoning_effort"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "low"
	req := dto.ProxyPluginMainRequest{
		APIKey:              strPtr("sk-test"),
		Endpoint:            strPtr("https://api.example.com/v1"),
		Model:               strPtr("gpt-5-test"),
		Provider:            strPtr("openai"),
		Messages:            []any{map[string]any{"role": "user", "content": "ping"}},
		MaxTokens:           int64Ptr(5),
		MaxCompletionTokens: int64Ptr(256),
		ReasoningEffort:     &effort,
	}
	if _, status, err := performProxyPluginMainWithRetryBudget(context.Background(), req, newLLMRetryBudget(0)); err == nil || status != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v, want original 400 without compatibility retry", status, err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want exactly one upstream attempt", calls)
	}
}

func TestRuntimeConfigPropagatesClampedLLMRetryBudget(t *testing.T) {
	srv := NewServer(config.Default())
	updated := srv.updateRuntimeConfig(map[string]any{"llmRetryCount": 99})
	if !containsString(updated, "llmRetryCount") {
		t.Fatalf("updated=%v", updated)
	}
	if got := srv.runtimeConfigSnapshot().LLMRetryCount; got != 10 {
		t.Fatalf("retry count=%d, want clamped 10", got)
	}
	budget := srv.supervisorLLMConfig().RetryBudget
	for i := 0; i < 10; i++ {
		if !budget.take() {
			t.Fatalf("budget exhausted at retry %d", i)
		}
	}
	if budget.take() {
		t.Fatal("clamped retry budget allowed an eleventh retry")
	}

	srv.updateRuntimeConfig(map[string]any{"llmRetryCount": 0})
	if srv.supervisorLLMConfig().RetryBudget.take() {
		t.Fatal("retry=0 allowed a second LLM call")
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
	if got := proxyProviderBaseURL("llmgateway", ""); got != "https://api.llmgateway.io/v1" {
		t.Fatalf("LLM Gateway default base = %q", got)
	}
}

func TestProxyNeuralWattStandardUsesChatCompletionsJSON(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.neuralwatt.com/v1/chat/completions" {
			t.Fatalf("upstream URL = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["stream"] != false || body["service_tier"] != "default" {
			t.Fatalf("standard request = %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"test/model","service_tier":"default","choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("nw-test"),
		Endpoint:              strPtr("https://api.neuralwatt.com/v1"),
		Model:                 strPtr("test/model"),
		Provider:              strPtr("neuralwatt"),
		LLMGatewayServiceTier: strPtr("standard"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err != nil || status != http.StatusOK || chatCompletionText(resp) != "ok" {
		t.Fatalf("status=%d err=%v resp=%+v", status, err, resp)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["provider"] != "neuralwatt" || trace["llm_gateway_service_tier_served"] != "default" {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestProxyNeuralWattFlexStreamsExactlyOnceAndReconstructsResponse(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["stream"] != true || body["service_tier"] != "flex" {
			t.Fatalf("flex request = %+v", body)
		}
		if mapFromAny(body["stream_options"])["include_usage"] != true {
			t.Fatalf("stream_options = %+v", body["stream_options"])
		}
		header := make(http.Header)
		header.Set("Content-Type", "text/event-stream")
		header.Set("X-NW-Service-Tier", "flex")
		stream := strings.Join([]string{
			": keepalive",
			"",
			`data: {"id":"chatcmpl-nw","object":"chat.completion.chunk","created":123,"model":"test/model","service_tier":"flex","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"brief"},"finish_reason":null}]}`,
			"",
			`data: {"id":"chatcmpl-nw","object":"chat.completion.chunk","created":123,"model":"test/model","choices":[{"index":0,"delta":{"content":"hello "},"finish_reason":null}]}`,
			"",
			`data: {"id":"chatcmpl-nw","object":"chat.completion.chunk","created":123,"model":"test/model","choices":[{"index":0,"delta":{"content":"world"},"finish_reason":"stop"}]}`,
			"",
			`data: {"id":"chatcmpl-nw","object":"chat.completion.chunk","created":123,"model":"test/model","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`,
			"",
			`: energy {"joules":12.5}`,
			`: cost {"usd":0.01}`,
			"",
			"data: [DONE]",
			"",
		}, "\n")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(stream)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMainWithRetryBudget(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("nw-test"),
		Endpoint:              strPtr("https://api.neuralwatt.com/v1"),
		Model:                 strPtr("test/model"),
		Provider:              strPtr("neuralwatt"),
		LLMGatewayServiceTier: strPtr("flex"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	}, newLLMRetryBudget(3))
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v resp=%+v", status, err, resp)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want one Flex request without hidden retry", calls)
	}
	if got := chatCompletionText(resp); got != "hello world" {
		t.Fatalf("content = %q", got)
	}
	choice := mapFromAny(sliceFromAny(resp["choices"])[0])
	message := mapFromAny(choice["message"])
	if message["reasoning_content"] != "brief" || message["reasoning"] != "brief" {
		t.Fatalf("message = %+v", message)
	}
	if mapFromAny(resp["usage"])["total_tokens"] != float64(13) {
		t.Fatalf("usage = %+v", resp["usage"])
	}
	if mapFromAny(resp["energy"])["joules"] != float64(12.5) || mapFromAny(resp["cost"])["usd"] != float64(0.01) {
		t.Fatalf("energy=%+v cost=%+v", resp["energy"], resp["cost"])
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["llm_gateway_service_tier_served"] != "flex" {
		t.Fatalf("trace = %+v", trace)
	}
}

func TestProxyNeuralWattFlexUpstreamErrorDoesNotFallback(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Status:     "429 Too Many Requests",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"flex queue full"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, status, err := performProxyPluginMainWithRetryBudget(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("nw-test"),
		Endpoint:              strPtr("https://api.neuralwatt.com/v1"),
		Model:                 strPtr("test/model"),
		Provider:              strPtr("neuralwatt"),
		LLMGatewayServiceTier: strPtr("flex"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	}, newLLMRetryBudget(3))
	if err == nil || status != http.StatusTooManyRequests || !strings.Contains(err.Error(), "flex queue full") {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want no Standard fallback", calls)
	}
}

func TestProxyNeuralWattFlexRejectsIncompleteStreamInsteadOfSavingPartialText(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Content-Type", "text/event-stream")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     header,
			Body: io.NopCloser(strings.NewReader(
				"data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n",
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:                strPtr("nw-test"),
		Endpoint:              strPtr("https://api.neuralwatt.com/v1"),
		Model:                 strPtr("test/model"),
		Provider:              strPtr("neuralwatt"),
		LLMGatewayServiceTier: strPtr("flex"),
		Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadGateway || !strings.Contains(err.Error(), "ended before final completion marker") {
		t.Fatalf("status=%d err=%v resp=%+v", status, err, resp)
	}
	if resp != nil {
		t.Fatalf("partial response must not be returned as success: %+v", resp)
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
		{name: "wrong provider", tier: "flex", wantError: "requires provider openai, llmgateway, vercel, neuralwatt, or custom"},
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
			endpoint := "https://api.llmgateway.io/v1"
			if tc.name == "wrong provider" {
				provider = "openrouter"
				endpoint = "https://openrouter.ai/api/v1"
			}
			req := dto.ProxyPluginMainRequest{
				APIKey:                strPtr("llmg-test"),
				Endpoint:              &endpoint,
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

func TestProxyOpenAICompatibleServiceTierProviders(t *testing.T) {
	tests := []struct {
		provider string
		endpoint string
	}{
		{provider: "openai", endpoint: "https://api.openai.com/v1"},
		{provider: "llmgateway", endpoint: "https://api.llmgateway.io/v1"},
		{provider: "vercel", endpoint: "https://ai-gateway.vercel.sh/v1"},
		{provider: "custom", endpoint: "https://custom.example/v1"},
	}
	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				if body["service_tier"] != "flex" {
					t.Fatalf("service_tier = %v, want flex; body=%+v", body["service_tier"], body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"service_tier":"flex","choices":[{"message":{"content":"ok"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:                strPtr("test-key"),
				Endpoint:              strPtr(tc.endpoint),
				Model:                 strPtr("provider/model"),
				Provider:              strPtr(tc.provider),
				LLMGatewayServiceTier: strPtr("flex"),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["llm_gateway_service_tier_applied"] != true {
				t.Fatalf("service tier trace = %+v", trace)
			}
		})
	}
	if got := proxyProviderBaseURL("vercel", ""); got != "https://ai-gateway.vercel.sh/v1" {
		t.Fatalf("Vercel default base = %q", got)
	}
}

func TestProxyOpenAICompatibleExtraOverridesSupportVercelCachingAndCustomJSON(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		endpoint    string
		extraHeader string
		extraBody   string
		assert      func(t *testing.T, r *http.Request, body map[string]any)
	}{
		{
			name:      "Vercel automatic provider caching",
			provider:  "vercel",
			endpoint:  "https://ai-gateway.vercel.sh/v1",
			extraBody: `{"providerOptions":{"gateway":{"caching":"auto"}}}`,
			assert: func(t *testing.T, _ *http.Request, body map[string]any) {
				gateway := mapFromAny(mapFromAny(body["providerOptions"])["gateway"])
				if gateway["caching"] != "auto" {
					t.Fatalf("Vercel caching override = %+v", body)
				}
			},
		},
		{
			name:        "Custom headers and body",
			provider:    "custom",
			endpoint:    "https://custom.example/v1",
			extraHeader: `{"X-Custom-Route":"economy"}`,
			extraBody:   `{"cache_control":{"type":"ephemeral"}}`,
			assert: func(t *testing.T, r *http.Request, body map[string]any) {
				if r.Header.Get("X-Custom-Route") != "economy" {
					t.Fatalf("custom header = %q", r.Header.Get("X-Custom-Route"))
				}
				if mapFromAny(body["cache_control"])["type"] != "ephemeral" {
					t.Fatalf("custom body override = %+v", body)
				}
			},
		},
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
				tc.assert(t, r, body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:           strPtr("test-key"),
				Endpoint:         strPtr(tc.endpoint),
				Model:            strPtr("provider/model"),
				Provider:         strPtr(tc.provider),
				ExtraHeadersJSON: strPtr(tc.extraHeader),
				ExtraBodyJSON:    strPtr(tc.extraBody),
				Messages:         []any{map[string]any{"role": "user", "content": "ping"}},
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["extra_body_applied"] != true {
				t.Fatalf("extra body trace = %+v", trace)
			}
			if tc.extraHeader != "" && trace["extra_headers_applied"] != true {
				t.Fatalf("extra header trace = %+v", trace)
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
						"stop_reason":"max_tokens",
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
			metadata := mapFromAny(resp[proxyResponseMetadataKey])
			if metadata["termination_kind"] != "length" || intFromAny(metadata["input_tokens"], 0) != 21 || intFromAny(metadata["output_tokens"], 0) != 3 {
				t.Fatalf("normalized Claude response metadata=%+v", metadata)
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

func TestProxyGLM52UsesDocumentedThinkingAndReasoningEffort(t *testing.T) {
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
		t.Fatalf("GLM-5.2 reasoning_effort = %v, want max: %+v", upstreamBody["reasoning_effort"], upstreamBody)
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

func TestProxyGLMPre52UsesThinkingToggleWithoutReasoningEffort(t *testing.T) {
	tests := []struct {
		model        string
		effort       string
		wantThinking string
	}{
		{model: "glm-5.1", effort: "enable", wantThinking: "enabled"},
		{model: "glm-5", effort: "disable", wantThinking: "disabled"},
		{model: "glm-4.7", effort: "enable", wantThinking: "enabled"},
	}
	for _, tt := range tests {
		t.Run(tt.model+"_"+tt.effort, func(t *testing.T) {
			oldClient := proxyHTTPClient
			var upstreamBody map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			preset := "glm"
			if _, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("sk-test"), Endpoint: strPtr("https://api.z.ai/api/paas/v4"), Model: &tt.model, Provider: strPtr("custom"),
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), ReasoningPreset: &preset, ReasoningEffort: &tt.effort,
			}); err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			if got := extractionStringFromAny(mapFromAny(upstreamBody["thinking"])["type"]); got != tt.wantThinking {
				t.Fatalf("thinking.type=%q want=%q body=%+v", got, tt.wantThinking, upstreamBody)
			}
			if _, ok := upstreamBody["reasoning_effort"]; ok {
				t.Fatalf("%s must use only the thinking toggle: %+v", tt.model, upstreamBody)
			}
		})
	}
}

func TestProxyOllamaDeepSeekV4ReasoningRequestUsesProviderTransportContract(t *testing.T) {
	tests := []struct {
		name       string
		effort     string
		wantEffort string
		wantTokens float64
	}{
		{name: "low", effort: "low", wantEffort: "low", wantTokens: 11873},
		{name: "high", effort: "high", wantEffort: "high", wantTokens: 11873},
		{name: "stored max", effort: "max", wantEffort: "high", wantTokens: 11873},
		{name: "none", effort: "none", wantEffort: "none", wantTokens: 4096},
		{name: "medium", effort: "medium", wantEffort: "medium", wantTokens: 11873},
		{name: "stored xhigh", effort: "xhigh", wantEffort: "high", wantTokens: 11873},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			upstreamCalls := 0
			var upstreamBody map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				upstreamCalls++
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("Ollama request path=%q want /v1/chat/completions", r.URL.Path)
				}
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &upstreamBody); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"deepseek-v4-flash:0731-cloud","choices":[{"message":{"content":"ok"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			preset := "auto"
			budget := int64(7777)
			req := dto.ProxyPluginMainRequest{
				APIKey:                strPtr("ollama-test"),
				Endpoint:              strPtr("http://127.0.0.1:11434"),
				Model:                 strPtr("deepseek-v4-flash:0731-cloud"),
				Provider:              strPtr("ollama"),
				Messages:              []any{map[string]any{"role": "user", "content": "ping"}},
				MaxTokens:             int64Ptr(5),
				MaxCompletionTokens:   int64Ptr(4096),
				ReasoningPreset:       &preset,
				ReasoningEffort:       &tt.effort,
				ReasoningBudgetTokens: &budget,
				BudgetTokens:          &budget,
			}
			if _, status, err := performProxyPluginMain(context.Background(), req); err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMain status=%d err=%v", status, err)
			}
			if upstreamCalls != 1 {
				t.Fatalf("upstreamCalls = %d, want exactly 1", upstreamCalls)
			}
			if upstreamBody["model"] != "deepseek-v4-flash:0731-cloud" {
				t.Fatalf("model = %v, want exact entered model name", upstreamBody["model"])
			}
			if _, ok := upstreamBody["thinking"]; ok {
				t.Fatalf("Ollama OpenAI-compatible request should not receive DeepSeek-native thinking: %+v", upstreamBody)
			}
			if upstreamBody["reasoning_effort"] != tt.wantEffort {
				t.Fatalf("reasoning_effort = %v, want %q; body=%+v", upstreamBody["reasoning_effort"], tt.wantEffort, upstreamBody)
			}
			if upstreamBody["max_tokens"] != tt.wantTokens {
				t.Fatalf("max_tokens = %v, want %v; body=%+v", upstreamBody["max_tokens"], tt.wantTokens, upstreamBody)
			}
			for _, forbidden := range []string{"max_completion_tokens", "reasoning_budget_tokens", "budget_tokens"} {
				if _, ok := upstreamBody[forbidden]; ok {
					t.Fatalf("DeepSeek V4 request unexpectedly included %q: %+v", forbidden, upstreamBody)
				}
			}
		})
	}
}

func TestProxyReasoningWireUsesProviderAndEndpointTransport(t *testing.T) {
	tests := []struct {
		name               string
		provider           string
		endpoint           string
		model              string
		effort             string
		wantEffort         string
		wantReasoning      string
		wantNativeThinking bool
		wantNoTemperature  bool
	}{
		{name: "LLM Gateway Luna", provider: "llmgateway", endpoint: "https://api.llmgateway.io/v1", model: "gpt-5.6-luna", effort: "low", wantEffort: "low", wantNoTemperature: true},
		{name: "LLM Gateway DeepSeek low", provider: "llmgateway", endpoint: "https://api.llmgateway.io/v1", model: "deepseek-v4-pro:0813-cloud", effort: "low", wantEffort: "low"},
		{name: "LLM Gateway DeepSeek medium compatibility", provider: "llmgateway", endpoint: "https://api.llmgateway.io/v1", model: "deepseek-v4-pro:0813-cloud", effort: "medium", wantEffort: "high"},
		{name: "OpenRouter DeepSeek low", provider: "openrouter", endpoint: "https://openrouter.ai/api/v1", model: "deepseek/deepseek-v4-pro", effort: "low", wantReasoning: "low"},
		{name: "OpenRouter DeepSeek high", provider: "openrouter", endpoint: "https://openrouter.ai/api/v1", model: "deepseek/deepseek-v4-pro", effort: "high", wantReasoning: "high"},
		{name: "NeuralWatt DeepSeek Pro low", provider: "neuralwatt", endpoint: "https://api.neuralwatt.com/v1", model: "deepseek-v4-pro", effort: "low", wantEffort: "low"},
		{name: "NeuralWatt DeepSeek Flash low aliases high", provider: "neuralwatt", endpoint: "https://api.neuralwatt.com/v1", model: "deepseek-v4-flash-flex", effort: "low", wantEffort: "high"},
		{name: "Vercel GPT", provider: "vercel", endpoint: "https://ai-gateway.vercel.sh/v1", model: "openai/gpt-5.6", effort: "medium", wantReasoning: "medium", wantNoTemperature: true},
		{name: "custom OpenAI-compatible DeepSeek low", provider: "custom", endpoint: "https://opencode.ai/zen/v1", model: "deepseek-v4-pro", effort: "low", wantEffort: "low"},
		{name: "custom exact DeepSeek endpoint low", provider: "custom", endpoint: "https://api.deepseek.com/v1", model: "deepseek-v4-pro", effort: "low", wantEffort: "low", wantNativeThinking: true},
		{name: "custom exact DeepSeek endpoint medium compatibility", provider: "custom", endpoint: "https://api.deepseek.com/v1", model: "deepseek-v4-pro", effort: "medium", wantEffort: "high", wantNativeThinking: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			var body map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("test"), Endpoint: &tt.endpoint, Model: &tt.model, Provider: &tt.provider,
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096), ReasoningEffort: &tt.effort,
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if got := extractionStringFromAny(body["reasoning_effort"]); got != tt.wantEffort {
				t.Fatalf("reasoning_effort=%q want=%q body=%+v", got, tt.wantEffort, body)
			}
			if got := extractionStringFromAny(mapFromAny(body["reasoning"])["effort"]); got != tt.wantReasoning {
				t.Fatalf("reasoning.effort=%q want=%q body=%+v", got, tt.wantReasoning, body)
			}
			_, hasThinking := body["thinking"]
			if hasThinking != tt.wantNativeThinking {
				t.Fatalf("thinking present=%v want=%v body=%+v", hasThinking, tt.wantNativeThinking, body)
			}
			_, hasTemperature := body["temperature"]
			if tt.wantNoTemperature && hasTemperature {
				t.Fatalf("reasoning transport kept temperature: %+v", body)
			}
		})
	}
}

func TestProxyReasoningProviderEndpointConflictFailsBeforeUpstream(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return nil, errors.New("unexpected upstream call")
	})}
	defer func() { proxyHTTPClient = oldClient }()

	provider, endpoint, model, effort := "ollama", "https://api.deepseek.com/v1", "deepseek-v4-pro", "low"
	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		Endpoint: &endpoint, Model: &model, Provider: &provider, ReasoningEffort: &effort,
		Messages: []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "conflicts with endpoint host") {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls=%d want 0", upstreamCalls)
	}
}

func TestProxyReasoningExtraBodyConflictFailsBeforeUpstream(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return nil, errors.New("unexpected upstream call")
	})}
	defer func() { proxyHTTPClient = oldClient }()

	provider, endpoint, model, effort := "llmgateway", "https://api.llmgateway.io/v1", "gpt-5.6-luna", "low"
	extraBody := `{"reasoning_effort":"high"}`
	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey: strPtr("test"), Endpoint: &endpoint, Model: &model, Provider: &provider, ReasoningEffort: &effort, ExtraBodyJSON: &extraBody,
		Messages: []any{map[string]any{"role": "user", "content": "ping"}},
	})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "conflicts with backend-managed reasoning_effort") {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls=%d want 0", upstreamCalls)
	}
}

func TestProxyReasoningFamilyUsesModelBeforeProviderDefault(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{model: "deepseek-v4-flash:0731-cloud", want: "deepseek_v4"},
		{model: "gemini-3-pro", want: "gemini"},
		{model: "glm-5.2", want: "glm"},
		{model: "anthropic/claude-4.1-opus", want: "claude"},
		{model: "gpt-5-mini", want: "gpt"},
		{model: "gpt-5.2", want: "gpt"},
		{model: "gpt-5.6-sol", want: "gpt"},
		{model: "qwen3:480b-cloud", want: "ollama_thinking"},
	}
	for _, tt := range tests {
		if got := proxyReasoningFamily("ollama", "auto", tt.model, "http://127.0.0.1:11434"); got != tt.want {
			t.Fatalf("model %q family = %q, want %q", tt.model, got, tt.want)
		}
	}
	if got := proxyReasoningFamily("ollama", "auto", "gpt-oss:20b", "http://127.0.0.1:11434"); got != "ollama_thinking" {
		t.Fatalf("Ollama thinking model family = %q, want ollama_thinking", got)
	}
	if got := proxyReasoningFamily("custom", "auto", "unverified-model", "https://example.invalid/v1"); got != "none" {
		t.Fatalf("unknown custom model family = %q, want none", got)
	}
}

func TestProxyGLMReasoningEffortUsesVersionBoundary(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "glm-4.7", want: false},
		{model: "glm-5", want: false},
		{model: "glm-5.1:cloud", want: false},
		{model: "glm-5.2", want: true},
		{model: "z-ai/glm-5.2:cloud", want: true},
		{model: "glm-6.0", want: true},
		{model: "glm-unversioned", want: false},
	}
	for _, tt := range tests {
		if got := proxyGLMSupportsReasoningEffort(tt.model); got != tt.want {
			t.Errorf("proxyGLMSupportsReasoningEffort(%q)=%v want=%v", tt.model, got, tt.want)
		}
	}
}

func TestProxyOllamaReasoningUsesOpenAICompatibleTransportForDetectedModels(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		effort          string
		glmThinkingType string
		wantEffort      string
		wantMaxTokens   float64
	}{
		{name: "GLM 5.2 high", model: "glm-5.2", effort: "high", wantEffort: "high", wantMaxTokens: 4096},
		{name: "GLM 5.2 max bounded by Ollama wire", model: "glm-5.2", effort: "max", wantEffort: "high", wantMaxTokens: 4096},
		{name: "GLM 5.1 enabled", model: "glm-5.1", effort: "enable", wantEffort: "high", wantMaxTokens: 4096},
		{name: "GLM 5.1 disabled", model: "glm-5.1", effort: "disable", wantEffort: "none", wantMaxTokens: 4096},
		{name: "GLM explicit disabled", model: "glm-5.2", effort: "high", glmThinkingType: "disabled", wantEffort: "none", wantMaxTokens: 4096},
		{name: "Gemini", model: "gemini-3-pro", effort: "medium", wantEffort: "medium", wantMaxTokens: 4096},
		{name: "Claude", model: "claude-sonnet-4-6", effort: "high", wantEffort: "high", wantMaxTokens: 4096},
		{name: "GPT 5.6 max bounded by Ollama wire", model: "gpt-5.6-sol", effort: "max", wantEffort: "high", wantMaxTokens: 4096},
		{name: "GPT 5 baseline minimal normalized by Ollama wire", model: "gpt-5", effort: "minimal", wantEffort: "low", wantMaxTokens: 4096},
		{name: "GPT 5.5 xhigh bounded by Ollama wire", model: "gpt-5.5", effort: "xhigh", wantEffort: "high", wantMaxTokens: 4096},
		{name: "unknown future Gemini", model: "gemini-4-pro", effort: "high", wantMaxTokens: 5},
		{name: "unknown Claude", model: "claude-unversioned", effort: "high", wantMaxTokens: 5},
		{name: "unknown GPT generation", model: "gpt-5.9", effort: "high", wantMaxTokens: 5},
		{name: "unknown model", model: "unverified-model", effort: "high", wantMaxTokens: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			upstreamCalls := 0
			var body map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				upstreamCalls++
				if r.URL.Path != "/v1/chat/completions" {
					t.Fatalf("Ollama request path=%q want /v1/chat/completions", r.URL.Path)
				}
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			req := dto.ProxyPluginMainRequest{
				Endpoint: strPtr("http://127.0.0.1:11434"), Model: &tt.model, Provider: strPtr("ollama"),
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096),
				ReasoningEffort: &tt.effort,
			}
			if tt.glmThinkingType != "" {
				req.GlmThinkingType = &tt.glmThinkingType
			}
			if _, status, err := performProxyPluginMain(context.Background(), req); err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if upstreamCalls != 1 {
				t.Fatalf("upstreamCalls=%d want 1", upstreamCalls)
			}
			if tt.wantEffort == "" {
				if _, ok := body["reasoning_effort"]; ok {
					t.Fatalf("unsupported reasoning value was sent: %+v", body)
				}
			} else if body["reasoning_effort"] != tt.wantEffort {
				t.Fatalf("reasoning_effort=%v want %q body=%+v", body["reasoning_effort"], tt.wantEffort, body)
			}
			if body["max_tokens"] != tt.wantMaxTokens {
				t.Fatalf("max_tokens=%v want %v body=%+v", body["max_tokens"], tt.wantMaxTokens, body)
			}
			for _, forbidden := range []string{"thinking", "thinkingConfig", "output_config", "max_completion_tokens", "reasoning_budget_tokens", "budget_tokens"} {
				if _, ok := body[forbidden]; ok {
					t.Fatalf("Ollama body included native/unsupported %q: %+v", forbidden, body)
				}
			}
		})
	}
}

func TestProxyOpenAIReasoningEffortUsesModelGenerationContract(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		effort     string
		wantEffort string
	}{
		{name: "GPT 5 baseline minimal", model: "gpt-5", effort: "minimal", wantEffort: "minimal"},
		{name: "GPT 5 baseline rejects none", model: "gpt-5", effort: "none"},
		{name: "GPT 5.2 xhigh", model: "gpt-5.2", effort: "xhigh", wantEffort: "xhigh"},
		{name: "GPT 5.5 xhigh", model: "gpt-5.5", effort: "xhigh", wantEffort: "xhigh"},
		{name: "GPT 5.6 max", model: "gpt-5.6-sol", effort: "max", wantEffort: "max"},
		{name: "GPT 5.5 rejects max", model: "gpt-5.5", effort: "max"},
		{name: "unknown GPT generation sends no reasoning", model: "gpt-5.9", effort: "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			var body map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			if _, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("openai-test"), Endpoint: strPtr("https://api.openai.com/v1"), Model: &tt.model, Provider: strPtr("openai"),
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096), ReasoningEffort: &tt.effort,
			}); err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if tt.wantEffort == "" {
				if _, ok := body["reasoning_effort"]; ok {
					t.Fatalf("unsupported effort was sent: %+v", body)
				}
				if body["max_tokens"] != float64(5) {
					t.Fatalf("unsupported effort changed token envelope: %+v", body)
				}
			} else {
				if body["reasoning_effort"] != tt.wantEffort || body["max_completion_tokens"] != float64(4096) {
					t.Fatalf("generation reasoning contract mismatch: %+v", body)
				}
				if _, ok := body["max_tokens"]; ok {
					t.Fatalf("reasoning request kept max_tokens: %+v", body)
				}
			}
		})
	}
}

func TestProxyOllamaUnsupportedReasoningDoesNotRetryWithoutIt(t *testing.T) {
	for _, model := range []string{"deepseek-v4-flash:0731-cloud", "glm-5.2", "gemini-3-pro", "claude-sonnet-4-6"} {
		t.Run(model, func(t *testing.T) {
			oldClient := proxyHTTPClient
			upstreamCalls := 0
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				upstreamCalls++
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Status:     "400 Bad Request",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"unsupported parameter: reasoning_effort"}}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			effort := "high"
			req := dto.ProxyPluginMainRequest{
				Endpoint:        strPtr("http://127.0.0.1:11434"),
				Model:           &model,
				Provider:        strPtr("ollama"),
				Messages:        []any{map[string]any{"role": "user", "content": "ping"}},
				ReasoningEffort: &effort,
			}
			_, status, err := performProxyPluginMainWithRetryBudget(context.Background(), req, newLLMRetryBudget(2))
			if err == nil || status != http.StatusBadRequest {
				t.Fatalf("status=%d err=%v, want provider rejection", status, err)
			}
			if upstreamCalls != 1 {
				t.Fatalf("upstreamCalls = %d, want 1 without a reasoning-stripping retry", upstreamCalls)
			}
		})
	}
}

func TestProxyReasoningContractIsSharedByPublisherAndCritic(t *testing.T) {
	oldClient := proxyHTTPClient
	var bodies []map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		bodies = append(bodies, body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"deepseek-v4-flash:0731-cloud","choices":[{"message":{"content":"{}"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "high"
	budget := int64(7777)
	req := dto.ProxyPluginMainRequest{
		APIKey:                strPtr("ollama-test"),
		Endpoint:              strPtr("http://127.0.0.1:11434"),
		Model:                 strPtr("deepseek-v4-flash:0731-cloud"),
		Provider:              strPtr("ollama"),
		Messages:              []any{map[string]any{"role": "user", "content": "return JSON"}},
		MaxTokens:             int64Ptr(5),
		MaxCompletionTokens:   int64Ptr(4096),
		ReasoningEffort:       &effort,
		ReasoningBudgetTokens: &budget,
		BudgetTokens:          &budget,
	}
	for _, purpose := range []string{"publisher", "complete_turn_critic"} {
		if _, status, err := performProxyPluginMainWithRetryBudgetAndPolicy(context.Background(), req, nil, proxyRequestPolicy{JSONResponse: true, Purpose: purpose}); err != nil || status != http.StatusOK {
			t.Fatalf("purpose=%s status=%d err=%v", purpose, status, err)
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("upstream calls = %d, want one Publisher and one Critic request", len(bodies))
	}
	for i, body := range bodies {
		if _, ok := body["thinking"]; ok {
			t.Fatalf("request %d sent DeepSeek-native thinking through Ollama: %+v", i, body)
		}
		if body["reasoning_effort"] != "high" {
			t.Fatalf("request %d reasoning contract = %+v", i, body)
		}
		if body["max_tokens"] != float64(11873) {
			t.Fatalf("request %d max_tokens = %v, want 11873", i, body["max_tokens"])
		}
		for _, forbidden := range []string{"reasoning_budget_tokens", "budget_tokens", "max_completion_tokens"} {
			if _, ok := body[forbidden]; ok {
				t.Fatalf("request %d included unsupported %s: %+v", i, forbidden, body)
			}
		}
	}
}

func TestProxyClaudeThinkingUsesModelGenerationContract(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		wantMode   string
		wantEffort string
	}{
		{name: "manual budget through 4.5", model: "claude-sonnet-4-5", wantMode: "manual_budget"},
		{name: "dated Claude 4 uses manual budget", model: "claude-opus-4-20250514", wantMode: "manual_budget"},
		{name: "dated Claude 4.5 uses manual budget", model: "claude-sonnet-4-5-20250929", wantMode: "manual_budget"},
		{name: "adaptive from 4.6", model: "claude-sonnet-4-6", wantMode: "adaptive", wantEffort: "high"},
		{name: "unknown model sends no thinking fields", model: "claude-unversioned", wantMode: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			var body map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			effort := "high"
			budget := int64(2048)
			_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("claude-test"), Endpoint: strPtr("https://api.anthropic.com"), Model: &tt.model, Provider: strPtr("claude"),
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096),
				ReasoningEffort: &effort, ReasoningBudgetTokens: &budget, BudgetTokens: &budget,
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if got := proxyClaudeThinkingMode(tt.model); got != tt.wantMode {
				t.Fatalf("mode=%q want=%q", got, tt.wantMode)
			}
			switch tt.wantMode {
			case "manual_budget":
				thinking := mapFromAny(body["thinking"])
				if thinking["type"] != "enabled" || thinking["budget_tokens"] != float64(2048) {
					t.Fatalf("manual thinking = %+v", thinking)
				}
			case "adaptive":
				if mapFromAny(body["thinking"])["type"] != "adaptive" || mapFromAny(body["output_config"])["effort"] != tt.wantEffort {
					t.Fatalf("adaptive body = %+v", body)
				}
			default:
				if _, ok := body["thinking"]; ok {
					t.Fatalf("unknown model received thinking: %+v", body)
				}
				if _, ok := body["output_config"]; ok {
					t.Fatalf("unknown model received output_config: %+v", body)
				}
			}
			if tt.wantMode != "none" {
				if _, ok := body["temperature"]; ok {
					t.Fatalf("thinking request kept temperature: %+v", body)
				}
			}
		})
	}
}

func TestProxyClaudeAdaptiveThinkingKeepsJSONFormatForPublisherAndCritic(t *testing.T) {
	oldClient := proxyHTTPClient
	var bodies []map[string]any
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"{}"}],"stop_reason":"end_turn"}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	effort := "high"
	req := dto.ProxyPluginMainRequest{
		APIKey: strPtr("claude-test"), Endpoint: strPtr("https://api.anthropic.com"), Model: strPtr("claude-sonnet-4-6"), Provider: strPtr("claude"),
		Messages: []any{map[string]any{"role": "user", "content": "return JSON"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096), ReasoningEffort: &effort,
	}
	for _, purpose := range []string{"publisher", "complete_turn_critic"} {
		if _, status, err := performProxyPluginMainWithRetryBudgetAndPolicy(context.Background(), req, nil, proxyRequestPolicy{JSONResponse: true, Purpose: purpose}); err != nil || status != http.StatusOK {
			t.Fatalf("purpose=%s status=%d err=%v", purpose, status, err)
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("upstream calls = %d, want 2", len(bodies))
	}
	for i, body := range bodies {
		outputConfig := mapFromAny(body["output_config"])
		format := mapFromAny(outputConfig["format"])
		if outputConfig["effort"] != "high" || format["type"] != "json_schema" || len(mapFromAny(format["schema"])) == 0 {
			t.Fatalf("request %d output_config = %+v", i, outputConfig)
		}
		if mapFromAny(body["thinking"])["type"] != "adaptive" {
			t.Fatalf("request %d thinking = %+v", i, body["thinking"])
		}
	}
}

func TestProxyGeminiThinkingUsesModelGenerationContract(t *testing.T) {
	tests := []struct {
		model        string
		wantBudget   bool
		wantLevel    bool
		wantThinking bool
	}{
		{model: "gemini-2.5-flash", wantBudget: true, wantThinking: true},
		{model: "gemini-3-pro", wantLevel: true, wantThinking: true},
		{model: "gemini-unversioned"},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			oldClient := proxyHTTPClient
			var body map[string]any
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			effort := "high"
			budget := int64(2048)
			_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("gemini-test"), Endpoint: strPtr("https://generativelanguage.googleapis.com/v1beta"), Model: &tt.model, Provider: strPtr("gemini"),
				Messages: []any{map[string]any{"role": "user", "content": "ping"}}, MaxTokens: int64Ptr(5), MaxCompletionTokens: int64Ptr(4096),
				ReasoningEffort: &effort, ReasoningBudgetTokens: &budget, BudgetTokens: &budget,
			})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			thinking := mapFromAny(mapFromAny(body["generationConfig"])["thinkingConfig"])
			if tt.wantThinking != (len(thinking) > 0) {
				t.Fatalf("thinkingConfig = %+v, want present=%v", thinking, tt.wantThinking)
			}
			if tt.wantBudget && thinking["thinkingBudget"] != float64(2048) {
				t.Fatalf("thinkingBudget = %+v", thinking)
			}
			if tt.wantLevel && thinking["thinkingLevel"] != "high" {
				t.Fatalf("thinkingLevel = %+v", thinking)
			}
			if tt.wantLevel {
				if _, ok := thinking["thinkingBudget"]; ok {
					t.Fatalf("Gemini 3 received stale thinkingBudget: %+v", thinking)
				}
			}
		})
	}
	if got := proxyGeminiThinkingLevel("gemini-3-pro-preview", "minimal"); got != "" {
		t.Fatalf("Gemini 3 Pro accepted unsupported minimal thinking level: %q", got)
	}
	if got := proxyGeminiThinkingLevel("gemini-3.6-flash", "minimal"); got != "minimal" {
		t.Fatalf("Gemini 3.6 Flash minimal thinking level = %q", got)
	}
}

func TestProxyResponseMetadataNormalizesWireTerminationAndTokenUsage(t *testing.T) {
	tests := []struct {
		name       string
		adapter    string
		reason     string
		usage      map[string]any
		wantKind   string
		wantInput  int
		wantOutput int
		wantReason int
	}{
		{
			name: "openai compatible length", adapter: "openai_compatible", reason: "length", wantKind: "length",
			usage:     map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120, "completion_tokens_details": map[string]any{"reasoning_tokens": 7}},
			wantInput: 100, wantOutput: 20, wantReason: 7,
		},
		{
			name: "anthropic max tokens", adapter: "anthropic_messages", reason: "max_tokens", wantKind: "length",
			usage: map[string]any{"input_tokens": 90, "output_tokens": 18}, wantInput: 90, wantOutput: 18,
		},
		{
			name: "google max tokens", adapter: "google_generate_content", reason: "MAX_TOKENS", wantKind: "length",
			usage:     map[string]any{"promptTokenCount": 80, "candidatesTokenCount": 16, "thoughtsTokenCount": 5, "totalTokenCount": 101},
			wantInput: 80, wantOutput: 16, wantReason: 5,
		},
		{
			name: "unknown adapter reason remains observable", adapter: "openai_compatible", reason: "future_reason", wantKind: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := buildProxyResponseMetadata(tt.adapter, tt.reason, tt.usage)
			if meta["termination_kind"] != tt.wantKind || meta["native_finish_reason"] != tt.reason {
				t.Fatalf("termination metadata=%#v", meta)
			}
			if intFromAny(meta["input_tokens"], 0) != tt.wantInput || intFromAny(meta["output_tokens"], 0) != tt.wantOutput || intFromAny(meta["reasoning_tokens"], 0) != tt.wantReason {
				t.Fatalf("token metadata=%#v", meta)
			}
		})
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
			Body: io.NopCloser(strings.NewReader(`{
				"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"text":"gemini ok"}]}}],
				"usageMetadata":{
					"promptTokenCount":4200,
					"candidatesTokenCount":12,
					"thoughtsTokenCount":6,
					"totalTokenCount":4218,
					"cachedContentTokenCount":4096
				}
			}`)),
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
	usage := mapFromAny(resp["usageMetadata"])
	if usage["cachedContentTokenCount"] != float64(4096) || usage["totalTokenCount"] != float64(4218) {
		t.Fatalf("Gemini cache usage was not preserved: %+v", usage)
	}
	metadata := mapFromAny(resp[proxyResponseMetadataKey])
	if metadata["termination_kind"] != "length" || intFromAny(metadata["reasoning_tokens"], 0) != 6 || intFromAny(metadata["total_tokens"], 0) != 4218 {
		t.Fatalf("normalized Gemini response metadata=%+v", metadata)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	usageTrace := mapFromAny(trace["gemini_usage"])
	if usageTrace["cachedContentTokenCount"] != float64(4096) {
		t.Fatalf("Gemini cache usage trace missing: %+v", trace)
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

func TestProxyJSONPolicyAddsOpenAICompatibleResponseFormatAndTrace(t *testing.T) {
	for _, provider := range []string{"openai", "openrouter", "llmgateway", "vercel"} {
		t.Run(provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				format := mapFromAny(body["response_format"])
				wantFormat := "json_object"
				if provider == "vercel" {
					wantFormat = "json_schema"
					schema := mapFromAny(mapFromAny(format["json_schema"])["schema"])
					if schema["type"] != "object" {
						t.Fatalf("Vercel JSON schema = %+v, want object schema", format)
					}
				}
				if format["type"] != wantFormat {
					t.Fatalf("response_format = %+v, want %s; body=%+v", format, wantFormat, body)
				}
				if body["generationConfig"] != nil {
					t.Fatalf("OpenAI-compatible request received Gemini generationConfig: %+v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"test","choices":[{"message":{"content":"{}"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:   strPtr("sk-test"),
				Endpoint: strPtr("https://api.example.com/v1"),
				Model:    strPtr("provider/model"),
				Provider: strPtr(provider),
				Messages: []any{map[string]any{"role": "user", "content": "return json"}},
			}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			wantFormat := "json_object"
			if provider == "vercel" {
				wantFormat = "json_schema"
			}
			if trace["json_response_applied"] != true ||
				trace["json_response_source"] != "backend_policy" ||
				trace["json_response_format"] != wantFormat ||
				trace["json_response_purpose"] != "complete_turn_critic" {
				t.Fatalf("unexpected JSON response trace: %+v", trace)
			}
		})
	}
}

func TestProxyJSONPolicySkipsUnverifiedOpenAILikeProviderWithoutExplicitOverride(t *testing.T) {
	for _, provider := range []string{"custom", "ollama"} {
		t.Run(provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				if body["response_format"] != nil {
					t.Fatalf("unverified provider received forced response_format: %+v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"test","choices":[{"message":{"content":"{}"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:   strPtr("sk-test"),
				Endpoint: strPtr("https://api.example.com/v1"),
				Model:    strPtr("provider/model"),
				Provider: strPtr(provider),
				Messages: []any{map[string]any{"role": "user", "content": "return json"}},
			}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
			if err != nil || status != http.StatusOK {
				t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["json_response_requested"] != true ||
				trace["json_response_applied"] != false ||
				trace["json_response_skip_reason"] != "provider_native_contract_not_verified" {
				t.Fatalf("unexpected capability skip trace: %+v", trace)
			}
		})
	}
}

func TestProxyPublisherJSONPolicyUsesJSONObjectForOpenAICompatibleAdapters(t *testing.T) {
	for _, test := range []struct {
		provider    string
		wantFormat  string
		wantApplied bool
	}{
		{provider: "ollama", wantFormat: "json_object", wantApplied: true},
		{provider: "custom", wantFormat: "json_object", wantApplied: true},
		{provider: "openrouter", wantFormat: "json_object", wantApplied: true},
		{provider: "llmgateway", wantFormat: "json_object", wantApplied: true},
		{provider: "vercel", wantFormat: "json_object", wantApplied: true},
	} {
		t.Run(test.provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			calls := 0
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				format := mapFromAny(body["response_format"])
				if test.wantFormat == "" {
					if len(format) != 0 {
						t.Fatalf("unverified provider received native JSON format: %+v", body)
					}
				} else if format["type"] != test.wantFormat {
					t.Fatalf("response_format = %+v, want %s", format, test.wantFormat)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"test","choices":[{"message":{"content":"{}"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			apiKey := "sk-test"
			if test.provider == "ollama" {
				apiKey = ""
			}
			resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
				APIKey:   &apiKey,
				Endpoint: strPtr("https://api.example.com/v1"),
				Model:    strPtr("provider/model"),
				Provider: strPtr(test.provider),
				Messages: []any{map[string]any{"role": "user", "content": "return publisher JSON"}},
			}, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if calls != 1 {
				t.Fatalf("provider calls = %d, want exactly 1", calls)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["json_response_applied"] != test.wantApplied || trace["json_response_purpose"] != "publisher" {
				t.Fatalf("unexpected publisher JSON trace: %+v", trace)
			}
		})
	}
	for _, provider := range []string{"copilot"} {
		body := map[string]any{}
		trace := map[string]any{}
		if err := proxyApplyOpenAIJSONResponsePolicy(body, trace, provider, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"}); err != nil {
			t.Fatalf("%s publisher JSON policy: %v", provider, err)
		}
		if mapFromAny(body["response_format"])["type"] != "json_object" || trace["json_response_applied"] != true {
			t.Fatalf("%s publisher JSON mode was not applied: body=%+v trace=%+v", provider, body, trace)
		}
	}
}

func TestProxyPublisherJSONPolicyUsesPublisherSchemaWithoutCriticFields(t *testing.T) {
	for _, provider := range []string{"openai"} {
		t.Run(provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				format := mapFromAny(body["response_format"])
				schemaBlock := mapFromAny(format["json_schema"])
				schema := mapFromAny(schemaBlock["schema"])
				serialized, _ := json.Marshal(schema)
				if format["type"] != "json_schema" || schemaBlock["name"] != "archive_center_publisher_output_v3" || schemaBlock["strict"] != true ||
					!strings.Contains(string(serialized), "publisher_output.v3") || !strings.Contains(string(serialized), "book_author") ||
					!strings.Contains(string(serialized), "items") || strings.Contains(string(serialized), "publisher_plan") ||
					strings.Contains(string(serialized), "turn_summary") || strings.Contains(string(serialized), "evidence_excerpts") {
					t.Fatalf("publisher received wrong schema: %+v", format)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"model":"test","choices":[{"message":{"content":"{}"}}]}`)),
				}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()
			resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("sk-test"), Endpoint: strPtr("https://api.example.com/v1"), Model: strPtr("provider/model"), Provider: strPtr(provider),
				Messages: []any{map[string]any{"role": "user", "content": "return publisher JSON"}},
			}, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			trace := mapFromAny(resp["_proxy_request_overrides"])
			if trace["json_response_schema_contract"] != publisherWireContractVersion {
				t.Fatalf("publisher schema trace = %+v", trace)
			}
		})
	}
}

func TestProxyPublisherStrictSchemaSeparatesOptionalPressureLevel(t *testing.T) {
	schema := proxyPublisherTopLevelJSONSchema()
	items := mapFromAny(mapFromAny(schema["properties"])["items"])
	variants := sliceFromAny(mapFromAny(items["items"])["anyOf"])
	if len(variants) != 2 {
		t.Fatalf("publisher item schema variants=%d, want standard and pressure variants: %#v", len(variants), items)
	}

	standard := mapFromAny(variants[0])
	standardProperties := mapFromAny(standard["properties"])
	if _, exists := standardProperties["level"]; exists {
		t.Fatalf("standard publisher item unexpectedly requires pressure level: %#v", standard)
	}
	standardRequired := stringSliceFromAny(standard["required"])
	for _, key := range []string{"role", "field", "text", "source_refs"} {
		if !containsString(standardRequired, key) {
			t.Fatalf("standard publisher item missing required field %q: %#v", key, standard)
		}
	}

	pressure := mapFromAny(variants[1])
	pressureProperties := mapFromAny(pressure["properties"])
	pressureRequired := stringSliceFromAny(pressure["required"])
	if _, exists := pressureProperties["level"]; !exists || !containsString(pressureRequired, "level") {
		t.Fatalf("pressure publisher item does not require level: %#v", pressure)
	}
	if fields := stringSliceFromAny(mapFromAny(pressureProperties["field"])["enum"]); len(fields) != 1 || fields[0] != "pressure_level" {
		t.Fatalf("pressure publisher item fields=%v, want pressure_level only", fields)
	}
}

func TestProxyPublisherLLMGatewaySoftJSONAvoidsUnsupportedStrictSchema(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		format := mapFromAny(body["response_format"])
		if format["type"] == "json_schema" {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"does not support JSON schema output mode"}}`)),
			}, nil
		}
		if format["type"] != "json_object" {
			t.Fatalf("LLM Gateway publisher response_format=%+v, want json_object", format)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"glm-5.3","choices":[{"message":{"content":"{}"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("sk-test"),
		Endpoint: strPtr("https://api.llmgateway.io/v1"),
		Model:    strPtr("glm-5.3"),
		Provider: strPtr("llmgateway"),
		Messages: []any{map[string]any{"role": "user", "content": "return publisher JSON"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstream calls=%d, want exactly one", upstreamCalls)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_format"] != "json_object" ||
		trace["json_response_schema_contract"] != publisherWireContractVersion+"_prompt_validated" ||
		trace["json_response_schema_source"] != "system_prompt" {
		t.Fatalf("LLM Gateway publisher compatibility trace=%#v", trace)
	}
}

func TestProxyPublisherJSONPolicyUsesPublisherSchemaForClaudeAndGemini(t *testing.T) {
	for _, provider := range []string{"claude", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode upstream body: %v", err)
				}
				var schema map[string]any
				if provider == "claude" {
					schema = mapFromAny(mapFromAny(mapFromAny(body["output_config"])["format"])["schema"])
				} else {
					schema = mapFromAny(mapFromAny(body["generationConfig"])["responseJsonSchema"])
				}
				serialized, _ := json.Marshal(schema)
				if !strings.Contains(string(serialized), "publisher_output.v3") || !strings.Contains(string(serialized), "director") ||
					strings.Contains(string(serialized), "publisher_plan") ||
					strings.Contains(string(serialized), "turn_summary") {
					t.Fatalf("%s publisher schema is wrong: %+v", provider, schema)
				}
				response := `{"content":[{"type":"text","text":"{}"}]}`
				if provider == "gemini" {
					response = `{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()
			resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
				APIKey: strPtr("sk-test"), Endpoint: strPtr("https://api.example.com/v1"), Model: strPtr("provider/model"), Provider: strPtr(provider),
				Messages: []any{map[string]any{"role": "user", "content": "return publisher JSON"}},
			}, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
			if err != nil || status != http.StatusOK {
				t.Fatalf("status=%d err=%v", status, err)
			}
			if trace := mapFromAny(resp["_proxy_request_overrides"]); trace["json_response_schema_contract"] != publisherWireContractVersion {
				t.Fatalf("publisher schema trace = %+v", trace)
			}
		})
	}
}

func TestProxyPublisherOpenAIOverrideCannotDowngradeOrReplaceSchema(t *testing.T) {
	policy := proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"}
	for _, provider := range []string{"openai"} {
		t.Run(provider+" rejects json object downgrade", func(t *testing.T) {
			body := map[string]any{"response_format": map[string]any{"type": "json_object"}}
			trace := map[string]any{}
			err := proxyApplyOpenAIJSONResponsePolicy(body, trace, provider, policy)
			if err == nil || !strings.Contains(err.Error(), "json_response_schema_conflict") || trace["json_response_conflict"] != true || trace["json_response_applied"] != false {
				t.Fatalf("%s Publisher json_object downgrade was not rejected: err=%v trace=%#v", provider, err, trace)
			}
		})
	}

	for _, provider := range []string{"custom", "ollama", "openrouter", "llmgateway", "vercel"} {
		t.Run(provider+" retains json object compatibility", func(t *testing.T) {
			body := map[string]any{"response_format": map[string]any{"type": "json_object"}}
			trace := map[string]any{}
			if err := proxyApplyOpenAIJSONResponsePolicy(body, trace, provider, policy); err != nil {
				t.Fatalf("%s Publisher json_object compatibility was rejected: %v", provider, err)
			}
			if trace["json_response_applied"] != true || trace["json_response_format"] != "json_object" {
				t.Fatalf("%s Publisher json_object trace=%#v", provider, trace)
			}
		})
	}

	t.Run("matching schema retained", func(t *testing.T) {
		body := map[string]any{"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "publisher", "schema": proxyPublisherTopLevelJSONSchema()},
		}}
		trace := map[string]any{}
		if err := proxyApplyOpenAIJSONResponsePolicy(body, trace, "llmgateway", policy); err != nil {
			t.Fatalf("matching Publisher schema was rejected: %v", err)
		}
		if trace["json_response_schema_contract"] != publisherWireContractVersion || trace["json_response_schema_source"] != "extra_body_json" {
			t.Fatalf("matching Publisher schema trace=%#v", trace)
		}
	})

	t.Run("wrong schema rejected", func(t *testing.T) {
		body := map[string]any{"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "publisher", "schema": map[string]any{"type": "object", "properties": map[string]any{}}},
		}}
		trace := map[string]any{}
		err := proxyApplyOpenAIJSONResponsePolicy(body, trace, "llmgateway", policy)
		if err == nil || !strings.Contains(err.Error(), "json_response_schema_conflict") || trace["json_response_conflict"] != true {
			t.Fatalf("wrong Publisher schema was not rejected: err=%v trace=%#v", err, trace)
		}
	})
}

func TestProxyPublisherStrictSchemaConflictStopsBeforeUpstreamCall(t *testing.T) {
	oldClient := proxyHTTPClient
	upstreamCalls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamCalls++
		return nil, fmt.Errorf("unexpected upstream call")
	})}
	defer func() { proxyHTTPClient = oldClient }()

	extraBody := `{"response_format":{"type":"json_object"}}`
	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:        strPtr("sk-test"),
		Endpoint:      strPtr("https://api.example.com/v1"),
		Model:         strPtr("provider-neutral-model"),
		Provider:      strPtr("openai"),
		ExtraBodyJSON: &extraBody,
		Messages:      []any{map[string]any{"role": "user", "content": "return publisher json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"})
	if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "json_response_schema_conflict") {
		t.Fatalf("status=%d err=%v, want explicit Publisher schema conflict", status, err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream calls=%d, want zero after local schema conflict", upstreamCalls)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_conflict"] != true || trace["json_response_source"] != "extra_body_json" {
		t.Fatalf("Publisher schema conflict trace=%#v", trace)
	}
}

func TestProxyPublisherClaudeAndGeminiOverridesRequireExactSchema(t *testing.T) {
	policy := proxyRequestPolicy{JSONResponse: true, Purpose: "publisher"}
	tests := []struct {
		name  string
		apply func(map[string]any, map[string]any) error
		body  func(any) map[string]any
	}{
		{
			name: "claude",
			apply: func(body, trace map[string]any) error {
				return proxyApplyClaudeJSONResponsePolicy(body, trace, policy)
			},
			body: func(schema any) map[string]any {
				return map[string]any{"output_config": map[string]any{"format": map[string]any{"type": "json_schema", "schema": schema}}}
			},
		},
		{
			name: "gemini",
			apply: func(body, trace map[string]any) error {
				return proxyApplyJSONResponsePolicy(body, trace, policy)
			},
			body: func(schema any) map[string]any {
				return map[string]any{"generationConfig": map[string]any{"responseMimeType": "application/json", "responseJsonSchema": schema}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name+" matching", func(t *testing.T) {
			trace := map[string]any{}
			if err := test.apply(test.body(proxyPublisherTopLevelJSONSchema()), trace); err != nil {
				t.Fatalf("matching %s Publisher schema was rejected: %v", test.name, err)
			}
			if trace["json_response_schema_contract"] != publisherWireContractVersion || trace["json_response_schema_source"] != "extra_body_json" {
				t.Fatalf("matching %s Publisher schema trace=%#v", test.name, trace)
			}
		})
		t.Run(test.name+" wrong", func(t *testing.T) {
			trace := map[string]any{}
			err := test.apply(test.body(map[string]any{"type": "object", "properties": map[string]any{}}), trace)
			if err == nil || !strings.Contains(err.Error(), "json_response_schema_conflict") || trace["json_response_conflict"] != true {
				t.Fatalf("wrong %s Publisher schema was not rejected: err=%v trace=%#v", test.name, err, trace)
			}
		})
	}
}

func TestProxyOpenAICompatibleJSONPolicyPreservesSchemaAndRejectsConflict(t *testing.T) {
	t.Run("preserves json schema", func(t *testing.T) {
		oldClient := proxyHTTPClient
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("decode upstream body: %v", err)
			}
			format := mapFromAny(body["response_format"])
			if format["type"] != "json_schema" || mapFromAny(format["json_schema"])["name"] != "critic" {
				t.Fatalf("json schema override was not preserved: %+v", format)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{}"}}]}`)),
			}, nil
		})}
		defer func() { proxyHTTPClient = oldClient }()

		extraBody := `{"response_format":{"type":"json_schema","json_schema":{"name":"critic","schema":{"type":"object"}}}}`
		resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:        strPtr("sk-test"),
			Endpoint:      strPtr("https://api.example.com/v1"),
			Model:         strPtr("gpt-test"),
			Provider:      strPtr("custom"),
			ExtraBodyJSON: &extraBody,
			Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
		}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
		if err != nil || status != http.StatusOK {
			t.Fatalf("status=%d err=%v", status, err)
		}
		trace := mapFromAny(resp["_proxy_request_overrides"])
		if trace["json_response_source"] != "extra_body_json" || trace["json_response_format"] != "json_schema" {
			t.Fatalf("unexpected schema trace: %+v", trace)
		}
	})

	t.Run("rejects conflicting format before upstream", func(t *testing.T) {
		oldClient := proxyHTTPClient
		upstreamCalls := 0
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			upstreamCalls++
			return nil, fmt.Errorf("unexpected upstream call")
		})}
		defer func() { proxyHTTPClient = oldClient }()

		extraBody := `{"response_format":{"type":"text"}}`
		resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:        strPtr("sk-test"),
			Endpoint:      strPtr("https://api.example.com/v1"),
			Model:         strPtr("gpt-test"),
			Provider:      strPtr("custom"),
			ExtraBodyJSON: &extraBody,
			Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
		}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
		if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "json_response_format_conflict") {
			t.Fatalf("status=%d err=%v, want JSON format conflict", status, err)
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstreamCalls=%d, want 0", upstreamCalls)
		}
		trace := mapFromAny(resp["_proxy_request_overrides"])
		if trace["json_response_conflict"] != true || trace["json_response_applied"] != false {
			t.Fatalf("unexpected conflict trace: %+v", trace)
		}
	})
}

func TestProxyClaudeJSONPolicyAddsOutputConfigAndTrace(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		format := mapFromAny(mapFromAny(body["output_config"])["format"])
		if format["type"] != "json_schema" {
			t.Fatalf("Claude output_config.format = %+v, want json_schema", format)
		}
		schema := mapFromAny(format["schema"])
		properties := mapFromAny(schema["properties"])
		if schema["type"] != "object" || schema["additionalProperties"] != true ||
			mapFromAny(properties["turn_summary"])["type"] != "string" ||
			mapFromAny(properties["importance_score"])["type"] != "number" ||
			len(mapFromAny(properties["records"])) != 0 ||
			len(mapFromAny(properties["contract_version"])) != 0 {
			t.Fatalf("Claude critic schema is incomplete: %+v", schema)
		}
		if required := sliceFromAny(schema["required"]); len(required) != 2 {
			t.Fatalf("Claude critic schema required fields = %#v, want lightweight core", required)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"{\"turn_summary\":\"ok\",\"importance_score\":5}"}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("claude-key"),
		Endpoint: strPtr("https://api.anthropic.com"),
		Model:    strPtr("claude-opus-5"),
		Provider: strPtr("claude"),
		Messages: []any{map[string]any{"role": "user", "content": "return json"}},
	}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("performProxyPluginMainWithPolicy status=%d err=%v", status, err)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	if trace["json_response_applied"] != true ||
		trace["json_response_source"] != "backend_policy" ||
		trace["json_response_format"] != "json_schema" ||
		trace["json_response_purpose"] != "complete_turn_critic" {
		t.Fatalf("unexpected Claude JSON trace: %+v", trace)
	}
}

func TestProxyClaudeJSONPolicyPreservesMatchingExtraBodyAndRejectsConflict(t *testing.T) {
	t.Run("preserves matching output config", func(t *testing.T) {
		oldClient := proxyHTTPClient
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("decode upstream body: %v", err)
			}
			format := mapFromAny(mapFromAny(body["output_config"])["format"])
			if format["type"] != "json_schema" || mapFromAny(format["schema"])["type"] != "object" {
				t.Fatalf("matching Claude output_config was not preserved: %+v", body)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"content":[{"type":"text","text":"{}"}]}`)),
			}, nil
		})}
		defer func() { proxyHTTPClient = oldClient }()

		extraBody := `{"output_config":{"format":{"type":"json_schema","schema":{"type":"object","properties":{},"additionalProperties":false}}}}`
		resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:        strPtr("claude-key"),
			Endpoint:      strPtr("https://api.anthropic.com"),
			Model:         strPtr("claude-opus-5"),
			Provider:      strPtr("claude"),
			ExtraBodyJSON: &extraBody,
			Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
		}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
		if err != nil || status != http.StatusOK {
			t.Fatalf("status=%d err=%v", status, err)
		}
		trace := mapFromAny(resp["_proxy_request_overrides"])
		if trace["json_response_source"] != "extra_body_json" || trace["json_response_applied"] != true {
			t.Fatalf("unexpected matching Claude trace: %+v", trace)
		}
	})

	t.Run("rejects conflicting output config before upstream", func(t *testing.T) {
		oldClient := proxyHTTPClient
		upstreamCalls := 0
		proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			upstreamCalls++
			return nil, fmt.Errorf("unexpected upstream call")
		})}
		defer func() { proxyHTTPClient = oldClient }()

		extraBody := `{"output_config":{"format":{"type":"text"}}}`
		resp, status, err := performProxyPluginMainWithPolicy(context.Background(), dto.ProxyPluginMainRequest{
			APIKey:        strPtr("claude-key"),
			Endpoint:      strPtr("https://api.anthropic.com"),
			Model:         strPtr("claude-opus-5"),
			Provider:      strPtr("claude"),
			ExtraBodyJSON: &extraBody,
			Messages:      []any{map[string]any{"role": "user", "content": "return json"}},
		}, proxyRequestPolicy{JSONResponse: true, Purpose: "complete_turn_critic"})
		if err == nil || status != http.StatusBadRequest || !strings.Contains(err.Error(), "json_response_format_conflict") {
			t.Fatalf("status=%d err=%v, want Claude JSON format conflict", status, err)
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstreamCalls=%d, want 0", upstreamCalls)
		}
		trace := mapFromAny(resp["_proxy_request_overrides"])
		if trace["json_response_conflict"] != true || trace["json_response_applied"] != false {
			t.Fatalf("unexpected Claude conflict trace: %+v", trace)
		}
	})
}

func TestProxyOpenAILikeRequestWithoutJSONPolicyHasNoResponseFormat(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["response_format"] != nil {
			t.Fatalf("ordinary request unexpectedly forced JSON: %+v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	_, status, err := performProxyPluginMain(context.Background(), dto.ProxyPluginMainRequest{
		APIKey:   strPtr("sk-test"),
		Endpoint: strPtr("https://api.example.com/v1"),
		Model:    strPtr("gpt-test"),
		Provider: strPtr("openai"),
		Messages: []any{map[string]any{"role": "user", "content": "normal reply"}},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v", status, err)
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
				Body: io.NopCloser(strings.NewReader(`{
					"candidates":[{"content":{"parts":[{"text":"vertex ok"}]}}],
					"usageMetadata":{
						"promptTokenCount":5000,
						"candidatesTokenCount":10,
						"totalTokenCount":5010,
						"cachedContentTokenCount":4096,
						"trafficType":"ON_DEMAND_FLEX"
					}
				}`)),
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
	usage := mapFromAny(resp["usageMetadata"])
	if usage["cachedContentTokenCount"] != float64(4096) || usage["trafficType"] != "ON_DEMAND_FLEX" {
		t.Fatalf("Vertex cache/tier usage was not preserved: %+v", usage)
	}
	trace := mapFromAny(resp["_proxy_request_overrides"])
	usageTrace := mapFromAny(trace["gemini_usage"])
	if usageTrace["cachedContentTokenCount"] != float64(4096) || usageTrace["trafficType"] != "ON_DEMAND_FLEX" {
		t.Fatalf("Vertex cache/tier trace missing: %+v", trace)
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

func TestVoyageContextDocumentEmbeddingUsesOneNestedSiblingGroupAndMapsIndexes(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if got := r.URL.String(); got != "https://api.voyageai.com/v1/contextualizedembeddings" {
			t.Fatalf("upstream URL = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got := extractionStringFromAny(body["input_type"]); got != "document" {
			t.Fatalf("input_type = %q", got)
		}
		groups := sliceFromAny(body["inputs"])
		if len(groups) != 1 {
			t.Fatalf("input groups = %d, want one logical document", len(groups))
		}
		chunks := sliceFromAny(groups[0])
		if len(chunks) != 2 || extractionStringFromAny(chunks[0]) != "chunk one" || extractionStringFromAny(chunks[1]) != "chunk two" {
			t.Fatalf("chunks = %#v, want two ordered siblings", chunks)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"data":[{"index":0,"data":[
					{"index":1,"embedding":[2.0,2.1],"text":"chunk two"},
					{"index":0,"embedding":[1.0,1.1],"text":"chunk one"}
				]}],
				"model":"voyage-context-4"
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embeddings, model, err := callDocumentEmbeddings(context.Background(), completeTurnEmbeddingConfig{
		Provider: "voyageai", APIKey: "voyage-key",
		Endpoint: "https://api.voyageai.com/v1/embeddings", Model: "voyage-context-4",
	}, []string{"chunk one", "chunk two"})
	if err != nil {
		t.Fatalf("callDocumentEmbeddings error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want one contextualized request", calls)
	}
	if model != "voyage-context-4" || !reflect.DeepEqual(embeddings, []string{"[1,1.1]", "[2,2.1]"}) {
		t.Fatalf("model=%q embeddings=%#v", model, embeddings)
	}
}

func TestVoyageContextDocumentEmbeddingKeepsDuplicateChunkPositionsDistinct(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		chunks := sliceFromAny(sliceFromAny(body["inputs"])[0])
		if len(chunks) != 2 || extractionStringFromAny(chunks[0]) != "same text" || extractionStringFromAny(chunks[1]) != "same text" {
			t.Fatalf("duplicate chunks were changed: %#v", chunks)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"index":0,"data":[{"index":1,"embedding":[2]},{"index":0,"embedding":[1]}]}],"model":"voyage-context-4"}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embeddings, _, err := callDocumentEmbeddings(context.Background(), completeTurnEmbeddingConfig{
		Provider: "voyageai", APIKey: "voyage-key", Endpoint: "https://api.voyageai.com/v1", Model: "voyage-context-4",
	}, []string{"same text", "same text"})
	if err != nil {
		t.Fatalf("callDocumentEmbeddings error: %v", err)
	}
	if !reflect.DeepEqual(embeddings, []string{"[1]", "[2]"}) {
		t.Fatalf("duplicate chunk embeddings = %#v", embeddings)
	}
}

func TestVoyageContextQueryUsesSingleQueryGroup(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got := extractionStringFromAny(body["input_type"]); got != "query" {
			t.Fatalf("input_type = %q", got)
		}
		groups := sliceFromAny(body["inputs"])
		if len(groups) != 1 || len(sliceFromAny(groups[0])) != 1 {
			t.Fatalf("query groups = %#v, want [[query]]", groups)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"index":0,"data":[{"index":0,"embedding":[0.4,0.5]}]}],"model":"voyage-context-4"}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embedding, _, err := callQueryEmbedding(context.Background(), completeTurnEmbeddingConfig{
		Provider: "voyageai", APIKey: "voyage-key", Endpoint: "https://api.voyageai.com/v1", Model: "voyage-context-4",
	}, "where is the key?")
	if err != nil || embedding != `[0.4,0.5]` {
		t.Fatalf("embedding=%q err=%v", embedding, err)
	}
}

func TestStandardVoyageModelKeepsEmbeddingsEndpoint(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://api.voyageai.com/v1/embeddings" {
			t.Fatalf("upstream URL = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, exists := body["inputs"]; exists {
			t.Fatalf("standard Voyage request used contextualized inputs: %#v", body)
		}
		if _, exists := body["input_type"]; exists {
			t.Fatalf("standard Voyage request changed its existing input_type contract: %#v", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"index":0,"embedding":[0.7,0.8]}],"model":"voyage-4-large"}`))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	embedding, model, err := callEmbedding(context.Background(), completeTurnEmbeddingConfig{
		Provider: "voyageai", APIKey: "voyage-key", Endpoint: "https://api.voyageai.com/v1", Model: "voyage-4-large",
	}, "ordinary document")
	if err != nil || model != "voyage-4-large" || embedding != `[0.7,0.8]` {
		t.Fatalf("model=%q embedding=%q err=%v", model, embedding, err)
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
		format := mapFromAny(upstreamReq["response_format"])
		schemaBlock := mapFromAny(format["json_schema"])
		schemaJSON, _ := json.Marshal(mapFromAny(schemaBlock["schema"]))
		if format["type"] != "json_schema" || schemaBlock["name"] != "archive_center_publisher_output_v3" ||
			!strings.Contains(string(schemaJSON), "publisher_output.v3") || strings.Contains(string(schemaJSON), "publisher_plan") ||
			strings.Contains(string(schemaJSON), "turn_summary") {
			t.Fatalf("supervisor request missing publisher-native schema or leaked critic schema: %+v", format)
		}
		if !strings.Contains(userPrompt, "response_execution_contract") ||
			!strings.Contains(userPrompt, "supervisor_support_packet") ||
			!strings.Contains(userPrompt, "guide_focus") {
			t.Fatalf("supervisor request body missing bounded memory guidance inputs: %s", userPrompt)
		}
		for _, removed := range []string{"required_output", "publisher_strength_profile", "publisher_strength_profile.v1"} {
			if strings.Contains(userPrompt, removed) {
				t.Fatalf("supervisor compact request retained duplicate field %q: %s", removed, userPrompt)
			}
		}
		if strings.Contains(userPrompt, "supervisor_proposal_coverage") {
			t.Fatalf("supervisor request body still exposes the legacy proposal vocabulary: %s", userPrompt)
		}
		if !strings.Contains(systemPrompt, "Archive Center's Publisher LLM") ||
			!strings.Contains(systemPrompt, "The current user input is the only command") ||
			!strings.Contains(systemPrompt, "Use accepted recent context") {
			t.Fatalf("supervisor system prompt missing memory-guide boundary: %s", systemPrompt)
		}
		for _, forbidden := range []string{"Story Initiative", "max_new_beats", "narrative_stance", "auto_advance_trigger"} {
			if strings.Contains(systemPrompt, forbidden) || strings.Contains(userPrompt, `"`+forbidden+`"`) {
				t.Fatalf("supervisor prompt contains story-control field %q: system=%s user=%s", forbidden, systemPrompt, userPrompt)
			}
		}
		if !strings.Contains(systemPrompt, "publisher_output.v3") ||
			!strings.Contains(systemPrompt, "cannot decide user or protagonist action") ||
			!strings.Contains(systemPrompt, "quiet supported scene") ||
			!strings.Contains(systemPrompt, "Do not add filler or meet a") {
			t.Fatalf("supervisor prompt missing bounded publisher contract: %s", systemPrompt)
		}
		response := publisherV3OpenAIResponse("input:test", "preserve the current request boundary")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(response)),
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
	plan := mapFromAny(proposal["publisher_plan"])
	accepted := anySliceFromAny(plan["accepted_items"])
	if plan["contract_version"] != "publisher_plan.v2" || len(accepted) != 1 ||
		mapFromAny(accepted[0])["field"] != "current_arc" {
		t.Fatalf("supervisor publisher plan = %+v", plan)
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
