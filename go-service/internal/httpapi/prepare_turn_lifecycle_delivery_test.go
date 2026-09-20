package httpapi

import (
	"bytes"
	"context"
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

func lifecycle46Fixture(sid string) (store.Memory, store.StatusCurrentValue) {
	promise := store.Memory{ID: 461, ChatSessionID: sid, TurnIndex: 10, Importance: .8,
		SummaryJSON: `{"narrative_events":[{"actor":"Mira","event":"Mira promised to return the brass compass after the voyage.","lifecycle_key":"compass-voyage","transition":"set","visibility":"public"}]}`}
	current := store.StatusCurrentValue{ID: 462, ChatSessionID: sid, StatusKey: narrativeStateStatusKey, OwnerScope: "session", OwnerID: "compass-voyage", SourceTurn: 20, WriteState: "current",
		ValueJSON:    `{"subject":"Mira's compass promise","subject_type":"entity","state_slot":"goal_status","lifecycle_key":"compass-voyage","value":"fulfilled at the northern quay","claim_scope":"objective","transition":"complete"}`,
		EvidenceJSON: `{"source":"critic.state_claims","source_revision":"accepted-20","source_turn":20,"evidence_excerpt":"Rowan accepted the compass and thanked Mira.","direct_evidence_ids":[4602]}`}
	return promise, current
}

func Test46PendingReadingPreservesStoredLifecycleProgress(t *testing.T) {
	_, current := lifecycle46Fixture("pending-reading46")
	current.ValueJSON = strings.ReplaceAll(current.ValueJSON, `"transition":"complete"`, `"transition":"partial"`)
	current.ValueJSON = strings.ReplaceAll(current.ValueJSON, "fulfilled at the northern quay", "compass repaired; return at the northern quay remains")
	query := "Mira returns the brass compass"
	pending := store.PendingThread{ID: 463, ChatSessionID: "pending-reading46", Description: "Mira promised to return the brass compass after the voyage.", Status: "open", SourceTurn: 10, Priority: 8, HookMetadataJSON: `{"lifecycle_key":"compass-voyage"}`}
	in := prepareTurnAssemblyInput{PendingThreads: []store.PendingThread{pending}, TopK: 5, MaxChars: 18000, UserInput: query, Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query, in.Perspective.Selection.CurrentTurn = query, 31
	control := buildPrepareTurnInjectionAssemblyWithBudget(in)
	in.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	plain, _ := multiAgentCandidatePool(&control)
	facts, _ := multiAgentCandidatePool(&out)
	if len(facts) == 0 || len(facts) != len(plain) {
		t.Fatal("fixture must project the pending source in both cases")
	}
	for i, fact := range facts {
		if fact.LifecycleKey != "compass-voyage" || !strings.Contains(prepareTurnMemoryReadingText(fact), "return at the northern quay remains") {
			t.Fatal("pending reading lost its stored lifecycle or current progress")
		}
		if fact.CanonicalFactID != plain[i].CanonicalFactID || fact.OriginalScore != plain[i].OriginalScore || fact.CompleteText != plain[i].CompleteText {
			t.Fatal("current linkage changed source identity, text or score")
		}
		if strings.Contains(prepareTurnMemoryReadingText(plain[i]), "return at the northern quay remains") {
			t.Fatal("negative control invented current progress")
		}
	}
	in.PendingThreads[0].HookMetadataJSON = "{}"
	legacy := buildPrepareTurnInjectionAssemblyWithBudget(in)
	legacyFacts, _ := multiAgentCandidatePool(&legacy)
	for _, fact := range legacyFacts {
		if fact.LifecycleKey != "" || strings.Contains(prepareTurnMemoryReadingText(fact), "return at the northern quay remains") {
			t.Fatal("missing stored key must not be inferred from similar text")
		}
	}
}

func Test46LifecycleLinkedReadingSurvivesCompletionOutsideRecall(t *testing.T) {
	const sid = "lifecycle46"
	promise, current := lifecycle46Fixture(sid)
	query := "Mira remembers the brass compass promise"
	input := prepareTurnAssemblyInput{Memories: []store.Memory{promise}, TopK: 1, MaxChars: 18000, UserInput: query, Profile: "default", BudgetMode: "auto",
		VectorTrace: map[string]any{"memory_search_result": "not_found", "search_result": "not_found"},
		Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(1))}
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = query, 31
	control := buildPrepareTurnInjectionAssemblyWithBudget(input)
	input.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	before := mustCompactJSON(input)
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	if mustCompactJSON(input) != before {
		t.Fatal("current linkage mutated source input")
	}
	facts, summaries := multiAgentCandidatePool(&out)
	plainFacts, _ := multiAgentCandidatePool(&control)
	plainByID := map[string]prepareTurnPriorityMemoryCandidate{}
	for _, fact := range plainFacts {
		plainByID[fact.CanonicalFactID] = fact
	}
	linked := 0
	for _, fact := range facts {
		if fact.LifecycleKey != "compass-voyage" {
			continue
		}
		linked++
		plain, ok := plainByID[fact.CanonicalFactID]
		if !ok || fact.SourceTurn != promise.TurnIndex || fact.CompleteText != plain.CompleteText || fact.OriginalScore != plain.OriginalScore || fact.Importance != plain.Importance {
			t.Fatalf("link changed historical identity/score: %+v vs %+v", fact, plain)
		}
		for _, value := range []string{"promised to return", "fulfilled at the northern quay", "Rowan accepted the compass", "source turn 20", "accepted-20"} {
			if !strings.Contains(prepareTurnMemoryReadingText(fact), value) {
				t.Errorf("selected promise lacks %q", value)
			}
		}
		for _, mode := range []string{"go", "ai", "go_fallback", "summary_only"} {
			t.Run(mode, func(t *testing.T) {
				selected := out
				if mode != "go" {
					role := multiAgentRoleResult{Role: fact.Lane, Source: "ai"}
					if mode == "ai" {
						role.Selection.SelectedIDs = []string{fact.CanonicalFactID}
					}
					if mode == "go_fallback" {
						role.Source = "go_default"
					}
					if mode == "summary_only" {
						for _, summary := range summaries {
							role.Selection.SelectedSummaryIDs = append(role.Selection.SelectedSummaryIDs, summary.SummaryID)
						}
					}
					selected.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{role}}
					selected.Preprocessing.captureBaseline(out.MemoryDeliveryPlan)
				}
				plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&selected, 18000, 1, "auto", nil, input.Perspective.Selection)
				final := extractionStringFromAny(plan["final_text"])
				for _, value := range []string{"promised to return", "fulfilled at the northern quay", "Rowan accepted the compass"} {
					if !strings.Contains(final, value) {
						t.Fatalf("%s lost linked reading %q: %s", mode, value, final)
					}
				}
				assert45Budget(t, plan, 18000)
			})
		}
	}
	if linked == 0 {
		t.Fatal("production assembly emitted no lifecycle promise")
	}
	// Supplemental projection reuses sources but must still attach current support.
	discovery := out.supplementProjection(input.VectorTrace, input.Perspective.Selection)
	for _, fact := range discovery.priorityCandidates {
		if fact.LifecycleKey == "compass-voyage" && !strings.Contains(prepareTurnMemoryReadingText(fact), "fulfilled at the northern quay") {
			t.Fatal("supplement lost linked current")
		}
	}
}

