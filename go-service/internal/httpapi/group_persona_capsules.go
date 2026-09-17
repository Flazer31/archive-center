package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleCreatePersonaCapsule(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	var req personaCapsuleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.PersonaKey = strings.TrimSpace(req.PersonaKey)
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	req.Title = strings.TrimSpace(req.Title)
	if req.PersonaKey == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "persona_key is required")
		return
	}
	if req.SourceChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	if len(req.Entries) == 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "at least one capsule entry is required")
		return
	}
	entries := make([]store.PersonaMemoryEntry, 0, len(req.Entries))
	for _, raw := range req.Entries {
		text := strings.TrimSpace(raw.MemoryText)
		if text == "" {
			continue
		}
		tagsJSON, err := json.Marshal(personaCapsuleEntryContractTags(req.Mode, raw.Portability, raw.Tags))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid tags")
			return
		}
		entries = append(entries, store.PersonaMemoryEntry{
			SourceMemoryType: strings.TrimSpace(raw.SourceMemoryType),
			SourceMemoryID:   raw.SourceMemoryID,
			SourceTurn:       raw.SourceTurn,
			MemoryText:       text,
			EmotionalWeight:  raw.EmotionalWeight,
			Importance10:     clampPersonaImportance10(raw.Importance10),
			Portability:      strings.TrimSpace(raw.Portability),
			TagsJSON:         string(tagsJSON),
			EvidenceExcerpt:  strings.TrimSpace(raw.EvidenceExcerpt),
			InjectionPolicy:  strings.TrimSpace(raw.InjectionPolicy),
		})
	}
	if len(entries) == 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "at least one non-empty capsule entry is required")
		return
	}
	capsule, err := st.CreatePersonaMemoryCapsule(r.Context(), &store.PersonaMemoryCapsule{
		PersonaKey:          req.PersonaKey,
		SourceChatSessionID: req.SourceChatSessionID,
		SourceCharacterName: strings.TrimSpace(req.SourceCharacterName),
		Title:               req.Title,
		Mode:                strings.TrimSpace(req.Mode),
		Summary:             strings.TrimSpace(req.Summary),
	}, entries)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":        "ok",
		"capsule":       capsule,
		"entries_count": len(entries),
		"policy":        personaCapsuleSupportPolicy(),
	})
}

func personaCapsuleEntryContractTags(mode, portability string, existing []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+2)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, tag := range existing {
		add(tag)
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	portability = strings.ToLower(strings.TrimSpace(portability))
	protectedKind := ""
	switch mode {
	case "full_loop_memory", "regression_recollection", "reincarnation_carryover", "isekai_carryover":
		protectedKind = mode
	}
	if protectedKind == "" && portability == "cross_world" {
		protectedKind = portability
	}
	if protectedKind != "" {
		add("secret_guard")
		add("protected_secret_kind:" + protectedKind)
	}
	return out
}

func (s *Server) handleListPersonaCapsules(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	items, err := st.ListPersonaMemoryCapsules(r.Context(), store.PersonaCapsuleFilter{
		PersonaKey:          strings.TrimSpace(r.URL.Query().Get("persona_key")),
		SourceChatSessionID: strings.TrimSpace(r.URL.Query().Get("source_chat_session_id")),
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"items":     items,
		"count":     len(items),
		"policy":    personaCapsuleSupportPolicy(),
		"next_step": "attach_capsule_to_target_session_before_prepare_turn_injection",
	})
}

func (s *Server) handleGetPersonaCapsule(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	capsuleID, err := personaCapsulePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	capsule, entries, err := st.GetPersonaMemoryCapsule(r.Context(), capsuleID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "persona capsule not found")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"capsule": capsule,
		"entries": entries,
		"policy":  personaCapsuleSupportPolicy(),
	})
}

func (s *Server) handleDeletePersonaCapsule(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	capsuleID, err := personaCapsulePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if err := st.DeletePersonaMemoryCapsule(r.Context(), capsuleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "persona capsule not found")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "deleted": true, "capsule_id": capsuleID})
}

func (s *Server) handleAttachPersonaCapsule(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	capsuleID, err := personaCapsulePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	var req personaCapsuleAttachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.TargetChatSessionID = strings.TrimSpace(req.TargetChatSessionID)
	if req.TargetChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "target_chat_session_id is required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := st.AttachPersonaMemoryCapsule(r.Context(), &store.PersonaCapsuleAttachment{
		CapsuleID:           capsuleID,
		TargetChatSessionID: req.TargetChatSessionID,
		InjectionMode:       strings.TrimSpace(req.InjectionMode),
		Enabled:             enabled,
	}); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"attached":               true,
		"capsule_id":             capsuleID,
		"target_chat_session_id": req.TargetChatSessionID,
		"policy":                 personaCapsuleSupportPolicy(),
	})
}

func (s *Server) handleDetachPersonaCapsule(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	capsuleID, err := personaCapsulePathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	targetSID := strings.TrimSpace(r.URL.Query().Get("target_chat_session_id"))
	if targetSID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "target_chat_session_id is required")
		return
	}
	if err := st.DetachPersonaMemoryCapsule(r.Context(), capsuleID, targetSID); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "detached": true, "capsule_id": capsuleID, "target_chat_session_id": targetSID})
}

func (s *Server) handleListPersonaCapsuleAttachments(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	targetSID := strings.TrimSpace(r.URL.Query().Get("target_chat_session_id"))
	if targetSID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "target_chat_session_id is required")
		return
	}
	items, err := st.ListPersonaCapsuleAttachments(r.Context(), targetSID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  items,
		"count":  len(items),
		"policy": personaCapsuleSupportPolicy(),
	})
}

func (s *Server) handleListAttachedPersonaEntries(w http.ResponseWriter, r *http.Request) {
	st, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	targetSID := strings.TrimSpace(r.URL.Query().Get("target_chat_session_id"))
	if targetSID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "target_chat_session_id is required")
		return
	}
	requestedLimit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	queryLimit := requestedLimit
	if requestedLimit > 0 && requestedLimit < int(^uint(0)>>1) {
		queryLimit++
	}
	items, err := st.ListAttachedPersonaMemoryEntries(r.Context(), targetSID, queryLimit)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	hasMore := requestedLimit > 0 && len(items) > requestedLimit
	if hasMore {
		items = items[:requestedLimit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"items":           items,
		"count":           len(items),
		"has_more":        hasMore,
		"complete_result": requestedLimit <= 0 || !hasMore,
		"selection_mode":  map[bool]string{true: "explicit_limit", false: "all_matching_rows"}[requestedLimit > 0],
		"policy":          personaCapsuleSupportPolicy(),
	})
}
