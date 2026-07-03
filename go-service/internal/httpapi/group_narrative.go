package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

var htmlImgTagPattern = regexp.MustCompile(`<img=[^>]*>`)

// registerNarrativeRoutes mounts session, narrative, character, world-rule,
// metric, feedback, and import endpoints.
func (s *Server) registerNarrativeRoutes(mux *http.ServeMux) {
	// Session read: R1
	mux.HandleFunc("GET /sessions", s.handleSessionsList)
	mux.HandleFunc("GET /sessions/{chat_session_id}/export", s.handleSessionExport)
	mux.HandleFunc("GET /sessions/{chat_session_id}/guidance-snapshot", s.handleSessionGuidanceSnapshot)
	mux.HandleFunc("GET /sessions/{chat_session_id}/step7-health", s.handleSessionStep7Health)
	mux.HandleFunc("GET /sessions/{chat_session_id}/resume-pack", s.handleSessionResumePack)
	mux.HandleFunc("GET /sessions/compare", s.handleSessionsCompare)
	mux.HandleFunc("GET /sessions/{sid}", s.handleSessionsGet404)
	mux.HandleFunc("DELETE /sessions/{chat_session_id}", s.handleSessionDelete)
	mux.HandleFunc("GET /active-states/{chat_session_id}", s.handleActiveStates)
	mux.HandleFunc("GET /canonical-state-layer/{chat_session_id}", s.handleCanonicalStateLayer)
	mux.HandleFunc("GET /session-state/{chat_session_id}", s.handleSessionState)
	mux.HandleFunc("GET /continuity-pack/{chat_session_id}", s.handleContinuityPack)
	mux.HandleFunc("GET /pending-threads/{chat_session_id}", s.handlePendingThreads)
	mux.HandleFunc("GET /continuity-hooks/{chat_session_id}", s.handleContinuityHooks)
	mux.HandleFunc("GET /narrative-recall/packet/preview", s.handleNarrativeRecallPacketPreview)
	mux.HandleFunc("GET /session/{chat_session_id}/active-scope", s.handleActiveScopeGet)
	mux.HandleFunc("GET /session/{sid}", s.handleSessionGet404)
	mux.HandleFunc("GET /momentum-packet/{chat_session_id}", s.handleMomentumPacket)
	mux.HandleFunc("GET /narrative-control/{chat_session_id}", s.handleNarrativeControlGet)

	// Session write: R2
	mux.HandleFunc("PATCH /session/{chat_session_id}/active-scope", s.handleActiveScopePatch)
	mux.HandleFunc("PATCH /narrative-control/{chat_session_id}/director-patch", s.handleDirectorPatch)

	// Storyline: R1 read, R2 write
	mux.HandleFunc("GET /storylines/{chat_session_id}", s.handleStorylinesGet)
	mux.HandleFunc("PATCH /storylines/{storyline_id}", s.handleStorylinePatch)
	mux.HandleFunc("PATCH /storylines/{storyline_id}/trust", s.handleStorylineTrust)
	mux.HandleFunc("DELETE /storylines/{storyline_id}", s.handleStorylineDelete)
	mux.HandleFunc("POST /storylines/sync", s.handleStorylinesSync)

	// Character: R1 read, R2 write
	mux.HandleFunc("GET /characters/{chat_session_id}", s.handleCharactersGet)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}", s.handleCharacterDetail)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}/events", s.handleCharacterEvents)
	mux.HandleFunc("GET /characters/{chat_session_id}/{character_name}/state-history", s.handleCharacterStateHistory)
	mux.HandleFunc("PATCH /characters/{chat_session_id}/{character_name}", s.handleCharacterPatch)
	mux.HandleFunc("PATCH /characters/{chat_session_id}/{character_name}/speech", s.handleCharacterSpeech)
	mux.HandleFunc("DELETE /characters/{chat_session_id}/{character_name}", s.handleCharacterDelete)

	// World rules: R1 read, R2 write
	mux.HandleFunc("GET /world-rules/{chat_session_id}", s.handleWorldRulesGet)
	mux.HandleFunc("GET /world-rules/{chat_session_id}/inherited", s.handleWorldRulesInherited)
	mux.HandleFunc("POST /world-rules/sync", s.handleWorldRulesSync)
	mux.HandleFunc("PATCH /world-rules/{rule_id}", s.handleWorldRulePatch)
	mux.HandleFunc("PATCH /world-rules/{rule_id}/trust", s.handleWorldRuleTrust)
	mux.HandleFunc("DELETE /world-rules/{rule_id}", s.handleWorldRuleDelete)

	// Episodes: R1 read/search, R2 generate/write
	mux.HandleFunc("GET /episodes/{chat_session_id}", s.handleEpisodesGet)
	mux.HandleFunc("GET /episodes/detail/{episode_id}", s.handleEpisodeDetail)
	mux.HandleFunc("POST /episodes/generate", s.handleEpisodeGenerate)
	mux.HandleFunc("POST /chapters/generate", s.handleChapterGenerate)
	mux.HandleFunc("POST /arcs/generate", s.handleArcGenerate)
	mux.HandleFunc("POST /sagas/generate", s.handleSagaGenerate)
	mux.HandleFunc("POST /chapters/dry-run", s.handleChapterDryRun)
	mux.HandleFunc("POST /chapters/search", s.handleChapterSearch)
	mux.HandleFunc("POST /episodes/search", s.handleEpisodeSearch)
	mux.HandleFunc("PATCH /episodes/{episode_id}", s.handleEpisodePatch)
	mux.HandleFunc("DELETE /episodes/{episode_id}", s.handleEpisodeDelete)
	mux.HandleFunc("POST /episodes/regenerate", s.handleEpisodeRegenerate)
	mux.HandleFunc("POST /episodes/merge", s.handleEpisodeMerge)

	// Pending threads: R1 read, R2 write
	mux.HandleFunc("PATCH /pending-threads/{hook_id}", s.handlePendingThreadPatch)
	mux.HandleFunc("PATCH /continuity-hooks/{hook_id}", s.handlePendingThreadPatch)
	mux.HandleFunc("PATCH /pending-threads/{hook_id}/trust", s.handlePendingThreadTrust)
	mux.HandleFunc("DELETE /pending-threads/{hook_id}", s.handlePendingThreadDelete)

	// Metrics: R1 read
	mux.HandleFunc("GET /metrics/lc1c/{chat_session_id}", s.handleMetricsLC1C)
	mux.HandleFunc("GET /metrics/lc1d/{chat_session_id}", s.handleMetricsLC1D)
	mux.HandleFunc("GET /metrics/lc1e/{chat_session_id}", s.handleMetricsLC1E)
	mux.HandleFunc("GET /metrics/lc1f/{chat_session_id}", s.handleMetricsLC1F)
	mux.HandleFunc("GET /metrics/lc1g/{chat_session_id}", s.handleMetricsLC1G)
	mux.HandleFunc("GET /metrics/lc1h/{chat_session_id}", s.handleMetricsLC1H)
	mux.HandleFunc("GET /metrics/lc1i/{chat_session_id}", s.handleMetricsLC1I)
	mux.HandleFunc("GET /metrics/lc1j/{chat_session_id}", s.handleMetricsLC1J)
	mux.HandleFunc("GET /metrics/lc1k/{chat_session_id}", s.handleMetricsLC1K)
	mux.HandleFunc("GET /metrics/lc1l/{chat_session_id}", s.handleMetricsLC1L)
	mux.HandleFunc("GET /metrics/lc1m/{chat_session_id}", s.handleMetricsLC1M)
	mux.HandleFunc("GET /metrics/lc1n/{chat_session_id}", s.handleMetricsLC1N)
	mux.HandleFunc("GET /metrics/lc1o/{chat_session_id}", s.handleMetricsLC1O)
	mux.HandleFunc("GET /metrics/lc1p/{chat_session_id}", s.handleMetricsLC1P)
	mux.HandleFunc("GET /metrics/lc1q/{chat_session_id}", s.handleMetricsLC1Q)
	mux.HandleFunc("GET /metrics/lc1r/regression-corpus", s.handleMetricsLC1R)
	mux.HandleFunc("GET /metrics/lc1s/step17-bundle-closure", s.handleMetricsLC1S)
	mux.HandleFunc("GET /metrics/tm1d/{chat_session_id}", s.handleMetricsTM1D)

	// Audit / feedback / import
	mux.HandleFunc("GET /audit", s.handleAuditGet)
	mux.HandleFunc("POST /feedback", s.handleFeedbackPost)
	mux.HandleFunc("GET /feedback/latest", s.handleFeedbackLatest)
	mux.HandleFunc("POST /import/hypamemory", s.handleImportHypamemory)
}

// Session read surfaces (R1).

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
func (s *Server) buildL3GuidanceSnapshot(ctx context.Context, sid string) (map[string]any, bool) {
	warnings := []any{}
	snapshot := map[string]any{
		"state_status":     "no_state",
		"last_turn":        -1,
		"story_plan":       map[string]any{},
		"director":         map[string]any{},
		"compact_records":  []any{},
		"maintenance_last": nil,
		"warnings":         warnings,
	}

	gps, ok := s.Store.(store.GuidancePlanStateStore)
	if !ok {
		warnings = append(warnings, "GuidancePlanStateStore not available; safe degrade to no_state.")
		snapshot["warnings"] = warnings
		return snapshot, false
	}

	cached, err := gps.GetGuidancePlanState(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "No cached guidance plan state found; safe degrade to no_state.")
		} else {
			warnings = append(warnings, fmt.Sprintf("GuidancePlanState read error: %v; safe degrade to no_state.", err))
		}
		snapshot["warnings"] = warnings
		return snapshot, false
	}
	if cached == nil {
		warnings = append(warnings, "Cached guidance plan state is nil; safe degrade to no_state.")
		snapshot["warnings"] = warnings
		return snapshot, false
	}

	var storyPlan map[string]any
	var director map[string]any
	if cached.StoryPlanJSON != "" {
		_ = json.Unmarshal([]byte(cached.StoryPlanJSON), &storyPlan)
	}
	if cached.DirectorJSON != "" {
		_ = json.Unmarshal([]byte(cached.DirectorJSON), &director)
	}
	if storyPlan == nil {
		storyPlan = map[string]any{}
	}
	if director == nil {
		director = map[string]any{}
	}

	var cachedWarnings []any
	if cached.WarningsJSON != "" {
		_ = json.Unmarshal([]byte(cached.WarningsJSON), &cachedWarnings)
	}
	if cachedWarnings == nil {
		cachedWarnings = []any{}
	}

	stateStatus := strings.TrimSpace(cached.StateStatus)
	if stateStatus == "empty" {
		cachedWarnings = append(cachedWarnings, "rebuild will be triggered by next GET /narrative-control call")
	} else {
		stateStatus = "active"
	}

	lastTurn := cached.LastTurn
	if lastTurn < 0 {
		lastTurn = -1
	}

	snapshot = map[string]any{
		"state_status":     stateStatus,
		"last_turn":        lastTurn,
		"story_plan":       storyPlan,
		"director":         director,
		"compact_records":  []any{},
		"maintenance_last": nil,
		"warnings":         cachedWarnings,
	}

	return snapshot, true
}

func (s *Server) handleSessionGuidanceSnapshot(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	snapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	snapshot["status"] = "ok"
	snapshot["chat_session_id"] = sid
	snapshot["generated_at"] = generatedAt()

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleSessionStep7Health(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	// L-5: parse passes query param (default 10, clamp 1..50)
	passes := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("passes")); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil {
			if p < 1 {
				passes = 1
			} else if p > 50 {
				passes = 50
			} else {
				passes = p
			}
		}
	}

	chatLogs, chatLogErr := s.Store.ListChatLogs(ctx, sid, 0, 0)
	chatLogs = nonNilSlice(chatLogs)

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)

	warnings := []any{}
	if chatLogErr != nil && !errors.Is(chatLogErr, store.ErrNotEnabled) {
		warnings = append(warnings, fmt.Sprintf("chat_logs read error: %v", chatLogErr))
	}

	// L-3 guidance snapshot read-through
	guidanceSnapshot, hasCached := s.buildL3GuidanceSnapshot(ctx, sid)
	stateStatus, _ := guidanceSnapshot["state_status"].(string)
	lastTurn := -1
	if v, ok := guidanceSnapshot["last_turn"].(int); ok {
		lastTurn = v
	}
	storyPlan, _ := guidanceSnapshot["story_plan"].(map[string]any)
	director, _ := guidanceSnapshot["director"].(map[string]any)

	arcAgeTurns := 0
	if hasCached && stateStatus == "active" && lastTurn >= 0 {
		arcAgeTurns = len(chatLogs) - lastTurn
		if arcAgeTurns < 0 {
			arcAgeTurns = 0
		}
	}

	// L-5 compaction via existing compact history builder
	_, compactMeta := buildNarrativeCompactHistory(storyPlan, director, storylines, pendingThreads)

	// L-5 maintenance via audit logs (existing store method, no new table)
	maintenanceLogs, maintErr := s.Store.ListAuditLogs(ctx, sid, "maintenance_enqueued", passes)
	if maintErr != nil && !errors.Is(maintErr, store.ErrNotEnabled) {
		warnings = append(warnings, fmt.Sprintf("maintenance audit log read error: %v", maintErr))
	}
	maintenanceLogs = nonNilSlice(maintenanceLogs)
	totalPasses := len(maintenanceLogs)
	okCount := totalPasses
	errorCount := 0
	lastSuggestions := []any{}
	if maintErr != nil {
		// read error means we can't confirm ok counts
		okCount = 0
		errorCount = totalPasses
	}
	for _, log := range maintenanceLogs {
		if strings.Contains(log.DetailsJSON, "suggestion") {
			lastSuggestions = append(lastSuggestions, log.DetailsJSON)
		}
	}
	if len(lastSuggestions) > 3 {
		lastSuggestions = lastSuggestions[:3]
	}
	okRate := 0.0
	if totalPasses > 0 {
		okRate = float64(okCount) / float64(totalPasses)
	}

	// L-5 drift summary (conservative: no new table)
	driftSummary := map[string]any{
		"passes_analyzed": totalPasses,
		"total_signals":   0,
		"high_severity":   0,
		"by_type":         map[string]any{},
	}

	// L-5 regression checks from actual data
	regressionChecks := map[string]any{
		"guidance_persistence": "skip",
		"arc_stability":        "skip",
		"compaction_health":    "skip",
		"maintenance_effect":   "skip",
		"notes":                []any{},
	}
	switch stateStatus {
	case "active":
		regressionChecks["guidance_persistence"] = "pass"
	case "empty":
		regressionChecks["guidance_persistence"] = "warn"
		warnings = append(warnings, "guidance snapshot state is empty; rebuild pending")
	default:
		regressionChecks["guidance_persistence"] = "fail"
	}
	if arcAgeTurns >= 3 {
		regressionChecks["arc_stability"] = "pass"
	} else if stateStatus == "active" {
		regressionChecks["arc_stability"] = "warn"
		regressionChecks["notes"] = append(regressionChecks["notes"].([]any), fmt.Sprintf("arc_age_turns=%d (<3)", arcAgeTurns))
	}
	records, _ := compactMeta["total_records"].(int)
	if records >= 1 {
		regressionChecks["compaction_health"] = "pass"
	} else {
		regressionChecks["compaction_health"] = "warn"
		regressionChecks["notes"] = append(regressionChecks["notes"].([]any), "compact record count is 0 - session may be unresolved")
	}
	if totalPasses > 0 {
		if okRate >= 0.8 {
			regressionChecks["maintenance_effect"] = "pass"
		} else {
			regressionChecks["maintenance_effect"] = "fail"
			regressionChecks["notes"] = append(regressionChecks["notes"].([]any), fmt.Sprintf("maintenance ok_rate=%.2f (<0.8)", okRate))
		}
	} else {
		regressionChecks["maintenance_effect"] = "skip"
	}

	guidanceWarnings, _ := guidanceSnapshot["warnings"].([]any)
	warnings = append(warnings, guidanceWarnings...)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"total_turns":     len(chatLogs),
		"guidance_state": map[string]any{
			"status":          stateStatus,
			"last_built_turn": lastTurn,
			"arc_age_turns":   arcAgeTurns,
			"active_tensions": firstPositiveInt(len(asStringSlice(storyPlan["active_tensions"])), len(storylines)),
			"next_beats":      len(asAnySlice(storyPlan["next_beats"])),
			"open_required":   firstPositiveInt(len(asStringSlice(director["required_outcomes"])), countPinnedPendingThreads(pendingThreads)),
			"forbidden_count": firstPositiveInt(len(asStringSlice(director["forbidden_moves"])), countRiskPendingThreads(pendingThreads)),
		},
		"drift_summary":      driftSummary,
		"compaction_summary": compactMeta,
		"maintenance_summary": map[string]any{
			"total_passes":     totalPasses,
			"ok_count":         okCount,
			"error_count":      errorCount,
			"ok_rate":          okRate,
			"last_suggestions": lastSuggestions,
		},
		"regression_checks": regressionChecks,
		"generated_at":      generatedAt(),
		"warnings":          warnings,
	})
}
func (s *Server) handleSessionResumePack(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	trigger := strings.TrimSpace(r.URL.Query().Get("continuity_trigger_mode"))
	if trigger == "" {
		trigger = strings.TrimSpace(r.URL.Query().Get("trigger"))
	}
	if trigger == "" {
		trigger = "resume"
	}
	ctx := r.Context()
	pack := emptyResumePack(trigger)
	warnings := []any{}
	if storedPack, err := s.Store.GetResumePack(ctx, sid, trigger); err == nil && storedPack != nil {
		pack = resumePackToResponse(storedPack, trigger)
	} else if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "resume pack read failed; safe empty resume pack returned")
	}
	guidanceSnapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"detail":            "resume_pack_returned",
		"chat_session_id":   sid,
		"resume_pack":       pack,
		"guidance_snapshot": guidanceSnapshot,
		"generated_at":      generatedAt(),
		"warnings":          warnings,
	})
}

func emptyResumePack(trigger string) map[string]any {
	return map[string]any{
		"pack_status":    "empty",
		"trigger":        trigger,
		"sources_used":   []string{},
		"layer_count":    0,
		"assembled_text": "",
		"saga":           nil,
		"arc":            nil,
		"chapter":        nil,
		"assembly_note":  "P-4c: read-only long-gap resume pack; not wired into injection or input_context",
	}
}

func resumePackToResponse(pack *store.ResumePack, trigger string) map[string]any {
	if pack == nil {
		return emptyResumePack(trigger)
	}
	sources := pack.SourcesUsed
	if sources == nil {
		sources = []string{}
	}
	packStatus := strings.TrimSpace(pack.PackStatus)
	if packStatus == "" {
		packStatus = "ready"
	}
	packTrigger := strings.TrimSpace(pack.Trigger)
	if packTrigger == "" {
		packTrigger = trigger
	}
	return map[string]any{
		"pack_status":    packStatus,
		"trigger":        packTrigger,
		"sources_used":   sources,
		"layer_count":    pack.LayerCount,
		"assembled_text": pack.AssembledText,
		"saga":           pack.Saga,
		"arc":            pack.Arc,
		"chapter":        pack.Chapter,
		"assembly_note":  pack.AssemblyNote,
	}
}

