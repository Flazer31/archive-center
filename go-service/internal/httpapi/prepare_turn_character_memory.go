package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	prepareTurnCharacterMemoryContractVersion = "character_memory_delivery.v1"
	prepareTurnCharacterMemoryContextKey      = "_character_memory_context"
)

// buildPrepareTurnCharacterMemoryReadContext reads the complete active-source
// fence and the complete current directional-relationship projection. These
// are support observations for the existing prepare-turn assembly; they do not
// introduce a second persistence or delivery owner.
func buildPrepareTurnCharacterMemoryReadContext(ctx context.Context, candidateStore store.Store, sid string) map[string]any {
	out := map[string]any{
		"contract_version":         prepareTurnCharacterMemoryContractVersion,
		"chat_session_id":          strings.TrimSpace(sid),
		"active_source_status":     "unavailable",
		"active_source_revisions":  map[string]bool{},
		"relationship_read_status": "unavailable",
		"relationship_values":      []store.StatusCurrentValue{},
	}
	if candidateStore == nil || strings.TrimSpace(sid) == "" {
		out["failure_reason"] = "store_or_session_unavailable"
		return out
	}
	lister, ok := candidateStore.(store.ActiveSourceRevisionLister)
	if !ok {
		out["failure_reason"] = "active_source_revision_lister_unavailable"
		return out
	}
	sources, err := lister.ListActiveSourceRevisions(ctx, sid, 0, 0)
	if err != nil {
		out["active_source_status"] = "error"
		out["failure_reason"] = "active_source_revision_read_failed"
		return out
	}
	active := map[string]bool{}
	for _, source := range sources {
		if source.ContractVersion != store.MemorySourceRevisionContract ||
			source.ChatSessionID != sid || source.LifecycleState != "active" ||
			strings.TrimSpace(source.SourceRevision) == "" {
			continue
		}
		active[strings.TrimSpace(source.SourceRevision)] = true
	}
	out["active_source_status"] = "ready"
	out["active_source_revisions"] = active
	out["active_source_revision_count"] = len(active)

	reader, ok := candidateStore.(store.ReversibleStatusTransitionStore)
	if !ok {
		return out
	}
	values, err := reader.ListReversibleStatusCurrentValues(
		ctx, sid, relationshipStateOwnerScope, []string{relationshipStateStatusKey},
	)
	if err != nil {
		out["relationship_read_status"] = "error"
		return out
	}
	values = canonicalPrepareTurnRelationshipValuesForRead(ctx, candidateStore, sid, values)
	out["relationship_read_status"] = "ready"
	out["relationship_values"] = values
	out["relationship_value_count"] = len(values)
	return out
}

func canonicalPrepareTurnRelationshipValuesForRead(ctx context.Context, candidateStore store.Store, sid string, values []store.StatusCurrentValue) []store.StatusCurrentValue {
	resolver, ok := candidateStore.(store.UniqueActiveEntitySurfaceIdentityResolver)
	if !ok || strings.TrimSpace(sid) == "" {
		return values
	}
	out := append([]store.StatusCurrentValue(nil), values...)
	for index := range out {
		projection := map[string]any{}
		if json.Unmarshal([]byte(out[index].ValueJSON), &projection) != nil || extractionStringFromAny(projection["version"]) != relationshipStateContractVersion {
			continue
		}
		changed := false
		for _, side := range []string{"source", "target"} {
			labelKey := side + "_label"
			idKey := side + "_entity_id"
			label := strings.TrimSpace(extractionStringFromAny(projection[labelKey]))
			if label == "" {
				continue
			}
			resolved, err := resolver.ResolveUniqueActiveEntityIdentityBySurface(ctx, sid, comparableEntityKey(label))
			if err != nil || strings.TrimSpace(resolved.StableEntityID) == "" {
				continue
			}
			projection[idKey] = strings.TrimSpace(resolved.StableEntityID)
			if canonicalLabel := strings.TrimSpace(resolved.CanonicalLabel); canonicalLabel != "" {
				projection[labelKey] = canonicalLabel
			}
			changed = true
		}
		if changed {
			out[index].ValueJSON = mustCompactJSON(projection)
		}
	}
	return out
}

