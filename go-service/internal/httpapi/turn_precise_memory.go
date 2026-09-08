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

const preciseMemoryObjectiveConfidence = 0.7

type preciseMemoryCandidate struct {
	kind            string
	subtype         string
	excerpt         string
	payload         map[string]any
	confidence      float64
	truthScope      string
	epistemicMode   string
	authorityClass  string
	admissionState  string
	reviewState     string
	visibility      string
	revealCondition string
	relationshipKey string
	surfaces        map[string]string
	requiredRoles   map[string]bool
	resolvedIDs     map[string]string
	participants    []string
}

// appendPreciseMemoryEvidenceExcerpts adds only source-bound utterance spans
// that the existing narrative-state helper does not already add. It is gated
// by accepted-source context, so legacy/rescan paths retain their old evidence
// projection.
func appendPreciseMemoryEvidenceExcerpts(ctx context.Context, extraction map[string]any) map[string]any {
	if extraction == nil {
		return extraction
	}
	source, ok := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !ok || source.ContractVersion != completeTurnSourceAcceptanceContract || strings.TrimSpace(source.Revision) == "" {
		return extraction
	}
	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	seen := map[string]bool{}
	for _, excerpt := range excerpts {
		seen[normalizeArtifactDedupeText(excerpt)] = true
	}
	for _, raw := range sliceFromAny(extraction["speaker_attributions"]) {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "evidence_excerpt"),
			stringFromMap(item, "source_excerpt"),
		))
		key := normalizeArtifactDedupeText(excerpt)
		if excerpt == "" || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		excerpts = append(excerpts, excerpt)
	}
	for _, lane := range []string{
		"interaction_events",
		"relationship_observations",
		"interaction_boundaries",
		"habit_observations",
		"character_profile_observations",
		"voice_observations",
		"user_interaction_profile",
		"rp_character_profile",
	} {
		for _, raw := range sliceFromAny(extraction[lane]) {
			excerpt := interactionAdmissionEvidence(mapFromAny(raw))
			key := normalizeArtifactDedupeText(excerpt)
			if excerpt == "" || key == "" || seen[key] {
				continue
			}
			seen[key] = true
			excerpts = append(excerpts, excerpt)
		}
	}
	extraction["evidence_excerpts"] = excerpts
	return extraction
}

func (s *Server) savePreciseMemoryUnitsFromExtraction(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	evidence []store.DirectEvidence,
	identities *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	writer, ok := s.Store.(store.PreciseMemoryWriter)
	if !ok {
		return
	}
	if availability, ok := s.Store.(store.PreciseMemoryWriteAvailability); ok && !availability.PreciseMemoryWritesEnabled() {
		return
	}
	units := s.buildPreciseMemoryUnitsFromExtraction(
		ctx, sid, turnIndex, extraction, content, evidence, identities, now, result,
	)
	savedUnits := make([]*store.PreciseMemoryUnit, 0, len(units))
	for _, unit := range units {
		result.Attempted++
		inserted, err := writer.SavePreciseMemoryUnit(ctx, unit)
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SavePreciseMemoryUnit: "+err.Error())
			continue
		}
		savedUnits = append(savedUnits, unit)
		if inserted {
			result.PreciseMemoryUnits++
		} else {
			result.addSkipReason("precise_memory_units", "idempotent_replay", map[string]any{
				"kind": unit.Kind, "idempotency_key": unit.IdempotencyKey,
			})
		}
	}
	s.savePostAdmissionPreciseMemoryProjections(ctx, sid, savedUnits, now, result)
}

