package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const prepareTurnEntityIdentityAliasesContextKey = "_entity_identity_aliases"

func buildPrepareTurnEntityIdentityAliases(ctx context.Context, candidateStore store.Store, sid string, states []store.CharacterState, memories []store.ProtagonistEntityMemory) map[string]any {
	out := map[string]any{}
	resolver, ok := candidateStore.(store.UniqueActiveEntitySurfaceIdentityResolver)
	if !ok || strings.TrimSpace(sid) == "" {
		return out
	}
	surfaces := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range surfaces {
			if comparableEntityKey(existing) == comparableEntityKey(value) {
				return
			}
		}
		surfaces = append(surfaces, value)
	}
	for _, state := range states {
		add(state.CharacterName)
	}
	for _, memory := range memories {
		add(firstNonEmpty(memory.OwnerEntityName, memory.PersonaEntityName))
		add(firstNonEmpty(memory.OwnerEntityKey, memory.PersonaEntityKey))
	}
	for _, surface := range surfaces {
		resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(surface))
		if err != nil || strings.TrimSpace(resolved.StableEntityID) == "" || strings.TrimSpace(resolved.CanonicalLabel) == "" {
			continue
		}
		out[surface] = strings.TrimSpace(resolved.CanonicalLabel)
		out[strings.TrimSpace(resolved.CanonicalLabel)] = strings.TrimSpace(resolved.CanonicalLabel)
	}
	return out
}

func prepareTurnCanonicalSurface(surface string, aliases map[string]any) string {
	surface = strings.TrimSpace(surface)
	if surface == "" {
		return ""
	}
	wanted := comparableEntityKey(surface)
	for alias, rawCanonical := range aliases {
		if comparableEntityKey(alias) != wanted {
			continue
		}
		if canonical := strings.TrimSpace(extractionStringFromAny(rawCanonical)); canonical != "" {
			return canonical
		}
	}
	return surface
}

func prepareTurnExplicitAliasLists(aliases map[string]any) map[string][]string {
	out := map[string][]string{}
	for alias, rawCanonical := range aliases {
		canonical := strings.TrimSpace(extractionStringFromAny(rawCanonical))
		alias = strings.TrimSpace(alias)
		if canonical == "" || alias == "" || comparableEntityKey(alias) == comparableEntityKey(canonical) {
			continue
		}
		key := normalizePrepareTurnEntityNeedle(canonical)
		out[key] = appendUniqueString(out[key], alias)
	}
	return out
}

