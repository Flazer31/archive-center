package httpapi

import (
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
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	director := mapFromAny(plan["director"])
	book["current_arc"] = map[string]any{"text": "Keep the quiet examination conversation central.", "source_refs": []any{"input:latest"}}
	book["narrative_goal"] = map[string]any{"text": "Let Han-eol weigh the opportunity without deciding for him.", "source_refs": []any{"input:latest"}}
	book["next_beats"] = []any{
		map[string]any{"text": "Preserve the pause before his answer.", "source_refs": []any{"memory:delivered"}},
		map[string]any{"text": "Keep the practical stakes visible.", "source_refs": []any{"input:latest"}},
	}
	book["guardrails"] = []any{map[string]any{"text": "Do not invent an examination result.", "source_refs": []any{"memory:delivered"}}}
	director["scene_mandate"] = map[string]any{"text": "Stay in the current room and conversation.", "source_refs": []any{"input:latest"}}
	director["required_outcomes"] = []any{map[string]any{"text": "Show the opportunity being understood.", "source_refs": []any{"memory:delivered"}}}
	director["forbidden_moves"] = []any{map[string]any{"text": "Do not force Han-eol to accept.", "source_refs": []any{"input:latest"}}}
	director["pressure_level"] = map[string]any{"level": "quiet", "text": "Keep the scene quiet.", "source_refs": []any{"input:latest"}}

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
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	director := mapFromAny(plan["director"])
	book["current_arc"] = map[string]any{"text": "Keep the current quiet scene.", "source_refs": []any{"input:latest"}}
	director["pressure_level"] = map[string]any{"level": "quiet", "text": "Do not raise the pressure.", "source_refs": []any{"input:latest"}}

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
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	book["current_arc"] = map[string]any{
		"text": "Stay with the current opportunity.", "source_refs": []any{"input:latest"}, "legacy_hint": "discard",
	}
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

func TestPublisherPlanV2MissingOneRoleKeepsOtherRole(t *testing.T) {
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	delete(plan, "director")
	book := mapFromAny(plan["book_author"])
	book["narrative_goal"] = map[string]any{"text": "Let the response acknowledge the practical path.", "source_refs": []any{"input:latest"}}
	result, _ := buildBoundedSupervisorResult(parsed, supervisorBoundaryTestPack("weak"))
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	acceptedPlan := mapFromAny(proposal["publisher_plan"])
	if proposal["status"] != "partial" || len(anySliceFromAny(acceptedPlan["accepted_items"])) != 1 {
		t.Fatalf("readable role was rejected with missing sibling role: %#v", proposal)
	}
}

func TestPublisherPlanV2GuidanceUsesExistingWholeBlockBudget(t *testing.T) {
	parsed := publisherV2EmptyParsed()
	plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
	book := mapFromAny(plan["book_author"])
	book["current_arc"] = map[string]any{"text": "Keep the current examination opportunity central.", "source_refs": []any{"input:latest"}}
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
		{"supervisor_scene_proposal": map[string]any{"publisher_plan": map[string]any{"contract_version": "publisher_plan.v1"}}},
		{"directive": map[string]any{"supervisor_scene_proposal": publisherV2EmptyParsed()}},
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
	valid := `{"supervisor_scene_proposal":{"publisher_plan":{"contract_version":"publisher_plan.v2","book_author":{"current_arc":null,"narrative_goal":null,"next_beats":[],"guardrails":[]},"director":{"scene_mandate":null,"required_outcomes":[],"forbidden_moves":[],"pressure_level":null}}}}`
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
		plan := mapFromAny(mapFromAny(parsed["supervisor_scene_proposal"])["publisher_plan"])
		if plan["contract_version"] != "publisher_plan.v2" {
			t.Fatalf("parsed contract = %#v", plan)
		}
	}
}

func TestPublisherJSONObjectParserRejectsAmbiguousOrMalformedOutput(t *testing.T) {
	valid := `{"supervisor_scene_proposal":{"publisher_plan":{"contract_version":"publisher_plan.v2","book_author":{"current_arc":null,"narrative_goal":null,"next_beats":[],"guardrails":[]},"director":{"scene_mandate":null,"required_outcomes":[],"forbidden_moves":[],"pressure_level":null}}}}`
	tests := map[string]string{
		"two objects":           valid + ` {}`,
		"incomplete object":     `{"supervisor_scene_proposal":`,
		"duplicate key":         `{"supervisor_scene_proposal":{},"supervisor_scene_proposal":{}}`,
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
			_, _ = w.Write([]byte(publisherV2OpenAIResponse("input:latest", "FIRST_REQUEST_PLAN_MUST_NOT_SURVIVE")))
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
		if !strings.Contains(userPrompt, `"guide_strength": "maximum"`) || !strings.Contains(userPrompt, "Continue the current scene.") {
			t.Fatalf("Publisher input or strength omitted: %s", userPrompt)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(publisherV2OpenAIResponse("input:latest", "Keep the present request central.")))
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
	if overrides["json_response_applied"] != true || overrides["json_response_schema_contract"] != "publisher_plan.v2_prompt_validated" {
		t.Fatalf("Ollama Publisher JSON trace = %+v", overrides)
	}
	if len(supervisorSceneProposalGuidanceItems(result, "standard")) != 1 {
		t.Fatalf("valid Publisher plan was not retained: %#v", result)
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
		_, _ = w.Write([]byte(publisherV2OpenAIResponse(lorebookRef, "Keep the supplied family background consistent.")))
	}))
	defer provider.Close()

	srv := setupTestServer()
	result, _, err := srv.runSupervisorLLM(context.Background(), "publisher-lorebook-reference", pack, completeTurnLLMConfig{
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
		_, _ = w.Write([]byte(publisherV2OpenAIResponse(deliveredRef, "Express only the delivered private-memory projection as subtext.")))
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
