package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildPrepareTurnInjectionAssembly(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, resumePack *store.ResumePack, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, topK, maxChars int, rawUserInput, profile string, documents []map[string]any, vectorShadow map[string]any, languageContext map[string]any, perspectiveContextArg ...map[string]any) prepareTurnInjectionAssembly {
	topK = prepareTurnRecallLimit(topK)
	maxChars = prepareTurnTextBudget(maxChars)
	recallLimit := prepareTurnSupportRecallLimit(topK)
	languageContext = normalizeCompleteTurnLanguageContext(languageContext)
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	narrativeCurrentValues, activeStates := prepareTurnNarrativeStateFromPerspective(perspectiveContextArg)

	out := prepareTurnInjectionAssembly{
		LanguageContext:    languageContext,
		PerspectiveContext: perspectiveContext,
		Counts: map[string]any{
			"memory_count":                         len(memories),
			"kg_count":                             len(kgTriples),
			"fallback_chat_log_count":              len(chatLogs),
			"evidence_count":                       len(evidence),
			"storyline_count":                      len(storylines),
			"world_rule_count":                     len(worldRules),
			"character_state_count":                len(charStates),
			"pending_thread_count":                 len(pendingThreads),
			"canonical_layer_count":                len(canonicalLayers),
			"episode_summary_count":                len(episodeSums),
			"persona_recollection_count":           len(personaEntries),
			"character_private_recollection_count": len(characterPrivateMemories),
			"scoped_verbatim_support_count":        0,
			"top_k_memory_target":                  topK,
			"support_recall_limit":                 recallLimit,
			"support_recall_limit_source":          "requested_top_k_bounded_by_final_injection_budget",
			"top_k_definition":                     "semantic_memory_recall_limit",
		},
	}

	memoryQuery := prepareTurnMemorySelectionQuery(rawUserInput, chatLogs, perspectiveContext, topK)
	memorySelection := selectPrepareTurnMemoryLanesWithVector(memories, memoryQuery, topK, vectorShadow)
	memorySelection = collapsePrepareTurnMemoryLaneSelection(memorySelection)
	memorySelection = filterPrepareTurnProtectedMemoryLaneSelection(memorySelection, rawUserInput, chatLogs, perspectiveContext)
	out.ContinuityCorrectionText, out.Counts["continuity_correction"] = buildNarrativeContinuityCorrection(
		narrativeCurrentValues,
		rawUserInput,
		chatLogs,
		activeStates,
		memorySelection,
		recallLimit,
	)
	memorySelection = filterMemorySelectionAgainstNarrativeCurrentState(memorySelection, narrativeCurrentValues)
	memoryLines, memoryLanguageTrace := prepareTurnMemoryLaneLines(memorySelection, languageContext, perspectiveContext)
	for k, v := range prepareTurnMemoryLaneProtectedCounts(memorySelection, perspectiveContext) {
		out.Counts[k] = v
	}
	artifactHydration := prepareTurnHydrateVectorArtifactHits(evidence, worldRules, vectorShadow, recallLimit)
	out.LanguageInjectionTrace = buildPrepareTurnLanguageInjectionTrace(languageContext, memoryLanguageTrace)
	out.MemoryText = makePrepareTurnSection("[Memory]", memoryLines)

	kgLines := make([]string, 0, minInt(len(kgTriples), recallLimit))
	kgClosedDropped := 0
	kgReferenceTurn := prepareTurnMaxObservedTurn(chatLogs, nil)
	for _, t := range kgTriples {
		if len(kgLines) >= recallLimit {
			break
		}
		if kgReferenceTurn > 0 && ((t.ValidTo > 0 && t.ValidTo < kgReferenceTurn) || (t.ValidFrom > 0 && t.ValidFrom > kgReferenceTurn)) {
			kgClosedDropped++
			continue
		}
		line := strings.TrimSpace(fmt.Sprintf("%s --%s--> %s", t.Subject, t.Predicate, t.Object))
		if line == "-->" {
			continue
		}
		kgLines = append(kgLines, line)
	}
	out.KGText = makePrepareTurnSection("[Knowledge Graph]", kgLines)

	directEvidenceLines := make([]string, 0, len(artifactHydration.Evidence))
	for _, ev := range artifactHydration.Evidence {
		text := compactPrepareTurnLine(ev.EvidenceText, 320)
		if text == "" {
			continue
		}
		meta := []string{"vector"}
		if ev.TurnAnchor > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", ev.TurnAnchor))
		} else if ev.SourceTurnEnd > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", ev.SourceTurnEnd))
		}
		directEvidenceLines = append(directEvidenceLines, fmt.Sprintf("- [%s] %s", strings.Join(meta, ", "), text))
	}
	out.DirectEvidenceText = makePrepareTurnSection("[Direct Evidence]", directEvidenceLines)

	fallbackLines := []string{}
	if prepareTurnNeedsRawFallback(memorySelection, topK) && len(chatLogs) > 0 {
		for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
			content := compactPrepareTurnLine(cl.Content, maxChars)
			if content == "" {
				continue
			}
			role := strings.TrimSpace(cl.Role)
			if role == "" {
				role = "unknown"
			}
			fallbackLines = append(fallbackLines, fmt.Sprintf("- %s: %s", role, content))
		}
	}
	out.FallbackText = makePrepareTurnSection("[Fallback Recent Chat]", fallbackLines)

	storylinesForInjection := collapsePrepareTurnStorylines(storylines)
	storylineLines := make([]string, 0, minInt(len(storylinesForInjection), recallLimit))
	for i, sl := range storylinesForInjection {
		if i >= recallLimit {
			break
		}
		desc := strings.TrimSpace(sl.CurrentContext)
		if desc == "" {
			desc = strings.TrimSpace(sl.Name)
		}
		desc = compactPrepareTurnLine(desc, 170)
		if desc != "" {
			storylineLines = append(storylineLines, "- "+desc)
		}
	}
	out.StorylineText = makePrepareTurnSection("[Storylines]", storylineLines)

	worldRulesForInjection := collapsePrepareTurnWorldRules(mergePrepareTurnWorldRulesForInjection(artifactHydration.WorldRules, worldRules))
	worldRuleLines := make([]string, 0, minInt(len(worldRulesForInjection), recallLimit))
	for i, wr := range worldRulesForInjection {
		if i >= recallLimit {
			break
		}
		desc := strings.TrimSpace(wr.Key)
		if desc == "" {
			desc = strings.TrimSpace(wr.Scope)
		}
		if value := compactPrepareTurnLine(wr.ValueJSON, 120); value != "" {
			desc = strings.TrimSpace(desc + ": " + value)
		}
		desc = compactPrepareTurnLine(desc, 180)
		if desc != "" {
			worldRuleLines = append(worldRuleLines, "- "+desc)
		}
	}
	out.WorldRulesText = makePrepareTurnSection("[World Rules]", worldRuleLines)

	charLines := make([]string, 0, minInt(len(charStates), recallLimit))
	for i, cs := range charStates {
		if i >= recallLimit {
			break
		}
		name := strings.TrimSpace(cs.CharacterName)
		state := prepareTurnSurfaceText(parseSurfacePayload(cs.StatusJSON))
		relationships := prepareTurnSurfaceText(parseSurfacePayload(cs.RelationshipsJSON))
		speechStyle := prepareTurnSurfaceText(parseSurfacePayload(cs.SpeechStyleJSON))
		parts := []string{}
		if state != "" {
			parts = append(parts, "state="+state)
		}
		if speechStyle != "" {
			parts = append(parts, "speech_style="+speechStyle)
		}
		if relationships != "" {
			parts = append(parts, "relationships="+relationships)
		}
		detail := compactPrepareTurnLine(strings.Join(parts, "; "), 520)
		if name == "" && detail == "" {
			continue
		}
		charLines = append(charLines, fmt.Sprintf("- %s: %s", name, detail))
	}
	out.CharacterText = makePrepareTurnSection("[Characters]", charLines)

	pendingLines := make([]string, 0, minInt(len(pendingThreads), recallLimit))
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		desc := compactPrepareTurnLine(pt.Description, 170)
		status := strings.TrimSpace(pt.Status)
		if status != "" && desc != "" {
			desc = compactPrepareTurnLine("status="+status+"; "+desc, 190)
		}
		if desc != "" {
			pendingLines = append(pendingLines, "- "+desc)
		}
	}
	out.PendingThreadText = makePrepareTurnSection("[Pending Threads]", pendingLines)

	episodeLines := make([]string, 0, minInt(len(episodeSums), recallLimit))
	for i, es := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := compactPrepareTurnLine(es.SummaryText, 180)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
		}
		if anchors := episodeDenseAnchorPreview(es, 260); anchors != "" {
			summary = compactPrepareTurnLine(summary+"; "+anchors, 360)
		}
		episodeLines = append(episodeLines, fmt.Sprintf("- turns %d-%d: %s", es.FromTurn, es.ToTurn, summary))
	}
	out.EpisodeText = makePrepareTurnSection("[Episode Summaries]", episodeLines)
	hierarchyEscalation := buildPrepareTurnHierarchyEscalation(resumePack, chatLogs, memorySelection, topK, rawUserInput, profile)
	out.ChapterText = hierarchyEscalation.ChapterText
	out.ArcText = hierarchyEscalation.ArcText
	out.SagaText = hierarchyEscalation.SagaText
	out.PersonaText = buildPersonaRecollectionText(personaEntries, recallLimit, maxChars)
	out.CharacterPrivateText = buildCharacterPrivateRecollectionText(characterPrivateMemories, recallLimit, maxChars)

	if latest := latestPrepareTurnEvidence(evidence); latest != nil {
		out.LatestDirectEvidenceText = compactPrepareTurnLine(latest.EvidenceText, 260)
	}
	out.RecentRawTurnText = recentPrepareTurnRawTurn(chatLogs, topK)
	out.ScopedVerbatimSupport = archivebridge.BuildScopedVerbatimSupport(evidence)
	out.ScopedVerbatimText = out.ScopedVerbatimSupport.Text

	canonLines := make([]string, 0, minInt(len(canonicalLayers), recallLimit))
	canonFiltered := 0
	canonTypeCounts := map[string]int{}
	for i, cl := range canonicalLayers {
		if i >= recallLimit {
			break
		}
		if !canonicalLayerEligibleForCurrentTruth(cl) {
			canonFiltered++
			continue
		}
		content := compactPrepareTurnLine(cl.Content, 180)
		if content == "" {
			continue
		}
		layer := strings.TrimSpace(cl.LayerType)
		if layer == "" {
			layer = "state"
		}
		canonLines = append(canonLines, fmt.Sprintf("- %s: %s", layer, content))
		canonTypeCounts[layer]++
	}
	out.CanonText = makePrepareTurnSection("[Canonical State]", canonLines)

	addPrepareTurnBlock(&out, "memory", "store.memories", out.MemoryText, len(memoryLines), maxChars)
	addPrepareTurnBlock(&out, "kg", "store.kg_triples", out.KGText, len(kgLines), maxChars)
	addPrepareTurnBlock(&out, "direct_evidence", "store.direct_evidence_records", out.DirectEvidenceText, len(directEvidenceLines), maxChars)
	addPrepareTurnBlock(&out, "fallback", "store.chat_logs", out.FallbackText, len(fallbackLines), maxChars)
	addPrepareTurnBlock(&out, "episode", "store.episode_summaries", out.EpisodeText, len(episodeLines), maxChars)
	addPrepareTurnBlock(&out, "chapter", "store.chapter_summaries", out.ChapterText, boolToInt(strings.TrimSpace(out.ChapterText) != ""), maxChars)
	addPrepareTurnBlock(&out, "arc", "store.arc_summaries", out.ArcText, boolToInt(strings.TrimSpace(out.ArcText) != ""), maxChars)
	addPrepareTurnBlock(&out, "saga", "store.saga_digests", out.SagaText, boolToInt(strings.TrimSpace(out.SagaText) != ""), maxChars)
	addPrepareTurnBlock(&out, "storyline", "store.storylines", out.StorylineText, len(storylineLines), maxChars)
	addPrepareTurnBlock(&out, "world_rules", "store.world_rules", out.WorldRulesText, len(worldRuleLines), maxChars)
	addPrepareTurnBlock(&out, "character", "store.character_states", out.CharacterText, len(charLines), maxChars)
	addPrepareTurnBlock(&out, "pending_thread", "store.pending_threads", out.PendingThreadText, len(pendingLines), maxChars)
	addPrepareTurnBlock(&out, "canonical_state_layer", "store.canonical_state_layers", out.CanonText, len(canonLines), maxChars)
	addPrepareTurnBlock(&out, "persona_recollection", "store.persona_memory_entries", out.PersonaText, minInt(len(personaEntries), recallLimit), maxChars)
	addPrepareTurnBlock(&out, "character_private_recollection", "store.protagonist_entity_memories", out.CharacterPrivateText, minInt(len(characterPrivateMemories), recallLimit), maxChars)
	addPrepareTurnBlock(&out, "continuity_correction", "store.status_current_values", out.ContinuityCorrectionText, intFromAny(mapFromAny(out.Counts["continuity_correction"])["selected_count"], 0), maxChars)

	parts := make([]string, 0, len(out.Blocks))
	for _, block := range out.Blocks {
		if strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	out.Text = strings.Join(parts, "\n")
	if len([]rune(out.Text)) > maxChars {
		out.Text = truncateRunes(out.Text, maxChars)
		out.Truncated = true
		out.Trimmed = append(out.Trimmed, map[string]any{
			"label":  "overall",
			"reason": "max_injection_chars",
			"budget": maxChars,
		})
	}

	out.Counts["memory_bound"] = len(memoryLines)
	out.Counts["memory_count"] = len(memoryLines)
	out.Counts["top_k_memory_target"] = topK
	out.Counts["support_recall_limit"] = recallLimit
	out.Counts["support_recall_limit_source"] = "requested_top_k_bounded_by_final_injection_budget"
	out.Counts["top_k_definition"] = "semantic_memory_recall_limit"
	out.Counts["recent_memory_bound"] = len(memorySelection.Recent)
	out.Counts["vector_memory_bound"] = len(memorySelection.VectorRelevant)
	out.Counts["relevant_memory_bound"] = len(memorySelection.Relevant)
	out.Counts["deep_memory_bound"] = len(memorySelection.Deep)
	out.Counts["memory_recall_lane_policy"] = memorySelection.Trace
	mergePrepareTurnMemoryLaneCounters(out.Counts, memorySelection, strings.TrimSpace(out.MemoryText) != "")
	mergePrepareTurnVectorArtifactCounters(out.Counts, artifactHydration, strings.TrimSpace(out.DirectEvidenceText) != "", len(directEvidenceLines), len(worldRuleLines))
	out.Counts["language_aware_injection"] = out.LanguageInjectionTrace
	if memoryTrace := mapFromAny(out.LanguageInjectionTrace["memory_language_trace"]); len(memoryTrace) > 0 {
		out.Counts["memory_summary_language_match"] = intFromAny(memoryTrace["memory_summary_language_match"], 0)
		out.Counts["memory_summary_language_mismatch"] = intFromAny(memoryTrace["memory_summary_language_mismatch"], 0)
		out.Counts["raw_evidence_attached_count"] = intFromAny(memoryTrace["raw_evidence_attached_count"], 0)
	}
	out.Counts["kg_bound"] = len(kgLines)
	out.Counts["kg_closed_or_not_yet_valid_dropped"] = kgClosedDropped
	out.Counts["direct_evidence_bound"] = len(directEvidenceLines)
	out.Counts["fallback_bound"] = len(fallbackLines)
	out.Counts["fallback_count"] = len(fallbackLines)
	out.Counts["episode_bound"] = len(episodeLines)
	out.Counts["chapter_delivered"] = strings.TrimSpace(out.ChapterText) != ""
	out.Counts["arc_delivered"] = strings.TrimSpace(out.ArcText) != ""
	out.Counts["saga_delivered"] = strings.TrimSpace(out.SagaText) != ""
	out.Counts["chapter_chars"] = len([]rune(strings.TrimSpace(out.ChapterText)))
	out.Counts["arc_chars"] = len([]rune(strings.TrimSpace(out.ArcText)))
	out.Counts["saga_chars"] = len([]rune(strings.TrimSpace(out.SagaText)))
	out.Counts["hierarchy_escalation"] = hierarchyEscalation.Trace
	out.Counts["persona_recollection_bound"] = minInt(len(personaEntries), recallLimit)
	out.Counts["persona_recollection_support_only"] = len(personaEntries) > 0
	out.Counts["character_private_recollection_bound"] = minInt(len(characterPrivateMemories), recallLimit)
	out.Counts["character_private_recollection_private_lane"] = len(characterPrivateMemories) > 0
	out.Counts["scoped_verbatim_support_count"] = out.ScopedVerbatimSupport.Count
	out.Counts["verbatim_support_active"] = out.ScopedVerbatimSupport.Active
	out.Counts["canonical_state_layers_filtered_count"] = canonFiltered
	out.Counts["canonical_state_relationship_layers_count"] = canonTypeCounts["relationship_state"]
	out.Counts["canonical_state_world_layers_count"] = canonTypeCounts["world_state"]
	out.Counts["canonical_state_scene_layers_count"] = canonTypeCounts["scene_state"]
	out.Counts["canonical_state_entity_layers_count"] = canonTypeCounts["entity_state"]
	out.Counts["storyline_collapsed_count"] = maxInt(len(storylines)-len(storylinesForInjection), 0)
	out.Counts["world_rule_collapsed_count"] = maxInt(len(worldRules)-len(worldRulesForInjection), 0)
	out.Counts["block_count"] = len(out.Blocks)
	out.Counts["total_chars"] = len([]rune(out.Text))
	out.BudgetDecisions = map[string]any{
		"policy_version":                              "rmg07.prepare_turn.bundle.v1",
		"max_injection_chars":                         maxChars,
		"final_budget_owner":                          "archive_center_js_assembleInjectionWithBudget",
		"fallback_chat_log_included":                  strings.TrimSpace(out.FallbackText) != "",
		"fallback_reason":                             fallbackReasonForPrepareTurn(len(memoryLines), len(chatLogs), topK),
		"verbatim_support_active":                     out.ScopedVerbatimSupport.Active,
		"verbatim_support_policy_version":             out.ScopedVerbatimSupport.PolicyVersion,
		"section_count":                               len(out.Blocks),
		"trimmed_count":                               len(out.Trimmed),
		"status_vocabulary":                           []string{"off", "skeleton", "partial", "ready", "degraded"},
		"canonical_state_hard_floor_enabled":          true,
		"persona_recollection_support_only":           len(personaEntries) > 0,
		"persona_recollection_priority":               "below_current_user_input_direct_evidence_and_canonical_state",
		"character_private_recollection_private_lane": len(characterPrivateMemories) > 0,
		"character_private_recollection_visibility":   "owner_private_not_player_visible_by_default",
		"hierarchy_escalation":                        hierarchyEscalation.Trace,
		"hierarchy_priority":                          "support_only_below_current_user_direct_evidence_and_canonical_state",
		"t1a_enforced_ready":                          true,
		"t1a_transition":                              "policy_only_to_enforced_shadow",
	}
	bd := buildBudgetDecisions(documents, maxChars, recallLimit)
	for k, v := range bd {
		out.BudgetDecisions[k] = v
	}
	out.Counts["relationship_first_budget"] = map[string]any{
		"version":          "p80a.v1",
		"status":           "shadow_only",
		"structure":        "relationship_first",
		"long_tier_cap":    2400,
		"ultra_tier_cap":   1800,
		"extreme_tier_cap": 1200,
		"reason":           "relationship_first_budget_structure_for_long_tier_profiles",
	}
	return out
}