func (s *Server) buildPreciseMemoryUnitsFromExtraction(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	evidence []store.DirectEvidence,
	identities *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) []*store.PreciseMemoryUnit {
	units := []*store.PreciseMemoryUnit{}
	source, accepted := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	if !accepted ||
		source.ContractVersion != completeTurnSourceAcceptanceContract ||
		strings.TrimSpace(source.Revision) == "" {
		if result != nil {
			result.addSkipReason("precise_memory_units", "accepted_current_source_required", nil)
		}
		return units
	}
	source.ContentHash = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	candidates := preciseMemoryCandidates(extraction)
	for _, candidate := range candidates {
		spanStart, spanEnd, exact := preciseMemoryExactSpan(candidate.payload, candidate.excerpt, content)
		if !exact {
			if result != nil {
				result.addSkipReason("precise_memory_units", "exact_unique_source_span_required", map[string]any{
					"kind": candidate.kind, "excerpt": candidate.excerpt,
				})
			}
			continue
		}
		evidenceIDs := preciseMemoryExactEvidenceIDs(evidence, sid, turnIndex, candidate.excerpt)
		if len(evidenceIDs) == 0 {
			if result != nil {
				result.addSkipReason("precise_memory_units", "accepted_direct_evidence_required", map[string]any{
					"kind": candidate.kind, "source_span_start": spanStart, "source_span_end": spanEnd,
				})
			}
			continue
		}
		candidate.applyIdentityPointers(identities, spanStart, spanEnd)
		candidate.resolveReviewedIdentityPointers(ctx, s.Store, sid)
		candidate.applyPerspectivePayloadIdentityPointers()
		payloadJSON := mustCompactJSON(normalizePreciseMemoryValue(candidate.payload))
		roleSurfaceJSON := mustCompactJSON(normalizePreciseMemoryValue(candidate.surfaces))
		keyMaterial := strings.Join([]string{
			source.Revision,
			candidate.kind,
			fmt.Sprintf("%d:%d", spanStart, spanEnd),
			normalizeArtifactComparableText(payloadJSON),
			normalizeArtifactComparableText(roleSurfaceJSON),
			candidate.truthScope,
			candidate.epistemicMode,
			candidate.authorityClass,
			candidate.admissionState,
			candidate.reviewState,
			candidate.visibility,
		}, "\x1f")
		idempotencyKey := fmt.Sprintf("%x", sha256.Sum256([]byte(keyMaterial)))
		evidenceHash := fmt.Sprintf("%x", sha256.Sum256([]byte(candidate.excerpt)))
		directEvidenceJSON := mustCompactJSON(evidenceIDs)
		sourceRole := strings.TrimSpace(source.SourceRole)
		if sourceRole == "" {
			sourceRole = "combined_turn_pair"
		}
		unit := &store.PreciseMemoryUnit{
			UnitID:                preciseMemoryStableID(sid, idempotencyKey),
			ContractVersion:       store.PreciseMemoryUnitContract,
			ChatSessionID:         sid,
			SourceTurnStart:       turnIndex,
			SourceTurnEnd:         turnIndex,
			SourceContract:        source.ContractVersion,
			SourceRevision:        source.Revision,
			SourceLogicalTurnID:   source.LogicalTurnID,
			SourceMessageID:       source.MessageID,
			SourceGenerationID:    source.GenerationID,
			SourceContentHash:     source.ContentHash,
			SourceRole:            sourceRole,
			SourceSpanStart:       spanStart,
			SourceSpanEnd:         spanEnd,
			EvidenceExcerpt:       candidate.excerpt,
			EvidenceHash:          evidenceHash,
			RootEvidenceID:        evidenceIDs[0],
			DirectEvidenceIDsJSON: directEvidenceJSON,
			Kind:                  candidate.kind,
			Subtype:               candidate.subtype,
			PayloadJSON:           payloadJSON,
			RelationshipKey:       candidate.relationshipKey,
			TruthScope:            candidate.truthScope,
			EpistemicMode:         candidate.epistemicMode,
			AuthorityClass:        candidate.authorityClass,
			AdmissionState:        candidate.admissionState,
			ReviewState:           candidate.reviewState,
			Visibility:            candidate.visibility,
			RevealCondition:       candidate.revealCondition,
			Confidence:            candidate.confidence,
			IdempotencyKey:        idempotencyKey,
			DerivationVersion:     store.PreciseMemoryUnitContract,
			ExtractorVersion:      "complete_turn.configured_critic_extract",
			IndexVersion:          "not_materialized",
			LifecycleState:        "active",
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		candidate.assignIdentityPointers(unit)
		units = append(units, unit)
	}
	return units
}

func preciseMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	for _, raw := range sliceFromAny(extraction["narrative_events"]) {
		item := mapFromAny(raw)
		summary := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "event")))
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		if summary == "" || excerpt == "" {
			continue
		}
		candidate := preciseMemoryCandidate{
			kind: "event", subtype: preciseMemorySubtype(item, "event_type", "observed_event"),
			excerpt: excerpt, confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			payload: preciseMemorySemanticPayload(item, []string{
				"summary", "event", "event_type", "actor", "actor_name",
				"affected_entity", "target", "location", "object", "participants",
				"relationship_key", "claim_scope", "epistemic_mode", "modality",
				"truth_scope", "truth_status", "event_status", "statement_type",
				"is_lie", "is_deception", "known_false", "is_uncertain",
				"is_speculation", "speculative", "is_proposal", "proposed",
				"hypothetical", "is_ooc", "ooc", "out_of_character",
				"source_span_start", "source_span_end",
			}),
			truthScope: "objective", epistemicMode: "direct", authorityClass: "objective_world_state",
			admissionState: "committed", reviewState: "source_observed", visibility: "public",
			relationshipKey: strings.TrimSpace(stringFromMap(item, "relationship_key")),
			surfaces: map[string]string{
				"actor":    extractionFirstNonEmpty(stringFromMap(item, "actor"), stringFromMap(item, "actor_name")),
				"affected": extractionFirstNonEmpty(stringFromMap(item, "affected_entity"), stringFromMap(item, "target")),
				"location": stringFromMap(item, "location"),
				"object":   stringFromMap(item, "object"),
			},
			requiredRoles: map[string]bool{},
			participants:  preciseMemoryParticipantSurfaces(item["participants"]),
		}
		for role, surface := range candidate.surfaces {
			candidate.requiredRoles[role] = strings.TrimSpace(surface) != ""
		}
		candidate.applySemanticAuthority(item)
		out = append(out, candidate)
	}
	for _, raw := range sliceFromAny(extraction["state_claims"]) {
		item := mapFromAny(raw)
		subject := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "subject"), stringFromMap(item, "entity"), stringFromMap(item, "owner"),
		))
		stateSlot := normalizeNarrativeStateSlot(extractionFirstNonEmpty(
			stringFromMap(item, "state_slot"), stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"),
		))
		value := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "value"), stringFromMap(item, "state_value"), stringFromMap(item, "belief"),
		))
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		if subject == "" || stateSlot == "" || value == "" || excerpt == "" {
			continue
		}
		subjectType := normalizeNarrativeSubjectType(stringFromMap(item, "subject_type"))
		if subjectType == "" {
			subjectType = "entity"
		}
		candidate := preciseMemoryCandidate{
			kind: "state", subtype: stateSlot, excerpt: excerpt,
			confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			payload: preciseMemorySemanticPayload(item, []string{
				"subject", "entity", "owner", "subject_type", "state_slot", "slot",
				"relation_dimension", "value", "state_value", "belief", "claim_scope",
				"transition", "epistemic_mode", "modality", "truth_scope",
				"truth_status", "statement_type", "is_lie", "is_deception",
				"known_false", "is_uncertain", "is_speculation", "speculative",
				"is_proposal", "proposed", "hypothetical", "is_ooc", "ooc",
				"out_of_character", "visibility", "source_span_start", "source_span_end",
			}),
			truthScope: "objective", epistemicMode: "direct", authorityClass: "objective_world_state",
			admissionState: "committed", reviewState: "source_observed",
			visibility:      preciseMemoryVisibility(item, "public"),
			relationshipKey: strings.TrimSpace(stringFromMap(item, "relationship_key")),
			surfaces:        map[string]string{"subject": subject},
			requiredRoles:   map[string]bool{"subject": preciseMemorySubjectNeedsIdentity(subjectType)},
		}
		candidate.applySemanticAuthority(item)
		out = append(out, candidate)
	}
	out = append(out, perspectiveMemoryCandidates(extraction)...)
	out = append(out, interactionAdmissionPreciseMemoryCandidates(extraction)...)
	for _, raw := range sliceFromAny(extraction["speaker_attributions"]) {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "source_excerpt")))
		if excerpt == "" {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(stringFromMap(item, "attribution_state")))
		candidate := preciseMemoryCandidate{
			kind: "utterance", subtype: preciseMemorySubtype(item, "attribution_kind", "unknown"),
			excerpt: excerpt, confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0), 0, 1),
			payload: preciseMemorySemanticPayload(item, []string{
				"speaker_name", "speaker", "attribution_kind", "attribution_state",
				"source_span_start", "source_span_end",
			}),
			truthScope: "source_occurrence", epistemicMode: "utterance_content_unverified",
			authorityClass: "objective_world_state", admissionState: "committed",
			reviewState: "source_observed", visibility: "public",
			surfaces: map[string]string{
				"actor": extractionFirstNonEmpty(stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker")),
			},
			requiredRoles: map[string]bool{"actor": true},
		}
		if state != "linked" {
			candidate.markNeedsReview()
		}
		out = append(out, candidate)
	}
	return out
}

