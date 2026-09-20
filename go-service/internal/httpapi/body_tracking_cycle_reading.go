package httpapi

// This is a limitation of the ordinary cyclic model, not a new body fact or a
// rule requiring an observed period. The caller supplies only facts visible to
// its existing perspective; hidden facts must not alter a visible forecast.
func bodyTrackingCycleReading(character bodyCharacterConfig, projection, clock map[string]any) map[string]any {
	projection = bodyTrackingTermProjection(character, projection)
	spec := character.Cycle
	if reference := mapFromAny(projection["cycle_reference"]); len(reference) > 0 {
		spec.ReferenceTime, spec.ReferenceKind = storyClockJSONMap(reference), "observed_period_start"
	}
	unknown := func(reason string) map[string]any {
		return map[string]any{
			"version": "body_cycle_estimate.v1", "kind": "calculated_estimate", "status": "unknown", "reason": reason,
			"reference_time": storyClockJSONMap(spec.ReferenceTime), "reference_kind": spec.ReferenceKind,
			"last_confirmed_story_clock": storyTimeReadingClock(clock), "knowledge": "not_inferred",
		}
	}
	atOrAfter := func(reference, event map[string]any) bool {
		switch stringFromMap(storyTimeRelation(event, reference), "relation") {
		case "past", "same_day", "same_instant":
			return true
		}
		return false
	}
	pregnancy := mapFromAny(projection["pregnancy"])
	status := stringFromMap(pregnancy, "status")
	ending := mapFromAny(pregnancy["occurred_at"])
	if status == "confirmed" && bodyPregnancyTermReading(pregnancy, clock)["stage"] == "modeled_birth_completed" {
		status, ending = "ended", mapFromAny(pregnancy["modeled_birth_time"])
	}
	// Missing optional occurrence time does not erase an authoritative current
	// confirmation. Only an explicit future observation is not current yet.
	if status == "confirmed" && stringFromMap(storyTimeRelation(mapFromAny(pregnancy["occurred_at"]), clock), "relation") != "future" {
		return unknown("ordinary_cycle_model_not_applicable_during_observed_pregnancy")
	}
	// The saved current model can belong to a later pregnancy than the old
	// observed ending. Old evaluation history is not a current model candidate.
	model := bodyPregnancyReading(mapFromAny(projection["modeled_pregnancy"]), clock)
	switch stringFromMap(model, "stage") {
	case "modeled_pre_implantation", "modeled_implanted_pregnancy":
		return unknown("ordinary_cycle_model_not_applicable_during_modeled_pregnancy")
	case "modeled_birth_completed":
		if !atOrAfter(spec.ReferenceTime, mapFromAny(model["modeled_birth_time"])) {
			return unknown("cycle_reference_after_pregnancy_unresolved")
		}
	}
	if status == "ended" {
		if !atOrAfter(spec.ReferenceTime, ending) {
			// A later explicit author reference remains valid after an ending.
			if character.Cycle.ReferenceKind == "author_setting" && atOrAfter(character.Cycle.ReferenceTime, ending) {
				spec = character.Cycle
			} else {
				return unknown("cycle_reference_after_pregnancy_unresolved")
			}
		}
	}
	return assessBodyCycle(spec, clock)
}
