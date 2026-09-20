package httpapi

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
)

const bodyPregnancyModelVersion = "body_conception_model.v1"

// This is an explicitly selected fiction model, not an individual medical risk
// calculator. Its full parameters travel with each immutable cycle checkpoint.
type bodyPregnancyParameters struct {
	Version         string  `json:"version"`
	CycleDays       int     `json:"cycle_days"`
	VariationDays   int     `json:"variation_days"`
	LutealMinDays   int     `json:"luteal_min_days"`
	LutealMaxDays   int     `json:"luteal_max_days"`
	Viability       float64 `json:"viability"`
	Peak            float64 `json:"peak"`
	KernelCenter    float64 `json:"kernel_center"`
	KernelWidth     float64 `json:"kernel_width"`
	SupportStart    int     `json:"support_start"`
	SupportEnd      int     `json:"support_end"`
	ImplantationMin int     `json:"implantation_min"`
	ImplantationMax int     `json:"implantation_max"`
}

type bodyPregnancySegment struct {
	FirstCycle int                     `json:"first_cycle"`
	StartDay   float64                 `json:"start_day"`
	Parameters bodyPregnancyParameters `json:"parameters"`
}

type bodyPregnancyDraw struct {
	Token    string  `json:"token"`
	Fraction float64 `json:"fraction"`
}

// Per-exposure-day outcomes are intentionally outside this immutable base. A
// new day reuses O and V but receives its own stable conditional day draw.
type bodyPregnancyCycle struct {
	Version          string                  `json:"model_version"`
	Family           string                  `json:"reference_family"`
	Calendar         string                  `json:"calendar"`
	Index            int                     `json:"cycle_index"`
	StartDay         float64                 `json:"start_day"`
	EndDay           float64                 `json:"end_day"`
	OvulationDay     float64                 `json:"ovulation_day"`
	ImplantationDay  float64                 `json:"implantation_day"`
	Parameters       bodyPregnancyParameters `json:"parameters"`
	LengthDraw       bodyPregnancyDraw       `json:"length_draw"`
	LutealDraw       bodyPregnancyDraw       `json:"luteal_draw"`
	ViabilityDraw    bodyPregnancyDraw       `json:"viability_draw"`
	ImplantationDraw bodyPregnancyDraw       `json:"implantation_draw"`
	Timeline         []bodyPregnancySegment  `json:"timeline"`
}

func bodyPregnancyParametersFor(character bodyCharacterConfig, spec bodyCycleSpec) bodyPregnancyParameters {
	return bodyPregnancyParameters{Version: bodyPregnancyModelVersion, CycleDays: spec.CycleDays, VariationDays: spec.VariationDays,
		LutealMinDays: spec.LutealMinDays, LutealMaxDays: spec.LutealMaxDays, Viability: character.CycleViability, Peak: character.ConditionalPeak,
		KernelCenter: -1, KernelWidth: 2, SupportStart: -5, SupportEnd: 0, ImplantationMin: 6, ImplantationMax: 12}
}

func (p bodyPregnancyParameters) valid() bool {
	minimum, maximum := p.CycleDays-p.VariationDays, p.CycleDays+p.VariationDays
	return p.Version == bodyPregnancyModelVersion && p.CycleDays > 0 && p.VariationDays >= 0 && minimum > 0 && maximum >= minimum &&
		p.LutealMinDays > 0 && p.LutealMaxDays >= p.LutealMinDays && p.LutealMaxDays < minimum &&
		p.Viability >= 0 && p.Viability <= 1 && p.Peak >= 0 && p.Peak <= 1 && p.KernelWidth > 0 &&
		!math.IsNaN(p.KernelCenter) && !math.IsInf(p.KernelCenter, 0) && !math.IsInf(p.KernelWidth, 0) &&
		p.SupportStart <= p.SupportEnd && p.ImplantationMin > 0 && p.ImplantationMax >= p.ImplantationMin
}

