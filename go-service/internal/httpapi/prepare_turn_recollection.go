package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildPersonaRecollectionText(entries []store.PersonaMemoryEntry, maxEntries, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
	maxEntries = prepareTurnRecallLimit(maxEntries)
	perEntryChars = prepareTurnTextBudget(perEntryChars)
	lines := []string{
		"support-only private recollection; not current-world truth.",
	}
	if personaRecollectionSecretGuardActive(entries) {
		lines = append(lines,
			"Secret Guard: protagonist-only private intuition. Never reveal its origin; use only as hesitation, instinct, or careful choice.",
		)
	}
	entryLineBase := len(lines)
	for _, entry := range entries {
		if len(lines)-entryLineBase >= maxEntries {
			break
		}
		text := personaRecollectionPromptLineText(entry, perEntryChars)
		if text == "" {
			continue
		}
		meta := []string{}
		if entry.SourceTurn > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", entry.SourceTurn))
		}
		if entry.Importance10 > 0 {
			meta = append(meta, fmt.Sprintf("imp %.1f/10", entry.Importance10))
		}
		if portability := strings.TrimSpace(entry.Portability); portability != "" {
			meta = append(meta, portability)
		}
		prefix := "-"
		if len(meta) > 0 {
			prefix = "- (" + strings.Join(meta, ", ") + ")"
		}
		lines = append(lines, prefix+" "+text)
	}
	if len(lines) <= entryLineBase {
		return ""
	}
	return makePrepareTurnSection("[Persona Recollection]", lines)
}

func buildCharacterPrivateRecollectionText(entries []store.ProtagonistEntityMemory, maxEntries, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
	maxEntries = prepareTurnRecallLimit(maxEntries)
	perEntryChars = prepareTurnTextBudget(perEntryChars)
	lines := []string{
		"NPC private memory is the owning NPC's interpretation/bias, not player knowledge, narrator knowledge, or current-world truth; do not present it as objective fact.",
		"Use only as subtext: hesitation, recognition, avoidance, attraction, suspicion, or careful choice.",
		"Do not imply protagonist knowledge or explain the memory unless current evidence or explicit user instruction reveals it.",
	}
	entryLineBase := len(lines)
	for _, entry := range entries {
		if len(lines)-entryLineBase >= maxEntries {
			break
		}
		text := characterPrivateRecollectionPromptLineText(entry, perEntryChars)
		if text == "" {
			continue
		}
		owner := strings.TrimSpace(entry.OwnerEntityName)
		if owner == "" {
			owner = strings.TrimSpace(entry.OwnerEntityKey)
		}
		if owner == "" {
			owner = "unknown NPC"
		}
		meta := []string{"owner " + owner}
		if entry.SourceTurn > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", entry.SourceTurn))
		}
		if entry.Importance10 > 0 {
			meta = append(meta, fmt.Sprintf("imp %.1f/10", entry.Importance10))
		}
		if policy := strings.TrimSpace(entry.TargetRevealPolicy); policy != "" {
			meta = append(meta, policy)
		}
		lines = append(lines, "- ("+strings.Join(meta, ", ")+") "+text)
	}
	if len(lines) <= entryLineBase {
		return ""
	}
	return makePrepareTurnSection("[Character Private Recollection]", lines)
}

func personaRecollectionPromptLineText(entry store.PersonaMemoryEntry, perEntryChars int) string {
	text := strings.TrimSpace(entry.MemoryText)
	if text == "" {
		return ""
	}
	if personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}) {
		prefix := "Protected hint: "
		text = protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.InjectionPolicy)
		contentBudget := perEntryChars - len([]rune(prefix))
		if contentBudget <= 0 {
			contentBudget = perEntryChars
		}
		return prefix + compactPrepareTurnLine(text, contentBudget)
	}
	return compactPrepareTurnLine(text, perEntryChars)
}

