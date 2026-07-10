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
	ChatSessionID string                          `json:"chat_session_id"`
	MaxItems      int                             `json:"max_items"`
	BatchSize     int                             `json:"batch_size"`
	TurnIndices   []int                           `json:"turn_indices"`
	ClientMeta    map[string]any                  `json:"client_meta"`
	RepairEntries []dto.ChatLogRepairEntryRequest `json:"repair_entries"`
	Entries       []dto.ChatLogRepairEntryRequest `json:"entries"`
	DryRun        bool                            `json:"dry_run"`
	SkipRepair    bool                            `json:"skip_repair"`
	SkipRescan    bool                            `json:"skip_rescan"`
	SkipReindex   bool                            `json:"skip_reindex"`
	ForceReindex  *bool                           `json:"force_reindex"`
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
	if req.MaxItems <= 0 {
		req.MaxItems = 1000
	}
	if req.MaxItems > 5000 {
		req.MaxItems = 5000
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 50
	}
	if req.BatchSize > 200 {
		req.BatchSize = 200
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
	reviewNeededTurns := adminSessionNormalizeConflictTurns(before)
	if len(reviewNeededTurns) > 0 {
		warnings = append(warnings, "raw_mismatch_or_partial_requires_review")
	}

	var repairResult map[string]any
	if !req.SkipRepair && len(entries) > 0 {
		if progress != nil {
			progress(map[string]any{
				"status":                "running",
				"stage":                 "raw_repair_replay",
				"repair_entry_count":    len(entries),
				"review_needed_turns":   reviewNeededTurns,
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
		result, err := s.runChatLogRepairReplay(ctx, sid, repairReq)
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

	var rescanResult map[string]any
	if !req.SkipRescan {
		meta := adminSessionNormalizeClientMeta(req.ClientMeta)
		rescanReq := adminRescanRequest{
			ChatSessionID: sid,
			MaxItems:      req.MaxItems,
			TurnIndices:   uniqueSortedInts(req.TurnIndices),
			ClientMeta:    meta,
			DryRun:        req.DryRun,
			Background:    false,
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

	var reindexResult map[string]any
	if !req.SkipReindex {
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
	status := adminSessionNormalizeStatus(repairResult, rescanResult, reindexResult, warnings)
	result := map[string]any{
		"status":              status,
		"contract_version":    "session-normalize.v1",
		"source":              s.storeWriteSource(),
		"chat_session_id":     sid,
		"dry_run":             req.DryRun,
		"destructive":         false,
		"rollback_attempted":  false,
		"delete_attempted":    false,
		"plan":                plan,
		"counts_before":       before,
		"counts_after":        after,
		"repair_replay":       repairResult,
		"rescan":              rescanResult,
		"reindex":             reindexResult,
		"review_needed_turns": reviewNeededTurns,
		"warnings":            uniqueStrings(warnings),
		"generated_at":        time.Now().UTC(),
		"note":                "session normalize orchestrated safe raw repair, Critic artifact backfill, hierarchy backfill, and vector reindex without rollback or destructive trim",
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "session_normalize",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session Normalize completed",
		DetailsJSON: mustCompactJSON(map[string]any{
			"status":              status,
			"dry_run":             req.DryRun,
			"destructive":         false,
			"repair_entry_count":  len(entries),
			"review_needed_turns": reviewNeededTurns,
			"plan":                plan,
			"warnings":            uniqueStrings(warnings),
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: time.Now().UTC(),
	})
	if progress != nil {
		progress(map[string]any{
			"status":              "completed",
			"stage":               "completed",
			"progress_percent":    100,
			"review_needed_turns": reviewNeededTurns,
			"counts_after":        after,
			"warnings":            uniqueStrings(warnings),
		})
	}
	return result, nil
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
		if item.TurnIndex <= 0 {
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

func adminSessionNormalizeClientMeta(raw map[string]any) map[string]any {
	meta := cloneMapAny(raw)
	if meta == nil {
		meta = map[string]any{}
	}
	meta["source"] = "session_normalize"
	meta["background"] = true
	meta["force_derived_rebuild"] = true
	meta["force_world_rule_backfill"] = true
	meta["force_raw_world_rule_audit"] = true
	meta["force_episode_backfill"] = true
	meta["force_hierarchy_backfill"] = true
	meta["full_session_backfill"] = true
	meta["session_normalize_full_session_backfill"] = true
	return meta
}

func adminSessionNormalizeForceReindex(req adminSessionNormalizeRequest) bool {
	if req.ForceReindex == nil {
		return true
	}
	return *req.ForceReindex
}

func adminSessionNormalizeJobRequest(sid string, req adminSessionNormalizeRequest, entries []dto.ChatLogRepairEntryRequest) map[string]any {
	return map[string]any{
		"chat_session_id":       sid,
		"max_items":             req.MaxItems,
		"batch_size":            req.BatchSize,
		"turn_indices":          uniqueSortedInts(req.TurnIndices),
		"repair_entry_count":    len(entries),
		"repair_turn_preview":   adminSessionNormalizeEntryTurns(entries, 20),
		"dry_run":               req.DryRun,
		"skip_repair":           req.SkipRepair,
		"skip_rescan":           req.SkipRescan,
		"skip_reindex":          req.SkipReindex,
		"force_reindex":         adminSessionNormalizeForceReindex(req),
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
	turns = uniqueSortedInts(turns)
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
		"chat_log_rows":        0,
		"raw_turns":            0,
		"raw_complete_turns":   0,
		"raw_partial_turns":    0,
		"memories":             0,
		"direct_evidence":      0,
		"kg_triples":           0,
		"world_rules":          0,
		"episode_summaries":    0,
		"chapter_summaries":    0,
		"arc_summaries":        0,
		"saga_digests":         0,
		"min_turn":             0,
		"max_turn":             0,
		"partial_turn_preview": []int{},
	}
	warnings := []string{}
	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		warnings = append(warnings, "list_chat_logs_failed: "+err.Error())
	} else {
		roleByTurn := map[int]map[string]bool{}
		minTurn, maxTurn := 0, 0
		for _, log := range logs {
			if log.ChatSessionID != sid || log.TurnIndex <= 0 {
				continue
			}
			if minTurn == 0 || log.TurnIndex < minTurn {
				minTurn = log.TurnIndex
			}
			if log.TurnIndex > maxTurn {
				maxTurn = log.TurnIndex
			}
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
		completeTurns := 0
		for turn, roles := range roleByTurn {
			if roles["user"] && roles["assistant"] {
				completeTurns++
			} else {
				partialTurns = append(partialTurns, turn)
			}
		}
		counts["chat_log_rows"] = len(logs)
		counts["raw_turns"] = len(roleByTurn)
		counts["raw_complete_turns"] = completeTurns
		counts["raw_partial_turns"] = len(partialTurns)
		counts["min_turn"] = minTurn
		counts["max_turn"] = maxTurn
		counts["partial_turn_preview"] = adminSessionNormalizeLimitInts(uniqueSortedInts(partialTurns), 20)
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
		"critic_rescan_turn_indices":     uniqueSortedInts(req.TurnIndices),
		"critic_rescan_force_backfills":  true,
		"world_rule_review_needed":       rawTurns > 0 && worldRules == 0,
		"episode_review_needed":          rawTurns >= normalizedEpisodeInterval(0) && episodes == 0,
		"vector_reindex_max_items":       req.MaxItems,
		"vector_reindex_force":           adminSessionNormalizeForceReindex(req),
		"safe_to_repeat":                 true,
		"destructive":                    false,
		"raw_turns_before":               rawTurns,
		"memory_rows_before":             memories,
		"advanced_tools_consolidated":    []string{"active_chat_dry_run", "repair_replay", "admin_rescan", "hierarchy_backfill", "admin_reindex"},
		"visible_trim_delete_protection": "enabled",
	}
}

func adminSessionNormalizeConflictTurns(snapshot map[string]any) []int {
	return adminSessionNormalizeLimitInts(intSliceFromAny(snapshot["partial_turn_preview"]), 50)
}

func adminSessionNormalizeLimitInts(values []int, limit int) []int {
	values = uniqueSortedInts(values)
	if limit > 0 && len(values) > limit {
		return values[:limit]
	}
	return values
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

func adminSessionNormalizeStatus(repairResult, rescanResult, reindexResult map[string]any, warnings []string) string {
	for _, result := range []map[string]any{repairResult, rescanResult, reindexResult} {
		status := strings.ToLower(strings.TrimSpace(stringFromMap(result, "status")))
		if status == "failed" || status == "error" {
			return "failed"
		}
		if status == "blocked" {
			return "blocked"
		}
		if intFromAny(result["failed"], 0) > 0 || len(sliceFromAny(result["failed_turns"])) > 0 || len(sliceFromAny(result["errors"])) > 0 {
			return "partial_error"
		}
	}
	if len(warnings) > 0 {
		return "partial_warning"
	}
	return "ok"
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
