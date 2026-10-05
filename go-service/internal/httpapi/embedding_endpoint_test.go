package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestVoyageEndpointFollowsModel(t *testing.T) {
	for _, endpoint := range []string{"", "  ", "https://api.voyageai.com/v1", "https://api.voyageai.com/v1/embeddings", "https://api.voyageai.com/v1/contextualizedembeddings/", "https://proxy.example.test/v1", "https://proxy.example.test/v1/embeddings", "https://proxy.example.test/v1/contextualizedembeddings"} {
		base := "https://api.voyageai.com/v1"
		if strings.Contains(endpoint, "proxy.example.test") {
			base = "https://proxy.example.test/v1"
		}
		if got := normalizeVoyageEmbeddingEndpoint(endpoint); got != base+"/embeddings" {
			t.Errorf("ordinary endpoint %q => %q", endpoint, got)
		}
		if got := normalizeVoyageContextualizedEmbeddingEndpoint(endpoint); got != base+"/contextualizedembeddings" {
			t.Errorf("contextual endpoint %q => %q", endpoint, got)
		}
	}
}

// Exercise config-save, configuration authority and the production recall owner.
// Provider HTTP and the external vector store are controlled boundaries, not
// live Voyage/Chroma or a RisuAI Host E2E test. Unexpected requests fail the test.
func TestVoyageAutomaticEndpointSaveAndRecall(t *testing.T) {
	for _, origin := range []string{"runtime", "client_meta"} {
		for _, endpoint := range []string{"", "https://api.voyageai.com/v1/embeddings", "https://proxy.example.test/v1/contextualizedembeddings"} {
			t.Run(origin+"/"+endpoint, func(t *testing.T) {
				cfg := config.Default()
				cfg.ChromaEndpoint = "http://controlled-vector.invalid"
				srv := NewServer(cfg)
				srv.Vector = &restorationRepeatedHitVector{}
				srv.VectorOpenError = nil
				mux := http.NewServeMux()
				srv.RegisterRoutes(mux)
				// Switching in both directions must not retain the previous model's route.
				for _, model := range []string{"voyage-4-large", "voyage-context-4", "voyage-4-large"} {
					settings := map[string]any{"embeddingProvider": "voyageai", "embeddingApiKey": "synthetic-unused-key", "embeddingModel": model, "embeddingEndpoint": endpoint, "embeddingTimeout": 30}
					encoded, _ := json.Marshal(settings)
					saved := httptest.NewRecorder()
					mux.ServeHTTP(saved, httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader(encoded)))
					if saved.Code != http.StatusOK {
						t.Fatalf("save: %d %s", saved.Code, saved.Body.String())
					}
					var response map[string]any
					if err := json.Unmarshal(saved.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					trace := mapFromAny(mapFromAny(response["runtime_config_trace"])["embedding"])
					if trace["configured"] != true || len(sliceFromAny(trace["missing_fields"])) != 0 {
						t.Fatalf("automatic endpoint was considered missing: %v", trace)
					}
					if endpoint == "" && trace["endpoint_source"] != "provider_default.voyageai" {
						t.Fatalf("default endpoint source: %v", trace)
					}
					if srv.runtimeConfigSnapshot().EmbeddingEndpoint != endpoint {
						t.Fatal("save overwrote the user's automatic/custom endpoint choice")
					}
					input := "Recall the silver shield."
					req := dto.PrepareTurnRequest{ChatSessionID: "synthetic-endpoint-probe", RawUserInput: &input}
					if origin == "client_meta" {
						req.ClientMeta = map[string]any{"embedding": map[string]any{"provider": "voyageai", "api_key": "synthetic-unused-key", "model": model, "endpoint": endpoint, "timeout_ms": 30000}}
					}
					resolved := srv.completeTurnExtractionConfig(req.ClientMeta).Embedder
					wantHost, wantPath := "api.voyageai.com", "/v1/embeddings"
					if strings.Contains(endpoint, "proxy.example.test") {
						wantHost = "proxy.example.test"
					}
					contextual := strings.HasPrefix(model, "voyage-context-")
					if contextual {
						wantPath = "/v1/contextualizedembeddings"
					}
					if !resolved.hasConfig() || resolved.Endpoint != "https://"+wantHost+wantPath {
						t.Fatalf("resolved endpoint=%q configured=%v", resolved.Endpoint, resolved.hasConfig())
					}
					calls := 0
					inputType := "query"
					oldClient := proxyHTTPClient
					proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
						calls++
						if r.URL.String() != resolved.Endpoint || r.Method != http.MethodPost {
							return nil, fmt.Errorf("unexpected destination: %s %s", r.Method, r.URL)
						}
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							return nil, err
						}
						if body["model"] != model {
							return nil, fmt.Errorf("unexpected model: %v", body)
						}
						if contextual {
							if body["input_type"] != inputType || !reflect.DeepEqual(body["inputs"], []any{[]any{input}}) {
								return nil, fmt.Errorf("unexpected contextual input: %v", body)
							}
						} else if !reflect.DeepEqual(body["input"], []any{input}) {
							return nil, fmt.Errorf("unexpected input: %v", body)
						}
						result := `{"data":[{"embedding":[0.1,0.2,0.3]}]}`
						if contextual {
							result = `{"data":[{"index":0,"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}]}`
						}
						return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(result))}, nil
					})}
					func() {
						defer func() { proxyHTTPClient = oldClient }()
						recall := srv.prepareTurnVectorShadow(context.Background(), req, 5)
						if calls != 1 || recall["query_embedding_status"] != "ok" || intFromAny(recall["query_embedding_count"], 0) != 1 {
							t.Fatalf("recall failed: calls=%d trace=%v", calls, recall)
						}
						inputType = "document"
						vector, gotModel, err := callEmbedding(context.Background(), resolved, input)
						if err != nil || vector != "[0.1,0.2,0.3]" || gotModel != model || calls != 2 {
							t.Fatalf("document embedding: calls=%d vector=%q model=%q err=%v", calls, vector, gotModel, err)
						}
					}()
					t.Logf("model=%s origin=%s endpoint=%s query_and_document_calls=%d", model, origin, resolved.Endpoint, calls)
				}
			})
		}
	}
}

