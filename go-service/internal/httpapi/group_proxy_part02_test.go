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

	"github.com/risulongmemory/archive-center-go/internal/dto"
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

		response := map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": publisherV3TestContent("input:test", "baseline_continue", "Continue from recent chat without storyline feedback.")}}},
			"model":   "supervisor-replay",
			"usage":   map[string]any{"total_tokens": 42},
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

	offArc := supervisorExpressionText(offResp, "portrayal")
	onArc := supervisorExpressionText(onResp, "portrayal")
	if offArc != "baseline_continue" || onArc != "baseline_continue" {
		t.Fatalf("storyline store changed current-input expression result off/on = %q/%q", offArc, onArc)
	}
	for i, resp := range []map[string]any{onResp2, onResp3} {
		if arc := supervisorExpressionText(resp, "portrayal"); arc != onArc {
			t.Fatalf("feedback-on replay %d current_arc = %q, want stable %q", i+2, arc, onArc)
		}
	}
	if got := supervisorProposalText(onResp, "fidelity_warnings"); got != "" {
		t.Fatalf("memory fidelity proposal passed without delivered memory text: %q", got)
	}
	if got := supervisorExpressionText(onResp, "pacing"); got != "Continue from recent chat without storyline feedback." {
		t.Fatalf("pacing expression = %q, want context-only result", got)
	}
}

