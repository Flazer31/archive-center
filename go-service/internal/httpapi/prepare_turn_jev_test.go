package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestJevReviewBudgetSurvivesUnavailableRoles(t *testing.T) {
	for _, mode := range []string{"auto", "custom"} {
		for _, response := range []string{"success", "event_http_error", "all_http_error", "event_missing_rank"} {
			t.Run(mode+"/"+response, func(t *testing.T) {
				const capChars = 2400
				selection := &multiAgentSelection{Contract: multiAgentContract}
				known := map[string]bool{}
				for _, lane := range multiAgentRoles {
					role := multiAgentRoleResult{Role: lane, Source: "ai"}
					for i := 0; i < 12; i++ {
						id := fmt.Sprintf("%s-%d", lane, i)
						text := fmt.Sprintf("%s source %d: %s", lane, i, strings.Repeat("complete evidence ", 6))
						selection.Candidates = append(selection.Candidates, prepareTurnPriorityMemoryCandidate{
							CanonicalFactID: id, Lane: lane, CompleteText: text, Chars: len([]rune(text)),
							SourceTable: "fixture", SourceRef: id, Visibility: "general",
						})
						role.Selection.SelectedIDs = append(role.Selection.SelectedIDs, id)
						known[id] = true
					}
					if lane == "event_recent" {
						for i := 0; i < 4; i++ {
							id := fmt.Sprintf("summary-%d", i)
							text := fmt.Sprintf("Episode %d: %s", i, strings.Repeat("whole chronology ", 12))
							selection.Summaries = append(selection.Summaries, prepareTurnPriorityTurnSummaryCandidate{SummaryID: id, CompleteText: text, Chars: len([]rune(text))})
							role.Selection.SelectedSummaryIDs = append(role.Selection.SelectedSummaryIDs, id)
							known[id] = true
						}
					}
					selection.Roles = append(selection.Roles, role)
				}
				originalFacts := string(mustJSON(selection.Candidates))
				originalSummaries := string(mustJSON(selection.Summaries))
				originalOrder := map[string][]string{}
				for _, role := range selection.Roles {
					originalOrder[role.Role] = append([]string(nil), role.Selection.SelectedIDs...)
				}
				var requests atomic.Int32
				provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					var body struct {
						State struct {
							Role  string           `json:"role"`
							Items []map[string]any `json:"items"`
						} `json:"state"`
						Questions map[string]any `json:"questions"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if _, ok := originalOrder[body.State.Role]; !ok || len(body.State.Items) == 0 {
						t.Error("unexpected review scope", body.State.Role)
					}
					if len(body.Questions) != 4*len(body.State.Items) {
						t.Error("review question set changed")
					}
					for _, item := range body.State.Items {
						if !known[stringFromMap(item, "id")] {
							t.Error("unexpected source", item["id"])
						}
					}
					if response == "all_http_error" || (response == "event_http_error" && body.State.Role == "event_recent") {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					answers := map[string]any{}
					for i := range body.State.Items {
						// Equal judgments preserve the original order: only response
						// availability varies in this budget regression.
						if response != "event_missing_rank" || body.State.Role != "event_recent" {
							answers[fmt.Sprintf("review_%d_rank", i)] = map[string]any{"type": "score", "score": 2}
						}
						for kind, choice := range map[string]string{"support": "supported", "time": "historical", "knowledge": "unknown"} {
							answers[fmt.Sprintf("review_%d_%s", i, kind)] = map[string]any{"type": "choice", "choice": choice}
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
				}))
				defer provider.Close()
				cfg := defaultMultiAgentSettings()
				cfg.Enabled = true
				cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, APIKey: "fixture", Model: "fixture"}
				budgets := map[string]int{}
				for _, lane := range multiAgentRoles {
					budgets[lane] = capChars / len(multiAgentRoles)
				}
				laneCaps, _ := prepareTurnPriorityDeliveryCaps(capChars, mode, budgets)
				selection.JevReview = (&Server{}).reviewWithJev(context.Background(), cfg, selection, dto.PrepareTurnRequest{RawUserInput: strPtr("Continue the journey using the established commitments and relationships.")}, capChars, 2, laneCaps, nil, nil)
				applyJevReview(selection)
				if requests.Load() != int32(len(multiAgentRoles)) {
					t.Fatal("unexpected extra dispatch or retry", requests.Load())
				}
				for _, role := range selection.Roles {
					if !reflect.DeepEqual(role.Selection.SelectedIDs, originalOrder[role.Role]) {
						t.Fatal("unavailable/equal judgments changed selection", role.Role)
					}
					wantSource := "jev_review"
					if response == "all_http_error" || (response != "success" && role.Role == "event_recent") {
						wantSource = "ai"
					}
					if role.Source != wantSource {
						t.Fatal("fixture failed to exercise retained selection", role.Role, role.Source, wantSource)
					}
				}
				out := prepareTurnInjectionAssembly{Preprocessing: selection}
				plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, capChars, 2, mode, budgets, testPrepareTurnMemorySelectionContext(priorityMemoryTestContext(2)))
				text := extractionStringFromAny(plan["final_text"])
				if len([]rune(text)) > capChars || intFromAny(plan["budget_overrun_chars"], 0) != 0 {
					t.Fatalf("unavailable review escaped the shared budget: used=%d cap=%d", len([]rune(text)), capChars)
				}
				for _, raw := range prepareTurnMemoryLineageSlice(plan["classes"]) {
					lane := mapFromAny(raw)
					key := stringFromMap(lane, "key")
					if _, ok := originalOrder[key]; ok {
						if intFromAny(lane["selected_count"], 0) == 0 {
							t.Error("another lane was starved by retained event selection", key)
						}
						if mode == "custom" && intFromAny(lane["used_chars"], 0) > budgets[key] {
							t.Error("custom lane budget escaped", key)
						}
					}
				}
				delivered := map[string]bool{}
				for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
					item := mapFromAny(raw)
					if item["selection_status"] == "selected" {
						delivered[stringFromMap(item, "canonical_fact_id")] = true
						if !strings.Contains(text, stringFromMap(item, "complete_text")) {
							t.Fatal("selected evidence was truncated")
						}
					}
				}
				for _, raw := range prepareTurnMemoryLineageSlice(plan["turn_summary_items"]) {
					item := mapFromAny(raw)
					if item["selection_status"] == "selected" {
						delivered[stringFromMap(item, "summary_id")] = true
					}
				}
				buildPrepareTurnPreprocessingNotes(selection, plan, nil)
				for _, item := range selection.JevReview.Items {
					if (item.DeliveryStatus == "delivered") != delivered[item.ID] {
						t.Error("review delivery audit disagrees with actual assembly", item.ID)
					}
				}
				if string(mustJSON(selection.Candidates)) != originalFacts || string(mustJSON(selection.Summaries)) != originalSummaries {
					t.Fatal("budgeting changed the original source pool")
				}
			})
		}
	}
}

func TestJevSettingsModesKeyPreservationAndConnectionRoute(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Error("credential missing")
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if !strings.Contains(fmt.Sprint(payload["state"]), "Mira returned") {
			t.Error("connection test is not synthetic")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture-jev", "answers": map[string]any{"check": map[string]any{"type": "choice", "choice": "rowan"}}, "usage": map[string]int{"input_tokens": 33}})
	}))
	defer provider.Close()
	s := &Server{}
	mux := http.NewServeMux()
	s.registerConfigRoutes(mux)
	put := func(body string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/config/memory-preprocessing", strings.NewReader(body)))
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "fixture-secret") {
			t.Fatal("Jev credential leaked")
		}
		var view map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &view)
		return view
	}
	for _, c := range []struct {
		llm, jev bool
		want     string
	}{{false, false, "go"}, {true, false, "llm"}, {false, true, "jev_primary"}, {true, true, "jev_review"}} {
		body, _ := json.Marshal(map[string]any{"enabled": c.llm, "jev": map[string]any{"enabled": c.jev, "endpoint": provider.URL, "model": "fixture-jev", "api_key": "fixture-secret"}})
		view := put(string(body))
		if view["effective_mode"] != c.want {
			t.Fatal("wrong mode", view["effective_mode"], c.want)
		}
		if !boolFromAny(mapFromAny(mapFromAny(view["settings"])["jev"])["api_key_set"]) {
			t.Fatal("missing key presence")
		}
	}
	put(fmt.Sprintf(`{"enabled":false,"jev":{"enabled":true,"endpoint":%q,"model":"fixture-jev"}}`, provider.URL))
	loaded, err := (&Server{}).loadMultiAgentSettings()
	if err != nil || loaded.Jev.APIKey != "fixture-secret" || loaded.effectiveMode() != "jev_primary" {
		t.Fatal("restart lost Jev settings")
	}
	put(`{"enabled":false}`) // Omitted optional feature preserves its saved settings.
	loaded, _ = s.loadMultiAgentSettings()
	if !loaded.Jev.Enabled || loaded.Jev.APIKey != "fixture-secret" {
		t.Fatal("omitted Jev erased settings")
	}
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"endpoint":%q,"model":"fixture-jev"}`, provider.URL)
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/config/memory-preprocessing/jev-test", strings.NewReader(body)))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ok":true`) || requests.Load() != 1 {
		t.Fatal("registered test route failed", rec.Body.String())
	}
	loaded, _ = s.loadMultiAgentSettings()
	if !loaded.Jev.Enabled {
		t.Fatal("connection test mutated settings")
	}
	put(fmt.Sprintf(`{"jev":{"endpoint":%q,"model":"fixture-jev","api_key":""}}`, provider.URL))
	loaded, _ = s.loadMultiAgentSettings()
	if loaded.Jev.APIKey != "" {
		t.Fatal("explicit key clear ignored")
	}
}

func jevFixtureProvider(t *testing.T, counter *atomic.Int32, search bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		var payload struct {
			State     map[string]any `json:"state"`
			Questions map[string]any `json:"questions"`
			Messages  any            `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		if payload.Messages != nil {
			t.Error("primary invoked generative LLM")
		}
		if r.Header.Get("Authorization") != "Bearer test-jev" {
			t.Error("wrong auth")
		}
		role := extractionStringFromAny(payload.State["role"])
		if role != "subjective_relationship" && strings.Contains(fmt.Sprint(payload.State), "PRIVATE_OWNER_ONLY") {
			t.Error("private candidate entered another role")
		}
		items, _ := payload.State["items"].([]any)
		answers := map[string]any{}
		for key := range payload.Questions {
			if key == "search" {
				choice := "none"
				if search && role == "event_recent" {
					choice = "cause"
				}
				answers[key] = map[string]any{"type": "choice", "choice": choice}
				continue
			}
			var i int
			if _, err := fmt.Sscanf(key, "rank_%d", &i); err == nil && i < len(items) {
				item := mapFromAny(items[i])
				value := extractionStringFromAny(item["text"])
				if strings.Contains(value, "unanswered") {
					continue
				}
				score := 0.2
				if strings.Contains(value, "key") {
					score = 2.9
				}
				answers[key] = map[string]any{"type": "score", "score": score, "confidence": 0.8}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture-jev", "answers": answers, "usage": map[string]int{"input_tokens": 100, "output_tokens": 20}})
	}))
}

func TestJevPrimaryUsesExistingSearchAndPreservesPartialEvidence(t *testing.T) {
	var calls atomic.Int32
	provider := jevFixtureProvider(t, &calls, true)
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture-jev", APIKey: "test-jev"}
	for role, c := range cfg.Roles {
		c.Enabled = false
		c.Endpoint = "http://generative-model-must-not-run.invalid"
		cfg.Roles[role] = c
	}
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "old", Lane: "event_recent", CompleteText: "An old festival was celebrated."},
		{CanonicalFactID: "private", Lane: "subjective_relationship", CompleteText: "PRIVATE_OWNER_ONLY", PerspectiveOwner: "Mira", Visibility: "owner_private"},
		{CanonicalFactID: "key", Lane: "character_objective", CompleteText: "Mira holds the archive key."},
		{CanonicalFactID: "unanswered", Lane: "character_objective", CompleteText: "unanswered source remains available."},
	}
	var searches atomic.Int32
	s := &Server{}
	result := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{RawUserInput: strPtr("Mira enters the archive with Rowan.")}, facts, nil, 2000, 5, nil, func(q string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		searches.Add(1)
		if !strings.Contains(q, "Mira enters") || !strings.Contains(q, "promise") {
			t.Error("query lost the grounded scene", q)
		}
		return []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "handover", Lane: "event_recent", CompleteText: "Rowan gave Mira the key yesterday.", Visibility: "public"}}, nil, map[string]any{"status": "ready"}
	})
	if searches.Load() != 1 || calls.Load() < 2 {
		t.Fatal("supplemental path not exercised", searches.Load(), calls.Load())
	}
	if result.role("event_recent").SelectionRound != 2 || result.role("event_recent").Selection.SelectedIDs[0] != "handover" {
		t.Fatal("searched evidence not selected", result.role("event_recent"))
	}
	character := result.role("character_objective")
	if character.Source != "jev" || !reflect.DeepEqual(character.Selection.SelectedIDs, []string{"key", "unanswered"}) {
		t.Fatal("partial ranking lost candidates", character)
	}
	if !multiAgentWants(result, "character_objective", "unanswered", false) {
		t.Fatal("unanswered became unavailable")
	}
	if result.JevReview != nil {
		t.Fatal("primary invoked review")
	}
	view := jevTimingView(result)
	if int64(intFromAny(view["calls"], 0)) != int64(calls.Load()) {
		t.Fatal("request count is not physical calls", view, calls.Load())
	}
}

