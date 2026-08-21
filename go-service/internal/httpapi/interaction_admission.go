package httpapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const (
	risuRequestObservationContract      = "risu_request_observation.v1"
	interactionEventContract            = "interaction_event.v1"
	relationshipObservationContract     = "relationship_observation.v1"
	interactionBoundaryContract         = "interaction_boundary.v1"
	habitObservationContract            = "habit_observation.v1"
	characterProfileObservationContract = "character_profile_observation.v1"
	voiceObservationContract            = "voice_observation.v1"
	userInteractionProfileContract      = "user_interaction_profile.v1"
	rpCharacterProfileContract          = "rp_character_profile.v1"
	publicVisibilitySupportContract     = "public_visibility_support.v1"
	inWorldIdentityProofContract        = "in_world_identity_proof.v1"
)

// shouldApplyCompleteTurnOOCGuard trusts only the versioned host observation.
// Content, punctuation, language, and previous chat messages are deliberately
// not request-class evidence.
func shouldApplyCompleteTurnOOCGuard(clientMeta map[string]any) bool {
	observation := mapFromAny(clientMeta["risu_request_observation"])
	return extractionStringFromAny(observation["contract_version"]) == risuRequestObservationContract &&
		strings.EqualFold(strings.TrimSpace(extractionStringFromAny(observation["ooc_class_state"])), "observed") &&
		strings.EqualFold(strings.TrimSpace(extractionStringFromAny(observation["ooc_class"])), "ooc")
}

// admitCriticInteractionLanes is an observation/admission boundary. It does
// not construct relationship current/history state; that remains a later
// relationship-state owner.
func admitCriticInteractionLanes(raw map[string]any, userInput, assistantContent string) (map[string]any, map[string]any) {
	return admitCriticInteractionLanesWithTrustedIdentities(raw, userInput, assistantContent, nil)
}