func TestNarrativeGuideModesControlledReplayDiverges(t *testing.T) {
	type modeCase struct {
		mode             string
		suffixNeedle     string
		emphasisNeedle   string
		expectedResponse string
	}
	cases := []modeCase{
		{mode: "off"},
		{mode: "romantic", suffixNeedle: "emotional nuance", emphasisNeedle: "relationship-aware subtext", expectedResponse: "romantic emotional beat"},
		{mode: "action", suffixNeedle: "clear cause and effect", emphasisNeedle: "grounded physical consequences", expectedResponse: "action forward motion"},
		{mode: "mature_soft", suffixNeedle: "sensory atmosphere", emphasisNeedle: "emotional and interpersonal nuance", expectedResponse: "sensual consent-aware beat"},
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
		var publisherInput map[string]any
		if err := json.Unmarshal([]byte(body), &publisherInput); err != nil {
			t.Fatalf("decode compact Publisher input: %v; body=%s", err, body)
		}
		mode := extractionFirstNonEmpty(extractionStringFromAny(publisherInput["guide_mode"]), "off")
		callByMode[mode]++
		capturedPromptByMode[mode] = body
		responseText := map[string]string{
			"romantic":    "romantic emotional beat",
			"action":      "action forward motion",
			"mature_soft": "sensual consent-aware beat",
		}[mode]
		content := publisherV3TestContent("input:test", responseText)
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
		if tc.mode == "off" {
			if resp["source"] != "guide_eligibility_gate" || resp["would_call_llm"] != false {
				t.Fatalf("off mode did not preserve no-call parity: %+v", resp)
			}
		} else if resp["source"] != "runtime_llm" || resp["would_call_llm"] != true {
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
		if tc.mode != "off" {
			if got := supervisorExpressionText(resp, "portrayal"); !strings.Contains(got, tc.expectedResponse) {
				t.Fatalf("%s portrayal guidance = %q, want %q", tc.mode, got, tc.expectedResponse)
			}
		} else if got := supervisorExpressionText(resp, "portrayal"); got != "" {
			t.Fatalf("off mode exposed expression guidance: %q", got)
		}
	}
	if len(callByMode) != len(cases)-1 || callByMode["off"] != 0 {
		t.Fatalf("runtime calls by mode = %+v, want only enabled modes", callByMode)
	}
	if supervisorExpressionText(results["romantic"], "portrayal") == supervisorExpressionText(results["action"], "portrayal") ||
		supervisorExpressionText(results["action"], "portrayal") == supervisorExpressionText(results["mature_soft"], "portrayal") {
		t.Fatalf("enabled guide mode expressions should diverge: romantic=%s action=%s mature=%s",
			supervisorExpressionText(results["romantic"], "portrayal"),
			supervisorExpressionText(results["action"], "portrayal"),
			supervisorExpressionText(results["mature_soft"], "portrayal"))
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
		content := publisherV3TestContent("input:test", "keep the current request perceptible")
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
			"guide_mode":"standard",
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
		if got := supervisorExpressionText(resp, "portrayal"); got != "keep the current request perceptible" {
			t.Fatalf("%s portrayal guidance = %q", mode, got)
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
	if supervisorExpressionText(results["reactive"], "portrayal") != supervisorExpressionText(results["balanced"], "portrayal") ||
		supervisorExpressionText(results["balanced"], "portrayal") != supervisorExpressionText(results["proactive"], "portrayal") {
		t.Fatalf("narrative stance changed expression output: %#v", results)
	}
}

func supervisorProposalText(resp map[string]any, field string) string {
	return publisherAcceptedFieldText(resp, field)
}

func supervisorExpressionText(resp map[string]any, kind string) string {
	field := kind
	switch kind {
	case "portrayal", "response_focus":
		field = "current_arc"
	case "pacing":
		field = "next_beats"
	}
	return publisherAcceptedFieldText(resp, field)
}

func publisherAcceptedFieldText(resp map[string]any, field string) string {
	result := mapFromAny(resp["supervisor_result"])
	proposal := mapFromAny(mapFromAny(result["directive"])["supervisor_scene_proposal"])
	plan := mapFromAny(proposal["publisher_plan"])
	for _, raw := range anySliceFromAny(plan["accepted_items"]) {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["field"]) == field {
			return extractionStringFromAny(item["text"])
		}
	}
	return ""
}

func publisherV3TestContent(ref, currentArc string, nextBeats ...string) string {
	items := []any{
		map[string]any{"role": "book_author", "field": "current_arc", "text": currentArc, "source_refs": []any{ref}},
	}
	for _, text := range nextBeats {
		items = append(items, map[string]any{"role": "book_author", "field": "next_beats", "text": text, "source_refs": []any{ref}})
	}
	content := map[string]any{
		"contract_version": publisherWireContractVersion,
		"items":            items,
	}
	data, _ := json.Marshal(content)
	return string(data)
}

func publisherV3OpenAIResponse(ref, currentArc string, nextBeats ...string) string {
	response := map[string]any{
		"model": "test-supervisor",
		"choices": []any{map[string]any{
			"message": map[string]any{"content": publisherV3TestContent(ref, currentArc, nextBeats...)},
		}},
	}
	data, _ := json.Marshal(response)
	return string(data)
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
	if updateResp["backend_instance_id"] != srv.BackendInstanceID {
		t.Fatalf("backend_instance_id = %v, want %q", updateResp["backend_instance_id"], srv.BackendInstanceID)
	}
	trace, ok := updateResp["runtime_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_config_trace missing from config/update response: %+v", updateResp)
	}
	if trace["top_k"] != float64(7) {
		t.Fatalf("runtime_config_trace.top_k = %v, want 7", trace["top_k"])
	}
	if trace["synced"] != true {
		t.Fatalf("runtime_config_trace.synced = %v, want true", trace["synced"])
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

func TestRuntimeConfigTraceMatchesRoleCompletenessContracts(t *testing.T) {
	standard := configuredTrace("openai", "key", "https://example.test/v1", "model", 0)
	missing, _ := standard["missing_fields"].([]string)
	if standard["configured"] != false || !strings.Contains(strings.Join(missing, ","), "timeout_ms") {
		t.Fatalf("standard trace accepted zero timeout: %+v", standard)
	}
	standard = configuredTrace("openai", "key", "https://example.test/v1", "model", 60)
	if standard["configured"] != true {
		t.Fatalf("standard trace rejected complete config: %+v", standard)
	}

	sourceSearch := sourceSearchConfiguredTrace("gemini", "key", "", "gemini-search", 60)
	if sourceSearch["configured"] != true {
		t.Fatalf("source-search trace must allow provider default endpoint: %+v", sourceSearch)
	}
}

func TestConfigUpdatePropagatesLLMGatewayServiceTierToAllGenerationRoles(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"llmgateway",
		"mainApiKey":"main-key",
		"mainEndpoint":"https://api.llmgateway.io/v1",
		"mainModel":"openai/gpt-test",
		"mainLlmGatewayServiceTier":"standard",
		"supervisorProvider":"llmgateway",
		"supervisorApiKey":"supervisor-key",
		"supervisorEndpoint":"https://api.llmgateway.io/v1",
		"supervisorModel":"google-vertex/gemini-test",
		"supervisorLlmGatewayServiceTier":"priority",
		"criticProvider":"llmgateway",
		"criticApiKey":"critic-key",
		"criticEndpoint":"https://api.llmgateway.io/v1",
		"criticModel":"google-ai-studio/gemini-test",
		"criticLlmGatewayServiceTier":"flex"
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	mainCfg := srv.chapterLLMConfig()
	supervisorCfg := srv.supervisorLLMConfig()
	criticCfg := srv.completeTurnExtractionConfig(map[string]any{}).Critic
	if mainCfg.Provider != "llmgateway" || mainCfg.LLMGatewayServiceTier != "standard" {
		t.Fatalf("main LLM Gateway config = %+v", mainCfg)
	}
	if supervisorCfg.Provider != "llmgateway" || supervisorCfg.LLMGatewayServiceTier != "priority" {
		t.Fatalf("supervisor LLM Gateway config = %+v", supervisorCfg)
	}
	if criticCfg.Provider != "llmgateway" || criticCfg.LLMGatewayServiceTier != "flex" {
		t.Fatalf("critic LLM Gateway config = %+v", criticCfg)
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace := mapFromAny(updateResp["runtime_config_trace"])
	if mapFromAny(trace["main"])["llm_gateway_service_tier"] != "standard" ||
		mapFromAny(trace["supervisor"])["llm_gateway_service_tier"] != "priority" ||
		mapFromAny(trace["critic"])["llm_gateway_service_tier"] != "flex" {
		t.Fatalf("runtime service tier trace missing: %+v", trace)
	}
}

func TestConfigUpdatePropagatesClaudePromptCacheModeToAllGenerationRoles(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	updateReq := httptest.NewRequest(http.MethodPost, "/config/update", bytes.NewReader([]byte(`{
		"mainProvider":"claude",
		"mainApiKey":"main-key",
		"mainEndpoint":"https://api.anthropic.com",
		"mainModel":"claude-main",
		"mainClaudePromptCacheMode":"ephemeral_5m",
		"supervisorProvider":"claude",
		"supervisorApiKey":"supervisor-key",
		"supervisorEndpoint":"https://api.anthropic.com",
		"supervisorModel":"claude-supervisor",
		"supervisorClaudePromptCacheMode":"ephemeral_1h",
		"criticProvider":"claude",
		"criticApiKey":"critic-key",
		"criticEndpoint":"https://api.anthropic.com",
		"criticModel":"claude-critic",
		"criticClaudePromptCacheMode":"off"
	}`)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	mux.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("config/update status = %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	mainCfg := srv.chapterLLMConfig()
	supervisorCfg := srv.supervisorLLMConfig()
	criticCfg := srv.completeTurnExtractionConfig(map[string]any{}).Critic
	if mainCfg.Provider != "claude" || mainCfg.ClaudePromptCacheMode != "ephemeral_5m" {
		t.Fatalf("main Claude cache config = %+v", mainCfg)
	}
	if supervisorCfg.Provider != "claude" || supervisorCfg.ClaudePromptCacheMode != "ephemeral_1h" {
		t.Fatalf("supervisor Claude cache config = %+v", supervisorCfg)
	}
	if criticCfg.Provider != "claude" || criticCfg.ClaudePromptCacheMode != "off" {
		t.Fatalf("critic Claude cache config = %+v", criticCfg)
	}

	var updateResp map[string]any
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("decode config/update response: %v", err)
	}
	trace := mapFromAny(updateResp["runtime_config_trace"])
	if mapFromAny(trace["main"])["claude_prompt_cache_mode"] != "ephemeral_5m" ||
		mapFromAny(trace["supervisor"])["claude_prompt_cache_mode"] != "ephemeral_1h" ||
		mapFromAny(trace["critic"])["claude_prompt_cache_mode"] != "off" {
		t.Fatalf("runtime Claude prompt cache trace missing: %+v", trace)
	}

	req := dto.ProxyPluginMainRequest{}
	applyProxyOverridesFromLLMConfig(&req, criticCfg)
	if req.ClaudePromptCacheMode == nil || *req.ClaudePromptCacheMode != "off" {
		t.Fatalf("critic proxy request cache mode = %v", req.ClaudePromptCacheMode)
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

func TestRuntimeLLMConfigUsesProviderDefaultEndpointWhenUnset(t *testing.T) {
	srv := setupTestServer()
	srv.RuntimeConfig.Synced = true
	srv.RuntimeConfig.MainProvider = "openai"
	srv.RuntimeConfig.MainAPIKey = "sk-main"
	srv.RuntimeConfig.MainEndpoint = ""
	srv.RuntimeConfig.MainModel = "gpt-test"
	srv.RuntimeConfig.MainTimeoutSec = 120
	srv.RuntimeConfig.CriticProvider = "neuralwatt"
	srv.RuntimeConfig.CriticAPIKey = "nw-critic"
	srv.RuntimeConfig.CriticEndpoint = ""
	srv.RuntimeConfig.CriticModel = "critic-test"
	srv.RuntimeConfig.CriticTimeoutSec = 120

	mainCfg := srv.chapterLLMConfig()
	if mainCfg.Endpoint != "https://api.openai.com/v1" || !mainCfg.hasConfig() {
		t.Fatalf("main config = %+v, want configured OpenAI default endpoint", mainCfg)
	}
	completeCfg := srv.completeTurnExtractionConfig(map[string]any{})
	if completeCfg.Critic.Endpoint != "https://api.neuralwatt.com/v1" || !completeCfg.Critic.hasConfig() {
		t.Fatalf("critic config = %+v, want configured NeuralWatt default endpoint", completeCfg.Critic)
	}

	trace := srv.runtimeConfigTrace()
	mainTrace := trace["main"].(map[string]any)
	if mainTrace["configured"] != true || mainTrace["endpoint_host"] != "api.openai.com" || mainTrace["endpoint_source"] != "provider_default.openai" {
		t.Fatalf("main trace = %+v", mainTrace)
	}
	criticTrace := trace["critic"].(map[string]any)
	if criticTrace["configured"] != true || criticTrace["endpoint_host"] != "api.neuralwatt.com" || criticTrace["endpoint_source"] != "provider_default.neuralwatt" {
		t.Fatalf("critic trace = %+v", criticTrace)
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
	if resp["reason_code"] != "publisher_llm_upstream_rejected" {
		t.Fatalf("reason_code = %v, want publisher_llm_upstream_rejected", resp["reason_code"])
	}
	llmTrace := mapFromAny(trace["llm_trace"])
	if llmTrace["failure_code"] != "publisher_llm_upstream_rejected" || intFromAny(llmTrace["upstream_status"], 0) != http.StatusUnauthorized {
		t.Fatalf("LLM failure trace did not preserve the rejected status: %+v", llmTrace)
	}
	if strings.Contains(extractionStringFromAny(llmTrace["failure_detail"]), apiKey) {
		t.Fatalf("LLM failure trace leaked API key: %+v", llmTrace)
	}
}

func TestHandleSupervisorMissingOrEmptyPromptFailsOpenWithoutProviderCall(t *testing.T) {
	for _, tc := range []struct {
		name      string
		writeFile bool
	}{
		{name: "missing"},
		{name: "empty", writeFile: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			promptDir := t.TempDir()
			if tc.writeFile {
				if err := os.WriteFile(filepath.Join(promptDir, "supervisor_system.txt"), []byte(" \n\t"), 0644); err != nil {
					t.Fatalf("write empty Publisher prompt: %v", err)
				}
			}

			providerCalls := 0
			oldClient := proxyHTTPClient
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				providerCalls++
				t.Fatal("Publisher provider must not be called without its system prompt")
				return nil, nil
			})}
			defer func() { proxyHTTPClient = oldClient }()

			mux := http.NewServeMux()
			srv := setupTestServer()
			srv.Cfg.PromptDir = promptDir
			srv.RuntimeConfig = RuntimeConfig{
				SupervisorProvider:   "openai",
				SupervisorAPIKey:     "test-publisher-key",
				SupervisorEndpoint:   "https://api.example.com/v1",
				SupervisorModel:      "test-publisher",
				SupervisorTimeoutSec: 10,
			}
			srv.RegisterRoutes(mux)

			body := `{"chat_session_id":"publisher-prompt-unavailable","guide_mode":"standard","guide_strength":"strong","response_execution_contract":{"contract_version":"response_execution_contract.v1","status":"ready","active":true,"source_refs":{"all":["input:test"],"current_input":["input:test"],"native_system":[],"memory":[]}},"context_messages":[{"role":"user","content":"Continue the current scene."}]}`
			req := httptest.NewRequest(http.MethodPost, "/supervisor", bytes.NewReader([]byte(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("fail-open status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			if providerCalls != 0 {
				t.Fatalf("Publisher provider calls = %d, want zero", providerCalls)
			}

			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode fail-open response: %v", err)
			}
			if resp["status"] != "partial" || resp["source"] != "runtime_llm_error" ||
				resp["fail_open"] != true || resp["would_call_llm"] != false ||
				resp["reason_code"] != "publisher_system_prompt_unavailable" {
				t.Fatalf("unexpected prompt-authority fail-open response: %#v", resp)
			}
			trace := mapFromAny(resp["trace_summary"])
			llmTrace := mapFromAny(trace["llm_trace"])
			if trace["llm_call"] != "skipped" || llmTrace["failure_code"] != "publisher_system_prompt_unavailable" {
				t.Fatalf("prompt-authority failure was not explicit: trace=%#v llm_trace=%#v", trace, llmTrace)
			}
		})
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
	if _, exists := proposal["publisher_plan"]; exists {
		t.Fatalf("gated proposal fabricated a publisher plan: %+v", proposal)
	}

	nativeOnlyBody := `{
		"chat_session_id":"sess-sv-native-only",
		"guide_mode":"standard",
		"guide_strength":"strong",
		"response_execution_contract":{
			"contract_version":"response_execution_contract.v1",
			"status":"ready",
			"active":true,
			"source_refs":{
				"all":["system:active"],
				"current_input":[],
				"native_system":["system:active"],
				"memory":[]
			}
		},
		"context_messages":[{"role":"user","content":"move forward"}]
	}`
	nativeOnlyReq := httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(nativeOnlyBody))
	nativeOnlyReq.Header.Set("Content-Type", "application/json")
	nativeOnlyRec := httptest.NewRecorder()
	mux.ServeHTTP(nativeOnlyRec, nativeOnlyReq)
	if nativeOnlyRec.Code != http.StatusOK || callCount != 0 {
		t.Fatalf("native-only support called supervisor: status=%d calls=%d body=%s", nativeOnlyRec.Code, callCount, nativeOnlyRec.Body.String())
	}
	var nativeOnlyResp map[string]any
	if err := json.Unmarshal(nativeOnlyRec.Body.Bytes(), &nativeOnlyResp); err != nil {
		t.Fatalf("decode native-only response: %v", err)
	}
	nativeOnlyProposal := mapFromAny(mapFromAny(mapFromAny(nativeOnlyResp["supervisor_result"])["directive"])["supervisor_scene_proposal"])
	if nativeOnlyResp["source"] != "execution_contract_gate" ||
		nativeOnlyProposal["reason_code"] != "supervisor_support_packet_has_no_supported_lane" {
		t.Fatalf("native-only request did not use no-supported-lane gate: response=%+v proposal=%+v", nativeOnlyResp, nativeOnlyProposal)
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
