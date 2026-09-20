package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const bodyTrackingStatusKey = "body_tracking"
const bodyTrackingStateContract = "body_tracking.v1"

func (s *Server) bodyTrackingCurrentValues(ctx context.Context, sid string) ([]store.StatusCurrentValue, error) {
	if reader, ok := s.Store.(store.ReversibleStatusTransitionStore); ok {
		return reader.ListReversibleStatusCurrentValues(ctx, sid, reversibleStateOwnerScope, []string{bodyTrackingStatusKey})
	}
	return nil, nil
}

func bodyTrackingCycleSpec(character bodyCharacterConfig, current store.StatusCurrentValue) bodyCycleSpec {
	spec := character.Cycle
	if reference := mapFromAny(parseJSONMap(current.ValueJSON)["cycle_reference"]); len(reference) > 0 {
		spec.ReferenceTime = storyClockJSONMap(reference)
		spec.ReferenceKind = "observed_period_start"
	}
	return spec
}

func bodyTrackingCharacter(cfg bodyTrackingConfig, event map[string]any) (bodyCharacterConfig, bool) {
	id := strings.TrimSpace(stringFromMap(event, "character_id"))
	name := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(event, "subject_name"), stringFromMap(event, "character_name")))
	for _, character := range cfg.Characters {
		if id != "" && id == character.EntityID || id == "" && name != "" && name == character.CharacterName {
			return character, true
		}
	}
	return bodyCharacterConfig{}, false
}

func bodyTrackingEventSourceUnit(character bodyCharacterConfig, semanticKey string) string {
	origin := extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
	sum := sha256.Sum256([]byte(bodyTrackingStateContract + "\x1f" + origin + "\x1f" + semanticKey))
	return "body:" + hex.EncodeToString(sum[:])
}

// Semantic event identity survives a later mention and a session copy. Source
// activity still belongs to the existing revision owner, including replacement.
func (s *Server) bodyTrackingActiveHistory(ctx context.Context, sid, ownerID string) ([]store.StatusChangeEvent, error) {
	values, err := s.bodyTrackingCurrentValues(ctx, sid)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.OwnerID == ownerID && parseJSONMap(value.ValueJSON)["deleted"] == true {
			return nil, nil
		}
	}
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		return nil, nil
	}
	events, err := lifecycle.ListStatusChangeEvents(ctx, sid, reversibleStateOwnerScope, ownerID, bodyTrackingStatusKey, -1)
	if err != nil {
		return nil, err
	}
	activeEvents := make([]store.StatusChangeEvent, 0, len(events))
	for _, event := range events {
		if event.ChatSessionID != sid || event.OwnerID != ownerID || event.StatusKey != bodyTrackingStatusKey {
			continue
		}
		evidence := parseJSONMap(event.EvidenceJSON)
		if revision := stringFromMap(evidence, "source_revision"); revision != "" {
			if sourceStore, ok := s.Store.(store.SourceRevisionStore); ok {
				active, err := sourceStore.IsSourceRevisionActive(ctx, sid, revision)
				if err != nil {
					return nil, err
				}
				if !active {
					continue
				}
			}
		}
		activeEvents = append(activeEvents, event)
	}
	return activeEvents, nil
}

// A later source may clarify the same event. Retain known optional fields,
// while the existing source-revision transaction still owns retry idempotency.
func bodyTrackingObservationUpdate(events []store.StatusChangeEvent, sourceUnit string, observation map[string]any) (map[string]any, bool) {
	var latest *store.StatusChangeEvent
	for i := range events {
		event := &events[i]
		if stringFromMap(parseJSONMap(event.EvidenceJSON), "source_unit_id") == sourceUnit {
			if latest == nil || event.SourceTurn > latest.SourceTurn || event.SourceTurn == latest.SourceTurn && event.ID > latest.ID {
				latest = event
			}
		}
	}
	if latest == nil {
		return observation, false
	}
	prior := mapFromAny(parseJSONMap(latest.EvidenceJSON)["history_observation"])
	before, after := map[string]any{}, map[string]any{}
	merged := storyClockJSONMap(observation)
	for _, key := range []string{"kind", "occurred_at", "conception_time", "gestation_days", "scene_scope", "exposure", "partners", "paternity", "visibility", "knowledge_source"} {
		if value, exists := prior[key]; exists {
			before[key], after[key] = value, value
			if _, supplied := merged[key]; !supplied {
				merged[key] = value
			}
		}
		if value, supplied := observation[key]; supplied {
			after[key] = value
		}
	}
	return merged, mustCompactJSON(before) == mustCompactJSON(after)
}

// Read the latest clarification per semantic event; retain the full history
// separately for frozen model checkpoints and source rollback.
func bodyTrackingLatestObservationEvents(events []store.StatusChangeEvent) []store.StatusChangeEvent {
	latest := map[string]store.StatusChangeEvent{}
	for _, event := range events {
		unit := stringFromMap(parseJSONMap(event.EvidenceJSON), "source_unit_id")
		if unit == "" {
			continue
		}
		prior, exists := latest[unit]
		if !exists || event.SourceTurn > prior.SourceTurn || event.SourceTurn == prior.SourceTurn && event.ID > prior.ID {
			latest[unit] = event
		}
	}
	out := make([]store.StatusChangeEvent, 0, len(events))
	for _, event := range events {
		unit := stringFromMap(parseJSONMap(event.EvidenceJSON), "source_unit_id")
		if unit == "" || latest[unit].ID == event.ID {
			out = append(out, event)
		}
	}
	return out
}

