package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	prepareTurnPriorityMemoryPlanVersion  = "memory_delivery_plan.v2"
	prepareTurnPriorityMemoryScoreVersion = "priority_score.static.v1"
	prepareTurnFinalizationImmediate      = "immediate_after_response"
	prepareTurnFinalizationNextInput      = "next_user_input"
)

func normalizePrepareTurnFinalizationMode(value string) string {
	if strings.TrimSpace(value) == prepareTurnFinalizationNextInput {
		return prepareTurnFinalizationNextInput
	}
	return prepareTurnFinalizationImmediate
}

func buildPrepareTurnFinalizationPolicy(value string) map[string]any {
	mode := normalizePrepareTurnFinalizationMode(value)
	return map[string]any{
		"contract_version":           "turn_finalization_policy.v1",
		"owner":                      "go",
		"mode":                       mode,
		"confirmed_memory_horizon":   map[bool]string{true: "through_previous_confirmed_turn", false: "through_current_confirmed_turn"}[mode == prepareTurnFinalizationNextInput],
		"previous_turn_critic":       map[bool]string{true: "pipeline_with_current_generation", false: "after_current_response"}[mode == prepareTurnFinalizationNextInput],
		"current_generation_blocked": false,
	}
}

type prepareTurnPriorityMemoryInput struct {
	Lane        string
	SourceTable string
	Text        string
	Tier        string
}

type prepareTurnPrioritySourceMetadata struct {
	SourceTable       string
	LineKey           string
	SourceRowID       any
	SourceOccurrence  string
	SourceTurn        int
	Importance        float64
	ImportancePresent bool
}

type prepareTurnPriorityMemoryFact struct {
	Text          string
	FamilyKey     string
	ValueKey      string
	EntitySurface string
	Structured    bool
}

type prepareTurnPriorityMemoryCandidate struct {
	CanonicalFactID  string
	CanonicalKey     string
	SourceTable      string
	SourceRef        string
	SourceRowID      any
	SourceOccurrence string
	Lane             string
	Tier             string
	CompleteText     string
	SourceTurn       int
	Relevance        float64
	Importance       float64
	Recency          float64
	ContinuityBonus  float64
	FinalScore       float64
	FinalRank        int
	Chars            int
	SelectionStatus  string
	SelectionReason  string
	SupersededBy     string
}

