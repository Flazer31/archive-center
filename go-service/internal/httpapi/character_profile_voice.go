package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	characterProfileContractVersion        = "character_profile.v1"
	voiceBehaviorProjectionContractVersion = "voice_behavior_projection.v1"
)

type characterProjectionUnit struct {
	unit        *store.PreciseMemoryUnit
	payload     map[string]any
	class       string
	fingerprint string
}

type characterProjectionGroup struct {
	subjectEntityID string
	subjectLabel    string
	units           []characterProjectionUnit
	maxTurn         int
}

// saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits implements
// 3.9-C/D on the existing character_states JSON columns. It compiles only
// already-admitted support evidence and deliberately does not deliver the new
// projections to generation; 3.9-E owns that selection and privacy boundary.
func (s *Server) saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(
	ctx context.Context,
	sid string,
	units []*store.PreciseMemoryUnit,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil || strings.TrimSpace(sid) == "" {
		return
	}
	saver, ok := s.Store.(characterStateSaver)
	if !ok {
		result.addSkipReason("character_profile_voice", "character_state_saver_unavailable", nil)
		return
	}
	groups := map[string]*characterProjectionGroup{}
	groupOrder := []string{}
	for _, unit := range units {
		candidate, relevant, reason := characterProjectionUnitFromPreciseMemory(sid, unit)
		if !relevant {
			continue
		}
		if reason != "" {
			result.addSkipReason("character_profile_voice", reason, map[string]any{
				"precise_memory_unit_id": nilIfEmpty(characterProjectionPreciseUnitID(unit)),
			})
			continue
		}
		key := candidate.unit.SubjectEntityID
		group := groups[key]
		if group == nil {
			group = &characterProjectionGroup{subjectEntityID: key, subjectLabel: extractionStringFromAny(candidate.payload["subject_entity"])}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		}
		if candidate.unit.SourceTurnEnd >= group.maxTurn {
			group.maxTurn = candidate.unit.SourceTurnEnd
			group.subjectLabel = extractionStringFromAny(candidate.payload["subject_entity"])
		}
		group.units = append(group.units, candidate)
	}
	if len(groupOrder) == 0 {
		return
	}
	sort.Strings(groupOrder)
	for _, key := range groupOrder {
		group := groups[key]
		sort.Slice(group.units, func(i, j int) bool {
			left, right := group.units[i], group.units[j]
			if left.unit.SourceTurnEnd != right.unit.SourceTurnEnd {
				return left.unit.SourceTurnEnd < right.unit.SourceTurnEnd
			}
			if left.unit.SourceRevision != right.unit.SourceRevision {
				return left.unit.SourceRevision < right.unit.SourceRevision
			}
			if left.class != right.class {
				return left.class < right.class
			}
			return left.unit.UnitID < right.unit.UnitID
		})
		s.saveCharacterProjectionGroup(ctx, sid, group, saver, now, result)
	}
}

