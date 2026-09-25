package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
	"weak"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func temporal46Fixture() (store.Memory, map[string]any) {
	clock := map[string]any{"version": storyClockContractVersion, "observation_kind": "absolute", "scene_scope": "current", "precision": "exact", "absolute": map[string]any{"date": "2026-03-15"}, "source_turn": 100}
	context := map[string]any{"observed_at": map[string]any{"kind": "source_observation", "story_clock": map[string]any{"absolute": map[string]any{"date": "2026-01-01"}}}}
	item := map[string]any{"subject": "Mira", "summary": "Mira promised to bring the charts tomorrow.", "relative_expression": "tomorrow", "relative": map[string]any{"anchor": "source_observation", "offset": 1, "unit": "day", "target_kind": "planned_event"}}
	memory := store.Memory{ID: 481, ChatSessionID: "temporal46", TurnIndex: 5, Importance: .9, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": item["summary"], "temporal_context": context, "narrative_events": []any{item}})}
	return memory, clock
}

func temporal46AssemblyInput(memory store.Memory, clock map[string]any) prepareTurnAssemblyInput {
	input := prepareTurnAssemblyInput{Memories: []store.Memory{memory}, TopK: 1, MaxChars: 18000, UserInput: "Mira remembers the charts promised tomorrow.", Profile: "default", BudgetMode: "auto", VectorTrace: map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(1))}
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = input.UserInput, 101
	input.Perspective.StoryClock = clock
	return input
}

func assert46TemporalReading(t *testing.T, text string) {
	t.Helper()
	for _, required := range []string{"promised to bring the charts tomorrow", "2026-01-01", "2026-03-15", "original wording", "⏳", "reference=", "before reference", "planned_event"} {
		if !strings.Contains(text, required) {
			t.Errorf("source-relative reading lost %s", required)
		}
	}
}

func Test46TemporalReadingTravelsWithFactsAndWholeSummary(t *testing.T) {
	memory, clock := temporal46Fixture()
	input := temporal46AssemblyInput(memory, clock)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, summaries := multiAgentCandidatePool(&out)
	if len(facts) == 0 || len(summaries) != 1 {
		t.Fatalf("source fixture absent: facts=%d summaries=%d", len(facts), len(summaries))
	}
	for _, fact := range facts {
		if fact.SourceTable != "memories" {
			continue
		}
		if fact.LifecycleKey != "" || fact.SourceTurn != 5 || fact.SourceRef != "memories:481" {
			t.Fatal("temporal reading rewrote historical source identity")
		}
		assert46TemporalReading(t, prepareTurnMemoryReadingText(fact))
	}
	baselineMemory := memory
	baselinePayload := parseJSONMap(memory.SummaryJSON)
	delete(baselinePayload, "temporal_context")
	baselineItem := mapFromAny(sliceFromAny(baselinePayload["narrative_events"])[0])
	delete(baselineItem, "relative_expression")
	delete(baselineItem, "relative")
	baselineMemory.SummaryJSON = mustCompactJSON(baselinePayload)
	baseline := buildPrepareTurnInjectionAssemblyWithBudget(temporal46AssemblyInput(baselineMemory, nil))
	baselineFacts, baselineSummaries := multiAgentCandidatePool(&baseline)
	if len(baselineFacts) != len(facts) || len(baselineSummaries) != len(summaries) {
		t.Fatal("source temporal metadata expanded candidate breadth")
	}
	for i := range facts {
		if facts[i].CanonicalFactID != baselineFacts[i].CanonicalFactID || facts[i].CompleteText != baselineFacts[i].CompleteText || facts[i].Importance != baselineFacts[i].Importance || facts[i].OriginalScore != baselineFacts[i].OriginalScore {
			t.Fatal("time context changed original fact identity, text, importance or atomic score")
		}
	}
	if summaries[0].SummaryID != baselineSummaries[0].SummaryID {
		t.Fatal("time context changed original summary identity")
	}
	if summaries[0].CompleteText != "Mira promised to bring the charts tomorrow." || summaries[0].Minimum == nil {
		t.Fatal("whole summary changed or lost non-lifecycle temporal context")
	}
	assert46TemporalReading(t, summaries[0].Minimum.Text)
	assert46TemporalReading(t, extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]))
	assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars)
	modelInput := multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), input.MaxChars, 1, nil)
	assert46TemporalReading(t, mustCompactJSON(modelInput))
	projected := out.supplementProjection(input.VectorTrace, input.Perspective.Selection)
	extraFacts, extraSummaries := multiAgentCandidatePool(&projected)
	if len(extraFacts) != len(facts) || len(extraSummaries) != len(summaries) {
		t.Fatal("supplemental assembly changed original candidate breadth")
	}
	assert46TemporalReading(t, extraSummaries[0].Minimum.Text)
	if strings.Count(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), "Last confirmed stored story clock") != 1 {
		t.Fatal("current clock support was omitted or duplicated outside its existing budget")
	}
}

