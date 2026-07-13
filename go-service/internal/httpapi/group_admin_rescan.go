package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type adminRescanRequest struct {
	ChatSessionID string         `json:"chat_session_id"`
	MaxItems      int            `json:"max_items"`
	TurnIndices   []int          `json:"turn_indices"`
	ClientMeta    map[string]any `json:"client_meta"`
	DryRun        bool           `json:"dry_run"`
	Background    bool           `json:"background"`
}

func (s *Server) runAdminRescan(ctx context.Context, sid string, req adminRescanRequest) (map[string]any, error) {
	return s.runAdminRescanWithProgress(ctx, sid, req, nil)
}

func (s *Server) runAdminRescanWithProgress(ctx context.Context, sid string, req adminRescanRequest, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = 50
	}
	if maxItems > 1000 {
		maxItems = 1000
	}

	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	targetTurns := map[int]bool{}
	for _, turn := range req.TurnIndices {
		if turn >= 0 {
			targetTurns[turn] = true
		}
	}
	forceWorldRuleBackfill := boolFromAny(req.ClientMeta["force_world_rule_backfill"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"])
	fullSessionBackfill := boolFromAny(req.ClientMeta["full_session_backfill"]) ||
		boolFromAny(req.ClientMeta["session_normalize_full_session_backfill"])
	forceRawWorldRuleAudit := boolFromAny(req.ClientMeta["force_raw_world_rule_audit"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"]) ||
		fullSessionBackfill
	forceDerivedRebuild := boolFromAny(req.ClientMeta["force_derived_rebuild"]) ||
		boolFromAny(req.ClientMeta["derived_backfill_only"])
	memoryTurns := map[int]bool{}
	for _, mem := range memories {
		if mem.ChatSessionID == sid && mem.TurnIndex >= 0 {
			memoryTurns[mem.TurnIndex] = true
		}
	}
	turnLogs := map[int]map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex < 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[log.TurnIndex] {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if turnLogs[log.TurnIndex] == nil {
			turnLogs[log.TurnIndex] = map[string]string{}
		}
		turnLogs[log.TurnIndex][role] = appendUniqueTurnRoleText(turnLogs[log.TurnIndex][role], log.Content)
	}

	turns := []int{}
	for turn, roleMap := range turnLogs {
		if memoryTurns[turn] && !forceDerivedRebuild {
			continue
		}
		if strings.TrimSpace(roleMap["user"]) == "" && strings.TrimSpace(roleMap["assistant"]) == "" {
			continue
		}
		turns = append(turns, turn)
	}
	turns = uniqueSortedNonNegativeInts(turns)
	if len(turns) > maxItems {
		turns = turns[:maxItems]
	}
	if progress != nil {
		progress(map[string]any{
			"status":             "running",
			"stage":              "candidate_scan",
			"candidate_count":    len(turns),
			"processed":          0,
			"succeeded":          0,
			"failed_count":       0,
			"skipped_count":      0,
			"processed_turns":    []int{},
			"failed_turns":       []map[string]any{},
			"skipped_turns":      []map[string]any{},
			"progress_percent":   0,
			"foreground_timeout": false,
			"timeout_policy":     "background_job_detached_from_http_request",
		})
	}

	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	failedTurns := []map[string]any{}
	skippedTurns := []map[string]any{}
	processedTurns := []int{}
	succeeded := 0
	failed := 0
	skipped := 0
	artifactCounts := map[string]int{
		"memories":          0,
		"evidence":          0,
		"kg_triples":        0,
		"character_events":  0,
		"storylines":        0,
		"world_rules":       0,
		"character_states":  0,
		"pending_threads":   0,
		"active_states":     0,
		"entities":          0,
		"trust_states":      0,
		"episode_summaries": 0,
		"chapter_summaries": 0,
		"arc_summaries":     0,
		"saga_digests":      0,
		"vectors_upserted":  0,
	}
	warnings := []string{}
	episodeInterval := normalizedEpisodeInterval(intFromAny(req.ClientMeta["episode_interval_turns"], 0))
	forceEpisodeBackfill := boolFromAny(req.ClientMeta["force_episode_backfill"])
	episodeBackfill := skippedEpisodeBackfillResult(req.DryRun, episodeInterval, forceEpisodeBackfill, "not_run")
	worldRuleBackfill := skippedWorldRuleBackfillResult(req.DryRun, "not_run")
	hierarchyBackfill := skippedHierarchyBackfillResult(req.DryRun, "not_run")
	runBackfills := func(runLogs []store.ChatLog, runMemories []store.Memory, runEvidence []store.DirectEvidence, runTargets map[int]bool) {
		if progress != nil {
			progress(map[string]any{"stage": "episode_backfill", "candidate_count": len(turns)})
		}
		episodeBackfill = s.backfillEpisodeSummariesFromChatLogs(ctx, sid, runLogs, runMemories, runEvidence, episodeInterval, req.DryRun, runTargets, forceEpisodeBackfill)
		artifactCounts["episode_summaries"] += intFromAny(episodeBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(episodeBackfill, "error")); errText != "" {
			warnings = append(warnings, "episode_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "world_rule_backfill", "episode_backfill": episodeBackfill})
		}
		worldRuleBackfill = s.backfillWorldRulesFromMemories(ctx, sid, runMemories, runTargets, req.DryRun)
		artifactCounts["world_rules"] += intFromAny(worldRuleBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(worldRuleBackfill, "error")); errText != "" {
			warnings = append(warnings, "world_rule_backfill_failed: "+errText)
		}
		shouldRunRawWorldAudit := forceWorldRuleBackfill &&
			(forceRawWorldRuleAudit || (artifactCounts["world_rules"] == 0 && intFromAny(worldRuleBackfill["generated"], 0) == 0))
		if shouldRunRawWorldAudit {
			rawWorldRuleBackfill := s.backfillWorldRulesFromChatLogs(ctx, sid, runLogs, runTargets, req.DryRun, extractionCfg.Critic, progress)
			worldRuleBackfill = mergeWorldRuleBackfillResults(worldRuleBackfill, rawWorldRuleBackfill)
			artifactCounts["world_rules"] += intFromAny(rawWorldRuleBackfill["generated"], 0)
			if errText := strings.TrimSpace(stringFromMap(rawWorldRuleBackfill, "error")); errText != "" {
				warnings = append(warnings, "raw_world_rule_backfill_failed: "+errText)
			}
		}
		if progress != nil {
			progress(map[string]any{"stage": "hierarchy_backfill", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill})
		}
		hierarchyBackfill = s.backfillHierarchySummaries(ctx, sid, runLogs, runTargets, req.ClientMeta, req.DryRun)
		artifactCounts["chapter_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["chapter"])["generated"], 0)
		artifactCounts["arc_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["arc"])["generated"], 0)
		artifactCounts["saga_digests"] += intFromAny(mapFromAny(hierarchyBackfill["saga"])["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(hierarchyBackfill, "error")); errText != "" {
			warnings = append(warnings, "hierarchy_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "backfill_done", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill, "hierarchy_backfill": hierarchyBackfill})
		}
	}
	episodeBackfillOnly := boolFromAny(req.ClientMeta["episode_backfill_only"])
	if episodeBackfillOnly {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":                "ok",
			"source":                s.storeWriteSource(),
			"chat_session_id":       sid,
			"dry_run":               req.DryRun,
			"episode_backfill_only": true,
			"candidate_count":       0,
			"succeeded":             0,
			"failed":                0,
			"skipped":               0,
			"processed_turns":       []int{},
			"failed_turns":          []map[string]any{},
			"skipped_turns":         []map[string]any{},
			"artifact_counts":       artifactCounts,
			"episode_backfill":      episodeBackfill,
			"world_rule_backfill":   worldRuleBackfill,
			"hierarchy_backfill":    hierarchyBackfill,
			"warnings":              warnings,
			"llm_config_trace":      llmTrace,
			"note":                  "rescan ran episode/world-rule backfill only and did not reprocess Critic-derived artifacts",
		}, nil
	}

	if len(turns) == 0 {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":              "ok",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     0,
			"succeeded":           0,
			"failed":              0,
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        []map[string]any{},
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan found no raw chat_log turns missing memory for this session/target set",
		}, nil
	}

	if !extractionCfg.Critic.hasConfig() {
		for _, turn := range turns {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_config_missing"})
		}
		runBackfills(logs, memories, nil, targetTurns)
		if progress != nil {
			progress(adminRescanProgress(len(turns), len(turns), 0, len(turns), 0, []int{}, failedTurns, []map[string]any{}, artifactCounts, 0, "critic_config_missing"))
		}
		return map[string]any{
			"status":              "ok",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     len(turns),
			"succeeded":           0,
			"failed":              len(turns),
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        failedTurns,
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan needs configured Critic LLM settings before derived Memory/Direct Evidence/KG/state can be regenerated",
		}, nil
	}

	now := time.Now().UTC()
	for _, turn := range turns {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		roleMap := turnLogs[turn]
		userText := sanitizeCriticStorageText(roleMap["user"])
		assistantText := sanitizeCriticStorageText(roleMap["assistant"])
		if strings.TrimSpace(assistantText) == "" {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "assistant_content_missing"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "assistant_content_missing"))
			}
			continue
		}
		if shouldApplyCompleteTurnOOCGuard(userText, assistantText, nil) {
			skipped++
			skippedTurns = append(skippedTurns, map[string]any{"turn_index": turn, "reason": "ooc_guard"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "ooc_guard"))
			}
			continue
		}
		if req.DryRun {
			skipped++
			skippedTurns = append(skippedTurns, map[string]any{"turn_index": turn, "reason": "dry_run"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "dry_run"))
			}
			continue
		}
		extraction, trace, err := s.runCompleteTurnCriticFromCanonicalLogs(ctx, sid, turn, userText, assistantText, extractionCfg.Critic)
		if err != nil {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_extract_failed: " + err.Error(), "trace": trace})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "critic_extract_failed"))
			}
			continue
		}
		content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
		saveResult := s.saveCriticExtractionArtifacts(ctx, sid, turn, extraction, content, extractionCfg.Embedder, now)
		if saveResult.Errors > 0 {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "artifact_save_failed", "errors": saveResult.ErrorDetails})
			warnings = append(warnings, saveResult.Warnings...)
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "artifact_save_failed"))
			}
			continue
		}
		succeeded++
		processedTurns = append(processedTurns, turn)
		artifactCounts["memories"] += saveResult.Memories
		artifactCounts["evidence"] += saveResult.Evidence
		artifactCounts["kg_triples"] += saveResult.KGTriples
		artifactCounts["character_events"] += saveResult.CharacterEvents
		artifactCounts["storylines"] += saveResult.Storylines
		artifactCounts["world_rules"] += saveResult.WorldRules
		artifactCounts["character_states"] += saveResult.CharacterStates
		artifactCounts["pending_threads"] += saveResult.PendingThreads
		artifactCounts["active_states"] += saveResult.ActiveStates
		artifactCounts["entities"] += saveResult.Entities
		artifactCounts["trust_states"] += saveResult.TrustStates
		artifactCounts["vectors_upserted"] += saveResult.VectorsUpserted
		warnings = append(warnings, saveResult.Warnings...)
		if progress != nil {
			progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "saved"))
		}
	}

	backfillTargets := targetTurns
	if fullSessionBackfill {
		backfillTargets = map[int]bool{}
	} else if len(processedTurns) > 0 {
		backfillTargets = intsToSet(processedTurns)
	}
	postLogs := logs
	postMemories := memories
	postEvidence := []store.DirectEvidence(nil)
	if s.Store != nil {
		if listed, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			postLogs = listed
		}
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			postMemories = listed
		}
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			postEvidence = listed
		}
	}
	runBackfills(postLogs, postMemories, postEvidence, backfillTargets)

	if succeeded > 0 {
		_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "rescan_rebuild",
			TargetType:    "session",
			TargetID:      0,
			Summary:       fmt.Sprintf("Rescan rebuilt derived artifacts for %d turns", succeeded),
			DetailsJSON: mustCompactJSON(map[string]any{
				"processed_turns":     processedTurns,
				"artifact_counts":     artifactCounts,
				"episode_backfill":    episodeBackfill,
				"world_rule_backfill": worldRuleBackfill,
				"hierarchy_backfill":  hierarchyBackfill,
				"failed":              failed,
				"skipped":             skipped,
			}),
			Source:    s.storeWriteSource(),
			CreatedAt: now,
		})
	}

	result := map[string]any{
		"status":              "ok",
		"source":              s.storeWriteSource(),
		"chat_session_id":     sid,
		"dry_run":             req.DryRun,
		"candidate_count":     len(turns),
		"succeeded":           succeeded,
		"failed":              failed,
		"skipped":             skipped,
		"processed_turns":     uniqueSortedNonNegativeInts(processedTurns),
		"failed_turns":        failedTurns,
		"skipped_turns":       skippedTurns,
		"artifact_counts":     artifactCounts,
		"episode_backfill":    episodeBackfill,
		"world_rule_backfill": worldRuleBackfill,
		"hierarchy_backfill":  hierarchyBackfill,
		"warnings":            warnings,
		"llm_config_trace":    llmTrace,
		"note":                "rescan reprocessed raw chat_logs that were missing memory and rebuilt derived artifacts through the configured Critic pipeline",
	}
	if progress != nil {
		progress(map[string]any{
			"status":              "completed",
			"stage":               "completed",
			"candidate_count":     len(turns),
			"processed":           len(processedTurns) + failed + skipped,
			"succeeded":           succeeded,
			"failed_count":        failed,
			"skipped_count":       skipped,
			"processed_turns":     uniqueSortedNonNegativeInts(processedTurns),
			"failed_turns":        failedTurns,
			"skipped_turns":       skippedTurns,
			"artifact_counts":     cloneIntMapAny(artifactCounts),
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"progress_percent":    100,
		})
	}
	return result, nil
}