func buildPersonaRecollectionText(entries []store.PersonaMemoryEntry, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
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

func buildCharacterPrivateRecollectionText(entries []store.ProtagonistEntityMemory, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
	perEntryChars = prepareTurnTextBudget(perEntryChars)
	lines := []string{
		"NPC private memory is the owning NPC's interpretation/bias, not player knowledge, narrator knowledge, or current-world truth; do not present it as objective fact.",
		"Use only as subtext: hesitation, recognition, avoidance, attraction, suspicion, or careful choice.",
		"Do not imply protagonist knowledge or explain the memory unless current evidence or explicit user instruction reveals it.",
	}
	entryLineBase := len(lines)
	for _, entry := range entries {
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
		text = strings.TrimSpace(text + " | " + protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.InjectionPolicy))
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
		text = strings.TrimSpace(text + " | " + protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.TargetRevealPolicy))
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

func personaRecollectionSecretGuardActive(entries []store.PersonaMemoryEntry) bool {
	for _, entry := range entries {
		tags := personaMemoryEntryTags(entry)
		portability := strings.ToLower(strings.TrimSpace(entry.Portability))
		if personaMemoryEntryHasTag(tags, "secret_guard") ||
			personaMemoryEntryHasTag(tags, "protected_secret") ||
			personaMemoryEntryHasTagPrefix(tags, "protected_secret_kind") ||
			personaMemoryEntryHasTagPrefix(tags, "identity_kind") ||
			portability == "cross_world" {
			return true
		}
	}
	return false
}

func characterPrivateRecollectionSecretGuardActive(entries []store.ProtagonistEntityMemory) bool {
	for _, entry := range entries {
		if entry.SecretGuard {
			return true
		}
		tags := []string{}
		_ = json.Unmarshal([]byte(strings.TrimSpace(entry.TagsJSON)), &tags)
		if personaMemoryEntryHasTag(tags, "secret_guard") ||
			personaMemoryEntryHasTag(tags, "protected_secret") ||
			personaMemoryEntryHasTagPrefix(tags, "protected_secret_kind") ||
			personaMemoryEntryHasTagPrefix(tags, "identity_kind") ||
			strings.EqualFold(strings.TrimSpace(entry.Portability), "cross_world") {
			return true
		}
	}
	return false
}

func personaMemoryEntryIsCharacterPrivate(entry store.PersonaMemoryEntry) bool {
	ownerRole := strings.ToLower(strings.TrimSpace(personaMemoryEntryTagValue(personaMemoryEntryTags(entry), "owner_entity_role")))
	return (ownerRole == "npc" || ownerRole == "supporting_character" || ownerRole == "supporting-character") &&
		personaMemoryEntryHasPrivateMarker(entry)
}

func personaMemoryEntryHasPrivateMarker(entry store.PersonaMemoryEntry) bool {
	portability := strings.ToLower(strings.TrimSpace(entry.Portability))
	injectionPolicy := strings.ToLower(strings.TrimSpace(entry.InjectionPolicy))
	return portability == "npc_private_recollection" ||
		injectionPolicy == "character_private_recollection"
}

func prepareTurnProtectedPrivateGuardKey(turn int, owner, text string) string {
	owner = normalizeCharacterKey(owner)
	text = normalizeSubjectiveMemoryDuplicateText(text)
	if turn <= 0 || owner == "" || text == "" {
		return ""
	}
	return fmt.Sprintf("%d|%s|%s", turn, owner, text)
}

func prepareTurnProtectedPrivateGuardIndex(memories []store.Memory) map[string]bool {
	index := map[string]bool{}
	add := func(turn int, owner, text string) {
		if key := prepareTurnProtectedPrivateGuardKey(turn, owner, text); key != "" {
			index[key] = true
		}
	}
	for _, memory := range memories {
		parsed := parseJSONMap(memory.SummaryJSON)
		for _, raw := range sliceFromAny(parsed["protected_secrets"]) {
			secret := mapFromAny(raw)
			add(memory.TurnIndex, stringFromMap(secret, "owner"), stringFromMap(secret, "summary"))
		}
		for _, raw := range sliceFromAny(parsed["character_identity_accuracy"]) {
			identity := mapFromAny(raw)
			owner := extractionFirstNonEmpty(
				stringFromMap(identity, "canonical_entity_name"),
				stringFromMap(identity, "true_identity_name"),
				stringFromMap(identity, "surface_identity_name"),
			)
			add(memory.TurnIndex, owner, protectedIdentityGuardSummary(identity))
		}
	}
	return index
}

func prepareTurnProtectedMemoryOwnsPrivateGuard(index map[string]bool, entry store.ProtagonistEntityMemory) bool {
	var tags []string
	if json.Unmarshal([]byte(strings.TrimSpace(entry.TagsJSON)), &tags) == nil {
		for _, tag := range tags {
			switch strings.ToLower(strings.TrimSpace(tag)) {
			case "derived_from_protected_secret", "derived_from_identity_accuracy":
				return true
			}
		}
	}
	for _, owner := range []string{entry.OwnerEntityKey, entry.OwnerEntityName, entry.PersonaEntityKey, entry.PersonaEntityName} {
		if index[prepareTurnProtectedPrivateGuardKey(entry.SourceTurn, owner, entry.MemoryText)] {
			return true
		}
	}
	return false
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

func personaMemoryEntryHasTagPrefix(tags []string, key string) bool {
	return strings.TrimSpace(personaMemoryEntryTagValue(tags, key)) != ""
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

func filterPrepareTurnPersonaRecollections(rawUserInput string, memories []store.Memory, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, pendingThreads []store.PendingThread, entries []store.PersonaMemoryEntry, chatLogGroups ...[]store.ChatLog) ([]store.PersonaMemoryEntry, map[string]any) {
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, memories, activeStates, canonicalLayers, pendingThreads, chatLogGroups...)
	query := ctx.relevanceText()
	type scoredPersonaEntry struct {
		entry store.PersonaMemoryEntry
		score int
	}
	eligible := make([]scoredPersonaEntry, 0, len(entries))
	dropped := []map[string]any{}
	for _, entry := range entries {
		tags := personaMemoryEntryTags(entry)
		relevanceTags := []string{}
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			lowerTag := strings.ToLower(tag)
			if strings.HasPrefix(lowerTag, "owner_") ||
				strings.HasPrefix(lowerTag, "raw_owner_") ||
				strings.HasPrefix(lowerTag, "source_chat_session_id:") ||
				strings.HasPrefix(lowerTag, "target_reveal_policy:") ||
				strings.HasPrefix(lowerTag, "protected_secret_") ||
				strings.HasPrefix(lowerTag, "entity_alias_") ||
				stringSliceContains([]string{
					"subjective_entity_memory",
					"secret_guard",
					"protected_secret",
					"npc_private",
					"owner_private",
				}, lowerTag) {
				continue
			}
			relevanceTags = append(relevanceTags, tag)
		}
		text := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
			entry.MemoryText,
			entry.EvidenceExcerpt,
			strings.Join(relevanceTags, " "),
		}), "\n"))
		if text == "" {
			dropped = append(dropped, map[string]any{"id": entry.ID, "reason": "empty_persona_recollection"})
			continue
		}
		queryTerms := prepareTurnDistinctiveRecallTerms(query)
		score := prepareTurnDistinctiveRecallOverlapCount(queryTerms, text)
		inflectionScore := prepareTurnPersonaInflectionOverlapCount(query, text)
		if score <= 0 && inflectionScore <= 0 {
			dropped = append(dropped, map[string]any{
				"id":     entry.ID,
				"reason": "persona_irrelevant_to_current_request",
			})
			continue
		}
		if inflectionScore > score {
			score = inflectionScore
		}
		owner := extractionFirstNonEmpty(
			personaMemoryEntryTagValue(tags, "owner_entity_name"),
			personaMemoryEntryTagValue(tags, "owner_entity_key"),
		)
		if owner != "" && prepareTurnAnyOwnerTokenMatches(prepareTurnOwnerTokens(owner, owner), query) {
			score += 2
		}
		eligible = append(eligible, scoredPersonaEntry{entry: entry, score: score})
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].score != eligible[j].score {
			return eligible[i].score > eligible[j].score
		}
		if eligible[i].entry.Importance10 != eligible[j].entry.Importance10 {
			return eligible[i].entry.Importance10 > eligible[j].entry.Importance10
		}
		if eligible[i].entry.SourceTurn != eligible[j].entry.SourceTurn {
			return eligible[i].entry.SourceTurn > eligible[j].entry.SourceTurn
		}
		return eligible[i].entry.ID > eligible[j].entry.ID
	})
	filtered := make([]store.PersonaMemoryEntry, 0, len(eligible))
	for _, candidate := range eligible {
		filtered = append(filtered, candidate.entry)
	}
	return filtered, map[string]any{
		"contract_version": "persona_recollection_relevance.v1",
		"status":           "active",
		"requested_count":  len(entries),
		"eligible_count":   len(filtered),
		"deferred_count":   len(dropped),
		"dropped":          dropped,
		"selection_policy": "current_request_or_confirmed_current_scene_overlap_then_importance_then_recency",
		"garbage_fill":     false,
		"truth_authority":  false,
		"canonical_write":  false,
	}
}

