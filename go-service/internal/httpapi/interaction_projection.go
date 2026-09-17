package httpapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const activeInteractionProjectionContract = "active_interaction_projection.v1"

type prepareTurnInteractionCandidate struct {
	unit          store.PreciseMemoryUnit
	payload       map[string]any
	kind          string
	groupKey      string
	actorID       string
	targetID      string
	actor         string
	target        string
	visibility    string
	sourceTurn    int
	decisionRank  int
	line          string
	guardedWriter bool
}

// buildPrepareTurnActiveInteractionProjection reads atomic precise-memory
// observations as source-backed constraints. It does not materialize durable
// relationship current/history state; that owner remains outside 3.8-D.
func buildPrepareTurnActiveInteractionProjection(
	units []store.PreciseMemoryUnit,
	perspectiveContext map[string]any,
	rawUserInput string,
	currentSceneEntities []string,
	currentTurn int,
	deliveryEnabled bool,
) (map[string]any, string, string) {
	packet := map[string]any{
		"contract_version":         activeInteractionProjectionContract,
		"status":                   "empty",
		"authority":                "source_observation_only",
		"relationship_state_owner": "not_materialized_by_3.8_d",
		"input_count":              len(units),
		"candidate_count":          0,
		"selected_count":           0,
		"public_selected_count":    0,
		"guarded_selected_count":   0,
		"used_chars":               0,
		"dropped_counts":           map[string]any{},
		"items":                    []map[string]any{},
	}
	dropped := packet["dropped_counts"].(map[string]any)
	drop := func(reason string) {
		dropped[reason] = intFromAny(dropped[reason], 0) + 1
	}
	if !deliveryEnabled {
		for range units {
			drop("delivery_disabled")
		}
		return packet, "", ""
	}

	relationshipGroups := map[string][]prepareTurnInteractionCandidate{}
	boundaryGroups := map[string][]prepareTurnInteractionCandidate{}
	for _, unit := range units {
		candidate, reason := normalizePrepareTurnInteractionCandidate(unit)
		if reason != "" {
			drop(reason)
			continue
		}
		switch candidate.kind {
		case "relationship":
			relationshipGroups[candidate.groupKey] = append(relationshipGroups[candidate.groupKey], candidate)
		case "boundary":
			boundaryGroups[candidate.groupKey] = append(boundaryGroups[candidate.groupKey], candidate)
		}
	}

	projected := []prepareTurnInteractionCandidate{}
	for _, candidates := range relationshipGroups {
		latestTurn := latestPrepareTurnInteractionSourceTurn(candidates)
		seen := map[string]bool{}
		for _, candidate := range candidates {
			if candidate.sourceTurn != latestTurn {
				drop("superseded_by_later_source_turn")
				continue
			}
			key := normalizeArtifactDedupeText(extractionStringFromAny(candidate.payload["observation"]))
			if seen[key] {
				drop("duplicate_latest_observation")
				continue
			}
			seen[key] = true
			projected = append(projected, candidate)
		}
	}
	for _, candidates := range boundaryGroups {
		latestTurn := latestPrepareTurnInteractionSourceTurn(candidates)
		latest := []prepareTurnInteractionCandidate{}
		for _, candidate := range candidates {
			if candidate.sourceTurn != latestTurn {
				drop("superseded_by_later_source_turn")
				continue
			}
			latest = append(latest, candidate)
		}
		if len(latest) == 0 {
			continue
		}
		sort.SliceStable(latest, func(i, j int) bool {
			if latest[i].decisionRank != latest[j].decisionRank {
				return latest[i].decisionRank > latest[j].decisionRank
			}
			return latest[i].unit.UnitID < latest[j].unit.UnitID
		})
		projected = append(projected, latest[0])
		for range latest[1:] {
			drop("lower_priority_same_turn_boundary")
		}
	}

	povID := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_entity_id"]))
	povResolved := extractionStringFromAny(perspectiveContext["identity_state"]) == "resolved" && povID != ""
	selected := []prepareTurnInteractionCandidate{}
	for _, candidate := range projected {
		if candidate.kind == "boundary" &&
			strings.EqualFold(extractionStringFromAny(candidate.payload["decision"]), "allow") &&
			strings.EqualFold(extractionStringFromAny(candidate.payload["effective_scope"]), "event") &&
			(currentTurn <= 0 || candidate.sourceTurn < currentTurn) {
			drop("event_scoped_allow_not_current")
			continue
		}
		switch candidate.visibility {
		case "public":
			if !prepareTurnInteractionEndpointRelevant(rawUserInput, currentSceneEntities, candidate.actor, candidate.target) {
				drop("public_endpoints_not_current")
				continue
			}
		case "owner_private", "restricted":
			if !povResolved || candidate.actorID != povID {
				drop("resolved_actor_pov_required")
				continue
			}
			candidate.guardedWriter = true
		case "user_private":
			drop("user_private_not_in_world_delivery")
			continue
		default:
			drop("unsupported_visibility")
			continue
		}
		selected = append(selected, candidate)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].sourceTurn != selected[j].sourceTurn {
			return selected[i].sourceTurn > selected[j].sourceTurn
		}
		if selected[i].kind != selected[j].kind {
			return selected[i].kind == "boundary"
		}
		return selected[i].unit.UnitID < selected[j].unit.UnitID
	})

	publicLines := []string{}
	guardedLines := []string{}
	items := []map[string]any{}
	for _, candidate := range selected {
		line := renderPrepareTurnInteractionCandidate(candidate)
		candidate.line = line
		if candidate.guardedWriter {
			guardedLines = append(guardedLines, line)
		} else {
			publicLines = append(publicLines, line)
		}
		items = append(items, map[string]any{
			"unit_id":          candidate.unit.UnitID,
			"source_turn":      candidate.sourceTurn,
			"source_ref":       "precise_memory:" + candidate.unit.UnitID,
			"kind":             candidate.kind,
			"actor_entity_id":  candidate.actorID,
			"target_entity_id": candidate.targetID,
			"visibility":       candidate.visibility,
			"writer_guarded":   candidate.guardedWriter,
			"disposition":      "candidate",
			"reason_code":      "latest_source_backed_observation",
			"line":             line,
		})
	}
	packet["items"] = items
	packet["candidate_count"] = len(items)
	if len(items) > 0 {
		packet["status"] = "candidate"
	}
	return packet,
		makePrepareTurnSection("[Directional Interaction Evidence - source observations only; no reciprocity, cross-domain inference, or current-state authority]", publicLines),
		makePrepareTurnSection("[Guarded Interaction Context - current POV writer context; do not reveal, reciprocate, or treat as general consent]", guardedLines)
}

