package httpapi

import (
	"strings"
	"testing"
)

func TestPublisherPlanStrengthLatticeAndSingleAdvanceChoice(t *testing.T) {
	raw := publisherPlanTestProposal()
	for _, tc := range []struct {
		strength    string
		status      string
		advanceMode string
		strongOnly  bool
	}{
		{strength: "none", status: "zero"},
		{strength: "weak", status: "ready"},
		{strength: "medium", status: "ready", advanceMode: "hold_allowed"},
		{strength: "strong", status: "ready", advanceMode: "hold_allowed", strongOnly: true},
	} {
		t.Run(tc.strength, func(t *testing.T) {
			result, _ := buildBoundedSupervisorResult(raw, publisherPlanTestPack(tc.strength))
			proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
			plan := mapFromAny(proposal["publisher_plan"])
			if plan["contract_version"] != "publisher_plan.v1" || plan["status"] != tc.status {
				t.Fatalf("%s publisher plan status = %#v", tc.strength, plan)
			}
			if plan["candidate_count_cap"] != nil || boolFromAny(plan["truth_authority"]) || boolFromAny(plan["would_write"]) {
				t.Fatalf("%s publisher plan gained count cap or authority: %#v", tc.strength, plan)
			}
			if got := extractionStringFromAny(plan["advance_mode"]); got != tc.advanceMode {
				t.Fatalf("%s advance_mode = %q, want %q: %#v", tc.strength, got, tc.advanceMode, plan)
			}
			advanceItems := 0
			for _, rawItem := range outputFidelityLineageSlice(plan["guidance_items"]) {
				slot := extractionStringFromAny(mapFromAny(rawItem)["slot"])
				if slot == "may_advance" || slot == "hold_allowed" {
					advanceItems++
					if slot != tc.advanceMode {
						t.Fatalf("%s rendered contradictory advance slot %q: %#v", tc.strength, slot, plan)
					}
				}
			}
			if tc.advanceMode == "" && advanceItems != 0 {
				t.Fatalf("%s rendered progression guidance: %#v", tc.strength, plan)
			}
			if tc.advanceMode != "" && advanceItems != 1 {
				t.Fatalf("%s advance guidance item count = %d, want exactly 1: %#v", tc.strength, advanceItems, plan)
			}
			if tc.strength == "weak" {
				if !stringSliceContains(stringSliceFromAny(plan["character_expression_refs"]), "character-memory:voice") ||
					!stringSliceContains(stringSliceFromAny(plan["relationship_expression_refs"]), "character-memory:relationship") {
					t.Fatalf("weak source-bound portrayal did not retain delivered E categories: %#v", plan)
				}
			}
			encodedGuidance := mustCompactJSON(plan["guidance_items"])
			if tc.strength != "none" && !strings.Contains(encodedGuidance, "Use the delivered continuity as a callback.") {
				t.Fatalf("%s lost the response-scoped continuity callback: %s", tc.strength, encodedGuidance)
			}
			if tc.strength == "medium" || tc.strength == "strong" {
				for _, required := range []string{"Keep the response pacing measured.", "Emphasize the established scene tension."} {
					if !strings.Contains(encodedGuidance, required) {
						t.Fatalf("%s lost medium scene direction %q: %s", tc.strength, required, encodedGuidance)
					}
				}
			}
			if tc.strength == "strong" && !strings.Contains(encodedGuidance, "Keep the supported option reversible.") {
				t.Fatalf("strong lost its reversible scene option: %s", encodedGuidance)
			}
			if tc.strongOnly {
				if extractionStringFromAny(plan["preferred_frontier_ref"]) != "memory:delivered" || len(mapFromAny(plan["ending_edge"])) == 0 ||
					len(stringSliceFromAny(plan["world_guard_refs"])) == 0 || len(stringSliceFromAny(plan["must_not_refs"])) == 0 {
					t.Fatalf("strong source-proven frontier/ending/guards missing: %#v", plan)
				}
				encoded := mustCompactJSON(plan["guidance_items"])
				for _, required := range []string{"boundary of the current response", "never close a thread, arc, session, or work", "current-response preference", "never as a persistent plot lock"} {
					if !strings.Contains(encoded, required) {
						t.Fatalf("strong publisher safety guard %q missing: %s", required, encoded)
					}
				}
			} else if plan["preferred_frontier_ref"] != nil || plan["ending_edge"] != nil {
				t.Fatalf("%s gained strong-only frontier or ending: %#v", tc.strength, plan)
			}
		})
	}
}

