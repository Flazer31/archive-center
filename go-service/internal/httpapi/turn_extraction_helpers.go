package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func chatCompletionText(resp map[string]any) string {
	choices := sliceFromAny(resp["choices"])
	if len(choices) == 0 {
		return ""
	}
	choice := mapFromAny(choices[0])
	msg := mapFromAny(choice["message"])
	if content := stringFromMap(msg, "content"); content != "" {
		return content
	}
	return extractionStringFromAny(choice["text"])
}

func canonicalCompleteTurnIndex(ctx context.Context, st store.Store, sid string, requested int) int {
	if requested <= 0 {
		requested = 1
	}
	logs, err := st.ListChatLogs(ctx, sid, 0, 0)
	if err != nil {
		return requested
	}
	maxTurn := 0
	for _, log := range logs {
		if log.ChatSessionID == sid && log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	if requested <= maxTurn {
		return maxTurn + 1
	}
	return requested
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func sliceFromAny(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{}
}

func stringsFromAny(v any) []string {
	if items, ok := v.([]string); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s != "" {
			return []string{s}
		}
	}
	out := []string{}
	for _, item := range sliceFromAny(v) {
		if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringFromMap(m map[string]any, key string) string {
	return strings.TrimSpace(extractionStringFromAny(m[key]))
}

func boolFromAny(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "y", "1", "on":
			return true
		default:
			return false
		}
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	case json.Number:
		n, err := t.Float64()
		return err == nil && n != 0
	default:
		return false
	}
}

func extractionStringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func int64FromMap(m map[string]any, key string, fallback int64) int64 {
	return int64(intFromAny(m[key], int(fallback)))
}

func intFromAny(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return fallback
}

func floatFromMap(m map[string]any, key string, fallback float64) float64 {
	return extractionFloatFromAny(m[key], fallback)
}

func extractionFloatFromAny(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if n, err := t.Float64(); err == nil {
			return n
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return n
		}
	}
	return fallback
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func extractionFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

var kgPredicateRe = regexp.MustCompile(`[^a-z0-9_]+`)

func sanitizeKGPredicate(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "_")
	v = kgPredicateRe.ReplaceAllString(v, "_")
	return strings.Trim(v, "_")
}

func sanitizeKGPart(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "null") || strings.EqualFold(v, "none") {
		return ""
	}
	return v
}

func sanitizeEvidenceExcerptForTurn(excerpt string, turnContent string) string {
	text := strings.TrimSpace(excerpt)
	if text == "" {
		return ""
	}
	turn := strings.TrimSpace(turnContent)
	if turn == "" {
		return ""
	}
	if len([]rune(text)) > 500 {
		text = string([]rune(text)[:500])
	}
	compactText := strings.Join(strings.Fields(text), " ")
	compactTurn := strings.Join(strings.Fields(turn), " ")
	if compactText == "" || compactText == compactTurn {
		return ""
	}
	if !strings.Contains(turn, text) && !strings.Contains(compactTurn, compactText) {
		return ""
	}
	return text
}

