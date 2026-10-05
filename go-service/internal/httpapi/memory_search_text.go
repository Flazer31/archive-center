package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	languageMemoryContractVersion = "language_memory.v1"
	languageMemorySearchPolicy    = "summary_plus_raw_plus_aliases"
)

type memorySearchTextBuild struct {
	Text            string
	AliasCount      int
	LanguageContext map[string]any
}

// publicMemoryProjection is the deterministic, item-level view used by the
// general lexical/vector lane. Extraction remains the canonical full critic
// result; this projection deliberately omits holder/private material while
// leaving the typed precise-memory lanes untouched.
type publicMemoryProjection struct {
	Extraction map[string]any
	SearchText memorySearchTextBuild
	Eligible   bool
}

// buildPublicMemoryProjection is shared by fresh admission, legacy reindex,
// /search, and prepare-turn general recall. storedEvidence is the canonical
// Memory.Evidence JSON when rebuilding an existing row; fresh admission may
// pass an empty string because grounded evidence_excerpts are already present
// in extraction.
func buildPublicMemoryProjection(extraction map[string]any, storedEvidence string, content ...string) publicMemoryProjection {
	projected := map[string]any{}
	if len(extraction) == 0 {
		return publicMemoryProjection{Extraction: projected}
	}

	privateEvidence := memoryPerspectiveEvidenceScopeFromStored(extraction, storedEvidence)
	if len(content) > 0 {
		privateEvidence, _ = memoryAdmissionPerspectiveEvidenceScope(extraction, content...)
	}
	rawSummaryUnsafe := false
	for _, key := range []string{
		"belief_updates",
		"protected_secrets",
		"character_identity_accuracy",
		"subjective_entity_memories",
		"user_interaction_profile",
		"body_events",
	} {
		if len(sliceFromAny(extraction[key])) > 0 {
			rawSummaryUnsafe = true
		}
	}

	interactionKeys := []string{
		"interaction_events",
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"rp_character_profile",
	}
	objectiveKeys := []string{
		"character_deltas",
		"pending_threads",
		"world_rules",
		"reversible_states",
		"physical_conditions",
		"entity_conditions",
		"narrative_events",
		"state_claims",
	}
	for _, key := range append(append([]string{}, interactionKeys...), objectiveKeys...) {
		for _, raw := range sliceFromAny(extraction[key]) {
			item := mapFromAny(raw)
			if publicMemoryProjectionHasScopedMaterial(item) {
				rawSummaryUnsafe = true
			}
		}
	}

	publicInteractionCount := 0
	for _, key := range interactionKeys {
		items := []any{}
		for _, raw := range sliceFromAny(extraction[key]) {
			item := mapFromAny(raw)
			item = publicMemoryProjectionCitationItem(item, extraction, privateEvidence)
			clean, ok := publicMemoryProjectionValue(item, privateEvidence)
			if !ok {
				continue
			}
			items = append(items, clean)
			publicInteractionCount++
		}
		if len(items) > 0 {
			projected[key] = items
		}
	}
	// A mixed turn's free-form summary can blend public and private facts. Keep
	// each public objective item and rebuild only that free-form summary below.
	objectiveMaterialCount := 0
	for _, key := range objectiveKeys {
		items := []any{}
		for _, raw := range sliceFromAny(extraction[key]) {
			item := publicMemoryProjectionCitationItem(mapFromAny(raw), extraction, privateEvidence)
			clean, ok := publicMemoryProjectionValue(item, privateEvidence)
			if !ok {
				continue
			}
			items = append(items, clean)
			objectiveMaterialCount++
		}
		if len(items) > 0 {
			projected[key] = items
		}
	}
	skip := map[string]bool{
		// Retain the import provenance canonically; project its turn_summary
		// through the same public/private handling as every other memory.
		"hypamemory_import": true,
		"turn_summary":      true, "summary": true, "scene_summary": true,
		"core_meaning": true, "emotional_shift": true,
		"evidence_excerpts": true,
		"belief_updates":    true, "protected_secrets": true,
		"character_identity_accuracy": true, "subjective_entity_memories": true,
		"user_interaction_profile": true,
		"body_events":              true,
	}
	for _, key := range interactionKeys {
		skip[key] = true
	}
	for _, key := range objectiveKeys {
		skip[key] = true
	}
	for key, value := range extraction {
		if skip[key] {
			continue
		}
		clean, ok := publicMemoryProjectionValue(value, privateEvidence)
		if !ok {
			continue
		}
		projected[key] = clean
		switch key {
		case "importance_score", "emotional_intensity", "narrative_significance", "language_context", "memory_write_contract", "archive_hint", "temporal_context":
		default:
			objectiveMaterialCount++
		}
	}
	if rawSummaryUnsafe {
		if summary := publicMemoryProjectionNarrativeSummary(projected); summary != "" {
			projected["turn_summary"] = summary
			objectiveMaterialCount++
		}
	} else if summary := memorySummaryFromParsed(extraction); summary != "" {
		projected["turn_summary"] = summary
		objectiveMaterialCount++
	} else if summary := publicMemoryProjectionNarrativeSummary(projected); summary != "" {
		projected["turn_summary"] = summary
		objectiveMaterialCount++
	}

	for _, key := range []string{
		"importance_score",
		"emotional_intensity",
		"narrative_significance",
		"language_context",
		"memory_write_contract",
	} {
		if value, ok := extraction[key]; ok {
			if clean, keep := publicMemoryProjectionValue(value, privateEvidence); keep {
				projected[key] = clean
			}
		}
	}

	publicEvidence := []string{}
	appendEvidence := func(values any) {
		for _, excerpt := range memorySearchStringValues(values) {
			if memoryAdmissionPerspectiveEvidenceContains(privateEvidence, excerpt) {
				continue
			}
			publicEvidence = appendUniqueMemorySearchText(publicEvidence, excerpt)
		}
	}
	appendEvidence(extraction["evidence_excerpts"])
	appendEvidence(parseJSONMap(storedEvidence)["evidence_excerpts"])
	if len(publicEvidence) > 0 {
		projected["evidence_excerpts"] = append([]string(nil), publicEvidence...)
		// These excerpts already passed the existing item-level public projection.
		// An absent mixed free-form summary must not hide their aggregate memory.
		if memorySummaryFromParsed(projected) == "" {
			projected["turn_summary"] = strings.Join(publicEvidence, " ")
		}
	}

	summary := memorySummaryFromParsed(projected)
	aliases := memorySearchAliasesFromExtraction(projected)
	languageContext := completeTurnLanguageContextFromExtraction(projected)
	searchText := buildMemorySearchText(summary, publicEvidence, aliases, languageContext)
	eligible := strings.TrimSpace(searchText.Text) != "" &&
		(summary != "" || len(publicEvidence) > 0 || publicInteractionCount > 0 || objectiveMaterialCount > 0)
	return publicMemoryProjection{
		Extraction: projected,
		SearchText: searchText,
		Eligible:   eligible,
	}
}

