package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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
		scopeVariants   map[string]bool
		count           int
		latestTurn      int
		latestText      string
		maxImportance   float64
	}
	memories = s.canonicalizeSubjectiveEntityMemoriesForRead(ctx, sourceSID, memories)
	explicitMergedOwnerKeys := map[string]bool{}
	for _, memory := range memories {
		ownerKey := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey))
		if ownerKey != "" && subjectiveEntityMemoryHasAnyTag(memory, "entity_alias_repaired", "entity_force_merged") {
			explicitMergedOwnerKeys[ownerKey] = true
		}
	}
	order := []string{}
	groups := map[string]*groupState{}
	for _, memory := range memories {
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
		ownerName := strings.TrimSpace(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName, ownerKey))
		key := "name:" + comparableEntityKey(ownerName)
		if explicitMergedOwnerKeys[ownerKey] {
			key = "explicit:" + ownerKey
		} else if key == "name:" {
			key = "key:" + ownerKey
		}
		group := groups[key]
		if group == nil {
			group = &groupState{
				ownerKey:        ownerKey,
				ownerName:       ownerName,
				ownerRole:       ownerRole,
				ownerVisibility: ownerVisibility,
				scopeVariants:   map[string]bool{},
				latestTurn:      memory.SourceTurn,
				latestText:      truncateRunes(strings.TrimSpace(memory.MemoryText), 180),
				maxImportance:   memory.Importance10,
			}
			groups[key] = group
			order = append(order, key)
		}
		group.scopeVariants[ownerRole+"\x1f"+ownerVisibility] = true
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
			"scope_variant_count":    len(group.scopeVariants),
			"mixed_owner_scope":      len(group.scopeVariants) > 1,
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

func subjectiveEntityMemoryHasAnyTag(memory store.ProtagonistEntityMemory, needles ...string) bool {
	var tags []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(memory.TagsJSON)), &tags); err != nil {
		return false
	}
	for _, tag := range tags {
		for _, needle := range needles {
			if strings.TrimSpace(tag) == needle {
				return true
			}
		}
	}
	return false
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
