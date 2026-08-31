package httpapi

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type completeTurnLLMConfig struct {
	APIKey                string
	Endpoint              string
	Model                 string
	Provider              string
	Source                string
	TimeoutMs             int64
	Temperature           float64
	MaxTokens             int64
	MaxCompletionTokens   int64
	ReasoningPreset       string
	ReasoningEffort       string
	ReasoningBudgetTokens int64
	GlmThinkingType       string
	ExtraHeadersJSON      string
	ExtraBodyJSON         string
	VertexFlexMode        string
	LLMGatewayServiceTier string
	ClaudePromptCacheMode string
	ForceWorldRuleAudit   bool
	RetryBudget           *llmRetryBudget
}

type completeTurnEmbeddingConfig struct {
	APIKey    string
	Endpoint  string
	Model     string
	Provider  string
	TimeoutMs int64
	Source    string
}

type completeTurnExtractionConfig struct {
	Critic   completeTurnLLMConfig
	Embedder completeTurnEmbeddingConfig
}

type artifactSaveResult struct {
	Memories                 int
	Evidence                 int
	KGTriples                int
	PersonaCapsuleCandidates int
	SubjectiveEntityMemories int
	CharacterEvents          int
	Storylines               int
	WorldRules               int
	CharacterStates          int
	PhysicalConditions       int
	EntityConditions         int
	StatusSchemaDefinitions  int
	StatusEffects            int
	NarrativeCurrentStates   int
	NarrativeStateEvents     int
	RelationCurrentStates    int
	RelationStateEvents      int
	HabitEvidenceCurrent     int
	HabitEvidenceEvents      int
	CharacterProfiles        int
	VoiceBehaviorProjections int
	PendingThreads           int
	ActiveStates             int
	Entities                 int
	EntityIdentities         int
	IdentitySurfaces         int
	EntityIdentityLinks      int
	IdentityBindings         int
	SpeakerAttributions      int
	PreciseMemoryUnits       int
	TrustStates              int
	VectorsUpserted          int
	VectorsMemoryUpserted    int
	VectorsEvidenceUpserted  int
	VectorsWorldRuleUpserted int
	EmbeddingStatus          string
	VectorStatus             string
	Attempted                int
	Errors                   int
	ErrorDetails             []string
	Warnings                 []string
	SkipReasons              []map[string]any
	ConflictResolutions      []map[string]any
	RetentionDecisions       []map[string]any
	CanonicalStateLayers     int
	CanonicalStateWriteCost  *canonicalStateWriteCostMeasurement
	TimingMS                 map[string]float64
}

func (r *artifactSaveResult) addTiming(stage string, startedAt time.Time) {
	if r == nil || stage == "" || startedAt.IsZero() {
		return
	}
	if r.TimingMS == nil {
		r.TimingMS = map[string]float64{}
	}
	r.TimingMS[stage] = roundMilliseconds(r.TimingMS[stage] + durationMilliseconds(time.Since(startedAt)))
}

type canonicalStateWriteCostMeasurement struct {
	PolicyVersion     string           `json:"policy_version"`
	StateWriteCount   int              `json:"state_write_count"`
	DeltaUpdateCount  int              `json:"delta_update_count"`
	FullRewriteCount  int              `json:"full_rewrite_count"`
	FallbackCount     int              `json:"fallback_count"`
	AvgWriteLatencyMs float64          `json:"avg_write_latency_ms"`
	P95WriteLatencyMs float64          `json:"p95_write_latency_ms"`
	TotalWriteChars   int              `json:"total_write_chars"`
	TotalElapsedMs    int64            `json:"total_elapsed_ms"`
	Items             []map[string]any `json:"items"`
}

const (
	conflictClassStateTransition    = "state_transition"
	conflictClassHardContradiction  = "hard_contradiction"
	conflictClassParallelContext    = "parallel_context"
	conflictClassLowConfidenceNoise = "low_confidence_noise"
)

const (
	conflictRouteSuperseded   = "superseded"
	conflictRouteTombstone    = "tombstone"
	conflictRouteHold         = "hold"
	conflictRouteManualReview = "manual_review"
)

