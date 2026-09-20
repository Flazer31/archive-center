package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func pregnancyReviewSetup46() (bodyTrackingConfig, bodyCharacterConfig, bodyCycleSpec) {
	cfg := bodyTrackingConfig{AutomaticPregnancyEnabled: true, SimulationSeed: "review-seed-46"}
	character := defaultBodyCharacterConfig()
	character.EntityID, character.OriginEntityID, character.CharacterName = "review-character", "review-origin", "Review Character"
	spec := bodyCycleSpec{ReferenceTime: cycleReviewDate46(0, 0), ReferenceKind: "author_setting", CycleDays: 28, PeriodDays: 5, LutealMinDays: 14, LutealMaxDays: 14}
	return cfg, character, spec
}

func pregnancyReviewEvent46(day int, key string) map[string]any {
	return map[string]any{"semantic_event_key": key, "occurred_at": cycleReviewDate46(day, day),
		"exposure": map[string]any{"classification": "potentially_conceiving", "partner_compatibility": "compatible", "contraception": "none"}}
}

func pregnancyReviewDecisions46(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	var decisions []map[string]any
	if err := json.Unmarshal([]byte(mustCompactJSON(out["cycles"])), &decisions); err != nil {
		t.Fatalf("cannot decode evaluated day decisions: %v / %#v", err, out)
	}
	return decisions
}

func pregnancyReviewCycle46(t *testing.T, cycles []bodyPregnancyCycle, index int) bodyPregnancyCycle {
	t.Helper()
	for _, cycle := range cycles {
		if cycle.Index == index {
			return cycle
		}
	}
	t.Fatalf("missing cycle %d in %#v", index, cycles)
	return bodyPregnancyCycle{}
}

func Test46PregnancyReviewTimelineSurvivesDeletedMiddleExposure(t *testing.T) {
	cfg, character, specA := pregnancyReviewSetup46()
	first, cyclesA := calculateBodyPregnancyEvent(cfg, character, specA, pregnancyReviewEvent46(90, "first"), nil)
	if first["status"] != "evaluated" {
		t.Fatalf("first evaluation: %#v", first)
	}
	a := pregnancyReviewCycle46(t, cyclesA, 3)
	if a.StartDay != 84 || a.EndDay != 112 {
		t.Fatalf("fixed 28-day oracle: %#v", a)
	}
	specB := specA
	specB.CycleDays = 30
	second, cyclesB := calculateBodyPregnancyEvent(cfg, character, specB, pregnancyReviewEvent46(150, "second"), cyclesA)
	b := pregnancyReviewCycle46(t, cyclesB, 5)
	if second["status"] != "evaluated" || b.StartDay != 142 || b.EndDay != 172 || len(b.Timeline) != 2 || b.Timeline[1].FirstCycle != 4 || b.Timeline[1].StartDay != 112 {
		t.Fatalf("mixed 28/30-day oracle: %#v / %#v", second, b)
	}
	// Only the later source survives. Its coordinate provenance must recover the
	// original older latent cycle without resurrecting the deleted exposure.
	survivors := []bodyPregnancyCycle{b}
	before := mustCompactJSON(survivors)
	backdated, reconstructed := calculateBodyPregnancyEvent(cfg, character, specB, pregnancyReviewEvent46(90, "later-backdated-observation"), survivors)
	if backdated["status"] != "evaluated" || mustCompactJSON(pregnancyReviewCycle46(t, reconstructed, 3)) != mustCompactJSON(a) {
		t.Fatalf("deleted earlier source changed latent cycle provenance: %#v / %#v", backdated, reconstructed)
	}
	_, intermediate := calculateBodyPregnancyEvent(cfg, character, specB, pregnancyReviewEvent46(120, "gap"), survivors)
	gap := pregnancyReviewCycle46(t, intermediate, 4)
	if gap.StartDay != 112 || gap.EndDay != 142 || gap.Parameters.CycleDays != 30 {
		t.Fatalf("intermediate parameter segment lost: %#v", gap)
	}
	specC := specA
	specC.CycleDays = 26
	_, cyclesC := calculateBodyPregnancyEvent(cfg, character, specC, pregnancyReviewEvent46(180, "third"), survivors)
	c := pregnancyReviewCycle46(t, cyclesC, 6)
	if c.StartDay != 172 || c.EndDay != 198 || len(c.Timeline) != 3 || c.Timeline[2].FirstCycle != 6 || c.Timeline[2].StartDay != 172 {
		t.Fatalf("third setting changed an already frozen cycle: %#v", c)
	}
	_, reconstructedGap := calculateBodyPregnancyEvent(cfg, character, specC, pregnancyReviewEvent46(120, "gap-after-second-deletion"), []bodyPregnancyCycle{c})
	if mustCompactJSON(pregnancyReviewCycle46(t, reconstructedGap, 4)) != mustCompactJSON(gap) || mustCompactJSON(survivors) != before {
		t.Fatal("later checkpoint did not retain complete immutable timeline prefix")
	}
}

