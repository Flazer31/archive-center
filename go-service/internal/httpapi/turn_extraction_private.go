package httpapi

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func normalizeSubjectiveEntityMemories(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		memory := mapFromAny(item)
		ownerName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "owner_entity_name"),
			stringFromMap(memory, "entity_name"),
			stringFromMap(memory, "name"),
			stringFromMap(memory, "persona_entity_name"),
		))
		ownerKey := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "owner_entity_key"),
			stringFromMap(memory, "entity_key"),
			stringFromMap(memory, "persona_entity_key"),
			normalizeCharacterKey(ownerName),
		))
		text := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(memory, "memory_text"),
			stringFromMap(memory, "subjective_memory"),
			stringFromMap(memory, "recollection"),
			stringFromMap(memory, "interpretation"),
			stringFromMap(memory, "summary"),
			stringFromMap(memory, "text"),
		))
		if ownerKey == "" || text == "" {
			continue
		}
		if ownerName == "" {
			ownerName = ownerKey
		}
		role := normalizeSubjectiveEntityRoleFilter(stringFromMap(memory, "owner_entity_role"))
		if role == "" {
			role = normalizeSubjectiveEntityRoleFilter(stringFromMap(memory, "entity_role"))
		}
		if role == "" {
			role = "protagonist"
		}
		visibility := normalizeSubjectiveEntityVisibilityFilter(stringFromMap(memory, "owner_visibility"))
		if visibility == "" {
			visibility = normalizeSubjectiveEntityVisibilityFilter(stringFromMap(memory, "visibility"))
		}
		if visibility == "" && role == "npc" {
			visibility = "owner_private"
		}
		if visibility == "" {
			visibility = "player_known"
		}
		targetRevealPolicy := normalizeTargetRevealPolicy(stringFromMap(memory, "target_reveal_policy"))
		if strings.TrimSpace(stringFromMap(memory, "target_reveal_policy")) == "" && (role == "npc" || visibility == "owner_private") {
			targetRevealPolicy = "owner_private_until_revealed"
		}
		portability := strings.ToLower(strings.TrimSpace(stringFromMap(memory, "portability")))
		switch portability {
		case "portable_subjective_entity_recollection", "portable_persona_recollection", "npc_private_recollection":
		default:
			if role == "npc" || visibility == "owner_private" {
				portability = "npc_private_recollection"
			} else {
				portability = "portable_subjective_entity_recollection"
			}
		}
		out = append(out, map[string]any{
			"owner_entity_key":     ownerKey,
			"owner_entity_name":    ownerName,
			"owner_entity_role":    role,
			"owner_visibility":     visibility,
			"memory_text":          text,
			"source_turn_index":    intFromAny(memory["source_turn_index"], 0),
			"importance_10":        clampFloat(extractionFloatFromAny(memory["importance_10"], extractionFloatFromAny(memory["importance_score"], 5)), 1, 10),
			"emotional_weight":     clampFloat(extractionFloatFromAny(memory["emotional_weight"], extractionFloatFromAny(memory["emotional_intensity"], 0.5)), 0, 1),
			"evidence_excerpt":     strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "evidence_excerpt"), stringFromMap(memory, "evidence"))),
			"secret_guard":         boolFromAny(memory["secret_guard"]),
			"target_reveal_policy": targetRevealPolicy,
			"tags":                 stringsFromAny(memory["tags"]),
			"portability":          portability,
		})
	}
	return out
}

