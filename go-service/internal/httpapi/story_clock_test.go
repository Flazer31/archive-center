package httpapi

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func acceptedStoryClockContext(revision, logicalTurn, generation string) context.Context {
	return contextWithEntityIdentitySource(context.Background(), completeTurnSourceAcceptanceDecision{
		Enabled:       true,
		Accepted:      true,
		Revision:      revision,
		LogicalTurnID: logicalTurn,
		Observation: completeTurnSourceObservation{
			HostChatID:   "host-chat",
			MessageIndex: 3,
			GenerationID: generation,
		},
	})
}

func storyClockEvidence(id int64, turn int, text string) []store.DirectEvidence {
	return []store.DirectEvidence{{
		ID:                  id,
		ChatSessionID:       "story-session",
		TurnAnchor:          turn,
		EvidenceText:        text,
		ArchiveState:        "verified_direct",
		CaptureVerification: "verified",
		CommittedGate:       "auto_grounded_excerpt",
		SourceTurnStart:     turn,
		SourceTurnEnd:       turn,
	}}
}

func saveStoryClockForTest(t *testing.T, fake *turnRecordingStore, turn int, revision string, proposal map[string]any, content string) artifactSaveResult {
	t.Helper()
	excerpt := extractionStringFromAny(proposal["evidence_excerpt"])
	if strings.TrimSpace(content) == strings.TrimSpace(excerpt) {
		content += " The scene continued."
	}
	server := NewServer(config.Default())
	server.Store = fake
	result := artifactSaveResult{}
	server.saveStoryClockFromExtraction(
		acceptedStoryClockContext(revision, "logical-story-turn", "generation-"+revision),
		"story-session",
		turn,
		map[string]any{"story_clock": proposal},
		content,
		storyClockEvidence(int64(turn), turn, excerpt),
		time.Unix(int64(turn), 0),
		&result,
	)
	if result.Errors != 0 {
		t.Fatalf("story clock save failed: %#v", result.ErrorDetails)
	}
	return result
}

func storyClockProposal(kind, scope, precision, excerpt string) map[string]any {
	return map[string]any{
		"version":          storyClockContractVersion,
		"observation_kind": kind,
		"scene_scope":      scope,
		"precision":        precision,
		"evidence_excerpt": excerpt,
		"transition":       "set",
	}
}

func decodeStoryClockValue(t *testing.T, value store.StatusCurrentValue) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal([]byte(value.ValueJSON), &out); err != nil {
		t.Fatalf("decode story clock value: %v", err)
	}
	return out
}

func TestStoryClockCompleteTurnPersistAndPrepareRead(t *testing.T) {
	fake := &turnRecordingStore{}
	server := NewServer(config.Default())
	server.Store = fake
	content := "The bell rang on 1423-04-12 at 13:00."
	excerpt := "1423-04-12 at 13:00"
	extraction := map[string]any{
		"turn_summary":     "The bell fixed the scene time.",
		"importance_score": 5.0,
		"evidence_excerpts": []any{
			excerpt,
		},
		"story_clock": map[string]any{
			"version":          storyClockContractVersion,
			"observation_kind": "absolute",
			"scene_scope":      "current",
			"precision":        "exact",
			"absolute":         map[string]any{"date": "1423-04-12", "time": "13:00"},
			"evidence_excerpt": excerpt,
			"transition":       "set",
		},
	}
	result := server.saveCriticExtractionArtifacts(
		acceptedStoryClockContext("revision-absolute", "logical-absolute", "generation-absolute"),
		"story-session",
		1,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(999999, 0),
	)
	if result.Errors != 0 || len(fake.savedStatusCurrent) != 1 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("production persist path did not write story clock: result=%#v current=%#v events=%#v", result, fake.savedStatusCurrent, fake.savedStatusEvents)
	}
	value := decodeStoryClockValue(t, fake.savedStatusCurrent[0])
	absolute := mapFromAny(value["absolute"])
	if absolute["date"] != "1423-04-12" || value["observation_kind"] != "absolute" {
		t.Fatalf("explicit story date changed: %#v", value)
	}
	if strings.Contains(fake.savedStatusCurrent[0].ValueJSON, "1970") {
		t.Fatalf("server audit time leaked into story clock: %s", fake.savedStatusCurrent[0].ValueJSON)
	}
	projected := resolveCurrentStoryClock(nil, nil, nil, fake.returnStatusCurrent)
	if projected["version"] != storyClockContractVersion ||
		projected["resolution_source"] != "status_current_values" ||
		projected["precision_label"] != "exact" {
		t.Fatalf("prepare read did not prefer canonical status current: %#v", projected)
	}
	evidence := map[string]any{}
	_ = json.Unmarshal([]byte(fake.savedStatusCurrent[0].EvidenceJSON), &evidence)
	if evidence["source_revision"] != "revision-absolute" ||
		evidence["logical_turn_id"] != "logical-absolute" ||
		evidence["source_message_id"] != "host-chat:index:3" ||
		len(sliceFromAny(evidence["direct_evidence_ids"])) != 1 {
		t.Fatalf("source lineage was not preserved: %#v", evidence)
	}
}