func prepareTurnPriorityMemoryInputs(out *prepareTurnInjectionAssembly) []prepareTurnPriorityMemoryInput {
	return []prepareTurnPriorityMemoryInput{
		{Lane: "event_recent", SourceTable: "memories", Text: out.ActualMemoryText, Tier: "required"},
		{Lane: "event_recent", SourceTable: "episode_summaries", Text: out.EpisodeText, Tier: "auxiliary"},
		{Lane: "event_recent", SourceTable: "chapter_summaries", Text: out.ChapterText, Tier: "auxiliary"},
		{Lane: "event_recent", SourceTable: "arc_summaries", Text: out.ArcText, Tier: "auxiliary"},
		{Lane: "event_recent", SourceTable: "saga_digests", Text: out.SagaText, Tier: "auxiliary"},
		{Lane: "event_recent", SourceTable: "canonical_state_layers", Text: out.CanonEventText, Tier: "required"},
		{Lane: "character_objective", SourceTable: "character_states", Text: out.CharacterObjectiveText, Tier: "required"},
		{Lane: "character_objective", SourceTable: "canonical_state_layers", Text: out.CanonCharacterText, Tier: "required"},
		{Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", Text: out.CharacterPrivateText, Tier: "required"},
		{Lane: "subjective_relationship", SourceTable: "character_states", Text: out.CharacterRelationshipText, Tier: "required"},
		{Lane: "subjective_relationship", SourceTable: "canonical_state_layers", Text: out.CanonRelationshipText, Tier: "required"},
		{Lane: "subjective_relationship", SourceTable: "persona_memory_entries", Text: out.PersonaText, Tier: "auxiliary"},
		{Lane: "subjective_relationship", SourceTable: "kg_triples", Text: out.KGText, Tier: "auxiliary"},
		{Lane: "world_state", SourceTable: "canonical_state_layers", Text: out.CanonWorldText, Tier: "required"},
		{Lane: "world_state", SourceTable: "world_rules", Text: out.WorldRulesText, Tier: "required"},
		{Lane: "unresolved_goal", SourceTable: "pending_threads", Text: out.PendingThreadText, Tier: "required"},
		{Lane: "unresolved_goal", SourceTable: "storylines", Text: out.StorylineText, Tier: "auxiliary"},
	}
}

func appendPrepareTurnPrioritySourceMetadata(out *prepareTurnInjectionAssembly, sourceTable, line, sourceOccurrence string, sourceRowID any, sourceTurn int, importance float64, importancePresent bool) {
	if out == nil || strings.TrimSpace(line) == "" {
		return
	}
	out.PrioritySourceMetadata = append(out.PrioritySourceMetadata, prepareTurnPrioritySourceMetadata{
		SourceTable:       strings.TrimSpace(sourceTable),
		LineKey:           prepareTurnPriorityCleanLine(line),
		SourceRowID:       sourceRowID,
		SourceOccurrence:  strings.TrimSpace(sourceOccurrence),
		SourceTurn:        sourceTurn,
		Importance:        prepareTurnPriorityNormalizeScore(importance),
		ImportancePresent: importancePresent,
	})
}

func prepareTurnPriorityStoredOccurrence(sourceTable string, sourceRowID int64, suffix string) string {
	if sourceRowID <= 0 {
		return ""
	}
	occurrence := fmt.Sprintf("%s:%d", strings.TrimSpace(sourceTable), sourceRowID)
	if strings.TrimSpace(suffix) != "" {
		occurrence += ":" + strings.TrimSpace(suffix)
	}
	return occurrence
}

func prepareTurnPriorityStoredRowID(sourceRowID int64) any {
	if sourceRowID <= 0 {
		return nil
	}
	return sourceRowID
}

func prepareTurnPrioritySourceMetadataByText(out *prepareTurnInjectionAssembly) map[string][]prepareTurnPrioritySourceMetadata {
	indexed := map[string][]prepareTurnPrioritySourceMetadata{}
	if out == nil {
		return indexed
	}
	for _, metadata := range out.PrioritySourceMetadata {
		key := metadata.SourceTable + "\x1f" + metadata.LineKey
		indexed[key] = append(indexed[key], metadata)
	}
	return indexed
}

func prepareTurnPriorityCleanLine(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	if !strings.HasPrefix(line, "[") {
		return line
	}
	end := strings.Index(line, "]")
	if end < 0 {
		return line
	}
	metadata := strings.ToLower(strings.TrimSpace(line[1:end]))
	if strings.Contains(metadata, "turn") || strings.Contains(metadata, "vector") || strings.Contains(metadata, "score") || strings.Contains(metadata, "source_") || strings.Contains(metadata, "valid=") {
		return strings.TrimSpace(line[end+1:])
	}
	return line
}

func prepareTurnPriorityIdentityPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	open := strings.LastIndex(prefix, "[")
	if open < 0 || !strings.HasSuffix(prefix, "]") {
		return prefix
	}
	metadata := strings.ToLower(prefix[open+1 : len(prefix)-1])
	if strings.Contains(metadata, "turn=") || strings.Contains(metadata, "turn ") || strings.Contains(metadata, "latest_observed") || strings.Contains(metadata, "historical") {
		return strings.TrimSpace(prefix[:open])
	}
	return prefix
}

func prepareTurnPriorityTopLevelParts(text string, separator rune) []string {
	parts := []string{}
	start := 0
	depth := 0
	inString := false
	escaped := false
	runes := []rune(text)
	for index, r := range runes {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		default:
			if r == separator && depth == 0 {
				part := strings.TrimSpace(string(runes[start:index]))
				if part != "" {
					parts = append(parts, part)
				}
				start = index + 1
			}
		}
	}
	if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func prepareTurnPriorityScalarText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if math.Trunc(typed) == typed {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		encoded, _ := json.Marshal(typed)
		return strings.TrimSpace(string(encoded))
	}
}

