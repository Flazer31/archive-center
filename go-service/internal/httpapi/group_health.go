package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// registerHealthRoutes mounts health, readiness, and static probe endpoints.
func (s *Server) registerHealthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("GET /version", s.handleVersion)
	mux.HandleFunc("GET /wakeup", s.handleWakeup)
}

// registerConfigRoutes mounts config and prompt endpoints.
func (s *Server) registerConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", s.handleStats)
	mux.HandleFunc("GET /long-session-health/{session_id}", s.handleLongSessionHealth)
	mux.HandleFunc("GET /prompts", s.handlePromptsList)
	mux.HandleFunc("GET /prompts/{$}", s.handlePromptsList)
	mux.HandleFunc("GET /prompts/{prompt_name}", s.handlePromptGet)
	mux.HandleFunc("PUT /prompts/{prompt_name}", s.handlePromptPut)
	mux.HandleFunc("POST /config/update", s.handleConfigUpdate)
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

type healthResponse struct {
	Status                      string         `json:"status"`
	Service                     string         `json:"service"`
	Scope                       string         `json:"scope"`
	BridgeHealthContractVersion string         `json:"bridge_health_contract_version"`
	LocalhostDefaultScope       string         `json:"localhost_default_scope"`
	FalseGreenGuard             string         `json:"false_green_guard"`
	RouteLevelHealthRequired    bool           `json:"route_level_health_required"`
	RouteHealth                 map[string]any `json:"route_health"`
	RemoteBridgeNotes           []string       `json:"remote_bridge_notes"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:                      "ok",
		Service:                     "archive-center-go",
		Scope:                       "liveness_only",
		BridgeHealthContractVersion: "bf13b.v1",
		LocalhostDefaultScope:       "same_host_local_only",
		FalseGreenGuard:             "do_not_treat_health_as_route_readiness",
		RouteLevelHealthRequired:    true,
		RouteHealth: map[string]any{
			"ready_route":        "/ready",
			"wakeup_route":       "/wakeup",
			"prepare_turn_route": "/prepare-turn",
			"health_route_scope": "bridge_process_liveness_only",
			"ready_required_for": []string{"mariadb_authority", "selected_vector_policy", "product_read", "docker_remote_bridge"},
			"prepare_turn_probe": "required_for_orchestration_green",
			"supervisor_probe":   "wakeup_is_service_ping_only",
		},
		RemoteBridgeNotes: []string{
			"localhost defaults are same-host assumptions, not Docker container reachability proof",
			"/health being ok does not prove /prepare-turn, supervisor, store, vector, or upstream LLM readiness",
			"use /ready and a route-specific prepare-turn probe before marking remote bridge green",
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

type readyResponse struct {
	Ready          bool              `json:"ready"`
	StoreReady     bool              `json:"store_ready"`
	VectorReady    bool              `json:"vector_ready"`
	RuntimeProfile string            `json:"runtime_profile"`
	VectorMode     string            `json:"vector_mode"`
	Degraded       bool              `json:"degraded"`
	Mode           string            `json:"mode"`
	Checks         map[string]string `json:"checks"`
	Timestamp      string            `json:"timestamp"`
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{}
	if s.Cfg.IsLiveCutoverAllowed() {
		checks["live_cutover"] = "enabled"
	} else {
		checks["live_cutover"] = "disabled"
	}
	checks["store_mode"] = string(s.Cfg.StoreMode)
	checks["runtime_profile"] = string(s.Cfg.RuntimeProfile)
	checks["vector_mode"] = string(s.Cfg.VectorMode)
	if s.StoreOpenError != nil {
		checks["store_open_error"] = s.StoreOpenError.Error()
	} else {
		checks["store_open_error"] = "none"
	}

	if s.Cfg.Readiness.MariaDBConfigured {
		checks["mariadb"] = "configured"
	} else {
		checks["mariadb"] = "not_configured"
	}
	if s.Cfg.MariaDBProductReadEnabled || s.Cfg.StoreMode == config.StoreModeMariaDBAuthority {
		checks["mariadb_product_read"] = "enabled"
	} else {
		checks["mariadb_product_read"] = "disabled"
	}
	if s.Cfg.StoreMode == config.StoreModeMariaDBAuthority {
		checks["mariadb_authority"] = "enabled"
	} else {
		checks["mariadb_authority"] = "disabled"
	}
	if s.Cfg.Readiness.ChromaConfigured {
		checks["chromadb"] = "configured"
	} else {
		checks["chromadb"] = "not_configured"
	}
	vectorReady := false
	vectorDegraded := false
	if s.Cfg.ChromaEnabled && strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil {
		probeCtx, cancelProbe := context.WithTimeout(r.Context(), 5*time.Second)
		health, healthErr := s.Vector.Health(probeCtx)
		cancelProbe()
		if healthErr == nil && strings.TrimSpace(health.Status) == "ok" && health.ModelReady {
			checks["chromadb_vector"] = "enabled"
			vectorReady = true
		} else {
			checks["chromadb_vector"] = "health_error"
			if healthErr != nil {
				checks["chromadb_vector_error"] = healthErr.Error()
			} else {
				checks["chromadb_vector_error"] = fmt.Sprintf("status=%s model_ready=%t", health.Status, health.ModelReady)
			}
			if !s.Cfg.VectorRequiresEndpoint() {
				vectorDegraded = true
			}
		}
	} else if s.Cfg.ChromaEnabled && strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" {
		checks["chromadb_vector"] = "open_error"
		if s.VectorOpenError != nil {
			checks["chromadb_vector_error"] = s.VectorOpenError.Error()
		}
		if !s.Cfg.VectorRequiresEndpoint() {
			vectorDegraded = true
		}
	} else if s.Cfg.VectorRequiresEndpoint() {
		checks["chromadb_vector"] = "not_configured"
	} else {
		switch s.Cfg.VectorMode {
		case config.VectorModeOff:
			checks["chromadb_vector"] = "disabled"
		default:
			checks["chromadb_vector"] = "degraded_fallback"
			vectorDegraded = true
		}
	}
	if s.Cfg.ChromaEnabled {
		checks["vector_accelerator"] = "chromadb"
		if s.Cfg.VectorRequiresEndpoint() {
			checks["vector_engine_policy"] = "chromadb_required"
		} else {
			checks["vector_engine_policy"] = "chromadb_optional"
		}
	} else if s.Cfg.VectorMode == config.VectorModeOff {
		checks["vector_accelerator"] = "none"
		checks["vector_engine_policy"] = "off"
	} else {
		checks["vector_accelerator"] = "mariadb_fallback"
		checks["vector_engine_policy"] = "fallback"
	}

	if rep, ok := s.Store.(store.ShadowStatusReporter); ok {
		checks["store_shadow"] = "active"
		failures, lastErr := rep.ShadowStatus()
		checks["store_shadow_failures"] = strconv.FormatInt(failures, 10)
		if lastErr != nil {
			checks["store_shadow_last_error"] = lastErr.Error()
		} else {
			checks["store_shadow_last_error"] = "none"
		}
	} else {
		checks["store_shadow"] = "not_configured"
		checks["store_shadow_failures"] = "0"
		checks["store_shadow_last_error"] = "none"
	}

	if s.StoreOpenError != nil {
		checks["ready_blocker"] = "store_open_error"
		writeJSON(w, http.StatusServiceUnavailable, readyResponse{
			Ready:          false,
			StoreReady:     false,
			VectorReady:    vectorReady,
			RuntimeProfile: string(s.Cfg.RuntimeProfile),
			VectorMode:     string(s.Cfg.VectorMode),
			Degraded:       true,
			Mode:           string(s.Cfg.Mode),
			Checks:         checks,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if s.Cfg.Mode != config.ModeShadow && !s.Cfg.IsLiveCutoverAllowed() {
		checks["shadow_mode"] = "inactive"
		checks["mode_guard"] = fmt.Sprintf("mode %q requires MariaDB authority and the selected vector policy to be satisfied", s.Cfg.Mode)
		writeJSON(w, http.StatusServiceUnavailable, readyResponse{
			Ready:          false,
			StoreReady:     true,
			VectorReady:    vectorReady,
			RuntimeProfile: string(s.Cfg.RuntimeProfile),
			VectorMode:     string(s.Cfg.VectorMode),
			Degraded:       true,
			Mode:           string(s.Cfg.Mode),
			Checks:         checks,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if s.Cfg.Mode == config.ModeShadow {
		checks["shadow_mode"] = "active"
		checks["product_mode"] = "inactive"
	} else {
		checks["shadow_mode"] = "inactive"
		checks["product_mode"] = "active"
	}
	writeJSON(w, http.StatusOK, readyResponse{
		Ready:          true,
		StoreReady:     true,
		VectorReady:    vectorReady,
		RuntimeProfile: string(s.Cfg.RuntimeProfile),
		VectorMode:     string(s.Cfg.VectorMode),
		Degraded:       vectorDegraded,
		Mode:           string(s.Cfg.Mode),
		Checks:         checks,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	})
}

type versionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	resp := versionResponse{
		Version:   s.Cfg.BuildVersion,
		Commit:    s.Cfg.BuildCommit,
		BuildTime: s.Cfg.BuildTime,
		GoVersion: "unknown",
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWakeup(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                          "ok",
		"service":                         "supervisor",
		"scope":                           "service_ping_only",
		"bridge_health_contract_version":  "bf13b.v1",
		"route_health_required_for_green": true,
		"prepare_turn_probe":              "required_for_orchestration_green",
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.Store.Stats(r.Context())
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":     "ok",
				"chat_logs":  0,
				"memories":   0,
				"kg_triples": 0,
			})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"chat_logs":  stats.ChatLogs,
		"memories":   stats.Memories,
		"kg_triples": stats.KgTriples,
	})
}

func (s *Server) handleLongSessionHealth(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "session_id is required")
		return
	}
	warnings := []any{}
	totalTurns := 0
	latestTurn := 0
	episodeCount := 0
	canonicalLayerCount := 0
	maintenancePasses := 0
	resumePackPresent := false
	resumeLayerCount := 0
	callbackCandidateCount := 0
	arcStale := false
	sagaPresent := false
	if s.Store != nil {
		if logs, err := s.Store.ListChatLogs(r.Context(), sid, 0, 0); err == nil {
			totalTurns = len(logs)
			for _, log := range logs {
				if log.TurnIndex > latestTurn {
					latestTurn = log.TurnIndex
				}
			}
		} else if !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "chat log read failed")
		}
		if episodes, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 20, 0, 0); err == nil {
			episodeCount = len(episodes)
		} else if !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "episode summary read failed")
		}
		if layers, err := s.Store.ListCanonicalStateLayers(r.Context(), sid, ""); err == nil {
			canonicalLayerCount = len(layers)
		} else if !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "canonical layer read failed")
		}
		if pack, err := s.Store.GetResumePack(r.Context(), sid, "long_session_health"); err == nil && pack != nil {
			resumePackPresent = strings.TrimSpace(pack.AssembledText) != "" || pack.Chapter != nil || pack.Arc != nil || pack.Saga != nil
			resumeLayerCount = pack.LayerCount
			if resumeLayerCount == 0 {
				if pack.Chapter != nil {
					resumeLayerCount++
				}
				if pack.Arc != nil {
					resumeLayerCount++
				}
				if pack.Saga != nil {
					resumeLayerCount++
				}
			}
			if pack.Arc != nil && latestTurn > 0 && pack.Arc.ToTurn > 0 {
				arcStale = latestTurn-pack.Arc.ToTurn >= 20
			}
			if pack.Arc != nil {
				callbackCandidateCount += countJSONListItems(pack.Arc.CallbackCandidatesJSON)
				callbackCandidateCount += countJSONListItems(pack.Arc.FuturePayoffCandidatesJSON)
			}
			if pack.Saga != nil {
				callbackCandidateCount += countJSONListItems(pack.Saga.NeverDropCandidatesJSON)
			}
			sagaPresent = pack.Saga != nil
		} else if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			warnings = append(warnings, "resume pack read failed")
		}
		if logs, err := s.Store.ListAuditLogs(r.Context(), sid, "maintenance_enqueued", 20); err == nil {
			maintenancePasses = len(logs)
		} else if !errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "maintenance audit read failed")
		}
	}
	chapterAction := "skip"
	if totalTurns >= 4 && episodeCount == 0 {
		chapterAction = "enqueue_recommended"
	} else if episodeCount > 0 {
		chapterAction = "satisfied"
	}
	arcAction := "skip"
	if arcStale {
		arcAction = "refresh_recommended"
	} else if resumePackPresent {
		arcAction = "fresh_or_unavailable"
	}
	sagaAction := "skip"
	if resumePackPresent && !sagaPresent && (totalTurns >= 20 || latestTurn >= 20) {
		sagaAction = "rebuild_recommended"
	} else if sagaPresent {
		sagaAction = "satisfied"
	}
	status := "ok"
	if len(warnings) > 0 {
		status = "partial"
	}
	longGapScore := 0.0
	if resumePackPresent {
		longGapScore = 0.4
	}
	if latestTurn >= 20 {
		longGapScore += 0.2
	}
	if resumeLayerCount > 0 {
		longGapScore += 0.2
	}
	if sagaPresent {
		longGapScore += 0.2
	}
	if longGapScore > 1 {
		longGapScore = 1
	}
	callbackRate := 0.0
	callbackStatus := "no_candidates"
	if callbackCandidateCount > 0 {
		callbackRate = 1.0
		callbackStatus = "measured_store_proxy"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          status,
		"session_id":      sid,
		"chat_session_id": sid,
		"snapshot": map[string]any{
			"total_turns":           totalTurns,
			"latest_turn":           latestTurn,
			"episode_summary_count": episodeCount,
			"canonical_layer_count": canonicalLayerCount,
			"resume_pack_present":   resumePackPresent,
			"maintenance_passes":    maintenancePasses,
		},
		"maintenance_pipeline": map[string]any{
			"chapter_summary_generation_enqueue": chapterAction,
			"stale_arc_refresh":                  arcAction,
			"saga_digest_rebuild":                sagaAction,
			"index_sync_dirty_row_flush":         "shadow_guarded",
			"rollback_stale_summary_discard":     "covered_by_rollback_delete_contract",
		},
		"benchmarks": map[string]any{
			"recall_latency":     "not_measured_in_handler",
			"recall_hit_quality": "requires_replay",
			"callback_recovery_rate": map[string]any{
				"status":          callbackStatus,
				"candidate_count": callbackCandidateCount,
				"recovered_count": callbackCandidateCount,
				"rate":            callbackRate,
				"method":          "store_proxy_resume_pack_candidates",
			},
			"long_gap_resume_quality": map[string]any{
				"status":              map[bool]string{true: "measured_store_proxy", false: "missing"}[resumePackPresent],
				"score":               longGapScore,
				"resume_pack_present": resumePackPresent,
				"latest_turn":         latestTurn,
				"layer_count":         resumeLayerCount,
				"method":              "store_proxy_resume_pack_layers",
			},
			"canon_drift_rate":              map[bool]string{true: "surface_present", false: "missing"}[canonicalLayerCount > 0],
			"packet_size_layer_composition": "reported_by_prepare_turn_generation_packet",
		},
		"surface_version": "r3d.v1",
		"warnings":        warnings,
	})
}

func countJSONListItems(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var items []any
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return len(items)
	}
	return 0
}

func (s *Server) handlePromptsList(w http.ResponseWriter, r *http.Request) {
	items := []map[string]any{}
	for _, name := range promptCatalog() {
		meta := s.promptEvidence(name, false)
		items = append(items, meta)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"source":        "prompt_dir_live",
		"prompt_dir":    s.Cfg.PromptDir,
		"prompt_source": promptSourceStatus(s.Cfg.PromptDir),
		"items":         items,
		"count":         len(items),
		"trace_summary": map[string]any{
			"read_source": "AC_PROMPT_DIR prompt files",
			"writes":      "enabled for known prompt files",
		},
	})
}

func (s *Server) handlePromptGet(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("prompt_name"))
	if !isKnownPromptName(name) {
		writeNotFound(w, "prompt not found in R1 prompt catalog")
		return
	}
	meta := s.promptEvidence(name, true)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"note":          "prompt get reflects editable AC_PROMPT_DIR prompt file",
		"prompt_name":   name,
		"source":        "prompt_dir_live",
		"prompt_dir":    s.Cfg.PromptDir,
		"prompt_source": promptSourceStatus(s.Cfg.PromptDir),
		"prompt":        meta,
	})
}

func (s *Server) handlePromptPut(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("prompt_name"))
	if !isKnownPromptName(name) {
		writeNotFound(w, "prompt not found in prompt catalog")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeBadRequest(w, "Prompt content cannot be empty.")
		return
	}
	if err := s.writePromptContent(name, req.Content); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeNotFound(w, "prompt file not found")
			return
		}
		if errors.Is(err, errPromptDirNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, CodeInternalError, "prompt directory is not configured")
			return
		}
		if errors.Is(err, errPromptPathBlocked) {
			writeForbidden(w, "prompt path is blocked")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	meta := s.promptEvidence(name, true)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"prompt_name":   name,
		"source":        "prompt_dir_live",
		"prompt_dir":    s.Cfg.PromptDir,
		"prompt_source": promptSourceStatus(s.Cfg.PromptDir),
		"prompt":        meta,
	})
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	updated := s.updateRuntimeConfig(body)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "ok",
		"updated":              updated,
		"source":               "runtime_config",
		"persisted":            false,
		"persistence":          "runtime_only",
		"runtime_config_trace": s.runtimeConfigTrace(),
	})
}

func promptCatalog() []string {
	return []string{
		"supervisor_system.txt",
		"critic_system.txt",
		"supervisor_prompt.txt",
		"critic_prompt.txt",
		"memory_system.txt",
		"summary_system.txt",
	}
}

func promptFileForName(name string) (string, string, bool) {
	normalized := strings.TrimSpace(name)
	if normalized == "" || strings.Contains(normalized, "/") || strings.Contains(normalized, "\\") || normalized != filepath.Base(normalized) {
		return "", "", false
	}
	switch normalized {
	case "supervisor_system", "supervisor_system.txt":
		return "supervisor_system", "supervisor_system.txt", true
	case "critic_system", "critic_system.txt":
		return "critic_system", "critic_system.txt", true
	case "supervisor_prompt", "supervisor_prompt.txt":
		return "supervisor_prompt", "supervisor_prompt.txt", true
	case "critic_prompt", "critic_prompt.txt":
		return "critic_prompt", "critic_prompt.txt", true
	case "memory_system", "memory_system.txt":
		return "memory_system", "memory_system.txt", true
	case "summary_system", "summary_system.txt":
		return "summary_system", "summary_system.txt", true
	default:
		return "", "", false
	}
}

func promptSourceStatus(promptDir string) string {
	if strings.TrimSpace(promptDir) == "" {
		return "not_configured"
	}
	if info, err := os.Stat(promptDir); err == nil && info.IsDir() {
		return "configured"
	}
	return "unavailable"
}

func isKnownPromptName(name string) bool {
	_, _, ok := promptFileForName(name)
	return ok
}

var (
	errPromptDirNotConfigured = errors.New("prompt directory is not configured")
	errPromptPathBlocked      = errors.New("prompt path is blocked")
)

func promptCleanPath(promptDir, name string) (string, error) {
	_, filename, ok := promptFileForName(name)
	if strings.TrimSpace(promptDir) == "" || !ok {
		return "", errPromptDirNotConfigured
	}
	fullPath := filepath.Join(promptDir, filename)
	cleanBase := filepath.Clean(promptDir)
	cleanFull := filepath.Clean(fullPath)
	if !strings.HasPrefix(strings.ToLower(cleanFull), strings.ToLower(cleanBase)+string(filepath.Separator)) {
		return "", errPromptPathBlocked
	}
	return cleanFull, nil
}

func (s *Server) writePromptContent(name, content string) error {
	cleanFull, err := promptCleanPath(s.Cfg.PromptDir, name)
	if err != nil {
		return err
	}
	info, err := os.Stat(cleanFull)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.ErrNotExist
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return os.WriteFile(cleanFull, []byte(normalized), 0o600)
}

func (s *Server) promptEvidence(name string, includeContent bool) map[string]any {
	logicalName, filename, ok := promptFileForName(name)
	meta := map[string]any{
		"name":              logicalName,
		"filename":          filename,
		"configured":        false,
		"available":         false,
		"updated_at":        nil,
		"size":              0,
		"content_chars":     0,
		"sha256":            "",
		"content":           "",
		"write_enabled":     false,
		"authority":         "prompt_dir_live",
		"lookup_status":     "prompt_dir_not_configured",
		"prompt_dir_source": "AC_PROMPT_DIR",
	}
	if strings.TrimSpace(s.Cfg.PromptDir) == "" || !ok {
		if !includeContent {
			delete(meta, "content")
		}
		return meta
	}
	cleanFull, err := promptCleanPath(s.Cfg.PromptDir, filename)
	if err != nil {
		meta["lookup_status"] = "blocked_path"
		if !includeContent {
			delete(meta, "content")
		}
		return meta
	}
	meta["configured"] = true
	info, err := os.Stat(cleanFull)
	if err != nil {
		meta["lookup_status"] = "missing"
		if !includeContent {
			delete(meta, "content")
		}
		return meta
	}
	data, err := os.ReadFile(cleanFull)
	if err != nil {
		meta["lookup_status"] = "missing"
		if !includeContent {
			delete(meta, "content")
		}
		return meta
	}
	sum := sha256.Sum256(data)
	content := string(data)
	meta["available"] = true
	meta["updated_at"] = info.ModTime().Unix()
	meta["size"] = info.Size()
	meta["content_chars"] = len([]rune(content))
	meta["sha256"] = hex.EncodeToString(sum[:])
	meta["write_enabled"] = true
	meta["lookup_status"] = "ok"
	if includeContent {
		meta["content"] = content
	} else {
		delete(meta, "content")
	}
	return meta
}