// An event is evaluated against its own occurrence date. Later observed periods
// never become the reference for an earlier exposure. An explicit author
// correction is read from current authority, so undo cannot revive its old row.
func bodyTrackingCycleSpecAtOccurrence(character bodyCharacterConfig, previous store.StatusCurrentValue, observation map[string]any, events []store.StatusChangeEvent) bodyCycleSpec {
	spec := character.Cycle
	current := mapFromAny(mapFromAny(parseJSONMap(previous.ValueJSON)["observed_facts"])["period_start"])
	occurred := mapFromAny(observation["occurred_at"])
	if len(current) > 0 && stringFromMap(storyTimeRelation(mapFromAny(current["occurred_at"]), occurred), "relation") != "future" {
		if stringFromMap(mapFromAny(current["source"]), "kind") == "author_setting" || !storyTimeBounds(bodyCycleDayCoordinate(mapFromAny(current["occurred_at"]))).valid {
			return bodyTrackingCycleSpec(character, previous)
		}
	}
	var chosen map[string]any
	var chosenOrder int64
	for _, event := range bodyTrackingLatestObservationEvents(events) {
		evidence := parseJSONMap(event.EvidenceJSON)
		if stringFromMap(evidence, "source_revision") == "" {
			continue
		}
		fact := mapFromAny(evidence["history_observation"])
		if stringFromMap(fact, "kind") != "period_start" || bodyTrackingNonActualObservation(fact) {
			continue
		}
		relation := stringFromMap(storyTimeRelation(mapFromAny(fact["occurred_at"]), occurred), "relation")
		if relation != "past" && relation != "same_day" && relation != "same_instant" {
			continue
		}
		order := int64(event.SourceTurn)<<32 | event.ID
		vsChosen := stringFromMap(storyTimeRelation(mapFromAny(fact["occurred_at"]), mapFromAny(chosen["occurred_at"])), "relation")
		if chosen == nil || vsChosen == "future" || vsChosen != "past" && order > chosenOrder {
			chosen, chosenOrder = fact, order
		}
	}
	if chosen != nil {
		spec.ReferenceTime = storyClockJSONMap(mapFromAny(chosen["occurred_at"]))
		spec.ReferenceKind = "observed_period_start"
	}
	return spec
}

func bodyTrackingNonActualObservation(observation map[string]any) bool {
	scene := stringFromMap(observation, "scene_scope")
	return scene == "planned" || scene == "hypothetical"
}

func bodyTrackingPregnancyAtOccurrence(previous store.StatusCurrentValue, observation map[string]any, events []store.StatusChangeEvent) map[string]any {
	current := mapFromAny(parseJSONMap(previous.ValueJSON)["pregnancy"])
	occurred := mapFromAny(observation["occurred_at"])
	if len(current) > 0 && stringFromMap(storyTimeRelation(mapFromAny(current["occurred_at"]), occurred), "relation") != "future" {
		if stringFromMap(mapFromAny(current["source"]), "kind") == "author_setting" || len(mapFromAny(current["occurred_at"])) == 0 {
			return current
		}
	}
	var chosen map[string]any
	var chosenOrder int64
	for _, event := range bodyTrackingLatestObservationEvents(events) {
		evidence := parseJSONMap(event.EvidenceJSON)
		if stringFromMap(evidence, "source_revision") == "" {
			continue
		}
		fact := mapFromAny(evidence["history_observation"])
		kind := stringFromMap(fact, "kind")
		if kind != "pregnancy_confirmed" && kind != "pregnancy_ended" || bodyTrackingNonActualObservation(fact) {
			continue
		}
		relation := stringFromMap(storyTimeRelation(mapFromAny(fact["occurred_at"]), occurred), "relation")
		if relation != "past" && relation != "same_day" && relation != "same_instant" {
			continue
		}
		order := int64(event.SourceTurn)<<32 | event.ID
		vsChosen := stringFromMap(storyTimeRelation(mapFromAny(fact["occurred_at"]), mapFromAny(chosen["occurred_at"])), "relation")
		if chosen == nil || vsChosen == "future" || vsChosen != "past" && order > chosenOrder {
			chosen, chosenOrder = storyClockJSONMap(fact), order
			chosen["status"] = strings.TrimPrefix(kind, "pregnancy_")
		}
	}
	return chosen
}

