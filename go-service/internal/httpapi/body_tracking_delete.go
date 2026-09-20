package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) planBodyTrackingDelete(ctx context.Context, sid string, request bodyTrackingStateRequest, now time.Time) (adminStateRepairPlan, error) {
	plan := adminStateRepairPlan{}
	cfg, err := s.loadBodyTrackingConfig(sid)
	if err != nil {
		return plan, err
	}
	character, found := bodyTrackingCharacter(cfg, map[string]any{"character_id": request.CharacterID})
	if !found {
		for _, person := range s.bodyTrackingRoster(ctx, sid, cfg) {
			if stringFromMap(person, "entity_id") == request.CharacterID {
				character = defaultBodyCharacterConfig()
				character.EntityID, character.CharacterName = request.CharacterID, stringFromMap(person, "character_name")
				found = true
				break
			}
		}
	}
	if !found {
		return plan, fmt.Errorf("body data character not found")
	}
	values, err := s.bodyTrackingCurrentValues(ctx, sid)
	if err != nil {
		return plan, err
	}
	var previous store.StatusCurrentValue
	for _, value := range values {
		if value.OwnerID == character.EntityID {
			previous = value
			copy := value
			plan.Before = &copy
		}
	}
	recorded, err := s.adminStateRepairTail(ctx, sid)
	if err != nil {
		return plan, err
	}
	if prior := store.StatusCurrentObservationTurn(previous); prior > recorded {
		recorded = prior
	}
	operation := extractionFirstNonEmpty(request.OperationID, stableKey("body-delete", sid+character.EntityID+now.Format(time.RFC3339Nano)))
	unit := "body-delete:" + operation
	projection := map[string]any{"contract_version": bodyTrackingStateContract, "subject_entity_id": character.EntityID, "subject_label": character.CharacterName, "deleted": true}
	evidence := map[string]any{"contract_version": bodyTrackingStateContract, "repair_contract": store.StateRepairContract,
		"source_contract": store.StateRepairContract, "source_unit_id": unit, "source_revision": "", "source_turn": 0,
		"repair_operation_id": operation, "repair_recorded_turn": recorded, "repair_before": plan.Before, "current_projection": true,
		"subject_entity_id": character.EntityID, "source": "author_body_delete", "repair_source": map[string]any{"kind": "author_body_delete"}}
	current := store.StatusCurrentValue{ChatSessionID: sid, RegistryID: previous.RegistryID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, OwnerLabel: character.CharacterName, ValueKind: "object", ValueJSON: mustCompactJSON(projection), EvidenceJSON: mustCompactJSON(evidence), WriteState: "current", CreatedAt: now, UpdatedAt: now}
	event := store.StatusChangeEvent{ChatSessionID: sid, RegistryID: previous.RegistryID, StatusKey: bodyTrackingStatusKey, OwnerScope: reversibleStateOwnerScope, OwnerID: character.EntityID, EventKind: "author_body_delete", PreviousValueJSON: previous.ValueJSON, NewValueJSON: current.ValueJSON, EvidenceJSON: current.EvidenceJSON, EventState: "recorded", CreatedAt: now}
	plan.OperationID, plan.After, plan.Event = operation, &current, event
	plan.Transition = store.ReversibleStatusTransition{SourceContract: store.StateRepairContract, SourceUnitID: unit, CurrentValue: &current, Event: event}
	if reader, ok := s.Store.(store.StateRepairArtifactReader); ok {
		changes, err := reader.ListBodyRepairArtifacts(ctx, sid, character.EntityID, character.CharacterName)
		if err != nil {
			return plan, err
		}
		selected := map[string]bool{}
		for _, key := range request.MemoryTargets {
			selected[key] = true
		}
		for _, change := range changes {
			if request.MemoryTargets == nil || selected[fmt.Sprintf("%s:%d", change.Table, change.ID)] {
				plan.Transition.ArtifactChanges = append(plan.Transition.ArtifactChanges, change)
			}
		}
	}
	return plan, nil
}