func bodyPregnancyConditionalProbability(parameters bodyPregnancyParameters, offset float64) float64 {
	if offset < float64(parameters.SupportStart) || offset > float64(parameters.SupportEnd) {
		return 0
	}
	x := (offset - parameters.KernelCenter) / parameters.KernelWidth
	return parameters.Peak * math.Exp(-.5*x*x)
}

// JSON array framing prevents ambiguous key concatenation. Taking the upper 53
// bits gives an exactly representable fraction in [0,1), never a rounded 1.
func bodyPregnancyRandom(seed, origin, family string, cycle int, purpose string, attempt int) (bodyPregnancyDraw, uint64) {
	key, _ := json.Marshal([]any{bodyPregnancyModelVersion, seed, origin, family, cycle, purpose, attempt})
	sum := sha256.Sum256(key)
	value := binary.BigEndian.Uint64(sum[:8])
	return bodyPregnancyDraw{Token: hex.EncodeToString(sum[:]), Fraction: float64(value>>11) / (1 << 53)}, value
}

func bodyPregnancyInteger(seed, origin, family string, cycle int, purpose string, minimum, maximum int) (int, bodyPregnancyDraw) {
	width := uint64(maximum-minimum) + 1
	threshold := -width % width
	for attempt := 0; ; attempt++ {
		draw, value := bodyPregnancyRandom(seed, origin, family, cycle, purpose, attempt)
		if value >= threshold { // Rejection sampling removes integer modulo bias.
			return minimum + int(value%width), draw
		}
	}
}

