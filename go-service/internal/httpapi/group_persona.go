package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// registerPersonaRoutes mounts portable persona-memory capsule endpoints.
func (s *Server) registerPersonaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /persona-entity-memories", s.handleCreateProtagonistEntityMemory)
	mux.HandleFunc("GET /persona-entity-memories", s.handleListProtagonistEntityMemories)
	mux.HandleFunc("POST /subjective-entity-memories", s.handleCreateProtagonistEntityMemory)
	mux.HandleFunc("GET /subjective-entity-memories", s.handleListProtagonistEntityMemories)
	mux.HandleFunc("PATCH /subjective-entity-memories/{memory_id}", s.handlePatchProtagonistEntityMemory)
	mux.HandleFunc("DELETE /subjective-entity-memories/{memory_id}", s.handleDeleteProtagonistEntityMemory)
	mux.HandleFunc("GET /subjective-entity-memories/entities", s.handleListSubjectiveEntityMemoryEntities)
	mux.HandleFunc("POST /subjective-entity-memories/capsule", s.handleCreateSubjectiveEntityMemoryCapsule)
	mux.HandleFunc("POST /subjective-entity-memories/alias-repair", s.handleRepairSubjectiveEntityMemoryAliases)
	mux.HandleFunc("POST /subjective-entity-memories/force-merge", s.handleForceMergeSubjectiveEntityMemories)
	mux.HandleFunc("POST /persona-capsules", s.handleCreatePersonaCapsule)
	mux.HandleFunc("GET /persona-capsules", s.handleListPersonaCapsules)
	mux.HandleFunc("GET /persona-capsules/attachments", s.handleListPersonaCapsuleAttachments)
	mux.HandleFunc("GET /persona-capsules/attached-entries", s.handleListAttachedPersonaEntries)
	mux.HandleFunc("GET /persona-capsules/{capsule_id}", s.handleGetPersonaCapsule)
	mux.HandleFunc("DELETE /persona-capsules/{capsule_id}", s.handleDeletePersonaCapsule)
	mux.HandleFunc("POST /persona-capsules/{capsule_id}/attach", s.handleAttachPersonaCapsule)
	mux.HandleFunc("DELETE /persona-capsules/{capsule_id}/attach", s.handleDetachPersonaCapsule)
}

type personaCapsuleCreateRequest struct {
	PersonaKey          string                     `json:"persona_key"`
	SourceChatSessionID string                     `json:"source_chat_session_id"`
	SourceCharacterName string                     `json:"source_character_name"`
	Title               string                     `json:"title"`
	Mode                string                     `json:"mode"`
	Summary             string                     `json:"summary"`
	Entries             []personaCapsuleEntryInput `json:"entries"`
}

type personaCapsuleEntryInput struct {
	SourceMemoryType string   `json:"source_memory_type"`
	SourceMemoryID   int64    `json:"source_memory_id"`
	SourceTurn       int      `json:"source_turn_index"`
	MemoryText       string   `json:"memory_text"`
	EmotionalWeight  float64  `json:"emotional_weight"`
	Importance10     float64  `json:"importance_10"`
	Portability      string   `json:"portability"`
	Tags             []string `json:"tags"`
	EvidenceExcerpt  string   `json:"evidence_excerpt"`
	InjectionPolicy  string   `json:"injection_policy"`
}

type subjectiveEntityMemoryCapsuleRequest struct {
	OwnerEntityKey      string  `json:"owner_entity_key"`
	OwnerEntityName     string  `json:"owner_entity_name"`
	OwnerEntityRole     string  `json:"owner_entity_role"`
	OwnerVisibility     string  `json:"owner_visibility"`
	SourceChatSessionID string  `json:"source_chat_session_id"`
	SourceCharacterName string  `json:"source_character_name"`
	Title               string  `json:"title"`
	Mode                string  `json:"mode"`
	Summary             string  `json:"summary"`
	TargetRevealPolicy  string  `json:"target_reveal_policy"`
	MemoryIDs           []int64 `json:"memory_ids"`
}

type personaCapsuleAttachRequest struct {
	TargetChatSessionID string `json:"target_chat_session_id"`
	InjectionMode       string `json:"injection_mode"`
	Enabled             *bool  `json:"enabled"`
}

type protagonistEntityMemoryRequest struct {
	PersonaEntityKey    string   `json:"persona_entity_key"`
	PersonaEntityName   string   `json:"persona_entity_name"`
	OwnerEntityKey      string   `json:"owner_entity_key"`
	OwnerEntityName     string   `json:"owner_entity_name"`
	OwnerEntityRole     string   `json:"owner_entity_role"`
	OwnerVisibility     string   `json:"owner_visibility"`
	SourceChatSessionID string   `json:"source_chat_session_id"`
	SourceCharacterName string   `json:"source_character_name"`
	SourceTurn          int      `json:"source_turn_index"`
	MemoryText          string   `json:"memory_text"`
	EvidenceExcerpt     string   `json:"evidence_excerpt"`
	SecretGuard         bool     `json:"secret_guard"`
	Portability         string   `json:"portability"`
	TargetRevealPolicy  string   `json:"target_reveal_policy"`
	Tags                []string `json:"tags"`
	Importance10        float64  `json:"importance_10"`
	EmotionalWeight     float64  `json:"emotional_weight"`
}

type subjectiveEntityAliasRepairRequest struct {
	SourceChatSessionID string `json:"source_chat_session_id"`
	Apply               bool   `json:"apply"`
	Limit               int    `json:"limit"`
}

type subjectiveEntityForceMergeRequest struct {
	SourceChatSessionID string   `json:"source_chat_session_id"`
	TargetOwnerKey      string   `json:"target_owner_key"`
	TargetOwnerName     string   `json:"target_owner_name"`
	TargetOwnerRole     string   `json:"target_owner_role"`
	TargetVisibility    string   `json:"target_visibility"`
	SourceOwnerKeys     []string `json:"source_owner_keys"`
	MemoryIDs           []int64  `json:"memory_ids"`
	Limit               int      `json:"limit"`
}