func normalizePrepareTurnInteractionCandidate(unit store.PreciseMemoryUnit) (prepareTurnInteractionCandidate, string) {
	candidate := prepareTurnInteractionCandidate{unit: unit}
	if unit.LifecycleState != "active" {
		return candidate, "inactive"
	}
	if strings.TrimSpace(unit.SourceRevision) == "" {
		return candidate, "source_revision_missing"
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(unit.PayloadJSON), &payload); err != nil {
		return candidate, "invalid_payload"
	}
	actorID := strings.TrimSpace(unit.ActorEntityID)
	targetID := strings.TrimSpace(unit.AffectedEntityID)
	if actorID == "" || targetID == "" || actorID == targetID {
		return candidate, "stable_directional_identity_required"
	}
	visibility := strings.ToLower(strings.TrimSpace(unit.Visibility))
	payloadVisibility := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["visibility"])))
	if payloadVisibility == "" || visibility != payloadVisibility {
		return candidate, "visibility_mismatch"
	}
	actor := ""
	target := ""
	sourceTurn := unit.SourceTurnEnd
	if sourceTurn <= 0 {
		sourceTurn = unit.SourceTurnStart
	}
	if sourceTurn <= 0 {
		return candidate, "source_turn_missing"
	}
	candidate.payload = payload
	candidate.actorID = actorID
	candidate.targetID = targetID
	candidate.visibility = visibility
	candidate.sourceTurn = sourceTurn

	switch {
	case unit.Kind == "observation" && extractionStringFromAny(payload["contract_version"]) == relationshipObservationContract:
		actor = strings.TrimSpace(extractionStringFromAny(payload["source_entity"]))
		target = strings.TrimSpace(extractionStringFromAny(payload["target_entity"]))
		domain := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["domain"])))
		observation := strings.TrimSpace(extractionStringFromAny(payload["observation"]))
		if actor == "" || target == "" || observation == "" || domain == "" {
			return candidate, "invalid_relationship_observation"
		}
		candidate.kind = "relationship"
		candidate.groupKey = strings.Join([]string{"relationship", actorID, targetID, domain}, "\x1f")
	case unit.Kind == "boundary" && extractionStringFromAny(payload["contract_version"]) == interactionBoundaryContract:
		actor = strings.TrimSpace(extractionStringFromAny(payload["actor"]))
		target = strings.TrimSpace(extractionStringFromAny(payload["counterpart"]))
		actionScope := strings.TrimSpace(extractionStringFromAny(payload["action_scope"]))
		decision := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["decision"])))
		effectiveScope := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["effective_scope"])))
		if actor == "" || target == "" || actionScope == "" || effectiveScope == "" || !validInteractionBoundaryDecision(decision) {
			return candidate, "invalid_interaction_boundary"
		}
		candidate.kind = "boundary"
		candidate.decisionRank = interactionBoundaryDecisionPriority(decision)
		candidate.groupKey = strings.Join([]string{
			"boundary", actorID, targetID, normalizeArtifactComparableText(actionScope),
			effectiveScope,
		}, "\x1f")
	default:
		return candidate, "unsupported_contract"
	}
	candidate.actor = actor
	candidate.target = target
	return candidate, ""
}

