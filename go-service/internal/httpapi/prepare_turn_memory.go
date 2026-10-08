package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnMemoryLaneLines(selection prepareTurnMemoryLaneSelection, languageContext map[string]any, canonicalMemories []store.Memory, perspectiveContextArg ...map[string]any) ([]string, map[string]any) {
	return prepareTurnMemoryLaneLinesPrepared(selection, languageContext, canonicalMemories, nil, perspectiveContextArg...)
}

func prepareTurnMemoryLaneLinesPrepared(selection prepareTurnMemoryLaneSelection, languageContext map[string]any, canonicalMemories []store.Memory, prepared *prepareTurnRequestPreparation, perspectiveContextArg ...map[string]any) ([]string, map[string]any) {
	lines := []string{}
	trace := newPrepareTurnMemoryLanguageTrace(languageContext)
	finalRenderDuplicates := 0
	lineageItems := []map[string]any{}
	actualLines := []string{}
	protectedLines := []string{}
	authorityScores := map[string]float64{}
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	protectedGroups, protectedGroupMembers := buildPrepareTurnProtectedDeliveryGroupsPrepared(selection, prepared, canonicalMemories...)
	emittedMemories := map[string]bool{}
	appendLane := func(label string, items []store.Memory) bool {
		appendedAny := false
		for laneRank, item := range items {
			memoryKey := prepareTurnMemoryLaneKey(item)
			renderMemoryKey := memoryKey
			if label == "protected" {
				renderMemoryKey = "protected:" + memoryKey
			}
			if emittedMemories[renderMemoryKey] {
				finalRenderDuplicates++
				continue
			}
			summary := prepared.summary(item)
			if summary == "" {
				continue
			}
			emittedMemories[renderMemoryKey] = true
			groups := protectedGroups[memoryKey]
			protectedGroupMember := protectedGroupMembers[memoryKey]
			if label != "protected" && !prepared.guard(item).Active {
				groups = nil
				protectedGroupMember = false
			}
			if len(groups) == 0 && protectedGroupMember {
				finalRenderDuplicates++
				continue
			}
			if len(groups) == 0 {
				groups = []prepareTurnProtectedDeliveryGroup{{Memory: item}}
			}
			for _, group := range groups {
				if group.Disclosure != nil {
					lineage := prepareTurnMemoryDeliveryLineageItem(item, label, laneRank, selection, "", group.CoverageKey, false, "released_by_later_disclosure", nil, perspectiveContext)
					lineage["disclosure_source_row_id"] = prepareTurnMemorySourceRowID(*group.Disclosure)
					lineage["disclosure_source_turn"] = group.Disclosure.TurnIndex
					lineageItems = append(lineageItems, lineage)
					continue
				}
				renderItem := group.Memory
				lineText, lineTrace := prepareTurnMemoryInjectionLineText(renderItem, summary, languageContext, perspectiveContext)
				updatePrepareTurnMemoryLanguageTrace(trace, lineTrace)
				finalKey := memoryKey
				if group.CoverageKey != "" {
					finalKey = "protected:" + group.CoverageKey
				}
				lineage := prepareTurnMemoryDeliveryLineageItem(
					item, label, laneRank, selection, lineText, finalKey, true,
					"delivered", nil, perspectiveContext,
				)
				if group.CoverageKey != "" {
					lineage["protected_coverage_key"] = group.CoverageKey
					lineage["merged_source_row_ids"] = group.SourceRowIDs
					lineage["merged_source_count"] = len(group.SourceRowIDs)
				}
				meta := prepareTurnMemoryLineMeta(item, label, selection)
				renderedLine := fmt.Sprintf("- [%s] %s", strings.Join(meta, ", "), lineText)
				lines = append(lines, renderedLine)
				if group.CoverageKey != "" {
					if score, ok := selection.VectorScores[memoryKey]; ok {
						authorityScores[renderedLine] = score
					} else if score, ok := selection.RelevantScores[memoryKey]; ok {
						authorityScores[renderedLine] = score
					}
					protectedLines = append(protectedLines, renderedLine)
				} else {
					actualLines = append(actualLines, renderedLine)
				}
				appendedAny = true
				lineageItems = append(lineageItems, lineage)
			}
		}
		return appendedAny
	}
	lanes := []struct {
		label string
		items []store.Memory
	}{
		{label: "vector_relevant", items: selection.VectorRelevant},
		{label: "relevant", items: selection.Relevant},
		{label: "deep", items: selection.Deep},
		{label: "recent", items: selection.Recent},
	}
	coveredDirectEntities := map[string]bool{}
	for _, entity := range selection.DirectlyReferenced {
		entityKey := normalizePrepareTurnEntityNeedle(entity)
		if entityKey == "" || coveredDirectEntities[entityKey] {
			continue
		}
		selected := false
		for _, lane := range lanes {
			for _, item := range lane.items {
				if prepared.guard(item).Active {
					continue
				}
				matches := prepareTurnMemoryDirectEntityMatches(item, selection.DirectlyReferenced)
				if !prepareTurnRelationshipNameInList(entity, matches) {
					continue
				}
				if !appendLane(lane.label, []store.Memory{item}) {
					continue
				}
				for _, matched := range matches {
					coveredDirectEntities[normalizePrepareTurnEntityNeedle(matched)] = true
				}
				selected = true
				break
			}
			if selected {
				break
			}
		}
	}
	for _, lane := range lanes {
		appendLane(lane.label, lane.items)
	}
	appendLane("protected", selection.ProtectedSelected)
	trace["line_count"] = len(lines)
	trace["direct_entity_render_requested_count"] = len(selection.DirectlyReferenced)
	trace["direct_entity_render_covered_count"] = len(coveredDirectEntities)
	trace["direct_entity_render_gap"] = maxInt(len(selection.DirectlyReferenced)-len(coveredDirectEntities), 0)
	trace["final_render_duplicate_count"] = finalRenderDuplicates
	trace["final_render_dedup_applied"] = finalRenderDuplicates > 0
	trace["final_render_dedup_scope"] = "same_stored_row_or_exact_protected_fact_artifact_and_boundary"
	trace["delivery_lineage_items"] = lineageItems
	trace["actual_lines"] = actualLines
	trace["protected_lines"] = protectedLines
	trace["authority_selection_scores"] = authorityScores
	return lines, trace
}

type prepareTurnProtectedDeliveryGroup struct {
	CoverageKey     string
	Representative  string
	Memory          store.Memory
	SourceRowIDs    []any
	ProtectedItems  []any
	ProtectionField string
	Disclosure      *store.Memory
}

func buildPrepareTurnProtectedDeliveryGroups(selection prepareTurnMemoryLaneSelection, canonicalMemories ...store.Memory) (map[string][]prepareTurnProtectedDeliveryGroup, map[string]bool) {
	return buildPrepareTurnProtectedDeliveryGroupsPrepared(selection, nil, canonicalMemories...)
}

