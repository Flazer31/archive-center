package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test44TypedQuerySetReachesCandidateScoring(t *testing.T) {
	queries := []string{"  current archive  ", "remembered silver compass", ""}
	want := []string{"current archive", "remembered silver compass"}
	if got := prepareTurnPriorityQuerySetFromAny(queries); !reflect.DeepEqual(got, want) {
		t.Fatalf("typed request queries disappeared: got %v want %v", got, want)
	}
	if got := prepareTurnPriorityQuerySetFromAny([]any{queries[0], queries[1], queries[2]}); !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON queries changed: %v", got)
	}
	input := priorityCandidatePoolTestInput(priorityMemoryTestContext(5))
	input.Perspective.Selection.Query = "unrelated staircase"
	input.Perspective.Selection.QuerySource = "current_input_and_recent_conversation_turns"
	input.Perspective.Selection.QuerySet = []string{input.Perspective.Selection.Query, "silver compass"}
	input.Perspective.CharacterSeeds = []prepareTurnPriorityFactSeed{{Lane: "event_recent", SourceTable: "memories", Fact: prepareTurnPriorityMemoryFact{Text: "The silver compass was delivered."}}}
	facts, _ := buildPrepareTurnSupplementCandidates(input)
	for _, fact := range facts {
		if fact.CompleteText == input.Perspective.CharacterSeeds[0].Fact.Text {
			if fact.LexicalRelevance != prepareTurnPriorityRelevance("silver compass", fact.CompleteText) {
				t.Fatalf("recent query did not reach production scoring: %+v", fact)
			}
			return
		}
	}
	t.Fatal("candidate vanished")
}

// Distinct long conversation terms and many short facts reproduce the shape of
// the reported workload without embedding a user's private session in tests.
func preprocessingEfficiencyInput() prepareTurnAssemblyInput {
	input := priorityCandidatePoolTestInput(priorityMemoryTestContext(5))
	input.Memories = nil
	for turn := 1; turn <= 48; turn++ {
		events := []any{}
		for item := 0; item < 20; item++ {
			events = append(events, map[string]any{"event": fmt.Sprintf("Mira recorded archive parcel %d at station %d with Rowan.", item, turn), "visibility": "public"})
		}
		data, _ := json.Marshal(map[string]any{"turn_summary": fmt.Sprintf("Mira checked archive station %d and Rowan kept the keys.", turn), "narrative_events": events})
		input.Memories = append(input.Memories, store.Memory{ID: int64(turn), ChatSessionID: "candidate-pool", TurnIndex: turn, Importance: 7, SummaryJSON: string(data)})
	}
	input.Perspective.Selection.CurrentTurn = 49
	for item := 0; item < 1000; item++ {
		fact := prepareTurnPriorityMemoryFact{Text: fmt.Sprintf("미라는 기록단서%c를 확인하고 로완과 약속한 보관 순서 %d를 기억했다.", rune('가'+item%500), item)}
		input.Perspective.CharacterSeeds = append(input.Perspective.CharacterSeeds, prepareTurnPriorityFactSeed{
			Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", SourceRowID: item + 100,
			SourceOccurrence: fmt.Sprintf("memory:%d", item), SourceTurn: 1 + item%48,
			Visibility: "owner_private", PerspectiveOwner: "Mira", AllowedViewers: []string{"Mira"},
			Fact: fact,
		})
		if item%2 == 0 {
			seed := &input.Perspective.CharacterSeeds[len(input.Perspective.CharacterSeeds)-1]
			seed.Visibility, seed.PerspectiveOwner, seed.AllowedViewers = "public", "", nil
			input.Perspective.Selection.SemanticFacts = append(input.Perspective.Selection.SemanticFacts, prepareTurnPrioritySemanticFact{
				UnitID: fmt.Sprintf("unit-%d", item), SourceTurn: seed.SourceTurn, Lane: seed.Lane, Fact: fact, Similarity: .8, SimilaritySource: "cosine", Visibility: "public",
			})
		}
	}
	input.Perspective.Selection.Query = input.UserInput
	input.Perspective.Selection.QuerySource = "current_input_and_recent_conversation_turns"
	input.Perspective.Selection.QuerySet = []string{input.UserInput}
	for turn := 0; turn < 5; turn++ {
		var text strings.Builder
		text.WriteString("Mira and Rowan inspect the archive. ")
		for word := 0; word < 500; word++ {
			fmt.Fprintf(&text, "기록단서%c%c을 ", rune('가'+turn), rune('가'+word))
		}
		input.Perspective.Selection.QuerySet = append(input.Perspective.Selection.QuerySet, text.String())
	}
	return input
}