type perspectiveHolderProposal struct {
	surface          string
	epistemicState   string
	acquisitionMode  string
	stateWasExplicit bool
	stateConflict    bool
}

func perspectiveMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	for _, raw := range sliceFromAny(extraction["belief_updates"]) {
		item := mapFromAny(raw)
		subject := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "subject"), stringFromMap(item, "entity"), stringFromMap(item, "owner"),
		))
		stateSlot := normalizeNarrativeStateSlot(extractionFirstNonEmpty(
			stringFromMap(item, "state_slot"), stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"),
		))
		value := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "value"), stringFromMap(item, "state_value"), stringFromMap(item, "belief"),
		))
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		if subject == "" || stateSlot == "" || value == "" || excerpt == "" {
			continue
		}
		subjectType := normalizeNarrativeSubjectType(stringFromMap(item, "subject_type"))
		if subjectType == "" {
			subjectType = "entity"
		}
		defaultState, stateExplicit := normalizePerspectiveMemoryState(extractionFirstNonEmpty(
			stringFromMap(item, "epistemic_state"), stringFromMap(item, "knowledge_state"),
			stringFromMap(item, "epistemic_mode"),
		))
		if defaultState == "" {
			defaultState = "unknown"
		}
		holders := perspectiveMemoryHolderProposals(item, defaultState, stateExplicit)
		if len(holders) == 0 {
			holders = []perspectiveHolderProposal{{epistemicState: defaultState, stateWasExplicit: stateExplicit}}
		}
		speaker := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "speaker_name"), stringFromMap(item, "speaker"), stringFromMap(item, "actor"),
		))
		for _, holder := range holders {
			payload := preciseMemorySemanticPayload(item, []string{
				"subject", "entity", "owner", "subject_type", "state_slot", "slot",
				"relation_dimension", "value", "state_value", "belief", "claim_scope",
				"speaker_name", "speaker", "actor",
				"transition", "epistemic_state", "knowledge_state", "epistemic_mode",
				"acquisition_mode", "modality", "truth_scope", "truth_status",
				"statement_type", "visibility", "reveal_condition",
				"source_span_start", "source_span_end",
			})
			payload["contract_version"] = "perspective_memory.v1"
			payload["epistemic_state"] = holder.epistemicState
			payload["knowledge_holder"] = holder.surface
			payload["subject"] = subject
			payload["state_slot"] = stateSlot
			payload["claim"] = value
			if holder.acquisitionMode != "" {
				payload["acquisition_mode"] = holder.acquisitionMode
			}
			candidate := preciseMemoryCandidate{
				kind: "observation", subtype: stateSlot, excerpt: excerpt, payload: payload,
				confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
				truthScope: "owner_scoped", epistemicMode: holder.epistemicState,
				authorityClass: "subjective_episodic", admissionState: "committed",
				reviewState: "source_observed", visibility: perspectiveMemoryVisibility(holder.epistemicState),
				revealCondition: strings.TrimSpace(stringFromMap(item, "reveal_condition")),
				surfaces: map[string]string{
					"subject": subject,
					"knower":  holder.surface,
					"actor":   speaker,
				},
				requiredRoles: map[string]bool{
					"subject": preciseMemorySubjectNeedsIdentity(subjectType),
					"knower":  true,
					"actor":   speaker != "",
				},
			}
			if holder.stateConflict ||
				!holder.stateWasExplicit ||
				!perspectiveMemoryRevealTransitionValid(holder.epistemicState, stringFromMap(item, "transition")) {
				candidate.markNeedsReview()
			}
			out = append(out, candidate)
		}
	}
	out = append(out, protectedSecretPerspectiveMemoryCandidates(extraction)...)
	out = append(out, subjectivePerspectiveMemoryCandidates(extraction)...)
	return out
}

