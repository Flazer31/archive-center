package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
)

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
					ids = append(ids, item["id"].(string))
					refs = append(refs, item["ref"].(string))
					if item["id"] == "duplicate" && item["text"] != roles[0] {
						t.Error("reverse completion replaced original-role duplicate content")
					}
				}
				want := []string{"event_recent", "duplicate", "world_state"}
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
		value.Provider, value.Endpoint, value.Model, value.APIKey = "custom", provider.URL, "fixture-model", "fixture-key"
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
