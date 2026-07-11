package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// Explorer write: R2 guards

func (s *Server) handlePatchMemory(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
		return
	}
	memoryID, ok := parseExplorerPathID(w, r, "memory_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	mem, found, err := s.findMemoryForExplorerPatch(r.Context(), sid, memoryID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.MemoryExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	if raw, exists := fields["summary_json"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "summary_json")
		if !ok {
			return
		}
		if strings.TrimSpace(value) != "" && !json.Valid([]byte(value)) {
			writeBadRequest(w, "summary_json must be valid JSON")
			return
		}
		patch.SummaryJSON = &value
		updatedFields = append(updatedFields, "summary_json")
		updatedValues["summary_json"] = value
	}
	if raw, exists := fields["importance"]; exists && !isJSONNull(raw) {
		value, ok := rawFloatField(w, raw, "importance")
		if !ok {
			return
		}
		patch.Importance = &value
		updatedFields = append(updatedFields, "importance")
		updatedValues["importance"] = value
	}
	if raw, exists := fields["archive_wing"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "archive_wing")
		if !ok {
			return
		}
		patch.PlaceWing = &value
		updatedFields = append(updatedFields, "archive_wing")
		updatedValues["archive_wing"] = value
	}
	if raw, exists := fields["archive_room"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "archive_room")
		if !ok {
			return
		}
		patch.PlaceRoom = &value
		updatedFields = append(updatedFields, "archive_room")
		updatedValues["archive_room"] = value
	}
	if len(updatedFields) == 0 {
		writeBadRequest(w, "no supported memory fields to update")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateMemoryExplorerFields(r.Context(), sid, memoryID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "memory",
		TargetID:      memoryID,
		Summary:       "Explorer manual memory edit",
		DetailsJSON: mustCompactJSON(map[string]any{
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"summary_json": mem.SummaryJSON,
				"importance":   mem.Importance,
				"archive_wing": mem.PlaceWing,
				"archive_room": mem.PlaceRoom,
				"created_at":   mem.CreatedAt,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "memory",
		"target_id":        memoryID,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func (s *Server) handlePatchKGTriple(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
		return
	}
	tripleID, ok := parseExplorerPathID(w, r, "triple_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	triple, found, err := s.findKGTripleForExplorerPatch(r.Context(), sid, tripleID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.KGTripleExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	for _, key := range []string{"subject", "predicate", "object"} {
		raw, exists := fields[key]
		if !exists || isJSONNull(raw) {
			continue
		}
		value, ok := rawStringField(w, raw, key)
		if !ok {
			return
		}
		switch key {
		case "subject":
			patch.Subject = &value
		case "predicate":
			patch.Predicate = &value
		case "object":
			patch.Object = &value
		}
		updatedFields = append(updatedFields, key)
		updatedValues[key] = value
	}
	if raw, exists := fields["valid_from"]; exists {
		value, ok := rawOptionalIntField(w, raw, "valid_from")
		if !ok {
			return
		}
		patch.ValidFrom = value
		updatedFields = append(updatedFields, "valid_from")
		updatedValues["valid_from"] = optionalIntValueForJSON(value)
	}
	if raw, exists := fields["valid_to"]; exists {
		value, ok := rawOptionalIntField(w, raw, "valid_to")
		if !ok {
			return
		}
		patch.ValidTo = value
		updatedFields = append(updatedFields, "valid_to")
		updatedValues["valid_to"] = optionalIntValueForJSON(value)
	}
	if len(updatedFields) == 0 {
		writeBadRequest(w, "no supported KG fields to update")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateKGTripleExplorerFields(r.Context(), sid, tripleID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "kg_triple",
		TargetID:      tripleID,
		Summary:       "Explorer manual KG triple edit",
		DetailsJSON: mustCompactJSON(map[string]any{
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"subject":    triple.Subject,
				"predicate":  triple.Predicate,
				"object":     triple.Object,
				"valid_from": triple.ValidFrom,
				"valid_to":   triple.ValidTo,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "kg_triple",
		"target_id":        tripleID,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func (s *Server) handlePatchEvidenceEdit(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "edit")
}

func (s *Server) handlePatchEvidenceReview(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "review")
}

func (s *Server) handlePatchEvidenceRevalidate(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "revalidate")
}

func (s *Server) handlePatchEvidenceTombstone(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "tombstone")
}

func (s *Server) handlePatchEvidenceSupersede(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "supersede")
}

func (s *Server) handlePatchEvidenceTransition(w http.ResponseWriter, r *http.Request, action string) {
	endpoint := "PATCH /explorer/direct-evidence/{record_id}/" + action
	if action == "edit" {
		endpoint = "PATCH /explorer/direct-evidence/{record_id}"
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	recordID, ok := parseExplorerPathID(w, r, "record_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	evidence, found, err := s.findEvidenceForExplorerPatch(r.Context(), sid, recordID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.DirectEvidenceExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	switch action {
	case "edit":
		if raw, exists := fields["archive_state"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "archive_state")
			if !ok {
				return
			}
			patch.ArchiveState = &value
			updatedFields = append(updatedFields, "archive_state")
			updatedValues["archive_state"] = value
		}
		if raw, exists := fields["capture_verification"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "capture_verification")
			if !ok {
				return
			}
			patch.CaptureVerification = &value
			updatedFields = append(updatedFields, "capture_verification")
			updatedValues["capture_verification"] = value
		}
		if raw, exists := fields["committed_gate"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "committed_gate")
			if !ok {
				return
			}
			patch.CommittedGate = &value
			updatedFields = append(updatedFields, "committed_gate")
			updatedValues["committed_gate"] = value
		}
		if raw, exists := fields["repair_needed"]; exists && !isJSONNull(raw) {
			value, ok := rawBoolField(w, raw, "repair_needed")
			if !ok {
				return
			}
			patch.RepairNeeded = &value
			updatedFields = append(updatedFields, "repair_needed")
			updatedValues["repair_needed"] = value
		}
		if raw, exists := fields["tombstoned"]; exists && !isJSONNull(raw) {
			value, ok := rawBoolField(w, raw, "tombstoned")
			if !ok {
				return
			}
			patch.Tombstoned = &value
			updatedFields = append(updatedFields, "tombstoned")
			updatedValues["tombstoned"] = value
		}
		if raw, exists := fields["superseded_by_id"]; exists {
			value, ok := rawOptionalIntField(w, raw, "superseded_by_id")
			if !ok {
				return
			}
			patch.SupersededByID = value
			updatedFields = append(updatedFields, "superseded_by_id")
			updatedValues["superseded_by_id"] = optionalIntValueForJSON(value)
		}
		if len(updatedFields) == 0 {
			writeBadRequest(w, "at least one editable direct evidence field is required")
			return
		}
	case "review":
		raw, exists := fields["capture_verification"]
		if !exists || isJSONNull(raw) {
			writeBadRequest(w, "capture_verification is required")
			return
		}
		value, ok := rawStringField(w, raw, "capture_verification")
		if !ok {
			return
		}
		patch.CaptureVerification = &value
		updatedFields = append(updatedFields, "capture_verification")
		updatedValues["capture_verification"] = value
	case "revalidate":
		verification := "verified"
		state := "committed"
		gate := "manual_revalidate"
		repairNeeded := false
		patch.CaptureVerification = &verification
		patch.ArchiveState = &state
		patch.CommittedGate = &gate
		patch.RepairNeeded = &repairNeeded
		updatedFields = append(updatedFields, "capture_verification", "archive_state", "committed_gate", "repair_needed")
		updatedValues["capture_verification"] = verification
		updatedValues["archive_state"] = state
		updatedValues["committed_gate"] = gate
		updatedValues["repair_needed"] = repairNeeded
	case "tombstone":
		tombstoned := true
		state := "tombstoned"
		patch.Tombstoned = &tombstoned
		patch.ArchiveState = &state
		updatedFields = append(updatedFields, "tombstoned", "archive_state")
		updatedValues["tombstoned"] = tombstoned
		updatedValues["archive_state"] = state
	case "supersede":
		raw, exists := fields["superseded_by_id"]
		if !exists {
			writeBadRequest(w, "superseded_by_id is required")
			return
		}
		value, ok := rawOptionalIntField(w, raw, "superseded_by_id")
		if !ok {
			return
		}
		patch.SupersededByID = value
		updatedFields = append(updatedFields, "superseded_by_id")
		updatedValues["superseded_by_id"] = optionalIntValueForJSON(value)
	default:
		writeBadRequest(w, "unsupported evidence action")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateDirectEvidenceExplorerFields(r.Context(), sid, recordID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "direct_evidence",
		TargetID:      recordID,
		Summary:       "Explorer manual direct evidence " + action,
		DetailsJSON: mustCompactJSON(map[string]any{
			"action":         action,
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"archive_state":        evidence.ArchiveState,
				"capture_verification": evidence.CaptureVerification,
				"committed_gate":       evidence.CommittedGate,
				"repair_needed":        evidence.RepairNeeded,
				"tombstoned":           evidence.Tombstoned,
				"superseded_by_id":     evidence.SupersededByID,
			},
			"review_note": stringFromRawField(fields["review_note"]),
			"changed_at":  changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "direct_evidence",
		"target_id":        recordID,
		"action":           action,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func parseExplorerPathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue(name)), 10, 64)
	if err != nil || id <= 0 {
		writeBadRequest(w, name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeExplorerPatchRequest(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, string, bool) {
	var fields map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeBadRequest(w, err.Error())
		return nil, "", false
	}
	sidRaw, ok := fields["chat_session_id"]
	if !ok || isJSONNull(sidRaw) {
		writeBadRequest(w, "chat_session_id is required")
		return nil, "", false
	}
	sid, ok := rawStringField(w, sidRaw, "chat_session_id")
	if !ok {
		return nil, "", false
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return nil, "", false
	}
	return fields, sid, true
}

func rawStringField(w http.ResponseWriter, raw json.RawMessage, field string) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be a string")
		return "", false
	}
	return value, true
}

func rawBoolField(w http.ResponseWriter, raw json.RawMessage, field string) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be a boolean")
		return false, false
	}
	return value, true
}

