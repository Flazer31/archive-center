package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func reversibleStateProposal(domain, transition, subject, slot, excerpt, valueText string) map[string]any {
	proposal := map[string]any{
		"version":          reversibleStateContractVersion,
		"domain":           domain,
		"transition":       transition,
		"subject_name":     subject,
		"state_slot":       slot,
		"evidence_excerpt": excerpt,
		"scene_scope":      "current",
		"authority":        "canonical_in_fiction",
		"assertion_kind":   "literal",
		"polarity":         "affirmative",
		"visibility":       "public",
		"sensitivity":      "ordinary",
	}
	if valueText != "" {
		proposal["value"] = map[string]any{"text": valueText}
	}
	return proposal
}

func saveReversibleTurn(
	t *testing.T,
	fake *identityRecordingStore,
	turn int,
	revision string,
	content string,
	proposals []any,
	characters []any,
	items []any,
) artifactSaveResult {
	t.Helper()
	excerpts := make([]any, 0, len(proposals))
	for _, candidate := range proposals {
		excerpts = append(excerpts, mapFromAny(candidate)["evidence_excerpt"])
	}
	persistedContent := content
	for _, excerpt := range excerpts {
		if strings.TrimSpace(persistedContent) == strings.TrimSpace(extractionStringFromAny(excerpt)) {
			persistedContent += " The scene continued."
			break
		}
	}
	extraction := map[string]any{
		"turn_summary":      persistedContent,
		"importance_score":  6,
		"evidence_excerpts": excerpts,
		"entities": map[string]any{
			"characters": characters,
			"locations":  []any{},
			"items":      items,
		},
		"reversible_states": proposals,
	}
	server := NewServer(config.Default())
	server.Store = fake
	normalized := normalizeCriticExtraction(extraction)
	if len(normalizeReversibleStateProposals(normalized["reversible_states"])) != len(proposals) {
		reasons := make([]string, 0, len(proposals))
		for _, proposal := range proposals {
			if err := validateReversibleStateProposal(proposal); err != nil {
				reasons = append(reasons, err.Error())
			}
		}
		t.Fatalf("reversible proposals were rejected during normalization: %v", reasons)
	}
	result := server.saveCriticExtractionArtifacts(
		acceptedStoryClockContext(revision, "logical-"+revision, "generation-"+revision),
		"story-session",
		turn,
		normalized,
		persistedContent,
		completeTurnEmbeddingConfig{},
		time.Unix(int64(turn), 0),
	)
	if result.Errors != 0 {
		t.Fatalf("reversible state save failed: %#v", result.ErrorDetails)
	}
	return result
}

func currentReversibleProjection(t *testing.T, fake *identityRecordingStore, statusKey string) map[string]any {
	t.Helper()
	for _, value := range fake.returnStatusCurrent {
		if value.StatusKey != statusKey {
			continue
		}
		projection := map[string]any{}
		if err := json.Unmarshal([]byte(value.ValueJSON), &projection); err != nil {
			t.Fatalf("decode reversible projection: %v", err)
		}
		return projection
	}
	t.Fatalf("missing current projection for %s", statusKey)
	return nil
}

