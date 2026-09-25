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
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

func Test47IdenticalSearchExecutesOnceAndRetainsRoleEvidence(t *testing.T) {
	for _, mode := range []string{"shared", "distinct", "shared_failure"} {
		t.Run(mode, func(t *testing.T) {
			const question = "Which recorded condition applies to the reed lantern?"
			facts := []prepareTurnPriorityMemoryCandidate{
				{CanonicalFactID: "visit", Lane: "event_recent", CompleteText: "Mira previously visited the lantern workshop.", SourceTurn: 2, Visibility: "public"},
				{CanonicalFactID: "lantern", Lane: "world_state", CompleteText: "The reed lantern stands at the gate.", SourceTurn: 8, Visibility: "public"},
			}
			found := []prepareTurnPriorityMemoryCandidate{
				{CanonicalFactID: "condition", Lane: "world_state", CompleteText: "RECOVERED_CONDITION: the reed lantern reveals wet ink only.", SourceTurn: 3, Visibility: "public", SupplementalQueryMatched: true},
				{CanonicalFactID: "private", Lane: "subjective_relationship", CompleteText: "PRIVATE_READER_MEMORY", PerspectiveOwner: "Nell", Visibility: "owner_private", SupplementalQueryMatched: true},
			}
			var mu sync.Mutex
			seen := map[string]int{}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var wire struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if json.NewDecoder(r.Body).Decode(&wire) != nil || len(wire.Messages) != 2 {
					t.Error("unexpected provider request")
					http.Error(w, "fixture", 400)
					return
				}
				var packet map[string]any
				if json.Unmarshal([]byte(wire.Messages[1].Content), &packet) != nil {
					t.Error("invalid reading packet")
					return
				}
				role := extractionStringFromAny(packet["role"])
				if role != "event_recent" && role != "world_state" {
					t.Error("unexpected role", role)
					return
				}
				mu.Lock()
				seen[role]++
				mu.Unlock()
				items := outputFidelityLineageSlice(packet["candidates"])
				if len(items) == 0 {
					t.Error("lost original candidates")
					return
				}
				answer := multiAgentRecommendation{SelectedIDs: []string{extractionStringFromAny(mapFromAny(items[0])["ref"])}}
				if packet["previous_result"] == nil {
					q := question
					if mode == "distinct" && role == "world_state" {
						q = "Who last moved the reed lantern?"
					}
					answer.SearchRequests = []string{q}
				} else {
					if strings.Contains(wire.Messages[1].Content, "PRIVATE_READER_MEMORY") {
						t.Error("private evidence entered public role")
					}
					if mode != "shared_failure" && !strings.Contains(wire.Messages[1].Content, "RECOVERED_CONDITION") {
						t.Error("shared search evidence did not reach", role)
					}
				}
				content, _ := json.Marshal(answer)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
			}))
			defer provider.Close()
			cfg := defaultMultiAgentSettings()
			cfg.Enabled = true
			for role, c := range cfg.Roles {
				c.Enabled = role == "event_recent" || role == "world_state"
				c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, role, "fixture-key"
				cfg.Roles[role] = c
			}
			searchCalls := 0
			search := func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				mu.Lock()
				searchCalls++
				mu.Unlock()
				if q != question && !(mode == "distinct" && q == "Who last moved the reed lantern?") {
					t.Error("unexpected question", q)
				}
				trace := map[string]any{"memory_search_result": "ok", "query_embedding_count": 1, "query_text_count": 1}
				if mode == "shared_failure" {
					trace["memory_search_result"] = "error"
					return nil, nil, trace
				}
				return found, nil, trace
			}
			// Two separate preparations must not share results across requests.
			for run := 0; run < 2; run++ {
				got := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 4000, 5, nil, search)
				want := run + 1
				if mode == "distinct" {
					want *= 2
				}
				if searchCalls != want {
					t.Fatalf("physical searches=%d want=%d", searchCalls, want)
				}
				if len(got.Searches) != 2 {
					t.Fatal("lost role-scoped search traces")
				}
				embeds := 0
				for _, trace := range got.Searches {
					embeds += intFromAny(trace["query_embedding_count"], 0)
				}
				wantEmbeds := 1
				if mode == "distinct" {
					wantEmbeds = 2
				}
				if embeds != wantEmbeds {
					t.Fatal("provider usage counted once per reader", embeds)
				}
				for _, role := range []string{"event_recent", "world_state"} {
					if got.role(role).Source != "ai" || len(got.role(role).Selection.SelectedIDs) == 0 {
						t.Error("working recommendation lost", role)
					}
				}
			}
			for _, role := range []string{"event_recent", "world_state"} {
				if seen[role] != 4 {
					t.Error("rounds changed", role, seen[role])
				}
			}
		})
	}
}

