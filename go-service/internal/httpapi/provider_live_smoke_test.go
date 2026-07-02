package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

type providerSmokeLaneConfig struct {
	Provider  string
	APIKey    string
	Endpoint  string
	Model     string
	TimeoutMs int64
}

func TestConfiguredProviderLiveSmoke(t *testing.T) {
	if os.Getenv("AC_PROVIDER_SMOKE") != "1" {
		t.Skip("set AC_PROVIDER_SMOKE=1 to run configured provider smoke")
	}
	envPath := strings.TrimSpace(os.Getenv("AC_PROVIDER_SMOKE_ENV"))
	envSource := "process_env"
	var env map[string]string
	if envPath != "" {
		envSource = filepath.Base(envPath)
		var err error
		env, err = readProviderSmokeEnv(envPath)
		if err != nil {
			t.Fatalf("read smoke env: %v", err)
		}
	} else {
		env = readProviderSmokeProcessEnv()
	}

	mainCfg := providerSmokeConfig(env, "PROJECT_MAIN", 60000)
	supervisorCfg := providerSmokeConfig(env, "PROJECT_SUPERVISOR", 60000)
	criticCfg := providerSmokeConfig(env, "PROJECT_CRITIC", 90000)
	embeddingCfg := providerSmokeConfig(env, "PROJECT_EMBEDDING", 30000)

	report := map[string]any{
		"schema":        "archive-center.provider-smoke.v1",
		"seq":           "SEQ-01",
		"rmg":           "RMG-01",
		"scope":         "configured Project Main/Supervisor/Critic/Embedding live provider smoke",
		"env_source":    envSource,
		"created_at":    time.Now().UTC().Format(time.RFC3339),
		"secret_policy": "API keys are never written to this report.",
		"lanes":         map[string]any{},
	}
	lanes := report["lanes"].(map[string]any)

	lanes["main"] = runProviderMainSmoke(t, mainCfg)
	lanes["supervisor"] = runProviderSupervisorSmoke(t, supervisorCfg)
	lanes["critic"] = runProviderCriticSmoke(t, criticCfg)
	lanes["embedding"] = runProviderEmbeddingSmoke(t, embeddingCfg)

	status := "ok"
	for _, name := range []string{"main", "supervisor", "critic", "embedding"} {
		lane := lanes[name].(map[string]any)
		if lane["status"] != "ok" {
			status = "failed"
		}
	}
	report["status"] = status

	if outPath := strings.TrimSpace(os.Getenv("AC_PROVIDER_SMOKE_REPORT")); outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			t.Fatalf("create smoke report dir: %v", err)
		}
		raw, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(outPath, append(raw, '\n'), 0644); err != nil {
			t.Fatalf("write smoke report: %v", err)
		}
	}
	if status != "ok" {
		t.Fatalf("provider smoke failed: %+v", lanes)
	}
}

func runProviderMainSmoke(t *testing.T, cfg providerSmokeLaneConfig) map[string]any {
	t.Helper()
	return runProviderChatSmoke(t, "main", cfg, []any{
		map[string]any{"role": "system", "content": "Return exactly AC_PROVIDER_SMOKE_OK."},
		map[string]any{"role": "user", "content": "Archive Center configured provider smoke."},
	})
}

