package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func TestPublisherGuidanceFormatsPreserveAcceptedPlanAndSingleBlock(t *testing.T) {
	parsed := publisherWireV3Parsed(
		publisherWireV3Item("book_author", "current_arc", "Keep the quiet examination conversation central.", "input:latest"),
		publisherWireV3Item("book_author", "narrative_goal", "Let Han-eol weigh the opportunity without deciding for him.", "input:latest"),
		publisherWireV3Item("book_author", "next_beats", "Preserve the pause before his answer.", "memory:delivered"),
		publisherWireV3Item("book_author", "next_beats", "Keep the practical stakes visible.", "input:latest"),
		publisherWireV3Item("book_author", "guardrails", "Do not invent an examination result.", "memory:delivered"),
		publisherWireV3Item("director", "scene_mandate", "Stay in the current room and conversation.", "input:latest"),
		publisherWireV3Item("director", "required_outcomes", "Show the opportunity being understood.", "memory:delivered"),
		publisherWireV3Item("director", "forbidden_moves", "Do not force Han-eol to accept.", "input:latest"),
		publisherWireV3PressureItem("quiet", "Keep the scene quiet.", "input:latest"),
	)

	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("maximum"))
	acceptedPlan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
	acceptedBefore, err := json.Marshal(acceptedPlan["accepted_items"])
	if err != nil {
		t.Fatal(err)
	}
	expectedRefs := []string{"input:latest", "memory:delivered"}
	expectedTexts := []string{
		"Keep the quiet examination conversation central.",
		"Let Han-eol weigh the opportunity without deciding for him.",
		"Preserve the pause before his answer.",
		"Keep the practical stakes visible.",
		"Do not invent an examination result.",
		"Stay in the current room and conversation.",
		"Show the opportunity being understood.",
		"Do not force Han-eol to accept.",
		"Keep the scene quiet.",
	}
	rendered := map[string]prepareTurnGuidanceItem{}
	for _, format := range []string{"compact", "standard", "explicit"} {
		items := supervisorSceneProposalGuidanceItems(result, format)
		if len(items) != 1 {
			t.Fatalf("format %s guidance blocks = %d, want exactly one", format, len(items))
		}
		if !reflect.DeepEqual(items[0].SourceRefs, expectedRefs) {
			t.Fatalf("format %s source refs = %#v, want %#v", format, items[0].SourceRefs, expectedRefs)
		}
		for _, expectedText := range expectedTexts {
			if strings.Count(items[0].Text, expectedText) != 1 {
				t.Fatalf("format %s did not preserve accepted text exactly once: %q in %q", format, expectedText, items[0].Text)
			}
		}
		if strings.Contains(items[0].Text, "input:latest") || strings.Contains(items[0].Text, "memory:delivered") {
			t.Fatalf("format %s exposed internal source refs: %q", format, items[0].Text)
		}
		rendered[format] = items[0]
	}
	acceptedAfter, err := json.Marshal(acceptedPlan["accepted_items"])
	if err != nil {
		t.Fatal(err)
	}
	if string(acceptedAfter) != string(acceptedBefore) {
		t.Fatalf("format rendering mutated accepted plan: before=%s after=%s", acceptedBefore, acceptedAfter)
	}
	if !strings.Contains(rendered["compact"].Text, "[PG|") ||
		!strings.Contains(rendered["standard"].Text, "[Publisher Guidance]") ||
		!strings.Contains(rendered["explicit"].Text, "[PUBLISHER_PLAN]") {
		t.Fatalf("format markers missing: compact=%q standard=%q explicit=%q", rendered["compact"].Text, rendered["standard"].Text, rendered["explicit"].Text)
	}
	standardBudget := len([]rune(rendered["standard"].Text))
	for _, format := range []string{"compact", "standard", "explicit"} {
		item := rendered[format]
		if len([]rune(item.Text)) > standardBudget {
			t.Fatalf("format %s added enough wrapper text to exceed the existing standard boundary: got=%d standard=%d", format, len([]rune(item.Text)), standardBudget)
		}
		application := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, standardBudget, []prepareTurnGuidanceItem{item}, "applied")
		lane := outputFidelity36FFindLane(application, "output_guidance")
		if !boolFromAny(lane["applied"]) || extractionStringFromAny(lane["text"]) == "" {
			t.Fatalf("format %s disappeared at a budget that fits standard: %#v", format, lane)
		}
		trace := mapFromAny(application["guidance_application_trace"])
		if intFromAny(trace["trimmed_count"], -1) != 0 || boolFromAny(trace["mid_item_truncation"]) {
			t.Fatalf("format %s introduced truncation: %#v", format, trace)
		}
	}
}