type protagonistEntityMemoryUpdateRequest struct {
	PersonaEntityKey    string   `json:"persona_entity_key"`
	PersonaEntityName   string   `json:"persona_entity_name"`
	OwnerEntityKey      string   `json:"owner_entity_key"`
	OwnerEntityName     string   `json:"owner_entity_name"`
	OwnerEntityRole     string   `json:"owner_entity_role"`
	OwnerVisibility     string   `json:"owner_visibility"`
	SourceChatSessionID string   `json:"source_chat_session_id"`
	SourceCharacterName string   `json:"source_character_name"`
	MemoryText          string   `json:"memory_text"`
	EvidenceExcerpt     string   `json:"evidence_excerpt"`
	SecretGuard         bool     `json:"secret_guard"`
	Portability         string   `json:"portability"`
	TargetRevealPolicy  string   `json:"target_reveal_policy"`
	Tags                []string `json:"tags"`
	TagsJSON            string   `json:"tags_json"`
	Importance10        float64  `json:"importance_10"`
	EmotionalWeight     float64  `json:"emotional_weight"`
}

func (s *Server) personaCapsuleStore() (store.PersonaCapsuleStore, bool) {
	if s == nil || s.Store == nil {
		return nil, false
	}
	st, ok := s.Store.(store.PersonaCapsuleStore)
	return st, ok
}

func (s *Server) protagonistEntityMemoryStore() (store.ProtagonistEntityMemoryStore, bool) {
	if s == nil || s.Store == nil {
		return nil, false
	}
	st, ok := s.Store.(store.ProtagonistEntityMemoryStore)
	return st, ok
}

func (s *Server) handleCreateProtagonistEntityMemory(w http.ResponseWriter, r *http.Request) {
	st, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	var req protagonistEntityMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.PersonaEntityKey = strings.TrimSpace(req.PersonaEntityKey)
	req.PersonaEntityName = strings.TrimSpace(req.PersonaEntityName)
	req.OwnerEntityKey = strings.TrimSpace(req.OwnerEntityKey)
	req.OwnerEntityName = strings.TrimSpace(req.OwnerEntityName)
	req.OwnerEntityRole = normalizeSubjectiveEntityRole(req.OwnerEntityRole)
	req.OwnerVisibility = normalizeSubjectiveEntityVisibility(req.OwnerVisibility)
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	req.MemoryText = strings.TrimSpace(req.MemoryText)
	if req.OwnerEntityKey == "" {
		req.OwnerEntityKey = req.PersonaEntityKey
	}
	if req.PersonaEntityKey == "" {
		req.PersonaEntityKey = req.OwnerEntityKey
	}
	if req.PersonaEntityKey == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "owner_entity_key is required")
		return
	}
	if req.OwnerEntityName == "" {
		req.OwnerEntityName = req.PersonaEntityName
	}
	if req.PersonaEntityName == "" {
		req.PersonaEntityName = req.OwnerEntityName
	}
	if req.PersonaEntityName == "" {
		req.PersonaEntityName = req.PersonaEntityKey
	}
	if req.OwnerEntityName == "" {
		req.OwnerEntityName = req.OwnerEntityKey
	}
	if req.SourceChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	if req.MemoryText == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "memory_text is required")
		return
	}
	canonicalOwner := s.canonicalSubjectiveEntityOwner(r.Context(), req.SourceChatSessionID, req.OwnerEntityKey, req.OwnerEntityName)
	req.OwnerEntityKey = canonicalOwner.Key
	req.PersonaEntityKey = canonicalOwner.Key
	if canonicalOwner.Name != "" {
		req.OwnerEntityName = canonicalOwner.Name
		req.PersonaEntityName = canonicalOwner.Name
	}
	req.Tags = append(req.Tags, canonicalOwner.AliasTags...)
	if canonicalOwner.Changed {
		req.Tags = append(req.Tags, "entity_alias_canonicalized")
	}
	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid tags")
		return
	}
	portability := strings.TrimSpace(req.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	targetRevealPolicy := normalizeTargetRevealPolicy(req.TargetRevealPolicy)
	item, err := st.CreateProtagonistEntityMemory(r.Context(), &store.ProtagonistEntityMemory{
		PersonaEntityKey:    req.PersonaEntityKey,
		PersonaEntityName:   req.PersonaEntityName,
		OwnerEntityKey:      req.OwnerEntityKey,
		OwnerEntityName:     req.OwnerEntityName,
		OwnerEntityRole:     req.OwnerEntityRole,
		OwnerVisibility:     req.OwnerVisibility,
		SourceChatSessionID: req.SourceChatSessionID,
		SourceCharacterName: strings.TrimSpace(req.SourceCharacterName),
		SourceTurn:          req.SourceTurn,
		MemoryText:          req.MemoryText,
		EvidenceExcerpt:     strings.TrimSpace(req.EvidenceExcerpt),
		SecretGuard:         req.SecretGuard,
		Portability:         portability,
		TargetRevealPolicy:  targetRevealPolicy,
		TagsJSON:            string(tagsJSON),
		Importance10:        clampPersonaImportance10(req.Importance10),
		EmotionalWeight:     clampUnitFloat(req.EmotionalWeight),
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "ok",
		"item":   item,
		"policy": protagonistEntityMemoryPolicy(),
	})
}

func (s *Server) handleListProtagonistEntityMemories(w http.ResponseWriter, r *http.Request) {
	st, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ownerEntityKey := strings.TrimSpace(r.URL.Query().Get("owner_entity_key"))
	personaEntityKey := strings.TrimSpace(r.URL.Query().Get("persona_entity_key"))
	if ownerEntityKey == "" {
		ownerEntityKey = personaEntityKey
	}
	items, err := s.listProtagonistEntityMemoriesByCanonicalOwner(r.Context(), st, store.ProtagonistEntityMemoryFilter{
		PersonaEntityKey:    personaEntityKey,
		OwnerEntityKey:      ownerEntityKey,
		OwnerEntityRole:     normalizeSubjectiveEntityRoleFilter(r.URL.Query().Get("owner_entity_role")),
		OwnerVisibility:     normalizeSubjectiveEntityVisibilityFilter(r.URL.Query().Get("owner_visibility")),
		SourceChatSessionID: strings.TrimSpace(r.URL.Query().Get("source_chat_session_id")),
		Limit:               limit,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  items,
		"count":  len(items),
		"policy": protagonistEntityMemoryPolicy(),
	})
}