func Benchmark44PreprocessingLongContext(b *testing.B) {
	input := preprocessingEfficiencyInput()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		facts, _ := buildPrepareTurnSupplementCandidates(input)
		if len(facts) < len(input.Perspective.CharacterSeeds) {
			b.Fatal("benchmark failed to exercise broad candidate construction")
		}
	}
}

func Test44SupplementRevisitsKnownEvidenceWithoutRepeatingWholePool(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "initial", Lane: "event_recent", CompleteText: "The gate closed."}}
	for i := 0; i < 30; i++ {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: fmt.Sprintf("filler-%d", i), Lane: "event_recent", CompleteText: fmt.Sprintf("Unrelated delivery number %d remained at a distant warehouse.", i)})
	}
	facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: "known-unsent", Lane: "event_recent", CompleteText: "Zirconium was stored in the old vault.", SourceTurn: 2, Visibility: "owner_private", PerspectiveOwner: "Mira", AllowedViewers: []string{"Mira"}})
	facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: "semantic-unsent", Lane: "event_recent", CompleteText: "The hidden mineral reserve was moved underground.", SourceTurn: 3, Visibility: "public"})
	before, _ := json.Marshal(facts)
	var firstCount int
	var systems []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		messages := outputFidelityLineageSlice(body["messages"])
		systems = append(systems, extractionStringFromAny(mapFromAny(messages[0])["content"]))
		var input map[string]any
		if err := json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input); err != nil {
			t.Error(err)
			return
		}
		items := outputFidelityLineageSlice(input["candidates"])
		refs := map[string]string{}
		for _, raw := range items {
			item := modelEvidenceForTest(t, input, raw)
			refs[extractionStringFromAny(item["text"])] = extractionStringFromAny(item["ref"])
			if item["id"] != nil {
				t.Error("canonical ID duplicated in model input")
			}
			if item["text"] == facts[len(facts)-2].CompleteText && (item["perspective_owner"] != "Mira" || !reflect.DeepEqual(stringsFromAny(item["allowed_viewers"]), []string{"Mira"})) {
				t.Error("private evidence lost attribution")
			}
		}
		answer := multiAgentRecommendation{}
		if intFromAny(input["analysis_round"], 0) == 1 {
			firstCount = len(items)
			if refs[facts[len(facts)-2].CompleteText] != "" {
				t.Error("fixture did not place the known evidence outside the first input window")
			}
			answer.SelectedIDs = []string{refs[facts[0].CompleteText]}
			answer.SearchRequests = []string{"Zirconium"}
		} else {
			if len(items) >= len(facts) || firstCount == 0 {
				t.Error("second analysis bypassed its input window")
			}
			for _, f := range []prepareTurnPriorityMemoryCandidate{facts[0], facts[len(facts)-2], facts[len(facts)-1]} {
				if refs[f.CompleteText] == "" {
					t.Errorf("second input lost first selection or rediscovered source %s", f.CanonicalFactID)
				}
			}
			answer.SelectedIDs = []string{refs[facts[len(facts)-1].CompleteText], refs[facts[len(facts)-2].CompleteText], refs[facts[0].CompleteText]}
		}
		content, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"usageMetadata": map[string]any{"promptTokenCount": 123, "candidatesTokenCount": 17}, "choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	cfg.CandidateChars = 512
	for role, c := range cfg.Roles {
		c.Enabled = role == "event_recent"
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture-"+role, "fixture"
		cfg.Roles[role] = c
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 2000, 5, nil, func(query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		if query != "Zirconium" {
			t.Errorf("unexpected supplemental query: %s", query)
		}
		found := append([]prepareTurnPriorityMemoryCandidate{}, facts...)
		found[len(found)-1].SupplementalQueryMatched = true
		return found, nil, map[string]any{"status": "ready"}
	})
	role := result.role("event_recent")
	if !reflect.DeepEqual(role.Selection.SelectedIDs, []string{"semantic-unsent", "known-unsent", "initial"}) {
		t.Fatalf("AI order or canonical resolution changed: %v", role.Selection.SelectedIDs)
	}
	if len(result.Candidates) != len(facts) {
		t.Error("focused model packet altered the full Go source pool")
	}
	if len(systems) != 2 || systems[0] != systems[1] {
		t.Error("round change invalidated the static system prefix")
	}
	for _, call := range role.Calls {
		if call.Usage == nil {
			t.Error("provider usageMetadata was lost")
		}
	}
	after, _ := json.Marshal(facts)
	if string(before) != string(after) {
		t.Error("supplement mutated the initial source pool")
	}
}