func classifyConflict(incomingText string, existing store.DirectEvidence) string {
	incomingText = strings.ToLower(strings.TrimSpace(incomingText))
	existingText := strings.ToLower(strings.TrimSpace(existing.EvidenceText))
	if incomingText == "" || existingText == "" {
		return conflictClassLowConfidenceNoise
	}
	if incomingText == existingText {
		return conflictClassStateTransition
	}
	if hasOpposedConflictSignal(incomingText, existingText) {
		return conflictClassHardContradiction
	}
	inWords := strings.Fields(incomingText)
	exWords := strings.Fields(existingText)
	shared := 0
	exSet := map[string]bool{}
	for _, w := range exWords {
		if len(w) > 2 {
			exSet[w] = true
		}
	}
	for _, w := range inWords {
		if len(w) > 2 && exSet[w] {
			shared++
		}
	}
	shorterLen := len(inWords)
	if len(exWords) < shorterLen {
		shorterLen = len(exWords)
	}
	if shorterLen > 0 && float64(shared)/float64(shorterLen) >= 0.75 {
		return conflictClassHardContradiction
	}
	if shared > 0 {
		return conflictClassParallelContext
	}
	return conflictClassLowConfidenceNoise
}

func hasOpposedConflictSignal(incomingText, existingText string) bool {
	positive := []string{"love", "loves", "trust", "trusts", "ally", "friend", "friends"}
	negative := []string{"hate", "hates", "distrust", "distrusts", "betray", "betrayed", "betrayal", "enemy", "enemies", "no longer"}
	return (containsAnyTerm(incomingText, negative) && containsAnyTerm(existingText, positive)) ||
		(containsAnyTerm(incomingText, positive) && containsAnyTerm(existingText, negative))
}