func buildPrepareTurnProtectedDeliveryGroupsPrepared(selection prepareTurnMemoryLaneSelection, prepared *prepareTurnRequestPreparation, canonicalMemories ...store.Memory) (map[string][]prepareTurnProtectedDeliveryGroup, map[string]bool) {
	selected := map[string]bool{}
	selectedItems := []store.Memory{}
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, item := range lane {
			if prepared.guard(item).Active {
				selected[prepareTurnMemoryLaneKey(item)] = true
			}
			selectedItems = append(selectedItems, item)
		}
	}
	for _, item := range selection.ProtectedSelected {
		selected[prepareTurnMemoryLaneKey(item)] = true
		selectedItems = append(selectedItems, item)
	}
	candidates := selection.ProtectedCandidates
	if len(candidates) == 0 {
		candidates = selectedItems
	}
	// Resolve the current disclosure of the same recorded secret before
	// rendering historical guards. Canonical rows themselves remain unchanged.
	type disclosure struct {
		turn    int
		guarded bool
		source  store.Memory
	}
	latestDisclosure := map[string]disclosure{}
	for _, source := range [][]store.Memory{canonicalMemories, candidates, selectedItems} {
		for _, item := range source {
			parsed := prepared.sourceMap(item.SummaryJSON)
			for _, field := range []string{"protected_secrets", "character_identity_accuracy"} {
				policyKey := "disclosure_policy"
				if field == "character_identity_accuracy" {
					policyKey = "reveal_policy"
				}
				for _, raw := range sliceFromAny(parsed[field]) {
					protectedItem := mapFromAny(raw)
					key := prepareTurnProtectedKnowledgeKey(item.ChatSessionID, field, protectedItem)
					if key == "" {
						continue
					}
					next := disclosure{turn: item.TurnIndex, guarded: protectedSecretRequiresGuard(protectedItem, policyKey), source: item}
					previous, found := latestDisclosure[key]
					if !found || next.turn > previous.turn || (next.turn == previous.turn && !next.guarded) {
						latestDisclosure[key] = next
					}
				}
			}
		}
	}
	groups := map[string]*prepareTurnProtectedDeliveryGroup{}
	members := map[string]bool{}
	order := []string{}
	add := func(item store.Memory, field string, ordinal int, protectedItem map[string]any) {
		occurrenceKey := prepareTurnMemorySourceOccurrenceKey(item)
		artifactID := strings.TrimSpace(extractionFirstNonEmpty(
			extractionStringFromAny(protectedItem["artifact_id"]),
			extractionStringFromAny(protectedItem["secret_id"]),
			extractionStringFromAny(protectedItem["identity_id"]),
			extractionStringFromAny(protectedItem["source_occurrence_id"]),
		))
		artifactCoordinate := fmt.Sprintf("ordinal:%d", ordinal)
		if artifactID != "" {
			artifactCoordinate = "artifact:" + artifactID
		}
		groupKey := ""
		if field == "protected_secrets" {
			// A shared owner/category/subject does not identify a secret. Consolidate
			// repeated claims only with their recorded artifact and knowledge boundary.
			claim := extractionFirstNonEmpty(stringFromMap(protectedItem, "summary"), stringFromMap(protectedItem, "secret_summary"), stringFromMap(protectedItem, "text"))
			if claim != "" {
				boundary := preciseMemorySemanticPayload(protectedItem, []string{"artifact_id", "secret_id", "identity_id", "source_occurrence_id", "subject", "owner", "secret_kind", "knowledge_scope", "owner_entity_id", "knower_entity_id", "visibility", "privacy_guard", "reveal_policy", "disclosure_policy"})
				material := strings.Join([]string{item.ChatSessionID, field, strings.Join(strings.Fields(claim), " "), mustCompactJSON(boundary)}, "\x1f")
				groupKey = fmt.Sprintf("protected-secret-fact:%x", sha256.Sum256([]byte(material)))
			}
		}
		if groupKey == "" && occurrenceKey != "" {
			identity := map[string]any{
				"knowledge_scope":   protectedItem["knowledge_scope"],
				"owner_entity_id":   protectedItem["owner_entity_id"],
				"knower_entity_id":  protectedItem["knower_entity_id"],
				"visibility":        protectedItem["visibility"],
				"privacy_guard":     protectedItem["privacy_guard"],
				"reveal_policy":     protectedItem["reveal_policy"],
				"disclosure_policy": protectedItem["disclosure_policy"],
			}
			material := strings.Join([]string{occurrenceKey, field, artifactCoordinate, mustCompactJSON(identity), mustCompactJSON(protectedItem)}, "\x1f")
			groupKey = fmt.Sprintf("protected-source-group:%x", sha256.Sum256([]byte(material)))
		} else if groupKey == "" {
			material := strings.Join([]string{prepareTurnMemoryLaneKey(item), field, artifactCoordinate, mustCompactJSON(protectedItem)}, "\x1f")
			groupKey = fmt.Sprintf("protected-distinct-row:%x", sha256.Sum256([]byte(material)))
		}
		group := groups[groupKey]
		if group == nil {
			group = &prepareTurnProtectedDeliveryGroup{CoverageKey: groupKey, ProtectionField: field}
			groups[groupKey] = group
			order = append(order, groupKey)
		}
		if len(group.ProtectedItems) == 0 {
			group.ProtectedItems = append(group.ProtectedItems, protectedItem)
		}
		group.SourceRowIDs = appendUniquePrepareTurnSourceRowID(group.SourceRowIDs, prepareTurnMemorySourceRowID(item))
		itemKey := prepareTurnMemoryLaneKey(item)
		if selected[itemKey] {
			members[itemKey] = true
		}
		// Latest observation first; within that turn retain the most complete
		// recorded named boundary, then the latest row. Never union conflicting
		// historical scopes or infer a missing knower.
		newerSecret := false
		if strings.HasPrefix(groupKey, "protected-secret-fact:") {
			previous := mapFromAny(group.ProtectedItems[0])
			completeness := prepareTurnProtectedBoundaryCompleteness(protectedItem)
			previousCompleteness := prepareTurnProtectedBoundaryCompleteness(previous)
			newerSecret = item.TurnIndex > group.Memory.TurnIndex || (item.TurnIndex == group.Memory.TurnIndex && (completeness > previousCompleteness || (completeness == previousCompleteness && item.ID > group.Memory.ID)))
		}
		if selected[itemKey] && (group.Representative == "" || newerSecret) {
			group.Representative = itemKey
			group.Memory = item
			group.ProtectedItems = []any{protectedItem}
			group.Disclosure = nil
			if latest, found := latestDisclosure[prepareTurnProtectedKnowledgeKey(item.ChatSessionID, field, protectedItem)]; found && !latest.guarded {
				source := latest.source
				group.Disclosure = &source
			}
		}
	}
	for _, item := range candidates {
		parsed := prepared.sourceMap(item.SummaryJSON)
		for ordinal, raw := range sliceFromAny(parsed["protected_secrets"]) {
			secret := mapFromAny(raw)
			if protectedSecretRequiresGuard(secret, "disclosure_policy") {
				add(item, "protected_secrets", ordinal, secret)
			}
		}
		for ordinal, raw := range sliceFromAny(parsed["character_identity_accuracy"]) {
			identity := mapFromAny(raw)
			if protectedSecretRequiresGuard(identity, "reveal_policy") {
				add(item, "character_identity_accuracy", ordinal, identity)
			}
		}
	}
	out := map[string][]prepareTurnProtectedDeliveryGroup{}
	for _, key := range order {
		group := groups[key]
		if group == nil || group.Representative == "" || len(group.ProtectedItems) == 0 {
			continue
		}
		parsed := maps.Clone(prepared.sourceMap(group.Memory.SummaryJSON))
		delete(parsed, "protected_secrets")
		delete(parsed, "character_identity_accuracy")
		parsed[group.ProtectionField] = group.ProtectedItems
		encoded, err := json.Marshal(parsed)
		if err != nil {
			continue
		}
		group.Memory.SummaryJSON = string(encoded)
		out[group.Representative] = append(out[group.Representative], *group)
	}
	return out, members
}

func prepareTurnProtectedBoundaryCompleteness(item map[string]any) int {
	scope := mapFromAny(item["knowledge_scope"])
	count := 0
	for _, field := range []string{"known_by", "suspected_by", "unknown_to", "misinformed_by", "revealed_to"} {
		names := map[string]bool{}
		for _, name := range stringsFromAny(scope[field]) {
			if key := normalizeCharacterKey(name); key != "" {
				names[key] = true
			}
		}
		count += len(names)
	}
	return count
}

func prepareTurnProtectedKnowledgeKey(sessionID, field string, item map[string]any) string {
	for _, name := range []string{"secret_id", "identity_id", "artifact_id"} {
		if id := strings.TrimSpace(stringFromMap(item, name)); id != "" {
			return strings.Join([]string{sessionID, field, name, id}, "\x1f")
		}
	}
	// Legacy rows have no persistent secret ID. An identical recorded claim
	// identifies a repeated secret; a shared category alone does not.
	if field == "protected_secrets" {
		claim := normalizeArtifactDedupeText(extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "secret_summary"), stringFromMap(item, "text")))
		if claim == "" {
			return ""
		}
		return strings.Join([]string{sessionID, field, comparableEntityKey(stringFromMap(item, "owner")), normalizeProtectedSecretToken(stringFromMap(item, "secret_kind")), mustCompactJSON(stringsFromAny(item["subject"])), claim}, "\x1f")
	}
	surface := extractionFirstNonEmpty(stringFromMap(item, "surface_identity_name"), stringFromMap(item, "public_identity_name"), stringFromMap(item, "alias_name"))
	owner := extractionFirstNonEmpty(stringFromMap(item, "true_identity_name"), stringFromMap(item, "canonical_entity_name"), stringFromMap(item, "real_identity_name"))
	if surface == "" || owner == "" {
		return ""
	}
	facts := preciseMemorySemanticPayload(item, []string{"identity_kind", "same_entity", "public_role", "true_role", "public_allegiance", "true_allegiance"})
	return strings.Join([]string{sessionID, field, comparableEntityKey(owner), comparableEntityKey(surface), mustCompactJSON(facts)}, "\x1f")
}

func appendUniquePrepareTurnSourceRowID(items []any, value any) []any {
	needle := fmt.Sprint(value)
	for _, item := range items {
		if fmt.Sprint(item) == needle {
			return items
		}
	}
	return append(items, value)
}

func prepareTurnMemoryLineMeta(item store.Memory, label string, selection prepareTurnMemoryLaneSelection) []string {
	meta := []string{label}
	if item.TurnIndex > 0 {
		meta = append(meta, fmt.Sprintf("turn %d", item.TurnIndex))
	}
	if label == "vector_relevant" {
		if score := selection.VectorScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
			meta = append(meta, fmt.Sprintf("vector %.2f", score))
		}
	}
	if label == "relevant" {
		if score := selection.RelevantScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
			meta = append(meta, fmt.Sprintf("score %.2f", score))
		}
	}
	if label == "deep" && item.Importance > 0 {
		meta = append(meta, fmt.Sprintf("imp %.2f", item.Importance))
	}
	return meta
}

func prepareTurnMemorySourceRowID(item store.Memory) any {
	if item.ID > 0 {
		return item.ID
	}
	return prepareTurnMemoryLaneKey(item)
}

func prepareTurnMemoryDeliveryLineageItem(item store.Memory, lane string, laneRank int, selection prepareTurnMemoryLaneSelection, finalText, finalKey string, delivered bool, status string, duplicateOf any, perspectiveContext map[string]any) map[string]any {
	guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext)
	score := 0.0
	if lane == "vector_relevant" {
		score = selection.VectorScores[prepareTurnMemoryLaneKey(item)]
	} else if lane == "relevant" {
		score = selection.RelevantScores[prepareTurnMemoryLaneKey(item)]
	}
	itemTrace := map[string]any{
		"source_table":                 "memories",
		"source_row_id":                prepareTurnMemorySourceRowID(item),
		"turn_index":                   item.TurnIndex,
		"selection_lane":               lane,
		"lane_rank":                    laneRank + 1,
		"selection_score":              score,
		"importance_score":             item.Importance,
		"emotional_boost":              item.EmotionalBoost,
		"emotional_intensity":          item.EmotionalIntensity,
		"narrative_significance":       item.NarrativeSignificance,
		"vector_hit":                   lane == "vector_relevant",
		"protected_guard":              guard.Active,
		"protected_identity_pov_scope": guard.POVScoped,
		"top_k_consumption":            "not_applicable_vector_candidate_limit",
		"core_objective_k_consumption": "objective_event_candidate",
		"delivered":                    delivered,
		"delivery_status":              status,
		"final_text":                   finalText,
		"final_text_chars":             len([]rune(finalText)),
		"final_render_key":             finalKey,
		"source_occurrence_key":        nilIfEmpty(prepareTurnMemorySourceOccurrenceKey(item)),
	}
	if lane == "protected" {
		if value, ok := selection.VectorScores[prepareTurnMemoryLaneKey(item)]; ok {
			itemTrace["selection_score"] = value
			itemTrace["selection_score_source"] = "same_row_vector_relevant"
		} else if value, ok := selection.RelevantScores[prepareTurnMemoryLaneKey(item)]; ok {
			itemTrace["selection_score"] = value
			itemTrace["selection_score_source"] = "same_row_relevant"
		} else {
			itemTrace["selection_score_source"] = "unobserved_original_order"
		}
	}
	if guard.Active {
		itemTrace["core_objective_k_consumption"] = "item_count_exempt_protected_guard"
	}
	if duplicateOf != nil {
		itemTrace["duplicate_of_source_row_id"] = duplicateOf
	}
	return itemTrace
}