func admitCriticInteractionLanesWithTrustedIdentities(raw map[string]any, userInput, assistantContent string, stableCharacterIdentities map[string]*interactionStableCharacterIdentity) (map[string]any, map[string]any) {
	if raw == nil {
		return raw, nil
	}
	out := map[string]any{}
	for key, value := range raw {
		out[key] = value
	}
	source := strings.TrimSpace(strings.Join([]string{userInput, assistantContent}, "\n"))
	sourceHash := fmt.Sprintf("%x", sha256.Sum256([]byte(source)))
	entitySurfaces := interactionAdmissionEntitySurfaces(raw)
	reasons := map[string]int{}
	seen := 0
	kept := 0
	reject := func(reason string) {
		reasons[reason]++
	}

	interactions := []any{}
	for _, rawItem := range sliceFromAny(raw["interaction_events"]) {
		seen++
		item := mapFromAny(rawItem)
		actor := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "actor"), stringFromMap(item, "source_entity")))
		counterpart := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "counterpart"), stringFromMap(item, "target_entity"), stringFromMap(item, "target")))
		action := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "action"), stringFromMap(item, "interaction"), stringFromMap(item, "summary")))
		actorExpression := strings.TrimSpace(stringFromMap(item, "actor_expression"))
		counterpartExpression := strings.TrimSpace(stringFromMap(item, "counterpart_expression"))
		actionExpression := strings.TrimSpace(stringFromMap(item, "action_expression"))
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || action == "" {
			reject("interaction_direction_or_action_missing")
			continue
		}
		admissionState := "review_required"
		reviewState := "needs_review"
		if criticEvidenceOccursInSource(evidence, source) {
			admissionState = "committed"
			reviewState = "source_observed"
		}
		interactions = append(interactions, map[string]any{
			"contract_version":       interactionEventContract,
			"actor":                  actor,
			"actor_expression":       actorExpression,
			"counterpart":            counterpart,
			"counterpart_expression": counterpartExpression,
			"action":                 action,
			"action_expression":      actionExpression,
			"interaction_kind":       strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "interaction_kind"), stringFromMap(item, "event_type"), "interaction")),
			"admission_state":        admissionState,
			"review_state":           reviewState,
			"visibility":             normalizeInteractionVisibilityWithDefault(stringFromMap(item, "visibility"), "public"),
			"evidence_excerpt":       evidence,
			"source_hash":            sourceHash,
		})
		kept++
	}
	out["interaction_events"] = interactions

	relationships := []any{}
	for _, rawItem := range sliceFromAny(raw["relationship_observations"]) {
		seen++
		normalized, reason := normalizeRelationshipObservation(mapFromAny(rawItem), raw, source, sourceHash, entitySurfaces)
		if reason != "" {
			reject(reason)
			continue
		}
		relationships = append(relationships, normalized)
		kept++
	}
	// Preserve the broad relationship memory record. When it also has enough
	// directional structure, expose the same candidate to the typed lane; later
	// projection and prepare-turn selection remain the precision boundary.
	if legacy := mapFromAny(raw["relationship_memory"]); len(legacy) > 0 {
		seen++
		normalized, reason := normalizeRelationshipObservation(legacy, raw, source, sourceHash, entitySurfaces)
		if reason != "" {
			reject("legacy_relationship_" + reason)
		} else {
			relationships = append(relationships, normalized)
			kept++
		}
	}
	out["relationship_observations"] = relationships
	out["relationship_memory"] = mapFromAny(raw["relationship_memory"])

	habitObservations := []any{}
	for _, rawItem := range sliceFromAny(raw["habit_observations"]) {
		seen++
		normalized, reason := normalizeHabitObservation(mapFromAny(rawItem), source, sourceHash, entitySurfaces)
		if reason != "" {
			reject(reason)
			continue
		}
		habitObservations = append(habitObservations, normalized)
		kept++
	}
	out["habit_observations"] = habitObservations

	profileObservations := []any{}
	for _, rawItem := range sliceFromAny(raw["character_profile_observations"]) {
		seen++
		normalized, reason := normalizeCharacterProfileObservation(mapFromAny(rawItem), source, sourceHash, entitySurfaces)
		if reason != "" {
			reject(reason)
			continue
		}
		profileObservations = append(profileObservations, normalized)
		kept++
	}
	out["character_profile_observations"] = profileObservations

	voiceObservations := []any{}
	for _, rawItem := range sliceFromAny(raw["voice_observations"]) {
		seen++
		normalized, reason := normalizeVoiceObservation(mapFromAny(rawItem), raw, source, sourceHash, entitySurfaces)
		if reason != "" {
			reject(reason)
			continue
		}
		voiceObservations = append(voiceObservations, normalized)
		kept++
	}
	out["voice_observations"] = voiceObservations

	boundaries := []any{}
	for _, rawItem := range sliceFromAny(raw["interaction_boundaries"]) {
		seen++
		item := mapFromAny(rawItem)
		actor := strings.TrimSpace(stringFromMap(item, "actor"))
		counterpart := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "counterpart"), stringFromMap(item, "target_entity")))
		actionScope := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "action_scope"), stringFromMap(item, "scope")))
		decision := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "decision"), stringFromMap(item, "boundary_state"))))
		supportKind := strings.ToLower(strings.TrimSpace(stringFromMap(item, "support_kind")))
		actorExpression := strings.TrimSpace(stringFromMap(item, "actor_expression"))
		counterpartExpression := strings.TrimSpace(stringFromMap(item, "counterpart_expression"))
		actionScopeExpression := strings.TrimSpace(stringFromMap(item, "action_scope_expression"))
		explicitExpression := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "decision_expression"),
			stringFromMap(item, "boundary_expression"),
		))
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || actionScope == "" {
			reject("boundary_direction_or_scope_missing")
			continue
		}
		admissionState := "review_required"
		reviewState := "needs_review"
		if criticEvidenceOccursInSource(evidence, source) {
			admissionState = "committed"
			reviewState = "source_observed"
		}
		effectiveScope := strings.ToLower(strings.TrimSpace(stringFromMap(item, "effective_scope")))
		if effectiveScope == "" {
			effectiveScope = "event"
		}
		scopeExpression := strings.TrimSpace(stringFromMap(item, "effective_scope_expression"))
		effectiveTime := normalizePreciseMemoryValue(item["effective_time"])
		timeExpression := strings.TrimSpace(stringFromMap(item, "effective_time_expression"))
		visibility, visibilitySupport, visibilityDisposition := interactionSourceBoundVisibility(item, evidence, "owner_private")
		normalized := map[string]any{
			"contract_version":           interactionBoundaryContract,
			"actor":                      actor,
			"actor_expression":           actorExpression,
			"counterpart":                counterpart,
			"counterpart_expression":     counterpartExpression,
			"action_scope":               actionScope,
			"action_scope_expression":    actionScopeExpression,
			"decision":                   decision,
			"decision_expression":        explicitExpression,
			"support_kind":               supportKind,
			"admission_state":            admissionState,
			"review_state":               reviewState,
			"effective_scope":            effectiveScope,
			"effective_scope_expression": scopeExpression,
			"effective_time":             effectiveTime,
			"effective_time_expression":  timeExpression,
			"visibility":                 visibility,
			"visibility_disposition":     visibilityDisposition,
			"evidence_excerpt":           evidence,
			"source_hash":                sourceHash,
		}
		if len(visibilitySupport) > 0 {
			normalized["public_visibility_support"] = visibilitySupport
		}
		boundaries = append(boundaries, normalized)
		kept++
	}
	out["interaction_boundaries"] = boundaries

	userProfiles := normalizeUserInteractionProfiles(raw["user_interaction_profile"], source, sourceHash, reject, &seen, &kept)
	out["user_interaction_profile"] = userProfiles
	out["rp_character_profile"] = normalizeRPCharacterProfiles(raw["rp_character_profile"], source, sourceHash, entitySurfaces, stableCharacterIdentities, reject, &seen, &kept)
	profileQuarantine := quarantineUserProfileEvidenceFromInWorldLanes(out, userProfiles)
	for lane, count := range profileQuarantine {
		reasons["user_profile_shared_evidence_quarantined:"+lane] += count
		switch lane {
		case "interaction_events", "relationship_observations", "interaction_boundaries", "habit_observations", "character_profile_observations", "voice_observations", "rp_character_profile":
			kept -= count
		}
	}
	if kept < 0 {
		kept = 0
	}
	rpProfileReviewProposals := []any{}
	for _, rawProfile := range sliceFromAny(out["rp_character_profile"]) {
		profile := mapFromAny(rawProfile)
		if stringFromMap(profile, "admission_state") != "committed" {
			rpProfileReviewProposals = append(rpProfileReviewProposals, profile)
		}
	}
	out["kg_triples"] = sliceFromAny(out["kg_triples"])

	reasonPayload := map[string]any{}
	for reason, count := range reasons {
		reasonPayload[reason] = count
	}
	return out, map[string]any{
		"contract_version":              "interaction_admission.v1",
		"candidate_count":               seen,
		"kept_count":                    kept,
		"user_profile_review_proposals": userProfiles,
		"user_profile_quarantine":       profileQuarantine,
		"rp_profile_review_proposals":   rpProfileReviewProposals,
		"reasons":                       reasonPayload,
	}
}