func Test46PregnancyReviewDayIdentityFrozenCycleAndBranchOrigin(t *testing.T) {
	cfg, character, spec := pregnancyReviewSetup46()
	firstEvent := pregnancyReviewEvent46(13, "semantic-one")
	beforeInput := mustCompactJSON(firstEvent)
	first, frozen := calculateBodyPregnancyEvent(cfg, character, spec, firstEvent, nil)
	frozenBefore := mustCompactJSON(frozen)
	sameDay, sameFrozen := calculateBodyPregnancyEvent(cfg, character, spec, pregnancyReviewEvent46(13, "semantic-two"), frozen)
	if first["status"] != "evaluated" || mustCompactJSON(first["cycles"]) != mustCompactJSON(sameDay["cycles"]) || mustCompactJSON(sameFrozen) != frozenBefore {
		t.Fatal("different event keys on the same day changed the cycle or day draw")
	}
	secondDay, secondFrozen := calculateBodyPregnancyEvent(cfg, character, spec, pregnancyReviewEvent46(14, "next-day"), frozen)
	if mustCompactJSON(secondFrozen) != frozenBefore {
		t.Fatal("new exposure day redrew the shared latent cycle")
	}
	firstDecision, secondDecision := pregnancyReviewDecisions46(t, first)[0], pregnancyReviewDecisions46(t, secondDay)[0]
	if mustCompactJSON(firstDecision["day_draw"]) == mustCompactJSON(secondDecision["day_draw"]) {
		t.Fatal("a distinct exposure day reused another day's conditional draw")
	}
	reverseFirst, reverseFrozen := calculateBodyPregnancyEvent(cfg, character, spec, pregnancyReviewEvent46(14, "next-day"), nil)
	reverseSecond, _ := calculateBodyPregnancyEvent(cfg, character, spec, firstEvent, reverseFrozen)
	if mustCompactJSON(reverseFirst["cycles"]) != mustCompactJSON(secondDay["cycles"]) || mustCompactJSON(reverseSecond["cycles"]) != mustCompactJSON(first["cycles"]) {
		t.Fatal("processing order changed independent day outcomes or shared latent variables")
	}
	branchCharacter := character
	branchCharacter.EntityID, branchCharacter.CharacterName = "renamed-branch-character", "Renamed Character"
	branch, branchFrozen := calculateBodyPregnancyEvent(cfg, branchCharacter, spec, firstEvent, frozen)
	if mustCompactJSON(branch["cycles"]) != mustCompactJSON(first["cycles"]) || mustCompactJSON(branchFrozen) != frozenBefore {
		t.Fatal("branch-local entity identity reseeded the inherited origin")
	}
	if mustCompactJSON(firstEvent) != beforeInput || mustCompactJSON(frozen) != frozenBefore || strings.Contains(mustCompactJSON(first), cfg.SimulationSeed) {
		t.Fatal("model computation mutated source/history or exposed the raw seed")
	}
}