func (s *Server) handlePatchProtagonistEntityMemory(w http.ResponseWriter, r *http.Request) {
	managementStore, ok := s.Store.(store.ProtagonistEntityMemoryManagementStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_management_store_not_enabled", "subjective entity memory management store is not enabled")
		return
	}
	memoryID, ok := subjectiveEntityMemoryPathID(w, r)
	if !ok {
		return
	}
	var req protagonistEntityMemoryUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.PersonaEntityKey = strings.TrimSpace(req.PersonaEntityKey)
	req.PersonaEntityName = strings.TrimSpace(req.PersonaEntityName)
	req.OwnerEntityKey = strings.TrimSpace(req.OwnerEntityKey)
	req.OwnerEntityName = strings.TrimSpace(req.OwnerEntityName)
	req.OwnerEntityRole = normalizeSubjectiveEntityRole(req.OwnerEntityRole)
	req.OwnerVisibility = normalizeSubjectiveEntityVisibility(req.OwnerVisibility)
	req.SourceCharacterName = strings.TrimSpace(req.SourceCharacterName)
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	req.MemoryText = strings.TrimSpace(req.MemoryText)
	if req.OwnerEntityKey == "" {
		req.OwnerEntityKey = req.PersonaEntityKey
	}
	if req.PersonaEntityKey == "" {
		req.PersonaEntityKey = req.OwnerEntityKey
	}
	if req.PersonaEntityKey == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "owner_entity_key is required")
		return
	}
	if req.OwnerEntityName == "" {
		req.OwnerEntityName = req.PersonaEntityName
	}
	if req.PersonaEntityName == "" {
		req.PersonaEntityName = req.OwnerEntityName
	}
	if req.PersonaEntityName == "" {
		req.PersonaEntityName = req.PersonaEntityKey
	}
	if req.OwnerEntityName == "" {
		req.OwnerEntityName = req.OwnerEntityKey
	}
	if req.MemoryText == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "memory_text is required")
		return
	}
	if req.SourceChatSessionID != "" {
		canonicalOwner := s.canonicalSubjectiveEntityOwner(r.Context(), req.SourceChatSessionID, req.OwnerEntityKey, req.OwnerEntityName)
		req.OwnerEntityKey = canonicalOwner.Key
		req.PersonaEntityKey = canonicalOwner.Key
		if canonicalOwner.Name != "" {
			req.OwnerEntityName = canonicalOwner.Name
			req.PersonaEntityName = canonicalOwner.Name
		}
		req.Tags = append(req.Tags, canonicalOwner.AliasTags...)
		if canonicalOwner.Changed {
			req.Tags = append(req.Tags, "entity_alias_canonicalized")
		}
	}
	tagsJSON := strings.TrimSpace(req.TagsJSON)
	if tagsJSON == "" {
		encoded, err := json.Marshal(req.Tags)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid tags")
			return
		}
		tagsJSON = string(encoded)
	} else if !json.Valid([]byte(tagsJSON)) {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "tags_json must be valid JSON")
		return
	}
	portability := strings.TrimSpace(req.Portability)
	if portability == "" {
		portability = "portable_persona_recollection"
	}
	update := store.ProtagonistEntityMemoryUpdate{
		ID:                  memoryID,
		PersonaEntityKey:    req.PersonaEntityKey,
		PersonaEntityName:   req.PersonaEntityName,
		OwnerEntityKey:      req.OwnerEntityKey,
		OwnerEntityName:     req.OwnerEntityName,
		OwnerEntityRole:     req.OwnerEntityRole,
		OwnerVisibility:     req.OwnerVisibility,
		SourceCharacterName: req.SourceCharacterName,
		MemoryText:          req.MemoryText,
		EvidenceExcerpt:     strings.TrimSpace(req.EvidenceExcerpt),
		SecretGuard:         req.SecretGuard,
		Portability:         portability,
		TargetRevealPolicy:  normalizeTargetRevealPolicy(req.TargetRevealPolicy),
		TagsJSON:            tagsJSON,
		Importance10:        clampPersonaImportance10(req.Importance10),
		EmotionalWeight:     clampUnitFloat(req.EmotionalWeight),
	}
	if err := managementStore.UpdateProtagonistEntityMemory(r.Context(), update); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"id":             memoryID,
		"updated_fields": []string{"owner_entity", "memory_text", "evidence_excerpt", "secret_guard", "portability", "target_reveal_policy", "importance_10", "emotional_weight", "tags_json"},
		"policy": map[string]any{
			"surface":              "subjective_entity_memory_manual_edit",
			"memory_text_mutation": true,
			"evidence_mutation":    true,
			"single_row_scope":     true,
		},
	})
}

func (s *Server) handleDeleteProtagonistEntityMemory(w http.ResponseWriter, r *http.Request) {
	managementStore, ok := s.Store.(store.ProtagonistEntityMemoryManagementStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_management_store_not_enabled", "subjective entity memory management store is not enabled")
		return
	}
	memoryID, ok := subjectiveEntityMemoryPathID(w, r)
	if !ok {
		return
	}
	if err := managementStore.DeleteProtagonistEntityMemory(r.Context(), memoryID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"id":     memoryID,
		"policy": map[string]any{
			"surface":          "subjective_entity_memory_manual_delete",
			"delete_duplicate": false,
			"single_row_scope": true,
		},
	})
}

func (s *Server) handleListSubjectiveEntityMemoryEntities(w http.ResponseWriter, r *http.Request) {
	st, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	sourceSID := strings.TrimSpace(r.URL.Query().Get("source_chat_session_id"))
	if sourceSID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	memories, err := st.ListProtagonistEntityMemories(r.Context(), store.ProtagonistEntityMemoryFilter{
		SourceChatSessionID: sourceSID,
		Limit:               200,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	items := s.subjectiveEntityMemoryGroups(r.Context(), sourceSID, memories)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  items,
		"count":  len(items),
		"policy": subjectiveEntityBundlePolicy(),
	})
}