func normalizeProtectedSecrets(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		secret := mapFromAny(item)
		kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(
			stringFromMap(secret, "secret_kind"),
			stringFromMap(secret, "kind"),
			stringFromMap(secret, "protected_secret_type"),
		))
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(secret, "owner"),
			stringFromMap(secret, "owner_entity_name"),
			stringFromMap(secret, "character_name"),
			firstStringFromAny(mapFromAny(secret["knowledge_scope"])["known_by"]),
		))
		subjects := stringsFromAny(secret["subject"])
		if len(subjects) == 0 {
			subject := strings.TrimSpace(extractionFirstNonEmpty(
				stringFromMap(secret, "subject"),
				stringFromMap(secret, "target"),
				stringFromMap(secret, "topic"),
			))
			if subject != "" {
				subjects = []string{subject}
			}
		}
		summary := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(secret, "summary"),
			stringFromMap(secret, "memory_text"),
			stringFromMap(secret, "secret_summary"),
			stringFromMap(secret, "text"),
		))
		if owner == "" && len(subjects) > 0 {
			owner = subjects[0]
		}
		if summary == "" || owner == "" {
			continue
		}
		knowledgeScope := normalizeProtectedSecretKnowledgeScope(secret["knowledge_scope"], owner)
		disclosurePolicy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(
			stringFromMap(secret, "disclosure_policy"),
			stringFromMap(secret, "target_reveal_policy"),
			stringFromMap(secret, "reveal_policy"),
		))
		if disclosurePolicy == "" || disclosurePolicy == "requires_explicit_attachment" {
			disclosurePolicy = "owner_private_until_revealed"
		}
		out = append(out, map[string]any{
			"contract_version":         "protected_secret.v1",
			"secret_kind":              firstNonEmpty(kind, "other"),
			"owner":                    owner,
			"subject":                  subjects,
			"summary":                  summary,
			"sensitivity":              normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")),
			"evidence_strength":        normalizeProtectedSecretToken(stringFromMap(secret, "evidence_strength")),
			"disclosure_policy":        disclosurePolicy,
			"knowledge_scope":          knowledgeScope,
			"evidence_excerpt":         strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(secret, "evidence_excerpt"), stringFromMap(secret, "evidence"))),
			"raw_evidence_rewritten":   false,
			"public_narration_allowed": boolFromAny(secret["public_narration_allowed"]),
		})
	}
	return out
}

func normalizeCharacterIdentityAccuracy(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		identity := mapFromAny(item)
		surface := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		))
		trueName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "real_identity_name"),
		))
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			trueName,
			surface,
		))
		kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(
			stringFromMap(identity, "identity_kind"),
			stringFromMap(identity, "kind"),
			stringFromMap(identity, "protected_secret_type"),
		))
		if owner == "" || (surface == "" && trueName == "" && kind == "") {
			continue
		}
		revealPolicy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(
			stringFromMap(identity, "reveal_policy"),
			stringFromMap(identity, "target_reveal_policy"),
			stringFromMap(identity, "disclosure_policy"),
		))
		if revealPolicy == "" || revealPolicy == "requires_explicit_attachment" {
			revealPolicy = "owner_private_until_revealed"
		}
		knowledgeScope := normalizeProtectedSecretKnowledgeScope(identity["knowledge_scope"], owner)
		out = append(out, map[string]any{
			"contract_version":       "character_identity_accuracy.v1",
			"canonical_entity_key":   normalizeCharacterKey(owner),
			"canonical_entity_name":  owner,
			"surface_identity_name":  surface,
			"true_identity_name":     trueName,
			"aliases":                stringsFromAny(identity["aliases"]),
			"identity_kind":          firstNonEmpty(kind, "identity"),
			"same_entity":            boolFromAny(identity["same_entity"]),
			"public_role":            strings.TrimSpace(stringFromMap(identity, "public_role")),
			"true_role":              strings.TrimSpace(stringFromMap(identity, "true_role")),
			"public_allegiance":      strings.TrimSpace(stringFromMap(identity, "public_allegiance")),
			"true_allegiance":        strings.TrimSpace(stringFromMap(identity, "true_allegiance")),
			"twist_sensitivity":      normalizeProtectedSecretToken(stringFromMap(identity, "twist_sensitivity")),
			"reveal_policy":          revealPolicy,
			"visibility":             extractionFirstNonEmpty(stringFromMap(identity, "visibility"), "internal_support_only"),
			"knowledge_scope":        knowledgeScope,
			"source_evidence_turns":  intsFromAny(identity["source_evidence_turns"]),
			"raw_evidence_rewritten": false,
		})
	}
	return out
}

type confirmedIdentityAliasMap struct {
	aliasToCanonical      map[string]string
	aliasesByCanonicalKey map[string][]string
	conflictedAliasKeys   map[string]bool
}

