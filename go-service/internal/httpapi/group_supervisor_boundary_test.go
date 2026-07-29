package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildBoundedSupervisorResultRejectsUnsupportedKindsAndReferences(t *testing.T) {
	pack := supervisorBoundaryTestPack("strong")
	parsed := map[string]any{
		"directive": map[string]any{
			"book_author": map[string]any{"current_arc": "invented arc"},
			"director":    map[string]any{"required_outcomes": []any{"force the protagonist to act"}},
		},
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{
				map[string]any{"text": "Preserve the delivered recollection.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"text": "Current input cannot support fidelity.", "source_refs": []any{"input:latest"}},
				map[string]any{"text": "Unknown evidence must not pass.", "source_refs": []any{"memory:invented"}},
			},
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "Keep the requested tone perceptible.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "callback", "text": "Recall the delivered promise.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "callback", "text": "Unsupported current-only callback.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "plot_twist", "text": "Unknown kind.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "pacing", "text": "Unknown ref.", "source_refs": []any{"input:invented"}},
			},
			"may_advance": []any{
				map[string]any{"text": "Legacy story advancement must not pass.", "source_refs": []any{"input:latest"}},
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
	proposal := mapFromAny(directive["supervisor_scene_proposal"])
	if proposal["contract_version"] != "supervisor_scene_proposal.v3" {
		t.Fatalf("proposal contract version = %v, want v3", proposal["contract_version"])
	}
	if proposal["truth_authority"] != false || proposal["would_write"] != false || proposal["authority"] != "proposal_only" {
		t.Fatalf("proposal gained authority: %#v", proposal)
	}
	if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != 1 {
		t.Fatalf("fidelity warning count = %d, want 1: %#v", got, proposal["fidelity_warnings"])
	}
	expressions := anySliceFromAny(proposal["expression_hints"])
	if len(expressions) != 2 {
		t.Fatalf("expression hint count = %d, want portrayal and callback: %#v", len(expressions), expressions)
	}
	if mapFromAny(expressions[0])["kind"] != "portrayal" || mapFromAny(expressions[1])["kind"] != "callback" {
		t.Fatalf("accepted expression kinds = %#v", expressions)
	}
	if _, exists := proposal["may_advance"]; exists {
		t.Fatalf("legacy story advancement lane leaked through bounded result: %#v", proposal)
	}
	if intFromAny(trace["accepted_items"], 0) != 3 || intFromAny(trace["rejected_items"], 0) != 6 {
		t.Fatalf("proposal trace = %#v, want 3 accepted / 6 rejected", trace)
	}
}

func TestBuildBoundedSupervisorResultStrengthChangesCoverageNotAuthority(t *testing.T) {
	raw := map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"fidelity_warnings": []any{
				map[string]any{"text": "warning", "source_refs": []any{"memory:delivered"}},
			},
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "portrayal", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "pacing", "text": "pacing", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "scene_emphasis", "text": "emphasis", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "callback", "text": "callback", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "reversible_option", "text": "reversible", "source_refs": []any{"input:latest"}},
			},
		},
	}
	cases := []struct {
		strength        string
		expressions     int
		coverageProfile string
	}{
		{strength: "weak", expressions: 1, coverageProfile: "fidelity_expression_low_impact"},
		{strength: "medium", expressions: 4, coverageProfile: "fidelity_expression_contextual"},
		{strength: "strong", expressions: 5, coverageProfile: "fidelity_expression_reversible"},
	}
	for _, tc := range cases {
		t.Run(tc.strength, func(t *testing.T) {
			result, _ := buildBoundedSupervisorResult(raw, supervisorBoundaryTestPack(tc.strength))
			proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
			if proposal["truth_authority"] != false || proposal["would_write"] != false || proposal["authority"] != "proposal_only" {
				t.Fatalf("%s changed authority instead of coverage: %#v", tc.strength, proposal)
			}
			if got := len(anySliceFromAny(proposal["fidelity_warnings"])); got != 1 {
				t.Fatalf("%s warning count = %d, want 1", tc.strength, got)
			}
			if got := len(anySliceFromAny(proposal["expression_hints"])); got != tc.expressions {
				t.Fatalf("%s expression count = %d, want %d", tc.strength, got, tc.expressions)
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
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "must not pass", "source_refs": []any{"input:latest"}},
			},
		}},
		map[string]any{"guide_strength": "strong"},
	)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "degraded_missing_execution_contract" ||
		proposal["reason_code"] != "supervisor_execution_contract_missing" {
		t.Fatalf("missing execution contract did not degrade: %#v", proposal)
	}
	if len(anySliceFromAny(proposal["expression_hints"])) != 0 || trace["contract_ready"] != false {
		t.Fatalf("unsupported proposal passed without execution contract: proposal=%#v trace=%#v", proposal, trace)
	}
}

