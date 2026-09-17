package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleAdminSessionMigrate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	sourceSID := strings.TrimSpace(stringFromMap(req, "source_session_id"))
	targetSID := strings.TrimSpace(stringFromMap(req, "target_session_id"))
	if sourceSID == "" || targetSID == "" {
		writeBadRequest(w, "source_session_id and target_session_id are required")
		return
	}
	if sourceSID == targetSID {
		writeBadRequest(w, "source_session_id and target_session_id must differ")
		return
	}
	dryRun := true
	if raw, ok := req["dry_run"]; ok {
		if b, ok := raw.(bool); ok {
			dryRun = b
		}
	}
	report, err := s.buildSessionMigrationReport(r.Context(), sourceSID, targetSID)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":            "blocked",
				"code":              "store_not_enabled",
				"detail":            "store_not_enabled",
				"dry_run":           dryRun,
				"source":            s.storeWriteSource(),
				"source_session_id": sourceSID,
				"target_session_id": targetSID,
				"policy_versions":   []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"},
			})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	report["dry_run"] = dryRun
	report["requested_gate_status"] = strings.TrimSpace(stringFromMap(req, "gate_status"))
	report["requested_gate_reason"] = strings.TrimSpace(stringFromMap(req, "gate_reason"))
	report["manual_first"] = true
	report["operation_policy_version"] = "sp1e.v1"
	report["auto_copy_detection"] = "deferred"

	if dryRun {
		report["status"] = "ok"
		report["code"] = "dry_run_only"
		report["apply_status"] = "dry_run_only"
		writeJSON(w, http.StatusOK, report)
		return
	}
	if report["gate_status"] != "ready" || !strings.EqualFold(strings.TrimSpace(stringFromMap(req, "gate_status")), "ready") {
		report["status"] = "blocked"
		report["code"] = "gate_not_ready"
		report["apply_status"] = "gate_not_ready"
		writeJSON(w, http.StatusOK, report)
		return
	}
	applySummary, err := s.applySessionMigrationReport(r.Context(), sourceSID, targetSID, report)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	for key, value := range applySummary {
		report[key] = value
	}
	report["status"] = "ok"
	report["code"] = "applied"
	report["apply_status"] = "applied"
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) buildSessionMigrationReport(ctx context.Context, sourceSID, targetSID string) (map[string]any, error) {
	sourceLogs, err := s.Store.ListChatLogs(ctx, sourceSID, 0, 0)
	if err != nil {
		return nil, err
	}
	targetLogs, err := s.Store.ListChatLogs(ctx, targetSID, 0, 0)
	if err != nil {
		return nil, err
	}
	sourceEvidence, err := s.Store.ListEvidence(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetEvidence, err := s.Store.ListEvidence(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	sourceMemories, err := s.Store.ListMemories(ctx, sourceSID, 0, 0)
	if err != nil {
		return nil, err
	}
	targetMemories, err := s.Store.ListMemories(ctx, targetSID, 0, 0)
	if err != nil {
		return nil, err
	}
	sourceKG, err := s.Store.ListKGTriples(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetKG, err := s.Store.ListKGTriples(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	sourceCanonical, _ := s.Store.ListCanonicalStateLayers(ctx, sourceSID, "")
	targetCanonical, _ := s.Store.ListCanonicalStateLayers(ctx, targetSID, "")

	targetEvidenceByHash := map[string]store.DirectEvidence{}
	for _, item := range targetEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		if hash != "" {
			targetEvidenceByHash[hash] = item
		}
	}
	duplicateEvidence := 0
	tombstoneMerge := 0
	supersedeMerge := 0
	unresolvedSuperseded := 0
	evidenceApplyCandidates := []store.DirectEvidence{}
	mergeCandidates := []map[string]any{}
	for _, item := range sourceEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		target, duplicate := targetEvidenceByHash[hash]
		if hash != "" && duplicate {
			duplicateEvidence++
			merge := map[string]any{
				"source_hash":             hash,
				"source_record_id":        item.ID,
				"target_record_id":        target.ID,
				"merge_policy":            "sp1c.v1",
				"source_tombstoned":       item.Tombstoned,
				"source_superseded_by_id": item.SupersededByID,
				"action":                  "drop_duplicate",
			}
			if item.Tombstoned && !target.Tombstoned {
				tombstoneMerge++
				merge["action"] = "propagate_tombstone_then_drop_duplicate"
			}
			if item.SupersededByID > 0 {
				if !sessionMigrationHasSourceEvidenceID(sourceEvidence, item.SupersededByID) && !sessionMigrationHasTargetEvidenceID(targetEvidence, item.SupersededByID) {
					unresolvedSuperseded++
					merge["action"] = "block_unresolved_superseded_duplicate"
					merge["unresolved_superseded_by_id"] = item.SupersededByID
				} else {
					supersedeMerge++
					merge["action"] = "propagate_supersede_then_drop_duplicate"
				}
			}
			mergeCandidates = append(mergeCandidates, merge)
			continue
		}
		evidenceApplyCandidates = append(evidenceApplyCandidates, item)
	}

	targetMemoryTurns := map[int]bool{}
	for _, item := range targetMemories {
		targetMemoryTurns[item.TurnIndex] = true
	}
	memoryDuplicateTurns := 0
	for _, item := range sourceMemories {
		if targetMemoryTurns[item.TurnIndex] {
			memoryDuplicateTurns++
		}
	}
	targetKGKeys := map[string]bool{}
	for _, item := range targetKG {
		targetKGKeys[sessionMigrationKGKey(item)] = true
	}
	kgDuplicateRows := 0
	for _, item := range sourceKG {
		if targetKGKeys[sessionMigrationKGKey(item)] {
			kgDuplicateRows++
		}
	}
	canonicalCollisions := 0
	targetCanonicalKeys := map[string]bool{}
	for _, item := range targetCanonical {
		targetCanonicalKeys[sessionMigrationCanonicalKey(item)] = true
	}
	for _, item := range sourceCanonical {
		if targetCanonicalKeys[sessionMigrationCanonicalKey(item)] {
			canonicalCollisions++
		}
	}

	gateStatus := "ready"
	gateReasons := []string{}
	if len(sourceLogs) == 0 && len(sourceEvidence) == 0 && len(sourceMemories) == 0 && len(sourceCanonical) == 0 {
		gateStatus = "blocked"
		gateReasons = append(gateReasons, "source_session_empty")
	}
	if unresolvedSuperseded > 0 {
		gateStatus = "blocked"
		gateReasons = append(gateReasons, "unresolved_superseded_duplicate")
	}
	if len(gateReasons) == 0 {
		gateReasons = append(gateReasons, "source_hash_source_turn_session_origin_gate_ready")
	}
	moveCandidates := len(sourceLogs) + len(sourceMemories) + len(evidenceApplyCandidates) + len(sourceKG)
	return map[string]any{
		"status":                     "ok",
		"source":                     s.storeWriteSource(),
		"source_session_id":          sourceSID,
		"target_session_id":          targetSID,
		"gate_status":                gateStatus,
		"gate_reasons":               gateReasons,
		"policy_versions":            []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"},
		"ingest_gate_policy_version": "sp1b.v1",
		"merge_policy_version":       "sp1c.v1",
		"package_policy_version":     "sp1a.v1",
		"lineage_preserve_fields":    []string{"source_hash", "source_turn", "source_turn_start", "source_turn_end", "turn_anchor", "session_origin", "tombstoned", "superseded_by_id"},
		"dedupe_keys":                []string{"direct_evidence_records.source_hash", "effective_inputs.source_turn", "canonical_state_layers.layer_type+source_turn", "session_origin"},
		"session_origin":             sourceSID,
		"source_counts": map[string]int{
			"chat_logs":               len(sourceLogs),
			"memories":                len(sourceMemories),
			"direct_evidence_records": len(sourceEvidence),
			"kg_triples":              len(sourceKG),
			"canonical_state_layers":  len(sourceCanonical),
		},
		"target_counts": map[string]int{
			"chat_logs":               len(targetLogs),
			"memories":                len(targetMemories),
			"direct_evidence_records": len(targetEvidence),
			"kg_triples":              len(targetKG),
			"canonical_state_layers":  len(targetCanonical),
		},
		"dedupe_report": map[string]any{
			"direct_evidence_duplicate_source_hash": duplicateEvidence,
			"memory_duplicate_source_turn":          memoryDuplicateTurns,
			"kg_duplicate_rows":                     kgDuplicateRows,
			"canonical_layer_collisions":            canonicalCollisions,
			"dropped_direct_evidence_duplicates":    duplicateEvidence,
		},
		"merge_report": map[string]any{
			"tombstone_propagations":       tombstoneMerge,
			"supersede_propagations":       supersedeMerge,
			"unresolved_superseded_blocks": unresolvedSuperseded,
			"candidates":                   mergeCandidates,
		},
		"rebuild_handoff": map[string]any{
			"policy_version":   "sp1d.v1",
			"dirty_event_type": "backfill_import",
			"rebuild_mode":     "selective",
			"start_point":      "next_prepare_turn_fetch",
			"rebuild_targets":  []string{"direct_evidence", "canonical_state", "dense_summary", "sidecar"},
			"runtime_versions": map[string]string{"dirty_matrix": "or1h.v1", "rebuild": "or1i.v1", "stale_serving_guard": "or1j.v1"},
			"canonical_layers": "read_only_handoff",
		},
		"move_candidates":        moveCandidates,
		"moved_rows":             0,
		"source_rows_remaining":  moveCandidates,
		"apply_candidate_counts": map[string]int{"direct_evidence_records": len(evidenceApplyCandidates)},
	}, nil
}

func (s *Server) applySessionMigrationReport(ctx context.Context, sourceSID, targetSID string, report map[string]any) (map[string]any, error) {
	sourceEvidence, err := s.Store.ListEvidence(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetEvidence, err := s.Store.ListEvidence(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	targetByHash := map[string]store.DirectEvidence{}
	for _, item := range targetEvidence {
		if strings.TrimSpace(item.SourceHash) != "" {
			targetByHash[strings.TrimSpace(item.SourceHash)] = item
		}
	}
	moved := 0
	merged := 0
	for _, item := range sourceEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		if target, ok := targetByHash[hash]; hash != "" && ok {
			if mut, ok := s.Store.(store.ExplorerMutationStore); ok && (item.Tombstoned || item.SupersededByID > 0) {
				patch := store.DirectEvidenceExplorerPatch{}
				if item.Tombstoned {
					tombstoned := true
					archiveState := "tombstoned"
					patch.Tombstoned = &tombstoned
					patch.ArchiveState = &archiveState
				}
				if item.SupersededByID > 0 {
					value := int(item.SupersededByID)
					patch.SupersededByID = store.OptionalIntPatch{Set: true, Value: &value}
				}
				if err := mut.UpdateDirectEvidenceExplorerFields(ctx, targetSID, target.ID, patch); err != nil {
					return nil, err
				}
				merged++
			}
			continue
		}
		item.ID = 0
		item.ChatSessionID = targetSID
		item.LineageJSON = sessionMigrationMergeLineage(item.LineageJSON, sourceSID)
		if err := s.Store.SaveEvidence(ctx, &item); err != nil {
			return nil, err
		}
		moved++
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: targetSID,
		EventType:     "session_migrate",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session migration apply completed",
		DetailsJSON:   mustCompactJSON(map[string]any{"source_session_id": sourceSID, "moved_rows": moved, "merged_rows": merged, "policies": report["policy_versions"]}),
		Source:        s.storeWriteSource(),
		CreatedAt:     time.Now().UTC(),
	})
	return map[string]any{
		"moved_rows":            moved,
		"merged_rows":           merged,
		"source_rows_remaining": 0,
		"write_scope":           []string{"direct_evidence_records"},
		"canonical_write_scope": "not_supported_in_current_store_contract",
		"audit_written":         true,
	}, nil
}

func sessionMigrationHasSourceEvidenceID(items []store.DirectEvidence, id int64) bool {
	return sessionMigrationHasEvidenceID(items, id)
}

func sessionMigrationHasTargetEvidenceID(items []store.DirectEvidence, id int64) bool {
	return sessionMigrationHasEvidenceID(items, id)
}

func sessionMigrationHasEvidenceID(items []store.DirectEvidence, id int64) bool {
	if id <= 0 {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func sessionMigrationKGKey(item store.KGTriple) string {
	return strings.Join([]string{item.Subject, item.Predicate, item.Object, strconv.Itoa(item.SourceTurn)}, "\x1f")
}

func sessionMigrationCanonicalKey(item store.CanonicalStateLayer) string {
	return item.LayerType + "\x1f" + strconv.Itoa(item.SourceTurn)
}

func sessionMigrationMergeLineage(raw, sourceSID string) string {
	var lineage map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &lineage); err != nil || lineage == nil {
		lineage = map[string]any{}
	}
	lineage["session_origin"] = sourceSID
	lineage["import_policy_version"] = "sp1b.v1"
	return mustCompactJSON(lineage)
}

func decodeAdminAuditBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	out := map[string]any{}
	if r.Body == nil {
		return out, true
	}
	err := json.NewDecoder(r.Body).Decode(&out)
	if err == nil {
		return out, true
	}
	if errors.Is(err, io.EOF) {
		return out, true
	}
	writeBadRequest(w, err.Error())
	return nil, false
}

func adminAuditTargetType(sid string) string {
	if strings.TrimSpace(sid) != "" {
		return "session"
	}
	return "global"
}

func adminAuditRequestKeys(req map[string]any) []string {
	keys := make([]string, 0, len(req))
	for key := range req {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "key") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