func TestStoryClockPartialRemainsPartial(t *testing.T) {
	fake := &turnRecordingStore{}
	excerpt := "It was late autumn, near dusk."
	proposal := storyClockProposal("partial", "current", "partial", excerpt)
	proposal["partial"] = map[string]any{"daypart": "dusk", "season": "late_autumn"}
	saveStoryClockForTest(t, fake, 2, "revision-partial", proposal, excerpt)
	value := decodeStoryClockValue(t, fake.savedStatusCurrent[0])
	if value["precision"] != "partial" || value["observation_kind"] != "partial" || value["absolute"] != nil {
		t.Fatalf("partial clock was promoted to exact: %#v", value)
	}
}

func TestStoryClockRelativeRequiresAnchorAndResolvesWithAnchor(t *testing.T) {
	fake := &turnRecordingStore{}
	relativeExcerpt := "Two days later, the gate opened."
	relative := storyClockProposal("relative", "current", "exact", relativeExcerpt)
	relative["relative"] = map[string]any{"offset": 2, "unit": "day", "anchor": "story_clock.current"}
	saveStoryClockForTest(t, fake, 3, "revision-relative-unanchored", relative, relativeExcerpt)
	if len(fake.savedStatusCurrent) != 1 || len(fake.savedStatusEvents) != 1 {
		t.Fatalf("unanchored relative time was not preserved safely: current=%#v events=%#v", fake.savedStatusCurrent, fake.savedStatusEvents)
	}
	unresolved := decodeStoryClockValue(t, fake.savedStatusCurrent[0])
	if unresolved["observation_kind"] != "relative" || unresolved["precision"] != "unknown" {
		t.Fatalf("unanchored relative time was promoted or discarded: %#v", unresolved)
	}

	fake = &turnRecordingStore{}
	absoluteExcerpt := "The scene began on 1423-04-12."
	absolute := storyClockProposal("absolute", "current", "exact", absoluteExcerpt)
	absolute["absolute"] = map[string]any{"date": "1423-04-12"}
	saveStoryClockForTest(t, fake, 4, "revision-anchor", absolute, absoluteExcerpt)
	saveStoryClockForTest(t, fake, 5, "revision-relative-anchored", relative, relativeExcerpt)
	if len(fake.savedStatusCurrent) != 2 {
		t.Fatalf("anchored relative time did not update current: %#v", fake.savedStatusCurrent)
	}
	value := decodeStoryClockValue(t, fake.savedStatusCurrent[1])
	if value["source_observation_kind"] != "relative" ||
		mapFromAny(value["absolute"])["date"] != "1423-04-14" {
		t.Fatalf("relative clock resolution mismatch: %#v", value)
	}

	rangedExcerpt := "Between two and four days later, the gate opened."
	ranged := storyClockProposal("relative", "current", "bounded_range", rangedExcerpt)
	ranged["relative"] = map[string]any{"offset_min": 2, "offset_max": 4, "unit": "day", "anchor": "story_clock.current"}
	saveStoryClockForTest(t, fake, 6, "revision-relative-range", ranged, rangedExcerpt)
	value = decodeStoryClockValue(t, fake.savedStatusCurrent[2])
	resolvedRange := mapFromAny(value["range"])
	if value["observation_kind"] != "bounded_range" ||
		mapFromAny(resolvedRange["start"])["date"] != "1423-04-16" ||
		mapFromAny(resolvedRange["end"])["date"] != "1423-04-18" {
		t.Fatalf("relative bounded range resolution mismatch: %#v", value)
	}
}

