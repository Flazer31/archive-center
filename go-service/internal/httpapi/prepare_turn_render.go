package httpapi

import (
	"encoding/json"
	"strings"

	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
)

type prepareTurnInjectionBlock struct {
	Label   string
	Text    string
	Source  string
	Count   int
	Budget  int
	Trimmed bool
}

type prepareTurnInjectionAssembly struct {
	Text                     string
	SagaText                 string
	ChapterText              string
	MemoryText               string
	KGText                   string
	DirectEvidenceText       string
	FallbackText             string
	StorylineText            string
	WorldRulesText           string
	CharacterText            string
	PendingThreadText        string
	EpisodeText              string
	PersonaText              string
	CharacterPrivateText     string
	ContinuityCorrectionText string
	LatestDirectEvidenceText string
	RecentRawTurnText        string
	ScopedVerbatimText       string
	ScopedVerbatimSupport    archivebridge.ScopedVerbatimSupport
	ArcText                  string
	CanonText                string
	Truncated                bool
	Blocks                   []prepareTurnInjectionBlock
	Trimmed                  []map[string]any
	BudgetDecisions          map[string]any
	Counts                   map[string]any
	LanguageContext          map[string]any
	LanguageInjectionTrace   map[string]any
	PerspectiveContext       map[string]any
}

