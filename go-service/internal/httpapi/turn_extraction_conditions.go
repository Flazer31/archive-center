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
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	reversibleStateContractVersion = "reversible_state.v1"
	characterBodyStateVersion      = "character_body_state.v1"
	reversibleBodyStatusKey        = "reversible_body_state"
	reversibleLocationStatusKey    = "reversible_location_state"
	reversiblePossessionStatusKey  = "reversible_possession_state"
	reversibleEmotionStatusKey     = "reversible_emotion_state"
	reversibleEntityStatusKey      = "reversible_entity_condition_state"
	reversibleStateOwnerScope      = "fictional_entity"
)

var (
	reversibleStateDomains = map[string]string{
		"body":             reversibleBodyStatusKey,
		"location":         reversibleLocationStatusKey,
		"possession":       reversiblePossessionStatusKey,
		"emotion":          reversibleEmotionStatusKey,
		"entity_condition": reversibleEntityStatusKey,
	}
	reversibleStateTransitions = map[string]bool{"set": true, "change": true, "recover": true, "clear": true}
	reversibleStateScenes      = map[string]bool{"current": true, "flashback": true, "planned": true, "hypothetical": true}
	reversibleStateAuthorities = map[string]bool{"canonical_in_fiction": true, "derived_estimate": true, "needs_review": true}
	reversibleStateAssertions  = map[string]bool{"literal": true, "figurative": true, "decorative": true}
	reversibleStatePolarities  = map[string]bool{"affirmative": true, "negative": true, "uncertain": true}
	reversibleStateVisibility  = map[string]bool{"public": true, "private": true, "unknown": true}
	reversibleStateSensitivity = map[string]bool{"ordinary": true, "sensitive": true, "reproductive": true}
	reversibleBodyCategories   = map[string]bool{"ordinary": true, "medical": true, "reproductive": true}
)

func reversibleStatusKeys() []string {
	return []string{
		reversibleBodyStatusKey,
		reversibleLocationStatusKey,
		reversiblePossessionStatusKey,
		reversibleEmotionStatusKey,
		reversibleEntityStatusKey,
	}
}

func validateReversibleStateProposal(value any) error {
	raw, ok := value.(map[string]any)
	if !ok {
		return errors.New("critic schema reversible_states item must be an object")
	}
	allowed := map[string]bool{
		"version": true, "domain": true, "transition": true, "subject_name": true,
		"state_slot": true, "value": true, "evidence_excerpt": true, "scene_scope": true,
		"authority": true, "assertion_kind": true, "polarity": true, "validity": true,
		"visibility": true, "sensitivity": true,
	}
	for key := range raw {
		if !allowed[key] {
			return fmt.Errorf("critic schema reversible_states field %s is not allowed", key)
		}
	}
	if strings.TrimSpace(extractionStringFromAny(raw["version"])) != reversibleStateContractVersion {
		return errors.New("critic schema reversible_states.version is invalid")
	}
	domain := strings.TrimSpace(extractionStringFromAny(raw["domain"]))
	if reversibleStateDomains[domain] == "" {
		return errors.New("critic schema reversible_states.domain is invalid")
	}
	transition := strings.TrimSpace(extractionStringFromAny(raw["transition"]))
	if !reversibleStateTransitions[transition] {
		return errors.New("critic schema reversible_states.transition is invalid")
	}
	for _, field := range []string{"subject_name", "state_slot", "evidence_excerpt"} {
		if _, ok := raw[field].(string); !ok || strings.TrimSpace(extractionStringFromAny(raw[field])) == "" {
			return fmt.Errorf("critic schema reversible_states.%s must be a non-empty string", field)
		}
	}
	if normalizeReversibleStateSlot(extractionStringFromAny(raw["state_slot"])) == "" {
		return errors.New("critic schema reversible_states.state_slot has no stable key characters")
	}
	if !reversibleStateScenes[strings.TrimSpace(extractionStringFromAny(raw["scene_scope"]))] {
		return errors.New("critic schema reversible_states.scene_scope is invalid")
	}
	if !reversibleStateAuthorities[strings.TrimSpace(extractionStringFromAny(raw["authority"]))] {
		return errors.New("critic schema reversible_states.authority is invalid")
	}
	if !reversibleStateAssertions[strings.TrimSpace(extractionStringFromAny(raw["assertion_kind"]))] {
		return errors.New("critic schema reversible_states.assertion_kind is invalid")
	}
	if !reversibleStatePolarities[strings.TrimSpace(extractionStringFromAny(raw["polarity"]))] {
		return errors.New("critic schema reversible_states.polarity is invalid")
	}
	if !reversibleStateVisibility[strings.TrimSpace(extractionStringFromAny(raw["visibility"]))] {
		return errors.New("critic schema reversible_states.visibility is invalid")
	}
	sensitivity := strings.TrimSpace(extractionStringFromAny(raw["sensitivity"]))
	if !reversibleStateSensitivity[sensitivity] {
		return errors.New("critic schema reversible_states.sensitivity is invalid")
	}
	valueMap := mapFromAny(raw["value"])
	if transition == "set" || transition == "change" {
		if len(valueMap) == 0 || strings.TrimSpace(extractionStringFromAny(valueMap["text"])) == "" {
			return errors.New("critic schema reversible_states set/change requires value.text")
		}
		if err := validateReversibleStateValue(domain, sensitivity, valueMap); err != nil {
			return err
		}
	} else if len(valueMap) > 0 {
		return errors.New("critic schema reversible_states recover/clear must not invent a replacement value")
	}
	if validity, exists := raw["validity"]; exists {
		validityMap, ok := validity.(map[string]any)
		if !ok {
			return errors.New("critic schema reversible_states.validity must be an object")
		}
		for key, value := range validityMap {
			if key != "valid_from" && key != "valid_to" {
				return fmt.Errorf("critic schema reversible_states.validity.%s is not allowed", key)
			}
			if _, ok := value.(string); !ok || strings.TrimSpace(extractionStringFromAny(value)) == "" {
				return fmt.Errorf("critic schema reversible_states.validity.%s must be a non-empty string", key)
			}
		}
		if err := validateReversibleValidityRange(validityMap); err != nil {
			return err
		}
	}
	return nil
}