// Retain the source attribution before the existing projection strips a private
// citation. Equal stored citation fields identify the source quotation, not the
// public claim's own knowledge scope or an accepted occurrence in raw text.
func publicMemoryProjectionCitationItem(item, extraction map[string]any, privateEvidence memoryPerspectiveEvidenceScope) map[string]any {
	boundaries := append([]any{}, sliceFromAny(item["knowledge_boundaries"])...)
	for index, raw := range sliceFromAny(extraction["protected_secrets"]) {
		source := mapFromAny(raw)
		if !protectedSecretRequiresGuard(source, "disclosure_policy") {
			continue
		}
		scope := mapFromAny(source["knowledge_scope"])
		quote := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(source, "evidence_excerpt"), stringFromMap(source, "evidence"), stringFromMap(source, "source_excerpt")))
		if quote == "" || len(scope) == 0 {
			continue
		}
		linked := false
		for _, field := range []string{"evidence_excerpt", "evidence", "source_excerpt", "identity_evidence_excerpt"} {
			citation := strings.TrimSpace(stringFromMap(item, field))
			if citation == quote && memoryAdmissionPerspectiveEvidenceContains(privateEvidence, citation) {
				linked = true
				break
			}
		}
		if !linked {
			continue
		}
		origin := cloneMapAny(mapFromAny(source["knowledge_source"]))
		if origin == nil {
			origin = map[string]any{}
		}
		if _, exists := origin["source"]; !exists {
			origin["source"] = "critic.protected_secrets"
		}
		if _, exists := origin["source_index"]; !exists {
			origin["source_index"] = index
		}
		ref := extractionFirstNonEmpty(stringFromMap(source, "protected_fact_ref"), fmt.Sprintf("critic.protected_secrets/%d", index))
		if revision := stringFromMap(origin, "source_revision"); revision != "" {
			ref = extractionFirstNonEmpty(stringFromMap(source, "protected_fact_ref"), fmt.Sprintf("source-revision:%s/protected_secrets/%d", revision, intFromAny(origin["source_index"], index)))
		}
		boundary := map[string]any{"knowledge_scope": cloneMapAny(scope), "knowledge_source": origin,
			"protected_fact_ref": ref, "fact_type": "protected_secrets", "owner": stringFromMap(source, "owner")}
		for _, field := range []string{"source_refs", "fact_ref", "fact_id", "canonical_fact_id", "secret_id", "artifact_id"} {
			if value, exists := source[field]; exists {
				boundary[field] = value
			}
		}
		duplicate := false
		for _, previous := range boundaries {
			duplicate = duplicate || mustCompactJSON(previous) == mustCompactJSON(boundary)
		}
		if !duplicate {
			boundaries = append(boundaries, boundary)
		}
	}
	if len(boundaries) == len(sliceFromAny(item["knowledge_boundaries"])) {
		return item
	}
	out := cloneMapAny(item)
	out["knowledge_boundaries"] = boundaries
	return out
}