func TestJevRankingReachesDeliveryWithinGoBudget(t *testing.T) {
	var calls atomic.Int32
	provider := jevFixtureProvider(t, &calls, false)
	defer provider.Close()
	out := prepareTurnInjectionAssembly{CharacterObjectiveText: "[Character Objective States]\n- Mira enjoys the autumn festival.\n- Rowan holds the archive key.\n- An unanswered distant tower remains standing."}
	ctx := testPrepareTurnMemorySelectionContext(priorityMemoryTestContext(1))
	baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 500, 1, "auto", nil, ctx)
	facts, summaries := multiAgentCandidatePool(&out)
	cfg := defaultMultiAgentSettings()
	cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "test-jev"}
	selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{RawUserInput: strPtr("Who can open the archive?")}, facts, summaries, 500, 1, nil, nil)
	selection.captureBaseline(baseline)
	out.Preprocessing = selection
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 180, 1, "auto", nil, ctx)
	text := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(text, "archive key") {
		t.Fatal("ranked key not delivered", text)
	}
	if intFromAny(plan["budget_overrun_chars"], 0) > 0 {
		t.Fatal("Jev bypassed the Go character budget", text)
	}
	payload := buildPrepareTurnPayloadApplicationPlan("Open the archive", "", text, "", true, true, 180, 0, 0, nil, "disabled", nil)
	encoded, _ := json.Marshal(payload)
	if !bytes.Contains(encoded, []byte("archive key")) {
		t.Fatal("selection missing from payload")
	}
}

