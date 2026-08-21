package httpapi

import (
	"reflect"
	"strings"
	"testing"
)

func TestPublisherResponseNormalizationUsesOneSupportedContainer(t *testing.T) {
	tests := []struct {
		name       string
		response   map[string]any
		want       string
		wantStatus string
	}{
		{
			name: "string content",
			response: map[string]any{"choices": []any{
				map[string]any{"message": map[string]any{"content": "{\"a\":1}"}},
				map[string]any{"message": map[string]any{"content": "ignored"}},
			}},
			want: "{\"a\":1}",
		},
		{
			name: "array text parts concatenate without separators",
			response: map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "{\"a\":"},
				map[string]any{"type": "image", "url": "ignored"},
				"1}",
			}}}}},
			want: "{\"a\":1}",
		},
		{
			name:       "empty message content does not use choice text",
			response:   map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": ""}, "text": "{\"a\":1}"}}},
			wantStatus: "publisher_llm_empty_content",
		},
		{
			name:       "unsupported content never falls back",
			response:   map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": map[string]any{"text": "wrong container"}}, "text": "{\"a\":1}"}}},
			wantStatus: "publisher_response_container_invalid",
		},
		{
			name:       "empty remains explicit",
			response:   map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]any{"type": "image"}}}}}},
			wantStatus: "publisher_llm_empty_content",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, status := normalizePublisherResponseContent(tc.response)
			if got != tc.want || status != tc.wantStatus {
				t.Fatalf("normalized content/status = %q/%q, want %q/%q", got, status, tc.want, tc.wantStatus)
			}
		})
	}
}

func TestPublisherSingleJSONObjectNormalizesWrappersAndRejectsAmbiguity(t *testing.T) {
	valid := `{"supervisor_scene_proposal":{"publisher_plan":{"contract_version":"publisher_plan.v2","book_author":{},"director":{}}}}`
	for _, content := range []string{
		valid,
		"```json\n" + valid + "\n```",
		"Here is the requested plan:\n" + valid + "\nEnd of response.",
		"요청한 계획입니다.\n" + valid + "\n이상입니다.",
	} {
		if _, err := parsePublisherJSONObject(content); err != nil {
			t.Fatalf("single complete publisher JSON object rejected: %v", err)
		}
	}
	for _, content := range []string{
		valid + valid,
		"stray { wrapper " + valid,
		valid + " stray } wrapper",
		`{"a":1,"a":2}`,
		`{"supervisor_scene_proposal":`,
	} {
		if _, err := parsePublisherJSONObject(content); err == nil {
			t.Fatalf("ambiguous or incomplete JSON was accepted: %q", content)
		}
	}
}