func characterPrivateRecollectionPromptLineText(entry store.ProtagonistEntityMemory, perEntryChars int) string {
	text := strings.TrimSpace(entry.MemoryText)
	if text == "" {
		return ""
	}
	if characterPrivateRecollectionSecretGuardActive([]store.ProtagonistEntityMemory{entry}) {
		prefix := "Protected NPC-private hint: "
		text = protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.TargetRevealPolicy)
		contentBudget := perEntryChars - len([]rune(prefix))
		if contentBudget <= 0 {
			contentBudget = perEntryChars
		}
		return prefix + compactPrepareTurnLine(text, contentBudget)
	}
	prefix := "Private interpretation: "
	contentBudget := perEntryChars - len([]rune(prefix))
	if contentBudget <= 0 {
		contentBudget = perEntryChars
	}
	return prefix + compactPrepareTurnLine(text, contentBudget)
}

func protectedRecollectionGuardText(tagsJSON string, policyHints ...string) string {
	tags := []string{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(tagsJSON)), &tags); err != nil {
		tags = nil
	}
	kinds := []string{}
	policies := []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "protected_secret_kind:") {
			kinds = appendUniqueMemorySearchText(kinds, strings.TrimSpace(strings.TrimPrefix(tag, "protected_secret_kind:")))
		}
		if strings.HasPrefix(tag, "identity_kind:") {
			kinds = appendUniqueMemorySearchText(kinds, strings.TrimSpace(strings.TrimPrefix(tag, "identity_kind:")))
		}
		if strings.HasPrefix(tag, "target_reveal_policy:") {
			policies = appendUniqueMemorySearchText(policies, strings.TrimSpace(strings.TrimPrefix(tag, "target_reveal_policy:")))
		}
	}
	for _, hint := range policyHints {
		if policy := normalizeTargetRevealPolicy(hint); policy != "" && policy != "requires_explicit_attachment" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
	}
	parts := []string{"protected private knowledge is present; use only as owner subtext, hesitation, avoidance, or careful choice; do not reveal content without current evidence"}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	return strings.Join(parts, " | ")
}

func personaRecollectionSecretSafeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"previous loop", "protected private memory",
		"Previous loop", "Protected private memory",
		"time loop", "protected private memory",
		"Time loop", "Protected private memory",
		"regression", "protected private memory",
		"Regression", "Protected private memory",
		"regressor", "person with protected private memory",
		"Regressor", "Person with protected private memory",
		"reincarnation", "protected private memory",
		"Reincarnation", "Protected private memory",
		"reincarnated", "protected private memory",
		"Reincarnated", "Protected private memory",
		"past life", "protected private memory",
		"Past life", "Protected private memory",
		"isekai", "protected private memory",
		"Isekai", "Protected private memory",
		"other world", "protected private memory",
		"Other world", "Protected private memory",
		"another world", "protected private memory",
		"Another world", "Protected private memory",
		"이전 루프", "보호된 사적 기억",
		"지난 루프", "보호된 사적 기억",
		"루프", "보호된 사적 기억",
		"회귀", "보호된 사적 기억",
		"환생", "보호된 사적 기억",
		"전생", "보호된 사적 기억",
		"빙의", "보호된 사적 기억",
		"이세계", "보호된 사적 기억",
		"다른 세계", "보호된 사적 기억",
	)
	return strings.TrimSpace(replacer.Replace(text))
}

func personaRecollectionSecretGuardActive(entries []store.PersonaMemoryEntry) bool {
	for _, entry := range entries {
		source := strings.ToLower(strings.Join([]string{
			entry.MemoryText,
			entry.Portability,
			entry.InjectionPolicy,
			entry.TagsJSON,
		}, " "))
		if containsAnyText(source,
			"regression", "regressor", "regressed", "loop", "looper", "previous loop", "time loop",
			"reincarnation", "reincarnated", "past life", "isekai", "other world", "another world",
			"secret_guard", "identity carry-over", "identity carryover", "possession", "rebirth",
			"이전 루프", "지난 루프", "루프", "회귀", "환생", "전생", "빙의", "이세계", "다른 세계",
		) {
			return true
		}
	}
	return false
}