func Test44CommonSourcesRemainRequestLocalAndQuestionIndependent(t *testing.T) {
	input := preprocessingEfficiencyInput()
	input.Perspective.Selection.QuerySet = []string{"Mira archive"}
	common := prepareTurnCommonAssemblySources(input)
	before, _ := json.Marshal(common)
	for _, query := range []string{"Mira archive", "Rowan parcel", "기록단서가"} {
		perspective := *input.Perspective
		perspective.Selection.QuerySet = []string{query}
		input.Perspective = &perspective
		input.Common = nil
		wantFacts, wantSummaries := buildPrepareTurnSupplementCandidates(input)
		input.Common = common
		gotFacts, gotSummaries := buildPrepareTurnSupplementCandidates(input)
		if !reflect.DeepEqual(wantFacts, gotFacts) || !reflect.DeepEqual(wantSummaries, gotSummaries) {
			t.Fatalf("common source preparation changed %q results", query)
		}
	}
	after, _ := json.Marshal(common)
	if string(before) != string(after) {
		t.Fatal("one search mutated common sources used by other roles")
	}
}

func Test44LowerScoredSupplementKeepsQueryMatch(t *testing.T) {
	input := priorityCandidatePoolTestInput(priorityMemoryTestContext(5))
	fact := prepareTurnPriorityMemoryFact{Text: "Mira keeps the silver compass."}
	input.Perspective.CharacterSeeds = []prepareTurnPriorityFactSeed{{Lane: "event_recent", SourceTable: "memories", SourceRowID: 12, SourceTurn: 3, Fact: fact}}
	input.Perspective.Selection.SemanticFacts = []prepareTurnPrioritySemanticFact{
		{UnitID: "original", SourceTurn: 3, Lane: "event_recent", Fact: fact, Similarity: .95, Visibility: "public"},
		{UnitID: "supplement", SourceTurn: 3, Lane: "event_recent", Fact: fact, Similarity: .4, Visibility: "public", SupplementalQueryMatched: true},
	}
	facts, _ := buildPrepareTurnSupplementCandidates(input)
	for _, c := range facts {
		if c.CompleteText == fact.Text {
			if c.SemanticUnitID != "original" || !c.SupplementalQueryMatched {
				t.Fatalf("lower-scored query lost its retrieval evidence: %+v", c)
			}
			return
		}
	}
	t.Fatal("matched canonical source vanished")
}