// Retained for the admission caller; publicMemoryProjectionValue owns the
// only item-level exclusion policy.
func publicMemoryProjectionInteractionEligible(item map[string]any) bool {
	clean, ok := publicMemoryProjectionValue(item, memoryPerspectiveEvidenceScope{})
	return ok && len(mapFromAny(clean)) > 0
}

func publicMemoryProjectionHasScopedMaterial(value any) bool {
	item := mapFromAny(value)
	visibility := strings.ToLower(strings.TrimSpace(stringFromMap(item, "visibility")))
	if visibility == "owner_private" || visibility == "restricted" || visibility == "user_private" {
		return true
	}
	if boolFromAny(item["secret_guard"]) || boolFromAny(item["privacy_guard"]) {
		return true
	}
	sensitivity := strings.ToLower(strings.TrimSpace(stringFromMap(item, "sensitivity")))
	if strings.Contains(sensitivity, "secret") || strings.Contains(sensitivity, "private") || strings.Contains(sensitivity, "protected") {
		return true
	}
	return false
}

func publicMemoryProjectionValue(value any, privateEvidence memoryPerspectiveEvidenceScope) (any, bool) {
	if value == nil {
		return nil, false
	}
	switch item := value.(type) {
	case map[string]any:
		if publicMemoryProjectionHasScopedMaterial(item) {
			return nil, false
		}
		out := map[string]any{}
		for key, nested := range item {
			switch key {
			case "knowledge_boundaries":
				// Source boundaries are attributed metadata, not public prose.
				// Preserve their scope arrays and receipts without copying bodies.
				boundaries := []any{}
				for _, boundary := range prepareTurnOwnKnowledgeBoundary(map[string]any{key: nested}) {
					boundaries = append(boundaries, boundary)
				}
				out[key] = boundaries
				continue
			case "evidence_excerpt", "evidence", "source_excerpt", "identity_evidence_excerpt":
				// Retain the independently expressed public fact, while its
				// private source quotation stays in canonical/typed storage.
				if memoryAdmissionPerspectiveEvidenceContains(privateEvidence, extractionStringFromAny(nested)) {
					continue
				}
			case "event", "summary", "description", "text", "content", "observation", "change":
				// Narrative source copies must not rebuild a public summary.
				// Typed public values/profile meanings retain their independent
				// classification even when their private citation contains them.
				if strings.EqualFold(strings.TrimSpace(stringFromMap(item, "visibility")), "public") {
					break // Explicit public meaning survives a mixed private citation.
				}
				if text, ok := nested.(string); ok && memoryAdmissionPerspectiveEvidenceContains(privateEvidence, text) {
					continue
				}
			}
			if clean, ok := publicMemoryProjectionValue(nested, privateEvidence); ok {
				out[key] = clean
			}
		}
		return out, len(out) > 0
	case []any:
		out := []any{}
		for _, nested := range item {
			if clean, ok := publicMemoryProjectionValue(nested, privateEvidence); ok {
				out = append(out, clean)
			}
		}
		return out, len(out) > 0
	case []map[string]any:
		out := []any{}
		for _, nested := range item {
			if clean, ok := publicMemoryProjectionValue(nested, privateEvidence); ok {
				out = append(out, clean)
			}
		}
		return out, len(out) > 0
	case []string:
		out := []string{}
		for _, nested := range item {
			if clean := strings.TrimSpace(nested); clean != "" {
				out = append(out, clean)
			}
		}
		return out, len(out) > 0
	case string:
		return item, strings.TrimSpace(item) != ""
	default:
		return item, true
	}
}