func prepareTurnPriorityFlattenValue(prefix string, path []string, value any, facts *[]prepareTurnPriorityMemoryFact) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			prepareTurnPriorityFlattenValue(prefix, append(append([]string{}, path...), key), typed[key], facts)
		}
	case []any:
		for index, item := range typed {
			prepareTurnPriorityFlattenValue(prefix, append(append([]string{}, path...), fmt.Sprintf("item_%d", index+1)), item, facts)
		}
	default:
		valueText := prepareTurnPriorityScalarText(typed)
		if valueText == "" {
			return
		}
		labelParts := append([]string{}, path...)
		label := strings.Join(labelParts, " · ")
		text := strings.TrimSpace(prefix)
		if label != "" {
			if text != "" {
				text += " · "
			}
			text += label
		}
		if text != "" {
			text += ": "
		}
		text += valueText
		family := collapseTextKey(strings.TrimSpace(prefix + "\x1f" + strings.Join(path, "\x1f")))
		*facts = append(*facts, prepareTurnPriorityMemoryFact{
			Text: strings.TrimSpace(text), FamilyKey: family,
			ValueKey: collapseTextKey(valueText), Structured: true,
		})
	}
}

func prepareTurnPriorityParseJSONFacts(prefix, raw string) []prepareTurnPriorityMemoryFact {
	var value any
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &value) != nil {
		return nil
	}
	facts := []prepareTurnPriorityMemoryFact{}
	prepareTurnPriorityFlattenValue(strings.TrimSpace(prefix), nil, value, &facts)
	return facts
}

func prepareTurnPriorityAttachEntitySurface(facts []prepareTurnPriorityMemoryFact, surface string) []prepareTurnPriorityMemoryFact {
	for index := range facts {
		facts[index].EntitySurface = strings.TrimSpace(surface)
	}
	return facts
}

func prepareTurnPrioritySplitFact(line string) []prepareTurnPriorityMemoryFact {
	clean := prepareTurnPriorityCleanLine(line)
	if clean == "" {
		return nil
	}
	if colon := strings.Index(clean, ":"); colon > 0 {
		prefix := prepareTurnPriorityIdentityPrefix(clean[:colon])
		raw := strings.TrimSpace(clean[colon+1:])
		if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
			if facts := prepareTurnPriorityParseJSONFacts(prefix, raw); len(facts) > 0 {
				return prepareTurnPriorityAttachEntitySurface(facts, prefix)
			}
		}
		parts := prepareTurnPriorityTopLevelParts(raw, ';')
		structured := []prepareTurnPriorityMemoryFact{}
		for _, part := range parts {
			equals := strings.Index(part, "=")
			if equals <= 0 {
				structured = nil
				break
			}
			field := strings.TrimSpace(part[:equals])
			value := strings.TrimSpace(part[equals+1:])
			factPrefix := strings.TrimSpace(prefix + " · " + field)
			if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
				facts := prepareTurnPriorityParseJSONFacts(factPrefix, value)
				if len(facts) == 0 {
					structured = nil
					break
				}
				structured = append(structured, facts...)
				continue
			}
			if value == "" {
				continue
			}
			structured = append(structured, prepareTurnPriorityMemoryFact{
				Text: factPrefix + ": " + value, EntitySurface: prefix,
				FamilyKey: collapseTextKey(factPrefix), ValueKey: collapseTextKey(value), Structured: true,
			})
		}
		if len(structured) > 0 {
			return prepareTurnPriorityAttachEntitySurface(structured, prefix)
		}
		key := collapseTextKey(clean)
		return []prepareTurnPriorityMemoryFact{{Text: clean, FamilyKey: key, ValueKey: key, EntitySurface: prefix}}
	}
	key := collapseTextKey(clean)
	return []prepareTurnPriorityMemoryFact{{Text: clean, FamilyKey: key, ValueKey: key}}
}

func prepareTurnPriorityCanonicalEntityIdentity(fact prepareTurnPriorityMemoryFact, aliases map[string]any) (string, bool) {
	surface := strings.TrimSpace(fact.EntitySurface)
	if surface == "" || len(aliases) == 0 {
		return fact.FamilyKey, false
	}
	canonical := ""
	for alias, rawCanonical := range aliases {
		if comparableEntityKey(alias) == comparableEntityKey(surface) {
			canonical = strings.TrimSpace(extractionStringFromAny(rawCanonical))
			break
		}
	}
	if canonical == "" {
		return fact.FamilyKey, false
	}
	surfaceKey := collapseTextKey(surface)
	suffix := strings.TrimSpace(strings.TrimPrefix(fact.FamilyKey, surfaceKey))
	return strings.TrimSpace("entity:" + comparableEntityKey(canonical) + "\x1f" + suffix), true
}

func prepareTurnPriorityUsesCanonicalFieldIdentity(sourceTable string, fact prepareTurnPriorityMemoryFact) bool {
	if !fact.Structured {
		return false
	}
	switch sourceTable {
	case "canonical_state_layers", "character_states", "world_rules":
		return true
	default:
		return false
	}
}

