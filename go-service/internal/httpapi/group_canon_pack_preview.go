package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/canonpack"
	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) registerCanonPackPreviewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /canon-packs/preview/v1", s.handleCanonPackPreviewV1)
	mux.HandleFunc("POST /canon-packs/install/v1", s.handleCanonPackInstallV1)
	mux.HandleFunc("GET /canon-packs/v1", s.handleCanonPackListV1)
	mux.HandleFunc("GET /canon-packs/{install_id}/v1", s.handleCanonPackDetailV1)
	mux.HandleFunc("POST /canon-packs/{install_id}/lifecycle/v1", s.handleCanonPackLifecycleV1)
	mux.HandleFunc("GET /canon-packs/{install_id}/diagnostics/v1", s.handleCanonPackDiagnosticsV1)
	mux.HandleFunc("GET /canon-registry/v1", s.handleCanonRegistrySearchV1)
	mux.HandleFunc("POST /canon-overlays/v1", s.handleCanonOverlayCreateV1)
	mux.HandleFunc("GET /canon-overlays/v1", s.handleCanonOverlayListV1)
}

func (s *Server) handleCanonRegistrySearchV1(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.canonRegistryAuthorityStore(w)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeBadRequest(w, "q is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := registry.SearchCanonRegistry(r.Context(), query, limit)
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contract": "canon_registry_search.v1", "query": query, "items": items,
		"count": len(items), "ambiguous": len(items) > 1,
	})
}