func buildPrepareTurnMemoryDeliveryLineage(selection prepareTurnMemoryLaneSelection, renderTrace map[string]any) map[string]any {
	items := prepareTurnMemoryLineageSlice(renderTrace["delivery_lineage_items"])
	deliveredActual := 0
	deliveredProtected := 0
	deliveredTotal := 0
	for _, raw := range items {
		item := mapFromAny(raw)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		deliveredTotal++
		if boolFromAny(item["protected_guard"]) {
			deliveredProtected++
		} else {
			deliveredActual++
		}
	}
	coverageStatus := "empty"
	issueCodes := []string{}
	if deliveredTotal > 0 {
		coverageStatus = "mixed"
	}
	if deliveredProtected == 0 && deliveredActual > 0 {
		coverageStatus = "actual_memory_only"
	} else if deliveredProtected > 0 && deliveredActual == 0 {
		coverageStatus = "protected_guard_only"
		issueCodes = append(issueCodes, "actual_memory_absent")
	} else if deliveredProtected > deliveredActual {
		coverageStatus = "protected_guard_dominant"
		issueCodes = append(issueCodes, "protected_guard_dominates_final_memory_lines")
	}
	vectorTrace := mapFromAny(selection.Trace["vector_recall"])
	return map[string]any{
		"contract_version":                     "memory_delivery_lineage.v1",
		"status":                               coverageStatus,
		"source_chain":                         []string{"store.memories", "vector_or_lexical_selection", "protected_guard_render", "final_memory_text"},
		"top_k_memory_target":                  intFromAny(selection.Trace["top_k_memory_target"], 0),
		"input_memory_count":                   intFromAny(selection.Trace["input_memory_count"], 0),
		"eligible_memory_count":                intFromAny(selection.Trace["eligible_memory_count"], 0),
		"vector_memory_hit_count":              intFromAny(vectorTrace["memory_hit_count"], 0),
		"vector_memory_hydrated_count":         intFromAny(vectorTrace["hydrated_count"], 0),
		"final_delivered_count":                deliveredTotal,
		"final_actual_memory_count":            deliveredActual,
		"final_protected_guard_count":          deliveredProtected,
		"pre_render_protected_duplicate_count": intFromAny(selection.Trace["protected_duplicate_candidate_count"], 0),
		"pre_render_protected_duplicates":      prepareTurnMemoryLineageSlice(selection.Trace["protected_duplicate_candidates"]),
		"protected_relevance_dropped":          prepareTurnMemoryLineageSlice(selection.Trace["protected_memory_dropped"]),
		"final_render_duplicate_count":         intFromAny(renderTrace["final_render_duplicate_count"], 0),
		"known_issue_codes":                    issueCodes,
		"items":                                items,
	}
}

func prepareTurnMemoryLineageSlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	if items, ok := value.([]map[string]any); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

func newPrepareTurnMemoryLanguageTrace(languageContext map[string]any) map[string]any {
	return map[string]any{
		"contract_version":                 languageMemoryContractVersion,
		"session_output_language":          nilIfEmpty(prepareTurnSessionOutputLanguage(languageContext)),
		"summary_language_target":          nilIfEmpty(prepareTurnSummaryLanguageTarget(languageContext)),
		"memory_summary_language_match":    0,
		"memory_summary_language_mismatch": 0,
		"memory_language_unknown":          0,
		"raw_evidence_attached_count":      0,
		"raw_evidence_preserved":           true,
		"raw_user_input_rewritten":         false,
	}
}

func updatePrepareTurnMemoryLanguageTrace(trace map[string]any, lineTrace map[string]any) {
	if trace == nil || lineTrace == nil {
		return
	}
	if boolFromAny(lineTrace["summary_language_matches_target"]) {
		trace["memory_summary_language_match"] = intFromAny(trace["memory_summary_language_match"], 0) + 1
	} else if strings.TrimSpace(extractionStringFromAny(lineTrace["summary_language"])) != "" &&
		strings.TrimSpace(extractionStringFromAny(lineTrace["summary_language_target"])) != "" {
		trace["memory_summary_language_mismatch"] = intFromAny(trace["memory_summary_language_mismatch"], 0) + 1
	} else {
		trace["memory_language_unknown"] = intFromAny(trace["memory_language_unknown"], 0) + 1
	}
	if boolFromAny(lineTrace["raw_evidence_attached"]) {
		trace["raw_evidence_attached_count"] = intFromAny(trace["raw_evidence_attached_count"], 0) + 1
	}
}

func buildPrepareTurnLanguageInjectionTrace(languageContext map[string]any, memoryTrace map[string]any) map[string]any {
	return map[string]any{
		"contract_version":            languageMemoryContractVersion,
		"status":                      prepareTurnLanguageInjectionStatus(languageContext),
		"session_output_language":     nilIfEmpty(prepareTurnSessionOutputLanguage(languageContext)),
		"summary_language_target":     nilIfEmpty(prepareTurnSummaryLanguageTarget(languageContext)),
		"output_language_source":      nilIfEmpty(extractionStringFromAny(languageContext["output_language_source"])),
		"current_user_input_priority": "highest",
		"raw_user_input_rewritten":    false,
		"raw_evidence_rewritten":      false,
		"related_memory_policy":       "prefer_stored_output_language_summary_preserve_raw_evidence_when_available",
		"translation_call_attempted":  false,
		"memory_language_trace":       nilIfEmptyMap(memoryTrace),
	}
}

func prepareTurnLanguageInjectionStatus(languageContext map[string]any) string {
	target := prepareTurnSessionOutputLanguage(languageContext)
	if target == "" || target == "unknown" || target == "auto" {
		return "trace_only_unknown_language"
	}
	return "ready"
}

func prepareTurnSessionOutputLanguage(languageContext map[string]any) string {
	return normalizePrepareTurnLanguageCode(extractionFirstNonEmpty(
		extractionStringFromAny(languageContext["session_output_language"]),
		extractionStringFromAny(languageContext["summary_language"]),
	))
}

func prepareTurnSummaryLanguageTarget(languageContext map[string]any) string {
	return normalizePrepareTurnLanguageCode(extractionFirstNonEmpty(
		extractionStringFromAny(languageContext["summary_language"]),
		extractionStringFromAny(languageContext["session_output_language"]),
	))
}

func normalizePrepareTurnLanguageCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "ko", "kr", "kor", "korean":
		return "ko"
	case "en", "eng", "english":
		return "en"
	case "ja", "jp", "jpn", "japanese":
		return "ja"
	case "auto":
		return "auto"
	case "unknown":
		return "unknown"
	default:
		return value
	}
}

func prepareTurnMemoryInjectionLineText(item store.Memory, summary string, languageContext map[string]any, perspectiveContextArg ...map[string]any) (string, map[string]any) {
	meta := memoryVectorLanguageMetadata(item)
	summaryLanguage := normalizePrepareTurnLanguageCode(meta["summary_language"])
	targetLanguage := prepareTurnSummaryLanguageTarget(languageContext)
	rawLanguage := normalizePrepareTurnLanguageCode(meta["raw_language"])
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active {
		lineTrace := map[string]any{
			"summary_language":                nilIfEmpty(summaryLanguage),
			"summary_language_target":         nilIfEmpty(targetLanguage),
			"raw_language":                    nilIfEmpty(rawLanguage),
			"summary_language_matches_target": summaryLanguage != "" && targetLanguage != "" && summaryLanguage == targetLanguage,
			"raw_evidence_attached":           false,
			"raw_evidence_preserved":          true,
			"protected_secret_guarded":        true,
			"protected_identity_pov_scoped":   guard.POVScoped,
		}
		return prepareTurnProtectedCardFromParsed(parseJSONMap(item.SummaryJSON), guard.LineText, perspectiveContext), lineTrace
	}
	parts := []string{summary}
	rawEvidence := prepareTurnMemoryRawEvidenceLines(item)
	if len(rawEvidence) > 0 && rawLanguage != "" && summaryLanguage != "" && rawLanguage != summaryLanguage {
		parts = append(parts, "raw_evidence: "+strings.Join(rawEvidence, " | "))
	}
	// Language is diagnostic metadata, not another version of this source.
	// Appending it here made the rendered summary miss its typed seed identity.
	lineTrace := map[string]any{
		"summary_language":                nilIfEmpty(summaryLanguage),
		"summary_language_target":         nilIfEmpty(targetLanguage),
		"raw_language":                    nilIfEmpty(rawLanguage),
		"summary_language_matches_target": summaryLanguage != "" && targetLanguage != "" && summaryLanguage == targetLanguage,
		"raw_evidence_attached":           len(rawEvidence) > 0 && rawLanguage != "" && summaryLanguage != "" && rawLanguage != summaryLanguage,
		"raw_evidence_preserved":          true,
	}
	return strings.Join(parts, " | "), lineTrace
}

type prepareTurnProtectedMemoryGuardResult struct {
	Active    bool
	LineText  string
	POVScoped bool
}

// Delivery is author-facing. Retain the established guard/POV decisions for
// recall and private routing, but do not replace a secret's meaning with counts.
// Missing knowledge metadata is not evidence that anyone knows (or does not).
const prepareTurnProtectedGuardPreamble = "Protected continuity guard: protected private knowledge exists. | Do not reveal, confess, or let unrelated characters discover it without current-scene evidence."