func (s *Server) saveCharacterProjectionGroup(
	ctx context.Context,
	sid string,
	group *characterProjectionGroup,
	saver characterStateSaver,
	now time.Time,
	result *artifactSaveResult,
) {
	label := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(group.subjectLabel))
	if label == "" {
		result.addSkipReason("character_profile_voice", "canonical_character_label_required", group.subjectEntityID)
		return
	}
	current, err := s.Store.GetCharacterState(ctx, sid, label)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("character_profile_voice", "character_state_read_failed", err.Error())
		return
	}
	if current == nil {
		current = &store.CharacterState{ChatSessionID: sid, CharacterName: label}
	}
	hasProfileEvidence := false
	hasVoiceEvidence := false
	for _, candidate := range group.units {
		hasProfileEvidence = hasProfileEvidence || candidate.class == "profile" || candidate.class == "habit"
		hasVoiceEvidence = hasVoiceEvidence || candidate.class == "voice"
	}

	var profile map[string]any
	profileWritable := false
	if hasProfileEvidence {
		profile, profileWritable = loadCharacterProjection(
			current.PersonalityJSON, characterProfileContractVersion, group.subjectEntityID,
			func() map[string]any { return newCharacterProfileProjection(group.subjectEntityID, label) },
		)
		if !profileWritable {
			result.addSkipReason("character_profiles", "existing_manual_or_malformed_personality_preserved", map[string]any{"character": label})
		}
	}
	var voice map[string]any
	voiceWritable := false
	if hasVoiceEvidence {
		voice, voiceWritable = loadCharacterProjection(
			current.SpeechStyleJSON, voiceBehaviorProjectionContractVersion, group.subjectEntityID,
			func() map[string]any { return newVoiceBehaviorProjection(group.subjectEntityID, label) },
		)
		if !voiceWritable {
			result.addSkipReason("voice_behavior_projections", "existing_manual_or_malformed_speech_style_preserved", map[string]any{"character": label})
		}
	}

	profileChanged := false
	voiceChanged := false
	for _, candidate := range group.units {
		switch candidate.class {
		case "profile", "habit":
			if profileWritable && !stringSliceContains(stringsFromAny(profile["evidence_fingerprints"]), candidate.fingerprint) {
				mergeCharacterProfileEvidence(profile, candidate)
				profileChanged = true
			}
		case "voice":
			if voiceWritable && !stringSliceContains(stringsFromAny(voice["evidence_fingerprints"]), candidate.fingerprint) {
				mergeVoiceBehaviorEvidence(voice, candidate)
				voiceChanged = true
			}
		}
	}
	if !profileChanged && !voiceChanged {
		return
	}
	next := *current
	next.ChatSessionID = sid
	next.CharacterName = label
	if profileChanged {
		next.PersonalityJSON = mustCompactJSON(profile)
	}
	if voiceChanged {
		next.SpeechStyleJSON = mustCompactJSON(voice)
	}
	if group.maxTurn > next.TurnIndex {
		next.TurnIndex = group.maxTurn
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	next.UpdatedAt = now
	result.Attempted++
	if err := saver.SaveCharacterState(ctx, &next); err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, "SaveCharacterState(character_profile_voice): "+err.Error())
		return
	}
	result.CharacterStates++
	if profileChanged {
		result.CharacterProfiles++
	}
	if voiceChanged {
		result.VoiceBehaviorProjections++
	}
}