func generatedAt() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func maxNarrativeEvidenceTurn(storylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, characters []store.CharacterState) int {
	maxTurn := 0
	for _, sl := range storylines {
		if sl.LastTurn > maxTurn {
			maxTurn = sl.LastTurn
		}
		if sl.LastEvidenceTurn > maxTurn {
			maxTurn = sl.LastEvidenceTurn
		}
	}
	for _, hook := range pendingThreads {
		if hook.SourceTurn > maxTurn {
			maxTurn = hook.SourceTurn
		}
		if hook.CreatedTurn > maxTurn {
			maxTurn = hook.CreatedTurn
		}
		if hook.ResolvedTurn > maxTurn {
			maxTurn = hook.ResolvedTurn
		}
	}
	for _, st := range activeStates {
		if st.TurnIndex > maxTurn {
			maxTurn = st.TurnIndex
		}
	}
	for _, ch := range characters {
		if ch.TurnIndex > maxTurn {
			maxTurn = ch.TurnIndex
		}
	}
	return maxTurn
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return nonNilSlice(items)
	}
	var anyItems []any
	if err := json.Unmarshal([]byte(raw), &anyItems); err != nil {
		return []string{}
	}
	out := []string{}
	for _, item := range anyItems {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func buildStoryPlanSnapshot(storylines []store.Storyline, pendingThreads []store.PendingThread, characters []store.CharacterState, worldRules []store.WorldRule, lastTurn int) map[string]any {
	currentArc := ""
	narrativeGoal := ""
	activeTensions := []string{}
	nextBeats := []string{}
	continuityAnchors := []string{}
	focusCharacters := []string{}
	guardrails := []string{}

	activeStorylines := activeNarrativeStorylines(storylines)
	openHooks := openNarrativeThreads(pendingThreads)
	keyWorldRules := narrativeKeyWorldRules(worldRules)

	if len(activeStorylines) > 0 {
		primary := activeStorylines[0]
		currentArc = primary.Name
		narrativeGoal = truncateRunes(strings.TrimSpace(primary.CurrentContext), 200)
		activeTensions = parseStorylineListJSON(primary.OngoingTensionsJSON)
		keyPoints := parseStorylineListJSON(primary.KeyPointsJSON)
		if len(keyPoints) > 2 {
			keyPoints = keyPoints[len(keyPoints)-2:]
		}
		continuityAnchors = append(continuityAnchors, keyPoints...)
		for _, entity := range parseStringList(primary.EntitiesJSON) {
			if len(focusCharacters) >= 4 {
				break
			}
			focusCharacters = append(focusCharacters, entity)
		}
		for _, sl := range activeStorylines[1:minInt(len(activeStorylines), 3)] {
			tensions := parseStorylineListJSON(sl.OngoingTensionsJSON)
			for _, tension := range tensions[:minInt(len(tensions), 1)] {
				activeTensions = appendUniqueString(activeTensions, tension)
			}
		}
	}
	for _, hook := range openHooks {
		if len(nextBeats) >= 4 {
			break
		}
		beat := pendingThreadNarrativeLabel(hook)
		nextBeats = append(nextBeats, beat)
		threadType := strings.TrimSpace(hook.ThreadType)
		if threadType == "" {
			threadType = strings.TrimSpace(hook.HookType)
		}
		if threadType == "promise" || threadType == "unresolved_goal" {
			continuityAnchors = appendUniqueString(continuityAnchors, pendingThreadTitle(hook))
		}
	}
	for _, wr := range keyWorldRules[:minInt(len(keyWorldRules), 3)] {
		guardrails = append(guardrails, worldRuleGuardrail(wr, false))
	}
	status := "empty"
	if currentArc != "" || len(nextBeats) > 0 || len(activeTensions) > 0 {
		status = "heuristic"
	}
	return map[string]any{
		"current_arc":        currentArc,
		"narrative_goal":     narrativeGoal,
		"active_tensions":    limitStrings(activeTensions, 4),
		"next_beats":         limitStrings(nextBeats, 6),
		"continuity_anchors": limitStrings(continuityAnchors, 4),
		"guardrails":         limitStrings(guardrails, 4),
		"persona_priorities": []any{},
		"execution_notes":    []string{},
		"focus_characters":   limitStrings(focusCharacters, 4),
		"last_plan_turn":     lastTurn,
		"state_status":       status,
	}
}

func buildDirectorSnapshot(storylines []store.Storyline, pendingThreads []store.PendingThread, characters []store.CharacterState, worldRules []store.WorldRule, lastTurn int) map[string]any {
	required := []string{}
	forbidden := []string{}
	executionChecklist := []string{}
	personaGuardrails := []string{}
	worldGuardrails := []string{}
	focusCharacters := []string{}
	activeStorylines := activeNarrativeStorylines(storylines)
	openHooks := openNarrativeThreads(pendingThreads)
	keyWorldRules := narrativeKeyWorldRules(worldRules)

	for _, hook := range openHooks {
		label := pendingThreadTitle(hook)
		if hook.Pinned {
			required = append(required, "Carry forward: "+label)
		}
		if pendingThreadType(hook) == "risk" {
			forbidden = append(forbidden, "Do not abruptly resolve: "+label)
		}
	}
	if len(activeStorylines) > 0 {
		executionChecklist = append(executionChecklist,
			"Continue from the current scene state; do not open a new scene without cause.",
			"Deliver at least one visible beat before the response ends.",
		)
	}
	if len(activeStorylines) > 0 {
		for _, entity := range parseStringList(activeStorylines[0].EntitiesJSON) {
			if len(focusCharacters) >= 4 {
				break
			}
			focusCharacters = appendUniqueString(focusCharacters, entity)
		}
	}
	for _, wr := range keyWorldRules {
		if len(worldGuardrails) >= 4 {
			break
		}
		if wr.Category == "physics" {
			worldGuardrails = append(worldGuardrails, worldRuleGuardrail(wr, true))
		}
	}
	for _, ch := range latestCharacterStatesByName(characters) {
		if len(personaGuardrails) >= 4 {
			break
		}
		if !containsString(focusCharacters, ch.CharacterName) {
			continue
		}
		if hint := characterPersonaGuardrail(ch); hint != "" {
			personaGuardrails = append(personaGuardrails, hint)
		}
	}
	stateStatus := "empty"
	if len(activeStorylines) > 0 || len(required) > 0 || len(forbidden) > 0 || len(worldGuardrails) > 0 || len(focusCharacters) > 0 {
		stateStatus = "heuristic"
	}
	currentArc := ""
	if len(activeStorylines) > 0 {
		currentArc = activeStorylines[0].Name
	}
	pressureLevel := "light"
	if countPinnedPendingThreads(openHooks) >= 2 || len(asStringSlice(buildStoryPlanSnapshot(activeStorylines, openHooks, characters, worldRules, lastTurn)["active_tensions"])) >= 3 {
		pressureLevel = "strong"
	} else if countPinnedPendingThreads(openHooks) >= 1 || len(activeStorylines) > 0 {
		pressureLevel = "steady"
	}
	return map[string]any{
		"scene_mandate":       sceneMandateForArc(currentArc),
		"required_outcomes":   limitStrings(required, 6),
		"forbidden_moves":     limitStrings(forbidden, 6),
		"pressure_level":      pressureLevel,
		"execution_checklist": limitStrings(dedupeStrings(executionChecklist), 4),
		"persona_guardrails":  limitStrings(personaGuardrails, 4),
		"world_guardrails":    limitStrings(worldGuardrails, 4),
		"focus_characters":    limitStrings(focusCharacters, 4),
		"last_turn":           lastTurn,
		"state_status":        stateStatus,
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
}

func hasNarrativePlanSignal(plan map[string]any) bool {
	return strings.TrimSpace(asString(plan["current_arc"])) != "" || len(asStringSlice(plan["next_beats"])) > 0
}

func hasDirectorSignal(director map[string]any) bool {
	return strings.TrimSpace(asString(director["scene_mandate"])) != "" ||
		len(asStringSlice(director["required_outcomes"])) > 0 ||
		len(asStringSlice(director["world_guardrails"])) > 0
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asStringSlice(value any) []string {
	if items, ok := value.([]string); ok {
		return items
	}
	if items, ok := value.([]any); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return []string{}
}

// asAnySlice coerces a value to []any, returning an empty slice on failure.
func asAnySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	if items, ok := value.([]string); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

// unionAnyStringSlices returns a deduplicated union of two []any slices that
// contain strings. New items come first; old items not already present are
// appended. This preserves order and avoids duplicates.
func unionAnyStringSlices(newItems, oldItems []any) []any {
	seen := map[string]bool{}
	out := []any{}
	for _, v := range newItems {
		if s, ok := v.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, v := range oldItems {
		if s, ok := v.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func pendingThreadLabel(hook store.PendingThread) string {
	if strings.TrimSpace(hook.Description) != "" {
		return strings.TrimSpace(hook.Description)
	}
	if strings.TrimSpace(hook.ThreadKey) != "" {
		return strings.TrimSpace(hook.ThreadKey)
	}
	return "thread"
}

func activeNarrativeStorylines(items []store.Storyline) []store.Storyline {
	out := []store.Storyline{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		if strings.TrimSpace(item.Status) != "" && !strings.EqualFold(item.Status, "active") {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].LastTurn != out[j].LastTurn {
			return out[i].LastTurn > out[j].LastTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func openNarrativeThreads(items []store.PendingThread) []store.PendingThread {
	out := []store.PendingThread{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status != "" && status != "open" && status != "paused" {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].LastSeenTurn != out[j].LastSeenTurn {
			return out[i].LastSeenTurn > out[j].LastSeenTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func narrativeKeyWorldRules(items []store.WorldRule) []store.WorldRule {
	out := []store.WorldRule{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		switch item.Category {
		case "exists", "physics", "systems":
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func pendingThreadType(hook store.PendingThread) string {
	if strings.TrimSpace(hook.ThreadType) != "" {
		return strings.TrimSpace(hook.ThreadType)
	}
	return strings.TrimSpace(hook.HookType)
}

func pendingThreadTitle(hook store.PendingThread) string {
	if strings.TrimSpace(hook.Title) != "" {
		return strings.TrimSpace(hook.Title)
	}
	return pendingThreadLabel(hook)
}

func pendingThreadNarrativeLabel(hook store.PendingThread) string {
	threadType := pendingThreadType(hook)
	title := pendingThreadTitle(hook)
	if threadType == "" {
		return title
	}
	return "[" + threadType + "] " + title
}

func worldRuleGuardrail(rule store.WorldRule, withPrefix bool) string {
	desc := worldRuleDescription(rule)
	if withPrefix {
		return "World rule [" + rule.Key + "]: " + truncateRunes(desc, 80)
	}
	if strings.TrimSpace(rule.Key) == "" {
		return truncateRunes(desc, 80)
	}
	return rule.Key + ": " + truncateRunes(desc, 80)
}

func worldRuleDescription(rule store.WorldRule) string {
	raw := strings.TrimSpace(rule.ValueJSON)
	if raw != "" {
		var parsed any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			if m, ok := parsed.(map[string]any); ok {
				if s := firstStringValue(m, "description", "value", "summary", "detail"); s != "" {
					return s
				}
				return truncateRunes(compactJSONForShadow(m, 120), 120)
			}
			if s, ok := parsed.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		return raw
	}
	return strings.TrimSpace(rule.Key)
}

func firstStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s := cleanShadowText(v, 180); s != "" {
				return s
			}
		}
	}
	return ""
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || containsString(items, value) {
		return items
	}
	return append(items, value)
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func dedupeStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		out = appendUniqueString(out, item)
	}
	return out
}

func sceneMandateForArc(arc string) string {
	arc = strings.TrimSpace(arc)
	if arc == "" {
		return ""
	}
	return "Continue arc: " + arc
}

func characterPersonaGuardrail(ch store.CharacterState) string {
	hints := []string{}
	if style, ok := parseSurfacePayload(ch.SpeechStyleJSON).(map[string]any); ok {
		if tone := firstStringValue(style, "tone", "style", "default_tone", "speech_notes", "honorific_style"); tone != "" {
			hints = append(hints, "speaks "+truncateRunes(tone, 60))
		}
	} else if s := cleanShadowText(parseSurfacePayload(ch.SpeechStyleJSON), 60); s != "" {
		hints = append(hints, "speaks "+s)
	}
	if pers, ok := parseSurfacePayload(ch.PersonalityJSON).(map[string]any); ok {
		if trait := firstStringValue(pers, "core_trait", "trait", "personality"); trait != "" {
			hints = append(hints, "core trait: "+truncateRunes(trait, 60))
		}
	} else if list, ok := parseSurfacePayload(ch.PersonalityJSON).([]any); ok && len(list) > 0 {
		if trait := cleanShadowText(list[0], 60); trait != "" {
			hints = append(hints, "core trait: "+trait)
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return "[" + ch.CharacterName + "] " + strings.Join(hints, "; ")
}

func limitStrings(items []string, limit int) []string {
	if items == nil {
		return []string{}
	}
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

// mergeDirectorPrev carries forward resolved_outcomes and expired_forbidden from a
// previous director snapshot. Newly resolved hooks (previously required but no
// longer present) are appended to resolved_outcomes. Newly expired risks
// (previously forbidden but no longer present) are appended to expired_forbidden.
// Hooks that reappear are removed from the historical lists.
func mergeDirectorPrev(newDirector, prevDirector map[string]any) map[string]any {
	if prevDirector == nil {
		return newDirector
	}
	prevRequired := asStringSlice(prevDirector["required_outcomes"])
	prevForbidden := asStringSlice(prevDirector["forbidden_moves"])
	prevResolved := asStringSlice(prevDirector["resolved_outcomes"])
	prevExpired := asStringSlice(prevDirector["expired_forbidden"])

	newRequired := asStringSlice(newDirector["required_outcomes"])
	newForbidden := asStringSlice(newDirector["forbidden_moves"])

	// resolved = previous resolved + (previously required that are no longer required)
	resolved := []string{}
	seenResolved := map[string]bool{}
	for _, item := range prevResolved {
		if !containsString(newRequired, item) && !seenResolved[item] {
			seenResolved[item] = true
			resolved = append(resolved, item)
		}
	}
	for _, item := range prevRequired {
		if !containsString(newRequired, item) && !seenResolved[item] {
			seenResolved[item] = true
			resolved = append(resolved, item)
		}
	}

	// expired = previous expired + (previously forbidden that are no longer forbidden)
	expired := []string{}
	seenExpired := map[string]bool{}
	for _, item := range prevExpired {
		if !containsString(newForbidden, item) && !seenExpired[item] {
			seenExpired[item] = true
			expired = append(expired, item)
		}
	}
	for _, item := range prevForbidden {
		if !containsString(newForbidden, item) && !seenExpired[item] {
			seenExpired[item] = true
			expired = append(expired, item)
		}
	}

	newDirector["resolved_outcomes"] = limitStrings(dedupeStrings(resolved), 6)
	newDirector["expired_forbidden"] = limitStrings(dedupeStrings(expired), 6)
	return newDirector
}

type narrativeCompactEntry struct {
	Summary    string
	RecordType string
	Weight     float64
	Turn       int
	Order      int
}

func buildNarrativeCompactHistory(storyPlan, director map[string]any, storylines []store.Storyline, pendingThreads []store.PendingThread) ([]string, map[string]any) {
	entries := []narrativeCompactEntry{}
	order := 0
	add := func(recordType, summary string, weight float64, turn int) {
		summary = strings.TrimSpace(summary)
		if summary == "" {
			return
		}
		entries = append(entries, narrativeCompactEntry{
			Summary:    truncateRunes(summary, 220),
			RecordType: recordType,
			Weight:     weight,
			Turn:       turn,
			Order:      order,
		})
		order++
	}

	baseWeight := 1.0
	switch strings.ToLower(strings.TrimSpace(asString(director["pressure_level"]))) {
	case "strong", "high":
		baseWeight += 0.65
	case "steady", "medium":
		baseWeight += 0.3
	}
	baseWeight += math.Min(0.4, float64(len(asStringSlice(storyPlan["active_tensions"])))*0.1)

	directorTurn := 0
	if v, ok := director["last_turn"].(int); ok {
		directorTurn = v
	} else if f, ok := director["last_turn"].(float64); ok {
		directorTurn = int(f)
	}
	for _, item := range asStringSlice(director["resolved_outcomes"]) {
		add("resolved_outcome", "Resolved: "+item, baseWeight+0.15, directorTurn)
	}
	for _, item := range asStringSlice(director["expired_forbidden"]) {
		add("expired_forbidden", "Forbidden expired: "+item, baseWeight+0.1, directorTurn)
	}

	for _, sl := range storylines {
		if !strings.EqualFold(strings.TrimSpace(sl.Status), "resolved") {
			continue
		}
		name := strings.TrimSpace(sl.Name)
		if name == "" {
			name = fmt.Sprintf("storyline_%d", sl.ID)
		}
		turn := firstPositiveInt(sl.LastTurn, sl.LastEvidenceTurn, sl.FirstTurn)
		weight := 0.9 + math.Min(0.5, sl.Confidence*0.5) + math.Min(0.35, float64(sl.EvidenceCount)*0.05)
		add("resolved_storyline", fmt.Sprintf("Resolved arc: %s resolved at turn %s", name, storylineObservedLabel(turn)), weight, turn)
	}

	for _, hook := range pendingThreads {
		if !strings.EqualFold(strings.TrimSpace(hook.Status), "resolved") {
			continue
		}
		title := pendingThreadTitle(hook)
		turn := firstPositiveInt(hook.ResolvedTurn, hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn)
		weight := 0.85
		if hook.Pinned {
			weight += 0.4
		}
		if pendingThreadType(hook) == "risk" || pendingThreadType(hook) == "emotional_debt" {
			weight += 0.25
		}
		if hook.Priority > 0 {
			weight += math.Min(0.3, float64(hook.Priority)*0.05)
		}
		add("resolved_hook", fmt.Sprintf("Resolved hook: %s resolved at turn %s", title, storylineObservedLabel(turn)), weight, turn)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Weight != entries[j].Weight {
			return entries[i].Weight > entries[j].Weight
		}
		if entries[i].Turn != entries[j].Turn {
			return entries[i].Turn > entries[j].Turn
		}
		return entries[i].Order < entries[j].Order
	})
	if len(entries) > 8 {
		entries = entries[:8]
	}

	summaries := make([]string, 0, len(entries))
	byType := map[string]int{}
	totalWeight := 0.0
	latestTurn := -1
	for _, entry := range entries {
		summaries = append(summaries, entry.Summary)
		byType[entry.RecordType]++
		totalWeight += entry.Weight
		if entry.Turn > latestTurn {
			latestTurn = entry.Turn
		}
	}
	avg := 0.0
	if len(entries) > 0 {
		avg = totalWeight / float64(len(entries))
	}
	return summaries, map[string]any{
		"total_records":           len(entries),
		"by_type":                 byType,
		"avg_emotional_weight":    avg,
		"latest_compaction_turn":  latestTurn,
		"emotion_weight_strategy": "pressure_level + active_tensions + pinned/risk/priority + storyline confidence/evidence",
	}
}

func countPinnedPendingThreads(items []store.PendingThread) int {
	count := 0
	for _, item := range items {
		if item.Pinned {
			count++
		}
	}
	return count
}

func countRiskPendingThreads(items []store.PendingThread) int {
	count := 0
	for _, item := range items {
		if strings.EqualFold(item.HookType, "risk") {
			count++
		}
	}
	return count
}

func buildNarrativeControlProgressionLedger(stateStatus string, director map[string]any, storyPlan map[string]any, lastTurn int) map[string]any {
	pressureLevel, _ := director["pressure_level"].(string)
	if pressureLevel == "" {
		pressureLevel = "light"
	}
	status := stateStatus
	if status == "" {
		status = "skeleton"
	}
	pendingBeats := []string{}
	if beats, ok := storyPlan["next_beats"].([]string); ok {
		for _, beat := range beats {
			b := strings.TrimSpace(beat)
			if b != "" && !containsString(pendingBeats, b) {
				pendingBeats = append(pendingBeats, b)
			}
		}
	}
	consumedBeats := []string{}
	if resolved, ok := director["resolved_outcomes"].([]string); ok {
		for _, item := range resolved {
			b := strings.TrimSpace(item)
			if b != "" && !containsString(consumedBeats, b) {
				consumedBeats = append(consumedBeats, b)
			}
		}
	}

	lastAdvancedTurn := any(nil)
	lastValidatedTurn := any(nil)
	ledgerStatus := "skeleton"
	if lastTurn > 0 || len(pendingBeats) > 0 || len(consumedBeats) > 0 {
		ledgerStatus = "tracking"
	}
	if lastTurn > 0 {
		lastAdvancedTurn = lastTurn
		if stateStatus == "ready" || stateStatus == "user_patched" {
			lastValidatedTurn = lastTurn
		}
	}

	pendingBeatsAny := make([]any, 0, len(pendingBeats))
	for _, b := range pendingBeats[:minInt(len(pendingBeats), 8)] {
		pendingBeatsAny = append(pendingBeatsAny, b)
	}
	consumedBeatsAny := make([]any, 0, len(consumedBeats))
	for _, b := range consumedBeats[:minInt(len(consumedBeats), 8)] {
		consumedBeatsAny = append(consumedBeatsAny, b)
	}

	consumedSet := map[string]bool{}
	for _, b := range consumedBeats {
		consumedSet[strings.ToLower(b)] = true
	}

	doNotResolveGuard := map[string]any{
		"status":                "active",
		"mode":                  "deterministic_no_llm",
		"min_turn_gap":          2,
		"protected_entry_types": []string{"unresolved_tension", "payoff"},
		"protected_sources":     []string{"story_plan.next_beats", "director.required_outcomes"},
		"long_horizon_tokens":   []string{"promise", "payoff", "callback", "\u003f\uC38C\uB0FD", "\u8E42\uB4ED\uAF51", "\u003f\uB6AF\uB2D4"},
	}

	lifecycleModel := map[string]any{
		"status":         "active",
		"states":         []string{"latent", "active", "escalating", "aftermath", "resolved", "dormant"},
		"pressure_scale": map[string]any{"min": 0, "max": 3},
		"decay_rules":    map[string]any{"latent": 5, "active": 4, "escalating": 3, "aftermath": 2, "resolved": 1, "dormant": 0},
		"mode":           "deterministic_no_llm",
	}

	unresolvedTensions := []any{}
	for _, beat := range pendingBeats {
		label := normalizeStoryLedgerLabel(beat)
		if label == "" {
			continue
		}
		anchor := buildLedgerAnchor(label, storyPlan, director)
		pressureScore, decayTurns := lifecycleProfileForState("latent", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "unresolved_tension",
			"label":              label,
			"source":             "story_plan.next_beats",
			"status":             "open",
			"lifecycle_state":    "latent",
			"pressure_score":     pressureScore,
			"decay_turns":        decayTurns,
			"deterministic":      true,
			"source_record_id":   nil,
			"source_message_ids": []any{},
			"affected_relations": anchor["affected_relations"],
			"affected_world":     anchor["affected_world"],
		}
		attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
		unresolvedTensions = append(unresolvedTensions, entry)
	}
	if requiredOutcomes, ok := director["required_outcomes"].([]string); ok {
		for _, item := range requiredOutcomes {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			if consumedSet[strings.ToLower(label)] {
				continue
			}
			dup := false
			for _, existing := range unresolvedTensions {
				if m, ok := existing.(map[string]any); ok && m["label"] == label {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "unresolved_tension",
				"label":              label,
				"source":             "director.required_outcomes",
				"status":             "open",
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
			unresolvedTensions = append(unresolvedTensions, entry)
		}
	}
	if len(unresolvedTensions) > 12 {
		unresolvedTensions = unresolvedTensions[:12]
	}

	consequences := []any{}
	if executionNotes, ok := storyPlan["execution_notes"].([]string); ok {
		for _, item := range executionNotes {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("escalating", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "story_plan.execution_notes",
				"status":             "pending",
				"lifecycle_state":    "escalating",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			attachDoNotResolveFields(entry, doNotResolveGuard, lastTurn)
			consequences = append(consequences, entry)
		}
	}
	if executionChecklist, ok := director["execution_checklist"].([]string); ok {
		for _, item := range executionChecklist {
			label := normalizeStoryLedgerLabel(item)
			if label == "" {
				continue
			}
			dup := false
			for _, existing := range consequences {
				if m, ok := existing.(map[string]any); ok && m["label"] == label {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			consequences = append(consequences, map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "director.execution_checklist",
				"status":             "pending",
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			})
		}
	}
	if len(consequences) > 12 {
		consequences = consequences[:12]
	}

	sceneDeltas := []any{}
	if sceneMandate, ok := director["scene_mandate"].(string); ok && strings.TrimSpace(sceneMandate) != "" {
		label := normalizeStoryLedgerLabel(sceneMandate)
		if label != "" {
			anchor := buildLedgerAnchor(label, storyPlan, director)
			pressureScore, decayTurns := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "scene_delta",
				"label":              label,
				"source":             "director.scene_mandate",
				"status":             "observed",
				"turn_hint":          lastTurn,
				"lifecycle_state":    "active",
				"pressure_score":     pressureScore,
				"decay_turns":        decayTurns,
				"deterministic":      true,
				"source_record_id":   nil,
				"source_message_ids": []any{},
				"affected_relations": anchor["affected_relations"],
				"affected_world":     anchor["affected_world"],
			}
			sceneDeltas = append(sceneDeltas, entry)
		}
	}
	if pressureLevel != "" {
		label := "pressure=" + pressureLevel
		anchor := buildLedgerAnchor(label, storyPlan, director)
		pressureScore, decayTurns := lifecycleProfileForState("escalating", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "scene_delta",
			"label":              label,
			"source":             "director.pressure_level",
			"status":             "observed",
			"turn_hint":          lastTurn,
			"lifecycle_state":    "escalating",
			"pressure_score":     pressureScore,
			"decay_turns":        decayTurns,
			"deterministic":      true,
			"source_record_id":   nil,
			"source_message_ids": []any{},
			"affected_relations": anchor["affected_relations"],
			"affected_world":     anchor["affected_world"],
		}
		sceneDeltas = append(sceneDeltas, entry)
	}
	if len(sceneDeltas) > 8 {
		sceneDeltas = sceneDeltas[:8]
	}

	worldPressure := buildWorldPressure(storyPlan, director, pendingBeats, consumedBeats, lastTurn)

	return map[string]any{
		"status":                               ledgerStatus,
		"last_advanced_turn":                   lastAdvancedTurn,
		"last_validated_turn":                  lastValidatedTurn,
		"consumed_beats":                       consumedBeatsAny,
		"pending_beats":                        pendingBeatsAny,
		"invalidation_reason":                  nil,
		"ledger_policy_version":                "lw1h.v1",
		"ledger_mode":                          "deterministic_no_llm",
		"unresolved_tensions":                  unresolvedTensions,
		"consequences":                         consequences,
		"payoffs":                              []any{},
		"scene_deltas":                         sceneDeltas,
		"world_pressure_policy_version":        "lw1d.v1",
		"world_pressure":                       worldPressure,
		"continuity_precedence_policy_version": "lw1e.v1",
		"supporting_precedence_guard": map[string]any{
			"status":                                   "supporting_only",
			"supporting_only":                          true,
			"cannot_override_current_user_input":       true,
			"cannot_override_verified_direct_evidence": true,
			"precedence_ceiling":                       "below_current_user_input_and_verified_direct_evidence",
			"allowed_usage":                            []string{"continuity_hint", "narrative_support"},
			"disallowed_usage":                         []string{"truth_overwrite", "canonical_override"},
		},
		"compatibility_policy_version": "lw1f.v1",
		"compatibility_contract": map[string]any{
			"status":           "compatible",
			"targets":          []string{"chapter_summary", "arc_summary", "continuity_pack"},
			"shape_mode":       "additive_non_breaking",
			"consumer_safe":    true,
			"adapter_required": false,
		},
		"lifecycle_policy_version":      "lw1g.v1",
		"lifecycle_model":               lifecycleModel,
		"do_not_resolve_policy_version": "lw1h.v1",
		"do_not_resolve_guard":          doNotResolveGuard,
	}
}

func normalizeStoryLedgerLabel(raw any) string {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	return text
}

func lifecycleProfileForState(state string, lifecycleModel map[string]any) (int, int) {
	pressureMap := map[string]int{"latent": 1, "active": 2, "escalating": 3, "aftermath": 1, "resolved": 0, "dormant": 0}
	pressure := pressureMap[state]
	if pressure == 0 && state != "resolved" && state != "dormant" {
		pressure = 1
	}
	decayRules, _ := lifecycleModel["decay_rules"].(map[string]any)
	decay := 2
	if dr, ok := decayRules[state]; ok {
		if di, ok := dr.(int); ok {
			decay = di
		}
	}
	return pressure, decay
}

func buildLedgerAnchor(label string, storyPlanData map[string]any, directorData map[string]any) map[string]any {
	return map[string]any{
		"source_record_id":   nil,
		"source_message_ids": []any{},
		"affected_relations": deriveAnchorRelations(label, storyPlanData, directorData),
		"affected_world":     deriveAnchorWorld(label, storyPlanData, directorData),
	}
}

func deriveAnchorRelations(label string, storyPlanData map[string]any, directorData map[string]any) []any {
	candidates := []string{}
	for _, raw := range asStringSlice(directorData["focus_characters"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	for _, raw := range asStringSlice(storyPlanData["focus_characters"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	lowered := strings.ToLower(label)
	matched := []string{}
	for _, name := range candidates {
		if strings.Contains(lowered, strings.ToLower(name)) {
			matched = append(matched, name)
		}
	}
	if len(matched) > 0 {
		if len(matched) > 4 {
			matched = matched[:4]
		}
		out := make([]any, len(matched))
		for i, m := range matched {
			out[i] = m
		}
		return out
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]any, len(candidates))
	for i, c := range candidates {
		out[i] = c
	}
	return out
}

func deriveAnchorWorld(label string, storyPlanData map[string]any, directorData map[string]any) []any {
	candidates := []string{}
	for _, raw := range asStringSlice(directorData["world_guardrails"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	for _, raw := range asStringSlice(storyPlanData["guardrails"]) {
		text := normalizeStoryLedgerLabel(raw)
		if text != "" && !containsString(candidates, text) {
			candidates = append(candidates, text)
		}
	}
	currentArc := normalizeStoryLedgerLabel(storyPlanData["current_arc"])
	if currentArc != "" && !containsString(candidates, currentArc) {
		candidates = append(candidates, currentArc)
	}
	lowered := strings.ToLower(label)
	matched := []string{}
	for _, item := range candidates {
		if strings.Contains(lowered, strings.ToLower(item)) {
			matched = append(matched, item)
		}
	}
	if len(matched) > 0 {
		if len(matched) > 4 {
			matched = matched[:4]
		}
		out := make([]any, len(matched))
		for i, m := range matched {
			out[i] = m
		}
		return out
	}
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	out := make([]any, len(candidates))
	for i, c := range candidates {
		out[i] = c
	}
	return out
}

func buildWorldPressure(storyPlanData map[string]any, directorData map[string]any, pendingBeats []string, consumedBeats []string, lastTurn int) map[string]any {
	buckets := map[string][]map[string]any{
		"factions":          {},
		"regions":           {},
		"offscreen_threads": {},
		"public_pressure":   {},
	}
	appendBucket := func(bucket string, label string, source string, pressureState string) {
		if label == "" {
			return
		}
		for _, item := range buckets[bucket] {
			if item["label"] == label {
				return
			}
		}
		buckets[bucket] = append(buckets[bucket], map[string]any{
			"label":          label,
			"source":         source,
			"pressure_state": pressureState,
			"deterministic":  true,
		})
	}
	for _, raw := range asStringSlice(directorData["world_guardrails"]) {
		label := normalizeStoryLedgerLabel(raw)
		appendBucket(classifyWorldPressureBucket(label), label, "director.world_guardrails", "active")
	}
	for _, raw := range asStringSlice(storyPlanData["execution_notes"]) {
		label := normalizeStoryLedgerLabel(raw)
		appendBucket(classifyWorldPressureBucket(label), label, "story_plan.execution_notes", "escalating")
	}
	for _, beat := range pendingBeats {
		label := normalizeStoryLedgerLabel(beat)
		appendBucket("offscreen_threads", label, "story_plan.next_beats", "latent")
	}
	for _, beat := range consumedBeats {
		label := normalizeStoryLedgerLabel(beat)
		appendBucket("public_pressure", label, "director.resolved_outcomes", "aftermath")
	}
	return map[string]any{
		"status":            "structured_support",
		"factions":          mapSliceToAny(buckets["factions"])[:minInt(len(buckets["factions"]), 10)],
		"regions":           mapSliceToAny(buckets["regions"])[:minInt(len(buckets["regions"]), 10)],
		"offscreen_threads": mapSliceToAny(buckets["offscreen_threads"])[:minInt(len(buckets["offscreen_threads"]), 10)],
		"public_pressure":   mapSliceToAny(buckets["public_pressure"])[:minInt(len(buckets["public_pressure"]), 10)],
		"timeline": []any{
			map[string]any{
				"turn":              lastTurn,
				"marker":            "world_pressure_snapshot",
				"factions":          len(buckets["factions"]),
				"regions":           len(buckets["regions"]),
				"offscreen_threads": len(buckets["offscreen_threads"]),
				"public_pressure":   len(buckets["public_pressure"]),
			},
		},
	}
}

func isLongHorizonCandidate(label string, source string, guard map[string]any) bool {
	normalized := strings.TrimSpace(strings.ToLower(label))
	sourceKey := strings.TrimSpace(strings.ToLower(source))
	if normalized == "" {
		return false
	}
	tokens := []string{}
	if raw, ok := guard["long_horizon_tokens"].([]string); ok {
		for _, item := range raw {
			t := strings.TrimSpace(strings.ToLower(item))
			if t != "" {
				tokens = append(tokens, t)
			}
		}
	}
	for _, token := range tokens {
		if token != "" && strings.Contains(normalized, token) {
			return true
		}
	}
	protectedSources := map[string]bool{}
	if raw, ok := guard["protected_sources"].([]string); ok {
		for _, item := range raw {
			s := strings.TrimSpace(strings.ToLower(item))
			if s != "" {
				protectedSources[s] = true
			}
		}
	}
	return protectedSources[sourceKey]
}

func attachDoNotResolveFields(entry map[string]any, guard map[string]any, lastTurn int) {
	label := asString(entry["label"])
	source := asString(entry["source"])
	shouldProtect := isLongHorizonCandidate(label, source, guard)
	minTurnGap := 0
	if v, ok := guard["min_turn_gap"].(int); ok {
		minTurnGap = v
	}
	baseTurn := lastTurn
	if baseTurn < 0 {
		baseTurn = 0
	}
	if shouldProtect {
		entry["do_not_resolve_yet"] = true
		entry["resolve_guard_reason"] = "long_horizon_candidate"
		entry["resolve_earliest_turn"] = baseTurn + minTurnGap
	} else {
		entry["do_not_resolve_yet"] = false
		entry["resolve_guard_reason"] = nil
		entry["resolve_earliest_turn"] = nil
	}
}

func classifyWorldPressureBucket(label string) string {
	lowered := strings.ToLower(label)
	if strings.Contains(lowered, "faction") || strings.Contains(lowered, "guild") || strings.Contains(lowered, "clan") || strings.Contains(lowered, "house") || strings.Contains(lowered, "family") || strings.Contains(lowered, "group") {
		return "factions"
	}
	if strings.Contains(lowered, "region") || strings.Contains(lowered, "city") || strings.Contains(lowered, "village") || strings.Contains(lowered, "harbor") || strings.Contains(lowered, "capital") || strings.Contains(lowered, "area") || strings.Contains(lowered, "town") {
		return "regions"
	}
	if strings.Contains(lowered, "offscreen") || strings.Contains(lowered, "elsewhere") || strings.Contains(lowered, "distant") {
		return "offscreen_threads"
	}
	if strings.Contains(lowered, "public") || strings.Contains(lowered, "rumor") || strings.Contains(lowered, "panic") || strings.Contains(lowered, "trust") {
		return "public_pressure"
	}
	return "public_pressure"
}

func characterEventPriority(eventType string) int {
	switch eventType {
	case "relationship_shift":
		return 0
	case "personality_change":
		return 1
	case "appearance_change":
		return 2
	case "status_change":
		return 3
	default:
		return 4
	}
}

func buildStoryGuidanceSurface(storyPlan map[string]any, director map[string]any) map[string]any {
	pressureLevel := asString(director["pressure_level"])
	if pressureLevel == "" {
		pressureLevel = "steady"
	}
	currentArc := asString(storyPlan["current_arc"])
	narrativeGoal := asString(storyPlan["narrative_goal"])
	activeTensions := asStringSlice(storyPlan["active_tensions"])
	nextBeats := asStringSlice(storyPlan["next_beats"])
	anchors := asStringSlice(storyPlan["continuity_anchors"])
	focusCharacters := asStringSlice(storyPlan["focus_characters"])
	required := asStringSlice(director["required_outcomes"])
	forbidden := asStringSlice(director["forbidden_moves"])
	executionChecklist := asStringSlice(director["execution_checklist"])
	worldGuardrails := asStringSlice(director["world_guardrails"])
	personaGuardrails := asStringSlice(director["persona_guardrails"])
	sceneDrive := asString(director["scene_mandate"])

	ending := "End on a conservative continuation edge without forcing a hard scene jump."
	if pressureLevel == "strong" || pressureLevel == "critical" {
		ending = "End on a visible pressure beat without forcing a full resolution."
	} else if len(required) > 0 {
		ending = "Land at least one visible beat before ending, while keeping unresolved carry targets open."
	} else if len(nextBeats) > 0 || sceneDrive != "" {
		ending = "End on a clear continuation edge that preserves the active scene drive."
	}

	storyFrame := map[string]any{
		"stage_type":           "story_frame",
		"arc_focus":            currentArc,
		"narrative_drive":      narrativeGoal,
		"live_tensions":        nonNilSlice(activeTensions),
		"beat_queue":           nonNilSlice(nextBeats),
		"carry_threads":        nonNilSlice(anchors),
		"spotlight_characters": nonNilSlice(focusCharacters),
	}
	storyFrame["status"] = surfaceStatus(map[string]any{
		"arc_focus":       storyFrame["arc_focus"],
		"narrative_drive": storyFrame["narrative_drive"],
		"live_tensions":   storyFrame["live_tensions"],
		"beat_queue":      storyFrame["beat_queue"],
		"carry_threads":   storyFrame["carry_threads"],
	})

	turnDirectives := map[string]any{
		"stage_type":           "turn_directives",
		"scene_drive":          sceneDrive,
		"carry_targets":        nonNilSlice(required),
		"blocked_routes":       nonNilSlice(forbidden),
		"tempo_band":           pressureLevel,
		"handoff_edge":         ending,
		"turn_checklist":       nonNilSlice(limitStrings(executionChecklist, 4)),
		"voice_guardrails":     nonNilSlice(personaGuardrails),
		"setting_guardrails":   nonNilSlice(worldGuardrails),
		"spotlight_characters": nonNilSlice(focusCharacters),
	}
	turnDirectives["execution_contract"] = map[string]any{
		"must_hit":           limitStrings(required, 4),
		"forbidden":          limitStrings(forbidden, 4),
		"pacing_pressure":    pressureLevel,
		"ending_requirement": ending,
		"continuity_lock":    firstNonEmptyString(anchors),
	}
	failMode := "conservative_continuation"
	if pressureLevel == "strong" || pressureLevel == "critical" {
		failMode = "pressure_continuation_without_resolution"
	} else if len(required) > 0 {
		failMode = "carry_forward_without_forcing_resolution"
	} else if sceneDrive != "" || len(nextBeats) > 0 || narrativeGoal != "" {
		failMode = "scene_continuation_without_scene_jump"
	}
	turnDirectives["fail_mode"] = map[string]any{
		"mode":                             failMode,
		"allow_scene_jump":                 false,
		"allow_forced_resolution":          false,
		"respect_explicit_user_correction": true,
		"preserve_carry_targets":           len(required) > 0,
	}
	turnDirectives["status"] = surfaceStatus(map[string]any{
		"scene_drive":    turnDirectives["scene_drive"],
		"carry_targets":  turnDirectives["carry_targets"],
		"blocked_routes": turnDirectives["blocked_routes"],
		"handoff_edge":   turnDirectives["handoff_edge"],
	})

	statusInputs := map[string]any{
		"story_frame":     nil,
		"turn_directives": nil,
	}
	if storyFrame["status"] != "empty" {
		statusInputs["story_frame"] = storyFrame
	}
	if turnDirectives["status"] != "empty" {
		statusInputs["turn_directives"] = turnDirectives
	}
	status := surfaceStatus(statusInputs)

	return map[string]any{
		"surface_version": "sg14a.v1",
		"surface_type":    "story_guidance_surface",
		"status":          status,
		"story_frame":     storyFrame,
		"turn_directives": turnDirectives,
		"conflict_policy": map[string]any{
			"policy_version":                   "sg14a-conflict.v1",
			"current_user_input_wins":          true,
			"explicit_user_correction_wins":    true,
			"guidance_may_suggest":             true,
			"guidance_may_override_user_input": false,
			"on_conflict":                      "yield_to_current_user_input",
		},
		"precedence": map[string]any{
			"policy_version":          "sg14a.v1",
			"status":                  "fixed",
			"guidance_authority":      "subordinate",
			"higher_priority_sources": []string{"current_user_input", "explicit_user_correction", "hard_world_rule", "latest_direct_evidence", "canonical_truth_floor"},
			"disallowed_usage":        []string{"current_user_input_override", "explicit_user_correction_override", "hard_world_rule_bypass", "canonical_truth_floor_overwrite"},
			"precedence_note":         "Story guidance is a subordinate planning surface. Follow current user input, explicit user corrections, direct evidence, canonical truth, and hard world rules first.",
		},
	}
}

func firstNonEmptyString(items []string) string {
	for _, s := range items {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (s *Server) handleSessionsCompare(w http.ResponseWriter, r *http.Request) {
	if idsRaw := strings.TrimSpace(r.URL.Query().Get("session_ids")); idsRaw != "" {
		ids := []string{}
		for _, part := range strings.Split(idsRaw, ",") {
			if sid := strings.TrimSpace(part); sid != "" {
				ids = append(ids, sid)
			}
		}
		if len(ids) < 2 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "At least 2 session_ids are required."})
			return
		}
		if len(ids) > 2 {
			ids = ids[:2]
		}
		previewLimit, _ := strconv.Atoi(r.URL.Query().Get("preview_limit"))
		if previewLimit < 1 {
			previewLimit = 10
		}
		if previewLimit > 30 {
			previewLimit = 30
		}
		result := map[string]any{}
		for _, sid := range ids {
			ev := s.collectNarrativeEvidence(r.Context(), sid)
			result[sid] = sessionComparePayload(ev, previewLimit)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "sessions": result})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "At least 2 session_ids are required."})
}

func sessionComparePayload(ev narrativeEvidence, previewLimit int) map[string]any {
	logs := append([]store.ChatLog(nil), ev.ChatLogs...)
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex == logs[j].TurnIndex {
			return logs[i].ID > logs[j].ID
		}
		return logs[i].TurnIndex > logs[j].TurnIndex
	})
	if len(logs) > previewLimit*2 {
		logs = logs[:previewLimit*2]
	}
	logsPreview := []map[string]any{}
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		logsPreview = append(logsPreview, map[string]any{
			"id":         log.ID,
			"turn_index": log.TurnIndex,
			"role":       log.Role,
			"content":    truncateForPreview(log.Content, 200),
			"created_at": formatKSTTime(log.CreatedAt),
		})
	}

	memories := append([]store.Memory(nil), ev.Memories...)
	sort.SliceStable(memories, func(i, j int) bool { return memories[i].ID > memories[j].ID })
	memPreview := []map[string]any{}
	for _, mem := range memories {
		if len(memPreview) >= previewLimit {
			break
		}
		memPreview = append(memPreview, map[string]any{
			"id":           mem.ID,
			"summary_json": truncateForPreview(mem.SummaryJSON, 200),
			"importance":   mem.Importance,
			"created_at":   formatKSTTime(mem.CreatedAt),
		})
	}

	triples := append([]store.KGTriple(nil), ev.KGTriples...)
	sort.SliceStable(triples, func(i, j int) bool { return triples[i].ID > triples[j].ID })
	kgPreview := []map[string]any{}
	for _, triple := range triples {
		if len(kgPreview) >= previewLimit {
			break
		}
		kgPreview = append(kgPreview, map[string]any{
			"id":         triple.ID,
			"subject":    triple.Subject,
			"predicate":  triple.Predicate,
			"object":     triple.Object,
			"created_at": formatKSTTime(triple.CreatedAt),
		})
	}

	feedbackUp := 0
	feedbackDown := 0
	for _, feedback := range ev.CriticFeedback {
		switch strings.ToLower(feedback.FeedbackValue) {
		case "up":
			feedbackUp++
		case "down":
			feedbackDown++
		}
	}

	var lastActivity any
	for _, log := range ev.ChatLogs {
		if t := log.CreatedAt; !t.IsZero() {
			if lastActivity == nil || t.After(lastActivity.(time.Time)) {
				lastActivity = t
			}
		}
	}
	if t, ok := lastActivity.(time.Time); ok {
		lastActivity = formatKSTTime(t)
	}

	return map[string]any{
		"counts": map[string]any{
			"chat_logs":     len(ev.ChatLogs),
			"memories":      len(ev.Memories),
			"kg_triples":    len(ev.KGTriples),
			"audit_logs":    len(ev.AuditLogs),
			"feedback_up":   feedbackUp,
			"feedback_down": feedbackDown,
		},
		"last_activity":    lastActivity,
		"logs_preview":     logsPreview,
		"memories_preview": memPreview,
		"kg_triples":       kgPreview,
	}
}

func (s *Server) handleActiveStates(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	stateType := r.URL.Query().Get("state_type")
	items, err := s.Store.ListActiveStates(r.Context(), sid, stateType)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"states":          items,
		"count":           len(items),
	})
}

func (s *Server) handleCanonicalStateLayer(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	layerType := r.URL.Query().Get("layer_type")
	items, err := s.Store.ListCanonicalStateLayers(r.Context(), sid, layerType)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"layers":          items,
		"count":           len(items),
	})
}

func (s *Server) handleSessionState(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()

	snapshot, err := s.readSessionStateSnapshot(ctx, sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	activeStates := nonNilSlice(snapshot.ActiveStates)
	canonicalLayers := nonNilSlice(snapshot.CanonicalStateLayers)
	storylines := nonNilSlice(snapshot.Storylines)
	characters := nonNilSlice(snapshot.CharacterStates)
	worldRules := nonNilSlice(snapshot.WorldRules)
	pendingThreads := nonNilSlice(snapshot.PendingThreads)
	characterEvents := nonNilSlice(snapshot.CharacterEvents)

	storylines = visibleSessionStateStorylines(storylines)
	worldRules = visibleSessionStateWorldRules(worldRules)
	pendingThreads = continuityPendingThreads(pendingThreads, 0)
	referenceTurn := resolveCharacterReferenceTurn(activeStates, storylines, characters)
	recentText, recentKeywords := characterRecentMentionSignalFromLogs(snapshot.RecentChatLogs, referenceTurn)
	if !snapshot.SingleConnection && len(snapshot.RecentChatLogs) == 0 {
		recentText, recentKeywords = s.characterRecentMentionSignal(ctx, sid, referenceTurn)
	}
	characterItems := characterResponseItems(characters, characterEvents, referenceTurn, recentText, recentKeywords)
	omittedCharacters := characterOmittedCount(characters, characterEvents, referenceTurn, recentText, recentKeywords)
	storylineItems := storylineResponseItems(storylines, resolveStorylineReferenceTurn(storylines, ""))
	worldRuleItems := worldRuleResponseItems(worldRules, "")
	pendingThreadItems := pendingThreadResponseItems(pendingThreads)

	sectionMeta := map[string]any{
		"active_states":         sessionStateMetaForActiveStates(activeStates),
		"storylines":            sessionStateMetaForStorylines(storylines),
		"characters":            sessionStateMetaForCharacterItems(characterItems),
		"world_rules":           sessionStateMetaForWorldRules(worldRules),
		"pending_threads":       sessionStateMetaForPendingThreads(pendingThreads),
		"continuity_hooks":      sessionStateMetaForPendingThreads(pendingThreads),
		"chapter_summaries":     map[string]any{"count": 0, "last_turn": nil, "updated_at": nil, "ready": false},
		"canonical_state_layer": sessionStateMetaForCanonicalLayers(canonicalLayers),
	}
	guidanceSnapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	sectionMeta["guidance_snapshot"] = map[string]any{
		"state_status": guidanceSnapshot["state_status"],
		"last_turn":    guidanceSnapshot["last_turn"],
		"ready":        guidanceSnapshot["state_status"] == "active",
	}
	snapshotStatus := sessionStateSnapshotStatus(len(activeStates), len(storylineItems), len(characterItems), len(worldRuleItems), len(pendingThreadItems))
	warnings := []any{}
	if omittedCharacters > 0 {
		warnings = append(warnings, fmt.Sprintf("%d transient descriptor character(s) omitted from characters section.", omittedCharacters))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active_states":         activeStates,
		"canonical_state_layer": canonicalLayers,
		"chapter_summaries":     []any{},
		"characters":            characterItems,
		"chat_session_id":       sid,
		"generated_at":          generatedAt(),
		"guidance_snapshot":     guidanceSnapshot,
		"pending_threads":       pendingThreadItems,
		"continuity_hooks":      pendingThreadItems,
		"section_meta":          sectionMeta,
		"snapshot_status":       snapshotStatus,
		"status":                "ok",
		"storylines":            storylineItems,
		"warnings":              warnings,
		"world_rules":           worldRuleItems,
	})
}

func (s *Server) readSessionStateSnapshot(ctx context.Context, sid string) (store.SessionStateSnapshot, error) {
	if s.Store == nil {
		return store.SessionStateSnapshot{}, nil
	}
	if reader, ok := s.Store.(store.SessionStateSnapshotReader); ok {
		snapshot, err := reader.ReadSessionStateSnapshot(ctx, sid)
		if err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				return store.SessionStateSnapshot{}, nil
			}
			return store.SessionStateSnapshot{}, err
		}
		if snapshot != nil {
			return *snapshot, nil
		}
	}
	activeStates, _ := s.Store.ListActiveStates(ctx, sid, "")
	canonicalLayers, _ := s.Store.ListCanonicalStateLayers(ctx, sid, "")
	storylines, _ := s.Store.ListStorylines(ctx, sid)
	characters, _ := s.Store.ListCharacterStates(ctx, sid)
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characterEvents, _ := s.Store.ListCharacterEvents(ctx, sid, "")
	return store.SessionStateSnapshot{
		ActiveStates:         activeStates,
		CanonicalStateLayers: canonicalLayers,
		Storylines:           storylines,
		CharacterStates:      characters,
		WorldRules:           worldRules,
		PendingThreads:       pendingThreads,
		CharacterEvents:      characterEvents,
		SingleConnection:     false,
	}, nil
}

func visibleSessionStateStorylines(items []store.Storyline) []store.Storyline {
	out := make([]store.Storyline, 0, len(items))
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].LastTurn != out[j].LastTurn {
			return out[i].LastTurn > out[j].LastTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func visibleSessionStateWorldRules(items []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(items))
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		for _, cmp := range []int{strings.Compare(out[i].Scope, out[j].Scope), strings.Compare(out[i].Category, out[j].Category), strings.Compare(out[i].Key, out[j].Key)} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func resolveCharacterReferenceTurn(activeStates []store.ActiveState, storylines []store.Storyline, characters []store.CharacterState) int {
	ref := 0
	for _, item := range activeStates {
		if item.TurnIndex > ref {
			ref = item.TurnIndex
		}
	}
	for _, item := range storylines {
		if item.LastTurn > ref {
			ref = item.LastTurn
		}
		if item.LastEvidenceTurn > ref {
			ref = item.LastEvidenceTurn
		}
	}
	for _, item := range characters {
		if item.TurnIndex > ref {
			ref = item.TurnIndex
		}
	}
	return ref
}

func sessionStateSnapshotStatus(counts ...int) string {
	readyCount := 0
	for _, count := range counts {
		if count > 0 {
			readyCount++
		}
	}
	if readyCount == len(counts) && len(counts) > 0 {
		return "ready"
	}
	if readyCount > 0 {
		return "partial"
	}
	return "empty"
}

func sessionStateMeta(count int, lastTurn int, updatedAt time.Time) map[string]any {
	var last any
	if lastTurn > 0 {
		last = lastTurn
	}
	return map[string]any{
		"count":      count,
		"last_turn":  last,
		"updated_at": nullableTime(updatedAt),
		"ready":      count > 0,
	}
}

func sessionStateMetaForActiveStates(items []store.ActiveState) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
		if item.CreatedAt.After(updated) {
			updated = item.CreatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForCanonicalLayers(items []store.CanonicalStateLayer) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
		if item.CreatedAt.After(updated) {
			updated = item.CreatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForStorylines(items []store.Storyline) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.LastTurn > maxTurn {
			maxTurn = item.LastTurn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForCharacterItems(items []map[string]any) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if turn, ok := mapIntValue(item, "turn_index"); ok && turn > maxTurn {
			maxTurn = turn
		}
		if t, ok := mapTimeValue(item, "updated_at"); ok && t.After(updated) {
			updated = t
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForWorldRules(items []store.WorldRule) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.SourceTurn > maxTurn {
			maxTurn = item.SourceTurn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForPendingThreads(items []store.PendingThread) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		turn := item.LastSeenTurn
		if turn == 0 {
			turn = item.SourceTurn
		}
		if turn == 0 {
			turn = item.ResolvedTurn
		}
		if turn > maxTurn {
			maxTurn = turn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func mapIntValue(item map[string]any, key string) (int, bool) {
	switch typed := item[key].(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case json.Number:
		i, err := typed.Int64()
		return int(i), err == nil
	}
	return 0, false
}

func mapTimeValue(item map[string]any, key string) (time.Time, bool) {
	raw, ok := item[key]
	if !ok || raw == nil {
		return time.Time{}, false
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Server) handleContinuityPack(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	episodes, _ := s.Store.ListEpisodeSummaries(ctx, sid, 1, 0, 0)
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	characterEvents, _ := s.Store.ListCharacterEvents(ctx, sid, "")

	storylines = nonNilSlice(storylines)
	episodes = nonNilSlice(episodes)
	worldRules = nonNilSlice(worldRules)
	storylineSelection := selectStorylinesForSupervisor(storylines, nil, 3)
	activeStorylines := storylineResponseItems(selectedStorylineItems(storylineSelection), storylineSelection.ReferenceTurn)
	relationshipShifts := continuityRelationshipShifts(characterEvents, 5)
	pendingThreads = continuityPendingThreads(pendingThreads, 5)
	pendingThreadItems := pendingThreadResponseItems(pendingThreads)
	if len(worldRules) > 8 {
		worldRules = worldRules[:8]
	}

	var latestEpisode any
	if len(episodes) > 0 {
		latestEpisode = episodes[0]
	}

	packStatus := "empty"
	if len(activeStorylines) > 0 || len(relationshipShifts) > 0 || len(pendingThreadItems) > 0 || len(episodes) > 0 || len(worldRules) > 0 {
		packStatus = "ready"
	}

	warnings := []any{}
	if len(activeStorylines) == 0 && len(relationshipShifts) == 0 && len(episodes) == 0 && len(worldRules) == 0 {
		warnings = append(warnings, "No continuity source data available for this session yet.")
	}
	if dropped := len(storylineSelection.Dropped); dropped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d storyline(s) omitted by continuity freshness gate.", dropped))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active_storylines":   activeStorylines,
		"chat_session_id":     sid,
		"generated_at":        generatedAt(),
		"latest_episode":      latestEpisode,
		"pack_status":         packStatus,
		"pending_threads":     pendingThreadItems,
		"relationship_shifts": relationshipShifts,
		"section_status": map[string]any{
			"active_storylines":   map[string]any{"ready": true, "count": len(activeStorylines), "note": "Selected with storyline freshness gate"},
			"relationship_shifts": map[string]any{"ready": true, "count": len(relationshipShifts), "note": "Recent relationship_shift events"},
			"pending_threads":     map[string]any{"ready": true, "count": len(pendingThreadItems), "note": "Open/paused hooks (pinned first, suppressed excluded)"},
			"continuity_hooks":    map[string]any{"ready": true, "count": len(pendingThreadItems), "note": "Alias for pending_threads; open/paused hooks (pinned first, suppressed excluded)"},
			"latest_episode":      map[string]any{"ready": true, "count": len(episodes), "note": "Latest generated episode summary"},
			"world_constraints":   map[string]any{"ready": true, "count": len(worldRules), "note": "Current world-rule snapshot"},
		},
		"skeleton_only":     false,
		"status":            "ok",
		"warnings":          warnings,
		"world_constraints": worldRules,
	})
}

func continuityRelationshipShifts(events []store.CharacterEvent, limit int) []map[string]any {
	items := []store.CharacterEvent{}
	for _, item := range nonNilSlice(events) {
		if strings.TrimSpace(item.EventType) != "relationship_shift" {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TurnIndex != items[j].TurnIndex {
			return items[i].TurnIndex > items[j].TurnIndex
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID > items[j].ID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"character_name":  item.CharacterName,
			"turn_index":      nullablePositiveInt(item.TurnIndex),
			"event_type":      item.EventType,
			"details_json":    nullableString(item.DetailsJSON),
			"created_at":      formatKSTTime(item.CreatedAt),
		})
	}
	return out
}

func continuityPendingThreads(items []store.PendingThread, limit int) []store.PendingThread {
	out := []store.PendingThread{}
	for _, item := range nonNilSlice(items) {
		status := strings.TrimSpace(item.Status)
		if status != "" && status != "open" && status != "paused" {
			continue
		}
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		left := out[i].LastSeenTurn
		if left == 0 {
			left = out[i].SourceTurn
		}
		right := out[j].LastSeenTurn
		if right == 0 {
			right = out[j].SourceTurn
		}
		if left != right {
			return left > right
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Server) handlePendingThreads(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := s.Store.ListPendingThreads(r.Context(), sid, status)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.PendingThread{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	statusFilter := status
	if statusFilter == "" {
		statusFilter = "open+paused"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"hooks":           pendingThreadResponseItems(items),
		"count":           len(items),
		"status_filter":   statusFilter,
	})
}

func (s *Server) handleContinuityHooks(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := s.Store.ListPendingThreads(r.Context(), sid, status)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.PendingThread{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	statusFilter := status
	if statusFilter == "" {
		statusFilter = "open+paused"
	}
	hooks := pendingThreadResponseItems(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"items":           hooks,
		"hooks":           hooks,
		"count":           len(hooks),
		"fetched":         true,
		"status_filter":   statusFilter,
	})
}

func pendingThreadResponseItems(items []store.PendingThread) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		meta := pendingThreadMetadata(item)
		threadType := firstNonEmpty(item.ThreadType, metadataString(meta, "thread_type"), item.HookType)
		title := firstNonEmpty(item.Title, metadataString(meta, "title"), item.Description, item.ThreadKey)
		lastSeen := item.LastSeenTurn
		if lastSeen == 0 {
			if v, ok := metadataInt(meta, "last_seen_turn"); ok {
				lastSeen = v
			}
		}
		if lastSeen == 0 {
			lastSeen = item.ResolvedTurn
		}
		confidence := item.Confidence
		if confidence == 0 {
			if v, ok := metadataFloat(meta, "confidence"); ok {
				confidence = v
			}
		}
		if confidence == 0 && item.Priority > 0 {
			confidence = float64(item.Priority) / 100.0
		}
		detailsJSON := firstNonEmpty(item.DetailsJSON, metadataJSONText(meta, "details_json"), metadataJSONText(meta, "details"), item.HookMetadataJSON)
		resolutionNote := firstNonEmpty(item.ResolutionNote, metadataString(meta, "resolution_note"))
		out = append(out, map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"thread_type":     threadType,
			"hook_type":       firstNonEmpty(item.HookType, threadType),
			"thread_key":      item.ThreadKey,
			"title":           title,
			"description":     item.Description,
			"status":          item.Status,
			"owner":           firstNonEmpty(item.Owner, metadataString(meta, "owner")),
			"target":          firstNonEmpty(item.Target, metadataString(meta, "target")),
			"source_turn":     nullablePositiveInt(item.SourceTurn),
			"last_seen_turn":  nullablePositiveInt(lastSeen),
			"confidence":      confidence,
			"details_json":    detailsJSON,
			"resolution_note": nullableString(resolutionNote),
			"pinned":          item.Pinned,
			"suppressed":      item.Suppressed,
			"user_corrected":  item.UserCorrected,
			"created_at":      formatKSTTime(item.CreatedAt),
			"updated_at":      formatKSTTime(item.UpdatedAt),
		})
	}
	return out
}

func pendingThreadMetadata(item store.PendingThread) map[string]any {
	raw := strings.TrimSpace(firstNonEmpty(item.HookMetadataJSON, item.DetailsJSON))
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	switch typed := meta[key].(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func metadataFloat(meta map[string]any, key string) (float64, bool) {
	if meta == nil {
		return 0, false
	}
	return storylineFloatPatchValue(meta[key])
}

func metadataInt(meta map[string]any, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	return storylineIntPatchValue(meta[key])
}

func metadataJSONText(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	val, ok := meta[key]
	if !ok || val == nil {
		return ""
	}
	if text, ok := val.(string); ok {
		return strings.TrimSpace(text)
	}
	return mustCompactJSON(val)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nullablePositiveInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func (s *Server) handleActiveScopeGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()
	item, source, err := s.resolveActiveScope(ctx, sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activeScopeResponse(sid, item, source))
}

func (s *Server) handleMomentumPacket(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characterStates, _ := s.Store.ListCharacterStates(ctx, sid)
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)
	characterStates = nonNilSlice(characterStates)

	nextPressure := momentumNextPressure(storylines)
	payoffCandidates := momentumPayoffCandidates(storylines, pendingThreads, characterStates)
	tensionToReuse := momentumTensionToReuse(pendingThreads)
	beatsToAvoid := momentumBeatsToAvoid(storylines)
	totalItems := len(nextPressure) + len(payoffCandidates) + len(tensionToReuse) + len(beatsToAvoid)

	warnings := []any{}
	if totalItems == 0 && len(storylines) == 0 && len(pendingThreads) == 0 && len(characterStates) == 0 {
		warnings = append(warnings, "No active storylines, open hooks, or relationship states found for this session.")
	}

	packetStatus := "partial"
	if totalItems == 0 {
		packetStatus = "empty"
	} else if totalItems >= 4 {
		packetStatus = "ready"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"beats_to_avoid":    beatsToAvoid,
		"chat_session_id":   sid,
		"generated_at":      generatedAt(),
		"next_pressure":     nextPressure,
		"packet_status":     packetStatus,
		"payoff_candidates": payoffCandidates,
		"status":            "ok",
		"tension_to_reuse":  tensionToReuse,
		"warnings":          warnings,
	})
}

func momentumNextPressure(storylines []store.Storyline) []map[string]any {
	active := activeNarrativeStorylines(storylines)
	out := []map[string]any{}
	for _, item := range active {
		tensions := parseStorylineListJSON(item.OngoingTensionsJSON)
		if len(tensions) == 0 {
			continue
		}
		out = append(out, momentumItem(tensions[0], "storyline", item.ID, item.Name, 100+item.EvidenceCount, map[string]any{
			"last_turn": item.LastTurn,
			"reason":    "active_storyline_ongoing_tension",
		}))
		if len(out) >= 2 {
			return out
		}
	}
	for _, item := range active {
		if len(out) >= 2 {
			break
		}
		out = append(out, momentumItem(firstNonEmpty(item.CurrentContext, item.Name), "storyline", item.ID, item.Name, 50+item.EvidenceCount, map[string]any{
			"last_turn": item.LastTurn,
			"reason":    "latest_active_storyline_fallback",
		}))
	}
	return out
}

func momentumPayoffCandidates(storylines []store.Storyline, pendingThreads []store.PendingThread, characterStates []store.CharacterState) []map[string]any {
	active := activeNarrativeStorylines(storylines)
	out := []map[string]any{}
	for _, item := range active {
		if item.EvidenceCount < 3 || item.Confidence < 0.7 {
			continue
		}
		label := firstNonEmpty(lastStorylineListItem(item.KeyPointsJSON), item.CurrentContext, item.Name)
		out = append(out, momentumItem(label, "storyline", item.ID, item.Name, 90+item.EvidenceCount, map[string]any{
			"confidence":     item.Confidence,
			"evidence_count": item.EvidenceCount,
			"reason":         "high_confidence_storyline_payoff",
		}))
		if len(out) >= 2 {
			break
		}
	}
	for _, item := range momentumRelationshipPayoffCandidates(characterStates) {
		if len(out) >= 4 {
			return out
		}
		out = append(out, item)
	}
	hooks := openNarrativeThreadsOldestFirst(pendingThreads)
	for _, hook := range hooks {
		if len(out) >= 4 {
			break
		}
		out = append(out, momentumItem(pendingThreadNarrativeLabel(hook), "pending_thread", hook.ID, pendingThreadTitle(hook), 60+hook.Priority, map[string]any{
			"last_seen_turn": firstPositiveInt(hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn),
			"reason":         "older_open_hook_payoff",
		}))
	}
	return out
}

func momentumRelationshipPayoffCandidates(characterStates []store.CharacterState) []map[string]any {
	candidates := []map[string]any{}
	seen := map[string]bool{}
	states := append([]store.CharacterState{}, nonNilSlice(characterStates)...)
	sort.SliceStable(states, func(i, j int) bool {
		if states[i].TurnIndex != states[j].TurnIndex {
			return states[i].TurnIndex > states[j].TurnIndex
		}
		return states[i].ID > states[j].ID
	})
	for _, character := range states {
		lane := buildCharacterRelationshipLane(character)
		rawItems, _ := lane["items"].([]any)
		for _, raw := range rawItems {
			relation, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			target := cleanShadowText(relation["target"], 80)
			summary := cleanShadowText(relation["summary_text"], 180)
			if target == "" || summary == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(character.CharacterName) + "\x00" + target + "\x00" + summary)
			if seen[key] {
				continue
			}
			seen[key] = true
			sourceName := strings.TrimSpace(character.CharacterName)
			if sourceName == "" {
				sourceName = "relationship"
			}
			sourceName = sourceName + " -> " + target
			priority := 75 + minInt(maxInt(character.TurnIndex, 0), 10)
			if displayPriority, ok := relation["display_priority"].(int); ok && displayPriority == 0 {
				priority += 8
			}
			candidates = append(candidates, momentumItem(summary, "relationship", character.ID, sourceName, priority, map[string]any{
				"character_name": character.CharacterName,
				"target":         target,
				"turn_index":     nullablePositiveInt(character.TurnIndex),
				"reason":         "relationship_state_payoff",
			}))
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return momentumPriority(candidates[i]) > momentumPriority(candidates[j])
	})
	if len(candidates) > 3 {
		return candidates[:3]
	}
	return candidates
}

func momentumTensionToReuse(pendingThreads []store.PendingThread) []map[string]any {
	hooks := openNarrativeThreadsOldestFirst(pendingThreads)
	out := []map[string]any{}
	for _, hook := range hooks {
		out = append(out, momentumItem(pendingThreadNarrativeLabel(hook), "pending_thread", hook.ID, pendingThreadTitle(hook), 70+hook.Priority, map[string]any{
			"last_seen_turn": firstPositiveInt(hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn),
			"reason":         "oldest_open_or_paused_hook",
		}))
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func momentumBeatsToAvoid(storylines []store.Storyline) []map[string]any {
	type duplicateBeat struct {
		label string
		count int
		ids   []int64
	}
	seen := map[string]*duplicateBeat{}
	for _, item := range visibleSessionStateStorylines(storylines) {
		for _, beat := range parseStorylineListJSON(item.KeyPointsJSON) {
			key := normalizeMomentumBeatKey(beat)
			if key == "" {
				continue
			}
			entry := seen[key]
			if entry == nil {
				entry = &duplicateBeat{label: beat}
				seen[key] = entry
			}
			entry.count++
			entry.ids = append(entry.ids, item.ID)
		}
	}
	out := []map[string]any{}
	for _, entry := range seen {
		if entry.count < 2 {
			continue
		}
		out = append(out, momentumItem(entry.label, "storyline_key_point", 0, entry.label, 40+entry.count, map[string]any{
			"duplicate_count": entry.count,
			"storyline_ids":   entry.ids,
			"reason":          "duplicate_key_point",
		}))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return momentumPriority(out[i]) > momentumPriority(out[j])
	})
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func momentumItem(label, sourceType string, sourceID int64, sourceName string, priority int, extra map[string]any) map[string]any {
	item := map[string]any{
		"label":       truncateRunes(strings.TrimSpace(label), 180),
		"source_type": sourceType,
		"source_id":   sourceID,
		"source_name": truncateRunes(strings.TrimSpace(sourceName), 120),
		"priority":    priority,
	}
	for key, value := range extra {
		item[key] = value
	}
	if item["label"] == "" {
		item["label"] = item["source_name"]
	}
	return item
}

func momentumPriority(item map[string]any) int {
	if val, ok := item["priority"].(int); ok {
		return val
	}
	return 0
}

func openNarrativeThreadsOldestFirst(items []store.PendingThread) []store.PendingThread {
	out := openNarrativeThreads(items)
	sort.SliceStable(out, func(i, j int) bool {
		left := firstPositiveInt(out[i].LastSeenTurn, out[i].SourceTurn, out[i].CreatedTurn)
		right := firstPositiveInt(out[j].LastSeenTurn, out[j].SourceTurn, out[j].CreatedTurn)
		if left != right {
			return left < right
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func lastStorylineListItem(raw string) string {
	items := parseStorylineListJSON(raw)
	if len(items) == 0 {
		return ""
	}
	return items[len(items)-1]
}

func normalizeMomentumBeatKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	return string(runes)
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *Server) handleNarrativeControlGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characters, _ := s.Store.ListCharacterStates(ctx, sid)
	activeStates, _ := s.Store.ListActiveStates(ctx, sid, "")
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)
	characters = nonNilSlice(characters)
	activeStates = nonNilSlice(activeStates)
	worldRules = nonNilSlice(worldRules)

	dbLastTurn := maxNarrativeEvidenceTurn(storylines, pendingThreads, activeStates, characters)

	var storyPlan map[string]any
	var director map[string]any
	var stateStatus string
	var warnings []any
	var fromCache bool

	// K-2b: try cached GuidancePlanState
	if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
		cached, _ := gps.GetGuidancePlanState(ctx, sid)
		if cached != nil && cached.StateStatus != "empty" && cached.StoryPlanJSON != "" && cached.DirectorJSON != "" {
			isUserPatched := cached.StateStatus == "user_patched"
			forwardFresh := cached.LastTurn >= max(0, dbLastTurn-3)
			backwardFresh := dbLastTurn >= max(0, cached.LastTurn-1)
			cacheFresh := isUserPatched || (forwardFresh && backwardFresh)
			if cacheFresh {
				var cachedStoryPlan map[string]any
				var cachedDirector map[string]any
				if err := json.Unmarshal([]byte(cached.StoryPlanJSON), &cachedStoryPlan); err == nil {
					if err := json.Unmarshal([]byte(cached.DirectorJSON), &cachedDirector); err == nil {
						fromCache = true
						storyPlan = cachedStoryPlan
						director = cachedDirector
						stateStatus = strings.TrimSpace(cached.StateStatus)
						if stateStatus == "" {
							stateStatus = "partial"
						}
						if cached.WarningsJSON != "" {
							_ = json.Unmarshal([]byte(cached.WarningsJSON), &warnings)
						}
					}
				}
			}
		}
	}

	if !fromCache {
		warnings = []any{}
		if len(storylines) == 0 && len(pendingThreads) == 0 {
			warnings = append(warnings, "No active storylines or open hooks found. Returning skeleton state.")
		}

		storyPlan = buildStoryPlanSnapshot(storylines, pendingThreads, characters, worldRules, dbLastTurn)
		director = buildDirectorSnapshot(storylines, pendingThreads, characters, worldRules, dbLastTurn)
		stateStatus = "partial"
		if len(storylines) == 0 && len(pendingThreads) == 0 && len(characters) == 0 && len(activeStates) == 0 && len(worldRules) == 0 {
			stateStatus = "skeleton"
		} else if hasNarrativePlanSignal(storyPlan) && hasDirectorSignal(director) {
			stateStatus = "ready"
		}

		// K-2c: conservative merge with previous cache when same arc
		if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
			prev, _ := gps.GetGuidancePlanState(ctx, sid)
			if prev != nil && prev.LastTurn > 0 {
				var prevPlan map[string]any
				var prevDirector map[string]any
				_ = json.Unmarshal([]byte(prev.StoryPlanJSON), &prevPlan)
				_ = json.Unmarshal([]byte(prev.DirectorJSON), &prevDirector)
				cachedArc := strings.TrimSpace(asString(prevPlan["current_arc"]))
				currentArc := strings.TrimSpace(asString(storyPlan["current_arc"]))
				if cachedArc != "" && (currentArc == cachedArc || currentArc == "") {
					oldBeats := asAnySlice(prevPlan["next_beats"])
					newBeats := asAnySlice(storyPlan["next_beats"])
					if len(oldBeats) > 0 {
						storyPlan["next_beats"] = unionAnyStringSlices(newBeats, oldBeats)
					}
					oldAnchors := asAnySlice(prevPlan["continuity_anchors"])
					newAnchors := asAnySlice(storyPlan["continuity_anchors"])
					if len(oldAnchors) > 0 {
						storyPlan["continuity_anchors"] = unionAnyStringSlices(newAnchors, oldAnchors)
					}
				}
				director = mergeDirectorPrev(director, prevDirector)
			}
		}

		// K-2b: non-fatal upsert
		if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
			spJSON, _ := json.Marshal(storyPlan)
			dirJSON, _ := json.Marshal(director)
			warnJSON, _ := json.Marshal(warnings)
			item := &store.GuidancePlanState{
				ChatSessionID: sid,
				StoryPlanJSON: string(spJSON),
				DirectorJSON:  string(dirJSON),
				StateStatus:   stateStatus,
				LastTurn:      dbLastTurn,
				WarningsJSON:  string(warnJSON),
				UpdatedAt:     time.Now().UTC(),
			}
			_ = gps.UpsertGuidancePlanState(ctx, item)
		}
	}

	lastTurnValue := any(nil)
	if dbLastTurn > 0 {
		lastTurnValue = dbLastTurn
	}
	compactHistory, compactMeta := buildNarrativeCompactHistory(storyPlan, director, storylines, pendingThreads)

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id":      sid,
		"compact_history":      compactHistory,
		"compact_history_meta": compactMeta,
		"director":             director,
		"generated_at":         generatedAt(),
		"last_advanced_turn":   lastTurnValue,
		"last_validated_turn":  lastTurnValue,
		"progression_ledger":   buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, dbLastTurn),
		"skeleton_only":        stateStatus == "skeleton",
		"state_status":         stateStatus,
		"status":               "ok",
		"story_guidance":       buildStoryGuidanceSurface(storyPlan, director),
		"story_plan":           storyPlan,
		"warnings":             warnings,
	})
}

func (s *Server) handleSessionsGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}

	rollbackStore, hasRollback := s.Store.(store.RollbackStore)
	if !hasRollback || !s.usesShadowWriteStore() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "ok",
			"source":           "shadow",
			"chat_session_id":  sid,
			"deleted":          false,
			"mutation_enabled": false,
			"note":             "session delete is a shadow plan; no mutations performed",
		})
		return
	}

	ctx := r.Context()
	if err := rollbackStore.DeleteSession(ctx, sid); err != nil {
		writeInternalError(w, err.Error())
		return
	}

	vectorCleanup := map[string]any{
		"attempted": false,
		"ok":        true,
		"error":     nil,
	}
	if s.Vector != nil {
		vectorCleanup["attempted"] = true
		if err := s.Vector.DeleteSession(ctx, sid); err != nil {
			if errors.Is(err, vector.ErrNotEnabled) {
				vectorCleanup["ok"] = true
				vectorCleanup["error"] = "vector_not_enabled"
				vectorCleanup["warning"] = "vector store is not enabled"
			} else {
				vectorCleanup["ok"] = false
				vectorCleanup["error"] = err.Error()
			}
		}
	}

	status := "ok"
	if vectorCleanup["ok"] == false {
		status = "partial_error"
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "session_delete",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session deleted",
		DetailsJSON:   mustCompactJSON(map[string]any{"vector_cleanup": vectorCleanup, "status": status}),
		Source:        s.storeWriteSource(),
		CreatedAt:     time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"chat_session_id":  sid,
		"deleted":          true,
		"mutation_enabled": true,
		"vector_cleanup":   vectorCleanup,
		"note":             "session deleted",
	})
}

func (s *Server) handleSessionGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
}

// Session write guards (R2)

func (s *Server) handleActiveScopePatch(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	activeStore, ok := s.Store.(store.ActiveScopeStore)
	if !ok {
		writeShadowGuard(w, "PATCH /session/{chat_session_id}/active-scope")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	activeScope := strings.TrimSpace(extractionStringFromAny(payload["active_scope"]))
	if activeScope == "" {
		activeScope = "root"
	}
	if !isValidWorldRuleScope(activeScope) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "error",
			"detail": "active_scope must be one of [root region location faction system session]",
		})
		return
	}
	scopeName := strings.TrimSpace(extractionStringFromAny(payload["scope_name"]))
	item := &store.SessionActiveScope{
		ChatSessionID: sid,
		ActiveScope:   activeScope,
		ScopeName:     scopeName,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := activeStore.UpsertActiveScope(r.Context(), item); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /session/{chat_session_id}/active-scope")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activeScopeResponse(sid, item, "store"))
}

func (s *Server) handleDirectorPatch(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}

	gps, ok := s.Store.(store.GuidancePlanStateStore)
	if !ok {
		writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
		return
	}

	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}

	ctx := r.Context()
	cached, err := gps.GetGuidancePlanState(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	var director map[string]any
	if cached != nil && cached.DirectorJSON != "" {
		_ = json.Unmarshal([]byte(cached.DirectorJSON), &director)
	}
	if director == nil {
		director = map[string]any{}
	}

	// K-3d: apply allowed patchable fields
	patchable := []string{
		"scene_mandate", "required_outcomes", "forbidden_moves",
		"pressure_level", "resolved_outcomes", "expired_forbidden",
		"execution_checklist", "persona_guardrails", "world_guardrails",
		"focus_characters",
	}
	for _, key := range patchable {
		if v, ok := payload[key]; ok {
			director[key] = v
		}
	}

	// Build updated state preserving story plan and warnings
	dirJSON, _ := json.Marshal(director)
	item := &store.GuidancePlanState{
		ChatSessionID: sid,
		DirectorJSON:  string(dirJSON),
		StateStatus:   "user_patched",
		LastTurn:      0,
	}
	if cached != nil {
		item.StoryPlanJSON = cached.StoryPlanJSON
		item.WarningsJSON = cached.WarningsJSON
		item.LastTurn = cached.LastTurn
	}

	if err := gps.UpsertGuidancePlanState(ctx, item); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"director":        director,
		"patched":         true,
		"state_status":    "user_patched",
	})
}