// The guard travels through general-memory paths; the protected section shares
// it once and charges that shared text exactly.
const prepareTurnProtectedCardGuard = "Protected continuity guard; Author-only (not public character knowledge; reveal only with current-scene evidence): "

func prepareTurnProtectedCardFromParsed(parsed map[string]any, guardText string, perspectiveContext ...map[string]any) string {
	cards := []string{}
	for _, field := range []string{"protected_secrets", "character_identity_accuracy"} {
		policyField := "disclosure_policy"
		if field == "character_identity_accuracy" {
			policyField = "reveal_policy"
		}
		for _, raw := range sliceFromAny(parsed[field]) {
			item := mapFromAny(raw)
			if !protectedSecretRequiresGuard(item, policyField) {
				continue
			}
			parts := []string{}
			claim := extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "secret_summary"), stringFromMap(item, "text"))
			if claim != "" {
				parts = append(parts, claim)
			}
			if subjects := stringsFromAny(item["subject"]); len(subjects) > 0 {
				parts = append(parts, "subject="+strings.Join(subjects, ", "))
			}
			if field == "character_identity_accuracy" {
				// Preserve the existing self/cover-role reading, including an
				// explicitly recorded same-person relation; add named boundaries.
				identityGuard := prepareTurnProtectedMemoryGuardFromParsed(map[string]any{field: []any{item}}, perspectiveContext...)
				// The card prefix already carries the generic guard; keep only relations.
				relation := strings.TrimPrefix(identityGuard.LineText, prepareTurnProtectedGuardPreamble)
				relation = strings.TrimPrefix(relation, " | ")
				if relation != "" {
					parts = append([]string{relation}, parts...)
				}
			} else if kind := stringFromMap(item, "secret_kind"); kind != "" {
				parts = append(parts, "kind="+kind)
			}
			if owner := stringFromMap(item, "owner"); owner != "" {
				parts = append(parts, "owner="+owner)
			}
			scope := mapFromAny(item["knowledge_scope"])
			for _, key := range []string{"known_by", "suspected_by", "unknown_to", "misinformed_by", "revealed_to"} {
				if names := stringsFromAny(scope[key]); len(names) > 0 {
					parts = append(parts, key+"="+strings.Join(names, ", "))
				}
			}
			if policy := stringFromMap(item, policyField); policy != "" {
				parts = append(parts, "policy="+policy)
			}
			if len(parts) > 0 {
				cards = append(cards, prepareTurnProtectedCardGuard+strings.Join(parts, "; "))
			}
		}
	}
	if len(cards) == 0 {
		return guardText
	}
	return strings.Join(cards, " | ")
}

func prepareTurnProtectedMemoryGuard(item store.Memory, perspectiveContextArg ...map[string]any) prepareTurnProtectedMemoryGuardResult {
	return prepareTurnProtectedMemoryGuardFromParsed(parseJSONMap(item.SummaryJSON), perspectiveContextArg...)
}

func prepareTurnProtectedMemoryGuardFromParsed(parsed map[string]any, perspectiveContextArg ...map[string]any) prepareTurnProtectedMemoryGuardResult {
	protectedSecrets := sliceFromAny(parsed["protected_secrets"])
	identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
	if len(protectedSecrets) == 0 && len(identityAccuracy) == 0 {
		return prepareTurnProtectedMemoryGuardResult{}
	}
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	if line := prepareTurnPOVScopedIdentityGuardLine(identityAccuracy, perspectiveContext); line != "" {
		return prepareTurnProtectedMemoryGuardResult{
			Active:    true,
			LineText:  line,
			POVScoped: true,
		}
	}
	if line := prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy); line != "" {
		return prepareTurnProtectedMemoryGuardResult{
			Active:   true,
			LineText: line,
		}
	}
	kinds := []string{}
	policies := []string{}
	knownBy := map[string]bool{}
	suspectedBy := map[string]bool{}
	boundaries := []string{}
	addScope := func(scope map[string]any) {
		for _, value := range stringsFromAny(scope["known_by"]) {
			knownBy[normalizeCharacterKey(value)] = true
		}
		for _, value := range stringsFromAny(scope["suspected_by"]) {
			suspectedBy[normalizeCharacterKey(value)] = true
		}
	}
	for _, raw := range protectedSecrets {
		secret := mapFromAny(raw)
		if !protectedSecretRequiresGuard(secret, "disclosure_policy") {
			continue
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(secret, "secret_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		addScope(mapFromAny(secret["knowledge_scope"]))
		owner := stringFromMap(secret, "owner")
		scope := mapFromAny(secret["knowledge_scope"])
		boundary := []string{}
		if owner != "" {
			boundary = append(boundary, "owner="+owner)
		}
		for _, field := range []string{"known_by", "suspected_by", "unknown_to", "misinformed_by", "revealed_to"} {
			if names := stringsFromAny(scope[field]); len(names) > 0 {
				boundary = append(boundary, field+"="+strings.Join(names, ", "))
			}
		}
		// Preserve the existing POV boundary. A suspicion is not knowledge.
		pov, povKey := stringFromMap(perspectiveContext, "current_pov"), stringFromMap(perspectiveContext, "current_pov_key")
		povAliases := stringsFromAny(perspectiveContext["current_pov_aliases"])
		knows := prepareTurnPerspectiveNameMatches(pov, povKey, owner, povAliases...)
		for _, name := range append(stringsFromAny(scope["known_by"]), stringsFromAny(scope["revealed_to"])...) {
			knows = knows || prepareTurnPerspectiveNameMatches(pov, povKey, name, povAliases...)
		}
		if knows {
			if subject := stringsFromAny(secret["subject"]); len(subject) > 0 {
				boundary = append(boundary, "subject="+strings.Join(subject, ", "))
			}
			if claim := extractionFirstNonEmpty(stringFromMap(secret, "summary"), stringFromMap(secret, "secret_summary"), stringFromMap(secret, "text")); claim != "" {
				boundary = append(boundary, "POV-private knowledge="+claim)
			}
		}
		if len(boundary) > 0 {
			boundaries = append(boundaries, strings.Join(boundary, "; "))
		}
	}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		addScope(mapFromAny(identity["knowledge_scope"]))
	}
	if len(kinds) == 0 && len(policies) == 0 {
		return prepareTurnProtectedMemoryGuardResult{}
	}
	parts := []string{prepareTurnProtectedGuardPreamble}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	if len(knownBy) > 0 || len(suspectedBy) > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", len(knownBy), len(suspectedBy)))
	}
	parts = append(parts, boundaries...)
	return prepareTurnProtectedMemoryGuardResult{
		Active:   true,
		LineText: strings.Join(parts, " | "),
	}
}

func prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy []any) string {
	relations := []string{}
	kinds := []string{}
	policies := []string{}
	knownBy := map[string]bool{}
	suspectedBy := map[string]bool{}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		surface := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		))
		trueName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "real_identity_name"),
		))
		if surface == "" || trueName == "" || normalizeCharacterKey(surface) == normalizeCharacterKey(trueName) {
			continue
		}
		if boolFromAny(identity["same_entity"]) {
			relations = appendUniqueMemorySearchText(relations, fmt.Sprintf("%s and %s refer to the same internal person", surface, trueName))
		} else {
			relations = appendUniqueMemorySearchText(relations, fmt.Sprintf("%s is protected identity context for %s", surface, trueName))
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		scope := mapFromAny(identity["knowledge_scope"])
		for _, value := range stringsFromAny(scope["known_by"]) {
			knownBy[normalizeCharacterKey(value)] = true
		}
		for _, value := range stringsFromAny(scope["suspected_by"]) {
			suspectedBy[normalizeCharacterKey(value)] = true
		}
	}
	if len(relations) == 0 {
		return ""
	}
	parts := []string{
		"Protected identity continuity: " + strings.Join(relations, "; ") + ".",
		"Maintain same-entity continuity internally; do not portray the surface identity and true identity as separate people.",
		"When same_entity is confirmed, keep aliases merged in entity resolution even when public roles or cover roles differ.",
		"This is author-side/private support, not public character knowledge; do not reveal, confess, or let unrelated characters discover it without current-scene evidence.",
	}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	if len(knownBy) > 0 || len(suspectedBy) > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", len(knownBy), len(suspectedBy)))
	}
	return strings.Join(parts, " | ")
}

func prepareTurnPOVScopedIdentityGuardLine(identityAccuracy []any, perspectiveContext map[string]any) string {
	povName := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"]))
	povKey := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_key"]))
	povAliases := stringsFromAny(perspectiveContext["current_pov_aliases"])
	if povName == "" && povKey == "" {
		return ""
	}
	relations := []string{}
	samePersonRules := []string{}
	kinds := []string{}
	policies := []string{}
	knownBy := map[string]bool{}
	suspectedBy := map[string]bool{}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		if !prepareTurnPerspectiveKnowsIdentity(identity, povName, povKey, povAliases...) {
			continue
		}
		surface := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		))
		trueName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "real_identity_name"),
		))
		if surface == "" || trueName == "" || normalizeCharacterKey(surface) == normalizeCharacterKey(trueName) {
			continue
		}
		relations = appendUniqueMemorySearchText(relations, fmt.Sprintf("%s is %s's own protected surface identity/persona", surface, trueName))
		samePersonRules = appendUniqueMemorySearchText(samePersonRules, fmt.Sprintf("%s and %s refer to the same recorded person", surface, trueName))
		if prepareTurnPerspectiveOwnsIdentity(identity, povName, povKey, povAliases...) {
			samePersonRules = appendUniqueMemorySearchText(samePersonRules, "the recorded surface name is this POV's self/cover-role continuity")
		} else {
			samePersonRules = appendUniqueMemorySearchText(samePersonRules, fmt.Sprintf("%s knows this identity relationship about %s", povName, trueName))
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		scope := mapFromAny(identity["knowledge_scope"])
		for _, value := range stringsFromAny(scope["known_by"]) {
			knownBy[normalizeCharacterKey(value)] = true
		}
		for _, value := range stringsFromAny(scope["suspected_by"]) {
			suspectedBy[normalizeCharacterKey(value)] = true
		}
	}
	if len(relations) == 0 {
		return ""
	}
	parts := []string{
		"POV-scoped identity continuity: " + strings.Join(relations, "; ") + ".",
		fmt.Sprintf("For current_pov=%s, %s.", povName, strings.Join(samePersonRules, "; ")),
		"Keep this as POV/private knowledge; do not reveal it to characters outside knowledge_scope without current reveal evidence.",
	}
	if len(kinds) == 0 {
		kinds = append(kinds, "identity")
	}
	parts = append(parts, "kind="+strings.Join(kinds, ","))
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	if len(knownBy) > 0 || len(suspectedBy) > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", len(knownBy), len(suspectedBy)))
	}
	return strings.Join(parts, " | ")
}

