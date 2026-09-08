package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	prepareTurnPriorityMemoryPlanVersion       = "memory_delivery_plan.v2"
	prepareTurnPriorityMemoryScoreVersion      = "priority_score.static.v4"
	prepareTurnFinalizationImmediate           = "immediate_after_response"
	prepareTurnFinalizationNextInput           = "next_user_input"
	prepareTurnPrioritySemanticFactsContextKey = "_priority_precise_memory_vector_facts"
	prepareTurnPriorityQuerySetContextKey      = "_priority_memory_query_set"
	prepareTurnPriorityRecencyHalfLifeTurns    = 32.0
	prepareTurnPriorityRecencyFloor            = 0.20
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
	Lane              string
	SourceTable       string
	Tier              string
	LineKey           string
	SourceRowID       any
	SourceOccurrence  string
	SourceTurn        int
	Importance        float64
	ImportancePresent bool
	Visibility        string
	PerspectiveOwner  string
	AllowedViewers    []string
}

type prepareTurnPriorityMemoryFact struct {
	Text                string
	FamilyKey           string
	ValueKey            string
	LifecycleKey        string
	LifecycleTransition string
	EntitySurface       string
	SpeakerSurface      string
	LocationSurface     string
	StorylineSurface    string
	MemoryRole          string
	MetadataKey         string
	Structured          bool
	ArrayOrdinalPath    bool
	Visibility          string
	PerspectiveOwner    string
	AllowedViewers      []string
}

type prepareTurnPrioritySemanticFact struct {
	UnitID           string
	ChatSessionID    string
	SourceRevision   string
	SourceTurn       int
	Lane             string
	Fact             prepareTurnPriorityMemoryFact
	Similarity       float64
	SimilaritySource string
	Visibility       string
}

type prepareTurnPriorityIdentityMetadata struct {
	Text            string
	EntityKey       string
	SourceTable     string
	SourceRef       string
	SelectionStatus string
	SelectionReason string
	AttachedFactID  string
}

// prepareTurnPriorityFactSeed is the request-local, pre-section-render unit
// consumed by the 4.2 scorer. It does not create or mutate a persistent row.
// Visibility and perspective are copied from the source projection that
// already admitted the item; they are lineage, not another eligibility gate.
type prepareTurnPriorityFactSeed struct {
	Lane                         string
	SourceTable                  string
	Tier                         string
	Fact                         prepareTurnPriorityMemoryFact
	SourceRowID                  any
	SourceOccurrence             string
	SourceTurn                   int
	Importance                   float64
	ImportancePresent            bool
	Visibility                   string
	PerspectiveOwner             string
	AllowedViewers               []string
	ProjectionSource             string
	ParentLineKey                string
	SourceFactCount              int
	SourceSelectionScore         float64
	SourceSelectionScoreObserved bool
	SourceSelectionScoreIsVector bool
	SemanticSimilarity           float64
	SemanticSimilarityObserved   bool
	SemanticUnitID               string
	SemanticSimilaritySource     string
}

type prepareTurnPriorityMemoryCandidate struct {
	CanonicalFactID              string
	CanonicalKey                 string
	SourceTable                  string
	SourceRef                    string
	SourceRowID                  any
	SourceOccurrence             string
	Lane                         string
	Tier                         string
	CompleteText                 string
	SourceTurn                   int
	Relevance                    float64
	LexicalRelevance             float64
	Importance                   float64
	Recency                      float64
	ContinuityBonus              float64
	SpeakerBias                  float64
	LocationBias                 float64
	StorylineBias                float64
	StructuredBias               float64
	FinalScore                   float64
	FinalRank                    int
	Chars                        int
	SelectionStatus              string
	SelectionReason              string
	SupersededBy                 string
	Visibility                   string
	PerspectiveOwner             string
	AllowedViewers               []string
	ProjectionSource             string
	ParentLineText               string
	SourceFactCount              int
	RelevanceSource              string
	SemanticUnitID               string
	SemanticSimilaritySource     string
	ImportanceSource             string
	SourceSelectionScore         float64
	SourceSelectionScoreObserved bool
	SourceSelectionScoreIsVector bool
	EntityKey                    string
	FactFamilyKey                string
	FactValueKey                 string
	LifecycleKey                 string
	LifecycleTransition          string
	SpeakerSurface               string
	LocationSurface              string
	StorylineSurface             string
	IdentityMetadata             []string
	RenderedText                 string
}

type prepareTurnPriorityTurnSummaryCandidate struct {
	SummaryID                      string
	SourceRef                      string
	SourceRowID                    any
	SourceOccurrence               string
	SourceTurn                     int
	CompleteText                   string
	RenderedText                   string
	FinalScore                     float64
	FinalRank                      int
	Chars                          int
	RepresentativeFactID           string
	MemberFactIDs                  []string
	SelectionStatus                string
	SelectionReason                string
	ScoreSource                    string
	SourceVectorSimilarity         float64
	SourceVectorSimilarityObserved bool
}

func clonePrepareTurnPriorityCandidatePool(facts []prepareTurnPriorityMemoryCandidate, summaries []prepareTurnPriorityTurnSummaryCandidate) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate) {
	facts = slices.Clone(facts)
	for index := range facts {
		facts[index].AllowedViewers = slices.Clone(facts[index].AllowedViewers)
		facts[index].IdentityMetadata = slices.Clone(facts[index].IdentityMetadata)
	}
	summaries = slices.Clone(summaries)
	for index := range summaries {
		summaries[index].MemberFactIDs = slices.Clone(summaries[index].MemberFactIDs)
	}
	return facts, summaries
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

func appendPrepareTurnPrioritySourceMetadata(out *prepareTurnInjectionAssembly, lane, sourceTable, tier, line, sourceOccurrence string, sourceRowID any, sourceTurn int, importance float64, importancePresent bool, visibility, perspectiveOwner string, allowedViewers []string) {
	if out == nil || strings.TrimSpace(line) == "" {
		return
	}
	metadata := prepareTurnPrioritySourceMetadata{
		Lane:              strings.TrimSpace(lane),
		SourceTable:       strings.TrimSpace(sourceTable),
		Tier:              strings.TrimSpace(tier),
		LineKey:           prepareTurnPriorityCleanLine(line),
		SourceRowID:       sourceRowID,
		SourceOccurrence:  strings.TrimSpace(sourceOccurrence),
		SourceTurn:        sourceTurn,
		Importance:        prepareTurnPriorityNormalizeScore(importance),
		ImportancePresent: importancePresent,
		Visibility:        strings.TrimSpace(visibility),
		PerspectiveOwner:  strings.TrimSpace(perspectiveOwner),
		AllowedViewers:    append([]string(nil), allowedViewers...),
	}
	out.PrioritySourceMetadata = append(out.PrioritySourceMetadata, metadata)
	appendPrepareTurnPriorityFactSeeds(out, metadata, line, prepareTurnPrioritySplitFact(line), "source_projection")
}

func appendPrepareTurnPriorityFactSeeds(out *prepareTurnInjectionAssembly, metadata prepareTurnPrioritySourceMetadata, parentLine string, facts []prepareTurnPriorityMemoryFact, projectionSource string) {
	if out == nil || len(facts) == 0 {
		return
	}
	parentLineKey := prepareTurnPriorityCleanLine(parentLine)
	for _, fact := range facts {
		if strings.TrimSpace(fact.Text) == "" {
			continue
		}
		factProjection := strings.TrimSpace(projectionSource)
		if fact.Structured {
			factProjection += ":structured"
		} else {
			factProjection += ":sentence"
		}
		visibility := metadata.Visibility
		if strings.TrimSpace(fact.Visibility) != "" {
			visibility = fact.Visibility
		}
		perspectiveOwner := metadata.PerspectiveOwner
		if strings.TrimSpace(fact.PerspectiveOwner) != "" {
			perspectiveOwner = fact.PerspectiveOwner
		}
		allowedViewers := append([]string(nil), metadata.AllowedViewers...)
		if len(fact.AllowedViewers) > 0 {
			allowedViewers = append([]string(nil), fact.AllowedViewers...)
		}
		out.PriorityFactSeeds = append(out.PriorityFactSeeds, prepareTurnPriorityFactSeed{
			Lane:              metadata.Lane,
			SourceTable:       metadata.SourceTable,
			Tier:              metadata.Tier,
			Fact:              fact,
			SourceRowID:       metadata.SourceRowID,
			SourceOccurrence:  metadata.SourceOccurrence,
			SourceTurn:        metadata.SourceTurn,
			Importance:        metadata.Importance,
			ImportancePresent: metadata.ImportancePresent,
			Visibility:        visibility,
			PerspectiveOwner:  perspectiveOwner,
			AllowedViewers:    allowedViewers,
			ProjectionSource:  factProjection,
			ParentLineKey:     parentLineKey,
			SourceFactCount:   len(facts),
		})
	}
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

func prepareTurnPriorityNaturalSentenceParts(text string) []string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return nil
	}
	runes := []rune(text)
	parts := []string{}
	start := 0
	depth := 0
	inDoubleQuote := false
	appendPart := func(end int) {
		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			parts = append(parts, part)
		}
	}
	for index, r := range runes {
		switch r {
		case '"', '“', '”':
			inDoubleQuote = !inDoubleQuote
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 || inDoubleQuote {
			continue
		}
		if r == '\n' {
			appendPart(index)
			start = index + 1
			continue
		}
		if r != '.' && r != '?' && r != '!' && r != '。' && r != '？' && r != '！' {
			continue
		}
		if r == '.' && index > 0 && index+1 < len(runes) && unicode.IsDigit(runes[index-1]) && unicode.IsDigit(runes[index+1]) {
			continue
		}
		if index+1 < len(runes) && !unicode.IsSpace(runes[index+1]) {
			continue
		}
		appendPart(index + 1)
		start = index + 1
	}
	if start < len(runes) {
		appendPart(len(runes))
	}
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}

