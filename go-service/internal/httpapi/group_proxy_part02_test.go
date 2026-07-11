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
				"directive": {
					"story_author": {
						"current_arc": "` + currentArc + `",
						"narrative_goal": "` + narrativeGoal + `"
					},
					"director": {
						"pressure_level": "normal",
						"required_outcomes": ["` + requiredOutcome + `"],
						"forbidden_moves": ["repeat stale storyline"]
					}
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
			"narrative_stance":"balanced",
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
	if strings.Contains(capturedPrompts[0], freshContext) {
		t.Fatalf("feedback-off prompt should not include storyline context: %s", capturedPrompts[0])
	}
	for i, prompt := range capturedPrompts[1:] {
		if !strings.Contains(prompt, freshContext) {
			t.Fatalf("feedback-on replay %d prompt missing fresh storyline context: %s", i+1, prompt)
		}
		for _, forbidden := range []string{staleContext, suppressedContext, "Resolved apology should remain summary-only."} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("feedback-on replay %d prompt contains stale/resolved/suppressed storyline %q: %s", i+1, forbidden, prompt)
			}
		}
	}

	onPack := onResp["supervisor_input_pack"].(map[string]any)
	selection := onPack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) || selection["suppressed_count"] != float64(1) || selection["resolved_summary_count"] != float64(1) {
		t.Fatalf("unexpected storyline selection summary: %+v", selection)
	}

	offArc := supervisorCurrentArc(offResp)
	onArc := supervisorCurrentArc(onResp)
	if offArc != "baseline_continue" || onArc != "gate_confrontation_push" {
		t.Fatalf("current_arc off/on = %q/%q, want baseline_continue/gate_confrontation_push", offArc, onArc)
	}
	for i, resp := range []map[string]any{onResp2, onResp3} {
		if arc := supervisorCurrentArc(resp); arc != onArc {
			t.Fatalf("feedback-on replay %d current_arc = %q, want stable %q", i+2, arc, onArc)
		}
	}
	onDirector := supervisorDirector(onResp)
	required, _ := onDirector["required_outcomes"].([]any)
	if len(required) == 0 || extractionStringFromAny(required[0]) != "advance fresh confrontation" {
		t.Fatalf("director.required_outcomes = %+v, want advance fresh confrontation", onDirector["required_outcomes"])
	}
	forbidden, _ := onDirector["forbidden_moves"].([]any)
	if len(forbidden) == 0 || !strings.Contains(extractionStringFromAny(forbidden[0]), "stale") {
		t.Fatalf("director.forbidden_moves = %+v, want stale-repeat guard", onDirector["forbidden_moves"])
	}
}