func TestStoryClockUnresolvedCurrentReplacesStaleExact(t *testing.T) {
	fake := &turnRecordingStore{}
	absoluteExcerpt := "The scene began on 1423-04-12."
	absolute := storyClockProposal("absolute", "current", "exact", absoluteExcerpt)
	absolute["absolute"] = map[string]any{"date": "1423-04-12"}
	saveStoryClockForTest(t, fake, 1, "revision-stale-anchor", absolute, absoluteExcerpt)

	relativeExcerpt := "Three hours later, the scene continued."
	relative := storyClockProposal("relative", "current", "exact", relativeExcerpt)
	relative["relative"] = map[string]any{"offset": 3, "unit": "hour", "anchor": "story_clock.current"}
	saveStoryClockForTest(t, fake, 2, "revision-unresolved-hour", relative, relativeExcerpt)
	current := decodeStoryClockValue(t, fake.savedStatusCurrent[len(fake.savedStatusCurrent)-1])
	if current["observation_kind"] != "relative" || current["precision"] != "unknown" {
		t.Fatalf("unresolvable relative time left stale exact current: %#v", current)
	}

	unknownPrecisionExcerpt := "Some number of days later, the scene changed."
	unknownPrecision := storyClockProposal("relative", "current", "unknown", unknownPrecisionExcerpt)
	unknownPrecision["relative"] = map[string]any{"offset": 2, "unit": "day", "anchor": "story_clock.current"}
	saveStoryClockForTest(t, fake, 3, "revision-unknown-relative", unknownPrecision, unknownPrecisionExcerpt)
	current = decodeStoryClockValue(t, fake.savedStatusCurrent[len(fake.savedStatusCurrent)-1])
	if current["observation_kind"] != "relative" || current["precision"] != "unknown" {
		t.Fatalf("unknown-precision relative time was promoted to exact: %#v", current)
	}

	fake = &turnRecordingStore{}
	monthEndExcerpt := "The scene began on 2024-01-31."
	monthEnd := storyClockProposal("absolute", "current", "exact", monthEndExcerpt)
	monthEnd["absolute"] = map[string]any{"date": "2024-01-31"}
	saveStoryClockForTest(t, fake, 4, "revision-month-end", monthEnd, monthEndExcerpt)
	nextMonthExcerpt := "One month later, the scene continued."
	nextMonth := storyClockProposal("relative", "current", "exact", nextMonthExcerpt)
	nextMonth["relative"] = map[string]any{"offset": 1, "unit": "month", "anchor": "story_clock.current"}
	saveStoryClockForTest(t, fake, 5, "revision-invalid-month-normalization", nextMonth, nextMonthExcerpt)
	current = decodeStoryClockValue(t, fake.savedStatusCurrent[len(fake.savedStatusCurrent)-1])
	if current["observation_kind"] != "relative" || current["precision"] != "unknown" {
		t.Fatalf("nonexistent month-end date was normalized to a fabricated exact date: %#v", current)
	}
}

