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
		case text == "":
			status = "deferred"
			if reason == "" {
				reason = "empty_guidance"
			}
			deferredCount++
		case chars+map[bool]int{true: 2, false: 0}[len(appliedGuidance) > 0] > remaining:
			status = "deferred"
			reason = "narrative_support_budget_exhausted"
			deferredCount++
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

func buildPrepareTurnRecomposerEnhancementContract(
	sessionID string,
	turnIndex int,
	memoryPlan map[string]any,
	memoryLineage map[string]any,
	payloadPlan map[string]any,
	supervisorCallStatus string,
) map[string]any {
	classCounts := map[string]int{}
	for _, rawClass := range prepareTurnMemoryLineageSlice(memoryPlan["classes"]) {
		class := mapFromAny(rawClass)
		key := extractionStringFromAny(class["key"])
		if key == "" {
			continue
		}
		classCounts[key] = intFromAny(class["selected_count"], 0)
	}
	countFor := func(keys ...string) int {
		total := 0
		for _, key := range keys {
			total += classCounts[key]
		}
		return total
	}
	feature := func(count int, sourceMode string) map[string]any {
		status := "empty"
		if count > 0 {
			status = "available"
		}
		return map[string]any{
			"status":         status,
			"selected_count": count,
			"source_mode":    sourceMode,
		}
	}

	objectiveCount := countFor(
		"event_recent",
		"character_objective",
		"world_state",
		"direct_evidence",
		"unresolved_goal",
	)
	subjectiveCount := countFor("subjective_relationship")
	protectedCount := countFor("protected_secret")
	guidanceTrace := mapFromAny(payloadPlan["guidance_application_trace"])
	supervisorCount := intFromAny(guidanceTrace["applied_count"], 0)
	directEvidenceCount := countFor("direct_evidence")

	supervisorFeature := feature(supervisorCount, "current_turn_supervisor_guidance")
	supervisorFeature["call_status"] = strings.TrimSpace(supervisorCallStatus)
	criticFeature := feature(directEvidenceCount, "prior_accepted_or_verified_direct_evidence")
	criticFeature["same_turn_result"] = false

	totalAvailable := objectiveCount + subjectiveCount + protectedCount + supervisorCount
	status := "ready"
	if totalAvailable == 0 {
		status = "empty"
	} else if supervisorCallStatus == "failed_open" || supervisorCallStatus == "malformed_failed_open" {
		status = "partial"
	}

	return map[string]any{
		"contract_version":                  "archive_center.recomposer_enhancement.v1",
		"status":                            status,
		"owner":                             "go",
		"read_only":                         true,
		"optional_enhancement":              true,
		"standalone_fallback_required":      true,
		"session_id":                        strings.TrimSpace(sessionID),
		"turn_index":                        turnIndex,
		"memory_plan_contract_version":      extractionStringFromAny(memoryPlan["contract_version"]),
		"memory_lineage_contract_version":   extractionStringFromAny(memoryLineage["contract_version"]),
		"same_turn_critic_result_available": false,
		"feature_status": map[string]any{
			"long_term_memory":        feature(objectiveCount, "go_selected_objective_and_grounded_memory"),
			"subjective_memory":       feature(subjectiveCount, "perspective_scoped_subjective_memory"),
			"protected_secret":        feature(protectedCount, "writer_only_secret_guard"),
			"supervisor_guidance":     supervisorFeature,
			"critic_curated_evidence": criticFeature,
		},
		"lane_semantics": map[string]any{
			"event_recent":            "objective_event_memory",
			"character_objective":     "objective_character_state",
			"subjective_relationship": "perspective_scoped_subjective",
			"world_state":             "objective_world_state",
			"protected_secret":        "writer_only",
			"unresolved_goal":         "open_thread_or_goal",
			"direct_evidence":         "accepted_or_verified_grounded_evidence",
			"output_guidance":         "supervisor_current_turn",
		},
		"privacy": map[string]any{
			"subjective_not_objective_truth": true,
			"protected_secret_writer_only":   true,
			"no_state_write":                 true,
		},
		"handoff": map[string]any{
			"mode":        "transient_current_turn_only",
			"text_source": "existing_go_memory_and_payload_plans",
			"copy_policy": "resolve_existing_plan_text_without_backend_reselection",
		},
	}
}

func prepareTurnTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func supervisorSceneProposalGuidanceItems(result map[string]any) []prepareTurnGuidanceItem {
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	if extractionStringFromAny(plan["contract_version"]) != "publisher_plan.v1" || extractionStringFromAny(plan["status"]) != "ready" {
		return nil
	}
	items := []prepareTurnGuidanceItem{}
	for _, raw := range outputFidelityLineageSlice(plan["guidance_items"]) {
		item := mapFromAny(raw)
		text := strings.TrimSpace(extractionFirstNonEmpty(extractionStringFromAny(item["render_text"]), extractionStringFromAny(item["text"])))
		itemRefs := stringSliceFromAny(item["source_refs"])
		slot := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item["slot"])))
		if slot == "" || text == "" || len(itemRefs) == 0 {
			continue
		}
		items = append(items, prepareTurnGuidanceItem{
			Key:        "publisher_" + slot,
			Title:      "Optional Publisher " + strings.ReplaceAll(slot, "_", " "),
			Text:       "[Optional Publisher " + strings.ReplaceAll(slot, "_", " ") + "]\n" + text,
			SourceRefs: itemRefs,
		})
	}
	return items
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
	MemoryRecallQuery         string
	ActualMemoryText          string
	ProtectedMemoryText       string
	MemoryDeliveryLineage     map[string]any
	MemoryDeliveryPlan        map[string]any
	CharacterMemorySupport    map[string]any
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
		"character_memory_support":              nilIfEmptyMap(assembly.CharacterMemorySupport),
		"language_context":                      nilIfEmptyMap(assembly.LanguageContext),
		"language_injection_trace":              nilIfEmptyMap(assembly.LanguageInjectionTrace),
		"perspective_context":                   nilIfEmptyMap(assembly.PerspectiveContext),
		"kg_text":                               nilIfEmpty(assembly.KGText),
		"direct_evidence_text":                  nilIfEmpty(assembly.DirectEvidenceText),
		"fallback_text":                         nilIfEmpty(assembly.FallbackText),
		"storyline_text":                        nilIfEmpty(assembly.StorylineText),
		"world_rules_text":                      nilIfEmpty(assembly.WorldRulesText),
		"character_text":                        nilIfEmpty(assembly.CharacterText),
		"character_objective_text":              nilIfEmpty(assembly.CharacterObjectiveText),
		"character_relationship_text":           nilIfEmpty(assembly.CharacterRelationshipText),
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