func Test46PregnancyReviewShortCyclesSelectOneChronologicalCandidate(t *testing.T) {
	cfg, character, spec := pregnancyReviewSetup46()
	spec.CycleDays, spec.PeriodDays, spec.LutealMinDays, spec.LutealMaxDays = 2, 1, 1, 1
	character.CycleViability, character.ConditionalPeak = 1, 1
	// Search a fixed bounded seed grid for the multi-success edge case; all
	// decisions come from the actual production engine, never a second model.
	found := false
	for seed := 0; seed < 128; seed++ {
		cfg.SimulationSeed = fmt.Sprintf("short-cycle-review-%d", seed)
		out, cycles := calculateBodyPregnancyEvent(cfg, character, spec, pregnancyReviewEvent46(4, "one-exposure"), nil)
		decisions := pregnancyReviewDecisions46(t, out)
		if len(cycles) != 3 || len(decisions) != 3 {
			t.Fatalf("one exposure should consider O=5,7,9: %#v", out)
		}
		var earliest map[string]any
		successes := 0
		for i, decision := range decisions {
			ovulation, _ := storyClockNumeric(decision["ovulation_day"])
			if ovulation != float64(5+2*i) {
				t.Fatalf("short cycle oracle: %#v", decisions)
			}
			if decision["latent_success"] == true {
				successes++
				if earliest == nil {
					earliest = decision
				}
			}
		}
		if successes < 2 {
			continue
		}
		found = true
		if out["outcome"] != "latent_model_implantation" || mustCompactJSON(parseJSONMap(mustCompactJSON(out["selected_pregnancy"]))) != mustCompactJSON(earliest) {
			t.Fatalf("multiple latent candidates did not select earliest ovulation: %#v", out)
		}
		reading := bodyPregnancyReading(mapFromAny(out["selected_pregnancy"]), cycleReviewDate46(4, 4))
		if reading["stage"] != "future_modeled_potential" || reading["knowledge"] != "not_inferred" || reading["symptoms"] != "not_inferred" {
			t.Fatalf("future candidate became an immediate known pregnancy: %#v", reading)
		}
		break
	}
	if !found {
		t.Fatal("fixed seed grid did not exercise multiple successful conception candidates")
	}
	// Ovulation order is intentional even when the implantation order differs.
	if !bodyPregnancyCandidateBefore(map[string]any{"ovulation_day": 5, "implantation_day": 17, "cycle_index": 2}, map[string]any{"ovulation_day": 7, "implantation_day": 13, "cycle_index": 3}) {
		t.Fatal("candidate ordering changed from earliest ovulation to earliest implantation")
	}
}

func Test46PregnancyReviewCalendarStagesAndReadPurity(t *testing.T) {
	for _, calendar := range []string{"gregorian", "world"} {
		t.Run(calendar, func(t *testing.T) {
			coordinate := func(day int) map[string]any {
				if calendar == "world" {
					return cycleReviewDate46(day, day)
				}
				return map[string]any{"date": fmt.Sprintf("2024-01-%02d", day+1)}
			}
			model := map[string]any{"model_version": bodyPregnancyModelVersion, "reference_family": "saved-family", "ovulation_time": coordinate(5), "implantation_time": coordinate(12)}
			before := mustCompactJSON(model)
			for _, tc := range []struct {
				day   int
				stage string
			}{{4, "future_modeled_potential"}, {5, "modeled_pre_implantation"}, {11, "modeled_pre_implantation"}, {12, "modeled_implanted_pregnancy"}, {20, "modeled_implanted_pregnancy"}} {
				reading := bodyPregnancyReading(model, coordinate(tc.day))
				if reading["stage"] != tc.stage || reading["knowledge"] != "not_inferred" || reading["symptoms"] != "not_inferred" {
					t.Errorf("day %d: %#v, want %s", tc.day, reading, tc.stage)
				}
			}
			uncertain := bodyPregnancyReading(model, map[string]any{"range": map[string]any{"start": coordinate(11), "end": coordinate(13)}})
			if uncertain["stage"] != "unknown" || mustCompactJSON(model) != before {
				t.Fatalf("overlapping time range or read mutated saved model: %#v", uncertain)
			}
		})
	}
	instant := map[string]any{"ovulation_time": map[string]any{"datetime": "2024-01-01T01:00:00Z"}, "implantation_time": map[string]any{"datetime": "2024-01-07T01:00:00Z"}}
	if got := bodyPregnancyReading(instant, map[string]any{"datetime": "2024-01-07T10:00:00+09:00"}); got["stage"] != "modeled_implanted_pregnancy" {
		t.Fatalf("same instant with different offset was not the implantation boundary: %#v", got)
	}
}