func characterPrivateRecollectionSecretGuardActive(entries []store.ProtagonistEntityMemory) bool {
	for _, entry := range entries {
		source := strings.ToLower(strings.Join([]string{
			entry.MemoryText,
			entry.Portability,
			entry.TargetRevealPolicy,
			entry.TagsJSON,
		}, " "))
		if entry.SecretGuard {
			return true
		}
		if containsAnyText(source,
			"regression", "regressor", "regressed", "loop", "looper", "previous loop", "time loop",
			"reincarnation", "reincarnated", "past life", "isekai", "other world", "another world",
			"이전 루프", "지난 루프", "루프", "회귀", "환생", "전생", "빙의", "이세계", "다른 세계",
		) {
			return true
		}
	}
	return false
}

func personaMemoryEntryIsCharacterPrivate(entry store.PersonaMemoryEntry) bool {
	source := strings.ToLower(strings.Join([]string{
		entry.Portability,
		entry.InjectionPolicy,
		entry.TagsJSON,
	}, " "))
	return strings.Contains(source, "npc_private") || strings.Contains(source, "character_private_recollection")
}

func personaMemoryEntryAsCharacterPrivateMemory(entry store.PersonaMemoryEntry, targetSID string) store.ProtagonistEntityMemory {
	tags := personaMemoryEntryTags(entry)
	ownerKey := personaMemoryEntryTagValue(tags, "owner_entity_key")
	ownerName := personaMemoryEntryTagValue(tags, "owner_entity_name")
	ownerRole := personaMemoryEntryTagValue(tags, "owner_entity_role")
	ownerVisibility := personaMemoryEntryTagValue(tags, "owner_visibility")
	sourceSID := personaMemoryEntryTagValue(tags, "source_chat_session_id")
	revealPolicy := personaMemoryEntryTagValue(tags, "target_reveal_policy")
	if ownerKey == "" {
		ownerKey = "npc"
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if ownerRole == "" {
		ownerRole = "npc"
	}
	if ownerVisibility == "" {
		ownerVisibility = "owner_private"
	}
	if sourceSID == "" {
		sourceSID = targetSID
	}
	if revealPolicy == "" {
		revealPolicy = "owner_private_until_revealed"
	}
	return store.ProtagonistEntityMemory{
		ID:                  entry.ID,
		OwnerEntityKey:      ownerKey,
		OwnerEntityName:     ownerName,
		OwnerEntityRole:     ownerRole,
		OwnerVisibility:     ownerVisibility,
		SourceChatSessionID: sourceSID,
		SourceTurn:          entry.SourceTurn,
		MemoryText:          entry.MemoryText,
		EvidenceExcerpt:     entry.EvidenceExcerpt,
		SecretGuard:         personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}) || personaMemoryEntryHasTag(tags, "secret_guard"),
		Portability:         firstNonEmpty(entry.Portability, "npc_private_recollection"),
		TargetRevealPolicy:  revealPolicy,
		TagsJSON:            entry.TagsJSON,
		Importance10:        entry.Importance10,
		EmotionalWeight:     entry.EmotionalWeight,
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.CreatedAt,
	}
}

func personaMemoryEntryTags(entry store.PersonaMemoryEntry) []string {
	var tags []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(entry.TagsJSON)), &tags); err == nil {
		return tags
	}
	return nil
}

func personaMemoryEntryTagValue(tags []string, key string) string {
	prefix := strings.TrimSpace(key) + ":"
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(tag, prefix))
		}
	}
	return ""
}

func personaMemoryEntryHasTag(tags []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, tag := range tags {
		if strings.TrimSpace(tag) == needle {
			return true
		}
	}
	return false
}

type prepareTurnRecollectionContext struct {
	rawUserInput       string
	immediateChatText  string
	currentSceneStates string
}