func buildBudgetDecisions(docs []map[string]any, maxChars, recallLimit int) map[string]any {
	status := "ready"
	if len(docs) == 0 {
		status = "off"
	}
	recallLimit = prepareTurnRecallLimit(recallLimit)

	globalCap := prepareTurnTextBudget(maxChars)

	canonHardFloor := 120
	policy := q3PacketBudgetPolicy()
	if caps, ok := policy["budget_caps"].(map[string]any); ok {
		if v, ok := caps["canon_hard_floor"].(int); ok && v > 0 {
			canonHardFloor = v
		}
	}

	intentDefs := []struct {
		name  string
		tiers []string
	}{
		{"scene", []string{"memory", "episode", "chapter"}},
		{"callback", []string{"arc", "saga", "memory"}},
		{"resume", []string{"chapter", "arc", "saga"}},
		{"canon", []string{"memory", "episode", "arc"}},
	}

	intentCapRatios := map[string]float64{
		"scene":    0.40,
		"callback": 0.25,
		"resume":   0.20,
		"canon":    0.15,
	}

	decisions := []map[string]any{}
	globalSelectedChars := 0
	canonSelectedChars := 0
	reasonCounts := map[string]int{"tier_cap": 0}
	for _, def := range intentDefs {
		capChars := int(float64(globalCap) * intentCapRatios[def.name])

		candidates := []map[string]any{}
		for _, doc := range docs {
			tier, _ := doc["tier"].(string)
			for _, allowed := range def.tiers {
				if tier == allowed {
					candidates = append(candidates, doc)
					break
				}
			}
		}

		runningTotal := 0
		for i, cand := range candidates {
			id, _ := cand["document_id"].(string)
			text, _ := cand["text"].(string)
			tier, _ := cand["tier"].(string)
			charCost := len([]rune(text))

			decision := "selected"
			reason := "tier_match_selected"
			if i >= recallLimit {
				decision = "dropped"
				reason = "tier_cap_exceeded"
				reasonCounts["tier_cap"]++
			} else {
				runningTotal += charCost
				globalSelectedChars += charCost
				if def.name == "canon" {
					canonSelectedChars += charCost
				}
			}

			decisions = append(decisions, map[string]any{
				"intent":              def.name,
				"tier":                tier,
				"document_id":         id,
				"decision":            decision,
				"reason":              reason,
				"cap_scope":           def.name,
				"char_cost":           charCost,
				"running_total_chars": runningTotal,
				"cap_chars":           capChars,
			})
		}
	}

	return map[string]any{
		"version":                    "t1c.v1",
		"mode":                       "read_only_surface",
		"status":                     status,
		"decision_count":             len(decisions),
		"decisions":                  decisions,
		"global_cap_chars":           globalCap,
		"global_selected_chars":      globalSelectedChars,
		"canon_floor_reserved_chars": canonHardFloor,
		"canon_selected_chars":       canonSelectedChars,
		"reason_counts":              reasonCounts,
		"source_mapping":             "recall_result.intent_execution_shadow.budget_enforcement",
		"source_event":               "budget_enforcement",
		"source_counters":            []string{"decision_count", "global_cap_chars", "global_selected_chars", "canon_floor_reserved_chars", "canon_selected_chars", "reason_counts"},
	}
}