func perspectiveMemoryHolderProposals(item map[string]any, defaultState string, stateExplicit bool) []perspectiveHolderProposal {
	out := []perspectiveHolderProposal{}
	stateIndexesByHolder := map[string]map[string]int{}
	add := func(surface, state, acquisition string, explicit bool) {
		surface = strings.TrimSpace(surface)
		if surface == "" {
			return
		}
		normalizedState, valid := normalizePerspectiveMemoryState(state)
		if !valid {
			normalizedState = defaultState
		}
		key := comparableEntityKey(surface)
		if key == "" {
			return
		}
		if stateIndexesByHolder[key] == nil {
			stateIndexesByHolder[key] = map[string]int{}
		}
		if _, duplicate := stateIndexesByHolder[key][normalizedState]; duplicate {
			return
		}
		conflict := len(stateIndexesByHolder[key]) > 0
		if conflict {
			for _, index := range stateIndexesByHolder[key] {
				out[index].stateConflict = true
			}
		}
		out = append(out, perspectiveHolderProposal{
			surface: surface, epistemicState: normalizedState,
			acquisitionMode:  strings.TrimSpace(acquisition),
			stateWasExplicit: explicit && valid,
			stateConflict:    conflict,
		})
		stateIndexesByHolder[key][normalizedState] = len(out) - 1
	}
	for _, key := range []string{"listener_names", "knowledge_holders", "knowers"} {
		for _, value := range stringsFromAny(item[key]) {
			add(value, defaultState, stringFromMap(item, "acquisition_mode"), stateExplicit)
		}
	}
	for _, raw := range sliceFromAny(item["listeners"]) {
		listener := mapFromAny(raw)
		if len(listener) == 0 {
			add(extractionStringFromAny(raw), defaultState, stringFromMap(item, "acquisition_mode"), stateExplicit)
			continue
		}
		state := extractionFirstNonEmpty(
			stringFromMap(listener, "epistemic_state"),
			stringFromMap(listener, "knowledge_state"),
			defaultState,
		)
		_, listenerStateExplicit := normalizePerspectiveMemoryState(extractionFirstNonEmpty(
			stringFromMap(listener, "epistemic_state"), stringFromMap(listener, "knowledge_state"),
		))
		add(extractionFirstNonEmpty(
			stringFromMap(listener, "name"), stringFromMap(listener, "listener_name"),
			stringFromMap(listener, "knowledge_holder"),
		), state, extractionFirstNonEmpty(
			stringFromMap(listener, "acquisition_mode"), stringFromMap(item, "acquisition_mode"),
		), listenerStateExplicit || stateExplicit)
	}
	if len(out) == 0 {
		add(extractionFirstNonEmpty(
			stringFromMap(item, "perspective_owner"), stringFromMap(item, "believer"),
			stringFromMap(item, "knower"), stringFromMap(item, "owner"),
			stringFromMap(item, "owner_entity_name"),
		), defaultState, stringFromMap(item, "acquisition_mode"), stateExplicit)
	}
	return out
}

func protectedSecretPerspectiveMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	items := append([]any{}, sliceFromAny(extraction["protected_secrets"])...)
	// Identity mappings carry the same per-holder knowledge contract. Feed
	// their supplied evidence through the existing protected-knowledge writer.
	for _, raw := range sliceFromAny(extraction["character_identity_accuracy"]) {
		identity := mapFromAny(raw)
		owner := extractionFirstNonEmpty(stringFromMap(identity, "canonical_entity_name"), stringFromMap(identity, "true_identity_name"))
		mapping := preciseMemorySemanticPayload(identity, []string{
			"surface_identity_name", "true_identity_name", "same_entity", "public_role", "true_role", "public_allegiance", "true_allegiance",
		})
		items = append(items, map[string]any{
			"secret_kind": extractionFirstNonEmpty(stringFromMap(identity, "identity_kind"), "identity"),
			"secret_id":   stringFromMap(identity, "identity_id"),
			"owner":       owner, "subject": owner,
			"summary":         "Identity context: " + mustCompactJSON(mapping),
			"knowledge_scope": identity["knowledge_scope"], "transition": identity["transition"],
			"disclosure_policy": identity["reveal_policy"], "evidence_excerpt": identity["evidence_excerpt"],
		})
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		value := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "secret_summary"), stringFromMap(item, "text")))
		owner := strings.TrimSpace(stringFromMap(item, "owner"))
		subject := firstStringFromAny(item["subject"])
		if subject == "" {
			subject = owner
		}
		if excerpt == "" || value == "" || subject == "" {
			continue
		}
		stateSlot := normalizeNarrativeStateSlot(extractionFirstNonEmpty(stringFromMap(item, "secret_kind"), "protected_knowledge"))
		scope := mapFromAny(item["knowledge_scope"])
		proposals := []perspectiveHolderProposal{}
		stateIndexesByHolder := map[string]map[string]int{}
		addProposal := func(holder, state string) {
			holder = strings.TrimSpace(holder)
			key := comparableEntityKey(holder)
			if key == "" {
				return
			}
			if stateIndexesByHolder[key] == nil {
				stateIndexesByHolder[key] = map[string]int{}
			}
			if _, duplicate := stateIndexesByHolder[key][state]; duplicate {
				return
			}
			conflict := false
			for existingState := range stateIndexesByHolder[key] {
				if perspectiveMemoryStateIdentity(existingState) != perspectiveMemoryStateIdentity(state) {
					conflict = true
				}
			}
			if conflict {
				for _, index := range stateIndexesByHolder[key] {
					proposals[index].stateConflict = true
				}
			}
			proposals = append(proposals, perspectiveHolderProposal{
				surface: holder, epistemicState: state,
				stateWasExplicit: true, stateConflict: conflict,
			})
			stateIndexesByHolder[key][state] = len(proposals) - 1
		}
		for _, stateScope := range []struct {
			key   string
			state string
		}{
			{key: "known_by", state: "known"},
			{key: "suspected_by", state: "suspected"},
			{key: "unknown_to", state: "unknown"},
			{key: "misinformed_by", state: "misinformed"},
			{key: "revealed_to", state: "revealed"},
		} {
			for _, holder := range stringsFromAny(scope[stateScope.key]) {
				addProposal(holder, stateScope.state)
			}
		}
		for _, proposal := range proposals {
			holder := proposal.surface
			state := proposal.epistemicState
			payload := preciseMemorySemanticPayload(item, []string{
				"secret_kind", "secret_id", "owner", "subject", "summary", "sensitivity",
				"evidence_strength", "disclosure_policy",
				"transition", "evidence_excerpt", "source_span_start", "source_span_end",
			})
			payload["contract_version"] = "perspective_memory.v1"
			payload["epistemic_state"] = state
			payload["knowledge_holder"] = holder
			payload["subject"] = subject
			payload["state_slot"] = stateSlot
			payload["claim"] = value
			candidate := preciseMemoryCandidate{
				kind: "observation", subtype: stateSlot, excerpt: excerpt, payload: payload,
				confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
				truthScope: "owner_scoped", epistemicMode: state,
				authorityClass: "subjective_episodic", admissionState: "committed",
				reviewState: "source_observed", visibility: perspectiveMemoryVisibility(state),
				revealCondition: strings.TrimSpace(stringFromMap(item, "disclosure_policy")),
				surfaces:        map[string]string{"subject": subject, "knower": holder},
				requiredRoles:   map[string]bool{"subject": true, "knower": true},
			}
			// A protected-secret summary is the guarded truth, not the false
			// proposition a misinformed holder believes. Only belief_updates
			// can commit the holder's source-grounded misinformation claim.
			if proposal.stateConflict ||
				state == "misinformed" ||
				(state == "revealed" && !strings.EqualFold(strings.TrimSpace(stringFromMap(item, "transition")), "reveal")) {
				candidate.markNeedsReview()
			}
			out = append(out, candidate)
		}
	}
	return out
}

