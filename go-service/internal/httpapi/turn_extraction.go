package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type completeTurnLLMConfig struct {
	APIKey                string
	Endpoint              string
	Model                 string
	Provider              string
	TimeoutMs             int64
	Temperature           float64
	MaxTokens             int64
	MaxCompletionTokens   int64
	ReasoningPreset       string
	ReasoningEffort       string
	ReasoningBudgetTokens int64
	GlmThinkingType       string
	ForceWorldRuleAudit   bool
}

type completeTurnEmbeddingConfig struct {
	APIKey    string
	Endpoint  string
	Model     string
	Provider  string
	TimeoutMs int64
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
	PendingThreads           int
	ActiveStates             int
	Entities                 int
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
		"archive_state": "canonical",
		"importance":    importance,
		"reason":        "direct_evidence_lineage",
		"ttl_turns":     0,
		"version":       "ea1l.v1",
	}
	if importance >= 0.8 {
		decision["archive_state"] = "canonical_direct"
		return decision
	}
	if importance >= 0.5 {
		decision["ttl_turns"] = 120
		decision["archive_state"] = "previous_archive"
		return decision
	}
	if evidence.Tombstoned {
		decision["ttl_turns"] = 240
		decision["archive_state"] = "tombstone_audit"
		decision["reason"] = "tombstone_preserve_for_audit"
		return decision
	}
	decision["ttl_turns"] = 30
	decision["archive_state"] = "transient"
	decision["reason"] = "low_importance_noise"
	for _, ex := range existing {
		if ex.SupersededByID == evidence.ID || ex.ID == evidence.SupersededByID {
			decision["archive_state"] = "superseded_archive"
			decision["ttl_turns"] = 60
			decision["reason"] = "superseded_lineage_preserve"
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

var placeholderKGPartPattern = regexp.MustCompile(`(?i)^\s*(?:char_\d+(?:_cid_[a-f0-9-]{8,})?|cid_[a-f0-9-]{8,}|turn_\d+|\{\{\s*(?:user|char)\s*\}\}|<\s*(?:user|char)\s*>|user|유저|사용자|ユーザー|player|플레이어|プレイヤー|participant|참가자|assistant|어시스턴트|system|시스템|developer|개발자|prompt|instruction|bot|agent)\s*$`)
var jsonTrailingCommaPattern = regexp.MustCompile(`,\s*([}\]])`)
var closedThoughtTagPattern = regexp.MustCompile(`(?is)<\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\b[^>]*>.*?<\s*/\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\s*>`)
var openThoughtTagPattern = regexp.MustCompile(`(?is)<\s*(?:thoughts|thinking|analysis|reasoning|scratchpad|filter)\b[^>]*>.*$`)
var filterCompleteMarkerPattern = regexp.MustCompile(`(?is)<\s*__filter_complete__\s*>`)
var thoughtLinePrefixPattern = regexp.MustCompile(`(?im)^\s*(?:chain of thought|hidden chain-of-thought|thought process|thinking|analysis|reasoning|scratchpad)\s*:\s*.*(?:\r?\n|$)`)

var oocPrefixPattern = regexp.MustCompile(`(?i)^\s*(?:/ooc\b|ooc\b\s*[:\-]|out\s+of\s+character\b|#{1,6}\s*(?:ooc|out\s+of\s+character)\b|\[\s*ooc\s*\]|\[\[\s*ooc\s*\]\]|\(\s*ooc\s*\)|\(\(\s*ooc\s*\)\)|오오씨)`)
var sourceControlHeaderPattern = regexp.MustCompile(`(?i)^\s*(?:#{1,6}\s*)?(?:\[+\s*)?(?:narrative guide|story intent|scene mandate|forbidden moves|pressure level|prompt template|response template|system prompt|developer message|author note|system note|meta note|common behaviou?r|behaviou?r guide|style guide|writing guide|response rules|instructions?|rules?|persona|pov|long-term memory archive|archive label|toggle expansion)(?:\s*\]+)?\s*:?\s*$`)
var sourceControlInlinePattern = regexp.MustCompile(`(?i)\b(?:narrative guide|story intent|scene mandate|forbidden moves|pressure level|prompt template|response template|system prompt|developer message|author note|system note|meta note|common behaviou?r|behaviou?r guide|style guide|writing guide|response rules|instructions?|rules?|persona|pov|long-term memory archive|archive label|toggle expansion|lorebook|preset)\b`)
var sourceControlPlaceholderPattern = regexp.MustCompile(`(?i)(?:\{\{\s*(?:user|char)\s*\}\}|<\s*(?:user|char|system|developer|assistant|thoughts)\s*>|</\s*thoughts\s*>)`)
var sourceControlFieldPattern = regexp.MustCompile(`(?i)(?:preset|template|control|prompt|system|developer|narrative_control|lorebook|decorator|cbs)`)
var criticRetrySensitivePattern = regexp.MustCompile(`(?i)(?:\b(?:penis|vagina|clitoris|ejaculat\w*|orgasm\w*|semen|cum|penetrat\w*)\b|성기|음경|질|귀두|사정|삽입|오르가즘|정액|클리토리스)`)

func shouldApplyCompleteTurnOOCGuard(userInput, assistantContent string, contextMessages []map[string]any) bool {
	if looksLikeOOCText(userInput) || looksLikeOOCText(assistantContent) {
		return true
	}
	start := len(contextMessages) - 3
	if start < 0 {
		start = 0
	}
	for i := len(contextMessages) - 1; i >= start; i-- {
		item := contextMessages[i]
		if !strings.EqualFold(strings.TrimSpace(stringFromMap(item, "role")), "user") {
			continue
		}
		if looksLikeOOCText(stringFromMap(item, "content")) {
			return true
		}
	}
	return false
}

func looksLikeOOCText(text string) bool {
	return oocPrefixPattern.MatchString(strings.TrimSpace(text))
}

func shouldSkipDerivedIngestForSourceAwareGuard(userInput, assistantContent string) bool {
	return looksLikeSourceControlResidue(userInput) || looksLikeSourceControlResidue(assistantContent)
}

func looksLikeSourceControlResidue(text string) bool {
	raw := strings.TrimSpace(text)
	if len(raw) < 12 {
		return false
	}
	cues := 0
	if sourceControlInlinePattern.MatchString(raw) {
		cues++
	}
	if sourceControlPlaceholderPattern.MatchString(raw) {
		cues++
	}
	if strings.Count(raw, "```") >= 2 {
		cues++
	}
	headerCount := 0
	bulletOrRuleCount := 0
	for _, line := range strings.Split(raw, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		if sourceControlHeaderPattern.MatchString(stripped) {
			headerCount++
			continue
		}
		if strings.HasPrefix(stripped, "- ") || strings.HasPrefix(stripped, "* ") || strings.Contains(stripped, ": ") {
			if sourceControlInlinePattern.MatchString(stripped) || sourceControlPlaceholderPattern.MatchString(stripped) {
				bulletOrRuleCount++
			}
		}
	}
	if headerCount >= 1 {
		cues++
	}
	if headerCount >= 2 || bulletOrRuleCount >= 2 {
		cues++
	}
	return cues >= 2
}

func sanitizeTextForCriticInput(text string) string {
	cleaned := sanitizeCriticStorageText(text)
	if cleaned == "" {
		return ""
	}
	if looksLikeSourceControlResidue(cleaned) {
		return ""
	}
	lines := []string{}
	skipControlSection := false
	for _, line := range strings.Split(cleaned, "\n") {
		stripped := strings.TrimSpace(line)
		if skipControlSection {
			if stripped == "" {
				skipControlSection = false
			}
			continue
		}
		if sourceControlHeaderPattern.MatchString(stripped) {
			skipControlSection = true
			continue
		}
		if sourceControlInlinePattern.MatchString(stripped) && (strings.HasPrefix(stripped, "#") || strings.HasPrefix(stripped, "- ") || strings.HasPrefix(stripped, "* ") || strings.HasSuffix(stripped, ":")) {
			continue
		}
		lines = append(lines, line)
	}
	out := strings.TrimSpace(strings.Join(lines, "\n"))
	if looksLikeSourceControlResidue(out) {
		return ""
	}
	return out
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

func redactSensitiveCriticRetryText(text string) (string, bool) {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return "", false
	}
	redacted := criticRetrySensitivePattern.ReplaceAllString(cleaned, "[intimate scene detail redacted for critic retry]")
	return redacted, redacted != cleaned
}

func sanitizeContextMessagesForCriticInput(messages []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, item := range messages {
		if contextMessageMarkedSourceControl(item) {
			continue
		}
		copied := map[string]any{}
		for k, v := range item {
			copied[k] = v
		}
		if content, ok := item["content"].(string); ok {
			cleaned := sanitizeTextForCriticInput(content)
			if cleaned == "" && strings.TrimSpace(content) != "" {
				continue
			}
			copied["content"] = cleaned
		}
		out = append(out, copied)
	}
	return out
}

func contextMessageMarkedSourceControl(item map[string]any) bool {
	for _, key := range []string{"source", "source_layer", "origin", "kind", "type", "name", "label"} {
		if sourceControlFieldPattern.MatchString(strings.TrimSpace(stringFromMap(item, key))) {
			return true
		}
	}
	if meta := mapFromAny(item["metadata"]); len(meta) > 0 {
		for _, key := range []string{"source", "source_layer", "origin", "kind", "type", "name", "label"} {
			if sourceControlFieldPattern.MatchString(strings.TrimSpace(stringFromMap(meta, key))) {
				return true
			}
		}
	}
	return false
}

func (s *Server) canonicalCharacterName(ctx context.Context, sid, proposed string) string {
	proposed = strings.TrimSpace(proposed)
	if proposed == "" || s.Store == nil {
		return proposed
	}
	states, err := s.Store.ListCharacterStates(ctx, sid)
	if err != nil || len(states) == 0 {
		return proposed
	}
	proposedKeys := comparableCharacterAliasKeys(proposed)
	proposedKey := firstNonEmpty(proposedKeys...)
	if proposedKey == "" {
		return proposed
	}
	bestName := proposed
	bestDistance := 999
	for _, state := range states {
		candidate := strings.TrimSpace(state.CharacterName)
		if candidate == "" {
			continue
		}
		candidateKeys := comparableCharacterAliasKeys(candidate)
		candidateKey := firstNonEmpty(candidateKeys...)
		if candidateKey == "" {
			continue
		}
		if characterAliasKeysOverlap(proposedKeys, candidateKeys) {
			return candidate
		}
		dist := levenshteinDistance(proposedKey, candidateKey)
		maxLen := len([]rune(proposedKey))
		if other := len([]rune(candidateKey)); other > maxLen {
			maxLen = other
		}
		if maxLen <= 4 {
			continue
		}
		if dist < bestDistance && dist <= 2 {
			bestDistance = dist
			bestName = candidate
		}
	}
	return bestName
}

func canonicalCharacterAliasKey(name string) string {
	key := normalizeCharacterKey(name)
	if key == "" {
		return ""
	}
	if canonical, ok := knownCharacterAliasKeys[key]; ok {
		return canonical
	}
	return key
}

var knownCharacterAliasKeys = map[string]string{
	"isiu":              "siwoo",
	"leesiwoo":          "siwoo",
	"leesiwu":           "siwoo",
	"siu":               "siwoo",
	"siwoo":             "siwoo",
	"siwu":              "siwoo",
	"chloe":             "chloe",
	"kloe":              "chloe",
	"keulroe":           "chloe",
	"asuna":             "asuna",
	"aseuna":            "asuna",
	"ichinoseasuna":     "asuna",
	"ichinoseaseuna":    "asuna",
	"ichinose":          "ichinose",
	"saori":             "saori",
	"sao-ri":            "saori",
	"ichinoseasna":      "asuna",
	"ichinoseasunah":    "asuna",
	"ichinoseaseunah":   "asuna",
	"ichinoseasunaich":  "asuna",
	"ichinoseaseunaich": "asuna",
	"vex":               "vex",
	"bex":               "vex",
	"bekseu":            "vex",
}

func comparableCharacterAliasKeys(name string) []string {
	added := map[string]bool{}
	out := []string{}
	add := func(value string) {
		key := canonicalCharacterAliasKey(value)
		if key == "" || added[key] {
			return
		}
		added[key] = true
		out = append(out, key)
	}
	add(name)
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '-', '_', '.', '/', '\\', '·', '・', '(', ')', '[', ']', '{', '}', ':', ';', ',', '\'':
			return true
		default:
			return false
		}
	}) {
		part = strings.TrimSpace(part)
		if len([]rune(canonicalCharacterAliasKey(part))) >= 4 {
			add(part)
		}
	}
	return out
}

func characterAliasKeysOverlap(left, right []string) bool {
	for _, l := range left {
		for _, r := range right {
			if l == "" || r == "" {
				continue
			}
			if l == r {
				return true
			}
			if len([]rune(l)) >= 5 && strings.HasSuffix(r, l) {
				return true
			}
			if len([]rune(r)) >= 5 && strings.HasSuffix(l, r) {
				return true
			}
		}
	}
	return false
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

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			cur[j] = min3Int(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func min3Int(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

var genericDescriptorHumanTokens = map[string]bool{
	"person": true, "people": true, "human": true, "stranger": true, "figure": true, "someone": true,
	"man": true, "woman": true, "boy": true, "girl": true, "guy": true, "lady": true, "male": true, "female": true,
	"guard": true, "maid": true, "teacher": true, "student": true, "clerk": true, "soldier": true,
	"사람": true, "남자": true, "여자": true, "소년": true, "소녀": true, "경비": true, "하녀": true, "학생": true,
}

var descriptorSplitPattern = regexp.MustCompile(`[\s_\-.,/|()[\]{}"'` + "`" + `]+`)

func looksLikeTransientDescriptorCharacterName(name string) bool {
	tokens := characterDescriptorKeywords(name)
	if len(tokens) == 0 {
		return false
	}
	hasGeneric := false
	nonGeneric := 0
	for _, token := range tokens {
		if genericDescriptorHumanTokens[token] {
			hasGeneric = true
		} else {
			nonGeneric++
		}
	}
	return hasGeneric && (nonGeneric > 0 || len(tokens) == 1)
}

func characterDescriptorKeywords(name string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range descriptorSplitPattern.Split(strings.ToLower(strings.TrimSpace(name)), -1) {
		token := strings.TrimSpace(strings.TrimSuffix(raw, "'s"))
		if token == "" || token == "a" || token == "an" || token == "the" {
			continue
		}
		if len([]rune(token)) < 3 && !containsKorean(token) {
			continue
		}
		if !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

func containsKorean(text string) bool {
	for _, r := range text {
		if (r >= 0xAC00 && r <= 0xD7A3) || (r >= 0x1100 && r <= 0x11FF) {
			return true
		}
	}
	return false
}

func characterDeltaHasContinuityAnchor(delta map[string]any) bool {
	for _, key := range []string{"appearance", "personality", "relationships", "speech_style"} {
		if hasMeaningfulPayload(delta[key]) {
			return true
		}
	}
	for _, item := range sliceFromAny(delta["events"]) {
		eventType := strings.TrimSpace(stringFromMap(mapFromAny(item), "type"))
		if eventType == "relationship_shift" || eventType == "personality_change" {
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
			TimeoutMs:             int64FromMap(criticMap, "timeout_ms", 60000),
			Temperature:           floatFromMap(criticMap, "temperature", 0.2),
			MaxTokens:             int64FromMap(criticMap, "max_tokens", 1600),
			MaxCompletionTokens:   int64FromMap(criticMap, "max_completion_tokens", 1600),
			ReasoningPreset:       stringFromMap(criticMap, "reasoning_preset"),
			ReasoningEffort:       stringFromMap(criticMap, "reasoning_effort"),
			ReasoningBudgetTokens: int64FromMap(criticMap, "reasoning_budget_tokens", 0),
			GlmThinkingType:       stringFromMap(criticMap, "glm_thinking_type"),
			ForceWorldRuleAudit:   boolFromAny(meta["force_world_rule_backfill"]) || boolFromAny(meta["force_focused_world_rule_audit"]),
		},
		Embedder: completeTurnEmbeddingConfig{
			APIKey:    stringFromMap(embeddingMap, "api_key"),
			Endpoint:  stringFromMap(embeddingMap, "endpoint"),
			Model:     stringFromMap(embeddingMap, "model"),
			Provider:  stringFromMap(embeddingMap, "provider"),
			TimeoutMs: int64FromMap(embeddingMap, "timeout_ms", 30000),
		},
	}
}

func (s *Server) completeTurnExtractionConfig(meta map[string]any) completeTurnExtractionConfig {
	cfg := completeTurnExtractionConfigFromMeta(meta)
	rt := s.runtimeConfigSnapshot()
	criticMap := mapFromAny(meta["critic"])

	cfg.Critic.Provider = extractionFirstNonEmpty(cfg.Critic.Provider, rt.CriticProvider)
	cfg.Critic.APIKey = extractionFirstNonEmpty(cfg.Critic.APIKey, rt.CriticAPIKey)
	cfg.Critic.Endpoint = extractionFirstNonEmpty(cfg.Critic.Endpoint, rt.CriticEndpoint)
	cfg.Critic.Model = extractionFirstNonEmpty(cfg.Critic.Model, rt.CriticModel)
	if rt.CriticTimeoutSec > 0 {
		cfg.Critic.TimeoutMs = runtimeTimeoutMs(rt.CriticTimeoutSec, cfg.Critic.TimeoutMs)
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

	if rt.Synced {
		cfg.Embedder.Provider = extractionFirstNonEmpty(cfg.Embedder.Provider, rt.EmbeddingProvider)
		cfg.Embedder.APIKey = extractionFirstNonEmpty(cfg.Embedder.APIKey, rt.EmbeddingAPIKey)
		cfg.Embedder.Endpoint = extractionFirstNonEmpty(cfg.Embedder.Endpoint, rt.EmbeddingEndpoint)
	} else {
		cfg.Embedder.Provider = extractionFirstNonEmpty(
			cfg.Embedder.Provider,
			rt.EmbeddingProvider,
			s.Cfg.EmbedderProvider,
			embeddingEnvFirst("AC_EMBEDDER_PROVIDER", "AC_LT_EMBEDDING_PROVIDER", "PROJECT_EMBEDDING_PROVIDER", "AC_PROJECT_EMBEDDING_PROVIDER"),
		)
		cfg.Embedder.APIKey = extractionFirstNonEmpty(
			cfg.Embedder.APIKey,
			rt.EmbeddingAPIKey,
			embeddingEnvFirst("AC_EMBEDDER_API_KEY", "AC_LT_EMBEDDING_API_KEY", "PROJECT_EMBEDDING_API_KEY", "AC_PROJECT_EMBEDDING_API_KEY"),
		)
		cfg.Embedder.Endpoint = extractionFirstNonEmpty(
			cfg.Embedder.Endpoint,
			rt.EmbeddingEndpoint,
			s.Cfg.EmbedderEndpoint,
			embeddingEnvFirst("AC_EMBEDDER_ENDPOINT", "AC_LT_EMBEDDING_ENDPOINT", "PROJECT_EMBEDDING_ENDPOINT", "AC_PROJECT_EMBEDDING_ENDPOINT"),
		)
	}
	cfg.Embedder.Model = extractionFirstNonEmpty(cfg.Embedder.Model, s.currentProjectEmbeddingModel())
	if rt.EmbeddingTimeoutSec > 0 {
		cfg.Embedder.TimeoutMs = runtimeTimeoutMs(rt.EmbeddingTimeoutSec, cfg.Embedder.TimeoutMs)
	}
	return cfg
}

func (c completeTurnLLMConfig) hasConfig() bool {
	return len(c.missingFields()) == 0
}

func (c completeTurnLLMConfig) missingFields() []string {
	return configMissingFieldsWithProvider(c.Provider, c.APIKey, c.Endpoint, c.Model)
}

func (c completeTurnEmbeddingConfig) hasConfig() bool {
	return len(c.missingFields()) == 0
}

func (c completeTurnEmbeddingConfig) missingFields() []string {
	return configMissingFieldsWithProvider(c.Provider, c.APIKey, c.Endpoint, c.Model)
}

func completeTurnLLMConfigTrace(cfg completeTurnExtractionConfig) map[string]any {
	criticTrace := map[string]any{
		"configured":     cfg.Critic.hasConfig(),
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

func (s *Server) runCompleteTurnCritic(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, cfg completeTurnLLMConfig, languageContextArg ...map[string]any) (map[string]any, map[string]any, error) {
	if !cfg.hasConfig() {
		return nil, nil, errors.New("critic_config_missing")
	}
	var languageContext map[string]any
	if len(languageContextArg) > 0 {
		languageContext = normalizeCompleteTurnLanguageContext(languageContextArg[0])
	}
	systemPrompt, promptSource := readCriticSystemPrompt(s.Cfg.PromptDir)
	sanitizedUserInput := sanitizeTextForCriticInput(userInput)
	sanitizedAssistantContent := sanitizeTextForCriticInput(assistantContent)
	safeUserInput := boundCompleteTurnCriticInput(sanitizedUserInput, 4000)
	safeAssistantContent := boundCompleteTurnCriticInput(sanitizedAssistantContent, 9000)
	if strings.TrimSpace(safeUserInput+"\n"+safeAssistantContent) == "" {
		return nil, map[string]any{"prompt_source": promptSource, "source_aware_ingest_guard": true}, errors.New("source_aware_ingest_guard")
	}
	safeContextMessages := sanitizeContextMessagesForCriticInput(contextMessages)
	previewPass := s.buildCompleteTurnCriticPreviewPass(ctx, sid, turnIndex, safeContextMessages, safeUserInput, safeAssistantContent)
	criticArchiveLedgerPromptInput, criticArchiveLedgerTrace := s.buildCompleteTurnCriticArchiveLedgerInput(ctx, sid, turnIndex, safeAssistantContent, outputLanguageOverride)
	userPrompt := buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, safeUserInput, safeAssistantContent, safeContextMessages, outputLanguageOverride, previewPass, languageContext, criticArchiveLedgerPromptInput)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1600
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}

	upstream, _, err := performProxyPluginMain(ctx, req)
	providerRetryTrace := map[string]any{}
	if err != nil {
		retryUserInput, userRedacted := redactSensitiveCriticRetryText(safeUserInput)
		retryAssistantContent, assistantRedacted := redactSensitiveCriticRetryText(safeAssistantContent)
		if !userRedacted && !assistantRedacted {
			return nil, nil, err
		}
		retryPreviewPass := s.buildCompleteTurnCriticPreviewPass(ctx, sid, turnIndex, safeContextMessages, retryUserInput, retryAssistantContent)
		retryPrompt := buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, retryUserInput, retryAssistantContent, safeContextMessages, outputLanguageOverride, retryPreviewPass, languageContext, criticArchiveLedgerPromptInput)
		retryReq := req
		retryReq.Messages = []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": retryPrompt}}
		retryUpstream, _, retryErr := performProxyPluginMain(ctx, retryReq)
		providerRetryTrace = map[string]any{
			"mode":                "sensitive_input_redacted_retry",
			"user_input_redacted": userRedacted,
			"assistant_redacted":  assistantRedacted,
			"first_error":         err.Error(),
			"retry_preview_pass":  retryPreviewPass,
		}
		if retryErr != nil {
			providerRetryTrace["retry_error"] = retryErr.Error()
			return nil, providerRetryTrace, fmt.Errorf("%w; redacted critic retry failed: %v", err, retryErr)
		}
		upstream = retryUpstream
		previewPass = retryPreviewPass
		safeUserInput = retryUserInput
		safeAssistantContent = retryAssistantContent
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return nil, map[string]any{"raw_preview": truncateRunes(content, 1000), "prompt_source": promptSource}, err
	}
	trace := map[string]any{
		"prompt_source": promptSource,
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
		"input_budget": map[string]any{
			"user_input_chars":        len([]rune(safeUserInput)),
			"assistant_content_chars": len([]rune(safeAssistantContent)),
			"user_input_bounded":      len([]rune(sanitizedUserInput)) > len([]rune(safeUserInput)),
			"assistant_bounded":       len([]rune(sanitizedAssistantContent)) > len([]rune(safeAssistantContent)),
		},
		"pipeline": map[string]any{
			"policy_version": completeTurnCriticPipelineVersion,
			"stages": map[string]any{
				"evidence_extractor": map[string]any{
					"status":                 "ok",
					"owner":                  "complete_turn.configured_critic_extract",
					"preview_policy_version": completeTurnCriticPreviewPassVersion,
					"preview_seed_applied":   true,
				},
				"deterministic_reducer": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.normalizeCriticExtraction",
				},
				"focused_recall_enricher": map[string]any{
					"status": "ok",
					"owner":  "complete_turn.enrichNormalizedCriticExtractionForFocusedRecall",
				},
				"summary_compactor_background": map[string]any{
					"status": "handoff",
					"owner":  "complete_turn.maintenance_handoff",
				},
			},
		},
		"preview_pass": previewPass,
	}
	trace["critic_archive_ledger"] = criticArchiveLedgerTrace
	if len(languageContext) > 0 {
		trace["language_context"] = languageContext
		trace["memory_write_contract"] = completeTurnMemoryWriteContract(languageContext)
	}
	if len(providerRetryTrace) > 0 {
		trace["provider_retry"] = providerRetryTrace
	}
	normalized := normalizeCriticExtraction(parsed)
	if len(worldRuleItemsForSave(normalized)) == 0 && (cfg.ForceWorldRuleAudit || shouldRunFocusedWorldRuleAudit(normalized)) {
		auditedRules, auditTrace := s.runCompleteTurnWorldRuleAudit(ctx, sid, turnIndex, safeUserInput, safeAssistantContent, safeContextMessages, previewPass, normalized, cfg)
		trace["world_rule_audit"] = auditTrace
		if len(worldRuleItemsForSave(auditedRules)) > 0 {
			var mergedCount int
			normalized, mergedCount = mergeWorldRuleAuditIntoExtraction(normalized, auditedRules)
			auditTrace["merged_world_rule_count"] = mergedCount
		}
	} else if len(worldRuleItemsForSave(normalized)) > 0 {
		trace["world_rule_audit"] = map[string]any{
			"status": "skipped",
			"reason": "initial_extraction_has_world_rules",
		}
	} else {
		reason := "initial_audit_did_not_request_focused_world_rule_pass"
		if cfg.ForceWorldRuleAudit {
			reason = "force_world_rule_audit_configured_but_not_reached"
		}
		trace["world_rule_audit"] = map[string]any{
			"status": "skipped",
			"reason": reason,
		}
	}
	normalized = enrichNormalizedCriticExtractionForFocusedRecall(normalized, safeUserInput, safeAssistantContent, turnIndex)
	normalized = applyLanguageMemoryWriteContract(normalized, languageContext)
	return normalized, trace, nil
}

func shouldRunFocusedWorldRuleAudit(extraction map[string]any) bool {
	audit := mapFromAny(extraction["world_rule_audit"])
	if len(audit) == 0 {
		audit = mapFromAny(extraction["world_rules_audit"])
	}
	if len(audit) == 0 {
		return false
	}
	for _, key := range []string{"durable_rule_found", "rule_found", "needs_world_rule", "audit_positive"} {
		if boolFromAny(audit[key]) {
			return true
		}
	}
	status := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(audit, "status"),
		stringFromMap(audit, "verdict"),
		stringFromMap(audit, "decision"),
	)))
	return status == "positive" || status == "found" || status == "needs_world_rule"
}

func (s *Server) runCompleteTurnWorldRuleAudit(ctx context.Context, sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, previewPass map[string]any, initialExtraction map[string]any, cfg completeTurnLLMConfig) (map[string]any, map[string]any) {
	trace := map[string]any{
		"status":           "skipped",
		"policy_version":   "world_rule_audit.v1",
		"llm_call_attempt": false,
	}
	if !cfg.hasConfig() {
		trace["reason"] = "critic_config_missing"
		return nil, trace
	}
	if strings.TrimSpace(userInput+"\n"+assistantContent) == "" {
		trace["reason"] = "empty_turn"
		return nil, trace
	}
	prompt := buildCompleteTurnWorldRuleAuditPrompt(sid, turnIndex, userInput, assistantContent, contextMessages, previewPass, initialExtraction)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 || maxTokens > 1200 {
		maxTokens = 1200
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 || maxCompletionTokens > 1200 {
		maxCompletionTokens = maxTokens
	}
	if maxCompletionTokens < 700 {
		maxCompletionTokens = 700
	}
	temp := cfg.Temperature
	if temp > 0.3 {
		temp = 0.2
	}
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": "You are Archive Center's world-rule audit extractor. Return only valid JSON. Do not use markdown fences."}, map[string]any{"role": "user", "content": prompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	trace["llm_call_attempt"] = true
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		return nil, trace
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		trace["status"] = "error"
		trace["error"] = err.Error()
		trace["raw_preview"] = truncateRunes(content, 1000)
		return nil, trace
	}
	normalized := normalizeCriticExtraction(parsed)
	count := len(worldRuleItemsForSave(normalized))
	trace["status"] = "ok"
	trace["model"] = extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model)
	trace["usage"] = upstream["usage"]
	trace["world_rule_count"] = count
	if count == 0 {
		trace["reason"] = extractionFirstNonEmpty(stringFromMap(mapFromAny(parsed["audit"]), "reason"), "audit_returned_no_durable_rule")
	}
	return normalized, trace
}

func buildCompleteTurnWorldRuleAuditPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, previewPass map[string]any, initialExtraction map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	preview, _ := json.Marshal(previewPass)
	initial, _ := json.Marshal(initialExtraction)
	return strings.Join([]string{
		"Audit whether the completed turn establishes durable world rules that the main extraction missed.",
		"Return ONLY JSON. Do not use markdown fences.",
		"Use this JSON shape:",
		`{"audit":{"durable_rule_found":false,"reason":""},"world_rules":[],"world_state":{"version":"world_state.v1","confidence":0,"verification":"","rules":[]}}`,
		"Decision contract:",
		"- This is an AI judgement step. Do not rely on keyword lists, genre names, or instruction examples as facts.",
		"- Extract the abstract invariant established by the session's own evidence.",
		"- A world rule is a durable constraint that should remain true after this exchange: physical/natural law, supernatural or technology mechanic, progression or reward economy, acquisition method, access gate, location constraint, social law, institution/custom, faction norm, rank/authority rule, contract, resource/logistics limit, schedule/calendar rule, taboo, or equivalent stable setting law.",
		"- Creation myths, cosmology, divine non-intervention rules, origin rules for monsters/threats, granted powers, chosen-agent roles, sacred/institutional authority, and stable religious doctrine are world rules when the turn presents them as setting truth rather than rumor or metaphor.",
		"- It can appear in any genre: academy, workplace, household, romance, survival, fantasy, dungeon/progression, sci-fi, political, slice-of-life, or apocalypse.",
		"- If the latest turn only has a temporary action, mood, one-off dialogue, rejected plan, speculation, or private thought with no durable setting constraint, return empty arrays.",
		"- If the latest turn confirms a durable rule, world_rules must not be empty. Emit compact evidence-bound rules with scope, category, key, value, and optional scope_name/genre/confidence/verification.",
		"- Use the canonical scope vocabulary exactly: root, region, location, faction, system, session.",
		"- Scope guidance: root=universal cosmology or setting-wide law; region=named country/city/territory/large area; location=concrete place/base/building/dungeon/site; faction=organization/church/guild/government/gang/party/team; system=magic/technology/progression/economy/combat/reward mechanics; session=temporary session-only plan or rule without a more specific stable scope.",
		"- Do not put named regions, named locations, named factions, or progression mechanics under root just because they are important. Use their specific scope and scope_name.",
		"- Mirror the same durable rules in world_state.rules when they shape the current setting state.",
		"- Do not invent mechanics. If uncertain, use audit.reason and return empty arrays.",
		"",
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Deterministic_Preview_Pass_JSON>",
		string(preview),
		"</Deterministic_Preview_Pass_JSON>",
		"",
		"<Initial_Critic_Extraction_JSON>",
		string(initial),
		"</Initial_Critic_Extraction_JSON>",
	}, "\n")
}

func mergeWorldRuleAuditIntoExtraction(base map[string]any, audit map[string]any) (map[string]any, int) {
	items := worldRuleItemsForSave(audit)
	if len(items) == 0 {
		return base, 0
	}
	out := make(map[string]any, len(base)+2)
	for k, v := range base {
		out[k] = v
	}
	out["world_rules"] = append(sliceFromAny(out["world_rules"]), items...)
	ws := mapFromAny(out["world_state"])
	if len(ws) == 0 {
		ws = map[string]any{
			"version":      "world_state.v1",
			"confidence":   0.85,
			"verification": "verified_by_world_rule_audit",
		}
	}
	ws["rules"] = append(sliceFromAny(ws["rules"]), items...)
	if strings.TrimSpace(stringFromMap(ws, "version")) == "" {
		ws["version"] = "world_state.v1"
	}
	if strings.TrimSpace(stringFromMap(ws, "verification")) == "" {
		ws["verification"] = "verified_by_world_rule_audit"
	}
	out["world_state"] = ws
	return out, len(worldRuleItemsForSave(out))
}

func (s *Server) buildCompleteTurnCriticArchiveLedgerInput(ctx context.Context, sid string, turnIndex int, assistantContent string, outputLanguageOverride *map[string]any) (map[string]any, map[string]any) {
	trace := map[string]any{
		"enabled":          s != nil && s.Cfg.CriticLedgerEnabled,
		"included":         false,
		"contract_version": criticArchiveLedgerContractVersion,
	}
	if s == nil || !s.Cfg.CriticLedgerEnabled {
		trace["status"] = "disabled"
		return nil, trace
	}
	req := criticArchiveLedgerPreviewRequest{
		ChatSessionID:          sid,
		TurnIndex:              turnIndex,
		AssistantFinalText:     assistantContent,
		AssistantFinalLanguage: completeTurnAssistantFinalLanguage(outputLanguageOverride),
		StreamingMismatch:      "unknown",
	}
	resp := s.buildCriticArchiveLedgerPreviewWithContext(ctx, req)
	promptInput := criticArchiveLedgerPromptInput(resp)
	trace["included"] = true
	trace["status"] = resp.Status
	trace["item_count"] = len(resp.Items)
	trace["vector_status"] = resp.VectorStatus
	trace["language"] = resp.Language
	trace["safety"] = resp.Safety
	trace["degraded"] = resp.Degraded
	trace["warnings"] = resp.Warnings
	trace["write_attempted"] = resp.WriteAttempted
	trace["vector_write_attempted"] = resp.VectorWriteAttempted
	trace["llm_call_attempted"] = resp.LLMCallAttempted
	return promptInput, trace
}

func completeTurnAssistantFinalLanguage(outputLanguageOverride *map[string]any) string {
	if outputLanguageOverride == nil || *outputLanguageOverride == nil {
		return ""
	}
	for _, key := range []string{"language", "lang", "target_language", "output_language"} {
		if value, ok := (*outputLanguageOverride)[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func criticArchiveLedgerPromptInput(resp criticArchiveLedgerPreviewResponse) map[string]any {
	items := make([]map[string]any, 0, len(resp.Items))
	for _, item := range resp.Items {
		items = append(items, map[string]any{
			"lane":       item.Lane,
			"id":         item.ID,
			"authority":  item.Authority,
			"status":     item.Status,
			"summary":    item.Summary,
			"updated_at": item.UpdatedAt,
			"source_ref": item.SourceRef,
		})
	}
	return map[string]any{
		"contract_version":         resp.ContractVersion,
		"status":                   resp.Status,
		"session_id":               resp.SessionID,
		"runtime_profile":          resp.RuntimeProfile,
		"store_mode":               resp.StoreMode,
		"vector_status":            resp.VectorStatus,
		"language":                 resp.Language,
		"limits":                   resp.Limits,
		"counts":                   resp.Counts,
		"safety":                   resp.Safety,
		"degraded":                 resp.Degraded,
		"warnings":                 resp.Warnings,
		"items":                    items,
		"read_only":                true,
		"write_attempted":          false,
		"vector_write_attempted":   false,
		"llm_call_attempted":       false,
		"raw_archive_dump_blocked": true,
		"usage_policy":             "support_only_do_not_copy_as_new_evidence_without_latest_turn_support",
	}
}

func readCriticSystemPrompt(configuredDir string) (string, string) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "critic_system.txt"))
	}
	candidates = append(candidates,
		filepath.Join("..", "prompts", "critic_system.txt"),
		filepath.Join("prompts", "critic_system.txt"),
		filepath.Join("..", "..", "prompts", "critic_system.txt"),
	)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), path
		}
	}
	return "You are Archive Center's critic extractor. Return only valid JSON matching the configured critic schema.", "fallback_builtin"
}

func readSupervisorSystemPrompt(configuredDir string) (string, string) {
	candidates := []string{}
	if strings.TrimSpace(configuredDir) != "" {
		candidates = append(candidates, filepath.Join(configuredDir, "supervisor_system.txt"))
	}
	candidates = append(candidates,
		filepath.Join("..", "prompts", "supervisor_system.txt"),
		filepath.Join("prompts", "supervisor_system.txt"),
		filepath.Join("..", "..", "prompts", "supervisor_system.txt"),
	)
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data), path
		}
	}
	return "You are Archive Center's supervisor. Return only valid JSON with a directive object.", "fallback_builtin"
}

func buildCompleteTurnCriticPrompt(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, previewPass map[string]any, archiveLedger ...map[string]any) string {
	return buildCompleteTurnCriticPromptWithLanguageContext(sid, turnIndex, userInput, assistantContent, contextMessages, outputLanguageOverride, previewPass, nil, archiveLedger...)
}