// Optional local replay checks the real serializer against exported canonical
// inputs. No session text or provider credentials are committed to the repo.
func Test44RecordedModelInputPacking(t *testing.T) {
	file := os.Getenv("AC_TEST_PREPROCESSING_TRACE")
	if file == "" {
		t.Skip("optional local exported trace")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	offset := strings.Index(string(data), "{")
	if offset < 0 {
		t.Fatal("missing trace JSON")
	}
	var trace map[string]any
	if err := json.NewDecoder(strings.NewReader(string(data[offset:]))).Decode(&trace); err != nil {
		t.Fatal(err)
	}
	report := []map[string]any{}
	for _, roleRaw := range outputFidelityLineageSlice(trace["roles"]) {
		role := mapFromAny(roleRaw)
		for _, callRaw := range outputFidelityLineageSlice(role["calls"]) {
			call := mapFromAny(callRaw)
			input := mapFromAny(call["input"])
			round := intFromAny(call["round"], 0)
			wire := multiAgentModelInput(input, round)
			var packed map[string]any
			_ = json.Unmarshal([]byte(wire), &packed)
			for _, group := range []string{"candidates", "turn_summaries", "lorebook_candidates", "related_evidence"} {
				originals := outputFidelityLineageSlice(input[group])
				items := outputFidelityLineageSlice(packed[group])
				if len(originals) != len(items) {
					t.Fatalf("%s candidate count changed", group)
				}
				for i, raw := range originals {
					old := mapFromAny(raw)
					got := modelEvidenceForTest(t, packed, items[i])
					for _, key := range []string{"text", "ref", "source_ref", "source_table", "source_turn", "visibility", "perspective_owner", "allowed_viewers"} {
						if !reflect.DeepEqual(old[key], got[key]) {
							t.Fatalf("%s lost %s", group, key)
						}
					}
				}
			}
			reconstructed := []any{}
			for _, raw := range outputFidelityLineageSlice(packed["recent_conversation"]) {
				turn := mapFromAny(raw)
				turn["Text"] = modelRecentTextForTest(turn)
				reconstructed = append(reconstructed, turn)
			}
			if !reflect.DeepEqual(input["recent_conversation"], reconstructed) {
				t.Fatal("configured recent context changed")
			}
			report = append(report, map[string]any{"role": role["role"], "round": round, "wire_chars": len([]rune(wire)), "before_wire_chars": call["model_input_chars"], "system_prompt_chars": call["system_prompt_chars"], "canonical_candidates": len(outputFidelityLineageSlice(input["candidates"]))})
		}
	}
	if len(report) == 0 {
		t.Fatal("trace replayed no model calls")
	}
	out := os.Getenv("AC_TEST_PREPROCESSING_REPORT")
	if out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			t.Fatal(err)
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(out, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("replayed %d recorded model inputs without changing evidence or recent context", len(report))
}

func modelRecentTextForTest(turn map[string]any) string {
	if text, ok := turn["Text"].(string); ok {
		return text
	}
	var text strings.Builder
	for _, raw := range outputFidelityLineageSlice(turn["Text"]) {
		text.WriteString(extractionStringFromAny(mapFromAny(raw)["text"]))
	}
	return text.String()
}

func Test44RecentContextIsChosenByEachEditorAndPreservedVerbatim(t *testing.T) {
	current := "Mira visits Rowan with the secret still private."
	limit := 2
	req := dto.PrepareTurnRequest{RawUserInput: &current, Settings: dto.PrepareTurnSettings{RecentConversationReferenceCount: &limit}, Messages: []map[string]any{
		{"role": "user", "content": "Remember the older promise."},
		{"role": "assistant", "content": "Rowan promised tea.\r\n\r\nMira privately feared rejection.\r\n\r\n" + strings.Repeat("Older scenery remains available in round one. ", 150)},
		{"role": "user", "content": "Keep Mira's knowledge private."},
		{"role": "assistant", "content": "Rowan opened the door.\n\nOnly Mira knows the password.\n\n" + strings.Repeat("New scenery remains available in round one. ", 150)},
		{"role": "user", "content": current},
	}}
	wantRecent := prepareTurnRecentConversationQueries(req.Messages, limit)
	facts := []prepareTurnPriorityMemoryCandidate{}
	for _, role := range multiAgentRoles {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: role, Lane: role, CompleteText: "An exact memory belonging to " + role, SourceTurn: 3})
	}
	var mu sync.Mutex
	firstCount := 0
	barrier := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		wire := extractionStringFromAny(mapFromAny(outputFidelityLineageSlice(body["messages"])[1])["content"])
		var input map[string]any
		_ = json.Unmarshal([]byte(wire), &input)
		role := extractionStringFromAny(input["role"])
		items := outputFidelityLineageSlice(input["candidates"])
		if len(items) != 1 || mapFromAny(items[0])["text"] != "An exact memory belonging to "+role || input["current_input"] != current {
			t.Error("candidate or current input changed while compacting recent context")
		}
		ref := extractionStringFromAny(mapFromAny(items[0])["ref"])
		recent := outputFidelityLineageSlice(input["recent_conversation"])
		if len(recent) != len(wantRecent) {
			t.Error("recent turn positions changed")
		}
		answer := map[string]any{"selected_ids": []string{ref}}
		if intFromAny(input["analysis_round"], 0) == 1 {
			for i, raw := range recent {
				if modelRecentTextForTest(mapFromAny(raw)) != wantRecent[i].Text {
					t.Error("first round did not receive all configured recent text verbatim")
				}
			}
			mu.Lock()
			firstCount++
			if firstCount == len(multiAgentRoles) {
				close(barrier)
			}
			mu.Unlock()
			select {
			case <-barrier:
			case <-time.After(3 * time.Second):
				t.Error("first round became serial")
			}
			contextRef := "C2.1"
			if role == "subjective_relationship" {
				contextRef = "C1.1"
			}
			answer["recent_context_refs"] = []string{contextRef}
			answer["search_requests"] = []string{"Find the consequence for " + role}
			answer["reasons"] = map[string]string{ref: "The prior interpretation for " + role}
		} else {
			if strings.Contains(wire, "scenery remains available") || input["recent_context_status"] != "first_round_selected_verbatim_passages" {
				t.Error("second round resent unselected recent prose")
			}
			if role == "subjective_relationship" {
				if modelRecentTextForTest(mapFromAny(recent[0])) != "user:\nKeep Mira's knowledge private.\nassistant:\nRowan opened the door.\n\nOnly Mira knows the password.\n\n" || modelRecentTextForTest(mapFromAny(recent[1])) != "" {
					t.Error("private editor's exact context choice changed")
				}
			} else if modelRecentTextForTest(mapFromAny(recent[1])) != "user:\nRemember the older promise.\nassistant:\nRowan promised tea.\r\n\r\nMira privately feared rejection.\r\n\r\n" || modelRecentTextForTest(mapFromAny(recent[0])) != "" {
				t.Error("one editor's choice leaked into another editor's recent context")
			}
			answer["reuse_previous_reasons"] = true
		}
		b, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(b)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for _, role := range multiAgentRoles {
		c := cfg.Roles[role]
		c.Enabled, c.UsePublisher = true, false
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture-"+role, "fixture-key"
		cfg.Roles[role] = c
	}
	got := (&Server{}).runMultiAgent(context.Background(), cfg, req, facts, nil, 32000, 8, nil, func(string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
		return nil, nil, map[string]any{"status": "ok"}
	})
	if got.AnalysisCalls != 10 {
		t.Fatalf("unexpected extra or missing calls: %d", got.AnalysisCalls)
	}
	for _, role := range got.Roles {
		if role.Source != "ai" || !reflect.DeepEqual(role.Selection.SelectedIDs, []string{role.Role}) || role.Selection.Reasons[role.Role] != "The prior interpretation for "+role.Role {
			t.Errorf("explicit reason reuse changed recommendation: %+v", role.Selection)
		}
		for _, call := range role.Calls {
			if !reflect.DeepEqual(call.Input["recent_conversation"], wantRecent) {
				t.Error("canonical full recent context mutated")
			}
		}
		if role.Calls[1].ModelInputChars >= role.Calls[0].ModelInputChars {
			t.Error("supplement did not reduce this long recent input")
		}
	}
}