func buildPrepareTurnCharacterMemorySupport(
	sid string,
	charStates []store.CharacterState,
	scope prepareTurnRequestEntityScope,
	perspectiveContext map[string]any,
	readContext map[string]any,
) map[string]any {
	dropped := map[string]any{}
	out := map[string]any{
		"contract_version":               prepareTurnCharacterMemoryContractVersion,
		"status":                         "empty",
		"chat_session_id":                strings.TrimSpace(sid),
		"eligible_items":                 []map[string]any{},
		"delivered_items":                []map[string]any{},
		"eligible_count":                 0,
		"delivered_count":                0,
		"deferred_by_budget_count":       0,
		"candidate_count":                0,
		"dropped_counts":                 dropped,
		"direct_entities":                append([]string{}, scope.Direct...),
		"current_scene_entities":         append([]string{}, scope.Scene...),
		"count_cap":                      nil,
		"selection_boundary":             "current_input_or_confirmed_scene_then_exact_entity_counterpart_pov_visibility_active_source_then_character_class_char_budget",
		"raw_dialogue_included":          false,
		"raw_private_originals_included": false,
		"reciprocity_inferred":           false,
	}
	if extractionStringFromAny(readContext["active_source_status"]) != "ready" {
		out["status"] = "fail_closed"
		out["failure_reason"] = extractionFirstNonEmpty(
			extractionStringFromAny(readContext["failure_reason"]),
			"active_source_revision_list_unavailable",
		)
		return out
	}
	active, ok := readContext["active_source_revisions"].(map[string]bool)
	if !ok {
		out["status"] = "fail_closed"
		out["failure_reason"] = "active_source_revision_set_invalid"
		return out
	}
	out["active_source_revision_count"] = len(active)
	out["relationship_read_status"] = extractionStringFromAny(readContext["relationship_read_status"])

	items := []map[string]any{}
	for _, state := range charStates {
		if state.ChatSessionID != sid {
			prepareTurnCharacterMemoryDrop(dropped, "wrong_session")
			continue
		}
		profile := mapFromAny(parseSurfacePayload(state.PersonalityJSON))
		if extractionStringFromAny(profile["contract_version"]) == characterProfileContractVersion {
			items = append(items, prepareTurnCharacterProfileItems(sid, state, profile, scope, perspectiveContext, active, dropped, out)...)
		}
		voice := mapFromAny(parseSurfacePayload(state.SpeechStyleJSON))
		if extractionStringFromAny(voice["contract_version"]) == voiceBehaviorProjectionContractVersion {
			items = append(items, prepareTurnVoiceBehaviorItems(sid, state, voice, scope, perspectiveContext, active, dropped, out)...)
		}
	}
	if values, ok := readContext["relationship_values"].([]store.StatusCurrentValue); ok {
		items = append(items, prepareTurnRelationshipStateItems(sid, values, scope, perspectiveContext, active, dropped, out)...)
	} else if extractionStringFromAny(readContext["relationship_read_status"]) == "ready" {
		prepareTurnCharacterMemoryDrop(dropped, "relationship_current_values_invalid")
	}

	sort.SliceStable(items, func(i, j int) bool {
		leftDirect := boolFromAny(items[i]["direct_entity_relevance"])
		rightDirect := boolFromAny(items[j]["direct_entity_relevance"])
		if leftDirect != rightDirect {
			return leftDirect
		}
		leftTurn := intFromAny(mapFromAny(items[i]["source_metadata"])["source_turn_end"], 0)
		rightTurn := intFromAny(mapFromAny(items[j]["source_metadata"])["source_turn_end"], 0)
		if leftTurn != rightTurn {
			return leftTurn > rightTurn
		}
		return extractionStringFromAny(items[i]["source_ref"]) < extractionStringFromAny(items[j]["source_ref"])
	})
	out["eligible_items"] = items
	out["eligible_count"] = len(items)
	if len(items) > 0 {
		out["status"] = "eligible"
	}
	return out
}

