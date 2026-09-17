package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type adminSessionNormalizeRequest struct {
	ChatSessionID  string                          `json:"chat_session_id"`
	MaxItems       int                             `json:"max_items"`
	BatchSize      int                             `json:"batch_size"`
	TurnIndices    []int                           `json:"turn_indices"`
	ClientMeta     map[string]any                  `json:"client_meta"`
	RepairEntries  []dto.ChatLogRepairEntryRequest `json:"repair_entries"`
	Entries        []dto.ChatLogRepairEntryRequest `json:"entries"`
	DryRun         bool                            `json:"dry_run"`
	SkipRepair     bool                            `json:"skip_repair"`
	SkipRescan     bool                            `json:"skip_rescan"`
	SkipReindex    bool                            `json:"skip_reindex"`
	ForceReindex   *bool                           `json:"force_reindex"`
	ResumeExisting *bool                           `json:"resume_existing"`
}

func (s *Server) handleAdminSessionNormalize(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/session-normalize")
		return
	}
	var req adminSessionNormalizeRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if s.AdminJobs == nil {
		s.AdminJobs = newAdminJobManager()
	}

	entries := adminSessionNormalizeRepairEntries(req)
	jobRequest := adminSessionNormalizeJobRequest(sid, req, entries)
	job := s.AdminJobs.start("session_normalize", sid, jobRequest, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
		return s.runAdminSessionNormalize(ctx, sid, req, progress)
	})
	job["status"] = "accepted"
	job["job_status"] = "queued"
	job["poll_route"] = "/admin/jobs/" + fmt.Sprint(job["job_id"])
	job["note"] = "session normalize is running in the background; poll the job route for progress"
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) runAdminSessionNormalize(ctx context.Context, sid string, req adminSessionNormalizeRequest, progress adminJobProgressFunc) (map[string]any, error) {
	if progress != nil {
		progress(map[string]any{
			"status":           "running",
			"stage":            "inspect_before",
			"progress_percent": 3,
			"timeout_policy":   "background_job_detached_from_http_request",
			"destructive":      false,
		})
	}

	entries := adminSessionNormalizeRepairEntries(req)
	before, warnings := s.adminSessionNormalizeSnapshot(ctx, sid)
	plan := adminSessionNormalizePlan(req, entries, before)
	initialReviewNeededTurns := adminSessionNormalizeConflictTurns(before)

	var repairResult map[string]any
	if !req.SkipRepair && len(entries) > 0 {
		if progress != nil {
			progress(map[string]any{
				"status":                "running",
				"stage":                 "raw_repair_replay",
				"repair_entry_count":    len(entries),
				"candidate_count":       len(entries),
				"processed":             0,
				"succeeded":             0,
				"failed_count":          0,
				"skipped_count":         0,
				"review_needed_turns":   initialReviewNeededTurns,
				"progress_percent":      8,
				"non_destructive_scope": "insert_missing_raw_roles_only",
			})
		}
		dryRun := req.DryRun
		repairReq := dto.ChatLogRepairReplayRequest{
			ChatSessionID: &sid,
			DryRun:        &dryRun,
			Entries:       entries,
		}
		result, err := s.runChatLogRepairReplayWithProgress(
			ctx,
			sid,
			repairReq,
			adminSessionNormalizeProgressAdapter(progress, "raw_repair_replay", 8, 10),
		)
		if err != nil {
			return nil, err
		}
		repairResult = result
	} else {
		repairResult = map[string]any{
			"status":          "skipped",
			"chat_session_id": sid,
			"dry_run":         req.DryRun,
			"entries_count":   len(entries),
			"reason":          adminSessionNormalizeSkipReason(req.SkipRepair, len(entries), "no_repair_entries"),
		}
	}
	afterRepair, afterRepairWarnings := s.adminSessionNormalizeSnapshot(ctx, sid)
	warnings = append(warnings, afterRepairWarnings...)
	reviewNeededTurns := append(
		adminSessionNormalizeConflictTurns(afterRepair),
		intSliceFromAny(repairResult["conflict_turns"])...,
	)
	reviewNeededTurns = append(reviewNeededTurns, intSliceFromAny(repairResult["failed_turn_indices"])...)
	reviewNeededTurns = uniqueSortedInts(reviewNeededTurns)
	if len(reviewNeededTurns) > 0 {
		warnings = append(warnings, "raw_mismatch_or_partial_requires_review")
	}
	requestedRescanTurns := uniqueSortedNonNegativeInts(req.TurnIndices)
	rescanTurns := append([]int{}, requestedRescanTurns...)
	if len(reviewNeededTurns) > 0 && len(rescanTurns) > 0 {
		reviewSet := map[int]bool{}
		for _, turn := range reviewNeededTurns {
			reviewSet[turn] = true
		}
		filtered := rescanTurns[:0]
		for _, turn := range rescanTurns {
			if !reviewSet[turn] {
				filtered = append(filtered, turn)
			}
		}
		rescanTurns = filtered
	}

	var rescanResult map[string]any
	if !req.SkipRescan && len(requestedRescanTurns) > 0 && len(rescanTurns) == 0 {
		rescanResult = map[string]any{
			"status":              "skipped",
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     0,
			"succeeded":           0,
			"failed":              0,
			"skipped":             len(requestedRescanTurns),
			"review_needed_turns": reviewNeededTurns,
			"reason":              "all_requested_turns_require_raw_review",
		}
	} else if !req.SkipRescan {
		meta := adminSessionNormalizeClientMeta(req.ClientMeta)
		rescanReq := adminRescanRequest{
			ChatSessionID:      sid,
			MaxItems:           req.MaxItems,
			TurnIndices:        rescanTurns,
			ClientMeta:         meta,
			DryRun:             req.DryRun,
			Background:         false,
			CanonicalRawReplay: true,
			SourceObservations: adminSessionNormalizeSourceObservations(entries),
		}
		res, err := s.runAdminRescanWithProgress(ctx, sid, rescanReq, adminSessionNormalizeProgressAdapter(progress, "critic_rescan_backfill", 18, 52))
		if err != nil {
			return nil, err
		}
		rescanResult = res
	} else {
		rescanResult = map[string]any{
			"status":          "skipped",
			"chat_session_id": sid,
			"dry_run":         req.DryRun,
			"reason":          "skip_rescan_requested",
		}
	}

	if progress != nil {
		progress(map[string]any{
			"status":           "running",
			"stage":            "character_identity_repair",
			"progress_percent": 71,
			"destructive":      false,
		})
	}
	identityRepairResult := s.repairMissingCharacterIdentities(ctx, sid, req.DryRun)
	if intFromAny(identityRepairResult["failed"], 0) > 0 {
		warnings = append(warnings, "character_identity_repair_partial")
	}

	var reindexResult map[string]any
	reindexDeferredReasons := adminSessionNormalizeReindexDeferredReasons(rescanResult)
	if !req.SkipReindex && len(reindexDeferredReasons) == 0 {
		reindexReq := map[string]any{
			"chat_session_id": sid,
			"max_items":       req.MaxItems,
			"batch_size":      req.BatchSize,
			"force":           adminSessionNormalizeForceReindex(req),
			"dry_run":         req.DryRun,
			"background":      true,
			"client_meta":     adminSessionNormalizeClientMeta(req.ClientMeta),
		}
		res, err := s.runAdminReindexJob(ctx, sid, reindexReq, adminSessionNormalizeProgressAdapter(progress, "vector_reindex", 72, 23))
		if err != nil {
			return nil, err
		}
		reindexResult = res
	} else if !req.SkipReindex {
		reindexResult = map[string]any{
			"status":          "deferred",
			"chat_session_id": sid,
			"dry_run":         req.DryRun,
			"reason":          "derived_reprocessing_incomplete",
			"deferred_by":     reindexDeferredReasons,
		}
		warnings = append(warnings, "vector_reindex_deferred_until_derived_reprocessing_completes")
	} else {
		reindexResult = map[string]any{
			"status":          "skipped",
			"chat_session_id": sid,
			"dry_run":         req.DryRun,
			"reason":          "skip_reindex_requested",
		}
	}

	after, afterWarnings := s.adminSessionNormalizeSnapshot(ctx, sid)
	warnings = append(warnings, afterWarnings...)
	status := adminSessionNormalizeStatus(
		repairResult,
		rescanResult,
		identityRepairResult,
		reindexResult,
		warnings,
	)
	pendingCount := maxInt(
		intFromAny(rescanResult["deferred"], 0),
		intFromAny(rescanResult["queued"], 0),
	)
	pendingTurns := []int{}
	for _, item := range adminSessionNormalizeMapItems(rescanResult["deferred_turns"]) {
		if turn := intFromAny(item["turn_index"], 0); turn > 0 {
			pendingTurns = append(pendingTurns, turn)
		}
	}
	pendingTurns = uniqueSortedNonNegativeInts(pendingTurns)
	pendingTurnsAvailable := pendingCount == 0 || len(pendingTurns) == pendingCount
	failedCount := intFromAny(rescanResult["failed"], 0)
	failedTurns := adminSessionNormalizeMapItems(rescanResult["failed_turns"])
	completionReason := ""
	if pendingCount > 0 {
		completionReason = "derived_reprocessing_pending"
	} else if failedCount > 0 {
		completionReason = "derived_reprocessing_failed"
	}
	result := map[string]any{
		"status":                    status,
		"contract_version":          "session-normalize.v1",
		"source":                    s.storeWriteSource(),
		"chat_session_id":           sid,
		"dry_run":                   req.DryRun,
		"destructive":               false,
		"rollback_attempted":        false,
		"delete_attempted":          false,
		"plan":                      plan,
		"counts_before":             before,
		"counts_after":              after,
		"repair_replay":             repairResult,
		"rescan":                    rescanResult,
		"character_identity_repair": identityRepairResult,
		"reindex":                   reindexResult,
		"review_needed_turns":       reviewNeededTurns,
		"pending_count":             pendingCount,
		"pending_turns_available":   pendingTurnsAvailable,
		"failed_count":              failedCount,
		"failed_turns":              failedTurns,
		"completion_reason":         completionReason,
		"warnings":                  uniqueStrings(warnings),
		"generated_at":              time.Now().UTC(),
		"note":                      "session normalize repaired canonical raw logs, rebuilt derived artifacts from canonical backend state, and reported vector reindex separately without rollback or destructive trim",
	}
	if pendingTurnsAvailable {
		result["pending_turns"] = pendingTurns
	} else {
		result["pending_turns_unavailable_reason"] = "durable_reprocessing_result_has_aggregate_count_only"
	}
	for _, key := range []string{"retry_attempt", "retry_max_attempts", "next_retry_at", "retry_after_seconds"} {
		if value, ok := rescanResult[key]; ok {
			result[key] = value
		}
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "session_normalize",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session Normalize finished",
		DetailsJSON: mustCompactJSON(map[string]any{
			"status":              status,
			"dry_run":             req.DryRun,
			"destructive":         false,
			"repair_entry_count":  len(entries),
			"review_needed_turns": reviewNeededTurns,
			"pending_count":       pendingCount,
			"failed_count":        failedCount,
			"character_identity_repair": map[string]any{
				"status":             identityRepairResult["status"],
				"candidates":         identityRepairResult["candidates"],
				"created_identities": identityRepairResult["created_identities"],
				"created_surfaces":   identityRepairResult["created_surfaces"],
				"skipped":            identityRepairResult["skipped"],
				"failed":             identityRepairResult["failed"],
			},
			"plan":     plan,
			"warnings": uniqueStrings(warnings),
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: time.Now().UTC(),
	})
	if progress != nil {
		finalStage := "completed"
		finalProgressStatus := "completed"
		finalProgressPercent := 100
		if status != "ok" {
			finalStage = status
		}
		switch status {
		case "partial_deferred":
			finalProgressStatus = "deferred"
			finalProgressPercent = 99
		case "partial_error":
			finalProgressStatus = "partial_error"
		case "failed", "blocked":
			finalProgressStatus = status
		}
		finalProgress := map[string]any{
			"status":                  finalProgressStatus,
			"stage":                   finalStage,
			"outcome_status":          status,
			"progress_percent":        finalProgressPercent,
			"review_needed_turns":     reviewNeededTurns,
			"pending_count":           pendingCount,
			"pending_turns_available": pendingTurnsAvailable,
			"failed_count":            failedCount,
			"failed_turns":            failedTurns,
			"reason":                  completionReason,
			"counts_after":            after,
			"warnings":                uniqueStrings(warnings),
		}
		if pendingTurnsAvailable {
			finalProgress["pending_turns"] = pendingTurns
		} else {
			finalProgress["pending_turns_unavailable_reason"] = "durable_reprocessing_result_has_aggregate_count_only"
		}
		for _, key := range []string{"retry_attempt", "retry_max_attempts", "next_retry_at", "retry_after_seconds"} {
			if value, ok := rescanResult[key]; ok {
				finalProgress[key] = value
			}
		}
		progress(finalProgress)
	}
	return result, nil
}

func (s *Server) repairMissingCharacterIdentities(ctx context.Context, sid string, dryRun bool) map[string]any {
	result := map[string]any{
		"status": "ok", "chat_session_id": sid, "dry_run": dryRun,
		"candidates": 0, "created_identities": 0, "created_surfaces": 0,
		"would_create": 0, "skipped": 0, "failed": 0, "skipped_items": []any{}, "errors": []any{},
		"character_candidates": 0, "character_created_identities": 0, "character_created_surfaces": 0,
		"item_candidates": 0, "item_created_identities": 0, "item_created_surfaces": 0,
	}
	writer, writerOK := s.Store.(store.EntityIdentityWriter)
	catalogReader, catalogOK := s.Store.(store.EntityIdentityCatalogReader)
	history, historyOK := s.Store.(store.SourceRevisionHistoryLister)
	if !writerOK || !catalogOK || !historyOK {
		result["status"] = "skipped"
		result["reason"] = "entity_identity_repair_not_supported"
		return result
	}
	if availability, ok := s.Store.(store.EntityIdentityWriteAvailability); ok && !availability.EntityIdentityWritesEnabled() {
		result["status"] = "skipped"
		result["reason"] = "entity_identity_writes_disabled"
		return result
	}
	states, err := s.Store.ListCharacterStates(ctx, sid)
	if err != nil {
		result["status"] = "partial_error"
		result["failed"] = 1
		result["errors"] = []any{map[string]any{"stage": "list_character_states", "detail": err.Error()}}
		return result
	}
	identities, err := catalogReader.ListActiveEntityIdentities(ctx, sid)
	if err != nil {
		result["status"] = "partial_error"
		result["failed"] = 1
		result["errors"] = []any{map[string]any{"stage": "list_entity_identities", "detail": err.Error()}}
		return result
	}
	surfaces, err := catalogReader.ListActiveEntityIdentitySurfaces(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		result["status"] = "partial_error"
		result["failed"] = 1
		result["errors"] = []any{map[string]any{"stage": "list_entity_surfaces", "detail": err.Error()}}
		return result
	}
	sources, err := history.ListSourceRevisions(ctx, sid, 0, 0)
	if err != nil {
		result["status"] = "partial_error"
		result["failed"] = 1
		result["errors"] = []any{map[string]any{"stage": "list_source_revisions", "detail": err.Error()}}
		return result
	}

	activeCharacterIdentities := map[string]store.EntityIdentity{}
	identityIDsByName := map[string][]string{}
	for _, identity := range identities {
		id := strings.TrimSpace(identity.StableEntityID)
		if identity.ChatSessionID != sid || identity.EntityKind != "character" || id == "" {
			continue
		}
		activeCharacterIdentities[id] = identity
		key := comparableEntityKey(identity.CanonicalLabel)
		if key != "" {
			identityIDsByName[key] = appendUniqueString(identityIDsByName[key], id)
		}
	}
	surfaceExistsByName := map[string]bool{}
	for _, surface := range surfaces {
		if surface.ChatSessionID != sid {
			continue
		}
		if _, ok := activeCharacterIdentities[strings.TrimSpace(surface.StableEntityID)]; !ok {
			continue
		}
		if key := comparableEntityKey(surface.SurfaceText); key != "" {
			surfaceExistsByName[key] = true
		}
	}
	activeSourcesByTurn := map[int][]store.MemorySourceRevision{}
	for _, source := range sources {
		if source.ChatSessionID == sid && strings.EqualFold(strings.TrimSpace(source.LifecycleState), "active") {
			activeSourcesByTurn[source.TurnIndex] = append(activeSourcesByTurn[source.TurnIndex], source)
		}
	}
	statesByName := map[string][]store.CharacterState{}
	nameOrder := []string{}
	for _, state := range states {
		name := strings.TrimSpace(state.CharacterName)
		key := comparableEntityKey(name)
		if state.ChatSessionID != sid || name == "" || key == "" || surfaceExistsByName[key] {
			continue
		}
		if _, seen := statesByName[key]; !seen {
			nameOrder = append(nameOrder, key)
		}
		statesByName[key] = append(statesByName[key], state)
	}
	sort.Strings(nameOrder)
	errorsOut := []any{}
	skippedOut := []any{}
	for _, key := range nameOrder {
		candidates := statesByName[key]
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].TurnIndex != candidates[j].TurnIndex {
				return candidates[i].TurnIndex < candidates[j].TurnIndex
			}
			return candidates[i].ID < candidates[j].ID
		})
		result["candidates"] = intFromAny(result["candidates"], 0) + 1
		result["character_candidates"] = intFromAny(result["character_candidates"], 0) + 1
		name := strings.TrimSpace(candidates[0].CharacterName)
		ids := uniqueNonEmptyStrings(identityIDsByName[key])
		if len(ids) > 1 {
			result["skipped"] = intFromAny(result["skipped"], 0) + 1
			skippedOut = append(skippedOut, map[string]any{"character_name": name, "reason": "multiple_existing_character_identities"})
			continue
		}
		var state store.CharacterState
		var source store.MemorySourceRevision
		foundSource := false
		for _, candidate := range candidates {
			turnSources := activeSourcesByTurn[candidate.TurnIndex]
			if len(turnSources) == 1 {
				state = candidate
				source = turnSources[0]
				foundSource = true
				break
			}
		}
		if !foundSource {
			result["skipped"] = intFromAny(result["skipped"], 0) + 1
			skippedOut = append(skippedOut, map[string]any{"character_name": name, "reason": "unique_active_source_revision_not_found"})
			continue
		}
		result["would_create"] = intFromAny(result["would_create"], 0) + 1
		if dryRun {
			continue
		}
		now := time.Now().UTC()
		idempotencyKey := entityIdentityIdempotencyKey("session_normalize_character", source.SourceRevision, key)
		stableID := ""
		if len(ids) == 1 {
			stableID = ids[0]
		} else {
			stableID = entityIdentityStableID("entity", sid, idempotencyKey)
			identity := store.EntityIdentity{
				StableEntityID: stableID, ChatSessionID: sid, IdentityNamespace: "session_npc", EntityKind: "character",
				CanonicalLabel: name, LifecycleState: "active", ReviewState: store.EntityIdentityReviewStateSourceObserved,
				PresenceAuthority: "observed", OccurrenceAuthority: "derived_character_state",
				SourceContract: completeTurnSourceAcceptanceContract, SourceRevision: source.SourceRevision,
				SourceLogicalTurnID: source.LogicalTurnID, SourceMessageID: source.SourceMessageID,
				SourceGenerationID: source.SourceGenerationID, SourceContentHash: source.CombinedContentHash,
				SourceTurn: state.TurnIndex, IdempotencyKey: idempotencyKey, MappingRevision: 1,
				FirstSeenTurn: state.TurnIndex, LastSeenTurn: state.TurnIndex, CreatedAt: now, UpdatedAt: now,
			}
			if err := writer.SaveEntityIdentity(ctx, &identity); err != nil {
				result["failed"] = intFromAny(result["failed"], 0) + 1
				errorsOut = append(errorsOut, map[string]any{"character_name": name, "stage": "save_identity", "detail": err.Error()})
				continue
			}
			result["created_identities"] = intFromAny(result["created_identities"], 0) + 1
			result["character_created_identities"] = intFromAny(result["character_created_identities"], 0) + 1
		}
		surfaceKey := entityIdentityIdempotencyKey("session_normalize_character_surface", stableID, key, source.SourceRevision)
		surface := store.EntityIdentitySurface{
			SurfaceID: entityIdentityStableID("surface", sid, surfaceKey), StableEntityID: stableID,
			ChatSessionID: sid, IdentityNamespace: "session_npc", SurfaceKind: "display_name",
			SurfaceText: name, NormalizedSurface: key, Scope: store.EntityIdentitySurfaceScopeCurrent,
			ValidFromTurn: state.TurnIndex, SourceContract: completeTurnSourceAcceptanceContract, SourceRevision: source.SourceRevision,
			SourceTurn: state.TurnIndex, SourceSpanStart: -1, SourceSpanEnd: -1,
			ReviewState: store.EntityIdentityReviewStateSourceObserved, IdempotencyKey: surfaceKey,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := writer.SaveEntityIdentitySurface(ctx, &surface); err != nil {
			result["failed"] = intFromAny(result["failed"], 0) + 1
			errorsOut = append(errorsOut, map[string]any{"character_name": name, "stage": "save_surface", "detail": err.Error()})
			continue
		}
		result["created_surfaces"] = intFromAny(result["created_surfaces"], 0) + 1
		result["character_created_surfaces"] = intFromAny(result["character_created_surfaces"], 0) + 1
	}

	activeItemIdentities := map[string]store.EntityIdentity{}
	itemIdentityIDsByName := map[string][]string{}
	for _, identity := range identities {
		id := strings.TrimSpace(identity.StableEntityID)
		if identity.ChatSessionID != sid || identity.EntityKind != "item" || id == "" {
			continue
		}
		activeItemIdentities[id] = identity
		if key := comparableEntityKey(identity.CanonicalLabel); key != "" {
			itemIdentityIDsByName[key] = appendUniqueString(itemIdentityIDsByName[key], id)
		}
	}
	itemSurfaceExistsByName := map[string]bool{}
	for _, surface := range surfaces {
		if surface.ChatSessionID != sid {
			continue
		}
		if _, ok := activeItemIdentities[strings.TrimSpace(surface.StableEntityID)]; !ok {
			continue
		}
		if key := comparableEntityKey(surface.SurfaceText); key != "" {
			itemSurfaceExistsByName[key] = true
		}
	}
	itemTriples, itemListErr := s.Store.ListKGTriples(ctx, sid)
	if itemListErr != nil {
		result["failed"] = intFromAny(result["failed"], 0) + 1
		errorsOut = append(errorsOut, map[string]any{"stage": "list_item_kg_triples", "detail": itemListErr.Error()})
	} else {
		triplesByName := map[string][]store.KGTriple{}
		itemNameOrder := []string{}
		for _, triple := range itemTriples {
			name := strings.TrimSpace(triple.Object)
			key := comparableEntityKey(name)
			if triple.ChatSessionID != sid || !durableItemIdentityPredicate(triple.Predicate) || name == "" || key == "" || itemSurfaceExistsByName[key] {
				continue
			}
			if _, seen := triplesByName[key]; !seen {
				itemNameOrder = append(itemNameOrder, key)
			}
			triplesByName[key] = append(triplesByName[key], triple)
		}
		sort.Strings(itemNameOrder)
		for _, key := range itemNameOrder {
			candidates := triplesByName[key]
			sort.SliceStable(candidates, func(i, j int) bool {
				if candidates[i].SourceTurn != candidates[j].SourceTurn {
					return candidates[i].SourceTurn < candidates[j].SourceTurn
				}
				return candidates[i].ID < candidates[j].ID
			})
			result["candidates"] = intFromAny(result["candidates"], 0) + 1
			result["item_candidates"] = intFromAny(result["item_candidates"], 0) + 1
			name := strings.TrimSpace(candidates[0].Object)
			ids := uniqueNonEmptyStrings(itemIdentityIDsByName[key])
			if len(ids) > 1 {
				result["skipped"] = intFromAny(result["skipped"], 0) + 1
				skippedOut = append(skippedOut, map[string]any{"item_name": name, "reason": "multiple_existing_item_identities"})
				continue
			}
			var triple store.KGTriple
			var source store.MemorySourceRevision
			foundSource := false
			for _, candidate := range candidates {
				turnSources := activeSourcesByTurn[candidate.SourceTurn]
				if len(turnSources) == 1 {
					triple = candidate
					source = turnSources[0]
					foundSource = true
					break
				}
			}
			if !foundSource {
				result["skipped"] = intFromAny(result["skipped"], 0) + 1
				skippedOut = append(skippedOut, map[string]any{"item_name": name, "reason": "unique_active_source_revision_not_found"})
				continue
			}
			result["would_create"] = intFromAny(result["would_create"], 0) + 1
			if dryRun {
				continue
			}
			now := time.Now().UTC()
			idempotencyKey := entityIdentityIdempotencyKey("session_normalize_item", source.SourceRevision, key)
			stableID := ""
			if len(ids) == 1 {
				stableID = ids[0]
			} else {
				stableID = entityIdentityStableID("entity", sid, idempotencyKey)
				identity := store.EntityIdentity{
					StableEntityID: stableID, ChatSessionID: sid, IdentityNamespace: "session_item", EntityKind: "item",
					CanonicalLabel: name, LifecycleState: "active", ReviewState: store.EntityIdentityReviewStateSourceObserved,
					PresenceAuthority: "observed", OccurrenceAuthority: "derived_kg_item",
					SourceContract: completeTurnSourceAcceptanceContract, SourceRevision: source.SourceRevision,
					SourceLogicalTurnID: source.LogicalTurnID, SourceMessageID: source.SourceMessageID,
					SourceGenerationID: source.SourceGenerationID, SourceContentHash: source.CombinedContentHash,
					SourceTurn: triple.SourceTurn, IdempotencyKey: idempotencyKey, MappingRevision: 1,
					FirstSeenTurn: triple.SourceTurn, LastSeenTurn: triple.SourceTurn, CreatedAt: now, UpdatedAt: now,
				}
				if err := writer.SaveEntityIdentity(ctx, &identity); err != nil {
					result["failed"] = intFromAny(result["failed"], 0) + 1
					errorsOut = append(errorsOut, map[string]any{"item_name": name, "stage": "save_item_identity", "detail": err.Error()})
					continue
				}
				result["created_identities"] = intFromAny(result["created_identities"], 0) + 1
				result["item_created_identities"] = intFromAny(result["item_created_identities"], 0) + 1
			}
			surfaceKey := entityIdentityIdempotencyKey("session_normalize_item_surface", stableID, key, source.SourceRevision)
			surface := store.EntityIdentitySurface{
				SurfaceID: entityIdentityStableID("surface", sid, surfaceKey), StableEntityID: stableID,
				ChatSessionID: sid, IdentityNamespace: "session_item", SurfaceKind: "display_name",
				SurfaceText: name, NormalizedSurface: key, Scope: store.EntityIdentitySurfaceScopeCurrent,
				ValidFromTurn: triple.SourceTurn, SourceContract: completeTurnSourceAcceptanceContract, SourceRevision: source.SourceRevision,
				SourceTurn: triple.SourceTurn, SourceSpanStart: -1, SourceSpanEnd: -1,
				ReviewState: store.EntityIdentityReviewStateSourceObserved, IdempotencyKey: surfaceKey,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := writer.SaveEntityIdentitySurface(ctx, &surface); err != nil {
				result["failed"] = intFromAny(result["failed"], 0) + 1
				errorsOut = append(errorsOut, map[string]any{"item_name": name, "stage": "save_item_surface", "detail": err.Error()})
				continue
			}
			result["created_surfaces"] = intFromAny(result["created_surfaces"], 0) + 1
			result["item_created_surfaces"] = intFromAny(result["item_created_surfaces"], 0) + 1
		}
	}
	result["skipped_items"] = skippedOut
	result["errors"] = errorsOut
	if intFromAny(result["failed"], 0) > 0 {
		result["status"] = "partial_error"
	}
	return result
}

