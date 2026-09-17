package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	relationshipStateContractVersion = "relationship_state.v1"
	relationshipStateStatusKey       = "relationship_state"
	relationshipStateOwnerScope      = "directional_relationship"
)

type relationshipStateCandidate struct {
	unit         *store.PreciseMemoryUnit
	payload      map[string]any
	ownerID      string
	sourceUnitID string
	signature    string
	valueJSON    string
}

// saveRelationshipStatesFromPreciseMemoryUnits is the 3.9-A durable owner for
// directional relationship current/history. It consumes only already-admitted
// 3.8-D relationship observations; atomic interactions and consent/boundary
// units remain independent and cannot create relationship state here.
func (s *Server) saveRelationshipStatesFromPreciseMemoryUnits(
	ctx context.Context,
	sid string,
	units []*store.PreciseMemoryUnit,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil || strings.TrimSpace(sid) == "" {
		return
	}
	candidates := make([]relationshipStateCandidate, 0, len(units))
	seenSourceUnits := map[string]bool{}
	for _, unit := range units {
		candidate, relevant, reason := relationshipStateCandidateFromUnit(sid, unit)
		if !relevant {
			continue
		}
		if reason != "" {
			result.addSkipReason("relationship_states", reason, map[string]any{
				"precise_memory_unit_id": nilIfEmpty(relationshipStatePreciseUnitID(unit)),
			})
			continue
		}
		if seenSourceUnits[candidate.sourceUnitID] {
			continue
		}
		seenSourceUnits[candidate.sourceUnitID] = true
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return
	}
	atomicStore, ok := s.Store.(store.ReversibleStatusTransitionStore)
	if !ok {
		result.addSkipReason("relationship_states", "atomic_status_transition_store_unavailable", nil)
		return
	}
	definition, ok := ensureRelationshipStateDefinition(ctx, s.Store, sid, now, result)
	if !ok {
		return
	}
	currentValues, err := atomicStore.ListReversibleStatusCurrentValues(
		ctx, sid, relationshipStateOwnerScope, []string{relationshipStateStatusKey},
	)
	if err != nil {
		result.addSkipReason("relationship_states", "current_projection_read_failed", err.Error())
		return
	}
	currentByOwner := map[string]store.StatusCurrentValue{}
	for _, current := range currentValues {
		if current.StatusKey == relationshipStateStatusKey && current.OwnerScope == relationshipStateOwnerScope {
			currentByOwner[current.OwnerID] = current
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i].unit.SourceRevision + "\x00" + candidates[i].ownerID + "\x00" + candidates[i].sourceUnitID
		right := candidates[j].unit.SourceRevision + "\x00" + candidates[j].ownerID + "\x00" + candidates[j].sourceUnitID
		return left < right
	})
	groups := map[string][]relationshipStateCandidate{}
	groupOrder := []string{}
	for _, candidate := range candidates {
		key := candidate.unit.SourceRevision + "\x00" + candidate.ownerID
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], candidate)
	}

	for _, groupKey := range groupOrder {
		group := groups[groupKey]
		signatures := map[string]bool{}
		conflictingSourceUnitIDs := make([]string, 0, len(group))
		for _, candidate := range group {
			signatures[candidate.signature] = true
			conflictingSourceUnitIDs = append(conflictingSourceUnitIDs, candidate.sourceUnitID)
		}
		sort.Strings(conflictingSourceUnitIDs)
		sameSourceConflict := len(signatures) > 1
		for candidateIndex, candidate := range group {
			unit := candidate.unit
			if _, err := atomicStore.GetReversibleStatusEventBySourceUnit(
				ctx, sid, unit.SourceRevision, candidate.sourceUnitID,
			); err == nil {
				result.addSkipReason("relationship_states", "source_unit_replay_idempotent", map[string]any{
					"source_unit_id": candidate.sourceUnitID,
				})
				continue
			} else if !errors.Is(err, store.ErrNotFound) {
				result.addSkipReason("relationship_states", "source_unit_lookup_failed", err.Error())
				continue
			}

			previous := currentByOwner[candidate.ownerID]
			currentAllowed := true
			resolutionStatus := "current_projection_applied"
			if sameSourceConflict {
				currentAllowed = false
				resolutionStatus = "same_source_conflict_history_only"
			} else if candidateIndex > 0 {
				currentAllowed = false
				resolutionStatus = "same_source_duplicate_history_only"
			} else if previous.ID != 0 && !relationshipStateCurrentMatchesCandidate(previous, candidate) {
				currentAllowed = false
				resolutionStatus = "malformed_or_mismatched_current_history_only"
			} else if previous.SourceTurn > unit.SourceTurnEnd {
				currentAllowed = false
				resolutionStatus = "older_source_history_only"
			}

			evidencePayload := relationshipStateEvidencePayload(
				candidate, currentAllowed, resolutionStatus, previous, conflictingSourceUnitIDs,
			)
			createdAt := now
			if !previous.CreatedAt.IsZero() {
				createdAt = previous.CreatedAt
			}
			var currentValue *store.StatusCurrentValue
			if currentAllowed {
				currentValue = &store.StatusCurrentValue{
					ChatSessionID: sid,
					RegistryID:    definition.ID,
					StatusKey:     relationshipStateStatusKey,
					OwnerScope:    relationshipStateOwnerScope,
					OwnerID:       candidate.ownerID,
					OwnerLabel:    relationshipStateOwnerLabel(candidate.payload),
					ValueKind:     "object",
					ValueJSON:     candidate.valueJSON,
					EvidenceJSON:  mustCompactJSON(evidencePayload),
					SourceTurn:    unit.SourceTurnEnd,
					WriteState:    "current",
					CreatedAt:     createdAt,
					UpdatedAt:     now,
				}
			}
			event := store.StatusChangeEvent{
				ChatSessionID:     sid,
				RegistryID:        definition.ID,
				StatusKey:         relationshipStateStatusKey,
				OwnerScope:        relationshipStateOwnerScope,
				OwnerID:           candidate.ownerID,
				EventKind:         relationshipStateEventKind(previous, candidate, sameSourceConflict),
				PreviousValueJSON: previous.ValueJSON,
				NewValueJSON:      candidate.valueJSON,
				EvidenceJSON:      mustCompactJSON(evidencePayload),
				SourceTurn:        unit.SourceTurnEnd,
				EventState:        map[bool]string{true: "recorded", false: "history_only"}[currentAllowed],
				CreatedAt:         now,
			}
			result.Attempted++
			saved, err := atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
				SourceContract: unit.SourceContract,
				SourceRevision: unit.SourceRevision,
				SourceUnitID:   candidate.sourceUnitID,
				CurrentValue:   currentValue,
				Event:          event,
			})
			if errors.Is(err, store.ErrStatusProjectionStale) && currentAllowed {
				currentAllowed = false
				evidencePayload["current_projection"] = false
				evidencePayload["resolution_status"] = "concurrent_newer_projection_history_only"
				event.EvidenceJSON = mustCompactJSON(evidencePayload)
				event.EventState = "history_only"
				saved, err = atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
					SourceContract: unit.SourceContract,
					SourceRevision: unit.SourceRevision,
					SourceUnitID:   candidate.sourceUnitID,
					Event:          event,
				})
			}
			if err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, "ApplyRelationshipStateTransition: "+err.Error())
				continue
			}
			if saved.Replayed {
				result.addSkipReason("relationship_states", "source_unit_replay_idempotent", map[string]any{
					"source_unit_id": candidate.sourceUnitID,
				})
				continue
			}
			result.RelationStateEvents++
			if currentAllowed {
				result.RelationCurrentStates++
				currentByOwner[candidate.ownerID] = saved.CurrentValue
			}
		}
	}
}