// Storyline: R1 read, R2 write

func (s *Server) handleStorylinesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListStorylines(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	referenceTurn := resolveStorylineReferenceTurn(items, r.URL.Query().Get("current_turn"))
	storylines := storylineResponseItems(items, referenceTurn)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"storylines":      storylines,
		"count":           len(storylines),
		"reference_turn":  nullableIntPtr(referenceTurn),
	})
}

func (s *Server) handleStorylinePatch(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchStoryline(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /storylines/{storyline_id}")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeStorylinePatchPayload(payload, false)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchStoryline(r.Context(), storylineID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /storylines/{storyline_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"storyline_id":   storylineID,
		"updated_fields": updatedFields,
	}
	updatedValues := make(map[string]any)
	for _, key := range updatedFields {
		if val, exists := updates[key]; exists {
			updatedValues[key] = val
		}
	}
	if len(updatedValues) > 0 {
		resp["updated_values"] = updatedValues
	}
	for _, key := range []string{"confidence", "evidence_count", "last_evidence_turn"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStorylineTrust(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchStorylineTrust(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /storylines/{storyline_id}/trust")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeStorylineTrustPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchStorylineTrust(r.Context(), storylineID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /storylines/{storyline_id}/trust")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"storyline_id":   storylineID,
		"updated_fields": updatedFields,
	}
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStorylineDelete(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		DeleteStoryline(context.Context, int64) error
	})
	if !ok {
		writeShadowGuard(w, "DELETE /storylines/{storyline_id}")
		return
	}
	if err := mutator.DeleteStoryline(r.Context(), storylineID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /storylines/{storyline_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"deleted_id": storylineID,
	})
}

func (s *Server) handleStorylinesSync(w http.ResponseWriter, r *http.Request) {
	saver, ok := s.Store.(interface {
		SaveStoryline(context.Context, *store.Storyline) error
	})
	if !ok {
		writeShadowGuard(w, "POST /storylines/sync")
		return
	}
	var req storylineSyncRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode != "apply" {
		mode = "dry_run"
	}
	candidates := parseStorylineCandidatesFromSupervisor(req.SupervisorResult)
	validated := make([]storylineSyncCandidate, 0, len(candidates))
	validationErrors := make([]map[string]any, 0)
	for _, candidate := range candidates {
		normalized, errs := normalizeStorylineSyncCandidate(candidate)
		if len(errs) > 0 {
			validationErrors = append(validationErrors, map[string]any{
				"name":   candidate.Name,
				"errors": errs,
			})
			continue
		}
		validated = append(validated, normalized)
	}
	if mode == "dry_run" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"mode":              "dry_run",
			"parsed_count":      len(candidates),
			"valid_count":       len(validated),
			"candidates":        storylineCandidatesPreview(validated),
			"validation_errors": validationErrors,
		})
		return
	}
	if len(validated) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"mode":              "apply",
			"parsed_count":      len(candidates),
			"applied_count":     0,
			"results":           []any{},
			"validation_errors": validationErrors,
		})
		return
	}

	existingRows, err := s.Store.ListStorylines(r.Context(), sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		writeInternalError(w, err.Error())
		return
	}
	existingByName := make(map[string]store.Storyline)
	for _, row := range existingRows {
		existingByName[row.Name] = row
	}
	now := time.Now().UTC()
	results := make([]map[string]any, 0, len(validated))
	for _, candidate := range validated {
		existing, hadExisting := existingByName[candidate.Name]
		item := candidate.toStoreStoryline(sid, req.TurnIndex, now, existing, hadExisting)
		if err := saver.SaveStoryline(r.Context(), &item); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				writeShadowGuard(w, "POST /storylines/sync")
				return
			}
			writeInternalError(w, err.Error())
			return
		}
		action := "created"
		if hadExisting {
			action = "updated"
		}
		results = append(results, map[string]any{
			"action":             action,
			"id":                 nullableInt64(item.ID),
			"name":               item.Name,
			"confidence":         item.Confidence,
			"evidence_count":     nullableInt(item.EvidenceCount),
			"last_evidence_turn": nullableInt(item.LastEvidenceTurn),
		})
		existingByName[candidate.Name] = item
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"mode":              "apply",
		"parsed_count":      len(candidates),
		"applied_count":     len(results),
		"results":           results,
		"validation_errors": validationErrors,
	})
}

type storylineSyncRequest struct {
	ChatSessionID    string         `json:"chat_session_id"`
	SupervisorResult map[string]any `json:"supervisor_result"`
	Mode             string         `json:"mode"`
	TurnIndex        *int           `json:"turn_index"`
}

type storylineSyncCandidate struct {
	Name   string
	Fields map[string]any
}

func parseNarrativeInt64Path(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := strings.TrimSpace(r.PathValue(name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeNarrativeJSONMap(r *http.Request) (map[string]any, error) {
	var payload map[string]any
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func normalizeStorylinePatchPayload(payload map[string]any, requireName bool) (map[string]any, error) {
	updates := make(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}
	if val, exists := payload["name"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("name must be a non-empty string")
		}
		updates["name"] = strings.TrimSpace(text)
	} else if requireName {
		return nil, fmt.Errorf("name is required")
	}
	if val, exists := payload["status"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("status must be a string")
		}
		text = firstNonEmpty(strings.TrimSpace(text), "active")
		if text != "active" && text != "paused" && text != "resolved" {
			return nil, fmt.Errorf("invalid status: %s", text)
		}
		updates["status"] = text
	}
	if val, exists := payload["current_context"]; exists {
		text, ok := storylineNullableStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("current_context must be a string or null")
		}
		updates["current_context"] = text
	}
	for _, key := range []string{"entities_json", "key_points_json", "ongoing_tensions_json"} {
		if val, exists := payload[key]; exists {
			normalized, err := normalizeStorylineJSONPatchValue(key, val)
			if err != nil {
				return nil, err
			}
			updates[key] = normalized
		}
	}
	if val, exists := payload["confidence"]; exists {
		f, ok := storylineFloatPatchValue(val)
		if !ok || f < 0 || f > 1 {
			return nil, fmt.Errorf("confidence must be between 0.0 and 1.0")
		}
		updates["confidence"] = f
	}
	for _, key := range []string{"evidence_count", "last_evidence_turn", "first_turn", "last_turn"} {
		if val, exists := payload[key]; exists {
			i, ok := storylineIntPatchValue(val)
			if !ok || i < 0 {
				return nil, fmt.Errorf("%s must be a non-negative integer", key)
			}
			updates[key] = i
		}
	}
	return updates, nil
}

func normalizeStorylineTrustPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		val, exists := payload[key]
		if !exists {
			continue
		}
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", key)
		}
		updates[key] = b
	}
	return updates, nil
}

func normalizePendingThreadPatchPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}
	if val, exists := payload["status"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("status must be a string")
		}
		text = strings.TrimSpace(text)
		if text != "open" && text != "paused" && text != "resolved" {
			return nil, fmt.Errorf("invalid status: %s", text)
		}
		updates["status"] = text
	}
	threadTypeVal, hasThreadType := payload["thread_type"]
	if !hasThreadType {
		threadTypeVal, hasThreadType = payload["hook_type"]
	}
	if hasThreadType {
		text, ok := storylineStringPatchValue(threadTypeVal)
		if !ok {
			return nil, fmt.Errorf("thread_type must be a string")
		}
		text = strings.TrimSpace(text)
		if !validPendingThreadType(text) {
			return nil, fmt.Errorf("invalid thread_type: %s", text)
		}
		updates["thread_type"] = text
	}
	if val, exists := payload["title"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("title must be a non-empty string")
		}
		updates["title"] = strings.TrimSpace(text)
	}
	for _, key := range []string{"owner", "target", "resolution_note"} {
		if val, exists := payload[key]; exists {
			text, ok := storylineNullableStringPatchValue(val)
			if !ok {
				return nil, fmt.Errorf("%s must be a string or null", key)
			}
			updates[key] = text
		}
	}
	if val, exists := payload["confidence"]; exists {
		f, ok := storylineFloatPatchValue(val)
		if !ok || f < 0 || f > 1 {
			return nil, fmt.Errorf("confidence must be between 0.0 and 1.0")
		}
		updates["confidence"] = f
	}
	if val, exists := payload["details_json"]; exists {
		normalized, err := normalizePendingThreadJSONPatchValue("details_json", val)
		if err != nil {
			return nil, err
		}
		updates["details_json"] = normalized
	}
	return updates, nil
}

func validPendingThreadType(text string) bool {
	switch text {
	case "promise", "unresolved_goal", "open_question", "risk", "emotional_debt":
		return true
	default:
		return false
	}
}

func normalizePendingThreadJSONPatchValue(field string, val any) (any, error) {
	if val == nil {
		return nil, nil
	}
	if text, ok := val.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("%s must contain valid JSON", field)
		}
		return mustCompactJSON(decoded), nil
	}
	return mustCompactJSON(val), nil
}

func normalizeStorylineJSONPatchValue(field string, val any) (any, error) {
	if val == nil {
		return nil, nil
	}
	switch typed := val.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("%s must contain valid JSON", field)
		}
		if field == "key_points_json" || field == "ongoing_tensions_json" {
			items, ok := compactStorylineTextList(decoded)
			if !ok {
				return nil, fmt.Errorf("%s must be a JSON string array", field)
			}
			return mustCompactJSON(items), nil
		}
		return mustCompactJSON(decoded), nil
	default:
		if field == "key_points_json" || field == "ongoing_tensions_json" {
			items, ok := compactStorylineTextList(typed)
			if !ok {
				return nil, fmt.Errorf("%s must be a string array", field)
			}
			return mustCompactJSON(items), nil
		}
		return mustCompactJSON(typed), nil
	}
}

func compactStorylineTextList(v any) ([]string, bool) {
	items, ok := v.([]any)
	if !ok {
		if typed, ok := v.([]string); ok {
			out := make([]string, 0, len(typed))
			seen := make(map[string]bool)
			for _, item := range typed {
				text := strings.TrimSpace(item)
				key := strings.ToLower(text)
				if text != "" && !seen[key] {
					seen[key] = true
					out = append(out, text)
				}
			}
			return out, true
		}
		return nil, false
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		if item == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(item))
		key := strings.ToLower(text)
		if text == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, text)
	}
	return out, true
}

func storylineStringPatchValue(v any) (string, bool) {
	text, ok := v.(string)
	return text, ok
}

func storylineNullableStringPatchValue(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	text, ok := v.(string)
	if !ok {
		return nil, false
	}
	if strings.TrimSpace(text) == "" {
		return nil, true
	}
	return text, true
}

func storylineFloatPatchValue(v any) (float64, bool) {
	switch typed := v.(type) {
	case float64:
		return typed, true
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func storylineIntPatchValue(v any) (int, bool) {
	switch typed := v.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		i, err := typed.Int64()
		return int(i), err == nil
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func parseStorylineCandidatesFromSupervisor(supervisorResult map[string]any) []storylineSyncCandidate {
	if supervisorResult == nil {
		return nil
	}
	var out []storylineSyncCandidate
	for _, raw := range sliceFromAny(supervisorResult["storylines"]) {
		item := mapFromAny(raw)
		name := strings.TrimSpace(stringFromMap(item, "name"))
		if name == "" {
			continue
		}
		fields := map[string]any{"name": name}
		copyStorylineCandidateField(fields, item, "status", "status")
		if _, ok := item["entities_json"]; ok {
			copyStorylineCandidateField(fields, item, "entities_json", "entities_json")
		} else if _, ok := item["entities"]; ok {
			copyStorylineCandidateField(fields, item, "entities", "entities_json")
		}
		copyStorylineCandidateField(fields, item, "current_context", "current_context")
		copyStorylineCandidateField(fields, item, "context", "current_context")
		if _, ok := item["key_points_json"]; ok {
			copyStorylineCandidateField(fields, item, "key_points_json", "key_points_json")
		} else {
			copyStorylineCandidateField(fields, item, "key_points", "key_points_json")
		}
		if _, ok := item["ongoing_tensions_json"]; ok {
			copyStorylineCandidateField(fields, item, "ongoing_tensions_json", "ongoing_tensions_json")
		} else {
			copyStorylineCandidateField(fields, item, "ongoing_tensions", "ongoing_tensions_json")
		}
		copyStorylineCandidateField(fields, item, "confidence", "confidence")
		copyStorylineCandidateField(fields, item, "evidence_count", "evidence_count")
		copyStorylineCandidateField(fields, item, "last_evidence_turn", "last_evidence_turn")
		out = append(out, storylineSyncCandidate{Name: name, Fields: fields})
	}
	if len(out) > 0 {
		return out
	}
	for _, key := range []string{"book_author", "story_author"} {
		author := mapFromAny(supervisorResult[key])
		arc := strings.TrimSpace(stringFromMap(author, "current_arc"))
		if arc == "" {
			continue
		}
		fields := map[string]any{
			"name":            arc,
			"status":          "active",
			"current_context": stringFromMap(author, "narrative_goal"),
		}
		if nextBeats := sliceFromAny(author["next_beats"]); len(nextBeats) > 0 {
			fields["key_points_json"] = nextBeats
		}
		if tensions := sliceFromAny(author["ongoing_tensions"]); len(tensions) > 0 {
			fields["ongoing_tensions_json"] = tensions
		} else if guardrails := sliceFromAny(author["guardrails"]); len(guardrails) > 0 {
			fields["ongoing_tensions_json"] = guardrails
		}
		out = append(out, storylineSyncCandidate{Name: arc, Fields: fields})
		return out
	}
	return out
}

func copyStorylineCandidateField(dst map[string]any, src map[string]any, srcKey, dstKey string) {
	if val, ok := src[srcKey]; ok {
		dst[dstKey] = val
	}
}

func normalizeStorylineSyncCandidate(candidate storylineSyncCandidate) (storylineSyncCandidate, []string) {
	updates, err := normalizeStorylinePatchPayload(candidate.Fields, true)
	if err != nil {
		return candidate, []string{err.Error()}
	}
	name, _ := updates["name"].(string)
	if _, ok := updates["status"]; !ok {
		updates["status"] = "active"
	}
	return storylineSyncCandidate{Name: name, Fields: updates}, nil
}

func storylineCandidatesPreview(items []storylineSyncCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		preview := make(map[string]any, len(item.Fields))
		for key, val := range item.Fields {
			preview[key] = val
		}
		out = append(out, preview)
	}
	return out
}

func (c storylineSyncCandidate) toStoreStoryline(sid string, turnIndex *int, now time.Time, existing store.Storyline, hadExisting bool) store.Storyline {
	item := existing
	if !hadExisting {
		item = store.Storyline{
			ChatSessionID: sid,
			CreatedAt:     now,
		}
	}
	item.ChatSessionID = sid
	item.Name = c.Name
	if status, ok := c.Fields["status"].(string); ok && status != "" {
		item.Status = status
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if val, ok := c.Fields["entities_json"].(string); ok {
		item.EntitiesJSON = val
	} else if val, ok := c.Fields["entities_json"]; ok && val == nil {
		item.EntitiesJSON = ""
	}
	if val, ok := c.Fields["current_context"].(string); ok {
		item.CurrentContext = val
	} else if val, ok := c.Fields["current_context"]; ok && val == nil {
		item.CurrentContext = ""
	}
	if val, ok := c.Fields["key_points_json"].(string); ok {
		item.KeyPointsJSON = val
	} else if val, ok := c.Fields["key_points_json"]; ok && val == nil {
		item.KeyPointsJSON = ""
	}
	if val, ok := c.Fields["ongoing_tensions_json"].(string); ok {
		item.OngoingTensionsJSON = val
	} else if val, ok := c.Fields["ongoing_tensions_json"]; ok && val == nil {
		item.OngoingTensionsJSON = ""
	}
	if val, ok := c.Fields["confidence"].(float64); ok {
		item.Confidence = val
	}
	item.EvidenceCount, item.LastEvidenceTurn = resolveStorylineEvidenceUpdate(existing, hadExisting, c.Fields, turnIndex)
	if turnIndex != nil {
		if item.FirstTurn == 0 {
			item.FirstTurn = *turnIndex
		}
		item.LastTurn = *turnIndex
	}
	item.UpdatedAt = now
	return item
}

func resolveStorylineEvidenceUpdate(existing store.Storyline, hadExisting bool, fields map[string]any, turnIndex *int) (int, int) {
	currentCount := 0
	currentLastTurn := 0
	if hadExisting {
		currentCount = existing.EvidenceCount
		currentLastTurn = existing.LastEvidenceTurn
	}
	increment, hasIncrement := fields["evidence_count"].(int)
	explicitTurn, hasExplicitTurn := fields["last_evidence_turn"].(int)
	hasPayload := false
	for _, key := range []string{"current_context", "key_points_json", "ongoing_tensions_json", "entities_json"} {
		val, ok := fields[key]
		if !ok || val == nil {
			continue
		}
		if text, ok := val.(string); !ok || strings.TrimSpace(text) != "" {
			hasPayload = true
			break
		}
	}
	if !hasIncrement {
		if hasPayload || hasExplicitTurn {
			increment = 1
		}
	}
	observedTurn := 0
	if hasExplicitTurn {
		observedTurn = explicitTurn
	} else if increment > 0 && turnIndex != nil {
		observedTurn = *turnIndex
	}
	if observedTurn != 0 && currentLastTurn == observedTurn {
		return currentCount, currentLastTurn
	}
	if increment <= 0 {
		return currentCount, currentLastTurn
	}
	return currentCount + increment, observedTurn
}

// Character: R1 read, R2 write

func (s *Server) handleCharactersGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListCharacterStates(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	events, err := s.Store.ListCharacterEvents(r.Context(), sid, "")
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			events = []store.CharacterEvent{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	events = nonNilSlice(events)
	referenceTurn := s.characterReferenceTurn(r.Context(), sid, items)
	recentMentionText, recentMentionKeywords := s.characterRecentMentionSignal(r.Context(), sid, referenceTurn)
	characters := characterResponseItems(items, events, referenceTurn, recentMentionText, recentMentionKeywords)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"characters":      characters,
		"count":           len(characters),
		"omitted_count":   characterOmittedCount(items, events, referenceTurn, recentMentionText, recentMentionKeywords),
	})
}

func (s *Server) characterReferenceTurn(ctx context.Context, sid string, characters []store.CharacterState) int {
	ref := 0
	if s.Store != nil {
		if logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			for _, log := range logs {
				if log.TurnIndex > ref {
					ref = log.TurnIndex
				}
			}
		}
	}
	for _, ch := range characters {
		if ch.TurnIndex > ref {
			ref = ch.TurnIndex
		}
	}
	return ref
}

func (s *Server) characterRecentMentionSignal(ctx context.Context, sid string, referenceTurn int) (string, map[string]struct{}) {
	if s.Store == nil || sid == "" || referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, fromTurn, 0)
	if err != nil {
		return "", map[string]struct{}{}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex != logs[j].TurnIndex {
			return logs[i].TurnIndex > logs[j].TurnIndex
		}
		return logs[i].ID > logs[j].ID
	})
	if len(logs) > 8 {
		logs = logs[:8]
	}
	parts := []string{}
	for _, log := range logs {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterRecentMentionSignalFromLogs(logs []store.ChatLog, referenceTurn int) (string, map[string]struct{}) {
	if referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	filtered := make([]store.ChatLog, 0, len(logs))
	for _, log := range logs {
		if log.TurnIndex < fromTurn {
			continue
		}
		filtered = append(filtered, log)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		return filtered[i].ID > filtered[j].ID
	})
	if len(filtered) > 8 {
		filtered = filtered[:8]
	}
	parts := []string{}
	for _, log := range filtered {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterResponseItems(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) []map[string]any {
	latest := latestCharacterStatesByName(items)
	out := []map[string]any{}
	for _, item := range latest {
		recentEvents := recentCharacterEvents(events, item.CharacterName, 8)
		snapshot := characterStaleSnapshot(item, recentEvents, referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			continue
		}
		var latestEvent *store.CharacterEvent
		if len(recentEvents) > 0 {
			latestEvent = &recentEvents[0]
		}
		out = append(out, characterResponseItem(item, snapshot, latestEvent, recentEvents))
	}
	return out
}

func characterOmittedCount(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) int {
	omitted := 0
	for _, item := range latestCharacterStatesByName(items) {
		snapshot := characterStaleSnapshot(item, recentCharacterEvents(events, item.CharacterName, 8), referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			omitted++
		}
	}
	return omitted
}

func latestCharacterStatesByName(items []store.CharacterState) []store.CharacterState {
	byName := map[string]store.CharacterState{}
	for _, item := range items {
		name := strings.TrimSpace(item.CharacterName)
		if name == "" {
			continue
		}
		current, ok := byName[name]
		if !ok || item.TurnIndex > current.TurnIndex || (item.TurnIndex == current.TurnIndex && item.ID > current.ID) {
			byName[name] = item
		}
	}
	out := make([]store.CharacterState, 0, len(byName))
	for _, item := range byName {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CharacterName < out[j].CharacterName })
	return out
}

func characterResponseItem(item store.CharacterState, snapshot map[string]any, latestEvent *store.CharacterEvent, recentEvents []store.CharacterEvent) map[string]any {
	relationshipLane := buildCharacterRelationshipLane(item)
	latestAnchor := buildCharacterLatestInteractionAnchor(latestEvent)
	stableSheet := buildStableCharacterSheet(item, snapshot)
	dynamicDigest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latestAnchor, recentEvents)
	return map[string]any{
		"id":                        item.ID,
		"chat_session_id":           item.ChatSessionID,
		"character_name":            item.CharacterName,
		"appearance_json":           nullableJSONString(item.AppearanceJSON),
		"personality_json":          nullableJSONString(item.PersonalityJSON),
		"status_json":               nullableJSONString(item.StatusJSON),
		"relationships_json":        nullableJSONString(item.RelationshipsJSON),
		"speech_style_json":         nullableJSONString(item.SpeechStyleJSON),
		"turn_index":                item.TurnIndex,
		"last_observed_turn":        snapshot["last_observed_turn"],
		"freshness_turn_gap":        snapshot["freshness_turn_gap"],
		"stale_after_turns":         snapshot["stale_after_turns"],
		"is_stale":                  snapshot["is_stale"],
		"stale_reason":              snapshot["stale_reason"],
		"admission_class":           snapshot["admission_class"],
		"admission_basis":           snapshot["admission_basis"],
		"continuity_anchor_types":   snapshot["continuity_anchor_types"],
		"recent_event_count":        snapshot["recent_event_count"],
		"stale_guard":               snapshot["stale_guard"],
		"stable_character_sheet":    stableSheet,
		"dynamic_continuity_digest": dynamicDigest,
		"relationship_lane":         relationshipLane,
		"latest_interaction_anchor": latestAnchor,
		"created_at":                formatNaiveUTCTime(item.CreatedAt),
		"updated_at":                formatNaiveUTCTime(item.UpdatedAt),
	}
}

