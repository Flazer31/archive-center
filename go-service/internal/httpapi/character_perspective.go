package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func resolvePrepareTurnPerspectiveIdentity(
	ctx context.Context,
	candidateStore store.Store,
	sid string,
	perspectiveContext map[string]any,
) map[string]any {
	out := normalizePrepareTurnPerspectiveContext(perspectiveContext)
	if len(out) == 0 {
		return nil
	}
	delete(out, "current_pov_entity_id")
	out["identity_state"] = "unobserved"
	surfaceKey := comparableEntityKey(extractionStringFromAny(out["current_pov"]))
	resolver, ok := candidateStore.(store.UniqueActiveEntitySurfaceResolver)
	if !ok {
		out["identity_state"] = "resolver_unavailable"
		return out
	}
	if surfaceKey == "" {
		return out
	}
	resolvedID, err := resolver.ResolveUniqueActiveEntityIDBySurface(ctx, sid, surfaceKey)
	switch {
	case err == nil && strings.TrimSpace(resolvedID) != "":
		requestedID := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_entity_id"]))
		if requestedID != "" && requestedID != strings.TrimSpace(resolvedID) {
			out["identity_state"] = "needs_review"
			return out
		}
		out["current_pov_entity_id"] = strings.TrimSpace(resolvedID)
		out["identity_state"] = "resolved"
	case errors.Is(err, store.ErrReviewedEntityIdentityAmbiguous):
		out["identity_state"] = "needs_review"
	default:
		out["identity_state"] = "unobserved"
	}
	return out
}

