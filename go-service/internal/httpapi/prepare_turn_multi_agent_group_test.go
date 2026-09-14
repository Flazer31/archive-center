package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func Test44GroupedMultiAgentRoundWireAndResults(t *testing.T) {
	for _, modelGroups := range [][]int{{0, 0, 0, 0, 0}, {0, 0, 0, 1, 1}, {0, 1, 2, 3, 4}} {
		t.Run(fmt.Sprint(modelGroups), func(t *testing.T) {
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			inputs := make([]map[string]any, len(multiAgentRoles))
			roles := make([]multiAgentRoleResult, len(multiAgentRoles))
			var mu sync.Mutex
			calls, inputChars := 0, 0
			seen := map[string]bool{}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
					MaxTokens           int64 `json:"max_tokens"`
					MaxCompletionTokens int64 `json:"max_completion_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Error(err)
					return
				}
				var packet map[string]any
				if len(wire.Messages) < 2 {
					t.Error("missing real request body")
					return
				}
				if err := json.Unmarshal([]byte(wire.Messages[1].Content), &packet); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				calls++
				inputChars += len([]rune(wire.Messages[1].Content))
				result := map[string]any{}
				members, grouped := packet["roles"].([]any)
				if !grouped {
					members = []any{map[string]any{"role": packet["role"], "input": packet}}
				}
				for _, member := range members {
					assignment := member.(map[string]any)
					role := assignment["role"].(string)
					if seen[role] {
						t.Errorf("duplicate role %s", role)
					}
					seen[role] = true
					expanded := map[string]any{}
					if shared, ok := packet["shared_input"].(map[string]any); ok {
						for k, v := range shared {
							expanded[k] = v
						}
					}
					for k, v := range assignment["input"].(map[string]any) {
						expanded[k] = v
					}
					for i, name := range multiAgentRoles {
						if name == role {
							var expected map[string]any
							_ = json.Unmarshal([]byte(multiAgentModelInput(inputs[i], 1)), &expected)
							if !reflect.DeepEqual(expanded, expected) {
								t.Errorf("%s lost source/reading fields", role)
							}
							if grouped && assignment["prompt"] != cfg.Roles[role].Prompt {
								t.Errorf("%s prompt overwritten", role)
							}
						}
					}
					result[role] = map[string]any{"selected_ids": []string{"id-" + role}, "reasons": map[string]string{"id-" + role: "chosen"}}
				}
				budget := wire.MaxCompletionTokens
				if budget == 0 {
					budget = wire.MaxTokens
				}
				if budget != int64(len(members))*1000 {
					t.Errorf("output budget=%d members=%d", budget, len(members))
				}
				var response any = map[string]any{"roles": result}
				if !grouped {
					response = result[packet["role"].(string)]
				}
				b, _ := json.Marshal(response)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 10}})
			}))
			defer provider.Close()
			for i, role := range multiAgentRoles {
				c := cfg.Roles[role]
				c.Enabled = true
				c.UsePublisher = false
				c.Provider = "openai"
				c.Endpoint = provider.URL
				c.APIKey = "fixture-key"
				c.Model = "fixture-" + string(rune('a'+modelGroups[i]))
				c.MaxTokens = 1000
				c.Prompt = "Keep assignment " + role
				cfg.Roles[role] = c
				roles[i] = multiAgentRoleResult{Role: role}
				inputs[i] = map[string]any{"role": role, "current_input": "A\n\nB", "recent_conversation": []map[string]any{{"Text": strings.Repeat("shared original scene ", 400)}}, "candidates": []map[string]any{{"id": "id-" + role, "ref": "F1", "text": "original " + role, "visibility": "private", "perspective_owner": role}}, "candidate_refs": map[string]string{"id-" + role: "F1"}}
			}
			server := &Server{}
			got := server.callMultiAgentRound(context.Background(), cfg, 1, roles, inputs, "fixture-session")
			groups := map[int]bool{}
			for _, n := range modelGroups {
				groups[n] = true
			}
			if calls != len(groups) || len(seen) != len(roles) {
				t.Fatalf("requests=%d seen=%v", calls, seen)
			}
			usageCount := 0
			plainChars := 0
			for i, call := range got {
				if call.Error != "" || !reflect.DeepEqual(call.Result.SelectedIDs, []string{"id-" + roles[i].Role}) {
					t.Errorf("role result: %+v", call)
				}
				if call.Usage != nil {
					usageCount++
				}
				plainChars += len([]rune(multiAgentModelInput(inputs[i], 1)))
			}
			if usageCount != calls {
				t.Errorf("usage duplicated: %d for %d requests", usageCount, calls)
			}
			if len(groups) < len(roles) && inputChars >= plainChars {
				t.Errorf("common input not reduced: %d >= %d", inputChars, plainChars)
			}
			t.Logf("physical_requests=%d model_input_chars=%d independent_model_input_chars=%d (fixture characters, not billed tokens)", calls, inputChars, plainChars)
		})
	}
}

func Test44GroupedConnectionOptionsStayIndependent(t *testing.T) {
	for _, name := range []string{"equal", "temperature", "timeout", "output_budget", "reasoning_effort", "reasoning_budget", "credential", "endpoint", "tier_alias", "tier_default", "effort_default"} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			count := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire map[string]any
				if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				count++
				mu.Unlock()
				messages := sliceFromAny(wire["messages"])
				var packet map[string]any
				if len(messages) != 2 {
					t.Error("messages")
					return
				}
				if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(messages[1]), "content")), &packet); err != nil {
					t.Error(err)
					return
				}
				var result any = map[string]any{"selected_ids": []string{}}
				if assignments, ok := packet["roles"]; ok {
					roles := map[string]any{}
					for _, raw := range sliceFromAny(assignments) {
						roles[stringFromMap(mapFromAny(raw), "role")] = result
					}
					result = map[string]any{"roles": roles}
				}
				b, _ := json.Marshal(result)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			roles := []multiAgentRoleResult{{Role: "event_recent"}, {Role: "character_objective"}}
			inputs := []map[string]any{{"role": roles[0].Role}, {"role": roles[1].Role}}
			for _, role := range roles {
				c := cfg.Roles[role.Role]
				c.Provider = "custom"
				c.Endpoint = provider.URL
				c.APIKey = "first"
				c.Model = "gpt-5"
				c.UsePublisher = false
				if strings.HasPrefix(name, "tier_") {
					c.Provider, c.LLMGatewayServiceTier = "llmgateway", "standard"
				}
				if name == "effort_default" {
					c.Provider, c.Model = "ollama", "deepseek-v4.1-flash:cloud"
				}
				cfg.Roles[role.Role] = c
			}
			c := cfg.Roles[roles[1].Role]
			switch name {
			case "temperature":
				c.Temperature += 0.1
			case "timeout":
				c.TimeoutMs += 1000
			case "output_budget":
				c.MaxTokens += 1000
			case "reasoning_effort":
				c.ReasoningEffort = "high"
			case "reasoning_budget":
				c.ReasoningBudgetTokens = int64Ptr(1024)
			case "credential":
				c.APIKey = "second"
			case "endpoint":
				c.Endpoint = provider.URL + "/other"
			case "tier_alias":
				c.LLMGatewayServiceTier = "default"
			case "tier_default":
				c.LLMGatewayServiceTier = ""
			case "effort_default":
				c.ReasoningEffort = "none"
			}
			cfg.Roles[roles[1].Role] = c
			calls := (&Server{}).callMultiAgentRound(context.Background(), cfg, 1, roles, inputs, "settings")
			want := 2
			if name == "equal" || name == "tier_alias" {
				want = 1
			}
			if count != want {
				t.Fatalf("option=%s physical=%d expected=%d", name, count, want)
			}
			for _, call := range calls {
				if call.Error != "" {
					t.Fatalf("option=%s: %s", name, call.Error)
				}
			}
		})
	}
}

func Test44GroupedRecommendationPartialRoleIsolation(t *testing.T) {
	raw := `{"roles":{"event_recent":{"selected_ids":["F1"]},"world_state":{"selected_ids":42,"unresolved":["world uncertain"]},"open_objectives":{"selected_ids":["F3"]}}}`
	parsed := parseMultiAgentGroupedResults(raw)
	for _, role := range []string{"event_recent", "open_objectives"} {
		call := finishMultiAgentCall(multiAgentCall{Round: 1, Raw: parsed[role]}, 200, nil, "")
		if call.Error != "" || len(call.Result.SelectedIDs) != 1 {
			t.Fatalf("valid role lost: %+v", call)
		}
	}
	broken := finishMultiAgentCall(multiAgentCall{Round: 1, Raw: parsed["world_state"]}, 200, nil, "")
	if broken.ResponseStatus != "partial" || len(broken.Result.Unresolved) != 1 {
		t.Fatalf("partial role lost: %+v", broken)
	}
	truncated := parseMultiAgentGroupedResults(`{"roles":{"event_recent":{"selected_ids":["F1"]},"world_state":{"selected_ids":["F2"`)
	if !strings.Contains(truncated["event_recent"], "F1") {
		t.Fatal("completed earlier role lost on truncated response")
	}
}

func Test44GroupedEmptyAndReasonReusePreserveRoleSemantics(t *testing.T) {
	previous := multiAgentRecommendation{SelectedIDs: []string{"F1"}, Reasons: map[string]string{"F1": "prior explanation"}}
	for _, raw := range []string{`{"selected_ids":["F1"],"reuse_previous_reasons":true}`, `{"selected_ids":[],"reuse_previous_reasons":true}`} {
		call := finishMultiAgentCall(multiAgentCall{Round: 2, Raw: raw, Input: map[string]any{"previous_result": previous}}, 200, nil, "")
		if call.Error != "" {
			t.Fatal(call.Error)
		}
		if len(call.Result.SelectedIDs) > 0 && call.Result.Reasons["F1"] != "prior explanation" {
			t.Fatal("prior explanation missing")
		}
		if len(call.Result.SelectedIDs) == 0 && len(call.Result.Reasons) != 0 {
			t.Fatal("empty final selection populated")
		}
	}
}

func Test44GroupedRoundsKeepIndividualSelectionsAndActualUsage(t *testing.T) {
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	facts := []prepareTurnPriorityMemoryCandidate{}
	for _, role := range multiAgentRoles {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: role + "-first", Lane: role, CompleteText: "original " + role})
	}
	secondRoles := map[string]bool{"event_recent": true, "character_objective": true, "world_state": true}
	requests, searches := 0, 0
	var mu sync.Mutex
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wire map[string]any
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			t.Error(err)
			return
		}
		messages := sliceFromAny(wire["messages"])
		var packet map[string]any
		if len(messages) != 2 {
			t.Error("wire messages")
			return
		}
		if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(messages[1]), "content")), &packet); err != nil {
			t.Error(err)
			return
		}
		mu.Lock()
		requests++
		round := requests
		mu.Unlock()
		assignments := sliceFromAny(packet["roles"])
		expected := len(multiAgentRoles)
		if round == 2 {
			expected = len(secondRoles)
		}
		if len(assignments) != expected || round > 2 {
			t.Errorf("round=%d members=%d", round, len(assignments))
		}
		results := map[string]any{}
		for _, raw := range assignments {
			assignment := mapFromAny(raw)
			role := stringFromMap(assignment, "role")
			if round == 2 && !secondRoles[role] {
				t.Errorf("unsolicited supplemental role %s", role)
			}
			input := map[string]any{}
			for k, v := range mapFromAny(packet["shared_input"]) {
				input[k] = v
			}
			for k, v := range mapFromAny(assignment["input"]) {
				input[k] = v
			}
			if round == 2 {
				if _, ok := input["previous_result"]; !ok {
					t.Error("first result missing")
				}
			}
			if round == 2 && role == "world_state" {
				continue
			} // Missing member keeps this role's first result only.
			answer := map[string]any{"selected_ids": []string{role + "-first"}, "reasons": map[string]string{role + "-first": "source"}}
			if round == 1 && secondRoles[role] {
				answer["search_requests"] = []string{role}
			}
			if round == 2 && role == "event_recent" {
				answer["selected_ids"] = []string{}
			}
			if round == 2 && role == "character_objective" {
				found := false
				for _, candidate := range sliceFromAny(input["candidates"]) {
					if stringFromMap(mapFromAny(candidate), "text") == "new character evidence" {
						found = true
					}
				}
				if !found {
					t.Error("supplemental evidence never reached second wire packet")
				}
				answer["selected_ids"] = []string{"character_objective-found"}
			}
			results[role] = answer
		}
		b, _ := json.Marshal(map[string]any{"roles": results})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}, "usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20}})
	}))
	defer provider.Close()
	for role, c := range cfg.Roles {
		c.Enabled = true
		c.UsePublisher = false
		c.Provider = "custom"
		c.Endpoint = provider.URL
		c.Model = "same"
		c.APIKey = "fixture"
		cfg.Roles[role] = c
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{ChatSessionID: "grouped"}, facts, nil, 32000, 8, nil, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		mu.Lock()
		searches++
		mu.Unlock()
		if !secondRoles[q] {
			t.Errorf("unexpected search %q", q)
		}
		return []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "character_objective-found", Lane: "character_objective", CompleteText: "new character evidence"}}, nil, map[string]any{"status": "ok"}
	})
	if requests != 2 || result.AnalysisCalls != requests || result.AnalysisAttempts != requests || searches != len(secondRoles) {
		t.Fatalf("physical=%d analysis=%d attempts=%d search=%d", requests, result.AnalysisCalls, result.AnalysisAttempts, searches)
	}
	usage := 0
	for _, role := range result.Roles {
		want := 1
		if secondRoles[role.Role] {
			want++
		}
		if len(role.Calls) != want {
			t.Errorf("%s rounds=%d", role.Role, len(role.Calls))
		}
		for _, call := range role.Calls {
			if call.Usage != nil {
				usage++
			}
		}
	}
	if usage != requests {
		t.Fatalf("duplicated usage: %d/%d", usage, requests)
	}
	if len(result.role("event_recent").Selection.SelectedIDs) != 0 || result.role("event_recent").Source == "ai" {
		t.Fatal("successful empty final must use ordinary Go")
	}
	if !reflect.DeepEqual(result.role("character_objective").Selection.SelectedIDs, []string{"character_objective-found"}) {
		t.Fatal("final recommendation replaced")
	}
	world := result.role("world_state")
	if !reflect.DeepEqual(world.Selection.SelectedIDs, []string{"world_state-first"}) || world.Calls[1].Error == "" {
		t.Fatalf("failed member lost first result: %+v", world)
	}
}