func containsAnyTerm(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func buildConflictConfidencePolicy(confidence float64, fieldClass string) map[string]any {
	threshold := 0.75
	switch fieldClass {
	case "relationship", "trust", "identity":
		threshold = 0.85
	case "world_rule", "lore":
		threshold = 0.90
	case "location", "item":
		threshold = 0.65
	}
	return map[string]any{
		"threshold":    threshold,
		"field_class":  fieldClass,
		"confidence":   confidence,
		"auto_promote": confidence >= threshold,
		"repair_queue": confidence >= 0.5 && confidence < threshold,
		"hold":         confidence >= 0.3 && confidence < 0.5,
		"user_confirm": confidence < 0.3,
		"version":      "ea1i.v1",
	}
}

func resolveCanonicalConflict(incoming store.DirectEvidence, existing []store.DirectEvidence) []map[string]any {
	results := []map[string]any{}
	if strings.TrimSpace(incoming.EvidenceText) == "" {
		return results
	}
	fieldClass := "general"
	lower := strings.ToLower(incoming.EvidenceText)
	if strings.Contains(lower, "trust") || strings.Contains(lower, "love") || strings.Contains(lower, "hate") || strings.Contains(lower, "friend") {
		fieldClass = "relationship"
	} else if strings.Contains(lower, "rule") || strings.Contains(lower, "world") || strings.Contains(lower, "law") {
		fieldClass = "world_rule"
	}
	for _, ex := range existing {
		if ex.ID == incoming.ID || ex.Tombstoned {
			continue
		}
		cls := classifyConflict(incoming.EvidenceText, ex)
		confidence := 0.6
		switch ex.CaptureVerification {
		case "verified":
			confidence = 0.9
		case "rejected":
			confidence = 0.3
		}
		policy := buildConflictConfidencePolicy(confidence, fieldClass)
		routing := conflictRouteHold
		switch cls {
		case conflictClassStateTransition:
			routing = conflictRouteSuperseded
		case conflictClassHardContradiction:
			if policy["auto_promote"].(bool) {
				routing = conflictRouteTombstone
			} else {
				routing = conflictRouteManualReview
			}
		case conflictClassParallelContext, conflictClassLowConfidenceNoise:
			routing = conflictRouteHold
		}
		results = append(results, map[string]any{
			"classification": cls,
			"routing":        routing,
			"confidence":     confidence,
			"field_class":    fieldClass,
			"reason":         fmt.Sprintf("existing_id=%d verification=%s", ex.ID, ex.CaptureVerification),
			"target_id":      ex.ID,
			"policy":         policy,
		})
	}
	return results
}

func applyRetentionPolicy(evidence *store.DirectEvidence, importance float64, existing []store.DirectEvidence) map[string]any {
	decision := map[string]any{
		"action":        "preserve",
		"archive_state": "canonical_direct",
		"importance":    importance,
		"reason":        "direct_evidence_lifecycle_and_lineage",
		"ttl_turns":     0,
		"version":       "ea1l.v1",
	}
	if evidence.Tombstoned {
		decision["archive_state"] = "tombstone_audit"
		decision["reason"] = "tombstone_preserve_for_audit_without_turn_expiry"
		return decision
	}
	for _, ex := range existing {
		if ex.SupersededByID == evidence.ID || ex.ID == evidence.SupersededByID {
			decision["archive_state"] = "superseded_archive"
			decision["reason"] = "superseded_lineage_preserve_without_turn_expiry"
			break
		}
	}
	return decision
}

type storylineSaver interface {
	SaveStoryline(ctx context.Context, item *store.Storyline) error
}

type worldRuleSaver interface {
	SaveWorldRule(ctx context.Context, item *store.WorldRule) error
}

type characterStateSaver interface {
	SaveCharacterState(ctx context.Context, item *store.CharacterState) error
}

type pendingThreadSaver interface {
	SavePendingThread(ctx context.Context, item *store.PendingThread) error
}

type activeStateSaver interface {
	SaveActiveState(ctx context.Context, item *store.ActiveState) error
}

type canonicalStateLayerSaver interface {
	SaveCanonicalStateLayer(ctx context.Context, item *store.CanonicalStateLayer) error
}

type entitySaver interface {
	SaveEntity(ctx context.Context, item *store.Entity) error
}

type trustSaver interface {
	SaveTrust(ctx context.Context, item *store.Trust) error
}

type memoryImportanceUpdater interface {
	UpdateMemoryImportance(ctx context.Context, chatSessionID string, memoryID int64, importance float64) error
}

var placeholderKGPartPattern = regexp.MustCompile(`(?i)^\s*(?:char_\d+(?:_cid_[a-f0-9-]{8,})?|cid_[a-f0-9-]{8,}|turn_\d+|\{\{\s*(?:user|char)\s*\}\}|<\s*(?:user|char)\s*>)\s*$`)
var closedThoughtTagPattern = regexp.MustCompile(`(?is)<\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\b[^>]*>.*?<\s*/\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\s*>`)
var openThoughtTagPattern = regexp.MustCompile(`(?is)<\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\b[^>]*>.*$`)
var filterCompleteMarkerPattern = regexp.MustCompile(`(?is)<\s*__filter_complete__\s*>`)
var thoughtLinePrefixPattern = regexp.MustCompile(`(?im)^\s*(?:chain of thought|hidden chain-of-thought|thought process|thinking|analysis|reasoning|scratchpad)\s*:\s*.*(?:\r?\n|$)`)

const risuChatMessageObservationContract = "risu_chat_message_observation.v1"

func sanitizeTextForCriticInput(text string) string {
	return sanitizeCriticStorageText(text)
}

func boundCompleteTurnCriticInput(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return strings.TrimSpace(text)
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	head := maxRunes * 2 / 3
	if head < 1 {
		head = maxRunes / 2
	}
	tail := maxRunes - head
	if tail < 1 {
		tail = 1
	}
	omitted := len(runes) - head - tail
	if omitted < 0 {
		omitted = 0
	}
	return strings.TrimSpace(string(runes[:head])) +
		fmt.Sprintf("\n\n[... %d chars omitted for critic input budget; raw turn is stored verbatim ...]\n\n", omitted) +
		strings.TrimSpace(string(runes[len(runes)-tail:]))
}

func sanitizeContextMessagesForCriticInput(messages []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, item := range messages {
		role := strings.ToLower(strings.TrimSpace(stringFromMap(item, "role")))
		if role != "user" && role != "assistant" {
			continue
		}
		contractVersion := strings.TrimSpace(stringFromMap(item, "contract_version"))
		if contractVersion != "" && (contractVersion != risuChatMessageObservationContract ||
			!strings.EqualFold(strings.TrimSpace(stringFromMap(item, "observation_state")), "observed") ||
			!strings.EqualFold(strings.TrimSpace(stringFromMap(item, "source_kind")), "active_chat_message")) {
			continue
		}
		copied := map[string]any{"role": role}
		if content, ok := item["content"].(string); ok {
			cleaned := sanitizeTextForCriticInput(content)
			if cleaned == "" && strings.TrimSpace(content) != "" {
				continue
			}
			copied["content"] = cleaned
		}
		if contractVersion == risuChatMessageObservationContract {
			copied["contract_version"] = risuChatMessageObservationContract
			copied["observation_state"] = "observed"
			copied["source_kind"] = "active_chat_message"
		}
		out = append(out, copied)
	}
	return out
}

func (s *Server) canonicalCharacterName(ctx context.Context, sid, proposed string) string {
	proposed = strings.TrimSpace(proposed)
	if proposed == "" || s.Store == nil {
		return proposed
	}
	if resolver, ok := s.Store.(store.UniqueActiveEntitySurfaceIdentityResolver); ok {
		resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(proposed))
		if err == nil {
			if label := strings.TrimSpace(resolved.CanonicalLabel); label != "" {
				return label
			}
		}
		if errors.Is(err, store.ErrReviewedEntityIdentityAmbiguous) {
			return proposed
		}
	}
	states, err := s.Store.ListCharacterStates(ctx, sid)
	if err != nil || len(states) == 0 {
		return proposed
	}
	proposedKey := comparableEntityKey(proposed)
	if proposedKey == "" {
		return proposed
	}
	matched := ""
	for _, state := range states {
		candidate := strings.TrimSpace(state.CharacterName)
		if candidate == "" || comparableEntityKey(candidate) != proposedKey {
			continue
		}
		if matched != "" && matched != candidate {
			return proposed
		}
		matched = candidate
	}
	if matched != "" {
		return matched
	}
	return proposed
}