func (s *Server) handleCreateSubjectiveEntityMemoryCapsule(w http.ResponseWriter, r *http.Request) {
	entityStore, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	capsuleStore, ok := s.personaCapsuleStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "persona_capsule_store_not_enabled", "persona capsule store is not enabled")
		return
	}
	var req subjectiveEntityMemoryCapsuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.OwnerEntityKey = strings.TrimSpace(req.OwnerEntityKey)
	req.OwnerEntityName = strings.TrimSpace(req.OwnerEntityName)
	req.OwnerEntityRole = normalizeSubjectiveEntityRoleFilter(req.OwnerEntityRole)
	req.OwnerVisibility = normalizeSubjectiveEntityVisibilityFilter(req.OwnerVisibility)
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	req.SourceCharacterName = strings.TrimSpace(req.SourceCharacterName)
	req.TargetRevealPolicy = normalizeTargetRevealPolicy(req.TargetRevealPolicy)
	if req.OwnerEntityKey == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "owner_entity_key is required")
		return
	}
	if req.SourceChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	canonicalOwner := s.canonicalSubjectiveEntityOwner(r.Context(), req.SourceChatSessionID, req.OwnerEntityKey, req.OwnerEntityName)
	req.OwnerEntityKey = canonicalOwner.Key
	if canonicalOwner.Name != "" && (req.OwnerEntityName != "" || canonicalOwner.Name != canonicalOwner.Key) {
		req.OwnerEntityName = canonicalOwner.Name
	}
	idSet := map[int64]bool{}
	for _, id := range req.MemoryIDs {
		if id > 0 {
			idSet[id] = true
		}
	}
	autoSelect := len(idSet) == 0
	memories, err := s.listProtagonistEntityMemoriesByCanonicalOwner(r.Context(), entityStore, store.ProtagonistEntityMemoryFilter{
		OwnerEntityKey:      req.OwnerEntityKey,
		OwnerEntityRole:     req.OwnerEntityRole,
		OwnerVisibility:     req.OwnerVisibility,
		SourceChatSessionID: req.SourceChatSessionID,
		Limit:               200,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	entries := make([]store.PersonaMemoryEntry, 0, len(req.MemoryIDs))
	sourceMemoryIDs := []int64{}
	for _, memory := range memories {
		if !autoSelect && !idSet[memory.ID] {
			continue
		}
		text := strings.TrimSpace(memory.MemoryText)
		if text == "" {
			continue
		}
		if req.OwnerEntityName == "" {
			req.OwnerEntityName = strings.TrimSpace(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName, req.OwnerEntityKey))
		}
		if req.OwnerEntityRole == "" {
			req.OwnerEntityRole = strings.TrimSpace(firstNonEmpty(memory.OwnerEntityRole, "protagonist"))
		}
		if req.OwnerVisibility == "" {
			req.OwnerVisibility = strings.TrimSpace(firstNonEmpty(memory.OwnerVisibility, "player_known"))
		}
		tagsJSON, err := json.Marshal(subjectiveEntityCapsuleTags(memory, req))
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid tags")
			return
		}
		entries = append(entries, store.PersonaMemoryEntry{
			SourceMemoryType: "subjective_entity_memory",
			SourceMemoryID:   memory.ID,
			SourceTurn:       memory.SourceTurn,
			MemoryText:       text,
			EmotionalWeight:  clampUnitFloat(memory.EmotionalWeight),
			Importance10:     clampPersonaImportance10(memory.Importance10),
			Portability:      subjectiveEntityCapsulePortability(memory, req),
			TagsJSON:         string(tagsJSON),
			EvidenceExcerpt:  strings.TrimSpace(memory.EvidenceExcerpt),
			InjectionPolicy:  subjectiveEntityCapsuleInjectionPolicy(memory, req),
		})
		sourceMemoryIDs = append(sourceMemoryIDs, memory.ID)
		if autoSelect && len(entries) >= 24 {
			break
		}
	}
	if len(entries) == 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "no matching subjective entity memories found for this owner/source scope")
		return
	}
	if req.OwnerEntityName == "" {
		req.OwnerEntityName = req.OwnerEntityKey
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = req.OwnerEntityName + " private recollection capsule"
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = subjectiveEntityCapsuleMode(req)
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = req.OwnerEntityName + " subjective recollections from " + req.SourceChatSessionID
	}
	capsule, err := capsuleStore.CreatePersonaMemoryCapsule(r.Context(), &store.PersonaMemoryCapsule{
		PersonaKey:          req.OwnerEntityKey,
		SourceChatSessionID: req.SourceChatSessionID,
		SourceCharacterName: firstNonEmpty(req.SourceCharacterName, req.OwnerEntityName),
		Title:               title,
		Mode:                mode,
		Summary:             summary,
	}, entries)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":            "ok",
		"capsule":           capsule,
		"entries_count":     len(entries),
		"source_memory_ids": sourceMemoryIDs,
		"policy":            subjectiveEntityCapsulePolicy(req),
	})
}

