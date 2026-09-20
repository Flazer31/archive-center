package httpapi

import "testing"

func test46BodyCycleSpec() bodyCycleSpec {
	return bodyCycleSpec{ReferenceTime: map[string]any{"date": "2024-01-01"}, ReferenceKind: "author_setting", CycleDays: 28, VariationDays: 2, PeriodDays: 5, LutealMinDays: 10, LutealMaxDays: 16}
}

func Test46BodyCycleUsesStoryDatesAndIndependentRanges(t *testing.T) {
	spec := test46BodyCycleSpec()
	first := assessBodyCycle(spec, map[string]any{"date": "2024-01-01", "source_turn": 1})
	if first["status"] != "estimated" || first["menstruation_estimate"] != true || first["kind"] != "calculated_estimate" || first["knowledge"] != "not_inferred" {
		t.Fatalf("reference meaning changed: %#v", first)
	}
	late := assessBodyCycle(spec, map[string]any{"date": "2024-01-15", "source_turn": 2})
	if late["fertile_window_possible"] != true || late["menstruation_estimate"] != false {
		t.Fatalf("phase estimate: %#v", late)
	}
	same := assessBodyCycle(spec, map[string]any{"date": "2024-01-15", "source_turn": 200})
	if mustCompactJSON(same["cycle_day"]) != mustCompactJSON(late["cycle_day"]) {
		t.Fatal("turn count advanced a cycle")
	}
	wide := assessBodyCycle(spec, map[string]any{"date": "2025-01-15"})
	span := mapFromAny(wide["cycle_day"])
	if span["min"] != float64(1) || span["max"] != float64(30) {
		t.Fatalf("year jump invented exact unobserved periods: %#v", wide)
	}
	if wide["status"] != "estimated" || wide["pregnant"] != nil {
		t.Fatalf("cycle calculation created pregnancy: %#v", wide)
	}
}

func Test46BodyCycleUnknownCustomAndParameterMeaning(t *testing.T) {
	spec := test46BodyCycleSpec()
	for _, clock := range []map[string]any{{"source_turn": 900}, {"date": "2023-12-31"}, {"calendar": map[string]any{"id": "moon", "day_index": 100}}} {
		got := assessBodyCycle(spec, clock)
		if got["status"] != "unknown" {
			t.Fatalf("invented comparable source time: %#v", got)
		}
	}
	spec.ReferenceTime = map[string]any{"calendar": map[string]any{"id": "moon", "day_index": 100}}
	got := assessBodyCycle(spec, map[string]any{"calendar": map[string]any{"id": "moon", "day_index": 114}})
	if got["status"] != "estimated" || got["fertile_window_possible"] != true {
		t.Fatalf("explicit custom days not used: %#v", got)
	}
	spec.VariationDays = spec.CycleDays
	got = assessBodyCycle(spec, map[string]any{"calendar": map[string]any{"id": "moon", "day_index": 114}})
	if got["status"] != "unknown" || got["reason"] != "cycle_parameters_unresolved" {
		t.Fatalf("invalid author model became a fabricated estimate: %#v", got)
	}
}

func Test46BodyCycleDateJumpEqualsDirectCalculationWithoutEvents(t *testing.T) {
	spec := test46BodyCycleSpec()
	spec.VariationDays = 0
	before := mustCompactJSON(spec.ReferenceTime)
	var sequential map[string]any
	for day := 0; day <= 90; day++ {
		clock := storyTimeRelative(spec.ReferenceTime, map[string]any{"anchor": "source_observation", "offset": day, "unit": "day"})
		sequential = assessBodyCycle(spec, clock)
	}
	direct := assessBodyCycle(spec, map[string]any{"date": "2024-03-31"})
	if mustCompactJSON(sequential) != mustCompactJSON(direct) {
		t.Fatalf("jump depends on prior evaluations: %#v / %#v", sequential, direct)
	}
	if mustCompactJSON(spec.ReferenceTime) != before {
		t.Fatal("calculation mutated reference")
	}
}
