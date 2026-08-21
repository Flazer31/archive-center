package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnCharacterMemoryEnumeratesAllEligibleWithoutCountCap(t *testing.T) {
	const sid = "character-memory-session"
	observations := make([]any, 0, 70)
	active := map[string]bool{}
	for index := 0; index < 70; index++ {
		revision := fmt.Sprintf("revision-%02d", index)
		active[revision] = true
		observations = append(observations, map[string]any{
			"profile_section": "stable", "trait_domain": "values", "trait_key": fmt.Sprintf("principle_%02d", index),
			"supported_expression": fmt.Sprintf("descriptive principle %02d", index), "observation_kind": "explicit_statement",
			"source_ref": prepareTurnCharacterMemoryTestSource(sid, revision, fmt.Sprintf("unit-%02d", index), "public"),
		})
	}
	state := store.CharacterState{
		ChatSessionID: sid,
		CharacterName: "Mira",
		PersonalityJSON: mustCompactJSON(map[string]any{
			"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"stable": map[string]any{"observations": observations}, "current": map[string]any{"observations": []any{}},
			"dynamic": map[string]any{"observations": []any{}}, "relationship_specific": map[string]any{"observations": []any{}},
			"counterevidence": []any{},
		}),
	}
	support := buildPrepareTurnCharacterMemorySupport(
		sid, []store.CharacterState{state},
		prepareTurnRequestEntityScope{Direct: []string{"Mira"}, Scene: []string{"Mira"}, Known: []string{"Mira"}},
		map[string]any{},
		prepareTurnCharacterMemoryTestReadContext(sid, active, nil),
	)
	if got := intFromAny(support["eligible_count"], 0); got != 70 {
		t.Fatalf("eligible_count = %d, want every one of 70 candidates; support=%#v", got, support)
	}
	if support["count_cap"] != nil {
		t.Fatalf("character memory must not expose a candidate count cap: %#v", support["count_cap"])
	}
	if got := len(prepareTurnCharacterMemoryLines(support, "character_objective")); got != 70 {
		t.Fatalf("objective candidate lines = %d, want 70", got)
	}
}

func TestPrepareTurnCharacterMemoryDoesNotTruncateDescriptiveFieldsBeforeClassBudget(t *testing.T) {
	const sid = "character-memory-long-description"
	description := "careful " + strings.Repeat("context-sensitive-description ", 24) + "complete"
	state := store.CharacterState{
		ChatSessionID: sid, CharacterName: "Mira",
		PersonalityJSON: mustCompactJSON(map[string]any{
			"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"stable": map[string]any{"observations": []any{map[string]any{
				"profile_section": "stable", "trait_domain": "values", "trait_key": "careful_consideration",
				"supported_expression": description, "observation_kind": "explicit_statement",
				"source_ref": prepareTurnCharacterMemoryTestSource(sid, "long-revision", "long-unit", "public"),
			}}},
			"current": map[string]any{"observations": []any{}}, "dynamic": map[string]any{"observations": []any{}},
			"relationship_specific": map[string]any{"observations": []any{}}, "counterevidence": []any{},
		}),
	}
	support := buildPrepareTurnCharacterMemorySupport(
		sid, []store.CharacterState{state}, prepareTurnRequestEntityScope{Direct: []string{"Mira"}}, nil,
		prepareTurnCharacterMemoryTestReadContext(sid, map[string]bool{"long-revision": true}, nil),
	)
	lines := prepareTurnCharacterMemoryLines(support, "character_objective")
	if len(lines) != 1 || !strings.Contains(lines[0], compactPrepareTurnLine(description, 0)) || strings.Contains(lines[0], "...") {
		t.Fatalf("descriptive field was truncated before final class packing: %#v", lines)
	}
}

