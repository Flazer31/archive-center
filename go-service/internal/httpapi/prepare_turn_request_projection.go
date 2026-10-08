package httpapi

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// This is an evaluation workspace, never a session cache. The route owns it
// until the request ends. Source values and independent query evidence remain
// separate; new search scores are applied after the reusable source conversion.
type prepareTurnRequestPreparation struct {
	metrics       *prepareTurnMeasurement
	baseQueries   []string
	baseLexical   func(string) float64
	seedTemplates map[prepareTurnSourceTemplateKey]prepareTurnSourceTemplate
	payloads      map[string]any
	surfaces      map[string]string
	facts         map[string][]prepareTurnPriorityMemoryFact
	recall        map[store.Memory]prepareTurnRecallMemory
	summaries     map[store.Memory]string
	memorySeeds   map[store.Memory][]prepareTurnPriorityFactSeed
	guards        map[prepareTurnGuardKey]prepareTurnProtectedMemoryGuardResult
	lexical       map[string]func(string) float64
	lexicalTexts  map[string]prepareTurnPriorityLexicalText
	partTerms     map[string][]string
	partNeedles   map[string]prepareTurnNeedleSegment
	termForms     map[string][]prepareTurnPriorityLexicalTerm
	partDigests   map[prepareTurnPartDigestKey][32]byte
	readingForms  map[prepareTurnMemoryFormKey]*prepareTurnMemoryForm
	readingGroups map[prepareTurnMemoryGroupKey]string
	matches       map[string]func(store.Memory) prepareTurnRecallEvidence
	similarities  map[string]map[store.Memory]float64
}

func newPrepareTurnRequestPreparation(common *prepareTurnAssemblyCommon) *prepareTurnRequestPreparation {
	recall := make(map[store.Memory]prepareTurnRecallMemory, len(common.RecallMemories))
	for item, value := range common.RecallMemories {
		recall[item] = value
	}
	return &prepareTurnRequestPreparation{
		seedTemplates: map[prepareTurnSourceTemplateKey]prepareTurnSourceTemplate{},
		payloads:      map[string]any{}, surfaces: map[string]string{}, facts: map[string][]prepareTurnPriorityMemoryFact{},
		recall: recall, summaries: map[store.Memory]string{},
		memorySeeds: map[store.Memory][]prepareTurnPriorityFactSeed{},
		guards:      map[prepareTurnGuardKey]prepareTurnProtectedMemoryGuardResult{},
		lexical:     map[string]func(string) float64{}, matches: map[string]func(store.Memory) prepareTurnRecallEvidence{},
		lexicalTexts: map[string]prepareTurnPriorityLexicalText{},
		partTerms:    map[string][]string{}, partNeedles: map[string]prepareTurnNeedleSegment{},
		termForms:    map[string][]prepareTurnPriorityLexicalTerm{},
		partDigests:  map[prepareTurnPartDigestKey][32]byte{},
		readingForms: map[prepareTurnMemoryFormKey]*prepareTurnMemoryForm{}, readingGroups: map[prepareTurnMemoryGroupKey]string{},
		similarities: map[string]map[store.Memory]float64{},
	}
}

type prepareTurnGuardKey struct {
	item store.Memory
	pov  string
}

func (p *prepareTurnRequestPreparation) sourceMap(raw string) map[string]any {
	if p == nil {
		return parseJSONMap(raw)
	}
	value, ok := p.payload(raw).(map[string]any)
	if !ok || value == nil {
		return map[string]any{}
	}
	return value
}

func (p *prepareTurnRequestPreparation) guard(item store.Memory, perspective ...map[string]any) prepareTurnProtectedMemoryGuardResult {
	if p == nil {
		return prepareTurnProtectedMemoryGuard(item, perspective...)
	}
	key := prepareTurnGuardKey{item: item}
	if len(perspective) > 0 {
		key.pov = mustCompactJSON(perspective[0])
	}
	if value, ok := p.guards[key]; ok {
		return value
	}
	value := prepareTurnProtectedMemoryGuardFromParsed(p.sourceMap(item.SummaryJSON), perspective...)
	p.guards[key] = value
	return value
}

func (p *prepareTurnRequestPreparation) entityMatches(item store.Memory, entities []string) []string {
	return prepareTurnMemoryEntityMatches(p.summary(item), p.recallSource(item).anchors, entities)
}

type prepareTurnSourceTemplateKey struct {
	source  [32]byte
	query   string
	reading [32]byte
}
type prepareTurnSourceTemplate struct {
	candidate prepareTurnPriorityMemoryCandidate
	metadata  *prepareTurnPriorityIdentityMetadata
}

