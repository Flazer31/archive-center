package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildAutonomyPlan(degraded bool, guideMode, narrativeStance string) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":           status,
		"guide_mode":       guideMode,
		"narrative_stance": narrativeStance,
		"suggested_action": "continue",
		"would_call_llm":   false,
		"would_write":      false,
	}
}

// buildMicroBeatProposal assembles the JS-adapter-consumable micro_beat_proposal bundle.
func buildMicroBeatProposal(degraded bool, pendingThreads []store.PendingThread, storylines []store.Storyline, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	beats := []map[string]any{}
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		beats = append(beats, map[string]any{
			"kind":        "pending_thread",
			"thread_key":  pt.ThreadKey,
			"description": strings.Join(strings.Fields(strings.TrimSpace(pt.Description)), " "),
		})
	}
	for _, sl := range storylines {
		if len(beats) >= recallLimit {
			break
		}
		beats = append(beats, map[string]any{
			"kind":            "storyline",
			"name":            sl.Name,
			"current_context": strings.Join(strings.Fields(strings.TrimSpace(sl.CurrentContext)), " "),
		})
	}
	return map[string]any{
		"status":         status,
		"beats":          beats,
		"would_call_llm": false,
		"would_write":    false,
	}
}

// buildSceneStepProposal assembles the JS-adapter-consumable scene_step_proposal bundle.
func buildSceneStepProposal(degraded bool, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	steps := []map[string]any{}
	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":       "active_state",
			"state_type": as.StateType,
			"content":    strings.Join(strings.Fields(strings.TrimSpace(as.Content)), " "),
		})
	}
	for _, cl := range canonicalLayers {
		if len(steps) >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":       "canonical_layer",
			"layer_type": cl.LayerType,
			"content":    strings.Join(strings.Fields(strings.TrimSpace(cl.Content)), " "),
		})
	}
	for _, es := range episodeSums {
		if len(steps) >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":      "episode_summary",
			"from_turn": es.FromTurn,
			"to_turn":   es.ToTurn,
			"summary":   strings.Join(strings.Fields(strings.TrimSpace(es.SummaryText)), " "),
		})
	}
	return map[string]any{
		"status":         status,
		"steps":          steps,
		"would_call_llm": false,
		"would_write":    false,
	}
}

// buildCombinedProposal assembles the JS-adapter-consumable combined_proposal bundle.
func buildCombinedProposal(degraded bool, microBeatProposal, sceneStepProposal map[string]any) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	beats, _ := microBeatProposal["beats"].([]map[string]any)
	steps, _ := sceneStepProposal["steps"].([]map[string]any)
	return map[string]any{
		"status":           status,
		"micro_beat_count": len(beats),
		"scene_step_count": len(steps),
		"source":           "go_r1_read_shadow",
		"would_call_llm":   false,
		"would_write":      false,
	}
}

// buildWritebackPreview assembles the JS-adapter-consumable writeback_preview bundle.
func buildWritebackPreview(degraded bool) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":      status,
		"would_write": false,
		"targets": []string{
			"memories",
			"direct_evidence",
			"kg_triples",
			"storylines",
			"world_rules",
			"pending_threads",
		},
		"notes": "R1 read-shadow: no writes performed. Writeback requires authority elevation and store-write mode.",
	}
}

// buildWritebackPlan assembles the JS-adapter-consumable writeback_plan bundle.
func buildWritebackPlan(sid string, turnIndex int, storeWriteEnabled bool, writeSource string, req dto.M4CompleteTurnRequest) map[string]any {
	preview := ""
	if req.UserInput != nil {
		preview = strings.TrimSpace(*req.UserInput)
	}
	if req.AssistantContent != nil {
		if preview != "" {
			preview += " | "
		}
		preview += strings.TrimSpace(*req.AssistantContent)
	}
	preview = truncateRunes(preview, 200)

	return map[string]any{
		"status":              "ready",
		"chat_session_id":     sid,
		"turn_index":          turnIndex,
		"store_write_enabled": storeWriteEnabled,
		"would_write":         storeWriteEnabled,
		"write_source":        writeSource,
		"targets": []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"direct_evidence",
			"kg_triples",
			"entities",
			"narrative_state",
		},
		"content_preview": preview,
		"notes":           writebackPlanNote(storeWriteEnabled, writeSource),
	}
}