func buildCompleteTurnCriticPromptWithLanguageContext(sid string, turnIndex int, userInput string, assistantContent string, contextMessages []map[string]any, outputLanguageOverride *map[string]any, previewPass map[string]any, languageContext map[string]any, archiveLedger ...map[string]any) string {
	ctx, _ := json.Marshal(contextMessages)
	lang, _ := json.Marshal(outputLanguageOverride)
	langCtx, _ := json.Marshal(normalizeCompleteTurnLanguageContext(languageContext))
	preview, _ := json.Marshal(previewPass)
	var ledgerInput any
	if len(archiveLedger) > 0 && archiveLedger[0] != nil {
		ledgerInput = archiveLedger[0]
	}
	ledger, _ := json.Marshal(ledgerInput)
	return strings.Join([]string{
		"Extract durable Archive Center memory data from the completed turn.",
		"Return ONLY JSON. Do not use markdown fences.",
		"Use this JSON shape. Omit unknown facts instead of inventing placeholders:",
		`{"turn_summary":"","importance_score":5,"evidence_excerpts":[],"kg_triples":[],"entities":{"characters":[],"locations":[],"items":[]},"relationship_memory":{},"state_deltas":{},"character_deltas":[],"physical_conditions":[],"entity_conditions":[],"pending_threads":[],"world_rule_audit":{"durable_rule_found":false,"reason":""},"world_rules":[],"world_state":{"version":"world_state.v1","confidence":0,"verification":"","rules":[]},"subjective_entity_memories":[],"protected_secrets":[],"character_identity_accuracy":[],"persona_capsule_candidates":[],"archive_hint":{}}`,
		"Rules:",
		"- Sensitivity policy: if the latest turn contains concrete in-story action, decision, relationship shift, promise, threat, injury, plan/resource, location movement, authority change, world constraint, or unresolved tension, extract it. Empty arrays are valid only for pure OOC/meta, repetition, or no new in-story information.",
		"- Prefer several small focused records over one vague memory. Aim to cover the user's intent, the assistant's visible outcome, affected named actors, and durable consequences without inventing anything beyond the latest turn and safe context.",
		"- evidence_excerpts must be short exact excerpts from the latest user/assistant turn, not the whole turn.",
		"- Language contract: use Language_Context_JSON as the memory-write contract. If summary_language/session_output_language is ko, en, or ja, generated natural-language memory fields must use that language. Do not default to English just because these instructions are English. Raw evidence excerpts must stay exact source text and must not be translated or rewritten.",
		"- Keep internal enum/category/predicate keys stable. Do not translate system keys per turn just because the output language changes.",
		"- For ordinary narrative turns with new information, include 1-3 evidence_excerpts that ground the most important user intent and assistant outcome.",
		"- kg_triples must use real in-story names only. Never use char_*, cid_*, turn_*, user, assistant, system, prompt, or has_turn edges.",
		"- For ordinary narrative turns with named actors, emit kg_triples for durable relations, assignments, locations, ownership, promises, threats, injuries, permissions, commands, faction links, or plan participation.",
		"- entities.characters/locations/items should contain only concrete in-story people, places, or objects observed in this turn.",
		"- Separate location/time fact classes. A current scene location or current scene time belongs in state_deltas.scene_state; a durable residence, hometown, birthplace, workplace, or affiliation belongs in character_deltas.status and/or kg_triples with predicates such as residence, hometown, lives_in, or based_in.",
		"- Do not treat 'X lives in London' as 'the current scene is London'. Do not treat a temporary visit as a durable residence unless the latest turn says it directly.",
		"- Story calendar facts such as 'summer vacation has started' belong in world_state/time_state or state_deltas.scene_state.time_state when they anchor the current scene. Do not infer an immediate return to school, a season change, or a day jump without direct evidence.",
		"- relationship_memory may include target_name or pair when trust changes. If no target exists, leave it empty.",
		"- character_deltas should capture named character status, location, emotional posture, relationship changes, injuries, intentions, or role/authority changes seen in the latest turn.",
		"- physical_conditions is for evidence-bound body/health continuity that can affect roleplay: illness, fever, cold, pregnancy, menstruation, poisoning, fracture, accident/fall injury, body damage, impairment, missing body part, recovery, worsening, or cleared condition.",
		"- Each physical_conditions item should include owner_entity_name or owner_entity_key, condition_label, effect_kind when obvious (temporary_effect or injury), evidence_excerpt, source_turn_index, and may include severity_text, body_area, onset_story_clock_json, duration_json, expires_at_clock_json, prognosis_text, age_or_vulnerability_note, uncertainty_note, and authority_hint.",
		"- Do not invent medical calendars, fixed cycles, healing times, or numeric severity values. If duration is not explicit in the latest turn or safe context, use duration_policy=unknown_until_updated and keep prognosis_text/age_or_vulnerability_note descriptive.",
		"- Do not hardcode rules such as menstruation lasting a fixed number of days or a cold always resolving quickly. Let later evidence update, clear, worsen, or extend the condition.",
		"- If LUA, a character sheet, or another chat runtime owns exact health/stat values, set authority_hint=external_runtime and record only the narrative evidence; do not override that runtime's numeric state.",
		"- entity_conditions is for evidence-bound continuity of important non-character entities, especially named items, equipment, locations, or artifacts whose changed state should persist: broken, repaired, sealed, unlocked, activated, depleted, contaminated, lost, inaccessible, transformed, or cleared.",
		"- Each entity_conditions item should include owner_entity_name or owner_entity_key, owner_entity_type when known, condition_label, evidence_excerpt, source_turn_index, and may include effect_kind, onset_story_clock_json, duration_json, expires_at_clock_json, uncertainty_note, and authority_hint.",
		"- Do not emit entity_conditions for ordinary props or unchanged descriptions. Use it only when the changed entity state would create a continuity error if forgotten later.",
		"- world_rules must describe durable world facts, not prompt instructions or style rules. You are responsible for judging them; backend code will not infer rules from keyword lists.",
		"- Emit world_rules and world_state.rules when the latest turn establishes a durable constraint that should affect future turns: natural/physical laws, magic/technology mechanics, apocalypse survival norms, unspoken social law, institutional policy, school/academy custom, workplace procedure, family/household rule, contract, rank/authority, faction/group norm, location access, schedule/calendar, economy/resource constraint, logistics doctrine, or other world-law equivalent.",
		"- The category list is non-exhaustive. If the story establishes a stable law of the setting, social order, organization, environment, or genre logic, capture it even when it does not literally use words like rule, law, policy, or protocol.",
		"- Use the canonical world-rule scope vocabulary exactly: root, region, location, faction, system, session.",
		"- Scope guidance: root=universal cosmology or setting-wide law; region=named country/city/territory/large area; location=concrete place/base/building/dungeon/site; faction=organization/church/guild/government/gang/party/team; system=magic/technology/progression/economy/combat/reward mechanics; session=temporary session-only plan or rule without a more specific stable scope.",
		"- Do not put named regions, named locations, named factions, or progression mechanics under root just because they are important. Use their specific scope and scope_name.",
		"- In system/progression stories, judge durable mechanics as world_rules when confirmed: randomized or conditional acquisition, base/home/environment constraints, challenge entry/clear/reward loops, exchange/cost economy, upgrade or unlock rules, item acquisition/crafting rules, stat growth, group/party limits, cooldowns, ranks, quests, or other recurring progression mechanics.",
		"- Mandatory world-rule audit: before returning JSON, check whether the latest turn established or confirmed any stable setting constraint, repeated system mechanic, progression mechanic, acquisition method, challenge/reward loop, exchange/cost rule, growth/unlock rule, access condition, social order, faction norm, institution rule, resource/logistics rule, environment constraint, magic/technology law, rank/authority rule, schedule/calendar rule, contract, taboo, or unspoken norm.",
		"- Always fill world_rule_audit. If that audit is positive, set world_rule_audit.durable_rule_found=true and world_rules must not be empty. Emit at least one compact evidence-bound rule with scope, category, key, and value; mirror it in world_state.rules when it shapes current setting state.",
		"- If you detect a durable rule but cannot fit the final rule list, still set world_rule_audit.durable_rule_found=true and explain the missing rule in world_rule_audit.reason. A focused follow-up audit may repair the omission.",
		"- Early-session setup counts. Do not wait for many turns: a 1-7 turn session can already establish foundational world rules such as randomized acquisition, progression currency exchange, challenge reward loops, environment/base constraints, access gates, or upgrade/item progression.",
		"- Extract the abstract invariant behind the session's surface nouns. Do not copy these instruction examples as setting facts; use the session's own evidence and names.",
		"- Do not leave world_rules empty for confirmed public facts, institutional rules, class/company policies, social obligations, access permissions, hierarchy/authority rules, special-world mechanics, supernatural/technology rules, recurring resource constraints, or implicit norms that remain true beyond this single exchange.",
		"- Accepted plans, procedures, methods, route/access decisions, chain-of-command decisions, class/club/company rules, household rules, contracts, recurring social obligations, and tacit survival codes are world_rules when they remain actionable after this turn.",
		"- If the latest turn confirms a named operation, tactical doctrine, world mechanic, setting law, or unspoken social/legal norm, emit at least one scoped world_rule unless it is only a rejected idea or unverified speculation.",
		"- Each world rule must include key and value; prefer scope, scope_name, category, confidence, and verification/evidence when available. Use world_state.rules for the same durable rules when they shape the current world state.",
		"- subjective_entity_memories is for each named in-story entity's subjective recollection or interpretation of the latest turn. It is not canonical truth.",
		"- Each subjective_entity_memories item must include owner_entity_key or owner_entity_name, memory_text, and may include owner_entity_role, owner_visibility, source_turn_index, importance_10, emotional_weight, evidence_excerpt, secret_guard, target_reveal_policy, tags, and portability.",
		"- When a named character clearly feels, fears, trusts, suspects, misunderstands, decides, resents, or privately interprets the event, include a subjective_entity_memories item for that owner. Keep it evidence-bound and support-only.",
		"- Use owner_entity_role=protagonist for the player/persona and owner_entity_role=npc with owner_visibility=owner_private for private NPC recollections. Keep NPC-only memories out of persona_capsule_candidates.",
		"- subjective_entity_memories must remain support-only: never use it to overwrite current-world truth, canonical memory, direct evidence, KG triples, character state, or world rules.",
		"- NPC/private subjective_entity_memories are interpretations, suspicions, misunderstandings, or private bias unless current direct evidence states otherwise; never promote them to objective fact or narrator-revealed truth.",
		"- Conflict or misunderstanding memories should stay owner-private and may only influence that owner entity's behavior, subtext, hesitation, avoidance, or selective silence until explicit current-session reveal.",
		"- protected_secrets is for any information that should not become public narration or impossible character knowledge: private affection, guilt, shame, mistakes, lies, fears, debts, hidden plans, hidden identity, hidden role, hidden allegiance, lineage, succession, protected power inheritance, or similar private knowledge.",
		"- Each protected_secrets item may include secret_kind, owner, subject, summary, sensitivity, evidence_strength, disclosure_policy, knowledge_scope, and evidence_excerpt. Keep the text evidence-bound and do not invent secrets.",
		"- If a protected secret exists, set secret_guard=true on the matching subjective_entity_memories item and use target_reveal_policy such as owner_private_until_revealed, explicit_reveal_event_required, or user_directed_reveal_only.",
		"- Stored secret truth is not permission for spontaneous confession, public narration, or unrelated-character discovery. Preserve it as owner-scoped support until current evidence reveals it.",
		"- character_identity_accuracy is for evidence-bound identity/role/allegiance mappings such as cover identity, disguise, hidden role, hidden allegiance, secret successor, hidden lineage, or protected power inheritance. Include same_entity, surface_identity_name, true_identity_name, identity_kind, reveal_policy, and knowledge_scope when supported.",
		"- Do not use character-specific hardcoded aliases. Identity/protected-secret candidates must come from the latest turn or safe context evidence only.",
		"- persona_capsule_candidates is optional and proposal-only. Use it only for protagonist/player subjective recollections that may be carried to another session, loop, regression, reincarnation, isekai, or same-character continuation.",
		"- persona_capsule_candidates must never be used to write current-world truth, canonical memory, direct evidence, KG triples, character state, or world rules. It is support_only_persona_recollection and requires later user/operator approval.",
		"- Each persona_capsule_candidates item may include memory_text, source_turn_index, importance_10, emotional_weight, portability, mode, secret_guard, tags, evidence_excerpt, and injection_policy.",
		"- Mark secret_guard true when the recollection reveals regression, loop, reincarnation, possession/rebirth, isekai transfer, or identity-carryover that should remain protagonist-private until explicitly revealed by current user input.",
		"- Critic_Archive_Ledger_JSON is a bounded read-only support ledger. Use it to avoid duplicate memories, stale residue, and contradiction drift.",
		"- Never copy Critic_Archive_Ledger_JSON item summaries as new evidence unless the latest user/assistant turn also supports the fact.",
		"- If Critic_Archive_Ledger_JSON is null, empty, or degraded, continue extracting only from the latest turn and safe context.",
		"",
		fmt.Sprintf("chat_session_id: %s", sid),
		fmt.Sprintf("turn_index: %d", turnIndex),
		"",
		"<Latest_Turn>",
		"[User]",
		userInput,
		"",
		"[Assistant]",
		assistantContent,
		"</Latest_Turn>",
		"",
		"<Recent_Context_JSON>",
		string(ctx),
		"</Recent_Context_JSON>",
		"",
		"<Deterministic_Preview_Pass_JSON>",
		string(preview),
		"</Deterministic_Preview_Pass_JSON>",
		"",
		"<Critic_Archive_Ledger_JSON>",
		string(ledger),
		"</Critic_Archive_Ledger_JSON>",
		"",
		"<Output_Language_Override_JSON>",
		string(lang),
		"</Output_Language_Override_JSON>",
		"",
		"<Language_Context_JSON>",
		string(langCtx),
		"</Language_Context_JSON>",
	}, "\n")
}

func (s *Server) buildCompleteTurnCriticPreviewPass(ctx context.Context, sid string, turnIndex int, contextMessages []map[string]any, userInput, assistantContent string) map[string]any {
	rawPreview := []map[string]any{}
	start := len(contextMessages) - 3
	if start < 0 {
		start = 0
	}
	for _, item := range contextMessages[start:] {
		content := strings.TrimSpace(stringFromMap(item, "content"))
		if content == "" {
			continue
		}
		rawPreview = append(rawPreview, map[string]any{
			"role":    extractionFirstNonEmpty(stringFromMap(item, "role"), "unknown"),
			"text":    truncateRunes(content, 240),
			"source":  extractionFirstNonEmpty(stringFromMap(item, "source"), "context"),
			"bounded": true,
		})
	}
	directSeed := []map[string]any{}
	if s.Store != nil {
		if rows, err := s.Store.ListEvidence(ctx, sid); err == nil {
			for i := len(rows) - 1; i >= 0 && len(directSeed) < 3; i-- {
				row := rows[i]
				if row.Tombstoned || strings.TrimSpace(row.EvidenceText) == "" {
					continue
				}
				if row.SourceTurnEnd > 0 && row.SourceTurnEnd > turnIndex {
					continue
				}
				evidenceText := sanitizeTextForCriticInput(row.EvidenceText)
				if strings.TrimSpace(evidenceText) == "" {
					continue
				}
				directSeed = append(directSeed, map[string]any{
					"text":        truncateRunes(evidenceText, 240),
					"turn_anchor": row.TurnAnchor,
					"source_turn": map[string]any{"start": row.SourceTurnStart, "end": row.SourceTurnEnd},
					"kind":        row.EvidenceKind,
				})
			}
		}
	}
	latestChars := len([]rune(strings.TrimSpace(userInput + "\n" + assistantContent)))
	priority := "low"
	if len(directSeed) > 0 || len(rawPreview) >= 2 || latestChars >= 1200 {
		priority = "medium"
	}
	shouldCompact := latestChars >= 4000 || len(rawPreview) >= 3
	return map[string]any{
		"policy_version":                       completeTurnCriticPreviewPassVersion,
		"status":                               "ok",
		"recent_raw_preview":                   rawPreview,
		"recent_verified_direct_evidence_seed": directSeed,
		"triage": map[string]any{
			"priority":        priority,
			"latest_chars":    latestChars,
			"raw_item_count":  len(rawPreview),
			"direct_seed_hit": len(directSeed) > 0,
		},
		"compaction_hint": map[string]any{
			"should_trigger": shouldCompact,
			"mode":           "hint_only",
		},
	}
}

func parseJSONFromLLMContent(content string) (map[string]any, error) {
	candidate, err := extractJSONCandidateFromLLMContent(content)
	if err != nil {
		return nil, err
	}
	candidate = repairJSONCandidate(candidate)
	var out map[string]any
	if err := json.Unmarshal([]byte(candidate), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func extractJSONCandidateFromLLMContent(content string) (string, error) {
	cleaned := normalizeLLMJSONText(content)
	start := strings.Index(cleaned, "{")
	if start < 0 {
		return "", errors.New("critic_json_missing")
	}
	stack := []byte{}
	inString := false
	escaped := false
	for i := start; i < len(cleaned); i++ {
		ch := cleaned[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return "", errors.New("critic_json_mismatched_braces")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return strings.TrimSpace(cleaned[start : i+1]), nil
			}
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return "", errors.New("critic_json_mismatched_brackets")
			}
			stack = stack[:len(stack)-1]
		}
	}
	return closeTruncatedJSONCandidate(cleaned[start:], stack, inString, escaped)
}

func normalizeLLMJSONText(content string) string {
	cleaned := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	replacer := strings.NewReplacer(
		"\u201c", `"`,
		"\u201d", `"`,
		"\u201e", `"`,
		"\u201f", `"`,
		"\u2018", `'`,
		"\u2019", `'`,
	)
	cleaned = replacer.Replace(cleaned)
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```JSON")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func repairJSONCandidate(candidate string) string {
	repaired := replaceJSONLiteralsOutsideStrings(candidate)
	repaired = repairMissingJSONValuesOutsideStrings(repaired)
	repaired = jsonTrailingCommaPattern.ReplaceAllString(repaired, "$1")
	return strings.TrimSpace(repaired)
}

func closeTruncatedJSONCandidate(candidate string, stack []byte, inString bool, escaped bool) (string, error) {
	if len(stack) == 0 && !inString {
		return "", errors.New("critic_json_unclosed")
	}
	repaired := strings.TrimSpace(candidate)
	if inString {
		if escaped {
			repaired += "\\"
		}
		repaired += `"`
	}
	repaired = strings.TrimRight(repaired, " \t\r\n,")
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case '{':
			repaired += "}"
		case '[':
			repaired += "]"
		default:
			return "", errors.New("critic_json_unclosed")
		}
	}
	return repaired, nil
}

func replaceJSONLiteralsOutsideStrings(input string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); {
		ch := input[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			i++
			continue
		}
		if hasJSONLiteralAt(input, i, "None") {
			b.WriteString("null")
			i += len("None")
			continue
		}
		if hasJSONLiteralAt(input, i, "True") {
			b.WriteString("true")
			i += len("True")
			continue
		}
		if hasJSONLiteralAt(input, i, "False") {
			b.WriteString("false")
			i += len("False")
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func repairMissingJSONValuesOutsideStrings(input string) string {
	var b strings.Builder
	inString := false
	escaped := false
	expectValue := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			expectValue = false
			b.WriteByte(ch)
			continue
		}
		if ch == ':' {
			expectValue = true
			b.WriteByte(ch)
			continue
		}
		if expectValue {
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
				b.WriteByte(ch)
				continue
			}
			if ch == '}' || ch == ']' || ch == ',' {
				b.WriteString("null")
				expectValue = false
			} else {
				expectValue = false
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func hasJSONLiteralAt(input string, pos int, literal string) bool {
	if pos+len(literal) > len(input) || input[pos:pos+len(literal)] != literal {
		return false
	}
	beforeOK := pos == 0 || !isJSONLiteralChar(input[pos-1])
	after := pos + len(literal)
	afterOK := after >= len(input) || !isJSONLiteralChar(input[after])
	return beforeOK && afterOK
}

func isJSONLiteralChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func normalizeCriticExtraction(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	out["turn_summary"] = normalizeCriticTurnSummary(raw["turn_summary"])
	out["importance_score"] = clampFloat(extractionFloatFromAny(raw["importance_score"], 3), 1, 10)
	out["emotional_intensity"] = clampFloat(extractionFloatFromAny(raw["emotional_intensity"], 0), 0, 1)
	out["narrative_significance"] = clampFloat(extractionFloatFromAny(raw["narrative_significance"], 0), 0, 1)
	out["evidence_excerpts"] = stringsFromAny(raw["evidence_excerpts"])
	out["kg_triples"] = sliceFromAny(raw["kg_triples"])
	out["character_deltas"] = sliceFromAny(raw["character_deltas"])
	out["pending_threads"] = sliceFromAny(raw["pending_threads"])
	out["entities"] = mapFromAny(raw["entities"])
	out["relationship_memory"] = mapFromAny(raw["relationship_memory"])
	out["state_deltas"] = mapFromAny(raw["state_deltas"])
	out["world_rules"] = sliceFromAny(raw["world_rules"])
	out["physical_conditions"] = sliceFromAny(raw["physical_conditions"])
	out["entity_conditions"] = sliceFromAny(raw["entity_conditions"])
	protectedSecrets := normalizeProtectedSecrets(raw["protected_secrets"])
	characterIdentityAccuracy := normalizeCharacterIdentityAccuracy(raw["character_identity_accuracy"])
	subjectiveMemories := normalizeSubjectiveEntityMemories(raw["subjective_entity_memories"])
	subjectiveMemories = appendProtectedSecretSubjectiveMemories(subjectiveMemories, protectedSecrets)
	subjectiveMemories = appendIdentityAccuracySubjectiveMemories(subjectiveMemories, characterIdentityAccuracy)
	out["protected_secrets"] = protectedSecrets
	out["character_identity_accuracy"] = characterIdentityAccuracy
	out["subjective_entity_memories"] = subjectiveMemories
	out["persona_capsule_candidates"] = normalizePersonaCapsuleCandidates(raw["persona_capsule_candidates"])
	return out
}

func enrichNormalizedCriticExtractionForFocusedRecall(extraction map[string]any, userInput, assistantContent string, turnIndex int) map[string]any {
	if extraction == nil {
		extraction = map[string]any{}
	}
	extraction["turn_summary"] = normalizeCriticTurnSummary(extraction["turn_summary"])
	if strings.TrimSpace(extractionStringFromAny(extraction["turn_summary"])) == "" {
		if summary := focusedRecallFallbackSummary(userInput, assistantContent); summary != "" {
			extraction["turn_summary"] = summary
		}
	}
	if len(stringsFromAny(extraction["evidence_excerpts"])) == 0 {
		if excerpts := focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent); len(excerpts) > 0 {
			extraction["evidence_excerpts"] = excerpts
			extraction["focused_recall_fallback"] = map[string]any{
				"policy_version": "focused_recall_fallback.v1",
				"source":         "latest_turn_exact_excerpts",
				"turn_index":     turnIndex,
				"reason":         "critic_returned_no_evidence_excerpts",
			}
		}
	}
	return extraction
}

func normalizeCriticTurnSummary(value any) string {
	if value == nil || isStructuredCriticTurnSummaryValue(value) {
		return ""
	}
	text := strings.TrimSpace(extractionStringFromAny(value))
	if looksLikeStructuredCriticPayloadText(text) {
		return ""
	}
	return text
}

func isStructuredCriticTurnSummaryValue(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func looksLikeStructuredCriticPayloadText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "map[") && !strings.HasPrefix(trimmed, "[") {
		return false
	}
	lower := strings.ToLower(trimmed)
	hits := 0
	for _, marker := range []string{
		"archive_hint",
		"character_deltas",
		"entity_conditions",
		"evidence_excerpts",
		"kg_triples",
		"pending_threads",
		"physical_conditions",
		"relationship_memory",
		"state_deltas",
		"subjective_entity_memories",
		"turn_summary",
		"world_rules",
	} {
		if strings.Contains(lower, marker) {
			hits++
		}
	}
	return hits >= 2
}

func focusedRecallFallbackSummary(userInput, assistantContent string) string {
	user := focusedRecallFirstExcerpt(userInput, 220)
	assistant := focusedRecallFirstExcerpt(assistantContent, 360)
	parts := []string{}
	if user != "" {
		parts = append(parts, "user: "+user)
	}
	if assistant != "" {
		parts = append(parts, "assistant: "+assistant)
	}
	return truncateRunes(strings.Join(parts, " / "), 700)
}

func focusedRecallFallbackEvidenceExcerpts(userInput, assistantContent string) []string {
	out := []string{}
	add := func(text string) {
		for _, excerpt := range focusedRecallExcerptCandidates(text) {
			if excerpt == "" || containsStringFold(out, excerpt) {
				continue
			}
			out = append(out, excerpt)
			if len(out) >= 3 {
				return
			}
		}
	}
	add(userInput)
	if len(out) < 3 {
		add(assistantContent)
	}
	return out
}

func focusedRecallFirstExcerpt(text string, limit int) string {
	for _, item := range focusedRecallExcerptCandidates(text) {
		return truncateRunes(item, limit)
	}
	return ""
}

func focusedRecallExcerptCandidates(text string) []string {
	text = strings.TrimSpace(sanitizeCriticStorageText(text))
	if text == "" {
		return nil
	}
	candidates := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, piece := range splitFocusedRecallLine(line) {
			piece = strings.TrimSpace(piece)
			if !looksLikeFocusedRecallExcerpt(piece) {
				continue
			}
			candidates = append(candidates, truncateRunes(piece, 240))
			if len(candidates) >= 4 {
				return candidates
			}
		}
	}
	if len(candidates) == 0 && looksLikeFocusedRecallExcerpt(text) {
		candidates = append(candidates, truncateRunes(text, 240))
	}
	return candidates
}