func legacyRelationshipShiftToken(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "relationship_shift", "relationship_change", "relationship_state", "bond_change":
		return true
	default:
		return false
	}
}

func normalizeRelationshipObservation(item, extraction map[string]any, source, sourceHash string, entitySurfaces map[string]map[string]bool) (map[string]any, string) {
	sourceEntity := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "source_entity"), stringFromMap(item, "actor"), stringFromMap(item, "owner")))
	targetEntity := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "target_entity"), stringFromMap(item, "counterpart"), stringFromMap(item, "target"), stringFromMap(item, "target_name")))
	domain := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "domain"), stringFromMap(item, "relation_domain"))))
	observation := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "observation"), stringFromMap(item, "state"), stringFromMap(item, "change"), stringFromMap(item, "summary"), stringFromMap(item, "bond_and_distance")))
	sourceExpression := strings.TrimSpace(stringFromMap(item, "source_entity_expression"))
	targetExpression := strings.TrimSpace(stringFromMap(item, "target_entity_expression"))
	domainExpression := strings.TrimSpace(stringFromMap(item, "domain_expression"))
	supportKind := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "support_kind"), stringFromMap(item, "evidence_basis"))))
	evidence := interactionAdmissionEvidence(item)
	if sourceEntity == "" || targetEntity == "" || comparableEntityKey(sourceEntity) == comparableEntityKey(targetEntity) {
		return nil, "direction_missing_or_self_relation"
	}
	if observation == "" {
		return nil, "observation_missing"
	}
	admissionState := "review_required"
	reviewState := "needs_review"
	if criticEvidenceOccursInSource(evidence, source) {
		admissionState = "committed"
		reviewState = "source_observed"
	}
	visibility, visibilitySupport, visibilityDisposition := interactionSourceBoundVisibility(item, evidence, "owner_private")
	normalized := map[string]any{
		"contract_version":         relationshipObservationContract,
		"source_entity":            sourceEntity,
		"source_entity_expression": sourceExpression,
		"target_entity":            targetEntity,
		"target_entity_expression": targetExpression,
		"domain":                   domain,
		"domain_expression":        domainExpression,
		"observation":              observation,
		"support_kind":             supportKind,
		"admission_state":          admissionState,
		"review_state":             reviewState,
		"visibility":               visibility,
		"visibility_disposition":   visibilityDisposition,
		"evidence_excerpt":         evidence,
		"source_hash":              sourceHash,
	}
	if len(visibilitySupport) > 0 {
		normalized["public_visibility_support"] = visibilitySupport
	}
	if expression := strings.TrimSpace(stringFromMap(item, "magnitude_expression")); expression != "" || item["magnitude"] != nil {
		normalized["magnitude"] = normalizePreciseMemoryValue(item["magnitude"])
		normalized["magnitude_expression"] = expression
	}
	if expression := strings.TrimSpace(stringFromMap(item, "duration_expression")); expression != "" || item["duration"] != nil {
		normalized["duration"] = normalizePreciseMemoryValue(item["duration"])
		normalized["duration_expression"] = expression
	}
	return normalized, ""
}

func normalizeHabitObservation(item map[string]any, source, sourceHash string, entitySurfaces map[string]map[string]bool) (map[string]any, string) {
	subject := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity"), stringFromMap(item, "subject"), stringFromMap(item, "character")))
	subjectExpression := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity_expression"), stringFromMap(item, "subject_expression"), stringFromMap(item, "character_expression")))
	behaviorExpression := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "behavior_expression"), stringFromMap(item, "observation_expression")))
	behaviorKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "behavior_key"), stringFromMap(item, "habit_key"), behaviorExpression))
	observationKind := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "observation_kind"), stringFromMap(item, "evidence_kind"))))
	evidence := interactionAdmissionEvidence(item)
	if subject == "" || (behaviorKey == "" && behaviorExpression == "") {
		return nil, "habit_subject_or_behavior_missing"
	}

	contextKey := strings.TrimSpace(stringFromMap(item, "context_key"))
	contextExpression := strings.TrimSpace(stringFromMap(item, "context_expression"))

	counterpart := strings.TrimSpace(stringFromMap(item, "counterpart"))
	counterpartExpression := strings.TrimSpace(stringFromMap(item, "counterpart_expression"))
	admissionState := "review_required"
	reviewState := "needs_review"
	if criticEvidenceOccursInSource(evidence, source) {
		admissionState = "committed"
		reviewState = "source_observed"
	}

	visibility, visibilitySupport, visibilityDisposition := interactionSourceBoundVisibility(item, evidence, "owner_private")
	normalized := map[string]any{
		"contract_version":          habitObservationContract,
		"subject_entity":            subject,
		"subject_entity_expression": subjectExpression,
		"behavior_key":              behaviorKey,
		"behavior_expression":       behaviorExpression,
		"observation_kind":          observationKind,
		"context_key":               contextKey,
		"context_expression":        contextExpression,
		"counterpart":               counterpart,
		"counterpart_expression":    counterpartExpression,
		"admission_state":           admissionState,
		"review_state":              reviewState,
		"visibility":                visibility,
		"visibility_disposition":    visibilityDisposition,
		"evidence_excerpt":          evidence,
		"source_hash":               sourceHash,
	}
	if len(visibilitySupport) > 0 {
		normalized["public_visibility_support"] = visibilitySupport
	}
	return normalized, ""
}