func characterProjectionUnitFromPreciseMemory(sid string, unit *store.PreciseMemoryUnit) (characterProjectionUnit, bool, string) {
	if unit == nil || unit.Kind != "observation" ||
		(unit.Subtype != "character_profile" && unit.Subtype != "habit_observation" && unit.Subtype != "voice_behavior") {
		return characterProjectionUnit{}, false, ""
	}
	payload := map[string]any{}
	if json.Unmarshal([]byte(unit.PayloadJSON), &payload) != nil {
		return characterProjectionUnit{}, true, "character_projection_payload_invalid"
	}
	if unit.ContractVersion != store.PreciseMemoryUnitContract || unit.ChatSessionID != sid ||
		unit.SourceContract != completeTurnSourceAcceptanceContract || strings.TrimSpace(unit.SourceRevision) == "" ||
		unit.SourceTurnStart <= 0 || unit.SourceTurnEnd < unit.SourceTurnStart || strings.TrimSpace(unit.UnitID) == "" ||
		strings.TrimSpace(unit.SourceContentHash) == "" || unit.LifecycleState != "active" {
		return characterProjectionUnit{}, true, "active_accepted_source_required"
	}
	if unit.AdmissionState != "committed" || unit.ReviewState != "source_observed" || unit.TruthScope != "support_only" ||
		unit.EpistemicMode != "direct" || unit.AuthorityClass != "support_hypothesis" || strings.TrimSpace(unit.SubjectEntityID) == "" {
		return characterProjectionUnit{}, true, "committed_support_only_character_evidence_required"
	}
	if extractionStringFromAny(payload["admission_state"]) != "committed" || extractionStringFromAny(payload["review_state"]) != "source_observed" {
		return characterProjectionUnit{}, true, "payload_admission_state_mismatch"
	}
	visibility := strings.ToLower(strings.TrimSpace(unit.Visibility))
	if visibility != strings.ToLower(strings.TrimSpace(extractionStringFromAny(payload["visibility"]))) ||
		!map[string]bool{"public": true, "owner_private": true, "restricted": true, "user_private": true}[visibility] {
		return characterProjectionUnit{}, true, "source_bound_visibility_mismatch"
	}
	if strings.TrimSpace(unit.EvidenceExcerpt) == "" || strings.TrimSpace(unit.EvidenceHash) == "" || unit.RootEvidenceID <= 0 ||
		len(jsonInt64Slice(unit.DirectEvidenceIDsJSON)) == 0 {
		return characterProjectionUnit{}, true, "exact_evidence_refs_required"
	}
	subject := strings.TrimSpace(extractionStringFromAny(payload["subject_entity"]))
	if subject == "" {
		return characterProjectionUnit{}, true, "subject_required"
	}
	if reason := validateCharacterProjectionOptionalBindings(unit, payload); reason != "" {
		return characterProjectionUnit{}, true, reason
	}

	class := ""
	fingerprintParts := []string{}
	switch unit.Subtype {
	case "character_profile":
		if extractionStringFromAny(payload["contract_version"]) != characterProfileObservationContract {
			return characterProjectionUnit{}, true, "character_profile_observation_contract_required"
		}
		section := strings.TrimSpace(extractionStringFromAny(payload["profile_section"]))
		domain := strings.TrimSpace(extractionStringFromAny(payload["trait_domain"]))
		traitKey := strings.TrimSpace(extractionStringFromAny(payload["trait_key"]))
		kind := strings.TrimSpace(extractionStringFromAny(payload["observation_kind"]))
		if traitKey == "" {
			return characterProjectionUnit{}, true, "profile_trait_required"
		}
		if !map[string]bool{"stable": true, "current": true, "dynamic": true, "relationship_specific": true}[section] {
			// An omitted auxiliary classifier must not discard a grounded trait.
			// Dynamic is the non-permanent evidence bucket, not a semantic guess.
			section = "dynamic"
			payload["profile_section"] = section
		}
		class = "profile"
		fingerprintParts = []string{section, domain, traitKey, characterEvidenceObservationClass(kind)}
	case "habit_observation":
		if extractionStringFromAny(payload["contract_version"]) != habitObservationContract {
			return characterProjectionUnit{}, true, "habit_observation_contract_required"
		}
		behaviorKey := strings.TrimSpace(extractionStringFromAny(payload["behavior_key"]))
		kind := strings.TrimSpace(extractionStringFromAny(payload["observation_kind"]))
		if behaviorKey == "" {
			return characterProjectionUnit{}, true, "habit_behavior_required"
		}
		class = "habit"
		fingerprintParts = []string{"dynamic", "behavior_tendency", behaviorKey, characterEvidenceObservationClass(kind)}
	case "voice_behavior":
		if extractionStringFromAny(payload["contract_version"]) != voiceObservationContract {
			return characterProjectionUnit{}, true, "voice_observation_contract_required"
		}
		domain := strings.TrimSpace(extractionStringFromAny(payload["trait_domain"]))
		principleKey := strings.TrimSpace(extractionStringFromAny(payload["principle_key"]))
		kind := strings.TrimSpace(extractionStringFromAny(payload["observation_kind"]))
		if principleKey == "" || voicePrincipleKeyReplaysUtterance(principleKey, extractionStringFromAny(payload["utterance_expression"])) {
			return characterProjectionUnit{}, true, "descriptive_voice_principle_required"
		}
		class = "voice"
		fingerprintParts = []string{domain, principleKey, characterEvidenceObservationClass(kind)}
	}
	return characterProjectionUnit{
		unit:        unit,
		payload:     payload,
		class:       class,
		fingerprint: characterProjectionFingerprint(unit.RootEvidenceID, class, fingerprintParts...),
	}, true, ""
}

func validateCharacterProjectionOptionalBindings(unit *store.PreciseMemoryUnit, payload map[string]any) string {
	counterpart := strings.TrimSpace(extractionStringFromAny(payload["counterpart"]))
	if counterpart != "" && strings.TrimSpace(unit.AffectedEntityID) == "" {
		return "stable_source_bound_counterpart_required"
	}
	return ""
}

func loadCharacterProjection(raw, contractVersion, subjectEntityID string, create func() map[string]any) (map[string]any, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "{}" || trimmed == "null" || trimmed == "[]" {
		return create(), true
	}
	projection := map[string]any{}
	if json.Unmarshal([]byte(trimmed), &projection) != nil ||
		extractionStringFromAny(projection["contract_version"]) != contractVersion ||
		extractionStringFromAny(projection["subject_entity_id"]) != subjectEntityID {
		return nil, false
	}
	return projection, true
}