func prepareTurnPerspectiveKnowsIdentity(identity map[string]any, povName, povKey string, povAliases ...string) bool {
	if prepareTurnPerspectiveOwnsIdentity(identity, povName, povKey, povAliases...) {
		return true
	}
	scope := mapFromAny(identity["knowledge_scope"])
	for _, field := range []string{"known_by", "revealed_to"} {
		for _, candidate := range stringsFromAny(scope[field]) {
			if prepareTurnPerspectiveNameMatches(povName, povKey, candidate, povAliases...) {
				return true
			}
		}
	}
	return false
}

func prepareTurnPerspectiveOwnsIdentity(identity map[string]any, povName, povKey string, povAliases ...string) bool {
	candidates := []string{
		stringFromMap(identity, "canonical_entity_name"),
		stringFromMap(identity, "true_identity_name"),
		stringFromMap(identity, "surface_identity_name"),
		stringFromMap(identity, "public_identity_name"),
		stringFromMap(identity, "alias_name"),
	}
	candidates = append(candidates, stringsFromAny(identity["aliases"])...)
	for _, candidate := range candidates {
		if prepareTurnPerspectiveNameMatches(povName, povKey, candidate, povAliases...) {
			return true
		}
	}
	return false
}

// A localized display can repeat the same name inside parentheses. Collapse
// only equivalent normalized components; different names need recorded aliases.
// Keep this separate from canonical storage and global character normalization.
func prepareTurnPerspectiveNameKey(name string) string {
	key := normalizeCharacterKey(name)
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '(' || r == ')' || r == '（' || r == '）' })
	if len(parts) < 2 {
		return key
	}
	partKey := normalizeCharacterKey(parts[0])
	if partKey == "" {
		return key
	}
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) != "" && normalizeCharacterKey(part) != partKey {
			return key
		}
	}
	return partKey
}

func prepareTurnPerspectiveNameMatches(povName, povKey, candidate string, povAliases ...string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	candidateKey := normalizeCharacterKey(candidate)
	if povKey != "" && candidateKey != "" && povKey == candidateKey {
		return true
	}
	for _, name := range append([]string{povName}, povAliases...) {
		if key := prepareTurnPerspectiveNameKey(name); key != "" && key == prepareTurnPerspectiveNameKey(candidate) {
			return true
		}
	}
	return strings.TrimSpace(povName) != "" && strings.EqualFold(strings.TrimSpace(povName), candidate)
}

func protectedSecretRequiresGuard(item map[string]any, policyKey string) bool {
	if boolFromAny(item["public_narration_allowed"]) {
		return false
	}
	scope := mapFromAny(item["knowledge_scope"])
	if boolFromAny(scope["publicly_revealed"]) || boolFromAny(scope["reader_visible"]) || boolFromAny(scope["protagonist_visible"]) {
		return false
	}
	policy := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, policyKey), stringFromMap(item, "target_reveal_policy")))
	if policy == "" {
		return true
	}
	switch normalizeTargetRevealPolicy(policy) {
	case "owner_private_until_revealed", "explicit_user_reveal_required", "current_session_confirmation_required", "explicit_reveal_event_required", "user_directed_reveal_only", "requires_explicit_attachment":
		return true
	default:
		return false
	}
}

func prepareTurnMemoryRawEvidenceLines(item store.Memory) []string {
	out := []string{}
	evidence := parseJSONMap(item.Evidence)
	for _, value := range memorySearchStringValues(evidence["evidence_excerpts"]) {
		value = strings.TrimSpace(value)
		if value != "" {
			out = appendMemorySearchAlias(out, value)
		}
	}
	return out
}

func prepareTurnMemoryLaneKey(item store.Memory) string {
	if item.ID > 0 {
		return fmt.Sprintf("memory:%d", item.ID)
	}
	return fmt.Sprintf("turn:%d:%s", item.TurnIndex, stableKey("memory", prepareTurnMemorySummary(item)))
}

type prepareTurnHierarchyEscalation struct {
	EpisodeText string
	ChapterText string
	ArcText     string
	SagaText    string
	Trace       map[string]any
}

func buildPrepareTurnHierarchyEscalation(resumePack *store.ResumePack, chatLogs []store.ChatLog, memorySelection prepareTurnMemoryLaneSelection, rawUserInput, profile string) prepareTurnHierarchyEscalation {
	trace := map[string]any{
		"version":                     "hierarchy_request_zoom.v1",
		"status":                      "off",
		"chapter_selected":            false,
		"arc_selected":                false,
		"saga_selected":               false,
		"chapter_reason":              "no_chapter",
		"arc_reason":                  "no_arc",
		"saga_reason":                 "no_saga",
		"chapter_mode":                "omitted",
		"arc_mode":                    "omitted",
		"saga_mode":                   "omitted",
		"priority":                    "current_user_input_and_direct_evidence_remain_higher_priority",
		"truth_boundary":              "hierarchy_summaries_are_support_only",
		"single_resolution_zoom":      true,
		"garbage_fill":                false,
		"recent_memory_bound":         len(memorySelection.Recent),
		"selected_memory_bound":       prepareTurnSelectedMemoryCount(memorySelection),
		"selection_reason_visibility": true,
	}
	out := prepareTurnHierarchyEscalation{Trace: trace}
	if resumePack == nil {
		trace["reason"] = "no_resume_pack"
		return out
	}
	maxTurn := prepareTurnMaxObservedTurn(chatLogs, resumePack)
	trace["status"] = "ready"
	trace["max_observed_turn"] = maxTurn
	trace["profile"] = profile

	queryParts := []string{strings.TrimSpace(rawUserInput)}
	for _, lane := range [][]store.Memory{memorySelection.VectorRelevant, memorySelection.Relevant, memorySelection.Deep, memorySelection.Recent} {
		for _, item := range lane {
			if prepareTurnProtectedMemoryGuard(item).Active {
				continue
			}
			queryParts = append(queryParts, prepareTurnMemorySummary(item))
		}
	}
	query := strings.TrimSpace(strings.Join(nonEmptyStrings(queryParts), "\n"))
	type hierarchyCandidate struct {
		kind     string
		text     string
		fromTurn int
		toTurn   int
		score    int
	}
	candidates := []hierarchyCandidate{}
	if resumePack.Chapter != nil {
		text := prepareTurnChapterRecallText(*resumePack.Chapter)
		score := 0
		if prepareTurnSupportRecallEligible(query, text) {
			score = prepareTurnRecallOverlapCount(query, text)
			if score == 0 {
				score = 1
			}
		}
		candidates = append(candidates, hierarchyCandidate{
			kind: "chapter", text: text, fromTurn: resumePack.Chapter.FromTurn, toTurn: resumePack.Chapter.ToTurn,
			score: score,
		})
	}
	if resumePack.Arc != nil {
		text := prepareTurnArcRecallText(*resumePack.Arc)
		score := 0
		if prepareTurnSupportRecallEligible(query, text) {
			score = prepareTurnRecallOverlapCount(query, text)
			if score == 0 {
				score = 1
			}
		}
		candidates = append(candidates, hierarchyCandidate{
			kind: "arc", text: text, fromTurn: resumePack.Arc.FromTurn, toTurn: resumePack.Arc.ToTurn,
			score: score,
		})
	}
	if resumePack.Saga != nil {
		text := prepareTurnSagaRecallText(*resumePack.Saga)
		score := 0
		if prepareTurnSupportRecallEligible(query, text) {
			score = prepareTurnRecallOverlapCount(query, text)
			if score == 0 {
				score = 1
			}
		}
		candidates = append(candidates, hierarchyCandidate{
			kind: "saga", text: text, fromTurn: resumePack.Saga.FromTurn, toTurn: resumePack.Saga.ToTurn,
			score: score,
		})
	}
	selectedKind := ""
	for _, candidate := range candidates {
		trace[candidate.kind+"_relevance_score"] = candidate.score
		trace[candidate.kind+"_range"] = map[string]int{"from_turn": candidate.fromTurn, "to_turn": candidate.toTurn}
		if selectedKind != "" {
			trace[candidate.kind+"_reason"] = "suppressed_by_narrower_relevant_resolution"
			continue
		}
		if candidate.score <= 0 || strings.TrimSpace(candidate.text) == "" {
			trace[candidate.kind+"_reason"] = "irrelevant_to_current_request"
			continue
		}
		selectedKind = candidate.kind
		trace[candidate.kind+"_selected"] = true
		trace[candidate.kind+"_reason"] = "current_request_relevance"
		trace[candidate.kind+"_mode"] = prepareTurnHierarchyMode(candidate.text)
		trace[candidate.kind+"_chars"] = len([]rune(strings.TrimSpace(candidate.text)))
		switch candidate.kind {
		case "chapter":
			out.ChapterText = candidate.text
		case "arc":
			out.ArcText = candidate.text
		case "saga":
			out.SagaText = candidate.text
		}
	}
	if selectedKind == "" {
		trace["status"] = "no_support"
		trace["reason"] = "no_hierarchy_level_relevant_to_current_request"
	} else {
		trace["selected_resolution"] = selectedKind
	}
	trace["selected_count"] = boolToInt(strings.TrimSpace(out.ChapterText) != "") + boolToInt(strings.TrimSpace(out.ArcText) != "") + boolToInt(strings.TrimSpace(out.SagaText) != "")
	return out
}

