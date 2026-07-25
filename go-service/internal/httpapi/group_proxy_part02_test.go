package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSupervisorStorylineFeedbackReplayAssumedRuntimeGate(t *testing.T) {
	const sid = "sess-e1f-replay"
	const freshContext = "Fresh gate confrontation: Mira chooses whether to expose the forged seal."
	const staleContext = "Old corridor rumor repeats the same key point without new evidence."
	const suppressedContext = "Suppressed detour must not enter supervisor prompt."

	callCount := 0
	capturedPrompts := []string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		messages, _ := body["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("upstream messages missing: %+v", body)
		}
		userMsg, _ := messages[1].(map[string]any)
		prompt := extractionStringFromAny(userMsg["content"])
		capturedPrompts = append(capturedPrompts, prompt)
		callCount++

		currentArc := "baseline_continue"
		narrativeGoal := "Continue from recent chat without storyline feedback."
		requiredOutcome := "preserve scene continuity"
		if strings.Contains(prompt, freshContext) {
			currentArc = "gate_confrontation_push"
			narrativeGoal = "Advance the fresh gate confrontation without repeating the old corridor rumor."
			requiredOutcome = "advance fresh confrontation"
		}
		response := map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": `{
				"supervisor_scene_proposal": {
					"fidelity_warnings": [{"text":"` + currentArc + `","source_refs":["memory:test:1"]}],
					"portrayal_notes": [{"text":"` + narrativeGoal + `","source_refs":["memory:test:1"]}],
					"may_advance": [{"text":"` + requiredOutcome + `","source_refs":["memory:test:1"]}]
				}
			}`}}},
			"model": "supervisor-replay",
			"usage": map[string]any{"total_tokens": 42},
		}
		data, _ := json.Marshal(response)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(data)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	run := func(withFeedback bool) map[string]any {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-supervisor-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "supervisor-replay",
			SupervisorTimeoutSec: 10,
		}
		if withFeedback {
			srv.Store = &turnRecordingStore{
				returnStorylines: []store.Storyline{
					{ID: 1, ChatSessionID: sid, Name: "Fresh gate confrontation", Status: "active", CurrentContext: freshContext, Confidence: 0.86, EvidenceCount: 4, LastEvidenceTurn: 14, LastTurn: 14},
					{ID: 2, ChatSessionID: sid, Name: "Old corridor rumor", Status: "active", CurrentContext: staleContext, Confidence: 0.91, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 2},
					{ID: 3, ChatSessionID: sid, Name: "Resolved apology", Status: "resolved", CurrentContext: "Resolved apology should remain summary-only.", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 6},
					{ID: 4, ChatSessionID: sid, Name: "Suppressed detour", Status: "active", CurrentContext: suppressedContext, Confidence: 1, EvidenceCount: 5, LastEvidenceTurn: 15, LastTurn: 15, Suppressed: true},
				},
			}
		}
		srv.RegisterRoutes(mux)

		body := `{
			"chat_session_id":"` + sid + `",
			"guide_mode":"standard",
			"guide_strength":"strong",
			"narrative_stance":"balanced",
			"response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test","memory:test:1"],"current_input":["input:test"],"native_system":[],"memory":["memory:test:1"]}},
			"wake_up_context":"The forged seal is in the guard captain's hand.",
			"persistent_guidance":"Avoid repeating stale hooks.",
			"context_messages":[
				{"role":"user","content":"I ask Mira whether we should expose the seal now."},
				{"role":"assistant","content":"Mira hesitates, watching the captain's expression."},
				{"role":"user","content":"Continue from here."}
			]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode supervisor response: %v", err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("supervisor did not use runtime LLM path: %+v", resp)
		}
		return resp
	}

	offResp := run(false)
	onResp := run(true)
	onResp2 := run(true)
	onResp3 := run(true)
	if callCount != 4 || len(capturedPrompts) != 4 {
		t.Fatalf("runtime replay calls = %d prompts = %d, want 4/4", callCount, len(capturedPrompts))
	}
	for i, prompt := range capturedPrompts {
		for _, forbidden := range []string{freshContext, staleContext, suppressedContext, "Resolved apology should remain summary-only."} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("replay %d passed storyline prose to memory fidelity reviewer %q: %s", i+1, forbidden, prompt)
			}
		}
	}

	onPack := onResp["supervisor_input_pack"].(map[string]any)
	selection := onPack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(0) {
		t.Fatalf("standalone supervisor selected storyline guidance: %+v", selection)
	}

	offArc := supervisorProposalText(offResp, "fidelity_warnings")
	onArc := supervisorProposalText(onResp, "fidelity_warnings")
	if offArc != "baseline_continue" || onArc != "baseline_continue" {
		t.Fatalf("storyline store changed fidelity result off/on = %q/%q", offArc, onArc)
	}
	for i, resp := range []map[string]any{onResp2, onResp3} {
		if arc := supervisorProposalText(resp, "fidelity_warnings"); arc != onArc {
			t.Fatalf("feedback-on replay %d current_arc = %q, want stable %q", i+2, arc, onArc)
		}
	}
	if got := supervisorProposalText(onResp, "may_advance"); got != "" {
		t.Fatalf("story advancement proposal leaked: %q", got)
	}
	if got := supervisorProposalText(onResp, "portrayal_notes"); got != "Continue from recent chat without storyline feedback." {
		t.Fatalf("portrayal_notes = %q, want context-only result", got)
	}
}

