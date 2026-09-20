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
	narrativeStateContractVersion   = "narrative_state.v1"
	narrativeStateStatusKey         = "narrative_state"
	narrativeStateMinimumConfidence = 0.7
)

type narrativeStateClaim struct {
	Subject          string
	SubjectType      string
	StateSlot        string
	LifecycleKey     string
	Value            string
	ClaimScope       string
	PerspectiveOwner string
	Transition       string
	Confidence       float64
	EvidenceExcerpt  string
	SourceKind       string
	SourceIndex      int
	PendingThread    *store.PendingThread
	LifecycleDetails map[string]any
	SourceFields     []string
	TemporalContext  map[string]any
}

func normalizeNarrativeStateClaims(extraction map[string]any) []narrativeStateClaim {
	out := []narrativeStateClaim{}
	resolvedLifecycleKeys, resolvedLegacySubjects := narrativeResolvedThreadIdentities(extraction)
	appendClaim := func(raw any, sourceKind string, index int) {
		item := mapFromAny(raw)
		if len(item) == 0 {
			return
		}
		claim := narrativeStateClaim{
			Subject:          strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "subject"), stringFromMap(item, "entity"), stringFromMap(item, "owner"))),
			SubjectType:      normalizeNarrativeSubjectType(stringFromMap(item, "subject_type")),
			StateSlot:        normalizeNarrativeStateSlot(extractionFirstNonEmpty(stringFromMap(item, "state_slot"), stringFromMap(item, "slot"), stringFromMap(item, "relation_dimension"))),
			LifecycleKey:     normalizeNarrativeLifecycleKey(stringFromMap(item, "lifecycle_key")),
			Value:            strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "value"), stringFromMap(item, "state_value"), stringFromMap(item, "belief"))),
			ClaimScope:       normalizeNarrativeClaimScope(extractionFirstNonEmpty(stringFromMap(item, "claim_scope"), sourceKind)),
			PerspectiveOwner: strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "perspective_owner"), stringFromMap(item, "believer"), stringFromMap(item, "knower"))),
			Transition:       normalizeNarrativeTransition(stringFromMap(item, "transition")),
			Confidence:       clampFloat(extractionFloatFromAny(item["confidence"], 0.8), 0, 1),
			EvidenceExcerpt:  strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence"))),
			SourceKind:       sourceKind,
			SourceIndex:      index,
			LifecycleDetails: item,
			SourceFields:     stringsFromAny(item["source_fields"]),
			TemporalContext:  map[string]any{},
		}
		for _, key := range []string{"observed_at", "occurrence_time", "validity", "scene_scope", "temporal_context", "relative_expression", "relative"} {
			if value, exists := item[key]; exists {
				claim.TemporalContext[key] = value
			}
		}
		if claim.SubjectType == "" {
			claim.SubjectType = "entity"
		}
		if claim.Transition == "" {
			claim.Transition = "set"
		}
		if claim.Transition == "set" && ((claim.LifecycleKey != "" && resolvedLifecycleKeys[claim.LifecycleKey]) ||
			(claim.LifecycleKey == "" && resolvedLegacySubjects[normalizeArtifactDedupeText(claim.Subject)])) {
			claim.Transition = "resolve"
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
	return out
}

func normalizeNarrativeLifecycleKey(raw string) string {
	return normalizeArtifactDedupeText(raw)
}

func narrativeLifecycleStorageKey(raw string) string {
	key := normalizeNarrativeLifecycleKey(raw)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("narrative-lifecycle.v1\x1f" + key))
	return "thread_lifecycle_" + hex.EncodeToString(sum[:12])
}

func narrativeResolvedThreadIdentities(extraction map[string]any) (map[string]bool, map[string]bool) {
	lifecycleKeys := map[string]bool{}
	legacySubjects := map[string]bool{}
	appendItems := func(raw any) {
		for _, value := range sliceFromAny(raw) {
			item := mapFromAny(value)
			if len(item) == 0 {
				if subject := normalizeArtifactDedupeText(extractionStringFromAny(value)); subject != "" {
					legacySubjects[subject] = true
				}
				continue
			}
			if key := normalizeNarrativeLifecycleKey(stringFromMap(item, "lifecycle_key")); key != "" {
				lifecycleKeys[key] = true
			}
			if subject := normalizeArtifactDedupeText(extractionFirstNonEmpty(
				stringFromMap(item, "subject"), stringFromMap(item, "title"),
				stringFromMap(item, "description"),
			)); subject != "" {
				legacySubjects[subject] = true
			}
		}
	}
	appendItems(extraction["resolved_threads"])
	stateDeltas := mapFromAny(extraction["state_deltas"])
	appendItems(stateDeltas["resolved_threads"])
	appendItems(mapFromAny(stateDeltas["unresolved_threads"])["resolved"])
	return lifecycleKeys, legacySubjects
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
	case "set", "reaffirm", "change", "reversal", "recovery", "correction", "reveal", "resolve", "uncertain", "clear",
		"defer", "abandon", "complete", "supersede", "reopen", "resume", "create", "progress", "partial", "cancel", "modify", "pause", "missed", "refused", "impossible":
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
	parts := []string{
		strings.ToLower(strings.TrimSpace(claim.SubjectType)),
		strings.ToLower(strings.TrimSpace(claim.Subject)),
		strings.ToLower(strings.TrimSpace(claim.StateSlot)),
		strings.ToLower(strings.TrimSpace(claim.ClaimScope)),
		strings.ToLower(strings.TrimSpace(claim.PerspectiveOwner)),
	}
	if claim.LifecycleKey != "" {
		parts = []string{
			"lifecycle",
			normalizeNarrativeLifecycleKey(claim.LifecycleKey),
			strings.ToLower(strings.TrimSpace(claim.ClaimScope)),
			strings.ToLower(strings.TrimSpace(claim.PerspectiveOwner)),
		}
	}
	key := strings.Join(parts, "|")
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
	payload := map[string]any{
		"contract_version":  narrativeStateContractVersion,
		"subject":           claim.Subject,
		"subject_type":      claim.SubjectType,
		"state_slot":        claim.StateSlot,
		"lifecycle_key":     claim.LifecycleKey,
		"value":             claim.Value,
		"claim_scope":       claim.ClaimScope,
		"perspective_owner": claim.PerspectiveOwner,
		"transition":        claim.Transition,
		"confidence":        claim.Confidence,
		"previous_value":    strings.TrimSpace(previousValue),
		"source_turn":       turnIndex,
	}
	if claim.PendingThread != nil {
		payload["pending_thread"] = claim.PendingThread
	}
	if len(claim.LifecycleDetails) > 0 && narrativeClaimIsLifecycle(claim) {
		payload["lifecycle_details"] = claim.LifecycleDetails
	}
	if len(claim.SourceFields) > 0 {
		payload["source_fields"] = claim.SourceFields
	}
	for key, value := range claim.TemporalContext {
		payload[key] = value
	}
	return payload
}