func relationshipStateCandidateFromUnit(sid string, unit *store.PreciseMemoryUnit) (relationshipStateCandidate, bool, string) {
	if unit == nil || unit.Kind != "observation" || !strings.HasPrefix(unit.Subtype, "relationship_") {
		return relationshipStateCandidate{}, false, ""
	}
	payload := map[string]any{}
	if json.Unmarshal([]byte(unit.PayloadJSON), &payload) != nil {
		return relationshipStateCandidate{}, true, "relationship_payload_invalid"
	}
	if extractionStringFromAny(payload["contract_version"]) != relationshipObservationContract {
		return relationshipStateCandidate{}, true, "relationship_observation_contract_required"
	}
	domain := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["domain"])))
	sourceLabel := strings.TrimSpace(extractionStringFromAny(payload["source_entity"]))
	targetLabel := strings.TrimSpace(extractionStringFromAny(payload["target_entity"]))
	observation := strings.TrimSpace(extractionStringFromAny(payload["observation"]))
	supportKind := strings.TrimSpace(extractionStringFromAny(payload["support_kind"]))
	sourceID := strings.TrimSpace(unit.ActorEntityID)
	targetID := strings.TrimSpace(unit.AffectedEntityID)
	if unit.ContractVersion != store.PreciseMemoryUnitContract ||
		unit.ChatSessionID != sid ||
		unit.SourceContract != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(unit.SourceRevision) == "" ||
		unit.SourceTurnStart <= 0 || unit.SourceTurnEnd < unit.SourceTurnStart ||
		strings.TrimSpace(unit.UnitID) == "" ||
		strings.TrimSpace(unit.SourceContentHash) == "" ||
		unit.LifecycleState != "active" {
		return relationshipStateCandidate{}, true, "active_accepted_source_required"
	}
	if unit.TruthScope != "source_scoped" || unit.EpistemicMode != "direct" ||
		unit.AuthorityClass != "subjective_episodic" {
		return relationshipStateCandidate{}, true, "source_scoped_relationship_required"
	}
	if sourceID == "" || targetID == "" || sourceID == targetID || sourceLabel == "" || targetLabel == "" {
		return relationshipStateCandidate{}, true, "stable_directional_entity_ids_required"
	}
	expectedRelationshipKey := comparableEntityKey(sourceLabel) + "->" + comparableEntityKey(targetLabel) + "/" + domain
	if strings.TrimSpace(unit.RelationshipKey) != expectedRelationshipKey {
		return relationshipStateCandidate{}, true, "directional_relationship_key_mismatch"
	}
	if unit.Subtype != "relationship_"+domain || observation == "" {
		return relationshipStateCandidate{}, true, "relationship_observation_required"
	}
	visibility := strings.ToLower(strings.TrimSpace(unit.Visibility))
	if !map[string]bool{"public": true, "owner_private": true, "restricted": true, "user_private": true}[visibility] {
		return relationshipStateCandidate{}, true, "valid_visibility_required"
	}
	directEvidenceIDs := jsonInt64Slice(unit.DirectEvidenceIDsJSON)
	expressionScope := relationshipStateExpressionScope(unit.Visibility)
	ownerID := relationshipStateOwnerID(sourceID, targetID, domain, expressionScope)
	sourceUnitID := relationshipStateSourceUnitID(unit.UnitID)
	projection := relationshipStateProjection(unit, payload, sourceUnitID, directEvidenceIDs, expressionScope)
	signaturePayload := map[string]any{
		"observation":  observation,
		"support_kind": supportKind,
		"visibility":   unit.Visibility,
	}
	for _, key := range []string{"magnitude", "duration"} {
		if value, exists := payload[key]; exists {
			signaturePayload[key] = normalizePreciseMemoryValue(value)
		}
	}
	return relationshipStateCandidate{
		unit:         unit,
		payload:      payload,
		ownerID:      ownerID,
		sourceUnitID: sourceUnitID,
		signature:    mustCompactJSON(signaturePayload),
		valueJSON:    mustCompactJSON(projection),
	}, true, ""
}