func TestNarrativeGuideModesControlledReplayDiverges(t *testing.T) {
	type modeCase struct {
		mode             string
		suffixNeedle     string
		emphasisNeedle   string
		expectedArc      string
		expectedResponse string
	}
	cases := []modeCase{
		{mode: "off", expectedArc: "baseline_arc", expectedResponse: "baseline continuation"},
		{mode: "romantic", suffixNeedle: "emotional nuance", emphasisNeedle: "relationship-aware subtext", expectedArc: "romantic_arc", expectedResponse: "romantic emotional beat"},
		{mode: "action", suffixNeedle: "clear cause and effect", emphasisNeedle: "grounded physical consequences", expectedArc: "action_arc", expectedResponse: "action forward motion"},
		{mode: "mature_soft", suffixNeedle: "sensory atmosphere", emphasisNeedle: "emotional and interpersonal nuance", expectedArc: "mature_soft_arc", expectedResponse: "sensual consent-aware beat"},
	}

	callByMode := map[string]int{}
	capturedPromptByMode := map[string]string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &reqBody); err != nil {
			t.Fatalf("decode guide replay proxy body: %v; raw=%s", err, raw)
		}
		messages, _ := reqBody["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("guide replay proxy body missing messages: %+v", reqBody)
		}
		userMessage, _ := messages[1].(map[string]any)
		body := extractionStringFromAny(userMessage["content"])
		mode := "off"
		for _, candidate := range []string{"romantic", "action", "mature_soft"} {
			if strings.Contains(body, `"guide_mode": "`+candidate+`"`) {
				mode = candidate
				break
			}
		}
		callByMode[mode]++
		capturedPromptByMode[mode] = body
		arc := map[string]string{
			"off":         "baseline_arc",
			"romantic":    "romantic_arc",
			"action":      "action_arc",
			"mature_soft": "mature_soft_arc",
		}[mode]
		responseText := map[string]string{
			"off":         "baseline continuation",
			"romantic":    "romantic emotional beat",
			"action":      "action forward motion",
			"mature_soft": "sensual consent-aware beat",
		}[mode]
		content := `{"supervisor_scene_proposal":{"fidelity_warnings":[{"text":"` + arc + `","source_refs":["memory:test:1"]}],"portrayal_notes":[{"text":"` + responseText + `","source_refs":["memory:test:1"]}],"may_advance":[{"text":"` + responseText + `","source_refs":["memory:test:1"]}]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"guide-replay","choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	results := map[string]map[string]any{}
	for _, tc := range cases {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-guide-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "guide-replay",
			SupervisorTimeoutSec: 10,
		}
		srv.RegisterRoutes(mux)
		body := `{
			"chat_session_id":"sess-guide-effect",
			"guide_mode":"` + tc.mode + `",
			"guide_strength":"strong",
			"narrative_stance":"balanced",
			"response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test","memory:test:1"],"current_input":["input:test"],"native_system":[],"memory":["memory:test:1"]}},
			"auto_advance_trigger":"none",
			"wake_up_context":"Same scene: Chloe faces the locked archive door.",
			"persistent_guidance":"Use the requested narrative mode without changing the factual scene.",
			"context_messages":[{"role":"user","content":"Continue the same scene from here."}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s supervisor status = %d, want 200: %s", tc.mode, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s decode response: %v", tc.mode, err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("%s did not use runtime supervisor path: %+v", tc.mode, resp)
		}
		results[tc.mode] = resp

		pack := resp["supervisor_input_pack"].(map[string]any)
		if pack["guide_mode"] != tc.mode {
			t.Fatalf("%s pack guide_mode = %v", tc.mode, pack["guide_mode"])
		}
		trace := resp["trace_summary"].(map[string]any)
		if trace["guide_mode"] != tc.mode {
			t.Fatalf("%s trace guide_mode = %v", tc.mode, trace["guide_mode"])
		}
		focus := stringSliceFromAny(pack["guide_focus"])
		focusText := strings.Join(focus, " / ")
		if tc.mode == "off" {
			if len(focus) != 0 {
				t.Fatalf("off mode should not add guide focus: %#v", focus)
			}
		} else {
			if !strings.Contains(focusText, tc.suffixNeedle) || !strings.Contains(focusText, tc.emphasisNeedle) {
				t.Fatalf("%s optional guide focus mismatch: %#v", tc.mode, focus)
			}
			if !strings.Contains(capturedPromptByMode[tc.mode], tc.suffixNeedle) || !strings.Contains(capturedPromptByMode[tc.mode], tc.emphasisNeedle) {
				t.Fatalf("%s upstream prompt missing optional guide focus: %s", tc.mode, capturedPromptByMode[tc.mode])
			}
		}
		if arc := supervisorProposalText(resp, "fidelity_warnings"); arc != tc.expectedArc {
			t.Fatalf("%s current_arc = %q, want %q", tc.mode, arc, tc.expectedArc)
		}
		if got := supervisorProposalText(resp, "may_advance"); got != "" {
			t.Fatalf("%s story advancement proposal leaked: %q", tc.mode, got)
		}
		if got := supervisorProposalText(resp, "portrayal_notes"); !strings.Contains(got, tc.expectedResponse) {
			t.Fatalf("%s portrayal guidance = %q, want %q", tc.mode, got, tc.expectedResponse)
		}
	}
	if len(callByMode) != len(cases) {
		t.Fatalf("runtime calls by mode = %+v, want all modes", callByMode)
	}
	if supervisorProposalText(results["off"], "fidelity_warnings") == supervisorProposalText(results["romantic"], "fidelity_warnings") ||
		supervisorProposalText(results["romantic"], "fidelity_warnings") == supervisorProposalText(results["action"], "fidelity_warnings") ||
		supervisorProposalText(results["action"], "fidelity_warnings") == supervisorProposalText(results["mature_soft"], "fidelity_warnings") {
		t.Fatalf("guide mode arcs should diverge: off=%s romantic=%s action=%s mature=%s",
			supervisorProposalText(results["off"], "fidelity_warnings"),
			supervisorProposalText(results["romantic"], "fidelity_warnings"),
			supervisorProposalText(results["action"], "fidelity_warnings"),
			supervisorProposalText(results["mature_soft"], "fidelity_warnings"))
	}
}

