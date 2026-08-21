package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnMemoryLaneLines(selection prepareTurnMemoryLaneSelection, languageContext map[string]any, perspectiveContextArg ...map[string]any) ([]string, map[string]any) {
	lines := []string{}
	trace := newPrepareTurnMemoryLanguageTrace(languageContext)
	finalRenderDuplicates := 0
	lineageItems := []map[string]any{}
	actualLines := []string{}
	protectedLines := []string{}
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	protectedGroups, protectedGroupMembers := buildPrepareTurnProtectedDeliveryGroups(selection)
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
			summary := prepareTurnMemorySummary(item)
			if summary == "" {
				continue
			}
			emittedMemories[renderMemoryKey] = true
			groups := protectedGroups[memoryKey]
			protectedGroupMember := protectedGroupMembers[memoryKey]
			if label != "protected" && !prepareTurnProtectedMemoryGuard(item).Active {
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
				if prepareTurnProtectedMemoryGuard(item).Active {
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
	trace["final_render_dedup_scope"] = "same_stored_memory_row_repeated_across_recall_lanes"
	trace["delivery_lineage_items"] = lineageItems
	trace["actual_lines"] = actualLines
	trace["protected_lines"] = protectedLines
	return lines, trace
}

type prepareTurnProtectedDeliveryGroup struct {
	CoverageKey     string
	Representative  string
	Memory          store.Memory
	SourceRowIDs    []any
	ProtectedItems  []any
	ProtectionField string
}

func buildPrepareTurnProtectedDeliveryGroups(selection prepareTurnMemoryLaneSelection) (map[string][]prepareTurnProtectedDeliveryGroup, map[string]bool) {
	selected := map[string]bool{}
	selectedItems := []store.Memory{}
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, item := range lane {
			if prepareTurnProtectedMemoryGuard(item).Active {
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
		if occurrenceKey != "" {
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
		} else {
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
		if group.Representative == "" && selected[itemKey] {
			group.Representative = itemKey
			group.Memory = item
		}
	}
	for _, item := range candidates {
		parsed := parseJSONMap(item.SummaryJSON)
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
		parsed := parseJSONMap(group.Memory.SummaryJSON)
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
		return guard.LineText, lineTrace
	}
	parts := []string{summary}
	rawEvidence := prepareTurnMemoryRawEvidenceLines(item)
	if len(rawEvidence) > 0 && rawLanguage != "" && summaryLanguage != "" && rawLanguage != summaryLanguage {
		parts = append(parts, "raw_evidence: "+strings.Join(rawEvidence, " | "))
	}
	if summaryLanguage != "" {
		parts = append(parts, "summary_language="+summaryLanguage)
	}
	if rawLanguage != "" {
		parts = append(parts, "raw_language="+rawLanguage)
	}
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

func prepareTurnProtectedMemoryGuard(item store.Memory, perspectiveContextArg ...map[string]any) prepareTurnProtectedMemoryGuardResult {
	parsed := parseJSONMap(item.SummaryJSON)
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
	parts := []string{
		"Protected continuity guard: protected private knowledge exists.",
		"Do not reveal, confess, or let unrelated characters discover it without current-scene evidence.",
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
		if !prepareTurnPerspectiveKnowsIdentity(identity, povName, povKey) {
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
		samePersonRules = appendUniqueMemorySearchText(samePersonRules, fmt.Sprintf("treat %s and %s as the same internal person, not two separate characters", surface, trueName))
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
		"If this POV references the surface identity, read it as self/cover-role continuity rather than a separate external character.",
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

func prepareTurnPerspectiveKnowsIdentity(identity map[string]any, povName, povKey string) bool {
	candidates := []string{
		povName,
		povKey,
		stringFromMap(identity, "canonical_entity_name"),
		stringFromMap(identity, "true_identity_name"),
		stringFromMap(identity, "surface_identity_name"),
		stringFromMap(identity, "public_identity_name"),
		stringFromMap(identity, "alias_name"),
	}
	candidates = append(candidates, stringsFromAny(identity["aliases"])...)
	scope := mapFromAny(identity["knowledge_scope"])
	candidates = append(candidates, stringsFromAny(scope["known_by"])...)
	for _, candidate := range candidates {
		if prepareTurnPerspectiveNameMatches(povName, povKey, candidate) {
			return true
		}
	}
	return false
}

func prepareTurnPerspectiveNameMatches(povName, povKey, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	candidateKey := normalizeCharacterKey(candidate)
	if povKey != "" && candidateKey != "" && povKey == candidateKey {
		return true
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
	title := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ChapterTitle, fmt.Sprintf("Chapter %d", ch.ChapterIndex)), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText), 360)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", ch.FromTurn, ch.ToTurn, title, summary))
	}
	if loops := compactEpisodeJSONPreview(ch.OpenLoopsJSON, 160); loops != "" {
		lines = append(lines, "- open_loop: "+loops)
	}
	if rel := compactEpisodeJSONPreview(ch.RelationshipChangesJSON, 160); rel != "" {
		lines = append(lines, "- relationship_shift: "+rel)
	}
	if world := compactEpisodeJSONPreview(ch.WorldChangesJSON, 160); world != "" {
		lines = append(lines, "- world_change: "+world)
	}
	if callbacks := compactEpisodeJSONPreview(ch.CallbackCandidatesJSON, 140); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Chapter Recall]", lines)
}

func prepareTurnArcRecallText(arc store.ArcSummary) string {
	lines := []string{}
	name := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcName, fmt.Sprintf("Arc %d", arc.ArcIndex)), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName), 360)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", arc.FromTurn, arc.ToTurn, name, summary))
	}
	if status := strings.TrimSpace(arc.ArcStatus); status != "" {
		lines = append(lines, "- status: "+compactPrepareTurnLine(status, 80))
	}
	if turns := compactEpisodeJSONPreview(arc.KeyTurningPointsJSON, 180); turns != "" {
		lines = append(lines, "- turning_point: "+turns)
	}
	if debts := compactEpisodeJSONPreview(arc.UnresolvedDebtsJSON, 160); debts != "" {
		lines = append(lines, "- unresolved: "+debts)
	}
	if callbacks := compactEpisodeJSONPreview(arc.CallbackCandidatesJSON, 140); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Arc Recall]", lines)
}

func prepareTurnSagaRecallText(saga store.SagaDigest) string {
	lines := []string{}
	label := compactPrepareTurnLine(q1FirstNonEmptyString(saga.EraLabel, "Saga"), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel), 420)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", saga.FromTurn, saga.ToTurn, label, summary))
	}
	if facts := compactEpisodeJSONPreview(saga.PersistentFactsJSON, 180); facts != "" {
		lines = append(lines, "- persistent_fact: "+facts)
	}
	if neverDrop := compactEpisodeJSONPreview(saga.NeverDropCandidatesJSON, 160); neverDrop != "" {
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