func characterStaleSnapshot(item store.CharacterState, recentEvents []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) map[string]any {
	lastObserved := item.TurnIndex
	gapInt := 0
	var gap any
	if referenceTurn > 0 && lastObserved > 0 {
		gapInt = referenceTurn - lastObserved
		if gapInt < 0 {
			gapInt = 0
		}
		gap = gapInt
	}
	anchors := []string{}
	if strings.TrimSpace(item.AppearanceJSON) != "" {
		anchors = append(anchors, "appearance")
	}
	if strings.TrimSpace(item.PersonalityJSON) != "" {
		anchors = append(anchors, "personality")
	}
	if strings.TrimSpace(item.RelationshipsJSON) != "" {
		anchors = append(anchors, "relationships")
	}
	if strings.TrimSpace(item.SpeechStyleJSON) != "" {
		anchors = append(anchors, "speech_style")
	}
	for _, ev := range recentEvents {
		eventType := strings.TrimSpace(ev.EventType)
		if eventType == "relationship_shift" || eventType == "personality_change" {
			anchors = appendUniqueString(anchors, "event_anchor")
			break
		}
	}
	hasAnchor := len(anchors) > 0
	staleAfter := 3
	descriptorLike := looksLikeTransientCharacterName(item.CharacterName)
	recentlyRementioned := descriptorRecentlyRementioned(item.CharacterName, recentMentionText, recentMentionKeywords)
	isStale := descriptorLike && referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor && !recentlyRementioned
	staleReason := any(nil)
	if isStale {
		staleReason = "transient_descriptor_not_rementioned"
	}
	admissionClass := "lightweight_named"
	if hasAnchor || recentlyRementioned || len(recentEvents) >= 2 {
		admissionClass = "major_recurring"
	} else if descriptorLike {
		admissionClass = "transient_descriptor"
	}
	admissionBasis := make([]string, len(anchors))
	copy(admissionBasis, anchors)
	recentEventCount := len(recentEvents)
	if recentEventCount > 3 {
		recentEventCount = 3
	}
	if recentlyRementioned {
		admissionBasis = append(admissionBasis, "recent_remention")
	}
	if len(recentEvents) >= 2 {
		admissionBasis = append(admissionBasis, "recent_event_history")
	}
	return map[string]any{
		"last_observed_turn":      nullablePositiveInt(lastObserved),
		"freshness_turn_gap":      gap,
		"stale_after_turns":       staleAfter,
		"is_stale":                isStale,
		"stale_reason":            staleReason,
		"admission_class":         admissionClass,
		"admission_basis":         admissionBasis,
		"continuity_anchor_types": anchors,
		"recent_event_count":      recentEventCount,
		"stale_guard": map[string]any{
			"active":                         isStale || (referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor),
			"reason":                         staleReasonIfNeeded(staleReason, referenceTurn, gapInt, staleAfter, hasAnchor),
			"allow_weak_input_carry_forward": !isStale && (hasAnchor || recentlyRementioned),
			"admission_class":                admissionClass,
			"admission_basis":                admissionBasis,
		},
	}
}

func nullableJSONString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func staleReasonIfNeeded(staleReason any, referenceTurn, gapInt, staleAfter int, hasAnchor bool) any {
	if staleReason != nil {
		return staleReason
	}
	if referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor {
		return "low_anchor_freshness_gap"
	}
	return nil
}

func descriptorRecentlyRementioned(name string, recentText string, recentKeywords map[string]struct{}) bool {
	rawName := normalizeCharacterDescriptorText(name)
	rawRecentText := normalizeCharacterDescriptorText(recentText)
	if rawName != "" && strings.Contains(rawRecentText, rawName) {
		return true
	}
	nameKeywords := extractCharacterDescriptorKeywords(name)
	genericHit := false
	for token := range characterGenericDescriptorTokens() {
		if _, ok := recentKeywords[token]; ok {
			genericHit = true
			break
		}
		if rawRecentText != "" && strings.Contains(rawRecentText, token) {
			genericHit = true
			break
		}
	}
	if !genericHit {
		return false
	}
	nonGeneric := []string{}
	generic := characterGenericDescriptorTokens()
	for _, token := range nameKeywords {
		if _, ok := generic[token]; !ok {
			nonGeneric = append(nonGeneric, token)
		}
	}
	if len(nonGeneric) == 0 {
		return true
	}
	overlap := 0
	for _, token := range nonGeneric {
		if _, ok := recentKeywords[token]; ok {
			overlap++
		}
	}
	required := 1
	if len(nonGeneric) > 1 {
		required = 2
		if len(nonGeneric) < required {
			required = len(nonGeneric)
		}
	}
	return overlap >= required
}

func extractCharacterRecentKeywords(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range splitCharacterDescriptorTokens(text) {
		if len([]rune(token)) >= characterKeywordMinLength(token) {
			out[token] = struct{}{}
		}
	}
	return out
}

func extractCharacterDescriptorKeywords(name string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, token := range splitCharacterDescriptorTokens(name) {
		if token == "" || characterDescriptorStopwords()[token] {
			continue
		}
		if len([]rune(token)) < characterKeywordMinLength(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func splitCharacterDescriptorTokens(text string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '.', ',', '!', '?', ':', ';', '(', ')', '[', ']', '{', '}', '/', '|', '\\', '"', '\'', '`', '~', '@', '#', '$', '%', '^', '&', '*', '+', '=', '<', '>', '-', '_':
			return true
		default:
			return false
		}
	})
}

func normalizeCharacterDescriptorText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func characterKeywordMinLength(token string) int {
	for _, r := range token {
		if (r >= '\uAC00' && r <= '\uD7A3') || (r >= '\u1100' && r <= '\u11FF') {
			return 2
		}
	}
	return 3
}

func characterGenericDescriptorTokens() map[string]struct{} {
	return map[string]struct{}{
		"woman": {}, "man": {}, "girl": {}, "boy": {}, "lady": {}, "gentleman": {}, "stranger": {}, "figure": {}, "person": {}, "voice": {},
	}
}

func characterDescriptorStopwords() map[string]bool {
	return map[string]bool{"a": true, "an": true, "the": true, "this": true, "that": true, "these": true, "those": true, "in": true, "on": true, "at": true, "of": true}
}

func looksLikeTransientCharacterName(name string) bool {
	text := strings.TrimSpace(name)
	if text == "" {
		return true
	}
	lower := strings.ToLower(text)
	transientTokens := []string{"unknown", "unnamed", "npc", "woman", "man", "girl", "boy", "person", "voice", "figure", "descriptor"}
	for _, token := range transientTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return strings.Count(text, " ") >= 2
}

func recentCharacterEvents(events []store.CharacterEvent, characterName string, limit int) []store.CharacterEvent {
	filtered := []store.CharacterEvent{}
	for _, ev := range events {
		if ev.CharacterName == characterName {
			filtered = append(filtered, ev)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func parseSurfacePayload(raw string) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return parsed
	}
	return text
}

func hasSurfaceValue(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		}
		return true
	}
}

func surfaceStatus(payloads map[string]any) string {
	filled := 0
	for _, value := range payloads {
		if hasSurfaceValue(value) {
			filled++
		}
	}
	if filled == 0 {
		return "empty"
	}
	if filled == len(payloads) {
		return "ready"
	}
	return "partial"
}

func filledAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func filledAxesInOrder(payloads map[string]any, order ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, key := range order {
		seen[key] = true
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	for _, key := range sortedMapKeys(payloads) {
		if seen[key] {
			continue
		}
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatNaiveUTCTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func cleanShadowText(value any, maxLen int) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return truncateRunes(strings.Join(strings.Fields(v), " "), maxLen)
	case float64, bool, int, int64:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	case map[string]any, []any:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	default:
		return truncateRunes(strings.Join(strings.Fields(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(compactJSONForShadow(v, maxLen)), "\n", " "), "\t", " "))), " "), maxLen)
	}
}

func preferredSummaryText(payload any, maxLen int, keys ...string) string {
	if m, ok := payload.(map[string]any); ok {
		parts := []string{}
		rawTextKeys := map[string]bool{"summary": true, "summary_text": true, "detail": true, "note": true, "message": true, "interaction": true}
		for _, key := range keys {
			text := cleanShadowText(m[key], 90)
			if text == "" {
				continue
			}
			if rawTextKeys[key] {
				parts = append(parts, text)
			} else {
				parts = append(parts, key+": "+text)
			}
			if len(parts) >= 3 {
				break
			}
		}
		if len(parts) > 0 {
			return truncateRunes(strings.Join(parts, "; "), maxLen)
		}
	}
	return cleanShadowText(payload, maxLen)
}

func buildCharacterRelationshipLane(item store.CharacterState) map[string]any {
	payload := parseSurfacePayload(item.RelationshipsJSON)
	protagonistItems := []map[string]any{}
	otherItems := []map[string]any{}
	seen := map[string]bool{}
	var protagonist map[string]any
	appendItem := func(targetRaw any, relation any) {
		target := relationDisplayTarget(targetRaw)
		if target == "" {
			return
		}
		summary := preferredSummaryText(relation, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
		if summary == "" {
			return
		}
		key := strings.ToLower(target + "\x00" + summary)
		if seen[key] {
			return
		}
		seen[key] = true
		isPlayer := isPlayerReference(targetRaw)
		relationMap := map[string]any{"value": relation}
		if m, ok := relation.(map[string]any); ok {
			relationMap = m
		}
		entry := map[string]any{
			"target":           target,
			"summary_text":     summary,
			"state_snapshot":   projectRelationPayload(relationMap),
			"descriptor_bands": relationDescriptorBands(relationMap),
			"display_priority": 1,
		}
		if isPlayer {
			entry["display_priority"] = 0
			protagonistItems = append(protagonistItems, entry)
			if protagonist == nil {
				protagonist = entry
			}
			return
		}
		otherItems = append(otherItems, entry)
	}
	switch v := payload.(type) {
	case []any:
		for _, raw := range v {
			if m, ok := raw.(map[string]any); ok {
				appendItem(firstPresentValue(m, "target", "name", "character_name", "title", "scope_name"), m)
			}
		}
	case map[string]any:
		for _, key := range sortedMapKeys(v) {
			value := v[key]
			target := any(key)
			if m, ok := value.(map[string]any); ok {
				if tv := firstPresentValue(m, "target", "name", "character_name"); tv != nil {
					target = tv
				}
			}
			appendItem(target, value)
		}
	}
	ordered := append([]map[string]any{}, protagonistItems...)
	ordered = append(ordered, otherItems...)
	if len(ordered) > 6 {
		ordered = ordered[:6]
	}
	preferred := protagonist
	if preferred == nil && len(ordered) > 0 {
		preferred = ordered[0]
	}
	secondary := []map[string]any{}
	for _, entry := range ordered {
		if preferred != nil && entry["target"] == preferred["target"] && entry["summary_text"] == preferred["summary_text"] {
			continue
		}
		secondary = append(secondary, entry)
	}
	summary := ""
	if preferred != nil {
		summary, _ = preferred["summary_text"].(string)
	}
	if summary == "" {
		summary = preferredSummaryText(payload, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
	}
	status := "empty"
	if len(ordered) > 0 {
		status = "ready"
	} else if summary != "" {
		status = "summary_only"
	}
	descriptorSummary := ""
	if preferred != nil {
		if bands, ok := preferred["descriptor_bands"].([]string); ok {
			descriptorSummary = truncateRunes(strings.Join(bands, "; "), 180)
		}
	}
	return map[string]any{
		"surface_version":          "rl14a.v1",
		"surface_type":             "relationship_lane",
		"status":                   status,
		"display_mode":             "protagonist_first_then_observed_order",
		"count":                    len(protagonistItems) + len(otherItems),
		"summary_text":             nullableString(summary),
		"descriptor_summary":       descriptorSummary,
		"primary_target":           mapStringOrNil(preferred, "target"),
		"primary_descriptor_bands": mapAnyOrEmptyStringSlice(preferred, "descriptor_bands"),
		"protagonist_relation":     protagonist,
		"other_relations":          limitMapSlice(secondary, 5),
		"items":                    mapSliceToAny(ordered),
	}
}

func buildCharacterLatestInteractionAnchor(event *store.CharacterEvent) any {
	if event == nil {
		return nil
	}
	details := parseSurfacePayload(event.DetailsJSON)
	summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
	if summary == "" {
		summary = cleanShadowText(event.EventType, 80)
	}
	return map[string]any{
		"surface_version": "rl14b.v1",
		"surface_type":    "latest_interaction_anchor",
		"status":          "ready",
		"event_type":      event.EventType,
		"turn_index":      event.TurnIndex,
		"summary_text":    summary,
		"details":         details,
		"created_at":      formatNaiveUTCTime(event.CreatedAt),
	}
}

func buildStableCharacterSheet(item store.CharacterState, snapshot map[string]any) map[string]any {
	appearance := parseSurfacePayload(item.AppearanceJSON)
	personality := parseSurfacePayload(item.PersonalityJSON)
	speechStyle := parseSurfacePayload(item.SpeechStyleJSON)
	appearanceCore, appearanceSnapshot := splitAppearancePayload(appearance)
	appearanceObservable, appearanceNonObservable := splitObservableAppearancePayload(appearanceCore)
	axes := map[string]any{"appearance": appearanceCore, "personality": personality, "speech_style": speechStyle}
	return map[string]any{
		"surface_version":           "cc14a.v1",
		"surface_type":              "stable_character_sheet",
		"status":                    surfaceStatus(axes),
		"filled_axes":               filledAxes(axes),
		"appearance":                appearance,
		"appearance_core":           appearanceCore,
		"appearance_observable":     appearanceObservable,
		"appearance_non_observable": appearanceNonObservable,
		"appearance_snapshot_keys":  sortedMapKeys(appearanceSnapshot),
		"personality":               personality,
		"speech_style":              speechStyle,
		"durable_profile": map[string]any{
			"appearance":   appearanceObservable,
			"personality":  personality,
			"speech_style": speechStyle,
		},
		"sparse_policy": map[string]any{
			"mode":              "omit_unknown_fields",
			"filled_axes":       filledAxes(axes),
			"empty_axes":        emptyAxes(axes),
			"dynamic_redirects": []string{"current_status", "relationship_lane", "latest_interaction_anchor", "appearance_snapshot"},
		},
		"source_turn": snapshot["last_observed_turn"],
	}
}

func buildDynamicCharacterDigest(item store.CharacterState, snapshot map[string]any, relationshipLane map[string]any, latestAnchor any, recentEvents []store.CharacterEvent) map[string]any {
	currentStatus := parseSurfacePayload(item.StatusJSON)
	appearance := parseSurfacePayload(item.AppearanceJSON)
	_, appearanceSnapshot := splitAppearancePayload(appearance)
	currentStatusSummary := ""
	if m, ok := currentStatus.(map[string]any); ok {
		currentStatusSummary = preferredSummaryText(m, 180, "location", "emotion", "goal", "status", "state", "condition", "mood")
	}
	relationshipItems := mapAnySlice(relationshipLane["items"])
	relationshipDescriptorLane := []map[string]any{}
	for _, entry := range relationshipItems[:minInt(len(relationshipItems), 4)] {
		relationshipDescriptorLane = append(relationshipDescriptorLane, map[string]any{
			"target":           entry["target"],
			"summary_text":     entry["summary_text"],
			"descriptor_bands": mapAnyOrEmptyStringSlice(entry, "descriptor_bands"),
		})
	}
	var preferredRelation map[string]any
	if pr, ok := relationshipLane["protagonist_relation"].(map[string]any); ok && pr != nil {
		preferredRelation = pr
	} else if len(relationshipItems) > 0 {
		preferredRelation = relationshipItems[0]
	}
	var relationshipFocus map[string]any
	if preferredRelation != nil {
		relationshipFocus = compactMap(map[string]any{
			"target":           mapStringOrNil(preferredRelation, "target"),
			"summary_text":     mapStringOrNil(preferredRelation, "summary_text"),
			"descriptor_bands": mapAnyOrEmptyStringSlice(preferredRelation, "descriptor_bands"),
		})
	}
	var relationshipSurface any
	if preferredRelation != nil {
		relationshipSurface = preferredRelation
	} else if len(relationshipItems) > 0 {
		relationshipSurface = relationshipItems
	} else if st, ok := relationshipLane["summary_text"].(string); ok && strings.TrimSpace(st) != "" {
		relationshipSurface = st
	}
	axes := map[string]any{"current_status": currentStatus, "relationship_surface": relationshipSurface, "latest_interaction_anchor": latestAnchor}
	return map[string]any{
		"surface_version":              "cc14b.v1",
		"surface_type":                 "dynamic_continuity_digest",
		"status":                       surfaceStatus(axes),
		"filled_axes":                  filledAxesInOrder(axes, "current_status", "relationship_surface", "latest_interaction_anchor"),
		"admission_class":              snapshot["admission_class"],
		"admission_basis":              snapshot["admission_basis"],
		"stale_guard":                  snapshot["stale_guard"],
		"current_status":               currentStatus,
		"current_status_summary":       nullableString(currentStatusSummary),
		"current_snapshot":             compactMap(map[string]any{"status": currentStatus, "appearance": appearanceSnapshot, "relationship_focus": relationshipFocus}),
		"appearance_snapshot":          appearanceSnapshot,
		"relationship_summary_text":    relationshipLane["summary_text"],
		"relationship_primary_target":  relationshipLane["primary_target"],
		"relationship_display_mode":    relationshipLane["display_mode"],
		"protagonist_relation":         relationshipLane["protagonist_relation"],
		"relationship_lane":            relationshipLane["items"],
		"other_relations":              relationshipLane["other_relations"],
		"relationship_descriptor_lane": mapSliceToAny(relationshipDescriptorLane),
		"latest_interaction_anchor":    latestAnchor,
		"milestone_ledger":             characterMilestoneLedger(recentEvents),
		"digest_budget": map[string]any{
			"policy":                     "priority_capped",
			"relationship_lane_cap":      4,
			"milestone_cap":              3,
			"milestone_read_window":      8,
			"milestone_selection_policy": "latest_plus_priority_events",
			"relationship_lane_used":     len(relationshipDescriptorLane),
			"milestones_used":            len(characterMilestoneLedger(recentEvents)),
		},
		"recent_change_summary": recentChangeSummary(latestAnchor),
		"source_turn":           snapshot["last_observed_turn"],
	}
}

func firstPresentValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && hasSurfaceValue(v) {
			return v
		}
	}
	return nil
}

func isPlayerReference(value any) bool {
	text := strings.ToLower(cleanShadowText(value, 60))
	switch text {
	case "__player__", "{{user}}", "user", "player", "participant":
		return true
	default:
		return false
	}
}

func relationDisplayTarget(value any) string {
	if isPlayerReference(value) {
		return "{{user}}"
	}
	return cleanShadowText(value, 60)
}

func projectRelationPayload(payload any) any {
	switch v := payload.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range v {
			if key == "target" || key == "name" || key == "character_name" || key == "owner" || key == "subject" || key == "object" || key == "from" || key == "to" {
				if display := relationDisplayTarget(value); display != "" {
					out[key] = display
					continue
				}
			}
			out[key] = projectRelationPayload(value)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, projectRelationPayload(item))
		}
		return out
	case string:
		if isPlayerReference(v) {
			return "{{user}}"
		}
		return v
	default:
		return v
	}
}

func relationDescriptorBands(payload map[string]any) []string {
	keys := []string{"trust", "closeness", "tension", "bond", "distance", "stance"}
	out := []string{}
	for _, key := range keys {
		if text := cleanShadowText(payload[key], 60); text != "" {
			out = append(out, key+": "+text)
		}
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func splitAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	durable := map[string]any{}
	snapshot := map[string]any{}
	snapshotTokens := []string{
		"outfit", "clothes", "clothing", "uniform", "coat", "jacket", "dress", "armor", "accessory",
		"expression", "posture", "condition", "injury", "blood", "mud", "wet",
	}
	for key, value := range m {
		normalized := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		isSnapshot := false
		for _, token := range snapshotTokens {
			if strings.Contains(normalized, token) {
				isSnapshot = true
				break
			}
		}
		if isSnapshot {
			snapshot[key] = value
			continue
		}
		durable[key] = value
	}
	if len(durable) == 0 {
		return payload, snapshot
	}
	return durable, snapshot
}

func splitObservableAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	observable := map[string]any{}
	nonObservable := map[string]any{}
	for key, value := range m {
		lower := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		if strings.Contains(lower, "thought") || strings.Contains(lower, "emotion") || strings.Contains(lower, "feeling") || strings.Contains(lower, "internal") {
			nonObservable[key] = value
			continue
		}
		observable[key] = value
	}
	return observable, nonObservable
}

func emptyAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if !hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func compactMap(payload map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range payload {
		if hasSurfaceValue(value) {
			out[key] = value
		}
	}
	return out
}

func characterMilestoneLedger(events []store.CharacterEvent) []any {
	candidates := []map[string]any{}
	for recencyIndex, ev := range events {
		details := parseSurfacePayload(ev.DetailsJSON)
		summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
		if summary == "" {
			summary = cleanShadowText(ev.EventType, 80)
		}
		if summary == "" {
			continue
		}
		priority := characterEventPriority(ev.EventType)
		candidates = append(candidates, map[string]any{
			"event_type":      ev.EventType,
			"turn_index":      ev.TurnIndex,
			"summary_text":    summary,
			"details":         details,
			"created_at":      formatNaiveUTCTime(ev.CreatedAt),
			"_event_priority": priority,
			"_recency_index":  recencyIndex,
		})
	}
	if len(candidates) == 0 {
		return []any{}
	}

	selected := []map[string]any{}
	seen := map[string]bool{}
	appendCandidate := func(candidate map[string]any) {
		key := fmt.Sprintf("%v|%v|%v", candidate["event_type"], candidate["turn_index"], candidate["summary_text"])
		if seen[key] {
			return
		}
		seen[key] = true
		selected = append(selected, candidate)
	}

	appendCandidate(candidates[0])
	if len(candidates) > 1 {
		remaining := make([]map[string]any, len(candidates[1:]))
		copy(remaining, candidates[1:])
		sort.SliceStable(remaining, func(i, j int) bool {
			pi, _ := remaining[i]["_event_priority"].(int)
			pj, _ := remaining[j]["_event_priority"].(int)
			ri, _ := remaining[i]["_recency_index"].(int)
			rj, _ := remaining[j]["_recency_index"].(int)
			if pi != pj {
				return pi < pj
			}
			return ri < rj
		})
		for _, candidate := range remaining {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}
	if len(selected) < 3 {
		for _, candidate := range candidates[1:] {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}

	sort.SliceStable(selected, func(i, j int) bool {
		ri, _ := selected[i]["_recency_index"].(int)
		rj, _ := selected[j]["_recency_index"].(int)
		return ri < rj
	})

	out := []any{}
	for _, candidate := range selected[:minInt(len(selected), 3)] {
		cleaned := map[string]any{}
		for k, v := range candidate {
			if !strings.HasPrefix(k, "_") {
				cleaned[k] = v
			}
		}
		out = append(out, cleaned)
	}
	return out
}

func recentChangeSummary(anchor any) any {
	if m, ok := anchor.(map[string]any); ok {
		return m["summary_text"]
	}
	return nil
}

func mapStringOrNil(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return nil
}

func mapAnyOrEmptyStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return []string{}
	}
	if v, ok := m[key].([]string); ok {
		return v
	}
	return []string{}
}

func limitMapSlice(items []map[string]any, limit int) []any {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return mapSliceToAny(items)
}

func mapSliceToAny(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func mapAnySlice(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (s *Server) handleCharacterDetail(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	item, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			item = nil
		} else if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status": "error",
				"detail": fmt.Sprintf("character not found: %s", cname),
			})
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"found":           item != nil,
		"character":       item,
	})
}

func (s *Server) handleCharacterEvents(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 30
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	items, err := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	total := len(items)
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"events":          page,
		"total":           total,
		"limit":           limit,
		"offset":          offset,
	})
}

func (s *Server) handleCharacterStateHistory(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	historyStore, ok := s.Store.(store.CharacterStateHistoryStore)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"chat_session_id": sid,
			"character_name":  cname,
			"state_history":   []store.CharacterState{},
			"count":           0,
			"limit":           limit,
			"offset":          offset,
			"mode":            "history_store_not_available",
		})
		return
	}
	items, err := historyStore.ListCharacterStateHistory(r.Context(), sid, cname, limit, offset)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"state_history":   items,
		"count":           len(items),
		"limit":           limit,
		"offset":          offset,
		"mode":            "append_only_snapshots_latest_first",
	})
}

func (s *Server) handleCharacterPatch(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, false)
}

func (s *Server) handleCharacterSpeech(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, true)
}

func (s *Server) handleCharacterDelete(w http.ResponseWriter, r *http.Request) {
	endpoint := "DELETE /characters/{chat_session_id}/{character_name}"
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	events, _ := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteCharacterByName(r.Context(), sid, cname); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "character",
		TargetID:      current.ID,
		Summary:       "Explorer manual character delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"character_name": cname,
			"previous": map[string]any{
				"turn_index":         current.TurnIndex,
				"appearance_json":    current.AppearanceJSON,
				"personality_json":   current.PersonalityJSON,
				"status_json":        current.StatusJSON,
				"relationships_json": current.RelationshipsJSON,
				"speech_style_json":  current.SpeechStyleJSON,
				"created_at":         current.CreatedAt,
				"updated_at":         current.UpdatedAt,
			},
			"character_events_deleted": len(events),
			"changed_at":               changedAt,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"mutation_enabled":         true,
		"chat_session_id":          sid,
		"target_type":              "character",
		"target_id":                current.ID,
		"character_name":           cname,
		"deleted":                  true,
		"character_events_deleted": len(events),
		"changed_at":               changedAt,
		"audit_written":            true,
	})
}

func (s *Server) handleCharacterStatePatch(w http.ResponseWriter, r *http.Request, speechOnly bool) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	saver, ok := s.Store.(characterStateSaver)
	if !ok {
		writeShadowGuard(w, r.Method+" "+r.URL.Path)
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeCharacterPatchPayload(payload, speechOnly)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if len(updates) == 0 {
		writeBadRequest(w, "no supported character fields to update")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	now := time.Now().UTC()
	next := *current
	next.ChatSessionID = sid
	next.CharacterName = cname
	next.UpdatedAt = now
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	changed := make([]string, 0, len(updates))
	for _, key := range []string{"appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "turn_index"} {
		val, exists := updates[key]
		if !exists {
			continue
		}
		changed = append(changed, key)
		switch key {
		case "appearance_json":
			next.AppearanceJSON = stringFromAnyNullable(val)
		case "personality_json":
			next.PersonalityJSON = stringFromAnyNullable(val)
		case "status_json":
			next.StatusJSON = stringFromAnyNullable(val)
		case "relationships_json":
			next.RelationshipsJSON = stringFromAnyNullable(val)
		case "speech_style_json":
			next.SpeechStyleJSON = stringFromAnyNullable(val)
		case "turn_index":
			if i, ok := val.(int); ok {
				next.TurnIndex = i
			}
		}
	}
	if err := saver.SaveCharacterState(r.Context(), &next); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	eventType := "manual_patch"
	if speechOnly {
		eventType = "speech_style_patch"
	}
	_ = s.Store.SaveCharacterEvent(r.Context(), &store.CharacterEvent{
		ChatSessionID: sid,
		CharacterName: cname,
		TurnIndex:     next.TurnIndex,
		EventType:     eventType,
		DetailsJSON:   mustCompactJSON(map[string]any{"updated_fields": changed, "source": "manual_patch"}),
		CreatedAt:     now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"updated_fields":  changed,
		"character":       characterResponseItem(next, characterStaleSnapshot(next, nil, next.TurnIndex, "", nil), nil, nil),
	})
}

func normalizeCharacterPatchPayload(payload map[string]any, speechOnly bool) (map[string]any, error) {
	updates := map[string]any{}
	fieldMap := map[string]string{
		"appearance_json":    "appearance_json",
		"appearance":         "appearance_json",
		"personality_json":   "personality_json",
		"personality":        "personality_json",
		"status_json":        "status_json",
		"status":             "status_json",
		"relationships_json": "relationships_json",
		"relationships":      "relationships_json",
		"speech_style_json":  "speech_style_json",
		"speech_style":       "speech_style_json",
	}
	if speechOnly {
		fieldMap = map[string]string{"speech_style_json": "speech_style_json", "speech_style": "speech_style_json"}
	}
	for rawKey, targetKey := range fieldMap {
		val, exists := payload[rawKey]
		if !exists {
			continue
		}
		normalized, err := normalizeStorylineJSONPatchValue(targetKey, val)
		if err != nil {
			return nil, err
		}
		updates[targetKey] = normalized
	}
	if !speechOnly {
		if val, exists := payload["turn_index"]; exists {
			i, ok := storylineIntPatchValue(val)
			if !ok || i < 0 {
				return nil, fmt.Errorf("turn_index must be a non-negative integer")
			}
			updates["turn_index"] = i
		}
	}
	return updates, nil
}

func stringFromAnyNullable(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// World rules: R1 read, R2 write

func (s *Server) handleWorldRulesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListWorldRules(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	responseItems := worldRuleResponseItems(items, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  responseItems,
		"count":  len(responseItems),
	})
}

func (s *Server) handleWorldRulesInherited(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	activeScope := strings.TrimSpace(r.URL.Query().Get("active_scope"))
	scopeName := strings.TrimSpace(r.URL.Query().Get("scope_name"))
	if activeScope == "" {
		if saved, _, err := s.resolveActiveScope(r.Context(), sid); err == nil && saved != nil {
			activeScope = strings.TrimSpace(saved.ActiveScope)
			if scopeName == "" {
				scopeName = strings.TrimSpace(saved.ScopeName)
			}
		} else if err != nil {
			writeInternalError(w, err.Error())
			return
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	if !isValidWorldRuleScope(activeScope) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "error",
			"detail": "active_scope must be one of [root region location faction system session]",
		})
		return
	}
	scopeChain := worldRuleScopeChain(activeScope)
	items, err := s.Store.ListInheritedWorldRules(r.Context(), sid, activeScope, scopeName)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	rules := worldRuleResponseItems(items, activeScope)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"active_scope": activeScope,
		"scope_name":   nullableString(scopeName),
		"scope_chain":  scopeChain,
		"rules":        rules,
		"count":        len(rules),
	})
}

func (s *Server) handleWorldRulesSync(w http.ResponseWriter, r *http.Request) {
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		writeShadowGuard(w, "POST /world-rules/sync")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(payload["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["mode"])))
	if mode == "" {
		mode = "apply"
	}
	if mode != "apply" && mode != "dry_run" {
		writeBadRequest(w, "mode must be apply or dry_run")
		return
	}
	turnIndex := intFromAny(payload["turn_index"], 0)
	candidates := buildWorldRuleSyncCandidates(sid, turnIndex, mapFromAny(payload["supervisor_response"]))
	if mode == "dry_run" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"mode":            mode,
			"chat_session_id": sid,
			"candidate_count": len(candidates),
			"would_write":     false,
		})
		return
	}
	applied := 0
	for i := range candidates {
		if err := saver.SaveWorldRule(r.Context(), &candidates[i]); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				writeShadowGuard(w, "POST /world-rules/sync")
				return
			}
			writeInternalError(w, err.Error())
			return
		}
		applied++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"mode":            mode,
		"chat_session_id": sid,
		"candidate_count": len(candidates),
		"applied_count":   applied,
	})
}

func (s *Server) handleWorldRulePatch(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchWorldRule(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /world-rules/{rule_id}")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeWorldRulePatchPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchWorldRule(r.Context(), ruleID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /world-rules/{rule_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "rule_id": ruleID, "updated_fields": updatedFields})
}

func (s *Server) handleWorldRuleTrust(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchWorldRuleTrust(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /world-rules/{rule_id}/trust")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeWorldRuleTrustPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchWorldRuleTrust(r.Context(), ruleID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /world-rules/{rule_id}/trust")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{"status": "ok", "rule_id": ruleID, "updated_fields": updatedFields}
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWorldRuleDelete(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		DeleteWorldRule(context.Context, int64) error
	})
	if !ok {
		writeShadowGuard(w, "DELETE /world-rules/{rule_id}")
		return
	}
	if err := mutator.DeleteWorldRule(r.Context(), ruleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /world-rules/{rule_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	vectorCleanup := map[string]any{"attempted": false, "ok": true, "skipped_reason": "chat_session_id_not_provided"}
	if sid != "" {
		vectorCleanup = s.deleteDerivedArtifactVectorDocuments(r.Context(), sid, "world_rule", ruleID)
	}
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "deleted_id": ruleID, "vector_cleanup": vectorCleanup})
}

func buildWorldRuleSyncCandidates(sid string, turnIndex int, supervisor map[string]any) []store.WorldRule {
	sectionWorld := mapFromAny(supervisor["section_world"])
	genre := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(sectionWorld, "genre"), stringFromMap(sectionWorld, "genre_hint")))
	rawItems := collectWorldRuleSyncItems(supervisor)
	now := time.Now().UTC()
	out := make([]store.WorldRule, 0, len(rawItems))
	seen := map[string]bool{}
	for _, raw := range rawItems {
		if text, ok := raw.(string); ok {
			key := truncateRunes(strings.TrimSpace(text), 500)
			if key == "" {
				continue
			}
			raw = map[string]any{
				"scope":    "root",
				"category": "custom",
				"key":      key,
				"genre":    genre,
			}
		}
		item := mapFromAny(raw)
		key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "key"), stringFromMap(item, "name"), stringFromMap(item, "rule_key")))
		if key == "" {
			key = strings.TrimSpace(stringFromMap(item, "rule"))
		}
		key = truncateRunes(key, 500)
		if key == "" {
			continue
		}
		scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(item, "scope"), "root"))
		if !isValidWorldRuleScope(scope) {
			continue
		}
		category := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "category"), "custom"))
		dedupeKey := strings.ToLower(scope + "\x00" + stringFromMap(item, "scope_name") + "\x00" + key)
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		value := item["value_json"]
		if value == nil {
			value = item["value"]
		}
		if value == nil {
			value = item["description"]
		}
		if value == nil {
			value = item["summary"]
		}
		out = append(out, store.WorldRule{
			ChatSessionID: sid,
			Scope:         scope,
			ScopeName:     strings.TrimSpace(stringFromMap(item, "scope_name")),
			Category:      category,
			Key:           key,
			ValueJSON:     normalizeWorldRuleValueJSON(value),
			Genre:         strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "genre"), genre)),
			SourceTurn:    turnIndex,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	return out
}