func TestNarrativeStanceDoesNotControlMemoryFidelityReviewer(t *testing.T) {
	cases := []string{"reactive", "balanced", "proactive"}

	callCount := 0
	capturedPrompts := []string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &reqBody); err != nil {
			t.Fatalf("decode stance replay proxy body: %v; raw=%s", err, raw)
		}
		messages, _ := reqBody["messages"].([]any)
		if len(messages) < 2 {
			t.Fatalf("stance replay proxy body missing messages: %+v", reqBody)
		}
		userMessage, _ := messages[1].(map[string]any)
		body := extractionStringFromAny(userMessage["content"])
		callCount++
		capturedPrompts = append(capturedPrompts, body)
		content := `{"supervisor_scene_proposal":{"fidelity_warnings":[{"text":"preserve the supported recollection","source_refs":["memory:test:1"]}],"portrayal_notes":[{"text":"keep the supported relationship perceptible","source_refs":["memory:test:1"]}],"may_advance":[{"text":"must be rejected","source_refs":["memory:test:1"]}]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"stance-replay","choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	results := map[string]map[string]any{}
	for _, mode := range cases {
		mux := http.NewServeMux()
		srv := setupTestServer()
		srv.RuntimeConfig = RuntimeConfig{
			SupervisorProvider:   "openai",
			SupervisorAPIKey:     "sk-stance-replay",
			SupervisorEndpoint:   "https://api.example.com/v1",
			SupervisorModel:      "stance-replay",
			SupervisorTimeoutSec: 10,
		}
		srv.RegisterRoutes(mux)
		body := `{
			"chat_session_id":"sess-stance-effect",
			"guide_mode":"off",
			"guide_strength":"strong",
			"narrative_stance":"` + mode + `",
			"response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test","memory:test:1"],"current_input":["input:test"],"native_system":[],"memory":["memory:test:1"]}},
			"auto_advance_trigger":"none",
			"wake_up_context":"Same scene: Chloe pauses at the archive door.",
			"persistent_guidance":"Use the requested initiative mode without changing the factual scene.",
			"context_messages":[{"role":"user","content":"Continue the same scene from here."}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s supervisor status = %d, want 200: %s", mode, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s decode response: %v", mode, err)
		}
		if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
			t.Fatalf("%s did not use runtime supervisor path: %+v", mode, resp)
		}
		results[mode] = resp
		pack := resp["supervisor_input_pack"].(map[string]any)
		for _, key := range []string{"narrative_stance", "narrative_stance_suffix", "narrative_stance_bounds"} {
			if _, exists := pack[key]; exists {
				t.Fatalf("%s pack exposes story initiative field %q: %#v", mode, key, pack[key])
			}
		}
		if arc := supervisorProposalText(resp, "fidelity_warnings"); arc != "preserve the supported recollection" {
			t.Fatalf("%s fidelity guidance = %q", mode, arc)
		}
		if got := supervisorProposalText(resp, "may_advance"); got != "" {
			t.Fatalf("%s story advancement proposal leaked: %q", mode, got)
		}
	}
	if callCount != len(cases) || len(capturedPrompts) != len(cases) {
		t.Fatalf("runtime calls/prompts = %d/%d, want %d", callCount, len(capturedPrompts), len(cases))
	}
	for i, prompt := range capturedPrompts {
		if strings.Contains(prompt, "narrative_stance") || strings.Contains(prompt, "Story Initiative") || strings.Contains(prompt, "max_new_beats") {
			t.Fatalf("prompt %d contains story initiative control: %s", i+1, prompt)
		}
	}
	if supervisorProposalText(results["reactive"], "fidelity_warnings") != supervisorProposalText(results["balanced"], "fidelity_warnings") ||
		supervisorProposalText(results["balanced"], "fidelity_warnings") != supervisorProposalText(results["proactive"], "fidelity_warnings") {
		t.Fatalf("narrative stance changed memory fidelity output: %#v", results)
	}
}