func TestJevReviewRankingChangesDeliveredPayload(t *testing.T) {
	out := prepareTurnInjectionAssembly{CharacterObjectiveText: "[Character Objective States]\n- Mira remembers " + strings.Repeat("a distant festival, ", 20) + ".\n- Rowan holds the archive key."}
	ctx := testPrepareTurnMemorySelectionContext(priorityMemoryTestContext(1))
	baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 2000, 1, "auto", nil, ctx)
	facts, summaries := multiAgentCandidatePool(&out)
	ids, scores := []string{}, map[string]float64{}
	for _, fact := range facts {
		ids = append(ids, fact.CanonicalFactID)
		if strings.Contains(fact.CompleteText, "archive key") {
			scores[fact.CanonicalFactID] = 3
		} else {
			scores[fact.CanonicalFactID] = 0
		}
	}
	sort.SliceStable(ids, func(i, j int) bool { return scores[ids[i]] < scores[ids[j]] })
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(mustJSON(map[string]any{"selected_ids": ids, "search_requests": []string{}}))}}}})
	}))
	defer llm.Close()
	var requests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var body struct {
			State struct {
				Items []map[string]any `json:"items"`
			} `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		answers := map[string]any{}
		for i, item := range body.State.Items {
			id := stringFromMap(item, "id")
			score, ok := scores[id]
			if !ok {
				t.Error("unexpected source", id)
			}
			answers[fmt.Sprintf("review_%d_rank", i)] = map[string]any{"type": "score", "score": score}
			for key, choice := range map[string]string{"support": "supported", "time": "current", "knowledge": "owner_scoped"} {
				answers[fmt.Sprintf("review_%d_%s", i, key)] = map[string]any{"type": "choice", "choice": choice}
			}
		}
		if len(answers) != len(body.Questions) {
			t.Error("unexpected review question set")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for name, role := range cfg.Roles {
		role.Enabled = name == "character_objective"
		role.Provider = "custom"
		role.Endpoint = llm.URL
		role.APIKey = "fixture"
		role.Model = "fixture"
		cfg.Roles[name] = role
	}
	cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, APIKey: "fixture", Model: "fixture"}
	selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{RawUserInput: strPtr("Open the archive")}, facts, summaries, 2000, 1, nil, nil, map[string]any{"go_baseline_plan": baseline})
	role := selection.role("character_objective")
	if requests.Load() == 0 || role.Source != "jev_review" || scores[role.Selection.SelectedIDs[0]] != 3 {
		t.Fatal("review rank never applied", role.Source, role.Selection.SelectedIDs)
	}
	out.Preprocessing = selection
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 180, 1, "auto", nil, ctx)
	text := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(text, "archive key") || strings.Contains(text, "distant festival") || intFromAny(plan["budget_overrun_chars"], 0) > 0 {
		t.Fatal("reviewed order did not reach budgeted delivery", text)
	}
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	if !strings.Contains(extractionStringFromAny(notes["final_text"]), "knowledge_scope=owner_scoped") {
		t.Fatal("review interpretation absent from delivered notes", notes)
	}
	payload := buildPrepareTurnPayloadApplicationPlan("Open the archive", "", text, "", true, true, 180, 0, 0, nil, "disabled", nil)
	encoded, _ := json.Marshal(payload)
	if !bytes.Contains(encoded, []byte("archive key")) || bytes.Contains(encoded, []byte("distant festival")) {
		t.Fatal("review selection not applied to payload")
	}
	for _, item := range selection.JevReview.Items {
		want := "not_delivered"
		if scores[item.ID] == 3 {
			want = "delivered"
		}
		if item.DeliveryStatus != want {
			t.Error("review delivery trace mismatch", item)
		}
	}
}

func TestJevReviewContextKeepsCitationsDirectionsAndLatest(t *testing.T) {
	latest := "user:\nContinue.\n\nassistant:\n" + strings.Repeat("Latest scene. ", 70)
	direction := "user:\n" + strings.Repeat("Keep the secret private. ", 40) + "\n\n"
	opening := "assistant:\n\n" + strings.Repeat("Old setting. ", 50) + "\n\n"
	cited := strings.Repeat("The key was returned. ", 40) + "\n\n"
	omitted := strings.Repeat("Unrelated festival details. ", 40)
	input := map[string]any{"current_input": "Open the archive", "recent_conversation_reading": []map[string]any{
		{"Source": "recent_conversation", "Text": latest},
		{"Source": "recent_conversation", "Text": direction + opening + cited + omitted},
		{"Source": "recent_conversation_stored_summary", "Text": "Stored short summary", "summary_sources": []string{"original-source"}},
	}}
	before := string(mustJSON(input))
	role := multiAgentRoleResult{Selection: multiAgentRecommendation{Reasons: map[string]string{"key": "C2.3–C2.4 show the return."}}}
	got := jevReviewReading(input, role)
	encoded := string(mustJSON(got))
	for _, part := range []string{direction, strings.TrimPrefix(opening, "assistant:\n\n"), cited, "Stored short summary", "original-source"} {
		quoted := string(mustJSON(part))
		if !strings.Contains(encoded, quoted[1:len(quoted)-1]) {
			t.Error("required context missing", part[:15])
		}
	}
	turns := got["recent_conversation_reading"].([]map[string]any)
	joined := ""
	for _, passage := range turns[0]["Text"].([]map[string]any) {
		joined += extractionStringFromAny(passage["text"])
	}
	if joined != latest {
		t.Fatal("latest scene changed")
	}
	if strings.Contains(encoded, "Unrelated festival details") || got["review_context_note"] == nil {
		t.Fatal("uncited older text was repeated")
	}
	if string(mustJSON(input)) != before {
		t.Fatal("context projection mutated canonical input")
	}
	for _, reason := range []string{"No passage citation", "Unresolvable C99.1"} {
		role.Selection.Reasons["key"] = reason
		if string(mustJSON(jevReviewReading(input, role))) != before {
			t.Error("unassessed context was removed", reason)
		}
	}
}

func TestJevReviewPartialRanksPreserveUnansweredPositionsAndFacts(t *testing.T) {
	score := func(n float64) jevAnswer { return jevAnswer{Score: &n} }
	selection := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: []string{"low", "unanswered", "high"}}}}, Candidates: []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "high", CompleteText: "Historical source fact"}}, JevReview: &jevReviewResult{Items: []jevReviewItem{
		{Role: "event_recent", ID: "low", Answers: map[string]jevAnswer{"rank": score(0)}},
		{Role: "event_recent", ID: "unanswered", Answers: map[string]jevAnswer{}},
		{Role: "event_recent", ID: "high", Answers: map[string]jevAnswer{"rank": score(3), "support": {Choice: "contradicted"}, "time": {Choice: "historical"}}},
	}}}
	facts := string(mustJSON(selection.Candidates))
	applyJevReview(selection)
	if !reflect.DeepEqual(selection.Roles[0].Selection.SelectedIDs, []string{"high", "unanswered", "low"}) {
		t.Fatal("partial ranking lost or displaced unanswered source")
	}
	if string(mustJSON(selection.Candidates)) != facts {
		t.Fatal("review changed original facts")
	}
	if selection.JevReview.Items[1].Application != "original_selection_retained" {
		t.Fatal("unanswered trace mislabeled")
	}
}

func TestJevReviewRetainsAcceptedRoundInputPassages(t *testing.T) {
	input := map[string]any{"recent_conversation_reading": []map[string]any{
		{"Source": "recent_conversation_turn", "Text": "user:\nContinue.\n\nassistant:\nMira reaches the archive."},
		{"Source": "recent_conversation_turn", "Text": "user:\nKeep Rowan's knowledge private.\n\nassistant:\n\n" + strings.Repeat("The archive key was returned. ", 40) + "\n\n" + strings.Repeat("Unrelated festival details. ", 40)},
	}}
	selected, discarded := []string{}, []string{}
	for _, p := range multiAgentRecentPassages(input["recent_conversation_reading"])[1]["Text"].([]map[string]any) {
		if strings.Contains(stringFromMap(p, "text"), "key was returned") {
			selected = append(selected, stringFromMap(p, "ref"))
		}
		if strings.Contains(stringFromMap(p, "text"), "festival details") {
			discarded = append(discarded, stringFromMap(p, "ref"))
		}
	}
	if len(selected) == 0 || len(discarded) == 0 {
		t.Fatal("fixture lacks distinct context passages")
	}
	role := multiAgentRoleResult{SelectionRound: 2, Calls: []multiAgentCall{
		{Round: 1, Result: multiAgentRecommendation{RecentContextRefs: &discarded}},
		{Round: 2, Input: map[string]any{"previous_result": multiAgentRecommendation{RecentContextRefs: &selected}}},
	}}
	before := string(mustJSON([]any{input, role}))
	got := string(mustJSON(jevReviewReading(input, role)))
	if !strings.Contains(got, "key was returned") || !strings.Contains(got, "Rowan's knowledge") || strings.Contains(got, "festival details") {
		t.Fatal("review did not use the accepted call's actual input passages")
	}
	if string(mustJSON([]any{input, role})) != before {
		t.Fatal("review changed source context or recommendations")
	}
	// Explicit final choices replace the previous reading, including an empty
	// list. Failed/discarded rounds never contribute extra passages.
	empty := []string{}
	role.Selection.RecentContextRefs = &empty
	got = string(mustJSON(jevReviewReading(input, role)))
	if strings.Contains(got, "key was returned") || strings.Contains(got, "festival details") || !strings.Contains(got, "Mira reaches") {
		t.Fatal("explicit final context selection was not respected")
	}
	role.Selection.RecentContextRefs = &discarded
	got = string(mustJSON(jevReviewReading(input, role)))
	if strings.Contains(got, "key was returned") || !strings.Contains(got, "festival details") {
		t.Fatal("previous reading replaced the final explicit reading")
	}
}

func TestJevReviewIncludesGoBaselineAfterSpecialistFailure(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
	}))
	defer llm.Close()
	var seen atomic.Int32
	jeV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			State struct {
				Items []map[string]any `json:"items"`
			} `json:"state"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.State.Items) != 1 || stringFromMap(body.State.Items[0], "id") != "baseline" {
			t.Error("Go baseline not reviewed", body)
			return
		}
		seen.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"review_0_rank": map[string]any{"type": "score", "score": 2}}})
	}))
	defer jeV.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for name, role := range cfg.Roles {
		role.Enabled = name == "unresolved_goal"
		role.Provider = "custom"
		role.Endpoint = llm.URL
		role.APIKey = "fixture"
		role.Model = "fixture"
		cfg.Roles[name] = role
	}
	cfg.Jev = jevSettings{Enabled: true, Endpoint: jeV.URL, Model: "fixture", APIKey: "fixture"}
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "baseline", Lane: "unresolved_goal", CompleteText: "The bridge repair is unfinished."}, {CanonicalFactID: "unused", Lane: "unresolved_goal", CompleteText: "Unselected old goal"}}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, nil, map[string]any{"go_baseline_plan": map[string]any{"selected_fact_ids": []string{"baseline"}}})
	if seen.Load() != 1 || !multiAgentWants(result, "unresolved_goal", "baseline", false) || multiAgentWants(result, "unresolved_goal", "unused", false) {
		t.Fatal("review failed to apply already-selected Go baseline", result)
	}
}