func filterPrepareTurnEntityRecollections(rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories *[]store.ProtagonistEntityMemory) map[string]any {
	const characterPrivateTotalCap = 2
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, activeStates, canonicalLayers)
	beforePrivate := len(*characterPrivateMemories)
	filteredPrivate := make([]store.ProtagonistEntityMemory, 0, beforePrivate)
	selectedOwners := []string{}
	selectedOwnerKeys := map[string]bool{}
	droppedOwners := []string{}
	dropped := []map[string]any{}
	for _, item := range *characterPrivateMemories {
		ownerKey := prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)
		if ownerKey != "" && selectedOwnerKeys[ownerKey] {
			owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
			if owner != "" && !stringSliceContains(droppedOwners, owner) {
				droppedOwners = append(droppedOwners, owner)
			}
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "owner_repetition_capped",
			})
			continue
		}
		if ok, reason := prepareTurnCharacterPrivateMemoryRelevant(item, ctx); ok {
			if len(filteredPrivate) >= characterPrivateTotalCap {
				owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
				if owner != "" && !stringSliceContains(droppedOwners, owner) {
					droppedOwners = append(droppedOwners, owner)
				}
				dropped = append(dropped, map[string]any{
					"id":                item.ID,
					"owner_entity_key":  item.OwnerEntityKey,
					"owner_entity_name": item.OwnerEntityName,
					"reason":            "private_recollection_total_capped",
				})
				continue
			}
			filteredPrivate = append(filteredPrivate, item)
			if ownerKey != "" {
				selectedOwnerKeys[ownerKey] = true
			}
			if owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName); owner != "" && !stringSliceContains(selectedOwners, owner) {
				selectedOwners = append(selectedOwners, owner)
			}
			continue
		} else {
			owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
			if owner != "" && !stringSliceContains(droppedOwners, owner) {
				droppedOwners = append(droppedOwners, owner)
			}
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            reason,
			})
		}
	}
	*characterPrivateMemories = filteredPrivate
	return map[string]any{
		"version":                         "pmc19.prepare_turn_entity_relevance.v1",
		"status":                          "active",
		"persona_recollection_count":      len(personaEntries),
		"persona_recollection_rule":       "protagonist_or_player_recollection_allowed_as_support_only_when_explicitly_attached",
		"character_private_before_filter": beforePrivate,
		"character_private_after_filter":  len(filteredPrivate),
		"character_private_dropped_count": beforePrivate - len(filteredPrivate),
		"character_private_gate":          "owner_entity_must_match_current_user_input_immediate_chat_or_current_scene_state",
		"character_private_owner_cap":     1,
		"character_private_total_cap":     characterPrivateTotalCap,
		"selected_owner_entities":         selectedOwners,
		"dropped_owner_entities":          droppedOwners,
		"dropped":                         dropped,
		"blocks_unrelated_session_memory": true,
		"blocks_unrelated_entity_memory":  true,
		"truth_authority":                 false,
		"canonical_write":                 false,
		"context_sources":                 []string{"current_user_input", "immediate_chat_tail", "latest_active_states"},
	}
}

func buildPrepareTurnRecollectionContext(rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer) prepareTurnRecollectionContext {
	_ = canonicalLayers
	immediate := []string{}
	start := len(chatLogs) - 2
	if start < 0 {
		start = 0
	}
	for _, item := range chatLogs[start:] {
		if text := strings.TrimSpace(item.Content); text != "" {
			immediate = append(immediate, text)
		}
	}
	state := []string{}
	latestStateTurn := 0
	for _, item := range activeStates {
		if item.TurnIndex > latestStateTurn {
			latestStateTurn = item.TurnIndex
		}
	}
	for _, item := range activeStates {
		if latestStateTurn > 0 && item.TurnIndex != latestStateTurn {
			continue
		}
		if text := strings.TrimSpace(item.Content); text != "" {
			state = append(state, text)
		}
	}
	return prepareTurnRecollectionContext{
		rawUserInput:       strings.TrimSpace(rawUserInput),
		immediateChatText:  strings.Join(immediate, "\n"),
		currentSceneStates: strings.Join(state, "\n"),
	}
}

func prepareTurnCharacterPrivateMemoryRelevant(item store.ProtagonistEntityMemory, ctx prepareTurnRecollectionContext) (bool, string) {
	ownerTokens := prepareTurnOwnerTokens(item.OwnerEntityKey, item.OwnerEntityName)
	if len(ownerTokens) == 0 {
		return false, "missing_owner_entity"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.rawUserInput) {
		return true, "explicit_current_user_input"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.immediateChatText) {
		return true, "immediate_chat_mention"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.currentSceneStates) {
		return true, "current_scene_state_mention"
	}
	return false, "owner_not_in_current_input_immediate_chat_or_current_state"
}

