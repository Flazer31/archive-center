package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test39CDCharacterProfileAndVoiceCompileSeparateConditionalEvidence(t *testing.T) {
	fake := newCharacterProjectionRecordingStore(nil)
	srv := &Server{Store: fake}
	units := []*store.PreciseMemoryUnit{
		characterProfileTestUnit("session-cd", "revision-1", 1, "profile-stable", 701, "entity-mira", "Mira", "stable", "values", "values_honesty", "values honesty", "explicit_statement", "", "", "", "", "", "Mira says she values honesty.", "owner_private"),
		characterProfileTestUnit("session-cd", "revision-2", 2, "profile-current", 702, "entity-mira", "Mira", "current", "current_emotion", "anger", "is angry", "current_state", "", "", "", "", "", "Mira is angry right now.", "owner_private"),
		habitEvidenceTestUnit("session-cd", "revision-3", 3, "habit-dynamic", 703, "entity-mira", "Mira", "checks_exits", "checks the exits", "occurrence", "on_arrival", "on arrival", "", "", "", "Mira checks the exits on arrival.", "owner_private"),
		characterProfileTestUnit("session-cd", "revision-4", 4, "profile-counter", 704, "entity-mira", "Mira", "stable", "values", "values_honesty", "lies to protect Rook", "counterexample", "protecting_rook", "to protect Rook", "entity-rook", "Rook", "Rook", "Mira lies to protect Rook.", "owner_private"),
		characterProfileTestUnit("session-cd", "revision-5", 5, "profile-relationship", 705, "entity-mira", "Mira", "relationship_specific", "communication_preference", "softens_with_rook", "speaks gently to Rook", "contextual_pattern", "with_rook", "to Rook", "entity-rook", "Rook", "Rook", "Mira speaks gently to Rook.", "owner_private"),
		voiceProjectionTestUnit("session-cd", "revision-6", 6, "voice-support", 706, "entity-mira", "Mira", "directness", "brief_imperatives", "utterance", `"Enough."`, "under_threat", "under threat", "entity-rook", "Rook", "Rook", "tense", "tense", `Mira tells Rook, "Enough." while tense under threat.`, "owner_private"),
		voiceProjectionTestUnit("session-cd", "revision-7", 7, "voice-counter", 707, "entity-mira", "Mira", "directness", "brief_imperatives", "counterexample", `"Could you wait?"`, "with_children", "with children", "", "", "", "calm", "calm", `Mira asks, "Could you wait?" with children when calm.`, "owner_private"),
	}
	result := artifactSaveResult{}
	srv.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-cd", units, time.Unix(700, 0), &result)

	if result.Errors != 0 || result.CharacterProfiles != 1 || result.VoiceBehaviorProjections != 1 || result.CharacterStates != 1 {
		t.Fatalf("C/D projection result=%+v", result)
	}
	state := fake.mustState(t, "session-cd", "Mira")
	profile := characterProjectionJSONMap(t, state.PersonalityJSON)
	if extractionStringFromAny(profile["contract_version"]) != characterProfileContractVersion ||
		len(sliceFromAny(mapFromAny(profile["stable"])["observations"])) != 1 ||
		len(sliceFromAny(mapFromAny(profile["current"])["observations"])) != 1 ||
		len(sliceFromAny(mapFromAny(profile["dynamic"])["observations"])) != 1 ||
		len(sliceFromAny(mapFromAny(profile["relationship_specific"])["observations"])) != 1 ||
		len(sliceFromAny(profile["counterevidence"])) != 1 {
		t.Fatalf("profile sections were flattened or lost: %s", state.PersonalityJSON)
	}
	stableText := mustCompactJSON(mapFromAny(profile["stable"]))
	if strings.Contains(stableText, "checks_exits") || boolFromAny(profile["fixed_count_trait_promotion"]) || boolFromAny(profile["cross_domain_inference"]) {
		t.Fatalf("habit occurrence promoted to stable trait: %s", state.PersonalityJSON)
	}
	for _, forbidden := range []string{"confidence", "score"} {
		if habitEvidenceHasKey(profile, forbidden) {
			t.Fatalf("profile contains forbidden %q: %s", forbidden, state.PersonalityJSON)
		}
	}

	voice := characterProjectionJSONMap(t, state.SpeechStyleJSON)
	principles := sliceFromAny(voice["principles"])
	if extractionStringFromAny(voice["contract_version"]) != voiceBehaviorProjectionContractVersion || len(principles) != 1 ||
		boolFromAny(voice["example_dialogue_projection"]) || extractionStringFromAny(voice["example_dialogue_storage"]) != "precise_memory_only" {
		t.Fatalf("voice projection contract is wrong: %s", state.SpeechStyleJSON)
	}
	principle := mapFromAny(principles[0])
	if len(sliceFromAny(principle["support_refs"])) != 1 || len(sliceFromAny(principle["counterevidence_refs"])) != 1 ||
		len(sliceFromAny(principle["contexts"])) != 2 || len(sliceFromAny(principle["counterparts"])) != 1 || len(sliceFromAny(principle["state_modulations"])) != 2 {
		t.Fatalf("voice conditions or counterevidence were lost: %s", state.SpeechStyleJSON)
	}
	if len(mapFromAny(mapFromAny(sliceFromAny(principle["support_refs"])[0])["context"])) == 0 ||
		len(mapFromAny(mapFromAny(sliceFromAny(principle["support_refs"])[0])["counterpart"])) == 0 ||
		len(mapFromAny(mapFromAny(sliceFromAny(principle["support_refs"])[0])["state_modulation"])) == 0 {
		t.Fatalf("voice conditions are not attached to their visibility-bearing source ref: %s", state.SpeechStyleJSON)
	}
	if strings.Contains(state.SpeechStyleJSON, "Enough") || strings.Contains(state.SpeechStyleJSON, "Could you wait") ||
		habitEvidenceHasKey(voice, "utterance_expression") || habitEvidenceHasKey(voice, "evidence_excerpt") {
		t.Fatalf("example dialogue leaked into durable voice projection: %s", state.SpeechStyleJSON)
	}
}