// Counterparts are optional observations. Reuse the session identity owner;
// an unresolved name never prevents storing or evaluating a body event.
func (s *Server) bodyTrackingPeople(ctx context.Context, sid string, raw any) []any {
	out := []any{}
	seen := map[string]bool{}
	for _, item := range sliceFromAny(raw) {
		person := mapFromAny(item)
		id, name := stringFromMap(person, "entity_id"), stringFromMap(person, "character_name")
		if resolver, ok := s.Store.(store.UniqueActiveEntitySurfaceIdentityResolver); ok && name != "" {
			if resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(name)); err == nil {
				id = resolved.StableEntityID
				if resolved.CanonicalLabel != "" {
					name = resolved.CanonicalLabel
				}
			}
		}
		if id != "" {
			id = s.characterIdentityRoot(ctx, sid, id)
		}
		key := extractionFirstNonEmpty(id, comparableEntityKey(name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		value := map[string]any{"character_name": name}
		if id != "" {
			value["entity_id"] = id
		}
		out = append(out, value)
	}
	return out
}

func (s *Server) bodyTrackingParentageObservation(ctx context.Context, sid string, observation map[string]any) {
	if people, supplied := observation["partners"]; supplied {
		observation["partners"] = s.bodyTrackingPeople(ctx, sid, people)
	}
	if raw, supplied := observation["paternity"]; supplied {
		input := mapFromAny(raw)
		p := map[string]any{"status": input["status"], "evidence_excerpt": input["evidence_excerpt"]}
		p["candidates"] = s.bodyTrackingPeople(ctx, sid, input["candidates"])
		p["knowledge"] = "not_inferred"
		p["visibility"] = extractionFirstNonEmpty(stringFromMap(observation, "visibility"), "private")
		if p["evidence_excerpt"] == nil {
			p["evidence_excerpt"] = observation["evidence_excerpt"]
		}
		if len(sliceFromAny(p["candidates"])) == 0 {
			p["status"] = "unknown"
		}
		observation["paternity"] = p
	}
}

// Associate counterparts using already-saved cycle decisions. Do not choose a
// father, reroll an outcome or infer parentage from marriage/current scene focus.
func bodyTrackingModelPaternity(model map[string]any, events []store.StatusChangeEvent, observation, result map[string]any) map[string]any {
	out := map[string]any{"status": "unknown", "knowledge": "not_inferred", "basis": "modeled_exposure", "visibility": "private", "model_event_key": model["semantic_event_key"]}
	people, bases := []any{}, []any{}
	seen := map[string]bool{}
	unknown := false
	var selectedCycle *bodyPregnancyCycle
	for _, event := range events {
		for _, cycle := range bodyPregnancyFrozenCycles([]map[string]any{parseJSONMap(event.EvidenceJSON)}) {
			if cycle.Family == stringFromMap(model, "reference_family") && cycle.Index == intFromAny(model["cycle_index"], -1) {
				copy := cycle
				selectedCycle = &copy
			}
		}
	}
	include := func(fact, trial map[string]any) {
		relevant := false
		for _, raw := range sliceFromAny(trial["cycles"]) {
			cycle := mapFromAny(raw)
			q, _ := storyClockNumeric(cycle["conditional_day_probability"])
			if cycle["reference_family"] == model["reference_family"] && cycle["cycle_index"] == model["cycle_index"] && q > 0 {
				relevant = true
			}
		}
		// Ongoing pregnancy suppresses another simulation, not observation of a
		// counterpart on the same fertile date. Reuse its frozen kernel without a draw.
		if selectedCycle != nil && stringFromMap(trial, "reason") == "modeled_pregnancy_ongoing" && stringFromMap(mapFromAny(fact["exposure"]), "classification") == "potentially_conceiving" {
			day := storyTimeBounds(bodyCycleDayCoordinate(mapFromAny(fact["occurred_at"])))
			if day.valid && day.calendar == selectedCycle.Calendar && day.minimum == day.maximum {
				relevant = bodyPregnancyConditionalProbability(selectedCycle.Parameters, day.minimum-selectedCycle.OvulationDay) > 0
			}
		}
		if !relevant {
			return
		}
		partners := sliceFromAny(fact["partners"])
		if len(partners) == 0 {
			unknown = true
		}
		for _, raw := range partners {
			person := mapFromAny(raw)
			key := extractionFirstNonEmpty(stringFromMap(person, "entity_id"), comparableEntityKey(stringFromMap(person, "character_name")))
			if key != "" && !seen[key] {
				seen[key] = true
				people = append(people, storyClockJSONMap(person))
			}
		}
		bases = append(bases, map[string]any{"semantic_event_key": fact["semantic_event_key"], "occurred_at": fact["occurred_at"]})
	}
	for _, event := range bodyTrackingLatestObservationEvents(events) {
		evidence := parseJSONMap(event.EvidenceJSON)
		fact := mapFromAny(evidence["history_observation"])
		if fact["semantic_event_key"] != observation["semantic_event_key"] {
			include(fact, mapFromAny(evidence["model_result"]))
		}
	}
	// Decode in-flight cycle numbers just as in persisted JSON.
	include(observation, parseJSONMap(mustCompactJSON(result)))
	out["candidates"], out["basis_events"], out["unidentified_partner"] = people, bases, unknown
	if len(people) > 0 {
		out["status"] = "candidates"
	}
	if len(people) == 1 && !unknown {
		out["status"] = "modeled_link"
	}
	return out
}

func bodyTrackingExposureModel(cfg bodyTrackingConfig, character bodyCharacterConfig, previous store.StatusCurrentValue, observation map[string]any, events []store.StatusChangeEvent) (map[string]any, []bodyPregnancyCycle) {
	context := map[string]any{"model_version": bodyPregnancyModelVersion, "authority": "fiction_simulation", "status": "unknown", "knowledge": "not_inferred", "symptoms": "not_inferred", "semantic_event_key": observation["semantic_event_key"], "occurrence_time": observation["occurred_at"]}
	if bodyTrackingNonActualObservation(observation) {
		context["reason"] = "planned_or_hypothetical_observation"
		return context, nil
	}
	if cfg.AutomaticPregnancyEnabled {
		pregnancy := bodyTrackingPregnancyAtOccurrence(previous, observation, events)
		pregnancy = bodyPregnancyTerm(character, pregnancy, false)
		if stringFromMap(pregnancy, "status") == "confirmed" && bodyPregnancyTermReading(pregnancy, mapFromAny(observation["occurred_at"]))["stage"] != "modeled_birth_completed" {
			context["status"], context["reason"] = "not_applicable", "observed_pregnancy_ongoing"
			return context, nil
		}
		modeled := bodyTrackingModeledPregnancyAtOccurrence(previous, observation, events, pregnancy)
		stage := stringFromMap(bodyPregnancyReading(bodyPregnancyTerm(character, modeled, true), mapFromAny(observation["occurred_at"])), "stage")
		if stage == "modeled_pre_implantation" || stage == "modeled_implanted_pregnancy" {
			context["status"], context["reason"] = "not_applicable", "modeled_pregnancy_ongoing"
			return context, nil
		}
	}
	evidence := make([]map[string]any, 0, len(events))
	for _, event := range events {
		evidence = append(evidence, parseJSONMap(event.EvidenceJSON))
	}
	spec := bodyTrackingCycleSpecAtOccurrence(character, previous, observation, events)
	model, cycles := calculateBodyPregnancyEvent(cfg, character, spec, observation, bodyPregnancyFrozenCycles(evidence))
	model["configuration"] = map[string]any{"automatic_pregnancy_enabled": cfg.AutomaticPregnancyEnabled, "can_conceive": character.CanConceive, "species": character.Species, "world_rule": character.WorldRule, "cycle_reference": spec.ReferenceTime, "cycle_reference_kind": spec.ReferenceKind}
	if selected := storyClockJSONMap(mapFromAny(model["selected_pregnancy"])); len(selected) > 0 {
		selected = bodyPregnancyTerm(character, selected, true)
		selected["source"], selected["visibility"] = observation["source"], observation["visibility"]
		selected["semantic_event_key"], selected["occurrence_time"] = observation["semantic_event_key"], observation["occurred_at"]
		selected["paternity"] = bodyTrackingModelPaternity(parseJSONMap(mustCompactJSON(selected)), events, observation, model)
		model["selected_pregnancy"] = selected
	}
	return model, cycles
}

func bodyTrackingModeledPregnancyAtOccurrence(previous store.StatusCurrentValue, observation map[string]any, events []store.StatusChangeEvent, observedPregnancy map[string]any) map[string]any {
	candidates := []map[string]any{mapFromAny(parseJSONMap(previous.ValueJSON)["modeled_pregnancy"])}
	for _, event := range bodyTrackingLatestObservationEvents(events) {
		evidence := parseJSONMap(event.EvidenceJSON)
		if bodyTrackingNonActualObservation(mapFromAny(evidence["history_observation"])) {
			continue
		}
		candidates = append(candidates, mapFromAny(mapFromAny(evidence["model_result"])["selected_pregnancy"]))
	}
	var selected map[string]any
	for _, candidate := range candidates {
		if len(candidate) == 0 || stringFromMap(observation, "semantic_event_key") != "" && stringFromMap(candidate, "semantic_event_key") == stringFromMap(observation, "semantic_event_key") {
			continue
		}
		if bodyPregnancyTermReading(candidate, mapFromAny(observation["occurred_at"]))["stage"] == "modeled_birth_completed" {
			continue
		}
		if stringFromMap(observedPregnancy, "status") == "ended" {
			end := mapFromAny(observedPregnancy["occurred_at"])
			vsEvent := stringFromMap(storyTimeRelation(end, mapFromAny(observation["occurred_at"])), "relation")
			vsExposure := stringFromMap(storyTimeRelation(end, mapFromAny(candidate["occurrence_time"])), "relation")
			if (vsEvent == "past" || vsEvent == "same_day" || vsEvent == "same_instant") && (vsExposure == "future" || vsExposure == "same_day" || vsExposure == "same_instant") {
				continue
			}
		}
		if selected == nil || bodyPregnancyCandidateBefore(candidate, selected) {
			selected = candidate
		}
	}
	return selected
}

func (s *Server) ensureBodyTrackingDefinition(ctx context.Context, sid string, now time.Time) (store.StatusSchemaDefinition, error) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		return store.StatusSchemaDefinition{}, errors.New("body tracking status registry unavailable")
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, bodyTrackingStatusKey, reversibleStateOwnerScope)
	if err == nil || !errors.Is(err, store.ErrNotFound) {
		return definition, err
	}
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid, SchemaName: bodyTrackingStatusKey, StatusKey: bodyTrackingStatusKey,
		Label: "Optional character body state", OwnerScope: reversibleStateOwnerScope, ValueKind: "object",
		OptionsJSON:   mustCompactJSON(map[string]any{"contract_version": bodyTrackingStateContract, "visibility": "private", "story_time_only": true}),
		RegistryState: "active", CreatedAt: now, UpdatedAt: now,
	}})
	if err != nil {
		return definition, err
	}
	if len(definitions) == 0 {
		return definition, errors.New("body tracking status registry returned no definition")
	}
	return definitions[0], nil
}

