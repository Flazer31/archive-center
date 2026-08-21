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
	habitEvidenceContractVersion = "habit_evidence.v1"
	habitEvidenceStatusKey       = "habit_evidence"
	habitEvidenceOwnerScope      = "character_habit_evidence"
)

type habitEvidenceCandidate struct {
	unit                *store.PreciseMemoryUnit
	payload             map[string]any
	ownerID             string
	sourceUnitID        string
	evidenceFingerprint string
	expressionScope     string
}

// savePostAdmissionPreciseMemoryProjections keeps all durable projections
// downstream of the one admitted precise-memory source. Each projection still
// enforces its own authority and source contract.
func (s *Server) savePostAdmissionPreciseMemoryProjections(
	ctx context.Context,
	sid string,
	units []*store.PreciseMemoryUnit,
	now time.Time,
	result *artifactSaveResult,
) {
	s.saveRelationshipStatesFromPreciseMemoryUnits(ctx, sid, units, now, result)
	s.saveHabitEvidenceFromPreciseMemoryUnits(ctx, sid, units, now, result)
	s.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(ctx, sid, units, now, result)
}

// saveHabitEvidenceFromPreciseMemoryUnits is the 3.9-B support-only owner. It
// accumulates source occurrences, explicit pattern statements, counterexamples,
// and exceptions without deciding that a stable habit or personality exists.
func (s *Server) saveHabitEvidenceFromPreciseMemoryUnits(
	ctx context.Context,
	sid string,
	units []*store.PreciseMemoryUnit,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil || strings.TrimSpace(sid) == "" {
		return
	}
	candidates := make([]habitEvidenceCandidate, 0, len(units))
	seenSourceUnits := map[string]bool{}
	for _, unit := range units {
		candidate, relevant, reason := habitEvidenceCandidateFromUnit(sid, unit)
		if !relevant {
			continue
		}
		if reason != "" {
			result.addSkipReason("habit_evidence", reason, map[string]any{
				"precise_memory_unit_id": nilIfEmpty(habitEvidencePreciseUnitID(unit)),
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
		result.addSkipReason("habit_evidence", "atomic_status_transition_store_unavailable", nil)
		return
	}
	definition, ok := ensureHabitEvidenceDefinition(ctx, s.Store, sid, now, result)
	if !ok {
		return
	}
	currentValues, err := atomicStore.ListReversibleStatusCurrentValues(
		ctx, sid, habitEvidenceOwnerScope, []string{habitEvidenceStatusKey},
	)
	if err != nil {
		result.addSkipReason("habit_evidence", "current_projection_read_failed", err.Error())
		return
	}
	currentByOwner := map[string]store.StatusCurrentValue{}
	for _, current := range currentValues {
		if current.StatusKey == habitEvidenceStatusKey && current.OwnerScope == habitEvidenceOwnerScope {
			currentByOwner[current.OwnerID] = current
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.unit.SourceTurnEnd != right.unit.SourceTurnEnd {
			return left.unit.SourceTurnEnd < right.unit.SourceTurnEnd
		}
		if left.unit.SourceRevision != right.unit.SourceRevision {
			return left.unit.SourceRevision < right.unit.SourceRevision
		}
		if left.ownerID != right.ownerID {
			return left.ownerID < right.ownerID
		}
		leftPriority := habitEvidenceObservationSortPriority(extractionStringFromAny(left.payload["observation_kind"]))
		rightPriority := habitEvidenceObservationSortPriority(extractionStringFromAny(right.payload["observation_kind"]))
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return left.sourceUnitID < right.sourceUnitID
	})

	for _, candidate := range candidates {
		unit := candidate.unit
		if _, err := atomicStore.GetReversibleStatusEventBySourceUnit(
			ctx, sid, unit.SourceRevision, candidate.sourceUnitID,
		); err == nil {
			result.addSkipReason("habit_evidence", "source_unit_replay_idempotent", map[string]any{
				"source_unit_id": candidate.sourceUnitID,
			})
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			result.addSkipReason("habit_evidence", "source_unit_lookup_failed", err.Error())
			continue
		}

		previous := currentByOwner[candidate.ownerID]
		currentAllowed := true
		resolutionStatus := "evidence_aggregate_applied"
		projection := map[string]any{}
		if previous.ID == 0 {
			projection = habitEvidenceInitialProjection(candidate)
		} else if !habitEvidenceCurrentMatchesCandidate(previous, candidate) {
			currentAllowed = false
			resolutionStatus = "malformed_or_mismatched_current_history_only"
			projection = habitEvidenceInitialProjection(candidate)
		} else if previous.SourceTurn > unit.SourceTurnEnd {
			currentAllowed = false
			resolutionStatus = "older_source_history_only"
			projection = habitEvidenceInitialProjection(candidate)
		} else {
			_ = json.Unmarshal([]byte(previous.ValueJSON), &projection)
			if stringSliceContains(stringsFromAny(projection["evidence_fingerprints"]), candidate.evidenceFingerprint) {
				result.addSkipReason("habit_evidence", "root_evidence_replay_idempotent", map[string]any{
					"root_evidence_id": unit.RootEvidenceID,
					"observation_kind": extractionStringFromAny(candidate.payload["observation_kind"]),
				})
				continue
			}
			habitEvidenceMergeProjection(projection, candidate)
		}

		valueJSON := mustCompactJSON(projection)
		evidencePayload := habitEvidenceEventEvidence(candidate, currentAllowed, resolutionStatus, previous)
		createdAt := now
		if !previous.CreatedAt.IsZero() {
			createdAt = previous.CreatedAt
		}
		var currentValue *store.StatusCurrentValue
		if currentAllowed {
			currentValue = &store.StatusCurrentValue{
				ChatSessionID: sid,
				RegistryID:    definition.ID,
				StatusKey:     habitEvidenceStatusKey,
				OwnerScope:    habitEvidenceOwnerScope,
				OwnerID:       candidate.ownerID,
				OwnerLabel:    habitEvidenceOwnerLabel(candidate.payload),
				ValueKind:     "object",
				ValueJSON:     valueJSON,
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
			StatusKey:         habitEvidenceStatusKey,
			OwnerScope:        habitEvidenceOwnerScope,
			OwnerID:           candidate.ownerID,
			EventKind:         "habit_" + extractionStringFromAny(candidate.payload["observation_kind"]) + "_observed",
			PreviousValueJSON: previous.ValueJSON,
			NewValueJSON:      valueJSON,
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
			result.ErrorDetails = append(result.ErrorDetails, "ApplyHabitEvidenceTransition: "+err.Error())
			continue
		}
		if saved.Replayed {
			result.addSkipReason("habit_evidence", "source_unit_replay_idempotent", map[string]any{
				"source_unit_id": candidate.sourceUnitID,
			})
			continue
		}
		result.HabitEvidenceEvents++
		if currentAllowed {
			result.HabitEvidenceCurrent++
			currentByOwner[candidate.ownerID] = saved.CurrentValue
		}
	}
}

func habitEvidenceCandidateFromUnit(sid string, unit *store.PreciseMemoryUnit) (habitEvidenceCandidate, bool, string) {
	if unit == nil || unit.Kind != "observation" || unit.Subtype != "habit_observation" {
		return habitEvidenceCandidate{}, false, ""
	}
	payload := map[string]any{}
	if json.Unmarshal([]byte(unit.PayloadJSON), &payload) != nil {
		return habitEvidenceCandidate{}, true, "habit_payload_invalid"
	}
	if extractionStringFromAny(payload["contract_version"]) != habitObservationContract {
		return habitEvidenceCandidate{}, true, "habit_observation_contract_required"
	}
	subjectLabel := strings.TrimSpace(extractionStringFromAny(payload["subject_entity"]))
	behaviorKey := strings.TrimSpace(extractionStringFromAny(payload["behavior_key"]))
	observationKind := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["observation_kind"])))
	if unit.ContractVersion != store.PreciseMemoryUnitContract ||
		unit.ChatSessionID != sid || unit.SourceContract != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(unit.SourceRevision) == "" || unit.SourceTurnStart <= 0 || unit.SourceTurnEnd < unit.SourceTurnStart ||
		strings.TrimSpace(unit.UnitID) == "" || strings.TrimSpace(unit.SourceContentHash) == "" || unit.LifecycleState != "active" {
		return habitEvidenceCandidate{}, true, "active_accepted_source_required"
	}
	if unit.TruthScope != "support_only" || unit.EpistemicMode != "direct" || unit.AuthorityClass != "support_hypothesis" {
		return habitEvidenceCandidate{}, true, "support_only_habit_observation_required"
	}
	if strings.TrimSpace(unit.SubjectEntityID) == "" || subjectLabel == "" || behaviorKey == "" {
		return habitEvidenceCandidate{}, true, "stable_subject_and_behavior_required"
	}
	visibility := strings.ToLower(strings.TrimSpace(unit.Visibility))
	if !map[string]bool{"public": true, "owner_private": true, "restricted": true, "user_private": true}[visibility] {
		return habitEvidenceCandidate{}, true, "valid_visibility_required"
	}
	expressionScope := relationshipStateExpressionScope(unit.Visibility)
	return habitEvidenceCandidate{
		unit:                unit,
		payload:             payload,
		ownerID:             habitEvidenceOwnerID(unit.SubjectEntityID, behaviorKey, expressionScope),
		sourceUnitID:        habitEvidenceSourceUnitID(unit.UnitID),
		evidenceFingerprint: habitEvidenceRootFingerprint(unit, observationKind),
		expressionScope:     expressionScope,
	}, true, ""
}

func habitEvidenceInitialProjection(candidate habitEvidenceCandidate) map[string]any {
	unit := candidate.unit
	projection := map[string]any{
		"version":                     habitEvidenceContractVersion,
		"subject_entity_id":           unit.SubjectEntityID,
		"subject_label":               candidate.payload["subject_entity"],
		"behavior_key":                candidate.payload["behavior_key"],
		"expression_scope":            candidate.expressionScope,
		"authority_class":             "support_hypothesis",
		"disposition":                 "evidence_only",
		"promotion_state":             "not_evaluated",
		"evidence_independence":       "not_evaluated",
		"repeated_support_observed":   false,
		"interpretation_state":        "evidence_only",
		"evidence_counts":             map[string]any{"occurrence": 0, "explicit_pattern": 0, "counterexample": 0, "exception": 0, "support_total": 0, "total": 0},
		"contexts":                    []any{},
		"counterparts":                []any{},
		"source_turns":                []any{},
		"evidence_fingerprints":       []string{},
		"history_owner":               "status_change_events",
		"fixed_count_promotion":       false,
		"fixed_score_promotion":       false,
		"quiet_turn_dormancy":         false,
		"speech_style_projection":     false,
		"objective_personality_write": false,
	}
	habitEvidenceMergeProjection(projection, candidate)
	return projection
}

func habitEvidenceMergeProjection(projection map[string]any, candidate habitEvidenceCandidate) {
	kind := extractionStringFromAny(candidate.payload["observation_kind"])
	counts := mapFromAny(projection["evidence_counts"])
	counts[kind] = intFromAny(counts[kind], 0) + 1
	counts["support_total"] = intFromAny(counts["occurrence"], 0) + intFromAny(counts["explicit_pattern"], 0)
	counts["total"] = intFromAny(counts["support_total"], 0) + intFromAny(counts["counterexample"], 0) + intFromAny(counts["exception"], 0)
	projection["evidence_counts"] = counts
	projection["evidence_fingerprints"] = append(stringsFromAny(projection["evidence_fingerprints"]), candidate.evidenceFingerprint)
	projection["source_turns"] = habitEvidenceAppendUniqueTurn(projection["source_turns"], candidate.unit.SourceTurnEnd)
	projection["contexts"] = habitEvidenceAppendUniqueContext(projection["contexts"], candidate.payload)
	projection["counterparts"] = habitEvidenceAppendUniqueCounterpart(projection["counterparts"], candidate)
	projection["repeated_support_observed"] = intFromAny(counts["support_total"], 0) > 1
	projection["interpretation_state"] = habitEvidenceInterpretation(counts)
	projection["time_distribution"] = habitEvidenceTimeDistribution(projection["source_turns"])
	projection["latest_observation"] = map[string]any{
		"observation_kind":    kind,
		"behavior_expression": candidate.payload["behavior_expression"],
		"context_key":         candidate.payload["context_key"],
		"context_expression":  candidate.payload["context_expression"],
		"counterpart":         candidate.payload["counterpart"],
		"source_turn":         candidate.unit.SourceTurnEnd,
		"source_revision":     candidate.unit.SourceRevision,
		"source_unit_id":      candidate.sourceUnitID,
	}
}

func habitEvidenceAppendUniqueTurn(raw any, turn int) []any {
	turns := []int{}
	seen := map[int]bool{}
	for _, value := range sliceFromAny(raw) {
		current := intFromAny(value, 0)
		if current > 0 && !seen[current] {
			seen[current] = true
			turns = append(turns, current)
		}
	}
	if turn > 0 && !seen[turn] {
		turns = append(turns, turn)
	}
	sort.Ints(turns)
	out := make([]any, 0, len(turns))
	for _, current := range turns {
		out = append(out, current)
	}
	return out
}

func habitEvidenceAppendUniqueContext(raw any, payload map[string]any) []any {
	items := append([]any{}, sliceFromAny(raw)...)
	key := strings.TrimSpace(extractionStringFromAny(payload["context_key"]))
	expression := strings.TrimSpace(extractionStringFromAny(payload["context_expression"]))
	if key == "" || expression == "" {
		return items
	}
	for _, rawItem := range items {
		item := mapFromAny(rawItem)
		if extractionStringFromAny(item["context_key"]) == key && extractionStringFromAny(item["context_expression"]) == expression {
			return items
		}
	}
	return append(items, map[string]any{"context_key": key, "context_expression": expression})
}

func habitEvidenceAppendUniqueCounterpart(raw any, candidate habitEvidenceCandidate) []any {
	items := append([]any{}, sliceFromAny(raw)...)
	label := strings.TrimSpace(extractionStringFromAny(candidate.payload["counterpart"]))
	id := strings.TrimSpace(candidate.unit.AffectedEntityID)
	expression := strings.TrimSpace(extractionStringFromAny(candidate.payload["counterpart_expression"]))
	if label == "" || id == "" || expression == "" {
		return items
	}
	for _, rawItem := range items {
		item := mapFromAny(rawItem)
		if extractionStringFromAny(item["counterpart_entity_id"]) == id && extractionStringFromAny(item["counterpart_expression"]) == expression {
			return items
		}
	}
	return append(items, map[string]any{
		"counterpart_entity_id":  id,
		"counterpart_label":      label,
		"counterpart_expression": expression,
	})
}

func habitEvidenceInterpretation(counts map[string]any) string {
	support := intFromAny(counts["support_total"], 0)
	counterexamples := intFromAny(counts["counterexample"], 0)
	exceptions := intFromAny(counts["exception"], 0)
	switch {
	case support > 0 && counterexamples > 0:
		return "contested_evidence"
	case support > 0 && exceptions > 0:
		return "contextual_evidence"
	case support > 0:
		return "support_evidence"
	case counterexamples > 0 || exceptions > 0:
		return "counterevidence_only"
	default:
		return "evidence_only"
	}
}

func habitEvidenceTimeDistribution(raw any) map[string]any {
	turns := habitEvidenceAppendUniqueTurn(raw, 0)
	if len(turns) == 0 {
		return map[string]any{"state": "observed_only", "distinct_turns": 0}
	}
	return map[string]any{
		"state":          "observed_only",
		"distinct_turns": len(turns),
		"first_turn":     turns[0],
		"latest_turn":    turns[len(turns)-1],
	}
}

func habitEvidenceEventEvidence(candidate habitEvidenceCandidate, currentProjection bool, resolutionStatus string, previous store.StatusCurrentValue) map[string]any {
	unit := candidate.unit
	evidence := map[string]any{
		"contract_version":       habitEvidenceContractVersion,
		"source":                 "precise_memory_unit.habit_observation.v1",
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
		"evidence_hash":          unit.EvidenceHash,
		"evidence_excerpt":       unit.EvidenceExcerpt,
		"evidence_fingerprint":   candidate.evidenceFingerprint,
		"current_projection":     currentProjection,
		"resolution_status":      resolutionStatus,
		"habit_observation": map[string]any{
			"subject_entity":         candidate.payload["subject_entity"],
			"behavior_key":           candidate.payload["behavior_key"],
			"behavior_expression":    candidate.payload["behavior_expression"],
			"observation_kind":       candidate.payload["observation_kind"],
			"context_key":            candidate.payload["context_key"],
			"context_expression":     candidate.payload["context_expression"],
			"counterpart":            candidate.payload["counterpart"],
			"counterpart_entity_id":  nilIfEmpty(unit.AffectedEntityID),
			"counterpart_expression": candidate.payload["counterpart_expression"],
			"visibility":             unit.Visibility,
		},
		"authority_class":        "support_hypothesis",
		"disposition":            "evidence_only",
		"stable_habit_inference": false,
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

func ensureHabitEvidenceDefinition(ctx context.Context, st store.Store, sid string, now time.Time, result *artifactSaveResult) (store.StatusSchemaDefinition, bool) {
	registry, ok := st.(store.StatusSchemaRegistryStore)
	if !ok {
		result.addSkipReason("habit_evidence", "status_schema_registry_unavailable", nil)
		return store.StatusSchemaDefinition{}, false
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, habitEvidenceStatusKey, habitEvidenceOwnerScope)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("habit_evidence", "status_schema_lookup_failed", err.Error())
		return store.StatusSchemaDefinition{}, false
	}
	result.Attempted++
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid,
		SchemaName:    "character_habit_evidence",
		StatusKey:     habitEvidenceStatusKey,
		Label:         "Character habit evidence",
		OwnerScope:    habitEvidenceOwnerScope,
		ValueKind:     "object",
		OptionsJSON: mustCompactJSON(map[string]any{
			"contract_version":              habitEvidenceContractVersion,
			"admitted_observation_contract": habitObservationContract,
			"authority_class":               "support_hypothesis",
			"disposition":                   "evidence_only",
			"source_root_idempotent":        true,
			"exceptions_preserved":          true,
			"counterevidence_preserved":     true,
			"contexts_preserved":            true,
			"counterparts_preserved":        true,
			"fixed_count_promotion":         false,
			"fixed_score_promotion":         false,
			"quiet_turn_dormancy":           false,
			"stable_habit_inference":        false,
			"speech_style_projection":       false,
		}),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil || len(definitions) == 0 {
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusSchemaDefinitions(habit_evidence): "+err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	result.StatusSchemaDefinitions++
	return definitions[0], true
}

func habitEvidenceCurrentMatchesCandidate(current store.StatusCurrentValue, candidate habitEvidenceCandidate) bool {
	projection := map[string]any{}
	if json.Unmarshal([]byte(current.ValueJSON), &projection) != nil {
		return false
	}
	return extractionStringFromAny(projection["version"]) == habitEvidenceContractVersion &&
		extractionStringFromAny(projection["subject_entity_id"]) == candidate.unit.SubjectEntityID &&
		extractionStringFromAny(projection["behavior_key"]) == extractionStringFromAny(candidate.payload["behavior_key"]) &&
		extractionStringFromAny(projection["expression_scope"]) == candidate.expressionScope &&
		extractionStringFromAny(projection["authority_class"]) == "support_hypothesis" &&
		extractionStringFromAny(projection["disposition"]) == "evidence_only"
}

func habitEvidenceOwnerID(subjectEntityID, behaviorKey, expressionScope string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		habitEvidenceContractVersion,
		strings.TrimSpace(subjectEntityID),
		strings.TrimSpace(behaviorKey),
		strings.TrimSpace(expressionScope),
	}, "\x1f")))
	return "habit-evidence:" + hex.EncodeToString(sum[:])
}