func relationshipStateProjection(
	unit *store.PreciseMemoryUnit,
	payload map[string]any,
	sourceUnitID string,
	directEvidenceIDs []int64,
	expressionScope string,
) map[string]any {
	claim := map[string]any{
		"observation":  extractionStringFromAny(payload["observation"]),
		"support_kind": extractionStringFromAny(payload["support_kind"]),
		"visibility":   unit.Visibility,
	}
	for _, key := range []string{"magnitude", "duration"} {
		if value, exists := payload[key]; exists {
			claim[key] = normalizePreciseMemoryValue(value)
		}
	}
	return map[string]any{
		"version":          relationshipStateContractVersion,
		"source_entity_id": unit.ActorEntityID,
		"source_label":     payload["source_entity"],
		"target_entity_id": unit.AffectedEntityID,
		"target_label":     payload["target_entity"],
		"direction": map[string]any{
			"from_entity_id": unit.ActorEntityID,
			"to_entity_id":   unit.AffectedEntityID,
		},
		"domain":           payload["domain"],
		"expression_scope": expressionScope,
		"current":          claim,
		"uncertainty": map[string]any{
			"state": "not_asserted",
		},
		"counterevidence": map[string]any{
			"state": "not_asserted",
			"refs":  []any{},
		},
		"reciprocity": map[string]any{
			"state": "not_inferred",
		},
		"consent": map[string]any{
			"state":              "not_inferred",
			"authority_contract": interactionBoundaryContract,
		},
		"stability": map[string]any{
			"state": "not_inferred",
		},
		"validity": map[string]any{
			"source_turn_start": unit.SourceTurnStart,
			"source_turn_end":   unit.SourceTurnEnd,
			"source_revision":   unit.SourceRevision,
			"lifecycle_state":   unit.LifecycleState,
		},
		"source": map[string]any{
			"source_contract":              unit.SourceContract,
			"source_revision":              unit.SourceRevision,
			"relationship_state_source_id": sourceUnitID,
			"precise_memory_unit_id":       unit.UnitID,
			"logical_turn_id":              nilIfEmpty(unit.SourceLogicalTurnID),
			"message_id":                   nilIfEmpty(unit.SourceMessageID),
			"generation_id":                nilIfEmpty(unit.SourceGenerationID),
			"content_hash":                 unit.SourceContentHash,
			"root_evidence_id":             unit.RootEvidenceID,
			"direct_evidence_ids":          directEvidenceIDs,
			"evidence_hash":                unit.EvidenceHash,
		},
		"branch": map[string]any{
			"scope":         "accepted_source_revision",
			"revision":      unit.SourceRevision,
			"generation_id": nilIfEmpty(unit.SourceGenerationID),
		},
		"history_owner": "status_change_events",
	}
}