func newCharacterProfileProjection(subjectEntityID, label string) map[string]any {
	return map[string]any{
		"contract_version":            characterProfileContractVersion,
		"subject_entity_id":           subjectEntityID,
		"subject_label":               label,
		"authority_class":             "support_hypothesis",
		"disposition":                 "compiled_profile",
		"stable":                      map[string]any{"observations": []any{}},
		"current":                     map[string]any{"observations": []any{}},
		"dynamic":                     map[string]any{"observations": []any{}},
		"relationship_specific":       map[string]any{"observations": []any{}},
		"counterevidence":             []any{},
		"unresolved_domains":          []any{},
		"evidence_fingerprints":       []string{},
		"source_turns":                []any{},
		"current_overwrites_stable":   false,
		"fixed_count_trait_promotion": false,
		"cross_domain_inference":      false,
		"unsupported_entailment": []string{
			"personality_from_capability", "personality_from_interest", "personality_from_occupation",
			"personality_from_appearance", "personality_from_social_role", "morality_from_non_morality_trait",
			"emotion_from_stable_profile", "voice_from_non_voice_trait",
		},
		"voice_projection_owner":   voiceBehaviorProjectionContractVersion,
		"relationship_state_owner": relationshipStateContractVersion,
	}
}

func mergeCharacterProfileEvidence(projection map[string]any, candidate characterProjectionUnit) {
	entry := characterProfileEvidenceEntry(candidate)
	section := extractionStringFromAny(candidate.payload["profile_section"])
	kind := extractionStringFromAny(candidate.payload["observation_kind"])
	if candidate.class == "habit" {
		section = "dynamic"
		kind = extractionStringFromAny(candidate.payload["observation_kind"])
	}
	if kind == "counterexample" || kind == "exception" {
		projection["counterevidence"] = append(sliceFromAny(projection["counterevidence"]), entry)
	} else if section == "current" {
		current := mapFromAny(projection["current"])
		current["observations"] = replaceCurrentProfileObservation(sliceFromAny(current["observations"]), entry)
		projection["current"] = current
	} else {
		bucket := mapFromAny(projection[section])
		bucket["observations"] = append(sliceFromAny(bucket["observations"]), entry)
		projection[section] = bucket
	}
	projection["evidence_fingerprints"] = append(stringsFromAny(projection["evidence_fingerprints"]), candidate.fingerprint)
	projection["source_turns"] = habitEvidenceAppendUniqueTurn(projection["source_turns"], candidate.unit.SourceTurnEnd)
	projection["latest_source"] = characterProjectionSourceRef(candidate)
}

func characterProfileEvidenceEntry(candidate characterProjectionUnit) map[string]any {
	payload := candidate.payload
	traitDomain := extractionStringFromAny(payload["trait_domain"])
	traitKey := extractionStringFromAny(payload["trait_key"])
	supportedExpression := extractionStringFromAny(payload["supported_expression"])
	if candidate.class == "habit" {
		traitDomain = "behavior_tendency"
		traitKey = extractionStringFromAny(payload["behavior_key"])
		supportedExpression = extractionStringFromAny(payload["behavior_expression"])
	}
	return map[string]any{
		"profile_section":      extractionStringFromAny(payload["profile_section"]),
		"trait_domain":         traitDomain,
		"trait_key":            traitKey,
		"supported_expression": supportedExpression,
		"observation_kind":     payload["observation_kind"],
		"context":              characterProjectionContext(payload),
		"counterpart":          characterProjectionCounterpart(candidate),
		"authority_class":      "support_hypothesis",
		"promotion_state":      "not_evaluated",
		"source_class":         map[bool]string{true: "habit_evidence", false: "profile_observation"}[candidate.class == "habit"],
		"source_ref":           characterProjectionSourceRef(candidate),
	}
}

func replaceCurrentProfileObservation(existing []any, next map[string]any) []any {
	domain := extractionStringFromAny(next["trait_domain"])
	key := extractionStringFromAny(next["trait_key"])
	out := make([]any, 0, len(existing)+1)
	replaced := false
	for _, raw := range existing {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["trait_domain"]) == domain && extractionStringFromAny(item["trait_key"]) == key {
			if !replaced {
				out = append(out, next)
				replaced = true
			}
			continue
		}
		out = append(out, raw)
	}
	if !replaced {
		out = append(out, next)
	}
	return out
}