func TestJevReviewAppliesInterpretationAndRetainsSourceOnOutage(t *testing.T) {
	var llmCalls, jevCalls atomic.Int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"selected_ids":["key"],"reasons":{"key":"Mira has the key."},"search_requests":[]}`}}}})
	}))
	defer llm.Close()
	jeV := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jevCalls.Add(1)
		if llmCalls.Load() == 0 {
			t.Error("review ran before preprocessing")
		}
		var payload struct {
			State     map[string]any `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if !strings.Contains(fmt.Sprint(payload.State), "Mira has the key.") {
			t.Error("editor claim missing")
		}
		answers := map[string]any{}
		for key := range payload.Questions {
			choice := "contradicted"
			if strings.HasSuffix(key, "_time") {
				choice = "historical"
			}
			if strings.HasSuffix(key, "_knowledge") {
				choice = "owner_scoped"
			}
			answers[key] = map[string]any{"type": "choice", "choice": choice, "confidence": 0.99}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "answers": answers})
	}))
	defer jeV.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Enabled = role == "character_objective"
		c.Provider = "custom"
		c.Endpoint = llm.URL
		c.Model = "fixture"
		c.APIKey = "fixture-llm"
		cfg.Roles[role] = c
	}
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "key", Lane: "character_objective", CompleteText: "Mira returned the key to Rowan."}}
	s := &Server{}
	baseline := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, nil)
	cfg.Jev = jevSettings{Enabled: true, Endpoint: jeV.URL, Model: "fixture", APIKey: "test-jev"}
	result := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, nil)
	if reflect.DeepEqual(result.Roles[0].Selection, baseline.Roles[0].Selection) || !strings.Contains(result.Roles[0].Selection.Reasons["key"], "support=contradicted") {
		t.Fatal("review did not qualify the mistaken editor interpretation")
	}
	if jevCalls.Load() != 1 || result.JevReview == nil || len(result.JevReview.Items) != 1 {
		t.Fatal("review missing")
	}
	if result.JevReview.Items[0].Answers["support"].Choice != "contradicted" {
		t.Fatal("verdict not exposed")
	}
	a := prepareTurnInjectionAssembly{Preprocessing: baseline}
	b := prepareTurnInjectionAssembly{Preprocessing: result}
	pa := buildPrepareTurnPriorityMemoryDeliveryPlan(&a, 2000, 5, "auto", nil, testPrepareTurnMemorySelectionContext(nil))
	pb := buildPrepareTurnPriorityMemoryDeliveryPlan(&b, 2000, 5, "auto", nil, testPrepareTurnMemorySelectionContext(nil))
	if pa["final_text"] != pb["final_text"] {
		t.Fatal("review changed delivered memory")
	}
	cfg.Jev.Endpoint = "http://127.0.0.1:1"
	failed := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, nil)
	if !reflect.DeepEqual(failed.Roles[0].Selection, baseline.Roles[0].Selection) || failed.JevReview.Calls[0].Error == "" {
		t.Fatal("review outage did not retain LLM result")
	}
}

func TestJevOffAndFailureRetainDefault(t *testing.T) {
	s := &Server{}
	cfg := defaultMultiAgentSettings()
	if s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, nil, nil, 2000, 5, nil, nil) != nil {
		t.Fatal("both off ran preprocessing")
	}
	cfg.Jev.Enabled = true
	result := s.runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{RawUserInput: strPtr("Enter the archive.")}, []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "key", Lane: "character_objective", CompleteText: "Mira has a key."}}, nil, 2000, 5, nil, nil)
	for _, role := range result.Roles {
		if role.Source != "go_default" {
			t.Fatal("missing key replaced Go result")
		}
	}
	result.captureBaseline(map[string]any{"selected_fact_ids": []string{"key"}})
	if !multiAgentWants(result, "character_objective", "key", false) {
		t.Fatal("default memory was lost")
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "echo fixture-secret", 401) }))
	defer provider.Close()
	eval := evaluateJev(context.Background(), jevSettings{Endpoint: provider.URL, Model: "fixture", APIKey: "fixture-secret"}, "synthetic", map[string]any{"q": defaultJevSettings().question("search", 0, "event_recent")})
	b, _ := json.Marshal(eval)
	if eval.Error != "jev_http_401" || strings.Contains(string(b), "fixture-secret") {
		t.Fatal("error code or redaction failed")
	}
}

func TestJevPrimaryRegisteredPrepareTurnToLorebookPayload(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	var calls atomic.Int32
	provider := jevFixtureProvider(t, &calls, false)
	defer provider.Close()
	srv := setupTestServer()
	srv.Store = &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore(), current: &store.LorebookReferenceCurrent{ScopeID: 12, Entries: []store.LorebookReferenceEntryObservation{
		{HostEntryID: "festival", EntryOrdinal: 0, Key: "archive", Content: "The archive hosts an autumn festival each year."},
		{HostEntryID: "key", EntryOrdinal: 1, Key: "archive", Content: "The archive key opens the bronze entrance door."},
	}}}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	settings := fmt.Sprintf(`{"enabled":false,"jev":{"enabled":true,"endpoint":%q,"model":"fixture","api_key":"test-jev"}}`, provider.URL)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/config/memory-preprocessing", strings.NewReader(settings)))
	if rec.Code != http.StatusOK {
		t.Fatal("settings route", rec.Body.String())
	}
	_, response := prepareTurnPerfRequest(t, srv, `{"chat_session_id":"jev-lore-http","raw_user_input":"Open the archive door","response_projection":"prepare_turn.production_compact.v1","lorebook_reference_scope":{"contract_version":"lorebook_reference_scope.v1","observation_state":"observed","character_index":1,"chat_index":2,"enabled_module_ids":[],"enabled_modules_observed":true},"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist","lorebook_reference_max_chars":90}}`)
	lore := mapFromAny(response["lorebook_reference"])
	if calls.Load() == 0 || lore["selection_source"] != "jev" {
		t.Fatal("preprocessing OFF did not reach Jev", calls.Load(), lore)
	}
	if intFromAny(lore["delivery_count"], 0) != 1 || intFromAny(lore["budget_deferred_count"], 0) != 1 || intFromAny(lore["used_chars"], 0) > intFromAny(lore["budget_chars"], 0) {
		t.Fatal("ranking bypassed lorebook budget", lore)
	}
	plan, _ := json.Marshal(response["payload_application_plan"])
	if !bytes.Contains(plan, []byte("key opens the bronze")) || bytes.Contains(plan, []byte("autumn festival")) {
		t.Fatal("ranked source did not reach final payload", string(plan))
	}
}

