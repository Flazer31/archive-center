package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test43MultiAgentParallelRoundsAndPartialFailure(t *testing.T) {
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("timed-request", "timed-session", 1)
	ctx := context.WithValue(context.Background(), multiAgentHUDRequestKey{}, "timed-request")
	server := &Server{TurnWorkflows: ledger}
	var mu sync.Mutex
	firstCount := 0
	barrier := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		messages := payload["messages"].([]any)
		var input map[string]any
		if err := json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &input); err != nil {
			t.Error(err)
			return
		}
		role := input["role"].(string)
		snapshot, _ := ledger.snapshot("timed-request")
		running := false
		for _, item := range snapshot.Preprocessing {
			if item.Role == role {
				running = item.Calls[len(item.Calls)-1].Status == "running"
			}
		}
		if !running {
			t.Error("provider request has no running HUD call")
		}
		_, second := input["previous_result"]
		if !second {
			mu.Lock()
			firstCount++
			if firstCount == len(multiAgentRoles) {
				close(barrier)
			}
			mu.Unlock()
			select {
			case <-barrier:
			case <-time.After(3 * time.Second):
				t.Error("first analyses were serialized")
			}
		}
		if role == "subjective_relationship" || (second && role == "world_state") {
			http.Error(w, "provider unavailable", 503)
			return
		}
		result := multiAgentRecommendation{}
		switch role {
		case "event_recent":
			if second {
				found := false
				for _, raw := range input["candidates"].([]any) {
					if raw.(map[string]any)["id"] == "found-event" {
						found = true
					}
				}
				if !found {
					t.Error("supplemental canonical evidence was not sent")
				}
				result.SelectedIDs = []string{"found-event"}
				result.SearchRequests = []string{"must not trigger a third round"}
			} else {
				result.SearchRequests = []string{"missing event", "unexecuted extra query"}
			}
		case "world_state":
			result.SelectedIDs = []string{"world-first"}
			result.SearchRequests = []string{"world evidence"}
		case "character_objective":
			result.SelectedIDs = []string{"objective-low"}
		}
		b, _ := json.Marshal(result)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.UsePublisher = false
		c.Provider = "custom"
		c.Endpoint = provider.URL
		c.Model = "test"
		c.APIKey = "test-key"
		cfg.Roles[role] = c
	}
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "world-first", Lane: "world_state", CompleteText: "The gate is closed."}, {CanonicalFactID: "objective-low", Lane: "character_objective", CompleteText: "Mira carries a key."}}
	searches := 0
	result := server.runMultiAgent(ctx, cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, map[string]int{}, func(query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		mu.Lock()
		searches++
		mu.Unlock()
		return []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "found-event", Lane: "event_recent", CompleteText: "Mira received the key yesterday."}}, nil, map[string]any{"status": "ready"}
	})
	if result.AnalysisCalls != 7 || searches != 2 {
		t.Fatalf("calls=%d searches=%d roles=%+v", result.AnalysisCalls, searches, result.Roles)
	}
	snapshot, _ := ledger.snapshot("timed-request")
	if len(snapshot.Preprocessing) != 5 || len(snapshot.Stages) != 12 {
		t.Fatal("optional timing changed role/stage count")
	}
	for i, item := range snapshot.Preprocessing {
		if item.Role != multiAgentRoles[i] {
			t.Fatal("provider completion order changed HUD order")
		}
		calls := result.role(item.Role).Calls
		if len(calls) != len(item.Calls) {
			t.Fatal("HUD lost supplemental round")
		}
		var total int64
		for j, call := range calls {
			status := "succeeded"
			if call.Error != "" {
				status = "failed"
			}
			if call.ResponseStatus != "" {
				status = call.ResponseStatus
			}
			if item.Calls[j].DurationMS != call.DurationMs || item.Calls[j].Status != status {
				t.Fatal("HUD differs from real provider call trace")
			}
			total += call.DurationMs
		}
		if total != item.DurationMS {
			t.Fatal("role time did not sum its own calls")
		}
	}
	snapshot.Preprocessing[0].Calls[0].Status = "mutated"
	isolated, _ := ledger.snapshot("timed-request")
	if isolated.Preprocessing[0].Calls[0].Status == "mutated" {
		t.Fatal("HUD snapshot aliases the ledger")
	}
	ledger.begin("disabled-request", "disabled-session", 1)
	cfg.Enabled = false
	server.runMultiAgent(context.WithValue(ctx, multiAgentHUDRequestKey{}, "disabled-request"), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 5, nil, nil)
	off, _ := ledger.snapshot("disabled-request")
	encoded, _ := json.Marshal(off)
	if strings.Contains(string(encoded), `"preprocessing"`) {
		t.Fatal("disabled preprocessing appeared in HUD")
	}
	ledger.begin("one-role", "one-role-session", 1)
	cfg.Enabled = true
	for role, value := range cfg.Roles {
		value.Enabled = role == "event_recent"
		value.Model = ""
		cfg.Roles[role] = value
	}
	server.runMultiAgent(context.WithValue(ctx, multiAgentHUDRequestKey{}, "one-role"), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 5, nil, nil)
	oneRole, _ := ledger.snapshot("one-role")
	if len(oneRole.Preprocessing) != 1 || oneRole.Preprocessing[0].Role != "event_recent" || oneRole.Preprocessing[0].Calls[0].Status != "failed" {
		t.Fatal("disabled roles appeared or a missing model looked successful")
	}
	if !reflect.DeepEqual(result.role("event_recent").Selection.SelectedIDs, []string{"found-event"}) {
		t.Fatal("supplemental recommendation lost")
	}
	world := result.role("world_state")
	if world.Source != "ai" || !reflect.DeepEqual(world.Selection.SelectedIDs, []string{"world-first"}) || len(world.Unresolved) == 0 {
		t.Fatalf("first selection not retained on failure: %+v", world)
	}
	if result.role("subjective_relationship").Source != "go_default" || result.role("subjective_relationship").Reason != "call_failed_without_recommendation" {
		t.Fatal("missing AI selection did not retain Go")
	}
	if len(result.role("event_recent").Unresolved) != 2 {
		t.Fatalf("unexecuted searches not reported: %+v", result.role("event_recent").Unresolved)
	}
	cfg.Enabled = false
	if got := server.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		t.Fatal("OFF performed search")
		return nil, nil, nil
	}); got != nil {
		t.Fatal("OFF returned active execution")
	}
}