func prepareTurnCharacterProfileItems(
	sid string,
	state store.CharacterState,
	profile map[string]any,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
	dropped map[string]any,
	trace map[string]any,
) []map[string]any {
	subjectID := strings.TrimSpace(extractionStringFromAny(profile["subject_entity_id"]))
	subjectLabel := strings.TrimSpace(extractionFirstNonEmpty(extractionStringFromAny(profile["subject_label"]), state.CharacterName))
	if subjectID == "" || subjectLabel == "" ||
		normalizePrepareTurnEntityNeedle(subjectLabel) != normalizePrepareTurnEntityNeedle(state.CharacterName) ||
		!prepareTurnCharacterMemoryEntityInScope(subjectID, subjectLabel, scope) {
		prepareTurnCharacterMemoryDrop(dropped, "profile_subject_out_of_scope")
		return nil
	}
	items := []map[string]any{}
	for _, section := range []string{"stable", "current", "dynamic", "relationship_specific"} {
		for _, raw := range sliceFromAny(mapFromAny(profile[section])["observations"]) {
			trace["candidate_count"] = intFromAny(trace["candidate_count"], 0) + 1
			if item, ok := prepareTurnCharacterProfileItem(sid, subjectID, subjectLabel, section, mapFromAny(raw), false, scope, perspective, active, dropped); ok {
				items = append(items, item)
			}
		}
	}
	for _, raw := range sliceFromAny(profile["counterevidence"]) {
		trace["candidate_count"] = intFromAny(trace["candidate_count"], 0) + 1
		entry := mapFromAny(raw)
		section := extractionStringFromAny(entry["profile_section"])
		if item, ok := prepareTurnCharacterProfileItem(sid, subjectID, subjectLabel, section, entry, true, scope, perspective, active, dropped); ok {
			items = append(items, item)
		}
	}
	return items
}

func prepareTurnCharacterProfileItem(
	sid, subjectID, subjectLabel, section string,
	entry map[string]any,
	counterevidence bool,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
	dropped map[string]any,
) (map[string]any, bool) {
	section = strings.TrimSpace(section)
	if !map[string]bool{"stable": true, "current": true, "dynamic": true, "relationship_specific": true}[section] {
		prepareTurnCharacterMemoryDrop(dropped, "profile_section_invalid")
		return nil, false
	}
	counterpart := mapFromAny(entry["counterpart"])
	counterpartID := strings.TrimSpace(extractionStringFromAny(counterpart["counterpart_entity_id"]))
	counterpartLabel := strings.TrimSpace(extractionStringFromAny(counterpart["counterpart_label"]))
	if section == "relationship_specific" && (counterpartID == "" || counterpartLabel == "" || !prepareTurnCharacterMemoryEntityInScope(counterpartID, counterpartLabel, scope)) {
		prepareTurnCharacterMemoryDrop(dropped, "counterpart_out_of_scope")
		return nil, false
	}
	source := mapFromAny(entry["source_ref"])
	visibility, guard, ok := prepareTurnCharacterMemorySourceEligible(sid, subjectID, source, scope, perspective, active)
	if !ok {
		prepareTurnCharacterMemoryDrop(dropped, "profile_source_ineligible")
		return nil, false
	}
	domain := strings.TrimSpace(extractionStringFromAny(entry["trait_domain"]))
	traitKey := strings.TrimSpace(extractionStringFromAny(entry["trait_key"]))
	expression := strings.TrimSpace(extractionStringFromAny(entry["supported_expression"]))
	if traitKey == "" {
		prepareTurnCharacterMemoryDrop(dropped, "profile_description_missing")
		return nil, false
	}
	kind := "character_profile"
	stateWord := "support"
	if counterevidence {
		kind = "character_profile_counterevidence"
		stateWord = "counterevidence"
	}
	ref := prepareTurnCharacterMemoryStableRef(sid, kind, subjectID, section, domain, traitKey, mustCompactJSON(source))
	parts := []string{
		fmt.Sprintf("%s profile %s", subjectLabel, stateWord),
		"section=" + section,
		"trait_key=" + traitKey,
	}
	if domain != "" {
		parts = append(parts, "trait_domain="+domain)
	}
	if expression != "" {
		parts = append(parts, "expression="+compactPrepareTurnLine(expression, 0))
	}
	if counterpartLabel != "" {
		parts = append(parts, "counterpart="+counterpartLabel)
	}
	line := prepareTurnCharacterMemoryLine(guard, strings.Join(parts, "; "))
	class := "character_objective"
	if section == "relationship_specific" {
		class = "subjective_relationship"
	}
	return map[string]any{
		"source_ref": ref, "source_metadata": source, "class": class, "kind": kind,
		"subject_entity_id": subjectID, "subject_label": subjectLabel,
		"counterpart_entity_id": nilIfEmpty(counterpartID), "counterpart_label": nilIfEmpty(counterpartLabel),
		"profile_section": section, "trait_domain": nilIfEmpty(domain), "trait_key": traitKey,
		"visibility": visibility, "privacy_guard": nilIfEmpty(guard),
		"direct_entity_relevance": prepareTurnCharacterMemoryEntityInList(subjectID, subjectLabel, scope.Direct),
		"text":                    line, "delivered": false,
	}, true
}