func buildInjectionPack(rawUserInput, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated bool, assembly prepareTurnInjectionAssembly, temporalSupportPacket map[string]any) map[string]any {
	status := "skeleton"
	if !injectionEnabled && !inputContextEnabled {
		status = "off"
	} else if strings.TrimSpace(assembly.Text) != "" || strings.TrimSpace(inputContextText) != "" {
		status = "ready"
	}

	blockTrace := make([]map[string]any, 0, len(assembly.Blocks))
	for _, block := range assembly.Blocks {
		blockTrace = append(blockTrace, map[string]any{
			"label":   block.Label,
			"source":  block.Source,
			"count":   block.Count,
			"chars":   len([]rune(block.Text)),
			"budget":  block.Budget,
			"trimmed": block.Trimmed,
		})
	}

	budgetDecisions := assembly.BudgetDecisions
	if budgetDecisions == nil {
		budgetDecisions = map[string]any{
			"version":                    "t1c.v1",
			"mode":                       "read_only_surface",
			"status":                     "off",
			"decision_count":             0,
			"decisions":                  []map[string]any{},
			"global_cap_chars":           6000,
			"global_selected_chars":      0,
			"canon_floor_reserved_chars": 120,
			"canon_selected_chars":       0,
			"reason_counts":              map[string]int{"tier_cap": 0},
			"source_mapping":             "recall_result.intent_execution_shadow.budget_enforcement",
			"source_event":               "budget_enforcement",
			"source_counters":            []string{"decision_count", "global_cap_chars", "global_selected_chars", "canon_floor_reserved_chars", "canon_selected_chars", "reason_counts"},
		}
	}

	var temporalPacket any
	var temporalPacketText string
	if temporalSupportPacket != nil {
		if packet, ok := temporalSupportPacket["temporal_packet"].(map[string]any); ok {
			temporalPacket = packet
		}
		if text, ok := temporalSupportPacket["temporal_packet_text"].(string); ok {
			temporalPacketText = text
		}
	}

	return map[string]any{
		"status":                                status,
		"source":                                "go_r1_read_shadow",
		"effective_user_input":                  rawUserInput,
		"injection_text":                        nilIfEmpty(assembly.Text),
		"input_context_text":                    nilIfEmpty(inputContextText),
		"continuity_correction_text":            nilIfEmpty(assembly.ContinuityCorrectionText),
		"memory_text":                           nilIfEmpty(assembly.MemoryText),
		"language_context":                      nilIfEmptyMap(assembly.LanguageContext),
		"language_injection_trace":              nilIfEmptyMap(assembly.LanguageInjectionTrace),
		"perspective_context":                   nilIfEmptyMap(assembly.PerspectiveContext),
		"kg_text":                               nilIfEmpty(assembly.KGText),
		"direct_evidence_text":                  nilIfEmpty(assembly.DirectEvidenceText),
		"fallback_text":                         nilIfEmpty(assembly.FallbackText),
		"storyline_text":                        nilIfEmpty(assembly.StorylineText),
		"world_rules_text":                      nilIfEmpty(assembly.WorldRulesText),
		"character_text":                        nilIfEmpty(assembly.CharacterText),
		"pending_thread_text":                   nilIfEmpty(assembly.PendingThreadText),
		"episode_text":                          nilIfEmpty(assembly.EpisodeText),
		"persona_recollection_text":             nilIfEmpty(assembly.PersonaText),
		"persona_recollection_active":           strings.TrimSpace(assembly.PersonaText) != "",
		"persona_recollection_policy":           personaRecollectionSupportPolicy(strings.TrimSpace(assembly.PersonaText) != ""),
		"character_private_recollection_text":   nilIfEmpty(assembly.CharacterPrivateText),
		"character_private_recollection_active": strings.TrimSpace(assembly.CharacterPrivateText) != "",
		"character_private_recollection_policy": characterPrivateRecollectionPolicy(strings.TrimSpace(assembly.CharacterPrivateText) != ""),
		"latest_direct_evidence_text": nilIfEmpty(
			assembly.LatestDirectEvidenceText,
		),
		"scoped_verbatim_support_text":  nilIfEmpty(assembly.ScopedVerbatimText),
		"scoped_verbatim_support_count": assembly.ScopedVerbatimSupport.Count,
		"scoped_verbatim_support_items": assembly.ScopedVerbatimSupport.Items,
		"verbatim_support":              assembly.ScopedVerbatimSupport,
		"hierarchy_escape_hatch":        buildHierarchyEscapeHatch(assembly.ScopedVerbatimSupport),
		"recent_raw_turn_text":          nilIfEmpty(assembly.RecentRawTurnText),
		"canon_text":                    nilIfEmpty(assembly.CanonText),
		"temporal_packet":               temporalPacket,
		"temporal_packet_text":          nilIfEmpty(temporalPacketText),
		"would_inject":                  injectionEnabled && strings.TrimSpace(assembly.Text) != "",
		"input_context_enabled":         inputContextEnabled,
		"injection_truncated":           assembly.Truncated,
		"input_context_truncated":       inputContextTruncated,
		"budget_decisions":              budgetDecisions,
		"section_blocks":                blockTrace,
		"trimmed":                       assembly.Trimmed,
		"counts":                        assembly.Counts,
		"status_vocabulary":             []string{"off", "skeleton", "partial", "ready", "degraded"},
		"final_budget_owner":            "archive_center_js_assembleInjectionWithBudget",
		"apply_verdict":                 "shadow_only",
		"apply_verdict_rule":            "trace_only",
		"saga_text":                     nilIfEmpty(assembly.SagaText),
		"saga_delivered":                strings.TrimSpace(assembly.SagaText) != "",
		"chapter_text":                  nilIfEmpty(assembly.ChapterText),
		"chapter_delivered":             strings.TrimSpace(assembly.ChapterText) != "",
		"arc_text":                      nilIfEmpty(assembly.ArcText),
		"arc_delivered":                 strings.TrimSpace(assembly.ArcText) != "",
		"would_call_llm":                false,
		"would_write":                   false,
	}
}