func prepareTurnSourceSeedKey(seed prepareTurnPriorityFactSeed, query string) prepareTurnSourceTemplateKey {
	return prepareTurnSourceSeedKeyWith(seed, query, seed.Fact.Reading.sourceFingerprint())
}

func (p *prepareTurnRequestPreparation) sourceSeedKey(seed prepareTurnPriorityFactSeed, query string) prepareTurnSourceTemplateKey {
	return prepareTurnSourceSeedKeyWith(seed, query, p.readingFingerprint(seed.Fact.Reading))
}

func prepareTurnSourceSeedKeyWith(seed prepareTurnPriorityFactSeed, query string, readingFingerprint [32]byte) prepareTurnSourceTemplateKey {
	// Query discoveries are observations on the source, not its identity.
	seed.SourceSelectionScore = 0
	seed.SourceSelectionScoreObserved, seed.SourceSelectionScoreIsVector = false, false
	seed.RecallQueries = nil
	seed.SemanticSimilarity, seed.SemanticSimilarityObserved = 0, false
	seed.SemanticUnitID, seed.SemanticSimilaritySource = "", ""
	encoded, _ := json.Marshal(seed)
	return prepareTurnSourceTemplateKey{sha256.Sum256(encoded), query, readingFingerprint}
}

// Callers that edit a payload must copy its map first (canonical world-state
// presentation removes displayed rules locally). The saved source is immutable.
func (p *prepareTurnRequestPreparation) payload(raw string) any {
	if value, ok := p.payloads[raw]; ok {
		return value
	}
	defer p.measurement().start("assembly.row_parse").end()
	value := parseSurfacePayload(raw)
	p.payloads[raw] = value
	return value
}

func (p *prepareTurnRequestPreparation) surface(raw string) string {
	if value, ok := p.surfaces[raw]; ok {
		return value
	}
	value := prepareTurnSurfaceText(p.payload(raw))
	p.surfaces[raw] = value
	return value
}

func prepareTurnPreparedSplitFact(out *prepareTurnInjectionAssembly, line string) []prepareTurnPriorityMemoryFact {
	if out.preparation == nil {
		return prepareTurnPrioritySplitFact(line)
	}
	p := out.preparation
	if facts, ok := p.facts[line]; ok {
		return facts
	}
	defer p.measurement().start("assembly.fact_parse").end()
	facts := prepareTurnPrioritySplitFact(line)
	p.facts[line] = facts
	return facts
}

func (p *prepareTurnRequestPreparation) similarity(query string, item store.Memory) float64 {
	values := p.similarities[query]
	if values == nil {
		values = map[store.Memory]float64{}
		p.similarities[query] = values
	}
	if value, ok := values[item]; ok {
		return value
	}
	value := simpleTokenSimilarity(query, p.recallSource(item).text)
	values[item] = value
	return value
}

func (p *prepareTurnRequestPreparation) summary(item store.Memory) string {
	if p == nil {
		return prepareTurnMemorySummary(item)
	}
	if value, ok := p.summaries[item]; ok {
		return value
	}
	value := prepareTurnMemorySummary(item)
	p.summaries[item] = value
	return value
}

func (p *prepareTurnRequestPreparation) recallSource(item store.Memory) prepareTurnRecallMemory {
	if value, ok := p.recall[item]; ok {
		return value
	}
	value := prepareTurnPrepareRecallMemory(item)
	p.recall[item] = value
	return value
}

func (p *prepareTurnRequestPreparation) recallMatcher(query string) func(store.Memory) prepareTurnRecallEvidence {
	if fn, ok := p.matches[query]; ok {
		return fn
	}
	match := prepareTurnMemoryRecallMatcher(query)
	values := map[store.Memory]prepareTurnRecallEvidence{}
	fn := func(item store.Memory) prepareTurnRecallEvidence {
		value, ok := values[item]
		if !ok {
			value = match(p.recallSource(item))
			values[item] = value
		}
		// The selector extends overlap evidence while merging independent queries.
		value.OverlapTerms = append([]string(nil), value.OverlapTerms...)
		return value
	}
	p.matches[query] = fn
	return fn
}

