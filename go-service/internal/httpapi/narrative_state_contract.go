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
	narrativeStateContractVersion = "narrative_state.v1"
	narrativeStateStatusKey       = "narrative_state"
)

type narrativeStateClaim struct {
	Subject          string
	SubjectType      string
	StateSlot        string
	Value            string
	ClaimScope       string
	PerspectiveOwner string
	Transition       string
	Confidence       float64
	EvidenceExcerpt  string
	SourceKind       string
	SourceIndex      int
}

func normalizeNarrativeStateClaims(extraction map[string]any) []narrativeStateClaim {
	out := []narrativeStateClaim{}
	appendClaim := func(raw any, sourceKind string, index int) {
		item := mapFromAny(raw)
		if len(item) == 0 {
			return
		}
		claim := narrativeStateClaim{
			Subject:          strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject"), stringFromMap(item, "entity"), stringFromMap(item, "owner"))),
			SubjectType:      normalizeNarrativeSubjectType(stringFromMap(item, "subject_type")),
			StateSlot:        normalizeNarrativeStateSlot(extractionFirstNonEmpty(stringFromMap(item, "state_slot"), stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"))),
			Value:            strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "value"), stringFromMap(item, "state_value"), stringFromMap(item, "belief"))),
			ClaimScope:       normalizeNarrativeClaimScope(extractionFirstNonEmpty(stringFromMap(item, "claim_scope"), sourceKind)),
			PerspectiveOwner: strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "perspective_owner"), stringFromMap(item, "believer"), stringFromMap(item, "knower"))),
			Transition:       normalizeNarrativeTransition(stringFromMap(item, "transition")),
			Confidence:       clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			EvidenceExcerpt:  strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence"))),
			SourceKind:       sourceKind,
			SourceIndex:      index,
		}
		if claim.SubjectType == "" {
			claim.SubjectType = "entity"
		}
		if claim.Transition == "" {
			claim.Transition = "set"
		}
		if claim.ClaimScope == "belief" && claim.PerspectiveOwner == "" {
			claim.PerspectiveOwner = claim.Subject
		}
		if claim.Subject == "" || claim.StateSlot == "" || claim.Value == "" || claim.EvidenceExcerpt == "" {
			return
		}
		out = append(out, claim)
	}
	for i, raw := range sliceFromAny(extraction["state_claims"]) {
		appendClaim(raw, "objective", i)
	}
	for i, raw := range sliceFromAny(extraction["belief_updates"]) {
		appendClaim(raw, "belief", i)
	}
	return out
}

func appendNarrativeStateEvidenceExcerpts(extraction map[string]any) map[string]any {
	if extraction == nil {
		return extraction
	}
	excerpts := stringsFromAny(extraction["evidence_excerpts"])
	seen := map[string]bool{}
	for _, excerpt := range excerpts {
		seen[normalizeArtifactDedupeText(excerpt)] = true
	}
	for _, key := range []string{"narrative_events", "state_claims", "belief_updates"} {
		for _, raw := range sliceFromAny(extraction[key]) {
			item := mapFromAny(raw)
			excerpt := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")))
			normalized := normalizeArtifactDedupeText(excerpt)
			if excerpt == "" || normalized == "" || seen[normalized] {
				continue
			}
			seen[normalized] = true
			excerpts = append(excerpts, excerpt)
		}
	}
	extraction["evidence_excerpts"] = excerpts
	return extraction
}

func normalizeNarrativeSubjectType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "character", "person", "npc":
		return "character"
	case "world", "setting":
		return "world"
	case "location", "place":
		return "location"
	case "faction", "organization", "group":
		return "faction"
	case "session", "scene":
		return "session"
	case "item", "object", "entity":
		return "entity"
	default:
		return ""
	}
}

func normalizeNarrativeClaimScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "objective", "fact", "canonical":
		return "objective"
	case "belief", "subjective", "perception":
		return "belief"
	case "rumor", "suspected":
		return "rumor"
	case "secret", "private":
		return "secret"
	default:
		return "objective"
	}
}

func normalizeNarrativeTransition(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "set", "reaffirm", "change", "reversal", "recovery", "correction", "reveal", "resolve", "uncertain", "clear":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeNarrativeStateSlot(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func narrativeStateOwnerID(claim narrativeStateClaim) string {
	key := strings.Join([]string{
		strings.ToLower(strings.TrimSpace(claim.SubjectType)),
		strings.ToLower(strings.TrimSpace(claim.Subject)),
		strings.ToLower(strings.TrimSpace(claim.StateSlot)),
		strings.ToLower(strings.TrimSpace(claim.ClaimScope)),
		strings.ToLower(strings.TrimSpace(claim.PerspectiveOwner)),
	}, "|")
	sum := sha256.Sum256([]byte(key))
	return "state:" + hex.EncodeToString(sum[:12])
}

func narrativeStateOwnerScope(claim narrativeStateClaim) string {
	// One generic registry owns the lane. The semantic subject/perspective scope
	// remains in ValueJSON, while OwnerID identifies the exact current slot.
	return "entity"
}

func narrativeStateOwnerLabel(claim narrativeStateClaim) string {
	if claim.PerspectiveOwner != "" {
		return claim.PerspectiveOwner + " -> " + claim.Subject + " / " + claim.StateSlot
	}
	return claim.Subject + " / " + claim.StateSlot
}

func narrativeStateValuePayload(claim narrativeStateClaim, previousValue string, turnIndex int) map[string]any {
	return map[string]any{
		"contract_version":  narrativeStateContractVersion,
		"subject":           claim.Subject,
		"subject_type":      claim.SubjectType,
		"state_slot":        claim.StateSlot,
		"value":             claim.Value,
		"claim_scope":       claim.ClaimScope,
		"perspective_owner": claim.PerspectiveOwner,
		"transition":        claim.Transition,
		"confidence":        claim.Confidence,
		"previous_value":    strings.TrimSpace(previousValue),
		"source_turn":       turnIndex,
	}
}

func narrativeStateEvidencePayload(claim narrativeStateClaim, evidenceIDs []int64, turnIndex int) map[string]any {
	return map[string]any{
		"contract_version":    narrativeStateContractVersion,
		"source":              "critic." + claim.SourceKind,
		"source_index":        claim.SourceIndex,
		"source_turn":         turnIndex,
		"evidence_excerpt":    claim.EvidenceExcerpt,
		"direct_evidence_ids": evidenceIDs,
	}
}

func (s *Server) ensureNarrativeStateDefinition(ctx context.Context, sid string, now time.Time, result *artifactSaveResult) (store.StatusSchemaDefinition, bool) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		result.addSkipReason("narrative_state", "status_schema_registry_unavailable", nil)
		return store.StatusSchemaDefinition{}, false
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, narrativeStateStatusKey, "entity")
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("narrative_state", "status_schema_lookup_failed", err.Error())
		return store.StatusSchemaDefinition{}, false
	}
	result.Attempted++
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid,
		SchemaName:    "narrative_state",
		StatusKey:     narrativeStateStatusKey,
		Label:         "Narrative current state",
		OwnerScope:    "entity",
		ValueKind:     "note",
		OptionsJSON: mustCompactJSON(map[string]any{
			"contract_version":         narrativeStateContractVersion,
			"generic_state_lane":       true,
			"current_value_key":        "subject+state_slot+claim_scope+perspective_owner",
			"turn_is_audit_order_only": true,
		}),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil || len(definitions) == 0 {
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusSchemaDefinitions(narrative_state): "+err.Error())
			result.addSkipReason("narrative_state", "status_schema_create_failed", err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	result.StatusSchemaDefinitions++
	return definitions[0], true
}

func (s *Server) saveNarrativeStateFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, evidence []store.DirectEvidence, now time.Time, result *artifactSaveResult) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	claims := normalizeNarrativeStateClaims(extraction)
	events := sliceFromAny(extraction["narrative_events"])
	if len(claims) == 0 && len(events) == 0 {
		return
	}
	currentStore, currentOK := s.Store.(store.StatusCurrentValueStore)
	lifecycle, lifecycleOK := s.Store.(store.StatusLifecycleStore)
	if !currentOK || !lifecycleOK {
		result.addSkipReason("narrative_state", "status_current_or_lifecycle_store_unavailable", map[string]any{"claims": len(claims), "events": len(events)})
		return
	}
	definition, ok := s.ensureNarrativeStateDefinition(ctx, sid, now, result)
	if !ok {
		return
	}
	currentValues, err := currentStore.ListStatusCurrentValues(ctx, sid, "", "", narrativeStateStatusKey, 1000)
	if err != nil {
		result.addSkipReason("narrative_state", "current_state_read_failed", err.Error())
		return
	}
	currentByOwner := map[string]store.StatusCurrentValue{}
	for _, item := range currentValues {
		currentByOwner[item.OwnerID] = item
	}
	existingEvents, _ := lifecycle.ListStatusChangeEvents(ctx, sid, "", "", narrativeStateStatusKey, 1000)
	for _, claim := range claims {
		claim.EvidenceExcerpt = sanitizeEvidenceExcerptForTurn(claim.EvidenceExcerpt, content)
		if claim.EvidenceExcerpt == "" {
			result.addSkipReason("narrative_state", "evidence_excerpt_not_grounded", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot})
			continue
		}
		ownerID := narrativeStateOwnerID(claim)
		previous := currentByOwner[ownerID]
		previousPayload := map[string]any{}
		_ = json.Unmarshal([]byte(strings.TrimSpace(previous.ValueJSON)), &previousPayload)
		previousValue := strings.TrimSpace(extractionStringFromAny(previousPayload["value"]))
		if previous.SourceTurn > turnIndex && turnIndex > 0 {
			result.addSkipReason("narrative_state", "older_turn_cannot_replace_current_state", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot, "current_turn": previous.SourceTurn, "incoming_turn": turnIndex})
			continue
		}
		if previousValue != "" && normalizeArtifactDedupeText(previousValue) == normalizeArtifactDedupeText(claim.Value) {
			result.addSkipReason("narrative_state", "exact_current_value_reaffirmed", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot, "value": claim.Value})
			continue
		}
		evidenceIDs := narrativeStateMatchingEvidenceIDs(evidence, turnIndex, claim.EvidenceExcerpt)
		valuePayload := narrativeStateValuePayload(claim, previousValue, turnIndex)
		evidencePayload := narrativeStateEvidencePayload(claim, evidenceIDs, turnIndex)
		result.Attempted++
		saved, err := currentStore.SaveStatusCurrentValue(ctx, store.StatusCurrentValue{
			ChatSessionID: sid,
			RegistryID:    definition.ID,
			StatusKey:     narrativeStateStatusKey,
			OwnerScope:    narrativeStateOwnerScope(claim),
			OwnerID:       ownerID,
			OwnerLabel:    narrativeStateOwnerLabel(claim),
			ValueKind:     "note",
			ValueJSON:     mustCompactJSON(valuePayload),
			EvidenceJSON:  mustCompactJSON(evidencePayload),
			SourceTurn:    turnIndex,
			WriteState:    "current",
			CreatedAt:     now,
			UpdatedAt:     now,
		})
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusCurrentValue(narrative_state): "+err.Error())
			continue
		}
		result.NarrativeCurrentStates++
		eventKind := "set"
		if previousValue != "" {
			eventKind = "change"
		}
		result.Attempted++
		_, err = lifecycle.SaveStatusChangeEvent(ctx, store.StatusChangeEvent{
			ChatSessionID:     sid,
			RegistryID:        definition.ID,
			StatusValueID:     saved.ID,
			StatusKey:         narrativeStateStatusKey,
			OwnerScope:        narrativeStateOwnerScope(claim),
			OwnerID:           ownerID,
			EventKind:         eventKind,
			PreviousValueJSON: previous.ValueJSON,
			NewValueJSON:      saved.ValueJSON,
			EvidenceJSON:      saved.EvidenceJSON,
			SourceTurn:        turnIndex,
			EventState:        "recorded",
			CreatedAt:         now,
		})
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusChangeEvent(narrative_state): "+err.Error())
		} else {
			result.NarrativeStateEvents++
		}
		currentByOwner[ownerID] = saved
	}
	for index, raw := range events {
		item := mapFromAny(raw)
		evidenceExcerpt := sanitizeEvidenceExcerptForTurn(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence")), content)
		summary := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "summary"), stringFromMap(item, "event")))
		if summary == "" || evidenceExcerpt == "" {
			continue
		}
		ownerID := fmt.Sprintf("event:%d:%d", turnIndex, index)
		payload := map[string]any{"contract_version": narrativeStateContractVersion, "summary": summary, "event_type": stringFromMap(item, "event_type"), "participants": item["participants"], "source_turn": turnIndex}
		evidencePayload := map[string]any{"contract_version": narrativeStateContractVersion, "source": "critic.narrative_events", "source_index": index, "source_turn": turnIndex, "evidence_excerpt": evidenceExcerpt, "direct_evidence_ids": narrativeStateMatchingEvidenceIDs(evidence, turnIndex, evidenceExcerpt)}
		duplicate := false
		for _, existing := range existingEvents {
			if existing.OwnerID == ownerID && existing.SourceTurn == turnIndex && normalizeArtifactDedupeText(existing.NewValueJSON) == normalizeArtifactDedupeText(mustCompactJSON(payload)) {
				duplicate = true
				break
			}
		}
		if duplicate {
			result.addSkipReason("narrative_events", "exact_event_replay_skipped", map[string]any{"turn": turnIndex, "index": index})
			continue
		}
		result.Attempted++
		_, err := lifecycle.SaveStatusChangeEvent(ctx, store.StatusChangeEvent{ChatSessionID: sid, RegistryID: definition.ID, StatusKey: narrativeStateStatusKey, OwnerScope: "session", OwnerID: ownerID, EventKind: "event_observed", NewValueJSON: mustCompactJSON(payload), EvidenceJSON: mustCompactJSON(evidencePayload), SourceTurn: turnIndex, EventState: "recorded", CreatedAt: now})
		if err == nil {
			result.NarrativeStateEvents++
		} else {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusChangeEvent(narrative_event): "+err.Error())
		}
	}
}