func validateReversibleStateValue(domain, sensitivity string, value map[string]any) error {
	for key := range value {
		if key != "text" && key != "body" {
			return fmt.Errorf("critic schema reversible_states.value.%s is not allowed", key)
		}
	}
	if _, ok := value["text"].(string); !ok {
		return errors.New("critic schema reversible_states.value.text must be a string")
	}
	body := mapFromAny(value["body"])
	if domain != "body" {
		if len(body) > 0 {
			return errors.New("critic schema reversible_states.value.body is only valid for body domain")
		}
		return nil
	}
	if len(body) == 0 {
		return errors.New("critic schema reversible_states body value requires body metadata")
	}
	for key, raw := range body {
		if key != "subtype" && key != "affected_area" && key != "category" {
			return fmt.Errorf("critic schema reversible_states.value.body.%s is not allowed", key)
		}
		if _, ok := raw.(string); !ok {
			return fmt.Errorf("critic schema reversible_states.value.body.%s must be a string", key)
		}
	}
	category := strings.TrimSpace(extractionStringFromAny(body["category"]))
	if !reversibleBodyCategories[category] {
		return errors.New("critic schema reversible_states.value.body.category is invalid")
	}
	if strings.TrimSpace(extractionStringFromAny(body["subtype"])) == "" {
		return errors.New("critic schema reversible_states.value.body.subtype must be exact source text")
	}
	if category == "medical" && sensitivity != "sensitive" {
		return errors.New("critic schema reversible_states medical body state must be sensitive")
	}
	if category == "reproductive" && sensitivity != "reproductive" {
		return errors.New("critic schema reversible_states reproductive body state must remain reproductive")
	}
	if category == "ordinary" && sensitivity != "ordinary" {
		return errors.New("critic schema reversible_states ordinary body state sensitivity is inconsistent")
	}
	return nil
}

func validateReversibleValidityRange(validity map[string]any) error {
	fromRaw := strings.TrimSpace(extractionStringFromAny(validity["valid_from"]))
	toRaw := strings.TrimSpace(extractionStringFromAny(validity["valid_to"]))
	if fromRaw == "" || toRaw == "" {
		return nil
	}
	from, fromOK := parseReversibleAbsolute(fromRaw)
	to, toOK := parseReversibleAbsolute(toRaw)
	if fromOK && toOK && to.Before(from) {
		return errors.New("critic schema reversible_states validity range is reversed")
	}
	return nil
}

func parseReversibleAbsolute(raw string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeReversibleStateProposals(value any) []map[string]any {
	out := []map[string]any{}
	candidates := sliceFromAny(value)
	if typed, ok := value.([]map[string]any); ok {
		candidates = make([]any, 0, len(typed))
		for _, candidate := range typed {
			candidates = append(candidates, candidate)
		}
	}
	for _, candidate := range candidates {
		raw := mapFromAny(candidate)
		if validateReversibleStateProposal(raw) != nil {
			continue
		}
		normalized := map[string]any{
			"version":          reversibleStateContractVersion,
			"domain":           strings.TrimSpace(extractionStringFromAny(raw["domain"])),
			"transition":       strings.TrimSpace(extractionStringFromAny(raw["transition"])),
			"subject_name":     strings.TrimSpace(extractionStringFromAny(raw["subject_name"])),
			"state_slot":       normalizeReversibleStateSlot(extractionStringFromAny(raw["state_slot"])),
			"evidence_excerpt": strings.TrimSpace(extractionStringFromAny(raw["evidence_excerpt"])),
			"scene_scope":      strings.TrimSpace(extractionStringFromAny(raw["scene_scope"])),
			"authority":        strings.TrimSpace(extractionStringFromAny(raw["authority"])),
			"assertion_kind":   strings.TrimSpace(extractionStringFromAny(raw["assertion_kind"])),
			"polarity":         strings.TrimSpace(extractionStringFromAny(raw["polarity"])),
			"visibility":       strings.TrimSpace(extractionStringFromAny(raw["visibility"])),
			"sensitivity":      strings.TrimSpace(extractionStringFromAny(raw["sensitivity"])),
		}
		if valueMap := mapFromAny(raw["value"]); len(valueMap) > 0 {
			normalized["value"] = storyClockJSONMap(valueMap)
		}
		if validity := mapFromAny(raw["validity"]); len(validity) > 0 {
			normalized["validity"] = storyClockJSONMap(validity)
		}
		out = append(out, normalized)
	}
	return out
}

func normalizeReversibleStateSlot(raw string) string {
	var out []rune
	separatorPending := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if separatorPending && len(out) > 0 {
				out = append(out, '_')
			}
			out = append(out, r)
			separatorPending = false
		default:
			separatorPending = true
		}
	}
	return strings.Trim(string(out), "_")
}

