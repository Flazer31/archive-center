package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
)

func TestCompleteTurnExtractionTimeoutsPreferRuntimeUISettings(t *testing.T) {
	srv := NewServer(config.Default())
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