func calculateBodyPregnancyEvent(cfg bodyTrackingConfig, character bodyCharacterConfig, spec bodyCycleSpec, observation map[string]any, frozen []bodyPregnancyCycle) (map[string]any, []bodyPregnancyCycle) {
	out := map[string]any{"model_version": bodyPregnancyModelVersion, "authority": "fiction_simulation", "status": "unknown",
		"unit": "distinct_subject_exposure_day", "knowledge": "not_inferred", "symptoms": "not_inferred",
		"occurrence_time": storyClockJSONMap(mapFromAny(observation["occurred_at"])), "semantic_event_key": observation["semantic_event_key"]}
	if !cfg.AutomaticPregnancyEnabled {
		out["status"] = "disabled"
		return out, nil
	}
	exposure := mapFromAny(observation["exposure"])
	classification, profile := stringFromMap(exposure, "classification"), stringFromMap(exposure, "model_profile")
	if !character.CanConceive || classification == "not_potentially_conceiving" || stringFromMap(exposure, "partner_compatibility") == "incompatible" {
		out["status"], out["reason"] = "not_applicable", "explicit_exposure_or_world_setting"
		return out, nil
	}
	if stringFromMap(exposure, "contraception") == "present" {
		out["reason"] = "contraception_context_not_modeled"
		return out, nil
	}
	// The declared exposure classification is sufficient input. Optional
	// details do not become redundant required observations; explicitly stated
	// uncertainty remains uncertainty rather than an invented compatibility fact.
	if profile != "author_allowed" && (classification != "potentially_conceiving" || stringFromMap(exposure, "partner_compatibility") == "unknown" || stringFromMap(exposure, "contraception") == "unknown") {
		out["reason"] = "exposure_context_not_modeled"
		return out, nil
	}
	out["exposure_basis"] = "declared_potential_exposure_under_configured_fiction_model"
	reference := bodyCycleDayCoordinate(spec.ReferenceTime)
	occurred := bodyCycleDayCoordinate(mapFromAny(observation["occurred_at"]))
	r, day := storyTimeBounds(reference), storyTimeBounds(occurred)
	if !r.valid || !day.valid || r.calendar != day.calendar || r.minimum != r.maximum || day.minimum != day.maximum || day.minimum < r.minimum {
		out["reason"] = "exact_event_date_or_reference_unresolved"
		return out, nil
	}
	if cfg.SimulationSeed == "" {
		out["reason"] = "simulation_seed_unavailable"
		return out, nil
	}
	origin := extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
	familyInput, _ := json.Marshal([]any{origin, r.calendar, r.minimum})
	familyDigest := sha256.Sum256(familyInput)
	family := hex.EncodeToString(familyDigest[:])
	parameters := bodyPregnancyParametersFor(character, spec)
	byIndex := map[int]bodyPregnancyCycle{}
	last := -1
	for _, cycle := range frozen {
		if cycle.Family == family {
			byIndex[cycle.Index] = cycle
			if cycle.Index > last {
				last = cycle.Index
			}
		}
	}
	timeline := []bodyPregnancySegment{{FirstCycle: 0, StartDay: r.minimum, Parameters: parameters}}
	if last >= 0 {
		checkpoint := byIndex[last]
		timeline = append([]bodyPregnancySegment(nil), checkpoint.Timeline...)
		if len(timeline) == 0 {
			out["reason"] = "stored_model_timeline_unresolved"
			return out, nil
		}
		if timeline[len(timeline)-1].Parameters != parameters {
			timeline = append(timeline, bodyPregnancySegment{FirstCycle: last + 1, StartDay: checkpoint.EndDay, Parameters: parameters})
		}
	}
	out["reference_family"], out["exposure_day"], out["calendar"] = family, day.minimum, day.calendar
	cycles := []bodyPregnancyCycle{}
	decisions := []map[string]any{}
	selected := map[string]any(nil)
	index, start, segmentIndex := 0, r.minimum, 0
	for start <= day.minimum+5 {
		for segmentIndex+1 < len(timeline) && timeline[segmentIndex+1].FirstCycle <= index {
			segmentIndex++
			start = timeline[segmentIndex].StartDay
		}
		p := timeline[segmentIndex].Parameters
		if !p.valid() {
			out["reason"] = "model_parameters_unresolved"
			return out, nil
		}
		// A fixed-length segment can skip empty historical cycles exactly. These
		// skipped coordinates are calculations, never invented body events.
		if p.VariationDays == 0 && start+float64(p.CycleDays) <= day.minimum {
			skip := int(math.Floor((day.minimum - start) / float64(p.CycleDays)))
			if segmentIndex+1 < len(timeline) {
				skip = min(skip, timeline[segmentIndex+1].FirstCycle-index)
			}
			if skip > 0 {
				start += float64(skip) * float64(p.CycleDays)
				index += skip
				continue
			}
		}
		cycle, exists := byIndex[index]
		if !exists {
			length, lengthDraw := bodyPregnancyInteger(cfg.SimulationSeed, origin, family, index, "cycle_length", p.CycleDays-p.VariationDays, p.CycleDays+p.VariationDays)
			luteal, lutealDraw := bodyPregnancyInteger(cfg.SimulationSeed, origin, family, index, "luteal_length", p.LutealMinDays, p.LutealMaxDays)
			implantation, implantationDraw := bodyPregnancyInteger(cfg.SimulationSeed, origin, family, index, "implantation_lag", p.ImplantationMin, p.ImplantationMax)
			viabilityDraw, _ := bodyPregnancyRandom(cfg.SimulationSeed, origin, family, index, "cycle_viability", 0)
			cycle = bodyPregnancyCycle{Version: p.Version, Family: family, Calendar: r.calendar, Index: index, StartDay: start,
				EndDay: start + float64(length), OvulationDay: start + float64(length-luteal), ImplantationDay: start + float64(length-luteal+implantation),
				Parameters: p, LengthDraw: lengthDraw, LutealDraw: lutealDraw, ViabilityDraw: viabilityDraw, ImplantationDraw: implantationDraw,
				Timeline: append([]bodyPregnancySegment(nil), timeline[:segmentIndex+1]...)}
		}
		q := bodyPregnancyConditionalProbability(cycle.Parameters, day.minimum-cycle.OvulationDay)
		if cycle.StartDay <= day.minimum && day.minimum < cycle.EndDay || q > 0 {
			cycles = append(cycles, cycle)
			purpose := "exposure_day:" + mustCompactJSON([]any{day.calendar, day.minimum})
			draw, _ := bodyPregnancyRandom(cfg.SimulationSeed, origin, family, index, purpose, 0)
			success := cycle.ViabilityDraw.Fraction < cycle.Parameters.Viability && draw.Fraction < q
			coordinate := func(target float64) map[string]any {
				return storyTimeRelative(reference, map[string]any{"anchor": "source_observation", "unit": "day", "offset": target - r.minimum})
			}
			decision := map[string]any{"model_version": cycle.Version, "reference_family": family, "cycle_index": index,
				"ovulation_day": cycle.OvulationDay, "implantation_day": cycle.ImplantationDay,
				"ovulation_time": coordinate(cycle.OvulationDay), "implantation_time": coordinate(cycle.ImplantationDay),
				"conditional_day_probability": q, "single_day_probability_given_latent_ovulation": cycle.Parameters.Viability * q,
				"day_draw": draw, "latent_success": success, "knowledge": "not_inferred", "authority": "fiction_simulation"}
			decisions = append(decisions, decision)
			if success && (selected == nil || bodyPregnancyCandidateBefore(decision, selected)) {
				selected = decision
			}
		}
		if cycle.EndDay <= start {
			out["reason"] = "model_calendar_precision_unresolved"
			return out, nil
		}
		start, index = cycle.EndDay, index+1
	}
	out["status"], out["cycles"] = "evaluated", decisions
	out["outcome"] = "no_modeled_implantation_from_this_day"
	if selected != nil {
		out["outcome"], out["selected_pregnancy"] = "latent_model_implantation", selected
	}
	return out, cycles
}