func adminSessionNormalizeProgressAdapter(progress adminJobProgressFunc, stage string, base, span int) adminJobProgressFunc {
	if progress == nil {
		return nil
	}
	return func(sub map[string]any) {
		out := cloneMapAny(sub)
		if out == nil {
			out = map[string]any{}
		}
		subPct := intFromAny(out["progress_percent"], 0)
		out["stage"] = stage
		out["subprogress"] = cloneMapAny(sub)
		out["progress_percent"] = base + (subPct*span)/100
		if out["progress_percent"].(int) > base+span {
			out["progress_percent"] = base + span
		}
		progress(out)
	}
}

func adminSessionNormalizeRepairEntries(req adminSessionNormalizeRequest) []dto.ChatLogRepairEntryRequest {
	combined := append([]dto.ChatLogRepairEntryRequest{}, req.RepairEntries...)
	combined = append(combined, req.Entries...)
	byTurn := map[int]dto.ChatLogRepairEntryRequest{}
	for _, item := range combined {
		if item.TurnIndex < 0 {
			continue
		}
		user := ""
		if item.UserContent != nil {
			user = strings.TrimSpace(*item.UserContent)
		}
		assistant := ""
		if item.AssistantContent != nil {
			assistant = strings.TrimSpace(*item.AssistantContent)
		}
		if user == "" && assistant == "" {
			continue
		}
		current := byTurn[item.TurnIndex]
		current.TurnIndex = item.TurnIndex
		if current.UserContent == nil || strings.TrimSpace(*current.UserContent) == "" {
			current.UserContent = item.UserContent
		}
		if current.AssistantContent == nil || strings.TrimSpace(*current.AssistantContent) == "" {
			current.AssistantContent = item.AssistantContent
		}
		if current.CreatedAt == nil || strings.TrimSpace(*current.CreatedAt) == "" {
			current.CreatedAt = item.CreatedAt
		}
		if current.Source == nil || strings.TrimSpace(*current.Source) == "" {
			current.Source = item.Source
		}
		if current.AssistantMessageID == nil || strings.TrimSpace(*current.AssistantMessageID) == "" {
			current.AssistantMessageID = item.AssistantMessageID
		}
		if current.AssistantGenerationID == nil || strings.TrimSpace(*current.AssistantGenerationID) == "" {
			current.AssistantGenerationID = item.AssistantGenerationID
		}
		if current.AssistantContentHash == nil || strings.TrimSpace(*current.AssistantContentHash) == "" {
			current.AssistantContentHash = item.AssistantContentHash
		}
		if current.InputMode == nil || strings.TrimSpace(*current.InputMode) == "" {
			current.InputMode = item.InputMode
		}
		if current.UserInputState == nil || strings.TrimSpace(*current.UserInputState) == "" {
			current.UserInputState = item.UserInputState
		}
		byTurn[item.TurnIndex] = current
	}
	turns := make([]int, 0, len(byTurn))
	for turn := range byTurn {
		turns = append(turns, turn)
	}
	sort.Ints(turns)
	out := make([]dto.ChatLogRepairEntryRequest, 0, len(turns))
	for _, turn := range turns {
		out = append(out, byTurn[turn])
	}
	return out
}