func subjectivePerspectiveMemoryCandidates(extraction map[string]any) []preciseMemoryCandidate {
	out := []preciseMemoryCandidate{}
	for _, raw := range sliceFromAny(extraction["subjective_entity_memories"]) {
		item := mapFromAny(raw)
		if boolFromAny(item["secret_guard"]) || stringSliceContains(stringsFromAny(item["tags"]), "protected_secret") {
			continue
		}
		holder := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "owner_entity_name"), stringFromMap(item, "entity_name"),
			stringFromMap(item, "name"), stringFromMap(item, "persona_entity_name"),
		))
		value := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(item, "memory_text"), stringFromMap(item, "subjective_memory"),
			stringFromMap(item, "recollection"), stringFromMap(item, "interpretation"),
			stringFromMap(item, "summary"), stringFromMap(item, "text"),
		))
		excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
		if holder == "" || value == "" || excerpt == "" {
			continue
		}
		payload := preciseMemorySemanticPayload(item, []string{
			"owner_entity_name", "entity_name", "name", "persona_entity_name",
			"memory_text", "subjective_memory", "recollection", "interpretation",
			"summary", "text", "evidence_excerpt", "source_span_start", "source_span_end",
		})
		payload["contract_version"] = "perspective_memory.v1"
		payload["epistemic_state"] = "known"
		payload["knowledge_holder"] = holder
		payload["subject"] = holder
		payload["state_slot"] = "subjective_memory"
		payload["claim"] = value
		out = append(out, preciseMemoryCandidate{
			kind: "observation", subtype: "subjective_memory", excerpt: excerpt, payload: payload,
			confidence: clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			truthScope: "owner_scoped", epistemicMode: "known",
			authorityClass: "subjective_episodic", admissionState: "committed",
			reviewState: "source_observed", visibility: "owner_private",
			surfaces:      map[string]string{"subject": holder, "knower": holder},
			requiredRoles: map[string]bool{"subject": true, "knower": true},
		})
	}
	return out
}