func Test46TemporalScheduleKeepsReadOnlyLifecycleContext(t *testing.T) {
	memory, current := lifecycle46Fixture("temporal46")
	_, clock := temporal46Fixture()
	payload := parseJSONMap(current.ValueJSON)
	payload["lifecycle_details"] = map[string]any{"schedule": map[string]any{"kind": "recurring", "last_fulfilled": map[string]any{"date": "2026-03-01"}, "recurrence": map[string]any{"unit": "day", "interval": 7}}}
	current.ValueJSON = mustCompactJSON(payload)
	input := temporal46AssemblyInput(memory, clock)
	input.UserInput = "Mira remembers the compass voyage promise."
	input.Perspective.Selection.Query = input.UserInput
	input.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, summaries := multiAgentCandidatePool(&out)
	if len(summaries) != 1 || summaries[0].Minimum == nil {
		t.Fatal("schedule-bearing whole summary missing")
	}
	for _, text := range []string{summaries[0].Minimum.Text, extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), mustCompactJSON(multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 18000, 1, nil))} {
		for _, required := range []string{"estimated next due", "2026-03-08", "2026-03-15", "due_passed_with_explicit_outcome", "reference=", "Rowan accepted the compass", "recurring"} {
			if !strings.Contains(text, required) {
				t.Errorf("existing lifecycle reading omitted schedule result %s", required)
			}
		}
		if strings.Contains(text, "missed") || strings.Contains(text, "cancelled") {
			t.Fatal("passing a due date manufactured an outcome")
		}
	}
	if stringFromMap(parseJSONMap(current.ValueJSON), "transition") != "complete" || memory.Importance != .8 {
		t.Fatal("read-only schedule computation changed source state or importance")
	}
	assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars)
}

func Test46TemporalSupportUsesExistingDeliveryBudget(t *testing.T) {
	for _, budget := range []int{100, 600, 18000} {
		t.Run(fmt.Sprintf("chars-%d", budget), func(t *testing.T) {
			memory, clock := temporal46Fixture()
			input := temporal46AssemblyInput(memory, clock)
			input.MaxChars = budget
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			assert45Budget(t, out.MemoryDeliveryPlan, budget)
			facts, _ := multiAgentCandidatePool(&out)
			if len(facts) == 0 {
				t.Fatal("delivery budget became a temporal recall filter")
			}
		})
	}
}