func Test39DVoiceProjectionMergesStructuralVariantsWithoutDomainOnlyCollapse(t *testing.T) {
	fake := newCharacterProjectionRecordingStore(nil)
	srv := &Server{Store: fake}
	units := []*store.PreciseMemoryUnit{
		voiceProjectionTestUnit("session-voice-variants", "revision-1", 1, "voice-primary", 901, "entity-mira", "Mira", "directness", "brief_imperatives", "utterance", `"Enough."`, "", "", "", "", "", "", "", `Mira says, "Enough."`, "owner_private"),
		voiceProjectionTestUnit("session-voice-variants", "revision-1", 1, "voice-same-occurrence", 901, "entity-mira", "Mira", "directness", "concise_commands", "utterance", `"Enough."`, "", "", "", "", "", "", "", `Mira says, "Enough."`, "owner_private"),
		voiceProjectionTestUnit("session-voice-variants", "revision-2", 2, "voice-same-principle", 902, "entity-mira", "Mira", "brevity", "brief_imperatives", "utterance", `"Stop."`, "", "", "", "", "", "", "", `Mira says, "Stop."`, "owner_private"),
		voiceProjectionTestUnit("session-voice-variants", "revision-3", 3, "voice-distinct", 903, "entity-mira", "Mira", "directness", "avoids_metaphors", "utterance", `"State the facts."`, "", "", "", "", "", "", "", `Mira says, "State the facts."`, "owner_private"),
		voiceProjectionTestUnit("session-voice-variants", "revision-4", 4, "voice-reuses-variant", 904, "entity-mira", "Mira", "command_density", "concise_commands", "utterance", `"Move."`, "", "", "", "", "", "", "", `Mira says, "Move."`, "owner_private"),
	}
	result := artifactSaveResult{}
	srv.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-voice-variants", units, time.Unix(901, 0), &result)
	if result.Errors != 0 || result.VoiceBehaviorProjections != 1 {
		t.Fatalf("voice variant projection result=%+v", result)
	}
	state := fake.mustState(t, "session-voice-variants", "Mira")
	voice := characterProjectionJSONMap(t, state.SpeechStyleJSON)
	principles := sliceFromAny(voice["principles"])
	if len(principles) != 2 {
		t.Fatalf("voice variants were over- or under-merged: %s", state.SpeechStyleJSON)
	}
	var merged, distinct map[string]any
	for _, raw := range principles {
		principle := mapFromAny(raw)
		switch extractionStringFromAny(principle["principle_key"]) {
		case "brief_imperatives":
			merged = principle
		case "avoids_metaphors":
			distinct = principle
		}
	}
	if len(merged) == 0 || len(distinct) == 0 {
		t.Fatalf("voice principle representatives are missing: %s", state.SpeechStyleJSON)
	}
	if len(sliceFromAny(merged["support_refs"])) != 4 ||
		!stringSliceContains(stringsFromAny(merged["principle_key_variants"]), "brief_imperatives") ||
		!stringSliceContains(stringsFromAny(merged["principle_key_variants"]), "concise_commands") ||
		!stringSliceContains(stringsFromAny(merged["trait_domain_variants"]), "directness") ||
		!stringSliceContains(stringsFromAny(merged["trait_domain_variants"]), "brevity") ||
		!stringSliceContains(stringsFromAny(merged["trait_domain_variants"]), "command_density") {
		t.Fatalf("merged voice variants or source refs were lost: %s", state.SpeechStyleJSON)
	}
	if len(sliceFromAny(distinct["support_refs"])) != 1 {
		t.Fatalf("different occurrence with only a shared domain was merged: %s", state.SpeechStyleJSON)
	}
}