func supervisorProposalText(resp map[string]any, field string) string {
	result, _ := resp["supervisor_result"].(map[string]any)
	directive, _ := result["directive"].(map[string]any)
	proposal, _ := directive["supervisor_scene_proposal"].(map[string]any)
	items, _ := proposal[field].([]any)
	if len(items) == 0 {
		return ""
	}
	item, _ := items[0].(map[string]any)
	return extractionStringFromAny(item["text"])
}

func anySliceContains(values []any, needle string) bool {
	for _, value := range values {
		if strings.Contains(extractionStringFromAny(value), needle) {
			return true
		}
	}
	return false
}

func TestConfigUpdateProjectGUISettingsTraceMasksSecrets(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	const mainKey = "test-main-secret-seq02"
	const criticKey = "test-critic-secret-seq02"
	const embeddingKey = "test-embedding-secret-seq02"
	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"ollama",
		"mainApiKey":"`+mainKey+`",
		"mainEndpoint":"http://127.0.0.1:11434/v1",
		"mainModel":"glm-5.1:cloud",
		"mainTimeout":61,
		"mainTemperature":0.65,
		"mainMaxCompletionTokens":2048,
		"mainReasoningPreset":"glm",
		"mainReasoningEffort":"enable",
		"mainReasoningBudgetTokens":4096,
		"criticProvider":"ollama",
		"criticApiKey":"`+criticKey+`",
		"criticEndpoint":"http://127.0.0.1:11434/v1",
		"criticModel":"glm-5.1:cloud",
		"criticTimeout":62,
		"criticTemperature":0.21,
		"criticMaxCompletionTokens":1536,
		"criticReasoningPreset":"custom",
		"criticReasoningEffort":"high",
		"criticReasoningBudgetTokens":2048,
		"supervisorProvider":"ollama",
		"supervisorApiKey":"`+mainKey+`",
		"supervisorEndpoint":"http://127.0.0.1:11434/v1",
		"supervisorModel":"glm-5.1:cloud",
		"supervisorTimeout":63,
		"supervisorTemperature":0.65,
		"supervisorMaxCompletionTokens":2048,
		"supervisorReasoningPreset":"glm",
		"supervisorReasoningEffort":"enable",
		"supervisorReasoningBudgetTokens":4096,
		"embeddingProvider":"ollama",
		"embeddingApiKey":"`+embeddingKey+`",
		"embeddingEndpoint":"http://127.0.0.1:11434",
		"embeddingModel":"nomic-embed-text",
		"embeddingTimeout":64,
		"topK":7
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}
	body := updateRec.Body.String()
	for _, secret := range []string{mainKey, criticKey, embeddingKey} {
		if strings.Contains(body, secret) {
			t.Fatalf("config/update response leaked secret %q: %s", secret, body)
		}
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update response: %+v", updateResp)
	}
	if trace["top_k"] != float64(7) {
		t.Fatalf("runtime_config_trace.top_k = %v, want 7", trace["top_k"])
	}
	mainTrace, ok := trace["main"].(map[string]any)
	if !ok {
		t.Fatalf("main trace missing: %+v", trace)
	}
	if mainTrace["provider"] != "ollama" || mainTrace["endpoint_host"] != "127.0.0.1:11434" || mainTrace["model"] != "glm-5.1:cloud" {
		t.Fatalf("main trace did not reflect GUI settings: %+v", mainTrace)
	}
	if mainTrace["config_authority"] != "runtime_config" || mainTrace["model_source"] != "runtime.mainModel" || mainTrace["provider_source"] != "runtime.mainProvider" {
		t.Fatalf("main trace did not expose runtime UI authority/source: %+v", mainTrace)
	}
	if mainTrace["temperature"] != float64(0.65) || mainTrace["max_completion_tokens"] != float64(2048) {
		t.Fatalf("main trace did not reflect generation settings: %+v", mainTrace)
	}
	if mainTrace["reasoning_preset"] != "glm" || mainTrace["reasoning_effort"] != "enable" || mainTrace["reasoning_budget_tokens"] != float64(4096) || mainTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("main trace did not reflect reasoning settings: %+v", mainTrace)
	}
	criticTrace, ok := trace["critic"].(map[string]any)
	if !ok {
		t.Fatalf("critic trace missing: %+v", trace)
	}
	if criticTrace["provider"] != "ollama" || criticTrace["temperature"] != float64(0.21) || criticTrace["max_completion_tokens"] != float64(1536) {
		t.Fatalf("critic trace did not reflect GUI settings: %+v", criticTrace)
	}
	if criticTrace["config_authority"] != "runtime_config" || criticTrace["model_source"] != "runtime.criticModel" || criticTrace["provider_source"] != "runtime.criticProvider" {
		t.Fatalf("critic trace did not expose runtime UI authority/source: %+v", criticTrace)
	}
	if criticTrace["reasoning_preset"] != "custom" || criticTrace["reasoning_effort"] != "high" || criticTrace["reasoning_budget_tokens"] != float64(2048) {
		t.Fatalf("critic trace did not reflect reasoning settings: %+v", criticTrace)
	}
	supervisorTrace, ok := trace["supervisor"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor trace missing: %+v", trace)
	}
	if supervisorTrace["reasoning_preset"] != "glm" || supervisorTrace["reasoning_effort"] != "enable" || supervisorTrace["reasoning_budget_tokens"] != float64(4096) || supervisorTrace["glm_thinking_type"] != "enabled" {
		t.Fatalf("supervisor trace did not reflect reasoning settings: %+v", supervisorTrace)
	}
	if supervisorTrace["config_authority"] != "runtime_config" || supervisorTrace["model_source"] != "runtime.supervisorModel" || supervisorTrace["provider_source"] != "runtime.supervisorProvider" {
		t.Fatalf("supervisor trace did not expose runtime UI authority/source: %+v", supervisorTrace)
	}
	embeddingTrace, ok := trace["embedding"].(map[string]any)
	if !ok {
		t.Fatalf("embedding trace missing: %+v", trace)
	}
	if embeddingTrace["provider"] != "ollama" || embeddingTrace["endpoint_host"] != "127.0.0.1:11434" || embeddingTrace["model"] != "nomic-embed-text" {
		t.Fatalf("embedding trace did not reflect GUI settings: %+v", embeddingTrace)
	}
	if embeddingTrace["config_authority"] != "runtime_config" || embeddingTrace["model_source"] != "runtime.embeddingModel" || embeddingTrace["provider_source"] != "runtime.embeddingProvider" {
		t.Fatalf("embedding trace did not expose runtime UI authority/source: %+v", embeddingTrace)
	}

	cfg := srv.supervisorLLMConfig()
	if cfg.Provider != "ollama" || cfg.Temperature != 0.65 || cfg.MaxTokens != 2048 {
		t.Fatalf("supervisor runtime config = provider %q temp %v max %d, want ollama/0.65/2048", cfg.Provider, cfg.Temperature, cfg.MaxTokens)
	}
	if cfg.ReasoningPreset != "glm" || cfg.ReasoningEffort != "enable" || cfg.ReasoningBudgetTokens != 4096 {
		t.Fatalf("supervisor reasoning config = preset %q effort %q budget %d, want glm/enable/4096", cfg.ReasoningPreset, cfg.ReasoningEffort, cfg.ReasoningBudgetTokens)
	}
}

