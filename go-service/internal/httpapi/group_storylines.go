package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Storyline: R1 read, R2 write

func (s *Server) handleStorylinesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListStorylines(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	referenceTurn := resolveStorylineReferenceTurn(items, r.URL.Query().Get("current_turn"))
	storylines := storylineResponseItems(items, referenceTurn)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"storylines":      storylines,
		"count":           len(storylines),
		"reference_turn":  nullableIntPtr(referenceTurn),
	})
}

func (s *Server) handleStorylinePatch(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchStoryline(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /storylines/{storyline_id}")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeStorylinePatchPayload(payload, false)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchStoryline(r.Context(), storylineID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /storylines/{storyline_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"storyline_id":   storylineID,
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
	for _, key := range []string{"confidence", "evidence_count", "last_evidence_turn"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStorylineTrust(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchStorylineTrust(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /storylines/{storyline_id}/trust")
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
	updatedFields, err := mutator.PatchStorylineTrust(r.Context(), storylineID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /storylines/{storyline_id}/trust")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{
		"status":         "ok",
		"storyline_id":   storylineID,
		"updated_fields": updatedFields,
	}
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStorylineDelete(w http.ResponseWriter, r *http.Request) {
	storylineID, ok := parseNarrativeInt64Path(w, r, "storyline_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		DeleteStoryline(context.Context, int64) error
	})
	if !ok {
		writeShadowGuard(w, "DELETE /storylines/{storyline_id}")
		return
	}
	if err := mutator.DeleteStoryline(r.Context(), storylineID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("storyline %d not found", storylineID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /storylines/{storyline_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"deleted_id": storylineID,
	})
}

func (s *Server) handleStorylinesSync(w http.ResponseWriter, r *http.Request) {
	saver, ok := s.Store.(interface {
		SaveStoryline(context.Context, *store.Storyline) error
	})
	if !ok {
		writeShadowGuard(w, "POST /storylines/sync")
		return
	}
	var req storylineSyncRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode != "apply" {
		mode = "dry_run"
	}
	candidates := parseStorylineCandidatesFromSupervisor(req.SupervisorResult)
	validated := make([]storylineSyncCandidate, 0, len(candidates))
	validationErrors := make([]map[string]any, 0)
	for _, candidate := range candidates {
		normalized, errs := normalizeStorylineSyncCandidate(candidate)
		if len(errs) > 0 {
			validationErrors = append(validationErrors, map[string]any{
				"name":   candidate.Name,
				"errors": errs,
			})
			continue
		}
		validated = append(validated, normalized)
	}
	if mode == "dry_run" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"mode":              "dry_run",
			"parsed_count":      len(candidates),
			"valid_count":       len(validated),
			"candidates":        storylineCandidatesPreview(validated),
			"validation_errors": validationErrors,
		})
		return
	}
	if len(validated) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"mode":              "apply",
			"parsed_count":      len(candidates),
			"applied_count":     0,
			"results":           []any{},
			"validation_errors": validationErrors,
		})
		return
	}

	existingRows, err := s.Store.ListStorylines(r.Context(), sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		writeInternalError(w, err.Error())
		return
	}
	existingByName := make(map[string]store.Storyline)
	for _, row := range existingRows {
		existingByName[row.Name] = row
	}
	now := time.Now().UTC()
	results := make([]map[string]any, 0, len(validated))
	for _, candidate := range validated {
		existing, hadExisting := existingByName[candidate.Name]
		item := candidate.toStoreStoryline(sid, req.TurnIndex, now, existing, hadExisting)
		if err := saver.SaveStoryline(r.Context(), &item); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				writeShadowGuard(w, "POST /storylines/sync")
				return
			}
			writeInternalError(w, err.Error())
			return
		}
		action := "created"
		if hadExisting {
			action = "updated"
		}
		results = append(results, map[string]any{
			"action":             action,
			"id":                 nullableInt64(item.ID),
			"name":               item.Name,
			"confidence":         item.Confidence,
			"evidence_count":     nullableInt(item.EvidenceCount),
			"last_evidence_turn": nullableInt(item.LastEvidenceTurn),
		})
		existingByName[candidate.Name] = item
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"mode":              "apply",
		"parsed_count":      len(candidates),
		"applied_count":     len(results),
		"results":           results,
		"validation_errors": validationErrors,
	})
}

type storylineSyncRequest struct {
	ChatSessionID    string         `json:"chat_session_id"`
	SupervisorResult map[string]any `json:"supervisor_result"`
	Mode             string         `json:"mode"`
	TurnIndex        *int           `json:"turn_index"`
}