func (s *Server) handleRepairSubjectiveEntityMemoryAliases(w http.ResponseWriter, r *http.Request) {
	st, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	var req subjectiveEntityAliasRepairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	if req.SourceChatSessionID == "" {
		req.SourceChatSessionID = r.URL.Query().Get("source_chat_session_id")
	}
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	if req.SourceChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	if v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("apply"))); v == "true" || v == "1" || v == "yes" {
		req.Apply = true
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 1000 {
		limit = 1000
	}
	memories, err := st.ListProtagonistEntityMemories(r.Context(), store.ProtagonistEntityMemoryFilter{
		SourceChatSessionID: req.SourceChatSessionID,
		Limit:               limit,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	plan := s.buildSubjectiveEntityAliasRepairPlan(r.Context(), req.SourceChatSessionID, memories)
	updated := 0
	updateErrors := []string{}
	if req.Apply && len(plan.Changes) > 0 {
		repairStore, ok := s.Store.(store.ProtagonistEntityMemoryRepairStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "subjective_entity_alias_repair_store_not_enabled", "subjective entity alias repair store is not enabled")
			return
		}
		for _, change := range plan.Changes {
			err := repairStore.UpdateProtagonistEntityMemoryOwner(r.Context(), store.ProtagonistEntityMemoryOwnerUpdate{
				ID:                change.ID,
				PersonaEntityKey:  change.ToOwnerKey,
				PersonaEntityName: change.ToOwnerName,
				OwnerEntityKey:    change.ToOwnerKey,
				OwnerEntityName:   change.ToOwnerName,
				TagsJSON:          change.TargetTagsJSON,
			})
			if err != nil {
				updateErrors = append(updateErrors, err.Error())
				continue
			}
			updated++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"source_chat_session_id": req.SourceChatSessionID,
		"dry_run_only":           !req.Apply,
		"apply":                  req.Apply,
		"scanned":                plan.Scanned,
		"alias_group_count":      len(plan.Groups),
		"repairable_count":       len(plan.Changes),
		"updated_count":          updated,
		"errors":                 updateErrors,
		"groups":                 plan.Groups,
		"policy":                 subjectiveEntityAliasRepairPolicy(),
	})
}

func (s *Server) handleForceMergeSubjectiveEntityMemories(w http.ResponseWriter, r *http.Request) {
	st, ok := s.protagonistEntityMemoryStore()
	if !ok {
		writeError(w, http.StatusNotImplemented, "protagonist_entity_memory_store_not_enabled", "protagonist entity memory store is not enabled")
		return
	}
	repairStore, ok := s.Store.(store.ProtagonistEntityMemoryRepairStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "subjective_entity_force_merge_store_not_enabled", "subjective entity force merge store is not enabled")
		return
	}
	var req subjectiveEntityForceMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	req.SourceChatSessionID = strings.TrimSpace(req.SourceChatSessionID)
	req.TargetOwnerKey = strings.TrimSpace(req.TargetOwnerKey)
	req.TargetOwnerName = strings.TrimSpace(req.TargetOwnerName)
	req.TargetOwnerRole = normalizeSubjectiveEntityRole(req.TargetOwnerRole)
	req.TargetVisibility = normalizeSubjectiveEntityVisibility(req.TargetVisibility)
	if req.SourceChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_chat_session_id is required")
		return
	}
	if req.TargetOwnerKey == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "target_owner_key is required")
		return
	}
	canonicalTarget := s.canonicalSubjectiveEntityOwner(r.Context(), req.SourceChatSessionID, req.TargetOwnerKey, req.TargetOwnerName)
	req.TargetOwnerKey = canonicalTarget.Key
	if req.TargetOwnerName == "" || canonicalTarget.Changed {
		req.TargetOwnerName = strings.TrimSpace(firstNonEmpty(canonicalTarget.Name, req.TargetOwnerName, req.TargetOwnerKey))
	}
	if req.TargetOwnerRole == "" {
		req.TargetOwnerRole = "protagonist"
	}
	if req.TargetVisibility == "" {
		req.TargetVisibility = "player_known"
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	if limit > 1000 {
		limit = 1000
	}
	memories, err := st.ListProtagonistEntityMemories(r.Context(), store.ProtagonistEntityMemoryFilter{
		SourceChatSessionID: req.SourceChatSessionID,
		Limit:               limit,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	sourceKeys := map[string]bool{}
	for _, key := range req.SourceOwnerKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			sourceKeys[key] = true
		}
	}
	sourceIDs := map[int64]bool{}
	for _, id := range req.MemoryIDs {
		if id > 0 {
			sourceIDs[id] = true
		}
	}
	if len(sourceKeys) == 0 && len(sourceIDs) == 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "source_owner_keys or memory_ids is required")
		return
	}
	updated := 0
	updateErrors := []string{}
	changedIDs := []int64{}
	for _, memory := range memories {
		ownerKey := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey))
		selected := (memory.ID > 0 && sourceIDs[memory.ID]) || sourceKeys[ownerKey]
		if !selected {
			continue
		}
		if memory.ID <= 0 {
			continue
		}
		tagsJSON := subjectiveEntityForceMergeTags(memory, req)
		err := repairStore.UpdateProtagonistEntityMemoryOwner(r.Context(), store.ProtagonistEntityMemoryOwnerUpdate{
			ID:                memory.ID,
			PersonaEntityKey:  req.TargetOwnerKey,
			PersonaEntityName: req.TargetOwnerName,
			OwnerEntityKey:    req.TargetOwnerKey,
			OwnerEntityName:   req.TargetOwnerName,
			OwnerEntityRole:   req.TargetOwnerRole,
			OwnerVisibility:   req.TargetVisibility,
			TagsJSON:          tagsJSON,
		})
		if err != nil {
			updateErrors = append(updateErrors, err.Error())
			continue
		}
		updated++
		changedIDs = append(changedIDs, memory.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"source_chat_session_id": req.SourceChatSessionID,
		"target_owner_key":       req.TargetOwnerKey,
		"target_owner_name":      req.TargetOwnerName,
		"target_owner_role":      req.TargetOwnerRole,
		"target_visibility":      req.TargetVisibility,
		"matched_count":          len(changedIDs),
		"updated_count":          updated,
		"updated_ids":            changedIDs,
		"errors":                 updateErrors,
		"policy":                 subjectiveEntityForceMergePolicy(),
	})
}

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
		tagsJSON, err := json.Marshal(raw.Tags)
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
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := st.ListAttachedPersonaMemoryEntries(r.Context(), targetSID, limit)
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

type subjectiveEntityAliasRepairPlan struct {
	Scanned int                                 `json:"scanned"`
	Groups  []subjectiveEntityAliasRepairGroup  `json:"groups"`
	Changes []subjectiveEntityAliasRepairChange `json:"changes"`
}

type subjectiveEntityAliasRepairGroup struct {
	CanonicalOwnerKey  string                              `json:"canonical_owner_key"`
	CanonicalOwnerName string                              `json:"canonical_owner_name"`
	OwnerEntityRole    string                              `json:"owner_entity_role"`
	OwnerVisibility    string                              `json:"owner_visibility"`
	MemoryCount        int                                 `json:"memory_count"`
	RepairableCount    int                                 `json:"repairable_count"`
	Aliases            []subjectiveEntityAliasRepairAlias  `json:"aliases"`
	Changes            []subjectiveEntityAliasRepairChange `json:"changes"`
}

type subjectiveEntityAliasRepairAlias struct {
	OwnerEntityKey  string  `json:"owner_entity_key"`
	OwnerEntityName string  `json:"owner_entity_name"`
	Count           int     `json:"count"`
	MemoryIDs       []int64 `json:"memory_ids"`
}

type subjectiveEntityAliasRepairChange struct {
	ID             int64  `json:"id"`
	FromOwnerKey   string `json:"from_owner_key"`
	FromOwnerName  string `json:"from_owner_name"`
	ToOwnerKey     string `json:"to_owner_key"`
	ToOwnerName    string `json:"to_owner_name"`
	SourceTurn     int    `json:"source_turn_index"`
	TargetTagsJSON string `json:"-"`
}

