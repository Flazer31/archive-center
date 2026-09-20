package httpapi

import (
	"fmt"
	"math"
	"testing"
)

func test46ConceptionObservation(day int) map[string]any {
	return map[string]any{"kind": "conception_exposure", "semantic_event_key": fmt.Sprintf("exposure-%d", day),
		"occurred_at": map[string]any{"calendar": map[string]any{"id": "model-test", "day_index": day}},
		"exposure":    map[string]any{"classification": "potentially_conceiving", "partner_compatibility": "compatible", "contraception": "none", "model_profile": "unprotected"}}
}

func test46ConceptionInputs() (bodyTrackingConfig, bodyCharacterConfig, bodyCycleSpec) {
	character := defaultBodyCharacterConfig()
	character.EntityID, character.OriginEntityID = "character-current", "character-origin"
	spec := character.Cycle
	spec.ReferenceTime = map[string]any{"calendar": map[string]any{"id": "model-test", "day_index": 0}}
	spec.CycleDays, spec.VariationDays, spec.LutealMinDays, spec.LutealMaxDays = 28, 0, 14, 14
	character.Cycle = spec
	return bodyTrackingConfig{AutomaticPregnancyEnabled: true, SimulationSeed: "test-only-model-seed"}, character, spec
}

func Test46BodyPregnancyKernelHasDeclaredUnitAndSharedCeiling(t *testing.T) {
	_, character, spec := test46ConceptionInputs()
	parameters := bodyPregnancyParametersFor(character, spec)
	product := 1.0
	for day := -5; day <= 0; day++ {
		q := bodyPregnancyConditionalProbability(parameters, float64(day))
		want := .7 * math.Exp(-math.Pow(float64(day+1), 2)/8)
		if math.Abs(q-want) > 1e-15 {
			t.Fatalf("day %d kernel=%g want %g", day, q, want)
		}
		product *= 1 - q
	}
	combined := parameters.Viability * (1 - product)
	if math.Abs(combined-.3929419673147731) > 1e-14 || combined >= parameters.Viability {
		t.Fatalf("joint outcome lost shared viability: %g", combined)
	}
	if bodyPregnancyConditionalProbability(parameters, -6) != 0 || bodyPregnancyConditionalProbability(parameters, 1) != 0 {
		t.Fatal("probability outside declared six-day support")
	}
	if got := parameters.Viability * bodyPregnancyConditionalProbability(parameters, -1); math.Abs(got-.28) > 1e-15 {
		t.Fatalf("single-day peak=%g", got)
	}
}

// Fixed seeds make this ensemble reproducible. The oracle is the closed-form
// conditional model, not the output of a second random implementation.
func Test46BodyPregnancySeedEnsembleMatchesSingleAndSharedCycleProbability(t *testing.T) {
	cfg, character, spec := test46ConceptionInputs()
	const count = 3000
	single, combined, completeFailures := 0, 0, 0
	for seed := 0; seed < count; seed++ {
		cfg.SimulationSeed = fmt.Sprintf("ensemble-%d", seed)
		var frozen []bodyPregnancyCycle
		anySuccess, viable := false, false
		for day := 9; day <= 14; day++ {
			result, cycles := calculateBodyPregnancyEvent(cfg, character, spec, test46ConceptionObservation(day), frozen)
			if result["status"] != "evaluated" || len(cycles) != 1 {
				t.Fatalf("incomplete production evaluation: %#v cycles=%d", result, len(cycles))
			}
			frozen = append(frozen, cycles...)
			viable = cycles[0].ViabilityDraw.Fraction < character.CycleViability
			success := result["outcome"] == "latent_model_implantation"
			if success && !viable {
				t.Fatal("nonviable cycle became successful through repetition")
			}
			if day == 13 && success {
				single++
			}
			anySuccess = anySuccess || success
		}
		if anySuccess {
			combined++
		} else {
			completeFailures++
		}
	}
	for name, tc := range map[string]struct {
		count       int
		probability float64
	}{
		"peak day": {single, .28}, "all six days": {combined, .3929419673147731},
	} {
		rate := float64(tc.count) / count
		// Six standard errors give a broad deterministic regression threshold;
		// this checks wrong cumulative redraws, not a fitted empirical risk.
		bound := 6 * math.Sqrt(tc.probability*(1-tc.probability)/count)
		if math.Abs(rate-tc.probability) > bound {
			t.Fatalf("%s simulated rate=%g oracle=%g allowed=%g", name, rate, tc.probability, bound)
		}
		t.Logf("%s: %d/%d=%g; declared model=%g", name, tc.count, count, rate, tc.probability)
	}
	if completeFailures == 0 {
		t.Fatal("six distinct exposure days became a guarantee")
	}
}

func Test46BodyPregnancyLiteralProbabilityEndpointsAndUnknownDoNotMutate(t *testing.T) {
	cfg, character, spec := test46ConceptionInputs()
	observation := test46ConceptionObservation(13)
	before := mustCompactJSON(observation)
	for _, tc := range []struct {
		viability, peak float64
		outcome         string
	}{
		{0, .7, "no_modeled_implantation_from_this_day"}, {.4, 0, "no_modeled_implantation_from_this_day"}, {1, 1, "latent_model_implantation"},
	} {
		character.CycleViability, character.ConditionalPeak = tc.viability, tc.peak
		got, _ := calculateBodyPregnancyEvent(cfg, character, spec, observation, nil)
		if got["outcome"] != tc.outcome || got["knowledge"] != "not_inferred" || got["symptoms"] != "not_inferred" {
			t.Fatalf("authored endpoints changed: %#v", got)
		}
	}
	character.CycleViability = math.NaN()
	got, cycles := calculateBodyPregnancyEvent(cfg, character, spec, observation, nil)
	if got["status"] != "unknown" || len(cycles) != 0 || mustCompactJSON(observation) != before {
		t.Fatalf("unresolved model fabricated outcome or altered input: %#v", got)
	}
	cfg.AutomaticPregnancyEnabled = false
	got, cycles = calculateBodyPregnancyEvent(cfg, character, spec, observation, nil)
	if got["status"] != "disabled" || len(cycles) != 0 {
		t.Fatalf("OFF sampled: %#v", got)
	}
}