func prepareTurnMaxObservedTurn(chatLogs []store.ChatLog, resumePack *store.ResumePack) int {
	maxTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > maxTurn {
			maxTurn = cl.TurnIndex
		}
	}
	if resumePack != nil {
		if resumePack.Chapter != nil && resumePack.Chapter.ToTurn > maxTurn {
			maxTurn = resumePack.Chapter.ToTurn
		}
		if resumePack.Arc != nil && resumePack.Arc.ToTurn > maxTurn {
			maxTurn = resumePack.Arc.ToTurn
		}
		if resumePack.Saga != nil && resumePack.Saga.ToTurn > maxTurn {
			maxTurn = resumePack.Saga.ToTurn
		}
	}
	return maxTurn
}

func prepareTurnHierarchyMode(text string) string {
	chars := len([]rune(strings.TrimSpace(text)))
	switch {
	case chars == 0:
		return "omitted"
	case chars <= 220:
		return "tiny"
	case chars <= 520:
		return "compact"
	default:
		return "full"
	}
}

func prepareTurnChapterRecallText(ch store.ChapterSummary) string {
	lines := []string{}
	title := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ChapterTitle, fmt.Sprintf("Chapter %d", ch.ChapterIndex)), 0)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText), 0)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", ch.FromTurn, ch.ToTurn, title, summary))
	}
	if loops := compactEpisodeJSONPreview(ch.OpenLoopsJSON, 0); loops != "" {
		lines = append(lines, "- open_loop: "+loops)
	}
	if rel := compactEpisodeJSONPreview(ch.RelationshipChangesJSON, 0); rel != "" {
		lines = append(lines, "- relationship_shift: "+rel)
	}
	if world := compactEpisodeJSONPreview(ch.WorldChangesJSON, 0); world != "" {
		lines = append(lines, "- world_change: "+world)
	}
	if callbacks := compactEpisodeJSONPreview(ch.CallbackCandidatesJSON, 0); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Chapter Recall]", lines)
}

func prepareTurnArcRecallText(arc store.ArcSummary) string {
	lines := []string{}
	name := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcName, fmt.Sprintf("Arc %d", arc.ArcIndex)), 0)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName), 0)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", arc.FromTurn, arc.ToTurn, name, summary))
	}
	if status := strings.TrimSpace(arc.ArcStatus); status != "" {
		lines = append(lines, "- status: "+compactPrepareTurnLine(status, 0))
	}
	if turns := compactEpisodeJSONPreview(arc.KeyTurningPointsJSON, 0); turns != "" {
		lines = append(lines, "- turning_point: "+turns)
	}
	if debts := compactEpisodeJSONPreview(arc.UnresolvedDebtsJSON, 0); debts != "" {
		lines = append(lines, "- unresolved: "+debts)
	}
	if callbacks := compactEpisodeJSONPreview(arc.CallbackCandidatesJSON, 0); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Arc Recall]", lines)
}

func prepareTurnSagaRecallText(saga store.SagaDigest) string {
	lines := []string{}
	label := compactPrepareTurnLine(q1FirstNonEmptyString(saga.EraLabel, "Saga"), 0)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel), 0)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", saga.FromTurn, saga.ToTurn, label, summary))
	}
	if facts := compactEpisodeJSONPreview(saga.PersistentFactsJSON, 0); facts != "" {
		lines = append(lines, "- persistent_fact: "+facts)
	}
	if neverDrop := compactEpisodeJSONPreview(saga.NeverDropCandidatesJSON, 0); neverDrop != "" {
		lines = append(lines, "- never_drop: "+neverDrop)
	}
	return makePrepareTurnSection("[Saga Recall]", lines)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

const prepareTurnKnowledgeBoundaryReading = "Preserve each linked fact's recorded knowledge scope; delivery does not establish additional character knowledge."

// Links are read only from named metadata fields. A turn, owner or similar
// sentence is provenance, not a fact-level disclosure relationship.
func prepareTurnKnowledgeRefs(item map[string]any) []string {
	refs := []string{}
	for _, key := range []string{"source_ref", "source_refs", "fact_ref", "fact_refs", "fact_id", "canonical_fact_id", "summary_ref", "summary_refs", "lineage_refs", "memory_ref", "memory_refs", "evidence_refs"} {
		for _, ref := range prepareTurnKnowledgeRefValues(item[key]) {
			if ref = strings.TrimSpace(ref); ref != "" {
				refs = appendUniqueStringValues(refs, ref)
			}
		}
	}
	for _, key := range []string{"lifecycle_key", "pending_thread_key", "thread_key"} {
		if ref := strings.TrimSpace(stringFromMap(item, key)); ref != "" {
			if key == "thread_key" {
				key = "pending_thread_key"
			}
			refs = appendUniqueStringValues(refs, key+":"+ref)
		}
	}
	// An explicit memory record reference is distinct from a source turn.
	if id := intFromAny(item["source_memory_id"], 0); id > 0 {
		refs = appendUniqueStringValues(refs, fmt.Sprintf("memories:%d", id))
	}
	return refs
}

func prepareTurnOwnKnowledgeBoundary(item map[string]any, classified ...bool) []map[string]any {
	boundaries := []map[string]any{}
	// Stored quotation boundaries already identify their protected source fact.
	// Ingest metadata only, without turning that source scope into this claim's.
	for _, raw := range sliceFromAny(item["knowledge_boundaries"]) {
		stored := mapFromAny(raw)
		boundary := map[string]any{}
		for _, key := range []string{"knowledge_scope", "knowledge_source", "protected_fact_ref", "fact_type", "owner", "source_refs", "fact_ref", "fact_id", "canonical_fact_id", "secret_id", "artifact_id"} {
			if value, exists := stored[key]; exists {
				boundary[key] = value
			}
		}
		boundaries = append(boundaries, boundary)
	}
	scope := mapFromAny(item["knowledge_scope"])
	if len(scope) == 0 {
		return boundaries
	}
	// An explicit public annotation already has its existing reading.
	// Carry does not add a second guard to that unlinked public fact.
	guarded := len(classified) > 0 && classified[0] || publicMemoryProjectionHasScopedMaterial(item)
	if !strings.EqualFold(strings.TrimSpace(stringFromMap(item, "visibility")), "public") {
		for _, key := range []string{"unknown_to", "suspected_by", "misinformed_by"} {
			guarded = guarded || len(stringsFromAny(scope[key])) > 0
		}
	}
	if !guarded && len(boundaries) == 0 {
		return boundaries
	}
	// Beside historical quotations, retain the supplied current claim scope too.
	// This is metadata on the existing reading, not private reclassification.
	// Attribution uses explicit identifiers, never the protected body. That body
	// is delivered only by its existing independently budgeted owner.
	boundary := map[string]any{"knowledge_scope": scope, "owner": extractionFirstNonEmpty(stringFromMap(item, "owner"), stringFromMap(item, "owner_entity_name"), stringFromMap(item, "perspective_owner"), stringFromMap(item, "knowledge_holder")), "source_refs": prepareTurnKnowledgeRefs(item)}
	for _, key := range []string{"fact_ref", "fact_id", "canonical_fact_id", "secret_id", "artifact_id"} {
		if value, ok := item[key]; ok {
			boundary[key] = value
		}
	}
	return append(boundaries, boundary)
}

// One linked scope record. Its encoding is fixed once indexed, so request-wide
// reuse never re-marshals the same boundary for each alias, pass or seed.
type prepareTurnKnowledgeBoundaryRecord struct {
	value   map[string]any
	encoded string
	partKey string
}

func newPrepareTurnKnowledgeBoundaryRecord(boundary map[string]any) *prepareTurnKnowledgeBoundaryRecord {
	encoded := mustCompactJSON(boundary)
	sum := sha256.Sum256([]byte(encoded))
	return &prepareTurnKnowledgeBoundaryRecord{value: boundary, encoded: encoded, partKey: fmt.Sprintf("@knowledge/%x", sum[:8])}
}

// The linked-scope graph depends only on immutable request sources. It is read
// by every carry in one request and never by another request.
type prepareTurnKnowledgeIndex struct {
	byRef    map[string][]*prepareTurnKnowledgeBoundaryRecord
	sessions map[string]string
	payloads map[string]map[string]any
}

type prepareTurnKnowledgeIndexCache struct {
	once  sync.Once
	index *prepareTurnKnowledgeIndex
}

func prepareTurnKnowledgeIndexFor(input prepareTurnAssemblyInput) *prepareTurnKnowledgeIndex {
	if input.knowledgeIndex == nil {
		return buildPrepareTurnKnowledgeIndex(input)
	}
	input.knowledgeIndex.once.Do(func() { input.knowledgeIndex.index = buildPrepareTurnKnowledgeIndex(input) })
	return input.knowledgeIndex.index
}