func Test46TemporalReadingKeepsUnknownAndCustomTime(t *testing.T) {
	for _, mode := range []string{"observation_only", "missing_anchor", "range", "custom", "unconfirmed_user_jump"} {
		t.Run(mode, func(t *testing.T) {
			memory, clock := temporal46Fixture()
			payload := parseJSONMap(memory.SummaryJSON)
			item := mapFromAny(sliceFromAny(payload["narrative_events"])[0])
			switch mode {
			case "observation_only":
				delete(item, "relative")
				delete(item, "relative_expression")
			case "missing_anchor":
				delete(payload, "temporal_context")
			case "range":
				item["occurrence_time"] = map[string]any{"range": map[string]any{"start": map[string]any{"date": "2026-01-02"}, "end": map[string]any{"date": "2026-01-05"}}}
			case "custom":
				payload["temporal_context"] = map[string]any{"observed_at": map[string]any{"kind": "source_observation", "story_clock": map[string]any{"calendar": map[string]any{"id": "harbor-era", "label": "Harbor day 20", "day_index": 20}}}}
				clock = map[string]any{"calendar": map[string]any{"id": "harbor-era", "label": "Harbor day 100", "day_index": 100}, "source_turn": 100}
			}
			memory.SummaryJSON = mustCompactJSON(payload)
			input := temporal46AssemblyInput(memory, clock)
			if mode == "unconfirmed_user_jump" {
				input.UserInput = "Ten years later, Mira remembers the charts promised tomorrow."
			}
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			facts, _ := multiAgentCandidatePool(&out)
			var timeReading map[string]any
			for _, fact := range facts {
				if fact.Reading == nil {
					continue
				}
				for _, part := range fact.Reading.Parts {
					if strings.HasPrefix(part.Key, "@temporal/") {
						timeReading = parseJSONMap(part.Value)
					}
				}
			}
			if len(timeReading) == 0 {
				t.Fatal("temporal metadata did not reach real assembly")
			}
			relation := mapFromAny(timeReading["current_relation"])
			switch mode {
			case "observation_only", "missing_anchor":
				if stringFromMap(relation, "relation") != "unknown" {
					t.Fatalf("unknown event acquired an observation date: %v", timeReading)
				}
			case "range":
				if stringFromMap(relation, "precision") != "range" {
					t.Fatalf("bounded date collapsed to exact: %v", timeReading)
				}
			case "custom":
				if !strings.Contains(mustCompactJSON(timeReading), "harbor-era") || !strings.Contains(mustCompactJSON(timeReading), "79") {
					t.Fatalf("custom source calendar lost: %v", timeReading)
				}
			case "unconfirmed_user_jump":
				if stringFromMap(relation, "basis") != "last_confirmed_story_clock" || strings.Contains(mustCompactJSON(timeReading), "2036") {
					t.Fatal("unadmitted current user jump became a confirmed scene date")
				}
			}
			assert45Budget(t, out.MemoryDeliveryPlan, input.MaxChars)
		})
	}
}

func Test46TemporalPrivateSourceCannotContaminatePublicSummary(t *testing.T) {
	memory, clock := temporal46Fixture()
	payload := parseJSONMap(memory.SummaryJSON)
	private := map[string]any{"subject": "Rowan", "summary": "PRIVATE-APPOINTMENT", "visibility": "owner_private", "perspective_owner": "Rowan", "relative_expression": "PRIVATE-TIME-EXPRESSION", "occurrence_time": map[string]any{"date": "1888-01-01"}}
	payload["narrative_events"] = append(sliceFromAny(payload["narrative_events"]), private)
	memory.SummaryJSON = mustCompactJSON(payload)
	out := buildPrepareTurnInjectionAssemblyWithBudget(temporal46AssemblyInput(memory, clock))
	facts, summaries := multiAgentCandidatePool(&out)
	text := mustCompactJSON(multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), 18000, 1, nil)) + extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	if strings.Contains(text, "PRIVATE-") || strings.Contains(text, "1888-01-01") {
		t.Fatal("private source time escaped existing public projection")
	}
	assert46TemporalReading(t, text)
}