func TestPrepareTurnLegacyCharacterProjectionDoesNotTruncateBeforeClassBudget(t *testing.T) {
	status := strings.Repeat("careful-status-context ", 24) + "status-tail-marker"
	voice := strings.Repeat("gentle-voice-context ", 24) + "voice-tail-marker"
	relationship := strings.Repeat("directional-trust-context ", 24) + "relationship-tail-marker"
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{
		StateType: "scene", TurnIndex: 9, Content: `{"present_entities":["Mira","Juno"]}`,
	}})
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil,
		[]store.CharacterState{{
			CharacterName:     "Mira",
			StatusJSON:        mustCompactJSON(map[string]any{"state": status}),
			SpeechStyleJSON:   mustCompactJSON(map[string]any{"principle": voice}),
			RelationshipsJSON: mustCompactJSON(map[string]any{"Juno": map[string]any{"description": relationship}}),
			TurnIndex:         9,
		}},
		nil, nil, nil, nil, nil, nil,
		5, 20000, "Mira speaks with Juno.", "default", nil, nil, nil, perspective,
	)
	for marker, text := range map[string]string{
		"status-tail-marker":       assembly.CharacterObjectiveText,
		"voice-tail-marker":        assembly.CharacterObjectiveText,
		"relationship-tail-marker": assembly.CharacterRelationshipText,
	} {
		if !strings.Contains(text, marker) || strings.Contains(text, "...") {
			t.Fatalf("legacy character projection was truncated before final class budget; marker=%s text=%q", marker, text)
		}
	}
}

func TestPrepareTurnCharacterMemoryRejectsWrongScopePOVSessionAndSource(t *testing.T) {
	const sid = "character-memory-session"
	baseEntry := func(source map[string]any) map[string]any {
		return map[string]any{
			"profile_section": "stable", "trait_domain": "values", "trait_key": "protective",
			"supported_expression": "protective of companions", "observation_kind": "explicit_statement", "source_ref": source,
		}
	}
	state := func(session string, entry map[string]any) store.CharacterState {
		return store.CharacterState{
			ChatSessionID: session, CharacterName: "Mira",
			PersonalityJSON: mustCompactJSON(map[string]any{
				"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
				"stable": map[string]any{"observations": []any{entry}}, "current": map[string]any{"observations": []any{}},
				"dynamic": map[string]any{"observations": []any{}}, "relationship_specific": map[string]any{"observations": []any{}},
				"counterevidence": []any{},
			}),
		}
	}
	validSource := prepareTurnCharacterMemoryTestSource(sid, "active-revision", "unit-1", "public")
	tests := []struct {
		name        string
		state       store.CharacterState
		scope       prepareTurnRequestEntityScope
		perspective map[string]any
		active      map[string]bool
	}{
		{name: "wrong session", state: state("other-session", baseEntry(validSource)), scope: prepareTurnRequestEntityScope{Direct: []string{"Mira"}}, active: map[string]bool{"active-revision": true}},
		{name: "wrong subject scope", state: state(sid, baseEntry(validSource)), scope: prepareTurnRequestEntityScope{Direct: []string{"Noah"}, Scene: []string{"Noah"}}, active: map[string]bool{"active-revision": true}},
		{name: "inactive revision", state: state(sid, baseEntry(validSource)), scope: prepareTurnRequestEntityScope{Direct: []string{"Mira"}}, active: map[string]bool{}},
		{name: "missing stable source ref", state: state(sid, baseEntry(map[string]any{"chat_session_id": sid, "source_revision": "active-revision"})), scope: prepareTurnRequestEntityScope{Direct: []string{"Mira"}}, active: map[string]bool{"active-revision": true}},
		{name: "wrong restricted POV", state: state(sid, baseEntry(prepareTurnCharacterMemoryTestSource(sid, "active-revision", "unit-private", "restricted"))), scope: prepareTurnRequestEntityScope{Direct: []string{"Mira"}}, perspective: map[string]any{"identity_state": "resolved", "current_pov_entity_id": "entity-noah"}, active: map[string]bool{"active-revision": true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			support := buildPrepareTurnCharacterMemorySupport(sid, []store.CharacterState{test.state}, test.scope, test.perspective, prepareTurnCharacterMemoryTestReadContext(sid, test.active, nil))
			if got := intFromAny(support["eligible_count"], 0); got != 0 {
				t.Fatalf("eligible_count = %d, want 0; support=%#v", got, support)
			}
		})
	}
}