func TestPublisherGuidanceFormatNormalizationKeepsStandardAsStableDefault(t *testing.T) {
	for input, want := range map[string]string{
		"": "standard", "unknown": "standard", "STANDARD": "standard",
		" compact ": "compact", "EXPLICIT": "explicit",
	} {
		if got := normalizePublisherGuidanceFormat(input); got != want {
			t.Fatalf("normalizePublisherGuidanceFormat(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPublisherStrengthAndGuidanceFormatAxesRemainIndependent(t *testing.T) {
	parsed := publisherWireV3Parsed(
		publisherWireV3Item("book_author", "current_arc", "Keep the current quiet scene.", "input:latest"),
		publisherWireV3PressureItem("quiet", "Do not raise the pressure.", "input:latest"),
	)

	var baselineAccepted any
	for _, strength := range []string{"weak", "medium", "strong", "extreme", "maximum"} {
		result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack(strength))
		acceptedPlan := mapFromAny(mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])["publisher_plan"])
		if baselineAccepted == nil {
			baselineAccepted = acceptedPlan["accepted_items"]
		} else if !reflect.DeepEqual(acceptedPlan["accepted_items"], baselineAccepted) {
			t.Fatalf("strength %s changed accepted items: got=%#v baseline=%#v", strength, acceptedPlan["accepted_items"], baselineAccepted)
		}
		for _, format := range []string{"compact", "standard", "explicit"} {
			items := supervisorSceneProposalGuidanceItems(result, format)
			if len(items) != 1 ||
				strings.Count(items[0].Text, "Keep the current quiet scene.") != 1 ||
				strings.Count(items[0].Text, "Do not raise the pressure.") != 1 {
				t.Fatalf("strength=%s format=%s changed or duplicated guidance: %#v", strength, format, items)
			}
		}
	}

	disabled, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("none"))
	for _, format := range []string{"compact", "standard", "explicit"} {
		if items := supervisorSceneProposalGuidanceItems(disabled, format); len(items) != 0 {
			t.Fatalf("none strength produced guidance in %s format: %#v", format, items)
		}
	}
}

func TestPublisherPlanV2UnknownFieldDoesNotEraseValidSibling(t *testing.T) {
	item := publisherWireV3Item("book_author", "current_arc", "Stay with the current opportunity.", "input:latest")
	item["legacy_hint"] = "discard"
	parsed := publisherWireV3Parsed(item)
	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("strong"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	acceptedPlan := mapFromAny(proposal["publisher_plan"])
	if proposal["status"] != "partial" || len(anySliceFromAny(acceptedPlan["accepted_items"])) != 1 {
		t.Fatalf("unknown field erased valid sibling: %#v", proposal)
	}
	rejected := anySliceFromAny(acceptedPlan["rejected_items"])
	if len(rejected) != 1 || mapFromAny(rejected[0])["code"] != "unknown_field" {
		t.Fatalf("unknown field trace = %#v", rejected)
	}
}

func TestPublisherWireDoesNotRequireEmptySiblingRole(t *testing.T) {
	parsed := publisherWireV3Parsed(
		publisherWireV3Item("book_author", "narrative_goal", "Let the response acknowledge the practical path.", "input:latest"),
	)
	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("weak"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	acceptedPlan := mapFromAny(proposal["publisher_plan"])
	if proposal["status"] != "ready" || len(anySliceFromAny(acceptedPlan["accepted_items"])) != 1 {
		t.Fatalf("supported item required an empty sibling role: %#v", proposal)
	}
}

func TestPublisherPlanV2GuidanceUsesExistingWholeBlockBudget(t *testing.T) {
	parsed := publisherWireV3Parsed(
		publisherWireV3Item("book_author", "current_arc", "Keep the current examination opportunity central.", "input:latest"),
	)
	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("strong"))
	guidance := supervisorSceneProposalGuidanceItems(result, "standard")
	if len(guidance) != 1 {
		t.Fatalf("guidance count = %d, want one coherent block", len(guidance))
	}
	tooSmall := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 1, guidance, "applied")
	tooSmallLane := outputFidelity36FFindLane(tooSmall, "output_guidance")
	if boolFromAny(tooSmallLane["applied"]) || extractionStringFromAny(tooSmallLane["text"]) != "" {
		t.Fatalf("whole block was partially truncated into budget: %#v", tooSmallLane)
	}
	largeEnough := buildPrepareTurnPayloadApplicationPlan("", "", "", "", true, false, 0, 0, 5000, guidance, "applied")
	largeLane := outputFidelity36FFindLane(largeEnough, "output_guidance")
	if !boolFromAny(largeLane["applied"]) || !strings.Contains(extractionStringFromAny(largeLane["text"]), "current examination opportunity") {
		t.Fatalf("whole publisher block was not delivered: %#v", largeLane)
	}
}

