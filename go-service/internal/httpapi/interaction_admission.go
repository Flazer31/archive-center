package httpapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const (
	risuRequestObservationContract  = "risu_request_observation.v1"
	interactionEventContract        = "interaction_event.v1"
	relationshipObservationContract = "relationship_observation.v1"
	interactionBoundaryContract     = "interaction_boundary.v1"
	userInteractionProfileContract  = "user_interaction_profile.v1"
	rpCharacterProfileContract      = "rp_character_profile.v1"
	publicVisibilitySupportContract = "public_visibility_support.v1"
	inWorldIdentityProofContract    = "in_world_identity_proof.v1"
	kgEndpointBindingContract       = "kg_endpoint_binding.v1"
)

var relationshipObservationDomains = map[string]bool{
	"trust": true, "attachment": true, "romantic": true, "rivalry": true,
	"fear": true, "obligation": true, "respect": true, "obedience": true,
	"intimacy": true,
}

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
		if !criticEvidenceOccursInSource(evidence, source) {
			reject("interaction_exact_current_evidence_required")
			continue
		}
		if !interactionEntityExpressionBound(actor, actorExpression, evidence, entitySurfaces) ||
			!interactionEntityExpressionBound(counterpart, counterpartExpression, evidence, entitySurfaces) {
			reject("interaction_entity_expression_unbound")
			continue
		}
		if !interactionExactValueExpression(action, actionExpression, evidence) {
			reject("interaction_action_expression_unbound")
			continue
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
		normalized, reason := normalizeRelationshipObservation(mapFromAny(rawItem), source, sourceHash, entitySurfaces)
		if reason != "" {
			reject(reason)
			continue
		}
		relationships = append(relationships, normalized)
		kept++
	}
	// A legacy relationship_memory object is accepted only as a typed,
	// directional observation. It is then removed from the legacy current-state
	// lane so it cannot update Trust/ActiveState in parallel.
	if legacy := mapFromAny(raw["relationship_memory"]); len(legacy) > 0 {
		seen++
		normalized, reason := normalizeRelationshipObservation(legacy, source, sourceHash, entitySurfaces)
		if reason != "" {
			reject("legacy_relationship_" + reason)
		} else {
			relationships = append(relationships, normalized)
			kept++
		}
	}
	out["relationship_observations"] = relationships
	out["relationship_memory"] = map[string]any{}

	boundariesByKey := map[string]map[string]any{}
	boundaryOrder := []string{}
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
		if !validInteractionBoundaryDecision(decision) {
			reject("boundary_decision_invalid")
			continue
		}
		if !validInteractionBoundarySupportKind(supportKind) {
			reject("boundary_explicit_support_kind_required")
			continue
		}
		if !criticEvidenceOccursInSource(evidence, source) {
			reject("boundary_exact_current_evidence_required")
			continue
		}
		if !interactionExplicitExpressionOccursInEvidence(explicitExpression, evidence) {
			reject("boundary_explicit_expression_required")
			continue
		}
		if !interactionEntityExpressionBound(actor, actorExpression, evidence, entitySurfaces) ||
			!interactionEntityExpressionBound(counterpart, counterpartExpression, evidence, entitySurfaces) {
			reject("boundary_entity_expression_unbound")
			continue
		}
		if !interactionExactValueExpression(actionScope, actionScopeExpression, evidence) {
			reject("boundary_action_scope_expression_unbound")
			continue
		}
		effectiveScope := strings.ToLower(strings.TrimSpace(stringFromMap(item, "effective_scope")))
		if effectiveScope == "" {
			effectiveScope = "event"
		}
		scopeExpression := strings.TrimSpace(stringFromMap(item, "effective_scope_expression"))
		if effectiveScope != "event" && !interactionExplicitExpressionOccursInEvidence(scopeExpression, evidence) {
			effectiveScope = "event"
			scopeExpression = ""
		}
		effectiveTime := any(nil)
		timeExpression := strings.TrimSpace(stringFromMap(item, "effective_time_expression"))
		if interactionExplicitExpressionOccursInEvidence(timeExpression, evidence) {
			effectiveTime = normalizePreciseMemoryValue(item["effective_time"])
		} else {
			timeExpression = ""
		}
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
			"admission_state":            "committed",
			"review_state":               "source_observed",
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
		key := interactionBoundaryKey(normalized)
		current, exists := boundariesByKey[key]
		if !exists {
			boundaryOrder = append(boundaryOrder, key)
			boundariesByKey[key] = normalized
			kept++
			continue
		}
		if interactionBoundaryDecisionPriority(decision) > interactionBoundaryDecisionPriority(stringFromMap(current, "decision")) {
			boundariesByKey[key] = normalized
		}
		reject("boundary_lower_priority_same_scope_superseded")
	}
	boundaries := make([]any, 0, len(boundaryOrder))
	for _, key := range boundaryOrder {
		boundaries = append(boundaries, boundariesByKey[key])
	}
	out["interaction_boundaries"] = boundaries

	userProfiles := normalizeUserInteractionProfiles(raw["user_interaction_profile"], source, sourceHash, reject, &seen, &kept)
	out["user_interaction_profile"] = userProfiles
	out["rp_character_profile"] = normalizeRPCharacterProfiles(raw["rp_character_profile"], source, sourceHash, entitySurfaces, stableCharacterIdentities, reject, &seen, &kept)
	profileQuarantine := quarantineUserProfileEvidenceFromInWorldLanes(out, userProfiles)
	for lane, count := range profileQuarantine {
		reasons["user_profile_shared_evidence_quarantined:"+lane] += count
		switch lane {
		case "interaction_events", "relationship_observations", "interaction_boundaries", "rp_character_profile":
			kept -= count
		}
	}
	if kept < 0 {
		kept = 0
	}
	rpProfileReviewProposals := []any{}
	rpProfileCommitted := []any{}
	for _, rawProfile := range sliceFromAny(out["rp_character_profile"]) {
		profile := mapFromAny(rawProfile)
		if stringFromMap(profile, "admission_state") == "committed" {
			rpProfileCommitted = append(rpProfileCommitted, profile)
			continue
		}
		rpProfileReviewProposals = append(rpProfileReviewProposals, profile)
	}
	out["rp_character_profile"] = rpProfileCommitted
	rpProfileQuarantine := quarantineReviewRPProfileIdentitySurfaces(out, rpProfileReviewProposals)
	for lane, count := range rpProfileQuarantine {
		reasons["rp_profile_unverified_identity_quarantined:"+lane] += count
	}
	for _, lane := range []string{"narrative_events", "state_claims", "belief_updates", "subjective_entity_memories"} {
		out[lane] = removeLegacyRelationshipItems(out[lane], reject, lane)
	}
	var kgReviewProposals []any
	out["kg_triples"], kgReviewProposals = admitSourceBoundKGTriples(out["kg_triples"], source, reject)

	// CharacterState.relationships is a legacy current-state write without the
	// typed source->target/domain provenance required here. Preserve every
	// other character delta field and strip only this unsafe relation field.
	characterDeltas := []any{}
	for _, rawItem := range sliceFromAny(out["character_deltas"]) {
		item := mapFromAny(rawItem)
		if _, present := item["relationships"]; present {
			delete(item, "relationships")
			reject("legacy_character_relationship_current_write_blocked")
		}
		events := []any{}
		for _, rawEvent := range sliceFromAny(item["events"]) {
			event := mapFromAny(rawEvent)
			if structurallyTypedRelationshipCandidate(event) {
				reject("legacy_character_relationship_event_blocked")
				continue
			}
			events = append(events, rawEvent)
		}
		if _, present := item["events"]; present {
			item["events"] = events
		}
		characterDeltas = append(characterDeltas, item)
	}
	out["character_deltas"] = characterDeltas

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
		"rp_profile_quarantine":         rpProfileQuarantine,
		"kg_review_proposals":           kgReviewProposals,
		"reasons":                       reasonPayload,
	}
}