func TestConfigUpdateSupervisorTraceDoesNotInferMainConfig(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"openai",
		"mainApiKey":"sk-main",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"main-runtime-model",
		"supervisorTimeout":30
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace := updateResp["runtime_config_trace"].(map[string]any)
	supervisorTrace := trace["supervisor"].(map[string]any)
	if supervisorTrace["configured"] != false {
		t.Fatalf("supervisor configured = %v, want false when supervisor fields are empty: %+v", supervisorTrace["configured"], supervisorTrace)
	}
	if supervisorTrace["model"] != "" || supervisorTrace["endpoint_host"] != "" {
		t.Fatalf("supervisor trace inferred main values: %+v", supervisorTrace)
	}
	if supervisorTrace["model_source"] != "unset" || supervisorTrace["api_key_source"] != "unset" || supervisorTrace["endpoint_source"] != "unset" {
		t.Fatalf("supervisor trace should mark empty runtime fields as unset: %+v", supervisorTrace)
	}
	missing, ok := supervisorTrace["missing_fields"].([]any)
	if !ok || len(missing) != 4 {
		t.Fatalf("supervisor missing_fields = %#v, want provider/api_key/endpoint/model", supervisorTrace["missing_fields"])
	}
}

func TestChapterLLMConfigDoesNotDefaultProvider(t *testing.T) {
	srv := setupTestServer()
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.MainAPIKey = "sk-main"
	srv.RuntimeConfig.MainEndpoint = "https://api.example.com/v1"
	srv.RuntimeConfig.MainModel = "chapter-model"

	cfg := srv.chapterLLMConfig()
	if cfg.Provider != "" {
		t.Fatalf("chapter provider = %q, want empty when runtime main provider is empty", cfg.Provider)
	}
	if cfg.hasConfig() {
		t.Fatalf("chapter config hasConfig=true, want false when provider is empty")
	}
	missing := cfg.missingFields()
	foundProvider := false
	for _, field := range missing {
		if field == "provider" {
			foundProvider = true
		}
	}
	if !foundProvider {
		t.Fatalf("chapter missing fields = %v, want provider", missing)
	}
}