func Test43MultiAgentSelectionReachesDeliveryWithoutScoreOrBudgetReplacement(t *testing.T) {
	out := prepareTurnInjectionAssembly{CharacterObjectiveText: "[Character Objective States]\n- Mira guards the archive.\n- Rook carries an old compass.", CanonWorldText: "[Item, Location, and World States]\n- The archive door is sealed."}
	perspective := priorityMemoryTestContext(1)
	perspective["_priority_memory_query"] = "Mira guards the archive"
	baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 2000, 1, "auto", nil, perspective)
	facts, summaries := multiAgentCandidatePool(&out, perspective)
	var selected []string
	for i := len(facts) - 1; i >= 0; i-- {
		if facts[i].Lane == "character_objective" {
			selected = append(selected, facts[i].CanonicalFactID)
		}
	}
	if len(selected) < 2 {
		t.Fatalf("fixture lacks competing canonical candidates: %+v", facts)
	}
	selection := &multiAgentSelection{Contract: multiAgentContract, Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "character_objective", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: selected}}}}
	selection.captureBaseline(baseline)
	out.Preprocessing = selection
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 1, 1, "auto", nil, perspective)
	got := []string{}
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if item["selection_reason"] == "ai_recommendation" {
			got = append(got, extractionStringFromAny(item["canonical_fact_id"]))
		}
	}
	if !reflect.DeepEqual(got, selected) {
		t.Fatalf("AI order/selection changed: got=%v want=%v", got, selected)
	}
	text := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(text, "old compass") || !strings.Contains(text, "Mira guards") || !strings.Contains(text, "door is sealed") {
		t.Fatalf("AI or Go default memory disappeared: %s", text)
	}
	if intFromAny(plan["budget_overrun_chars"], 0) == 0 {
		t.Fatal("overrun was hidden")
	}
	lane := prepareTurnPayloadLane("long_term_memory", "Long-term Memory Context", text, 1, true, nil)
	if lane["text"] != text {
		t.Fatalf("payload changed AI memory: %+v", lane)
	}
	selection.Roles[0].Source = "go_default"
	plan = buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 2000, 1, "auto", nil, perspective)
	if plan["final_text"] != baseline["final_text"] {
		t.Fatalf("all-empty changed ordinary Go delivery: %q vs %q", plan["final_text"], baseline["final_text"])
	}
}

