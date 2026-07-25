package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
)

type prepareTurnGuidanceItem struct {
	Key        string
	Title      string
	Text       string
	SourceRefs []string
	Status     string
	ReasonCode string
}

func buildPrepareTurnPayloadApplicationPlan(rawUserInput, referenceText, memoryText, inputContextText string, injectionEnabled, inputContextEnabled bool, memoryBudget, referenceBudget, narrativeBudget int, guidanceItems []prepareTurnGuidanceItem, supervisorCallStatus string) map[string]any {
	if narrativeBudget < 0 {
		narrativeBudget = 0
	}
	if !injectionEnabled {
		narrativeBudget = 0
		guidanceItems = nil
	}
	remaining := narrativeBudget
	appliedGuidance := make([]string, 0, len(guidanceItems))
	appliedGuidanceRefs := []string{}
	guidanceTrace := make([]map[string]any, 0, len(guidanceItems))
	appliedCount := 0
	deferredCount := 0
	failedCount := 0
	guidanceBlocked := false
	for _, item := range guidanceItems {
		text := strings.TrimSpace(item.Text)
		status := strings.TrimSpace(item.Status)
		reason := strings.TrimSpace(item.ReasonCode)
		chars := len([]rune(text))
		if status == "" {
			status = "eligible"
		}
		switch {
		case status == "failed":
			failedCount++
		case guidanceBlocked:
			status = "deferred"
			if reason == "" {
				reason = "prior_guidance_not_applied"
			}
			deferredCount++
		case text == "":
			status = "deferred"
			if reason == "" {
				reason = "empty_guidance"
			}
			deferredCount++
			guidanceBlocked = true
		case chars+map[bool]int{true: 2, false: 0}[len(appliedGuidance) > 0] > remaining:
			status = "deferred"
			reason = "narrative_support_budget_exhausted"
			deferredCount++
			guidanceBlocked = true
		default:
			status = "applied"
			if len(appliedGuidance) > 0 {
				remaining -= 2
			}
			remaining -= chars
			appliedGuidance = append(appliedGuidance, text)
			for _, ref := range item.SourceRefs {
				appliedGuidanceRefs = appendUniqueMemorySearchText(appliedGuidanceRefs, ref)
			}
			appliedCount++
		}
		guidanceTrace = append(guidanceTrace, map[string]any{
			"key":          item.Key,
			"title":        item.Title,
			"status":       status,
			"reason_code":  nilIfEmpty(reason),
			"chars":        chars,
			"content_hash": prepareTurnTextHash(text),
			"source_refs":  item.SourceRefs,
		})
	}
	narrativeText := strings.Join(appliedGuidance, "\n\n")
	lanes := []map[string]any{
		prepareTurnPayloadLane("original_work", "Original Work Context", referenceText, referenceBudget, injectionEnabled && referenceText != "", nil),
		prepareTurnPayloadLane("long_term_memory", "Long-term Memory Context", memoryText, memoryBudget, injectionEnabled && memoryText != "", nil),
		prepareTurnPayloadLane("output_guidance", "Output Guidance Context", narrativeText, narrativeBudget, injectionEnabled && narrativeText != "", appliedGuidanceRefs),
	}
	auxiliaryParts := []string{}
	for _, lane := range lanes {
		if applied, _ := lane["applied"].(bool); applied {
			if text, _ := lane["text"].(string); strings.TrimSpace(text) != "" {
				auxiliaryParts = append(auxiliaryParts, text)
			}
		}
	}
	auxiliaryText := strings.Join(auxiliaryParts, "\n\n")
	if !injectionEnabled {
		auxiliaryText = ""
	}
	inputText := ""
	if inputContextEnabled {
		inputText = strings.TrimSpace(inputContextText)
	}
	usedNarrative := len([]rune(narrativeText))
	status := "ready"
	if auxiliaryText == "" && inputText == "" {
		status = "empty"
	}
	return map[string]any{
		"contract_version": "payload_application_plan.v1",
		"status":           status,
		"owner":            "go",
		"apply_rule":       "apply_exact_text_without_reassembly",
		"raw_user": map[string]any{
			"preserved":    true,
			"chars":        len([]rune(rawUserInput)),
			"content_hash": prepareTurnTextHash(rawUserInput),
		},
		"lane_order":          []string{"original_work", "long_term_memory", "output_guidance"},
		"lanes":               lanes,
		"auxiliary_text":      auxiliaryText,
		"auxiliary_chars":     len([]rune(auxiliaryText)),
		"auxiliary_hash":      prepareTurnTextHash(auxiliaryText),
		"input_context_text":  inputText,
		"input_context_chars": len([]rune(inputText)),
		"input_context_hash":  prepareTurnTextHash(inputText),
		"guidance_application_trace": map[string]any{
			"contract_version":       "guidance_application_trace.v1",
			"owner":                  "go",
			"budget_chars":           narrativeBudget,
			"used_chars":             usedNarrative,
			"remaining_chars":        maxInt(0, remaining),
			"applied_count":          appliedCount,
			"deferred_count":         deferredCount,
			"failed_count":           failedCount,
			"trimmed_count":          0,
			"mid_item_truncation":    false,
			"supervisor_call_status": supervisorCallStatus,
			"items":                  guidanceTrace,
			"final_text":             narrativeText,
			"final_hash":             prepareTurnTextHash(narrativeText),
		},
	}
}