func buildConfirmedIdentityAliasMapFromExtraction(extraction map[string]any) confirmedIdentityAliasMap {
	out := confirmedIdentityAliasMap{
		aliasToCanonical:      map[string]string{},
		aliasesByCanonicalKey: map[string][]string{},
		conflictedAliasKeys:   map[string]bool{},
	}
	addAlias := func(alias, canonical string) {
		alias = strings.TrimSpace(alias)
		canonical = strings.TrimSpace(canonical)
		aliasKey := normalizeCharacterKey(alias)
		canonicalKey := normalizeCharacterKey(canonical)
		if aliasKey == "" || canonicalKey == "" || out.conflictedAliasKeys[aliasKey] {
			return
		}
		if existing, ok := out.aliasToCanonical[aliasKey]; ok && normalizeCharacterKey(existing) != canonicalKey {
			delete(out.aliasToCanonical, aliasKey)
			out.conflictedAliasKeys[aliasKey] = true
			return
		}
		out.aliasToCanonical[aliasKey] = canonical
		if aliasKey != canonicalKey {
			out.aliasesByCanonicalKey[canonicalKey] = appendUniqueIdentityAlias(out.aliasesByCanonicalKey[canonicalKey], alias)
		}
	}
	for _, raw := range sliceFromAny(extraction["character_identity_accuracy"]) {
		identity := mapFromAny(raw)
		if !boolFromAny(identity["same_entity"]) {
			continue
		}
		canonical := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "true_identity_name"),
		))
		if canonical == "" {
			continue
		}
		addAlias(canonical, canonical)
		addAlias(stringFromMap(identity, "true_identity_name"), canonical)
		addAlias(stringFromMap(identity, "surface_identity_name"), canonical)
		for _, alias := range stringsFromAny(identity["aliases"]) {
			addAlias(alias, canonical)
		}
	}
	return out
}

func (m confirmedIdentityAliasMap) empty() bool {
	return len(m.aliasToCanonical) == 0
}

func (m confirmedIdentityAliasMap) canonicalizeName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || len(m.aliasToCanonical) == 0 {
		return name, false
	}
	key := normalizeCharacterKey(name)
	if key == "" || m.conflictedAliasKeys[key] {
		return name, false
	}
	canonical := strings.TrimSpace(m.aliasToCanonical[key])
	if canonical == "" || normalizeCharacterKey(canonical) == key {
		return name, false
	}
	return canonical, true
}

func (m confirmedIdentityAliasMap) aliasesForCanonical(canonical string) []string {
	key := normalizeCharacterKey(canonical)
	if key == "" {
		return nil
	}
	return append([]string{}, m.aliasesByCanonicalKey[key]...)
}

func applyConfirmedIdentityAliasCanonicalMerge(extraction map[string]any) (map[string]any, int) {
	aliases := buildConfirmedIdentityAliasMapFromExtraction(extraction)
	if aliases.empty() {
		return extraction, 0
	}
	applied := 0
	canonicalizeField := func(item map[string]any, field string) bool {
		raw := stringFromMap(item, field)
		canonical, changed := aliases.canonicalizeName(raw)
		if !changed {
			return false
		}
		item[field] = canonical
		applied++
		return true
	}
	addEntityAliases := func(entity map[string]any, rawName, canonical string) {
		values := stringsFromAny(entity["aliases"])
		values = appendUniqueIdentityAlias(values, rawName)
		for _, alias := range aliases.aliasesForCanonical(canonical) {
			values = appendUniqueIdentityAlias(values, alias)
		}
		if len(values) > 0 {
			entity["aliases"] = values
		}
		entity["identity_canonicalized"] = true
	}
	entities := mapFromAny(extraction["entities"])
	if len(entities) > 0 {
		for _, rawEntity := range sliceFromAny(entities["characters"]) {
			entity := mapFromAny(rawEntity)
			rawName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title")))
			canonical, changed := aliases.canonicalizeName(rawName)
			if !changed {
				continue
			}
			entity["name"] = canonical
			addEntityAliases(entity, rawName, canonical)
			applied++
		}
		extraction["entities"] = entities
	}
	for _, raw := range sliceFromAny(extraction["kg_triples"]) {
		triple := mapFromAny(raw)
		canonicalizeField(triple, "subject")
		canonicalizeField(triple, "object")
	}
	for _, raw := range sliceFromAny(extraction["character_deltas"]) {
		delta := mapFromAny(raw)
		if rawName := stringFromMap(delta, "name"); canonicalizeField(delta, "name") {
			delta["aliases"] = appendUniqueIdentityAlias(stringsFromAny(delta["aliases"]), rawName)
		}
	}
	for _, raw := range sliceFromAny(extraction["subjective_entity_memories"]) {
		memory := mapFromAny(raw)
		rawOwnerName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "owner_entity_name"), stringFromMap(memory, "persona_entity_name")))
		rawOwnerKey := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(memory, "owner_entity_key"), stringFromMap(memory, "persona_entity_key")))
		canonical, changed := aliases.canonicalizeName(firstNonEmpty(rawOwnerName, rawOwnerKey))
		if !changed {
			continue
		}
		canonicalKey := normalizeCharacterKey(canonical)
		memory["owner_entity_name"] = canonical
		memory["persona_entity_name"] = canonical
		memory["owner_entity_key"] = canonicalKey
		memory["persona_entity_key"] = canonicalKey
		tags := stringsFromAny(memory["tags"])
		tags = appendUniqueIdentityAlias(tags, "confirmed_identity_alias_canonicalized")
		if rawOwnerName != "" {
			tags = appendUniqueIdentityAlias(tags, "raw_owner_entity_name:"+rawOwnerName)
		}
		if rawOwnerKey != "" {
			tags = appendUniqueIdentityAlias(tags, "raw_owner_entity_key:"+rawOwnerKey)
		}
		for _, alias := range aliases.aliasesForCanonical(canonical) {
			tags = appendUniqueIdentityAlias(tags, "owner_entity_alias:"+alias)
		}
		memory["tags"] = tags
		applied++
	}
	for _, raw := range sliceFromAny(extraction["protected_secrets"]) {
		secret := mapFromAny(raw)
		canonicalizeField(secret, "owner")
		subjects := stringsFromAny(secret["subject"])
		for idx, subject := range subjects {
			if canonical, changed := aliases.canonicalizeName(subject); changed {
				subjects[idx] = canonical
				applied++
			}
		}
		if len(subjects) > 0 {
			secret["subject"] = subjects
		}
		scope := mapFromAny(secret["knowledge_scope"])
		if len(scope) > 0 {
			for _, key := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
				values := stringsFromAny(scope[key])
				changed := false
				for idx, value := range values {
					if canonical, ok := aliases.canonicalizeName(value); ok {
						values[idx] = canonical
						changed = true
						applied++
					}
				}
				if changed {
					scope[key] = values
				}
			}
			secret["knowledge_scope"] = scope
		}
	}
	if applied > 0 {
		extraction["confirmed_identity_alias_canonical_merge"] = map[string]any{
			"contract_version": "character_identity_canonical_merge.v1",
			"applied":          true,
			"applied_count":    applied,
			"conflicts":        len(aliases.conflictedAliasKeys),
		}
	}
	return extraction, applied
}