func splitFocusedRecallLine(line string) []string {
	out := []string{}
	start := 0
	runes := []rune(line)
	for i, r := range runes {
		switch r {
		case '.', '!', '?', '。', '！', '？', '…':
			if i+1-start >= 12 {
				out = append(out, string(runes[start:i+1]))
				start = i + 1
			}
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

func looksLikeFocusedRecallExcerpt(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	runeLen := len([]rune(text))
	if runeLen < 8 {
		return false
	}
	lower := strings.ToLower(text)
	blocked := []string{"```", "archive center", "auxiliary context", "direct evidence", "latest direct evidence", "recent raw turn"}
	for _, item := range blocked {
		if strings.Contains(lower, item) {
			return false
		}
	}
	return true
}

func containsStringFold(items []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, item := range items {
		if strings.TrimSpace(strings.ToLower(item)) == target {
			return true
		}
	}
	return false
}

func appendUniqueTurnRoleText(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	for _, part := range strings.Split(existing, "\n") {
		if strings.EqualFold(strings.TrimSpace(part), next) {
			return existing
		}
	}
	return existing + "\n" + next
}

func normalizeSubjectiveEntityMemories(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		memory := mapFromAny(item)
		ownerName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "owner_entity_name"),
			stringFromMap(memory, "entity_name"),
			stringFromMap(memory, "name"),
			stringFromMap(memory, "persona_entity_name"),
		))
		ownerKey := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "owner_entity_key"),
			stringFromMap(memory, "entity_key"),
			stringFromMap(memory, "persona_entity_key"),
			normalizeCharacterKey(ownerName),
		))
		text := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "memory_text"),
			stringFromMap(memory, "subjective_memory"),
			stringFromMap(memory, "recollection"),
			stringFromMap(memory, "interpretation"),
			stringFromMap(memory, "summary"),
			stringFromMap(memory, "text"),
		))
		if ownerKey == "" || text == "" {
			continue
		}
		if ownerName == "" {
			ownerName = ownerKey
		}
		role := normalizeSubjectiveEntityRoleFilter(stringFromMap(memory, "owner_entity_role"))
		if role == "" {
			role = normalizeSubjectiveEntityRoleFilter(stringFromMap(memory, "entity_role"))
		}
		if role == "" {
			role = "protagonist"
		}
		visibility := normalizeSubjectiveEntityVisibilityFilter(stringFromMap(memory, "owner_visibility"))
		if visibility == "" {
			visibility = normalizeSubjectiveEntityVisibilityFilter(stringFromMap(memory, "visibility"))
		}
		if visibility == "" && role == "npc" {
			visibility = "owner_private"
		}
		if visibility == "" {
			visibility = "player_known"
		}
		targetRevealPolicy := normalizeTargetRevealPolicy(stringFromMap(memory, "target_reveal_policy"))
		if strings.TrimSpace(stringFromMap(memory, "target_reveal_policy")) == "" && (role == "npc" || visibility == "owner_private") {
			targetRevealPolicy = "owner_private_until_revealed"
		}
		portability := strings.ToLower(strings.TrimSpace(stringFromMap(memory, "portability")))
		switch portability {
		case "portable_subjective_entity_recollection", "portable_persona_recollection", "npc_private_recollection":
		default:
			if role == "npc" || visibility == "owner_private" {
				portability = "npc_private_recollection"
			} else {
				portability = "portable_subjective_entity_recollection"
			}
		}
		out = append(out, map[string]any{
			"owner_entity_key":     ownerKey,
			"owner_entity_name":    ownerName,
			"owner_entity_role":    role,
			"owner_visibility":     visibility,
			"memory_text":          text,
			"source_turn_index":    intFromAny(memory["source_turn_index"], 0),
			"importance_10":        clampFloat(extractionFloatFromAny(memory["importance_10"], extractionFloatFromAny(memory["importance_score"], 5)), 1, 10),
			"emotional_weight":     clampFloat(extractionFloatFromAny(memory["emotional_weight"], extractionFloatFromAny(memory["emotional_intensity"], 0.5)), 0, 1),
			"evidence_excerpt":     strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "evidence_excerpt"), stringFromMap(memory, "evidence"))),
			"secret_guard":         boolFromAny(memory["secret_guard"]),
			"target_reveal_policy": targetRevealPolicy,
			"tags":                 stringsFromAny(memory["tags"]),
			"portability":          portability,
		})
	}
	return out
}

func normalizeProtectedSecrets(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		secret := mapFromAny(item)
		kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(
			stringFromMap(secret, "secret_kind"),
			stringFromMap(secret, "kind"),
			stringFromMap(secret, "protected_secret_type"),
		))
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(secret, "owner"),
			stringFromMap(secret, "owner_entity_name"),
			stringFromMap(secret, "character_name"),
			firstStringFromAny(mapFromAny(secret["knowledge_scope"])["known_by"]),
		))
		subjects := stringsFromAny(secret["subject"])
		if len(subjects) == 0 {
			subject := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(secret, "subject"),
				stringFromMap(secret, "target"),
				stringFromMap(secret, "topic"),
			))
			if subject != "" {
				subjects = []string{subject}
			}
		}
		summary := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(secret, "summary"),
			stringFromMap(secret, "memory_text"),
			stringFromMap(secret, "secret_summary"),
			stringFromMap(secret, "text"),
		))
		if owner == "" && len(subjects) > 0 {
			owner = subjects[0]
		}
		if summary == "" || owner == "" {
			continue
		}
		knowledgeScope := normalizeProtectedSecretKnowledgeScope(secret["knowledge_scope"], owner)
		disclosurePolicy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(
			stringFromMap(secret, "disclosure_policy"),
			stringFromMap(secret, "target_reveal_policy"),
			stringFromMap(secret, "reveal_policy"),
		))
		if disclosurePolicy == "" || disclosurePolicy == "requires_explicit_attachment" {
			disclosurePolicy = "owner_private_until_revealed"
		}
		out = append(out, map[string]any{
			"contract_version":         "protected_secret.v1",
			"secret_kind":              firstNonEmpty(kind, "other"),
			"owner":                    owner,
			"subject":                  subjects,
			"summary":                  summary,
			"sensitivity":              normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")),
			"evidence_strength":        normalizeProtectedSecretToken(stringFromMap(secret, "evidence_strength")),
			"disclosure_policy":        disclosurePolicy,
			"knowledge_scope":          knowledgeScope,
			"evidence_excerpt":         strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(secret, "evidence_excerpt"), stringFromMap(secret, "evidence"))),
			"raw_evidence_rewritten":   false,
			"public_narration_allowed": boolFromAny(secret["public_narration_allowed"]),
		})
	}
	return out
}

func normalizeCharacterIdentityAccuracy(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		identity := mapFromAny(item)
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
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			trueName,
			surface,
		))
		kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(
			stringFromMap(identity, "identity_kind"),
			stringFromMap(identity, "kind"),
			stringFromMap(identity, "protected_secret_type"),
		))
		if owner == "" || (surface == "" && trueName == "" && kind == "") {
			continue
		}
		revealPolicy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(
			stringFromMap(identity, "reveal_policy"),
			stringFromMap(identity, "target_reveal_policy"),
			stringFromMap(identity, "disclosure_policy"),
		))
		if revealPolicy == "" || revealPolicy == "requires_explicit_attachment" {
			revealPolicy = "owner_private_until_revealed"
		}
		knowledgeScope := normalizeProtectedSecretKnowledgeScope(identity["knowledge_scope"], owner)
		out = append(out, map[string]any{
			"contract_version":       "character_identity_accuracy.v1",
			"canonical_entity_key":   normalizeCharacterKey(owner),
			"canonical_entity_name":  owner,
			"surface_identity_name":  surface,
			"true_identity_name":     trueName,
			"aliases":                stringsFromAny(identity["aliases"]),
			"identity_kind":          firstNonEmpty(kind, "identity"),
			"same_entity":            boolFromAny(identity["same_entity"]),
			"public_role":            strings.TrimSpace(stringFromMap(identity, "public_role")),
			"true_role":              strings.TrimSpace(stringFromMap(identity, "true_role")),
			"public_allegiance":      strings.TrimSpace(stringFromMap(identity, "public_allegiance")),
			"true_allegiance":        strings.TrimSpace(stringFromMap(identity, "true_allegiance")),
			"twist_sensitivity":      normalizeProtectedSecretToken(stringFromMap(identity, "twist_sensitivity")),
			"reveal_policy":          revealPolicy,
			"visibility":             extractionFirstNonEmpty(stringFromMap(identity, "visibility"), "internal_support_only"),
			"knowledge_scope":        knowledgeScope,
			"source_evidence_turns":  intsFromAny(identity["source_evidence_turns"]),
			"raw_evidence_rewritten": false,
		})
	}
	return out
}

type confirmedIdentityAliasMap struct {
	aliasToCanonical      map[string]string
	aliasesByCanonicalKey map[string][]string
	conflictedAliasKeys   map[string]bool
}

func buildConfirmedIdentityAliasMapFromExtraction(extraction map[string]any) confirmedIdentityAliasMap {
	out := confirmedIdentityAliasMap{
		aliasToCanonical:      map[string]string{},
		aliasesByCanonicalKey: map[string][]string{},
		conflictedAliasKeys:   map[string]bool{},
	}
	addAlias := func(alias, canonical string) {
		alias = strings.TrimSpace(alias)
		canonical = strings.TrimSpace(canonical)
		aliasKey := normalizeCharacterKey(alias)
		canonicalKey := normalizeCharacterKey(canonical)
		if aliasKey == "" || canonicalKey == "" || out.conflictedAliasKeys[aliasKey] {
			return
		}
		if existing, ok := out.aliasToCanonical[aliasKey]; ok && normalizeCharacterKey(existing) != canonicalKey {
			delete(out.aliasToCanonical, aliasKey)
			out.conflictedAliasKeys[aliasKey] = true
			return
		}
		out.aliasToCanonical[aliasKey] = canonical
		if aliasKey != canonicalKey {
			out.aliasesByCanonicalKey[canonicalKey] = appendUniqueIdentityAlias(out.aliasesByCanonicalKey[canonicalKey], alias)
		}
	}
	for _, raw := range sliceFromAny(extraction["character_identity_accuracy"]) {
		identity := mapFromAny(raw)
		if !boolFromAny(identity["same_entity"]) {
			continue
		}
		canonical := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "true_identity_name"),
		))
		if canonical == "" {
			continue
		}
		addAlias(canonical, canonical)
		addAlias(stringFromMap(identity, "true_identity_name"), canonical)
		addAlias(stringFromMap(identity, "surface_identity_name"), canonical)
		for _, alias := range stringsFromAny(identity["aliases"]) {
			addAlias(alias, canonical)
		}
	}
	return out
}

func (m confirmedIdentityAliasMap) empty() bool {
	return len(m.aliasToCanonical) == 0
}

func (m confirmedIdentityAliasMap) canonicalizeName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len(m.aliasToCanonical) == 0 {
		return name, false
	}
	key := normalizeCharacterKey(name)
	if key == "" || m.conflictedAliasKeys[key] {
		return name, false
	}
	canonical := strings.TrimSpace(m.aliasToCanonical[key])
	if canonical == "" || normalizeCharacterKey(canonical) == key {
		return name, false
	}
	return canonical, true
}

func (m confirmedIdentityAliasMap) aliasesForCanonical(canonical string) []string {
	key := normalizeCharacterKey(canonical)
	if key == "" {
		return nil
	}
	return append([]string{}, m.aliasesByCanonicalKey[key]...)
}

func applyConfirmedIdentityAliasCanonicalMerge(extraction map[string]any) (map[string]any, int) {
	aliases := buildConfirmedIdentityAliasMapFromExtraction(extraction)
	if aliases.empty() {
		return extraction, 0
	}
	applied := 0
	canonicalizeField := func(item map[string]any, field string) bool {
		raw := stringFromMap(item, field)
		canonical, changed := aliases.canonicalizeName(raw)
		if !changed {
			return false
		}
		item[field] = canonical
		applied++
		return true
	}
	addEntityAliases := func(entity map[string]any, rawName, canonical string) {
		values := stringsFromAny(entity["aliases"])
		values = appendUniqueIdentityAlias(values, rawName)
		for _, alias := range aliases.aliasesForCanonical(canonical) {
			values = appendUniqueIdentityAlias(values, alias)
		}
		if len(values) > 0 {
			entity["aliases"] = values
		}
		entity["identity_canonicalized"] = true
	}
	entities := mapFromAny(extraction["entities"])
	if len(entities) > 0 {
		for _, rawEntity := range sliceFromAny(entities["characters"]) {
			entity := mapFromAny(rawEntity)
			rawName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title")))
			canonical, changed := aliases.canonicalizeName(rawName)
			if !changed {
				continue
			}
			entity["name"] = canonical
			addEntityAliases(entity, rawName, canonical)
			applied++
		}
		extraction["entities"] = entities
	}
	for _, raw := range sliceFromAny(extraction["kg_triples"]) {
		triple := mapFromAny(raw)
		canonicalizeField(triple, "subject")
		canonicalizeField(triple, "object")
	}
	for _, raw := range sliceFromAny(extraction["character_deltas"]) {
		delta := mapFromAny(raw)
		if rawName := stringFromMap(delta, "name"); canonicalizeField(delta, "name") {
			delta["aliases"] = appendUniqueIdentityAlias(stringsFromAny(delta["aliases"]), rawName)
		}
	}
	for _, raw := range sliceFromAny(extraction["subjective_entity_memories"]) {
		memory := mapFromAny(raw)
		rawOwnerName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "owner_entity_name"), stringFromMap(memory, "persona_entity_name")))
		rawOwnerKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "owner_entity_key"), stringFromMap(memory, "persona_entity_key")))
		canonical, changed := aliases.canonicalizeName(firstNonEmpty(rawOwnerName, rawOwnerKey))
		if !changed {
			continue
		}
		canonicalKey := normalizeCharacterKey(canonical)
		memory["owner_entity_name"] = canonical
		memory["persona_entity_name"] = canonical
		memory["owner_entity_key"] = canonicalKey
		memory["persona_entity_key"] = canonicalKey
		tags := stringsFromAny(memory["tags"])
		tags = appendUniqueIdentityAlias(tags, "confirmed_identity_alias_canonicalized")
		if rawOwnerName != "" {
			tags = appendUniqueIdentityAlias(tags, "raw_owner_entity_name:"+rawOwnerName)
		}
		if rawOwnerKey != "" {
			tags = appendUniqueIdentityAlias(tags, "raw_owner_entity_key:"+rawOwnerKey)
		}
		for _, alias := range aliases.aliasesForCanonical(canonical) {
			tags = appendUniqueIdentityAlias(tags, "owner_entity_alias:"+alias)
		}
		memory["tags"] = tags
		applied++
	}
	for _, raw := range sliceFromAny(extraction["protected_secrets"]) {
		secret := mapFromAny(raw)
		canonicalizeField(secret, "owner")
		subjects := stringsFromAny(secret["subject"])
		for idx, subject := range subjects {
			if canonical, changed := aliases.canonicalizeName(subject); changed {
				subjects[idx] = canonical
				applied++
			}
		}
		if len(subjects) > 0 {
			secret["subject"] = subjects
		}
		scope := mapFromAny(secret["knowledge_scope"])
		if len(scope) > 0 {
			for _, key := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
				values := stringsFromAny(scope[key])
				changed := false
				for idx, value := range values {
					if canonical, ok := aliases.canonicalizeName(value); ok {
						values[idx] = canonical
						changed = true
						applied++
					}
				}
				if changed {
					scope[key] = values
				}
			}
			secret["knowledge_scope"] = scope
		}
	}
	if applied > 0 {
		extraction["confirmed_identity_alias_canonical_merge"] = map[string]any{
			"contract_version": "character_identity_canonical_merge.v1",
			"applied":          true,
			"applied_count":    applied,
			"conflicts":        len(aliases.conflictedAliasKeys),
		}
	}
	return extraction, applied
}

func appendUniqueIdentityAlias(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return items
		}
	}
	return append(items, value)
}

func appendProtectedSecretSubjectiveMemories(existing []any, secrets []any) []any {
	out := append([]any{}, existing...)
	for _, raw := range secrets {
		secret := mapFromAny(raw)
		owner := strings.TrimSpace(stringFromMap(secret, "owner"))
		summary := strings.TrimSpace(stringFromMap(secret, "summary"))
		if owner == "" || summary == "" {
			continue
		}
		out = appendSubjectiveMemoryIfMissing(out, map[string]any{
			"owner_entity_key":     normalizeCharacterKey(owner),
			"owner_entity_name":    owner,
			"owner_entity_role":    "npc",
			"owner_visibility":     "owner_private",
			"memory_text":          summary,
			"importance_10":        protectedSecretImportance(secret),
			"emotional_weight":     protectedSecretEmotionalWeight(secret),
			"evidence_excerpt":     strings.TrimSpace(stringFromMap(secret, "evidence_excerpt")),
			"secret_guard":         true,
			"target_reveal_policy": normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")),
			"tags":                 protectedSecretTags(secret),
			"portability":          "npc_private_recollection",
		})
	}
	return out
}

func appendIdentityAccuracySubjectiveMemories(existing []any, identities []any) []any {
	out := append([]any{}, existing...)
	for _, raw := range identities {
		identity := mapFromAny(raw)
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "surface_identity_name"),
		))
		if owner == "" {
			continue
		}
		out = appendSubjectiveMemoryIfMissing(out, map[string]any{
			"owner_entity_key":     normalizeCharacterKey(owner),
			"owner_entity_name":    owner,
			"owner_entity_role":    "npc",
			"owner_visibility":     "owner_private",
			"memory_text":          protectedIdentityGuardSummary(identity),
			"importance_10":        8.0,
			"emotional_weight":     0.7,
			"secret_guard":         true,
			"target_reveal_policy": normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")),
			"tags":                 protectedIdentityTags(identity),
			"portability":          "npc_private_recollection",
		})
	}
	return out
}

func appendSubjectiveMemoryIfMissing(items []any, next map[string]any) []any {
	owner := strings.ToLower(strings.TrimSpace(stringFromMap(next, "owner_entity_key")))
	text := strings.ToLower(strings.TrimSpace(stringFromMap(next, "memory_text")))
	for _, raw := range items {
		item := mapFromAny(raw)
		if strings.ToLower(strings.TrimSpace(stringFromMap(item, "owner_entity_key"))) == owner &&
			strings.ToLower(strings.TrimSpace(stringFromMap(item, "memory_text"))) == text {
			return items
		}
	}
	return append(items, next)
}

func normalizeProtectedSecretKnowledgeScope(raw any, owner string) map[string]any {
	scope := mapFromAny(raw)
	out := map[string]any{
		"publicly_revealed":   boolFromAny(scope["publicly_revealed"]),
		"known_by":            stringsFromAny(scope["known_by"]),
		"unknown_to":          stringsFromAny(scope["unknown_to"]),
		"suspected_by":        stringsFromAny(scope["suspected_by"]),
		"misinformed_by":      stringsFromAny(scope["misinformed_by"]),
		"revealed_to":         stringsFromAny(scope["revealed_to"]),
		"reader_visible":      boolFromAny(scope["reader_visible"]),
		"protagonist_visible": boolFromAny(scope["protagonist_visible"]),
	}
	if len(stringsFromAny(out["known_by"])) == 0 && strings.TrimSpace(owner) != "" {
		out["known_by"] = []string{owner}
	}
	return out
}

func protectedSecretTags(secret map[string]any) []string {
	tags := []string{"protected_secret", "secret_guard"}
	if kind := normalizeProtectedSecretToken(stringFromMap(secret, "secret_kind")); kind != "" {
		tags = append(tags, "protected_secret_kind:"+kind)
	}
	if sensitivity := normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")); sensitivity != "" {
		tags = append(tags, "sensitivity:"+sensitivity)
	}
	if policy := normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")); policy != "" {
		tags = append(tags, "target_reveal_policy:"+policy)
	}
	return tags
}

func protectedIdentityTags(identity map[string]any) []string {
	tags := []string{"protected_secret", "character_identity_accuracy", "secret_guard"}
	if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
		tags = append(tags, "identity_kind:"+kind)
		tags = append(tags, "protected_secret_kind:"+kind)
	}
	if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
		tags = append(tags, "target_reveal_policy:"+policy)
	}
	return tags
}

func protectedIdentityGuardSummary(identity map[string]any) string {
	kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind"))
	if kind == "" {
		kind = "identity"
	}
	return "Protected identity/role knowledge is present; preserve same-entity continuity internally, but do not reveal, confess, or grant knowledge without current-scene evidence. kind=" + kind
}

func protectedSecretImportance(secret map[string]any) float64 {
	switch normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")) {
	case "critical":
		return 9
	case "high":
		return 8
	case "medium":
		return 6
	default:
		return 5
	}
}

func protectedSecretEmotionalWeight(secret map[string]any) float64 {
	switch normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")) {
	case "critical":
		return 0.9
	case "high":
		return 0.75
	case "medium":
		return 0.55
	default:
		return 0.35
	}
}

func normalizeProtectedSecretToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func firstStringFromAny(value any) string {
	values := stringsFromAny(value)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func intsFromAny(value any) []int {
	out := []int{}
	for _, item := range sliceFromAny(value) {
		n := intFromAny(item, 0)
		if n != 0 {
			out = append(out, n)
		}
	}
	return out
}

type subjectiveEntityOwnerCanonical struct {
	Key       string
	Name      string
	AliasTags []string
	Changed   bool
}

func (s *Server) canonicalSubjectiveEntityOwner(ctx context.Context, sid, rawKey, rawName string) subjectiveEntityOwnerCanonical {
	rawKey = strings.TrimSpace(rawKey)
	rawName = strings.TrimSpace(rawName)
	proposed := strings.TrimSpace(firstNonEmpty(rawName, rawKey))
	canonicalName := proposed
	if proposed != "" {
		canonicalName = strings.TrimSpace(s.canonicalCharacterName(ctx, sid, proposed))
	}
	if canonicalName == "" {
		canonicalName = proposed
	}
	canonicalKey := canonicalCharacterAliasKey(canonicalName)
	if canonicalKey == "" {
		canonicalKey = canonicalCharacterAliasKey(rawKey)
	}
	if canonicalKey == "" {
		canonicalKey = canonicalCharacterAliasKey(rawName)
	}
	if canonicalKey == "" {
		canonicalKey = rawKey
	}
	if canonicalName == "" {
		canonicalName = canonicalKey
	}
	out := subjectiveEntityOwnerCanonical{
		Key:  canonicalKey,
		Name: canonicalName,
	}
	addAlias := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if prefix == "owner_entity_alias:" && strings.EqualFold(value, canonicalName) {
			return
		}
		if prefix == "owner_entity_alias_key:" && value == canonicalKey {
			return
		}
		tag := prefix + value
		for _, existing := range out.AliasTags {
			if existing == tag {
				return
			}
		}
		out.AliasTags = append(out.AliasTags, tag)
		out.Changed = true
	}
	addAlias("owner_entity_alias:", rawName)
	addAlias("owner_entity_alias_key:", rawKey)
	return out
}