func collectWorldRuleSyncItems(supervisor map[string]any) []any {
	items := []any{}
	appendItems := func(raw any) {
		if raw == nil {
			return
		}
		for _, item := range sliceFromAny(raw) {
			items = append(items, item)
		}
	}
	sectionWorld := mapFromAny(supervisor["section_world"])
	appendItems(sectionWorld["constants"])
	appendItems(sectionWorld["rules"])
	appendItems(sectionWorld["world_rules"])
	appendItems(sectionWorld["confidence_notes"])
	appendItems(supervisor["world_rules"])
	if len(items) == 0 && strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(sectionWorld, "key"), stringFromMap(sectionWorld, "name"))) != "" {
		items = append(items, sectionWorld)
	}
	return items
}

func normalizeWorldRulePatchPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	for _, key := range []string{"scope", "scope_name", "category", "key", "genre"} {
		val, exists := payload[key]
		if !exists {
			continue
		}
		rawText, ok := storylineNullableStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("%s must be a string or null", key)
		}
		text, _ := rawText.(string)
		if key == "scope" {
			text = store.NormalizeWorldRuleScope(firstNonEmpty(strings.TrimSpace(text), "root"))
			if !isValidWorldRuleScope(text) {
				return nil, fmt.Errorf("invalid scope: %s", text)
			}
		}
		if key == "key" && strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("key must be a non-empty string")
		}
		if key == "scope_name" && strings.TrimSpace(text) == "" {
			updates[key] = nil
			continue
		}
		updates[key] = text
	}
	if val, exists := payload["value_json"]; exists {
		updates["value_json"] = normalizeWorldRuleValueJSON(val)
	} else if val, exists := payload["value"]; exists {
		updates["value_json"] = normalizeWorldRuleValueJSON(val)
	}
	if val, exists := payload["source_turn"]; exists {
		i, ok := storylineIntPatchValue(val)
		if !ok || i < 0 {
			return nil, fmt.Errorf("source_turn must be a non-negative integer")
		}
		updates["source_turn"] = i
	}
	return updates, nil
}

func normalizeWorldRuleTrustPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		val, exists := payload[key]
		if !exists {
			continue
		}
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", key)
		}
		updates[key] = b
	}
	return updates, nil
}

func normalizeWorldRuleValueJSON(val any) string {
	if val == nil {
		return ""
	}
	if text, ok := val.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return ""
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err == nil {
			return mustCompactJSON(decoded)
		}
		return mustCompactJSON(text)
	}
	return mustCompactJSON(val)
}

// Episodes: R1 read/search, R2 generate/write

type episodeSummaryIDDeleter interface {
	DeleteEpisodeSummary(ctx context.Context, episodeID int64) error
}

type episodeSummaryRangeDeleter interface {
	DeleteEpisodeSummariesInRange(ctx context.Context, chatSessionID string, fromTurn, toTurn int) (int64, error)
}

func (s *Server) handleEpisodesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	items, err := s.Store.ListEpisodeSummaries(r.Context(), sid, limit, fromTurn, toTurn)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.EpisodeSummary{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	if items == nil {
		items = []store.EpisodeSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"episodes":        items,
		"count":           len(items),
	})
}

func (s *Server) handleEpisodeDetail(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("episode_id")
	episodeID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || episodeID <= 0 {
		writeBadRequest(w, "episode_id must be a positive integer")
		return
	}
	item, err := s.Store.GetEpisodeSummary(r.Context(), episodeID)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			item = nil
		} else if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status": "error",
				"detail": "episode not found",
			})
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"episode_id": episodeID,
		"found":      item != nil,
		"episode":    item,
	})
}

func (s *Server) handleEpisodeGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /episodes/generate")
		return
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "episode_store_not_available",
			"detail": "episode summary store is not available",
		})
		return
	}
	resp, statusCode := s.generateEpisodeSummaryResponse(r.Context(), req, episodeStore, "episode_generated")
	writeJSON(w, statusCode, resp)
}

func (s *Server) generateEpisodeSummaryResponse(ctx context.Context, req narrativeSearchRequest, episodeStore store.EpisodeSummaryStore, successCode string) (map[string]any, int) {
	interval := normalizedEpisodeInterval(req.Interval)
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 {
		fromTurn, toTurn = episodeRangeFromTurn(req.TurnIndex, interval)
	}
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		return map[string]any{
			"status":           "skipped",
			"code":             "episode_range_not_ready",
			"chat_session_id":  req.ChatSessionID,
			"turn_index":       req.TurnIndex,
			"interval":         interval,
			"blocking_reasons": []string{"episode_range_not_ready"},
			"llm_attempted":    false,
			"saved":            false,
		}, http.StatusOK
	}

	if !req.Force && s.Store != nil {
		if existing, err := s.Store.ListEpisodeSummaries(ctx, req.ChatSessionID, 0, fromTurn, toTurn); err == nil {
			for _, item := range existing {
				if item.FromTurn == fromTurn && item.ToTurn == toTurn {
					return map[string]any{
						"status":          "skipped",
						"code":            "episode_already_exists",
						"chat_session_id": req.ChatSessionID,
						"from_turn":       fromTurn,
						"to_turn":         toTurn,
						"interval":        interval,
						"episode":         item,
						"llm_attempted":   false,
						"saved":           false,
					}, http.StatusOK
				}
			}
		}
	}

	ev := s.collectNarrativeEvidence(ctx, req.ChatSessionID)
	chatLogs := filterChatLogsForTurnRange(ev.ChatLogs, fromTurn, toTurn, req.normalizedLimit(24))
	if len(chatLogs) == 0 {
		return map[string]any{
			"status":           "skipped",
			"code":             "no_chat_logs",
			"chat_session_id":  req.ChatSessionID,
			"from_turn":        fromTurn,
			"to_turn":          toTurn,
			"interval":         interval,
			"blocking_reasons": []string{"no_chat_logs"},
			"llm_attempted":    false,
			"saved":            false,
		}, http.StatusOK
	}

	replaced := int64(0)
	if req.Force {
		deleter, ok := s.Store.(episodeSummaryRangeDeleter)
		if !ok {
			return map[string]any{
				"status":          "error",
				"code":            "episode_range_delete_not_available",
				"chat_session_id": req.ChatSessionID,
				"from_turn":       fromTurn,
				"to_turn":         toTurn,
				"saved":           false,
			}, http.StatusServiceUnavailable
		}
		n, err := deleter.DeleteEpisodeSummariesInRange(ctx, req.ChatSessionID, fromTurn, toTurn)
		if err != nil {
			return map[string]any{
				"status":          "error",
				"code":            "episode_range_delete_failed",
				"chat_session_id": req.ChatSessionID,
				"from_turn":       fromTurn,
				"to_turn":         toTurn,
				"detail":          err.Error(),
				"saved":           false,
			}, http.StatusInternalServerError
		}
		replaced = n
	}

	episode, generationTrace := buildEpisodeSummaryForRangeWithArtifacts(req.ChatSessionID, fromTurn, toTurn, chatLogs,
		filterMemoriesForTurnRange(ev.Memories, req.ChatSessionID, fromTurn, toTurn),
		filterEvidenceForTurnRange(ev.Evidence, req.ChatSessionID, fromTurn, toTurn))
	if err := episodeStore.SaveEpisodeSummary(ctx, &episode); err != nil {
		return map[string]any{
			"status":          "error",
			"code":            "episode_summary_save_failed",
			"chat_session_id": req.ChatSessionID,
			"from_turn":       fromTurn,
			"to_turn":         toTurn,
			"detail":          err.Error(),
			"saved":           false,
		}, http.StatusInternalServerError
	}
	if successCode == "" {
		successCode = "episode_generated"
	}
	return map[string]any{
		"status":           "ok",
		"code":             successCode,
		"chat_session_id":  req.ChatSessionID,
		"from_turn":        fromTurn,
		"to_turn":          toTurn,
		"interval":         interval,
		"episode":          episode,
		"generation_trace": generationTrace,
		"llm_attempted":    false,
		"replaced":         replaced,
		"saved":            true,
	}, http.StatusOK
}

func (s *Server) handleChapterGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /chapters/generate")
		return
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "chapter_store_not_available",
			"detail": "chapter summary store is not available",
		})
		return
	}

	interval := normalizedChapterInterval(req.Interval)
	fromTurn, toTurn := req.normalizedTurnRange()
	intervalCheck := chapterIntervalCheck(req.TurnIndex, interval)
	if fromTurn == 0 || toTurn == 0 {
		if rawRange, ok := intervalCheck["range"].([]int); ok && len(rawRange) == 2 {
			fromTurn = rawRange[0]
			toTurn = rawRange[1]
		}
	}
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "skipped",
			"chat_session_id":  req.ChatSessionID,
			"turn_index":       req.TurnIndex,
			"interval":         interval,
			"interval_check":   intervalCheck,
			"blocking_reasons": []string{"chapter_range_not_ready"},
			"llm_attempted":    false,
			"saved":            false,
		})
		return
	}

	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	episodes := filterEpisodes(ev.EpisodeSummaries, "", fromTurn, toTurn, req.normalizedLimit(8))
	if len(episodes) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "skipped",
			"chat_session_id":  req.ChatSessionID,
			"from_turn":        fromTurn,
			"to_turn":          toTurn,
			"interval":         interval,
			"blocking_reasons": []string{"no_episode_summaries"},
			"llm_attempted":    false,
			"saved":            false,
		})
		return
	}

	chapterIndex := chapterIndexForRange(toTurn, interval)
	if !req.Force {
		existing, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 1)
		if err == nil && len(existing) > 0 {
			for _, ec := range existing {
				if ec.FromTurn == fromTurn && ec.ToTurn == toTurn {
					writeJSON(w, http.StatusOK, map[string]any{
						"status":          "skipped",
						"chat_session_id": req.ChatSessionID,
						"from_turn":       fromTurn,
						"to_turn":         toTurn,
						"already_exists":  true,
						"chapter":         ec,
						"llm_attempted":   false,
						"saved":           false,
					})
					return
				}
			}
		}
	}
	chapter, generationTrace := s.buildChapterSummaryForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, chapterIndex, episodes)
	if err := chapterStore.SaveChapterSummary(r.Context(), &chapter); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":          "error",
			"code":            "chapter_save_failed",
			"detail":          err.Error(),
			"chat_session_id": req.ChatSessionID,
			"llm_attempted":   generationTrace["llm_attempted"],
			"saved":           false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        req.ChatSessionID,
		"from_turn":              fromTurn,
		"to_turn":                toTurn,
		"chapter":                chapter,
		"chapter_result":         map[string]any{"checked": true, "triggered": true, "range": map[string]any{"from_turn": fromTurn, "to_turn": toTurn}},
		"input_stats":            chapterDenseInputStats(episodes),
		"generation_source":      generationTrace["generation_source"],
		"llm_attempted":          generationTrace["llm_attempted"],
		"llm_error":              generationTrace["llm_error"],
		"chapter_llm_trace":      generationTrace["llm_trace"],
		"chapter_shadow_compare": generationTrace["chapter_shadow_compare"],
		"saved":                  true,
	})
}

func (s *Server) handleArcGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /arcs/generate")
		return
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "arc_store_not_available", "detail": "arc summary store is not available"})
		return
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "chapter_store_not_available", "detail": "chapter summary store is required for arc generation"})
		return
	}
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "blocking_reasons": []string{"arc_range_not_ready"}, "saved": false})
		return
	}
	existing, err := arcStore.SearchArcSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 20)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	for _, arc := range existing {
		if arc.FromTurn == fromTurn && arc.ToTurn == toTurn && !req.Force {
			writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "already_exists": true, "arc": arc, "saved": false})
			return
		}
	}
	chapters, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, req.normalizedLimit(6))
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	if len(chapters) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "blocking_reasons": []string{"no_chapter_summaries"}, "saved": false})
		return
	}
	arcIndex := hierarchyIndexForRange(toTurn, 240)
	arc, generationTrace := s.buildArcSummaryForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, arcIndex, chapters)
	if err := arcStore.SaveArcSummary(r.Context(), req.ChatSessionID, &arc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "code": "arc_save_failed", "detail": err.Error(), "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"chat_session_id":    req.ChatSessionID,
		"from_turn":          fromTurn,
		"to_turn":            toTurn,
		"arc":                arc,
		"input_stats":        arcDenseInputStats(chapters, fromTurn, toTurn),
		"generation_source":  generationTrace["generation_source"],
		"llm_attempted":      generationTrace["llm_attempted"],
		"llm_error":          generationTrace["llm_error"],
		"arc_llm_trace":      generationTrace["llm_trace"],
		"arc_shadow_compare": generationTrace["shadow_compare"],
		"lifecycle":          map[string]any{"final_status": arc.ArcStatus, "status_reason": generationTrace["status_reason"]},
		"saved":              true,
	})
}

func (s *Server) handleSagaGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /sagas/generate")
		return
	}
	sagaStore, ok := s.Store.(store.SagaDigestStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "saga_store_not_available", "detail": "saga digest store is not available"})
		return
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "arc_store_not_available", "detail": "arc summary store is required for saga generation"})
		return
	}
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "blocking_reasons": []string{"saga_range_not_ready"}, "saved": false})
		return
	}
	existing, err := sagaStore.SearchSagaDigests(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 20)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	for _, saga := range existing {
		if saga.FromTurn == fromTurn && saga.ToTurn == toTurn && !req.Force {
			writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "already_exists": true, "saga": saga, "saved": false})
			return
		}
	}
	arcs, err := arcStore.SearchArcSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, req.normalizedLimit(6))
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	if len(arcs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "blocking_reasons": []string{"no_arc_summaries"}, "saved": false})
		return
	}
	saga, generationTrace := s.buildSagaDigestForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, arcs)
	if err := sagaStore.SaveSagaDigest(r.Context(), req.ChatSessionID, &saga); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "code": "saga_save_failed", "detail": err.Error(), "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "ok",
		"chat_session_id":     req.ChatSessionID,
		"from_turn":           fromTurn,
		"to_turn":             toTurn,
		"saga":                saga,
		"input_stats":         sagaDenseInputStats(arcs, fromTurn, toTurn),
		"generation_source":   generationTrace["generation_source"],
		"llm_attempted":       generationTrace["llm_attempted"],
		"llm_error":           generationTrace["llm_error"],
		"saga_llm_trace":      generationTrace["llm_trace"],
		"saga_shadow_compare": generationTrace["shadow_compare"],
		"saved":               true,
	})
}

func (s *Server) handleChapterDryRun(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.TurnIndex < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "turn_index must be >= 0"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	interval := normalizedChapterInterval(req.Interval)
	intervalCheck := chapterIntervalCheck(req.TurnIndex, interval)
	fromTurn, toTurn := 0, 0
	candidateRange := any(nil)
	blockingReasons := []string{}
	warnings := []string{}
	turnSpan := any(nil)
	if rawRange, ok := intervalCheck["range"].([]int); ok && len(rawRange) == 2 {
		fromTurn = rawRange[0]
		toTurn = rawRange[1]
		span := (toTurn - fromTurn) + 1
		turnSpan = span
		candidateRange = map[string]any{
			"from_turn": fromTurn,
			"to_turn":   toTurn,
			"turn_span": span,
		}
	} else if reason, _ := intervalCheck["reason"].(string); reason != "" {
		blockingReasons = append(blockingReasons, reason)
	}
	episodes := []store.EpisodeSummary{}
	if fromTurn > 0 || toTurn > 0 {
		episodes = filterEpisodes(ev.EpisodeSummaries, "", fromTurn, toTurn, req.normalizedLimit(8))
		if len(episodes) == 0 {
			blockingReasons = append(blockingReasons, "no_episode_summaries")
		} else if len(episodes) < 4 || len(episodes) > 8 {
			warnings = append(warnings, "episode_count_outside_recommended_window")
		}
		if span, ok := turnSpan.(int); ok && (span < 40 || span > 80) {
			warnings = append(warnings, "turn_span_outside_recommended_window")
		}
	}
	episodeInputs := episodeInputPreviews(episodes, req.normalizedLimit(8))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"mode":             "dry_run",
		"chat_session_id":  req.ChatSessionID,
		"turn_index":       req.TurnIndex,
		"interval":         interval,
		"force":            req.Force,
		"triggered":        candidateRange != nil,
		"interval_check":   intervalCheck,
		"candidate_range":  candidateRange,
		"already_exists":   false,
		"ready":            len(blockingReasons) == 0 && candidateRange != nil,
		"blocking_reasons": blockingReasons,
		"warnings":         warnings,
		"input_stats": map[string]any{
			"episode_count":             len(episodes),
			"episode_count_recommended": len(episodes) >= 4 && len(episodes) <= 8,
			"turn_span":                 turnSpan,
			"turn_span_recommended":     turnSpanRecommended(turnSpan),
		},
		"episode_inputs": episodeInputs,
	})
}

func (s *Server) handleChapterSearch(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "query is required"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	fromTurn, toTurn := req.normalizedTurnRange()
	limit := req.normalizedLimit(3)
	results := []any{}
	if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
		chapters, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit))
		if err == nil {
			sortChapterSummariesByDensePriority(chapters)
			if len(chapters) > limit {
				chapters = chapters[:limit]
			}
			results = chapterResultsWithEvidence(chapters, ev.Evidence)
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
			return
		}
	}
	if len(results) < limit {
		episodes := filterEpisodes(ev.EpisodeSummaries, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit-len(results)))
		sortEpisodeSummariesByDensePriority(episodes)
		if remaining := limit - len(results); len(episodes) > remaining {
			episodes = episodes[:remaining]
		}
		results = append(results, episodeResultsWithEvidence(episodes, ev.Evidence)...)
	}
	if ev.ResumePack != nil && ev.ResumePack.Chapter != nil && matchesChapter(ev.ResumePack.Chapter, req.Query) && len(results) < limit {
		item := map[string]any{
			"id":            ev.ResumePack.Chapter.ID,
			"source":        "resume_pack_chapter",
			"from_turn":     ev.ResumePack.Chapter.FromTurn,
			"to_turn":       ev.ResumePack.Chapter.ToTurn,
			"title":         ev.ResumePack.Chapter.ChapterTitle,
			"summary_text":  ev.ResumePack.Chapter.SummaryText,
			"resume_text":   ev.ResumePack.Chapter.ResumeText,
			"chapter_index": ev.ResumePack.Chapter.ChapterIndex,
		}
		for k, v := range denseSummarySurfaceFields("chapter", ev.ResumePack.Chapter.ID, ev.ResumePack.Chapter.FromTurn, ev.ResumePack.Chapter.ToTurn, q1FirstNonEmptyString(ev.ResumePack.Chapter.ResumeText, ev.ResumePack.Chapter.SummaryText, ev.ResumePack.Chapter.ChapterTitle), chapterDenseStructuredPayload(*ev.ResumePack.Chapter), chapterDensePriorityScores(*ev.ResumePack.Chapter), ev.Evidence) {
			item[k] = v
		}
		results = append(results, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": req.ChatSessionID,
		"query":           truncateString(req.Query, 200),
		"chapters":        results,
		"count":           len(results),
	})
}

func (s *Server) handleEpisodeSearch(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "query is required"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	fromTurn, toTurn := req.normalizedTurnRange()
	limit := req.normalizedLimit(3)
	episodes := filterEpisodes(ev.EpisodeSummaries, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit))
	sortEpisodeSummariesByDensePriority(episodes)
	if len(episodes) > limit {
		episodes = episodes[:limit]
	}
	results := episodeResultsWithEvidence(episodes, ev.Evidence)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": req.ChatSessionID,
		"query":           truncateString(req.Query, 200),
		"episodes":        results,
		"count":           len(episodes),
	})
}

func (s *Server) handleEpisodePatch(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "PATCH /episodes/{episode_id}")
}

func (s *Server) handleEpisodeDelete(w http.ResponseWriter, r *http.Request) {
	episodeID, ok := parseNarrativeInt64Path(w, r, "episode_id")
	if !ok {
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "DELETE /episodes/{episode_id}")
		return
	}
	deleter, ok := s.Store.(episodeSummaryIDDeleter)
	if !ok {
		writeShadowGuard(w, "DELETE /episodes/{episode_id}")
		return
	}
	if err := deleter.DeleteEpisodeSummary(r.Context(), episodeID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("episode %d not found", episodeID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /episodes/{episode_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"episode_id": episodeID,
		"deleted":    true,
	})
}

func (s *Server) handleEpisodeRegenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /episodes/regenerate")
		return
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "episode_store_not_available",
			"detail": "episode summary store is not available",
		})
		return
	}
	req.Force = true
	resp, statusCode := s.generateEpisodeSummaryResponse(r.Context(), req, episodeStore, "episode_regenerated")
	writeJSON(w, statusCode, resp)
}

func (s *Server) handleEpisodeMerge(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /episodes/merge")
}

// Pending threads: R2 live store mutations

func (s *Server) handlePendingThreadPatch(w http.ResponseWriter, r *http.Request) {
	hookID, ok := parseNarrativeInt64Path(w, r, "hook_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchPendingThread(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /pending-threads/{hook_id}")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizePendingThreadPatchPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchPendingThread(r.Context(), hookID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("pending thread %d not found", hookID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /pending-threads/{hook_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"hook_id":        hookID,
		"updated_fields": updatedFields,
	}
	updatedValues := make(map[string]any)
	for _, key := range updatedFields {
		if val, exists := updates[key]; exists {
			updatedValues[key] = val
		}
	}
	if len(updatedValues) > 0 {
		resp["updated_values"] = updatedValues
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePendingThreadTrust(w http.ResponseWriter, r *http.Request) {
	hookID, ok := parseNarrativeInt64Path(w, r, "hook_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchPendingThreadTrust(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /pending-threads/{hook_id}/trust")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeStorylineTrustPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchPendingThreadTrust(r.Context(), hookID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("pending thread %d not found", hookID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /pending-threads/{hook_id}/trust")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"hook_id":        hookID,
		"updated_fields": updatedFields,
	}
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePendingThreadDelete(w http.ResponseWriter, r *http.Request) {
	hookID, ok := parseNarrativeInt64Path(w, r, "hook_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		DeletePendingThread(context.Context, int64) error
	})
	if !ok {
		writeShadowGuard(w, "DELETE /pending-threads/{hook_id}")
		return
	}
	if err := mutator.DeletePendingThread(r.Context(), hookID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("pending thread %d not found", hookID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /pending-threads/{hook_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"deleted_id": hookID,
	})
}

// Metrics: R1 read-only evidence surfaces.

func (s *Server) handleMetricsLC1C(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	ctx := r.Context()
	policyVersion := "lc1c.v1"
	turnWindow := 300
	status := "ok"
	latestTurnIndex := 0
	windowStart := 1
	canonicalChars := 0
	denseChars := 0
	liveLedgerChars := 0

	if sid != "" && s.Store != nil {
		logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err == nil {
			for _, l := range logs {
				if l.TurnIndex > latestTurnIndex {
					latestTurnIndex = l.TurnIndex
				}
			}
		}
		if latestTurnIndex > 0 {
			windowStart = latestTurnIndex - turnWindow + 1
			if windowStart < 1 {
				windowStart = 1
			}
		}

		layers, err := s.Store.ListCanonicalStateLayers(ctx, sid, "")
		if err == nil {
			for _, layer := range layers {
				canonicalChars += len([]rune(layer.Content))
			}
		}

		eps, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, windowStart, 0)
		if err == nil {
			for _, ep := range eps {
				denseChars += len([]rune(ep.SummaryText))
			}
		}

		pack, err := s.Store.GetResumePack(ctx, sid, "resume")
		if err == nil && pack != nil {
			if pack.Chapter != nil {
				denseChars += len([]rune(pack.Chapter.SummaryText)) + len([]rune(pack.Chapter.ResumeText))
			}
			if pack.Arc != nil {
				denseChars += len([]rune(pack.Arc.CoreConflict)) + len([]rune(pack.Arc.ArcResumeText))
			}
			if pack.Saga != nil {
				denseChars += len([]rune(pack.Saga.SagaSummary)) + len([]rune(pack.Saga.ResumePackText))
			}
		}

		ev := s.collectNarrativeEvidence(ctx, sid)
		lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
		storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
		director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
		stateStatus := "skeleton"
		if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
			stateStatus = "heuristic"
		}
		liveLedgerChars = pythonDefaultJSONRuneLen(buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn))
	}

	totalChars := canonicalChars + denseChars + liveLedgerChars

	counts := map[string]any{
		"canonical_layers": 0,
		"episodes":         0,
		"chapters":         0,
		"arcs":             0,
		"sagas":            0,
	}
	if sid != "" && s.Store != nil {
		layers, _ := s.Store.ListCanonicalStateLayers(ctx, sid, "")
		counts["canonical_layers"] = len(layers)
		eps, _ := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
		counts["episodes"] = len(eps)
		pack, _ := s.Store.GetResumePack(ctx, sid, "resume")
		if pack != nil {
			if pack.Chapter != nil {
				counts["chapters"] = 1
			}
			if pack.Arc != nil {
				counts["arcs"] = 1
			}
			if pack.Saga != nil {
				counts["sagas"] = 1
			}
		}
	}

	if sid == "" {
		status = "off"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id": sid,
		"memory_footprint": map[string]any{
			"policy_version":        policyVersion,
			"turn_window":           turnWindow,
			"status":                status,
			"latest_turn_index":     nullableInt(latestTurnIndex),
			"window_start_turn":     windowStart,
			"canonical_state_chars": canonicalChars,
			"dense_summary_chars":   denseChars,
			"live_ledger_chars":     liveLedgerChars,
			"total_chars":           totalChars,
			"counts":                counts,
		},
		"status": "ok",
	})
}

// narrativeEvidence aggregates store-backed read evidence for a single session.
type narrativeEvidence struct {
	ChatLogs             []store.ChatLog
	Memories             []store.Memory
	Evidence             []store.DirectEvidence
	KGTriples            []store.KGTriple
	Storylines           []store.Storyline
	WorldRules           []store.WorldRule
	CharacterStates      []store.CharacterState
	PendingThreads       []store.PendingThread
	ActiveStates         []store.ActiveState
	CanonicalStateLayers []store.CanonicalStateLayer
	EpisodeSummaries     []store.EpisodeSummary
	ResumePack           *store.ResumePack
	AuditLogs            []store.AuditLog
	CriticFeedback       []store.CriticFeedback
	Disabled             bool
}

func (s *Server) collectNarrativeEvidence(ctx context.Context, chatSessionID string) narrativeEvidence {
	var ev narrativeEvidence
	if s.Store == nil {
		ev.Disabled = true
		return ev
	}
	disabled := false

	mark := func(err error) {
		if err != nil && errors.Is(err, store.ErrNotEnabled) {
			disabled = true
		}
	}

	if v, err := s.Store.ListChatLogs(ctx, chatSessionID, 0, 0); err != nil {
		mark(err)
	} else {
		ev.ChatLogs = v
	}
	if v, err := s.Store.ListMemories(ctx, chatSessionID, 0, 0); err != nil {
		mark(err)
	} else {
		ev.Memories = v
	}
	if v, err := s.Store.ListEvidence(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.Evidence = v
	}
	if v, err := s.Store.ListKGTriples(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.KGTriples = v
	}
	if v, err := s.Store.ListStorylines(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.Storylines = v
	}
	if v, err := s.Store.ListWorldRules(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.WorldRules = v
	}
	if v, err := s.Store.ListCharacterStates(ctx, chatSessionID); err != nil {
		mark(err)
	} else {
		ev.CharacterStates = v
	}
	if v, err := s.Store.ListPendingThreads(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.PendingThreads = v
	}
	if v, err := s.Store.ListActiveStates(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.ActiveStates = v
	}
	if v, err := s.Store.ListCanonicalStateLayers(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.CanonicalStateLayers = v
	}
	if v, err := s.Store.ListEpisodeSummaries(ctx, chatSessionID, 0, 0, 0); err != nil {
		mark(err)
	} else {
		ev.EpisodeSummaries = v
	}
	if v, err := s.Store.GetResumePack(ctx, chatSessionID, ""); err != nil {
		mark(err)
	} else {
		ev.ResumePack = v
	}
	if v, err := s.Store.ListAuditLogs(ctx, chatSessionID, "", 1000); err != nil {
		mark(err)
	} else {
		ev.AuditLogs = v
	}
	if v, err := s.Store.ListCriticFeedback(ctx, chatSessionID, "", 0); err != nil {
		mark(err)
	} else {
		ev.CriticFeedback = v
	}
	ev.Disabled = disabled
	return ev
}

func sourceFromEvidence(ev narrativeEvidence) string {
	if ev.Disabled {
		return "shadow-degraded"
	}
	return "shadow"
}

func storeStatusFromEvidence(ev narrativeEvidence) string {
	if ev.Disabled {
		return "disabled"
	}
	return "active"
}

func latestTurnIndex(chatLogs []store.ChatLog) any {
	if len(chatLogs) == 0 {
		return nil
	}
	maxTurn := 0
	for _, item := range chatLogs {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
	}
	return maxTurn
}

func countAuditEvents(items []store.AuditLog, eventType string) int {
	count := 0
	for _, item := range items {
		if item.EventType == eventType {
			count++
		}
	}
	return count
}

func regressionCorpusManifest() map[string]any {
	return map[string]any{
		"policy_version":     "lc1r.v1",
		"definition_state":   "defined",
		"execution_state":    "pending_restart_replay",
		"release_gate_ready": false,
		"corpus": []any{
			map[string]any{"step": "14", "lane": "character_and_guidance", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "15", "lane": "inspection_and_context", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16", "lane": "retrieval_temporal_foundation", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16.5", "lane": "adaptive_governor", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
			map[string]any{"step": "16.8", "lane": "replay_gate_and_stale_arc_suppression", "definition_state": "defined", "execution_state": "pending_restart_replay", "suite_refs": []string{"backend replay", "runtime contract test"}},
		},
	}
}

func step17BundleClosure() map[string]any {
	return map[string]any{
		"policy_version":      "lc1s.v1",
		"bundle_label":        "Archive Center Release 1.0.0",
		"runtime_version":     "1.0.0",
		"step_context":        "21st step",
		"closure_status":      "closed",
		"release_gate_closed": true,
		"closure_scope":       "bundle_release_artifact_sync",
		"closure_mode":        "bundle_release_closed_session_cutover_separate",
		"closure_record": map[string]any{
			"source":      "historical_step17_release_gate_record",
			"recorded_at": "2026-05-07",
			"meaning":     "Step 17 closure record remains preserved inside the Step 21 Release 1.0.0 source artifact.",
		},
		"checklist": []any{
			map[string]any{"item": "historical_release_gate_record", "passed": true, "detail": "2026-05-07 current-candidate closure record"},
			map[string]any{"item": "bundle_release_artifact_sync", "passed": true, "detail": "README, BUNDLE_NOTES, plugin version markers, and runtime metadata align to the Step 21 Release 1.0.0 target while preserving Step 17 closure carry-in"},
			map[string]any{"item": "runtime_gate_surface_present", "passed": true, "detail": "Inspection, visibility, and adoption/release read-only panels are present in the bundle runtime"},
			map[string]any{"item": "fresh_bundle_embedding_baseline_ready", "passed": true, "detail": "default embedding model aligns to text-embedding-3-small so startup preflight is not falsely blocked"},
		},
		"warnings": []string{
			"Session-local adoption/release gates may remain hold or pending until live shadow signal and operator evidence are supplied.",
			"This snapshot does not approve live limited cutover or default runtime change.",
		},
	}
}

func resolveStorylineReferenceTurn(items []store.Storyline, rawCurrentTurn string) *int {
	if raw := strings.TrimSpace(rawCurrentTurn); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			return &parsed
		}
	}
	var latest *int
	for _, item := range items {
		for _, candidate := range []int{item.LastEvidenceTurn, item.LastTurn} {
			if candidate < 0 {
				continue
			}
			if latest == nil || candidate > *latest {
				value := candidate
				latest = &value
			}
		}
	}
	return latest
}

func storylineResponseItems(items []store.Storyline, referenceTurn *int) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		snapshot := storylineStaleSnapshot(item, referenceTurn)
		keyPointsJSON := normalizeStorylineListJSONString(item.KeyPointsJSON)
		tensionsJSON := normalizeStorylineListJSONString(item.OngoingTensionsJSON)
		out = append(out, map[string]any{
			"id":                    item.ID,
			"chat_session_id":       item.ChatSessionID,
			"name":                  item.Name,
			"status":                item.Status,
			"entities_json":         item.EntitiesJSON,
			"current_context":       item.CurrentContext,
			"key_points_json":       keyPointsJSON,
			"ongoing_tensions_json": tensionsJSON,
			"confidence":            item.Confidence,
			"evidence_count":        item.EvidenceCount,
			"last_evidence_turn":    item.LastEvidenceTurn,
			"last_observed_turn":    snapshot["last_observed_turn"],
			"freshness_turn_gap":    snapshot["freshness_turn_gap"],
			"stale_after_turns":     snapshot["stale_after_turns"],
			"is_stale":              snapshot["is_stale"],
			"stale_reason":          snapshot["stale_reason"],
			"first_turn":            item.FirstTurn,
			"last_turn":             item.LastTurn,
			"pinned":                item.Pinned,
			"suppressed":            item.Suppressed,
			"user_corrected":        item.UserCorrected,
			"created_at":            nullableTime(item.CreatedAt),
			"updated_at":            nullableTime(item.UpdatedAt),
		})
	}
	return out
}

func storylineStaleSnapshot(item store.Storyline, referenceTurn *int) map[string]any {
	evidenceCount := item.EvidenceCount
	if evidenceCount < 0 {
		evidenceCount = 0
	}
	lastObserved := item.LastEvidenceTurn
	if lastObserved <= 0 {
		lastObserved = item.LastTurn
	}
	var freshness any
	if referenceTurn != nil && lastObserved >= 0 {
		gap := *referenceTurn - lastObserved
		if gap < 0 {
			gap = 0
		}
		freshness = gap
	}
	staleAfter := evidenceCount + 2
	if staleAfter < 3 {
		staleAfter = 3
	}
	if staleAfter > 8 {
		staleAfter = 8
	}
	isStale := false
	var staleReason any
	if item.Status == "active" {
		if gap, ok := freshness.(int); ok && gap >= staleAfter {
			isStale = true
			if evidenceCount <= 1 {
				staleReason = "low_evidence_gap"
			} else {
				staleReason = "freshness_gap"
			}
		}
	}
	return map[string]any{
		"last_observed_turn": lastObserved,
		"freshness_turn_gap": freshness,
		"stale_after_turns":  staleAfter,
		"is_stale":           isStale,
		"stale_reason":       staleReason,
	}
}

type storylineSelectionEntry struct {
	Item             store.Storyline
	Snapshot         map[string]any
	LastObservedTurn int
	FreshnessGap     *int
	StaleAfterTurns  int
	IsStale          bool
	StaleReason      any
	Confidence       float64
}

type storylineSupervisorSelection struct {
	ReferenceTurn *int
	Selected      []storylineSelectionEntry
	Dropped       []storylineSelectionEntry
	Resolved      []storylineSelectionEntry
	Suppressed    []storylineSelectionEntry
}

func selectStorylinesForSupervisor(items []store.Storyline, referenceTurn *int, limit int) storylineSupervisorSelection {
	if limit <= 0 {
		limit = 5
	}
	if referenceTurn == nil {
		referenceTurn = resolveStorylineReferenceTurn(items, "")
	}
	selection := storylineSupervisorSelection{ReferenceTurn: referenceTurn}
	active := make([]storylineSelectionEntry, 0, len(items))
	for _, item := range items {
		entry := buildStorylineSelectionEntry(item, referenceTurn)
		status := normalizedStorylineStatus(item.Status)
		if item.Suppressed {
			selection.Suppressed = append(selection.Suppressed, entry)
			continue
		}
		if status != "active" {
			if isResolvedStorylineStatus(status) {
				selection.Resolved = append(selection.Resolved, entry)
			} else {
				selection.Dropped = append(selection.Dropped, entry)
			}
			continue
		}
		active = append(active, entry)
	}
	sort.SliceStable(active, func(i, j int) bool {
		return storylineSelectionLess(active[i], active[j])
	})
	pinned := make([]storylineSelectionEntry, 0, len(active))
	fresh := make([]storylineSelectionEntry, 0, len(active))
	stale := make([]storylineSelectionEntry, 0, len(active))
	for _, entry := range active {
		if entry.Item.Pinned {
			pinned = append(pinned, entry)
		} else if entry.IsStale {
			stale = append(stale, entry)
		} else {
			fresh = append(fresh, entry)
		}
	}
	if len(pinned) > 0 || len(fresh) > 0 {
		for _, group := range [][]storylineSelectionEntry{pinned, fresh} {
			for _, entry := range group {
				if len(selection.Selected) < limit {
					selection.Selected = append(selection.Selected, entry)
				} else {
					selection.Dropped = append(selection.Dropped, entry)
				}
			}
		}
		selection.Dropped = append(selection.Dropped, stale...)
		return selection
	}
	if len(stale) > 0 {
		selection.Selected = append(selection.Selected, stale[0])
		selection.Dropped = append(selection.Dropped, stale[1:]...)
	}
	return selection
}

func selectedStorylineItems(selection storylineSupervisorSelection) []store.Storyline {
	out := make([]store.Storyline, 0, len(selection.Selected))
	for _, entry := range selection.Selected {
		out = append(out, entry.Item)
	}
	return out
}

func storylineDetailCompareText(value string) string {
	clean := strings.TrimLeftFunc(strings.TrimSpace(value), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '-', 0x2022, 0x26A1:
			return true
		default:
			return false
		}
	})
	return strings.ToLower(strings.Join(strings.Fields(clean), " "))
}

func isStorylineSelfEchoDetail(item store.Storyline, detail string) bool {
	key := storylineDetailCompareText(detail)
	if key == "" {
		return false
	}
	name := strings.TrimSpace(item.Name)
	context := strings.TrimSpace(item.CurrentContext)
	refs := []string{name, context}
	if name != "" && context != "" {
		refs = append(refs, name+" "+string(rune(0x2014))+" "+context, name+" - "+context)
	}
	for _, ref := range refs {
		if storylineDetailCompareText(ref) == key {
			return true
		}
	}
	return false
}