func TestReversibleStateSetChangeRecoverClearAndReplay(t *testing.T) {
	fake := newIdentityRecordingStore()
	fake.reviewCanonicalLabel("Mina")
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	body := func(transition, excerpt, text string) map[string]any {
		proposal := reversibleStateProposal("body", transition, "Mina", "left_arm_injury", excerpt, text)
		proposal["sensitivity"] = "sensitive"
		if text != "" {
			subtype := "fracture"
			if strings.Contains(excerpt, "fractured") {
				subtype = "fractured"
			} else if strings.Contains(excerpt, "injured") {
				subtype = "injured"
			} else if strings.Contains(excerpt, "bruised") {
				subtype = "bruised"
			}
			proposal["value"] = map[string]any{
				"text": text,
				"body": map[string]any{
					"subtype":       subtype,
					"affected_area": "left arm",
					"category":      "medical",
				},
			}
		}
		return proposal
	}

	saveReversibleTurn(t, fake, 1, "rev-set", "Mina's left arm was fractured.", []any{
		body("set", "Mina's left arm was fractured.", "left arm was fractured"),
	}, characters, nil)
	saveReversibleTurn(t, fake, 2, "rev-change", "Mina's left arm fracture worsened.", []any{
		body("change", "Mina's left arm fracture worsened.", "left arm fracture worsened"),
	}, characters, nil)
	saveReversibleTurn(t, fake, 3, "rev-recover", "Mina's left arm fracture healed.", []any{
		body("recover", "Mina's left arm fracture healed.", ""),
	}, characters, nil)
	saveReversibleTurn(t, fake, 4, "rev-set-again", "Mina's left arm was injured again.", []any{
		body("set", "Mina's left arm was injured again.", "left arm was injured again"),
	}, characters, nil)
	saveReversibleTurn(t, fake, 5, "rev-clear", "Mina's left arm injury was cleared.", []any{
		body("clear", "Mina's left arm injury was cleared.", ""),
	}, characters, nil)

	if len(fake.savedStatusEvents) != 5 {
		t.Fatalf("expected immutable five-event lifecycle, got %#v", fake.savedStatusEvents)
	}
	if got := len(mapFromAny(currentReversibleProjection(t, fake, reversibleBodyStatusKey)["slots"])); got != 0 {
		t.Fatalf("clear did not remove the current slot: %d", got)
	}
	if fake.savedStatusEvents[1].PreviousValueJSON == "" ||
		fake.savedStatusEvents[2].PreviousValueJSON == "" ||
		fake.savedStatusEvents[4].PreviousValueJSON == "" {
		t.Fatalf("state transitions lost previous projection history: %#v", fake.savedStatusEvents)
	}

	currentWrites := len(fake.savedStatusCurrent)
	eventWrites := len(fake.savedStatusEvents)
	saveReversibleTurn(t, fake, 5, "rev-clear", "Mina's left arm injury was cleared.", []any{
		body("clear", "Mina's left arm injury was cleared.", ""),
	}, characters, nil)
	if len(fake.savedStatusCurrent) != currentWrites || len(fake.savedStatusEvents) != eventWrites {
		t.Fatalf("same source replay was not idempotent: current=%d/%d events=%d/%d",
			len(fake.savedStatusCurrent), currentWrites, len(fake.savedStatusEvents), eventWrites)
	}

	older := body("set", "An old account said Mina's left arm was bruised.", "left arm was bruised")
	currentWrites = len(fake.savedStatusCurrent)
	saveReversibleTurn(t, fake, 2, "rev-older", "An old account said Mina's left arm was bruised.", []any{older}, characters, nil)
	if len(fake.savedStatusCurrent) != currentWrites {
		t.Fatalf("older observation overwrote the current projection: %#v", fake.savedStatusCurrent)
	}
	last := fake.savedStatusEvents[len(fake.savedStatusEvents)-1]
	if last.EventState != "history_only" || strings.Contains(last.EvidenceJSON, `"current_projection":true`) {
		t.Fatalf("older observation was not isolated as history: %#v", last)
	}
}

func TestReversibleStateDomainsAndPreparePrivacyBoundary(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	items := []any{map[string]any{"name": "Sacred Sword", "entity_type": "item"}}

	body := reversibleStateProposal("body", "set", "Mina", "fever", "Mina had a high fever.", "had a high fever")
	body["visibility"] = "private"
	body["sensitivity"] = "sensitive"
	body["value"] = map[string]any{
		"text": "had a high fever",
		"body": map[string]any{"subtype": "fever", "category": "medical"},
	}
	location := reversibleStateProposal("location", "set", "Mina", "current_location", "Mina arrived at the north gate.", "arrived at the north gate")
	possession := reversibleStateProposal("possession", "set", "Mina", "brass_key", "Mina held the brass key.", "held the brass key")
	emotion := reversibleStateProposal("emotion", "set", "Mina", "fear", "Mina felt afraid.", "felt afraid")
	emotion["visibility"] = "private"
	entity := reversibleStateProposal("entity_condition", "set", "Sacred Sword", "blade_integrity", "Sacred Sword was broken.", "was broken")

	result := saveReversibleTurn(t, fake, 10, "rev-domains",
		"Mina had a high fever. Mina arrived at the north gate. Mina held the brass key. Mina felt afraid. Sacred Sword was broken.",
		[]any{body, location, possession, emotion, entity}, characters, items)
	if result.PhysicalConditions != 1 || result.EntityConditions != 1 || len(fake.savedStatusEvents) != 5 || len(fake.returnStatusCurrent) != 3 {
		t.Fatalf("five reversible domains were not persisted: result=%#v events=%#v", result, fake.savedStatusEvents)
	}

	scope := prepareTurnRequestEntityScope{Direct: []string{"Mina", "Sacred Sword"}}
	packet, text := buildReversibleStatePacket(fake.returnStatusCurrent, map[string]any{}, 10000, scope)
	if intFromAny(packet["active_count"], 0) != 3 {
		t.Fatalf("prepare packet should expose only public ordinary current state: %#v", packet)
	}
	excluded, ok := packet["excluded_counts"].(map[string]int)
	if !ok || excluded["sensitive"] != 0 || excluded["non_public"] != 0 || excluded["private_emotion"] != 0 {
		t.Fatalf("privacy exclusion accounting mismatch: %#v", excluded)
	}
	if strings.Contains(text, "high fever") || strings.Contains(text, "felt afraid") ||
		!strings.Contains(text, "north gate") || !strings.Contains(text, "brass key") || !strings.Contains(text, "broken") {
		t.Fatalf("prepare text crossed privacy boundary or omitted public state: %q", text)
	}
	minaOnlyPacket, minaOnlyText := buildReversibleStatePacket(
		fake.returnStatusCurrent, map[string]any{}, 10000,
		prepareTurnRequestEntityScope{Direct: []string{"Mina"}},
	)
	minaOnlyExcluded := minaOnlyPacket["excluded_counts"].(map[string]int)
	if strings.Contains(minaOnlyText, "Sacred Sword") ||
		minaOnlyExcluded["unrelated_subject"] != 1 {
		t.Fatalf("unrelated entity current state leaked into delivery: packet=%#v text=%q",
			minaOnlyPacket, minaOnlyText)
	}
	budgetPacket, budgetText := buildReversibleStatePacket(fake.returnStatusCurrent, map[string]any{}, 1, scope)
	budgetExcluded, ok := budgetPacket["excluded_counts"].(map[string]int)
	if !ok {
		t.Fatalf("budget exclusion accounting has unexpected type: %#v", budgetPacket["excluded_counts"])
	}
	if budgetText != "" ||
		intFromAny(budgetPacket["eligible_count"], 0) != 3 ||
		intFromAny(budgetPacket["delivered_count"], 0) != 0 ||
		intFromAny(budgetExcluded["budget"], 0) != 3 {
		t.Fatalf("reversible delivery did not preserve whole-item budget accounting: %#v text=%q", budgetPacket, budgetText)
	}
}