func removeLegacyRelationshipItems(raw any, reject func(string), lane string) []any {
	out := []any{}
	for _, rawItem := range sliceFromAny(raw) {
		item := mapFromAny(rawItem)
		if structurallyTypedRelationshipCandidate(item) {
			reject("legacy_relationship_candidate_blocked:" + lane)
			continue
		}
		out = append(out, rawItem)
	}
	return out
}

func validNonRelationshipKGSemanticClass(item map[string]any) bool {
	semanticClass := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "semantic_class"),
		stringFromMap(item, "candidate_class"),
	)))
	switch semanticClass {
	case "entity_fact", "event_fact", "state_fact", "world_fact", "identity_fact", "location_fact", "item_fact":
		return true
	default:
		return false
	}
}

func admitSourceBoundKGTriples(raw any, source string, reject func(string)) ([]any, []any) {
	committed := []any{}
	review := []any{}
	for _, rawItem := range sliceFromAny(raw) {
		item := mapFromAny(rawItem)
		block := func(reason string) {
			reject(reason)
			review = append(review, rawItem)
		}
		if structurallyTypedRelationshipCandidate(item) {
			block("legacy_relationship_candidate_blocked:kg_triples")
			continue
		}
		if !validNonRelationshipKGSemanticClass(item) {
			block("untyped_or_unknown_kg_semantic_class_blocked")
			continue
		}
		evidence := interactionAdmissionEvidence(item)
		if !criticEvidenceOccursInSource(evidence, source) {
			block("kg_exact_current_evidence_required")
			continue
		}
		subject := strings.TrimSpace(stringFromMap(item, "subject"))
		predicate := strings.TrimSpace(stringFromMap(item, "predicate"))
		object := strings.TrimSpace(stringFromMap(item, "object"))
		predicateExpression := strings.TrimSpace(stringFromMap(item, "predicate_expression"))
		if subject == "" || predicate == "" || object == "" ||
			!interactionExactValueExpression(predicate, predicateExpression, evidence) {
			block("kg_predicate_expression_unbound")
			continue
		}
		subjectBinding, subjectOK := normalizeKGEndpointBinding(item["subject_binding"], subject, evidence)
		objectBinding, objectOK := normalizeKGEndpointBinding(item["object_binding"], object, evidence)
		if !subjectOK || !objectOK {
			block("kg_endpoint_binding_unresolved")
			continue
		}
		// Generic KG is not an authority for entity-to-entity semantics. Even
		// reviewed IDs only prove endpoint identity, not the meaning of an edge.
		if stringFromMap(subjectBinding, "endpoint_kind") != "entity" ||
			stringFromMap(objectBinding, "endpoint_kind") != "scalar" {
			block("generic_entity_edge_requires_dedicated_typed_lane")
			continue
		}
		committed = append(committed, map[string]any{
			"semantic_class":       strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "semantic_class"), stringFromMap(item, "candidate_class")))),
			"subject":              subject,
			"subject_binding":      subjectBinding,
			"predicate":            predicate,
			"predicate_expression": predicateExpression,
			"object":               object,
			"object_binding":       objectBinding,
			"evidence_excerpt":     evidence,
			"valid_from":           item["valid_from"],
			"valid_to":             item["valid_to"],
		})
	}
	return committed, review
}