func appendUniqueIdentityAlias(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return items
		}
	}
	return append(items, value)
}

func appendProtectedSecretSubjectiveMemories(existing []any, secrets []any) []any {
	out := append([]any{}, existing...)
	for _, raw := range secrets {
		secret := mapFromAny(raw)
		owner := strings.TrimSpace(stringFromMap(secret, "owner"))
		summary := strings.TrimSpace(stringFromMap(secret, "summary"))
		if owner == "" || summary == "" {
			continue
		}
		out = appendSubjectiveMemoryIfMissing(out, map[string]any{
			"owner_entity_key":     normalizeCharacterKey(owner),
			"owner_entity_name":    owner,
			"owner_entity_role":    "npc",
			"owner_visibility":     "owner_private",
			"memory_text":          summary,
			"importance_10":        protectedSecretImportance(secret),
			"emotional_weight":     protectedSecretEmotionalWeight(secret),
			"evidence_excerpt":     strings.TrimSpace(stringFromMap(secret, "evidence_excerpt")),
			"secret_guard":         true,
			"target_reveal_policy": normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")),
			"tags":                 protectedSecretTags(secret),
			"portability":          "npc_private_recollection",
		})
	}
	return out
}

func appendIdentityAccuracySubjectiveMemories(existing []any, identities []any) []any {
	out := append([]any{}, existing...)
	for _, raw := range identities {
		identity := mapFromAny(raw)
		owner := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "surface_identity_name"),
		))
		if owner == "" {
			continue
		}
		out = appendSubjectiveMemoryIfMissing(out, map[string]any{
			"owner_entity_key":     normalizeCharacterKey(owner),
			"owner_entity_name":    owner,
			"owner_entity_role":    "npc",
			"owner_visibility":     "owner_private",
			"memory_text":          protectedIdentityGuardSummary(identity),
			"importance_10":        8.0,
			"emotional_weight":     0.7,
			"secret_guard":         true,
			"target_reveal_policy": normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")),
			"tags":                 protectedIdentityTags(identity),
			"portability":          "npc_private_recollection",
		})
	}
	return out
}

func appendSubjectiveMemoryIfMissing(items []any, next map[string]any) []any {
	owner := strings.ToLower(strings.TrimSpace(stringFromMap(next, "owner_entity_key")))
	text := strings.ToLower(strings.TrimSpace(stringFromMap(next, "memory_text")))
	for _, raw := range items {
		item := mapFromAny(raw)
		if strings.ToLower(strings.TrimSpace(stringFromMap(item, "owner_entity_key"))) == owner &&
			strings.ToLower(strings.TrimSpace(stringFromMap(item, "memory_text"))) == text {
			return items
		}
	}
	return append(items, next)
}

