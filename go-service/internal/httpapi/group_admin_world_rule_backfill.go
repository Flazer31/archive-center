package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) backfillWorldRulesFromMemories(ctx context.Context, sid string, memories []store.Memory, targetTurns map[int]bool, dryRun bool) map[string]any {
	result := map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		result["reason"] = "world_rule_store_not_available"
		return result
	}
	if memories == nil {
		listed, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		memories = listed
	}
	existingRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = err.Error()
		return result
	}
	seen := map[string]bool{}
	for _, rule := range existingRules {
		seen[worldRuleDedupeSignature(rule.Scope, rule.ScopeName, rule.Key)] = true
	}
	now := time.Now().UTC()
	candidates := 0
	generated := 0
	existing := 0
	skipped := 0
	for _, mem := range memories {
		if mem.ChatSessionID != sid || mem.TurnIndex < 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[mem.TurnIndex] {
			continue
		}
		extraction := map[string]any{}
		if err := json.Unmarshal([]byte(mem.SummaryJSON), &extraction); err != nil {
			skipped++
			continue
		}
		for _, raw := range worldRuleItemsForSave(extraction) {
			ruleMap := mapFromAny(raw)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(ruleMap, "key"), stringFromMap(ruleMap, "name")))
			if key == "" {
				skipped++
				continue
			}
			scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(ruleMap, "scope"), "session"))
			scopeName := stringFromMap(ruleMap, "scope_name")
			sig := worldRuleDedupeSignature(scope, scopeName, key)
			candidates++
			if seen[sig] {
				existing++
				continue
			}
			seen[sig] = true
			if dryRun {
				continue
			}
			err := saver.SaveWorldRule(ctx, &store.WorldRule{
				ChatSessionID: sid,
				Scope:         scope,
				ScopeName:     scopeName,
				Category:      extractionFirstNonEmpty(stringFromMap(ruleMap, "category"), "critic_backfill"),
				Key:           key,
				ValueJSON:     normalizeWorldRuleValueJSON(extractionFirstNonEmpty(stringFromMap(ruleMap, "value"), stringFromMap(ruleMap, "value_json"), mustCompactJSON(ruleMap))),
				Genre:         stringFromMap(ruleMap, "genre"),
				SourceTurn:    mem.TurnIndex,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
			if err != nil {
				result["status"] = "partial_error"
				result["error"] = err.Error()
				return result
			}
			generated++
		}
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existing
	result["skipped"] = skipped
	return result
}

