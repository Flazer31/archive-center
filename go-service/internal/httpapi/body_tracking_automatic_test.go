package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func automaticBodyFixture(t *testing.T, count int) (*Server, *identityAliasLinkRecordingStore) {
	t.Helper()
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	f := newIdentityAliasLinkRecordingStore()
	for i := 0; i < count; i++ {
		name, id := fmt.Sprintf("Woman %03d", i), fmt.Sprintf("woman-%03d", i)
		f.identities = append(f.identities, &store.EntityIdentity{ChatSessionID: "auto", StableEntityID: id, CanonicalLabel: name, EntityKind: "character"})
		f.returnCharStates = append(f.returnCharStates, store.CharacterState{ChatSessionID: "auto", CharacterName: name,
			AppearanceJSON: fmt.Sprintf(`{"gender":"female","species":"elf","age":%d}`, 500+i), TurnIndex: 1})
	}
	clock := storyClockProposal("absolute", "current", "exact", "The calendar says January fifteenth.")
	clock["absolute"] = map[string]any{"date": "1423-01-15"}
	f.returnStatusCurrent = []store.StatusCurrentValue{{ID: 1, SourceTurn: 1, ChatSessionID: "auto", OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, StatusKey: storyClockStatusKey, ValueJSON: mustCompactJSON(clock)}}
	return &Server{Store: f, Cfg: config.Default()}, f
}

func Test46AutomaticBodyTargetsAndIndependentStableRandomPhases(t *testing.T) {
	s, f := automaticBodyFixture(t, 112)
	for _, item := range []struct{ name, gender string }{{"Man", "male"}, {"Unknown", "unknown"}} {
		f.identities = append(f.identities, &store.EntityIdentity{ChatSessionID: "auto", StableEntityID: item.name, CanonicalLabel: item.name, EntityKind: "character"})
		f.returnCharStates = append(f.returnCharStates, store.CharacterState{ChatSessionID: "auto", CharacterName: item.name, AppearanceJSON: mustCompactJSON(map[string]any{"gender": item.gender})})
	}
	path, _ := bodyTrackingSettingsPath()
	before := bodySettingsRequest46(t, s, "auto", http.MethodGet, "")
	if mapFromAny(before["settings"])["cycle_tracking_enabled"] != false {
		t.Fatal("automatic membership enabled the global feature")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read initialized model settings")
	}
	// No per-character selection or reference date is supplied.
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":true,"automatic_pregnancy_enabled":false}`)
	stored, _ := s.loadBodyTrackingConfig("auto")
	if len(stored.Characters) != 112 || stored.AutomaticPregnancyEnabled {
		t.Fatalf("automatic women or independent toggle missing: %d", len(stored.Characters))
	}
	phases := map[string]int{}
	for _, character := range stored.Characters {
		if character.Species != "elf" || character.Cycle.ReferenceKind != "model_initialization" {
			t.Fatalf("species/age handling or model authority changed: %+v", character)
		}
		span := storyTimeBounds(character.Cycle.ReferenceTime)
		current := storyTimeBounds(map[string]any{"date": "1423-01-15"})
		phase := current.dayMinimum - span.dayMinimum
		if !span.valid || phase < 0 || phase >= float64(character.Cycle.CycleDays) {
			t.Fatalf("initial phase outside its configured cycle: %v", phase)
		}
		phases[mustCompactJSON(character.Cycle.ReferenceTime)]++
	}
	// More subjects than dates necessarily permits coincidences; independent
	// phases must still spread across dates, rather than all use the anchor day.
	if len(phases) < 14 {
		t.Fatalf("initial phases are synchronized or not independently spread: %v", phases)
	}
	bytesBefore, _ := os.ReadFile(path)
	restarted := &Server{Store: f}
	bodySettingsRequest46(t, restarted, "auto", http.MethodGet, "")
	if _, err := restarted.initializeAutomaticBodyTracking(context.Background(), "auto"); err != nil {
		t.Fatal(err)
	}
	bytesAfter, _ := os.ReadFile(path)
	if string(bytesBefore) != string(bytesAfter) || len(f.savedStatusEvents)+len(f.savedCharacterStates) != 0 {
		t.Fatal("read/restart/reprocessing resampled or invented observed facts")
	}
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":false,"automatic_pregnancy_enabled":false}`)
	off, _ := s.loadBodyTrackingConfig("auto")
	if !reflect.DeepEqual(off.Characters, stored.Characters) || off.SimulationSeed != stored.SimulationSeed {
		t.Fatal("OFF discarded model phase/seed")
	}
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":true}`)
	on, _ := s.loadBodyTrackingConfig("auto")
	if !reflect.DeepEqual(on, stored) {
		t.Fatal("reenabling resampled an initialized model")
	}
}

