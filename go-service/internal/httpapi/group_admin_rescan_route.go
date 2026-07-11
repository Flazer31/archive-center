package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleAdminRescan(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/rescan")
		return
	}

	var req adminRescanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.Background || boolFromAny(req.ClientMeta["background"]) {
		if s.AdminJobs == nil {
			s.AdminJobs = newAdminJobManager()
		}
		jobRequest := map[string]any{
			"chat_session_id": sid,
			"max_items":       req.MaxItems,
			"turn_indices":    req.TurnIndices,
			"client_meta":     req.ClientMeta,
			"dry_run":         req.DryRun,
			"background":      true,
		}
		job := s.AdminJobs.start("rescan", sid, jobRequest, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
			bgReq := req
			bgReq.Background = false
			return s.runAdminRescanWithProgress(ctx, sid, bgReq, progress)
		})
		reusedRunningJob := boolFromAny(job["reused_running_job"])
		jobStatus := strings.TrimSpace(fmt.Sprint(job["status"]))
		job["status"] = "accepted"
		job["job_status"] = jobStatus
		job["poll_route"] = "/admin/jobs/" + fmt.Sprint(job["job_id"])
		if reusedRunningJob {
			job["note"] = "an existing rescan for this session is still running; poll the existing job route"
		} else {
			job["note"] = "rescan is running in the background; poll the job route for progress"
		}
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	result, err := s.runAdminRescan(r.Context(), sid, req)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_rescan",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Admin rescan requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"dry_run":         req.DryRun,
			"candidate_count": result["candidate_count"],
			"succeeded":       result["succeeded"],
			"failed":          result["failed"],
			"skipped":         result["skipped"],
			"processed_turns": result["processed_turns"],
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	result["audit_written"] = true
	result["changed_at"] = now
	writeJSON(w, http.StatusOK, result)
}