func TestStoryClockBoundedRangeAndNonCurrentScopes(t *testing.T) {
	fake := &turnRecordingStore{}
	rangeExcerpt := "The journey ended sometime from 1423-04-12 through 1423-04-15."
	ranged := storyClockProposal("bounded_range", "current", "bounded_range", rangeExcerpt)
	ranged["range"] = map[string]any{
		"start": map[string]any{"date": "1423-04-12"},
		"end":   map[string]any{"date": "1423-04-15"},
	}
	saveStoryClockForTest(t, fake, 6, "revision-range", ranged, rangeExcerpt)
	value := decodeStoryClockValue(t, fake.savedStatusCurrent[0])
	if value["observation_kind"] != "bounded_range" || value["precision"] != "bounded_range" {
		t.Fatalf("bounded range was flattened: %#v", value)
	}

	for index, scope := range []string{"flashback", "planned", "hypothetical"} {
		excerpt := "A non-current scene time was stated for " + scope + "."
		proposal := storyClockProposal("absolute", scope, "exact", excerpt)
		proposal["absolute"] = map[string]any{"date": "1400-01-01"}
		saveStoryClockForTest(t, fake, 7+index, "revision-"+scope, proposal, excerpt)
	}
	if len(fake.savedStatusCurrent) != 1 {
		t.Fatalf("non-current scope overwrote current clock: %#v", fake.savedStatusCurrent)
	}
	for _, event := range fake.savedStatusEvents[1:] {
		if event.EventState != "recorded" || event.OwnerID == storyClockOwnerID {
			t.Fatalf("non-current observation was not isolated as history: %#v", event)
		}
	}
}

func TestStoryClockUnknownCorrectionReplayAndOlderTurn(t *testing.T) {
	fake := &turnRecordingStore{}
	unknownExcerpt := "The date remained unknown."
	unknown := storyClockProposal("unknown", "current", "unknown", unknownExcerpt)
	saveStoryClockForTest(t, fake, 7, "revision-unknown", unknown, unknownExcerpt)
	if len(fake.savedStatusCurrent) != 1 || decodeStoryClockValue(t, fake.savedStatusCurrent[0])["precision"] != "unknown" {
		t.Fatalf("unknown current clock was discarded or promoted: %#v", fake.savedStatusCurrent)
	}

	firstExcerpt := "The date was 1423-04-12."
	first := storyClockProposal("absolute", "current", "exact", firstExcerpt)
	first["absolute"] = map[string]any{"date": "1423-04-12"}
	saveStoryClockForTest(t, fake, 8, "revision-first", first, firstExcerpt)
	correctedExcerpt := "Correction: the date was 1423-04-13."
	corrected := storyClockProposal("absolute", "current", "exact", correctedExcerpt)
	corrected["absolute"] = map[string]any{"date": "1423-04-13"}
	corrected["transition"] = "correction"
	saveStoryClockForTest(t, fake, 9, "revision-correction", corrected, correctedExcerpt)
	if got := fake.savedStatusEvents[len(fake.savedStatusEvents)-1]; got.EventKind != "correction" || strings.TrimSpace(got.PreviousValueJSON) == "" {
		t.Fatalf("correction did not preserve prior history: %#v", got)
	}
	currentWrites := len(fake.savedStatusCurrent)
	eventWrites := len(fake.savedStatusEvents)
	saveStoryClockForTest(t, fake, 9, "revision-correction", corrected, correctedExcerpt)
	if len(fake.savedStatusCurrent) != currentWrites || len(fake.savedStatusEvents) != eventWrites {
		t.Fatalf("same accepted source replay was not idempotent: current=%d/%d events=%d/%d", len(fake.savedStatusCurrent), currentWrites, len(fake.savedStatusEvents), eventWrites)
	}
	saveStoryClockForTest(t, fake, 8, "revision-first", first, firstExcerpt)
	if len(fake.savedStatusCurrent) != currentWrites || len(fake.savedStatusEvents) != eventWrites {
		t.Fatalf("older replay changed source-fenced history: current=%d/%d events=%d/%d", len(fake.savedStatusCurrent), currentWrites, len(fake.savedStatusEvents), eventWrites)
	}

	olderExcerpt := "An older source said 1400-01-01."
	older := storyClockProposal("absolute", "current", "exact", olderExcerpt)
	older["absolute"] = map[string]any{"date": "1400-01-01"}
	saveStoryClockForTest(t, fake, 4, "revision-older", older, olderExcerpt)
	if len(fake.savedStatusCurrent) != currentWrites {
		t.Fatalf("older turn overwrote current clock: %#v", fake.savedStatusCurrent)
	}
	if len(fake.savedStatusEvents) != eventWrites+1 || fake.savedStatusEvents[len(fake.savedStatusEvents)-1].EventState != "recorded" {
		t.Fatalf("older backfill did not remain historical: %#v", fake.savedStatusEvents)
	}

	retractExcerpt := "The previously stated current date was withdrawn."
	retract := storyClockProposal("unknown", "current", "unknown", retractExcerpt)
	retract["transition"] = "retract"
	saveStoryClockForTest(t, fake, 10, "revision-retract", retract, retractExcerpt)
	retractedCurrent := decodeStoryClockValue(t, fake.savedStatusCurrent[len(fake.savedStatusCurrent)-1])
	lastEvent := fake.savedStatusEvents[len(fake.savedStatusEvents)-1]
	if retractedCurrent["precision"] != "unknown" || lastEvent.EventKind != "retract" || lastEvent.EventState != "retracted" {
		t.Fatalf("retraction left a stale exact current clock: current=%#v event=%#v", retractedCurrent, lastEvent)
	}
}