func relationshipStateEvidencePayload(
	candidate relationshipStateCandidate,
	currentProjection bool,
	resolutionStatus string,
	previous store.StatusCurrentValue,
	conflictingSourceUnitIDs []string,
) map[string]any {
	unit := candidate.unit
	evidence := map[string]any{
		"contract_version":       relationshipStateContractVersion,
		"source":                 "precise_memory_unit.relationship_observation.v1",
		"source_revision":        unit.SourceRevision,
		"source_unit_id":         candidate.sourceUnitID,
		"precise_memory_unit_id": unit.UnitID,
		"source_turn_start":      unit.SourceTurnStart,
		"source_turn_end":        unit.SourceTurnEnd,
		"logical_turn_id":        nilIfEmpty(unit.SourceLogicalTurnID),
		"source_message_id":      nilIfEmpty(unit.SourceMessageID),
		"source_generation_id":   nilIfEmpty(unit.SourceGenerationID),
		"content_hash":           unit.SourceContentHash,
		"root_evidence_id":       unit.RootEvidenceID,
		"direct_evidence_ids":    jsonInt64Slice(unit.DirectEvidenceIDsJSON),
		"evidence_excerpt":       unit.EvidenceExcerpt,
		"current_projection":     currentProjection,
		"resolution_status":      resolutionStatus,
		"relationship_observation": map[string]any{
			"source_entity": candidate.payload["source_entity"],
			"target_entity": candidate.payload["target_entity"],
			"domain":        candidate.payload["domain"],
			"observation":   candidate.payload["observation"],
			"support_kind":  candidate.payload["support_kind"],
			"visibility":    unit.Visibility,
		},
		"reciprocity_state":          "not_inferred",
		"consent_authority_contract": interactionBoundaryContract,
		"counterevidence_refs":       []any{},
	}
	if len(conflictingSourceUnitIDs) > 1 {
		evidence["same_source_unit_ids"] = conflictingSourceUnitIDs
	}
	if previous.ID != 0 {
		prior := map[string]any{}
		_ = json.Unmarshal([]byte(previous.EvidenceJSON), &prior)
		evidence["previous_current_ref"] = map[string]any{
			"source_revision": prior["source_revision"],
			"source_unit_id":  prior["source_unit_id"],
			"source_turn":     previous.SourceTurn,
		}
	}
	return evidence
}