func prepareTurnPriorityTrailingPipeMetadata(text string) (string, string) {
	parts := strings.Split(text, " | ")
	if len(parts) < 2 {
		return strings.TrimSpace(text), ""
	}
	metadataStart := len(parts)
	for metadataStart > 1 {
		part := strings.TrimSpace(parts[metadataStart-1])
		equals := strings.Index(part, "=")
		if equals <= 0 {
			break
		}
		key := strings.TrimSpace(part[:equals])
		if key == "" || strings.ContainsAny(key, " .?!。？！") {
			break
		}
		metadataStart--
	}
	if metadataStart == len(parts) {
		return strings.TrimSpace(text), ""
	}
	return strings.TrimSpace(strings.Join(parts[:metadataStart], " | ")),
		strings.TrimSpace(strings.Join(parts[metadataStart:], " | "))
}

func prepareTurnPriorityNaturalFacts(prefix, text string) []prepareTurnPriorityMemoryFact {
	topLevelParts := prepareTurnPriorityTopLevelParts(text, ';')
	leadingMetadata := []string{}
	for len(topLevelParts) > 1 {
		part := strings.TrimSpace(topLevelParts[0])
		equals := strings.Index(part, "=")
		if equals <= 0 || strings.ContainsAny(strings.TrimSpace(part[:equals]), " .?!。？！") {
			break
		}
		leadingMetadata = append(leadingMetadata, part)
		topLevelParts = topLevelParts[1:]
	}
	parts := []string{}
	for _, topLevel := range topLevelParts {
		body, trailingMetadata := prepareTurnPriorityTrailingPipeMetadata(topLevel)
		for _, part := range prepareTurnPriorityNaturalSentenceParts(body) {
			if trailingMetadata != "" {
				part = strings.TrimSpace(part) + " | " + trailingMetadata
			}
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	facts := make([]prepareTurnPriorityMemoryFact, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		complete := part
		if len(leadingMetadata) > 0 {
			complete = strings.Join(leadingMetadata, "; ") + "; " + complete
		}
		if strings.TrimSpace(prefix) != "" {
			complete = strings.TrimSpace(prefix) + ": " + complete
		}
		key := collapseTextKey(complete)
		facts = append(facts, prepareTurnPriorityMemoryFact{
			Text: complete, FamilyKey: key, ValueKey: collapseTextKey(part), EntitySurface: strings.TrimSpace(prefix),
		})
	}
	return facts
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

func prepareTurnPriorityIdentityMetadataField(path []string) string {
	for index := len(path) - 1; index >= 0; index-- {
		field := strings.ToLower(strings.TrimSpace(path[index]))
		if strings.HasPrefix(field, "item_") {
			continue
		}
		switch field {
		case "name", "character_name", "display_name", "canonical_name", "aliases", "identity_evidence", "identity_evidence_excerpt":
			return field
		default:
			return ""
		}
	}
	return ""
}

func prepareTurnPriorityPathHasCharacterIdentity(path []string) bool {
	for _, raw := range path {
		field := strings.ToLower(strings.TrimSpace(raw))
		switch field {
		case "character", "characters", "character_profile", "character_profiles":
			return true
		}
	}
	return false
}

func prepareTurnPriorityPathHasArrayOrdinal(path []string) bool {
	for _, raw := range path {
		field := strings.TrimSpace(raw)
		if !strings.HasPrefix(field, "item_") {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimPrefix(field, "item_")); err == nil {
			return true
		}
	}
	return false
}

func prepareTurnPriorityNestedEntitySurface(current string, value map[string]any) string {
	for _, key := range []string{"name", "character_name", "display_name", "canonical_name", "subject", "subject_entity", "actor", "actor_name"} {
		if surface := strings.TrimSpace(stringFromMap(value, key)); surface != "" {
			return surface
		}
	}
	return strings.TrimSpace(current)
}

func prepareTurnPriorityFlattenValue(prefix string, path []string, value any, facts *[]prepareTurnPriorityMemoryFact) {
	prepareTurnPriorityFlattenValueWithEntity(prefix, path, "", value, facts)
}

func prepareTurnPriorityFlattenValueWithEntity(prefix string, path []string, entitySurface string, value any, facts *[]prepareTurnPriorityMemoryFact) {
	switch typed := value.(type) {
	case map[string]any:
		entitySurface = prepareTurnPriorityNestedEntitySurface(entitySurface, typed)
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			prepareTurnPriorityFlattenValueWithEntity(prefix, append(append([]string{}, path...), key), entitySurface, typed[key], facts)
		}
	case []any:
		for index, item := range typed {
			prepareTurnPriorityFlattenValueWithEntity(prefix, append(append([]string{}, path...), fmt.Sprintf("item_%d", index+1)), entitySurface, item, facts)
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
		memoryRole := "fact"
		metadataKey := ""
		if prepareTurnPriorityPathHasCharacterIdentity(path) {
			metadataKey = prepareTurnPriorityIdentityMetadataField(path)
			if metadataKey != "" {
				memoryRole = "identity_metadata"
				text = metadataKey + "=" + valueText
			}
		}
		*facts = append(*facts, prepareTurnPriorityMemoryFact{
			Text: strings.TrimSpace(text), FamilyKey: family,
			ValueKey: collapseTextKey(valueText), EntitySurface: strings.TrimSpace(entitySurface),
			SpeakerSurface: strings.TrimSpace(entitySurface), MemoryRole: memoryRole, MetadataKey: metadataKey, Structured: true,
			ArrayOrdinalPath: prepareTurnPriorityPathHasArrayOrdinal(path),
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
		if strings.TrimSpace(facts[index].EntitySurface) == "" {
			facts[index].EntitySurface = strings.TrimSpace(surface)
		}
		if strings.TrimSpace(facts[index].SpeakerSurface) == "" {
			facts[index].SpeakerSurface = facts[index].EntitySurface
		}
	}
	return facts
}

func prepareTurnPrioritySplitFact(line string) []prepareTurnPriorityMemoryFact {
	clean := prepareTurnPriorityCleanLine(line)
	if clean == "" {
		return nil
	}
	// A protected recollection's guard describes how the preceding memory may be
	// used. It is not a set of independent memory facts and must travel with that
	// memory instead of consuming separate K slots.
	if strings.Contains(strings.ToLower(clean), " | protected private knowledge is present;") {
		key := collapseTextKey(clean)
		entitySurface := ""
		if colon := strings.Index(clean, ":"); colon > 0 {
			entitySurface = prepareTurnPriorityIdentityPrefix(clean[:colon])
		}
		return []prepareTurnPriorityMemoryFact{{
			Text: clean, FamilyKey: key, ValueKey: key, EntitySurface: entitySurface,
		}}
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
		if facts := prepareTurnPriorityNaturalFacts(prefix, raw); len(facts) > 1 {
			return facts
		}
		key := collapseTextKey(clean)
		return []prepareTurnPriorityMemoryFact{{Text: clean, FamilyKey: key, ValueKey: key, EntitySurface: prefix}}
	}
	if facts := prepareTurnPriorityNaturalFacts("", clean); len(facts) > 0 {
		return facts
	}
	return nil
}

func prepareTurnPriorityStructuredMemoryItemFacts(sourceKey string, raw any) []prepareTurnPriorityMemoryFact {
	item := mapFromAny(raw)
	if len(item) == 0 {
		text := strings.TrimSpace(extractionStringFromAny(raw))
		return prepareTurnPriorityNaturalFacts("", text)
	}
	subject := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "subject_entity"), stringFromMap(item, "subject"),
		stringFromMap(item, "actor"), stringFromMap(item, "character_name"),
		stringFromMap(item, "character"), stringFromMap(item, "entity"),
		stringFromMap(item, "name"), stringFromMap(item, "subject_name"),
		stringFromMap(item, "source_entity"), stringFromMap(item, "owner_entity_name"),
		stringFromMap(item, "owner_entity_key"),
	))
	target := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "target_entity"), stringFromMap(item, "target"),
		stringFromMap(item, "counterpart"),
	))
	speaker := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker"),
		stringFromMap(item, "actor_name"), stringFromMap(item, "actor"), subject,
	))
	location := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "location"), stringFromMap(item, "location_name"),
		stringFromMap(item, "place"), stringFromMap(item, "scene_location"),
		stringFromMap(item, "current_location"),
	))
	storyline := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "storyline"), stringFromMap(item, "storyline_name"),
		stringFromMap(item, "arc"), stringFromMap(item, "arc_name"),
		stringFromMap(item, "plotline"), stringFromMap(item, "objective"),
		stringFromMap(item, "goal"),
	))
	detail := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "summary"), stringFromMap(item, "description"),
		stringFromMap(item, "change"), stringFromMap(item, "observation"),
		stringFromMap(item, "event"), stringFromMap(item, "action"),
		stringFromMap(item, "supported_expression"), stringFromMap(item, "utterance_expression"),
		stringFromMap(item, "state_expression"), stringFromMap(item, "condition"),
		stringFromMap(item, "condition_label"), stringFromMap(item, "title"),
		stringFromMap(item, "value_expression"),
		stringFromMap(item, "memory_text"), stringFromMap(item, "text"),
	))
	fieldKey := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "state_slot"), stringFromMap(item, "trait_key"),
		stringFromMap(item, "principle_key"), stringFromMap(item, "behavior_key"),
		stringFromMap(item, "profile_key"), stringFromMap(item, "thread_key"), stringFromMap(item, "key"),
	))
	value := strings.TrimSpace(extractionFirstNonEmpty(
		prepareTurnPriorityScalarText(item["value"]), prepareTurnPriorityScalarText(item["status"]),
		prepareTurnPriorityScalarText(item["state"]),
	))
	lifecycleKey := normalizeNarrativeLifecycleKey(stringFromMap(item, "lifecycle_key"))
	lifecycleTransition := normalizeNarrativeTransition(stringFromMap(item, "transition"))
	if detail == "" && sourceKey == "interaction_boundaries" {
		actionScope := strings.TrimSpace(stringFromMap(item, "action_scope"))
		decision := strings.TrimSpace(stringFromMap(item, "decision"))
		if actionScope != "" || decision != "" {
			detail = strings.TrimSpace(actionScope + ": " + decision)
		}
	}
	if detail == "" && (fieldKey != "" || value != "") {
		detail = strings.TrimSpace(fieldKey)
		if value != "" {
			if detail != "" {
				detail += ": "
			}
			detail += value
		}
	}
	if detail == "" {
		detail = strings.TrimSpace(stringFromMap(item, "evidence_excerpt"))
	}
	if detail == "" {
		return nil
	}
	visibility := strings.TrimSpace(stringFromMap(item, "visibility"))
	perspectiveOwner := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "perspective_owner"), stringFromMap(item, "owner_entity_name"),
		stringFromMap(item, "owner_entity_key"), stringFromMap(item, "owner"),
	))
	allowedViewers := stringsFromAny(item["allowed_viewers"])
	if len(allowedViewers) == 0 {
		allowedViewers = stringsFromAny(mapFromAny(item["knowledge_scope"])["known_by"])
	}
	prefix := subject
	if target != "" {
		if prefix != "" {
			prefix += " → "
		}
		prefix += target
	}
	parts := prepareTurnPriorityNaturalSentenceParts(detail)
	facts := make([]prepareTurnPriorityMemoryFact, 0, len(parts))
	for _, part := range parts {
		complete := strings.TrimSpace(part)
		if subject != "" && !prepareTurnRecallContainsAnchor(complete, subject) {
			complete = strings.TrimSpace(prefix) + ": " + complete
		}
		factIdentity := fieldKey
		if factIdentity == "" {
			factIdentity = strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "event_id"), stringFromMap(item, "occurrence_id"),
				stringFromMap(item, "state_claim_id"), stringFromMap(item, "source_occurrence_id"),
				stringFromMap(item, "id"),
			))
		}
		if factIdentity == "" || len(parts) > 1 {
			factIdentity = strings.TrimSpace(factIdentity + "\x1f" + collapseTextKey(part))
		}
		familyParts := []string{sourceKey, subject, target, factIdentity}
		family := collapseTextKey(strings.Join(familyParts, "\x1f"))
		if family == "" {
			family = collapseTextKey(complete)
		}
		facts = append(facts, prepareTurnPriorityMemoryFact{
			Text: complete, FamilyKey: family, ValueKey: collapseTextKey(part), LifecycleKey: lifecycleKey,
			LifecycleTransition: lifecycleTransition, EntitySurface: subject, Structured: true,
			SpeakerSurface: speaker, LocationSurface: location, StorylineSurface: storyline,
			Visibility: visibility, PerspectiveOwner: perspectiveOwner, AllowedViewers: append([]string(nil), allowedViewers...),
		})
	}
	return facts
}