func Test44RecentContextOptionalAndMalformedReferencesKeepFullContext(t *testing.T) {
	for _, raw := range []string{`{}`, `{"recent_context_refs":null}`, `{"recent_context_refs":["C99.1"]}`, `{"recent_context_refs":["C1.1","unknown"]}`, `{"recent_context_refs":[]}`} {
		previous, err := parseMultiAgentRecommendation(raw)
		if err != nil {
			t.Fatal(err)
		}
		input := map[string]any{"recent_conversation": []prepareTurnRetrievalQuery{{Text: "첫 문단\n\n둘째 문단", Source: "recent_conversation_turn"}}, "previous_result": previous}
		before, _ := json.Marshal(input)
		var packed map[string]any
		_ = json.Unmarshal([]byte(multiAgentModelInput(input, 2)), &packed)
		got := modelRecentTextForTest(mapFromAny(outputFidelityLineageSlice(packed["recent_conversation"])[0]))
		want := "첫 문단\n\n둘째 문단"
		if raw == `{"recent_context_refs":[]}` {
			want = ""
		}
		if got != want {
			t.Errorf("%s: got %q want %q", raw, got, want)
		}
		after, _ := json.Marshal(input)
		if string(before) != string(after) {
			t.Fatal("canonical input mutated")
		}
	}
}

func Test44ReasonReusePreservesCompleteSelectionAndExplicitChanges(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		ids            []string
		reasons        map[string]string
	}{
		{"unchanged", `{"selected_ids":["F2","F1"],"reuse_previous_reasons":true}`, []string{"second", "first"}, map[string]string{"first": "first reason", "second": "second reason"}},
		{"changed", `{"selected_ids":["F3","F1"],"reuse_previous_reasons":true,"reasons":{"F3":"new evidence","F1":"revised meaning"}}`, []string{"third", "first"}, map[string]string{"third": "new evidence", "first": "revised meaning"}},
		{"cleared", `{"selected_ids":["F1"],"reuse_previous_reasons":true,"reasons":{"F1":""}}`, []string{"first"}, map[string]string{"first": ""}},
		{"empty", `{"selected_ids":[],"reuse_previous_reasons":true}`, []string{}, map[string]string{}},
		{"legacy", `{"selected_ids":["F1"],"reasons":{}}`, []string{"first"}, map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				_ = json.NewDecoder(r.Body).Decode(&request)
				system := extractionStringFromAny(mapFromAny(outputFidelityLineageSlice(request["messages"])[0])["content"])
				if !strings.Contains(system, "Saved shared editor instructions") || !strings.Contains(system, "Saved role instructions") || !strings.Contains(system, multiAgentReviewTransport) {
					t.Error("saved prompts or current response transport instructions were not delivered")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": tc.response}}}})
			}))
			defer provider.Close()
			facts := []prepareTurnPriorityMemoryCandidate{}
			for _, id := range []string{"first", "second", "third"} {
				facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: "event_recent", CompleteText: "Exact source " + id})
			}
			cfg := defaultMultiAgentSettings()
			cfg.SharedPrompt = "Saved shared editor instructions"
			c := cfg.Roles["event_recent"]
			c.Prompt = "Saved role instructions"
			c.UsePublisher = false
			c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
			cfg.Roles["event_recent"] = c
			input := multiAgentInput("event_recent", facts, nil, dto.PrepareTurnRequest{}, cfg, 32000, 8, nil)
			prior := multiAgentRecommendation{SelectedIDs: []string{"first", "second"}, Reasons: map[string]string{"first": "first reason", "second": "second reason"}}
			input["previous_result"] = prior
			call := (&Server{}).callMultiAgent(context.Background(), "event_recent", cfg, 2, input)
			if call.Error != "" || !reflect.DeepEqual(call.Result.SelectedIDs, tc.ids) || !reflect.DeepEqual(call.Result.Reasons, tc.reasons) {
				t.Fatalf("model-directed merge changed meaning: %+v error=%s", call.Result, call.Error)
			}
			if prior.Reasons["first"] != "first reason" || len(prior.Reasons) != 2 {
				t.Fatal("first analysis was mutated")
			}
			if call.ModelInputSectionsChars["previous_result"] == 0 {
				t.Fatal("input cost sections were not recorded")
			}
		})
	}
}

