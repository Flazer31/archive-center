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

func Test39BHabitEvidenceSingleOccurrenceRemainsSupportOnly(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	unit := habitEvidenceTestUnit(
		"session-single", "revision-1", 1, "habit-unit-1", 101,
		"entity-mira", "Mira", "checks_exits", "checks every exit", "occurrence",
		"before_sitting", "before sitting", "", "", "",
		"Mira checks every exit before sitting.", "owner_private",
	)
	result := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-single", []*store.PreciseMemoryUnit{unit}, time.Unix(100, 0), &result)

	if result.Errors != 0 || result.HabitEvidenceCurrent != 1 || result.HabitEvidenceEvents != 1 {
		t.Fatalf("habit evidence result=%+v", result)
	}
	if len(fake.returnStatusCurrent) != 1 || len(fake.savedStatusEvents) != 1 || len(fake.savedStatusDefinitions) != 1 {
		t.Fatalf("current=%d events=%d definitions=%d", len(fake.returnStatusCurrent), len(fake.savedStatusEvents), len(fake.savedStatusDefinitions))
	}
	projection := habitEvidenceJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	counts := mapFromAny(projection["evidence_counts"])
	if extractionStringFromAny(projection["authority_class"]) != "support_hypothesis" ||
		extractionStringFromAny(projection["disposition"]) != "evidence_only" ||
		extractionStringFromAny(projection["promotion_state"]) != "not_evaluated" ||
		boolFromAny(projection["repeated_support_observed"]) || intFromAny(counts["support_total"], 0) != 1 {
		t.Fatalf("single occurrence was promoted or miscounted: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
	for _, forbidden := range []string{"confidence", "score", "quiet_turns", "dormant_after_quiet_turns"} {
		if habitEvidenceHasKey(projection, forbidden) {
			t.Fatalf("habit projection contains forbidden %q: %s", forbidden, fake.returnStatusCurrent[0].ValueJSON)
		}
	}
	options := habitEvidenceJSONMap(t, fake.savedStatusDefinitions[0].OptionsJSON)
	if boolFromAny(options["fixed_count_promotion"]) || boolFromAny(options["fixed_score_promotion"]) || boolFromAny(options["quiet_turn_dormancy"]) || boolFromAny(options["stable_habit_inference"]) {
		t.Fatalf("habit schema enables promotion shortcut: %s", fake.savedStatusDefinitions[0].OptionsJSON)
	}
	evidence := habitEvidenceJSONMap(t, fake.savedStatusEvents[0].EvidenceJSON)
	if extractionStringFromAny(evidence["source_revision"]) != "revision-1" ||
		intFromAny(evidence["root_evidence_id"], 0) != 101 ||
		extractionStringFromAny(evidence["evidence_excerpt"]) != unit.EvidenceExcerpt {
		t.Fatalf("habit event lost exact source lineage: %s", fake.savedStatusEvents[0].EvidenceJSON)
	}
}

func Test39BHabitEvidencePreservesRepeatedSupportCounterevidenceExceptionsAndContext(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	units := []*store.PreciseMemoryUnit{
		habitEvidenceTestUnit("session-mixed", "revision-1", 1, "support-1", 201, "entity-mira", "Mira", "checks_exits", "checks the exits", "occurrence", "on_arrival", "on arrival", "", "", "", "Mira checks the exits on arrival.", "owner_private"),
		habitEvidenceTestUnit("session-mixed", "revision-2", 3, "support-2", 202, "entity-mira", "Mira", "checks_exits", "always checks the exits", "explicit_pattern", "in_unfamiliar_rooms", "in unfamiliar rooms", "", "", "", "Mira always checks the exits in unfamiliar rooms.", "owner_private"),
		habitEvidenceTestUnit("session-mixed", "revision-3", 5, "counter-1", 203, "entity-mira", "Mira", "checks_exits", "walks in without checking", "counterexample", "with_rook", "when Rook is beside her", "entity-rook", "Rook", "Rook", "Mira walks in without checking when Rook is beside her.", "owner_private"),
		habitEvidenceTestUnit("session-mixed", "revision-4", 6, "exception-1", 204, "entity-mira", "Mira", "checks_exits", "skips the check", "exception", "during_drills", "during drills", "", "", "", "Mira skips the check during drills.", "owner_private"),
	}
	result := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-mixed", units, time.Unix(600, 0), &result)

	if result.Errors != 0 || result.HabitEvidenceCurrent != 4 || result.HabitEvidenceEvents != 4 || len(fake.returnStatusCurrent) != 1 || len(fake.savedStatusEvents) != 4 {
		t.Fatalf("mixed habit evidence result=%+v current=%#v events=%#v", result, fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	projection := habitEvidenceJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	counts := mapFromAny(projection["evidence_counts"])
	if intFromAny(counts["occurrence"], 0) != 1 || intFromAny(counts["explicit_pattern"], 0) != 1 ||
		intFromAny(counts["counterexample"], 0) != 1 || intFromAny(counts["exception"], 0) != 1 ||
		!boolFromAny(projection["repeated_support_observed"]) || extractionStringFromAny(projection["interpretation_state"]) != "contested_evidence" {
		t.Fatalf("mixed evidence was flattened or promoted: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
	if len(sliceFromAny(projection["contexts"])) != 4 || len(sliceFromAny(projection["counterparts"])) != 1 || len(sliceFromAny(projection["source_turns"])) != 4 {
		t.Fatalf("context/counterpart/time distribution was lost: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
	if extractionStringFromAny(projection["authority_class"]) != "support_hypothesis" || extractionStringFromAny(projection["disposition"]) != "evidence_only" {
		t.Fatalf("repeated evidence became a stable trait: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
}

func Test39BHabitEvidenceExactAndRootReplayDoNotIncreaseWeight(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	first := habitEvidenceTestUnit("session-replay", "revision-1", 2, "habit-original", 301, "entity-mira", "Mira", "checks_exits", "checks the exits", "occurrence", "", "", "", "", "", "Mira checks the exits.", "owner_private")
	firstResult := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-replay", []*store.PreciseMemoryUnit{first}, time.Unix(200, 0), &firstResult)

	exactReplay := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-replay", []*store.PreciseMemoryUnit{first}, time.Unix(201, 0), &exactReplay)
	duplicateSummary := habitEvidenceTestUnit("session-replay", "revision-2", 3, "habit-summary-copy", 301, "entity-mira", "Mira", "checks_exits", "checks the exits", "explicit_pattern", "", "", "", "", "", "Mira checks the exits.", "owner_private")
	rootReplay := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-replay", []*store.PreciseMemoryUnit{duplicateSummary}, time.Unix(202, 0), &rootReplay)

	if firstResult.HabitEvidenceCurrent != 1 || firstResult.HabitEvidenceEvents != 1 ||
		exactReplay.HabitEvidenceCurrent != 0 || exactReplay.HabitEvidenceEvents != 0 ||
		rootReplay.HabitEvidenceCurrent != 0 || rootReplay.HabitEvidenceEvents != 0 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("replay changed habit weight: first=%+v exact=%+v root=%+v events=%#v", firstResult, exactReplay, rootReplay, fake.savedStatusEvents)
	}
	projection := habitEvidenceJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	if intFromAny(mapFromAny(projection["evidence_counts"])["total"], 0) != 1 || len(stringsFromAny(projection["evidence_fingerprints"])) != 1 {
		t.Fatalf("root evidence replay increased aggregate: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
}

func Test39BHabitEvidenceSeparatesSubjectBehaviorAndSession(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	for _, test := range []struct {
		sid, revision, unitID, subjectID, subject, behaviorKey, behaviorExpression, evidence string
	}{
		{"session-one", "revision-1", "mira-exits", "entity-mira", "Mira", "checks_exits", "checks exits", "Mira checks exits."},
		{"session-one", "revision-1", "mira-cups", "entity-mira", "Mira", "aligns_cups", "aligns cups", "Mira aligns cups."},
		{"session-one", "revision-1", "rook-exits", "entity-rook", "Rook", "checks_exits", "checks exits", "Rook checks exits."},
		{"session-two", "revision-2", "mira-exits", "entity-mira", "Mira", "checks_exits", "checks exits", "Mira checks exits."},
	} {
		result := artifactSaveResult{}
		unit := habitEvidenceTestUnit(test.sid, test.revision, 1, test.unitID, int64(len(fake.savedStatusEvents)+401), test.subjectID, test.subject, test.behaviorKey, test.behaviorExpression, "occurrence", "", "", "", "", "", test.evidence, "owner_private")
		srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), test.sid, []*store.PreciseMemoryUnit{unit}, time.Unix(400, 0), &result)
		if result.HabitEvidenceCurrent != 1 {
			t.Fatalf("separated habit write failed for %+v: %+v", test, result)
		}
	}
	if len(fake.returnStatusCurrent) != 4 {
		t.Fatalf("subject, behavior, or session collapsed: %#v", fake.returnStatusCurrent)
	}
}

func Test39BHabitAdmissionRequiresExactExpressionsAndBlocksLegacyTraitWrites(t *testing.T) {
	evidence := "Mira watches the door when Rook arrives."
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rook"}}},
		"habit_observations": []any{
			map[string]any{
				"subject_entity": "Mira", "subject_entity_expression": "Mira",
				"behavior_key": "watches_doors", "behavior_expression": "watches the door",
				"observation_kind": "occurrence", "context_key": "on_arrival", "context_expression": "when Rook arrives",
				"counterpart": "Rook", "counterpart_expression": "Rook", "evidence_excerpt": evidence,
			},
			map[string]any{
				"subject_entity": "Mira", "subject_entity_expression": "Mira",
				"behavior_key": "invented", "behavior_expression": "checks every window",
				"observation_kind": "occurrence", "evidence_excerpt": evidence,
			},
		},
		"character_deltas": []any{map[string]any{
			"name": "Mira", "personality": map[string]any{"watchful": true}, "speech_style": map[string]any{"tone": "dry"},
			"appearance": map[string]any{"coat": "black"},
		}},
	}
	admitted, trace := admitCriticInteractionLanes(raw, "", evidence)
	items := sliceFromAny(admitted["habit_observations"])
	if len(items) != 2 {
		t.Fatalf("habit candidates were not archived broadly: %#v trace=%#v", items, trace)
	}
	item := mapFromAny(items[0])
	if extractionStringFromAny(item["contract_version"]) != habitObservationContract ||
		extractionStringFromAny(item["admission_state"]) != "committed" ||
		extractionStringFromAny(item["review_state"]) != "source_observed" ||
		extractionStringFromAny(item["visibility"]) != "public" {
		t.Fatalf("habit observation was not normalized to source-bound support evidence: %#v", item)
	}
	if extractionStringFromAny(mapFromAny(items[1])["admission_state"]) != "committed" {
		t.Fatalf("understandable habit candidate was blocked by exact-expression proof: %#v", items[1])
	}
	candidates := interactionAdmissionPreciseMemoryCandidates(admitted)
	if len(candidates) != 2 || candidates[0].truthScope != "support_only" || candidates[0].authorityClass != "support_hypothesis" || candidates[0].subtype != "habit_observation" ||
		candidates[0].admissionState != "committed" || candidates[1].admissionState != "committed" {
		t.Fatalf("habit precise candidate authority is wrong: %#v", candidates)
	}
	protected, incomplete := memoryAdmissionPerspectiveEvidenceScope(admitted)
	if incomplete || protected[normalizeArtifactDedupeText(evidence)] {
		t.Fatalf("public habit evidence was incorrectly protected: protected=%#v incomplete=%v", protected, incomplete)
	}
	delta := mapFromAny(sliceFromAny(admitted["character_deltas"])[0])
	if _, exists := delta["personality"]; !exists {
		t.Fatalf("raw personality candidate was deleted during collection: %#v", delta)
	}
	if _, exists := delta["speech_style"]; !exists {
		t.Fatalf("raw speech-style candidate was deleted during collection: %#v", delta)
	}
	if len(mapFromAny(delta["appearance"])) == 0 {
		t.Fatalf("unrelated character delta was removed: %#v", delta)
	}
	current := mapFromAny(sanitizeLegacyReversibleCharacterDeltas([]any{delta})[0])
	if _, exists := current["personality"]; exists {
		t.Fatalf("legacy personality candidate entered the current-state projection: %#v", current)
	}
	if _, exists := current["speech_style"]; exists {
		t.Fatalf("raw speech-style candidate bypassed the 3.9-D projection: %#v", current)
	}
}

func Test39BHabitCriticContractIsTypedAndDoesNotClaimSpeechStyle(t *testing.T) {
	prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt("session-prompt", 1, "Mira checks the door.", "Rook waits.", nil, nil))
	for _, needle := range []string{
		"habit_observations",
		"habit_observations collect behavior occurrences, patterns, counterexamples, and exceptions",
		"Do not use a fixed count to decide that a habit exists",
		"character_profile_observations collect personality, values, desires, fears, contradictions",
		"voice_observations collect conditional speech and nonverbal style principles broadly",
		"Store traits or principles rather than forcing future dialogue to repeat an example sentence",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("critic prompt missing 3.9-B guard %q", needle)
		}
	}
	if _, _, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary": "ok",
		"habit_observations": []any{
			map[string]any{"subject_entity": "Mira", "behavior_key": "checks doors"},
		},
	}); err != nil {
		t.Fatalf("typed habit lane rejected by critic schema: %v", err)
	}
	sanitized, trace, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary":       "kept",
		"habit_observations": []any{"wrong wire value"},
	})
	if err != nil || sanitized["turn_summary"] != "kept" || intFromAny(trace["dropped_item_count"], 0) != 1 {
		t.Fatalf("invalid habit record was not isolated: sanitized=%#v trace=%#v err=%v", sanitized, trace, err)
	}
	schema := proxyCriticTopLevelJSONSchema()
	if schema["additionalProperties"] != true || len(mapFromAny(mapFromAny(schema["properties"])["records"])) != 0 {
		t.Fatalf("provider critic schema is not sparse top-level: %#v", schema)
	}
}

func Test39BHabitEvidenceRunsAfterCommonMemoryAdmission(t *testing.T) {
	fake := newRelationshipMemoryAdmissionStore()
	srv := NewServer(config.Default())
	srv.Store = fake
	excerpt := "Mira checks the exit before sitting."
	extraction := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}}},
		"habit_observations": []any{map[string]any{
			"contract_version": habitObservationContract,
			"subject_entity":   "Mira", "subject_entity_expression": "Mira",
			"behavior_key": "checks_exits", "behavior_expression": "checks the exit",
			"observation_kind": "occurrence", "context_key": "before_sitting", "context_expression": "before sitting",
			"admission_state": "committed", "review_state": "source_observed",
			"visibility": "owner_private", "evidence_excerpt": excerpt,
		}},
	}
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-admission-habit"), "session-admission-habit", 7,
		extraction, "The room is quiet. "+excerpt, completeTurnEmbeddingConfig{}, time.Unix(700, 0),
	)
	if result.Errors != 0 || result.PreciseMemoryUnits != 1 || result.HabitEvidenceCurrent != 1 || result.HabitEvidenceEvents != 1 {
		t.Fatalf("common admission did not materialize habit evidence: %+v", result)
	}
	if len(fake.admissions) != 1 || len(fake.returnStatusCurrent) != 1 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("admissions=%d current=%#v events=%#v", len(fake.admissions), fake.returnStatusCurrent, fake.savedStatusEvents)
	}
	projection := habitEvidenceJSONMap(t, fake.returnStatusCurrent[0].ValueJSON)
	if extractionStringFromAny(projection["subject_label"]) != "Mira" || extractionStringFromAny(projection["behavior_key"]) != "checks_exits" {
		t.Fatalf("materialized habit evidence owner is wrong: %s", fake.returnStatusCurrent[0].ValueJSON)
	}
}

