package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.Store.ListSessions(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":   "ok",
				"sessions": []any{},
				"count":    0,
			})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	if sessions == nil {
		sessions = []store.SessionSummary{}
	}
	// Convert to JSON-friendly maps to match Python 0.8 snake_case keys
	out := make([]map[string]any, 0, len(sessions))
	for _, sess := range sessions {
		m := map[string]any{
			"chat_session_id":  sess.ChatSessionID,
			"chat_logs_count":  sess.ChatLogsCount,
			"memories_count":   sess.MemoriesCount,
			"kg_triples_count": sess.KGTriplesCount,
			"last_activity":    formatNaiveUTCTime(sess.LastActivity),
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"sessions": out,
		"count":    len(out),
	})
}

func (s *Server) handleSessionExport(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	ctx := r.Context()
	chatLogs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil {
		writeSessionExportError(w, err)
		return
	}

	effectiveInputs := make([]store.EffectiveInput, 0)
	seenTurns := map[int]bool{}
	for _, log := range chatLogs {
		if log.TurnIndex <= 0 || seenTurns[log.TurnIndex] {
			continue
		}
		seenTurns[log.TurnIndex] = true
		input, err := s.Store.GetEffectiveInput(ctx, sid, log.TurnIndex)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			writeSessionExportError(w, err)
			return
		}
		if input != nil {
			effectiveInputs = append(effectiveInputs, *input)
		}
	}

	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		writeSessionExportError(w, err)
		return
	}
	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil {
		writeSessionExportError(w, err)
		return
	}
	exportEffectiveInputs := make([]map[string]any, 0, len(effectiveInputs))
	for _, input := range effectiveInputs {
		exportEffectiveInputs = append(exportEffectiveInputs, sessionExportEffectiveInputMap(input))
	}
	exportEvidence := make([]map[string]any, 0, len(evidence))
	for _, item := range evidence {
		exportEvidence = append(exportEvidence, sessionExportDirectEvidenceMap(item))
	}
	kgTriples, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil {
		writeSessionExportError(w, err)
		return
	}
	_, err = s.Store.ListCriticFeedback(ctx, sid, "", 0)
	if err != nil {
		writeSessionExportError(w, err)
		return
	}
	_, err = s.Store.ListCharacterEvents(ctx, sid, "")
	if err != nil {
		writeSessionExportError(w, err)
		return
	}

	guidanceSnapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)

	canonicalLayers, _ := s.Store.ListCanonicalStateLayers(ctx, sid, "")
	canonicalLayers = nonNilSlice(canonicalLayers)

	chapterSummaries := make([]map[string]any, 0)
	arcSummaries := make([]map[string]any, 0)
	sagaDigests := make([]map[string]any, 0)
	pack, _ := s.Store.GetResumePack(ctx, sid, "resume")
	if pack != nil {
		if pack.Chapter != nil {
			chapterSummaries = append(chapterSummaries, map[string]any{
				"id":              pack.Chapter.ID,
				"chat_session_id": sid,
				"from_turn":       pack.Chapter.FromTurn,
				"to_turn":         pack.Chapter.ToTurn,
				"chapter_index":   pack.Chapter.ChapterIndex,
				"chapter_title":   pack.Chapter.ChapterTitle,
				"resume_text":     pack.Chapter.ResumeText,
				"summary_text":    pack.Chapter.SummaryText,
				"source":          "resume_pack_chapter",
			})
		}
		if pack.Arc != nil {
			arcSummaries = append(arcSummaries, map[string]any{
				"id":              pack.Arc.ID,
				"chat_session_id": sid,
				"from_turn":       pack.Arc.FromTurn,
				"to_turn":         pack.Arc.ToTurn,
				"arc_index":       pack.Arc.ArcIndex,
				"arc_name":        pack.Arc.ArcName,
				"arc_status":      pack.Arc.ArcStatus,
				"arc_resume_text": pack.Arc.ArcResumeText,
				"core_conflict":   pack.Arc.CoreConflict,
				"source":          "resume_pack_arc",
			})
		}
		if pack.Saga != nil {
			sagaDigests = append(sagaDigests, map[string]any{
				"id":               pack.Saga.ID,
				"chat_session_id":  sid,
				"from_turn":        pack.Saga.FromTurn,
				"to_turn":          pack.Saga.ToTurn,
				"era_label":        pack.Saga.EraLabel,
				"resume_pack_text": pack.Saga.ResumePackText,
				"saga_summary":     pack.Saga.SagaSummary,
				"source":           "resume_pack_saga",
			})
		}
	}

	embeddingIdentity := s.currentEmbeddingModelIdentity()
	currentModel := embeddingIdentity.Model
	memoryStatusCounts := map[string]int{}
	needsReembedCount := 0
	embeddingProvenance := map[string]any{
		"policy_version":                  "em1a.v1",
		"needs_reembed_policy_version":    "em1b.v1",
		"current_project_embedding_model": currentModel,
		"current_embedding_model_source":  embeddingIdentity.Source,
		"memory_status_counts":            memoryStatusCounts,
		"needs_reembed_count":             needsReembedCount,
	}
	for _, m := range memories {
		status := classifyMemoryEmbeddingStatus(m, currentModel)
		memoryStatusCounts[status]++
		if memoryEmbeddingNeedsReembed(status) == true {
			needsReembedCount++
		}
	}
	embeddingProvenance["needs_reembed_count"] = needsReembedCount

	lineageSummary := map[string]any{
		"direct_evidence_source_hash_count":         0,
		"direct_evidence_tombstoned_count":          0,
		"direct_evidence_superseded_count":          0,
		"canonical_layers_with_source_turn_count":   0,
		"canonical_layers_with_source_record_count": 0,
	}
	for _, e := range evidence {
		if strings.TrimSpace(e.SourceHash) != "" {
			lineageSummary["direct_evidence_source_hash_count"] = lineageSummary["direct_evidence_source_hash_count"].(int) + 1
		}
		if e.Tombstoned {
			lineageSummary["direct_evidence_tombstoned_count"] = lineageSummary["direct_evidence_tombstoned_count"].(int) + 1
		}
		if e.SupersededByID > 0 {
			lineageSummary["direct_evidence_superseded_count"] = lineageSummary["direct_evidence_superseded_count"].(int) + 1
		}
	}
	for _, layer := range canonicalLayers {
		if layer.SourceTurn > 0 {
			lineageSummary["canonical_layers_with_source_turn_count"] = lineageSummary["canonical_layers_with_source_turn_count"].(int) + 1
		}
		if layer.SourceRecord > 0 {
			lineageSummary["canonical_layers_with_source_record_count"] = lineageSummary["canonical_layers_with_source_record_count"].(int) + 1
		}
	}

	summary := map[string]int{
		"chat_logs_count":                len(chatLogs),
		"effective_inputs_count":         len(effectiveInputs),
		"memories_count":                 len(memories),
		"direct_evidence_records_count":  len(evidence),
		"canonical_state_layers_count":   len(canonicalLayers),
		"kg_triples_count":               len(kgTriples),
		"chapter_summaries_count":        len(chapterSummaries),
		"arc_summaries_count":            len(arcSummaries),
		"saga_digests_count":             len(sagaDigests),
		"guidance_compact_records_count": 0,
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "export",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session export generated",
		DetailsJSON:   mustCompactJSON(map[string]any{"export_version": "1.1", "summary": summary}),
		Source:        "session_export",
		CreatedAt:     time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"export_version":  "1.1",
		"exported_at":     formatUTCISOOffset(time.Now()),
		"summary":         summary,
		"portability_contract": map[string]any{
			"package_policy_version":         "sp1a.v1",
			"rebuild_handoff_policy_version": "sp1d.v1",
			"operation_policy_version":       "sp1e.v1",
			"package_mode":                   "logical_event_package",
			"db_snapshot_policy":             "admin_full_profile_explicit_only",
			"db_snapshot_default_included":   false,
			"runtime_artifact_policy":        "exclude_cache_temp_logs_downloads_git_runtime_proofs",
			"vector_artifact_policy":         "exclude_from_default_package_rebuildable_retrieval_artifact",
			"canonical_truth_authority":      "mariadb_store",
			"vector_retrieval_lane":          "chromadb_only",
			"vector_engine_policy":           "chromadb_only",
			"manual_first":                   true,
			"auto_copy_detection":            "deferred",
			"session_origin":                 sid,
			"portable_units":                 []string{"chat_logs", "effective_inputs", "memories", "direct_evidence_records", "canonical_state_layers", "kg_triples", "chapter_summaries", "arc_summaries", "saga_digests", "guidance_snapshot"},
			"lineage_surfaces": []string{
				"direct_evidence_records.source_hash",
				"direct_evidence_records.source_turn_start",
				"direct_evidence_records.source_turn_end",
				"direct_evidence_records.turn_anchor",
				"direct_evidence_records.lineage",
				"direct_evidence_records.tombstoned",
				"direct_evidence_records.superseded_by_id",
				"canonical_state_layers.source_turn",
				"canonical_state_layers.source_record",
				"canonical_state_layers.last_verified_turn",
			},
			"rebuild_handoff": map[string]any{
				"dirty_event_type": "backfill_import",
				"rebuild_mode":     "selective",
				"start_point":      "next_prepare_turn_fetch",
				"source_rows":      []string{"chat_logs", "effective_input_logs", "direct_evidence_records", "canonical_state_layers"},
				"rebuild_targets":  []string{"direct_evidence", "canonical_state", "dense_summary", "sidecar"},
				"inherited_runtime_policy_versions": map[string]string{
					"dirty_matrix":        "or1h.v1",
					"rebuild":             "or1i.v1",
					"stale_serving_guard": "or1j.v1",
				},
			},
		},
		"chat_logs":               chatLogs,
		"effective_inputs":        exportEffectiveInputs,
		"memories":                memories,
		"direct_evidence_records": exportEvidence,
		"canonical_state_layers":  canonicalLayers,
		"kg_triples":              kgTriples,
		"chapter_summaries":       chapterSummaries,
		"arc_summaries":           arcSummaries,
		"saga_digests":            sagaDigests,
		"guidance_snapshot":       guidanceSnapshot,
		"embedding_provenance":    embeddingProvenance,
		"lineage_summary":         lineageSummary,
	})
}