func TestStoryClockRequiresAcceptedSourceAndDirectEvidence(t *testing.T) {
	excerpt := "The date was 1423-04-12."
	proposal := storyClockProposal("absolute", "current", "exact", excerpt)
	proposal["absolute"] = map[string]any{"date": "1423-04-12"}

	fake := &turnRecordingStore{}
	server := NewServer(config.Default())
	server.Store = fake
	result := artifactSaveResult{}
	server.saveStoryClockFromExtraction(
		context.Background(),
		"story-session",
		10,
		map[string]any{"story_clock": proposal},
		excerpt+" The scene continued.",
		storyClockEvidence(10, 10, excerpt),
		time.Unix(10, 0),
		&result,
	)
	if len(fake.savedStatusCurrent) != 0 || !hasStoryClockSkipReason(result, "accepted_source_required") {
		t.Fatalf("legacy/unaccepted source wrote canonical clock: current=%#v skips=%#v", fake.savedStatusCurrent, result.SkipReasons)
	}

	result = artifactSaveResult{}
	server.saveStoryClockFromExtraction(
		acceptedStoryClockContext("revision-no-evidence", "logical-no-evidence", "generation-no-evidence"),
		"story-session",
		10,
		map[string]any{"story_clock": proposal},
		excerpt+" The scene continued.",
		nil,
		time.Unix(10, 0),
		&result,
	)
	if len(fake.savedStatusCurrent) != 0 || !hasStoryClockSkipReason(result, "direct_evidence_required") {
		t.Fatalf("clock without direct evidence became canonical: current=%#v skips=%#v", fake.savedStatusCurrent, result.SkipReasons)
	}

	for _, mutate := range []func(*store.DirectEvidence){
		func(item *store.DirectEvidence) { item.Tombstoned = true },
		func(item *store.DirectEvidence) { item.SupersededByID = 99 },
		func(item *store.DirectEvidence) { item.RepairNeeded = true },
		func(item *store.DirectEvidence) { item.CaptureVerification = "unverified" },
		func(item *store.DirectEvidence) { item.ChatSessionID = "other-session" },
		func(item *store.DirectEvidence) { item.TurnAnchor = 9 },
		func(item *store.DirectEvidence) { item.ArchiveState = "candidate" },
		func(item *store.DirectEvidence) { item.CommittedGate = "manual" },
		func(item *store.DirectEvidence) { item.EvidenceText = "The date was" },
	} {
		rejected := storyClockEvidence(10, 10, excerpt)
		mutate(&rejected[0])
		result = artifactSaveResult{}
		server.saveStoryClockFromExtraction(
			acceptedStoryClockContext("revision-rejected-evidence", "logical-rejected-evidence", "generation-rejected-evidence"),
			"story-session", 10, map[string]any{"story_clock": proposal},
			excerpt+" The scene continued.", rejected, time.Unix(10, 0), &result,
		)
		if len(fake.savedStatusCurrent) != 0 || !hasStoryClockSkipReason(result, "direct_evidence_required") {
			t.Fatalf("inactive evidence became canonical: current=%#v evidence=%#v skips=%#v", fake.savedStatusCurrent, rejected[0], result.SkipReasons)
		}
	}
}