func prepareTurnPriorityNormalizeScore(value float64) float64 {
	if value > 1 {
		value /= 10
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func prepareTurnPriorityRelevance(query, text string) float64 {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0.5
	}
	queryTerms := prepareTurnDistinctiveRecallTerms(query)
	if len(queryTerms) == 0 {
		queryTerms = prepareTurnRecallTerms(query)
	}
	if len(queryTerms) == 0 {
		return 0.5
	}
	overlap := prepareTurnDistinctiveRecallOverlapCount(queryTerms, text)
	denominator := minInt(len(queryTerms), 4)
	if denominator < 1 {
		denominator = 1
	}
	score := float64(overlap) / float64(denominator)
	if prepareTurnRecallContainsAnchor(text, query) {
		score = 1
	}
	if score > 1 {
		score = 1
	}
	return score
}

func prepareTurnPriorityContinuityBonus(lane, sourceTable, text string) float64 {
	bonus := 0.0
	switch lane {
	case "character_objective", "world_state":
		bonus = 0.04
	case "unresolved_goal":
		bonus = 0.03
	}
	if sourceTable == "canonical_state_layers" {
		bonus += 0.04
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{"completed", "complete", "resolved", "finished", "done", "완료", "해결", "종료"} {
		if strings.Contains(lower, marker) {
			bonus += 0.04
			break
		}
	}
	return bonus
}

func prepareTurnPriorityLifecycleRank(text string) int {
	lower := strings.ToLower(text)
	for _, marker := range []string{"completed", "complete", "resolved", "finished", "done", "완료", "해결", "종료"} {
		if strings.Contains(lower, marker) {
			return 3
		}
	}
	for _, marker := range []string{"active", "started", "progress", "진행", "시작"} {
		if strings.Contains(lower, marker) {
			return 2
		}
	}
	for _, marker := range []string{"planned", "plan", "pending", "open", "계획", "예정", "대기"} {
		if strings.Contains(lower, marker) {
			return 1
		}
	}
	return 0
}

func prepareTurnPrioritySourceTurn(line string) int {
	lower := strings.ToLower(line)
	for _, marker := range []string{"source_turn=", "turn=", "turn "} {
		start := strings.Index(lower, marker)
		if start < 0 {
			continue
		}
		start += len(marker)
		end := start
		for end < len(lower) && lower[end] >= '0' && lower[end] <= '9' {
			end++
		}
		if end > start {
			value, _ := strconv.Atoi(lower[start:end])
			return value
		}
	}
	return 0
}

func prepareTurnPriorityMemoryLineageByText(lineage map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, raw := range prepareTurnMemoryLineageSlice(lineage["items"]) {
		item := mapFromAny(raw)
		text := prepareTurnPriorityCleanLine(extractionStringFromAny(item["final_text"]))
		if text == "" || boolFromAny(item["protected_guard"]) {
			continue
		}
		out[text] = append(out[text], item)
	}
	return out
}

func prepareTurnPriorityCandidateMap(candidate prepareTurnPriorityMemoryCandidate, exposeText bool) map[string]any {
	out := map[string]any{
		"canonical_fact_id": candidate.CanonicalFactID,
		"source_refs":       []string{candidate.SourceRef},
		"source_table":      candidate.SourceTable,
		"source_row_id":     candidate.SourceRowID,
		"source_turn":       candidate.SourceTurn,
		"lane":              candidate.Lane,
		"relevance_score":   candidate.Relevance,
		"importance_score":  candidate.Importance,
		"recency_score":     candidate.Recency,
		"continuity_bonus":  candidate.ContinuityBonus,
		"final_score":       candidate.FinalScore,
		"final_rank":        candidate.FinalRank,
		"chars":             candidate.Chars,
		"selection_status":  candidate.SelectionStatus,
		"selection_reason":  candidate.SelectionReason,
	}
	if exposeText {
		out["complete_text"] = candidate.CompleteText
	}
	if candidate.SourceOccurrence != "" {
		out["source_occurrence_key"] = candidate.SourceOccurrence
	}
	if candidate.SupersededBy != "" {
		out["superseded_by"] = candidate.SupersededBy
	}
	return out
}

func prepareTurnPrioritySourceRef(sourceTable string, sourceRowID any, line string) string {
	if strings.TrimSpace(fmt.Sprint(sourceRowID)) != "" && fmt.Sprint(sourceRowID) != "<nil>" {
		return fmt.Sprintf("%s:%v", sourceTable, sourceRowID)
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(line)))
	return fmt.Sprintf("%s:line:%x", sourceTable, digest[:8])
}

func prepareTurnPriorityFactID(key string) string {
	digest := sha256.Sum256([]byte("priority-memory-fact.v1\x1f" + key))
	return fmt.Sprintf("pmf_%x", digest[:16])
}

func prepareTurnPriorityRounded(value float64) float64 {
	return math.Round(value*1000000) / 1000000
}

func prepareTurnBuildPriorityCandidates(out *prepareTurnInjectionAssembly, query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityMemoryCandidate) {
	lineageByText := prepareTurnPriorityMemoryLineageByText(out.MemoryDeliveryLineage)
	metadataByText := prepareTurnPrioritySourceMetadataByText(out)
	candidates := []prepareTurnPriorityMemoryCandidate{}
	for _, input := range prepareTurnPriorityMemoryInputs(out) {
		for _, line := range prepareTurnDeliveryItems(input.Text) {
			if (input.SourceTable == "persona_memory_entries" || input.SourceTable == "protagonist_entity_memories") && !strings.HasPrefix(strings.TrimSpace(line), "-") {
				continue
			}
			var lineage map[string]any
			lineageKey := prepareTurnPriorityCleanLine(line)
			if input.SourceTable == "memories" && len(lineageByText[lineageKey]) > 0 {
				lineage = lineageByText[lineageKey][0]
				lineageByText[lineageKey] = lineageByText[lineageKey][1:]
			}
			var sourceMetadata *prepareTurnPrioritySourceMetadata
			metadataKey := input.SourceTable + "\x1f" + lineageKey
			if values := metadataByText[metadataKey]; len(values) > 0 {
				metadata := values[0]
				sourceMetadata = &metadata
				metadataByText[metadataKey] = values[1:]
			}
			for _, fact := range prepareTurnPrioritySplitFact(line) {
				if strings.TrimSpace(fact.Text) == "" {
					continue
				}
				var sourceRowID any
				var sourceOccurrence string
				var sourceTurn int
				if lineage != nil {
					sourceRowID = lineage["source_row_id"]
					sourceOccurrence = strings.TrimSpace(extractionStringFromAny(lineage["source_occurrence_key"]))
					sourceTurn = intFromAny(lineage["turn_index"], 0)
				}
				if sourceMetadata != nil {
					sourceRowID = sourceMetadata.SourceRowID
					sourceOccurrence = sourceMetadata.SourceOccurrence
					sourceTurn = sourceMetadata.SourceTurn
				}
				if sourceTurn == 0 {
					sourceTurn = prepareTurnPrioritySourceTurn(line)
				}
				importance := 0.5
				if lineage != nil {
					importance = prepareTurnPriorityNormalizeScore(extractionFloatFromAny(lineage["importance_score"], 0.5))
				}
				if sourceMetadata != nil && sourceMetadata.ImportancePresent {
					importance = sourceMetadata.Importance
				}
				relevance := prepareTurnPriorityRelevance(query, fact.Text)
				if lineageScore := extractionFloatFromAny(lineage["selection_score"], 0); lineageScore > relevance {
					relevance = prepareTurnPriorityNormalizeScore(lineageScore)
				}
				identity, entityIdentityObserved := prepareTurnPriorityCanonicalEntityIdentity(fact, out.PriorityEntityAliases)
				if identity == "" {
					identity = collapseTextKey(fact.Text)
				}
				if sourceOccurrence != "" {
					// A stored occurrence is the stable event identity for a natural-language
					// memory sentence. Structured rows retain their field path because one
					// occurrence may legitimately carry several independent state facts.
					if entityIdentityObserved && input.SourceTable == "character_states" {
						// Reviewed entity aliases share one current-state family while
						// the rendered wording remains untouched.
					} else if prepareTurnPriorityUsesCanonicalFieldIdentity(input.SourceTable, fact) {
						// Current-state projections resolve by canonical field path across
						// row revisions. Source occurrence remains in provenance only.
					} else if fact.Structured {
						identity = sourceOccurrence + "\x1f" + identity
					} else {
						identity = sourceOccurrence
					}
				}
				sourceRef := prepareTurnPrioritySourceRef(input.SourceTable, sourceRowID, line)
				candidates = append(candidates, prepareTurnPriorityMemoryCandidate{
					CanonicalKey: identity, SourceTable: input.SourceTable, SourceRef: sourceRef,
					SourceRowID: sourceRowID, SourceOccurrence: sourceOccurrence,
					Lane: input.Lane, Tier: input.Tier, CompleteText: fact.Text,
					SourceTurn: sourceTurn, Relevance: relevance, Importance: importance,
					ContinuityBonus: prepareTurnPriorityContinuityBonus(input.Lane, input.SourceTable, fact.Text),
					Chars:           len([]rune(fact.Text)),
				})
			}
		}
	}
	maxTurn := 0
	minTurn := 0
	for _, candidate := range candidates {
		if candidate.SourceTurn <= 0 {
			continue
		}
		if minTurn == 0 || candidate.SourceTurn < minTurn {
			minTurn = candidate.SourceTurn
		}
		if candidate.SourceTurn > maxTurn {
			maxTurn = candidate.SourceTurn
		}
	}
	for index := range candidates {
		candidate := &candidates[index]
		candidate.Recency = 0.5
		if candidate.SourceTurn > 0 {
			candidate.Recency = 1
			if maxTurn > minTurn {
				candidate.Recency = float64(candidate.SourceTurn-minTurn) / float64(maxTurn-minTurn)
			}
		}
		candidate.FinalScore = prepareTurnPriorityRounded(candidate.Relevance*0.60 + candidate.Importance*0.25 + candidate.Recency*0.15 + candidate.ContinuityBonus)
		candidate.CanonicalFactID = prepareTurnPriorityFactID(candidate.CanonicalKey)
	}

	// Request-scoped resolution replaces only candidates with an observed shared
	// identity. Distinct source occurrences remain distinct even when their text
	// happens to match.
	grouped := map[string][]int{}
	for index, candidate := range candidates {
		grouped[candidate.CanonicalKey] = append(grouped[candidate.CanonicalKey], index)
	}
	resolved := []prepareTurnPriorityMemoryCandidate{}
	superseded := []prepareTurnPriorityMemoryCandidate{}
	for _, indexes := range grouped {
		best := indexes[0]
		for _, index := range indexes[1:] {
			left := candidates[index]
			right := candidates[best]
			leftLifecycle := prepareTurnPriorityLifecycleRank(left.CompleteText)
			rightLifecycle := prepareTurnPriorityLifecycleRank(right.CompleteText)
			if leftLifecycle > rightLifecycle ||
				(leftLifecycle == rightLifecycle && left.SourceTurn > right.SourceTurn) ||
				(leftLifecycle == rightLifecycle && left.SourceTurn == right.SourceTurn && left.FinalScore > right.FinalScore) {
				best = index
			}
		}
		winner := candidates[best]
		resolved = append(resolved, winner)
		for _, index := range indexes {
			if index == best {
				continue
			}
			loser := candidates[index]
			loser.SelectionStatus = "deferred"
			loser.SelectionReason = "canonical_current_resolution"
			loser.SupersededBy = winner.CanonicalFactID
			superseded = append(superseded, loser)
		}
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		left, right := resolved[i], resolved[j]
		if left.FinalScore != right.FinalScore {
			return left.FinalScore > right.FinalScore
		}
		if left.Relevance != right.Relevance {
			return left.Relevance > right.Relevance
		}
		if left.Importance != right.Importance {
			return left.Importance > right.Importance
		}
		if left.SourceTurn != right.SourceTurn {
			return left.SourceTurn > right.SourceTurn
		}
		return left.CanonicalFactID < right.CanonicalFactID
	})
	for index := range resolved {
		resolved[index].FinalRank = index + 1
	}
	return resolved, superseded
}