func TestPublisherPlanV2FailureHasNoDefaultOrStaleGuidance(t *testing.T) {
	for _, parsed := range []map[string]any{
		nil,
		{"contract_version": "publisher_output.v2", "items": []any{}},
		{"directive": map[string]any{"contract_version": publisherWireContractVersion, "items": []any{}}},
	} {
		result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("strong"))
		proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
		if len(supervisorSceneProposalGuidanceItems(result, "standard")) != 0 {
			t.Fatalf("failed plan produced guidance: %#v", result)
		}
		if _, exists := proposal["publisher_plan"]; exists {
			t.Fatalf("failed plan fabricated a default plan: %#v", proposal)
		}
	}
}

func TestPublisherJSONObjectParserAcceptsOneObjectWithHarmlessWrapper(t *testing.T) {
	valid := `{"contract_version":"publisher_output.v3","items":[]}`
	for _, content := range []string{
		valid,
		"```json\n" + valid + "\n```",
		"<think>reasoning kept outside the plan</think>\n" + valid,
		"<think>reasoning kept outside the plan</think>\n```json\n" + valid + "\n```",
		"Here is the plan:\n" + valid + "\nUse the supported result.",
		"요청한 출판사 계획입니다.\n" + valid + "\n이상입니다.",
	} {
		parsed, err := parsePublisherJSONObject(content)
		if err != nil {
			t.Fatalf("harmless wrapper was rejected: %v; content=%s", err, content)
		}
		if parsed["contract_version"] != publisherWireContractVersion {
			t.Fatalf("parsed contract = %#v", parsed)
		}
	}
}