func TestPublisherPlanV2PartialKeepsValidItems(t *testing.T) {
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	director := mapFromAny(plan["director"])
	book["current_arc"] = map[string]any{"text": "Keep the immediate examination scene in view.", "source_refs": []any{"input:latest"}}
	book["next_beats"] = []any{
		map[string]any{"text": "Let Han-eol evaluate the opportunity.", "source_refs": []any{"input:latest"}},
		map[string]any{"text": "This item has a wrong ref.", "source_refs": []any{"memory:not-delivered"}},
	}
	director["scene_mandate"] = "wrong type"
	director["pressure_level"] = map[string]any{"level": "low", "text": "Keep the pressure restrained.", "source_refs": []any{"memory:delivered"}}

	result, trace := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("strong"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	acceptedPlan := mapFromAny(proposal["publisher_plan"])
	if proposal["status"] != "partial" || acceptedPlan["contract_version"] != "publisher_plan.v2" {
		t.Fatalf("partial v2 classification missing: %#v", proposal)
	}
	accepted := anySliceFromAny(acceptedPlan["accepted_items"])
	if len(accepted) != 3 || intFromAny(trace["rejected_items"], 0) != 2 {
		t.Fatalf("valid siblings were lost or invalid siblings accepted: plan=%#v trace=%#v", acceptedPlan, trace)
	}
	for _, raw := range accepted {
		if strings.Contains(extractionStringFromAny(mapFromAny(raw)["text"]), "wrong ref") {
			t.Fatalf("invalid item reached accepted list: %#v", accepted)
		}
	}
}

func TestPublisherPlanV2SeparatesValidEmptyNoValidAndSchemaFailure(t *testing.T) {
	pack := supervisorBoundaryTestPack("weak")
	emptyResult, _ := buildBoundedSupervisorResult(publisherV2EmptyParsed(), pack)
	emptyProposal := mapFromAny(mapFromAny(emptyResult["directive"])["supervisor_scene_proposal"])
	if emptyProposal["status"] != "valid_empty" || mapFromAny(emptyProposal["publisher_plan"])["accepted_count"] != 0 {
		t.Fatalf("explicit empty v2 plan was not accepted: %#v", emptyProposal)
	}

	noValid := publisherV2EmptyParsed()
	noValidBook := mapFromAny(mapFromAny(mapFromAny(noValid["supervisor_scene_proposal"])["publisher_plan"])["book_author"])
	noValidBook["current_arc"] = map[string]any{"text": "unsupported", "source_refs": []any{"not:allowed"}}
	noValidResult, _ := buildBoundedSupervisorResult(noValid, pack)
	noValidProposal := mapFromAny(mapFromAny(noValidResult["directive"])["supervisor_scene_proposal"])
	if noValidProposal["status"] != "publisher_plan_no_valid_items" {
		t.Fatalf("invalid-only v2 plan was disguised as empty: %#v", noValidProposal)
	}

	schemaResult, _ := buildBoundedSupervisorResult(map[string]any{"directive": map[string]any{"supervisor_scene_proposal": map[string]any{}}}, pack)
	schemaProposal := mapFromAny(mapFromAny(schemaResult["directive"])["supervisor_scene_proposal"])
	if schemaProposal["status"] != "publisher_schema_invalid" {
		t.Fatalf("legacy wrapper was accepted: %#v", schemaProposal)
	}
	if _, exists := schemaProposal["publisher_plan"]; exists {
		t.Fatalf("schema failure fabricated a fallback plan: %#v", schemaProposal)
	}
}

func TestPublisherPlanV2RendersOneAcceptedOnlyGuidanceBlock(t *testing.T) {
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	director := mapFromAny(plan["director"])
	book["current_arc"] = map[string]any{"text": "Frame the immediate examination opportunity.", "source_refs": []any{"input:latest"}}
	director["required_outcomes"] = []any{
		map[string]any{"text": "Show Han-eol weighing the practical opening.", "source_refs": []any{"memory:delivered"}},
		map[string]any{"text": "Rejected text must not appear.", "source_refs": []any{"not:allowed"}},
	}
	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("medium"))
	items := supervisorSceneProposalGuidanceItems(result, "standard")
	if len(items) != 1 {
		t.Fatalf("publisher guidance blocks = %d, want exactly 1: %#v", len(items), items)
	}
	if !strings.Contains(items[0].Text, "[Book Author]") || !strings.Contains(items[0].Text, "[Director]") ||
		strings.Contains(items[0].Text, "Rejected text") || strings.Contains(items[0].Text, "memory:delivered") {
		t.Fatalf("single guidance block contains rejected text or visible refs: %s", items[0].Text)
	}
	if !reflect.DeepEqual(items[0].SourceRefs, []string{"input:latest", "memory:delivered"}) {
		t.Fatalf("guidance trace refs = %#v", items[0].SourceRefs)
	}
}

func TestPublisherSkippedGateDoesNotFabricateV2Plan(t *testing.T) {
	pack := supervisorBoundaryTestPack("none")
	result, _ := buildBoundedSupervisorResult(nil, pack)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "disabled" {
		t.Fatalf("disabled status = %#v", proposal)
	}
	if _, exists := proposal["publisher_plan"]; exists {
		t.Fatalf("disabled gate fabricated a v2 plan: %#v", proposal)
	}
}

