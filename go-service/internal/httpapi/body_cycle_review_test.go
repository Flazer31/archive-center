package httpapi

import "testing"

type cycleReviewOracle46 struct {
	minCycle, maxCycle, minPhase, maxPhase, minNext, maxNext, minOvulation, maxOvulation int
	minFertileCycle, maxFertileCycle                                                     int
	periodPossible, periodAll, fertilePossible                                           bool
	observed                                                                             bool
}

// Enumerate actual complete cycle lengths rather than mirroring the production
// bounds formula. Equal reachable (start,index) states can be merged because
// this oracle asks about possible values, never probability weights.
func enumerateCycleReview46(spec bodyCycleSpec, referenceMin, referenceMax, currentMin, currentMax int) cycleReviewOracle46 {
	out := cycleReviewOracle46{periodAll: true}
	for reference := referenceMin; reference <= referenceMax; reference++ {
		for current := currentMin; current <= currentMax; current++ {
			type state struct{ start, index int }
			queue := []state{{reference, 0}}
			seen := map[state]bool{}
			for len(queue) > 0 {
				node := queue[0]
				queue = queue[1:]
				if seen[node] {
					continue
				}
				seen[node] = true
				for length := spec.CycleDays - spec.VariationDays; length <= spec.CycleDays+spec.VariationDays; length++ {
					next := node.start + length
					if next <= current+5 {
						queue = append(queue, state{next, node.index + 1})
					}
					phase := current - node.start
					for luteal := spec.LutealMinDays; luteal <= spec.LutealMaxDays; luteal++ {
						ovulation := next - luteal
						if current >= ovulation-5 && current <= ovulation {
							if !out.fertilePossible {
								out.minFertileCycle, out.maxFertileCycle = node.index, node.index
							}
							out.minFertileCycle, out.maxFertileCycle = min(out.minFertileCycle, node.index), max(out.maxFertileCycle, node.index)
							out.fertilePossible = true
						}
						if current >= next || current < node.start {
							continue
						}
						if !out.observed {
							out.minCycle, out.maxCycle, out.minPhase, out.maxPhase = node.index, node.index, phase, phase
							out.minNext, out.maxNext, out.minOvulation, out.maxOvulation = next, next, ovulation, ovulation
							out.observed = true
						}
						out.minCycle, out.maxCycle = min(out.minCycle, node.index), max(out.maxCycle, node.index)
						out.minPhase, out.maxPhase = min(out.minPhase, phase), max(out.maxPhase, phase)
						out.minNext, out.maxNext = min(out.minNext, next), max(out.maxNext, next)
						out.minOvulation, out.maxOvulation = min(out.minOvulation, ovulation), max(out.maxOvulation, ovulation)
						out.periodPossible = out.periodPossible || phase < spec.PeriodDays
						out.periodAll = out.periodAll && phase < spec.PeriodDays
					}
				}
			}
		}
	}
	return out
}

func cycleReviewDate46(minimum, maximum int) map[string]any {
	day := func(value int) map[string]any {
		return map[string]any{"calendar": map[string]any{"id": "review-calendar", "day_index": value}}
	}
	if minimum == maximum {
		return day(minimum)
	}
	return map[string]any{"range": map[string]any{"start": day(minimum), "end": day(maximum)}}
}