func TestReversibleStateRequiresReviewedIdentityAndExistingSlotForMutation(t *testing.T) {
	fake := newIdentityRecordingStore()
	fake.reviewCanonicalLabel("Mina")
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}

	missing := reversibleStateProposal("location", "change", "Mina", "Current Location",
		"Mina moved to the south gate.", "moved to the south gate")
	saveReversibleTurn(t, fake, 1, "rev-missing-prior",
		"Mina moved to the south gate.", []any{missing}, characters, nil)
	if len(fake.returnStatusCurrent) != 0 || len(fake.savedStatusEvents) != 1 ||
		fake.savedStatusEvents[0].EventState != "history_only" ||
		!strings.Contains(fake.savedStatusEvents[0].EvidenceJSON, "prior_slot_missing_history_only") {
		t.Fatalf("change without a prior slot mutated current: current=%#v events=%#v", fake.returnStatusCurrent, fake.savedStatusEvents)
	}

	set := reversibleStateProposal("location", "set", "Mina", "Current Location",
		"Mina stood at the east gate.", "stood at the east gate")
	saveReversibleTurn(t, fake, 2, "rev-normalized-set",
		"Mina stood at the east gate.", []any{set}, characters, nil)
	change := reversibleStateProposal("location", "change", "Mina", "current-location",
		"Mina moved to the west gate.", "moved to the west gate")
	saveReversibleTurn(t, fake, 3, "rev-normalized-change",
		"Mina moved to the west gate.", []any{change}, characters, nil)
	projection := currentReversibleProjection(t, fake, reversibleLocationStatusKey)
	slots := mapFromAny(projection["slots"])
	if len(slots) != 1 || slots["current_location"] == nil {
		t.Fatalf("equivalent machine slot spellings diverged: %#v", slots)
	}
}

func TestReversibleStateDoesNotContinuePriorSlotByLabelOnly(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Alex", "entity_type": "character"}}
	set := reversibleStateProposal("location", "set", "Alex", "current_location",
		"Alex stood at the east gate.", "stood at the east gate")
	saveReversibleTurn(t, fake, 1, "rev-alex-first",
		"Alex stood at the east gate.", []any{set}, characters, nil)
	change := reversibleStateProposal("location", "change", "Alex", "current_location",
		"Alex moved to the west gate.", "moved to the west gate")
	saveReversibleTurn(t, fake, 2, "rev-alex-second",
		"Alex moved to the west gate.", []any{change}, characters, nil)
	if len(fake.returnStatusCurrent) != 1 {
		t.Fatalf("label-only change created or replaced a current owner: %#v", fake.returnStatusCurrent)
	}
	projection := currentReversibleProjection(t, fake, reversibleLocationStatusKey)
	slot := mapFromAny(mapFromAny(projection["slots"])["current_location"])
	if extractionStringFromAny(mapFromAny(slot["value"])["text"]) != "stood at the east gate" {
		t.Fatalf("label-only change replaced the original occurrence state: %#v", projection)
	}
	last := fake.savedStatusEvents[len(fake.savedStatusEvents)-1]
	if last.EventState != "history_only" ||
		!strings.Contains(last.EvidenceJSON, "identity_continuity_unresolved_history_only") {
		t.Fatalf("label-only mutation was not retained as history-only: %#v", last)
	}
}