func TestPublisherStrengthDoesNotFilterAcceptedItemsOrForcePressure(t *testing.T) {
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	director := mapFromAny(plan["director"])
	book["current_arc"] = map[string]any{"text": "Keep the current quiet conversation in view.", "source_refs": []any{"input:latest"}}
	book["next_beats"] = []any{map[string]any{"text": "Let the response preserve the pause.", "source_refs": []any{"memory:delivered"}}}
	director["pressure_level"] = map[string]any{"level": "quiet", "text": "Keep the scene quiet.", "source_refs": []any{"input:latest"}}

	weakResult, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("weak"))
	maximumResult, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("maximum"))
	weakProposal := mapFromAny(mapFromAny(weakResult["directive"])["supervisor_scene_proposal"])
	maximumProposal := mapFromAny(mapFromAny(maximumResult["directive"])["supervisor_scene_proposal"])
	weakPlan := mapFromAny(weakProposal["publisher_plan"])
	maximumPlan := mapFromAny(maximumProposal["publisher_plan"])
	if !reflect.DeepEqual(weakPlan["accepted_items"], maximumPlan["accepted_items"]) {
		t.Fatalf("strength filtered otherwise valid E-4 items: weak=%#v maximum=%#v", weakPlan, maximumPlan)
	}
	if maximumProposal["guide_strength"] != "maximum" || maximumPlan["accepted_count"] != 3 {
		t.Fatalf("maximum strength or accepted items were lost: %#v", maximumProposal)
	}
	accepted := anySliceFromAny(maximumPlan["accepted_items"])
	quietFound := false
	for _, raw := range accepted {
		item := mapFromAny(raw)
		if item["field"] == "pressure_level" && item["level"] == "quiet" {
			quietFound = true
		}
	}
	if !quietFound {
		t.Fatalf("maximum strength forced pressure above quiet: %#v", accepted)
	}
	if items := supervisorSceneProposalGuidanceItems(maximumResult, "standard"); len(items) != 1 {
		t.Fatalf("maximum strength guidance blocks = %d, want exactly 1", len(items))
	}
}

func publisherV2EmptyParsed() map[string]any {
	return map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"publisher_plan": map[string]any{
				"contract_version": "publisher_plan.v2",
				"book_author": map[string]any{
					"current_arc": nil, "narrative_goal": nil, "next_beats": []any{}, "guardrails": []any{},
				},
				"director": map[string]any{
					"scene_mandate": nil, "required_outcomes": []any{}, "forbidden_moves": []any{}, "pressure_level": nil,
				},
			},
		},
	}
}

func supervisorBoundaryTestPack(strength string) map[string]any {
	return map[string]any{
		"guide_mode":     "standard",
		"guide_strength": strength,
		"response_execution_contract": map[string]any{
			"contract_version": "response_execution_contract.v1",
			"status":           "ready",
			"active":           true,
			"source_refs": map[string]any{
				"all":               []string{"input:latest", "memory:delivered", "system:active"},
				"current_input":     []string{"input:latest"},
				"memory":            []string{"memory:delivered"},
				"continuity":        []string{},
				"delivered_context": []string{},
				"native_system":     []string{"system:active"},
			},
		},
		"support_packet": map[string]any{
			"contract_version": "supervisor_support_packet.v2",
			"status":           "ready",
			"current_input": map[string]any{
				"source_ref": "input:latest", "raw_text": "Continue the current scene.",
			},
			"accepted_recent_context":    []any{},
			"delivered_memory":           []any{map[string]any{"source_ref": "memory:delivered", "final_text": "A delivered continuity fact."}},
			"delivered_character_memory": []any{},
			"delivered_context":          []any{},
		},
	}
}

func anySliceFromAny(value any) []any {
	switch values := value.(type) {
	case []any:
		return values
	case []map[string]any:
		out := make([]any, 0, len(values))
		for _, item := range values {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