func rawFloatField(w http.ResponseWriter, raw json.RawMessage, field string) (float64, bool) {
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		writeBadRequest(w, field+" must be a finite number")
		return 0, false
	}
	return value, true
}

func rawOptionalIntField(w http.ResponseWriter, raw json.RawMessage, field string) (store.OptionalIntPatch, bool) {
	out := store.OptionalIntPatch{Set: true}
	if isJSONNull(raw) {
		return out, true
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be an integer or null")
		return out, false
	}
	out.Value = &value
	return out, true
}

func optionalIntValueForJSON(value store.OptionalIntPatch) any {
	if value.Value == nil {
		return nil
	}
	return *value.Value
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func stringFromRawField(raw json.RawMessage) string {
	if len(raw) == 0 || isJSONNull(raw) {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func (s *Server) findMemoryForExplorerPatch(ctx context.Context, sid string, memoryID int64) (store.Memory, bool, error) {
	items, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		return store.Memory{}, false, err
	}
	for _, item := range items {
		if item.ID == memoryID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.Memory{}, false, nil
}

func (s *Server) findKGTripleForExplorerPatch(ctx context.Context, sid string, tripleID int64) (store.KGTriple, bool, error) {
	items, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil {
		return store.KGTriple{}, false, err
	}
	for _, item := range items {
		if item.ID == tripleID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.KGTriple{}, false, nil
}

func (s *Server) findEvidenceForExplorerPatch(ctx context.Context, sid string, recordID int64) (store.DirectEvidence, bool, error) {
	items, err := s.Store.ListEvidence(ctx, sid)
	if err != nil {
		return store.DirectEvidence{}, false, err
	}
	for _, item := range items {
		if item.ID == recordID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.DirectEvidence{}, false, nil
}

type explorerRegenerateMemoryRequest struct {
	ChatSessionID string         `json:"chat_session_id"`
	TurnIndex     int            `json:"turn_index"`
	ClientMeta    map[string]any `json:"client_meta"`
	DryRun        bool           `json:"dry_run"`
}

func (s *Server) handleRegenerateMemory(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /explorer/memories/regenerate")
		return
	}
	if s.Store == nil {
		writeInternalError(w, "store is not configured")
		return
	}
	var req explorerRegenerateMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.TurnIndex <= 0 {
		writeBadRequest(w, "turn_index is required")
		return
	}

	logs, err := s.Store.ListChatLogs(r.Context(), sid, req.TurnIndex, req.TurnIndex)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeInternalError(w, err.Error())
		return
	}
	roleMap := map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex != req.TurnIndex {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		roleMap[role] = appendUniqueTurnRoleText(roleMap[role], log.Content)
	}
	userText := sanitizeCriticStorageText(roleMap["user"])
	assistantText := sanitizeCriticStorageText(roleMap["assistant"])
	if strings.TrimSpace(assistantText) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "failed",
			"code":            "assistant_content_missing",
			"detail":          "no completed assistant output was found for this turn",
			"chat_session_id": sid,
			"turn_index":      req.TurnIndex,
			"source":          s.storeWriteSource(),
		})
		return
	}
	if shouldApplyCompleteTurnOOCGuard(userText, assistantText, nil) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "skipped",
			"code":            "ooc_guard",
			"detail":          "this turn was excluded from memory regeneration by the OOC guard",
			"chat_session_id": sid,
			"turn_index":      req.TurnIndex,
			"source":          s.storeWriteSource(),
		})
		return
	}
	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	if !extractionCfg.Critic.hasConfig() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "failed",
			"code":             "critic_config_missing",
			"detail":           "critic LLM configuration is incomplete",
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"llm_config_trace": llmTrace,
		})
		return
	}
	if req.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "ok",
			"code":             "dry_run_ready",
			"dry_run":          true,
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"llm_config_trace": llmTrace,
			"note":             "Explorer regenerate dry-run found a completed turn but did not call Critic or write artifacts",
		})
		return
	}

	extraction, criticTrace, err := s.runCompleteTurnCriticFromCanonicalLogs(r.Context(), sid, req.TurnIndex, userText, assistantText, extractionCfg.Critic)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "failed",
			"code":             "critic_extract_failed",
			"detail":           err.Error(),
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"critic_trace":     criticTrace,
			"llm_config_trace": llmTrace,
		})
		return
	}
	now := time.Now().UTC()
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
	saveResult := s.saveCriticExtractionArtifacts(r.Context(), sid, req.TurnIndex, extraction, content, extractionCfg.Embedder, now)
	status := "ok"
	code := "regenerated"
	if saveResult.Errors > 0 {
		status = "partial_error"
		code = "regenerated_with_warnings"
	}

	allLogs, _ := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
	allMemories, _ := s.Store.ListMemories(r.Context(), sid, 0, 0)
	allEvidence, _ := s.Store.ListEvidence(r.Context(), sid)
	targetTurns := map[int]bool{req.TurnIndex: true}
	episodeInterval := normalizedEpisodeInterval(intFromAny(req.ClientMeta["episode_interval_turns"], 0))
	episodeBackfill := s.backfillEpisodeSummariesFromChatLogs(r.Context(), sid, allLogs, allMemories, allEvidence, episodeInterval, false, targetTurns, true)
	worldRuleBackfill := s.backfillWorldRulesFromMemories(r.Context(), sid, allMemories, targetTurns, false)

	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "explorer_regenerate_memory",
		TargetType:    "turn",
		TargetID:      int64(req.TurnIndex),
		Summary:       fmt.Sprintf("Explorer regenerated derived artifacts for turn %d", req.TurnIndex),
		DetailsJSON: mustCompactJSON(map[string]any{
			"artifact_result":     saveResult,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"critic_trace":        criticTrace,
		}),
		Source:    "explorer_regenerate",
		CreatedAt: now,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                           status,
		"code":                             code,
		"source":                           s.storeWriteSource(),
		"chat_session_id":                  sid,
		"turn_index":                       req.TurnIndex,
		"memories_saved":                   saveResult.Memories,
		"evidence_saved":                   saveResult.Evidence,
		"kg_triples_saved":                 saveResult.KGTriples,
		"subjective_entity_memories_saved": saveResult.SubjectiveEntityMemories,
		"character_states_saved":           saveResult.CharacterStates,
		"world_rules_saved":                saveResult.WorldRules,
		"entities_saved":                   saveResult.Entities,
		"trust_states_saved":               saveResult.TrustStates,
		"storylines_saved":                 saveResult.Storylines,
		"pending_threads_saved":            saveResult.PendingThreads,
		"active_states_saved":              saveResult.ActiveStates,
		"canonical_state_layers_saved":     saveResult.CanonicalStateLayers,
		"vectors_upserted":                 saveResult.VectorsUpserted,
		"episode_backfill":                 episodeBackfill,
		"world_rule_backfill":              worldRuleBackfill,
		"warnings":                         saveResult.Warnings,
		"skip_reasons":                     saveResult.SkipReasons,
		"store_write_errors":               saveResult.Errors,
		"store_write_error_details":        saveResult.ErrorDetails,
		"critic_result":                    extraction,
		"critic_trace":                     criticTrace,
		"llm_config_trace":                 llmTrace,
		"note":                             "Explorer regenerate rebuilt this turn through the same Critic artifact pipeline used by complete-turn and admin rescan",
	})
}