func TestReversibleStateAmbiguousSameLabelPriorSlotsStayHistoryOnly(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Alex", "entity_type": "character"}}
	saveReversibleTurn(t, fake, 1, "rev-alex-guard",
		"Alex stood at the east gate.", []any{reversibleStateProposal(
			"location", "set", "Alex", "current_location",
			"Alex stood at the east gate.", "stood at the east gate",
		)}, characters, nil)
	saveReversibleTurn(t, fake, 2, "rev-alex-merchant",
		"Alex stood at the west gate.", []any{reversibleStateProposal(
			"location", "set", "Alex", "current_location",
			"Alex stood at the west gate.", "stood at the west gate",
		)}, characters, nil)
	if len(fake.returnStatusCurrent) != 2 {
		t.Fatalf("same-label set observations were merged instead of isolated: %#v", fake.returnStatusCurrent)
	}
	saveReversibleTurn(t, fake, 3, "rev-alex-ambiguous",
		"Alex moved to the north gate.", []any{reversibleStateProposal(
			"location", "change", "Alex", "current_location",
			"Alex moved to the north gate.", "moved to the north gate",
		)}, characters, nil)
	last := fake.savedStatusEvents[len(fake.savedStatusEvents)-1]
	if last.EventState != "history_only" ||
		!strings.Contains(last.EvidenceJSON, "identity_continuity_unresolved_history_only") {
		t.Fatalf("ambiguous same-label mutation was not isolated: %#v", last)
	}
	packet, text := buildReversibleStatePacket(
		fake.returnStatusCurrent, map[string]any{}, 10000,
		prepareTurnRequestEntityScope{Direct: []string{"Alex"}},
	)
	excluded := packet["excluded_counts"].(map[string]int)
	if text != "" || intFromAny(packet["delivered_count"], 0) != 0 ||
		excluded["ambiguous_subject"] != 2 {
		t.Fatalf("ambiguous same-label current states leaked into delivery: packet=%#v text=%q", packet, text)
	}
}

func TestReversibleBodyMetadataStoresHistoryWithoutExactSourceText(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	proposal := reversibleStateProposal("body", "set", "Mina", "body_condition",
		"Mina felt nauseous.", "felt nauseous")
	proposal["sensitivity"] = "reproductive"
	proposal["value"] = map[string]any{
		"text": "felt nauseous",
		"body": map[string]any{"subtype": "pregnant", "category": "reproductive"},
	}
	result := saveReversibleTurn(t, fake, 1, "rev-body-metadata",
		"Mina felt nauseous.", []any{proposal}, characters, nil)
	if len(fake.savedStatusEvents) != 1 || len(fake.returnStatusCurrent) != 0 {
		t.Fatalf("sensitive observation was not retained as history-only: events=%#v current=%#v", fake.savedStatusEvents, fake.returnStatusCurrent)
	}
	if fake.savedStatusEvents[0].EventState != "history_only" ||
		!strings.Contains(fake.savedStatusEvents[0].EvidenceJSON, "private_or_sensitive_history_only") {
		t.Fatalf("sensitive observation became current: %#v", fake.savedStatusEvents[0])
	}
	for _, decision := range result.SkipReasons {
		if extractionStringFromAny(decision["reason"]) == "body_metadata_not_evidence_bound" {
			t.Fatalf("exact-text gate still rejected broad semantic storage: %#v", result.SkipReasons)
		}
	}
}