func buildHierarchyEscapeHatch(support archivebridge.ScopedVerbatimSupport) map[string]any {
	status := "inactive"
	reason := "no_direct_evidence_support"
	if support.Active {
		status = "active"
		reason = "support_route_available"
	}
	return map[string]any{
		"status":        status,
		"route":         "scoped_verbatim_support",
		"reason":        reason,
		"support_count": support.Count,
	}
}

func addPrepareTurnBlock(out *prepareTurnInjectionAssembly, label, source, text string, count, budget int) {
	text = strings.TrimSpace(text)
	if text == "" || budget <= 0 {
		return
	}
	out.Blocks = append(out.Blocks, prepareTurnInjectionBlock{
		Label:   label,
		Text:    text,
		Source:  source,
		Count:   count,
		Budget:  budget,
		Trimmed: false,
	})
	switch label {
	case "memory":
		out.MemoryText = text
	case "kg":
		out.KGText = text
	case "fallback":
		out.FallbackText = text
	case "episode":
		out.EpisodeText = text
	case "chapter":
		out.ChapterText = text
	case "arc":
		out.ArcText = text
	case "saga":
		out.SagaText = text
	case "storyline":
		out.StorylineText = text
	case "world_rules":
		out.WorldRulesText = text
	case "character":
		out.CharacterText = text
	case "pending_thread":
		out.PendingThreadText = text
	case "canonical_state_layer":
		out.CanonText = text
	case "persona_recollection":
		out.PersonaText = text
	case "character_private_recollection":
		out.CharacterPrivateText = text
	}
}