func Test46HTTPTemporalContextReachesAllOptionalAIModes(t *testing.T) {
	for _, preprocess := range []bool{false, true} {
		for _, publisher := range []bool{false, true} {
			for _, mode := range []string{"normal", "empty", "failure"} {
				if !preprocess && !publisher && mode != "normal" {
					continue
				}
				t.Run(fmt.Sprintf("preprocess-%v/publisher-%v/%s", preprocess, publisher, mode), func(t *testing.T) {
					t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
					agentCalls, publisherCalls := 0, 0
					provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						if _, embedding := body["input"]; embedding {
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{.1, .2}, "index": 0}}})
							return
						}
						messages := outputFidelityLineageSlice(body["messages"])
						if len(messages) < 2 {
							t.Error("unexpected provider request")
							return
						}
						var input map[string]any
						_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input)
						assert46TemporalReading(t, mustCompactJSON(input))
						_, isPublisher := input["supervisor_support_packet"]
						if isPublisher {
							publisherCalls++
						} else {
							agentCalls++
						}
						if mode == "failure" {
							http.Error(w, "fixture unavailable", 400)
							return
						}
						if mode == "empty" {
							_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
							return
						}
						if isPublisher {
							_, _ = w.Write([]byte(publisherV3OpenAIResponse("memories:481", "Read tomorrow against its original observation.")))
							return
						}
						selected := []string{}
						for _, raw := range outputFidelityLineageSlice(input["turn_summaries"]) {
							item := mapFromAny(raw)
							assert46TemporalReading(t, extractionStringFromAny(item["text"]))
							selected = append(selected, extractionStringFromAny(item["ref"]))
						}
						if len(selected) != 1 {
							t.Error("non-lifecycle whole summary missing from real preprocessing input")
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": mustCompactJSON(map[string]any{"selected_summary_ids": selected})}}}})
					}))
					defer provider.Close()
					cfg := config.Default()
					cfg.PromptDir, cfg.StoreMode = filepath.Join("..", "..", "..", "prompts"), config.StoreModeDualShadow
					cfg.ChromaEndpoint, cfg.Readiness.ChromaConfigured = "http://127.0.0.1:8000", true
					s := NewServer(cfg)
					memory, clock := temporal46Fixture()
					clock["transition"], clock["evidence_excerpt"] = "set", "The calendar shows March fifteenth."
					st := &turnRecordingStore{returnMemories: []store.Memory{memory}, returnStatusCurrent: []store.StatusCurrentValue{{ID: 482, ChatSessionID: memory.ChatSessionID, StatusKey: storyClockStatusKey, OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, SourceTurn: 100, ValueJSON: mustCompactJSON(clock)}}}
					s.Store = &priorityPrepareTurnStore{turnRecordingStore: st}
					s.Vector = &lifecycle46RecallVector{doc: vector.VectorDocument{ID: "memory:temporal46:481", Tier: "memory", ChatSessionID: memory.ChatSessionID, SourceTable: "memories", SourceRowID: "481", Similarity: .94, SimilarityAvailable: true, SimilaritySource: "cosine"}}
					s.RuntimeConfig.EmbeddingProvider, s.RuntimeConfig.EmbeddingEndpoint, s.RuntimeConfig.EmbeddingModel, s.RuntimeConfig.EmbeddingAPIKey = "custom", provider.URL, "fixture", "fixture-key"
					s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "fixture", "fixture-key"
					s.RuntimeConfig.SupervisorTimeoutSec, s.RuntimeConfig.EmbeddingTimeoutSec = 30, 30
					settings := defaultMultiAgentSettings()
					settings.Enabled = preprocess
					for role, c := range settings.Roles {
						c.Enabled, c.UsePublisher = role == "event_recent", false
						c.Provider, c.Endpoint, c.Model, c.APIKey = "custom", provider.URL, "fixture", "fixture-key"
						settings.Roles[role] = c
					}
					encoded, _ := json.Marshal(settings)
					recorder := httptest.NewRecorder()
					s.handleMultiAgentSettings(recorder, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(encoded)))
					if recorder.Code != 200 {
						t.Fatal(recorder.Body.String())
					}
					mux := http.NewServeMux()
					s.RegisterRoutes(mux)
					encoded, _ = json.Marshal(map[string]any{"chat_session_id": memory.ChatSessionID, "turn_index": 101, "raw_user_input": "Ten years later, Mira remembers the charts promised tomorrow.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 18000, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}, "client_meta": map[string]any{"chroma_query_vector": []float64{.1, .2}}})
					recorder = httptest.NewRecorder()
					mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded)))
					if recorder.Code != 200 {
						t.Fatal(recorder.Body.String())
					}
					var response map[string]any
					_ = json.Unmarshal(recorder.Body.Bytes(), &response)
					plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
					assert46TemporalReading(t, extractionStringFromAny(plan["final_text"]))
					assert46TemporalReading(t, mustCompactJSON(response["payload_application_plan"]))
					assert45Budget(t, plan, 18000)
					if strings.Contains(extractionStringFromAny(plan["final_text"]), "2036") || len(st.savedStatusCurrent) > 0 || len(st.savedStatusEvents) > 0 {
						t.Fatal("read-only preparation promoted user time or wrote story state")
					}
					if (preprocess && agentCalls != 1) || (!preprocess && agentCalls != 0) || (publisher && publisherCalls != 1) || (!publisher && publisherCalls != 0) {
						t.Fatalf("wrong provider calls: preprocessing=%d publisher=%d", agentCalls, publisherCalls)
					}
				})
			}
		}
	}
}