// buildInputTransparency assembles the JS-adapter-consumable input_transparency bundle.
func buildInputTransparency(sid string, turnIndex int, text string, storeWriteEnabled bool, writeSource string) map[string]any {
	return map[string]any{
		"status":                "ready",
		"chat_session_id":       sid,
		"turn_index":            turnIndex,
		"effective_input_chars": len([]rune(text)),
		"preview":               truncateRunes(text, 200),
		"store_write_enabled":   storeWriteEnabled,
		"would_write":           storeWriteEnabled,
		"write_source":          writeSource,
		"notes":                 inputTransparencyNote(storeWriteEnabled, writeSource),
	}
}

func writebackPlanNote(storeWriteEnabled bool, writeSource string) string {
	if !storeWriteEnabled {
		return "R1 read-shadow writeback plan: no Store writes are enabled for this request."
	}
	return "Store write path is enabled for " + writeSource + "; this bundle reflects the targets written or attempted by the turn handler."
}

func inputTransparencyNote(storeWriteEnabled bool, writeSource string) string {
	if !storeWriteEnabled {
		return "R1 read-shadow input transparency: no Store writes are enabled for this request."
	}
	return "Store write path is enabled for " + writeSource + "; input transparency persistence was attempted by the handler."
}

// buildRepairReplayPlan assembles the JS-adapter-consumable repair_replay_plan bundle.
func buildRepairReplayPlan(sid string, req dto.ChatLogRepairReplayRequest, mutationEnabled bool, source string) map[string]any {
	entries := []map[string]any{}
	for i, e := range req.Entries {
		if i >= 3 {
			break
		}
		preview := ""
		if e.AssistantContent != nil {
			preview = truncateRunes(strings.TrimSpace(*e.AssistantContent), 120)
		}
		entries = append(entries, map[string]any{
			"index":   i,
			"preview": preview,
		})
	}
	status := "shadow_plan"
	notes := "R1 read-shadow repair-replay plan: no replay or write triggered."
	wouldReplay := false
	wouldWrite := false
	if mutationEnabled {
		status = "mutation_ready"
		notes = "Store write path is enabled; repair-replay checks missing raw chat_log roles and inserts only missing rows."
		wouldReplay = len(req.Entries) > 0
		wouldWrite = wouldReplay && !(req.DryRun != nil && *req.DryRun)
		if strings.TrimSpace(source) == "" {
			source = "store_write"
		}
	} else {
		source = "go_r1_read_shadow"
	}
	return map[string]any{
		"status":                  status,
		"source":                  source,
		"chat_session_id":         sid,
		"entries_count":           len(req.Entries),
		"dry_run":                 req.DryRun != nil && *req.DryRun,
		"would_replay":            wouldReplay,
		"would_write":             wouldWrite,
		"mutation_enabled":        mutationEnabled,
		"sync_replay_gate":        true,
		"save_update_delete_gate": true,
		"write_scope":             "chat_log_effective_input_memory_evidence_kg",
		"delete_scope":            "rollback_delete_gate_only",
		"canonical_input_source":  "sqlite_store",
		"entries_preview":         entries,
		"notes":                   notes,
	}
}