func TestPrepareTurnCharacterMemoryRequiresRelationshipCounterpartScope(t *testing.T) {
	const sid = "character-memory-session"
	source := prepareTurnCharacterMemoryTestSource(sid, "active-revision", "unit-relationship-profile", "public")
	state := store.CharacterState{
		ChatSessionID: sid, CharacterName: "Mira",
		PersonalityJSON: mustCompactJSON(map[string]any{
			"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"stable": map[string]any{"observations": []any{}}, "current": map[string]any{"observations": []any{}}, "dynamic": map[string]any{"observations": []any{}},
			"relationship_specific": map[string]any{"observations": []any{map[string]any{
				"profile_section": "relationship_specific", "trait_domain": "interpersonal_style", "trait_key": "gentle_with_noah",
				"supported_expression": "gentle around Noah", "observation_kind": "observed_behavior", "source_ref": source,
				"counterpart": map[string]any{"counterpart_entity_id": "entity-noah", "counterpart_label": "Noah"},
			}}},
			"counterevidence": []any{},
		}),
	}
	active := map[string]bool{"active-revision": true}
	withoutCounterpart := buildPrepareTurnCharacterMemorySupport(sid, []store.CharacterState{state}, prepareTurnRequestEntityScope{Direct: []string{"Mira"}, Scene: []string{"Mira"}}, nil, prepareTurnCharacterMemoryTestReadContext(sid, active, nil))
	if intFromAny(withoutCounterpart["eligible_count"], 0) != 0 {
		t.Fatalf("out-of-scope relationship counterpart was delivered: %#v", withoutCounterpart)
	}
	withCounterpart := buildPrepareTurnCharacterMemorySupport(sid, []store.CharacterState{state}, prepareTurnRequestEntityScope{Direct: []string{"Mira"}, Scene: []string{"Mira", "Noah"}}, nil, prepareTurnCharacterMemoryTestReadContext(sid, active, nil))
	if intFromAny(withCounterpart["eligible_count"], 0) != 1 || len(prepareTurnCharacterMemoryLines(withCounterpart, "subjective_relationship")) != 1 {
		t.Fatalf("in-scope relationship-specific profile was not selected: %#v", withCounterpart)
	}
}