func prepareTurnPriorityPreciseSourceAndLane(kind, subtype string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "state":
		return "state_claims", "world_state"
	case "boundary":
		return "interaction_boundaries", "subjective_relationship"
	case "observation":
		if strings.Contains(strings.ToLower(strings.TrimSpace(subtype)), "relationship") || strings.Contains(strings.ToLower(strings.TrimSpace(subtype)), "interaction") {
			return "relationship_observations", "subjective_relationship"
		}
		return "narrative_events", "event_recent"
	case "utterance":
		return "speaker_attributions", "event_recent"
	default:
		return "narrative_events", "event_recent"
	}
}

func prepareTurnPrioritySemanticFactFromPreciseUnit(unit store.PreciseMemoryUnit, similarity float64, similaritySource string) (prepareTurnPrioritySemanticFact, bool) {
	unitID := strings.TrimSpace(unit.UnitID)
	if unitID == "" {
		return prepareTurnPrioritySemanticFact{}, false
	}
	sourceKey, lane := prepareTurnPriorityPreciseSourceAndLane(unit.Kind, unit.Subtype)
	payload := parseJSONMap(unit.PayloadJSON)
	facts := prepareTurnPriorityStructuredMemoryItemFacts(sourceKey, payload)
	if len(facts) == 0 {
		semanticText := strings.TrimSpace(store.PreciseMemorySemanticText(&unit))
		facts = prepareTurnPriorityNaturalFacts("", semanticText)
	}
	if len(facts) == 0 {
		return prepareTurnPrioritySemanticFact{}, false
	}
	fact := facts[0]
	if len(facts) > 1 {
		parts := make([]string, 0, len(facts))
		for _, part := range facts {
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, strings.TrimSpace(part.Text))
			}
		}
		fact.Text = strings.Join(parts, " ")
		fact.FamilyKey = collapseTextKey(strings.Join([]string{sourceKey, unit.SourceRevision, unitID}, "\x1f"))
		fact.ValueKey = collapseTextKey(fact.Text)
	}
	fact.Visibility = strings.TrimSpace(unit.Visibility)
	return prepareTurnPrioritySemanticFact{
		UnitID: unitID, ChatSessionID: strings.TrimSpace(unit.ChatSessionID),
		SourceRevision: strings.TrimSpace(unit.SourceRevision), SourceTurn: maxInt(unit.SourceTurnStart, unit.SourceTurnEnd),
		Lane: lane, Fact: fact, Similarity: prepareTurnPriorityNormalizeScore(similarity),
		SimilaritySource: strings.TrimSpace(similaritySource), Visibility: strings.TrimSpace(unit.Visibility),
	}, true
}

func prepareTurnPrioritySemanticFactsFromAny(value any) []prepareTurnPrioritySemanticFact {
	switch typed := value.(type) {
	case []prepareTurnPrioritySemanticFact:
		return append([]prepareTurnPrioritySemanticFact(nil), typed...)
	case []*prepareTurnPrioritySemanticFact:
		out := make([]prepareTurnPrioritySemanticFact, 0, len(typed))
		for _, item := range typed {
			if item != nil {
				out = append(out, *item)
			}
		}
		return out
	default:
		return nil
	}
}

func prepareTurnPriorityFactsFromMemory(item store.Memory) ([]prepareTurnPriorityMemoryFact, string) {
	parsed := parseJSONMap(item.SummaryJSON)
	facts := []prepareTurnPriorityMemoryFact{}
	for _, key := range []string{
		"narrative_events", "character_deltas", "state_claims", "reversible_states",
		"physical_conditions", "entity_conditions", "pending_threads", "world_rules",
		"interaction_events", "relationship_observations", "interaction_boundaries",
		"habit_observations", "character_profile_observations", "voice_observations", "rp_character_profile",
	} {
		for _, raw := range sliceFromAny(parsed[key]) {
			facts = append(facts, prepareTurnPriorityStructuredMemoryItemFacts(key, raw)...)
		}
	}
	if len(facts) > 0 {
		return prepareTurnPriorityDistinctFacts(facts), "memory_public_projection"
	}
	summary := strings.TrimSpace(memorySummaryFromParsed(parsed))
	if summary == "" {
		summary = prepareTurnMemorySummary(item)
	}
	fallbackFacts := prepareTurnPriorityNaturalFacts("", summary)
	speakers := stringsFromAny(parsed["characters"])
	if len(speakers) == 0 {
		for _, rawEntity := range sliceFromAny(parsed["entities"]) {
			entity := mapFromAny(rawEntity)
			if name := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), extractionStringFromAny(rawEntity))); name != "" {
				speakers = append(speakers, name)
			}
		}
	}
	locations := stringsFromAny(parsed["locations"])
	storyline := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(parsed, "storyline"), stringFromMap(parsed, "arc"), stringFromMap(parsed, "plotline"),
	))
	for index := range fallbackFacts {
		fallbackFacts[index].SpeakerSurface = strings.Join(speakers, " ")
		fallbackFacts[index].LocationSurface = strings.Join(locations, " ")
		fallbackFacts[index].StorylineSurface = storyline
	}
	return prepareTurnPriorityDistinctFacts(fallbackFacts), "memory_summary"
}

func prepareTurnPriorityDistinctFacts(facts []prepareTurnPriorityMemoryFact) []prepareTurnPriorityMemoryFact {
	out := make([]prepareTurnPriorityMemoryFact, 0, len(facts))
	seen := map[string]bool{}
	for _, fact := range facts {
		key := collapseTextKey(fact.Text)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, fact)
	}
	return out
}