func Test43MultiAgentPromptPersistenceAndAPIKeyEditing(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := &Server{}
	cfg := defaultMultiAgentSettings()
	for role, value := range cfg.Roles {
		if value.UsePublisher {
			t.Fatalf("new role %s silently inherits the Publisher connection", role)
		}
	}
	cfg.SharedPrompt = "Custom common instructions. Preserve this exact text."
	r := cfg.Roles["subjective_relationship"]
	r.UsePublisher = true // A previously saved explicit sharing choice remains supported.
	r.Prompt = "Keep the character's secrets and distinguish knowledge."
	r.APIKey = "sensitive-test-key"
	r.LLMGatewayServiceTier, r.VertexFlexMode = "flex", "flex_only"
	cfg.Roles["subjective_relationship"] = r
	put := func(c multiAgentSettings) map[string]any {
		t.Helper()
		b, _ := json.Marshal(c)
		rec := httptest.NewRecorder()
		s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(b)))
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		var v map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
		if mapFromAny(mapFromAny(mapFromAny(v["settings"])["roles"])["subjective_relationship"])["api_key"] != c.Roles["subjective_relationship"].APIKey {
			t.Fatal("saved API key was not returned to the password editor")
		}
		return v
	}
	view := put(cfg)
	if view["shared_prompt"] != cfg.SharedPrompt || view["default_shared_prompt"] != multiAgentSharedPrompt {
		t.Fatal("config view did not separate the applied shared prompt from its default")
	}
	fresh := &Server{}
	loaded, err := fresh.loadMultiAgentSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.SharedPrompt != cfg.SharedPrompt || !loaded.Roles["subjective_relationship"].UsePublisher || loaded.Roles["subjective_relationship"].Prompt != r.Prompt || loaded.Roles["subjective_relationship"].APIKey != r.APIKey {
		t.Fatal("restart lost config or editing enabled feature")
	}
	if loaded.Roles["subjective_relationship"].LLMGatewayServiceTier != "flex" || loaded.Roles["subjective_relationship"].VertexFlexMode != "flex_only" {
		t.Fatal("restart lost independently configured processing modes")
	}
	get := httptest.NewRecorder()
	fresh.handleMultiAgentSettings(get, httptest.NewRequest(http.MethodGet, "/config/memory-preprocessing", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), r.APIKey) || get.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("reopening settings did not return the saved key without caching")
	}
	// Older settings clients omit the new field; their saves must preserve the user's edit.
	oldBody, _ := json.Marshal(cfg)
	var oldPayload map[string]any
	_ = json.Unmarshal(oldBody, &oldPayload)
	delete(oldPayload, "shared_prompt")
	delete(mapFromAny(mapFromAny(oldPayload["roles"])["subjective_relationship"]), "api_key")
	oldBody, _ = json.Marshal(oldPayload)
	rec := httptest.NewRecorder()
	s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(oldBody)))
	loaded, err = fresh.loadMultiAgentSettings()
	if rec.Code != 200 || err != nil || loaded.SharedPrompt != cfg.SharedPrompt || loaded.Roles["subjective_relationship"].APIKey != r.APIKey {
		t.Fatal("omitted common prompt or key erased the stored value")
	}
	r.Prompt = ""
	cfg.SharedPrompt = ""
	cfg.Roles["subjective_relationship"] = r
	view = put(cfg)
	if view["shared_prompt"] != multiAgentSharedPrompt {
		t.Fatal("shared prompt default restore was not reflected in the config view")
	}
	loaded, err = fresh.loadMultiAgentSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SharedPrompt != "" || loaded.Roles["subjective_relationship"].APIKey == "" || loaded.Roles["subjective_relationship"].Prompt != "" {
		t.Fatal("default restore lost stored key")
	}
	r.APIKey = "replaced-fixture-key"
	cfg.Roles["subjective_relationship"] = r
	put(cfg)
	loaded, err = fresh.loadMultiAgentSettings()
	if err != nil || loaded.Roles["subjective_relationship"].APIKey != r.APIKey {
		t.Fatal("editing the password field did not replace the stored API key")
	}
	r.APIKey = ""
	cfg.Roles["subjective_relationship"] = r
	put(cfg)
	loaded, err = fresh.loadMultiAgentSettings()
	if err != nil || loaded.Roles["subjective_relationship"].APIKey != "" {
		t.Fatal("explicitly clearing the password field did not clear the saved key")
	}
	for _, role := range multiAgentRoles {
		if strings.TrimSpace(multiAgentRolePrompts[role]) == "" {
			t.Fatal("role lacks actual default prompt")
		}
	}
}

func Test43MultiAgentIndependentConnectionsAndSharedPromptReachBothRounds(t *testing.T) {
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	cfg.SharedPrompt = "Edited common prompt: use the user's exact shared instructions."
	expectedShared := cfg.SharedPrompt
	tiers := map[string]string{"event_recent": "flex", "character_objective": "priority", "subjective_relationship": "", "world_state": "flex", "unresolved_goal": ""}
	var mu sync.Mutex
	counts := map[string]int{}
	for _, role := range multiAgentRoles {
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			messages := body["messages"].([]any)
			var input map[string]any
			if err := json.Unmarshal([]byte(messages[1].(map[string]any)["content"].(string)), &input); err != nil {
				t.Error(err)
				return
			}
			round := 1
			if _, second := input["previous_result"]; second {
				round = 2
			}
			prompt := messages[0].(map[string]any)["content"].(string)
			rolePrompt := cfg.Roles[role].Prompt
			if rolePrompt == "" {
				rolePrompt = multiAgentRolePrompts[role]
			}
			want := expectedShared + "\n\nAssigned role: " + role + "\n" + rolePrompt + fmt.Sprintf("\nRound %d of at most 2.", round)
			if input["role"] != role || body["model"] != "model-"+role || r.Header.Get("Authorization") != "Bearer key-"+role {
				t.Errorf("role %s used another connection or model", role)
			}
			if prompt != want {
				t.Errorf("role %s round %d did not receive the exact edited shared and role prompts", role, round)
			}
			if extractionStringFromAny(body["service_tier"]) != tiers[role] {
				t.Errorf("role %s round %d received service tier %v, want %q", role, round, body["service_tier"], tiers[role])
			}
			mu.Lock()
			counts[role]++
			mu.Unlock()
			answer := multiAgentRecommendation{}
			if round == 1 {
				answer.SearchRequests = []string{"supplemental evidence for " + role}
			}
			content, _ := json.Marshal(answer)
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
		}))
		t.Cleanup(provider.Close)
		value := cfg.Roles[role]
		value.Provider, value.Endpoint, value.Model, value.APIKey = "custom", provider.URL, "model-"+role, "key-"+role
		value.Prompt = "Only this specialist: " + role
		value.LLMGatewayServiceTier = tiers[role]
		cfg.Roles[role] = value
	}
	s := &Server{}
	result := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 5, nil,
		func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
			return nil, nil, map[string]any{"status": "no_matches"}
		})
	if result.AnalysisCalls != 2*len(multiAgentRoles) {
		t.Fatalf("expected two independent calls per role, got %d", result.AnalysisCalls)
	}
	for _, role := range multiAgentRoles {
		if counts[role] != 2 {
			t.Fatalf("role %s reached its provider %d times", role, counts[role])
		}
		for _, call := range result.role(role).Calls {
			if call.Error != "" || call.Model != "model-"+role || !strings.HasPrefix(call.Prompt, cfg.SharedPrompt+"\n\n") {
				t.Fatalf("applied prompt/model trace mismatch for %s: %+v", role, call)
			}
		}
	}
	cfg.SharedPrompt = ""
	expectedShared = multiAgentSharedPrompt
	for _, role := range multiAgentRoles {
		saved := cfg.Roles[role]
		value := saved
		value.Prompt = ""
		cfg.Roles[role] = value
		for round := 1; round <= 2; round++ {
			input := map[string]any{"role": role}
			if round == 2 {
				input["previous_result"] = multiAgentRecommendation{}
			}
			call := s.callMultiAgent(context.Background(), role, cfg, round, input)
			if call.Error != "" || !strings.Contains(call.Prompt, multiAgentRolePrompts[role]) {
				t.Fatalf("restored role %s did not receive its bundled prompt in round %d", role, round)
			}
		}
		cfg.Roles[role] = saved
	}
	role := multiAgentRoles[0]
	call := s.callMultiAgent(context.Background(), role, cfg, 1, map[string]any{"role": role})
	if call.Error != "" || !strings.HasPrefix(call.Prompt, multiAgentSharedPrompt+"\n\n") {
		t.Fatal("blank override did not restore the bundled common prompt in the actual provider call")
	}
	value := cfg.Roles[role]
	s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint = value.Provider, value.Endpoint
	s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = value.Model, value.APIKey
	s.RuntimeConfig.SupervisorLLMGatewayServiceTier = "priority"
	tiers[role] = "priority" // Sharing follows the Publisher, not the saved individual Flex value.
	value.UsePublisher, value.Model, value.APIKey = true, "unused-individual-model", "unused-individual-key"
	cfg.Roles[role] = value
	call = s.callMultiAgent(context.Background(), role, cfg, 1, map[string]any{"role": role})
	if call.Error != "" || call.Model != "model-"+role {
		t.Fatal("explicit Publisher connection sharing did not preserve the independent role prompt")
	}
}