func makePrepareTurnSection(header string, lines []string) string {
	cleaned := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return header + "\n" + strings.Join(cleaned, "\n")
}

func prepareTurnMemorySummary(m store.Memory) string {
	summary := strings.TrimSpace(m.SummaryJSON)
	if summary == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(summary), &parsed); err == nil {
		for _, key := range []string{"turn_summary", "summary", "content", "text"} {
			if s, ok := parsed[key].(string); ok && strings.TrimSpace(s) != "" {
				summary = s
				break
			}
		}
	}
	placeParts := []string{}
	if wing := strings.TrimSpace(m.PlaceWing); wing != "" {
		placeParts = append(placeParts, "archive_wing="+wing)
	}
	if room := strings.TrimSpace(m.PlaceRoom); room != "" {
		placeParts = append(placeParts, "archive_room="+room)
	}
	if len(placeParts) > 0 {
		summary = strings.TrimSpace(summary + " (" + strings.Join(placeParts, ", ") + ")")
	}
	return compactPrepareTurnLine(summary, 220)
}

func prepareTurnMemoryRelevanceText(m store.Memory) string {
	searchText := strings.TrimSpace(memorySearchTextFromMemory(m).Text)
	summary := strings.TrimSpace(prepareTurnMemorySummary(m))
	if searchText == "" {
		return summary
	}
	if summary == "" || strings.Contains(searchText, summary) {
		return searchText
	}
	return strings.TrimSpace(summary + "\n" + searchText)
}