func TestPrepareTurnCharacterMemoryVoiceAndDirectionalRelationshipDelivery(t *testing.T) {
	const sid = "character-memory-session"
	voiceRef := prepareTurnCharacterMemoryTestSource(sid, "voice-revision", "voice-unit", "owner_private")
	voiceRef["context"] = map[string]any{"context_key": "under_pressure", "context_expression": "when pressured"}
	voiceRef["counterpart"] = map[string]any{"counterpart_entity_id": "entity-noah", "counterpart_label": "Noah", "counterpart_expression": "Noah"}
	voiceRef["state_modulation"] = map[string]any{"state_modulation_key": "afraid", "state_modulation_expression": "when afraid"}
	voiceCounter := prepareTurnCharacterMemoryTestSource(sid, "voice-counter-revision", "voice-counter-unit", "public")
	state := store.CharacterState{
		ChatSessionID: sid, CharacterName: "Mira",
		SpeechStyleJSON: mustCompactJSON(map[string]any{
			"contract_version": voiceBehaviorProjectionContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"principles": []any{map[string]any{
				"trait_domain": "directness", "principle_key": "brief_direct_requests",
				"support_refs": []any{voiceRef}, "counterevidence_refs": []any{voiceCounter}, "exception_refs": []any{},
			}},
		}),
	}
	relationship := store.StatusCurrentValue{
		ChatSessionID: sid, StatusKey: relationshipStateStatusKey, OwnerScope: relationshipStateOwnerScope,
		OwnerID: "relationship-owner", WriteState: "current", SourceTurn: 8,
		ValueJSON: mustCompactJSON(map[string]any{
			"version": relationshipStateContractVersion, "source_entity_id": "entity-mira", "source_label": "Mira", "target_entity_id": "entity-noah", "target_label": "Noah", "domain": "trust",
			"current":     map[string]any{"observation": "trusts Noah with warnings", "support_kind": "explicit_observed_state", "visibility": "public"},
			"reciprocity": map[string]any{"state": "not_inferred"},
			"validity":    map[string]any{"source_turn_start": 8, "source_turn_end": 8, "source_revision": "relationship-revision", "lifecycle_state": "active"},
			"source":      map[string]any{"source_contract": completeTurnSourceAcceptanceContract, "source_revision": "relationship-revision", "precise_memory_unit_id": "relationship-unit", "content_hash": "relationship-content-hash"},
		}),
	}
	active := map[string]bool{"voice-revision": true, "voice-counter-revision": true, "relationship-revision": true}
	support := buildPrepareTurnCharacterMemorySupport(
		sid, []store.CharacterState{state},
		prepareTurnRequestEntityScope{Direct: []string{"Mira"}, Scene: []string{"Mira", "Noah"}},
		map[string]any{"identity_state": "resolved", "current_pov_entity_id": "entity-mira"},
		prepareTurnCharacterMemoryTestReadContext(sid, active, []store.StatusCurrentValue{relationship}),
	)
	if got := intFromAny(support["eligible_count"], 0); got != 2 {
		t.Fatalf("eligible_count = %d, want voice + directional relationship; support=%#v", got, support)
	}
	objective := strings.Join(prepareTurnCharacterMemoryLines(support, "character_objective"), "\n")
	if !strings.Contains(objective, "principle=brief_direct_requests") || !strings.Contains(objective, "contexts=under_pressure") ||
		!strings.Contains(objective, "counterparts=Noah") || !strings.Contains(objective, "state_modulations=afraid") ||
		!strings.Contains(objective, "counterevidence=present") || !strings.Contains(objective, "subtext_only_do_not_reveal_private_fact") {
		t.Fatalf("voice projection lost descriptive conditions or privacy guard: %s", objective)
	}
	if strings.Contains(objective, "example dialogue") || strings.Contains(objective, "utterance") {
		t.Fatalf("raw/sample dialogue leaked into voice projection: %s", objective)
	}
	if strings.Contains(objective, "character-memory:") || strings.Contains(objective, "voice-unit") {
		t.Fatalf("opaque backend identity leaked into model-facing character memory: %s", objective)
	}
	structuredRelationshipFound := false
	for _, raw := range outputFidelityLineageSlice(support["eligible_items"]) {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["class"]) != "subjective_relationship" {
			continue
		}
		structuredRelationshipFound = true
		if extractionStringFromAny(item["subject_entity_id"]) != "entity-mira" || extractionStringFromAny(item["counterpart_entity_id"]) != "entity-noah" {
			t.Fatalf("typed relationship lost its directional entity coordinates: %#v", item)
		}
	}
	if !structuredRelationshipFound {
		t.Fatal("typed relationship support item missing")
	}
	relationshipText := strings.Join(prepareTurnCharacterMemoryLines(support, "subjective_relationship"), "\n")
	if !strings.Contains(relationshipText, "Mira -> Noah") || !strings.Contains(relationshipText, "reciprocity=not_inferred") || strings.Contains(relationshipText, "Noah -> Mira") {
		t.Fatalf("directional relationship was reversed or reciprocity inferred: %s", relationshipText)
	}
}