func Test43MultiAgentFlexProviderTransportAndGenerationSettings(t *testing.T) {
	credential := testVertexServiceAccountJSON(t)
	for _, tc := range []struct {
		name, provider, tier, vertexMode, wantTier, wantSharedHeader, wantRequestHeader string
	}{
		{name: "OpenAI Flex", provider: "openai", tier: "flex", wantTier: "flex"},
		{name: "Gateway Flex", provider: "llmgateway", tier: "flex", wantTier: "flex"},
		{name: "NeuralWatt Flex stream", provider: "neuralwatt", tier: "flex", wantTier: "flex"},
		{name: "AI Studio Flex", provider: "gemini", tier: "flex", wantTier: "flex"},
		{name: "AI Studio Standard", provider: "gemini", tier: "standard", wantTier: "standard"},
		{name: "AI Studio default", provider: "gemini"},
		{name: "Vertex Flex only", provider: "vertex", tier: "flex", vertexMode: "flex_only", wantSharedHeader: "flex", wantRequestHeader: "shared"},
		{name: "Vertex provisioned then Flex", provider: "vertex", vertexMode: "provisioned_then_flex", wantSharedHeader: "flex"},
		{name: "Vertex off", provider: "vertex", vertexMode: "off"},
		{name: "Claude ignores inactive individual tier", provider: "claude", tier: "flex"},
		{name: "OpenCode Go ignores inactive individual tier", provider: "opencode-go", tier: "flex"},
		{name: "OpenCode Go ignores inactive individual tier", provider: "opencode-go", tier: "flex"},
		{name: "OpenCode ignores inactive individual tier", provider: "opencode", tier: "flex"},
		{name: "OpenRouter ignores inactive individual tier", provider: "openrouter", tier: "flex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldClient := proxyHTTPClient
			t.Cleanup(func() { proxyHTTPClient = oldClient })
			calls := 0
			answer := `{"selected_ids":["supplied-id"]}`
			proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				response := func(body string) *http.Response {
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
				}
				if r.URL.String() == "https://oauth2.googleapis.com/token" {
					if tc.provider != "vertex" {
						t.Fatal("unexpected credential request")
					}
					return response(`{"access_token":"fixture-token","expires_in":3600}`), nil
				}
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				key := "service_tier"
				generation := body
				maxKey := "max_tokens"
				if tc.provider == "gemini" || tc.provider == "vertex" {
					key, generation, maxKey = "serviceTier", mapFromAny(body["generationConfig"]), "maxOutputTokens"
					if body["service_tier"] != nil {
						t.Fatal("OpenAI tier field leaked into Google request")
					}
				}
				if extractionStringFromAny(body[key]) != tc.wantTier {
					t.Fatalf("tier = %v, want %q", body[key], tc.wantTier)
				}
				if r.Header.Get("X-Vertex-AI-LLM-Shared-Request-Type") != tc.wantSharedHeader || r.Header.Get("X-Vertex-AI-LLM-Request-Type") != tc.wantRequestHeader {
					t.Fatal("wrong Vertex Flex headers")
				}
				if generation["temperature"] != 0.6 || generation[maxKey] != float64(3072) {
					t.Fatalf("generation controls not forwarded: %+v", generation)
				}
				var payload map[string]any
				switch tc.provider {
				case "gemini", "vertex":
					payload = map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": answer}}}}}}
				case "claude":
					payload = map[string]any{"content": []any{map[string]any{"type": "text", "text": answer}}}
				default:
					payload = map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}}
				}
				if tc.provider == "neuralwatt" {
					if body["stream"] != true {
						t.Fatal("Flex lost the existing streaming transport")
					}
					chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": answer}, "finish_reason": "stop"}}})
					resp := response("data: " + string(chunk) + "\n\ndata: [DONE]\n\n")
					resp.Header.Set("Content-Type", "text/event-stream")
					return resp, nil
				}
				encoded, _ := json.Marshal(payload)
				return response(string(encoded)), nil
			})}
			cfg := defaultMultiAgentSettings()
			role := multiAgentRoles[0]
			value := cfg.Roles[role]
			value.Provider, value.Model, value.APIKey = tc.provider, "test-model", "fixture-key"
			value.LLMGatewayServiceTier, value.VertexFlexMode = tc.tier, tc.vertexMode
			value.Temperature, value.MaxTokens = 0.6, 3072
			if tc.provider == "vertex" {
				value.APIKey = credential
				value.Endpoint = "https://aiplatform.googleapis.com/v1/projects/PROJECT_ID/locations/global/publishers/google/models"
			}
			cfg.Roles[role] = value
			s := &Server{}
			for round := 1; round <= 2; round++ {
				call := s.callMultiAgent(context.Background(), role, cfg, round, map[string]any{"role": role})
				if call.Error != "" || !reflect.DeepEqual(call.Result.SelectedIDs, []string{"supplied-id"}) {
					t.Fatalf("round %d: %+v", round, call)
				}
			}
			if calls != 2 {
				t.Fatalf("provider calls = %d; expected one per round", calls)
			}
			s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint = value.Provider, value.Endpoint
			s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = value.Model, value.APIKey
			s.RuntimeConfig.SupervisorVertexFlexMode = value.VertexFlexMode
			if proxyProviderSupportsServiceTier(value.Provider) {
				s.RuntimeConfig.SupervisorLLMGatewayServiceTier = value.LLMGatewayServiceTier
			}
			value.UsePublisher = true
			value.LLMGatewayServiceTier, value.VertexFlexMode = "priority", "off"
			cfg.Roles[role] = value
			call := s.callMultiAgent(context.Background(), role, cfg, 1, map[string]any{"role": role})
			if call.Error != "" || calls != 3 {
				t.Fatalf("shared Publisher processing mode was not applied once: %+v", call)
			}
		})
	}
}