func normalizeCharacterProfileObservation(item map[string]any, source, sourceHash string, entitySurfaces map[string]map[string]bool) (map[string]any, string) {
	subject := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity"), stringFromMap(item, "subject"), stringFromMap(item, "character")))
	subjectExpression := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity_expression"), stringFromMap(item, "subject_expression"), stringFromMap(item, "character_expression")))
	section := strings.ToLower(strings.TrimSpace(stringFromMap(item, "profile_section")))
	traitDomain := strings.ToLower(strings.TrimSpace(stringFromMap(item, "trait_domain")))
	supportedExpression := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "supported_expression"), stringFromMap(item, "trait_expression")))
	traitKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "trait_key"), supportedExpression))
	observationKind := strings.ToLower(strings.TrimSpace(stringFromMap(item, "observation_kind")))
	evidence := interactionAdmissionEvidence(item)
	if subject == "" || (traitKey == "" && supportedExpression == "") {
		return nil, "character_profile_required_field_missing"
	}
	contextKey := strings.TrimSpace(stringFromMap(item, "context_key"))
	contextExpression := strings.TrimSpace(stringFromMap(item, "context_expression"))
	counterpart := strings.TrimSpace(stringFromMap(item, "counterpart"))
	counterpartExpression := strings.TrimSpace(stringFromMap(item, "counterpart_expression"))
	admissionState := "review_required"
	reviewState := "needs_review"
	if criticEvidenceOccursInSource(evidence, source) {
		admissionState = "committed"
		reviewState = "source_observed"
	}
	visibility, visibilitySupport, visibilityDisposition := interactionSourceBoundVisibility(item, evidence, "owner_private")
	normalized := map[string]any{
		"contract_version":          characterProfileObservationContract,
		"subject_entity":            subject,
		"subject_entity_expression": subjectExpression,
		"profile_section":           section,
		"trait_domain":              traitDomain,
		"trait_key":                 traitKey,
		"supported_expression":      supportedExpression,
		"observation_kind":          observationKind,
		"context_key":               contextKey,
		"context_expression":        contextExpression,
		"counterpart":               counterpart,
		"counterpart_expression":    counterpartExpression,
		"admission_state":           admissionState,
		"review_state":              reviewState,
		"visibility":                visibility,
		"visibility_disposition":    visibilityDisposition,
		"evidence_excerpt":          evidence,
		"source_hash":               sourceHash,
	}
	if len(visibilitySupport) > 0 {
		normalized["public_visibility_support"] = visibilitySupport
	}
	return normalized, ""
}

func normalizeVoiceObservation(item, extraction map[string]any, source, sourceHash string, entitySurfaces map[string]map[string]bool) (map[string]any, string) {
	subject := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity"), stringFromMap(item, "speaker"), stringFromMap(item, "character")))
	subjectExpression := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject_entity_expression"), stringFromMap(item, "speaker_expression"), stringFromMap(item, "character_expression")))
	traitDomain := strings.ToLower(strings.TrimSpace(stringFromMap(item, "trait_domain")))
	principleKey := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "principle_key"),
		stringFromMap(item, "style_key"),
		stringFromMap(item, "principle"),
		stringFromMap(item, "speech_style"),
	))
	observationKind := strings.ToLower(strings.TrimSpace(stringFromMap(item, "observation_kind")))
	utteranceExpression := strings.TrimSpace(stringFromMap(item, "utterance_expression"))
	evidence := interactionAdmissionEvidence(item)
	if subject == "" || (principleKey == "" && utteranceExpression == "") {
		return nil, "voice_observation_required_field_or_domain_invalid"
	}
	contextKey := strings.TrimSpace(stringFromMap(item, "context_key"))
	contextExpression := strings.TrimSpace(stringFromMap(item, "context_expression"))
	counterpart := strings.TrimSpace(stringFromMap(item, "counterpart"))
	counterpartExpression := strings.TrimSpace(stringFromMap(item, "counterpart_expression"))
	stateKey := strings.TrimSpace(stringFromMap(item, "state_modulation_key"))
	stateExpression := strings.TrimSpace(stringFromMap(item, "state_modulation_expression"))
	admissionState := "review_required"
	reviewState := "needs_review"
	if criticEvidenceOccursInSource(evidence, source) {
		admissionState = "committed"
		reviewState = "source_observed"
	}
	visibility, visibilitySupport, visibilityDisposition := interactionSourceBoundVisibility(item, evidence, "owner_private")
	normalized := map[string]any{
		"contract_version":            voiceObservationContract,
		"subject_entity":              subject,
		"subject_entity_expression":   subjectExpression,
		"trait_domain":                traitDomain,
		"principle_key":               principleKey,
		"observation_kind":            observationKind,
		"utterance_expression":        utteranceExpression,
		"context_key":                 contextKey,
		"context_expression":          contextExpression,
		"counterpart":                 counterpart,
		"counterpart_expression":      counterpartExpression,
		"state_modulation_key":        stateKey,
		"state_modulation_expression": stateExpression,
		"admission_state":             admissionState,
		"review_state":                reviewState,
		"visibility":                  visibility,
		"visibility_disposition":      visibilityDisposition,
		"evidence_excerpt":            evidence,
		"source_hash":                 sourceHash,
	}
	if len(visibilitySupport) > 0 {
		normalized["public_visibility_support"] = visibilitySupport
	}
	return normalized, ""
}