func writeSessionExportError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotEnabled) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                  "ok",
			"chat_session_id":         "",
			"export_version":          "1.1",
			"exported_at":             formatUTCISOOffset(time.Now()),
			"summary":                 map[string]int{},
			"portability_contract":    map[string]any{},
			"chat_logs":               []any{},
			"effective_inputs":        []any{},
			"memories":                []any{},
			"direct_evidence_records": []any{},
			"canonical_state_layers":  []any{},
			"kg_triples":              []any{},
			"chapter_summaries":       []any{},
			"arc_summaries":           []any{},
			"saga_digests":            []any{},
			"guidance_snapshot":       map[string]any{},
			"embedding_provenance":    map[string]any{},
			"lineage_summary":         map[string]any{},
		})
		return
	}
	writeInternalError(w, err.Error())
}

func sessionExportEffectiveInputMap(input store.EffectiveInput) map[string]any {
	return map[string]any{
		"id":              input.ID,
		"chat_session_id": input.ChatSessionID,
		"turn_index":      input.TurnIndex,
		"effective_input": input.EffectiveInput,
		"created_at":      formatNaiveUTCTime(input.CreatedAt),
	}
}

func sessionExportDirectEvidenceMap(item store.DirectEvidence) map[string]any {
	return map[string]any{
		"id":                   item.ID,
		"chat_session_id":      item.ChatSessionID,
		"evidence_kind":        item.EvidenceKind,
		"evidence_text":        item.EvidenceText,
		"source_turn_start":    item.SourceTurnStart,
		"source_turn_end":      item.SourceTurnEnd,
		"turn_anchor":          item.TurnAnchor,
		"source_message_ids":   sessionExportJSONValue(item.SourceMessageIDsJSON, []any{}),
		"source_hash":          item.SourceHash,
		"archive_state":        item.ArchiveState,
		"capture_stage":        item.CaptureStage,
		"capture_verification": item.CaptureVerification,
		"committed_gate":       item.CommittedGate,
		"lineage":              sessionExportJSONValue(item.LineageJSON, map[string]any{}),
		"repair_needed":        item.RepairNeeded,
		"tombstoned":           item.Tombstoned,
		"superseded_by_id":     item.SupersededByID,
		"created_at":           formatNaiveUTCTime(item.CreatedAt),
	}
}

