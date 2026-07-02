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

func completeTurnEvidenceLineage(source string, excerptIndex int, languageContext map[string]any) map[string]any {
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
	parsed := parseJSONMap(mem.SummaryJSON)
	summary := memorySummaryFromParsed(parsed)
	evidence := memorySearchEvidenceFromStoredMemory(mem)
	aliases := memorySearchAliasesFromExtraction(parsed)
	if mem.PlaceWing != "" {
		aliases = appendMemorySearchAlias(aliases, mem.PlaceWing)
	}
	if mem.PlaceRoom != "" {
		aliases = appendMemorySearchAlias(aliases, mem.PlaceRoom)
	}
	languageContext := completeTurnLanguageContextFromExtraction(parsed)
	return buildMemorySearchText(summary, evidence, aliases, languageContext)
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