func (s *Server) backfillWorldRulesFromChatLogs(ctx context.Context, sid string, logs []store.ChatLog, targetTurns map[int]bool, dryRun bool, cfg completeTurnLLMConfig, progress adminJobProgressFunc) map[string]any {
	result := map[string]any{
		"status":     "skipped",
		"source":     "raw_chat_logs_world_rule_audit",
		"dry_run":    dryRun,
		"candidate":  0,
		"generated":  0,
		"existing":   0,
		"skipped":    0,
		"audit_runs": 0,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	if dryRun {
		result["reason"] = "dry_run_no_llm_call"
		return result
	}
	if !cfg.hasConfig() {
		result["reason"] = "critic_config_missing"
		return result
	}
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		result["reason"] = "world_rule_store_not_available"
		return result
	}
	if logs == nil {
		listed, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		logs = listed
	}
	chunks := buildWorldRuleAuditChatLogChunks(sid, logs, targetTurns)
	if len(chunks) == 0 {
		result["reason"] = "no_chat_log_chunks"
		return result
	}
	result["chunk_count"] = len(chunks)
	existingRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = err.Error()
		return result
	}
	seen := map[string]bool{}
	for _, rule := range existingRules {
		seen[worldRuleDedupeSignature(rule.Scope, rule.ScopeName, rule.Key)] = true
	}
	now := time.Now().UTC()
	generated := 0
	existing := 0
	skipped := 0
	candidates := 0
	auditRuns := 0
	traces := []map[string]any{}
	if progress != nil {
		progress(map[string]any{
			"status":           "running",
			"stage":            "raw_world_rule_backfill",
			"phase":            "chunk_scan_start",
			"candidate_count":  len(chunks),
			"processed":        0,
			"generated":        0,
			"existing":         0,
			"skipped_count":    0,
			"progress_percent": 0,
		})
	}
	for idx, chunk := range chunks {
		select {
		case <-ctx.Done():
			result["status"] = "partial_error"
			result["error"] = ctx.Err().Error()
			if progress != nil {
				progress(map[string]any{
					"status":           "error",
					"stage":            "raw_world_rule_backfill",
					"phase":            "canceled",
					"candidate_count":  len(chunks),
					"processed":        auditRuns,
					"generated":        generated,
					"existing":         existing,
					"skipped_count":    skipped,
					"progress_percent": adminJobProgressPercent(auditRuns, len(chunks)),
					"error":            ctx.Err().Error(),
				})
			}
			return result
		default:
		}
		auditRuns++
		if progress != nil {
			progress(map[string]any{
				"status":           "running",
				"stage":            "raw_world_rule_backfill",
				"phase":            "llm_call_start",
				"candidate_count":  len(chunks),
				"processed":        idx,
				"chunk_index":      idx + 1,
				"chunk_count":      len(chunks),
				"start_turn":       chunk.startTurn,
				"end_turn":         chunk.endTurn,
				"generated":        generated,
				"existing":         existing,
				"skipped_count":    skipped,
				"progress_percent": adminJobProgressPercent(idx, len(chunks)),
			})
		}
		audited, trace := s.runCompleteTurnWorldRuleAudit(
			ctx,
			sid,
			chunk.endTurn,
			"Session-level world rule audit from raw chat logs. Extract only durable rules grounded in the transcript.",
			chunk.text,
			nil,
			nil,
			map[string]any{},
			cfg,
		)
		traces = append(traces, map[string]any{
			"start_turn": chunk.startTurn,
			"end_turn":   chunk.endTurn,
			"trace":      trace,
		})
		if progress != nil {
			progress(map[string]any{
				"status":           "running",
				"stage":            "raw_world_rule_backfill",
				"phase":            "llm_response_received",
				"candidate_count":  len(chunks),
				"processed":        idx,
				"chunk_index":      idx + 1,
				"chunk_count":      len(chunks),
				"start_turn":       chunk.startTurn,
				"end_turn":         chunk.endTurn,
				"generated":        generated,
				"existing":         existing,
				"skipped_count":    skipped,
				"last_trace":       trace,
				"progress_percent": adminJobProgressPercent(idx, len(chunks)),
			})
		}
		if stringFromMap(trace, "status") == "error" {
			skipped++
			if progress != nil {
				progress(map[string]any{
					"status":           "running",
					"stage":            "raw_world_rule_backfill",
					"phase":            "llm_call_error",
					"candidate_count":  len(chunks),
					"processed":        idx + 1,
					"chunk_index":      idx + 1,
					"chunk_count":      len(chunks),
					"start_turn":       chunk.startTurn,
					"end_turn":         chunk.endTurn,
					"generated":        generated,
					"existing":         existing,
					"skipped_count":    skipped,
					"last_trace":       trace,
					"progress_percent": adminJobProgressPercent(idx+1, len(chunks)),
				})
			}
			continue
		}
		if progress != nil {
			progress(map[string]any{
				"status":           "running",
				"stage":            "raw_world_rule_backfill",
				"phase":            "parse_start",
				"candidate_count":  len(chunks),
				"processed":        idx,
				"chunk_index":      idx + 1,
				"chunk_count":      len(chunks),
				"start_turn":       chunk.startTurn,
				"end_turn":         chunk.endTurn,
				"progress_percent": adminJobProgressPercent(idx, len(chunks)),
			})
		}
		rawRules := worldRuleItemsForSave(audited)
		if progress != nil {
			progress(map[string]any{
				"status":                "running",
				"stage":                 "raw_world_rule_backfill",
				"phase":                 "parse_done",
				"candidate_count":       len(chunks),
				"processed":             idx,
				"chunk_index":           idx + 1,
				"chunk_count":           len(chunks),
				"start_turn":            chunk.startTurn,
				"end_turn":              chunk.endTurn,
				"chunk_rule_candidates": len(rawRules),
				"progress_percent":      adminJobProgressPercent(idx, len(chunks)),
			})
		}
		chunkGeneratedBefore := generated
		chunkExistingBefore := existing
		chunkSkippedBefore := skipped
		for _, raw := range rawRules {
			ruleMap := mapFromAny(raw)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(ruleMap, "key"), stringFromMap(ruleMap, "name")))
			if key == "" {
				skipped++
				continue
			}
			scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(ruleMap, "scope"), "session"))
			scopeName := stringFromMap(ruleMap, "scope_name")
			sig := worldRuleDedupeSignature(scope, scopeName, key)
			candidates++
			if seen[sig] {
				existing++
				continue
			}
			seen[sig] = true
			if progress != nil {
				progress(map[string]any{
					"status":           "running",
					"stage":            "raw_world_rule_backfill",
					"phase":            "save_start",
					"candidate_count":  len(chunks),
					"processed":        idx,
					"chunk_index":      idx + 1,
					"chunk_count":      len(chunks),
					"start_turn":       chunk.startTurn,
					"end_turn":         chunk.endTurn,
					"rule_key":         key,
					"rule_scope":       scope,
					"rule_scope_name":  scopeName,
					"generated":        generated,
					"existing":         existing,
					"skipped_count":    skipped,
					"progress_percent": adminJobProgressPercent(idx, len(chunks)),
				})
			}
			err := saver.SaveWorldRule(ctx, &store.WorldRule{
				ChatSessionID: sid,
				Scope:         scope,
				ScopeName:     scopeName,
				Category:      extractionFirstNonEmpty(stringFromMap(ruleMap, "category"), "raw_chat_audit"),
				Key:           key,
				ValueJSON:     normalizeWorldRuleValueJSON(extractionFirstNonEmpty(stringFromMap(ruleMap, "value"), stringFromMap(ruleMap, "value_json"), mustCompactJSON(ruleMap))),
				Genre:         stringFromMap(ruleMap, "genre"),
				SourceTurn:    chunk.endTurn,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
			if err != nil {
				result["status"] = "partial_error"
				result["error"] = err.Error()
				result["candidate"] = candidates
				result["generated"] = generated
				result["existing"] = existing
				result["skipped"] = skipped
				result["audit_runs"] = auditRuns
				result["audit_trace"] = traces
				if progress != nil {
					progress(map[string]any{
						"status":           "error",
						"stage":            "raw_world_rule_backfill",
						"phase":            "save_error",
						"candidate_count":  len(chunks),
						"processed":        idx + 1,
						"chunk_index":      idx + 1,
						"chunk_count":      len(chunks),
						"start_turn":       chunk.startTurn,
						"end_turn":         chunk.endTurn,
						"generated":        generated,
						"existing":         existing,
						"skipped_count":    skipped,
						"error":            err.Error(),
						"progress_percent": adminJobProgressPercent(idx+1, len(chunks)),
					})
				}
				return result
			}
			generated++
			if progress != nil {
				progress(map[string]any{
					"status":           "running",
					"stage":            "raw_world_rule_backfill",
					"phase":            "save_done",
					"candidate_count":  len(chunks),
					"processed":        idx,
					"chunk_index":      idx + 1,
					"chunk_count":      len(chunks),
					"start_turn":       chunk.startTurn,
					"end_turn":         chunk.endTurn,
					"rule_key":         key,
					"rule_scope":       scope,
					"rule_scope_name":  scopeName,
					"generated":        generated,
					"existing":         existing,
					"skipped_count":    skipped,
					"progress_percent": adminJobProgressPercent(idx, len(chunks)),
				})
			}
		}
		if progress != nil {
			progress(map[string]any{
				"status":           "running",
				"stage":            "raw_world_rule_backfill",
				"phase":            "chunk_done",
				"candidate_count":  len(chunks),
				"processed":        idx + 1,
				"chunk_index":      idx + 1,
				"chunk_count":      len(chunks),
				"start_turn":       chunk.startTurn,
				"end_turn":         chunk.endTurn,
				"generated":        generated,
				"existing":         existing,
				"skipped_count":    skipped,
				"chunk_generated":  generated - chunkGeneratedBefore,
				"chunk_existing":   existing - chunkExistingBefore,
				"chunk_skipped":    skipped - chunkSkippedBefore,
				"last_trace":       trace,
				"progress_percent": adminJobProgressPercent(idx+1, len(chunks)),
			})
		}
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existing
	result["skipped"] = skipped
	result["audit_runs"] = auditRuns
	result["audit_trace"] = traces
	if candidates == 0 {
		result["reason"] = "raw_audit_returned_no_world_rules"
	}
	return result
}