func sanitizeCriticStorageText(text string) string {
	cleaned := filterCompleteMarkerPattern.ReplaceAllString(text, "")
	cleaned = closedThoughtTagPattern.ReplaceAllString(cleaned, "")
	cleaned = openThoughtTagPattern.ReplaceAllString(cleaned, "")
	cleaned = thoughtLinePrefixPattern.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

func isPlaceholderKGPart(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	return placeholderKGPartPattern.MatchString(v)
}

func shouldSkipKGTriple(subject, predicate, object, sid string) bool {
	if subject == "" || predicate == "" || object == "" || subject == sid {
		return true
	}
	if isPlaceholderKGPart(subject) || isPlaceholderKGPart(object) {
		return true
	}
	switch predicate {
	case "has_turn", "turn", "mentions_turn", "source_turn":
		return true
	}
	return false
}

func extractedEntityNames(ctx context.Context, s *Server, sid string, entities map[string]any) []string {
	out := []string{}
	add := func(items []any) {
		for _, item := range items {
			entity := mapFromAny(item)
			name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title"))))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			out = appendUniqueString(out, name)
		}
	}
	add(sliceFromAny(entities["characters"]))
	return out
}

func relationshipMemoryTargets(relationshipMemory map[string]any, characterNames []string) []string {
	targets := []string{}
	for _, key := range []string{"target_name", "target", "character", "entity", "subject", "object"} {
		target := sanitizeParticipantActorName(stringFromMap(relationshipMemory, key))
		if target != "" {
			targets = appendUniqueString(targets, target)
		}
	}
	for _, item := range stringsFromAny(relationshipMemory["pair"]) {
		for _, part := range relationshipPairParts(item) {
			target := sanitizeParticipantActorName(part)
			if target != "" {
				targets = appendUniqueString(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		for _, item := range characterNames {
			targets = appendUniqueString(targets, item)
		}
	}
	return targets
}

func sanitizeParticipantActorName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" || isPlaceholderKGPart(name) {
		return ""
	}
	return name
}

func relationshipPairParts(value string) []string {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	splitters := []string{"<->", "↔", "->", "→", "/", "|", "&"}
	for _, sep := range splitters {
		if strings.Contains(text, sep) {
			parts := []string{}
			for _, part := range strings.Split(text, sep) {
				part = strings.TrimSpace(part)
				if part != "" {
					parts = append(parts, part)
				}
			}
			return parts
		}
	}
	return []string{text}
}

func sanitizeStateDeltasForParticipant(raw any) map[string]any {
	state := mapFromAny(raw)
	if len(state) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range state {
		if key == "relationship_changes" {
			cleaned := []any{}
			for _, item := range sliceFromAny(value) {
				rel := mapFromAny(item)
				left, right := relationshipChangeActors(rel)
				left = sanitizeParticipantActorName(left)
				right = sanitizeParticipantActorName(right)
				if left == "" || right == "" {
					continue
				}
				rel["from"] = left
				rel["to"] = right
				delete(rel, "pair")
				delete(rel, "pair_key")
				identity := mapFromAny(rel["identity"])
				identity["left_entity"] = left
				identity["right_entity"] = right
				delete(identity, "pair")
				delete(identity, "pair_key")
				rel["identity"] = identity
				cleaned = append(cleaned, rel)
			}
			if len(cleaned) > 0 {
				out[key] = cleaned
			}
			continue
		}
		out[key] = value
	}
	return out
}

func relationshipChangeActors(rel map[string]any) (string, string) {
	identity := mapFromAny(rel["identity"])
	left := extractionFirstNonEmpty(
		stringFromMap(rel, "from"),
		stringFromMap(rel, "source"),
		stringFromMap(identity, "left_entity"),
		stringFromMap(identity, "source"),
	)
	right := extractionFirstNonEmpty(
		stringFromMap(rel, "to"),
		stringFromMap(rel, "target"),
		stringFromMap(identity, "right_entity"),
		stringFromMap(identity, "target"),
	)
	if left != "" && right != "" {
		return left, right
	}
	for _, key := range []string{"pair", "pair_key"} {
		for _, part := range relationshipPairParts(extractionFirstNonEmpty(stringFromMap(rel, key), stringFromMap(identity, key))) {
			if left == "" {
				left = part
				continue
			}
			if right == "" {
				right = part
				break
			}
		}
		if left != "" && right != "" {
			break
		}
	}
	return left, right
}

func stableKey(prefix, text string) string {
	key := strings.ToLower(strings.TrimSpace(text))
	key = kgPredicateRe.ReplaceAllString(strings.ReplaceAll(key, " ", "_"), "_")
	key = strings.Trim(key, "_")
	if key == "" {
		key = "item"
	}
	if len(key) > 80 {
		key = key[:80]
	}
	return prefix + "_" + key
}

// normalizeRelationshipStateV2 ensures v1 relationship_memory payloads stay intact
// while injecting safe minimal defaults for missing v2 additive sections (P518).
// It preserves identity, core_state, dynamics, context, history, verification,
// and branch-style fields (desire, fear, wound, mask, bond, fixation).
func normalizeRelationshipStateV2(raw map[string]any) map[string]any {
	if raw == nil {
		raw = map[string]any{}
	}
	out := make(map[string]any, len(raw)+12)
	for k, v := range raw {
		out[k] = v
	}
	for _, key := range []string{"identity", "core_state", "dynamics", "context", "history", "verification"} {
		if _, ok := out[key]; !ok {
			out[key] = map[string]any{}
		}
	}
	for _, key := range []string{"desire", "fear", "wound", "mask", "bond", "fixation"} {
		if _, ok := out[key]; !ok {
			out[key] = map[string]any{}
		}
	}
	return out
}

func worldRuleItemsForSave(extraction map[string]any) []any {
	out := make([]any, 0)
	seen := map[string]bool{}
	add := func(raw any, fromWorldState bool) {
		rule := mapFromAny(raw)
		if len(rule) == 0 {
			text := strings.TrimSpace(extractionStringFromAny(raw))
			if text == "" {
				return
			}
			rule = map[string]any{
				"key":      stableKey("world_rule", text),
				"value":    text,
				"scope":    "session",
				"category": "world_state",
			}
		}
		key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name")))
		if key == "" {
			return
		}
		scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(rule, "scope"), "session"))
		scopeName := strings.TrimSpace(stringFromMap(rule, "scope_name"))
		rule["scope"] = scope
		sig := strings.ToLower(scope) + "\x00" + strings.ToLower(scopeName) + "\x00" + strings.ToLower(key)
		if seen[sig] {
			return
		}
		seen[sig] = true
		if fromWorldState {
			cp := make(map[string]any, len(rule)+2)
			for k, v := range rule {
				cp[k] = v
			}
			if strings.TrimSpace(stringFromMap(cp, "scope")) == "" {
				cp["scope"] = "session"
			} else {
				cp["scope"] = store.NormalizeWorldRuleScope(stringFromMap(cp, "scope"))
			}
			if strings.TrimSpace(stringFromMap(cp, "category")) == "" {
				cp["category"] = "world_state"
			}
			out = append(out, cp)
			return
		}
		out = append(out, rule)
	}
	for _, item := range sliceFromAny(extraction["world_rules"]) {
		add(item, false)
	}
	if ws := mapFromAny(extraction["world_state"]); len(ws) > 0 {
		for _, item := range sliceFromAny(ws["rules"]) {
			add(item, true)
		}
	}
	return out
}

