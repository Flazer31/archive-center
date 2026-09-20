package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test46SourceClockCaptureUsesSameAcceptedTurnAcrossMemoryAndState(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Cfg: config.Default(), Store: st}
	proposal := storyClockProposal("absolute", "current", "exact", "It was April twelfth.")
	proposal["absolute"] = map[string]any{"date": "1423-04-12"}
	extraction := map[string]any{
		"turn_summary": "Mina was released; yesterday's promise remains historical.", "story_clock": proposal,
		"temporal_context": map[string]any{"relative_expression": "yesterday", "relative": map[string]any{"anchor": "source_observation", "offset": -1, "unit": "day", "target_kind": "recalled_event"}},
		"state_claims":     []any{map[string]any{"subject": "Mina", "subject_type": "character", "state_slot": "restraint", "transition": "change", "value": "released", "evidence_excerpt": "Mina was released."}},
		"character_deltas": []any{map[string]any{"name": "Mina", "status": map[string]any{"movement": "released"}, "evidence_excerpt": "Mina was released."}},
		"narrative_events": []any{map[string]any{"summary": "Yesterday Mina promised delivery", "evidence_excerpt": "Yesterday Mina promised delivery.", "occurrence_time": map[string]any{"date": "1423-04-11"}}},
	}
	result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext("source-april", "logical-april", "generation-april"), "story-session", 1, extraction, "It was April twelfth. Mina was released. Yesterday Mina promised delivery. The scene continued.", completeTurnEmbeddingConfig{}, time.Unix(999999, 0), nil)
	if result.Errors != 0 || len(st.savedMemories) != 1 || len(st.savedCharacterStates) != 1 {
		t.Fatalf("source persist failed: %+v", result)
	}
	assertDate := func(label string, observed map[string]any) {
		t.Helper()
		if stringFromMap(mapFromAny(mapFromAny(observed["story_clock"])["absolute"]), "date") != "1423-04-12" {
			t.Fatalf("%s borrowed previous/global clock instead of accepted source: %#v", label, observed)
		}
	}
	memory := parseJSONMap(st.savedMemories[0].SummaryJSON)
	temporal := mapFromAny(memory["temporal_context"])
	assertDate("memory", mapFromAny(temporal["observed_at"]))
	if temporal["relative_expression"] != "yesterday" || temporal["occurrence_time"] != nil {
		t.Fatal("original relative wording or item-local occurrence was rewritten")
	}
	fields := store.DecodeCharacterFieldProvenance(st.savedCharacterStates[0].FieldProvenanceJSON)
	assertDate("character field", mapFromAny(fields["/status/movement"]["observed_at"]))
	for _, event := range st.savedStatusEvents {
		if event.StatusKey == narrativeStateStatusKey {
			assertDate("narrative event", mapFromAny(parseJSONMap(event.NewValueJSON)["observed_at"]))
			if stringFromMap(mapFromAny(parseJSONMap(event.StoryClockJSON)["absolute"]), "date") != "1423-04-12" {
				t.Fatalf("event source anchor absent: %+v", event)
			}
		}
	}
}

