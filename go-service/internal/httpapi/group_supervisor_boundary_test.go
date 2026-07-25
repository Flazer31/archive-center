package httpapi

import "testing"

func TestBuildBoundedSupervisorResultRejectsIndependentStoryAuthority(t *testing.T) {
	pack := supervisorBoundaryTestPack("strong")
	parsed := map[string]any{
		"directive": map[string]any{
			"book_author": map[string]any{"current_arc": "invented arc"},
			"director":    map[string]any{"required_outcomes": []any{"force the protagonist to act"}},
			"section_world": map[string]any{
				"applies": true,
				"rules":   []any{"invented world rule"},
			},
			"storylines": []any{map[string]any{"name": "invented resolved thread", "status": "resolved"}},
		},
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{
				map[string]any{"text": "Preserve the delivered recollection.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"text": "Preserve the explicit current request.", "source_refs": []any{"input:latest"}},
				map[string]any{"text": "Unknown evidence must not pass.", "source_refs": []any{"memory:invented"}},
			},
			"portrayal_notes": []any{
				map[string]any{"text": "Keep the supported relationship perceptible.", "source_refs": []any{"input:latest", "memory:delivered"}},
			},
			"may_advance": []any{
				map[string]any{"text": "Offer only a reversible next possibility.", "source_refs": []any{"input:latest"}},
			},
		},
	}

	result, trace := buildBoundedSupervisorResult(parsed, pack)
	if result["truth_authority"] != false || result["would_write"] != false || result["authority"] != "proposal_only" {
		t.Fatalf("unsafe supervisor authority: %#v", result)
	}
	directive := mapFromAny(result["directive"])
	if _, exists := directive["book_author"]; exists {
		t.Fatalf("book_author leaked through bounded result: %#v", directive)
	}
	if _, exists := directive["director"]; exists {
		t.Fatalf("director leaked through bounded result: %#v", directive)
	}
	if _, exists := directive["section_world"]; exists {
		t.Fatalf("section_world leaked through bounded result: %#v", directive)
	}
	if _, exists := directive["storylines"]; exists {
		t.Fatalf("storylines leaked through bounded result: %#v", directive)
	}
	proposal := mapFromAny(directive["supervisor_scene_proposal"])
	if proposal["contract_version"] != "supervisor_scene_proposal.v2" {
		t.Fatalf("proposal contract version = %v, want v2", proposal["contract_version"])
	}
	if proposal["truth_authority"] != false || proposal["would_write"] != false {
		t.Fatalf("proposal gained truth/write authority: %#v", proposal)
	}
	if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != 1 {
		t.Fatalf("fidelity warning count = %d, want 1 valid source-linked item: %#v", got, proposal["fidelity_warnings"])
	}
	if got := len(anySliceFromAny(proposal["portrayal_notes"])); got != 1 {
		t.Fatalf("portrayal note count = %d, want 1: %#v", got, proposal["portrayal_notes"])
	}
	if _, exists := proposal["may_advance"]; exists {
		t.Fatalf("story advancement lane leaked through bounded result: %#v", proposal["may_advance"])
	}
	if trace["accepted_items"] != 2 || trace["rejected_items"] != 2 {
		t.Fatalf("proposal trace = %#v, want 2 accepted / 2 rejected", trace)
	}
}