type storylineSyncCandidate struct {
	Name   string
	Fields map[string]any
}

func parseNarrativeInt64Path(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := strings.TrimSpace(r.PathValue(name))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeNarrativeJSONMap(r *http.Request) (map[string]any, error) {
	var payload map[string]any
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func normalizeStorylinePatchPayload(payload map[string]any, requireName bool) (map[string]any, error) {
	updates := make(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}
	if val, exists := payload["name"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("name must be a non-empty string")
		}
		updates["name"] = strings.TrimSpace(text)
	} else if requireName {
		return nil, fmt.Errorf("name is required")
	}
	if val, exists := payload["status"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("status must be a string")
		}
		text = firstNonEmpty(strings.TrimSpace(text), "active")
		if text != "active" && text != "paused" && text != "resolved" {
			return nil, fmt.Errorf("invalid status: %s", text)
		}
		updates["status"] = text
	}
	if val, exists := payload["current_context"]; exists {
		text, ok := storylineNullableStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("current_context must be a string or null")
		}
		updates["current_context"] = text
	}
	for _, key := range []string{"entities_json", "key_points_json", "ongoing_tensions_json"} {
		if val, exists := payload[key]; exists {
			normalized, err := normalizeStorylineJSONPatchValue(key, val)
			if err != nil {
				return nil, err
			}
			updates[key] = normalized
		}
	}
	if val, exists := payload["confidence"]; exists {
		f, ok := storylineFloatPatchValue(val)
		if !ok || f < 0 || f > 1 {
			return nil, fmt.Errorf("confidence must be between 0.0 and 1.0")
		}
		updates["confidence"] = f
	}
	for _, key := range []string{"evidence_count", "last_evidence_turn", "first_turn", "last_turn"} {
		if val, exists := payload[key]; exists {
			i, ok := storylineIntPatchValue(val)
			if !ok || i < 0 {
				return nil, fmt.Errorf("%s must be a non-negative integer", key)
			}
			updates[key] = i
		}
	}
	return updates, nil
}

func normalizeStorylineTrustPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		val, exists := payload[key]
		if !exists {
			continue
		}
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be a boolean", key)
		}
		updates[key] = b
	}
	return updates, nil
}

func normalizePendingThreadPatchPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}
	if val, exists := payload["status"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("status must be a string")
		}
		text = strings.TrimSpace(text)
		if text != "open" && text != "paused" && text != "resolved" {
			return nil, fmt.Errorf("invalid status: %s", text)
		}
		updates["status"] = text
	}
	threadTypeVal, hasThreadType := payload["thread_type"]
	if !hasThreadType {
		threadTypeVal, hasThreadType = payload["hook_type"]
	}
	if hasThreadType {
		text, ok := storylineStringPatchValue(threadTypeVal)
		if !ok {
			return nil, fmt.Errorf("thread_type must be a string")
		}
		text = strings.TrimSpace(text)
		if !validPendingThreadType(text) {
			return nil, fmt.Errorf("invalid thread_type: %s", text)
		}
		updates["thread_type"] = text
	}
	if val, exists := payload["title"]; exists {
		text, ok := storylineStringPatchValue(val)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("title must be a non-empty string")
		}
		updates["title"] = strings.TrimSpace(text)
	}
	for _, key := range []string{"owner", "target", "resolution_note"} {
		if val, exists := payload[key]; exists {
			text, ok := storylineNullableStringPatchValue(val)
			if !ok {
				return nil, fmt.Errorf("%s must be a string or null", key)
			}
			updates[key] = text
		}
	}
	if val, exists := payload["confidence"]; exists {
		f, ok := storylineFloatPatchValue(val)
		if !ok || f < 0 || f > 1 {
			return nil, fmt.Errorf("confidence must be between 0.0 and 1.0")
		}
		updates["confidence"] = f
	}
	if val, exists := payload["details_json"]; exists {
		normalized, err := normalizePendingThreadJSONPatchValue("details_json", val)
		if err != nil {
			return nil, err
		}
		updates["details_json"] = normalized
	}
	return updates, nil
}

func validPendingThreadType(text string) bool {
	switch text {
	case "promise", "unresolved_goal", "open_question", "risk", "emotional_debt":
		return true
	default:
		return false
	}
}

func normalizePendingThreadJSONPatchValue(field string, val any) (any, error) {
	if val == nil {
		return nil, nil
	}
	if text, ok := val.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("%s must contain valid JSON", field)
		}
		return mustCompactJSON(decoded), nil
	}
	return mustCompactJSON(val), nil
}