func canonicalCharacterAliasKey(name string) string {
	return normalizeCharacterKey(name)
}

var koreanInitialRoman = []string{"g", "kk", "n", "d", "tt", "r", "m", "b", "pp", "s", "ss", "", "j", "jj", "ch", "k", "t", "p", "h"}
var koreanMedialRoman = []string{"a", "ae", "ya", "yae", "eo", "e", "yeo", "ye", "o", "wa", "wae", "oe", "yo", "u", "wo", "we", "wi", "yu", "eu", "ui", "i"}
var koreanFinalRoman = []string{"", "k", "k", "ks", "n", "nj", "nh", "t", "l", "lk", "lm", "lb", "ls", "lt", "lp", "lh", "m", "p", "ps", "t", "t", "ng", "t", "t", "k", "t", "p", "h"}

var katakanaRoman = map[string]string{
	"ア": "a", "イ": "i", "ウ": "u", "エ": "e", "オ": "o",
	"カ": "ka", "キ": "ki", "ク": "ku", "ケ": "ke", "コ": "ko",
	"サ": "sa", "シ": "shi", "ス": "su", "セ": "se", "ソ": "so",
	"タ": "ta", "チ": "chi", "ツ": "tsu", "テ": "te", "ト": "to",
	"ナ": "na", "ニ": "ni", "ヌ": "nu", "ネ": "ne", "ノ": "no",
	"ハ": "ha", "ヒ": "hi", "フ": "fu", "ヘ": "he", "ホ": "ho",
	"マ": "ma", "ミ": "mi", "ム": "mu", "メ": "me", "モ": "mo",
	"ヤ": "ya", "ユ": "yu", "ヨ": "yo",
	"ラ": "ra", "リ": "ri", "ル": "ru", "レ": "re", "ロ": "ro",
	"ワ": "wa", "ヲ": "wo", "ン": "n",
	"ガ": "ga", "ギ": "gi", "グ": "gu", "ゲ": "ge", "ゴ": "go",
	"ザ": "za", "ジ": "ji", "ズ": "zu", "ゼ": "ze", "ゾ": "zo",
	"ダ": "da", "ヂ": "di", "ヅ": "du", "デ": "de", "ド": "do",
	"バ": "ba", "ビ": "bi", "ブ": "bu", "ベ": "be", "ボ": "bo",
	"パ": "pa", "ピ": "pi", "プ": "pu", "ペ": "pe", "ポ": "po",
	"キャ": "kya", "キュ": "kyu", "キョ": "kyo",
	"シャ": "sha", "シュ": "shu", "ショ": "sho",
	"チャ": "cha", "チュ": "chu", "チョ": "cho",
	"ニャ": "nya", "ニュ": "nyu", "ニョ": "nyo",
	"ヒャ": "hya", "ヒュ": "hyu", "ヒョ": "hyo",
	"ミャ": "mya", "ミュ": "myu", "ミョ": "myo",
	"リャ": "rya", "リュ": "ryu", "リョ": "ryo",
	"ギャ": "gya", "ギュ": "gyu", "ギョ": "gyo",
	"ジャ": "ja", "ジュ": "ju", "ジョ": "jo",
	"ビャ": "bya", "ビュ": "byu", "ビョ": "byo",
	"ピャ": "pya", "ピュ": "pyu", "ピョ": "pyo",
	"ヴァ": "va", "ヴィ": "vi", "ヴ": "vu", "ヴェ": "ve", "ヴォ": "vo",
	"ファ": "fa", "フィ": "fi", "フェ": "fe", "フォ": "fo",
	"ティ": "ti", "ディ": "di",
}