func (s *Server) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteMemoryMutation(w, r, "DELETE /explorer/memories/{memory_id}")
}

func (s *Server) handleDeleteMemoryPost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteMemoryMutation(w, r, "POST /explorer/memories/{memory_id}/delete")
}

func (s *Server) handleDeleteDirectEvidence(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteDirectEvidenceMutation(w, r, "DELETE /explorer/direct-evidence/{record_id}")
}

func (s *Server) handleDeleteDirectEvidencePost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteDirectEvidenceMutation(w, r, "POST /explorer/direct-evidence/{record_id}/delete")
}

func (s *Server) handleDeleteKGTriple(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteKGTripleMutation(w, r, "DELETE /explorer/kg_triples/{triple_id}")
}

func (s *Server) handleDeleteKGTriplePost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteKGTripleMutation(w, r, "POST /explorer/kg_triples/{triple_id}/delete")
}

func (s *Server) handleDeleteMemoryMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	memoryID, ok := parseExplorerPathID(w, r, "memory_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	mem, found, err := s.findMemoryForExplorerPatch(r.Context(), sid, memoryID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteMemoryByID(r.Context(), sid, memoryID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	vectorCleanup := s.deleteMemoryVectorDocument(r.Context(), sid, mem)
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "memory",
		TargetID:      memoryID,
		Summary:       "Explorer manual memory delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"turn_index":    mem.TurnIndex,
				"summary_json":  mem.SummaryJSON,
				"importance":    mem.Importance,
				"archive_wing":  mem.PlaceWing,
				"archive_room":  mem.PlaceRoom,
				"created_at":    mem.CreatedAt,
				"evidence":      mem.Evidence,
				"embedding_set": strings.TrimSpace(mem.Embedding) != "",
			},
			"changed_at":     changedAt,
			"vector_cleanup": vectorCleanup,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "memory",
		"target_id":        memoryID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
		"vector_cleanup":   vectorCleanup,
	})
}