func prepareTurnPayloadLane(key, title, text string, budget int, enabled bool, sourceRefs []string) map[string]any {
	text = strings.TrimSpace(text)
	applied := enabled && text != ""
	if !enabled {
		text = ""
	}
	status := "empty"
	if !enabled {
		status = "disabled"
	} else if applied {
		status = "applied"
	}
	return map[string]any{
		"key":            key,
		"title":          title,
		"status":         status,
		"applied":        applied,
		"budget_chars":   maxInt(0, budget),
		"used_chars":     len([]rune(text)),
		"deferred_chars": 0,
		"trimmed_chars":  0,
		"failed_chars":   0,
		"text":           text,
		"content_hash":   prepareTurnTextHash(text),
		"source_refs":    sourceRefs,
	}
}

func prepareTurnTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func formatSupervisorSceneProposalGuidance(result map[string]any) (string, []string) {
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if len(proposal) == 0 {
		return "", nil
	}
	lines := []string{"[Supervisor Proposal]", "Proposal only. Do not treat it as new facts, user actions, relationship changes, or event completion."}
	refs := []string{}
	for _, lane := range []struct {
		key   string
		label string
	}{
		{"fidelity_warnings", "Fidelity"},
		{"portrayal_notes", "Portrayal"},
	} {
		for _, raw := range sliceFromAny(proposal[lane.key]) {
			item := mapFromAny(raw)
			text := strings.TrimSpace(extractionStringFromAny(item["text"]))
			itemRefs := stringSliceFromAny(item["source_refs"])
			if text == "" || len(itemRefs) == 0 {
				continue
			}
			lines = append(lines, "- "+lane.label+": "+text+" (evidence: "+strings.Join(itemRefs, ", ")+")")
			refs = appendUniqueStringValues(refs, itemRefs...)
		}
	}
	if len(lines) == 2 {
		return "", nil
	}
	return strings.Join(lines, "\n"), refs
}

type prepareTurnInjectionBlock struct {
	Label   string
	Text    string
	Source  string
	Count   int
	Budget  int
	Trimmed bool
}

type prepareTurnInjectionAssembly struct {
	Text                      string
	SagaText                  string
	ChapterText               string
	MemoryText                string
	ActualMemoryText          string
	ProtectedMemoryText       string
	MemoryDeliveryLineage     map[string]any
	MemoryDeliveryPlan        map[string]any
	KGText                    string
	DirectEvidenceText        string
	FallbackText              string
	StorylineText             string
	WorldRulesText            string
	CharacterText             string
	CharacterObjectiveText    string
	CharacterRelationshipText string
	PendingThreadText         string
	EpisodeText               string
	PersonaText               string
	CharacterPrivateText      string
	ContinuityCorrectionText  string
	LatestDirectEvidenceText  string
	RecentRawTurnText         string
	ScopedVerbatimText        string
	ScopedVerbatimSupport     archivebridge.ScopedVerbatimSupport
	ArcText                   string
	CanonText                 string
	CanonEventText            string
	CanonCharacterText        string
	CanonRelationshipText     string
	CanonWorldText            string
	Truncated                 bool
	Blocks                    []prepareTurnInjectionBlock
	Trimmed                   []map[string]any
	BudgetDecisions           map[string]any
	Counts                    map[string]any
	LanguageContext           map[string]any
	LanguageInjectionTrace    map[string]any
	PerspectiveContext        map[string]any
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
		"protected_memory_text":                 nilIfEmpty(assembly.ProtectedMemoryText),
		"memory_delivery_lineage":               nilIfEmptyMap(assembly.MemoryDeliveryLineage),
		"memory_delivery_plan":                  nilIfEmptyMap(assembly.MemoryDeliveryPlan),
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
		"final_budget_owner":            "go_memory_delivery_plan",
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
	appendPrepareTurnRenderBlock(&blocks, "related_memories", "Related Memories", "store.memories", assembly.ActualMemoryText, intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0)), injectionEnabled, false, intFromAny(counts["top_k_memory_target"], 0))
	appendPrepareTurnRenderBlock(&blocks, "protected_memory_guidance", "Protected Memory Guidance", "go.memory_protection_policy", assembly.ProtectedMemoryText, intFromAny(counts["protected_memory_injected_line_count"], 0), injectionEnabled, false, intFromAny(counts["protected_secret_budget_chars"], 0))
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

func buildPrepareTurnEffectiveInputPreview(sid string, turnIndex int, rawUserInput, finalUserSource, requestType, applyMode, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded bool, fallbackReason string, assembly prepareTurnInjectionAssembly) map[string]any {
	status := prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled, assembly.Text, inputContextText)
	if strings.TrimSpace(finalUserSource) == "" {
		finalUserSource = "go_current_input_decision"
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

func nilIfEmptyMap(value map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	return value
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