func adminSessionNormalizeSourceObservations(entries []dto.ChatLogRepairEntryRequest) map[int]adminRescanSourceObservation {
	out := map[int]adminRescanSourceObservation{}
	for _, entry := range entries {
		if entry.TurnIndex <= 0 {
			continue
		}
		out[entry.TurnIndex] = adminRescanSourceObservation{
			AssistantMessageID:    stringFromOptional(entry.AssistantMessageID),
			AssistantGenerationID: stringFromOptional(entry.AssistantGenerationID),
			AssistantContentHash:  stringFromOptional(entry.AssistantContentHash),
			InputMode:             stringFromOptional(entry.InputMode),
			UserInputState:        stringFromOptional(entry.UserInputState),
		}
	}
	return out
}

func stringFromOptional(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func adminSessionNormalizeClientMeta(raw map[string]any) map[string]any {
	meta := cloneMapAny(raw)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["source"] = "session_normalize"
	meta["background"] = true
	meta["resume_existing_artifacts"] = true
	meta["full_session_backfill"] = true
	meta["session_normalize_full_session_backfill"] = true
	return meta
}

func adminSessionNormalizeForceReindex(req adminSessionNormalizeRequest) bool {
	if adminSessionNormalizeResumeExisting(req) {
		return false
	}
	if req.ForceReindex == nil {
		return false
	}
	return *req.ForceReindex
}

func adminSessionNormalizeResumeExisting(req adminSessionNormalizeRequest) bool {
	if req.ResumeExisting == nil {
		return true
	}
	return *req.ResumeExisting
}

func adminSessionNormalizeJobRequest(sid string, req adminSessionNormalizeRequest, entries []dto.ChatLogRepairEntryRequest) map[string]any {
	return map[string]any{
		"chat_session_id":       sid,
		"max_items":             req.MaxItems,
		"batch_size":            req.BatchSize,
		"turn_indices":          uniqueSortedNonNegativeInts(req.TurnIndices),
		"repair_entry_count":    len(entries),
		"repair_turn_preview":   adminSessionNormalizeEntryTurns(entries, 20),
		"dry_run":               req.DryRun,
		"skip_repair":           req.SkipRepair,
		"skip_rescan":           req.SkipRescan,
		"skip_reindex":          req.SkipReindex,
		"force_reindex":         adminSessionNormalizeForceReindex(req),
		"resume_existing":       adminSessionNormalizeResumeExisting(req),
		"client_meta_keys":      adminSessionNormalizeMetaKeys(req.ClientMeta),
		"content_redacted":      true,
		"background":            true,
		"contract_version":      "session-normalize.v1",
		"destructive":           false,
		"rollback_delete_scope": "never",
	}
}

func adminSessionNormalizeEntryTurns(entries []dto.ChatLogRepairEntryRequest, limit int) []int {
	turns := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.TurnIndex > 0 {
			turns = append(turns, entry.TurnIndex)
		}
	}
	turns = uniqueSortedNonNegativeInts(turns)
	if limit > 0 && len(turns) > limit {
		return turns[:limit]
	}
	return turns
}