func reversibleStateSourceUnitID(sourceRevision, domain, subjectEntityID, slot string, sourceOrdinal int) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		reversibleStateContractVersion,
		strings.TrimSpace(sourceRevision),
		strings.TrimSpace(domain),
		strings.TrimSpace(subjectEntityID),
		strings.TrimSpace(slot),
		fmt.Sprint(sourceOrdinal),
	}, "\x1f")))
	return "reversible:" + hex.EncodeToString(sum[:])
}

func reversibleStateSubject(projection *entityIdentityProjection, domain, subject string) (*entityIdentityOccurrence, string) {
	if projection == nil {
		return nil, "entity_identity_projection_unavailable"
	}
	occurrence, _, ambiguous := projection.resolveUnique(subject)
	if ambiguous {
		return nil, "subject_identity_ambiguous"
	}
	if occurrence == nil {
		return nil, "subject_identity_unavailable"
	}
	if !map[string]bool{
		"session_npc": true, "session_player": true, "session_item": true,
		"session_location": true, "session_group": true,
	}[occurrence.Namespace] {
		return nil, "subject_namespace_not_fictional"
	}
	if (domain == "body" || domain == "emotion" || domain == "possession") &&
		occurrence.EntityKind != "character" && occurrence.EntityKind != "player" {
		return nil, "subject_kind_not_character"
	}
	return occurrence, ""
}

func reversibleProjectionFromCurrent(current store.StatusCurrentValue, domain, subjectID, subjectLabel string) (map[string]any, error) {
	if current.ID == 0 || strings.TrimSpace(current.ValueJSON) == "" {
		return map[string]any{
			"version":           reversibleStateContractVersion,
			"domain":            domain,
			"subject_entity_id": subjectID,
			"subject_label":     subjectLabel,
			"slots":             map[string]any{},
		}, nil
	}
	projection := map[string]any{}
	if json.Unmarshal([]byte(current.ValueJSON), &projection) != nil ||
		strings.TrimSpace(extractionStringFromAny(projection["version"])) != reversibleStateContractVersion ||
		strings.TrimSpace(extractionStringFromAny(projection["domain"])) != domain ||
		strings.TrimSpace(extractionStringFromAny(projection["subject_entity_id"])) != subjectID {
		return nil, errors.New("malformed reversible current projection")
	}
	slots, ok := projection["slots"].(map[string]any)
	if !ok {
		return nil, errors.New("malformed reversible current slots")
	}
	projection["slots"] = storyClockJSONMap(slots)
	projection["subject_label"] = subjectLabel
	return projection, nil
}

func reversibleObservationContext(ctx context.Context, st store.Store, sid string) map[string]any {
	out := map[string]any{"kind": "source_observation", "story_time": "unknown"}
	valueStore, ok := st.(store.StatusCurrentValueStore)
	if !ok {
		return out
	}
	values, err := valueStore.ListStatusCurrentValues(ctx, sid, storyClockOwnerScope, storyClockOwnerID, storyClockStatusKey, 1)
	if err != nil {
		return out
	}
	current := storyClockCurrentValue(values)
	if current.ID == 0 {
		return out
	}
	storyClock := map[string]any{}
	if json.Unmarshal([]byte(current.ValueJSON), &storyClock) == nil {
		out["story_clock"] = storyClock
		delete(out, "story_time")
	}
	return out
}

