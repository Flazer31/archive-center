package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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
