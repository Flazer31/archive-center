package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
		{strength: "weak", expressions: 2, coverageProfile: "fidelity_expression_low_impact"},
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

func TestBuildBoundedSupervisorResultStrengthSpecificGuidanceRemainsOptional(t *testing.T) {
	raw := map[string]any{
		"supervisor_scene_proposal": map[string]any{
			"expression_hints": []any{
				map[string]any{"kind": "response_focus", "text": "Keep the immediate request in focus.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "must_account", "text": "Account for the delivered promise.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "may_advance", "text": "The existing negotiation may advance.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "hold_allowed", "text": "Holding the scene is also allowed.", "source_refs": []any{"input:latest"}},
				map[string]any{"kind": "arc_anchor", "text": "Keep the delivered negotiation as the arc anchor.", "source_refs": []any{"memory:delivered"}},
				map[string]any{"kind": "preferred_frontier", "text": "Prefer the already-open negotiation frontier.", "source_refs": []any{"memory:delivered"}},
			},
		},
	}
	tests := []struct {
		strength string
		kinds    []string
	}{
		{strength: "weak", kinds: []string{"response_focus", "must_account"}},
		{strength: "medium", kinds: []string{"response_focus", "must_account", "may_advance", "hold_allowed"}},
		{strength: "strong", kinds: []string{"response_focus", "must_account", "may_advance", "hold_allowed", "arc_anchor", "preferred_frontier"}},
	}
	for _, tc := range tests {
		t.Run(tc.strength, func(t *testing.T) {
			result, _ := buildBoundedSupervisorResult(raw, supervisorBoundaryTestPack(tc.strength))
			proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
			expressions := anySliceFromAny(proposal["expression_hints"])
			if len(expressions) != len(tc.kinds) {
				t.Fatalf("%s accepted %d expression hints, want %d: %#v", tc.strength, len(expressions), len(tc.kinds), expressions)
			}
			for index, kind := range tc.kinds {
				if mapFromAny(expressions[index])["kind"] != kind {
					t.Fatalf("%s expression %d = %#v, want %q", tc.strength, index, expressions[index], kind)
				}
			}
			if proposal["truth_authority"] != false || proposal["would_write"] != false || proposal["authority"] != "proposal_only" {
				t.Fatalf("%s strength-specific guidance gained authority: %#v", tc.strength, proposal)
			}
			coverage := mapFromAny(proposal["coverage"])
			if coverage["force_progress"] != false ||
				coverage["proactive_complication_opt_in"] != false ||
				coverage["blocked_user_action"] != true ||
				coverage["blocked_new_truth"] != true ||
				coverage["blocked_relationship_change"] != true ||
				coverage["blocked_unresolved_event_closure"] != true {
				t.Fatalf("%s coverage weakened authority boundaries: %#v", tc.strength, coverage)
			}
		})
	}
}

func TestBuildBoundedSupervisorResultSeparatesMalformedEmptyAndUnsupported(t *testing.T) {
	tests := []struct {
		name       string
		parsed     map[string]any
		status     string
		reasonCode string
		failOpen   bool
	}{
		{
			name:       "malformed parse failure",
			parsed:     nil,
			status:     "malformed_failed_open",
			reasonCode: "supervisor_malformed_json",
			failOpen:   true,
		},
		{
			name:       "valid empty JSON",
			parsed:     map[string]any{},
			status:     "valid_empty",
			reasonCode: "supervisor_valid_empty",
		},
		{
			name: "valid empty proposal",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"fidelity_warnings": []any{},
					"expression_hints":  []any{},
				},
			},
			status:     "valid_empty",
			reasonCode: "supervisor_valid_empty",
		},
		{
			name: "unsupported proposal",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"expression_hints": []any{
						map[string]any{
							"kind":        "force_outcome",
							"text":        "Force a relationship change.",
							"source_refs": []any{"input:latest"},
						},
					},
				},
			},
			status:     "unsupported_rejected",
			reasonCode: "supervisor_unsupported_proposal_rejected",
		},
		{
			name: "invalid proposal envelope type",
			parsed: map[string]any{
				"supervisor_scene_proposal": []any{},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
		{
			name: "invalid proposal item collection type",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"fidelity_warnings": "not-an-array",
				},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
		{
			name: "invalid proposal item type",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"expression_hints": []any{"not-an-object"},
				},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
		{
			name: "invalid proposal text type",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"fidelity_warnings": []any{
						map[string]any{"text": 123, "source_refs": []any{"memory:delivered"}},
					},
				},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
		{
			name: "invalid proposal reference element type",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"expression_hints": []any{
						map[string]any{"kind": "portrayal", "text": "typed text", "source_refs": []any{123}},
					},
				},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
		{
			name: "invalid proposal kind type",
			parsed: map[string]any{
				"supervisor_scene_proposal": map[string]any{
					"expression_hints": []any{
						map[string]any{"kind": []any{"portrayal"}, "text": "typed text", "source_refs": []any{"input:latest"}},
					},
				},
			},
			status:     "malformed_failed_open",
			reasonCode: "supervisor_schema_invalid",
			failOpen:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, trace := buildBoundedSupervisorResult(tc.parsed, supervisorBoundaryTestPack("strong"))
			proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
			if proposal["status"] != tc.status || proposal["reason_code"] != tc.reasonCode {
				t.Fatalf("proposal classification = %#v, want status=%q reason=%q", proposal, tc.status, tc.reasonCode)
			}
			if boolFromAny(trace["fail_open"]) != tc.failOpen {
				t.Fatalf("trace fail_open = %#v, want %t: %#v", trace["fail_open"], tc.failOpen, trace)
			}
			if len(anySliceFromAny(proposal["fidelity_warnings"])) != 0 ||
				len(anySliceFromAny(proposal["expression_hints"])) != 0 {
				t.Fatalf("classified empty/rejected proposal delivered items: %#v", proposal)
			}
		})
	}
}