func Test46AutomaticBodyCurrentGenderBranchAndObservedReference(t *testing.T) {
	s, f := automaticBodyFixture(t, 2)
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":true}`)
	original, _ := s.loadBodyTrackingConfig("auto")
	f.returnCharStates[0].AppearanceJSON = `{"gender":"male"}`
	effective, _ := s.effectiveBodyTrackingConfig(context.Background(), "auto")
	if len(effective.Characters) != 1 || effective.Characters[0].EntityID != original.Characters[1].EntityID {
		t.Fatal("cached model settings became a manual allowlist")
	}
	// Reverting the source snapshot restores membership without changing phases.
	f.returnCharStates[0].AppearanceJSON = `{"gender":"female","species":"elf"}`
	effective, _ = s.effectiveBodyTrackingConfig(context.Background(), "auto")
	if !reflect.DeepEqual(effective, original) {
		t.Fatal("source rollback changed stored model parameters")
	}
	mapping := map[string]string{}
	for _, character := range original.Characters {
		mapping[character.EntityID] = "branch-" + character.EntityID
		f.identities = append(f.identities, &store.EntityIdentity{ChatSessionID: "branch", StableEntityID: mapping[character.EntityID], CanonicalLabel: character.CharacterName, EntityKind: "character"})
		f.returnCharStates = append(f.returnCharStates, store.CharacterState{ChatSessionID: "branch", CharacterName: character.CharacterName, AppearanceJSON: `{"gender":"female"}`})
	}
	if _, err := s.copyBodyTrackingConfig("auto", "branch", mapping); err != nil {
		t.Fatal(err)
	}
	branch, _ := s.effectiveBodyTrackingConfig(context.Background(), "branch")
	if len(branch.Characters) != len(original.Characters) || branch.SimulationSeed != original.SimulationSeed {
		t.Fatal("branch lost automatic model configuration")
	}
	for i, character := range branch.Characters {
		if character.OriginEntityID != original.Characters[i].OriginEntityID || !reflect.DeepEqual(character.Cycle, original.Characters[i].Cycle) {
			t.Fatal("branch resampled the cycle")
		}
	}
	current := store.StatusCurrentValue{ValueJSON: `{"cycle_reference":{"date":"1423-01-14"}}`}
	if got := bodyTrackingCycleSpec(original.Characters[0], current); got.ReferenceKind != "observed_period_start" || got.ReferenceTime["date"] != "1423-01-14" {
		t.Fatal("model initialization overrode an observed period")
	}
}

func Test46AutomaticBodyWaitsForStoryDateThenInitializesOnce(t *testing.T) {
	s, f := automaticBodyFixture(t, 1)
	clock := f.returnStatusCurrent
	f.returnStatusCurrent = nil
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"automatic_pregnancy_enabled":true}`)
	unknown, _ := s.loadBodyTrackingConfig("auto")
	if len(unknown.Characters[0].Cycle.ReferenceTime) != 0 {
		t.Fatal("PC time invented a story date")
	}
	f.returnStatusCurrent = clock
	initialized, err := s.initializeAutomaticBodyTracking(context.Background(), "auto")
	if err != nil || initialized.Characters[0].Cycle.ReferenceKind != "model_initialization" {
		t.Fatalf("known story date did not initialize: %+v %v", initialized, err)
	}
	if len(f.savedStatusEvents) != 0 {
		t.Fatal("model initialization wrote a period event")
	}
}

type automaticBodyProjectionStore struct {
	*identityAliasLinkRecordingStore
}

func (f *automaticBodyProjectionStore) SaveCharacterState(ctx context.Context, state *store.CharacterState) error {
	f.returnCharStates = append(f.returnCharStates, *state)
	return f.turnRecordingStore.SaveCharacterState(ctx, state)
}

func Test46AutomaticBodyNewWomanWorksWithinProductionExtraction(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	f := &automaticBodyProjectionStore{newIdentityAliasLinkRecordingStore()}
	s := &Server{Store: f, Cfg: config.Default()}
	bodySettingsRequest46(t, s, "auto", http.MethodPut, `{"cycle_tracking_enabled":true}`)
	text := "The calendar says January fifteenth. Mira is an elf woman and records her period beginning today."
	clock := storyClockProposal("absolute", "current", "exact", "The calendar says January fifteenth.")
	clock["absolute"] = map[string]any{"date": "1423-01-15"}
	extraction := map[string]any{"turn_summary": text, "story_clock": clock, "evidence_excerpts": []any{text},
		"entities":         map[string]any{"characters": []any{map[string]any{"name": "Mira"}}},
		"character_deltas": []any{map[string]any{"name": "Mira", "appearance": map[string]any{"gender": "female", "species": "elf"}}},
		"body_events":      []any{map[string]any{"subject_name": "Mira", "kind": "period_start", "semantic_event_key": "first-period", "occurred_at": map[string]any{"date": "1423-01-15"}, "evidence_excerpt": text}},
	}
	result := s.saveCriticExtractionArtifacts(acceptedStoryClockContext("auto-source", "auto-turn", "auto-generation"), "auto", 1, extraction, text, completeTurnEmbeddingConfig{}, time.Unix(123, 0), nil)
	if result.Errors != 0 {
		t.Fatalf("production extraction: %+v", result.ErrorDetails)
	}
	values, _ := s.bodyTrackingCurrentValues(context.Background(), "auto")
	if len(values) != 1 || values[0].OwnerLabel != "Mira" {
		t.Fatalf("new female snapshot was not available to same-turn body projection: %+v skips=%+v", values, result)
	}
	stored, _ := s.loadBodyTrackingConfig("auto")
	if len(stored.Characters) != 1 || stored.Characters[0].Cycle.ReferenceKind != "model_initialization" {
		t.Fatal("new automatic participant was not initialized")
	}
}