func (s *Server) buildSubjectiveEntityAliasRepairPlan(ctx context.Context, sourceSID string, memories []store.ProtagonistEntityMemory) subjectiveEntityAliasRepairPlan {
	type groupState struct {
		subjectiveEntityAliasRepairGroup
		aliasIndex map[string]int
	}
	plan := subjectiveEntityAliasRepairPlan{Scanned: len(memories)}
	order := []string{}
	groups := map[string]*groupState{}
	for _, memory := range memories {
		originalOwnerKey := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey))
		originalOwnerName := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName, originalOwnerKey))
		ownerRole := strings.TrimSpace(memory.OwnerEntityRole)
		if ownerRole == "" {
			ownerRole = "protagonist"
		}
		ownerVisibility := strings.TrimSpace(memory.OwnerVisibility)
		if ownerVisibility == "" {
			ownerVisibility = "player_known"
		}
		canonicalMemory := s.canonicalizeSubjectiveEntityMemoryForRead(ctx, sourceSID, memory)
		canonicalKey := strings.TrimSpace(firstNonEmpty(canonicalMemory.OwnerEntityKey, canonicalMemory.PersonaEntityKey))
		if canonicalKey == "" {
			continue
		}
		canonicalName := strings.TrimSpace(firstNonEmpty(canonicalMemory.OwnerEntityName, canonicalMemory.PersonaEntityName, canonicalKey))
		groupKey := canonicalKey + "\x1f" + ownerRole + "\x1f" + ownerVisibility
		group := groups[groupKey]
		if group == nil {
			group = &groupState{
				subjectiveEntityAliasRepairGroup: subjectiveEntityAliasRepairGroup{
					CanonicalOwnerKey:  canonicalKey,
					CanonicalOwnerName: canonicalName,
					OwnerEntityRole:    ownerRole,
					OwnerVisibility:    ownerVisibility,
				},
				aliasIndex: map[string]int{},
			}
			groups[groupKey] = group
			order = append(order, groupKey)
		}
		group.MemoryCount++
		aliasKey := originalOwnerKey + "\x1f" + originalOwnerName
		if idx, ok := group.aliasIndex[aliasKey]; ok {
			group.Aliases[idx].Count++
			if memory.ID > 0 {
				group.Aliases[idx].MemoryIDs = append(group.Aliases[idx].MemoryIDs, memory.ID)
			}
		} else {
			group.aliasIndex[aliasKey] = len(group.Aliases)
			ids := []int64{}
			if memory.ID > 0 {
				ids = append(ids, memory.ID)
			}
			group.Aliases = append(group.Aliases, subjectiveEntityAliasRepairAlias{
				OwnerEntityKey:  originalOwnerKey,
				OwnerEntityName: originalOwnerName,
				Count:           1,
				MemoryIDs:       ids,
			})
		}
		if !subjectiveEntityMemoryOwnerNeedsRepair(memory, canonicalMemory) {
			continue
		}
		change := subjectiveEntityAliasRepairChange{
			ID:             memory.ID,
			FromOwnerKey:   originalOwnerKey,
			FromOwnerName:  originalOwnerName,
			ToOwnerKey:     canonicalKey,
			ToOwnerName:    canonicalName,
			SourceTurn:     memory.SourceTurn,
			TargetTagsJSON: subjectiveEntityAliasRepairTags(memory, canonicalMemory),
		}
		group.RepairableCount++
		group.Changes = append(group.Changes, change)
		plan.Changes = append(plan.Changes, change)
	}
	for _, key := range order {
		group := groups[key]
		if group.RepairableCount == 0 && len(group.Aliases) <= 1 {
			continue
		}
		plan.Groups = append(plan.Groups, group.subjectiveEntityAliasRepairGroup)
	}
	return plan
}

func subjectiveEntityMemoryOwnerNeedsRepair(before, after store.ProtagonistEntityMemory) bool {
	return strings.TrimSpace(firstNonEmpty(before.OwnerEntityKey, before.PersonaEntityKey)) != strings.TrimSpace(after.OwnerEntityKey) ||
		strings.TrimSpace(firstNonEmpty(before.PersonaEntityKey, before.OwnerEntityKey)) != strings.TrimSpace(after.PersonaEntityKey) ||
		strings.TrimSpace(firstNonEmpty(before.OwnerEntityName, before.PersonaEntityName)) != strings.TrimSpace(after.OwnerEntityName) ||
		strings.TrimSpace(firstNonEmpty(before.PersonaEntityName, before.OwnerEntityName)) != strings.TrimSpace(after.PersonaEntityName)
}

func subjectiveEntityAliasRepairTags(before, after store.ProtagonistEntityMemory) string {
	seen := map[string]bool{}
	tags := []string{}
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	var existing []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(before.TagsJSON)), &existing); err == nil {
		for _, tag := range existing {
			add(tag)
		}
	}
	beforeKey := strings.TrimSpace(firstNonEmpty(before.OwnerEntityKey, before.PersonaEntityKey))
	beforeName := strings.TrimSpace(firstNonEmpty(before.OwnerEntityName, before.PersonaEntityName, beforeKey))
	afterKey := strings.TrimSpace(after.OwnerEntityKey)
	afterName := strings.TrimSpace(after.OwnerEntityName)
	add("subjective_entity_memory")
	add("entity_alias_repaired")
	add("owner_entity_key:" + afterKey)
	add("owner_entity_name:" + afterName)
	if beforeKey != "" && beforeKey != afterKey {
		add("owner_entity_alias_key:" + beforeKey)
		add("raw_owner_entity_key:" + beforeKey)
	}
	if beforeName != "" && beforeName != afterName {
		add("owner_entity_alias:" + beforeName)
		add("raw_owner_entity_name:" + beforeName)
	}
	return mustCompactJSON(tags)
}

func subjectiveEntityAliasRepairPolicy() map[string]any {
	return map[string]any{
		"surface":                 "subjective_entity_alias_repair",
		"default_mode":            "dry_run",
		"apply_requires_explicit": true,
		"mutation_scope":          "owner_persona_identity_fields_only",
		"memory_text_mutation":    false,
		"evidence_mutation":       false,
		"delete_duplicate_rows":   false,
		"merge_mode":              "canonical_owner_key_rewrite_no_delete",
	}
}

func subjectiveEntityForceMergeTags(before store.ProtagonistEntityMemory, req subjectiveEntityForceMergeRequest) string {
	seen := map[string]bool{}
	tags := []string{}
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	var existing []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(before.TagsJSON)), &existing); err == nil {
		for _, tag := range existing {
			add(tag)
		}
	}
	beforeKey := strings.TrimSpace(firstNonEmpty(before.OwnerEntityKey, before.PersonaEntityKey))
	beforeName := strings.TrimSpace(firstNonEmpty(before.OwnerEntityName, before.PersonaEntityName, beforeKey))
	add("subjective_entity_memory")
	add("entity_force_merged")
	add("owner_entity_key:" + req.TargetOwnerKey)
	add("owner_entity_name:" + req.TargetOwnerName)
	add("owner_entity_role:" + req.TargetOwnerRole)
	add("owner_visibility:" + req.TargetVisibility)
	if beforeKey != "" && beforeKey != req.TargetOwnerKey {
		add("force_merge_source_owner_key:" + beforeKey)
	}
	if beforeName != "" && beforeName != req.TargetOwnerName {
		add("force_merge_source_owner_name:" + beforeName)
	}
	if before.OwnerEntityRole != "" && before.OwnerEntityRole != req.TargetOwnerRole {
		add("force_merge_source_owner_role:" + before.OwnerEntityRole)
	}
	if before.OwnerVisibility != "" && before.OwnerVisibility != req.TargetVisibility {
		add("force_merge_source_visibility:" + before.OwnerVisibility)
	}
	return mustCompactJSON(tags)
}

