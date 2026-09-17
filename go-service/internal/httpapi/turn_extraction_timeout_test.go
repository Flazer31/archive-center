package httpapi

import (
	"context"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestCompleteTurnExtractionTimeoutsPreferRuntimeUISettings(t *testing.T) {
	srv := NewServer(config.Default())
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.CriticTimeoutSec = 5365
	srv.RuntimeConfig.EmbeddingTimeoutSec = 30

	cfg := srv.completeTurnExtractionConfig(map[string]any{
		"critic": map[string]any{
			"timeout_ms": int64(15000),
		},
		"embedding": map[string]any{
			"timeout_ms": int64(15000),
		},
	})

	if cfg.Critic.TimeoutMs != 5365000 {
		t.Fatalf("critic timeout = %d, want runtime UI setting 5365000", cfg.Critic.TimeoutMs)
	}
	if cfg.Embedder.TimeoutMs != 30000 {
		t.Fatalf("embedding timeout = %d, want runtime UI setting 30000", cfg.Embedder.TimeoutMs)
	}
}

func TestRuntimeTimeoutConversionDoesNotInventFallback(t *testing.T) {
	for _, seconds := range []int64{0, -1} {
		if got := runtimeTimeoutMs(seconds); got != 0 {
			t.Fatalf("runtimeTimeoutMs(%d)=%d, want no timeout", seconds, got)
		}
	}
	if got := runtimeTimeoutMs(45); got != 45000 {
		t.Fatalf("runtimeTimeoutMs(45)=%d, want 45000", got)
	}
}

func TestPublisherAndCriticDefaultCompletionTokensAreThirtyThousand(t *testing.T) {
	critic := completeTurnExtractionConfigFromMeta(nil).Critic
	if critic.MaxTokens != 30000 || critic.MaxCompletionTokens != 30000 {
		t.Fatalf("critic defaults = max_tokens:%d max_completion_tokens:%d, want 30000/30000", critic.MaxTokens, critic.MaxCompletionTokens)
	}

	publisher := (&Server{}).supervisorLLMConfig()
	if publisher.MaxTokens != 30000 {
		t.Fatalf("publisher default max_tokens=%d, want 30000", publisher.MaxTokens)
	}
}

func TestCompleteTurnConfigRequiresExplicitPositiveTimeout(t *testing.T) {
	srv := &Server{Cfg: config.Default()}
	cfg := srv.completeTurnExtractionConfig(map[string]any{
		"critic": map[string]any{
			"provider": "openai", "api_key": "critic-key",
			"endpoint": "https://example.invalid/v1", "model": "critic-model",
		},
		"embedding": map[string]any{
			"provider": "openai", "api_key": "embedding-key",
			"endpoint": "https://example.invalid/v1", "model": "embedding-model",
		},
	})
	if cfg.Critic.TimeoutMs != 0 || cfg.Critic.hasConfig() ||
		!stringSliceContains(cfg.Critic.missingFields(), "timeout_ms") ||
		cfg.Critic.Source != "client_meta_partial.critic" {
		t.Fatalf("critic config=%+v missing=%v", cfg.Critic, cfg.Critic.missingFields())
	}
	if cfg.Embedder.TimeoutMs != 0 || cfg.Embedder.hasConfig() ||
		!stringSliceContains(cfg.Embedder.missingFields(), "timeout_ms") ||
		cfg.Embedder.Source != "client_meta_partial" {
		t.Fatalf("embedding config=%+v missing=%v", cfg.Embedder, cfg.Embedder.missingFields())
	}

	_, trace, err := srv.runCompleteTurnCritic(
		context.Background(), "session", 1, "user", "assistant", nil, nil, cfg.Critic,
	)
	details := criticPipelineErrorDetails(err)
	if details["code"] != "CRITIC_CONFIG_MISSING" ||
		details["stage"] != "configuration" ||
		details["retryable"] != false ||
		trace["code"] != "CRITIC_CONFIG_MISSING" {
		t.Fatalf("error=%v details=%+v trace=%+v", err, details, trace)
	}
}

func TestRuntimeRoleConfigsRequireUISyncedPositiveTimeouts(t *testing.T) {
	srv := &Server{
		RuntimeConfig: RuntimeConfig{
			Synced:       true,
			MainProvider: "openai", MainAPIKey: "main-key",
			MainEndpoint: "https://example.invalid/v1", MainModel: "main-model",
			SupervisorProvider: "openai", SupervisorAPIKey: "supervisor-key",
			SupervisorEndpoint: "https://example.invalid/v1", SupervisorModel: "supervisor-model",
			SourceSearchPlannerProvider: "openai", SourceSearchPlannerAPIKey: "source-key",
			SourceSearchPlannerEndpoint: "https://example.invalid/v1", SourceSearchPlannerModel: "source-model",
		},
	}
	for name, cfg := range map[string]completeTurnLLMConfig{
		"main":                  srv.chapterLLMConfig(),
		"supervisor":            srv.supervisorLLMConfig(),
		"source_search_planner": srv.sourceSearchPlannerLLMConfig(),
	} {
		if cfg.TimeoutMs != 0 || cfg.hasConfig() ||
			!stringSliceContains(cfg.missingFields(), "timeout_ms") {
			t.Fatalf("%s config=%+v missing=%v", name, cfg, cfg.missingFields())
		}
	}
}