func adminSessionNormalizeMetaKeys(meta map[string]any) []string {
	keys := make([]string, 0, len(meta))
	for key := range meta {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "key") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *Server) adminSessionNormalizeSnapshot(ctx context.Context, sid string) (map[string]any, []string) {
	counts := map[string]any{
		"chat_log_rows":               0,
		"raw_turns":                   0,
		"raw_complete_turns":          0,
		"raw_partial_turns":           0,
		"raw_assistant_only_turns":    0,
		"raw_user_only_turns":         0,
		"raw_processable_turns":       0,
		"starter_turn_present":        false,
		"memories":                    0,
		"direct_evidence":             0,
		"kg_triples":                  0,
		"world_rules":                 0,
		"episode_summaries":           0,
		"chapter_summaries":           0,
		"arc_summaries":               0,
		"saga_digests":                0,
		"min_turn":                    0,
		"max_turn":                    0,
		"partial_turn_preview":        []int{},
		"assistant_only_turn_preview": []int{},
		"user_only_turn_preview":      []int{},
	}
	warnings := []string{}
	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_chat_logs_failed: "+err.Error())
	} else {
		roleByTurn := map[int]map[string]bool{}
		minTurn, maxTurn := 0, 0
		hasTurn := false
		for _, log := range logs {
			if log.ChatSessionID != sid || log.TurnIndex < 0 {
				continue
			}
			if !hasTurn || log.TurnIndex < minTurn {
				minTurn = log.TurnIndex
			}
			if !hasTurn || log.TurnIndex > maxTurn {
				maxTurn = log.TurnIndex
			}
			hasTurn = true
			role := strings.ToLower(strings.TrimSpace(log.Role))
			if role != "user" && role != "assistant" {
				continue
			}
			if roleByTurn[log.TurnIndex] == nil {
				roleByTurn[log.TurnIndex] = map[string]bool{}
			}
			roleByTurn[log.TurnIndex][role] = true
		}
		partialTurns := []int{}
		assistantOnlyTurns := []int{}
		userOnlyTurns := []int{}
		completeTurns := 0
		dialogueTurns := 0
		starterTurnPresent := false
		for turn, roles := range roleByTurn {
			if turn == 0 {
				starterTurnPresent = roles["assistant"]
				if !starterTurnPresent {
					partialTurns = append(partialTurns, turn)
				}
				continue
			}
			dialogueTurns++
			if roles["user"] && roles["assistant"] {
				completeTurns++
			} else {
				partialTurns = append(partialTurns, turn)
				if roles["assistant"] {
					assistantOnlyTurns = append(assistantOnlyTurns, turn)
				} else if roles["user"] {
					userOnlyTurns = append(userOnlyTurns, turn)
				}
			}
		}
		counts["chat_log_rows"] = len(logs)
		counts["raw_turns"] = dialogueTurns
		counts["raw_complete_turns"] = completeTurns
		counts["raw_partial_turns"] = len(partialTurns)
		counts["raw_assistant_only_turns"] = len(assistantOnlyTurns)
		counts["raw_user_only_turns"] = len(userOnlyTurns)
		counts["raw_processable_turns"] = completeTurns + len(assistantOnlyTurns)
		counts["starter_turn_present"] = starterTurnPresent
		counts["min_turn"] = minTurn
		counts["max_turn"] = maxTurn
		counts["partial_turn_preview"] = uniqueSortedNonNegativeInts(partialTurns)
		counts["assistant_only_turn_preview"] = uniqueSortedNonNegativeInts(assistantOnlyTurns)
		counts["user_only_turn_preview"] = uniqueSortedNonNegativeInts(userOnlyTurns)
	}
	if memories, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
		counts["memories"] = len(memories)
	} else if !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_memories_failed: "+err.Error())
	}
	if evidence, err := s.Store.ListEvidence(ctx, sid); err == nil {
		counts["direct_evidence"] = len(evidence)
	} else if !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_evidence_failed: "+err.Error())
	}
	if kg, err := s.Store.ListKGTriples(ctx, sid); err == nil {
		counts["kg_triples"] = len(kg)
	} else if !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_kg_failed: "+err.Error())
	}
	if rules, err := s.Store.ListWorldRules(ctx, sid); err == nil {
		counts["world_rules"] = len(rules)
	} else if !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_world_rules_failed: "+err.Error())
	}
	if episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0); err == nil {
		counts["episode_summaries"] = len(episodes)
	} else if !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_episode_summaries_failed: "+err.Error())
	}
	if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
		if chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", 0, 0, 0); err == nil {
			counts["chapter_summaries"] = len(chapters)
		} else if !errors.Is(err, store.ErrNotFound) {
			warnings = append(warnings, "list_chapter_summaries_failed: "+err.Error())
		}
	}
	if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
		if arcs, err := arcStore.ListArcSummaries(ctx, sid, "", 0); err == nil {
			counts["arc_summaries"] = len(arcs)
		} else if !errors.Is(err, store.ErrNotFound) {
			warnings = append(warnings, "list_arc_summaries_failed: "+err.Error())
		}
	}
	if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
		if sagas, err := sagaStore.ListSagaDigests(ctx, sid, 0); err == nil {
			counts["saga_digests"] = len(sagas)
		} else if !errors.Is(err, store.ErrNotFound) {
			warnings = append(warnings, "list_saga_digests_failed: "+err.Error())
		}
	}
	return counts, warnings
}