func publicMemoryProjectionNarrativeSummary(projected map[string]any) string {
	parts := []string{}
	for _, raw := range sliceFromAny(projected["narrative_events"]) {
		item := mapFromAny(raw)
		for _, key := range []string{"summary", "event", "description"} {
			text := strings.TrimSpace(extractionStringFromAny(item[key]))
			if text != "" && !looksLikeStructuredCriticPayloadText(text) {
				parts = appendUniqueMemorySearchText(parts, text)
				break
			}
		}
	}
	for _, key := range []string{
		"interaction_events",
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"rp_character_profile",
	} {
		for _, raw := range sliceFromAny(projected[key]) {
			item := mapFromAny(raw)
			semanticParts := []string{}
			for _, field := range []string{"subject_entity", "subject", "actor", "source_entity", "character"} {
				if text := strings.TrimSpace(extractionStringFromAny(item[field])); text != "" {
					semanticParts = appendUniqueMemorySearchText(semanticParts, text)
					break
				}
			}
			for _, field := range []string{"target_entity", "counterpart"} {
				if text := strings.TrimSpace(extractionStringFromAny(item[field])); text != "" {
					semanticParts = appendUniqueMemorySearchText(semanticParts, text)
					break
				}
			}
			for _, field := range []string{
				"summary", "description", "observation", "supported_expression",
				"utterance_expression", "action", "behavior_key", "trait_key", "principle_key",
			} {
				text := strings.TrimSpace(extractionStringFromAny(item[field]))
				if text != "" && !looksLikeStructuredCriticPayloadText(text) {
					semanticParts = appendUniqueMemorySearchText(semanticParts, text)
					break
				}
			}
			if len(semanticParts) > 0 {
				parts = appendUniqueMemorySearchText(parts, strings.Join(semanticParts, " | "))
			}
		}
	}
	for _, raw := range sliceFromAny(projected["state_claims"]) {
		item := mapFromAny(raw)
		described := false
		for _, key := range []string{"summary", "description", "change", "observation"} {
			text := strings.TrimSpace(extractionStringFromAny(item[key]))
			if text != "" && !looksLikeStructuredCriticPayloadText(text) {
				parts = appendUniqueMemorySearchText(parts, text)
				described = true
				break
			}
		}
		if described {
			continue
		}
		subject := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "subject"),
			stringFromMap(item, "entity"),
		))
		slot := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "state_slot"),
			stringFromMap(item, "key"),
		))
		value := strings.TrimSpace(extractionStringFromAny(item["value"]))
		if subject != "" && (slot != "" || value != "") {
			parts = appendUniqueMemorySearchText(parts, strings.TrimSpace(subject+" "+slot+": "+value))
			continue
		}
		if evidence := interactionAdmissionEvidence(item); evidence != "" {
			parts = appendUniqueMemorySearchText(parts, evidence)
		}
	}
	return strings.Join(parts, " ")
}

func publicMemoryFromCanonical(mem store.Memory) (store.Memory, bool) {
	projection := buildPublicMemoryProjection(parseJSONMap(mem.SummaryJSON), mem.Evidence)
	if !projection.Eligible {
		return store.Memory{}, false
	}
	out := mem
	out.SummaryJSON = mustCompactJSON(projection.Extraction)
	out.Evidence = mustCompactJSON(map[string]any{
		"evidence_excerpts": stringsFromAny(projection.Extraction["evidence_excerpts"]),
	})
	return out, true
}

func completeTurnLanguageContextFromClientMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	return normalizeCompleteTurnLanguageContext(mapFromAny(meta["language_context"]))
}

func completeTurnLanguageContextFromExtraction(extraction map[string]any) map[string]any {
	if len(extraction) == 0 {
		return nil
	}
	return normalizeCompleteTurnLanguageContext(mapFromAny(extraction["language_context"]))
}

func normalizeCompleteTurnLanguageContext(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := map[string]any{
		"contract_version":       extractionFirstNonEmpty(stringFromMap(raw, "contract_version"), languageMemoryContractVersion),
		"search_text_policy":     extractionFirstNonEmpty(stringFromMap(raw, "search_text_policy"), languageMemorySearchPolicy),
		"raw_evidence_rewritten": false,
	}
	for _, key := range []string{
		"session_output_language",
		"output_language_source",
		"ui_language",
		"raw_user_language",
		"assistant_output_language",
		"summary_language",
	} {
		if value := strings.TrimSpace(extractionStringFromAny(raw[key])); value != "" {
			out[key] = truncateRunes(value, 64)
		}
	}
	if _, ok := raw["locked_for_turn"]; ok {
		out["locked_for_turn"] = boolFromAny(raw["locked_for_turn"])
	}
	if _, ok := raw["confidence"]; ok {
		out["confidence"] = clampFloat(extractionFloatFromAny(raw["confidence"], 0), 0, 1)
	}
	if values := stringsFromAny(raw["violations"]); len(values) > 0 {
		out["violations"] = values
	}
	return out
}

func completeTurnMemoryWriteContract(languageContext map[string]any) map[string]any {
	if len(languageContext) == 0 {
		return nil
	}
	return map[string]any{
		"contract_version":         languageMemoryContractVersion,
		"raw_evidence_lane":        "raw_evidence",
		"raw_evidence_rewritten":   false,
		"canonical_summary_lane":   "canonical_summary",
		"summary_language":         extractionFirstNonEmpty(extractionStringFromAny(languageContext["summary_language"]), extractionStringFromAny(languageContext["session_output_language"])),
		"search_text_policy":       extractionFirstNonEmpty(extractionStringFromAny(languageContext["search_text_policy"]), languageMemorySearchPolicy),
		"internal_key_policy":      "stable_keys_not_translated_per_turn",
		"applied_to_current_write": true,
	}
}

func completeTurnEvidenceLineage(source string, excerptIndex int, languageContext map[string]any, inputMode ...string) map[string]any {
	lineage := map[string]any{
		"source":                     source,
		"excerpt_index":              excerptIndex,
		"auto_verify_policy_version": "p1245.grounded_excerpt.v1",
	}
	if len(languageContext) > 0 {
		lineage["lane"] = "raw_evidence"
		lineage["raw_evidence_rewritten"] = false
		lineage["language_context"] = languageContext
	}
	if len(inputMode) > 0 && strings.TrimSpace(inputMode[0]) == "assistant_only" {
		lineage["source_role"] = "assistant_output"
		lineage["user_input_state"] = "missing"
	}
	return lineage
}

func applyLanguageMemoryWriteContract(extraction map[string]any, languageContext map[string]any) map[string]any {
	if len(extraction) == 0 || len(languageContext) == 0 {
		return extraction
	}
	extraction["language_context"] = languageContext
	extraction["memory_write_contract"] = completeTurnMemoryWriteContract(languageContext)
	return extraction
}

func completeTurnMemorySearchText(summary string, extraction map[string]any, content string) memorySearchTextBuild {
	if summary == "" {
		summary = memorySummaryFromParsed(extraction)
	}
	evidence := memorySearchEvidenceFromExtraction(extraction, content)
	aliases := memorySearchAliasesFromExtraction(extraction)
	languageContext := completeTurnLanguageContextFromExtraction(extraction)
	return buildMemorySearchText(summary, evidence, aliases, languageContext)
}

func memorySearchTextFromMemory(mem store.Memory) memorySearchTextBuild {
	return buildPublicMemoryProjection(parseJSONMap(mem.SummaryJSON), mem.Evidence).SearchText
}