func Test39TypedVoiceManualPatchPreservesCompiledPrinciples(t *testing.T) {
	current := mustCompactJSON(map[string]any{
		"contract_version":  voiceBehaviorProjectionContractVersion,
		"subject_entity_id": "entity-jiyu",
		"subject_label":     "Hyun Jiyu",
		"principles":        []any{map[string]any{"trait_domain": "warmth", "principle_key": "gentle_teasing"}},
	})
	updates := preserveTypedVoiceProjectionManualOverrides(current, map[string]any{
		"speech_style_json": `{"default_tone":"warm","speech_notes":"manual note"}`,
	})
	payload := map[string]any{}
	if json.Unmarshal([]byte(extractionStringFromAny(updates["speech_style_json"])), &payload) != nil {
		t.Fatalf("typed voice patch produced invalid JSON: %#v", updates)
	}
	if extractionStringFromAny(payload["contract_version"]) != voiceBehaviorProjectionContractVersion || len(sliceFromAny(payload["principles"])) != 1 {
		t.Fatalf("manual patch replaced compiled voice projection: %#v", payload)
	}
	manual := mapFromAny(payload["manual_overrides"])
	if extractionStringFromAny(manual["default_tone"]) != "warm" || extractionStringFromAny(manual["speech_notes"]) != "manual note" {
		t.Fatalf("manual overrides missing: %#v", payload)
	}
}

func Test39CProfileAccumulatesDurableEvidenceAndReplacesOnlyCurrentSlot(t *testing.T) {
	fake := newCharacterProjectionRecordingStore(nil)
	srv := &Server{Store: fake}
	units := []*store.PreciseMemoryUnit{
		characterProfileTestUnit("session-current", "revision-1", 1, "stable-1", 901, "entity-mira", "Mira", "stable", "values", "core_value", "values honesty", "explicit_statement", "", "", "", "", "", "Mira says she values honesty.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-2", 2, "stable-2", 902, "entity-mira", "Mira", "stable", "values", "core_value", "values compassion", "explicit_statement", "", "", "", "", "", "Mira says she values compassion.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-3", 3, "dynamic-1", 903, "entity-mira", "Mira", "dynamic", "contextual_response", "pressure_response", "pauses under pressure", "contextual_pattern", "under_pressure", "under pressure", "", "", "", "Mira pauses under pressure.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-4", 4, "dynamic-2", 904, "entity-mira", "Mira", "dynamic", "contextual_response", "pressure_response", "asks for time under pressure", "contextual_pattern", "under_pressure", "under pressure", "", "", "", "Mira asks for time under pressure.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-5", 5, "current-1", 905, "entity-mira", "Mira", "current", "current_emotion", "mood", "is angry", "current_state", "", "", "", "", "", "Mira is angry now.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-6", 6, "current-2", 906, "entity-mira", "Mira", "current", "current_emotion", "mood", "is calm", "current_state", "", "", "", "", "", "Mira is calm now.", "owner_private"),
		characterProfileTestUnit("session-current", "revision-7", 7, "stable-counter", 907, "entity-mira", "Mira", "stable", "values", "core_value", "rejects compassion", "counterexample", "", "", "", "", "", "Mira rejects compassion here.", "owner_private"),
	}
	result := artifactSaveResult{}
	srv.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-current", units, time.Unix(907, 0), &result)
	if result.Errors != 0 || result.CharacterProfiles != 1 {
		t.Fatalf("profile compilation failed: %+v", result)
	}
	profile := characterProjectionJSONMap(t, fake.mustState(t, "session-current", "Mira").PersonalityJSON)
	stable := sliceFromAny(mapFromAny(profile["stable"])["observations"])
	dynamic := sliceFromAny(mapFromAny(profile["dynamic"])["observations"])
	current := sliceFromAny(mapFromAny(profile["current"])["observations"])
	counter := sliceFromAny(profile["counterevidence"])
	if len(stable) != 2 || len(dynamic) != 2 || len(current) != 1 || len(counter) != 1 {
		t.Fatalf("stable/dynamic accumulation or current replacement failed: %#v", profile)
	}
	if extractionStringFromAny(mapFromAny(current[0])["supported_expression"]) != "is calm" {
		t.Fatalf("current slot did not keep latest evidence: %#v", current)
	}
	if extractionStringFromAny(mapFromAny(counter[0])["profile_section"]) != "stable" {
		t.Fatalf("counterevidence lost its profile section: %#v", counter)
	}
	ref := mapFromAny(mapFromAny(stable[0])["source_ref"])
	if extractionStringFromAny(ref["chat_session_id"]) != "session-current" ||
		extractionStringFromAny(ref["source_contract"]) != completeTurnSourceAcceptanceContract ||
		extractionStringFromAny(ref["source_revision"]) == "" ||
		extractionStringFromAny(ref["precise_memory_unit_id"]) == "" ||
		intFromAny(ref["source_turn_end"], 0) == 0 ||
		intFromAny(ref["root_evidence_id"], 0) == 0 ||
		len(sliceFromAny(ref["direct_evidence_ids"])) == 0 {
		t.Fatalf("profile source ref is incomplete: %#v", ref)
	}
}