func Test46LifecycleReadingKeepsNewKeysAndPrivateEvidenceSeparate(t *testing.T) {
	promise, current := lifecycle46Fixture("separate46")
	newPromise := promise
	newPromise.ID, newPromise.TurnIndex = 463, 25
	newPromise.SummaryJSON = strings.ReplaceAll(promise.SummaryJSON, "compass-voyage", "compass-next-voyage")
	newPromise.SummaryJSON = strings.ReplaceAll(newPromise.SummaryJSON, "after the voyage", "after the next expedition")
	private := current
	private.ID, private.OwnerID, private.SourceTurn = 464, "secret-compass", 26
	private.ValueJSON = strings.ReplaceAll(current.ValueJSON, `"claim_scope":"objective"`, `"claim_scope":"secret","perspective_owner":"Rowan"`)
	private.EvidenceJSON = `{"evidence_excerpt":"PRIVATE-COMPASS-EVIDENCE"}`
	input := prepareTurnAssemblyInput{Memories: []store.Memory{promise, newPromise}, TopK: 5, MaxChars: 18000, UserInput: "Mira compass promise", Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	hits := []map[string]any{}
	for _, memory := range input.Memories {
		hits = append(hits, map[string]any{"id": fmt.Sprintf("memory:separate46:%d", memory.ID), "tier": "memory", "source_table": "memories", "source_row_id": fmt.Sprint(memory.ID), "chat_session_id": "separate46", "similarity": .9})
	}
	input.VectorTrace = map[string]any{"memory_search_result": "ok", "search_result": "ok", "memory_search_results": hits, "search_results": hits}
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = input.UserInput, 31
	input.Perspective.NarrativeValues = []store.StatusCurrentValue{current, private}
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, _ := multiAgentCandidatePool(&out)
	seenNew := false
	for _, fact := range facts {
		reading := prepareTurnMemoryReadingText(fact)
		if strings.Contains(reading, "PRIVATE-COMPASS-EVIDENCE") {
			t.Fatal("private current evidence entered general reading")
		}
		if fact.LifecycleKey == "compass-next-voyage" {
			seenNew = true
			if strings.Contains(reading, "fulfilled at the northern quay") {
				t.Fatal("new lifecycle inherited old completion")
			}
		}
	}
	if !seenNew {
		t.Fatalf("separate new promise missing: %+v", facts)
	}
	encoded, _ := json.Marshal(out.MemoryDeliveryPlan)
	if strings.Contains(string(encoded), "PRIVATE-COMPASS-EVIDENCE") {
		t.Fatal("private support leaked to final plan")
	}
}

func Test46CriticLedgerIncludesClosedLifecycleWithoutPendingThread(t *testing.T) {
	promise, current := lifecycle46Fixture("critic46")
	s := &Server{Store: &turnRecordingStore{returnMemories: []store.Memory{promise}, returnStatusCurrent: []store.StatusCurrentValue{current}}}
	out := s.buildCriticArchiveLedgerPreviewWithContext(nil, criticArchiveLedgerPreviewRequest{ChatSessionID: "critic46", TurnIndex: 31})
	text := fmt.Sprint(out.Items)
	for _, value := range []string{"fulfilled at the northern quay", "Rowan accepted the compass", "recent_resolution_event"} {
		if !strings.Contains(text, value) {
			t.Fatalf("closed current/evidence missing from Critic: %s", text)
		}
	}
}

type lifecycle46DeliveryStore struct {
	*priorityPrepareTurnStore
	t *testing.T
}

func (s *lifecycle46DeliveryStore) ListStatusCurrentValues(ctx context.Context, sid, scope, owner, key string, limit int) ([]store.StatusCurrentValue, error) {
	values, err := s.turnRecordingStore.ListStatusCurrentValues(ctx, sid, scope, owner, key, limit)
	if key == narrativeStateStatusKey {
		if limit != -1 {
			s.t.Errorf("narrative linkage must read uncapped current projection, limit=%d", limit)
		}
		if limit == 0 && len(values) > 100 {
			values = values[:100]
		}
	}
	return values, err
}

type lifecycle46RecallVector struct {
	vector.VectorStore
	doc vector.VectorDocument
}

func (v *lifecycle46RecallVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "lifecycle46", ModelReady: true}, nil
}
func (v *lifecycle46RecallVector) Search(_ context.Context, _ string, _ []float32, _ int, filter string) ([]vector.VectorDocument, error) {
	if strings.Contains(filter, `source_table == "precise_memory_units"`) {
		return nil, vector.ErrNotFound
	}
	return []vector.VectorDocument{v.doc}, nil
}

