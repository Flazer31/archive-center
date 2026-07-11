package httpapi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
	"github.com/shirou/gopsutil/v3/disk"
)

// Chroma shadow: R0 probes

func (s *Server) handleChromaPreflight(w http.ResponseWriter, r *http.Request) {
	persistDir := s.Cfg.ChromaShadowPersistDir
	if persistDir == "" {
		persistDir = ".chroma_shadow"
	}
	if abs, err := filepath.Abs(persistDir); err == nil {
		persistDir = abs
	}

	_, err := os.Stat(persistDir)
	exists := err == nil
	writable := false
	if exists {
		f, err := os.CreateTemp(persistDir, ".write_probe_*")
		if err == nil {
			writable = true
			f.Close()
			os.Remove(f.Name())
		}
	}

	diskFree := 0.0
	diskTotal := 0.0
	if exists {
		if usage, err := disk.Usage(persistDir); err == nil && usage != nil {
			diskFree = safeRound2Float(float64(usage.Free) / (1024 * 1024))
			diskTotal = safeRound2Float(float64(usage.Total) / (1024 * 1024))
		}
	}

	provider := s.Cfg.EmbedderProvider
	model := s.Cfg.EmbedderModel
	endpoint := s.Cfg.EmbedderEndpoint
	if provider == "" {
		provider = "voyageai"
	}
	if model == "" {
		model = "voyage-4-large"
	}
	if endpoint == "" {
		endpoint = "https://api.voyageai.com/v1/embeddings"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"step":            "17-C1",
		"ready":           false,
		"issues":          []any{"chromadb_dependency_unavailable"},
		"enabled":         true,
		"collection_name": "archive_center_shadow",
		"embedder_identity": map[string]any{
			"provider": provider,
			"model":    model,
			"endpoint": endpoint,
		},
		"retrieval_document_schema": map[string]any{
			"version":       "q1a.v1",
			"tiers":         []any{"memory", "episode", "chapter", "arc", "saga"},
			"index_version": "q1e.v1",
		},
		"session_partitioning": map[string]any{
			"mode":                 "session_partitioned",
			"session_partitioned":  true,
			"shadow_runtime_mode":  "shadow",
			"shadow_write_enabled": true,
			"active_session_count": 0,
		},
		"persist_directory": map[string]any{
			"path":     persistDir,
			"exists":   exists,
			"writable": writable,
		},
		"disk_budget": map[string]any{
			"budget_mb":      2048,
			"free_mb":        diskFree,
			"total_mb":       diskTotal,
			"target_size_mb": 0.16,
		},
		"dependency": map[string]any{
			"package":   "chromadb",
			"available": false,
			"version":   nil,
			"detail":    "ModuleNotFoundError",
		},
	})
}

// Chroma shadow: R1 read/audit evidence surfaces.

func (s *Server) handleChromaBackfillDryRun(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowBackfillDryRunRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	storeEnabled := false
	vectorCount := 0
	vectorErr := "unavailable"
	memoryCount := 0
	evidenceCount := 0
	kgTripleCount := 0
	episodeCount := 0

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
		if eps, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0); err == nil {
			episodeCount = len(eps)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow backfill-dry-run is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":         storeEnabled,
			"memory_count":          memoryCount,
			"evidence_count":        evidenceCount,
			"kg_triple_count":       kgTripleCount,
			"episode_count":         episodeCount,
			"vector_count":          vectorCount,
			"vector_error":          vectorErr,
			"eligible_for_backfill": memoryCount + evidenceCount + kgTripleCount + episodeCount - vectorCount,
			"sync_scope":            "selected_tiers",
			"allowed_tiers":         []string{"memory", "evidence", "kg_triple", "episode"},
			"primary_source":        "canonical_row",
			"vector_role":           "shadow_backfill",
		},
		"counts": map[string]any{
			"memory":    memoryCount,
			"evidence":  evidenceCount,
			"kg_triple": kgTripleCount,
			"episode":   episodeCount,
			"vector":    vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaReembedAudit(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReembedAuditRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	memoriesWithEmbedding := 0
	memoryModels := map[string]int{}
	embeddingIdentity := s.currentEmbeddingModelIdentity()
	currentModel := embeddingIdentity.Model
	statusCounts := map[string]int{}
	needsReembedCount := 0
	evidenceCount := 0
	episodeCount := 0
	vectorCount := 0
	vectorErr := "unavailable"

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
			for _, m := range mems {
				status := classifyMemoryEmbeddingStatus(m, currentModel)
				statusCounts[status]++
				if memoryEmbeddingNeedsReembed(status) == true {
					needsReembedCount++
				}
				if strings.TrimSpace(m.Embedding) != "" {
					memoriesWithEmbedding++
				}
				model := m.EmbeddingModel
				if model == "" {
					model = "none"
				}
				memoryModels[model]++
			}
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if eps, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0); err == nil {
			episodeCount = len(eps)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow reembed-audit is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":                      storeEnabled,
			"memory_count":                       memoryCount,
			"memories_with_embedding":            memoriesWithEmbedding,
			"memory_embedding_models":            memoryModels,
			"current_project_embedding_model":    currentModel,
			"current_embedding_model_source":     embeddingIdentity.Source,
			"memory_status_counts":               statusCounts,
			"needs_reembed_count":                needsReembedCount,
			"evidence_count":                     evidenceCount,
			"episode_count":                      episodeCount,
			"vector_count":                       vectorCount,
			"vector_error":                       vectorErr,
			"reembed_rule":                       "summary_edit_triggers_upsert",
			"model_switch_replay_policy_version": "em1e.v1",
			"retrieval_fallback_before_reembed":  "hybrid_degrade_or_importance_only",
			"retrieval_state_after_reembed":      "embedding_current",
			"truth_authority":                    "store_canonical",
			"vector_role":                        "accelerator_only",
		},
		"counts": map[string]any{
			"memory":   memoryCount,
			"evidence": evidenceCount,
			"episode":  episodeCount,
			"vector":   vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"policy_version":  "em1e.v1",
			"chat_session_id": sid,
		},
	})
}