func normalizeStorylineJSONPatchValue(field string, val any) (any, error) {
	if val == nil {
		return nil, nil
	}
	switch typed := val.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("%s must contain valid JSON", field)
		}
		if field == "key_points_json" || field == "ongoing_tensions_json" {
			items, ok := compactStorylineTextList(decoded)
			if !ok {
				return nil, fmt.Errorf("%s must be a JSON string array", field)
			}
			return mustCompactJSON(items), nil
		}
		return mustCompactJSON(decoded), nil
	default:
		if field == "key_points_json" || field == "ongoing_tensions_json" {
			items, ok := compactStorylineTextList(typed)
			if !ok {
				return nil, fmt.Errorf("%s must be a string array", field)
			}
			return mustCompactJSON(items), nil
		}
		return mustCompactJSON(typed), nil
	}
}

func compactStorylineTextList(v any) ([]string, bool) {
	items, ok := v.([]any)
	if !ok {
		if typed, ok := v.([]string); ok {
			out := make([]string, 0, len(typed))
			seen := make(map[string]bool)
			for _, item := range typed {
				text := strings.TrimSpace(item)
				key := strings.ToLower(text)
				if text != "" && !seen[key] {
					seen[key] = true
					out = append(out, text)
				}
			}
			return out, true
		}
		return nil, false
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		if item == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(item))
		key := strings.ToLower(text)
		if text == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, text)
	}
	return out, true
}

func storylineStringPatchValue(v any) (string, bool) {
	text, ok := v.(string)
	return text, ok
}

func storylineNullableStringPatchValue(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	text, ok := v.(string)
	if !ok {
		return nil, false
	}
	if strings.TrimSpace(text) == "" {
		return nil, true
	}
	return text, true
}