func normalizeProtectedSecretKnowledgeScope(raw any, owner string) map[string]any {
	scope := mapFromAny(raw)
	out := map[string]any{
		"publicly_revealed":   boolFromAny(scope["publicly_revealed"]),
		"known_by":            stringsFromAny(scope["known_by"]),
		"unknown_to":          stringsFromAny(scope["unknown_to"]),
		"suspected_by":        stringsFromAny(scope["suspected_by"]),
		"misinformed_by":      stringsFromAny(scope["misinformed_by"]),
		"revealed_to":         stringsFromAny(scope["revealed_to"]),
		"reader_visible":      boolFromAny(scope["reader_visible"]),
		"protagonist_visible": boolFromAny(scope["protagonist_visible"]),
	}
	if len(stringsFromAny(out["known_by"])) == 0 && strings.TrimSpace(owner) != "" {
		out["known_by"] = []string{owner}
	}
	return out
}

func protectedSecretTags(secret map[string]any) []string {
	tags := []string{"protected_secret", "secret_guard"}
	if kind := normalizeProtectedSecretToken(stringFromMap(secret, "secret_kind")); kind != "" {
		tags = append(tags, "protected_secret_kind:"+kind)
	}
	if sensitivity := normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")); sensitivity != "" {
		tags = append(tags, "sensitivity:"+sensitivity)
	}
	if policy := normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")); policy != "" {
		tags = append(tags, "target_reveal_policy:"+policy)
	}
	return tags
}

func protectedIdentityTags(identity map[string]any) []string {
	tags := []string{"protected_secret", "character_identity_accuracy", "secret_guard"}
	if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
		tags = append(tags, "identity_kind:"+kind)
		tags = append(tags, "protected_secret_kind:"+kind)
	}
	if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
		tags = append(tags, "target_reveal_policy:"+policy)
	}
	return tags
}

func protectedIdentityGuardSummary(identity map[string]any) string {
	kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind"))
	if kind == "" {
		kind = "identity"
	}
	return "Protected identity/role knowledge is present; preserve same-entity continuity internally, but do not reveal, confess, or grant knowledge without current-scene evidence. kind=" + kind
}

func protectedSecretImportance(secret map[string]any) float64 {
	switch normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")) {
	case "critical":
		return 9
	case "high":
		return 8
	case "medium":
		return 6
	default:
		return 5
	}
}

func protectedSecretEmotionalWeight(secret map[string]any) float64 {
	switch normalizeProtectedSecretToken(stringFromMap(secret, "sensitivity")) {
	case "critical":
		return 0.9
	case "high":
		return 0.75
	case "medium":
		return 0.55
	default:
		return 0.35
	}
}

func normalizeProtectedSecretToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func firstStringFromAny(value any) string {
	values := stringsFromAny(value)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func intsFromAny(value any) []int {
	out := []int{}
	for _, item := range sliceFromAny(value) {
		n := intFromAny(item, 0)
		if n != 0 {
			out = append(out, n)
		}
	}
	return out
}

type subjectiveEntityOwnerCanonical struct {
	Key       string
	Name      string
	AliasTags []string
	Changed   bool
}

func (s *Server) canonicalSubjectiveEntityOwner(ctx context.Context, sid, rawKey, rawName string) subjectiveEntityOwnerCanonical {
	rawKey = strings.TrimSpace(rawKey)
	rawName = strings.TrimSpace(rawName)
	proposed := strings.TrimSpace(firstNonEmpty(rawName, rawKey))
	canonicalName := proposed
	if proposed != "" {
		canonicalName = strings.TrimSpace(s.canonicalCharacterName(ctx, sid, proposed))
	}
	if canonicalName == "" {
		canonicalName = proposed
	}
	canonicalKey := canonicalCharacterAliasKey(canonicalName)
	if canonicalKey == "" {
		canonicalKey = canonicalCharacterAliasKey(rawKey)
	}
	if canonicalKey == "" {
		canonicalKey = canonicalCharacterAliasKey(rawName)
	}
	if canonicalKey == "" {
		canonicalKey = rawKey
	}
	if canonicalName == "" {
		canonicalName = canonicalKey
	}
	out := subjectiveEntityOwnerCanonical{
		Key:  canonicalKey,
		Name: canonicalName,
	}
	addAlias := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if prefix == "owner_entity_alias:" && strings.EqualFold(value, canonicalName) {
			return
		}
		if prefix == "owner_entity_alias_key:" && value == canonicalKey {
			return
		}
		tag := prefix + value
		for _, existing := range out.AliasTags {
			if existing == tag {
				return
			}
		}
		out.AliasTags = append(out.AliasTags, tag)
		out.Changed = true
	}
	addAlias("owner_entity_alias:", rawName)
	addAlias("owner_entity_alias_key:", rawKey)
	return out
}