func Test44IndexedRelevancePreservesExistingTermPolicy(t *testing.T) {
	queries := []string{"", "Mira silver compass", "스트레칭을 다시 하자", "스트레칭", "a a b", "x xxyz", "東京へ 大阪から", "caé aé abc한", "한글.값!", "Mira Mira ancient promise returned"}
	texts := append([]string{"unrelated words", "The SILVER compass was found by Mira", "스트레칭을 기억했다", "스트레칭을을 연습했다", "東京 이동", "caé aé abc한글"}, queries...)
	for _, query := range queries {
		for _, text := range texts {
			terms := prepareTurnDistinctiveRecallTerms(query)
			if len(terms) == 0 {
				terms = prepareTurnRecallTerms(query)
			}
			want := .5
			if len(terms) > 0 {
				n := prepareTurnPriorityOverlapCount(terms, text)
				denom := maxInt(minInt(len(terms), 4), 1)
				if n == 0 {
					all := prepareTurnRecallTerms(query)
					if extra := prepareTurnPriorityInflectedNonASCIIOverlapCount(all, text); extra > 0 {
						n, denom = extra, maxInt(minInt(len(all), 4), 1)
					}
				}
				want = float64(n) / float64(denom)
				if prepareTurnRecallContainsAnchor(text, query) || want > 1 {
					want = 1
				}
			}
			if got := prepareTurnPriorityRelevance(query, text); got != want {
				t.Fatalf("indexed match changed existing exact/inflection policy for %q in %q: %v != %v", query, text, got, want)
			}
		}
	}
}

func Test44RepeatedSpecialistReasonsKeepEveryScope(t *testing.T) {
	reason := "The recorded handover explains the present inventory."
	selection := &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "one"}, {CanonicalFactID: "two"}}, Roles: []multiAgentRoleResult{{Role: "event_recent", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{"one", "two"}, Reasons: map[string]string{"one": reason, "two": reason}}}}}
	plan := map[string]any{"priority_items": []any{map[string]any{"canonical_fact_id": "one", "selection_status": "selected", "source_ref": "memories:1", "source_turn": 1, "visibility": "public"}, map[string]any{"canonical_fact_id": "two", "selection_status": "selected", "source_ref": "memories:2", "source_turn": 2, "visibility": "owner_private", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira"}}}}
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	text := extractionStringFromAny(notes["final_text"])
	if strings.Count(text, reason) != 1 {
		t.Fatal("identical adjacent prose repeated")
	}
	items := outputFidelityLineageSlice(notes["items"])
	if len(items) != 2 || len(mapFromAny(notes["source_catalog"])) != 2 {
		t.Fatal("grouped prose lost individual evidence or scope")
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		if !strings.Contains(text, extractionStringFromAny(item["evidence_ref"])) || !strings.Contains(extractionStringFromAny(item["final_text"]), reason) {
			t.Fatal("individual interpretation no longer traceable")
		}
	}
	if !strings.Contains(text, "Mira") || !strings.Contains(text, "owner_private") {
		t.Fatal("grouping broadened knowledge scope")
	}
}

func Test44LargeRecommendationPacketKeepsEvidenceWithoutReasonEcho(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{}
	previous := multiAgentRecommendation{Reasons: map[string]string{}, SearchRequests: []string{"Which public disclosure changed recognition?"}, Unresolved: []string{"The disclosure date is unstated."}}
	for i := 0; i < 176; i++ {
		id := fmt.Sprintf("fact-%d", i)
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: "subjective_relationship", SourceTable: "protagonist_entity_memories", SourceRef: fmt.Sprintf("protagonist_entity_memories:%d", i), SourceTurn: i + 1, CompleteText: fmt.Sprintf("Mira remembers episode %d and the private promise made to Rowan.", i), Visibility: "owner_private", PerspectiveOwner: "Mira", AllowedViewers: []string{"Mira"}})
		previous.SelectedIDs = append(previous.SelectedIDs, id)
		previous.Reasons[id] = fmt.Sprintf("Unique first analysis %d: %s", i, strings.Repeat("Recorded detail and inferred scene connection. ", 8))
	}
	input := multiAgentInput("subjective_relationship", facts, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 16000, 8, nil)
	input["previous_result"] = previous
	before, _ := json.Marshal(input)
	wire := multiAgentModelInput(input, 2)
	var packed map[string]any
	if err := json.Unmarshal([]byte(wire), &packed); err != nil {
		t.Fatal(err)
	}
	result := mapFromAny(packed["previous_result"])
	if _, exists := result["reasons"]; exists || strings.Contains(wire, "Unique first analysis") {
		t.Fatal("first reasons repeated in the second request")
	}
	if !reflect.DeepEqual(stringsFromAny(result["search_requests"]), previous.SearchRequests) || !reflect.DeepEqual(stringsFromAny(result["unresolved"]), previous.Unresolved) {
		t.Fatal("questions or uncertainties disappeared")
	}
	selected := stringsFromAny(result["selected_ids"])
	if len(selected) != len(facts) {
		t.Fatal("selection was shortened to save tokens")
	}
	for i, raw := range outputFidelityLineageSlice(packed["candidates"]) {
		item := modelEvidenceForTest(t, packed, raw)
		if item["ref"] != selected[i] || item["text"] != facts[i].CompleteText || item["source_turn"] != float64(facts[i].SourceTurn) || item["perspective_owner"] != "Mira" {
			t.Fatal("selected source or order changed")
		}
	}
	if len(mapFromAny(packed["source_scopes"])) != 1 || len(mapFromAny(packed["source_catalog"])) != len(facts) {
		t.Fatal("provenance was not factored by scope independently of source rows")
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("canonical first analysis was changed")
	}
	// Reinsert only the omitted prose: this must be larger, independent of any
	// candidate-size threshold or model behavior.
	echoed := mapFromAny(packed["previous_result"])
	echoed["reasons"] = previous.Reasons
	full, _ := json.Marshal(packed)
	if len(full) <= len(wire) {
		t.Fatal("whole packet did not shrink")
	}
}