func prepareTurnPersonaInflectionOverlapCount(query, text string) int {
	queryTerms := prepareTurnRecallTerms(query)
	textTerms := prepareTurnRecallTerms(text)
	seen := map[string]bool{}
	overlap := 0
	for _, queryTerm := range queryTerms {
		queryRunes := []rune(queryTerm)
		if len(queryRunes) < 2 || !prepareTurnContainsNonASCII(queryRunes) {
			continue
		}
		for _, textTerm := range textTerms {
			textRunes := []rune(textTerm)
			if len(textRunes) < 2 || !prepareTurnContainsNonASCII(textRunes) {
				continue
			}
			shorter := queryTerm
			longer := textTerm
			if len(queryRunes) > len(textRunes) {
				shorter, longer = textTerm, queryTerm
			}
			key := shorter + "\x1f" + longer
			if seen[key] || len([]rune(shorter)) < 2 || !strings.HasPrefix(longer, shorter) {
				continue
			}
			seen[key] = true
			overlap++
			break
		}
	}
	return overlap
}

func prepareTurnContainsNonASCII(value []rune) bool {
	for _, r := range value {
		if r > 127 {
			return true
		}
	}
	return false
}

type prepareTurnRecollectionContext struct {
	rawUserInput              string
	previousEventSummary      string
	previousEventGuardSummary string
	currentAssistantContext   string
	currentSceneStates        string
	unresolvedGoals           string
	currentEntities           string
	latestAssistantTurn       int
	currentSceneTurn          int
	currentSceneIsCurrent     bool
}

func (ctx prepareTurnRecollectionContext) relevanceText() string {
	return strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
		ctx.rawUserInput,
		ctx.previousEventSummary,
		ctx.currentAssistantContext,
		ctx.currentSceneStates,
		ctx.unresolvedGoals,
		ctx.currentEntities,
	}), "\n"))
}

type prepareTurnRequestEntityScope struct {
	Direct []string
	Scene  []string
	Known  []string
}

func buildPrepareTurnRequestEntityScope(rawUserInput, currentSceneEntities string, knownNames []string, acceptedContextText ...string) prepareTurnRequestEntityScope {
	return buildPrepareTurnRequestEntityScopeWithAliases(rawUserInput, currentSceneEntities, knownNames, nil, acceptedContextText...)
}