func sessionExportJSONValue(raw string, fallback any) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return fallback
	}
	var decoded any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		return fallback
	}
	return decoded
}

// classifyMemoryEmbeddingStatus returns the embedding lifecycle status for a memory row
// against the current project embedding model. This implements EM-1a/EM-1b provenance.
func classifyMemoryEmbeddingStatus(m store.Memory, currentModel any) string {
	cm := ""
	if s, ok := currentModel.(string); ok {
		cm = strings.TrimSpace(s)
	}
	embeddingEmpty := strings.TrimSpace(m.Embedding) == ""
	model := strings.TrimSpace(m.EmbeddingModel)
	modelEmpty := model == ""
	if cm == "" {
		return "project_model_unset"
	}
	if embeddingEmpty && modelEmpty {
		return "missing_embedding_and_model"
	}
	if embeddingEmpty {
		return "missing_embedding_vector"
	}
	if modelEmpty {
		return "missing_embedding_model"
	}
	if model != cm {
		return "model_mismatch"
	}
	return "current_model_match"
}

func memoryEmbeddingNeedsReembed(status string) any {
	switch status {
	case "missing_embedding_and_model", "missing_embedding_vector", "missing_embedding_model", "model_mismatch":
		return true
	case "current_model_match":
		return false
	default:
		return nil
	}
}

func memoryEmbeddingRetrievalFallback(status string) string {
	switch status {
	case "current_model_match":
		return "embedding_current"
	case "model_mismatch", "missing_embedding_model":
		return "hybrid_degrade"
	case "missing_embedding_vector", "missing_embedding_and_model":
		return "importance_only"
	default:
		return "store_only_project_model_unset"
	}
}

func formatUTCISOOffset(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000+00:00")
}

func pythonDefaultJSONRuneLen(value any) int {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	inString := false
	escaped := false
	length := 0
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRune(raw[i:])
		i += size
		length++
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case ':', ',':
			length++
		}
	}
	return length
}

// buildL3GuidanceSnapshot reads GuidancePlanState from store and normalizes
// the L-3 contract response shape. Returns the snapshot map and a bool
// indicating whether a cached state was found.