func memorySummaryFromParsed(parsed map[string]any) string {
	if len(parsed) == 0 {
		return ""
	}
	if summary := normalizeCriticTurnSummary(parsed["turn_summary"]); summary != "" {
		return summary
	}
	for _, key := range []string{"summary", "scene_summary", "core_meaning", "emotional_shift", "content", "text"} {
		value := strings.TrimSpace(extractionStringFromAny(parsed[key]))
		if looksLikeStructuredCriticPayloadText(value) {
			continue
		}
		if value != "" {
			return value
		}
	}
	return ""
}

func buildMemorySearchText(summary string, evidence []string, aliases []string, languageContext map[string]any) memorySearchTextBuild {
	parts := []string{}
	seen := map[string]bool{}
	appendMemorySearchTextPart(&parts, seen, "Canonical Summary", summary)
	if len(evidence) > 0 {
		appendMemorySearchTextPart(&parts, seen, "Raw Evidence", strings.Join(evidence, "\n"))
	}
	if len(aliases) > 0 {
		appendMemorySearchTextPart(&parts, seen, "Aliases", strings.Join(aliases, "\n"))
	}
	return memorySearchTextBuild{
		Text:            strings.TrimSpace(strings.Join(parts, "\n\n")),
		AliasCount:      len(aliases),
		LanguageContext: languageContext,
	}
}

func appendMemorySearchTextPart(parts *[]string, seen map[string]bool, label string, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	key := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*parts = append(*parts, fmt.Sprintf("[%s]\n%s", label, text))
}

func memorySearchEvidenceFromExtraction(extraction map[string]any, content string) []string {
	out := []string{}
	for _, excerpt := range memorySearchStringValues(extraction["evidence_excerpts"]) {
		if clean := groundedMemorySearchEvidence(excerpt, content); clean != "" {
			out = appendUniqueMemorySearchText(out, clean)
		}
	}
	return out
}

func memorySearchEvidenceFromStoredMemory(mem store.Memory) []string {
	out := []string{}
	evidence := parseJSONMap(mem.Evidence)
	for _, excerpt := range memorySearchStringValues(evidence["evidence_excerpts"]) {
		if clean := strings.TrimSpace(excerpt); clean != "" {
			out = appendUniqueMemorySearchText(out, clean)
		}
	}
	return out
}

func groundedMemorySearchEvidence(excerpt string, content string) string {
	text := strings.TrimSpace(excerpt)
	if text == "" {
		return ""
	}
	turn := strings.TrimSpace(content)
	if turn == "" {
		return text
	}
	compactText := strings.Join(strings.Fields(text), " ")
	compactTurn := strings.Join(strings.Fields(turn), " ")
	if compactText == "" || compactText == compactTurn {
		return ""
	}
	if !strings.Contains(turn, text) && !strings.Contains(compactTurn, compactText) {
		return ""
	}
	return text
}

func memorySearchAliasesFromExtraction(extraction map[string]any) []string {
	aliases := []string{}
	for _, key := range []string{
		"characters", "character_names", "people", "places", "locations", "items", "factions", "keywords", "tags",
	} {
		for _, value := range memorySearchStringValues(extraction[key]) {
			aliases = appendMemorySearchAlias(aliases, value)
		}
	}
	archiveHint := mapFromAny(extraction["archive_hint"])
	for _, key := range []string{"wing", "room", "section", "shelf"} {
		aliases = appendMemorySearchAlias(aliases, stringFromMap(archiveHint, key))
	}
	for _, item := range memorySearchMapItems(extraction["entities"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"name", "canonical_name", "display_name", "role", "entity_type", "type", "location"})
	}
	for _, item := range memorySearchMapItems(extraction["character_states"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"name", "role", "location", "status_emotion"})
	}
	for _, item := range memorySearchMapItems(extraction["kg_triples"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"subject", "predicate", "object"})
	}
	for _, item := range memorySearchMapItems(extraction["world_rules"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"category", "key", "scope", "scope_name"})
	}
	for _, item := range memorySearchMapItems(extraction["storylines"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"name", "title"})
	}
	for _, item := range memorySearchMapItems(extraction["pending_threads"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"name", "title", "thread", "goal"})
	}
	for _, item := range memorySearchMapItems(extraction["protected_secrets"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{"secret_kind", "owner", "sensitivity", "evidence_strength", "disclosure_policy"})
		for _, subject := range memorySearchStringValues(item["subject"]) {
			aliases = appendMemorySearchAlias(aliases, subject)
		}
		aliases = appendMemorySearchKnowledgeScopeAliases(aliases, item["knowledge_scope"])
	}
	for _, item := range memorySearchMapItems(extraction["character_identity_accuracy"]) {
		aliases = appendMemorySearchMapAliases(aliases, item, []string{
			"canonical_entity_name",
			"surface_identity_name",
			"true_identity_name",
			"public_identity_name",
			"alias_name",
			"real_identity_name",
			"identity_kind",
			"public_role",
			"true_role",
			"public_allegiance",
			"true_allegiance",
			"reveal_policy",
		})
		aliases = appendMemorySearchKnowledgeScopeAliases(aliases, item["knowledge_scope"])
	}
	return aliases
}