func TestJevReviewRegisteredPrepareTurnToLorebookPayload(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body struct {
			State struct {
				Items []map[string]any `json:"items"`
			} `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		answers := map[string]any{}
		for i, item := range body.State.Items {
			score := 0.0
			if strings.Contains(stringFromMap(item, "text"), "bronze entrance") {
				score = 3
			}
			answers[fmt.Sprintf("review_%d_rank", i)] = map[string]any{"type": "score", "score": score}
			for k, v := range map[string]string{"support": "supported", "time": "current", "knowledge": "unknown"} {
				answers[fmt.Sprintf("review_%d_%s", i, k)] = map[string]any{"type": "choice", "choice": v}
			}
		}
		if len(answers) != len(body.Questions) {
			t.Error("unexpected provider questions")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer provider.Close()
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		refs := []string{}
		for _, raw := range outputFidelityLineageSlice(body["messages"]) {
			var input map[string]any
			_ = json.Unmarshal([]byte(stringFromMap(mapFromAny(raw), "content")), &input)
			for _, candidate := range outputFidelityLineageSlice(input["lorebook_candidates"]) {
				refs = append(refs, stringFromMap(mapFromAny(candidate), "ref"))
			}
		}
		if len(refs) != 2 {
			t.Error("LLM did not receive source candidates", refs)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(mustJSON(map[string]any{"selected_lorebook_refs": refs, "search_requests": []string{}}))}}}})
	}))
	defer llm.Close()
	srv := setupTestServer()
	srv.Store = &prepareTurnLorebookReferenceStore{Store: store.NewNoopStore(), current: &store.LorebookReferenceCurrent{ScopeID: 12, Entries: []store.LorebookReferenceEntryObservation{
		{HostEntryID: "festival", EntryOrdinal: 0, Key: "archive", Content: "The archive hosts an autumn festival each year."},
		{HostEntryID: "key", EntryOrdinal: 1, Key: "archive", Content: "The archive key opens the bronze entrance door."},
	}}}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	roles := map[string]any{}
	for _, name := range multiAgentRoles {
		roles[name] = map[string]any{"enabled": name == "world_state", "provider": "custom", "endpoint": llm.URL, "model": "fixture", "api_key": "fixture"}
	}
	settings := string(mustJSON(map[string]any{"enabled": true, "roles": roles, "jev": map[string]any{"enabled": true, "endpoint": provider.URL, "model": "fixture", "api_key": "test-jev"}}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/config/memory-preprocessing", strings.NewReader(settings)))
	if rec.Code != http.StatusOK {
		t.Fatal("settings route", rec.Body.String())
	}
	_, response := prepareTurnPerfRequest(t, srv, `{"chat_session_id":"jev-review-lore-http","raw_user_input":"Open the archive door","response_projection":"prepare_turn.production_compact.v1","lorebook_reference_scope":{"contract_version":"lorebook_reference_scope.v1","observation_state":"observed","character_index":1,"chat_index":2,"enabled_module_ids":[],"enabled_modules_observed":true},"settings":{"guide_strength":"none","lorebook_reference_mode":"reference_assist","lorebook_reference_max_chars":90}}`)
	lore := mapFromAny(response["lorebook_reference"])
	if calls.Load() == 0 || lore["selection_source"] != "jev" {
		t.Fatal("review did not reach Jev", calls.Load(), lore)
	}
	if intFromAny(lore["delivery_count"], 0) != 1 || intFromAny(lore["budget_deferred_count"], 0) != 1 || intFromAny(lore["used_chars"], 0) > intFromAny(lore["budget_chars"], 0) {
		t.Fatal("ranking bypassed lorebook budget", lore)
	}
	plan, _ := json.Marshal(response["payload_application_plan"])
	if !bytes.Contains(plan, []byte("key opens the bronze")) || bytes.Contains(plan, []byte("autumn festival")) {
		t.Fatal("ranked source did not reach final payload", string(plan))
	}
}

func TestJevLargeRankingBatchesQuestionsWithoutLosingSources(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var payload struct {
			State     map[string]any `json:"state"`
			Questions map[string]any `json:"questions"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
			return
		}
		stateTokens, total, longest := jevEstimatedInputTokens(payload.State), 0, 0
		for _, question := range payload.Questions {
			size := jevEstimatedInputTokens(question)
			total += size
			longest = max(longest, size)
		}
		if stateTokens+longest > 28000 || stateTokens+total > 60000 {
			t.Errorf("request exceeds packing targets: state=%d longest=%d questions=%d", stateTokens, longest, total)
		}
		if _, duplicated := payload.State["recent_conversation_reading"]; duplicated {
			t.Error("recent reading duplicated")
		}
		answers := map[string]any{}
		for id := range payload.Questions {
			if id == "search" {
				answers[id] = map[string]any{"type": "choice", "choice": "none"}
			} else {
				answers[id] = map[string]any{"type": "score", "score": 2.5}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer provider.Close()
	facts := []prepareTurnPriorityMemoryCandidate{}
	for i := 0; i < 300; i++ {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: fmt.Sprintf("source-%d", i), Lane: "event_recent", CompleteText: fmt.Sprintf("Mira catalogued the archive shelf numbered %d.", i)})
	}
	cfg := defaultMultiAgentSettings()
	cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "fixture"}
	selection := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 6000, 5, nil, nil)
	role := selection.role("event_recent")
	if role.Calls[0].Jev.Requests <= 1 || len(role.Selection.SelectedIDs) != len(facts) {
		t.Fatal("batching lost source coverage", role.Calls[0].Jev.Requests, len(role.Selection.SelectedIDs))
	}
	if selection.AnalysisCalls != int(calls.Load()) {
		t.Fatal("analysis count lost batch calls", selection.AnalysisCalls, calls.Load())
	}
}

func TestJevIndependentSavePreservesPreprocessingAndViceVersa(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	s := setupTestServer()
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	put := func(body string) multiAgentSettings {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		cfg, err := s.loadMultiAgentSettings()
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	before := put(`{"enabled":true,"candidate_chars":12345,"shared_prompt":"retain shared prompt","roles":{"character_objective":{"enabled":true,"model":"custom-role-model","api_key":"role-secret","prompt":"retain role prompt"}},"jev":{"enabled":false,"endpoint":"https://fixture.invalid/systemone","model":"fixture-jev","api_key":"jev-secret"}}`)
	for _, enabled := range []bool{true, false} {
		got := put(fmt.Sprintf(`{"jev":{"enabled":%t,"endpoint":"https://fixture.invalid/systemone","model":"fixture-jev"}}`, enabled))
		if got.Enabled != before.Enabled || got.CandidateChars != before.CandidateChars || got.SharedPrompt != before.SharedPrompt || !reflect.DeepEqual(got.Roles, before.Roles) {
			t.Fatal("Jev page reset preprocessing settings")
		}
		if got.Jev.Enabled != enabled || got.Jev.APIKey != before.Jev.APIKey {
			t.Fatal("Jev-only save lost toggle or key")
		}
	}
	jev := put(`{"jev":{"enabled":true,"endpoint":"https://fixture.invalid/systemone","model":"fixture-jev"}}`).Jev
	got := put(`{"enabled":false}`)
	if got.Enabled || got.effectiveMode() != "jev_primary" || !reflect.DeepEqual(got.Jev, jev) {
		t.Fatal("preprocessing OFF changed Jev")
	}
	if got.CandidateChars != before.CandidateChars || got.SharedPrompt != before.SharedPrompt || !reflect.DeepEqual(got.Roles, before.Roles) {
		t.Fatal("toggle-only save reset settings")
	}
}

func TestJevPromptEditingPersistsAndReachesBothModes(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	custom := map[string]jevQuestionPrompt{}
	criterionKeys := map[string]string{"rank": "3", "search": "cause", "support": "supported", "time": "historical", "knowledge": "owner_scoped"}
	for key, criterion := range criterionKeys {
		custom[key] = jevQuestionPrompt{Instructions: "편집 " + key + ": {item} / {role_focus}\n<tag> & 100%", Criteria: map[string]string{criterion: "기준 " + key + " {item}"}}
	}
	var seenMu sync.Mutex
	seen := map[string]bool{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-jev-secret" {
			t.Error("saved key was lost")
		}
		var payload struct {
			State     map[string]any            `json:"state"`
			Questions map[string]map[string]any `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		answers := map[string]any{}
		for id, question := range payload.Questions {
			key, index := "search", 0
			if strings.HasPrefix(id, "rank_") {
				key = "rank"
				fmt.Sscanf(id, "rank_%d", &index)
			} else if strings.HasPrefix(id, "review_") {
				fmt.Sscanf(id, "review_%d_", &index)
				key = id[strings.LastIndex(id, "_")+1:]
			}
			seenMu.Lock()
			seen[key] = true
			seenMu.Unlock()
			role := extractionStringFromAny(payload.State["role"])
			want := fmt.Sprintf("편집 %s: items[%d] / %s\n<tag> & 100%%", key, index, jevRoleFocus[role])
			if key != "search" {
				shared := mapFromAny(payload.State["question_instructions"])
				if shared[key] != strings.ReplaceAll(custom[key].Instructions, "{role_focus}", jevRoleFocus[role]) {
					t.Errorf("edited shared instruction lost in %s", id)
				}
				want = fmt.Sprintf("Apply `question_instructions.%s`, replacing `{item}` with `items[%d]`.", key, index)
			}
			if question["instructions"] != want {
				t.Errorf("saved instructions missing in %s: %v", id, question["instructions"])
			}
			var gotCriterion any
			if key == "rank" {
				gotCriterion = question["criteria"].([]any)[3]
				answers[id] = map[string]any{"type": "score", "score": 2.5}
			} else {
				gotCriterion = mapFromAny(question["criteria"])[criterionKeys[key]]
				choice := criterionKeys[key]
				if key == "search" {
					choice = "none"
				}
				answers[id] = map[string]any{"type": "choice", "choice": choice}
			}
			if gotCriterion != fmt.Sprintf("기준 %s items[%d]", key, index) {
				t.Errorf("saved criterion missing in %s: %v", id, gotCriterion)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
	}))
	defer provider.Close()
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"selected_ids":["F1"],"reasons":{"F1":"The key grants entry."}}`}}}})
	}))
	defer llm.Close()
	s := setupTestServer()
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	put := func(payload any) map[string]any {
		t.Helper()
		body, _ := json.Marshal(payload)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(body)))
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "fixture-jev-secret") {
			t.Fatal("settings save/redaction failed", rec.Code)
		}
		var view map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &view)
		return view
	}
	cfg := defaultMultiAgentSettings()
	cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "fixture-jev-secret"}
	for role, c := range cfg.Roles {
		c.Enabled, c.Provider, c.Endpoint, c.Model = role == "character_objective", "custom", llm.URL, "fixture"
		c.APIKey = "fixture-llm"
		cfg.Roles[role] = c
	}
	put(cfg)
	view := put(map[string]any{"jev": map[string]any{"prompts": custom}})
	if len(view["jev_prompts"].([]any)) != len(custom) {
		t.Fatal("prompt editors missing")
	}
	restarted := &Server{}
	loaded, err := restarted.loadMultiAgentSettings()
	if err != nil || !reflect.DeepEqual(loaded.Jev.Prompts, custom) || loaded.Jev.APIKey != cfg.Jev.APIKey || !loaded.Jev.Enabled || loaded.Jev.Endpoint != provider.URL {
		t.Fatal("prompt-only save or restart lost settings", err)
	}
	facts := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "key", Lane: "character_objective", CompleteText: "Mira carries the archive key."},
		{CanonicalFactID: "coat", Lane: "character_objective", CompleteText: "Mira wears a red coat."},
	}
	for _, enabled := range []bool{false, true} {
		put(map[string]any{"enabled": enabled})
		loaded, _ = s.loadMultiAgentSettings()
		result := s.runMultiAgent(context.Background(), loaded, dto.PrepareTurnRequest{RawUserInput: strPtr("Mira enters the archive.")}, facts, nil, 2000, 5, nil, nil)
		if result == nil || (enabled && result.JevReview == nil) {
			t.Fatal("expected mode did not run")
		}
	}
	for key := range custom {
		if !seen[key] {
			t.Error("question never dispatched:", key)
		}
	}
	// A connection-only save and an empty prompt patch retain all wording.
	put(map[string]any{"jev": map[string]any{"model": "fixture-next", "prompts": map[string]any{}}})
	loaded, _ = s.loadMultiAgentSettings()
	if !reflect.DeepEqual(loaded.Jev.Prompts, custom) {
		t.Fatal("connection save erased prompts")
	}
	// Reset one whole question without changing other questions or either feature.
	put(map[string]any{"jev": map[string]any{"prompts": map[string]any{"rank": map[string]any{}}}})
	loaded, _ = s.loadMultiAgentSettings()
	if _, ok := loaded.Jev.Prompts["rank"]; ok {
		t.Fatal("reset retained rank override")
	}
	delete(custom, "rank")
	if !reflect.DeepEqual(loaded.Jev.Prompts, custom) || !loaded.Enabled || !loaded.Jev.Enabled {
		t.Fatal("reset changed other settings")
	}
	if !reflect.DeepEqual(loaded.Jev.question("rank", 1, "character_objective"), defaultJevSettings().question("rank", 1, "character_objective")) {
		t.Fatal("reset did not restore default")
	}
}