func (s *Server) canonicalizeSubjectiveEntityMemoryForRead(ctx context.Context, sid string, memory store.ProtagonistEntityMemory) store.ProtagonistEntityMemory {
	owner := s.canonicalSubjectiveEntityOwner(ctx, sid, firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey), firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName))
	if owner.Key == "" {
		return memory
	}
	memory.OwnerEntityKey = owner.Key
	memory.PersonaEntityKey = owner.Key
	if owner.Name != "" {
		memory.OwnerEntityName = owner.Name
		memory.PersonaEntityName = owner.Name
		if strings.TrimSpace(memory.SourceCharacterName) == "" {
			memory.SourceCharacterName = owner.Name
		}
	}
	return memory
}

func (s *Server) canonicalizeSubjectiveEntityMemoriesForRead(ctx context.Context, sid string, memories []store.ProtagonistEntityMemory) []store.ProtagonistEntityMemory {
	out := make([]store.ProtagonistEntityMemory, 0, len(memories))
	for _, memory := range memories {
		out = append(out, s.canonicalizeSubjectiveEntityMemoryForRead(ctx, sid, memory))
	}
	return out
}

func normalizePersonaCapsuleCandidates(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		candidate := mapFromAny(item)
		memoryText := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(candidate, "memory_text"),
			stringFromMap(candidate, "summary"),
			stringFromMap(candidate, "text"),
		))
		evidence := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(candidate, "evidence_excerpt"),
			stringFromMap(candidate, "evidence"),
		))
		if memoryText == "" || evidence == "" {
			continue
		}
		portability := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "portability")))
		switch portability {
		case "same_session", "cross_session", "cross_world", "cross_chat":
		default:
			portability = "cross_session"
		}
		mode := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "mode")))
		switch mode {
		case "subtle_deja_vu", "full_loop_memory", "isekai_carryover", "same_character_continuation", "regression_recollection", "reincarnation_carryover":
		default:
			mode = "same_character_continuation"
		}
		injectionPolicy := strings.TrimSpace(stringFromMap(candidate, "injection_policy"))
		if injectionPolicy == "" {
			injectionPolicy = "support_only_persona_recollection"
		}
		normalized := map[string]any{
			"memory_text":       memoryText,
			"source_turn_index": intFromAny(candidate["source_turn_index"], 0),
			"importance_10":     clampFloat(extractionFloatFromAny(candidate["importance_10"], extractionFloatFromAny(candidate["importance_score"], 5)), 1, 10),
			"emotional_weight":  clampFloat(extractionFloatFromAny(candidate["emotional_weight"], extractionFloatFromAny(candidate["emotional_intensity"], 0.5)), 0, 1),
			"portability":       portability,
			"mode":              mode,
			"secret_guard":      boolFromAny(candidate["secret_guard"]),
			"tags":              stringsFromAny(candidate["tags"]),
			"evidence_excerpt":  evidence,
			"injection_policy":  injectionPolicy,
		}
		out = append(out, normalized)
	}
	return out
}

func recordPersonaCapsuleCandidateTrace(extraction map[string]any, turnIndex int, result *artifactSaveResult) {
	if result == nil {
		return
	}
	candidates := sliceFromAny(extraction["persona_capsule_candidates"])
	if len(candidates) == 0 {
		return
	}
	result.PersonaCapsuleCandidates = len(candidates)
	result.Warnings = append(result.Warnings, "persona_capsule_candidates_detected:auto_create_disabled")
	for idx, item := range candidates {
		candidate := mapFromAny(item)
		result.addSkipReason("persona_capsule_candidates", "requires_explicit_user_or_operator_approval", map[string]any{
			"candidate_index":   idx,
			"source_turn_index": intFromAny(candidate["source_turn_index"], turnIndex),
			"portability":       stringFromMap(candidate, "portability"),
			"mode":              stringFromMap(candidate, "mode"),
			"injection_policy":  stringFromMap(candidate, "injection_policy"),
			"secret_guard":      boolFromAny(candidate["secret_guard"]),
		})
	}
}

func (s *Server) saveSubjectiveEntityMemoriesFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, now time.Time, result *artifactSaveResult) {
	if result == nil {
		return
	}
	items := sliceFromAny(extraction["subjective_entity_memories"])
	if len(items) == 0 {
		return
	}
	if s.Store == nil {
		result.addSkipReason("subjective_entity_memories", "store_unavailable", map[string]any{"count": len(items)})
		return
	}
	st, ok := s.Store.(store.ProtagonistEntityMemoryStore)
	if !ok {
		result.addSkipReason("subjective_entity_memories", "store_not_supported", map[string]any{"count": len(items)})
		return
	}
	for idx, raw := range items {
		item := mapFromAny(raw)
		rawOwnerKey := strings.TrimSpace(stringFromMap(item, "owner_entity_key"))
		rawOwnerName := strings.TrimSpace(stringFromMap(item, "owner_entity_name"))
		ownerKey := rawOwnerKey
		ownerName := rawOwnerName
		memoryText := strings.TrimSpace(stringFromMap(item, "memory_text"))
		if ownerKey == "" || memoryText == "" {
			result.addSkipReason("subjective_entity_memories", "missing_owner_or_memory_text", map[string]any{"index": idx})
			continue
		}
		if ownerName == "" {
			ownerName = ownerKey
		}
		canonicalOwner := s.canonicalSubjectiveEntityOwner(ctx, sid, ownerKey, ownerName)
		ownerKey = canonicalOwner.Key
		ownerName = canonicalOwner.Name
		if ownerKey == "" {
			result.addSkipReason("subjective_entity_memories", "missing_owner_or_memory_text", map[string]any{"index": idx})
			continue
		}
		sourceTurn := intFromAny(item["source_turn_index"], turnIndex)
		if sourceTurn <= 0 {
			sourceTurn = turnIndex
		}
		evidence := strings.TrimSpace(stringFromMap(item, "evidence_excerpt"))
		if evidence != "" {
			grounded := sanitizeEvidenceExcerptForTurn(evidence, content)
			if grounded == "" {
				result.addSkipReason("subjective_entity_memories", "evidence_excerpt_not_grounded", map[string]any{"index": idx, "owner_entity_key": ownerKey})
			}
			evidence = grounded
		}
		if subjectiveEntityMemoryAlreadyExists(ctx, st, sid, ownerKey, sourceTurn, memoryText) {
			result.addSkipReason("subjective_entity_memories", "duplicate_source_turn_owner_memory", map[string]any{
				"index":            idx,
				"owner_entity_key": ownerKey,
				"source_turn":      sourceTurn,
			})
			continue
		}
		ownerRole := normalizeSubjectiveEntityRoleFilter(stringFromMap(item, "owner_entity_role"))
		if ownerRole == "" {
			ownerRole = "protagonist"
		}
		ownerVisibility := normalizeSubjectiveEntityVisibilityFilter(stringFromMap(item, "owner_visibility"))
		if ownerVisibility == "" && ownerRole == "npc" {
			ownerVisibility = "owner_private"
		}
		if ownerVisibility == "" {
			ownerVisibility = "player_known"
		}
		portability := strings.TrimSpace(stringFromMap(item, "portability"))
		if portability == "" {
			if ownerRole == "npc" || ownerVisibility == "owner_private" {
				portability = "npc_private_recollection"
			} else {
				portability = "portable_subjective_entity_recollection"
			}
		}
		targetRevealPolicy := strings.TrimSpace(stringFromMap(item, "target_reveal_policy"))
		if targetRevealPolicy == "" {
			if ownerRole == "npc" || ownerVisibility == "owner_private" {
				targetRevealPolicy = "owner_private_until_revealed"
			} else {
				targetRevealPolicy = "requires_explicit_attachment"
			}
		}
		ownerTags := append([]string{}, stringsFromAny(item["tags"])...)
		ownerTags = append(ownerTags,
			"subjective_entity_memory",
			"owner_entity_key:"+ownerKey,
			"owner_entity_name:"+ownerName,
			"owner_entity_role:"+ownerRole,
			"owner_visibility:"+ownerVisibility,
		)
		ownerTags = append(ownerTags, canonicalOwner.AliasTags...)
		if canonicalOwner.Changed {
			ownerTags = append(ownerTags, "entity_alias_canonicalized")
		}
		if rawOwnerKey != "" && rawOwnerKey != ownerKey {
			ownerTags = append(ownerTags, "raw_owner_entity_key:"+rawOwnerKey)
		}
		if rawOwnerName != "" && rawOwnerName != ownerName {
			ownerTags = append(ownerTags, "raw_owner_entity_name:"+rawOwnerName)
		}
		if boolFromAny(item["secret_guard"]) {
			ownerTags = append(ownerTags, "secret_guard")
			ownerTags = append(ownerTags, "protected_secret")
		}
		for _, tag := range protectedSecretTagsFromSubjectiveItem(item) {
			ownerTags = append(ownerTags, tag)
		}
		result.trySave("CreateProtagonistEntityMemory(subjective_entity_memories)", func() error {
			_, err := st.CreateProtagonistEntityMemory(ctx, &store.ProtagonistEntityMemory{
				PersonaEntityKey:    ownerKey,
				PersonaEntityName:   ownerName,
				OwnerEntityKey:      ownerKey,
				OwnerEntityName:     ownerName,
				OwnerEntityRole:     ownerRole,
				OwnerVisibility:     ownerVisibility,
				SourceChatSessionID: sid,
				SourceCharacterName: ownerName,
				SourceTurn:          sourceTurn,
				MemoryText:          memoryText,
				EvidenceExcerpt:     evidence,
				SecretGuard:         boolFromAny(item["secret_guard"]),
				Portability:         portability,
				TargetRevealPolicy:  normalizeTargetRevealPolicy(targetRevealPolicy),
				TagsJSON:            mustCompactJSON(ownerTags),
				Importance10:        clampFloat(extractionFloatFromAny(item["importance_10"], 5), 1, 10),
				EmotionalWeight:     clampFloat(extractionFloatFromAny(item["emotional_weight"], 0.5), 0, 1),
				CreatedAt:           now,
				UpdatedAt:           now,
			})
			return err
		}, result, func() { result.SubjectiveEntityMemories++ })
	}
}

func protectedSecretTagsFromSubjectiveItem(item map[string]any) []string {
	out := []string{}
	if kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(stringFromMap(item, "secret_kind"), stringFromMap(item, "protected_secret_kind"))); kind != "" {
		out = append(out, "protected_secret_kind:"+kind)
	}
	if policy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(stringFromMap(item, "target_reveal_policy"), stringFromMap(item, "disclosure_policy"))); policy != "" {
		out = append(out, "target_reveal_policy:"+policy)
	}
	return out
}

func subjectiveEntityMemoryAlreadyExists(ctx context.Context, st store.ProtagonistEntityMemoryStore, sid, ownerKey string, sourceTurn int, memoryText string) bool {
	existing, err := st.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{
		OwnerEntityKey:      ownerKey,
		SourceChatSessionID: sid,
		Limit:               80,
	})
	if err != nil {
		return false
	}
	normalizedText := strings.TrimSpace(strings.ToLower(memoryText))
	for _, item := range existing {
		if item.SourceTurn != sourceTurn {
			continue
		}
		if strings.TrimSpace(strings.ToLower(item.MemoryText)) == normalizedText {
			return true
		}
	}
	return false
}

func emotionalImportanceBoost(emotionalIntensity float64) float64 {
	if emotionalIntensity >= 0.90 {
		return 2.0
	}
	if emotionalIntensity >= 0.70 {
		return 1.0
	}
	return 0
}

func simpleTokenSimilarity(a, b string) float64 {
	aTokens := map[string]int{}
	bTokens := map[string]int{}
	for _, t := range strings.Fields(strings.ToLower(a)) {
		aTokens[t]++
	}
	for _, t := range strings.Fields(strings.ToLower(b)) {
		bTokens[t]++
	}
	if len(aTokens) == 0 && len(bTokens) == 0 {
		return 1.0
	}
	intersection := 0
	for t, ca := range aTokens {
		cb := bTokens[t]
		if ca < cb {
			intersection += ca
		} else {
			intersection += cb
		}
	}
	union := len(aTokens) + len(bTokens) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func saveCanonicalStateLayerWithCost(ctx context.Context, clSaver canonicalStateLayerSaver, sid string, layer *store.CanonicalStateLayer, existing []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement) error {
	var prev *store.CanonicalStateLayer
	for i := range existing {
		e := &existing[i]
		if e.LayerType != layer.LayerType {
			continue
		}
		if prev == nil || e.TurnIndex > prev.TurnIndex {
			prev = e
		}
	}
	prevChars := 0
	similarity := 0.0
	if prev != nil {
		prevChars = len([]rune(prev.Content))
		similarity = simpleTokenSimilarity(prev.Content, layer.Content)
	}
	newChars := len([]rune(layer.Content))
	charDelta := newChars - prevChars
	if charDelta < 0 {
		charDelta = -charDelta
	}

	var mode string
	if prev == nil {
		mode = "full_rewrite_bootstrap"
		cost.FullRewriteCount++
	} else if similarity >= 0.55 {
		mode = "delta_update"
		cost.DeltaUpdateCount++
	} else {
		mode = "full_rewrite"
		cost.FullRewriteCount++
	}

	start := time.Now()
	err := clSaver.SaveCanonicalStateLayer(ctx, layer)
	latencyMs := time.Since(start).Milliseconds()

	cost.StateWriteCount++
	cost.TotalWriteChars += newChars
	cost.TotalElapsedMs += latencyMs
	cost.Items = append(cost.Items, map[string]any{
		"layer_type":             layer.LayerType,
		"write_mode":             mode,
		"write_latency_ms":       latencyMs,
		"previous_content_chars": prevChars,
		"new_content_chars":      newChars,
		"char_delta_abs":         charDelta,
		"token_similarity":       similarity,
	})
	return err
}

func finalizeCanonicalStateWriteCost(cost *canonicalStateWriteCostMeasurement) {
	if cost == nil || cost.StateWriteCount == 0 {
		return
	}
	cost.PolicyVersion = "lc1b.v1"
	var sum int64
	latencies := make([]int64, 0, len(cost.Items))
	for _, item := range cost.Items {
		l, _ := item["write_latency_ms"].(int64)
		sum += l
		latencies = append(latencies, l)
	}
	cost.AvgWriteLatencyMs = float64(sum) / float64(cost.StateWriteCount)
	if len(latencies) > 0 {
		// simple p95: sort and pick 95th percentile index
		for i := 0; i < len(latencies); i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}
		idx := int(float64(len(latencies)-1) * 0.95)
		if idx < 0 {
			idx = 0
		}
		cost.P95WriteLatencyMs = float64(latencies[idx])
	}
}

func (s *Server) saveCriticExtractionArtifacts(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, embCfg completeTurnEmbeddingConfig, now time.Time, existingEvidenceArg ...[]store.DirectEvidence) artifactSaveResult {
	result := artifactSaveResult{EmbeddingStatus: "not_requested", VectorStatus: "not_requested"}
	cost := &canonicalStateWriteCostMeasurement{}
	var existingCanonicalLayers []store.CanonicalStateLayer
	if s.Store != nil {
		existingCanonicalLayers, _ = s.Store.ListCanonicalStateLayers(ctx, sid, "")
	}
	existingEvidence := []store.DirectEvidence{}
	if len(existingEvidenceArg) > 0 {
		existingEvidence = existingEvidenceArg[0]
	} else if s.Store != nil {
		existingEvidence, _ = s.Store.ListEvidence(ctx, sid)
	}
	existingKGTriples := []store.KGTriple{}
	if s.Store != nil {
		existingKGTriples, _ = s.Store.ListKGTriples(ctx, sid)
	}
	rawTurnSummary := extraction["turn_summary"]
	summary := normalizeCriticTurnSummary(rawTurnSummary)
	if summary == "" && (isStructuredCriticTurnSummaryValue(rawTurnSummary) || looksLikeStructuredCriticPayloadText(extractionStringFromAny(rawTurnSummary))) {
		if fallback := strings.Join(strings.Fields(content), " "); fallback != "" {
			summary = fallback
			extraction["turn_summary"] = fallback
			result.Warnings = append(result.Warnings, "turn_summary_rebuilt_from_grounded_turn_text")
		}
	} else {
		extraction["turn_summary"] = summary
	}
	languageContext := completeTurnLanguageContextFromExtraction(extraction)
	extraction = applyLanguageMemoryWriteContract(extraction, languageContext)
	if mergedExtraction, applied := applyConfirmedIdentityAliasCanonicalMerge(extraction); applied > 0 {
		extraction = mergedExtraction
		result.Warnings = append(result.Warnings, "confirmed_identity_alias_canonical_merge_applied")
	}
	memorySearchText := completeTurnMemorySearchText(summary, extraction, content)
	searchText := strings.TrimSpace(memorySearchText.Text)
	if searchText == "" {
		searchText = summary
	}
	embedding := "[]"
	embeddingModel := "not_configured"
	var embeddingVector []float32
	if embCfg.hasConfig() && searchText != "" {
		emb, model, err := callEmbedding(ctx, embCfg, searchText)
		if err != nil {
			result.EmbeddingStatus = "error: " + err.Error()
			result.Warnings = append(result.Warnings, "embedding_call_failed")
		} else {
			embedding = emb
			embeddingModel = model
			result.EmbeddingStatus = "ok"
			embeddingVector = parseFloat32JSONList(emb)
		}
	} else if summary != "" {
		result.EmbeddingStatus = "missing_config"
	}

	recordPersonaCapsuleCandidateTrace(extraction, turnIndex, &result)
	s.saveSubjectiveEntityMemoriesFromExtraction(ctx, sid, turnIndex, extraction, content, now, &result)

	if summary != "" {
		archiveHint := mapFromAny(extraction["archive_hint"])
		emotionalIntensity := clampFloat(extractionFloatFromAny(extraction["emotional_intensity"], 0), 0, 1)
		narrativeSignificance := clampFloat(extractionFloatFromAny(extraction["narrative_significance"], 0), 0, 1)
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10)
		emotionalBoost := emotionalImportanceBoost(emotionalIntensity)
		finalImportance := clampFloat(baseImportance+emotionalBoost, 1, 10)
		mem := &store.Memory{
			ChatSessionID:         sid,
			TurnIndex:             turnIndex,
			SummaryJSON:           mustCompactJSON(extraction),
			Embedding:             embedding,
			EmbeddingModel:        embeddingModel,
			Importance:            finalImportance / 10.0,
			EmotionalBoost:        emotionalBoost,
			Evidence:              mustCompactJSON(map[string]any{"evidence_excerpts": stringsFromAny(extraction["evidence_excerpts"]), "relationship_memory": extraction["relationship_memory"]}),
			EmotionalIntensity:    emotionalIntensity,
			NarrativeSignificance: narrativeSignificance,
			PlaceWing:             stringFromMap(archiveHint, "wing"),
			PlaceRoom:             stringFromMap(archiveHint, "room"),
			CreatedAt:             now,
		}
		if existingID, existingSummary := s.memoryForTurnAlreadyExists(ctx, sid, turnIndex, &result); existingID > 0 {
			result.addSkipReason("memories", "duplicate_source_turn_memory", map[string]any{
				"turn_index":       turnIndex,
				"existing_id":      existingID,
				"existing_summary": existingSummary,
				"new_summary":      summary,
			})
			result.Warnings = append(result.Warnings, "memory_duplicate_source_turn_skipped")
		} else if s.mergeSimilarMemoryInsteadOfInsert(ctx, sid, summary, mem.Importance, now, &result) {
			result.Warnings = append(result.Warnings, "memory_semantic_dedup_merged")
		} else {
			result.trySave("SaveMemory", func() error {
				return s.Store.SaveMemory(ctx, mem)
			}, &result, func() {
				result.Memories++
				s.upsertMemoryVector(ctx, sid, turnIndex, mem, searchText, embeddingVector, &result)
			})
		}
	}

	for excerptIndex, text := range stringsFromAny(extraction["evidence_excerpts"]) {
		originalText := text
		text = sanitizeEvidenceExcerptForTurn(text, content)
		if text == "" {
			result.addSkipReason("direct_evidence", "not_grounded_in_current_turn", originalText)
			continue
		}
		if directEvidenceAlreadyExistsForTurn(existingEvidence, sid, turnIndex, text) {
			result.addSkipReason("direct_evidence", "duplicate_source_turn_excerpt", map[string]any{"turn_index": turnIndex, "text": text})
			continue
		}
		ev := &store.DirectEvidence{
			ChatSessionID:        sid,
			EvidenceKind:         "turn_excerpt",
			EvidenceText:         text,
			SourceTurnStart:      turnIndex,
			SourceTurnEnd:        turnIndex,
			TurnAnchor:           turnIndex,
			ArchiveState:         "verified_direct",
			CaptureStage:         "critic_extract",
			CaptureVerification:  "verified",
			CommittedGate:        "auto_grounded_excerpt",
			LineageJSON:          mustCompactJSON(completeTurnEvidenceLineage("critic.evidence_excerpts", excerptIndex, languageContext)),
			SourceMessageIDsJSON: mustCompactJSON([]string{fmt.Sprintf("turn:%d", turnIndex)}),
			CreatedAt:            now,
		}
		baseImportance := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10) / 10.0
		result.ConflictResolutions = append(result.ConflictResolutions, resolveCanonicalConflict(*ev, existingEvidence)...)
		result.RetentionDecisions = append(result.RetentionDecisions, applyRetentionPolicy(ev, baseImportance, existingEvidence))
		result.trySave("SaveEvidence", func() error {
			return s.Store.SaveEvidence(ctx, ev)
		}, &result, func() {
			result.Evidence++
			existingEvidence = append(existingEvidence, *ev)
			s.upsertDerivedArtifactVector(ctx, sid, turnIndex, "evidence", "direct_evidence_records", ev.ID, "direct_evidence.v1", directEvidenceVectorDocumentText(*ev), embCfg, &result)
		})
	}

	for _, item := range sliceFromAny(extraction["kg_triples"]) {
		triple := mapFromAny(item)
		subject := s.canonicalCharacterName(ctx, sid, sanitizeKGPart(stringFromMap(triple, "subject")))
		predicate := sanitizeKGPredicate(stringFromMap(triple, "predicate"))
		object := s.canonicalCharacterName(ctx, sid, sanitizeKGPart(stringFromMap(triple, "object")))
		if shouldSkipKGTriple(subject, predicate, object, sid) {
			result.addSkipReason("kg_triples", "placeholder_or_control_edge", map[string]any{"subject": subject, "predicate": predicate, "object": object})
			continue
		}
		validFrom := intFromAny(triple["valid_from"], turnIndex)
		validTo := intFromAny(triple["valid_to"], 0)
		if kgTripleAlreadyExistsForTurn(existingKGTriples, sid, turnIndex, subject, predicate, object, validFrom, validTo) {
			result.addSkipReason("kg_triples", "duplicate_source_turn_triple", map[string]any{
				"turn_index": turnIndex,
				"subject":    subject,
				"predicate":  predicate,
				"object":     object,
			})
			continue
		}
		result.trySave("SaveKGTriple", func() error {
			return s.Store.SaveKGTriple(ctx, &store.KGTriple{
				ChatSessionID: sid,
				Subject:       subject,
				Predicate:     predicate,
				Object:        object,
				ValidFrom:     validFrom,
				ValidTo:       validTo,
				SourceTurn:    turnIndex,
				CreatedAt:     now,
			})
		}, &result, func() {
			result.KGTriples++
			existingKGTriples = append(existingKGTriples, store.KGTriple{
				ChatSessionID: sid,
				Subject:       subject,
				Predicate:     predicate,
				Object:        object,
				ValidFrom:     validFrom,
				ValidTo:       validTo,
				SourceTurn:    turnIndex,
			})
		})
	}

	s.saveCharacterAndStateArtifacts(ctx, sid, turnIndex, extraction, embCfg, now, &result, existingCanonicalLayers, cost)
	finalizeCanonicalStateWriteCost(cost)
	if cost.StateWriteCount > 0 {
		result.CanonicalStateWriteCost = cost
	}
	s.applyCriticSoftPrune(ctx, sid, turnIndex, extraction, now, &result)
	return result
}