func TestReversibleStateSameSourceSlotStoresDistinctObservations(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	first := reversibleStateProposal("emotion", "set", "Mina", "current_emotion",
		"Mina looked calm, then became afraid.", "looked calm")
	second := reversibleStateProposal("emotion", "change", "Mina", "current-emotion",
		"Mina looked calm, then became afraid.", "became afraid")
	result := saveReversibleTurn(t, fake, 1, "rev-duplicate-slot",
		"Mina looked calm, then became afraid.", []any{first, second}, characters, nil)
	if len(fake.savedStatusEvents) != 2 || len(fake.returnStatusCurrent) != 1 {
		t.Fatalf("same-slot observations were not both retained: events=%#v current=%#v",
			fake.savedStatusEvents, fake.returnStatusCurrent)
	}
	firstEvidence := map[string]any{}
	secondEvidence := map[string]any{}
	if err := json.Unmarshal([]byte(fake.savedStatusEvents[0].EvidenceJSON), &firstEvidence); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(fake.savedStatusEvents[1].EvidenceJSON), &secondEvidence); err != nil {
		t.Fatal(err)
	}
	if extractionStringFromAny(firstEvidence["source_unit_id"]) == extractionStringFromAny(secondEvidence["source_unit_id"]) {
		t.Fatalf("same-slot observations collided on source identity: %#v", fake.savedStatusEvents)
	}
	projection := currentReversibleProjection(t, fake, reversibleEmotionStatusKey)
	current := mapFromAny(mapFromAny(mapFromAny(projection["slots"])["current_emotion"])["value"])
	if extractionStringFromAny(current["text"]) != "became afraid" {
		t.Fatalf("ordered same-slot observations did not leave the last current value: %#v", projection)
	}
	for _, decision := range result.SkipReasons {
		if extractionStringFromAny(decision["reason"]) == "duplicate_source_slot_rejected" {
			t.Fatalf("duplicate-slot gate still rejected semantic storage: %#v", result.SkipReasons)
		}
	}
}

func TestReversibleStateNegativeOrUncertainClaimCannotMutateCurrent(t *testing.T) {
	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	negative := reversibleStateProposal("body", "set", "Mina", "pregnancy",
		"Mina said she was not pregnant.", "pregnant")
	negative["polarity"] = "negative"
	negative["sensitivity"] = "reproductive"
	negative["value"] = map[string]any{
		"text": "pregnant",
		"body": map[string]any{"subtype": "pregnant", "category": "reproductive"},
	}
	saveReversibleTurn(t, fake, 1, "rev-negative-pregnancy",
		"Mina said she was not pregnant.", []any{negative}, characters, nil)
	if len(fake.returnStatusCurrent) != 0 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("nonaffirmative claim mutated current: current=%#v events=%#v",
			fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	event := fake.savedStatusEvents[0]
	if event.EventState != "history_only" ||
		!strings.Contains(event.EvidenceJSON, "nonaffirmative_claim_history_only") {
		t.Fatalf("nonaffirmative claim was not preserved only as history: %#v", event)
	}
}

func TestReversibleStateAffirmativeLabelCannotBypassSourceEpistemicGuard(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		excerpt string
	}{
		{name: "english negation", subject: "Mina", excerpt: "Mina was not at the north gate."},
		{name: "english possibility", subject: "Mina", excerpt: "Mina might be at the north gate."},
		{name: "english question", subject: "Mina", excerpt: "Was Mina at the north gate?"},
		{name: "english denial", subject: "Mina", excerpt: "Mina denied being at the north gate."},
		{name: "english allegation", subject: "Mina", excerpt: "Mina allegedly stood at the north gate."},
		{name: "english appearance", subject: "Mina", excerpt: "Mina seemed to be at the north gate."},
		{name: "english unlikely", subject: "Mina", excerpt: "Mina was unlikely to be at the north gate."},
		{name: "korean negation", subject: "미나", excerpt: "미나는 북문에 있지 않았다."},
		{name: "korean uncertainty", subject: "미나", excerpt: "미나는 북문에 있을지도 모른다."},
		{name: "korean question", subject: "미나", excerpt: "미나는 북문에 있습니까?"},
		{name: "korean denial", subject: "미나", excerpt: "미나는 북문에 있었다는 말을 부인했다."},
		{name: "korean appearance", subject: "미나", excerpt: "미나는 북문에 있는 듯했다."},
		{name: "korean inference", subject: "미나", excerpt: "미나는 북문에 있다고 추정된다."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proposal := reversibleStateProposal(
				"location", "set", test.subject, "current_location", test.excerpt, "북문",
			)
			eligible, reason := reversibleStateCurrentClaimEligibility(
				map[string]any{},
				proposal,
				test.excerpt,
			)
			if eligible || reason != "source_epistemic_guard_history_only" {
				t.Fatalf("eligible=%v reason=%q for %q", eligible, reason, test.excerpt)
			}
		})
	}

	affirmative := reversibleStateProposal(
		"location", "set", "Mina", "current_location",
		"Mina stood at the north gate.", "stood at the north gate",
	)
	if eligible, reason := reversibleStateCurrentClaimEligibility(
		map[string]any{},
		affirmative,
		extractionStringFromAny(affirmative["evidence_excerpt"]),
	); !eligible || reason != "" {
		t.Fatalf("plain affirmative was blocked: eligible=%v reason=%q", eligible, reason)
	}
	koreanAffirmative := reversibleStateProposal(
		"possession", "set", "미나", "held_item",
		"미나는 지도를 들고 있다.", "지도를 들고 있다",
	)
	if eligible, reason := reversibleStateCurrentClaimEligibility(
		map[string]any{},
		koreanAffirmative,
		extractionStringFromAny(koreanAffirmative["evidence_excerpt"]),
	); !eligible || reason != "" {
		t.Fatalf("Korean affirmative containing 지도 was blocked: eligible=%v reason=%q", eligible, reason)
	}

	fake := newIdentityRecordingStore()
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	mislabeled := reversibleStateProposal(
		"location", "set", "Mina", "current_location",
		"Mina might be at the north gate.", "at the north gate",
	)
	saveReversibleTurn(t, fake, 1, "rev-mislabeled-possibility",
		"Mina might be at the north gate.", []any{mislabeled}, characters, nil)
	if len(fake.returnStatusCurrent) != 0 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("mislabeled possibility mutated current: current=%#v events=%#v",
			fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	event := fake.savedStatusEvents[0]
	if event.EventState != "history_only" ||
		!strings.Contains(event.EvidenceJSON, "source_epistemic_guard_history_only") {
		t.Fatalf("mislabeled possibility was not retained as history-only: %#v", event)
	}

	body := reversibleStateProposal(
		"body", "set", "Mina", "pregnancy",
		"Mina was described by Rowan as pregnant.", "pregnant",
	)
	body["sensitivity"] = "reproductive"
	body["value"] = map[string]any{
		"text": "pregnant",
		"body": map[string]any{"subtype": "pregnant", "category": "reproductive"},
	}
	if eligible, reason := reversibleStateCurrentClaimEligibility(
		map[string]any{}, body, extractionStringFromAny(body["evidence_excerpt"]),
	); eligible || reason != "body_assertion_span_guard_history_only" {
		t.Fatalf("reported body claim bypassed structural assertion guard: eligible=%v reason=%q", eligible, reason)
	}
}