func prepareTurnVoiceBehaviorItems(
	sid string,
	state store.CharacterState,
	voice map[string]any,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
	dropped map[string]any,
	trace map[string]any,
) []map[string]any {
	subjectID := strings.TrimSpace(extractionStringFromAny(voice["subject_entity_id"]))
	subjectLabel := strings.TrimSpace(extractionFirstNonEmpty(extractionStringFromAny(voice["subject_label"]), state.CharacterName))
	if subjectID == "" || subjectLabel == "" ||
		normalizePrepareTurnEntityNeedle(subjectLabel) != normalizePrepareTurnEntityNeedle(state.CharacterName) ||
		!prepareTurnCharacterMemoryEntityInScope(subjectID, subjectLabel, scope) {
		prepareTurnCharacterMemoryDrop(dropped, "voice_subject_out_of_scope")
		return nil
	}
	items := []map[string]any{}
	for _, raw := range sliceFromAny(voice["principles"]) {
		trace["candidate_count"] = intFromAny(trace["candidate_count"], 0) + 1
		principle := mapFromAny(raw)
		domain := strings.TrimSpace(extractionStringFromAny(principle["trait_domain"]))
		principleKey := strings.TrimSpace(extractionStringFromAny(principle["principle_key"]))
		if principleKey == "" {
			prepareTurnCharacterMemoryDrop(dropped, "voice_principle_description_missing")
			continue
		}
		supportRefs, supportGuards := prepareTurnEligibleVoiceRefs(sid, subjectID, principle["support_refs"], scope, perspective, active, dropped)
		if len(supportRefs) == 0 {
			prepareTurnCharacterMemoryDrop(dropped, "voice_support_source_ineligible")
			continue
		}
		counterRefs, _ := prepareTurnEligibleVoiceRefs(sid, subjectID, principle["counterevidence_refs"], scope, perspective, active, dropped)
		exceptionRefs, _ := prepareTurnEligibleVoiceRefs(sid, subjectID, principle["exception_refs"], scope, perspective, active, dropped)
		contexts, counterparts, modulations := prepareTurnVoiceConditionsFromRefs(supportRefs)
		ref := prepareTurnCharacterMemoryStableRef(sid, "voice_behavior", subjectID, domain, principleKey, mustCompactJSON(supportRefs))
		parts := []string{
			fmt.Sprintf("%s voice principle", subjectLabel),
			"principle=" + principleKey,
		}
		if domain != "" {
			parts = append(parts, "trait_domain="+domain)
		}
		if len(contexts) > 0 {
			parts = append(parts, "contexts="+strings.Join(contexts, ","))
		}
		if len(counterparts) > 0 {
			parts = append(parts, "counterparts="+strings.Join(counterparts, ","))
		}
		if len(modulations) > 0 {
			parts = append(parts, "state_modulations="+strings.Join(modulations, ","))
		}
		if len(counterRefs) > 0 {
			parts = append(parts, "counterevidence=present")
		}
		if len(exceptionRefs) > 0 {
			parts = append(parts, "exceptions=present")
		}
		guard := prepareTurnCharacterMemoryStrongestGuard(supportGuards)
		items = append(items, map[string]any{
			"source_ref": ref, "source_metadata": supportRefs[0], "support_source_metadata": supportRefs,
			"class": "character_objective", "kind": "voice_behavior", "subject_entity_id": subjectID, "subject_label": subjectLabel,
			"voice_domain": nilIfEmpty(domain), "voice_principle_key": principleKey,
			"visibility": extractionStringFromAny(supportRefs[0]["visibility"]), "privacy_guard": nilIfEmpty(guard),
			"direct_entity_relevance": prepareTurnCharacterMemoryEntityInList(subjectID, subjectLabel, scope.Direct),
			"counterevidence_present": len(counterRefs) > 0, "exception_present": len(exceptionRefs) > 0,
			"text": prepareTurnCharacterMemoryLine(guard, strings.Join(parts, "; ")), "delivered": false,
		})
	}
	return items
}

