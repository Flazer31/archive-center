package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type lorebookReferenceSnapshotRequest struct {
	ContractVersion        string                          `json:"contract_version"`
	ConsentState           string                          `json:"consent_state"`
	ObservationState       string                          `json:"observation_state"`
	CompleteSnapshot       bool                            `json:"complete_snapshot"`
	CharacterIndex         *int64                          `json:"character_index"`
	ChatIndex              *int64                          `json:"chat_index"`
	EnabledModuleIDs       []string                        `json:"enabled_module_ids"`
	EnabledModulesObserved bool                            `json:"enabled_modules_observed"`
	Entries                []lorebookReferenceEntryRequest `json:"entries"`
	ObservedAt             *time.Time                      `json:"observed_at"`
	HostProduct            string                          `json:"host_product,omitempty"`
}

// The JSON names below intentionally match the official RisuAI/PocketRisu
// loreBook shape. Unknown future fields are ignored; their presence must not
// invalidate an otherwise supported snapshot.
type lorebookReferenceEntryRequest struct {
	ID                string          `json:"id"`
	Key               string          `json:"key"`
	SecondKey         string          `json:"secondkey"`
	InsertOrder       *int            `json:"insertorder"`
	Comment           string          `json:"comment"`
	Content           string          `json:"content"`
	Mode              string          `json:"mode"`
	AlwaysActive      *bool           `json:"alwaysActive"`
	Selective         *bool           `json:"selective"`
	Extensions        json.RawMessage `json:"extentions"`
	ActivationPercent *float64        `json:"activationPercent"`
	UseRegex          *bool           `json:"useRegex"`
	BookVersion       *int64          `json:"bookVersion"`
	Folder            string          `json:"folder"`
}

func (s *Server) registerLorebookReferenceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions/{chat_session_id}/lorebook-reference/snapshots", s.handleLorebookReferenceSnapshot)
	mux.HandleFunc("GET /sessions/{chat_session_id}/lorebook-reference/current", s.handleLorebookReferenceCurrent)
}

func (s *Server) lorebookReferenceStore(w http.ResponseWriter) (store.LorebookReferenceStore, bool) {
	ref, ok := s.Store.(store.LorebookReferenceStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "lorebook_reference_store_unavailable", "MariaDB lorebook reference storage is unavailable")
		return nil, false
	}
	return ref, true
}

func (s *Server) lorebookReferenceExplorerStore(w http.ResponseWriter) (store.LorebookReferenceExplorerStore, bool) {
	ref, ok := s.Store.(store.LorebookReferenceExplorerStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "lorebook_reference_explorer_unavailable", "MariaDB lorebook reference browsing is unavailable")
		return nil, false
	}
	return ref, true
}

func lorebookReferenceOptionalIndex(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, store.ErrInvalidLorebookReference
	}
	return &value, nil
}

func (s *Server) handleLorebookReferenceCurrent(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.lorebookReferenceExplorerStore(w)
	if !ok {
		return
	}
	chatSessionID := strings.TrimSpace(r.PathValue("chat_session_id"))
	if chatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	scopeMode := strings.TrimSpace(r.URL.Query().Get("scope_mode"))
	if scopeMode == "" {
		scopeMode = "exact"
	}
	if scopeMode != "exact" && scopeMode != "latest_session" {
		writeError(w, http.StatusBadRequest, "lorebook_reference_scope_mode_invalid", "scope_mode must be exact or latest_session")
		return
	}
	characterIndex, err := lorebookReferenceOptionalIndex(r.URL.Query().Get("character_index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "lorebook_reference_scope_invalid", "character_index must be a non-negative integer")
		return
	}
	chatIndex, err := lorebookReferenceOptionalIndex(r.URL.Query().Get("chat_index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "lorebook_reference_scope_invalid", "chat_index must be a non-negative integer")
		return
	}
	enabledModulesObserved := false
	if raw := strings.TrimSpace(r.URL.Query().Get("enabled_modules_observed")); raw != "" {
		enabledModulesObserved, err = strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "lorebook_reference_scope_invalid", "enabled_modules_observed must be true or false")
			return
		}
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit <= 0 || limit > 100 {
			writeError(w, http.StatusBadRequest, "lorebook_reference_page_invalid", "limit must be between 1 and 100")
			return
		}
	}
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			writeError(w, http.StatusBadRequest, "lorebook_reference_page_invalid", "offset must be a non-negative integer")
			return
		}
	}
	var page *store.LorebookReferenceCurrentPage
	if scopeMode == "latest_session" {
		page, err = ref.GetLorebookReferenceLatestSessionPage(r.Context(), chatSessionID, limit, offset)
	} else {
		page, err = ref.GetLorebookReferenceCurrentPage(r.Context(), store.LorebookReferenceScope{
			ChatSessionID: chatSessionID, CharacterIndex: characterIndex, ChatIndex: chatIndex,
			EnabledModuleIDs: r.URL.Query()["enabled_module_id"], EnabledModulesObserved: enabledModulesObserved,
		}, limit, offset)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) && scopeMode == "latest_session" {
			page = &store.LorebookReferenceCurrentPage{
				Scope:   store.LorebookReferenceScope{ChatSessionID: chatSessionID},
				Entries: []store.LorebookReferenceEntryObservation{}, Limit: limit, Offset: offset,
			}
		} else if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "lorebook_reference_scope_not_found", "no stored lorebook reference exists for the observed Host scope")
			return
		} else if errors.Is(err, store.ErrInvalidLorebookReference) {
			writeError(w, http.StatusBadRequest, "lorebook_reference_scope_invalid", err.Error())
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "contract_version": store.LorebookReferenceCurrentViewV1,
		"scope_id": page.ScopeID, "scope": page.Scope, "latest_snapshot": page.LatestSnapshot,
		"items": page.Entries, "total": page.Total, "limit": page.Limit, "offset": page.Offset,
		"has_more": page.Offset+len(page.Entries) < page.Total,
	})
}