func newVoiceBehaviorProjection(subjectEntityID, label string) map[string]any {
	return map[string]any{
		"contract_version":            voiceBehaviorProjectionContractVersion,
		"subject_entity_id":           subjectEntityID,
		"subject_label":               label,
		"authority_class":             "support_hypothesis",
		"disposition":                 "evidence_only",
		"principles":                  []any{},
		"evidence_fingerprints":       []string{},
		"source_turns":                []any{},
		"example_dialogue_projection": false,
		"example_dialogue_storage":    "precise_memory_only",
		"fixed_count_promotion":       false,
		"cross_domain_inference":      false,
		"unsupported_entailment": []string{
			"personality_from_voice", "morality_from_voice", "emotion_from_voice",
			"relationship_state_from_voice", "consent_from_voice",
		},
	}
}

func mergeVoiceBehaviorEvidence(projection map[string]any, candidate characterProjectionUnit) {
	domain := extractionStringFromAny(candidate.payload["trait_domain"])
	principleKey := extractionStringFromAny(candidate.payload["principle_key"])
	principles := sliceFromAny(projection["principles"])
	found := -1
	for index, raw := range principles {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["trait_domain"]) == domain && extractionStringFromAny(item["principle_key"]) == principleKey {
			found = index
			break
		}
	}
	principle := map[string]any{
		"trait_domain":         domain,
		"principle_key":        principleKey,
		"authority_class":      "support_hypothesis",
		"promotion_state":      "not_evaluated",
		"support_refs":         []any{},
		"counterevidence_refs": []any{},
		"exception_refs":       []any{},
		"contexts":             []any{},
		"counterparts":         []any{},
		"state_modulations":    []any{},
		"unsupported_entailment": []string{
			"personality_from_voice", "morality_from_voice", "emotion_from_voice",
			"relationship_state_from_voice", "consent_from_voice",
		},
		"example_dialogue_projected": false,
	}
	if found >= 0 {
		principle = mapFromAny(principles[found])
	}
	ref := voiceBehaviorEvidenceRef(candidate)
	switch extractionStringFromAny(candidate.payload["observation_kind"]) {
	case "counterexample":
		principle["counterevidence_refs"] = append(sliceFromAny(principle["counterevidence_refs"]), ref)
	case "exception":
		principle["exception_refs"] = append(sliceFromAny(principle["exception_refs"]), ref)
	default:
		principle["support_refs"] = append(sliceFromAny(principle["support_refs"]), ref)
	}
	principle["support_observation_count"] = len(sliceFromAny(principle["support_refs"]))
	principle["repeated_support_observed"] = len(sliceFromAny(principle["support_refs"])) > 1
	principle["contexts"] = appendUniqueProjectionObject(principle["contexts"], characterProjectionContext(candidate.payload), "context_key", "context_expression")
	principle["counterparts"] = appendUniqueProjectionObject(principle["counterparts"], characterProjectionCounterpart(candidate), "counterpart_entity_id", "counterpart_expression")
	principle["state_modulations"] = appendUniqueProjectionObject(principle["state_modulations"], characterProjectionStateModulation(candidate.payload), "state_modulation_key", "state_modulation_expression")
	if found >= 0 {
		principles[found] = principle
	} else {
		principles = append(principles, principle)
	}
	sort.SliceStable(principles, func(i, j int) bool {
		left, right := mapFromAny(principles[i]), mapFromAny(principles[j])
		return extractionStringFromAny(left["trait_domain"])+"\x00"+extractionStringFromAny(left["principle_key"]) <
			extractionStringFromAny(right["trait_domain"])+"\x00"+extractionStringFromAny(right["principle_key"])
	})
	projection["principles"] = principles
	projection["evidence_fingerprints"] = append(stringsFromAny(projection["evidence_fingerprints"]), candidate.fingerprint)
	projection["source_turns"] = habitEvidenceAppendUniqueTurn(projection["source_turns"], candidate.unit.SourceTurnEnd)
	projection["latest_source"] = ref
}

