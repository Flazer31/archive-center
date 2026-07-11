package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnMemoryLaneLines(selection prepareTurnMemoryLaneSelection, languageContext map[string]any, perspectiveContextArg ...map[string]any) ([]string, map[string]any) {
	lines := []string{}
	trace := newPrepareTurnMemoryLanguageTrace(languageContext)
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	appendLane := func(label string, items []store.Memory) {
		for _, item := range items {
			summary := prepareTurnMemorySummary(item)
			if summary == "" {
				continue
			}
			lineText, lineTrace := prepareTurnMemoryInjectionLineText(item, summary, languageContext, perspectiveContext)
			updatePrepareTurnMemoryLanguageTrace(trace, lineTrace)
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
			lines = append(lines, fmt.Sprintf("- [%s] %s", strings.Join(meta, ", "), lineText))
		}
	}
	appendLane("vector_relevant", selection.VectorRelevant)
	appendLane("relevant", selection.Relevant)
	appendLane("deep", selection.Deep)
	appendLane("recent", selection.Recent)
	trace["line_count"] = len(lines)
	return lines, trace
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
	knownByCount := 0
	suspectedByCount := 0
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
		scope := mapFromAny(secret["knowledge_scope"])
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
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
		scope := mapFromAny(identity["knowledge_scope"])
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
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
	if knownByCount > 0 || suspectedByCount > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", knownByCount, suspectedByCount))
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
	knownByCount := 0
	suspectedByCount := 0
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
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
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
	if knownByCount > 0 || suspectedByCount > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", knownByCount, suspectedByCount))
	}
	return strings.Join(parts, " | ")
}

func prepareTurnPOVScopedIdentityGuardLine(identityAccuracy []any, perspectiveContext map[string]any) string {
	povName := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"]))
	povKey := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_key"]))
	if povName == "" && povKey == "" {
		return ""
	}
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
		kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind"))
		if kind == "" {
			kind = "identity"
		}
		policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy"))
		parts := []string{
			fmt.Sprintf("POV-scoped identity continuity: %s is %s's own protected surface identity/persona.", surface, trueName),
			fmt.Sprintf("For current_pov=%s, treat %s and %s as the same internal person, not two separate characters.", povName, surface, trueName),
			"If this POV references the surface identity, read it as self/cover-role continuity rather than a separate external character.",
			"Keep this as POV/private knowledge; do not reveal it to characters outside knowledge_scope without current reveal evidence.",
			"kind=" + kind,
		}
		if policy != "" {
			parts = append(parts, "policy="+policy)
		}
		return strings.Join(parts, " | ")
	}
	return ""
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
	ChapterText string
	ArcText     string
	SagaText    string
	Trace       map[string]any
}