func (s *Server) canonicalizeSubjectiveEntityMemoryForRead(ctx context.Context, sid string, memory store.ProtagonistEntityMemory) store.ProtagonistEntityMemory {
	owner := s.canonicalSubjectiveEntityOwner(ctx, sid, firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey), firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName))
	if owner.Key == "" {
		return memory
	}
	memory.OwnerEntityKey = owner.Key
	memory.PersonaEntityKey = owner.Key
	if owner.Name != "" {
		memory.OwnerEntityName = owner.Name
		memory.PersonaEntityName = owner.Name
		if strings.TrimSpace(memory.SourceCharacterName) == "" {
			memory.SourceCharacterName = owner.Name
		}
	}
	return memory
}

func (s *Server) canonicalizeSubjectiveEntityMemoriesForRead(ctx context.Context, sid string, memories []store.ProtagonistEntityMemory) []store.ProtagonistEntityMemory {
	out := make([]store.ProtagonistEntityMemory, 0, len(memories))
	for _, memory := range memories {
		out = append(out, s.canonicalizeSubjectiveEntityMemoryForRead(ctx, sid, memory))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SourceTurn != out[j].SourceTurn {
			return out[i].SourceTurn > out[j].SourceTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func normalizePersonaCapsuleCandidates(raw any) []any {
	out := []any{}
	for _, item := range sliceFromAny(raw) {
		candidate := mapFromAny(item)
		memoryText := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(candidate, "memory_text"),
			stringFromMap(candidate, "summary"),
			stringFromMap(candidate, "text"),
		))
		evidence := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(candidate, "evidence_excerpt"),
			stringFromMap(candidate, "evidence"),
		))
		if memoryText == "" || evidence == "" {
			continue
		}
		portability := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "portability")))
		switch portability {
		case "same_session", "cross_session", "cross_world", "cross_chat":
		default:
			portability = "cross_session"
		}
		mode := strings.ToLower(strings.TrimSpace(stringFromMap(candidate, "mode")))
		switch mode {
		case "subtle_deja_vu", "full_loop_memory", "isekai_carryover", "same_character_continuation", "regression_recollection", "reincarnation_carryover":
		default:
			mode = "same_character_continuation"
		}
		injectionPolicy := strings.TrimSpace(stringFromMap(candidate, "injection_policy"))
		if injectionPolicy == "" {
			injectionPolicy = "support_only_persona_recollection"
		}
		normalized := map[string]any{
			"memory_text":       memoryText,
			"source_turn_index": intFromAny(candidate["source_turn_index"], 0),
			"importance_10":     clampFloat(extractionFloatFromAny(candidate["importance_10"], extractionFloatFromAny(candidate["importance_score"], 5)), 1, 10),
			"emotional_weight":  clampFloat(extractionFloatFromAny(candidate["emotional_weight"], extractionFloatFromAny(candidate["emotional_intensity"], 0.5)), 0, 1),
			"portability":       portability,
			"mode":              mode,
			"secret_guard":      boolFromAny(candidate["secret_guard"]),
			"tags":              stringsFromAny(candidate["tags"]),
			"evidence_excerpt":  evidence,
			"injection_policy":  injectionPolicy,
		}
		out = append(out, normalized)
	}
	return out
}

func recordPersonaCapsuleCandidateTrace(extraction map[string]any, turnIndex int, result *artifactSaveResult) {
	if result == nil {
		return
	}
	candidates := sliceFromAny(extraction["persona_capsule_candidates"])
	if len(candidates) == 0 {
		return
	}
	result.PersonaCapsuleCandidates = len(candidates)
	result.Warnings = append(result.Warnings, "persona_capsule_candidates_detected:auto_create_disabled")
	for idx, item := range candidates {
		candidate := mapFromAny(item)
		result.addSkipReason("persona_capsule_candidates", "requires_explicit_user_or_operator_approval", map[string]any{
			"candidate_index":   idx,
			"source_turn_index": intFromAny(candidate["source_turn_index"], turnIndex),
			"portability":       stringFromMap(candidate, "portability"),
			"mode":              stringFromMap(candidate, "mode"),
			"injection_policy":  stringFromMap(candidate, "injection_policy"),
			"secret_guard":      boolFromAny(candidate["secret_guard"]),
		})
	}
}