func TestNarrativeGuideModesControlledReplayDiverges(t *testing.T) {
	type modeCase struct {
		mode             string
		suffixNeedle     string
		emphasisNeedle   string
		forbiddenNeedle  string
		expectedArc      string
		expectedResponse string
	}
	cases := []modeCase{
		{mode: "off", expectedArc: "baseline_arc", expectedResponse: "baseline continuation"},
		{mode: "romantic", suffixNeedle: "Romantic", emphasisNeedle: "emotional resonance", forbiddenNeedle: "trivializing emotional moments", expectedArc: "romantic_arc", expectedResponse: "romantic emotional beat"},
		{mode: "action", suffixNeedle: "Action", emphasisNeedle: "combat choreography", forbiddenNeedle: "excessive monologuing during action", expectedArc: "action_arc", expectedResponse: "action forward motion"},
		{mode: "mature_soft", suffixNeedle: "Mature (Sensual)", emphasisNeedle: "sensory atmosphere", forbiddenNeedle: "ignoring character consent", expectedArc: "mature_soft_arc", expectedResponse: "sensual consent-aware beat"},
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
		content := `{"directive":{"story_author":{"current_arc":"` + arc + `","narrative_goal":"` + responseText + `"},"director":{"pressure_level":"normal","required_outcomes":["` + responseText + `"],"forbidden_moves":["mode-specific guard"]}}}`
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
			"narrative_stance":"balanced",
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
		suffix, _ := pack["guide_suffix"].(string)
		directorOverrides := pack["director_overrides"].(map[string]any)
		emphasis, _ := directorOverrides["emphasis"].([]any)
		forbidden, _ := directorOverrides["forbidden_moves"].([]any)
		if tc.mode == "off" {
			if suffix != "" || len(emphasis) != 0 || len(forbidden) != 0 || trace["guide_suffix_present"] != false {
				t.Fatalf("off mode should not add suffix/overrides, suffix=%q emphasis=%+v forbidden=%+v trace=%+v", suffix, emphasis, forbidden, trace["guide_suffix_present"])
			}
		} else {
			if !strings.Contains(suffix, tc.suffixNeedle) || trace["guide_suffix_present"] != true {
				t.Fatalf("%s suffix/trace mismatch: suffix=%q trace=%+v", tc.mode, suffix, trace["guide_suffix_present"])
			}
			if !anySliceContains(emphasis, tc.emphasisNeedle) {
				t.Fatalf("%s emphasis missing %q: %+v", tc.mode, tc.emphasisNeedle, emphasis)
			}
			if !anySliceContains(forbidden, tc.forbiddenNeedle) {
				t.Fatalf("%s forbidden_moves missing %q: %+v", tc.mode, tc.forbiddenNeedle, forbidden)
			}
			if !strings.Contains(capturedPromptByMode[tc.mode], tc.suffixNeedle) || !strings.Contains(capturedPromptByMode[tc.mode], tc.emphasisNeedle) {
				t.Fatalf("%s upstream prompt missing suffix/emphasis: %s", tc.mode, capturedPromptByMode[tc.mode])
			}
		}
		if arc := supervisorCurrentArc(resp); arc != tc.expectedArc {
			t.Fatalf("%s current_arc = %q, want %q", tc.mode, arc, tc.expectedArc)
		}
		director := supervisorDirector(resp)
		outcomes, _ := director["required_outcomes"].([]any)
		if len(outcomes) == 0 || !strings.Contains(extractionStringFromAny(outcomes[0]), tc.expectedResponse) {
			t.Fatalf("%s required_outcomes = %+v, want %q", tc.mode, outcomes, tc.expectedResponse)
		}
	}
	if len(callByMode) != len(cases) {
		t.Fatalf("runtime calls by mode = %+v, want all modes", callByMode)
	}
	if supervisorCurrentArc(results["off"]) == supervisorCurrentArc(results["romantic"]) ||
		supervisorCurrentArc(results["romantic"]) == supervisorCurrentArc(results["action"]) ||
		supervisorCurrentArc(results["action"]) == supervisorCurrentArc(results["mature_soft"]) {
		t.Fatalf("guide mode arcs should diverge: off=%s romantic=%s action=%s mature=%s",
			supervisorCurrentArc(results["off"]),
			supervisorCurrentArc(results["romantic"]),
			supervisorCurrentArc(results["action"]),
			supervisorCurrentArc(results["mature_soft"]))
	}
}