func TestPrepareTurnEntityScopeUsesReviewedIdentityAliasWithoutNameHeuristics(t *testing.T) {
	aliases := map[string]any{"지유": "현지유", "현지유": "현지유"}
	scope := buildPrepareTurnRequestEntityScopeWithAliases(
		"지유가 한얼의 반응을 살폈다.", "", []string{"현지유"}, aliases,
	)
	if len(scope.Direct) != 1 || scope.Direct[0] != "현지유" {
		t.Fatalf("reviewed alias did not resolve to canonical scope: %#v", scope)
	}
	withoutLink := buildPrepareTurnRequestEntityScopeWithAliases(
		"지유가 한얼의 반응을 살폈다.", "", []string{"현지유"}, nil,
	)
	if len(withoutLink.Direct) != 0 {
		t.Fatalf("short/full name was inferred without a reviewed identity link: %#v", withoutLink)
	}
}

func TestPrepareTurnCharacterMemoryDeliversGroundedItemsWithoutAuxiliaryDescriptions(t *testing.T) {
	const sid = "character-memory-optional-fields"
	profileRef := prepareTurnCharacterMemoryTestSource(sid, "profile-revision", "profile-unit", "public")
	voiceRef := prepareTurnCharacterMemoryTestSource(sid, "voice-revision", "voice-unit", "public")
	state := store.CharacterState{
		ChatSessionID: sid, CharacterName: "Mira",
		PersonalityJSON: mustCompactJSON(map[string]any{
			"contract_version": characterProfileContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"stable": map[string]any{"observations": []any{}}, "current": map[string]any{"observations": []any{}},
			"dynamic": map[string]any{"observations": []any{map[string]any{
				"profile_section": "dynamic", "trait_key": "invites_informality", "source_ref": profileRef,
			}}},
			"relationship_specific": map[string]any{"observations": []any{}}, "counterevidence": []any{},
		}),
		SpeechStyleJSON: mustCompactJSON(map[string]any{
			"contract_version": voiceBehaviorProjectionContractVersion, "subject_entity_id": "entity-mira", "subject_label": "Mira",
			"principles": []any{map[string]any{"principle_key": "speaks_warmly", "support_refs": []any{voiceRef}}},
		}),
	}
	relationship := store.StatusCurrentValue{
		ChatSessionID: sid, StatusKey: relationshipStateStatusKey, OwnerScope: relationshipStateOwnerScope,
		OwnerID: "relationship-owner", WriteState: "current", SourceTurn: 4,
		ValueJSON: mustCompactJSON(map[string]any{
			"version": relationshipStateContractVersion, "source_entity_id": "entity-mira", "source_label": "Mira",
			"target_entity_id": "entity-noah", "target_label": "Noah", "domain": "",
			"current":  map[string]any{"observation": "now welcomes Noah's informal address", "visibility": "public"},
			"validity": map[string]any{"source_turn_start": 4, "source_turn_end": 4, "source_revision": "relationship-revision"},
			"source":   map[string]any{"source_contract": completeTurnSourceAcceptanceContract, "source_revision": "relationship-revision", "precise_memory_unit_id": "relationship-unit", "content_hash": "relationship-hash"},
		}),
	}
	support := buildPrepareTurnCharacterMemorySupport(
		sid, []store.CharacterState{state},
		prepareTurnRequestEntityScope{Direct: []string{"Mira"}, Scene: []string{"Mira", "Noah"}}, nil,
		prepareTurnCharacterMemoryTestReadContext(sid, map[string]bool{
			"profile-revision": true, "voice-revision": true, "relationship-revision": true,
		}, []store.StatusCurrentValue{relationship}),
	)
	if got := intFromAny(support["eligible_count"], 0); got != 3 {
		t.Fatalf("eligible_count=%d want profile + voice + relationship; support=%#v", got, support)
	}
	text := strings.Join(append(
		prepareTurnCharacterMemoryLines(support, "character_objective"),
		prepareTurnCharacterMemoryLines(support, "subjective_relationship")...,
	), "\n")
	for _, want := range []string{"trait_key=invites_informality", "principle=speaks_warmly", "current=now welcomes Noah's informal address"} {
		if !strings.Contains(text, want) {
			t.Fatalf("optional description omission dropped %q: %s", want, text)
		}
	}
}