func voiceBehaviorEvidenceRef(candidate characterProjectionUnit) map[string]any {
	ref := characterProjectionSourceRef(candidate)
	ref["observation_class"] = characterEvidenceObservationClass(extractionStringFromAny(candidate.payload["observation_kind"]))
	if context := characterProjectionContext(candidate.payload); len(context) > 0 {
		ref["context"] = context
	}
	if counterpart := characterProjectionCounterpart(candidate); len(counterpart) > 0 {
		ref["counterpart"] = counterpart
	}
	if modulation := characterProjectionStateModulation(candidate.payload); len(modulation) > 0 {
		ref["state_modulation"] = modulation
	}
	return ref
}

func characterProjectionContext(payload map[string]any) map[string]any {
	key := extractionStringFromAny(payload["context_key"])
	expression := extractionStringFromAny(payload["context_expression"])
	if key == "" && expression == "" {
		return nil
	}
	return map[string]any{"context_key": nilIfEmpty(key), "context_expression": nilIfEmpty(expression)}
}

func characterProjectionCounterpart(candidate characterProjectionUnit) map[string]any {
	label := extractionStringFromAny(candidate.payload["counterpart"])
	expression := extractionStringFromAny(candidate.payload["counterpart_expression"])
	if label == "" || strings.TrimSpace(candidate.unit.AffectedEntityID) == "" {
		return nil
	}
	return map[string]any{
		"counterpart_entity_id":  candidate.unit.AffectedEntityID,
		"counterpart_label":      label,
		"counterpart_expression": nilIfEmpty(expression),
	}
}

func characterProjectionStateModulation(payload map[string]any) map[string]any {
	key := extractionStringFromAny(payload["state_modulation_key"])
	expression := extractionStringFromAny(payload["state_modulation_expression"])
	if key == "" && expression == "" {
		return nil
	}
	return map[string]any{"state_modulation_key": nilIfEmpty(key), "state_modulation_expression": nilIfEmpty(expression)}
}

func appendUniqueProjectionObject(raw any, candidate map[string]any, keys ...string) []any {
	items := append([]any{}, sliceFromAny(raw)...)
	if len(candidate) == 0 {
		return items
	}
	for _, existingRaw := range items {
		existing := mapFromAny(existingRaw)
		match := true
		for _, key := range keys {
			match = match && extractionStringFromAny(existing[key]) == extractionStringFromAny(candidate[key])
		}
		if match {
			return items
		}
	}
	return append(items, candidate)
}

func characterProjectionSourceRef(candidate characterProjectionUnit) map[string]any {
	unit := candidate.unit
	return map[string]any{
		"chat_session_id":        unit.ChatSessionID,
		"source_contract":        unit.SourceContract,
		"source_revision":        unit.SourceRevision,
		"precise_memory_unit_id": unit.UnitID,
		"source_turn_start":      unit.SourceTurnStart,
		"source_turn_end":        unit.SourceTurnEnd,
		"logical_turn_id":        nilIfEmpty(unit.SourceLogicalTurnID),
		"message_id":             nilIfEmpty(unit.SourceMessageID),
		"generation_id":          nilIfEmpty(unit.SourceGenerationID),
		"content_hash":           unit.SourceContentHash,
		"root_evidence_id":       unit.RootEvidenceID,
		"direct_evidence_ids":    jsonInt64Slice(unit.DirectEvidenceIDsJSON),
		"evidence_hash":          unit.EvidenceHash,
		"visibility":             unit.Visibility,
	}
}

func voicePrincipleKeyReplaysUtterance(principleKey, utteranceExpression string) bool {
	principleKey = normalizeNarrativeStateSlot(principleKey)
	utteranceKey := normalizeNarrativeStateSlot(utteranceExpression)
	return principleKey != "" && utteranceKey != "" && principleKey == utteranceKey
}

func characterEvidenceObservationClass(kind string) string {
	switch strings.TrimSpace(kind) {
	case "counterexample":
		return "counterexample"
	case "exception":
		return "exception"
	default:
		return "support"
	}
}

func characterProjectionFingerprint(rootEvidenceID int64, class string, parts ...string) string {
	contractVersion := characterProfileContractVersion
	if class == "voice" {
		contractVersion = voiceBehaviorProjectionContractVersion
	}
	material := []string{contractVersion, fmt.Sprintf("%d", rootEvidenceID), class}
	material = append(material, parts...)
	sum := sha256.Sum256([]byte(strings.Join(material, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func characterProjectionPreciseUnitID(unit *store.PreciseMemoryUnit) string {
	if unit == nil {
		return ""
	}
	return unit.UnitID
}
