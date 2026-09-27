package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test48CommitmentRelationsReachGoAndPreprocessing(t *testing.T) {
	thread := store.PendingThread{ID: 41, ChatSessionID: "s", ThreadKey: "bridge", Description: "Repair the bridge", Status: "open", Owner: "Mira", Target: "Rowan", SourceTurn: 4, CreatedTurn: 4, Pinned: true,
		DetailsJSON: `{"schedule":{"kind":"conditional","condition":"only when the river subsides","due":"2900-09-06 정오"},"evidence_excerpt":"Mira promised Rowan to repair the bridge only when the river subsides."}`}
	in := prepareTurnAssemblyInput{PendingThreads: []store.PendingThread{thread}, TopK: 5, MaxChars: 32000, UserInput: "Mira and Rowan discuss repairing the bridge", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	in.Perspective.StoryClock = map[string]any{"absolute": map[string]any{"date": "2900-09-06", "time": "13:15"}}
	before := mustCompactJSON(in)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	if len(facts) == 0 {
		t.Fatal("no production candidates")
	}
	for _, mode := range []string{"go", "preprocessing"} {
		t.Run(mode, func(t *testing.T) {
			selected := out
			if mode == "preprocessing" {
				ids := []string{}
				for _, f := range facts {
					ids = append(ids, f.CanonicalFactID)
				}
				selected.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "unresolved_goal", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: ids}}}}
			}
			plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&selected, in.MaxChars, 5, "auto", nil, in.Perspective.Selection)
			text := extractionStringFromAny(plan["final_text"])
			if !containsAll(text, "Repair the bridge", "Mira", "Rowan", "only when the river subsides", "due=2900-09-06 12:00", "1h 15m before reference", "due_passed_outcome_unknown") {
				t.Fatalf("lost commitment roles/condition/schedule: %s", text)
			}
			if strings.Contains(text, "status=resolved") || strings.Contains(text, "status=completed") {
				t.Fatalf("past due date completed the promise: %s", text)
			}
			assert45Budget(t, plan, in.MaxChars)
			t.Logf("%s: chars=%d", mode, len([]rune(text)))
		})
	}
	if before != mustCompactJSON(in) {
		t.Fatal("reading mutated input")
	}
}

func Test48CommitmentRelationActualPreprocessingRequestAndSelection(t *testing.T) {
	in := prepareTurnAssemblyInput{PendingThreads: []store.PendingThread{{ID: 41, ChatSessionID: "s", ThreadKey: "bridge", Description: "Repair the bridge", Status: "open", Owner: "Mira", Target: "Rowan", Pinned: true, SourceTurn: 4, DetailsJSON: `{"condition":"only when the river subsides","due":"2900-09-06 정오"}`}}, TopK: 5, MaxChars: 32000, UserInput: "Mira and Rowan discuss the bridge", Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	in.Perspective.StoryClock = map[string]any{"absolute": map[string]any{"date": "2900-09-06", "time": "13:15"}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	var calls, witnessed atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		messages := outputFidelityLineageSlice(body["messages"])
		wire := extractionStringFromAny(mapFromAny(messages[1])["content"])
		if containsAll(wire, "Mira", "Rowan", "only when the river subsides", "due=2900-09-06 12:00", "1h 15m before reference") {
			witnessed.Add(1)
		}
		var packet map[string]any
		if err := json.Unmarshal([]byte(wire), &packet); err != nil {
			t.Error(err)
			return
		}
		choose := func(input map[string]any) map[string]any {
			ids := []string{}
			for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
				ids = append(ids, extractionStringFromAny(mapFromAny(raw)["ref"]))
			}
			return map[string]any{"selected_ids": ids}
		}
		var answer any
		if roles := outputFidelityLineageSlice(packet["roles"]); len(roles) > 0 {
			results := map[string]any{}
			for _, raw := range roles {
				role := mapFromAny(raw)
				input := map[string]any{}
				for k, v := range mapFromAny(packet["shared_input"]) {
					input[k] = v
				}
				for k, v := range mapFromAny(role["input"]) {
					input[k] = v
				}
				results[extractionStringFromAny(role["role"])] = choose(input)
			}
			answer = map[string]any{"roles": results}
		} else {
			answer = choose(packet)
		}
		raw, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(raw)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = true
		c.UsePublisher = false
		c.Provider = "custom"
		c.Endpoint = provider.URL
		c.Model = "same-model"
		c.APIKey = "synthetic-key"
		cfg.Roles[role] = c
	}
	server := &Server{}
	out.Preprocessing = server.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, in.MaxChars, 5, nil, nil)
	if witnessed.Load() == 0 {
		t.Fatal("model never received the commitment's qualified roles and date")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected existing grouped call only; calls=%d", calls.Load())
	}
	if role := out.Preprocessing.role("unresolved_goal"); role.Source != "ai" || len(role.Selection.SelectedIDs) == 0 {
		t.Fatalf("actual AI response not applied: %+v", role)
	}
	plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, in.MaxChars, 5, "auto", nil, in.Perspective.Selection)
	text := extractionStringFromAny(plan["final_text"])
	if !containsAll(text, "Mira", "Rowan", "only when the river subsides", "due=2900-09-06 12:00", "1h 15m before reference", "due_passed_outcome_unknown") {
		t.Fatalf("AI selection lost relation: %s", text)
	}
	assert45Budget(t, plan, in.MaxChars)
}

