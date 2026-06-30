package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestParseJSONFromLLMContentRepairsMalformedCriticJSON(t *testing.T) {
	raw := "preface\n```json\n{\n  \u201cturn_summary\u201d: \u201cMina found the brass key.\u201d,\n  \"importance_score\": 8,\n  \"evidence_excerpts\": [\"Mina found the brass key.\",],\n  \"archive_hint\": None,\n}\n```\nignored trailing text {\"wrong\":true}"
	got, err := parseJSONFromLLMContent(raw)
	if err != nil {
		t.Fatalf("parseJSONFromLLMContent failed: %v", err)
	}
	if got["turn_summary"] != "Mina found the brass key." {
		t.Fatalf("turn_summary = %#v", got["turn_summary"])
	}
	if got["archive_hint"] != nil {
		t.Fatalf("archive_hint = %#v, want nil from repaired None", got["archive_hint"])
	}
	items, ok := got["evidence_excerpts"].([]any)
	if !ok || len(items) != 1 || items[0] != "Mina found the brass key." {
		t.Fatalf("evidence_excerpts = %#v", got["evidence_excerpts"])
	}
}

func TestParseJSONFromLLMContentRepairsTruncatedCriticJSON(t *testing.T) {
	raw := `{"turn_summary":"Mina found the brass key","evidence_excerpts":["Mina found the brass key.",`
	got, err := parseJSONFromLLMContent(raw)
	if err != nil {
		t.Fatalf("parseJSONFromLLMContent failed: %v", err)
	}
	if got["turn_summary"] != "Mina found the brass key" {
		t.Fatalf("turn_summary = %#v", got["turn_summary"])
	}
	items, ok := got["evidence_excerpts"].([]any)
	if !ok || len(items) != 1 || items[0] != "Mina found the brass key." {
		t.Fatalf("evidence_excerpts = %#v", got["evidence_excerpts"])
	}
}

func TestParseJSONFromLLMContentRepairsTruncatedStringValue(t *testing.T) {
	raw := `{"turn_summary":"Mina found`
	got, err := parseJSONFromLLMContent(raw)
	if err != nil {
		t.Fatalf("parseJSONFromLLMContent failed: %v", err)
	}
	if got["turn_summary"] != "Mina found" {
		t.Fatalf("turn_summary = %#v", got["turn_summary"])
	}
}

func TestParseJSONFromLLMContentRepairsMissingObjectValue(t *testing.T) {
	raw := `{"turn_summary":"Mina found the key","archive_hint":},"importance_score":7}`
	got, err := parseJSONFromLLMContent(raw)
	if err != nil {
		t.Fatalf("parseJSONFromLLMContent failed: %v", err)
	}
	if got["turn_summary"] != "Mina found the key" {
		t.Fatalf("turn_summary = %#v", got["turn_summary"])
	}
	if got["archive_hint"] != nil {
		t.Fatalf("archive_hint = %#v, want nil for repaired missing value", got["archive_hint"])
	}
}

func TestNormalizeCriticExtractionRejectsStructuredTurnSummary(t *testing.T) {
	raw := map[string]any{
		"turn_summary": map[string]any{
			"archive_hint":     map[string]any{"room": "hall_events"},
			"character_deltas": []any{map[string]any{"name": "Luka"}},
		},
		"importance_score": 8,
	}
	normalized := normalizeCriticExtraction(raw)
	if got := extractionStringFromAny(normalized["turn_summary"]); got != "" {
		t.Fatalf("normalized turn_summary = %q, want empty for structured payload", got)
	}
	enriched := enrichNormalizedCriticExtractionForFocusedRecall(normalized, "Luka asks how the island reward system works.", "The party confirms rewards are exchanged after clearing challenges.", 7)
	got := extractionStringFromAny(enriched["turn_summary"])
	if !strings.Contains(got, "Luka asks") || strings.Contains(got, "archive_hint") {
		t.Fatalf("fallback turn_summary = %q", got)
	}
}

func TestNormalizeCriticExtractionRejectsSerializedExtractionSummary(t *testing.T) {
	raw := map[string]any{
		"turn_summary":     `{"archive_hint":{"room":"hall_events"},"character_deltas":[{"name":"Luka"}],"kg_triples":[]}`,
		"importance_score": 6,
	}
	normalized := normalizeCriticExtraction(raw)
	if got := extractionStringFromAny(normalized["turn_summary"]); got != "" {
		t.Fatalf("normalized turn_summary = %q, want empty for serialized extraction payload", got)
	}
}