func normalizeKGEndpointBinding(raw any, endpointValue, evidence string) (map[string]any, bool) {
	binding := mapFromAny(raw)
	if stringFromMap(binding, "contract_version") != kgEndpointBindingContract {
		return nil, false
	}
	kind := strings.ToLower(strings.TrimSpace(stringFromMap(binding, "endpoint_kind")))
	expression := strings.TrimSpace(stringFromMap(binding, "expression"))
	if !interactionExactValueExpression(endpointValue, expression, evidence) {
		return nil, false
	}
	result := map[string]any{
		"contract_version": kgEndpointBindingContract,
		"endpoint_kind":    kind,
		"expression":       expression,
	}
	switch kind {
	case "entity":
		entityKind := strings.ToLower(strings.TrimSpace(stringFromMap(binding, "entity_kind")))
		switch entityKind {
		case "character", "location", "item", "group", "event", "world":
			result["entity_kind"] = entityKind
		default:
			return nil, false
		}
	case "scalar":
		scalarType := strings.ToLower(strings.TrimSpace(stringFromMap(binding, "scalar_type")))
		switch scalarType {
		case "string", "number", "boolean", "state", "status", "quantity", "time":
			result["scalar_type"] = scalarType
		default:
			return nil, false
		}
	default:
		return nil, false
	}
	return result, true
}