func Test44FiveRoleSearchesKeepPriorAndSurroundingEvidence(t *testing.T) {
	words := []string{"Amberium", "Berylith", "Cobaltium", "Dunecite", "Ebonium"}
	facts := []prepareTurnPriorityMemoryCandidate{}
	choices := map[string][]string{}
	questions := map[string]string{}
	for index, role := range multiAgentRoles {
		questions[role] = words[index]
		count := 1
		if role == "subjective_relationship" {
			count = 176
		}
		for i := 0; i < count; i++ {
			id := fmt.Sprintf("%s-selected-%d", role, i)
			facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, Lane: role, CompleteText: fmt.Sprintf("Remembered independent episode %d held by role %d.", i, index), SourceRef: id, Visibility: "public"})
			choices[role] = append(choices[role], id)
		}
		// A known precise hit from the initial query is not evidence of every new query.
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: role + "-old-semantic", Lane: role, CompleteText: "A remote old tower has a bell.", SemanticUnitID: "old", Relevance: .99})
		for _, word := range words {
			facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: role + "-" + word, Lane: role, CompleteText: word + " was documented in the ledger.", Visibility: "public"})
		}
	}
	aliases := multiAgentReferences(facts, nil, nil, nil)
	byRef := map[string]string{}
	for id, ref := range aliases {
		byRef[ref] = id
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		messages := outputFidelityLineageSlice(body["messages"])
		var input map[string]any
		if err := json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input); err != nil {
			t.Error(err)
			return
		}
		role := extractionStringFromAny(input["role"])
		available := map[string]bool{}
		for _, raw := range outputFidelityLineageSlice(input["candidates"]) {
			available[byRef[extractionStringFromAny(mapFromAny(raw)["ref"])]] = true
		}
		answer := multiAgentRecommendation{SelectedIDs: append([]string{}, choices[role]...)}
		for _, id := range choices[role] {
			if !available[id] {
				t.Errorf("lost first selected source %s", id)
			}
		}
		if intFromAny(input["analysis_round"], 0) == 1 {
			answer.SearchRequests = []string{questions[role]}
		} else {
			if !available[role+"-"+questions[role]] {
				t.Errorf("own question evidence missing for %s", role)
			}
			for _, word := range words {
				if !available[role+"-"+word] {
					t.Errorf("surrounding discovery disappeared from %s: %s", role, word)
				}
			}
			answer.SelectedIDs = append([]string{role + "-" + questions[role]}, answer.SelectedIDs...)
		}
		content, _ := json.Marshal(answer)
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}}}})
	}))
	defer provider.Close()
	cfg := defaultMultiAgentSettings()
	cfg.Enabled = true
	for role, c := range cfg.Roles {
		c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture-"+role, "fixture"
		cfg.Roles[role] = c
	}
	result := (&Server{}).runMultiAgent(context.Background(), cfg, dto.PrepareTurnRequest{}, facts, nil, 32000, 8, nil,
		func(query string) ([]prepareTurnPriorityMemoryCandidate, []prepareTurnPriorityTurnSummaryCandidate, map[string]any) {
			found := append([]prepareTurnPriorityMemoryCandidate{}, facts...)
			for i := range found {
				if strings.Contains(found[i].CompleteText, query) {
					found[i].SupplementalQueryMatched = true
				}
			}
			return found, nil, map[string]any{"status": "ready"}
		})
	if len(result.Candidates) != len(facts) {
		t.Fatal("model focus removed Go fallback candidates")
	}
	for _, role := range result.Roles {
		want := append([]string{role.Role + "-" + questions[role.Role]}, choices[role.Role]...)
		if len(role.Calls) != 2 || !reflect.DeepEqual(role.Selection.SelectedIDs, want) {
			t.Fatalf("final AI order changed for %s", role.Role)
		}
	}
}