func bodyPregnancyCandidateBefore(a, b map[string]any) bool {
	for _, key := range []string{"ovulation_day", "implantation_day", "cycle_index"} {
		left, _ := storyClockNumeric(a[key])
		right, _ := storyClockNumeric(b[key])
		if left != right {
			return left < right
		}
	}
	return false
}

// Reading a saved model only relates its effective coordinates to admitted time.
// Neither retrieval nor a date jump samples, creates symptoms or grants knowledge.
func bodyPregnancyReading(model, clock map[string]any) map[string]any {
	out := map[string]any{"authority": "fiction_simulation", "knowledge": "not_inferred", "symptoms": "not_inferred", "stage": "unknown"}
	out["last_confirmed_story_clock"] = storyTimeReadingClock(clock)
	if len(model) == 0 {
		return out
	}
	out["model_version"], out["reference_family"] = model["model_version"], model["reference_family"]
	out["ovulation_time"], out["implantation_time"] = model["ovulation_time"], model["implantation_time"]
	ovulation := storyTimeRelation(mapFromAny(model["ovulation_time"]), clock)
	implantation := storyTimeRelation(mapFromAny(model["implantation_time"]), clock)
	out["ovulation_relation"], out["implantation_relation"] = ovulation, implantation
	term := bodyPregnancyTermReading(model, clock)
	for _, key := range []string{"modeled_birth_time", "gestation_days", "term_basis", "birth_relation", "birth_observed", "child_outcome", "paternity"} {
		out[key] = term[key]
	}
	if term["stage"] == "modeled_birth_completed" {
		out["stage"], out["interpretation"] = term["stage"], term["interpretation"]
		return out
	}
	switch implantation["relation"] {
	case "past", "same_instant", "same_day":
		out["stage"] = "modeled_implanted_pregnancy"
	default:
		switch ovulation["relation"] {
		case "future":
			out["stage"] = "future_modeled_potential"
		case "past", "same_instant", "same_day":
			if implantation["relation"] == "future" {
				out["stage"] = "modeled_pre_implantation"
			}
		}
	}
	return out
}

func bodyPregnancyFrozenCycles(events []map[string]any) []bodyPregnancyCycle {
	// This helper decodes provenance only. The caller supplies source-active events.
	out := []bodyPregnancyCycle{}
	for _, event := range events {
		var cycles []bodyPregnancyCycle
		if json.Unmarshal([]byte(mustCompactJSON(event["model_cycles"])), &cycles) == nil {
			out = append(out, cycles...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}
