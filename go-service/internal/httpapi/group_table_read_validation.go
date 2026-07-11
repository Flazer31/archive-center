package httpapi

import (
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func tableReadValidateLLM(req dto.ProxyPluginMainRequest) error {
	if strings.TrimSpace(tableReadStringPtrValue(req.Provider, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Endpoint, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.APIKey, "")) == "" ||
		strings.TrimSpace(tableReadStringPtrValue(req.Model, "")) == "" {
		return &tableReadValidationError{"llm.provider, llm.endpoint, llm.api_key, and llm.model are required"}
	}
	return nil
}

type tableReadValidationError struct {
	message string
}

func (e *tableReadValidationError) Error() string {
	return e.message
}

func tableReadStringPtrValue(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func tableReadInt64PtrValue(v *int64, fallback int64) int64 {
	if v == nil {
		return fallback
	}
	return *v
}

func tableReadFloatPtrValue(v *float64, fallback float64) float64 {
	if v == nil {
		return fallback
	}
	return *v
}

func nilIfEmptyMap(v map[string]any) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

func buildTableReadOrchestration(req tableReadMultiModelRequest, agentCount int) map[string]any {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "single_orchestrator_dry_run"
	}
	maxParallel := req.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 1
	}
	if maxParallel > agentCount && agentCount > 0 {
		maxParallel = agentCount
	}
	return map[string]any{
		"multi_model_supported": true,
		"multi_model_enabled":   req.Enabled,
		"execution_mode":        mode,
		"max_parallel":          maxParallel,
		"require_consensus":     req.RequireConsensus,
		"execution_order": []string{
			"agent_private_notes",
			"cross_character_discussion",
			"moderator_synthesis",
			"support_only_prepare_turn_hint",
		},
		"tr1_execution_guard": "no_llm_call_in_tr1",
	}
}

func tableReadMemoryCards(memories []store.ProtagonistEntityMemory, limit int) []map[string]any {
	if limit <= 0 || limit > len(memories) {
		limit = len(memories)
	}
	out := make([]map[string]any, 0, limit)
	for _, memory := range memories[:limit] {
		out = append(out, map[string]any{
			"id":                   memory.ID,
			"source_turn_index":    memory.SourceTurn,
			"memory_text_preview":  tableReadPreview(memory.MemoryText, 180),
			"secret_guard":         memory.SecretGuard,
			"target_reveal_policy": memory.TargetRevealPolicy,
			"portability":          memory.Portability,
			"importance_10":        memory.Importance10,
			"emotional_weight":     memory.EmotionalWeight,
		})
	}
	return out
}

func tableReadPrivateMemoryPolicy(role string) map[string]any {
	role = strings.ToLower(strings.TrimSpace(role))
	private := role == "npc" || role == "character" || strings.Contains(role, "private")
	lane := "persona_recollection"
	if private {
		lane = "character_private_recollection"
	}
	return map[string]any{
		"lane":              lane,
		"support_only":      true,
		"reveal_to_player":  !private,
		"treat_as":          "subjective_interpretation",
		"truth_authority":   false,
		"canonical_write":   false,
		"scene_use_allowed": true,
	}
}

func tableReadDefaultPerspective(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "protagonist", "player", "persona":
		return "what this person remembers, fears, and intends without declaring it as narrator truth"
	case "npc", "character":
		return "private character recollection and possible misunderstanding, not direct exposition"
	default:
		return "scene participant reading"
	}
}

func tableReadPreview(text string, max int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if max <= 0 || len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "..."
}

func tableReadPreviewPreserveLines(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if max <= 0 || len(text) <= max {
		return text
	}
	return strings.TrimSpace(text[:max]) + "..."
}