func TestVoyageAutomaticEndpointConfigurationBoundaries(t *testing.T) {
	srv := NewServer(config.Default())
	t.Setenv("AC_EMBEDDER_PROVIDER", "voyageai")
	t.Setenv("AC_EMBEDDER_API_KEY", "synthetic-env-key")
	t.Setenv("AC_EMBEDDER_MODEL", "voyage-context-4")
	srv.RuntimeConfig.EmbeddingTimeoutSec = 30
	envCfg := srv.completeTurnExtractionConfig(nil).Embedder
	if !envCfg.hasConfig() || envCfg.Endpoint != "https://api.voyageai.com/v1/contextualizedembeddings" || envCfg.Source != "env_or_config" {
		t.Fatalf("environment endpoint=%q source=%q missing=%v", envCfg.Endpoint, envCfg.Source, envCfg.missingFields())
	}
	partial := srv.completeTurnExtractionConfig(map[string]any{"embedding": map[string]any{"provider": "voyageai", "model": "voyage-4-large"}}).Embedder
	if partial.APIKey != "" || partial.Source != "client_meta_partial" || !reflect.DeepEqual(partial.missingFields(), []string{"api_key"}) {
		t.Fatal("automatic address changed credential authority or missing-key behavior")
	}
	srv.RuntimeConfig.EmbeddingProvider = "voyageai"
	srv.RuntimeConfig.EmbeddingAPIKey = "synthetic-runtime-key"
	srv.RuntimeConfig.EmbeddingModel = "voyage-context-4"
	providerOnly := srv.completeTurnExtractionConfig(map[string]any{"embedding": map[string]any{"provider": "voyageai"}}).Embedder
	if !providerOnly.hasConfig() || providerOnly.Source != "runtime_config" || providerOnly.APIKey != srv.RuntimeConfig.EmbeddingAPIKey {
		t.Fatal("provider-only observation replaced the selected runtime configuration")
	}
	for _, provider := range []string{"openai", "custom", "ollama", "gemini", "vertex"} {
		c := completeTurnEmbeddingConfig{Provider: provider, APIKey: "synthetic", Model: "fixture", TimeoutMs: 30000}
		if !reflect.DeepEqual(c.missingFields(), []string{"endpoint"}) {
			t.Fatalf("unrelated provider %s changed: %v", provider, c.missingFields())
		}
	}
	missingModel := completeTurnEmbeddingConfig{Provider: "voyageai", APIKey: "synthetic", TimeoutMs: 30000}
	if !reflect.DeepEqual(missingModel.missingFields(), []string{"model"}) {
		t.Fatalf("missing model behavior: %v", missingModel.missingFields())
	}
}