func (s *Server) handleLorebookReferenceSnapshot(w http.ResponseWriter, r *http.Request) {
	ref, ok := s.lorebookReferenceStore(w)
	if !ok {
		return
	}
	chatSessionID := strings.TrimSpace(r.PathValue("chat_session_id"))
	if chatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	var request lorebookReferenceSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.ContractVersion) != store.LorebookReferenceSnapshotContractV1 {
		writeError(w, http.StatusBadRequest, "lorebook_reference_contract_unsupported", "contract_version must be lorebook_reference_snapshot.v1")
		return
	}
	observedAt := time.Now().UTC()
	if request.ObservedAt != nil && !request.ObservedAt.IsZero() {
		observedAt = request.ObservedAt.UTC()
	}
	entries := make([]store.LorebookReferenceEntryObservation, 0, len(request.Entries))
	for ordinal, entry := range request.Entries {
		extensions := strings.TrimSpace(string(entry.Extensions))
		if extensions == "" || extensions == "null" {
			extensions = "{}"
		}
		if !json.Valid([]byte(extensions)) {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "lorebook entry extentions must be valid JSON")
			return
		}
		entries = append(entries, store.LorebookReferenceEntryObservation{
			HostEntryID: entry.ID, EntryOrdinal: ordinal, SourceKind: "current_host_aggregate",
			Key: entry.Key, SecondKey: entry.SecondKey, Comment: entry.Comment, Content: entry.Content,
			Mode: entry.Mode, AlwaysActive: entry.AlwaysActive, Selective: entry.Selective,
			UseRegex: entry.UseRegex, InsertOrder: entry.InsertOrder, ActivationPct: entry.ActivationPercent,
			BookVersion: entry.BookVersion, Folder: entry.Folder, ExtensionsJSON: extensions,
		})
	}
	provenance, _ := json.Marshal(map[string]any{
		"source":       "risu_host_adapter",
		"host_product": strings.TrimSpace(request.HostProduct),
		"api":          "getCurrentLorebookEntries",
		"scope_source": "current_host_aggregate",
	})
	snapshotID, err := newLorebookReferenceSnapshotID()
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	snapshot := &store.LorebookReferenceSnapshot{
		SnapshotID: snapshotID, ContractVersion: request.ContractVersion,
		ConsentState: request.ConsentState, ObservationState: request.ObservationState,
		CompleteSnapshot: request.CompleteSnapshot,
		Scope: store.LorebookReferenceScope{
			ChatSessionID: chatSessionID, CharacterIndex: request.CharacterIndex, ChatIndex: request.ChatIndex,
			EnabledModuleIDs: request.EnabledModuleIDs, EnabledModulesObserved: request.EnabledModulesObserved,
		},
		Entries: entries, ProvenanceJSON: string(provenance), ObservedAt: observedAt,
	}
	if err := store.ValidateLorebookReferenceSnapshot(snapshot); err != nil {
		writeError(w, http.StatusBadRequest, "lorebook_reference_snapshot_invalid", err.Error())
		return
	}
	result, err := ref.ApplyLorebookReferenceSnapshot(r.Context(), snapshot)
	if err != nil {
		if errors.Is(err, store.ErrInvalidLorebookReference) {
			writeError(w, http.StatusBadRequest, "lorebook_reference_snapshot_invalid", err.Error())
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "ok", "contract_version": store.LorebookReferenceSnapshotContractV1,
		"snapshot": result,
	})
}

func newLorebookReferenceSnapshotID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