func structurallyTypedRelationshipCandidate(item map[string]any) bool {
	contract := strings.ToLower(strings.TrimSpace(stringFromMap(item, "contract_version")))
	if contract == relationshipObservationContract {
		return true
	}
	semanticClass := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "semantic_class"),
		stringFromMap(item, "candidate_class"),
		stringFromMap(item, "observation_class"),
		stringFromMap(item, "memory_class"),
	)))
	switch semanticClass {
	case "relationship", "relationship_observation", "directional_relationship":
		return true
	}
	if legacyRelationshipShiftToken(extractionFirstNonEmpty(
		stringFromMap(item, "event_type"),
		stringFromMap(item, "type"),
	)) {
		return true
	}
	domain := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "relation_domain"),
		stringFromMap(item, "relationship_domain"),
		stringFromMap(item, "relationship_type"),
	)))
	if !relationshipObservationDomains[domain] {
		return false
	}
	hasSource := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "source_entity"), stringFromMap(item, "actor"),
		stringFromMap(item, "owner"), stringFromMap(item, "subject"),
	)) != ""
	hasTarget := strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(item, "target_entity"), stringFromMap(item, "counterpart"),
		stringFromMap(item, "target"), stringFromMap(item, "object"),
	)) != ""
	return hasSource && hasTarget
}

func legacyRelationshipShiftToken(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "relationship_shift", "relationship_change", "relationship_state", "bond_change":
		return true
	default:
		return false
	}
}