func Test39DArchivesExampleDialoguePrincipleWithoutExactWordingGate(t *testing.T) {
	evidence := `Mira says, "Enough."`
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}}},
		"speaker_attributions": []any{map[string]any{
			"speaker_name": "Mira", "attribution_kind": "dialogue", "attribution_state": "linked", "evidence_excerpt": evidence,
		}},
		"voice_observations": []any{
			map[string]any{"subject_entity": "Mira", "subject_entity_expression": "Mira", "trait_domain": "directness", "principle_key": "enough", "observation_kind": "utterance", "utterance_expression": `"Enough."`, "evidence_excerpt": evidence},
			map[string]any{"subject_entity": "Mira", "subject_entity_expression": "Mira", "trait_domain": "directness", "principle_key": "brief_imperatives", "observation_kind": "utterance", "utterance_expression": `"Enough."`, "evidence_excerpt": evidence},
		},
	}
	admitted, trace := admitCriticInteractionLanes(raw, "", evidence)
	items := sliceFromAny(admitted["voice_observations"])
	if len(items) != 2 {
		t.Fatalf("voice candidates were not archived broadly: admitted=%#v trace=%#v", admitted, trace)
	}
	if extractionStringFromAny(mapFromAny(items[0])["admission_state"]) != "committed" ||
		extractionStringFromAny(mapFromAny(items[1])["admission_state"]) != "committed" {
		t.Fatalf("voice candidates were rejected by wording identity: %#v", items)
	}

	fake := newCharacterProjectionRecordingStore(nil)
	unit := voiceProjectionTestUnit("session-voice-replay", "revision-1", 1, "voice-replay", 908, "entity-mira", "Mira", "directness", "enough", "utterance", `"Enough."`, "", "", "", "", "", "", "", evidence, "owner_private")
	result := artifactSaveResult{}
	(&Server{Store: fake}).saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-voice-replay", []*store.PreciseMemoryUnit{unit}, time.Unix(908, 0), &result)
	if result.VoiceBehaviorProjections != 1 || len(fake.saved) != 1 {
		t.Fatalf("understandable voice principle was erased because it matched the utterance: %+v", result)
	}
}