func TestPublisherPlanUsesAcceptedRecentContextAndInputBackedPortrayal(t *testing.T) {
	pack := publisherPlanTestPack("weak")
	contract := mapFromAny(pack["response_execution_contract"])
	refs := mapFromAny(contract["source_refs"])
	refs["continuity"] = []string{supervisorAcceptedRecentContextSourceRef}
	refs["all"] = append(stringSliceFromAny(refs["all"]), supervisorAcceptedRecentContextSourceRef)
	pack["support_packet"] = buildSupervisorSupportPacket(
		"session",
		"current request",
		contract,
		nil,
		"[Recent Chat]\n- [assistant] The previous accepted response.",
		nil,
	)
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "Portray the response as a careful direct reaction.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "callback", "text": "Carry forward the previous accepted response.", "source_refs": []any{supervisorAcceptedRecentContextSourceRef}},
			},
		},
	}, pack)
	plan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
	if extractionStringFromAny(plan["status"]) != "ready" ||
		!stringSliceContains(stringSliceFromAny(plan["response_focus_refs"]), "input:latest") ||
		extractionStringFromAny(plan["continuity_anchor_ref"]) != supervisorAcceptedRecentContextSourceRef {
		t.Fatalf("accepted recent context or input-backed portrayal did not compile: %#v", plan)
	}
}

func TestPublisherPlanExcludesWrongAndUndeliveredReferences(t *testing.T) {
	pack := publisherPlanTestPack("strong")
	contractRefs := mapFromAny(mapFromAny(pack["response_execution_contract"])["source_refs"])
	contractRefs["memory"] = append(stringSliceFromAny(contractRefs["memory"]), "character-memory:undelivered")
	contractRefs["all"] = append(stringSliceFromAny(contractRefs["all"]), "character-memory:undelivered")
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "Use delivered voice behavior.", "source_refs": []any{"character-memory:voice"}},
				map[string]any{"kind": "portrayal", "text": "Must not use undelivered behavior.", "source_refs": []any{"character-memory:undelivered"}},
				map[string]any{"kind": "portrayal", "text": "Must not use an unknown ref.", "source_refs": []any{"character-memory:unknown"}},
			},
		},
	}, pack)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	encoded := mustCompactJSON(plan)
	if !strings.Contains(encoded, "character-memory:voice") || strings.Contains(encoded, "character-memory:undelivered") || strings.Contains(encoded, "character-memory:unknown") {
		t.Fatalf("publisher plan did not enforce delivered-only refs: %s", encoded)
	}
}

func TestPublisherPlanAmbiguousMayAdvanceFailsClosedWithoutHold(t *testing.T) {
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "may_advance", "text": "Advance through the negotiation.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "may_advance", "text": "Advance through the unresolved warning instead.", "source_refs": []any{"memory:delivered"}},
			},
		},
	}, publisherPlanTestPack("medium"))
	plan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
	if plan["advance_mode"] != nil {
		t.Fatalf("ambiguous may_advance options selected an arbitrary mode: %#v", plan)
	}
	for _, raw := range outputFidelityLineageSlice(plan["guidance_items"]) {
		if extractionStringFromAny(mapFromAny(raw)["slot"]) == "may_advance" {
			t.Fatalf("ambiguous may_advance guidance escaped fail-closed selection: %#v", plan)
		}
	}
	singleResult, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "may_advance", "text": "Advance only through the delivered negotiation.", "source_refs": []any{"memory:delivered"}},
			},
		},
	}, publisherPlanTestPack("medium"))
	singlePlan := mapFromAny(mapFromAny(mapFromAny(singleResult["directive"])["supervisor_scene_proposal"])["publisher_plan"])
	if extractionStringFromAny(singlePlan["advance_mode"]) != "may_advance" ||
		!strings.Contains(mustCompactJSON(singlePlan["guidance_items"]), "do not create a new fact, relationship change, user action, or closure") {
		t.Fatalf("single supported may_advance lost reversible-expression guard: %#v", singlePlan)
	}
}

func TestPublisherPlanRejectsAdvanceWithoutGoPreapprovedRef(t *testing.T) {
	pack := publisherPlanTestPack("medium")
	refs := mapFromAny(mapFromAny(pack["response_execution_contract"])["source_refs"])
	delete(refs, "may_advance")
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "may_advance", "text": "Advance from an ordinary delivered memory.", "source_refs": []any{"memory:delivered"}},
			},
		},
	}, pack)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	if extractionStringFromAny(proposal["status"]) != "unsupported_rejected" ||
		extractionStringFromAny(plan["status"]) != "zero" ||
		len(outputFidelityLineageSlice(plan["guidance_items"])) != 0 {
		t.Fatalf("ordinary delivered memory authorized may_advance: proposal=%#v plan=%#v", proposal, plan)
	}
}

func TestPublisherPlanPreservesPrivateAndDirectionalRelationshipGuards(t *testing.T) {
	result, _ := buildBoundedSupervisorResult(map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "Let the supported relationship affect the scene subtly.", "source_refs": []any{"character-memory:relationship"}},
			},
		},
	}, publisherPlanTestPack("weak"))
	plan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
	if !stringSliceContains(stringSliceFromAny(plan["relationship_expression_refs"]), "character-memory:relationship") {
		t.Fatalf("directional relationship ref was not categorized from delivered E metadata: %#v", plan)
	}
	guidance := supervisorSceneProposalGuidanceItems(result)
	if len(guidance) != 1 || !strings.Contains(guidance[0].Text, "guarded subtext") || !strings.Contains(guidance[0].Text, "do not infer reciprocity") {
		t.Fatalf("rendered publisher guidance lost privacy or directional guard: %#v", guidance)
	}
}