func buildPrepareTurnInputTransparencyRenderModel(sid string, turnIndex int, rawUserInput, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded bool, fallbackReason string, assembly prepareTurnInjectionAssembly) map[string]any {
	status := prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled, assembly.Text, inputContextText)
	counts := prepareTurnRenderCounts(assembly, inputContextText)
	blocks := []map[string]any{}
	appendPrepareTurnRenderBlock(&blocks, "user_input", "User Input", "prepare_turn.raw_user_input", rawUserInput, 1, true, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "input_context", "Input Context", "prepare_turn.input_context_text", inputContextText, boolToInt(strings.TrimSpace(inputContextText) != ""), inputContextEnabled, inputContextTruncated, 0)
	appendPrepareTurnRenderBlock(&blocks, "related_memories", "Related Memories", "store.memories", assembly.MemoryText, intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0)), injectionEnabled, false, intFromAny(counts["top_k_memory_target"], 0))
	appendPrepareTurnRenderBlock(&blocks, "kg_relationships", "KG Relationships", "store.kg_triples", assembly.KGText, intFromAny(counts["kg_bound"], intFromAny(counts["kg_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "direct_evidence", "Direct Evidence", "store.direct_evidence_records", assembly.DirectEvidenceText, intFromAny(counts["direct_evidence_bound"], intFromAny(counts["vector_evidence_injected_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "fallback_chat_logs", "Fallback Chat Logs", "store.chat_logs", assembly.FallbackText, intFromAny(counts["fallback_bound"], intFromAny(counts["fallback_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "episode_summaries", "Episode Summaries", "store.episode_summaries", assembly.EpisodeText, intFromAny(counts["episode_bound"], intFromAny(counts["episode_summary_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "chapter_recall", "Chapter Recall", "store.chapter_summaries", assembly.ChapterText, boolToInt(strings.TrimSpace(assembly.ChapterText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "arc_recall", "Arc Recall", "store.arc_summaries", assembly.ArcText, boolToInt(strings.TrimSpace(assembly.ArcText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "saga_recall", "Saga Recall", "store.saga_digests", assembly.SagaText, boolToInt(strings.TrimSpace(assembly.SagaText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "storylines", "Ongoing Storylines", "store.storylines", assembly.StorylineText, intFromAny(counts["storyline_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "world_context", "World Context", "store.world_rules", assembly.WorldRulesText, intFromAny(counts["world_rule_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "character_states", "Character States", "store.character_states", assembly.CharacterText, intFromAny(counts["character_state_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "open_threads", "Open Threads", "store.pending_threads", assembly.PendingThreadText, intFromAny(counts["pending_thread_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "canonical_state_layer", "Canonical State Layer", "store.canonical_state_layers", assembly.CanonText, intFromAny(counts["canonical_layer_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "persona_recollection", "Persona Recollection", "store.persona_memory_entries", assembly.PersonaText, intFromAny(counts["persona_recollection_bound"], intFromAny(counts["persona_recollection_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "character_private_recollection", "Character Private Recollection", "store.protagonist_entity_memories", assembly.CharacterPrivateText, intFromAny(counts["character_private_recollection_bound"], intFromAny(counts["character_private_recollection_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "latest_direct_evidence", "Latest Direct Evidence", "store.direct_evidence_records", assembly.LatestDirectEvidenceText, boolToInt(strings.TrimSpace(assembly.LatestDirectEvidenceText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "recent_raw_turn", "Recent Raw Turn", "store.chat_logs", assembly.RecentRawTurnText, boolToInt(strings.TrimSpace(assembly.RecentRawTurnText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "scoped_verbatim_support", "Scoped Verbatim Support", "store.direct_evidence_records", assembly.ScopedVerbatimText, intFromAny(counts["scoped_verbatim_support_count"], 0), injectionEnabled, false, 0)
	counts["render_block_count"] = len(blocks)
	counts["render_included_block_count"] = prepareTurnIncludedRenderBlockCount(blocks)
	return map[string]any{
		"contract_version":        "input_transparency_render.v1",
		"status":                  status,
		"source":                  "prepare_turn_backend_render_model",
		"session_id":              sid,
		"turn_index":              turnIndex,
		"read_only":               true,
		"write_attempted":         false,
		"llm_call_attempted":      false,
		"raw_user_rewritten":      false,
		"injection_enabled":       injectionEnabled,
		"input_context_enabled":   inputContextEnabled,
		"injection_truncated":     assembly.Truncated,
		"input_context_truncated": inputContextTruncated,
		"fallback_reason":         nilIfEmpty(fallbackReason),
		"blocks":                  blocks,
		"counts":                  counts,
		"language_context":        nilIfEmptyMap(assembly.LanguageContext),
		"language_injection_trace": nilIfEmptyMap(
			assembly.LanguageInjectionTrace,
		),
		"perspective_context":   nilIfEmptyMap(assembly.PerspectiveContext),
		"secret_display_policy": "counts_only_no_secret_text",
	}
}

func buildPrepareTurnEffectiveInputPreview(sid string, turnIndex int, rawUserInput, requestType, applyMode, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded bool, fallbackReason string, assembly prepareTurnInjectionAssembly) map[string]any {
	status := prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled, assembly.Text, inputContextText)
	finalUserSource := "input_hook"
	if strings.TrimSpace(requestType) != "" && strings.TrimSpace(requestType) != "model" {
		finalUserSource = strings.TrimSpace(requestType)
	}
	return map[string]any{
		"contract_version":        "effective_input_preview.v1",
		"status":                  status,
		"source":                  "prepare_turn_backend_render_model",
		"session_id":              sid,
		"turn_index":              turnIndex,
		"payload_apply_mode":      strings.TrimSpace(applyMode),
		"final_user_source":       finalUserSource,
		"final_user_text":         rawUserInput,
		"final_user_chars":        len([]rune(rawUserInput)),
		"auxiliary_context_chars": len([]rune(assembly.Text)),
		"input_context_chars":     len([]rune(inputContextText)),
		"injection_applied":       injectionEnabled && strings.TrimSpace(assembly.Text) != "",
		"input_context_applied":   inputContextEnabled && strings.TrimSpace(inputContextText) != "",
		"injection_truncated":     assembly.Truncated,
		"input_context_truncated": inputContextTruncated,
		"raw_user_rewritten":      false,
		"read_only":               true,
		"write_attempted":         false,
		"llm_call_attempted":      false,
		"fallback_reason":         nilIfEmpty(fallbackReason),
		"counts":                  prepareTurnRenderCounts(assembly, inputContextText),
	}
}

func prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled bool, injectionText, inputContextText string) string {
	if degraded {
		return "degraded"
	}
	if !injectionEnabled && !inputContextEnabled {
		return "off"
	}
	if strings.TrimSpace(injectionText) == "" && strings.TrimSpace(inputContextText) == "" {
		return "empty"
	}
	return "ready"
}

func prepareTurnRenderCounts(assembly prepareTurnInjectionAssembly, inputContextText string) map[string]any {
	counts := map[string]any{}
	for k, v := range assembly.Counts {
		counts[k] = v
	}
	counts["vector_found"] = intFromAny(counts["vector_hit_count"], intFromAny(counts["vector_memory_hit_count"], 0)+intFromAny(counts["vector_evidence_hit_count"], 0)+intFromAny(counts["vector_world_rule_hit_count"], 0))
	counts["vector_hydrated"] = intFromAny(counts["vector_hydrated_count"], intFromAny(counts["vector_memory_hydrated_count"], 0)+intFromAny(counts["vector_evidence_hydrated_count"], 0)+intFromAny(counts["vector_world_rule_hydrated_count"], 0))
	counts["vector_selected"] = intFromAny(counts["vector_selected_count"], intFromAny(counts["vector_memory_selected_count"], 0)+intFromAny(counts["vector_evidence_selected_count"], 0)+intFromAny(counts["vector_world_rule_selected_count"], 0))
	counts["vector_injected"] = intFromAny(counts["vector_injected_count"], intFromAny(counts["vector_memory_injected_count"], 0)+intFromAny(counts["vector_evidence_injected_count"], 0)+intFromAny(counts["vector_world_rule_injected_count"], 0))
	counts["related_memory_count"] = intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0))
	if strings.TrimSpace(assembly.MemoryText) == "" {
		counts["memory_injected"] = 0
	} else {
		counts["memory_injected"] = intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0))
	}
	counts["auxiliary_context_chars"] = len([]rune(assembly.Text))
	counts["input_context_chars"] = len([]rune(inputContextText))
	counts["injection_truncated"] = assembly.Truncated
	return counts
}

func appendPrepareTurnRenderBlock(blocks *[]map[string]any, key, title, source, text string, count int, enabled, truncated bool, budget int) {
	status := "empty"
	if !enabled {
		status = "disabled"
	} else if strings.TrimSpace(text) != "" {
		status = "included"
	}
	block := map[string]any{
		"key":       key,
		"title":     title,
		"status":    status,
		"source":    source,
		"count":     count,
		"chars":     len([]rune(text)),
		"truncated": truncated,
		"text":      nilIfEmpty(text),
	}
	if budget > 0 {
		block["budget"] = budget
	}
	*blocks = append(*blocks, block)
}

func prepareTurnIncludedRenderBlockCount(blocks []map[string]any) int {
	count := 0
	for _, block := range blocks {
		if strings.TrimSpace(extractionStringFromAny(block["status"])) == "included" {
			count++
		}
	}
	return count
}

func nilIfEmpty(text string) any {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return text
}

func truncateTextForShadow(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	return truncateRunes(text, limit)
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func compactJSONForShadow(v any, limit int) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return truncateTextForShadow(string(data), limit)
}