func buildPrepareTurnRequestEntityScopeWithAliases(rawUserInput, currentSceneEntities string, knownNames []string, identityAliases map[string]any, acceptedContextText ...string) prepareTurnRequestEntityScope {
	scope := prepareTurnRequestEntityScope{}
	addUnique := func(target *[]string, value string) {
		value = strings.TrimSpace(value)
		if value == "" || prepareTurnRelationshipNameInList(value, *target) {
			return
		}
		*target = append(*target, value)
	}
	type observedSurface struct {
		surface   string
		canonical string
	}
	observedSurfaces := make([]observedSurface, 0, len(knownNames)+len(identityAliases))
	observedKeys := map[string]bool{}
	addSurface := func(surface, canonical string) {
		surface = strings.TrimSpace(surface)
		canonical = strings.TrimSpace(canonical)
		key := normalizePrepareTurnEntityNeedle(surface)
		if surface == "" || canonical == "" || key == "" || observedKeys[key] {
			return
		}
		observedKeys[key] = true
		observedSurfaces = append(observedSurfaces, observedSurface{surface: surface, canonical: canonical})
	}
	for _, name := range knownNames {
		canonical := prepareTurnCanonicalSurface(name, identityAliases)
		addUnique(&scope.Known, canonical)
		addSurface(name, canonical)
	}
	aliasKeys := make([]string, 0, len(identityAliases))
	for alias := range identityAliases {
		aliasKeys = append(aliasKeys, alias)
	}
	sort.Slice(aliasKeys, func(i, j int) bool {
		return normalizePrepareTurnEntityNeedle(aliasKeys[i]) < normalizePrepareTurnEntityNeedle(aliasKeys[j])
	})
	for _, alias := range aliasKeys {
		rawCanonical := identityAliases[alias]
		canonical := strings.TrimSpace(extractionStringFromAny(rawCanonical))
		if alias == "" || canonical == "" {
			continue
		}
		addSurface(alias, canonical)
		addUnique(&scope.Known, canonical)
	}
	for _, observed := range observedSurfaces {
		if prepareTurnRecallContainsAnchor(rawUserInput, observed.surface) {
			addUnique(&scope.Direct, observed.canonical)
		}
	}
	for _, observed := range nonEmptyStrings(strings.Split(currentSceneEntities, "\n")) {
		canonical := prepareTurnCanonicalSurface(observed, identityAliases)
		for _, known := range scope.Known {
			if normalizePrepareTurnEntityNeedle(known) == normalizePrepareTurnEntityNeedle(observed) {
				canonical = known
				break
			}
		}
		addUnique(&scope.Scene, canonical)
	}
	contextText := strings.TrimSpace(strings.Join(nonEmptyStrings(acceptedContextText), "\n"))
	if contextText != "" {
		for _, observed := range observedSurfaces {
			if prepareTurnRecallContainsAnchor(contextText, observed.surface) {
				addUnique(&scope.Scene, observed.canonical)
			}
		}
	}
	return scope
}

func prepareTurnEntityScopeQuery(rawUserInput string, names ...[]string) string {
	parts := []string{strings.TrimSpace(rawUserInput)}
	for _, group := range names {
		parts = append(parts, strings.Join(group, "\n"))
	}
	return strings.TrimSpace(strings.Join(nonEmptyStrings(parts), "\n"))
}

func prepareTurnDirectEntityMemoryOwners(rawUserInput string, owners []store.ProtagonistEntityMemoryOwner) []store.ProtagonistEntityMemoryOwner {
	return prepareTurnDirectEntityMemoryOwnersWithAliases(rawUserInput, owners, nil)
}

func prepareTurnDirectEntityMemoryOwnersWithAliases(rawUserInput string, owners []store.ProtagonistEntityMemoryOwner, identityAliases map[string]any) []store.ProtagonistEntityMemoryOwner {
	names := make([]string, 0, len(owners))
	for _, owner := range owners {
		names = append(names, strings.TrimSpace(firstNonEmpty(owner.OwnerEntityName, owner.OwnerEntityKey)))
	}
	aliases := prepareTurnExplicitAliasLists(identityAliases)
	out := make([]store.ProtagonistEntityMemoryOwner, 0)
	seen := map[string]bool{}
	for _, owner := range owners {
		name := strings.TrimSpace(firstNonEmpty(owner.OwnerEntityName, owner.OwnerEntityKey))
		if prepareTurnDirectEntityMentionRank(rawUserInput, name, aliases) == 0 {
			continue
		}
		key := strings.TrimSpace(owner.OwnerEntityKey)
		identity := strings.ToLower(firstNonEmpty(key, name))
		if identity == "" || seen[identity] {
			continue
		}
		seen[identity] = true
		out = append(out, owner)
	}
	return out
}

func prepareTurnEntityMentionPosition(rawUserInput, ownerName string, aliases map[string][]string) int {
	haystack := strings.ToLower(rawUserInput)
	best := len(haystack) + 1
	candidates := []string{strings.TrimSpace(ownerName)}
	canonical := normalizePrepareTurnEntityNeedle(ownerName)
	candidates = append(candidates, aliases[canonical]...)
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "" || !prepareTurnRecallContainsAnchor(rawUserInput, candidate) {
			continue
		}
		if pos := strings.Index(haystack, candidate); pos >= 0 && pos < best {
			best = pos
		}
	}
	return best
}

func mergePrepareTurnEntityMemories(priority, fallback []store.ProtagonistEntityMemory) []store.ProtagonistEntityMemory {
	out := make([]store.ProtagonistEntityMemory, 0, len(priority)+len(fallback))
	seen := map[int64]bool{}
	add := func(item store.ProtagonistEntityMemory) {
		if item.ID > 0 {
			if seen[item.ID] {
				return
			}
			seen[item.ID] = true
		}
		out = append(out, item)
	}
	for _, item := range priority {
		add(item)
	}
	for _, item := range fallback {
		add(item)
	}
	return out
}