func runProviderSupervisorSmoke(t *testing.T, cfg providerSmokeLaneConfig) map[string]any {
	t.Helper()
	start := time.Now()
	lane := providerSmokeLaneTrace("supervisor", cfg)
	if missing := providerSmokeMissing(cfg); len(missing) > 0 {
		lane["status"] = "missing_config"
		lane["missing_fields"] = missing
		return lane
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerSmokeTimeout(cfg))
	defer cancel()
	srv := setupTestServer()
	sid := "provider-smoke-seq01"
	req := dto.SupervisorRequest{
		ChatSessionID:         strPtr(sid),
		GuideMode:             strPtr("strict"),
		NarrativeStance:       strPtr("balanced"),
		AutoAdvanceTrigger:    strPtr("none"),
		WakeUpContext:         strPtr("provider smoke"),
		PersistentGuidance:    strPtr("return bounded JSON"),
		ContextMessages:       []map[string]any{{"role": "user", "content": "A short smoke turn."}},
		NarrativeStanceBounds: map[string]any{"pressure": "normal"},
	}
	pack := buildSupervisorInputPack(sid, 1, "provider smoke", "strict", "weak", "balanced", "none", "provider smoke", map[string]any{}, map[string]any{"context_messages": 1}, nil, storylineSupervisorSelection{}, false, "", nil)
	result, trace, err := srv.runSupervisorLLM(ctx, sid, pack, req, completeTurnLLMConfig{
		APIKey:      cfg.APIKey,
		Endpoint:    cfg.Endpoint,
		Model:       cfg.Model,
		Provider:    cfg.Provider,
		TimeoutMs:   cfg.TimeoutMs,
		MaxTokens:   256,
		Temperature: 0,
	})
	lane["elapsed_ms"] = time.Since(start).Milliseconds()
	if err != nil {
		lane["status"] = "error"
		lane["error"] = scrubProxySecret(err.Error(), cfg.APIKey)
		return lane
	}
	lane["status"] = "ok"
	lane["model_reported"] = extractionFirstNonEmpty(extractionStringFromAny(trace["model"]), cfg.Model)
	lane["usage_present"] = trace["usage"] != nil
	lane["result_keys_count"] = len(result)
	return lane
}

func runProviderCriticSmoke(t *testing.T, cfg providerSmokeLaneConfig) map[string]any {
	t.Helper()
	start := time.Now()
	lane := providerSmokeLaneTrace("critic", cfg)
	if missing := providerSmokeMissing(cfg); len(missing) > 0 {
		lane["status"] = "missing_config"
		lane["missing_fields"] = missing
		return lane
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerSmokeTimeout(cfg))
	defer cancel()
	srv := setupTestServer()
	result, trace, err := srv.runCompleteTurnCritic(ctx, "provider-smoke-seq01", 1, "Mina asks Rowan to remember the blue key.", "Rowan promises to keep the blue key safe.", nil, nil, completeTurnLLMConfig{
		APIKey:      cfg.APIKey,
		Endpoint:    cfg.Endpoint,
		Model:       cfg.Model,
		Provider:    cfg.Provider,
		TimeoutMs:   cfg.TimeoutMs,
		MaxTokens:   900,
		Temperature: 0,
	})
	lane["elapsed_ms"] = time.Since(start).Milliseconds()
	if err != nil {
		lane["status"] = "error"
		lane["error"] = scrubProxySecret(err.Error(), cfg.APIKey)
		return lane
	}
	lane["status"] = "ok"
	lane["model_reported"] = extractionFirstNonEmpty(extractionStringFromAny(trace["model"]), cfg.Model)
	lane["usage_present"] = trace["usage"] != nil
	lane["turn_summary_present"] = strings.TrimSpace(extractionStringFromAny(result["turn_summary"])) != ""
	lane["evidence_count"] = len(stringsFromAny(result["evidence_excerpts"]))
	lane["kg_triples_count"] = len(sliceFromAny(result["kg_triples"]))
	return lane
}

func runProviderEmbeddingSmoke(t *testing.T, cfg providerSmokeLaneConfig) map[string]any {
	t.Helper()
	start := time.Now()
	lane := providerSmokeLaneTrace("embedding", cfg)
	if missing := providerSmokeMissing(cfg); len(missing) > 0 {
		lane["status"] = "missing_config"
		lane["missing_fields"] = missing
		return lane
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerSmokeTimeout(cfg))
	defer cancel()
	embedding, model, err := callEmbedding(ctx, completeTurnEmbeddingConfig{
		APIKey:    cfg.APIKey,
		Endpoint:  cfg.Endpoint,
		Model:     cfg.Model,
		Provider:  cfg.Provider,
		TimeoutMs: cfg.TimeoutMs,
	}, "Archive Center SEQ-01 provider smoke embedding.")
	lane["elapsed_ms"] = time.Since(start).Milliseconds()
	if err != nil {
		lane["status"] = "error"
		lane["error"] = scrubProxySecret(err.Error(), cfg.APIKey)
		return lane
	}
	var vector []float64
	if err := json.Unmarshal([]byte(embedding), &vector); err != nil {
		lane["status"] = "error"
		lane["error"] = "embedding_vector_decode_failed"
		return lane
	}
	lane["status"] = "ok"
	lane["model_reported"] = extractionFirstNonEmpty(model, cfg.Model)
	lane["dimension"] = len(vector)
	return lane
}

