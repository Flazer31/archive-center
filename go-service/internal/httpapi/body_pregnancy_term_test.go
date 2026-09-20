package httpapi

import (
	"testing"
)

func Test46PregnancyTermAbsenceAndRestoreReadDoNotInventBirthFact(t *testing.T) {
	srv, _, cfg := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("conception", "1423-01-14"))
	current := bodyCurrent46(t, srv)
	projection := parseJSONMap(current.ValueJSON)
	model := mapFromAny(projection["modeled_pregnancy"])
	birth := mapFromAny(model["modeled_birth_time"])
	if !storyTimeBounds(birth).valid {
		t.Fatalf("accepted model omitted finite term: %v", model)
	}
	for _, tc := range []struct {
		clock map[string]any
		stage string
	}{
		{map[string]any{"date": "1423-02-01"}, "modeled_implanted_pregnancy"},
		{birth, "modeled_birth_completed"},
		{map[string]any{"date": "1426-01-14"}, "modeled_birth_completed"},
	} {
		reading := bodyTrackingModelReading(projection, tc.clock)
		if reading["stage"] != tc.stage || reading["birth_observed"] != false || reading["child_outcome"] != "not_inferred" {
			t.Fatalf("absence/term reading: %#v", reading)
		}
	}
	// Configuration changes do not rewrite an already modeled term.
	cfg.Characters[0].GestationDays = 9000
	frozen := bodyTrackingTermProjection(cfg.Characters[0], projection)
	if mustCompactJSON(mapFromAny(frozen["modeled_pregnancy"])["modeled_birth_time"]) != mustCompactJSON(birth) {
		t.Fatal("settings change shifted an existing pregnancy")
	}
	if bodyCurrent46(t, srv).ValueJSON != current.ValueJSON || projection["pregnancy"] != nil {
		t.Fatal("reading created an observed birth or rewrote saved state")
	}
	late := map[string]any{"date": "1426-01-14"}
	if bodyTrackingCycleReading(cfg.Characters[0], projection, late)["reason"] != "cycle_reference_after_pregnancy_unresolved" {
		t.Fatal("old pregnancy/cycle continued across modeled birth")
	}
	// A restored snapshot is interpreted against current story time, not its save date.
	restored := parseJSONMap(current.ValueJSON)
	if bodyTrackingModelReading(restored, late)["stage"] != "modeled_birth_completed" {
		t.Fatal("restoring old data restarted pregnancy")
	}
}

func Test46PregnancyConfirmationKeepsOriginalModeledTerm(t *testing.T) {
	srv, _, cfg := bodyModelState46Fixture(t, 1)
	saveBodyEvent46(t, srv, 1, bodyExposure46("conception", "1423-01-14"))
	before := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	model := mapFromAny(before["modeled_pregnancy"])
	saveBodyEvent46(t, srv, 2, bodyEvent46("pregnancy_confirmed", "confirmation", "1423-08-01"))
	after := parseJSONMap(bodyCurrent46(t, srv).ValueJSON)
	confirmed := mapFromAny(after["pregnancy"])
	if mustCompactJSON(confirmed["modeled_birth_time"]) != mustCompactJSON(model["modeled_birth_time"]) || confirmed["term_basis"] != "existing_modeled_conception" {
		t.Fatalf("confirmation restarted the gestation clock: %#v", confirmed)
	}
	if bodyTrackingCycleReading(cfg.Characters[0], after, map[string]any{"date": "1426-01-14"})["reason"] == "ordinary_cycle_model_not_applicable_during_observed_pregnancy" {
		t.Fatal("old confirmation still reported an ongoing pregnancy")
	}
}

func Test46ObservedPregnancyTermCustomSpeciesAndUnknownTime(t *testing.T) {
	c := defaultBodyCharacterConfig()
	c.Species, c.GestationDays = "dragon", 1200
	fact := map[string]any{"kind": "pregnancy_confirmed", "status": "confirmed", "occurred_at": map[string]any{"calendar": map[string]any{"id": "dragon", "day_index": 100}}}
	p := bodyTrackingTermProjection(c, map[string]any{"pregnancy": fact})
	clock := func(day int) map[string]any {
		return map[string]any{"calendar": map[string]any{"id": "dragon", "day_index": day}}
	}
	if bodyTrackingCycleReading(c, p, clock(1195))["reason"] != "ordinary_cycle_model_not_applicable_during_observed_pregnancy" {
		t.Fatal("human duration substituted for species settings")
	}
	if bodyTrackingModelReading(p, clock(1300))["stage"] != "modeled_birth_completed" {
		t.Fatal("custom calendar term failed")
	}
	if bodyTrackingCycleReading(c, p, clock(1400))["reason"] != "cycle_reference_after_pregnancy_unresolved" {
		t.Fatal("confirmed pregnancy stayed current after its term")
	}
	unknown := bodyPregnancyTerm(c, map[string]any{"status": "confirmed"}, false)
	if bodyPregnancyTermReading(unknown, clock(5000))["stage"] == "modeled_birth_completed" {
		t.Fatal("unknown dates fabricated birth")
	}
}