func prepareTurnOwnerTokens(ownerKey, ownerName string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range []string{ownerKey, ownerName, strings.ReplaceAll(ownerKey, "_", " "), strings.ReplaceAll(ownerName, "_", " ")} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, token := range []string{raw, normalizePrepareTurnEntityNeedle(raw)} {
			token = strings.TrimSpace(token)
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

func prepareTurnAnyOwnerTokenMatches(tokens []string, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	normalized := normalizePrepareTurnEntityNeedle(text)
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(token)) {
			return true
		}
		if normalized != "" && strings.Contains(normalized, normalizePrepareTurnEntityNeedle(token)) {
			return true
		}
	}
	return false
}

func normalizePrepareTurnEntityNeedle(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func prepareTurnMemoryOwnerLabel(ownerKey, ownerName string) string {
	if text := strings.TrimSpace(ownerName); text != "" {
		return text
	}
	return strings.TrimSpace(ownerKey)
}

func prepareTurnMemoryOwnerIdentity(ownerKey, ownerName string) string {
	for _, raw := range []string{ownerKey, ownerName} {
		if normalized := normalizePrepareTurnEntityNeedle(raw); normalized != "" {
			return normalized
		}
	}
	return ""
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func buildPersonaRecollectionSurface(sid string, entries []store.PersonaMemoryEntry, text string, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []map[string]any{}
	for i, entry := range entries {
		if i >= recallLimit {
			break
		}
		memoryText := strings.TrimSpace(entry.MemoryText)
		if memoryText == "" {
			continue
		}
		memoryText = strings.Join(strings.Fields(memoryText), " ")
		items = append(items, map[string]any{
			"id":                entry.ID,
			"capsule_id":        entry.CapsuleID,
			"source_turn_index": entry.SourceTurn,
			"memory_text":       memoryText,
			"importance_10":     entry.Importance10,
			"emotional_weight":  entry.EmotionalWeight,
			"portability":       entry.Portability,
			"injection_policy":  entry.InjectionPolicy,
			"secret_guard":      personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}),
		})
	}
	status := "empty"
	if len(items) > 0 {
		status = "ready"
	}
	secretGuardActive := personaRecollectionSecretGuardActive(entries)
	return map[string]any{
		"status":                 status,
		"target_chat_session_id": sid,
		"count":                  len(items),
		"text":                   nilIfEmpty(text),
		"items":                  items,
		"policy":                 personaRecollectionSupportPolicy(len(items) > 0),
		"secret_guard_active":    secretGuardActive,
		"secret_guard":           personaRecollectionSecretGuardPolicy(secretGuardActive),
		"would_write":            false,
		"would_call_llm":         false,
	}
}

func buildCharacterPrivateRecollectionSurface(sid string, entries []store.ProtagonistEntityMemory, text string, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []map[string]any{}
	for i, entry := range entries {
		if i >= recallLimit {
			break
		}
		memoryText := strings.TrimSpace(entry.MemoryText)
		if memoryText == "" {
			continue
		}
		memoryText = strings.Join(strings.Fields(memoryText), " ")
		items = append(items, map[string]any{
			"id":                   entry.ID,
			"owner_entity_key":     entry.OwnerEntityKey,
			"owner_entity_name":    entry.OwnerEntityName,
			"owner_entity_role":    entry.OwnerEntityRole,
			"owner_visibility":     entry.OwnerVisibility,
			"source_turn_index":    entry.SourceTurn,
			"memory_text":          memoryText,
			"importance_10":        entry.Importance10,
			"emotional_weight":     entry.EmotionalWeight,
			"portability":          entry.Portability,
			"target_reveal_policy": entry.TargetRevealPolicy,
			"secret_guard":         characterPrivateRecollectionSecretGuardActive([]store.ProtagonistEntityMemory{entry}),
		})
	}
	status := "empty"
	if len(items) > 0 {
		status = "ready"
	}
	secretGuardActive := characterPrivateRecollectionSecretGuardActive(entries)
	return map[string]any{
		"status":                       status,
		"target_chat_session_id":       sid,
		"count":                        len(items),
		"text":                         nilIfEmpty(text),
		"items":                        items,
		"policy":                       characterPrivateRecollectionPolicy(len(items) > 0),
		"secret_guard_active":          secretGuardActive,
		"secret_guard":                 personaRecollectionSecretGuardPolicy(secretGuardActive),
		"interpretation_not_fact":      true,
		"private_conflict_guard":       true,
		"visible_to_player":            false,
		"narrator_reveal_blocked":      true,
		"narrator_fact_reveal_blocked": true,
		"would_write":                  false,
		"would_call_llm":               false,
	}
}