func (s *Server) memoryForTurnAlreadyExists(ctx context.Context, sid string, turnIndex int, result *artifactSaveResult) (int64, string) {
	if s == nil || s.Store == nil || turnIndex <= 0 {
		return 0, ""
	}
	memories, err := s.Store.ListMemories(ctx, sid, turnIndex, turnIndex)
	if err != nil {
		if result != nil {
			result.Warnings = append(result.Warnings, "memory_duplicate_turn_check_failed")
		}
		return 0, ""
	}
	for _, mem := range memories {
		if mem.ChatSessionID != sid || mem.TurnIndex != turnIndex || mem.ID <= 0 {
			continue
		}
		summary := memorySummaryText(mem)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		return mem.ID, summary
	}
	return 0, ""
}

func (s *Server) mergeSimilarMemoryInsteadOfInsert(ctx context.Context, sid, summary string, newImportance float64, now time.Time, result *artifactSaveResult) bool {
	if s.Store == nil || strings.TrimSpace(summary) == "" {
		return false
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		result.Warnings = append(result.Warnings, "memory_semantic_dedup_list_failed")
		return false
	}
	var best *store.Memory
	bestScore := 0.0
	for i := range memories {
		mem := &memories[i]
		if mem.ID <= 0 {
			continue
		}
		existingSummary := memorySummaryText(*mem)
		if existingSummary == "" {
			continue
		}
		score := simpleTokenSimilarity(summary, existingSummary)
		if score > bestScore {
			bestScore = score
			best = mem
		}
	}
	if best == nil || bestScore < 0.78 {
		return false
	}
	if updater, ok := s.Store.(memoryImportanceUpdater); ok && newImportance > best.Importance {
		targetImportance := newImportance
		result.trySave("UpdateMemoryImportance(memory_dedup)", func() error {
			return updater.UpdateMemoryImportance(ctx, sid, best.ID, targetImportance)
		}, result, func() {})
	}
	details := map[string]any{
		"policy_version":      "p1250.memory_semantic_dedup.v1",
		"merged_memory_id":    best.ID,
		"similarity":          bestScore,
		"new_turn_summary":    summary,
		"existing_summary":    memorySummaryText(*best),
		"new_importance":      newImportance,
		"existing_importance": best.Importance,
	}
	result.trySave("SaveAuditLog(memory_semantic_dedup)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "memory_semantic_dedup",
			Source:        "critic",
			DetailsJSON:   mustCompactJSON(details),
			CreatedAt:     now,
		})
	}, result, func() {})
	return true
}

func memorySummaryText(mem store.Memory) string {
	raw := strings.TrimSpace(mem.SummaryJSON)
	if raw == "" {
		return ""
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		for _, key := range []string{"turn_summary", "summary", "memory", "text"} {
			if value := strings.TrimSpace(extractionStringFromAny(parsed[key])); value != "" {
				return value
			}
		}
	}
	return raw
}

func directEvidenceAlreadyExistsForTurn(existing []store.DirectEvidence, sid string, turnIndex int, text string) bool {
	needle := normalizeArtifactDedupeText(text)
	if needle == "" {
		return false
	}
	for _, item := range existing {
		if item.ChatSessionID != sid {
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = start
		}
		if turnIndex > 0 && start > 0 && end > 0 && (turnIndex < start || turnIndex > end) {
			continue
		}
		existingText := normalizeArtifactDedupeText(item.EvidenceText)
		if existingText == "" {
			continue
		}
		if existingText == needle ||
			strings.Contains(existingText, needle) ||
			strings.Contains(needle, existingText) ||
			simpleTokenSimilarity(existingText, needle) >= 0.86 {
			return true
		}
	}
	return false
}

func normalizeArtifactDedupeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	text = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '\t', '\u00a0':
			return ' '
		case '\u201c', '\u201d', '\u2033':
			return '"'
		case '\u2018', '\u2019', '\u2032':
			return '\''
		default:
			return r
		}
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	return strings.Trim(text, " \t\r\n.,;:!?\"'`()[]{}")
}

func normalizeArtifactComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"“", `"`,
		"”", `"`,
		"‘", `'`,
		"’", `'`,
	)
	text = replacer.Replace(text)
	text = strings.Join(strings.Fields(text), " ")
	return strings.Trim(text, " \t\r\n.,;:!?\"'`“”‘’()[]{}")
}

func kgTripleAlreadyExistsForTurn(existing []store.KGTriple, sid string, turnIndex int, subject, predicate, object string, validFrom, validTo int) bool {
	subject = strings.TrimSpace(strings.ToLower(subject))
	predicate = strings.TrimSpace(strings.ToLower(predicate))
	object = strings.TrimSpace(strings.ToLower(object))
	for _, item := range existing {
		if item.ChatSessionID != sid {
			continue
		}
		if strings.TrimSpace(strings.ToLower(item.Subject)) != subject ||
			strings.TrimSpace(strings.ToLower(item.Predicate)) != predicate ||
			strings.TrimSpace(strings.ToLower(item.Object)) != object {
			continue
		}
		if item.SourceTurn == turnIndex {
			return true
		}
		if item.ValidTo == 0 && validTo == 0 {
			return true
		}
		if validFrom > 0 && item.ValidFrom > 0 && item.ValidFrom != validFrom {
			continue
		}
		if validTo > 0 && item.ValidTo > 0 && item.ValidTo != validTo {
			continue
		}
		return true
	}
	return false
}

func (s *Server) applyCriticSoftPrune(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	targets := stringsFromAny(extraction["prune_targets"])
	if len(targets) == 0 || s.Store == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(s.Cfg.PrunePolicy), "off") {
		result.Warnings = append(result.Warnings, "soft_prune_disabled")
		return
	}
	updater, ok := s.Store.(memoryImportanceUpdater)
	if !ok {
		result.Warnings = append(result.Warnings, "soft_prune_update_not_supported")
		return
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		result.Warnings = append(result.Warnings, "soft_prune_list_failed")
		return
	}
	pruned := []map[string]any{}
	for _, target := range targets {
		keyword := strings.ToLower(strings.TrimSpace(target))
		if keyword == "" {
			continue
		}
		for _, mem := range memories {
			if mem.ID <= 0 || mem.Importance <= 0.1 {
				continue
			}
			if !strings.Contains(strings.ToLower(mem.SummaryJSON), keyword) {
				continue
			}
			oldImportance := mem.Importance
			newImportance := oldImportance - 0.2
			if newImportance < 0.1 {
				newImportance = 0.1
			}
			result.trySave("UpdateMemoryImportance", func() error {
				return updater.UpdateMemoryImportance(ctx, sid, mem.ID, newImportance)
			}, result, func() {
				pruned = append(pruned, map[string]any{
					"id":      mem.ID,
					"old":     oldImportance,
					"new":     newImportance,
					"keyword": keyword,
				})
			})
		}
	}
	if len(pruned) == 0 {
		return
	}
	result.trySave("SaveAuditLog(soft_prune)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "soft_prune",
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Summary:       fmt.Sprintf("Soft prune: %d memories, turn %d", len(pruned), turnIndex),
			DetailsJSON:   mustCompactJSON(map[string]any{"pruned": pruned, "targets": targets}),
			Source:        "critic",
			CreatedAt:     now,
		})
	}, result, func() {})
	for _, item := range pruned {
		memoryID, _ := item["id"].(int64)
		if memoryID <= 0 {
			continue
		}
		keyword, _ := item["keyword"].(string)
		s.saveSupersessionResolutionBestEffort(ctx, store.SupersessionResolutionDecision{
			ChatSessionID:   sid,
			TargetType:      "memory",
			TargetID:        memoryID,
			SourceTurn:      turnIndex,
			ResolutionClass: "stale_demote",
			RelationshipKey: keyword,
			Reason:          "critic_prune_target",
			EvidenceJSON:    mustCompactJSON(item),
			Operator:        "critic",
		}, now, result)
	}
}

func (s *Server) saveSupersessionResolutionBestEffort(ctx context.Context, decision store.SupersessionResolutionDecision, now time.Time, result *artifactSaveResult) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	if resolver, ok := s.Store.(store.SupersessionResolutionStore); ok {
		result.trySave("SaveSupersessionResolution", func() error {
			_, err := resolver.SaveSupersessionResolution(ctx, &decision)
			return err
		}, result, func() {})
		return
	}
	details := map[string]any{
		"contract_version": store.SupersessionResolutionContractVersion,
		"resolution_class": decision.ResolutionClass,
		"source_turn":      decision.SourceTurn,
		"target":           map[string]any{"type": decision.TargetType, "id": decision.TargetID},
		"afterglow_turns":  store.SupersessionResolutionAfterglowTurns,
		"hard_delete":      false,
	}
	if strings.TrimSpace(decision.NewTargetType) != "" || decision.NewTargetID > 0 {
		details["new_target"] = map[string]any{"type": strings.TrimSpace(decision.NewTargetType), "id": decision.NewTargetID}
	}
	if strings.TrimSpace(decision.RelationshipKey) != "" {
		details["relationship_key"] = strings.TrimSpace(decision.RelationshipKey)
	}
	if strings.TrimSpace(decision.Reason) != "" {
		details["reason"] = strings.TrimSpace(decision.Reason)
	}
	if strings.TrimSpace(decision.EvidenceJSON) != "" {
		var parsed any
		if err := json.Unmarshal([]byte(decision.EvidenceJSON), &parsed); err == nil {
			details["evidence"] = parsed
		} else {
			details["evidence_text"] = strings.TrimSpace(decision.EvidenceJSON)
		}
	}
	source := strings.TrimSpace(decision.Operator)
	if source == "" {
		source = "critic"
	}
	summary := fmt.Sprintf("Resolution %s: %s #%d", decision.ResolutionClass, decision.TargetType, decision.TargetID)
	result.trySave("SaveAuditLog(supersession_resolution)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: decision.ChatSessionID,
			EventType:     "supersession_resolution",
			TargetType:    decision.TargetType,
			TargetID:      decision.TargetID,
			Summary:       summary,
			DetailsJSON:   mustCompactJSON(details),
			Source:        source,
			CreatedAt:     now,
		})
	}, result, func() {})
}

const (
	physicalConditionIngestContractVersion = "physical_condition_ingest.v1"
	physicalConditionStatusKey             = "physical_condition"
	entityConditionIngestContractVersion   = "entity_condition_ingest.v1"
	entityConditionStatusKey               = "entity_condition"
)

type conditionEffectLane struct {
	ExtractionKey     string
	StatusKey         string
	SchemaName        string
	Label             string
	OwnerScope        string
	ContractVersion   string
	SourceLabel       string
	EntityTypeDefault string
	CharacterOwner    bool
	Options           map[string]any
}

func (s *Server) savePhysicalConditionsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	s.saveConditionEffectsFromExtraction(ctx, sid, turnIndex, extraction, now, result, conditionEffectLane{
		ExtractionKey:   "physical_conditions",
		StatusKey:       physicalConditionStatusKey,
		SchemaName:      "physical_condition_status",
		Label:           "Physical condition",
		OwnerScope:      "character",
		ContractVersion: physicalConditionIngestContractVersion,
		SourceLabel:     "critic.physical_conditions",
		CharacterOwner:  true,
		Options: map[string]any{
			"condition_lane":                  true,
			"authority_mode":                  "archive_canonical",
			"projection_density":              "light",
			"duration_policy":                 "evidence_bound_no_default_duration",
			"severity_policy":                 "descriptive_no_numeric_scale",
			"runtime_status_override_allowed": true,
		},
	})
}

func (s *Server) saveEntityConditionsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult) {
	s.saveConditionEffectsFromExtraction(ctx, sid, turnIndex, extraction, now, result, conditionEffectLane{
		ExtractionKey:     "entity_conditions",
		StatusKey:         entityConditionStatusKey,
		SchemaName:        "entity_condition_status",
		Label:             "Entity condition",
		OwnerScope:        "entity",
		ContractVersion:   entityConditionIngestContractVersion,
		SourceLabel:       "critic.entity_conditions",
		EntityTypeDefault: "entity",
		Options: map[string]any{
			"condition_lane":                  true,
			"entity_condition_lane":           true,
			"authority_mode":                  "archive_canonical",
			"projection_density":              "light",
			"duration_policy":                 "evidence_bound_no_default_duration",
			"severity_policy":                 "descriptive_no_numeric_scale",
			"runtime_status_override_allowed": true,
		},
	})
}

func (s *Server) saveConditionEffectsFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, now time.Time, result *artifactSaveResult, lane conditionEffectLane) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	conditions := normalizePhysicalConditionItems(extraction[lane.ExtractionKey])
	if len(conditions) == 0 {
		return
	}
	registry, hasRegistry := s.Store.(store.StatusSchemaRegistryStore)
	lifecycle, hasLifecycle := s.Store.(store.StatusLifecycleStore)
	if !hasRegistry || !hasLifecycle {
		result.addSkipReason(lane.ExtractionKey, "status_schema_lifecycle_store_unavailable", map[string]any{"items": len(conditions)})
		return
	}
	definition, ok := s.ensureConditionStatusDefinition(ctx, sid, now, result, registry, lane.StatusKey, lane.SchemaName, lane.Label, lane.OwnerScope, lane.Options, lane.ExtractionKey)
	if !ok {
		return
	}
	for _, condition := range conditions {
		owner := conditionOwnerName(condition)
		if lane.CharacterOwner {
			owner = s.canonicalCharacterName(ctx, sid, owner)
		}
		if owner == "" || isPlaceholderKGPart(owner) {
			result.addSkipReason(lane.ExtractionKey, "missing_owner", condition)
			continue
		}
		evidence := physicalConditionEvidence(condition)
		if evidence == "" {
			result.addSkipReason(lane.ExtractionKey, "missing_evidence_excerpt", condition)
			continue
		}
		label := conditionLabel(condition)
		if label == "" {
			result.addSkipReason(lane.ExtractionKey, "missing_condition_label", condition)
			continue
		}
		effectKind := statusNormalizeEffectKind(stringFromMap(condition, "effect_kind"))
		if effectKind == "" {
			effectKind = "temporary_effect"
		}
		effectState := statusNormalizeEffectState(extractionFirstNonEmpty(stringFromMap(condition, "effect_state"), "active"))
		if effectState == "" {
			effectState = "active"
		}
		payload := physicalConditionPayload(condition, turnIndex)
		payload["contract_version"] = lane.ContractVersion
		if lane.EntityTypeDefault != "" {
			payload["entity_type"] = extractionFirstNonEmpty(stringFromMap(condition, "owner_entity_type"), stringFromMap(condition, "entity_type"), lane.EntityTypeDefault)
		}
		evidenceJSON := mustCompactJSON(map[string]any{
			"contract_version": lane.ContractVersion,
			"source":           lane.SourceLabel,
			"source_turn":      turnIndex,
			"evidence_excerpt": evidence,
			"authority_hint":   stringFromMap(condition, "authority_hint"),
		})
		result.trySave("SaveStatusEffect("+lane.StatusKey+")", func() error {
			_, err := lifecycle.SaveStatusEffect(ctx, store.StatusEffect{
				ChatSessionID:      sid,
				RegistryID:         definition.ID,
				StatusKey:          definition.StatusKey,
				OwnerScope:         definition.OwnerScope,
				OwnerID:            owner,
				EffectKind:         effectKind,
				EffectLabel:        label,
				EffectPayloadJSON:  mustCompactJSON(payload),
				EvidenceJSON:       evidenceJSON,
				SourceTurn:         turnIndex,
				StartClockJSON:     physicalConditionStartClockJSON(condition, turnIndex),
				DurationJSON:       physicalConditionDurationJSON(condition),
				ExpiresAtClockJSON: physicalConditionExpiresAtClockJSON(condition),
				EffectState:        effectState,
				CreatedAt:          now,
				UpdatedAt:          now,
			})
			return err
		}, result, func() {
			if lane.ExtractionKey == "physical_conditions" {
				result.PhysicalConditions++
			}
			if lane.ExtractionKey == "entity_conditions" {
				result.EntityConditions++
			}
			result.StatusEffects++
		})
	}
}