func TestReversibleStateHistoryOnlyValidityAndRollbackRestore(t *testing.T) {
	fake := newIdentityRecordingStore()
	fake.reviewCanonicalLabel("Mina")
	characters := []any{map[string]any{"name": "Mina", "entity_type": "character"}}
	current := reversibleStateProposal("location", "set", "Mina", "current_location",
		"Mina was at the north gate on 1423-04-12.", "at the north gate")
	current["validity"] = map[string]any{"valid_to": "1423-04-12"}
	saveReversibleTurn(t, fake, 20, "rev-current-location",
		"Mina was at the north gate on 1423-04-12.", []any{current}, characters, nil)

	flashback := reversibleStateProposal("location", "set", "Mina", "current_location",
		"In a flashback, Mina stood in the old palace.", "stood in the old palace")
	flashback["scene_scope"] = "flashback"
	saveReversibleTurn(t, fake, 21, "rev-flashback-location",
		"In a flashback, Mina stood in the old palace.", []any{flashback}, characters, nil)
	projection := currentReversibleProjection(t, fake, reversibleLocationStatusKey)
	slot := mapFromAny(mapFromAny(projection["slots"])["current_location"])
	if extractionStringFromAny(mapFromAny(slot["value"])["text"]) != "at the north gate" {
		t.Fatalf("flashback location overwrote current location: %#v", projection)
	}
	if fake.savedStatusEvents[len(fake.savedStatusEvents)-1].EventState != "history_only" {
		t.Fatalf("flashback location was not preserved as history-only")
	}

	clock := map[string]any{
		"version":          storyClockContractVersion,
		"observation_kind": "absolute",
		"precision":        "exact",
		"absolute":         map[string]any{"date": "1423-04-13"},
	}
	packet, text := buildReversibleStatePacket(
		fake.returnStatusCurrent, clock, 10000,
		prepareTurnRequestEntityScope{Scene: []string{"Mina"}},
	)
	excluded, ok := packet["excluded_counts"].(map[string]int)
	if intFromAny(packet["active_count"], 0) != 0 ||
		!ok || excluded["outside_validity"] != 1 ||
		text != "" {
		t.Fatalf("expired explicit validity remained deliverable: packet=%#v text=%q", packet, text)
	}

	fake.returnStatusCurrent = nil
	restored, err := restoreReversibleStateCurrentAfterRollback(context.Background(), fake, "story-session")
	if err != nil || restored != 1 || len(fake.returnStatusCurrent) != 1 {
		t.Fatalf("rollback restore failed: restored=%d err=%v current=%#v", restored, err, fake.returnStatusCurrent)
	}
	restoredProjection := currentReversibleProjection(t, fake, reversibleLocationStatusKey)
	if extractionStringFromAny(mapFromAny(mapFromAny(restoredProjection["slots"])["current_location"])["value"]) == "stood in the old palace" {
		t.Fatalf("rollback restore selected history-only flashback: %#v", restoredProjection)
	}
}