// Readings are their parts joined by "\n", and many readings share the same
// long parts. Recall fields never cross that separator, so the analysis of the
// joined text is composed from cached per-part fields using the same first-seen
// order as prepareTurnRecallTerms, and the needle from per-part needle
// characters with the whole text's surrounding white space removed. The result
// equals prepareTurnPriorityAnalyzeText(joined); later lookups reuse it.
func (p *prepareTurnRequestPreparation) primeJoinedLexicalText(joined string, parts []string) {
	if p == nil {
		return
	}
	if _, ok := p.lexicalTexts[joined]; ok {
		return
	}
	defer p.measurement().start("assembly.tokenize").end()
	seen := map[string]bool{}
	terms := []prepareTurnPriorityLexicalTerm{}
	for _, part := range parts {
		fields, ok := p.partTerms[part]
		if !ok {
			fields = []string{}
			distinct := map[string]bool{}
			// A repeated field cannot add a form: its own forms were added first.
			for _, field := range strings.FieldsFunc(strings.ToLower(part), prepareTurnRecallTermBreak) {
				if !distinct[field] {
					distinct[field] = true
					fields = append(fields, field)
				}
			}
			p.partTerms[part] = fields
		}
		for _, field := range fields {
			if seen[field] {
				continue
			}
			forms, ok := p.termForms[field]
			if !ok {
				for _, form := range prepareTurnRecallTermForms(field) {
					forms = append(forms, prepareTurnPriorityLexicalTermOf(form))
				}
				p.termForms[field] = forms
			}
			for _, form := range forms {
				if !seen[form.value] {
					seen[form.value] = true
					terms = append(terms, form)
				}
			}
		}
	}
	first, last := -1, -1
	for i, part := range parts {
		if strings.TrimSpace(part) != "" {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	// The needle stays segmented: middle parts are shared request segments and
	// only the trimmed first and last parts are this reading's own text.
	analyzed := prepareTurnPriorityLexicalText{terms: terms}
	if first >= 0 {
		analyzed.needleSegments = make([]prepareTurnNeedleSegment, 0, last-first+1)
		for i := first; i <= last; i++ {
			part := parts[i]
			if i != first && i != last {
				segment, ok := p.partNeedles[part]
				if !ok {
					segment = prepareTurnNeedleSegment{text: prepareTurnEntityNeedleChars(part), id: int32(len(p.partNeedles))}
					p.partNeedles[part] = segment
				}
				analyzed.needleSegments = append(analyzed.needleSegments, segment)
				continue
			}
			if i == first {
				part = strings.TrimLeftFunc(part, unicode.IsSpace)
			}
			if i == last {
				part = strings.TrimRightFunc(part, unicode.IsSpace)
			}
			analyzed.needleSegments = append(analyzed.needleSegments, prepareTurnNeedleSegment{text: prepareTurnEntityNeedleChars(part), id: -1})
		}
	}
	p.lexicalTexts[joined] = analyzed
}

// Same value as sourceFingerprint; digests of parts shared by many readings are
// computed once per request. Without a preparation it hashes directly.
func (p *prepareTurnRequestPreparation) readingFingerprint(c *prepareTurnMemoryContext) [32]byte {
	if p == nil {
		return c.sourceFingerprint()
	}
	return c.fingerprintWith(p.partDigest)
}

func (p *prepareTurnRequestPreparation) partDigest(part prepareTurnMemoryPart) [32]byte {
	key := prepareTurnPartDigestKey{part.Key, part.Label, part.Value, part.DeliveryLabel, part.ReferenceOnly, part.FactTexts == nil, ""}
	if len(part.FactTexts) > 0 {
		var facts strings.Builder
		for _, text := range part.FactTexts {
			facts.WriteString(strconv.Itoa(len(text)))
			facts.WriteByte(':')
			facts.WriteString(text)
		}
		key.facts = facts.String()
	}
	digest, ok := p.partDigests[key]
	if !ok {
		digest = prepareTurnMemoryPartDigest(part)
		p.partDigests[key] = digest
	}
	return digest
}

// Distinct parts give distinct keys: facts are length-prefixed and nil and
// empty fact lists are kept apart.
type prepareTurnPartDigestKey struct {
	key, label, value, deliveryLabel string
	referenceOnly, factsNil          bool
	facts                            string
}

// nil when the request has no preparation; callers then analyze text directly.
func prepareTurnPreparationLexicalText(p *prepareTurnRequestPreparation) func(string) prepareTurnPriorityLexicalText {
	if p == nil {
		return nil
	}
	return p.lexicalText
}

func (p *prepareTurnRequestPreparation) lexicalText(text string) prepareTurnPriorityLexicalText {
	if value, ok := p.lexicalTexts[text]; ok {
		return value
	}
	defer p.measurement().start("assembly.tokenize").end()
	value := prepareTurnPriorityAnalyzeText(text)
	p.lexicalTexts[text] = value
	return value
}

func (p *prepareTurnRequestPreparation) relevanceScorer(queries []string, fallback string) func(string) float64 {
	if len(queries) == 0 {
		queries = []string{fallback}
	}
	functions := make([]func(string) float64, 0, len(queries))
	if p.baseLexical == nil {
		p.baseQueries = append([]string{}, queries...)
		p.baseLexical = memoPrepareTurnLexicalScore(prepareTurnPriorityRelevanceScorer(queries, fallback, p.lexicalText))
	}
	// Preserve one combined base evaluation (including one text tokenization).
	// Appended questions add their own evidence without rebuilding that base.
	basePresent := len(queries) >= len(p.baseQueries)
	for i, query := range p.baseQueries {
		if i >= len(queries) || queries[i] != query {
			basePresent = false
			break
		}
	}
	if basePresent {
		functions = append(functions, p.baseLexical)
		queries = queries[len(p.baseQueries):]
	}
	for _, query := range queries {
		fn, ok := p.lexical[query]
		if !ok {
			score := prepareTurnPriorityRelevanceScorer([]string{query}, "", p.lexicalText)
			values := map[string]float64{}
			fn = func(text string) float64 {
				if value, ok := values[text]; ok {
					return value
				}
				value := score(text)
				values[text] = value
				return value
			}
			p.lexical[query] = fn
		}
		functions = append(functions, fn)
	}
	return func(text string) float64 {
		defer p.measurement().start("assembly.relevance_score").end()
		best := 0.0
		for _, score := range functions {
			best = math.Max(best, score(text))
		}
		return best
	}
}

func memoPrepareTurnLexicalScore(score func(string) float64) func(string) float64 {
	values := map[string]float64{}
	return func(text string) float64 {
		if value, ok := values[text]; ok {
			return value
		}
		value := score(text)
		values[text] = value
		return value
	}
}

func (p *prepareTurnRequestPreparation) publicMemorySeeds(item store.Memory) []prepareTurnPriorityFactSeed {
	if seeds, ok := p.memorySeeds[item]; ok {
		return seeds
	}
	projected, ok := publicMemoryFromCanonical(item)
	if !ok {
		p.memorySeeds[item] = nil
		return nil
	}
	facts, projection := prepareTurnPriorityFactsFromMemory(projected)
	summary := p.summary(projected)
	metadata := prepareTurnPrioritySourceMetadata{
		Lane: "event_recent", SourceTable: "memories", Tier: "required",
		LineKey: prepareTurnPriorityCleanLine(summary), SourceRowID: prepareTurnMemorySourceRowID(projected),
		SourceOccurrence: prepareTurnMemorySourceOccurrenceKey(projected), SourceTurn: projected.TurnIndex,
		Importance: prepareTurnPriorityNormalizeScore(projected.Importance), ImportancePresent: projected.Importance > 0, Visibility: "public_projection",
	}
	out := prepareTurnInjectionAssembly{}
	appendPrepareTurnPriorityFactSeeds(&out, metadata, summary, facts, projection)
	p.memorySeeds[item] = out.PriorityFactSeeds
	return out.PriorityFactSeeds
}

func prepareTurnProjectionQuerySet(selection prepareTurnMemorySelectionContext, fallback string) []string {
	if strings.TrimSpace(selection.Query) == "" {
		return []string{fallback}
	}
	return prepareTurnPriorityQuerySetFromAny(selection.QuerySet)
}

// These fragments contain source preparation only. Their order is the original
// source traversal order, so score ties and source occurrence identities do not
// change when supplemental questions reuse them. Lifetime: one prepare request.
type prepareTurnFactFragment struct {
	seeds    []prepareTurnPriorityFactSeed
	metadata []prepareTurnPrioritySourceMetadata
}

type prepareTurnFactFragmentOffset struct{ seeds, metadata int }

func prepareTurnFactFragmentStart(out *prepareTurnInjectionAssembly) prepareTurnFactFragmentOffset {
	return prepareTurnFactFragmentOffset{len(out.PriorityFactSeeds), len(out.PrioritySourceMetadata)}
}

func (start prepareTurnFactFragmentOffset) capture(out *prepareTurnInjectionAssembly) prepareTurnFactFragment {
	return prepareTurnFactFragment{out.PriorityFactSeeds[start.seeds:], out.PrioritySourceMetadata[start.metadata:]}
}

func (fragment prepareTurnFactFragment) appendTo(out *prepareTurnInjectionAssembly) {
	out.PriorityFactSeeds = append(out.PriorityFactSeeds, fragment.seeds...)
	out.PrioritySourceMetadata = append(out.PrioritySourceMetadata, fragment.metadata...)
}

func clonePrepareTurnProjectionCounts(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