func (s *Server) deleteMemoryVectorDocument(ctx context.Context, sid string, mem store.Memory) map[string]any {
	docID := memoryVectorDocumentID(sid, mem)
	cleanup := map[string]any{
		"attempted":   false,
		"ok":          true,
		"document_id": docID,
	}
	if docID == "" {
		cleanup["skipped_reason"] = "missing_vector_document_id"
		return cleanup
	}
	if s.Vector == nil {
		cleanup["skipped_reason"] = "vector_store_not_configured"
		return cleanup
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		cleanup["skipped_reason"] = "chromadb_endpoint_not_configured"
		return cleanup
	}
	deleter, ok := s.Vector.(vector.DocumentDeleter)
	if !ok {
		cleanup["ok"] = false
		cleanup["skipped_reason"] = "vector_store_does_not_support_document_delete"
		return cleanup
	}
	cleanup["attempted"] = true
	if err := deleter.DeleteDocuments(ctx, []string{docID}); err != nil {
		if errors.Is(err, vector.ErrNotEnabled) {
			cleanup["warning"] = "vector_store_not_enabled"
			cleanup["deleted_ids"] = 0
			return cleanup
		}
		cleanup["ok"] = false
		cleanup["error"] = err.Error()
		cleanup["deleted_ids"] = 0
		return cleanup
	}
	cleanup["deleted_ids"] = 1
	return cleanup
}