func buildCharacterPerspectivePacket(
	units []store.PreciseMemoryUnit,
	perspectiveContext map[string]any,
	maxChars int,
) (map[string]any, string) {
	type perspectiveCandidate struct {
		unit       store.PreciseMemoryUnit
		payload    map[string]any
		state      string
		subject    string
		slot       string
		claim      string
		sourceTurn int
	}

	holderID := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_entity_id"]))
	packet := map[string]any{
		"contract_version": "character_perspective_packet.v1",
		"status":           "empty",
		"identity_state":   extractionFirstNonEmpty(extractionStringFromAny(perspectiveContext["identity_state"]), "unobserved"),
		"input_count":      len(units),
		"selected_count":   0,
		"candidate_count":  0,
		"candidate_states": map[string]any{},
		"selected_states":  map[string]any{},
		"dropped_counts":   map[string]any{},
		"used_chars":       0,
		"truncated":        false,
	}
	dropped := packet["dropped_counts"].(map[string]any)
	drop := func(reason string) {
		dropped[reason] = intFromAny(dropped[reason], 0) + 1
	}
	if holderID == "" || extractionStringFromAny(perspectiveContext["identity_state"]) != "resolved" {
		drop("stable_pov_identity_required")
		return packet, ""
	}
	if maxChars <= 0 {
		drop("delivery_budget_unavailable")
		return packet, ""
	}
	candidatesByKey := map[string][]perspectiveCandidate{}
	for _, unit := range units {
		if strings.TrimSpace(unit.KnowledgeHolderEntityID) != holderID {
			drop("wrong_knowledge_holder")
			continue
		}
		if unit.Kind != "observation" || unit.LifecycleState != "active" {
			drop("inactive_or_unsupported")
			continue
		}
		state, validState := normalizePerspectiveMemoryState(unit.EpistemicMode)
		if !validState {
			drop("unsupported_epistemic_state")
			continue
		}
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(unit.PayloadJSON), &payload); err != nil {
			drop("invalid_payload")
			continue
		}
		if extractionStringFromAny(payload["contract_version"]) != "perspective_memory.v1" ||
			strings.TrimSpace(extractionStringFromAny(payload["knowledge_holder_entity_id"])) != holderID ||
			extractionStringFromAny(payload["epistemic_state"]) != state {
			drop("payload_identity_or_contract_mismatch")
			continue
		}
		subject := strings.TrimSpace(extractionFirstNonEmpty(
			extractionStringFromAny(payload["subject"]),
			extractionStringFromAny(payload["subject_entity_id"]),
		))
		slot := strings.TrimSpace(extractionStringFromAny(payload["state_slot"]))
		claim := strings.TrimSpace(extractionStringFromAny(payload["claim"]))
		if subject == "" || slot == "" || claim == "" {
			drop("incomplete_claim")
			continue
		}
		subjectKey := strings.TrimSpace(extractionStringFromAny(payload["subject_entity_id"]))
		if subjectKey == "" {
			subjectKey = comparableEntityKey(subject)
		}
		sourceTurn := unit.SourceTurnEnd
		if sourceTurn <= 0 {
			sourceTurn = unit.SourceTurnStart
		}
		currentKey := strings.Join([]string{holderID, subjectKey, comparableEntityKey(slot)}, "\x1f")
		// A secret category and an episodic recollection are collections, not
		// single-valued state slots. Keep independent claims in those collections.
		if stringFromMap(payload, "secret_kind") != "" || unit.Subtype == "subjective_memory" {
			claimKey := extractionFirstNonEmpty(stringFromMap(payload, "secret_id"), stringFromMap(payload, "artifact_id"), normalizeArtifactDedupeText(claim))
			currentKey += "\x1f" + claimKey
		}
		candidatesByKey[currentKey] = append(candidatesByKey[currentKey], perspectiveCandidate{
			unit:       unit,
			payload:    payload,
			state:      state,
			subject:    subject,
			slot:       slot,
			claim:      claim,
			sourceTurn: sourceTurn,
		})
	}

	current := make([]perspectiveCandidate, 0, len(candidatesByKey))
	for _, candidates := range candidatesByKey {
		latestTurn := -1
		for _, candidate := range candidates {
			if candidate.sourceTurn > latestTurn {
				latestTurn = candidate.sourceTurn
			}
		}
		latest := make([]perspectiveCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate.sourceTurn != latestTurn {
				drop("superseded_epistemic_state")
				continue
			}
			latest = append(latest, candidate)
		}
		distinct := map[string]bool{}
		for _, candidate := range latest {
			distinct[perspectiveMemoryStateIdentity(candidate.state)+"\x1f"+normalizeArtifactDedupeText(candidate.claim)] = true
		}
		if len(distinct) != 1 {
			for range latest {
				drop("conflicting_latest_epistemic_state")
			}
			continue
		}
		current = append(current, latest[0])
		for range latest[1:] {
			drop("duplicate_latest_epistemic_state")
		}
	}
	sort.SliceStable(current, func(i, j int) bool {
		if current[i].sourceTurn != current[j].sourceTurn {
			return current[i].sourceTurn > current[j].sourceTurn
		}
		return current[i].unit.UnitID < current[j].unit.UnitID
	})

	lines := []string{}
	recollections := []prepareTurnPriorityFactSeed{}
	protectedUnits := map[string]store.PreciseMemoryUnit{}
	candidateStates := packet["candidate_states"].(map[string]any)
	for _, candidate := range current {
		if candidate.state == "unknown" || candidate.state == "hidden" {
			drop("not_known_by_current_holder")
			continue
		}
		state := candidate.state
		subject := candidate.subject
		slot := candidate.slot
		claim := candidate.claim
		payload := candidate.payload
		line := fmt.Sprintf("- %s | %s / %s: %s", state, subject, slot, claim)
		if acquisition := strings.TrimSpace(extractionStringFromAny(payload["acquisition_mode"])); acquisition != "" {
			line += " | acquisition=" + acquisition
		}
		line += fmt.Sprintf(" | holder=%s | source_turn=%d | source=%s", holderID, candidate.sourceTurn, candidate.unit.UnitID)
		// Ordinary experiences use the same scoped candidate selection as other
		// memories. Their number must not consume the protected-guidance budget.
		// The existing writer assigns this subtype only to non-secret recollections.
		if candidate.unit.Subtype == "subjective_memory" {
			text := strings.TrimPrefix(line, "- ")
			recollections = append(recollections, prepareTurnPriorityFactSeed{
				Lane: "subjective_relationship", SourceTable: "precise_memory_units", Tier: "required",
				Fact:        prepareTurnPriorityMemoryFact{Text: text, SourceSession: candidate.unit.ChatSessionID, ExplicitRefs: prepareTurnKnowledgeRefs(payload), KnowledgeBoundaries: prepareTurnOwnKnowledgeBoundary(payload)},
				SourceRowID: candidate.unit.UnitID, SourceOccurrence: "precise-memory-unit:" + candidate.unit.UnitID,
				SourceTurn: candidate.sourceTurn, Visibility: "owner_private",
				PerspectiveOwner: holderID, AllowedViewers: []string{holderID},
				ProjectionSource: "character_perspective", ParentLineKey: text, SourceFactCount: 1,
			})
			lines = append(lines, line)
			candidateStates[state] = intFromAny(candidateStates[state], 0) + 1
			continue
		}
		// Keep whole protected candidates until the independent protected budget
		// owner selects them. A main-memory cap cannot establish their delivery.
		protectedUnits[line] = candidate.unit
		lines = append(lines, line)
		candidateStates[state] = intFromAny(candidateStates[state], 0) + 1
	}
	if len(lines) == 0 {
		return packet, ""
	}
	text := "[Character Perspective]\n" + strings.Join(lines, "\n")
	packet["status"] = "candidate"
	packet["candidate_count"] = len(lines)
	packet["candidate_chars"] = utf8.RuneCountInString(text)
	packet["_character_perspective_fact_seeds"] = recollections
	packet["_character_perspective_protected_units"] = protectedUnits
	return packet, text
}

