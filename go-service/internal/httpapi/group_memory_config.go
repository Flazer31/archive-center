package httpapi

import (
	"net/http"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

// Retrieval config: R2 write guards

func (s *Server) handleRetrievalIndexRuntimeConfigPost(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /retrieval-index/runtime-config")
}

func (s *Server) handleIntentRoutingRuntimeConfigPost(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"source":          "go_r1_read_shadow",
		"routing_version": "p58a.v1",
		"routing_mode":    "per_intent_shadow",
		"default_route":   "single_query_shared",
		"intents": []map[string]any{
			{"intent": "scene", "tiers": []string{"episode", "memory", "chapter"}, "budget_share": 0.34},
			{"intent": "callback", "tiers": []string{"arc", "saga", "memory"}, "budget_share": 0.22},
			{"intent": "resume", "tiers": []string{"chapter", "arc", "saga"}, "budget_share": 0.28},
			{"intent": "canon", "tiers": []string{"memory", "episode", "arc"}, "budget_share": 0.16},
		},
		"budget_policy": map[string]any{
			"budget_mode":    "policy_only",
			"degrade_policy": "drop_low_score_then_shorten_text",
		},
		"trace": map[string]any{
			"intent_route": "single_query_shared",
			"shadow_ready": true,
		},
	})
}