func Test46BodyCycleMatchesEnumeratedPossibleCycles(t *testing.T) {
	for _, tc := range []struct {
		name                                                                                        string
		length, variation, referenceMin, referenceMax, currentMin, currentMax, lutealMin, lutealMax int
	}{
		{"fixed-clock-boundary", 28, 0, 0, 0, 27, 29, 14, 14},
		{"uncertain-reference-boundary", 28, 0, 0, 2, 29, 29, 14, 14},
		{"variable-exact-current", 28, 2, 0, 0, 60, 60, 10, 16},
		{"variable-both-ranges", 28, 2, 0, 2, 59, 61, 10, 16},
		{"short-world-calendar", 14, 2, 0, 1, 26, 29, 5, 7},
		{"fertile-window-crosses-cycle", 5, 0, 0, 0, 4, 4, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := bodyCycleSpec{ReferenceTime: cycleReviewDate46(tc.referenceMin, tc.referenceMax), ReferenceKind: "author_setting", CycleDays: tc.length, VariationDays: tc.variation, PeriodDays: 3, LutealMinDays: tc.lutealMin, LutealMaxDays: tc.lutealMax}
			want := enumerateCycleReview46(spec, tc.referenceMin, tc.referenceMax, tc.currentMin, tc.currentMax)
			got := assessBodyCycle(spec, cycleReviewDate46(tc.currentMin, tc.currentMax))
			if !want.observed || got["status"] != "estimated" {
				t.Fatalf("missing estimate: want=%+v got=%#v", want, got)
			}
			for _, bounds := range []struct {
				key              string
				minimum, maximum int
			}{{"possible_cycle_indices", want.minCycle, want.maxCycle}, {"cycle_day", want.minPhase + 1, want.maxPhase + 1}} {
				span := mapFromAny(got[bounds.key])
				lo, _ := storyClockNumeric(span["min"])
				hi, _ := storyClockNumeric(span["max"])
				if lo != float64(bounds.minimum) || hi != float64(bounds.maximum) {
					t.Errorf("%s=%#v want [%d,%d]", bounds.key, span, bounds.minimum, bounds.maximum)
				}
			}
			for _, bounds := range []struct {
				key              string
				minimum, maximum int
			}{{"next_period_estimate", want.minNext, want.maxNext}, {"ovulation_estimate", want.minOvulation, want.maxOvulation}, {"fertile_window_estimate", want.minOvulation - 5, want.maxOvulation}} {
				span := storyTimeBounds(mapFromAny(got[bounds.key]))
				if !span.valid || span.minimum != float64(bounds.minimum) || span.maximum != float64(bounds.maximum) {
					t.Errorf("%s=%#v want [%d,%d]", bounds.key, got[bounds.key], bounds.minimum, bounds.maximum)
				}
			}
			if got["menstruation_possible"] != want.periodPossible || got["menstruation_estimate"] != want.periodAll || got["fertile_window_possible"] != want.fertilePossible {
				t.Errorf("phase classification differs from enumerated possibilities: got=%#v want=%+v", got, want)
			}
			if want.fertilePossible {
				indices := mapFromAny(got["possible_fertile_cycle_indices"])
				minimum, minOK := storyClockNumeric(indices["min"])
				maximum, maxOK := storyClockNumeric(indices["max"])
				if !minOK || !maxOK || minimum != float64(want.minFertileCycle) || maximum != float64(want.maxFertileCycle) {
					t.Errorf("conception-cycle indices differ from enumerated cycles: got=%#v want=[%d,%d]", indices, want.minFertileCycle, want.maxFertileCycle)
				}
			}
		})
	}
}

func Test46BodyCycleUsesCalendarDayAcrossMidnight(t *testing.T) {
	spec := test46BodyCycleSpec()
	spec.ReferenceTime = map[string]any{"datetime": "2024-01-01T23:00:00+09:00"}
	got := assessBodyCycle(spec, map[string]any{"datetime": "2024-01-02T01:00:00+09:00"})
	span := mapFromAny(got["cycle_day"])
	if span["min"] != float64(2) || span["max"] != float64(2) {
		t.Fatalf("calendar date changed but cycle day did not: %#v", got)
	}
}

func Test46BodyCycleReviewKeepsUnknownAndDoesNotMutate(t *testing.T) {
	spec := test46BodyCycleSpec()
	clock := map[string]any{"range": map[string]any{"start": map[string]any{"date": "2024-01-28"}, "end": map[string]any{"date": "2024-01-30"}}}
	beforeSpec, beforeClock := mustCompactJSON(spec), mustCompactJSON(clock)
	_ = assessBodyCycle(spec, clock)
	if mustCompactJSON(spec) != beforeSpec || mustCompactJSON(clock) != beforeClock {
		t.Fatal("cycle reading mutated source settings or clock")
	}
	for _, changed := range []bodyCycleSpec{
		{ReferenceTime: spec.ReferenceTime, CycleDays: 0, PeriodDays: 5, LutealMinDays: 10, LutealMaxDays: 16},
		{ReferenceTime: spec.ReferenceTime, CycleDays: 28, VariationDays: -1, PeriodDays: 5, LutealMinDays: 10, LutealMaxDays: 16},
		{ReferenceTime: spec.ReferenceTime, CycleDays: 28, PeriodDays: 5, LutealMinDays: 16, LutealMaxDays: 10},
		{ReferenceTime: map[string]any{"precision": "unknown"}, CycleDays: 28, PeriodDays: 5, LutealMinDays: 10, LutealMaxDays: 16},
	} {
		got := assessBodyCycle(changed, clock)
		if got["status"] != "unknown" || got["pregnant"] != nil || got["last_observed_period"] != nil {
			t.Fatalf("unresolved input invented body facts: %#v", got)
		}
	}
}