func adminSessionNormalizePlan(req adminSessionNormalizeRequest, entries []dto.ChatLogRepairEntryRequest, before map[string]any) map[string]any {
	rawTurns := intFromAny(before["raw_turns"], 0)
	memories := intFromAny(before["memories"], 0)
	worldRules := intFromAny(before["world_rules"], 0)
	episodes := intFromAny(before["episode_summaries"], 0)
	return map[string]any{
		"raw_import_candidates":          len(entries),
		"raw_partial_review_candidates":  intFromAny(before["raw_partial_turns"], 0),
		"critic_rescan_max_items":        req.MaxItems,
		"critic_rescan_turn_indices":     uniqueSortedNonNegativeInts(req.TurnIndices),
		"critic_rescan_force_backfills":  false,
		"critic_rescan_resume_existing":  true,
		"world_rule_review_needed":       rawTurns > 0 && worldRules == 0,
		"episode_review_needed":          rawTurns >= normalizedEpisodeInterval(0) && episodes == 0,
		"vector_reindex_max_items":       req.MaxItems,
		"vector_reindex_force":           adminSessionNormalizeForceReindex(req),
		"resume_existing":                adminSessionNormalizeResumeExisting(req),
		"safe_to_repeat":                 true,
		"destructive":                    false,
		"raw_turns_before":               rawTurns,
		"memory_rows_before":             memories,
		"advanced_tools_consolidated":    []string{"active_chat_dry_run", "repair_replay", "admin_rescan", "hierarchy_backfill", "admin_reindex"},
		"character_identity_repair":      "missing_exact_character_names_only",
		"item_identity_repair":           "missing_exact_kg_item_names_only",
		"visible_trim_delete_protection": "enabled",
	}
}

