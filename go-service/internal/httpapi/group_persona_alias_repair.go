package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type subjectiveEntityAliasRepairPlan struct {
	Scanned             int                                 `json:"scanned"`
	ReviewRequiredCount int                                 `json:"review_required_count"`
	Groups              []subjectiveEntityAliasRepairGroup  `json:"groups"`
	Changes             []subjectiveEntityAliasRepairChange `json:"changes"`
}

type subjectiveEntityAliasRepairGroup struct {
	CanonicalOwnerKey   string                              `json:"canonical_owner_key"`
	CanonicalOwnerName  string                              `json:"canonical_owner_name"`
	OwnerEntityRole     string                              `json:"owner_entity_role"`
	OwnerVisibility     string                              `json:"owner_visibility"`
	MemoryCount         int                                 `json:"memory_count"`
	RepairableCount     int                                 `json:"repairable_count"`
	ReviewRequiredCount int                                 `json:"review_required_count"`
	Decision            string                              `json:"decision"`
	EvidenceStatus      string                              `json:"evidence_status"`
	CandidateReasons    []string                            `json:"candidate_reasons"`
	Aliases             []subjectiveEntityAliasRepairAlias  `json:"aliases"`
	Changes             []subjectiveEntityAliasRepairChange `json:"changes"`
}

type subjectiveEntityAliasRepairAlias struct {
	OwnerEntityKey  string   `json:"owner_entity_key"`
	OwnerEntityName string   `json:"owner_entity_name"`
	Count           int      `json:"count"`
	MemoryIDs       []int64  `json:"memory_ids"`
	SourceTurns     []int    `json:"source_turns"`
	MemoryPreviews  []string `json:"memory_previews"`
}

type subjectiveEntityAliasRepairChange struct {
	ID             int64  `json:"id"`
	FromOwnerKey   string `json:"from_owner_key"`
	FromOwnerName  string `json:"from_owner_name"`
	ToOwnerKey     string `json:"to_owner_key"`
	ToOwnerName    string `json:"to_owner_name"`
	SourceTurn     int    `json:"source_turn_index"`
	Decision       string `json:"decision"`
	EvidenceReason string `json:"evidence_reason"`
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
			group.Aliases[idx].SourceTurns = appendUniqueInt(group.Aliases[idx].SourceTurns, memory.SourceTurn)
			preview := truncateRunes(strings.TrimSpace(memory.MemoryText), 120)
			if preview != "" {
				group.Aliases[idx].MemoryPreviews = appendUniqueString(group.Aliases[idx].MemoryPreviews, preview)
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
				SourceTurns:     []int{memory.SourceTurn},
				MemoryPreviews:  []string{truncateRunes(strings.TrimSpace(memory.MemoryText), 120)},
			})
		}
		if !subjectiveEntityMemoryOwnerNeedsRepair(memory, canonicalMemory) {
			continue
		}
		decision, evidenceReason := subjectiveEntityAliasRepairDecision(memory)
		group.CandidateReasons = appendUniqueString(group.CandidateReasons, evidenceReason)
		if decision != "confirmed_auto_apply" {
			group.ReviewRequiredCount++
			plan.ReviewRequiredCount++
			continue
		}
		change := subjectiveEntityAliasRepairChange{
			ID:             memory.ID,
			FromOwnerKey:   originalOwnerKey,
			FromOwnerName:  originalOwnerName,
			ToOwnerKey:     canonicalKey,
			ToOwnerName:    canonicalName,
			SourceTurn:     memory.SourceTurn,
			Decision:       decision,
			EvidenceReason: evidenceReason,
			TargetTagsJSON: subjectiveEntityAliasRepairTags(memory, canonicalMemory),
		}
		group.RepairableCount++
		group.Changes = append(group.Changes, change)
		plan.Changes = append(plan.Changes, change)
	}
	for _, key := range order {
		group := groups[key]
		switch {
		case group.RepairableCount > 0 && group.ReviewRequiredCount > 0:
			group.Decision = "mixed"
			group.EvidenceStatus = "partially_confirmed"
		case group.RepairableCount > 0:
			group.Decision = "confirmed_auto_apply"
			group.EvidenceStatus = "confirmed_identity_evidence"
		case group.ReviewRequiredCount > 0:
			group.Decision = "review_required"
			group.EvidenceStatus = "name_similarity_only"
		default:
			group.Decision = "clean"
			group.EvidenceStatus = "no_change"
		}
		if group.RepairableCount == 0 && len(group.Aliases) <= 1 {
			continue
		}
		plan.Groups = append(plan.Groups, group.subjectiveEntityAliasRepairGroup)
	}
	return plan
}

func subjectiveEntityAliasRepairDecision(memory store.ProtagonistEntityMemory) (string, string) {
	if subjectiveEntityMemoryHasAnyTag(memory, "confirmed_identity_alias_canonicalized") {
		if strings.TrimSpace(memory.EvidenceExcerpt) != "" {
			return "confirmed_auto_apply", "confirmed_same_entity_record_with_grounded_evidence"
		}
		return "review_required", "confirmed_identity_tag_without_grounded_evidence"
	}
	if subjectiveEntityMemoryHasAnyTag(memory, "entity_alias_repaired", "entity_force_merged") {
		return "confirmed_auto_apply", "previous_explicit_user_merge"
	}
	return "review_required", "name_similarity_without_confirmed_identity"
}

func appendUniqueInt(items []int, value int) []int {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
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
		"surface":                    "subjective_entity_alias_repair",
		"default_mode":               "dry_run",
		"apply_requires_explicit":    true,
		"mutation_scope":             "owner_persona_identity_fields_only",
		"memory_text_mutation":       false,
		"evidence_mutation":          false,
		"delete_duplicate_rows":      false,
		"merge_mode":                 "canonical_owner_key_rewrite_no_delete",
		"automatic_apply_gate":       "confirmed_identity_tag_plus_grounded_evidence_or_previous_explicit_merge",
		"memory_text_is_proof":       false,
		"name_similarity_is_proof":   false,
		"ambiguous_candidate_action": "review_or_manual_force_merge",
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