func bodyTrackingObservationProjection(previous store.StatusCurrentValue, character bodyCharacterConfig, observation map[string]any, turn int, authorCorrection ...bool) (map[string]any, bool) {
	manual := len(authorCorrection) > 0 && authorCorrection[0]
	projection := storyClockJSONMap(parseJSONMap(previous.ValueJSON))
	projection["contract_version"] = bodyTrackingStateContract
	projection["subject_entity_id"], projection["subject_label"] = character.EntityID, character.CharacterName
	projection["origin_entity_id"] = extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
	if !manual && store.StatusCurrentObservationTurn(previous) > turn {
		return projection, false
	}
	if scene := stringFromMap(observation, "scene_scope"); !manual && (scene == "flashback" || scene == "planned" || scene == "hypothetical") {
		return projection, false
	}
	kind := stringFromMap(observation, "kind")
	facts := storyClockJSONMap(mapFromAny(projection["observed_facts"]))
	priorFact := mapFromAny(facts[kind])
	sameEvent := stringFromMap(observation, "semantic_event_key") != "" && stringFromMap(observation, "semantic_event_key") == stringFromMap(priorFact, "semantic_event_key")
	if !manual && !sameEvent && stringFromMap(storyTimeRelation(mapFromAny(observation["occurred_at"]), mapFromAny(priorFact["occurred_at"])), "relation") == "past" {
		return projection, false
	}
	facts[kind] = observation
	projection["observed_facts"] = facts
	switch kind {
	case "period_start":
		reference := storyClockJSONMap(mapFromAny(observation["occurred_at"]))
		if len(reference) == 0 {
			reference["story_time"] = "unknown"
		}
		projection["cycle_reference"] = reference
	case "pregnancy_confirmed", "pregnancy_ended", "recovery_started", "recovery_ended":
		field := "pregnancy"
		if strings.HasPrefix(kind, "recovery_") {
			field = "recovery"
		}
		prior := mapFromAny(projection[field])
		if manual || sameEvent && stringFromMap(prior, "semantic_event_key") == stringFromMap(observation, "semantic_event_key") || stringFromMap(storyTimeRelation(mapFromAny(observation["occurred_at"]), mapFromAny(prior["occurred_at"])), "relation") != "past" {
			state := storyClockJSONMap(observation)
			state["status"] = strings.TrimPrefix(kind, field+"_")
			if kind == "pregnancy_confirmed" {
				state = bodyPregnancyConfirmedTerm(character, state, projection)
			}
			if kind == "pregnancy_ended" {
				if _, supplied := state["paternity"]; !supplied {
					state["paternity"] = prior["paternity"]
					if state["paternity"] == nil {
						state["paternity"] = mapFromAny(projection["modeled_pregnancy"])["paternity"]
					}
				}
			}
			projection[field] = state
			if p, exists := state["paternity"]; exists {
				mapFromAny(facts[kind])["paternity"] = p
			}
			if kind == "pregnancy_ended" {
				delete(projection, "modeled_pregnancy")
			}
		}
	}
	return projection, true
}