func subjectiveEntityForceMergePolicy() map[string]any {
	return map[string]any{
		"surface":                 "subjective_entity_force_merge",
		"apply_requires_explicit": true,
		"mutation_scope":          "selected_memory_owner_persona_role_visibility_fields_only",
		"memory_text_mutation":    false,
		"evidence_mutation":       false,
		"delete_duplicate_rows":   false,
		"merge_mode":              "manual_selected_owner_rewrite_no_delete",
	}
}

func (s *Server) listProtagonistEntityMemoriesByCanonicalOwner(ctx context.Context, st store.ProtagonistEntityMemoryStore, filter store.ProtagonistEntityMemoryFilter) ([]store.ProtagonistEntityMemory, error) {
	requestedOwner := strings.TrimSpace(firstNonEmpty(filter.OwnerEntityKey, filter.PersonaEntityKey))
	if requestedOwner == "" {
		items, err := st.ListProtagonistEntityMemories(ctx, filter)
		if err != nil {
			return nil, err
		}
		return s.canonicalizeSubjectiveEntityMemoriesForRead(ctx, filter.SourceChatSessionID, items), nil
	}
	canonicalOwner := s.canonicalSubjectiveEntityOwner(ctx, filter.SourceChatSessionID, requestedOwner, requestedOwner)
	filter.OwnerEntityKey = canonicalOwner.Key
	filter.PersonaEntityKey = ""
	items, err := st.ListProtagonistEntityMemories(ctx, filter)
	if err != nil {
		return nil, err
	}
	if filter.SourceChatSessionID == "" {
		return s.canonicalizeSubjectiveEntityMemoriesForRead(ctx, filter.SourceChatSessionID, items), nil
	}
	broadFilter := filter
	broadFilter.OwnerEntityKey = ""
	broadFilter.PersonaEntityKey = ""
	if broadFilter.Limit <= 0 || broadFilter.Limit < 200 {
		broadFilter.Limit = 200
	}
	broad, err := st.ListProtagonistEntityMemories(ctx, broadFilter)
	if err != nil {
		return s.canonicalizeSubjectiveEntityMemoriesForRead(ctx, filter.SourceChatSessionID, items), nil
	}
	seen := map[int64]bool{}
	out := make([]store.ProtagonistEntityMemory, 0, len(items)+len(broad))
	add := func(memory store.ProtagonistEntityMemory) {
		if memory.ID > 0 {
			if seen[memory.ID] {
				return
			}
			seen[memory.ID] = true
		}
		out = append(out, memory)
	}
	for _, memory := range items {
		add(memory)
	}
	for _, memory := range broad {
		canonicalMemory := s.canonicalizeSubjectiveEntityMemoryForRead(ctx, filter.SourceChatSessionID, memory)
		if canonicalMemory.OwnerEntityKey != canonicalOwner.Key {
			continue
		}
		add(memory)
	}
	return s.canonicalizeSubjectiveEntityMemoriesForRead(ctx, filter.SourceChatSessionID, out), nil
}

func (s *Server) subjectiveEntityMemoryGroups(ctx context.Context, sourceSID string, memories []store.ProtagonistEntityMemory) []map[string]any {
	type groupState struct {
		ownerKey        string
		ownerName       string
		ownerRole       string
		ownerVisibility string
		count           int
		latestTurn      int
		latestText      string
		maxImportance   float64
	}
	order := []string{}
	groups := map[string]*groupState{}
	for _, memory := range memories {
		memory = s.canonicalizeSubjectiveEntityMemoryForRead(ctx, sourceSID, memory)
		ownerKey := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey))
		if ownerKey == "" {
			ownerKey = "unknown"
		}
		ownerRole := strings.TrimSpace(memory.OwnerEntityRole)
		if ownerRole == "" {
			ownerRole = "protagonist"
		}
		ownerVisibility := strings.TrimSpace(memory.OwnerVisibility)
		if ownerVisibility == "" {
			ownerVisibility = "player_known"
		}
		key := ownerKey + "\x1f" + ownerRole + "\x1f" + ownerVisibility
		group := groups[key]
		if group == nil {
			ownerName := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName, ownerKey))
			group = &groupState{
				ownerKey:        ownerKey,
				ownerName:       ownerName,
				ownerRole:       ownerRole,
				ownerVisibility: ownerVisibility,
				latestTurn:      memory.SourceTurn,
				latestText:      truncateRunes(strings.TrimSpace(memory.MemoryText), 180),
				maxImportance:   memory.Importance10,
			}
			groups[key] = group
			order = append(order, key)
		}
		group.count++
		if memory.SourceTurn > group.latestTurn {
			group.latestTurn = memory.SourceTurn
		}
		if group.latestText == "" {
			group.latestText = truncateRunes(strings.TrimSpace(memory.MemoryText), 180)
		}
		if memory.Importance10 > group.maxImportance {
			group.maxImportance = memory.Importance10
		}
	}
	items := make([]map[string]any, 0, len(order))
	for _, key := range order {
		group := groups[key]
		npcPrivate := group.ownerRole == "npc" || group.ownerVisibility == "owner_private"
		revealPolicy := "requires_explicit_attachment"
		lane := "persona_recollection"
		if npcPrivate {
			revealPolicy = "owner_private_until_revealed"
			lane = "character_private_recollection"
		}
		items = append(items, map[string]any{
			"owner_entity_key":       group.ownerKey,
			"owner_entity_name":      group.ownerName,
			"owner_entity_role":      group.ownerRole,
			"owner_visibility":       group.ownerVisibility,
			"source_chat_session_id": sourceSID,
			"memory_count":           group.count,
			"latest_turn_index":      group.latestTurn,
			"latest_memory_text":     group.latestText,
			"max_importance_10":      group.maxImportance,
			"default_reveal_policy":  revealPolicy,
			"default_prepare_lane":   lane,
			"secret_guard_required":  npcPrivate,
		})
	}
	return items
}

