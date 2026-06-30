package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// registerCanonicalRoutes mounts store-backed canonical read/write endpoints.
// These endpoints exercise the Store interface directly; in noop mode they are
// harmless, and in dual_shadow / mariadb_shadow mode they write to the shadow.
func (s *Server) registerCanonicalRoutes(mux *http.ServeMux) {
	// R1 read endpoints
	mux.HandleFunc("GET /canonical/{chat_session_id}/chat-logs", s.handleCanonicalListChatLogs)
	mux.HandleFunc("GET /canonical/{chat_session_id}/effective-inputs", s.handleCanonicalGetEffectiveInput)
	mux.HandleFunc("GET /canonical/{chat_session_id}/memories", s.handleCanonicalListMemories)
	mux.HandleFunc("GET /canonical/{chat_session_id}/evidence", s.handleCanonicalListEvidence)
	mux.HandleFunc("GET /canonical/{chat_session_id}/kg-triples", s.handleCanonicalListKGTriples)
	mux.HandleFunc("GET /canonical/{chat_session_id}/audit-logs", s.handleCanonicalListAuditLogs)
	mux.HandleFunc("GET /canonical/{chat_session_id}/critic-feedback", s.handleCanonicalListCriticFeedback)
	mux.HandleFunc("GET /canonical/{chat_session_id}/character-events", s.handleCanonicalListCharacterEvents)

	// R2 shadow write endpoints (no authority switch; source is always "shadow")
	mux.HandleFunc("POST /canonical/{chat_session_id}/chat-logs", s.handleCanonicalSaveChatLog)
	mux.HandleFunc("POST /canonical/{chat_session_id}/effective-inputs", s.handleCanonicalSaveEffectiveInput)
	mux.HandleFunc("POST /canonical/{chat_session_id}/memories", s.handleCanonicalSaveMemory)
	mux.HandleFunc("POST /canonical/{chat_session_id}/evidence", s.handleCanonicalSaveEvidence)
	mux.HandleFunc("POST /canonical/{chat_session_id}/kg-triples", s.handleCanonicalSaveKGTriple)
	mux.HandleFunc("POST /canonical/{chat_session_id}/audit-logs", s.handleCanonicalSaveAuditLog)
	mux.HandleFunc("POST /canonical/{chat_session_id}/critic-feedback", s.handleCanonicalSaveCriticFeedback)
	mux.HandleFunc("POST /canonical/{chat_session_id}/character-events", s.handleCanonicalSaveCharacterEvent)
}

// GET /canonical/{chat_session_id}/chat-logs
func (s *Server) handleCanonicalListChatLogs(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))

	items, err := s.Store.ListChatLogs(r.Context(), sid, fromTurn, toTurn)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/effective-inputs
func (s *Server) handleCanonicalGetEffectiveInput(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	turnIndex, _ := strconv.Atoi(r.URL.Query().Get("turn_index"))
	if turnIndex <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "turn_index is required")
		return
	}

	item, err := s.Store.GetEffectiveInput(r.Context(), sid, turnIndex)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"source":          "shadow",
				"chat_session_id": sid,
				"turn_index":      turnIndex,
				"found":           false,
			})
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"turn_index":      turnIndex,
		"found":           true,
		"item":            item,
	})
}

// GET /canonical/{chat_session_id}/memories
func (s *Server) handleCanonicalListMemories(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))

	items, err := s.Store.ListMemories(r.Context(), sid, fromTurn, toTurn)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/evidence