func TestBuildBoundedSupervisorResultRequiresSupportedPacketLane(t *testing.T) {
	pack := supervisorBoundaryTestPack("strong")
	pack["support_packet"] = map[string]any{
		"contract_version": "supervisor_support_packet.v1",
		"status":           "empty",
	}
	result, trace := buildBoundedSupervisorResult(
		map[string]any{"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "portrayal", "text": "must not pass", "source_refs": []any{"input:latest"}},
			},
		}},
		pack,
	)
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "degraded_missing_execution_contract" ||
		proposal["reason_code"] != "supervisor_support_packet_has_no_supported_lane" {
		t.Fatalf("empty support packet did not degrade: %#v", proposal)
	}
	if len(anySliceFromAny(proposal["expression_hints"])) != 0 || trace["contract_ready"] != false {
		t.Fatalf("unsupported proposal passed without support: proposal=%#v trace=%#v", proposal, trace)
	}
}

func TestBuildSupervisorSupportPacketUsesOnlyDeliveredRenderedText(t *testing.T) {
	const rawSecret = "the hidden raw password is swordfish"
	const undeliveredSecret = "undelivered candidate secret"
	contract := map[string]any{
		"source_refs": map[string]any{
			"current_input": []string{"input:latest"},
			"memory":        []string{"memory:sess:1", "memory:sess:2"},
		},
	}
	lineage := map[string]any{
		"items": []map[string]any{
			{
				"source_row_id": 1,
				"delivered":     true,
				"final_text":    "Safe delivered recollection.",
				"raw_private":   rawSecret,
			},
			{
				"source_row_id":   2,
				"delivered":       true,
				"final_text":      "Protected continuity guard: private knowledge exists.",
				"protected_guard": true,
				"raw_private":     rawSecret,
			},
			{
				"source_row_id": 3,
				"delivered":     false,
				"final_text":    undeliveredSecret,
			},
		},
	}
	packet := buildSupervisorSupportPacket("sess", "exact current input", contract, lineage)
	encoded, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{rawSecret, undeliveredSecret} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("support packet exposed %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "exact current input") ||
		!strings.Contains(text, "Safe delivered recollection.") ||
		!strings.Contains(text, "Protected continuity guard: private knowledge exists.") {
		t.Fatalf("support packet omitted safe support: %s", text)
	}
	items := anySliceFromAny(packet["delivered_memory"])
	if len(items) != 2 || mapFromAny(items[1])["visibility_boundary"] != "rendered_protection_guard_only" {
		t.Fatalf("protected support metadata = %#v", items)
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
		"support_packet": map[string]any{
			"contract_version": "supervisor_support_packet.v1",
			"status":           "ready",
			"current_input": map[string]any{
				"source_ref": "input:latest",
				"raw_text":   "latest request",
			},
			"delivered_memory": []map[string]any{
				{
					"source_ref": "memory:delivered",
					"final_text": "delivered safe memory",
				},
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