func narrativeStateEvidencePayload(claim narrativeStateClaim, evidenceIDs []int64, turnIndex int, sourceRevision string) map[string]any {
	payload := map[string]any{
		"contract_version":    narrativeStateContractVersion,
		"source":              "critic." + claim.SourceKind,
		"source_index":        claim.SourceIndex,
		"source_turn":         turnIndex,
		"evidence_excerpt":    claim.EvidenceExcerpt,
		"direct_evidence_ids": evidenceIDs,
		"current_projection":  true,
	}
	if sourceRevision = strings.TrimSpace(sourceRevision); sourceRevision != "" {
		payload["source_revision"] = sourceRevision
	}
	return payload
}

func narrativeClaimIsLifecycle(claim narrativeStateClaim) bool {
	return claim.ClaimScope == "objective" && (claim.LifecycleKey != "" ||
		(claim.SubjectType == "entity" && claim.StateSlot == "goal_status"))
}

func narrativeLifecycleProjectionStatus(transition string) string {
	switch transition {
	case "defer", "pause":
		return "paused"
	case "abandon", "cancel", "complete", "supersede", "resolve", "clear":
		return "resolved"
	default:
		return "open"
	}
}

func narrativePendingSnapshot(payload map[string]any) *store.PendingThread {
	raw := mapFromAny(payload["pending_thread"])
	if len(raw) == 0 {
		return nil
	}
	var thread store.PendingThread
	if json.Unmarshal([]byte(mustCompactJSON(raw)), &thread) != nil || thread.ThreadKey == "" {
		return nil
	}
	return &thread
}

func narrativePendingClaim(thread store.PendingThread, sourceKind string, index int) narrativeStateClaim {
	metadata := parseJSONMap(thread.HookMetadataJSON)
	subject := strings.TrimSpace(extractionFirstNonEmpty(thread.Title, stringFromMap(metadata, "title"), thread.Description))
	transition := normalizeNarrativeTransition(stringFromMap(metadata, "transition"))
	if transition == "" {
		transition = "set"
	}
	return narrativeStateClaim{
		Subject: subject, SubjectType: "entity", StateSlot: "goal_status",
		LifecycleKey: normalizeNarrativeLifecycleKey(stringFromMap(metadata, "lifecycle_key")),
		Value:        extractionFirstNonEmpty(stringFromMap(metadata, "value"), thread.Description, subject),
		ClaimScope:   "objective", Transition: transition, Confidence: thread.Confidence,
		EvidenceExcerpt: extractionFirstNonEmpty(stringFromMap(metadata, "evidence_excerpt"), stringFromMap(metadata, "evidence")),
		SourceKind:      sourceKind, SourceIndex: index, PendingThread: &thread, LifecycleDetails: metadata,
	}
}

func narrativePendingThreadForExtraction(sid string, turnIndex int, raw map[string]any, now time.Time) store.PendingThread {
	title := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(raw, "title"), stringFromMap(raw, "description"), stringFromMap(raw, "thread_type")))
	key := stableKey("thread", title)
	if lifecycleKey := narrativeLifecycleStorageKey(stringFromMap(raw, "lifecycle_key")); lifecycleKey != "" {
		key = lifecycleKey
	}
	return store.PendingThread{
		ChatSessionID: sid, ThreadKey: key, Title: title,
		Description: extractionFirstNonEmpty(stringFromMap(raw, "details"), title), Status: "open",
		CreatedTurn: turnIndex, SourceTurn: turnIndex, Priority: intFromAny(raw["priority"], 0),
		HookType: stringFromMap(raw, "thread_type"), ThreadType: stringFromMap(raw, "thread_type"),
		HookMetadataJSON: mustCompactJSON(raw), DetailsJSON: mustCompactJSON(raw),
		Owner: sanitizeParticipantActorName(stringFromMap(raw, "owner")), Target: sanitizeParticipantActorName(stringFromMap(raw, "target")),
		LastSeenTurn: turnIndex, Confidence: clampFloat(extractionFloatFromAny(raw["confidence"], 0), 0, 1), CreatedAt: now, UpdatedAt: now,
	}
}