func Test46HTTPLifecycleCurrentReachesAllOptionalAIModes(t *testing.T) {
	for _, preprocess := range []bool{false, true} {
		for _, publisher := range []bool{false, true} {
			for _, mode := range []string{"normal", "empty", "failure"} {
				if !preprocess && !publisher && mode != "normal" {
					continue
				}
				t.Run(fmt.Sprintf("preprocess-%v/publisher-%v/%s", preprocess, publisher, mode), func(t *testing.T) {
					t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
					publisherCalls, agentCalls := 0, 0
					assertReading := func(text string) {
						t.Helper()
						for _, required := range []string{"promised to return", "fulfilled at the northern quay", "Rowan accepted the compass"} {
							if !strings.Contains(text, required) {
								t.Errorf("actual provider/payload missing %q: %s", required, text)
							}
						}
					}
					provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						if _, ok := body["input"]; ok {
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"embedding": []float64{.2, .1}, "index": 0}}})
							return
						}
						messages := outputFidelityLineageSlice(body["messages"])
						if len(messages) < 2 {
							t.Error("unexpected provider request")
							http.Error(w, "unexpected", 400)
							return
						}
						var input map[string]any
						_ = json.Unmarshal([]byte(extractionStringFromAny(mapFromAny(messages[1])["content"])), &input)
						assertReading(mustCompactJSON(input))
						_, isPublisher := input["supervisor_support_packet"]
						// Validate the real request assembly for both editorial routes,
						// not a separately assembled copy of their instructions.
						system := extractionStringFromAny(mapFromAny(messages[0])["content"])
						for _, guidance := range []string{"current progression", "partial fulfillment", "cancellation", "last_confirmed_story_clock", "modeled_birth_completed"} {
							if !strings.Contains(system, guidance) {
								t.Errorf("publisher=%v request lost 4.6 guidance %q", isPublisher, guidance)
							}
						}
						if isPublisher {
							publisherCalls++
						} else {
							agentCalls++
						}
						if mode == "failure" {
							http.Error(w, "fixture provider unavailable", http.StatusBadRequest)
							return
						}
						if mode == "empty" {
							_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
							return
						}
						if isPublisher {
							_, _ = w.Write([]byte(publisherV3OpenAIResponse("memories:461", "Recall the promise together with its recorded fulfillment.")))
							return
						}
						selected := []string{}
						for _, raw := range outputFidelityLineageSlice(input["turn_summaries"]) {
							item := mapFromAny(raw)
							assertReading(extractionStringFromAny(item["text"]))
							selected = append(selected, extractionStringFromAny(item["ref"]))
						}
						if len(selected) == 0 {
							t.Error("expected historical summary missing from production AI input")
						}
						result := mustCompactJSON(map[string]any{"selected_summary_ids": selected})
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": result}}}})
					}))
					defer provider.Close()
					cfg := config.Default()
					cfg.PromptDir, cfg.StoreMode = filepath.Join("..", "..", "..", "prompts"), config.StoreModeDualShadow
					cfg.ChromaEndpoint, cfg.Readiness.ChromaConfigured = "http://127.0.0.1:8000", true
					s := NewServer(cfg)
					promise, current := lifecycle46Fixture("http46")
					// Legacy accepted current evidence need not have a source revision.
					current.EvidenceJSON = `{"evidence_excerpt":"Rowan accepted the compass and thanked Mira."}`
					values := make([]store.StatusCurrentValue, 0, 106)
					for i := 0; i < 105; i++ {
						row := current
						row.ID = int64(470 + i)
						row.OwnerID = fmt.Sprintf("other-%d", i)
						row.ValueJSON = mustCompactJSON(map[string]any{"subject": row.OwnerID, "subject_type": "entity", "state_slot": "goal_status", "lifecycle_key": row.OwnerID, "value": "unrelated completed errand", "claim_scope": "objective", "transition": "complete"})
						row.EvidenceJSON = `{"evidence_excerpt":"Unrelated errand evidence."}`
						values = append(values, row)
					}
					values = append(values, current)
					s.Store = &lifecycle46DeliveryStore{priorityPrepareTurnStore: &priorityPrepareTurnStore{turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{promise}, returnStatusCurrent: values}}, t: t}
					s.Vector = &lifecycle46RecallVector{doc: vector.VectorDocument{ID: "memory:http46:461", Tier: "memory", ChatSessionID: "http46", SourceTable: "memories", SourceRowID: "461", Similarity: .94, SimilarityAvailable: true, SimilaritySource: "cosine"}}
					s.RuntimeConfig.EmbeddingProvider, s.RuntimeConfig.EmbeddingEndpoint, s.RuntimeConfig.EmbeddingModel, s.RuntimeConfig.EmbeddingAPIKey = "custom", provider.URL, "fixture-embedding", "fixture-key"
					s.RuntimeConfig.SupervisorProvider, s.RuntimeConfig.SupervisorEndpoint, s.RuntimeConfig.SupervisorModel, s.RuntimeConfig.SupervisorAPIKey = "custom", provider.URL, "fixture-publisher", "fixture-key"
					s.RuntimeConfig.EmbeddingTimeoutSec, s.RuntimeConfig.SupervisorTimeoutSec = 30, 30
					settings := defaultMultiAgentSettings()
					settings.Enabled = preprocess
					for role, c := range settings.Roles {
						c.Enabled = role == "event_recent"
						c.UsePublisher = false
						c.Provider = "custom"
						c.Endpoint = provider.URL
						c.Model = "fixture"
						c.APIKey = "fixture-key"
						settings.Roles[role] = c
					}
					b, _ := json.Marshal(settings)
					rec := httptest.NewRecorder()
					s.handleMultiAgentSettings(rec, httptest.NewRequest(http.MethodPut, "/config/memory-preprocessing", bytes.NewReader(b)))
					if rec.Code != 200 {
						t.Fatal(rec.Body.String())
					}
					mux := http.NewServeMux()
					s.RegisterRoutes(mux)
					b, _ = json.Marshal(map[string]any{"chat_session_id": "http46", "turn_index": 31, "raw_user_input": "Mira remembers the brass compass promise.", "settings": map[string]any{"injection_enabled": true, "max_injection_chars": 18000, "supervisor_enabled": publisher, "guide_mode": "standard", "guide_strength": "strong"}, "client_meta": map[string]any{"chroma_query_vector": []float64{.1, .2}}})
					rec = httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(b)))
					if rec.Code != 200 {
						t.Fatal(rec.Body.String())
					}
					var response map[string]any
					_ = json.Unmarshal(rec.Body.Bytes(), &response)
					plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
					assertReading(extractionStringFromAny(plan["final_text"]))
					assertReading(mustCompactJSON(response["payload_application_plan"]))
					assert45Budget(t, plan, 18000)
					if (publisher && publisherCalls == 0) || (!publisher && publisherCalls != 0) || (preprocess && agentCalls == 0) || (!preprocess && agentCalls != 0) {
						t.Fatalf("unexpected provider calls publisher=%d preprocessing=%d trace=%v contract=%v", publisherCalls, agentCalls, mapFromAny(response["payload_application_plan"])["guidance_application_trace"], mapFromAny(mapFromAny(response["supervisor_input_pack"])["response_execution_contract"])["status"])
					}
				})
			}
		}
	}
}