func latestPrepareTurnInteractionSourceTurn(candidates []prepareTurnInteractionCandidate) int {
	latest := 0
	for _, candidate := range candidates {
		if candidate.sourceTurn > latest {
			latest = candidate.sourceTurn
		}
	}
	return latest
}

func prepareTurnInteractionEndpointRelevant(rawUserInput string, currentSceneEntities []string, actor, target string) bool {
	if prepareTurnRecallContainsAnchor(rawUserInput, actor) || prepareTurnRecallContainsAnchor(rawUserInput, target) {
		return true
	}
	actorKey := comparableEntityKey(actor)
	targetKey := comparableEntityKey(target)
	for _, entity := range currentSceneEntities {
		key := comparableEntityKey(entity)
		if key != "" && (key == actorKey || key == targetKey) {
			return true
		}
	}
	return false
}

func renderPrepareTurnInteractionCandidate(candidate prepareTurnInteractionCandidate) string {
	source := fmt.Sprintf("source_turn=%d", candidate.sourceTurn)
	prefix := "[Directional Interaction Evidence]"
	if candidate.guardedWriter {
		prefix = "[Guarded Interaction Context]"
	}
	if candidate.kind == "relationship" {
		return fmt.Sprintf("- %s relationship | %s -> %s | domain=%s | %s | %s",
			prefix, candidate.actor, candidate.target,
			extractionStringFromAny(candidate.payload["domain"]),
			extractionStringFromAny(candidate.payload["observation"]), source)
	}
	return fmt.Sprintf("- %s boundary | %s -> %s | action=%s | decision=%s | effective_scope=%s | %s",
		prefix, candidate.actor, candidate.target,
		extractionStringFromAny(candidate.payload["action_scope"]),
		extractionStringFromAny(candidate.payload["decision"]),
		extractionStringFromAny(candidate.payload["effective_scope"]), source)
}

func finalizePrepareTurnActiveInteractionProjection(packet map[string]any, publicCandidateText, guardedCandidateText, finalMemoryText string) (map[string]any, string, string) {
	if len(packet) == 0 {
		return packet, "", ""
	}
	items, _ := packet["items"].([]map[string]any)
	publicLines := []string{}
	guardedLines := []string{}
	selected := 0
	for _, item := range items {
		line := strings.TrimSpace(extractionStringFromAny(item["line"]))
		if line != "" && strings.Contains(finalMemoryText, line) {
			item["disposition"] = "delivered"
			item["reason_code"] = "selected_within_memory_delivery_plan"
			selected++
			if boolFromAny(item["writer_guarded"]) {
				guardedLines = append(guardedLines, line)
			} else {
				publicLines = append(publicLines, line)
			}
			continue
		}
		item["disposition"] = "deferred"
		item["reason_code"] = "memory_delivery_char_budget"
	}
	packet["selected_count"] = selected
	packet["public_selected_count"] = len(publicLines)
	packet["guarded_selected_count"] = len(guardedLines)
	packet["used_chars"] = len([]rune(strings.Join(append(append([]string{}, publicLines...), guardedLines...), "\n")))
	packet["items"] = items
	if selected == 0 {
		if intFromAny(packet["candidate_count"], 0) > 0 {
			packet["status"] = "deferred_by_memory_delivery_plan"
		}
		return packet, "", ""
	}
	packet["status"] = "ready"
	publicHeader := firstPrepareTurnSectionHeader(publicCandidateText)
	guardedHeader := firstPrepareTurnSectionHeader(guardedCandidateText)
	return packet,
		makePrepareTurnSection(publicHeader, publicLines),
		makePrepareTurnSection(guardedHeader, guardedLines)
}

func firstPrepareTurnSectionHeader(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			return line
		}
	}
	return "[Source-backed Interaction Context]"
}