// Legacy pending/resolution inputs already carry a Critic decision. Normalize
// them into the same state owner without requiring a redundant state_claim.
func narrativeLifecycleClaims(claims []narrativeStateClaim, extraction map[string]any, currents []store.StatusCurrentValue, threads []store.PendingThread, sid string, turnIndex int, now time.Time) []narrativeStateClaim {
	byOwner := map[string]int{}
	for i := range claims {
		byOwner[narrativeStateOwnerID(claims[i])] = i
	}
	threadByKey := map[string]store.PendingThread{}
	for _, thread := range threads {
		if _, exists := threadByKey[thread.ThreadKey]; !exists {
			threadByKey[thread.ThreadKey] = thread
		}
	}
	for _, current := range currents {
		if snapshot := narrativePendingSnapshot(parseJSONMap(current.ValueJSON)); snapshot != nil {
			if _, found := threadByKey[snapshot.ThreadKey]; !found {
				threadByKey[snapshot.ThreadKey] = *snapshot
			}
		}
	}
	for i, raw := range sliceFromAny(extraction["pending_threads"]) {
		thread := narrativePendingThreadForExtraction(sid, turnIndex, mapFromAny(raw), now)
		if thread.Title == "" {
			continue
		}
		if previous, ok := threadByKey[thread.ThreadKey]; ok {
			thread.ID, thread.CreatedTurn, thread.CreatedAt = previous.ID, previous.CreatedTurn, previous.CreatedAt
		}
		threadByKey[thread.ThreadKey] = thread
		claim := narrativePendingClaim(thread, "pending_threads", i)
		ownerID := narrativeStateOwnerID(claim)
		if index, exists := byOwner[ownerID]; exists {
			claims[index].PendingThread = &thread
			// Preserve the independently accepted legacy pending input. A valid
			// explicit transition is processed first; the existing current-state
			// rules make its accompanying pending mention a reaffirmation.
			claims = append(claims, claim)
		} else {
			byOwner[ownerID] = len(claims)
			claims = append(claims, claim)
		}
	}
	resolved := []any{}
	resolved = append(resolved, sliceFromAny(extraction["resolved_threads"])...)
	deltas := mapFromAny(extraction["state_deltas"])
	resolved = append(resolved, sliceFromAny(deltas["resolved_threads"])...)
	resolved = append(resolved, sliceFromAny(mapFromAny(deltas["unresolved_threads"])["resolved"])...)
	for i, raw := range resolved {
		item := mapFromAny(raw)
		key := normalizeNarrativeLifecycleKey(stringFromMap(item, "lifecycle_key"))
		subject := extractionFirstNonEmpty(stringFromMap(item, "subject"), stringFromMap(item, "title"), stringFromMap(item, "description"))
		if len(item) == 0 {
			subject = extractionStringFromAny(raw)
		}
		matches := []store.PendingThread{}
		for _, thread := range threadByKey {
			metadata := parseJSONMap(thread.HookMetadataJSON)
			threadLifecycle := normalizeNarrativeLifecycleKey(stringFromMap(metadata, "lifecycle_key"))
			matched := key != "" && key == threadLifecycle
			if key == "" && threadLifecycle == "" {
				for _, value := range []string{thread.Title, thread.Description, stringFromMap(metadata, "subject"), stringFromMap(metadata, "title")} {
					if subject != "" && normalizeArtifactDedupeText(subject) == normalizeArtifactDedupeText(value) {
						matched = true
					}
				}
			}
			if matched {
				matches = append(matches, thread)
			}
		}
		if len(matches) == 0 && (key != "" || subject != "") {
			if subject == "" {
				for _, current := range currents {
					payload := parseJSONMap(current.ValueJSON)
					if normalizeNarrativeLifecycleKey(stringFromMap(payload, "lifecycle_key")) == key {
						subject = stringFromMap(payload, "subject")
						break
					}
				}
			}
			metadata := map[string]any{"title": subject, "lifecycle_key": key}
			unknownOrigin := narrativePendingThreadForExtraction(sid, turnIndex, metadata, now)
			unknownOrigin.CreatedTurn = 0
			matches = append(matches, unknownOrigin)
		}
		for _, thread := range matches {
			claim := narrativePendingClaim(thread, "resolved_threads", i)
			claim.Transition = "resolve"
			claim.Value = extractionFirstNonEmpty(stringFromMap(item, "value"), stringFromMap(item, "resolution_note"), "resolved")
			claim.EvidenceExcerpt = extractionFirstNonEmpty(stringFromMap(item, "evidence_excerpt"), stringFromMap(item, "evidence"))
			claim.LifecycleDetails = item
			if thread.CreatedTurn == 0 && thread.ID == 0 {
				claim.PendingThread = nil
			}
			ownerID := narrativeStateOwnerID(claim)
			if index, exists := byOwner[ownerID]; exists {
				if narrativeLifecycleProjectionStatus(claims[index].Transition) != "open" {
					claims[index].PendingThread = claim.PendingThread
					claims = append(claims, claim)
					continue
				}
				claims[index] = claim
			} else {
				byOwner[ownerID] = len(claims)
				claims = append(claims, claim)
			}
		}
	}
	for i := range claims {
		if !narrativeClaimIsLifecycle(claims[i]) || claims[i].PendingThread != nil {
			continue
		}
		key := narrativeLifecycleStorageKey(claims[i].LifecycleKey)
		if key == "" {
			key = stableKey("thread", claims[i].Subject)
		}
		if thread, ok := threadByKey[key]; ok {
			claims[i].PendingThread = &thread
		}
	}
	sort.SliceStable(claims, func(i, j int) bool {
		return claims[i].SourceKind != "pending_threads" && claims[j].SourceKind == "pending_threads"
	})
	return claims
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
			"current_value_key":        "lifecycle_key_or_subject+state_slot+claim_scope+perspective_owner",
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
	resolvedKeys, resolvedSubjects := narrativeResolvedThreadIdentities(extraction)
	if len(claims) == 0 && len(events) == 0 && len(sliceFromAny(extraction["pending_threads"])) == 0 && len(resolvedKeys) == 0 && len(resolvedSubjects) == 0 {
		return
	}
	sourceRevision := ""
	sourceContract := ""
	if sourceContext, ok := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext); ok {
		sourceRevision = strings.TrimSpace(sourceContext.Revision)
		sourceContract = sourceContext.ContractVersion
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
	currentValues, err := currentStore.ListStatusCurrentValues(ctx, sid, "entity", "", narrativeStateStatusKey, -1)
	if err != nil {
		result.addSkipReason("narrative_state", "current_state_read_failed", err.Error())
		return
	}
	currentByOwner := map[string]store.StatusCurrentValue{}
	for _, item := range currentValues {
		currentByOwner[item.OwnerID] = item
	}
	threads, threadErr := s.Store.ListPendingThreads(ctx, sid, "all")
	if threadErr != nil {
		result.addSkipReason("narrative_state", "pending_projection_read_failed", threadErr.Error())
	}
	claims = narrativeLifecycleClaims(claims, extraction, currentValues, threads, sid, turnIndex, now)
	existingEvents, _ := lifecycle.ListStatusChangeEvents(ctx, sid, "", "", narrativeStateStatusKey, 1000)
	observationContext := reversibleObservationContext(ctx, s.Store, sid)
	acceptedTransitions := map[string]bool{}
	for _, claim := range claims {
		ownerID := narrativeStateOwnerID(claim)
		if (claim.SourceKind == "pending_threads" || claim.SourceKind == "resolved_threads") && acceptedTransitions[ownerID] {
			// A legacy item accompanying an accepted transition describes that
			// same projection; it is not another write decision or failure route.
			continue
		}
		legacyLifecycleInput := claim.SourceKind == "pending_threads" || claim.SourceKind == "resolved_threads"
		claim.EvidenceExcerpt = sanitizeEvidenceExcerptForTurn(claim.EvidenceExcerpt, content)
		if claim.EvidenceExcerpt == "" && !legacyLifecycleInput {
			result.addSkipReason("narrative_state", "evidence_excerpt_not_grounded", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot})
			continue
		}
		goalLifecycle := narrativeClaimIsLifecycle(claim)
		if goalLifecycle && claim.Confidence < narrativeStateMinimumConfidence && !legacyLifecycleInput {
			result.addSkipReason("narrative_state", "low_confidence_current_state_change", map[string]any{
				"subject": claim.Subject, "state_slot": claim.StateSlot, "confidence": claim.Confidence,
			})
			continue
		}
		previous := currentByOwner[ownerID]
		previousPayload := map[string]any{}
		_ = json.Unmarshal([]byte(strings.TrimSpace(previous.ValueJSON)), &previousPayload)
		previousEventJSON := previous.ValueJSON
		if claim.PendingThread != nil && narrativePendingSnapshot(previousPayload) == nil {
			for _, thread := range threads {
				if thread.ThreadKey == claim.PendingThread.ThreadKey {
					priorPayload := parseJSONMap(previous.ValueJSON)
					priorPayload["pending_thread"] = thread
					previousEventJSON = mustCompactJSON(priorPayload)
					break
				}
			}
		}
		previousValue := strings.TrimSpace(extractionStringFromAny(previousPayload["value"]))
		if store.StatusCurrentObservationTurn(previous) > turnIndex && turnIndex > 0 {
			result.addSkipReason("narrative_state", "older_turn_cannot_replace_current_state", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot, "current_turn": store.StatusCurrentObservationTurn(previous), "incoming_turn": turnIndex})
			continue
		}
		previousTransition := normalizeNarrativeTransition(extractionStringFromAny(previousPayload["transition"]))
		incomingClosing := narrativeLifecycleProjectionStatus(claim.Transition) != "open"
		sameValue := previousValue != "" && normalizeArtifactDedupeText(previousValue) == normalizeArtifactDedupeText(claim.Value)
		progressEvidenceChanged := goalLifecycle && (claim.Transition == "progress" || claim.Transition == "partial" || claim.Transition == "modify" || claim.Transition == "missed" || claim.Transition == "refused" || claim.Transition == "impossible") &&
			(mustCompactJSON(previousPayload["lifecycle_details"]) != mustCompactJSON(claim.LifecycleDetails) ||
				stringFromMap(parseJSONMap(previous.EvidenceJSON), "evidence_excerpt") != claim.EvidenceExcerpt)
		if sameValue && (!goalLifecycle || (previousTransition == claim.Transition && !progressEvidenceChanged) || claim.Transition == "set" || claim.Transition == "reaffirm" || claim.Transition == "uncertain") {
			result.addSkipReason("narrative_state", "exact_current_value_reaffirmed", map[string]any{"subject": claim.Subject, "state_slot": claim.StateSlot, "value": claim.Value})
			continue
		}
		if previousValue != "" && goalLifecycle {
			confirmedReplacement := false
			switch claim.Transition {
			case "change", "reversal", "recovery", "correction", "reveal", "resolve", "clear",
				"defer", "abandon", "complete", "supersede", "reopen", "resume", "progress", "partial", "cancel", "modify", "pause", "missed", "refused", "impossible":
				confirmedReplacement = true
			}
			if !confirmedReplacement {
				result.addSkipReason("narrative_state", "non_final_transition_cannot_replace_current_state", map[string]any{
					"subject": claim.Subject, "state_slot": claim.StateSlot, "transition": claim.Transition,
				})
				continue
			}
			previousClosed := narrativeLifecycleProjectionStatus(previousTransition) != "open"
			explicitReactivation := claim.Transition == "reopen" ||
				claim.Transition == "resume" ||
				claim.Transition == "correction" ||
				claim.Transition == "reversal"
			if previousClosed && !incomingClosing && !explicitReactivation {
				result.addSkipReason("narrative_state", "closed_state_requires_explicit_reactivation_transition", map[string]any{
					"subject": claim.Subject, "state_slot": claim.StateSlot,
					"current_transition": previousTransition, "incoming_transition": claim.Transition,
				})
				continue
			}
		}
		acceptedTransitions[ownerID] = true
		if len(claim.SourceFields) == 0 {
			claim.SourceFields = stringsFromAny(previousPayload["source_fields"])
		}
		if claim.PendingThread == nil {
			claim.PendingThread = narrativePendingSnapshot(previousPayload)
		}
		if claim.PendingThread != nil {
			thread := *claim.PendingThread
			if predecessor := narrativePendingSnapshot(previousPayload); predecessor != nil {
				thread.CreatedTurn, thread.CreatedAt = predecessor.CreatedTurn, predecessor.CreatedAt
			}
			thread.Status = narrativeLifecycleProjectionStatus(claim.Transition)
			thread.SourceTurn, thread.LastSeenTurn, thread.UpdatedAt = turnIndex, turnIndex, now
			thread.ResolvedTurn = 0
			if thread.Status == "resolved" {
				thread.ResolvedTurn = turnIndex
				thread.ResolutionNote = claim.Value
			}
			metadata := parseJSONMap(thread.HookMetadataJSON)
			metadata["status"], metadata["transition"], metadata["value"] = thread.Status, claim.Transition, claim.Value
			metadata["source_turn"] = turnIndex
			if len(claim.LifecycleDetails) > 0 {
				metadata["lifecycle_details"] = claim.LifecycleDetails
			}
			thread.HookMetadataJSON = mustCompactJSON(metadata)
			claim.PendingThread = &thread
		}
		evidenceIDs := narrativeStateMatchingEvidenceIDs(evidence, turnIndex, claim.EvidenceExcerpt)
		valuePayload := narrativeStateValuePayload(claim, previousValue, turnIndex)
		if _, explicit := valuePayload["observed_at"]; !explicit {
			valuePayload["observed_at"] = observationContext
		}
		evidencePayload := narrativeStateEvidencePayload(claim, evidenceIDs, turnIndex, sourceRevision)
		evidencePayload["source_unit_id"] = fmt.Sprintf("narrative:%s:%s:%d:%d", ownerID, claim.SourceKind, claim.SourceIndex, turnIndex)
		result.Attempted++
		currentValue := store.StatusCurrentValue{
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
		}
		eventKind := "set"
		if previousValue != "" {
			eventKind = "change"
		}
		event := store.StatusChangeEvent{
			ChatSessionID:     sid,
			RegistryID:        definition.ID,
			StatusKey:         narrativeStateStatusKey,
			OwnerScope:        narrativeStateOwnerScope(claim),
			OwnerID:           ownerID,
			EventKind:         eventKind,
			PreviousValueJSON: previousEventJSON,
			NewValueJSON:      currentValue.ValueJSON,
			EvidenceJSON:      currentValue.EvidenceJSON,
			SourceTurn:        turnIndex,
			StoryClockJSON:    reversibleObservationStoryClockJSON(mapFromAny(valuePayload["observed_at"])),
			EventState:        "recorded",
			CreatedAt:         now,
		}
		atomicStore, atomicOK := s.Store.(store.ReversibleStatusTransitionStore)
		if atomicOK && sourceRevision != "" && sourceContract == completeTurnSourceAcceptanceContract {
			saved, err := atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
				SourceContract: sourceContract, SourceRevision: sourceRevision,
				SourceUnitID: extractionStringFromAny(evidencePayload["source_unit_id"]), CurrentValue: &currentValue, Event: event,
			})
			if err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, "ApplyReversibleStatusTransition(narrative_state): "+err.Error())
				continue
			}
			if !saved.Replayed {
				result.NarrativeCurrentStates++
				result.NarrativeStateEvents++
				if claim.PendingThread != nil {
					result.PendingThreads++
				}
			}
			currentByOwner[ownerID] = saved.CurrentValue
		} else {
			// Existing unversioned/import callers retain their established write path.
			saved, err := currentStore.SaveStatusCurrentValue(ctx, currentValue)
			if err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, "SaveStatusCurrentValue(narrative_state): "+err.Error())
				continue
			}
			result.NarrativeCurrentStates++
			result.Attempted++
			event.StatusValueID = saved.ID
			if _, err := lifecycle.SaveStatusChangeEvent(ctx, event); err != nil {
				result.Errors++
				result.ErrorDetails = append(result.ErrorDetails, "SaveStatusChangeEvent(narrative_state): "+err.Error())
			} else {
				result.NarrativeStateEvents++
			}
			currentByOwner[ownerID] = saved
		}
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
		payload["observed_at"] = observationContext
		for _, key := range []string{"observed_at", "occurrence_time", "temporal_context", "relative_expression", "relative"} {
			if value, exists := item[key]; exists {
				payload[key] = value
			}
		}
		evidencePayload := map[string]any{"contract_version": narrativeStateContractVersion, "source": "critic.narrative_events", "source_index": index, "source_turn": turnIndex, "evidence_excerpt": evidenceExcerpt, "direct_evidence_ids": narrativeStateMatchingEvidenceIDs(evidence, turnIndex, evidenceExcerpt)}
		if sourceRevision != "" {
			evidencePayload["source_revision"] = sourceRevision
		}
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
		_, err := lifecycle.SaveStatusChangeEvent(ctx, store.StatusChangeEvent{ChatSessionID: sid, RegistryID: definition.ID, StatusKey: narrativeStateStatusKey, OwnerScope: "session", OwnerID: ownerID, EventKind: "event_observed", NewValueJSON: mustCompactJSON(payload), EvidenceJSON: mustCompactJSON(evidencePayload), SourceTurn: turnIndex, StoryClockJSON: reversibleObservationStoryClockJSON(mapFromAny(payload["observed_at"])), EventState: "recorded", CreatedAt: now})
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

