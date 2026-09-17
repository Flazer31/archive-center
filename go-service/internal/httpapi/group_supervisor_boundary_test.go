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
	valid := `{"contract_version":"publisher_output.v3","items":[]}`
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
		`{"contract_version":"publisher_output.v3","items":`,
	} {
		if _, err := parsePublisherJSONObject(content); err == nil {
			t.Fatalf("ambiguous or incomplete JSON was accepted: %q", content)
		}
	}
}

func TestPublisherWireV3PreservesAllCanonicalFields(t *testing.T) {
	parsed := publisherWireV3Parsed(
		map[string]any{"role": "book_author", "field": "current_arc", "text": "Current arc", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "book_author", "field": "narrative_goal", "text": "Narrative goal", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "book_author", "field": "next_beats", "text": "Next beat", "source_refs": []any{"memory:delivered"}},
		map[string]any{"role": "book_author", "field": "guardrails", "text": "Guardrail", "source_refs": []any{"memory:delivered"}},
		map[string]any{"role": "director", "field": "scene_mandate", "text": "Scene mandate", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "director", "field": "required_outcomes", "text": "Required outcome", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "director", "field": "forbidden_moves", "text": "Forbidden move", "source_refs": []any{"memory:delivered"}},
		map[string]any{"role": "director", "field": "pressure_level", "level": "quiet", "text": "Quiet pressure", "source_refs": []any{"input:latest"}},
	)
	result, trace := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("maximum"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	accepted := anySliceFromAny(plan["accepted_items"])
	if proposal["status"] != "ready" || plan["contract_version"] != "publisher_plan.v2" || len(accepted) != 8 {
		t.Fatalf("wire v3 did not preserve all canonical Publisher fields: proposal=%#v trace=%#v", proposal, trace)
	}
	want := map[string]bool{
		"book_author.current_arc": false, "book_author.narrative_goal": false,
		"book_author.next_beats": false, "book_author.guardrails": false,
		"director.scene_mandate": false, "director.required_outcomes": false,
		"director.forbidden_moves": false, "director.pressure_level": false,
	}
	for _, raw := range accepted {
		item := mapFromAny(raw)
		key := extractionStringFromAny(item["role"]) + "." + extractionStringFromAny(item["field"])
		if _, exists := want[key]; !exists {
			t.Fatalf("unexpected canonical item: %#v", item)
		}
		want[key] = true
		if key == "director.pressure_level" && item["level"] != "quiet" {
			t.Fatalf("pressure level was not preserved: %#v", item)
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("canonical field %s was lost: %#v", key, accepted)
		}
	}
}

func TestPublisherWireV3RejectsOnlyInvalidItems(t *testing.T) {
	parsed := publisherWireV3Parsed(
		map[string]any{"role": "book_author", "field": "current_arc", "text": "Bad first single item", "source_refs": []any{"memory:not-delivered"}},
		map[string]any{"role": "book_author", "field": "current_arc", "text": "Keep the current scene", "source_refs": []any{"input:latest"}, "unexpected": true},
		map[string]any{"role": "book_author", "field": "next_beats", "text": "Bad ref", "source_refs": []any{"memory:not-delivered"}},
		map[string]any{"role": "producer", "field": "scene_mandate", "text": "Wrong role", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "director", "field": "pressure_level", "level": "low", "text": "Keep pressure low", "source_refs": []any{"memory:delivered"}},
	)
	result, trace := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("strong"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	if proposal["status"] != "partial" || plan["accepted_count"] != 2 || plan["rejected_count"] != 4 {
		t.Fatalf("one invalid wire item deleted valid siblings or was accepted: proposal=%#v trace=%#v", proposal, trace)
	}
	if trace["wire_contract_version"] != publisherWireContractVersion {
		t.Fatalf("wire contract trace missing: %#v", trace)
	}
}

func TestPublisherWireV3RejectsLegacyProviderShape(t *testing.T) {
	legacy := map[string]any{"supervisor_scene_proposal": map[string]any{"publisher_plan": map[string]any{"contract_version": "publisher_plan.v2"}}}
	result, _ := buildBoundedSupervisorResult(legacy, supervisorBoundaryTestPack("strong"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "publisher_schema_invalid" {
		t.Fatalf("legacy nested provider wire was accepted: %#v", proposal)
	}
	if _, exists := proposal["publisher_plan"]; exists {
		t.Fatalf("legacy wire fabricated a canonical plan: %#v", proposal)
	}
}

func TestPublisherPlanV2SeparatesValidEmptyNoValidAndSchemaFailure(t *testing.T) {
	pack := supervisorBoundaryTestPack("weak")
	emptyResult, _ := buildBoundedSupervisorResult(publisherWireV3Parsed(), pack)
	emptyProposal := mapFromAny(mapFromAny(emptyResult["directive"])["supervisor_scene_proposal"])
	if emptyProposal["status"] != "valid_empty" || mapFromAny(emptyProposal["publisher_plan"])["accepted_count"] != 0 {
		t.Fatalf("explicit empty v2 plan was not accepted: %#v", emptyProposal)
	}

	noValid := publisherWireV3Parsed(map[string]any{
		"role": "book_author", "field": "current_arc", "text": "unsupported", "source_refs": []any{"not:allowed"},
	})
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
	parsed := publisherWireV3Parsed(
		map[string]any{"role": "book_author", "field": "current_arc", "text": "Frame the immediate examination opportunity.", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "director", "field": "required_outcomes", "text": "Show Han-eol weighing the practical opening.", "source_refs": []any{"memory:delivered"}},
		map[string]any{"role": "director", "field": "required_outcomes", "text": "Rejected text must not appear.", "source_refs": []any{"not:allowed"}},
	)
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
	parsed := publisherWireV3Parsed(
		map[string]any{"role": "book_author", "field": "current_arc", "text": "Keep the current quiet conversation in view.", "source_refs": []any{"input:latest"}},
		map[string]any{"role": "book_author", "field": "next_beats", "text": "Let the response preserve the pause.", "source_refs": []any{"memory:delivered"}},
		map[string]any{"role": "director", "field": "pressure_level", "level": "quiet", "text": "Keep the scene quiet.", "source_refs": []any{"input:latest"}},
	)

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

func publisherWireV3Parsed(items ...map[string]any) map[string]any {
	rawItems := make([]any, 0, len(items))
	for _, item := range items {
		rawItems = append(rawItems, item)
	}
	return map[string]any{
		"contract_version": publisherWireContractVersion,
		"items":            rawItems,
	}
}

func publisherWireV3Item(role, field, text string, refs ...string) map[string]any {
	sourceRefs := make([]any, 0, len(refs))
	for _, ref := range refs {
		sourceRefs = append(sourceRefs, ref)
	}
	return map[string]any{"role": role, "field": field, "text": text, "source_refs": sourceRefs}
}

func publisherWireV3PressureItem(level, text string, refs ...string) map[string]any {
	item := publisherWireV3Item("director", "pressure_level", text, refs...)
	item["level"] = level
	return item
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