func buildPrepareTurnKnowledgeIndex(input prepareTurnAssemblyInput) *prepareTurnKnowledgeIndex {
	byRef := map[string][]*prepareTurnKnowledgeBoundaryRecord{}
	sessions := map[string]string{}
	payloads := map[string]map[string]any{}
	aliases := map[string][]string{}
	declaredFacts := map[string]bool{}
	originalBindings := []struct {
		origin, session string
		refs            []string
		boundaries      []*prepareTurnKnowledgeBoundaryRecord
	}{}
	indexed := map[string]map[string]bool{}
	index := func(session string, refs []string, boundaries []*prepareTurnKnowledgeBoundaryRecord) bool {
		changed := false
		for _, ref := range refs {
			key := session + "\x1f" + ref
			if indexed[key] == nil {
				indexed[key] = map[string]bool{}
			}
			for _, boundary := range boundaries {
				if !indexed[key][boundary.encoded] {
					indexed[key][boundary.encoded] = true
					byRef[key] = append(byRef[key], boundary)
					changed = true
				}
			}
		}
		return changed
	}
	addNode := func(key, session string, payload map[string]any, boundaries []map[string]any) []*prepareTurnKnowledgeBoundaryRecord {
		sessions[key] = session
		payloads[key] = payload
		// Only this record's identifiers export acquired scope. Source references
		// are incoming edges, not aliases for unrelated facts in their parent.
		ownRefs := map[string]any{}
		for _, name := range []string{"fact_ref", "fact_id", "canonical_fact_id", "lifecycle_key", "pending_thread_key", "thread_key"} {
			ownRefs[name] = payload[name]
		}
		aliases[key] = appendUniqueStringValues([]string{key}, prepareTurnKnowledgeRefs(ownRefs)...)
		aliases[key] = appendUniqueStringValues(aliases[key], prepareTurnKnowledgeRefValues(payload["secret_id"])...)
		for _, name := range []string{"fact_id", "canonical_fact_id", "fact_ref", "secret_id"} {
			for _, ref := range prepareTurnKnowledgeRefValues(payload[name]) {
				declaredFacts[session+"\x1f"+ref] = true
			}
		}
		records := make([]*prepareTurnKnowledgeBoundaryRecord, 0, len(boundaries))
		for _, boundary := range boundaries {
			if stringFromMap(boundary, "protected_fact_ref") == "" {
				boundary["protected_fact_ref"] = key
			}
			records = append(records, newPrepareTurnKnowledgeBoundaryRecord(boundary))
		}
		index(session, aliases[key], records)
		return records
	}
	addSource := func(table string, id int64, session string, payload map[string]any) {
		key := prepareTurnPriorityStoredOccurrence(table, id, "")
		if table == "memories" || table == "character_states" {
			// These aggregate rows provide provenance, not sibling aliases.
			sessions[key], payloads[key] = session, payload
			return
		}
		addNode(key, session, payload, prepareTurnOwnKnowledgeBoundary(payload))
	}
	for _, m := range input.Memories {
		parsed := parseJSONMap(m.SummaryJSON)
		addSource("memories", m.ID, m.ChatSessionID, parsed)
		fields := make([]string, 0, len(parsed))
		for field := range parsed {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			for ordinal, raw := range sliceFromAny(parsed[field]) {
				item := mapFromAny(raw)
				// Reuse the existing scope owner for supplied unknown/suspected/
				// misinformed metadata, even without a separate visibility flag.
				guarded := publicMemoryProjectionHasScopedMaterial(item) || len(prepareTurnOwnKnowledgeBoundary(item)) > 0
				switch field {
				case "protected_secrets":
					guarded = protectedSecretRequiresGuard(item, "disclosure_policy")
				case "character_identity_accuracy":
					guarded = protectedSecretRequiresGuard(item, "reveal_policy")
				case "belief_updates", "subjective_entity_memories", "user_interaction_profile", "body_events":
					guarded = true // Existing scoped extraction buckets.
				}
				var boundaries []map[string]any
				if guarded {
					boundaries = prepareTurnOwnKnowledgeBoundary(item, true)
				} else {
					boundaries = prepareTurnOwnKnowledgeBoundary(map[string]any{"knowledge_boundaries": item["knowledge_boundaries"]})
				}
				ref := fmt.Sprintf("memories:%d/%s/%d", m.ID, field, ordinal)
				for _, boundary := range boundaries {
					if stringFromMap(boundary, "fact_type") == "" {
						boundary["fact_type"] = field
					}
				}
				records := addNode(ref, m.ChatSessionID, item, boundaries)
				originalBindings = append(originalBindings, struct {
					origin, session string
					refs            []string
					boundaries      []*prepareTurnKnowledgeBoundaryRecord
				}{ref, m.ChatSessionID, prepareTurnKnowledgeRefs(item), records})
				// An explicit source_memory_id can read original scoped facts. Never
				// export a child's acquired scope through the aggregate memory row.
				index(m.ChatSessionID, []string{prepareTurnPriorityStoredOccurrence("memories", m.ID, "")}, records)
			}
		}
	}
	for _, pt := range input.PendingThreads {
		p := parseJSONMap(pt.HookMetadataJSON)
		for k, v := range parseJSONMap(pt.DetailsJSON) {
			if _, ok := p[k]; !ok {
				p[k] = v
			}
		}
		p["thread_key"] = pt.ThreadKey
		p["description"] = pt.Description
		addSource("pending_threads", pt.ID, pt.ChatSessionID, p)
	}
	for _, sl := range input.Storylines {
		p := parseJSONMap(sl.KeyPointsJSON)
		for key, value := range parseJSONMap(sl.OngoingTensionsJSON) {
			if _, exists := p[key]; !exists {
				p[key] = value
			}
		}
		p["source_refs"] = appendUniqueStringValues(prepareTurnKnowledgeJSONRefs(sl.KeyPointsJSON), prepareTurnKnowledgeJSONRefs(sl.OngoingTensionsJSON)...)
		p["description"] = sl.CurrentContext
		addSource("storylines", sl.ID, sl.ChatSessionID, p)
	}
	for _, es := range input.EpisodeSummaries {
		p := parseJSONMap(es.OpenLoopsJSON)
		p["source_refs"] = appendUniqueStringValues(prepareTurnKnowledgeJSONRefs(es.OpenLoopsJSON), append(prepareTurnKnowledgeJSONRefs(es.KeyEvents), prepareTurnKnowledgeJSONRefs(es.RelationshipChangesJSON)...)...)
		p["summary"] = es.SummaryText
		addSource("episode_summaries", es.ID, es.ChatSessionID, p)
	}
	for _, ev := range input.Evidence {
		p := parseJSONMap(ev.LineageJSON)
		p["source_refs"] = prepareTurnKnowledgeJSONRefs(ev.LineageJSON)
		p["text"] = ev.EvidenceText
		addSource("direct_evidence_records", ev.ID, ev.ChatSessionID, p)
	}
	for _, cs := range input.CharacterStates {
		addSource("character_states", cs.ID, cs.ChatSessionID, map[string]any{"status": parseJSONMap(cs.StatusJSON), "personality": parseJSONMap(cs.PersonalityJSON), "appearance": parseJSONMap(cs.AppearanceJSON), "relationships": parseJSONMap(cs.RelationshipsJSON)})
	}
	for _, rule := range input.WorldRules {
		addSource("world_rules", rule.ID, rule.ChatSessionID, parseJSONMap(rule.ValueJSON))
	}
	for _, layer := range input.CanonicalLayers {
		addSource("canonical_state_layers", layer.ID, layer.ChatSessionID, parseJSONMap(layer.Content))
	}
	if pack := input.ResumePack; pack != nil {
		if chapter := pack.Chapter; chapter != nil {
			refs := []string{}
			for _, raw := range []string{chapter.OpenLoopsJSON, chapter.RelationshipChangesJSON, chapter.WorldChangesJSON, chapter.CallbackCandidatesJSON} {
				refs = appendUniqueStringValues(refs, prepareTurnKnowledgeJSONRefs(raw)...)
			}
			addSource("chapter_summaries", chapter.ID, chapter.ChatSessionID, map[string]any{"source_refs": refs})
		}
		if arc := pack.Arc; arc != nil {
			refs := []string{}
			for _, raw := range []string{arc.KeyTurningPointsJSON, arc.ActivePromisesJSON, arc.UnresolvedDebtsJSON, arc.ResolvedPayoffsJSON, arc.CallbackCandidatesJSON, arc.FuturePayoffCandidatesJSON, arc.IrreversibleTurnsJSON, arc.CallbackDebtsJSON, arc.RelationshipPivotsJSON} {
				refs = appendUniqueStringValues(refs, prepareTurnKnowledgeJSONRefs(raw)...)
			}
			addSource("arc_summaries", arc.ID, arc.ChatSessionID, map[string]any{"source_refs": refs})
		}
		if saga := pack.Saga; saga != nil {
			addSource("saga_digests", saga.ID, saga.ChatSessionID, map[string]any{"source_refs": appendUniqueStringValues(prepareTurnKnowledgeJSONRefs(saga.PersistentFactsJSON), prepareTurnKnowledgeJSONRefs(saga.NeverDropCandidatesJSON)...)})
		}
	}
	for _, entry := range input.CharacterPrivateMemories {
		addSource("protagonist_entity_memories", entry.ID, entry.SourceChatSessionID, map[string]any{"source_refs": prepareTurnKnowledgeJSONRefs(entry.TagsJSON)})
	}
	// Keep original same-fact and record-target bindings. A reference to a
	// separately declared fact is incoming provenance, not an alias exporting
	// this fact's own scope to that fact and its other descendants.
	for _, binding := range originalBindings {
		item := payloads[binding.origin]
		if extractionFirstNonEmpty(stringFromMap(item, "fact_id"), stringFromMap(item, "canonical_fact_id"), stringFromMap(item, "fact_ref"), stringFromMap(item, "secret_id")) == "" {
			index(binding.session, binding.refs, binding.boundaries)
			continue
		}
		for _, ref := range binding.refs {
			if declaredFacts[binding.session+"\x1f"+ref] && !slices.Contains(aliases[binding.origin], ref) {
				continue
			}
			index(binding.session, []string{ref}, binding.boundaries)
		}
	}

	// Follow explicit references through logical downstream records. Aggregate
	// memory/character rows never become aliases for every co-turn fact.
	recordKeys := make([]string, 0, len(payloads))
	for key := range payloads {
		recordKeys = append(recordKeys, key)
	}
	sort.Strings(recordKeys)
	// Payloads are not changed by indexing; read each record's links once.
	recordRefs := make(map[string][]string, len(recordKeys))
	for _, occurrence := range recordKeys {
		recordRefs[occurrence] = prepareTurnKnowledgeRefs(payloads[occurrence])
	}
	for pass := 0; pass < len(payloads); pass++ {
		changed := false
		for _, occurrence := range recordKeys {
			if len(aliases[occurrence]) == 0 {
				continue
			}
			for _, ref := range recordRefs[occurrence] {
				changed = index(sessions[occurrence], aliases[occurrence], byRef[sessions[occurrence]+"\x1f"+ref]) || changed
			}
		}
		if !changed {
			break
		}
	}
	return &prepareTurnKnowledgeIndex{byRef: byRef, sessions: sessions, payloads: payloads}
}