func normalizeCharacterKey(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if containsKatakana(name) && !containsKorean(name) {
		return normalizeKatakanaMixedKey(name)
	}
	var b strings.Builder
	hasKorean := false
	for _, r := range name {
		if r >= 0xAC00 && r <= 0xD7A3 {
			hasKorean = true
			idx := int(r - 0xAC00)
			b.WriteString(koreanInitialRoman[idx/588])
			b.WriteString(koreanMedialRoman[(idx%588)/28])
			b.WriteString(koreanFinalRoman[idx%28])
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		} else if r >= 0x4e00 && r <= 0x9fff {
			b.WriteRune(r)
		} else if !hasKorean && r >= 0x30a1 && r <= 0x30ff {
			b.WriteString(romanizeKatakanaRune(r))
		}
	}
	return b.String()
}

func containsKatakana(text string) bool {
	for _, r := range text {
		if r >= 0x30a1 && r <= 0x30ff {
			return true
		}
	}
	return false
}

func normalizeKatakanaMixedKey(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if i+1 < len(runes) {
			pair := string([]rune{r, runes[i+1]})
			if v, ok := katakanaRoman[pair]; ok {
				b.WriteString(v)
				i++
				continue
			}
		}
		if r == 'ー' || r == 'ッ' {
			continue
		}
		if v, ok := katakanaRoman[string(r)]; ok {
			b.WriteString(v)
		} else if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		} else if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r >= 0x4e00 && r <= 0x9fff {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func romanizeKatakanaRune(r rune) string {
	if r == 'ー' || r == 'ッ' {
		return ""
	}
	if v, ok := katakanaRoman[string(r)]; ok {
		return v
	}
	return ""
}

func containsKorean(text string) bool {
	for _, r := range text {
		if (r >= 0xAC00 && r <= 0xD7A3) || (r >= 0x1100 && r <= 0x11FF) {
			return true
		}
	}
	return false
}

func hasMeaningfulPayload(raw any) bool {
	switch v := raw.(type) {
	case nil:
		return false
	case string:
		text := strings.TrimSpace(v)
		return text != "" && text != "{}" && text != "[]" && text != "null"
	case []any:
		for _, item := range v {
			if hasMeaningfulPayload(item) {
				return true
			}
		}
		return false
	case map[string]any:
		for _, item := range v {
			if hasMeaningfulPayload(item) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func completeTurnExtractionConfigFromMeta(meta map[string]any) completeTurnExtractionConfig {
	criticMap := mapFromAny(meta["critic"])
	embeddingMap := mapFromAny(meta["embedding"])
	return completeTurnExtractionConfig{
		Critic: completeTurnLLMConfig{
			APIKey:                stringFromMap(criticMap, "api_key"),
			Endpoint:              stringFromMap(criticMap, "endpoint"),
			Model:                 stringFromMap(criticMap, "model"),
			Provider:              stringFromMap(criticMap, "provider"),
			TimeoutMs:             int64FromMap(criticMap, "timeout_ms", 0),
			Temperature:           floatFromMap(criticMap, "temperature", 0.2),
			MaxTokens:             int64FromMap(criticMap, "max_tokens", 30000),
			MaxCompletionTokens:   int64FromMap(criticMap, "max_completion_tokens", 30000),
			ReasoningPreset:       stringFromMap(criticMap, "reasoning_preset"),
			ReasoningEffort:       stringFromMap(criticMap, "reasoning_effort"),
			ReasoningBudgetTokens: int64FromMap(criticMap, "reasoning_budget_tokens", 0),
			GlmThinkingType:       stringFromMap(criticMap, "glm_thinking_type"),
			ExtraHeadersJSON:      stringFromMap(criticMap, "extra_headers_json"),
			ExtraBodyJSON:         stringFromMap(criticMap, "extra_body_json"),
			VertexFlexMode:        stringFromMap(criticMap, "vertex_flex_mode"),
			LLMGatewayServiceTier: stringFromMap(criticMap, "llm_gateway_service_tier"),
			ClaudePromptCacheMode: stringFromMap(criticMap, "claude_prompt_cache_mode"),
			ForceWorldRuleAudit:   boolFromAny(meta["force_world_rule_backfill"]) || boolFromAny(meta["force_focused_world_rule_audit"]),
		},
		Embedder: completeTurnEmbeddingConfig{
			APIKey:    stringFromMap(embeddingMap, "api_key"),
			Endpoint:  stringFromMap(embeddingMap, "endpoint"),
			Model:     stringFromMap(embeddingMap, "model"),
			Provider:  stringFromMap(embeddingMap, "provider"),
			TimeoutMs: int64FromMap(embeddingMap, "timeout_ms", 0),
		},
	}
}

func (s *Server) completeTurnExtractionConfig(meta map[string]any) completeTurnExtractionConfig {
	cfg := completeTurnExtractionConfigFromMeta(meta)
	rt := s.runtimeConfigSnapshot()
	criticMap := mapFromAny(meta["critic"])

	cfg.Critic = selectCompleteTurnLLMCoreConfig(cfg.Critic, criticMap, rt.CriticProvider, rt.CriticAPIKey, rt.CriticEndpoint, rt.CriticModel, "critic")
	if rt.CriticTimeoutSec > 0 {
		cfg.Critic.TimeoutMs = runtimeTimeoutMs(rt.CriticTimeoutSec)
	}
	if rt.CriticTemperature != nil && criticMap["temperature"] == nil {
		cfg.Critic.Temperature = *rt.CriticTemperature
	}
	if rt.CriticMaxTokens != nil && *rt.CriticMaxTokens > 0 {
		if criticMap["max_completion_tokens"] == nil {
			cfg.Critic.MaxCompletionTokens = *rt.CriticMaxTokens
		}
		if criticMap["max_tokens"] == nil {
			cfg.Critic.MaxTokens = *rt.CriticMaxTokens
		}
	}
	if strings.TrimSpace(cfg.Critic.ReasoningPreset) == "" {
		cfg.Critic.ReasoningPreset = rt.CriticReasoningPreset
	}
	if strings.TrimSpace(cfg.Critic.ReasoningEffort) == "" {
		cfg.Critic.ReasoningEffort = rt.CriticReasoningEffort
	}
	if cfg.Critic.ReasoningBudgetTokens <= 0 && rt.CriticReasoningBudget != nil {
		cfg.Critic.ReasoningBudgetTokens = *rt.CriticReasoningBudget
	}
	if strings.TrimSpace(cfg.Critic.GlmThinkingType) == "" {
		cfg.Critic.GlmThinkingType = glmThinkingTypeFromReasoning(cfg.Critic.ReasoningPreset, cfg.Critic.ReasoningEffort)
	}
	if strings.TrimSpace(cfg.Critic.ExtraHeadersJSON) == "" {
		cfg.Critic.ExtraHeadersJSON = rt.CriticExtraHeadersJSON
	}
	if strings.TrimSpace(cfg.Critic.ExtraBodyJSON) == "" {
		cfg.Critic.ExtraBodyJSON = rt.CriticExtraBodyJSON
	}
	if strings.TrimSpace(cfg.Critic.VertexFlexMode) == "" {
		cfg.Critic.VertexFlexMode = rt.CriticVertexFlexMode
	}
	if strings.TrimSpace(cfg.Critic.LLMGatewayServiceTier) == "" {
		cfg.Critic.LLMGatewayServiceTier = rt.CriticLLMGatewayServiceTier
	}
	if strings.TrimSpace(cfg.Critic.ClaudePromptCacheMode) == "" {
		cfg.Critic.ClaudePromptCacheMode = rt.CriticClaudePromptCacheMode
	}
	cfg.Critic.RetryBudget = newLLMRetryBudget(rt.LLMRetryCount)

	cfg.Embedder = s.selectCompleteTurnEmbeddingConfig(meta, cfg.Embedder, rt)
	return cfg
}

func selectCompleteTurnLLMCoreConfig(metaCfg completeTurnLLMConfig, metaMap map[string]any, runtimeProvider, runtimeAPIKey, runtimeEndpoint, runtimeModel, label string) completeTurnLLMConfig {
	metaCfg.APIKey = normalizeConfigSecret(metaCfg.APIKey)
	metaCfg.Endpoint = proxyProviderBaseURL(metaCfg.Provider, metaCfg.Endpoint)
	if len(metaMap) > 0 && metaCfg.hasAnyAuthorityConfigField() {
		metaCfg.Source = "client_meta." + label
		if !metaCfg.hasConfig() {
			metaCfg.Source = "client_meta_partial." + label
		}
		return metaCfg
	}
	runtimeCfg := metaCfg
	runtimeCfg.Provider = strings.TrimSpace(runtimeProvider)
	runtimeCfg.APIKey = normalizeConfigSecret(runtimeAPIKey)
	runtimeCfg.Endpoint = proxyProviderBaseURL(runtimeCfg.Provider, runtimeEndpoint)
	runtimeCfg.Model = strings.TrimSpace(runtimeModel)
	runtimeCfg.Source = "runtime_config." + label
	if runtimeCfg.hasAnyAuthorityConfigField() {
		if !runtimeCfg.hasConfig() {
			runtimeCfg.Source = "runtime_config_partial." + label
		}
		return runtimeCfg
	}
	metaCfg.Source = "missing." + label
	return metaCfg
}

func (s *Server) selectCompleteTurnEmbeddingConfig(meta map[string]any, metaCfg completeTurnEmbeddingConfig, rt RuntimeConfig) completeTurnEmbeddingConfig {
	metaCfg.APIKey = normalizeConfigSecret(metaCfg.APIKey)
	timeoutMs := metaCfg.TimeoutMs
	if rt.EmbeddingTimeoutSec > 0 {
		timeoutMs = runtimeTimeoutMs(rt.EmbeddingTimeoutSec)
	}
	metaCfg.TimeoutMs = timeoutMs
	metaEmbedding := mapFromAny(meta["embedding"])
	if len(metaEmbedding) > 0 && metaCfg.hasAnyAuthorityConfigField() {
		metaCfg.Source = "client_meta"
		if !metaCfg.hasConfig() {
			metaCfg.Source = "client_meta_partial"
		}
		return metaCfg
	}

	runtimeCfg := completeTurnEmbeddingConfig{
		Provider:  strings.TrimSpace(rt.EmbeddingProvider),
		APIKey:    normalizeConfigSecret(rt.EmbeddingAPIKey),
		Endpoint:  strings.TrimSpace(rt.EmbeddingEndpoint),
		Model:     strings.TrimSpace(rt.EmbeddingModel),
		TimeoutMs: timeoutMs,
		Source:    "runtime_config",
	}
	if runtimeCfg.hasAnyAuthorityConfigField() {
		if !runtimeCfg.hasConfig() {
			runtimeCfg.Source = "runtime_config_partial"
		}
		return runtimeCfg
	}

	envCfg := completeTurnEmbeddingConfig{
		Provider: strings.TrimSpace(extractionFirstNonEmpty(
			s.Cfg.EmbedderProvider,
			embeddingEnvFirst("AC_EMBEDDER_PROVIDER", "AC_LT_EMBEDDING_PROVIDER", "PROJECT_EMBEDDING_PROVIDER", "AC_PROJECT_EMBEDDING_PROVIDER"),
		)),
		APIKey: normalizeConfigSecret(embeddingEnvFirst("AC_EMBEDDER_API_KEY", "AC_LT_EMBEDDING_API_KEY", "PROJECT_EMBEDDING_API_KEY", "AC_PROJECT_EMBEDDING_API_KEY")),
		Endpoint: strings.TrimSpace(extractionFirstNonEmpty(
			s.Cfg.EmbedderEndpoint,
			embeddingEnvFirst("AC_EMBEDDER_ENDPOINT", "AC_LT_EMBEDDING_ENDPOINT", "PROJECT_EMBEDDING_ENDPOINT", "AC_PROJECT_EMBEDDING_ENDPOINT"),
		)),
		Model: strings.TrimSpace(extractionFirstNonEmpty(
			s.Cfg.EmbedderModel,
			embeddingEnvFirst("AC_EMBEDDER_MODEL", "AC_LT_EMBEDDING_MODEL", "PROJECT_EMBEDDING_MODEL", "AC_PROJECT_EMBEDDING_MODEL"),
		)),
		TimeoutMs: timeoutMs,
		Source:    "env_or_config",
	}
	if !envCfg.hasConfig() {
		envCfg.Source = "missing"
	}
	return envCfg
}

func (c completeTurnLLMConfig) hasConfig() bool {
	return len(c.missingFields()) == 0
}

func (c completeTurnLLMConfig) missingFields() []string {
	missing := configMissingFieldsWithProvider(c.Provider, c.APIKey, c.Endpoint, c.Model)
	if c.TimeoutMs <= 0 {
		missing = append(missing, "timeout_ms")
	}
	return missing
}

func (c completeTurnLLMConfig) hasAnyAuthorityConfigField() bool {
	return strings.TrimSpace(normalizeConfigSecret(c.APIKey)) != "" ||
		strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Model) != ""
}

func (c completeTurnEmbeddingConfig) hasConfig() bool {
	return len(c.missingFields()) == 0
}

func (c completeTurnEmbeddingConfig) missingFields() []string {
	missing := configMissingFieldsWithProvider(c.Provider, c.APIKey, c.Endpoint, c.Model)
	if c.TimeoutMs <= 0 {
		missing = append(missing, "timeout_ms")
	}
	return missing
}

func (c completeTurnEmbeddingConfig) hasAnyConfigField() bool {
	return strings.TrimSpace(c.Provider) != "" ||
		strings.TrimSpace(normalizeConfigSecret(c.APIKey)) != "" ||
		strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Model) != ""
}

func (c completeTurnEmbeddingConfig) hasAnyAuthorityConfigField() bool {
	return strings.TrimSpace(normalizeConfigSecret(c.APIKey)) != "" ||
		strings.TrimSpace(c.Endpoint) != "" ||
		strings.TrimSpace(c.Model) != ""
}

func normalizeConfigSecret(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || isMaskedSecretValue(v) {
		return ""
	}
	return v
}

func isMaskedSecretValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if strings.Contains(v, "•") || strings.Contains(v, "●") {
		return true
	}
	allMask := true
	for _, r := range v {
		if r != '*' && r != '＊' {
			allMask = false
			break
		}
	}
	return allMask && len([]rune(v)) >= 4
}

func completeTurnLLMConfigTrace(cfg completeTurnExtractionConfig) map[string]any {
	criticTrace := map[string]any{
		"configured":     cfg.Critic.hasConfig(),
		"source":         strings.TrimSpace(cfg.Critic.Source),
		"provider":       strings.TrimSpace(cfg.Critic.Provider),
		"endpoint_host":  endpointHost(cfg.Critic.Endpoint),
		"model":          strings.TrimSpace(cfg.Critic.Model),
		"timeout_ms":     cfg.Critic.TimeoutMs,
		"missing_fields": cfg.Critic.missingFields(),
	}
	addCompleteTurnReasoningTraceFields(criticTrace, cfg.Critic)
	return map[string]any{
		"critic": criticTrace,
		"embedding": map[string]any{
			"configured":     cfg.Embedder.hasConfig(),
			"source":         strings.TrimSpace(cfg.Embedder.Source),
			"provider":       strings.TrimSpace(cfg.Embedder.Provider),
			"endpoint_host":  endpointHost(cfg.Embedder.Endpoint),
			"model":          strings.TrimSpace(cfg.Embedder.Model),
			"timeout_ms":     cfg.Embedder.TimeoutMs,
			"missing_fields": cfg.Embedder.missingFields(),
		},
	}
}

func addCompleteTurnReasoningTraceFields(trace map[string]any, cfg completeTurnLLMConfig) {
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		trace["reasoning_preset"] = strings.TrimSpace(cfg.ReasoningPreset)
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		trace["reasoning_effort"] = strings.TrimSpace(cfg.ReasoningEffort)
	}
	if cfg.ReasoningBudgetTokens > 0 {
		trace["reasoning_budget_tokens"] = cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		trace["glm_thinking_type"] = strings.TrimSpace(cfg.GlmThinkingType)
	}
	if strings.TrimSpace(cfg.VertexFlexMode) != "" {
		trace["vertex_flex_mode"] = strings.TrimSpace(cfg.VertexFlexMode)
	}
	if strings.TrimSpace(cfg.LLMGatewayServiceTier) != "" {
		trace["llm_gateway_service_tier"] = strings.TrimSpace(cfg.LLMGatewayServiceTier)
	}
	if strings.TrimSpace(cfg.ClaudePromptCacheMode) != "" {
		trace["claude_prompt_cache_mode"] = strings.TrimSpace(cfg.ClaudePromptCacheMode)
	}
	if strings.TrimSpace(cfg.ExtraHeadersJSON) != "" {
		trace["extra_headers_json_configured"] = true
	}
	if strings.TrimSpace(cfg.ExtraBodyJSON) != "" {
		trace["extra_body_json_configured"] = true
	}
}