func Test48PreciseRelationKeepsOccurrenceTimeInSharedDelivery(t *testing.T) {
	u := store.PreciseMemoryUnit{UnitID: "event-1", ChatSessionID: "s", SourceRevision: "rev", SourceTurnStart: 3, SourceTurnEnd: 3, Kind: "event", Visibility: "public", PayloadJSON: `{"event":"Mira delivered the Brass Compass to Rowan.","actor":"Mira","occurrence_time":{"date":"2002-06-02","time":"10:00"},"observed_at":{"date":"2002-06-04","time":"13:00"}}`}
	fact, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(u, .8, "fixture")
	if !ok {
		t.Fatal("no precise fact")
	}
	if len(fact.Fact.TemporalContext) == 0 {
		t.Fatal("precise relation lost source occurrence/observation time")
	}
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{Lane: fact.Lane, SourceTable: "precise_memory_units", SourceTurn: 3, Fact: fact.Fact, Importance: .8, ImportancePresent: true}}}
	prepareTurnAttachTemporalContext(&out, map[string]any{"absolute": map[string]any{"date": "2002-06-04", "time": "13:00"}})
	if !containsAll(mustCompactJSON(out.PriorityFactSeeds), "2002-06-02", "2002-06-04") {
		t.Fatal("shared source time reading lost one of the distinct dates")
	}
}

func Test48CommitmentReadKeepsLegacyAndResolvedMeaning(t *testing.T) {
	for _, tc := range []struct{ name, status, details, want string }{
		{"legacy", "open", "not-json", ""},
		{"open-past-due", "open", `{"due":"2002-06-04"}`, "due_passed_outcome_unknown"},
		{"resolved-past-due", "resolved", `{"due":"2002-06-04"}`, "due_passed_with_explicit_outcome"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			thread := store.PendingThread{ID: 1, ThreadKey: "bridge", Status: tc.status, Description: "Repair the bridge", DetailsJSON: tc.details}
			record := readMemoryRelations(memoryRelationInput{Threads: []store.PendingThread{thread}}).Records[0]
			before := mustCompactJSON(record)
			fact := prepareTurnPriorityMemoryFact{Text: thread.Description}
			prepareTurnAttachCommitmentRelation(&fact, record, map[string]any{"absolute": map[string]any{"date": "2002-06-10"}})
			if tc.want == "" {
				if fact.Reading != nil || fact.Text != thread.Description {
					t.Fatal("legacy unknown metadata changed original delivery")
				}
			} else if !strings.Contains(mustCompactJSON(fact.Reading), tc.want) {
				t.Fatalf("outcome drift: %+v", fact.Reading)
			}
			if before != mustCompactJSON(record) {
				t.Fatal("enrichment mutated common source")
			}
		})
	}
}