func (s *Server) saveSubjectiveEntityMemoriesFromExtraction(ctx context.Context, sid string, turnIndex int, extraction map[string]any, content string, now time.Time, result *artifactSaveResult) {
	if result == nil {
		return
	}
	items := sliceFromAny(extraction["subjective_entity_memories"])
	if len(items) == 0 {
		return
	}
	if s.Store == nil {
		result.addSkipReason("subjective_entity_memories", "store_unavailable", map[string]any{"count": len(items)})
		return
	}
	st, ok := s.Store.(store.ProtagonistEntityMemoryStore)
	if !ok {
		result.addSkipReason("subjective_entity_memories", "store_not_supported", map[string]any{"count": len(items)})
		return
	}
	for idx, raw := range items {
		item := mapFromAny(raw)
		rawOwnerKey := strings.TrimSpace(stringFromMap(item, "owner_entity_key"))
		rawOwnerName := strings.TrimSpace(stringFromMap(item, "owner_entity_name"))
		ownerKey := rawOwnerKey
		ownerName := rawOwnerName
		memoryText := strings.TrimSpace(stringFromMap(item, "memory_text"))
		if ownerKey == "" || memoryText == "" {
			result.addSkipReason("subjective_entity_memories", "missing_owner_or_memory_text", map[string]any{"index": idx})
			continue
		}
		if ownerName == "" {
			ownerName = ownerKey
		}
		canonicalOwner := s.canonicalSubjectiveEntityOwner(ctx, sid, ownerKey, ownerName)
		ownerKey = canonicalOwner.Key
		ownerName = canonicalOwner.Name
		if ownerKey == "" {
			result.addSkipReason("subjective_entity_memories", "missing_owner_or_memory_text", map[string]any{"index": idx})
			continue
		}
		sourceTurn := intFromAny(item["source_turn_index"], turnIndex)
		if sourceTurn <= 0 {
			sourceTurn = turnIndex
		}
		evidence := strings.TrimSpace(stringFromMap(item, "evidence_excerpt"))
		if evidence != "" {
			grounded := sanitizeEvidenceExcerptForTurn(evidence, content)
			if grounded == "" {
				result.addSkipReason("subjective_entity_memories", "evidence_excerpt_not_grounded", map[string]any{"index": idx, "owner_entity_key": ownerKey})
			}
			evidence = grounded
		}
		if duplicateReason := subjectiveEntityMemoryDuplicateReason(ctx, st, sid, ownerKey, sourceTurn, memoryText, evidence); duplicateReason != "" {
			result.addSkipReason("subjective_entity_memories", duplicateReason, map[string]any{
				"index":            idx,
				"owner_entity_key": ownerKey,
				"source_turn":      sourceTurn,
			})
			continue
		}
		ownerRole := normalizeSubjectiveEntityRoleFilter(stringFromMap(item, "owner_entity_role"))
		if ownerRole == "" {
			ownerRole = "protagonist"
		}
		ownerVisibility := normalizeSubjectiveEntityVisibilityFilter(stringFromMap(item, "owner_visibility"))
		if ownerVisibility == "" && ownerRole == "npc" {
			ownerVisibility = "owner_private"
		}
		if ownerVisibility == "" {
			ownerVisibility = "player_known"
		}
		portability := strings.TrimSpace(stringFromMap(item, "portability"))
		if portability == "" {
			if ownerRole == "npc" || ownerVisibility == "owner_private" {
				portability = "npc_private_recollection"
			} else {
				portability = "portable_subjective_entity_recollection"
			}
		}
		targetRevealPolicy := strings.TrimSpace(stringFromMap(item, "target_reveal_policy"))
		if targetRevealPolicy == "" {
			if ownerRole == "npc" || ownerVisibility == "owner_private" {
				targetRevealPolicy = "owner_private_until_revealed"
			} else {
				targetRevealPolicy = "requires_explicit_attachment"
			}
		}
		ownerTags := append([]string{}, stringsFromAny(item["tags"])...)
		ownerTags = append(ownerTags,
			"subjective_entity_memory",
			"owner_entity_key:"+ownerKey,
			"owner_entity_name:"+ownerName,
			"owner_entity_role:"+ownerRole,
			"owner_visibility:"+ownerVisibility,
		)
		ownerTags = append(ownerTags, canonicalOwner.AliasTags...)
		if canonicalOwner.Changed {
			ownerTags = append(ownerTags, "entity_alias_canonicalized")
		}
		if rawOwnerKey != "" && rawOwnerKey != ownerKey {
			ownerTags = append(ownerTags, "raw_owner_entity_key:"+rawOwnerKey)
		}
		if rawOwnerName != "" && rawOwnerName != ownerName {
			ownerTags = append(ownerTags, "raw_owner_entity_name:"+rawOwnerName)
		}
		if boolFromAny(item["secret_guard"]) {
			ownerTags = append(ownerTags, "secret_guard")
			ownerTags = append(ownerTags, "protected_secret")
		}
		for _, tag := range protectedSecretTagsFromSubjectiveItem(item) {
			ownerTags = append(ownerTags, tag)
		}
		result.trySave("CreateProtagonistEntityMemory(subjective_entity_memories)", func() error {
			_, err := st.CreateProtagonistEntityMemory(ctx, &store.ProtagonistEntityMemory{
				PersonaEntityKey:    ownerKey,
				PersonaEntityName:   ownerName,
				OwnerEntityKey:      ownerKey,
				OwnerEntityName:     ownerName,
				OwnerEntityRole:     ownerRole,
				OwnerVisibility:     ownerVisibility,
				SourceChatSessionID: sid,
				SourceCharacterName: ownerName,
				SourceTurn:          sourceTurn,
				MemoryText:          memoryText,
				EvidenceExcerpt:     evidence,
				SecretGuard:         boolFromAny(item["secret_guard"]),
				Portability:         portability,
				TargetRevealPolicy:  normalizeTargetRevealPolicy(targetRevealPolicy),
				TagsJSON:            mustCompactJSON(ownerTags),
				Importance10:        clampFloat(extractionFloatFromAny(item["importance_10"], 5), 1, 10),
				EmotionalWeight:     clampFloat(extractionFloatFromAny(item["emotional_weight"], 0.5), 0, 1),
				CreatedAt:           now,
				UpdatedAt:           now,
			})
			return err
		}, result, func() { result.SubjectiveEntityMemories++ })
	}
}