func Test39CDProjectionReplayIsIdempotentAndManualFieldsArePreserved(t *testing.T) {
	initial := []store.CharacterState{{
		ChatSessionID: "session-manual", CharacterName: "Mira",
		PersonalityJSON: `{}`, SpeechStyleJSON: `{"tone":"dry","origin":"manual"}`,
		StatusJSON: `{"mood":"watchful"}`, RelationshipsJSON: `{"Rook":{"trust":"mixed"}}`, TurnIndex: 2,
	}}
	fake := newCharacterProjectionRecordingStore(initial)
	srv := &Server{Store: fake}
	profileUnit := characterProfileTestUnit("session-manual", "revision-3", 3, "profile", 801, "entity-mira", "Mira", "dynamic", "contextual_response", "calms_under_pressure", "calms under pressure", "contextual_pattern", "under_pressure", "under pressure", "", "", "", "Mira calms under pressure.", "owner_private")
	voiceUnit := voiceProjectionTestUnit("session-manual", "revision-3", 3, "voice", 802, "entity-mira", "Mira", "pacing", "short_replies", "utterance", `"Go."`, "", "", "", "", "", "", "", `Mira says, "Go."`, "owner_private")
	first := artifactSaveResult{}
	srv.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-manual", []*store.PreciseMemoryUnit{profileUnit, voiceUnit}, time.Unix(801, 0), &first)
	state := fake.mustState(t, "session-manual", "Mira")
	if first.CharacterProfiles != 1 || first.VoiceBehaviorProjections != 0 || state.SpeechStyleJSON != initial[0].SpeechStyleJSON ||
		state.StatusJSON != initial[0].StatusJSON || state.RelationshipsJSON != initial[0].RelationshipsJSON {
		t.Fatalf("manual speech or unrelated state was overwritten: result=%+v state=%+v", first, state)
	}
	second := artifactSaveResult{}
	srv.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-manual", []*store.PreciseMemoryUnit{profileUnit}, time.Unix(802, 0), &second)
	if second.CharacterProfiles != 0 || second.CharacterStates != 0 || len(fake.saved) != 1 {
		t.Fatalf("exact replay rewrote profile: result=%+v saves=%d", second, len(fake.saved))
	}

	voiceOnlyFake := newCharacterProjectionRecordingStore([]store.CharacterState{{
		ChatSessionID: "session-manual-personality", CharacterName: "Mira",
		PersonalityJSON: `{"values":"manual"}`, SpeechStyleJSON: `{}`,
	}})
	voiceOnlyServer := &Server{Store: voiceOnlyFake}
	voiceOnly := voiceProjectionTestUnit("session-manual-personality", "revision-1", 1, "voice-only", 803, "entity-mira", "Mira", "pacing", "short_replies", "utterance", `"Go."`, "", "", "", "", "", "", "", `Mira says, "Go."`, "owner_private")
	voiceOnlyResult := artifactSaveResult{}
	voiceOnlyServer.saveCharacterProfileAndVoiceProjectionsFromPreciseMemoryUnits(context.Background(), "session-manual-personality", []*store.PreciseMemoryUnit{voiceOnly}, time.Unix(803, 0), &voiceOnlyResult)
	voiceOnlyState := voiceOnlyFake.mustState(t, "session-manual-personality", "Mira")
	if voiceOnlyResult.CharacterProfiles != 0 || voiceOnlyResult.VoiceBehaviorProjections != 1 || voiceOnlyState.PersonalityJSON != `{"values":"manual"}` {
		t.Fatalf("voice projection overwrote manual personality: result=%+v state=%+v", voiceOnlyResult, voiceOnlyState)
	}
}

func Test39CDAdmissionKeepsSourceBoundProfileAndVoiceWithoutRedundantAttribution(t *testing.T) {
	validVoiceEvidence := `Mira tells Rook, "Enough."`
	source := "Mira is angry now. Mira is a guard. " + validVoiceEvidence + ` Rook says, "Maybe."`
	raw := map[string]any{
		"entities": map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rook"}}},
		"speaker_attributions": []any{map[string]any{
			"speaker_name": "Mira", "attribution_kind": "dialogue", "attribution_state": "linked", "evidence_excerpt": validVoiceEvidence,
		}},
		"character_profile_observations": []any{
			map[string]any{"subject_entity": "Mira", "subject_entity_expression": "Mira", "profile_section": "current", "trait_domain": "current_emotion", "trait_key": "anger", "supported_expression": "is angry", "observation_kind": "current_state", "evidence_excerpt": "Mira is angry now."},
			map[string]any{"subject_entity": "Mira", "subject_entity_expression": "Mira", "profile_section": "stable", "trait_domain": "occupation", "trait_key": "guarded", "supported_expression": "is a guard", "observation_kind": "explicit_statement", "evidence_excerpt": "Mira is a guard."},
		},
		"voice_observations": []any{
			map[string]any{"subject_entity": "Mira", "subject_entity_expression": "Mira", "trait_domain": "directness", "principle_key": "brief_imperatives", "observation_kind": "utterance", "utterance_expression": `"Enough."`, "counterpart": "Rook", "counterpart_expression": "Rook", "evidence_excerpt": validVoiceEvidence},
			map[string]any{"subject_entity": "Rook", "subject_entity_expression": "Rook", "trait_domain": "hedging", "principle_key": "hedged", "observation_kind": "utterance", "utterance_expression": `"Maybe."`, "evidence_excerpt": `Rook says, "Maybe."`},
		},
	}
	admitted, trace := admitCriticInteractionLanes(raw, "", source)
	if len(sliceFromAny(admitted["character_profile_observations"])) != 2 || len(sliceFromAny(admitted["voice_observations"])) != 2 {
		t.Fatalf("broad C/D collection lost structurally complete candidates: admitted=%#v trace=%#v", admitted, trace)
	}
	voiceCommitted, voiceReview := 0, 0
	for _, rawVoice := range sliceFromAny(admitted["voice_observations"]) {
		switch stringFromMap(mapFromAny(rawVoice), "admission_state") {
		case "committed":
			voiceCommitted++
		case "review_required":
			voiceReview++
		}
	}
	if voiceCommitted != 2 || voiceReview != 0 {
		t.Fatalf("voice projection states mismatch: committed=%d review=%d voices=%#v", voiceCommitted, voiceReview, admitted["voice_observations"])
	}
	candidates := interactionAdmissionPreciseMemoryCandidates(admitted)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.subtype] = candidate.truthScope == "support_only" && candidate.authorityClass == "support_hypothesis"
	}
	if !seen["character_profile"] || !seen["voice_behavior"] {
		t.Fatalf("typed precise candidates missing support-only authority: %#v", candidates)
	}
	if !memoryAdmissionHasHolderScopedPerspectiveContent(admitted) {
		t.Fatalf("C/D private evidence escaped perspective scope: %#v", admitted)
	}
}