func compactPrepareTurnLine(text string, limit int) string {
	_ = limit
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	return text
}

func prepareTurnSurfaceText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return compactPrepareTurnLine(v, 0)
	case float64, bool, int, int64:
		return compactPrepareTurnJSON(v)
	case map[string]any, []any:
		return compactPrepareTurnJSON(v)
	default:
		return compactPrepareTurnJSON(v)
	}
}

func compactPrepareTurnJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return compactPrepareTurnLine(string(data), 0)
}

func episodeDenseAnchorPreview(es store.EpisodeSummary, limit int) string {
	parts := []string{}
	if key := compactEpisodeJSONPreview(es.KeyEvents, 120); key != "" {
		parts = append(parts, "key_event="+key)
	}
	if rel := compactEpisodeJSONPreview(es.RelationshipChangesJSON, 120); rel != "" {
		parts = append(parts, "rel="+rel)
	}
	if loop := compactEpisodeJSONPreview(es.OpenLoopsJSON, 120); loop != "" {
		parts = append(parts, "open_loop="+loop)
	}
	return compactPrepareTurnLine(strings.Join(parts, "; "), limit)
}

func compactEpisodeJSONPreview(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" || raw == "null" {
		return ""
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		parts := []string{}
		for _, item := range arr {
			text := strings.TrimSpace(extractionStringFromAny(item))
			if text == "" {
				text = strings.TrimSpace(compactPrepareTurnJSON(item))
			}
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return compactPrepareTurnLine(strings.Join(parts, " / "), limit)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		return compactPrepareTurnLine(compactPrepareTurnJSON(obj), limit)
	}
	return compactPrepareTurnLine(raw, limit)
}