func prepareTurnEligibleVoiceRefs(
	sid, subjectID string,
	raw any,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
	dropped map[string]any,
) ([]map[string]any, []string) {
	refs := []map[string]any{}
	guards := []string{}
	for _, rawRef := range sliceFromAny(raw) {
		ref := mapFromAny(rawRef)
		counterpart := mapFromAny(ref["counterpart"])
		counterpartID := strings.TrimSpace(extractionStringFromAny(counterpart["counterpart_entity_id"]))
		counterpartLabel := strings.TrimSpace(extractionStringFromAny(counterpart["counterpart_label"]))
		if counterpartID != "" || counterpartLabel != "" {
			if counterpartID == "" || counterpartLabel == "" || !prepareTurnCharacterMemoryEntityInScope(counterpartID, counterpartLabel, scope) {
				prepareTurnCharacterMemoryDrop(dropped, "voice_counterpart_out_of_scope")
				continue
			}
		}
		_, guard, ok := prepareTurnCharacterMemorySourceEligible(sid, subjectID, ref, scope, perspective, active)
		if !ok {
			continue
		}
		refs = append(refs, ref)
		guards = append(guards, guard)
	}
	return refs, guards
}

func prepareTurnVoiceConditionsFromRefs(refs []map[string]any) ([]string, []string, []string) {
	contexts := []string{}
	counterparts := []string{}
	modulations := []string{}
	for _, ref := range refs {
		context := mapFromAny(ref["context"])
		contexts = appendUniqueMemorySearchText(contexts, extractionStringFromAny(context["context_key"]))
		counterpart := mapFromAny(ref["counterpart"])
		counterparts = appendUniqueMemorySearchText(counterparts, extractionStringFromAny(counterpart["counterpart_label"]))
		modulation := mapFromAny(ref["state_modulation"])
		modulations = appendUniqueMemorySearchText(modulations, extractionStringFromAny(modulation["state_modulation_key"]))
	}
	return nonEmptyStrings(contexts), nonEmptyStrings(counterparts), nonEmptyStrings(modulations)
}