func prepareTurnCarryKnowledgeBoundaries(out *prepareTurnInjectionAssembly, input prepareTurnAssemblyInput) {
	if out == nil {
		return
	}
	if len(out.PriorityFactSeeds) == 0 {
		return
	}
	graph := prepareTurnKnowledgeIndexFor(input)
	byRef, sessions, payloads := graph.byRef, graph.sessions, graph.payloads
	// The shared graph is request-wide. Each carry receives its own copies, so a
	// later reader of one result never changes another carry's scope records.
	copies := map[*prepareTurnKnowledgeBoundaryRecord]map[string]any{}
	copyOf := func(record *prepareTurnKnowledgeBoundaryRecord) map[string]any {
		if value, ok := copies[record]; ok {
			return value
		}
		value, _ := prepareTurnCloneKnowledgeValue(record.value).(map[string]any)
		copies[record] = value
		return value
	}
	for i := range out.PriorityFactSeeds {
		seed := &out.PriorityFactSeeds[i]
		fact := &seed.Fact
		occurrence := prepareTurnPriorityStoredOccurrence(seed.SourceTable, int64(intFromAny(seed.SourceRowID, 0)), "")
		session := sessions[occurrence]
		if fact.SourceSession != "" {
			session = fact.SourceSession
		}
		refs := append([]string{}, fact.ExplicitRefs...)
		if seed.SourceTable != "memories" && seed.SourceTable != "character_states" {
			refs = appendUniqueStringValues(refs, seed.SourceRef, seed.SourceOccurrence, occurrence)
		} else {
			if seed.SourceRef != "" && seed.SourceRef != occurrence {
				refs = appendUniqueStringValues(refs, seed.SourceRef)
			}
		}
		payload := payloads[occurrence]
		if seed.SourceTable == "character_states" || (seed.SourceTable == "memories" && (fact.Structured || fact.SourcePath != "")) {
			// Public arrays may have removed private siblings. Their projected ordinal
			// is not a canonical fact reference. Typed facts carry their admitted refs
			// and own scopes; never recover these by indexing the original turn.
			payload = nil
		}

		refs = appendUniqueStringValues(refs, prepareTurnKnowledgeRefs(payload)...)
		boundaries := make([]*prepareTurnKnowledgeBoundaryRecord, 0, len(fact.KnowledgeBoundaries))
		for _, b := range fact.KnowledgeBoundaries {
			boundaries = append(boundaries, newPrepareTurnKnowledgeBoundaryRecord(b))
		}
		linked := 0
		for _, ref := range refs {
			linked += len(byRef[session+"\x1f"+ref])
		}
		if len(boundaries) == 0 && linked == 0 {
			continue
		}
		// Multiple linked facts retain separately attributed scope records. They do
		// not change this representation's owner or union its allowed_viewers.
		reading := prepareTurnMemoryContext{Path: fact.SourcePath, Parts: []prepareTurnMemoryPart{{Key: fact.SourcePath, Value: fact.Text, FactTexts: []string{fact.Text}}}}
		if fact.Reading != nil {
			reading = *fact.Reading
			reading.Parts = append([]prepareTurnMemoryPart{}, fact.Reading.Parts...)
			reading.fingerprint = [32]byte{}
		}
		seen := map[string]bool{}
		for _, part := range reading.Parts {
			seen[part.Key] = true
		}
		unique := []map[string]any{}
		boundarySeen := map[string]bool{}
		accept := func(record *prepareTurnKnowledgeBoundaryRecord, value func() map[string]any) {
			if boundarySeen[record.encoded] {
				return
			}
			boundarySeen[record.encoded] = true
			unique = append(unique, value())
			if !seen[record.partKey] {
				reading.Parts = append(reading.Parts, prepareTurnMemoryPart{Key: record.partKey, Label: "linked fact knowledge boundary", Value: record.encoded + "; " + prepareTurnKnowledgeBoundaryReading})
				seen[record.partKey] = true
			}
		}
		for _, record := range boundaries {
			accept(record, func() map[string]any { return record.value })
		}
		for _, ref := range refs {
			for _, record := range byRef[session+"\x1f"+ref] {
				accept(record, func() map[string]any { return copyOf(record) })
			}
		}
		fact.KnowledgeBoundaries = unique
		fact.Reading = &reading
	}
}

func prepareTurnCloneKnowledgeValue(value any) any {
	// nil and empty values encode differently and stay distinct.
	switch v := value.(type) {
	case map[string]any:
		if v == nil {
			return v
		}
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = prepareTurnCloneKnowledgeValue(item)
		}
		return out
	case []any:
		if v == nil {
			return v
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = prepareTurnCloneKnowledgeValue(item)
		}
		return out
	case []string:
		if v == nil {
			return v
		}
		return append(make([]string, 0, len(v)), v...)
	case []map[string]any:
		if v == nil {
			return v
		}
		out := make([]map[string]any, len(v))
		for i, item := range v {
			out[i], _ = prepareTurnCloneKnowledgeValue(item).(map[string]any)
		}
		return out
	}
	return value
}

func prepareTurnCarryDirectKnowledgeLine(line string, ev store.DirectEvidence, input prepareTurnAssemblyInput) string {
	projected := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{SourceTable: "direct_evidence_records", SourceRowID: ev.ID, Fact: prepareTurnPriorityMemoryFact{Text: line}}}}
	prepareTurnCarryKnowledgeBoundaries(&projected, input)
	boundaries := projected.PriorityFactSeeds[0].Fact.KnowledgeBoundaries
	if len(boundaries) == 0 {
		return line
	}
	return line + " | linked fact knowledge boundaries=" + mustCompactJSON(boundaries) + "; " + prepareTurnKnowledgeBoundaryReading
}

// Legacy sections do not consume the typed reading forms. Append only linked,
// individually attributed scope records to their existing complete source row.
func prepareTurnCarryLegacyKnowledgeSections(out *prepareTurnInjectionAssembly) {
	for _, section := range []struct {
		table string
		text  *string
	}{{"pending_threads", &out.PendingThreadText}, {"storylines", &out.StorylineText}, {"episode_summaries", &out.EpisodeText}, {"memories", &out.MemoryText}, {"character_states", &out.CharacterText}, {"chapter_summaries", &out.ChapterText}, {"arc_summaries", &out.ArcText}, {"saga_digests", &out.SagaText}, {"protagonist_entity_memories", &out.CharacterPrivateText}} {
		lines := strings.Split(*section.text, "\n")
		for i, line := range lines {
			key := prepareTurnPriorityCleanLine(line)
			bounds := []map[string]any{}
			seen := map[string]bool{}
			for _, seed := range out.PriorityFactSeeds {
				if seed.SourceTable != section.table || seed.ParentLineKey != key {
					continue
				}
				for _, b := range seed.Fact.KnowledgeBoundaries {
					encoded := mustCompactJSON(b)
					if !seen[encoded] {
						bounds = append(bounds, b)
						seen[encoded] = true
					}
				}
			}
			if len(bounds) > 0 {
				lines[i] = line + " | linked fact knowledge boundaries=" + mustCompactJSON(bounds) + "; " + prepareTurnKnowledgeBoundaryReading
				newKey := prepareTurnPriorityCleanLine(lines[i])
				for j := range out.PriorityFactSeeds {
					seed := &out.PriorityFactSeeds[j]
					if seed.SourceTable == section.table && seed.ParentLineKey == key {
						seed.ParentLineKey = newKey
					}
				}
				for j := range out.PrioritySourceMetadata {
					meta := &out.PrioritySourceMetadata[j]
					if meta.SourceTable == section.table && meta.LineKey == key {
						meta.LineKey = newKey
					}
				}
			}
		}
		*section.text = strings.Join(lines, "\n")
	}
}

func prepareTurnKnowledgeRefValues(value any) []string {
	switch v := value.(type) {
	case string:
		if ref := strings.TrimSpace(v); ref != "" {
			return []string{ref}
		}
	case []string:
		return append([]string(nil), v...)
	case []any:
		refs := []string{}
		for _, item := range v {
			refs = appendUniqueStringValues(refs, prepareTurnKnowledgeRefValues(item)...)
		}
		return refs
	case map[string]any:
		refs := []string{}
		for _, key := range []string{"source_ref", "fact_ref", "summary_ref", "ref"} {
			refs = appendUniqueStringValues(refs, prepareTurnKnowledgeRefValues(v[key])...)
		}
		table := stringFromMap(v, "source_table")
		id := intFromAny(v["source_row_id"], 0)
		if table != "" && id > 0 {
			refs = appendUniqueStringValues(refs, prepareTurnPriorityStoredOccurrence(table, int64(id), ""))
		}
		return refs
	}
	return nil
}

// JSON record fields can be object or array shaped. Only named reference
// metadata establishes links; plain string entries remain ordinary content.
func prepareTurnKnowledgeJSONRefs(raw string) []string {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return nil
	}
	refs := []string{}
	var visit func(any)
	visit = func(value any) {
		switch item := value.(type) {
		case map[string]any:
			refs = appendUniqueStringValues(refs, prepareTurnKnowledgeRefs(item)...)
			keys := make([]string, 0, len(item))
			for key := range item {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				visit(item[key])
			}
		case []any:
			for _, nested := range item {
				visit(nested)
			}
		}
	}
	visit(value)
	return refs
}