func TestPrepareTurnCharacterMemoryFailsClosedWithoutActiveRevisionList(t *testing.T) {
	support := buildPrepareTurnCharacterMemorySupport(
		"session", nil, prepareTurnRequestEntityScope{}, nil,
		map[string]any{"active_source_status": "unavailable", "failure_reason": "active_source_revision_lister_unavailable"},
	)
	if extractionStringFromAny(support["status"]) != "fail_closed" || intFromAny(support["eligible_count"], 0) != 0 {
		t.Fatalf("active source unavailability must fail closed only for character delivery: %#v", support)
	}
}

func TestPrepareTurnCharacterMemoryPublisherReceivesDeliveredOnly(t *testing.T) {
	eligible := []map[string]any{
		{"source_ref": "character-memory:delivered", "class": "character_objective", "kind": "voice_behavior", "text": "- Mira voice principle; principle=brief_direct_requests", "delivered": false, "source_metadata": map[string]any{"private_original": "must-not-copy"}},
		{"source_ref": "character-memory:deferred", "class": "character_objective", "kind": "character_profile", "text": "- Mira profile support; trait_key=patient", "delivered": false},
	}
	support := map[string]any{
		"contract_version": prepareTurnCharacterMemoryContractVersion, "status": "eligible", "eligible_items": eligible,
		"eligible_count": 2, "delivered_items": []map[string]any{}, "raw_private_originals_included": false,
	}
	plan := map[string]any{"classes": []map[string]any{
		{"key": "character_objective", "text": "[Character Objective States]\n- Mira voice principle; principle=brief_direct_requests"},
	}}
	support = finalizePrepareTurnCharacterMemorySupport(support, plan)
	rules := buildResponseExecutionSourceRulesWithMemory(dto.PrepareTurnCurrentInputDecisionV1{}, dto.PrepareTurnHostContextReferenceEvidenceV1{}, "session", nil, "", support, nil)
	refs := stringSliceFromAny(mapFromAny(rules["source_refs"])["character_memory"])
	if len(refs) != 1 || refs[0] != "character-memory:delivered" {
		t.Fatalf("response source refs included an undelivered candidate: %#v", refs)
	}
	packet := buildSupervisorSupportPacket("session", "", rules, nil, "", support, nil)
	delivered := outputFidelityLineageSlice(packet["delivered_character_memory"])
	if len(delivered) != 1 || extractionStringFromAny(mapFromAny(delivered[0])["source_ref"]) != "character-memory:delivered" {
		t.Fatalf("publisher packet did not receive exactly the delivered projection: %#v", packet)
	}
	serialized := mustCompactJSON(packet)
	if strings.Contains(serialized, "character-memory:deferred") || strings.Contains(serialized, "must-not-copy") {
		t.Fatalf("publisher packet leaked deferred metadata or private originals: %s", serialized)
	}
}

func TestPrepareTurnCharacterMemoryFinalizerConsumesDuplicateTextMultiplicity(t *testing.T) {
	const text = "- Mira profile support; trait_key=patient"
	support := map[string]any{
		"contract_version": prepareTurnCharacterMemoryContractVersion,
		"status":           "eligible",
		"eligible_items": []map[string]any{
			{"source_ref": "character-memory:first", "class": "character_objective", "kind": "character_profile", "text": text, "delivered": false},
			{"source_ref": "character-memory:second", "class": "character_objective", "kind": "character_profile", "text": text, "delivered": false},
		},
		"eligible_count": 2, "delivered_items": []map[string]any{},
	}
	plan := map[string]any{"classes": []map[string]any{{"key": "character_objective", "text": "[Character Objective States]\n" + text}}}
	support = finalizePrepareTurnCharacterMemorySupport(support, plan)
	if intFromAny(support["delivered_count"], 0) != 1 || intFromAny(support["deferred_by_budget_count"], 0) != 1 {
		t.Fatalf("duplicate display text multiplicity was not consumed once: %#v", support)
	}
	delivered := outputFidelityLineageSlice(support["delivered_items"])
	if len(delivered) != 1 || extractionStringFromAny(mapFromAny(delivered[0])["source_ref"]) != "character-memory:first" {
		t.Fatalf("duplicate display text delivered lineage is not one-to-one: %#v", support)
	}
}