func prepareTurnRelationshipStateItems(
	sid string,
	values []store.StatusCurrentValue,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
	dropped map[string]any,
	trace map[string]any,
) []map[string]any {
	items := []map[string]any{}
	for _, value := range values {
		trace["candidate_count"] = intFromAny(trace["candidate_count"], 0) + 1
		if value.ChatSessionID != sid || value.OwnerScope != relationshipStateOwnerScope ||
			value.StatusKey != relationshipStateStatusKey || value.WriteState != "current" {
			prepareTurnCharacterMemoryDrop(dropped, "relationship_current_contract_mismatch")
			continue
		}
		projection := map[string]any{}
		if json.Unmarshal([]byte(value.ValueJSON), &projection) != nil || extractionStringFromAny(projection["version"]) != relationshipStateContractVersion {
			prepareTurnCharacterMemoryDrop(dropped, "relationship_value_invalid")
			continue
		}
		sourceID := strings.TrimSpace(extractionStringFromAny(projection["source_entity_id"]))
		sourceLabel := strings.TrimSpace(extractionStringFromAny(projection["source_label"]))
		targetID := strings.TrimSpace(extractionStringFromAny(projection["target_entity_id"]))
		targetLabel := strings.TrimSpace(extractionStringFromAny(projection["target_label"]))
		if sourceID == "" || targetID == "" || sourceID == targetID ||
			!prepareTurnCharacterMemoryEntityInScope(sourceID, sourceLabel, scope) ||
			!prepareTurnCharacterMemoryEntityInScope(targetID, targetLabel, scope) {
			prepareTurnCharacterMemoryDrop(dropped, "relationship_endpoint_out_of_scope")
			continue
		}
		claim := mapFromAny(projection["current"])
		source := mapFromAny(projection["source"])
		validity := mapFromAny(projection["validity"])
		source["chat_session_id"] = value.ChatSessionID
		source["source_turn_start"] = validity["source_turn_start"]
		source["source_turn_end"] = validity["source_turn_end"]
		source["visibility"] = claim["visibility"]
		visibility, guard, ok := prepareTurnCharacterMemorySourceEligible(sid, sourceID, source, scope, perspective, active)
		if !ok {
			prepareTurnCharacterMemoryDrop(dropped, "relationship_source_ineligible")
			continue
		}
		domain := strings.TrimSpace(extractionStringFromAny(projection["domain"]))
		observation := strings.TrimSpace(extractionStringFromAny(claim["observation"]))
		if observation == "" {
			prepareTurnCharacterMemoryDrop(dropped, "relationship_description_missing")
			continue
		}
		ref := prepareTurnCharacterMemoryStableRef(sid, "relationship_state", sourceID, targetID, domain, mustCompactJSON(source))
		parts := []string{
			fmt.Sprintf("%s -> %s relationship", sourceLabel, targetLabel),
			"current=" + compactPrepareTurnLine(observation, 0),
			"directional_only",
			"reciprocity=not_inferred",
		}
		if domain != "" {
			parts = append(parts, "domain="+domain)
		}
		for _, key := range []string{"magnitude", "duration"} {
			if text := strings.TrimSpace(extractionStringFromAny(claim[key])); text != "" {
				parts = append(parts, key+"="+compactPrepareTurnLine(text, 0))
			}
		}
		items = append(items, map[string]any{
			"source_ref": ref, "source_metadata": source, "class": "subjective_relationship", "kind": "relationship_state",
			"subject_entity_id": sourceID, "subject_label": sourceLabel,
			"counterpart_entity_id": targetID, "counterpart_label": targetLabel,
			"visibility": visibility, "privacy_guard": nilIfEmpty(guard),
			"direct_entity_relevance": prepareTurnCharacterMemoryEntityInList(sourceID, sourceLabel, scope.Direct) || prepareTurnCharacterMemoryEntityInList(targetID, targetLabel, scope.Direct),
			"text":                    prepareTurnCharacterMemoryLine(guard, strings.Join(parts, "; ")), "delivered": false,
		})
	}
	return items
}

func prepareTurnCharacterMemorySourceEligible(
	sid, ownerID string,
	ref map[string]any,
	scope prepareTurnRequestEntityScope,
	perspective map[string]any,
	active map[string]bool,
) (string, string, bool) {
	revision := strings.TrimSpace(extractionStringFromAny(ref["source_revision"]))
	if extractionStringFromAny(ref["chat_session_id"]) != sid ||
		extractionStringFromAny(ref["source_contract"]) != completeTurnSourceAcceptanceContract ||
		revision == "" || !active[revision] ||
		strings.TrimSpace(extractionStringFromAny(ref["precise_memory_unit_id"])) == "" ||
		intFromAny(ref["source_turn_start"], 0) <= 0 ||
		intFromAny(ref["source_turn_end"], 0) < intFromAny(ref["source_turn_start"], 0) ||
		strings.TrimSpace(extractionStringFromAny(ref["content_hash"])) == "" {
		return "", "", false
	}
	visibility := strings.ToLower(strings.TrimSpace(extractionStringFromAny(ref["visibility"])))
	switch visibility {
	case "public":
		return visibility, "", true
	case "owner_private":
		// Callers have already required the exact source subject/owner to be in
		// current input or confirmed scene scope. Do not try to rematch a stable
		// entity ID against a label-only scene surface here.
		if strings.TrimSpace(ownerID) == "" {
			return "", "", false
		}
		return visibility, "subtext_only_do_not_reveal_private_fact", true
	case "restricted", "user_private":
		povID := strings.TrimSpace(extractionStringFromAny(perspective["current_pov_entity_id"]))
		if extractionStringFromAny(perspective["identity_state"]) != "resolved" || povID == "" || povID != strings.TrimSpace(ownerID) {
			return "", "", false
		}
		return visibility, "pov_private_do_not_leak_outside_holder", true
	default:
		return "", "", false
	}
}

func prepareTurnCharacterMemoryEntityInScope(id, label string, scope prepareTurnRequestEntityScope) bool {
	all := append(append([]string{}, scope.Direct...), scope.Scene...)
	return prepareTurnCharacterMemoryEntityInList(id, label, all)
}