// The provider and retrieval are external boundaries. All packet construction,
// grouped dispatch, reference resolution, selection and payload assembly are real.
func Test44OwnSearchCompletionEvidenceReachesRequestingRole(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, reply := range []string{"recommendation", "empty", "failure"} {
			t.Run(fmt.Sprintf("grouped_%v_%s", grouped, reply), func(t *testing.T) {
				facts := []prepareTurnPriorityMemoryCandidate{
					{CanonicalFactID: "promise", Lane: "unresolved_goal", CompleteText: "status=open; Nero promised Jiwoo five casks of ale.", SourceRef: "pending_threads:403", SourceTable: "pending_threads", SourceTurn: 46},
					{CanonicalFactID: "plate", Lane: "unresolved_goal", CompleteText: "An adventurer plate was also promised.", SourceRef: "pending_threads:401", SourceTable: "pending_threads", SourceTurn: 41},
					{CanonicalFactID: "completed-event", Lane: "event_recent", CompleteText: "The ale outing was completed at the inn.", SourceRef: "memories:1523", SourceTurn: 47, Visibility: "public_projection"},
					{CanonicalFactID: "private-ale", Lane: "subjective_relationship", CompleteText: "PRIVATE_ALE_INTERPRETATION", SourceRef: "private:1", PerspectiveOwner: "Nero", Visibility: "owner_private", SupplementalQueryMatched: true},
					{CanonicalFactID: "walk", Lane: "event_recent", CompleteText: "Jiwoo and Mira are walking around Westport.", SourceRef: "memories:1600", SourceTurn: 105},
				}
				summaries := []prepareTurnPriorityTurnSummaryCandidate{
					{SummaryID: "fulfilled-summary", SourceRef: "memories:1523", SourceTurn: 47, CompleteText: "네로스가 여관 1층 홀을 정오부터 통째로 빌려 지우와의 생맥주 약속을 이행했다.", SourceVectorSimilarityObserved: true, SourceVectorSimilarity: .95},
					{SummaryID: "unrelated-summary", SourceRef: "memories:9", SourceTurn: 9, CompleteText: "Distant nebula survey."},
				}
				var mu sync.Mutex
				requests, seen := 0, map[string]int{}
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body struct {
						Messages []struct {
							Content string `json:"content"`
						} `json:"messages"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) < 2 {
						t.Error("missing provider messages", err)
						http.Error(w, "bad fixture request", 400)
						return
					}
					var packet map[string]any
					if err := json.Unmarshal([]byte(body.Messages[1].Content), &packet); err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					requests++
					members, isGroup := packet["roles"].([]any)
					if !isGroup {
						members = []any{map[string]any{"role": packet["role"], "input": packet}}
					}
					results := map[string]any{}
					failed := false
					for _, raw := range members {
						member := mapFromAny(raw)
						role := extractionStringFromAny(member["role"])
						input := map[string]any{}
						for k, v := range mapFromAny(packet["shared_input"]) {
							input[k] = v
						}
						for k, v := range mapFromAny(member["input"]) {
							input[k] = v
						}
						seen[role]++
						second := input["previous_result"] != nil
						answer := multiAgentRecommendation{}
						switch role {
						case "event_recent":
							answer.SelectedIDs = []string{"F5"}
							if !second {
								answer.SearchRequests = []string{"Mira walking around Westport"}
							}
						case "unresolved_goal":
							answer.SelectedIDs = []string{"F1", "F2"}
							if !second {
								answer.SearchRequests = []string{"Was the five casks of ale promise fulfilled?"}
							} else {
								found, total := map[string]int{}, 0
								for _, key := range []string{"candidates", "turn_summaries", "search_evidence"} {
									for _, raw := range outputFidelityLineageSlice(input[key]) {
										item := mapFromAny(raw)
										ref, text := extractionStringFromAny(item["ref"]), extractionStringFromAny(item["text"])
										found[ref]++
										total += len([]rune(text))
										if ref == "S1" {
											if key != "search_evidence" || text != summaries[0].CompleteText {
												t.Error("completion changed text or became selectable")
											}
											p := mapFromAny(mapFromAny(input["source_catalog"])[extractionStringFromAny(item["source"])])
											if p["r"] != summaries[0].SourceRef || p["n"] != float64(summaries[0].SourceTurn) {
												t.Error("completion lost its source or turn", p)
											}
										}
										if key == "candidates" && ref != "F1" && ref != "F2" {
											t.Error("selection category changed", ref)
										}
									}
								}
								if found["S1"] != 1 || found["F3"] != 1 || found["F4"] != 0 || found["S2"] != 0 || found["F5"] != 0 {
									t.Error("own-query evidence missing, duplicated, private, or borrowed from another question", found)
								}
								if total > intFromAny(mapFromAny(input["budgets"])["candidate_chars"], 0) {
									t.Error("reading exceeded existing input budget")
								}
								if len(outputFidelityLineageSlice(input["turn_summaries"])) != 0 {
									t.Error("goal gained summary selection")
								}
								answer.SelectedIDs = []string{"F2", "F1"}
								answer.Reasons = map[string]string{"F1": "S1 records fulfillment at turn 47; this is a past promise. The plate is a separate question."}
								if reply == "empty" {
									answer = multiAgentRecommendation{}
								}
								failed = reply == "failure"
							}
						default:
							t.Error("unexpected role", role)
						}
						results[role] = answer
					}
					if failed {
						http.Error(w, "fixture provider failure", http.StatusBadRequest)
						return
					}
					var response any = map[string]any{"roles": results}
					if !isGroup {
						response = results[packet["role"].(string)]
					}
					content, _ := json.Marshal(response)
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
				}))
				defer provider.Close()
				cfg := defaultMultiAgentSettings()
				cfg.Enabled = true
				for role, c := range cfg.Roles {
					c.Enabled = role == "event_recent" || role == "unresolved_goal"
					c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
					if !grouped {
						c.Model += "-" + role
					}
					cfg.Roles[role] = c
				}
				selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, summaries, 4000, 5, nil,
					func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
						if strings.Contains(q, "five casks") {
							return facts[:4], summaries, map[string]any{"memory_search_result": "ok"}
						}
						if strings.Contains(q, "Mira") {
							return facts[4:], nil, map[string]any{"memory_search_result": "ok"}
						}
						t.Error("unexpected search", q)
						return nil, nil, nil
					})
				wantRequests := 4
				if grouped {
					wantRequests = 2
				}
				if requests != wantRequests || seen["event_recent"] != 2 || seen["unresolved_goal"] != 2 {
					t.Fatal("dispatch/round count changed", requests, seen)
				}
				selection.BaselineIDs = map[string]bool{"promise": true, "walk": true}
				// Reading budgets do not shrink the canonical pool, including old,
				// unselected or private records retained by the existing owners.
				if !reflect.DeepEqual(selection.Candidates, facts) || !reflect.DeepEqual(selection.Summaries, summaries) {
					t.Fatal("supplemental reading removed or changed original memory candidates")
				}
				goal := selection.role("unresolved_goal")
				wantIDs := []string{"plate", "promise"}
				if reply == "failure" {
					wantIDs = []string{"promise", "plate"}
				}
				if reply == "empty" {
					if goal.Source != "go_default" || !multiAgentWants(selection, "unresolved_goal", "promise", false) {
						t.Fatal("Go selection fallback changed")
					}
				} else if !reflect.DeepEqual(goal.Selection.SelectedIDs, wantIDs) {
					t.Fatal("AI recommendation/order changed", goal.Selection.SelectedIDs)
				}
				if multiAgentWants(selection, "event_recent", "fulfilled-summary", true) {
					t.Fatal("reading-only completion became automatic selection")
				}
				assembly := prepareTurnInjectionAssembly{Preprocessing: selection}
				plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 4000, 5, "auto", nil, testPrepareTurnMemorySelectionContext(nil))
				notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
				for _, publisher := range []string{"disabled", "failed_open", "succeeded"} {
					payload := buildPrepareTurnPayloadApplicationPlan("Walk around town", "", extractionStringFromAny(plan["final_text"]), "", true, true, 4000, 0, 0, nil, publisher, notes)
					got := strings.Contains(extractionStringFromAny(payload["auxiliary_text"]), "S1 records fulfillment at turn 47")
					if got != (reply == "recommendation") {
						t.Error("accepted completion interpretation lost or fabricated", publisher, reply)
					}
				}
			})
		}
	}
}

func Test44OwnSearchReadingSharesBudgetAndPreservesSelections(t *testing.T) {
	for _, role := range multiAgentRoles {
		t.Run(role, func(t *testing.T) {
			cfg := defaultMultiAgentSettings()
			cfg.CandidateChars = 80
			facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "selected", Lane: role, CompleteText: strings.Repeat("사", 10)}}
			summaries := []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "complete", CompleteText: strings.Repeat("요", 60), SourceRef: "memories:47", SourceTurn: 47}}
			ctx := map[string]any{"retained_ids": map[string]bool{"selected": true}, "search_evidence_ranks": map[string]float64{"complete": 1}}
			input := multiAgentInput(role, facts, summaries, dto.PrepareTurnRequest{}, cfg, 160, 3, nil, ctx)
			if len(input["candidates"].([]map[string]any)) != len(facts) {
				t.Fatal("first-round selection disappeared")
			}
			key := "search_evidence"
			if role == "event_recent" {
				key = "turn_summaries"
			}
			items := input[key].([]map[string]any)
			if len(items) != 1 || items[0]["text"] != summaries[0].CompleteText {
				t.Fatal("whole completed source disappeared", input)
			}
			want := len([]rune(facts[0].CompleteText)) + len([]rune(summaries[0].CompleteText))
			if input["input_candidate_chars"] != want || want > cfg.CandidateChars {
				t.Fatal("existing character cap changed")
			}
		})
	}
}

func Test43MultiAgentSearchOverlapDeterministicMergeAndHUD(t *testing.T) {
	roles := []string{"event_recent", "character_objective", "world_state"}
	ledger := newTurnWorkflowHUDLedger()
	ledger.begin("search-request", "search-session", 1)
	ledger.begin("other-request", "other-session", 1)
	ctx := context.WithValue(context.Background(), multiAgentHUDRequestKey{}, "search-request")
	server := &Server{TurnWorkflows: ledger}
	var mu sync.Mutex
	providerCalls := map[string]int{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(body.Messages[1].Content), &input); err != nil {
			t.Error(err)
			return
		}
		role := input["role"].(string)
		mu.Lock()
		providerCalls[role]++
		mu.Unlock()
		answer := multiAgentRecommendation{}
		if _, second := input["previous_result"]; !second {
			answer.SearchRequests = []string{role, "extra question remains unresolved"}
			if role == "world_state" {
				answer.SelectedIDs = []string{"world-baseline"}
			}
		} else {
			snapshot, _ := ledger.snapshot("search-request")
			if snapshot.PreprocessingSearch == nil || snapshot.PreprocessingSearch.CompletedCount != len(roles) || snapshot.PreprocessingSearch.Status != "partial" {
				t.Errorf("round two began before the complete search barrier: %+v", snapshot.PreprocessingSearch)
			}
			if role == "event_recent" {
				ids, refs := []string{}, []string{}
				for _, raw := range input["candidates"].([]any) {
					item := raw.(map[string]any)
					ids = append(ids, item["text"].(string))
					refs = append(refs, item["ref"].(string))
					if item["ref"] == "F3" && item["text"] != roles[0] {
						t.Error("reverse completion replaced original-role duplicate content")
					}
				}
				want := []string{"Retrieved source for event_recent", "event_recent", "Retrieved source for world_state"}
				if !reflect.DeepEqual(ids, want) || !reflect.DeepEqual(refs, []string{"F2", "F3", "F4"}) {
					t.Errorf("all merged evidence/aliases changed: ids=%v refs=%v", ids, refs)
				}
				answer.SelectedIDs = []string{refs[len(refs)-1], refs[0]}
				answer.SearchRequests = []string{"no third search"}
			}
			if role == "world_state" {
				http.Error(w, "supplement unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		content, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, value := range cfg.Roles {
		value.Enabled = role == roles[0] || role == roles[1] || role == roles[2]
		value.Provider, value.Endpoint, value.Model, value.APIKey = "custom", provider.URL, "fixture-model-"+role, "fixture-key"
		cfg.Roles[role] = value
	}
	started := make(chan string, len(roles))
	releases := map[string]chan struct{}{}
	for _, role := range roles {
		releases[role] = make(chan struct{}, 1)
	}
	defer func() {
		for _, release := range releases {
			select {
			case release <- struct{}{}:
			default:
			}
		}
	}()
	resultCh := make(chan *multiAgentSelection, 1)
	go func() {
		resultCh <- server.runMultiAgent(ctx, cfg, dto.PrepareTurnRequest{}, []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "world-baseline", Lane: "world_state", CompleteText: "The gate is closed."}}, nil, 2000, 5, nil,
			func(query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
				started <- query
				<-releases[query]
				trace := map[string]any{"status": "ready", "memory_search_result": "ok", "breakdown_ms": map[string]float64{"health": 0.25, "embedding": 0.5}}
				if query == roles[1] {
					return nil, nil, map[string]any{"status": "degraded", "memory_search_result": "error", "memory_search_error": "private error detail"}
				}
				if query == roles[2] {
					trace["status"] = "partial"
					trace["precise_search_result"] = "error"
				}
				return []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: query, Lane: "event_recent", CompleteText: "Retrieved source for " + query}, {CanonicalFactID: "duplicate", Lane: "event_recent", CompleteText: query}}, []prepareTurnPriorityTurnSummaryCandidate{{SummaryID: "summary-" + query, CompleteText: query}}, trace
			})
	}()
	seen := map[string]bool{}
	for range roles {
		select {
		case role := <-started:
			seen[role] = true
		case <-time.After(3 * time.Second):
			t.Fatal("supplemental queries did not overlap at the search boundary")
		}
	}
	if len(seen) != len(roles) {
		t.Fatalf("one-query-per-role contract changed: %v", seen)
	}
	running, _ := ledger.snapshot("search-request")
	if running.PreprocessingSearch == nil || running.PreprocessingSearch.Status != "running" || running.PreprocessingSearch.CompletedCount != 0 || running.PreprocessingSearch.StartedAt.Location() != time.UTC {
		t.Fatalf("searching state absent: %+v", running.PreprocessingSearch)
	}
	other, _ := ledger.snapshot("other-request")
	if other.PreprocessingSearch != nil {
		t.Fatal("query timing escaped its request")
	}
	// This common blocked interval distinguishes phase wall time from a sum.
	// The channels above, not this delay, establish concurrency.
	time.Sleep(60 * time.Millisecond)
	for i := len(roles) - 1; i >= 0; i-- {
		releases[roles[i]] <- struct{}{}
		deadline := time.Now().Add(3 * time.Second)
		for {
			view, _ := ledger.snapshot("search-request")
			if view.PreprocessingSearch.CompletedCount >= len(roles)-i {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("query completion did not update the existing HUD ledger")
			}
			time.Sleep(time.Millisecond)
		}
	}
	var result *multiAgentSelection
	select {
	case result = <-resultCh:
	case <-time.After(3 * time.Second):
		t.Fatal("supplement did not complete")
	}
	if result.AnalysisCalls != 2*len(roles) || len(result.Searches) != len(roles) {
		t.Fatalf("call count changed: %+v", result)
	}
	for _, role := range roles {
		if providerCalls[role] != 2 {
			t.Fatalf("role %s made %d provider calls", role, providerCalls[role])
		}
	}
	if len(result.Summaries) != 2 || result.Summaries[0].SummaryID != "summary-"+roles[0] || result.Summaries[1].SummaryID != "summary-"+roles[2] {
		t.Fatalf("summary merge followed completion order: %+v", result.Summaries)
	}
	var summedMS float64
	for i, trace := range result.Searches {
		if trace["role"] != roles[i] {
			t.Fatal("completion order changed trace order")
		}
		summedMS += trace["duration_ms"].(float64)
	}
	if result.SearchDurationMS <= 0 || summedMS <= result.SearchDurationMS+20 {
		t.Fatalf("phase duration is not actual overlapping wall time: phase=%v sum=%v", result.SearchDurationMS, summedMS)
	}
	if !reflect.DeepEqual(result.role("event_recent").Selection.SelectedIDs, []string{"world_state", "event_recent"}) || !reflect.DeepEqual(result.role("world_state").Selection.SelectedIDs, []string{"world-baseline"}) || result.role("character_objective").Source != "go_default" {
		t.Fatal("success/partial/failed supplement changed selection preservation")
	}
	if len(result.role("event_recent").Unresolved) != 2 {
		t.Fatal("per-role query and round limits changed")
	}
	view, _ := ledger.snapshot("search-request")
	hudElapsed := float64(view.PreprocessingSearch.DurationMS)
	if view.PreprocessingSearch.Status != "partial" || result.SearchDurationMS < hudElapsed || result.SearchDurationMS-hudElapsed > 1 {
		t.Fatalf("HUD differs from phase timing: %+v", view.PreprocessingSearch)
	}
	wantStatuses := []string{"succeeded", "failed", "partial"}
	for i, query := range view.PreprocessingSearch.Queries {
		if query.Role != roles[i] || query.Status != wantStatuses[i] {
			t.Fatalf("query diagnostic order/state changed: %+v", query)
		}
	}
	encoded, _ := json.Marshal(view.PreprocessingSearch)
	if strings.Contains(string(encoded), "private error") || strings.Contains(string(encoded), "Retrieved source") || strings.Contains(string(encoded), "fixture-key") {
		t.Fatal("HUD exposed private content")
	}
	view.PreprocessingSearch.Queries[0].BreakdownMS["health"] = -1
	view.PreprocessingSearch.Queries[0].Status = "changed"
	clone, _ := ledger.snapshot("search-request")
	if clone.PreprocessingSearch.Queries[0].BreakdownMS["health"] != 0.25 || clone.PreprocessingSearch.Queries[0].Status != "succeeded" {
		t.Fatal("HUD snapshot aliases search diagnostics")
	}
	for _, enabled := range []bool{false, true} {
		ledger.begin("no-search", "no-search-session", 1)
		cfg.Enabled = enabled
		for role, value := range cfg.Roles {
			value.Enabled = false
			cfg.Roles[role] = value
		}
		server.runMultiAgent(context.WithValue(ctx, multiAgentHUDRequestKey{}, "no-search"), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 5, nil, nil)
		off, _ := ledger.snapshot("no-search")
		if off.PreprocessingSearch != nil {
			t.Fatal("OFF/no-query run showed search timing")
		}
	}
}

func Test43PreprocessingSearchDiagnosticStatusAndTimingCopy(t *testing.T) {
	for _, fixture := range []struct {
		trace  map[string]any
		status string
	}{
		{map[string]any{"status": "ready", "memory_search_result": "not_found"}, "succeeded"},
		{map[string]any{"status": "degraded", "memory_search_result": "ok", "search_result": "error"}, "partial"},
		{map[string]any{"status": "ready", "memory_search_result": "error"}, "failed"},
		{map[string]any{"status": "ready", "search_skipped_reason": "missing_query_text_for_embedding"}, "failed"},
	} {
		if got := multiAgentSearchOutcomeStatus(fixture.trace); got != fixture.status {
			t.Errorf("status=%s want=%s", got, fixture.status)
		}
	}
	shadow := map[string]any{"status": "ready", "breakdown_ms": map[string]float64{"health": 1.25}}
	trace := prepareTurnPreprocessingSearchTrace(shadow)
	trace["breakdown_ms"].(map[string]float64)["health"] = 2.5
	if shadow["breakdown_ms"].(map[string]float64)["health"] != 1.25 {
		t.Fatal("search trace aliases retrieval timing map")
	}
}