func TestSanitizeCriticStorageTextRemovesThoughtAndFilterTags(t *testing.T) {
	got := sanitizeCriticStorageText(strings.Join([]string{
		"Visible user text. <thinking>hidden chain</thinking> Still visible.",
		"<analysis>private analysis</analysis>",
		"<reasoning>private reasoning</reasoning>",
		"<scratchpad>private scratchpad</scratchpad>",
		"Scratchpad: line-only private draft",
		"Reasoning: line-only private reasoning",
		"<__filter_complete__><filter>truncated hidden",
	}, "\n"))
	for _, blocked := range []string{"hidden", "thinking", "filter", "__filter_complete__", "private analysis", "private reasoning", "private scratchpad", "line-only"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(blocked)) {
			t.Fatalf("sanitizeCriticStorageText leaked %q in %q", blocked, got)
		}
	}
	if !strings.Contains(got, "Visible user text.") || !strings.Contains(got, "Still visible.") {
		t.Fatalf("sanitizeCriticStorageText removed visible text: %q", got)
	}
}

// ---------------------------------------------------------------------------
// EA-1h / P280: canonical conflict resolution state machine
// ---------------------------------------------------------------------------

func TestClassifyConflictExactMatchIsStateTransition(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice trusts Bob."}
	existing := store.DirectEvidence{EvidenceText: "Alice trusts Bob."}
	got := classifyConflict(incoming.EvidenceText, existing)
	if got != conflictClassStateTransition {
		t.Fatalf("classifyConflict exact match = %q, want state_transition", got)
	}
}

func TestClassifyConflictHardContradictionOnOverlap(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice no longer trusts Bob after the betrayal."}
	existing := store.DirectEvidence{EvidenceText: "Alice trusts Bob deeply."}
	got := classifyConflict(incoming.EvidenceText, existing)
	if got != conflictClassHardContradiction {
		t.Fatalf("classifyConflict overlap = %q, want hard_contradiction", got)
	}
}

func TestClassifyConflictParallelContextOnWeakOverlap(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice likes Bob."}
	existing := store.DirectEvidence{EvidenceText: "Alice trusts Bob."}
	got := classifyConflict(incoming.EvidenceText, existing)
	if got != conflictClassParallelContext {
		t.Fatalf("classifyConflict weak overlap = %q, want parallel_context", got)
	}
}

func TestClassifyConflictLowConfidenceNoiseWhenUnrelated(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "The sky is blue."}
	existing := store.DirectEvidence{EvidenceText: "Alice trusts Bob."}
	got := classifyConflict(incoming.EvidenceText, existing)
	if got != conflictClassLowConfidenceNoise {
		t.Fatalf("classifyNoise unrelated = %q, want low_confidence_noise", got)
	}
}

func TestResolveCanonicalConflictRoutesSupersededForExactMatch(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice trusts Bob."}
	existing := []store.DirectEvidence{
		{ID: 1, EvidenceText: "Alice trusts Bob.", CaptureVerification: "verified"},
	}
	results := resolveCanonicalConflict(incoming, existing)
	if len(results) != 1 {
		t.Fatalf("expected 1 conflict result, got %d", len(results))
	}
	if results[0]["routing"] != conflictRouteSuperseded {
		t.Fatalf("routing = %q, want superseded", results[0]["routing"])
	}
	if results[0]["classification"] != conflictClassStateTransition {
		t.Fatalf("classification = %q, want state_transition", results[0]["classification"])
	}
}

func TestResolveCanonicalConflictRoutesTombstoneForHighConfidenceContradiction(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice hates Bob."}
	existing := []store.DirectEvidence{
		{ID: 2, EvidenceText: "Alice loves Bob.", CaptureVerification: "verified"},
	}
	results := resolveCanonicalConflict(incoming, existing)
	if len(results) != 1 {
		t.Fatalf("expected 1 conflict result, got %d", len(results))
	}
	if results[0]["classification"] != conflictClassHardContradiction {
		t.Fatalf("classification = %q, want hard_contradiction", results[0]["classification"])
	}
	if results[0]["routing"] != conflictRouteTombstone {
		t.Fatalf("routing = %q, want tombstone (auto_promote high confidence)", results[0]["routing"])
	}
}