func filterPrepareTurnEntityRecollections(rawUserInput string, memories []store.Memory, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, pendingThreads []store.PendingThread, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories *[]store.ProtagonistEntityMemory, chatLogGroups ...[]store.ChatLog) map[string]any {
	return filterPrepareTurnEntityRecollectionsWithAliases(rawUserInput, memories, activeStates, canonicalLayers, pendingThreads, personaEntries, characterPrivateMemories, nil, chatLogGroups...)
}

func filterPrepareTurnEntityRecollectionsWithAliases(rawUserInput string, memories []store.Memory, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, pendingThreads []store.PendingThread, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories *[]store.ProtagonistEntityMemory, identityAliases map[string]any, chatLogGroups ...[]store.ChatLog) map[string]any {
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, memories, activeStates, canonicalLayers, pendingThreads, chatLogGroups...)
	beforePrivate := len(*characterPrivateMemories)
	eligiblePrivate := make([]store.ProtagonistEntityMemory, 0, beforePrivate)
	ownerNames := make([]string, 0, beforePrivate)
	for _, item := range *characterPrivateMemories {
		ownerNames = append(ownerNames, prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName))
	}
	ownerAliases := prepareTurnExplicitAliasLists(identityAliases)
	ownerScope := buildPrepareTurnRequestEntityScopeWithAliases(
		rawUserInput,
		ctx.currentEntities,
		ownerNames,
		identityAliases,
		ctx.currentAssistantContext,
		ctx.previousEventSummary,
	)
	if len(ownerScope.Scene) > 0 {
		ctx.currentEntities = strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
			ctx.currentEntities,
			strings.Join(ownerScope.Scene, "\n"),
		}), "\n"))
	}
	activeOwnerKeys := map[string]bool{}
	for _, item := range *characterPrivateMemories {
		owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
		if prepareTurnRelationshipNameInList(owner, ownerScope.Direct) || prepareTurnRelationshipNameInList(owner, ownerScope.Scene) {
			activeOwnerKeys[prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)] = true
		}
	}
	relevanceQuery := prepareTurnEntityScopeQuery(ctx.rawUserInput, ownerScope.Direct, ownerScope.Scene)
	sort.SliceStable(*characterPrivateMemories, func(i, j int) bool {
		left := (*characterPrivateMemories)[i]
		right := (*characterPrivateMemories)[j]
		leftOwner := prepareTurnMemoryOwnerLabel(left.OwnerEntityKey, left.OwnerEntityName)
		rightOwner := prepareTurnMemoryOwnerLabel(right.OwnerEntityKey, right.OwnerEntityName)
		leftDirect := prepareTurnDirectEntityMentionRank(rawUserInput, leftOwner, ownerAliases)
		rightDirect := prepareTurnDirectEntityMentionRank(rawUserInput, rightOwner, ownerAliases)
		if leftDirect != rightDirect {
			return leftDirect > rightDirect
		}
		leftIdentity := prepareTurnMemoryOwnerIdentity(left.OwnerEntityKey, left.OwnerEntityName)
		rightIdentity := prepareTurnMemoryOwnerIdentity(right.OwnerEntityKey, right.OwnerEntityName)
		if leftIdentity != rightIdentity {
			leftPosition := prepareTurnEntityMentionPosition(rawUserInput, leftOwner, ownerAliases)
			rightPosition := prepareTurnEntityMentionPosition(rawUserInput, rightOwner, ownerAliases)
			if leftPosition != rightPosition {
				return leftPosition < rightPosition
			}
			return false
		}
		leftText := strings.Join(nonEmptyStrings([]string{left.MemoryText, left.EvidenceExcerpt, left.TagsJSON}), " ")
		rightText := strings.Join(nonEmptyStrings([]string{right.MemoryText, right.EvidenceExcerpt, right.TagsJSON}), " ")
		leftOverlap := prepareTurnRecallOverlapCount(relevanceQuery, leftText)
		rightOverlap := prepareTurnRecallOverlapCount(relevanceQuery, rightText)
		if leftOverlap != rightOverlap {
			return leftOverlap > rightOverlap
		}
		if left.EmotionalWeight != right.EmotionalWeight {
			return left.EmotionalWeight > right.EmotionalWeight
		}
		if left.Importance10 != right.Importance10 {
			return left.Importance10 > right.Importance10
		}
		if left.SourceTurn != right.SourceTurn {
			return left.SourceTurn > right.SourceTurn
		}
		return left.ID > right.ID
	})
	droppedOwners := []string{}
	dropped := []map[string]any{}
	protectedPrivateGuardIndex := prepareTurnProtectedPrivateGuardIndex(memories)
	for _, item := range *characterPrivateMemories {
		ownerKey := prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)
		ownerRole := strings.ToLower(strings.TrimSpace(item.OwnerEntityRole))
		if ownerRole != "npc" && ownerRole != "supporting_character" && ownerRole != "supporting-character" {
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "non_npc_memory_domain",
			})
			continue
		}
		if item.SecretGuard && prepareTurnProtectedMemoryOwnsPrivateGuard(protectedPrivateGuardIndex, item) {
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "protected_secret_owned_by_protected_lane",
			})
			continue
		}
		if len(activeOwnerKeys) > 0 && !activeOwnerKeys[ownerKey] {
			owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
			if owner != "" && !stringSliceContains(droppedOwners, owner) {
				droppedOwners = append(droppedOwners, owner)
			}
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "owner_not_confirmed_in_current_input",
			})
			continue
		}
		if ok, reason := prepareTurnCharacterPrivateMemoryRelevant(item, ctx, ownerAliases); ok {
			eligiblePrivate = append(eligiblePrivate, item)
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
	filteredPrivate := make([]store.ProtagonistEntityMemory, 0, len(eligiblePrivate))
	selectedOwners := []string{}
	selectedOwnerKeys := map[string]bool{}
	selectedIDs := map[int64]bool{}
	selectedMemoryKeys := map[string]bool{}
	appendEligible := func(item store.ProtagonistEntityMemory) bool {
		if item.ID > 0 && selectedIDs[item.ID] {
			return false
		}
		memoryKey := collapseTextKey(strings.Join(nonEmptyStrings([]string{
			prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName),
			item.MemoryText,
			item.EvidenceExcerpt,
		}), "\n"))
		if memoryKey != "" && selectedMemoryKeys[memoryKey] {
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "subjective_memory_semantic_duplicate",
			})
			return false
		}
		filteredPrivate = append(filteredPrivate, item)
		if item.ID > 0 {
			selectedIDs[item.ID] = true
		}
		if memoryKey != "" {
			selectedMemoryKeys[memoryKey] = true
		}
		ownerKey := prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)
		if ownerKey != "" {
			selectedOwnerKeys[ownerKey] = true
		}
		if owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName); owner != "" && !stringSliceContains(selectedOwners, owner) {
			selectedOwners = append(selectedOwners, owner)
		}
		return true
	}
	// Pass one gives each already-eligible owner one relevant recollection.
	for _, item := range eligiblePrivate {
		ownerKey := prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)
		if ownerKey != "" && selectedOwnerKeys[ownerKey] {
			continue
		}
		appendEligible(item)
	}
	// Pass two keeps distinct, relevant recollections from the same owner. The
	// existing subjective_relationship character budget remains the final cap.
	for _, item := range eligiblePrivate {
		appendEligible(item)
	}
	*characterPrivateMemories = filteredPrivate
	return map[string]any{
		"version":                                     "pmc19.prepare_turn_entity_relevance.v1",
		"status":                                      "active",
		"persona_recollection_count":                  len(personaEntries),
		"persona_recollection_rule":                   "protagonist_or_player_recollection_allowed_as_support_only_when_explicitly_attached",
		"character_private_before_filter":             beforePrivate,
		"character_private_after_filter":              len(filteredPrivate),
		"character_private_dropped_count":             beforePrivate - len(filteredPrivate),
		"character_private_gate":                      "owner_entity_must_match_current_user_input_or_observed_current_scene_entity",
		"character_private_owner_cap":                 "removed_after_eligibility",
		"character_private_owner_selection":           "coverage_first_then_relevant_distinct_fill",
		"character_private_owner_coverage_count":      len(selectedOwnerKeys),
		"character_private_eligible_count":            len(eligiblePrivate),
		"character_private_total_cap":                 "final_subjective_relationship_char_budget",
		"character_private_reviewed_identity_aliases": len(ownerAliases),
		"character_private_candidate_order":           "direct_owner_then_scene_overlap_then_emotional_weight_then_importance_then_recency",
		"current_input_owner_count":                   len(ownerScope.Direct),
		"accepted_context_owner_count":                len(ownerScope.Scene),
		"current_input_owner_memory_cap":              "final_subjective_relationship_char_budget",
		"protagonist_npc_memory_domains_separate":     true,
		"selected_owner_entities":                     selectedOwners,
		"dropped_owner_entities":                      droppedOwners,
		"dropped":                                     dropped,
		"blocks_unrelated_session_memory":             true,
		"blocks_unrelated_entity_memory":              true,
		"truth_authority":                             false,
		"canonical_write":                             false,
		"context_sources":                             []string{"current_user_input", "confirmed_current_entities", "latest_accepted_assistant_entity_mentions"},
	}
}