// The protected source card already carries the named knowledge boundary. Its
// known projection is the same fact, while suspicion, misinformation and a
// different source/claim remain independent perspective material.
func characterPerspectiveKnownCoveredByProtectedSource(unit store.PreciseMemoryUnit, payload map[string]any, memories []store.Memory, perspective map[string]any) bool {
	if unit.EpistemicMode != "known" || stringFromMap(payload, "secret_kind") == "" {
		return false
	}
	claim := strings.Join(strings.Fields(stringFromMap(payload, "claim")), " ")
	holder := extractionFirstNonEmpty(stringFromMap(payload, "knowledge_holder"), stringFromMap(perspective, "current_pov"))
	turn := unit.SourceTurnEnd
	if turn <= 0 {
		turn = unit.SourceTurnStart
	}
	for _, memory := range memories {
		if turn <= 0 || memory.TurnIndex != turn || memory.ChatSessionID != unit.ChatSessionID {
			continue
		}
		parsed := parseJSONMap(memory.SummaryJSON)
		if revision := stringFromMap(parsed, "source_revision"); revision != "" && unit.SourceRevision != "" && revision != unit.SourceRevision {
			continue
		}
		for _, raw := range sliceFromAny(parsed["protected_secrets"]) {
			secret := mapFromAny(raw)
			body := extractionFirstNonEmpty(stringFromMap(secret, "summary"), stringFromMap(secret, "secret_summary"), stringFromMap(secret, "text"))
			if claim != strings.Join(strings.Fields(body), " ") || !protectedSecretRequiresGuard(secret, "disclosure_policy") {
				continue
			}
			if normalizeProtectedSecretToken(stringFromMap(secret, "secret_kind")) != normalizeProtectedSecretToken(stringFromMap(payload, "secret_kind")) {
				continue
			}
			if comparableEntityKey(stringFromMap(secret, "owner")) != comparableEntityKey(stringFromMap(payload, "owner")) {
				continue
			}
			subject := extractionFirstNonEmpty(firstStringFromAny(secret["subject"]), stringFromMap(secret, "owner"))
			if comparableEntityKey(subject) != comparableEntityKey(stringFromMap(payload, "subject")) {
				continue
			}
			secretID := extractionFirstNonEmpty(stringFromMap(secret, "secret_id"), stringFromMap(secret, "artifact_id"))
			projectionID := extractionFirstNonEmpty(stringFromMap(payload, "secret_id"), stringFromMap(payload, "artifact_id"))
			if projectionID != "" && secretID != projectionID {
				continue
			}
			if prepareTurnRelationshipNameInList(holder, stringsFromAny(mapFromAny(secret["knowledge_scope"])["known_by"])) {
				return true
			}
		}
	}
	return false
}