func TestReversibleStateProviderSchemaStaysOpenWhileRuntimeValidatesProjection(t *testing.T) {
	schema := proxyCriticTopLevelJSONSchema()
	properties := mapFromAny(schema["properties"])
	if schema["additionalProperties"] != true || len(mapFromAny(properties["reversible_state"])) != 0 {
		t.Fatalf("provider schema restored a fixed reversible-state vocabulary: %#v", schema)
	}

	invalid := reversibleStateProposal("body", "set", "Mina", "pregnancy", "Mina smiled.", "pregnant")
	invalid["sensitivity"] = "ordinary"
	invalid["value"] = map[string]any{
		"text": "pregnant",
		"body": map[string]any{"subtype": "pregnancy", "category": "reproductive"},
	}
	if err := validateReversibleStateProposal(invalid); err == nil {
		t.Fatalf("reproductive body state with ordinary sensitivity was accepted")
	}
	reversed := reversibleStateProposal("location", "set", "Mina", "current_location",
		"Mina stayed there from 1423-04-15 to 1423-04-12.", "stayed there")
	reversed["validity"] = map[string]any{"valid_from": "1423-04-15", "valid_to": "1423-04-12"}
	if err := validateReversibleStateProposal(reversed); err == nil {
		t.Fatalf("reversed validity interval was accepted")
	}
}

func TestPrepareTurnAppliesReversibleStateToActualPayloadPlan(t *testing.T) {
	fake := newIdentityRecordingStore()
	fake.returnStatusCurrent = []store.StatusCurrentValue{{
		ID:            1,
		ChatSessionID: "payload-reversible",
		RegistryID:    1,
		StatusKey:     reversibleLocationStatusKey,
		OwnerScope:    reversibleStateOwnerScope,
		OwnerID:       "reviewed-mina",
		OwnerLabel:    "Mina",
		ValueKind:     "object",
		ValueJSON: mustCompactJSON(map[string]any{
			"version":           reversibleStateContractVersion,
			"domain":            "location",
			"subject_entity_id": "reviewed-mina",
			"subject_label":     "Mina",
			"slots": map[string]any{
				"current_location": map[string]any{
					"value":       map[string]any{"text": "at the north gate"},
					"validity":    map[string]any{},
					"visibility":  "public",
					"sensitivity": "ordinary",
				},
			},
		}),
		EvidenceJSON: `{}`,
		SourceTurn:   4,
		WriteState:   "current",
	}}
	server := setupTestServer()
	server.Store = fake
	_, response := prepareTurnPerfRequest(t, server, `{
		"chat_session_id":"payload-reversible",
		"turn_index":5,
		"raw_user_input":"Mina looks around.",
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"guide_strength":"none","injection_enabled":true,"input_context_enabled":false,"max_injection_chars":500}
	}`)
	plan := mapFromAny(response["payload_application_plan"])
	auxiliary := extractionStringFromAny(plan["auxiliary_text"])
	if !strings.Contains(auxiliary, "[Reversible Current State]") ||
		!strings.Contains(auxiliary, "at the north gate") {
		t.Fatalf("typed reversible state did not reach the applied payload plan: %#v", plan)
	}
	pack := mapFromAny(response["injection_pack"])
	statePacket := mapFromAny(pack["reversible_state_packet"])
	if intFromAny(statePacket["delivered_count"], 0) != 1 ||
		extractionStringFromAny(pack["reversible_state_text"]) == "" {
		t.Fatalf("compact packet drifted from applied reversible payload: %#v", pack)
	}
}