func buildPrepareTurnRecollectionContext(rawUserInput string, memories []store.Memory, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, pendingThreads []store.PendingThread, chatLogGroups ...[]store.ChatLog) prepareTurnRecollectionContext {
	previousEventTurn := 0
	for _, item := range memories {
		if item.TurnIndex > previousEventTurn {
			previousEventTurn = item.TurnIndex
		}
	}
	previousEventCandidates := []string{}
	for _, item := range memories {
		if item.TurnIndex != previousEventTurn {
			continue
		}
		if summary := compactPrepareTurnLine(prepareTurnMemorySummary(item), 0); summary != "" && !stringSliceContains(previousEventCandidates, summary) {
			previousEventCandidates = append(previousEventCandidates, summary)
		}
	}
	latestAssistantTurn := 0
	latestAssistantContext := ""
	if len(chatLogGroups) > 0 {
		for _, item := range chatLogGroups[0] {
			if strings.EqualFold(strings.TrimSpace(item.Role), "assistant") && item.TurnIndex > latestAssistantTurn {
				latestAssistantTurn = item.TurnIndex
				latestAssistantContext = compactPrepareTurnLine(item.Content, 0)
			}
		}
	}
	previousEventGuardSummary := ""
	if previousEventTurn > 0 && previousEventTurn == latestAssistantTurn {
		previousEventGuardSummary = strings.Join(previousEventCandidates, "\n")
	}
	state := []string{}
	currentEntities := []string{}
	addSceneEntities := func(content string) {
		payload := parseJSONMap(content)
		for _, key := range []string{"present_entities", "present_characters"} {
			for _, name := range stringsFromAny(payload[key]) {
				name = strings.TrimSpace(name)
				if name != "" && !stringSliceContains(currentEntities, name) {
					currentEntities = append(currentEntities, name)
				}
			}
		}
	}
	latestStateTurn := 0
	for _, item := range activeStates {
		if item.TurnIndex > latestStateTurn {
			latestStateTurn = item.TurnIndex
		}
	}
	// Active-state rows are the current-state owner. Older databases may not
	// have a turn index on that row, so zero remains an explicit unversioned
	// current value; a positive turn can be rejected when it trails the latest
	// completed assistant turn.
	activeStateIsCurrent := len(activeStates) > 0 &&
		(latestStateTurn == 0 || latestAssistantTurn == 0 || latestStateTurn >= latestAssistantTurn)
	if activeStateIsCurrent {
		for _, item := range activeStates {
			stateType := strings.ToLower(strings.TrimSpace(item.StateType))
			if item.TurnIndex != latestStateTurn || (stateType != "scene" && stateType != "scene_state" && stateType != "state_deltas") {
				continue
			}
			if stateType == "state_deltas" {
				payload, ok := parseSurfacePayload(item.Content).(map[string]any)
				if !ok {
					continue
				}
				if _, ok := payload["scene_state"].(map[string]any); !ok {
					continue
				}
			}
			if text := prepareTurnSceneStateWithoutUnresolvedThreads(item.Content); text != "" && !stringSliceContains(state, text) {
				addSceneEntities(text)
				state = append(state, text)
			}
		}
	}
	if len(state) == 0 {
		latestCanonicalTurn := 0
		for _, item := range canonicalLayers {
			if item.LayerType != "scene_state" {
				continue
			}
			if item.TurnIndex > latestCanonicalTurn {
				latestCanonicalTurn = item.TurnIndex
			}
		}
		canonicalSceneCount := 0
		for _, item := range canonicalLayers {
			if item.LayerType == "scene_state" && item.TurnIndex == latestCanonicalTurn {
				canonicalSceneCount++
			}
		}
		canonicalStateIsCurrent := canonicalSceneCount > 0 &&
			(latestCanonicalTurn == 0 || latestAssistantTurn == 0 || latestCanonicalTurn >= latestAssistantTurn)
		if canonicalStateIsCurrent {
			latestStateTurn = latestCanonicalTurn
			for _, item := range canonicalLayers {
				if item.TurnIndex != latestCanonicalTurn || item.LayerType != "scene_state" {
					continue
				}
				addSceneEntities(item.Content)
				if text := prepareTurnSceneStateWithoutUnresolvedThreads(item.Content); text != "" && !stringSliceContains(state, text) {
					state = append(state, text)
				}
			}
		}
	}
	sceneQuery := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
		strings.TrimSpace(rawUserInput),
		strings.Join(state, "\n"),
		strings.Join(currentEntities, "\n"),
	}), "\n"))
	previousEventQuery := strings.TrimSpace(rawUserInput)
	if len(prepareTurnRecallTerms(previousEventQuery)) < 2 {
		previousEventQuery = sceneQuery
	}
	sparseCurrentContext := len(prepareTurnRecallTerms(previousEventQuery)) < 2
	previousEvents := []string{}
	for _, summary := range previousEventCandidates {
		if !sparseCurrentContext && !prepareTurnSupportRecallEligible(previousEventQuery, summary) {
			continue
		}
		previousEvents = append(previousEvents, summary)
	}
	goalQuery := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{sceneQuery, strings.Join(previousEvents, "\n")}), "\n"))
	goals := []string{}
	for _, item := range openNarrativeThreads(pendingThreads) {
		goal := strings.TrimSpace(firstNonEmpty(item.Description, item.Title, item.ThreadKey))
		if goal == "" {
			continue
		}
		if !prepareTurnSupportRecallEligible(goalQuery, goal) {
			continue
		}
		goals = append(goals, compactPrepareTurnLine(goal, 0))
	}
	return prepareTurnRecollectionContext{
		rawUserInput:              strings.TrimSpace(rawUserInput),
		previousEventSummary:      strings.Join(previousEvents, "\n"),
		previousEventGuardSummary: previousEventGuardSummary,
		currentAssistantContext:   latestAssistantContext,
		currentSceneStates:        strings.Join(state, "\n"),
		unresolvedGoals:           strings.Join(goals, "\n"),
		currentEntities:           strings.Join(currentEntities, "\n"),
		latestAssistantTurn:       latestAssistantTurn,
		currentSceneTurn:          latestStateTurn,
		currentSceneIsCurrent:     len(state) > 0,
	}
}