func Test43MultiAgentPartialJSONKeepsReceivedSelection(t *testing.T) {
	r, err := parseMultiAgentRecommendation("```json\n{\"selected_ids\":[\"known-memory\"],\"reasons\":")
	if err == nil || !reflect.DeepEqual(r.SelectedIDs, []string{"known-memory"}) {
		t.Fatalf("partial recommendation erased: %+v %v", r, err)
	}
	r, err = parseMultiAgentRecommendation(`{"selected_ids":["known-memory","unfinished`)
	if err == nil || !reflect.DeepEqual(r.SelectedIDs, []string{"known-memory"}) {
		t.Fatal("an unfinished ID erased the already received ID")
	}
	if multiAgentHasSelection(multiAgentRecommendation{SelectedIDs: []string{"", " "}}) {
		t.Fatal("blank IDs manufactured an AI recommendation")
	}
}

func Test43MultiAgentFieldRecoveryKeepsLaterRecommendations(t *testing.T) {
	r, err := parseMultiAgentRecommendation(`{"selected_ids":["F2","F1"],"search_requests":"where is the key?","related_requests":[{"role":"world_state","refs":["F2"],"reason":"handover"}],"unresolved":"arrival time"}`)
	if err != nil || !reflect.DeepEqual(r.SelectedIDs, []string{"F2", "F1"}) || !reflect.DeepEqual(r.SearchRequests, []string{"where is the key?"}) || len(r.RelatedRequests) != 1 || !reflect.DeepEqual(r.Unresolved, []string{"arrival time"}) {
		t.Fatalf("single strings blocked later fields: %+v %v", r, err)
	}
	r, err = parseMultiAgentRecommendation(`{"selected_ids":[],"search_requests":["where is the key?","related_requests":[{"role":"world_state","refs":[],"reason":"handover"}],"unresolved":["arrival time"]}`)
	if err != nil || !reflect.DeepEqual(r.SearchRequests, []string{"where is the key?"}) || len(r.RelatedRequests) != 1 || !reflect.DeepEqual(r.Unresolved, []string{"arrival time"}) {
		t.Fatalf("missing array closer lost a usable search: %+v %v", r, err)
	}
	r, err = parseMultiAgentRecommendation(`{"reasons":"wrong field type","selected_ids":["F2","F1"],"search_requests":["handover?"],"unresolved":["date"]}`)
	if err == nil || !reflect.DeepEqual(r.SelectedIDs, []string{"F2", "F1"}) || len(r.SearchRequests) != 1 || len(r.Unresolved) != 1 {
		t.Fatalf("one invalid field erased later results: %+v %v", r, err)
	}
}