func buildPrepareTurnHierarchyEscalation(resumePack *store.ResumePack, chatLogs []store.ChatLog, memorySelection prepareTurnMemoryLaneSelection, topK int, rawUserInput, profile string) prepareTurnHierarchyEscalation {
	trace := map[string]any{
		"version":                     "r2.hierarchy_escalation.v1",
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
		"top_k_memory_target":         topK,
		"top_k_definition":            "semantic_memory_recall_limit",
		"recent_memory_bound":         len(memorySelection.Recent),
		"selected_memory_bound":       prepareTurnSelectedMemoryCount(memorySelection),
		"selection_reason_visibility": true,
	}
	out := prepareTurnHierarchyEscalation{Trace: trace}
	if resumePack == nil {
		trace["reason"] = "no_resume_pack"
		return out
	}
	topK = prepareTurnRecallLimit(topK)

	maxTurn := prepareTurnMaxObservedTurn(chatLogs, resumePack)
	resumeCue := prepareTurnQuerySuggestsResume(rawUserInput)
	thinMemoryRecall := prepareTurnNeedsRawFallback(memorySelection, topK)
	longSession := maxTurn >= 50 || prepareTurnProfileWide(profile)
	trace["status"] = "ready"
	trace["max_observed_turn"] = maxTurn
	trace["resume_query_cue"] = resumeCue
	trace["thin_memory_recall"] = thinMemoryRecall
	trace["long_session"] = longSession
	trace["profile"] = profile

	if resumePack.Chapter != nil {
		selectChapter := longSession || resumeCue || thinMemoryRecall || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectChapter {
			reason = prepareTurnHierarchyReason("chapter", longSession, resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Chapter.FromTurn, resumePack.Chapter.ToTurn)
			out.ChapterText = prepareTurnChapterRecallText(*resumePack.Chapter)
			trace["chapter_selected"] = strings.TrimSpace(out.ChapterText) != ""
			trace["chapter_reason"] = reason
			trace["chapter_mode"] = prepareTurnHierarchyMode(out.ChapterText)
			trace["chapter_range"] = map[string]int{"from_turn": resumePack.Chapter.FromTurn, "to_turn": resumePack.Chapter.ToTurn}
			trace["chapter_chars"] = len([]rune(strings.TrimSpace(out.ChapterText)))
		} else {
			trace["chapter_reason"] = reason
		}
	}
	if resumePack.Arc != nil {
		activeArc := strings.EqualFold(strings.TrimSpace(resumePack.Arc.ArcStatus), "active") || strings.TrimSpace(resumePack.Arc.ArcStatus) == ""
		selectArc := longSession || resumeCue || thinMemoryRecall || activeArc || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectArc {
			reason = prepareTurnHierarchyReason("arc", longSession || activeArc, resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Arc.FromTurn, resumePack.Arc.ToTurn)
			out.ArcText = prepareTurnArcRecallText(*resumePack.Arc)
			trace["arc_selected"] = strings.TrimSpace(out.ArcText) != ""
			trace["arc_reason"] = reason
			trace["arc_mode"] = prepareTurnHierarchyMode(out.ArcText)
			trace["arc_range"] = map[string]int{"from_turn": resumePack.Arc.FromTurn, "to_turn": resumePack.Arc.ToTurn}
			trace["arc_chars"] = len([]rune(strings.TrimSpace(out.ArcText)))
		} else {
			trace["arc_reason"] = reason
		}
	}
	if resumePack.Saga != nil {
		selectSaga := maxTurn >= 100 || resumeCue || thinMemoryRecall || prepareTurnProfileUltra(profile) || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectSaga {
			reason = prepareTurnHierarchyReason("saga", maxTurn >= 100 || prepareTurnProfileUltra(profile), resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Saga.FromTurn, resumePack.Saga.ToTurn)
			out.SagaText = prepareTurnSagaRecallText(*resumePack.Saga)
			trace["saga_selected"] = strings.TrimSpace(out.SagaText) != ""
			trace["saga_reason"] = reason
			trace["saga_mode"] = prepareTurnHierarchyMode(out.SagaText)
			trace["saga_range"] = map[string]int{"from_turn": resumePack.Saga.FromTurn, "to_turn": resumePack.Saga.ToTurn}
			trace["saga_chars"] = len([]rune(strings.TrimSpace(out.SagaText)))
		} else {
			trace["saga_reason"] = reason
		}
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

func prepareTurnProfileWide(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "wide", "ultra", "extreme", "wide_context_500k", "ultra_long_1m_plus", "extreme_long_2m_plus":
		return true
	default:
		return false
	}
}

func prepareTurnProfileUltra(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "ultra", "extreme", "ultra_long_1m_plus", "extreme_long_2m_plus":
		return true
	default:
		return false
	}
}

func prepareTurnQuerySuggestsResume(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false
	}
	for _, cue := range []string{"remember", "recap", "resume", "continue", "previous", "past", "long ago", "기억", "이전", "전에", "계속", "이어", "요약", "정리", "오랜만", "과거"} {
		if strings.Contains(raw, cue) {
			return true
		}
	}
	return false
}

func prepareTurnHierarchyReason(kind string, longSession, resumeCue, thinMemoryRecall, unknownTurn bool, fromTurn, toTurn int) string {
	reasons := []string{}
	if longSession {
		reasons = append(reasons, kind+"_continuity")
	}
	if resumeCue {
		reasons = append(reasons, "resume_query_cue")
	}
	if thinMemoryRecall {
		reasons = append(reasons, "thin_memory_recall_backstop")
	}
	if unknownTurn {
		reasons = append(reasons, "resume_pack_only_backstop")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, kind+"_available")
	}
	return strings.Join(reasons, "+") + fmt.Sprintf("_turns_%d_%d", fromTurn, toTurn)
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