func ensureReversibleStateDefinition(ctx context.Context, st store.Store, sid, domain string, now time.Time, result *artifactSaveResult) (store.StatusSchemaDefinition, bool) {
	registry, ok := st.(store.StatusSchemaRegistryStore)
	if !ok {
		result.addSkipReason("reversible_states", "status_schema_registry_unavailable", map[string]any{"domain": domain})
		return store.StatusSchemaDefinition{}, false
	}
	statusKey := reversibleStateDomains[domain]
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, reversibleStateOwnerScope)
	if err == nil {
		return definition, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		result.addSkipReason("reversible_states", "status_schema_lookup_failed", err.Error())
		return store.StatusSchemaDefinition{}, false
	}
	result.Attempted++
	definitions, err := registry.SaveStatusSchemaDefinitions(ctx, []store.StatusSchemaDefinition{{
		ChatSessionID: sid,
		SchemaName:    "reversible_" + domain + "_state",
		StatusKey:     statusKey,
		Label:         "Reversible " + domain + " state",
		OwnerScope:    reversibleStateOwnerScope,
		ValueKind:     "object",
		OptionsJSON: mustCompactJSON(map[string]any{
			"contract_version":        reversibleStateContractVersion,
			"fixed_domain_status_key": true,
			"atomic_slots_projection": true,
			"story_time_only":         true,
			"body_contract_version":   nilIfEmpty(map[string]string{"body": characterBodyStateVersion}[domain]),
		}),
		RegistryState: "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}})
	if err != nil || len(definitions) == 0 {
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveStatusSchemaDefinitions("+statusKey+"): "+err.Error())
		}
		return store.StatusSchemaDefinition{}, false
	}
	result.StatusSchemaDefinitions++
	return definitions[0], true
}

func reversibleStateOwnerID(
	ctx context.Context,
	st store.Store,
	sid string,
	subject *entityIdentityOccurrence,
	transition string,
	currentValues []store.StatusCurrentValue,
) (string, bool, string) {
	if subject == nil {
		return "", false, "subject_identity_unavailable"
	}
	occurrenceID := strings.TrimSpace(subject.StableEntityID)
	for _, current := range currentValues {
		if strings.TrimSpace(current.OwnerID) == occurrenceID {
			return occurrenceID, true, ""
		}
	}
	if resolver, ok := st.(store.ReviewedEntityIdentityResolver); ok {
		rootID, err := resolver.ResolveReviewedCanonicalEntityID(ctx, sid, occurrenceID)
		if err == nil && strings.TrimSpace(rootID) != "" {
			return strings.TrimSpace(rootID), true, ""
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return occurrenceID, false, "reviewed_identity_root_ambiguous_or_unavailable"
		}
	}
	if transition == "set" {
		return occurrenceID, true, ""
	}
	return occurrenceID, false, "identity_continuity_unresolved_history_only"
}

func reversibleStateCurrentClaimEligibility(extraction, proposal map[string]any, excerpt string) (bool, string) {
	if extractionStringFromAny(proposal["polarity"]) != "affirmative" {
		return false, "nonaffirmative_claim_history_only"
	}
	if reversibleStateExcerptHasEpistemicOperator(excerpt, extractionStringFromAny(proposal["subject_name"])) {
		return false, "source_epistemic_guard_history_only"
	}
	if extractionStringFromAny(proposal["domain"]) == "body" {
		valueText := extractionStringFromAny(mapFromAny(proposal["value"])["text"])
		if valueText != "" && !reversibleStateValueCoversSourceAssertion(
			excerpt,
			extractionStringFromAny(proposal["subject_name"]),
			valueText,
		) {
			return false, "body_assertion_span_guard_history_only"
		}
	}
	for _, lane := range []string{"state_claims", "belief_updates", "narrative_events"} {
		for _, raw := range sliceFromAny(extraction[lane]) {
			item := mapFromAny(raw)
			if strings.TrimSpace(extractionStringFromAny(item["evidence_excerpt"])) != strings.TrimSpace(excerpt) {
				continue
			}
			if lane == "belief_updates" || preciseMemoryStructuredClassification(item) != "objective" {
				return false, "source_epistemic_guard_history_only"
			}
		}
	}
	return true, ""
}

func reversibleStateValueCoversSourceAssertion(excerpt, subjectName, valueText string) bool {
	normalize := func(raw string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, raw)
	}
	assertion := normalize(excerpt)
	value := normalize(valueText)
	if assertion == "" || value == "" {
		return false
	}
	if assertion == value {
		return true
	}
	subject := normalize(subjectName)
	if subject == "" {
		return false
	}
	withoutSubject := strings.Replace(assertion, subject, "", 1)
	if withoutSubject == value {
		return true
	}
	// A single attached possessive/topic/case particle may remain after the
	// exact subject span is removed (for example English "'s" or Korean "는").
	prefix := strings.TrimSuffix(withoutSubject, value)
	return strings.HasSuffix(withoutSubject, value) && len([]rune(prefix)) <= 1
}

// This fixed safety lexicon is defense-in-depth for critic misclassification,
// not a content-generation policy. A typed affirmative label alone must never
// promote visibly negated, questioned, or uncertain prose into current state.
func reversibleStateExcerptHasEpistemicOperator(excerpt, subjectName string) bool {
	assertion := strings.ToLower(strings.TrimSpace(excerpt))
	if subject := strings.ToLower(strings.TrimSpace(subjectName)); subject != "" {
		assertion = strings.Replace(assertion, subject, "", 1)
	}
	if strings.Contains(assertion, "n't") || strings.Contains(assertion, "n’t") {
		return true
	}
	for _, r := range assertion {
		switch r {
		case '?', '？', '؟', '՞', '፧':
			return true
		}
	}
	operators := map[string]bool{
		"no": true, "not": true, "never": true, "neither": true, "nor": true, "without": true,
		"cannot": true, "cant": true,
		"maybe": true, "perhaps": true, "possibly": true, "possible": true, "might": true,
		"allegedly": true, "reportedly": true, "seem": true, "seems": true, "seemed": true,
		"appear": true, "appears": true, "appeared": true, "unlikely": true,
		"uncertain": true, "unsure": true, "doubt": true, "doubts": true, "doubted": true,
		"deny": true, "denies": true, "denied": true, "denying": true,
	}
	tokens := strings.FieldsFunc(assertion, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	for index, token := range tokens {
		if operators[token] {
			return true
		}
		if (token == "may" || token == "could") && index+1 < len(tokens) &&
			(tokens[index+1] == "be" || tokens[index+1] == "have") {
			return true
		}
	}
	compact := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}, assertion)
	for _, marker := range []string{
		"않", "아니", "못", "없",
		"아마", "모르", "모른", "의심", "부인", "부정", "듯", "추정",
	} {
		if strings.Contains(compact, marker) {
			return true
		}
	}
	return false
}