type worldRuleAuditChatLogChunk struct {
	startTurn int
	endTurn   int
	text      string
}

func buildWorldRuleAuditChatLogChunks(sid string, logs []store.ChatLog, targetTurns map[int]bool) []worldRuleAuditChatLogChunk {
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
	turns := make([]int, 0, len(turnLogs))
	for turn := range turnLogs {
		turns = append(turns, turn)
	}
	sort.Ints(turns)
	chunks := []worldRuleAuditChatLogChunk{}
	var b strings.Builder
	startTurn := 0
	endTurn := 0
	flush := func() {
		text := strings.TrimSpace(b.String())
		if text == "" || startTurn <= 0 || endTurn <= 0 {
			b.Reset()
			startTurn = 0
			endTurn = 0
			return
		}
		chunks = append(chunks, worldRuleAuditChatLogChunk{startTurn: startTurn, endTurn: endTurn, text: text})
		b.Reset()
		startTurn = 0
		endTurn = 0
	}
	for _, turn := range turns {
		roleMap := turnLogs[turn]
		userText := strings.TrimSpace(roleMap["user"])
		assistantText := strings.TrimSpace(roleMap["assistant"])
		if userText == "" && assistantText == "" {
			continue
		}
		var tb strings.Builder
		fmt.Fprintf(&tb, "\n[turn %d]\n", turn)
		if userText != "" {
			tb.WriteString("user: ")
			tb.WriteString(truncateRunes(userText, 1600))
			tb.WriteString("\n")
		}
		if assistantText != "" {
			tb.WriteString("assistant: ")
			tb.WriteString(truncateRunes(assistantText, 3200))
			tb.WriteString("\n")
		}
		turnBlock := tb.String()
		if b.Len() > 0 && (b.Len()+len(turnBlock) > 14000 || endTurn-startTurn >= 11) {
			flush()
		}
		if startTurn == 0 {
			startTurn = turn
		}
		endTurn = turn
		b.WriteString(turnBlock)
	}
	flush()
	return chunks
}

