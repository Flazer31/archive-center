package httpapi

import (
	"net/http"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

const (
	completeTurnCriticPipelineVersion          = "ea1j.v1"
	completeTurnCriticPreviewPassVersion       = "ea1k.v1"
	completeTurnDirectEvidenceRetentionVersion = "ea1l.v1"
	completeTurnMaintenancePlanVersion         = "r3c.v1"
	completeTurnHierarchyPromotionVersion      = "step23.guarded_worker.v1"
	completeTurnAutoContinueUserInputMarker    = "[auto-continue]"
)

type completeTurnMaintenanceHandoff struct {
	Enqueued       bool
	QueueStatus    string
	QueueDepth     int
	RefreshEnabled bool
	RefreshPlan    map[string]any
	Trace          map[string]any
	AuditSaved     int
	Attempted      int
	Errors         int
	ErrorDetails   []string
}

// ---------------------------------------------------------------------------
// Route registration
// ---------------------------------------------------------------------------

// registerTurnRoutes mounts the core turn surface.
// All endpoints in this group are R2 (authority-required write).
func (s *Server) registerTurnRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /turns", s.handleTurns)
	mux.HandleFunc("POST /turns/repair-replay", s.handleTurnsRepairReplay)
	mux.HandleFunc("POST /turns/complete", s.handleTurnsComplete)
	mux.HandleFunc("POST /complete-turn", s.handleCompleteTurn)
	mux.HandleFunc("GET /complete-turn/request-status", s.handleCompleteTurnRequestStatus)
	mux.HandleFunc("POST /prepare-turn", s.handlePrepareTurn)
	mux.HandleFunc("POST /effective-inputs", s.handleEffectiveInputs)
	mux.HandleFunc("DELETE /rollback/{turn_index}", s.handleRollback)
	mux.HandleFunc("POST /rollback/decision", s.handleRollbackDecision)
	mux.HandleFunc("POST /session-routing/turn-resolution", s.handleSessionRoutingTurnResolution)

}

func (s *Server) handleTurns(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /turns")
}

func (s *Server) handleTurnsRepairReplay(w http.ResponseWriter, r *http.Request) {
	var req dto.ChatLogRepairReplayRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(*req.ChatSessionID)
	if sid == "" {
		sid = "default"
	}

	repairReplayPlan := buildRepairReplayPlan(sid, req, s.usesShadowWriteStore(), s.storeWriteSource())
	if s.usesShadowWriteStore() {
		result, err := s.runChatLogRepairReplay(r.Context(), sid, req)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		result["repair_replay_plan"] = repairReplayPlan
		writeJSON(w, http.StatusOK, result)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"source":             "shadow",
		"chat_session_id":    sid,
		"repair_replay_plan": repairReplayPlan,
		"note":               "repair-replay is a shadow plan; no mutations performed",
	})
}

func (s *Server) handleTurnsComplete(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /turns/complete")
}

// handleCompleteTurn processes a turn completion.
// In default/noop mode it does not write.
// In store-write-enabled modes it persists chat logs, effective input, audit
// logs, memory, direct evidence, and KG triples when clearly present.