func Test43MultiAgentRecoveryUsesExistingRoundsAndReportsOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, first, second, callStatus, source string
	}{
		{"string_list", `{"selected_ids":"F1"}`, "", "repaired", "ai"},
		{"later_selection", `{"reasons":false,"selected_ids":["F1"]}`, "", "partial", "ai"},
		{"empty", `{"selected_ids":[]}`, "", "no_recommendation", "go_default"},
		{"syntax_search", `{"selected_ids":[],"search_requests":["where is the key?","unresolved":["date"]}`, `{"selected_ids":["F1"]}`, "repaired", "ai"},
		{"object_questions", `{"selected_ids":["F1"],"search_requests":[{"question":"where is the key?"}]}`, `{"selected_ids":["F1"],"search_requests":[{"query":"who has the key?"}]}`, "repaired", "ai"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, searches := 0, 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				answer := tc.first
				if calls == 2 {
					answer = tc.second
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled, c.UsePublisher = role == "event_recent", false
				c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "test", "key"
				cfg.Roles[role] = c
			}
			ledger := newTurnWorkflowHUDLedger()
			ledger.begin("recovery-request", "recovery-session", 1)
			s := &Server{TurnWorkflows: ledger}
			ctx := context.WithValue(context.Background(), multiAgentHUDRequestKey{}, "recovery-request")
			facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "fact-original", Lane: "event_recent", CompleteText: "The key arrived yesterday."}}
			result := s.runMultiAgent(ctx, cfg, dto.PrepareTurnRequest{}, facts, nil, 1000, 3, map[string]int{}, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				searches++
				if q != "where is the key?" {
					t.Errorf("search question changed: %q", q)
				}
				return facts, nil, map[string]any{"status": "ok"}
			})
			wantCalls, wantSearches := 1, 0
			if tc.second != "" {
				wantCalls, wantSearches = 2, 1
			}
			view, _ := ledger.snapshot("recovery-request")
			if calls != wantCalls || searches != wantSearches || result.AnalysisCalls != wantCalls {
				t.Fatalf("format recovery added/lost calls: calls=%d searches=%d", calls, searches)
			}
			if result.Roles[0].Source != tc.source || view.Preprocessing[0].SelectionSource != tc.source || view.Preprocessing[0].Calls[0].Status != tc.callStatus {
				t.Fatalf("wrong actual result/HUD: %+v %+v", result.Roles[0], view.Preprocessing)
			}
			if tc.source == "ai" && !reflect.DeepEqual(result.Roles[0].Selection.SelectedIDs, []string{facts[0].CanonicalFactID}) {
				t.Fatalf("original selection changed: %+v", result.Roles[0].Selection)
			}
		})
	}
}

func Test43MultiAgentSettingsRegisteredOnProductionRouter(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := NewServer(config.Default())
	mux := http.NewServeMux()
	s.registerConfigRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config/memory-preprocessing", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), multiAgentContract) {
		t.Fatalf("route unavailable: %d %s", rec.Code, rec.Body.String())
	}
}