func prepareTurnSceneStateWithoutUnresolvedThreads(content string) string {
	surface := parseSurfacePayload(content)
	payload, ok := surface.(map[string]any)
	if !ok {
		return compactPrepareTurnLine(prepareTurnSurfaceText(surface), 0)
	}
	if nested, nestedOK := payload["scene_state"].(map[string]any); nestedOK {
		payload = nested
	}
	delete(payload, "unresolved_threads")
	if !hasMeaningfulPayload(payload) {
		return ""
	}
	return compactPrepareTurnLine(prepareTurnSurfaceText(payload), 0)
}

func prepareTurnCharacterPrivateMemoryRelevant(item store.ProtagonistEntityMemory, ctx prepareTurnRecollectionContext, aliasMaps ...map[string][]string) (bool, string) {
	ownerTokens := prepareTurnOwnerTokens(item.OwnerEntityKey, item.OwnerEntityName)
	if len(ownerTokens) == 0 {
		return false, "missing_owner_entity"
	}
	var aliases []string
	if len(aliasMaps) > 0 {
		for _, identity := range []string{item.OwnerEntityName, item.OwnerEntityKey, prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)} {
			identity = normalizePrepareTurnEntityNeedle(identity)
			for _, alias := range aliasMaps[0][identity] {
				if !stringSliceContains(aliases, alias) {
					aliases = append(aliases, alias)
				}
			}
		}
	}
	ownerReason := ""
	switch {
	case prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.rawUserInput):
		ownerReason = "explicit_current_user_input"
	case prepareTurnAnyOwnerTokenMatches(aliases, ctx.rawUserInput):
		ownerReason = "explicit_current_user_input_reviewed_identity_alias"
	case prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.currentEntities):
		ownerReason = "observed_current_scene_entity"
	case prepareTurnAnyOwnerTokenMatches(aliases, ctx.currentEntities):
		ownerReason = "observed_current_scene_entity_reviewed_identity_alias"
	default:
		return false, "owner_not_in_current_input_or_observed_scene"
	}
	if !prepareTurnCharacterPrivateMemoryContentRelevant(item, ctx, append(ownerTokens, aliases...)) {
		return false, "subjective_memory_irrelevant_to_current_request"
	}
	return true, ownerReason + "_and_memory_overlap"
}