func subjectiveEntityBundlePolicy() map[string]any {
	return map[string]any{
		"surface":                         "subjective_entity_memory_bundle_index",
		"unit":                            "source_session_entity_memory_bank",
		"user_selects":                    "entity_bundle",
		"memory_id_selection_required":    false,
		"auto_capsule_entry_limit":        24,
		"truth_authority":                 false,
		"canonical_write":                 false,
		"requires_explicit_attachment":    true,
		"npc_private_lane":                "character_private_recollection",
		"protagonist_support_lane":        "persona_recollection",
		"later_long_memory_compatible":    true,
		"per_entity_emotional_divergence": true,
	}
}

func subjectiveEntityCapsuleTags(memory store.ProtagonistEntityMemory, req subjectiveEntityMemoryCapsuleRequest) []string {
	seen := map[string]bool{}
	tags := []string{}
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	var existing []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(memory.TagsJSON)), &existing); err == nil {
		for _, tag := range existing {
			add(tag)
		}
	}
	add("subjective_entity_memory")
	add("owner_entity_key:" + req.OwnerEntityKey)
	add("owner_entity_name:" + req.OwnerEntityName)
	add("owner_entity_role:" + req.OwnerEntityRole)
	add("owner_visibility:" + req.OwnerVisibility)
	add("source_chat_session_id:" + req.SourceChatSessionID)
	add("target_reveal_policy:" + req.TargetRevealPolicy)
	add("entity_memory_id:" + strconv.FormatInt(memory.ID, 10))
	if req.OwnerEntityRole == "npc" || req.OwnerVisibility == "owner_private" {
		add("npc_private")
		add("character_private_recollection")
	}
	if memory.SecretGuard {
		add("secret_guard")
	}
	return tags
}

func subjectiveEntityCapsuleMode(req subjectiveEntityMemoryCapsuleRequest) string {
	if req.OwnerEntityRole == "npc" || req.OwnerVisibility == "owner_private" {
		return "npc_private_recollection"
	}
	return "subjective_entity_recollection"
}

func subjectiveEntityCapsulePortability(memory store.ProtagonistEntityMemory, req subjectiveEntityMemoryCapsuleRequest) string {
	if req.OwnerEntityRole == "npc" || req.OwnerVisibility == "owner_private" {
		return "npc_private_recollection"
	}
	if portability := strings.TrimSpace(memory.Portability); portability != "" {
		return portability
	}
	return "portable_subjective_entity_recollection"
}

func subjectiveEntityCapsuleInjectionPolicy(memory store.ProtagonistEntityMemory, req subjectiveEntityMemoryCapsuleRequest) string {
	if req.OwnerEntityRole == "npc" || req.OwnerVisibility == "owner_private" {
		return "support_only_npc_private_recollection"
	}
	if policy := strings.TrimSpace(memory.Portability); strings.Contains(policy, "npc_private") {
		return "support_only_npc_private_recollection"
	}
	return "support_only_persona_recollection"
}

func subjectiveEntityCapsulePolicy(req subjectiveEntityMemoryCapsuleRequest) map[string]any {
	npcPrivate := req.OwnerEntityRole == "npc" || req.OwnerVisibility == "owner_private"
	lane := "persona_recollection"
	authority := "support_only_persona_recollection"
	if npcPrivate {
		lane = "character_private_recollection"
		authority = "support_only_npc_private_recollection"
	}
	return map[string]any{
		"surface":                               "subjective_entity_memory_capsule",
		"authority":                             authority,
		"allowed_prepare_turn_lane":             lane,
		"owner_entity_key":                      req.OwnerEntityKey,
		"owner_entity_role":                     req.OwnerEntityRole,
		"owner_visibility":                      req.OwnerVisibility,
		"target_reveal_policy":                  req.TargetRevealPolicy,
		"truth_authority":                       false,
		"canonical_write":                       false,
		"current_world_fact":                    false,
		"visible_to_player":                     !npcPrivate,
		"narrator_reveal_blocked":               npcPrivate,
		"requires_explicit_attachment":          true,
		"requires_current_session_confirmation": true,
		"entry_reference_type":                  "subjective_entity_memory",
		"snapshot_fallback_required":            true,
	}
}

func personaCapsulePathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(r.PathValue("capsule_id")), 10, 64)
}

func subjectiveEntityMemoryPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("memory_id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "memory_id must be a positive integer")
		return 0, false
	}
	return id, true
}

func personaCapsuleSupportPolicy() map[string]any {
	return map[string]any{
		"surface":                    "persona_memory_capsule",
		"authority":                  "support_only_persona_recollection",
		"canonical_write":            false,
		"current_world_truth":        false,
		"allowed_prepare_turn_lane":  "persona_recollection",
		"requires_target_attachment": true,
		"legacy_snapshot_entries":    true,
		"memory_reference_entries":   true,
	}
}

func protagonistEntityMemoryPolicy() map[string]any {
	return map[string]any{
		"surface":                         "subjective_entity_memory_bank",
		"legacy_surface":                  "protagonist_entity_memory_bank",
		"authority":                       "entity_subjective_memory",
		"canonical_world_truth":           false,
		"current_world_fact":              false,
		"capsule_source":                  true,
		"requires_explicit_attachment":    true,
		"default_scope":                   "source_chat_session_id",
		"owner_separation_required":       true,
		"default_owner_visibility":        "player_known",
		"npc_private_lane":                "character_private_recollection",
		"npc_private_default_player_view": false,
	}
}

func normalizeSubjectiveEntityRole(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "npc", "supporting_character", "unknown":
		return strings.ToLower(strings.TrimSpace(raw))
	case "player", "persona", "protagonist":
		return "protagonist"
	default:
		return "protagonist"
	}
}

func normalizeSubjectiveEntityRoleFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "protagonist", "player", "persona":
		return "protagonist"
	case "npc", "supporting_character", "unknown":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeSubjectiveEntityVisibility(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner_private", "narrator_private", "admin_only":
		return strings.ToLower(strings.TrimSpace(raw))
	case "player_known":
		return "player_known"
	default:
		return "player_known"
	}
}

func normalizeSubjectiveEntityVisibilityFilter(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "player_known", "owner_private", "narrator_private", "admin_only":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeTargetRevealPolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner_private_until_revealed", "explicit_user_reveal_required", "current_session_confirmation_required", "explicit_reveal_event_required", "user_directed_reveal_only":
		return strings.ToLower(strings.TrimSpace(raw))
	case "requires_explicit_attachment":
		return "requires_explicit_attachment"
	default:
		return "requires_explicit_attachment"
	}
}

func clampPersonaImportance10(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 10 {
		return 10
	}
	return v
}

func clampUnitFloat(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