func interactionExplicitExpressionOccursInEvidence(expression, evidence string) bool {
	expression = normalizeArtifactComparableText(expression)
	evidence = normalizeArtifactComparableText(evidence)
	return expression != "" && evidence != "" && strings.Contains(evidence, expression)
}

func interactionExactValueExpression(value, expression, evidence string) bool {
	value = normalizeArtifactComparableText(value)
	expression = normalizeArtifactComparableText(expression)
	return value != "" && value == expression && interactionExplicitExpressionOccursInEvidence(expression, evidence)
}

func interactionAdmissionEntitySurfaces(extraction map[string]any) map[string]map[string]bool {
	result := map[string]map[string]bool{}
	entities := mapFromAny(extraction["entities"])
	for _, bucket := range []string{"characters", "locations", "places", "items", "objects", "groups"} {
		for _, raw := range sliceFromAny(entities[bucket]) {
			item := mapFromAny(raw)
			name := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(item, "name"),
				stringFromMap(item, "label"),
				stringFromMap(item, "title"),
			))
			if name == "" {
				name = strings.TrimSpace(extractionStringFromAny(raw))
			}
			if name == "" {
				continue
			}
			surfaces := map[string]bool{comparableEntityKey(name): true}
			for _, alias := range stringsFromAny(item["aliases"]) {
				if key := comparableEntityKey(alias); key != "" {
					surfaces[key] = true
				}
			}
			for key := range surfaces {
				if key == "" {
					continue
				}
				if existing, ok := result[key]; ok && !interactionSameSurfaceSet(existing, surfaces) {
					// A shared surface cannot prove which entity the model meant.
					result[key] = nil
					continue
				}
				result[key] = surfaces
			}
		}
	}
	return result
}

type interactionStableCharacterIdentity struct {
	stableEntityID string
	namespace      string
}

func interactionSameSurfaceSet(left, right map[string]bool) bool {
	if left == nil || right == nil || len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

func interactionEntityExpressionBound(entity, expression, evidence string, surfaces map[string]map[string]bool) bool {
	entityKey := comparableEntityKey(entity)
	if entityKey == "" {
		return false
	}
	if expressionKey := comparableEntityKey(expression); expressionKey != "" && entityKey == expressionKey {
		return interactionExplicitExpressionOccursInEvidence(expression, evidence)
	}
	known, exists := surfaces[entityKey]
	if !exists || known == nil {
		return false
	}
	if expressionKey := comparableEntityKey(expression); expressionKey != "" {
		return interactionExplicitExpressionOccursInEvidence(expression, evidence) &&
			(entityKey == expressionKey || known[expressionKey])
	}
	for surfaceKey := range known {
		if interactionExplicitExpressionOccursInEvidence(surfaceKey, evidence) {
			return true
		}
	}
	return false
}

func quarantineUserProfileEvidenceFromInWorldLanes(extraction map[string]any, profiles []any) map[string]int {
	evidenceSpans := []string{}
	for _, raw := range profiles {
		if key := normalizeArtifactComparableText(interactionAdmissionEvidence(mapFromAny(raw))); key != "" {
			evidenceSpans = append(evidenceSpans, key)
		}
	}
	if len(evidenceSpans) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, lane := range []string{
		"interaction_events", "relationship_observations", "interaction_boundaries", "habit_observations", "character_profile_observations", "voice_observations", "rp_character_profile",
		"narrative_events", "state_claims", "belief_updates", "kg_triples", "reversible_states",
		"world_rules", "subjective_entity_memories", "protected_secrets",
		"character_identity_accuracy", "persona_capsule_candidates",
	} {
		items := sliceFromAny(extraction[lane])
		keptItems := make([]any, 0, len(items))
		for _, raw := range items {
			if interactionCandidateSharesEvidence(raw, evidenceSpans) {
				counts[lane]++
				continue
			}
			keptItems = append(keptItems, raw)
		}
		extraction[lane] = keptItems
	}

	characterDeltas := sliceFromAny(extraction["character_deltas"])
	keptCharacters := make([]any, 0, len(characterDeltas))
	for _, raw := range characterDeltas {
		item := mapFromAny(raw)
		if interactionCandidateSharesEvidence(item, evidenceSpans) {
			counts["character_deltas"]++
			continue
		}
		events := sliceFromAny(item["events"])
		if len(events) > 0 {
			keptEvents := make([]any, 0, len(events))
			for _, event := range events {
				if interactionCandidateSharesEvidence(event, evidenceSpans) {
					counts["character_deltas.events"]++
					continue
				}
				keptEvents = append(keptEvents, event)
			}
			item["events"] = keptEvents
		}
		keptCharacters = append(keptCharacters, item)
	}
	extraction["character_deltas"] = keptCharacters

	worldState := mapFromAny(extraction["world_state"])
	if len(worldState) > 0 {
		if interactionCandidateSharesEvidence(worldState, evidenceSpans) {
			counts["world_state"]++
			extraction["world_state"] = map[string]any{}
		} else {
			rules := sliceFromAny(worldState["rules"])
			keptRules := make([]any, 0, len(rules))
			for _, rule := range rules {
				if interactionCandidateSharesEvidence(rule, evidenceSpans) {
					counts["world_state.rules"]++
					continue
				}
				keptRules = append(keptRules, rule)
			}
			worldState["rules"] = keptRules
			extraction["world_state"] = worldState
		}
	}

	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	keptExcerpts := make([]string, 0, len(excerpts))
	for _, excerpt := range excerpts {
		if interactionEvidenceOverlapsAny(excerpt, evidenceSpans) {
			counts["evidence_excerpts"]++
			continue
		}
		keptExcerpts = append(keptExcerpts, excerpt)
	}
	extraction["evidence_excerpts"] = keptExcerpts
	return counts
}

func interactionCandidateSharesEvidence(raw any, evidenceSpans []string) bool {
	item := mapFromAny(raw)
	if len(item) == 0 {
		return false
	}
	return interactionEvidenceOverlapsAny(interactionAdmissionEvidence(item), evidenceSpans)
}

func interactionEvidenceOverlapsAny(candidate string, evidenceSpans []string) bool {
	candidate = normalizeArtifactComparableText(candidate)
	if candidate == "" {
		return false
	}
	for _, span := range evidenceSpans {
		span = normalizeArtifactComparableText(span)
		if span != "" && (candidate == span || strings.Contains(candidate, span) || strings.Contains(span, candidate)) {
			return true
		}
	}
	return false
}

func normalizeUserInteractionProfiles(raw any, source, sourceHash string, reject func(string), seen, kept *int) []any {
	out := []any{}
	for _, rawItem := range sliceFromAny(raw) {
		(*seen)++
		item := mapFromAny(rawItem)
		profileKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "profile_key"), stringFromMap(item, "action_scope"), stringFromMap(item, "setting")))
		value := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "value"), stringFromMap(item, "preference"), stringFromMap(item, "decision")))
		profileKeyExpression := strings.TrimSpace(stringFromMap(item, "profile_key_expression"))
		valueExpression := strings.TrimSpace(stringFromMap(item, "value_expression"))
		evidence := interactionAdmissionEvidence(item)
		if profileKey == "" || value == "" {
			reject("user_profile_key_or_value_missing")
			continue
		}
		out = append(out, map[string]any{
			"contract_version":       userInteractionProfileContract,
			"namespace":              "user_interaction_profile",
			"profile_key":            profileKey,
			"profile_key_expression": profileKeyExpression,
			"value":                  value,
			"value_expression":       valueExpression,
			"admission_state":        "review_required",
			"review_state":           "ooc_class_unobserved",
			"visibility":             "user_private",
			"evidence_excerpt":       evidence,
			"source_hash":            sourceHash,
		})
		(*kept)++
	}
	return out
}