func Test46SourceClockAnchorSurvivesLaterDateAndSameTurnReplay(t *testing.T) {
	st := &turnRecordingStore{}
	srv := &Server{Cfg: config.Default(), Store: st}
	save := func(turn int, date string) {
		t.Helper()
		proposal := storyClockProposal("absolute", "current", "exact", "The date was confirmed.")
		proposal["absolute"] = map[string]any{"date": date}
		result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext(fmt.Sprint(turn), fmt.Sprint(turn), fmt.Sprint(turn)), "story-session", turn, map[string]any{"turn_summary": "The date was confirmed.", "story_clock": proposal}, "The date was confirmed. The scene continued.", completeTurnEmbeddingConfig{}, time.Unix(int64(turn), 0), nil)
		if result.Errors != 0 {
			t.Fatalf("persist: %+v", result)
		}
		last := *st.savedMemories[len(st.savedMemories)-1]
		last.ID = int64(turn)
		st.returnMemories = append(st.returnMemories, last)
	}
	save(1, "1423-04-12")
	firstJSON := st.savedMemories[0].SummaryJSON
	save(2, "1423-04-12")
	save(9, "1426-08-20")
	firstClock := mapFromAny(mapFromAny(parseJSONMap(firstJSON)["temporal_context"])["observed_at"])
	secondClock := mapFromAny(mapFromAny(parseJSONMap(st.savedMemories[1].SummaryJSON)["temporal_context"])["observed_at"])
	if mustCompactJSON(firstClock) != mustCompactJSON(secondClock) {
		t.Fatal("same-day turn count changed observed story time")
	}
	replay := srv.captureSourceStoryClock(acceptedStoryClockContext("1", "1", "1"), "story-session", 1, map[string]any{"turn_summary": "yesterday"}, "Later recall.")
	if mustCompactJSON(mapFromAny(replay["temporal_context"])["observed_at"]) != mustCompactJSON(firstClock) || st.savedMemories[0].SummaryJSON != firstJSON {
		t.Fatal("same source replay reanchored original memory to the later date")
	}
	legacy := st.returnMemories[0]
	legacy.SummaryJSON = `{"turn_summary":"An old memory without a saved date"}`
	st.returnMemories[0] = legacy
	unknown := srv.captureSourceStoryClock(context.Background(), "story-session", 1, nil, "An old recall.")
	if stringFromMap(mapFromAny(mapFromAny(unknown["temporal_context"])["observed_at"]), "story_time") != "unknown" {
		t.Fatal("legacy source without anchor borrowed the present clock")
	}
}

func Test46CommittedSourceClockReplayKeepsExactAdmissionJSON(t *testing.T) {
	first := map[string]any{"turn_summary": "Mina found the key yesterday.", "evidence_excerpts": []any{"Mina found the key yesterday."}, "temporal_context": map[string]any{"observed_at": map[string]any{"kind": "source_observation", "story_clock": map[string]any{"absolute": map[string]any{"date": "1423-04-12"}}}, "relative_expression": "yesterday"}}
	firstJSON := memoryAdmissionCanonicalResultJSON(first)
	st := &memoryAdmissionWorkerStore{Store: store.NewNoopStore(), source: &store.MemorySourceRevision{SourceRevision: "revision", ChatSessionID: "session", LogicalTurnID: "turn:3", TurnIndex: 3, CombinedContentHash: strings.Repeat("a", 64), LifecycleState: "active", DerivedAdmissionState: "committed", DerivedAdmissionVersion: store.MemoryAdmissionContract, DerivedExtractorVersion: completeTurnCriticPipelineVersion, DerivedIndexVersion: memoryAdmissionIndexVersion, DerivedResultHash: strings.Repeat("b", 64), DerivedResultJSON: firstJSON}}
	srv := &Server{Cfg: config.Default(), Store: st}
	result := srv.saveCriticExtractionArtifacts(contextWithStoredMemorySource(context.Background(), st.source), "session", 3, map[string]any{"turn_summary": "A replacement interpretation", "temporal_context": map[string]any{"observed_at": map[string]any{"story_clock": map[string]any{"absolute": map[string]any{"date": "2000-01-01"}}}}}, "Mina found the key yesterday.", completeTurnEmbeddingConfig{}, time.Unix(300, 0), nil)
	if result.Errors != 0 || len(st.admissions) != 1 || st.admissions[0].ResultJSON != firstJSON || st.admissions[0].Memory.SummaryJSON != firstJSON {
		t.Fatalf("committed source clock changed on replay: errors=%v expected=%s admission=%+v", result.ErrorDetails, firstJSON, st.admissions)
	}
}