func (s *Server) saveReversibleStatesFromExtraction(
	ctx context.Context,
	sid string,
	turnIndex int,
	extraction map[string]any,
	content string,
	evidence []store.DirectEvidence,
	identities *entityIdentityProjection,
	now time.Time,
	result *artifactSaveResult,
) {
	if s == nil || s.Store == nil || result == nil {
		return
	}
	proposals := normalizeReversibleStateProposals(extraction["reversible_states"])
	if len(proposals) == 0 {
		return
	}
	atomicStore, atomicOK := s.Store.(store.ReversibleStatusTransitionStore)
	if !atomicOK {
		result.addSkipReason("reversible_states", "atomic_reversible_status_store_unavailable", map[string]any{"items": len(proposals)})
		return
	}
	source, accepted := storyClockSourceMetadata(ctx, sid, turnIndex, content)
	if !accepted {
		result.addSkipReason("reversible_states", "accepted_source_required", nil)
		return
	}
	observationContext := reversibleObservationContext(ctx, s.Store, sid)
	for index, proposal := range proposals {
		domain := extractionStringFromAny(proposal["domain"])
		statusKey := reversibleStateDomains[domain]
		excerpt := sanitizeEvidenceExcerptForTurn(extractionStringFromAny(proposal["evidence_excerpt"]), content)
		proposal["evidence_excerpt"] = excerpt
		evidenceIDs := []int64{}
		if excerpt != "" {
			evidenceIDs = storyClockMatchingEvidenceIDs(evidence, sid, turnIndex, excerpt)
		}
		subjectName := extractionStringFromAny(proposal["subject_name"])
		subject, subjectReason := reversibleStateSubject(identities, domain, subjectName)
		if subject == nil {
			result.addSkipReason("reversible_states", subjectReason, map[string]any{"index": index, "subject_name": subjectName})
			continue
		}
		valueMap := mapFromAny(proposal["value"])
		validity := mapFromAny(proposal["validity"])
		if err := validateReversibleValidityRange(validity); err != nil {
			result.addSkipReason("reversible_states", "validity_range_reversed", map[string]any{"index": index})
			continue
		}
		allCurrentValues, err := atomicStore.ListReversibleStatusCurrentValues(ctx, sid, reversibleStateOwnerScope, []string{statusKey})
		if err != nil {
			result.addSkipReason("reversible_states", "current_projection_read_failed", err.Error())
			continue
		}
		slot := strings.TrimSpace(extractionStringFromAny(proposal["state_slot"]))
		transition := extractionStringFromAny(proposal["transition"])
		ownerID, identityCurrentEligible, identityResolution := reversibleStateOwnerID(
			ctx, s.Store, sid, subject, transition, allCurrentValues,
		)
		var previous store.StatusCurrentValue
		newerSameLabelTurn := 0
		for _, candidate := range allCurrentValues {
			if candidate.OwnerID == ownerID {
				previous = candidate
			}
			if comparableEntityKey(candidate.OwnerLabel) == comparableEntityKey(subject.Name) &&
				candidate.SourceTurn > newerSameLabelTurn {
				newerSameLabelTurn = candidate.SourceTurn
			}
		}
		// Exact replay follows the accepted source occurrence identity. The
		// proposal ordinal keeps multiple same-slot observations from the same
		// accepted source distinct without inventing evidence.
		sourceUnitID := reversibleStateSourceUnitID(
			source.Revision, domain, subject.StableEntityID, slot, index,
		)
		if _, err := atomicStore.GetReversibleStatusEventBySourceUnit(ctx, sid, source.Revision, sourceUnitID); err == nil {
			result.addSkipReason("reversible_states", "source_unit_replay_idempotent", map[string]any{"source_unit_id": sourceUnitID})
			continue
		} else if !errors.Is(err, store.ErrNotFound) {
			result.addSkipReason("reversible_states", "source_unit_lookup_failed", err.Error())
			continue
		}
		definition, ok := ensureReversibleStateDefinition(ctx, s.Store, sid, domain, now, result)
		if !ok {
			continue
		}
		projection, err := reversibleProjectionFromCurrent(previous, domain, ownerID, subject.Name)
		if err != nil {
			result.addSkipReason("reversible_states", "current_projection_malformed", err.Error())
			continue
		}
		previousJSON := mustCompactJSON(projection)
		claimCurrentEligible, claimResolution := reversibleStateCurrentClaimEligibility(extraction, proposal, excerpt)
		explicitPrivate := extractionStringFromAny(proposal["visibility"]) != "public" ||
			extractionStringFromAny(proposal["sensitivity"]) == "reproductive"
		currentAllowed := identityCurrentEligible && claimCurrentEligible && !explicitPrivate &&
			extractionStringFromAny(proposal["scene_scope"]) == "current" &&
			extractionStringFromAny(proposal["authority"]) == "canonical_in_fiction" &&
			extractionStringFromAny(proposal["assertion_kind"]) == "literal"
		resolutionStatus := identityResolution
		slots := mapFromAny(projection["slots"])
		_, priorSlotExists := slots[slot]
		if !identityCurrentEligible {
			currentAllowed = false
		} else if !claimCurrentEligible {
			currentAllowed = false
			resolutionStatus = claimResolution
		} else if explicitPrivate {
			currentAllowed = false
			resolutionStatus = "private_or_sensitive_history_only"
		} else if newerSameLabelTurn > turnIndex {
			currentAllowed = false
			resolutionStatus = "older_turn_same_label_history_only"
		} else if (transition == "change" || transition == "recover" || transition == "clear") && !priorSlotExists {
			currentAllowed = false
			resolutionStatus = "prior_slot_missing_history_only"
		} else if previous.SourceTurn > turnIndex {
			currentAllowed = false
			resolutionStatus = "older_turn_historical_only"
		} else if extractionStringFromAny(proposal["scene_scope"]) != "current" {
			resolutionStatus = "non_current_scene_history_only"
		} else if extractionStringFromAny(proposal["authority"]) != "canonical_in_fiction" {
			resolutionStatus = "noncanonical_authority_history_only"
		} else if extractionStringFromAny(proposal["assertion_kind"]) != "literal" {
			resolutionStatus = "nonliteral_assertion_history_only"
		}
		if currentAllowed {
			if transition == "set" || transition == "change" {
				valueCopy := storyClockJSONMap(valueMap)
				if domain == "body" {
					valueCopy["contract_version"] = characterBodyStateVersion
				}
				slots[slot] = map[string]any{
					"state_slot":  slot,
					"value":       valueCopy,
					"validity":    reversibleStateValidity(validity, observationContext),
					"visibility":  proposal["visibility"],
					"sensitivity": proposal["sensitivity"],
					"source": map[string]any{
						"source_revision":             source.Revision,
						"source_unit_id":              sourceUnitID,
						"source_turn":                 turnIndex,
						"direct_evidence_ids":         evidenceIDs,
						"source_entity_occurrence_id": subject.StableEntityID,
						"subject_owner_id":            ownerID,
					},
				}
			} else {
				delete(slots, slot)
			}
			projection["slots"] = slots
			projection["source_turn"] = turnIndex
		}
		newProjectionJSON := mustCompactJSON(projection)
		evidencePayload := map[string]any{
			"contract_version":     reversibleStateContractVersion,
			"source":               "critic.reversible_states",
			"source_turn":          turnIndex,
			"source_revision":      source.Revision,
			"source_unit_id":       sourceUnitID,
			"source_entity_id":     subject.StableEntityID,
			"subject_owner_id":     ownerID,
			"logical_turn_id":      source.LogicalTurnID,
			"source_message_id":    source.MessageID,
			"source_generation_id": source.GenerationID,
			"content_hash":         source.ContentHash,
			"direct_evidence_ids":  evidenceIDs,
			"evidence_excerpt":     excerpt,
			"current_projection":   currentAllowed,
			"history_observation":  proposal,
			"observed_at":          observationContext,
		}
		if resolutionStatus != "" {
			evidencePayload["resolution_status"] = resolutionStatus
		}
		var currentValue *store.StatusCurrentValue
		if currentAllowed {
			currentValue = &store.StatusCurrentValue{
				ChatSessionID: sid,
				RegistryID:    definition.ID,
				StatusKey:     statusKey,
				OwnerScope:    reversibleStateOwnerScope,
				OwnerID:       ownerID,
				OwnerLabel:    subject.Name,
				ValueKind:     "object",
				ValueJSON:     newProjectionJSON,
				EvidenceJSON:  mustCompactJSON(evidencePayload),
				SourceTurn:    turnIndex,
				WriteState:    "current",
				CreatedAt:     now,
				UpdatedAt:     now,
			}
		}
		event := store.StatusChangeEvent{
			ChatSessionID:     sid,
			RegistryID:        definition.ID,
			StatusKey:         statusKey,
			OwnerScope:        reversibleStateOwnerScope,
			OwnerID:           ownerID,
			EventKind:         transition,
			PreviousValueJSON: previousJSON,
			NewValueJSON:      newProjectionJSON,
			EvidenceJSON:      mustCompactJSON(evidencePayload),
			SourceTurn:        turnIndex,
			StoryClockJSON:    reversibleObservationStoryClockJSON(observationContext),
			EventState:        map[bool]string{true: "recorded", false: "history_only"}[currentAllowed],
			CreatedAt:         now,
		}
		result.Attempted++
		saved, err := atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
			SourceContract: source.ContractVersion,
			SourceRevision: source.Revision,
			SourceUnitID:   sourceUnitID,
			CurrentValue:   currentValue,
			Event:          event,
		})
		if errors.Is(err, store.ErrStatusProjectionStale) && currentAllowed {
			evidencePayload["current_projection"] = false
			evidencePayload["resolution_status"] = "concurrent_newer_projection_history_only"
			event.EvidenceJSON = mustCompactJSON(evidencePayload)
			event.EventState = "history_only"
			saved, err = atomicStore.ApplyReversibleStatusTransition(ctx, store.ReversibleStatusTransition{
				SourceContract: source.ContractVersion,
				SourceRevision: source.Revision,
				SourceUnitID:   sourceUnitID,
				Event:          event,
			})
			currentAllowed = false
		}
		if err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "ApplyReversibleStatusTransition("+statusKey+"): "+err.Error())
			continue
		}
		if saved.Replayed {
			result.addSkipReason("reversible_states", "source_unit_replay_idempotent", map[string]any{"source_unit_id": sourceUnitID})
			continue
		}
		result.NarrativeStateEvents++
		if currentAllowed {
			result.NarrativeCurrentStates++
		}
		switch domain {
		case "body":
			result.PhysicalConditions++
		case "entity_condition":
			result.EntityConditions++
		}
	}
}