func adminRescanProgress(processed, total, succeeded, failed, skipped int, processedTurns []int, failedTurns, skippedTurns []map[string]any, artifactCounts map[string]int, lastTurn int, lastReason string) map[string]any {
	return map[string]any{
		"status":           "running",
		"stage":            "critic_artifact_rebuild",
		"candidate_count":  total,
		"processed":        processed,
		"succeeded":        succeeded,
		"failed_count":     failed,
		"skipped_count":    skipped,
		"processed_turns":  uniqueSortedNonNegativeInts(processedTurns),
		"failed_turns":     append([]map[string]any{}, failedTurns...),
		"skipped_turns":    append([]map[string]any{}, skippedTurns...),
		"artifact_counts":  cloneIntMapAny(artifactCounts),
		"last_processed":   lastTurn,
		"last_reason":      nilIfEmpty(lastReason),
		"progress_percent": adminJobProgressPercent(processed, total),
	}
}

func cloneIntMapAny(in map[string]int) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func skippedEpisodeBackfillResult(dryRun bool, interval int, force bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
		"reason":    reason,
	}
}

func skippedWorldRuleBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"reason":    reason,
	}
}

func skippedHierarchyBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":  "skipped",
		"dry_run": dryRun,
		"reason":  reason,
		"chapter": hierarchyLayerBackfillResult(dryRun, reason),
		"arc":     hierarchyLayerBackfillResult(dryRun, reason),
		"saga":    hierarchyLayerBackfillResult(dryRun, reason),
	}
}

func hierarchyLayerBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"blocked":   []map[string]any{},
		"reason":    reason,
	}
}

func (s *Server) backfillHierarchySummaries(ctx context.Context, sid string, logs []store.ChatLog, targetTurns map[int]bool, meta map[string]any, dryRun bool) map[string]any {
	result := skippedHierarchyBackfillResult(dryRun, "not_run")
	result["status"] = "ok"
	result["reason"] = nil
	if s == nil || s.Store == nil {
		result["status"] = "skipped"
		result["reason"] = "store_unavailable"
		return result
	}
	minTurn, maxTurn := chatLogTurnBounds(sid, logs)
	if minTurn <= 0 || maxTurn <= 0 {
		result["status"] = "skipped"
		result["reason"] = "no_chat_logs"
		return result
	}
	chapterInterval := normalizedChapterInterval(intFromAny(meta["chapter_interval_turns"], 0))
	arcInterval := normalizedHierarchyInterval(intFromAny(meta["arc_interval_turns"], 0), 240, chapterInterval, 1200)
	sagaInterval := normalizedHierarchyInterval(intFromAny(meta["saga_interval_turns"], 0), 960, arcInterval, 4800)
	force := boolFromAny(meta["force_hierarchy_backfill"]) || boolFromAny(meta["force_chapter_backfill"]) || boolFromAny(meta["force_arc_backfill"]) || boolFromAny(meta["force_saga_backfill"])

	var chapterResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "chapter_auto_enabled", true) {
		var err error
		chapterResult, err = s.backfillChapterSummaries(ctx, sid, minTurn, maxTurn, chapterInterval, targetTurns, dryRun, force)
		if err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		chapterResult = hierarchyLayerBackfillResult(dryRun, "chapter_auto_disabled")
		chapterResult["interval"] = chapterInterval
	}
	result["chapter"] = chapterResult

	var arcResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "arc_auto_enabled", true) {
		var err error
		arcResult, err = s.backfillArcSummaries(ctx, sid, minTurn, maxTurn, arcInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		arcResult = hierarchyLayerBackfillResult(dryRun, "arc_auto_disabled")
		arcResult["interval"] = arcInterval
	}
	result["arc"] = arcResult

	var sagaResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "saga_auto_enabled", true) {
		var err error
		sagaResult, err = s.backfillSagaDigests(ctx, sid, minTurn, maxTurn, sagaInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		sagaResult = hierarchyLayerBackfillResult(dryRun, "saga_auto_disabled")
		sagaResult["interval"] = sagaInterval
	}
	result["saga"] = sagaResult
	result["chapter_interval_turns"] = chapterInterval
	result["arc_interval_turns"] = arcInterval
	result["saga_interval_turns"] = sagaInterval
	result["range"] = map[string]any{"from_turn": minTurn, "to_turn": maxTurn}
	result["policy"] = "step23_closed_range_hierarchy_backfill"
	return result
}