func (s *Server) ensureConditionStatusDefinition(ctx context.Context, sid string, now time.Time, result *artifactSaveResult, registry store.StatusSchemaRegistryStore, statusKey, schemaName, label, ownerScope string, options map[string]any, skipKey string) (store.StatusSchemaDefinition, bool) {
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		if result != nil {
			result.addSkipReason(skipKey, "status_schema_lookup_failed", err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	definition = store.StatusSchemaDefinition{
		ChatSessionID: sid,
		SchemaName:    schemaName,
		StatusKey:     statusKey,
		Label:         label,
		OwnerScope:    ownerScope,
		ValueKind:     "note",
		OptionsJSON:   mustCompactJSON(options),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	var saved []store.StatusSchemaDefinition
	if result == nil {
		var saveErr error
		saved, saveErr = registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{definition})
		return firstSavedPhysicalConditionDefinition(definition, saved), saveErr == nil
	}
	result.trySave("SaveStatusSchemaDefinitions("+statusKey+")", func() error {
		var saveErr error
		saved, saveErr = registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{definition})
		return saveErr
	}, result, func() { result.StatusSchemaDefinitions++ })
	if len(saved) == 0 {
		return store.StatusSchemaDefinition{}, false
	}
	return firstSavedPhysicalConditionDefinition(definition, saved), true
}

func firstSavedPhysicalConditionDefinition(fallback store.StatusSchemaDefinition, saved []store.StatusSchemaDefinition) store.StatusSchemaDefinition {
	if len(saved) == 0 {
		return fallback
	}
	return saved[0]
}

func normalizePhysicalConditionItems(raw any) []map[string]any {
	out := []map[string]any{}
	for _, item := range sliceFromAny(raw) {
		m := mapFromAny(item)
		if len(m) == 0 || !hasMeaningfulPayload(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func physicalConditionEvidence(item map[string]any) string {
	if text := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "evidence_excerpt"),
		stringFromMap(item, "evidence"),
		stringFromMap(item, "source_excerpt"),
	)); text != "" {
		return text
	}
	excerpts := stringsFromAny(item["evidence_excerpts"])
	if len(excerpts) == 0 {
		return ""
	}
	return excerpts[0]
}

func physicalConditionPayload(item map[string]any, turnIndex int) map[string]any {
	payload := map[string]any{
		"contract_version":          physicalConditionIngestContractVersion,
		"source_turn":               turnIndex,
		"condition":                 item,
		"duration_policy":           extractionFirstNonEmpty(stringFromMap(item, "duration_policy"), "unknown_until_updated"),
		"hardcoded_duration":        false,
		"numeric_severity_required": false,
	}
	if text := strings.TrimSpace(stringFromMap(item, "severity_text")); text != "" {
		payload["severity_text"] = text
	}
	if text := strings.TrimSpace(stringFromMap(item, "age_or_vulnerability_note")); text != "" {
		payload["age_or_vulnerability_note"] = text
	}
	if text := strings.TrimSpace(stringFromMap(item, "uncertainty_note")); text != "" {
		payload["uncertainty_note"] = text
	}
	return payload
}

func physicalConditionStartClockJSON(item map[string]any, turnIndex int) string {
	for _, key := range []string{"start_clock_json", "onset_story_clock_json", "story_clock_json"} {
		if raw := mapFromAny(item[key]); hasMeaningfulPayload(raw) {
			return mustCompactJSON(raw)
		}
	}
	return mustCompactJSON(map[string]any{
		"source_turn":      turnIndex,
		"precision":        "turn",
		"precision_label":  "turn_anchor",
		"calendar_unknown": true,
	})
}

func physicalConditionDurationJSON(item map[string]any) string {
	if raw := mapFromAny(item["duration_json"]); hasMeaningfulPayload(raw) {
		return mustCompactJSON(raw)
	}
	if raw := mapFromAny(item["expires_at_clock_json"]); hasMeaningfulPayload(raw) {
		return ""
	}
	return mustCompactJSON(map[string]any{
		"policy":             "unknown_until_updated",
		"reason":             "no_explicit_duration_in_evidence",
		"hardcoded_duration": false,
	})
}

func physicalConditionExpiresAtClockJSON(item map[string]any) string {
	if raw := mapFromAny(item["expires_at_clock_json"]); hasMeaningfulPayload(raw) {
		return mustCompactJSON(raw)
	}
	return ""
}

func conditionOwnerName(item map[string]any) string {
	return strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "owner_entity_name"),
		stringFromMap(item, "owner_entity_key"),
		stringFromMap(item, "owner_name"),
		stringFromMap(item, "entity_name"),
		stringFromMap(item, "character_name"),
		stringFromMap(item, "subject"),
		stringFromMap(item, "name"),
	))
}

func conditionLabel(item map[string]any) string {
	return strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "condition_label"),
		stringFromMap(item, "condition"),
		stringFromMap(item, "status_label"),
		stringFromMap(item, "summary"),
	))
}

func entityDescriptionWithConditions(base, name, entityType string, physicalConditions, entityConditions []map[string]any) string {
	base = strings.TrimSpace(base)
	nameKey := comparableEntityKey(name)
	if nameKey == "" {
		return base
	}
	var candidates []map[string]any
	if strings.EqualFold(strings.TrimSpace(entityType), "character") {
		candidates = physicalConditions
	} else {
		candidates = entityConditions
	}
	parts := []string{}
	for _, condition := range candidates {
		if comparableEntityKey(conditionOwnerName(condition)) != nameKey {
			continue
		}
		label := conditionLabel(condition)
		if label == "" {
			continue
		}
		if bodyArea := strings.TrimSpace(stringFromMap(condition, "body_area")); bodyArea != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(bodyArea)) {
			label = label + " (" + bodyArea + ")"
		}
		parts = append(parts, "condition: "+label)
	}
	if len(parts) == 0 {
		return base
	}
	extra := strings.Join(dedupeStrings(parts), "; ")
	if base == "" {
		return extra
	}
	if strings.Contains(base, extra) {
		return base
	}
	return base + " | " + extra
}

func comparableEntityKey(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}

func (s *Server) saveCharacterAndStateArtifacts(ctx context.Context, sid string, turnIndex int, extraction map[string]any, embCfg completeTurnEmbeddingConfig, now time.Time, result *artifactSaveResult, existingCanonicalLayers []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement) {
	entities := mapFromAny(extraction["entities"])
	physicalConditions := normalizePhysicalConditionItems(extraction["physical_conditions"])
	entityConditions := normalizePhysicalConditionItems(extraction["entity_conditions"])
	saveEntityItems := func(items []any, entityType string) {
		for _, item := range items {
			entity := mapFromAny(item)
			name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title"))))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			if saver, ok := s.Store.(entitySaver); ok {
				localType := extractionFirstNonEmpty(stringFromMap(entity, "entity_type"), stringFromMap(entity, "role"), entityType)
				description := entityDescriptionWithConditions(
					extractionFirstNonEmpty(stringFromMap(entity, "status_emotion"), stringFromMap(entity, "description"), stringFromMap(entity, "summary")),
					name,
					localType,
					physicalConditions,
					entityConditions,
				)
				result.trySave("SaveEntity", func() error {
					return saver.SaveEntity(ctx, &store.Entity{
						ChatSessionID: sid,
						Name:          name,
						EntityType:    localType,
						Description:   description,
						AliasesJSON:   mustCompactJSON(stringsFromAny(entity["aliases"])),
						FirstSeenTurn: turnIndex,
						LastSeenTurn:  turnIndex,
						Confidence:    clampFloat(extractionFloatFromAny(entity["confidence"], 0.7), 0, 1),
						CreatedAt:     now,
						UpdatedAt:     now,
					})
				}, result, func() { result.Entities++ })
			}
		}
	}
	saveEntityItems(sliceFromAny(entities["characters"]), "character")
	saveEntityItems(sliceFromAny(entities["locations"]), "location")
	saveEntityItems(sliceFromAny(entities["places"]), "location")
	saveEntityItems(sliceFromAny(entities["items"]), "item")
	saveEntityItems(sliceFromAny(entities["objects"]), "item")
	characterNames := extractedEntityNames(ctx, s, sid, entities)

	relationshipMemory := mapFromAny(extraction["relationship_memory"])
	if trustText := strings.TrimSpace(stringFromMap(relationshipMemory, "bond_and_distance")); trustText != "" {
		if saver, ok := s.Store.(trustSaver); ok {
			for _, target := range relationshipMemoryTargets(relationshipMemory, characterNames) {
				result.trySave("SaveTrust", func() error {
					return saver.SaveTrust(ctx, &store.Trust{
						ChatSessionID: sid,
						TargetName:    target,
						TargetType:    "relationship",
						Score:         clampFloat(extractionFloatFromAny(relationshipMemory["trust"], 0.5), 0, 1),
						ReasonJSON:    mustCompactJSON(relationshipMemory),
						SourceTurn:    turnIndex,
						CreatedAt:     now,
						UpdatedAt:     now,
					})
				}, result, func() { result.TrustStates++ })
			}
		}
	}

	for _, item := range sliceFromAny(extraction["character_deltas"]) {
		charDelta := mapFromAny(item)
		name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(stringFromMap(charDelta, "name")))
		if name == "" {
			continue
		}
		if looksLikeTransientDescriptorCharacterName(name) && !characterDeltaHasContinuityAnchor(charDelta) {
			continue
		}
		var currentState *store.CharacterState
		if current, err := s.Store.GetCharacterState(ctx, sid, name); err == nil {
			currentState = current
		}
		if saver, ok := s.Store.(characterStateSaver); ok {
			appearanceJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "appearance"), charDelta["appearance"])
			personalityJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "personality"), charDelta["personality"])
			statusJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "status"), charDelta["status"])
			relationshipsJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "relationships"), charDelta["relationships"])
			speechStyleJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "speech_style"), charDelta["speech_style"])
			result.trySave("SaveCharacterState", func() error {
				return saver.SaveCharacterState(ctx, &store.CharacterState{
					ChatSessionID:     sid,
					CharacterName:     name,
					AppearanceJSON:    appearanceJSON,
					PersonalityJSON:   personalityJSON,
					StatusJSON:        statusJSON,
					RelationshipsJSON: relationshipsJSON,
					SpeechStyleJSON:   speechStyleJSON,
					TurnIndex:         turnIndex,
					CreatedAt:         now,
					UpdatedAt:         now,
				})
			}, result, func() { result.CharacterStates++ })
		}
		for _, ev := range sliceFromAny(charDelta["events"]) {
			evMap := mapFromAny(ev)
			detail := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(evMap, "detail"), stringFromMap(evMap, "summary"), mustCompactJSON(evMap)))
			if detail == "" {
				continue
			}
			result.trySave("SaveCharacterEvent", func() error {
				return s.Store.SaveCharacterEvent(ctx, &store.CharacterEvent{
					ChatSessionID: sid,
					CharacterName: name,
					TurnIndex:     turnIndex,
					EventType:     extractionFirstNonEmpty(stringFromMap(evMap, "type"), "critic_delta"),
					DetailsJSON:   mustCompactJSON(map[string]any{"detail": detail, "delta": charDelta}),
					CreatedAt:     now,
				})
			}, result, func() { result.CharacterEvents++ })
		}
	}

	s.savePhysicalConditionsFromExtraction(ctx, sid, turnIndex, extraction, now, result)
	s.saveEntityConditionsFromExtraction(ctx, sid, turnIndex, extraction, now, result)

	if saver, ok := s.Store.(activeStateSaver); ok {
		for _, key := range []string{"relationship_memory", "state_deltas", "entities"} {
			rawState, present := extraction[key]
			if !present {
				continue
			}
			if key == "state_deltas" {
				rawState = sanitizeStateDeltasForParticipant(rawState)
			}
			if key == "relationship_memory" {
				rawState = normalizeRelationshipStateV2(mapFromAny(rawState))
			}
			if !hasMeaningfulPayload(rawState) {
				continue
			}
			stateType := key
			result.trySave("SaveActiveState", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     stateType,
					Content:       mustCompactJSON(rawState),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
			// P358 HS-1a: canonical state layer from active state with provenance (P407)
			if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 {
				layerType := mapKeyToCanonicalLayerType(key)
				confidence := extractConfidenceForStateKey(extraction, key)
				if canonicalStatePromotionAllowed(rawState, confidence) {
					result.trySave("SaveCanonicalStateLayer", func() error {
						return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
							ChatSessionID:    sid,
							LayerType:        layerType,
							Content:          mustCompactJSON(rawState),
							SourceStateType:  stateType,
							TurnIndex:        turnIndex,
							SourceTurn:       turnIndex,
							SourceRecord:     0,
							LastVerifiedTurn: turnIndex,
							Confidence:       confidence,
							CreatedAt:        now,
						}, existingCanonicalLayers, cost)
					}, result, func() { result.CanonicalStateLayers++ })
				}
			}
		}
	}

	// P469 HS-1h: world current state minimal canonical snapshot
	if wsPayload, ok := extractWorldStatePayload(extraction); ok && hasMeaningfulPayload(wsPayload) {
		if saver, ok := s.Store.(activeStateSaver); ok {
			result.trySave("SaveActiveState(world_state)", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     "world_state",
					Content:       mustCompactJSON(wsPayload),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
		}
		if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 {
			confidence := extractConfidenceForStateKey(extraction, "world_state")
			if canonicalStatePromotionAllowed(wsPayload, confidence) {
				result.trySave("SaveCanonicalStateLayer(world_state)", func() error {
					return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
						ChatSessionID:    sid,
						LayerType:        "world_state",
						Content:          mustCompactJSON(wsPayload),
						SourceStateType:  "world_state",
						TurnIndex:        turnIndex,
						SourceTurn:       turnIndex,
						SourceRecord:     0,
						LastVerifiedTurn: turnIndex,
						Confidence:       confidence,
						CreatedAt:        now,
					}, existingCanonicalLayers, cost)
				}, result, func() { result.CanonicalStateLayers++ })
			}
		}
	}

	for _, item := range sliceFromAny(extraction["pending_threads"]) {
		thread := mapFromAny(item)
		title := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(thread, "title"), stringFromMap(thread, "description"), stringFromMap(thread, "thread_type")))
		if title == "" {
			result.addSkipReason("pending_threads", "missing_title", thread)
			continue
		}
		threadType := strings.TrimSpace(stringFromMap(thread, "thread_type"))
		if threadType == "" {
			threadType = "open_question"
		}
		if !validPendingThreadType(threadType) {
			result.addSkipReason("pending_threads", "invalid_thread_type", thread)
			continue
		}
		confidence := clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1)
		if _, hasConfidence := thread["confidence"]; hasConfidence && confidence < 0.3 {
			result.addSkipReason("pending_threads", "low_confidence", thread)
			continue
		}
		if saver, ok := s.Store.(pendingThreadSaver); ok {
			result.trySave("SavePendingThread", func() error {
				return saver.SavePendingThread(ctx, &store.PendingThread{
					ChatSessionID:    sid,
					ThreadKey:        stableKey("thread", title),
					Description:      extractionFirstNonEmpty(stringFromMap(thread, "details"), title),
					Status:           "open",
					CreatedTurn:      turnIndex,
					SourceTurn:       turnIndex,
					Priority:         intFromAny(thread["priority"], 0),
					HookType:         threadType,
					HookMetadataJSON: mustCompactJSON(thread),
					ThreadType:       threadType,
					Title:            title,
					Owner:            sanitizeParticipantActorName(stringFromMap(thread, "owner")),
					Target:           sanitizeParticipantActorName(stringFromMap(thread, "target")),
					LastSeenTurn:     turnIndex,
					Confidence:       confidence,
					DetailsJSON:      mustCompactJSON(thread),
					CreatedAt:        now,
					UpdatedAt:        now,
				})
			}, result, func() { result.PendingThreads++ })
		}
		threadState := map[string]any{
			"thread_type": threadType,
			"title":       title,
			"status":      "open",
			"confidence":  confidence,
			"source_turn": turnIndex,
		}
		if saver, ok := s.Store.(activeStateSaver); ok {
			result.trySave("SaveActiveState(unresolved_threads)", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     "unresolved_threads",
					Content:       mustCompactJSON(threadState),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
		}
		// P358 HS-1a: canonical state layer for unresolved threads with provenance (P407)
		if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 && confidence >= 0.7 {
			result.trySave("SaveCanonicalStateLayer", func() error {
				return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
					ChatSessionID:    sid,
					LayerType:        "unresolved_threads",
					Content:          mustCompactJSON(threadState),
					SourceStateType:  "pending_threads",
					TurnIndex:        turnIndex,
					SourceTurn:       turnIndex,
					SourceRecord:     0,
					LastVerifiedTurn: turnIndex,
					Confidence:       confidence,
					CreatedAt:        now,
				}, existingCanonicalLayers, cost)
			}, result, func() { result.CanonicalStateLayers++ })
		}
		if saver, ok := s.Store.(storylineSaver); ok {
			result.trySave("SaveStoryline", func() error {
				return saver.SaveStoryline(ctx, &store.Storyline{
					ChatSessionID:       sid,
					Name:                title,
					Status:              "active",
					EntitiesJSON:        mustCompactJSON(extraction["entities"]),
					CurrentContext:      extractionFirstNonEmpty(stringFromMap(thread, "details"), title),
					KeyPointsJSON:       mustCompactJSON([]string{title}),
					OngoingTensionsJSON: mustCompactJSON(thread),
					Confidence:          clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1),
					EvidenceCount:       len(stringsFromAny(extraction["evidence_excerpts"])),
					LastEvidenceTurn:    turnIndex,
					FirstTurn:           turnIndex,
					LastTurn:            turnIndex,
					CreatedAt:           now,
					UpdatedAt:           now,
				})
			}, result, func() { result.Storylines++ })
		}
	}

	if saver, ok := s.Store.(worldRuleSaver); ok {
		for _, item := range worldRuleItemsForSave(extraction) {
			rule := mapFromAny(item)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name")))
			if key == "" {
				continue
			}
			wr := &store.WorldRule{
				ChatSessionID: sid,
				Scope:         extractionFirstNonEmpty(stringFromMap(rule, "scope"), "session"),
				ScopeName:     stringFromMap(rule, "scope_name"),
				Category:      extractionFirstNonEmpty(stringFromMap(rule, "category"), "critic"),
				Key:           key,
				ValueJSON:     mustCompactJSON(extractionFirstNonEmpty(stringFromMap(rule, "value"), stringFromMap(rule, "value_json"), mustCompactJSON(rule))),
				Genre:         stringFromMap(rule, "genre"),
				SourceTurn:    turnIndex,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			result.trySave("SaveWorldRule", func() error {
				return saver.SaveWorldRule(ctx, wr)
			}, result, func() {
				result.WorldRules++
				s.upsertDerivedArtifactVector(ctx, sid, turnIndex, "world_rule", "world_rules", wr.ID, "world_rule.v1", worldRuleVectorDocumentText(*wr), embCfg, result)
			})
		}
	}
	s.saveCriticIngestTrace(ctx, sid, turnIndex, now, result)
}

func (r *artifactSaveResult) addSkipReason(surface, reason string, input any) {
	if r == nil {
		return
	}
	r.SkipReasons = append(r.SkipReasons, map[string]any{
		"surface": surface,
		"reason":  reason,
		"input":   input,
	})
}

func (s *Server) saveCriticIngestTrace(ctx context.Context, sid string, turnIndex int, now time.Time, result *artifactSaveResult) {
	if s.Store == nil || result == nil {
		return
	}
	details := map[string]any{
		"policy_version":              "critic_ingest_trace.v1",
		"turn_index":                  turnIndex,
		"memories":                    result.Memories,
		"direct_evidence":             result.Evidence,
		"kg_triples":                  result.KGTriples,
		"persona_capsule_candidates":  result.PersonaCapsuleCandidates,
		"subjective_entity_memories":  result.SubjectiveEntityMemories,
		"character_states":            result.CharacterStates,
		"physical_conditions":         result.PhysicalConditions,
		"entity_conditions":           result.EntityConditions,
		"status_schema_definitions":   result.StatusSchemaDefinitions,
		"status_effects":              result.StatusEffects,
		"pending_threads":             result.PendingThreads,
		"active_states":               result.ActiveStates,
		"canonical_layers":            result.CanonicalStateLayers,
		"skip_reasons":                result.SkipReasons,
		"warnings":                    result.Warnings,
		"embedding_status":            result.EmbeddingStatus,
		"vector_status":               result.VectorStatus,
		"vectors_upserted":            result.VectorsUpserted,
		"vectors_memory_upserted":     result.VectorsMemoryUpserted,
		"vectors_evidence_upserted":   result.VectorsEvidenceUpserted,
		"vectors_world_rule_upserted": result.VectorsWorldRuleUpserted,
		"artifact_save_errors":        result.ErrorDetails,
	}
	result.trySave("SaveAuditLog(critic_ingest_trace)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "critic_ingest_trace",
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Source:        "critic",
			Summary:       fmt.Sprintf("critic ingest trace turn %d", turnIndex),
			DetailsJSON:   mustCompactJSON(details),
			CreatedAt:     now,
		})
	}, result, func() {})
}

func currentCharacterJSON(current *store.CharacterState, field string) string {
	if current == nil {
		return ""
	}
	switch field {
	case "appearance":
		return current.AppearanceJSON
	case "personality":
		return current.PersonalityJSON
	case "status":
		return current.StatusJSON
	case "relationships":
		return current.RelationshipsJSON
	case "speech_style":
		return current.SpeechStyleJSON
	default:
		return ""
	}
}

func mergeCharacterStateJSONField(existing string, incoming any) string {
	if !hasMeaningfulPayload(incoming) {
		return strings.TrimSpace(existing)
	}
	incomingJSON := mustCompactJSON(incoming)
	if strings.TrimSpace(existing) == "" {
		return incomingJSON
	}
	var existingMap map[string]any
	var incomingMap map[string]any
	if json.Unmarshal([]byte(existing), &existingMap) != nil || json.Unmarshal([]byte(incomingJSON), &incomingMap) != nil {
		return incomingJSON
	}
	return mustCompactJSON(mergeJSONMaps(existingMap, incomingMap))
}