func (s *Server) saveBodyTrackingFromExtraction(ctx context.Context, sid string, turn int, extraction map[string]any, content string, evidence []store.DirectEvidence, now time.Time, result *artifactSaveResult) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	source, accepted := storyClockSourceMetadata(ctx, sid, turn, content)
	if !accepted {
		if len(sliceFromAny(extraction["body_events"])) > 0 {
			result.addSkipReason("body_events", "accepted_source_required", nil)
		}
		return
	}
	cfg, err := s.initializeAutomaticBodyTracking(ctx, sid)
	if err != nil {
		result.ErrorDetails = append(result.ErrorDetails, "body tracking settings: "+err.Error())
		result.Errors++
		return
	}
	if !cfg.CycleTrackingEnabled && !cfg.AutomaticPregnancyEnabled || len(sliceFromAny(extraction["body_events"])) == 0 {
		return
	}
	atomicStore, ok := s.Store.(store.ReversibleStatusTransitionStore)
	if !ok {
		result.addSkipReason("body_events", "atomic_status_store_unavailable", nil)
		return
	}
	for index, raw := range sliceFromAny(extraction["body_events"]) {
		observation := storyClockJSONMap(mapFromAny(raw))
		kind := stringFromMap(observation, "kind")
		switch kind {
		case "period_start", "pregnancy_confirmed", "pregnancy_ended", "recovery_started", "recovery_ended", "conception_exposure":
		default:
			continue // Other observations remain in the admitted source memory.
		}
		character, found := bodyTrackingCharacter(cfg, observation)
		if !found {
			result.addSkipReason("body_events", "character_not_configured", map[string]any{"index": index})
			continue
		}
		semanticKey := strings.TrimSpace(stringFromMap(observation, "semantic_event_key"))
		if semanticKey == "" {
			semanticKey = fmt.Sprintf("source:%s:body:%d", source.LogicalTurnID, index)
		}
		sourceUnit := bodyTrackingEventSourceUnit(character, semanticKey)
		history, err := s.bodyTrackingActiveHistory(ctx, sid, character.EntityID)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "body event history: "+err.Error())
			continue
		}
		s.bodyTrackingParentageObservation(ctx, sid, observation)
		var unchanged bool
		observation, unchanged = bodyTrackingObservationUpdate(history, sourceUnit, observation)
		if unchanged {
			continue
		}
		values, err := s.bodyTrackingCurrentValues(ctx, sid)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "body current state: "+err.Error())
			continue
		}
		var previous store.StatusCurrentValue
		for _, value := range values {
			if value.OwnerID == character.EntityID {
				previous = value
			}
		}
		definition, err := s.ensureBodyTrackingDefinition(ctx, sid, now)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, err.Error())
			continue
		}
		excerpt := sanitizeEvidenceExcerptForTurn(stringFromMap(observation, "evidence_excerpt"), content)
		evidenceIDs := storyClockMatchingEvidenceIDs(evidence, sid, turn, excerpt)
		observation["semantic_event_key"], observation["character_id"] = semanticKey, character.EntityID
		observation["origin_entity_id"] = extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
		observation["observed_at"] = reversibleObservationContext(ctx, s.Store, sid)
		observation["source"] = map[string]any{"source_revision": source.Revision, "source_unit_id": sourceUnit, "source_turn": turn, "evidence_excerpt": excerpt, "direct_evidence_ids": evidenceIDs}
		if kind == "pregnancy_confirmed" {
			observation = bodyPregnancyConfirmedTerm(character, observation, parseJSONMap(previous.ValueJSON))
		}
		projection, currentAllowed := bodyTrackingObservationProjection(previous, character, observation, turn)
		var model map[string]any
		var modelCycles []bodyPregnancyCycle
		if kind == "conception_exposure" {
			model, modelCycles = bodyTrackingExposureModel(cfg, character, previous, observation, history)
			model["source"], model["visibility"] = observation["source"], observation["visibility"]
			if currentAllowed {
				projection["latest_model_result"] = model
				selected := mapFromAny(model["selected_pregnancy"])
				prior := mapFromAny(projection["modeled_pregnancy"])
				priorEnded := bodyPregnancyTermReading(bodyPregnancyTerm(character, prior, true), mapFromAny(observation["occurred_at"]))["stage"] == "modeled_birth_completed"
				sameEvent := len(prior) > 0 && stringFromMap(prior, "semantic_event_key") == semanticKey
				if sameEvent {
					delete(projection, "modeled_pregnancy")
				}
				if len(selected) > 0 && (sameEvent || len(prior) == 0 || priorEnded || bodyPregnancyCandidateBefore(selected, prior)) {
					projection["modeled_pregnancy"] = selected
				}
				if current := mapFromAny(projection["modeled_pregnancy"]); len(current) > 0 {
					current["paternity"] = bodyTrackingModelPaternity(parseJSONMap(mustCompactJSON(current)), history, observation, model)
				}
				// A confirmation may have inherited this model association. Refresh that
				// association on source correction; explicit parentage has no model key.
				pregnancy := mapFromAny(projection["pregnancy"])
				parentage := mapFromAny(pregnancy["paternity"])
				if key := stringFromMap(parentage, "model_event_key"); key != "" && (key == semanticKey || key == stringFromMap(mapFromAny(projection["modeled_pregnancy"]), "semantic_event_key")) {
					linked := mapFromAny(projection["modeled_pregnancy"])["paternity"]
					if linked == nil {
						linked = map[string]any{"status": "unknown", "knowledge": "not_inferred", "visibility": "private"}
					}
					pregnancy["paternity"] = linked
					if fact := mapFromAny(mapFromAny(projection["observed_facts"])["pregnancy_confirmed"]); len(fact) > 0 {
						fact["paternity"] = linked
					}
				}

			}
		}
		evidencePayload := map[string]any{
			"contract_version": bodyTrackingStateContract, "source": "critic.body_events", "source_contract": source.ContractVersion,
			"source_revision": source.Revision, "source_unit_id": sourceUnit, "logical_turn_id": source.LogicalTurnID, "source_turn": turn,
			"subject_entity_id": character.EntityID, "origin_entity_id": observation["origin_entity_id"], "semantic_event_key": semanticKey,
			"evidence_excerpt": excerpt, "direct_evidence_ids": evidenceIDs, "history_observation": observation, "current_projection": currentAllowed,
		}
		if model != nil {
			evidencePayload["model_result"], evidencePayload["model_cycles"] = model, modelCycles
		}
		var current *store.StatusCurrentValue
		if currentAllowed {
			current = &store.StatusCurrentValue{ChatSessionID: sid, RegistryID: definition.ID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, OwnerLabel: character.CharacterName, ValueKind: "object", ValueJSON: mustCompactJSON(projection), EvidenceJSON: mustCompactJSON(evidencePayload), SourceTurn: turn, WriteState: "current", CreatedAt: now, UpdatedAt: now}
		}
		event := store.StatusChangeEvent{ChatSessionID: sid, RegistryID: definition.ID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, EventKind: kind, PreviousValueJSON: previous.ValueJSON, NewValueJSON: mustCompactJSON(projection), EvidenceJSON: mustCompactJSON(evidencePayload), SourceTurn: turn, StoryClockJSON: reversibleObservationStoryClockJSON(mapFromAny(observation["observed_at"])), EventState: map[bool]string{true: "recorded", false: "history_only"}[currentAllowed], CreatedAt: now}
		result.Attempted++
		saved, err := atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{SourceContract: source.ContractVersion, SourceRevision: source.Revision, SourceUnitID: sourceUnit, CurrentValue: current, Event: event})
		if errors.Is(err, store.ErrStatusProjectionStale) && currentAllowed {
			// Match the existing reversible owner: an intervening later source
			// changes current projection eligibility, not the observed history.
			evidencePayload["current_projection"] = false
			evidencePayload["resolution_status"] = "concurrent_newer_projection_history_only"
			event.EvidenceJSON = mustCompactJSON(evidencePayload)
			event.EventState = "history_only"
			saved, err = atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{SourceContract: source.ContractVersion, SourceRevision: source.Revision, SourceUnitID: sourceUnit, Event: event})
			currentAllowed = false
		}
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "ApplyReversibleStatusTransition(body_tracking): "+err.Error())
			continue
		}
		if !saved.Replayed {
			result.PhysicalConditions++
			result.NarrativeStateEvents++
			if currentAllowed {
				result.NarrativeCurrentStates++
			}
		}
	}
}