func TestHandleSupervisorFailOpenOnRuntimeLLMError(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	const apiKey = "sk-supervisor-fail"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad key sk-supervisor-fail"}}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainApiKey":"`+apiKey+`",
		"mainEndpoint":"https://api.example.com/v1",
		"mainModel":"supervisor-model",
		"mainProvider":"openai",
		"supervisorProvider":"openai",
		"supervisorApiKey":"`+apiKey+`",
		"supervisorEndpoint":"https://api.example.com/v1",
		"supervisorModel":"supervisor-model",
		"supervisorTimeout":30
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-sv-fail","guide_mode":"standard","guide_strength":"weak","narrative_stance":"immersive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test","memory:test:1"],"current_input":["input:test"],"native_system":[],"memory":["memory:test:1"]}},"context_messages":[{"role":"user","content":"move forward"}]}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), apiKey) {
		t.Fatalf("supervisor fail-open response leaked API key: %s", rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["source"] != "runtime_llm_error" || resp["fail_open"] != true || resp["would_call_llm"] != true {
		t.Fatalf("unexpected fail-open response: %+v", resp)
	}
	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary missing: %+v", resp)
	}
	if trace["llm_call"] != "failed" || trace["fail_open"] != true {
		t.Fatalf("trace did not expose failed fail-open call: %+v", trace)
	}
}