func (s *Server) captureBodyRepairVectors(ctx context.Context, changes []store.StateRepairArtifactChange) []string {
	errors := []string{}
	for i := range changes {
		change := &changes[i]
		if len(change.VectorIDs) == 0 {
			continue
		}
		if s.Vector == nil {
			change.VectorBeforeJSON = "[]"
			continue
		}
		reader, ok := s.Vector.(vector.ExactDocumentReader)
		if !ok {
			errors = append(errors, "vector snapshot reader unavailable")
			continue
		}
		docs, err := reader.GetDocuments(ctx, change.VectorIDs)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		change.VectorBeforeJSON = mustCompactJSON(docs)
	}
	return errors
}

// Existing outbox operations own source-bound vectors. Pre-outbox projections
// use the existing exact vector mutation API and the same repair retry identity.
func (s *Server) applyBodyRepairVectors(ctx context.Context, sid string, event store.StatusChangeEvent) []string {
	errors := []string{}
	var changes []store.StateRepairArtifactChange
	if err := json.Unmarshal([]byte(mustCompactJSON(parseJSONMap(event.EvidenceJSON)["repair_artifacts"])), &changes); err != nil {
		return []string{err.Error()}
	}
	for _, change := range changes {
		if len(change.VectorIDs) == 0 || s.Vector == nil {
			continue
		}
		if change.VectorAfterJSON == "" {
			errors = append(errors, "vector backup unavailable for "+change.Table)
			continue
		}
		var docs []vector.VectorDocument
		if err := json.Unmarshal([]byte(change.VectorAfterJSON), &docs); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if len(docs) == 0 {
			deleter, ok := s.Vector.(vector.DocumentDeleter)
			if !ok {
				errors = append(errors, "vector delete unavailable")
				continue
			}
			if err := deleter.DeleteDocuments(ctx, change.VectorIDs); err != nil {
				errors = append(errors, err.Error())
			}
			continue
		}
		for i := range docs {
			// A branch restores into its own remapped row, never the origin index.
			legacy := docs[i].ID == docs[i].Tier+":"+docs[i].SourceRowID
			docs[i].ID = change.VectorIDs[0]
			if legacy && len(change.VectorIDs) > 1 {
				docs[i].ID = change.VectorIDs[1]
			}
			docs[i].ChatSessionID = sid
			docs[i].SourceRowID = strconv.FormatInt(change.ID, 10)
			if docs[i].Tier == "precise_memory" {
				docs[i].SourceRowID = strings.TrimPrefix(change.VectorIDs[0], "precise_memory:"+sid+":")
			}
			if docs[i].Metadata != nil {
				docs[i].Metadata["chat_session_id"], docs[i].Metadata["source_row_id"] = sid, docs[i].SourceRowID
			}
		}
		if err := s.Vector.Upsert(ctx, sid, docs); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return errors
}

func (s *Server) bodyTrackingDataView(ctx context.Context, sid string, cfg bodyTrackingConfig, values []store.StatusCurrentValue) map[string]any {
	byID := map[string]store.StatusCurrentValue{}
	for _, value := range values {
		byID[value.OwnerID] = value
	}
	characters := []map[string]any{}
	for _, person := range s.bodyTrackingRoster(ctx, sid, cfg) {
		id := stringFromMap(person, "entity_id")
		person["deleted"] = parseJSONMap(byID[id].ValueJSON)["deleted"] == true
		characters = append(characters, person)
	}
	backups := []map[string]any{}
	if reader, ok := s.Store.(store.StatusLifecycleStore); ok {
		if events, err := reader.ListStatusChangeEvents(ctx, sid, reversibleStateOwnerScope, "", bodyTrackingStatusKey, -1); err == nil {
			for _, event := range events {
				if event.EventKind != "author_body_delete" {
					continue
				}
				value := parseJSONMap(event.NewValueJSON)
				backups = append(backups, map[string]any{"event_id": event.ID, "character_id": event.OwnerID, "character_name": value["subject_label"], "created_at": event.CreatedAt, "memory_count": len(sliceFromAny(parseJSONMap(event.EvidenceJSON)["repair_artifacts"]))})
			}
		}
	}
	return map[string]any{"characters": characters, "backups": backups}
}