func applyProxyOverridesFromLLMConfig(req *dto.ProxyPluginMainRequest, cfg completeTurnLLMConfig) {
	if req == nil {
		return
	}
	if strings.TrimSpace(cfg.ExtraHeadersJSON) != "" {
		req.ExtraHeadersJSON = &cfg.ExtraHeadersJSON
	}
	if strings.TrimSpace(cfg.ExtraBodyJSON) != "" {
		req.ExtraBodyJSON = &cfg.ExtraBodyJSON
	}
	if strings.TrimSpace(cfg.VertexFlexMode) != "" {
		req.VertexFlexMode = &cfg.VertexFlexMode
	}
	if strings.TrimSpace(cfg.LLMGatewayServiceTier) != "" {
		req.LLMGatewayServiceTier = &cfg.LLMGatewayServiceTier
	}
	if strings.TrimSpace(cfg.ClaudePromptCacheMode) != "" {
		req.ClaudePromptCacheMode = &cfg.ClaudePromptCacheMode
	}
}

func glmThinkingTypeFromReasoning(preset, effort string) string {
	if !strings.EqualFold(strings.TrimSpace(preset), "glm") {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "enable", "enabled", "on", "true", "minimal", "low", "medium", "high", "xhigh", "max":
		return "enabled"
	case "none", "disable", "disabled", "off", "false":
		return "disabled"
	default:
		return ""
	}
}
