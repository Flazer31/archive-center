package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// Synthetic recorded role inputs only. The two facts deliberately share a
// source/scene but differ in knowledge: hearing the order does not reveal the plan.
func scopePrompt49Input(role string, round int) map[string]any {
	input := map[string]any{
		"role":                  role,
		"current_input":         "Mira and Vale stand together at the gate.",
		"recent_context_policy": "full_configured",
		"candidates": []map[string]any{
			{"id": "contingency", "ref": "F1", "source_ref": "memories:101", "source_turn": 7,
				"text":       "Mira privately planned to lock the inner gate if the alarm sounded.",
				"visibility": "owner_private", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira"},
				"knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Vale"}, "suspected_by": []string{"Ivo"}, "misinformed_by": []string{"Tess"}, "revealed_to": []string{}}},
			{"id": "instruction", "ref": "F2", "source_ref": "memories:101", "source_turn": 7,
				"text":       "Vale heard only the instruction to wait by the outer gate.",
				"visibility": "source_scoped", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira", "Vale"},
				"knowledge_scope": map[string]any{"known_by": []string{"Mira", "Vale"}, "unknown_to": []string{}, "suspected_by": []string{}, "misinformed_by": []string{}, "revealed_to": []string{"Vale"}}},
			{"id": "public_meeting", "ref": "F3", "source_ref": "memories:101", "source_turn": 7,
				"text": "Mira and Vale met at the gate.", "visibility": "public"},
			{"id": "scope_unrecorded", "ref": "F4", "source_ref": "memories:102", "source_turn": 8,
				"text": "The record supplies no named knowledge scope.", "visibility": "general"},
		},
		"selectable_refs":     map[string]any{"selected_ids": []string{"F1", "F2", "F3", "F4"}},
		"public_handoff_refs": []string{"F3"},
	}
	if round == 2 {
		input["previous_result"] = multiAgentRecommendation{SelectedIDs: []string{"F1", "F2"}}
	}
	return input
}

func scopePrompt49Settings(customShared, customRole bool) multiAgentSettings {
	cfg := defaultMultiAgentSettings()
	if customShared {
		cfg.SharedPrompt = "Custom common task: explain only useful scene evidence."
	}
	for _, role := range multiAgentRoles {
		value := cfg.Roles[role]
		value.Provider, value.Endpoint, value.Model, value.APIKey = "custom", "https://scope-prompt.invalid", "synthetic-model", "synthetic-key"
		if customRole {
			value.Prompt = "Custom assignment for " + role + ": keep source attribution."
		}
		cfg.Roles[role] = value
	}
	return cfg
}

func assertScopePrompt49Instruction(t *testing.T, prompt string) {
	t.Helper()
	// These checks cover the instruction contract, not model compliance.
	for _, clause := range []string{
		"Selection and interpretation notes",
		"fact-level known_by, unknown_to, suspected_by, misinformed_by and revealed_to",
		"do not infer a knower or co-planner from shared scene participation",
		"Hearing an instruction does not establish knowledge of the full plan",
		"Keep different facts' knowledge scopes separate",
	} {
		if !strings.Contains(prompt, clause) {
			t.Errorf("assembled request is missing knowledge-boundary instruction %q", clause)
		}
	}
}

func assertScopePrompt49RecordedInput(t *testing.T, packed map[string]any, input map[string]any) {
	t.Helper()
	got := outputFidelityLineageSlice(packed["candidates"])
	want := outputFidelityLineageSlice(input["candidates"])
	if len(got) != len(want) {
		t.Fatalf("candidate reading changed: got %d, want %d", len(got), len(want))
	}
	for i, raw := range want {
		original, wire := mapFromAny(raw), mapFromAny(got[i])
		for _, key := range []string{"ref", "text", "knowledge_scope"} {
			a, _ := json.Marshal(original[key])
			b, _ := json.Marshal(wire[key])
			if !bytes.Equal(a, b) {
				t.Errorf("fact %s changed supplied %s: %s != %s", original["ref"], key, b, a)
			}
		}
		if _, present := original["knowledge_scope"]; !present {
			if _, invented := wire["knowledge_scope"]; invented {
				t.Errorf("invented scope on %s", original["ref"])
			}
		}
	}
}