func TestRunSupervisorLLMMalformedFailsOpenWithoutRawProviderText(t *testing.T) {
	const rawProviderText = "RAW_PROVIDER_TEXT_MUST_NOT_ESCAPE"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"model":"supervisor-test","choices":[{"message":{"content":"` + rawProviderText + `"}}]}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	pack := supervisorBoundaryTestPack("strong")
	pack["guide_mode"] = "standard"
	result, trace, err := setupTestServer().runSupervisorLLM(
		context.Background(),
		"sess-supervisor-malformed",
		pack,
		completeTurnLLMConfig{
			APIKey:    "sk-test",
			Endpoint:  "https://api.example.com/v1",
			Model:     "supervisor-test",
			Provider:  "openai",
			TimeoutMs: 1000,
		},
	)
	if err != nil {
		t.Fatalf("malformed provider content must fail open as a bounded result, got error: %v", err)
	}
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "malformed_failed_open" ||
		proposal["reason_code"] != "supervisor_malformed_json" {
		t.Fatalf("malformed provider result classification = %#v", proposal)
	}
	if trace["parse_status"] != "malformed_failed_open" {
		t.Fatalf("parse trace = %#v", trace)
	}
	publicBytes, marshalErr := json.Marshal(map[string]any{"result": result, "trace": trace})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(publicBytes), rawProviderText) {
		t.Fatalf("malformed raw provider text escaped into result or trace: %s", publicBytes)
	}
}

func TestRunPublisherLLMForwardsOllamaReasoningNoneOnFirstCall(t *testing.T) {
	oldClient := proxyHTTPClient
	calls := 0
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode publisher request: %v", err)
		}
		if body["reasoning_effort"] != "none" {
			t.Fatalf("publisher reasoning_effort = %#v, want none", body["reasoning_effort"])
		}
		if intFromAny(body["max_tokens"], 0) != 2048 {
			t.Fatalf("publisher max_tokens = %#v, want 2048", body["max_tokens"])
		}
		if _, exists := body["max_completion_tokens"]; exists {
			t.Fatalf("reasoning-disabled Ollama request must keep max_tokens without max_completion_tokens: %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"model":"publisher-test","choices":[{"message":{"content":"{\"supervisor_scene_proposal\":{\"fidelity_warnings\":[],\"expression_hints\":[]}}"}}]}`,
			)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	pack := supervisorBoundaryTestPack("strong")
	pack["guide_mode"] = "standard"
	_, trace, err := setupTestServer().runSupervisorLLM(
		context.Background(),
		"sess-publisher-ollama-no-thinking",
		pack,
		completeTurnLLMConfig{
			APIKey:          "sk-test",
			Endpoint:        "https://ollama.example/v1",
			Model:           "publisher-test",
			Provider:        "ollama",
			TimeoutMs:       1000,
			MaxTokens:       2048,
			ReasoningPreset: "auto",
			ReasoningEffort: "none",
		},
	)
	if err != nil {
		t.Fatalf("publisher call failed: %v; trace=%#v", err, trace)
	}
	if calls != 1 {
		t.Fatalf("publisher calls = %d, want exactly one", calls)
	}
	if trace["parse_status"] != "parsed" {
		t.Fatalf("publisher parse trace = %#v", trace)
	}
}

func TestRunSupervisorLLMClassifiesEmptyProviderContent(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"supervisor-test","choices":[{"message":{"content":""}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	pack := supervisorBoundaryTestPack("strong")
	pack["guide_mode"] = "standard"
	_, trace, err := setupTestServer().runSupervisorLLM(
		context.Background(),
		"sess-supervisor-empty",
		pack,
		completeTurnLLMConfig{
			APIKey:    "sk-test",
			Endpoint:  "https://api.example.com/v1",
			Model:     "supervisor-test",
			Provider:  "ollama",
			TimeoutMs: 1000,
		},
	)
	if err == nil {
		t.Fatal("empty provider content must return an error")
	}
	if trace["failure_code"] != "publisher_llm_empty_content" || intFromAny(trace["upstream_status"], 0) != http.StatusBadGateway {
		t.Fatalf("empty-content failure trace = %#v", trace)
	}
	if !strings.Contains(extractionStringFromAny(trace["failure_detail"]), "returned no text content") {
		t.Fatalf("empty-content detail missing: %#v", trace)
	}
}

func TestRunSupervisorLLMClassifiesTimeout(t *testing.T) {
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	defer func() { proxyHTTPClient = oldClient }()

	pack := supervisorBoundaryTestPack("strong")
	pack["guide_mode"] = "standard"
	_, trace, err := setupTestServer().runSupervisorLLM(
		context.Background(),
		"sess-supervisor-timeout",
		pack,
		completeTurnLLMConfig{
			APIKey:    "sk-test",
			Endpoint:  "https://api.example.com/v1",
			Model:     "supervisor-test",
			Provider:  "openai",
			TimeoutMs: 5,
		},
	)
	if err == nil {
		t.Fatal("timed-out provider call must return an error")
	}
	if trace["failure_code"] != "publisher_llm_timeout" {
		t.Fatalf("timeout failure trace = %#v", trace)
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
	packet := buildSupervisorSupportPacket("sess", "exact current input", contract, lineage, "", nil)
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
				"all":                []string{"input:latest", "system:active", "memory:delivered"},
				"current_input":      []string{"input:latest"},
				"native_system":      []string{"system:active"},
				"memory":             []string{"memory:delivered"},
				"may_advance":        []string{"memory:delivered"},
				"arc_anchor":         []string{"memory:delivered"},
				"preferred_frontier": []string{"memory:delivered"},
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