func latestPrepareTurnEvidence(evidence []store.DirectEvidence) *store.DirectEvidence {
	var latest *store.DirectEvidence
	latestTurn := -1
	for i := range evidence {
		if evidence[i].Tombstoned || strings.TrimSpace(evidence[i].EvidenceText) == "" {
			continue
		}
		turn := maxInt(evidence[i].TurnAnchor, maxInt(evidence[i].SourceTurnEnd, evidence[i].SourceTurnStart))
		if latest == nil || turn >= latestTurn {
			latest = &evidence[i]
			latestTurn = turn
		}
	}
	return latest
}

func recentPrepareTurnRawTurn(chatLogs []store.ChatLog, turnLimit int) string {
	if len(chatLogs) == 0 {
		return ""
	}
	turnLimit = prepareTurnRecallLimit(turnLimit)
	selected := selectRecentChatLogsByTurn(chatLogs, turnLimit)
	lines := make([]string, 0, len(selected))
	for _, cl := range selected {
		content := compactPrepareTurnLine(cl.Content, 0)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(cl.Role)
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
}

func selectRecentChatLogsByTurn(chatLogs []store.ChatLog, turnLimit int) []store.ChatLog {
	if len(chatLogs) == 0 || turnLimit <= 0 {
		return nil
	}
	turns := map[int]bool{}
	for i := len(chatLogs) - 1; i >= 0 && len(turns) < turnLimit; i-- {
		turn := chatLogs[i].TurnIndex
		if turn <= 0 {
			continue
		}
		turns[turn] = true
	}
	out := make([]store.ChatLog, 0, minInt(len(chatLogs), turnLimit*2))
	for _, cl := range chatLogs {
		if !turns[cl.TurnIndex] {
			continue
		}
		out = append(out, cl)
	}
	return out
}

func fallbackReasonForPrepareTurn(memoryBound, chatLogCount, topK int) string {
	if memoryBound >= prepareTurnRecallLimit(topK) {
		return "memory_sufficient"
	}
	if chatLogCount > 0 {
		return "memory_below_threshold"
	}
	return "no_chat_log_fallback_available"
}

func buildInputContextText(evidence []store.DirectEvidence, chatLogs []store.ChatLog, resumePack *store.ResumePack, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, maxChars, recallLimit int) (string, bool) {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	maxChars = prepareTurnTextBudget(maxChars)
	var parts []string

	// [Resume Pack]
	if resumePack != nil && resumePack.AssembledText != "" {
		parts = append(parts, "[Resume Pack]")
		text := strings.Join(strings.Fields(resumePack.AssembledText), " ")
		parts = append(parts, text)
	}

	// [Direct Evidence]
	if len(evidence) > 0 {
		var evParts []string
		for _, e := range evidence {
			txt := strings.TrimSpace(e.EvidenceText)
			if txt == "" {
				continue
			}
			txt = strings.Join(strings.Fields(txt), " ")
			evParts = append(evParts, "- "+txt)
			if len(evParts) >= recallLimit {
				break
			}
		}
		if len(evParts) > 0 {
			parts = append(parts, "[Direct Evidence]")
			parts = append(parts, evParts...)
		}
	}

	// [Recent Chat]
	if len(chatLogs) > 0 {
		var logParts []string
		for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
			content := strings.TrimSpace(cl.Content)
			if content == "" {
				continue
			}
			content = strings.Join(strings.Fields(content), " ")
			logParts = append(logParts, fmt.Sprintf("- [%s] %s", cl.Role, content))
		}
		if len(logParts) > 0 {
			parts = append(parts, "[Recent Chat]")
			parts = append(parts, logParts...)
		}
	}

	// [Active States]
	if len(activeStates) > 0 {
		var asParts []string
		for i, as := range activeStates {
			if i >= recallLimit {
				break
			}
			content := strings.TrimSpace(as.Content)
			content = strings.Join(strings.Fields(content), " ")
			asParts = append(asParts, fmt.Sprintf("- [%s] %s", as.StateType, content))
		}
		if len(asParts) > 0 {
			parts = append(parts, "[Active States]")
			parts = append(parts, asParts...)
		}
	}

	// [Canonical State Layers]
	if len(canonicalLayers) > 0 {
		var clParts []string
		for i, cl := range canonicalLayers {
			if i >= recallLimit {
				break
			}
			content := strings.TrimSpace(cl.Content)
			content = strings.Join(strings.Fields(content), " ")
			clParts = append(clParts, fmt.Sprintf("- [%s] %s", cl.LayerType, content))
		}
		if len(clParts) > 0 {
			parts = append(parts, "[Canonical State Layers]")
			parts = append(parts, clParts...)
		}
	}

	// [Episode Summaries]
	if len(episodeSums) > 0 {
		var esParts []string
		for i, es := range episodeSums {
			if i >= recallLimit {
				break
			}
			summary := strings.TrimSpace(es.SummaryText)
			if summary == "" {
				summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
			}
			summary = strings.Join(strings.Fields(summary), " ")
			esParts = append(esParts, "- "+summary)
		}
		if len(esParts) > 0 {
			parts = append(parts, "[Episode Summaries]")
			parts = append(parts, esParts...)
		}
	}

	if personaText := buildPersonaRecollectionText(personaEntries, recallLimit, maxChars); personaText != "" {
		parts = append(parts, personaText)
	}
	if privateText := buildCharacterPrivateRecollectionText(characterPrivateMemories, recallLimit, maxChars); privateText != "" {
		parts = append(parts, privateText)
	}

	text := strings.Join(parts, "\n")
	truncated := false
	if len([]rune(text)) > maxChars {
		text = truncateRunes(text, maxChars)
		truncated = true
	}
	return text, truncated
}