func reversibleStateValidity(asserted, observedAt map[string]any) map[string]any {
	out := map[string]any{"observed_at": storyClockJSONMap(observedAt)}
	for _, key := range []string{"valid_from", "valid_to"} {
		if value := strings.TrimSpace(extractionStringFromAny(asserted[key])); value != "" {
			out[key] = value
		}
	}
	return out
}

func reversibleObservationStoryClockJSON(observedAt map[string]any) string {
	if clock := mapFromAny(observedAt["story_clock"]); len(clock) > 0 {
		return mustCompactJSON(clock)
	}
	return ""
}

func restoreReversibleStateCurrentAfterRollback(ctx context.Context, st store.Store, sid string) (int, error) {
	atomicStore, atomicOK := st.(store.ReversibleStatusTransitionStore)
	currentStore, currentOK := st.(store.StatusCurrentValueStore)
	if !atomicOK || !currentOK {
		return 0, nil
	}
	events, err := atomicStore.ListLatestReversibleCurrentProjectionEvents(ctx, sid, reversibleStatusKeys())
	if err != nil {
		return 0, err
	}
	restored := 0
	for _, event := range events {
		if strings.TrimSpace(event.NewValueJSON) == "" {
			continue
		}
		projection := map[string]any{}
		if json.Unmarshal([]byte(event.NewValueJSON), &projection) != nil ||
			extractionStringFromAny(projection["version"]) != reversibleStateContractVersion {
			return restored, errors.New("malformed reversible projection history")
		}
		label := extractionStringFromAny(projection["subject_label"])
		if _, err := currentStore.SaveStatusCurrentValue(ctx, store.StatusCurrentValue{
			ChatSessionID: sid,
			RegistryID:    event.RegistryID,
			StatusKey:     event.StatusKey,
			OwnerScope:    event.OwnerScope,
			OwnerID:       event.OwnerID,
			OwnerLabel:    label,
			ValueKind:     "object",
			ValueJSON:     event.NewValueJSON,
			EvidenceJSON:  event.EvidenceJSON,
			SourceTurn:    event.SourceTurn,
			WriteState:    "current",
			CreatedAt:     event.CreatedAt,
			UpdatedAt:     time.Now().UTC(),
		}); err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

func buildReversibleStatePacket(
	values []store.StatusCurrentValue,
	currentStoryClock map[string]any,
	maxChars int,
	scope prepareTurnRequestEntityScope,
) (map[string]any, string) {
	items := []map[string]any{}
	lines := []string{}
	excluded := map[string]int{
		"non_public": 0, "sensitive": 0, "reproductive": 0,
		"private_emotion": 0, "outside_validity": 0, "unrelated_subject": 0,
		"ambiguous_subject": 0, "budget": 0,
	}
	eligibleCount := 0
	const header = "[Reversible Current State]\n"
	usedChars := 0
	values = append([]store.StatusCurrentValue(nil), values...)
	subjectRank := func(label string) int {
		if prepareTurnRelationshipNameInList(label, scope.Direct) {
			return 0
		}
		if prepareTurnRelationshipNameInList(label, scope.Scene) {
			return 1
		}
		return 2
	}
	sort.SliceStable(values, func(i, j int) bool {
		leftRank := subjectRank(values[i].OwnerLabel)
		rightRank := subjectRank(values[j].OwnerLabel)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if values[i].StatusKey != values[j].StatusKey {
			return values[i].StatusKey < values[j].StatusKey
		}
		return values[i].OwnerID < values[j].OwnerID
	})
	subjectSlotCounts := map[string]int{}
	for _, current := range values {
		projection := map[string]any{}
		if json.Unmarshal([]byte(current.ValueJSON), &projection) != nil ||
			extractionStringFromAny(projection["version"]) != reversibleStateContractVersion {
			continue
		}
		domain := extractionStringFromAny(projection["domain"])
		subject := extractionStringFromAny(projection["subject_label"])
		for slotName := range mapFromAny(projection["slots"]) {
			key := strings.Join([]string{domain, normalizePrepareTurnEntityNeedle(subject), slotName}, "\x1f")
			subjectSlotCounts[key]++
		}
	}
	for _, current := range values {
		projection := map[string]any{}
		if json.Unmarshal([]byte(current.ValueJSON), &projection) != nil ||
			extractionStringFromAny(projection["version"]) != reversibleStateContractVersion {
			continue
		}
		domain := extractionStringFromAny(projection["domain"])
		subject := extractionStringFromAny(projection["subject_label"])
		if subjectRank(subject) > 1 {
			excluded["unrelated_subject"] += len(mapFromAny(projection["slots"]))
			continue
		}
		slots := mapFromAny(projection["slots"])
		slotNames := make([]string, 0, len(slots))
		for slot := range slots {
			slotNames = append(slotNames, slot)
		}
		sort.Strings(slotNames)
		for _, slotName := range slotNames {
			slot := mapFromAny(slots[slotName])
			subjectSlotKey := strings.Join([]string{domain, normalizePrepareTurnEntityNeedle(subject), slotName}, "\x1f")
			if subjectSlotCounts[subjectSlotKey] > 1 {
				excluded["ambiguous_subject"]++
				continue
			}
			value := mapFromAny(slot["value"])
			text := strings.TrimSpace(extractionStringFromAny(value["text"]))
			if text == "" {
				continue
			}
			if reversibleSlotOutsideComparableValidity(mapFromAny(slot["validity"]), currentStoryClock) {
				excluded["outside_validity"]++
				continue
			}
			visibility := extractionStringFromAny(slot["visibility"])
			sensitivity := extractionStringFromAny(slot["sensitivity"])
			if visibility != "public" {
				excluded["non_public"]++
				if domain == "emotion" {
					excluded["private_emotion"]++
				}
				continue
			}
			if sensitivity == "reproductive" {
				excluded["reproductive"]++
				continue
			}
			if sensitivity != "ordinary" {
				excluded["sensitive"]++
				continue
			}
			item := map[string]any{
				"domain":            domain,
				"subject_entity_id": projection["subject_entity_id"],
				"subject_label":     subject,
				"state_slot":        slotName,
				"value":             value,
				"validity":          slot["validity"],
			}
			eligibleCount++
			line := fmt.Sprintf("- %s [%s/%s]: %s", subject, domain, slotName, text)
			addedChars := len([]rune(line))
			if len(lines) == 0 {
				addedChars += len([]rune(header))
			} else {
				addedChars++
			}
			if maxChars >= 0 && usedChars+addedChars > maxChars {
				excluded["budget"]++
				continue
			}
			items = append(items, item)
			lines = append(lines, line)
			usedChars += addedChars
		}
	}
	packet := map[string]any{
		"version":             reversibleStateContractVersion,
		"status":              map[bool]string{true: "ready", false: "empty"}[len(items) > 0],
		"active_items":        items,
		"active_count":        len(items),
		"eligible_count":      eligibleCount,
		"delivered_count":     len(items),
		"budget_chars":        maxInt(0, maxChars),
		"used_chars":          usedChars,
		"excluded_counts":     excluded,
		"history_included":    false,
		"mid_item_truncation": false,
	}
	text := ""
	if len(lines) > 0 {
		text = header + strings.Join(lines, "\n")
	}
	return packet, text
}

func reversibleSlotOutsideComparableValidity(validity, currentStoryClock map[string]any) bool {
	if extractionStringFromAny(currentStoryClock["version"]) != storyClockContractVersion ||
		extractionStringFromAny(currentStoryClock["observation_kind"]) != "absolute" ||
		extractionStringFromAny(currentStoryClock["precision"]) != "exact" {
		return false
	}
	current, currentKind, ok := parseStoryClockAbsolute(mapFromAny(currentStoryClock["absolute"]))
	if !ok {
		return false
	}
	if fromRaw := strings.TrimSpace(extractionStringFromAny(validity["valid_from"])); fromRaw != "" {
		if from, kind, comparable := parseReversibleComparableAbsolute(fromRaw); comparable && kind == currentKind && current.Before(from) {
			return true
		}
	}
	if toRaw := strings.TrimSpace(extractionStringFromAny(validity["valid_to"])); toRaw != "" {
		if to, kind, comparable := parseReversibleComparableAbsolute(toRaw); comparable && kind == currentKind && current.After(to) {
			return true
		}
	}
	return false
}

func parseReversibleComparableAbsolute(raw string) (time.Time, string, bool) {
	raw = strings.TrimSpace(raw)
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		return parsed, "date", true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, "date_time", true
		}
	}
	return time.Time{}, "", false
}