func Test39BHabitEvidenceRejectsWrongAuthorityAndIgnoresOtherPreciseKinds(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	wrongAuthority := habitEvidenceTestUnit("session-invalid", "revision-1", 1, "wrong-authority", 501, "entity-mira", "Mira", "checks_exits", "checks exits", "occurrence", "", "", "", "", "", "Mira checks exits.", "owner_private")
	wrongAuthority.AuthorityClass = "objective_world_state"
	other := habitEvidenceTestUnit("session-invalid", "revision-1", 1, "other-unit", 502, "entity-mira", "Mira", "checks_exits", "checks exits", "occurrence", "", "", "", "", "", "Mira checks exits.", "owner_private")
	other.Subtype = "relationship_trust"
	result := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-invalid", []*store.PreciseMemoryUnit{wrongAuthority, other}, time.Unix(500, 0), &result)
	if result.HabitEvidenceCurrent != 0 || result.HabitEvidenceEvents != 0 || len(fake.returnStatusCurrent) != 0 || len(fake.savedStatusEvents) != 0 {
		t.Fatalf("wrong authority or other lane created habit evidence: %+v", result)
	}
}

func Test39BHabitEvidenceMalformedCurrentDegradesToHistoryOnly(t *testing.T) {
	fake := newPreciseMemoryRecordingStore()
	srv := &Server{Store: fake}
	unit := habitEvidenceTestUnit("session-malformed", "revision-2", 2, "valid-observation", 601, "entity-mira", "Mira", "checks_exits", "checks exits", "occurrence", "", "", "", "", "", "Mira checks exits.", "owner_private")
	candidate, relevant, reason := habitEvidenceCandidateFromUnit("session-malformed", unit)
	if !relevant || reason != "" {
		t.Fatalf("test candidate invalid: relevant=%v reason=%q", relevant, reason)
	}
	fake.returnStatusCurrent = []store.StatusCurrentValue{{
		ID: 1, ChatSessionID: "session-malformed", StatusKey: habitEvidenceStatusKey,
		OwnerScope: habitEvidenceOwnerScope, OwnerID: candidate.ownerID,
		ValueKind: "object", ValueJSON: `{"version":"malformed"}`, EvidenceJSON: `{}`,
		SourceTurn: 1, WriteState: "current", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}}
	result := artifactSaveResult{}
	srv.saveHabitEvidenceFromPreciseMemoryUnits(context.Background(), "session-malformed", []*store.PreciseMemoryUnit{unit}, time.Unix(602, 0), &result)
	if result.Errors != 0 || result.HabitEvidenceCurrent != 0 || result.HabitEvidenceEvents != 1 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("malformed current did not degrade safely: result=%+v events=%#v", result, fake.savedStatusEvents)
	}
	event := fake.savedStatusEvents[0]
	evidence := habitEvidenceJSONMap(t, event.EvidenceJSON)
	if event.EventState != "history_only" || boolFromAny(evidence["current_projection"]) || extractionStringFromAny(evidence["resolution_status"]) != "malformed_or_mismatched_current_history_only" {
		t.Fatalf("malformed current overwrote projection: %+v", event)
	}
}