func adminSessionNormalizeConflictTurns(snapshot map[string]any) []int {
	return uniqueSortedInts(intSliceFromAny(snapshot["user_only_turn_preview"]))
}

func intSliceFromAny(v any) []int {
	switch items := v.(type) {
	case []int:
		return append([]int{}, items...)
	case []any:
		out := []int{}
		for _, item := range items {
			if value := intFromAny(item, 0); value > 0 {
				out = append(out, value)
			}
		}
		return out
	default:
		return []int{}
	}
}

func adminSessionNormalizeSkipReason(skipped bool, count int, fallback string) string {
	if skipped {
		return "skip_repair_requested"
	}
	if count == 0 {
		return fallback
	}
	return "not_run"
}

func adminSessionNormalizeStatus(repairResult, rescanResult, identityRepairResult, reindexResult map[string]any, warnings []string) string {
	deferred := false
	rescanPendingRetryOnly := false
	rescanFailedCount := intFromAny(rescanResult["failed"], 0)
	if rescanFailedCount > 0 && intFromAny(rescanResult["queued"], 0) > 0 {
		failedTurns := adminSessionNormalizeMapItems(rescanResult["failed_turns"])
		rescanPendingRetryOnly = len(failedTurns) == rescanFailedCount
		for _, item := range failedTurns {
			if !strings.EqualFold(strings.TrimSpace(stringFromAny(item["state"])), "retryable") {
				rescanPendingRetryOnly = false
				break
			}
		}
		for _, warning := range stringsFromAny(rescanResult["warnings"]) {
			if strings.Contains(strings.ToLower(warning), "reprocessing_enqueue_failed") {
				rescanPendingRetryOnly = false
				break
			}
		}
	}
	for index, result := range []map[string]any{repairResult, rescanResult, identityRepairResult, reindexResult} {
		status := strings.ToLower(strings.TrimSpace(stringFromMap(result, "status")))
		if index == 1 && rescanPendingRetryOnly {
			deferred = true
			continue
		}
		if status == "failed" || status == "error" {
			return "failed"
		}
		if status == "blocked" {
			return "blocked"
		}
		if status == "deferred" {
			deferred = true
		}
		if intFromAny(result["failed"], 0) > 0 || len(sliceFromAny(result["failed_turns"])) > 0 || len(sliceFromAny(result["errors"])) > 0 {
			return "partial_error"
		}
	}
	if deferred {
		return "partial_deferred"
	}
	if len(warnings) > 0 {
		return "partial_warning"
	}
	return "ok"
}

func adminSessionNormalizeMapItems(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return append([]map[string]any{}, items...)
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if mapped, ok := item.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return []map[string]any{}
	}
}

func adminSessionNormalizeReindexDeferredReasons(rescanResult map[string]any) []string {
	reasons := []string{}
	for label, result := range map[string]map[string]any{
		"rescan": rescanResult,
	} {
		status := strings.ToLower(strings.TrimSpace(stringFromMap(result, "status")))
		switch status {
		case "failed", "error", "blocked", "cancelled", "partial_error", "deferred":
			reasons = append(reasons, label+":"+status)
		}
		if intFromAny(result["failed"], 0) > 0 || len(sliceFromAny(result["failed_turns"])) > 0 {
			reasons = append(reasons, label+":failed_items")
		}
		if intFromAny(result["deferred"], 0) > 0 || intFromAny(result["queued"], 0) > 0 {
			reasons = append(reasons, label+":pending_reprocessing")
		}
	}
	return uniqueStrings(reasons)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