func Test46PregnancyReviewUnknownContextAndDatesRemainUnknown(t *testing.T) {
	cfg, character, spec := pregnancyReviewSetup46()
	for _, tc := range []struct {
		name, partner, contraception, profile string
		want                                  string
	}{
		{"missing_partner", "", "none", "", "evaluated"},
		{"unknown_partner", "unknown", "none", "", "unknown"},
		{"missing_contraception", "compatible", "", "", "evaluated"},
		{"classification_only", "", "", "", "evaluated"},
		{"unknown_contraception", "compatible", "unknown", "", "unknown"},
		{"explicit_author_world", "unknown", "unknown", "author_allowed", "evaluated"},
		{"contraception_even_author", "compatible", "present", "author_allowed", "unknown"},
		{"incompatible_even_author", "incompatible", "none", "author_allowed", "not_applicable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := pregnancyReviewEvent46(13, tc.name)
			exposure := mapFromAny(event["exposure"])
			exposure["partner_compatibility"], exposure["contraception"], exposure["model_profile"] = tc.partner, tc.contraception, tc.profile
			before := mustCompactJSON(event)
			out, cycles := calculateBodyPregnancyEvent(cfg, character, spec, event, nil)
			if out["status"] != tc.want || tc.want != "evaluated" && len(cycles) != 0 || mustCompactJSON(event) != before {
				t.Fatalf("context uncertainty changed source or produced a trial: %#v / %#v", out, cycles)
			}
		})
	}
	for _, date := range []map[string]any{{}, cycleReviewDate46(12, 14), {"calendar": map[string]any{"id": "different-calendar", "day_index": 13}}} {
		event := pregnancyReviewEvent46(13, "unknown-date")
		event["occurred_at"] = date
		out, cycles := calculateBodyPregnancyEvent(cfg, character, spec, event, nil)
		if out["status"] != "unknown" || len(cycles) != 0 || out["outcome"] != nil {
			t.Fatalf("unresolved date became zero risk or a modeled result: %#v", out)
		}
	}
}

func Test46PregnancyReviewLargeJumpsKeepOutputBounded(t *testing.T) {
	cfg, character, spec := pregnancyReviewSetup46()
	for _, tc := range []struct {
		name           string
		day, variation int
	}{{"fixed_billion_days", 1000000000, 0}, {"variable_hundred_thousand_days", 100000, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			spec.VariationDays = tc.variation
			before := mustCompactJSON(spec)
			started := time.Now()
			out, cycles := calculateBodyPregnancyEvent(cfg, character, spec, pregnancyReviewEvent46(tc.day, "large-jump"), nil)
			t.Logf("latent coordinate calculation took %s", time.Since(started))
			if out["status"] != "evaluated" || len(cycles) != 1 || len(pregnancyReviewDecisions46(t, out)) != 1 || len(mustCompactJSON(out)) > 6000 || len(mustCompactJSON(cycles)) > 6000 {
				t.Fatalf("date jump emitted historical trials or unbounded output: %#v / %d cycles", out, len(cycles))
			}
			current := cycles[0]
			if current.StartDay > float64(tc.day) || current.EndDay <= float64(tc.day) || mustCompactJSON(spec) != before {
				t.Fatalf("jump did not locate actual current latent cycle or mutated settings: %#v", current)
			}
			if tc.variation == 0 && (current.Index != tc.day/28 || current.StartDay != float64(tc.day/28*28)) {
				t.Fatalf("fixed-length independent arithmetic oracle failed: %#v", current)
			}
		})
	}
}