// handleChromaReembedSchedule implements EM-1d: session-level reembed schedule surface.
// This is a shadow/dry-run contract; it does not execute live reembed.
func (s *Server) handleChromaReembedSchedule(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReembedAuditRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	embeddingIdentity := s.currentEmbeddingModelIdentity()
	currentModel := embeddingIdentity.Model
	schedule := []map[string]any{}
	candidateCount := 0
	modelMismatchCount := 0
	missingCount := 0

	if s.Store != nil && sid != "" {
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			for _, m := range mems {
				status := classifyMemoryEmbeddingStatus(m, currentModel)
				if memoryEmbeddingNeedsReembed(status) == true {
					candidateCount++
					if strings.HasPrefix(status, "missing_embedding") {
						missingCount++
					} else {
						modelMismatchCount++
					}
					schedule = append(schedule, map[string]any{
						"memory_id":            m.ID,
						"turn_index":           m.TurnIndex,
						"status":               status,
						"stored_model":         m.EmbeddingModel,
						"current_model":        currentModel,
						"current_model_source": embeddingIdentity.Source,
						"needs_reembed":        memoryEmbeddingNeedsReembed(status),
						"retrieval_fallback":   memoryEmbeddingRetrievalFallback(status),
						"action":               "dry_run_reembed",
						"truth_authority":      "store_canonical",
					})
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow reembed-schedule is Store-backed dry-run contract (EM-1d)",
		"evidence": map[string]any{
			"store_enabled":          s.Store != nil,
			"chat_session_id":        sid,
			"current_model":          currentModel,
			"current_model_source":   embeddingIdentity.Source,
			"candidate_count":        candidateCount,
			"missing_count":          missingCount,
			"model_mismatch_count":   modelMismatchCount,
			"schedule":               schedule,
			"live_execution_allowed": false,
			"truth_authority":        "store_canonical",
			"vector_role":            "accelerator_only",
		},
		"trace_summary": map[string]any{
			"step":            "EM-1d",
			"source":          "shadow",
			"policy_version":  "em1d.v1",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaFallbackRunbook(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowFallbackRunbookRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	var storeStats *store.StatsResult
	vectorAvailable := false
	vectorCount := 0
	vectorErr := "unavailable"

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if st, err := s.Store.Stats(r.Context()); err == nil {
			storeStats = &st
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorAvailable = true
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	statsMap := map[string]any{}
	if storeStats != nil {
		statsMap = map[string]any{
			"chat_logs":  storeStats.ChatLogs,
			"memories":   storeStats.Memories,
			"kg_triples": storeStats.KgTriples,
		}
	}

	degradedMode := "canonical_baseline"
	if vectorAvailable {
		degradedMode = "vector_ready"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow fallback-runbook is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":             storeEnabled,
			"store_stats":               statsMap,
			"vector_available":          vectorAvailable,
			"vector_count":              vectorCount,
			"vector_error":              vectorErr,
			"fallback_policy":           "store_first_then_vector",
			"degraded_mode":             degradedMode,
			"fail_open_baseline":        true,
			"retrieval_baseline":        "sqlite_canonical",
			"canonical_baseline_source": "sqlite_store",
			"sqlite_canonical_baseline": true,
		},
		"counts": map[string]any{
			"vector": vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaReleaseHygiene(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReleaseHygieneRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	evidenceCount := 0
	tombstonedCount := 0
	kgTripleCount := 0
	chatLogCount := 0

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
			for _, e := range evs {
				if e.Tombstoned {
					tombstonedCount++
				}
			}
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
		if logs, err := s.Store.ListChatLogs(r.Context(), sid, 0, 0); err == nil {
			chatLogCount = len(logs)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow release-hygiene is Store-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":       storeEnabled,
			"memory_count":        memoryCount,
			"evidence_count":      evidenceCount,
			"tombstoned_count":    tombstonedCount,
			"kg_triple_count":     kgTripleCount,
			"chat_log_count":      chatLogCount,
			"stale_vector_policy": "tombstone_before_delete",
			"delete_policy":       "canonical_row_first",
			"rollback_policy":     "vector_doc_rollback_with_id",
			"merge_policy":        "merge_stale_vectors_to_tombstone",
		},
		"counts": map[string]any{
			"memory":     memoryCount,
			"evidence":   evidenceCount,
			"tombstoned": tombstonedCount,
			"kg_triple":  kgTripleCount,
			"chat_log":   chatLogCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaVisibilityGuard(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowVisibilityGuardRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	evidenceCount := 0
	kgTripleCount := 0
	vectorCount := 0
	vectorErr := "unavailable"
	visibilityGap := 0

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	storeTotal := memoryCount + evidenceCount + kgTripleCount
	if vectorErr == "" && storeTotal >= vectorCount {
		visibilityGap = storeTotal - vectorCount
	}

	driftStatus := "aligned"
	if visibilityGap > 0 {
		driftStatus = "drift_detected"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow visibility-guard is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":           storeEnabled,
			"memory_count":            memoryCount,
			"evidence_count":          evidenceCount,
			"kg_triple_count":         kgTripleCount,
			"vector_count":            vectorCount,
			"vector_error":            vectorErr,
			"visibility_gap":          visibilityGap,
			"drift_policy":            "shadow_degraded",
			"drift_status":            driftStatus,
			"canonical_count":         storeTotal,
			"canonical_to_vector_gap": visibilityGap,
			"drift_action":            "keep_canonical_baseline",
		},
		"counts": map[string]any{
			"memory":    memoryCount,
			"evidence":  evidenceCount,
			"kg_triple": kgTripleCount,
			"vector":    vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaHealthProbe(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowHealthProbeRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	storeErr := ""
	vectorHealthStatus := "unavailable"
	vectorCount := 0
	vectorHealth := map[string]any{}

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if _, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				storeErr = "not_enabled"
				storeEnabled = false
			} else {
				storeErr = err.Error()
			}
		}
	}

	if s.Vector != nil {
		if h, err := s.Vector.Health(r.Context()); err == nil {
			vectorHealthStatus = h.Status
			vectorHealth = map[string]any{
				"status":      h.Status,
				"collection":  h.Collection,
				"total_count": h.TotalCount,
				"model_ready": h.ModelReady,
			}
			if sid != "" {
				if c, cerr := s.Vector.Count(r.Context(), sid); cerr == nil {
					vectorCount = c
				}
			}
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorHealthStatus = "not_enabled"
		} else {
			vectorHealthStatus = "error"
			vectorHealth["error"] = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow health-probe is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":        storeEnabled,
			"store_error":          storeErr,
			"vector_health_status": vectorHealthStatus,
			"vector_count":         vectorCount,
			"vector_health":        vectorHealth,
		},
		"counts": map[string]any{
			"vector": vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaBootstrap(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /chroma-shadow/bootstrap")
}

func (s *Server) handleChromaBackfillBatch(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /chroma-shadow/backfill-batch")
}

func (s *Server) handleChromaRebuildDrill(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"status": "error",
		"code":   CodeShadowGuard,
		"error":  "POST /chroma-shadow/rebuild-drill is not available in R0/R1 shadow mode",
		"trace_summary": map[string]any{
			"step":          "17-C1-r1",
			"source":        "shadow",
			"rebuild_owner": "chroma_shadow_orchestrator",
			"rebuild_modes": []string{"targeted", "partial", "full"},
		},
	})
}

func (s *Server) handleChromaAdoptionGate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "ok",
		"code":                 "chroma_adoption_gate_closed",
		"note":                 "chroma-shadow adoption-gate is closed in R1 shadow mode",
		"live_cutover_allowed": false,
		"cutover_prerequisites": []string{
			"vector_health_green",
			"visibility_gap_zero",
			"fallback_rate_acceptable",
		},
		"required_green_gates": []string{
			"health_probe",
			"visibility_guard",
			"fallback_runbook",
		},
		"multi_tier_cutover_scope":  "memory_only",
		"adoption_gate_state":       "closed",
		"owner_decision_state":      "pending_pre_12_5",
		"scope_truth_authority":     "store_canonical_truth",
		"long_memory_input_quality": "requires_replay_green",
		"future_125_owner_decision": map[string]any{
			"owner_decision_state":      "pending_pre_12_5",
			"scope_truth_authority":     "store_canonical_truth",
			"long_memory_input_quality": "requires_replay_green",
			"required_green_gates": []string{
				"sync_replay_gate",
				"stale_vector_rollback_rebuild_gate",
				"fail_open_sqlite_baseline_gate",
			},
		},
		"trace_summary": map[string]any{
			"step":   "17-C1-r1",
			"source": "shadow",
		},
	})
}
