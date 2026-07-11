package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// World rules: R1 read, R2 write

func (s *Server) handleWorldRulesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListWorldRules(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	responseItems := worldRuleResponseItems(items, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  responseItems,
		"count":  len(responseItems),
	})
}

func (s *Server) handleWorldRulesInherited(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	activeScope := strings.TrimSpace(r.URL.Query().Get("active_scope"))
	scopeName := strings.TrimSpace(r.URL.Query().Get("scope_name"))
	if activeScope == "" {
		if saved, _, err := s.resolveActiveScope(r.Context(), sid); err == nil && saved != nil {
			activeScope = strings.TrimSpace(saved.ActiveScope)
			if scopeName == "" {
				scopeName = strings.TrimSpace(saved.ScopeName)
			}
		} else if err != nil {
			writeInternalError(w, err.Error())
			return
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	if !isValidWorldRuleScope(activeScope) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "error",
			"detail": "active_scope must be one of [root region location faction system session]",
		})
		return
	}
	scopeChain := worldRuleScopeChain(activeScope)
	items, err := s.Store.ListInheritedWorldRules(r.Context(), sid, activeScope, scopeName)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	rules := worldRuleResponseItems(items, activeScope)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"active_scope": activeScope,
		"scope_name":   nullableString(scopeName),
		"scope_chain":  scopeChain,
		"rules":        rules,
		"count":        len(rules),
	})
}

func (s *Server) handleWorldRulesSync(w http.ResponseWriter, r *http.Request) {
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		writeShadowGuard(w, "POST /world-rules/sync")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(payload["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["mode"])))
	if mode == "" {
		mode = "apply"
	}
	if mode != "apply" && mode != "dry_run" {
		writeBadRequest(w, "mode must be apply or dry_run")
		return
	}
	turnIndex := intFromAny(payload["turn_index"], 0)
	candidates := buildWorldRuleSyncCandidates(sid, turnIndex, mapFromAny(payload["supervisor_response"]))
	if mode == "dry_run" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"mode":            mode,
			"chat_session_id": sid,
			"candidate_count": len(candidates),
			"would_write":     false,
		})
		return
	}
	applied := 0
	for i := range candidates {
		if err := saver.SaveWorldRule(r.Context(), &candidates[i]); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				writeShadowGuard(w, "POST /world-rules/sync")
				return
			}
			writeInternalError(w, err.Error())
			return
		}
		applied++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"mode":            mode,
		"chat_session_id": sid,
		"candidate_count": len(candidates),
		"applied_count":   applied,
	})
}

func (s *Server) handleWorldRulePatch(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchWorldRule(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /world-rules/{rule_id}")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeWorldRulePatchPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchWorldRule(r.Context(), ruleID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /world-rules/{rule_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "rule_id": ruleID, "updated_fields": updatedFields})
}

func (s *Server) handleWorldRuleTrust(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		PatchWorldRuleTrust(context.Context, int64, map[string]any) ([]string, error)
	})
	if !ok {
		writeShadowGuard(w, "PATCH /world-rules/{rule_id}/trust")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeWorldRuleTrustPayload(payload)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	updatedFields, err := mutator.PatchWorldRuleTrust(r.Context(), ruleID, updates)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /world-rules/{rule_id}/trust")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	resp := map[string]any{"status": "ok", "rule_id": ruleID, "updated_fields": updatedFields}
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if val, exists := updates[key]; exists {
			resp[key] = val
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleWorldRuleDelete(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseNarrativeInt64Path(w, r, "rule_id")
	if !ok {
		return
	}
	mutator, ok := s.Store.(interface {
		DeleteWorldRule(context.Context, int64) error
	})
	if !ok {
		writeShadowGuard(w, "DELETE /world-rules/{rule_id}")
		return
	}
	if err := mutator.DeleteWorldRule(r.Context(), ruleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("world rule %d not found", ruleID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /world-rules/{rule_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	vectorCleanup := map[string]any{"attempted": false, "ok": true, "skipped_reason": "chat_session_id_not_provided"}
	if sid != "" {
		vectorCleanup = s.deleteDerivedArtifactVectorDocuments(r.Context(), sid, "world_rule", ruleID)
	}
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "deleted_id": ruleID, "vector_cleanup": vectorCleanup})
}

func buildWorldRuleSyncCandidates(sid string, turnIndex int, supervisor map[string]any) []store.WorldRule {
	sectionWorld := mapFromAny(supervisor["section_world"])
	genre := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(sectionWorld, "genre"), stringFromMap(sectionWorld, "genre_hint")))
	rawItems := collectWorldRuleSyncItems(supervisor)
	now := time.Now().UTC()
	out := make([]store.WorldRule, 0, len(rawItems))
	seen := map[string]bool{}
	for _, raw := range rawItems {
		if text, ok := raw.(string); ok {
			key := truncateRunes(strings.TrimSpace(text), 500)
			if key == "" {
				continue
			}
			raw = map[string]any{
				"scope":    "root",
				"category": "custom",
				"key":      key,
				"genre":    genre,
			}
		}
		item := mapFromAny(raw)
		key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "key"), stringFromMap(item, "name"), stringFromMap(item, "rule_key")))
		if key == "" {
			key = strings.TrimSpace(stringFromMap(item, "rule"))
		}
		key = truncateRunes(key, 500)
		if key == "" {
			continue
		}
		scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(item, "scope"), "root"))
		if !isValidWorldRuleScope(scope) {
			continue
		}
		category := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "category"), "custom"))
		dedupeKey := strings.ToLower(scope + "\x00" + stringFromMap(item, "scope_name") + "\x00" + key)
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		value := item["value_json"]
		if value == nil {
			value = item["value"]
		}
		if value == nil {
			value = item["description"]
		}
		if value == nil {
			value = item["summary"]
		}
		out = append(out, store.WorldRule{
			ChatSessionID: sid,
			Scope:         scope,
			ScopeName:     strings.TrimSpace(stringFromMap(item, "scope_name")),
			Category:      category,
			Key:           key,
			ValueJSON:     normalizeWorldRuleValueJSON(value),
			Genre:         strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, "genre"), genre)),
			SourceTurn:    turnIndex,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	return out
}