func normalizeRPCharacterProfiles(raw any, source, sourceHash string, entitySurfaces map[string]map[string]bool, stableIdentities map[string]*interactionStableCharacterIdentity, reject func(string), seen, kept *int) []any {
	out := []any{}
	for _, rawItem := range sliceFromAny(raw) {
		(*seen)++
		item := mapFromAny(rawItem)
		character := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "character"), stringFromMap(item, "entity"), stringFromMap(item, "name")))
		profileKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "profile_key"), stringFromMap(item, "trait"), stringFromMap(item, "preference")))
		value := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "value"), stringFromMap(item, "state"), stringFromMap(item, "description")))
		characterExpression := strings.TrimSpace(stringFromMap(item, "character_expression"))
		valueExpression := strings.TrimSpace(stringFromMap(item, "value_expression"))
		evidence := interactionAdmissionEvidence(item)
		if character == "" || (profileKey == "" && value == "") {
			reject("rp_profile_character_or_content_missing")
			continue
		}
		identityProof, proofValid := normalizeRPCharacterIdentityProof(item, character, characterExpression, evidence, stableIdentities)
		admissionState := "review_required"
		reviewState := "stable_in_world_identity_unverified"
		if proofValid &&
			criticEvidenceOccursInSource(evidence, source) &&
			interactionEntityExpressionBound(character, characterExpression, evidence, entitySurfaces) &&
			(value == "" || interactionExactValueExpression(value, valueExpression, evidence)) {
			admissionState = "committed"
			reviewState = "source_observed"
		}
		out = append(out, map[string]any{
			"contract_version":     rpCharacterProfileContract,
			"namespace":            "rp_character_profile",
			"character":            character,
			"character_expression": characterExpression,
			"profile_key":          profileKey,
			"value":                value,
			"value_expression":     valueExpression,
			"identity_proof":       identityProof,
			"admission_state":      admissionState,
			"review_state":         reviewState,
			"visibility":           "owner_private",
			"evidence_excerpt":     evidence,
			"source_hash":          sourceHash,
		})
		(*kept)++
	}
	return out
}

func normalizeRPCharacterIdentityProof(item map[string]any, character, characterExpression, evidence string, stableIdentities map[string]*interactionStableCharacterIdentity) (map[string]any, bool) {
	proof := mapFromAny(item["identity_proof"])
	if stringFromMap(proof, "contract_version") != inWorldIdentityProofContract {
		return nil, false
	}
	stableEntityID := strings.TrimSpace(stringFromMap(proof, "stable_entity_id"))
	namespace := strings.ToLower(strings.TrimSpace(stringFromMap(proof, "identity_namespace")))
	proofExpression := strings.TrimSpace(stringFromMap(proof, "character_expression"))
	if stableEntityID == "" || (namespace != "session_npc" && namespace != "session_player") ||
		normalizeArtifactComparableText(proofExpression) != normalizeArtifactComparableText(characterExpression) ||
		!interactionExplicitExpressionOccursInEvidence(proofExpression, evidence) {
		return nil, false
	}
	identity := stableIdentities[comparableEntityKey(character)]
	if identity == nil || identity.stableEntityID != stableEntityID || identity.namespace != namespace {
		return nil, false
	}
	return map[string]any{
		"contract_version":     inWorldIdentityProofContract,
		"stable_entity_id":     stableEntityID,
		"identity_namespace":   namespace,
		"character_expression": proofExpression,
	}, true
}