func hasStoryClockSkipReason(result artifactSaveResult, reason string) bool {
	for _, item := range result.SkipReasons {
		if stringFromMap(item, "surface") == "story_clock" && stringFromMap(item, "reason") == reason {
			return true
		}
	}
	return false
}

func TestStoryClockRollbackProjectionRestore(t *testing.T) {
	fake := &turnRecordingStore{}
	prior := mustCompactJSON(map[string]any{
		"version": storyClockContractVersion, "observation_kind": "absolute",
		"scene_scope": "current", "precision": "exact",
		"absolute": map[string]any{"date": "1423-04-12"}, "source_turn": 8,
	})
	fake.savedStatusEvents = []store.StatusChangeEvent{{
		ID: 10, ChatSessionID: "story-session", RegistryID: 3, StatusKey: storyClockStatusKey,
		OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, EventKind: "set",
		NewValueJSON: prior, EvidenceJSON: `{"source_revision":"prior","current_projection":true}`, SourceTurn: 8,
		EventState: "recorded", CreatedAt: time.Unix(8, 0),
	}}
	restored, err := restoreStoryClockCurrentAfterRollback(context.Background(), fake, "story-session")
	if err != nil || restored != 1 || len(fake.savedStatusCurrent) != 1 {
		t.Fatalf("rollback projection restore failed: restored=%d err=%v current=%#v", restored, err, fake.savedStatusCurrent)
	}
	if decodeStoryClockValue(t, fake.savedStatusCurrent[0])["source_turn"] != float64(8) {
		t.Fatalf("rollback restored the wrong clock: %#v", fake.savedStatusCurrent[0])
	}
}

func TestStoryClockSchemaRejectsFabricatedOrInvalidPrecision(t *testing.T) {
	tests := []map[string]any{
		{
			"version":          storyClockContractVersion,
			"observation_kind": "absolute", "scene_scope": "current", "precision": "exact",
			"absolute": map[string]any{"date": "not-a-date"}, "evidence_excerpt": "bad",
		},
		{
			"version":          storyClockContractVersion,
			"observation_kind": "bounded_range", "scene_scope": "current", "precision": "bounded_range",
			"range":            map[string]any{"start": map[string]any{"date": "1423-04-15"}, "end": map[string]any{"date": "1423-04-12"}},
			"evidence_excerpt": "reversed",
		},
		{
			"version":          storyClockContractVersion,
			"observation_kind": "relative", "scene_scope": "current", "precision": "exact",
			"relative": map[string]any{"offset": math.Inf(1), "unit": "day"}, "evidence_excerpt": "infinite",
		},
		{
			"version":          storyClockContractVersion,
			"observation_kind": "unknown", "scene_scope": "current", "precision": "exact", "evidence_excerpt": "unknown",
		},
		{
			"version":          storyClockContractVersion,
			"observation_kind": "relative", "scene_scope": "current", "precision": "partial",
			"relative":         map[string]any{"offset_min": 2, "offset_max": 2, "unit": "day", "anchor": "story_clock.current"},
			"evidence_excerpt": "invalid precision",
		},
		{
			"version":          storyClockContractVersion,
			"observation_kind": "absolute", "scene_scope": "current", "precision": "exact",
			"absolute":         map[string]any{"date": "1423-04-12"},
			"relative":         map[string]any{"offset": 1, "unit": "day", "anchor": "story_clock.current"},
			"evidence_excerpt": "mixed",
		},
	}
	for index, proposal := range tests {
		if err := validateStoryClockProposal(proposal); err == nil {
			t.Fatalf("invalid story clock proposal %d was accepted: %#v", index, proposal)
		}
	}
}

func TestProxyCriticSchemaLeavesStoryClockVocabularyOpen(t *testing.T) {
	schema := proxyCriticTopLevelJSONSchema()
	properties := mapFromAny(schema["properties"])
	storySchema := mapFromAny(properties["story_clock"])
	if len(storySchema) != 0 {
		t.Fatalf("proxy critic story_clock restored a fixed provider vocabulary: %#v", storySchema)
	}
}