// Attach identity to the already admitted/rendered representations, without a
// second relevance pass or a delivery decision. CoverageKey retains the exact
// protected fact, artifact and boundary from the existing grouping owner.
func attachCharacterPerspectiveProtectedFactKeys(out *prepareTurnInjectionAssembly, selection prepareTurnMemoryLaneSelection, memories []store.Memory, trace map[string]any, units map[string]store.PreciseMemoryUnit, perspective map[string]any, prepared *prepareTurnRequestPreparation) {
	if len(units) == 0 {
		return
	}
	out.protectedFactKeys = map[string][]string{}
	lines := stringsFromAny(trace["protected_lines"])
	lineIndex := 0
	renderedKeys := map[string]bool{}
	for _, raw := range prepareTurnMemoryLineageSlice(trace["delivery_lineage_items"]) {
		item := mapFromAny(raw)
		key := stringFromMap(item, "protected_coverage_key")
		if key == "" || !boolFromAny(item["delivered"]) {
			continue
		}
		line := lines[lineIndex]
		lineIndex++
		out.protectedFactKeys[line] = append(out.protectedFactKeys[line], key)
		renderedKeys[key] = true
	}
	groups, _ := buildPrepareTurnProtectedDeliveryGroupsPrepared(selection, prepared, memories...)
	sources := map[string]store.Memory{}
	for _, memory := range memories {
		sources[fmt.Sprint(prepareTurnMemorySourceRowID(memory))] = memory
	}
	for line, unit := range units {
		keys := map[string]bool{}
		payload := prepared.sourceMap(unit.PayloadJSON)
		for _, sourceGroups := range groups {
			for _, group := range sourceGroups {
				if !renderedKeys[group.CoverageKey] || group.Disclosure != nil || group.ProtectionField != "protected_secrets" {
					continue
				}
				for _, id := range group.SourceRowIDs {
					if memory, ok := sources[fmt.Sprint(id)]; ok {
						memory.SummaryJSON = group.Memory.SummaryJSON
						if characterPerspectiveKnownCoveredByProtectedSource(unit, payload, []store.Memory{memory}, perspective) {
							keys[group.CoverageKey] = true
						}
					}
				}
			}
		}
		// An ambiguous projection must not merge distinct artifacts or scopes.
		if len(keys) == 1 {
			for key := range keys {
				out.protectedFactKeys[line] = []string{key}
			}
		}
	}
}

func finalizeCharacterPerspectivePacket(
	packet map[string]any,
	candidateText string,
	finalMemoryText string,
	protectedDuplicates ...map[string]bool,
) (map[string]any, string) {
	delete(packet, "_character_perspective_fact_seeds")
	delete(packet, "_character_perspective_protected_units")
	if len(packet) == 0 || strings.TrimSpace(candidateText) == "" {
		return packet, ""
	}
	delivered := []string{}
	selectedStates := map[string]any{}
	if len(protectedDuplicates) > 0 {
		dropped := mapFromAny(packet["dropped_counts"])
		dropped["same_source_protected_secret"] = len(protectedDuplicates[0])
	}
	for _, line := range strings.Split(candidateText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "[Character Perspective]" {
			continue
		}
		if strings.Contains(finalMemoryText, strings.TrimPrefix(line, "- ")) {
			delivered = append(delivered, line)
			state := strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, "- "), " |", 2)[0])
			if normalized, valid := normalizePerspectiveMemoryState(state); valid {
				selectedStates[normalized] = intFromAny(selectedStates[normalized], 0) + 1
			}
		}
	}
	packet["selected_count"] = len(delivered)
	packet["selected_states"] = selectedStates
	if len(delivered) == 0 {
		packet["status"] = "deferred_by_memory_delivery_plan"
		packet["used_chars"] = 0
		return packet, ""
	}
	text := "[Character Perspective]\n" + strings.Join(delivered, "\n")
	packet["status"] = "ready"
	packet["used_chars"] = utf8.RuneCountInString(text)
	return packet, text
}

func prepareTurnPerspectiveHardFilterActive(perspectiveContext map[string]any) bool {
	switch extractionStringFromAny(perspectiveContext["identity_state"]) {
	case "resolved", "unobserved", "needs_review":
		return true
	default:
		return false
	}
}