func runProviderChatSmoke(t *testing.T, name string, cfg providerSmokeLaneConfig, messages []any) map[string]any {
	t.Helper()
	start := time.Now()
	lane := providerSmokeLaneTrace(name, cfg)
	if missing := providerSmokeMissing(cfg); len(missing) > 0 {
		lane["status"] = "missing_config"
		lane["missing_fields"] = missing
		return lane
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerSmokeTimeout(cfg))
	defer cancel()
	maxTokens := int64(512)
	temp := 0.0
	resp, status, err := performProxyPluginMain(ctx, dto.ProxyPluginMainRequest{
		APIKey:      &cfg.APIKey,
		Endpoint:    &cfg.Endpoint,
		Model:       &cfg.Model,
		Provider:    &cfg.Provider,
		Messages:    messages,
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		TimeoutMs:   &cfg.TimeoutMs,
	})
	lane["elapsed_ms"] = time.Since(start).Milliseconds()
	lane["http_status"] = status
	if err != nil {
		lane["status"] = "error"
		lane["error"] = scrubProxySecret(err.Error(), cfg.APIKey)
		return lane
	}
	lane["status"] = "ok"
	lane["model_reported"] = extractionFirstNonEmpty(extractionStringFromAny(resp["model"]), cfg.Model)
	lane["usage_present"] = resp["usage"] != nil
	lane["content_present"] = strings.TrimSpace(chatCompletionText(resp)) != ""
	return lane
}

func providerSmokeLaneTrace(name string, cfg providerSmokeLaneConfig) map[string]any {
	return map[string]any{
		"lane":          name,
		"provider":      cfg.Provider,
		"endpoint_host": endpointHost(cfg.Endpoint),
		"model":         cfg.Model,
		"timeout_ms":    cfg.TimeoutMs,
	}
}

func providerSmokeMissing(cfg providerSmokeLaneConfig) []string {
	return configMissingFieldsWithProvider(cfg.Provider, cfg.APIKey, cfg.Endpoint, cfg.Model)
}

func providerSmokeTimeout(cfg providerSmokeLaneConfig) time.Duration {
	if cfg.TimeoutMs > 0 {
		return time.Duration(cfg.TimeoutMs) * time.Millisecond
	}
	return 60 * time.Second
}

func providerSmokeConfig(env map[string]string, prefix string, fallbackTimeout int64) providerSmokeLaneConfig {
	timeout := int64(intFromAny(env[prefix+"_TIMEOUT"], 0))
	if timeout > 0 {
		timeout *= 1000
	} else {
		timeout = fallbackTimeout
	}
	return providerSmokeLaneConfig{
		Provider:  strings.TrimSpace(env[prefix+"_PROVIDER"]),
		APIKey:    strings.TrimSpace(env[prefix+"_API_KEY"]),
		Endpoint:  strings.TrimSpace(env[prefix+"_ENDPOINT"]),
		Model:     strings.TrimSpace(env[prefix+"_MODEL"]),
		TimeoutMs: timeout,
	}
}

func readProviderSmokeEnv(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid env line %d", i+1)
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if len(out) == 0 {
		return nil, errors.New("env_empty")
	}
	return out, nil
}