func Test39CDCommonAdmissionPopulatesExistingCharacterStateColumns(t *testing.T) {
	fake := newCharacterProjectionAdmissionStore(nil)
	srv := NewServer(config.Default())
	srv.Store = fake
	voiceEvidence := `Mira tells Rook, "Enough."`
	profileEvidence := "Mira says she values honesty."
	content := profileEvidence + " " + voiceEvidence
	extraction := map[string]any{
		"entities":             map[string]any{"characters": []any{map[string]any{"name": "Mira"}, map[string]any{"name": "Rook"}}},
		"speaker_attributions": []any{map[string]any{"speaker_name": "Mira", "attribution_kind": "dialogue", "attribution_state": "linked", "confidence": 1, "evidence_excerpt": voiceEvidence}},
		"character_profile_observations": []any{map[string]any{
			"contract_version": characterProfileObservationContract, "subject_entity": "Mira", "subject_entity_expression": "Mira", "profile_section": "stable", "trait_domain": "values", "trait_key": "values_honesty", "supported_expression": "values honesty", "observation_kind": "explicit_statement", "admission_state": "committed", "review_state": "source_observed", "visibility": "owner_private", "evidence_excerpt": profileEvidence,
		}},
		"voice_observations": []any{map[string]any{
			"contract_version": voiceObservationContract, "subject_entity": "Mira", "subject_entity_expression": "Mira", "trait_domain": "directness", "principle_key": "brief_imperatives", "observation_kind": "utterance", "utterance_expression": `"Enough."`, "counterpart": "Rook", "counterpart_expression": "Rook", "admission_state": "committed", "review_state": "source_observed", "visibility": "owner_private", "evidence_excerpt": voiceEvidence,
		}},
	}
	result := srv.saveCriticExtractionArtifacts(acceptedPreciseMemoryContext("revision-cd-admission"), "session-cd-admission", 9, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	if result.Errors != 0 || result.PreciseMemoryUnits != 3 || result.CharacterProfiles != 1 || result.VoiceBehaviorProjections != 1 {
		t.Fatalf("common admission did not populate C/D projections: %+v", result)
	}
	state := fake.mustState(t, "session-cd-admission", "Mira")
	if !strings.Contains(state.PersonalityJSON, characterProfileContractVersion) || !strings.Contains(state.SpeechStyleJSON, voiceBehaviorProjectionContractVersion) || strings.Contains(state.SpeechStyleJSON, "Enough") {
		t.Fatalf("existing DB columns were not populated correctly: %+v", state)
	}
}

func Test39CharacterMemoryCommonAdmissionDoesNotRequireRedundantExpressionFields(t *testing.T) {
	fake := newCharacterProjectionAdmissionStore(nil)
	srv := NewServer(config.Default())
	srv.Store = fake
	evidence := `현지유는 강한얼을 향해 웃었다. "편하게 지유라고 불러요."`
	content := "카페의 긴장이 풀렸다. " + evidence
	extraction := map[string]any{
		"evidence_excerpts": []any{evidence},
		"entities": map[string]any{"characters": []any{
			map[string]any{"name": "현지유", "aliases": []any{"지유"}},
			map[string]any{"name": "강한얼"},
		}},
		"relationship_observations": []any{map[string]any{
			"source_entity": "현지유", "target_entity": "강한얼", "domain": "trust",
			"observation": "한얼을 편하게 대하기 시작함", "support_kind": "explicit_observed_state",
			"visibility": "owner_private", "evidence_excerpt": evidence,
		}},
		"character_profile_observations": []any{map[string]any{
			"subject_entity": "현지유", "profile_section": "dynamic", "trait_domain": "social_manner",
			"trait_key":        "invites_informality",
			"observation_kind": "observed_behavior", "visibility": "owner_private", "evidence_excerpt": evidence,
		}},
		"voice_observations": []any{map[string]any{
			"subject_entity": "현지유", "trait_domain": "warmth", "principle_key": "친근하게 호칭을 완화함",
			"observation_kind": "utterance", "utterance_expression": `"편하게 지유라고 불러요."`,
			"visibility": "owner_private", "evidence_excerpt": evidence,
		}},
	}
	extraction, _ = admitCriticInteractionLanes(extraction, "", content)
	result := srv.saveCriticExtractionArtifacts(
		acceptedPreciseMemoryContext("revision-no-redundant-expressions"),
		"session-no-redundant-expressions",
		4,
		extraction,
		content,
		completeTurnEmbeddingConfig{},
		time.Unix(940, 0),
	)
	if result.Errors != 0 || result.RelationCurrentStates != 1 || result.CharacterProfiles != 1 || result.VoiceBehaviorProjections != 1 {
		t.Fatalf("typed character memory did not survive common admission: %+v", result)
	}
	state := fake.mustState(t, "session-no-redundant-expressions", "현지유")
	if !strings.Contains(state.PersonalityJSON, characterProfileContractVersion) ||
		!strings.Contains(state.SpeechStyleJSON, voiceBehaviorProjectionContractVersion) {
		t.Fatalf("profile or voice projection missing: %+v", state)
	}
}

func Test39CDCriticAndProviderContractsExposeTypedLanes(t *testing.T) {
	prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt("session-prompt", 1, `Mira says, "Enough."`, "Rook waits.", nil, nil))
	for _, needle := range []string{"character_profile_observations", "voice_observations", "speaker_attributions is optional and source-bound", "retained story context and the latest turn uses an alias", "Do not use a fixed count to decide that a habit exists", "a first observation is valid contextual evidence", "rather than forcing future dialogue to repeat an example sentence"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("critic prompt missing 3.9-C/D guard %q", needle)
		}
	}
	if _, _, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary": "ok",
		"character_profile_observations": []any{
			map[string]any{"subject_entity": "Mira", "trait_key": "reserved"},
		},
		"voice_observations": []any{
			map[string]any{"subject_entity": "Mira", "principle_key": "concise"},
		},
	}); err != nil {
		t.Fatalf("typed C/D lanes rejected by critic schema: %v", err)
	}
	sanitized, trace, err := validateCriticExtractionSchema(map[string]any{
		"turn_summary":       "kept",
		"voice_observations": []any{"wrong wire value"},
	})
	if err != nil || sanitized["turn_summary"] != "kept" || intFromAny(trace["dropped_item_count"], 0) != 1 {
		t.Fatalf("invalid voice record was not isolated: sanitized=%#v trace=%#v err=%v", sanitized, trace, err)
	}
	schema := proxyCriticTopLevelJSONSchema()
	if schema["additionalProperties"] != true || len(mapFromAny(mapFromAny(schema["properties"])["records"])) != 0 {
		t.Fatalf("provider critic schema is not sparse top-level: %#v", schema)
	}
}