func filterStorylineContextDetails(item store.Storyline, details []string) []string {
	out := make([]string, 0, len(details))
	seen := make(map[string]bool, len(details))
	for _, detail := range details {
		clean := strings.TrimSpace(detail)
		key := storylineDetailCompareText(clean)
		if key == "" || seen[key] || isStorylineSelfEchoDetail(item, clean) {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func formatStorylinesForSupervisor(selection storylineSupervisorSelection) string {
	lines := make([]string, 0, len(selection.Selected)+len(selection.Resolved)+2)
	if len(selection.Selected) > 0 {
		lines = append(lines, "[Storylines]")
		for _, entry := range selection.Selected {
			desc := strings.TrimSpace(entry.Item.CurrentContext)
			if desc == "" {
				desc = strings.TrimSpace(entry.Item.Name)
			}
			if desc == "" {
				continue
			}
			keyPoints := filterStorylineContextDetails(entry.Item, parseStorylineListJSON(entry.Item.KeyPointsJSON))
			tensions := filterStorylineContextDetails(entry.Item, parseStorylineListJSON(entry.Item.OngoingTensionsJSON))
			lines = append(lines, fmt.Sprintf(
				"- %s (confidence=%.2f, evidence=%d, freshness_gap=%s)",
				truncateRunes(desc, 180),
				entry.Confidence,
				entry.Item.EvidenceCount,
				storylineGapLabel(entry.FreshnessGap),
			))
			if len(keyPoints) > 0 {
				lines = append(lines, fmt.Sprintf("  key_points: %s", strings.Join(keyPoints[:minInt(len(keyPoints), 3)], " / ")))
			}
			if len(tensions) > 0 {
				lines = append(lines, fmt.Sprintf("  tensions: %s", strings.Join(tensions[:minInt(len(tensions), 3)], " / ")))
			}
		}
	}
	if len(selection.Resolved) > 0 {
		lines = append(lines, "[Resolved Storylines Summary]")
		for i, entry := range selection.Resolved {
			if i >= 2 {
				break
			}
			name := strings.TrimSpace(entry.Item.Name)
			if name == "" {
				name = fmt.Sprintf("storyline_%d", entry.Item.ID)
			}
			lines = append(lines, fmt.Sprintf("- %s resolved at turn %s", truncateRunes(name, 120), storylineObservedLabel(entry.LastObservedTurn)))
		}
	}
	return strings.Join(lines, "\n")
}

func storylineSelectionSummary(selection storylineSupervisorSelection) map[string]any {
	selected := make([]map[string]any, 0, len(selection.Selected))
	for _, entry := range selection.Selected {
		selected = append(selected, storylineSelectionEntryMap(entry))
	}
	dropped := make([]map[string]any, 0, len(selection.Dropped))
	for _, entry := range selection.Dropped {
		dropped = append(dropped, storylineSelectionEntryMap(entry))
	}
	resolved := make([]map[string]any, 0, minInt(len(selection.Resolved), 3))
	for i, entry := range selection.Resolved {
		if i >= 3 {
			break
		}
		resolved = append(resolved, storylineSelectionEntryMap(entry))
	}
	suppressed := make([]map[string]any, 0, minInt(len(selection.Suppressed), 3))
	for i, entry := range selection.Suppressed {
		if i >= 3 {
			break
		}
		suppressed = append(suppressed, storylineSelectionEntryMap(entry))
	}
	staleDropped := 0
	for _, entry := range selection.Dropped {
		if entry.IsStale {
			staleDropped++
		}
	}
	staleSelected := 0
	for _, entry := range selection.Selected {
		if entry.IsStale {
			staleSelected++
		}
	}
	return map[string]any{
		"policy_version":           "storyline_selection.h2d.go.v1",
		"reference_turn":           nullableIntPtr(selection.ReferenceTurn),
		"selected_count":           len(selection.Selected),
		"dropped_count":            len(selection.Dropped),
		"resolved_summary_count":   len(selection.Resolved),
		"suppressed_count":         len(selection.Suppressed),
		"stale_selected_count":     staleSelected,
		"stale_dropped_count":      staleDropped,
		"selected":                 selected,
		"dropped":                  dropped,
		"resolved_summary":         resolved,
		"suppressed_summary":       suppressed,
		"fresh_rows_take_priority": true,
	}
}

func buildStorylineSelectionEntry(item store.Storyline, referenceTurn *int) storylineSelectionEntry {
	snapshot := storylineStaleSnapshot(item, referenceTurn)
	entry := storylineSelectionEntry{
		Item:       item,
		Snapshot:   snapshot,
		Confidence: normalizeStorylineConfidence(item.Confidence),
	}
	if v, ok := snapshot["last_observed_turn"].(int); ok {
		entry.LastObservedTurn = v
	}
	if v, ok := snapshot["freshness_turn_gap"].(int); ok {
		value := v
		entry.FreshnessGap = &value
	}
	if v, ok := snapshot["stale_after_turns"].(int); ok {
		entry.StaleAfterTurns = v
	}
	if v, ok := snapshot["is_stale"].(bool); ok {
		entry.IsStale = v
	}
	entry.StaleReason = snapshot["stale_reason"]
	return entry
}

func storylineSelectionEntryMap(entry storylineSelectionEntry) map[string]any {
	return map[string]any{
		"id":                 entry.Item.ID,
		"name":               entry.Item.Name,
		"status":             normalizedStorylineStatus(entry.Item.Status),
		"confidence":         entry.Confidence,
		"evidence_count":     entry.Item.EvidenceCount,
		"last_evidence_turn": nullablePositiveInt(entry.Item.LastEvidenceTurn),
		"last_observed_turn": nullablePositiveInt(entry.LastObservedTurn),
		"freshness_turn_gap": entry.Snapshot["freshness_turn_gap"],
		"stale_after_turns":  entry.StaleAfterTurns,
		"is_stale":           entry.IsStale,
		"stale_reason":       entry.StaleReason,
		"pinned":             entry.Item.Pinned,
		"suppressed":         entry.Item.Suppressed,
		"user_corrected":     entry.Item.UserCorrected,
	}
}

func storylineSelectionLess(a, b storylineSelectionEntry) bool {
	if a.IsStale != b.IsStale {
		return !a.IsStale
	}
	if a.Item.Pinned != b.Item.Pinned {
		return a.Item.Pinned
	}
	aGap, bGap := storylineSortGap(a), storylineSortGap(b)
	if aGap != bGap {
		return aGap < bGap
	}
	if a.Confidence != b.Confidence {
		return a.Confidence > b.Confidence
	}
	if a.Item.EvidenceCount != b.Item.EvidenceCount {
		return a.Item.EvidenceCount > b.Item.EvidenceCount
	}
	if a.LastObservedTurn != b.LastObservedTurn {
		return a.LastObservedTurn > b.LastObservedTurn
	}
	if a.Item.LastTurn != b.Item.LastTurn {
		return a.Item.LastTurn > b.Item.LastTurn
	}
	return a.Item.ID < b.Item.ID
}

func storylineSortGap(entry storylineSelectionEntry) int {
	if entry.FreshnessGap == nil {
		return 1_000_000
	}
	return *entry.FreshnessGap
}

func normalizeStorylineConfidence(value float64) float64 {
	if value <= 0 {
		return 0.5
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizedStorylineStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "active"
	}
	return status
}

func isResolvedStorylineStatus(status string) bool {
	switch status {
	case "resolved", "completed", "closed", "done":
		return true
	default:
		return false
	}
}

func storylineGapLabel(gap *int) string {
	if gap == nil {
		return "unknown"
	}
	return strconv.Itoa(*gap)
}

func storylineObservedLabel(turn int) string {
	if turn <= 0 {
		return "unknown"
	}
	return strconv.Itoa(turn)
}

func parseStorylineListJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return []string{}
	}
	items, ok := compactStorylineTextList(decoded)
	if !ok {
		return []string{}
	}
	return items
}

func normalizeStorylineListJSONString(raw string) string {
	items := parseStorylineListJSON(raw)
	if len(items) == 0 {
		return ""
	}
	return mustCompactJSON(items)
}

func isValidWorldRuleScope(scope string) bool {
	switch store.NormalizeWorldRuleScope(scope) {
	case "root", "region", "location", "faction", "system", "session":
		return true
	default:
		return false
	}
}

func worldRuleScopeChain(scope string) []string {
	return store.WorldRuleScopeChain(scope)
}

func (s *Server) resolveActiveScope(ctx context.Context, sid string) (*store.SessionActiveScope, string, error) {
	activeScopeStore, ok := s.Store.(store.ActiveScopeStore)
	if ok {
		item, err := activeScopeStore.GetActiveScope(ctx, sid)
		if err == nil && item != nil {
			if strings.TrimSpace(item.ActiveScope) == "" {
				item.ActiveScope = "root"
			}
			return item, "store", nil
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return nil, "", err
		}
	}
	return &store.SessionActiveScope{
		ChatSessionID: sid,
		ActiveScope:   "root",
	}, "default", nil
}

func activeScopeResponse(sid string, item *store.SessionActiveScope, source string) map[string]any {
	activeScope := "root"
	scopeName := ""
	var updatedAt time.Time
	if item != nil {
		if strings.TrimSpace(item.ActiveScope) != "" {
			activeScope = strings.TrimSpace(item.ActiveScope)
		}
		scopeName = strings.TrimSpace(item.ScopeName)
		updatedAt = item.UpdatedAt
	}
	return map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"active_scope":    activeScope,
		"scope_name":      nullableString(scopeName),
		"scope_chain":     worldRuleScopeChain(activeScope),
		"source":          source,
		"updated_at":      nullableTime(updatedAt),
	}
}

func worldRuleResponseItems(items []store.WorldRule, activeScope string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		originalScope := item.Scope
		scope := store.NormalizeWorldRuleScope(item.Scope)
		m := map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"scope":           scope,
			"scope_name":      item.ScopeName,
			"category":        item.Category,
			"key":             item.Key,
			"value_json":      item.ValueJSON,
			"genre":           item.Genre,
			"source_turn":     item.SourceTurn,
			"pinned":          item.Pinned,
			"suppressed":      item.Suppressed,
			"user_corrected":  item.UserCorrected,
			"created_at":      nullableTime(item.CreatedAt),
			"updated_at":      nullableTime(item.UpdatedAt),
		}
		if originalScope != "" && originalScope != scope {
			m["original_scope"] = originalScope
		}
		if activeScope != "" {
			m["inherited"] = scope != store.NormalizeWorldRuleScope(activeScope)
		}
		out = append(out, m)
	}
	return out
}

func nullableIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTime(v time.Time) any {
	if v.IsZero() {
		return nil
	}
	return v.Format(time.RFC3339Nano)
}

func boundedQueryLimit(r *http.Request, defaultLimit, maxLimit int) int {
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	if maxLimit <= 0 {
		maxLimit = defaultLimit
	}
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

type narrativeSearchRequest struct {
	ChatSessionID string `json:"chat_session_id"`
	Query         string `json:"query"`
	Limit         int    `json:"limit"`
	TopK          int    `json:"top_k"`
	FromTurn      int    `json:"from_turn"`
	ToTurn        int    `json:"to_turn"`
	TurnIndex     int    `json:"turn_index"`
	Interval      int    `json:"interval"`
	Force         bool   `json:"force"`
}

func (req narrativeSearchRequest) normalizedLimit(defaultLimit ...int) int {
	fallback := 20
	if len(defaultLimit) > 0 && defaultLimit[0] > 0 {
		fallback = defaultLimit[0]
	}
	limit := req.Limit
	if limit <= 0 {
		limit = req.TopK
	}
	if limit <= 0 {
		limit = fallback
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (req narrativeSearchRequest) normalizedTurnRange() (int, int) {
	fromTurn := req.FromTurn
	toTurn := req.ToTurn
	if fromTurn < 0 {
		fromTurn = 0
	}
	if toTurn < 0 {
		toTurn = 0
	}
	if fromTurn > 0 && toTurn > 0 && fromTurn > toTurn {
		fromTurn, toTurn = toTurn, fromTurn
	}
	return fromTurn, toTurn
}

func decodeNarrativeSearchRequest(w http.ResponseWriter, r *http.Request) (narrativeSearchRequest, bool) {
	req := narrativeSearchRequest{
		ChatSessionID: strings.TrimSpace(r.URL.Query().Get("chat_session_id")),
		Query:         strings.TrimSpace(r.URL.Query().Get("query")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.Limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("top_k")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.TopK = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from_turn")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.FromTurn = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to_turn")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.ToTurn = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("turn_index")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.TurnIndex = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("interval")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.Interval = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("force")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			req.Force = parsed
		}
	}
	if r.Body != nil && r.ContentLength != 0 {
		var body narrativeSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return narrativeSearchRequest{}, false
		}
		if strings.TrimSpace(body.ChatSessionID) != "" {
			req.ChatSessionID = strings.TrimSpace(body.ChatSessionID)
		}
		if strings.TrimSpace(body.Query) != "" {
			req.Query = strings.TrimSpace(body.Query)
		}
		if body.Limit != 0 {
			req.Limit = body.Limit
		}
		if body.TopK != 0 {
			req.TopK = body.TopK
		}
		if body.FromTurn != 0 {
			req.FromTurn = body.FromTurn
		}
		if body.ToTurn != 0 {
			req.ToTurn = body.ToTurn
		}
		if body.TurnIndex != 0 {
			req.TurnIndex = body.TurnIndex
		}
		if body.Interval != 0 {
			req.Interval = body.Interval
		}
		if body.Force {
			req.Force = true
		}
	}
	return req, true
}

func normalizedEpisodeInterval(interval int) int {
	if interval <= 0 {
		interval = 5
	}
	if interval < 5 {
		return 5
	}
	if interval > 60 {
		return 60
	}
	return interval
}

func episodeRangeFromTurn(turnIndex, interval int) (int, int) {
	if turnIndex <= 0 {
		return 0, 0
	}
	if interval <= 0 {
		interval = normalizedEpisodeInterval(interval)
	}
	toTurn := turnIndex
	fromTurn := toTurn - interval + 1
	if fromTurn < 1 {
		fromTurn = 1
	}
	return fromTurn, toTurn
}

func filterChatLogsForTurnRange(logs []store.ChatLog, fromTurn, toTurn, limit int) []store.ChatLog {
	out := []store.ChatLog{}
	for _, item := range logs {
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TurnIndex == out[j].TurnIndex {
			return out[i].ID < out[j].ID
		}
		return out[i].TurnIndex < out[j].TurnIndex
	})
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func buildEpisodeSummaryForRange(sid string, fromTurn, toTurn int, logs []store.ChatLog) (store.EpisodeSummary, map[string]any) {
	return buildEpisodeSummaryForRangeWithArtifacts(sid, fromTurn, toTurn, logs, nil, nil)
}

func buildEpisodeSummaryForRangeWithArtifacts(sid string, fromTurn, toTurn int, logs []store.ChatLog, memories []store.Memory, evidence []store.DirectEvidence) (store.EpisodeSummary, map[string]any) {
	lines := []string{}
	for _, mem := range memories {
		content := cleanEpisodeSourceText(memorySummaryText(mem))
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("memory: %s", truncateRunes(content, 260)))
	}
	for _, ev := range evidence {
		content := cleanEpisodeSourceText(ev.EvidenceText)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("evidence: %s", truncateRunes(content, 220)))
	}
	for _, log := range logs {
		content := cleanEpisodeSourceText(log.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", firstNonEmpty(strings.TrimSpace(log.Role), "unknown"), truncateRunes(content, 220)))
	}
	keyEvents := episodeKeyEvents(lines)
	relationshipChanges := episodeRelationshipAnchors(lines)
	openLoops := episodeOpenLoopAnchors(lines)
	keyEntities := episodeKeyEntities(lines)
	summary := episodeDenseSummary(keyEvents, lines)
	if summary == "" {
		summary = fmt.Sprintf("Episode %d-%d", fromTurn, toTurn)
	}
	item := store.EpisodeSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		SummaryText:             truncateRunes(summary, 700),
		KeyEntities:             mustCompactJSON(keyEntities),
		KeyEvents:               mustCompactJSON(keyEvents),
		OpenLoopsJSON:           mustCompactJSON(openLoops),
		RelationshipChangesJSON: mustCompactJSON(relationshipChanges),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
		CreatedAt:               time.Now().UTC(),
	}
	trace := map[string]any{
		"generation_source":          "deterministic_ds1a_fallback",
		"dense_summary_contract":     "ds1a.v1",
		"input_chat_log_count":       len(logs),
		"input_memory_count":         len(memories),
		"input_evidence_count":       len(evidence),
		"key_event_count":            len(keyEvents),
		"relationship_anchor_count":  len(relationshipChanges),
		"open_loop_anchor_count":     len(openLoops),
		"summary_text_anchor_policy": "memory_evidence_first_then_raw_line",
	}
	return item, trace
}

func episodeDenseSummary(keyEvents []string, lines []string) string {
	source := keyEvents
	if len(source) == 0 {
		source = lines
	}
	parts := []string{}
	for _, line := range source {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		parts = append(parts, truncateRunes(text, 220))
		if len(parts) >= 4 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(parts, " / "))
}

func cleanEpisodeSourceText(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#### Chatindex:") || strings.HasPrefix(line, "Chatindex:") {
			continue
		}
		line = htmlImgTagPattern.ReplaceAllString(line, "")
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func episodeKeyEvents(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		out = append(out, truncateRunes(text, 180))
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeRelationshipAnchors(lines []string) []string {
	keywords := []string{"trust", "trusted", "trusts", "relationship", "bond", "promise", "confess", "confession", "kiss", "love", "betray", "betrayal", "ally", "friend"}
	out := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				out = append(out, truncateRunes(strings.TrimSpace(line), 180))
				break
			}
		}
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeOpenLoopAnchors(lines []string) []string {
	keywords := []string{"unresolved", "remains", "must", "will", "next", "promise", "debt", "mystery", "clue", "sealed", "gate", "return"}
	out := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				out = append(out, truncateRunes(strings.TrimSpace(line), 180))
				break
			}
		}
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeKeyEntities(lines []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, line := range lines {
		for _, token := range strings.Fields(line) {
			token = strings.Trim(token, "[](){}.,:;!?\"'")
			if len(token) < 3 {
				continue
			}
			if isEpisodeMetaEntityToken(token) {
				continue
			}
			r := []rune(token)[0]
			if r < 'A' || r > 'Z' {
				continue
			}
			key := strings.ToLower(token)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, token)
			if len(out) >= 8 {
				return out
			}
		}
	}
	return out
}

func isEpisodeMetaEntityToken(token string) bool {
	normalized := strings.Trim(strings.TrimSpace(token), "[](){}.,:;!?\"'")
	if normalized == "" {
		return true
	}
	lower := strings.ToLower(normalized)
	switch lower {
	case "chatindex", "step", "user", "assistant", "system", "narration", "narrative", "mon", "tue", "wed", "thu", "fri", "sat", "sun", "am", "pm":
		return true
	}
	if len(normalized) <= 5 && strings.ToUpper(normalized) == normalized {
		return true
	}
	if strings.Contains(lower, "chatindex") {
		return true
	}
	return false
}

func normalizedChapterInterval(interval int) int {
	if interval <= 0 {
		interval = 60
	}
	if interval < 10 {
		return 10
	}
	if interval > 200 {
		return 200
	}
	return interval
}

func chapterIntervalCheck(turnIndex, interval int) map[string]any {
	info := map[string]any{
		"checked":   true,
		"triggered": false,
		"range":     nil,
		"reason":    "",
	}
	if turnIndex < interval || turnIndex%interval != 0 {
		info["reason"] = "not_interval_boundary"
		return info
	}
	fromTurn := turnIndex - interval + 1
	info["triggered"] = true
	info["range"] = []int{fromTurn, turnIndex}
	info["reason"] = "interval_boundary"
	return info
}

func turnSpanRecommended(turnSpan any) bool {
	span, ok := turnSpan.(int)
	return ok && span >= 40 && span <= 80
}

const (
	chapterDenseSummaryPolicyVersion  = "ds1b.v1"
	arcDenseSummaryPolicyVersion      = "ds1c.v1"
	denseSummaryPriorityPolicyVersion = "ds1d.v1"
	denseSourceAnchorPolicyVersion    = "ds1f.v1"
	denseRetentionPolicyVersion       = "ds1g.v1"
	denseRoleSplitPolicyVersion       = "ds1h.v1"
	denseEvidencePromotionPolicy      = "ds1i.v1"
)

func denseJSONItems(raw string, limit int) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" || raw == "null" {
		return nil
	}
	var decoded any
	items := []string{}
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		collectDenseJSONStrings(decoded, &items, limit)
	} else {
		items = appendDenseUnique(items, raw, limit)
	}
	return items
}

func collectDenseJSONStrings(value any, out *[]string, limit int) {
	if limit > 0 && len(*out) >= limit {
		return
	}
	switch v := value.(type) {
	case string:
		*out = appendDenseUnique(*out, v, limit)
	case []any:
		for _, item := range v {
			collectDenseJSONStrings(item, out, limit)
			if limit > 0 && len(*out) >= limit {
				return
			}
		}
	case map[string]any:
		preferredKeys := []string{"text", "summary", "event", "fact", "change", "debt", "callback", "turn", "relationship", "world", "value"}
		for _, key := range preferredKeys {
			if item, ok := v[key]; ok {
				collectDenseJSONStrings(item, out, limit)
				if limit > 0 && len(*out) >= limit {
					return
				}
			}
		}
		for _, item := range v {
			collectDenseJSONStrings(item, out, limit)
			if limit > 0 && len(*out) >= limit {
				return
			}
		}
	case float64, bool:
		*out = appendDenseUnique(*out, fmt.Sprint(v), limit)
	}
}

func appendDenseUnique(items []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	value = truncateRunes(value, 240)
	for _, existing := range items {
		if strings.EqualFold(existing, value) {
			return items
		}
	}
	if limit > 0 && len(items) >= limit {
		return items
	}
	return append(items, value)
}

func denseJSONFromItems(items []string, limit int) string {
	out := []string{}
	for _, item := range items {
		out = appendDenseUnique(out, item, limit)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func denseLabeledLines(label string, items []string, limit int) []string {
	out := []string{}
	for _, item := range items {
		out = appendDenseUnique(out, fmt.Sprintf("%s: %s", label, item), limit)
	}
	return out
}

func containsWorldSignal(text string) bool {
	lowered := strings.ToLower(text)
	for _, token := range []string{"world", "rule", "law", "region", "faction", "public", "pressure", "city", "kingdom", "archive", "gate", "tower"} {
		if strings.Contains(lowered, token) {
			return true
		}
	}
	return false
}

func episodeInputPreviews(episodes []store.EpisodeSummary, limit int) []map[string]any {
	if limit <= 0 || limit > len(episodes) {
		limit = len(episodes)
	}
	out := make([]map[string]any, 0, limit)
	for _, ep := range episodes[:limit] {
		preview := ep.SummaryText
		if len(preview) > 160 {
			preview = preview[:160] + "..."
		}
		openLoops := denseJSONItems(ep.OpenLoopsJSON, 4)
		relationshipChanges := denseJSONItems(ep.RelationshipChangesJSON, 4)
		out = append(out, map[string]any{
			"id":                        ep.ID,
			"from_turn":                 ep.FromTurn,
			"to_turn":                   ep.ToTurn,
			"summary_preview":           preview,
			"key_events":                ep.KeyEvents,
			"open_loops_json":           ep.OpenLoopsJSON,
			"relationship_changes_json": ep.RelationshipChangesJSON,
			"dense_anchor_counts": map[string]any{
				"open_loops":           len(openLoops),
				"relationship_changes": len(relationshipChanges),
				"key_events":           len(denseJSONItems(ep.KeyEvents, 4)),
			},
		})
	}
	return out
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func filterEpisodes(episodes []store.EpisodeSummary, query string, fromTurn, toTurn, limit int) []store.EpisodeSummary {
	if limit <= 0 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]store.EpisodeSummary, 0, len(episodes))
	for _, ep := range episodes {
		if fromTurn > 0 && ep.ToTurn > 0 && ep.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && ep.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(ep.SummaryText+" "+ep.KeyEntities+" "+ep.KeyEvents+" "+ep.OpenLoopsJSON+" "+ep.RelationshipChangesJSON), query) {
			continue
		}
		results = append(results, ep)
		if len(results) >= limit {
			break
		}
	}
	return results
}

func episodeResults(episodes []store.EpisodeSummary) []any {
	return episodeResultsWithEvidence(episodes, nil)
}

func episodeResultsWithEvidence(episodes []store.EpisodeSummary, evidence []store.DirectEvidence) []any {
	results := make([]any, 0, len(episodes))
	for _, ep := range episodes {
		denseScores := episodeDensePriorityScores(ep)
		item := map[string]any{
			"id":                           ep.ID,
			"source":                       "episode_summary",
			"chat_session_id":              ep.ChatSessionID,
			"from_turn":                    ep.FromTurn,
			"to_turn":                      ep.ToTurn,
			"summary_text":                 ep.SummaryText,
			"key_entities":                 ep.KeyEntities,
			"key_events":                   ep.KeyEvents,
			"open_loops_json":              ep.OpenLoopsJSON,
			"relationship_changes_json":    ep.RelationshipChangesJSON,
			"embedding_model":              ep.EmbeddingModel,
			"dense_summary_policy_version": denseSummaryPriorityPolicyVersion,
			"dense_priority_score":         denseScores["dense_priority_score"],
			"dense_importance_score":       denseScores["dense_importance_score"],
			"dense_relationship_score":     denseScores["dense_relationship_score"],
			"dense_world_score":            denseScores["dense_world_score"],
		}
		for k, v := range denseSummarySurfaceFields("episode", ep.ID, ep.FromTurn, ep.ToTurn, ep.SummaryText, episodeDenseStructuredPayload(ep), denseScores, evidence) {
			item[k] = v
		}
		results = append(results, item)
	}
	return results
}

func chapterResults(chapters []store.ChapterSummary) []any {
	return chapterResultsWithEvidence(chapters, nil)
}

func chapterResultsWithEvidence(chapters []store.ChapterSummary, evidence []store.DirectEvidence) []any {
	results := make([]any, 0, len(chapters))
	for _, ch := range chapters {
		denseScores := chapterDensePriorityScores(ch)
		item := map[string]any{
			"id":                           ch.ID,
			"source":                       "chapter_summary",
			"source_type":                  "chapter",
			"chat_session_id":              ch.ChatSessionID,
			"from_turn":                    ch.FromTurn,
			"to_turn":                      ch.ToTurn,
			"chapter_index":                ch.ChapterIndex,
			"title":                        ch.ChapterTitle,
			"chapter_title":                ch.ChapterTitle,
			"summary_text":                 ch.SummaryText,
			"resume_text":                  ch.ResumeText,
			"open_loops_json":              ch.OpenLoopsJSON,
			"relationship_changes_json":    ch.RelationshipChangesJSON,
			"world_changes_json":           ch.WorldChangesJSON,
			"callback_candidates_json":     ch.CallbackCandidatesJSON,
			"embedding_model":              ch.EmbeddingModel,
			"dense_summary_policy_version": denseSummaryPriorityPolicyVersion,
			"dense_priority_score":         denseScores["dense_priority_score"],
			"dense_importance_score":       denseScores["dense_importance_score"],
			"dense_relationship_score":     denseScores["dense_relationship_score"],
			"dense_world_score":            denseScores["dense_world_score"],
		}
		for k, v := range denseSummarySurfaceFields("chapter", ch.ID, ch.FromTurn, ch.ToTurn, q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText, ch.ChapterTitle), chapterDenseStructuredPayload(ch), denseScores, evidence) {
			item[k] = v
		}
		results = append(results, item)
	}
	return results
}

func denseSummarySurfaceFields(recordType string, id int64, fromTurn, toTurn int, narrativeText string, structuredPayload map[string]any, denseScores map[string]int, evidence []store.DirectEvidence) map[string]any {
	if structuredPayload == nil {
		structuredPayload = map[string]any{}
	}
	fields := denseSourceAnchorFields(recordType, id, fromTurn, toTurn)
	fields["dense_role_split_policy_version"] = denseRoleSplitPolicyVersion
	fields["dense_narrative_text"] = strings.TrimSpace(narrativeText)
	fields["dense_narrative_usage"] = "read_only"
	fields["dense_structured_payload"] = structuredPayload
	fields["dense_structured_usage"] = "adjudication_retrieval"

	relationshipScore := denseScoreValue(denseScores, "dense_relationship_score")
	worldScore := denseScoreValue(denseScores, "dense_world_score")
	importanceScore := denseScoreValue(denseScores, "dense_importance_score")
	structuredCount := denseStructuredPayloadCount(structuredPayload)
	retentionApplied := relationshipScore > 0 || worldScore > 0 || importanceScore >= 2 || structuredCount >= 2
	retentionReason := "standard_dense_priority"
	if retentionApplied {
		retentionReason = "important_fact_retention"
	}
	fields["dense_retention_policy_version"] = denseRetentionPolicyVersion
	fields["dense_retention_applied"] = retentionApplied
	fields["dense_retention_reason"] = retentionReason
	fields["dense_retention_signal_count"] = structuredCount

	for k, v := range denseEvidencePromotionFields(evidence, fromTurn, toTurn, structuredPayload) {
		fields[k] = v
	}
	return fields
}

func denseSourceAnchorFields(recordType string, id int64, fromTurn, toTurn int) map[string]any {
	return map[string]any{
		"dense_source_anchor_policy_version": denseSourceAnchorPolicyVersion,
		"source_record_id":                   id,
		"source_record_type":                 recordType,
		"source_turn_range": map[string]any{
			"from_turn": fromTurn,
			"to_turn":   toTurn,
		},
	}
}

func episodeDenseStructuredPayload(ep store.EpisodeSummary) map[string]any {
	return map[string]any{
		"key_events":           denseJSONItems(ep.KeyEvents, 8),
		"open_loops":           denseJSONItems(ep.OpenLoopsJSON, 8),
		"relationship_changes": denseJSONItems(ep.RelationshipChangesJSON, 8),
	}
}

func chapterDenseStructuredPayload(ch store.ChapterSummary) map[string]any {
	return map[string]any{
		"open_loops":           denseJSONItems(ch.OpenLoopsJSON, 8),
		"relationship_changes": denseJSONItems(ch.RelationshipChangesJSON, 8),
		"world_changes":        denseJSONItems(ch.WorldChangesJSON, 8),
		"callback_candidates":  denseJSONItems(ch.CallbackCandidatesJSON, 8),
	}
}

func arcDenseStructuredPayload(arc store.ArcSummary) map[string]any {
	return map[string]any{
		"key_turning_points":       denseJSONItems(arc.KeyTurningPointsJSON, 8),
		"active_promises":          denseJSONItems(arc.ActivePromisesJSON, 8),
		"unresolved_debts":         denseJSONItems(arc.UnresolvedDebtsJSON, 8),
		"resolved_payoffs":         denseJSONItems(arc.ResolvedPayoffsJSON, 8),
		"callback_candidates":      denseJSONItems(arc.CallbackCandidatesJSON, 8),
		"future_payoff_candidates": denseJSONItems(arc.FuturePayoffCandidatesJSON, 8),
		"irreversible_turns":       denseJSONItems(arc.IrreversibleTurnsJSON, 8),
		"callback_debts":           denseJSONItems(arc.CallbackDebtsJSON, 8),
		"relationship_pivots":      denseJSONItems(arc.RelationshipPivotsJSON, 8),
	}
}

func sagaDenseStructuredPayload(saga store.SagaDigest) map[string]any {
	return map[string]any{
		"persistent_facts":      denseJSONItems(saga.PersistentFactsJSON, 8),
		"never_drop_candidates": denseJSONItems(saga.NeverDropCandidatesJSON, 8),
	}
}

func denseScoreValue(scores map[string]int, key string) int {
	if scores == nil {
		return 0
	}
	return scores[key]
}

func denseStructuredPayloadCount(payload map[string]any) int {
	count := 0
	for _, value := range payload {
		switch v := value.(type) {
		case []string:
			count += len(v)
		case []any:
			count += len(v)
		case string:
			if strings.TrimSpace(v) != "" {
				count++
			}
		}
	}
	return count
}

func denseEvidencePromotionFields(evidence []store.DirectEvidence, fromTurn, toTurn int, structuredPayload map[string]any) map[string]any {
	relationshipCount := 0
	worldCount := 0
	promiseCount := 0
	for _, ev := range evidence {
		if !denseEvidenceOverlaps(ev, fromTurn, toTurn) {
			continue
		}
		text := strings.ToLower(strings.TrimSpace(ev.EvidenceKind + " " + ev.EvidenceText + " " + ev.LineageJSON))
		if denseTextHasAny(text, []string{"relationship", "trust", "ally", "friend", "bond", "betray", "love", "rival"}) {
			relationshipCount++
		}
		if denseTextHasAny(text, []string{"world", "rule", "law", "faction", "city", "kingdom", "gate", "region", "pressure"}) {
			worldCount++
		}
		if denseTextHasAny(text, []string{"promise", "vow", "oath", "callback", "debt", "repay", "payoff"}) {
			promiseCount++
		}
	}
	structuredText := strings.ToLower(fmt.Sprint(structuredPayload))
	if relationshipCount == 0 && denseTextHasAny(structuredText, []string{"relationship", "trust", "ally", "bond", "pivot"}) {
		relationshipCount = 1
	}
	if worldCount == 0 && denseTextHasAny(structuredText, []string{"world", "rule", "law", "faction", "city", "gate"}) {
		worldCount = 1
	}
	if promiseCount == 0 && denseTextHasAny(structuredText, []string{"promise", "callback", "debt", "payoff"}) {
		promiseCount = 1
	}
	score := relationshipCount + worldCount + promiseCount
	return map[string]any{
		"dense_direct_evidence_promotion_policy_version":    denseEvidencePromotionPolicy,
		"dense_direct_evidence_promotion_score":             score,
		"dense_direct_evidence_promoted_relationship_count": relationshipCount,
		"dense_direct_evidence_promoted_world_count":        worldCount,
		"dense_direct_evidence_promoted_promise_count":      promiseCount,
		"dense_structured_precedence_applied":               score > 0,
	}
}

func denseEvidenceOverlaps(ev store.DirectEvidence, fromTurn, toTurn int) bool {
	start := ev.SourceTurnStart
	end := ev.SourceTurnEnd
	if start <= 0 {
		start = ev.TurnAnchor
	}
	if end <= 0 {
		end = start
	}
	if fromTurn <= 0 && toTurn <= 0 {
		return true
	}
	if toTurn <= 0 {
		toTurn = fromTurn
	}
	if fromTurn <= 0 {
		fromTurn = toTurn
	}
	return start <= toTurn && end >= fromTurn
}

func denseTextHasAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func chapterIndexForRange(toTurn, interval int) int {
	if interval <= 0 {
		interval = 60
	}
	if toTurn <= 0 {
		return 1
	}
	idx := toTurn / interval
	if toTurn%interval != 0 {
		idx++
	}
	if idx <= 0 {
		return 1
	}
	return idx
}

func hierarchyIndexForRange(toTurn, interval int) int {
	return chapterIndexForRange(toTurn, interval)
}

func deterministicChapterSummaryForRange(sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary) store.ChapterSummary {
	return store.ChapterSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		ChapterIndex:            chapterIndex,
		ChapterTitle:            fmt.Sprintf("Chapter %d", chapterIndex),
		SummaryText:             deterministicChapterSummaryText(episodes),
		OpenLoopsJSON:           deterministicChapterOpenLoopsJSON(episodes),
		RelationshipChangesJSON: deterministicChapterRelationshipChangesJSON(episodes),
		WorldChangesJSON:        deterministicChapterWorldChangesJSON(episodes),
		CallbackCandidatesJSON:  deterministicChapterCallbacksJSON(episodes),
		ResumeText:              deterministicChapterResumeText(episodes),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) buildChapterSummaryForRange(ctx context.Context, sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary) (store.ChapterSummary, map[string]any) {
	deterministic := deterministicChapterSummaryForRange(sid, fromTurn, toTurn, chapterIndex, episodes)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
	}

	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["chapter_shadow_compare"] = chapterShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}

	trace["llm_attempted"] = true
	llmChapter, llmTrace, err := s.callChapterSummaryLLM(ctx, sid, fromTurn, toTurn, chapterIndex, episodes, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["chapter_shadow_compare"] = chapterShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["chapter_shadow_compare"] = chapterShadowCompare(llmChapter, deterministic, true, "configured_llm")
	return llmChapter, trace
}

func (s *Server) callChapterSummaryLLM(ctx context.Context, sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary, cfg completeTurnLLMConfig) (store.ChapterSummary, map[string]any, error) {
	systemPrompt := "You generate Archive Center chapter summaries. Return only a compact JSON object with chapter_title, summary_text, open_loops, relationship_changes, world_changes, callback_candidates, and resume_text. Prefer structured episode anchors before prose summary_text in this order: open_loops, relationship_changes, world_changes, callback_candidates, resume_text, summary_text. Keep facts grounded in the provided episode summaries."
	episodePayload := episodeInputPreviews(episodes, 12)
	payload := map[string]any{
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"chapter_index":   chapterIndex,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"episodes": episodePayload,
	}
	payloadBytes, _ := json.Marshal(payload)
	userPrompt := "Create one chapter summary JSON for this range:\n" + string(payloadBytes)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1400
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return store.ChapterSummary{}, map[string]any{
			"configured":    true,
			"endpoint_host": endpointHost(cfg.Endpoint),
			"model":         cfg.Model,
		}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return store.ChapterSummary{}, map[string]any{
			"configured":    true,
			"endpoint_host": endpointHost(cfg.Endpoint),
			"model":         cfg.Model,
			"raw_preview":   truncateRunes(content, 1000),
		}, err
	}
	chapter := chapterSummaryFromLLMJSON(sid, fromTurn, toTurn, chapterIndex, parsed, episodes)
	trace := map[string]any{
		"configured":    true,
		"endpoint_host": endpointHost(cfg.Endpoint),
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
	}
	return chapter, trace, nil
}

func chapterSummaryFromLLMJSON(sid string, fromTurn, toTurn, chapterIndex int, parsed map[string]any, episodes []store.EpisodeSummary) store.ChapterSummary {
	fallback := deterministicChapterSummaryForRange(sid, fromTurn, toTurn, chapterIndex, episodes)
	title := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["chapter_title"]),
		extractionStringFromAny(parsed["title"]),
		fallback.ChapterTitle,
	)
	summary := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["summary_text"]),
		extractionStringFromAny(parsed["summary"]),
		fallback.SummaryText,
	)
	resume := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["resume_text"]),
		extractionStringFromAny(parsed["resume"]),
		fallback.ResumeText,
	)
	return store.ChapterSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		ChapterIndex:            chapterIndex,
		ChapterTitle:            title,
		SummaryText:             summary,
		OpenLoopsJSON:           compactChapterJSONField(parsed, fallback.OpenLoopsJSON, "open_loops", "openLoops"),
		RelationshipChangesJSON: compactChapterJSONField(parsed, fallback.RelationshipChangesJSON, "relationship_changes", "relationshipChanges"),
		WorldChangesJSON:        compactChapterJSONField(parsed, fallback.WorldChangesJSON, "world_changes", "worldChanges"),
		CallbackCandidatesJSON:  compactChapterJSONField(parsed, fallback.CallbackCandidatesJSON, "callback_candidates", "callbackCandidates"),
		ResumeText:              resume,
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func compactChapterJSONField(parsed map[string]any, fallback string, keys ...string) string {
	for _, key := range keys {
		value, ok := parsed[key]
		if !ok || value == nil {
			continue
		}
		if raw := strings.TrimSpace(extractionStringFromAny(value)); raw != "" {
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
				if data, err := json.Marshal(decoded); err == nil {
					return string(data)
				}
			}
			if data, err := json.Marshal([]string{raw}); err == nil {
				return string(data)
			}
		}
		if data, err := json.Marshal(value); err == nil {
			return string(data)
		}
	}
	if strings.TrimSpace(fallback) == "" {
		return "[]"
	}
	return fallback
}