func habitEvidenceSourceUnitID(preciseMemoryUnitID string) string {
	sum := sha256.Sum256([]byte(habitEvidenceContractVersion + "\x1f" + strings.TrimSpace(preciseMemoryUnitID)))
	return "habit-evidence-event:" + hex.EncodeToString(sum[:])
}

func habitEvidenceRootFingerprint(unit *store.PreciseMemoryUnit, observationKind string) string {
	observationClass := strings.TrimSpace(observationKind)
	if observationClass == "occurrence" || observationClass == "explicit_pattern" {
		observationClass = "support"
	}
	provenance := ""
	if unit != nil && unit.RootEvidenceID > 0 {
		provenance = fmt.Sprintf("evidence:%d", unit.RootEvidenceID)
	} else if unit != nil {
		provenance = strings.Join([]string{"unit", unit.UnitID, unit.SourceRevision}, ":")
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{habitEvidenceContractVersion, provenance, observationClass}, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func habitEvidenceObservationSortPriority(observationKind string) int {
	switch strings.TrimSpace(observationKind) {
	case "explicit_pattern":
		return 1
	case "occurrence":
		return 2
	case "counterexample":
		return 3
	case "exception":
		return 4
	default:
		return 5
	}
}

func habitEvidenceOwnerLabel(payload map[string]any) string {
	return fmt.Sprintf("%s [%s]", extractionStringFromAny(payload["subject_entity"]), extractionStringFromAny(payload["behavior_key"]))
}

func habitEvidencePreciseUnitID(unit *store.PreciseMemoryUnit) string {
	if unit == nil {
		return ""
	}
	return unit.UnitID
}