func storylineFloatPatchValue(v any) (float64, bool) {
	switch typed := v.(type) {
	case float64:
		return typed, true
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func storylineIntPatchValue(v any) (int, bool) {
	switch typed := v.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		i, err := typed.Int64()
		return int(i), err == nil
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func parseStorylineCandidatesFromSupervisor(supervisorResult map[string]any) []storylineSyncCandidate {
	if supervisorResult == nil {
		return nil
	}
	var out []storylineSyncCandidate
	for _, raw := range sliceFromAny(supervisorResult["storylines"]) {
		item := mapFromAny(raw)
		name := strings.TrimSpace(stringFromMap(item, "name"))
		if name == "" {
			continue
		}
		fields := map[string]any{"name": name}
		copyStorylineCandidateField(fields, item, "status", "status")
		if _, ok := item["entities_json"]; ok {
			copyStorylineCandidateField(fields, item, "entities_json", "entities_json")
		} else if _, ok := item["entities"]; ok {
			copyStorylineCandidateField(fields, item, "entities", "entities_json")
		}
		copyStorylineCandidateField(fields, item, "current_context", "current_context")
		copyStorylineCandidateField(fields, item, "context", "current_context")
		if _, ok := item["key_points_json"]; ok {
			copyStorylineCandidateField(fields, item, "key_points_json", "key_points_json")
		} else {
			copyStorylineCandidateField(fields, item, "key_points", "key_points_json")
		}
		if _, ok := item["ongoing_tensions_json"]; ok {
			copyStorylineCandidateField(fields, item, "ongoing_tensions_json", "ongoing_tensions_json")
		} else {
			copyStorylineCandidateField(fields, item, "ongoing_tensions", "ongoing_tensions_json")
		}
		copyStorylineCandidateField(fields, item, "confidence", "confidence")
		copyStorylineCandidateField(fields, item, "evidence_count", "evidence_count")
		copyStorylineCandidateField(fields, item, "last_evidence_turn", "last_evidence_turn")
		out = append(out, storylineSyncCandidate{Name: name, Fields: fields})
	}
	if len(out) > 0 {
		return out
	}
	for _, key := range []string{"book_author", "story_author"} {
		author := mapFromAny(supervisorResult[key])
		arc := strings.TrimSpace(stringFromMap(author, "current_arc"))
		if arc == "" {
			continue
		}
		fields := map[string]any{
			"name":            arc,
			"status":          "active",
			"current_context": stringFromMap(author, "narrative_goal"),
		}
		if nextBeats := sliceFromAny(author["next_beats"]); len(nextBeats) > 0 {
			fields["key_points_json"] = nextBeats
		}
		if tensions := sliceFromAny(author["ongoing_tensions"]); len(tensions) > 0 {
			fields["ongoing_tensions_json"] = tensions
		} else if guardrails := sliceFromAny(author["guardrails"]); len(guardrails) > 0 {
			fields["ongoing_tensions_json"] = guardrails
		}
		out = append(out, storylineSyncCandidate{Name: arc, Fields: fields})
		return out
	}
	return out
}

func copyStorylineCandidateField(dst map[string]any, src map[string]any, srcKey, dstKey string) {
	if val, ok := src[srcKey]; ok {
		dst[dstKey] = val
	}
}

func normalizeStorylineSyncCandidate(candidate storylineSyncCandidate) (storylineSyncCandidate, []string) {
	updates, err := normalizeStorylinePatchPayload(candidate.Fields, true)
	if err != nil {
		return candidate, []string{err.Error()}
	}
	name, _ := updates["name"].(string)
	if _, ok := updates["status"]; !ok {
		updates["status"] = "active"
	}
	return storylineSyncCandidate{Name: name, Fields: updates}, nil
}

func storylineCandidatesPreview(items []storylineSyncCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		preview := make(map[string]any, len(item.Fields))
		for key, val := range item.Fields {
			preview[key] = val
		}
		out = append(out, preview)
	}
	return out
}

func (c storylineSyncCandidate) toStoreStoryline(sid string, turnIndex *int, now time.Time, existing store.Storyline, hadExisting bool) store.Storyline {
	item := existing
	if !hadExisting {
		item = store.Storyline{
			ChatSessionID: sid,
			CreatedAt:     now,
		}
	}
	item.ChatSessionID = sid
	item.Name = c.Name
	if status, ok := c.Fields["status"].(string); ok && status != "" {
		item.Status = status
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if val, ok := c.Fields["entities_json"].(string); ok {
		item.EntitiesJSON = val
	} else if val, ok := c.Fields["entities_json"]; ok && val == nil {
		item.EntitiesJSON = ""
	}
	if val, ok := c.Fields["current_context"].(string); ok {
		item.CurrentContext = val
	} else if val, ok := c.Fields["current_context"]; ok && val == nil {
		item.CurrentContext = ""
	}
	if val, ok := c.Fields["key_points_json"].(string); ok {
		item.KeyPointsJSON = val
	} else if val, ok := c.Fields["key_points_json"]; ok && val == nil {
		item.KeyPointsJSON = ""
	}
	if val, ok := c.Fields["ongoing_tensions_json"].(string); ok {
		item.OngoingTensionsJSON = val
	} else if val, ok := c.Fields["ongoing_tensions_json"]; ok && val == nil {
		item.OngoingTensionsJSON = ""
	}
	if val, ok := c.Fields["confidence"].(float64); ok {
		item.Confidence = val
	}
	item.EvidenceCount, item.LastEvidenceTurn = resolveStorylineEvidenceUpdate(existing, hadExisting, c.Fields, turnIndex)
	if turnIndex != nil {
		if item.FirstTurn == 0 {
			item.FirstTurn = *turnIndex
		}
		item.LastTurn = *turnIndex
	}
	item.UpdatedAt = now
	return item
}

func resolveStorylineEvidenceUpdate(existing store.Storyline, hadExisting bool, fields map[string]any, turnIndex *int) (int, int) {
	currentCount := 0
	currentLastTurn := 0
	if hadExisting {
		currentCount = existing.EvidenceCount
		currentLastTurn = existing.LastEvidenceTurn
	}
	increment, hasIncrement := fields["evidence_count"].(int)
	explicitTurn, hasExplicitTurn := fields["last_evidence_turn"].(int)
	hasPayload := false
	for _, key := range []string{"current_context", "key_points_json", "ongoing_tensions_json", "entities_json"} {
		val, ok := fields[key]
		if !ok || val == nil {
			continue
		}
		if text, ok := val.(string); !ok || strings.TrimSpace(text) != "" {
			hasPayload = true
			break
		}
	}
	if !hasIncrement {
		if hasPayload || hasExplicitTurn {
			increment = 1
		}
	}
	observedTurn := 0
	if hasExplicitTurn {
		observedTurn = explicitTurn
	} else if increment > 0 && turnIndex != nil {
		observedTurn = *turnIndex
	}
	if observedTurn != 0 && currentLastTurn == observedTurn {
		return currentCount, currentLastTurn
	}
	if increment <= 0 {
		return currentCount, currentLastTurn
	}
	return currentCount + increment, observedTurn
}