// extractWorldStatePayload builds a minimal world_state snapshot from critic extraction.
// It prefers an explicit world_state map, then falls back to world_rules array (P469).
func extractWorldStatePayload(extraction map[string]any) (map[string]any, bool) {
	if ws := mapFromAny(extraction["world_state"]); len(ws) > 0 {
		return ws, true
	}
	rules := sliceFromAny(extraction["world_rules"])
	if len(rules) > 0 {
		out := map[string]any{
			"rules":   rules,
			"version": "world_state.v1",
		}
		for _, key := range []string{"faction_status", "region_pressure", "offscreen_threads"} {
			if v, ok := extraction[key]; ok {
				out[key] = v
			}
		}
		return out, true
	}
	return nil, false
}

// mapKeyToCanonicalLayerType maps extraction keys to canonical layer types (P358).
func mapKeyToCanonicalLayerType(key string) string {
	switch key {
	case "relationship_memory":
		return "relationship_state"
	case "state_deltas":
		return "scene_state"
	case "entities":
		return "entity_state"
	case "world_rules", "world_state":
		return "world_state"
	default:
		return key
	}
}

func canonicalStatePromotionAllowed(raw any, confidence float64) bool {
	if confidence < 0.7 {
		return false
	}
	payload := mapFromAny(raw)
	status := strings.ToLower(strings.TrimSpace(extractionFirstNonEmpty(
		stringFromMap(payload, "verification"),
		stringFromMap(payload, "capture_verification"),
		stringFromMap(payload, "promotion_status"),
		stringFromMap(payload, "status"),
	)))
	switch status {
	case "pending", "rejected", "unverified", "repair_queue", "hold", "manual_review":
		return false
	}
	if rawVerified, ok := payload["verified"]; ok {
		switch v := rawVerified.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(strings.TrimSpace(v), "true") || strings.EqualFold(strings.TrimSpace(v), "verified")
		}
	}
	return true
}

// extractConfidenceForStateKey extracts confidence from critic extraction for a given state key (P407).
func extractConfidenceForStateKey(extraction map[string]any, key string) float64 {
	switch key {
	case "relationship_memory":
		if rm := mapFromAny(extraction["relationship_memory"]); len(rm) > 0 {
			return clampFloat(extractionFloatFromAny(rm["confidence"], 0.7), 0, 1)
		}
	case "state_deltas":
		if sd := mapFromAny(extraction["state_deltas"]); len(sd) > 0 {
			return clampFloat(extractionFloatFromAny(sd["confidence"], 0.7), 0, 1)
		}
	case "entities":
		return 0.7
	case "world_rules", "world_state":
		if ws, ok := extractWorldStatePayload(extraction); ok {
			return clampFloat(extractionFloatFromAny(ws["confidence"], 0.75), 0, 1)
		}
		return 0.75
	}
	return 0.7
}