func Test43MultiAgentHTTPPrepareDeliversSelectedCanonicalMemoryAndPublisherSupport(t *testing.T) {
	for _, publisherMode := range []string{"disabled", "success", "failure"} {
		t.Run(publisherMode, func(t *testing.T) {
			t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
			var chosenID, chosenText string
			var chosenCandidate map[string]any
			handoffObserved := false
			publisherCalls := 0
			var embedded []string
			const question = "Who delivered the compass before the archive scene?"
			const handoffReason = "Check the recorded operating condition of this acquired compass."
			const interpretation = "The acquired compass can support this scene's navigation."
			const uncertainty = "Its calibration remains uncertain."
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if raw, ok := body["input"]; ok {
					encoded, _ := json.Marshal(raw)
					embedded = append(embedded, string(encoded))
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{0.2, 0.1}, "index": 0}}, "model": "test-embedding"})
					return
				}
				messages := body["messages"].([]any)
				var input map[string]any
				if err := json.Unmarshal([]byte(mapFromAny(messages[1])["content"].(string)), &input); err != nil {
					t.Error(err)
					return
				}
				if packet, publisher := input["supervisor_support_packet"]; publisher {
					publisherCalls++
					notes := outputFidelityLineageSlice(mapFromAny(packet)["delivered_preprocessing_notes"])
					encoded, _ := json.Marshal(notes)
					if !strings.Contains(string(encoded), interpretation) || !strings.Contains(string(encoded), uncertainty) {
						t.Error("actual Publisher request lost specialist interpretations")
					}
					catalog := mapFromAny(mapFromAny(packet)["preprocessing_source_catalog"])
					for _, raw := range notes {
						item := mapFromAny(raw)
						for _, ref := range stringsFromAny(item["scope_refs"]) {
							if len(mapFromAny(catalog[ref])) == 0 {
								t.Error("Publisher lost the interpretation's source scope")
							}
						}
						if item["kind"] == "selection_reason" && extractionStringFromAny(item["evidence_ref"]) == "" {
							t.Error("Publisher lost the individual evidence reference")
						}
					}
					if publisherMode == "failure" {
						http.Error(w, "fixture Publisher failure", http.StatusBadRequest)
						return
					}
					refs := stringsFromAny(mapFromAny(notes[0])["source_refs"])
					_, _ = w.Write([]byte(publisherV3OpenAIResponse(refs[0], "Develop the compass scene at the user's chosen pace.")))
					return
				}
				if input["role"] == "world_state" {
					if _, second := input["previous_result"]; second {
						related := outputFidelityLineageSlice(input["related_evidence"])
						if len(related) != 1 {
							t.Errorf("production public projection did not reach the second specialist request: %+v", related)
						} else {
							item := modelEvidenceForTest(t, input, related[0])
							if item["request_reason"] != handoffReason || item["from_role"] != "event_recent" {
								t.Error("actual recipient request lost the originating editor's purpose")
							}
							for _, key := range []string{"id", "ref", "text", "source_ref", "source_table", "source_turn"} {
								if item[key] != chosenCandidate[key] {
									t.Errorf("public handoff changed %s: got=%v want=%v", key, item[key], chosenCandidate[key])
								}
							}
							handoffObserved = true
						}
						if len(input["candidates"].([]any)) != 0 {
							t.Error("cross-role evidence was relabeled as world-state candidates")
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{}`}}}})
					return
				}
				_, second := input["previous_result"]
				for _, raw := range input["candidates"].([]any) {
					c := modelEvidenceForTest(t, input, raw)
					if !second && strings.Contains(extractionStringFromAny(c["text"]), "compass") {
						chosenID = c["id"].(string)
						chosenText = c["text"].(string)
						chosenCandidate = c
						if c["visibility"] != "public_projection" {
							t.Errorf("fixture missed the production projection path: %v", c["visibility"])
						}
					}
				}
				if chosenID == "" {
					t.Error("canonical candidates were not passed before K selection")
				}
				recommendation := multiAgentRecommendation{SelectedIDs: []string{chosenID}, Reasons: map[string]string{chosenID: interpretation}, Unresolved: []string{uncertainty}}
				if _, second := input["previous_result"]; !second {
					recommendation.SearchRequests = []string{question}
					recommendation.RelatedRequests = []multiAgentRelatedRequest{{Role: "world_state", Refs: []string{chosenCandidate["ref"].(string)}, Reason: handoffReason}}
				}
				answer, _ := json.Marshal(recommendation)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(answer)}}}})
			}))
			defer provider.Close()
			cfg := config.Default()
			cfg.PromptDir = filepath.Join("..", "..", "..", "prompts")
			cfg.StoreMode = config.StoreModeDualShadow
			cfg.ChromaEndpoint = "http://127.0.0.1:8000"
			cfg.Readiness.ChromaConfigured = true
			s := NewServer(cfg)
			s.Vector = &priorityPreciseCandidateVector{}
			s.RuntimeConfig.EmbeddingProvider, s.RuntimeConfig.EmbeddingEndpoint, s.RuntimeConfig.EmbeddingModel, s.RuntimeConfig.EmbeddingAPIKey = "custom", provider.URL, "test-embedding", "test-key"
			s.RuntimeConfig.EmbeddingTimeoutSec = 30
			s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "test-publisher", "test-key"
			s.RuntimeConfig.SupervisorTimeoutSec = 30
			s.Store = &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{
				{ID: 701, ChatSessionID: "multi-http", TurnIndex: 1, Importance: 10, SummaryJSON: `{"narrative_events":[{"event":"Mira guarded the archive gate.","visibility":"public"}]}`},
				{ID: 702, ChatSessionID: "multi-http", TurnIndex: 2, Importance: 1, SummaryJSON: `{"narrative_events":[{"event":"At the archive gate, Rook acquired an old compass."}]}`},
			}}}
			settings := defaultMultiAgentSettings()
			settings.Enabled = true
			for role, value := range settings.Roles {
				value.Enabled = role == "event_recent" || role == "world_state"
				value.UsePublisher = false
				value.Provider = "custom"
				value.Endpoint = provider.URL
				value.APIKey = "test-key"
				value.Model = "test"
				settings.Roles[role] = value
			}
			b, _ := json.Marshal(settings)
			rec := httptest.NewRecorder()
			s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(b)))
			if rec.Code != 200 {
				t.Fatal(rec.Body.String())
			}
			mux := http.NewServeMux()
			s.RegisterRoutes(mux)
			body := map[string]any{"chat_session_id": "multi-http", "turn_index": 3, "raw_user_input": "Mira checks the archive gate.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 6000, "core_objective_memory_max_items": 1, "supervisor_enabled": publisherMode != "disabled"}}
			mapFromAny(body["settings"])["guide_mode"] = "standard"
			mapFromAny(body["settings"])["guide_strength"] = "strong"
			body["client_meta"] = map[string]any{"chroma_query_vector": []float64{0.1, 0.2}}
			b, _ = json.Marshal(body)
			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(b)))
			if rec.Code != 200 {
				t.Fatalf("prepare failed: %d %s", rec.Code, rec.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			pack := mapFromAny(response["injection_pack"])
			plan := mapFromAny(pack["memory_delivery_plan"])
			if !handoffObserved {
				t.Error("registered prepare-turn route never delivered the public handoff")
			}
			preprocessing := mapFromAny(plan["preprocessing"])
			if intFromAny(preprocessing["analysis_calls"], 0) != 2*2 {
				t.Errorf("expected two existing rounds for two active roles: %+v", preprocessing["analysis_calls"])
			}
			for _, raw := range outputFidelityLineageSlice(preprocessing["roles"]) {
				if strings.Contains(strings.Join(stringsFromAny(mapFromAny(raw)["unresolved"]), "\n"), "related_reference_not_public: "+chosenID) {
					t.Error("production public projection is still diagnosed as non-public")
				}
			}
			if chosenText == "" || !strings.Contains(extractionStringFromAny(plan["final_text"]), chosenText) {
				t.Fatalf("AI selection not delivered: chosen=%s plan=%+v", chosenText, plan)
			}
			if strings.Contains(extractionStringFromAny(plan["final_text"]), "Mira guarded") {
				t.Fatal("Go inserted its own event recommendation despite the AI selection")
			}
			if !strings.Contains(extractionStringFromAny(pack["injection_text"]), chosenText) {
				t.Fatal("selected memory did not reach actual injection pack")
			}
			payload, _ := json.Marshal(response["payload_application_plan"])
			if !strings.Contains(string(payload), chosenText) {
				t.Fatalf("selected memory not in Go payload plan: %s", payload)
			}
			support, _ := json.Marshal(mapFromAny(response["supervisor_input_pack"])["support_packet"])
			if !strings.Contains(string(support), chosenText) {
				t.Fatalf("Publisher support lost selected evidence: %s", support)
			}
			for _, note := range []string{interpretation, uncertainty} {
				if !strings.Contains(string(payload), note) || !strings.Contains(string(support), note) {
					t.Fatalf("specialist interpretation did not reach both main payload and Publisher support: %q", note)
				}
				if strings.Contains(extractionStringFromAny(plan["final_text"]), note) {
					t.Fatal("specialist interpretation was mixed into canonical memory text")
				}
			}
			if len(embedded) != 1 || !strings.Contains(embedded[0], question) {
				t.Fatalf("supplemental search reused the original input embedding: %v trace=%v", embedded, mapFromAny(plan["preprocessing"])["searches"])
			}
			wantCalls := 1
			if publisherMode == "disabled" {
				wantCalls = 0
			}
			if publisherCalls != wantCalls {
				t.Fatalf("Publisher calls=%d want=%d guidance=%v error=%v trace=%v", publisherCalls, wantCalls, mapFromAny(response["payload_application_plan"])["guidance_application_trace"], mapFromAny(response["supervisor_input_pack"])["llm_error"], mapFromAny(response["supervisor_input_pack"])["llm_trace"])
			}
			payloadPlan := mapFromAny(response["payload_application_plan"])
			publisherStatus := extractionStringFromAny(mapFromAny(payloadPlan["guidance_application_trace"])["supervisor_call_status"])
			wantStatus := map[string]string{"disabled": "disabled", "success": "applied", "failure": "failed_open"}[publisherMode]
			if publisherStatus != wantStatus {
				t.Fatalf("Publisher fixture did not exercise %s: status=%s", publisherMode, publisherStatus)
			}
			noteLane := outputFidelity36FFindLane(payloadPlan, "preprocessing_notes")
			if !boolFromAny(noteLane["applied"]) || !strings.Contains(extractionStringFromAny(noteLane["text"]), interpretation) {
				t.Fatal("independent specialist-notes lane was not applied")
			}
			budgetFound := false
			for _, raw := range outputFidelityLineageSlice(mapFromAny(payloadPlan["budget_ledger"])["lanes"]) {
				lane := mapFromAny(raw)
				if lane["key"] == "preprocessing_notes" {
					budgetFound = true
				}
				if lane["key"] == "preprocessing_notes" && (lane["budget_mode"] != "additional_observed" || intFromAny(lane["final_delivery_chars"], 0) != len([]rune(extractionStringFromAny(noteLane["text"])))) {
					t.Fatal("additional specialist text is missing from actual input accounting")
				}
			}
			if !budgetFound {
				t.Fatal("specialist-notes budget lane is missing")
			}
		})
	}
}

func TestOpenCodeGoPreprocessingSessionBothRounds(t *testing.T) {
	var mu sync.Mutex
	sessions := []string{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "ArchiveCenter/4.3.0" {
			t.Error("missing client identity")
		}
		mu.Lock()
		sessions = append(sessions, r.Header.Get("x-opencode-session"))
		mu.Unlock()
		var b map[string]any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Error(err)
			return
		}
		messages := sliceFromAny(b["messages"])
		content := stringFromMap(mapFromAny(messages[1]), "content")
		answer := `{"selected_ids":[]}`
		if !strings.Contains(content, "previous_result") {
			answer = `{"search_requests":["Find the missing evidence"]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": answer}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for _, role := range multiAgentRoles {
		c := cfg.Roles[role]
		c.Provider, c.Endpoint, c.Model, c.APIKey = "opencode-go", provider.URL+"/v1/chat/completions", "kimi-k2.7-code", "fixture-key"
		c.UsePublisher = false
		cfg.Roles[role] = c
	}
	srv := &Server{}
	for _, sid := range []string{"chat-A", "chat-B"} {
		result := srv.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{ChatSessionID: sid}, nil, nil, 2000, 5, nil,
			func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				return nil, nil, map[string]any{"status": "no_matches"}
			})
		if result.AnalysisCalls != len(multiAgentRoles)*2 {
			t.Fatalf("calls=%d", result.AnalysisCalls)
		}
	}
	n := len(multiAgentRoles) * 2
	if len(sessions) != n*2 || sessions[0] == "" {
		t.Fatalf("session headers=%v", sessions)
	}
	for _, got := range sessions[:n] {
		if got != sessions[0] {
			t.Fatal("first/second round changed session")
		}
	}
	for _, got := range sessions[n:] {
		if got == sessions[0] || got != sessions[n] {
			t.Fatal("new chat session not propagated")
		}
	}
}