func TestJevPromptDefaultsAndLongEditedQuestions(t *testing.T) {
	defaults := defaultJevSettings()
	before, _ := json.Marshal(defaults.promptViews())
	for _, definition := range jevPromptDefinitions {
		criteria := map[string]string{}
		for _, c := range definition.Criteria {
			criteria[c.Key] = c.Text
		}
		saved := applyJevSettingsUpdate(defaults, &jevSettingsUpdate{Prompts: map[string]jevQuestionPrompt{definition.Key: {Instructions: definition.Instructions, Criteria: criteria}}})
		if len(saved.Prompts) != 0 {
			t.Fatal("default text persisted as an override")
		}
		blank := applyJevSettingsUpdate(saved, &jevSettingsUpdate{Prompts: map[string]jevQuestionPrompt{definition.Key: {Instructions: " \n", Criteria: map[string]string{definition.Criteria[0].Key: " \n"}}}})
		if !reflect.DeepEqual(blank.question(definition.Key, 2, "event_recent"), defaults.question(definition.Key, 2, "event_recent")) {
			t.Fatal("blank text did not use defaults")
		}
	}
	input := map[string]any{"role": "event_recent", "current_input": "Mira enters the archive."}
	items := []jevCandidate{{ID: "a", Value: map[string]any{"text": "Mira carries a key."}}, {ID: "b", Value: map[string]any{"text": "Rowan follows Mira."}}, {ID: "c", Value: map[string]any{"text": "The archive is open."}}}
	for _, review := range []bool{false, true} {
		key := "rank"
		if review {
			key = "support"
		}
		cfg := defaults
		cfg.Prompts = map[string]jevQuestionPrompt{key: {Instructions: strings.Repeat("Long edited instruction. ", 2100)}}
		// Isolate the long override from unrelated shipped wording. This checks
		// shared versus repeated instructions, not the length of default rubrics.
		for _, definition := range jevPromptDefinitions {
			if definition.Key != key && definition.Key != "search" {
				cfg.Prompts[definition.Key] = jevQuestionPrompt{Instructions: "Assess {item} using the supplied evidence."}
			}
		}
		if len(jevCandidateBatches(defaults, items, input, review)) != 1 || len(jevCandidateBatches(cfg, items, input, review)) != 1 {
			t.Fatal("shared edited instruction counted more than once", review)
		}
		// Choice descriptions and score levels remain per question and must still
		// be counted repeatedly. A large shared instruction occupies state once.
		cfg.Prompts[key] = jevQuestionPrompt{Instructions: strings.Repeat("Long edited instruction. ", 4000)}
		if len(jevCandidateBatches(cfg, items, input, review)) != 1 {
			t.Fatal("oversized shared instruction was repeated per source", review)
		}
		criterion := "3"
		if review {
			criterion = "supported"
		}
		cfg.Prompts[key] = jevQuestionPrompt{Criteria: map[string]string{criterion: strings.Repeat("Long edited criterion. ", 3000)}}
		if len(jevCandidateBatches(cfg, items, input, review)) <= 1 {
			t.Fatal("packing ignored repeated edited criterion", review)
		}
	}
	after, _ := json.Marshal(defaults.promptViews())
	if !bytes.Equal(before, after) {
		t.Fatal("editing mutated shared default definitions")
	}
}