func normalizePerspectiveMemoryState(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "known", "suspected", "unknown", "misinformed", "hidden", "revealed":
		return strings.ToLower(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

// A disclosure and an existing known fact express compatible holder knowledge.
// Keep their original states in storage and display; compare their meaning here.
func perspectiveMemoryStateIdentity(state string) string {
	if state == "revealed" {
		return "known"
	}
	return state
}

func perspectiveMemoryVisibility(state string) string {
	switch state {
	case "unknown", "hidden":
		return "restricted"
	default:
		return "owner_private"
	}
}

func perspectiveMemoryRevealTransitionValid(state, transition string) bool {
	if state != "revealed" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(transition), "reveal")
}

func (candidate *preciseMemoryCandidate) applySemanticAuthority(item map[string]any) {
	classification := preciseMemoryStructuredClassification(item)
	switch classification {
	case "ooc_meta":
		candidate.truthScope = "ooc_meta"
		candidate.epistemicMode = "meta"
		candidate.authorityClass = "ooc_meta"
		candidate.visibility = "restricted"
	case "proposal":
		candidate.truthScope = "proposed"
		candidate.epistemicMode = "proposal"
		candidate.authorityClass = "support_hypothesis"
	case "deception":
		candidate.truthScope = "speaker_claim"
		candidate.epistemicMode = "deception_or_false_claim"
		candidate.authorityClass = "subjective_episodic"
	case "non_objective":
		candidate.truthScope = "non_objective"
		candidate.epistemicMode = "inferred_or_uncertain"
		candidate.authorityClass = "support_hypothesis"
	}
	if candidate.authorityClass == "objective_world_state" && candidate.confidence < preciseMemoryObjectiveConfidence {
		candidate.truthScope = "unverified"
		candidate.epistemicMode = "uncertain"
		candidate.authorityClass = "support_hypothesis"
		candidate.markNeedsReview()
	}
}

func (candidate *preciseMemoryCandidate) applyIdentityPointers(identities *entityIdentityProjection, spanStart, spanEnd int) {
	for role, required := range candidate.requiredRoles {
		surface := strings.TrimSpace(candidate.surfaces[role])
		if surface == "" {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		if identities == nil {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		id, ok := identities.preciseMemoryEntityPointer(surface, spanStart, spanEnd)
		if !ok {
			if required {
				candidate.markNeedsReview()
			}
			continue
		}
		if candidate.resolvedIDs == nil {
			candidate.resolvedIDs = map[string]string{}
		}
		candidate.resolvedIDs[role] = id
	}
	if len(candidate.participants) == 0 {
		return
	}
	participantIDs := make([]string, 0, len(candidate.participants))
	for _, surface := range candidate.participants {
		if identities == nil {
			candidate.markNeedsReview()
			continue
		}
		id, ok := identities.preciseMemoryEntityPointer(surface, spanStart, spanEnd)
		if !ok {
			candidate.markNeedsReview()
			continue
		}
		participantIDs = append(participantIDs, id)
	}
	if len(participantIDs) == len(candidate.participants) {
		sort.Strings(participantIDs)
		candidate.payload["participant_entity_ids"] = participantIDs
	}
}

func (candidate *preciseMemoryCandidate) resolveReviewedIdentityPointers(ctx context.Context, candidateStore store.Store, sid string) {
	resolver, ok := candidateStore.(store.ReviewedEntityIdentityResolver)
	if !ok || len(candidate.resolvedIDs) == 0 {
		return
	}
	for role, sourceEntityID := range candidate.resolvedIDs {
		canonicalEntityID, err := resolver.ResolveReviewedCanonicalEntityID(ctx, sid, sourceEntityID)
		switch {
		case err == nil && strings.TrimSpace(canonicalEntityID) != "":
			candidate.resolvedIDs[role] = strings.TrimSpace(canonicalEntityID)
		case errors.Is(err, store.ErrNotFound):
			// One active source-observed occurrence is already a stable ID.
		case err != nil:
			delete(candidate.resolvedIDs, role)
			if candidate.requiredRoles[role] {
				candidate.markNeedsReview()
			}
		}
	}
}

func (candidate *preciseMemoryCandidate) applyPerspectivePayloadIdentityPointers() {
	if extractionStringFromAny(candidate.payload["contract_version"]) != "perspective_memory.v1" {
		return
	}
	if id := strings.TrimSpace(candidate.resolvedIDs["knower"]); id != "" {
		candidate.payload["knowledge_holder_entity_id"] = id
	}
	if id := strings.TrimSpace(candidate.resolvedIDs["subject"]); id != "" {
		candidate.payload["subject_entity_id"] = id
	}
	if id := strings.TrimSpace(candidate.resolvedIDs["actor"]); id != "" {
		candidate.payload["speaker_entity_id"] = id
	}
}

func (candidate *preciseMemoryCandidate) assignIdentityPointers(unit *store.PreciseMemoryUnit) {
	for role, id := range candidate.resolvedIDs {
		switch role {
		case "actor":
			unit.ActorEntityID = id
		case "subject":
			unit.SubjectEntityID = id
		case "affected":
			unit.AffectedEntityID = id
		case "location":
			unit.LocationEntityID = id
		case "object":
			unit.ObjectEntityID = id
		case "knower":
			unit.KnowledgeHolderEntityID = id
		}
	}
}

func (candidate *preciseMemoryCandidate) markNeedsReview() {
	candidate.admissionState = "review_required"
	candidate.reviewState = "needs_review"
	if candidate.authorityClass == "objective_world_state" && candidate.truthScope == "objective" {
		candidate.truthScope = "unresolved_subject"
		candidate.epistemicMode = "unresolved"
		candidate.authorityClass = "support_hypothesis"
	}
}

func preciseMemoryStructuredClassification(item map[string]any) string {
	for _, key := range []string{"is_ooc", "ooc", "out_of_character"} {
		if boolFromAny(item[key]) {
			return "ooc_meta"
		}
	}
	for _, key := range []string{"is_proposal", "proposed", "hypothetical"} {
		if boolFromAny(item[key]) {
			return "proposal"
		}
	}
	for _, key := range []string{"is_lie", "is_deception", "known_false"} {
		if boolFromAny(item[key]) {
			return "deception"
		}
	}
	for _, key := range []string{"is_uncertain", "is_speculation", "speculative"} {
		if boolFromAny(item[key]) {
			return "non_objective"
		}
	}
	for _, key := range []string{
		"claim_scope", "epistemic_mode", "modality", "truth_scope",
		"truth_status", "assertion_status", "event_status", "event_type",
		"statement_type", "transition",
	} {
		value := strings.ToLower(strings.TrimSpace(extractionStringFromAny(item[key])))
		switch value {
		case "ooc", "ooc_meta", "meta", "out_of_character":
			return "ooc_meta"
		case "proposal", "proposed", "plan", "planned", "hypothetical":
			return "proposal"
		case "lie", "false", "deception", "deceptive", "known_false":
			return "deception"
		case "belief", "subjective", "perception", "rumor", "hearsay", "inferred",
			"misunderstood", "uncertain", "uncertainty", "speculation", "speculative",
			"suspected", "secret", "private":
			return "non_objective"
		}
	}
	return "objective"
}

func preciseMemoryExactSpan(payload map[string]any, excerpt, content string) (int, int, bool) {
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" || content == "" {
		return 0, 0, false
	}
	start := intFromAny(payload["source_span_start"], -1)
	end := intFromAny(payload["source_span_end"], -1)
	if start >= 0 && end > start && end <= len(content) && content[start:end] == excerpt {
		return start, end, true
	}
	first := strings.Index(content, excerpt)
	if first < 0 {
		return 0, 0, false
	}
	if strings.Index(content[first+len(excerpt):], excerpt) >= 0 {
		return 0, 0, false
	}
	return first, first + len(excerpt), true
}

func preciseMemoryExactEvidenceIDs(evidence []store.DirectEvidence, sid string, turnIndex int, excerpt string) []int64 {
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" {
		return []int64{}
	}
	ids := []int64{}
	for _, item := range evidence {
		if item.ID <= 0 || item.ChatSessionID != sid ||
			strings.TrimSpace(item.EvidenceText) != excerpt ||
			item.RepairNeeded || item.Tombstoned || item.SupersededByID > 0 ||
			item.ArchiveState != "verified_direct" ||
			item.CaptureStage != "critic_extract" ||
			item.CaptureVerification != "verified" ||
			item.CommittedGate != "auto_grounded_excerpt" {
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = start
		}
		if start != turnIndex || end != turnIndex {
			continue
		}
		ids = append(ids, item.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func preciseMemorySemanticPayload(item map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		if value, ok := item[field]; ok {
			out[field] = normalizePreciseMemoryValue(value)
		}
	}
	return out
}

func preciseMemoryParticipantSurfaces(value any) []string {
	out := []string{}
	for _, raw := range sliceFromAny(value) {
		surface := ""
		switch typed := raw.(type) {
		case string:
			surface = typed
		default:
			item := mapFromAny(typed)
			surface = extractionFirstNonEmpty(
				stringFromMap(item, "name"),
				stringFromMap(item, "label"),
				stringFromMap(item, "entity"),
			)
		}
		surface = strings.TrimSpace(surface)
		if surface != "" {
			out = append(out, surface)
		}
	}
	sort.Strings(out)
	return out
}

func normalizePreciseMemoryValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, nested := range typed {
			out[strings.TrimSpace(key)] = normalizePreciseMemoryValue(nested)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, nested := range typed {
			out = append(out, normalizePreciseMemoryValue(nested))
		}
		sort.SliceStable(out, func(i, j int) bool {
			left, _ := json.Marshal(out[i])
			right, _ := json.Marshal(out[j])
			return string(left) < string(right)
		})
		return out
	case string:
		return strings.Join(strings.Fields(strings.TrimSpace(typed)), " ")
	default:
		return value
	}
}

func preciseMemorySubtype(item map[string]any, field, fallback string) string {
	value := strings.ToLower(strings.TrimSpace(stringFromMap(item, field)))
	if value == "" {
		return fallback
	}
	return normalizeNarrativeStateSlot(value)
}

func preciseMemoryVisibility(item map[string]any, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(stringFromMap(item, "visibility"))) {
	case "public", "player_known", "owner_private", "restricted", "reveal_required":
		return strings.ToLower(strings.TrimSpace(stringFromMap(item, "visibility")))
	default:
		return fallback
	}
}

func preciseMemorySubjectNeedsIdentity(subjectType string) bool {
	switch strings.ToLower(strings.TrimSpace(subjectType)) {
	case "world", "session":
		return false
	default:
		return true
	}
}

func preciseMemoryStableID(sid, idempotencyKey string) string {
	sum := sha256.Sum256([]byte("precise-memory\x00" + sid + "\x00" + idempotencyKey))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(bytes)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}