func TestNarrativeStanceModesControlledReplayDiverges(t *testing.T) {
	type stanceCase struct {
		mode          string
		suffixNeedle  string
		expectedBeats any
		expectedArc   string
		expectedGoal  string
	}
	cases := []stanceCase{
		{mode: "reactive", suffixNeedle: "Story Initiative - Reactive", expectedBeats: float64(0), expectedArc: "reactive_hold", expectedGoal: "hold the current beat and avoid new hooks"},
		{mode: "balanced", suffixNeedle: "Story Initiative - Balanced", expectedBeats: float64(1), expectedArc: "balanced_follow", expectedGoal: "advance one grounded beat"},
		{mode: "proactive", suffixNeedle: "Story Initiative - Proactive", expectedBeats: float64(1), expectedArc: "proactive_push", expectedGoal: "introduce a grounded follow-up hook"},
	}

	callByMode := map[string]int{}
	capturedPromptByMode := map[string]string{}
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
		mode := "balanced"
		for _, candidate := range []string{"reactive", "balanced", "proactive"} {
			if strings.Contains(body, `"narrative_stance": "`+candidate+`"`) {
				mode = candidate
				break
			}
		}
		callByMode[mode]++
		capturedPromptByMode[mode] = body
		arc := map[string]string{
			"reactive":  "reactive_hold",
			"balanced":  "balanced_follow",
			"proactive": "proactive_push",
		}[mode]
		goal := map[string]string{
			"reactive":  "hold the current beat and avoid new hooks",
			"balanced":  "advance one grounded beat",
			"proactive": "introduce a grounded follow-up hook",
		}[mode]
		content := `{"directive":{"story_author":{"current_arc":"` + arc + `","narrative_goal":"` + goal + `"},"director":{"pressure_level":"normal","required_outcomes":["` + goal + `"],"forbidden_moves":["stance-specific guard"]}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"stance-replay","choices":[{"message":{"content":` + strconv.Quote(content) + `}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	results := map[string]map[string]any{}
	for _, tc := range cases {
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
			"narrative_stance":"` + tc.mode + `",
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
		if pack["narrative_stance"] != tc.mode {
			t.Fatalf("%s pack narrative_stance = %v", tc.mode, pack["narrative_stance"])
		}
		suffix := extractionStringFromAny(pack["narrative_stance_suffix"])
		if !strings.Contains(suffix, tc.suffixNeedle) || !strings.Contains(capturedPromptByMode[tc.mode], tc.suffixNeedle) {
			t.Fatalf("%s prompt/suffix missing %q: suffix=%q prompt=%s", tc.mode, tc.suffixNeedle, suffix, capturedPromptByMode[tc.mode])
		}
		bounds, _ := pack["narrative_stance_bounds"].(map[string]any)
		if bounds["max_new_beats"] != tc.expectedBeats {
			t.Fatalf("%s max_new_beats = %v, want %v in bounds %+v", tc.mode, bounds["max_new_beats"], tc.expectedBeats, bounds)
		}
		trace := resp["trace_summary"].(map[string]any)
		if trace["narrative_stance"] != tc.mode || trace["narrative_stance_suffix_present"] != true || trace["narrative_stance_bounds_present"] != true {
			t.Fatalf("%s trace missing stance evidence: %+v", tc.mode, trace)
		}
		if arc := supervisorCurrentArc(resp); arc != tc.expectedArc {
			t.Fatalf("%s current_arc = %q, want %q", tc.mode, arc, tc.expectedArc)
		}
		director := supervisorDirector(resp)
		outcomes, _ := director["required_outcomes"].([]any)
		if len(outcomes) == 0 || !strings.Contains(extractionStringFromAny(outcomes[0]), tc.expectedGoal) {
			t.Fatalf("%s required_outcomes = %+v, want %q", tc.mode, outcomes, tc.expectedGoal)
		}
	}
	if len(callByMode) != len(cases) {
		t.Fatalf("runtime calls by stance = %+v, want all stances", callByMode)
	}
	if supervisorCurrentArc(results["reactive"]) == supervisorCurrentArc(results["balanced"]) ||
		supervisorCurrentArc(results["balanced"]) == supervisorCurrentArc(results["proactive"]) {
		t.Fatalf("narrative stance arcs should diverge: reactive=%s balanced=%s proactive=%s",
			supervisorCurrentArc(results["reactive"]),
			supervisorCurrentArc(results["balanced"]),
			supervisorCurrentArc(results["proactive"]))
	}
}

func anySliceContains(values []any, needle string) bool {
	for _, value := range values {
		if strings.Contains(extractionStringFromAny(value), needle) {
			return true
		}
	}
	return false
}

func supervisorCurrentArc(resp map[string]any) string {
	result, _ := resp["supervisor_result"].(map[string]any)
	directive, _ := result["directive"].(map[string]any)
	author, _ := directive["story_author"].(map[string]any)
	return extractionStringFromAny(author["current_arc"])
}

func supervisorDirector(resp map[string]any) map[string]any {
	result, _ := resp["supervisor_result"].(map[string]any)
	directive, _ := result["directive"].(map[string]any)
	director, _ := directive["director"].(map[string]any)
	return director
}

func TestConfigUpdateProjectGUISettingsTraceMasksSecrets(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)

	const mainKey = "sk-main-secret-seq02"
	const criticKey = "sk-critic-secret-seq02"
	const embeddingKey = "sk-embedding-secret-seq02"
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

	body := `{"chat_session_id":"sess-sv-fail","guide_mode":"strict","narrative_stance":"immersive","auto_advance_trigger":"none","wake_up_context":"hello","persistent_guidance":"be kind","context_messages":[{"role":"user","content":"move forward"}]}`
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