func (s *Server) bodyTrackingCriticContext(ctx context.Context, sid string) map[string]any {
	cfg, err := s.effectiveBodyTrackingConfig(ctx, sid)
	if err != nil || !cfg.CycleTrackingEnabled && !cfg.AutomaticPregnancyEnabled {
		return nil
	}
	values, _ := s.bodyTrackingCurrentValues(ctx, sid)
	clock := reversibleObservationContext(ctx, s.Store, sid)
	characters := []any{}
	for _, character := range cfg.Characters {
		item := map[string]any{"character_id": character.EntityID, "subject_name": character.CharacterName}
		for _, value := range values {
			if value.OwnerID == character.EntityID {
				projection := bodyTrackingTermProjection(character, parseJSONMap(value.ValueJSON))
				item["observed_facts"] = projection["observed_facts"]
				item["current_model_interpretation"] = bodyTrackingModelReading(projection, clock)
			}
		}
		characters = append(characters, item)
	}
	return map[string]any{
		"contract_version": bodyTrackingStateContract, "characters": characters,
		"time_interpretation": "Observed pregnancy confirmations are dated history. When current_model_interpretation says modeled_birth_completed, the configured term has elapsed; do not repeat the old pregnancy as ongoing or fabricate an observed birth or child details.",
		"automatic_targets":   "Apply to female characters established in story/character context, including newly introduced women; no age or human menopause rules. Record established gender as character_deltas:[{name,appearance:{gender:female|male|other,species}}] using the existing character snapshot path, even when the appearance is newly learned rather than changed. Read retained character context, narration and attributed dialogue together; do not require a literal gender declaration. Attribute gender descriptions to their subject, not automatically the speaker. Do not infer gender from a name or speech style alone. body_events may use subject_name for newly introduced characters. Existing characters listed here are automatic targets, not a user-selected allowlist. Do not invent period-start facts from model initialization; preserve explicit species/world body rules.",
		"extraction":          "Optional body_events objects: kind=period_start|pregnancy_confirmed|pregnancy_ended|recovery_started|recovery_ended|conception_exposure, character_id, semantic_event_key, occurred_at (explicit date/range/calendar), evidence_excerpt, scene_scope, visibility, optional knowledge_source. For conception_exposure also record partners:[{entity_id,character_name}] for source-described counterpart(s); use established identity IDs when available, retaining unresolved names. pregnancy_confirmed may include paternity:{status:confirmed|candidates|unknown,candidates:[{entity_id,character_name}],evidence_excerpt}. Confirmed means story-established parentage, not marriage, suspicion or a model association. Preserve multiple candidates; never choose one without source support. Omitted paternity retains the ongoing pregnancy association; explicit unknown corrects it to unknown. Counterparts are optional, never prerequisites for pregnancy calculation. Another person appearing or a later relationship does not change the father of an existing pregnancy. For conception_exposure preserve exposure:{classification:potentially_conceiving|not_potentially_conceiving|unknown,partner_compatibility:compatible|incompatible|unknown,contraception:none|present|unknown,model_profile:unprotected|author_allowed|unknown}; omit unknown fields freely, never invent numeric probabilities. Reuse an existing event key for the same event; a new occurrence gets a new key. Preserve unknown dates and flashback/planned scope. Record observed facts separately from cycle estimates or suspicions. A body fact never creates character knowledge; preserve explicit knowledge evidence through existing perspective lanes. Do not infer pregnancy or symptoms from delay, desire or repetition. Keep ordinary source memory independently.",
	}
}