func Test39DTypedVoiceProjectionIsDeferredUntil39E(t *testing.T) {
	perspective := prepareTurnPerspectiveWithNarrativeState(map[string]any{}, nil, []store.ActiveState{{StateType: "scene", Content: `{"present_entities":["Mira"]}`}})
	assembly := buildPrepareTurnInjectionAssembly(nil, nil, nil, nil, nil, nil, []store.CharacterState{{
		ChatSessionID: "session-defer", CharacterName: "Mira",
		SpeechStyleJSON: mustCompactJSON(newVoiceBehaviorProjection("entity-mira", "Mira")),
	}}, nil, nil, nil, nil, nil, nil, 3, 1000, "How does Mira answer?", "default", nil, nil, nil, perspective)
	if strings.Contains(assembly.CharacterText, "speech_style") || strings.Contains(assembly.Text, voiceBehaviorProjectionContractVersion) || intFromAny(assembly.Counts["typed_voice_projection_deferred_to_3_9_e"], 0) != 1 {
		t.Fatalf("typed voice projection bypassed 3.9-E delivery boundary: text=%q counts=%#v", assembly.Text, assembly.Counts)
	}
}

type characterProjectionRecordingStore struct {
	*preciseMemoryRecordingStore
	states map[string]store.CharacterState
	saved  []store.CharacterState
}

func newCharacterProjectionRecordingStore(initial []store.CharacterState) *characterProjectionRecordingStore {
	f := &characterProjectionRecordingStore{preciseMemoryRecordingStore: newPreciseMemoryRecordingStore(), states: map[string]store.CharacterState{}}
	for _, item := range initial {
		f.states[relationshipStateKey(item.ChatSessionID, item.CharacterName)] = item
	}
	return f
}

func (f *characterProjectionRecordingStore) GetCharacterState(_ context.Context, sid, characterName string) (*store.CharacterState, error) {
	item, ok := f.states[relationshipStateKey(sid, characterName)]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := item
	return &cp, nil
}