func readProviderSmokeProcessEnv() map[string]string {
	out := map[string]string{}
	keys := []string{
		"PROJECT_MAIN_PROVIDER", "PROJECT_MAIN_API_KEY", "PROJECT_MAIN_ENDPOINT", "PROJECT_MAIN_MODEL", "PROJECT_MAIN_TIMEOUT",
		"PROJECT_SUPERVISOR_PROVIDER", "PROJECT_SUPERVISOR_API_KEY", "PROJECT_SUPERVISOR_ENDPOINT", "PROJECT_SUPERVISOR_MODEL", "PROJECT_SUPERVISOR_TIMEOUT",
		"PROJECT_CRITIC_PROVIDER", "PROJECT_CRITIC_API_KEY", "PROJECT_CRITIC_ENDPOINT", "PROJECT_CRITIC_MODEL", "PROJECT_CRITIC_TIMEOUT",
		"PROJECT_EMBEDDING_PROVIDER", "PROJECT_EMBEDDING_API_KEY", "PROJECT_EMBEDDING_ENDPOINT", "PROJECT_EMBEDDING_MODEL", "PROJECT_EMBEDDING_TIMEOUT",
	}
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			out[key] = value
		}
	}
	if strings.TrimSpace(out["PROJECT_MAIN_API_KEY"]) == "" {
		if nanoKey := strings.TrimSpace(os.Getenv("NANOGPT_API_KEY")); nanoKey != "" {
			nanoEndpoint := extractionFirstNonEmpty(os.Getenv("NANOGPT_BASE_URL"), "https://nano-gpt.com/api/v1")
			nanoChatModel := extractionFirstNonEmpty(os.Getenv("NANOGPT_CHAT_MODEL"), "openai/gpt-5.4-nano")
			nanoCriticModel := extractionFirstNonEmpty(os.Getenv("NANOGPT_CRITIC_MODEL"), nanoChatModel)
			nanoEmbeddingModel := extractionFirstNonEmpty(os.Getenv("NANOGPT_EMBEDDING_MODEL"), "text-embedding-3-small")
			out["PROJECT_MAIN_PROVIDER"] = "openai"
			out["PROJECT_MAIN_API_KEY"] = nanoKey
			out["PROJECT_MAIN_ENDPOINT"] = nanoEndpoint
			out["PROJECT_MAIN_MODEL"] = nanoChatModel
			out["PROJECT_MAIN_TIMEOUT"] = extractionFirstNonEmpty(os.Getenv("NANOGPT_CHAT_TIMEOUT"), "90")
			out["PROJECT_SUPERVISOR_PROVIDER"] = "openai"
			out["PROJECT_SUPERVISOR_API_KEY"] = nanoKey
			out["PROJECT_SUPERVISOR_ENDPOINT"] = nanoEndpoint
			out["PROJECT_SUPERVISOR_MODEL"] = nanoChatModel
			out["PROJECT_SUPERVISOR_TIMEOUT"] = extractionFirstNonEmpty(os.Getenv("NANOGPT_CHAT_TIMEOUT"), "90")
			out["PROJECT_CRITIC_PROVIDER"] = "openai"
			out["PROJECT_CRITIC_API_KEY"] = nanoKey
			out["PROJECT_CRITIC_ENDPOINT"] = nanoEndpoint
			out["PROJECT_CRITIC_MODEL"] = nanoCriticModel
			out["PROJECT_CRITIC_TIMEOUT"] = extractionFirstNonEmpty(os.Getenv("NANOGPT_CRITIC_TIMEOUT"), "120")
			out["PROJECT_EMBEDDING_PROVIDER"] = "openai"
			out["PROJECT_EMBEDDING_API_KEY"] = nanoKey
			out["PROJECT_EMBEDDING_ENDPOINT"] = nanoEndpoint
			out["PROJECT_EMBEDDING_MODEL"] = nanoEmbeddingModel
			out["PROJECT_EMBEDDING_TIMEOUT"] = extractionFirstNonEmpty(os.Getenv("NANOGPT_EMBEDDING_TIMEOUT"), "60")
		}
	}
	return out
}