func TestResolveCanonicalConflictRoutesManualReviewForLowConfidenceContradiction(t *testing.T) {
	incoming := store.DirectEvidence{EvidenceText: "Alice hates Bob."}
	existing := []store.DirectEvidence{
		{ID: 3, EvidenceText: "Alice loves Bob.", CaptureVerification: "rejected"},
	}
	results := resolveCanonicalConflict(incoming, existing)
	if len(results) != 1 {
		t.Fatalf("expected 1 conflict result, got %d", len(results))
	}
	if results[0]["routing"] != conflictRouteManualReview {
		t.Fatalf("routing = %q, want manual_review (low confidence)", results[0]["routing"])
	}
}

// ---------------------------------------------------------------------------
// EA-1i / P294: conflict confidence / high-impact policy
// ---------------------------------------------------------------------------

func TestBuildConflictConfidencePolicyRelationshipFieldHighThreshold(t *testing.T) {
	policy := buildConflictConfidencePolicy(0.9, "relationship")
	if policy["auto_promote"] != true {
		t.Fatalf("auto_promote should be true for 0.9 relationship")
	}
	if policy["threshold"] != 0.85 {
		t.Fatalf("threshold = %v, want 0.85", policy["threshold"])
	}
}

func TestBuildConflictConfidencePolicyLowConfidenceGoesToRepairQueue(t *testing.T) {
	policy := buildConflictConfidencePolicy(0.6, "relationship")
	if policy["auto_promote"] != false {
		t.Fatalf("auto_promote should be false for 0.6 relationship")
	}
	if policy["repair_queue"] != true {
		t.Fatalf("repair_queue should be true for 0.6 relationship")
	}
}

func TestBuildConflictConfidencePolicyWorldRuleVeryHighThreshold(t *testing.T) {
	policy := buildConflictConfidencePolicy(0.95, "world_rule")
	if policy["threshold"] != 0.90 {
		t.Fatalf("threshold = %v, want 0.90", policy["threshold"])
	}
	if policy["auto_promote"] != true {
		t.Fatalf("auto_promote should be true for 0.95 world_rule")
	}
}

// ---------------------------------------------------------------------------
// EA-1l / P337: importance-aware retention / TTL policy
// ---------------------------------------------------------------------------

func TestApplyRetentionPolicyHighImportanceDirectEvidence(t *testing.T) {
	ev := store.DirectEvidence{EvidenceText: "Key fact."}
	decision := applyRetentionPolicy(&ev, 0.85, nil)
	if decision["archive_state"] != "canonical_direct" {
		t.Fatalf("archive_state = %q, want canonical_direct", decision["archive_state"])
	}
	if decision["ttl_turns"] != 0 {
		t.Fatalf("ttl_turns = %v, want 0", decision["ttl_turns"])
	}
}

func TestApplyRetentionPolicyMediumImportancePreviousArchive(t *testing.T) {
	ev := store.DirectEvidence{EvidenceText: "Secondary fact."}
	decision := applyRetentionPolicy(&ev, 0.6, nil)
	if decision["archive_state"] != "previous_archive" {
		t.Fatalf("archive_state = %q, want previous_archive", decision["archive_state"])
	}
	if decision["ttl_turns"] != 120 {
		t.Fatalf("ttl_turns = %v, want 120", decision["ttl_turns"])
	}
}

func TestApplyRetentionPolicyTombstonePreserveForAudit(t *testing.T) {
	ev := store.DirectEvidence{EvidenceText: "Old fact.", Tombstoned: true}
	decision := applyRetentionPolicy(&ev, 0.2, nil)
	if decision["archive_state"] != "tombstone_audit" {
		t.Fatalf("archive_state = %q, want tombstone_audit", decision["archive_state"])
	}
	if decision["ttl_turns"] != 240 {
		t.Fatalf("ttl_turns = %v, want 240", decision["ttl_turns"])
	}
}

func TestApplyRetentionPolicySupersededLineagePreserve(t *testing.T) {
	ev := store.DirectEvidence{ID: 5, EvidenceText: "Updated fact."}
	existing := []store.DirectEvidence{
		{ID: 4, SupersededByID: 5},
	}
	decision := applyRetentionPolicy(&ev, 0.2, existing)
	if decision["archive_state"] != "superseded_archive" {
		t.Fatalf("archive_state = %q, want superseded_archive", decision["archive_state"])
	}
	if decision["ttl_turns"] != 60 {
		t.Fatalf("ttl_turns = %v, want 60", decision["ttl_turns"])
	}
}