func (f *characterProjectionRecordingStore) ListCharacterStates(_ context.Context, sid string) ([]store.CharacterState, error) {
	out := []store.CharacterState{}
	for _, item := range f.states {
		if sid == "" || item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterProjectionRecordingStore) SaveCharacterState(_ context.Context, item *store.CharacterState) error {
	cp := *item
	f.states[relationshipStateKey(cp.ChatSessionID, cp.CharacterName)] = cp
	f.saved = append(f.saved, cp)
	return nil
}

func (f *characterProjectionRecordingStore) mustState(t *testing.T, sid, name string) store.CharacterState {
	t.Helper()
	item, ok := f.states[relationshipStateKey(sid, name)]
	if !ok {
		t.Fatalf("missing character state %s/%s: %#v", sid, name, f.states)
	}
	return item
}

type characterProjectionAdmissionStore struct {
	*relationshipMemoryAdmissionStore
	states map[string]store.CharacterState
}

func newCharacterProjectionAdmissionStore(initial []store.CharacterState) *characterProjectionAdmissionStore {
	f := &characterProjectionAdmissionStore{relationshipMemoryAdmissionStore: newRelationshipMemoryAdmissionStore(), states: map[string]store.CharacterState{}}
	for _, item := range initial {
		f.states[relationshipStateKey(item.ChatSessionID, item.CharacterName)] = item
	}
	return f
}

func (f *characterProjectionAdmissionStore) GetCharacterState(_ context.Context, sid, characterName string) (*store.CharacterState, error) {
	item, ok := f.states[relationshipStateKey(sid, characterName)]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := item
	return &cp, nil
}

func (f *characterProjectionAdmissionStore) ListCharacterStates(_ context.Context, sid string) ([]store.CharacterState, error) {
	out := []store.CharacterState{}
	for _, item := range f.states {
		if sid == "" || item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *characterProjectionAdmissionStore) SaveCharacterState(_ context.Context, item *store.CharacterState) error {
	cp := *item
	f.states[relationshipStateKey(cp.ChatSessionID, cp.CharacterName)] = cp
	return nil
}

func (f *characterProjectionAdmissionStore) mustState(t *testing.T, sid, name string) store.CharacterState {
	t.Helper()
	item, ok := f.states[relationshipStateKey(sid, name)]
	if !ok {
		t.Fatalf("missing character state %s/%s: %#v", sid, name, f.states)
	}
	return item
}

func characterProfileTestUnit(sid, revision string, turn int, unitID string, rootEvidenceID int64, subjectID, subject, section, domain, traitKey, supportedExpression, observationKind, contextKey, contextExpression, counterpartID, counterpart, counterpartExpression, evidence, visibility string) *store.PreciseMemoryUnit {
	unit := habitEvidenceTestUnit(sid, revision, turn, unitID, rootEvidenceID, subjectID, subject, traitKey, supportedExpression, "occurrence", contextKey, contextExpression, counterpartID, counterpart, counterpartExpression, evidence, visibility)
	unit.Subtype = "character_profile"
	unit.PayloadJSON = mustCompactJSON(map[string]any{
		"contract_version": characterProfileObservationContract, "subject_entity": subject, "subject_entity_expression": subject,
		"profile_section": section, "trait_domain": domain, "trait_key": traitKey, "supported_expression": supportedExpression,
		"observation_kind": observationKind, "context_key": contextKey, "context_expression": contextExpression,
		"counterpart": counterpart, "counterpart_expression": counterpartExpression, "admission_state": "committed", "review_state": "source_observed", "visibility": visibility,
	})
	return unit
}

func voiceProjectionTestUnit(sid, revision string, turn int, unitID string, rootEvidenceID int64, subjectID, subject, domain, principleKey, observationKind, utteranceExpression, contextKey, contextExpression, counterpartID, counterpart, counterpartExpression, stateKey, stateExpression, evidence, visibility string) *store.PreciseMemoryUnit {
	unit := habitEvidenceTestUnit(sid, revision, turn, unitID, rootEvidenceID, subjectID, subject, principleKey, utteranceExpression, "occurrence", contextKey, contextExpression, counterpartID, counterpart, counterpartExpression, evidence, visibility)
	unit.Subtype = "voice_behavior"
	unit.PayloadJSON = mustCompactJSON(map[string]any{
		"contract_version": voiceObservationContract, "subject_entity": subject, "subject_entity_expression": subject,
		"trait_domain": domain, "principle_key": principleKey, "observation_kind": observationKind, "utterance_expression": utteranceExpression,
		"context_key": contextKey, "context_expression": contextExpression, "counterpart": counterpart, "counterpart_expression": counterpartExpression,
		"state_modulation_key": stateKey, "state_modulation_expression": stateExpression,
		"admission_state": "committed", "review_state": "source_observed", "visibility": visibility,
	})
	return unit
}

func characterProjectionJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	value := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return value
}