func mergeJSONMaps(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range overlay {
		if overlayMap, ok := value.(map[string]any); ok {
			if baseMap, ok := out[key].(map[string]any); ok {
				out[key] = mergeJSONMaps(baseMap, overlayMap)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func (r *artifactSaveResult) trySave(label string, save func() error, result *artifactSaveResult, onOK func()) {
	result.Attempted++
	if err := save(); err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, label+": "+err.Error())
		return
	}
	onOK()
}

func (s *Server) upsertMemoryVector(ctx context.Context, sid string, turnIndex int, mem *store.Memory, documentText string, embedding []float32, result *artifactSaveResult) {
	if result == nil {
		return
	}
	if len(embedding) == 0 {
		if result.VectorStatus == "not_requested" {
			result.VectorStatus = "missing_embedding"
		}
		return
	}
	if s.Vector == nil {
		result.VectorStatus = "vector_not_configured"
		return
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		result.VectorStatus = "vector_not_configured"
		return
	}
	sourceRowID := strconv.FormatInt(mem.ID, 10)
	if mem.ID <= 0 {
		sourceRowID = fmt.Sprintf("turn_%d_memory", turnIndex)
	}
	searchBuild := memorySearchTextBuild{}
	if mem != nil {
		searchBuild = memorySearchTextFromMemory(*mem)
	}
	documentText = strings.TrimSpace(documentText)
	if documentText == "" {
		documentText = strings.TrimSpace(searchBuild.Text)
	}
	languageMeta := map[string]string{}
	if mem != nil {
		languageMeta = memoryVectorLanguageMetadata(*mem)
	}
	doc := vector.VectorDocument{
		ID:                    fmt.Sprintf("memory:%s:%s", sid, sourceRowID),
		Embedding:             embedding,
		Tier:                  "memory",
		ChatSessionID:         sid,
		SourceTable:           "memories",
		SourceRowID:           sourceRowID,
		SchemaVersion:         "memory.v2",
		DocumentText:          documentText,
		SearchTextPolicy:      extractionFirstNonEmpty(languageMeta["search_text_policy"], languageMemorySearchPolicy),
		RawLanguage:           languageMeta["raw_language"],
		SummaryLanguage:       languageMeta["summary_language"],
		SessionOutputLanguage: languageMeta["session_output_language"],
		AliasCount:            searchBuild.AliasCount,
	}
	if err := s.Vector.Upsert(ctx, sid, []vector.VectorDocument{doc}); err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_upsert_failed")
		return
	}
	result.VectorsUpserted++
	result.VectorsMemoryUpserted++
	result.VectorStatus = "ok"
}

func (s *Server) upsertDerivedArtifactVector(ctx context.Context, sid string, turnIndex int, tier, sourceTable string, sourceRowID int64, schemaVersion, documentText string, embCfg completeTurnEmbeddingConfig, result *artifactSaveResult) {
	if result == nil {
		return
	}
	tier = strings.TrimSpace(tier)
	sourceTable = strings.TrimSpace(sourceTable)
	documentText = strings.TrimSpace(documentText)
	if tier == "" || sourceTable == "" || documentText == "" {
		return
	}
	if sourceRowID <= 0 {
		result.VectorStatus = "missing_source_row_id"
		result.Warnings = append(result.Warnings, "vector_"+tier+"_source_row_id_missing")
		return
	}
	if !embCfg.hasConfig() {
		if result.VectorStatus == "not_requested" {
			result.VectorStatus = "missing_embedding_config"
		}
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_config_missing")
		return
	}
	if s.Vector == nil || strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		result.VectorStatus = "vector_not_configured"
		return
	}
	emb, _, err := callEmbedding(ctx, embCfg, documentText)
	if err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_failed")
		return
	}
	embedding := parseFloat32JSONList(emb)
	if len(embedding) == 0 {
		result.VectorStatus = "empty_embedding"
		result.Warnings = append(result.Warnings, "vector_"+tier+"_embedding_empty")
		return
	}
	rowID := strconv.FormatInt(sourceRowID, 10)
	doc := vector.VectorDocument{
		ID:               fmt.Sprintf("%s:%s:%s", tier, sid, rowID),
		Embedding:        embedding,
		Tier:             tier,
		ChatSessionID:    sid,
		SourceTable:      sourceTable,
		SourceRowID:      rowID,
		SchemaVersion:    schemaVersion,
		DocumentText:     documentText,
		SearchTextPolicy: "derived_artifact_search_text.v1",
	}
	if err := s.Vector.Upsert(ctx, sid, []vector.VectorDocument{doc}); err != nil {
		result.VectorStatus = "error: " + err.Error()
		result.Warnings = append(result.Warnings, "vector_"+tier+"_upsert_failed")
		return
	}
	result.VectorsUpserted++
	switch tier {
	case "evidence":
		result.VectorsEvidenceUpserted++
	case "world_rule":
		result.VectorsWorldRuleUpserted++
	}
	result.VectorStatus = "ok"
}

func directEvidenceVectorDocumentText(ev store.DirectEvidence) string {
	parts := []string{}
	if kind := strings.TrimSpace(ev.EvidenceKind); kind != "" {
		parts = append(parts, "kind: "+kind)
	}
	if text := strings.TrimSpace(ev.EvidenceText); text != "" {
		parts = append(parts, text)
	}
	if ev.SourceTurnStart > 0 || ev.SourceTurnEnd > 0 || ev.TurnAnchor > 0 {
		parts = append(parts, fmt.Sprintf("turns: %d-%d anchor:%d", ev.SourceTurnStart, ev.SourceTurnEnd, ev.TurnAnchor))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func worldRuleVectorDocumentText(wr store.WorldRule) string {
	parts := []string{}
	for _, part := range []string{
		strings.TrimSpace(wr.Scope),
		strings.TrimSpace(wr.ScopeName),
		strings.TrimSpace(wr.Category),
		strings.TrimSpace(wr.Key),
		strings.TrimSpace(wr.ValueJSON),
	} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func callEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string) (string, string, error) {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "":
		return "", "", errors.New("embedding provider is required")
	case "ollama":
		return callOllamaEmbedding(ctx, cfg, input)
	case "gemini":
		return callGeminiEmbedding(ctx, cfg, input, false)
	case "vertex":
		return callGeminiEmbedding(ctx, cfg, input, true)
	case "voyageai":
		return callOpenAICompatibleEmbedding(ctx, cfg, input, normalizeVoyageEmbeddingEndpoint(cfg.Endpoint), true)
	case "openai", "custom":
		return callOpenAICompatibleEmbedding(ctx, cfg, input, normalizeEmbeddingEndpoint(cfg.Endpoint), false)
	default:
		return "", "", fmt.Errorf("unsupported embedding provider %q", provider)
	}
}

func callOllamaEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string) (string, string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint = strings.TrimSuffix(endpoint, "/v1")
	}
	if !strings.HasSuffix(endpoint, "/api/embed") {
		endpoint += "/api/embed"
	}
	payload, _ := json.Marshal(map[string]any{
		"model": cfg.Model,
		"input": input,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("embedding upstream returned %s", resp.Status)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", err
	}
	embedding := data["embedding"]
	if embedding == nil {
		rows := sliceFromAny(data["embeddings"])
		if len(rows) > 0 {
			embedding = rows[0]
		}
	}
	if embedding == nil {
		return "", "", errors.New("embedding_data_empty")
	}
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), cfg.Model, nil
}

func callOpenAICompatibleEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string, endpoint string, arrayInput bool) (string, string, error) {
	body := map[string]any{"model": cfg.Model, "input": input}
	if arrayInput {
		body["input"] = []string{input}
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("embedding upstream returned %s", resp.Status)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", "", err
	}
	rows := sliceFromAny(data["data"])
	if len(rows) == 0 {
		return "", "", errors.New("embedding_data_empty")
	}
	embedding := mapFromAny(rows[0])["embedding"]
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), extractionFirstNonEmpty(extractionStringFromAny(data["model"]), cfg.Model), nil
}

func callGeminiEmbedding(ctx context.Context, cfg completeTurnEmbeddingConfig, input string, vertex bool) (string, string, error) {
	target := proxyNormalizeGeminiEndpoint(cfg.Endpoint, cfg.Model, "embedContent")
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	if vertex {
		token, _, err := proxyGetVertexAccessToken(ctx, cfg.APIKey)
		if err != nil {
			return "", "", err
		}
		target = proxyNormalizeVertexEmbeddingEndpoint(cfg.Endpoint, cfg.Model)
		target, err = proxyResolveVertexProjectID(target, cfg.APIKey)
		if err != nil {
			return "", "", err
		}
		headers["Authorization"] = "Bearer " + token
	} else {
		headers["x-goog-api-key"] = cfg.APIKey
	}
	status, data, raw, err := proxyDoJSON(ctx, target, headers, map[string]any{
		"content": map[string]any{"parts": []map[string]any{{"text": input}}},
	})
	if err != nil {
		return "", "", err
	}
	if status < 200 || status >= 300 {
		detail := proxyErrorDetail(status, data, raw)
		if vertex {
			detail = proxyVertexEndpointErrorDetail(status, target, data, raw)
		}
		return "", "", fmt.Errorf("embedding upstream returned %s: %s", http.StatusText(status), detail)
	}
	embedding := mapFromAny(data["embedding"])["values"]
	if embedding == nil {
		return "", "", errors.New("embedding_data_empty")
	}
	b, err := json.Marshal(embedding)
	if err != nil {
		return "", "", err
	}
	return string(b), cfg.Model, nil
}

func normalizeEmbeddingEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(endpoint, "/embeddings") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return strings.TrimSuffix(endpoint, "/chat/completions") + "/embeddings"
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/embeddings"
	}
	return endpoint + "/embeddings"
}

func normalizeVoyageEmbeddingEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return "https://api.voyageai.com/v1/embeddings"
	}
	if strings.HasSuffix(endpoint, "/embeddings") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/embeddings"
	}
	return endpoint + "/embeddings"
}

func proxyNormalizeVertexEmbeddingEndpoint(endpoint, model string) string {
	base := proxyNormalizeVertexBaseEndpoint(endpoint)
	if strings.Contains(base, ":streamGenerateContent") {
		return strings.Replace(base, ":streamGenerateContent", ":embedContent", 1)
	}
	if strings.Contains(base, ":generateContent") {
		return strings.Replace(base, ":generateContent", ":embedContent", 1)
	}
	if strings.Contains(base, ":embedContent") {
		return base
	}
	return base + "/" + strings.TrimSpace(model) + ":embedContent"
}

func parseFloat32JSONList(raw string) []float32 {
	var values []any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	out := make([]float32, 0, len(values))
	for _, item := range values {
		switch v := item.(type) {
		case float64:
			out = append(out, float32(v))
		case float32:
			out = append(out, v)
		case int:
			out = append(out, float32(v))
		case json.Number:
			f, err := v.Float64()
			if err == nil {
				out = append(out, float32(f))
			}
		default:
			return nil
		}
	}
	return out
}

func chatCompletionText(resp map[string]any) string {
	choices := sliceFromAny(resp["choices"])
	if len(choices) == 0 {
		return ""
	}
	choice := mapFromAny(choices[0])
	msg := mapFromAny(choice["message"])
	if content := stringFromMap(msg, "content"); content != "" {
		return content
	}
	return extractionStringFromAny(choice["text"])
}

func canonicalCompleteTurnIndex(ctx context.Context, st store.Store, sid string, requested int) int {
	if requested <= 0 {
		requested = 1
	}
	logs, err := st.ListChatLogs(ctx, sid, 0, 0)
	if err != nil {
		return requested
	}
	maxTurn := 0
	for _, log := range logs {
		if log.ChatSessionID == sid && log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	if requested <= maxTurn {
		return maxTurn + 1
	}
	return requested
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func sliceFromAny(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{}
}

func stringsFromAny(v any) []string {
	if items, ok := v.([]string); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s != "" {
			return []string{s}
		}
	}
	out := []string{}
	for _, item := range sliceFromAny(v) {
		if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringFromMap(m map[string]any, key string) string {
	return strings.TrimSpace(extractionStringFromAny(m[key]))
}

func boolFromAny(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "y", "1", "on":
			return true
		default:
			return false
		}
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	case json.Number:
		n, err := t.Float64()
		return err == nil && n != 0
	default:
		return false
	}
}

func extractionStringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func int64FromMap(m map[string]any, key string, fallback int64) int64 {
	return int64(intFromAny(m[key], int(fallback)))
}

func intFromAny(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return fallback
}

func floatFromMap(m map[string]any, key string, fallback float64) float64 {
	return extractionFloatFromAny(m[key], fallback)
}

func extractionFloatFromAny(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if n, err := t.Float64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return n
		}
	}
	return fallback
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func extractionFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

var kgPredicateRe = regexp.MustCompile(`[^a-z0-9_]+`)

func sanitizeKGPredicate(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "_")
	v = kgPredicateRe.ReplaceAllString(v, "_")
	return strings.Trim(v, "_")
}

func sanitizeKGPart(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "null") || strings.EqualFold(v, "none") {
		return ""
	}
	return v
}

func sanitizeEvidenceExcerptForTurn(excerpt string, turnContent string) string {
	text := strings.TrimSpace(excerpt)
	if text == "" {
		return ""
	}
	turn := strings.TrimSpace(turnContent)
	if turn == "" {
		return ""
	}
	if len([]rune(text)) > 500 {
		text = string([]rune(text)[:500])
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

func sanitizeCriticStorageText(text string) string {
	cleaned := filterCompleteMarkerPattern.ReplaceAllString(text, "")
	cleaned = closedThoughtTagPattern.ReplaceAllString(cleaned, "")
	cleaned = openThoughtTagPattern.ReplaceAllString(cleaned, "")
	cleaned = thoughtLinePrefixPattern.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

func isPlaceholderKGPart(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	return placeholderKGPartPattern.MatchString(v)
}

func shouldSkipKGTriple(subject, predicate, object, sid string) bool {
	if subject == "" || predicate == "" || object == "" || subject == sid {
		return true
	}
	if isPlaceholderKGPart(subject) || isPlaceholderKGPart(object) {
		return true
	}
	switch predicate {
	case "has_turn", "turn", "mentions_turn", "source_turn":
		return true
	}
	return false
}

func extractedEntityNames(ctx context.Context, s *Server, sid string, entities map[string]any) []string {
	out := []string{}
	add := func(items []any) {
		for _, item := range items {
			entity := mapFromAny(item)
			name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title"))))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			out = appendUniqueString(out, name)
		}
	}
	add(sliceFromAny(entities["characters"]))
	return out
}

func relationshipMemoryTargets(relationshipMemory map[string]any, characterNames []string) []string {
	targets := []string{}
	for _, key := range []string{"target_name", "target", "character", "entity", "subject", "object"} {
		target := sanitizeParticipantActorName(stringFromMap(relationshipMemory, key))
		if target != "" {
			targets = appendUniqueString(targets, target)
		}
	}
	for _, item := range stringsFromAny(relationshipMemory["pair"]) {
		for _, part := range relationshipPairParts(item) {
			target := sanitizeParticipantActorName(part)
			if target != "" {
				targets = appendUniqueString(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		for _, item := range characterNames {
			targets = appendUniqueString(targets, item)
		}
	}
	return targets
}

func sanitizeParticipantActorName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" || isPlaceholderKGPart(name) {
		return ""
	}
	return name
}

func relationshipPairParts(value string) []string {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	splitters := []string{"<->", "↔", "->", "→", "/", "|", "&"}
	for _, sep := range splitters {
		if strings.Contains(text, sep) {
			parts := []string{}
			for _, part := range strings.Split(text, sep) {
				part = strings.TrimSpace(part)
				if part != "" {
					parts = append(parts, part)
				}
			}
			return parts
		}
	}
	return []string{text}
}

func sanitizeStateDeltasForParticipant(raw any) map[string]any {
	state := mapFromAny(raw)
	if len(state) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range state {
		if key == "relationship_changes" {
			cleaned := []any{}
			for _, item := range sliceFromAny(value) {
				rel := mapFromAny(item)
				left, right := relationshipChangeActors(rel)
				left = sanitizeParticipantActorName(left)
				right = sanitizeParticipantActorName(right)
				if left == "" || right == "" {
					continue
				}
				rel["from"] = left
				rel["to"] = right
				delete(rel, "pair")
				delete(rel, "pair_key")
				identity := mapFromAny(rel["identity"])
				identity["left_entity"] = left
				identity["right_entity"] = right
				delete(identity, "pair")
				delete(identity, "pair_key")
				rel["identity"] = identity
				cleaned = append(cleaned, rel)
			}
			if len(cleaned) > 0 {
				out[key] = cleaned
			}
			continue
		}
		out[key] = value
	}
	return out
}

func relationshipChangeActors(rel map[string]any) (string, string) {
	identity := mapFromAny(rel["identity"])
	left := extractionFirstNonEmpty(
		stringFromMap(rel, "from"),
		stringFromMap(rel, "source"),
		stringFromMap(identity, "left_entity"),
		stringFromMap(identity, "source"),
	)
	right := extractionFirstNonEmpty(
		stringFromMap(rel, "to"),
		stringFromMap(rel, "target"),
		stringFromMap(identity, "right_entity"),
		stringFromMap(identity, "target"),
	)
	if left != "" && right != "" {
		return left, right
	}
	for _, key := range []string{"pair", "pair_key"} {
		for _, part := range relationshipPairParts(extractionFirstNonEmpty(stringFromMap(rel, key), stringFromMap(identity, key))) {
			if left == "" {
				left = part
				continue
			}
			if right == "" {
				right = part
				break
			}
		}
		if left != "" && right != "" {
			break
		}
	}
	return left, right
}

func stableKey(prefix, text string) string {
	key := strings.ToLower(strings.TrimSpace(text))
	key = kgPredicateRe.ReplaceAllString(strings.ReplaceAll(key, " ", "_"), "_")
	key = strings.Trim(key, "_")
	if key == "" {
		key = "item"
	}
	if len(key) > 80 {
		key = key[:80]
	}
	return prefix + "_" + key
}

// normalizeRelationshipStateV2 ensures v1 relationship_memory payloads stay intact
// while injecting safe minimal defaults for missing v2 additive sections (P518).
// It preserves identity, core_state, dynamics, context, history, verification,
// and branch-style fields (desire, fear, wound, mask, bond, fixation).
func normalizeRelationshipStateV2(raw map[string]any) map[string]any {
	if raw == nil {
		raw = map[string]any{}
	}
	out := make(map[string]any, len(raw)+12)
	for k, v := range raw {
		out[k] = v
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "verification"} {
		if _, ok := out[key]; !ok {
			out[key] = map[string]any{}
		}
	}
	for _, key := range []string{"desire", "fear", "wound", "mask", "bond", "fixation"} {
		if _, ok := out[key]; !ok {
			out[key] = map[string]any{}
		}
	}
	return out
}

func worldRuleItemsForSave(extraction map[string]any) []any {
	out := make([]any, 0)
	seen := map[string]bool{}
	add := func(raw any, fromWorldState bool) {
		rule := mapFromAny(raw)
		if len(rule) == 0 {
			text := strings.TrimSpace(extractionStringFromAny(raw))
			if text == "" {
				return
			}
			rule = map[string]any{
				"key":      stableKey("world_rule", text),
				"value":    text,
				"scope":    "session",
				"category": "world_state",
			}
		}
		key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name")))
		if key == "" {
			return
		}
		scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(rule, "scope"), "session"))
		scopeName := strings.TrimSpace(stringFromMap(rule, "scope_name"))
		rule["scope"] = scope
		sig := strings.ToLower(scope) + "\x00" + strings.ToLower(scopeName) + "\x00" + strings.ToLower(key)
		if seen[sig] {
			return
		}
		seen[sig] = true
		if fromWorldState {
			cp := make(map[string]any, len(rule)+2)
			for k, v := range rule {
				cp[k] = v
			}
			if strings.TrimSpace(stringFromMap(cp, "scope")) == "" {
				cp["scope"] = "session"
			} else {
				cp["scope"] = store.NormalizeWorldRuleScope(stringFromMap(cp, "scope"))
			}
			if strings.TrimSpace(stringFromMap(cp, "category")) == "" {
				cp["category"] = "world_state"
			}
			out = append(out, cp)
			return
		}
		out = append(out, rule)
	}
	for _, item := range sliceFromAny(extraction["world_rules"]) {
		add(item, false)
	}
	if ws := mapFromAny(extraction["world_state"]); len(ws) > 0 {
		for _, item := range sliceFromAny(ws["rules"]) {
			add(item, true)
		}
	}
	return out
}

// extractWorldStatePayload builds a minimal world_state snapshot from critic extraction.
// It prefers an explicit world_state map, then falls back to world_rules array (P469).
func extractWorldStatePayload(extraction map[string]any) (map[string]any, bool) {
	if ws := mapFromAny(extraction["world_state"]); len(ws) > 0 {
		return ws, true
	}
	rules := sliceFromAny(extraction["world_rules"])
	if len(rules) > 0 {
		out := map[string]any{
			"rules":   rules,
			"version": "world_state.v1",
		}
		for _, key := range []string{"faction_status", "region_pressure", "offscreen_threads"} {
			if v, ok := extraction[key]; ok {
				out[key] = v
			}
		}
		return out, true
	}
	return nil, false
}

// mapKeyToCanonicalLayerType maps extraction keys to canonical layer types (P358).
func mapKeyToCanonicalLayerType(key string) string {
	switch key {
	case "relationship_memory":
		return "relationship_state"
	case "state_deltas":
		return "scene_state"
	case "entities":
		return "entity_state"
	case "world_rules", "world_state":
		return "world_state"
	default:
		return key
	}
}

func canonicalStatePromotionAllowed(raw any, confidence float64) bool {
	if confidence < 0.7 {
		return false
	}
	payload := mapFromAny(raw)
	status := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(payload, "verification"),
		stringFromMap(payload, "capture_verification"),
		stringFromMap(payload, "promotion_status"),
		stringFromMap(payload, "status"),
	)))
	switch status {
	case "pending", "rejected", "unverified", "repair_queue", "hold", "manual_review":
		return false
	}
	if rawVerified, ok := payload["verified"]; ok {
		switch v := rawVerified.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(strings.TrimSpace(v), "true") || strings.EqualFold(strings.TrimSpace(v), "verified")
		}
	}
	return true
}

// extractConfidenceForStateKey extracts confidence from critic extraction for a given state key (P407).
func extractConfidenceForStateKey(extraction map[string]any, key string) float64 {
	switch key {
	case "relationship_memory":
		if rm := mapFromAny(extraction["relationship_memory"]); len(rm) > 0 {
			return clampFloat(extractionFloatFromAny(rm["confidence"], 0.7), 0, 1)
		}
	case "state_deltas":
		if sd := mapFromAny(extraction["state_deltas"]); len(sd) > 0 {
			return clampFloat(extractionFloatFromAny(sd["confidence"], 0.7), 0, 1)
		}
	case "entities":
		return 0.7
	case "world_rules", "world_state":
		if ws, ok := extractWorldStatePayload(extraction); ok {
			return clampFloat(extractionFloatFromAny(ws["confidence"], 0.75), 0, 1)
		}
		return 0.75
	}
	return 0.7
}