func interactionAdmissionEvidence(item map[string]any) string {
	return strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "evidence_excerpt"),
		stringFromMap(item, "evidence"),
		stringFromMap(item, "source_excerpt"),
	))
}

func normalizeInteractionVisibility(raw string) string {
	return normalizeInteractionVisibilityWithDefault(raw, "public")
}

func normalizeInteractionVisibilityWithDefault(raw, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "public", "owner_private", "restricted", "user_private":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	switch strings.ToLower(strings.TrimSpace(fallback)) {
	case "owner_private", "restricted", "user_private":
		return strings.ToLower(strings.TrimSpace(fallback))
	default:
		return "public"
	}
}

func interactionSourceBoundVisibility(item map[string]any, evidence, fallback string) (string, map[string]any, string) {
	visibility := normalizeInteractionVisibilityWithDefault(stringFromMap(item, "visibility"), "public")
	if visibility != "public" {
		return visibility, nil, "private_or_restricted"
	}
	return "public", nil, "public_or_unspecified"
}

func validInteractionBoundaryDecision(value string) bool {
	switch value {
	case "allow", "refuse", "withdrawn", "unknown":
		return true
	default:
		return false
	}
}

func interactionBoundaryDecisionPriority(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "withdrawn":
		return 4
	case "refuse":
		return 3
	case "allow":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func interactionBoundaryKey(item map[string]any) string {
	return strings.Join([]string{
		comparableEntityKey(stringFromMap(item, "actor")),
		comparableEntityKey(stringFromMap(item, "counterpart")),
		normalizeArtifactComparableText(stringFromMap(item, "action_scope")),
		strings.ToLower(strings.TrimSpace(stringFromMap(item, "effective_scope"))),
		normalizeArtifactComparableText(mustCompactJSON(item["effective_time"])),
	}, "\x1f")
}

func interactionAdmissionPreciseMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	for _, rawItem := range sliceFromAny(extraction["interaction_events"]) {
		item := mapFromAny(rawItem)
		actor := stringFromMap(item, "actor")
		counterpart := stringFromMap(item, "counterpart")
		action := stringFromMap(item, "action")
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || action == "" {
			continue
		}
		out = append(out, preciseMemoryCandidate{
			kind: "event", subtype: "atomic_interaction", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "actor", "actor_expression", "counterpart", "counterpart_expression", "action", "action_expression",
				"interaction_kind", "admission_state", "review_state", "visibility", "source_hash",
			}),
			confidence: 1, truthScope: "source_occurrence", epistemicMode: "direct",
			authorityClass: "objective_world_state", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "review_required"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "needs_review"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces:      map[string]string{"actor": actor, "affected": counterpart},
			requiredRoles: map[string]bool{"actor": true, "affected": true},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["relationship_observations"]) {
		item := mapFromAny(rawItem)
		sourceEntity := stringFromMap(item, "source_entity")
		targetEntity := stringFromMap(item, "target_entity")
		domain := stringFromMap(item, "domain")
		observation := stringFromMap(item, "observation")
		evidence := interactionAdmissionEvidence(item)
		if sourceEntity == "" || targetEntity == "" || observation == "" {
			continue
		}
		subtype := "relationship_observation"
		if domain != "" {
			subtype = "relationship_" + domain
		}
		payloadKeys := []string{
			"contract_version", "source_entity", "source_entity_expression", "target_entity", "target_entity_expression", "domain", "domain_expression",
			"observation", "support_kind", "admission_state", "review_state",
			"visibility", "public_visibility_support", "visibility_disposition", "source_hash",
		}
		if _, exists := item["magnitude"]; exists || strings.TrimSpace(stringFromMap(item, "magnitude_expression")) != "" {
			payloadKeys = append(payloadKeys, "magnitude", "magnitude_expression")
		}
		if _, exists := item["duration"]; exists || strings.TrimSpace(stringFromMap(item, "duration_expression")) != "" {
			payloadKeys = append(payloadKeys, "duration", "duration_expression")
		}
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: subtype, excerpt: evidence,
			payload:    preciseMemorySemanticPayload(item, payloadKeys),
			confidence: 1, truthScope: "source_scoped", epistemicMode: "direct",
			authorityClass: "subjective_episodic", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			relationshipKey: comparableEntityKey(sourceEntity) + "->" + comparableEntityKey(targetEntity) + "/" + domain,
			surfaces:        map[string]string{"actor": sourceEntity, "affected": targetEntity},
			requiredRoles:   map[string]bool{"actor": true, "affected": true},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["habit_observations"]) {
		item := mapFromAny(rawItem)
		subject := stringFromMap(item, "subject_entity")
		counterpart := stringFromMap(item, "counterpart")
		behaviorKey := strings.TrimSpace(stringFromMap(item, "behavior_key"))
		evidence := interactionAdmissionEvidence(item)
		if subject == "" || behaviorKey == "" {
			continue
		}
		requiredRoles := map[string]bool{"subject": true}
		surfaces := map[string]string{"subject": subject}
		if counterpart != "" {
			requiredRoles["affected"] = true
			surfaces["affected"] = counterpart
		}
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: "habit_observation", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "subject_entity", "subject_entity_expression",
				"behavior_key", "behavior_expression", "observation_kind",
				"context_key", "context_expression", "counterpart", "counterpart_expression",
				"admission_state", "review_state", "visibility", "public_visibility_support",
				"visibility_disposition", "source_hash",
			}),
			confidence: 1, truthScope: "support_only", epistemicMode: "direct",
			authorityClass: "support_hypothesis", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces: surfaces, requiredRoles: requiredRoles,
		})
	}
	for _, rawItem := range sliceFromAny(extraction["character_profile_observations"]) {
		item := mapFromAny(rawItem)
		subject := stringFromMap(item, "subject_entity")
		counterpart := stringFromMap(item, "counterpart")
		evidence := interactionAdmissionEvidence(item)
		if subject == "" || strings.TrimSpace(stringFromMap(item, "trait_key")) == "" {
			continue
		}
		requiredRoles := map[string]bool{"subject": true}
		surfaces := map[string]string{"subject": subject}
		if counterpart != "" {
			requiredRoles["affected"] = true
			surfaces["affected"] = counterpart
		}
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: "character_profile", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "subject_entity", "subject_entity_expression", "profile_section",
				"trait_domain", "trait_key", "supported_expression", "observation_kind",
				"context_key", "context_expression", "counterpart", "counterpart_expression",
				"admission_state", "review_state", "visibility", "public_visibility_support",
				"visibility_disposition", "source_hash",
			}),
			confidence: 1, truthScope: "support_only", epistemicMode: "direct",
			authorityClass: "support_hypothesis", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces: surfaces, requiredRoles: requiredRoles,
		})
	}
	for _, rawItem := range sliceFromAny(extraction["voice_observations"]) {
		item := mapFromAny(rawItem)
		subject := stringFromMap(item, "subject_entity")
		counterpart := stringFromMap(item, "counterpart")
		evidence := interactionAdmissionEvidence(item)
		if subject == "" || strings.TrimSpace(stringFromMap(item, "principle_key")) == "" {
			continue
		}
		requiredRoles := map[string]bool{"subject": true}
		surfaces := map[string]string{"subject": subject}
		if counterpart != "" {
			requiredRoles["affected"] = true
			surfaces["affected"] = counterpart
		}
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: "voice_behavior", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "subject_entity", "subject_entity_expression", "trait_domain",
				"principle_key", "observation_kind", "utterance_expression",
				"context_key", "context_expression", "counterpart", "counterpart_expression",
				"state_modulation_key", "state_modulation_expression", "admission_state", "review_state",
				"visibility", "public_visibility_support", "visibility_disposition", "source_hash",
			}),
			confidence: 1, truthScope: "support_only", epistemicMode: "direct",
			authorityClass: "support_hypothesis", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces: surfaces, requiredRoles: requiredRoles,
		})
	}
	for _, rawItem := range sliceFromAny(extraction["interaction_boundaries"]) {
		item := mapFromAny(rawItem)
		actor := stringFromMap(item, "actor")
		counterpart := stringFromMap(item, "counterpart")
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || strings.TrimSpace(stringFromMap(item, "action_scope")) == "" {
			continue
		}
		visibility := normalizeInteractionVisibility(stringFromMap(item, "visibility"))
		out = append(out, preciseMemoryCandidate{
			kind: "boundary", subtype: stringFromMap(item, "decision"), excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "actor", "actor_expression", "counterpart", "counterpart_expression", "action_scope", "action_scope_expression",
				"decision", "decision_expression", "support_kind", "effective_scope", "effective_scope_expression",
				"effective_time", "effective_time_expression", "visibility", "public_visibility_support", "visibility_disposition",
				"admission_state", "review_state",
				"source_hash",
			}),
			confidence: 1, truthScope: "actor_scoped", epistemicMode: "explicit_boundary",
			authorityClass: "subjective_episodic", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: visibility,
			relationshipKey: interactionBoundaryKey(item),
			surfaces:        map[string]string{"actor": actor, "affected": counterpart},
			requiredRoles:   map[string]bool{"actor": true, "affected": true},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["user_interaction_profile"]) {
		item := mapFromAny(rawItem)
		evidence := interactionAdmissionEvidence(item)
		if strings.TrimSpace(stringFromMap(item, "profile_key")) == "" ||
			strings.TrimSpace(stringFromMap(item, "value")) == "" {
			continue
		}
		out = append(out, preciseMemoryCandidate{
			kind: "profile", subtype: "user_interaction", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "namespace", "profile_key", "profile_key_expression", "value", "value_expression",
				"admission_state", "review_state",
				"visibility", "source_hash",
			}),
			confidence: 1, truthScope: "user_ooc", epistemicMode: "explicit_ooc_setting",
			authorityClass: "ooc_meta", admissionState: "review_required",
			reviewState: "needs_review", visibility: "user_private",
			surfaces: map[string]string{}, requiredRoles: map[string]bool{},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["rp_character_profile"]) {
		item := mapFromAny(rawItem)
		character := stringFromMap(item, "character")
		evidence := interactionAdmissionEvidence(item)
		if character == "" || (strings.TrimSpace(stringFromMap(item, "profile_key")) == "" &&
			strings.TrimSpace(stringFromMap(item, "value")) == "") {
			continue
		}
		out = append(out, preciseMemoryCandidate{
			kind: "profile", subtype: "rp_character", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "namespace", "character", "character_expression", "profile_key",
				"value", "value_expression", "identity_proof", "admission_state", "review_state", "visibility", "source_hash",
			}),
			confidence: 1, truthScope: "character_scoped", epistemicMode: "direct",
			authorityClass: "subjective_episodic", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces:      map[string]string{"subject": character},
			requiredRoles: map[string]bool{"subject": true},
		})
	}
	return out
}