func Test46StoryClockPreservesUnknownRangeAndCustomCalendar(t *testing.T) {
	for _, tc := range []struct {
		name  string
		clock map[string]any
	}{
		{"unknown", storyClockProposal("unknown", "current", "unknown", "The day was unknown.")},
		{"range", map[string]any{"version": storyClockContractVersion, "observation_kind": "bounded_range", "scene_scope": "current", "precision": "bounded_range", "evidence_excerpt": "Between the twelfth and fourteenth.", "range": map[string]any{"start": map[string]any{"date": "1423-04-12"}, "end": map[string]any{"date": "1423-04-14"}}}},
		{"custom", map[string]any{"version": storyClockContractVersion, "observation_kind": "absolute", "scene_scope": "current", "precision": "exact", "evidence_excerpt": "Harbor era, day 40.", "calendar": map[string]any{"id": "harbor", "label": "Harbor day 40", "day_index": 40}}},
		{"custom-label", map[string]any{"version": storyClockContractVersion, "observation_kind": "absolute", "scene_scope": "current", "precision": "unknown", "evidence_excerpt": "The festival of moons.", "calendar": map[string]any{"id": "moons", "label": "Festival"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &turnRecordingStore{}
			srv := &Server{Cfg: config.Default(), Store: st}
			result := srv.saveCriticExtractionArtifacts(acceptedStoryClockContext(tc.name, tc.name, tc.name), "story-session", 1, map[string]any{"turn_summary": "Time observation retained.", "story_clock": tc.clock}, stringFromMap(tc.clock, "evidence_excerpt")+" The scene continued.", completeTurnEmbeddingConfig{}, time.Unix(900000, 0), nil)
			if result.Errors != 0 || len(st.savedMemories) != 1 || len(st.savedStatusCurrent) != 1 {
				t.Fatalf("clock/source capture failed: %+v", result)
			}
			captured := mapFromAny(mapFromAny(mapFromAny(parseJSONMap(st.savedMemories[0].SummaryJSON)["temporal_context"])["observed_at"])["story_clock"])
			for _, key := range []string{"precision", "absolute", "range", "calendar"} {
				if mustCompactJSON(captured[key]) != mustCompactJSON(tc.clock[key]) {
					t.Fatalf("%s invented or dropped %s: %#v", tc.name, key, captured)
				}
			}
		})
	}
	st := &turnRecordingStore{}
	custom := storyClockProposal("absolute", "current", "exact", "Harbor era day forty.")
	custom["calendar"] = map[string]any{"id": "harbor", "day_index": 40}
	saveStoryClockForTest(t, st, 1, "custom1", custom, "Harbor era day forty.")
	next := storyClockProposal("relative", "current", "exact", "Two days passed.")
	next["relative"] = map[string]any{"anchor": "story_clock.current", "offset": 2, "unit": "day"}
	saveStoryClockForTest(t, st, 2, "custom2", next, "Two days passed.")
	if intFromAny(mapFromAny(parseJSONMap(st.savedStatusCurrent[len(st.savedStatusCurrent)-1].ValueJSON)["calendar"])["day_index"], 0) != 42 {
		t.Fatal("explicit custom day offset did not use existing source clock owner")
	}
}

func Test46OccurrenceOutcomesPreserveSeriesAndUnfulfilledObligation(t *testing.T) {
	st := &turnRecordingStore{}
	save46LifecycleFixture(t, st, 1, map[string]any{"pending_threads": []any{map[string]any{"title": "Weekly report", "lifecycle_key": "report-series", "schedule": map[string]any{"kind": "standing"}}, map[string]any{"title": "First report", "lifecycle_key": "report-one", "series_key": "report-series"}}}, "The reporting obligation was accepted.")
	for i, outcome := range []string{"missed", "refused", "impossible"} {
		save46LifecycleFixture(t, st, i+2, map[string]any{"state_claims": []any{map[string]any{"subject": "First report", "state_slot": "goal_status", "lifecycle_key": "report-one", "transition": outcome, "value": "Unfulfilled report", "outcome": outcome, "evidence_excerpt": "The report outcome was confirmed."}}}, "The report outcome was confirmed. The standing obligation remained.")
		for _, pending := range st.returnPendingThreads {
			if pending.Status != "open" {
				t.Fatalf("%s implied obligation/series cancellation: %+v", outcome, pending)
			}
		}
		latest := st.savedStatusEvents[len(st.savedStatusEvents)-1]
		if stringFromMap(parseJSONMap(latest.NewValueJSON), "transition") != outcome {
			t.Fatalf("outcome collapsed in history: %+v", latest)
		}
	}
}

func Test46EarlierAcceptedSourceRetainsItsExplicitClockWithoutRewindingCurrent(t *testing.T) {
	for _, coordinate := range []map[string]any{
		{"absolute": map[string]any{"date": "1423-04-12"}},
		{"range": map[string]any{"start": map[string]any{"date": "1423-04-12"}, "end": map[string]any{"date": "1423-04-14"}}},
		{"calendar": map[string]any{"id": "harbor", "label": "Harbor day 40", "day_index": 40}},
	} {
		st := &turnRecordingStore{}
		current := storyClockProposal("absolute", "current", "exact", "The later date was confirmed.")
		current["absolute"] = map[string]any{"date": "1426-08-20"}
		saveStoryClockForTest(t, st, 9, "latest", current, "The later date was confirmed.")
		before := st.savedStatusCurrent[0].ValueJSON
		proposal := storyClockProposal("absolute", "current", "exact", "The original date was confirmed.")
		for key, value := range coordinate {
			proposal[key] = value
			if key == "range" {
				proposal["observation_kind"], proposal["precision"] = "bounded_range", "bounded_range"
			}
		}
		result := (&Server{Cfg: config.Default(), Store: st}).saveCriticExtractionArtifacts(acceptedStoryClockContext("earlier", "earlier", "earlier"), "story-session", 3, map[string]any{"turn_summary": "The original date was confirmed.", "story_clock": proposal}, "The original date was confirmed. The scene continued.", completeTurnEmbeddingConfig{}, time.Unix(300, 0), nil)
		if result.Errors != 0 || len(st.savedMemories) != 1 || len(st.savedStatusCurrent) != 1 || st.returnStatusCurrent[0].ValueJSON != before {
			t.Fatalf("earlier source changed current clock: %+v", result)
		}
		captured := mapFromAny(mapFromAny(mapFromAny(parseJSONMap(st.savedMemories[0].SummaryJSON)["temporal_context"])["observed_at"])["story_clock"])
		for key, value := range coordinate {
			if mustCompactJSON(captured[key]) != mustCompactJSON(value) {
				t.Fatalf("explicit earlier source %s lost: %#v", key, captured)
			}
		}
	}
}

func Test46StoryClockRelativeOffsetPreservesSubsecondSource(t *testing.T) {
	st := &turnRecordingStore{}
	initial := storyClockProposal("absolute", "current", "exact", "The clock showed the precise instant.")
	initial["absolute"] = map[string]any{"datetime": "1423-04-12T10:30:00.5Z"}
	saveStoryClockForTest(t, st, 1, "fractional", initial, "The clock showed the precise instant.")
	next := storyClockProposal("relative", "current", "exact", "One second passed.")
	next["relative"] = map[string]any{"anchor": "story_clock.current", "offset": 1, "unit": "second"}
	saveStoryClockForTest(t, st, 2, "plus-second", next, "One second passed.")
	got := stringFromMap(mapFromAny(parseJSONMap(st.savedStatusCurrent[len(st.savedStatusCurrent)-1].ValueJSON)["absolute"]), "datetime")
	if got != "1423-04-12T10:30:01.5Z" {
		t.Fatalf("relative clock dropped confirmed subsecond precision: %s", got)
	}
}