func (s *Server) runChatLogRepairReplay(ctx context.Context, sid string, req dto.ChatLogRepairReplayRequest) (map[string]any, error) {
	dryRun := req.DryRun != nil && *req.DryRun
	now := time.Now().UTC()
	repairedTurns := []int{}
	failedTurns := []map[string]any{}
	checkedTurns := []int{}
	totalMissingRoles := 0
	totalRepairedRoles := 0
	totalConflictRoles := 0
	totalExistingRoles := 0

	for _, entry := range req.Entries {
		turnIndex := entry.TurnIndex
		if turnIndex < 0 {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "reason": "invalid_turn_index"})
			continue
		}
		checkedTurns = append(checkedTurns, turnIndex)
		existingRows, err := s.Store.ListChatLogs(ctx, sid, turnIndex, turnIndex)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "reason": "list_chat_logs_failed: " + err.Error()})
			continue
		}
		existing := map[string]string{}
		for _, row := range existingRows {
			if row.ChatSessionID != sid || row.TurnIndex != turnIndex {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(row.Role))
			if role == "user" || role == "assistant" {
				existing[role] = row.Content
			}
		}

		createdAt := parseRepairReplayCreatedAt(entry.CreatedAt, now)
		repairedThisTurn := 0
		for _, candidate := range []struct {
			role    string
			content *string
		}{
			{role: "user", content: entry.UserContent},
			{role: "assistant", content: entry.AssistantContent},
		} {
			content := ""
			if candidate.content != nil {
				content = sanitizeCriticStorageText(*candidate.content)
			}
			if strings.TrimSpace(content) == "" {
				continue
			}
			if current, ok := existing[candidate.role]; ok {
				if strings.TrimSpace(current) != strings.TrimSpace(content) {
					totalConflictRoles++
				} else {
					totalExistingRoles++
				}
				continue
			}
			totalMissingRoles++
			if dryRun {
				continue
			}
			if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
				ChatSessionID: sid,
				TurnIndex:     turnIndex,
				Role:          candidate.role,
				Content:       content,
				CreatedAt:     createdAt,
			}); err != nil {
				failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "role": candidate.role, "reason": "save_chat_log_failed: " + err.Error()})
				continue
			}
			totalRepairedRoles++
			repairedThisTurn++
			existing[candidate.role] = content
		}
		if repairedThisTurn > 0 {
			repairedTurns = append(repairedTurns, turnIndex)
		}
	}

	if !dryRun && totalRepairedRoles > 0 {
		_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "repair_replay",
			TargetType:    "session",
			TargetID:      0,
			Summary:       fmt.Sprintf("Repair replay restored %d chat log roles", totalRepairedRoles),
			DetailsJSON: mustCompactJSON(map[string]any{
				"repaired_turns":            repairedTurns,
				"total_repaired_role_count": totalRepairedRoles,
				"total_conflict_role_count": totalConflictRoles,
			}),
			Source:    s.storeWriteSource(),
			CreatedAt: now,
		})
	}

	return map[string]any{
		"status":                    "ok",
		"source":                    s.storeWriteSource(),
		"chat_session_id":           sid,
		"dry_run":                   dryRun,
		"entries_count":             len(req.Entries),
		"checked_turns":             uniqueSortedInts(checkedTurns),
		"repaired_turns":            uniqueSortedInts(repairedTurns),
		"failed_turns":              failedTurns,
		"total_missing_role_count":  totalMissingRoles,
		"total_repaired_role_count": totalRepairedRoles,
		"total_conflict_role_count": totalConflictRoles,
		"total_existing_role_count": totalExistingRoles,
		"note":                      "repair-replay checked supplied failed-queue/delete-snapshot/active-chat entries and inserted only missing raw chat_log roles",
	}, nil
}

func parseRepairReplayCreatedAt(raw *string, fallback time.Time) time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return fallback
	}
	text := strings.TrimSpace(*raw)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC()
		}
	}
	return fallback
}

func uniqueSortedInts(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// buildRollbackPlan assembles the JS-adapter-consumable rollback_plan bundle.
func buildRollbackPlan(sid string, turnIndex int, reqSource string) map[string]any {
	return map[string]any{
		"status":                      "shadow_plan",
		"source":                      "go_r1_read_shadow",
		"chat_session_id":             sid,
		"turn_index":                  turnIndex,
		"req_source":                  reqSource,
		"would_delete":                false,
		"would_write":                 false,
		"mutation_enabled":            false,
		"reason":                      "R1 shadow mode: rollback not executed",
		"sync_replay_gate":            true,
		"save_update_delete_gate":     true,
		"stale_vector_replay_gate":    true,
		"rollback_vector_delete_gate": true,
		"rebuild_replay_gate":         false,
		"vector_doc_delete_policy":    "canonical_row_first_then_vector",
		"stale_summary_policy":        "tombstone_before_rebuild",
		"turn_delete_policy":          "tail_from_earliest_deleted_turn",
		"hierarchy_invalidation":      "delete_overlapping_episode_chapter_arc_saga_ranges",
		"step23_invalidation":         "delete_turn_scoped_support_records_from_from_turn",
		"rebuild_owner":               "chroma_shadow_orchestrator",
		"cleanup_surfaces": []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"subjective_entity_memories",
			"direct_evidence",
			"kg_triples",
			"critic_feedback",
			"character_events",
			"entities",
			"trust_states",
			"storylines",
			"world_rules",
			"character_states",
			"pending_threads",
			"active_states",
			"canonical_state_layers",
			"episode_summaries",
			"guidance_plan_states",
			"chapter_summaries",
			"arc_summaries",
			"saga_digests",
			"session_active_scopes",
			"consequence_records",
			"psychology_branches",
			"theme_offscreen_carries",
			"capture_verification_records",
		},
		"notes": "R1 read-shadow rollback plan: no deletion or write triggered.",
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstMapOrNil(items []map[string]any) any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func q1FirstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func q1TimePtrAny(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func chapterFromResumePack(rp *store.ResumePack) *store.ChapterSummary {
	if rp == nil {
		return nil
	}
	return rp.Chapter
}

func arcFromResumePack(rp *store.ResumePack) *store.ArcSummary {
	if rp == nil {
		return nil
	}
	return rp.Arc
}

func sagaFromResumePack(rp *store.ResumePack) *store.SagaDigest {
	if rp == nil {
		return nil
	}
	return rp.Saga
}