func restoreNarrativeCurrentStatesAfterRollback(ctx context.Context, st store.Store, sid string, maxSourceTurn int) (int, error) {
	currentStore, currentOK := st.(store.StatusCurrentValueStore)
	lifecycle, lifecycleOK := st.(store.StatusLifecycleStore)
	if !currentOK || !lifecycleOK || maxSourceTurn < 0 {
		return 0, nil
	}
	var events []store.StatusChangeEvent
	var err error
	if reversible, ok := st.(store.ReversibleStatusTransitionStore); ok {
		events, err = reversible.ListLatestReversibleCurrentProjectionEvents(ctx, sid, []string{narrativeStateStatusKey})
		if err == nil {
			legacyEvents, legacyErr := lifecycle.ListStatusChangeEvents(ctx, sid, "", "", narrativeStateStatusKey, -1)
			if legacyErr != nil {
				return 0, legacyErr
			}
			for _, event := range legacyEvents {
				if stringFromMap(parseJSONMap(event.EvidenceJSON), "source_revision") == "" {
					events = append(events, event)
				}
			}
		}
	} else {
		events, err = lifecycle.ListStatusChangeEvents(ctx, sid, "", "", narrativeStateStatusKey, -1)
	}
	if err != nil {
		return 0, err
	}
	latest := map[string]store.StatusChangeEvent{}
	seenEvents := map[int64]bool{}
	for _, event := range events {
		if event.ID > 0 && seenEvents[event.ID] {
			continue
		}
		seenEvents[event.ID] = event.ID > 0
		evidence := parseJSONMap(event.EvidenceJSON)
		unknownRepairSource := event.SourceTurn == 0 && stringFromMap(evidence, "source_contract") == store.StateRepairContract
		if event.StatusKey != narrativeStateStatusKey || event.EventKind == "event_observed" || strings.TrimSpace(event.NewValueJSON) == "" || (event.SourceTurn <= 0 && !unknownRepairSource) || event.SourceTurn > maxSourceTurn {
			continue
		}
		current, exists := latest[event.OwnerID]
		observedTurn, previousObservedTurn := store.StatusChangeEventObservationTurn(event), store.StatusChangeEventObservationTurn(current)
		if !exists || observedTurn > previousObservedTurn || (observedTurn == previousObservedTurn && event.ID > current.ID) {
			latest[event.OwnerID] = event
		}
	}
	restored := 0
	for _, event := range latest {
		evidence := parseJSONMap(event.EvidenceJSON)
		if stringFromMap(evidence, "projection_action") == "remove" {
			// The latest repair records absence. Its pending snapshot is an
			// exact undo value, not a narrative current value to resurrect.
			continue
		}
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
		if stringFromMap(evidence, "source_revision") == "" && payload["repair_pending_snapshot"] != true {
			if snapshot := narrativePendingSnapshot(payload); snapshot != nil {
				if saver, ok := st.(pendingThreadSaver); ok {
					threads, err := st.ListPendingThreads(ctx, sid, "all")
					if err != nil {
						return restored, err
					}
					snapshot.ID = 0
					for _, thread := range threads {
						if thread.ThreadKey == snapshot.ThreadKey {
							snapshot.ID = thread.ID
							break
						}
					}
					snapshot.UpdatedAt = time.Now().UTC()
					if err := saver.SavePendingThread(ctx, snapshot); err != nil {
						return restored, err
					}
				}
			}
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
		if (view.Subject == "" && normalizeNarrativeLifecycleKey(stringFromMap(payload, "lifecycle_key")) == "") || view.Slot == "" || view.Current == "" {
			continue
		}
		out = append(out, view)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return store.StatusCurrentObservationTurn(out[i].Value) > store.StatusCurrentObservationTurn(out[j].Value)
	})
	return out
}