func TestPrepareTurnAssemblySanitizesLegacyReversibleCharacterState(t *testing.T) {
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene",
		Content:   `{"present_entities":["Mina"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{{
			ChatSessionID: "payload-reversible", CharacterName: "Mina",
			StatusJSON: `{"injury":"broken arm","current_location":"old tower","emotion":"afraid","role":"captain"}`,
		}},
		nil, nil, nil, nil, nil, nil,
		3, 1000,
		"Mina looks around.",
		"default",
		nil, nil, nil,
		perspective,
	)
	if strings.Contains(assembly.CharacterText, "broken arm") ||
		strings.Contains(assembly.CharacterText, "old tower") ||
		strings.Contains(assembly.CharacterText, "afraid") ||
		!strings.Contains(assembly.CharacterText, "captain") {
		t.Fatalf("actual prepareTurn assembly leaked legacy reversible state: %q", assembly.CharacterText)
	}
}

func TestPrepareTurnReservesDynamicBudgetForRelevantReversibleState(t *testing.T) {
	fake := newIdentityRecordingStore()
	fake.returnMemories = []store.Memory{{
		ID: 1, TurnIndex: 4,
		SummaryJSON: `{"turn_summary":"Mina remembers a deliberately long account of the north gate preparations, guards, supplies, weather, visitors, tools, and every earlier conversation."}`,
	}}
	fake.returnStatusCurrent = []store.StatusCurrentValue{{
		ID: 1, ChatSessionID: "payload-reversible-budget", RegistryID: 1,
		StatusKey: reversibleLocationStatusKey, OwnerScope: reversibleStateOwnerScope,
		OwnerID: "mina-state-root", OwnerLabel: "Mina", ValueKind: "object",
		ValueJSON: mustCompactJSON(map[string]any{
			"version": reversibleStateContractVersion, "domain": "location",
			"subject_entity_id": "mina-state-root", "subject_label": "Mina",
			"slots": map[string]any{"current_location": map[string]any{
				"value":    map[string]any{"text": "at the north gate"},
				"validity": map[string]any{}, "visibility": "public", "sensitivity": "ordinary",
			}},
		}),
		EvidenceJSON: `{}`, SourceTurn: 4, WriteState: "current",
	}}
	server := setupTestServer()
	server.Store = fake
	_, response := prepareTurnPerfRequest(t, server, `{
		"chat_session_id":"payload-reversible-budget",
		"turn_index":5,
		"raw_user_input":"Mina looks around.",
		"response_projection":"prepare_turn.production_compact.v1",
		"settings":{"guide_strength":"none","injection_enabled":true,"input_context_enabled":false,"max_injection_chars":120}
	}`)
	pack := mapFromAny(response["injection_pack"])
	statePacket := mapFromAny(pack["reversible_state_packet"])
	if intFromAny(statePacket["delivered_count"], 0) != 1 ||
		!strings.Contains(extractionStringFromAny(pack["reversible_state_text"]), "at the north gate") {
		t.Fatalf("ordinary memory consumed the entire budget before relevant current state: %#v", pack)
	}
	if used := intFromAny(statePacket["used_chars"], 0); used <= 0 || used > 120 {
		t.Fatalf("reversible state budget accounting is invalid: %#v", statePacket)
	}
}

func TestReversibleStateOwnsPerEntityCurrentWhileSceneProjectionRemainsDistinct(t *testing.T) {
	normalized := normalizeCriticExtraction(map[string]any{
		"turn_summary": "Mina moved.",
		"character_deltas": []any{map[string]any{
			"name":              "Mina",
			"location":          "north gate",
			"emotional_posture": "afraid",
			"status": map[string]any{
				"injury": "broken arm",
				"role":   "captain",
			},
			"relationships": map[string]any{"Rowan": "trusted"},
		}},
		"state_deltas": map[string]any{
			"scene_state": map[string]any{
				"location":         "north gate",
				"mood":             "tense",
				"time_state":       "night",
				"present_entities": []any{"Mina"},
			},
		},
	})
	deltas := sliceFromAny(normalized["character_deltas"])
	if len(deltas) != 1 {
		t.Fatalf("character delta disappeared: %#v", normalized)
	}
	character := mapFromAny(deltas[0])
	status := mapFromAny(character["status"])
	if extractionStringFromAny(character["location"]) != "north gate" ||
		extractionStringFromAny(character["emotional_posture"]) != "afraid" ||
		extractionStringFromAny(status["injury"]) != "broken arm" || extractionStringFromAny(status["role"]) != "captain" ||
		character["relationships"] == nil {
		t.Fatalf("raw character observations were deleted during collection: %#v", character)
	}
	current := mapFromAny(sanitizeLegacyReversibleCharacterDeltas([]any{character})[0])
	currentStatus := mapFromAny(current["status"])
	if current["location"] != nil || current["emotional_posture"] != nil ||
		currentStatus["injury"] != nil || extractionStringFromAny(currentStatus["role"]) != "captain" ||
		current["relationships"] == nil {
		t.Fatalf("reversible observations entered the durable current-state projection: %#v", current)
	}
	scene := mapFromAny(mapFromAny(normalized["state_deltas"])["scene_state"])
	if extractionStringFromAny(scene["location"]) != "north gate" ||
		extractionStringFromAny(scene["mood"]) != "tense" ||
		extractionStringFromAny(scene["time_state"]) != "night" ||
		scene["present_entities"] == nil {
		t.Fatalf("global scene projection was confused with per-entity reversible state: %#v", scene)
	}

	oldStatus := map[string]any{"injury": "broken arm", "role": "captain"}
	if rendered := prepareTurnSurfaceText(sanitizeLegacyReversibleMap(oldStatus)); strings.Contains(rendered, "broken arm") ||
		!strings.Contains(rendered, "captain") {
		t.Fatalf("legacy stored status leaked through prepare filtering: %q", rendered)
	}
}