func lifecycle46AssemblyBenchmarkInput() prepareTurnAssemblyInput {
	promise, current := lifecycle46Fixture("benchmark46")
	input := prepareTurnAssemblyInput{Memories: []store.Memory{promise}, TopK: 1, MaxChars: 18000, UserInput: "Mira remembers the brass compass promise", Profile: "default", BudgetMode: "auto",
		VectorTrace: map[string]any{"memory_search_result": "not_found", "search_result": "not_found"},
		Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(1))}
	input.Perspective.Selection.Query, input.Perspective.Selection.CurrentTurn = input.UserInput, 31
	input.Perspective.NarrativeValues = []store.StatusCurrentValue{current}
	return input
}

func Benchmark46LifecycleAssembly(b *testing.B) {
	input := lifecycle46AssemblyBenchmarkInput()
	baselineInput := input
	perspective := *input.Perspective
	perspective.NarrativeValues = nil
	baselineInput.Perspective = &perspective
	baseline := buildPrepareTurnInjectionAssemblyWithBudget(baselineInput)
	linked := buildPrepareTurnInjectionAssemblyWithBudget(input)
	for _, key := range []string{"selected_fact_ids", "selected_turn_summary_ids"} {
		before, after := stringsFromAny(baseline.MemoryDeliveryPlan[key]), stringsFromAny(linked.MemoryDeliveryPlan[key])
		sort.Strings(before)
		sort.Strings(after)
		if !reflect.DeepEqual(before, after) {
			b.Fatalf("benchmark changes original selected references %s: %v => %v", key, before, after)
		}
	}
	beforeFacts, _ := multiAgentCandidatePool(&baseline)
	facts, summaries := multiAgentCandidatePool(&linked)
	original := map[string]prepareTurnPriorityMemoryCandidate{}
	for _, fact := range beforeFacts {
		original[fact.CanonicalFactID] = fact
	}
	for _, fact := range facts {
		before, ok := original[fact.CanonicalFactID]
		if !ok || fact.CompleteText != before.CompleteText || fact.SourceRef != before.SourceRef || fact.SourceTurn != before.SourceTurn || fact.OriginalScore != before.OriginalScore {
			b.Fatal("benchmark changed original fact lineage/score")
		}
	}
	modelInput := multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), input.MaxChars, 1, nil)
	preprocessChars := utf8.RuneCountInString(multiAgentModelInput(modelInput, 1))
	finalChars := utf8.RuneCountInString(extractionStringFromAny(linked.MemoryDeliveryPlan["final_text"]))
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		runtime.KeepAlive(out)
	}
	b.StopTimer()
	b.ReportMetric(float64(finalChars), "delivered_chars/op")
	b.ReportMetric(float64(preprocessChars), "preprocess_chars/op")
}

func Test46LifecycleRequestPreparationReleased(t *testing.T) {
	// Same weak-owner probe as Test44RequestPreparationReleased. The only retained
	// application value is final text; lifecycle contexts must end with the request.
	run := func() (weak.Pointer[prepareTurnRequestPreparation], string) {
		input := lifecycle46AssemblyBenchmarkInput()
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
			t.Fatal("linked lifecycle reading retained request preparation")
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		heap = append(heap, stats.HeapAlloc)
		runtime.KeepAlive(delivered)
	}
	t.Logf("post-GC process live heap across ten linked requests (bytes): %v", heap)
}