func filterNarrativeCurrentStateViews(values []store.StatusCurrentValue, rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState) (facts, perceptions []narrativeCurrentStateView, dropped int) {
	context := buildPrepareTurnRecollectionContext(rawUserInput, nil, activeStates, nil, nil)
	relevanceText := context.relevanceText()
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
			// Legacy generic narrative-state rows do not have the stable
			// holder/speaker/listener boundary required by perspective_memory.v1.
			// Keep them persisted for audit/reprocessing, but never let them
			// bypass the precise per-holder prepare-turn path.
			dropped++
			continue
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
	case "defer", "abandon", "complete", "supersede", "reopen", "resume", "progress", "partial", "cancel", "modify", "pause", "missed", "refused", "impossible":
		return normalizeNarrativeLifecycleKey(extractionStringFromAny(payload["lifecycle_key"])) != "" ||
			(normalizeNarrativeSubjectType(extractionStringFromAny(payload["subject_type"])) == "entity" &&
				normalizeNarrativeStateSlot(extractionStringFromAny(payload["state_slot"])) == "goal_status" &&
				normalizeNarrativeClaimScope(extractionStringFromAny(payload["claim_scope"])) == "objective")
	default:
		return false
	}
}

func buildNarrativeContinuityCorrection(values []store.StatusCurrentValue, rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, selection prepareTurnMemoryLaneSelection, limit int) (string, map[string]any) {
	limit = prepareTurnRecallLimit(limit)
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, nil, activeStates, nil, nil)
	recentText := ctx.relevanceText()
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
			lines = append(lines, fmt.Sprintf("- Perspective only: %s currently believes about %s / %s: %s (%s).", owner, view.Subject, view.Slot, compactPrepareTurnLine(view.Current, 0), reason))
		} else {
			lines = append(lines, fmt.Sprintf("- Current continuity: %s / %s: %s (%s).", view.Subject, view.Slot, compactPrepareTurnLine(view.Current, 0), reason))
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

func narrativeCurrentStateSupersedesOpenArtifact(values []store.StatusCurrentValue, sourceTurn int, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, view := range narrativeCurrentStateViews(values) {
		if view.Scope != "objective" {
			continue
		}
		transition := normalizeNarrativeTransition(extractionStringFromAny(view.Payload["transition"]))
		switch transition {
		case "defer", "abandon", "complete", "supersede", "resolve", "clear", "cancel", "pause":
		default:
			continue
		}
		subjectType := normalizeNarrativeSubjectType(extractionStringFromAny(view.Payload["subject_type"]))
		lifecycleKey := normalizeNarrativeLifecycleKey(extractionStringFromAny(view.Payload["lifecycle_key"]))
		evidencePayload := parseJSONMap(view.Value.EvidenceJSON)
		explicitRepair := stringFromMap(evidencePayload, "source_contract") == store.StateRepairContract
		if sourceTurn <= 0 && !explicitRepair {
			continue
		}
		legacyLifecycle := stringFromMap(evidencePayload, "source") == "critic.resolved_threads" || stringFromMap(evidencePayload, "source") == "critic.pending_threads"
		confirmedProjection := legacyLifecycle || explicitRepair
		if (!confirmedProjection && extractionFloatFromAny(view.Payload["confidence"], 0) < narrativeStateMinimumConfidence) ||
			(lifecycleKey == "" && (subjectType != "entity" || view.Slot != "goal_status")) {
			continue
		}
		claim := narrativeStateClaim{
			Subject:          view.Subject,
			SubjectType:      subjectType,
			StateSlot:        view.Slot,
			LifecycleKey:     lifecycleKey,
			ClaimScope:       view.Scope,
			PerspectiveOwner: view.Perspective,
		}
		if view.Value.OwnerScope != narrativeStateOwnerScope(claim) ||
			view.Value.OwnerID != narrativeStateOwnerID(claim) {
			continue
		}
		if (!confirmedProjection && strings.TrimSpace(extractionStringFromAny(evidencePayload["evidence_excerpt"])) == "") ||
			intFromAny(evidencePayload["source_turn"], 0) != view.Value.SourceTurn {
			continue
		}
		artifact := parseJSONMap(text)
		artifactLifecycleKey := normalizeNarrativeLifecycleKey(extractionStringFromAny(artifact["lifecycle_key"]))
		if claim.LifecycleKey != "" && artifactLifecycleKey == claim.LifecycleKey {
			return true
		}
		if store.StatusCurrentObservationTurn(view.Value) > sourceTurn && normalizeNarrativeStateSlot(extractionStringFromAny(artifact["state_slot"])) == "goal_status" &&
			normalizeArtifactDedupeText(extractionStringFromAny(artifact["subject"])) == normalizeArtifactDedupeText(view.Subject) {
			return true
		}
	}
	return false
}

func prepareTurnOpenGoalArtifact(raw any) string {
	payload := mapFromAny(raw)
	if len(payload) == 0 {
		return ""
	}
	subject := strings.TrimSpace(stringFromMap(payload, "subject"))
	title := strings.TrimSpace(stringFromMap(payload, "title"))
	if subject != "" && title != "" &&
		normalizeArtifactDedupeText(subject) != normalizeArtifactDedupeText(title) {
		return ""
	}
	identity := strings.TrimSpace(extractionFirstNonEmpty(subject, title))
	stateSlot := normalizeNarrativeStateSlot(stringFromMap(payload, "state_slot"))
	lifecycleKey := normalizeNarrativeLifecycleKey(stringFromMap(payload, "lifecycle_key"))
	if lifecycleKey != "" {
		return mustCompactJSON(map[string]any{
			"lifecycle_key": lifecycleKey,
			"subject":       identity,
			"state_slot":    stateSlot,
		})
	}
	if identity == "" || stateSlot != "goal_status" {
		return ""
	}
	return mustCompactJSON(map[string]any{
		"subject":    identity,
		"state_slot": stateSlot,
	})
}

func prepareTurnOpenGoalArtifactForTitle(raw any, title string) string {
	artifact := prepareTurnOpenGoalArtifact(raw)
	if artifact == "" {
		return ""
	}
	payload := parseJSONMap(artifact)
	if normalizeArtifactDedupeText(extractionStringFromAny(payload["subject"])) != normalizeArtifactDedupeText(title) {
		return ""
	}
	return artifact
}

func filterPrepareTurnOpenGoalContent(raw string, sourceTurn int, values []store.StatusCurrentValue) (string, bool) {
	payload := parseJSONMap(raw)
	if len(payload) == 0 {
		return raw, false
	}
	changed := false
	var pruneOpened func(map[string]any)
	pruneOpened = func(node map[string]any) {
		for key, value := range node {
			child := mapFromAny(value)
			if key == "unresolved_threads" && len(child) > 0 {
				opened := sliceFromAny(child["opened"])
				if len(opened) > 0 {
					kept := make([]any, 0, len(opened))
					for _, item := range opened {
						artifact := prepareTurnOpenGoalArtifact(item)
						if artifact != "" && narrativeCurrentStateSupersedesOpenArtifact(values, sourceTurn, artifact) {
							changed = true
							continue
						}
						kept = append(kept, item)
					}
					if len(kept) == 0 {
						delete(child, "opened")
					} else {
						child["opened"] = kept
					}
				}
				if len(child) == 0 {
					delete(node, key)
					continue
				}
			}
			if len(child) > 0 {
				pruneOpened(child)
				if len(child) == 0 {
					delete(node, key)
				}
			}
		}
	}
	pruneOpened(payload)
	if !changed {
		return raw, false
	}
	if !hasMeaningfulPayload(payload) {
		return "", true
	}
	return mustCompactJSON(payload), true
}

func prepareTurnOpenLifecycleStatus(raw string, allowEmpty bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "active", "open", "ongoing", "pending":
		return true
	case "":
		return allowEmpty
	default:
		return false
	}
}