func habitEvidenceTestUnit(
	sid, revision string,
	turn int,
	unitID string,
	rootEvidenceID int64,
	subjectID, subject, behaviorKey, behaviorExpression, observationKind string,
	contextKey, contextExpression, counterpartID, counterpart, counterpartExpression string,
	evidence, visibility string,
) *store.PreciseMemoryUnit {
	payload := map[string]any{
		"contract_version":          habitObservationContract,
		"subject_entity":            subject,
		"subject_entity_expression": subject,
		"behavior_key":              behaviorKey,
		"behavior_expression":       behaviorExpression,
		"observation_kind":          observationKind,
		"context_key":               contextKey,
		"context_expression":        contextExpression,
		"counterpart":               counterpart,
		"counterpart_expression":    counterpartExpression,
		"admission_state":           "committed",
		"review_state":              "source_observed",
		"visibility":                visibility,
	}
	return &store.PreciseMemoryUnit{
		UnitID:                unitID,
		ContractVersion:       store.PreciseMemoryUnitContract,
		ChatSessionID:         sid,
		SourceTurnStart:       turn,
		SourceTurnEnd:         turn,
		SourceContract:        completeTurnSourceAcceptanceContract,
		SourceRevision:        revision,
		SourceLogicalTurnID:   "logical-" + revision,
		SourceMessageID:       "message-" + revision,
		SourceGenerationID:    "generation-" + revision,
		SourceContentHash:     "content-hash-" + revision,
		EvidenceExcerpt:       evidence,
		EvidenceHash:          "evidence-hash-" + unitID,
		RootEvidenceID:        rootEvidenceID,
		DirectEvidenceIDsJSON: mustCompactJSON([]int64{rootEvidenceID}),
		Kind:                  "observation",
		Subtype:               "habit_observation",
		PayloadJSON:           mustCompactJSON(payload),
		SubjectEntityID:       subjectID,
		AffectedEntityID:      counterpartID,
		TruthScope:            "support_only",
		EpistemicMode:         "direct",
		AuthorityClass:        "support_hypothesis",
		AdmissionState:        "committed",
		ReviewState:           "source_observed",
		Visibility:            visibility,
		Confidence:            1,
		IdempotencyKey:        "idempotency-" + unitID,
		DerivationVersion:     store.PreciseMemoryUnitContract,
		ExtractorVersion:      "test",
		IndexVersion:          "test",
		LifecycleState:        "active",
		CreatedAt:             time.Unix(int64(turn), 0),
		UpdatedAt:             time.Unix(int64(turn), 0),
	}
}

func habitEvidenceJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	value := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return value
}

func habitEvidenceHasKey(value any, wanted string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == wanted || habitEvidenceHasKey(child, wanted) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if habitEvidenceHasKey(child, wanted) {
				return true
			}
		}
	}
	return false
}