type bodyTrackingStateRequest struct {
	Action        string         `json:"action"`
	OperationID   string         `json:"operation_id"`
	CharacterID   string         `json:"character_id"`
	Event         map[string]any `json:"event"`
	EventID       int64          `json:"event_id"`
	DryRun        bool           `json:"dry_run"`
	MemoryTargets []string       `json:"memory_targets"`
}

func (s *Server) planBodyTrackingState(ctx context.Context, sid string, request bodyTrackingStateRequest, now time.Time) (adminStateRepairPlan, error) {
	if request.Action == "delete" {
		return s.planBodyTrackingDelete(ctx, sid, request, now)
	}
	if request.Action == "undo" || request.Action == "restore" {
		plan, err := s.planAdminStateRepair(ctx, sid, adminStateRepairEntry{OperationID: request.OperationID, UndoEventID: request.EventID}, now)
		if err == nil && plan.Event.StatusKey != bodyTrackingStatusKey {
			return plan, errors.New("selected event is not a body state correction")
		}
		return plan, err
	}
	plan := adminStateRepairPlan{}
	cfg, err := s.effectiveBodyTrackingConfig(ctx, sid)
	if err != nil {
		return plan, err
	}
	character, found := bodyTrackingCharacter(cfg, map[string]any{"character_id": request.CharacterID})
	if !found {
		return plan, errors.New("select a configured character for this body state correction")
	}
	observation := storyClockJSONMap(request.Event)
	s.bodyTrackingParentageObservation(ctx, sid, observation)
	kind := stringFromMap(observation, "kind")
	switch kind {
	case "period_start", "pregnancy_confirmed", "pregnancy_ended", "recovery_started", "recovery_ended":
	default:
		return plan, errors.New("select the observed body fact to correct")
	}
	values, err := s.bodyTrackingCurrentValues(ctx, sid)
	if err != nil {
		return plan, err
	}
	var previous store.StatusCurrentValue
	for _, current := range values {
		if current.OwnerID == character.EntityID {
			previous = current
			copy := current
			plan.Before = &copy
		}
	}
	recordedTurn, err := s.adminStateRepairTail(ctx, sid)
	if err != nil {
		return plan, err
	}
	if prior := store.StatusCurrentObservationTurn(previous); prior > recordedTurn {
		recordedTurn = prior
	}
	operation := strings.TrimSpace(request.OperationID)
	if operation == "" {
		operation = stableKey("body-author", sid+character.EntityID+now.Format(time.RFC3339Nano))
	}
	unit := "body-author:" + operation
	observation["character_id"], observation["origin_entity_id"] = character.EntityID, extractionFirstNonEmpty(character.OriginEntityID, character.EntityID)
	observation["semantic_event_key"] = unit
	observedAt := reversibleObservationContext(ctx, s.Store, sid)
	observedAt["kind"], observedAt["recorded_turn"] = "author_setting", recordedTurn
	observation["observed_at"] = observedAt
	observation["source"] = map[string]any{"kind": "author_setting", "source_turn": 0, "operation_id": operation, "recorded_turn": recordedTurn, "evidence_excerpt": stringFromMap(observation, "evidence_excerpt")}
	if kind == "pregnancy_confirmed" {
		observation = bodyPregnancyConfirmedTerm(character, observation, parseJSONMap(previous.ValueJSON))
	}
	projection, _ := bodyTrackingObservationProjection(previous, character, observation, recordedTurn, true)
	evidence := map[string]any{
		"contract_version": bodyTrackingStateContract, "repair_contract": store.StateRepairContract,
		"source": "author_setting", "source_contract": store.StateRepairContract, "source_turn": 0, "source_revision": "", "source_unit_id": unit,
		"repair_operation_id": operation, "repair_recorded_turn": recordedTurn, "repair_before": plan.Before, "current_projection": true,
		"repair_source": observation["source"], "history_observation": observation, "subject_entity_id": character.EntityID,
		"origin_entity_id": observation["origin_entity_id"], "evidence_excerpt": stringFromMap(observation, "evidence_excerpt"),
	}
	current := store.StatusCurrentValue{ChatSessionID: sid, RegistryID: previous.RegistryID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, OwnerLabel: character.CharacterName, ValueKind: "object", ValueJSON: mustCompactJSON(projection), EvidenceJSON: mustCompactJSON(evidence), SourceTurn: 0, WriteState: "current", CreatedAt: now, UpdatedAt: now}
	event := store.StatusChangeEvent{ChatSessionID: sid, RegistryID: previous.RegistryID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, EventKind: "author_" + kind, PreviousValueJSON: previous.ValueJSON, NewValueJSON: current.ValueJSON, EvidenceJSON: current.EvidenceJSON, SourceTurn: 0, EventState: "recorded", CreatedAt: now}
	plan.OperationID, plan.After, plan.Event, plan.Source = operation, &current, event, mapFromAny(observation["source"])
	plan.Transition = store.ReversibleStatusTransition{SourceContract: store.StateRepairContract, SourceUnitID: unit, CurrentValue: &current, Event: event}
	return plan, nil
}