func Benchmark46TemporalAssembly(b *testing.B) {
	memory, clock := temporal46Fixture()
	input := temporal46AssemblyInput(memory, clock)
	baselineMemory := memory
	baselinePayload := parseJSONMap(memory.SummaryJSON)
	delete(baselinePayload, "temporal_context")
	item := mapFromAny(sliceFromAny(baselinePayload["narrative_events"])[0])
	delete(item, "relative_expression")
	delete(item, "relative")
	baselineMemory.SummaryJSON = mustCompactJSON(baselinePayload)
	baseline := buildPrepareTurnInjectionAssemblyWithBudget(temporal46AssemblyInput(baselineMemory, nil))
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	for _, key := range []string{"selected_fact_ids", "selected_turn_summary_ids"} {
		before, after := stringsFromAny(baseline.MemoryDeliveryPlan[key]), stringsFromAny(out.MemoryDeliveryPlan[key])
		sort.Strings(before)
		sort.Strings(after)
		if !reflect.DeepEqual(before, after) {
			b.Fatalf("temporal benchmark changed selected source references %s", key)
		}
	}
	facts, summaries := multiAgentCandidatePool(&out)
	priorFacts, _ := multiAgentCandidatePool(&baseline)
	if len(facts) != len(priorFacts) {
		b.Fatal("temporal benchmark changed candidate breadth")
	}
	for i := range facts {
		if facts[i].CanonicalFactID != priorFacts[i].CanonicalFactID || facts[i].CompleteText != priorFacts[i].CompleteText || facts[i].SourceTurn != priorFacts[i].SourceTurn || facts[i].OriginalScore != priorFacts[i].OriginalScore {
			b.Fatal("temporal benchmark changed original source or atomic score")
		}
	}
	modelInput := multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), input.MaxChars, 1, nil)
	preprocessChars := utf8.RuneCountInString(multiAgentModelInput(modelInput, 1))
	finalChars := utf8.RuneCountInString(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]))
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		assembled := buildPrepareTurnInjectionAssemblyWithBudget(input)
		runtime.KeepAlive(assembled)
	}
	b.StopTimer()
	b.ReportMetric(float64(finalChars), "delivered_chars/op")
	b.ReportMetric(float64(preprocessChars), "preprocess_chars/op")
}

func Test46TemporalRequestPreparationReleased(t *testing.T) {
	run := func() (weak.Pointer[prepareTurnRequestPreparation], string) {
		memory, clock := temporal46Fixture()
		input := temporal46AssemblyInput(memory, clock)
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		out.supplementProjection(input.VectorTrace, input.Perspective.Selection)
		return weak.Make(out.preparation), extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])
	}
	var heap []uint64
	for n := 0; n < 10; n++ {
		pointer, delivered := run()
		for i := 0; i < 3; i++ {
			runtime.GC()
		}
		if pointer.Value() != nil {
			t.Fatal("source temporal readings retained request preparation")
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		heap = append(heap, stats.HeapAlloc)
		runtime.KeepAlive(delivered)
	}
	t.Logf("post-GC process live heap across ten temporal requests (bytes): %v", heap)
}