func personaRecollectionSupportPolicy(active bool) map[string]any {
	return map[string]any{
		"active":                                active,
		"lane":                                  "persona_recollection",
		"authority":                             "support_only_persona_recollection",
		"truth_authority":                       false,
		"canonical_write":                       false,
		"current_world_fact":                    false,
		"priority_ceiling":                      "below_current_user_input_direct_evidence_and_canonical_state",
		"allowed_usage":                         []string{"subjective_memory_hint", "deja_vu_continuity", "loop_or_isekai_recollection"},
		"blocked_write_targets":                 []string{"memories", "kg_triples", "direct_evidence_records", "character_states", "world_rules", "canonical_state_layers"},
		"requires_current_session_confirmation": true,
		"secret_guard_active":                   active,
		"secret_guard":                          personaRecollectionSecretGuardPolicy(active),
	}
}

func characterPrivateRecollectionPolicy(active bool) map[string]any {
	return map[string]any{
		"active":                      active,
		"lane":                        "character_private_recollection",
		"authority":                   "support_only_npc_private_recollection",
		"truth_authority":             false,
		"canonical_write":             false,
		"current_world_fact":          false,
		"interpretation_not_fact":     true,
		"private_conflict_guard":      true,
		"ordinary_long_context_guard": true,
		"visible_to_player":           false,
		"narrator_reveal_blocked":     true,
		"narrator_must_not_confirm_private_memory": true,
		"priority_ceiling":                         "below_current_user_input_direct_evidence_and_canonical_state",
		"allowed_usage":                            []string{"npc_internal_bias", "hesitation", "recognition", "avoidance", "attraction", "suspicion", "careful_choice"},
		"allowed_expression":                       []string{"hesitation", "avoidance", "subtext", "misunderstanding", "conflicted_reaction", "selective_silence"},
		"blocked_usage":                            []string{"player_knowledge", "protagonist_knowledge", "narrator_reveal", "canonical_overwrite", "dialogue_confession_without_current_evidence", "objective_fact_from_private_recollection", "narrator_exposition_of_private_memory"},
		"blocked_write_targets":                    []string{"memories", "kg_triples", "direct_evidence_records", "character_states", "world_rules", "canonical_state_layers"},
		"requires_current_session_confirmation":    true,
		"reveal_requires":                          []string{"explicit_current_user_reveal_instruction", "current_session_direct_evidence", "owning_character_dialogue_or_action_in_current_turn"},
		"injection_gate":                           "owner_entity_must_match_current_user_input_recent_chat_or_current_scene_state",
		"blocks_unrelated_session_memory":          true,
		"blocks_unrelated_entity_memory":           true,
		"secret_guard_active":                      active,
		"secret_guard":                             personaRecollectionSecretGuardPolicy(active),
	}
}

func personaRecollectionSecretGuardPolicy(active bool) map[string]any {
	return map[string]any{
		"active": active,
		"protected_secret_types": []string{
			"regression",
			"loop",
			"reincarnation",
			"isekai_transfer",
			"possession_or_rebirth",
		},
		"allowed_expression": []string{
			"private_inner_recollection",
			"subtle_deja_vu",
			"uncertain_sensation",
			"protagonist_only_reasoning_hint",
		},
		"blocked_reveals": []string{
			"narrator_confirms_secret_identity",
			"npc_knows_without_current_evidence",
			"dialogue_announces_regressor_or_reincarnation",
			"canonical_world_fact_from_capsule_only",
		},
		"reveal_requires": []string{
			"explicit_current_user_reveal_instruction",
			"current_session_direct_evidence",
		},
	}
}
