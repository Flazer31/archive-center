package httpapi

import (
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
	ownerEntityName := strings.TrimSpace(r.URL.Query().Get("owner_entity_name"))
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
	if ownerEntityName != "" {
		exactName := comparableEntityKey(ownerEntityName)
		filtered := make([]store.ProtagonistEntityMemory, 0, len(items))
		for _, item := range items {
			itemName := strings.TrimSpace(firstNonEmpty(item.OwnerEntityName, item.PersonaEntityName, item.OwnerEntityKey, item.PersonaEntityKey))
			if comparableEntityKey(itemName) == exactName {
				filtered = append(filtered, item)
			}
		}
		items = filtered
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
	if req.OwnerEntityName != "" {
		exactName := comparableEntityKey(req.OwnerEntityName)
		filtered := make([]store.ProtagonistEntityMemory, 0, len(memories))
		for _, memory := range memories {
			memoryName := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName, memory.OwnerEntityKey, memory.PersonaEntityKey))
			if comparableEntityKey(memoryName) == exactName {
				filtered = append(filtered, memory)
			}
		}
		memories = filtered
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
		"review_required_count":  plan.ReviewRequiredCount,
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