func ensureRelationshipStateDefinition(
	ctx context.Context,
	st store.Store,
	sid string,
	now time.Time,
	result *artifactSaveResult,
) (store.StatusSchemaDefinition, bool) {
	registry, ok := st.(store.StatusSchemaRegistryStore)
	if !ok {
		result.addSkipReason("relationship_states", "status_schema_registry_unavailable", nil)
		return store.StatusSchemaDefinition{}, false
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(
		ctx, sid, relationshipStateStatusKey, relationshipStateOwnerScope,
	)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("relationship_states", "status_schema_lookup_failed", err.Error())
		return store.StatusSchemaDefinition{}, false
	}
	result.Attempted++
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid,
		SchemaName:    "directional_relationship_state",
		StatusKey:     relationshipStateStatusKey,
		Label:         "Directional relationship state",
		OwnerScope:    relationshipStateOwnerScope,
		ValueKind:     "object",
		OptionsJSON: mustCompactJSON(map[string]any{
			"contract_version":              relationshipStateContractVersion,
			"directional":                   true,
			"domain_separated":              true,
			"expression_scope_separated":    true,
			"current_history_separated":     true,
			"reciprocity_inference":         false,
			"consent_authority_contract":    interactionBoundaryContract,
			"single_relationship_score":     false,
			"user_or_player_priority":       false,
			"admitted_observation_contract": relationshipObservationContract,
			"domain_vocabulary":             "open",
		}),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil || len(definitions) == 0 {
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusSchemaDefinitions(relationship_state): "+err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	result.StatusSchemaDefinitions++
	return definitions[0], true
}

func relationshipStateCurrentMatchesCandidate(current store.StatusCurrentValue, candidate relationshipStateCandidate) bool {
	projection := map[string]any{}
	if json.Unmarshal([]byte(current.ValueJSON), &projection) != nil {
		return false
	}
	return extractionStringFromAny(projection["version"]) == relationshipStateContractVersion &&
		extractionStringFromAny(projection["source_entity_id"]) == candidate.unit.ActorEntityID &&
		extractionStringFromAny(projection["target_entity_id"]) == candidate.unit.AffectedEntityID &&
		extractionStringFromAny(projection["domain"]) == extractionStringFromAny(candidate.payload["domain"]) &&
		extractionStringFromAny(projection["expression_scope"]) == relationshipStateExpressionScope(candidate.unit.Visibility)
}

func relationshipStateEventKind(previous store.StatusCurrentValue, candidate relationshipStateCandidate, conflict bool) string {
	if conflict {
		return "observed_conflict"
	}
	if previous.ID == 0 {
		return "observed_set"
	}
	previousProjection := map[string]any{}
	if json.Unmarshal([]byte(previous.ValueJSON), &previousProjection) == nil {
		previousClaim := mapFromAny(previousProjection["current"])
		if normalizeArtifactComparableText(extractionStringFromAny(previousClaim["observation"])) ==
			normalizeArtifactComparableText(extractionStringFromAny(candidate.payload["observation"])) {
			return "observed_reaffirmation"
		}
	}
	return "observed_change"
}

func relationshipStateOwnerID(sourceEntityID, targetEntityID, domain, expressionScope string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		relationshipStateContractVersion,
		strings.TrimSpace(sourceEntityID),
		strings.TrimSpace(targetEntityID),
		strings.TrimSpace(domain),
		strings.TrimSpace(expressionScope),
	}, "\x1f")))
	return "relationship:" + hex.EncodeToString(sum[:])
}

func relationshipStateSourceUnitID(preciseMemoryUnitID string) string {
	sum := sha256.Sum256([]byte(relationshipStateContractVersion + "\x1f" + strings.TrimSpace(preciseMemoryUnitID)))
	return "relationship-state:" + hex.EncodeToString(sum[:])
}

func relationshipStateExpressionScope(visibility string) string {
	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case "public":
		return "public"
	case "owner_private":
		return "private_owner"
	case "user_private":
		return "private_user"
	default:
		return "restricted"
	}
}

func relationshipStateOwnerLabel(payload map[string]any) string {
	return fmt.Sprintf(
		"%s -> %s [%s]",
		extractionStringFromAny(payload["source_entity"]),
		extractionStringFromAny(payload["target_entity"]),
		extractionStringFromAny(payload["domain"]),
	)
}

func relationshipStatePreciseUnitID(unit *store.PreciseMemoryUnit) string {
	if unit == nil {
		return ""
	}
	return unit.UnitID
}

func jsonInt64Slice(raw string) []int64 {
	values := []int64{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}