func TestJevLongKoreanContextSharesRequestsAndKeepsAnswerMapping(t *testing.T) {
	// The former estimate exceeded the shared-state budget before a second
	// candidate could be packed, reproducing one HTTP call per source.
	reading := strings.Repeat("미라는 서고의 문을 열었다. Mira opened the archive.\n", 1000)
	encoded, _ := json.Marshal(reading)
	if len(encoded) <= 28000 {
		t.Fatal("fixture must reproduce the former common-context byte overflow")
	}
	for _, review := range []bool{false, true} {
		t.Run(fmt.Sprintf("review=%t", review), func(t *testing.T) {
			cfg := defaultMultiAgentSettings()
			cfg.CandidateChars = 100000
			selection := &multiAgentSelection{}
			req := dto.PrepareTurnRequest{RawUserInput: strPtr("서고에 들어가서 열쇠를 확인한다.")}
			inputContext := map[string]any{"recent_conversation_reading": reading}
			expected := map[string]map[string]any{}
			scores := map[string]float64{}
			for _, role := range multiAgentRoles {
				result := multiAgentRoleResult{Role: role, Selection: multiAgentRecommendation{Reasons: map[string]string{}}}
				for i := 0; i < 36; i++ {
					id := fmt.Sprintf("%s-%d", role, i)
					selection.Candidates = append(selection.Candidates, prepareTurnPriorityMemoryCandidate{
						CanonicalFactID: id, Lane: role, CompleteText: fmt.Sprintf("Source %d: %s", i, strings.Repeat("미라가 보관한 열쇠의 소유자와 당시의 기록. ", 12)),
					})
					result.Selection.SelectedIDs = append(result.Selection.SelectedIDs, id)
					result.Selection.Reasons[id] = "이 기록은 서고 열쇠의 이전 소유자를 설명한다."
					scores[id] = float64(i % 4)
				}
				selection.Roles = append(selection.Roles, result)
			}
			inputs := map[string]map[string]any{}
			for _, role := range selection.Roles {
				input := multiAgentInput(role.Role, selection.Candidates, nil, req, cfg, 100000, 5, nil, inputContext)
				inputs[role.Role] = input
				for _, item := range jevCandidates(input) {
					value := map[string]any{}
					for k, v := range item.Value {
						value[k] = v
					}
					if review {
						value["editor_interpretation"] = role.Selection.Reasons[item.ID]
					}
					expected[item.ID] = value
				}
			}
			var mu sync.Mutex
			seen, callCounts := map[string]int{}, map[string]int{}
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					State struct {
						Role         string            `json:"role"`
						Recent       string            `json:"recent_conversation"`
						Current      string            `json:"current_input"`
						Items        []map[string]any  `json:"items"`
						Instructions map[string]string `json:"question_instructions"`
					} `json:"state"`
					Questions map[string]any `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					return
				}
				if payload.State.Recent != reading || payload.State.Current != *req.RawUserInput {
					t.Error("batch changed shared context")
				}
				// Inspect the actual dispatched representation, not just a helper's
				// output: each applicable full instruction occurs once, and every
				// item retains a question with an explicit target and full criteria.
				wantDefinitions := 1
				if review {
					wantDefinitions = 4
				}
				if len(payload.State.Instructions) != wantDefinitions {
					t.Error("shared instruction definitions missing or wrong mode")
				}
				for _, definition := range jevPromptDefinitions {
					if definition.Key == "search" || (!review && definition.Key != "rank") {
						continue
					}
					want := strings.ReplaceAll(definition.Instructions, "{role_focus}", jevRoleFocus[payload.State.Role])
					if payload.State.Instructions[definition.Key] != want {
						t.Error("shared instruction changed", definition.Key)
					}
				}
				for id, value := range payload.Questions {
					if id == "search" {
						continue
					}
					question := mapFromAny(value)
					instruction := extractionStringFromAny(question["instructions"])
					key, index := "rank", 0
					if review {
						fmt.Sscanf(id, "review_%d_", &index)
						key = id[strings.LastIndex(id, "_")+1:]
					} else {
						fmt.Sscanf(id, "rank_%d", &index)
					}
					if !strings.Contains(instruction, "`question_instructions."+key+"`") || !strings.Contains(instruction, fmt.Sprintf("`items[%d]`", index)) || !strings.Contains(instruction, "`{item}`") {
						t.Error("question must bind shared instruction to its own item", id)
					}
					if len(instruction) >= len(payload.State.Instructions[key]) {
						t.Error("default repeated question text was not reduced", id)
					}
				}
				mu.Lock()
				defer mu.Unlock()
				callCounts[payload.State.Role]++
				answers := map[string]any{}
				for j, item := range payload.State.Items {
					id := extractionStringFromAny(item["id"])
					seen[id]++
					if !strings.HasPrefix(id, payload.State.Role+"-") {
						t.Error("batch mixed role scopes", id, payload.State.Role)
					}
					a, _ := json.Marshal(item)
					b, _ := json.Marshal(expected[id])
					if !bytes.Equal(a, b) {
						t.Error("batch changed source/linked evidence/editor interpretation", id)
					}
					questions := map[string]any{fmt.Sprintf("rank_%d", j): cfg.Jev.question("rank", j, payload.State.Role)}
					if review {
						questions = map[string]any{}
						for kind, question := range cfg.Jev.reviewQuestions(j, payload.State.Role) {
							questions[fmt.Sprintf("review_%d_%s", j, kind)] = question
						}
					}
					for key, question := range questions {
						want, _ := json.Marshal(question)
						got, _ := json.Marshal(payload.Questions[key])
						if !bytes.Equal(want, got) {
							t.Error("question lost its batch-local item index", key)
						}
						if review && !strings.HasSuffix(key, "_rank") {
							choice := "unknown"
							if strings.HasSuffix(key, "_support") && scores[id] >= 2 {
								choice = "supported"
							}
							answers[key] = map[string]any{"type": "choice", "choice": choice}
						} else {
							answers[key] = map[string]any{"type": "score", "score": scores[id]}
						}
					}
				}
				if _, ok := payload.Questions["search"]; ok {
					answers["search"] = map[string]any{"type": "choice", "choice": "none"}
				}
				if len(answers) != len(payload.Questions) {
					t.Error("unaccounted question in request")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
			}))
			defer provider.Close()
			cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "fixture"}
			s := &Server{}
			if review {
				before, _ := json.Marshal(selection)
				result := s.reviewWithJev(context.Background(), cfg, selection, req, 100000, 5, nil, inputContext, nil)
				after, _ := json.Marshal(selection)
				if !bytes.Equal(before, after) || len(result.Items) != len(expected) {
					t.Fatal("review changed selection or lost reviewed sources")
				}
				for _, item := range result.Items {
					want := "unknown"
					if scores[item.ID] >= 2 {
						want = "supported"
					}
					if item.Status != "reviewed" || item.Answers["support"].Choice != want {
						t.Error("answer mapped to the wrong original source", item.ID)
					}
				}
				for _, call := range result.Calls {
					if len(call.QuestionInstructions) != 4 {
						t.Error("review trace lost shared instructions")
					}
				}
			} else {
				for role, input := range inputs {
					call := s.callJevPrimary(context.Background(), cfg.Jev, role, 1, input)
					if call.Error != "" || len(call.Result.SelectedIDs) != 36 {
						t.Fatal("primary lost candidates", role, call.Error)
					}
					if len(call.Jev.QuestionInstructions) != 1 {
						t.Error("primary trace lost shared instructions")
					}
					previous := 4.0
					for _, id := range call.Result.SelectedIDs {
						if scores[id] > previous {
							t.Error("batched score mapped to wrong original source", id)
						}
						previous = scores[id]
					}
				}
			}
			for id := range expected {
				if seen[id] != 1 {
					t.Error("candidate duplicated or missing", id, seen[id])
				}
			}
			for _, role := range multiAgentRoles {
				t.Logf("role=%s sources=%d physical_requests=%d", role, 36, callCounts[role])
				if callCounts[role] < 2 || callCounts[role] >= 36/2 {
					t.Error("long context must pack multiple items and exercise multiple batches", role, callCounts[role])
				}
			}
		})
	}
}

func TestJevOversizedSourceAndContextAreNeverTrimmed(t *testing.T) {
	for _, field := range []string{"context", "source", "question"} {
		t.Run(field, func(t *testing.T) {
			cfg := defaultJevSettings()
			input := map[string]any{"role": "event_recent", "current_input": "Open the archive."}
			items := []jevCandidate{{ID: "large", Value: map[string]any{"text": "Mira has a key."}}, {ID: "small", Value: map[string]any{"text": "Rowan waits."}}}
			large := strings.Repeat("긴 기록 Long source. ", 12000)
			switch field {
			case "context":
				input["recent_conversation"] = large
			case "source":
				items[0].Value["text"] = large
			case "question":
				cfg.Prompts = map[string]jevQuestionPrompt{"rank": {Instructions: large}, "support": {Instructions: large}}
			}
			for _, review := range []bool{false, true} {
				before, _ := json.Marshal([]any{input, items, cfg})
				batches := jevCandidateBatches(cfg, items, input, review)
				after, _ := json.Marshal([]any{input, items, cfg})
				want := [][]jevCandidate{items}
				if field == "source" {
					want = [][]jevCandidate{items[:1], items[1:]}
				}
				if !reflect.DeepEqual(batches, want) || !bytes.Equal(before, after) {
					t.Fatal("oversized input was changed or shared content was repeated per source", review)
				}
			}
		})
	}
}

func TestJevSharedContextOverflowDispatchesRoleOnce(t *testing.T) {
	for _, review := range []bool{false, true} {
		for _, repeats := range []int{1200, 2000} {
			if !review && repeats == 1200 {
				continue
			} // Near-target review includes three extra questions.
			for _, status := range []int{http.StatusOK, http.StatusUnprocessableEntity} {
				t.Run(fmt.Sprintf("review=%t/repeats=%d/status=%d", review, repeats, status), func(t *testing.T) {
					cfg := defaultMultiAgentSettings()
					cfg.CandidateChars = 100000
					role := multiAgentRoleResult{Role: "character_objective", Selection: multiAgentRecommendation{Reasons: map[string]string{}}}
					selection := &multiAgentSelection{}
					for i := 0; i < 36; i++ {
						id := fmt.Sprintf("source-%d", i)
						selection.Candidates = append(selection.Candidates, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: role.Role, CompleteText: fmt.Sprintf("Source %d: %s", i, strings.Repeat("미라가 보관한 열쇠의 소유자와 당시의 기록. ", 12))})
						role.Selection.SelectedIDs = append(role.Selection.SelectedIDs, id)
					}
					selection.Roles = []multiAgentRoleResult{role}
					reading := strings.Repeat("미라는 서고의 문을 열었다. Mira opened the archive.\n", repeats)
					inputContext := map[string]any{"recent_conversation_reading": reading}
					req := dto.PrepareTurnRequest{RawUserInput: strPtr("Mira checks the archive key.")}
					calls := 0
					provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls++
						var payload struct {
							State     map[string]any
							Questions map[string]any
						}
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Error(err)
							return
						}
						if payload.State["recent_conversation"] != reading || len(outputFidelityLineageSlice(payload.State["items"])) != len(selection.Candidates) {
							t.Error("shared context or candidate set was split or trimmed")
						}
						if status != http.StatusOK {
							w.WriteHeader(status)
							return
						}
						answers := map[string]any{}
						for id, question := range payload.Questions {
							if mapFromAny(question)["type"] == "score" {
								answers[id] = map[string]any{"type": "score", "score": 2}
							} else {
								choice := "unknown"
								if id == "search" {
									choice = "none"
								}
								answers[id] = map[string]any{"type": "choice", "choice": choice}
							}
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
					}))
					defer provider.Close()
					cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "synthetic"}
					if review {
						selection.JevReview = (&Server{}).reviewWithJev(context.Background(), cfg, selection, req, 100000, 5, nil, inputContext, nil)
						applyJevReview(selection)
						if len(selection.JevReview.Items) != len(role.Selection.SelectedIDs) || !reflect.DeepEqual(selection.Roles[0].Selection.SelectedIDs, role.Selection.SelectedIDs) {
							t.Fatal("review lost original source mappings or selection")
						}
						for _, item := range selection.JevReview.Items {
							if status == http.StatusOK && item.Status != "reviewed" {
								t.Error("missing review result", item.ID)
							}
							if status != http.StatusOK && item.Application != "original_selection_retained" {
								t.Error("provider failure changed selection", item.ID)
							}
						}
					} else {
						input := multiAgentInput(role.Role, selection.Candidates, nil, req, cfg, 100000, 5, nil, inputContext)
						out := (&Server{}).callJevPrimary(context.Background(), cfg.Jev, role.Role, 1, input)
						if status == http.StatusOK && len(out.Result.SelectedIDs) != len(selection.Candidates) {
							t.Fatal("ranking lost candidates")
						}
						if status != http.StatusOK && out.Error != "jev_http_422" {
							t.Fatal("provider failure was not reported", out.Error)
						}
					}
					if calls != 1 {
						t.Fatalf("shared context caused %d requests; want one role attempt without singleton retry", calls)
					}
				})
			}
		}
	}
}

func TestJevReviewCandidatesShareOnlyIdenticalScopedInput(t *testing.T) {
	base := jevCandidate{ID: "a", Ref: "F1", Group: "candidates", Value: map[string]any{
		"id": "a", "ref": "F1", "text": "Mira holds the key.", "source_ref": "record:1", "source_table": "memories",
		"source_turn": 3, "visibility": "owner_private", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira"},
		"current_state": map[string]any{"value": "held", "date": "1400-03-02"}, "editor_interpretation": "Mira can open the door.",
		"context_refs": []string{"F-alias"},
	}}
	copyItem := func(id string) jevCandidate {
		item := base
		item.ID, item.Ref = id, "F-"+id
		item.Value = map[string]any{}
		for k, v := range base.Value {
			item.Value[k] = v
		}
		item.Value["id"], item.Value["ref"] = item.ID, item.Ref
		return item
	}
	items := []jevCandidate{base, copyItem("alias")}
	// Each changed non-identifier field must remain independently reviewable.
	for key := range base.Value {
		if key == "id" || key == "ref" {
			continue
		}
		item := copyItem(key)
		item.Value[key] = "a different value"
		items = append(items, item)
	}
	group := copyItem("other-group")
	group.Group = "lorebook_candidates"
	items = append(items, group)
	before, _ := json.Marshal(items)
	unique, indexes := jevReviewCandidates(items)
	after, _ := json.Marshal(items)
	if !bytes.Equal(before, after) || len(unique) != len(items)-1 || len(indexes) != len(items) || indexes[1] != indexes[0] {
		t.Fatal("identical input not shared or original changed")
	}
	if !reflect.DeepEqual(unique[0].Value["review_aliases"], []map[string]string{{"id": items[1].ID, "ref": items[1].Ref}}) {
		t.Fatal("representative lost names referenced by other records")
	}
	for i := 2; i < len(items); i++ {
		if unique[indexes[i]].ID != items[i].ID {
			t.Error("different input reused another judgment", items[i].ID)
		}
	}
	bad := copyItem("unsupported")
	bad.Value["unsupported"] = make(chan int)
	unique, indexes = jevReviewCandidates([]jevCandidate{bad, bad})
	if len(unique) != 2 || indexes[0] == indexes[1] {
		t.Fatal("encoding failure silently merged inputs")
	}
}

func TestJevReviewDedupRestoresOriginalItemsAndFailureStatus(t *testing.T) {
	for _, reply := range []string{"complete", "partial", "failed"} {
		t.Run(reply, func(t *testing.T) {
			cfg := defaultMultiAgentSettings()
			cfg.CandidateChars = 200000
			selection := &multiAgentSelection{}
			expected := map[string]string{}
			for _, role := range []string{"character_objective", "world_state"} {
				r := multiAgentRoleResult{Role: role, Selection: multiAgentRecommendation{Reasons: map[string]string{}}}
				for _, suffix := range []string{"first", "different", "alias", "interpretation"} {
					id := role + "-" + suffix
					text := strings.Repeat("Mira holds the archive key. ", 800)
					if suffix == "different" {
						text = strings.Repeat("Rowan returned the map. ", 1500)
					}
					selection.Candidates = append(selection.Candidates, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: role, CompleteText: text, SourceRef: "record:1", SourceTurn: 3})
					r.Selection.SelectedIDs = append(r.Selection.SelectedIDs, id)
					r.Selection.Reasons[id] = "This is recorded evidence."
					if suffix == "interpretation" {
						r.Selection.Reasons[id] = "A different interpretation."
					}
					expected[id] = id
					if suffix == "alias" {
						expected[id] = role + "-first"
					}
				}
				selection.Roles = append(selection.Roles, r)
			}
			var mu sync.Mutex
			seen := map[string]int{}
			questions, requests := 0, 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					State struct {
						Role  string           `json:"role"`
						Items []map[string]any `json:"items"`
					} `json:"state"`
					Questions map[string]any `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				requests++
				questions += len(body.Questions)
				answers := map[string]any{}
				for j, item := range body.State.Items {
					id := extractionStringFromAny(item["id"])
					seen[id]++
					if expected[id] != id || !strings.HasPrefix(id, body.State.Role+"-") {
						t.Error("duplicate or wrong role dispatched", id)
					}
					if strings.HasSuffix(id, "first") {
						aliases := outputFidelityLineageSlice(item["review_aliases"])
						if len(aliases) != 1 || extractionStringFromAny(mapFromAny(aliases[0])["id"]) != body.State.Role+"-alias" {
							t.Error("wire lost original alias identity")
						}
					}
					for kind := range cfg.Jev.reviewQuestions(j, body.State.Role) {
						key := fmt.Sprintf("review_%d_%s", j, kind)
						if _, ok := body.Questions[key]; !ok {
							t.Error("missing question", key)
						}
						if reply == "partial" && kind == "knowledge" {
							continue
						}
						choice := "unknown"
						if kind == "support" && strings.HasSuffix(id, "first") {
							choice = "supported"
						}
						answers[key] = map[string]any{"type": "choice", "choice": choice}
						if kind == "rank" {
							answers[key] = map[string]any{"type": "score", "score": 2}
						}
					}
				}
				if reply == "failed" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers, "usage": map[string]int{"input_tokens": 120, "output_tokens": 6}})
			}))
			defer provider.Close()
			cfg.Jev = jevSettings{Enabled: true, Endpoint: provider.URL, Model: "fixture", APIKey: "fixture"}
			before, _ := json.Marshal(selection)
			result := (&Server{}).reviewWithJev(context.Background(), cfg, selection, dto.PrepareTurnRequest{}, 10000, 5, nil, nil, nil)
			after, _ := json.Marshal(selection)
			if !bytes.Equal(before, after) || len(result.Items) != len(expected) {
				t.Fatal("dedup changed selection or lost original results")
			}
			refs := multiAgentReferences(selection.Candidates, nil, nil, nil)
			for i, item := range result.Items {
				if item.ID != selection.Candidates[i].CanonicalFactID || item.Ref != refs[item.ID] {
					t.Error("original order or ref changed")
				}
				wantStatus := map[string]string{"complete": "reviewed", "partial": "partial", "failed": "unavailable"}[reply]
				if item.Status != wantStatus {
					t.Error("alias lost failure/partial status", item.ID)
				}
				wantReused := expected[item.ID]
				if wantReused == item.ID {
					wantReused = ""
				}
				if item.ReusedFromID != wantReused {
					t.Error("alias trace lost representative", item.ID)
				}
				if reply != "failed" {
					choice := "unknown"
					if strings.HasSuffix(expected[item.ID], "first") {
						choice = "supported"
					}
					if item.Answers["support"].Choice != choice {
						t.Error("wrong representative answer", item.ID)
					}
				}
			}
			uniqueCount := 0
			for id, representative := range expected {
				if id == representative {
					uniqueCount++
					if seen[id] != 1 {
						t.Error("source not dispatched once", id)
					}
				}
			}
			if questions != uniqueCount*4 || len(seen) != uniqueCount {
				t.Fatal("identical reviews were still sent repeatedly", questions)
			}
			if requests <= len(selection.Roles) {
				t.Fatal("fixture must exercise nonadjacent aliases across batches")
			}
			timing := jevTimingView(&multiAgentSelection{JevReview: result})
			if intFromAny(timing["calls"], 0) != requests {
				t.Error("HUD counted logical items as physical requests")
			}
			if reply != "failed" && intFromAny(timing["input_tokens"], 0) != requests*120 {
				t.Error("HUD duplicated billed usage")
			}
		})
	}
}

func TestJevCalibratedKoreanPackingUsesAvailableStateBudget(t *testing.T) {
	cfg := defaultJevSettings()
	input := map[string]any{"role": "event_recent", "recent_conversation": strings.Repeat("가", 10000)}
	items := []jevCandidate{}
	for i := 0; i < 4; i++ {
		items = append(items, jevCandidate{ID: fmt.Sprint(i), Value: map[string]any{"text": strings.Repeat("나", 2000)}})
	}
	for _, review := range []bool{false, true} {
		batches := jevCandidateBatches(cfg, items, input, review)
		if len(batches) != 1 || !reflect.DeepEqual(batches[0], items) {
			t.Fatal("Korean estimate still splits a request that fits", review)
		}
	}
	// Quotes are ASCII too; uncommon BMP scripts and supplementary runes retain
	// their previous weights. Estimates round up after counting the JSON value.
	for text, want := range map[string]int{"abcd": 2, "가나다라": 5, "힣힣힣힣": 5, "ㄱㄴ": 5, "字字": 5, "😀": 5} {
		if got := jevEstimatedInputTokens(text); got != want {
			t.Errorf("estimate for %q = %d, want %d", text, got, want)
		}
	}
}