func chapterShadowCompare(selected store.ChapterSummary, deterministic store.ChapterSummary, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                     true,
		"selected_source":             source,
		"llm_attempted":               llmAttempted,
		"deterministic_summary_chars": utf8.RuneCountInString(deterministic.SummaryText),
		"selected_summary_chars":      utf8.RuneCountInString(selected.SummaryText),
		"deterministic_resume_chars":  utf8.RuneCountInString(deterministic.ResumeText),
		"selected_resume_chars":       utf8.RuneCountInString(selected.ResumeText),
		"summary_diverged":            selected.SummaryText != deterministic.SummaryText,
		"resume_diverged":             selected.ResumeText != deterministic.ResumeText,
	}
}

func deterministicArcSummaryForRange(sid string, fromTurn, toTurn, arcIndex int, chapters []store.ChapterSummary) store.ArcSummary {
	summaryParts := []string{}
	turningPoints := []string{}
	activePromises := []string{}
	unresolvedDebts := []string{}
	callbacks := []string{}
	futurePayoffs := []string{}
	irreversibleTurns := []string{}
	callbackDebts := []string{}
	relationshipPivots := []string{}
	for _, ch := range chapters {
		openLoops := denseJSONItems(ch.OpenLoopsJSON, 8)
		relationshipChanges := denseJSONItems(ch.RelationshipChangesJSON, 8)
		worldChanges := denseJSONItems(ch.WorldChangesJSON, 8)
		chapterCallbacks := denseJSONItems(ch.CallbackCandidatesJSON, 8)
		summaryParts = append(summaryParts, chapterDensePriorityLines(ch, 8)...)
		turningPoints = append(turningPoints, worldChanges...)
		turningPoints = append(turningPoints, denseJSONItems(ch.ResumeText, 3)...)
		activePromises = append(activePromises, relationshipChanges...)
		unresolvedDebts = append(unresolvedDebts, openLoops...)
		unresolvedDebts = append(unresolvedDebts, chapterCallbacks...)
		callbacks = append(callbacks, chapterCallbacks...)
		callbacks = append(callbacks, openLoops...)
		futurePayoffs = append(futurePayoffs, openLoops...)
		futurePayoffs = append(futurePayoffs, chapterCallbacks...)
		irreversibleTurns = append(irreversibleTurns, worldChanges...)
		if strings.TrimSpace(ch.ResumeText) != "" {
			irreversibleTurns = append(irreversibleTurns, truncateRunes(ch.ResumeText, 180))
		}
		callbackDebts = append(callbackDebts, openLoops...)
		callbackDebts = append(callbackDebts, chapterCallbacks...)
		relationshipPivots = append(relationshipPivots, relationshipChanges...)
	}
	core := strings.Join(summaryParts, " ")
	if core == "" {
		core = "Arc summary pending richer chapter material."
	}
	return store.ArcSummary{
		ChatSessionID:              sid,
		FromTurn:                   fromTurn,
		ToTurn:                     toTurn,
		ArcIndex:                   arcIndex,
		ArcName:                    fmt.Sprintf("Arc %d", arcIndex),
		ArcStatus:                  "active",
		CoreConflict:               truncateRunes(core, 600),
		KeyTurningPointsJSON:       denseJSONFromItems(turningPoints, 12),
		ActivePromisesJSON:         denseJSONFromItems(activePromises, 12),
		UnresolvedDebtsJSON:        denseJSONFromItems(unresolvedDebts, 12),
		ResolvedPayoffsJSON:        "[]",
		CallbackCandidatesJSON:     denseJSONFromItems(callbacks, 12),
		FuturePayoffCandidatesJSON: denseJSONFromItems(futurePayoffs, 12),
		IrreversibleTurnsJSON:      denseJSONFromItems(irreversibleTurns, 12),
		CallbackDebtsJSON:          denseJSONFromItems(callbackDebts, 12),
		RelationshipPivotsJSON:     denseJSONFromItems(relationshipPivots, 12),
		ArcResumeText:              fmt.Sprintf("Turns %d-%d: %s", fromTurn, toTurn, truncateRunes(core, 420)),
		EmbeddingVector:            "[]",
		EmbeddingModel:             "none",
	}
}

func (s *Server) buildArcSummaryForRange(ctx context.Context, sid string, fromTurn, toTurn, arcIndex int, chapters []store.ChapterSummary) (store.ArcSummary, map[string]any) {
	deterministic := deterministicArcSummaryForRange(sid, fromTurn, toTurn, arcIndex, chapters)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
		"status_reason":     "deterministic_default_active",
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"arc_dense_summary_policy_version":               arcDenseSummaryPolicyVersion,
	}
	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["shadow_compare"] = arcShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}
	trace["llm_attempted"] = true
	parsed, llmTrace, err := s.callHierarchySummaryLLM(ctx, "arc", sid, fromTurn, toTurn, chapters, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["shadow_compare"] = arcShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	arc := arcSummaryFromLLMJSON(sid, fromTurn, toTurn, arcIndex, parsed, chapters)
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["status_reason"] = "configured_llm_normalized"
	trace["shadow_compare"] = arcShadowCompare(arc, deterministic, true, "configured_llm")
	return arc, trace
}

func arcSummaryFromLLMJSON(sid string, fromTurn, toTurn, arcIndex int, parsed map[string]any, chapters []store.ChapterSummary) store.ArcSummary {
	fallback := deterministicArcSummaryForRange(sid, fromTurn, toTurn, arcIndex, chapters)
	status := strings.ToLower(extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_status"]), fallback.ArcStatus))
	if status != "active" && status != "paused" && status != "resolved" {
		status = "active"
	}
	if status == "resolved" {
		if compactChapterJSONField(parsed, "[]", "active_promises", "activePromises") != "[]" ||
			compactChapterJSONField(parsed, "[]", "unresolved_debts", "unresolvedDebts") != "[]" {
			status = "active"
		}
	}
	return store.ArcSummary{
		ChatSessionID:              sid,
		FromTurn:                   fromTurn,
		ToTurn:                     toTurn,
		ArcIndex:                   arcIndex,
		ArcName:                    extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_name"]), extractionStringFromAny(parsed["name"]), fallback.ArcName),
		ArcStatus:                  status,
		CoreConflict:               extractionFirstNonEmpty(extractionStringFromAny(parsed["core_conflict"]), fallback.CoreConflict),
		KeyTurningPointsJSON:       compactChapterJSONField(parsed, fallback.KeyTurningPointsJSON, "key_turning_points", "keyTurningPoints"),
		ActivePromisesJSON:         compactChapterJSONField(parsed, fallback.ActivePromisesJSON, "active_promises", "activePromises"),
		UnresolvedDebtsJSON:        compactChapterJSONField(parsed, fallback.UnresolvedDebtsJSON, "unresolved_debts", "unresolvedDebts"),
		ResolvedPayoffsJSON:        compactChapterJSONField(parsed, fallback.ResolvedPayoffsJSON, "resolved_payoffs", "resolvedPayoffs"),
		CallbackCandidatesJSON:     compactChapterJSONField(parsed, fallback.CallbackCandidatesJSON, "callback_candidates", "callbackCandidates"),
		FuturePayoffCandidatesJSON: compactChapterJSONField(parsed, fallback.FuturePayoffCandidatesJSON, "future_payoff_candidates", "futurePayoffCandidates"),
		IrreversibleTurnsJSON:      compactChapterJSONField(parsed, fallback.IrreversibleTurnsJSON, "irreversible_turns", "irreversibleTurns"),
		CallbackDebtsJSON:          compactChapterJSONField(parsed, fallback.CallbackDebtsJSON, "callback_debts", "callbackDebts"),
		RelationshipPivotsJSON:     compactChapterJSONField(parsed, fallback.RelationshipPivotsJSON, "relationship_pivots", "relationshipPivots"),
		ArcResumeText:              extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_resume_text"]), extractionStringFromAny(parsed["resume_text"]), fallback.ArcResumeText),
		EmbeddingVector:            "[]",
		EmbeddingModel:             "none",
	}
}

func deterministicSagaDigestForRange(sid string, fromTurn, toTurn int, arcs []store.ArcSummary) store.SagaDigest {
	parts := []string{}
	neverDrop := []string{}
	for _, arc := range arcs {
		parts = append(parts, arcDensePriorityLines(arc, 12)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.CallbackDebtsJSON, 6)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.CallbackCandidatesJSON, 6)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.RelationshipPivotsJSON, 6)...)
	}
	summary := strings.Join(parts, " ")
	if summary == "" {
		summary = "Saga digest pending richer arc material."
	}
	return store.SagaDigest{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		EraLabel:                fmt.Sprintf("Era %d-%d", fromTurn, toTurn),
		SagaSummary:             truncateRunes(summary, 900),
		PersistentFactsJSON:     "[]",
		NeverDropCandidatesJSON: denseJSONFromItems(neverDrop, 18),
		ResumePackText:          fmt.Sprintf("Turns %d-%d: %s", fromTurn, toTurn, truncateRunes(summary, 520)),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) buildSagaDigestForRange(ctx context.Context, sid string, fromTurn, toTurn int, arcs []store.ArcSummary) (store.SagaDigest, map[string]any) {
	deterministic := deterministicSagaDigestForRange(sid, fromTurn, toTurn, arcs)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
	}
	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["shadow_compare"] = sagaShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}
	trace["llm_attempted"] = true
	parsed, llmTrace, err := s.callHierarchySummaryLLM(ctx, "saga", sid, fromTurn, toTurn, arcs, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["shadow_compare"] = sagaShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	saga := sagaDigestFromLLMJSON(sid, fromTurn, toTurn, parsed, arcs)
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["shadow_compare"] = sagaShadowCompare(saga, deterministic, true, "configured_llm")
	return saga, trace
}

func sagaDigestFromLLMJSON(sid string, fromTurn, toTurn int, parsed map[string]any, arcs []store.ArcSummary) store.SagaDigest {
	fallback := deterministicSagaDigestForRange(sid, fromTurn, toTurn, arcs)
	return store.SagaDigest{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		EraLabel:                extractionFirstNonEmpty(extractionStringFromAny(parsed["era_label"]), extractionStringFromAny(parsed["label"]), fallback.EraLabel),
		SagaSummary:             extractionFirstNonEmpty(extractionStringFromAny(parsed["saga_summary"]), extractionStringFromAny(parsed["summary"]), fallback.SagaSummary),
		PersistentFactsJSON:     compactChapterJSONField(parsed, fallback.PersistentFactsJSON, "persistent_facts", "persistentFacts"),
		NeverDropCandidatesJSON: compactChapterJSONField(parsed, fallback.NeverDropCandidatesJSON, "never_drop_candidates", "neverDropCandidates"),
		ResumePackText:          extractionFirstNonEmpty(extractionStringFromAny(parsed["resume_pack_text"]), extractionStringFromAny(parsed["resume_text"]), fallback.ResumePackText),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) callHierarchySummaryLLM(ctx context.Context, kind string, sid string, fromTurn, toTurn int, inputs any, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	systemPrompt := "You generate Archive Center " + kind + " summaries. Return only compact JSON. Ground the result in the provided hierarchy inputs and do not invent unrelated facts."
	switch kind {
	case "arc":
		systemPrompt = "You generate Archive Center arc summaries. Return only compact JSON. Preserve chapter dense anchors before prose in this order: open_loops, relationship_changes, world_changes, callback_candidates, resume_text, summary_text. Include irreversible_turns, callback_debts, and relationship_pivots when supported by the input."
	case "saga":
		systemPrompt = "You generate Archive Center saga digests. Return only compact JSON. Preserve arc dense anchors before prose in this order: irreversible_turns, callback_debts, relationship_pivots, promises, debts, callbacks, resume_text, core_conflict."
	}
	payload := map[string]any{
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"inputs":          inputs,
	}
	if kind == "arc" {
		payload["chapter_dense_summary_injection_policy_version"] = chapterDenseSummaryPolicyVersion
		payload["arc_dense_summary_policy_version"] = arcDenseSummaryPolicyVersion
	}
	if kind == "saga" {
		payload["arc_dense_summary_policy_version"] = arcDenseSummaryPolicyVersion
	}
	payloadBytes, _ := json.Marshal(payload)
	userPrompt := "Create one " + kind + " summary JSON for this range:\n" + string(payloadBytes)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1400
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return nil, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": cfg.Model}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return nil, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": cfg.Model, "raw_preview": truncateRunes(content, 1000)}, err
	}
	return parsed, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model), "usage": upstream["usage"]}, nil
}

func arcShadowCompare(selected store.ArcSummary, deterministic store.ArcSummary, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                    true,
		"selected_source":            source,
		"llm_attempted":              llmAttempted,
		"deterministic_resume_chars": utf8.RuneCountInString(deterministic.ArcResumeText),
		"selected_resume_chars":      utf8.RuneCountInString(selected.ArcResumeText),
		"resume_diverged":            selected.ArcResumeText != deterministic.ArcResumeText,
		"status_diverged":            selected.ArcStatus != deterministic.ArcStatus,
	}
}

func sagaShadowCompare(selected store.SagaDigest, deterministic store.SagaDigest, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                    true,
		"selected_source":            source,
		"llm_attempted":              llmAttempted,
		"deterministic_resume_chars": utf8.RuneCountInString(deterministic.ResumePackText),
		"selected_resume_chars":      utf8.RuneCountInString(selected.ResumePackText),
		"resume_diverged":            selected.ResumePackText != deterministic.ResumePackText,
	}
}

func deterministicChapterSummaryText(episodes []store.EpisodeSummary) string {
	parts := make([]string, 0, len(episodes))
	for _, ep := range episodes {
		parts = append(parts, episodeDensePriorityLines(ep, 5)...)
	}
	if len(parts) == 0 {
		return "Chapter summary pending richer episode material."
	}
	return strings.Join(parts, " ")
}

func deterministicChapterResumeText(episodes []store.EpisodeSummary) string {
	if len(episodes) == 0 {
		return ""
	}
	first := episodes[0]
	last := episodes[len(episodes)-1]
	summary := deterministicChapterSummaryText(episodes)
	return fmt.Sprintf("Turns %d-%d: %s", first.FromTurn, last.ToTurn, truncateRunes(summary, 360))
}

func deterministicChapterCallbacksJSON(episodes []store.EpisodeSummary) string {
	callbacks := []string{}
	for _, ep := range episodes {
		callbacks = append(callbacks, denseJSONItems(ep.OpenLoopsJSON, 4)...)
		callbacks = append(callbacks, denseJSONItems(ep.KeyEvents, 2)...)
		callbacks = append(callbacks, denseJSONItems(ep.KeyEntities, 2)...)
		if len(callbacks) >= 8 {
			break
		}
	}
	return denseJSONFromItems(callbacks, 8)
}

func deterministicChapterOpenLoopsJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		items = append(items, denseJSONItems(ep.OpenLoopsJSON, 5)...)
	}
	return denseJSONFromItems(items, 12)
}

func deterministicChapterRelationshipChangesJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		items = append(items, denseJSONItems(ep.RelationshipChangesJSON, 5)...)
	}
	return denseJSONFromItems(items, 12)
}

func deterministicChapterWorldChangesJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		for _, item := range denseJSONItems(ep.KeyEvents, 5) {
			if containsWorldSignal(item) {
				items = appendDenseUnique(items, item, 12)
			}
		}
		for _, item := range denseJSONItems(ep.OpenLoopsJSON, 5) {
			if containsWorldSignal(item) {
				items = appendDenseUnique(items, item, 12)
			}
		}
	}
	return denseJSONFromItems(items, 12)
}

func episodeDensePriorityLines(ep store.EpisodeSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("open_loop", denseJSONItems(ep.OpenLoopsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship", denseJSONItems(ep.RelationshipChangesJSON, 4), limit)...)
	for _, event := range denseJSONItems(ep.KeyEvents, 4) {
		label := "callback"
		if containsWorldSignal(event) {
			label = "world"
		}
		lines = appendDenseUnique(lines, fmt.Sprintf("%s: %s", label, event), limit)
	}
	if len(lines) < limit {
		for _, item := range denseJSONItems(ep.SummaryText, 2) {
			lines = appendDenseUnique(lines, "summary: "+item, limit)
		}
	}
	return lines
}

func chapterDensePriorityLines(ch store.ChapterSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("open_loop", denseJSONItems(ch.OpenLoopsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship", denseJSONItems(ch.RelationshipChangesJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("world", denseJSONItems(ch.WorldChangesJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("callback", denseJSONItems(ch.CallbackCandidatesJSON, 4), limit)...)
	if len(lines) < limit && strings.TrimSpace(ch.ResumeText) != "" {
		lines = appendDenseUnique(lines, "resume: "+truncateRunes(ch.ResumeText, 180), limit)
	}
	if len(lines) < limit && strings.TrimSpace(ch.SummaryText) != "" {
		lines = appendDenseUnique(lines, "summary: "+truncateRunes(ch.SummaryText, 180), limit)
	}
	return lines
}

func arcDensePriorityLines(arc store.ArcSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("irreversible", denseJSONItems(arc.IrreversibleTurnsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("callback_debt", denseJSONItems(arc.CallbackDebtsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship_pivot", denseJSONItems(arc.RelationshipPivotsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("promise", denseJSONItems(arc.ActivePromisesJSON, 3), limit)...)
	lines = append(lines, denseLabeledLines("debt", denseJSONItems(arc.UnresolvedDebtsJSON, 3), limit)...)
	lines = append(lines, denseLabeledLines("callback", denseJSONItems(arc.CallbackCandidatesJSON, 3), limit)...)
	if len(lines) < limit && strings.TrimSpace(arc.ArcResumeText) != "" {
		lines = appendDenseUnique(lines, "resume: "+truncateRunes(arc.ArcResumeText, 220), limit)
	}
	if len(lines) < limit && strings.TrimSpace(arc.CoreConflict) != "" {
		lines = appendDenseUnique(lines, "core: "+truncateRunes(arc.CoreConflict, 220), limit)
	}
	return lines
}

func chapterDenseInputStats(episodes []store.EpisodeSummary) map[string]any {
	openLoops, relationshipChanges, worldChanges, callbacks := 0, 0, 0, 0
	for _, ep := range episodes {
		openLoops += len(denseJSONItems(ep.OpenLoopsJSON, 100))
		relationshipChanges += len(denseJSONItems(ep.RelationshipChangesJSON, 100))
		for _, item := range denseJSONItems(ep.KeyEvents, 100) {
			if containsWorldSignal(item) {
				worldChanges++
			}
			callbacks++
		}
	}
	return map[string]any{
		"episode_count": len(episodes),
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"anchor_priority":                   []string{"open_loops", "relationship_changes", "world_changes", "callback_candidates", "resume_text", "summary_text"},
		"episode_open_loop_anchor_count":    openLoops,
		"episode_relationship_anchor_count": relationshipChanges,
		"episode_world_anchor_count":        worldChanges,
		"episode_callback_anchor_count":     callbacks,
	}
}

func arcDenseInputStats(chapters []store.ChapterSummary, fromTurn, toTurn int) map[string]any {
	openLoops, relationshipChanges, worldChanges, callbacks := 0, 0, 0, 0
	for _, ch := range chapters {
		openLoops += len(denseJSONItems(ch.OpenLoopsJSON, 100))
		relationshipChanges += len(denseJSONItems(ch.RelationshipChangesJSON, 100))
		worldChanges += len(denseJSONItems(ch.WorldChangesJSON, 100))
		callbacks += len(denseJSONItems(ch.CallbackCandidatesJSON, 100))
	}
	return map[string]any{
		"chapter_count":             len(chapters),
		"chapter_count_recommended": len(chapters) >= 3 && len(chapters) <= 6,
		"turn_span":                 (toTurn - fromTurn) + 1,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"arc_dense_summary_policy_version":               arcDenseSummaryPolicyVersion,
		"semantic_field_mapping": map[string]any{
			"irreversible_turns_json":  "world_changes_json + resume_text anchors",
			"callback_debts_json":      "open_loops_json + callback_candidates_json",
			"relationship_pivots_json": "relationship_changes_json",
		},
		"chapter_open_loop_anchor_count":    openLoops,
		"chapter_relationship_anchor_count": relationshipChanges,
		"chapter_world_anchor_count":        worldChanges,
		"chapter_callback_anchor_count":     callbacks,
	}
}

func sagaDenseInputStats(arcs []store.ArcSummary, fromTurn, toTurn int) map[string]any {
	irreversible, debts, pivots := 0, 0, 0
	for _, arc := range arcs {
		irreversible += len(denseJSONItems(arc.IrreversibleTurnsJSON, 100))
		debts += len(denseJSONItems(arc.CallbackDebtsJSON, 100))
		pivots += len(denseJSONItems(arc.RelationshipPivotsJSON, 100))
	}
	return map[string]any{
		"arc_count":                        len(arcs),
		"arc_count_recommended":            len(arcs) >= 2 && len(arcs) <= 6,
		"turn_span":                        (toTurn - fromTurn) + 1,
		"arc_dense_summary_policy_version": arcDenseSummaryPolicyVersion,
		"arc_irreversible_anchor_count":    irreversible,
		"arc_callback_debt_count":          debts,
		"arc_relationship_pivot_count":     pivots,
		"saga_input_priority":              []string{"irreversible_turns", "callback_debts", "relationship_pivots", "promises", "debts", "callbacks", "resume_text", "core_conflict"},
	}
}

func chapterDensePriorityScores(ch store.ChapterSummary) map[string]int {
	relationshipScore := len(denseJSONItems(ch.RelationshipChangesJSON, 100))
	worldScore := len(denseJSONItems(ch.WorldChangesJSON, 100))
	importanceScore := len(denseJSONItems(ch.OpenLoopsJSON, 100)) + len(denseJSONItems(ch.CallbackCandidatesJSON, 100))
	if strings.TrimSpace(ch.ResumeText) != "" {
		importanceScore++
	}
	priorityScore := relationshipScore*4 + worldScore*4 + importanceScore*2
	return map[string]int{
		"dense_priority_score":     priorityScore,
		"dense_importance_score":   importanceScore,
		"dense_relationship_score": relationshipScore,
		"dense_world_score":        worldScore,
	}
}

func episodeDensePriorityScores(ep store.EpisodeSummary) map[string]int {
	relationshipScore := len(denseJSONItems(ep.RelationshipChangesJSON, 100))
	worldScore := 0
	for _, item := range denseJSONItems(ep.KeyEvents, 100) {
		if containsWorldSignal(item) {
			worldScore++
		}
	}
	importanceScore := len(denseJSONItems(ep.OpenLoopsJSON, 100)) + len(denseJSONItems(ep.KeyEvents, 100))
	if strings.TrimSpace(ep.SummaryText) != "" {
		importanceScore++
	}
	priorityScore := relationshipScore*4 + worldScore*4 + importanceScore*2
	return map[string]int{
		"dense_priority_score":     priorityScore,
		"dense_importance_score":   importanceScore,
		"dense_relationship_score": relationshipScore,
		"dense_world_score":        worldScore,
	}
}

func sortChapterSummariesByDensePriority(chapters []store.ChapterSummary) {
	sort.SliceStable(chapters, func(i, j int) bool {
		left := chapterDensePriorityScores(chapters[i])["dense_priority_score"]
		right := chapterDensePriorityScores(chapters[j])["dense_priority_score"]
		if left != right {
			return left > right
		}
		if chapters[i].ToTurn != chapters[j].ToTurn {
			return chapters[i].ToTurn > chapters[j].ToTurn
		}
		return chapters[i].ID > chapters[j].ID
	})
}

func sortEpisodeSummariesByDensePriority(episodes []store.EpisodeSummary) {
	sort.SliceStable(episodes, func(i, j int) bool {
		left := episodeDensePriorityScores(episodes[i])["dense_priority_score"]
		right := episodeDensePriorityScores(episodes[j])["dense_priority_score"]
		if left != right {
			return left > right
		}
		if episodes[i].ToTurn != episodes[j].ToTurn {
			return episodes[i].ToTurn > episodes[j].ToTurn
		}
		return episodes[i].ID > episodes[j].ID
	})
}

func denseSearchStoreLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	widened := limit * 4
	if widened < limit+8 {
		widened = limit + 8
	}
	if widened > 100 {
		return 100
	}
	return widened
}

func matchesChapter(ch *store.ChapterSummary, query string) bool {
	if ch == nil {
		return false
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	text := strings.ToLower(ch.ChapterTitle + " " + ch.ResumeText + " " + ch.SummaryText)
	return strings.Contains(text, query)
}

func (s *Server) handleMetricsLC1D(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)
	replay := buildLC1DIntegrityReplay(ev)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"chat_session_id":  chatSessionID,
		"integrity_replay": replay,
	})
}

func buildLC1DIntegrityReplay(ev narrativeEvidence) map[string]any {
	latest := latestNarrativeEvidenceTurn(ev)
	latestAny := any(nil)
	if latest > 0 {
		latestAny = latest
	}
	longThreshold := 300
	ultraThreshold := 600
	scopeCounts := map[string]any{"long": 0, "ultra_long": 0}
	retainedByLayer := map[string]any{
		"canonical":       0,
		"dense_summary":   0,
		"direct_evidence": 0,
		"live_ledger":     0,
		"memory":          0,
	}
	candidatesTotal := 0
	retainedTotal := 0
	examples := []any{}
	addCandidate := func(layer string, turn int, label string, score float64) {
		if latest <= 0 || turn <= 0 || latest-turn < longThreshold {
			return
		}
		candidatesTotal++
		retainedTotal++
		scopeCounts["long"] = intFromAny(scopeCounts["long"], 0) + 1
		if latest-turn >= ultraThreshold {
			scopeCounts["ultra_long"] = intFromAny(scopeCounts["ultra_long"], 0) + 1
		}
		retainedByLayer[layer] = intFromAny(retainedByLayer[layer], 0) + 1
		if len(examples) < 5 {
			examples = append(examples, map[string]any{
				"layer":        layer,
				"turn_index":   turn,
				"age_turns":    latest - turn,
				"label":        truncateRunes(label, 120),
				"retain_score": score,
			})
		}
	}

	for _, item := range ev.Memories {
		score := lc1dMemoryRetainScore(item)
		if score >= 0.7 {
			addCandidate("memory", item.TurnIndex, lc1dMemoryLabel(item), score)
		}
	}
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			addCandidate("direct_evidence", lc1dEvidenceTurn(item), item.EvidenceText, 1.0)
		}
	}
	for _, item := range ev.CanonicalStateLayers {
		turn := item.SourceTurn
		if turn <= 0 {
			turn = item.TurnIndex
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["canonical"] = intFromAny(retainedByLayer["canonical"], 0) + 1
		}
	}
	for _, item := range ev.EpisodeSummaries {
		if latest > 0 && item.ToTurn > 0 && latest-item.ToTurn >= longThreshold {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
	}
	if ev.ResumePack != nil {
		if ev.ResumePack.Chapter != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
		if ev.ResumePack.Arc != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
		if ev.ResumePack.Saga != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
	}
	for _, item := range ev.Storylines {
		turn := item.LastEvidenceTurn
		if turn <= 0 {
			turn = item.LastTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["live_ledger"] = intFromAny(retainedByLayer["live_ledger"], 0) + 1
		}
	}
	for _, item := range ev.PendingThreads {
		turn := item.SourceTurn
		if turn <= 0 {
			turn = item.CreatedTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["live_ledger"] = intFromAny(retainedByLayer["live_ledger"], 0) + 1
		}
	}
	retentionRate := 0.0
	if candidatesTotal > 0 {
		retentionRate = float64(retainedTotal) / float64(candidatesTotal)
	}
	return map[string]any{
		"policy_version":               "lc1d.v1",
		"status":                       "ok",
		"replay_query":                 "",
		"replay_query_source":          "query_independent_store_replay",
		"long_turn_threshold":          longThreshold,
		"ultra_long_turn_threshold":    ultraThreshold,
		"replay_non_similarity_max":    0.2,
		"latest_turn_index":            latestAny,
		"scope_counts":                 scopeCounts,
		"retained_by_layer":            retainedByLayer,
		"retained_total":               retainedTotal,
		"gaps_total":                   candidatesTotal - retainedTotal,
		"retention_rate":               retentionRate,
		"candidate_examples":           examples,
		"candidates_total":             candidatesTotal,
		"scanned_direct_evidence_rows": len(ev.Evidence),
	}
}

func latestNarrativeEvidenceTurn(ev narrativeEvidence) int {
	latest := 0
	for _, item := range ev.ChatLogs {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	for _, item := range ev.Memories {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	for _, item := range ev.Evidence {
		for _, turn := range []int{item.TurnAnchor, item.SourceTurnEnd, item.SourceTurnStart} {
			if turn > latest {
				latest = turn
			}
		}
	}
	return latest
}

func lc1dMemoryRetainScore(item store.Memory) float64 {
	return math.Max(item.Importance, math.Max(item.NarrativeSignificance, item.EmotionalIntensity))
}

func lc1dMemoryLabel(item store.Memory) string {
	for _, value := range []string{item.SummaryJSON, item.Evidence, item.PlaceRoom, item.PlaceWing} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return fmt.Sprintf("memory:%d", item.ID)
}

func lc1dEvidenceTurn(item store.DirectEvidence) int {
	for _, turn := range []int{item.TurnAnchor, item.SourceTurnEnd, item.SourceTurnStart} {
		if turn > 0 {
			return turn
		}
	}
	return 0
}

func lc1dEvidenceRetained(item store.DirectEvidence) bool {
	if item.Tombstoned || item.RepairNeeded {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(item.ArchiveState))
	verification := strings.ToLower(strings.TrimSpace(item.CaptureVerification))
	return state == "verified_direct" || state == "previous_archive" || verification == "verified"
}

func (s *Server) handleMetricsLC1E(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	budgetCompare := buildLC1EBudgetCompare(ev)
	counts := map[string]any{"evidence_count": len(ev.Evidence), "kg_triple_count": len(ev.KGTriples), "memory_count": len(ev.Memories)}
	trace := []string{}
	if len(ev.Evidence) > 0 {
		trace = append(trace, "evidence")
	}
	if len(ev.KGTriples) > 0 {
		trace = append(trace, "kg_triples")
	}
	if intFromAny(budgetCompare["hypamemory_always_on_chars"], 0) > 0 || intFromAny(budgetCompare["archive_center_layered_chars"], 0) > 0 {
		trace = append(trace, "budget_compare")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		budgetCompare = map[string]any{"policy_version": "lc1e.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"note":            "metrics lc1e is Store-backed HypaMemory always-on budget compare evidence",
		"source":          source,
		"budget_compare":  budgetCompare,
		"counts":          counts,
		"trace_summary":   trace,
		"store_status":    storeStatus,
	})
}

func buildLC1EBudgetCompare(ev narrativeEvidence) map[string]any {
	hypaAlwaysOnChars := 0
	for _, item := range ev.Memories {
		hypaAlwaysOnChars += len([]rune(item.SummaryJSON))
		hypaAlwaysOnChars += len([]rune(item.Evidence))
	}
	for _, item := range ev.Evidence {
		if strings.Contains(strings.ToLower(item.LineageJSON), "hypamemory") || strings.Contains(strings.ToLower(item.CaptureStage), "hypamemory") {
			hypaAlwaysOnChars += len([]rune(item.EvidenceText))
		}
	}

	canonicalChars := 0
	for _, item := range ev.CanonicalStateLayers {
		canonicalChars += len([]rune(item.Content))
	}
	denseChars := 0
	for _, item := range ev.EpisodeSummaries {
		denseChars += len([]rune(item.SummaryText))
	}
	if ev.ResumePack != nil {
		if ev.ResumePack.Chapter != nil {
			denseChars += len([]rune(ev.ResumePack.Chapter.SummaryText)) + len([]rune(ev.ResumePack.Chapter.ResumeText))
		}
		if ev.ResumePack.Arc != nil {
			denseChars += len([]rune(ev.ResumePack.Arc.CoreConflict)) + len([]rune(ev.ResumePack.Arc.ArcResumeText))
		}
		if ev.ResumePack.Saga != nil {
			denseChars += len([]rune(ev.ResumePack.Saga.SagaSummary)) + len([]rune(ev.ResumePack.Saga.ResumePackText))
		}
	}
	lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
	storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	stateStatus := "skeleton"
	if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
		stateStatus = "heuristic"
	}
	liveLedgerChars := pythonDefaultJSONRuneLen(buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn))
	layeredChars := canonicalChars + denseChars + liveLedgerChars
	savedChars := hypaAlwaysOnChars - layeredChars
	savingsRatio := 0.0
	if hypaAlwaysOnChars > 0 {
		savingsRatio = float64(savedChars) / float64(hypaAlwaysOnChars)
	}
	recommendedMode := "archive_center_layered"
	if hypaAlwaysOnChars > 0 && layeredChars > hypaAlwaysOnChars {
		recommendedMode = "investigate_layered_overhead"
	}
	return map[string]any{
		"policy_version":                     "lc1e.v1",
		"status":                             "ok",
		"hypamemory_always_on_mode":          "discouraged_after_import",
		"hypamemory_always_on_chars":         hypaAlwaysOnChars,
		"archive_center_layered_chars":       layeredChars,
		"archive_center_canonical_chars":     canonicalChars,
		"archive_center_dense_summary_chars": denseChars,
		"archive_center_live_ledger_chars":   liveLedgerChars,
		"saved_chars_vs_hypamemory":          savedChars,
		"savings_ratio":                      savingsRatio,
		"recommended_mode":                   recommendedMode,
	}
}

func (s *Server) handleMetricsLC1F(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	regressionConfirm := buildLC1FRegressionConfirm(ev)
	counts := map[string]any{"storyline_count": len(ev.Storylines), "episode_summary_count": len(ev.EpisodeSummaries)}
	trace := []string{}
	if len(ev.Storylines) > 0 {
		trace = append(trace, "storylines")
	}
	if len(ev.EpisodeSummaries) > 0 {
		trace = append(trace, "episode_summaries")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		regressionConfirm = map[string]any{"policy_version": "lc1f.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"chat_session_id":    chatSessionID,
		"note":               "metrics lc1f is Store-backed short/mid non-regression evidence",
		"source":             source,
		"counts":             counts,
		"regression_confirm": regressionConfirm,
		"trace_summary":      trace,
		"store_status":       storeStatus,
	})
}

func buildLC1FRegressionConfirm(ev narrativeEvidence) map[string]any {
	hasVerifiedEvidence := false
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			hasVerifiedEvidence = true
			break
		}
	}
	shortChecks := map[string]bool{
		"chat_logs_present":       len(ev.ChatLogs) > 0,
		"direct_evidence_present": hasVerifiedEvidence,
		"kg_present":              len(ev.KGTriples) > 0,
		"current_state_present":   len(ev.ActiveStates) > 0 || len(ev.CanonicalStateLayers) > 0 || len(ev.CharacterStates) > 0,
	}
	midChecks := map[string]bool{
		"storyline_present":       len(ev.Storylines) > 0,
		"episode_summary_present": len(ev.EpisodeSummaries) > 0,
		"world_rule_present":      len(ev.WorldRules) > 0,
		"pending_thread_present":  len(ev.PendingThreads) > 0,
		"resume_pack_present":     ev.ResumePack != nil,
	}
	authorityChecks := map[string]bool{
		"canonical_state_available": len(ev.CanonicalStateLayers) > 0,
		"direct_evidence_available": hasVerifiedEvidence,
		"retrieval_support_only":    true,
	}
	failed := []string{}
	for key, ok := range shortChecks {
		if !ok {
			failed = append(failed, "short."+key)
		}
	}
	for key, ok := range midChecks {
		if !ok {
			failed = append(failed, "mid."+key)
		}
	}
	for key, ok := range authorityChecks {
		if !ok {
			failed = append(failed, "authority."+key)
		}
	}
	status := "pass"
	if len(failed) > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":   "lc1f.v1",
		"status":           status,
		"short_term":       shortChecks,
		"mid_term":         midChecks,
		"authority_checks": authorityChecks,
		"failed_checks":    failed,
		"checked_layers": []string{
			"chat_logs",
			"direct_evidence",
			"kg_triples",
			"active_states",
			"canonical_state_layers",
			"character_states",
			"storylines",
			"episode_summaries",
			"world_rules",
			"pending_threads",
			"resume_pack",
		},
	}
}

func (s *Server) handleMetricsLC1G(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	promotionReplay := buildLC1GPromotionReplay(ev)
	counts := map[string]any{"world_rule_count": len(ev.WorldRules), "canonical_state_layer_count": len(ev.CanonicalStateLayers)}
	trace := []string{}
	if len(ev.WorldRules) > 0 {
		trace = append(trace, "world_rules")
	}
	if len(ev.CanonicalStateLayers) > 0 {
		trace = append(trace, "canonical_state_layers")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		promotionReplay = map[string]any{"policy_version": "lc1g.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"chat_session_id":  chatSessionID,
		"note":             "metrics lc1g is Store-backed stale/current promotion replay evidence",
		"source":           source,
		"counts":           counts,
		"promotion_replay": promotionReplay,
		"trace_summary":    trace,
		"store_status":     storeStatus,
	})
}

func buildLC1GPromotionReplay(ev narrativeEvidence) map[string]any {
	latest := latestNarrativeEvidenceTurn(ev)
	staleStorylines := 0
	for _, item := range ev.Storylines {
		turn := item.LastEvidenceTurn
		if turn <= 0 {
			turn = item.LastTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= 300 && strings.EqualFold(item.Status, "active") {
			staleStorylines++
		}
	}
	verifiedPromotions := 0
	lowConfidencePromotions := 0
	for _, item := range ev.CanonicalStateLayers {
		if item.Confidence >= 0.7 {
			verifiedPromotions++
		} else {
			lowConfidencePromotions++
		}
	}
	status := "pass"
	if lowConfidencePromotions > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":                 "lc1g.v1",
		"status":                         status,
		"latest_turn_index":              latest,
		"stale_storyline_candidates":     staleStorylines,
		"current_active_state_count":     len(ev.ActiveStates),
		"current_canonical_count":        len(ev.CanonicalStateLayers),
		"verified_promotion_count":       verifiedPromotions,
		"low_confidence_promotion_count": lowConfidencePromotions,
		"promotion_gate":                 "verified_confidence_required",
	}
}

func (s *Server) handleMetricsLC1H(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	errorReplay := buildLC1HFalseNegativePositiveReplay(ev)
	counts := map[string]any{"character_state_count": len(ev.CharacterStates), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.CharacterStates) > 0 {
		trace = append(trace, "character_states")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		errorReplay = map[string]any{"policy_version": "lc1h.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                         "ok",
		"chat_session_id":                chatSessionID,
		"note":                           "metrics lc1h is Store-backed false negative/false positive replay evidence",
		"source":                         source,
		"false_negative_positive_replay": errorReplay,
		"counts":                         counts,
		"trace_summary":                  trace,
		"store_status":                   storeStatus,
	})
}

func buildLC1HFalseNegativePositiveReplay(ev narrativeEvidence) map[string]any {
	verifiedEvidence := 0
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			verifiedEvidence++
		}
	}
	lowConfidenceCanonical := 0
	for _, item := range ev.CanonicalStateLayers {
		if item.Confidence > 0 && item.Confidence < 0.7 {
			lowConfidenceCanonical++
		}
	}
	falseNegativeRisk := 0
	if verifiedEvidence > 0 && len(ev.CanonicalStateLayers) == 0 && len(ev.ActiveStates) == 0 {
		falseNegativeRisk = verifiedEvidence
	}
	falsePositiveRisk := lowConfidenceCanonical
	status := "pass"
	if falseNegativeRisk > 0 || falsePositiveRisk > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":            "lc1h.v1",
		"status":                    status,
		"verified_evidence_count":   verifiedEvidence,
		"current_state_count":       len(ev.ActiveStates),
		"canonical_state_count":     len(ev.CanonicalStateLayers),
		"false_negative_risk_count": falseNegativeRisk,
		"false_positive_risk_count": falsePositiveRisk,
		"repair_action":             "keep_current_state_and_direct_evidence_visible",
	}
}