func (s *Server) handleCanonicalListEvidence(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	items, err := s.Store.ListEvidence(r.Context(), sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/kg-triples
func (s *Server) handleCanonicalListKGTriples(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	items, err := s.Store.ListKGTriples(r.Context(), sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/audit-logs
func (s *Server) handleCanonicalListAuditLogs(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	eventType := r.URL.Query().Get("event_type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	items, err := s.Store.ListAuditLogs(r.Context(), sid, eventType, limit)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"event_type":      eventType,
		"limit":           limit,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/critic-feedback
func (s *Server) handleCanonicalListCriticFeedback(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	targetType := r.URL.Query().Get("target_type")
	targetID, _ := strconv.ParseInt(r.URL.Query().Get("target_id"), 10, 64)

	items, err := s.Store.ListCriticFeedback(r.Context(), sid, targetType, targetID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"target_type":     targetType,
		"target_id":       targetID,
		"items":           items,
		"count":           len(items),
	})
}

// GET /canonical/{chat_session_id}/character-events
func (s *Server) handleCanonicalListCharacterEvents(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	characterName := r.URL.Query().Get("character_name")

	items, err := s.Store.ListCharacterEvents(r.Context(), sid, characterName)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"character_name":  characterName,
		"items":           items,
		"count":           len(items),
	})
}

// POST /canonical/{chat_session_id}/chat-logs
func (s *Server) handleCanonicalSaveChatLog(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		TurnIndex int    `json:"turn_index"`
		Role      string `json:"role"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	content := strings.TrimSpace(req.Content)
	if req.TurnIndex >= 0 && role != "" && content != "" {
		if existing, err := s.Store.ListChatLogs(r.Context(), sid, req.TurnIndex, req.TurnIndex); err == nil {
			for _, item := range existing {
				if item.ChatSessionID != sid || item.TurnIndex != req.TurnIndex {
					continue
				}
				if strings.ToLower(strings.TrimSpace(item.Role)) != role {
					continue
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"source":          "shadow",
					"chat_session_id": sid,
					"saved":           true,
					"deduped":         true,
					"conflict":        strings.TrimSpace(item.Content) != content,
					"error":           "",
				})
				return
			}
		}
	}

	log := &store.ChatLog{
		ChatSessionID: sid,
		TurnIndex:     req.TurnIndex,
		Role:          req.Role,
		Content:       req.Content,
	}

	err := s.Store.SaveChatLog(r.Context(), log)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/effective-inputs
func (s *Server) handleCanonicalSaveEffectiveInput(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		TurnIndex      int    `json:"turn_index"`
		EffectiveInput string `json:"effective_input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	input := &store.EffectiveInput{
		ChatSessionID:  sid,
		TurnIndex:      req.TurnIndex,
		EffectiveInput: req.EffectiveInput,
	}

	err := s.Store.SaveEffectiveInput(r.Context(), input)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/memories
func (s *Server) handleCanonicalSaveMemory(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		TurnIndex             int     `json:"turn_index"`
		SummaryJSON           string  `json:"summary_json"`
		Embedding             string  `json:"embedding"`
		EmbeddingModel        string  `json:"embedding_model"`
		Importance            float64 `json:"importance"`
		EmotionalBoost        float64 `json:"emotional_boost"`
		Evidence              string  `json:"evidence"`
		EmotionalIntensity    float64 `json:"emotional_intensity"`
		NarrativeSignificance float64 `json:"narrative_significance"`
		PlaceWing             string  `json:"place_wing"`
		PlaceRoom             string  `json:"place_room"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	mem := &store.Memory{
		ChatSessionID:         sid,
		TurnIndex:             req.TurnIndex,
		SummaryJSON:           req.SummaryJSON,
		Embedding:             req.Embedding,
		EmbeddingModel:        req.EmbeddingModel,
		Importance:            req.Importance,
		EmotionalBoost:        req.EmotionalBoost,
		Evidence:              req.Evidence,
		EmotionalIntensity:    req.EmotionalIntensity,
		NarrativeSignificance: req.NarrativeSignificance,
		PlaceWing:             req.PlaceWing,
		PlaceRoom:             req.PlaceRoom,
	}

	err := s.Store.SaveMemory(r.Context(), mem)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/evidence
func (s *Server) handleCanonicalSaveEvidence(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		EvidenceKind    string `json:"evidence_kind"`
		EvidenceText    string `json:"evidence_text"`
		SourceTurnStart int    `json:"source_turn_start"`
		SourceTurnEnd   int    `json:"source_turn_end"`
		TurnAnchor      int    `json:"turn_anchor"`
		SourceHash      string `json:"source_hash"`
		ArchiveState    string `json:"archive_state"`
		CaptureStage    string `json:"capture_stage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	ev := &store.DirectEvidence{
		ChatSessionID:   sid,
		EvidenceKind:    req.EvidenceKind,
		EvidenceText:    req.EvidenceText,
		SourceTurnStart: req.SourceTurnStart,
		SourceTurnEnd:   req.SourceTurnEnd,
		TurnAnchor:      req.TurnAnchor,
		SourceHash:      req.SourceHash,
		ArchiveState:    req.ArchiveState,
		CaptureStage:    req.CaptureStage,
	}

	err := s.Store.SaveEvidence(r.Context(), ev)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/kg-triples
func (s *Server) handleCanonicalSaveKGTriple(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		Subject    string `json:"subject"`
		Predicate  string `json:"predicate"`
		Object     string `json:"object"`
		ValidFrom  int    `json:"valid_from"`
		ValidTo    int    `json:"valid_to"`
		SourceTurn int    `json:"source_turn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	triple := &store.KGTriple{
		ChatSessionID: sid,
		Subject:       req.Subject,
		Predicate:     req.Predicate,
		Object:        req.Object,
		ValidFrom:     req.ValidFrom,
		ValidTo:       req.ValidTo,
		SourceTurn:    req.SourceTurn,
	}

	err := s.Store.SaveKGTriple(r.Context(), triple)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/audit-logs
func (s *Server) handleCanonicalSaveAuditLog(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		EventType   string `json:"event_type"`
		TargetType  string `json:"target_type"`
		TargetID    int64  `json:"target_id"`
		Summary     string `json:"summary"`
		DetailsJSON string `json:"details_json"`
		Source      string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	audit := &store.AuditLog{
		ChatSessionID: sid,
		EventType:     req.EventType,
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		Summary:       req.Summary,
		DetailsJSON:   req.DetailsJSON,
		Source:        req.Source,
	}

	err := s.Store.SaveAuditLog(r.Context(), audit)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/critic-feedback
func (s *Server) handleCanonicalSaveCriticFeedback(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		TargetType    string `json:"target_type"`
		TargetID      int64  `json:"target_id"`
		FeedbackValue string `json:"feedback_value"`
		FeedbackNote  string `json:"feedback_note"`
		Source        string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	feedback := &store.CriticFeedback{
		ChatSessionID: sid,
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		FeedbackValue: req.FeedbackValue,
		FeedbackNote:  req.FeedbackNote,
		Source:        req.Source,
	}

	err := s.Store.SaveCriticFeedback(r.Context(), feedback)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

// POST /canonical/{chat_session_id}/character-events
func (s *Server) handleCanonicalSaveCharacterEvent(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	if !s.usesShadowWriteStore() {
		writeCanonicalShadowWriteGuarded(w, sid)
		return
	}
	var req struct {
		CharacterName string `json:"character_name"`
		TurnIndex     int    `json:"turn_index"`
		EventType     string `json:"event_type"`
		DetailsJSON   string `json:"details_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error())
		return
	}

	event := &store.CharacterEvent{
		ChatSessionID: sid,
		CharacterName: req.CharacterName,
		TurnIndex:     req.TurnIndex,
		EventType:     req.EventType,
		DetailsJSON:   req.DetailsJSON,
	}

	err := s.Store.SaveCharacterEvent(r.Context(), event)
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": sid,
		"saved":           err == nil,
		"error":           errString(err),
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeCanonicalShadowWriteGuarded(w http.ResponseWriter, chatSessionID string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          "shadow",
		"chat_session_id": chatSessionID,
		"saved":           false,
		"error":           "shadow_write_store_not_configured",
	})
}