func protectedSecretTagsFromSubjectiveItem(item map[string]any) []string {
	out := []string{}
	if kind := normalizeProtectedSecretToken(extractionFirstNonEmpty(stringFromMap(item, "secret_kind"), stringFromMap(item, "protected_secret_kind"))); kind != "" {
		out = append(out, "protected_secret_kind:"+kind)
	}
	if policy := normalizeTargetRevealPolicy(extractionFirstNonEmpty(stringFromMap(item, "target_reveal_policy"), stringFromMap(item, "disclosure_policy"))); policy != "" {
		out = append(out, "target_reveal_policy:"+policy)
	}
	return out
}

func subjectiveEntityMemoryDuplicateReason(ctx context.Context, st store.ProtagonistEntityMemoryStore, sid, ownerKey string, sourceTurn int, memoryText, evidence string) string {
	existing, err := st.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{
		OwnerEntityKey:      ownerKey,
		SourceChatSessionID: sid,
		Limit:               80,
	})
	if err != nil {
		return ""
	}
	normalizedText := normalizeSubjectiveMemoryDuplicateText(memoryText)
	normalizedEvidence := normalizeSubjectiveMemoryDuplicateText(evidence)
	for _, item := range existing {
		if normalizeSubjectiveMemoryDuplicateText(item.MemoryText) == normalizedText {
			if item.SourceTurn == sourceTurn {
				return "duplicate_source_turn_owner_memory"
			}
			return "duplicate_owner_memory_text"
		}
		turnDistance := item.SourceTurn - sourceTurn
		if turnDistance < 0 {
			turnDistance = -turnDistance
		}
		if len(normalizedEvidence) >= 24 && turnDistance <= 3 && normalizeSubjectiveMemoryDuplicateText(item.EvidenceExcerpt) == normalizedEvidence {
			return "duplicate_nearby_owner_evidence"
		}
	}
	return ""
}

func normalizeSubjectiveMemoryDuplicateText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func emotionalImportanceBoost(emotionalIntensity float64) float64 {
	if emotionalIntensity >= 0.90 {
		return 2.0
	}
	if emotionalIntensity >= 0.70 {
		return 1.0
	}
	return 0
}

func simpleTokenSimilarity(a, b string) float64 {
	aTokens := map[string]int{}
	bTokens := map[string]int{}
	for _, t := range strings.Fields(strings.ToLower(a)) {
		aTokens[t]++
	}
	for _, t := range strings.Fields(strings.ToLower(b)) {
		bTokens[t]++
	}
	if len(aTokens) == 0 && len(bTokens) == 0 {
		return 1.0
	}
	intersection := 0
	for t, ca := range aTokens {
		cb := bTokens[t]
		if ca < cb {
			intersection += ca
		} else {
			intersection += cb
		}
	}
	union := len(aTokens) + len(bTokens) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}