func collectWorldRuleSyncItems(supervisor map[string]any) []any {
	items := []any{}
	appendItems := func(raw any) {
		if raw == nil {
			return
		}
		for _, item := range sliceFromAny(raw) {
			items = append(items, item)
		}
	}
	sectionWorld := mapFromAny(supervisor["section_world"])
	appendItems(sectionWorld["constants"])
	appendItems(sectionWorld["rules"])
	appendItems(sectionWorld["world_rules"])
	appendItems(sectionWorld["confidence_notes"])
	appendItems(supervisor["world_rules"])
	if len(items) == 0 && strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(sectionWorld, "key"), stringFromMap(sectionWorld, "name"))) != "" {
		items = append(items, sectionWorld)
	}
	return items
}

func normalizeWorldRulePatchPayload(payload map[string]any) (map[string]any, error) {
	updates := make(map[string]any)
	for _, key := range []string{"scope", "scope_name", "category", "key", "genre"} {
		val, exists := payload[key]
		if !exists {
			continue
		}
		rawText, ok := storylineNullableStringPatchValue(val)
		if !ok {
			return nil, fmt.Errorf("%s must be a string or null", key)
		}
		text, _ := rawText.(string)
		if key == "scope" {
			text = store.NormalizeWorldRuleScope(firstNonEmpty(strings.TrimSpace(text), "root"))
			if !isValidWorldRuleScope(text) {
				return nil, fmt.Errorf("invalid scope: %s", text)
			}
		}
		if key == "key" && strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("key must be a non-empty string")
		}
		if key == "scope_name" && strings.TrimSpace(text) == "" {
			updates[key] = nil
			continue
		}
		updates[key] = text
	}
	if val, exists := payload["value_json"]; exists {
		updates["value_json"] = normalizeWorldRuleValueJSON(val)
	} else if val, exists := payload["value"]; exists {
		updates["value_json"] = normalizeWorldRuleValueJSON(val)
	}
	if val, exists := payload["source_turn"]; exists {
		i, ok := storylineIntPatchValue(val)
		if !ok || i < 0 {
			return nil, fmt.Errorf("source_turn must be a non-negative integer")
		}
		updates["source_turn"] = i
	}
	return updates, nil
}

func normalizeWorldRuleTrustPayload(payload map[string]any) (map[string]any, error) {
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

func normalizeWorldRuleValueJSON(val any) string {
	if val == nil {
		return ""
	}
	if text, ok := val.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return ""
		}
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err == nil {
			return mustCompactJSON(decoded)
		}
		return mustCompactJSON(text)
	}
	return mustCompactJSON(val)
}