func normalizeRelationshipObservation(item map[string]any, source, sourceHash string, entitySurfaces map[string]map[string]bool) (map[string]any, string) {
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
	if !relationshipObservationDomains[domain] {
		return nil, "domain_invalid"
	}
	if observation == "" {
		return nil, "observation_missing"
	}
	switch supportKind {
	case "explicit_statement", "explicit_narrated_change", "explicit_observed_state":
	default:
		return nil, "explicit_support_kind_required"
	}
	if !criticEvidenceOccursInSource(evidence, source) {
		return nil, "exact_current_evidence_required"
	}
	if !interactionExplicitExpressionOccursInEvidence(observation, evidence) {
		return nil, "relationship_observation_not_explicit_in_evidence"
	}
	if !interactionEntityExpressionBound(sourceEntity, sourceExpression, evidence, entitySurfaces) ||
		!interactionEntityExpressionBound(targetEntity, targetExpression, evidence, entitySurfaces) {
		return nil, "relationship_entity_expression_unbound"
	}
	if !interactionExplicitExpressionOccursInEvidence(domainExpression, evidence) ||
		!interactionExplicitExpressionOccursInEvidence(domainExpression, observation) {
		return nil, "relationship_domain_expression_unbound"
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
		"admission_state":          "committed",
		"review_state":             "source_observed",
		"visibility":               visibility,
		"visibility_disposition":   visibilityDisposition,
		"evidence_excerpt":         evidence,
		"source_hash":              sourceHash,
	}
	if len(visibilitySupport) > 0 {
		normalized["public_visibility_support"] = visibilitySupport
	}
	if expression := strings.TrimSpace(stringFromMap(item, "magnitude_expression")); interactionExplicitExpressionOccursInEvidence(expression, evidence) {
		normalized["magnitude"] = normalizePreciseMemoryValue(item["magnitude"])
		normalized["magnitude_expression"] = expression
	}
	if expression := strings.TrimSpace(stringFromMap(item, "duration_expression")); interactionExplicitExpressionOccursInEvidence(expression, evidence) {
		normalized["duration"] = normalizePreciseMemoryValue(item["duration"])
		normalized["duration_expression"] = expression
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
	if !interactionExplicitExpressionOccursInEvidence(expression, evidence) {
		return false
	}
	entityKey := comparableEntityKey(entity)
	expressionKey := comparableEntityKey(expression)
	if entityKey == "" || expressionKey == "" {
		return false
	}
	if entityKey == expressionKey {
		return true
	}
	known, exists := surfaces[entityKey]
	return exists && known != nil && known[expressionKey]
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
		"interaction_events", "relationship_observations", "interaction_boundaries", "rp_character_profile",
		"narrative_events", "state_claims", "belief_updates", "kg_triples", "reversible_states",
		"world_rules", "subjective_entity_memories", "protected_secrets",
		"character_identity_accuracy", "persona_capsule_candidates",
	} {
		items := sliceFromAny(extraction[lane])
		safe := make([]any, 0, len(items))
		for _, raw := range items {
			if interactionCandidateSharesEvidence(raw, evidenceSpans) {
				counts[lane]++
				continue
			}
			safe = append(safe, raw)
		}
		extraction[lane] = safe
	}

	characterDeltas := sliceFromAny(extraction["character_deltas"])
	safeCharacters := make([]any, 0, len(characterDeltas))
	for _, raw := range characterDeltas {
		item := mapFromAny(raw)
		if interactionCandidateSharesEvidence(item, evidenceSpans) {
			counts["character_deltas"]++
			continue
		}
		events := sliceFromAny(item["events"])
		if len(events) > 0 {
			safeEvents := make([]any, 0, len(events))
			for _, event := range events {
				if interactionCandidateSharesEvidence(event, evidenceSpans) {
					counts["character_deltas.events"]++
					continue
				}
				safeEvents = append(safeEvents, event)
			}
			item["events"] = safeEvents
		}
		safeCharacters = append(safeCharacters, item)
	}
	extraction["character_deltas"] = safeCharacters

	worldState := mapFromAny(extraction["world_state"])
	if len(worldState) > 0 {
		if interactionCandidateSharesEvidence(worldState, evidenceSpans) {
			counts["world_state"]++
			extraction["world_state"] = map[string]any{}
		} else {
			rules := sliceFromAny(worldState["rules"])
			safeRules := make([]any, 0, len(rules))
			for _, rule := range rules {
				if interactionCandidateSharesEvidence(rule, evidenceSpans) {
					counts["world_state.rules"]++
					continue
				}
				safeRules = append(safeRules, rule)
			}
			worldState["rules"] = safeRules
			extraction["world_state"] = worldState
		}
	}

	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	safeExcerpts := make([]string, 0, len(excerpts))
	for _, excerpt := range excerpts {
		if interactionEvidenceOverlapsAny(excerpt, evidenceSpans) {
			counts["evidence_excerpts"]++
			continue
		}
		safeExcerpts = append(safeExcerpts, excerpt)
	}
	extraction["evidence_excerpts"] = safeExcerpts
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
		if !criticEvidenceOccursInSource(evidence, source) {
			reject("user_profile_exact_current_evidence_required")
			continue
		}
		if !interactionExplicitExpressionOccursInEvidence(profileKeyExpression, evidence) ||
			!interactionExplicitExpressionOccursInEvidence(valueExpression, evidence) {
			reject("user_profile_expression_unbound")
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
		if character == "" || profileKey == "" || value == "" {
			reject("rp_profile_character_key_or_value_missing")
			continue
		}
		if !criticEvidenceOccursInSource(evidence, source) {
			reject("rp_profile_exact_current_evidence_required")
			continue
		}
		if !interactionEntityExpressionBound(character, characterExpression, evidence, entitySurfaces) {
			reject("rp_profile_character_expression_unbound")
			continue
		}
		if !interactionExactValueExpression(value, valueExpression, evidence) {
			reject("rp_profile_value_expression_unbound")
			continue
		}
		identityProof, proofValid := normalizeRPCharacterIdentityProof(item, character, characterExpression, evidence, stableIdentities)
		admissionState := "review_required"
		reviewState := "stable_in_world_identity_unverified"
		if proofValid {
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

func quarantineReviewRPProfileIdentitySurfaces(extraction map[string]any, profiles []any) map[string]int {
	surfaces := map[string]bool{}
	for _, raw := range profiles {
		profile := mapFromAny(raw)
		for _, surface := range []string{stringFromMap(profile, "character"), stringFromMap(profile, "character_expression")} {
			if key := comparableEntityKey(surface); key != "" {
				surfaces[key] = true
			}
		}
	}
	if len(surfaces) == 0 {
		return nil
	}
	counts := map[string]int{}
	entities := mapFromAny(extraction["entities"])
	characters := []any{}
	for _, raw := range sliceFromAny(entities["characters"]) {
		item := mapFromAny(raw)
		name := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "name"), stringFromMap(item, "label"), stringFromMap(item, "title")))
		matched := surfaces[comparableEntityKey(name)]
		for _, alias := range stringsFromAny(item["aliases"]) {
			matched = matched || surfaces[comparableEntityKey(alias)]
		}
		if matched {
			counts["entities.characters"]++
			continue
		}
		characters = append(characters, raw)
	}
	entities["characters"] = characters
	extraction["entities"] = entities

	for _, lane := range []struct {
		key      string
		nameKeys []string
	}{
		{key: "character_deltas", nameKeys: []string{"name", "character", "entity"}},
		{key: "speaker_attributions", nameKeys: []string{"speaker_name", "speaker"}},
	} {
		safe := []any{}
		for _, raw := range sliceFromAny(extraction[lane.key]) {
			item := mapFromAny(raw)
			matched := false
			for _, key := range lane.nameKeys {
				matched = matched || surfaces[comparableEntityKey(stringFromMap(item, key))]
			}
			if matched {
				counts[lane.key]++
				continue
			}
			safe = append(safe, raw)
		}
		extraction[lane.key] = safe
	}
	return counts
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
	visibility := normalizeInteractionVisibilityWithDefault(stringFromMap(item, "visibility"), fallback)
	if visibility != "public" {
		return visibility, nil, "private_or_restricted"
	}
	support := mapFromAny(item["public_visibility_support"])
	supportKind := strings.ToLower(strings.TrimSpace(stringFromMap(support, "support_kind")))
	assertion := strings.TrimSpace(stringFromMap(support, "visibility_assertion"))
	validKind := supportKind == "explicit_public_statement" || supportKind == "explicit_public_narration" || supportKind == "explicit_public_observation"
	if stringFromMap(support, "contract_version") == publicVisibilitySupportContract && validKind &&
		normalizeArtifactComparableText(assertion) != "" &&
		normalizeArtifactComparableText(assertion) == normalizeArtifactComparableText(evidence) {
		return "public", map[string]any{
			"contract_version":     publicVisibilitySupportContract,
			"support_kind":         supportKind,
			"visibility_assertion": assertion,
		}, "source_observed_public"
	}
	return normalizeInteractionVisibilityWithDefault(fallback, "owner_private"), nil, "public_downgraded_unbound"
}