var legacyReversibleStateFieldKeys = map[string]bool{
	"body_state":          true,
	"current_location":    true,
	"emotion":             true,
	"emotional_posture":   true,
	"emotional_state":     true,
	"health_state":        true,
	"injuries":            true,
	"injury":              true,
	"inventory":           true,
	"physical_condition":  true,
	"physical_conditions": true,
	"possession":          true,
	"possessions":         true,
	"status_emotion":      true,
}

func sanitizeLegacyReversibleMap(value any) map[string]any {
	source := mapFromAny(value)
	out := make(map[string]any, len(source))
	for key, raw := range source {
		if legacyReversibleStateFieldKeys[strings.ToLower(strings.TrimSpace(key))] {
			continue
		}
		out[key] = raw
	}
	return out
}

func sanitizeLegacyReversibleCharacterDeltas(value any) []any {
	out := []any{}
	for _, raw := range sliceFromAny(value) {
		item := sanitizeLegacyReversibleMap(raw)
		// These two top-level fields are unambiguously current per-character
		// projections. A generic status.location may instead be an explicitly
		// durable residence/workplace fact and is therefore preserved.
		delete(item, "location")
		delete(item, "emotional_posture")
		// Character traits and speaking principles are projected only through
		// their 3.9 typed observation lanes. Keep them in the raw extraction,
		// but do not let the legacy character_delta copy bypass those lanes.
		delete(item, "personality")
		delete(item, "speech_style")
		if status, exists := item["status"]; exists {
			cleaned := sanitizeLegacyReversibleMap(status)
			if len(cleaned) == 0 {
				delete(item, "status")
			} else {
				item["status"] = cleaned
			}
		}
		out = append(out, item)
	}
	return out
}

func sanitizeLegacyReversibleStateDeltas(value any) map[string]any {
	// Scene mood/location describe the global scene projection, not a
	// per-entity reversible owner. They remain in state_deltas while
	// per-character location/emotion/body/possession use reversible_states.
	return mapFromAny(value)
}

func comparableEntityKey(raw string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(raw)), " "))
}