func prepareTurnCharacterMemoryTestSource(sid, revision, unitID, visibility string) map[string]any {
	return map[string]any{
		"chat_session_id": sid, "source_contract": completeTurnSourceAcceptanceContract, "source_revision": revision,
		"precise_memory_unit_id": unitID, "source_turn_start": 4, "source_turn_end": 4,
		"content_hash": "content-hash-" + unitID, "root_evidence_id": 11, "direct_evidence_ids": []int64{12},
		"evidence_hash": "evidence-hash-" + unitID, "visibility": visibility,
	}
}

func prepareTurnCharacterMemoryTestReadContext(sid string, active map[string]bool, values []store.StatusCurrentValue) map[string]any {
	return map[string]any{
		"contract_version": prepareTurnCharacterMemoryContractVersion, "chat_session_id": sid,
		"active_source_status": "ready", "active_source_revisions": active,
		"relationship_read_status": "ready", "relationship_values": values,
	}
}

type prepareTurnCharacterMemoryReadStore struct {
	store.Store
	sources       []store.MemorySourceRevision
	relationships []store.StatusCurrentValue
}

func (s *prepareTurnCharacterMemoryReadStore) ListActiveSourceRevisions(context.Context, string, int, int) ([]store.MemorySourceRevision, error) {
	return s.sources, nil
}

func (s *prepareTurnCharacterMemoryReadStore) ApplyReversibleStatusTransition(context.Context, store.ReversibleStatusTransition) (store.ReversibleStatusTransitionResult, error) {
	return store.ReversibleStatusTransitionResult{}, store.ErrNotEnabled
}

func (s *prepareTurnCharacterMemoryReadStore) GetReversibleStatusEventBySourceUnit(context.Context, string, string, string) (store.StatusChangeEvent, error) {
	return store.StatusChangeEvent{}, store.ErrNotFound
}

func (s *prepareTurnCharacterMemoryReadStore) ListReversibleStatusCurrentValues(context.Context, string, string, []string) ([]store.StatusCurrentValue, error) {
	return s.relationships, nil
}

func (s *prepareTurnCharacterMemoryReadStore) ListLatestReversibleCurrentProjectionEvents(context.Context, string, []string) ([]store.StatusChangeEvent, error) {
	return nil, nil
}

func TestPrepareTurnCharacterMemoryReadContextReadsAllActiveAndRelationshipCurrent(t *testing.T) {
	const sid = "read-session"
	reader := &prepareTurnCharacterMemoryReadStore{
		sources: []store.MemorySourceRevision{
			{ContractVersion: store.MemorySourceRevisionContract, ChatSessionID: sid, SourceRevision: "active-a", LifecycleState: "active"},
			{ContractVersion: store.MemorySourceRevisionContract, ChatSessionID: sid, SourceRevision: "active-b", LifecycleState: "active"},
			{ContractVersion: store.MemorySourceRevisionContract, ChatSessionID: sid, SourceRevision: "inactive", LifecycleState: "superseded"},
		},
		relationships: []store.StatusCurrentValue{{ChatSessionID: sid}, {ChatSessionID: sid}},
	}
	read := buildPrepareTurnCharacterMemoryReadContext(context.Background(), reader, sid)
	active := read["active_source_revisions"].(map[string]bool)
	if extractionStringFromAny(read["active_source_status"]) != "ready" || len(active) != 2 || !active["active-a"] || !active["active-b"] {
		t.Fatalf("active source read did not preserve the complete active set: %#v", read)
	}
	if got := len(read["relationship_values"].([]store.StatusCurrentValue)); got != 2 {
		t.Fatalf("relationship current read count = %d, want 2", got)
	}
}