func (s *Server) handleCanonPackDiagnosticsV1(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.canonRegistryAuthorityStore(w)
	if !ok {
		return
	}
	result, err := registry.GetCanonPackDiagnostics(r.Context(), strings.TrimSpace(r.PathValue("install_id")))
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCanonOverlayCreateV1(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.canonRegistryAuthorityStore(w)
	if !ok {
		return
	}
	var req struct {
		store.CanonOverlayInput
		ClientMeta map[string]any `json:"client_meta"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	rule, err := registry.CreateCanonOverlay(r.Context(), req.CanonOverlayInput)
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	indexStatus, indexResult := s.refreshCanonPackReferenceIndex(r.Context(), &store.CanonPackInstall{WorkID: rule.WorkID}, req.ClientMeta)
	writeJSON(w, http.StatusCreated, map[string]any{
		"contract": "canon_overlay_write.v1", "rule": rule,
		"index_status": indexStatus, "index_result": indexResult,
	})
}

func (s *Server) handleCanonOverlayListV1(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.canonRegistryAuthorityStore(w)
	if !ok {
		return
	}
	workID := strings.TrimSpace(r.URL.Query().Get("work_id"))
	if workID == "" {
		writeBadRequest(w, "work_id is required")
		return
	}
	items, err := registry.ListCanonOverlays(r.Context(), workID, strings.TrimSpace(r.URL.Query().Get("edition_row_id")))
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contract": "canon_overlay_list.v1", "items": items, "count": len(items)})
}

func (s *Server) handleCanonPackPreviewV1(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/zip") {
		writeError(w, http.StatusUnsupportedMediaType, "canon_pack_content_type_invalid", "Content-Type must be application/zip")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, canonpack.MaxArchiveBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "canon_pack_archive_too_large", "compressed archive exceeds the preview limit")
		return
	}
	report := canonpack.PreviewZIP(payload, s.Cfg.BuildVersion)
	status := http.StatusOK
	if !report.Valid {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, report)
}

func (s *Server) handleCanonPackInstallV1(w http.ResponseWriter, r *http.Request) {
	packStore, ok := s.canonPackAuthorityStore(w)
	if !ok {
		return
	}
	payload, ok := readCanonPackZIP(w, r)
	if !ok {
		return
	}
	inspection, err := canonpack.InspectZIP(payload, s.Cfg.BuildVersion)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !inspection.Report.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, inspection.Report)
		return
	}
	if inspection.Report.Summary.ReviewStatus != "approved" {
		writeError(w, http.StatusUnprocessableEntity, "canon_pack_not_approved", "only an approved Canon Pack can be installed")
		return
	}
	validationJSON, err := json.Marshal(inspection.Report)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	installed, err := packStore.InstallCanonPack(r.Context(), store.CanonPackInstallInput{
		ManifestJSON: inspection.ManifestJSON, ManifestSHA256: inspection.ManifestSHA256,
		ArchiveSHA256: inspection.ArchiveSHA256, ValidationReportJSON: validationJSON,
	})
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	installed.IndexStatus, installed.IndexResult = s.refreshCanonPackReferenceIndex(r.Context(), installed, nil)
	writeJSON(w, http.StatusCreated, installed)
}

func (s *Server) handleCanonPackListV1(w http.ResponseWriter, r *http.Request) {
	packStore, ok := s.canonPackAuthorityStore(w)
	if !ok {
		return
	}
	lifecycle := strings.TrimSpace(r.URL.Query().Get("lifecycle_status"))
	if lifecycle != "" && !canonPackLifecycleStatus(lifecycle) {
		writeError(w, http.StatusBadRequest, "canon_pack_lifecycle_invalid", "invalid lifecycle_status filter")
		return
	}
	items, err := packStore.ListCanonPackInstalls(r.Context(), lifecycle)
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contract": store.CanonPackLifecycleContract, "items": items, "count": len(items)})
}

func (s *Server) handleCanonPackDetailV1(w http.ResponseWriter, r *http.Request) {
	packStore, ok := s.canonPackAuthorityStore(w)
	if !ok {
		return
	}
	item, err := packStore.GetCanonPackInstall(r.Context(), strings.TrimSpace(r.PathValue("install_id")))
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCanonPackLifecycleV1(w http.ResponseWriter, r *http.Request) {
	packStore, ok := s.canonPackAuthorityStore(w)
	if !ok {
		return
	}
	var req struct {
		Action     string         `json:"action"`
		ClientMeta map[string]any `json:"client_meta"`
	}
	if !decodeReferenceJSON(w, r, &req) {
		return
	}
	action := strings.TrimSpace(req.Action)
	if action != "activate" && action != "deactivate" && action != "remove" && action != "rollback" {
		writeError(w, http.StatusBadRequest, "canon_pack_lifecycle_action_invalid", "action must be activate, deactivate, remove, or rollback")
		return
	}
	result, err := packStore.SetCanonPackLifecycle(r.Context(), strings.TrimSpace(r.PathValue("install_id")), action)
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	installed, err := packStore.GetCanonPackInstall(r.Context(), strings.TrimSpace(r.PathValue("install_id")))
	if err != nil {
		writeCanonPackStoreError(w, err)
		return
	}
	result.IndexStatus, result.IndexResult = s.refreshCanonPackReferenceIndex(r.Context(), installed, req.ClientMeta)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) refreshCanonPackReferenceIndex(ctx context.Context, installed *store.CanonPackInstall, clientMeta map[string]any) (string, map[string]any) {
	ref, ok := s.Store.(store.ReferenceLibraryStore)
	if !ok || installed == nil {
		return "pending_reindex", map[string]any{"reason": "reference_library_unavailable"}
	}
	embedder := s.completeTurnExtractionConfig(clientMeta).Embedder
	if !embedder.hasConfig() {
		return "pending_reindex", map[string]any{"reason": "embedding_config_missing"}
	}
	if s.ReferenceVectorOpenError != nil || s.ReferenceVector == nil {
		reason := "reference_vector_unavailable"
		if s.ReferenceVectorOpenError != nil {
			reason = s.ReferenceVectorOpenError.Error()
		}
		return "pending_reindex", map[string]any{"reason": reason}
	}
	continuities, err := ref.ListReferenceContinuities(ctx, installed.WorkID)
	if err != nil {
		return "pending_reindex", map[string]any{"reason": err.Error()}
	}
	results := map[string]any{}
	for _, continuity := range continuities {
		if !strings.EqualFold(strings.TrimSpace(continuity.Status), "active") {
			continue
		}
		result := s.runReferenceAutomaticVectorIndex(ctx, ref, installed.WorkID, continuity.ContinuityID, embedder, func(map[string]any) {})
		results[continuity.ContinuityID] = result
		if strings.TrimSpace(fmt.Sprint(result["status"])) != "completed" {
			return "pending_reindex", map[string]any{"continuities": results}
		}
	}
	return "current", map[string]any{"continuities": results}
}

func (s *Server) canonPackAuthorityStore(w http.ResponseWriter) (store.CanonPackStore, bool) {
	if s.Cfg.StoreMode != config.StoreModeMariaDBAuthority {
		writeError(w, http.StatusServiceUnavailable, "canon_pack_install_unavailable", "Canon Pack lifecycle requires MariaDB authority mode")
		return nil, false
	}
	packStore, ok := s.Store.(store.CanonPackStore)
	if !ok || packStore == nil {
		writeError(w, http.StatusServiceUnavailable, "canon_pack_install_unavailable", "Canon Pack lifecycle store is unavailable")
		return nil, false
	}
	return packStore, true
}

func (s *Server) canonRegistryAuthorityStore(w http.ResponseWriter) (store.CanonRegistryStore, bool) {
	if s.Cfg.StoreMode != config.StoreModeMariaDBAuthority {
		writeError(w, http.StatusServiceUnavailable, "canon_registry_unavailable", "Canon Registry requires MariaDB authority mode")
		return nil, false
	}
	registry, ok := s.Store.(store.CanonRegistryStore)
	if !ok || registry == nil {
		writeError(w, http.StatusServiceUnavailable, "canon_registry_unavailable", "Canon Registry store is unavailable")
		return nil, false
	}
	return registry, true
}

func readCanonPackZIP(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/zip") {
		writeError(w, http.StatusUnsupportedMediaType, "canon_pack_content_type_invalid", "Content-Type must be application/zip")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, canonpack.MaxArchiveBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "canon_pack_archive_too_large", "compressed archive exceeds the preview limit")
		return nil, false
	}
	return payload, true
}

func canonPackLifecycleStatus(value string) bool {
	switch value {
	case "staged", "active", "inactive", "failed", "removed":
		return true
	default:
		return false
	}
}

func writeCanonPackStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "canon_pack_not_found", err.Error())
	case errors.Is(err, store.ErrReferenceConflict):
		writeError(w, http.StatusConflict, "canon_pack_lifecycle_conflict", err.Error())
	case errors.Is(err, store.ErrInvalidReference):
		writeError(w, http.StatusUnprocessableEntity, "canon_pack_install_invalid", err.Error())
	default:
		writeInternalError(w, err.Error())
	}
}