func TestPublisherJSONObjectParserRejectsAmbiguousOrMalformedOutput(t *testing.T) {
	valid := `{"contract_version":"publisher_output.v3","items":[]}`
	tests := map[string]string{
		"two objects":           valid + ` {}`,
		"incomplete object":     `{"contract_version":"publisher_output.v3","items":`,
		"duplicate key":         `{"contract_version":"publisher_output.v3","contract_version":"publisher_output.v3","items":[]}`,
		"stray opening brace":   `unfinished { prefix ` + valid,
		"stray closing brace":   valid + ` suffix }`,
		"array before object":   `[] ` + valid,
		"array after object":    valid + ` []`,
		"true before object":    `true ` + valid,
		"null after object":     valid + ` null`,
		"number before object":  `42 ` + valid,
		"unmatched array open":  `[ prefix ` + valid,
		"unmatched array close": valid + ` suffix ]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if parsed, err := parsePublisherJSONObject(content); err == nil {
				t.Fatalf("ambiguous or malformed output was accepted: %#v", parsed)
			}
		})
	}
}

func TestPublisherE8FailedRequestDoesNotReusePreviousRequestPlan(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte(publisherV3OpenAIResponse("input:latest", "FIRST_REQUEST_PLAN_MUST_NOT_SURVIVE")))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": `{"supervisor_scene_proposal":`}}},
		})
	}))
	defer provider.Close()

	srv := setupTestServer()
	cfg := completeTurnLLMConfig{
		Provider: "openai", APIKey: "test-publisher-key", Endpoint: provider.URL,
		Model: "test-publisher", TimeoutMs: 2000, MaxTokens: 1200,
	}
	first, _, err := srv.runSupervisorLLM(context.Background(), "same-session-after-reroll-or-delete", supervisorBoundaryTestPack("strong"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	firstGuidance := supervisorSceneProposalGuidanceItems(first, "standard")
	if len(firstGuidance) != 1 || !strings.Contains(firstGuidance[0].Text, "FIRST_REQUEST_PLAN_MUST_NOT_SURVIVE") {
		t.Fatalf("first request did not produce its request-scoped plan: %#v", first)
	}

	secondPack := supervisorBoundaryTestPack("strong")
	mapFromAny(mapFromAny(secondPack["support_packet"])["current_input"])["raw_text"] = "A new request after a reroll or tail deletion."
	second, trace, err := srv.runSupervisorLLM(context.Background(), "same-session-after-reroll-or-delete", secondPack, cfg)
	if err != nil {
		t.Fatal(err)
	}
	secondProposal := mapFromAny(mapFromAny(second["directive"])["supervisor_scene_proposal"])
	if secondProposal["status"] != "publisher_json_malformed" || trace["parse_status"] != "publisher_json_malformed" {
		t.Fatalf("second request failure was not explicit: result=%#v trace=%#v", second, trace)
	}
	if _, exists := secondProposal["publisher_plan"]; exists {
		t.Fatalf("second request reused or fabricated a publisher plan: %#v", secondProposal)
	}
	if guidance := supervisorSceneProposalGuidanceItems(second, "standard"); len(guidance) != 0 {
		t.Fatalf("second request reused previous guidance: %#v", guidance)
	}
	serialized, _ := json.Marshal(second)
	if strings.Contains(string(serialized), "FIRST_REQUEST_PLAN_MUST_NOT_SURVIVE") {
		t.Fatalf("previous request plan survived in the next result: %s", serialized)
	}
	if calls.Load() != 2 {
		t.Fatalf("publisher calls = %d, want exactly one per request", calls.Load())
	}
}

func TestPublisherTruncatedJSONFailsOpenAfterOneProviderCall(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "provider-neutral-model",
			"choices": []any{map[string]any{
				"finish_reason": "length",
				"message":       map[string]any{"content": `{"supervisor_scene_proposal":{"publisher_plan":`},
			}},
			"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120},
		})
	}))
	defer provider.Close()

	srv := setupTestServer()
	result, trace, err := srv.runSupervisorLLM(context.Background(), "publisher-truncated", supervisorBoundaryTestPack("strong"), completeTurnLLMConfig{
		Provider: "openai", APIKey: "test-publisher-key", Endpoint: provider.URL,
		Model: "provider-neutral-model", TimeoutMs: 2000, MaxTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Publisher provider calls=%d, want exactly one", calls.Load())
	}
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "publisher_json_truncated" || trace["parse_status"] != "publisher_json_truncated" {
		t.Fatalf("truncated Publisher result was not explicit fail-open: result=%#v trace=%#v", result, trace)
	}
	if len(supervisorSceneProposalGuidanceItems(result, "standard")) != 0 {
		t.Fatalf("truncated Publisher response produced guidance: %#v", result)
	}
	metadata := mapFromAny(trace["provider_response"])
	if metadata["termination_kind"] != "length" || intFromAny(metadata["output_tokens"], 0) != 20 {
		t.Fatalf("Publisher provider response metadata=%#v", metadata)
	}
	ledger := mapFromAny(trace["provider_call_budget_ledger"])
	if ledger["contract_version"] != providerCallBudgetLedgerContractV1 || ledger["owner"] != "go" ||
		ledger["call_kind"] != "publisher" || ledger["status"] != "failed_open" || ledger["failure_stage"] != "json_parse" {
		t.Fatalf("Publisher call ledger failure classification=%#v", ledger)
	}
	if intFromAny(ledger["base_prompt_chars"], 0) != intFromAny(ledger["system_prompt_chars"], 0) ||
		intFromAny(ledger["system_prompt_chars"], 0) <= 0 || intFromAny(ledger["final_prompt_chars"], 0) <= intFromAny(ledger["system_prompt_chars"], 0) ||
		intFromAny(ledger["json_schema_output_requirement_chars"], -1) != 0 || ledger["json_schema_output_requirement_accounting"] != "system_prompt_and_provider_schema" ||
		intFromAny(ledger["support_packet_text_chars"], 0) <= 0 || intFromAny(ledger["support_packet_metadata_chars"], 0) <= 0 || ledger["provider_usage_status"] != "reported" ||
		intFromAny(ledger["input_tokens"], 0) != 100 || intFromAny(ledger["output_tokens"], 0) != 20 {
		t.Fatalf("Publisher call ledger sizes or usage=%#v", ledger)
	}
	if trace["parser_error"] == "" || trace["json_incomplete"] != true || intFromAny(trace["top_level_object_count"], -1) != 0 ||
		len([]rune(extractionStringFromAny(trace["raw_preview"]))) > 1000 {
		t.Fatalf("Publisher parse diagnostics=%#v", trace)
	}
}

func TestProviderCallBudgetLedgerDoesNotEstimateUnreportedTokens(t *testing.T) {
	ledger := newProviderCallBudgetLedger("publisher", "SYSTEM", "USER", providerCallBudgetComponents{
		CurrentTurnChars:                      2,
		AuxiliaryMemoryChars:                  1,
		OriginalWorkReferenceStatus:           "not_in_call_contract",
		LorebookReferenceStatus:               "not_in_call_contract",
		JSONSchemaOutputRequirementAccounting: "separate_user_payload_field",
	})
	observeProviderCallBudgetResult(ledger, map[string]any{
		"usage_reported":   false,
		"termination_kind": "complete",
	}, http.StatusOK, "succeeded", "")
	if ledger["provider_usage_status"] != "unreported" {
		t.Fatalf("unreported provider usage was reclassified: %#v", ledger)
	}
	for _, key := range []string{"input_tokens", "output_tokens", "reasoning_tokens", "cached_input_tokens", "total_tokens"} {
		if _, exists := ledger[key]; exists {
			t.Fatalf("unreported provider usage gained estimated %s: %#v", key, ledger)
		}
	}
}

func TestPublisherOllamaSingleCallPreservesInputStrengthModelAndReasoning(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode Publisher request: %v", err)
		}
		if request["model"] != "deepseek-v4-flash:0731-cloud" {
			t.Fatalf("model = %v", request["model"])
		}
		if _, ok := request["thinking"]; ok {
			t.Fatalf("Ollama Publisher received DeepSeek-native thinking: %+v", request)
		}
		if request["reasoning_effort"] != "high" {
			t.Fatalf("reasoning settings were changed or omitted: %+v", request)
		}
		if request["max_tokens"] != float64(3600) {
			t.Fatalf("DeepSeek Publisher did not preserve output plus reasoning capacity: %+v", request)
		}
		if _, ok := request["reasoning_budget_tokens"]; ok {
			t.Fatalf("DeepSeek Publisher sent an unsupported reasoning budget field: %+v", request)
		}
		if mapFromAny(request["response_format"])["type"] != "json_object" {
			t.Fatalf("Ollama Publisher JSON mode missing: %+v", request)
		}
		messages := anySliceFromAny(request["messages"])
		if len(messages) != 2 {
			t.Fatalf("messages = %#v", messages)
		}
		userPrompt := extractionStringFromAny(mapFromAny(messages[1])["content"])
		var publisherInput map[string]any
		if err := json.Unmarshal([]byte(userPrompt), &publisherInput); err != nil {
			t.Fatalf("decode compact Publisher input: %v; payload=%s", err, userPrompt)
		}
		if publisherInput["guide_strength"] != "maximum" || !strings.Contains(userPrompt, "Continue the current scene.") {
			t.Fatalf("Publisher input or strength omitted: %s", userPrompt)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publisherV3OpenAIResponse("input:latest", "Keep the present request central.")))
	}))
	defer provider.Close()

	server := setupTestServer()
	result, trace, err := server.runSupervisorLLM(context.Background(), "publisher-ollama-values", supervisorBoundaryTestPack("maximum"), completeTurnLLMConfig{
		Provider: "ollama", Endpoint: provider.URL, Model: "deepseek-v4-flash:0731-cloud",
		ReasoningEffort: "high", ReasoningBudgetTokens: 2400, TimeoutMs: 2000, MaxTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Publisher provider calls = %d, want exactly one", calls.Load())
	}
	overrides := mapFromAny(trace["request_overrides"])
	if overrides["json_response_applied"] != true || overrides["json_response_schema_contract"] != publisherWireContractVersion+"_prompt_validated" {
		t.Fatalf("Ollama Publisher JSON trace = %+v", overrides)
	}
	if len(supervisorSceneProposalGuidanceItems(result, "standard")) != 1 {
		t.Fatalf("valid Publisher plan was not retained: %#v", result)
	}
}

func TestPublisherProviderReceivesCompactProjectionWithoutMutatingInternalContracts(t *testing.T) {
	pack := supervisorBoundaryTestPack("strong")
	supportPacket := mapFromAny(pack["support_packet"])
	supportPacket["contract_version"] = "supervisor_support_packet.v2"
	supportPacket["status"] = "ready"
	supportPacket["delivered_memory_count"] = 1
	supportPacket["undelivered_candidates_included"] = false
	memoryItem := mapFromAny(anySliceFromAny(supportPacket["delivered_memory"])[0])
	memoryItem["protected_guard"] = true
	memoryItem["visibility_boundary"] = "main_model_visible"

	executionContract := mapFromAny(pack["response_execution_contract"])
	executionContract["planner_support_language"] = "ko"
	executionContract["would_write"] = false
	executionContract["would_call_llm"] = true
	executionContract["concealment_guard"] = map[string]any{"active": true, "count": 2}
	executionContract["must_preserve"] = map[string]any{
		"items": []any{map[string]any{"instruction": "Preserve delivered continuity.", "source_refs": []string{"memory:delivered"}, "status": "ready"}},
		"count": 1,
	}
	executionContract["must_respond"] = map[string]any{"items": []any{}, "count": 0}
	executionContract["must_account"] = map[string]any{"items": []any{}, "count": 0}
	executionContract["must_not_assert"] = map[string]any{"items": []any{}, "count": 0}
	before, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		messages := anySliceFromAny(request["messages"])
		userPrompt := extractionStringFromAny(mapFromAny(messages[1])["content"])
		var modelInput map[string]any
		if err := json.Unmarshal([]byte(userPrompt), &modelInput); err != nil {
			t.Fatalf("decode model input: %v; input=%s", err, userPrompt)
		}
		for _, removed := range []string{"required_output", "publisher_strength_profile"} {
			if _, exists := modelInput[removed]; exists {
				t.Fatalf("compact Publisher input retained %s: %#v", removed, modelInput)
			}
		}
		modelSupport := mapFromAny(modelInput["supervisor_support_packet"])
		for _, removed := range []string{"contract_version", "status", "delivered_memory_count", "undelivered_candidates_included"} {
			if _, exists := modelSupport[removed]; exists {
				t.Fatalf("model support packet retained audit field %s: %#v", removed, modelSupport)
			}
		}
		projectedMemory := mapFromAny(anySliceFromAny(modelSupport["delivered_memory"])[0])
		if projectedMemory["source_ref"] != "memory:delivered" || projectedMemory["final_text"] != "A delivered continuity fact." || projectedMemory["protected_guard"] != true {
			t.Fatalf("model support packet lost semantic fields: %#v", projectedMemory)
		}
		if _, exists := projectedMemory["visibility_boundary"]; exists {
			t.Fatalf("model support packet retained redundant visibility metadata: %#v", projectedMemory)
		}
		modelExecution := mapFromAny(modelInput["response_execution_contract"])
		if modelExecution["planner_support_language"] != "ko" || modelExecution["concealment_active"] != true || len(modelExecution) != 6 {
			t.Fatalf("model execution projection=%#v", modelExecution)
		}
		preserve := mapFromAny(anySliceFromAny(modelExecution["must_preserve"])[0])
		if preserve["instruction"] != "Preserve delivered continuity." || !reflect.DeepEqual(stringSliceFromAny(preserve["source_refs"]), []string{"memory:delivered"}) {
			t.Fatalf("model execution instruction lost semantics: %#v", preserve)
		}
		if _, exists := preserve["status"]; exists {
			t.Fatalf("model execution instruction retained processing metadata: %#v", preserve)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publisherV3OpenAIResponse("input:latest", "Keep the current request central.")))
	}))
	defer provider.Close()

	srv := setupTestServer()
	result, trace, err := srv.runSupervisorLLM(context.Background(), "publisher-compact-projection", pack, completeTurnLLMConfig{
		Provider: "openai", APIKey: "test-publisher-key", Endpoint: provider.URL,
		Model: "test-publisher", TimeoutMs: 2000, MaxTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(supervisorSceneProposalGuidanceItems(result, "standard")) != 1 {
		t.Fatalf("valid compact Publisher plan was not retained: %#v", result)
	}
	after, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("model projection mutated internal contracts: before=%s after=%s", before, after)
	}
	ledger := mapFromAny(trace["provider_call_budget_ledger"])
	if ledger["json_schema_output_requirement_accounting"] != "system_prompt_and_provider_schema" ||
		intFromAny(ledger["json_schema_output_requirement_chars"], -1) != 0 ||
		intFromAny(ledger["support_packet_text_chars"], 0) <= 0 || intFromAny(ledger["execution_instruction_chars"], 0) <= 0 ||
		ledger["json_response_format"] != "json_schema" || ledger["json_response_schema_contract"] != publisherWireContractVersion {
		t.Fatalf("compact Publisher ledger=%#v", ledger)
	}
}

func TestPublisherJSONFailureDiagnosticsAreBoundedAndIdentifyDuplicateKey(t *testing.T) {
	content := `{"contract_version":"publisher_output.v3","contract_version":"publisher_output.v3","items":[]}` + strings.Repeat(" secret-token", 200)
	_, parseErr := parsePublisherJSONObject(content)
	if parseErr == nil {
		t.Fatal("duplicate Publisher key was accepted")
	}
	diagnostics := publisherJSONFailureDiagnostics(content, parseErr, "secret-token")
	if diagnostics["duplicate_key_name"] != "contract_version" || diagnostics["parser_error"] == "" ||
		intFromAny(diagnostics["top_level_object_count"], 0) != 1 || diagnostics["json_incomplete"] != false {
		t.Fatalf("duplicate Publisher diagnostics=%#v", diagnostics)
	}
	preview := extractionStringFromAny(diagnostics["raw_preview"])
	if len([]rune(preview)) > 1000 || strings.Contains(preview, "secret-token") {
		t.Fatalf("Publisher preview was unbounded or unsanitized: %q", preview)
	}
}

func TestPublisherJSONFailureDiagnosticsReportSyntaxOffsetWhenAvailable(t *testing.T) {
	content := `{"contract_version":"publisher_output.v3","items":[,]}`
	_, parseErr := parsePublisherJSONObject(content)
	if parseErr == nil {
		t.Fatal("malformed Publisher array was accepted")
	}
	diagnostics := publisherJSONFailureDiagnostics(content, parseErr, "")
	if intFromAny(diagnostics["syntax_offset"], 0) <= 0 || diagnostics["json_incomplete"] != false {
		t.Fatalf("Publisher syntax diagnostics=%#v", diagnostics)
	}
}

func TestPublisherAllowsOnlyDeliveredLorebookReferenceExactRefs(t *testing.T) {
	const (
		lorebookRef  = "lorebook-reference:entry:han-profile"
		lorebookText = "Han-eol is a restrained scholar from the northern branch family."
	)
	pack := supervisorBoundaryTestPack("strong")
	sourceRefs := mapFromAny(mapFromAny(pack["response_execution_contract"])["source_refs"])
	sourceRefs["lorebook_reference"] = []string{lorebookRef}
	sourceRefs["all"] = append(stringSliceFromAny(sourceRefs["all"]), lorebookRef)
	mapFromAny(pack["support_packet"])["delivered_lorebook_reference"] = []any{map[string]any{
		"final_text":  lorebookText,
		"source_refs": []string{lorebookRef},
	}}

	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode Publisher request: %v", err)
		}
		messages := anySliceFromAny(request["messages"])
		if len(messages) != 2 {
			t.Fatalf("messages = %#v", messages)
		}
		systemPrompt := extractionStringFromAny(mapFromAny(messages[0])["content"])
		userPrompt := extractionStringFromAny(mapFromAny(messages[1])["content"])
		if !strings.Contains(systemPrompt, "`delivered_lorebook_reference`") ||
			!strings.Contains(systemPrompt, "`source_ref` or `source_refs` values") {
			t.Fatalf("Publisher system prompt did not authorize exact-ref lorebook support: %s", systemPrompt)
		}
		if !strings.Contains(userPrompt, lorebookText) || !strings.Contains(userPrompt, lorebookRef) {
			t.Fatalf("Publisher request lost delivered lorebook text or exact ref: %s", userPrompt)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publisherV3OpenAIResponse(lorebookRef, "Keep the supplied family background consistent.")))
	}))
	defer provider.Close()

	srv := setupTestServer()
	result, trace, err := srv.runSupervisorLLM(context.Background(), "publisher-lorebook-reference", pack, completeTurnLLMConfig{
		Provider: "openai", APIKey: "test-publisher-key", Endpoint: provider.URL,
		Model: "test-publisher", TimeoutMs: 2000, MaxTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Publisher provider calls = %d, want exactly one", calls.Load())
	}
	guidance := supervisorSceneProposalGuidanceItems(result, "standard")
	if len(guidance) != 1 || !reflect.DeepEqual(guidance[0].SourceRefs, []string{lorebookRef}) {
		t.Fatalf("Publisher did not retain the exact delivered lorebook ref: %#v", guidance)
	}
	ledger := mapFromAny(trace["provider_call_budget_ledger"])
	if intFromAny(ledger["lorebook_reference_chars"], 0) <= 0 || ledger["lorebook_reference_status"] != "delivered" ||
		intFromAny(ledger["original_work_reference_chars"], -1) != 0 || ledger["original_work_reference_status"] != "not_in_call_contract" {
		t.Fatalf("Publisher reference-lane call ledger=%#v", ledger)
	}
}

func TestPublisherEmptyMessageContentDoesNotUseChoiceTextFallback(t *testing.T) {
	content, trace, status := normalizePublisherResponseContent(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]any{"content": ""},
			"text":    `{"supervisor_scene_proposal":{"publisher_plan":{}}}`,
		}},
	})
	if content != "" || status != "publisher_llm_empty_content" || trace["container"] != "message_content_string" {
		t.Fatalf("empty message content used another container: content=%q status=%q trace=%+v", content, status, trace)
	}
}

func TestPublisherE8ProviderReceivesProjectionWithoutRawPrivateMemory(t *testing.T) {
	const (
		rawPrivate       = "RAW_PRIVATE_MEMORY_MUST_NEVER_REACH_PUBLISHER"
		deferredPrivate  = "DEFERRED_PRIVATE_MEMORY_MUST_NEVER_REACH_PUBLISHER"
		deliveredRef     = "character-memory:delivered"
		deliveredText    = "- Mira voice principle; principle=brief_direct_requests"
		currentInputText = "Keep the present conversation quiet."
	)
	eligible := []map[string]any{
		{"source_ref": deliveredRef, "class": "character_objective", "kind": "voice_behavior", "text": deliveredText, "delivered": false, "source_metadata": map[string]any{"private_original": rawPrivate}},
		{"source_ref": "character-memory:deferred", "class": "character_objective", "kind": "character_profile", "text": deferredPrivate, "delivered": false},
	}
	support := map[string]any{
		"contract_version": prepareTurnCharacterMemoryContractVersion, "status": "eligible", "eligible_items": eligible,
		"eligible_count": 2, "delivered_items": []map[string]any{}, "raw_private_originals_included": false,
	}
	support = finalizePrepareTurnCharacterMemorySupport(support, map[string]any{"classes": []map[string]any{
		{"key": "character_objective", "text": "[Character Objective States]\n" + deliveredText},
	}})
	currentRef := "input:latest"
	sourceRules := buildResponseExecutionSourceRulesWithMemory(
		dto.PrepareTurnCurrentInputDecisionV1{SelectedObservationRef: &currentRef},
		dto.PrepareTurnHostContextReferenceEvidenceV1{},
		"publisher-e8-private-session", nil, "", support, nil,
	)
	packet := buildSupervisorSupportPacket(
		"publisher-e8-private-session", currentInputText, sourceRules, nil, "", support, nil,
	)
	pack := supervisorBoundaryTestPack("maximum")
	mapFromAny(pack["response_execution_contract"])["source_refs"] = sourceRules["source_refs"]
	pack["support_packet"] = packet

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode publisher provider request: %v", err)
		}
		messages := outputFidelityLineageSlice(request["messages"])
		if len(messages) != 2 {
			t.Errorf("publisher messages = %#v", messages)
		} else {
			userPrompt := extractionStringFromAny(mapFromAny(messages[1])["content"])
			if !strings.Contains(userPrompt, currentInputText) || !strings.Contains(userPrompt, deliveredText) {
				t.Errorf("publisher request lost current input or delivered projection: %s", userPrompt)
			}
			if strings.Contains(userPrompt, rawPrivate) || strings.Contains(userPrompt, deferredPrivate) {
				t.Errorf("publisher request exposed raw or deferred private memory: %s", userPrompt)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publisherV3OpenAIResponse(deliveredRef, "Express only the delivered private-memory projection as subtext.")))
	}))
	defer provider.Close()

	srv := setupTestServer()
	result, _, err := srv.runSupervisorLLM(context.Background(), "publisher-e8-private-session", pack, completeTurnLLMConfig{
		Provider: "openai", APIKey: "test-publisher-key", Endpoint: provider.URL,
		Model: "test-publisher", TimeoutMs: 2000, MaxTokens: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	guidance := supervisorSceneProposalGuidanceItems(result, "standard")
	if len(guidance) != 1 || !strings.Contains(guidance[0].Text, "Express only the delivered private-memory projection as subtext.") {
		t.Fatalf("publisher guidance did not preserve the delivered projection: %#v", guidance)
	}
	serialized, _ := json.Marshal(result)
	if strings.Contains(string(serialized), rawPrivate) || strings.Contains(string(serialized), deferredPrivate) {
		t.Fatalf("publisher result exposed raw or deferred private memory: %s", serialized)
	}
}
