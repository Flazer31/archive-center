package httpapi

// Term dates are fiction-model coordinates. They never create a birth event,
// a child, symptoms or knowledge. Explicit story endings remain authoritative.
// The editable default is a model template, not a species or age inference.
func bodyPregnancyTerm(character bodyCharacterConfig, value map[string]any, modeled bool) map[string]any {
	if len(value) == 0 {
		return value
	}
	out := storyClockJSONMap(value)
	if len(mapFromAny(out["modeled_birth_time"])) > 0 {
		return out
	}
	days := intFromAny(out["gestation_days"], character.GestationDays)
	anchor, basis := mapFromAny(out["conception_time"]), "stated_conception_time"
	if modeled {
		anchor, basis = mapFromAny(out["ovulation_time"]), "modeled_ovulation_time"
	} else if len(anchor) == 0 {
		// Confirmation is the latest possible start, not proof that conception
		// happened that day. Preserve that uncertainty instead of inventing age.
		anchor, basis = mapFromAny(out["occurred_at"]), "confirmation_as_latest_start"
		if len(anchor) == 0 {
			anchor, basis = mapFromAny(out["observed_at"]), "observation_as_latest_start"
		}
	}
	if days > 0 && storyTimeBounds(bodyCycleDayCoordinate(anchor)).valid {
		out["gestation_days"] = days
		out["term_basis"] = basis
		out["modeled_birth_time"] = storyTimeRelative(bodyCycleDayCoordinate(anchor), map[string]any{"anchor": "source_observation", "unit": "day", "offset": days})
	}
	return out
}

func bodyPregnancyTermReading(value, clock map[string]any) map[string]any {
	out := map[string]any{"authority": "fiction_simulation", "birth_observed": false, "child_outcome": "not_inferred", "knowledge": "not_inferred"}
	for _, key := range []string{"modeled_birth_time", "gestation_days", "term_basis", "paternity"} {
		if v, ok := value[key]; ok {
			out[key] = v
		}
	}
	relation := storyTimeRelation(mapFromAny(value["modeled_birth_time"]), clock)
	out["birth_relation"] = relation
	switch stringFromMap(relation, "relation") {
	case "past", "same_day", "same_instant":
		out["stage"] = "modeled_birth_completed"
		out["interpretation"] = "The configured pregnancy term has elapsed in story time. Treat childbirth as completed only in the fiction model; no observed birth, child details or knowledge are established."
	default:
		out["stage"] = "term_not_elapsed_or_unknown"
	}
	return out
}

func bodyTrackingTermProjection(character bodyCharacterConfig, projection map[string]any) map[string]any {
	out := storyClockJSONMap(projection)
	if pregnancy := mapFromAny(out["pregnancy"]); stringFromMap(pregnancy, "status") == "confirmed" {
		out["pregnancy"] = bodyPregnancyTerm(character, pregnancy, false)
	}
	if model := mapFromAny(out["modeled_pregnancy"]); len(model) > 0 {
		out["modeled_pregnancy"] = bodyPregnancyTerm(character, model, true)
	}
	return out
}

func bodyPregnancyConfirmedTerm(character bodyCharacterConfig, observation, projection map[string]any) map[string]any {
	if len(mapFromAny(observation["modeled_birth_time"])) == 0 && len(mapFromAny(observation["conception_time"])) == 0 {
		model := bodyPregnancyTerm(character, mapFromAny(projection["modeled_pregnancy"]), true)
		at := mapFromAny(observation["occurred_at"])
		if len(at) == 0 {
			at = mapFromAny(observation["observed_at"])
		}
		// Reconfirmation of an observed pregnancy keeps its established term too.
		// A completed/ended prior pregnancy does not supply a new pregnancy's dates.
		prior := bodyPregnancyTerm(character, mapFromAny(projection["pregnancy"]), false)
		if stringFromMap(prior, "status") == "confirmed" && bodyPregnancyTermReading(prior, at)["stage"] != "modeled_birth_completed" {
			if len(mapFromAny(prior["modeled_birth_time"])) > 0 {
				// Correcting the original confirmation date also corrects a term that
				// was anchored solely to that date. A later confirmation keeps the term.
				sameEvent := stringFromMap(observation, "semantic_event_key") != "" && stringFromMap(observation, "semantic_event_key") == stringFromMap(prior, "semantic_event_key")
				if sameEvent && stringFromMap(prior, "term_basis") == "confirmation_as_latest_start" && mustCompactJSON(at) != mustCompactJSON(prior["occurred_at"]) {
					if _, supplied := observation["paternity"]; !supplied {
						observation["paternity"] = prior["paternity"]
					}
					return bodyPregnancyTerm(character, observation, false)
				}
				out := storyClockJSONMap(observation)
				if _, supplied := out["paternity"]; !supplied {
					out["paternity"] = prior["paternity"]
				}
				for _, key := range []string{"conception_time", "modeled_birth_time", "gestation_days", "term_basis"} {
					if value, exists := prior[key]; exists {
						out[key] = value
					}
				}
				return out
			}
		}
		stage := bodyPregnancyReading(model, at)["stage"]
		if stage == "modeled_implanted_pregnancy" || stage == "modeled_pre_implantation" {
			out := storyClockJSONMap(observation)
			if _, supplied := out["paternity"]; !supplied {
				out["paternity"] = model["paternity"]
			}
			out["modeled_birth_time"], out["gestation_days"] = model["modeled_birth_time"], model["gestation_days"]
			out["term_basis"] = "existing_modeled_conception"
			return out
		}
	}
	return bodyPregnancyTerm(character, observation, false)
}
