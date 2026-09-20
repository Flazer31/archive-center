package httpapi

import (
	"reflect"
	"runtime"
	"testing"
	"unicode/utf8"
	"weak"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func bodyAssemblyBenchmarkInput46(enabled bool) prepareTurnAssemblyInput {
	body, clock, character := bodyDeliveryFixture46()
	addBodyPregnancyFixture46(body)
	body.Config.CycleTrackingEnabled, body.Config.AutomaticPregnancyEnabled = enabled, enabled
	input := bodyAssemblyInput46(body, clock, character)
	ordinary, _ := lifecycle46Fixture(body.SessionID)
	input.Memories = []store.Memory{ordinary}
	input.VectorTrace = map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}
	input.UserInput = "Mira remembers the brass compass promise and checks her calendar and recovery."
	input.Perspective.Selection.Query = input.UserInput
	return input
}

func Benchmark46BodyAssembly(b *testing.B) {
	baseline := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyBenchmarkInput46(false))
	enabled := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyBenchmarkInput46(true))
	beforeFacts, _ := multiAgentCandidatePool(&baseline)
	afterFacts, _ := multiAgentCandidatePool(&enabled)
	if len(beforeFacts) == 0 || len(afterFacts) != len(beforeFacts)+4 {
		b.Fatalf("fixture lost original memory or body support breadth: %d => %d", len(beforeFacts), len(afterFacts))
	}
	for _, before := range beforeFacts {
		found := false
		for _, after := range afterFacts {
			if before.CanonicalFactID == after.CanonicalFactID {
				found = true
				if before.CompleteText != after.CompleteText || before.SourceRef != after.SourceRef || before.SourceTurn != after.SourceTurn || before.OriginalScore != after.OriginalScore {
					b.Fatal("optional body support changed original memory provenance/score")
				}
			}
		}
		if !found {
			b.Fatal("optional body support dropped original memory candidate")
		}
	}
	for _, key := range []string{"selected_fact_ids", "selected_turn_summary_ids"} {
		prior, current := stringsFromAny(baseline.MemoryDeliveryPlan[key]), stringsFromAny(enabled.MemoryDeliveryPlan[key])
		for _, id := range prior {
			if !containsString(current, id) {
				b.Fatalf("optional body support displaced original selected ID %s", id)
			}
		}
	}
	for _, active := range []bool{false, true} {
		name := "disabled"
		if active {
			name = "enabled"
		}
		b.Run(name, func(b *testing.B) {
			input := bodyAssemblyBenchmarkInput46(active)
			out := buildPrepareTurnInjectionAssemblyWithBudget(input)
			facts, summaries := multiAgentCandidatePool(&out)
			modelInput := multiAgentInput("subjective_relationship", facts, summaries, dto.PrepareTurnRequest{}, defaultMultiAgentSettings(), input.MaxChars, 1, nil)
			preprocessChars := utf8.RuneCountInString(multiAgentModelInput(modelInput, 1))
			finalChars := utf8.RuneCountInString(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]))
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				assembled := buildPrepareTurnInjectionAssemblyWithBudget(input)
				runtime.KeepAlive(assembled)
			}
			b.StopTimer()
			b.ReportMetric(float64(len(facts)), "candidates/op")
			b.ReportMetric(float64(finalChars), "delivered_chars/op")
			b.ReportMetric(float64(preprocessChars), "preprocess_chars/op")
		})
	}
}

func Test46BodyRequestPreparationReleased(t *testing.T) {
	run := func() (weak.Pointer[prepareTurnRequestPreparation], string) {
		input := bodyAssemblyBenchmarkInput46(true)
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
			t.Fatal("body reading retained request preparation")
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		heap = append(heap, stats.HeapAlloc)
		runtime.KeepAlive(delivered)
	}
	t.Logf("post-GC process live heap across ten body requests (bytes): %v", heap)
}

func Test46BodyForecastSharedOperatorAssemblyReading(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	addBodyPregnancyFixture46(body)
	clockValue := store.StatusCurrentValue{ID: 748, ChatSessionID: "body46", OwnerScope: storyClockOwnerScope, OwnerID: storyClockOwnerID, StatusKey: storyClockStatusKey, ValueJSON: mustCompactJSON(clock)}
	s := &Server{Store: &turnRecordingStore{returnCharStates: []store.CharacterState{character}, returnStatusCurrent: append(body.Values, clockValue)}}
	view := s.bodyTrackingSettingsView(t.Context(), "body46", body.Config)
	estimates := view["cycle_estimates"].([]map[string]any)
	if len(estimates) != 1 {
		t.Fatalf("operator cycle estimate missing: %v", view)
	}
	operator := estimates[0]["estimate"]
	want := bodyTrackingCycleReading(body.Config.Characters[0], parseJSONMap(body.Values[0].ValueJSON), clock)
	if !reflect.DeepEqual(operator, want) {
		t.Fatalf("operator differs from shared cycle reading: %v", operator)
	}
	out := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyInput46(body, clock, character))
	assertBodyReading46(t, extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]), true)
}