func buildPrepareTurnPriorityMemoryDeliveryPlan(out *prepareTurnInjectionAssembly, maxChars, maxItems int, perspective map[string]any) map[string]any {
	deliveryCap := maxInt(maxChars, 0)
	if maxItems < 1 {
		maxItems = 1
	}
	query := strings.TrimSpace(extractionStringFromAny(perspective["_priority_memory_query"]))
	resolved, superseded := prepareTurnBuildPriorityCandidates(out, query)
	selected := map[string][]string{}
	usedGlobal := 0
	authoritySelected := 0
	authorityExactKeys := map[string]bool{}
	authorityLaneKeys := map[string]bool{}
	appendAuthority := func(lane string, items []string, preserveDistinctSourceOccurrences bool) {
		for _, item := range items {
			exactKey := collapseTextKey(prepareTurnPriorityCleanLine(item))
			laneKey := lane + "\x1f" + exactKey
			if exactKey == "" || (!preserveDistinctSourceOccurrences && authorityLaneKeys[laneKey]) {
				continue
			}
			candidate := append(append([]string{}, selected[lane]...), item)
			newText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", candidate)
			oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", selected[lane])
			delta := len([]rune(newText)) - len([]rune(oldText))
			if usedGlobal+delta > deliveryCap {
				continue
			}
			selected[lane] = candidate
			usedGlobal += delta
			authoritySelected++
			authorityLaneKeys[laneKey] = true
			authorityExactKeys[exactKey] = true
		}
	}
	appendAuthority("direct_evidence", prepareTurnDeliveryItems(out.LatestDirectEvidenceText, out.DirectEvidenceText, out.ScopedVerbatimText, out.ContinuityCorrectionText), false)
	// Protected rows carry distinct source occurrences even when their redacted
	// wording is identical. Preserve those occurrences; they are authority
	// material outside the optional-memory K competition.
	appendAuthority("protected_secret", prepareTurnDeliveryItems(out.ProtectedMemoryText), true)

	prioritySelected := 0
	priorityRank := 0
	guidanceByLane := map[string][]string{}
	guidanceSourceApplied := map[string]bool{}
	guidanceForCandidate := func(candidate *prepareTurnPriorityMemoryCandidate) []string {
		if candidate == nil || guidanceSourceApplied[candidate.SourceTable] {
			return nil
		}
		var sourceText string
		switch candidate.SourceTable {
		case "persona_memory_entries":
			sourceText = out.PersonaText
		case "protagonist_entity_memories":
			sourceText = out.CharacterPrivateText
		default:
			return nil
		}
		lines := []string{}
		for _, item := range prepareTurnDeliveryItems(sourceText) {
			if !strings.HasPrefix(strings.TrimSpace(item), "-") {
				lines = append(lines, item)
			}
		}
		return lines
	}
	renderLaneItems := func(lane string, items []string, extraGuidance []string) []string {
		combined := append([]string{}, guidanceByLane[lane]...)
		combined = append(combined, extraGuidance...)
		return append(combined, items...)
	}
	for index := range resolved {
		candidate := &resolved[index]
		if authorityExactKeys[collapseTextKey(candidate.CompleteText)] {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = "authority_exact_duplicate"
			continue
		}
		priorityRank++
		candidate.FinalRank = priorityRank
		if priorityRank > maxItems {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = "priority_memory_max_items"
			continue
		}
		line := "- " + candidate.CompleteText
		laneItems := append(append([]string{}, selected[candidate.Lane]...), line)
		extraGuidance := guidanceForCandidate(candidate)
		newText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[candidate.Lane]+"]", renderLaneItems(candidate.Lane, laneItems, extraGuidance))
		oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[candidate.Lane]+"]", renderLaneItems(candidate.Lane, selected[candidate.Lane], nil))
		delta := len([]rune(newText)) - len([]rune(oldText))
		if usedGlobal+delta > deliveryCap {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = "memory_char_budget_reached"
			continue
		}
		if len(extraGuidance) > 0 {
			guidanceByLane[candidate.Lane] = append(guidanceByLane[candidate.Lane], extraGuidance...)
			guidanceSourceApplied[candidate.SourceTable] = true
		}
		selected[candidate.Lane] = laneItems
		usedGlobal += delta
		prioritySelected++
		candidate.SelectionStatus = "selected"
		candidate.SelectionReason = "global_priority_rank"
	}

	classes := []map[string]any{}
	parts := []string{}
	classEligible := map[string]int{}
	classDeferred := map[string]int{}
	for _, candidate := range resolved {
		classEligible[candidate.Lane]++
		if candidate.SelectionStatus != "selected" {
			classDeferred[candidate.Lane]++
		}
	}
	for _, candidate := range superseded {
		classEligible[candidate.Lane]++
		classDeferred[candidate.Lane]++
	}
	for _, lane := range prepareTurnMemoryDeliveryOrder {
		text := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", renderLaneItems(lane, selected[lane], nil))
		if text != "" {
			parts = append(parts, text)
		}
		classes = append(classes, map[string]any{
			"key": lane, "title": prepareTurnMemoryDeliveryTitles[lane],
			"used_chars": len([]rune(text)), "eligible_count": classEligible[lane],
			"selected_count": len(selected[lane]), "deferred_count": classDeferred[lane],
			"selection_policy": map[bool]string{true: "authority_exempt", false: "global_priority_score"}[lane == "direct_evidence" || lane == "protected_secret"],
			"text":             nilIfEmpty(text),
		})
	}
	finalText := strings.Join(parts, "\n\n")
	finalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalText)))
	priorityItems := make([]map[string]any, 0, len(resolved)+len(superseded))
	selectedFactIDs := []string{}
	exclusionReasons := map[string]int{}
	for _, candidate := range resolved {
		exposeText := candidate.SourceTable != "protagonist_entity_memories" || candidate.SelectionStatus == "selected"
		priorityItems = append(priorityItems, prepareTurnPriorityCandidateMap(candidate, exposeText))
		if candidate.SelectionStatus == "selected" {
			selectedFactIDs = append(selectedFactIDs, candidate.CanonicalFactID)
		} else if candidate.SelectionReason != "" {
			exclusionReasons[candidate.SelectionReason]++
		}
	}
	for _, candidate := range superseded {
		exposeText := candidate.SourceTable != "protagonist_entity_memories"
		priorityItems = append(priorityItems, prepareTurnPriorityCandidateMap(candidate, exposeText))
		exclusionReasons[candidate.SelectionReason]++
	}
	return map[string]any{
		"contract_version": prepareTurnPriorityMemoryPlanVersion,
		"status":           "ready", "mode": "auto", "owner": "go",
		"score_version":      prepareTurnPriorityMemoryScoreVersion,
		"score_formula":      "relevance*0.60+importance*0.25+recency*0.15+continuity_bonus",
		"final_budget_owner": "go_priority_memory_delivery_plan",
		"global_cap_chars":   maxChars, "delivery_cap_chars": deliveryCap,
		"priority_memory_max_items":       maxItems,
		"priority_candidate_count":        len(resolved) + len(superseded),
		"priority_resolved_count":         len(resolved),
		"priority_selected_count":         prioritySelected,
		"authority_exempt_selected_count": authoritySelected,
		"candidate_count":                 len(resolved) + len(superseded),
		"selected_count":                  prioritySelected + authoritySelected,
		"selected_chars":                  len([]rune(finalText)),
		"final_delivery_count":            prioritySelected + authoritySelected,
		"final_delivery_chars":            len([]rune(finalText)),
		"excluded_count":                  len(resolved) + len(superseded) - prioritySelected,
		"exclusion_reasons":               exclusionReasons,
		"used_chars":                      len([]rune(finalText)), "order": prepareTurnMemoryDeliveryOrder,
		"selection_order":                   "global_final_score_descending_before_lane_render",
		"low_score_backfill_after_k":        false,
		"source_rows_mutated":               false,
		"request_scoped_current_resolution": true,
		"priority_items":                    priorityItems,
		"selected_fact_ids":                 selectedFactIDs,
		"final_text_sha256":                 finalHash,
		"rendering_hash":                    finalHash,
		"classes":                           classes,
		"final_text":                        nilIfEmpty(finalText),
		"core_objective_memory": map[string]any{
			"contract_version": "core_priority_memory_delivery.v2",
			"status":           "active", "requested_max_items": maxItems,
			"eligible_distinct_count": len(resolved), "delivered_count": prioritySelected,
			"deferred_by_limit_count":  exclusionReasons["priority_memory_max_items"],
			"deferred_by_budget_count": exclusionReasons["memory_char_budget_reached"],
			"missing_to_limit":         maxInt(maxItems-prioritySelected, 0),
			"garbage_fill":             false, "top_k_reinterpreted": false,
			"counted_lane":            "all_scored_memory_facts",
			"item_count_exempt_lanes": []string{"direct_evidence", "protected_secret"},
		},
		"historical_chat_authority_policy": "previous_logical_turn_owned_by_input_context_not_direct_evidence",
		"recent_raw_turn_delivery":         "excluded_from_final_memory_delivery",
		"raw_chat_fallback_delivery":       "diagnostic_only_excluded_from_final_memory_delivery",
	}
}