func mergeWorldRuleBackfillResults(primary, raw map[string]any) map[string]any {
	if primary == nil {
		primary = map[string]any{}
	}
	if raw == nil {
		return primary
	}
	out := map[string]any{}
	for k, v := range primary {
		out[k] = v
	}
	out["raw_chat_audit"] = raw
	out["candidate"] = intFromAny(primary["candidate"], 0) + intFromAny(raw["candidate"], 0)
	out["generated"] = intFromAny(primary["generated"], 0) + intFromAny(raw["generated"], 0)
	out["existing"] = intFromAny(primary["existing"], 0) + intFromAny(raw["existing"], 0)
	out["skipped"] = intFromAny(primary["skipped"], 0) + intFromAny(raw["skipped"], 0)
	if intFromAny(raw["generated"], 0) > 0 || intFromAny(raw["candidate"], 0) > 0 {
		out["status"] = raw["status"]
		delete(out, "reason")
	}
	return out
}

func worldRuleDedupeSignature(scope, scopeName, key string) string {
	return strings.ToLower(store.NormalizeWorldRuleScope(scope) + "\x00" + strings.TrimSpace(scopeName) + "\x00" + strings.TrimSpace(key))
}

const (
	maintenanceTM1dPolicyVersion      = "tm1d.v1"
	maintenanceTM1dDirtyMatrixVersion = "or1h.tm1d.v1"
)

func buildTM1dAuditReplayContract(eventType string, passState map[string]any, importanceReweighting map[string]any, turnIndex int) map[string]any {
	var rows []map[string]any
	switch eventType {
	case "drift_detected":
		rows = buildTM1dDriftDirtyMatrixRows(passState, turnIndex)
	case "importance_reevaluation":
		rows = buildTM1dImportanceDirtyMatrixRows(importanceReweighting, turnIndex)
	}
	replay := buildTM1dReplayMeasurements(eventType, passState, importanceReweighting, len(rows))
	return map[string]any{
		"or_phase_dirty_matrix": map[string]any{
			"policy_version": maintenanceTM1dPolicyVersion,
			"matrix_version": maintenanceTM1dDirtyMatrixVersion,
			"event_type":     eventType,
			"row_count":      len(rows),
			"rows":           rows,
		},
		"replay_measurements": replay,
	}
}