func narrativeStateMatchingEvidenceIDs(evidence []store.DirectEvidence, turnIndex int, excerpt string) []int64 {
	needle := normalizeArtifactDedupeText(excerpt)
	ids := []int64{}
	for _, item := range evidence {
		if item.ID <= 0 || needle == "" {
			continue
		}
		anchor := item.TurnAnchor
		if anchor == 0 {
			anchor = item.SourceTurnStart
		}
		if turnIndex > 0 && anchor > 0 && anchor != turnIndex {
			continue
		}
		candidate := normalizeArtifactDedupeText(item.EvidenceText)
		if candidate == needle || strings.Contains(candidate, needle) || strings.Contains(needle, candidate) {
			ids = append(ids, item.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func restoreNarrativeCurrentStatesAfterRollback(ctx context.Context, st store.Store, sid string) (int, error) {
	currentStore, currentOK := st.(store.StatusCurrentValueStore)
	lifecycle, lifecycleOK := st.(store.StatusLifecycleStore)
	if !currentOK || !lifecycleOK {
		return 0, nil
	}
	events, err := lifecycle.ListStatusChangeEvents(ctx, sid, "", "", narrativeStateStatusKey, 1000)
	if err != nil {
		return 0, err
	}
	latest := map[string]store.StatusChangeEvent{}
	for _, event := range events {
		if event.StatusKey != narrativeStateStatusKey || event.EventKind == "event_observed" || strings.TrimSpace(event.NewValueJSON) == "" {
			continue
		}
		current, exists := latest[event.OwnerID]
		if !exists || event.SourceTurn > current.SourceTurn || (event.SourceTurn == current.SourceTurn && event.ID > current.ID) {
			latest[event.OwnerID] = event
		}
	}
	restored := 0
	for _, event := range latest {
		payload := map[string]any{}
		if json.Unmarshal([]byte(event.NewValueJSON), &payload) != nil {
			continue
		}
		claim := narrativeStateClaim{
			Subject:          strings.TrimSpace(extractionStringFromAny(payload["subject"])),
			StateSlot:        strings.TrimSpace(extractionStringFromAny(payload["state_slot"])),
			PerspectiveOwner: strings.TrimSpace(extractionStringFromAny(payload["perspective_owner"])),
		}
		_, err := currentStore.SaveStatusCurrentValue(ctx, store.StatusCurrentValue{
			ChatSessionID: sid,
			RegistryID:    event.RegistryID,
			StatusKey:     narrativeStateStatusKey,
			OwnerScope:    event.OwnerScope,
			OwnerID:       event.OwnerID,
			OwnerLabel:    narrativeStateOwnerLabel(claim),
			ValueKind:     "note",
			ValueJSON:     event.NewValueJSON,
			EvidenceJSON:  event.EvidenceJSON,
			SourceTurn:    event.SourceTurn,
			WriteState:    "current",
			CreatedAt:     event.CreatedAt,
			UpdatedAt:     time.Now().UTC(),
		})
		if err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

type narrativeCurrentStateView struct {
	Value       store.StatusCurrentValue
	Payload     map[string]any
	Subject     string
	Slot        string
	Current     string
	Previous    string
	Scope       string
	Perspective string
}

func prepareTurnPerspectiveWithNarrativeState(base map[string]any, values []store.StatusCurrentValue, activeStates []store.ActiveState) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	out["_narrative_current_values"] = values
	out["_narrative_active_states"] = activeStates
	return out
}

func prepareTurnNarrativeStateFromPerspective(args []map[string]any) ([]store.StatusCurrentValue, []store.ActiveState) {
	if len(args) == 0 || args[0] == nil {
		return nil, nil
	}
	values, _ := args[0]["_narrative_current_values"].([]store.StatusCurrentValue)
	activeStates, _ := args[0]["_narrative_active_states"].([]store.ActiveState)
	return values, activeStates
}

func narrativeCurrentStateViews(values []store.StatusCurrentValue) []narrativeCurrentStateView {
	out := []narrativeCurrentStateView{}
	for _, value := range values {
		if value.StatusKey != narrativeStateStatusKey || value.WriteState != "current" {
			continue
		}
		payload := map[string]any{}
		if json.Unmarshal([]byte(strings.TrimSpace(value.ValueJSON)), &payload) != nil {
			continue
		}
		view := narrativeCurrentStateView{Value: value, Payload: payload, Subject: strings.TrimSpace(extractionStringFromAny(payload["subject"])), Slot: strings.TrimSpace(extractionStringFromAny(payload["state_slot"])), Current: strings.TrimSpace(extractionStringFromAny(payload["value"])), Previous: strings.TrimSpace(extractionStringFromAny(payload["previous_value"])), Scope: normalizeNarrativeClaimScope(extractionStringFromAny(payload["claim_scope"])), Perspective: strings.TrimSpace(extractionStringFromAny(payload["perspective_owner"]))}
		if view.Subject == "" || view.Slot == "" || view.Current == "" {
			continue
		}
		out = append(out, view)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Value.SourceTurn > out[j].Value.SourceTurn })
	return out
}

func filterNarrativeCurrentStateViews(values []store.StatusCurrentValue, rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState) (facts, perceptions []narrativeCurrentStateView, dropped int) {
	context := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, activeStates, nil)
	relevanceText := strings.Join([]string{context.rawUserInput, context.immediateChatText, context.currentSceneStates}, "\n")
	for _, view := range narrativeCurrentStateViews(values) {
		subjectType := strings.TrimSpace(extractionStringFromAny(view.Payload["subject_type"]))
		global := subjectType == "world" || subjectType == "session"
		relevantSubject := prepareTurnAnyOwnerTokenMatches(prepareTurnOwnerTokens(view.Subject, view.Subject), relevanceText)
		relevantPerspective := view.Perspective != "" && prepareTurnAnyOwnerTokenMatches(prepareTurnOwnerTokens(view.Perspective, view.Perspective), relevanceText)
		if !global && !relevantSubject && !relevantPerspective {
			dropped++
			continue
		}
		switch view.Scope {
		case "belief", "rumor", "secret":
			if !relevantPerspective {
				dropped++
				continue
			}
			perceptions = append(perceptions, view)
		default:
			facts = append(facts, view)
		}
	}
	return facts, perceptions, dropped
}

func narrativeCorrectionContainsValue(haystack, value string) bool {
	needle := normalizeArtifactDedupeText(value)
	if len([]rune(needle)) < 3 {
		return false
	}
	return strings.Contains(normalizeArtifactDedupeText(haystack), needle)
}

func narrativeCorrectionMemoryText(selection prepareTurnMemoryLaneSelection) string {
	seen := map[int64]bool{}
	parts := []string{}
	appendItems := func(items []store.Memory) {
		for _, item := range items {
			if item.ID > 0 && seen[item.ID] {
				continue
			}
			if item.ID > 0 {
				seen[item.ID] = true
			}
			if summary := strings.TrimSpace(prepareTurnMemorySummary(item)); summary != "" {
				parts = append(parts, summary)
			}
		}
	}
	appendItems(selection.VectorRelevant)
	appendItems(selection.Relevant)
	appendItems(selection.Recent)
	appendItems(selection.Deep)
	return strings.Join(parts, "\n")
}

func narrativeCorrectionTransitionNeedsCarry(payload map[string]any) bool {
	switch normalizeNarrativeTransition(extractionStringFromAny(payload["transition"])) {
	case "change", "reversal", "recovery", "correction", "reveal", "resolve", "clear":
		return true
	default:
		return false
	}
}

func buildNarrativeContinuityCorrection(values []store.StatusCurrentValue, rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, selection prepareTurnMemoryLaneSelection, limit int) (string, map[string]any) {
	limit = prepareTurnRecallLimit(limit)
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, activeStates, nil)
	recentText := strings.Join([]string{ctx.rawUserInput, ctx.immediateChatText, ctx.currentSceneStates}, "\n")
	memoryText := narrativeCorrectionMemoryText(selection)
	facts, perceptions, irrelevantDropped := filterNarrativeCurrentStateViews(values, rawUserInput, chatLogs, activeStates)
	lines := []string{
		"Use these corrections only to prevent contradictions or restore omitted continuity. Do not expand them into new events.",
	}
	selected := 0
	conflictCount := 0
	omissionCount := 0
	redundantDropped := 0
	notNeededDropped := 0

	appendView := func(view narrativeCurrentStateView, perspective bool) {
		if selected >= limit {
			return
		}
		currentInRecent := narrativeCorrectionContainsValue(recentText, view.Current)
		if currentInRecent {
			redundantDropped++
			return
		}
		currentInMemory := narrativeCorrectionContainsValue(memoryText, view.Current)
		previousInRecent := view.Previous != "" && narrativeCorrectionContainsValue(recentText, view.Previous)
		previousInMemory := view.Previous != "" && narrativeCorrectionContainsValue(memoryText, view.Previous)
		conflict := previousInRecent || previousInMemory
		explicitSubject := prepareTurnAnyOwnerTokenMatches(prepareTurnOwnerTokens(view.Subject, view.Subject), ctx.rawUserInput)
		explicitPerspective := view.Perspective != "" && prepareTurnAnyOwnerTokenMatches(prepareTurnOwnerTokens(view.Perspective, view.Perspective), ctx.rawUserInput)
		omission := !currentInMemory && (explicitSubject || explicitPerspective) && (view.Previous != "" || narrativeCorrectionTransitionNeedsCarry(view.Payload))
		if !conflict && !omission {
			notNeededDropped++
			return
		}
		reason := "missing current continuity"
		if conflict {
			reason = "supersedes conflicting older state"
			conflictCount++
		} else {
			omissionCount++
		}
		if perspective {
			owner := view.Perspective
			if owner == "" {
				owner = view.Subject
			}
			lines = append(lines, fmt.Sprintf("- Perspective only: %s currently believes about %s / %s: %s (%s).", owner, view.Subject, view.Slot, compactPrepareTurnLine(view.Current, 180), reason))
		} else {
			lines = append(lines, fmt.Sprintf("- Current continuity: %s / %s: %s (%s).", view.Subject, view.Slot, compactPrepareTurnLine(view.Current, 180), reason))
		}
		selected++
	}
	for _, view := range facts {
		appendView(view, false)
	}
	for _, view := range perceptions {
		appendView(view, true)
	}

	text := ""
	if selected > 0 {
		text = makePrepareTurnSection("[Continuity Correction]", lines)
	}
	trace := map[string]any{
		"policy_version":             "continuity_correction.v1",
		"mode":                       "conditional_postscript",
		"selected_count":             selected,
		"conflict_count":             conflictCount,
		"omission_count":             omissionCount,
		"already_present_dropped":    redundantDropped,
		"not_needed_dropped":         notNeededDropped,
		"irrelevant_state_dropped":   irrelevantDropped,
		"memory_candidates_compared": len(selection.VectorRelevant) + len(selection.Relevant) + len(selection.Recent) + len(selection.Deep),
		"injection_rule":             "append_only_when_retrieved_or_recent_context_conflicts_with_or_omits_a_relevant_changed_current_state",
	}
	return text, trace
}

func filterMemorySelectionAgainstNarrativeCurrentState(selection prepareTurnMemoryLaneSelection, values []store.StatusCurrentValue) prepareTurnMemoryLaneSelection {
	views := narrativeCurrentStateViews(values)
	dropped := 0
	filter := func(items []store.Memory) []store.Memory {
		out := make([]store.Memory, 0, len(items))
		for _, memory := range items {
			summary := normalizeArtifactDedupeText(memorySummaryText(memory))
			conflicts := false
			for _, view := range views {
				if view.Previous == "" || view.Value.SourceTurn <= 0 || memory.TurnIndex >= view.Value.SourceTurn {
					continue
				}
				previous := normalizeArtifactDedupeText(view.Previous)
				subject := normalizeArtifactDedupeText(view.Subject)
				if previous != "" && strings.Contains(summary, previous) && (subject == "" || strings.Contains(summary, subject)) {
					conflicts = true
					break
				}
			}
			if conflicts {
				dropped++
				continue
			}
			out = append(out, memory)
		}
		return out
	}
	selection.VectorRelevant = filter(selection.VectorRelevant)
	selection.Relevant = filter(selection.Relevant)
	selection.Deep = filter(selection.Deep)
	selection.Recent = filter(selection.Recent)
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["superseded_state_memory_dropped_count"] = dropped
	selection.Trace["superseded_state_memory_gate"] = "older_memory_containing_explicit_previous_value_is_not_injected"
	return selection
}