func validInteractionBoundaryDecision(value string) bool {
	switch value {
	case "allow", "refuse", "withdrawn", "unknown":
		return true
	default:
		return false
	}
}

func validInteractionBoundarySupportKind(value string) bool {
	switch value {
	case "explicit_statement", "explicit_narrated_boundary", "explicit_observed_boundary":
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
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || evidence == "" {
			continue
		}
		out = append(out, preciseMemoryCandidate{
			kind: "event", subtype: "atomic_interaction", excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "actor", "actor_expression", "counterpart", "counterpart_expression", "action", "action_expression",
				"interaction_kind", "visibility", "source_hash",
			}),
			confidence: 1, truthScope: "source_occurrence", epistemicMode: "direct",
			authorityClass: "objective_world_state", admissionState: "committed",
			reviewState: "source_observed", visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			surfaces:      map[string]string{"actor": actor, "affected": counterpart},
			requiredRoles: map[string]bool{"actor": true, "affected": true},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["relationship_observations"]) {
		item := mapFromAny(rawItem)
		sourceEntity := stringFromMap(item, "source_entity")
		targetEntity := stringFromMap(item, "target_entity")
		domain := stringFromMap(item, "domain")
		evidence := interactionAdmissionEvidence(item)
		if sourceEntity == "" || targetEntity == "" || !relationshipObservationDomains[domain] || evidence == "" {
			continue
		}
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: "relationship_" + domain, excerpt: evidence,
			payload: preciseMemorySemanticPayload(item, []string{
				"contract_version", "source_entity", "source_entity_expression", "target_entity", "target_entity_expression", "domain", "domain_expression",
				"observation", "support_kind", "magnitude", "magnitude_expression",
				"duration", "duration_expression",
				"admission_state", "review_state",
				"visibility", "public_visibility_support", "visibility_disposition", "source_hash",
			}),
			confidence: 1, truthScope: "source_scoped", epistemicMode: "direct",
			authorityClass: "subjective_episodic", admissionState: extractionFirstNonEmpty(stringFromMap(item, "admission_state"), "committed"),
			reviewState: extractionFirstNonEmpty(stringFromMap(item, "review_state"), "source_observed"), visibility: normalizeInteractionVisibility(stringFromMap(item, "visibility")),
			relationshipKey: comparableEntityKey(sourceEntity) + "->" + comparableEntityKey(targetEntity) + "/" + domain,
			surfaces:        map[string]string{"actor": sourceEntity, "affected": targetEntity},
			requiredRoles:   map[string]bool{"actor": true, "affected": true},
		})
	}
	for _, rawItem := range sliceFromAny(extraction["interaction_boundaries"]) {
		item := mapFromAny(rawItem)
		actor := stringFromMap(item, "actor")
		counterpart := stringFromMap(item, "counterpart")
		evidence := interactionAdmissionEvidence(item)
		if actor == "" || counterpart == "" || evidence == "" {
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
		if evidence == "" {
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
		if character == "" || evidence == "" {
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