func buildTM1dDriftDirtyMatrixRows(passState map[string]any, turnIndex int) []map[string]any {
	var canonicalUpdates []map[string]any
	if rawList, ok := passState["canonical_updates"].([]map[string]any); ok {
		canonicalUpdates = rawList
	} else {
		for _, raw := range asAnySlice(passState["canonical_updates"]) {
			if m, ok := raw.(map[string]any); ok {
				canonicalUpdates = append(canonicalUpdates, m)
			}
		}
	}
	driftSignalTypes := []string{}
	for _, t := range asAnySlice(passState["drift_signal_types"]) {
		if s, ok := t.(string); ok {
			driftSignalTypes = append(driftSignalTypes, s)
		}
	}
	severity := extractionStringFromAny(passState["strongest_signal_severity"])
	sourcePolicyVersion := extractionStringFromAny(passState["version"])
	if sourcePolicyVersion == "" {
		sourcePolicyVersion = "tm1b.shadow.v1"
	}

	rows := []map[string]any{}
	for i, update := range canonicalUpdates {
		if !completeTurnBoolFromAny(update["would_degrade_confidence"]) {
			continue
		}
		layerType := extractionStringFromAny(update["layer_type"])
		if layerType == "" {
			layerType = "canonical_state"
		}
		signalType := "canonical_drift"
		if i < len(driftSignalTypes) {
			signalType = driftSignalTypes[i]
		} else if len(driftSignalTypes) > 0 {
			signalType = driftSignalTypes[0]
		}
		rows = append(rows, map[string]any{
			"event_type":            "drift_detected",
			"turn_index":            turnIndex,
			"source_policy_version": sourcePolicyVersion,
			"or_phase_trigger":      "truth_maintenance_drift",
			"dirty_signal":          signalType,
			"dirty_scope":           layerType,
			"dirty_targets":         tm1dDirtyTargetsForDriftSignal(signalType),
			"replay_metric_refs":    []string{"tm_drift_pass_count", "tm_drift_layer_count", "tm_drift_signal_type_count"},
			"severity":              severity,
		})
	}
	return rows
}

func buildTM1dImportanceDirtyMatrixRows(importanceReweighting map[string]any, turnIndex int) []map[string]any {
	var updates []map[string]any
	if rawList, ok := importanceReweighting["updates"].([]map[string]any); ok {
		updates = rawList
	} else {
		for _, raw := range asAnySlice(importanceReweighting["updates"]) {
			if m, ok := raw.(map[string]any); ok {
				updates = append(updates, m)
			}
		}
	}
	sourcePolicyVersion := extractionStringFromAny(importanceReweighting["policy_version"])
	if sourcePolicyVersion == "" {
		sourcePolicyVersion = "tm1c.v1"
	}

	grouped := map[string]map[string]any{}
	for _, item := range updates {
		memoryID := int64FromMap(item, "memory_id", 0)
		oldImp := maintenanceFloatFromAnyTM1b(item["old_importance"], 0)
		nextImp := maintenanceFloatFromAnyTM1b(item["next_importance"], 0)
		delta := maintenanceTM1cRound(nextImp - oldImp)
		reasons := []string{}
		for _, r := range asAnySlice(item["reasons"]) {
			if s, ok := r.(string); ok {
				reasons = append(reasons, s)
			}
		}
		if len(reasons) == 0 {
			reasons = append(reasons, "importance_changed")
		}
		for _, reason := range reasons {
			if _, ok := grouped[reason]; !ok {
				grouped[reason] = map[string]any{
					"event_type":            "importance_reevaluation",
					"turn_index":            turnIndex,
					"source_policy_version": sourcePolicyVersion,
					"or_phase_trigger":      "truth_maintenance_importance",
					"dirty_signal":          reason,
					"dirty_scope":           "memory_importance",
					"dirty_targets":         tm1dDirtyTargetsForImportanceReason(reason),
					"replay_metric_refs":    tm1dImportanceReplayMetricRefs(reason),
					"affected_memory_ids":   []int64{},
				}
			}
			row := grouped[reason]
			ids := row["affected_memory_ids"].([]int64)
			ids = append(ids, memoryID)
			row["affected_memory_ids"] = ids
			row["importance_delta"] = delta
		}
	}

	rows := []map[string]any{}
	for _, reason := range sortedStringKeys(grouped) {
		row := grouped[reason]
		ids := row["affected_memory_ids"].([]int64)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		deduped := []int64{}
		seen := map[int64]bool{}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				deduped = append(deduped, id)
			}
		}
		row["affected_memory_ids"] = deduped
		row["affected_count"] = len(deduped)
		delta := maintenanceFloatFromAnyTM1b(row["importance_delta"], 0)
		direction := "stable"
		if delta > 0 {
			direction = "boost"
		} else if delta < 0 {
			direction = "decay"
		}
		row["delta_direction"] = direction
		delete(row, "importance_delta")
		rows = append(rows, row)
	}
	return rows
}