func TestPublisherPlanFailOpenStatesRenderZeroGuidance(t *testing.T) {
	tests := []struct {
		name   string
		parsed map[string]any
	}{
		{name: "provider malformed", parsed: nil},
		{
			name:   "schema invalid",
			parsed: map[string]any{"supervisor_scene_proposal": map[string]any{"expression_hints": "invalid"}},
		},
		{
			name: "unsupported",
			parsed: map[string]any{"supervisor_scene_proposal": map[string]any{
				"expression_hints": []any{map[string]any{"kind": "invented", "text": "x", "source_refs": []any{"input:latest"}}},
			}},
		},
		{
			name:   "valid empty",
			parsed: map[string]any{"supervisor_scene_proposal": map[string]any{}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _ := buildBoundedSupervisorResult(test.parsed, publisherPlanTestPack("strong"))
			plan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
			if extractionStringFromAny(plan["status"]) != "zero" || len(outputFidelityLineageSlice(plan["guidance_items"])) != 0 || len(supervisorSceneProposalGuidanceItems(result)) != 0 {
				t.Fatalf("%s produced partial/default publisher guidance: result=%#v", test.name, result)
			}
		})
	}
}

func TestPublisherPlanUsesOneExistingOutputGuidanceLane(t *testing.T) {
	result, _ := buildBoundedSupervisorResult(publisherPlanTestProposal(), publisherPlanTestPack("strong"))
	plan := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 10000, supervisorSceneProposalGuidanceItems(result), "applied")
	outputLanes := 0
	for _, raw := range outputFidelityLineageSlice(plan["lanes"]) {
		if extractionStringFromAny(mapFromAny(raw)["key"]) == "output_guidance" {
			outputLanes++
		}
	}
	if outputLanes != 1 {
		t.Fatalf("output_guidance lane count = %d, want exactly 1: %#v", outputLanes, plan["lanes"])
	}
}

func publisherPlanTestPack(strength string) map[string]any {
	pack := supervisorBoundaryTestPack(strength)
	contractRefs := mapFromAny(mapFromAny(pack["response_execution_contract"])["source_refs"])
	characterRefs := []string{"character-memory:voice", "character-memory:relationship"}
	contractRefs["memory"] = append(stringSliceFromAny(contractRefs["memory"]), characterRefs...)
	contractRefs["character_memory"] = characterRefs
	contractRefs["all"] = append(stringSliceFromAny(contractRefs["all"]), characterRefs...)
	packet := mapFromAny(pack["support_packet"])
	packet["delivered_character_memory"] = []map[string]any{
		{
			"source_ref": "character-memory:voice", "final_text": "Mira voice principle", "class": "character_objective", "kind": "voice_behavior",
			"visibility_boundary": "delivered_projection_text_only",
		},
		{
			"source_ref": "character-memory:relationship", "final_text": "Mira -> Noah trust", "class": "subjective_relationship", "kind": "relationship_state",
			"privacy_guard": "subtext_only_do_not_reveal_private_fact", "visibility_boundary": "delivered_projection_text_only",
		},
	}
	return pack
}

func publisherPlanTestProposal() map[string]any {
	return map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{
				map[string]any{"text": "Account for the delivered memory.", "source_refs": []any{"memory:delivered"}},
			},
			"expression_hints": []any{
				map[string]any{"kind": "response_focus", "text": "Keep the current request in focus.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "must_account", "text": "Account for the delivered continuity.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "portrayal", "text": "Use the delivered voice principle.", "source_refs": []any{"character-memory:voice"}},
				map[string]any{"kind": "portrayal", "text": "Keep the directional relationship subtle.", "source_refs": []any{"character-memory:relationship"}},
				map[string]any{"kind": "callback", "text": "Use the delivered continuity as a callback.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "pacing", "text": "Keep the response pacing measured.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "scene_emphasis", "text": "Emphasize the established scene tension.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "may_advance", "text": "The delivered scene may advance.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "hold_allowed", "text": "Holding the current beat is allowed.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "hold_allowed", "text": "The delivered memory also permits holding.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "arc_anchor", "text": "Use the delivered continuity as the anchor.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "preferred_frontier", "text": "Prefer the already-open frontier.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "ending_edge", "text": "End at the supported open edge.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "world_guard", "text": "Do not contradict the delivered world state.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "must_not", "text": "Do not close the delivered unresolved point.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "reversible_option", "text": "Keep the supported option reversible.", "source_refs": []any{"memory:delivered"}},
			},
		},
	}
}