func (s *Server) runBodyTrackingState(ctx context.Context, sid string, request bodyTrackingStateRequest) (map[string]any, error) {
	plan, err := s.planBodyTrackingState(ctx, sid, request, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	response := map[string]any{"status": "preview", "operation_id": plan.OperationID, "before": plan.Before, "current": plan.After, "event_id": int64(0), "replayed": false, "memory_changes": plan.Transition.ArtifactChanges}
	if !request.DryRun {
		writer, ok := s.Store.(store.ReversibleStatusTransitionStore)
		if !ok {
			return nil, errors.New("atomic body state writer unavailable")
		}
		transition := plan.Transition
		vectorErrors := []string{}
		if request.Action == "delete" {
			vectorErrors = s.captureBodyRepairVectors(ctx, transition.ArtifactChanges)
		}
		if transition.Event.RegistryID == 0 {
			definition, err := s.ensureBodyTrackingDefinition(ctx, sid, time.Now().UTC())
			if err != nil {
				return nil, err
			}
			transition.Event.RegistryID = definition.ID
			if transition.CurrentValue != nil {
				transition.CurrentValue.RegistryID = definition.ID
			}
		}
		saved, err := writer.ApplyReversibleStatusTransition(ctx, transition)
		if err != nil {
			return nil, err
		}
		response["status"], response["event_id"], response["replayed"] = "applied", saved.Event.ID, saved.Replayed
		if len(sliceFromAny(parseJSONMap(saved.Event.EvidenceJSON)["repair_artifacts"])) > 0 {
			current, projectionErr := s.adminRepairEventIsCurrent(ctx, sid, saved.Event, saved.Replayed)
			if projectionErr != nil {
				vectorErrors = append(vectorErrors, projectionErr.Error())
			} else if current {
				vectorErrors = append(vectorErrors, s.applyBodyRepairVectors(ctx, sid, saved.Event)...)
			}
		}
		if len(vectorErrors) > 0 {
			response["status"], response["projection_errors"] = "partial_error", vectorErrors
		}
		delete(response, "memory_changes")
		response["before"] = parseJSONMap(saved.Event.EvidenceJSON)["repair_before"]
		// A retry after a later correction/undo reports current authority, never
		// reprojects the old operation's after snapshot.
		response["current"] = nil
		values, err := s.bodyTrackingCurrentValues(ctx, sid)
		if err != nil {
			return response, err
		}
		for _, value := range values {
			if value.OwnerID == saved.Event.OwnerID {
				response["current"] = value
				break
			}
		}
	}
	for _, field := range []string{"before", "current"} {
		encoded := parseJSONMap(mustCompactJSON(response[field]))
		response[field+"_state"] = nil
		if raw := stringFromMap(encoded, "value_json"); raw != "" {
			response[field+"_state"] = parseJSONMap(raw)
		}
	}
	return response, nil
}

func (s *Server) handleBodyTrackingState(w http.ResponseWriter, r *http.Request) {
	var request bodyTrackingStateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body_state_correction", "Body state correction must be a JSON object.")
		return
	}
	response, err := s.runBodyTrackingState(r.Context(), strings.TrimSpace(r.PathValue("chat_session_id")), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "body_state_correction_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}