func (s *Server) handleDeleteDirectEvidenceMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	recordID, ok := parseExplorerPathID(w, r, "record_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	evidence, found, err := s.findEvidenceForExplorerPatch(r.Context(), sid, recordID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteDirectEvidenceByID(r.Context(), sid, recordID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	vectorCleanup := s.deleteDerivedArtifactVectorDocuments(r.Context(), sid, "evidence", recordID)
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "direct_evidence",
		TargetID:      recordID,
		Summary:       "Explorer manual direct evidence delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"evidence_kind":        evidence.EvidenceKind,
				"evidence_text":        evidence.EvidenceText,
				"archive_state":        evidence.ArchiveState,
				"capture_verification": evidence.CaptureVerification,
				"committed_gate":       evidence.CommittedGate,
				"tombstoned":           evidence.Tombstoned,
				"turn_anchor":          evidence.TurnAnchor,
				"source_turn_start":    evidence.SourceTurnStart,
				"source_turn_end":      evidence.SourceTurnEnd,
				"created_at":           evidence.CreatedAt,
			},
			"changed_at":     changedAt,
			"vector_cleanup": vectorCleanup,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "direct_evidence",
		"target_id":        recordID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
		"vector_cleanup":   vectorCleanup,
	})
}

func (s *Server) handleDeleteKGTripleMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	tripleID, ok := parseExplorerPathID(w, r, "triple_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	triple, found, err := s.findKGTripleForExplorerPatch(r.Context(), sid, tripleID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteKGTripleByID(r.Context(), sid, tripleID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "kg_triple",
		TargetID:      tripleID,
		Summary:       "Explorer manual KG triple delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"subject":     triple.Subject,
				"predicate":   triple.Predicate,
				"object":      triple.Object,
				"valid_from":  triple.ValidFrom,
				"valid_to":    triple.ValidTo,
				"source_turn": triple.SourceTurn,
				"created_at":  triple.CreatedAt,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "kg_triple",
		"target_id":        tripleID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}