func sortedStringKeys(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func tm1dDirtyTargetsForDriftSignal(signalType string) []string {
	switch signalType {
	case "canonical_relationship_mismatch":
		return []string{"guidance_state", "entity_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_unresolved_thread_mismatch":
		return []string{"guidance_state", "entity_coprocessor", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_scene_archive_mismatch":
		return []string{"guidance_state", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_drift":
		return []string{"guidance_state", "canonical_state", "sidecar_cache"}
	case "memory_conflict":
		return []string{"memory_index", "guidance_state", "canonical_state"}
	case "entity_drift":
		return []string{"entity_index", "guidance_state"}
	case "narrative_drift":
		return []string{"storyline_index", "guidance_state", "director_state"}
	case "sidecar_drift":
		return []string{"sidecar_cache", "guidance_state"}
	default:
		return []string{"guidance_state", "sidecar_cache"}
	}
}

func tm1dDirtyTargetsForImportanceReason(reason string) []string {
	switch reason {
	case "recent_remention_boost":
		return []string{"guidance_state", "entity_coprocessor", "narrative_quality", "sidecar_cache"}
	case "freshness_decay":
		return []string{"guidance_state", "narrative_quality", "sidecar_cache"}
	case "resolved_reference_decay":
		return []string{"guidance_state", "entity_coprocessor", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "emotional_decay":
		return []string{"guidance_state", "narrative_quality", "sidecar_cache"}
	case "protected_reference":
		return []string{"guidance_state", "sidecar_cache"}
	default:
		return []string{"guidance_state", "sidecar_cache"}
	}
}

func tm1dImportanceReplayMetricRefs(reason string) []string {
	refs := []string{"tm_importance_pass_count", "tm_importance_updated_count"}
	switch reason {
	case "recent_remention_boost":
		refs = append(refs, "tm_importance_boosted_count")
	case "freshness_decay", "resolved_reference_decay", "emotional_decay":
		refs = append(refs, "tm_importance_decayed_count")
	case "protected_reference":
		refs = append(refs, "tm_importance_protected_count")
	}
	return uniquePreserveOrderStrings(refs)
}

func buildTM1dReplayMeasurements(eventType string, passState map[string]any, importanceReweighting map[string]any, rowCount int) map[string]any {
	switch eventType {
	case "drift_detected":
		driftSignalTypes := []string{}
		for _, t := range asAnySlice(passState["drift_signal_types"]) {
			if s, ok := t.(string); ok {
				driftSignalTypes = append(driftSignalTypes, s)
			}
		}
		return map[string]any{
			"measurement_policy_version": maintenanceTM1dPolicyVersion,
			"tm_drift_pass_count":        1,
			"tm_drift_layer_count":       rowCount,
			"tm_drift_signal_type_count": len(driftSignalTypes),
		}
	case "importance_reevaluation":
		var updates []map[string]any
		if rawList, ok := importanceReweighting["updates"].([]map[string]any); ok {
			updates = rawList
		} else {
			for _, raw := range asAnySlice(importanceReweighting["updates"]) {
				if m, ok := raw.(map[string]any); ok {
					updates = append(updates, m)
				}
			}
		}
		return map[string]any{
			"measurement_policy_version":    maintenanceTM1dPolicyVersion,
			"tm_importance_pass_count":      1,
			"tm_importance_updated_count":   intFromAny(importanceReweighting["updated_count"], len(updates)),
			"tm_importance_boosted_count":   intFromAny(importanceReweighting["boosted_count"], 0),
			"tm_importance_decayed_count":   intFromAny(importanceReweighting["decayed_count"], 0),
			"tm_importance_protected_count": intFromAny(importanceReweighting["protected_count"], 0),
		}
	default:
		return map[string]any{
			"measurement_policy_version": maintenanceTM1dPolicyVersion,
		}
	}
}

func uniquePreserveOrderStrings(values []string) []string {
	ordered := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		ordered = append(ordered, v)
	}
	return ordered
}
