package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type prepareTurnBodyTrackingContext struct {
	SessionID string
	Config    bodyTrackingConfig
	Values    []store.StatusCurrentValue
}

const bodyTrackingAdditionalBudgetChars = 3000

// The injection contains a current reading, not a serialization of the ledger.
// Keep dates, uncertainty, evidence and authority; storage IDs remain in lineage.
func bodyTrackingPromptReading(kind string, value, clock map[string]any) map[string]any {
	out := map[string]any{"current_story_date": bodyCycleDayCoordinate(clock)}
	copyFields := func(target, source map[string]any, keys ...string) {
		for _, key := range keys {
			if v, ok := source[key]; ok && v != nil {
				target[key] = v
			}
		}
	}
	switch kind {
	case "observed_fact":
		fact := mapFromAny(value["fact"])
		copyFields(out, fact, "kind", "status", "historical_status", "occurred_at", "paternity")
		out["evidence"] = mapFromAny(fact["source"])["evidence_excerpt"]
		out["current_relation"] = mapFromAny(value["time"])["current_relation"]
		if term := mapFromAny(value["current_model_interpretation"]); len(term) > 0 {
			out["current_model_interpretation"] = bodyTrackingPromptReading("fiction_simulation", term, clock)
		}
	case "calculated_estimate":
		copyFields(out, value, "authority", "species", "world_rule")
		copyFields(out, mapFromAny(value["estimate"]), "status", "reason", "reference_time", "reference_kind", "cycle_day", "menstruation_possible", "menstruation_estimate", "fertile_window_possible", "ovulation_estimate", "fertile_window_estimate", "next_period_estimate", "uncertainty", "date_bounds_kind")
	case "fiction_simulation":
		copyFields(out, value, "authority", "stage", "ovulation_time", "implantation_time", "modeled_birth_time", "gestation_days", "term_basis", "birth_observed", "child_outcome", "interpretation", "symptoms", "knowledge", "paternity")
	}
	if p := mapFromAny(out["paternity"]); len(p) > 0 {
		compact := map[string]any{}
		copyFields(compact, p, "status", "unidentified_partner", "knowledge", "pregnancy_status")
		people := []any{}
		for _, raw := range sliceFromAny(p["candidates"]) {
			person := mapFromAny(raw)
			people = append(people, map[string]any{"character_name": extractionFirstNonEmpty(stringFromMap(person, "character_name"), stringFromMap(person, "entity_id"))})
		}
		compact["candidates"] = people
		out["paternity"] = compact
	}
	return out
}

// Pick the pregnancy association, never the most recent relationship partner.
func bodyTrackingPaternityReading(projection, clock map[string]any) map[string]any {
	pregnancy, model := mapFromAny(projection["pregnancy"]), mapFromAny(projection["modeled_pregnancy"])
	value, kind := model, "modeled"
	if len(pregnancy) > 0 && (len(model) == 0 || stringFromMap(pregnancy, "status") == "confirmed" && stringFromMap(storyTimeRelation(mapFromAny(model["ovulation_time"]), mapFromAny(pregnancy["modeled_birth_time"])), "relation") != "future") {
		value, kind = pregnancy, stringFromMap(pregnancy, "status")
	}
	if len(value) == 0 {
		return nil
	}
	out := storyClockJSONMap(mapFromAny(value["paternity"]))
	if len(out) == 0 {
		out["status"] = "unknown"
	}
	out["pregnancy_status"], out["knowledge"] = kind, "not_inferred"
	return out
}

// Only the saved current candidate is a current model state. A prior successful
// evaluation cannot resurrect it after an explicit pregnancy ending.
func bodyTrackingModelReading(projection, clock map[string]any) map[string]any {
	model, latest := mapFromAny(projection["modeled_pregnancy"]), mapFromAny(projection["latest_model_result"])
	pregnancy := mapFromAny(projection["pregnancy"])
	if stringFromMap(pregnancy, "status") == "confirmed" {
		term := bodyPregnancyTermReading(pregnancy, clock)
		if term["stage"] == "modeled_birth_completed" && stringFromMap(storyTimeRelation(mapFromAny(model["ovulation_time"]), mapFromAny(pregnancy["modeled_birth_time"])), "relation") != "future" {
			return term
		}
	}
	if len(model) == 0 && len(latest) == 0 {
		return nil
	}
	out := bodyPregnancyReading(model, clock)
	out["paternity"] = bodyTrackingPaternityReading(projection, clock)
	delete(out, "reference_family") // Internal deterministic identity is not narration/UI material.
	out["disclosure"], out["desire"] = "not_inferred", "not_inferred"
	if out["interpretation"] == nil {
		out["interpretation"] = "Explicit observed body facts take precedence over model inference."
	}
	if len(latest) > 0 {
		out["latest_evaluation"] = map[string]any{"status": latest["status"], "outcome": latest["outcome"], "reason": latest["reason"]}
	}
	return out
}