func memorySearchMapItems(value any) []map[string]any {
	out := []map[string]any{}
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		for _, item := range items {
			if m := mapFromAny(item); len(m) > 0 {
				out = append(out, m)
			}
		}
	case map[string]any:
		out = append(out, items)
	}
	return out
}

func appendMemorySearchMapAliases(aliases []string, item map[string]any, keys []string) []string {
	for _, key := range keys {
		aliases = appendMemorySearchAlias(aliases, item[key])
	}
	for _, alias := range memorySearchStringValues(item["aliases"]) {
		aliases = appendMemorySearchAlias(aliases, alias)
	}
	return aliases
}

func appendMemorySearchKnowledgeScopeAliases(aliases []string, value any) []string {
	scope := mapFromAny(value)
	for _, key := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
		for _, item := range memorySearchStringValues(scope[key]) {
			aliases = appendMemorySearchAlias(aliases, item)
		}
	}
	return aliases
}

func memorySearchStringValues(value any) []string {
	switch items := value.(type) {
	case []string:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if clean := strings.TrimSpace(item); clean != "" {
				out = append(out, clean)
			}
		}
		return out
	case string:
		if clean := strings.TrimSpace(items); clean != "" {
			return []string{clean}
		}
	case []any:
		out := []string{}
		for _, item := range items {
			switch item.(type) {
			case map[string]any, []any:
				continue
			}
			if clean := strings.TrimSpace(extractionStringFromAny(item)); clean != "" {
				out = append(out, clean)
			}
		}
		return out
	}
	return nil
}

func appendMemorySearchAlias(aliases []string, value any) []string {
	return appendUniqueMemorySearchText(aliases, extractionStringFromAny(value))
}

func appendUniqueMemorySearchText(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	key := strings.ToLower(strings.Join(strings.Fields(value), " "))
	for _, existing := range items {
		if strings.ToLower(strings.Join(strings.Fields(existing), " ")) == key {
			return items
		}
	}
	return append(items, value)
}

func memoryVectorLanguageMetadata(mem store.Memory) map[string]string {
	parsed := parseJSONMap(mem.SummaryJSON)
	languageContext := completeTurnLanguageContextFromExtraction(parsed)
	contract := mapFromAny(parsed["memory_write_contract"])
	return memoryVectorLanguageMetadataFromContext(languageContext, contract)
}

func memoryVectorLanguageMetadataFromContext(languageContext map[string]any, contract map[string]any) map[string]string {
	out := map[string]string{
		"search_text_policy": extractionFirstNonEmpty(extractionStringFromAny(languageContext["search_text_policy"]), extractionStringFromAny(contract["search_text_policy"]), languageMemorySearchPolicy),
	}
	for key, value := range map[string]string{
		"raw_language":              extractionFirstNonEmpty(extractionStringFromAny(languageContext["raw_user_language"]), extractionStringFromAny(languageContext["raw_language"])),
		"summary_language":          extractionFirstNonEmpty(extractionStringFromAny(languageContext["summary_language"]), extractionStringFromAny(contract["summary_language"])),
		"session_output_language":   extractionStringFromAny(languageContext["session_output_language"]),
		"output_language_source":    extractionStringFromAny(languageContext["output_language_source"]),
		"assistant_output_language": extractionStringFromAny(languageContext["assistant_output_language"]),
	} {
		if clean := strings.TrimSpace(value); clean != "" {
			out[key] = clean
		}
	}
	return out
}