func prepareTurnCharacterMemoryEntityInList(id, label string, values []string) bool {
	for _, candidate := range []string{id, label} {
		if strings.TrimSpace(candidate) != "" && prepareTurnRelationshipNameInList(candidate, values) {
			return true
		}
	}
	return false
}

func prepareTurnCharacterMemoryLine(guard, body string) string {
	parts := []string{"-"}
	if strings.TrimSpace(guard) != "" {
		parts = append(parts, "[guard="+strings.TrimSpace(guard)+"]")
	}
	parts = append(parts, strings.TrimSpace(body))
	return strings.Join(parts, " ")
}

func prepareTurnCharacterMemoryStableRef(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return "character-memory:" + hex.EncodeToString(sum[:])
}

func prepareTurnCharacterMemoryStrongestGuard(guards []string) string {
	for _, guard := range guards {
		if guard == "pov_private_do_not_leak_outside_holder" {
			return guard
		}
	}
	for _, guard := range guards {
		if guard == "subtext_only_do_not_reveal_private_fact" {
			return guard
		}
	}
	return ""
}

func prepareTurnCharacterMemoryDrop(dropped map[string]any, reason string) {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		dropped[reason] = intFromAny(dropped[reason], 0) + 1
	}
}

func prepareTurnCharacterMemoryLines(support map[string]any, class string) []string {
	lines := []string{}
	for _, raw := range outputFidelityLineageSlice(support["eligible_items"]) {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["class"]) != class {
			continue
		}
		if line := strings.TrimSpace(extractionStringFromAny(item["text"])); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func finalizePrepareTurnCharacterMemorySupport(support, plan map[string]any) map[string]any {
	if len(support) == 0 || extractionStringFromAny(support["status"]) == "fail_closed" {
		return support
	}
	deliveredClassItems := map[string]map[string]int{}
	for _, raw := range outputFidelityLineageSlice(plan["classes"]) {
		class := mapFromAny(raw)
		classKey := extractionStringFromAny(class["key"])
		deliveredClassItems[classKey] = map[string]int{}
		for _, item := range prepareTurnDeliveryItems(extractionStringFromAny(class["text"])) {
			if itemKey := collapseTextKey(item); itemKey != "" {
				deliveredClassItems[classKey][itemKey]++
			}
		}
	}
	items := []map[string]any{}
	delivered := []map[string]any{}
	deliveredRefs := []string{}
	deliveredTextByClass := map[string][]string{}
	for _, raw := range outputFidelityLineageSlice(support["eligible_items"]) {
		item := mapFromAny(raw)
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		class := extractionStringFromAny(item["class"])
		text := strings.TrimSpace(extractionStringFromAny(item["text"]))
		textKey := collapseTextKey(text)
		wasDelivered := ref != "" && text != "" && deliveredClassItems[class][textKey] > 0
		if wasDelivered {
			deliveredClassItems[class][textKey]--
		}
		item["delivered"] = wasDelivered
		items = append(items, item)
		if wasDelivered {
			delivered = append(delivered, item)
			deliveredRefs = appendUniqueMemorySearchText(deliveredRefs, ref)
			deliveredTextByClass[class] = append(deliveredTextByClass[class], text)
		}
	}
	support["eligible_items"] = items
	support["delivered_items"] = delivered
	support["delivered_source_refs"] = deliveredRefs
	support["delivered_count"] = len(delivered)
	support["deferred_by_budget_count"] = maxInt(len(items)-len(delivered), 0)
	support["delivered_text"] = map[string]any{
		"character_objective":     nilIfEmpty(strings.Join(deliveredTextByClass["character_objective"], "\n")),
		"subjective_relationship": nilIfEmpty(strings.Join(deliveredTextByClass["subjective_relationship"], "\n")),
	}
	switch {
	case len(delivered) > 0:
		support["status"] = "delivered"
	case len(items) > 0:
		support["status"] = "eligible_not_delivered"
	default:
		support["status"] = "empty"
	}
	return support
}

func deliveredPrepareTurnCharacterMemorySourceRefs(support map[string]any) []string {
	refs := []string{}
	for _, raw := range outputFidelityLineageSlice(support["delivered_items"]) {
		item := mapFromAny(raw)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		refs = appendUniqueMemorySearchText(refs, extractionStringFromAny(item["source_ref"]))
	}
	return nonEmptyStrings(refs)
}