// Body facts and author-model estimates enter the existing measured candidate
// path. This projection never updates a cycle, a body fact or character knowledge.
func prepareTurnAppendBodyTracking(out *prepareTurnInjectionAssembly, input *prepareTurnBodyTrackingContext, scope prepareTurnRequestEntityScope, perspective, clock map[string]any) {
	if out == nil || input == nil || !input.Config.CycleTrackingEnabled && !input.Config.AutomaticPregnancyEnabled {
		return
	}
	out.BodyTrackingBudgetChars = bodyTrackingAdditionalBudgetChars
	currentByEntity := map[string]store.StatusCurrentValue{}
	for _, current := range input.Values {
		if current.ChatSessionID == input.SessionID && current.StatusKey == bodyTrackingStatusKey {
			currentByEntity[current.OwnerID] = current
		}
	}
	start := len(out.PriorityFactSeeds)
	visibilityFor := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || value == "private" {
			return "owner_private"
		}
		return value
	}
	for _, character := range input.Config.Characters {
		if !prepareTurnCharacterMemoryEntityInScope(character.EntityID, character.CharacterName, scope) {
			continue
		}
		current := currentByEntity[character.EntityID]
		projection := bodyTrackingTermProjection(character, parseJSONMap(current.ValueJSON))
		// Reuse the same visibility decision for facts, cycle limitations and
		// model readings. A restricted fact cannot leak through a missing forecast.
		visibleProjection := map[string]any{}
		for _, key := range []string{"pregnancy", "recovery", "modeled_pregnancy", "latest_model_result"} {
			value := mapFromAny(projection[key])
			if len(value) > 0 {
				if _, _, eligible := prepareTurnCharacterMemoryVisibility(visibilityFor(stringFromMap(value, "visibility")), character.EntityID, perspective); eligible {
					visibleProjection[key] = value
				}
			}
		}
		if period := mapFromAny(mapFromAny(projection["observed_facts"])["period_start"]); len(period) > 0 {
			if _, _, eligible := prepareTurnCharacterMemoryVisibility(visibilityFor(stringFromMap(period, "visibility")), character.EntityID, perspective); eligible {
				visibleProjection["cycle_reference"] = projection["cycle_reference"]
			}
		}
		appendReading := func(kind string, value map[string]any, visibility, table, occurrence string, rowID any, sourceTurn int) {
			visibility = visibilityFor(visibility)
			visibility, guard, eligible := prepareTurnCharacterMemoryVisibility(visibility, character.EntityID, perspective)
			if !eligible {
				return
			}
			lane, owner := "subjective_relationship", character.EntityID
			viewers := []string{character.EntityID}
			if visibility == "public" && kind == "observed_fact" {
				lane, owner, viewers = "character_objective", "", nil
			}
			text := fmt.Sprintf("%s: %s; narrator context; knowledge_not_inferred; %s", character.CharacterName, kind, mustCompactJSON(bodyTrackingPromptReading(kind, value, clock)))
			if guard != "" {
				text = "[guard=" + guard + "] " + text
			}
			path := prepareTurnMemoryPath([]string{"body_tracking", character.EntityID, occurrence})
			fact := prepareTurnPriorityMemoryFact{
				Text: text, SourcePath: path, EntitySurface: character.CharacterName, Structured: true,
				MemoryRole: kind,
				Reading:    &prepareTurnMemoryContext{Path: path, Label: "Body context", Parts: []prepareTurnMemoryPart{{Key: path, Value: text, FactTexts: []string{text}}}},
			}
			appendPrepareTurnPriorityFactSeeds(out, prepareTurnPrioritySourceMetadata{
				Lane: lane, SourceTable: table, Tier: "required", SourceRowID: rowID,
				SourceOccurrence: occurrence, SourceTurn: sourceTurn,
				Visibility: visibility, PerspectiveOwner: owner, AllowedViewers: viewers,
			}, text, []prepareTurnPriorityMemoryFact{fact}, "body_tracking:"+kind)
		}
		// Period start is a dated observation, never a statement that menstruation
		// is occurring now. Pregnancy/recovery use the authoritative current slot,
		// rather than replaying both the old confirmation and a later ending.
		facts := []map[string]any{mapFromAny(mapFromAny(projection["observed_facts"])["period_start"]), mapFromAny(projection["pregnancy"]), mapFromAny(projection["recovery"])}
		for _, fact := range facts {
			if len(fact) == 0 {
				continue
			}
			kind := stringFromMap(fact, "kind")
			source := mapFromAny(fact["source"])
			bodyFact := storyClockJSONMap(fact)
			if kind == "pregnancy_confirmed" && bodyFact["paternity"] == nil {
				bodyFact["paternity"] = map[string]any{"status": "unknown"}
			}
			if p := mapFromAny(bodyFact["paternity"]); len(p) > 0 {
				parentage := map[string]any{"kind": "pregnancy_parentage", "paternity": p, "occurred_at": fact["occurred_at"]}
				parentageKind, parentageValue := "observed_fact", map[string]any{"fact": parentage}
				if p["basis"] == "modeled_exposure" {
					parentageKind = "fiction_simulation"
					parentageValue = map[string]any{"authority": "fiction_simulation", "paternity": p, "knowledge": "not_inferred"}
				}
				appendReading(parentageKind, parentageValue, extractionFirstNonEmpty(stringFromMap(p, "visibility"), stringFromMap(fact, "visibility")), "status_current_values", fmt.Sprintf("body-parentage:%d", current.ID), current.ID, intFromAny(source["source_turn"], 0))
				delete(bodyFact, "paternity")
			}
			delete(bodyFact, "knowledge_source") // The existing holder-scoped perspective lane owns knowledge evidence.
			reading := map[string]any{"fact": bodyFact, "time": buildStoryTimeReading(map[string]any{"observed_at": fact["observed_at"], "occurrence_time": fact["occurred_at"]}, clock)}
			if kind == "pregnancy_confirmed" {
				term := bodyPregnancyTermReading(fact, clock)
				delete(term, "paternity") // Parentage has its own existing visibility lane.
				reading["current_model_interpretation"] = term
				if term["stage"] == "modeled_birth_completed" {
					bodyFact["historical_status"] = bodyFact["status"]
					delete(bodyFact, "status")
				}
			}
			occurrence := extractionFirstNonEmpty(stringFromMap(source, "source_unit_id"), stringFromMap(fact, "semantic_event_key"), fmt.Sprintf("body-current:%d:%s", current.ID, kind))
			appendReading("observed_fact", reading, stringFromMap(fact, "visibility"), "status_current_values", occurrence, current.ID, intFromAny(source["source_turn"], 0))
		}
		if input.Config.CycleTrackingEnabled && character.CycleEnabled {
			reading := map[string]any{
				"authority": "author_model", "estimate": bodyTrackingCycleReading(character, visibleProjection, clock),
				"species": character.Species, "world_rule": character.WorldRule,
				"knowledge": "not_inferred", "disclosure": "not_inferred", "desire": "not_inferred",
			}
			appendReading("calculated_estimate", reading, "owner_private", "body_tracking_settings", "body-model:"+input.SessionID+":"+character.EntityID, character.EntityID, 0)
		}
		if input.Config.AutomaticPregnancyEnabled {
			if reading := bodyTrackingModelReading(visibleProjection, clock); len(reading) > 0 {
				origin := mapFromAny(visibleProjection["modeled_pregnancy"])
				if len(origin) == 0 {
					origin = mapFromAny(visibleProjection["latest_model_result"])
				}
				source := mapFromAny(origin["source"])
				visibility := stringFromMap(origin, "visibility")
				if visibility == "public" {
					visibility = "owner_private" // A public exposure does not disclose its latent model outcome.
				}
				appendReading("fiction_simulation", reading, visibility, "status_current_values", fmt.Sprintf("body-pregnancy-model:%d", current.ID), current.ID, intFromAny(source["source_turn"], 0))
			}
		}
	}
	out.Counts["body_tracking_candidate_count"] = len(out.PriorityFactSeeds) - start
}