func hierarchyBackfillLayerEnabled(meta map[string]any, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}
	if _, ok := meta[key]; !ok {
		return fallback
	}
	return boolFromAny(meta[key])
}

func (s *Server) backfillChapterSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		episodes = filterEpisodes(episodes, "", fromTurn, toTurn, 0)
		if !episodeCoverageComplete(episodes, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_episode")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := chapterSummaryExists(ctx, chapterStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		chapter, _ := s.buildChapterSummaryForRange(ctx, sid, fromTurn, toTurn, chapterIndexForRange(toTurn, interval), episodes)
		if err := chapterStore.SaveChapterSummary(ctx, &chapter); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillArcSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", fromTurn, toTurn, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		if !chapterCoverageComplete(chapters, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_chapter")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := arcSummaryExists(ctx, arcStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		arc, _ := s.buildArcSummaryForRange(ctx, sid, fromTurn, toTurn, hierarchyIndexForRange(toTurn, interval), chapters)
		if err := arcStore.SaveArcSummary(ctx, sid, &arc); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillSagaDigests(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	sagaStore, ok := s.Store.(store.SagaDigestStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "saga_store_not_available"
		return layer, nil
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		arcs, err := arcStore.SearchArcSummaries(ctx, sid, "", fromTurn, toTurn, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		if !arcCoverageComplete(arcs, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_arc")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := sagaDigestExists(ctx, sagaStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		saga, _ := s.buildSagaDigestForRange(ctx, sid, fromTurn, toTurn, arcs)
		if err := sagaStore.SaveSagaDigest(ctx, sid, &saga); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func normalizedHierarchyInterval(value, fallback, minValue, maxValue int) int {
	if value <= 0 {
		value = fallback
	}
	if minValue > 0 && value < minValue {
		value = minValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func alignHierarchyStart(minTurn, interval int) int {
	if minTurn <= 1 {
		return 1
	}
	return ((minTurn-1)/interval)*interval + 1
}

func chatLogTurnBounds(sid string, logs []store.ChatLog) (int, int) {
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(log.Content) == "" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	return minTurn, maxTurn
}

func addHierarchyBlocked(layer map[string]any, fromTurn, toTurn int, reason string) {
	layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
	blocked := []map[string]any{}
	if raw, ok := layer["blocked"].([]map[string]any); ok {
		blocked = append(blocked, raw...)
	}
	blocked = append(blocked, map[string]any{"from_turn": fromTurn, "to_turn": toTurn, "reason": reason})
	layer["blocked"] = blocked
}

func chapterSummaryExists(ctx context.Context, chapterStore store.ChapterSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := chapterStore.SearchChapterSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func arcSummaryExists(ctx context.Context, arcStore store.ArcSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := arcStore.SearchArcSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func sagaDigestExists(ctx context.Context, sagaStore store.SagaDigestStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := sagaStore.SearchSagaDigests(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func episodeCoverageComplete(items []store.EpisodeSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func chapterCoverageComplete(items []store.ChapterSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func arcCoverageComplete(items []store.ArcSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

type turnRange struct {
	fromTurn int
	toTurn   int
}

func turnRangesCover(ranges []turnRange, fromTurn, toTurn int) bool {
	if fromTurn <= 0 || toTurn < fromTurn {
		return false
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].fromTurn == ranges[j].fromTurn {
			return ranges[i].toTurn < ranges[j].toTurn
		}
		return ranges[i].fromTurn < ranges[j].fromTurn
	})
	next := fromTurn
	for _, item := range ranges {
		if item.fromTurn <= 0 || item.toTurn < item.fromTurn {
			continue
		}
		if item.toTurn < next {
			continue
		}
		if item.fromTurn > next {
			return false
		}
		next = item.toTurn + 1
		if next > toTurn {
			return true
		}
	}
	return next > toTurn
}

func intsToSet(items []int) map[int]bool {
	out := map[int]bool{}
	for _, item := range items {
		if item >= 0 {
			out[item] = true
		}
	}
	return out
}

func uniqueSortedNonNegativeInts(values []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if value < 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func (s *Server) backfillEpisodeSummariesFromChatLogs(ctx context.Context, sid string, logs []store.ChatLog, memories []store.Memory, evidence []store.DirectEvidence, interval int, dryRun bool, targetTurns map[int]bool, force bool) map[string]any {
	result := map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		result["reason"] = "episode_store_not_available"
		return result
	}
	if interval <= 0 {
		interval = normalizedEpisodeInterval(0)
	}
	if memories == nil {
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			memories = listed
		}
	}
	if evidence == nil {
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			evidence = listed
		}
	}
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	if minTurn <= 0 || maxTurn <= 0 {
		result["reason"] = "no_chat_logs"
		return result
	}
	if minTurn > 1 {
		minTurn = ((minTurn-1)/interval)*interval + 1
	}
	candidates := 0
	generated := 0
	existingCount := 0
	skipped := 0
	partialSkipped := 0
	for fromTurn := minTurn; fromTurn <= maxTurn; fromTurn += interval {
		fullToTurn := fromTurn + interval - 1
		if fullToTurn > maxTurn {
			partialSkipped++
			skipped++
			continue
		}
		toTurn := fullToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			skipped++
			continue
		}
		chatLogs := filterChatLogsForTurnRange(logs, fromTurn, toTurn, 24)
		rangeMemories := filterMemoriesForTurnRange(memories, sid, fromTurn, toTurn)
		rangeEvidence := filterEvidenceForTurnRange(evidence, sid, fromTurn, toTurn)
		if len(chatLogs) == 0 && len(rangeMemories) == 0 && len(rangeEvidence) == 0 {
			skipped++
			continue
		}
		candidates++
		existing, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, fromTurn, toTurn)
		if err == nil {
			foundExact := false
			for _, item := range existing {
				if item.FromTurn == fromTurn && item.ToTurn == toTurn {
					foundExact = true
					break
				}
			}
			if foundExact {
				if !force {
					existingCount++
					continue
				}
				if !dryRun {
					if deleter, ok := s.Store.(episodeSummaryRangeDeleter); ok {
						if _, err := deleter.DeleteEpisodeSummariesInRange(ctx, sid, fromTurn, toTurn); err != nil {
							result["status"] = "partial_error"
							result["error"] = err.Error()
							return result
						}
					} else {
						existingCount++
						continue
					}
				}
			}
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		if dryRun {
			continue
		}
		episode, _ := buildEpisodeSummaryForRangeWithArtifacts(sid, fromTurn, toTurn, chatLogs, rangeMemories, rangeEvidence)
		if err := episodeStore.SaveEpisodeSummary(ctx, &episode); err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		generated++
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existingCount
	result["skipped"] = skipped
	result["partial_skipped"] = partialSkipped
	return result
}

func turnRangeContainsTargetTurn(fromTurn, toTurn int, targetTurns map[int]bool) bool {
	if len(targetTurns) == 0 {
		return true
	}
	for turn := range targetTurns {
		if turn >= fromTurn && turn <= toTurn {
			return true
		}
	}
	return false
}

func filterMemoriesForTurnRange(items []store.Memory, sid string, fromTurn, toTurn int) []store.Memory {
	out := []store.Memory{}
	for _, item := range items {
		if item.ChatSessionID != sid || item.TurnIndex <= 0 {
			continue
		}
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterEvidenceForTurnRange(items []store.DirectEvidence, sid string, fromTurn, toTurn int) []store.DirectEvidence {
	out := []store.DirectEvidence{}
	for _, item := range items {
		if item.ChatSessionID != sid {
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
		if start <= 0 {
			continue
		}
		if toTurn > 0 && start > toTurn {
			continue
		}
		if fromTurn > 0 && end < fromTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}