func Test44PreparedRecallKeepsSourceDataAcrossConcurrentQuestions(t *testing.T) {
	input := preprocessingEfficiencyInput()
	common := prepareTurnCommonAssemblySources(input)
	before := map[store.Memory]prepareTurnRecallMemory{}
	for key, value := range common.RecallMemories {
		copyValue := value
		copyValue.anchors = append([]string{}, value.anchors...)
		copyValue.terms, copyValue.phrases = map[string]bool{}, map[string]bool{}
		for k, v := range value.terms {
			copyValue.terms[k] = v
		}
		for k, v := range value.phrases {
			copyValue.phrases[k] = v
		}
		before[key] = copyValue
	}
	var wg sync.WaitGroup
	for _, question := range []string{"Mira archive", "Rowan parcel", "기록단서가", "unknown", "promise"} {
		wg.Add(1)
		go func(question string) {
			defer wg.Done()
			local := input
			perspective := *input.Perspective
			perspective.Selection.QuerySet = append(append([]string{}, input.Perspective.Selection.QuerySet...), question)
			local.Perspective = &perspective
			wantFacts, wantSummaries := buildPrepareTurnSupplementCandidates(local)
			local.Common = common
			gotFacts, gotSummaries := buildPrepareTurnSupplementCandidates(local)
			if !reflect.DeepEqual(wantFacts, gotFacts) || !reflect.DeepEqual(wantSummaries, gotSummaries) {
				t.Errorf("prepared recall changed %q", question)
			}
		}(question)
	}
	wg.Wait()
	for key, value := range common.RecallMemories {
		old := before[key]
		if strings.Join(old.anchors, "|") != strings.Join(value.anchors, "|") || !reflect.DeepEqual(old.terms, value.terms) || !reflect.DeepEqual(old.phrases, value.phrases) || old.text != value.text {
			t.Fatal("shared source mutated")
		}
	}
}

func Benchmark44FiveSupplementAssemblies(b *testing.B) {
	input := preprocessingEfficiencyInput()
	input.Common = prepareTurnCommonAssemblySources(input)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, question := range []string{"Mira archive", "Rowan parcel", "기록단서가", "hidden mineral", "old promise"} {
			local := input
			perspective := *input.Perspective
			perspective.Selection.QuerySet = append(append([]string{}, input.Perspective.Selection.QuerySet...), question)
			local.Perspective = &perspective
			facts, _ := buildPrepareTurnSupplementCandidates(local)
			if len(facts) < len(input.Perspective.CharacterSeeds) {
				b.Fatal("memory breadth lost")
			}
		}
	}
}

func Test44SupplementLoreKeepsPriorSelectionsWithoutSpareRefill(t *testing.T) {
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "fact", Lane: "world_state", CompleteText: "The old gate is open."}}
	lore := []map[string]any{{"id": "retained", "text": strings.Repeat("A", 18000)}, {"id": "extra", "text": strings.Repeat("B", 5000)}}
	ctx := map[string]any{"lorebook_candidates": lore, "supplemental_lore_review": true, "retained_ids": map[string]bool{"retained": true, "fact": true}}
	in := multiAgentInput("world_state", facts, nil, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 32000, 8, nil, ctx)
	supplied := outputFidelityLineageSlice(in["lorebook_candidates"])
	if len(supplied) != 1 || mapFromAny(supplied[0])["id"] != "retained" {
		t.Fatal("prior whole lore selection lost or spare space refilled with another entry")
	}
	if len(outputFidelityLineageSlice(in["candidates"])) != 1 || intFromAny(in["input_candidate_chars"], 0) > 32000 {
		t.Fatal("cross-group reservation lost facts or exceeded input space")
	}
}