func Test49PreprocessingFactScopePromptIndependentAssembly(t *testing.T) {
	s := &Server{}
	for _, customShared := range []bool{false, true} {
		for _, customRole := range []bool{false, true} {
			cfg := scopePrompt49Settings(customShared, customRole)
			for _, role := range multiAgentRoles {
				for round := 1; round <= 2; round++ {
					t.Run(fmt.Sprintf("shared_%t_role_%t/%s/round_%d", customShared, customRole, role, round), func(t *testing.T) {
						input := scopePrompt49Input(role, round)
						before, _ := json.Marshal(input)
						call, req := s.multiAgentProxyRequest(role, cfg, round, input)
						if call.Error != "" || len(req.Messages) != 2 {
							t.Fatalf("production request assembly failed: %s", call.Error)
						}
						prompt := stringFromMap(mapFromAny(req.Messages[0]), "content")
						assertScopePrompt49Instruction(t, prompt)
						shared, assigned := cfg.SharedPrompt, cfg.Roles[role].Prompt
						if !customShared {
							shared = multiAgentSharedPrompt
						}
						if !customRole {
							assigned = multiAgentRolePrompts[role]
						}
						if !strings.HasPrefix(prompt, shared+"\n\nAssigned role: "+role+"\n"+assigned+"\n\n") {
							t.Fatal("effective common/role prompt was replaced")
						}
						var packed map[string]any
						if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(req.Messages[1]), "content")), &packed); err != nil {
							t.Fatal(err)
						}
						assertScopePrompt49RecordedInput(t, packed, input)
						after, _ := json.Marshal(input)
						if !bytes.Equal(before, after) {
							t.Fatal("request assembly mutated recorded input")
						}
					})
				}
			}
		}
	}
}

func Test49PreprocessingFactScopePromptGroupedAssembly(t *testing.T) {
	// Replace the external HTTP boundary in memory; no listener or network I/O.
	oldClient := proxyHTTPClient
	t.Cleanup(func() { proxyHTTPClient = oldClient })
	for _, customShared := range []bool{false, true} {
		for _, customRole := range []bool{false, true} {
			for round := 1; round <= 2; round++ {
				t.Run(fmt.Sprintf("shared_%t_role_%t/round_%d", customShared, customRole, round), func(t *testing.T) {
					cfg := scopePrompt49Settings(customShared, customRole)
					inputs := make([]map[string]any, len(multiAgentRoles))
					for i, role := range multiAgentRoles {
						inputs[i] = scopePrompt49Input(role, round)
					}
					before, _ := json.Marshal(inputs)
					requests := 0
					proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
						requests++
						if requests != 1 || r.URL.Host != "scope-prompt.invalid" || r.Method != http.MethodPost {
							t.Fatalf("unexpected external-boundary request: %s %s, count=%d", r.Method, r.URL.Host, requests)
						}
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						messages := outputFidelityLineageSlice(body["messages"])
						if len(messages) != 2 {
							t.Fatalf("unexpected message count: %d", len(messages))
						}
						prompt := stringFromMap(mapFromAny(messages[0]), "content")
						assertScopePrompt49Instruction(t, prompt)
						shared := cfg.SharedPrompt
						if !customShared {
							shared = multiAgentSharedPrompt
						}
						if !strings.HasPrefix(prompt, shared+"\n\n") {
							t.Fatal("group replaced effective common prompt")
						}
						var packet map[string]any
						if err := json.Unmarshal([]byte(stringFromMap(mapFromAny(messages[1]), "content")), &packet); err != nil {
							t.Fatal(err)
						}
						assignments := outputFidelityLineageSlice(packet["roles"])
						if len(assignments) != len(multiAgentRoles) {
							t.Fatal("group dropped an assignment")
						}
						results := map[string]any{}
						for i, role := range multiAgentRoles {
							assignment := mapFromAny(assignments[i])
							assigned := cfg.Roles[role].Prompt
							if !customRole {
								assigned = multiAgentRolePrompts[role]
							}
							if assignment["role"] != role || assignment["prompt"] != assigned {
								t.Fatal("group replaced effective role prompt")
							}
							reading := map[string]any{}
							for key, value := range mapFromAny(packet["shared_input"]) {
								reading[key] = value
							}
							for key, value := range mapFromAny(assignment["input"]) {
								reading[key] = value
							}
							assertScopePrompt49RecordedInput(t, reading, inputs[i])
							results[role] = map[string]any{"selected_ids": []string{}, "selected_summary_ids": []string{}}
						}
						// A synthetic empty reply terminates transport only; it does not
						// claim any model interpreted the knowledge scope correctly.
						reply, _ := json.Marshal(map[string]any{"roles": results})
						encoded, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(reply)}}}})
						return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(encoded)), Request: r}, nil
					})}
					calls := (&Server{}).callMultiAgentGroup(context.Background(), cfg, round, multiAgentRoles, inputs, "synthetic-scope-prompt")
					if requests != 1 || len(calls) != len(multiAgentRoles) {
						t.Fatal("group bypassed recorded request assertions")
					}
					for _, call := range calls {
						if call.Error != "" {
							t.Fatalf("synthetic group transport failed: %s", call.Error)
						}
					}
					after, _ := json.Marshal(inputs)
					if !reflect.DeepEqual(before, after) {
						t.Fatal("group request assembly mutated recorded inputs")
					}
				})
			}
		}
	}
}