func prepareTurnCharacterPrivateMemoryContentRelevant(item store.ProtagonistEntityMemory, ctx prepareTurnRecollectionContext, ownerTokens []string) bool {
	ownerTerms := map[string]bool{}
	for _, token := range ownerTokens {
		for _, term := range prepareTurnRecallTerms(token) {
			ownerTerms[term] = true
		}
	}
	isOwnerTerm := func(term string) bool {
		if ownerTerms[term] {
			return true
		}
		termRunes := []rune(term)
		if !prepareTurnContainsNonASCII(termRunes) {
			return false
		}
		for ownerTerm := range ownerTerms {
			ownerRunes := []rune(ownerTerm)
			if len(ownerRunes) >= 2 && prepareTurnContainsNonASCII(ownerRunes) && strings.HasPrefix(term, ownerTerm) {
				return true
			}
		}
		return false
	}
	queryTerms := []string{}
	for _, term := range prepareTurnRecallTerms(ctx.relevanceText()) {
		if !isOwnerTerm(term) {
			queryTerms = append(queryTerms, term)
		}
	}
	memoryTerms := []string{}
	for _, term := range prepareTurnRecallTerms(strings.Join(nonEmptyStrings([]string{
		item.MemoryText,
		item.EvidenceExcerpt,
		item.TagsJSON,
	}), "\n")) {
		if !isOwnerTerm(term) {
			memoryTerms = append(memoryTerms, term)
		}
	}
	if len(queryTerms) == 0 || len(memoryTerms) == 0 {
		return false
	}
	queryText := strings.Join(queryTerms, "\n")
	memoryText := strings.Join(memoryTerms, "\n")
	return prepareTurnDistinctiveRecallOverlapCount(prepareTurnDistinctiveRecallTerms(queryText), memoryText) > 0 ||
		prepareTurnPersonaInflectionOverlapCount(queryText, memoryText) > 0
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

func buildPersonaRecollectionSurface(sid string, entries []store.PersonaMemoryEntry, text string) map[string]any {
	items := []map[string]any{}
	for _, entry := range entries {
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

func buildCharacterPrivateRecollectionSurface(sid string, entries []store.ProtagonistEntityMemory, text string) map[string]any {
	items := []map[string]any{}
	for _, entry := range entries {
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
