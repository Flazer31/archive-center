package httpapi

import "math"

// bodyCycleSpec describes an author's model, not a diagnosis or an observed
// sequence of periods. Unobserved cycle-length variation accumulates uncertainty.
type bodyCycleSpec struct {
	ReferenceTime map[string]any `json:"reference_time"`
	ReferenceKind string         `json:"reference_kind"`
	CycleDays     int            `json:"cycle_days"`
	VariationDays int            `json:"variation_days"`
	PeriodDays    int            `json:"period_days"`
	LutealMinDays int            `json:"luteal_min_days"`
	LutealMaxDays int            `json:"luteal_max_days"`
}

func assessBodyCycle(spec bodyCycleSpec, clock map[string]any) map[string]any {
	out := map[string]any{
		"version": "body_cycle_estimate.v1", "kind": "calculated_estimate",
		"reference_time": storyClockJSONMap(spec.ReferenceTime), "reference_kind": spec.ReferenceKind,
		"last_confirmed_story_clock": storyTimeReadingClock(clock),
		"status":                     "unknown", "knowledge": "not_inferred",
	}
	minimumLength, maximumLength := spec.CycleDays-spec.VariationDays, spec.CycleDays+spec.VariationDays
	if spec.CycleDays <= 0 || spec.VariationDays < 0 || minimumLength <= 0 || maximumLength < minimumLength ||
		spec.PeriodDays <= 0 || spec.LutealMinDays <= 0 || spec.LutealMaxDays < spec.LutealMinDays || spec.LutealMaxDays >= minimumLength {
		out["reason"] = "cycle_parameters_unresolved"
		return out
	}
	referenceDays, currentDays := bodyCycleDayCoordinate(spec.ReferenceTime), bodyCycleDayCoordinate(clock)
	relation := storyTimeRelation(referenceDays, currentDays)
	elapsed := mapFromAny(relation["elapsed_days"])
	minimumDays, minimumOK := storyClockNumeric(elapsed["min"])
	maximumDays, maximumOK := storyClockNumeric(elapsed["max"])
	if !minimumOK || !maximumOK || minimumDays < 0 {
		out["reason"] = "story_date_or_reference_unresolved"
		return out
	}
	// The cycle is a date model. No fractional time implies a new calendar day.
	minimumDays, maximumDays = math.Floor(minimumDays), math.Floor(maximumDays)
	minimumCycle := math.Floor(minimumDays / float64(maximumLength))
	maximumCycle := math.Floor(maximumDays / float64(minimumLength))
	nominalCycle := math.Floor((minimumDays + maximumDays) / 2 / float64(spec.CycleDays))
	phaseMinimum, phaseMaximum := float64(maximumLength), float64(0)
	// The extrema are attained at the endpoint cycle indices; no iteration over
	// missed turns, unobserved scenes or historical cycle events is necessary.
	for _, cycle := range []float64{minimumCycle, maximumCycle} {
		startMinimum, startMaximum := cycle*float64(minimumLength), cycle*float64(maximumLength)
		lo := math.Max(0, minimumDays-startMaximum)
		hi := math.Min(float64(maximumLength-1), maximumDays-startMinimum)
		if lo <= hi {
			phaseMinimum = math.Min(phaseMinimum, lo)
			phaseMaximum = math.Max(phaseMaximum, hi)
		}
	}
	if phaseMinimum > phaseMaximum {
		out["reason"] = "cycle_position_unresolved"
		return out
	}
	shift := func(offset float64, maximum bool) map[string]any {
		base := currentDays
		if span := mapFromAny(base["range"]); len(span) > 0 {
			base = mapFromAny(span["start"])
			if maximum {
				base = mapFromAny(span["end"])
			}
		}
		return storyTimeRelative(base, map[string]any{"anchor": "source_observation", "offset": offset, "unit": "day"})
	}
	referenceBounds, currentBounds := storyTimeBounds(referenceDays), storyTimeBounds(currentDays)
	nextAbsoluteMin := math.Max(currentBounds.minimum+1, referenceBounds.minimum+(minimumCycle+1)*float64(minimumLength))
	nextAbsoluteMax := math.Min(currentBounds.maximum+float64(maximumLength), referenceBounds.maximum+(maximumCycle+1)*float64(maximumLength))
	nextMinimum := nextAbsoluteMin - currentBounds.minimum
	nextMaximum := nextAbsoluteMax - currentBounds.maximum
	ovulationMinimum := nextMinimum - float64(spec.LutealMaxDays)
	ovulationMaximum := nextMaximum - float64(spec.LutealMinDays)
	fertileMinimum, fertileMaximum := ovulationMinimum-5, ovulationMaximum
	fertilePhaseMin := float64(minimumLength - spec.LutealMaxDays - 5)
	fertilePhaseMax := float64(maximumLength - spec.LutealMinDays)
	// Keep the same possible cycle in both sides of the comparison. A wide
	// envelope joining two distinct cycles must not fill the non-fertile gap.
	// A short fictional cycle can have its fertile window begin before that
	// cycle's period. These are conception-cycle indices, not current cycles.
	fertileCycleMin := math.Max(0, math.Ceil((minimumDays-fertilePhaseMax)/float64(maximumLength)))
	fertileCycleMax := math.Floor((maximumDays - fertilePhaseMin) / float64(minimumLength))
	out["status"] = "estimated"
	out["cycle_index"] = nominalCycle
	out["possible_cycle_indices"] = map[string]any{"min": minimumCycle, "max": maximumCycle}
	out["cycle_day"] = map[string]any{"min": phaseMinimum + 1, "max": phaseMaximum + 1}
	out["cycle_length_days"] = map[string]any{"min": minimumLength, "max": maximumLength}
	out["menstruation_possible"] = phaseMinimum < float64(spec.PeriodDays)
	out["menstruation_estimate"] = phaseMaximum < float64(spec.PeriodDays)
	out["fertile_window_possible"] = fertileCycleMin <= fertileCycleMax
	if fertileCycleMin <= fertileCycleMax {
		out["possible_fertile_cycle_indices"] = map[string]any{"min": fertileCycleMin, "max": fertileCycleMax}
	}
	out["date_estimate_scope"] = "cycles_containing_current_story_date"
	out["ovulation_estimate"] = map[string]any{"range": map[string]any{"start": shift(ovulationMinimum, false), "end": shift(ovulationMaximum, true)}}
	out["fertile_window_estimate"] = map[string]any{"range": map[string]any{"start": shift(fertileMinimum, false), "end": shift(fertileMaximum, true)}}
	out["next_period_estimate"] = map[string]any{"range": map[string]any{"start": shift(nextMinimum, false), "end": shift(nextMaximum, true)}}
	out["date_bounds_kind"] = "conservative_envelope_may_include_gaps"
	out["uncertainty"] = "bounded_cycle_length_and_luteal_range; unobserved_variation_accumulates"
	return out
}

func bodyCycleDayCoordinate(raw map[string]any) map[string]any {
	raw = storyTimeCoordinate(raw)
	if raw["precision"] == "unknown" || raw["observation_kind"] == "unknown" {
		return raw
	}
	if bounds := mapFromAny(raw["range"]); len(bounds) > 0 {
		return map[string]any{"range": map[string]any{"start": bodyCycleDayCoordinate(mapFromAny(bounds["start"])), "end": bodyCycleDayCoordinate(mapFromAny(bounds["end"]))}}
	}
	if len(mapFromAny(raw["calendar"])) > 0 {
		return raw
	}
	absolute := mapFromAny(raw["absolute"])
	if len(absolute) == 0 {
		absolute = raw
	}
	if value, _, ok := parseStoryClockAbsolute(absolute); ok {
		return map[string]any{"date": value.Format("2006-01-02")}
	}
	return raw
}