func (s *Server) handleMetricsLC1I(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	ablationCompare := buildLC1IRecallAblationCompare(ev)
	counts := map[string]any{"active_state_count": len(ev.ActiveStates), "pending_thread_count": len(ev.PendingThreads)}
	trace := []string{}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		ablationCompare = map[string]any{"policy_version": "lc1i.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                  "ok",
		"chat_session_id":         chatSessionID,
		"note":                    "metrics lc1i is Store-backed relationship/ledger/world-pressure ablation evidence",
		"source":                  source,
		"recall_ablation_compare": ablationCompare,
		"counts":                  counts,
		"trace_summary":           trace,
		"store_status":            storeStatus,
	})
}

func buildLC1IRecallAblationCompare(ev narrativeEvidence) map[string]any {
	relationshipSignals := 0
	for _, item := range ev.CharacterStates {
		if strings.TrimSpace(item.RelationshipsJSON) != "" {
			relationshipSignals++
		}
	}
	for _, item := range ev.CanonicalStateLayers {
		if item.LayerType == "relationship_state" {
			relationshipSignals++
		}
	}
	ledgerSignals := len(ev.Storylines) + len(ev.PendingThreads)
	worldPressureSignals := len(ev.WorldRules) + len(ev.ActiveStates)
	fullRecall := relationshipSignals + ledgerSignals + worldPressureSignals
	return map[string]any{
		"policy_version":                       "lc1i.v1",
		"status":                               "ok",
		"relationship_v2_signal_count":         relationshipSignals,
		"ledger_signal_count":                  ledgerSignals,
		"world_pressure_signal_count":          worldPressureSignals,
		"full_recall_signal_count":             fullRecall,
		"without_relationship_v2_signal_count": fullRecall - relationshipSignals,
		"without_ledger_signal_count":          fullRecall - ledgerSignals,
		"without_world_pressure_signal_count":  fullRecall - worldPressureSignals,
	}
}

func (s *Server) handleMetricsLC1J(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	verificationGate := buildLC1JVerificationGate(ev)
	resumePresent := 0
	if ev.ResumePack != nil {
		resumePresent = 1
	}
	counts := map[string]any{"chat_log_count": len(ev.ChatLogs), "resume_pack_present": resumePresent}
	trace := []string{}
	if len(ev.ChatLogs) > 0 {
		trace = append(trace, "chat_logs")
	}
	if resumePresent > 0 {
		trace = append(trace, "resume_pack")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		verificationGate = map[string]any{"policy_version": "lc1j.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"chat_session_id":   chatSessionID,
		"note":              "metrics lc1j is Store-backed verification gate evidence",
		"source":            source,
		"verification_gate": verificationGate,
		"counts":            counts,
		"trace_summary":     trace,
		"store_status":      storeStatus,
	})
}

func buildLC1JVerificationGate(ev narrativeEvidence) map[string]any {
	hasChat := len(ev.ChatLogs) > 0
	hasResume := ev.ResumePack != nil
	hasCanonical := len(ev.CanonicalStateLayers) > 0
	hasDirect := false
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			hasDirect = true
			break
		}
	}
	passed := hasChat && hasResume && hasCanonical && hasDirect
	status := "pass"
	if !passed {
		status = "warn"
	}
	return map[string]any{
		"policy_version":           "lc1j.v1",
		"status":                   status,
		"chat_log_gate":            hasChat,
		"resume_pack_gate":         hasResume,
		"canonical_state_gate":     hasCanonical,
		"direct_evidence_gate":     hasDirect,
		"release_gate_ready":       passed,
		"default_runtime_takeover": false,
	}
}

func (s *Server) handleMetricsLC1K(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	priorityBudget := buildLC1KPriorityBudgetTrace(ev)
	counts := map[string]any{"memory_count": len(ev.Memories), "kg_triple_count": len(ev.KGTriples)}
	trace := []string{}
	if len(ev.Memories) > 0 {
		trace = append(trace, "memories")
	}
	if len(ev.KGTriples) > 0 {
		trace = append(trace, "kg_triples")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		priorityBudget = map[string]any{"policy_version": "lc1k.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"chat_session_id":       chatSessionID,
		"note":                  "metrics lc1k is Store-backed priority/budget hybrid evidence",
		"source":                source,
		"priority_budget_trace": priorityBudget,
		"counts":                counts,
		"trace_summary":         trace,
		"store_status":          storeStatus,
	})
}

func buildLC1KPriorityBudgetTrace(ev narrativeEvidence) map[string]any {
	highPriority := len(ev.CanonicalStateLayers) + len(ev.Evidence)
	lowerTierSupport := len(ev.Memories) + len(ev.KGTriples) + len(ev.EpisodeSummaries)
	return map[string]any{
		"policy_version":               "lc1k.v1",
		"status":                       "ok",
		"priority_order":               []string{"direct_evidence", "canonical_state", "dense_summary", "memory", "kg"},
		"high_priority_layer_count":    highPriority,
		"lower_tier_support_count":     lowerTierSupport,
		"lower_tier_support_preserved": lowerTierSupport > 0,
		"budget_gate":                  "authority_first_then_support",
	}
}

func (s *Server) handleMetricsLC1L(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	importedIdeaGate := buildLC1LImportedIdeaContractGate(ev)
	counts := map[string]any{"world_rule_count": len(ev.WorldRules), "evidence_count": len(ev.Evidence)}
	trace := []string{}
	if len(ev.WorldRules) > 0 {
		trace = append(trace, "world_rules")
	}
	if len(ev.Evidence) > 0 {
		trace = append(trace, "evidence")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		importedIdeaGate = map[string]any{"policy_version": "lc1l.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                      "ok",
		"chat_session_id":             chatSessionID,
		"note":                        "metrics lc1l is Store-backed imported-idea contract gate evidence",
		"source":                      source,
		"imported_idea_contract_gate": importedIdeaGate,
		"counts":                      counts,
		"trace_summary":               trace,
		"store_status":                storeStatus,
	})
}

func buildLC1LImportedIdeaContractGate(ev narrativeEvidence) map[string]any {
	importedSignals := 0
	for _, item := range ev.Memories {
		if strings.Contains(strings.ToLower(item.PlaceWing+" "+item.PlaceRoom+" "+item.SummaryJSON), "hypa") {
			importedSignals++
		}
	}
	for _, item := range ev.Evidence {
		if strings.Contains(strings.ToLower(item.LineageJSON+" "+item.CaptureStage), "hypa") {
			importedSignals++
		}
	}
	defaultTakeoverBlocked := true
	return map[string]any{
		"policy_version":               "lc1l.v1",
		"status":                       "pass",
		"imported_signal_count":        importedSignals,
		"default_takeover_blocked":     defaultTakeoverBlocked,
		"requires_contract_before_use": true,
		"allowed_destination_layers":   []string{"memory", "direct_evidence", "kg", "audit"},
		"blocked_destination_layers":   []string{"current_truth_without_verification", "canonical_without_gate"},
	}
}

func (s *Server) handleMetricsLC1M(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	splitCompare := buildLC1MSplitPipelineCompare(ev)
	counts := map[string]any{"episode_summary_count": len(ev.EpisodeSummaries), "chat_log_count": len(ev.ChatLogs)}
	trace := []string{}
	if len(ev.EpisodeSummaries) > 0 {
		trace = append(trace, "episode_summaries")
	}
	if len(ev.ChatLogs) > 0 {
		trace = append(trace, "chat_logs")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		splitCompare = map[string]any{"policy_version": "lc1m.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        chatSessionID,
		"note":                   "metrics lc1m is Store-backed split-pipeline compare evidence",
		"source":                 source,
		"split_pipeline_compare": splitCompare,
		"counts":                 counts,
		"trace_summary":          trace,
		"store_status":           storeStatus,
	})
}

func buildLC1MSplitPipelineCompare(ev narrativeEvidence) map[string]any {
	traceCount := countAuditEvents(ev.AuditLogs, "critic_pipeline_trace")
	errorCount := 0
	for _, item := range ev.AuditLogs {
		if strings.Contains(strings.ToLower(item.DetailsJSON), `"status":"error"`) || strings.Contains(strings.ToLower(item.Summary), "error") {
			errorCount++
		}
	}
	status := "pass"
	if errorCount > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":              "lc1m.v1",
		"status":                      status,
		"split_pipeline_enabled":      true,
		"single_call_mode":            false,
		"critic_pipeline_trace_count": traceCount,
		"pipeline_error_count":        errorCount,
		"extractor_stage":             "evidence_extractor",
		"reducer_stage":               "deterministic_reducer",
		"compactor_stage":             "summary_compactor_background",
	}
}

func (s *Server) handleMetricsLC1N(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	backfillReplay := buildLC1NBackfillReplay(ev)
	counts := map[string]any{"pending_thread_count": len(ev.PendingThreads), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		backfillReplay = map[string]any{"policy_version": "lc1n.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                  "ok",
		"chat_session_id":         chatSessionID,
		"note":                    "metrics lc1n is Store-backed rebuild/backfill rehearsal evidence",
		"source":                  source,
		"rebuild_backfill_replay": backfillReplay,
		"counts":                  counts,
		"trace_summary":           trace,
		"store_status":            storeStatus,
	})
}

func buildLC1NBackfillReplay(ev narrativeEvidence) map[string]any {
	inputRows := len(ev.ChatLogs)
	derivedRows := len(ev.Evidence) + len(ev.CanonicalStateLayers) + len(ev.EpisodeSummaries) + len(ev.Storylines) + len(ev.PendingThreads)
	status := "pass"
	if inputRows > 0 && derivedRows == 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":       "lc1n.v1",
		"status":               status,
		"chat_log_rows":        len(ev.ChatLogs),
		"direct_evidence_rows": len(ev.Evidence),
		"canonical_rows":       len(ev.CanonicalStateLayers),
		"dense_summary_rows":   len(ev.EpisodeSummaries),
		"ledger_rows":          len(ev.Storylines) + len(ev.PendingThreads),
		"drift_detected":       status != "pass",
		"rebuild_mode":         "store_backed_read_rehearsal",
	}
}

func (s *Server) handleMetricsLC1O(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	previewLedger := buildLC1ODeterministicPreviewLedger(ev)
	counts := map[string]any{"canonical_state_layer_count": len(ev.CanonicalStateLayers), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.CanonicalStateLayers) > 0 {
		trace = append(trace, "canonical_state_layers")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		previewLedger = map[string]any{"policy_version": "lc1o.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                       "ok",
		"chat_session_id":              chatSessionID,
		"note":                         "metrics lc1o is deterministic no-LLM preview/ledger evidence",
		"source":                       source,
		"deterministic_preview_ledger": previewLedger,
		"counts":                       counts,
		"trace_summary":                trace,
		"store_status":                 storeStatus,
	})
}

func buildLC1ODeterministicPreviewLedger(ev narrativeEvidence) map[string]any {
	start := time.Now()
	lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
	storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	stateStatus := "skeleton"
	if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
		stateStatus = "heuristic"
	}
	ledger := buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn)
	elapsedMs := time.Since(start).Milliseconds()
	return map[string]any{
		"policy_version":        "lc1o.v1",
		"status":                "pass",
		"llm_call_required":     false,
		"preview_path":          "deterministic",
		"ledger_policy_version": ledger["ledger_policy_version"],
		"world_pressure_ready":  ledger["world_pressure"] != nil,
		"latency_ms":            elapsedMs,
		"storyline_count":       len(ev.Storylines),
		"pending_thread_count":  len(ev.PendingThreads),
		"active_state_count":    len(ev.ActiveStates),
	}
}

func (s *Server) handleMetricsLC1P(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	counts := map[string]any{"storyline_count": len(ev.Storylines), "pending_thread_count": len(ev.PendingThreads)}
	trace := []string{}
	if len(ev.Storylines) > 0 {
		trace = append(trace, "storylines")
	}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
	}
	evaluationSplit := buildLC1PEvaluationSplit(ev)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        chatSessionID,
		"note":                   "metrics lc1p is Store-backed R1 read evidence",
		"source":                 source,
		"counts":                 counts,
		"trace_summary":          trace,
		"store_status":           storeStatus,
		"evaluation_split":       evaluationSplit,
		"retrieval_completeness": evaluationSplit["retrieval_completeness"],
		"final_answer_quality":   evaluationSplit["final_answer_quality"],
		"failure_split":          evaluationSplit["failure_split"],
	})
}

func buildLC1PEvaluationSplit(ev narrativeEvidence) map[string]any {
	retrievalSignals := []bool{
		len(ev.Memories) > 0,
		len(ev.Evidence) > 0,
		len(ev.KGTriples) > 0,
		len(ev.EpisodeSummaries) > 0,
		len(ev.ActiveStates) > 0,
	}
	answerSignals := []bool{
		len(ev.Storylines) > 0,
		len(ev.PendingThreads) > 0,
		len(ev.WorldRules) > 0,
		len(ev.CharacterStates) > 0,
		len(ev.CanonicalStateLayers) > 0,
	}
	retrievalScore := ratioOfTrue(retrievalSignals)
	answerQuality := ratioOfTrue(answerSignals)
	classification := "healthy"
	if retrievalScore < 0.5 && answerQuality < 0.5 {
		classification = "mixed_failure"
	} else if retrievalScore < 0.5 {
		classification = "retrieval_failure_dominant"
	} else if answerQuality < 0.5 {
		classification = "reader_failure_dominant"
	}
	if ev.Disabled {
		classification = "store_disabled"
	}
	return map[string]any{
		"policy_version": "lc1p.v1",
		"status":         map[bool]string{true: "disabled", false: "pass"}[ev.Disabled],
		"retrieval_completeness": map[string]any{
			"policy_version":        "s17-1a.v1",
			"metric_defined":        true,
			"score":                 retrievalScore,
			"memory_count":          len(ev.Memories),
			"direct_evidence_count": len(ev.Evidence),
			"kg_triple_count":       len(ev.KGTriples),
			"episode_summary_count": len(ev.EpisodeSummaries),
			"active_state_count":    len(ev.ActiveStates),
		},
		"final_answer_quality": map[string]any{
			"policy_version":              "s17-1b.v1",
			"metric_defined":              true,
			"score":                       answerQuality,
			"storyline_count":             len(ev.Storylines),
			"pending_thread_count":        len(ev.PendingThreads),
			"world_rule_count":            len(ev.WorldRules),
			"character_state_count":       len(ev.CharacterStates),
			"canonical_state_layer_count": len(ev.CanonicalStateLayers),
		},
		"failure_split": map[string]any{
			"policy_version":     "s17-1c.v1",
			"replay_defined":     true,
			"classification":     classification,
			"retrieval_failure":  retrievalScore < 0.5,
			"reader_failure":     answerQuality < 0.5,
			"truth_authority":    false,
			"inspection_surface": true,
		},
	}
}

func ratioOfTrue(values []bool) float64 {
	if len(values) == 0 {
		return 0
	}
	hits := 0
	for _, value := range values {
		if value {
			hits++
		}
	}
	return float64(hits) / float64(len(values))
}

func (s *Server) handleMetricsLC1Q(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"freshness_lag_summary": map[string]any{
			"policy_version": "lc1q.v1",
			"status":         "off",
			"classification": "insufficient_signal",
			"metric_defined": true,
			"timestamps": map[string]any{
				"latest_chat_log_created_at":                 nil,
				"latest_complete_turn_visibility_created_at": nil,
				"latest_critic_pipeline_trace_created_at":    nil,
				"latest_canonical_state_layer_created_at":    nil,
			},
			"lags_seconds": map[string]any{
				"save_delay":               nil,
				"extraction_delay":         nil,
				"promotion_visibility_lag": nil,
			},
			"signal_coverage": map[string]any{
				"chat_logs":                false,
				"complete_turn_visibility": false,
				"critic_pipeline_trace":    false,
				"canonical_state_layers":   false,
			},
			"answer_quality_split": map[string]any{
				"extraction_delay_affects_answer_quality":         true,
				"save_delay_affects_answer_quality":               true,
				"promotion_visibility_lag_affects_answer_quality": true,
			},
			"warnings": []any{},
		},
	})
}

func (s *Server) handleMetricsLC1R(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                     "ok",
		"regression_corpus_manifest": regressionCorpusManifest(),
	})
}

func (s *Server) handleMetricsLC1S(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"step17_bundle_closure": step17BundleClosure(),
	})
}

func (s *Server) handleMetricsTM1D(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"truth_maintenance_audit_replay": map[string]any{
			"policy_version": "tm1d.v1",
			"status":         "off",
			"gate_result":    "off",
			"reason":         "audit_surface_missing",
			"audit_event_counts": map[string]any{
				"importance_reevaluation": countAuditEvents(ev.AuditLogs, "importance_reevaluation"),
				"drift_detected":          countAuditEvents(ev.AuditLogs, "drift_detected"),
			},
		},
	})
}

// Audit / feedback / import

func (s *Server) handleAuditGet(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			writeBadRequest(w, "limit must be a non-negative integer")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}
	chatSessionID := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))

	items, err := s.Store.ListAuditLogs(r.Context(), chatSessionID, eventType, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.AuditLog{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	// Convert to snake_case maps to match Python 0.8 response shape
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":              item.ID,
			"created_at":      formatKSTTime(item.CreatedAt),
			"event_type":      item.EventType,
			"chat_session_id": item.ChatSessionID,
			"target_type":     nullableString(item.TargetType),
			"target_id":       nullableInt64(item.TargetID),
			"summary":         item.Summary,
			"details_json":    item.DetailsJSON,
			"source":          item.Source,
		})
	}
	total := len(out)
	if counter, ok := s.Store.(store.AuditLogCounter); ok {
		if found, err := counter.CountAuditLogs(r.Context(), chatSessionID, eventType); err == nil {
			total = found
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  out,
		"limit":  limit,
		"offset": 0,
		"total":  total,
	})
}

func (s *Server) saveAuditLogBestEffort(ctx context.Context, audit *store.AuditLog) {
	if s == nil || s.Store == nil || audit == nil {
		return
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now().UTC()
	}
	_ = s.Store.SaveAuditLog(ctx, audit)
}

func (s *Server) handleFeedbackPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatSessionID string `json:"chat_session_id"`
		TargetType    string `json:"target_type"`
		TargetID      int64  `json:"target_id"`
		FeedbackValue string `json:"feedback_value"`
		FeedbackNote  string `json:"feedback_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	req.ChatSessionID = strings.TrimSpace(req.ChatSessionID)
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.FeedbackValue = strings.TrimSpace(strings.ToLower(req.FeedbackValue))
	req.FeedbackNote = strings.TrimSpace(req.FeedbackNote)
	if req.ChatSessionID == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.TargetID <= 0 {
		writeBadRequest(w, "target_id must be a positive integer")
		return
	}
	if req.TargetType != "memory" && req.TargetType != "kg_triple" {
		writeBadRequest(w, "target_type must be memory or kg_triple")
		return
	}
	if req.FeedbackValue != "up" && req.FeedbackValue != "down" {
		writeBadRequest(w, "feedback_value must be up or down")
		return
	}
	if ok, err := s.feedbackTargetBelongsToSession(r.Context(), req.ChatSessionID, req.TargetType, req.TargetID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, CodeShadowGuard, "POST /feedback requires canonical store reads")
			return
		}
		writeInternalError(w, err.Error())
		return
	} else if !ok {
		writeBadRequest(w, fmt.Sprintf("%s #%d not found for chat_session_id %s", req.TargetType, req.TargetID, req.ChatSessionID))
		return
	}

	now := time.Now().UTC()
	feedback := &store.CriticFeedback{
		ChatSessionID: req.ChatSessionID,
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		FeedbackValue: req.FeedbackValue,
		FeedbackNote:  req.FeedbackNote,
		Source:        "manual_ui",
		CreatedAt:     now,
	}
	if err := s.Store.SaveCriticFeedback(r.Context(), feedback); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, CodeShadowGuard, "POST /feedback requires canonical store writes")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	if items, err := s.Store.ListCriticFeedback(r.Context(), req.ChatSessionID, req.TargetType, req.TargetID); err == nil && len(items) > 0 {
		feedback.ID = items[0].ID
		feedback.CreatedAt = items[0].CreatedAt
	}
	audit := &store.AuditLog{
		ChatSessionID: req.ChatSessionID,
		EventType:     "critic_feedback",
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		Summary:       fmt.Sprintf("Feedback %s on %s #%d", req.FeedbackValue, req.TargetType, req.TargetID),
		DetailsJSON:   mustCompactJSON(map[string]any{"feedback_value": req.FeedbackValue, "feedback_note": req.FeedbackNote}),
		Source:        "manual_ui",
		CreatedAt:     now,
	}
	_ = s.Store.SaveAuditLog(r.Context(), audit)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"ok":             true,
		"detail":         fmt.Sprintf("%s #%d feedback saved", req.TargetType, req.TargetID),
		"feedback_id":    feedback.ID,
		"feedback_value": feedback.FeedbackValue,
	})
}

func (s *Server) handleFeedbackLatest(w http.ResponseWriter, r *http.Request) {
	chatSessionID := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	targetType := strings.TrimSpace(r.URL.Query().Get("target_type"))
	targetID := int64(0)
	if rawTargetID := strings.TrimSpace(r.URL.Query().Get("target_id")); rawTargetID != "" {
		parsed, err := strconv.ParseInt(rawTargetID, 10, 64)
		if err != nil || parsed < 0 {
			writeBadRequest(w, "target_id must be a non-negative integer")
			return
		}
		targetID = parsed
	}
	targetIDs := []int64{}
	if targetID > 0 {
		targetIDs = append(targetIDs, targetID)
	}
	if rawTargetIDs := strings.TrimSpace(r.URL.Query().Get("target_ids")); rawTargetIDs != "" {
		for _, part := range strings.Split(rawTargetIDs, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			parsed, err := strconv.ParseInt(part, 10, 64)
			if err != nil || parsed <= 0 {
				writeBadRequest(w, "target_ids must be comma-separated positive integers")
				return
			}
			targetIDs = append(targetIDs, parsed)
		}
	}

	items := []store.CriticFeedback{}
	if chatSessionID != "" {
		queryTargetID := int64(0)
		if len(targetIDs) == 1 {
			queryTargetID = targetIDs[0]
		}
		found, err := s.Store.ListCriticFeedback(r.Context(), chatSessionID, targetType, queryTargetID)
		if err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				found = nil
			} else {
				writeInternalError(w, err.Error())
				return
			}
		}
		items = found
	}

	mappedItems := make([]map[string]any, 0, len(items))
	feedbacks := map[string]any{}
	targetFilter := map[int64]bool{}
	for _, id := range targetIDs {
		targetFilter[id] = true
	}
	for _, item := range items {
		mapped := map[string]any{
			"id":              item.ID,
			"created_at":      formatKSTTime(item.CreatedAt),
			"chat_session_id": item.ChatSessionID,
			"target_type":     item.TargetType,
			"target_id":       item.TargetID,
			"feedback_value":  item.FeedbackValue,
			"feedback_note":   nullableString(item.FeedbackNote),
			"source":          item.Source,
		}
		mappedItems = append(mappedItems, mapped)
		if len(targetFilter) > 0 && !targetFilter[item.TargetID] {
			continue
		}
		key := strconv.FormatInt(item.TargetID, 10)
		if _, exists := feedbacks[key]; exists {
			continue
		}
		feedbacks[key] = map[string]any{
			"feedback_value": item.FeedbackValue,
			"feedback_note":  nullableString(item.FeedbackNote),
			"created_at":     formatKSTTime(item.CreatedAt),
		}
	}
	var latest any
	if len(mappedItems) > 0 {
		latest = mappedItems[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"source":          "shadow",
		"chat_session_id": chatSessionID,
		"target_type":     targetType,
		"target_id":       targetID,
		"target_ids":      targetIDs,
		"latest":          latest,
		"feedbacks":       feedbacks,
		"items":           mappedItems,
		"count":           len(mappedItems),
	})
}

func (s *Server) feedbackTargetBelongsToSession(ctx context.Context, sid string, targetType string, targetID int64) (bool, error) {
	switch targetType {
	case "memory":
		items, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.ID == targetID && item.ChatSessionID == sid {
				return true, nil
			}
		}
	case "kg_triple":
		items, err := s.Store.ListKGTriples(ctx, sid)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.ID == targetID && item.ChatSessionID == sid {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Server) handleImportHypamemory(w http.ResponseWriter, r *http.Request) {
	var req dto.HypaImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_json", "detail": err.Error()})
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "missing_chat_session_id", "detail": "chat_session_id is required"})
		return
	}
	if len(req.Summaries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "empty_summaries", "detail": "summaries must not be empty"})
		return
	}
	if len(req.Summaries) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "too_many_summaries", "detail": "summaries are limited to 500 items", "total": len(req.Summaries)})
		return
	}
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "store_not_enabled", "detail": "store is not enabled"})
		return
	}

	extractionCfg := s.completeTurnExtractionConfig(nil)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	if !extractionCfg.Critic.hasConfig() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "error",
			"code":             "critic_config_missing",
			"detail":           "Critic provider settings are required for HypaMemory import.",
			"chat_session_id":  sid,
			"total":            len(req.Summaries),
			"succeeded":        0,
			"failed":           len(req.Summaries),
			"llm_config_trace": llmTrace,
			"warnings":         []string{"critic_config_missing"},
		})
		return
	}

	now := time.Now().UTC()
	succeeded := 0
	failed := 0
	skipped := 0
	errorDetails := []string{}
	warnings := []string{}
	criticTraces := []map[string]any{}
	scoringTraces := []map[string]any{}
	artifactCounts := map[string]int{
		"memories":         0,
		"direct_evidence":  0,
		"kg_triples":       0,
		"entities":         0,
		"trust_states":     0,
		"character_states": 0,
		"character_events": 0,
		"storylines":       0,
		"world_rules":      0,
		"pending_threads":  0,
		"active_states":    0,
		"vectors":          0,
	}

	for idx, summary := range req.Summaries {
		summary.ApplyDefaults()
		text := strings.TrimSpace(summary.Text)
		if text == "" {
			skipped++
			continue
		}
		turnIndex := hypaImportTurnIndex(summary, idx)
		tags := []string{}
		for _, tag := range summary.Tags {
			if cleaned := strings.TrimSpace(tag); cleaned != "" {
				tags = append(tags, cleaned)
			}
		}
		hints := []string{"source=HypaMemory import"}
		if summary.IsImportant != nil && *summary.IsImportant {
			hints = append(hints, "importance_hint=important")
		}
		if summary.Category != nil && strings.TrimSpace(*summary.Category) != "" {
			hints = append(hints, "category="+strings.TrimSpace(*summary.Category))
		}
		if len(tags) > 0 {
			hints = append(hints, "tags="+strings.Join(tags, ", "))
		}
		score, scoringTrace, scoringErr := s.scoreHypaMemoryImport(r.Context(), sid, summary, idx, turnIndex, extractionCfg.Critic)
		if scoringErr != nil {
			warnings = append(warnings, fmt.Sprintf("hypamemory_import_score_failed[%d]: %v", idx, scoringErr))
			score = fallbackHypaMemoryImportScore(summary)
			scoringTrace = map[string]any{"status": "fallback", "error": scoringErr.Error(), "score": score.mapValue()}
		}
		scoringTraces = append(scoringTraces, map[string]any{"index": idx, "turn_index": turnIndex, "trace": scoringTrace})
		hints = append(hints, "hypamemory_import_score="+mustCompactJSON(score.mapValue()))
		content := strings.Join(hints, "; ") + "\n" + text

		extraction, trace, err := s.runCompleteTurnCritic(r.Context(), sid, turnIndex, "HypaMemory import summary", content, nil, nil, extractionCfg.Critic)
		if trace != nil {
			criticTraces = append(criticTraces, map[string]any{"index": idx, "turn_index": turnIndex, "trace": trace})
		}
		if err != nil {
			failed++
			errorDetails = append(errorDetails, fmt.Sprintf("summary[%d]: %v", idx, err))
			continue
		}
		applyHypaMemoryImportScore(extraction, score)
		result := s.saveCriticExtractionArtifacts(r.Context(), sid, turnIndex, extraction, content, extractionCfg.Embedder, now)
		artifactCounts["memories"] += result.Memories
		artifactCounts["direct_evidence"] += result.Evidence
		artifactCounts["kg_triples"] += result.KGTriples
		artifactCounts["entities"] += result.Entities
		artifactCounts["trust_states"] += result.TrustStates
		artifactCounts["character_states"] += result.CharacterStates
		artifactCounts["character_events"] += result.CharacterEvents
		artifactCounts["storylines"] += result.Storylines
		artifactCounts["world_rules"] += result.WorldRules
		artifactCounts["pending_threads"] += result.PendingThreads
		artifactCounts["active_states"] += result.ActiveStates
		artifactCounts["vectors"] += result.VectorsUpserted
		warnings = append(warnings, result.Warnings...)
		if result.Errors > 0 {
			failed++
			errorDetails = append(errorDetails, result.ErrorDetails...)
			continue
		}
		succeeded++
	}

	status := "ok"
	if failed > 0 {
		status = "partial_error"
	}
	if succeeded == 0 && failed > 0 {
		status = "error"
	}
	detail := fmt.Sprintf("HypaMemory import processed: %d/%d succeeded", succeeded, len(req.Summaries))
	auditDetails := map[string]any{"total": len(req.Summaries), "succeeded": succeeded, "failed": failed, "skipped": skipped, "artifact_counts": artifactCounts}
	_ = s.Store.SaveAuditLog(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "hypamemory_import",
		TargetType:    "session",
		Summary:       detail,
		DetailsJSON:   mustCompactJSON(auditDetails),
		Source:        "import",
		CreatedAt:     now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"code":             "hypamemory_import",
		"detail":           detail,
		"chat_session_id":  sid,
		"total":            len(req.Summaries),
		"succeeded":        succeeded,
		"failed":           failed,
		"skipped":          skipped,
		"artifact_counts":  artifactCounts,
		"errors":           errorDetails,
		"warnings":         warnings,
		"llm_config_trace": llmTrace,
		"critic_traces":    criticTraces,
		"scoring_traces":   scoringTraces,
		"scoring_policy":   hypaMemoryImportScoringPolicyVersion,
	})
}

const hypaMemoryImportScoringPolicyVersion = "hypa-import-score.v1"

type hypaMemoryImportScore struct {
	Importance10             float64
	RetrievalPriority        float64
	ContinuityWeight         float64
	DialogueOrSensoryDensity float64
	MemoryKind               string
	TimeAnchorQuality        string
	KeepReason               string
	EntityRelevance          []string
	Source                   string
}

func (s hypaMemoryImportScore) mapValue() map[string]any {
	return map[string]any{
		"policy_version":                hypaMemoryImportScoringPolicyVersion,
		"importance_10":                 roundHypaImportScore(s.Importance10),
		"retrieval_priority":            roundHypaImportScore(s.RetrievalPriority),
		"continuity_weight":             roundHypaImportScore(s.ContinuityWeight),
		"dialogue_or_sensory_density":   roundHypaImportScore(s.DialogueOrSensoryDensity),
		"memory_kind":                   s.MemoryKind,
		"time_anchor_quality":           s.TimeAnchorQuality,
		"keep_reason":                   s.KeepReason,
		"entity_relevance":              s.EntityRelevance,
		"source":                        s.Source,
		"hypamemory_min_importance_10":  5.0,
		"used_as_importance_floor":      true,
		"truth_authority":               "support_import_scoring_only",
		"canonical_truth_write_allowed": false,
	}
}

func (s *Server) scoreHypaMemoryImport(ctx context.Context, sid string, summary dto.HypaImportSummary, idx int, turnIndex int, cfg completeTurnLLMConfig) (hypaMemoryImportScore, map[string]any, error) {
	fallback := fallbackHypaMemoryImportScore(summary)
	if !cfg.hasConfig() {
		return fallback, map[string]any{"status": "fallback", "reason": "critic_config_missing", "score": fallback.mapValue()}, nil
	}
	systemPrompt := "You score imported HypaMemory summaries for long-term story recall. Return only compact JSON."
	userPrompt := buildHypaMemoryImportScoringPrompt(sid, summary, idx, turnIndex)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 || maxTokens > 700 {
		maxTokens = 700
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 || maxCompletionTokens > maxTokens {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}

	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return fallback, nil, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return fallback, map[string]any{"status": "parse_failed", "raw_preview": truncateRunes(content, 800)}, err
	}
	score := normalizeHypaMemoryImportScore(parsed, fallback)
	trace := map[string]any{
		"status":         "ok",
		"policy_version": hypaMemoryImportScoringPolicyVersion,
		"model":          extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":          upstream["usage"],
		"score":          score.mapValue(),
	}
	return score, trace, nil
}

func buildHypaMemoryImportScoringPrompt(sid string, summary dto.HypaImportSummary, idx int, turnIndex int) string {
	payload := map[string]any{
		"chat_session_id":    sid,
		"summary_index":      idx,
		"import_turn_index":  turnIndex,
		"text":               boundCompleteTurnCriticInput(sanitizeTextForCriticInput(summary.Text), 6000),
		"source":             "HypaMemory import",
		"is_important":       boolPtrValue(summary.IsImportant, false),
		"category":           stringPtrValue(summary.Category, ""),
		"tags":               summary.Tags,
		"minimum_importance": "5/10 for any useful imported long-term memory",
		"truth_authority":    "support scoring only; do not invent new facts",
		"scoring_dimensions": []string{"importance_10", "retrieval_priority", "continuity_weight", "dialogue_or_sensory_density", "memory_kind", "entity_relevance", "time_anchor_quality", "keep_reason"},
	}
	body, _ := json.Marshal(payload)
	return strings.Join([]string{
		"Score this imported HypaMemory summary for Archive Center retrieval.",
		"Use the summary as an old long-term memory candidate, not as a current-turn fact.",
		"Return only JSON with:",
		`{"importance_10":number 1..10,"retrieval_priority":number 0..1,"continuity_weight":number 0..1,"dialogue_or_sensory_density":number 0..1,"memory_kind":string,"entity_relevance":[string],"time_anchor_quality":string,"keep_reason":string}`,
		"Rules:",
		"- importance_10 below 5 is only allowed for empty/noise/control text.",
		"- Prefer higher scores for relationship shifts, injuries, vows, secrets, locations, time anchors, recurring conflicts, irreversible decisions, and strong dialogue/sensory detail.",
		"- Do not add facts not present in the summary.",
		"Input JSON:",
		string(body),
	}, "\n")
}

func normalizeHypaMemoryImportScore(raw map[string]any, fallback hypaMemoryImportScore) hypaMemoryImportScore {
	score := hypaMemoryImportScore{
		Importance10:             clampFloat(extractionFloatFromAny(raw["importance_10"], extractionFloatFromAny(raw["importance_score"], fallback.Importance10)), 1, 10),
		RetrievalPriority:        clampFloat(extractionFloatFromAny(raw["retrieval_priority"], fallback.RetrievalPriority), 0, 1),
		ContinuityWeight:         clampFloat(extractionFloatFromAny(raw["continuity_weight"], fallback.ContinuityWeight), 0, 1),
		DialogueOrSensoryDensity: clampFloat(extractionFloatFromAny(raw["dialogue_or_sensory_density"], fallback.DialogueOrSensoryDensity), 0, 1),
		MemoryKind:               extractionFirstNonEmpty(extractionStringFromAny(raw["memory_kind"]), fallback.MemoryKind),
		TimeAnchorQuality:        extractionFirstNonEmpty(extractionStringFromAny(raw["time_anchor_quality"]), fallback.TimeAnchorQuality),
		KeepReason:               extractionFirstNonEmpty(extractionStringFromAny(raw["keep_reason"]), fallback.KeepReason),
		EntityRelevance:          stringsFromAny(raw["entity_relevance"]),
		Source:                   "llm_scoring",
	}
	if score.Importance10 < 5 {
		score.Importance10 = 5
	}
	if len(score.EntityRelevance) == 0 {
		score.EntityRelevance = fallback.EntityRelevance
	}
	return score
}

func fallbackHypaMemoryImportScore(summary dto.HypaImportSummary) hypaMemoryImportScore {
	important := boolPtrValue(summary.IsImportant, false)
	importance := 5.0
	if important {
		importance = 6.0
	}
	tags := make([]string, 0, len(summary.Tags))
	for _, tag := range summary.Tags {
		if cleaned := strings.TrimSpace(tag); cleaned != "" {
			tags = append(tags, cleaned)
		}
	}
	return hypaMemoryImportScore{
		Importance10:             importance,
		RetrievalPriority:        0.55,
		ContinuityWeight:         0.55,
		DialogueOrSensoryDensity: 0.35,
		MemoryKind:               extractionFirstNonEmpty(stringPtrValue(summary.Category, ""), "imported_hypamemory_summary"),
		TimeAnchorQuality:        "summary_level",
		KeepReason:               "Imported HypaMemory should remain retrievable as long-term continuity support.",
		EntityRelevance:          tags,
		Source:                   "fallback_floor",
	}
}

func applyHypaMemoryImportScore(extraction map[string]any, score hypaMemoryImportScore) {
	if extraction == nil {
		return
	}
	current := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10)
	next := current
	if score.Importance10 > next {
		next = score.Importance10
	}
	if next < 5 {
		next = 5
	}
	extraction["importance_score"] = next
	extraction["hypamemory_import_score"] = score.mapValue()
}

func boolPtrValue(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func roundHypaImportScore(v float64) float64 {
	return math.Round(v*100) / 100
}

func hypaImportTurnIndex(summary dto.HypaImportSummary, idx int) int {
	if summary.SourceTurnIndex != nil && *summary.SourceTurnIndex != 0 {
		n := *summary.SourceTurnIndex
		if n < 0 {
			return n
		}
		return -n
	}
	return -(idx + 1)
}