func appendPrepareTurnPriorityMemoryFactSeeds(out *prepareTurnInjectionAssembly, memories []store.Memory) {
	if out == nil {
		return
	}
	byRow := map[string]store.Memory{}
	for _, item := range memories {
		byRow[fmt.Sprint(prepareTurnMemorySourceRowID(item))] = item
	}
	for _, raw := range prepareTurnMemoryLineageSlice(out.MemoryDeliveryLineage["items"]) {
		lineage := mapFromAny(raw)
		if !boolFromAny(lineage["delivered"]) || boolFromAny(lineage["protected_guard"]) {
			continue
		}
		rowKey := fmt.Sprint(lineage["source_row_id"])
		item, ok := byRow[rowKey]
		if !ok {
			continue
		}
		facts, projectionSource := prepareTurnPriorityFactsFromMemory(item)
		if len(facts) == 0 {
			facts = prepareTurnPrioritySplitFact(extractionStringFromAny(lineage["final_text"]))
			projectionSource = "memory_rendered_line"
		}
		metadata := prepareTurnPrioritySourceMetadata{
			Lane: "event_recent", SourceTable: "memories", Tier: "required",
			LineKey:     prepareTurnPriorityCleanLine(extractionStringFromAny(lineage["final_text"])),
			SourceRowID: lineage["source_row_id"], SourceOccurrence: strings.TrimSpace(extractionStringFromAny(lineage["source_occurrence_key"])),
			SourceTurn: intFromAny(lineage["turn_index"], item.TurnIndex),
			Importance: prepareTurnPriorityNormalizeScore(item.Importance), ImportancePresent: item.Importance > 0,
			Visibility: "public_projection",
		}
		turnSummary := prepareTurnMemorySummary(item)
		if strings.TrimSpace(turnSummary) == "" {
			turnSummary = extractionStringFromAny(lineage["final_text"])
		}
		start := len(out.PriorityFactSeeds)
		appendPrepareTurnPriorityFactSeeds(out, metadata, turnSummary, facts, projectionSource)
		sourceScore := prepareTurnPriorityNormalizeScore(extractionFloatFromAny(lineage["selection_score"], 0))
		for index := start; index < len(out.PriorityFactSeeds); index++ {
			out.PriorityFactSeeds[index].SourceSelectionScore = sourceScore
			out.PriorityFactSeeds[index].SourceSelectionScoreObserved = sourceScore > 0
			out.PriorityFactSeeds[index].SourceSelectionScoreIsVector = boolFromAny(lineage["vector_hit"])
		}
	}
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
	if fact.LifecycleKey != "" {
		// Lifecycle is Critic-produced diagnostic metadata. It must not collapse
		// plan/progress/completion/follow-up facts into one request-local winner.
		return false
	}
	if !fact.Structured {
		return false
	}
	if fact.ArrayOrdinalPath {
		// Array positions describe one projection's rendering order, not a stable
		// fact identity shared by unrelated source rows.
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
	overlap := prepareTurnPriorityOverlapCount(queryTerms, text)
	denominator := minInt(len(queryTerms), 4)
	if denominator < 1 {
		denominator = 1
	}
	if overlap == 0 {
		allQueryTerms := prepareTurnRecallTerms(query)
		if inflectedOverlap := prepareTurnPriorityInflectedNonASCIIOverlapCount(allQueryTerms, text); inflectedOverlap > 0 {
			overlap = inflectedOverlap
			denominator = minInt(len(allQueryTerms), 4)
			if denominator < 1 {
				denominator = 1
			}
		}
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

func prepareTurnPriorityQuerySetFromAny(value any) []string {
	queries := []string{}
	for _, raw := range sliceFromAny(value) {
		query := strings.TrimSpace(extractionStringFromAny(raw))
		if query != "" {
			queries = append(queries, query)
		}
	}
	return queries
}

func prepareTurnPriorityQuerySetRelevance(queries []string, fallbackQuery, text string) float64 {
	if len(queries) == 0 {
		return prepareTurnPriorityRelevance(fallbackQuery, text)
	}
	best := 0.0
	for _, query := range queries {
		if score := prepareTurnPriorityRelevance(query, text); score > best {
			best = score
		}
	}
	return best
}

func prepareTurnPriorityOverlapCount(queryTerms []string, text string) int {
	textTerms := prepareTurnRecallTerms(text)
	overlap := 0
	for _, queryTerm := range queryTerms {
		matched := false
		for _, textTerm := range textTerms {
			if queryTerm == textTerm || prepareTurnPriorityInflectedNonASCIIMatch(queryTerm, textTerm) {
				matched = true
				break
			}
		}
		if matched {
			overlap++
		}
	}
	return overlap
}

func prepareTurnPriorityInflectedNonASCIIOverlapCount(queryTerms []string, text string) int {
	textTerms := prepareTurnRecallTerms(text)
	overlap := 0
	for _, queryTerm := range queryTerms {
		for _, textTerm := range textTerms {
			if prepareTurnPriorityInflectedNonASCIIMatch(queryTerm, textTerm) {
				overlap++
				break
			}
		}
	}
	return overlap
}

func prepareTurnPriorityInflectedNonASCIIMatch(left, right string) bool {
	leftRunes := []rune(strings.TrimSpace(left))
	rightRunes := []rune(strings.TrimSpace(right))
	if len(leftRunes) < 2 || len(rightRunes) < 2 || !prepareTurnContainsNonASCII(leftRunes) || !prepareTurnContainsNonASCII(rightRunes) {
		return false
	}
	lengthDelta := len(leftRunes) - len(rightRunes)
	if lengthDelta != 1 && lengthDelta != -1 {
		return false
	}
	shorter, longer := leftRunes, rightRunes
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	return string(shorter) == string(longer[:len(shorter)])
}

func prepareTurnPriorityContextQuery(rawUserInput, latestAssistantContext string, names ...[]string) string {
	parts := []string{
		strings.TrimSpace(rawUserInput),
		strings.TrimSpace(latestAssistantContext),
	}
	for _, group := range names {
		parts = append(parts, strings.Join(group, "\n"))
	}
	return strings.TrimSpace(strings.Join(nonEmptyStrings(parts), "\n"))
}

func prepareTurnPriorityTurnDistanceRecency(sourceTurn, currentTurn int) float64 {
	if sourceTurn <= 0 || currentTurn <= 0 {
		return 0.5
	}
	distance := maxInt(currentTurn-sourceTurn, 0)
	decayed := math.Pow(0.5, float64(distance)/prepareTurnPriorityRecencyHalfLifeTurns)
	return prepareTurnPriorityRounded(prepareTurnPriorityRecencyFloor + (1-prepareTurnPriorityRecencyFloor)*decayed)
}

func prepareTurnPrioritySurfaceMatches(query, surface string) bool {
	query = strings.TrimSpace(query)
	surface = strings.TrimSpace(surface)
	if query == "" || surface == "" {
		return false
	}
	if prepareTurnRecallContainsAnchor(query, surface) || prepareTurnRecallContainsAnchor(surface, query) {
		return true
	}
	terms := prepareTurnDistinctiveRecallTerms(surface)
	if len(terms) == 0 {
		terms = prepareTurnRecallTerms(surface)
	}
	return prepareTurnPriorityOverlapCount(terms, query) > 0
}

func prepareTurnPriorityStructuredBias(query string, fact prepareTurnPriorityMemoryFact) (float64, float64, float64, float64) {
	speakerBias := 0.0
	locationBias := 0.0
	storylineBias := 0.0
	speaker := strings.TrimSpace(extractionFirstNonEmpty(fact.SpeakerSurface, fact.EntitySurface))
	if prepareTurnPrioritySurfaceMatches(query, speaker) {
		speakerBias = 0.04
	}
	if prepareTurnPrioritySurfaceMatches(query, fact.LocationSurface) {
		locationBias = 0.05
	}
	if prepareTurnPrioritySurfaceMatches(query, fact.StorylineSurface) {
		storylineBias = 0.06
	}
	total := speakerBias + locationBias + storylineBias
	if total > 0.12 {
		total = 0.12
	}
	return speakerBias, locationBias, storylineBias, prepareTurnPriorityRounded(total)
}

func prepareTurnPriorityEntityKey(surface string, aliases map[string]any) string {
	surface = strings.TrimSpace(surface)
	if surface == "" {
		return ""
	}
	for alias, rawCanonical := range aliases {
		if comparableEntityKey(alias) == comparableEntityKey(surface) {
			if canonical := strings.TrimSpace(extractionStringFromAny(rawCanonical)); canonical != "" {
				return comparableEntityKey(canonical)
			}
		}
	}
	return comparableEntityKey(surface)
}

func prepareTurnPrioritySemanticMatchRank(candidate prepareTurnPriorityMemoryCandidate, semantic prepareTurnPrioritySemanticFact) int {
	if candidate.SourceTurn > 0 && semantic.SourceTurn > 0 && candidate.SourceTurn != semantic.SourceTurn {
		return 0
	}
	if collapseTextKey(candidate.CompleteText) != "" && collapseTextKey(candidate.CompleteText) == collapseTextKey(semantic.Fact.Text) {
		return 3
	}
	if candidate.FactValueKey != "" && candidate.FactValueKey == semantic.Fact.ValueKey {
		return 2
	}
	if candidate.FactFamilyKey != "" && candidate.FactFamilyKey == semantic.Fact.FamilyKey {
		return 2
	}
	return 0
}

func prepareTurnPriorityContinuityBonus(lane, sourceTable, _ string) float64 {
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
	return bonus
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
	// Retain importance_after_turn_decay in the trace; v4 preserves importance.
	out := map[string]any{
		"canonical_fact_id":           candidate.CanonicalFactID,
		"source_refs":                 []string{candidate.SourceRef},
		"source_table":                candidate.SourceTable,
		"source_row_id":               candidate.SourceRowID,
		"source_turn":                 candidate.SourceTurn,
		"lane":                        candidate.Lane,
		"relevance_score":             candidate.Relevance,
		"lexical_relevance":           candidate.LexicalRelevance,
		"importance_score":            candidate.Importance,
		"importance_after_turn_decay": candidate.Importance,
		"recency_score":               candidate.Recency,
		"continuity_bonus":            candidate.ContinuityBonus,
		"speaker_bias":                candidate.SpeakerBias,
		"location_bias":               candidate.LocationBias,
		"storyline_bias":              candidate.StorylineBias,
		"structured_bias":             candidate.StructuredBias,
		"final_score":                 candidate.FinalScore,
		"final_rank":                  candidate.FinalRank,
		"chars":                       candidate.Chars,
		"selection_status":            candidate.SelectionStatus,
		"selection_reason":            candidate.SelectionReason,
		"visibility":                  nilIfEmpty(candidate.Visibility),
		"perspective_owner":           nilIfEmpty(candidate.PerspectiveOwner),
		"allowed_viewers":             append([]string(nil), candidate.AllowedViewers...),
		"projection_source":           candidate.ProjectionSource,
		"score_lineage": map[string]any{
			"relevance_source":                              candidate.RelevanceSource,
			"semantic_unit_id":                              nilIfEmpty(candidate.SemanticUnitID),
			"semantic_similarity_source":                    nilIfEmpty(candidate.SemanticSimilaritySource),
			"importance_source":                             candidate.ImportanceSource,
			"importance_decay_source":                       "none_stored_importance_preserved",
			"source_selection_score":                        candidate.SourceSelectionScore,
			"source_selection_score_observed":               candidate.SourceSelectionScoreObserved,
			"source_selection_score_is_vector":              candidate.SourceSelectionScoreIsVector,
			"source_selection_score_used_as_fact_relevance": false,
		},
	}
	if len(candidate.IdentityMetadata) > 0 {
		out["identity_metadata"] = append([]string(nil), candidate.IdentityMetadata...)
	}
	if strings.TrimSpace(candidate.RenderedText) != "" && candidate.RenderedText != candidate.CompleteText {
		out["rendered_text"] = candidate.RenderedText
	}
	if exposeText {
		out["complete_text"] = candidate.CompleteText
	}
	if candidate.SourceOccurrence != "" {
		out["source_occurrence_key"] = candidate.SourceOccurrence
	}
	if candidate.LifecycleKey != "" {
		out["lifecycle_key"] = candidate.LifecycleKey
		out["lifecycle_transition"] = candidate.LifecycleTransition
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

func prepareTurnPriorityScore(relevance, importance, recency, continuityBonus, structuredBias float64) float64 {
	return prepareTurnPriorityRounded(relevance*0.60 + importance*0.25 + recency*0.15 + continuityBonus + structuredBias)
}

func prepareTurnPrioritySummaryID(sourceRef, text string) string {
	digest := sha256.Sum256([]byte("priority-memory-turn-summary.v1\x1f" + strings.TrimSpace(sourceRef) + "\x1f" + strings.TrimSpace(text)))
	return fmt.Sprintf("pms_%x", digest[:16])
}

func prepareTurnBuildPriorityTurnSummaries(resolved []prepareTurnPriorityMemoryCandidate) []prepareTurnPriorityTurnSummaryCandidate {
	bySource := map[string]*prepareTurnPriorityTurnSummaryCandidate{}
	order := []string{}
	for _, candidate := range resolved {
		if candidate.SourceTable != "memories" || strings.TrimSpace(candidate.ParentLineText) == "" {
			continue
		}
		key := strings.TrimSpace(candidate.SourceRef)
		if key == "" {
			key = prepareTurnPrioritySourceRef(candidate.SourceTable, candidate.SourceRowID, candidate.ParentLineText)
		}
		summary := bySource[key]
		if summary == nil {
			summary = &prepareTurnPriorityTurnSummaryCandidate{
				SummaryID:        prepareTurnPrioritySummaryID(key, candidate.ParentLineText),
				SourceRef:        key,
				SourceRowID:      candidate.SourceRowID,
				SourceOccurrence: candidate.SourceOccurrence,
				SourceTurn:       candidate.SourceTurn,
				CompleteText:     strings.TrimSpace(candidate.ParentLineText),
				Chars:            len([]rune(strings.TrimSpace(candidate.ParentLineText))),
			}
			bySource[key] = summary
			order = append(order, key)
		}
		summary.MemberFactIDs = append(summary.MemberFactIDs, candidate.CanonicalFactID)
		if summary.RepresentativeFactID == "" || candidate.FinalScore > summary.FinalScore {
			summary.FinalScore = candidate.FinalScore
			summary.RepresentativeFactID = candidate.CanonicalFactID
			summary.ScoreSource = "highest_child_fact_score"
		}
		// Aggregate retrieval describes this source as a whole. Preserve it for
		// the complete summary without assigning its similarity to sibling facts.
		if candidate.SourceSelectionScoreIsVector {
			summary.SourceVectorSimilarityObserved = true
			summary.SourceVectorSimilarity = candidate.SourceSelectionScore
			vectorScore := prepareTurnPriorityScore(candidate.SourceSelectionScore, candidate.Importance, candidate.Recency, candidate.ContinuityBonus, candidate.StructuredBias)
			if vectorScore > summary.FinalScore {
				summary.FinalScore = vectorScore
				summary.RepresentativeFactID = candidate.CanonicalFactID
				summary.ScoreSource = "aggregate_memory_vector_similarity"
			}
		}
		if candidate.SourceTurn > summary.SourceTurn {
			summary.SourceTurn = candidate.SourceTurn
		}
	}
	summaries := make([]prepareTurnPriorityTurnSummaryCandidate, 0, len(order))
	for _, key := range order {
		summaries = append(summaries, *bySource[key])
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].FinalScore != summaries[j].FinalScore {
			return summaries[i].FinalScore > summaries[j].FinalScore
		}
		if summaries[i].SourceTurn != summaries[j].SourceTurn {
			return summaries[i].SourceTurn > summaries[j].SourceTurn
		}
		return summaries[i].SummaryID < summaries[j].SummaryID
	})
	for index := range summaries {
		summaries[index].FinalRank = index + 1
	}
	return summaries
}

func prepareTurnPrioritySummaryMap(candidate prepareTurnPriorityTurnSummaryCandidate) map[string]any {
	return map[string]any{
		"summary_id": candidate.SummaryID, "source_ref": candidate.SourceRef,
		"source_table":  "memories",
		"source_row_id": candidate.SourceRowID, "source_occurrence_key": nilIfEmpty(candidate.SourceOccurrence),
		"source_turn": candidate.SourceTurn, "complete_text": candidate.CompleteText,
		"rendered_text": candidate.RenderedText,
		"final_score":   candidate.FinalScore, "final_rank": candidate.FinalRank, "chars": candidate.Chars,
		"representative_fact_id": candidate.RepresentativeFactID,
		"member_fact_ids":        append([]string(nil), candidate.MemberFactIDs...),
		"selection_status":       candidate.SelectionStatus, "selection_reason": candidate.SelectionReason,
		"score_source":                      candidate.ScoreSource,
		"source_vector_similarity":          candidate.SourceVectorSimilarity,
		"source_vector_similarity_observed": candidate.SourceVectorSimilarityObserved,
	}
}

func prepareTurnPriorityDeliveryCaps(deliveryCap int, mode string, budgets map[string]int) (map[string]int, map[string]int) {
	caps := map[string]int{}
	configured := map[string]int{}
	for _, lane := range prepareTurnMemoryDeliveryOrder {
		caps[lane] = deliveryCap
	}
	if mode != "custom" {
		return caps, configured
	}
	total := 0
	for _, lane := range prepareTurnMemoryDeliveryOrder {
		value := maxInt(budgets[lane], 0)
		configured[lane] = value
		total += value
	}
	for _, lane := range prepareTurnMemoryDeliveryOrder {
		value := configured[lane]
		if total > deliveryCap && total > 0 {
			value = value * deliveryCap / total
		}
		if value <= 0 {
			value = deliveryCap
		}
		caps[lane] = value
	}
	return caps, configured
}

func prepareTurnBuildPriorityCandidates(out *prepareTurnInjectionAssembly, query string, querySet []string, currentTurn int, semanticFacts []prepareTurnPrioritySemanticFact) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityIdentityMetadata) {
	lineageByText := prepareTurnPriorityMemoryLineageByText(out.MemoryDeliveryLineage)
	metadataByText := prepareTurnPrioritySourceMetadataByText(out)
	candidates := []prepareTurnPriorityMemoryCandidate{}
	identityMetadata := []prepareTurnPriorityIdentityMetadata{}
	seededParentKeys := map[string]bool{}
	appendCandidate := func(seed prepareTurnPriorityFactSeed) {
		fact := seed.Fact
		if strings.TrimSpace(fact.Text) == "" {
			return
		}
		sourceRef := prepareTurnPrioritySourceRef(seed.SourceTable, seed.SourceRowID, seed.ParentLineKey)
		entityKey := prepareTurnPriorityEntityKey(fact.EntitySurface, out.PriorityEntityAliases)
		if fact.MemoryRole == "identity_metadata" {
			identityMetadata = append(identityMetadata, prepareTurnPriorityIdentityMetadata{
				Text: fact.Text, EntityKey: entityKey, SourceTable: seed.SourceTable, SourceRef: sourceRef,
				SelectionStatus: "metadata", SelectionReason: "attached_support_not_k_candidate",
			})
			return
		}
		importance := 0.5
		importanceSource := "default_neutral"
		if seed.ImportancePresent {
			importance = seed.Importance
			importanceSource = "stored_source_value"
		}
		lexicalRelevance := prepareTurnPriorityQuerySetRelevance(querySet, query, fact.Text)
		relevance := lexicalRelevance
		relevanceSource := "atomic_fact_query"
		if seed.SemanticSimilarityObserved {
			relevance = prepareTurnPriorityNormalizeScore(seed.SemanticSimilarity)
			relevanceSource = "precise_memory_unit_vector_similarity"
		}
		speakerBias, locationBias, storylineBias, structuredBias := prepareTurnPriorityStructuredBias(query, fact)
		identity, entityIdentityObserved := prepareTurnPriorityCanonicalEntityIdentity(fact, out.PriorityEntityAliases)
		if identity == "" {
			identity = collapseTextKey(fact.Text)
		}
		if seed.SourceOccurrence != "" {
			if fact.ArrayOrdinalPath {
				identity = seed.SourceOccurrence + "\x1f" + identity
			} else if entityIdentityObserved && seed.SourceTable == "character_states" {
				// Reviewed aliases already supply the current-state family.
			} else if prepareTurnPriorityUsesCanonicalFieldIdentity(seed.SourceTable, fact) {
				// Structured current-state fields resolve across row revisions.
			} else {
				// Keep the fact identity under its source occurrence. This still
				// deduplicates the exact same fact, but changed plan/progress/
				// completion/follow-up expressions remain independent candidates.
				identity = seed.SourceOccurrence + "\x1f" + identity
			}
		}
		viewers := append([]string(nil), seed.AllowedViewers...)
		sort.Strings(viewers)
		scopeKey := strings.Join([]string{strings.TrimSpace(seed.Visibility), strings.TrimSpace(seed.PerspectiveOwner), strings.Join(viewers, "\x1f")}, "\x1f")
		if strings.Trim(scopeKey, "\x1f") != "" {
			identity = "scope:" + scopeKey + "\x1e" + identity
		}
		factIdentity := identity
		if prepareTurnPriorityUsesCanonicalFieldIdentity(seed.SourceTable, fact) || (entityIdentityObserved && seed.SourceTable == "character_states" && !fact.ArrayOrdinalPath) {
			// A field groups updates; an evidence reference names one source/value.
			// Supplemental retrieval can therefore expose a changed value without
			// replacing a first-round recommendation behind its existing reference.
			factIdentity += "\x1f" + sourceRef + "\x1f" + strconv.Itoa(seed.SourceTurn) + "\x1f" + fact.Text
		}
		candidates = append(candidates, prepareTurnPriorityMemoryCandidate{
			CanonicalFactID: prepareTurnPriorityFactID(factIdentity),
			CanonicalKey:    identity, SourceTable: seed.SourceTable, SourceRef: sourceRef,
			SourceRowID: seed.SourceRowID, SourceOccurrence: seed.SourceOccurrence,
			Lane: seed.Lane, Tier: seed.Tier, CompleteText: fact.Text,
			SourceTurn: seed.SourceTurn, Relevance: relevance, LexicalRelevance: lexicalRelevance, Importance: importance,
			ContinuityBonus: prepareTurnPriorityContinuityBonus(seed.Lane, seed.SourceTable, fact.Text),
			SpeakerBias:     speakerBias, LocationBias: locationBias, StorylineBias: storylineBias, StructuredBias: structuredBias,
			Chars: len([]rune(fact.Text)), Visibility: strings.TrimSpace(seed.Visibility),
			PerspectiveOwner: strings.TrimSpace(seed.PerspectiveOwner), AllowedViewers: viewers,
			ProjectionSource: seed.ProjectionSource, ParentLineText: seed.ParentLineKey,
			SourceFactCount: seed.SourceFactCount, RelevanceSource: relevanceSource,
			SemanticUnitID: strings.TrimSpace(seed.SemanticUnitID), SemanticSimilaritySource: strings.TrimSpace(seed.SemanticSimilaritySource),
			ImportanceSource: importanceSource, SourceSelectionScore: seed.SourceSelectionScore,
			SourceSelectionScoreObserved: seed.SourceSelectionScoreObserved,
			SourceSelectionScoreIsVector: seed.SourceSelectionScoreIsVector,
			EntityKey:                    entityKey, FactFamilyKey: fact.FamilyKey, FactValueKey: fact.ValueKey,
			LifecycleKey: fact.LifecycleKey, LifecycleTransition: fact.LifecycleTransition,
			SpeakerSurface: fact.SpeakerSurface, LocationSurface: fact.LocationSurface, StorylineSurface: fact.StorylineSurface,
		})
	}
	for _, seed := range out.PriorityFactSeeds {
		seededParentKeys[seed.SourceTable+"\x1f"+seed.ParentLineKey] = true
		appendCandidate(seed)
	}
	for _, input := range prepareTurnPriorityMemoryInputs(out) {
		for _, line := range prepareTurnDeliveryItems(input.Text) {
			if (input.SourceTable == "persona_memory_entries" || input.SourceTable == "protagonist_entity_memories") && !strings.HasPrefix(strings.TrimSpace(line), "-") {
				continue
			}
			var lineage map[string]any
			lineageKey := prepareTurnPriorityCleanLine(line)
			if seededParentKeys[input.SourceTable+"\x1f"+lineageKey] {
				continue
			}
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
			facts := prepareTurnPrioritySplitFact(line)
			for _, fact := range facts {
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
				importancePresent := false
				if lineage != nil {
					importance = prepareTurnPriorityNormalizeScore(extractionFloatFromAny(lineage["importance_score"], 0.5))
					importancePresent = true
				}
				if sourceMetadata != nil && sourceMetadata.ImportancePresent {
					importance = sourceMetadata.Importance
					importancePresent = true
				}
				visibility := "source_scope"
				perspectiveOwner := ""
				allowedViewers := []string(nil)
				if sourceMetadata != nil {
					visibility = sourceMetadata.Visibility
					perspectiveOwner = sourceMetadata.PerspectiveOwner
					allowedViewers = append([]string(nil), sourceMetadata.AllowedViewers...)
				}
				sourceSelectionScore := prepareTurnPriorityNormalizeScore(extractionFloatFromAny(lineage["selection_score"], 0))
				seed := prepareTurnPriorityFactSeed{
					Lane: input.Lane, SourceTable: input.SourceTable, Tier: input.Tier, Fact: fact,
					SourceRowID: sourceRowID, SourceOccurrence: sourceOccurrence, SourceTurn: sourceTurn,
					Importance: importance, ImportancePresent: importancePresent,
					Visibility: visibility, PerspectiveOwner: perspectiveOwner, AllowedViewers: allowedViewers,
					ProjectionSource: "legacy_rendered_line", ParentLineKey: lineageKey, SourceFactCount: len(facts),
					SourceSelectionScore: sourceSelectionScore, SourceSelectionScoreObserved: sourceSelectionScore > 0,
					SourceSelectionScoreIsVector: boolFromAny(lineage["vector_hit"]),
				}
				appendCandidate(seed)
			}
		}
	}
	semanticFacts = append([]prepareTurnPrioritySemanticFact(nil), semanticFacts...)
	sort.SliceStable(semanticFacts, func(i, j int) bool {
		if semanticFacts[i].Similarity != semanticFacts[j].Similarity {
			return semanticFacts[i].Similarity > semanticFacts[j].Similarity
		}
		return semanticFacts[i].UnitID < semanticFacts[j].UnitID
	})
	for _, semantic := range semanticFacts {
		bestIndex := -1
		bestRank := 0
		for index := range candidates {
			rank := prepareTurnPrioritySemanticMatchRank(candidates[index], semantic)
			if rank > bestRank {
				bestRank = rank
				bestIndex = index
			}
		}
		if bestIndex >= 0 {
			candidate := &candidates[bestIndex]
			if candidate.SemanticUnitID == "" || semantic.Similarity > candidate.Relevance {
				candidate.Relevance = prepareTurnPriorityNormalizeScore(semantic.Similarity)
				candidate.RelevanceSource = "precise_memory_unit_vector_similarity"
				candidate.SemanticUnitID = semantic.UnitID
				candidate.SemanticSimilaritySource = semantic.SimilaritySource
			}
			continue
		}
		lane := strings.TrimSpace(semantic.Lane)
		if lane == "" {
			lane = "event_recent"
		}
		appendCandidate(prepareTurnPriorityFactSeed{
			Lane: lane, SourceTable: "precise_memory_units", Tier: "required", Fact: semantic.Fact,
			SourceRowID: semantic.UnitID, SourceOccurrence: "precise-memory-unit:" + semantic.UnitID,
			SourceTurn: semantic.SourceTurn, Visibility: semantic.Visibility,
			ProjectionSource: "precise_memory_unit", ParentLineKey: semantic.Fact.Text, SourceFactCount: 1,
			SemanticSimilarity: semantic.Similarity, SemanticSimilarityObserved: true,
			SemanticUnitID: semantic.UnitID, SemanticSimilaritySource: semantic.SimilaritySource,
		})
	}
	maxTurn := 0
	for _, candidate := range candidates {
		if candidate.SourceTurn <= 0 {
			continue
		}
		if candidate.SourceTurn > maxTurn {
			maxTurn = candidate.SourceTurn
		}
	}
	if currentTurn <= 0 && maxTurn > 0 {
		currentTurn = maxTurn + 1
	}
	for index := range candidates {
		candidate := &candidates[index]
		candidate.Recency = prepareTurnPriorityTurnDistanceRecency(candidate.SourceTurn, currentTurn)
		candidate.FinalScore = prepareTurnPriorityScore(candidate.Relevance, candidate.Importance, candidate.Recency, candidate.ContinuityBonus, candidate.StructuredBias)
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
			if left.SourceTurn > right.SourceTurn ||
				(left.SourceTurn == right.SourceTurn && left.FinalScore > right.FinalScore) {
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
	return resolved, superseded, identityMetadata
}

func buildPrepareTurnPriorityMemoryDeliveryPlan(out *prepareTurnInjectionAssembly, maxChars, maxItems int, budgetMode string, budgets map[string]int, perspective map[string]any) map[string]any {
	deliveryCap := maxInt(maxChars, 0)
	if maxItems < 1 {
		maxItems = 1
	}
	query := strings.TrimSpace(extractionStringFromAny(perspective["_priority_memory_query"]))
	querySource := strings.TrimSpace(extractionStringFromAny(perspective["_priority_memory_query_source"]))
	querySet := prepareTurnPriorityQuerySetFromAny(perspective[prepareTurnPriorityQuerySetContextKey])
	if querySource == "" {
		querySource = "assembly_context"
	}
	currentTurn := intFromAny(perspective["_priority_memory_current_turn"], 0)
	semanticFacts := prepareTurnPrioritySemanticFactsFromAny(perspective[prepareTurnPrioritySemanticFactsContextKey])
	resolved, superseded, identityMetadata := prepareTurnBuildPriorityCandidates(out, query, querySet, currentTurn, semanticFacts)
	turnSummaries := prepareTurnBuildPriorityTurnSummaries(resolved)
	// Keep the same request's resolved source pool before selection/rendering.
	// Preprocessing consumes these typed copies, including private source text.
	out.priorityCandidates, out.priorityTurnSummaries = clonePrepareTurnPriorityCandidatePool(resolved, turnSummaries)
	if out.Preprocessing != nil {
		resolved, turnSummaries = clonePrepareTurnPriorityCandidatePool(out.Preprocessing.Candidates, out.Preprocessing.Summaries)
		multiAgentOrderCandidates(out.Preprocessing, resolved, turnSummaries)
	}
	laneCaps, configuredLaneBudgets := prepareTurnPriorityDeliveryCaps(deliveryCap, budgetMode, budgets)
	identityMetadataByEntity := map[string][]int{}
	for index := range identityMetadata {
		if identityMetadata[index].EntityKey != "" {
			identityMetadataByEntity[identityMetadata[index].EntityKey] = append(identityMetadataByEntity[identityMetadata[index].EntityKey], index)
		}
	}
	selected := map[string][]string{}
	usedGlobal := 0
	usedByLane := map[string]int{}
	authoritySelected := 0
	authorityCandidateCount := 0
	authorityCandidateChars := 0
	authorityDeferredReasons := map[string]int{}
	authorityExactKeys := map[string]bool{}
	authorityLaneKeys := map[string]bool{}
	budgetReason := func(lane string, newText, oldText string) (int, string) {
		delta := len([]rune(newText)) - len([]rune(oldText))
		if oldText == "" && newText != "" && usedGlobal > 0 {
			delta += 2 // The final text joins nonempty sections with a blank line.
		}
		if len([]rune(newText)) > laneCaps[lane] {
			return delta, "memory_lane_char_budget_reached"
		}
		if usedGlobal+delta > deliveryCap {
			return delta, "memory_char_budget_reached"
		}
		return delta, ""
	}
	appendAuthority := func(lane string, items []string, preserveDistinctSourceOccurrences bool) {
		for _, item := range items {
			exactKey := collapseTextKey(prepareTurnPriorityCleanLine(item))
			laneKey := lane + "\x1f" + exactKey
			if exactKey == "" || (!preserveDistinctSourceOccurrences && authorityLaneKeys[laneKey]) {
				continue
			}
			authorityCandidateCount++
			authorityCandidateChars += len([]rune(item))
			candidate := append(append([]string{}, selected[lane]...), item)
			newText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", candidate)
			oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", selected[lane])
			delta, reason := budgetReason(lane, newText, oldText)
			if reason != "" {
				authorityDeferredReasons[reason]++
				continue
			}
			selected[lane] = candidate
			usedGlobal += delta
			usedByLane[lane] = len([]rune(newText))
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

	priorityFactSelected := 0
	turnSummarySelected := 0
	quotaEligible := map[string]int{}
	quotaSelected := map[string]int{}
	quotaDeferredByLimit := map[string]int{}
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
	selectedTurnSummaryExactKeys := map[string]bool{}
	preprocessingRefs := multiAgentSelectionReferences(out.Preprocessing)
	appendSummary := func(index int) {
		summary := &turnSummaries[index]
		aiSelected := out.Preprocessing.usesAI("event_recent")
		if !multiAgentWants(out.Preprocessing, "event_recent", summary.SummaryID, true) {
			summary.SelectionStatus, summary.SelectionReason = "deferred", "ai_not_selected"
			if !aiSelected {
				summary.SelectionReason = "go_baseline_not_selected"
			}
			return
		}
		if out.Preprocessing == nil && authorityExactKeys[collapseTextKey(summary.CompleteText)] {
			summary.SelectionStatus = "deferred"
			summary.SelectionReason = "authority_exact_duplicate"
			return
		}
		quotaEligible["turn_summary"]++
		line := "- " + summary.CompleteText
		if summary.SourceTurn > 0 {
			line = fmt.Sprintf("- [turn %d] %s", summary.SourceTurn, summary.CompleteText)
		}
		if ref := preprocessingRefs[summary.SummaryID]; aiSelected && ref != "" {
			line = "- [" + ref + "] " + strings.TrimPrefix(line, "- ")
		}
		lane := "event_recent"
		laneItems := append(append([]string{}, selected[lane]...), line)
		newText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", renderLaneItems(lane, laneItems, nil))
		oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", renderLaneItems(lane, selected[lane], nil))
		delta, reason := budgetReason(lane, newText, oldText)
		if reason != "" && out.Preprocessing == nil {
			summary.SelectionStatus = "deferred"
			summary.SelectionReason = reason
			return
		}
		selected[lane] = laneItems
		summary.RenderedText = strings.TrimPrefix(line, "- ")
		usedGlobal += delta
		usedByLane[lane] = len([]rune(newText))
		turnSummarySelected++
		quotaSelected["turn_summary"]++
		selectedTurnSummaryExactKeys[collapseTextKey(summary.CompleteText)] = true
		summary.SelectionStatus = "selected"
		summary.SelectionReason = "turn_summary_priority_rank"
		if aiSelected {
			summary.SelectionReason = "ai_recommendation"
		}
	}
	identityMetadataApplied := map[string]bool{}
	appendFact := func(index int) {
		candidate := &resolved[index]
		aiSelected := out.Preprocessing.usesAI(candidate.Lane)
		if !multiAgentWants(out.Preprocessing, candidate.Lane, candidate.CanonicalFactID, false) {
			candidate.SelectionStatus, candidate.SelectionReason = "deferred", "ai_not_selected"
			if !aiSelected {
				candidate.SelectionReason = "go_baseline_not_selected"
			}
			return
		}
		if out.Preprocessing == nil && authorityExactKeys[collapseTextKey(candidate.CompleteText)] {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = "authority_exact_duplicate"
			return
		}
		if out.Preprocessing == nil && selectedTurnSummaryExactKeys[collapseTextKey(candidate.CompleteText)] {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = "turn_summary_exact_duplicate"
			return
		}
		quotaEligible[candidate.Lane]++
		candidate.RenderedText = candidate.CompleteText
		if candidate.SourceTurn > 0 {
			candidate.RenderedText = fmt.Sprintf("[source turn %d] %s", candidate.SourceTurn, candidate.CompleteText)
			if candidate.SourceTable == "character_states" {
				candidate.RenderedText = fmt.Sprintf("[state snapshot turn %d; fields may be older] %s", candidate.SourceTurn, candidate.CompleteText)
			}
		}
		if ref := preprocessingRefs[candidate.CanonicalFactID]; aiSelected && ref != "" {
			candidate.RenderedText = "[" + ref + "] " + candidate.RenderedText
		}
		line := "- " + candidate.RenderedText
		laneItems := append(append([]string{}, selected[candidate.Lane]...), line)
		extraGuidance := guidanceForCandidate(candidate)
		newText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[candidate.Lane]+"]", renderLaneItems(candidate.Lane, laneItems, extraGuidance))
		oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[candidate.Lane]+"]", renderLaneItems(candidate.Lane, selected[candidate.Lane], nil))
		delta, reason := budgetReason(candidate.Lane, newText, oldText)
		if reason != "" && out.Preprocessing == nil {
			candidate.SelectionStatus = "deferred"
			candidate.SelectionReason = reason
			return
		}
		if candidate.EntityKey != "" && !identityMetadataApplied[candidate.EntityKey] {
			metadataIndexes := identityMetadataByEntity[candidate.EntityKey]
			metadataText := []string{}
			seenMetadata := map[string]bool{}
			for _, metadataIndex := range metadataIndexes {
				text := strings.TrimSpace(identityMetadata[metadataIndex].Text)
				key := collapseTextKey(text)
				if text == "" || seenMetadata[key] {
					continue
				}
				seenMetadata[key] = true
				metadataText = append(metadataText, text)
			}
			if len(metadataText) > 0 {
				enrichedText := candidate.RenderedText + " | identity_metadata: " + strings.Join(metadataText, "; ")
				enrichedItems := append(append([]string{}, selected[candidate.Lane]...), "- "+enrichedText)
				enrichedSection := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[candidate.Lane]+"]", renderLaneItems(candidate.Lane, enrichedItems, extraGuidance))
				enrichedDelta, enrichedReason := budgetReason(candidate.Lane, enrichedSection, oldText)
				if enrichedReason == "" {
					candidate.RenderedText = enrichedText
					candidate.IdentityMetadata = append([]string(nil), metadataText...)
					laneItems = enrichedItems
					newText = enrichedSection
					delta = enrichedDelta
					identityMetadataApplied[candidate.EntityKey] = true
					for _, metadataIndex := range metadataIndexes {
						identityMetadata[metadataIndex].SelectionStatus = "attached"
						identityMetadata[metadataIndex].SelectionReason = "selected_fact_identity_support"
						identityMetadata[metadataIndex].AttachedFactID = candidate.CanonicalFactID
					}
				}
			}
		}
		if len(extraGuidance) > 0 {
			guidanceByLane[candidate.Lane] = append(guidanceByLane[candidate.Lane], extraGuidance...)
			guidanceSourceApplied[candidate.SourceTable] = true
		}
		selected[candidate.Lane] = laneItems
		usedGlobal += delta
		usedByLane[candidate.Lane] = len([]rune(newText))
		priorityFactSelected++
		quotaSelected[candidate.Lane]++
		candidate.SelectionStatus = "selected"
		candidate.SelectionReason = "lane_priority_rank"
		if aiSelected {
			candidate.SelectionReason = "ai_recommendation"
		}
	}
	// Give every populated group its core turn before any group's overflow.
	// K is a priority target, not a second delivery ceiling after the budget.
	type deliveryItem struct {
		index   int
		summary bool
		score   float64
	}
	groups := map[string][]deliveryItem{}
	for index, summary := range turnSummaries {
		groups["turn_summary"] = append(groups["turn_summary"], deliveryItem{index, true, summary.FinalScore})
	}
	for index, candidate := range resolved {
		groups[candidate.Lane] = append(groups[candidate.Lane], deliveryItem{index, false, candidate.FinalScore})
	}
	keys := append([]string{"turn_summary"}, multiAgentRoles...)
	appendItem := func(item deliveryItem) {
		if item.summary {
			appendSummary(item.index)
		} else {
			appendFact(item.index)
		}
	}
	coreRounds := 0
	for _, key := range keys {
		coreRounds = maxInt(coreRounds, minInt(maxItems, len(groups[key])))
	}
	for rank := 0; rank < coreRounds; rank++ {
		for _, key := range keys {
			if rank < len(groups[key]) {
				appendItem(groups[key][rank])
			}
		}
	}
	// Merge the remaining group heads by score. Each group keeps its existing
	// order, including an AI's explicit order even when its scores differ.
	positions := map[string]int{}
	for _, key := range keys {
		positions[key] = minInt(maxItems, len(groups[key]))
	}
	for {
		nextKey := ""
		for _, key := range keys {
			position := positions[key]
			if position < len(groups[key]) && (nextKey == "" || groups[key][position].score > groups[nextKey][positions[nextKey]].score) {
				nextKey = key
			}
		}
		if nextKey == "" {
			break
		}
		appendItem(groups[nextKey][positions[nextKey]])
		positions[nextKey]++
	}
	prioritySelected := priorityFactSelected + turnSummarySelected

	classes := []map[string]any{}
	parts := []string{}
	classEligible := map[string]int{}
	classDeferred := map[string]int{}
	classEligible["event_recent"] += len(turnSummaries)
	for _, summary := range turnSummaries {
		if summary.SelectionStatus != "selected" {
			classDeferred["event_recent"]++
		}
	}
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
		selectionPolicy := "independent_lane_priority_score"
		if lane == "direct_evidence" || lane == "protected_secret" {
			selectionPolicy = "authority_exempt"
		} else if out.Preprocessing.usesAI(lane) {
			selectionPolicy = "ai_recommendation_order"
		} else if out.Preprocessing != nil {
			selectionPolicy = "go_baseline_selection"
		}
		if text != "" {
			parts = append(parts, text)
		}
		classes = append(classes, map[string]any{
			"key": lane, "title": prepareTurnMemoryDeliveryTitles[lane],
			"used_chars": usedByLane[lane], "eligible_count": classEligible[lane],
			"selected_count": len(selected[lane]), "deferred_count": classDeferred[lane],
			"selection_policy":     selectionPolicy,
			"budget_overrun_chars": maxInt(usedByLane[lane]-laneCaps[lane], 0),
			"reserved_chars":       laneCaps[lane], "configured_reserved_chars": configuredLaneBudgets[lane],
			"unused_chars": maxInt(laneCaps[lane]-usedByLane[lane], 0), "text": nilIfEmpty(text),
		})
	}
	finalText := strings.Join(parts, "\n\n")
	finalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalText)))
	priorityItems := make([]map[string]any, 0, len(resolved)+len(superseded))
	selectedFactIDs := []string{}
	exclusionReasons := map[string]int{}
	priorityCandidateChars := 0
	projectionCounts := map[string]int{}
	for _, candidate := range resolved {
		priorityCandidateChars += candidate.Chars
		projectionCounts[candidate.ProjectionSource]++
		exposeText := candidate.SourceTable != "protagonist_entity_memories" || candidate.SelectionStatus == "selected"
		priorityItems = append(priorityItems, prepareTurnPriorityCandidateMap(candidate, exposeText))
		if candidate.SelectionStatus == "selected" {
			selectedFactIDs = append(selectedFactIDs, candidate.CanonicalFactID)
		} else if candidate.SelectionReason != "" {
			exclusionReasons[candidate.SelectionReason]++
		}
	}
	for _, candidate := range superseded {
		priorityCandidateChars += candidate.Chars
		projectionCounts[candidate.ProjectionSource]++
		exposeText := candidate.SourceTable != "protagonist_entity_memories"
		priorityItems = append(priorityItems, prepareTurnPriorityCandidateMap(candidate, exposeText))
		exclusionReasons[candidate.SelectionReason]++
	}
	turnSummaryItems := make([]map[string]any, 0, len(turnSummaries))
	selectedTurnSummaryIDs := []string{}
	turnSummaryCandidateChars := 0
	for _, summary := range turnSummaries {
		turnSummaryCandidateChars += summary.Chars
		turnSummaryItems = append(turnSummaryItems, prepareTurnPrioritySummaryMap(summary))
		if summary.SelectionStatus == "selected" {
			selectedTurnSummaryIDs = append(selectedTurnSummaryIDs, summary.SummaryID)
		} else if summary.SelectionReason != "" {
			exclusionReasons[summary.SelectionReason]++
		}
	}
	for reason, count := range authorityDeferredReasons {
		exclusionReasons[reason] += count
	}
	identityMetadataItems := make([]map[string]any, 0, len(identityMetadata))
	identityMetadataAttached := 0
	for _, metadata := range identityMetadata {
		if metadata.SelectionStatus == "attached" {
			identityMetadataAttached++
		}
		identityMetadataItems = append(identityMetadataItems, map[string]any{
			"text": metadata.Text, "entity_key": metadata.EntityKey,
			"source_table": metadata.SourceTable, "source_ref": metadata.SourceRef,
			"status": metadata.SelectionStatus, "reason": metadata.SelectionReason,
			"attached_fact_id": nilIfEmpty(metadata.AttachedFactID),
		})
	}
	totalCandidateCount := len(turnSummaries) + len(resolved) + len(superseded) + authorityCandidateCount
	totalSelectedCount := prioritySelected + authoritySelected
	totalCandidateChars := maxInt(turnSummaryCandidateChars+priorityCandidateChars+authorityCandidateChars, len([]rune(finalText)))
	relevanceQueryCount := len(querySet)
	if relevanceQueryCount == 0 && query != "" {
		relevanceQueryCount = 1
	}
	quotaDeferredByBudget := map[string]int{}
	for _, summary := range turnSummaries {
		if summary.SelectionReason == "memory_char_budget_reached" || summary.SelectionReason == "memory_lane_char_budget_reached" {
			quotaDeferredByBudget["turn_summary"]++
		}
	}
	for _, candidate := range resolved {
		if candidate.SelectionReason == "memory_char_budget_reached" || candidate.SelectionReason == "memory_lane_char_budget_reached" {
			quotaDeferredByBudget[candidate.Lane]++
		}
	}
	quotaGroups := []map[string]any{}
	quotaCapacity := 0
	quotaKeys := append([]string{"turn_summary"}, prepareTurnMemoryDeliveryOrder...)
	for _, key := range quotaKeys {
		if key == "direct_evidence" || key == "protected_secret" {
			continue
		}
		eligible := quotaEligible[key]
		capacity := minInt(maxItems, eligible)
		quotaCapacity += capacity
		quotaGroups = append(quotaGroups, map[string]any{
			"key": key, "requested_max_items": maxItems, "core_priority_target": maxItems, "eligible_count": eligible,
			"selected_count": quotaSelected[key], "deferred_by_limit_count": quotaDeferredByLimit[key],
			"deferred_by_budget_count": quotaDeferredByBudget[key],
			"unused_slots":             maxInt(capacity-quotaSelected[key], 0),
		})
	}
	deferredByLimitCount := 0
	priorityEligibleCount := 0
	for _, count := range quotaDeferredByLimit {
		deferredByLimitCount += count
	}
	for key, count := range quotaEligible {
		if key != "direct_evidence" && key != "protected_secret" {
			priorityEligibleCount += count
		}
	}
	return map[string]any{
		"contract_version":     prepareTurnPriorityMemoryPlanVersion,
		"preprocessing":        out.Preprocessing,
		"budget_overrun_chars": maxInt(len([]rune(finalText))-deliveryCap, 0),
		"status":               "ready", "mode": budgetMode, "owner": "go",
		"score_version":      prepareTurnPriorityMemoryScoreVersion,
		"score_formula":      "relevance*0.60+importance*0.25+turn_distance_recency*0.15+continuity_bonus+structured_bias",
		"final_budget_owner": "go_priority_memory_delivery_plan",
		"global_cap_chars":   maxChars, "delivery_cap_chars": deliveryCap,
		"priority_memory_max_items":        maxItems,
		"priority_candidate_count":         len(turnSummaries) + len(resolved) + len(superseded),
		"priority_resolved_count":          len(resolved),
		"priority_selected_count":          prioritySelected,
		"priority_fact_selected_count":     priorityFactSelected,
		"turn_summary_candidate_count":     len(turnSummaries),
		"turn_summary_selected_count":      turnSummarySelected,
		"authority_exempt_selected_count":  authoritySelected,
		"candidate_count":                  totalCandidateCount,
		"candidate_chars":                  totalCandidateChars,
		"candidate_unit":                   "complete_turn_summary+atomic_fact",
		"fact_seed_count":                  len(out.PriorityFactSeeds),
		"projection_counts":                projectionCounts,
		"relevance_query_source":           querySource,
		"relevance_query_count":            relevanceQueryCount,
		"semantic_fact_vector_count":       len(semanticFacts),
		"semantic_fact_vector_trace":       perspective["_priority_precise_vector_trace"],
		"recency_policy":                   "rp_turn_distance_half_life_32_floor_0.20",
		"importance_decay_policy":          "stored_importance_preserved_recency_separate",
		"structured_bias_policy":           "speaker_0.04_location_0.05_storyline_0.06_cap_0.12_score_only",
		"lifecycle_ranking_policy":         "diagnostic_only_not_scored",
		"identity_metadata_count":          len(identityMetadata),
		"identity_metadata_attached_count": identityMetadataAttached,
		"identity_metadata_items":          identityMetadataItems,
		"selected_count":                   totalSelectedCount,
		"selected_chars":                   len([]rune(finalText)),
		"final_delivery_count":             totalSelectedCount,
		"final_delivery_chars":             len([]rune(finalText)),
		"excluded_count":                   totalCandidateCount - totalSelectedCount,
		"exclusion_reasons":                exclusionReasons,
		"used_chars":                       len([]rune(finalText)), "order": prepareTurnMemoryDeliveryOrder,
		"selection_order":                   "core_round_robin_then_remaining_score_within_character_budgets",
		"priority_k_semantics":              "core_priority_target_per_group_not_delivery_maximum",
		"low_score_backfill_after_k":        true,
		"unused_k_transfer_between_groups":  false,
		"configured_lane_budgets":           configuredLaneBudgets,
		"effective_lane_caps":               laneCaps,
		"source_rows_mutated":               false,
		"request_scoped_current_resolution": true,
		"visibility_handling":               "existing_source_scope_carried_per_fact_without_new_rejection_gate",
		"priority_items":                    priorityItems,
		"selected_fact_ids":                 selectedFactIDs,
		"turn_summary_items":                turnSummaryItems,
		"selected_turn_summary_ids":         selectedTurnSummaryIDs,
		"final_text_sha256":                 finalHash,
		"rendering_hash":                    finalHash,
		"classes":                           classes,
		"final_text":                        nilIfEmpty(finalText),
		"core_objective_memory": map[string]any{
			"contract_version": "core_priority_memory_delivery.v4",
			"status":           "active", "requested_max_items": maxItems, "core_priority_target": maxItems,
			"eligible_distinct_count": priorityEligibleCount, "delivered_count": prioritySelected,
			"deferred_by_limit_count":  deferredByLimitCount,
			"deferred_by_budget_count": exclusionReasons["memory_char_budget_reached"] + exclusionReasons["memory_lane_char_budget_reached"],
			"missing_to_limit":         maxInt(quotaCapacity-prioritySelected, 0),
			"garbage_fill":             false, "top_k_reinterpreted": false,
			"counted_lane":            "complete_turn_summaries_and_each_scored_fact_lane_independently",
			"quota_groups":            quotaGroups,
			"item_count_exempt_lanes": []string{"direct_evidence", "protected_secret"},
		},
		"historical_chat_authority_policy": "previous_logical_turn_owned_by_input_context_not_direct_evidence",
		"recent_raw_turn_delivery":         "excluded_from_final_memory_delivery",
		"raw_chat_fallback_delivery":       "diagnostic_only_excluded_from_final_memory_delivery",
	}
}