func TestBuildBoundedSupervisorResultStrengthChangesCoverageNotAuthority(t *testing.T) {
	raw := map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{map[string]any{"text": "warning", "source_refs": []any{"memory:delivered"}}},
			"portrayal_notes":   []any{map[string]any{"text": "portrayal", "source_refs": []any{"memory:delivered"}}},
			"may_advance":       []any{map[string]any{"text": "reversible option", "source_refs": []any{"input:latest"}}},
		},
	}
	cases := []struct {
		strength        string
		warnings        int
		portrayal       int
		coverageProfile string
	}{
		{strength: "weak", warnings: 1, portrayal: 0, coverageProfile: "guard_only"},
		{strength: "medium", warnings: 1, portrayal: 1, coverageProfile: "guard_and_portrayal"},
		{strength: "strong", warnings: 1, portrayal: 1, coverageProfile: "guard_and_portrayal"},
	}
	for _, tc := range cases {
		t.Run(tc.strength, func(t *testing.T) {
			result, _ := buildBoundedSupervisorResult(raw, supervisorBoundaryTestPack(tc.strength))
			proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
			if proposal["truth_authority"] != false || proposal["authority"] != "proposal_only" {
				t.Fatalf("%s changed authority instead of coverage: %#v", tc.strength, proposal)
			}
			if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != tc.warnings {
				t.Fatalf("%s warning count = %d, want %d", tc.strength, got, tc.warnings)
			}
			if got := len(anySliceFromAny(proposal["portrayal_notes"])); got != tc.portrayal {
				t.Fatalf("%s portrayal count = %d, want %d", tc.strength, got, tc.portrayal)
			}
			if _, exists := proposal["may_advance"]; exists {
				t.Fatalf("%s exposes story advancement lane: %#v", tc.strength, proposal["may_advance"])
			}
			coverage := mapFromAny(proposal["coverage"])
			if coverage["profile"] != tc.coverageProfile {
				t.Fatalf("%s coverage = %#v, want %q", tc.strength, coverage, tc.coverageProfile)
			}
		})
	}
}

func TestBuildBoundedSupervisorResultRequiresExecutionContract(t *testing.T) {
	result, trace := buildBoundedSupervisorResult(
		map[string]any{"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{map[string]any{"text": "must not pass", "source_refs": []any{"input:latest"}}},
		}},
		map[string]any{"guide_strength": "strong"},
	)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "degraded_missing_execution_contract" ||
		proposal["reason_code"] != "supervisor_execution_contract_missing" {
		t.Fatalf("missing execution contract did not degrade: %#v", proposal)
	}
	if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != 0 {
		t.Fatalf("unsupported proposal passed without execution contract: %#v", proposal)
	}
	if trace["contract_ready"] != false {
		t.Fatalf("contract trace = %#v, want not ready", trace)
	}
}

func TestBuildBoundedSupervisorResultRequiresAtLeastOneExecutionSourceRef(t *testing.T) {
	pack := supervisorBoundaryTestPack("strong")
	contract := mapFromAny(pack["response_execution_contract"])
	contract["source_refs"] = map[string]any{
		"all":           []string{},
		"current_input": []string{},
		"native_system": []string{},
		"memory":        []string{},
	}
	result, trace := buildBoundedSupervisorResult(
		map[string]any{"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{map[string]any{"text": "must not pass", "source_refs": []any{"input:latest"}}},
		}},
		pack,
	)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "degraded_missing_execution_contract" ||
		proposal["reason_code"] != "supervisor_execution_contract_has_no_memory_refs" {
		t.Fatalf("empty execution refs did not degrade: %#v", proposal)
	}
	if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != 0 {
		t.Fatalf("unsupported proposal passed without execution refs: %#v", proposal)
	}
	if trace["contract_ready"] != false {
		t.Fatalf("contract trace = %#v, want not ready", trace)
	}
}

func supervisorBoundaryTestPack(strength string) map[string]any {
	return map[string]any{
		"guide_strength": strength,
		"response_execution_contract": map[string]any{
			"contract_version": "response_execution_contract.v1",
			"status":           "ready",
			"active":           true,
			"source_refs": map[string]any{
				"all":           []string{"input:latest", "system:active", "memory:delivered"},
				"current_input": []string{"input:latest"},
				"native_system": []string{"system:active"},
				"memory":        []string{"memory:delivered"},
			},
		},
	}
}

func anySliceFromAny(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	if values, ok := value.([]map[string]any); ok {
		out := make([]any, len(values))
		for index := range values {
			out[index] = values[index]
		}
		return out
	}
	return nil
}