func filterPrepareTurnSupersededOpenGoals(
	values []store.StatusCurrentValue,
	storylines []store.Storyline,
	pendingThreads []store.PendingThread,
	activeStates []store.ActiveState,
	canonicalLayers []store.CanonicalStateLayer,
) ([]store.Storyline, []store.PendingThread, []store.ActiveState, []store.CanonicalStateLayer, map[string]any) {
	trace := map[string]any{
		"policy_version":             "prepare_turn.superseded_open_goals.v1",
		"storylines_dropped":         0,
		"pending_threads_dropped":    0,
		"active_states_dropped":      0,
		"canonical_layers_dropped":   0,
		"user_owned_items_preserved": 0,
	}

	filteredStorylines := make([]store.Storyline, 0, len(storylines))
	for _, item := range storylines {
		if item.Pinned || item.UserCorrected {
			trace["user_owned_items_preserved"] = intFromAny(trace["user_owned_items_preserved"], 0) + 1
			filteredStorylines = append(filteredStorylines, item)
			continue
		}
		sourceTurn := item.LastEvidenceTurn
		if sourceTurn <= 0 {
			sourceTurn = item.LastTurn
		}
		if sourceTurn <= 0 {
			sourceTurn = item.FirstTurn
		}
		artifact := prepareTurnOpenGoalArtifactForTitle(parseJSONMap(item.OngoingTensionsJSON), item.Name)
		if prepareTurnOpenLifecycleStatus(item.Status, false) &&
			artifact != "" &&
			narrativeCurrentStateSupersedesOpenArtifact(values, sourceTurn, artifact) {
			trace["storylines_dropped"] = intFromAny(trace["storylines_dropped"], 0) + 1
			continue
		}
		filteredStorylines = append(filteredStorylines, item)
	}

	filteredPendingThreads := make([]store.PendingThread, 0, len(pendingThreads))
	for _, item := range pendingThreads {
		if item.Pinned || item.UserCorrected {
			trace["user_owned_items_preserved"] = intFromAny(trace["user_owned_items_preserved"], 0) + 1
			filteredPendingThreads = append(filteredPendingThreads, item)
			continue
		}
		sourceTurn := item.SourceTurn
		if sourceTurn <= 0 {
			sourceTurn = item.CreatedTurn
		}
		metadata := parseJSONMap(item.HookMetadataJSON)
		if len(metadata) == 0 {
			metadata = parseJSONMap(item.DetailsJSON)
		}
		artifact := prepareTurnOpenGoalArtifactForTitle(metadata, item.Title)
		if prepareTurnOpenLifecycleStatus(item.Status, true) &&
			artifact != "" &&
			narrativeCurrentStateSupersedesOpenArtifact(values, sourceTurn, artifact) {
			trace["pending_threads_dropped"] = intFromAny(trace["pending_threads_dropped"], 0) + 1
			continue
		}
		filteredPendingThreads = append(filteredPendingThreads, item)
	}

	filteredActiveStates := make([]store.ActiveState, 0, len(activeStates))
	for _, item := range activeStates {
		if item.StateType == "unresolved_threads" &&
			narrativeCurrentStateSupersedesOpenArtifact(values, item.TurnIndex, prepareTurnOpenGoalArtifact(parseJSONMap(item.Content))) {
			trace["active_states_dropped"] = intFromAny(trace["active_states_dropped"], 0) + 1
			continue
		}
		if content, changed := filterPrepareTurnOpenGoalContent(item.Content, item.TurnIndex, values); changed {
			item.Content = content
			if strings.TrimSpace(item.Content) == "" {
				trace["active_states_dropped"] = intFromAny(trace["active_states_dropped"], 0) + 1
				continue
			}
		}
		filteredActiveStates = append(filteredActiveStates, item)
	}

	filteredCanonicalLayers := make([]store.CanonicalStateLayer, 0, len(canonicalLayers))
	for _, item := range canonicalLayers {
		sourceTurn := item.SourceTurn
		if sourceTurn <= 0 {
			sourceTurn = item.TurnIndex
		}
		if item.LayerType == "unresolved_threads" &&
			narrativeCurrentStateSupersedesOpenArtifact(values, sourceTurn, prepareTurnOpenGoalArtifact(parseJSONMap(item.Content))) {
			trace["canonical_layers_dropped"] = intFromAny(trace["canonical_layers_dropped"], 0) + 1
			continue
		}
		if content, changed := filterPrepareTurnOpenGoalContent(item.Content, sourceTurn, values); changed {
			item.Content = content
			if strings.TrimSpace(item.Content) == "" {
				trace["canonical_layers_dropped"] = intFromAny(trace["canonical_layers_dropped"], 0) + 1
				continue
			}
		}
		filteredCanonicalLayers = append(filteredCanonicalLayers, item)
	}

	return filteredStorylines, filteredPendingThreads, filteredActiveStates, filteredCanonicalLayers, trace
}