func TestHandleSupervisorSkipsRuntimeLLMWithoutExecutionContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	callCount := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		t.Fatalf("supervisor upstream must not be called without a ready execution contract")
		return nil, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"supervisorProvider":"openai",
		"supervisorApiKey":"sk-supervisor-gated",
		"supervisorEndpoint":"https://api.example.com/v1",
		"supervisorModel":"supervisor-model",
		"supervisorTimeout":30
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	body := `{"chat_session_id":"sess-sv-gated","guide_mode":"standard","guide_strength":"strong","context_messages":[{"role":"user","content":"move forward"}]}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected gated status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if callCount != 0 {
		t.Fatalf("supervisor upstream call count = %d, want 0", callCount)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode gated response: %v", err)
	}
	if resp["source"] != "execution_contract_gate" || resp["would_call_llm"] != false {
		t.Fatalf("unexpected execution-contract gate response: %+v", resp)
	}
	result := mapFromAny(resp["supervisor_result"])
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	if proposal["status"] != "degraded_missing_execution_contract" ||
		proposal["reason_code"] != "supervisor_execution_contract_missing" {
		t.Fatalf("missing execution contract did not produce bounded degraded result: %+v", proposal)
	}
	if len(anySliceFromAny(proposal["fidelity_warnings"])) != 0 ||
		len(anySliceFromAny(proposal["portrayal_notes"])) != 0 ||
		len(anySliceFromAny(proposal["may_advance"])) != 0 {
		t.Fatalf("gated proposal delivered unsupported items: %+v", proposal)
	}
}

func TestHandleSupervisorMissingChatSessionID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"","guide_mode":"strict"}`
	req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "missing_param" {
		t.Errorf("code = %v, want missing_param", resp["code"])
	}
}

func TestHandleCriticTestPromptAssemblyTrace(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "critic_system.txt"), []byte("critic system prompt"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "critic_prompt.txt"), []byte("critic prompt content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.PromptDir = tmpDir
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-critic2","turn_index":3,"turn_content":"test content","context":[{"role":"user"}],"output_language_override":{"language":"ko"}}`
	req := httptest.NewRequest(http.MethodPost, "/critic/test", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	trace, ok := resp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["prompt_source"] != "configured" {
		t.Errorf("trace.prompt_source = %v, want configured", trace["prompt_source"])
	}
	if trace["files_found"] != float64(2) {
		t.Errorf("trace.files_found = %v, want 2", trace["files_found"])
	}
	if trace["llm_call"] != "disabled" {
		t.Errorf("trace.llm_call = %v, want disabled", trace["llm_call"])
	}
	if trace["verdict"] != "not_executed" {
		t.Errorf("trace.verdict = %v, want not_executed", trace["verdict"])
	}
	if trace["turn_content_chars"] != float64(12) {
		t.Errorf("trace.turn_content_chars = %v, want 12", trace["turn_content_chars"])
	}
	pack, ok := resp["critic_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("critic_input_pack is not an object")
	}
	if pack["prompt_source"] != "configured" {
		t.Errorf("critic_input_pack.prompt_source = %v, want configured", pack["prompt_source"])
	}
	if pack["would_call_llm"] != false {
		t.Errorf("critic_input_pack.would_call_llm = %v, want false", pack["would_call_llm"])
	}
}
